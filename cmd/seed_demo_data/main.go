package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// ============================================================================
// TYPE DEFINITIONS (matching database.go schema)
// ============================================================================

type CustomerMaster struct {
	ID               uint `gorm:"primaryKey"`
	CustomerID       string `gorm:"uniqueIndex;size:50"`
	BusinessName     string `gorm:"index;size:255"`
	PaymentGrade     string `gorm:"index;size:10"`
	AvgPaymentDays   float64
	PaymentTermsDays int
	RelationYears    int
	Industry         string
	Country          string
	DisputeCount     int
	HasABBCompetition bool
	IsEmergencyOnly  bool
}

func (CustomerMaster) TableName() string { return "customers" }

type ProductMaster struct {
	ID               uint `gorm:"primaryKey"`
	ProductCode      string `gorm:"uniqueIndex;size:100"`
	ProductName      string
	ProductCategory  string
	SupplierCode     string
	StandardCostBHD  float64
	StandardPriceBHD float64
	StandardMargin   float64
}

func (ProductMaster) TableName() string { return "products" }

type Offer struct {
	ID              uint `gorm:"primaryKey"`
	OfferNumber     string `gorm:"uniqueIndex;size:50"`
	RevisionNumber  int
	CustomerID      uint
	CustomerName    string
	RFQReceivedDate time.Time
	QuotationDate   time.Time
	ValidityDate    time.Time
	FollowUpDate    *time.Time
	TotalValueBHD   float64
	DiscountPercent float64
	NetValueBHD     float64
	EstimatedMargin float64
	Stage           string
	WinProbability  float64
	HasABBCompetition bool
	IsEmergency     bool
	PaymentTerms    string
	WonDate         *time.Time
	LostDate        *time.Time
	LostReason      string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	CreatedBy       string
}

func (Offer) TableName() string { return "offers" }

type OfferItem struct {
	ID              uint `gorm:"primaryKey"`
	OfferID         uint
	LineNumber      int
	ProductCode     string
	Description     string
	Quantity        float64
	UnitOfMeasure   string
	UnitCostBHD     float64
	UnitPriceBHD    float64
	DiscountPercent float64
	LineTotalBHD    float64
	MarginPercent   float64
	LeadTimeDays    int
	SupplierCode    string
	CreatedAt       time.Time
}

func (OfferItem) TableName() string { return "offer_items" }

type Invoice struct {
	ID                   uint `gorm:"primaryKey"`
	InvoiceNumber        string `gorm:"uniqueIndex;size:50"`
	InvoiceDate          time.Time
	CustomerID           uint
	CustomerName         string
	CustomerPONumber     string
	SubtotalBHD          float64
	VATPercent           float64
	VATAmountBHD         float64
	GrandTotalBHD        float64
	PaymentTerms         string
	DueDate              time.Time
	PredictedPaymentDays int
	PredictionGrade      string
	Status               string
	PaidDate             *time.Time
	PaidAmountBHD        float64
	OutstandingBHD       float64
	DaysOutstanding      int
	AgingBucket          string
	CreatedAt            time.Time
	UpdatedAt            time.Time
	CreatedBy            string
}

func (Invoice) TableName() string { return "invoices" }

type DBInvoiceItem struct {
	ID              uint `gorm:"primaryKey"`
	InvoiceID       uint
	LineNumber      int
	Description     string
	Quantity        float64
	UnitOfMeasure   string
	Rate            float64
	MonthlyAmount   float64
	VATPercent      float64
	TaxableValueBHD float64
	VATAmountBHD    float64
	TotalIncVATBHD  float64
	CreatedAt       time.Time
}

func (DBInvoiceItem) TableName() string { return "invoice_items" }

type PredictionRecord struct {
	ID            uint `gorm:"primaryKey"`
	CustomerID    string
	CustomerName  string
	Grade         string
	PredictedDays int
	Confidence    float64
	R1            float64
	R2            float64
	R3            float64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (PredictionRecord) TableName() string { return "prediction_records" }

type QuickCapture struct {
	ID        uint `gorm:"primaryKey"`
	Title     string
	Content   string
	Tags      string  // Comma-separated tags
	Priority  string  // Low, Medium, High, Urgent
	Status    string  // Open, InProgress, Done
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (QuickCapture) TableName() string { return "quick_captures" }

// ============================================================================
// MAIN SEEDING FUNCTION
// ============================================================================

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║      PH HOLDINGS SOVEREIGN UI - DEMO DATA SEEDING             ║")
	fmt.Println("║  Professional sample data for demos and presentations        ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Connect to database
	dbPath := "ph_holdings.db"
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Check existing data
	var customerCount, offerCount, invoiceCount, predictionCount, captureCount int64
	db.Model(&CustomerMaster{}).Count(&customerCount)
	db.Model(&Offer{}).Count(&offerCount)
	db.Model(&Invoice{}).Count(&invoiceCount)
	db.Model(&PredictionRecord{}).Count(&predictionCount)
	db.Model(&QuickCapture{}).Count(&captureCount)

	fmt.Printf("📊 Current Database State:\n")
	fmt.Printf("   Customers:    %d\n", customerCount)
	fmt.Printf("   Offers/RFQs:  %d\n", offerCount)
	fmt.Printf("   Invoices:     %d\n", invoiceCount)
	fmt.Printf("   Predictions:  %d\n", predictionCount)
	fmt.Printf("   Quick Captures: %d\n\n", captureCount)

	if customerCount == 0 {
		fmt.Println("⚠️  WARNING: No customers found!")
		fmt.Println("   Please run 'seed_master_data' first to create customers/products.")
		return
	}

	// Initialize random seed
	rand.Seed(time.Now().UnixNano())
	now := time.Now()

	// Get all customers for reference
	var customers []CustomerMaster
	db.Find(&customers)
	if len(customers) == 0 {
		log.Fatal("No customers found. Run seed_master_data first.")
	}

	// Get products
	var products []ProductMaster
	db.Find(&products)
	if len(products) == 0 {
		log.Fatal("No products found. Run seed_master_data first.")
	}

	// ======================================================================
	// SEED OFFERS/RFQs (Realistic pipeline)
	// ======================================================================
	fmt.Println("💼 Seeding Offers/RFQs...")

	offerData := []struct {
		CustomerID string
		Stage      string
		DaysAgo    int
		Value      float64
		WinProb    float64
		IsEmergency bool
		HasABB      bool
		Outcome    string // "won", "lost", or ""
		LostReason string
	}{
		{"CO1", "Won", 45, 85000, 0.95, false, false, "won", ""},
		{"EC3", "Won", 30, 42000, 0.90, false, false, "won", ""},
		{"CO5", "Quoted", 15, 120000, 0.85, false, false, "", ""},
		{"EP1", "Follow-up", 20, 95000, 0.75, false, false, "", ""},
		{"CO2", "RFQ", 3, 38000, 0.60, false, true, "", ""},
		{"EC12", "Negotiating", 8, 52000, 0.70, false, false, "", ""},
		{"CO8", "Quoted", 12, 28000, 0.65, true, false, "", ""},
		{"CO11", "Lost", 60, 45000, 0.40, false, true, "lost", "Lost to ABB on price"},
		{"EC14", "Follow-up", 25, 32000, 0.55, true, false, "", ""},
		{"CO17", "RFQ", 5, 68000, 0.50, false, false, "", ""},
	}

	offerCounter := 1
	for _, od := range offerData {
		// Find customer
		var customer CustomerMaster
		if err := db.Where("customer_id = ?", od.CustomerID).First(&customer).Error; err != nil {
			log.Printf("⚠️  Customer %s not found, skipping offer", od.CustomerID)
			continue
		}

		rfqDate := now.AddDate(0, 0, -od.DaysAgo)
		quotationDate := rfqDate.AddDate(0, 0, 2)
		validityDate := quotationDate.AddDate(0, 0, 30)
		followUpDate := quotationDate.AddDate(0, 0, 7)

		offer := Offer{
			OfferNumber:       fmt.Sprintf("PH25/%03d", offerCounter),
			RevisionNumber:    0,
			CustomerID:        customer.ID,
			CustomerName:      customer.BusinessName,
			RFQReceivedDate:   rfqDate,
			QuotationDate:     quotationDate,
			ValidityDate:      validityDate,
			FollowUpDate:      &followUpDate,
			TotalValueBHD:     od.Value,
			DiscountPercent:   5.0,
			NetValueBHD:       od.Value * 0.95,
			EstimatedMargin:   45.0,
			Stage:             od.Stage,
			WinProbability:    od.WinProb,
			HasABBCompetition: od.HasABB,
			IsEmergency:       od.IsEmergency,
			PaymentTerms:      fmt.Sprintf("%d Days", customer.PaymentTermsDays),
			CreatedAt:         rfqDate,
			UpdatedAt:         now,
			CreatedBy:         "demo_seed",
		}

		// Set outcome dates
		if od.Outcome == "won" {
			wonDate := quotationDate.AddDate(0, 0, 14)
			offer.WonDate = &wonDate
		} else if od.Outcome == "lost" {
			lostDate := quotationDate.AddDate(0, 0, 21)
			offer.LostDate = &lostDate
			offer.LostReason = od.LostReason
		}

		if err := db.Create(&offer).Error; err != nil {
			log.Printf("❌ Failed to create offer %s: %v", offer.OfferNumber, err)
			continue
		}

		// Create 2-4 offer items
		numItems := 2 + rand.Intn(3)
		for i := 0; i < numItems; i++ {
			product := products[rand.Intn(len(products))]
			qty := float64(1 + rand.Intn(5))
			unitPrice := product.StandardPriceBHD
			lineTotal := qty * unitPrice

			item := OfferItem{
				OfferID:         offer.ID,
				LineNumber:      i + 1,
				ProductCode:     product.ProductCode,
				Description:     product.ProductName,
				Quantity:        qty,
				UnitOfMeasure:   "Each",
				UnitCostBHD:     product.StandardCostBHD,
				UnitPriceBHD:    unitPrice,
				DiscountPercent: 0,
				LineTotalBHD:    lineTotal,
				MarginPercent:   product.StandardMargin,
				LeadTimeDays:    60,
				SupplierCode:    product.SupplierCode,
				CreatedAt:       quotationDate,
			}

			if err := db.Create(&item).Error; err != nil {
				log.Printf("⚠️  Failed to create offer item: %v", err)
			}
		}

		fmt.Printf("   ✓ %s [%s] - %s (%.0f BHD)\n",
			offer.OfferNumber, offer.Stage, customer.BusinessName, offer.NetValueBHD)
		offerCounter++
	}

	// ======================================================================
	// SEED INVOICES (Mix of paid, pending, overdue)
	// ======================================================================
	fmt.Println("\n📄 Seeding Invoices...")

	invoiceData := []struct {
		CustomerID   string
		DaysAgo      int
		Amount       float64
		Status       string
		DaysPaid     int // Days after invoice date when paid (0 = not paid)
	}{
		// PAID INVOICES (Good payment history)
		{"CO1", 90, 45000, "Paid", 28},
		{"CO5", 75, 62000, "Paid", 30},
		{"EC3", 60, 38000, "Paid", 40},
		{"EP1", 50, 85000, "Paid", 55},
		{"CO2", 45, 28000, "Paid", 48},

		// PENDING INVOICES (Within terms, not overdue)
		{"EC12", 25, 42000, "Sent", 0},
		{"CO8", 20, 32000, "Sent", 0},
		{"EP4", 15, 58000, "Sent", 0},

		// OVERDUE INVOICES (Need follow-up!)
		{"CO11", 75, 28000, "Overdue", 0},
		{"EC9", 65, 18000, "Overdue", 0},
		{"CO13", 90, 22000, "Overdue", 0},
		{"CO3", 50, 15000, "Overdue", 0},

		// RECENT INVOICES
		{"CO15", 10, 35000, "Sent", 0},
		{"CO22", 8, 24000, "Sent", 0},
		{"EC18", 5, 19000, "Sent", 0},
	}

	invoiceCounter := 1
	for _, id := range invoiceData {
		var customer CustomerMaster
		if err := db.Where("customer_id = ?", id.CustomerID).First(&customer).Error; err != nil {
			log.Printf("⚠️  Customer %s not found, skipping invoice", id.CustomerID)
			continue
		}

		invoiceDate := now.AddDate(0, 0, -id.DaysAgo)
		dueDate := invoiceDate.AddDate(0, 0, customer.PaymentTermsDays)

		subtotal := id.Amount
		vat := subtotal * 0.10
		grandTotal := subtotal + vat

		invoice := Invoice{
			InvoiceNumber:        fmt.Sprintf("INV-2025-%04d", invoiceCounter),
			InvoiceDate:          invoiceDate,
			CustomerID:           customer.ID,
			CustomerName:         customer.BusinessName,
			CustomerPONumber:     fmt.Sprintf("PO-%s-%03d", id.CustomerID, invoiceCounter),
			SubtotalBHD:          subtotal,
			VATPercent:           10.0,
			VATAmountBHD:         vat,
			GrandTotalBHD:        grandTotal,
			PaymentTerms:         fmt.Sprintf("%d Days", customer.PaymentTermsDays),
			DueDate:              dueDate,
			PredictedPaymentDays: int(customer.AvgPaymentDays),
			PredictionGrade:      customer.PaymentGrade,
			Status:               id.Status,
			CreatedAt:            invoiceDate,
			UpdatedAt:            now,
			CreatedBy:            "demo_seed",
		}

		// Handle paid invoices
		if id.Status == "Paid" && id.DaysPaid > 0 {
			paidDate := invoiceDate.AddDate(0, 0, id.DaysPaid)
			invoice.PaidDate = &paidDate
			invoice.PaidAmountBHD = grandTotal
			invoice.OutstandingBHD = 0
			invoice.DaysOutstanding = 0
			invoice.AgingBucket = "Paid"
		} else {
			// Calculate outstanding and aging
			invoice.PaidAmountBHD = 0
			invoice.OutstandingBHD = grandTotal
			invoice.DaysOutstanding = int(now.Sub(invoiceDate).Hours() / 24)

			// Determine aging bucket
			daysOut := invoice.DaysOutstanding
			if daysOut <= 30 {
				invoice.AgingBucket = "Current"
			} else if daysOut <= 60 {
				invoice.AgingBucket = "30-60"
			} else if daysOut <= 90 {
				invoice.AgingBucket = "60-90"
			} else if daysOut <= 120 {
				invoice.AgingBucket = "90-120"
			} else {
				invoice.AgingBucket = "120+"
			}
		}

		if err := db.Create(&invoice).Error; err != nil {
			log.Printf("❌ Failed to create invoice %s: %v", invoice.InvoiceNumber, err)
			continue
		}

		// Create 1-3 invoice items
		numItems := 1 + rand.Intn(3)
		itemAmount := subtotal / float64(numItems)
		for i := 0; i < numItems; i++ {
			product := products[rand.Intn(len(products))]
			qty := float64(1 + rand.Intn(3))
			rate := itemAmount / qty
			vatItem := itemAmount * 0.10

			item := DBInvoiceItem{
				InvoiceID:       invoice.ID,
				LineNumber:      i + 1,
				Description:     product.ProductName,
				Quantity:        qty,
				UnitOfMeasure:   "Each",
				Rate:            rate,
				MonthlyAmount:   itemAmount,
				VATPercent:      10.0,
				TaxableValueBHD: itemAmount,
				VATAmountBHD:    vatItem,
				TotalIncVATBHD:  itemAmount + vatItem,
				CreatedAt:       invoiceDate,
			}

			if err := db.Create(&item).Error; err != nil {
				log.Printf("⚠️  Failed to create invoice item: %v", err)
			}
		}

		statusIcon := "📄"
		if id.Status == "Paid" {
			statusIcon = "✅"
		} else if id.Status == "Overdue" {
			statusIcon = "⚠️ "
		}

		fmt.Printf("   %s %s [%s] - %s (%.0f BHD)\n",
			statusIcon, invoice.InvoiceNumber, invoice.Status,
			customer.BusinessName, invoice.GrandTotalBHD)
		invoiceCounter++
	}

	// ======================================================================
	// SEED PREDICTIONS (One per customer with realistic regimes)
	// ======================================================================
	fmt.Println("\n🔮 Seeding Payment Predictions...")

	// Generate predictions for top 10 customers
	predictCount := 0
	for i := 0; i < len(customers) && i < 10; i++ {
		customer := customers[i]

		// Calculate realistic R1/R2/R3 based on payment grade
		var r1, r2, r3 float64
		var predictedDays int
		var confidence float64

		switch customer.PaymentGrade {
		case "A":
			// Grade A: High stability (R3 dominant)
			r1, r2, r3 = 0.15, 0.10, 0.75
			predictedDays = 30 + rand.Intn(10)
			confidence = 0.90 + rand.Float64()*0.08

		case "B":
			// Grade B: Good balance
			r1, r2, r3 = 0.25, 0.20, 0.55
			predictedDays = 45 + rand.Intn(15)
			confidence = 0.75 + rand.Float64()*0.12

		case "C":
			// Grade C: Higher exploration/optimization (R1+R2)
			r1, r2, r3 = 0.35, 0.25, 0.40
			predictedDays = 60 + rand.Intn(20)
			confidence = 0.60 + rand.Float64()*0.12

		case "D":
			// Grade D: Chaotic (R1 dominant, low R3)
			r1, r2, r3 = 0.45, 0.30, 0.25
			predictedDays = 90 + rand.Intn(30)
			confidence = 0.45 + rand.Float64()*0.12

		default:
			r1, r2, r3 = 0.30, 0.20, 0.50
			predictedDays = 60
			confidence = 0.70
		}

		prediction := PredictionRecord{
			CustomerID:    customer.CustomerID,
			CustomerName:  customer.BusinessName,
			Grade:         customer.PaymentGrade,
			PredictedDays: predictedDays,
			Confidence:    confidence,
			R1:            r1,
			R2:            r2,
			R3:            r3,
			CreatedAt:     now.AddDate(0, 0, -rand.Intn(30)),
			UpdatedAt:     now,
		}

		if err := db.Create(&prediction).Error; err != nil {
			log.Printf("⚠️  Failed to create prediction for %s: %v", customer.CustomerID, err)
			continue
		}

		fmt.Printf("   ✓ %s [Grade %s] - %d days (Confidence: %.1f%%, R3: %.1f%%)\n",
			customer.CustomerID, customer.PaymentGrade,
			predictedDays, confidence*100, r3*100)
		predictCount++
	}

	// ======================================================================
	// SEED QUICK CAPTURES (Demo notes/tasks)
	// ======================================================================
	fmt.Println("\n📝 Seeding Quick Captures...")

	captures := []QuickCapture{
		{
			Title:     "Follow up on BAPCO payment for INV-2025-0012",
			Content:   "Need to check with accounts - should have been paid last week. Customer: CO1 (BAPCO)",
			Tags:      "payment,followup,CO1",
			Priority:  "High",
			Status:    "Open",
			CreatedAt: now.AddDate(0, 0, -2),
			UpdatedAt: now.AddDate(0, 0, -2),
		},
		{
			Title:     "Prepare quotation for Saudi Engineering Services RFQ",
			Content:   "New RFQ received for 5x Coriolis flowmeters - deadline Friday. Customer: EC3",
			Tags:      "sales,rfq,EC3,urgent",
			Priority:  "High",
			Status:    "Open",
			CreatedAt: now.AddDate(0, 0, -1),
			UpdatedAt: now.AddDate(0, 0, -1),
		},
		{
			Title:     "Schedule technical meeting with ALBA",
			Content:   "They want to discuss E+H radar level transmitters for new tank farm. Customer: CO2 (ALBA)",
			Tags:      "meeting,technical,CO2",
			Priority:  "Medium",
			Status:    "Open",
			CreatedAt: now.AddDate(0, 0, -3),
			UpdatedAt: now.AddDate(0, 0, -3),
		},
		{
			Title:     "Review pricing with E+H on bulk order",
			Content:   "Petrofac wants 20+ units - need better pricing from supplier. Customer: EP1 (Petrofac)",
			Tags:      "procurement,pricing,EP1",
			Priority:  "Medium",
			Status:    "InProgress",
			CreatedAt: now.AddDate(0, 0, -5),
			UpdatedAt: now.AddDate(0, 0, -1),
		},
		{
			Title:     "Collect overdue payment from Bahrain Chemical Company",
			Content:   "INV-2025-0011 is 90 days overdue - escalate to manager. Customer: CO13",
			Tags:      "collections,overdue,CO13,urgent",
			Priority:  "Urgent",
			Status:    "Open",
			CreatedAt: now.AddDate(0, 0, -7),
			UpdatedAt: now.AddDate(0, 0, -7),
		},
	}

	for _, capture := range captures {
		if err := db.Create(&capture).Error; err != nil {
			log.Printf("⚠️  Failed to create quick capture: %v", err)
			continue
		}
		fmt.Printf("   ✓ [%s] %s\n", capture.Priority, capture.Title)
	}

	// ======================================================================
	// FINAL SUMMARY
	// ======================================================================

	db.Model(&Offer{}).Count(&offerCount)
	db.Model(&Invoice{}).Count(&invoiceCount)
	db.Model(&PredictionRecord{}).Count(&predictionCount)
	db.Model(&QuickCapture{}).Count(&captureCount)

	fmt.Println("\n╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    SEEDING COMPLETE!                          ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Printf("📊 Demo Data Summary:\n")
	fmt.Printf("   Offers/RFQs:     %d (RFQ → Quoted → Won/Lost pipeline)\n", offerCount)
	fmt.Printf("   Invoices:        %d (Paid, Pending, Overdue mix)\n", invoiceCount)
	fmt.Printf("   Predictions:     %d (With three-regime R1/R2/R3 data)\n", predictionCount)
	fmt.Printf("   Quick Captures:  %d (Sample notes and tasks)\n", captureCount)
	fmt.Println()
	fmt.Println("🎉 PH Holdings Sovereign UI is ready for professional demos!")
	fmt.Println()
	fmt.Println("💡 Next Steps:")
	fmt.Println("   1. Run the app: wails dev")
	fmt.Println("   2. Navigate to different screens to see the data")
	fmt.Println("   3. Test payment predictions, customer 360, pricing tools")
	fmt.Println()
	fmt.Println("🔄 To reset: Delete ph_holdings.db and re-run both seed commands")
	fmt.Println()
}
