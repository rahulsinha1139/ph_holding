package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
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

// Consolidated customer names - merge all variations into ~30 canonical customers
var customerConsolidate = map[string]string{
	// ALBA - all variations
	"aluminium bahrain b.s.c":          "ALBA",
	"aluminium bahrain b.s.c.":         "ALBA",
	"aluminium bahrain b.s.c (c)":      "ALBA",
	"aluminium bahrain b.s.c.(c)":      "ALBA",
	"alba":                             "ALBA",

	// BAPCO GROUP - consolidate all entities
	"bapco":                            "BAPCO",
	"bapco upstream w.l.l":             "BAPCO",
	"bapco upstream":                   "BAPCO",
	"bapco gas company b.s.c closed":   "BAPCO",
	"bapco gas expansion company":      "BAPCO",
	"bapco refining b.s.c closed":      "BAPCO",
	"bapco refining":                   "BAPCO",

	// ENGIE - Al Ezzel variations
	"al ezzel operation and maintenance company w.l.l": "Engie",
	"al ezzel operation and maintenance company":       "Engie",
	"al ezzel operation and maintenance compa":         "Engie",
	"al ezzel engie o&m co. w.l.l":                     "Engie",
	"engie":                                            "Engie",

	// SULB - all steel variations
	"bahrain steel bsc c (ec)":         "SULB",
	"bahrain steel bscc (e.c.) - sulb": "SULB",
	"sulb company b.s.c closed":        "SULB",
	"bahrain steel":                    "SULB",
	"sulb":                             "SULB",

	// VEOLIA - consolidate both entities
	"veolia energy gulf wll":                              "Veolia",
	"veolia energy gulf w.l.l.":                           "Veolia",
	"veolia water technologies & solutions bahrain w.l.l": "Veolia",
	"veolia water technologies & solutions ba":            "Veolia",
	"veolia water tech & solutions":                       "Veolia",
	"veolia":                                              "Veolia",

	// AL MOAYYED - consolidate
	"almoayyed electrical & instrumentation services w.l.l": "Al Moayyed",
	"almoayyed electrical & instrumentation s":              "Al Moayyed",
	"almoayyed industrial contracting & t":                  "Al Moayyed",
	"almoayyed":                                             "Al Moayyed",

	// Other major customers (keep separate)
	"seapeak bahrain operations l.l.c":      "Seapeak LNG",
	"bahrain lng/ seapeak":                  "Seapeak LNG",
	"seapeak":                               "Seapeak LNG",
	"gulf petrochemical industries company": "GPIC",
	"gpic":                                  "GPIC",
	"ttsjv w.l.l":                           "TTSJV",
	"ttsjv":                                 "TTSJV",
	"arla foods w.l.l":                      "Arla Foods",
	"arla foods bahrain s.p.c":              "Arla Foods",
	"arla":                                  "Arla Foods",
	"electricity and water authority":       "EWA",
	"ewa":                                   "EWA",
	"muharraq wastewater services company wll": "Muharraq Wastewater",
	"muharraq wastewater services co":          "Muharraq Wastewater",
	"muharraq wastewater":                      "Muharraq Wastewater",
	"wabag belhasa jv wll":                     "Wabag",
	"wabag":                                    "Wabag",
	"bahrain sugar refinery w.l.l":             "Bahrain Sugar Refinery",
	"baharain sugar":                           "Bahrain Sugar Refinery",
	"bahrain sugar":                            "Bahrain Sugar Refinery",
	"aluserv middle east":                      "AluServ",
	"aluserv middle east w.l.l.":               "AluServ",
	"aluserv":                                  "AluServ",
	"westpoint home (bahrain) w.l.l":           "Westpoint Home",
	"west point":                               "Westpoint Home",
	"internationalagencies co. ltd":            "International Agencies",
	"ahs trading w.l.l":                        "AHS Trading",
	"ahs trading w.l.l customer":               "AHS Trading",
	"awal dairy company w.l.l.":                "Awal Dairy",
	"awal dairy":                               "Awal Dairy",
	"arabian international contracting co. wl": "Arabian International",
	"nomac al dur power & water services wll":  "NOMAC",
	"nomac":                                    "NOMAC",
	"sedres analyzer system factory co limite": "Sedres",
	"orbitus/sedres":                           "Sedres",
	"tecton engineering and construction bran": "Tecton",
	"tecton":                                   "Tecton",
	"hidd power co. bsc":                       "Hidd Power",
	"hidd power":                               "Hidd Power",
	"hpc":                                      "Hidd Power",
	"tabreed":                                  "Tabreed",
	"banagas":                                  "BANAGAS",
	"ministry of works":                        "Ministry of Works",
	"mow":                                      "Ministry of Works",
	"garmco":                                   "GARMCO",
	"gramco":                                   "GARMCO",
	"delta bahrain":                            "Delta Bahrain",
	"yokogawa":                                 "Yokogawa",
	"intercol":                                 "Intercol",
	"asiabah accelerated services":             "ASIABAH",
	"asiabah":                                  "ASIABAH",
	"al bakali":                                "Al Bakali",
	"pentium technology":                       "Pentium Technology",
	"serviq international trading":             "Serviq International",
	"agile arabia co. w.l.l.":                  "Agile Arabia",
	"kraum & spruk technologies":               "Kraum & Spruk",
	"emaar":                                    "EMAAR",
	"al mufeed electrical services w.l.l":      "Al Mufeed Electrical",
	"kuwait oil company":                       "Kuwait Oil Company",
	"saudi electricity company":                "Saudi Electricity",
}

// Supplier normalization
var supplierConsolidate = map[string]string{
	"endress + hauser":                          "Endress+Hauser",
	"servomex":                                  "Servomex",
	"landys and gyr":                            "Landis+Gyr",
	"gauges bourdon india pvt ltd":              "Gauges Bourdon India",
	"rsv industries pvt ltd":                    "RSV Industries",
	"mohammed jalal & sons technology w.l.l":    "Mohammed Jalal & Sons",
	"houdec innovation":                         "HOUDEC Innovation",
	"daly middle east llc":                      "Daly Middle East",
	"m t e uk":                                  "MTE UK",
	"metalec steel workshop llc":                "Metalec Steel",
	"fast magic spans w.l.l":                    "Fast Magic Spans",
	"machinery and reliability services":        "Machinery & Reliability",
	"electricity and water authority":           "EWA",
	"sick ag":                                   "SICK AG",
}

// Models
type Base struct {
	ID        string         `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type CustomerMaster struct {
	Base
	CustomerID       string  `gorm:"uniqueIndex;size:50"`
	CustomerCode     string  `gorm:"uniqueIndex;size:50"`
	BusinessName     string  `gorm:"index;size:255"`
	Country          string  `gorm:"size:100"`
	PaymentGrade     string  `gorm:"size:10"`
	CustomerGrade    string  `gorm:"size:10"`
	PaymentTermsDays int     `gorm:"default:30"`
	CreditLimitBHD   float64 `gorm:"default:50000"`
	TRN              string  `gorm:"size:50"`
}

func (CustomerMaster) TableName() string { return "customers" }

type SupplierMaster struct {
	Base
	SupplierCode string `gorm:"uniqueIndex;size:50"`
	SupplierName string `gorm:"index;size:255"`
	Country      string `gorm:"size:100"`
	SupplierType string `gorm:"size:50"`
	PaymentTerms string `gorm:"size:100"`
	Rating       int    `gorm:"default:3"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

type Opportunity struct {
	Base
	FolderNumber string     `gorm:"uniqueIndex;size:50"`
	CustomerID   string     `gorm:"index;size:36"`
	CustomerName string     `gorm:"index;size:255"`
	Salesperson  string     `gorm:"index;size:100"`
	OfferDate    time.Time  `gorm:"index"`
	ExpectedDate *time.Time
	RevenueBHD   float64
	Stage        string `gorm:"index;size:50"`
	EHReference  string `gorm:"size:100"`
	PaymentTerms string `gorm:"size:200"`
	Notes        string `gorm:"type:text"`
	LostReason   string `gorm:"type:varchar(500)"`
}

func (Opportunity) TableName() string { return "opportunities" }

type Invoice struct {
	Base
	InvoiceNumber  string    `gorm:"uniqueIndex;size:50"`
	CustomerID     string    `gorm:"index;size:36"`
	CustomerName   string    `gorm:"size:255"`
	OrderID        string    `gorm:"index;size:36"`
	InvoiceDate    time.Time `gorm:"index"`
	DueDate        time.Time
	SubtotalBHD    float64
	VATBHD         float64   `gorm:"column:vatbhd"`
	VATPercent     float64   `gorm:"column:vat_percent;default:10"`
	TotalBHD       float64
	Status         string    `gorm:"index;size:50;default:'Unpaid'"`
	Reference      string    `gorm:"size:100"`
	TRN            string    `gorm:"size:50"`
}

func (Invoice) TableName() string { return "invoices" }

type Role struct {
	Base
	Name        string `gorm:"uniqueIndex;size:50"`
	Permissions string `gorm:"type:varchar(2000)"`
	Description string `gorm:"type:varchar(500)"`
}

func (Role) TableName() string { return "roles" }

var db *gorm.DB
var customerIDMap = make(map[string]string) // lowercase name -> UUID

func main() {
	log.Println("=== PH HOLDINGS - COMPLETE DATA IMPORT ===")
	log.Println("Consolidating customers, importing invoices & opportunities")

	godotenv.Load("../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	dsn := convertDatabaseURL(databaseURL)
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Drop and recreate
	log.Println("\n=== Dropping tables ===")
	dropAllTables()

	log.Println("\n=== Creating tables ===")
	db.AutoMigrate(&CustomerMaster{}, &SupplierMaster{}, &Opportunity{}, &Invoice{}, &Role{})

	// Seed roles
	log.Println("\n=== Seeding roles ===")
	seedRoles()

	// Step 1: Extract and consolidate customers from Tally
	log.Println("\n=== Step 1: Extracting customers from Tally invoices ===")
	customers := extractAndConsolidateCustomers()
	log.Printf("Consolidated to %d unique customers", len(customers))

	// Create customers in DB
	createCustomers(customers)

	// Step 2: Extract suppliers from Tally
	log.Println("\n=== Step 2: Extracting suppliers from Tally purchases ===")
	suppliers := extractSuppliers()
	log.Printf("Found %d unique suppliers", len(suppliers))
	createSuppliers(suppliers)

	// Step 3: Import invoices from Tally
	log.Println("\n=== Step 3: Importing invoices from Tally ===")
	invoiceCount := importInvoices()
	log.Printf("Imported %d invoices", invoiceCount)

	// Step 4: Import opportunities and link to customers
	log.Println("\n=== Step 4: Importing opportunities ===")
	oppCount := importOpportunities()
	log.Printf("Imported %d opportunities", oppCount)

	// Print summary
	printSummary()
}

func dropAllTables() {
	tables := []string{
		"device_users", "devices", "opportunities", "customers", "suppliers", "roles",
		"offers", "offer_items", "orders", "order_items", "invoices", "invoice_items",
		"payments", "purchase_orders", "purchase_order_items", "delivery_notes",
		"delivery_note_items", "goods_received_notes", "grn_items", "supplier_invoices",
		"supplier_invoice_items", "supplier_payments", "conversations", "chat_messages",
		"entity_notes", "supplier_issues", "costing_sheets", "costing_items",
		"customer_contacts", "supplier_contacts", "products", "rfqs", "quick_captures",
		"sync_records", "audit_logs", "settings", "alerts",
	}
	for _, t := range tables {
		db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", t))
	}
}

func seedRoles() {
	roles := []Role{
		{Base: Base{ID: uuid.New().String()}, Name: "admin", Permissions: `["*"]`, Description: "Full access"},
		{Base: Base{ID: uuid.New().String()}, Name: "manager", Permissions: `["read","write","finance","operations","crm"]`, Description: "Manager"},
		{Base: Base{ID: uuid.New().String()}, Name: "sales", Permissions: `["read","write","crm","offers"]`, Description: "Sales"},
		{Base: Base{ID: uuid.New().String()}, Name: "operations", Permissions: `["read","write","operations"]`, Description: "Operations"},
		{Base: Base{ID: uuid.New().String()}, Name: "staff", Permissions: `["read"]`, Description: "Read-only"},
	}
	for _, r := range roles {
		db.Create(&r)
	}
}

func extractAndConsolidateCustomers() map[string]string {
	customers := make(map[string]string) // consolidated name -> TRN

	invoiceFiles := []string{
		"../../Data_for_database/Tally_data/invoices 2025.xlsx",
		"../../Data_for_database/Tally_data/invoices 2024.xlsx",
		"../../Data_for_database/Tally_data/invoices 2023.xlsx",
	}

	for _, file := range invoiceFiles {
		f, err := excelize.OpenFile(file)
		if err != nil {
			log.Printf("WARN: Cannot open %s: %v", file, err)
			continue
		}

		rows, _ := f.GetRows(f.GetSheetName(0))
		for i := 8; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 2 {
				continue
			}
			rawName := strings.TrimSpace(row[1])
			trn := ""
			if len(row) > 2 {
				trn = strings.TrimSpace(row[2])
			}
			if rawName == "" || strings.HasPrefix(rawName, "Total") || strings.HasPrefix(rawName, "Grand") {
				continue
			}

			// Consolidate
			consolidated := consolidateCustomerName(rawName)
			if consolidated != "" {
				if _, exists := customers[consolidated]; !exists {
					customers[consolidated] = trn
				}
			}
		}
		f.Close()
	}

	return customers
}

func consolidateCustomerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	key := strings.ToLower(name)

	// Check consolidation map
	if consolidated, ok := customerConsolidate[key]; ok {
		return consolidated
	}

	// Try partial matching for truncated Tally names
	for pattern, consolidated := range customerConsolidate {
		if strings.HasPrefix(key, pattern) || strings.HasPrefix(pattern, key) {
			return consolidated
		}
		// Also check if key contains pattern
		if len(pattern) > 5 && strings.Contains(key, pattern) {
			return consolidated
		}
	}

	// Clean up and return as-is for unknown customers
	name = strings.TrimSuffix(name, " W.L.L")
	name = strings.TrimSuffix(name, " W.L.L.")
	name = strings.TrimSuffix(name, " B.S.C")
	name = strings.TrimSuffix(name, " B.S.C.")
	name = strings.TrimSuffix(name, " CLOSED")

	return strings.TrimSpace(name)
}

func createCustomers(customers map[string]string) {
	count := 0
	for name, trn := range customers {
		count++
		id := uuid.New().String()
		customerIDMap[strings.ToLower(name)] = id

		grade := "B"
		creditLimit := 50000.0
		// Top customers get better grades
		topCustomers := map[string]bool{
			"ALBA": true, "BAPCO": true, "Engie": true, "SULB": true,
			"EWA": true, "GPIC": true, "Arla Foods": true,
		}
		if topCustomers[name] {
			grade = "A"
			creditLimit = 100000.0
		}

		db.Create(&CustomerMaster{
			Base:             Base{ID: id},
			CustomerID:       fmt.Sprintf("CUST-%04d", count),
			CustomerCode:     fmt.Sprintf("C%04d", count),
			BusinessName:     name,
			Country:          "Bahrain",
			PaymentGrade:     grade,
			CustomerGrade:    grade,
			PaymentTermsDays: 30,
			CreditLimitBHD:   creditLimit,
			TRN:              trn,
		})
	}
	log.Printf("Created %d customers", count)
}

func extractSuppliers() map[string]bool {
	suppliers := make(map[string]bool)

	purchaseFiles := []string{
		"../../Data_for_database/Tally_data/purchase 2025.xlsx",
		"../../Data_for_database/Tally_data/purchase 2024.xlsx",
	}

	for _, file := range purchaseFiles {
		f, err := excelize.OpenFile(file)
		if err != nil {
			continue
		}

		rows, _ := f.GetRows(f.GetSheetName(0))
		for i := 8; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 2 {
				continue
			}
			name := strings.TrimSpace(row[1])
			if name == "" || strings.HasPrefix(name, "Total") {
				continue
			}

			// Consolidate
			key := strings.ToLower(name)
			if consolidated, ok := supplierConsolidate[key]; ok {
				suppliers[consolidated] = true
			} else {
				suppliers[name] = true
			}
		}
		f.Close()
	}

	return suppliers
}

func createSuppliers(suppliers map[string]bool) {
	count := 0
	for name := range suppliers {
		count++
		country := "International"
		if strings.Contains(strings.ToLower(name), "india") {
			country = "India"
		} else if strings.Contains(strings.ToLower(name), "uk") {
			country = "UK"
		} else if name == "Endress+Hauser" || name == "Landis+Gyr" || name == "SICK AG" {
			country = "Germany/Switzerland"
		} else if name == "Servomex" {
			country = "UK"
		}

		db.Create(&SupplierMaster{
			Base:         Base{ID: uuid.New().String()},
			SupplierCode: fmt.Sprintf("SUP-%04d", count),
			SupplierName: name,
			Country:      country,
			SupplierType: "Manufacturer",
			PaymentTerms: "Net 60",
			Rating:       4,
		})
	}
	log.Printf("Created %d suppliers", count)
}

func importInvoices() int {
	invoiceFiles := []string{
		"../../Data_for_database/Tally_data/invoices 2025.xlsx",
		"../../Data_for_database/Tally_data/invoices 2024.xlsx",
		"../../Data_for_database/Tally_data/invoices 2023.xlsx",
	}

	count := 0
	for _, file := range invoiceFiles {
		f, err := excelize.OpenFile(file)
		if err != nil {
			continue
		}

		rows, _ := f.GetRows(f.GetSheetName(0))
		// Headers at row 7: Date | Particulars | TRN | Vch Type | Vch No. | Supplier Inv/Ref | Taxable Value | Tax Amount
		for i := 8; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 8 {
				continue
			}

			dateStr := strings.TrimSpace(row[0])
			rawCustomer := strings.TrimSpace(row[1])
			trn := strings.TrimSpace(row[2])
			vchNo := strings.TrimSpace(row[4])
			reference := strings.TrimSpace(row[5])
			taxableStr := strings.TrimSpace(row[6])
			taxStr := strings.TrimSpace(row[7])

			if dateStr == "" || rawCustomer == "" || vchNo == "" {
				continue
			}
			if strings.HasPrefix(rawCustomer, "Total") || strings.HasPrefix(rawCustomer, "Grand") {
				continue
			}

			// Parse values
			invoiceDate := parseTallyDate(dateStr)
			if invoiceDate.IsZero() {
				continue
			}

			taxable := parseAmount(taxableStr)
			tax := parseAmount(taxStr)
			total := taxable + tax

			// Find customer
			customerName := consolidateCustomerName(rawCustomer)
			customerID := customerIDMap[strings.ToLower(customerName)]

			// Due date = invoice date + 30 days
			dueDate := invoiceDate.AddDate(0, 0, 30)

			// Determine status based on date (simplified)
			status := "Unpaid"
			if invoiceDate.Before(time.Now().AddDate(0, -2, 0)) {
				status = "Paid" // Assume older invoices are paid
			}

			inv := Invoice{
				Base:          Base{ID: uuid.New().String()},
				InvoiceNumber: vchNo,
				CustomerID:    customerID,
				CustomerName:  customerName,
				InvoiceDate:   invoiceDate,
				DueDate:       dueDate,
				SubtotalBHD:   taxable,
				VATBHD:        tax,
				VATPercent:    10.0,
				TotalBHD:      total,
				Status:        status,
				Reference:     reference,
				TRN:           trn,
			}

			result := db.Create(&inv)
			if result.Error == nil {
				count++
			}
		}
		f.Close()
	}

	return count
}

func importOpportunities() int {
	f, err := excelize.OpenFile("../../Data_for_database/opportunities created 2025.xlsx")
	if err != nil {
		log.Printf("ERROR opening opportunities file: %v", err)
		return 0
	}
	defer f.Close()

	rows, _ := f.GetRows(f.GetSheetName(0))
	if len(rows) < 2 {
		return 0
	}

	header := rows[0]
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	count := 0
	matched := 0

	for rowNum, row := range rows[1:] {
		folderNo := getCell(row, colIdx, "folder no.", "folder no")
		folderName := getCell(row, colIdx, "folder name")
		endUser := getCell(row, colIdx, "end user")
		owner := getCell(row, colIdx, "owner")
		valueStr := getCell(row, colIdx, "value in bhd", " value in bhd ")
		status := getCell(row, colIdx, "status", " status ")
		lostReason := getCell(row, colIdx, "reason for loss", " reason for loss ")
		ehRef := getCell(row, colIdx, "e+h refference no")
		paymentTerms := getCell(row, colIdx, "payment terms", " payment terms ")
		quoteDateStr := getCell(row, colIdx, "quote date")
		year := getCell(row, colIdx, "year")

		if folderNo == "" && endUser == "" {
			continue
		}

		if folderNo == "" {
			folderNo = fmt.Sprintf("UNK-%d", rowNum+1)
		}

		// Find customer - try multiple methods
		customerName := ""
		customerID := ""

		// Method 1: Direct End User lookup
		if endUser != "" && !isJustNumber(endUser) {
			customerName = findCustomer(endUser)
		}

		// Method 2: Extract from folder name
		if customerName == "" && folderName != "" {
			extracted := extractFromFolderName(folderName)
			if extracted != "" {
				customerName = findCustomer(extracted)
			}
		}

		if customerName != "" {
			customerID = customerIDMap[strings.ToLower(customerName)]
			matched++
		}

		// Parse date
		offerDate := parseDate(quoteDateStr)
		if offerDate.IsZero() {
			if y, _ := strconv.Atoi(year); y > 2000 {
				offerDate = time.Date(y, 6, 1, 0, 0, 0, 0, time.UTC)
			} else {
				offerDate = time.Now().AddDate(0, -3, 0)
			}
		}

		opp := Opportunity{
			Base:         Base{ID: uuid.New().String()},
			FolderNumber: folderNo,
			CustomerID:   customerID,
			CustomerName: customerName,
			Salesperson:  cleanOwner(owner),
			OfferDate:    offerDate,
			RevenueBHD:   parseAmount(valueStr),
			Stage:        mapStatus(status),
			EHReference:  ehRef,
			PaymentTerms: paymentTerms,
			LostReason:   lostReason,
			Notes:        folderName,
		}

		result := db.Create(&opp)
		if result.Error != nil {
			// Duplicate folder number - add suffix
			opp.FolderNumber = fmt.Sprintf("%s-R%d", folderNo, rowNum)
			db.Create(&opp)
		}
		count++
	}

	log.Printf("Matched %d/%d opportunities to customers", matched, count)
	return count
}

func findCustomer(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Try consolidation first
	consolidated := consolidateCustomerName(name)
	if _, exists := customerIDMap[strings.ToLower(consolidated)]; exists {
		return consolidated
	}

	// Keyword matching
	keywords := map[string]string{
		"alba": "ALBA", "aluminium": "ALBA",
		"bapco": "BAPCO",
		"engie": "Engie", "ezzel": "Engie",
		"sulb": "SULB", "steel": "SULB",
		"veolia": "Veolia",
		"muharraq": "Muharraq Wastewater",
		"seapeak": "Seapeak LNG", "lng": "Seapeak LNG",
		"gpic": "GPIC", "petrochemical": "GPIC",
		"ewa": "EWA", "electricity": "EWA",
		"arla": "Arla Foods",
		"ttsjv": "TTSJV",
		"sugar": "Bahrain Sugar Refinery",
		"moayyed": "Al Moayyed",
		"wabag": "Wabag",
		"nomac": "NOMAC",
		"hidd": "Hidd Power", "hpc": "Hidd Power",
		"tabreed": "Tabreed",
		"banagas": "BANAGAS",
		"ministry": "Ministry of Works", "mow": "Ministry of Works",
		"garmco": "GARMCO",
		"yokogawa": "Yokogawa",
		"intercol": "Intercol",
		"awal": "Awal Dairy",
		"westpoint": "Westpoint Home",
		"aluserv": "AluServ",
		"delta": "Delta Bahrain",
	}

	key := strings.ToLower(name)
	for keyword, custName := range keywords {
		if strings.Contains(key, keyword) {
			if _, exists := customerIDMap[strings.ToLower(custName)]; exists {
				return custName
			}
		}
	}

	return ""
}

func extractFromFolderName(folderName string) string {
	// Pattern: "NUMBER- CUSTOMER SUFFIX"
	re := regexp.MustCompile(`^\d+[-\s]*\d*[-\s]+(.+)$`)
	if m := re.FindStringSubmatch(folderName); len(m) >= 2 {
		rest := strings.TrimSpace(m[1])
		words := strings.Fields(rest)
		techSuffixes := map[string]bool{
			"FIT": true, "LIT": true, "TIT": true, "PIT": true, "SP": true,
			"SPARES": true, "MASS": true, "COND": true, "SERVICE": true,
			"FEED": true, "CABLE": true, "CALIBRATION": true, "SGR": true,
			"AIT": true, "FTS": true, "BTU": true, "CEMS": true,
		}
		var customerWords []string
		for _, w := range words {
			if techSuffixes[strings.ToUpper(w)] {
				break
			}
			customerWords = append(customerWords, w)
			if len(customerWords) >= 2 {
				break
			}
		}
		if len(customerWords) > 0 {
			return strings.Join(customerWords, " ")
		}
	}
	return ""
}

func isJustNumber(s string) bool {
	_, err := strconv.Atoi(strings.TrimSpace(s))
	return err == nil
}

func parseTallyDate(s string) time.Time {
	// Tally format: DD/MM/YYYY
	s = strings.TrimSpace(s)
	formats := []string{"02/01/2006", "2/1/2006", "02-01-2006", "2-Jan-06", "2-Jan-2006"}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	formats := []string{"2-Jan-06", "2-Jan-2006", "02-Jan-06", "02-Jan-2006", "2/1/2006", "02/01/2006"}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseAmount(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	re := regexp.MustCompile(`[\d.]+`)
	if m := re.FindString(s); m != "" {
		f, _ := strconv.ParseFloat(m, 64)
		return f
	}
	return 0
}

func getCell(row []string, colIdx map[string]int, names ...string) string {
	for _, name := range names {
		if idx, ok := colIdx[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
	}
	return ""
}

func mapStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch {
	case strings.Contains(status, "payment received"):
		return "Won"
	case strings.Contains(status, "order placed"), strings.Contains(status, "po/loi"):
		return "Won"
	case strings.Contains(status, "lost"):
		return "Lost"
	case strings.Contains(status, "in process"):
		return "Negotiation"
	default:
		return "Lead"
	}
}

func cleanOwner(owner string) string {
	owner = strings.TrimSpace(owner)
	switch strings.ToLower(owner) {
	case "sundar":
		return "Sundar"
	case "ramya":
		return "Ramya"
	case "ebin":
		return "Ebin"
	case "peri":
		return "Peri"
	default:
		if owner == "" {
			return "Unassigned"
		}
		return owner
	}
}

func printSummary() {
	var custCount, suppCount, oppCount, invCount, roleCount int64
	db.Model(&CustomerMaster{}).Count(&custCount)
	db.Model(&SupplierMaster{}).Count(&suppCount)
	db.Model(&Opportunity{}).Count(&oppCount)
	db.Model(&Invoice{}).Count(&invCount)
	db.Model(&Role{}).Count(&roleCount)

	var stageStats []struct {
		Stage string
		Count int64
	}
	db.Model(&Opportunity{}).Select("stage, count(*) as count").Group("stage").Scan(&stageStats)

	var topCustomers []struct {
		BusinessName string
		OppCount     int64
		InvCount     int64
		Revenue      float64
	}
	db.Raw(`
		SELECT c.business_name,
			   COUNT(DISTINCT o.id) as opp_count,
			   COUNT(DISTINCT i.id) as inv_count,
			   COALESCE(SUM(i.total_bhd), 0) as revenue
		FROM customers c
		LEFT JOIN opportunities o ON o.customer_id = c.id
		LEFT JOIN invoices i ON i.customer_id = c.id
		GROUP BY c.id, c.business_name
		ORDER BY revenue DESC
		LIMIT 20
	`).Scan(&topCustomers)

	log.Println("\n==========================================")
	log.Println("       COMPLETE DATA IMPORT FINISHED")
	log.Println("==========================================")
	log.Printf("Customers:     %d", custCount)
	log.Printf("Suppliers:     %d", suppCount)
	log.Printf("Opportunities: %d", oppCount)
	log.Printf("Invoices:      %d", invCount)
	log.Printf("Roles:         %d", roleCount)

	log.Println("\n--- Opportunity Stage Breakdown ---")
	for _, s := range stageStats {
		log.Printf("  %-15s: %d", s.Stage, s.Count)
	}

	log.Println("\n--- Top Customers (by Invoice Revenue) ---")
	for i, c := range topCustomers {
		log.Printf("  %2d. %-25s | %3d opps | %3d invs | %.2f BHD", i+1, c.BusinessName, c.OppCount, c.InvCount, c.Revenue)
	}
	log.Println("==========================================")
}

func convertDatabaseURL(url string) string {
	url = strings.Replace(url, "postgresql://", "postgres://", 1)
	if !strings.HasPrefix(url, "postgres://") {
		return url
	}
	url = url[11:]
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
