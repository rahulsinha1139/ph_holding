// =============================================================================
// BANK STATEMENT PARSER SERVICE
//
// MISSION: Parse bank statements from PDF (via OCR) and CSV formats
// FORMATS: NBB (National Bank of Bahrain), generic CSV
// =============================================================================

package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// PARSED STRUCTURES
// =============================================================================

type parsedStatement struct {
	AccountNumber  string
	IBAN           string
	Currency       string
	PeriodStart    time.Time
	PeriodEnd      time.Time
	OpeningBalance float64
	ClosingBalance float64
	TotalDebits    float64
	TotalCredits   float64
	DebitCount     int
	CreditCount    int
	Lines          []parsedLine
}

type parsedLine struct {
	LineNumber  int
	Date        time.Time
	ValueDate   time.Time
	Reference   string
	Description string
	Debit       float64
	Credit      float64
	Balance     float64
}

// =============================================================================
// PDF IMPORT (via OCR + Butler AI)
// =============================================================================

// ImportBankStatementPDF imports a bank statement from PDF using OCR
func (a *App) ImportBankStatementPDF(filePath string, bankAccountID string) (*BankStatement, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Verify bank account exists
	var bankAccount CompanyBankAccount
	if err := a.db.First(&bankAccount, "id = ?", bankAccountID).Error; err != nil {
		return nil, fmt.Errorf("bank account not found: %w", err)
	}

	// Step 1: OCR the PDF. Use the direct OCR pipeline so we always receive the
	// extracted text payload instead of the lightweight quick-capture summary map.
	ocrResult, err := a.ProcessDocumentWithOCR(filePath, "bank_statement")
	if err != nil {
		return nil, fmt.Errorf("OCR failed: %w", err)
	}

	if ocrResult == nil {
		return nil, fmt.Errorf("OCR returned empty result")
	}

	extractedText := strings.TrimSpace(ocrResult.Text)

	if extractedText == "" {
		return nil, fmt.Errorf("OCR returned empty text")
	}

	// Step 2: Parse and validate the bank statement.
	parseOutcome, err := a.parseImportedPDFBankStatement(extractedText)
	if err != nil {
		return nil, fmt.Errorf("failed to parse statement: %w", err)
	}
	parsed := parseOutcome.Statement

	// Step 3: Create the BankStatement
	statementNumber := fmt.Sprintf("%s-%s", bankAccount.AccountNumber, parsed.PeriodEnd.Format("200601"))

	statement := &BankStatement{
		Base:            Base{ID: uuid.New().String()},
		BankAccountID:   bankAccountID,
		StatementNumber: statementNumber,
		StatementDate:   time.Now(),
		PeriodStart:     parsed.PeriodStart,
		PeriodEnd:       parsed.PeriodEnd,
		OpeningBalance:  parsed.OpeningBalance,
		ClosingBalance:  parsed.ClosingBalance,
		Status:          "Imported",
		ImportedFrom:    filePath,
		ImportMethod:    parseOutcome.ImportMethod,
		OCRConfidence:   parseOutcome.OCRConfidence,
		TotalDebits:     parsed.TotalDebits,
		TotalCredits:    parsed.TotalCredits,
		DebitCount:      parsed.DebitCount,
		CreditCount:     parsed.CreditCount,
	}

	// Step 4: Create statement lines
	var lines []BankStatementLine
	for _, pl := range parsed.Lines {
		line := BankStatementLine{
			Base:            Base{ID: uuid.New().String()},
			BankStatementID: statement.ID,
			LineNumber:      pl.LineNumber,
			TransactionDate: pl.Date,
			ValueDate:       pl.ValueDate,
			Reference:       pl.Reference,
			Description:     pl.Description,
			Debit:           pl.Debit,
			Credit:          pl.Credit,
			Balance:         pl.Balance,
			IsMatched:       false,
			MatchType:       "Unmatched",
		}
		lines = append(lines, line)
	}
	statement.Lines = lines

	// Step 5: Save to database
	if err := a.db.Create(statement).Error; err != nil {
		return nil, fmt.Errorf("failed to save statement: %w", err)
	}

	// Step 6: Archive the original PDF
	a.ArchiveStatementPDF(statement.ID, filePath)

	log.Printf("📊 Imported PDF statement: %s with %d lines (Opening: %.3f, Closing: %.3f)",
		statementNumber, len(lines), parsed.OpeningBalance, parsed.ClosingBalance)

	return statement, nil
}

// =============================================================================
// CSV IMPORT
// =============================================================================

// ImportBankStatementCSV imports a bank statement from CSV file
func (a *App) ImportBankStatementCSV(filePath string, bankAccountID string) (*BankStatement, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Verify bank account exists
	var bankAccount CompanyBankAccount
	if err := a.db.First(&bankAccount, "id = ?", bankAccountID).Error; err != nil {
		return nil, fmt.Errorf("bank account not found: %w", err)
	}

	// Open and parse CSV
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // Allow variable number of fields per row (bank CSV metadata)
	reader.LazyQuotes = true    // Handle irregular quoting in bank CSVs
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file has no data rows")
	}

	// Parse the CSV content
	parsed, err := parseCSVRecords(records)
	if err != nil {
		return nil, fmt.Errorf("failed to parse CSV: %w", err)
	}

	// Generate statement number
	statementNumber := fmt.Sprintf("%s-%s", bankAccount.AccountNumber, parsed.PeriodEnd.Format("200601"))

	// Create statement
	statement := &BankStatement{
		Base:            Base{ID: uuid.New().String()},
		BankAccountID:   bankAccountID,
		StatementNumber: statementNumber,
		StatementDate:   time.Now(),
		PeriodStart:     parsed.PeriodStart,
		PeriodEnd:       parsed.PeriodEnd,
		OpeningBalance:  parsed.OpeningBalance,
		ClosingBalance:  parsed.ClosingBalance,
		Status:          "Imported",
		ImportedFrom:    filePath,
		TotalDebits:     parsed.TotalDebits,
		TotalCredits:    parsed.TotalCredits,
		DebitCount:      parsed.DebitCount,
		CreditCount:     parsed.CreditCount,
	}

	// Create lines
	var lines []BankStatementLine
	for _, pl := range parsed.Lines {
		line := BankStatementLine{
			Base:            Base{ID: uuid.New().String()},
			BankStatementID: statement.ID,
			LineNumber:      pl.LineNumber,
			TransactionDate: pl.Date,
			ValueDate:       pl.ValueDate,
			Reference:       pl.Reference,
			Description:     pl.Description,
			Debit:           pl.Debit,
			Credit:          pl.Credit,
			Balance:         pl.Balance,
			IsMatched:       false,
			MatchType:       "Unmatched",
		}
		lines = append(lines, line)
	}
	statement.Lines = lines

	// Save to database
	if err := a.db.Create(statement).Error; err != nil {
		return nil, fmt.Errorf("failed to save statement: %w", err)
	}

	log.Printf("📊 Imported CSV statement: %s with %d lines", statementNumber, len(lines))
	return statement, nil
}

// =============================================================================
// NBB FORMAT PARSER (National Bank of Bahrain)
// =============================================================================

// parseNBBFormat parses NBB bank statement format from OCR text
func parseNBBFormat(text string) (*parsedStatement, error) {
	result := &parsedStatement{
		Currency: "BHD",
	}

	lines := strings.Split(text, "\n")

	// Extract header information
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Account Number
		if strings.Contains(line, "Account Number") || strings.Contains(line, "Account No") {
			result.AccountNumber = extractNumber(line)
		}

		// IBAN
		if strings.Contains(line, "IBAN") {
			ibanMatch := regexp.MustCompile(`BH\d{2}[A-Z]{4}\d{14}`).FindString(line)
			if ibanMatch != "" {
				result.IBAN = ibanMatch
			}
		}

		// Opening Balance
		if strings.Contains(line, "Opening Balance") {
			result.OpeningBalance = extractAmount(line)
		}

		// Closing Balance
		if strings.Contains(line, "Closing Balance") {
			result.ClosingBalance = extractAmount(line)
		}

		// Statement Period
		if strings.Contains(line, "Statement") && (strings.Contains(line, "To") || strings.Contains(line, "to")) {
			result.PeriodStart, result.PeriodEnd = extractPeriod(line)
		}
	}

	// Parse transaction lines
	transactionPattern := regexp.MustCompile(`(\d{2}/\d{2}/\d{4})\s+(\d+)\s+(.+?)\s+([\d,]+\.\d{3})?\s*([\d,]+\.\d{3})?\s+([\d,]+\.\d{3})`)

	lineNum := 0
	for _, line := range lines {
		matches := transactionPattern.FindStringSubmatch(line)
		if len(matches) >= 7 {
			lineNum++
			date, _ := time.Parse("02/01/2006", matches[1])

			pl := parsedLine{
				LineNumber:  lineNum,
				Date:        date,
				ValueDate:   date,
				Reference:   matches[2],
				Description: strings.TrimSpace(matches[3]),
				Debit:       parseAmount(matches[4]),
				Credit:      parseAmount(matches[5]),
				Balance:     parseAmount(matches[6]),
			}

			if pl.Debit > 0 {
				result.DebitCount++
				result.TotalDebits += pl.Debit
			}
			if pl.Credit > 0 {
				result.CreditCount++
				result.TotalCredits += pl.Credit
			}

			result.Lines = append(result.Lines, pl)
		}
	}

	// If no transactions found, return error
	if len(result.Lines) == 0 {
		return nil, fmt.Errorf("no transactions found in NBB format")
	}

	// Infer period from transactions if not found in header
	if result.PeriodStart.IsZero() && len(result.Lines) > 0 {
		result.PeriodStart = result.Lines[0].Date
		result.PeriodEnd = result.Lines[len(result.Lines)-1].Date
	}

	return result, nil
}

// =============================================================================
// GENERIC FORMAT PARSER
// =============================================================================

// parseGenericBankFormat attempts to parse any bank statement format
func parseGenericBankFormat(text string) (*parsedStatement, error) {
	result := &parsedStatement{
		Currency: "BHD",
	}

	lines := strings.Split(text, "\n")

	// Try to find dates, amounts, and descriptions
	datePattern := regexp.MustCompile(`\d{1,2}[/-]\d{1,2}[/-]\d{2,4}`)
	amountPattern := regexp.MustCompile(`[\d,]+\.\d{2,3}`)

	lineNum := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) < 10 {
			continue
		}

		dates := datePattern.FindAllString(line, -1)
		amounts := amountPattern.FindAllString(line, -1)

		if len(dates) > 0 && len(amounts) > 0 {
			lineNum++
			date := parseFlexibleDate(dates[0])

			var debit, credit, balance float64
			switch len(amounts) {
			case 1:
				balance = parseAmount(amounts[0])
			case 2:
				credit = parseAmount(amounts[0])
				balance = parseAmount(amounts[1])
			case 3:
				debit = parseAmount(amounts[0])
				credit = parseAmount(amounts[1])
				balance = parseAmount(amounts[2])
			}

			// Extract description (text between date and amounts)
			desc := line
			for _, d := range dates {
				desc = strings.Replace(desc, d, "", 1)
			}
			for _, a := range amounts {
				desc = strings.Replace(desc, a, "", 1)
			}
			desc = strings.TrimSpace(desc)

			pl := parsedLine{
				LineNumber:  lineNum,
				Date:        date,
				ValueDate:   date,
				Description: desc,
				Debit:       debit,
				Credit:      credit,
				Balance:     balance,
			}

			if pl.Debit > 0 {
				result.DebitCount++
				result.TotalDebits += pl.Debit
			}
			if pl.Credit > 0 {
				result.CreditCount++
				result.TotalCredits += pl.Credit
			}

			result.Lines = append(result.Lines, pl)
		}
	}

	if len(result.Lines) == 0 {
		return nil, fmt.Errorf("no transactions found in generic format")
	}

	// Infer balances and period from lines
	result.PeriodStart = result.Lines[0].Date
	result.PeriodEnd = result.Lines[len(result.Lines)-1].Date
	result.OpeningBalance = result.Lines[0].Balance + result.Lines[0].Debit - result.Lines[0].Credit
	result.ClosingBalance = result.Lines[len(result.Lines)-1].Balance

	return result, nil
}

// =============================================================================
// CSV PARSER
// =============================================================================

// parseCSVRecords parses CSV records into a statement structure
// Supports multiple formats:
// 1. Al Salam format: Header metadata, then Posting Date, Value Date, FT Reference, Description, Amount, Balance
// 2. Generic format: Date, Description, Debit, Credit, Reference, Balance
func parseCSVRecords(records [][]string) (*parsedStatement, error) {
	result := &parsedStatement{
		Currency: "BHD",
	}

	// Try to detect Al Salam format (has metadata rows at top)
	if isAlSalamFormat(records) {
		return parseAlSalamCSV(records)
	}

	// Generic CSV format: Date, Description, Debit, Credit, Reference (optional), Balance (optional)
	var openingBalance, runningBalance float64

	for i, record := range records[1:] { // Skip header row
		if len(record) < 4 {
			continue
		}

		date := parseFlexibleDate(strings.TrimSpace(record[0]))
		description := strings.TrimSpace(record[1])
		debit := parseAmount(record[2])
		credit := parseAmount(record[3])

		reference := ""
		if len(record) > 4 {
			reference = strings.TrimSpace(record[4])
		}

		var balance float64
		if len(record) > 5 {
			balance = parseAmount(record[5])
		} else {
			runningBalance = runningBalance + credit - debit
			balance = runningBalance
		}

		if i == 0 {
			openingBalance = balance - credit + debit
		}

		pl := parsedLine{
			LineNumber:  i + 1,
			Date:        date,
			ValueDate:   date,
			Reference:   reference,
			Description: description,
			Debit:       debit,
			Credit:      credit,
			Balance:     balance,
		}

		if pl.Debit > 0 {
			result.DebitCount++
			result.TotalDebits += pl.Debit
		}
		if pl.Credit > 0 {
			result.CreditCount++
			result.TotalCredits += pl.Credit
		}

		result.Lines = append(result.Lines, pl)
	}

	if len(result.Lines) == 0 {
		return nil, fmt.Errorf("no valid transaction rows in CSV")
	}

	result.PeriodStart = result.Lines[0].Date
	result.PeriodEnd = result.Lines[len(result.Lines)-1].Date
	result.OpeningBalance = openingBalance
	result.ClosingBalance = result.Lines[len(result.Lines)-1].Balance

	return result, nil
}

// isAlSalamFormat detects Al Salam Bank CSV format
func isAlSalamFormat(records [][]string) bool {
	// Check for metadata rows like "Statement of Account", "Client Name", "IBAN"
	for i := 0; i < len(records) && i < 10; i++ {
		if len(records[i]) > 0 {
			firstCell := strings.TrimSpace(records[i][0])
			if strings.Contains(firstCell, "Statement of Account") ||
				strings.Contains(firstCell, "Client Name") ||
				strings.Contains(firstCell, "IBAN") {
				return true
			}
		}
	}
	return false
}

// parseAlSalamCSV parses Al Salam Bank CSV format
// Format: Posting Date, Value Date, FT Reference, Description, Amount, Balance
// Amount uses (-) prefix for debits
func parseAlSalamCSV(records [][]string) (*parsedStatement, error) {
	result := &parsedStatement{
		Currency: "BHD",
	}

	// Extract metadata from header rows
	for _, record := range records {
		if len(record) < 2 {
			continue
		}
		key := strings.TrimSpace(record[0])
		value := strings.TrimSpace(record[1])

		switch key {
		case "From":
			result.PeriodStart = parseFlexibleDate(value)
		case "To":
			result.PeriodEnd = parseFlexibleDate(value)
		case "IBAN":
			result.IBAN = value
		case "Account Currency":
			if strings.Contains(strings.ToLower(value), "euro") {
				result.Currency = "EUR"
			} else if strings.Contains(strings.ToLower(value), "usd") || strings.Contains(strings.ToLower(value), "dollar") {
				result.Currency = "USD"
			}
		case "Total Credits":
			result.TotalCredits = parseAmount(value)
		case "Total Debits":
			result.TotalDebits = parseAmount(value)
		}
	}

	// Find the header row (contains "Posting Date")
	headerRowIdx := -1
	for i, record := range records {
		if len(record) > 0 && strings.TrimSpace(record[0]) == "Posting Date" {
			headerRowIdx = i
			break
		}
	}

	if headerRowIdx == -1 {
		return nil, fmt.Errorf("could not find header row in Al Salam CSV")
	}

	// Parse transaction rows
	lineNum := 0
	for i := headerRowIdx + 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 5 {
			continue
		}

		// Skip empty rows
		postingDate := strings.TrimSpace(record[0])
		if postingDate == "" {
			// Check for Opening/Closing Balance rows
			desc := ""
			if len(record) > 3 {
				desc = strings.TrimSpace(record[3])
			}
			if strings.Contains(desc, "Opening Balance") {
				if len(record) > 5 {
					result.OpeningBalance = parseAmount(record[5])
				}
			} else if strings.Contains(desc, "Closing Balance") {
				if len(record) > 5 {
					result.ClosingBalance = parseAmount(record[5])
				}
			}
			continue
		}

		lineNum++
		date := parseFlexibleDate(postingDate)
		valueDate := parseFlexibleDate(strings.TrimSpace(record[1]))
		reference := strings.TrimSpace(record[2])
		description := strings.TrimSpace(record[3])
		amountStr := strings.TrimSpace(record[4])
		balanceStr := ""
		if len(record) > 5 {
			balanceStr = strings.TrimSpace(record[5])
		}

		// Parse amount - (-) prefix means debit
		var debit, credit float64
		if strings.HasPrefix(amountStr, "(-)") {
			debit = parseAmount(strings.TrimPrefix(amountStr, "(-)"))
			result.DebitCount++
		} else if strings.HasPrefix(amountStr, "-") {
			debit = parseAmount(strings.TrimPrefix(amountStr, "-"))
			result.DebitCount++
		} else if amountStr != "" {
			credit = parseAmount(amountStr)
			if credit > 0 {
				result.CreditCount++
			}
		}

		balance := parseAmount(balanceStr)

		pl := parsedLine{
			LineNumber:  lineNum,
			Date:        date,
			ValueDate:   valueDate,
			Reference:   reference,
			Description: description,
			Debit:       debit,
			Credit:      credit,
			Balance:     balance,
		}

		result.Lines = append(result.Lines, pl)
	}

	if len(result.Lines) == 0 {
		return nil, fmt.Errorf("no transaction rows found in Al Salam CSV")
	}

	// Infer opening/closing if not found
	if result.OpeningBalance == 0 && len(result.Lines) > 0 {
		first := result.Lines[0]
		result.OpeningBalance = first.Balance + first.Debit - first.Credit
	}
	if result.ClosingBalance == 0 && len(result.Lines) > 0 {
		result.ClosingBalance = result.Lines[len(result.Lines)-1].Balance
	}

	log.Printf("📊 Parsed Al Salam CSV: %d lines, Opening: %.3f, Closing: %.3f",
		len(result.Lines), result.OpeningBalance, result.ClosingBalance)

	return result, nil
}

// =============================================================================
// EXTRACTION HELPERS
// =============================================================================

// extractNumber extracts a numeric string from a line
func extractNumber(line string) string {
	re := regexp.MustCompile(`\d+`)
	return re.FindString(line)
}

// extractAmount extracts a BHD amount (3 decimal places) from a line
func extractAmount(line string) float64 {
	re := regexp.MustCompile(`[\d,]+\.\d{3}`)
	match := re.FindString(line)
	return parseAmount(match)
}

// parseAmount converts a string amount to float64
func parseAmount(s string) float64 {
	if s == "" {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	amount, _ := strconv.ParseFloat(s, 64)
	return amount
}

// extractPeriod extracts start and end dates from a period string
func extractPeriod(line string) (start, end time.Time) {
	datePattern := regexp.MustCompile(`\d{1,2}[/-]\d{1,2}[/-]\d{2,4}`)
	dates := datePattern.FindAllString(line, -1)

	if len(dates) >= 2 {
		start = parseFlexibleDate(dates[0])
		end = parseFlexibleDate(dates[1])
	} else if len(dates) == 1 {
		end = parseFlexibleDate(dates[0])
		start = time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, time.UTC)
	}

	return
}

// parseFlexibleDate parses dates in various formats
func parseFlexibleDate(s string) time.Time {
	formats := []string{
		"02/01/2006", "2/1/2006",
		"02-01-2006", "2-1-2006",
		"2006-01-02", "2006/01/02",
		"01/02/2006", "1/2/2006",
		"02/01/06", "2/1/06",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			// Handle 2-digit years
			if t.Year() < 100 {
				t = t.AddDate(2000, 0, 0)
			}
			return t
		}
	}

	return time.Time{}
}
