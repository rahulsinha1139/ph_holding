package main

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	log.Println("=== Excel Import Utility ===")

	// Open database
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Import opportunities
	oppFile := `C:\Users\rishu\Downloads\opportunities created 2025.xlsx`
	if _, err := os.Stat(oppFile); err == nil {
		log.Println("\n📊 Processing: opportunities created 2025.xlsx")
		importOpportunities(db, oppFile)
	} else {
		log.Printf("File not found: %s", oppFile)
	}

	// Import supplier payments
	payFile := `C:\Users\rishu\Downloads\Payments to suppliers.xlsx`
	if _, err := os.Stat(payFile); err == nil {
		log.Println("\n💰 Processing: Payments to suppliers.xlsx")
		importSupplierPayments(db, payFile)
	} else {
		log.Printf("File not found: %s", payFile)
	}

	log.Println("\n✅ Import complete!")
}

func importOpportunities(db *gorm.DB, filepath string) {
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		return
	}
	defer f.Close()

	// Get sheet names
	sheets := f.GetSheetList()
	log.Printf("Sheets found: %v", sheets)

	// Get first sheet data
	if len(sheets) == 0 {
		log.Println("No sheets found")
		return
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		log.Printf("Failed to read rows: %v", err)
		return
	}

	log.Printf("Found %d rows", len(rows))

	// Print header row and first few data rows for analysis
	if len(rows) > 0 {
		log.Printf("Header: %v", rows[0])
	}
	for i := 1; i < min(6, len(rows)); i++ {
		log.Printf("Row %d: %v", i, rows[i])
	}

	// Analyze column mapping
	// Common opportunity fields: customer, project, value, date, status
	var customerCol, projectCol, valueCol, dateCol, statusCol int = -1, -1, -1, -1, -1
	if len(rows) > 0 {
		for i, h := range rows[0] {
			h = strings.ToLower(strings.TrimSpace(h))
			if strings.Contains(h, "customer") || strings.Contains(h, "client") {
				customerCol = i
			}
			if strings.Contains(h, "project") || strings.Contains(h, "opportunity") || strings.Contains(h, "description") {
				projectCol = i
			}
			if strings.Contains(h, "value") || strings.Contains(h, "amount") || strings.Contains(h, "total") {
				valueCol = i
			}
			if strings.Contains(h, "date") {
				dateCol = i
			}
			if strings.Contains(h, "status") || strings.Contains(h, "stage") {
				statusCol = i
			}
		}
		log.Printf("Column mapping: customer=%d, project=%d, value=%d, date=%d, status=%d",
			customerCol, projectCol, valueCol, dateCol, statusCol)
	}

	// For now, just report what we found - actual import would need more logic
	log.Printf("📋 Summary: Found %d opportunities to review", len(rows)-1)
}

func importSupplierPayments(db *gorm.DB, filepath string) {
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		log.Printf("Failed to open file: %v", err)
		return
	}
	defer f.Close()

	// Get sheet names
	sheets := f.GetSheetList()
	log.Printf("Sheets found: %v", sheets)

	// Get first sheet data
	if len(sheets) == 0 {
		log.Println("No sheets found")
		return
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		log.Printf("Failed to read rows: %v", err)
		return
	}

	log.Printf("Found %d rows", len(rows))

	// Print header row and first few data rows for analysis
	if len(rows) > 0 {
		log.Printf("Header: %v", rows[0])
	}
	for i := 1; i < min(6, len(rows)); i++ {
		log.Printf("Row %d: %v", i, rows[i])
	}

	// Analyze column mapping
	// Common payment fields: supplier, invoice, amount, date, reference
	var supplierCol, invoiceCol, amountCol, dateCol, refCol int = -1, -1, -1, -1, -1
	if len(rows) > 0 {
		for i, h := range rows[0] {
			h = strings.ToLower(strings.TrimSpace(h))
			if strings.Contains(h, "supplier") || strings.Contains(h, "vendor") || strings.Contains(h, "name") {
				supplierCol = i
			}
			if strings.Contains(h, "invoice") {
				invoiceCol = i
			}
			if strings.Contains(h, "amount") || strings.Contains(h, "payment") || strings.Contains(h, "total") || strings.Contains(h, "bhd") {
				amountCol = i
			}
			if strings.Contains(h, "date") {
				dateCol = i
			}
			if strings.Contains(h, "ref") || strings.Contains(h, "reference") || strings.Contains(h, "cheque") {
				refCol = i
			}
		}
		log.Printf("Column mapping: supplier=%d, invoice=%d, amount=%d, date=%d, ref=%d",
			supplierCol, invoiceCol, amountCol, dateCol, refCol)
	}

	// Count existing payments
	var existingCount int64
	db.Table("supplier_payments").Count(&existingCount)
	log.Printf("Current supplier_payments count: %d", existingCount)

	// For now, just report what we found - actual import would need more logic
	log.Printf("💰 Summary: Found %d payments to review", len(rows)-1)
}

func parseDate(s string) time.Time {
	// Try common formats
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"02-01-2006",
		"01-02-2006",
		"2006/01/02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
