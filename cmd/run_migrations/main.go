package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// All the models that need tables

type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
	CreatedBy string
}

// RFQ Data
type RFQData struct {
	Base
	RFQNumber      string    `gorm:"uniqueIndex;size:50"`
	CustomerID     string    `gorm:"index;size:36"`
	CustomerName   string    `gorm:"size:255"`
	Subject        string    `gorm:"size:500"`
	ReceivedDate   time.Time `gorm:"index"`
	DueDate        time.Time
	Status         string `gorm:"index;size:50;default:'Pending'"`
	Priority       string `gorm:"size:20;default:'Normal'"`
	AssignedTo     string `gorm:"size:100"`
	SourceFile     string `gorm:"size:500"`
	Notes          string `gorm:"type:text"`
	RequirementsJSON string `gorm:"type:text"`
}

func (RFQData) TableName() string { return "rfq_data" }

// Role
type Role struct {
	Base
	Name        string `gorm:"uniqueIndex;size:50"`
	Description string `gorm:"size:255"`
	Permissions string `gorm:"type:text"` // JSON array of permissions
	IsSystem    bool   `gorm:"default:false"`
}

func (Role) TableName() string { return "roles" }

// User
type User struct {
	Base
	Username     string `gorm:"uniqueIndex;size:100"`
	Email        string `gorm:"uniqueIndex;size:255"`
	PasswordHash string `gorm:"size:255"`
	RoleID       string `gorm:"index;size:36"`
	IsActive     bool   `gorm:"default:true"`
	LastLoginAt  *time.Time
}

func (User) TableName() string { return "users" }

// AuditLog
type AuditLog struct {
	Base
	UserID     string `gorm:"index;size:36"`
	Action     string `gorm:"index;size:100"`
	EntityType string `gorm:"index;size:100"`
	EntityID   string `gorm:"index;size:36"`
	OldValue   string `gorm:"type:text"`
	NewValue   string `gorm:"type:text"`
	IPAddress  string `gorm:"size:50"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// UserSession
type UserSession struct {
	Base
	UserID       string    `gorm:"index;size:36"`
	Token        string    `gorm:"uniqueIndex;size:255"`
	ExpiresAt    time.Time `gorm:"index"`
	IPAddress    string    `gorm:"size:50"`
	UserAgent    string    `gorm:"size:255"`
}

func (UserSession) TableName() string { return "user_sessions" }

// PurchaseOrder
type PurchaseOrder struct {
	Base
	PONumber     string    `gorm:"uniqueIndex;size:50"`
	SupplierID   string    `gorm:"index;size:36"`
	SupplierName string    `gorm:"size:255"`
	OrderID      string    `gorm:"index;size:36"`
	OrderNumber  string    `gorm:"size:50"`
	IssueDate    time.Time `gorm:"index"`
	DeliveryDate time.Time
	TotalBHD     float64
	Status       string `gorm:"index;size:50;default:'Draft'"`
	Notes        string `gorm:"type:text"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

// PurchaseOrderItem
type PurchaseOrderItem struct {
	Base
	POID        string  `gorm:"index;size:36"`
	ProductID   string  `gorm:"index;size:36"`
	Description string  `gorm:"size:500"`
	Quantity    float64
	UnitPrice   float64
	TotalPrice  float64
}

func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

// GoodsReceivedNote
type GoodsReceivedNote struct {
	Base
	GRNNumber      string    `gorm:"uniqueIndex;size:50"`
	POID           string    `gorm:"index;size:36"`
	PONumber       string    `gorm:"size:50"`
	SupplierID     string    `gorm:"index;size:36"`
	SupplierName   string    `gorm:"size:255"`
	ReceivedDate   time.Time `gorm:"index"`
	ReceivedBy     string    `gorm:"size:100"`
	WarehouseID    string    `gorm:"size:36"`
	Status         string    `gorm:"index;size:50;default:'Pending'"`
	Notes          string    `gorm:"type:text"`
}

func (GoodsReceivedNote) TableName() string { return "goods_received_notes" }

// GRNItem
type GRNItem struct {
	Base
	GRNID           string  `gorm:"index;size:36"`
	POItemID        string  `gorm:"index;size:36"`
	ProductID       string  `gorm:"index;size:36"`
	Description     string  `gorm:"size:500"`
	OrderedQty      float64
	ReceivedQty     float64
	RejectedQty     float64
	AcceptedQty     float64
	QCStatus        string `gorm:"size:50;default:'Pending'"`
	QCNotes         string `gorm:"type:text"`
}

func (GRNItem) TableName() string { return "grn_items" }

// DeliveryNote
type DeliveryNote struct {
	Base
	DNNumber      string    `gorm:"uniqueIndex;size:50"`
	OrderID       string    `gorm:"index;size:36"`
	OrderNumber   string    `gorm:"size:50"`
	CustomerID    string    `gorm:"index;size:36"`
	CustomerName  string    `gorm:"size:255"`
	DeliveryDate  time.Time `gorm:"index"`
	DeliveredBy   string    `gorm:"size:100"`
	ReceivedBy    string    `gorm:"size:100"`
	Status        string    `gorm:"index;size:50;default:'Pending'"`
	Notes         string    `gorm:"type:text"`
}

func (DeliveryNote) TableName() string { return "delivery_notes" }

// DeliveryNoteItem
type DeliveryNoteItem struct {
	Base
	DNID           string  `gorm:"index;size:36"`
	OrderItemID    string  `gorm:"index;size:36"`
	ProductID      string  `gorm:"index;size:36"`
	Description    string  `gorm:"size:500"`
	OrderedQty     float64
	DeliveredQty   float64
}

func (DeliveryNoteItem) TableName() string { return "delivery_note_items" }

// Conversation
type Conversation struct {
	Base
	Title        string `gorm:"size:255"`
	UserID       string `gorm:"index;size:36"`
	LastActivity time.Time
	MessageCount int
}

func (Conversation) TableName() string { return "conversations" }

// ChatMessage
type ChatMessage struct {
	Base
	ConversationID string `gorm:"index;size:36"`
	Role           string `gorm:"size:20"` // user, assistant
	Content        string `gorm:"type:text"`
	Timestamp      time.Time
}

func (ChatMessage) TableName() string { return "chat_messages" }

// SupplierPayment
type SupplierPayment struct {
	Base
	PaymentNumber   string    `gorm:"uniqueIndex;size:50"`
	SupplierID      string    `gorm:"index;size:36"`
	SupplierName    string    `gorm:"size:255"`
	InvoiceID       string    `gorm:"index;size:36"`
	InvoiceNumber   string    `gorm:"size:50"`
	PaymentDate     time.Time `gorm:"index"`
	AmountBHD       float64
	PaymentMethod   string    `gorm:"size:50"`
	Reference       string    `gorm:"size:100"`
	Notes           string    `gorm:"type:text"`
}

func (SupplierPayment) TableName() string { return "supplier_payments" }

// OfferFollowUp
type OfferFollowUp struct {
	Base
	OfferID      string    `gorm:"index;size:36"`
	OfferNumber  string    `gorm:"size:50"`
	ScheduledFor time.Time `gorm:"index"`
	FollowUpType string    `gorm:"size:50"` // call, email, meeting
	Notes        string    `gorm:"type:text"`
	Completed    bool      `gorm:"default:false"`
	CompletedAt  *time.Time
	CompletedBy  string    `gorm:"size:100"`
	Outcome      string    `gorm:"type:text"`
}

func (OfferFollowUp) TableName() string { return "offer_follow_ups" }

// SyncRecord
type SyncRecord struct {
	Base
	SyncTableName string    `gorm:"column:table_name;index;size:100"`
	RecordID      string    `gorm:"index;size:36"`
	Operation     string    `gorm:"size:20"` // insert, update, delete
	SyncStatus    string    `gorm:"index;size:20;default:'pending'"`
	LastSyncAt    *time.Time
	SyncError     string    `gorm:"type:text"`
	RetryCount    int       `gorm:"default:0"`
}

func (SyncRecord) TableName() string { return "sync_records" }

// Alert
type Alert struct {
	Base
	AlertType    string    `gorm:"index;size:50"`
	Severity     string    `gorm:"index;size:20"` // critical, warning, info
	Title        string    `gorm:"size:255"`
	Message      string    `gorm:"type:text"`
	EntityType   string    `gorm:"index;size:50"`
	EntityID     string    `gorm:"index;size:36"`
	IsRead       bool      `gorm:"default:false"`
	IsResolved   bool      `gorm:"default:false"`
	ResolvedAt   *time.Time
	ResolvedBy   string    `gorm:"size:100"`
}

func (Alert) TableName() string { return "alerts" }

// Setting
type Setting struct {
	Base
	Key   string `gorm:"uniqueIndex;size:100"`
	Value string `gorm:"type:text"`
	Type  string `gorm:"size:20"` // string, number, boolean, json
}

func (Setting) TableName() string { return "settings" }

// EntityNote
type EntityNote struct {
	Base
	EntityType string `gorm:"index;size:50"` // customer, supplier
	EntityID   string `gorm:"index;size:36"`
	NoteType   string `gorm:"index;size:50"` // general, payment, quality, delivery
	Content    string `gorm:"type:text"`
	Author     string `gorm:"size:100"`
}

func (EntityNote) TableName() string { return "entity_notes" }

// SupplierIssue
type SupplierIssue struct {
	Base
	SupplierID   string    `gorm:"index;size:36"`
	SupplierName string    `gorm:"size:255"`
	IssueType    string    `gorm:"index;size:50"` // quality, delivery, communication, pricing
	Severity     string    `gorm:"size:20"`       // high, medium, low
	Description  string    `gorm:"type:text"`
	Status       string    `gorm:"index;size:20;default:'open'"` // open, resolved
	ReportedBy   string    `gorm:"size:100"`
	ResolvedAt   *time.Time
	Resolution   string    `gorm:"type:text"`
}

func (SupplierIssue) TableName() string { return "supplier_issues" }

// InboxDocument
type InboxDocument struct {
	Base
	FileName     string    `gorm:"size:255"`
	FilePath     string    `gorm:"size:500"`
	DocumentType string    `gorm:"index;size:50"` // rfq, po, invoice, quote
	Status       string    `gorm:"index;size:50;default:'pending'"`
	ProcessedAt  *time.Time
	ExtractedData string   `gorm:"type:text"` // JSON
}

func (InboxDocument) TableName() string { return "inbox_documents" }

// FollowUpTask
type FollowUpTask struct {
	Base
	EntityType  string    `gorm:"index;size:50"` // offer, order, invoice
	EntityID    string    `gorm:"index;size:36"`
	TaskType    string    `gorm:"size:50"` // follow_up, reminder, action
	Description string    `gorm:"type:text"`
	DueDate     time.Time `gorm:"index"`
	Completed   bool      `gorm:"default:false"`
	CompletedAt *time.Time
	AssignedTo  string    `gorm:"size:100"`
}

func (FollowUpTask) TableName() string { return "follow_up_tasks" }

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	dbFile := "ph_holdings.db"
	buildDBFile := "build/bin/data/ph_holdings.db"

	fmt.Println("🔧 Running database migrations...")

	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Run all migrations
	err = db.AutoMigrate(
		&RFQData{},
		&Role{},
		&User{},
		&AuditLog{},
		&UserSession{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&DeliveryNote{},
		&DeliveryNoteItem{},
		&Conversation{},
		&ChatMessage{},
		&SupplierPayment{},
		&OfferFollowUp{},
		&SyncRecord{},
		&Alert{},
		&Setting{},
		&EntityNote{},
		&SupplierIssue{},
		&InboxDocument{},
		&FollowUpTask{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	fmt.Println("✅ Migrations complete!")

	// Create default roles
	roles := []Role{
		{Base: Base{ID: "role-admin"}, Name: "admin", Description: "Full system access", Permissions: `["*"]`, IsSystem: true},
		{Base: Base{ID: "role-manager"}, Name: "manager", Description: "Manager access", Permissions: `["read:*","write:*","approve:*"]`, IsSystem: true},
		{Base: Base{ID: "role-staff"}, Name: "staff", Description: "Staff access", Permissions: `["read:*","write:own"]`, IsSystem: true},
	}

	for _, role := range roles {
		var existing Role
		if db.Where("name = ?", role.Name).First(&existing).Error != nil {
			role.CreatedAt = time.Now()
			role.UpdatedAt = time.Now()
			db.Create(&role)
			fmt.Printf("   Created role: %s\n", role.Name)
		}
	}

	// List all tables
	var tables []string
	db.Raw("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name").Scan(&tables)
	fmt.Printf("\n📊 Database tables (%d):\n", len(tables))
	for _, t := range tables {
		fmt.Printf("   • %s\n", t)
	}

	// Copy to build folder
	fmt.Println("\n📦 Copying to build folder...")
	sqlDB, _ := db.DB()
	sqlDB.Close()

	input, err := os.ReadFile(dbFile)
	if err != nil {
		log.Printf("Warning: %v", err)
	} else {
		os.WriteFile(buildDBFile, input, 0644)
		fmt.Println("✅ Database deployed to build/bin/data/")
	}

	fmt.Println("\n✨ Done! Database is ready.")
}
