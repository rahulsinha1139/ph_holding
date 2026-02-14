package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// =============================================================================
// REIMPORT SUPPLIER INVOICE NUMBERS FROM EXCEL SOURCE
// =============================================================================
//
// Purpose: Restore original invoice numbers from "Payments to suppliers.xlsx"
//          to replace auto-generated SI-* numbers in database
//
// Excel Structure:
//   - Sheet "E+H": 696 records (10-digit format: 6017248465)
//   - Sheet "L&G MTE": 90 records (8-digit format: 90038014)
//   - Sheet "Enerdat": 42 records (format: 2020-0006)
//   - Sheet "AHS SALE": 10 records
//   - Sheet "various brands": 361 records
//
// Matching Strategy:
//   1. Match by supplier_name + invoice_date + total_bhd (approximate)
//   2. Extract invoice number from Excel column "Invoice No."
//   3. Update database record with original invoice_number
//
// =============================================================================

type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
}

type SupplierInvoice struct {
	Base
	InvoiceNumber      string    `gorm:"size:100"`
	SupplierID         string    `gorm:"index;size:36"`
	SupplierName       string    `gorm:"size:255"`
	POID               string    `gorm:"column:po_id;size:36"`
	PONumber           string    `gorm:"size:50"`
	InvoiceDate        time.Time `gorm:"index"`
	DueDate            time.Time
	SubtotalBHD        float64   `gorm:"type:decimal(15,3)"`
	VATBHD             float64   `gorm:"type:decimal(15,3)"`
	TotalBHD           float64   `gorm:"type:decimal(15,3)"`
	Status             string    `gorm:"size:50;default:'Pending'"`
	PaymentStatus      string    `gorm:"size:50"`
	Notes              string    `gorm:"type:text"`
	PurchaseOrderID    string    `gorm:"size:36"`
	SubtotalForeign    float64   `gorm:"type:decimal(15,3)"`
	VATForeign         float64   `gorm:"type:decimal(15,3)"`
	TotalForeign       float64   `gorm:"type:decimal(15,3)"`
	Currency           string    `gorm:"size:10"`
	ExchangeRate       float64   `gorm:"type:decimal(10,6)"`
	MatchStatus        string    `gorm:"size:50"`
	POMatchOK          bool
	GRNMatchOK         bool
	GRNID              string `gorm:"size:36"`
	DiscrepancyReason  string `gorm:"type:text"`
	OCRDocumentID      string `gorm:"size:36"`
	OCRConfidence      float64
	ApprovedBy         string `gorm:"size:100"`
	ApprovedAt         *time.Time
	PaymentDate        *time.Time
	PaymentRef         string `gorm:"size:100"`
	PaymentMethod      string `gorm:"size:50"`
	DisputeReason      string `gorm:"type:text"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

// Sheet name to supplier name mapping
var sheetToSupplierMapping = map[string][]string{
	"E+H":            {"Endress + Hauser", "Endress+Hauser", "E+H"},
	"L&G MTE":        {"Landys and Gyr", "Landis & Gyr", "L&G", "Landis+Gyr"},
	"Enerdat":        {"Enerdat"},
	"G+I":            {"G+I"},
	"AHS SALE":       {"AHS Trading", "AHS"},
	"various brands": {"Various", "Various Brands"},
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	dbFile := "ph_holdings.db"
	excelFile := "Data_for_database/Payments to suppliers.xlsx"

	fmt.Println("🔄 Reimporting Supplier Invoice Numbers from Excel Source...")

	// Connect to database
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Open Excel file
	f, err := excelize.OpenFile(excelFile)
	if err != nil {
		log.Fatalf("Failed to open Excel file: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	fmt.Printf("📋 Found sheets: %v\n\n", sheets)

	// Statistics
	totalProcessed := 0
	totalUpdated := 0
	totalSkipped := 0
	totalNotFound := 0

	// Process each sheet
	for _, sheetName := range sheets {
		// Get supplier names for this sheet
		supplierNames, ok := sheetToSupplierMapping[sheetName]
		if !ok {
			log.Printf("⚠️  Skipping unknown sheet: %s", sheetName)
			continue
		}

		fmt.Printf("📄 Processing sheet: %s\n", sheetName)

		rows, err := f.GetRows(sheetName)
		if err != nil {
			log.Printf("⚠️  Failed to read rows: %v", err)
			continue
		}

		if len(rows) < 4 {
			log.Printf("⚠️  Sheet has insufficient rows: %d", len(rows))
			continue
		}

		sheetUpdated := 0
		sheetSkipped := 0
		sheetNotFound := 0

		// Process all data rows (start from row 3, skip header assumptions)
		for i := 3; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 2 {
				continue
			}

			// Scan all cells for invoice number, date, and amount (pattern-based)
			var invoiceNo string
			var invoiceDate time.Time
			var amount float64

			for _, cell := range row {
				cell = strings.TrimSpace(cell)
				if cell == "" || cell == "-" {
					continue
				}

				// Pattern detection for invoice numbers
				// E+H: 10-digit starting with 6 (6017248465)
				// L&G: 8-digit starting with 9 (90038014) or other patterns
				// Enerdat: YYYY-NNNN (2020-0006)
				if invoiceNo == "" {
					if matched := extractInvoiceNumber(cell, sheetName); matched != "" {
						invoiceNo = matched
					}
				}

				// Extract date
				if invoiceDate.IsZero() {
					if t := parseDate(cell); !t.IsZero() {
						invoiceDate = t
					}
				}

				// Extract amount
				if amount == 0 {
					if strings.Contains(cell, "€") || strings.Contains(cell, "$") ||
						strings.Contains(cell, "BHD") || strings.Contains(cell, "AED") ||
						strings.Contains(cell, "Euro") {
						amount = parseAmount(cell)
					}
				}
			}

			if invoiceNo == "" || amount == 0 {
				// Need invoice number and amount at minimum
				continue
			}

			totalProcessed++

			// Try to find matching invoice in database
			// Strategy: Match by supplier + amount only (dates are unreliable due to year shifts)
			var invoice SupplierInvoice
			query := db.Where("supplier_name IN ?", supplierNames)

			// Match by amount with ±20% tolerance (accounts for currency conversion variations)
			minAmount := amount * 0.80
			maxAmount := amount * 1.20

			query = query.Where("total_bhd >= ? AND total_bhd <= ?", minAmount, maxAmount)

			// Find first match with auto-generated number (SI-* or SUPP-*)
			// Prefer SI-* first (most recently auto-generated), then SUPP-*
			query = query.Where("(invoice_number LIKE 'SI-%' OR invoice_number LIKE 'SUPP-%')").
				Order("CASE WHEN invoice_number LIKE 'SI-%' THEN 0 ELSE 1 END, invoice_date ASC")

			if err := query.First(&invoice).Error; err != nil {
				// No match found
				sheetNotFound++
				totalNotFound++
				continue
			}

			// Check if already a real invoice number (not auto-generated)
			if !strings.HasPrefix(invoice.InvoiceNumber, "SI-") &&
				!strings.HasPrefix(invoice.InvoiceNumber, "SUPP-") {
				sheetSkipped++
				totalSkipped++
				continue
			}

			// Update with original invoice number
			oldNumber := invoice.InvoiceNumber
			invoice.InvoiceNumber = invoiceNo
			invoice.UpdatedAt = time.Now()

			if err := db.Save(&invoice).Error; err != nil {
				log.Printf("⚠️  Failed to update invoice %s: %v", oldNumber, err)
				continue
			}

			sheetUpdated++
			totalUpdated++

			if sheetUpdated <= 5 {
				fmt.Printf("  ✓ Updated: %s → %s (Supplier: %s, Date: %s, Amount: %.2f)\n",
					oldNumber, invoiceNo, invoice.SupplierName,
					invoice.InvoiceDate.Format("2006-01-02"), invoice.TotalBHD)
			}
		}

		fmt.Printf("  📊 Sheet %s: %d updated, %d skipped, %d not found\n\n",
			sheetName, sheetUpdated, sheetSkipped, sheetNotFound)
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Printf("✅ Reimport Complete!\n")
	fmt.Printf("   Total Processed: %d\n", totalProcessed)
	fmt.Printf("   Total Updated:   %d\n", totalUpdated)
	fmt.Printf("   Total Skipped:   %d (already original)\n", totalSkipped)
	fmt.Printf("   Total Not Found: %d (no match in DB)\n\n", totalNotFound)

	// Show final statistics
	var autoGenSI, autoGenSUPP, originalCount int64
	db.Model(&SupplierInvoice{}).Where("invoice_number LIKE 'SI-%'").Count(&autoGenSI)
	db.Model(&SupplierInvoice{}).Where("invoice_number LIKE 'SUPP-%'").Count(&autoGenSUPP)
	db.Model(&SupplierInvoice{}).Where("invoice_number NOT LIKE 'SI-%' AND invoice_number NOT LIKE 'SUPP-%' AND invoice_number NOT LIKE 'OCR-%'").Count(&originalCount)

	fmt.Printf("📈 Database Status:\n")
	fmt.Printf("   Auto-Generated (SI-*):   %d\n", autoGenSI)
	fmt.Printf("   Auto-Generated (SUPP-*): %d\n", autoGenSUPP)
	fmt.Printf("   Original Numbers:        %d\n", originalCount)
}

// findColumn searches for column index by possible header names
func findColumn(headerRow []string, possibleNames []string) int {
	for i, header := range headerRow {
		headerClean := strings.TrimSpace(strings.ToLower(header))
		for _, name := range possibleNames {
			if strings.Contains(headerClean, strings.ToLower(name)) {
				return i
			}
		}
	}
	return -1
}

// extractInvoiceNumber detects and extracts invoice numbers by pattern
func extractInvoiceNumber(cell string, sheetName string) string {
	cell = strings.TrimSpace(cell)

	// Skip common non-invoice values
	if cell == "" || cell == "-" || cell == "N/A" {
		return ""
	}

	// Remove prefixes
	cell = strings.TrimPrefix(cell, "Invoice No:")
	cell = strings.TrimPrefix(cell, "Inv:")
	cell = strings.TrimSpace(cell)

	// E+H sheet: Look for 10-digit numbers starting with 6
	if sheetName == "E+H" {
		if matched, _ := regexp.MatchString(`^6[0-9]{9}$`, cell); matched {
			return cell
		}
	}

	// L&G sheet: Look for 8-digit numbers starting with 9
	if sheetName == "L&G MTE" {
		if matched, _ := regexp.MatchString(`^9[0-9]{7}$`, cell); matched {
			return cell
		}
		// Also try 7-digit or other patterns
		if matched, _ := regexp.MatchString(`^[0-9]{7,10}$`, cell); matched {
			return cell
		}
	}

	// Enerdat sheet: Look for YYYY-NNNN format
	if sheetName == "Enerdat" {
		if matched, _ := regexp.MatchString(`^20[0-9]{2}-[0-9]{4,6}$`, cell); matched {
			return cell
		}
	}

	// Generic: Any alphanumeric invoice-like pattern (6-20 chars, mix of letters/numbers)
	if matched, _ := regexp.MatchString(`^[A-Z0-9-/]{6,20}$`, cell); matched {
		// Exclude dates (contains /)
		if !strings.Contains(cell, "/") {
			return cell
		}
	}

	// If it contains "/" but looks like invoice (not date)
	if strings.Contains(cell, "/") && len(cell) < 20 {
		// Check if it's a date first
		if t := parseDate(cell); t.IsZero() {
			// Not a date, probably invoice number
			return cell
		}
	}

	return ""
}

// parseAmount extracts numeric amount from string
func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)

	// Remove currency symbols
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, "Euro", "")
	s = strings.ReplaceAll(s, "AED", "")
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.TrimSpace(s)

	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

// parseDate attempts to parse date from string using common formats
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return time.Time{}
	}

	formats := []string{
		"2/1/2006",
		"02/01/2006",
		"2-Jan-06",
		"02-Jan-2006",
		"2006-01-02",
		"1/2/2006",
		"01/02/2006",
		"2-1-2006",
		"02-01-2006",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			// Sanity check: year should be 2020-2025
			if t.Year() >= 2020 && t.Year() <= 2025 {
				return t
			}
		}
	}

	return time.Time{}
}
