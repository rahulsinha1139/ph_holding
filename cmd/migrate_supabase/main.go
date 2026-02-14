package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Base model for all entities
type Base struct {
	ID        string     `gorm:"primaryKey;size:36" json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

// ============ MASTER DATA ============

type CustomerMaster struct {
	Base
	CompanyName     string  `gorm:"size:255;not null" json:"company_name"`
	ContactPerson   string  `gorm:"size:255" json:"contact_person"`
	Email           string  `gorm:"size:255" json:"email"`
	Phone           string  `gorm:"size:50" json:"phone"`
	Address         string  `gorm:"type:text" json:"address"`
	City            string  `gorm:"size:100" json:"city"`
	Country         string  `gorm:"size:100" json:"country"`
	Grade           string  `gorm:"size:10;default:'B'" json:"grade"`
	CreditLimit     float64 `gorm:"default:0" json:"credit_limit"`
	PaymentTerms    string  `gorm:"size:50" json:"payment_terms"`
	TaxID           string  `gorm:"size:50" json:"tax_id"`
	Notes           string  `gorm:"type:text" json:"notes"`
	IsActive        bool    `gorm:"default:true" json:"is_active"`
	TotalRevenue    float64 `gorm:"default:0" json:"total_revenue"`
	TotalOrders     int     `gorm:"default:0" json:"total_orders"`
	LastOrderDate   *time.Time `json:"last_order_date"`
	AvgPaymentDays  float64 `gorm:"default:0" json:"avg_payment_days"`
}

type CustomerContact struct {
	Base
	CustomerID  string `gorm:"size:36;index" json:"customer_id"`
	Name        string `gorm:"size:255" json:"name"`
	Title       string `gorm:"size:100" json:"title"`
	Email       string `gorm:"size:255" json:"email"`
	Phone       string `gorm:"size:50" json:"phone"`
	Mobile      string `gorm:"size:50" json:"mobile"`
	IsPrimary   bool   `gorm:"default:false" json:"is_primary"`
	Department  string `gorm:"size:100" json:"department"`
}

type SupplierMaster struct {
	Base
	CompanyName     string  `gorm:"size:255;not null" json:"company_name"`
	ContactPerson   string  `gorm:"size:255" json:"contact_person"`
	Email           string  `gorm:"size:255" json:"email"`
	Phone           string  `gorm:"size:50" json:"phone"`
	Address         string  `gorm:"type:text" json:"address"`
	City            string  `gorm:"size:100" json:"city"`
	Country         string  `gorm:"size:100" json:"country"`
	PaymentTerms    string  `gorm:"size:50" json:"payment_terms"`
	LeadTimeDays    int     `gorm:"default:14" json:"lead_time_days"`
	TaxID           string  `gorm:"size:50" json:"tax_id"`
	BankDetails     string  `gorm:"type:text" json:"bank_details"`
	Notes           string  `gorm:"type:text" json:"notes"`
	IsActive        bool    `gorm:"default:true" json:"is_active"`
	Rating          int     `gorm:"default:3" json:"rating"`
	TotalPurchases  float64 `gorm:"default:0" json:"total_purchases"`
}

type SupplierContact struct {
	Base
	SupplierID  string `gorm:"size:36;index" json:"supplier_id"`
	Name        string `gorm:"size:255" json:"name"`
	Title       string `gorm:"size:100" json:"title"`
	Email       string `gorm:"size:255" json:"email"`
	Phone       string `gorm:"size:50" json:"phone"`
	Mobile      string `gorm:"size:50" json:"mobile"`
	IsPrimary   bool   `gorm:"default:false" json:"is_primary"`
	Department  string `gorm:"size:100" json:"department"`
}

type ProductMaster struct {
	Base
	SKU           string  `gorm:"size:50;uniqueIndex" json:"sku"`
	Name          string  `gorm:"size:255;not null" json:"name"`
	Description   string  `gorm:"type:text" json:"description"`
	Category      string  `gorm:"size:100" json:"category"`
	Brand         string  `gorm:"size:100" json:"brand"`
	UnitPrice     float64 `gorm:"default:0" json:"unit_price"`
	CostPrice     float64 `gorm:"default:0" json:"cost_price"`
	UnitOfMeasure string  `gorm:"size:20" json:"unit_of_measure"`
	MinStock      int     `gorm:"default:0" json:"min_stock"`
	MaxStock      int     `gorm:"default:0" json:"max_stock"`
	CurrentStock  int     `gorm:"default:0" json:"current_stock"`
	IsActive      bool    `gorm:"default:true" json:"is_active"`
	SupplierID    string  `gorm:"size:36" json:"supplier_id"`
}

// ============ SALES ============

type Offer struct {
	Base
	OfferNumber   string    `gorm:"size:50;uniqueIndex" json:"offer_number"`
	CustomerID    string    `gorm:"size:36;index" json:"customer_id"`
	RFQID         string    `gorm:"size:36" json:"rfq_id"`
	ValidUntil    time.Time `json:"valid_until"`
	Status        string    `gorm:"size:20;default:'Draft'" json:"status"`
	Subtotal      float64   `gorm:"default:0" json:"subtotal"`
	DiscountPct   float64   `gorm:"default:0" json:"discount_pct"`
	DiscountAmt   float64   `gorm:"default:0" json:"discount_amt"`
	TaxPct        float64   `gorm:"default:10" json:"tax_pct"`
	TaxAmt        float64   `gorm:"default:0" json:"tax_amt"`
	Total         float64   `gorm:"default:0" json:"total"`
	Notes         string    `gorm:"type:text" json:"notes"`
	WinProbability float64  `gorm:"default:50" json:"win_probability"`
}

type OfferItem struct {
	Base
	OfferID     string  `gorm:"size:36;index" json:"offer_id"`
	ProductID   string  `gorm:"size:36" json:"product_id"`
	Description string  `gorm:"type:text" json:"description"`
	Quantity    float64 `gorm:"default:1" json:"quantity"`
	UnitPrice   float64 `gorm:"default:0" json:"unit_price"`
	DiscountPct float64 `gorm:"default:0" json:"discount_pct"`
	Total       float64 `gorm:"default:0" json:"total"`
}

type Order struct {
	Base
	OrderNumber   string     `gorm:"size:50;uniqueIndex" json:"order_number"`
	CustomerID    string     `gorm:"size:36;index" json:"customer_id"`
	OfferID       string     `gorm:"size:36" json:"offer_id"`
	RFQID         string     `gorm:"size:36" json:"rfq_id"`
	OrderDate     time.Time  `json:"order_date"`
	DeliveryDate  *time.Time `json:"delivery_date"`
	Status        string     `gorm:"size:20;default:'Confirmed'" json:"status"`
	Subtotal      float64    `gorm:"default:0" json:"subtotal"`
	DiscountPct   float64    `gorm:"default:0" json:"discount_pct"`
	DiscountAmt   float64    `gorm:"default:0" json:"discount_amt"`
	TaxPct        float64    `gorm:"default:10" json:"tax_pct"`
	TaxAmt        float64    `gorm:"default:0" json:"tax_amt"`
	Total         float64    `gorm:"default:0" json:"total"`
	Notes         string     `gorm:"type:text" json:"notes"`
	ShippingAddress string   `gorm:"type:text" json:"shipping_address"`
}

type OrderItem struct {
	Base
	OrderID     string  `gorm:"size:36;index" json:"order_id"`
	ProductID   string  `gorm:"size:36" json:"product_id"`
	Description string  `gorm:"type:text" json:"description"`
	Quantity    float64 `gorm:"default:1" json:"quantity"`
	UnitPrice   float64 `gorm:"default:0" json:"unit_price"`
	DiscountPct float64 `gorm:"default:0" json:"discount_pct"`
	Total       float64 `gorm:"default:0" json:"total"`
}

// ============ FINANCE ============

type Invoice struct {
	Base
	InvoiceNumber string     `gorm:"size:50;uniqueIndex" json:"invoice_number"`
	OrderID       string     `gorm:"size:36;uniqueIndex" json:"order_id"`
	CustomerID    string     `gorm:"size:36;index" json:"customer_id"`
	InvoiceDate   time.Time  `json:"invoice_date"`
	DueDate       time.Time  `json:"due_date"`
	Status        string     `gorm:"size:20;default:'Draft'" json:"status"`
	Subtotal      float64    `gorm:"default:0" json:"subtotal"`
	TaxPct        float64    `gorm:"default:10" json:"tax_pct"`
	TaxAmt        float64    `gorm:"default:0" json:"tax_amt"`
	Total         float64    `gorm:"default:0" json:"total"`
	AmountPaid    float64    `gorm:"default:0" json:"amount_paid"`
	Notes         string     `gorm:"type:text" json:"notes"`
}

type DBInvoiceItem struct {
	Base
	InvoiceID   string  `gorm:"size:36;index" json:"invoice_id"`
	ProductID   string  `gorm:"size:36" json:"product_id"`
	Description string  `gorm:"type:text" json:"description"`
	Quantity    float64 `gorm:"default:1" json:"quantity"`
	UnitPrice   float64 `gorm:"default:0" json:"unit_price"`
	Total       float64 `gorm:"default:0" json:"total"`
}

type Payment struct {
	Base
	PaymentNumber string    `gorm:"size:50;uniqueIndex" json:"payment_number"`
	InvoiceID     string    `gorm:"size:36;index" json:"invoice_id"`
	CustomerID    string    `gorm:"size:36;index" json:"customer_id"`
	PaymentDate   time.Time `json:"payment_date"`
	Amount        float64   `gorm:"default:0" json:"amount"`
	PaymentMethod string    `gorm:"size:50" json:"payment_method"`
	Reference     string    `gorm:"size:100" json:"reference"`
	Notes         string    `gorm:"type:text" json:"notes"`
}

type SupplierPayment struct {
	Base
	PaymentNumber     string    `gorm:"size:50;uniqueIndex" json:"payment_number"`
	SupplierInvoiceID string    `gorm:"size:36;index" json:"supplier_invoice_id"`
	SupplierID        string    `gorm:"size:36;index" json:"supplier_id"`
	PaymentDate       time.Time `json:"payment_date"`
	Amount            float64   `gorm:"default:0" json:"amount"`
	PaymentMethod     string    `gorm:"size:50" json:"payment_method"`
	Reference         string    `gorm:"size:100" json:"reference"`
	Notes             string    `gorm:"type:text" json:"notes"`
}

// ============ OPERATIONS ============

type PurchaseOrder struct {
	Base
	PONumber       string     `gorm:"size:50;uniqueIndex" json:"po_number"`
	SupplierID     string     `gorm:"size:36;index" json:"supplier_id"`
	OrderID        string     `gorm:"size:36" json:"order_id"`
	PODate         time.Time  `json:"po_date"`
	ExpectedDate   *time.Time `json:"expected_date"`
	Status         string     `gorm:"size:20;default:'Draft'" json:"status"`
	Subtotal       float64    `gorm:"default:0" json:"subtotal"`
	TaxPct         float64    `gorm:"default:0" json:"tax_pct"`
	TaxAmt         float64    `gorm:"default:0" json:"tax_amt"`
	Total          float64    `gorm:"default:0" json:"total"`
	Notes          string     `gorm:"type:text" json:"notes"`
	ShippingAddress string    `gorm:"type:text" json:"shipping_address"`
}

type PurchaseOrderItem struct {
	Base
	PurchaseOrderID string  `gorm:"size:36;index" json:"purchase_order_id"`
	ProductID       string  `gorm:"size:36" json:"product_id"`
	Description     string  `gorm:"type:text" json:"description"`
	Quantity        float64 `gorm:"default:1" json:"quantity"`
	UnitPrice       float64 `gorm:"default:0" json:"unit_price"`
	Total           float64 `gorm:"default:0" json:"total"`
}

type GoodsReceivedNote struct {
	Base
	GRNNumber       string    `gorm:"size:50;uniqueIndex" json:"grn_number"`
	PurchaseOrderID string    `gorm:"size:36;index" json:"purchase_order_id"`
	SupplierID      string    `gorm:"size:36;index" json:"supplier_id"`
	ReceivedDate    time.Time `json:"received_date"`
	Status          string    `gorm:"size:20;default:'Pending'" json:"status"`
	ReceivedBy      string    `gorm:"size:100" json:"received_by"`
	Notes           string    `gorm:"type:text" json:"notes"`
}

type GRNItem struct {
	Base
	GRNID            string  `gorm:"size:36;index" json:"grn_id"`
	POItemID         string  `gorm:"size:36" json:"po_item_id"`
	ProductID        string  `gorm:"size:36" json:"product_id"`
	QuantityOrdered  float64 `gorm:"default:0" json:"quantity_ordered"`
	QuantityReceived float64 `gorm:"default:0" json:"quantity_received"`
	QuantityAccepted float64 `gorm:"default:0" json:"quantity_accepted"`
	QuantityRejected float64 `gorm:"default:0" json:"quantity_rejected"`
	Notes            string  `gorm:"type:text" json:"notes"`
}

type SupplierInvoice struct {
	Base
	InvoiceNumber   string    `gorm:"size:50" json:"invoice_number"`
	SupplierID      string    `gorm:"size:36;index" json:"supplier_id"`
	PurchaseOrderID string    `gorm:"size:36" json:"purchase_order_id"`
	GRNID           string    `gorm:"size:36" json:"grn_id"`
	InvoiceDate     time.Time `json:"invoice_date"`
	DueDate         time.Time `json:"due_date"`
	Status          string    `gorm:"size:20;default:'Pending'" json:"status"`
	Subtotal        float64   `gorm:"default:0" json:"subtotal"`
	TaxPct          float64   `gorm:"default:0" json:"tax_pct"`
	TaxAmt          float64   `gorm:"default:0" json:"tax_amt"`
	Total           float64   `gorm:"default:0" json:"total"`
	AmountPaid      float64   `gorm:"default:0" json:"amount_paid"`
	Notes           string    `gorm:"type:text" json:"notes"`
}

type SupplierInvoiceItem struct {
	Base
	SupplierInvoiceID string  `gorm:"size:36;index" json:"supplier_invoice_id"`
	ProductID         string  `gorm:"size:36" json:"product_id"`
	Description       string  `gorm:"type:text" json:"description"`
	Quantity          float64 `gorm:"default:1" json:"quantity"`
	UnitPrice         float64 `gorm:"default:0" json:"unit_price"`
	Total             float64 `gorm:"default:0" json:"total"`
}

type DeliveryNote struct {
	Base
	DNNumber      string    `gorm:"size:50;uniqueIndex" json:"dn_number"`
	OrderID       string    `gorm:"size:36;index" json:"order_id"`
	CustomerID    string    `gorm:"size:36;index" json:"customer_id"`
	DeliveryDate  time.Time `json:"delivery_date"`
	Status        string    `gorm:"size:20;default:'Pending'" json:"status"`
	DeliveredBy   string    `gorm:"size:100" json:"delivered_by"`
	ReceivedBy    string    `gorm:"size:100" json:"received_by"`
	Notes         string    `gorm:"type:text" json:"notes"`
	ShippingAddress string  `gorm:"type:text" json:"shipping_address"`
}

type DeliveryNoteItem struct {
	Base
	DeliveryNoteID string  `gorm:"size:36;index" json:"delivery_note_id"`
	OrderItemID    string  `gorm:"size:36" json:"order_item_id"`
	ProductID      string  `gorm:"size:36" json:"product_id"`
	Quantity       float64 `gorm:"default:0" json:"quantity"`
	Notes          string  `gorm:"type:text" json:"notes"`
}

// ============ INTELLIGENCE ============

type Conversation struct {
	Base
	Title     string `gorm:"size:255" json:"title"`
	UserID    string `gorm:"size:36" json:"user_id"`
	IsActive  bool   `gorm:"default:true" json:"is_active"`
}

type ChatMessage struct {
	Base
	ConversationID string `gorm:"size:36;index" json:"conversation_id"`
	Role           string `gorm:"size:20" json:"role"`
	Content        string `gorm:"type:text" json:"content"`
}

type OfferFollowUp struct {
	Base
	OfferID     string     `gorm:"size:36;index" json:"offer_id"`
	ScheduledAt time.Time  `json:"scheduled_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Notes       string     `gorm:"type:text" json:"notes"`
	Status      string     `gorm:"size:20;default:'Pending'" json:"status"`
}

type EntityNote struct {
	Base
	EntityType string `gorm:"size:50;index" json:"entity_type"`
	EntityID   string `gorm:"size:36;index" json:"entity_id"`
	NoteType   string `gorm:"size:50" json:"note_type"`
	Content    string `gorm:"type:text" json:"content"`
	CreatedBy  string `gorm:"size:100" json:"created_by"`
}

type SupplierIssue struct {
	Base
	SupplierID   string     `gorm:"size:36;index" json:"supplier_id"`
	IssueType    string     `gorm:"size:50" json:"issue_type"`
	Description  string     `gorm:"type:text" json:"description"`
	Severity     string     `gorm:"size:20" json:"severity"`
	Status       string     `gorm:"size:20;default:'Open'" json:"status"`
	ResolvedAt   *time.Time `json:"resolved_at"`
	Resolution   string     `gorm:"type:text" json:"resolution"`
}

// ============ SYSTEM ============

type SyncRecord struct {
	Base
	SyncTable     string    `gorm:"size:100" json:"sync_table"`
	RecordID      string    `gorm:"size:36" json:"record_id"`
	SyncedAt      time.Time `json:"synced_at"`
	Direction     string    `gorm:"size:10" json:"direction"`
	ConflictState string    `gorm:"size:20" json:"conflict_state"`
}

type Role struct {
	Base
	Name        string `gorm:"size:50;uniqueIndex" json:"name"`
	DisplayName string `gorm:"size:100" json:"display_name"`
	Description string `gorm:"type:text" json:"description"`
	Permissions string `gorm:"type:text" json:"permissions"`
	IsSystem    bool   `gorm:"default:false" json:"is_system"`
}

type User struct {
	Base
	Username     string     `gorm:"size:100;uniqueIndex" json:"username"`
	Email        string     `gorm:"size:255" json:"email"`
	PasswordHash string     `gorm:"size:255" json:"-"`
	RoleID       string     `gorm:"size:36" json:"role_id"`
	FullName     string     `gorm:"size:255" json:"full_name"`
	DisplayName  string     `gorm:"size:100" json:"display_name"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

type AuditLog struct {
	Base
	UserID     string `gorm:"size:36;index" json:"user_id"`
	Action     string `gorm:"size:50" json:"action"`
	EntityType string `gorm:"size:50" json:"entity_type"`
	EntityID   string `gorm:"size:36" json:"entity_id"`
	OldValue   string `gorm:"type:text" json:"old_value"`
	NewValue   string `gorm:"type:text" json:"new_value"`
	IPAddress  string `gorm:"size:50" json:"ip_address"`
}

type Setting struct {
	Base
	Key         string `gorm:"size:100;uniqueIndex" json:"key"`
	Value       string `gorm:"type:text" json:"value"`
	Description string `gorm:"type:text" json:"description"`
	Category    string `gorm:"size:50" json:"category"`
}

type Device struct {
	Base
	MachineID     string     `gorm:"size:64;uniqueIndex" json:"machine_id"`
	DeviceName    string     `gorm:"size:255" json:"device_name"`
	OSInfo        string     `gorm:"size:100" json:"os_info"`
	FirstSeenAt   time.Time  `json:"first_seen_at"`
	LastSeenAt    *time.Time `json:"last_seen_at"`
	Status        string     `gorm:"size:20;default:'pending'" json:"status"`
	IsAdminDevice bool       `gorm:"default:false" json:"is_admin_device"`
	ApprovedBy    string     `gorm:"size:36" json:"approved_by"`
	ApprovedAt    *time.Time `json:"approved_at"`
}

type DeviceUser struct {
	Base
	DeviceID  string `gorm:"size:36;index" json:"device_id"`
	UserID    string `gorm:"size:36;index" json:"user_id"`
	IsPrimary bool   `gorm:"default:false" json:"is_primary"`
}

// RFQ and Costing
type RFQData struct {
	Base
	RFQNumber    string    `gorm:"size:50" json:"rfq_number"`
	CustomerID   string    `gorm:"size:36;index" json:"customer_id"`
	CustomerName string    `gorm:"size:255" json:"customer_name"`
	ReceivedDate time.Time `json:"received_date"`
	DueDate      *time.Time `json:"due_date"`
	Status       string    `gorm:"size:20;default:'Pending'" json:"status"`
	Description  string    `gorm:"type:text" json:"description"`
	Notes        string    `gorm:"type:text" json:"notes"`
}

type CostingSheetData struct {
	Base
	RFQID          string  `gorm:"size:36;index" json:"rfq_id"`
	FolderNumber   int     `gorm:"default:0" json:"folder_number"`
	TotalCost      float64 `gorm:"default:0" json:"total_cost"`
	TotalSelling   float64 `gorm:"default:0" json:"total_selling"`
	MarginPercent  float64 `gorm:"default:0" json:"margin_percent"`
	Status         string  `gorm:"size:20;default:'Draft'" json:"status"`
	Notes          string  `gorm:"type:text" json:"notes"`
}

type QuickCapture struct {
	Base
	Content     string  `gorm:"type:text" json:"content"`
	Priority    string  `gorm:"size:20;default:'medium'" json:"priority"`
	IsCompleted bool    `gorm:"default:false" json:"is_completed"`
	Source      string  `gorm:"size:50" json:"source"`
}

func main() {
	host := "aws-1-ap-southeast-1.pooler.supabase.com"
	port := "5432"
	user := "postgres.jgpwtuyfndtuxxyrytel"
	password := os.Getenv("SUPABASE_DB_PASSWORD")
	if password == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dbname := "postgres"

	fmt.Println("========================================")
	fmt.Println("   SUPABASE SCHEMA MIGRATION")
	fmt.Println("========================================")

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		host, port, user, password, dbname,
	)

	fmt.Println("\nConnecting to Supabase...")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	fmt.Println("Connected!")

	fmt.Println("\nRunning AutoMigrate...")
	start := time.Now()

	err = db.AutoMigrate(
		// Master Data
		&CustomerMaster{},
		&CustomerContact{},
		&SupplierMaster{},
		&SupplierContact{},
		&ProductMaster{},
		&EntityNote{},
		&SupplierIssue{},
		// Sales
		&Offer{},
		&OfferItem{},
		&Order{},
		&OrderItem{},
		&RFQData{},
		&CostingSheetData{},
		// Finance
		&Invoice{},
		&DBInvoiceItem{},
		&Payment{},
		&SupplierPayment{},
		// Operations
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&DeliveryNote{},
		&DeliveryNoteItem{},
		// Intelligence
		&Conversation{},
		&ChatMessage{},
		&OfferFollowUp{},
		&QuickCapture{},
		// System
		&SyncRecord{},
		&Role{},
		&User{},
		&AuditLog{},
		&Setting{},
		&Device{},
		&DeviceUser{},
	)

	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("Migration complete in %v\n", elapsed)

	// Count tables
	var tables []string
	db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public'").Scan(&tables)
	fmt.Printf("\nTables created: %d\n", len(tables))
	for _, t := range tables {
		fmt.Printf("  - %s\n", t)
	}

	// Seed default roles
	fmt.Println("\nSeeding default roles...")
	roles := []Role{
		{Base: Base{ID: "role-admin"}, Name: "admin", DisplayName: "Administrator", Permissions: `["*"]`, IsSystem: true},
		{Base: Base{ID: "role-manager"}, Name: "manager", DisplayName: "Manager", Permissions: `["customers:*","suppliers:*","orders:*","offers:*","finance:view","intelligence:chat"]`, IsSystem: true},
		{Base: Base{ID: "role-sales"}, Name: "sales", DisplayName: "Sales", Permissions: `["customers:*","offers:*","orders:view","intelligence:chat"]`, IsSystem: true},
		{Base: Base{ID: "role-operations"}, Name: "operations", DisplayName: "Operations", Permissions: `["orders:*","suppliers:view","inventory:*"]`, IsSystem: true},
		{Base: Base{ID: "role-staff"}, Name: "staff", DisplayName: "Staff", Permissions: `["dashboard:view"]`, IsSystem: true},
	}

	for _, role := range roles {
		var existing Role
		if db.Where("name = ?", role.Name).First(&existing).Error != nil {
			role.CreatedAt = time.Now()
			role.UpdatedAt = time.Now()
			db.Create(&role)
			fmt.Printf("  Created role: %s\n", role.DisplayName)
		} else {
			fmt.Printf("  Role exists: %s\n", role.DisplayName)
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("   MIGRATION COMPLETE!")
	fmt.Println("========================================")

	sqlDB, _ := db.DB()
	sqlDB.Close()
}
