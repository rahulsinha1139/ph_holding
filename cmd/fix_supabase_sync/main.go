package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// All models that need syncing
type Customer struct {
	ID           string    `gorm:"primaryKey;size:36"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	BusinessName string    `gorm:"size:255" json:"business_name"`
	ContactName  string    `gorm:"size:255" json:"contact_name"`
	Email        string    `gorm:"size:255" json:"email"`
	Phone        string    `gorm:"size:50" json:"phone"`
	Address      string    `json:"address"`
	Country      string    `gorm:"size:100" json:"country"`
	City         string    `gorm:"size:100" json:"city"`
	Industry     string    `gorm:"size:100" json:"industry"`
	TaxNumber    string    `gorm:"size:50" json:"tax_number"`
	Notes        string    `json:"notes"`
	CreditLimit  float64   `json:"credit_limit"`
	PaymentTerms int       `json:"payment_terms"`
}

func (Customer) TableName() string { return "customers" }

type CustomerContact struct {
	ID               string    `gorm:"primaryKey;size:36"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CustomerID       string    `gorm:"size:36" json:"customer_id"`
	ContactName      string    `gorm:"size:255" json:"contact_name"`
	Salutation       string    `gorm:"size:20" json:"salutation"`
	JobTitle         string    `gorm:"size:100" json:"job_title"`
	Email            string    `gorm:"size:255" json:"email"`
	Phone            string    `gorm:"size:50" json:"phone"`
	Address          string    `json:"address"`
	IsPrimaryContact bool      `json:"is_primary_contact"`
}

func (CustomerContact) TableName() string { return "customer_contacts" }

func main() {
	fmt.Println("========================================")
	fmt.Println("   FIXED Customers Sync to Supabase")
	fmt.Println("========================================")

	// Connect to SQLite
	sqliteDB, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("SQLite connection failed: %v", err)
	}
	fmt.Println("✅ SQLite connected")

	// Connect to Supabase
	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dsn := fmt.Sprintf("host=aws-1-ap-southeast-1.pooler.supabase.com port=5432 user=postgres.xkyydcxxkzjbnsrmzkpp password=%s dbname=postgres sslmode=require", supabasePassword)
	supabaseDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Supabase connection failed: %v", err)
	}
	fmt.Println("✅ Supabase connected")

	// DELETE instead of TRUNCATE (safer, works with RLS)
	fmt.Println("\n📋 Clearing existing data...")
	supabaseDB.Exec("DELETE FROM customer_contacts")
	supabaseDB.Exec("DELETE FROM customers")
	fmt.Println("✅ Tables cleared")

	// Sync customers ONE BY ONE to see exactly what happens
	fmt.Println("\n📋 Syncing customers...")
	var customers []Customer
	if err := sqliteDB.Find(&customers).Error; err != nil {
		log.Fatalf("Failed to read customers: %v", err)
	}

	succeeded := 0
	failed := 0
	for i, customer := range customers {
		if err := supabaseDB.Create(&customer).Error; err != nil {
			if i < 5 { // Only show first 5 errors
				fmt.Printf("  ❌ Customer %d (%s): %v\n", i+1, customer.BusinessName, err)
			}
			failed++
		} else {
			succeeded++
		}
		if (i+1)%50 == 0 {
			fmt.Printf("  ... processed %d/%d customers\n", i+1, len(customers))
		}
	}
	fmt.Printf("✅ Customers: %d succeeded, %d failed (total: %d)\n", succeeded, failed, len(customers))

	// Sync customer_contacts ONE BY ONE
	fmt.Println("\n📋 Syncing customer_contacts...")
	var contacts []CustomerContact
	if err := sqliteDB.Find(&contacts).Error; err != nil {
		log.Fatalf("Failed to read customer_contacts: %v", err)
	}

	succeeded = 0
	failed = 0
	for i, contact := range contacts {
		if err := supabaseDB.Create(&contact).Error; err != nil {
			if i < 5 { // Only show first 5 errors
				fmt.Printf("  ❌ Contact %d (%s): %v\n", i+1, contact.ContactName, err)
			}
			failed++
		} else {
			succeeded++
		}
		if (i+1)%100 == 0 {
			fmt.Printf("  ... processed %d/%d contacts\n", i+1, len(contacts))
		}
	}
	fmt.Printf("✅ Customer Contacts: %d succeeded, %d failed (total: %d)\n", succeeded, failed, len(contacts))

	// Verify
	fmt.Println("\n📋 Verifying...")
	var custCount, contactCount int64
	supabaseDB.Table("customers").Count(&custCount)
	supabaseDB.Table("customer_contacts").Count(&contactCount)
	fmt.Printf("  Customers in Supabase: %d\n", custCount)
	fmt.Printf("  Contacts in Supabase:  %d\n", contactCount)

	fmt.Println("\n========================================")
	fmt.Println("  ✅ Sync Complete!")
	fmt.Println("========================================")
}
