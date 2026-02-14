package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
	CreatedBy string
}

// Invoice model - must match app's database.go Invoice struct
type Invoice struct {
	Base
	InvoiceNumber  string    `gorm:"uniqueIndex;size:50"`
	CustomerID     string    `gorm:"index;size:36"`
	CustomerName   string    `gorm:"size:255"`
	OrderID        string    `gorm:"index;size:36"`
	InvoiceDate    time.Time `gorm:"index"`
	DueDate        time.Time
	SubtotalBHD    float64
	GrandTotalBHD  float64 // App uses this for display
	OutstandingBHD float64 // App uses this for AR tracking
	Status         string  `gorm:"index;size:50;default:'Draft'"`
	Notes          string  `gorm:"type:text"`
}

func (Invoice) TableName() string { return "invoices" }

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	dbFile := "ph_holdings.db"
	invoiceFile := "Data_for_database/Tally_data/invoices 2025.xlsx"

	fmt.Println("📊 Importing Tally 2025 Data...")

	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Ensure invoice table exists
	db.AutoMigrate(&Invoice{})

	// Import invoices
	invoiceCount, err := importInvoices2025(db, invoiceFile)
	if err != nil {
		fmt.Printf("⚠️ Invoice import error: %v\n", err)
	} else {
		fmt.Printf("✅ Imported %d invoices from 2025\n", invoiceCount)
	}

	// Copy to build
	fmt.Println("\n📦 Deploying to build folder...")
	input, _ := os.ReadFile(dbFile)
	os.WriteFile("build/bin/data/ph_holdings.db", input, 0644)

	fmt.Println("✨ Done!")
}

func importInvoices2025(db *gorm.DB, filePath string) (int, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	// Get all sheet names
	sheets := f.GetSheetList()
	fmt.Printf("   Sheets in file: %v\n", sheets)

	if len(sheets) == 0 {
		return 0, fmt.Errorf("no sheets found")
	}

	// Try to find the right sheet
	sheetName := sheets[0]
	for _, s := range sheets {
		lower := strings.ToLower(s)
		if strings.Contains(lower, "invoice") || strings.Contains(lower, "sales") {
			sheetName = s
			break
		}
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, err
	}

	fmt.Printf("   Using sheet: %s (%d rows)\n", sheetName, len(rows))

	// Print first 15 rows to understand structure
	fmt.Println("   First rows:")
	for i, row := range rows {
		if i < 15 {
			fmt.Printf("   Row %d: %v\n", i+1, row)
		}
	}

	// Find header row and data start
	headerRow := -1
	for i, row := range rows {
		if len(row) > 0 {
			firstCell := strings.ToLower(strings.TrimSpace(row[0]))
			if strings.Contains(firstCell, "date") || strings.Contains(firstCell, "particulars") {
				headerRow = i
				break
			}
		}
	}

	if headerRow == -1 {
		// Try to find data rows directly (format: Date | Customer | Type | No | Amount)
		headerRow = 6 // Skip company header rows
	}

	fmt.Printf("   Data starts at row: %d\n", headerRow+2)

	// Format: Date | Customer | TRN | Vch Type | Vch No | Ref | Taxable Value | Tax Amount
	created := 0
	for i := 8; i < len(rows); i++ { // Data starts at row 9 (index 8)
		row := rows[i]
		if len(row) < 6 {
			continue
		}

		dateStr := strings.TrimSpace(getCell(row, 0))
		customerName := strings.TrimSpace(getCell(row, 1))
		trn := strings.TrimSpace(getCell(row, 2))
		vchNo := strings.TrimSpace(getCell(row, 4))
		taxableValue := parseFloat(getCell(row, 6))
		taxAmount := parseFloat(getCell(row, 7))

		// Skip empty or header rows
		if dateStr == "" || customerName == "" {
			continue
		}

		// Skip total/summary rows
		lowerCustomer := strings.ToLower(customerName)
		if strings.Contains(lowerCustomer, "total") || strings.Contains(lowerCustomer, "grand") || strings.Contains(lowerCustomer, "particulars") {
			continue
		}

		// Skip if no value
		if taxableValue == 0 {
			continue
		}

		totalAmount := taxableValue + taxAmount

		// Use the actual invoice number (PH25/001 format)
		invNumber := vchNo
		if invNumber == "" {
			invNumber = fmt.Sprintf("INV-2025-%d", i)
		}

		// Check if already exists
		var existing Invoice
		if db.Where("invoice_number = ?", invNumber).First(&existing).Error == nil {
			continue
		}

		// Parse date
		invDate := parseDate(dateStr)

		// Find or link customer
		customerID := findCustomerID(db, customerName)

		invoice := Invoice{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			InvoiceNumber:  invNumber,
			CustomerID:     customerID,
			CustomerName:   customerName,
			InvoiceDate:    invDate,
			DueDate:        invDate.AddDate(0, 0, 30),
			SubtotalBHD:    taxableValue,
			GrandTotalBHD:  totalAmount,
			OutstandingBHD: totalAmount, // All imported as unpaid
			Status:         "Sent",
			Notes:          fmt.Sprintf("TRN: %s | VAT: %.3f BHD", trn, taxAmount),
		}

		if err := db.Create(&invoice).Error; err == nil {
			created++
			fmt.Printf("   + %s: %s - %.3f BHD\n", invNumber, customerName, totalAmount)
		}
	}

	return created, nil
}

func findCustomerID(db *gorm.DB, name string) string {
	// Try to match customer by name
	var result struct {
		ID string
	}

	// Normalize name for matching
	searchName := strings.ToUpper(name)

	// Try exact match first
	db.Table("customers").Select("id").Where("UPPER(business_name) LIKE ? ESCAPE '\\\\'", "%"+escapeLikeWildcards(searchName[:min(10, len(searchName))])+"%").First(&result)

	return result.ID
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func getCell(row []string, idx int) string {
	if idx < len(row) {
		return row[idx]
	}
	return ""
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{
		"02/01/2006",
		"2/1/2006",
		"02-01-2006",
		"2-Jan-2006",
		"02-Jan-06",
		"2006-01-02",
		"01/02/2006",
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func escapeLikeWildcards(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
