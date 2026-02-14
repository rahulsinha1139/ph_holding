package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Extracted customer data structure
type CustomerData struct {
	SourceFile      string   `json:"source_file"`
	FolderNumber    string   `json:"folder_number"`
	CustomerName    string   `json:"customer_name"`
	CustomerAbbr    string   `json:"customer_abbr"`
	ContactPerson   string   `json:"contact_person,omitempty"`
	TaxCode         string   `json:"tax_code,omitempty"`
	VATNumber       string   `json:"vat_number,omitempty"`
	Address         string   `json:"address,omitempty"`
	Phone           string   `json:"phone,omitempty"`
	Email           string   `json:"email,omitempty"`
	RFQNumber       string   `json:"rfq_number,omitempty"`
	PONumber        string   `json:"po_number,omitempty"`
	Salesperson     string   `json:"salesperson,omitempty"`
	Date            string   `json:"date,omitempty"`
	Supplier        string   `json:"supplier,omitempty"`
	ProductType     string   `json:"product_type,omitempty"`
	TotalValue      float64  `json:"total_value"`
	CurrencyCode    string   `json:"currency_code,omitempty"`
	PaymentTerms    string   `json:"payment_terms,omitempty"`
	DeliveryTerms   string   `json:"delivery_terms,omitempty"`
	Validity        string   `json:"validity,omitempty"`
	RawFields       []string `json:"raw_fields,omitempty"`
}

// Database models
type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
	CreatedBy string
}

type CustomerMaster struct {
	Base
	CustomerID    string `gorm:"uniqueIndex;size:50"`
	CustomerCode  string `gorm:"uniqueIndex;size:50"`
	CustomerType  string `gorm:"index;size:50"`
	BusinessName  string `gorm:"index;size:255"`
	ShortCode     string `gorm:"size:10"`
	City          string `gorm:"index;size:100"`
	Country       string `gorm:"index;size:100"`
	Industry      string `gorm:"index;size:100"`
	CustomerGrade string `gorm:"index;size:10"`
	PaymentGrade  string `gorm:"index;size:10"`
	TaxCode       string `gorm:"size:50"`
	VATNumber     string `gorm:"size:50"`
	Address       string `gorm:"size:500"`
	Phone         string `gorm:"size:50"`
	Email         string `gorm:"size:100"`
}

func (CustomerMaster) TableName() string { return "customers" }

type CustomerContact struct {
	Base
	CustomerID   string `gorm:"index;size:36"`
	ContactName  string `gorm:"size:255"`
	ContactRole  string `gorm:"size:100"`
	ContactEmail string `gorm:"size:100"`
	ContactPhone string `gorm:"size:50"`
	IsPrimary    bool   `gorm:"default:false"`
}

func (CustomerContact) TableName() string { return "customer_contacts" }

type Opportunity struct {
	Base
	FolderNumber string    `gorm:"uniqueIndex;size:50"`
	CustomerID   string    `gorm:"index;size:36"`
	CustomerName string    `gorm:"index;size:255"`
	Salesperson  string    `gorm:"index;size:100"`
	OfferDate    time.Time `gorm:"index"`
	RevenueBHD   float64
	Stage        string `gorm:"index;size:50"`
	ProductType  string `gorm:"index;size:100"`
}

func (Opportunity) TableName() string { return "opportunities" }

// Customer name mappings
var customerMappings = map[string]string{
	"EWA":        "Electricity & Water Authority",
	"ARLA":       "Arla Foods Bahrain S.P.C",
	"ALBA":       "Aluminium Bahrain B.S.C.(C)",
	"BAPCO":      "Bapco Energies",
	"GPIC":       "Gulf Petrochemical Industries Co.",
	"SULB":       "SULB Company B.S.C.",
	"ENGIE":      "Al Ezzel Engie O&M Co. W.L.L",
	"TTSJV":      "Technip-Technicas Reunidas JV",
	"SEAPEAK":    "Seapeak Maritime",
	"MOW":        "Ministry of Works",
	"ALUSERV":    "Aluserv Middle East W.L.L.",
	"INTERCOL":   "Intercol",
	"VEOLIA":     "Veolia Water Technologies",
	"NOMAC":      "Nomac Bahrain",
	"BANAGAS":    "Banagas Co. B.S.C.",
	"YOKOGAWA":   "Yokogawa Middle East",
	"BSR":        "Bahrain Sugar Refinery",
	"MUHARRAQ":   "Muharraq Wastewater Services",
	"WEST POINT": "West Point Engineering",
	"WESTPOINT":  "West Point Engineering",
	"TECTON":     "Tecton Engineering",
	"KRAUM":      "Kraum & Spruk Technologies",
	"YATEEM":     "Yateem Group",
	"AIC":        "Arabian Industrial Chemicals",
	"AL FARIS":   "Al Faris Equipment Rental",
	"ALFARIS":    "Al Faris Equipment Rental",
	"COMSIP":     "Comsip Bahrain",
	"AL BAKALI":  "Al Bakali Construction",
	"DELTA":      "Delta Engineering",
	"TATWEER":    "Tatweer Petroleum",
	"TABREED":    "Tabreed",
	"BU":         "Bapco Energies",
	"BAPCO UPSTREAM": "Bapco Energies",
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	costingDir := "Costing_Sheets"
	dbFile := "ph_holdings.db"

	fmt.Println("🔍 OCR Extraction from Costing Sheets...")
	fmt.Printf("📁 Source: %s\n\n", costingDir)

	// Get all Excel files
	files, err := filepath.Glob(filepath.Join(costingDir, "*.xlsx"))
	if err != nil {
		log.Fatalf("Failed to glob: %v", err)
	}

	fmt.Printf("📊 Found %d Excel files\n\n", len(files))

	var allData []CustomerData
	customerSet := make(map[string]*CustomerData)

	for _, file := range files {
		data, err := extractFromCostingSheet(file)
		if err != nil {
			fmt.Printf("⚠️ Error processing %s: %v\n", filepath.Base(file), err)
			continue
		}

		if data.CustomerName != "" {
			allData = append(allData, data)
			// Keep the most complete record per customer
			if existing, ok := customerSet[data.CustomerName]; ok {
				mergeCustomerData(existing, &data)
			} else {
				copy := data
				customerSet[data.CustomerName] = &copy
			}
			fmt.Printf("✅ %s\n", filepath.Base(file))
			fmt.Printf("   Customer: %s\n", data.CustomerName)
			if data.ContactPerson != "" {
				fmt.Printf("   Contact: %s\n", data.ContactPerson)
			}
			if data.TaxCode != "" || data.VATNumber != "" {
				fmt.Printf("   Tax/VAT: %s / %s\n", data.TaxCode, data.VATNumber)
			}
			if data.TotalValue > 0 {
				fmt.Printf("   Value: %.3f BHD\n", data.TotalValue)
			}
		}
	}

	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   Files processed: %d\n", len(allData))
	fmt.Printf("   Unique customers: %d\n", len(customerSet))

	// Save extracted data to JSON
	jsonData, _ := json.MarshalIndent(allData, "", "  ")
	os.WriteFile("extracted_customer_data.json", jsonData, 0644)
	fmt.Println("   Saved to: extracted_customer_data.json")

	// Update database
	fmt.Println("\n🗄️ Updating database...")
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Ensure tables exist with correct schema
	db.AutoMigrate(&CustomerMaster{}, &CustomerContact{}, &Opportunity{})

	// Clear and reseed
	db.Exec("DELETE FROM customers")
	db.Exec("DELETE FROM opportunities")
	db.Exec("DELETE FROM customer_contacts")

	// Insert customers with full details (skip invalid names)
	customerIDs := make(map[string]string)
	for customerName, data := range customerSet {
		// Skip very short or invalid names
		if len(customerName) < 3 || customerName == "Bu" || customerName == "The" {
			continue
		}
		id := uuid.New().String()
		code := generateCode(customerName)

		customer := CustomerMaster{
			Base: Base{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			CustomerID:    id,
			CustomerCode:  code,
			BusinessName:  customerName,
			CustomerType:  "Industrial",
			Country:       "Bahrain",
			City:          "Manama",
			Industry:      "Process Instrumentation",
			CustomerGrade: "B",
			PaymentGrade:  "B",
			TaxCode:       data.TaxCode,
			VATNumber:     data.VATNumber,
			Address:       data.Address,
			Phone:         data.Phone,
			Email:         data.Email,
		}
		db.Create(&customer)
		customerIDs[customerName] = id

		// Add contact if available
		if data.ContactPerson != "" {
			contact := CustomerContact{
				Base: Base{
					ID:        uuid.New().String(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				CustomerID:  id,
				ContactName: data.ContactPerson,
				ContactRole: "Primary Contact",
				IsPrimary:   true,
			}
			db.Create(&contact)
		}
	}
	fmt.Printf("   Created %d customers\n", len(customerSet))

	// Insert opportunities (dedupe by folder number)
	seenFolders := make(map[string]bool)
	oppCount := 0
	for _, data := range allData {
		if data.FolderNumber == "" || seenFolders[data.FolderNumber] {
			continue
		}
		seenFolders[data.FolderNumber] = true

		customerID := customerIDs[data.CustomerName]
		opp := Opportunity{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			FolderNumber: data.FolderNumber,
			CustomerID:   customerID,
			CustomerName: data.CustomerName,
			Salesperson:  data.Salesperson,
			OfferDate:    parseDate(data.Date),
			RevenueBHD:   data.TotalValue,
			Stage:        "Quoted",
			ProductType:  data.ProductType,
		}
		if err := db.Create(&opp).Error; err != nil {
			fmt.Printf("⚠️ Error creating opportunity %s: %v\n", data.FolderNumber, err)
		} else {
			oppCount++
		}
	}
	fmt.Printf("   Created %d opportunities\n", oppCount)

	// Copy to build
	input, _ := os.ReadFile(dbFile)
	os.WriteFile("build/bin/data/ph_holdings.db", input, 0644)
	fmt.Println("   Database deployed to build/bin/data/")

	fmt.Println("\n✨ OCR Extraction Complete!")
}

func extractFromCostingSheet(filePath string) (CustomerData, error) {
	data := CustomerData{
		SourceFile: filepath.Base(filePath),
	}

	// Extract folder number from filename
	re := regexp.MustCompile(`^(\d+)[-\s]`)
	if match := re.FindStringSubmatch(filepath.Base(filePath)); match != nil {
		data.FolderNumber = fmt.Sprintf("2025-%s", match[1])
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return data, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return data, fmt.Errorf("no sheets found")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return data, err
	}

	// Scan all rows for key fields
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		firstCell := strings.ToUpper(strings.TrimSpace(row[0]))

		// Customer name - check various patterns
		if containsAny(firstCell, []string{"CUSTOMER", "CLIENT", "BUYER", "END USER"}) {
			if len(row) > 1 && strings.TrimSpace(row[1]) != "" {
				data.CustomerName = normalizeCustomer(strings.TrimSpace(row[1]))
			}
		}

		// Contact person
		if containsAny(firstCell, []string{"CONTACT", "ATTN", "ATTENTION", "PERSON"}) {
			if len(row) > 1 && strings.TrimSpace(row[1]) != "" {
				data.ContactPerson = strings.TrimSpace(row[1])
			}
		}

		// Tax code / VAT
		if containsAny(firstCell, []string{"TAX", "TIN", "TAX ID", "TAX CODE"}) {
			if len(row) > 1 {
				data.TaxCode = strings.TrimSpace(row[1])
			}
		}
		if containsAny(firstCell, []string{"VAT", "VAT NO", "VAT NUMBER"}) {
			if len(row) > 1 {
				data.VATNumber = strings.TrimSpace(row[1])
			}
		}

		// Address
		if containsAny(firstCell, []string{"ADDRESS", "LOCATION"}) {
			if len(row) > 1 {
				data.Address = strings.TrimSpace(row[1])
			}
		}

		// Phone
		if containsAny(firstCell, []string{"PHONE", "TEL", "MOBILE", "FAX"}) {
			if len(row) > 1 {
				data.Phone = strings.TrimSpace(row[1])
			}
		}

		// Email
		if containsAny(firstCell, []string{"EMAIL", "E-MAIL"}) {
			if len(row) > 1 {
				data.Email = strings.TrimSpace(row[1])
			}
		}

		// RFQ / PO
		if containsAny(firstCell, []string{"RFQ", "ENQUIRY", "INQUIRY"}) {
			if len(row) > 1 {
				data.RFQNumber = strings.TrimSpace(row[1])
			}
		}
		if containsAny(firstCell, []string{"PO NO", "P.O.", "PURCHASE ORDER"}) {
			if len(row) > 1 {
				data.PONumber = strings.TrimSpace(row[1])
			}
		}

		// Salesperson
		if containsAny(firstCell, []string{"COSTING MADE BY", "PREPARED BY", "SALESPERSON", "SALES"}) {
			if len(row) > 1 {
				data.Salesperson = strings.TrimSpace(row[1])
			}
		}

		// Date
		if containsAny(firstCell, []string{"DATE"}) && !containsAny(firstCell, []string{"DELIVERY", "DUE"}) {
			if len(row) > 1 {
				data.Date = strings.TrimSpace(row[1])
			}
		}

		// Supplier
		if containsAny(firstCell, []string{"SUPPLIER", "VENDOR", "MANUFACTURER"}) {
			if len(row) > 1 {
				data.Supplier = strings.TrimSpace(row[1])
			}
		}

		// Payment terms
		if containsAny(firstCell, []string{"PAYMENT", "TERMS"}) {
			if len(row) > 1 {
				data.PaymentTerms = strings.TrimSpace(row[1])
			}
		}

		// Total value - look for TOTAL rows with numeric values
		if strings.Contains(firstCell, "TOTAL") && i > 20 {
			for _, cell := range row {
				val := parseFloat(cell)
				if val > 1000 && val > data.TotalValue {
					data.TotalValue = val
				}
			}
		}

		// Also capture all non-empty fields for debugging
		for _, cell := range row {
			if strings.TrimSpace(cell) != "" {
				data.RawFields = append(data.RawFields, strings.TrimSpace(cell))
			}
		}
	}

	// Try to extract customer from filename if not found in content
	if data.CustomerName == "" {
		data.CustomerName = extractCustomerFromFilename(filepath.Base(filePath))
	}

	// Extract product type from filename
	data.ProductType = extractProductType(filepath.Base(filePath))

	return data, nil
}

func containsAny(s string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

func normalizeCustomer(name string) string {
	name = strings.TrimSpace(name)
	upper := strings.ToUpper(name)

	// Check mappings
	for abbr, fullName := range customerMappings {
		if strings.Contains(upper, abbr) {
			return fullName
		}
	}

	// Clean up common patterns
	name = strings.TrimSuffix(name, " spare")
	name = strings.TrimSuffix(name, " Spare")

	// Title case
	if len(name) > 0 {
		return strings.Title(strings.ToLower(name))
	}
	return name
}

func extractCustomerFromFilename(filename string) string {
	// Pattern: "XX customer costing" or "XX-Costing sheet"
	upper := strings.ToUpper(filename)

	for abbr, fullName := range customerMappings {
		if strings.Contains(upper, abbr) {
			return fullName
		}
	}

	return ""
}

func extractProductType(s string) string {
	productTypes := []string{"FIT", "LIT", "TIT", "AIT", "PIT", "DPT", "DPIT", "PT", "LS", "BTU", "SP", "ELE"}
	upper := strings.ToUpper(s)
	for _, pt := range productTypes {
		if strings.Contains(upper, pt) {
			return pt
		}
	}
	return ""
}

func mergeCustomerData(existing *CustomerData, new *CustomerData) {
	// Merge non-empty fields from new into existing
	if existing.ContactPerson == "" && new.ContactPerson != "" {
		existing.ContactPerson = new.ContactPerson
	}
	if existing.TaxCode == "" && new.TaxCode != "" {
		existing.TaxCode = new.TaxCode
	}
	if existing.VATNumber == "" && new.VATNumber != "" {
		existing.VATNumber = new.VATNumber
	}
	if existing.Address == "" && new.Address != "" {
		existing.Address = new.Address
	}
	if existing.Phone == "" && new.Phone != "" {
		existing.Phone = new.Phone
	}
	if existing.Email == "" && new.Email != "" {
		existing.Email = new.Email
	}
	if new.TotalValue > existing.TotalValue {
		existing.TotalValue = new.TotalValue
	}
}

func generateCode(name string) string {
	words := strings.Fields(name)
	code := ""
	for _, word := range words {
		if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
			code += string(word[0])
		}
		if len(code) >= 5 {
			break
		}
	}
	if code == "" {
		code = "CUST"
	}
	return code + "-" + uuid.New().String()[:4]
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{
		"02-01-2006",
		"2-Jan-2006",
		"02/01/2006",
		"2006-01-02",
		"02-Jan-06",
		"2-1-2006",
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
	s = strings.ReplaceAll(s, "BHD", "")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
