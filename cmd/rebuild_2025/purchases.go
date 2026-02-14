package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// =============================================================================
// PURCHASE DATA IMPORT - 2025 PURCHASES & SUPPLIER PAYMENTS
// =============================================================================

// import2025Purchases imports supplier invoices from 2025 Excel file
func import2025Purchases(db *gorm.DB, supplierMap map[string]string) error {
	log.Println("\n=== IMPORTING 2025 PURCHASES ===")

	// Open Excel file
	filepath := "Data_for_database/Tally_data/purchase 2025.xlsx"
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filepath, err)
	}
	defer f.Close()

	// Get first sheet name dynamically
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets found in Excel file")
	}
	sheetName := sheets[0]

	// Get rows
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("failed to read sheet %s: %w", sheetName, err)
	}

	if len(rows) < 2 {
		return fmt.Errorf("file has no data rows")
	}

	// Find actual header row (Tally exports have company info in first rows)
	log.Printf("→ Reading sheet: %s (%d rows)", sheetName, len(rows))
	headerRowIdx, header := findHeaderRow(rows)
	log.Printf("→ Header found at row %d: %v", headerRowIdx+1, header)
	log.Printf("→ Found %d columns in header", len(header))

	colMap := make(map[string]int)
	for i, colName := range header {
		normalized := strings.ToLower(strings.TrimSpace(colName))
		colMap[normalized] = i
	}

	// Find required columns
	supplierCol := findColumn(colMap, "supplier", "supplier name", "party", "party name", "vendor", "ledger name", "particulars")
	dateCol := findColumn(colMap, "date", "invoice date", "purchase date", "voucher date")
	amountCol := findColumn(colMap, "amount", "total amount", "net amount", "invoice amount", "grand total", "debit", "credit")
	invoiceNumCol := findColumn(colMap, "invoice number", "invoice no", "voucher number", "voucher no", "bill no", "reference", "vch no.")

	if supplierCol == -1 {
		return fmt.Errorf("supplier column not found. Headers: %v", header)
	}
	if amountCol == -1 {
		return fmt.Errorf("amount column not found. Headers: %v", header)
	}

	log.Printf("→ Column mapping:")
	log.Printf("  Supplier: %d (%s)", supplierCol, safeHeader(header, supplierCol))
	log.Printf("  Date: %d (%s)", dateCol, safeHeader(header, dateCol))
	log.Printf("  Amount: %d (%s)", amountCol, safeHeader(header, amountCol))
	log.Printf("  Invoice#: %d (%s)", invoiceNumCol, safeHeader(header, invoiceNumCol))

	// Parse data rows (start from row after header)
	var invoices []SupplierInvoice
	successCount := 0
	skipCount := 0
	unmappedSuppliers := make(map[string]int)
	dataStartRow := headerRowIdx + 1

	for i, row := range rows[dataStartRow:] {
		rowNum := dataStartRow + i + 1 // Excel row number (1-indexed)

		// Extract supplier name
		if supplierCol >= len(row) {
			log.Printf("⚠ Row %d: supplier column out of bounds, skipping", rowNum)
			skipCount++
			continue
		}

		supplierName := strings.TrimSpace(row[supplierCol])
		if supplierName == "" {
			skipCount++
			continue
		}

		// Normalize supplier name
		supplierName = normalizeSupplierName(supplierName)
		supplierID, found := supplierMap[supplierName]
		if !found {
			unmappedSuppliers[supplierName]++
			skipCount++
			continue
		}

		// Extract amount
		if amountCol >= len(row) {
			log.Printf("⚠ Row %d: amount column out of bounds, skipping", rowNum)
			skipCount++
			continue
		}

		amountStr := strings.TrimSpace(row[amountCol])
		amount, err := parseAmount(amountStr)
		if err != nil {
			log.Printf("⚠ Row %d: invalid amount '%s', skipping: %v", rowNum, amountStr, err)
			skipCount++
			continue
		}

		if amount <= 0 {
			skipCount++
			continue
		}

		// Extract date (optional)
		var invoiceDate time.Time
		if dateCol != -1 && dateCol < len(row) {
			dateStr := strings.TrimSpace(row[dateCol])
			invoiceDate, err = parseDate(dateStr)
			if err != nil {
				// Default to 2025-06-01 if date parsing fails
				invoiceDate = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
			}
		} else {
			// Default to mid-2025
			invoiceDate = time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
		}

		// Ensure date is in 2025
		if invoiceDate.Year() != 2025 {
			invoiceDate = time.Date(2025, invoiceDate.Month(), invoiceDate.Day(), 0, 0, 0, 0, time.UTC)
		}

		// Extract invoice number (optional)
		var invoiceNumber string
		if invoiceNumCol != -1 && invoiceNumCol < len(row) {
			invoiceNumber = strings.TrimSpace(row[invoiceNumCol])
		}
		if invoiceNumber == "" {
			invoiceNumber = fmt.Sprintf("PURC-2025-%04d", i+1)
		}

		// Create supplier invoice
		invoiceID := uuid.New().String()
		dueDate := invoiceDate.AddDate(0, 0, 30) // Net 30 default
		vatPercent := 10.0                       // 10% VAT Bahrain
		vatAmount := amount * vatPercent / (100 + vatPercent)
		netAmount := amount

		invoice := SupplierInvoice{
			Base: Base{
				ID:        invoiceID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			InvoiceNumber:   invoiceNumber,
			SupplierID:      supplierID,
			SupplierName:    supplierName,
			InvoiceDate:     invoiceDate,
			DueDate:         dueDate,
			TotalAmount:     amount - vatAmount,
			VATAmount:       vatAmount,
			NetAmount:       netAmount,
			AmountPaid:      netAmount, // Assume historical data is fully paid
			AmountDue:       0,
			Status:          "Posted",
			PaymentStatus:   "Paid",
			ApprovalStatus:  "Approved",
			ThreeWayMatched: false,
			ImportSource:    "2025_rebuild",
		}

		invoices = append(invoices, invoice)
		successCount++
	}

	log.Printf("\n→ Parsed %d purchases:", successCount)
	log.Printf("  Success: %d", successCount)
	log.Printf("  Skipped: %d", skipCount)
	if len(unmappedSuppliers) > 0 {
		log.Printf("  Unmapped suppliers: %d", len(unmappedSuppliers))
		for supplier, count := range unmappedSuppliers {
			log.Printf("    - %s (%d occurrences)", supplier, count)
		}
	}

	// Batch insert
	if len(invoices) > 0 {
		batchSize := 100
		for i := 0; i < len(invoices); i += batchSize {
			end := i + batchSize
			if end > len(invoices) {
				end = len(invoices)
			}

			batch := invoices[i:end]
			if err := db.Create(&batch).Error; err != nil {
				log.Printf("⚠ Failed to insert batch %d-%d: %v", i, end, err)
				continue
			}
			log.Printf("  Inserted batch %d-%d (%d invoices)", i+1, end, len(batch))
		}

		log.Printf("\n✅ Successfully imported %d supplier invoices", len(invoices))
	} else {
		log.Println("⚠ No purchases to import")
	}

	return nil
}

// import2025SupplierPayments imports supplier payments from Excel (filtered to 2025)
func import2025SupplierPayments(db *gorm.DB, supplierMap map[string]string) error {
	log.Println("\n=== IMPORTING 2025 SUPPLIER PAYMENTS ===")

	// Open Excel file
	filepath := "Data_for_database/Payments to suppliers.xlsx"
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filepath, err)
	}
	defer f.Close()

	// Get first sheet name dynamically
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets found in Excel file")
	}
	sheetName := sheets[0]

	// Get rows
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("failed to read sheet %s: %w", sheetName, err)
	}

	if len(rows) < 2 {
		return fmt.Errorf("file has no data rows")
	}

	// Find actual header row (Tally exports have company info in first rows)
	log.Printf("→ Reading sheet: %s (%d rows)", sheetName, len(rows))
	headerRowIdx, header := findHeaderRow(rows)
	log.Printf("→ Header found at row %d: %v", headerRowIdx+1, header)
	log.Printf("→ Found %d columns in header", len(header))

	colMap := make(map[string]int)
	for i, colName := range header {
		normalized := strings.ToLower(strings.TrimSpace(colName))
		colMap[normalized] = i
	}

	// Find required columns
	supplierCol := findColumn(colMap, "supplier", "supplier name", "party", "party name", "vendor", "ledger name", "payee", "particulars")
	dateCol := findColumn(colMap, "date", "payment date", "paid date", "voucher date", "transaction date")
	amountCol := findColumn(colMap, "amount", "payment amount", "paid amount", "total amount", "net amount", "debit", "credit")
	referenceCol := findColumn(colMap, "reference", "ref", "reference no", "voucher no", "cheque no", "transaction ref", "vch no.")
	methodCol := findColumn(colMap, "method", "payment method", "mode", "payment mode")

	if supplierCol == -1 {
		return fmt.Errorf("supplier column not found. Headers: %v", header)
	}
	if dateCol == -1 {
		return fmt.Errorf("date column not found. Headers: %v", header)
	}
	if amountCol == -1 {
		return fmt.Errorf("amount column not found. Headers: %v", header)
	}

	log.Printf("→ Column mapping:")
	log.Printf("  Supplier: %d (%s)", supplierCol, safeHeader(header, supplierCol))
	log.Printf("  Date: %d (%s)", dateCol, safeHeader(header, dateCol))
	log.Printf("  Amount: %d (%s)", amountCol, safeHeader(header, amountCol))
	log.Printf("  Reference: %d (%s)", referenceCol, safeHeader(header, referenceCol))
	log.Printf("  Method: %d (%s)", methodCol, safeHeader(header, methodCol))

	// Parse data rows (start from row after header)
	var payments []SupplierPayment
	successCount := 0
	skipCount := 0
	non2025Count := 0
	unmappedSuppliers := make(map[string]int)
	dataStartRow := headerRowIdx + 1

	for i, row := range rows[dataStartRow:] {
		rowNum := dataStartRow + i + 1 // Excel row number

		// Extract date (CRITICAL for 2025 filtering)
		if dateCol >= len(row) {
			log.Printf("⚠ Row %d: date column out of bounds, skipping", rowNum)
			skipCount++
			continue
		}

		dateStr := strings.TrimSpace(row[dateCol])
		paymentDate, err := parseDate(dateStr)
		if err != nil {
			log.Printf("⚠ Row %d: invalid date '%s', skipping: %v", rowNum, dateStr, err)
			skipCount++
			continue
		}

		// Filter to 2025 only
		if paymentDate.Year() != 2025 {
			non2025Count++
			continue
		}

		// Extract supplier name
		if supplierCol >= len(row) {
			log.Printf("⚠ Row %d: supplier column out of bounds, skipping", rowNum)
			skipCount++
			continue
		}

		supplierName := strings.TrimSpace(row[supplierCol])
		if supplierName == "" {
			skipCount++
			continue
		}

		// Normalize supplier name
		supplierName = normalizeSupplierName(supplierName)
		supplierID, found := supplierMap[supplierName]
		if !found {
			unmappedSuppliers[supplierName]++
			skipCount++
			continue
		}

		// Extract amount
		if amountCol >= len(row) {
			log.Printf("⚠ Row %d: amount column out of bounds, skipping", rowNum)
			skipCount++
			continue
		}

		amountStr := strings.TrimSpace(row[amountCol])
		amount, err := parseAmount(amountStr)
		if err != nil {
			log.Printf("⚠ Row %d: invalid amount '%s', skipping: %v", rowNum, amountStr, err)
			skipCount++
			continue
		}

		if amount <= 0 {
			skipCount++
			continue
		}

		// Extract payment method (optional)
		paymentMethod := "Bank Transfer" // Default
		if methodCol != -1 && methodCol < len(row) {
			method := strings.TrimSpace(row[methodCol])
			if method != "" {
				paymentMethod = method
			}
		}

		// Extract reference (optional)
		var referenceNo string
		if referenceCol != -1 && referenceCol < len(row) {
			referenceNo = strings.TrimSpace(row[referenceCol])
		}
		if referenceNo == "" {
			referenceNo = fmt.Sprintf("PAY-2025-%04d", i+1)
		}

		// Create supplier payment
		paymentID := uuid.New().String()
		paymentNumber := fmt.Sprintf("SPAY-2025-%05d", successCount+1)

		payment := SupplierPayment{
			Base: Base{
				ID:        paymentID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			PaymentNumber: paymentNumber,
			SupplierID:    supplierID,
			SupplierName:  supplierName,
			PaymentDate:   paymentDate,
			Amount:        amount,
			PaymentMethod: paymentMethod,
			ReferenceNo:   referenceNo,
			Status:        "Completed",
			ImportSource:  "2025_rebuild",
		}

		payments = append(payments, payment)
		successCount++
	}

	log.Printf("\n→ Parsed payments:")
	log.Printf("  2025 payments: %d", successCount)
	log.Printf("  Non-2025: %d", non2025Count)
	log.Printf("  Skipped: %d", skipCount)
	if len(unmappedSuppliers) > 0 {
		log.Printf("  Unmapped suppliers: %d", len(unmappedSuppliers))
		for supplier, count := range unmappedSuppliers {
			log.Printf("    - %s (%d occurrences)", supplier, count)
		}
	}

	// Batch insert
	if len(payments) > 0 {
		batchSize := 100
		for i := 0; i < len(payments); i += batchSize {
			end := i + batchSize
			if end > len(payments) {
				end = len(payments)
			}

			batch := payments[i:end]
			if err := db.Create(&batch).Error; err != nil {
				log.Printf("⚠ Failed to insert batch %d-%d: %v", i, end, err)
				continue
			}
			log.Printf("  Inserted batch %d-%d (%d payments)", i+1, end, len(batch))
		}

		log.Printf("\n✅ Successfully imported %d supplier payments (2025 only)", len(payments))
	} else {
		log.Println("⚠ No 2025 payments to import")
	}

	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// parseAmount extracts numeric value from various formats
func parseAmount(s string) (float64, error) {
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}

	// Remove common currency symbols and whitespace
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.ReplaceAll(s, "BD", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, "£", "")
	s = strings.ReplaceAll(s, ",", "") // Remove thousands separator
	s = strings.TrimSpace(s)

	// Handle parentheses as negative (accounting notation)
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = "-" + strings.Trim(s, "()")
	}

	// Parse as float
	amount, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse '%s' as number: %w", s, err)
	}

	return amount, nil
}

// parseDate attempts to parse date from multiple formats
func parseDate(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}

	s = strings.TrimSpace(s)

	// Try common formats
	formats := []string{
		"2006-01-02",
		"02-01-2006",
		"02/01/2006",
		"01/02/2006",
		"2006/01/02",
		"Jan 02 2006",
		"02-Jan-2006",
		"02-Jan-06",
		"2-Jan-2006",
		"2-Jan-06",
		"January 2, 2006",
		"2 January 2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	// Try Excel serial date (days since 1900-01-01)
	if serial, err := strconv.ParseFloat(s, 64); err == nil {
		// Excel date serial (1900-01-01 = 1)
		epoch := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
		days := int(serial) - 2 // Excel has a leap year bug for 1900
		return epoch.AddDate(0, 0, days), nil
	}

	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}

// safeHeader returns header value or "N/A" if index invalid
func safeHeader(header []string, idx int) string {
	if idx == -1 || idx >= len(header) {
		return "N/A"
	}
	return header[idx]
}
