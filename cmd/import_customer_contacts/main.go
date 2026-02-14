// Customer Contacts Import Tool
// Imports contacts from Excel and enriches customer database
//
// Usage: go run cmd/import_customer_contacts/main.go <excel_file>
//
// Excel Expected Columns:
//   - Choose Contact Person (Contact name)
//   - Salutation (Mr./Ms.)
//   - Phone
//   - Email
//   - Address
//   - Business Account Name (Customer company)
//   - Customer Type (National Reseller, System Integrator, End Customer, Service Provider)

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

// CustomerMaster matches the main app's model
type CustomerMaster struct {
	ID               string         `gorm:"primaryKey;size:36"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
	CustomerID       string         `gorm:"uniqueIndex;size:50"`
	CustomerCode     string         `gorm:"uniqueIndex;size:50"`
	CustomerType     string         `gorm:"index;size:50"`
	BusinessName     string         `gorm:"index;size:255"`
	ShortCode        string         `gorm:"size:10"`
	AddressLine1     string
	City             string         `gorm:"index;size:100"`
	Country          string         `gorm:"index;size:100"`
	TRN              string         `gorm:"size:100"`
	Industry         string         `gorm:"index;size:100"`
	RelationYears    int
	PaymentGrade     string         `gorm:"index;size:10;default:'C'"`
	CustomerGrade    string         `gorm:"index;size:10;default:'C'"`
	PaymentTermsDays int            `gorm:"index"`
}

func (CustomerMaster) TableName() string { return "customers" }

// CustomerContact matches the main app's model
type CustomerContact struct {
	ID               string         `gorm:"primaryKey;size:36"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        gorm.DeletedAt `gorm:"index"`
	CustomerID       string         `gorm:"index;size:36"`
	ContactName      string         `gorm:"size:255"`
	JobTitle         string         `gorm:"size:100"`
	Email            string         `gorm:"size:255"`
	Phone            string         `gorm:"size:50"`
	Address          string         `gorm:"type:varchar(1000)"`
	IsPrimaryContact bool
	Salutation       string         `gorm:"size:20"`
}

func (CustomerContact) TableName() string { return "customer_contacts" }

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <excel_file>")
		fmt.Println("Example: go run main.go \"customer database.xlsx\"")
		os.Exit(1)
	}

	excelPath := os.Args[1]
	dbPath := "./ph_holdings.db"

	// Check if a different db path was provided
	if len(os.Args) >= 3 {
		dbPath = os.Args[2]
	}

	// SECURITY: Validate file paths - prevent path traversal
	// Clean paths to resolve any .. or symlinks
	cleanExcelPath, err := filepath.Abs(excelPath)
	if err != nil {
		log.Fatalf("❌ Invalid Excel path: %v", err)
	}
	cleanDbPath, err := filepath.Abs(dbPath)
	if err != nil {
		log.Fatalf("❌ Invalid database path: %v", err)
	}

	// Validate Excel file exists and has valid extension
	if !strings.HasSuffix(strings.ToLower(cleanExcelPath), ".xlsx") {
		log.Fatalf("❌ Excel file must have .xlsx extension")
	}
	if _, err := os.Stat(cleanExcelPath); os.IsNotExist(err) {
		log.Fatalf("❌ Excel file not found: %s", cleanExcelPath)
	}

	// Use cleaned paths
	excelPath = cleanExcelPath
	dbPath = cleanDbPath

	fmt.Printf("📊 Customer Contacts Import Tool\n")
	fmt.Printf("================================\n")
	fmt.Printf("Excel file: %s\n", excelPath)
	fmt.Printf("Database: %s\n\n", dbPath)

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}

	// Auto-migrate to ensure Salutation column exists
	db.AutoMigrate(&CustomerContact{})

	// Open Excel file
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		log.Fatalf("❌ Failed to open Excel file: %v", err)
	}
	defer f.Close()

	// Get the first sheet
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		log.Fatal("❌ No sheets found in Excel file")
	}
	sheetName := sheets[0]
	fmt.Printf("📄 Reading sheet: %s\n\n", sheetName)

	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Fatalf("❌ Failed to read rows: %v", err)
	}

	if len(rows) < 2 {
		log.Fatal("❌ No data rows found (only header?)")
	}

	// Parse header to find column indices
	header := rows[0]
	colMap := make(map[string]int)
	for i, col := range header {
		colName := strings.ToLower(strings.TrimSpace(col))
		colMap[colName] = i
	}

	// Expected columns (flexible matching)
	contactNameCol := findColumn(colMap, "choose contact person", "contact person", "contact name", "name")
	salutationCol := findColumn(colMap, "salutation", "title")
	phoneCol := findColumn(colMap, "phone", "telephone", "mobile")
	emailCol := findColumn(colMap, "email", "e-mail", "mail")
	addressCol := findColumn(colMap, "address", "location")
	businessCol := findColumn(colMap, "business account name", "account name", "business name", "company", "customer")
	typeCol := findColumn(colMap, "customer type", "type", "category")

	fmt.Printf("Column mapping:\n")
	fmt.Printf("  Contact Name: col %d\n", contactNameCol)
	fmt.Printf("  Salutation: col %d\n", salutationCol)
	fmt.Printf("  Phone: col %d\n", phoneCol)
	fmt.Printf("  Email: col %d\n", emailCol)
	fmt.Printf("  Address: col %d\n", addressCol)
	fmt.Printf("  Business Name: col %d\n", businessCol)
	fmt.Printf("  Customer Type: col %d\n\n", typeCol)

	// Track stats
	customersCreated := 0
	customersUpdated := 0
	contactsCreated := 0
	contactsSkipped := 0
	errors := 0

	// Process each row
	for i, row := range rows {
		if i == 0 {
			continue // Skip header
		}

		// Extract values safely
		contactName := getCell(row, contactNameCol)
		salutation := getCell(row, salutationCol)
		phone := getCell(row, phoneCol)
		email := getCell(row, emailCol)
		address := getCell(row, addressCol)
		businessName := getCell(row, businessCol)
		customerType := getCell(row, typeCol)

		// Skip empty rows
		if businessName == "" && contactName == "" {
			continue
		}

		if businessName == "" {
			fmt.Printf("⚠️ Row %d: Missing business name for contact %s, skipping\n", i+1, contactName)
			contactsSkipped++
			continue
		}

		// STEP 1: Find or Create Customer
		var customer CustomerMaster
		result := db.Where("business_name = ?", businessName).First(&customer)

		if result.Error == gorm.ErrRecordNotFound {
			// CREATE new customer from Excel data
			customer = CustomerMaster{
				ID:            uuid.New().String(),
				CustomerID:    generateCustomerID(businessName),
				CustomerCode:  generateCustomerCode(businessName),
				BusinessName:  businessName,
				CustomerGrade: mapCustomerType(customerType),
				Industry:      customerType,
				Country:       "Bahrain",
				City:          extractCity(address),
				AddressLine1:  address,
			}
			if err := db.Create(&customer).Error; err != nil {
				fmt.Printf("❌ Row %d: Failed to create customer %s: %v\n", i+1, businessName, err)
				errors++
				continue
			}
			customersCreated++
			fmt.Printf("✅ Created customer: %s (Grade: %s)\n", businessName, customer.CustomerGrade)
		} else if result.Error != nil {
			fmt.Printf("❌ Row %d: Database error for %s: %v\n", i+1, businessName, result.Error)
			errors++
			continue
		} else {
			// UPDATE existing customer with customer type if missing
			if customer.Industry == "" && customerType != "" {
				db.Model(&customer).Update("industry", customerType)
				customersUpdated++
			}
		}

		// STEP 2: Create Contact (skip if email already exists for this customer)
		if contactName == "" {
			continue // No contact to add
		}

		var existing CustomerContact
		if email != "" {
			db.Where("customer_id = ? AND email = ?", customer.ID, email).First(&existing)
		} else if phone != "" {
			db.Where("customer_id = ? AND phone = ?", customer.ID, phone).First(&existing)
		}

		if existing.ID != "" {
			contactsSkipped++
			continue // Contact already exists
		}

		// Check if this is the first contact for this customer
		var contactCount int64
		db.Model(&CustomerContact{}).Where("customer_id = ?", customer.ID).Count(&contactCount)

		contact := CustomerContact{
			ID:               uuid.New().String(),
			CustomerID:       customer.ID,
			ContactName:      contactName,
			Salutation:       salutation,
			Phone:            phone,
			Email:            email,
			Address:          address,
			IsPrimaryContact: contactCount == 0, // First contact is primary
		}

		if err := db.Create(&contact).Error; err != nil {
			fmt.Printf("❌ Row %d: Failed to create contact %s: %v\n", i+1, contactName, err)
			errors++
			continue
		}
		contactsCreated++
	}

	fmt.Printf("\n================================\n")
	fmt.Printf("📊 Import Summary\n")
	fmt.Printf("================================\n")
	fmt.Printf("Customers created: %d\n", customersCreated)
	fmt.Printf("Customers updated: %d\n", customersUpdated)
	fmt.Printf("Contacts created:  %d\n", contactsCreated)
	fmt.Printf("Contacts skipped:  %d (duplicates)\n", contactsSkipped)
	fmt.Printf("Errors:            %d\n", errors)
	fmt.Printf("================================\n")

	if errors == 0 {
		fmt.Println("✅ Import completed successfully!")
	} else {
		fmt.Printf("⚠️ Import completed with %d errors\n", errors)
	}
}

// findColumn finds a column index by trying multiple possible names
func findColumn(colMap map[string]int, names ...string) int {
	for _, name := range names {
		for key, idx := range colMap {
			if strings.Contains(key, name) {
				return idx
			}
		}
	}
	return -1
}

// getCell safely gets a cell value from a row
func getCell(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// mapCustomerType converts customer type to grade
func mapCustomerType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "end customer":
		return "A" // Direct customers are highest value
	case "system integrator":
		return "B" // Project-based, good margin
	case "national reseller":
		return "C" // Volume, lower margin
	case "service provider", "service company":
		return "B" // Recurring service revenue
	default:
		return "C"
	}
}

// generateCustomerID creates a unique customer ID from business name
func generateCustomerID(name string) string {
	// Take first 3 letters + timestamp suffix
	prefix := strings.ToUpper(strings.ReplaceAll(name, " ", ""))
	if len(prefix) > 3 {
		prefix = prefix[:3]
	}
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()%10000)
}

// generateCustomerCode creates a short customer code
func generateCustomerCode(name string) string {
	// Take first 6 letters
	code := strings.ToUpper(strings.ReplaceAll(name, " ", ""))
	if len(code) > 6 {
		code = code[:6]
	}
	return code
}

// extractCity tries to extract city from address
func extractCity(address string) string {
	// Common Bahrain cities
	cities := []string{"Manama", "Riffa", "Muharraq", "Hamad Town", "Isa Town", "Sitra", "Budaiya", "Jidhafs", "A'ali", "Sanabis"}

	for _, city := range cities {
		if strings.Contains(strings.ToLower(address), strings.ToLower(city)) {
			return city
		}
	}

	// Default to Manama if address contains "Bahrain"
	if strings.Contains(strings.ToLower(address), "bahrain") {
		return "Manama"
	}

	return ""
}
