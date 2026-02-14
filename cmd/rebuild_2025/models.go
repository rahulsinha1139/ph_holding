package main

import (
	"time"

	"gorm.io/gorm"
)

// =============================================================================
// GORM MODEL DEFINITIONS FOR AUTOMIGRATE
// =============================================================================
// These models are copied from database.go to ensure AutoMigrate creates
// all tables with correct schema, foreign keys, and indexes.

// Base provides common fields for all database models
type Base struct {
	ID        string         `gorm:"primaryKey;size:36" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
	Version   int            `gorm:"default:1" json:"version"`
	CreatedBy string         `json:"created_by"`
}

// =============================================================================
// CRM & MASTER DATA
// =============================================================================

type CustomerMaster struct {
	Base
	CompanyName        string                         `gorm:"index" json:"company_name"`
	ContactPerson      string                         `json:"contact_person"`
	Email              string                         `gorm:"index" json:"email"`
	Phone              string                         `json:"phone"`
	Website            string                         `json:"website"`
	BusinessType       string                         `json:"business_type"`
	IndustryCode       string                         `json:"industry_code"`
	CustomerCategory   string                         `json:"customer_category"`
	CustomerRating     string                         `json:"customer_rating"`
	CustomerSince      time.Time                      `json:"customer_since"`
	BillingAddress     string                         `json:"billing_address"`
	ShippingAddress    string                         `json:"shipping_address"`
	City               string                         `gorm:"index" json:"city"`
	State              string                         `json:"state"`
	PostalCode         string                         `json:"postal_code"`
	Country            string                         `gorm:"default:Bahrain" json:"country"`
	TaxID              string                         `gorm:"index" json:"tax_id"`
	PaymentTerms       string                         `json:"payment_terms"`
	DefaultDiscount    float64                        `json:"default_discount"`
	CreditLimit        float64                        `json:"credit_limit"`
	OutstandingBalance float64                        `json:"outstanding_balance"`
	TotalRevenue       float64                        `json:"total_revenue"`
	LastOrderDate      *time.Time                     `json:"last_order_date"`
	IsActive           bool                           `gorm:"index;default:true" json:"is_active"`
	Notes              string                         `json:"notes"`
	TallyLedgerName    string                         `gorm:"index" json:"tally_ledger_name"`
	Grade              string                         `gorm:"index;default:B" json:"grade"`
	Contacts           []CustomerContact              `gorm:"foreignKey:CustomerID" json:"contacts"`
	Orders             []Order                        `gorm:"foreignKey:CustomerID" json:"orders"`
	Invoices           []Invoice                      `gorm:"foreignKey:CustomerID" json:"invoices"`
	Payments           []Payment                      `gorm:"foreignKey:CustomerID" json:"payments"`
	Opportunities      []Opportunity                  `gorm:"foreignKey:CustomerID" json:"opportunities"`
	Predictions        []WinProbabilityPrediction     `gorm:"foreignKey:CustomerID" json:"predictions"`
	Recommendations    []DiscountRecommendationRecord `gorm:"foreignKey:CustomerID" json:"recommendations"`
	Snapshots          []CustomerSnapshot             `gorm:"foreignKey:CustomerID" json:"snapshots"`
	Outcomes           []ActualOutcome                `gorm:"foreignKey:CustomerID" json:"outcomes"`
}

func (CustomerMaster) TableName() string { return "customers" }

type CustomerContact struct {
	Base
	CustomerID       string `gorm:"index" json:"customer_id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	Designation      string `json:"designation"`
	IsPrimaryContact bool   `json:"is_primary_contact"`
}

func (CustomerContact) TableName() string { return "customer_contacts" }

type SupplierContact struct {
	Base
	SupplierID       string `gorm:"index" json:"supplier_id"`
	Name             string `json:"name"`
	Email            string `json:"email"`
	Phone            string `json:"phone"`
	Designation      string `json:"designation"`
	IsPrimaryContact bool   `json:"is_primary_contact"`
}

func (SupplierContact) TableName() string { return "supplier_contacts" }

type SupplierMaster struct {
	Base
	CompanyName        string              `gorm:"index" json:"company_name"`
	ContactPerson      string              `json:"contact_person"`
	Email              string              `gorm:"index" json:"email"`
	Phone              string              `json:"phone"`
	Website            string              `json:"website"`
	Address            string              `json:"address"`
	City               string              `gorm:"index" json:"city"`
	State              string              `json:"state"`
	PostalCode         string              `json:"postal_code"`
	Country            string              `gorm:"default:Germany" json:"country"`
	TaxID              string              `gorm:"index" json:"tax_id"`
	PaymentTerms       string              `json:"payment_terms"`
	LeadTimeDays       int                 `json:"lead_time_days"`
	CreditLimit        float64             `json:"credit_limit"`
	OutstandingBalance float64             `json:"outstanding_balance"`
	TotalPurchases     float64             `json:"total_purchases"`
	LastPurchaseDate   *time.Time          `json:"last_purchase_date"`
	IsActive           bool                `gorm:"index;default:true" json:"is_active"`
	Notes              string              `json:"notes"`
	TallyLedgerName    string              `gorm:"index" json:"tally_ledger_name"`
	Rating             string              `gorm:"default:★★★" json:"rating"`
	PurchaseOrders     []PurchaseOrder     `gorm:"foreignKey:SupplierID" json:"purchase_orders"`
	SupplierInvoices   []SupplierInvoice   `gorm:"foreignKey:SupplierID" json:"supplier_invoices"`
	SupplierPayments   []SupplierPayment   `gorm:"foreignKey:SupplierID" json:"supplier_payments"`
	GRNs               []GoodsReceivedNote `gorm:"foreignKey:SupplierID" json:"grns"`
	Contacts           []SupplierContact   `gorm:"foreignKey:SupplierID" json:"contacts"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

type EntityNote struct {
	Base
	EntityType string `gorm:"index" json:"entity_type"`
	EntityID   string `gorm:"index" json:"entity_id"`
	NoteType   string `json:"note_type"`
	Content    string `gorm:"type:text" json:"content"`
	CreatedBy  string `json:"created_by"`
}

func (EntityNote) TableName() string { return "entity_notes" }

type SupplierIssue struct {
	Base
	SupplierID  string     `gorm:"index" json:"supplier_id"`
	IssueType   string     `json:"issue_type"`
	Severity    string     `json:"severity"`
	Description string     `gorm:"type:text" json:"description"`
	Status      string     `gorm:"default:Open" json:"status"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	Resolution  string     `gorm:"type:text" json:"resolution"`
}

func (SupplierIssue) TableName() string { return "supplier_issues" }

type ProductMaster struct {
	Base
	ProductCode        string  `gorm:"uniqueIndex;size:50" json:"product_code"`
	ProductName        string  `gorm:"index" json:"product_name"`
	Description        string  `gorm:"type:text" json:"description"`
	Category           string  `gorm:"index" json:"category"`
	Brand              string  `gorm:"index" json:"brand"`
	Manufacturer       string  `json:"manufacturer"`
	ManufacturerPartNo string  `gorm:"index" json:"manufacturer_part_no"`
	Unit               string  `json:"unit"`
	HSNCode            string  `json:"hsn_code"`
	StandardCost       float64 `json:"standard_cost"`
	SellingPrice       float64 `json:"selling_price"`
	MinStockLevel      float64 `json:"min_stock_level"`
	MaxStockLevel      float64 `json:"max_stock_level"`
	ReorderPoint       float64 `json:"reorder_point"`
	LeadTimeDays       int     `json:"lead_time_days"`
	IsActive           bool    `gorm:"index;default:true" json:"is_active"`
	Notes              string  `json:"notes"`
}

func (ProductMaster) TableName() string { return "products" }

// =============================================================================
// SALES & RFQ
// =============================================================================

type Offer struct {
	Base
	OfferNumber     string      `gorm:"uniqueIndex" json:"offer_number"`
	RFQNumber       string      `gorm:"index" json:"rfq_number"`
	RFQID           string      `gorm:"index" json:"rfq_id"`
	CustomerID      string      `gorm:"index" json:"customer_id"`
	CustomerName    string      `json:"customer_name"`
	QuotationDate   time.Time   `json:"quotation_date"`
	ValidUntil      time.Time   `json:"valid_until"`
	PaymentTerms    string      `json:"payment_terms"`
	DeliveryTerms   string      `json:"delivery_terms"`
	TotalAmount     float64     `json:"total_amount"`
	DiscountPercent float64     `json:"discount_percent"`
	DiscountAmount  float64     `json:"discount_amount"`
	VATPercent      float64     `json:"vat_percent"`
	VATAmount       float64     `json:"vat_amount"`
	NetAmount       float64     `json:"net_amount"`
	Status          string      `gorm:"index;default:Draft" json:"status"`
	WonDate         *time.Time  `json:"won_date"`
	LostDate        *time.Time  `json:"lost_date"`
	LostReason      string      `json:"lost_reason"`
	Notes           string      `gorm:"type:text" json:"notes"`
	Items           []OfferItem `gorm:"foreignKey:OfferID" json:"items"`
}

func (Offer) TableName() string { return "offers" }

type OfferItem struct {
	Base
	OfferID      string  `gorm:"index" json:"offer_id"`
	ProductCode  string  `json:"product_code"`
	ProductName  string  `json:"product_name"`
	Description  string  `json:"description"`
	Quantity     float64 `json:"quantity"`
	Unit         string  `json:"unit"`
	UnitPrice    float64 `json:"unit_price"`
	Discount     float64 `json:"discount"`
	TotalPrice   float64 `json:"total_price"`
	DeliveryDays int     `json:"delivery_days"`
	Notes        string  `json:"notes"`
}

func (OfferItem) TableName() string { return "offer_items" }

type Opportunity struct {
	Base
	RFQNumber          string     `gorm:"uniqueIndex" json:"rfq_number"`
	CustomerID         string     `gorm:"index" json:"customer_id"`
	CustomerName       string     `json:"customer_name"`
	Description        string     `gorm:"type:text" json:"description"`
	EstimatedValue     float64    `json:"estimated_value"`
	EstimatedCloseDate time.Time  `json:"estimated_close_date"`
	Stage              string     `gorm:"index" json:"stage"`
	WinProbability     float64    `json:"win_probability"`
	Status             string     `gorm:"index;default:Active" json:"status"`
	OfferID            string     `gorm:"index" json:"offer_id"`
	OrderID            string     `gorm:"index" json:"order_id"`
	WonDate            *time.Time `json:"won_date"`
	LostDate           *time.Time `json:"lost_date"`
	LostReason         string     `json:"lost_reason"`
	ActualValue        float64    `json:"actual_value"`
	Source             string     `json:"source"`
	AssignedTo         string     `json:"assigned_to"`
	LastContactDate    *time.Time `json:"last_contact_date"`
	NextFollowupDate   *time.Time `json:"next_followup_date"`
	Notes              string     `gorm:"type:text" json:"notes"`
	TallyQuotationRef  string     `json:"tally_quotation_ref"`
	ImportSource       string     `json:"import_source"`
}

func (Opportunity) TableName() string { return "opportunities" }

type ARAgingBucket struct {
	Base
	CustomerID       string    `gorm:"index" json:"customer_id"`
	CustomerName     string    `json:"customer_name"`
	Current          float64   `json:"current"`
	Days1to30        float64   `json:"days_1_to_30"`
	Days31to60       float64   `json:"days_31_to_60"`
	Days61to90       float64   `json:"days_61_to_90"`
	DaysOver90       float64   `json:"days_over_90"`
	TotalOutstanding float64   `json:"total_outstanding"`
	CreditLimit      float64   `json:"credit_limit"`
	SnapshotDate     time.Time `json:"snapshot_date"`
}

func (ARAgingBucket) TableName() string { return "ar_aging" }

// =============================================================================
// OPERATIONS
// =============================================================================

type Order struct {
	Base
	OrderNumber         string         `gorm:"uniqueIndex" json:"order_number"`
	CustomerID          string         `gorm:"index" json:"customer_id"`
	CustomerName        string         `json:"customer_name"`
	OrderDate           time.Time      `json:"order_date"`
	DeliveryDate        time.Time      `json:"delivery_date"`
	PaymentTerms        string         `json:"payment_terms"`
	TotalAmount         float64        `json:"total_amount"`
	DiscountAmount      float64        `json:"discount_amount"`
	VATAmount           float64        `json:"vat_amount"`
	NetAmount           float64        `json:"net_amount"`
	Status              string         `gorm:"index;default:Confirmed" json:"status"`
	PercentageInvoiced  float64        `json:"percentage_invoiced"`
	PercentageDelivered float64        `json:"percentage_delivered"`
	PercentagePaid      float64        `json:"percentage_paid"`
	RFQID               string         `gorm:"index" json:"rfq_id"`
	RFQNumber           string         `json:"rfq_number"`
	OfferID             string         `gorm:"index" json:"offer_id"`
	Notes               string         `gorm:"type:text" json:"notes"`
	Items               []OrderItem    `gorm:"foreignKey:OrderID" json:"items"`
	Invoices            []Invoice      `gorm:"foreignKey:OrderID" json:"invoices"`
	DeliveryNotes       []DeliveryNote `gorm:"foreignKey:OrderID" json:"delivery_notes"`
}

func (Order) TableName() string { return "orders" }

type OrderItem struct {
	Base
	OrderID     string  `gorm:"index" json:"order_id"`
	ProductCode string  `json:"product_code"`
	ProductName string  `json:"product_name"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	Discount    float64 `json:"discount"`
	TotalPrice  float64 `json:"total_price"`
	Status      string  `json:"status"`
	Notes       string  `json:"notes"`
}

func (OrderItem) TableName() string { return "order_items" }

type Shipment struct {
	Base
	ShipmentNumber string     `gorm:"uniqueIndex" json:"shipment_number"`
	OrderID        string     `gorm:"index" json:"order_id"`
	Carrier        string     `json:"carrier"`
	TrackingNumber string     `json:"tracking_number"`
	ShipDate       time.Time  `json:"ship_date"`
	DeliveryDate   *time.Time `json:"delivery_date"`
	Status         string     `json:"status"`
}

func (Shipment) TableName() string { return "shipments" }

type PostSaleNote struct {
	Base
	OrderID        string     `gorm:"index" json:"order_id"`
	InvoiceID      string     `gorm:"index" json:"invoice_id"`
	NoteType       string     `json:"note_type"`
	Description    string     `gorm:"type:text" json:"description"`
	ReportedBy     string     `json:"reported_by"`
	Status         string     `json:"status"`
	ResolvedAt     *time.Time `json:"resolved_at"`
	ResolutionNote string     `json:"resolution_note"`
}

func (PostSaleNote) TableName() string { return "post_sale_notes" }

type DeliveryNote struct {
	Base
	DNNumber        string             `gorm:"uniqueIndex" json:"dn_number"`
	OrderID         string             `gorm:"index" json:"order_id"`
	OrderNumber     string             `json:"order_number"`
	CustomerID      string             `gorm:"index" json:"customer_id"`
	CustomerName    string             `json:"customer_name"`
	DeliveryDate    time.Time          `json:"delivery_date"`
	DeliveryAddress string             `json:"delivery_address"`
	ReceivedBy      string             `json:"received_by"`
	Status          string             `gorm:"default:Draft" json:"status"`
	Notes           string             `gorm:"type:text" json:"notes"`
	Items           []DeliveryNoteItem `gorm:"foreignKey:DNID" json:"items"`
}

func (DeliveryNote) TableName() string { return "delivery_notes" }

type DeliveryNoteItem struct {
	Base
	DNID         string  `gorm:"index;column:dn_id" json:"dn_id"`
	ProductCode  string  `json:"product_code"`
	ProductName  string  `json:"product_name"`
	Description  string  `json:"description"`
	OrderedQty   float64 `json:"ordered_qty"`
	DeliveredQty float64 `json:"delivered_qty"`
	Unit         string  `json:"unit"`
	Notes        string  `json:"notes"`
}

func (DeliveryNoteItem) TableName() string { return "delivery_note_items" }

// =============================================================================
// COSTING
// =============================================================================

type DBCostingSheet struct {
	Base
	SheetNumber        string                    `gorm:"uniqueIndex" json:"sheet_number"`
	RFQID              string                    `gorm:"index" json:"rfq_id"`
	RFQNumber          string                    `json:"rfq_number"`
	CustomerID         string                    `gorm:"index" json:"customer_id"`
	CustomerName       string                    `json:"customer_name"`
	PreparedBy         string                    `json:"prepared_by"`
	PreparedDate       time.Time                 `json:"prepared_date"`
	Version            int                       `json:"version"`
	TotalCost          float64                   `json:"total_cost"`
	TargetMargin       float64                   `json:"target_margin"`
	TargetSellingPrice float64                   `json:"target_selling_price"`
	Notes              string                    `gorm:"type:text" json:"notes"`
	Items              []DBCostingItem           `gorm:"foreignKey:CostingSheetID" json:"items"`
	AdditionalCosts    []DBCostingAdditionalCost `gorm:"foreignKey:CostingSheetID" json:"additional_costs"`
}

func (DBCostingSheet) TableName() string { return "costing_sheets" }

type DBCostingItem struct {
	Base
	CostingSheetID string  `gorm:"index" json:"costing_sheet_id"`
	ProductCode    string  `json:"product_code"`
	ProductName    string  `json:"product_name"`
	Quantity       float64 `json:"quantity"`
	Unit           string  `json:"unit"`
	UnitCost       float64 `json:"unit_cost"`
	TotalCost      float64 `json:"total_cost"`
}

func (DBCostingItem) TableName() string { return "costing_items" }

type DBCostingAdditionalCost struct {
	Base
	CostingSheetID string  `gorm:"index" json:"costing_sheet_id"`
	Description    string  `json:"description"`
	Amount         float64 `json:"amount"`
}

func (DBCostingAdditionalCost) TableName() string { return "costing_additional_costs" }

// =============================================================================
// FINANCE
// =============================================================================

type Invoice struct {
	Base
	InvoiceNumber      string          `gorm:"uniqueIndex" json:"invoice_number"`
	OrderID            string          `gorm:"uniqueIndex;index" json:"order_id"`
	OrderNumber        string          `json:"order_number"`
	CustomerID         string          `gorm:"index" json:"customer_id"`
	CustomerName       string          `json:"customer_name"`
	InvoiceDate        time.Time       `json:"invoice_date"`
	DueDate            time.Time       `json:"due_date"`
	TotalAmount        float64         `json:"total_amount"`
	DiscountAmount     float64         `json:"discount_amount"`
	VATAmount          float64         `json:"vat_amount"`
	NetAmount          float64         `json:"net_amount"`
	AmountPaid         float64         `json:"amount_paid"`
	AmountDue          float64         `json:"amount_due"`
	Status             string          `gorm:"index;default:Draft" json:"status"`
	PaymentStatus      string          `gorm:"index;default:Unpaid" json:"payment_status"`
	TallyVoucherRef    string          `gorm:"index" json:"tally_voucher_ref"`
	TallyVoucherType   string          `json:"tally_voucher_type"`
	TallyVoucherNumber string          `json:"tally_voucher_number"`
	ImportSource       string          `json:"import_source"`
	Notes              string          `gorm:"type:text" json:"notes"`
	Items              []DBInvoiceItem `gorm:"foreignKey:InvoiceID" json:"items"`
	Payments           []Payment       `gorm:"foreignKey:InvoiceID" json:"payments"`
}

func (Invoice) TableName() string { return "invoices" }

type DBInvoiceItem struct {
	Base
	InvoiceID   string  `gorm:"index" json:"invoice_id"`
	ProductCode string  `json:"product_code"`
	ProductName string  `json:"product_name"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	Discount    float64 `json:"discount"`
	TotalPrice  float64 `json:"total_price"`
	VATPercent  float64 `json:"vat_percent"`
	VATAmount   float64 `json:"vat_amount"`
	NetAmount   float64 `json:"net_amount"`
}

func (DBInvoiceItem) TableName() string { return "invoice_items" }

type ChartOfAccount struct {
	Base
	AccountCode   string  `gorm:"uniqueIndex;size:20" json:"account_code"`
	AccountNumber string  `gorm:"size:20" json:"account_number"`
	AccountName   string  `json:"account_name"`
	AccountType   string  `gorm:"index;size:50" json:"account_type"`
	Balance       float64 `json:"balance"`
	IsActive      bool    `gorm:"default:true" json:"is_active"`
	ParentAccount string  `gorm:"size:20" json:"parent_account"`
	AccountGroup  string  `gorm:"index;size:10" json:"account_group"`
}

func (ChartOfAccount) TableName() string { return "chart_of_accounts" }

type JournalEntry struct {
	Base
	JournalNumber string        `gorm:"uniqueIndex" json:"journal_number"`
	EntryDate     time.Time     `json:"entry_date"`
	Description   string        `json:"description"`
	TotalDebit    float64       `json:"total_debit"`
	TotalCredit   float64       `json:"total_credit"`
	Status        string        `gorm:"default:Draft" json:"status"`
	CreatedBy     string        `json:"created_by"`
	ApprovedBy    string        `json:"approved_by"`
	ApprovedAt    *time.Time    `json:"approved_at"`
	Lines         []JournalLine `gorm:"foreignKey:JournalEntryID" json:"lines"`
}

func (JournalEntry) TableName() string { return "journal_entries" }

type JournalLine struct {
	Base
	JournalEntryID string  `gorm:"index" json:"journal_entry_id"`
	AccountCode    string  `json:"account_code"`
	Description    string  `json:"description"`
	DebitAmount    float64 `json:"debit_amount"`
	CreditAmount   float64 `json:"credit_amount"`
}

func (JournalLine) TableName() string { return "journal_lines" }

type VATReturn struct {
	Base
	ReturnPeriod   string    `gorm:"uniqueIndex" json:"return_period"`
	StartDate      time.Time `json:"start_date"`
	EndDate        time.Time `json:"end_date"`
	TotalSales     float64   `json:"total_sales"`
	OutputVAT      float64   `json:"output_vat"`
	TotalPurchases float64   `json:"total_purchases"`
	InputVAT       float64   `json:"input_vat"`
	NetVAT         float64   `json:"net_vat"`
	Status         string    `gorm:"default:Draft" json:"status"`
}

func (VATReturn) TableName() string { return "vat_returns" }

type Payment struct {
	Base
	PaymentNumber   string    `gorm:"uniqueIndex" json:"payment_number"`
	InvoiceID       string    `gorm:"index" json:"invoice_id"`
	InvoiceNumber   string    `json:"invoice_number"`
	CustomerID      string    `gorm:"index" json:"customer_id"`
	CustomerName    string    `json:"customer_name"`
	PaymentDate     time.Time `json:"payment_date"`
	Amount          float64   `json:"amount"`
	PaymentMethod   string    `json:"payment_method"`
	ReferenceNo     string    `json:"reference_no"`
	Notes           string    `gorm:"type:text" json:"notes"`
	Status          string    `gorm:"default:Completed" json:"status"`
	TallyVoucherRef string    `gorm:"index" json:"tally_voucher_ref"`
	ImportSource    string    `json:"import_source"`
}

func (Payment) TableName() string { return "payments" }

// =============================================================================
// INVENTORY
// =============================================================================

type InventoryItem struct {
	Base
	ProductCode       string     `gorm:"uniqueIndex" json:"product_code"`
	ProductName       string     `json:"product_name"`
	WarehouseID       string     `gorm:"index" json:"warehouse_id"`
	QuantityOnHand    float64    `json:"quantity_on_hand"`
	QuantityReserved  float64    `json:"quantity_reserved"`
	QuantityAvailable float64    `json:"quantity_available"`
	QuantityOnOrder   float64    `json:"quantity_on_order"`
	ReorderPoint      float64    `json:"reorder_point"`
	LastRestockDate   *time.Time `json:"last_restock_date"`
	LastMovementDate  *time.Time `json:"last_movement_date"`
}

func (InventoryItem) TableName() string { return "inventory_items" }

type StockMovement struct {
	Base
	ProductCode   string    `gorm:"index" json:"product_code"`
	WarehouseID   string    `gorm:"index" json:"warehouse_id"`
	MovementType  string    `json:"movement_type"`
	Quantity      float64   `json:"quantity"`
	MovementDate  time.Time `json:"movement_date"`
	ReferenceType string    `json:"reference_type"`
	ReferenceID   string    `json:"reference_id"`
	Notes         string    `json:"notes"`
}

func (StockMovement) TableName() string { return "stock_movements" }

type StockAdjustment struct {
	Base
	ProductCode    string    `gorm:"index" json:"product_code"`
	WarehouseID    string    `gorm:"index" json:"warehouse_id"`
	AdjustmentType string    `json:"adjustment_type"`
	QuantityBefore float64   `json:"quantity_before"`
	QuantityAfter  float64   `json:"quantity_after"`
	Difference     float64   `json:"difference"`
	AdjustmentDate time.Time `json:"adjustment_date"`
	Reason         string    `json:"reason"`
	ApprovedBy     string    `json:"approved_by"`
}

func (StockAdjustment) TableName() string { return "stock_adjustments" }

type Warehouse struct {
	Base
	Code     string `gorm:"uniqueIndex;size:50" json:"code"`
	Name     string `json:"name"`
	IsActive bool   `gorm:"default:true" json:"is_active"`
}

func (Warehouse) TableName() string { return "warehouses" }

// =============================================================================
// INTELLIGENCE
// =============================================================================

type PredictionRecord struct {
	Base
	CustomerID            string    `gorm:"index" json:"customer_id"`
	CustomerName          string    `json:"customer_name"`
	PredictionDate        time.Time `json:"prediction_date"`
	PredictedPaymentDays  float64   `json:"predicted_payment_days"`
	ActualPaymentDays     float64   `json:"actual_payment_days"`
	PredictionError       float64   `json:"prediction_error"`
	PredictionAccuracyPct float64   `json:"prediction_accuracy_pct"`
}

func (PredictionRecord) TableName() string { return "prediction_records" }

type Setting struct {
	Base
	Key         string `gorm:"uniqueIndex" json:"key"`
	Value       string `gorm:"type:text" json:"value"`
	Description string `json:"description"`
}

func (Setting) TableName() string { return "settings" }

type UserSession struct {
	Base
	UserID       string    `gorm:"index" json:"user_id"`
	DeviceID     string    `gorm:"index" json:"device_id"`
	SessionToken string    `gorm:"uniqueIndex" json:"session_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	IsActive     bool      `json:"is_active"`
}

func (UserSession) TableName() string { return "user_sessions" }

type WinProbabilityPrediction struct {
	Base
	CustomerID     string  `gorm:"index" json:"customer_id"`
	OrderValue     float64 `json:"order_value"`
	WinProbability float64 `json:"win_probability"`
}

func (WinProbabilityPrediction) TableName() string { return "win_probability_predictions" }

type DiscountRecommendationRecord struct {
	Base
	CustomerID          string  `gorm:"index" json:"customer_id"`
	RecommendedDiscount float64 `json:"recommended_discount"`
}

func (DiscountRecommendationRecord) TableName() string { return "discount_recommendations" }

type CustomerSnapshot struct {
	Base
	CustomerID   string    `gorm:"index" json:"customer_id"`
	SnapshotDate time.Time `json:"snapshot_date"`
}

func (CustomerSnapshot) TableName() string { return "customer_snapshots" }

type ActualOutcome struct {
	Base
	CustomerID  string    `gorm:"index" json:"customer_id"`
	OutcomeDate time.Time `json:"outcome_date"`
}

func (ActualOutcome) TableName() string { return "actual_outcomes" }

type CostingHistory struct {
	Base
	CostingSheetID string `gorm:"index" json:"costing_sheet_id"`
	Version        int    `json:"version"`
}

func (CostingHistory) TableName() string { return "costing_history" }

type GradeChange struct {
	Base
	CustomerID string    `gorm:"index" json:"customer_id"`
	OldGrade   string    `json:"old_grade"`
	NewGrade   string    `json:"new_grade"`
	ChangeDate time.Time `json:"change_date"`
	Reason     string    `json:"reason"`
}

func (GradeChange) TableName() string { return "grade_changes" }

type PaymentPredictionAccuracy struct {
	Base
	ModelVersion string    `json:"model_version"`
	TestDate     time.Time `json:"test_date"`
	RMSE         float64   `json:"rmse"`
	MAE          float64   `json:"mae"`
	AccuracyPct  float64   `json:"accuracy_pct"`
}

func (PaymentPredictionAccuracy) TableName() string { return "payment_prediction_accuracy" }

// =============================================================================
// ADMIN
// =============================================================================

type Role struct {
	Base
	Name        string `gorm:"uniqueIndex;size:50" json:"name"`
	DisplayName string `gorm:"size:255" json:"display_name"`
	Description string `gorm:"type:varchar(1000)" json:"description"`
	Permissions string `gorm:"type:text" json:"permissions"`
	IsActive    bool   `gorm:"index;default:true" json:"is_active"`
	IsSystem    bool   `gorm:"default:false" json:"is_system"`
}

func (Role) TableName() string { return "roles" }

type User struct {
	Base
	Username       string     `gorm:"uniqueIndex;size:50" json:"username"`
	Email          string     `gorm:"uniqueIndex" json:"email"`
	PasswordHash   string     `json:"-"`
	FullName       string     `json:"full_name"`
	Phone          string     `json:"phone"`
	RoleID         string     `gorm:"index" json:"role_id"`
	IsActive       bool       `gorm:"index;default:true" json:"is_active"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	FailedAttempts int        `json:"failed_attempts"`
	LockedUntil    *time.Time `json:"locked_until"`
}

func (User) TableName() string { return "users" }

type Device struct {
	Base
	MachineID  string     `gorm:"uniqueIndex" json:"machine_id"`
	DeviceName string     `json:"device_name"`
	Status     string     `gorm:"index;default:pending" json:"status"`
	ApprovedAt *time.Time `json:"approved_at"`
	ApprovedBy string     `json:"approved_by"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

func (Device) TableName() string { return "devices" }

type DeviceUser struct {
	Base
	DeviceID     string     `gorm:"index" json:"device_id"`
	Username     string     `gorm:"index" json:"username"`
	FullName     string     `json:"full_name"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	RoleID       string     `gorm:"index" json:"role_id"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at"`
}

func (DeviceUser) TableName() string { return "device_users" }

type Alert struct {
	Base
	AlertType  string     `gorm:"index" json:"alert_type"`
	Severity   string     `json:"severity"`
	Message    string     `gorm:"type:text" json:"message"`
	EntityType string     `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	IsRead     bool       `gorm:"default:false" json:"is_read"`
	ReadAt     *time.Time `json:"read_at"`
}

func (Alert) TableName() string { return "alerts" }

type FollowUpTask struct {
	Base
	TaskType    string     `json:"task_type"`
	EntityType  string     `json:"entity_type"`
	EntityID    string     `json:"entity_id"`
	AssignedTo  string     `json:"assigned_to"`
	DueDate     time.Time  `json:"due_date"`
	Priority    string     `json:"priority"`
	Status      string     `gorm:"default:Pending" json:"status"`
	Description string     `gorm:"type:text" json:"description"`
	CompletedAt *time.Time `json:"completed_at"`
	CompletedBy string     `json:"completed_by"`
}

func (FollowUpTask) TableName() string { return "followup_tasks" }

type AuditLog struct {
	Base
	UserID     string `gorm:"index" json:"user_id"`
	Action     string `json:"action"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
}

func (AuditLog) TableName() string { return "audit_logs" }

type FileWatchEvent struct {
	Base
	FilePath  string `json:"file_path"`
	EventType string `json:"event_type"`
	Status    string `json:"status"`
}

func (FileWatchEvent) TableName() string { return "file_watch_events" }

type SyncStatus struct {
	Base
	LastSyncAt    *time.Time `json:"last_sync_at"`
	Status        string     `json:"status"`
	ErrorMessage  string     `gorm:"type:text" json:"error_message"`
	RecordsSynced int        `json:"records_synced"`
}

func (SyncStatus) TableName() string { return "sync_status" }

type Job struct {
	Base
	JobType      string     `gorm:"index" json:"job_type"`
	Status       string     `gorm:"index;default:Pending" json:"status"`
	ScheduledAt  time.Time  `json:"scheduled_at"`
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	ErrorMessage string     `gorm:"type:text" json:"error_message"`
	RetryCount   int        `json:"retry_count"`
	MaxRetries   int        `json:"max_retries"`
	Payload      string     `gorm:"type:text" json:"payload"`
}

func (Job) TableName() string { return "jobs" }

// =============================================================================
// PROCUREMENT
// =============================================================================

type PurchaseOrder struct {
	Base
	PONumber         string              `gorm:"uniqueIndex" json:"po_number"`
	OrderID          string              `gorm:"index" json:"order_id"`
	OrderNumber      string              `json:"order_number"`
	SupplierID       string              `gorm:"index" json:"supplier_id"`
	SupplierName     string              `json:"supplier_name"`
	PODate           time.Time           `json:"po_date"`
	ExpectedDelivery time.Time           `json:"expected_delivery"`
	PaymentTerms     string              `json:"payment_terms"`
	TotalAmount      float64             `json:"total_amount"`
	VATAmount        float64             `json:"vat_amount"`
	NetAmount        float64             `json:"net_amount"`
	Status           string              `gorm:"index;default:Draft" json:"status"`
	ApprovedBy       string              `json:"approved_by"`
	ApprovedAt       *time.Time          `json:"approved_at"`
	Notes            string              `gorm:"type:text" json:"notes"`
	Items            []PurchaseOrderItem `gorm:"foreignKey:PurchaseOrderID" json:"items"`
	SupplierInvoices []SupplierInvoice   `gorm:"foreignKey:PurchaseOrderID" json:"supplier_invoices"`
	GRNs             []GoodsReceivedNote `gorm:"foreignKey:PurchaseOrderID" json:"grns"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

type PurchaseOrderItem struct {
	Base
	PurchaseOrderID string  `gorm:"index" json:"purchase_order_id"`
	ProductCode     string  `json:"product_code"`
	ProductName     string  `json:"product_name"`
	Description     string  `json:"description"`
	Quantity        float64 `json:"quantity"`
	Unit            string  `json:"unit"`
	UnitPrice       float64 `json:"unit_price"`
	TotalPrice      float64 `json:"total_price"`
	Notes           string  `json:"notes"`
}

func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

type GoodsReceivedNote struct {
	Base
	GRNNumber        string    `gorm:"uniqueIndex" json:"grn_number"`
	PurchaseOrderID  string    `gorm:"index" json:"purchase_order_id"`
	PONumber         string    `json:"po_number"`
	SupplierID       string    `gorm:"index" json:"supplier_id"`
	SupplierName     string    `json:"supplier_name"`
	ReceivedDate     time.Time `json:"received_date"`
	ReceivedBy       string    `json:"received_by"`
	InspectedBy      string    `json:"inspected_by"`
	Status           string    `gorm:"default:Draft" json:"status"`
	Notes            string    `gorm:"type:text" json:"notes"`
	QuantityOrdered  float64   `json:"quantity_ordered"`
	QuantityReceived float64   `json:"quantity_received"`
	QuantityAccepted float64   `json:"quantity_accepted"`
	QuantityRejected float64   `json:"quantity_rejected"`
	Items            []GRNItem `gorm:"foreignKey:GRNID" json:"items"`
}

func (GoodsReceivedNote) TableName() string { return "goods_received_notes" }

type GRNItem struct {
	Base
	GRNID           string  `gorm:"index" json:"grn_id"`
	ProductCode     string  `json:"product_code"`
	ProductName     string  `json:"product_name"`
	OrderedQty      float64 `json:"ordered_qty"`
	ReceivedQty     float64 `json:"received_qty"`
	AcceptedQty     float64 `json:"accepted_qty"`
	RejectedQty     float64 `json:"rejected_qty"`
	Unit            string  `json:"unit"`
	UnitPrice       float64 `json:"unit_price"`
	TotalValue      float64 `json:"total_value"`
	QCStatus        string  `gorm:"default:Pending" json:"qc_status"`
	RejectionReason string  `json:"rejection_reason"`
	Notes           string  `json:"notes"`
}

func (GRNItem) TableName() string { return "grn_items" }

type SupplierInvoice struct {
	Base
	InvoiceNumber   string                `gorm:"uniqueIndex" json:"invoice_number"`
	PurchaseOrderID string                `gorm:"index" json:"purchase_order_id"`
	PONumber        string                `json:"po_number"`
	GRNID           string                `gorm:"index" json:"grn_id"`
	GRNNumber       string                `json:"grn_number"`
	SupplierID      string                `gorm:"index" json:"supplier_id"`
	SupplierName    string                `json:"supplier_name"`
	InvoiceDate     time.Time             `json:"invoice_date"`
	DueDate         time.Time             `json:"due_date"`
	TotalAmount     float64               `json:"total_amount"`
	VATAmount       float64               `json:"vat_amount"`
	NetAmount       float64               `json:"net_amount"`
	AmountPaid      float64               `json:"amount_paid"`
	AmountDue       float64               `json:"amount_due"`
	Status          string                `gorm:"index;default:Draft" json:"status"`
	PaymentStatus   string                `gorm:"index;default:Unpaid" json:"payment_status"`
	ApprovalStatus  string                `gorm:"default:Pending" json:"approval_status"`
	ApprovedBy      string                `json:"approved_by"`
	ApprovedAt      *time.Time            `json:"approved_at"`
	ThreeWayMatched bool                  `gorm:"default:false" json:"three_way_matched"`
	MatchedAt       *time.Time            `json:"matched_at"`
	TallyVoucherRef string                `gorm:"index" json:"tally_voucher_ref"`
	ImportSource    string                `json:"import_source"`
	Notes           string                `gorm:"type:text" json:"notes"`
	Items           []SupplierInvoiceItem `gorm:"foreignKey:SupplierInvoiceID" json:"items"`
	Payments        []SupplierPayment     `gorm:"foreignKey:SupplierInvoiceID" json:"payments"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

type SupplierInvoiceItem struct {
	Base
	SupplierInvoiceID string  `gorm:"index" json:"supplier_invoice_id"`
	ProductCode       string  `json:"product_code"`
	ProductName       string  `json:"product_name"`
	Description       string  `json:"description"`
	Quantity          float64 `json:"quantity"`
	Unit              string  `json:"unit"`
	UnitPrice         float64 `json:"unit_price"`
	TotalPrice        float64 `json:"total_price"`
}

func (SupplierInvoiceItem) TableName() string { return "supplier_invoice_items" }

// =============================================================================
// CHAT
// =============================================================================

type Conversation struct {
	Base
	Title         string        `json:"title"`
	UserID        string        `gorm:"index" json:"user_id"`
	LastMessageAt time.Time     `json:"last_message_at"`
	IsArchived    bool          `gorm:"default:false" json:"is_archived"`
	Messages      []ChatMessage `gorm:"foreignKey:ConversationID" json:"messages"`
}

func (Conversation) TableName() string { return "conversations" }

type ChatMessage struct {
	Base
	ConversationID string    `gorm:"index" json:"conversation_id"`
	Role           string    `json:"role"`
	Content        string    `gorm:"type:text" json:"content"`
	Timestamp      time.Time `json:"timestamp"`
}

func (ChatMessage) TableName() string { return "chat_messages" }

type SupplierPayment struct {
	Base
	PaymentNumber     string    `gorm:"uniqueIndex" json:"payment_number"`
	SupplierInvoiceID string    `gorm:"index" json:"supplier_invoice_id"`
	InvoiceNumber     string    `json:"invoice_number"`
	SupplierID        string    `gorm:"index" json:"supplier_id"`
	SupplierName      string    `json:"supplier_name"`
	PaymentDate       time.Time `json:"payment_date"`
	Amount            float64   `json:"amount"`
	PaymentMethod     string    `json:"payment_method"`
	ReferenceNo       string    `json:"reference_no"`
	Notes             string    `gorm:"type:text" json:"notes"`
	Status            string    `gorm:"default:Completed" json:"status"`
	TallyVoucherRef   string    `gorm:"index" json:"tally_voucher_ref"`
	ImportSource      string    `json:"import_source"`
}

func (SupplierPayment) TableName() string { return "supplier_payments" }

// =============================================================================
// FOLLOW-UPS
// =============================================================================

type OfferFollowUp struct {
	Base
	OfferID          string     `gorm:"index" json:"offer_id"`
	OfferNumber      string     `json:"offer_number"`
	FollowUpDate     time.Time  `json:"followup_date"`
	FollowUpType     string     `json:"followup_type"`
	Notes            string     `gorm:"type:text" json:"notes"`
	Status           string     `gorm:"default:Pending" json:"status"`
	CompletedAt      *time.Time `json:"completed_at"`
	CompletedBy      string     `json:"completed_by"`
	NextFollowUpDate *time.Time `json:"next_followup_date"`
}

func (OfferFollowUp) TableName() string { return "offer_follow_ups" }

// =============================================================================
// SYNC & IMPORT
// =============================================================================

type SyncRecord struct {
	Base
	SyncTableName string    `gorm:"index;column:table_name" json:"table_name"`
	RecordID      string    `gorm:"index" json:"record_id"`
	SyncDirection string    `json:"sync_direction"`
	SyncStatus    string    `json:"sync_status"`
	LastSyncAt    time.Time `json:"last_sync_at"`
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`
}

func (SyncRecord) TableName() string { return "sync_records" }

type TallyInvoiceImport struct {
	Base
	VoucherType   string    `json:"voucher_type"`
	VoucherNumber string    `gorm:"index" json:"voucher_number"`
	Date          time.Time `json:"date"`
	PartyName     string    `json:"party_name"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	InvoiceID     string    `gorm:"index" json:"invoice_id"`
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`
}

func (TallyInvoiceImport) TableName() string { return "tally_invoice_imports" }

type TallyPurchaseImport struct {
	Base
	VoucherType   string    `json:"voucher_type"`
	VoucherNumber string    `gorm:"index" json:"voucher_number"`
	Date          time.Time `json:"date"`
	PartyName     string    `json:"party_name"`
	Amount        float64   `json:"amount"`
	Status        string    `json:"status"`
	PurchaseID    string    `gorm:"index" json:"purchase_id"`
	ErrorMessage  string    `gorm:"type:text" json:"error_message"`
}

func (TallyPurchaseImport) TableName() string { return "tally_purchase_imports" }

// =============================================================================
// ACCOUNTING
// =============================================================================

type AccountMapping struct {
	Base
	ExternalSystem string `json:"external_system"`
	ExternalCode   string `json:"external_code"`
	AccountCode    string `json:"account_code"`
	Description    string `json:"description"`
}

func (AccountMapping) TableName() string { return "account_mappings" }

type FiscalPeriod struct {
	Base
	PeriodName string     `gorm:"uniqueIndex" json:"period_name"`
	StartDate  time.Time  `json:"start_date"`
	EndDate    time.Time  `json:"end_date"`
	Status     string     `gorm:"default:Open" json:"status"`
	ClosedBy   string     `json:"closed_by"`
	ClosedAt   *time.Time `json:"closed_at"`
}

func (FiscalPeriod) TableName() string { return "fiscal_periods" }

type BankAccount struct {
	Base
	AccountNumber string          `gorm:"uniqueIndex" json:"account_number"`
	BankName      string          `json:"bank_name"`
	AccountName   string          `json:"account_name"`
	Currency      string          `gorm:"default:BHD" json:"currency"`
	Balance       float64         `json:"balance"`
	IsActive      bool            `gorm:"default:true" json:"is_active"`
	Statements    []BankStatement `gorm:"foreignKey:BankAccountID" json:"statements"`
}

func (BankAccount) TableName() string { return "bank_accounts" }

type BankStatement struct {
	Base
	BankAccountID  string              `gorm:"index" json:"bank_account_id"`
	StatementDate  time.Time           `json:"statement_date"`
	OpeningBalance float64             `json:"opening_balance"`
	ClosingBalance float64             `json:"closing_balance"`
	TotalDebits    float64             `json:"total_debits"`
	TotalCredits   float64             `json:"total_credits"`
	Status         string              `gorm:"default:Unreconciled" json:"status"`
	Lines          []BankStatementLine `gorm:"foreignKey:BankStatementID" json:"lines"`
}

func (BankStatement) TableName() string { return "bank_statements" }

type BankStatementLine struct {
	Base
	BankStatementID string    `gorm:"index" json:"bank_statement_id"`
	TransactionDate time.Time `json:"transaction_date"`
	Description     string    `json:"description"`
	ReferenceNo     string    `json:"reference_no"`
	DebitAmount     float64   `json:"debit_amount"`
	CreditAmount    float64   `json:"credit_amount"`
	Balance         float64   `json:"balance"`
	IsReconciled    bool      `gorm:"default:false" json:"is_reconciled"`
}

func (BankStatementLine) TableName() string { return "bank_statement_lines" }

// QuickCapture represents FocusCard tasks (My Tasks)
type QuickCapture struct {
	Base
	TaskText   string `gorm:"type:text" json:"task_text"`
	IsComplete bool   `gorm:"default:false" json:"is_complete"`
	Priority   string `gorm:"default:Medium" json:"priority"`
}

func (QuickCapture) TableName() string { return "quick_captures" }
