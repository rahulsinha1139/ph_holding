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
// INVOICE IMPORT - 2025 DATA
// =============================================================================

// import2025Invoices imports invoices from 2025 Excel file
func import2025Invoices(db *gorm.DB, customerMap map[string]string) error {
	log.Println("\n→ Importing 2025 invoices from Excel...")

	// Open Excel file
	filepath := "Data_for_database/Tally_data/invoices 2025.xlsx"
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

	log.Printf("  Reading sheet: %s (%d rows)", sheetName, len(rows))

	// Find actual header row (Tally exports have company info in first rows)
	headerRowIdx, header := findHeaderRow(rows)
	log.Printf("  Header found at row %d: %v", headerRowIdx+1, header)

	// Parse header
	colMap := make(map[string]int)
	for i, colName := range header {
		normalized := strings.ToLower(strings.TrimSpace(colName))
		colMap[normalized] = i
	}

	// Find columns (flexible matching - Tally exports use "Particulars" for customer/party names)
	invoiceNumCol := findColumn(colMap, "invoice number", "voucher number", "inv no", "invoice no", "voucher no", "vch no.", "vch no")
	dateCol := findColumn(colMap, "date", "invoice date", "voucher date", "inv date")
	customerCol := findColumn(colMap, "customer", "customer name", "party", "party name", "customer account", "particulars", "ledger name")
	amountCol := findColumn(colMap, "amount", "total", "total amount", "net amount", "invoice amount", "grand total", "taxable\nvalue", "taxable value")
	voucherTypeCol := findColumn(colMap, "voucher type", "type", "transaction type", "vch type")
	voucherRefCol := findColumn(colMap, "voucher ref", "reference", "ref no", "reference no", "supplier inv/ref no./ date")

	if customerCol == -1 {
		return fmt.Errorf("customer column not found. Headers: %v", header)
	}

	if amountCol == -1 {
		return fmt.Errorf("amount column not found. Headers: %v", header)
	}

	log.Printf("  Column mapping: Invoice=%d, Date=%d, Customer=%d, Amount=%d, VoucherType=%d, VoucherRef=%d",
		invoiceNumCol, dateCol, customerCol, amountCol, voucherTypeCol, voucherRefCol)

	// Parse invoices (start from row after header)
	var invoices []Invoice
	var skippedRows []string
	invoiceCounter := 1
	dataStartRow := headerRowIdx + 1

	for i, row := range rows[dataStartRow:] {
		rowNum := dataStartRow + i + 1 // Excel row number (1-indexed)

		// Extract customer name
		if customerCol >= len(row) {
			skippedRows = append(skippedRows, fmt.Sprintf("Row %d: customer column out of bounds", rowNum))
			continue
		}

		customerName := strings.TrimSpace(row[customerCol])
		if customerName == "" {
			skippedRows = append(skippedRows, fmt.Sprintf("Row %d: empty customer name", rowNum))
			continue
		}

		// Normalize customer name and look up ID
		normalizedCustomer := normalizeCustomerName(customerName)
		customerID, exists := customerMap[normalizedCustomer]
		if !exists {
			// Try to find a partial match
			found := false
			for mappedName, mappedID := range customerMap {
				if strings.Contains(strings.ToLower(normalizedCustomer), strings.ToLower(mappedName)) ||
					strings.Contains(strings.ToLower(mappedName), strings.ToLower(normalizedCustomer)) {
					customerID = mappedID
					normalizedCustomer = mappedName
					found = true
					break
				}
			}
			if !found {
				skippedRows = append(skippedRows, fmt.Sprintf("Row %d: customer not found: %s (normalized: %s)", rowNum, customerName, normalizedCustomer))
				continue
			}
		}

		// Extract amount
		if amountCol >= len(row) {
			skippedRows = append(skippedRows, fmt.Sprintf("Row %d: amount column out of bounds", rowNum))
			continue
		}

		amountStr := strings.TrimSpace(row[amountCol])
		if amountStr == "" {
			skippedRows = append(skippedRows, fmt.Sprintf("Row %d: empty amount", rowNum))
			continue
		}

		// Parse amount (handle BHD prefix, commas, etc.)
		amountStr = strings.ReplaceAll(amountStr, "BHD", "")
		amountStr = strings.ReplaceAll(amountStr, ",", "")
		amountStr = strings.TrimSpace(amountStr)

		totalAmount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil {
			skippedRows = append(skippedRows, fmt.Sprintf("Row %d: invalid amount: %s", rowNum, amountStr))
			continue
		}

		// Bahrain VAT is 10% - calculate breakdown
		// Total = Subtotal + VAT
		// Total = Subtotal * 1.1
		// Subtotal = Total / 1.1
		subtotal := totalAmount / 1.1
		vatAmount := totalAmount - subtotal

		// Extract invoice number
		var invoiceNumber string
		if invoiceNumCol != -1 && invoiceNumCol < len(row) {
			invoiceNumber = strings.TrimSpace(row[invoiceNumCol])
		}
		if invoiceNumber == "" {
			// Generate invoice number: INV-2025-0001
			invoiceNumber = fmt.Sprintf("INV-2025-%04d", invoiceCounter)
			invoiceCounter++
		}

		// Extract date
		var invoiceDate time.Time
		if dateCol != -1 && dateCol < len(row) {
			dateStr := strings.TrimSpace(row[dateCol])
			invoiceDate = parseFlexibleDate(dateStr)
		}
		if invoiceDate.IsZero() {
			// Default to mid-2025 if date parsing fails
			invoiceDate = time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
		}

		// Extract voucher metadata
		var voucherType, voucherRef string
		if voucherTypeCol != -1 && voucherTypeCol < len(row) {
			voucherType = strings.TrimSpace(row[voucherTypeCol])
		}
		if voucherRefCol != -1 && voucherRefCol < len(row) {
			voucherRef = strings.TrimSpace(row[voucherRefCol])
		}

		// Determine status and payment status
		// Heuristic: If invoice is older than 90 days, assume Paid
		// Otherwise, assume Sent/Unpaid
		status := "Sent"
		paymentStatus := "Unpaid"
		amountPaid := 0.0
		amountDue := totalAmount

		daysSinceInvoice := time.Since(invoiceDate).Hours() / 24
		if daysSinceInvoice > 90 {
			status = "Sent"
			paymentStatus = "Paid"
			amountPaid = totalAmount
			amountDue = 0.0
		}

		// Calculate due date (Net 30 default)
		dueDate := invoiceDate.AddDate(0, 0, 30)

		// Generate unique IDs
		invoiceID := uuid.New().String()
		// For imports: leave OrderID empty (no linked order exists)
		// We'll drop the FK constraint before import
		placeholderOrderID := ""

		// Create invoice record
		invoice := Invoice{
			Base: Base{
				ID:        invoiceID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			InvoiceNumber:      invoiceNumber,
			OrderID:            placeholderOrderID, // Unique placeholder for imports
			CustomerID:         customerID,
			CustomerName:       normalizedCustomer,
			InvoiceDate:        invoiceDate,
			DueDate:            dueDate,
			TotalAmount:        subtotal,
			DiscountAmount:     0.0,
			VATAmount:          vatAmount,
			NetAmount:          totalAmount,
			AmountPaid:         amountPaid,
			AmountDue:          amountDue,
			Status:             status,
			PaymentStatus:      paymentStatus,
			TallyVoucherType:   voucherType,
			TallyVoucherRef:    voucherRef,
			TallyVoucherNumber: invoiceNumber,
			ImportSource:       "Tally 2025 Import",
		}

		invoices = append(invoices, invoice)
	}

	log.Printf("  Parsed %d invoices, skipped %d rows", len(invoices), len(skippedRows))

	if len(skippedRows) > 0 && len(skippedRows) <= 10 {
		log.Println("  Skipped rows details:")
		for _, reason := range skippedRows {
			log.Printf("    - %s", reason)
		}
	} else if len(skippedRows) > 10 {
		log.Printf("  Skipped rows: %d (showing first 10):", len(skippedRows))
		for _, reason := range skippedRows[:10] {
			log.Printf("    - %s", reason)
		}
	}

	// Batch insert invoices (1000 at a time)
	if len(invoices) == 0 {
		log.Println("  ⚠️  No invoices to insert")
		return nil
	}

	// Drop FK constraint and uniqueIndex for import (historical invoices without orders)
	log.Println("  Dropping constraints for import...")
	db.Exec("ALTER TABLE invoices DROP CONSTRAINT IF EXISTS fk_orders_invoices")
	db.Exec("DROP INDEX IF EXISTS idx_invoices_order_id") // Drop the uniqueIndex too

	batchSize := 1000
	for i := 0; i < len(invoices); i += batchSize {
		end := i + batchSize
		if end > len(invoices) {
			end = len(invoices)
		}

		batch := invoices[i:end]
		if err := db.Create(&batch).Error; err != nil {
			return fmt.Errorf("batch insert failed (batch %d-%d): %w", i, end, err)
		}

		log.Printf("  ✅ Inserted batch %d-%d (%d invoices)", i+1, end, len(batch))
	}

	// Calculate totals
	var totalRevenue float64
	var totalVAT float64
	for _, inv := range invoices {
		totalRevenue += inv.NetAmount
		totalVAT += inv.VATAmount
	}

	log.Printf("\n  📊 Invoice Import Summary:")
	log.Printf("     Total Invoices:       %d", len(invoices))
	log.Printf("     Total Revenue (incl VAT): BHD %.3f", totalRevenue)
	log.Printf("     Total VAT Collected:  BHD %.3f", totalVAT)
	log.Printf("     Average Invoice:      BHD %.3f", totalRevenue/float64(len(invoices)))

	return nil
}

// =============================================================================
// DATE PARSING UTILITIES
// =============================================================================

// parseFlexibleDate tries multiple date formats
func parseFlexibleDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	// Common formats in Tally exports
	formats := []string{
		"02/01/2006",       // DD/MM/YYYY (UK/Bahrain)
		"2/1/2006",         // D/M/YYYY
		"02-01-2006",       // DD-MM-YYYY
		"2-1-2006",         // D-M-YYYY
		"2006-01-02",       // YYYY-MM-DD (ISO)
		"01/02/2006",       // MM/DD/YYYY (US)
		"1/2/2006",         // M/D/YYYY
		"02-Jan-2006",      // DD-Mon-YYYY
		"2-Jan-2006",       // D-Mon-YYYY
		"02 Jan 2006",      // DD Mon YYYY
		"January 2, 2006",  // Month D, YYYY
		"Jan 2, 2006",      // Mon D, YYYY
		"2006/01/02",       // YYYY/MM/DD
		"20060102",         // YYYYMMDD
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t
		}
	}

	// Try parsing Excel serial date (days since 1900-01-01)
	if serial, err := strconv.ParseFloat(dateStr, 64); err == nil {
		// Excel epoch: 1900-01-01 (but Excel has a leap year bug for 1900)
		// Days since 1900-01-01, adjusting for Excel's 1900 leap year bug
		excelEpoch := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC)
		return excelEpoch.AddDate(0, 0, int(serial))
	}

	log.Printf("    ⚠️  Could not parse date: %s", dateStr)
	return time.Time{}
}
