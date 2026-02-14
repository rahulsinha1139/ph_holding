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

type SupplierInvoice struct {
	Base
	InvoiceNumber  string    `gorm:"uniqueIndex;size:50"`
	SupplierID     string    `gorm:"index;size:36"`
	SupplierName   string    `gorm:"index;size:255"`
	POID           string    `gorm:"index;size:36"`
	PONumber       string    `gorm:"size:50"`
	InvoiceDate    time.Time `gorm:"index"`
	DueDate        time.Time
	SubtotalBHD    float64
	VATBHD         float64
	TotalBHD       float64
	Status         string `gorm:"index;size:50;default:'Pending'"`
	PaymentStatus  string `gorm:"index;size:50;default:'Unpaid'"`
	Notes          string `gorm:"type:text"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

type SupplierMaster struct {
	Base
	SupplierCode string `gorm:"uniqueIndex;size:50"`
	SupplierName string `gorm:"index;size:255"`
	Country      string `gorm:"index;size:100"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	dbFile := "ph_holdings.db"
	purchaseFiles := []string{
		"Data_for_database/Tally_data/purchase 2023.xlsx",
		"Data_for_database/Tally_data/purchase 2024.xlsx",
		"Data_for_database/Tally_data/purchase 2025.xlsx",
	}

	fmt.Println("🚀 Seeding Supplier Invoices from Tally Purchase Data...")

	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Ensure table exists
	if err := db.AutoMigrate(&SupplierInvoice{}); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	totalCreated := 0
	for _, file := range purchaseFiles {
		created, err := importPurchaseFile(db, file)
		if err != nil {
			fmt.Printf("⚠️ Error processing %s: %v\n", file, err)
			continue
		}
		totalCreated += created
		fmt.Printf("  ✅ %s: %d invoices\n", file, created)
	}

	fmt.Printf("\n📈 Total Supplier Invoices Created: %d\n", totalCreated)

	// Update build database
	fmt.Println("\n📦 Deploying updated database...")
	if err := deployDatabase(dbFile); err != nil {
		fmt.Printf("⚠️ Warning: %v\n", err)
	}
}

func importPurchaseFile(db *gorm.DB, filePath string) (int, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, err
	}

	// Extract year from file path for invoice numbering
	year := "2025"
	if strings.Contains(filePath, "2023") {
		year = "2023"
	} else if strings.Contains(filePath, "2024") {
		year = "2024"
	}

	created := 0
	for i, row := range rows {
		// Data starts at row 9 (index 8), skip headers
		if i < 8 {
			continue
		}
		if len(row) < 2 {
			continue
		}

		// Format: Date | Supplier | Vch Type | Vch No | Debit | Credit
		// The cells might be merged, so parse carefully
		dateStr := strings.TrimSpace(getCell(row, 0))
		supplierName := strings.TrimSpace(getCell(row, 1))
		vchNo := strings.TrimSpace(getCell(row, 3))

		// Get amount from Credit column (index 5) or last numeric value
		amount := 0.0
		for j := len(row) - 1; j >= 4; j-- {
			val := parseFloat(getCell(row, j))
			if val > 0 {
				amount = val
				break
			}
		}

		// Skip rows without valid data
		if dateStr == "" || supplierName == "" || amount == 0 {
			continue
		}

		// Skip summary/total rows
		lowerSupplier := strings.ToLower(supplierName)
		if strings.Contains(lowerSupplier, "total") || strings.Contains(lowerSupplier, "grand") {
			continue
		}

		// Generate unique invoice number
		invNumber := fmt.Sprintf("SUPP-%s-%s-%d", year, vchNo, i)
		if vchNo == "" {
			invNumber = fmt.Sprintf("SUPP-%s-%d", year, i)
		}

		// Check if exists
		var existing SupplierInvoice
		if err := db.Where("invoice_number = ?", invNumber).First(&existing).Error; err == nil {
			continue
		}

		// Parse date
		invDate := parseDate(dateStr)

		// Find or create supplier
		supplierID := findOrCreateSupplier(db, supplierName)

		// Calculate due date (Net 30)
		dueDate := invDate.AddDate(0, 0, 30)

		// Determine payment status based on age
		paymentStatus := "Unpaid"
		if time.Since(dueDate) > 90*24*time.Hour {
			paymentStatus = "Paid" // Assume old invoices are paid
		}

		invoice := SupplierInvoice{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			InvoiceNumber: invNumber,
			SupplierID:    supplierID,
			SupplierName:  supplierName,
			InvoiceDate:   invDate,
			DueDate:       dueDate,
			SubtotalBHD:   amount / 1.1, // Assume 10% VAT included
			VATBHD:        amount * 0.1 / 1.1,
			TotalBHD:      amount,
			Status:        "Approved",
			PaymentStatus: paymentStatus,
		}

		if err := db.Create(&invoice).Error; err == nil {
			created++
		}
	}

	return created, nil
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
		"2006-01-02",
		"02-Jan-2006",
		"02/01/2006",
		"01/02/2006",
		"2-Jan-06",
		"Jan 2, 2006",
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func findOrCreateSupplier(db *gorm.DB, name string) string {
	var supplier SupplierMaster
	normalizedName := normalizeName(name)

	if err := db.Where("supplier_name LIKE ? ESCAPE '\\\\'", "%"+escapeLikeWildcards(normalizedName)+"%").First(&supplier).Error; err == nil {
		return supplier.ID
	}

	// Create new supplier
	newSupplier := SupplierMaster{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		SupplierCode: generateCode(name),
		SupplierName: name,
		Country:      "International",
	}
	db.Create(&newSupplier)
	return newSupplier.ID
}

func normalizeName(name string) string {
	name = strings.ToLower(name)
	// Extract key parts
	if strings.Contains(name, "endress") {
		return "Endress"
	}
	if strings.Contains(name, "servomex") {
		return "Servomex"
	}
	if strings.Contains(name, "siemens") {
		return "Siemens"
	}
	// Return first word
	parts := strings.Fields(name)
	if len(parts) > 0 {
		return parts[0]
	}
	return name
}

func generateCode(name string) string {
	code := strings.ToUpper(strings.ReplaceAll(name, " ", ""))
	if len(code) > 6 {
		code = code[:6]
	}
	return code + "-" + uuid.New().String()[:4]
}

func deployDatabase(dbFile string) error {
	input, err := os.ReadFile(dbFile)
	if err != nil {
		return err
	}
	return os.WriteFile("build/bin/data/ph_holdings.db", input, 0644)
}

func escapeLikeWildcards(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
