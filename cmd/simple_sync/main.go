package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Core models for sync
type Customer struct {
	ID           string     `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time `gorm:"index"`
	BusinessName string
	ContactName  string
	Email        string
	Phone        string
	Address      string
	Country      string
	City         string
	Industry     string
	TaxNumber    string
	Notes        string
	CreditLimit  float64
	PaymentTerms int
}

type CustomerContact struct {
	ID         string     `gorm:"primaryKey"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time `gorm:"index"`
	CustomerID string
	Name       string
	Email      string
	Phone      string
	JobTitle   string
	IsPrimary  bool
}

type Supplier struct {
	ID           string     `gorm:"primaryKey"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time `gorm:"index"`
	SupplierCode string
	SupplierName string
	Country      string
	LeadTimeDays int
	TaxID        string
	SupplierType string
	BrandsHandled string
	ProductTypes  string
	PrimaryContact string
	Email          string
	Phone          string
	Address        string
	BankName       string
	AccountNumber  string
	IBAN           string
	SwiftCode      string
	PaymentTerms   string
	Rating         int
	Notes          string
}

type SupplierInvoice struct {
	ID            string     `gorm:"primaryKey"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time `gorm:"index"`
	SupplierID    string
	InvoiceNumber string
	InvoiceDate   time.Time
	DueDate       time.Time
	TotalBHD      float64
	VatBHD        float64
	Status        string
	Currency      string
	ExchangeRate  float64
	OriginalAmount float64
	Notes         string
	SupplierName  string
}

type SupplierPayment struct {
	ID                string     `gorm:"primaryKey"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time `gorm:"index"`
	SupplierID        string
	SupplierInvoiceID string
	PaymentDate       time.Time
	Amount            float64
	PaymentMethod     string
	Reference         string
	Notes             string
}

type PurchaseOrder struct {
	ID             string     `gorm:"primaryKey"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time `gorm:"index"`
	PONumber       string
	SupplierID     string
	SupplierName   string
	OrderID        string
	RfqID          string
	PODate         time.Time
	ExpectedDate   time.Time
	TotalBHD       float64
	VatBHD         float64
	Status         string
	Currency       string
	ExchangeRate   float64
	OriginalAmount float64
	Notes          string
}

func main() {
	fmt.Println("========================================")
	fmt.Println("   SIMPLE SYNC: SQLite → Supabase")
	fmt.Println("========================================")

	godotenv.Load()

	// Connect to SQLite
	local, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("SQLite error:", err)
	}
	fmt.Println("✅ SQLite connected")

	// Connect to Supabase
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=require",
		os.Getenv("SUPABASE_DB_HOST"),
		os.Getenv("SUPABASE_DB_PORT"),
		os.Getenv("SUPABASE_DB_USER"),
		os.Getenv("SUPABASE_DB_PASSWORD"))

	cloud, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Supabase error:", err)
	}
	fmt.Println("✅ Supabase connected")

	// Step 1: Clear critical tables
	fmt.Println("\n🗑️  Clearing Supabase tables...")
	tables := []string{
		"customer_contacts", "supplier_contacts", "supplier_payments",
		"supplier_invoices", "purchase_orders", "customers", "suppliers",
	}
	for _, t := range tables {
		cloud.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", t))
		fmt.Printf("  ✅ %s cleared\n", t)
	}

	// Step 2: Sync each model
	fmt.Println("\n📤 Syncing data...")

	// Customers
	var customers []Customer
	local.Where("deleted_at IS NULL").Find(&customers)
	if len(customers) > 0 {
		for _, c := range customers {
			cloud.Create(&c)
		}
		fmt.Printf("  ✅ customers: %d synced\n", len(customers))
	}

	// Customer Contacts
	var contacts []CustomerContact
	local.Where("deleted_at IS NULL").Find(&contacts)
	if len(contacts) > 0 {
		for _, c := range contacts {
			cloud.Create(&c)
		}
		fmt.Printf("  ✅ customer_contacts: %d synced\n", len(contacts))
	}

	// Suppliers
	var suppliers []Supplier
	local.Where("deleted_at IS NULL").Find(&suppliers)
	if len(suppliers) > 0 {
		for _, s := range suppliers {
			cloud.Create(&s)
		}
		fmt.Printf("  ✅ suppliers: %d synced\n", len(suppliers))
	}

	// Supplier Invoices
	var sInvoices []SupplierInvoice
	local.Where("deleted_at IS NULL").Find(&sInvoices)
	if len(sInvoices) > 0 {
		for _, si := range sInvoices {
			cloud.Create(&si)
		}
		fmt.Printf("  ✅ supplier_invoices: %d synced\n", len(sInvoices))
	}

	// Supplier Payments
	var sPayments []SupplierPayment
	local.Where("deleted_at IS NULL").Find(&sPayments)
	if len(sPayments) > 0 {
		for _, sp := range sPayments {
			cloud.Create(&sp)
		}
		fmt.Printf("  ✅ supplier_payments: %d synced\n", len(sPayments))
	}

	// Purchase Orders
	var pos []PurchaseOrder
	local.Where("deleted_at IS NULL").Find(&pos)
	if len(pos) > 0 {
		for _, po := range pos {
			cloud.Create(&po)
		}
		fmt.Printf("  ✅ purchase_orders: %d synced\n", len(pos))
	}

	// Step 3: Verify
	fmt.Println("\n🔍 Verifying...")
	var custCount, contCount, suppCount, siCount, spCount, poCount int64
	cloud.Model(&Customer{}).Count(&custCount)
	cloud.Model(&CustomerContact{}).Count(&contCount)
	cloud.Model(&Supplier{}).Count(&suppCount)
	cloud.Model(&SupplierInvoice{}).Count(&siCount)
	cloud.Model(&SupplierPayment{}).Count(&spCount)
	cloud.Model(&PurchaseOrder{}).Count(&poCount)

	fmt.Printf("  Customers:         %d\n", custCount)
	fmt.Printf("  CustomerContacts:  %d\n", contCount)
	fmt.Printf("  Suppliers:         %d\n", suppCount)
	fmt.Printf("  SupplierInvoices:  %d\n", siCount)
	fmt.Printf("  SupplierPayments:  %d\n", spCount)
	fmt.Printf("  PurchaseOrders:    %d\n", poCount)

	fmt.Println("\n========================================")
	fmt.Println("   ✅ SYNC COMPLETE!")
	fmt.Println("========================================")
}
