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

type Supplier struct {
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
	TaxNumber    string    `gorm:"size:50" json:"tax_number"`
	Notes        string    `json:"notes"`
	PaymentTerms int       `json:"payment_terms"`
}

func (Supplier) TableName() string { return "suppliers" }

type Invoice struct {
	ID                  string    `gorm:"primaryKey;size:36"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	CustomerID          string    `gorm:"size:36" json:"customer_id"`
	CustomerName        string    `gorm:"size:255" json:"customer_name"`
	InvoiceNumber       string    `gorm:"size:50" json:"invoice_number"`
	InvoiceDate         time.Time `json:"invoice_date"`
	DueDate             time.Time `json:"due_date"`
	Status              string    `gorm:"size:50" json:"status"`
	SubtotalBHD         float64   `json:"subtotal_bhd"`
	VATBHD              float64   `json:"vatbhd"`
	GrandTotalBHD       float64   `json:"grand_total_bhd"`
	PaidBHD             float64   `json:"paid_bhd"`
	Currency            string    `gorm:"size:10" json:"currency"`
	Notes               string    `json:"notes"`
	OrderID             string    `gorm:"size:36" json:"order_id"`
	TallyVoucherNumber  string    `gorm:"size:50" json:"tally_voucher_number"`
}

func (Invoice) TableName() string { return "invoices" }

type Order struct {
	ID             string    `gorm:"primaryKey;size:36"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	CustomerID     string    `gorm:"size:36" json:"customer_id"`
	CustomerName   string    `gorm:"size:255" json:"customer_name"`
	OrderNumber    string    `gorm:"size:50" json:"order_number"`
	OrderDate      time.Time `json:"order_date"`
	Status         string    `gorm:"size:50" json:"status"`
	TotalBHD       float64   `json:"total_bhd"`
	Notes          string    `json:"notes"`
	DeliveryDate   time.Time `json:"delivery_date"`
	CustomerPONum  string    `gorm:"size:100" json:"customer_po_num"`
}

func (Order) TableName() string { return "orders" }

type Payment struct {
	ID            string    `gorm:"primaryKey;size:36"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CustomerID    string    `gorm:"size:36" json:"customer_id"`
	CustomerName  string    `gorm:"size:255" json:"customer_name"`
	InvoiceID     string    `gorm:"size:36" json:"invoice_id"`
	PaymentDate   time.Time `json:"payment_date"`
	AmountBHD     float64   `json:"amount_bhd"`
	PaymentMethod string    `gorm:"size:50" json:"payment_method"`
	ReferenceNo   string    `gorm:"size:100" json:"reference_no"`
	Notes         string    `json:"notes"`
}

func (Payment) TableName() string { return "payments" }

type SupplierInvoice struct {
	ID            string    `gorm:"primaryKey;size:36"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	SupplierID    string    `gorm:"size:36" json:"supplier_id"`
	SupplierName  string    `gorm:"size:255" json:"supplier_name"`
	InvoiceNumber string    `gorm:"size:50" json:"invoice_number"`
	InvoiceDate   time.Time `json:"invoice_date"`
	DueDate       time.Time `json:"due_date"`
	Status        string    `gorm:"size:50" json:"status"`
	SubtotalBHD   float64   `json:"subtotal_bhd"`
	VATBHD        float64   `json:"vatbhd"`
	GrandTotalBHD float64   `json:"grand_total_bhd"`
	PaidBHD       float64   `json:"paid_bhd"`
	Currency      string    `gorm:"size:10" json:"currency"`
	Notes         string    `json:"notes"`
	POID          string    `gorm:"size:36" json:"po_id"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

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

type PurchaseOrder struct {
	ID           string    `gorm:"primaryKey;size:36"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	SupplierID   string    `gorm:"size:36" json:"supplier_id"`
	SupplierName string    `gorm:"size:255" json:"supplier_name"`
	PONumber     string    `gorm:"size:50" json:"po_number"`
	PODate       time.Time `json:"po_date"`
	Status       string    `gorm:"size:50" json:"status"`
	TotalBHD     float64   `json:"total_bhd"`
	Notes        string    `json:"notes"`
	OrderID      string    `gorm:"size:36" json:"order_id"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

type RFQData struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Client         string    `json:"client"`
	Project        string    `json:"project"`
	Value          float64   `json:"value"`
	Notes          string    `json:"notes"`
	Status         string    `gorm:"default:'pending'" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DocumentHash   string    `json:"document_hash"`
	VisitLocations string    `json:"visit_locations"`
	ProductDetails string    `json:"product_details"`
	SourceDocPath  string    `json:"source_doc_path"`
}

func (RFQData) TableName() string { return "rfq_data" }

type Offer struct {
	ID              string    `gorm:"primaryKey;size:36"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	OfferNumber     string    `gorm:"size:50" json:"offer_number"`
	RFQID           string    `gorm:"size:36" json:"rfq_id"`
	CustomerID      string    `gorm:"size:36" json:"customer_id"`
	CustomerName    string    `gorm:"size:255" json:"customer_name"`
	QuotationDate   time.Time `json:"quotation_date"`
	TotalValueBHD   float64   `json:"total_value_bhd"`
	EstimatedMargin float64   `json:"estimated_margin"`
	Stage           string    `gorm:"size:50" json:"stage"`
	CreatedBy       string    `json:"created_by"`
}

func (Offer) TableName() string { return "offers" }

type OfferItem struct {
	ID            string    `gorm:"primaryKey;size:36"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	OfferID       string    `gorm:"size:36" json:"offer_id"`
	LineNumber    int       `json:"line_number"`
	ProductCode   string    `gorm:"size:100" json:"product_code"`
	Description   string    `json:"description"`
	Quantity      float64   `json:"quantity"`
	UnitPrice     float64   `json:"unit_price_bhd"`
	Equipment     string    `gorm:"size:255" json:"equipment"`
	Model         string    `gorm:"size:255" json:"model"`
	Specification string    `json:"specification"`
	TotalCost     float64   `json:"total_cost"`
	MarginPercent float64   `json:"margin_percent"`
	TotalPrice    float64   `json:"total_price"`
}

func (OfferItem) TableName() string { return "offer_items" }

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
	fmt.Println("   Complete Database -> Supabase Sync")
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

	// Drop and recreate tables for clean sync
	fmt.Println("\n📋 Dropping existing tables...")
	tables := []string{
		"customer_contacts", "offer_items", "offers", "rfq_data",
		"purchase_orders", "supplier_payments", "supplier_invoices",
		"payments", "orders", "invoices", "suppliers", "customers",
	}
	for _, t := range tables {
		supabaseDB.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", t))
	}
	fmt.Println("✅ Tables dropped")

	// AutoMigrate all tables on Supabase
	fmt.Println("\n📋 Creating tables...")
	if err := supabaseDB.AutoMigrate(
		&Customer{},
		&Supplier{},
		&Invoice{},
		&Order{},
		&Payment{},
		&SupplierInvoice{},
		&SupplierPayment{},
		&PurchaseOrder{},
		&RFQData{},
		&Offer{},
		&OfferItem{},
		&CustomerContact{},
	); err != nil {
		log.Fatalf("AutoMigrate failed: %v", err)
	}
	fmt.Println("✅ Tables created")

	// Sync each table
	totalRecords := 0

	totalRecords += syncTable(sqliteDB, supabaseDB, "customers", &[]Customer{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "suppliers", &[]Supplier{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "invoices", &[]Invoice{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "orders", &[]Order{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "payments", &[]Payment{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "supplier_invoices", &[]SupplierInvoice{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "supplier_payments", &[]SupplierPayment{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "purchase_orders", &[]PurchaseOrder{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "rfq_data", &[]RFQData{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "offers", &[]Offer{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "offer_items", &[]OfferItem{})
	totalRecords += syncTable(sqliteDB, supabaseDB, "customer_contacts", &[]CustomerContact{})

	fmt.Println("\n========================================")
	fmt.Printf("  ✅ Sync Complete! Total: %d records\n", totalRecords)
	fmt.Println("========================================")
}

func syncTable[T any](sqliteDB, supabaseDB *gorm.DB, tableName string, dest *[]T) int {
	// Get data from SQLite
	if err := sqliteDB.Find(dest).Error; err != nil {
		fmt.Printf("❌ %s: read error: %v\n", tableName, err)
		return 0
	}

	count := len(*dest)
	if count == 0 {
		fmt.Printf("⏭️ %s: empty\n", tableName)
		return 0
	}

	// Tables already fresh from DROP/CREATE - just insert

	// Insert in batches
	batchSize := 100
	for i := 0; i < count; i += batchSize {
		end := i + batchSize
		if end > count {
			end = count
		}
		batch := (*dest)[i:end]
		if err := supabaseDB.Create(&batch).Error; err != nil {
			fmt.Printf("⚠️ %s: batch %d error: %v\n", tableName, i/batchSize+1, err)
		}
	}

	fmt.Printf("✅ %s: %d records synced\n", tableName, count)
	return count
}
