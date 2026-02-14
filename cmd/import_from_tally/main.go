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

// Canonical customer names - normalize Tally variations
var customerNormalize = map[string]string{
	// ALBA
	"aluminium bahrain b.s.c":         "ALBA",
	"aluminium bahrain b.s.c.":        "ALBA",
	"aluminium bahrain b.s.c (c)":     "ALBA",
	"aluminium bahrain b.s.c.(c)":     "ALBA",
	// BAPCO Group (multiple entities)
	"bapco upstream w.l.l":            "BAPCO Upstream",
	"bapco gas company b.s.c closed":  "BAPCO Gas",
	"bapco gas expansion company":     "BAPCO Gas Expansion",
	"bapco refining b.s.c closed":     "BAPCO Refining",
	// Engie
	"al ezzel operation and maintenance company w.l.l":   "Engie",
	"al ezzel operation and maintenance company":         "Engie",
	"al ezzel operation and maintenance compa":           "Engie",
	// SULB
	"bahrain steel bsc c (ec)":        "SULB",
	"bahrain steel bscc (e.c.) - sulb": "SULB",
	"sulb company b.s.c closed":       "SULB",
	// Veolia
	"veolia energy gulf wll":          "Veolia Energy",
	"veolia water technologies & solutions bahrain w.l.l": "Veolia Water Tech",
	"veolia water technologies & solutions ba": "Veolia Water Tech",
	// Others (normalize to clean names)
	"seapeak bahrain operations l.l.c": "Seapeak LNG",
	"gulf petrochemical industries company": "GPIC",
	"ttsjv w.l.l":                      "TTSJV",
	"arla foods w.l.l":                 "Arla Foods",
	"electricity and water authority":  "EWA",
	"muharraq wastewater services company wll": "Muharraq Wastewater",
	"wabag belhasa jv wll":             "Wabag",
	"bahrain sugar refinery w.l.l":     "Bahrain Sugar Refinery",
	"almoayyed electrical & instrumentation services w.l.l": "Al Moayyed E&I",
	"almoayyed electrical & instrumentation s": "Al Moayyed E&I",
	"almoayyed industrial contracting & t": "Al Moayyed Industrial",
	"aluserv middle east":              "AluServ",
	"westpoint home (bahrain) w.l.l":   "Westpoint Home",
	"internationalagencies co. ltd":    "International Agencies",
	"ahs trading w.l.l customer":       "AHS Trading",
	"ahs trading w.l.l":                "AHS Trading",
	"awal dairy company w.l.l.":        "Awal Dairy",
	"arabian international contracting co. wl": "Arabian International",
	"nomac al dur power & water services wll": "NOMAC",
	"sedres analyzer system factory co limite": "Sedres",
	"tecton engineering and construction bran": "Tecton",
	"hidd power co. bsc":               "Hidd Power",
	"tabreed":                          "Tabreed",
	"banagas":                          "BANAGAS",
	"ministry of works":                "Ministry of Works",
	"garmco":                           "GARMCO",
	"delta bahrain":                    "Delta Bahrain",
	"yokogawa":                         "Yokogawa",
	"intercol":                         "Intercol",
}

// Supplier normalization
var supplierNormalize = map[string]string{
	"endress + hauser":                 "Endress+Hauser",
	"servomex":                         "Servomex",
	"landys and gyr":                   "Landis+Gyr",
	"gauges bourdon india pvt ltd":     "Gauges Bourdon India",
	"rsv industries pvt ltd":           "RSV Industries",
	"mohammed jalal & sons technology w.l.l": "Mohammed Jalal & Sons",
	"houdec innovation":                "HOUDEC Innovation",
	"daly middle east llc":             "Daly Middle East",
	"m t e uk":                         "MTE UK",
	"metalec steel workshop llc":       "Metalec Steel",
	"fast magic spans w.l.l":           "Fast Magic Spans",
	"machinery and reliability services": "Machinery & Reliability",
	"electricity and water authority":  "EWA", // EWA can be both customer and supplier
}

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
	TRN              string  `gorm:"size:50"` // Tax Registration Number from Tally
}

func (CustomerMaster) TableName() string { return "customers" }

type SupplierMaster struct {
	Base
	SupplierCode  string `gorm:"uniqueIndex;size:50"`
	SupplierName  string `gorm:"index;size:255"`
	Country       string `gorm:"size:100"`
	SupplierType  string `gorm:"size:50"`
	PaymentTerms  string `gorm:"size:100"`
	Rating        int    `gorm:"default:3"`
}

func (SupplierMaster) TableName() string { return "suppliers" }

type Opportunity struct {
	Base
	FolderNumber   string     `gorm:"uniqueIndex;size:50"`
	CustomerID     string     `gorm:"index;size:36"`
	CustomerName   string     `gorm:"index;size:255"`
	Salesperson    string     `gorm:"index;size:100"`
	OfferDate      time.Time  `gorm:"index"`
	ExpectedDate   *time.Time
	RevenueBHD     float64
	Stage          string     `gorm:"index;size:50"`
	EHReference    string     `gorm:"size:100"`
	PaymentTerms   string     `gorm:"size:200"`
	Notes          string     `gorm:"type:text"`
	LostReason     string     `gorm:"type:varchar(500)"`
}

func (Opportunity) TableName() string { return "opportunities" }

type Role struct {
	Base
	Name        string `gorm:"uniqueIndex;size:50"`
	Permissions string `gorm:"type:varchar(2000)"`
	Description string `gorm:"type:varchar(500)"`
}

func (Role) TableName() string { return "roles" }

func main() {
	log.Println("=== PH HOLDINGS - TALLY DATA IMPORT ===")

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

	// Drop and recreate
	log.Println("\n=== Dropping tables ===")
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

	log.Println("\n=== Creating tables ===")
	db.AutoMigrate(&CustomerMaster{}, &SupplierMaster{}, &Opportunity{}, &Role{})

	// Seed roles
	log.Println("\n=== Seeding roles ===")
	seedRoles(db)

	// Extract customers from Tally invoices
	log.Println("\n=== Extracting customers from Tally invoices ===")
	customers := extractCustomersFromTally()
	log.Printf("Found %d unique customers in Tally data", len(customers))

	// Create customers
	customerIDMap := make(map[string]string) // normalized name -> UUID
	custCount := 0
	for name, trn := range customers {
		custCount++
		id := uuid.New().String()
		customerIDMap[strings.ToLower(name)] = id
		db.Create(&CustomerMaster{
			Base:           Base{ID: id},
			CustomerID:     fmt.Sprintf("CUST-%04d", custCount),
			CustomerCode:   fmt.Sprintf("C%04d", custCount),
			BusinessName:   name,
			Country:        "Bahrain",
			PaymentGrade:   "B",
			CustomerGrade:  "B",
			CreditLimitBHD: 50000,
			TRN:            trn,
		})
	}
	log.Printf("Created %d customers", custCount)

	// Extract suppliers from Tally purchases
	log.Println("\n=== Extracting suppliers from Tally purchases ===")
	suppliers := extractSuppliersFromTally()
	log.Printf("Found %d unique suppliers in Tally data", len(suppliers))

	// Create suppliers
	suppCount := 0
	for name := range suppliers {
		suppCount++
		country := "International"
		if strings.Contains(strings.ToLower(name), "w.l.l") || strings.Contains(strings.ToLower(name), "bahrain") {
			country = "Bahrain"
		} else if strings.Contains(strings.ToLower(name), "india") {
			country = "India"
		} else if strings.Contains(strings.ToLower(name), "uk") {
			country = "UK"
		}
		db.Create(&SupplierMaster{
			Base:         Base{ID: uuid.New().String()},
			SupplierCode: fmt.Sprintf("SUP-%04d", suppCount),
			SupplierName: name,
			Country:      country,
			SupplierType: "Manufacturer",
			PaymentTerms: "Net 60",
			Rating:       4,
		})
	}
	log.Printf("Created %d suppliers", suppCount)

	// Import opportunities
	log.Println("\n=== Importing opportunities ===")
	importOpportunities(db, customerIDMap, "../../Data_for_database/opportunities created 2025.xlsx")

	printSummary(db)
}

func extractCustomersFromTally() map[string]string {
	customers := make(map[string]string) // name -> TRN

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
		// Data starts at row 8 (index 8), column 1 = Particulars, column 2 = TRN
		for i := 8; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 2 {
				continue
			}
			name := strings.TrimSpace(row[1])
			trn := ""
			if len(row) > 2 {
				trn = strings.TrimSpace(row[2])
			}
			if name == "" || strings.HasPrefix(name, "Total") || strings.HasPrefix(name, "Grand") {
				continue
			}

			// Normalize the name
			normalized := normalizeCustomerName(name)
			if normalized != "" {
				if _, exists := customers[normalized]; !exists {
					customers[normalized] = trn
				}
			}
		}
		f.Close()
	}

	return customers
}

func extractSuppliersFromTally() map[string]bool {
	suppliers := make(map[string]bool)

	purchaseFiles := []string{
		"../../Data_for_database/Tally_data/purchase 2025.xlsx",
		"../../Data_for_database/Tally_data/purchase 2024.xlsx",
	}

	for _, file := range purchaseFiles {
		f, err := excelize.OpenFile(file)
		if err != nil {
			log.Printf("WARN: Cannot open %s: %v", file, err)
			continue
		}

		rows, _ := f.GetRows(f.GetSheetName(0))
		// Data starts at row 8, column 1 = Particulars (supplier name)
		for i := 8; i < len(rows); i++ {
			row := rows[i]
			if len(row) < 2 {
				continue
			}
			name := strings.TrimSpace(row[1])
			if name == "" || strings.HasPrefix(name, "Total") || strings.HasPrefix(name, "Grand") {
				continue
			}

			// Normalize
			normalized := normalizeSupplierName(name)
			if normalized != "" {
				suppliers[normalized] = true
			}
		}
		f.Close()
	}

	return suppliers
}

func normalizeCustomerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Check normalization map
	key := strings.ToLower(name)
	if normalized, ok := customerNormalize[key]; ok {
		return normalized
	}

	// Try partial matching for truncated names
	for pattern, normalized := range customerNormalize {
		if strings.HasPrefix(key, pattern) || strings.HasPrefix(pattern, key) {
			return normalized
		}
	}

	// Clean up generic suffixes and return as-is
	name = strings.TrimSuffix(name, " W.L.L")
	name = strings.TrimSuffix(name, " W.L.L.")
	name = strings.TrimSuffix(name, " B.S.C")
	name = strings.TrimSuffix(name, " B.S.C.")
	name = strings.TrimSuffix(name, " CLOSED")

	return strings.TrimSpace(name)
}

func normalizeSupplierName(name string) string {
	name = strings.TrimSpace(name)
	key := strings.ToLower(name)
	if normalized, ok := supplierNormalize[key]; ok {
		return normalized
	}
	return name
}

func importOpportunities(db *gorm.DB, customerIDMap map[string]string, filepath string) {
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		log.Printf("ERROR: %v", err)
		return
	}
	defer f.Close()

	rows, _ := f.GetRows(f.GetSheetName(0))
	if len(rows) < 2 {
		return
	}

	header := rows[0]
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}

	oppCount := 0
	matched := 0
	unmatched := 0

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

		// Try to match customer
		customerName := ""
		customerID := ""

		// First try End User
		if endUser != "" && !isJustNumber(endUser) {
			customerName = findMatchingCustomer(endUser, customerIDMap)
		}

		// If no match, try folder name
		if customerName == "" && folderName != "" {
			extracted := extractCustomerFromFolderName(folderName)
			if extracted != "" {
				customerName = findMatchingCustomer(extracted, customerIDMap)
			}
		}

		if customerName != "" {
			customerID = customerIDMap[strings.ToLower(customerName)]
			matched++
		} else {
			unmatched++
		}

		// Map status
		stage := mapStatus(status)

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
			RevenueBHD:   parseValue(valueStr),
			Stage:        stage,
			EHReference:  ehRef,
			PaymentTerms: paymentTerms,
			LostReason:   lostReason,
			Notes:        folderName,
		}

		result := db.Create(&opp)
		if result.Error != nil {
			opp.FolderNumber = fmt.Sprintf("%s-R%d", folderNo, rowNum)
			db.Create(&opp)
		}
		oppCount++
	}

	log.Printf("Created %d opportunities (%d matched to customers, %d unmatched)", oppCount, matched, unmatched)
}

func isJustNumber(s string) bool {
	s = strings.TrimSpace(s)
	_, err := strconv.Atoi(s)
	return err == nil
}

func findMatchingCustomer(name string, customerIDMap map[string]string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	key := strings.ToLower(name)

	// Try exact match
	for custName := range customerIDMap {
		if custName == key {
			return strings.ToUpper(custName[:1]) + custName[1:] // Return proper case
		}
	}

	// Try normalized match
	normalized := normalizeCustomerName(name)
	normalizedKey := strings.ToLower(normalized)
	for custName := range customerIDMap {
		if custName == normalizedKey {
			return normalized
		}
	}

	// Try keyword matching
	keywords := map[string]string{
		"alba":      "ALBA",
		"aluminium": "ALBA",
		"bapco":     "BAPCO",
		"engie":     "Engie",
		"ezzel":     "Engie",
		"sulb":      "SULB",
		"steel":     "SULB",
		"veolia":    "Veolia",
		"muharraq":  "Muharraq Wastewater",
		"seapeak":   "Seapeak LNG",
		"lng":       "Seapeak LNG",
		"gpic":      "GPIC",
		"petrochemical": "GPIC",
		"ewa":       "EWA",
		"electricity": "EWA",
		"arla":      "Arla Foods",
		"ttsjv":     "TTSJV",
		"sugar":     "Bahrain Sugar Refinery",
		"moayyed":   "Al Moayyed",
		"wabag":     "Wabag",
		"nomac":     "NOMAC",
		"hidd":      "Hidd Power",
		"tabreed":   "Tabreed",
		"banagas":   "BANAGAS",
		"ministry":  "Ministry of Works",
		"mow":       "Ministry of Works",
		"garmco":    "GARMCO",
		"yokogawa":  "Yokogawa",
		"intercol":  "Intercol",
		"awal":      "Awal Dairy",
		"westpoint": "Westpoint Home",
		"aluserv":   "AluServ",
		"delta":     "Delta Bahrain",
	}

	for keyword, custName := range keywords {
		if strings.Contains(key, keyword) {
			if _, exists := customerIDMap[strings.ToLower(custName)]; exists {
				return custName
			}
		}
	}

	return ""
}

func extractCustomerFromFolderName(folderName string) string {
	// Pattern: "NUMBER- CUSTOMER SUFFIX" or "NUMBER-NUMBER CUSTOMER SUFFIX"
	re := regexp.MustCompile(`^\d+[-\s]*\d*[-\s]+(.+)$`)
	if m := re.FindStringSubmatch(folderName); len(m) >= 2 {
		rest := strings.TrimSpace(m[1])
		// Take first word(s) before technical suffixes
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

func seedRoles(db *gorm.DB) {
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
	log.Printf("Created %d roles", len(roles))
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

func parseValue(s string) float64 {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	re := regexp.MustCompile(`[\d.]+`)
	if m := re.FindString(s); m != "" {
		f, _ := strconv.ParseFloat(m, 64)
		return f
	}
	return 0
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

func printSummary(db *gorm.DB) {
	var custCount, suppCount, oppCount, roleCount int64
	db.Model(&CustomerMaster{}).Count(&custCount)
	db.Model(&SupplierMaster{}).Count(&suppCount)
	db.Model(&Opportunity{}).Count(&oppCount)
	db.Model(&Role{}).Count(&roleCount)

	var stageStats []struct {
		Stage string
		Count int64
	}
	db.Model(&Opportunity{}).Select("stage, count(*) as count").Group("stage").Scan(&stageStats)

	var topCustomers []struct {
		BusinessName string
		Count        int64
	}
	db.Raw(`
		SELECT c.business_name, COUNT(o.id) as count
		FROM opportunities o
		JOIN customers c ON o.customer_id = c.id
		GROUP BY c.business_name
		ORDER BY count DESC
		LIMIT 20
	`).Scan(&topCustomers)

	log.Println("\n==========================================")
	log.Println("       TALLY DATA IMPORT COMPLETE")
	log.Println("==========================================")
	log.Printf("Customers:     %d", custCount)
	log.Printf("Suppliers:     %d", suppCount)
	log.Printf("Opportunities: %d", oppCount)
	log.Printf("Roles:         %d", roleCount)

	log.Println("\n--- Stage Breakdown ---")
	for _, s := range stageStats {
		log.Printf("  %-15s: %d", s.Stage, s.Count)
	}

	log.Println("\n--- Top Customers by Opportunities ---")
	for i, c := range topCustomers {
		log.Printf("  %2d. %-30s (%d)", i+1, c.BusinessName, c.Count)
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
