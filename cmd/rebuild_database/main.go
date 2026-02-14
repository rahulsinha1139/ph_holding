package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// =============================================================================
// MODELS (Simplified for import)
// =============================================================================

type Base struct {
	ID        string         `gorm:"primaryKey;size:36"`
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
	CustomerID       string  `gorm:"uniqueIndex;size:50"`
	CustomerCode     string  `gorm:"uniqueIndex;size:50"`
	CustomerType     string  `gorm:"index;size:50"`
	BusinessName     string  `gorm:"index;size:255"`
	ShortCode        string  `gorm:"size:10"`
	AddressLine1     string
	City             string  `gorm:"index;size:100"`
	Country          string  `gorm:"index;size:100"`
	TRN              string  `gorm:"size:100"`
	Industry         string  `gorm:"index;size:100"`
	RelationYears    int
	PaymentGrade     string  `gorm:"index;size:10;default:'C'"`
	CustomerGrade    string  `gorm:"index;size:10;default:'C'"`
	PaymentTermsDays int     `gorm:"index;default:30"`
	AvgPaymentDays   float64
	DisputeCount     int
	TotalOrdersValue float64
	TotalOrdersCount int
	AvgOrderValue    float64
	LastOrderDate    *time.Time
	ARRiskTier       string  `gorm:"index;size:20"`
	OutstandingBHD   float64
	OverdueDays      int     `gorm:"index"`
	CreditLimitBHD   float64 `gorm:"default:50000"`
	IsCreditBlocked  bool    `gorm:"index;default:false"`
}

func (CustomerMaster) TableName() string { return "customers" }

type SupplierMaster struct {
	Base
	SupplierCode   string `gorm:"uniqueIndex;size:50"`
	SupplierName   string `gorm:"index;size:255"`
	Country        string `gorm:"index;size:100"`
	LeadTimeDays   int
	TaxID          string `gorm:"size:100"`
	SupplierType   string `gorm:"index;size:50"`
	BrandsHandled  string `gorm:"type:varchar(2000)"`
	ProductTypes   string `gorm:"type:varchar(2000)"`
	PrimaryContact string `gorm:"size:255"`
	Email          string `gorm:"size:255"`
	Phone          string `gorm:"size:50"`
	Address        string `gorm:"type:varchar(1000)"`
	BankName       string
	AccountNumber  string
	IBAN           string `gorm:"size:50"`
	SwiftCode      string `gorm:"size:20"`
	PaymentTerms   string `gorm:"index;size:100;default:'Net 30'"`
	Rating         int    `gorm:"index;default:3"`
	Notes          string `gorm:"type:varchar(5000)"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

type Opportunity struct {
	Base
	FolderNumber     string     `gorm:"uniqueIndex;size:50"`
	OfferID          string     `gorm:"index;size:36"`
	CustomerID       string     `gorm:"index;size:36"`
	CustomerName     string     `gorm:"index;size:255"`
	Salesperson      string     `gorm:"index;size:100"`
	OfferDate        time.Time  `gorm:"index"`
	ExpectedDate     *time.Time
	ClosedDate       *time.Time
	EstimatedValue   float64    `gorm:"default:0"`
	Stage            string     `gorm:"index;size:50;default:'Open'"`
	WinProbability   float64    `gorm:"default:0"`
	Notes            string     `gorm:"type:varchar(5000)"`
	PHInvoiceNo      string     `gorm:"size:50"`
	EHInvoiceNo      string     `gorm:"size:50"`
	RevenueBHD       float64    `gorm:"default:0"`
	CostBHD          float64    `gorm:"default:0"`
	ProfitBHD        float64    `gorm:"default:0"`
	ProfitPercent    float64    `gorm:"default:0"`
	HasDeliveryNote  bool       `gorm:"default:false"`
	HasCostingSheet  bool       `gorm:"default:false"`
	HasInvoice       bool       `gorm:"default:false"`
}

func (Opportunity) TableName() string { return "opportunities" }

type Offer struct {
	Base
	OfferNumber     string    `gorm:"uniqueIndex;size:50"`
	RevisionNumber  int       `gorm:"index"`
	RFQID           string    `gorm:"index;size:36"`
	CustomerID      string    `gorm:"index;size:36"`
	CustomerName    string    `gorm:"index;size:255"`
	QuotationDate   time.Time `gorm:"index"`
	ValidityDate    time.Time
	TotalValueBHD   float64   `gorm:"default:0"`
	EstimatedMargin float64   `gorm:"default:0"`
	Stage           string    `gorm:"index;size:50;default:'Quoted'"`
	LostReason      string    `gorm:"type:varchar(2000)"`
	PaymentTerms    string    `gorm:"type:varchar(500);default:'30 days from Date of Delivery'"`
	DeliveryTerms   string    `gorm:"type:varchar(500);default:'DAP Bahrain'"`
	DeliveryWeeks   string    `gorm:"size:50;default:'4-6 weeks'"`
	CountryOfOrigin string    `gorm:"size:100;default:'Germany / USA'"`
	IssuedBy        string    `gorm:"size:100"`
}

func (Offer) TableName() string { return "offers" }

type Invoice struct {
	Base
	InvoiceNumber        string    `gorm:"uniqueIndex;size:50"`
	InvoiceDate          time.Time `gorm:"index"`
	CustomerID           string    `gorm:"index;size:36"`
	CustomerName         string    `gorm:"index;size:255"`
	OrderID              string    `gorm:"index;size:36"`
	CustomerPONumber     string    `gorm:"index;size:100"`
	GrandTotalBHD        float64   `gorm:"type:decimal(15,3);default:0"`
	Status               string    `gorm:"index;size:50;default:'Sent'"`
	OutstandingBHD       float64   `gorm:"type:decimal(15,3);default:0"`
	SubtotalBHD          float64   `gorm:"type:decimal(15,3);default:0"`
	DueDate              time.Time `gorm:"index"`
	VATBHD               float64   `gorm:"column:vatbhd;type:decimal(15,3);default:0"`
	VATPercent           float64   `gorm:"default:0"`
	TotalSupplierCostBHD float64   `gorm:"default:0"`
	GrossMarginBHD       float64   `gorm:"default:0"`
	GrossMarginPercent   float64   `gorm:"default:0"`
}

func (Invoice) TableName() string { return "invoices" }

type SupplierPayment struct {
	Base
	SupplierID        string    `gorm:"index;size:36"`
	SupplierName      string    `gorm:"size:255"`
	SupplierInvoiceID string    `gorm:"index;size:36"`
	InvoiceNumber     string    `gorm:"size:100"`
	PaymentDate       time.Time `gorm:"index"`
	AmountBHD         float64   `gorm:"type:decimal(15,3);default:0"`
	AmountUSD         float64   `gorm:"type:decimal(15,2);default:0"`
	ExchangeRate      float64   `gorm:"default:1"`
	PaymentMethod     string    `gorm:"size:50"`
	Reference         string    `gorm:"size:100"`
	Notes             string    `gorm:"type:varchar(2000)"`
}

func (SupplierPayment) TableName() string { return "supplier_payments" }

type Role struct {
	Base
	Name        string `gorm:"uniqueIndex;size:50"`
	Permissions string `gorm:"type:varchar(2000)"`
	Description string `gorm:"type:varchar(500)"`
}

func (Role) TableName() string { return "roles" }

type Device struct {
	Base
	MachineID   string `gorm:"uniqueIndex;size:64"`
	DeviceName  string `gorm:"size:255"`
	Status      string `gorm:"index;size:20;default:'pending'"`
	ApprovedBy  string `gorm:"size:36"`
	ApprovedAt  *time.Time
	LastLoginAt *time.Time
}

func (Device) TableName() string { return "devices" }

type DeviceUser struct {
	Base
	DeviceID     string `gorm:"index;size:36"`
	Username     string `gorm:"index;size:100"`
	PasswordHash string `gorm:"size:255"`
	RoleID       string `gorm:"index;size:36"`
	RoleName     string `gorm:"size:50"`
	IsActive     bool   `gorm:"default:true"`
}

func (DeviceUser) TableName() string { return "device_users" }

func main() {
	log.Println("=== PH HOLDINGS DATABASE REBUILD ===")
	log.Println("Starting fresh database creation...")

	// Load .env
	godotenv.Load("../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	dsn := convertDatabaseURL(databaseURL)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// =======================================================================
	// STEP 1: DROP ALL TABLES
	// =======================================================================
	log.Println("\n=== STEP 1: Dropping all tables ===")
	dropAllTables(db)

	// =======================================================================
	// STEP 2: RECREATE SCHEMA
	// =======================================================================
	log.Println("\n=== STEP 2: Creating fresh schema ===")
	err = db.AutoMigrate(
		&CustomerMaster{},
		&SupplierMaster{},
		&Opportunity{},
		&Offer{},
		&Invoice{},
		&SupplierPayment{},
		&Role{},
		&Device{},
		&DeviceUser{},
	)
	if err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	log.Println("Schema created successfully")

	// =======================================================================
	// STEP 3: SEED ROLES
	// =======================================================================
	log.Println("\n=== STEP 3: Seeding roles ===")
	seedRoles(db)

	// =======================================================================
	// STEP 4: IMPORT ARCHAEOLOGY DATA (Customers + Opportunities)
	// =======================================================================
	log.Println("\n=== STEP 4: Importing archaeology data ===")
	importArchaeologyData(db, "../../Data_for_database/archaeology_data.csv")

	// =======================================================================
	// STEP 5: IMPORT INVOICES FROM TALLY
	// =======================================================================
	log.Println("\n=== STEP 5: Importing Tally invoices ===")
	importTallyInvoices(db, "../../Data_for_database/Tally_data/invoices 2025.xlsx")
	importTallyInvoices(db, "../../Data_for_database/Tally_data/invoices 2024.xlsx")
	importTallyInvoices(db, "../../Data_for_database/Tally_data/invoices 2023.xlsx")

	// =======================================================================
	// STEP 6: IMPORT SUPPLIER PAYMENTS
	// =======================================================================
	log.Println("\n=== STEP 6: Importing supplier payments ===")
	importSupplierPayments(db, "../../Data_for_database/Payments to suppliers.xlsx")

	// =======================================================================
	// STEP 7: SEED SUPPLIERS (E+H, Servomex, etc.)
	// =======================================================================
	log.Println("\n=== STEP 7: Seeding suppliers ===")
	seedSuppliers(db)

	// =======================================================================
	// SUMMARY
	// =======================================================================
	log.Println("\n=== DATABASE REBUILD COMPLETE ===")
	printSummary(db)
}

func dropAllTables(db *gorm.DB) {
	tables := []string{
		"device_users", "devices", "roles", "audit_logs", "settings",
		"chat_messages", "conversations",
		"supplier_payments", "supplier_invoice_items", "supplier_invoices",
		"grn_items", "goods_received_notes",
		"purchase_order_items", "purchase_orders",
		"delivery_note_items", "delivery_notes",
		"invoice_items", "invoices",
		"payments", "payment_predictions",
		"order_items", "orders",
		"offer_items", "offers", "offer_followups",
		"opportunities", "rfqs",
		"costing_additional_costs", "costing_items", "costing_sheets",
		"entity_notes", "supplier_issues",
		"customer_contacts", "supplier_contacts",
		"products", "customers", "suppliers",
		"sync_records", "tally_invoice_imports", "tally_purchase_imports",
		"ar_aging", "quick_captures", "alerts", "shipments", "post_sale_notes",
	}

	for _, table := range tables {
		db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
	}
	log.Printf("Dropped %d tables", len(tables))
}

func seedRoles(db *gorm.DB) {
	roles := []Role{
		{Base: Base{ID: uuid.New().String()}, Name: "admin", Permissions: `["*"]`, Description: "Full system access"},
		{Base: Base{ID: uuid.New().String()}, Name: "manager", Permissions: `["read","write","finance","operations","crm","reports"]`, Description: "Manager access with finance"},
		{Base: Base{ID: uuid.New().String()}, Name: "sales", Permissions: `["read","write","crm","offers","orders"]`, Description: "Sales team - no finance"},
		{Base: Base{ID: uuid.New().String()}, Name: "operations", Permissions: `["read","write","operations","grn","delivery"]`, Description: "Operations team - no finance"},
		{Base: Base{ID: uuid.New().String()}, Name: "staff", Permissions: `["read"]`, Description: "Read-only access"},
	}

	for _, role := range roles {
		db.Create(&role)
	}
	log.Printf("Created %d roles", len(roles))
}

func importArchaeologyData(db *gorm.DB, filepath string) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		log.Printf("Could not open archaeology data: %v", err)
		return
	}

	// Remove BOM if present
	content := string(data)
	content = strings.TrimPrefix(content, "\ufeff")

	reader := csv.NewReader(strings.NewReader(content))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("Could not parse CSV: %v", err)
		return
	}

	// Skip header
	if len(records) < 2 {
		log.Println("No data in archaeology CSV")
		return
	}

	// Create customers map to avoid duplicates
	customers := make(map[string]string) // name -> id
	opportunities := 0

	for i, record := range records[1:] {
		if len(record) < 16 {
			continue
		}

		folderNum := record[0]
		folderName := record[1]
		customerName := record[2]
		salesperson := record[3]
		phInvoice := record[4]
		ehInvoice := record[5]
		revenueBHD := parseFloat(record[6])
		costBHD := parseFloat(record[7])
		profitBHD := parseFloat(record[8])
		profitPct := parseFloat(record[9])
		hasDeliveryNote := record[11] == "True"
		hasCostingSheet := record[12] == "True"
		hasInvoice := record[13] == "True"

		// Create customer if not exists
		customerID := ""
		if customerName != "" {
			if existingID, ok := customers[customerName]; ok {
				customerID = existingID
			} else {
				customerID = uuid.New().String()
				customer := CustomerMaster{
					Base:         Base{ID: customerID},
					CustomerID:   fmt.Sprintf("CUST-%04d", i+1),
					CustomerCode: fmt.Sprintf("C%04d", i+1),
					BusinessName: customerName,
					Country:      "Bahrain",
					PaymentGrade: "C",
					CustomerGrade: "C",
				}
				db.Create(&customer)
				customers[customerName] = customerID
			}
		}

		// Determine stage
		stage := "Open"
		if hasInvoice {
			stage = "Won"
		} else if hasCostingSheet {
			stage = "Quoted"
		}

		// Create opportunity
		opp := Opportunity{
			Base:            Base{ID: uuid.New().String()},
			FolderNumber:    folderNum,
			CustomerID:      customerID,
			CustomerName:    customerName,
			Salesperson:     salesperson,
			OfferDate:       time.Now().AddDate(0, -3, 0), // Default to 3 months ago
			EstimatedValue:  revenueBHD,
			Stage:           stage,
			PHInvoiceNo:     phInvoice,
			EHInvoiceNo:     ehInvoice,
			RevenueBHD:      revenueBHD,
			CostBHD:         costBHD,
			ProfitBHD:       profitBHD,
			ProfitPercent:   profitPct,
			HasDeliveryNote: hasDeliveryNote,
			HasCostingSheet: hasCostingSheet,
			HasInvoice:      hasInvoice,
			Notes:           folderName,
		}
		db.Create(&opp)
		opportunities++
	}

	log.Printf("Imported %d customers, %d opportunities", len(customers), opportunities)
}

func importTallyInvoices(db *gorm.DB, filepath string) {
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		log.Printf("Could not open %s: %v", filepath, err)
		return
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Printf("Could not read rows: %v", err)
		return
	}

	if len(rows) < 2 {
		log.Println("No data rows")
		return
	}

	// Find column indices from header
	header := rows[0]
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	invoiceCount := 0
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}

		// Try to extract invoice data
		invoiceNum := getColValue(row, colMap, "invoice no", "invoice number", "voucher no")
		customerName := getColValue(row, colMap, "party name", "customer", "party")
		dateStr := getColValue(row, colMap, "date", "invoice date", "voucher date")
		amountStr := getColValue(row, colMap, "amount", "total", "gross total", "invoice amount")

		if invoiceNum == "" || invoiceNum == "0" {
			continue
		}

		// Parse date
		invoiceDate := parseDate(dateStr)
		amount := parseFloat(amountStr)

		// Look up customer
		var customer CustomerMaster
		db.Where("business_name LIKE ? ESCAPE '\\\\'", "%"+escapeLikeWildcards(customerName)+"%").First(&customer)

		inv := Invoice{
			Base:          Base{ID: uuid.New().String()},
			InvoiceNumber: invoiceNum,
			InvoiceDate:   invoiceDate,
			CustomerID:    customer.ID,
			CustomerName:  customerName,
			GrandTotalBHD: amount,
			SubtotalBHD:   amount / 1.1, // Assume 10% VAT
			VATBHD:        amount - (amount / 1.1),
			VATPercent:    10,
			OutstandingBHD: 0,
			Status:        "Paid",
			DueDate:       invoiceDate.AddDate(0, 0, 30),
		}

		result := db.Create(&inv)
		if result.Error == nil {
			invoiceCount++
		}
	}

	log.Printf("Imported %d invoices from %s", invoiceCount, filepath)
}

func importSupplierPayments(db *gorm.DB, filepath string) {
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		log.Printf("Could not open %s: %v", filepath, err)
		return
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Printf("Could not read rows: %v", err)
		return
	}

	if len(rows) < 2 {
		log.Println("No data rows")
		return
	}

	header := rows[0]
	colMap := make(map[string]int)
	for i, col := range header {
		colMap[strings.ToLower(strings.TrimSpace(col))] = i
	}

	paymentCount := 0
	for _, row := range rows[1:] {
		if len(row) == 0 {
			continue
		}

		supplierName := getColValue(row, colMap, "supplier", "party name", "vendor", "party")
		dateStr := getColValue(row, colMap, "date", "payment date", "voucher date")
		amountStr := getColValue(row, colMap, "amount", "payment amount", "total")
		reference := getColValue(row, colMap, "reference", "ref", "voucher no")

		if supplierName == "" {
			continue
		}

		payment := SupplierPayment{
			Base:          Base{ID: uuid.New().String()},
			SupplierName:  supplierName,
			PaymentDate:   parseDate(dateStr),
			AmountBHD:     parseFloat(amountStr),
			Reference:     reference,
			PaymentMethod: "Bank Transfer",
		}

		result := db.Create(&payment)
		if result.Error == nil {
			paymentCount++
		}
	}

	log.Printf("Imported %d supplier payments", paymentCount)
}

func seedSuppliers(db *gorm.DB) {
	suppliers := []SupplierMaster{
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "EH-001", SupplierName: "Endress+Hauser", Country: "Germany", SupplierType: "Manufacturer", PaymentTerms: "Net 60", Rating: 5, BrandsHandled: `["Endress+Hauser"]`},
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "SERVO-001", SupplierName: "Servomex", Country: "UK", SupplierType: "Manufacturer", PaymentTerms: "Net 45", Rating: 5, BrandsHandled: `["Servomex"]`},
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "LG-001", SupplierName: "Landis+Gyr", Country: "Switzerland", SupplierType: "Manufacturer", PaymentTerms: "Net 30", Rating: 4, BrandsHandled: `["Landis+Gyr"]`},
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "YOKO-001", SupplierName: "Yokogawa", Country: "Japan", SupplierType: "Manufacturer", PaymentTerms: "Net 60", Rating: 5, BrandsHandled: `["Yokogawa"]`},
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "SICK-001", SupplierName: "SICK AG", Country: "Germany", SupplierType: "Manufacturer", PaymentTerms: "Net 45", Rating: 4, BrandsHandled: `["SICK"]`},
	}

	for _, s := range suppliers {
		db.Create(&s)
	}
	log.Printf("Created %d suppliers", len(suppliers))
}

func printSummary(db *gorm.DB) {
	var counts struct {
		Customers     int64
		Suppliers     int64
		Opportunities int64
		Invoices      int64
		Payments      int64
		Roles         int64
	}

	db.Model(&CustomerMaster{}).Count(&counts.Customers)
	db.Model(&SupplierMaster{}).Count(&counts.Suppliers)
	db.Model(&Opportunity{}).Count(&counts.Opportunities)
	db.Model(&Invoice{}).Count(&counts.Invoices)
	db.Model(&SupplierPayment{}).Count(&counts.Payments)
	db.Model(&Role{}).Count(&counts.Roles)

	log.Println("==========================================")
	log.Printf("Customers:     %d", counts.Customers)
	log.Printf("Suppliers:     %d", counts.Suppliers)
	log.Printf("Opportunities: %d", counts.Opportunities)
	log.Printf("Invoices:      %d", counts.Invoices)
	log.Printf("Supplier Pmts: %d", counts.Payments)
	log.Printf("Roles:         %d", counts.Roles)
	log.Println("==========================================")
	log.Println("Database ready! Run: wails dev")
}

// =============================================================================
// HELPERS
// =============================================================================

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Now()
	}

	formats := []string{
		"2006-01-02",
		"02-01-2006",
		"01/02/2006",
		"02/01/2006",
		"2-Jan-2006",
		"2-Jan-06",
		"Jan 2, 2006",
		"2006/01/02",
	}

	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}
	return time.Now()
}

func getColValue(row []string, colMap map[string]int, names ...string) string {
	for _, name := range names {
		if idx, ok := colMap[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
	}
	return ""
}

func convertDatabaseURL(url string) string {
	url = strings.Replace(url, "postgresql://", "postgres://", 1)
	if !strings.HasPrefix(url, "postgres://") {
		return url
	}
	url = url[11:] // Remove "postgres://"

	atIdx := strings.LastIndex(url, "@")
	if atIdx == -1 {
		return url
	}

	credentials := url[:atIdx]
	hostPart := url[atIdx+1:]

	colonIdx := strings.Index(credentials, ":")
	user := credentials
	password := ""
	if colonIdx != -1 {
		user = credentials[:colonIdx]
		password = credentials[colonIdx+1:]
	}

	slashIdx := strings.Index(hostPart, "/")
	hostPort := hostPart
	dbname := "ph_holdings"
	if slashIdx != -1 {
		hostPort = hostPart[:slashIdx]
		dbname = hostPart[slashIdx+1:]
		if qIdx := strings.Index(dbname, "?"); qIdx != -1 {
			dbname = dbname[:qIdx]
		}
	}

	host := hostPort
	port := "5432"
	if colonIdx := strings.Index(hostPort, ":"); colonIdx != -1 {
		host = hostPort[:colonIdx]
		port = hostPort[colonIdx+1:]
	}

	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require", host, port, user, password, dbname)
}

func escapeLikeWildcards(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
