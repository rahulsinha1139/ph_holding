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

// Import all models from main package - we'll define them inline for standalone migration
// This mirrors the models in database.go

type BaseModel struct {
	ID        string `gorm:"primaryKey;type:varchar(36)"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type CustomerMaster struct {
	BaseModel
	Code              string  `gorm:"uniqueIndex;size:50"`
	Name              string  `gorm:"size:255;not null"`
	TradingName       string  `gorm:"size:255"`
	Industry          string  `gorm:"size:100"`
	CustomerType      string  `gorm:"size:50"`
	Grade             string  `gorm:"size:10"`
	Email             string  `gorm:"size:255"`
	Phone             string  `gorm:"size:50"`
	Mobile            string  `gorm:"size:50"`
	Website           string  `gorm:"size:255"`
	Address           string  `gorm:"type:text"`
	City              string  `gorm:"size:100"`
	Country           string  `gorm:"size:100"`
	PostalCode        string  `gorm:"size:20"`
	TaxID             string  `gorm:"size:50"`
	CreditLimit       float64 `gorm:"type:decimal(15,3)"`
	PaymentTerms      string  `gorm:"size:50"`
	Currency          string  `gorm:"size:10;default:BHD"`
	IsActive          bool    `gorm:"default:true"`
	Notes             string  `gorm:"type:text"`
	Tags              string  `gorm:"type:text"`
	TotalRevenue      float64 `gorm:"type:decimal(15,3)"`
	TotalOrders       int
	LastOrderDate     *time.Time
	AccountReceivable float64 `gorm:"type:decimal(15,3)"`
}

type CustomerContact struct {
	BaseModel
	CustomerID  string `gorm:"index;size:36"`
	Name        string `gorm:"size:255"`
	Title       string `gorm:"size:100"`
	Email       string `gorm:"size:255"`
	Phone       string `gorm:"size:50"`
	Mobile      string `gorm:"size:50"`
	IsPrimary   bool   `gorm:"default:false"`
	Department  string `gorm:"size:100"`
	Notes       string `gorm:"type:text"`
}

type SupplierMaster struct {
	BaseModel
	Code            string  `gorm:"uniqueIndex;size:50"`
	Name            string  `gorm:"size:255;not null"`
	TradingName     string  `gorm:"size:255"`
	Category        string  `gorm:"size:100"`
	SupplierType    string  `gorm:"size:50"`
	Rating          int     `gorm:"default:3"`
	Email           string  `gorm:"size:255"`
	Phone           string  `gorm:"size:50"`
	Website         string  `gorm:"size:255"`
	Address         string  `gorm:"type:text"`
	City            string  `gorm:"size:100"`
	Country         string  `gorm:"size:100"`
	PostalCode      string  `gorm:"size:20"`
	TaxID           string  `gorm:"size:50"`
	PaymentTerms    string  `gorm:"size:50"`
	Currency        string  `gorm:"size:10;default:USD"`
	LeadTimeDays    int     `gorm:"default:14"`
	MinOrderValue   float64 `gorm:"type:decimal(15,3)"`
	IsActive        bool    `gorm:"default:true"`
	IsPreferred     bool    `gorm:"default:false"`
	Notes           string  `gorm:"type:text"`
	Tags            string  `gorm:"type:text"`
	TotalPurchases  float64 `gorm:"type:decimal(15,3)"`
	TotalPOs        int
	LastPODate      *time.Time
	AccountsPayable float64 `gorm:"type:decimal(15,3)"`
	BankName        string  `gorm:"size:255"`
	BankAccount     string  `gorm:"size:100"`
	BankSWIFT       string  `gorm:"size:20"`
	BankIBAN        string  `gorm:"size:50"`
}

type SupplierContact struct {
	BaseModel
	SupplierID string `gorm:"index;size:36"`
	Name       string `gorm:"size:255"`
	Title      string `gorm:"size:100"`
	Email      string `gorm:"size:255"`
	Phone      string `gorm:"size:50"`
	Mobile     string `gorm:"size:50"`
	IsPrimary  bool   `gorm:"default:false"`
	Department string `gorm:"size:100"`
	Notes      string `gorm:"type:text"`
}

type ProductMaster struct {
	BaseModel
	SKU              string  `gorm:"uniqueIndex;size:100"`
	Name             string  `gorm:"size:255;not null"`
	Description      string  `gorm:"type:text"`
	Category         string  `gorm:"size:100"`
	SubCategory      string  `gorm:"size:100"`
	Brand            string  `gorm:"size:100"`
	Manufacturer     string  `gorm:"size:255"`
	ManufacturerPart string  `gorm:"size:100"`
	UnitOfMeasure    string  `gorm:"size:20"`
	UnitPrice        float64 `gorm:"type:decimal(15,3)"`
	CostPrice        float64 `gorm:"type:decimal(15,3)"`
	Currency         string  `gorm:"size:10;default:BHD"`
	TaxRate          float64 `gorm:"type:decimal(5,2);default:10"`
	IsActive         bool    `gorm:"default:true"`
	IsTaxable        bool    `gorm:"default:true"`
	MinStock         int     `gorm:"default:0"`
	MaxStock         int     `gorm:"default:0"`
	ReorderPoint     int     `gorm:"default:0"`
	LeadTimeDays     int     `gorm:"default:14"`
	Weight           float64 `gorm:"type:decimal(10,3)"`
	Dimensions       string  `gorm:"size:100"`
	Notes            string  `gorm:"type:text"`
	Tags             string  `gorm:"type:text"`
	ImageURL         string  `gorm:"size:500"`
	PreferredSupplierID string `gorm:"size:36"`
}

type EntityNote struct {
	BaseModel
	EntityType string `gorm:"size:50;index"`
	EntityID   string `gorm:"size:36;index"`
	NoteType   string `gorm:"size:50"`
	Content    string `gorm:"type:text"`
	CreatedBy  string `gorm:"size:100"`
	IsPinned   bool   `gorm:"default:false"`
}

type SupplierIssue struct {
	BaseModel
	SupplierID   string     `gorm:"index;size:36"`
	IssueType    string     `gorm:"size:50"`
	Severity     string     `gorm:"size:20"`
	Description  string     `gorm:"type:text"`
	Status       string     `gorm:"size:20;default:Open"`
	Resolution   string     `gorm:"type:text"`
	ResolvedAt   *time.Time
	ResolvedBy   string     `gorm:"size:100"`
	ReportedBy   string     `gorm:"size:100"`
}

type RFQ struct {
	BaseModel
	RFQNumber       string     `gorm:"uniqueIndex;size:50"`
	CustomerID      string     `gorm:"index;size:36"`
	CustomerName    string     `gorm:"size:255"`
	ContactPerson   string     `gorm:"size:255"`
	ContactEmail    string     `gorm:"size:255"`
	Subject         string     `gorm:"size:500"`
	Description     string     `gorm:"type:text"`
	Status          string     `gorm:"size:50;default:Pending"`
	Priority        string     `gorm:"size:20;default:Medium"`
	ReceivedDate    time.Time
	DueDate         *time.Time
	QuotedDate      *time.Time
	ExpiryDate      *time.Time
	TotalValue      float64    `gorm:"type:decimal(15,3)"`
	Currency        string     `gorm:"size:10;default:BHD"`
	WinProbability  float64    `gorm:"type:decimal(5,2)"`
	Notes           string     `gorm:"type:text"`
	Tags            string     `gorm:"type:text"`
	AssignedTo      string     `gorm:"size:100"`
	Source          string     `gorm:"size:100"`
	SourceReference string     `gorm:"size:255"`
	FolderPath      string     `gorm:"size:500"`
}

type RFQItem struct {
	BaseModel
	RFQID            string  `gorm:"index;size:36"`
	LineNumber       int
	ProductID        string  `gorm:"size:36"`
	SKU              string  `gorm:"size:100"`
	Description      string  `gorm:"type:text"`
	Quantity         float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure    string  `gorm:"size:20"`
	RequestedDate    *time.Time
	Notes            string  `gorm:"type:text"`
}

type Offer struct {
	BaseModel
	OfferNumber    string     `gorm:"uniqueIndex;size:50"`
	RFQID          string     `gorm:"index;size:36"`
	CustomerID     string     `gorm:"index;size:36"`
	CustomerName   string     `gorm:"size:255"`
	Subject        string     `gorm:"size:500"`
	Status         string     `gorm:"size:50;default:Draft"`
	ValidUntil     *time.Time
	SubTotal       float64    `gorm:"type:decimal(15,3)"`
	DiscountAmount float64    `gorm:"type:decimal(15,3)"`
	TaxAmount      float64    `gorm:"type:decimal(15,3)"`
	TotalAmount    float64    `gorm:"type:decimal(15,3)"`
	Currency       string     `gorm:"size:10;default:BHD"`
	Terms          string     `gorm:"type:text"`
	Notes          string     `gorm:"type:text"`
	InternalNotes  string     `gorm:"type:text"`
	SentDate       *time.Time
	WonDate        *time.Time
	LostDate       *time.Time
	LostReason     string     `gorm:"type:text"`
	CreatedBy      string     `gorm:"size:100"`
	ApprovedBy     string     `gorm:"size:100"`
	ApprovedAt     *time.Time
	Version        int        `gorm:"default:1"`
}

type OfferItem struct {
	BaseModel
	OfferID       string  `gorm:"index;size:36"`
	LineNumber    int
	ProductID     string  `gorm:"size:36"`
	SKU           string  `gorm:"size:100"`
	Description   string  `gorm:"type:text"`
	Quantity      float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure string  `gorm:"size:20"`
	UnitCost      float64 `gorm:"type:decimal(15,3)"`
	UnitPrice     float64 `gorm:"type:decimal(15,3)"`
	DiscountPct   float64 `gorm:"type:decimal(5,2)"`
	TaxRate       float64 `gorm:"type:decimal(5,2);default:10"`
	LineTotal     float64 `gorm:"type:decimal(15,3)"`
	Margin        float64 `gorm:"type:decimal(5,2)"`
	Notes         string  `gorm:"type:text"`
	SupplierID    string  `gorm:"size:36"`
	SupplierName  string  `gorm:"size:255"`
	LeadTimeDays  int
}

type Order struct {
	BaseModel
	OrderNumber     string     `gorm:"uniqueIndex;size:50"`
	OfferID         string     `gorm:"index;size:36"`
	RFQID           string     `gorm:"index;size:36"`
	CustomerID      string     `gorm:"index;size:36"`
	CustomerName    string     `gorm:"size:255"`
	CustomerPO      string     `gorm:"size:100"`
	Status          string     `gorm:"size:50;default:Confirmed"`
	OrderDate       time.Time
	RequiredDate    *time.Time
	ShippedDate     *time.Time
	DeliveredDate   *time.Time
	SubTotal        float64    `gorm:"type:decimal(15,3)"`
	DiscountAmount  float64    `gorm:"type:decimal(15,3)"`
	TaxAmount       float64    `gorm:"type:decimal(15,3)"`
	ShippingAmount  float64    `gorm:"type:decimal(15,3)"`
	TotalAmount     float64    `gorm:"type:decimal(15,3)"`
	PaidAmount      float64    `gorm:"type:decimal(15,3)"`
	Currency        string     `gorm:"size:10;default:BHD"`
	PaymentTerms    string     `gorm:"size:50"`
	PaymentStatus   string     `gorm:"size:50;default:Unpaid"`
	ShippingAddress string     `gorm:"type:text"`
	BillingAddress  string     `gorm:"type:text"`
	Notes           string     `gorm:"type:text"`
	InternalNotes   string     `gorm:"type:text"`
	CreatedBy       string     `gorm:"size:100"`
	ApprovedBy      string     `gorm:"size:100"`
	ApprovedAt      *time.Time
}

type OrderItem struct {
	BaseModel
	OrderID          string  `gorm:"index;size:36"`
	LineNumber       int
	ProductID        string  `gorm:"size:36"`
	SKU              string  `gorm:"size:100"`
	Description      string  `gorm:"type:text"`
	Quantity         float64 `gorm:"type:decimal(15,3)"`
	DeliveredQty     float64 `gorm:"type:decimal(15,3)"`
	InvoicedQty      float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure    string  `gorm:"size:20"`
	UnitPrice        float64 `gorm:"type:decimal(15,3)"`
	DiscountPct      float64 `gorm:"type:decimal(5,2)"`
	TaxRate          float64 `gorm:"type:decimal(5,2);default:10"`
	LineTotal        float64 `gorm:"type:decimal(15,3)"`
	Notes            string  `gorm:"type:text"`
	Status           string  `gorm:"size:50;default:Pending"`
}

type Invoice struct {
	BaseModel
	InvoiceNumber   string     `gorm:"uniqueIndex;size:50"`
	OrderID         string     `gorm:"index;size:36"`
	CustomerID      string     `gorm:"index;size:36"`
	CustomerName    string     `gorm:"size:255"`
	Status          string     `gorm:"size:50;default:Draft"`
	InvoiceDate     time.Time
	DueDate         *time.Time
	SubTotal        float64    `gorm:"type:decimal(15,3)"`
	DiscountAmount  float64    `gorm:"type:decimal(15,3)"`
	TaxAmount       float64    `gorm:"type:decimal(15,3)"`
	TotalAmount     float64    `gorm:"type:decimal(15,3)"`
	PaidAmount      float64    `gorm:"type:decimal(15,3)"`
	BalanceDue      float64    `gorm:"type:decimal(15,3)"`
	Currency        string     `gorm:"size:10;default:BHD"`
	PaymentTerms    string     `gorm:"size:50"`
	Notes           string     `gorm:"type:text"`
	InternalNotes   string     `gorm:"type:text"`
	BillingAddress  string     `gorm:"type:text"`
	SentDate        *time.Time
	PaidDate        *time.Time
	CreatedBy       string     `gorm:"size:100"`
}

type DBInvoiceItem struct {
	BaseModel
	InvoiceID     string  `gorm:"index;size:36"`
	OrderItemID   string  `gorm:"size:36"`
	LineNumber    int
	ProductID     string  `gorm:"size:36"`
	SKU           string  `gorm:"size:100"`
	Description   string  `gorm:"type:text"`
	Quantity      float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure string  `gorm:"size:20"`
	UnitPrice     float64 `gorm:"type:decimal(15,3)"`
	DiscountPct   float64 `gorm:"type:decimal(5,2)"`
	TaxRate       float64 `gorm:"type:decimal(5,2);default:10"`
	LineTotal     float64 `gorm:"type:decimal(15,3)"`
	Notes         string  `gorm:"type:text"`
}

func (DBInvoiceItem) TableName() string {
	return "invoice_items"
}

type Payment struct {
	BaseModel
	PaymentNumber  string     `gorm:"uniqueIndex;size:50"`
	InvoiceID      string     `gorm:"index;size:36"`
	CustomerID     string     `gorm:"index;size:36"`
	CustomerName   string     `gorm:"size:255"`
	Amount         float64    `gorm:"type:decimal(15,3)"`
	Currency       string     `gorm:"size:10;default:BHD"`
	PaymentDate    time.Time
	PaymentMethod  string     `gorm:"size:50"`
	Reference      string     `gorm:"size:255"`
	BankAccount    string     `gorm:"size:100"`
	Status         string     `gorm:"size:50;default:Completed"`
	Notes          string     `gorm:"type:text"`
	ReceivedBy     string     `gorm:"size:100"`
	ReconciliationStatus string `gorm:"size:50;default:Pending"`
	ReconciledAt   *time.Time
	ReconciledBy   string     `gorm:"size:100"`
}

type PurchaseOrder struct {
	BaseModel
	PONumber        string     `gorm:"uniqueIndex;size:50"`
	SupplierID      string     `gorm:"index;size:36"`
	SupplierName    string     `gorm:"size:255"`
	OrderID         string     `gorm:"index;size:36"`
	Status          string     `gorm:"size:50;default:Draft"`
	OrderDate       time.Time
	ExpectedDate    *time.Time
	ReceivedDate    *time.Time
	SubTotal        float64    `gorm:"type:decimal(15,3)"`
	TaxAmount       float64    `gorm:"type:decimal(15,3)"`
	ShippingAmount  float64    `gorm:"type:decimal(15,3)"`
	TotalAmount     float64    `gorm:"type:decimal(15,3)"`
	PaidAmount      float64    `gorm:"type:decimal(15,3)"`
	Currency        string     `gorm:"size:10;default:USD"`
	PaymentTerms    string     `gorm:"size:50"`
	ShippingAddress string     `gorm:"type:text"`
	Notes           string     `gorm:"type:text"`
	InternalNotes   string     `gorm:"type:text"`
	CreatedBy       string     `gorm:"size:100"`
	ApprovedBy      string     `gorm:"size:100"`
	ApprovedAt      *time.Time
}

type PurchaseOrderItem struct {
	BaseModel
	PurchaseOrderID string  `gorm:"index;size:36"`
	LineNumber      int
	ProductID       string  `gorm:"size:36"`
	SKU             string  `gorm:"size:100"`
	Description     string  `gorm:"type:text"`
	Quantity        float64 `gorm:"type:decimal(15,3)"`
	ReceivedQty     float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure   string  `gorm:"size:20"`
	UnitPrice       float64 `gorm:"type:decimal(15,3)"`
	TaxRate         float64 `gorm:"type:decimal(5,2)"`
	LineTotal       float64 `gorm:"type:decimal(15,3)"`
	Notes           string  `gorm:"type:text"`
	Status          string  `gorm:"size:50;default:Pending"`
}

type GoodsReceivedNote struct {
	BaseModel
	GRNNumber       string     `gorm:"uniqueIndex;size:50"`
	PurchaseOrderID string     `gorm:"index;size:36"`
	SupplierID      string     `gorm:"index;size:36"`
	SupplierName    string     `gorm:"size:255"`
	Status          string     `gorm:"size:50;default:Draft"`
	ReceivedDate    time.Time
	ReceivedBy      string     `gorm:"size:100"`
	Notes           string     `gorm:"type:text"`
	InternalNotes   string     `gorm:"type:text"`
	QCStatus        string     `gorm:"size:50"`
	QCNotes         string     `gorm:"type:text"`
	QCCompletedAt   *time.Time
	QCCompletedBy   string     `gorm:"size:100"`
}

type GRNItem struct {
	BaseModel
	GRNID           string  `gorm:"index;size:36"`
	POItemID        string  `gorm:"size:36"`
	LineNumber      int
	ProductID       string  `gorm:"size:36"`
	SKU             string  `gorm:"size:100"`
	Description     string  `gorm:"type:text"`
	OrderedQty      float64 `gorm:"type:decimal(15,3)"`
	ReceivedQty     float64 `gorm:"type:decimal(15,3)"`
	AcceptedQty     float64 `gorm:"type:decimal(15,3)"`
	RejectedQty     float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure   string  `gorm:"size:20"`
	Notes           string  `gorm:"type:text"`
	QCStatus        string  `gorm:"size:50"`
	QCNotes         string  `gorm:"type:text"`
}

type SupplierInvoice struct {
	BaseModel
	InvoiceNumber     string     `gorm:"uniqueIndex;size:50"`
	SupplierInvNumber string     `gorm:"size:100"`
	PurchaseOrderID   string     `gorm:"index;size:36"`
	GRNID             string     `gorm:"index;size:36"`
	SupplierID        string     `gorm:"index;size:36"`
	SupplierName      string     `gorm:"size:255"`
	Status            string     `gorm:"size:50;default:Draft"`
	InvoiceDate       time.Time
	DueDate           *time.Time
	SubTotal          float64    `gorm:"type:decimal(15,3)"`
	TaxAmount         float64    `gorm:"type:decimal(15,3)"`
	TotalAmount       float64    `gorm:"type:decimal(15,3)"`
	PaidAmount        float64    `gorm:"type:decimal(15,3)"`
	BalanceDue        float64    `gorm:"type:decimal(15,3)"`
	Currency          string     `gorm:"size:10;default:USD"`
	PaymentTerms      string     `gorm:"size:50"`
	Notes             string     `gorm:"type:text"`
	InternalNotes     string     `gorm:"type:text"`
	ApprovalStatus    string     `gorm:"size:50;default:Pending"`
	ApprovedBy        string     `gorm:"size:100"`
	ApprovedAt        *time.Time
	PaidDate          *time.Time
	CreatedBy         string     `gorm:"size:100"`
	ThreeWayMatch     string     `gorm:"size:50"`
	MatchNotes        string     `gorm:"type:text"`
}

type SupplierInvoiceItem struct {
	BaseModel
	SupplierInvoiceID string  `gorm:"index;size:36"`
	POItemID          string  `gorm:"size:36"`
	GRNItemID         string  `gorm:"size:36"`
	LineNumber        int
	ProductID         string  `gorm:"size:36"`
	SKU               string  `gorm:"size:100"`
	Description       string  `gorm:"type:text"`
	Quantity          float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure     string  `gorm:"size:20"`
	UnitPrice         float64 `gorm:"type:decimal(15,3)"`
	TaxRate           float64 `gorm:"type:decimal(5,2)"`
	LineTotal         float64 `gorm:"type:decimal(15,3)"`
	Notes             string  `gorm:"type:text"`
}

type DeliveryNote struct {
	BaseModel
	DNNumber       string     `gorm:"uniqueIndex;size:50"`
	OrderID        string     `gorm:"index;size:36"`
	CustomerID     string     `gorm:"index;size:36"`
	CustomerName   string     `gorm:"size:255"`
	Status         string     `gorm:"size:50;default:Draft"`
	DeliveryDate   time.Time
	DeliveredBy    string     `gorm:"size:100"`
	ReceivedBy     string     `gorm:"size:100"`
	Address        string     `gorm:"type:text"`
	Notes          string     `gorm:"type:text"`
	InternalNotes  string     `gorm:"type:text"`
	SignatureURL   string     `gorm:"size:500"`
	CreatedBy      string     `gorm:"size:100"`
}

type DeliveryNoteItem struct {
	BaseModel
	DeliveryNoteID string  `gorm:"index;size:36"`
	OrderItemID    string  `gorm:"size:36"`
	LineNumber     int
	ProductID      string  `gorm:"size:36"`
	SKU            string  `gorm:"size:100"`
	Description    string  `gorm:"type:text"`
	OrderedQty     float64 `gorm:"type:decimal(15,3)"`
	DeliveredQty   float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure  string  `gorm:"size:20"`
	Notes          string  `gorm:"type:text"`
}

type Conversation struct {
	BaseModel
	Title     string `gorm:"size:255"`
	UserID    string `gorm:"index;size:36"`
	Context   string `gorm:"type:text"`
	IsArchived bool  `gorm:"default:false"`
}

type ChatMessage struct {
	BaseModel
	ConversationID string `gorm:"index;size:36"`
	Role           string `gorm:"size:20"`
	Content        string `gorm:"type:text"`
	TokensUsed     int
	Model          string `gorm:"size:100"`
}

type SupplierPayment struct {
	BaseModel
	PaymentNumber     string     `gorm:"uniqueIndex;size:50"`
	SupplierInvoiceID string     `gorm:"index;size:36"`
	SupplierID        string     `gorm:"index;size:36"`
	SupplierName      string     `gorm:"size:255"`
	Amount            float64    `gorm:"type:decimal(15,3)"`
	Currency          string     `gorm:"size:10;default:USD"`
	PaymentDate       time.Time
	PaymentMethod     string     `gorm:"size:50"`
	Reference         string     `gorm:"size:255"`
	BankAccount       string     `gorm:"size:100"`
	Status            string     `gorm:"size:50;default:Completed"`
	Notes             string     `gorm:"type:text"`
	PaidBy            string     `gorm:"size:100"`
	ApprovedBy        string     `gorm:"size:100"`
	ApprovedAt        *time.Time
}

type OfferFollowUp struct {
	BaseModel
	OfferID       string     `gorm:"index;size:36"`
	ScheduledDate time.Time
	FollowUpType  string     `gorm:"size:50"`
	Notes         string     `gorm:"type:text"`
	Status        string     `gorm:"size:50;default:Pending"`
	CompletedAt   *time.Time
	CompletedBy   string     `gorm:"size:100"`
	Outcome       string     `gorm:"type:text"`
}

type SyncRecord struct {
	BaseModel
	SyncTable     string    `gorm:"size:100;index"`
	RecordID      string    `gorm:"size:36;index"`
	SyncedAt      time.Time `gorm:"index"`
	Direction     string    `gorm:"size:10"`
	ConflictState string    `gorm:"size:20"`
	ErrorMessage  string    `gorm:"type:text"`
}

type TallyInvoiceImport struct {
	BaseModel
	TallyVoucherNo   string     `gorm:"size:100;index"`
	TallyDate        *time.Time
	CustomerName     string     `gorm:"size:255"`
	MatchedCustomerID string    `gorm:"size:36"`
	Amount           float64    `gorm:"type:decimal(15,3)"`
	ImportStatus     string     `gorm:"size:50"`
	ImportNotes      string     `gorm:"type:text"`
	SourceFile       string     `gorm:"size:500"`
	ImportedAt       time.Time
}

type TallyPurchaseImport struct {
	BaseModel
	TallyVoucherNo    string     `gorm:"size:100;index"`
	TallyDate         *time.Time
	SupplierName      string     `gorm:"size:255"`
	MatchedSupplierID string     `gorm:"size:36"`
	Amount            float64    `gorm:"type:decimal(15,3)"`
	ImportStatus      string     `gorm:"size:50"`
	ImportNotes       string     `gorm:"type:text"`
	SourceFile        string     `gorm:"size:500"`
	ImportedAt        time.Time
}

type Role struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;size:50"`
	Description string `gorm:"size:255"`
	Permissions string `gorm:"type:text"`
	IsSystem    bool   `gorm:"default:false"`
}

type User struct {
	BaseModel
	Email       string `gorm:"uniqueIndex;size:255"`
	Name        string `gorm:"size:255"`
	RoleID      string `gorm:"index;size:36"`
	IsActive    bool   `gorm:"default:true"`
	LastLoginAt *time.Time
	Preferences string `gorm:"type:text"`
}

type AuditLog struct {
	BaseModel
	UserID      string    `gorm:"index;size:36"`
	UserEmail   string    `gorm:"size:255"`
	Action      string    `gorm:"size:100"`
	EntityType  string    `gorm:"size:50"`
	EntityID    string    `gorm:"size:36"`
	OldValues   string    `gorm:"type:text"`
	NewValues   string    `gorm:"type:text"`
	IPAddress   string    `gorm:"size:50"`
	UserAgent   string    `gorm:"size:500"`
	Timestamp   time.Time `gorm:"index"`
}

type Setting struct {
	BaseModel
	Key         string `gorm:"uniqueIndex;size:100"`
	Value       string `gorm:"type:text"`
	Description string `gorm:"size:500"`
	Category    string `gorm:"size:50;index"`
	IsSecret    bool   `gorm:"default:false"`
}

type QuickCapture struct {
	BaseModel
	UserID      string     `gorm:"index;size:36"`
	Content     string     `gorm:"type:text"`
	CaptureType string     `gorm:"size:50"`
	Priority    string     `gorm:"size:20;default:medium"`
	Status      string     `gorm:"size:20;default:pending"`
	DueDate     *time.Time
	CompletedAt *time.Time
	Tags        string     `gorm:"type:text"`
	Source      string     `gorm:"size:100"`
}

type CostingSheet struct {
	BaseModel
	CostingNumber    string     `gorm:"uniqueIndex;size:50"`
	RFQID            string     `gorm:"index;size:36"`
	CustomerID       string     `gorm:"index;size:36"`
	CustomerName     string     `gorm:"size:255"`
	Status           string     `gorm:"size:50;default:Draft"`
	FolderPath       string     `gorm:"size:500"`
	TotalCost        float64    `gorm:"type:decimal(15,3)"`
	TotalPrice       float64    `gorm:"type:decimal(15,3)"`
	OverallMargin    float64    `gorm:"type:decimal(5,2)"`
	Currency         string     `gorm:"size:10;default:BHD"`
	Notes            string     `gorm:"type:text"`
	CreatedBy        string     `gorm:"size:100"`
	ApprovedBy       string     `gorm:"size:100"`
	ApprovedAt       *time.Time
}

type CostingSheetItem struct {
	BaseModel
	CostingSheetID  string  `gorm:"index;size:36"`
	LineNumber      int
	SupplierID      string  `gorm:"size:36"`
	SupplierName    string  `gorm:"size:255"`
	ProductID       string  `gorm:"size:36"`
	SKU             string  `gorm:"size:100"`
	Description     string  `gorm:"type:text"`
	Quantity        float64 `gorm:"type:decimal(15,3)"`
	UnitOfMeasure   string  `gorm:"size:20"`
	UnitCost        float64 `gorm:"type:decimal(15,3)"`
	UnitPrice       float64 `gorm:"type:decimal(15,3)"`
	Margin          float64 `gorm:"type:decimal(5,2)"`
	TotalCost       float64 `gorm:"type:decimal(15,3)"`
	TotalPrice      float64 `gorm:"type:decimal(15,3)"`
	LeadTimeDays    int
	Notes           string  `gorm:"type:text"`
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgresql://ph_admin:Indcy4LP1o5uDJT2oFQ4KzyXKcFllSKu@dpg-d5rmv2hr0fns73e8sq1g-a.singapore-postgres.render.com/ph_holdings"
	}

	fmt.Println("🚀 Migrating schema to Render.com PostgreSQL...")
	fmt.Println("═══════════════════════════════════════════════════════════")

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("❌ Connection failed: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	fmt.Println("\n📋 Running AutoMigrate for all models...")
	start := time.Now()

	err = db.AutoMigrate(
		&CustomerMaster{},
		&CustomerContact{},
		&SupplierMaster{},
		&SupplierContact{},
		&ProductMaster{},
		&EntityNote{},
		&SupplierIssue{},
		&RFQ{},
		&RFQItem{},
		&Offer{},
		&OfferItem{},
		&Order{},
		&OrderItem{},
		&Invoice{},
		&DBInvoiceItem{},
		&Payment{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&DeliveryNote{},
		&DeliveryNoteItem{},
		&Conversation{},
		&ChatMessage{},
		&SupplierPayment{},
		&OfferFollowUp{},
		&SyncRecord{},
		&TallyInvoiceImport{},
		&TallyPurchaseImport{},
		&Role{},
		&User{},
		&AuditLog{},
		&Setting{},
		&QuickCapture{},
		&CostingSheet{},
		&CostingSheetItem{},
	)

	if err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("\n✅ Migration completed in %v\n", elapsed)

	// Count tables
	var tables []string
	db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename").Scan(&tables)
	fmt.Printf("\n📊 Created %d tables:\n", len(tables))
	for _, t := range tables {
		fmt.Printf("   ✓ %s\n", t)
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("🎉 Render.com database is ready for production!")
}
