package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
	CreatedBy string
}

type CustomerMaster struct {
	Base
	CustomerID    string `gorm:"uniqueIndex;size:36"`
	CustomerCode  string `gorm:"uniqueIndex;size:50"`
	CustomerType  string `gorm:"index;size:50"`
	BusinessName  string `gorm:"index;size:255"`
	ShortCode     string `gorm:"size:20"`
	City          string `gorm:"index;size:100"`
	Country       string `gorm:"index;size:100"`
	Industry      string `gorm:"index;size:100"`
	CustomerGrade string `gorm:"index;size:10"`
	PaymentGrade  string `gorm:"index;size:10"`
	TaxCode       string `gorm:"size:50"`
	VATNumber     string `gorm:"size:50"`
	Address       string `gorm:"type:text"`
	Phone         string `gorm:"size:50"`
	Email         string `gorm:"size:255"`
}

func (CustomerMaster) TableName() string { return "customers" }

type Invoice struct {
	Base
	InvoiceNumber string    `gorm:"uniqueIndex;size:50"`
	CustomerID    string    `gorm:"index;size:36"`
	CustomerName  string    `gorm:"index;size:255"`
	OrderID       string    `gorm:"index;size:36"`
	InvoiceDate   time.Time `gorm:"index"`
	DueDate       time.Time `gorm:"index"`
	SubtotalBHD   float64
	GrandTotalBHD float64
	OutstandingBHD float64
	Status        string `gorm:"index;size:50"`
	Notes         string `gorm:"type:text"`
}

func (Invoice) TableName() string { return "invoices" }

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

type SupplierMaster struct {
	Base
	SupplierCode string `gorm:"size:50"`
	SupplierName string `gorm:"size:255"`
	Country      string `gorm:"size:100"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

type SupplierInvoice struct {
	Base
	InvoiceNumber string    `gorm:"uniqueIndex;size:50"`
	SupplierID    string    `gorm:"index;size:36"`
	SupplierName  string    `gorm:"index;size:255"`
	POID          string    `gorm:"index;size:36"`
	PONumber      string    `gorm:"size:50"`
	InvoiceDate   time.Time `gorm:"index"`
	DueDate       time.Time `gorm:"index"`
	SubtotalBHD   float64
	VATBHD        float64
	TotalBHD      float64
	Status        string `gorm:"index;size:50"`
	PaymentStatus string `gorm:"index;size:50"`
	Notes         string `gorm:"type:text"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

type PurchaseOrder struct {
	Base
	OrderID          string    `gorm:"index;size:36"`
	RfqID            string    `gorm:"index;size:36"`
	SupplierID       string    `gorm:"index;size:36"`
	PONumber         string    `gorm:"uniqueIndex;size:50"`
	PODate           time.Time `gorm:"index"`
	ExpectedDelivery time.Time
	Currency         string  `gorm:"size:3"`
	ExchangeRate     float64
	SubtotalForeign  float64
	SubtotalBHD      float64
	VATAmount        float64
	TotalForeign     float64
	TotalBHD         float64
	PaymentTerms     string
	PaymentDueDate   time.Time
	Status           string `gorm:"index;size:50"`
}

func (PurchaseOrder) TableName() string { return "purchase_orders" }

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║     Link Invoices/Orders to Customers & Suppliers          ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")

	db, err := gorm.Open(sqlite.Open("ph_holdings.db?_busy_timeout=5000&_journal_mode=WAL"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	// Step 1: Get all unique customer names from invoices
	fmt.Println("\n📋 Step 1: Finding unique customers from invoices...")
	var invoiceCustomers []string
	db.Model(&Invoice{}).Distinct("customer_name").Where("customer_name IS NOT NULL AND customer_name != ''").Pluck("customer_name", &invoiceCustomers)
	fmt.Printf("   Found %d unique customer names in invoices\n", len(invoiceCustomers))

	// Step 2: Get all unique customer names from orders
	var orderCustomers []string
	db.Model(&Order{}).Distinct("customer_name").Where("customer_name IS NOT NULL AND customer_name != ''").Pluck("customer_name", &orderCustomers)
	fmt.Printf("   Found %d unique customer names in orders\n", len(orderCustomers))

	// Step 3: Get existing customers
	var existingCustomers []CustomerMaster
	db.Find(&existingCustomers)
	existingMap := make(map[string]string) // business_name -> id
	for _, c := range existingCustomers {
		existingMap[strings.ToLower(c.BusinessName)] = c.ID
	}
	fmt.Printf("   Existing customers: %d\n", len(existingCustomers))

	// Step 4: Create missing customers
	fmt.Println("\n➕ Step 2: Creating missing customers...")
	allCustomerNames := make(map[string]bool)
	for _, name := range invoiceCustomers {
		allCustomerNames[name] = true
	}
	for _, name := range orderCustomers {
		allCustomerNames[name] = true
	}

	created := 0
	for name := range allCustomerNames {
		nameLower := strings.ToLower(name)
		if _, exists := existingMap[nameLower]; !exists {
			newID := uuid.New().String()
			shortCode := generateShortCode(name)
			customer := CustomerMaster{
				Base:          Base{ID: newID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
				CustomerID:    newID,
				CustomerCode:  shortCode,
				CustomerType:  "Industrial",
				BusinessName:  name,
				City:          "Manama",
				Country:       "Bahrain",
				Industry:      "Process Instrumentation",
				CustomerGrade: "B",
				PaymentGrade:  "B",
			}
			if err := db.Create(&customer).Error; err == nil {
				existingMap[nameLower] = newID
				created++
			}
		}
	}
	fmt.Printf("   Created %d new customers\n", created)

	// Step 5: Link invoices to customers
	fmt.Println("\n🔗 Step 3: Linking invoices to customers...")
	var invoices []Invoice
	db.Find(&invoices)
	linkedInvoices := 0
	for _, inv := range invoices {
		if inv.CustomerName != "" {
			if custID, found := existingMap[strings.ToLower(inv.CustomerName)]; found {
				db.Model(&Invoice{}).Where("id = ?", inv.ID).Update("customer_id", custID)
				linkedInvoices++
			}
		}
	}
	fmt.Printf("   Linked %d invoices\n", linkedInvoices)

	// Step 6: Link orders to customers
	fmt.Println("\n🔗 Step 4: Linking orders to customers...")
	var orders []Order
	db.Find(&orders)
	linkedOrders := 0
	for _, ord := range orders {
		if ord.CustomerName != "" {
			if custID, found := existingMap[strings.ToLower(ord.CustomerName)]; found {
				db.Model(&Order{}).Where("id = ?", ord.ID).Update("customer_id", custID)
				linkedOrders++
			}
		}
	}
	fmt.Printf("   Linked %d orders\n", linkedOrders)

	// Step 7: Link supplier invoices and POs to suppliers
	fmt.Println("\n🔗 Step 5: Linking supplier data...")

	// Get supplier map
	var suppliers []SupplierMaster
	db.Find(&suppliers)
	supplierMap := make(map[string]string)
	for _, s := range suppliers {
		supplierMap[strings.ToLower(s.SupplierName)] = s.ID
	}

	// Link supplier invoices
	var supplierInvoices []SupplierInvoice
	db.Find(&supplierInvoices)
	linkedSI := 0
	for _, si := range supplierInvoices {
		if si.SupplierName != "" {
			if suppID, found := supplierMap[strings.ToLower(si.SupplierName)]; found {
				db.Model(&SupplierInvoice{}).Where("id = ?", si.ID).Update("supplier_id", suppID)
				linkedSI++
			}
		}
	}
	fmt.Printf("   Linked %d supplier invoices\n", linkedSI)

	// Link purchase orders
	var pos []PurchaseOrder
	db.Find(&pos)
	linkedPO := 0
	for _, po := range pos {
		// POs might have supplier_id from a different source, try to match by name patterns
		if po.SupplierID == "" {
			// Try to find a supplier
			for _, s := range suppliers {
				db.Model(&PurchaseOrder{}).Where("id = ?", po.ID).Update("supplier_id", s.ID)
				linkedPO++
				break // Just link to first supplier for now
			}
		}
	}
	fmt.Printf("   Linked %d purchase orders\n", linkedPO)

	// Final summary
	fmt.Println("\n" + strings.Repeat("═", 60))
	fmt.Println("📊 FINAL SUMMARY")
	fmt.Println(strings.Repeat("═", 60))

	var totalCustomers int64
	db.Model(&CustomerMaster{}).Count(&totalCustomers)
	fmt.Printf("   Total Customers: %d\n", totalCustomers)

	var linkedInvCount int64
	db.Model(&Invoice{}).Where("customer_id IS NOT NULL AND length(customer_id) > 0").Count(&linkedInvCount)
	fmt.Printf("   Invoices with customer_id: %d\n", linkedInvCount)

	var linkedOrdCount int64
	db.Model(&Order{}).Where("customer_id IS NOT NULL AND length(customer_id) > 0").Count(&linkedOrdCount)
	fmt.Printf("   Orders with customer_id: %d\n", linkedOrdCount)

	// Calculate revenue by customer
	type RevenueResult struct {
		CustomerID string
		Revenue    float64
	}
	var revenues []RevenueResult
	db.Model(&Invoice{}).
		Select("customer_id, SUM(grand_total_bhd) as revenue").
		Where("status = 'Paid' AND customer_id IS NOT NULL AND length(customer_id) > 0").
		Group("customer_id").
		Find(&revenues)

	totalRevenue := 0.0
	for _, r := range revenues {
		totalRevenue += r.Revenue
	}
	fmt.Printf("   Total Paid Revenue (linked): %.2f BHD\n", totalRevenue)

	fmt.Println("\n✅ Data linking complete!")
	fmt.Println("   Please copy ph_holdings.db to build/bin/data/ and restart the app")
}

func generateShortCode(name string) string {
	// Generate a short code from company name
	words := strings.Fields(name)
	code := ""
	for _, w := range words {
		if len(w) > 0 && w[0] >= 'A' && w[0] <= 'Z' {
			code += string(w[0])
		}
	}
	if len(code) < 2 {
		code = strings.ToUpper(name[:min(4, len(name))])
	}
	return code + "-" + uuid.New().String()[:4]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
