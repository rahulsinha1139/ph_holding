// =============================================================================
// BANK RECONCILIATION SERVICE
//
// MISSION: Core service for bank statement reconciliation as SSOT
// FEATURES: Statement management, matching, cash position, validation
// =============================================================================

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// =============================================================================
// STATEMENT MANAGEMENT
// =============================================================================

// GetBankStatements retrieves all statements for a bank account
func (a *App) GetBankStatements(bankAccountID string) ([]BankStatement, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var statements []BankStatement
	query := a.db.Order("statement_date DESC")
	if bankAccountID != "" {
		query = query.Where("bank_account_id = ?", bankAccountID)
	}

	if err := query.Find(&statements).Error; err != nil {
		log.Printf("GetBankStatements error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	return statements, nil
}

// GetBankStatementByID retrieves a single statement with all lines
func (a *App) GetBankStatementByID(id string) (*BankStatement, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.Preload("Lines").First(&statement, "id = ?", id).Error; err != nil {
		log.Printf("GetBankStatementByID error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	return &statement, nil
}

// CreateBankStatement creates a new bank statement
func (a *App) CreateBankStatement(statement *BankStatement) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if strings.TrimSpace(statement.Status) == "" {
		statement.Status = bankStatementStatusImported
	} else {
		normalizedStatus, err := normalizeBankStatementStatus(statement.Status)
		if err != nil {
			return err
		}
		statement.Status = normalizedStatus
	}
	if strings.TrimSpace(statement.Division) == "" {
		statement.Division = a.resolveBankAccountDivision(statement.BankAccountID)
	} else {
		statement.Division = normalizeDivisionName(statement.Division)
	}

	if err := a.db.Create(statement).Error; err != nil {
		log.Printf("CreateBankStatement error: %v", err)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	log.Printf("📊 Bank statement created: %s (Period: %s to %s)",
		statement.StatementNumber,
		statement.PeriodStart.Format("2006-01-02"),
		statement.PeriodEnd.Format("2006-01-02"))

	return nil
}

// UpdateBankStatement updates statement fields
func (a *App) UpdateBankStatement(id string, updates map[string]interface{}) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	statement, err := a.getBankStatementByIDForWorkflow(id)
	if err != nil {
		return err
	}
	if err := ensureBankStatementMutable(*statement, "edit this statement"); err != nil {
		return err
	}
	if err := normalizeBankStatementUpdates(updates); err != nil {
		return err
	}
	if bankAccountID, ok := updates["bank_account_id"].(string); ok && strings.TrimSpace(bankAccountID) != "" {
		updates["division"] = a.resolveBankAccountDivision(bankAccountID)
	}
	if division, ok := updates["division"].(string); ok && strings.TrimSpace(division) != "" {
		updates["division"] = normalizeDivisionName(division)
	}

	result := a.db.Model(&BankStatement{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		log.Printf("UpdateBankStatement error: %v", result.Error)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("statement not found: %s", id)
	}

	return nil
}

// DeleteBankStatement soft-deletes a statement
func (a *App) DeleteBankStatement(id string) error {
	if ok, err := a.guardDeleteOrRequest("finance:delete", "bank_statement", id, "Bank statement"); !ok {
		return err
	}
	if err := a.requirePermission("finance:delete"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	statement, err := a.getBankStatementByIDForWorkflow(id)
	if err != nil {
		return err
	}
	if err := ensureBankStatementMutable(*statement, "delete this statement"); err != nil {
		return err
	}

	result := a.db.Delete(&BankStatement{}, "id = ?", id)
	if result.Error != nil {
		log.Printf("DeleteBankStatement error: %v", result.Error)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	log.Printf("🗑️ Bank statement deleted: %s", id)
	return nil
}

// =============================================================================
// STATEMENT LINES
// =============================================================================

// GetBankStatementLines retrieves all lines for a statement
func (a *App) GetBankStatementLines(statementID string) ([]BankStatementLine, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var lines []BankStatementLine
	if err := a.db.Where("bank_statement_id = ?", statementID).
		Order("line_number ASC").
		Find(&lines).Error; err != nil {
		log.Printf("GetBankStatementLines error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	return lines, nil
}

// GetUnmatchedLines retrieves only unmatched lines for a statement
func (a *App) GetUnmatchedLines(statementID string) ([]BankStatementLine, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var lines []BankStatementLine
	if err := a.db.Where("bank_statement_id = ? AND is_matched = ?", statementID, false).
		Order("line_number ASC").
		Find(&lines).Error; err != nil {
		log.Printf("GetUnmatchedLines error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	return lines, nil
}

// UpdateBankStatementLine updates a statement line
func (a *App) UpdateBankStatementLine(lineID string, updates map[string]interface{}) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, statement, err := a.getBankStatementForLine(lineID)
	if err != nil {
		return err
	}
	if isFinalBankStatementStatus(statement.Status) {
		var line BankStatementLine
		if err := a.db.First(&line, "id = ?", lineID).Error; err != nil {
			return fmt.Errorf("line not found: %w", err)
		}
		if statement.Status != bankStatementStatusReconciled || !line.IsMatched {
			if err := ensureBankStatementMutable(*statement, "edit statement lines"); err != nil {
				return err
			}
		}
	}

	var originalLine BankStatementLine
	editedMatchedLine := false

	err = a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&originalLine, "id = ?", lineID).Error; err != nil {
			return fmt.Errorf("line not found: %w", err)
		}

		sanitizedUpdates, err := sanitizeBankStatementLineUpdates(originalLine, updates)
		if err != nil {
			return err
		}
		if len(sanitizedUpdates) == 0 {
			return nil
		}

		if originalLine.IsMatched {
			if err := clearBankStatementLineMatchTx(tx, lineID); err != nil {
				return err
			}
			editedMatchedLine = true
		}

		if err := tx.Model(&BankStatementLine{}).Where("id = ?", lineID).Updates(sanitizedUpdates).Error; err != nil {
			return fmt.Errorf("failed to update line: %w", err)
		}

		return refreshBankStatementRollupTx(tx, originalLine.BankStatementID)
	})
	if err != nil {
		log.Printf("UpdateBankStatementLine error: %v", err)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	if editedMatchedLine {
		previousMatch := originalLine.MatchedInvoiceIDs
		if previousMatch == "" {
			previousMatch = originalLine.MatchedPaymentID
		}
		a.LogReconciliationAction(originalLine.BankStatementID, &lineID, "UNMATCH",
			map[string]interface{}{
				"previous_match": previousMatch,
				"reason":         "Line edited during OCR review",
			},
			a.getCurrentUserID(), false, 1.0, "Line edited during OCR review")
	}

	return nil
}

// CreateBankStatementLine adds a new line to a statement
func (a *App) CreateBankStatementLine(statementID string, line map[string]interface{}) (*BankStatementLine, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	statement, err := a.getBankStatementByIDForWorkflow(statementID)
	if err != nil {
		return nil, err
	}
	if err := ensureBankStatementMutable(*statement, "create statement lines"); err != nil {
		return nil, err
	}

	var newLine BankStatementLine
	err = a.db.Transaction(func(tx *gorm.DB) error {
		var maxLineNum int
		if err := tx.Model(&BankStatementLine{}).Where("bank_statement_id = ?", statementID).
			Select("COALESCE(MAX(line_number), 0)").Scan(&maxLineNum).Error; err != nil {
			return fmt.Errorf("failed to determine next line number: %w", err)
		}

		debit, err := parseBankStatementAmountValue(line["debit"])
		if err != nil {
			return err
		}
		credit, err := parseBankStatementAmountValue(line["credit"])
		if err != nil {
			return err
		}
		balance, err := parseBankStatementAmountValue(line["balance"])
		if err != nil {
			return err
		}
		if err := validateBankStatementAmounts(debit, credit); err != nil {
			return err
		}

		transactionDate, err := parseBankStatementDateValue(line["transaction_date"])
		if err != nil {
			return err
		}
		description := strings.TrimSpace(fmt.Sprintf("%v", line["description"]))
		if description == "" {
			return fmt.Errorf("description is required")
		}

		newLine = BankStatementLine{
			Base:            Base{ID: uuid.New().String()},
			BankStatementID: statementID,
			LineNumber:      maxLineNum + 1,
			TransactionDate: transactionDate,
			ValueDate:       transactionDate,
			Description:     description,
			Reference:       strings.TrimSpace(fmt.Sprintf("%v", line["reference"])),
			Debit:           debit,
			Credit:          credit,
			Balance:         balance,
			MatchType:       "Unmatched",
		}

		if err := tx.Create(&newLine).Error; err != nil {
			return fmt.Errorf("failed to create line: %w", err)
		}

		return refreshBankStatementRollupTx(tx, statementID)
	})
	if err != nil {
		return nil, err
	}

	log.Printf("➕ Bank statement line created: %s in statement %s", newLine.ID, statementID)
	return &newLine, nil
}

// DeleteBankStatementLine soft-deletes a statement line
// SECURITY: Audit log before deletion (financial transaction)
func (a *App) DeleteBankStatementLine(lineID string) error {
	if ok, err := a.guardDeleteOrRequest("finance:delete", "bank_statement_line", lineID, "Bank statement line"); !ok {
		return err
	}
	if err := a.requirePermission("finance:delete"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, statement, err := a.getBankStatementForLine(lineID)
	if err != nil {
		return err
	}
	if err := ensureBankStatementMutable(*statement, "delete statement lines"); err != nil {
		return err
	}

	var line BankStatementLine
	err = a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&line, "id = ?", lineID).Error; err != nil {
			return fmt.Errorf("line not found: %w", err)
		}

		if err := tx.Model(&PayrollPayout{}).
			Where("bank_statement_line_id = ?", lineID).
			Updates(map[string]interface{}{
				"bank_statement_line_id": nil,
				"updated_at":             time.Now(),
			}).Error; err != nil {
			return fmt.Errorf("failed to unlink payroll payout from deleted line: %w", err)
		}

		if err := tx.Delete(&BankStatementLine{}, "id = ?", lineID).Error; err != nil {
			return fmt.Errorf("failed to delete line: %w", err)
		}

		return refreshBankStatementRollupTx(tx, line.BankStatementID)
	})
	if err != nil {
		return err
	}

	// SECURITY: Audit log BEFORE deletion
	log.Printf("🔒 AUDIT: Bank statement line %s deleted (Amount: %.3f BHD, Desc: %s) by user %s",
		lineID, line.Debit+line.Credit, line.Description, a.getCurrentUserID())

	if GlobalAuditLogger != nil {
		GlobalAuditLogger.LogFinancialTransaction(
			a.getCurrentUserID(),
			"bank_line_deleted",
			"bank_statement_line",
			lineID,
			line.Debit+line.Credit,
			"BHD",
			true,
			map[string]interface{}{
				"bank_statement_id": line.BankStatementID,
				"description":       line.Description,
				"reference":         line.Reference,
				"debit":             line.Debit,
				"credit":            line.Credit,
			},
		)
	}

	log.Printf("🗑️ Bank statement line deleted: %s", lineID)
	return nil
}

func sanitizeBankStatementLineUpdates(existing BankStatementLine, updates map[string]interface{}) (map[string]interface{}, error) {
	sanitized := make(map[string]interface{})

	for key, value := range updates {
		switch key {
		case "transaction_date":
			transactionDate, err := parseBankStatementDateValue(value)
			if err != nil {
				return nil, err
			}
			sanitized["transaction_date"] = transactionDate
			sanitized["value_date"] = transactionDate
		case "description":
			description := strings.TrimSpace(fmt.Sprintf("%v", value))
			if description == "" {
				return nil, fmt.Errorf("description is required")
			}
			sanitized["description"] = description
		case "reference":
			sanitized["reference"] = strings.TrimSpace(fmt.Sprintf("%v", value))
		case "debit", "credit", "balance":
			amount, err := parseBankStatementAmountValue(value)
			if err != nil {
				return nil, err
			}
			sanitized[key] = amount
		}
	}

	debit := existing.Debit
	if value, ok := sanitized["debit"].(float64); ok {
		debit = value
	}
	credit := existing.Credit
	if value, ok := sanitized["credit"].(float64); ok {
		credit = value
	}

	if err := validateBankStatementAmounts(debit, credit); err != nil {
		return nil, err
	}

	return sanitized, nil
}

func parseBankStatementAmountValue(value interface{}) (float64, error) {
	switch val := value.(type) {
	case nil:
		return 0, nil
	case float64:
		return roundBHD(val), nil
	case float32:
		return roundBHD(float64(val)), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		trimmed := strings.TrimSpace(val)
		if trimmed == "" {
			return 0, nil
		}
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(trimmed, ",", ""), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid amount: %q", val)
		}
		return roundBHD(parsed), nil
	default:
		return 0, fmt.Errorf("invalid amount value")
	}
}

func parseBankStatementDateValue(value interface{}) (time.Time, error) {
	switch val := value.(type) {
	case time.Time:
		if val.IsZero() {
			return time.Time{}, fmt.Errorf("transaction date is required")
		}
		return val, nil
	case string:
		trimmed := strings.TrimSpace(val)
		if trimmed == "" {
			return time.Time{}, fmt.Errorf("transaction date is required")
		}
		parsed := parseDate(trimmed)
		if parsed.IsZero() {
			return time.Time{}, fmt.Errorf("invalid transaction date")
		}
		return parsed, nil
	default:
		return time.Time{}, fmt.Errorf("invalid transaction date")
	}
}

func validateBankStatementAmounts(debit, credit float64) error {
	if debit < 0 || credit < 0 {
		return fmt.Errorf("debit and credit must be non-negative")
	}
	if debit == 0 && credit == 0 {
		return fmt.Errorf("either debit or credit must be non-zero")
	}
	if debit > 0 && credit > 0 {
		return fmt.Errorf("cannot have both debit and credit on the same line")
	}
	return nil
}

func clearBankStatementLineMatchTx(tx *gorm.DB, lineID string) error {
	if err := tx.Where("bank_statement_line_id = ?", lineID).Delete(&BankLinePaymentAllocation{}).Error; err != nil {
		return fmt.Errorf("failed to clear bank line allocations: %w", err)
	}

	if err := tx.Model(&PayrollPayout{}).
		Where("bank_statement_line_id = ?", lineID).
		Updates(map[string]interface{}{
			"bank_statement_line_id": nil,
			"updated_at":             time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to unlink payroll payout from bank line: %w", err)
	}

	return tx.Model(&BankStatementLine{}).Where("id = ?", lineID).Updates(map[string]interface{}{
		"is_matched":          false,
		"match_type":          "Unmatched",
		"match_confidence":    0,
		"matched_invoice_ids": "",
		"matched_payment_id":  "",
		"matched_expense_id":  nil,
		"matched_journal_id":  "",
	}).Error
}

func refreshBankStatementRollupTx(tx *gorm.DB, statementID string) error {
	var statement BankStatement
	if err := tx.First(&statement, "id = ?", statementID).Error; err != nil {
		return fmt.Errorf("statement not found: %w", err)
	}

	var rollup struct {
		TotalDebits  float64
		TotalCredits float64
		DebitCount   int64
		CreditCount  int64
	}

	if err := tx.Model(&BankStatementLine{}).
		Select(`
			COALESCE(SUM(debit), 0) as total_debits,
			COALESCE(SUM(credit), 0) as total_credits,
			COALESCE(SUM(CASE WHEN debit > 0 THEN 1 ELSE 0 END), 0) as debit_count,
			COALESCE(SUM(CASE WHEN credit > 0 THEN 1 ELSE 0 END), 0) as credit_count
		`).
		Where("bank_statement_id = ?", statementID).
		Scan(&rollup).Error; err != nil {
		return fmt.Errorf("failed to calculate statement totals: %w", err)
	}

	expectedClosing := roundBHD(statement.OpeningBalance + rollup.TotalCredits - rollup.TotalDebits)
	discrepancy := roundBHD(math.Abs(expectedClosing - statement.ClosingBalance))
	isValid := discrepancy <= 0.001

	updateMap := map[string]interface{}{
		"total_debits":       roundBHD(rollup.TotalDebits),
		"total_credits":      roundBHD(rollup.TotalCredits),
		"debit_count":        int(rollup.DebitCount),
		"credit_count":       int(rollup.CreditCount),
		"balance_verified":   isValid,
		"discrepancy_amount": discrepancy,
	}
	if statement.Status == "Reconciled" || statement.Status == "Verified" {
		updateMap["status"] = "InProgress"
		updateMap["reconciled_at"] = nil
		updateMap["reconciled_by"] = ""
	}

	if err := tx.Model(&BankStatement{}).Where("id = ?", statementID).Updates(updateMap).Error; err != nil {
		return fmt.Errorf("failed to update statement totals: %w", err)
	}

	return nil
}

// =============================================================================
// CASH POSITION SSOT
// =============================================================================

type CashPositionAccount struct {
	BankAccountID        string    `json:"bank_account_id"`
	Division             string    `json:"division"`
	BankName             string    `json:"bank_name"`
	AccountName          string    `json:"account_name"`
	AccountNumber        string    `json:"account_number"`
	Currency             string    `json:"currency"`
	CurrentBalance       float64   `json:"current_balance"`
	CurrentBalanceBHD    float64   `json:"current_balance_bhd"`
	LastStatementDate    time.Time `json:"last_statement_date"`
	LastStatementNumber  string    `json:"last_statement_number"`
	HasStatement         bool      `json:"has_statement"`
	IsCurrentStatement   bool      `json:"is_current_statement"`
	StatementPeriodLabel string    `json:"statement_period_label"`
	Notice               string    `json:"notice"`
}

type CashPositionSnapshot struct {
	Accounts              []CashPositionAccount `json:"accounts"`
	TotalBHD              float64               `json:"total_bhd"`
	TotalEUR              float64               `json:"total_eur"`
	CashBalanceBHD        float64               `json:"cash_balance_bhd"`
	LatestStatementPeriod string                `json:"latest_statement_period"`
	Notices               []string              `json:"notices"`
	StaleAccounts         []string              `json:"stale_accounts"`
	AsOf                  time.Time             `json:"as_of"`
}

// GetCashPosition returns real-time cash position for all bank accounts
func (a *App) GetCashPosition() (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	snapshot, err := a.computeCashPositionSnapshot()
	if err != nil {
		return nil, fmt.Errorf("failed to get cash position: %w", err)
	}

	return map[string]interface{}{
		"accounts":                snapshot.Accounts,
		"total_bhd":               snapshot.TotalBHD,
		"total_eur":               snapshot.TotalEUR,
		"cash_balance_bhd":        snapshot.CashBalanceBHD,
		"latest_statement_period": snapshot.LatestStatementPeriod,
		"notices":                 snapshot.Notices,
		"stale_accounts":          snapshot.StaleAccounts,
		"as_of":                   snapshot.AsOf,
	}, nil
}

func (a *App) computeCashPositionSnapshot() (*CashPositionSnapshot, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var accounts []CompanyBankAccount
	if err := a.db.Where("is_active = ?", true).
		Order("division ASC, display_order ASC, bank_name ASC").
		Find(&accounts).Error; err != nil {
		return nil, err
	}

	statuses := []string{"Imported", "InProgress", "In Progress", "Reconciled", "Verified"}
	positions := make([]CashPositionAccount, 0, len(accounts))
	latestByDivision := make(map[string]time.Time)
	var latestOverall time.Time

	for _, account := range accounts {
		division := normalizeDivisionName(account.Division)
		if division == "" {
			division = "PH Trading"
		}

		position := CashPositionAccount{
			BankAccountID: account.ID,
			Division:      division,
			BankName:      account.BankName,
			AccountName:   account.AccountName,
			AccountNumber: account.AccountNumber,
			Currency:      strings.ToUpper(strings.TrimSpace(account.Currency)),
		}
		if position.Currency == "" {
			position.Currency = "BHD"
		}

		var statement BankStatement
		err := a.db.Where("bank_account_id = ? AND status IN ?", account.ID, statuses).
			Order("period_end DESC, updated_at DESC, created_at DESC").
			First(&statement).Error
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, err
			}
			position.Notice = fmt.Sprintf("No statement imported for %s.", formatCashAccountName(position))
			positions = append(positions, position)
			continue
		}

		rate := normalizeExchangeRateToBHD(position.Currency, account.BookingRate)
		position.CurrentBalance = roundBHD(statement.ClosingBalance)
		position.CurrentBalanceBHD = roundBHD(statement.ClosingBalance * rate)
		position.LastStatementDate = statement.PeriodEnd
		position.LastStatementNumber = statement.StatementNumber
		position.HasStatement = true
		position.StatementPeriodLabel = formatCashStatementMonth(statement.PeriodEnd)
		positions = append(positions, position)

		if statement.PeriodEnd.After(latestByDivision[division]) {
			latestByDivision[division] = statement.PeriodEnd
		}
		if statement.PeriodEnd.After(latestOverall) {
			latestOverall = statement.PeriodEnd
		}
	}

	snapshot := &CashPositionSnapshot{
		Accounts:              positions,
		LatestStatementPeriod: formatCashStatementMonth(latestOverall),
		AsOf:                  time.Now(),
	}

	for i := range snapshot.Accounts {
		account := &snapshot.Accounts[i]
		if !account.HasStatement {
			snapshot.Notices = append(snapshot.Notices, account.Notice)
			snapshot.StaleAccounts = append(snapshot.StaleAccounts, formatCashAccountName(*account))
			continue
		}

		latestForDivision := latestByDivision[account.Division]
		account.IsCurrentStatement = sameCashStatementMonth(account.LastStatementDate, latestForDivision)
		if !account.IsCurrentStatement {
			account.Notice = fmt.Sprintf("%s latest statement is %s; latest imported month for %s is %s.",
				formatCashAccountName(*account),
				formatCashStatementMonth(account.LastStatementDate),
				account.Division,
				formatCashStatementMonth(latestForDivision),
			)
			snapshot.Notices = append(snapshot.Notices, account.Notice)
			snapshot.StaleAccounts = append(snapshot.StaleAccounts, formatCashAccountName(*account))
		}

		if account.Currency == "EUR" {
			snapshot.TotalEUR = roundBHD(snapshot.TotalEUR + account.CurrentBalance)
		}
		snapshot.TotalBHD = roundBHD(snapshot.TotalBHD + account.CurrentBalanceBHD)
	}
	snapshot.CashBalanceBHD = snapshot.TotalBHD

	return snapshot, nil
}

func sameCashStatementMonth(a, b time.Time) bool {
	if a.IsZero() || b.IsZero() {
		return false
	}
	return a.Year() == b.Year() && a.Month() == b.Month()
}

func formatCashStatementMonth(value time.Time) string {
	if value.IsZero() {
		return "no statement"
	}
	return value.Format("Jan 2006")
}

func formatCashAccountName(account CashPositionAccount) string {
	parts := []string{strings.TrimSpace(account.BankName)}
	if strings.TrimSpace(account.AccountName) != "" {
		parts = append(parts, strings.TrimSpace(account.AccountName))
	}
	if strings.TrimSpace(account.AccountNumber) != "" {
		parts = append(parts, strings.TrimSpace(account.AccountNumber))
	}
	name := strings.Join(parts, " ")
	if name == "" {
		return "Bank account"
	}
	return name
}

// GetCashPositionByAccount returns cash position for a specific account
func (a *App) GetCashPositionByAccount(bankAccountID string) (float64, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return 0, err
	}
	if a.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var balance float64
	err := a.db.Raw(`
		SELECT COALESCE(closing_balance, 0)
		FROM bank_statements
		WHERE bank_account_id = ? AND status IN ('Imported', 'InProgress', 'In Progress', 'Reconciled', 'Verified')
		ORDER BY period_end DESC
		LIMIT 1
	`, bankAccountID).Scan(&balance).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get account balance: %w", err)
	}

	return balance, nil
}

// =============================================================================
// RECONCILIATION WORKFLOW
// =============================================================================

// ValidateStatementBalance checks if Opening + Credits - Debits = Closing
func (a *App) ValidateStatementBalance(statementID string) (bool, float64, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return false, 0, err
	}
	if a.db == nil {
		return false, 0, fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return false, 0, fmt.Errorf("statement not found: %w", err)
	}

	// Sum credits and debits from lines
	var totals struct {
		TotalDebits  float64
		TotalCredits float64
	}

	a.db.Model(&BankStatementLine{}).
		Select("COALESCE(SUM(debit), 0) as total_debits, COALESCE(SUM(credit), 0) as total_credits").
		Where("bank_statement_id = ?", statementID).
		Scan(&totals)

	// Calculate expected closing balance with proper BHD precision
	expectedClosing := roundBHD(statement.OpeningBalance + totals.TotalCredits - totals.TotalDebits)
	discrepancy := roundBHD(math.Abs(expectedClosing - statement.ClosingBalance))

	// Allow tolerance of 0.001 BHD for rounding
	isValid := discrepancy <= 0.001

	// Update statement verification status
	a.db.Model(&statement).Updates(map[string]interface{}{
		"balance_verified":   isValid,
		"discrepancy_amount": discrepancy,
		"total_debits":       totals.TotalDebits,
		"total_credits":      totals.TotalCredits,
		"debit_count":        0, // Will be updated by trigger or separate query
		"credit_count":       0,
	})

	if isValid {
		log.Printf("✅ Statement %s balance verified: Opening %.3f + Credits %.3f - Debits %.3f = Closing %.3f",
			statementID, statement.OpeningBalance, totals.TotalCredits, totals.TotalDebits, statement.ClosingBalance)
	} else {
		log.Printf("❌ Statement %s balance mismatch: Expected %.3f, Got %.3f (Discrepancy: %.3f)",
			statementID, expectedClosing, statement.ClosingBalance, discrepancy)
	}

	return isValid, discrepancy, nil
}

// FinalizeReconciliation marks a statement as reconciled
func (a *App) FinalizeReconciliation(statementID string, reconciledBy string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return fmt.Errorf("statement not found: %w", err)
	}

	if isFinalBankStatementStatus(statement.Status) {
		return fmt.Errorf("statement already reconciled")
	}

	// Validate balance before finalizing
	isValid, discrepancy, err := a.ValidateStatementBalance(statementID)
	if err != nil {
		return fmt.Errorf("balance validation failed: %w", err)
	}

	if !isValid {
		return fmt.Errorf("cannot finalize: balance discrepancy of %.3f BHD", discrepancy)
	}

	// Check for unmatched lines
	var unmatchedCount int64
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ? AND is_matched = ?", statementID, false).Count(&unmatchedCount)

	if unmatchedCount > 0 {
		return fmt.Errorf("cannot finalize: %d unmatched transactions", unmatchedCount)
	}

	now := time.Now()
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&statement).Updates(map[string]interface{}{
			"status":        bankStatementStatusReconciled,
			"reconciled_at": now,
			"reconciled_by": reconciledBy,
		}).Error; err != nil {
			return fmt.Errorf("failed to finalize reconciliation: %w", err)
		}

		return a.logReconciliationActionWithDB(
			tx,
			statementID,
			nil,
			"RECONCILE",
			map[string]interface{}{"status": bankStatementStatusReconciled, "unmatched": unmatchedCount},
			reconciledBy,
			false,
			1.0,
			"Statement finalized",
		)
	}); err != nil {
		return err
	}

	log.Printf("✅ Bank statement %s reconciled by %s", statementID, reconciledBy)
	return nil
}

// ReopenReconciliation reverts a reconciled statement to InProgress
func (a *App) ReopenReconciliation(statementID string, user, reason string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return fmt.Errorf("statement not found: %w", err)
	}

	if !isFinalBankStatementStatus(statement.Status) {
		return fmt.Errorf("statement is not reconciled")
	}

	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&statement).Updates(map[string]interface{}{
			"status":        bankStatementStatusInProgress,
			"reconciled_at": nil,
			"reconciled_by": "",
		}).Error; err != nil {
			return fmt.Errorf("failed to reopen reconciliation: %w", err)
		}

		return a.logReconciliationActionWithDB(
			tx,
			statementID,
			nil,
			"REOPEN",
			map[string]interface{}{"previous_status": statement.Status, "reason": reason},
			user,
			false,
			1.0,
			reason,
		)
	}); err != nil {
		return err
	}

	log.Printf("🔓 Bank statement %s reopened by %s: %s", statementID, user, reason)
	return nil
}

// =============================================================================
// SUMMARY & STATS
// =============================================================================

// GetReconciliationSummary returns summary stats for a statement
func (a *App) GetReconciliationSummary(statementID string) (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return nil, fmt.Errorf("statement not found: %w", err)
	}

	// Count matched and unmatched
	var matchedCount, unmatchedCount int64
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ? AND is_matched = ?", statementID, true).Count(&matchedCount)
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ? AND is_matched = ?", statementID, false).Count(&unmatchedCount)

	totalCount := matchedCount + unmatchedCount
	matchedPercent := 0.0
	if totalCount > 0 {
		matchedPercent = float64(matchedCount) / float64(totalCount) * 100
	}

	// Sum amounts
	var amounts struct {
		TotalDebits   float64
		TotalCredits  float64
		MatchedAmount float64
	}
	a.db.Model(&BankStatementLine{}).
		Select("COALESCE(SUM(debit), 0) as total_debits, COALESCE(SUM(credit), 0) as total_credits, COALESCE(SUM(CASE WHEN is_matched THEN debit + credit ELSE 0 END), 0) as matched_amount").
		Where("bank_statement_id = ?", statementID).
		Scan(&amounts)

	return map[string]interface{}{
		"statement_id":     statementID,
		"statement_number": statement.StatementNumber,
		"status":           statement.Status,
		"period_start":     statement.PeriodStart,
		"period_end":       statement.PeriodEnd,
		"opening_balance":  statement.OpeningBalance,
		"closing_balance":  statement.ClosingBalance,
		"total_debits":     amounts.TotalDebits,
		"total_credits":    amounts.TotalCredits,
		"total_lines":      totalCount,
		"matched_count":    matchedCount,
		"unmatched_count":  unmatchedCount,
		"matched_percent":  matchedPercent,
		"matched_amount":   amounts.MatchedAmount,
		"balance_verified": statement.BalanceVerified,
		"reconciled_at":    statement.ReconciledAt,
		"reconciled_by":    statement.ReconciledBy,
	}, nil
}

// GetReconciliationStats returns account-level reconciliation statistics
func (a *App) GetReconciliationStats(bankAccountID string) (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Count statements by status
	var stats struct {
		TotalStatements      int64
		ReconciledStatements int64
		InProgressStatements int64
		ImportedStatements   int64
	}

	scopedQuery := func() *gorm.DB {
		query := a.db.Model(&BankStatement{})
		if bankAccountID != "" {
			query = query.Where("bank_account_id = ?", bankAccountID)
		}
		return query
	}

	scopedQuery().Count(&stats.TotalStatements)
	scopedQuery().Where("status = ?", bankStatementStatusReconciled).Count(&stats.ReconciledStatements)
	scopedQuery().Where("status = ?", bankStatementStatusInProgress).Count(&stats.InProgressStatements)
	scopedQuery().Where("status = ?", bankStatementStatusImported).Count(&stats.ImportedStatements)

	// Get latest statement date
	var latestDate time.Time
	scopedQuery().
		Select("MAX(period_end)").
		Scan(&latestDate)

	return map[string]interface{}{
		"bank_account_id":        bankAccountID,
		"total_statements":       stats.TotalStatements,
		"reconciled_statements":  stats.ReconciledStatements,
		"in_progress_statements": stats.InProgressStatements,
		"imported_statements":    stats.ImportedStatements,
		"latest_statement_date":  latestDate,
	}, nil
}

// =============================================================================
// HELPERS
// =============================================================================

// roundBHD rounds a float to 3 decimal places for BHD precision
func roundBHD(amount float64) float64 {
	return math.Round(amount*1000) / 1000
}

// formatBHD formats a float to BHD with 3 decimal places
func formatBHD(amount float64) string {
	return fmt.Sprintf("%.3f", roundBHD(amount))
}

// parseJSONArray parses a JSON array string to []string
func parseJSONArray(jsonStr string) []string {
	if jsonStr == "" {
		return []string{}
	}
	var result []string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return []string{}
	}
	return result
}

// toJSONArray converts []string to JSON array string
func toJSONArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(arr)
	return string(data)
}
