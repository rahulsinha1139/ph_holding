package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Base struct with UUID and timestamps
type Base struct {
	ID        string         `gorm:"primaryKey;size:36" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Version   int            `gorm:"default:1" json:"version"`
	CreatedBy string         `json:"created_by"`
}

// Minimal type definitions for migration
// (In production, these would import from main package)

// Post-Sale Note Tracking
type PostSaleNote struct {
	Base
	OrderID     string `gorm:"index;size:36" json:"order_id"`
	OrderNumber string `gorm:"index;size:50" json:"order_number"`

	NoteType    string    `gorm:"index;size:50" json:"note_type"` // repair, replace, reinstall, warranty, refund, other
	Description string    `gorm:"type:text" json:"description"`
	CostBHD     float64   `json:"cost_bhd"`
	NoteDate    time.Time `json:"note_date"`

	// Resolution tracking
	ResolvedAt *time.Time `json:"resolved_at"`
	Resolution string     `gorm:"type:text" json:"resolution"`
}

func (PostSaleNote) TableName() string { return "post_sale_notes" }

// Costing Sheet System
type DBCostingSheet struct {
	Base
	CostingNumber string `gorm:"uniqueIndex;size:50" json:"costing_number"`
	CustomerID    string `gorm:"index;size:36" json:"customer_id"`
	CustomerName  string `json:"customer_name"`

	// Dates
	CostingDate time.Time `json:"costing_date"`
	ValidUntil  time.Time `json:"valid_until"`

	// Line Item Totals
	SubtotalBHD    float64 `json:"subtotal_bhd"`
	TotalMarginBHD float64 `json:"total_margin_bhd"`

	// Bahrain Logistics Costs
	ShippingCostBHD  float64 `json:"shipping_cost_bhd"`
	CustomsDutyBHD   float64 `json:"customs_duty_bhd"`
	ClearanceCostBHD float64 `json:"clearance_cost_bhd"`
	HandlingCostBHD  float64 `json:"handling_cost_bhd"`

	// Additional Costs Total
	AdditionalCostsBHD float64 `json:"additional_costs_bhd"`

	// Grand Total
	GrandTotalBHD float64 `json:"grand_total_bhd"`

	// Status
	Status             string `gorm:"index;size:50" json:"status"` // Draft, Approved, Converted
	ConvertedToOfferID string `gorm:"size:36" json:"converted_to_offer_id"`

	// Relationships
	Items           []DBCostingItem           `gorm:"foreignKey:CostingSheetID" json:"items,omitempty"`
	AdditionalCosts []DBCostingAdditionalCost `gorm:"foreignKey:CostingSheetID" json:"additional_costs,omitempty"`
}

func (DBCostingSheet) TableName() string { return "costing_sheets" }

// DBCostingItem represents a line item in a costing sheet (DB model)
type DBCostingItem struct {
	Base
	CostingSheetID string `gorm:"index;size:36" json:"costing_sheet_id"`
	LineNumber     int    `json:"line_number"`

	ProductID   string `gorm:"size:36" json:"product_id"`
	ProductType string `gorm:"size:100" json:"product_type"`
	Description string `gorm:"type:text" json:"description"`

	Quantity      float64 `json:"quantity"`
	UnitCostBHD   float64 `json:"unit_cost_bhd"`
	MarginPercent float64 `json:"margin_percent"`
	UnitPriceBHD  float64 `json:"unit_price_bhd"`
	LineTotalBHD  float64 `json:"line_total_bhd"`
}

func (DBCostingItem) TableName() string { return "costing_items" }

// DBCostingAdditionalCost represents ad-hoc costs in a costing sheet (DB model)
type DBCostingAdditionalCost struct {
	Base
	CostingSheetID string  `gorm:"index;size:36" json:"costing_sheet_id"`
	Description    string  `json:"description"`
	AmountBHD      float64 `json:"amount_bhd"`
}

func (DBCostingAdditionalCost) TableName() string { return "costing_additional_costs" }

// ChartOfAccount Enhancement
type ChartOfAccount struct {
	Base
	AccountCode string  `gorm:"uniqueIndex;size:20" json:"account_code"`
	AccountName string  `json:"account_name"`
	AccountType string  `gorm:"index;size:50" json:"account_type"`
	Balance     float64 `json:"balance"`
	IsActive    bool    `gorm:"default:true" json:"is_active"`

	// VAT Tracking (NEW)
	IsVATAccount bool   `gorm:"index" json:"is_vat_account"`
	VATDirection string `gorm:"size:20" json:"vat_direction"` // "input", "output", or null

	// Hierarchy (NEW)
	ParentAccountID string `gorm:"index;size:36" json:"parent_account_id"`
	AccountGroup    string `gorm:"index;size:10" json:"account_group"` // "BS" or "PL"
}

func (ChartOfAccount) TableName() string { return "chart_of_accounts" }

type PredictionRecord struct {
	ID            uint   `gorm:"primaryKey"`
	CustomerID    string `gorm:"index"`
	CustomerName  string
	Grade         string `gorm:"index"`
	PredictedDays int
	Confidence    float64
	R1, R2, R3    float64
	CreatedAt     time.Time `gorm:"index"`
	UpdatedAt     time.Time
}

func (PredictionRecord) TableName() string { return "prediction_records" }

type CustomerSnapshot struct {
	ID            uint   `gorm:"primaryKey"`
	PredictionID  uint   `gorm:"index"`
	CustomerID    string `gorm:"index"`
	BusinessName  string
	OrderValue    float64
	RelationYears int
	Industry      string
	Country       string
	IsEmergency   bool
	HasABB        bool
	DisputeCount  int
	CreatedAt     time.Time
}

type ActualOutcome struct {
	ID               uint   `gorm:"primaryKey"`
	PredictionID     uint   `gorm:"index;unique"`
	CustomerID       string `gorm:"index"`
	ActualPaymentDay int
	WasPaid          bool
	WasDispute       bool
	Notes            string
	RecordedAt       time.Time
	CreatedAt        time.Time
}

type CustomerMaster struct {
	ID                 uint   `gorm:"primaryKey"`
	CustomerID         string `gorm:"uniqueIndex;size:50"`
	CustomerType       string `gorm:"index;size:50"`
	BusinessName       string `gorm:"index;size:255"`
	ShortCode          string `gorm:"size:10"`
	AddressLine1       string `gorm:"size:255"`
	AddressLine2       string `gorm:"size:255"`
	City               string `gorm:"index;size:100"`
	Country            string `gorm:"index;size:100"`
	PostalCode         string `gorm:"size:50"`
	TRN                string `gorm:"size:100"`
	Industry           string `gorm:"index;size:100"`
	RelationYears      int
	PaymentGrade       string `gorm:"index;size:10"`
	PaymentTermsDays   int
	AvgPaymentDays     float64
	DisputeCount       int
	TotalOrdersValue   float64
	TotalOrdersCount   int
	AvgOrderValue      float64
	LastOrderDate      *time.Time
	HasABBCompetition  bool
	IsEmergencyOnly    bool
	IsCreditBlocked    bool
	RequiresPrepayment bool
	CreatedAt          time.Time `gorm:"index"`
	UpdatedAt          time.Time
	CreatedBy          string     `gorm:"size:100"`
	UpdatedBy          string     `gorm:"size:100"`
	DeletedAt          *time.Time `gorm:"index"`
}

func (CustomerMaster) TableName() string { return "customers" }

type CustomerContact struct {
	ID               uint   `gorm:"primaryKey"`
	CustomerID       uint   `gorm:"index"`
	ContactName      string `gorm:"index;size:255"`
	JobTitle         string `gorm:"size:255"`
	Department       string `gorm:"size:100"`
	Email            string `gorm:"size:255"`
	Phone            string `gorm:"size:50"`
	Mobile           string `gorm:"size:50"`
	WhatsApp         string `gorm:"size:50"`
	IsPrimaryContact bool
	IsDecisionMaker  bool
	CanApproveOrders bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time `gorm:"index"`
}

func (CustomerContact) TableName() string { return "customer_contacts" }

type SupplierMaster struct {
	Base
	SupplierCode string `gorm:"uniqueIndex;size:50" json:"supplier_code"`
	SupplierName string `gorm:"index;size:255" json:"supplier_name"`
	Country      string `gorm:"index;size:100" json:"country"`
	LeadTimeDays int    `json:"lead_time_days"`

	// Identity Section
	TaxID        string `gorm:"size:100" json:"tax_id"`
	SupplierType string `gorm:"index;size:50" json:"supplier_type"` // Manufacturer, Distributor, Agent

	// Offerings Section
	BrandsHandled string `gorm:"type:text" json:"brands_handled"` // JSON array of brand names
	ProductTypes  string `gorm:"type:text" json:"product_types"`  // JSON array

	// Contact Section
	PrimaryContact string `json:"primary_contact"`
	Email          string `json:"email"`
	Phone          string `json:"phone"`
	Address        string `gorm:"type:text" json:"address"`

	// Bank Details Section (for payments)
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	IBAN          string `gorm:"size:50" json:"iban"`
	SwiftCode     string `gorm:"size:20" json:"swift_code"`

	// Rating Section
	Rating int    `gorm:"default:3" json:"rating"` // 1-5 stars
	Notes  string `gorm:"type:text" json:"notes"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

type ProductMaster struct {
	Base
	ProductCode      string  `gorm:"uniqueIndex;size:100" json:"product_code"`
	ProductName      string  `gorm:"index;size:255" json:"product_name"`
	ProductCategory  string  `gorm:"index;size:100" json:"product_category"`
	SupplierID       string  `gorm:"index;size:36" json:"supplier_id"`
	SupplierCode     string  `gorm:"index;size:50" json:"supplier_code"`
	StandardCostBHD  float64 `json:"standard_cost_bhd"`
	StandardPriceBHD float64 `json:"standard_price_bhd"`
	Description      string  `gorm:"type:text" json:"description"`
	IsActive         bool    `gorm:"index" json:"is_active"`
	StockQuantity    int     `json:"stock_quantity"`

	// Identity Section (NEW)
	SKU        string `gorm:"index;size:100" json:"sku"`
	PartNumber string `gorm:"size:100" json:"part_number"`

	// Commercial Section (NEW)
	HSCode        string `gorm:"size:20" json:"hs_code"` // Harmonized System Code
	UnitOfMeasure string `gorm:"size:20" json:"unit_of_measure"`

	// Technical Section (NEW)
	DatasheetURL   string `gorm:"type:text" json:"datasheet_url"`
	Specifications string `gorm:"type:text" json:"specifications"` // JSON
}

func (ProductMaster) TableName() string { return "products" }

type Offer struct {
	ID                 uint       `gorm:"primaryKey"`
	OfferNumber        string     `gorm:"uniqueIndex;size:50"`
	RevisionNumber     int        `gorm:"index"`
	CustomerID         uint       `gorm:"index"`
	CustomerName       string     `gorm:"index;size:255"`
	RFQReceivedDate    time.Time  `gorm:"index"`
	QuotationDate      time.Time  `gorm:"index"`
	ValidityDate       time.Time  `gorm:"index"`
	FollowUpDate       *time.Time `gorm:"index"`
	TotalValueBHD      float64
	DiscountPercent    float64
	NetValueBHD        float64
	EstimatedMargin    float64
	Stage              string `gorm:"index;size:50"`
	WinProbability     float64
	HasABBCompetition  bool
	CompetitorNotes    string `gorm:"type:text"`
	IsEmergency        bool
	PaymentTerms       string `gorm:"size:100"`
	DeliveryTerms      string `gorm:"size:255"`
	WonDate            *time.Time
	LostDate           *time.Time
	LostReason         string `gorm:"type:text"`
	ConvertedToOrderID *uint
	CreatedAt          time.Time `gorm:"index"`
	UpdatedAt          time.Time
	CreatedBy          string     `gorm:"size:100"`
	UpdatedBy          string     `gorm:"size:100"`
	DeletedAt          *time.Time `gorm:"index"`
}

func (Offer) TableName() string { return "offers" }

type OfferItem struct {
	ID              uint `gorm:"primaryKey"`
	OfferID         uint `gorm:"index"`
	LineNumber      int
	ProductID       *uint  `gorm:"index"`
	ProductCode     string `gorm:"size:100"`
	Description     string `gorm:"type:text"`
	Quantity        float64
	UnitOfMeasure   string `gorm:"size:50"`
	UnitCostBHD     float64
	UnitPriceBHD    float64
	DiscountPercent float64
	LineTotalBHD    float64
	MarginPercent   float64
	LeadTimeDays    int
	SupplierCode    string `gorm:"size:50"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (OfferItem) TableName() string { return "offer_items" }

type Order struct {
	ID                   uint      `gorm:"primaryKey"`
	OrderNumber          string    `gorm:"uniqueIndex;size:50"`
	CustomerPONumber     string    `gorm:"index;size:100"`
	CustomerID           uint      `gorm:"index"`
	CustomerName         string    `gorm:"index;size:255"`
	OrderDate            time.Time `gorm:"index"`
	RequiredDate         time.Time
	PromisedDate         time.Time
	OfferID              *uint `gorm:"index"`
	TotalValueBHD        float64
	VATPercent           float64
	VATAmountBHD         float64
	GrandTotalBHD        float64
	PaymentTerms         string `gorm:"size:100"`
	PaymentGrade         string `gorm:"index;size:10"`
	PredictedPaymentDays int
	DeliveryTerms        string    `gorm:"size:255"`
	DeliveryAddress      string    `gorm:"type:text"`
	Status               string    `gorm:"index;size:50"`
	CreatedAt            time.Time `gorm:"index"`
	UpdatedAt            time.Time
	CreatedBy            string     `gorm:"size:100"`
	DeletedAt            *time.Time `gorm:"index"`
}

func (Order) TableName() string { return "orders" }

type OrderItem struct {
	ID               uint `gorm:"primaryKey"`
	OrderID          uint `gorm:"index"`
	LineNumber       int
	ProductID        *uint  `gorm:"index"`
	ProductCode      string `gorm:"size:100"`
	Description      string `gorm:"type:text"`
	Quantity         float64
	UnitOfMeasure    string `gorm:"size:50"`
	UnitPriceBHD     float64
	LineTotalBHD     float64
	UnitCostBHD      float64
	ActualMargin     float64
	QuantityShipped  float64
	QuantityInvoiced float64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (OrderItem) TableName() string { return "order_items" }

type Invoice struct {
	ID                   uint      `gorm:"primaryKey"`
	InvoiceNumber        string    `gorm:"uniqueIndex;size:50"`
	InvoiceDate          time.Time `gorm:"index"`
	CustomerID           uint      `gorm:"index"`
	CustomerName         string    `gorm:"index;size:255"`
	CustomerAddress      string    `gorm:"type:text"`
	CustomerTRN          string    `gorm:"size:100"`
	OrderID              *uint     `gorm:"index"`
	CustomerPONumber     string    `gorm:"index;size:100"`
	DeliveryNoteNumber   string    `gorm:"size:100"`
	SubtotalBHD          float64
	VATPercent           float64
	VATAmountBHD         float64
	GrandTotalBHD        float64
	PaymentTerms         string    `gorm:"size:100"`
	DueDate              time.Time `gorm:"index"`
	PredictedPaymentDays int
	PredictionGrade      string     `gorm:"index;size:10"`
	Status               string     `gorm:"index;size:50"`
	PaidDate             *time.Time `gorm:"index"`
	PaidAmountBHD        float64
	OutstandingBHD       float64
	DaysOutstanding      int
	AgingBucket          string    `gorm:"index;size:50"`
	CreatedAt            time.Time `gorm:"index"`
	UpdatedAt            time.Time
	CreatedBy            string     `gorm:"size:100"`
	DeletedAt            *time.Time `gorm:"index"`
}

func (Invoice) TableName() string { return "invoices" }

type DBInvoiceItem struct {
	ID              uint `gorm:"primaryKey"`
	InvoiceID       uint `gorm:"index"`
	LineNumber      int
	Description     string `gorm:"type:text"`
	Quantity        float64
	UnitOfMeasure   string `gorm:"size:50"`
	Rate            float64
	AnnualTotal     float64
	MonthlyAmount   float64
	VATPercent      float64
	TaxableValueBHD float64
	VATAmountBHD    float64
	TotalIncVATBHD  float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (DBInvoiceItem) TableName() string { return "invoice_items" }

type Payment struct {
	ID              uint      `gorm:"primaryKey"`
	PaymentNumber   string    `gorm:"uniqueIndex;size:50"`
	PaymentDate     time.Time `gorm:"index"`
	CustomerID      uint      `gorm:"index"`
	CustomerName    string    `gorm:"index;size:255"`
	InvoiceID       *uint     `gorm:"index"`
	InvoiceNumber   string    `gorm:"index;size:50"`
	AmountBHD       float64
	PaymentMethod   string `gorm:"size:100"`
	ReferenceNumber string `gorm:"size:255"`
	InvoiceDateRef  *time.Time
	DaysToPayment   int
	PredictedDays   int
	VarianceDays    int
	Notes           string    `gorm:"type:text"`
	CreatedAt       time.Time `gorm:"index"`
	UpdatedAt       time.Time
	CreatedBy       string     `gorm:"size:100"`
	DeletedAt       *time.Time `gorm:"index"`
}

func (Payment) TableName() string { return "payments" }

type Shipment struct {
	ID              uint       `gorm:"primaryKey"`
	ShipmentNumber  string     `gorm:"uniqueIndex;size:50"`
	ShipmentDate    time.Time  `gorm:"index"`
	OrderID         uint       `gorm:"index"`
	OrderNumber     string     `gorm:"index;size:50"`
	CustomerID      uint       `gorm:"index"`
	CourierName     string     `gorm:"size:255"`
	TrackingNumber  string     `gorm:"size:255"`
	DeliveryAddress string     `gorm:"type:text"`
	Destination     string     `gorm:"size:255"`
	Status          string     `gorm:"index;size:50"`
	DeliveredDate   *time.Time `gorm:"index"`
	SignedBy        string     `gorm:"size:255"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time `gorm:"index"`
}

func (Shipment) TableName() string { return "shipments" }

type CostingHistory struct {
	ID               uint      `gorm:"primaryKey"`
	RecordDate       time.Time `gorm:"index"`
	ProductID        uint      `gorm:"index"`
	ProductCode      string    `gorm:"index;size:100"`
	SupplierCostBHD  float64
	StandardPriceBHD float64
	StandardMargin   float64
	ABBPriceBHD      float64
	MarketNotes      string `gorm:"type:text"`
	SourceType       string `gorm:"size:50"`
	SourceID         *uint
	CreatedAt        time.Time
	CreatedBy        string `gorm:"size:100"`
}

func (CostingHistory) TableName() string { return "costing_history" }

type GradeChange struct {
	ID                 uint      `gorm:"primaryKey"`
	ChangeDate         time.Time `gorm:"index"`
	CustomerID         uint      `gorm:"index"`
	CustomerName       string    `gorm:"index;size:255"`
	OldGrade           string    `gorm:"size:10"`
	NewGrade           string    `gorm:"index;size:10"`
	ChangeReason       string    `gorm:"type:text"`
	TriggerType        string    `gorm:"size:50"`
	AvgPaymentDays     float64
	RecentDisputeCount int
	CreatedAt          time.Time
	CreatedBy          string `gorm:"size:100"`
}

func (GradeChange) TableName() string { return "grade_changes" }

type SyncStatus struct {
	ID              uint       `gorm:"primaryKey"`
	FilePath        string     `gorm:"uniqueIndex;size:1000"`
	FileName        string     `gorm:"index;size:255"`
	FileType        string     `gorm:"index;size:50"`
	SyncStatus      string     `gorm:"index;size:50"`
	LastSyncTime    *time.Time `gorm:"index"`
	OneDrivePath    string     `gorm:"size:1000"`
	SharePointPath  string     `gorm:"size:1000"`
	FileHashMD5     string     `gorm:"size:32"`
	FileSize        int64
	IsProcessed     bool `gorm:"index"`
	ProcessedDate   *time.Time
	ProcessingError string    `gorm:"type:text"`
	EntityType      string    `gorm:"index;size:50"`
	EntityID        *uint     `gorm:"index"`
	CreatedAt       time.Time `gorm:"index"`
	UpdatedAt       time.Time
}

func (SyncStatus) TableName() string { return "sync_status" }

type FileWatchEvent struct {
	ID               uint      `gorm:"primaryKey"`
	EventTime        time.Time `gorm:"index"`
	EventType        string    `gorm:"index;size:50"`
	FilePath         string    `gorm:"index;size:1000"`
	OldPath          string    `gorm:"size:1000"`
	FileName         string    `gorm:"index;size:255"`
	FileExtension    string    `gorm:"index;size:50"`
	FileSize         int64
	IsProcessed      bool `gorm:"index"`
	ProcessedAt      *time.Time
	ProcessingNotes  string `gorm:"type:text"`
	GeometryPipeline string `gorm:"index;size:100"`
	RoutingReason    string `gorm:"type:text"`
	CreatedAt        time.Time
}

func (FileWatchEvent) TableName() string { return "file_watch_events" }

type QuickCapture struct {
	ID        uint `gorm:"primaryKey"`
	Title     string
	Content   string
	Tags      string // Comma-separated tags
	Priority  string // Low, Medium, High, Urgent
	Status    string // Open, InProgress, Done
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (QuickCapture) TableName() string { return "quick_captures" }

func main() {
	fmt.Println("=== PH Holdings Schema Migration ===")
	fmt.Println()

	// Connect to database
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	fmt.Println("Migrating schema...")

	// Migrate all tables
	err = db.AutoMigrate(
		// Predictions (existing)
		&PredictionRecord{},
		&CustomerSnapshot{},
		&ActualOutcome{},

		// Master Data
		&CustomerMaster{},
		&CustomerContact{},
		&SupplierMaster{},
		&ProductMaster{},

		// Transactions
		&Offer{},
		&OfferItem{},
		&Order{},
		&OrderItem{},
		&Invoice{},
		&DBInvoiceItem{},
		&Payment{},
		&Shipment{},

		// Post-Sale Tracking
		&PostSaleNote{},

		// Costing System
		&DBCostingSheet{},
		&DBCostingItem{},
		&DBCostingAdditionalCost{},

		// Finance
		&ChartOfAccount{},

		// Analytics
		&CostingHistory{},
		&GradeChange{},

		// Sync
		&SyncStatus{},
		&FileWatchEvent{},

		// Quick Captures
		&QuickCapture{},
	)

	if err != nil {
		log.Fatal("Migration failed:", err)
	}

	fmt.Println("✓ Migration complete!")
	fmt.Println()

	// Query table count
	var tables []string
	db.Raw("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name").Scan(&tables)

	fmt.Printf("Total Tables: %d\n\n", len(tables))
	fmt.Println("Tables Created:")
	for i, table := range tables {
		var count int64
		db.Table(table).Count(&count)
		fmt.Printf("  %2d. %-30s (%d rows)\n", i+1, table, count)
	}

	fmt.Println("\n🎉 Full ERP schema deployed!")
}
