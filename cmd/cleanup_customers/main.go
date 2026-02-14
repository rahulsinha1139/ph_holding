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
	"gorm.io/gorm/logger"
)

// CustomerMaster - matches the main app's struct
type CustomerMaster struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	CustomerID       string  `gorm:"uniqueIndex;size:50" json:"customer_id"`
	CustomerCode     string  `gorm:"uniqueIndex;size:50" json:"customer_code"`
	CustomerType     string  `gorm:"index;size:50" json:"customer_type"`
	BusinessName     string  `gorm:"index;size:255" json:"business_name"`
	ShortCode        string  `gorm:"size:10" json:"short_code"`
	AddressLine1     string  `json:"address_line1"`
	City             string  `gorm:"index;size:100" json:"city"`
	Country          string  `gorm:"index;size:100" json:"country"`
	TRN              string  `gorm:"size:100" json:"trn"`
	Industry         string  `gorm:"index;size:100" json:"industry"`
	RelationYears    int     `json:"relation_years"`
	PaymentGrade     string  `gorm:"index;size:10;default:'C'" json:"payment_grade"`
	CustomerGrade    string  `gorm:"index;size:10;default:'C'" json:"customer_grade"`
	PaymentTermsDays int     `gorm:"index" json:"payment_terms_days"`
	AvgPaymentDays   float64 `json:"avg_payment_days"`
	DisputeCount     int     `json:"dispute_count"`
	CreditLimitBHD   float64 `gorm:"default:50000" json:"credit_limit_bhd"`
}

func (CustomerMaster) TableName() string { return "customers" }

// CustomerContact - matches the main app's struct
type CustomerContact struct {
	ID        uint       `gorm:"primarykey" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`

	CustomerID       string `gorm:"index;size:36" json:"customer_id"`
	ContactName      string `gorm:"size:255" json:"contact_name"`
	JobTitle         string `gorm:"size:100" json:"job_title"`
	Email            string `gorm:"size:255" json:"email"`
	Phone            string `gorm:"size:50" json:"phone"`
	Address          string `gorm:"type:varchar(1000)" json:"address"`
	IsPrimaryContact bool   `json:"is_primary_contact"`
}

func (CustomerContact) TableName() string { return "customer_contacts" }

func main() {
	log.Println("🧹 Customer Database Cleanup & Import Tool")
	log.Println("==========================================")

	// Connect to database
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Get current stats
	var currentCount int64
	db.Model(&CustomerMaster{}).Count(&currentCount)
	log.Printf("📊 Current customers in database: %d", currentCount)

	// Step 1: Analyze duplicates
	log.Println("\n--- STEP 1: Analyzing duplicates ---")
	type DuplicateGroup struct {
		BusinessName string
		Count        int64
	}
	var duplicates []DuplicateGroup
	db.Model(&CustomerMaster{}).
		Select("business_name, count(*) as count").
		Group("LOWER(TRIM(business_name))").
		Having("count(*) > 1").
		Order("count desc").
		Scan(&duplicates)

	if len(duplicates) > 0 {
		log.Printf("⚠️  Found %d business names with duplicates:", len(duplicates))
		for i, d := range duplicates {
			if i < 20 {
				log.Printf("   - %q: %d entries", d.BusinessName, d.Count)
			}
		}
		if len(duplicates) > 20 {
			log.Printf("   ... and %d more", len(duplicates)-20)
		}
	} else {
		log.Println("✅ No duplicates found in current database")
	}

	// Step 2: Read Excel file
	log.Println("\n--- STEP 2: Reading Excel file ---")
	excelPath := "Error_Logs/customer database .xlsx"
	if _, err := os.Stat(excelPath); os.IsNotExist(err) {
		log.Fatal("Excel file not found:", excelPath)
	}

	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		log.Fatal("Failed to open Excel:", err)
	}
	defer f.Close()

	rows, err := f.GetRows("Tabelle1")
	if err != nil {
		log.Fatal("Failed to read sheet:", err)
	}
	log.Printf("📊 Excel has %d rows (including header)", len(rows))

	// Parse Excel data - group by business name to deduplicate
	type ExcelCustomer struct {
		BusinessName string
		CustomerType string
		Address      string
		Contacts     []struct {
			Name   string
			Phone  string
			Email  string
			Prefix string
		}
	}

	customerMap := make(map[string]*ExcelCustomer)

	for i, row := range rows {
		if i == 0 { // Skip header
			continue
		}
		if len(row) < 7 {
			continue
		}

		contactName := strings.TrimSpace(row[0])
		salutation := strings.TrimSpace(row[1])
		phone := strings.TrimSpace(row[2])
		email := strings.TrimSpace(row[3])
		address := strings.TrimSpace(row[4])
		businessName := strings.TrimSpace(row[5])
		customerType := strings.TrimSpace(row[6])

		if businessName == "" {
			continue
		}

		// Normalize business name for deduplication
		key := strings.ToLower(businessName)

		if _, exists := customerMap[key]; !exists {
			customerMap[key] = &ExcelCustomer{
				BusinessName: businessName,
				CustomerType: customerType,
				Address:      address,
			}
		}

		// Add contact
		customerMap[key].Contacts = append(customerMap[key].Contacts, struct {
			Name   string
			Phone  string
			Email  string
			Prefix string
		}{
			Name:   contactName,
			Phone:  phone,
			Email:  email,
			Prefix: salutation,
		})
	}

	log.Printf("📊 Found %d unique business accounts in Excel", len(customerMap))

	// Step 3: Clear existing customers and contacts (fresh import)
	log.Println("\n--- STEP 3: Clearing existing data ---")

	// First, let's backup customer IDs that are referenced in other tables
	var existingRefs []string
	db.Raw(`
		SELECT DISTINCT c.customer_id FROM customers c
		WHERE EXISTS (SELECT 1 FROM invoices i WHERE i.customer_id = c.id)
		   OR EXISTS (SELECT 1 FROM orders o WHERE o.customer_id = c.customer_id)
		   OR EXISTS (SELECT 1 FROM rfq_data r WHERE r.client = c.business_name)
	`).Scan(&existingRefs)
	log.Printf("📝 Found %d customers referenced in other tables (will preserve)", len(existingRefs))

	// Delete all customer contacts first (foreign key)
	db.Exec("DELETE FROM customer_contacts")
	log.Println("✅ Cleared customer_contacts table")

	// Delete customers not referenced elsewhere
	result := db.Exec("DELETE FROM customers WHERE customer_id NOT IN (SELECT DISTINCT customer_id FROM invoices WHERE customer_id IS NOT NULL AND customer_id != '') AND id NOT IN (SELECT DISTINCT customer_id FROM orders WHERE customer_id IS NOT NULL AND customer_id != 0)")
	log.Printf("✅ Removed %d unreferenced customers", result.RowsAffected)

	// Step 4: Import from Excel
	log.Println("\n--- STEP 4: Importing from Excel ---")

	imported := 0
	skipped := 0
	contactsAdded := 0

	for _, cust := range customerMap {
		// Check if customer already exists (case-insensitive match)
		var existing CustomerMaster
		err := db.Where("LOWER(business_name) = LOWER(?)", cust.BusinessName).First(&existing).Error

		var customerID string

		if err == gorm.ErrRecordNotFound {
			// Create new customer
			customerID = uuid.New().String()
			newCust := CustomerMaster{
				CustomerID:     customerID,
				CustomerCode:   fmt.Sprintf("CUST-%d", time.Now().UnixNano()%100000),
				BusinessName:   cust.BusinessName,
				CustomerType:   cust.CustomerType,
				AddressLine1:   cust.Address,
				Country:        "Bahrain",
				PaymentGrade:   "C",
				CustomerGrade:  "C",
				CreditLimitBHD: 50000,
			}

			if err := db.Create(&newCust).Error; err != nil {
				log.Printf("⚠️  Failed to create %q: %v", cust.BusinessName, err)
				continue
			}
			imported++
		} else if err == nil {
			// Customer exists, update if needed
			customerID = existing.CustomerID
			if existing.AddressLine1 == "" && cust.Address != "" {
				db.Model(&existing).Update("address_line1", cust.Address)
			}
			if existing.CustomerType == "" && cust.CustomerType != "" {
				db.Model(&existing).Update("customer_type", cust.CustomerType)
			}
			skipped++
		} else {
			log.Printf("⚠️  Error checking %q: %v", cust.BusinessName, err)
			continue
		}

		// Add contacts (avoid duplicates by email)
		for i, contact := range cust.Contacts {
			if contact.Name == "" && contact.Email == "" {
				continue
			}

			// Check if contact exists
			var existingContact CustomerContact
			err := db.Where("customer_id = ? AND email = ?", customerID, contact.Email).First(&existingContact).Error
			if err == gorm.ErrRecordNotFound {
				newContact := CustomerContact{
					CustomerID:       customerID,
					ContactName:      contact.Name,
					Phone:            contact.Phone,
					Email:            contact.Email,
					JobTitle:         contact.Prefix,
					IsPrimaryContact: i == 0, // First contact is primary
				}
				if err := db.Create(&newContact).Error; err == nil {
					contactsAdded++
				}
			}
		}
	}

	log.Printf("✅ Imported %d new customers, %d already existed", imported, skipped)
	log.Printf("✅ Added %d contacts", contactsAdded)

	// Final stats
	var finalCount int64
	db.Model(&CustomerMaster{}).Count(&finalCount)
	var contactCount int64
	db.Model(&CustomerContact{}).Count(&contactCount)

	log.Println("\n==========================================")
	log.Printf("📊 FINAL STATS:")
	log.Printf("   Customers: %d (was %d)", finalCount, currentCount)
	log.Printf("   Contacts: %d", contactCount)
	log.Println("✅ Customer database cleanup complete!")
}
