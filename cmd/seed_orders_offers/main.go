package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
	CreatedBy string
}

// Offer model
type Offer struct {
	Base
	OfferNumber     string    `gorm:"uniqueIndex;size:50"`
	RevisionNumber  int       `gorm:"index"`
	RFQID           uint      `gorm:"index"`
	CustomerID      string    `gorm:"index;size:36"`
	CustomerName    string    `gorm:"index;size:255"`
	QuotationDate   time.Time `gorm:"index"`
	ValidityDate    time.Time
	TotalValueBHD   float64
	EstimatedMargin float64
	Stage           string `gorm:"index;size:50"` // RFQ, Quoted, Won, Lost
	PaymentTerms    string `gorm:"type:text"`
	DeliveryTerms   string `gorm:"type:text"`
	DeliveryWeeks   string `gorm:"size:50"`
	CountryOfOrigin string `gorm:"size:100"`
	IssuedBy        string `gorm:"size:100"`
}

func (Offer) TableName() string { return "offers" }

// OfferItem model
type OfferItem struct {
	Base
	OfferID     string  `gorm:"index;size:36"`
	LineNumber  int
	ProductID   string  `gorm:"index;size:36"`
	ProductCode string  `gorm:"size:100"`
	Description string  `gorm:"type:text"`
	Quantity    float64
	UnitPrice   float64
}

func (OfferItem) TableName() string { return "offer_items" }

// Order model
type Order struct {
	Base
	OrderNumber      string    `gorm:"uniqueIndex;size:50"`
	CustomerPONumber string    `gorm:"index;size:100"`
	CustomerID       string    `gorm:"index;size:36"`
	CustomerName     string    `gorm:"index;size:255"`
	OrderDate        time.Time `gorm:"index"`
	RequiredDate     time.Time
	TotalValueBHD    float64
	GrandTotalBHD    float64
	Status           string `gorm:"index;size:50"`
	PaymentTerms     string
	DeliveryTerms    string
}

func (Order) TableName() string { return "orders" }

// OrderItem model
type OrderItem struct {
	Base
	OrderID          string  `gorm:"index;size:36"`
	LineNumber       int
	ProductID        string  `gorm:"index;size:36"`
	ProductCode      string  `gorm:"size:100"`
	Description      string  `gorm:"type:text"`
	Quantity         float64
	UnitPrice        float64
	QuantityShipped  float64
	QuantityInvoiced float64
}

func (OrderItem) TableName() string { return "order_items" }

// Opportunity model (for reading existing data)
type Opportunity struct {
	Base
	FolderNumber string `gorm:"uniqueIndex;size:50"`
	CustomerID   string `gorm:"index;size:36"`
	CustomerName string `gorm:"index;size:255"`
	Salesperson  string `gorm:"index;size:100"`
	OfferDate    time.Time
	RevenueBHD   float64
	CostBHD      float64
	ProfitBHD    float64
	Stage        string `gorm:"index;size:50"`
}

func (Opportunity) TableName() string { return "opportunities" }

// Invoice model (for reading existing data)
type Invoice struct {
	Base
	InvoiceNumber  string    `gorm:"uniqueIndex;size:50"`
	CustomerID     string    `gorm:"index;size:36"`
	CustomerName   string    `gorm:"size:255"`
	InvoiceDate    time.Time `gorm:"index"`
	GrandTotalBHD  float64
	SubtotalBHD    float64
	Status         string `gorm:"index;size:50"`
}

func (Invoice) TableName() string { return "invoices" }

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	dbFile := "ph_holdings.db"

	fmt.Println("🚀 Seeding Orders and Offers...")

	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Ensure tables exist
	db.AutoMigrate(&Offer{}, &OfferItem{}, &Order{}, &OrderItem{})

	// Create Offers from Opportunities
	offersCreated := createOffersFromOpportunities(db)
	fmt.Printf("✅ Created %d offers from opportunities\n", offersCreated)

	// Create Orders from Invoices (since invoices represent completed sales)
	ordersCreated := createOrdersFromInvoices(db)
	fmt.Printf("✅ Created %d orders from invoices\n", ordersCreated)

	// Deploy to build
	fmt.Println("\n📦 Deploying updated database...")
	if err := deployDatabase(dbFile); err != nil {
		fmt.Printf("⚠️ Warning: %v\n", err)
	}

	fmt.Println("✨ Done!")
}

func createOffersFromOpportunities(db *gorm.DB) int {
	var opportunities []Opportunity
	db.Find(&opportunities)

	created := 0
	for i, opp := range opportunities {
		// Generate offer number from folder number
		offerNumber := fmt.Sprintf("QUO-%s", opp.FolderNumber)
		if opp.FolderNumber == "" {
			offerNumber = fmt.Sprintf("QUO-2025-%03d", i+1)
		}

		// Check if exists
		var existing Offer
		if db.Where("offer_number = ?", offerNumber).First(&existing).Error == nil {
			continue
		}

		// Determine stage based on whether there's revenue
		stage := "Quoted"
		if opp.RevenueBHD > 0 {
			stage = "Won"
		}

		offer := Offer{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OfferNumber:     offerNumber,
			RevisionNumber:  0,
			CustomerID:      opp.CustomerID,
			CustomerName:    opp.CustomerName,
			QuotationDate:   opp.OfferDate,
			ValidityDate:    opp.OfferDate.AddDate(0, 1, 0), // Valid for 1 month
			TotalValueBHD:   opp.RevenueBHD,
			EstimatedMargin: opp.ProfitBHD / opp.RevenueBHD * 100,
			Stage:           stage,
			PaymentTerms:    "30 days from Date of Delivery",
			DeliveryTerms:   "DAP Bahrain",
			DeliveryWeeks:   "4-6 weeks",
			CountryOfOrigin: "Germany / USA",
			IssuedBy:        opp.Salesperson,
		}

		if opp.RevenueBHD == 0 {
			offer.TotalValueBHD = 5000 // Default value for quotes without amount
			offer.EstimatedMargin = 25
		}

		if err := db.Create(&offer).Error; err == nil {
			created++
		}
	}

	return created
}

func createOrdersFromInvoices(db *gorm.DB) int {
	var invoices []Invoice
	db.Find(&invoices)

	created := 0
	for _, inv := range invoices {
		// Generate order number from invoice number
		orderNumber := fmt.Sprintf("ORD-%s", inv.InvoiceNumber)

		// Check if exists
		var existing Order
		if db.Where("order_number = ?", orderNumber).First(&existing).Error == nil {
			continue
		}

		order := Order{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrderNumber:      orderNumber,
			CustomerPONumber: inv.InvoiceNumber,
			CustomerID:       inv.CustomerID,
			CustomerName:     inv.CustomerName,
			OrderDate:        inv.InvoiceDate.AddDate(0, 0, -7), // Order 1 week before invoice
			RequiredDate:     inv.InvoiceDate,
			TotalValueBHD:    inv.SubtotalBHD,
			GrandTotalBHD:    inv.GrandTotalBHD,
			Status:           "Delivered", // Since invoice exists, order is complete
			PaymentTerms:     "Net 30",
			DeliveryTerms:    "DAP Bahrain",
		}

		if err := db.Create(&order).Error; err == nil {
			created++
		}
	}

	return created
}

func deployDatabase(dbFile string) error {
	input, err := os.ReadFile(dbFile)
	if err != nil {
		return err
	}
	return os.WriteFile("build/bin/data/ph_holdings.db", input, 0644)
}
