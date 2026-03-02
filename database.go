package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// =============================================================================
// INFRASTRUCTURE: UUID & SYNC BASE
// =============================================================================

// Base provides common fields for all database models including UUID primary keys,
// timestamps, soft deletes, optimistic locking (Version), and audit trail (CreatedBy).
type Base struct {
	ID        string         `gorm:"primaryKey;size:36" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Version   int            `gorm:"default:1" json:"version"`
	CreatedBy string         `json:"created_by"`
}

func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

// =============================================================================
// MODULE: CRM & MASTER DATA
// =============================================================================

// CustomerMaster represents a customer in the CRM system with payment grading,
// AR tracking, credit limits, and relationship history.
type CustomerMaster struct {
	Base
	CustomerID   string `gorm:"uniqueIndex;size:50" json:"customer_id"`
	CustomerCode string `gorm:"uniqueIndex;size:50" json:"customer_code"` // Alternative business identifier
	CustomerType string `gorm:"index;size:50" json:"customer_type"`
	BusinessName string `gorm:"index;size:255" json:"business_name"`
	ShortCode    string `gorm:"size:10" json:"short_code"`

	// Contact & Regional
	AddressLine1 string `json:"address_line1"`
	City         string `gorm:"index;size:100" json:"city"`
	Country      string `gorm:"index;size:100" json:"country"`
	TRN          string `gorm:"size:100" json:"trn"`

	// Business Details
	Industry      string `gorm:"index;size:100" json:"industry"`
	RelationYears int    `json:"relation_years"`

	// Performance Profile
	// PaymentGrade has standalone index for grade-only queries: WHERE payment_grade = ?
	// Used by: GetCustomersByGrade() - retrieves all customers of a specific grade (A/B/C/D)
	// No composite index needed here - CustomerID is uniqueIndex (1:1 lookup), PaymentGrade is low cardinality
	PaymentGrade     string  `gorm:"index;size:10;default:'C'" json:"payment_grade"`     // A/B/C/D grade (P1 Fix: default)
	CustomerGrade    string  `gorm:"index;size:10;default:'C'" json:"customer_grade"`    // Overall customer grade (P1 Fix: default)
	PaymentTermsDays int     `gorm:"index" json:"payment_terms_days"`                    // P1 Fix: indexed for payment terms queries
	AvgPaymentDays   float64 `json:"avg_payment_days"`
	DisputeCount     int     `json:"dispute_count"`

	// Financial Metrics
	TotalOrdersValue float64    `json:"total_orders_value"`
	TotalOrdersCount int        `json:"total_orders_count"`
	AvgOrderValue    float64    `json:"avg_order_value"`
	LastOrderDate    *time.Time `json:"last_order_date"`

	// AR Tracking (Accounts Receivable)
	ARRiskTier     string  `gorm:"index;size:20" json:"ar_risk_tier"`      // Low/Medium/High/Critical
	OutstandingBHD float64 `json:"outstanding_bhd"`                        // Total outstanding amount
	OverdueDays    int     `gorm:"index" json:"overdue_days"`              // Days overdue for oldest invoice
	CreditLimitBHD float64 `gorm:"default:50000" json:"credit_limit_bhd"`  // P1 FIX: Credit limit in BHD (default 50,000)

	// Flags (PostgreSQL boolean types)
	IsCreditBlocked    bool `gorm:"index;default:false" json:"is_credit_blocked"`
	RequiresPrepayment bool `gorm:"default:false" json:"requires_prepayment"`
	HasABBCompetition  bool `gorm:"default:false" json:"has_abb_competition"`
	IsEmergencyOnly    bool `gorm:"default:false" json:"is_emergency_only"`
}

func (CustomerMaster) TableName() string { return "customers" }

// CustomerContact stores multiple contact persons for a customer account.
type CustomerContact struct {
	Base
	CustomerID       string `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`
	ContactName      string `gorm:"size:255" json:"contact_name"`
	JobTitle         string `gorm:"size:100" json:"job_title"`
	Email            string `gorm:"size:255" json:"email"`
	Phone            string `gorm:"size:50" json:"phone"`
	Address          string `gorm:"type:varchar(1000)" json:"address"`
	IsPrimaryContact bool   `json:"is_primary_contact"`
}

func (CustomerContact) TableName() string { return "customer_contacts" }

// SupplierContact stores multiple contact persons for a supplier account.
type SupplierContact struct {
	Base
	SupplierID       string `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:SupplierID;references:ID" json:"supplier_id"`
	ContactName      string `gorm:"size:255" json:"contact_name"`
	JobTitle         string `gorm:"size:100" json:"job_title"`
	Email            string `gorm:"size:255" json:"email"`
	Phone            string `gorm:"size:50" json:"phone"`
	Address          string `gorm:"type:varchar(1000)" json:"address"`
	IsPrimaryContact bool   `json:"is_primary_contact"`
}

func (SupplierContact) TableName() string { return "supplier_contacts" }

// SupplierMaster represents a supplier in the procurement system with ratings,
// lead times, payment terms, and bank details for payments.
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
	BrandsHandled string `gorm:"type:varchar(2000)" json:"brands_handled"` // JSON array of brand names
	ProductTypes  string `gorm:"type:varchar(2000)" json:"product_types"`  // JSON array

	// Contact Section
	PrimaryContact string `gorm:"size:255" json:"primary_contact"`
	Email          string `gorm:"size:255" json:"email"`
	Phone          string `gorm:"size:50" json:"phone"`
	Address        string `gorm:"type:varchar(1000)" json:"address"`

	// Bank Details Section (for payments)
	BankName      string `json:"bank_name"`
	AccountNumber string `json:"account_number"`
	IBAN          string `gorm:"size:50" json:"iban"`
	SwiftCode     string `gorm:"size:20" json:"swift_code"`

	// Commercial Terms
	PaymentTerms string `gorm:"index;size:100;default:'Net 30'" json:"payment_terms"` // P1 Fix: indexed for payment terms queries

	// Rating Section
	Rating int    `gorm:"index;default:3" json:"rating"` // P1 Fix: indexed for star rating queries
	Notes  string `gorm:"type:varchar(5000)" json:"notes"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

// EntityNote stores notes for customers and suppliers
type EntityNote struct {
	Base
	EntityType string `gorm:"index;size:20;check:entity_type IN ('customer','supplier')" json:"entity_type"` // "customer" or "supplier"
	EntityID   string `gorm:"index;size:36" json:"entity_id"`
	NoteType   string `gorm:"index;size:50;check:note_type IN ('delivery','issue','general','payment','commercial','technical')" json:"note_type"` // "delivery", "issue", "general", "payment"
	Content    string `gorm:"type:varchar(5000)" json:"content"`
}

func (EntityNote) TableName() string { return "entity_notes" }

// SupplierIssue tracks instrumentation problems with suppliers
type SupplierIssue struct {
	Base
	SupplierID  string     `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:SupplierID;references:ID" json:"supplier_id"`
	OrderRef    string     `gorm:"size:100" json:"order_ref"`
	Description string     `gorm:"type:varchar(5000)" json:"description"`
	Status      string     `gorm:"index;size:50;check:status IN ('open','pending','resolved','escalated')" json:"status"` // "open", "pending", "resolved"
	Resolution  string     `gorm:"type:varchar(5000)" json:"resolution"`
	CostBHD     float64    `gorm:"check:cost_bhd >= 0" json:"cost_bhd"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}

func (SupplierIssue) TableName() string { return "supplier_issues" }

type ProductMaster struct {
	Base
	ProductCode      string  `gorm:"uniqueIndex;size:100" json:"product_code"`
	ProductName      string  `gorm:"index;size:255" json:"product_name"`
	ProductCategory  string  `gorm:"index;size:100" json:"product_category"`
	SupplierID       string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT;" json:"supplier_id"`
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
	DatasheetURL   string `gorm:"type:varchar(2000)" json:"datasheet_url"`
	Specifications string `gorm:"type:varchar(5000)" json:"specifications"` // JSON

	// Serial Tracking (Phase 23)
	RequiresSerialTracking bool `gorm:"default:false" json:"requires_serial_tracking"`
}

func (ProductMaster) TableName() string { return "products" }

// =============================================================================
// MODULE: SALES & RFQ
// =============================================================================

// Offer represents a customer quotation with pricing, terms, and linked items.
// Maps to RFQ opportunities and can be won/lost to create Orders.
type Offer struct {
	Base
	OfferNumber    string `gorm:"uniqueIndex;size:50" json:"offer_number"`
	RevisionNumber int    `gorm:"index" json:"revision_number"`

	// Relationships
	RFQID        string `gorm:"index;size:36" json:"rfq_id"` // Fixed: was uint, now string for consistency with Order.RFQID
	CustomerID   string `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`
	CustomerName string `gorm:"index;size:255" json:"customer_name"`

	// Dates
	// QuotationDate indexed for dashboard date range filters: WHERE quotation_date BETWEEN ? AND ?
	// Used by: Dashboard timeline, monthly/quarterly reports, offer aging analysis
	QuotationDate time.Time `gorm:"index" json:"quotation_date"`
	ValidityDate  time.Time `json:"validity_date"`

	// Metrics
	TotalValueBHD   float64 `gorm:"not null;default:0;check:total_value_bhd >= 0" json:"total_value_bhd"`
	EstimatedMargin float64 `gorm:"not null;default:0" json:"estimated_margin"`
	Stage           string  `gorm:"index;size:50;check:stage IN ('RFQ','Quoted','Won','Lost','Expired')" json:"stage"` // RFQ, Quoted, Won, Lost

	// Competition
	HasABBCompetition bool   `json:"has_abb_competition"`
	LostReason        string `gorm:"type:varchar(2000)" json:"lost_reason"`

	// PDF Generation Fields (Terms & Conditions)
	PaymentTerms       string `gorm:"type:varchar(500);default:'30 days from Date of Delivery'" json:"payment_terms"`
	DeliveryTerms      string `gorm:"type:varchar(500);default:'DAP Bahrain at your store or PH Trading'" json:"delivery_terms"`
	DeliveryWeeks      string `gorm:"size:50;default:'4-6 weeks'" json:"delivery_weeks"`
	CountryOfOrigin    string `gorm:"size:100;default:'Germany / USA'" json:"country_of_origin"`
	IssuedBy           string `gorm:"size:100" json:"issued_by"`
	ContactPhone       string `gorm:"size:50" json:"contact_phone"`
	CustomerReference  string `gorm:"size:255" json:"customer_reference"` // RFQ/enquiry reference
	AttentionPerson    string `gorm:"size:255" json:"attention_person"`   // Contact name
	AttentionCompany   string `gorm:"size:255" json:"attention_company"`
	AttentionPhone     string `gorm:"size:50" json:"attention_phone"`
	AttentionAddress   string `gorm:"type:varchar(1000)" json:"attention_address"`
	DiscountPercent    float64 `json:"discount_percent"`

	// Additional costing header fields
	CocCoo          string `gorm:"size:50" json:"coc_coo"`                   // Certificate of Conformity/Country of Origin
	TestCertificate string `gorm:"type:varchar(500)" json:"test_certificate"` // Test Certificate requirements
	Installation    string `gorm:"size:50" json:"installation"`              // Installation included?
	Commissioning   string `gorm:"size:50" json:"commissioning"`             // Commissioning included?
	Testing         string `gorm:"type:varchar(500)" json:"testing"`          // Testing requirements

	Items []OfferItem `gorm:"foreignKey:OfferID" json:"items,omitempty"`
}

func (Offer) TableName() string { return "offers" }

// OfferItem represents a line item in an offer with full costing data from the
// costing sheet including FOB, freight, margins, and detailed specifications.
type OfferItem struct {
	Base
	OfferID    string `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:OfferID;references:ID" json:"offer_id"`
	LineNumber int    `json:"line_number"`

	ProductID   string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:ProductID;references:ID" json:"product_id"`
	ProductCode string  `gorm:"size:100" json:"product_code"` // Model number (legacy)
	Model       string  `gorm:"size:255" json:"model"`        // Model number (explicit)
	Description string  `gorm:"type:varchar(2000)" json:"description"` // Equipment name - Model
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price_bhd"`

	// Extended fields for full costing data visibility (Fix #5)
	LongCode            string  `gorm:"type:varchar(500)" json:"long_code"`                  // Long supplier code (e.g. E+H instrumentation codes)
	Equipment           string  `gorm:"size:255" json:"equipment"`                           // Equipment/Product name
	Specification       string  `gorm:"type:varchar(2000)" json:"specification"`             // Short specification
	DetailedDescription string  `gorm:"type:varchar(5000)" json:"detailed_description"`      // Full instrumentation specs
	Currency            string  `gorm:"size:10" json:"currency"`                             // Source currency
	FOB                 float64 `json:"fob"`                                                 // FOB cost
	Freight             float64 `json:"freight"`                                             // Freight cost
	TotalCost           float64 `json:"total_cost"`                                          // Total cost (landed + handling + finance)
	MarginPercent       float64 `json:"margin_percent"`                                      // Margin % used
	TotalPrice          float64 `json:"total_price"`                                         // Line total (qty × unit price)

	// Full cost breakdown (for detailed costing persistence - Fix 2026-02-05)
	ExchangeRate    float64 `json:"exchange_rate"`     // Currency conversion rate
	FobBHD          float64 `json:"fob_bhd"`           // FOB cost in BHD
	FreightBHD      float64 `json:"freight_bhd"`       // Freight cost in BHD
	Insurance       float64 `json:"insurance"`         // Insurance charges
	CustomsPercent  float64 `json:"customs_percent"`   // Customs duty percentage
	CustomsBHD      float64 `json:"customs_bhd"`       // Customs duty in BHD
	HandlingPercent float64 `json:"handling_percent"`  // Handling charges percentage
	HandlingBHD     float64 `json:"handling_bhd"`      // Handling charges in BHD
	FinancePercent  float64 `json:"finance_percent"`   // Finance cost percentage
	FinanceBHD      float64 `json:"finance_bhd"`       // Finance cost in BHD
	OtherCosts      float64 `json:"other_costs"`       // Other miscellaneous costs
	UserPrice       float64 `json:"user_price"`        // User-overridden price (if manually set)
	UserPriceSet    bool    `json:"user_price_set"`    // Boolean flag for manual price override
}

func (OfferItem) TableName() string { return "offer_items" }

// Opportunity represents a sales opportunity with three-regime dynamics tracking
type Opportunity struct {
	Base
	FolderNumber string `gorm:"uniqueIndex;size:50" json:"folder_number"` // Physical folder tracking number
	OfferID      string `gorm:"index;size:36;constraint:OnDelete:SET NULL,OnUpdate:CASCADE;foreignKey:OfferID;references:ID" json:"offer_id"` // Link to Offer if quoted

	// Customer & Sales
	CustomerID   string `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`
	CustomerName string `gorm:"index;size:255" json:"customer_name"`
	Salesperson  string `gorm:"index;size:100" json:"salesperson"`

	// Dates
	OfferDate    time.Time  `gorm:"index" json:"offer_date"`
	ExpectedDate *time.Time `json:"expected_date"` // Expected close date
	ClosedDate   *time.Time `json:"closed_date"`   // Actual close date

	// Financial
	RevenueBHD float64 `json:"revenue_bhd"` // Expected revenue
	CostBHD    float64 `json:"cost_bhd"`    // Expected cost
	ProfitBHD  float64 `json:"profit_bhd"`  // Expected profit

	// Status Tracking
	SPOCStatus string `gorm:"index;size:50" json:"spoc_status"` // Single Point of Contact status
	WIPStatus  string `gorm:"index;size:50" json:"wip_status"`  // Work In Progress status
	Stage      string `gorm:"index;size:50;check:stage IN ('Lead','Qualified','Proposal','Negotiation','Won','Lost','Expired')" json:"stage"`       // Lead/Qualified/Proposal/Negotiation/Won/Lost

	// Three-Regime Dynamics (Asymmetrica Mathematical Framework)
	Regime     int     `gorm:"index" json:"regime"`     // 1=Exploration(0-30%), 2=Optimization(30-50%), 3=Stabilization(50%+)
	Confidence float64 `json:"confidence"`              // 0.0-1.0 confidence score
	R1         float64 `json:"r1"`                      // Regime 1 weight (Exploration)
	R2         float64 `json:"r2"`                      // Regime 2 weight (Optimization)
	R3         float64 `json:"r3"`                      // Regime 3 weight (Stabilization)

	// Competition & Product
	HasABBCompetition bool   `json:"has_abb_competition"`
	ProductType       string `gorm:"index;size:100" json:"product_type"` // Servomex, E+H_flow, etc.

	// Outcome
	WonReason  string `gorm:"type:varchar(2000)" json:"won_reason"`
	LostReason string `gorm:"type:varchar(2000)" json:"lost_reason"`
}

func (Opportunity) TableName() string { return "opportunities" }

// ARAgingBucket represents AR aging analysis with risk bucketing
type ARAgingBucket struct {
	Base
	CustomerID   string    `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`
	CustomerName string    `gorm:"index;size:255" json:"customer_name"`
	SnapshotDate time.Time `gorm:"index" json:"snapshot_date"` // Date of aging calculation

	// AR Aging Buckets (in BHD)
	Less15Days float64 `json:"less_15_days"`   // 0-15 days (current)
	Days16_30  float64 `json:"days_16_30"`     // 16-30 days
	Days31_60  float64 `json:"days_31_60"`     // 31-60 days
	Days61_90  float64 `json:"days_61_90"`     // 61-90 days
	Over90Days float64 `json:"over_90_days"`   // 90+ days (high risk)

	// Totals
	TotalOutstanding float64 `json:"total_outstanding"`
	TotalOverdue     float64 `json:"total_overdue"` // Sum of all overdue buckets

	// Risk Classification
	RiskTier   string  `gorm:"index;size:20;check:risk_tier IN ('Low','Medium','High','Critical')" json:"risk_tier"` // Low/Medium/High/Critical
	RiskScore  float64 `gorm:"check:risk_score >= 0 AND risk_score <= 1" json:"risk_score"`                     // 0.0-1.0 calculated risk
	OverdueDays int    `gorm:"check:overdue_days >= 0" json:"overdue_days"`                   // Days for oldest overdue invoice
}

func (ARAgingBucket) TableName() string { return "ar_aging" }

// =============================================================================
// MODULE: OPERATIONS & LOGISTICS
// =============================================================================

// Order represents a confirmed customer order with fulfillment tracking,
// linking to offers, invoices, deliveries, and payments.
type Order struct {
	Base
	OrderNumber      string `gorm:"uniqueIndex;size:50" json:"order_number"`
	CustomerPONumber string `gorm:"index;size:100" json:"customer_po_number"`

	// P1 Fix: Added composite index for customer order queries: WHERE customer_id = ? AND status = ?
	CustomerID   string `gorm:"index:idx_order_customer_status,priority:1;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`
	CustomerName string `gorm:"index;size:255" json:"customer_name"`

	// OrderDate indexed for date range queries: WHERE order_date BETWEEN ? AND ?
	// Used by: Order history, timeline filters, monthly reports, customer 360 view
	OrderDate     time.Time `gorm:"index;autoCreateTime:false" json:"order_date"`
	RequiredDate  time.Time `json:"required_date"`
	TotalValueBHD float64   `gorm:"type:decimal(15,3);not null;default:0;check:total_value_bhd >= 0" json:"total_value_bhd"` // P1 Fix: BHD precision
	GrandTotalBHD float64   `gorm:"type:decimal(15,3);not null;default:0;check:grand_total_bhd >= 0" json:"grand_total_bhd"` // P1 Fix: BHD precision
	// Status indexed for order pipeline: WHERE status NOT IN ('Delivered', 'Cancelled')
	// P1 Fix: Part of composite index idx_order_customer_status
	Status string `gorm:"index:idx_order_customer_status,priority:2;size:50;default:'Processing'" json:"status"` // Removed CHECK constraint - data has various statuses

	// P1 Fix: Audit fields
	UpdatedBy string `json:"updated_by"`

	PaymentTerms  string `gorm:"type:varchar(500)" json:"payment_terms"`
	DeliveryTerms string `gorm:"type:varchar(500)" json:"delivery_terms"`

	// Traceability - link to source Offer
	OfferID     string `gorm:"index;size:36" json:"offer_id"`
	OfferNumber string `gorm:"size:50" json:"offer_number"`
	RFQID       string `gorm:"index;size:36" json:"rfq_id"` // Link to RFQ/Opportunity

	// Contact & RFQ Details (from Offer - for invoice generation)
	CustomerReference string `gorm:"size:255" json:"customer_reference"` // RFQ/enquiry reference
	AttentionPerson   string `gorm:"size:255" json:"attention_person"`   // Contact name
	AttentionCompany  string `gorm:"size:255" json:"attention_company"`
	AttentionPhone    string `gorm:"size:50" json:"attention_phone"`
	AttentionAddress  string `gorm:"type:varchar(1000)" json:"attention_address"`
	DeliveryWeeks     string `gorm:"size:50" json:"delivery_weeks"`
	CountryOfOrigin   string `gorm:"size:100" json:"country_of_origin"`
	IssuedBy          string `gorm:"size:100" json:"issued_by"`
	ContactPhone      string `gorm:"size:50" json:"contact_phone"`
	DiscountPercent   float64 `json:"discount_percent"`

	// E2: Division field - 'PH Trading' or 'AHS Trading' (sister companies)
	Division string `gorm:"size:100;default:'PH Trading'" json:"division"`

	Items []OrderItem `gorm:"foreignKey:OrderID" json:"items,omitempty"`
}

func (Order) TableName() string { return "orders" }

// OrderItem represents a line item in an order with fulfillment tracking
// (quantities shipped/invoiced) and full costing data inherited from OfferItem.
type OrderItem struct {
	Base
	OrderID    string `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:OrderID;references:ID" json:"order_id"`
	LineNumber int    `json:"line_number"`

	ProductID   string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:ProductID;references:ID" json:"product_id"`
	ProductCode string  `gorm:"size:100" json:"product_code"`
	Description string  `gorm:"type:varchar(2000)" json:"description"`
	Quantity    float64 `json:"quantity"`
	UnitPrice   float64 `json:"unit_price_bhd"`

	// Fulfillment tracking
	QuantityShipped  float64 `json:"quantity_shipped"`
	QuantityInvoiced float64 `json:"quantity_invoiced"`

	// Extended costing fields (from OfferItem - costing sheet data)
	Equipment           string  `gorm:"size:255" json:"equipment"`             // Equipment/Product name
	Model               string  `gorm:"size:255" json:"model"`                 // Model number
	Specification       string  `gorm:"type:varchar(2000)" json:"specification"`        // Short specification
	DetailedDescription string  `gorm:"type:varchar(5000)" json:"detailed_description"` // Full instrumentation specs
	Currency            string  `gorm:"size:10" json:"currency"`               // Source currency (USD, EUR, etc.)
	FOB                 float64 `json:"fob"`                                   // FOB cost
	Freight             float64 `json:"freight"`                               // Freight cost
	TotalCost           float64 `json:"total_cost"`                            // Total landed cost
	MarginPercent       float64 `json:"margin_percent"`                        // Margin % applied
	TotalPrice          float64 `json:"total_price"`                           // Line total (qty × unit price)
}

func (OrderItem) TableName() string { return "order_items" }

// Shipment tracks courier deliveries for orders with tracking numbers and status.
type Shipment struct {
	Base
	OrderID        string     `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:OrderID;references:ID" json:"order_id"`
	OrderNumber    string     `gorm:"index;size:50" json:"order_number"`
	Status         string     `gorm:"index;size:50;check:status IN ('Pending','In Transit','Delivered','Failed','Cancelled')" json:"status"`
	ShipmentDate   time.Time  `json:"shipment_date"`
	DeliveredDate  *time.Time `json:"delivered_date,omitempty"`
	CourierName    string     `json:"courier_name"`
	TrackingNumber string     `json:"tracking_number"`
}

func (Shipment) TableName() string { return "shipments" }

// PostSaleNote tracks warranty claims, repairs, and expenses after order delivery
type PostSaleNote struct {
	Base
	OrderID     string `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:OrderID;references:ID" json:"order_id"`
	OrderNumber string `gorm:"index;size:50" json:"order_number"`

	NoteType    string    `gorm:"index;size:50;check:note_type IN ('repair','replace','reinstall','warranty','refund','calibration','other')" json:"note_type"` // repair, replace, reinstall, warranty, refund, other
	Description string    `gorm:"type:varchar(5000)" json:"description"`
	CostBHD     float64   `gorm:"check:cost_bhd >= 0" json:"cost_bhd"`
	NoteDate    time.Time `gorm:"index" json:"note_date"`

	// Resolution tracking
	ResolvedAt *time.Time `json:"resolved_at"`
	Resolution string     `gorm:"type:varchar(5000)" json:"resolution"`
}

func (PostSaleNote) TableName() string { return "post_sale_notes" }

// DeliveryNote tracks customer deliveries with partial delivery support
type DeliveryNote struct {
	Base
	// Links - P1 Fix: Composite index for order delivery lookups: WHERE order_id = ? AND status = ?
	OrderID    string `gorm:"index:idx_dn_order_status,priority:1;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:OrderID;references:ID" json:"order_id"`
	CustomerID string `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`

	// Document Info
	DNNumber     string    `gorm:"uniqueIndex;size:50" json:"dn_number"` // DN-2026-0001
	DeliveryDate time.Time `gorm:"index;autoCreateTime:false" json:"delivery_date"`

	// Delivery Details
	DeliveryAddress string `gorm:"type:varchar(1000)" json:"delivery_address"`
	ContactPerson   string `json:"contact_person"`
	ContactPhone    string `json:"contact_phone"`

	// Transport
	DriverName      string `json:"driver_name"`
	VehicleNumber   string `json:"vehicle_number"`
	TransportMethod string `json:"transport_method"` // Own Vehicle, Courier, Customer Pickup

	// Status Flow: Prepared → Dispatched → InTransit → Delivered → Signed
	// P1 Fix: Part of composite index idx_dn_order_status
	Status string `gorm:"index:idx_dn_order_status,priority:2;size:50;default:'Prepared';check:status IN ('Prepared','Dispatched','InTransit','Delivered','Signed','Cancelled')" json:"status"` // P1 Fix: default status

	// P1 Fix: Audit fields
	UpdatedBy string `json:"updated_by"`

	// Proof of Delivery
	SignedBy       string     `json:"signed_by"`
	SignedAt       *time.Time `json:"signed_at"`
	SignatureImage string     `json:"signature_image"` // Base64 or file path (future)

	// Partial Delivery Tracking
	IsPartialDelivery bool `json:"is_partial_delivery"`
	DeliverySequence  int  `json:"delivery_sequence"` // 1 of 3, 2 of 3, etc.
	TotalDeliveries   int  `json:"total_deliveries"`

	Items []DeliveryNoteItem `gorm:"foreignKey:DeliveryNoteID" json:"items,omitempty"`
}

func (DeliveryNote) TableName() string { return "delivery_notes" }

type DeliveryNoteItem struct {
	Base
	DeliveryNoteID    string  `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:DeliveryNoteID;references:ID" json:"delivery_note_id"`
	OrderItemID       string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:OrderItemID;references:ID" json:"order_item_id"`
	ProductID         string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:ProductID;references:ID" json:"product_id"`
	ProductCode       string  `gorm:"size:100" json:"product_code"`
	Description       string  `gorm:"type:varchar(2000)" json:"description"`
	QuantityOrdered   float64 `json:"quantity_ordered"`   // Original order qty
	QuantityDelivered float64 `json:"quantity_delivered"` // This delivery
	QuantityRemaining float64 `json:"quantity_remaining"` // Left to deliver
}

func (DeliveryNoteItem) TableName() string { return "delivery_note_items" }

// =============================================================================
// MODULE: COSTING ENGINE
// =============================================================================

// DBCostingSheet captures complete cost analysis for customer quotations with
// line items, Bahrain logistics costs, and margin calculations. Converts to Offers.
type DBCostingSheet struct {
	Base
	CostingNumber string `gorm:"uniqueIndex;size:50" json:"costing_number"`
	CustomerID    string `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`
	CustomerName  string `json:"customer_name"`

	// Dates
	CostingDate time.Time `json:"costing_date"`
	ValidUntil  time.Time `json:"valid_until"`

	// Line Item Totals
	SubtotalBHD    float64 `gorm:"not null;default:0;check:subtotal_bhd >= 0" json:"subtotal_bhd"`
	TotalMarginBHD float64 `gorm:"not null;default:0" json:"total_margin_bhd"`

	// Bahrain Logistics Costs
	ShippingCostBHD  float64 `gorm:"not null;default:0;check:shipping_cost_bhd >= 0" json:"shipping_cost_bhd"`
	CustomsDutyBHD   float64 `gorm:"not null;default:0;check:customs_duty_bhd >= 0" json:"customs_duty_bhd"`
	ClearanceCostBHD float64 `gorm:"not null;default:0;check:clearance_cost_bhd >= 0" json:"clearance_cost_bhd"`
	HandlingCostBHD  float64 `gorm:"not null;default:0;check:handling_cost_bhd >= 0" json:"handling_cost_bhd"`

	// Additional Costs Total
	AdditionalCostsBHD float64 `gorm:"not null;default:0;check:additional_costs_bhd >= 0" json:"additional_costs_bhd"`

	// Grand Total
	GrandTotalBHD float64 `gorm:"not null;default:0;check:grand_total_bhd >= 0" json:"grand_total_bhd"`

	// Status
	Status             string `gorm:"index;size:50;default:'Draft';check:status IN ('Draft','Approved','Converted','Rejected')" json:"status"` // Draft, Approved, Converted
	ConvertedToOfferID string `gorm:"size:36" json:"converted_to_offer_id"`

	// Relationships
	Items           []DBCostingItem           `gorm:"foreignKey:CostingSheetID" json:"items,omitempty"`
	AdditionalCosts []DBCostingAdditionalCost `gorm:"foreignKey:CostingSheetID" json:"additional_costs,omitempty"`
}

func (DBCostingSheet) TableName() string { return "costing_sheets" }

// DBCostingItem represents a line item in a costing sheet with unit costs,
// margins, and pricing calculations.
type DBCostingItem struct {
	Base
	CostingSheetID string `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:CostingSheetID;references:ID" json:"costing_sheet_id"`
	LineNumber     int    `json:"line_number"`

	ProductID   string `gorm:"size:36" json:"product_id"`
	ProductType string `gorm:"size:100" json:"product_type"`
	Description string `gorm:"type:varchar(2000)" json:"description"`

	Quantity      float64 `gorm:"not null;default:0;check:quantity >= 0" json:"quantity"`
	UnitCostBHD   float64 `gorm:"not null;default:0;check:unit_cost_bhd >= 0" json:"unit_cost_bhd"`
	MarginPercent float64 `gorm:"not null;default:0" json:"margin_percent"`
	UnitPriceBHD  float64 `gorm:"not null;default:0;check:unit_price_bhd >= 0" json:"unit_price_bhd"`
	LineTotalBHD  float64 `gorm:"not null;default:0;check:line_total_bhd >= 0" json:"line_total_bhd"`
}

func (DBCostingItem) TableName() string { return "costing_items" }

// DBCostingAdditionalCost represents ad-hoc costs in a costing sheet such as
// special handling, permits, or one-time charges.
type DBCostingAdditionalCost struct {
	Base
	CostingSheetID string  `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:CostingSheetID;references:ID" json:"costing_sheet_id"`
	Description    string  `gorm:"size:500" json:"description"`
	AmountBHD      float64 `gorm:"not null;default:0;check:amount_bhd >= 0" json:"amount_bhd"`
}

func (DBCostingAdditionalCost) TableName() string { return "costing_additional_costs" }

// =============================================================================
// MODULE: FINANCE & ACCOUNTING
// =============================================================================

// Invoice represents a customer invoice with AR tracking, payment status, VAT,
// and full costing visibility for margin analysis.
type Invoice struct {
	Base
	InvoiceNumber string    `gorm:"uniqueIndex;size:50" json:"invoice_number"`
	// InvoiceDate indexed for revenue reports: WHERE invoice_date BETWEEN ? AND ?
	// P1 Fix: Part of composite index idx_invoice_customer_date for customer invoice history
	InvoiceDate time.Time `gorm:"index:idx_invoice_customer_date,priority:2;autoCreateTime:false" json:"invoice_date"`

	// P1 Fix: Added composite index for customer invoice queries: WHERE customer_id = ? ORDER BY invoice_date
	// P2 Fix: Added covering index for AR aging queries: WHERE customer_id = ? AND status = ? ORDER BY due_date
	CustomerID   string `gorm:"index:idx_invoice_customer_date,priority:1;index:idx_invoice_ar_aging,priority:1;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`
	CustomerName string `gorm:"index;size:255" json:"customer_name"`

	// OrderID indexed for order-invoice relationship lookups
	// NOTE: NOT unique - multiple invoices can exist per order (partial, corrections, etc.)
	OrderID          string `gorm:"index;size:36" json:"order_id,omitempty"`
	CustomerPONumber string `gorm:"index;size:100" json:"customer_po_number"`

	GrandTotalBHD float64 `gorm:"type:decimal(15,3);not null;default:0;check:grand_total_bhd >= 0" json:"grand_total_bhd"` // P1 Fix: BHD precision
	// Status indexed for invoice filtering: WHERE status = 'Paid', status IN ('Sent', 'Overdue')
	// Used by: Dashboard KPIs, AR tracking, collection workflows
	// P2 Fix: Part of covering index idx_invoice_ar_aging
	Status         string    `gorm:"index:idx_invoice_ar_aging,priority:2;size:50;default:'Sent';check:status IN ('Draft','Sent','Paid','PartiallyPaid','Overdue','Cancelled','Void','Proforma')" json:"status"` // P1 Fix: added PartiallyPaid, Proforma
	OutstandingBHD float64   `gorm:"type:decimal(15,3);not null;default:0;check:outstanding_bhd >= 0" json:"outstanding_bhd"` // P1 Fix: BHD precision
	SubtotalBHD    float64   `gorm:"type:decimal(15,3);not null;default:0;check:subtotal_bhd >= 0" json:"subtotal_bhd"`       // P1 Fix: BHD precision
	// DueDate indexed for overdue tracking: WHERE due_date < NOW()
	// Used by: Collection alerts, AR aging, payment prediction
	// P2 Fix: Part of covering index idx_invoice_ar_aging
	DueDate time.Time `gorm:"index:idx_invoice_ar_aging,priority:3" json:"due_date"`

	// P1 Fix: Audit fields
	UpdatedBy string `json:"updated_by"`

	// Traceability Links (Operations Pipeline)
	RfqID              string `gorm:"index;size:36" json:"rfq_id"`
	QuoteID            string `gorm:"index;size:36" json:"quote_id"`
	OfferID            string `gorm:"index;size:36" json:"offer_id"`
	OfferNumber        string `gorm:"size:50" json:"offer_number"`
	DeliveryNoteID     string `gorm:"index;size:36" json:"delivery_note_id"` // Links invoice to delivery note
	DeliveryNoteNumber string `gorm:"size:50" json:"delivery_note_number"`   // DN number for display

	// Margin & Cost Analysis
	TotalSupplierCostBHD float64 `gorm:"not null;default:0;check:total_supplier_cost_bhd >= 0" json:"total_supplier_cost_bhd"`
	GrossMarginBHD       float64 `gorm:"not null;default:0" json:"gross_margin_bhd"`
	GrossMarginPercent   float64 `gorm:"not null;default:0" json:"gross_margin_percent"`

	// Contact & RFQ Details (from Order/Offer - costing sheet data for invoice)
	CustomerReference string  `gorm:"size:255" json:"customer_reference"` // RFQ/enquiry reference
	AttentionPerson   string  `gorm:"size:255" json:"attention_person"`   // Contact name
	AttentionCompany  string  `gorm:"size:255" json:"attention_company"`
	AttentionPhone    string  `gorm:"size:50" json:"attention_phone"`
	AttentionAddress  string  `gorm:"type:varchar(1000)" json:"attention_address"`
	DeliveryWeeks     string  `gorm:"size:50" json:"delivery_weeks"`
	CountryOfOrigin   string  `gorm:"size:100" json:"country_of_origin"`
	IssuedBy          string  `gorm:"size:100" json:"issued_by"`
	ContactPhone      string  `gorm:"size:50" json:"contact_phone"`
	DiscountPercent   float64 `json:"discount_percent"`
	PaymentTerms      string  `gorm:"type:varchar(500)" json:"payment_terms"`
	DeliveryTerms     string  `gorm:"type:varchar(500)" json:"delivery_terms"`

	// E2: Division field - 'PH Trading' or 'AHS Trading' (sister companies)
	Division string `gorm:"size:100;default:'PH Trading'" json:"division"`

	// Field Visibility Settings (JSON - user can toggle which fields appear on invoice PDF)
	// Example: {"show_fob": false, "show_freight": false, "show_margin": false, "show_cost": false, "show_contact": true}
	FieldVisibility string `gorm:"type:varchar(1000)" json:"field_visibility"`

	// Extended Invoice Header Fields (Issue #19 - matches client's Tally format)
	// These fields enable the full 11-field invoice header as per client reference PDF
	DeliveryNoteRef    string     `gorm:"size:100" json:"delivery_note_ref"`                                 // e.g., "EH/253/25"
	ModeOfPayment      string     `gorm:"size:100" json:"mode_of_payment"`                                   // e.g., "30 Days"
	SuppliersRef       string     `gorm:"size:100" json:"suppliers_ref"`                                     // Supplier's reference number
	OtherReferences    string     `gorm:"size:200" json:"other_references"`                                  // e.g., "E+H"
	BuyersOrderNumber  string     `gorm:"size:100" json:"buyers_order_number"`                               // e.g., "LPS -11347"
	BuyersOrderDate    *time.Time `json:"buyers_order_date"`                                                 // Buyer's PO date
	DespatchDocumentNo string     `gorm:"size:100" json:"despatch_document_no"`                              // Despatch/delivery document number
	DeliveryNoteDate   *time.Time `json:"delivery_note_date"`                                                // Delivery note date
	DespatchedThrough  string     `gorm:"size:100;default:'Direct'" json:"despatched_through"`               // Mode of despatch
	Destination        string     `gorm:"size:100;default:'Bahrain'" json:"destination"`                     // Delivery destination
	PlaceOfSupply      string     `gorm:"size:100;default:'Kingdom of Bahrain'" json:"place_of_supply"`      // VAT place of supply
	TermsOfDelivery    string     `gorm:"size:200;default:'Direct Bank Transfer'" json:"terms_of_delivery"`  // Payment method

	// VAT & Accounting (Tally Killer feature)
	VATBHD         float64 `gorm:"column:vatbhd;type:decimal(15,3);not null;default:0;check:vatbhd >= 0" json:"vat_bhd"` // P1 Fix: BHD precision
	VATPercent     float64 `gorm:"not null;default:0;check:vat_percent >= 0" json:"vat_percent"`                         // VAT rate applied (default 10)
	JournalEntryID string  `gorm:"index;size:36" json:"journal_entry_id"`                                                 // Link to auto-generated GL entry

	// E-Invoicing (Phase 23)
	InvoiceHash string `gorm:"size:64" json:"invoice_hash"` // SHA-256 hash for integrity verification

	Items []DBInvoiceItem `gorm:"foreignKey:InvoiceID" json:"items"`
}

func (Invoice) TableName() string { return "invoices" }

// DBInvoiceItem represents a line item in an invoice with full costing data
// inherited from OrderItem including FOB, freight, margins, and specifications.
type DBInvoiceItem struct {
	Base
	// NOTE: Removed constraint tag - SQLite can't modify FK constraints on existing tables
	InvoiceID   string  `gorm:"index;size:36" json:"invoice_id"`
	LineNumber  int     `json:"line_number"`
	Description string  `gorm:"type:varchar(2000)" json:"description"`
	Quantity    float64 `gorm:"not null;default:0;check:quantity >= 0" json:"quantity"`
	Rate        float64 `gorm:"not null;default:0;check:rate >= 0" json:"rate"`
	TotalBHD    float64 `gorm:"not null;default:0;check:total_bhd >= 0" json:"total_bhd"`

	// Full costing data (from OrderItem/OfferItem - the costing sheet becomes the invoice)
	ProductID           string  `gorm:"index;size:36" json:"product_id"`
	ProductCode         string  `gorm:"size:100" json:"product_code"`        // Model number
	Equipment           string  `gorm:"size:255" json:"equipment"`           // Equipment/Product name
	Model               string  `gorm:"size:255" json:"model"`               // Model number
	Specification       string  `gorm:"type:varchar(2000)" json:"specification"`      // Short specification
	DetailedDescription string  `gorm:"type:varchar(5000)" json:"detailed_description"` // Full instrumentation specs
	Currency            string  `gorm:"size:10" json:"currency"`             // Source currency (USD, EUR, etc.)
	FOB                 float64 `gorm:"not null;default:0;check:fob >= 0" json:"fob"`                                 // FOB cost
	Freight             float64 `gorm:"not null;default:0;check:freight >= 0" json:"freight"`                             // Freight cost
	TotalCost           float64 `gorm:"not null;default:0;check:total_cost >= 0" json:"total_cost"`                          // Total landed cost
	MarginPercent       float64 `gorm:"not null;default:0" json:"margin_percent"`                      // Margin % applied
	TotalPrice          float64 `gorm:"not null;default:0;check:total_price >= 0" json:"total_price"`                         // Line total (qty × unit price)
}

func (DBInvoiceItem) TableName() string { return "invoice_items" }

// InvoiceSequence provides race-condition-safe sequential invoice numbering.
// Uses row-level locking (SELECT FOR UPDATE) to ensure no duplicate numbers
// even under concurrent invoice creation.
type InvoiceSequence struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Prefix     string    `gorm:"size:10;uniqueIndex:idx_invoice_sequence_prefix_year" json:"prefix"`
	Year       int       `gorm:"uniqueIndex:idx_invoice_sequence_prefix_year" json:"year"`
	LastNumber int       `gorm:"not null;default:0" json:"last_number"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (InvoiceSequence) TableName() string { return "invoice_sequences" }

// =============================================================================
// MODULE: CREDIT NOTES (Phase 23 - E-Invoicing)
// =============================================================================

// CreditNote represents a credit note issued against a customer invoice
// for returns, pricing corrections, or other adjustments.
type CreditNote struct {
	Base
	CreditNoteNumber string     `gorm:"uniqueIndex;size:50" json:"credit_note_number"`
	CreditNoteDate   time.Time  `gorm:"index" json:"credit_note_date"`
	InvoiceID        string     `gorm:"index;size:36" json:"invoice_id"`
	InvoiceNumber    string     `gorm:"size:50" json:"invoice_number"`
	CustomerID       string     `gorm:"index;size:36" json:"customer_id"`
	CustomerName     string     `gorm:"size:255" json:"customer_name"`
	Reason           string     `gorm:"type:varchar(2000)" json:"reason"`
	SubtotalBHD      float64    `gorm:"type:decimal(15,3)" json:"subtotal_bhd"`
	VATBHD           float64    `gorm:"type:decimal(15,3)" json:"vat_bhd"`
	VATPercent       float64    `json:"vat_percent"`
	GrandTotalBHD    float64    `gorm:"type:decimal(15,3)" json:"grand_total_bhd"`
	Status           string     `gorm:"index;size:50;default:'Draft'" json:"status"` // Draft, Issued, Applied
	AppliedAt        *time.Time `json:"applied_at"`
	CreditNoteHash   string     `gorm:"size:64" json:"credit_note_hash"`
	Items            []CreditNoteItem `gorm:"foreignKey:CreditNoteID" json:"items"`
}

func (CreditNote) TableName() string { return "credit_notes" }

// CreditNoteItem represents a line item in a credit note.
type CreditNoteItem struct {
	Base
	CreditNoteID string  `gorm:"index;size:36" json:"credit_note_id"`
	LineNumber   int     `json:"line_number"`
	Description  string  `gorm:"type:varchar(2000)" json:"description"`
	Quantity     float64 `json:"quantity"`
	Rate         float64 `json:"rate"`
	TotalBHD     float64 `gorm:"type:decimal(15,3)" json:"total_bhd"`
}

func (CreditNoteItem) TableName() string { return "credit_note_items" }

// =============================================================================
// MODULE: SERIAL NUMBER TRACKING (Phase 23)
// =============================================================================

// SerialNumber tracks individual serialized items through the full lifecycle:
// PO receipt (GRN) → inventory → dispatch (DN) → delivery → customer invoicing.
type SerialNumber struct {
	Base
	ProductID   string `gorm:"index;size:36" json:"product_id"`
	ProductCode string `gorm:"index;size:100" json:"product_code"`
	SerialNo    string `gorm:"uniqueIndex;size:255" json:"serial_no"`
	LotNumber   string `gorm:"index;size:100" json:"lot_number"`
	Status      string `gorm:"index;size:50;default:'Available'" json:"status"` // Available, Reserved, Shipped, Delivered

	// Traceability chain
	POID          string `gorm:"index;size:36" json:"po_id"`
	PONumber      string `gorm:"size:50" json:"po_number"`
	GRNItemID     string `gorm:"index;size:36" json:"grn_item_id"`
	GRNNumber     string `gorm:"size:50" json:"grn_number"`
	DNItemID      string `gorm:"index;size:36" json:"dn_item_id"`
	DNNumber      string `gorm:"size:50" json:"dn_number"`
	InvoiceID     string `gorm:"index;size:36" json:"invoice_id"`
	InvoiceNumber string `gorm:"size:50" json:"invoice_number"`
	CustomerID    string `gorm:"index;size:36" json:"customer_id"`
	CustomerName  string `gorm:"size:255" json:"customer_name"`

	// Dates
	ReceivedDate *time.Time `json:"received_date"`
	ShippedDate  *time.Time `json:"shipped_date"`

	// Warranty
	WarrantyStartDate *time.Time `json:"warranty_start_date"`
	WarrantyEndDate   *time.Time `json:"warranty_end_date"`
	WarrantyMonths    int        `json:"warranty_months"`

	// Calibration
	CalibrationDate     *time.Time `json:"calibration_date"`
	CalibrationDueDate  *time.Time `json:"calibration_due_date"`
	CalibrationCertPath string     `gorm:"size:500" json:"calibration_cert_path"`

	Notes string `gorm:"type:varchar(2000)" json:"notes"`
}

func (SerialNumber) TableName() string { return "serial_numbers" }

// ChartOfAccount represents a GL account in the accounting system with hierarchy,
// VAT tracking, and balance management for Tally Killer integration.
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
	ParentAccountID string `gorm:"index;size:36;constraint:OnDelete:RESTRICT;" json:"parent_account_id"`
	AccountGroup    string `gorm:"index;size:10" json:"account_group"` // "BS" or "PL"
}

func (ChartOfAccount) TableName() string { return "chart_of_accounts" }

// JournalEntry represents a GL journal entry with auto-posting from invoices,
// payments, and reversals. Supports Tally Killer accounting automation.
type JournalEntry struct {
	Base
	EntryNumber string    `gorm:"uniqueIndex;size:50" json:"entry_number"`
	EntryDate   time.Time `gorm:"index" json:"entry_date"`
	Description string    `json:"description"`

	DebitTotal   float64    `gorm:"type:decimal(15,3)" json:"debit_total"`  // P1 Fix: BHD precision
	CreditTotal  float64    `gorm:"type:decimal(15,3)" json:"credit_total"` // P1 Fix: BHD precision
	IsPosted     bool       `gorm:"index" json:"is_posted"`
	PostedAt     *time.Time `json:"posted_at"`
	PostedBy     string     `json:"posted_by"`
	FiscalYear   int        `gorm:"index" json:"fiscal_year"`
	FiscalPeriod int        `json:"fiscal_period"`

	// Auto-posting source tracking (Tally Killer feature)
	SourceType      string `gorm:"index;size:50" json:"source_type"`      // invoice, payment, supplier_invoice, supplier_payment, manual
	SourceID        string `gorm:"index;size:36" json:"source_id"`        // ID of source document
	IsAutoGenerated bool   `gorm:"index;default:false" json:"is_auto_generated"`
	ReversedByID    string `gorm:"index;size:36" json:"reversed_by_id"`   // If this entry was reversed, link to reversal
	ReversesID      string `gorm:"index;size:36" json:"reverses_id"`      // If this is a reversal, link to original

	// P1 Fix: Audit field (DeletedAt already in Base for soft delete)
	UpdatedBy string `json:"updated_by"`

	Lines []JournalLine `gorm:"foreignKey:EntryID" json:"lines,omitempty"`
}

func (JournalEntry) TableName() string { return "journal_entries" }

// JournalLine represents a debit or credit line in a journal entry with
// account reference and description.
type JournalLine struct {
	Base
	// P0-4: Changed CASCADE to RESTRICT to preserve audit trail
	EntryID     string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT;" json:"entry_id"`
	AccountID   string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT;" json:"account_id"`
	AccountName string  `json:"account_name"`
	Debit       float64 `gorm:"type:decimal(15,3);not null;default:0;check:debit >= 0" json:"debit"`   // P1 Fix: BHD precision
	Credit      float64 `gorm:"type:decimal(15,3);not null;default:0;check:credit >= 0" json:"credit"` // P1 Fix: BHD precision
	Description string  `json:"description"`

	// P1 Fix: Audit field (DeletedAt already in Base for soft delete)
	UpdatedBy string `json:"updated_by"`
}

func (JournalLine) TableName() string { return "journal_lines" }

// VATReturn represents a VAT return filing for a fiscal period with calculated
// net VAT payable/receivable.
type VATReturn struct {
	Base
	ReturnNumber string    `gorm:"uniqueIndex;size:50" json:"return_number"`
	PeriodStart  time.Time `json:"period_start"`
	PeriodEnd    time.Time `json:"period_end"`
	FiscalYear   int       `json:"fiscal_year"`
	Quarter      int       `json:"quarter"`
	NetVAT       float64   `json:"net_vat"`
	Status       string    `json:"status"`
}

func (VATReturn) TableName() string { return "vat_returns" }

// Payment represents a customer payment against an invoice with days-to-payment
// tracking for payment prediction analytics and GL integration.
type Payment struct {
	Base
	// P2 Fix: Added covering index for payment history queries: WHERE invoice_id = ? ORDER BY payment_date
	// P2.4 Fix: Changed CASCADE to RESTRICT - payments are financial records, must never auto-delete
	InvoiceID     string    `gorm:"index:idx_payment_invoice_date,priority:1;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:InvoiceID;references:ID" json:"invoice_id"`
	InvoiceNumber string    `gorm:"index;size:50" json:"invoice_number"`
	AmountBHD     float64   `gorm:"type:decimal(15,3);not null;default:0;check:amount_bhd >= 0" json:"amount_bhd"` // P1 Fix: BHD precision
	// P2 Fix: Part of covering index idx_payment_invoice_date
	PaymentDate   time.Time `gorm:"index:idx_payment_invoice_date,priority:2;autoCreateTime:false" json:"payment_date"`
	PaymentMethod string    `gorm:"size:50;check:payment_method IN ('Cash','Cheque','Bank Transfer','Credit Card','LC','PDC','Other')" json:"payment_method"`
	// DaysToPayment indexed for payment analytics: AVG(days_to_payment), payment speed reports
	// Used by: Payment prediction accuracy, customer payment behavior analysis, survival curves
	DaysToPayment int `gorm:"index;check:days_to_payment >= 0" json:"days_to_payment"`

	// BE-8 FIX: Idempotency key to prevent duplicate payments from network retries
	// Format: sha256(invoice_id + amount + date + ref) - unique per payment attempt
	IdempotencyKey string `gorm:"uniqueIndex;size:64" json:"idempotency_key"`

	// Accounting Integration (Tally Killer feature)
	JournalEntryID string `gorm:"index;size:36" json:"journal_entry_id"` // Link to auto-generated GL entry
	BankAccountID  string `gorm:"index;size:36" json:"bank_account_id"`  // Bank account for reconciliation
	Reference      string `gorm:"size:100" json:"reference"`             // Payment reference (cheque no, transfer ref)

	// P1 Fix: Audit fields
	UpdatedBy string `json:"updated_by"`
}

func (Payment) TableName() string { return "payments" }

// =============================================================================
// MODULE: INVENTORY & WAREHOUSING
// =============================================================================

// InventoryItem represents stock levels for a product at a warehouse with
// reorder points, stock status, and valuation tracking.
type InventoryItem struct {
	Base
	ProductID         string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT;" json:"product_id"`
	ProductCode       string  `gorm:"index;size:100" json:"product_code"`
	WarehouseID       string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT;" json:"warehouse_id"`
	QuantityOnHand    float64 `gorm:"type:decimal(15,3);default:0" json:"quantity_on_hand"`
	QuantityReserved  float64 `gorm:"type:decimal(15,3);default:0" json:"quantity_reserved"`
	QuantityAvailable float64 `gorm:"type:decimal(15,3);default:0" json:"quantity_available"`
	UnitCost          float64 `gorm:"type:decimal(15,2)" json:"unit_cost"`
	StockStatus       string  `gorm:"index;size:50" json:"stock_status"`
	IsActive          bool    `gorm:"index" json:"is_active"`

	ReorderPoint     float64    `json:"reorder_point"`
	MinimumStock     float64    `json:"minimum_stock"`
	MaximumStock     float64    `json:"maximum_stock"`
	TotalValue       float64    `json:"total_value"`
	LastPurchaseCost float64    `json:"last_purchase_cost"`
	LastMovementAt   *time.Time `json:"last_movement_at"`
}

func (InventoryItem) TableName() string { return "inventory_items" }

// StockMovement represents an inventory transaction with direction (in/out),
// quantity, and balance tracking for audit trails.
type StockMovement struct {
	Base
	InventoryItemID string    `gorm:"index;size:36;constraint:OnDelete:CASCADE;" json:"inventory_item_id"`
	MovementType    string    `json:"movement_type"`
	MovementNumber  string    `json:"movement_number"`
	Quantity        float64   `json:"quantity"`
	Direction       string    `json:"direction"`
	BalanceBefore   float64   `json:"balance_before"`
	BalanceAfter    float64   `json:"balance_after"`
	MovementDate    time.Time `json:"movement_date"`

	UnitCost   float64 `json:"unit_cost"`
	TotalValue float64 `json:"total_value"`
	// CreatedBy is inherited from Base - no need to duplicate
}

func (StockMovement) TableName() string { return "stock_movements" }

// StockAdjustment represents a variance correction between system and physical
// counts with approval workflow and value impact tracking.
type StockAdjustment struct {
	Base
	InventoryItemID string    `gorm:"index;size:36;constraint:OnDelete:CASCADE;" json:"inventory_item_id"`
	AdjustmentDate  time.Time `json:"adjustment_date"`
	AdjustmentType  string    `json:"adjustment_type"`
	Reason          string    `json:"reason"`
	Variance        float64   `json:"variance"`

	SystemQuantity   float64    `json:"system_quantity"`
	PhysicalQuantity float64    `json:"physical_quantity"`
	UnitCost         float64    `json:"unit_cost"`
	ValueImpact      float64    `json:"value_impact"`
	Notes            string     `json:"notes"`
	Status           string     `json:"status"`
	AdjustmentNumber string     `gorm:"uniqueIndex;size:50" json:"adjustment_number"`
	ApprovedBy       string     `json:"approved_by"`
	ApprovedAt       *time.Time `json:"approved_at"`
}

func (StockAdjustment) TableName() string { return "stock_adjustments" }

// Warehouse represents a physical storage location for inventory management.
type Warehouse struct {
	Base
	Code     string `gorm:"uniqueIndex;size:50" json:"code"`
	Name     string `json:"name"`
	IsActive bool   `gorm:"default:true" json:"is_active"`
}

func (Warehouse) TableName() string { return "warehouses" }

// =============================================================================
// MODULE: INTELLIGENCE & ANALYTICS
// =============================================================================

// PredictionRecord stores payment prediction results for customers with
// confidence scores and three-regime dynamics (R1, R2, R3).
type PredictionRecord struct {
	Base
	// Composite index for payment prediction lookups: WHERE customer_id = ? AND grade = ?
	// Used by: Payment intelligence engine, predictive Butler, survival analysis
	// Priority 1 = CustomerID (high cardinality), Priority 2 = Grade (low cardinality)
	CustomerID    string  `gorm:"index:idx_prediction_customer_grade,priority:1;size:36" json:"customer_id"`
	CustomerName  string  `json:"customer_name"`
	Grade         string  `gorm:"index:idx_prediction_customer_grade,priority:2;size:10" json:"grade"`
	PredictedDays int     `json:"predicted_days"`
	Confidence    float64 `json:"confidence"`
	R1            float64 `json:"r1"`
	R2            float64 `json:"r2"`
	R3            float64 `json:"r3"`
}

func (PredictionRecord) TableName() string { return "prediction_records" }

// Setting represents application configuration with categories and optional
// encryption for sensitive values.
type Setting struct {
	Base
	Key         string `gorm:"uniqueIndex;size:255" json:"key"`
	Value       string `gorm:"type:varchar(5000)" json:"value"`
	Category    string `gorm:"index;size:100" json:"category"`
	Description string `gorm:"type:varchar(1000)" json:"description"`
	IsEncrypted bool   `gorm:"default:false" json:"is_encrypted"`
}

func (Setting) TableName() string { return "settings" }

// CurrencyExchangeRate stores exchange rates to BHD (base currency)
type CurrencyExchangeRate struct {
	ID            string         `gorm:"primaryKey;size:36" json:"id"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	CurrencyCode  string     `gorm:"type:varchar(3);not null;index" json:"currency_code"` // USD, EUR, CHF, SAR, AED
	Rate          float64    `gorm:"type:real;not null" json:"rate"`                      // e.g., 0.376 for 1 USD = 0.376 BHD
	EffectiveFrom time.Time  `gorm:"not null;index" json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to"`   // NULL = currently active
	SetBy         string     `gorm:"size:100" json:"set_by"`
	Notes         string     `gorm:"size:500" json:"notes"`
}

func (CurrencyExchangeRate) TableName() string { return "currency_exchange_rates" }

// UserSession represents an active user session with access/refresh tokens
// and expiry tracking for authentication management.
type UserSession struct {
	Base
	UserID             string     `gorm:"index;size:36;constraint:OnDelete:CASCADE;" json:"user_id"`
	Token              string     `gorm:"uniqueIndex;size:255" json:"token"`
	RefreshToken       string     `gorm:"uniqueIndex;size:255" json:"refresh_token"`
	AccessTokenExpiry  time.Time  `gorm:"index" json:"access_token_expiry"`
	RefreshTokenExpiry time.Time  `gorm:"index" json:"refresh_token_expiry"`
	LastActivityAt     time.Time  `gorm:"index" json:"last_activity_at"`
	IsActive           bool       `gorm:"index;default:true" json:"is_active"`
	InvalidatedAt      *time.Time `json:"invalidated_at,omitempty"`
	InvalidatedReason  string     `gorm:"size:255" json:"invalidated_reason,omitempty"`
}

func (UserSession) TableName() string { return "user_sessions" }

type WinProbabilityPrediction struct {
	Base
	OfferID              string  `gorm:"index;size:36" json:"offer_id"`
	PredictedProbability float64 `json:"predicted_probability"`
}

func (WinProbabilityPrediction) TableName() string { return "win_probability_predictions" }

type DiscountRecommendationRecord struct {
	Base
	OfferID             string  `gorm:"index;size:36" json:"offer_id"`
	RecommendedDiscount float64 `json:"recommended_discount"`
}

func (DiscountRecommendationRecord) TableName() string { return "discount_recommendations" }

type CustomerSnapshot struct {
	Base
	CustomerID string  `gorm:"index;size:36" json:"customer_id"`
	OrderValue float64 `json:"order_value"`
}

func (CustomerSnapshot) TableName() string { return "customer_snapshots" }

type ActualOutcome struct {
	Base
	CustomerID string `gorm:"index;size:36" json:"customer_id"`
	WasPaid    bool   `json:"was_paid"`
}

func (ActualOutcome) TableName() string { return "actual_outcomes" }

type CostingHistory struct {
	Base
	ProductID string  `gorm:"index;size:36" json:"product_id"`
	CostBHD   float64 `json:"cost_bhd"`
}

func (CostingHistory) TableName() string { return "costing_history" }

type GradeChange struct {
	Base
	// Composite index for grade change history: WHERE customer_id = ? AND new_grade = ?
	// Used by: Customer grade history, survival intelligence, grade trend analysis
	// High cardinality on CustomerID, low cardinality on NewGrade (A/B/C/D)
	CustomerID string `gorm:"index:idx_grade_change_customer_grade,priority:1;size:36" json:"customer_id"`
	NewGrade   string `gorm:"index:idx_grade_change_customer_grade,priority:2;size:10" json:"new_grade"`
}

func (GradeChange) TableName() string { return "grade_changes" }

type PaymentPredictionAccuracy struct {
	Base
	CustomerID  string `gorm:"index;size:36" json:"customer_id"`
	WasAccurate bool   `json:"was_accurate"`
}

func (PaymentPredictionAccuracy) TableName() string { return "payment_prediction_accuracy" }

// =============================================================================
// MODULE: ADMINISTRATION & LOGGING
// =============================================================================

// Role represents a user role with permission sets for RBAC authorization.
type Role struct {
	Base
	Name        string `gorm:"uniqueIndex;size:50" json:"name"`
	DisplayName string `gorm:"size:255" json:"display_name"`
	Description string `gorm:"type:varchar(1000)" json:"description"`
	Permissions string `gorm:"type:text" json:"permissions"` // FIX MEDIUM-2: Changed from varchar(5000) to text (no limit)
	IsActive    bool   `gorm:"index;default:true" json:"is_active"`
	IsSystem    bool   `gorm:"default:false" json:"is_system"`
}

func (Role) TableName() string { return "roles" }

// User represents an application user with authentication credentials,
// role assignment, and last login tracking.
type User struct {
	Base
	Username     string `gorm:"uniqueIndex;size:100" json:"username"`
	Email        string `gorm:"uniqueIndex;size:255" json:"email"`
	PasswordHash string `gorm:"size:255" json:"-"`
	RoleID       string `gorm:"index;size:36" json:"role_id"` // RBAC DEPRECATED - no FK constraint

	FullName    string `json:"full_name"`
	DisplayName string `json:"display_name"`
	Department  string `json:"department"`
	JobTitle    string `json:"job_title"`

	IsActive           bool       `gorm:"index;default:true" json:"is_active"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	PasswordChangedAt  *time.Time `json:"password_changed_at"`
	MustChangePassword bool       `gorm:"default:false" json:"must_change_password"`

	// Virtual / Relationships - RBAC DEPRECATED
	RoleName string `gorm:"-" json:"role_name,omitempty"`
	Role     Role   `gorm:"-" json:"role"` // Not a DB relationship anymore
}

func (User) TableName() string { return "users" }

// Device represents a registered installation of the application
type Device struct {
	Base
	MachineID     string     `gorm:"uniqueIndex;size:255" json:"machine_id"`                                                                   // SHA-256 hash of hardware identifiers
	DeviceName    string     `gorm:"size:255" json:"device_name"`                                                                              // Hostname
	OSInfo        string     `gorm:"size:255" json:"os_info"`                                                                                  // "Windows 11 Pro"
	FirstSeenAt   time.Time  `gorm:"autoCreateTime;index:idx_status_first_seen" json:"first_seen_at"`                                          // FIX CRITICAL-3: Added composite index
	LastSeenAt    *time.Time `json:"last_seen_at"`
	Status        string     `gorm:"size:20;default:pending;index:idx_status_first_seen;check:status IN ('first_setup','pending','approved','blocked','revoked')" json:"status"` // FIX: Added 'first_setup' for device registration flow
	ApprovedBy    string     `gorm:"size:36" json:"approved_by"` // RBAC DEPRECATED - no FK constraint
	ApprovedAt    *time.Time `json:"approved_at"`
	IsAdminDevice bool       `gorm:"default:false" json:"is_admin_device"` // First device = admin device
	Notes         string     `gorm:"type:varchar(2000)" json:"notes"`

	// Virtual fields
	ApproverName string `gorm:"-" json:"approver_name,omitempty"` // FIX LOW-1: Added omitempty
}

func (Device) TableName() string { return "devices" }

// DeviceUser links users to devices (a device can have multiple users)
type DeviceUser struct {
	Base
	DeviceID  string `gorm:"uniqueIndex:idx_device_user;index;size:36" json:"device_id"` // RBAC DEPRECATED - no FK constraint
	UserID    string `gorm:"uniqueIndex:idx_device_user;index;size:36" json:"user_id"`   // RBAC DEPRECATED - no FK constraint
	IsPrimary bool   `gorm:"default:false" json:"is_primary"`

	// Virtual fields - not DB relationships
	User   User   `gorm:"-" json:"user"`
	Device Device `gorm:"-" json:"device"`
}

func (DeviceUser) TableName() string { return "device_users" }

// Alert represents a system alert with severity levels and acknowledgment tracking.
type Alert struct {
	Base
	AlertType      string `gorm:"index;size:100" json:"alert_type"`
	Severity       string `gorm:"index;size:50;check:severity IN ('low','medium','high','critical')" json:"severity"`
	Title          string `gorm:"size:255" json:"title"`
	Message        string `gorm:"type:varchar(2000)" json:"message"`
	IsActive       bool   `gorm:"index;default:true" json:"is_active"`
	IsAcknowledged bool   `gorm:"index;default:false" json:"is_acknowledged"`
}

func (Alert) TableName() string { return "alerts" }

// FollowUpTask represents a scheduled task for customer follow-ups with
// priority, due date, and status tracking.
type FollowUpTask struct {
	Base
	CustomerID  string    `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:CustomerID;references:ID" json:"customer_id"`
	Title       string    `gorm:"size:255" json:"title"`
	Description string    `gorm:"type:varchar(2000)" json:"description"`
	// DueDate indexed for task queue queries: WHERE due_date < NOW(), ORDER BY due_date
	// Used by: Overdue task alerts, calendar view, predictive Butler task scheduling
	DueDate     time.Time `gorm:"index;autoCreateTime:false" json:"due_date"`
	Status      string    `gorm:"index;size:50;check:status IN ('pending','in_progress','completed','cancelled','overdue')" json:"status"`
	Priority    string    `gorm:"index;size:50;check:priority IN ('low','medium','high','urgent')" json:"priority"`
	Type        string    `gorm:"index;size:50" json:"type"`
	Amount      float64   `gorm:"check:amount >= 0" json:"amount"`
	Contact     string     `gorm:"size:255" json:"contact"`
	Notes       string     `gorm:"type:varchar(2000)" json:"notes"`
	CompletedAt *time.Time `json:"completed_at"`
}

func (FollowUpTask) TableName() string { return "followup_tasks" }

// AuditLog records user actions on resources for compliance and security tracking.
type AuditLog struct {
	Base
	UserID   string `gorm:"index;size:36;constraint:OnDelete:SET NULL;" json:"user_id"`
	Action   string `gorm:"index;size:50" json:"action"`
	Resource string `gorm:"index;size:100" json:"resource"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// FileWatchEvent records file system change events for document monitoring.
type FileWatchEvent struct {
	Base
	FilePath  string `gorm:"index;size:1000" json:"file_path"`
	EventType string `json:"event_type"`
}

func (FileWatchEvent) TableName() string { return "file_watch_events" }

// SyncStatus tracks synchronization state for files with last sync timestamps.
type SyncStatus struct {
	Base
	FilePath string `gorm:"uniqueIndex;size:1000" json:"file_path"`
	Status   string `gorm:"index;size:50" json:"status"`

	LastSyncTime time.Time `json:"last_sync_time"`
}

func (SyncStatus) TableName() string { return "sync_status" }

// Job represents an async background job with progress tracking, retry logic,
// and status management for long-running operations.
type Job struct {
	Base
	Type        string     `gorm:"index;size:50" json:"type"`
	Status      string     `gorm:"index;size:50;check:status IN ('pending','running','completed','failed','cancelled')" json:"status"`
	Input       string     `gorm:"type:varchar(5000)" json:"input"`
	Output      string     `gorm:"type:varchar(5000)" json:"output"`
	Error       string     `gorm:"type:varchar(2000)" json:"error,omitempty"`
	Progress    int        `gorm:"check:progress >= 0 AND progress <= 100" json:"progress"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Retry logic
	Attempts    int `gorm:"default:0;check:attempts >= 0" json:"attempts"`
	MaxAttempts int `gorm:"default:3;check:max_attempts >= 0" json:"max_attempts"`
}

func (Job) TableName() string { return "jobs" }

// =============================================================================
// MODULE: OPERATIONS PIPELINE - PROCUREMENT
// =============================================================================

// PurchaseOrder represents a purchase order to a supplier with multi-currency
// support, approval workflow, and partial receiving tracking.
type PurchaseOrder struct {
	Base
	// Links (One Order → Many POs)
	OrderID    string `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:OrderID;references:ID" json:"order_id"`
	RfqID      string `gorm:"index;size:36" json:"rfq_id"`
	// P1 Fix: Composite index for supplier PO queries: WHERE supplier_id = ? ORDER BY po_date
	SupplierID   string `gorm:"index:idx_po_supplier_date,priority:1;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:SupplierID;references:ID" json:"supplier_id"`
	SupplierName string `gorm:"index;size:255" json:"supplier_name"` // Denormalized for display

	// Document Info
	PONumber         string    `gorm:"uniqueIndex;size:50" json:"po_number"`
	// P1 Fix: Part of composite index idx_po_supplier_date
	PODate           time.Time `gorm:"index:idx_po_supplier_date,priority:2;autoCreateTime:false" json:"po_date"`
	ExpectedDelivery time.Time `json:"expected_delivery"`

	// Multi-Currency Support
	Currency        string  `gorm:"size:3" json:"currency"`
	ExchangeRate    float64 `gorm:"not null;default:1;check:exchange_rate > 0" json:"exchange_rate"`
	SubtotalForeign float64 `gorm:"not null;default:0;check:subtotal_foreign >= 0" json:"subtotal_foreign"`
	SubtotalBHD     float64 `gorm:"type:decimal(15,3);not null;default:0;check:subtotal_bhd >= 0" json:"subtotal_bhd"` // P1 Fix: BHD precision
	VATAmount       float64 `gorm:"type:decimal(15,3);not null;default:0;check:vat_amount >= 0" json:"vat_amount"`     // P1 Fix: BHD precision
	TotalForeign    float64 `gorm:"not null;default:0;check:total_foreign >= 0" json:"total_foreign"`
	TotalBHD        float64 `gorm:"type:decimal(15,3);not null;default:0;check:total_bhd >= 0" json:"total_bhd"` // P1 Fix: BHD precision

	// Payment
	PaymentTerms   string    `gorm:"type:varchar(500)" json:"payment_terms"`
	PaymentDueDate time.Time `json:"payment_due_date"`

	// Status Flow: Draft → Pending Approval → Approved → Sent → Acknowledged → Partially Received → Received → Closed
	Status string `gorm:"index;size:50;default:'Draft';check:status IN ('Draft','Pending Approval','Approved','Sent','Acknowledged','Partially Received','Received','Cancelled','Closed')" json:"status"` // P1 Fix: default status

	// P1 Fix: Approval workflow fields
	ApprovedBy string     `json:"approved_by"` // User who approved the PO
	ApprovedAt *time.Time `json:"approved_at"` // When PO was approved

	// P1 Fix: Audit fields
	UpdatedBy string `json:"updated_by"`

	// Relationships
	Items []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID" json:"items,omitempty"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

// PurchaseOrderItem represents a line item in a purchase order with quantities
// and pricing in both foreign currency and BHD.
type PurchaseOrderItem struct {
	Base
	PurchaseOrderID  string  `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:PurchaseOrderID;references:ID" json:"purchase_order_id"`
	OrderItemID      string  `gorm:"index;size:36" json:"order_item_id"`
	ProductID        string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:ProductID;references:ID" json:"product_id"`
	ProductCode      string  `gorm:"size:100" json:"product_code"`
	SupplierPartNumber string `gorm:"size:100" json:"supplier_part_number"` // P2 Fix: Supplier's internal part number
	Description      string  `gorm:"type:varchar(2000)" json:"description"`
	Quantity         float64 `json:"quantity"`
	UnitPriceForeign float64 `json:"unit_price_foreign"`
	UnitPriceBHD     float64 `json:"unit_price_bhd"`
	TotalForeign     float64 `json:"total_foreign"`
	TotalBHD         float64 `json:"total_bhd"`
	QuantityReceived float64 `json:"quantity_received"`
}

func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

// GoodsReceivedNote tracks receipt of goods from supplier with quality control
// workflow, rejection tracking, and warehouse assignment.
type GoodsReceivedNote struct {
	Base
	// P0 Fix: Changed from CASCADE to RESTRICT - deleting PO shouldn't delete GRN history
	PurchaseOrderID string    `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:PurchaseOrderID;references:ID" json:"purchase_order_id"`
	GRNNumber       string    `gorm:"uniqueIndex;size:50" json:"grn_number"`
	ReceivedDate    time.Time `gorm:"index;autoCreateTime:false" json:"received_date"`
	ReceivedBy      string    `gorm:"size:255" json:"received_by"`

	// Location
	WarehouseID string `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:WarehouseID;references:ID" json:"warehouse_id"`

	// Supplier Delivery Note Reference (P1 Fix)
	SupplierDNNumber string `gorm:"size:100" json:"supplier_dn_number"` // Supplier's delivery note reference

	// Quality Control
	QCStatus string     `gorm:"index;default:'Pending';check:qc_status IN ('Pending','Passed','Failed','Partial')" json:"qc_status"` // P1 Fix: indexed + default
	QCNotes  string     `gorm:"type:varchar(5000)" json:"qc_notes"`
	QCDate   *time.Time `json:"qc_date"`
	QCBy     string     `json:"qc_by"`

	// P1 Fix: Audit fields
	UpdatedBy string `json:"updated_by"`

	Items []GRNItem `gorm:"foreignKey:GRNID" json:"items,omitempty"`
}

func (GoodsReceivedNote) TableName() string { return "goods_received_notes" }

// GRNItem represents a line item in a GRN with quantities received, accepted,
// and rejected, with auto-calculated acceptance via BeforeSave hook.
type GRNItem struct {
	Base
	GRNID            string  `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:GRNID;references:ID" json:"grn_id"`
	// P0-5: Added FK constraint to prevent orphaned GRN items
	POItemID         string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:POItemID;references:ID" json:"po_item_id"`
	ProductID        string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:ProductID;references:ID" json:"product_id"`
	QuantityOrdered  float64 `gorm:"not null;default:0;check:quantity_ordered >= 0" json:"quantity_ordered"`
	QuantityReceived float64 `gorm:"not null;default:0;check:quantity_received >= 0" json:"quantity_received"`
	QuantityAccepted float64 `gorm:"not null;default:0;check:quantity_accepted >= 0" json:"quantity_accepted"`  // P2 Fix: QuantityReceived - QuantityRejected
	QuantityRejected float64 `gorm:"not null;default:0;check:quantity_rejected >= 0" json:"quantity_rejected"`
	RejectionReason  string  `gorm:"type:varchar(2000)" json:"rejection_reason"`
}

func (GRNItem) TableName() string { return "grn_items" }

// BeforeSave hook auto-calculates QuantityAccepted before saving (P0-1 Fix)
func (g *GRNItem) BeforeSave(tx *gorm.DB) error {
	g.QuantityAccepted = g.QuantityReceived - g.QuantityRejected
	return nil
}

// =============================================================================
// MODULE: OPERATIONS PIPELINE - SUPPLIER INVOICES
// =============================================================================

// SupplierInvoice represents a supplier invoice with OCR support, 3-way matching
// against PO and GRN, approval workflow, and GL integration.
type SupplierInvoice struct {
	Base
	// Links - P1 Fix: Composite index for supplier invoice queries: WHERE supplier_id = ? ORDER BY invoice_date
	// P2 Fix: Added covering index for supplier ledger: WHERE supplier_id = ? AND status = ?
	SupplierID      string `gorm:"index:idx_supplier_inv_date,priority:1;index:idx_supplier_inv_status,priority:1;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:SupplierID;references:ID" json:"supplier_id"`
	SupplierName    string `gorm:"index;size:255" json:"supplier_name"` // Denormalized for display
	PurchaseOrderID string `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:PurchaseOrderID;references:ID" json:"purchase_order_id"`
	PONumber        string `gorm:"size:50" json:"po_number"` // Denormalized for display
	GRNID           string `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:GRNID;references:ID" json:"grn_id"`
	OrderID         string `gorm:"index;size:36" json:"order_id"` // Internal customer order reference

	// Document Info (can be OCR'd)
	InvoiceNumber string    `gorm:"index;size:100" json:"invoice_number"` // Supplier's number
	// P1 Fix: Part of composite index idx_supplier_inv_date
	InvoiceDate   time.Time `gorm:"index:idx_supplier_inv_date,priority:2;autoCreateTime:false" json:"invoice_date"`
	DueDate       time.Time `gorm:"index" json:"due_date"`

	// Multi-Currency
	Currency        string  `gorm:"size:3" json:"currency"`
	ExchangeRate    float64 `gorm:"not null;default:1;check:exchange_rate > 0" json:"exchange_rate"`
	SubtotalForeign float64 `gorm:"not null;default:0;check:subtotal_foreign >= 0" json:"subtotal_foreign"`
	SubtotalBHD     float64 `gorm:"type:decimal(15,3);not null;default:0;check:subtotal_bhd >= 0" json:"subtotal_bhd"`     // P1 Fix: BHD precision
	VATForeign      float64 `gorm:"not null;default:0;check:vat_foreign >= 0" json:"vat_foreign"`
	VATBHD          float64 `gorm:"column:vatbhd;type:decimal(15,3);not null;default:0;check:vatbhd >= 0" json:"vat_bhd"` // P1 Fix: BHD precision
	TotalForeign    float64 `gorm:"not null;default:0;check:total_foreign >= 0" json:"total_foreign"`
	TotalBHD        float64 `gorm:"type:decimal(15,3);not null;default:0;check:total_bhd >= 0" json:"total_bhd"` // P1 Fix: BHD precision

	// 3-Way Match Status
	MatchStatus string `gorm:"index;default:'Pending';check:match_status IN ('Pending','Matched','Discrepancy','Dispute')" json:"match_status"` // P1 Fix: indexed + default
	POMatchOK   bool   `json:"po_match_ok"`  // Matches PO amounts?
	GRNMatchOK  bool   `json:"grn_match_ok"` // Matches GRN quantities?

	// Approval Workflow
	// P2 Fix: Part of covering index idx_supplier_inv_status
	Status     string     `gorm:"index:idx_supplier_inv_status,priority:2;size:50;default:'Pending';check:status IN ('Pending','Approved','Rejected','Paid','Dispute')" json:"status"` // P1 Fix: default status
	ApprovedBy string     `json:"approved_by"`
	ApprovedAt *time.Time `json:"approved_at"`

	// P1 Fix: Audit fields
	UpdatedBy string `json:"updated_by"`

	// Payment
	PaymentStatus string     `json:"payment_status"` // Unpaid, Scheduled, Paid
	PaymentDate   *time.Time `json:"payment_date"`
	PaymentRef    string     `json:"payment_ref"`
	PaymentMethod string     `json:"payment_method"` // Bank Transfer, Cheque, etc.

	// OCR Metadata
	OCRDocumentID string  `json:"ocr_document_id"` // Link to OCR scan
	OCRConfidence float64 `json:"ocr_confidence"`

	// Discrepancy tracking
	DiscrepancyReason string `gorm:"type:varchar(2000)" json:"discrepancy_reason"`
	DisputeReason     string `gorm:"type:varchar(2000)" json:"dispute_reason"`

	// Accounting Integration (Tally Killer feature)
	JournalEntryID string `gorm:"index;size:36" json:"journal_entry_id"` // Link to auto-generated GL entry

	// Line Items
	Items []SupplierInvoiceItem `gorm:"foreignKey:SupplierInvoiceID" json:"items"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

// SupplierInvoiceItem represents a line item on a supplier invoice with
// quantities and pricing in the invoice currency.
type SupplierInvoiceItem struct {
	Base
	SupplierInvoiceID string  `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:SupplierInvoiceID;references:ID" json:"supplier_invoice_id"`
	LineNumber        int     `json:"line_number"`
	Description       string  `gorm:"type:varchar(2000)" json:"description"`
	Quantity          float64 `json:"quantity"`
	UnitPrice         float64 `json:"unit_price"`
	TotalPrice        float64 `json:"total_price"`
	Currency          string  `gorm:"size:3" json:"currency"`
}

func (SupplierInvoiceItem) TableName() string { return "supplier_invoice_items" }

// =============================================================================
// MODULE: CHAT PERSISTENCE (Butler AI Conversations)
// =============================================================================

// Conversation represents a Butler AI chat session with title, summary,
// and message history for persistent intelligence interactions.
type Conversation struct {
	Base
	Title     string    `gorm:"size:255" json:"title"`
	Summary   string    `gorm:"type:varchar(2000)" json:"summary"`
	IsActive  bool      `gorm:"index;default:true" json:"is_active"`
	LastMsgAt time.Time `gorm:"index;autoUpdateTime" json:"last_msg_at"`
}

func (Conversation) TableName() string { return "conversations" }

// ChatMessage represents a single message in a conversation with role
// (user/assistant/system), content, and token usage tracking.
type ChatMessage struct {
	Base
	ConversationID string `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:ConversationID;references:ID" json:"conversation_id"`
	Role           string `gorm:"size:20;check:role IN ('user','assistant','system')" json:"role"`    // "user" or "assistant"
	Content        string `gorm:"type:varchar(5000)" json:"content"`
	TokensUsed     int    `gorm:"check:tokens_used >= 0" json:"tokens_used"`
}

func (ChatMessage) TableName() string { return "chat_messages" }

// =============================================================================
// MODULE: SUPPLIER PAYMENTS
// =============================================================================

// SupplierPayment tracks payments made to suppliers for their invoices with
// multi-currency support, payment method, and GL integration.
type SupplierPayment struct {
	Base
	// P2.4 Fix: Changed CASCADE to RESTRICT - payment records must never auto-delete
	SupplierInvoiceID string    `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:SupplierInvoiceID;references:ID" json:"supplier_invoice_id"`
	SupplierID        string    `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:SupplierID;references:ID" json:"supplier_id"`
	AmountForeign     float64   `gorm:"not null;default:0;check:amount_foreign >= 0" json:"amount_foreign"`
	Currency          string    `gorm:"size:3" json:"currency"`
	ExchangeRate      float64   `gorm:"not null;default:1;check:exchange_rate > 0" json:"exchange_rate"`
	AmountBHD         float64   `gorm:"type:decimal(15,3);not null;default:0;check:amount_bhd >= 0" json:"amount_bhd"` // P1 Fix: BHD precision
	PaymentDate       time.Time `gorm:"index;autoCreateTime:false" json:"payment_date"`
	PaymentMethod     string    `gorm:"size:50;check:payment_method IN ('Bank Transfer','Cheque','LC','Cash','Wire Transfer','PDC','Other')" json:"payment_method"` // Bank Transfer, Cheque, LC
	Reference         string    `gorm:"size:100" json:"reference"`     // Cheque/transfer reference
	Notes             string    `gorm:"type:varchar(2000)" json:"notes"`

	// Accounting Integration (Tally Killer feature)
	JournalEntryID string `gorm:"index;size:36" json:"journal_entry_id"` // Link to auto-generated GL entry
	BankAccountID  string `gorm:"index;size:36" json:"bank_account_id"`  // Bank account for reconciliation

	// P1 Fix: Audit field (DeletedAt already in Base for soft delete)
	UpdatedBy string `json:"updated_by"`

	// Display fields (populated from linked invoice for UI display)
	SupplierName  string `gorm:"column:supplier_name;size:255" json:"supplier_name"`
	InvoiceNumber string `gorm:"column:invoice_number;size:100" json:"invoice_number"`
}

func (SupplierPayment) TableName() string { return "supplier_payments" }

// =============================================================================
// MODULE: OFFER FOLLOW-UPS
// =============================================================================

// OfferFollowUp schedules and tracks follow-up actions on offers with
// completion status and overdue tracking.
type OfferFollowUp struct {
	Base
	OfferID      string     `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:OfferID;references:ID" json:"offer_id"`
	FollowUpDate time.Time  `gorm:"index;autoCreateTime:false" json:"follow_up_date"`
	Notes        string     `gorm:"type:varchar(2000)" json:"notes"`
	Status       string     `gorm:"index;size:20;default:'pending';check:status IN ('pending','completed','cancelled','overdue')" json:"status"` // pending, completed, cancelled
	CompletedAt  *time.Time `json:"completed_at"`
	CompletedBy  string     `gorm:"size:255" json:"completed_by"`
}

func (OfferFollowUp) TableName() string { return "offer_follow_ups" }

// OfferNote stores freeform notes/comments on offers
type OfferNote struct {
	Base
	OfferID   string    `gorm:"index;size:36" json:"offer_id"`
	NoteDate  time.Time `gorm:"index" json:"note_date"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedBy string    `gorm:"size:255" json:"created_by"`
}

func (OfferNote) TableName() string { return "offer_notes" }

// =============================================================================
// MODULE: SYNC INFRASTRUCTURE
// =============================================================================

// SyncRecord tracks synchronization state between local and remote databases
// with conflict resolution (local_wins/remote_wins) and version tracking.
type SyncRecord struct {
	Base
	SyncTable     string    `gorm:"index;size:100" json:"sync_table"`
	RecordID      string    `gorm:"index;size:36" json:"record_id"`
	SyncedAt      time.Time `gorm:"index;autoUpdateTime" json:"synced_at"`
	Direction     string    `gorm:"size:10;check:direction IN ('push','pull')" json:"direction"` // "push" or "pull"
	RemoteVersion int       `gorm:"check:remote_version >= 0" json:"remote_version"`
	LocalVersion  int       `gorm:"check:local_version >= 0" json:"local_version"`
	ConflictState string    `gorm:"size:20;check:conflict_state IN ('none','local_wins','remote_wins')" json:"conflict_state"` // "none", "local_wins", "remote_wins"
}

func (SyncRecord) TableName() string { return "sync_records" }

// =============================================================================
// MODULE: TALLY DATA IMPORTS
// =============================================================================

// TallyInvoiceImport stores imported Tally invoice records with customer matching,
// status tracking, and raw data preservation for reconciliation.
type TallyInvoiceImport struct {
	Base
	ImportBatch     string    `gorm:"index;size:36" json:"import_batch"`
	Year            int       `gorm:"index;check:year >= 2000 AND year <= 2100" json:"year"`
	InvoiceNumber   string    `gorm:"index;size:100" json:"invoice_number"`
	CustomerName    string    `gorm:"index;size:255" json:"customer_name"`
	MatchedCustomerID string  `gorm:"index;size:36" json:"matched_customer_id"`
	InvoiceDate     time.Time `gorm:"autoCreateTime:false" json:"invoice_date"`
	Amount          float64   `gorm:"check:amount >= 0" json:"amount"`
	Currency        string    `gorm:"size:3" json:"currency"`
	Status          string    `gorm:"index;size:50;check:status IN ('imported','matched','duplicate','error','pending')" json:"status"` // imported, matched, duplicate, error
	RawData         string    `gorm:"type:varchar(5000)" json:"raw_data"`
}

func (TallyInvoiceImport) TableName() string { return "tally_invoice_imports" }

// TallyPurchaseImport stores imported Tally purchase records with supplier matching,
// status tracking, and raw data preservation for reconciliation.
type TallyPurchaseImport struct {
	Base
	ImportBatch     string    `gorm:"index;size:36" json:"import_batch"`
	Year            int       `gorm:"index;check:year >= 2000 AND year <= 2100" json:"year"`
	InvoiceNumber   string    `gorm:"index;size:100" json:"invoice_number"`
	SupplierName    string    `gorm:"index;size:255" json:"supplier_name"`
	MatchedSupplierID string  `gorm:"index;size:36" json:"matched_supplier_id"`
	InvoiceDate     time.Time `gorm:"autoCreateTime:false" json:"invoice_date"`
	Amount          float64   `gorm:"check:amount >= 0" json:"amount"`
	Currency        string    `gorm:"size:3" json:"currency"`
	Status          string    `gorm:"index;size:50;check:status IN ('imported','matched','duplicate','error','pending')" json:"status"` // imported, matched, duplicate, error
	RawData         string    `gorm:"type:varchar(5000)" json:"raw_data"`
}

func (TallyPurchaseImport) TableName() string { return "tally_purchase_imports" }

// =============================================================================
// MODULE: ACCOUNTING ENGINE (Tally Killer)
// =============================================================================

// AccountMapping maps transaction types to GL account codes for automatic
// journal entry generation (AR, AP, Revenue, COGS, VAT, etc.).
type AccountMapping struct {
	Base
	TransactionType string `gorm:"uniqueIndex;size:50" json:"transaction_type"` // AR, AP, Revenue, COGS, Cash, Bank, VAT_Output, VAT_Input
	AccountID       string `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:AccountID;references:ID" json:"account_id"`
	AccountCode     string `gorm:"size:20" json:"account_code"`
	AccountName     string `gorm:"size:255" json:"account_name"`
	Description     string `gorm:"type:varchar(1000)" json:"description"`
	IsActive        bool   `gorm:"default:true" json:"is_active"`
}

func (AccountMapping) TableName() string { return "account_mappings" }

// FiscalPeriod represents a fiscal period (month) within a fiscal year with
// open/closed/locked status and closing audit trail.
type FiscalPeriod struct {
	Base
	FiscalYear  int        `gorm:"index;check:fiscal_year >= 2000 AND fiscal_year <= 2100" json:"fiscal_year"`
	Period      int        `gorm:"index;check:period >= 1 AND period <= 12" json:"period"`      // 1-12 for months
	PeriodStart time.Time  `json:"period_start"`
	PeriodEnd   time.Time  `json:"period_end"`
	Status      string     `gorm:"index;size:20;default:'Open';check:status IN ('Open','Closed','Locked')" json:"status"` // Open, Closed, Locked
	ClosedAt    *time.Time `json:"closed_at"`
	ClosedBy    string     `gorm:"size:255" json:"closed_by"`
}

func (FiscalPeriod) TableName() string { return "fiscal_periods" }

// BankAccount represents a bank account for reconciliation with GL integration,
// balance tracking, and multi-currency support.
type BankAccount struct {
	Base
	AccountID     string  `gorm:"index;size:36;constraint:OnDelete:RESTRICT,OnUpdate:CASCADE;foreignKey:AccountID;references:ID" json:"account_id"` // Link to ChartOfAccount
	BankName      string  `gorm:"size:255" json:"bank_name"`
	AccountNumber string  `gorm:"size:100" json:"account_number"`
	AccountName   string  `gorm:"size:255" json:"account_name"`
	Currency      string  `gorm:"size:3;default:'BHD'" json:"currency"`
	CurrentBalance float64 `gorm:"type:decimal(15,3)" json:"current_balance"`
	IsActive      bool    `gorm:"index;default:true" json:"is_active"`
}

func (BankAccount) TableName() string { return "bank_accounts" }

// BankStatement represents an imported bank statement with opening/closing
// balances, reconciliation status, and line items for matching.
type BankStatement struct {
	Base
	BankAccountID   string    `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:BankAccountID;references:ID" json:"bank_account_id"`
	StatementNumber string    `gorm:"uniqueIndex;size:50" json:"statement_number"`
	StatementDate   time.Time `gorm:"index;autoCreateTime:false" json:"statement_date"`
	PeriodStart     time.Time `json:"period_start"`
	PeriodEnd       time.Time `json:"period_end"`
	OpeningBalance  float64   `gorm:"type:decimal(15,3)" json:"opening_balance"`
	ClosingBalance  float64   `gorm:"type:decimal(15,3)" json:"closing_balance"`
	Currency        string    `gorm:"size:3;default:'BHD'" json:"currency"`

	// Totals
	TotalDebits  float64 `gorm:"type:decimal(15,3)" json:"total_debits"`
	TotalCredits float64 `gorm:"type:decimal(15,3)" json:"total_credits"`
	DebitCount   int     `json:"debit_count"`
	CreditCount  int     `json:"credit_count"`

	// Status
	Status       string     `gorm:"index;size:20;default:'Imported';check:status IN ('Imported','InProgress','Reconciled','Verified','Cancelled')" json:"status"`
	ReconciledAt *time.Time `json:"reconciled_at"`
	ReconciledBy string     `gorm:"size:255" json:"reconciled_by"`

	// Import metadata
	ImportedFrom  string  `gorm:"size:500" json:"imported_from"`
	ImportMethod  string  `gorm:"size:20" json:"import_method"` // PDF_OCR, CSV, Manual
	OCRConfidence float64 `json:"ocr_confidence"`

	// Verification
	BalanceVerified   bool    `gorm:"default:false" json:"balance_verified"`
	DiscrepancyAmount float64 `gorm:"type:decimal(15,3)" json:"discrepancy_amount"`

	Lines []BankStatementLine `gorm:"foreignKey:BankStatementID" json:"lines,omitempty"`
}

func (BankStatement) TableName() string { return "bank_statements" }

// BankStatementLine represents a single line in a bank statement with
// automatic/manual matching to payments and journal entries.
type BankStatementLine struct {
	Base
	BankStatementID string    `gorm:"index;size:36;constraint:OnDelete:CASCADE,OnUpdate:CASCADE;foreignKey:BankStatementID;references:ID" json:"bank_statement_id"`
	LineNumber      int       `json:"line_number"`
	TransactionDate time.Time `gorm:"index;autoCreateTime:false" json:"transaction_date"`
	ValueDate       time.Time `json:"value_date"`
	Description     string    `gorm:"type:varchar(2000)" json:"description"`
	Reference       string    `gorm:"index;size:100" json:"reference"`
	Debit           float64   `gorm:"type:decimal(15,3);check:debit >= 0" json:"debit"`
	Credit          float64   `gorm:"type:decimal(15,3);check:credit >= 0" json:"credit"`
	Balance         float64   `gorm:"type:decimal(15,3)" json:"balance"`

	// Categorization (Enhanced)
	TransactionType    string `gorm:"size:50" json:"transaction_type"`
	Category           string `gorm:"size:50" json:"category"`
	SubCategory        string `gorm:"size:50" json:"sub_category"`
	ExtractedCustomer  string `gorm:"size:100" json:"extracted_customer"`
	ExtractedSupplier  string `gorm:"size:100" json:"extracted_supplier"`
	ExtractedInvoices  string `gorm:"type:text" json:"extracted_invoices"`   // JSON array
	ExtractedPONumbers string `gorm:"type:text" json:"extracted_po_numbers"` // JSON array

	// Matching
	IsMatched         bool    `gorm:"index;default:false" json:"is_matched"`
	MatchedPaymentID  string  `gorm:"index;size:36" json:"matched_payment_id"`
	MatchedJournalID  string  `gorm:"index;size:36" json:"matched_journal_id"`
	MatchedInvoiceIDs string  `gorm:"type:text" json:"matched_invoice_ids"` // JSON array
	MatchedExpenseID  *string `gorm:"size:36" json:"matched_expense_id"`
	MatchType         string  `gorm:"size:20" json:"match_type"` // Auto, Manual, Split, Unmatched
	MatchConfidence   float64 `json:"match_confidence"`

	// Verification
	VerifiedBy string     `gorm:"size:100" json:"verified_by"`
	VerifiedAt *time.Time `json:"verified_at"`
	Notes      string     `gorm:"type:text" json:"notes"`
}

func (BankStatementLine) TableName() string { return "bank_statement_lines" }

// =============================================================================
// MODULE: BANK RECONCILIATION SSOT (Feature B - February 2026)
// =============================================================================

// BankLinePaymentAllocation supports split payments where one bank line
// pays multiple invoices or one invoice is paid across multiple bank lines.
type BankLinePaymentAllocation struct {
	Base
	BankStatementLineID string  `gorm:"index;size:36" json:"bank_statement_line_id"`
	AllocationType      string  `gorm:"size:30" json:"allocation_type"` // CUSTOMER_INVOICE, SUPPLIER_INVOICE, EXPENSE
	CustomerInvoiceID   *string `gorm:"size:36" json:"customer_invoice_id"`
	SupplierInvoiceID   *string `gorm:"size:36" json:"supplier_invoice_id"`
	ExpenseEntryID      *string `gorm:"size:36" json:"expense_entry_id"`
	AllocatedAmount     float64 `gorm:"type:decimal(15,3)" json:"allocated_amount"`
	Currency            string  `gorm:"size:3;default:'BHD'" json:"currency"`
	Status              string  `gorm:"size:20;default:'Allocated'" json:"status"` // Allocated, Verified, Disputed
}

func (BankLinePaymentAllocation) TableName() string { return "bank_line_payment_allocations" }

// BankCashBalance is the SSOT for cash position per bank account per date.
type BankCashBalance struct {
	Base
	BankAccountID    string    `gorm:"uniqueIndex:idx_bank_date;size:36" json:"bank_account_id"`
	BalanceDate      time.Time `gorm:"uniqueIndex:idx_bank_date" json:"balance_date"`
	StatementBalance float64   `gorm:"type:decimal(15,3)" json:"statement_balance"`
	ComputedBalance  float64   `gorm:"type:decimal(15,3)" json:"computed_balance"`
	Currency         string    `gorm:"size:3;default:'BHD'" json:"currency"`
	Discrepancy      float64   `gorm:"type:decimal(15,3)" json:"discrepancy"`
	IsReconciled     bool      `gorm:"default:false" json:"is_reconciled"`
	BankStatementID  *string   `gorm:"size:36" json:"bank_statement_id"`
}

func (BankCashBalance) TableName() string { return "bank_cash_balances" }

// BankExpenseEntry auto-creates expense entries for bank fees, VAT, etc.
type BankExpenseEntry struct {
	Base
	BankStatementLineID string    `gorm:"index;size:36" json:"bank_statement_line_id"`
	ExpenseDate         time.Time `json:"expense_date"`
	Description         string    `gorm:"size:500" json:"description"`
	Category            string    `gorm:"size:50" json:"category"` // BANK_FEE, SWIFT_CHARGE, VAT, BG_FEE, OTHER
	Amount              float64   `gorm:"type:decimal(15,3)" json:"amount"`
	Currency            string    `gorm:"size:3;default:'BHD'" json:"currency"`
	VATAmount           float64   `gorm:"type:decimal(15,3)" json:"vat_amount"`
	GLAccountID         *string   `gorm:"size:36" json:"gl_account_id"`
	IsPosted            bool      `gorm:"default:false" json:"is_posted"`
	JournalEntryID      *string   `gorm:"size:36" json:"journal_entry_id"`
}

func (BankExpenseEntry) TableName() string { return "bank_expense_entries" }

// StatementHash prevents duplicate statement imports via SHA-256 hash.
type StatementHash struct {
	Base
	BankAccountID    string    `gorm:"index;size:36" json:"bank_account_id"`
	StatementHash    string    `gorm:"uniqueIndex;size:64" json:"statement_hash"`
	PeriodStart      time.Time `json:"period_start"`
	PeriodEnd        time.Time `json:"period_end"`
	TransactionCount int       `json:"transaction_count"`
	ClosingBalance   float64   `gorm:"type:decimal(15,3)" json:"closing_balance"`
	ImportedAt       time.Time `json:"imported_at"`
	BankStatementID  string    `gorm:"size:36" json:"bank_statement_id"`
}

func (StatementHash) TableName() string { return "statement_hashes" }

// BookBankReconciliation for traditional book vs bank reconciliation.
type BookBankReconciliation struct {
	Base
	BankAccountID          string     `gorm:"index;size:36" json:"bank_account_id"`
	ReconciliationDate     time.Time  `json:"reconciliation_date"`
	Currency               string     `gorm:"size:3;default:'BHD'" json:"currency"`
	BankStatementBalance   float64    `gorm:"type:decimal(15,3)" json:"bank_statement_balance"`
	DepositsInTransit      float64    `gorm:"type:decimal(15,3)" json:"deposits_in_transit"`
	OutstandingCheques     float64    `gorm:"type:decimal(15,3)" json:"outstanding_cheques"`
	BankErrors             float64    `gorm:"type:decimal(15,3)" json:"bank_errors"`
	AdjustedBankBalance    float64    `gorm:"type:decimal(15,3)" json:"adjusted_bank_balance"`
	BookBalance            float64    `gorm:"type:decimal(15,3)" json:"book_balance"`
	BankChargesNotRecorded float64    `gorm:"type:decimal(15,3)" json:"bank_charges_not_recorded"`
	InterestNotRecorded    float64    `gorm:"type:decimal(15,3)" json:"interest_not_recorded"`
	NSFCheques             float64    `gorm:"type:decimal(15,3)" json:"nsf_cheques"`
	BookErrors             float64    `gorm:"type:decimal(15,3)" json:"book_errors"`
	AdjustedBookBalance    float64    `gorm:"type:decimal(15,3)" json:"adjusted_book_balance"`
	Difference             float64    `gorm:"type:decimal(15,3)" json:"difference"`
	IsReconciled           bool       `gorm:"default:false" json:"is_reconciled"`
	ReconciledBy           string     `gorm:"size:100" json:"reconciled_by"`
	ReconciledAt           *time.Time `json:"reconciled_at"`
	Notes                  string     `gorm:"type:text" json:"notes"`
}

func (BookBankReconciliation) TableName() string { return "book_bank_reconciliations" }

// OutstandingCheque tracks cheques from issuance through clearance.
type OutstandingCheque struct {
	Base
	BankAccountID string     `gorm:"index;size:36" json:"bank_account_id"`
	ChequeNumber  string     `gorm:"index;size:20" json:"cheque_number"`
	Amount        float64    `gorm:"type:decimal(15,3)" json:"amount"`
	Currency      string     `gorm:"size:3;default:'BHD'" json:"currency"`
	IssuedDate    time.Time  `json:"issued_date"`
	PayeeName     string     `gorm:"size:200" json:"payee_name"`
	PayeeType     string     `gorm:"size:20" json:"payee_type"` // SUPPLIER, EMPLOYEE, OTHER
	SupplierID    *string    `gorm:"size:36" json:"supplier_id"`
	Purpose       string     `gorm:"size:500" json:"purpose"`
	Status        string     `gorm:"index;size:20;default:'ISSUED'" json:"status"` // ISSUED, PRESENTED, CLEARED, STALE, CANCELLED, BOUNCED
	ClearedDate   *time.Time `json:"cleared_date"`
	MatchedLineID *string    `gorm:"size:36" json:"matched_line_id"`
	IsStale       bool       `gorm:"default:false" json:"is_stale"`
	StaleDate     *time.Time `json:"stale_date"`
	ReissuedAs    *string    `gorm:"size:20" json:"reissued_as"`
}

func (OutstandingCheque) TableName() string { return "outstanding_cheques" }

// DepositInTransit tracks deposits made but not yet appearing on bank statement.
type DepositInTransit struct {
	Base
	BankAccountID string     `gorm:"index;size:36" json:"bank_account_id"`
	DepositDate   time.Time  `json:"deposit_date"`
	Amount        float64    `gorm:"type:decimal(15,3)" json:"amount"`
	Currency      string     `gorm:"size:3;default:'BHD'" json:"currency"`
	DepositSlipNo string     `gorm:"size:50" json:"deposit_slip_no"`
	Description   string     `gorm:"size:500" json:"description"`
	SourceType    string     `gorm:"size:30" json:"source_type"` // CUSTOMER_PAYMENT, CASH_SALE, OTHER
	CustomerID    *string    `gorm:"size:36" json:"customer_id"`
	InvoiceIDs    string     `gorm:"type:text" json:"invoice_ids"` // JSON array
	Status        string     `gorm:"index;size:20;default:'PENDING'" json:"status"` // PENDING, CLEARED, RETURNED
	ClearedDate   *time.Time `json:"cleared_date"`
	MatchedLineID *string    `gorm:"size:36" json:"matched_line_id"`
}

func (DepositInTransit) TableName() string { return "deposits_in_transit" }

// ChequeRegister tracks cheque books and sequential numbering.
type ChequeRegister struct {
	Base
	BankAccountID string     `gorm:"index;size:36" json:"bank_account_id"`
	ChequeBookNo  string     `gorm:"size:20" json:"cheque_book_no"`
	StartNumber   int        `json:"start_number"`
	EndNumber     int        `json:"end_number"`
	CurrentNumber int        `json:"current_number"`
	Status        string     `gorm:"index;size:20;default:'ACTIVE'" json:"status"` // ACTIVE, EXHAUSTED, CANCELLED
	IssuedDate    time.Time  `json:"issued_date"`
	ExhaustedDate *time.Time `json:"exhausted_date"`
}

func (ChequeRegister) TableName() string { return "cheque_registers" }

// FXRate stores foreign exchange rates for multi-currency accounts.
type FXRate struct {
	Base
	FromCurrency string    `gorm:"size:3;uniqueIndex:idx_fx_date,priority:1" json:"from_currency"`
	ToCurrency   string    `gorm:"size:3;uniqueIndex:idx_fx_date,priority:2" json:"to_currency"`
	RateDate     time.Time `gorm:"uniqueIndex:idx_fx_date,priority:3" json:"rate_date"`
	Rate         float64   `gorm:"type:decimal(12,6)" json:"rate"`
	Source       string    `gorm:"size:20" json:"source"` // CBB, MANUAL, API
}

func (FXRate) TableName() string { return "fx_rates" }

// FXRevaluation tracks unrealized FX gain/loss for foreign currency accounts.
type FXRevaluation struct {
	Base
	BankAccountID   string     `gorm:"index;size:36" json:"bank_account_id"`
	RevaluationDate time.Time  `json:"revaluation_date"`
	ForeignCurrency string     `gorm:"size:3" json:"foreign_currency"`
	ForeignBalance  float64    `gorm:"type:decimal(15,3)" json:"foreign_balance"`
	PreviousRate    float64    `gorm:"type:decimal(12,6)" json:"previous_rate"`
	PreviousBHD     float64    `gorm:"type:decimal(15,3)" json:"previous_bhd"`
	CurrentRate     float64    `gorm:"type:decimal(12,6)" json:"current_rate"`
	CurrentBHD      float64    `gorm:"type:decimal(15,3)" json:"current_bhd"`
	GainLossBHD     float64    `gorm:"type:decimal(15,3)" json:"gain_loss_bhd"`
	IsPosted        bool       `gorm:"default:false" json:"is_posted"`
	JournalEntryID  *string    `gorm:"size:36" json:"journal_entry_id"`
	PostedBy        string     `gorm:"size:100" json:"posted_by"`
	PostedAt        *time.Time `json:"posted_at"`
}

func (FXRevaluation) TableName() string { return "fx_revaluations" }

// BankStatementFile archives original PDF/CSV files for audit trail.
type BankStatementFile struct {
	Base
	BankStatementID string     `gorm:"index;size:36" json:"bank_statement_id"`
	FileName        string     `gorm:"size:255" json:"file_name"`
	FileType        string     `gorm:"size:10" json:"file_type"` // PDF, CSV, XLS
	FileSize        int64      `json:"file_size"`
	FileHash        string     `gorm:"size:64" json:"file_hash"` // SHA-256
	StoragePath     string     `gorm:"size:500" json:"storage_path"`
	IsStored        bool       `gorm:"default:false" json:"is_stored"`
	OCREngine       string     `gorm:"size:30" json:"ocr_engine"`
	OCRConfidence   float64    `json:"ocr_confidence"`
	OCRProcessedAt  *time.Time `json:"ocr_processed_at"`
}

func (BankStatementFile) TableName() string { return "bank_statement_files" }

// BankReconciliationAuditLog provides complete audit trail for all reconciliation actions.
type BankReconciliationAuditLog struct {
	Base
	BankStatementID     string     `gorm:"index;size:36" json:"bank_statement_id"`
	BankStatementLineID *string    `gorm:"index;size:36" json:"bank_statement_line_id"`
	Action              string     `gorm:"size:30" json:"action"` // IMPORT, MATCH, UNMATCH, SPLIT, CATEGORIZE, RECONCILE, VERIFY
	ActionDetail        string     `gorm:"type:text" json:"action_detail"` // JSON
	PerformedBy         string     `gorm:"size:100" json:"performed_by"`
	PerformedAt         time.Time  `gorm:"index" json:"performed_at"`
	IsAutomatic         bool       `gorm:"default:false" json:"is_automatic"`
	ConfidenceScore     float64    `json:"confidence_score"`
	Reason              string     `gorm:"type:text" json:"reason"`
	IsReversed          bool       `gorm:"default:false" json:"is_reversed"`
	ReversedBy          string     `gorm:"size:100" json:"reversed_by"`
	ReversedAt          *time.Time `json:"reversed_at"`
	ReversalReason      string     `gorm:"type:text" json:"reversal_reason"`
}

func (BankReconciliationAuditLog) TableName() string { return "bank_reconciliation_audit_logs" }

// =============================================================================
// DATABASE BACKUP & INTEGRITY
// =============================================================================

const maxBackups = 7

// BackupDatabase creates an atomic backup of the SQLite database using VACUUM INTO.
// Backups are stored in a "backups/" directory next to the database file.
// Keeps the last 7 backups and deletes older ones.
func (a *App) BackupDatabase() (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	// Determine database file path
	sqlDB, err := a.db.DB()
	if err != nil {
		return "", fmt.Errorf("cannot access database connection: %w", err)
	}

	// Get the database file path from the DSN
	dbPath := "ph_holdings.db" // default
	var dbName string
	row := sqlDB.QueryRow("PRAGMA database_list")
	var seq int
	if err := row.Scan(&seq, &dbName, &dbPath); err != nil {
		// Fallback to default
		dbPath = "ph_holdings.db"
	}

	// Create backups directory next to the database
	backupDir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(backupDir, 0700); err != nil {
		return "", fmt.Errorf("cannot create backup directory: %w", err)
	}

	// Generate backup filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("ph_holdings_%s.db", timestamp))

	// Use VACUUM INTO for an atomic, consistent backup
	// Escape single quotes in path to prevent malformed SQL (e.g. /Users/O'Brien/...)
	escapedPath := strings.ReplaceAll(backupPath, "'", "''")
	if _, err := sqlDB.Exec(fmt.Sprintf("VACUUM INTO '%s'", escapedPath)); err != nil {
		return "", fmt.Errorf("backup failed: %w", err)
	}

	// Restrict backup file permissions (owner read/write only)
	if err := os.Chmod(backupPath, 0600); err != nil {
		log.Printf("Warning: could not set backup file permissions: %v", err)
	}

	log.Printf("Database backup created: %s", backupPath)

	// Prune old backups (keep last maxBackups)
	a.pruneOldBackups(backupDir)

	return backupPath, nil
}

// pruneOldBackups keeps the most recent maxBackups files and deletes older ones.
func (a *App) pruneOldBackups(backupDir string) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		log.Printf("Warning: cannot read backup directory: %v", err)
		return
	}

	// Filter to only backup .db files matching expected naming pattern
	var backups []os.DirEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		matched, _ := filepath.Match("ph_holdings_*_*.db", e.Name())
		if matched {
			backups = append(backups, e)
		}
	}

	if len(backups) <= maxBackups {
		return
	}

	// Sort by name (timestamp-based names sort chronologically)
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Name() < backups[j].Name()
	})

	// Delete oldest backups
	for i := 0; i < len(backups)-maxBackups; i++ {
		path := filepath.Join(backupDir, backups[i].Name())
		if err := os.Remove(path); err != nil {
			log.Printf("Warning: cannot delete old backup %s: %v", path, err)
		} else {
			log.Printf("Pruned old backup: %s", backups[i].Name())
		}
	}
}

// runIntegrityCheck is the internal implementation (used at startup, no RBAC).
func (a *App) runIntegrityCheck() string {
	if a.db == nil {
		return "error: database not initialized"
	}

	sqlDB, err := a.db.DB()
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}

	var result string
	if err := sqlDB.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		log.Printf("CRITICAL: Database integrity check failed: %v", err)
		return fmt.Sprintf("error: %v", err)
	}

	if result != "ok" {
		log.Printf("WARNING: Database integrity issues detected: %s", result)
	} else {
		log.Println("Database integrity check: OK")
	}

	return result
}

// RunIntegrityCheck is the exported version with RBAC guard (expensive PRAGMA).
func (a *App) RunIntegrityCheck() string {
	if err := a.requirePermission("settings:view"); err != nil {
		return "error: permission denied"
	}
	return a.runIntegrityCheck()
}

// TriggerBackup is an exported function callable from the UI to create a manual backup.
func (a *App) TriggerBackup() map[string]interface{} {
	if err := a.requirePermission("settings:update"); err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Permission denied: admin access required",
		}
	}

	path, err := a.BackupDatabase()
	if err != nil {
		return map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		}
	}

	return map[string]interface{}{
		"success":     true,
		"backup_path": path,
		"timestamp":   time.Now().Format(time.RFC3339),
	}
}

// GetBackupInfo returns information about existing backups.
func (a *App) GetBackupInfo() map[string]interface{} {
	if err := a.requirePermission("settings:view"); err != nil {
		return map[string]interface{}{
			"count":       0,
			"last_backup": "",
			"error":       "permission denied",
		}
	}
	// Compute backup dir the same way as BackupDatabase — next to the DB file
	backupDir := filepath.Join(".", "backups") // default
	if a.db != nil {
		if sqlDB, err := a.db.DB(); err == nil {
			var seq int
			var dbName, dbPath string
			if err := sqlDB.QueryRow("PRAGMA database_list").Scan(&seq, &dbName, &dbPath); err == nil && dbPath != "" {
				backupDir = filepath.Join(filepath.Dir(dbPath), "backups")
			}
		}
	}
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return map[string]interface{}{
			"count":       0,
			"last_backup": "",
		}
	}

	var backups []string
	var totalSize int64
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".db" {
			backups = append(backups, e.Name())
			if info, err := e.Info(); err == nil {
				totalSize += info.Size()
			}
		}
	}

	sort.Strings(backups)

	lastBackup := ""
	if len(backups) > 0 {
		lastBackup = backups[len(backups)-1]
	}

	return map[string]interface{}{
		"count":          len(backups),
		"last_backup":    lastBackup,
		"total_size_mb":  float64(totalSize) / (1024 * 1024),
		"backup_dir":     backupDir,
	}
}
