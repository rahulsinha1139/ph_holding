package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Data Structures
type ExtractedData struct {
	Customers []ExtractedCustomer `json:"customers"`
	Products  []ExtractedProduct  `json:"products"`
}

type ExtractedCustomer struct {
	Name string `json:"name"`
}

type ExtractedProduct struct {
	Code        string  `json:"code"`
	Description string  `json:"description"`
	CostEUR     float64 `json:"cost_eur"`
}

// Database Structs
type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
}

type ProductMaster struct {
	Base
	ProductCode      string `gorm:"uniqueIndex;size:100"`
	ProductName      string `gorm:"index;size:255"`
	Description      string `gorm:"type:text"`
	SupplierCode     string `gorm:"index;size:50"`
	StandardCostBHD  float64
	StandardPriceBHD float64
}

func (ProductMaster) TableName() string { return "products" }

type CustomerMaster struct {
	Base
	CustomerID   string `gorm:"uniqueIndex;size:50"`
	BusinessName string `gorm:"index;size:255"`
	CustomerType string
	City         string
	Country      string

	// Flags
	IsCreditBlocked    bool
	RequiresPrepayment bool
	HasABBCompetition  bool
	IsEmergencyOnly    bool

	// Metrics
	TotalOrdersValue float64
	TotalOrdersCount int
	AvgOrderValue    float64
	LastOrderDate    *time.Time
	PaymentGrade     string
	AvgPaymentDays   float64
}

func (CustomerMaster) TableName() string { return "customers" }

type Offer struct {
	Base
	OfferNumber   string `gorm:"uniqueIndex;size:50"`
	CustomerID    string `gorm:"index;size:36"`
	CustomerName  string `gorm:"index;size:255"`
	QuotationDate time.Time
	TotalValueBHD float64
	Stage         string
}

func (Offer) TableName() string { return "offers" }

type OfferItem struct {
	Base
	OfferID     string `gorm:"index;size:36"`
	ProductID   string `gorm:"index;size:36"`
	ProductCode string `gorm:"size:100"`
	Description string `gorm:"type:text"`
	Quantity    float64
	UnitPrice   float64
}

func (OfferItem) TableName() string { return "offer_items" }

type Order struct {
	Base
	OrderNumber   string `gorm:"uniqueIndex;size:50"`
	CustomerID    string `gorm:"index;size:36"`
	CustomerName  string `gorm:"index;size:255"`
	OrderDate     time.Time
	GrandTotalBHD float64
	Status        string
	PaymentTerms  string
}

func (Order) TableName() string { return "orders" }

type Invoice struct {
	Base
	InvoiceNumber  string `gorm:"uniqueIndex;size:50"`
	InvoiceDate    time.Time
	CustomerID     string `gorm:"index;size:36"`
	CustomerName   string `gorm:"index;size:255"`
	OrderID        string `gorm:"index;size:36"`
	GrandTotalBHD  float64
	Status         string // Sent, Paid, Overdue
	OutstandingBHD float64
	DueDate        time.Time
}

func (Invoice) TableName() string { return "invoices" }

type Payment struct {
	Base
	InvoiceID     string `gorm:"index;size:36"`
	InvoiceNumber string `gorm:"index;size:50"`
	CustomerID    string `gorm:"index;size:36"` // Denormalized for speed
	AmountBHD     float64
	PaymentDate   time.Time
	PaymentMethod string
	DaysToPayment int
}

func (Payment) TableName() string { return "payments" }

type PredictionRecord struct {
	Base
	CustomerID    string `gorm:"index;size:36"`
	CustomerName  string
	Grade         string
	PredictedDays int
	Confidence    float64
	R1            float64
	R2            float64
	R3            float64
}

func (PredictionRecord) TableName() string { return "prediction_records" }

func main() {
	// 1. Load Seed Data
	jsonFile, err := os.Open("seed_data.json")
	if err != nil {
		log.Fatal("Run archaeology first! seed_data.json not found.")
	}
	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)
	var data ExtractedData
	json.Unmarshal(byteValue, &data)

	// 2. Connect DB
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("🚀 Seeding Rich Real Data (Time Travel Mode)...")

	// Ensure Schema
	db.AutoMigrate(
		&ProductMaster{}, &CustomerMaster{},
		&Offer{}, &OfferItem{},
		&Order{}, &Invoice{}, &Payment{},
		&PredictionRecord{},
	)

	// 3. Seed Products
	fmt.Printf("📦 Seeding %d Products...\n", len(data.Products))
	productMap := make(map[string]string) // Code -> ID
	productsList := []ProductMaster{}     // Keep track for random selection

	for _, p := range data.Products {
		costBHD := p.CostEUR * 0.41
		prod := ProductMaster{
			Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			ProductCode:      p.Code,
			ProductName:      p.Description,
			Description:      p.Description,
			SupplierCode:     "EH",
			StandardCostBHD:  costBHD,
			StandardPriceBHD: costBHD * 1.35,
		}

		// Upsert
		var existing ProductMaster
		if err := db.Where("product_code = ?", p.Code).First(&existing).Error; err == nil {
			productMap[p.Code] = existing.ID
			productsList = append(productsList, existing)
		} else {
			db.Create(&prod)
			productMap[p.Code] = prod.ID
			productsList = append(productsList, prod)
		}
	}

	// 4. Seed Customers & Transactions from Folders
	rootPath := `C:\Users\rishu\OneDrive\Desktop\asymm\production_shenanigans\asymmetrica-production\ph_holdings_sovereign_ui\Offers No 1-50 (2025)`

	fmt.Println("📂 Scanning Folders & Generating History...")

	files, _ := os.ReadDir(rootPath)

	// Random generator
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		folderName := f.Name()
		custName := extractCustomerName(folderName)
		if custName == "" {
			continue
		}

		// Find/Create Customer
		var cust CustomerMaster
		if err := db.Where("business_name = ?", custName).First(&cust).Error; err != nil {
			cust = CustomerMaster{
				Base:               Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
				CustomerID:         fmt.Sprintf("CUST-%d", rnd.Intn(9000)+1000),
				BusinessName:       custName,
				CustomerType:       "Industrial",
				City:               "Manama",
				Country:            "Bahrain",
				IsCreditBlocked:    false,
				RequiresPrepayment: false,
				HasABBCompetition:  rnd.Intn(2) == 1, // Random true/false
				IsEmergencyOnly:    false,
				PaymentGrade:       "B", // Default
			}
			db.Create(&cust)
			fmt.Printf("  👤 Customer: %s\n", custName)
		}

		// Create Offer (Date derived from file info or random recent)
		fileInfo, _ := f.Info()
		offerDate := fileInfo.ModTime()
		if offerDate.Year() < 2024 {
			offerDate = time.Now().AddDate(0, -rnd.Intn(6), -rnd.Intn(30))
		}

		offer := Offer{
			Base:          Base{ID: uuid.New().String(), CreatedAt: offerDate, UpdatedAt: offerDate},
			OfferNumber:   fmt.Sprintf("OFF-%s-%d", strings.Split(folderName, " ")[0], rnd.Intn(99)),
			CustomerID:    cust.ID,
			CustomerName:  cust.BusinessName,
			QuotationDate: offerDate,
			Stage:         "Quoted",
			TotalValueBHD: 0,
		}
		db.Create(&offer)

		// Add Items
		itemCount := rnd.Intn(3) + 1
		totalValue := 0.0
		for i := 0; i < itemCount; i++ {
			p := productsList[rnd.Intn(len(productsList))]
			qty := float64(rnd.Intn(5) + 1)
			price := p.StandardPriceBHD

			item := OfferItem{
				Base:        Base{ID: uuid.New().String()},
				OfferID:     offer.ID,
				ProductID:   p.ID,
				ProductCode: p.ProductCode,
				Description: p.Description,
				Quantity:    qty,
				UnitPrice:   price,
			}
			db.Create(&item)
			totalValue += price * qty
		}
		db.Model(&offer).Update("total_value_bhd", totalValue)

		// 60% chance to convert to Order
		if rnd.Float32() < 0.60 {
			db.Model(&offer).Update("stage", "Won")

			orderDate := offerDate.AddDate(0, 0, rnd.Intn(10)+2)
			order := Order{
				Base:          Base{ID: uuid.New().String(), CreatedAt: orderDate, UpdatedAt: orderDate},
				OrderNumber:   strings.Replace(offer.OfferNumber, "OFF", "ORD", 1),
				CustomerID:    cust.ID,
				CustomerName:  cust.BusinessName,
				OrderDate:     orderDate,
				GrandTotalBHD: totalValue,
				Status:        "Processing",
				PaymentTerms:  "Net 30",
			}
			db.Create(&order)

			// 90% chance to be Invoiced
			if rnd.Float32() < 0.90 {
				invoiceDate := orderDate.AddDate(0, 0, rnd.Intn(5)+1)
				dueDate := invoiceDate.AddDate(0, 0, 30)

				invoice := Invoice{
					Base:           Base{ID: uuid.New().String(), CreatedAt: invoiceDate, UpdatedAt: invoiceDate},
					InvoiceNumber:  strings.Replace(offer.OfferNumber, "OFF", "INV", 1),
					InvoiceDate:    invoiceDate,
					CustomerID:     cust.ID,
					CustomerName:   cust.BusinessName,
					OrderID:        order.ID,
					GrandTotalBHD:  totalValue,
					Status:         "Sent",
					OutstandingBHD: totalValue,
					DueDate:        dueDate,
				}
				db.Create(&invoice)

				// Payment Logic - Mark all invoices as PAID by default
				// User can change status manually later if needed
				daysToPay := 30 + rnd.Intn(30) // 30-60 days payment cycle
				paymentDate := invoiceDate.AddDate(0, 0, daysToPay)

				payment := Payment{
					Base:          Base{ID: uuid.New().String(), CreatedAt: paymentDate},
					InvoiceID:     invoice.ID,
					InvoiceNumber: invoice.InvoiceNumber,
					CustomerID:    cust.ID,
					AmountBHD:     totalValue,
					PaymentDate:   paymentDate,
					PaymentMethod: "Bank Transfer",
					DaysToPayment: daysToPay,
				}
				db.Create(&payment)

				db.Model(&invoice).Updates(map[string]interface{}{
					"status":          "Paid",
					"outstanding_bhd": 0,
				})
				db.Model(&order).Update("status", "Completed")
			}
		}
	}

	// 5. Update Customer Metrics (Orders, Total Value, Payment Days)
	fmt.Println("📊 Calculating Customer Metrics & Predictions...")

	var customers []CustomerMaster
	db.Find(&customers)

	for _, c := range customers {
		var orderCount int64
		var totalVal float64
		db.Model(&Order{}).Where("customer_id = ?", c.ID).Count(&orderCount)
		db.Model(&Order{}).Where("customer_id = ?", c.ID).Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&totalVal)

		// Payment Stats
		var payments []Payment
		db.Where("customer_id = ?", c.ID).Find(&payments)

		var totalDays int
		var countPayments int
		for _, p := range payments {
			totalDays += p.DaysToPayment
			countPayments++
		}

		avgDays := 0.0
		grade := "C" // Default
		if countPayments > 0 {
			avgDays = float64(totalDays) / float64(countPayments)
			if avgDays <= 35 {
				grade = "A"
			} else if avgDays <= 50 {
				grade = "B"
			} else if avgDays <= 75 {
				grade = "C"
			} else {
				grade = "D"
			}
		}

		// Update Customer
		db.Model(&c).Updates(map[string]interface{}{
			"total_orders_count": orderCount,
			"total_orders_value": totalVal,
			"avg_order_value":    0, // Simplified
			"avg_payment_days":   avgDays,
			"payment_grade":      grade,
		})

		// Create Insight/Prediction Record
		pred := PredictionRecord{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now()},
			CustomerID:    c.ID, // UUID
			CustomerName:  c.BusinessName,
			Grade:         grade,
			PredictedDays: int(avgDays),
			Confidence:    85.5 + rnd.Float64()*10,
			R1:            rnd.Float64() * 0.3,
			R2:            rnd.Float64() * 0.3,
			R3:            0.5 + rnd.Float64()*0.4, // Mostly stable (R3)
		}
		db.Create(&pred)
	}

	fmt.Println("✅ Rich Data Seeding Complete!")

	// Auto-deploy to build directory for app to use
	fmt.Println("\n📦 Deploying database to app location...")
	deployPath := "build/bin/data"
	if err := os.MkdirAll(deployPath, 0755); err != nil {
		fmt.Printf("⚠️ Warning: Could not create deploy directory: %v\n", err)
		return
	}

	// Copy database to build location
	sourceDB := "ph_holdings.db"
	targetDB := deployPath + "/ph_holdings.db"

	input, err := os.ReadFile(sourceDB)
	if err != nil {
		fmt.Printf("⚠️ Warning: Could not read source database: %v\n", err)
		return
	}

	err = os.WriteFile(targetDB, input, 0644)
	if err != nil {
		fmt.Printf("⚠️ Warning: Could not deploy database: %v\n", err)
		return
	}

	fmt.Printf("✅ Database deployed to: %s\n", targetDB)
	fmt.Println("\n🚀 Ready! Run: .\\build\\bin\\ph_holdings_app.exe")
}

func extractCustomerName(folder string) string {
	upper := strings.ToUpper(folder)
	if strings.Contains(upper, "EWA") {
		return "Electricity & Water Authority"
	}
	if strings.Contains(upper, "ALBA") {
		return "Aluminium Bahrain (ALBA)"
	}
	if strings.Contains(upper, "BAPCO") {
		return "Bapco Energies"
	}
	if strings.Contains(upper, "GPIC") {
		return "Gulf Petrochemical Industries Co."
	}
	if strings.Contains(upper, "SULB") {
		return "SULB Company"
	}
	if strings.Contains(upper, "MOW") {
		return "Ministry of Works"
	}
	if strings.Contains(upper, "ENGIE") {
		return "Engie Power & Water"
	}
	if strings.Contains(upper, "ARLA") {
		return "Arla Foods"
	}
	if strings.Contains(upper, "TTSJV") {
		return "Technip-Technicas Reunidas JV"
	}
	if strings.Contains(upper, "VEOLIA") {
		return "Veolia Water Technologies"
	}
	if strings.Contains(upper, "SEAPEAK") {
		return "Seapeak Maritime"
	}

	parts := strings.Split(folder, " ")
	if len(parts) > 1 {
		name := parts[1]
		name = strings.ReplaceAll(name, "-", " ")
		return name
	}
	return "General Client"
}
