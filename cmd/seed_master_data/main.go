package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Use the same type definitions from migrate_schema.go
// (In production, these would be imported from main package)

type CustomerMaster struct {
	ID                 uint   `gorm:"primaryKey"`
	CustomerID         string `gorm:"uniqueIndex;size:50"`
	CustomerType       string `gorm:"index;size:50"`
	BusinessName       string `gorm:"index;size:255"`
	ShortCode          string `gorm:"size:10"`
	City               string `gorm:"index;size:100"`
	Country            string `gorm:"index;size:100"`
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
	HasABBCompetition  bool `gorm:"index"`
	IsEmergencyOnly    bool
	IsCreditBlocked    bool `gorm:"index"`
	RequiresPrepayment bool
	CreatedAt          time.Time `gorm:"index"`
	UpdatedAt          time.Time
	CreatedBy          string     `gorm:"size:100"`
	UpdatedBy          string     `gorm:"size:100"`
	DeletedAt          *time.Time `gorm:"index"`
}

func (CustomerMaster) TableName() string { return "customers" }

type SupplierMaster struct {
	ID                 uint   `gorm:"primaryKey"`
	SupplierCode       string `gorm:"uniqueIndex;size:50"`
	SupplierName       string `gorm:"index;size:255"`
	Country            string `gorm:"index;size:100"`
	Website            string `gorm:"size:255"`
	Email              string `gorm:"size:255"`
	PaymentTermsDays   int
	LeadTimeDays       int
	MinimumOrderQty    int
	IsAuthorizedDealer bool
	IsASPCertified     bool
	GCCAuthorization   bool
	CreditLimitBHD     float64
	CurrentBalanceBHD  float64
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CreatedBy          string     `gorm:"size:100"`
	DeletedAt          *time.Time `gorm:"index"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

type ProductMaster struct {
	ID               uint   `gorm:"primaryKey"`
	ProductCode      string `gorm:"uniqueIndex;size:100"`
	ProductName      string `gorm:"index;size:255"`
	ProductCategory  string `gorm:"index;size:100"`
	SupplierID       uint   `gorm:"index"`
	SupplierCode     string `gorm:"index;size:50"`
	Description      string `gorm:"type:text"`
	StandardCostBHD  float64
	StandardPriceBHD float64
	StandardMargin   float64
	ABBEquivalent    string `gorm:"size:100"`
	IsActive         bool   `gorm:"index"`
	IsStocked        bool
	StockQuantity    int
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time `gorm:"index"`
}

func (ProductMaster) TableName() string { return "products" }

func main() {
	fmt.Println("=== PH Holdings Master Data Seeding ===")
	fmt.Println()

	// Connect to database
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	// Ensure schema exists
	fmt.Println("Migrating schema...")
	if err := db.AutoMigrate(&CustomerMaster{}, &SupplierMaster{}, &ProductMaster{}); err != nil {
		log.Fatal("Failed to migrate:", err)
	}

	// Check existing data
	var customerCount, supplierCount, productCount int64
	db.Model(&CustomerMaster{}).Count(&customerCount)
	db.Model(&SupplierMaster{}).Count(&supplierCount)
	db.Model(&ProductMaster{}).Count(&productCount)

	fmt.Printf("Existing Data:\n")
	fmt.Printf("  Customers: %d\n", customerCount)
	fmt.Printf("  Suppliers: %d\n", supplierCount)
	fmt.Printf("  Products:  %d\n\n", productCount)

	if customerCount > 0 || supplierCount > 0 || productCount > 0 {
		fmt.Println("⚠ Master data already exists. Skipping seed.")
		fmt.Println("(Delete rows manually if you want to re-seed)")
		return
	}

	rand.Seed(time.Now().UnixNano())
	now := time.Now()

	// ======================================================================
	// SEED SUPPLIERS (Key Manufacturers)
	// ======================================================================

	fmt.Println("Seeding Suppliers...")

	suppliers := []SupplierMaster{
		{
			SupplierCode:       "EH",
			SupplierName:       "Endress+Hauser AG",
			Country:            "Switzerland",
			Website:            "www.endress.com",
			Email:              "info@endress.com",
			PaymentTermsDays:   60,
			LeadTimeDays:       75, // 2-3 months typical
			MinimumOrderQty:    1,  // Will do small orders (advantage!)
			IsAuthorizedDealer: true,
			IsASPCertified:     true, // PH Holdings is E+H ASP
			GCCAuthorization:   true,
			CreditLimitBHD:     500000,
			CurrentBalanceBHD:  120000,
			CreatedAt:          now,
			UpdatedAt:          now,
			CreatedBy:          "system_seed",
		},
		{
			SupplierCode:       "LG",
			SupplierName:       "LG Electronics",
			Country:            "South Korea",
			Website:            "www.lg.com",
			Email:              "b2b@lge.com",
			PaymentTermsDays:   45,
			LeadTimeDays:       30,
			MinimumOrderQty:    10,
			IsAuthorizedDealer: true,
			IsASPCertified:     false,
			GCCAuthorization:   true,
			CreditLimitBHD:     200000,
			CurrentBalanceBHD:  45000,
			CreatedAt:          now,
			UpdatedAt:          now,
			CreatedBy:          "system_seed",
		},
		{
			SupplierCode:       "SERVOMEX",
			SupplierName:       "Servomex Group Ltd",
			Country:            "United Kingdom",
			Website:            "www.servomex.com",
			Email:              "sales@servomex.com",
			PaymentTermsDays:   60,
			LeadTimeDays:       60,
			MinimumOrderQty:    1,
			IsAuthorizedDealer: true,
			IsASPCertified:     false,
			GCCAuthorization:   true,
			CreditLimitBHD:     150000,
			CurrentBalanceBHD:  28000,
			CreatedAt:          now,
			UpdatedAt:          now,
			CreatedBy:          "system_seed",
		},
		{
			SupplierCode:       "ABB",
			SupplierName:       "ABB Ltd (Competitor)",
			Country:            "Switzerland",
			Website:            "www.abb.com",
			Email:              "contact@abb.com",
			PaymentTermsDays:   60,
			LeadTimeDays:       90,
			MinimumOrderQty:    100, // Won't do small orders (weakness!)
			IsAuthorizedDealer: false,
			IsASPCertified:     false,
			GCCAuthorization:   true,
			CreditLimitBHD:     0,
			CurrentBalanceBHD:  0,
			CreatedAt:          now,
			UpdatedAt:          now,
			CreatedBy:          "system_seed",
		},
	}

	for _, supplier := range suppliers {
		if err := db.Create(&supplier).Error; err != nil {
			log.Printf("Failed to create supplier %s: %v", supplier.SupplierCode, err)
		} else {
			fmt.Printf("  ✓ %s - %s\n", supplier.SupplierCode, supplier.SupplierName)
		}
	}

	// ======================================================================
	// SEED CUSTOMERS (Realistic PH Holdings clients)
	// ======================================================================

	fmt.Println("\nSeeding Customers...")

	// Customer types in Bahrain/GCC
	customerData := []struct {
		ID            string
		Type          string
		Name          string
		ShortCode     string
		City          string
		Country       string
		Industry      string
		Years         int
		Grade         string
		Terms         int
		AvgDays       float64
		Disputes      int
		OrdersValue   float64
		OrdersCount   int
		HasABB        bool
		EmergencyOnly bool
	}{
		// Grade A Customers (Excellent - 4 customers)
		{"CO1", "End Customer", "Bahrain Petroleum Company (BAPCO)", "CO", "Sitra", "Bahrain", "Oil & Gas", 8, "A", 30, 28.5, 0, 850000, 45, false, false},
		{"CO5", "End Customer", "National Oil & Gas Authority (NOGA)", "CO", "Manama", "Bahrain", "Oil & Gas", 6, "A", 30, 32.0, 0, 620000, 32, false, false},
		{"EC3", "Engineering Company", "Saudi Engineering Services", "EC", "Dammam", "Saudi Arabia", "EPC Contractor", 5, "A", 45, 42.0, 0, 480000, 28, false, false},
		{"EP1", "EPC Contractor", "Petrofac Engineering", "EP", "Manama", "Bahrain", "EPC Contractor", 7, "A", 60, 58.0, 0, 920000, 38, false, false},

		// Grade B Customers (Good - 8 customers)
		{"CO2", "End Customer", "Aluminum Bahrain (ALBA)", "CO", "Askar", "Bahrain", "Metals & Mining", 5, "B", 45, 52.0, 1, 380000, 22, true, false},
		{"CO8", "End Customer", "Gulf Petrochemical Industries", "CO", "Sitra", "Bahrain", "Petrochemicals", 4, "B", 30, 48.0, 0, 290000, 18, false, false},
		{"EC7", "Engineering Company", "Al-Khobar Engineering Co.", "EC", "Al Khobar", "Saudi Arabia", "Industrial Automation", 3, "B", 45, 50.0, 0, 185000, 14, true, false},
		{"EC12", "Engineering Company", "Bahrain Engineering Services", "EC", "Manama", "Bahrain", "Industrial Automation", 6, "B", 60, 65.0, 1, 420000, 26, false, false},
		{"EP4", "EPC Contractor", "Tecnicas Reunidas Middle East", "EP", "Abu Dhabi", "UAE", "EPC Contractor", 4, "B", 60, 68.0, 0, 550000, 19, false, false},
		{"CO15", "End Customer", "Qatar Petrochemical Company", "CO", "Doha", "Qatar", "Petrochemicals", 3, "B", 45, 54.0, 1, 320000, 16, true, false},
		{"EC18", "Engineering Company", "Middle East Engineering", "EC", "Manama", "Bahrain", "Industrial Automation", 5, "B", 45, 56.0, 0, 240000, 15, false, false},
		{"CO22", "End Customer", "Tatweer Petroleum", "CO", "Manama", "Bahrain", "Oil & Gas", 2, "B", 30, 44.0, 0, 180000, 11, false, false},

		// Grade C Customers (Fair - 12 customers)
		{"CO3", "End Customer", "Gulf Industrial Services", "CO", "Manama", "Bahrain", "Manufacturing", 3, "C", 30, 58.0, 2, 145000, 12, true, false},
		{"EC9", "Engineering Company", "Al-Rashid Engineering", "EC", "Riyadh", "Saudi Arabia", "Industrial Automation", 2, "C", 45, 62.0, 1, 95000, 8, false, false},
		{"CO11", "End Customer", "Bahrain Steel Company", "CO", "Hidd", "Bahrain", "Metals & Mining", 4, "C", 45, 68.0, 2, 175000, 14, true, false},
		{"EC14", "Engineering Company", "Kuwait Engineering Group", "EC", "Kuwait City", "Kuwait", "EPC Contractor", 2, "C", 60, 75.0, 1, 125000, 9, false, true},
		{"CO17", "End Customer", "Oman Refineries Company", "CO", "Muscat", "Oman", "Oil & Gas", 3, "C", 30, 52.0, 1, 210000, 13, false, false},
		{"EC20", "Engineering Company", "United Gulf Engineering", "EC", "Manama", "Bahrain", "Industrial Automation", 2, "C", 45, 64.0, 2, 88000, 7, true, false},
		{"CO24", "End Customer", "Delmon Poultry Company", "CO", "Hamad Town", "Bahrain", "Food Processing", 5, "C", 30, 56.0, 3, 62000, 18, false, false},
		{"EC25", "Engineering Company", "Arabian Engineering Solutions", "EC", "Dammam", "Saudi Arabia", "Industrial Automation", 1, "C", 60, 78.0, 1, 72000, 5, false, false},
		{"CO28", "End Customer", "Bahrain Tourism Company", "CO", "Manama", "Bahrain", "Hospitality", 2, "C", 45, 70.0, 2, 45000, 6, false, false},
		{"EC31", "Engineering Company", "Gulf Tech Industries", "EC", "Abu Dhabi", "UAE", "Industrial Automation", 1, "C", 45, 66.0, 1, 58000, 4, true, false},
		{"CO33", "End Customer", "National Flour Mills", "CO", "Manama", "Bahrain", "Food Processing", 4, "C", 30, 62.0, 2, 92000, 11, false, false},
		{"CO36", "End Customer", "Bahrain Airport Services", "CO", "Muharraq", "Bahrain", "Aviation", 3, "C", 45, 72.0, 1, 105000, 8, false, false},

		// Grade D Customers (Risky - 6 customers)
		{"EC10", "Engineering Company", "Al-Jazira Engineering", "EC", "Riyadh", "Saudi Arabia", "Industrial Automation", 1, "D", 60, 95.0, 3, 42000, 3, false, true},
		{"CO13", "End Customer", "Bahrain Chemical Company", "CO", "Sitra", "Bahrain", "Chemicals", 2, "D", 30, 88.0, 4, 68000, 5, true, false},
		{"EC16", "Engineering Company", "Desert Engineering LLC", "EC", "Dubai", "UAE", "EPC Contractor", 1, "D", 45, 102.0, 2, 35000, 2, false, true},
		{"CO19", "End Customer", "Muharraq Trading Company", "CO", "Muharraq", "Bahrain", "Trading", 2, "D", 30, 92.0, 5, 28000, 4, false, false},
		{"EC27", "Engineering Company", "New Horizon Engineering", "EC", "Kuwait City", "Kuwait", "Industrial Automation", 1, "D", 60, 110.0, 2, 22000, 2, false, true},
		{"CO30", "End Customer", "Riffa Industrial Supplies", "CO", "Riffa", "Bahrain", "Manufacturing", 1, "D", 45, 98.0, 3, 18000, 3, true, false},
	}

	for _, c := range customerData {
		lastOrder := now.AddDate(0, 0, -rand.Intn(30))
		avgOrder := c.OrdersValue / float64(c.OrdersCount)

		customer := CustomerMaster{
			CustomerID:         c.ID,
			CustomerType:       c.Type,
			BusinessName:       c.Name,
			ShortCode:          c.ShortCode,
			City:               c.City,
			Country:            c.Country,
			Industry:           c.Industry,
			RelationYears:      c.Years,
			PaymentGrade:       c.Grade,
			PaymentTermsDays:   c.Terms,
			AvgPaymentDays:     c.AvgDays,
			DisputeCount:       c.Disputes,
			TotalOrdersValue:   c.OrdersValue,
			TotalOrdersCount:   c.OrdersCount,
			AvgOrderValue:      avgOrder,
			LastOrderDate:      &lastOrder,
			HasABBCompetition:  c.HasABB,
			IsEmergencyOnly:    c.EmergencyOnly,
			IsCreditBlocked:    c.Grade == "D" && c.Disputes > 2,
			RequiresPrepayment: c.Grade == "D",
			CreatedAt:          now,
			UpdatedAt:          now,
			CreatedBy:          "system_seed",
		}

		if err := db.Create(&customer).Error; err != nil {
			log.Printf("Failed to create customer %s: %v", c.ID, err)
		} else {
			fmt.Printf("  ✓ %s [Grade %s] - %s\n", c.ID, c.Grade, c.Name)
		}
	}

	// ======================================================================
	// SEED PRODUCTS (Common E+H Products)
	// ======================================================================

	fmt.Println("\nSeeding Products...")

	// Get supplier IDs
	var ehSupplier, lgSupplier, servomexSupplier SupplierMaster
	db.Where("supplier_code = ?", "EH").First(&ehSupplier)
	db.Where("supplier_code = ?", "LG").First(&lgSupplier)
	db.Where("supplier_code = ?", "SERVOMEX").First(&servomexSupplier)

	products := []ProductMaster{
		// Flow Meters (E+H Coriolis)
		{ProductCode: "8F5B50-AAABBBA1A", ProductName: "Proline Promass F 500 Coriolis Flowmeter", ProductCategory: "Flow Measurement", SupplierID: ehSupplier.ID, SupplierCode: "EH", Description: "High-accuracy Coriolis mass flowmeter for liquids and gases", StandardCostBHD: 3200, StandardPriceBHD: 4800, StandardMargin: 50.0, ABBEquivalent: "CoriolisMaster FCB400", IsActive: true, IsStocked: true, StockQuantity: 3, CreatedAt: now},
		{ProductCode: "8F5B80-AAABBBA1A", ProductName: "Proline Promass E 500 Coriolis Flowmeter", ProductCategory: "Flow Measurement", SupplierID: ehSupplier.ID, SupplierCode: "EH", Description: "Compact Coriolis flowmeter with integrated temperature measurement", StandardCostBHD: 2800, StandardPriceBHD: 4200, StandardMargin: 50.0, ABBEquivalent: "CoriolisMaster FCB330", IsActive: true, IsStocked: true, StockQuantity: 2, CreatedAt: now},
		{ProductCode: "8F2B50-AAABBBA1A", ProductName: "Proline Promag W 500 Electromagnetic Flowmeter", ProductCategory: "Flow Measurement", SupplierID: ehSupplier.ID, SupplierCode: "EH", Description: "Wafer-style electromagnetic flowmeter for water and wastewater", StandardCostBHD: 1200, StandardPriceBHD: 1800, StandardMargin: 50.0, ABBEquivalent: "ProcessMaster FEP300", IsActive: true, IsStocked: true, StockQuantity: 5, CreatedAt: now},

		// Level Measurement (E+H Radar)
		{ProductCode: "FMR67-AAACCCA1A", ProductName: "Micropilot FMR67 Radar Level Transmitter", ProductCategory: "Level Measurement", SupplierID: ehSupplier.ID, SupplierCode: "EH", Description: "2-wire 80 GHz radar for challenging applications", StandardCostBHD: 1800, StandardPriceBHD: 2700, StandardMargin: 50.0, ABBEquivalent: "Radar Master FMR130", IsActive: true, IsStocked: true, StockQuantity: 4, CreatedAt: now},
		{ProductCode: "FMR62-AAACCCA1A", ProductName: "Micropilot FMR62 Radar Level Transmitter", ProductCategory: "Level Measurement", SupplierID: ehSupplier.ID, SupplierCode: "EH", Description: "Basic 2-wire radar for simple applications", StandardCostBHD: 980, StandardPriceBHD: 1470, StandardMargin: 50.0, ABBEquivalent: "Radar Master FMR120", IsActive: true, IsStocked: true, StockQuantity: 6, CreatedAt: now},

		// Pressure Transmitters (E+H)
		{ProductCode: "PMC71-AAABBBA1A", ProductName: "Cerabar PMC71 Pressure Transmitter", ProductCategory: "Pressure Measurement", SupplierID: ehSupplier.ID, SupplierCode: "EH", Description: "Ceramic sensor for gauge/absolute pressure", StandardCostBHD: 420, StandardPriceBHD: 630, StandardMargin: 50.0, ABBEquivalent: "PressMaster PMD230", IsActive: true, IsStocked: true, StockQuantity: 10, CreatedAt: now},
		{ProductCode: "PMP71-AAABBBA1A", ProductName: "Cerabar PMP71 Pressure Transmitter", ProductCategory: "Pressure Measurement", SupplierID: ehSupplier.ID, SupplierCode: "EH", Description: "Absolute/gauge pressure with flush diaphragm", StandardCostBHD: 580, StandardPriceBHD: 870, StandardMargin: 50.0, ABBEquivalent: "PressMaster PMD235", IsActive: true, IsStocked: true, StockQuantity: 8, CreatedAt: now},

		// Temperature (E+H)
		{ProductCode: "TMT162-AAABBBA1A", ProductName: "iTHERM TM162 Modular Temperature Transmitter", ProductCategory: "Temperature Measurement", SupplierID: ehSupplier.ID, SupplierCode: "EH", Description: "Modular 2-wire transmitter with HART", StandardCostBHD: 320, StandardPriceBHD: 480, StandardMargin: 50.0, ABBEquivalent: "TempMaster TMT142", IsActive: true, IsStocked: true, StockQuantity: 12, CreatedAt: now},

		// Gas Analyzers (Servomex)
		{ProductCode: "SERVOPRO4900-MH", ProductName: "SERVOPRO 4900 Multigas Analyzer", ProductCategory: "Gas Analysis", SupplierID: servomexSupplier.ID, SupplierCode: "SERVOMEX", Description: "Portable multigas analyzer - %vol O2, %vol CO2, ppm CO, ppm H2S", StandardCostBHD: 8500, StandardPriceBHD: 12750, StandardMargin: 50.0, ABBEquivalent: "EasyLine EL3000", IsActive: true, IsStocked: false, StockQuantity: 0, CreatedAt: now},
		{ProductCode: "SERVOPRO-MonoExact", ProductName: "SERVOPRO MonoExact DF310E", ProductCategory: "Gas Analysis", SupplierID: servomexSupplier.ID, SupplierCode: "SERVOMEX", Description: "Single-component O2 analyzer with paramagnetic sensor", StandardCostBHD: 5200, StandardPriceBHD: 7800, StandardMargin: 50.0, ABBEquivalent: "Magnos206", IsActive: true, IsStocked: false, StockQuantity: 0, CreatedAt: now},

		// LG Electronics (Industrial Displays/HMI)
		{ProductCode: "LG-IPS55-4K", ProductName: "LG 55-inch 4K Industrial Display", ProductCategory: "Industrial Display", SupplierID: lgSupplier.ID, SupplierCode: "LG", Description: "55-inch 4K IPS panel for control rooms", StandardCostBHD: 420, StandardPriceBHD: 630, StandardMargin: 50.0, ABBEquivalent: "ABB Panel800", IsActive: true, IsStocked: true, StockQuantity: 4, CreatedAt: now},
		{ProductCode: "LG-IPS43-FHD", ProductName: "LG 43-inch FHD Industrial Display", ProductCategory: "Industrial Display", SupplierID: lgSupplier.ID, SupplierCode: "LG", Description: "43-inch Full HD panel for operator stations", StandardCostBHD: 280, StandardPriceBHD: 420, StandardMargin: 50.0, ABBEquivalent: "ABB Panel600", IsActive: true, IsStocked: true, StockQuantity: 6, CreatedAt: now},
	}

	for _, product := range products {
		if err := db.Create(&product).Error; err != nil {
			log.Printf("Failed to create product %s: %v", product.ProductCode, err)
		} else {
			fmt.Printf("  ✓ %s - %s\n", product.ProductCode, product.ProductName)
		}
	}

	// ======================================================================
	// FINAL SUMMARY
	// ======================================================================

	db.Model(&CustomerMaster{}).Count(&customerCount)
	db.Model(&SupplierMaster{}).Count(&supplierCount)
	db.Model(&ProductMaster{}).Count(&productCount)

	fmt.Println("\n=== Seeding Complete! ===")
	fmt.Printf("\nFinal Counts:\n")
	fmt.Printf("  Customers: %d (4 Grade A, 8 Grade B, 12 Grade C, 6 Grade D)\n", customerCount)
	fmt.Printf("  Suppliers: %d (E+H, LG, SERVOMEX, ABB)\n", supplierCount)
	fmt.Printf("  Products:  %d (Flow, Level, Pressure, Temp, Gas Analysis)\n", productCount)
	fmt.Println("\n🎉 PH Holdings master data deployed!")
}
