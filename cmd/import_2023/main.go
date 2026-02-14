package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Invoice model
type Invoice struct {
	ID            string `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int `gorm:"default:1"`
	InvoiceNumber string
	InvoiceDate   time.Time
	CustomerID    string
	CustomerName  string
	SubtotalBHD   float64
	GrandTotalBHD float64
	OutstandingBHD float64
	Status        string `gorm:"default:'Paid'"`
}

func (Invoice) TableName() string { return "invoices" }

// SupplierInvoice model
type SupplierInvoice struct {
	ID            string `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Version       int `gorm:"default:1"`
	InvoiceNumber string
	InvoiceDate   time.Time
	DueDate       time.Time
	SupplierID    string
	SupplierName  string
	SubtotalBHD   float64
	VATBHD        float64 `gorm:"column:vatbhd"`
	TotalBHD      float64
	Status        string `gorm:"default:'Paid'"`
	PaymentStatus string `gorm:"default:'Paid'"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

// Customer model
type Customer struct {
	ID           string `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int `gorm:"default:1"`
	CustomerID   string
	BusinessName string
	CustomerType string `gorm:"default:'Corporate'"`
	Country      string `gorm:"default:'Bahrain'"`
}

func (Customer) TableName() string { return "customers" }

// Supplier model
type Supplier struct {
	ID           string `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Version      int `gorm:"default:1"`
	SupplierCode string
	SupplierName string
	Country      string `gorm:"default:'International'"`
}

func (Supplier) TableName() string { return "suppliers" }

func main() {
	log.Println("🔧 2023 & 2024 Tally Data Import Tool")

	// Connect to database
	dbPath := "ph_holdings.db"
	if _, err := os.Stat(filepath.Join("build", "bin", "data", "ph_holdings.db")); err == nil {
		dbPath = filepath.Join("build", "bin", "data", "ph_holdings.db")
	}
	log.Printf("📂 Database: %s", dbPath)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}

	// Import 2023 data
	log.Println("\n" + strings.Repeat("=", 50))
	log.Println("IMPORTING 2023 DATA")
	log.Println(strings.Repeat("=", 50))

	invoicesPath := filepath.Join("Data_for_database", "Tally_data", "invoices 2023.xlsx")
	if _, err := os.Stat(invoicesPath); err == nil {
		log.Println("\n📋 Importing invoices 2023.xlsx...")
		importInvoices(db, invoicesPath, 2023)
	}

	purchasesPath := filepath.Join("Data_for_database", "Tally_data", "purchase 2023.xlsx")
	if _, err := os.Stat(purchasesPath); err == nil {
		log.Println("\n📋 Importing purchase 2023.xlsx...")
		importPurchases(db, purchasesPath, 2023)
	}

	// Import 2024 data
	log.Println("\n" + strings.Repeat("=", 50))
	log.Println("IMPORTING 2024 DATA")
	log.Println(strings.Repeat("=", 50))

	invoicesPath = filepath.Join("Data_for_database", "Tally_data", "invoices 2024.xlsx")
	if _, err := os.Stat(invoicesPath); err == nil {
		log.Println("\n📋 Importing invoices 2024.xlsx...")
		importInvoices(db, invoicesPath, 2024)
	}

	purchasesPath = filepath.Join("Data_for_database", "Tally_data", "purchase 2024.xlsx")
	if _, err := os.Stat(purchasesPath); err == nil {
		log.Println("\n📋 Importing purchase 2024.xlsx...")
		importPurchases(db, purchasesPath, 2024)
	}

	// Verify counts
	log.Println("\n" + strings.Repeat("=", 50))
	log.Println("FINAL VERIFICATION")
	log.Println(strings.Repeat("=", 50))

	for _, year := range []string{"2023", "2024", "2025"} {
		var invCount int64
		db.Model(&Invoice{}).Where("strftime('%Y', invoice_date) = ?", year).Count(&invCount)
		var siCount int64
		db.Model(&SupplierInvoice{}).Where("strftime('%Y', invoice_date) = ?", year).Count(&siCount)
		log.Printf("   %s: %d invoices, %d supplier invoices", year, invCount, siCount)
	}

	log.Println("\n✅ Import complete!")
}

func importInvoices(db *gorm.DB, path string, year int) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		log.Printf("❌ Failed to open file: %v", err)
		return
	}
	defer f.Close()

	// Get first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		log.Printf("❌ No sheets found in file")
		return
	}
	sheetName := sheets[0]
	log.Printf("   Using sheet: %s", sheetName)

	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Printf("❌ Failed to read rows: %v", err)
		return
	}

	log.Printf("   Found %d rows", len(rows))

	// Tally Voucher Register structure:
	// Rows 1-7: Company info and report metadata
	// Row 8 (index 7): Column headers
	// Row 9+ (index 8+): Data
	if len(rows) < 10 {
		log.Printf("❌ Not enough rows in file (need at least 10)")
		return
	}

	// Column positions based on Tally export (fixed structure):
	// [0]=Date, [1]=Particulars (Customer), [2]=TRN, [3]=Vch Type,
	// [4]=Vch No. (Invoice#), [5]=Supplier Inv/Ref, [6]=Taxable Value, [7]=Tax Amount
	colDate := 0
	colParty := 1     // Particulars = Customer name
	colInvNo := 4     // Vch No. = Invoice number
	colTaxable := 6   // Taxable Value
	colVAT := 7       // Tax Amount

	log.Printf("   Header row (8): %v", rows[7])
	log.Printf("   First data row (9): %v", rows[8])
	log.Printf("   Using columns: Date=%d, Party=%d, InvNo=%d, Taxable=%d, VAT=%d", colDate, colParty, colInvNo, colTaxable, colVAT)

	// Customer cache
	customerMap := make(map[string]string) // name -> id

	imported := 0
	skipped := 0
	for i, row := range rows[8:] { // Start from row 9 (index 8)
		if len(row) < 5 {
			continue
		}

		// Extract values
		dateStr := safeGet(row, colDate)
		invNo := safeGet(row, colInvNo)
		party := strings.TrimSpace(safeGet(row, colParty))
		taxableStr := safeGet(row, colTaxable)
		vatStr := safeGet(row, colVAT)

		if party == "" || invNo == "" {
			continue
		}

		// Parse date
		invDate, _ := parseDate(dateStr)
		if invDate.IsZero() {
			invDate = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		}

		// Parse amounts
		taxable := parseAmount(taxableStr)
		vat := parseAmount(vatStr)
		total := taxable + vat

		// Get or create customer
		custID, ok := customerMap[party]
		if !ok {
			custID = findOrCreateCustomer(db, party)
			customerMap[party] = custID
		}

		// Check if invoice exists
		var exists int64
		db.Model(&Invoice{}).Where("invoice_number = ?", invNo).Count(&exists)
		if exists > 0 {
			skipped++
			continue
		}

		// Create invoice
		inv := Invoice{
			ID:            uuid.New().String(),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			InvoiceNumber: invNo,
			InvoiceDate:   invDate,
			CustomerID:    custID,
			CustomerName:  party,
			SubtotalBHD:   taxable,
			GrandTotalBHD: total,
			OutstandingBHD: 0, // Historical - assume paid
			Status:        "Paid",
		}

		if err := db.Create(&inv).Error; err != nil {
			log.Printf("   Row %d: Failed to create invoice %s: %v", i+2, invNo, err)
		} else {
			imported++
		}
	}

	log.Printf("   ✅ Imported %d invoices, skipped %d duplicates", imported, skipped)
}

func importPurchases(db *gorm.DB, path string, year int) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		log.Printf("❌ Failed to open file: %v", err)
		return
	}
	defer f.Close()

	// Get first sheet name
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		log.Printf("❌ No sheets found in file")
		return
	}
	sheetName := sheets[0]
	log.Printf("   Using sheet: %s", sheetName)

	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Printf("❌ Failed to read rows: %v", err)
		return
	}

	log.Printf("   Found %d rows", len(rows))

	// Tally Purchase Register structure:
	// Rows 1-6: Company info and report metadata
	// Row 7-8 (index 6-7): Column headers (split across 2 rows)
	// Row 9+ (index 8+): Data
	if len(rows) < 10 {
		log.Printf("❌ Not enough rows in file (need at least 10)")
		return
	}

	// Column positions based on Tally export (fixed structure):
	// [0]=Date, [1]=Particulars (Supplier), [2]=Vch Type, [3]=Vch No., [4]=Debit Amount, [5]=Credit Amount
	colDate := 0
	colParty := 1     // Particulars = Supplier name
	colVchNo := 3     // Vch No. = Reference number
	colAmount := 5    // Credit Amount = Purchase value

	log.Printf("   Header row (7): %v", rows[6])
	log.Printf("   First data row (9): %v", rows[8])
	log.Printf("   Using columns: Date=%d, Party=%d, VchNo=%d, Amount=%d", colDate, colParty, colVchNo, colAmount)

	// Supplier cache and dedup tracking
	supplierMap := make(map[string]string) // name -> id
	seen := make(map[string]bool)          // voucher number dedup

	imported := 0
	skipped := 0
	for i, row := range rows[8:] { // Start from row 9 (index 8)
		if len(row) < 3 {
			continue
		}

		// Extract values
		dateStr := safeGet(row, colDate)
		vchNo := strings.TrimSpace(safeGet(row, colVchNo))
		party := strings.TrimSpace(safeGet(row, colParty))
		amountStr := safeGet(row, colAmount)

		if party == "" || vchNo == "" {
			continue
		}

		// Skip duplicates (Tally exports double-entry)
		key := vchNo + "|" + party
		if seen[key] {
			skipped++
			continue
		}
		seen[key] = true

		// Parse date
		invDate, _ := parseDate(dateStr)
		if invDate.IsZero() {
			invDate = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		}

		// Parse amount
		amount := parseAmount(amountStr)
		if amount <= 0 {
			continue
		}

		// Get or create supplier
		suppID, ok := supplierMap[party]
		if !ok {
			suppID = findOrCreateSupplier(db, party)
			supplierMap[party] = suppID
		}

		// Create unique invoice number
		// Format: SI-{year}-{SupplierInitial}-{VchNo} to avoid conflicts
		supplierInitial := strings.ToUpper(string(party[0]))
		if len(party) > 1 {
			supplierInitial = strings.ToUpper(party[:2])
		}
		uniqueInvNo := fmt.Sprintf("SI-%d-%s-%s", year, supplierInitial, vchNo)

		// Check if invoice exists
		var exists int64
		db.Model(&SupplierInvoice{}).Where("invoice_number = ?", uniqueInvNo).Count(&exists)
		if exists > 0 {
			skipped++
			continue
		}

		// Create supplier invoice
		si := SupplierInvoice{
			ID:            uuid.New().String(),
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			InvoiceNumber: uniqueInvNo,
			InvoiceDate:   invDate,
			DueDate:       invDate.AddDate(0, 1, 0),
			SupplierID:    suppID,
			SupplierName:  party,
			SubtotalBHD:   amount,
			VATBHD:        0,
			TotalBHD:      amount,
			Status:        "Paid",
			PaymentStatus: "Paid",
		}

		if err := db.Create(&si).Error; err != nil {
			log.Printf("   Row %d: Failed to create supplier invoice %s: %v", i+2, vchNo, err)
		} else {
			imported++
		}
	}

	log.Printf("   ✅ Imported %d supplier invoices, skipped %d (duplicates/double-entry)", imported, skipped)
}

func safeGet(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return row[idx]
	}
	return ""
}

func parseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	formats := []string{
		"2-Jan-06", "02-Jan-06", "2-Jan-2006", "02-Jan-2006",
		"2006-01-02", "01/02/2006", "1/2/2006",
		"2-1-2006", "02-01-2006",
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date: %s", s)
}

func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	var amount float64
	fmt.Sscanf(s, "%f", &amount)
	if amount < 0 {
		amount = -amount
	}
	return amount
}

func findOrCreateCustomer(db *gorm.DB, name string) string {
	normalized := normalizeCustomerName(name)

	var cust Customer
	if err := db.Where("business_name = ?", normalized).First(&cust).Error; err == nil {
		return cust.ID
	}

	// Create new customer
	cust = Customer{
		ID:           uuid.New().String(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		CustomerID:   fmt.Sprintf("CUST-%d", time.Now().UnixNano()%10000),
		BusinessName: normalized,
		CustomerType: "Corporate",
		Country:      "Bahrain",
	}
	db.Create(&cust)
	return cust.ID
}

func findOrCreateSupplier(db *gorm.DB, name string) string {
	normalized := normalizeSupplierName(name)

	var supp Supplier
	if err := db.Where("supplier_name = ?", normalized).First(&supp).Error; err == nil {
		return supp.ID
	}

	// Create new supplier
	supp = Supplier{
		ID:           uuid.New().String(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		SupplierCode: fmt.Sprintf("SUPP-%d", time.Now().UnixNano()%10000),
		SupplierName: normalized,
		Country:      "International",
	}
	db.Create(&supp)
	return supp.ID
}

func normalizeCustomerName(name string) string {
	name = strings.TrimSpace(name)
	// Common normalizations
	replacements := map[string]string{
		"ALUMINIUM BAHRAIN B.S,C":  "Aluminium Bahrain B.S.C",
		"ALUMINIUM BAHRAIN B.S.C":  "Aluminium Bahrain B.S.C",
		"Electricity and Water Authority": "Electricity & Water Authority (EWA)",
	}
	for old, new := range replacements {
		if strings.EqualFold(name, old) {
			return new
		}
	}
	return name
}

func normalizeSupplierName(name string) string {
	name = strings.TrimSpace(name)
	// Common normalizations
	if strings.Contains(strings.ToLower(name), "endress") {
		return "Endress+Hauser"
	}
	if strings.Contains(strings.ToLower(name), "landis") {
		return "Landis+Gyr"
	}
	return name
}
