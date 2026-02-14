package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Database Structs (matching database.go)
type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
	CreatedBy string
}

func (b *Base) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

type CustomerMaster struct {
	Base
	CustomerID    string `gorm:"uniqueIndex;size:50"`
	CustomerCode  string `gorm:"uniqueIndex;size:50"`
	CustomerType  string `gorm:"index;size:50"`
	BusinessName  string `gorm:"index;size:255"`
	ShortCode     string `gorm:"size:10"`
	AddressLine1  string
	City          string `gorm:"index;size:100"`
	Country       string `gorm:"index;size:100"`
	TRN           string `gorm:"size:100"`
	Industry      string `gorm:"index;size:100"`
	RelationYears int

	PaymentGrade     string  `gorm:"index;size:10"`
	CustomerGrade    string  `gorm:"index;size:10"`
	PaymentTermsDays int
	AvgPaymentDays   float64
	DisputeCount     int

	TotalOrdersValue float64
	TotalOrdersCount int
	AvgOrderValue    float64
	LastOrderDate    *time.Time

	ARRiskTier     string  `gorm:"index;size:20"`
	OutstandingBHD float64
	OverdueDays    int     `gorm:"index"`

	IsCreditBlocked    bool `gorm:"index;default:false"`
	RequiresPrepayment bool `gorm:"default:false"`
	HasABBCompetition  bool `gorm:"default:false"`
	IsEmergencyOnly    bool `gorm:"default:false"`
}

func (CustomerMaster) TableName() string { return "customers" }

type CustomerContact struct {
	Base
	CustomerID       string `gorm:"index;size:36"`
	ContactName      string
	JobTitle         string
	Email            string
	Phone            string
	Address          string `gorm:"type:text"`
	IsPrimaryContact bool
}

func (CustomerContact) TableName() string { return "customer_contacts" }

type SupplierMaster struct {
	Base
	SupplierCode   string `gorm:"uniqueIndex;size:50"`
	SupplierName   string `gorm:"index;size:255"`
	Country        string `gorm:"index;size:100"`
	LeadTimeDays   int
	TaxID          string `gorm:"size:100"`
	SupplierType   string `gorm:"index;size:50"`
	BrandsHandled  string `gorm:"type:text"`
	ProductTypes   string `gorm:"type:text"`
	PrimaryContact string
	Email          string
	Phone          string
	Address        string `gorm:"type:text"`
	BankName       string
	AccountNumber  string
	IBAN           string `gorm:"size:50"`
	SwiftCode      string `gorm:"size:20"`
	PaymentTerms   string `gorm:"size:100;default:'Net 30'"`
	Rating         int    `gorm:"default:3"`
	Notes          string `gorm:"type:text"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

type Opportunity struct {
	Base
	FolderNumber string `gorm:"uniqueIndex;size:50"`
	OfferID      string `gorm:"index;size:36"`
	CustomerID   string `gorm:"index;size:36"`
	CustomerName string `gorm:"index;size:255"`
	Salesperson  string `gorm:"index;size:100"`
	OfferDate    time.Time `gorm:"index"`
	ExpectedDate *time.Time
	ClosedDate   *time.Time
	RevenueBHD   float64
	CostBHD      float64
	ProfitBHD    float64
	SPOCStatus   string `gorm:"index;size:50"`
	WIPStatus    string `gorm:"index;size:50"`
	Stage        string `gorm:"index;size:50"`
	Regime       int    `gorm:"index"`
	Confidence   float64
	R1           float64
	R2           float64
	R3           float64
	HasABBCompetition bool
	ProductType  string `gorm:"index;size:100"`
	WonReason    string `gorm:"type:text"`
	LostReason   string `gorm:"type:text"`
}

func (Opportunity) TableName() string { return "opportunities" }

type OpportunityData struct {
	Year         int
	OppNumber    int
	FolderNumber int
	FolderName   string
	EndUser      string
	Status       string
	ValueBHD     float64
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	// Default paths (relative to ph_holdings_sovereign_ui directory)
	opportunitiesFile := `Data_for_database\opportunities created 2025.xlsx`
	dbFile := "ph_holdings.db"

	if len(os.Args) > 1 {
		opportunitiesFile = os.Args[1]
	}
	if len(os.Args) > 2 {
		dbFile = os.Args[2]
	}

	fmt.Println("🚀 Seeding database from Opportunities Excel...")
	fmt.Printf("📄 Opportunities file: %s\n", opportunitiesFile)
	fmt.Printf("🗄️ Database file: %s\n", dbFile)

	// Load opportunities from Excel
	opportunities, err := loadOpportunities(opportunitiesFile)
	if err != nil {
		log.Fatalf("Failed to load opportunities: %v", err)
	}
	fmt.Printf("📊 Loaded %d opportunities\n", len(opportunities))

	// Connect to database
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate models
	if err := db.AutoMigrate(
		&CustomerMaster{},
		&CustomerContact{},
		&SupplierMaster{},
		&Opportunity{},
	); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	// Seed customers from opportunities
	customersCreated := seedCustomers(db, opportunities)
	fmt.Printf("✅ Created %d customers\n", customersCreated)

	// Seed opportunities
	opportunitiesCreated := seedOpportunities(db, opportunities)
	fmt.Printf("✅ Created %d opportunities\n", opportunitiesCreated)

	// Seed default suppliers
	suppliersCreated := seedSuppliers(db)
	fmt.Printf("✅ Created %d suppliers\n", suppliersCreated)

	// Summary
	fmt.Println("\n📈 Seeding Summary:")
	fmt.Printf("   Customers: %d\n", customersCreated)
	fmt.Printf("   Opportunities: %d\n", opportunitiesCreated)
	fmt.Printf("   Suppliers: %d\n", suppliersCreated)
	fmt.Println("\n✨ Database seeding complete!")

	// Copy to build location
	if err := deployDatabase(dbFile); err != nil {
		fmt.Printf("⚠️ Warning: Could not deploy database: %v\n", err)
	}
}

func loadOpportunities(filePath string) ([]OpportunityData, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	// Get first sheet
	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read rows: %w", err)
	}

	var opportunities []OpportunityData
	for i, row := range rows {
		if i == 0 { // Skip header
			continue
		}
		if len(row) < 6 {
			continue
		}

		opp := OpportunityData{}

		// Parse Year (column 0: Year)
		if len(row) > 0 {
			fmt.Sscanf(row[0], "%d", &opp.Year)
		}

		// Parse Opp. No. (column 1: Opp. No.)
		if len(row) > 1 {
			fmt.Sscanf(row[1], "%d", &opp.OppNumber)
		}

		// Parse Folder No. (column 2: Folder No.)
		if len(row) > 2 {
			folderStr := strings.TrimSpace(row[2])
			// Handle "5-24" format vs "5"
			if strings.Contains(folderStr, "-") {
				parts := strings.Split(folderStr, "-")
				fmt.Sscanf(parts[0], "%d", &opp.FolderNumber)
			} else {
				fmt.Sscanf(folderStr, "%d", &opp.FolderNumber)
			}
		}

		// Folder Name (column 3: Folder Name)
		if len(row) > 3 {
			opp.FolderName = strings.TrimSpace(row[3])
		}

		// End User (column 5: End User) - skip SFDC column at 4
		if len(row) > 5 {
			opp.EndUser = strings.TrimSpace(row[5])
		}

		// Value in BHD (column 8: Value in BHD)
		if len(row) > 8 {
			valueStr := strings.ReplaceAll(row[8], ",", "")
			valueStr = strings.TrimSpace(valueStr)
			fmt.Sscanf(valueStr, "%f", &opp.ValueBHD)
		}

		// Status (column 9: Status)
		if len(row) > 9 {
			opp.Status = strings.TrimSpace(row[9])
		}

		if opp.EndUser != "" || opp.FolderName != "" {
			opportunities = append(opportunities, opp)
		}
	}

	return opportunities, nil
}

func seedCustomers(db *gorm.DB, opportunities []OpportunityData) int {
	customerNames := make(map[string]bool)
	for _, opp := range opportunities {
		name := normalizeCustomerName(opp.EndUser)
		if name != "" && name != "Unknown" {
			customerNames[name] = true
		}
	}

	created := 0
	for name := range customerNames {
		// Check if exists
		var existing CustomerMaster
		if err := db.Where("business_name = ?", name).First(&existing).Error; err == nil {
			continue
		}

		// Generate customer code
		code := strings.ToUpper(strings.ReplaceAll(name, " ", ""))
		if len(code) > 8 {
			code = code[:8]
		}
		code = code + "-" + uuid.New().String()[:4]

		customer := CustomerMaster{
			CustomerID:    uuid.New().String(),
			CustomerCode:  code,
			BusinessName:  name,
			CustomerType:  "Industrial",
			Country:       "Bahrain",
			City:          "Manama",
			CustomerGrade: "B",
			PaymentGrade:  "B",
		}

		if err := db.Create(&customer).Error; err == nil {
			created++
		}
	}

	return created
}

func seedOpportunities(db *gorm.DB, opportunities []OpportunityData) int {
	created := 0
	for _, opp := range opportunities {
		folderNumber := fmt.Sprintf("%d-%d", opp.Year, opp.FolderNumber)
		if opp.FolderNumber == 0 {
			folderNumber = fmt.Sprintf("%d-OPP-%d", opp.Year, opp.OppNumber)
		}

		// Check if exists
		var existing Opportunity
		if err := db.Where("folder_number = ?", folderNumber).First(&existing).Error; err == nil {
			continue
		}

		// Determine stage based on actual status values from Excel
		stage := "Lead"
		statusUpper := strings.ToUpper(opp.Status)

		if strings.Contains(statusUpper, "PAYMENT RECEIVED") || strings.Contains(statusUpper, "PO RECEIVED") {
			stage = "Won"
		} else if strings.Contains(statusUpper, "LOST") || strings.Contains(statusUpper, "CLOSED") && strings.Contains(statusUpper, "LOST") {
			stage = "Lost"
		} else if strings.Contains(statusUpper, "IN PROCESS") || strings.Contains(statusUpper, "FINAL STAGE") {
			stage = "Proposal"
		} else if strings.Contains(statusUpper, "OFFER SUBMITTED") {
			stage = "Quoted"
		} else if strings.Contains(statusUpper, "NEGOTIATION") {
			stage = "Negotiation"
		}

		// Find customer ID
		customerName := normalizeCustomerName(opp.EndUser)
		var customer CustomerMaster
		db.Where("business_name = ?", customerName).First(&customer)

		opportunity := Opportunity{
			FolderNumber: folderNumber,
			CustomerID:   customer.ID,
			CustomerName: customerName,
			Stage:        stage,
			RevenueBHD:   opp.ValueBHD,
			OfferDate:    time.Date(opp.Year, time.January, 1, 0, 0, 0, 0, time.UTC),
		}

		if err := db.Create(&opportunity).Error; err == nil {
			created++
		}
	}

	return created
}

func seedSuppliers(db *gorm.DB) int {
	suppliers := []struct {
		Name         string
		Code         string
		Country      string
		PaymentTerms string
		Brands       string
	}{
		{"Endress+Hauser", "EH", "Germany", "Net 60", "E+H Flow, E+H Level, E+H Pressure"},
		{"Servomex", "SRVX", "UK", "Net 45", "Servomex Gas Analyzers"},
		{"Siemens", "SIEM", "Germany", "Net 60", "Siemens Automation, Sitrans"},
		{"Emerson", "EMER", "USA", "Net 45", "Rosemount, Fisher, DeltaV"},
		{"ABB", "ABB", "Switzerland", "Net 45", "ABB Instrumentation, Drives"},
		{"Yokogawa", "YOKO", "Japan", "Net 60", "DPharp, ADMAG, Centum VP"},
		{"Honeywell", "HON", "USA", "Net 45", "Honeywell Sensing, ST Series"},
		{"Landis+Gyr", "LG", "Switzerland", "Net 60", "Metering Solutions"},
		{"SICK AG", "SICK", "Germany", "Net 45", "Sensors, Encoders"},
		{"Pepperl+Fuchs", "PF", "Germany", "Net 45", "Intrinsic Safety, Sensors"},
	}

	created := 0
	for _, s := range suppliers {
		var existing SupplierMaster
		if err := db.Where("supplier_name = ?", s.Name).First(&existing).Error; err == nil {
			continue
		}

		supplier := SupplierMaster{
			SupplierCode:  s.Code,
			SupplierName:  s.Name,
			Country:       s.Country,
			PaymentTerms:  s.PaymentTerms,
			BrandsHandled: s.Brands,
			SupplierType:  "Manufacturer",
			Rating:        5,
		}

		if err := db.Create(&supplier).Error; err == nil {
			created++
		}
	}

	return created
}

func normalizeCustomerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "Unknown"
	}

	upper := strings.ToUpper(name)

	// Map common abbreviations to full names
	mappings := map[string]string{
		"BAPCO":    "Bapco Energies",
		"ALBA":     "Aluminium Bahrain (ALBA)",
		"EWA":      "Electricity & Water Authority",
		"GPIC":     "Gulf Petrochemical Industries Co.",
		"SULB":     "SULB Company",
		"ASRY":     "Arab Shipbuilding & Repair Yard",
		"GARMCO":   "Gulf Aluminium Rolling Mill",
		"GIIC":     "Gulf Industrial Investment Co.",
		"TATWEER":  "Tatweer Petroleum",
		"MOW":      "Ministry of Works",
	}

	for abbr, fullName := range mappings {
		if strings.Contains(upper, abbr) {
			return fullName
		}
	}

	// Title case the name
	return strings.Title(strings.ToLower(name))
}

func deployDatabase(dbFile string) error {
	deployPath := "build/bin/data"
	if err := os.MkdirAll(deployPath, 0755); err != nil {
		return err
	}

	targetDB := deployPath + "/ph_holdings.db"
	input, err := os.ReadFile(dbFile)
	if err != nil {
		return err
	}

	if err := os.WriteFile(targetDB, input, 0644); err != nil {
		return err
	}

	fmt.Printf("✅ Database deployed to: %s\n", targetDB)
	return nil
}
