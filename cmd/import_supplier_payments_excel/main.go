package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SupplierPayment model
type SupplierPayment struct {
	ID             string    `gorm:"primaryKey;size:36"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	SupplierID     string    `gorm:"size:36" json:"supplier_id"`
	SupplierName   string    `gorm:"size:255" json:"supplier_name"`
	PaymentDate    time.Time `json:"payment_date"`
	AmountBHD      float64   `json:"amount_bhd"`
	AmountForeign  float64   `json:"amount_foreign"`
	Currency       string    `gorm:"size:10" json:"currency"`
	PaymentMethod  string    `gorm:"size:50" json:"payment_method"`
	ReferenceNo    string    `gorm:"size:100" json:"reference_no"`
	Notes          string    `json:"notes"`
	InvoiceNumbers string    `gorm:"size:500" json:"invoice_numbers"`
}

func (SupplierPayment) TableName() string { return "supplier_payments" }

// Supplier for lookups
type Supplier struct {
	ID           string `gorm:"primaryKey;size:36"`
	BusinessName string `gorm:"size:255" json:"business_name"`
}

func (Supplier) TableName() string { return "suppliers" }

// Known supplier name mappings
var supplierMappings = map[string]string{
	"L&G MTE":            "Landis+Gyr",
	"LG MTE":             "Landis+Gyr",
	"LANDIS":             "Landis+Gyr",
	"Enerdat":            "Enerdat",
	"ENERDAT":            "Enerdat",
	"E+H":                "Endress+Hauser",
	"ENDRESS":            "Endress+Hauser",
	"Servomex":           "Servomex",
	"SERVOMEX":           "Servomex",
	"Krohne":             "Krohne",
	"KROHNE":             "Krohne",
	"Yokogawa":           "Yokogawa",
	"YOKOGAWA":           "Yokogawa",
	"ABB":                "ABB",
	"Siemens":            "Siemens",
	"SIEMENS":            "Siemens",
	"Emerson":            "Emerson",
	"EMERSON":            "Emerson",
	"Honeywell":          "Honeywell",
	"HONEYWELL":          "Honeywell",
	"Schneider":          "Schneider Electric",
	"SCHNEIDER":          "Schneider Electric",
	"Phoenix":            "Phoenix Contact",
	"PHOENIX":            "Phoenix Contact",
	"Danfoss":            "Danfoss",
	"DANFOSS":            "Danfoss",
	"Pepperl":            "Pepperl+Fuchs",
	"PEPPERL":            "Pepperl+Fuchs",
	"IFM":                "IFM Electronic",
	"ifm":                "IFM Electronic",
	"Turck":              "Turck",
	"TURCK":              "Turck",
	"Wika":               "Wika",
	"WIKA":               "Wika",
	"Rosemount":          "Rosemount",
	"ROSEMOUNT":          "Rosemount",
	"Fisher":             "Fisher",
	"FISHER":             "Fisher",
}

func main() {
	excelPath := "Data_for_database/Payments to suppliers.xlsx"
	dbPath := "ph_holdings.db"
	analyzeOnly := false

	if len(os.Args) > 1 {
		if os.Args[1] == "--analyze" {
			analyzeOnly = true
		} else {
			excelPath = os.Args[1]
		}
	}
	if len(os.Args) > 2 {
		dbPath = os.Args[2]
	}

	fmt.Println("📊 Supplier Payments Import Tool")
	fmt.Println("================================")
	fmt.Printf("Excel: %s\n", excelPath)
	fmt.Printf("Database: %s\n\n", dbPath)

	// Open Excel file
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		log.Fatalf("Failed to open Excel: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	fmt.Printf("Found %d sheets:\n", len(sheets))
	for i, s := range sheets {
		fmt.Printf("  %d. %s\n", i+1, s)
	}
	fmt.Println()

	// Identify supplier sheets vs customer sheets
	supplierSheets := []string{}
	customerSheets := []string{}

	for _, sheet := range sheets {
		sheetLower := strings.ToLower(sheet)
		if strings.Contains(sheetLower, "sale") ||
			strings.Contains(sheetLower, "customer") ||
			strings.Contains(sheetLower, "receipt") {
			customerSheets = append(customerSheets, sheet)
		} else {
			supplierSheets = append(supplierSheets, sheet)
		}
	}

	fmt.Println("📋 Sheet Classification:")
	fmt.Println("  SUPPLIER PAYMENT SHEETS (will import):")
	for _, s := range supplierSheets {
		fmt.Printf("    ✅ %s\n", s)
	}
	fmt.Println("  CUSTOMER SHEETS (skipping):")
	for _, s := range customerSheets {
		fmt.Printf("    ⏭️ %s\n", s)
	}
	fmt.Println()

	if analyzeOnly {
		// Just analyze structure
		for _, sheet := range supplierSheets {
			analyzeSheet(f, sheet)
		}
		return
	}

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Load suppliers for lookup
	var suppliers []Supplier
	db.Find(&suppliers)
	supplierMap := make(map[string]string) // name -> ID
	for _, s := range suppliers {
		supplierMap[strings.ToLower(s.BusinessName)] = s.ID
	}
	fmt.Printf("Loaded %d suppliers for lookup\n\n", len(suppliers))

	// Import each supplier sheet
	totalImported := 0
	totalSkipped := 0
	totalErrors := 0

	for _, sheet := range supplierSheets {
		imported, skipped, errors := importSheet(db, f, sheet, supplierMap)
		totalImported += imported
		totalSkipped += skipped
		totalErrors += errors
	}

	fmt.Println("\n================================")
	fmt.Println("📊 Import Summary")
	fmt.Println("================================")
	fmt.Printf("Total Imported: %d\n", totalImported)
	fmt.Printf("Total Skipped:  %d (duplicates/empty)\n", totalSkipped)
	fmt.Printf("Total Errors:   %d\n", totalErrors)
	fmt.Println("================================")
}

func analyzeSheet(f *excelize.File, sheet string) {
	fmt.Printf("\n=== Analyzing: %s ===\n", sheet)
	rows, err := f.GetRows(sheet)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Total rows: %d\n", len(rows))
	fmt.Println("First 10 rows:")
	for i := 0; i < 10 && i < len(rows); i++ {
		row := rows[i]
		preview := []string{}
		for j := 0; j < len(row) && j < 6; j++ {
			v := strings.TrimSpace(row[j])
			if len(v) > 20 {
				v = v[:17] + "..."
			}
			preview = append(preview, v)
		}
		fmt.Printf("  Row %2d: %v\n", i+1, preview)
	}
}

func importSheet(db *gorm.DB, f *excelize.File, sheet string, supplierMap map[string]string) (imported, skipped, errors int) {
	fmt.Printf("\n📥 Importing: %s\n", sheet)

	rows, err := f.GetRows(sheet)
	if err != nil {
		fmt.Printf("  ❌ Error reading sheet: %v\n", err)
		return 0, 0, 1
	}

	if len(rows) < 2 {
		fmt.Println("  ⏭️ Sheet is empty")
		return 0, 0, 0
	}

	// Detect header row and column mapping
	headerRow := -1
	colMap := make(map[string]int) // column name -> index

	for i, row := range rows {
		rowLower := strings.ToLower(strings.Join(row, " "))
		if strings.Contains(rowLower, "date") &&
			(strings.Contains(rowLower, "amount") || strings.Contains(rowLower, "bhd") || strings.Contains(rowLower, "payment")) {
			headerRow = i
			for j, cell := range row {
				cellLower := strings.ToLower(strings.TrimSpace(cell))
				if strings.Contains(cellLower, "date") {
					colMap["date"] = j
				}
				if strings.Contains(cellLower, "amount") || strings.Contains(cellLower, "bhd") {
					if _, exists := colMap["amount"]; !exists {
						colMap["amount"] = j
					}
				}
				if strings.Contains(cellLower, "invoice") || strings.Contains(cellLower, "inv") {
					colMap["invoice"] = j
				}
				if strings.Contains(cellLower, "reference") || strings.Contains(cellLower, "ref") || strings.Contains(cellLower, "cheque") || strings.Contains(cellLower, "check") {
					colMap["reference"] = j
				}
				if strings.Contains(cellLower, "remark") || strings.Contains(cellLower, "note") || strings.Contains(cellLower, "description") {
					colMap["notes"] = j
				}
				if strings.Contains(cellLower, "currency") || strings.Contains(cellLower, "curr") {
					colMap["currency"] = j
				}
				if strings.Contains(cellLower, "foreign") {
					colMap["foreign"] = j
				}
			}
			break
		}
	}

	if headerRow == -1 {
		fmt.Println("  ⚠️ Could not find header row")
		return 0, 0, 1
	}

	fmt.Printf("  Header row: %d, Columns: %v\n", headerRow+1, colMap)

	// Determine supplier name from sheet name
	supplierName := guessSupplierName(sheet)
	supplierID := findSupplierID(supplierName, supplierMap)

	fmt.Printf("  Supplier: %s (ID: %s)\n", supplierName, supplierID)

	// Process data rows
	for i := headerRow + 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		// Parse date
		dateStr := getCell(row, colMap["date"])
		paymentDate := parseDate(dateStr)
		if paymentDate.IsZero() {
			skipped++
			continue
		}

		// Parse amount
		amountStr := getCell(row, colMap["amount"])
		amount := parseAmount(amountStr)
		if amount == 0 {
			skipped++
			continue
		}

		// Parse optional fields
		invoiceNums := getCell(row, colMap["invoice"])
		reference := getCell(row, colMap["reference"])
		notes := getCell(row, colMap["notes"])
		currency := getCell(row, colMap["currency"])
		foreignStr := getCell(row, colMap["foreign"])
		foreignAmt := parseAmount(foreignStr)

		if currency == "" {
			currency = "BHD"
		}

		// Check for duplicate
		var existing SupplierPayment
		if db.Where("supplier_name = ? AND payment_date = ? AND amount_bhd = ?",
			supplierName, paymentDate, amount).First(&existing).Error == nil {
			skipped++
			continue
		}

		// Create payment record
		payment := SupplierPayment{
			ID:             uuid.New().String(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
			SupplierID:     supplierID,
			SupplierName:   supplierName,
			PaymentDate:    paymentDate,
			AmountBHD:      amount,
			AmountForeign:  foreignAmt,
			Currency:       currency,
			PaymentMethod:  "Bank Transfer",
			ReferenceNo:    reference,
			Notes:          notes,
			InvoiceNumbers: invoiceNums,
		}

		if err := db.Create(&payment).Error; err != nil {
			fmt.Printf("  ❌ Row %d error: %v\n", i+1, err)
			errors++
			continue
		}
		imported++
	}

	fmt.Printf("  ✅ Imported: %d, Skipped: %d\n", imported, skipped)
	return
}

func guessSupplierName(sheetName string) string {
	// Check mappings
	for key, name := range supplierMappings {
		if strings.Contains(strings.ToUpper(sheetName), strings.ToUpper(key)) {
			return name
		}
	}
	// Clean up sheet name as fallback
	name := strings.TrimSpace(sheetName)
	name = regexp.MustCompile(`[^a-zA-Z0-9\s&+]`).ReplaceAllString(name, "")
	return strings.TrimSpace(name)
}

func findSupplierID(name string, supplierMap map[string]string) string {
	nameLower := strings.ToLower(name)
	if id, ok := supplierMap[nameLower]; ok {
		return id
	}
	// Partial match
	for supplierName, id := range supplierMap {
		if strings.Contains(supplierName, nameLower) || strings.Contains(nameLower, supplierName) {
			return id
		}
	}
	return ""
}

func getCell(row []string, idx int) string {
	if idx >= 0 && idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	// Try various formats
	formats := []string{
		"2-Jan-06",
		"02-Jan-06",
		"2-Jan-2006",
		"02-Jan-2006",
		"02/01/2006",
		"2/1/2006",
		"01/02/2006",
		"1/2/2006",
		"2006-01-02",
		"02-01-2006",
		"2-1-2006",
		"Jan 2, 2006",
		"January 2, 2006",
	}

	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}

	// Try Excel serial date
	if serial, err := strconv.ParseFloat(s, 64); err == nil && serial > 1 {
		// Excel serial date: days since Jan 1, 1900
		return time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, int(serial))
	}

	return time.Time{}
}

func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.ReplaceAll(s, "bhd", "")
	s = strings.TrimSpace(s)

	// Handle negative in parentheses
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = "-" + s[1:len(s)-1]
	}

	v, _ := strconv.ParseFloat(s, 64)
	if v < 0 {
		v = -v // Convert to positive for payments
	}
	return v
}
