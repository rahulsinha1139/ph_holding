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

// Customer name normalization map - EXACT lowercase matches for ALL 67 End Users
// Target: ~30 consolidated customers from 67 unique entries
var customerMap = map[string]string{
	// === ALBA (49 opps) ===
	"aluminium bahrain b.s.c.(c)": "ALBA",
	"aluminium bahrain":           "ALBA",
	"alba":                        "ALBA",

	// === BAPCO GROUP (33 opps combined) ===
	"bapco":              "BAPCO",
	"bapco refining":     "BAPCO",
	"bapco upstream w.l.l": "BAPCO",
	"bapco upstream":     "BAPCO",

	// === ENGIE (27 opps) ===
	"al ezzel engie o&m co. w.l.l": "Engie",
	"al ezzel engie":               "Engie",
	"engie":                        "Engie",

	// === SULB (19 opps combined) ===
	"bahrain steel bscc (e.c.) - sulb": "SULB",
	"bahrain steel":                    "SULB",
	"sulb":                             "SULB",

	// === VEOLIA (16 opps combined) ===
	"veolia energy gulf w.l.l.":     "Veolia",
	"veolia energy gulf":            "Veolia",
	"veolia water tech & solutions": "Veolia",
	"veolia":                        "Veolia",

	// === MUHARRAQ WASTEWATER (15 opps) ===
	"muharraq wastewater services co": "Muharraq Wastewater",
	"muharraq wastewater":             "Muharraq Wastewater",

	// === SEAPEAK/LNG (14 opps combined) ===
	"bahrain lng/ seapeak": "Seapeak/LNG",
	"bahrain lng":          "Seapeak/LNG",
	"seapeak":              "Seapeak/LNG",

	// === MINISTRY OF WORKS (13 opps) ===
	"ministry of works": "Ministry of Works",
	"mow":               "Ministry of Works",

	// === EWA (13 opps combined) ===
	"ewa": "EWA",

	// === ARLA FOODS (12 opps) ===
	"arla foods bahrain s.p.c": "Arla Foods",
	"arla foods bahrain":       "Arla Foods",
	"arla":                     "Arla Foods",

	// === TTSJV (10 opps) ===
	"ttsjv": "TTSJV",

	// === BAHRAIN SUGAR REFINERY (10 opps combined) ===
	"bahrain sugar refinery": "Bahrain Sugar Refinery",
	"baharain sugar":         "Bahrain Sugar Refinery",
	"bahrain sugar":          "Bahrain Sugar Refinery",

	// === GPIC (7 opps) ===
	"gpic": "GPIC",

	// === TABREED (7 opps combined) ===
	"tabreed": "Tabreed",

	// === HIDD POWER CO (7 opps combined) ===
	"hidd power co. bsc": "Hidd Power Co",
	"hidd power":         "Hidd Power Co",
	"hpc":                "Hidd Power Co",

	// === NOMAC (6 opps) ===
	"nomac": "NOMAC",

	// === WABAG (5 opps) ===
	"wabag belhasa jv wll": "Wabag",
	"wabag":                "Wabag",

	// === AL MOAYYED (5 opps) ===
	"almoayyed":  "Al Moayyed",
	"al moayyed": "Al Moayyed",

	// === YOKOGAWA (4 opps) ===
	"yokogawa": "Yokogawa",

	// === INTERCOL (4 opps combined) ===
	"intercol": "Intercol",

	// === GARMCO (4 opps combined) ===
	"garmco": "GARMCO",
	"gramco": "GARMCO",

	// === BANAGAS (3 opps) ===
	"banagas": "BANAGAS",

	// === ASIABAH (3 opps) ===
	"asiabah accelerated services": "ASIABAH",
	"asiabah":                      "ASIABAH",

	// === AL BAKALI (3 opps) ===
	"al bakali": "Al Bakali",

	// === DELTA BAHRAIN (3 opps) ===
	"delta bahrain": "Delta Bahrain",
	"delta":         "Delta Bahrain",

	// === AWAL DAIRY (3 opps) ===
	"awal dairy": "Awal Dairy",

	// === ALUSERV (3 opps) ===
	"aluserv middle east w.l.l.": "AluServ",
	"aluserv":                    "AluServ",

	// === WEST POINT (2 opps) ===
	"west point": "West Point",

	// === PENTIUM TECHNOLOGY (2 opps) ===
	"pentium technology": "Pentium Technology",

	// === SERVIQ INTERNATIONAL (2 opps) ===
	"serviq international trading": "Serviq International",
	"serviq":                       "Serviq International",

	// === ONE-OFF CUSTOMERS (1 opp each) ===
	"agile arabia co. w.l.l.":                    "Agile Arabia",
	"kraum & spruk technologies":                 "Kraum & Spruk",
	"orbitus/sedres":                             "Orbitus/Sedres",
	"tecton":                                     "Tecton",
	"al madar trading":                           "Al Madar Trading",
	"emaar":                                      "EMAAR",
	"wahat al ansari":                            "Wahat Al Ansari",
	"airmech w.l.l":                              "Airmech",
	"gulf market":                                "Gulf Market",
	"sepcon":                                     "Sepcon",
	"abuobaid":                                   "Abuobaid",
	"mcsc":                                       "MCSC",
	"al hillo trdg co w.l.l,":                    "Al Hillo Trading",
	"arrowfinch":                                 "Arrowfinch",
	"jbf filims":                                 "JBF Films",
	"al mufeed electrical services w.l.l":        "Al Mufeed Electrical",
	"flora contracting maintenance services w.l.l": "Flora Contracting",
	"kuwait oil company":                         "Kuwait Oil Company",
	"transglobal contracting":                    "Transglobal Contracting",
	"saudi electricity company":                  "Saudi Electricity Company",
	"marasi mall":                                "Marasi Mall",
	"omni":                                       "OMNI",
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
	log.Println("=== PH HOLDINGS - CLEAN DATA IMPORT ===")

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

	// Seed suppliers
	log.Println("\n=== Seeding suppliers ===")
	seedSuppliers(db)

	// Import opportunities
	log.Println("\n=== Importing opportunities ===")
	importOpportunities(db, "../../Data_for_database/opportunities created 2025.xlsx")

	printSummary(db)
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

func seedSuppliers(db *gorm.DB) {
	suppliers := []SupplierMaster{
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "EH-001", SupplierName: "Endress+Hauser", Country: "Germany", SupplierType: "Manufacturer", PaymentTerms: "Net 60", Rating: 5},
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "SERVO-001", SupplierName: "Servomex", Country: "UK", SupplierType: "Manufacturer", PaymentTerms: "Net 45", Rating: 5},
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "LG-001", SupplierName: "Landis+Gyr", Country: "Switzerland", SupplierType: "Manufacturer", PaymentTerms: "Net 30", Rating: 4},
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "YOKO-001", SupplierName: "Yokogawa", Country: "Japan", SupplierType: "Manufacturer", PaymentTerms: "Net 60", Rating: 5},
		{Base: Base{ID: uuid.New().String()}, SupplierCode: "SICK-001", SupplierName: "SICK AG", Country: "Germany", SupplierType: "Manufacturer", PaymentTerms: "Net 45", Rating: 4},
	}
	for _, s := range suppliers {
		db.Create(&s)
	}
	log.Printf("Created %d suppliers", len(suppliers))
}

func importOpportunities(db *gorm.DB, filepath string) {
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

	customers := make(map[string]string) // normalized name -> ID
	customerCount := 0
	oppCount := 0

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

		// Create unique folder number
		if folderNo == "" {
			folderNo = fmt.Sprintf("UNK-%d", rowNum+1)
		}

		// Normalize customer name
		// If End User is empty or just a number, extract from folder name
		customerName := ""
		if isValidCustomerName(endUser) {
			customerName = normalizeCustomer(endUser)
		}
		if customerName == "" {
			// Try extracting from folder name (e.g., "302- ENGIE FIT" -> "ENGIE")
			extracted := extractFromFolder(folderName)
			customerName = normalizeCustomer(extracted)
		}

		// Create customer if new
		customerID := ""
		if customerName != "" {
			key := strings.ToLower(customerName)
			if existingID, ok := customers[key]; ok {
				customerID = existingID
			} else {
				customerID = uuid.New().String()
				customerCount++
				db.Create(&CustomerMaster{
					Base:           Base{ID: customerID},
					CustomerID:     fmt.Sprintf("CUST-%04d", customerCount),
					CustomerCode:   fmt.Sprintf("C%04d", customerCount),
					BusinessName:   customerName,
					Country:        "Bahrain",
					PaymentGrade:   "C",
					CustomerGrade:  "C",
					CreditLimitBHD: 50000,
				})
				customers[key] = customerID
			}
		}

		// Map status
		stage := mapStatus(status)

		// Parse date
		offerDate := parseDate(quoteDateStr)
		if offerDate.IsZero() {
			// Use year if available
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

	log.Printf("Created %d customers, %d opportunities", customerCount, oppCount)
}

// isValidCustomerName checks if the End User value is a real customer name
// Returns false if it's empty, just a number, or looks like a folder reference
func isValidCustomerName(endUser string) bool {
	endUser = strings.TrimSpace(endUser)
	if endUser == "" {
		return false
	}

	// If it's just a number, it's a folder number not a customer
	if _, err := strconv.Atoi(endUser); err == nil {
		return false
	}

	// If it starts with a number followed by space or dash, it's likely folder info
	// e.g., "35 Al faris", "164 E&H KSA SHIPMENT", "AIC PIT 221"
	if matched, _ := regexp.MatchString(`^\d+\s`, endUser); matched {
		return false
	}

	// If it's "TENDER" followed by something, skip it
	if strings.HasPrefix(strings.ToUpper(endUser), "TENDER") {
		return false
	}

	// If it looks like a technical reference (all caps + numbers)
	if matched, _ := regexp.MatchString(`^[A-Z]+\s*(PIT|FIT|LIT|TIT)?\s*\d+$`, strings.ToUpper(endUser)); matched {
		return false
	}

	return true
}

func normalizeCustomer(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Normalize spaces (multiple spaces → single)
	spaceRe := regexp.MustCompile(`\s+`)
	key := strings.ToLower(spaceRe.ReplaceAllString(name, " "))

	// Try exact match first
	if canonical, ok := customerMap[key]; ok {
		return canonical
	}

	// Try without trailing punctuation
	keyClean := strings.TrimRight(key, ".,;:")
	if canonical, ok := customerMap[keyClean]; ok {
		return canonical
	}

	// Try keyword-based matching for major customers
	keywordMap := map[string]string{
		"aluminium bahrain": "ALBA",
		"alba":              "ALBA",
		"bapco":             "BAPCO",
		"engie":             "Engie",
		"al ezzel":          "Engie",
		"sulb":              "SULB",
		"bahrain steel":     "SULB",
		"veolia":            "Veolia",
		"muharraq":          "Muharraq Wastewater",
		"seapeak":           "Seapeak/LNG",
		"bahrain lng":       "Seapeak/LNG",
		"ministry of works": "Ministry of Works",
		"ewa":               "EWA",
		"arla":              "Arla Foods",
		"ttsjv":             "TTSJV",
		"sugar":             "Bahrain Sugar Refinery",
		"gpic":              "GPIC",
		"tabreed":           "Tabreed",
		"hidd power":        "Hidd Power Co",
		"hpc":               "Hidd Power Co",
		"nomac":             "NOMAC",
		"wabag":             "Wabag",
		"moayyed":           "Al Moayyed",
		"yokogawa":          "Yokogawa",
		"intercol":          "Intercol",
		"garmco":            "GARMCO",
		"gramco":            "GARMCO",
		"banagas":           "BANAGAS",
		"asiabah":           "ASIABAH",
		"aluserv":           "AluServ",
	}
	for keyword, canonical := range keywordMap {
		if strings.Contains(key, keyword) {
			return canonical
		}
	}

	// If still no match, check if it's a known small customer by partial match
	for pattern, canonical := range customerMap {
		if strings.Contains(key, pattern) {
			return canonical
		}
	}

	// Log unmapped customers for debugging
	log.Printf("UNMAPPED CUSTOMER: %q", name)

	// Clean up generic suffixes and return as-is
	name = strings.TrimSuffix(name, " W.L.L")
	name = strings.TrimSuffix(name, " W.L.L.")
	name = strings.TrimSuffix(name, " B.S.C")
	name = strings.TrimSuffix(name, " B.S.C.")
	name = strings.TrimSuffix(name, " SPC")
	name = strings.TrimSuffix(name, " S.P.C")

	return strings.TrimSpace(name)
}

// extractCustomerFromFolder extracts customer name from folder name like "302- ENGIE FIT" or "215-25 ARLA FIT"
func extractFromFolder(folderName string) string {
	if folderName == "" {
		return ""
	}

	// Pattern: "NUMBER- CUSTOMER SUFFIX" or "NUMBER-NUMBER CUSTOMER SUFFIX"
	// Examples: "302- ENGIE FIT", "215-25 ARLA FIT MASS", "306- ALBA LIT"

	// Remove the leading number and separator
	re := regexp.MustCompile(`^\d+[-\s]*\d*[-\s]+(.+)$`)
	if m := re.FindStringSubmatch(folderName); len(m) >= 2 {
		rest := strings.TrimSpace(m[1])
		// Extract first word as customer name (before technical suffixes)
		// Technical suffixes: FIT, LIT, TIT, PIT, SP, SPARES, MASS, COND, etc.
		words := strings.Fields(rest)
		if len(words) > 0 {
			// Take first 1-3 words until we hit a technical suffix
			techSuffixes := map[string]bool{
				"FIT": true, "LIT": true, "TIT": true, "PIT": true, "SP": true,
				"SPARES": true, "MASS": true, "COND": true, "SERVICE": true,
				"FEED": true, "CABLE": true, "CALIBRATION": true, "SGR": true,
			}
			var customerWords []string
			for _, w := range words {
				if techSuffixes[strings.ToUpper(w)] {
					break
				}
				customerWords = append(customerWords, w)
				// Stop after 3 words max
				if len(customerWords) >= 3 {
					break
				}
			}
			if len(customerWords) > 0 {
				return strings.Join(customerWords, " ")
			}
		}
	}

	// Fallback: try simple split on "-"
	parts := strings.SplitN(folderName, "-", 2)
	if len(parts) >= 2 {
		return strings.TrimSpace(parts[1])
	}
	return ""
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
	case strings.Contains(status, "follow-up"), strings.Contains(status, "evaluation"):
		return "Lead"
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
		LIMIT 15
	`).Scan(&topCustomers)

	log.Println("\n==========================================")
	log.Println("       CLEAN DATABASE IMPORT COMPLETE")
	log.Println("==========================================")
	log.Printf("Customers:     %d", custCount)
	log.Printf("Suppliers:     %d", suppCount)
	log.Printf("Opportunities: %d", oppCount)
	log.Printf("Roles:         %d", roleCount)

	log.Println("\n--- Stage Breakdown ---")
	for _, s := range stageStats {
		log.Printf("  %-15s: %d", s.Stage, s.Count)
	}

	log.Println("\n--- Top Customers ---")
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
