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
	BrandsHandled string `gorm:"type:varchar(2000)"`
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
	SPOCStatus     string     `gorm:"size:50"`
	WIPStatus      string     `gorm:"size:50"`
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
	log.Println("=== PH HOLDINGS - REAL DATA IMPORT ===")

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

	// Drop and recreate tables
	log.Println("\n=== Dropping existing tables ===")
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

	// Create tables
	log.Println("\n=== Creating tables ===")
	db.AutoMigrate(&CustomerMaster{}, &SupplierMaster{}, &Opportunity{}, &Role{})

	// Seed roles
	log.Println("\n=== Seeding roles ===")
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

	// Seed suppliers
	log.Println("\n=== Seeding suppliers ===")
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

	// Import opportunities from Excel
	log.Println("\n=== Importing opportunities ===")
	importOpportunities(db, "../../Data_for_database/opportunities created 2025.xlsx")

	// Summary
	printSummary(db)
}

func importOpportunities(db *gorm.DB, filepath string) {
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		log.Printf("ERROR: Cannot open %s: %v", filepath, err)
		return
	}
	defer f.Close()

	rows, err := f.GetRows(f.GetSheetName(0))
	if err != nil {
		log.Printf("ERROR: Cannot read rows: %v", err)
		return
	}

	if len(rows) < 2 {
		log.Println("No data rows")
		return
	}

	// Build column index from header
	header := rows[0]
	colIdx := make(map[string]int)
	for i, h := range header {
		colIdx[strings.TrimSpace(strings.ToLower(h))] = i
	}

	// Track unique customers
	customers := make(map[string]string) // name -> ID
	customerCount := 0
	oppCount := 0
	skipped := 0

	for rowNum, row := range rows[1:] {
		// Get values
		folderNo := getCell(row, colIdx, "folder no.", "folder no")
		folderName := getCell(row, colIdx, "folder name")
		endUser := getCell(row, colIdx, "end user")
		owner := getCell(row, colIdx, "owner")
		valueStr := getCell(row, colIdx, "value in bhd", " value in bhd ")
		status := getCell(row, colIdx, "status", " status ")
		lostReason := getCell(row, colIdx, "reason for loss", " reason for loss ")
		ehRef := getCell(row, colIdx, "e+h refference no", "e+h reference no")
		paymentTerms := getCell(row, colIdx, "payment terms", " payment terms ")
		quoteDateStr := getCell(row, colIdx, "quote date")
		orderDateStr := getCell(row, colIdx, "order date")
		comment := getCell(row, colIdx, "latest comment")
		year := getCell(row, colIdx, "year")

		// Skip empty rows
		if folderNo == "" && endUser == "" {
			skipped++
			continue
		}

		// Create unique folder number with year
		if folderNo == "" {
			folderNo = fmt.Sprintf("UNK-%d", rowNum+1)
		}
		if !strings.Contains(folderNo, "-") && year != "" {
			// Add year suffix if not present
			folderNo = folderNo + "-" + year[len(year)-2:]
		}

		// Clean customer name
		customerName := cleanCustomerName(endUser)
		if customerName == "" {
			customerName = extractCustomerFromFolder(folderName)
		}

		// Create or get customer
		customerID := ""
		if customerName != "" {
			normalizedName := strings.ToUpper(strings.TrimSpace(customerName))
			if existingID, ok := customers[normalizedName]; ok {
				customerID = existingID
			} else {
				customerID = uuid.New().String()
				customerCount++
				cust := CustomerMaster{
					Base:           Base{ID: customerID},
					CustomerID:     fmt.Sprintf("CUST-%04d", customerCount),
					CustomerCode:   fmt.Sprintf("C%04d", customerCount),
					BusinessName:   customerName,
					Country:        "Bahrain",
					PaymentGrade:   "C",
					CustomerGrade:  "C",
					CreditLimitBHD: 50000,
				}
				db.Create(&cust)
				customers[normalizedName] = customerID
			}
		}

		// Parse value
		value := parseValue(valueStr)

		// Determine stage from status
		stage := mapStatus(status)

		// Parse dates
		offerDate := parseDate(quoteDateStr)
		if offerDate.IsZero() {
			offerDate = time.Now().AddDate(0, -3, 0)
		}
		var expectedDate *time.Time
		if od := parseDate(orderDateStr); !od.IsZero() {
			expectedDate = &od
		}

		// Create opportunity
		opp := Opportunity{
			Base:           Base{ID: uuid.New().String()},
			FolderNumber:   folderNo,
			CustomerID:     customerID,
			CustomerName:   customerName,
			Salesperson:    cleanOwner(owner),
			OfferDate:      offerDate,
			ExpectedDate:   expectedDate,
			RevenueBHD:     value,
			Stage:          stage,
			EHReference:    ehRef,
			PaymentTerms:   paymentTerms,
			LostReason:     lostReason,
			Notes:          fmt.Sprintf("%s | %s", folderName, comment),
			SPOCStatus:     "",
			WIPStatus:      comment,
		}

		result := db.Create(&opp)
		if result.Error != nil {
			// Duplicate folder number - append row number
			opp.FolderNumber = fmt.Sprintf("%s-R%d", folderNo, rowNum)
			db.Create(&opp)
		}
		oppCount++
	}

	log.Printf("Imported %d opportunities, %d customers (skipped %d empty rows)", oppCount, customerCount, skipped)
}

func getCell(row []string, colIdx map[string]int, names ...string) string {
	for _, name := range names {
		if idx, ok := colIdx[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}
	}
	return ""
}

func cleanCustomerName(name string) string {
	name = strings.TrimSpace(name)
	// Remove common suffixes/prefixes that don't add value
	name = strings.TrimSuffix(name, " W.L.L")
	name = strings.TrimSuffix(name, " W.L.L.")
	name = strings.TrimSuffix(name, " B.S.C")
	name = strings.TrimSuffix(name, " B.S.C.")
	name = strings.TrimSuffix(name, " SPC")
	name = strings.TrimSuffix(name, " S.P.C")
	name = strings.TrimSuffix(name, " LLC")

	// But keep the full name for business purposes
	if len(name) < 2 {
		return ""
	}
	return name
}

func extractCustomerFromFolder(folderName string) string {
	// Try to extract customer name from folder name
	// Common patterns: "XX - CustomerName", "CustomerName - Description"
	parts := strings.SplitN(folderName, "-", 2)
	if len(parts) >= 2 {
		// Check if first part is a number
		if _, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
			return cleanCustomerName(parts[1])
		}
		return cleanCustomerName(parts[0])
	}
	return cleanCustomerName(folderName)
}

func parseValue(s string) float64 {
	s = strings.TrimSpace(s)
	// Remove currency symbols, commas, spaces
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.ReplaceAll(s, "bhd", "")

	// Extract number using regex
	re := regexp.MustCompile(`[\d.]+`)
	match := re.FindString(s)
	if match == "" {
		return 0
	}

	f, _ := strconv.ParseFloat(match, 64)
	return f
}

func mapStatus(status string) string {
	status = strings.ToLower(strings.TrimSpace(status))

	switch {
	case strings.Contains(status, "payment received"):
		return "Won"
	case strings.Contains(status, "closed") && strings.Contains(status, "lost"):
		return "Lost"
	case strings.Contains(status, "won"), strings.Contains(status, "order"):
		return "Won"
	case strings.Contains(status, "lost"), strings.Contains(status, "cancel"):
		return "Lost"
	case strings.Contains(status, "process"), strings.Contains(status, "progress"):
		return "Negotiation"
	case strings.Contains(status, "proposal"), strings.Contains(status, "quoted"):
		return "Proposal"
	case strings.Contains(status, "qualified"):
		return "Qualified"
	case strings.Contains(status, "expired"):
		return "Expired"
	default:
		return "Lead"
	}
}

func cleanOwner(owner string) string {
	owner = strings.TrimSpace(owner)
	// Normalize common names
	switch strings.ToLower(owner) {
	case "sundar", "suindar":
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

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	formats := []string{
		"2-Jan-06", "2-Jan-2006", "02-Jan-06", "02-Jan-2006",
		"2/1/2006", "02/01/2006", "1/2/2006", "01/02/2006",
		"2006-01-02", "2006/01/02",
		"Jan 2, 2006", "January 2, 2006",
	}

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

	// Get stage breakdown
	var stageStats []struct {
		Stage string
		Count int64
	}
	db.Model(&Opportunity{}).Select("stage, count(*) as count").Group("stage").Scan(&stageStats)

	// Get top customers
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
		LIMIT 10
	`).Scan(&topCustomers)

	log.Println("\n==========================================")
	log.Println("         DATABASE IMPORT COMPLETE")
	log.Println("==========================================")
	log.Printf("Customers:     %d", custCount)
	log.Printf("Suppliers:     %d", suppCount)
	log.Printf("Opportunities: %d", oppCount)
	log.Printf("Roles:         %d", roleCount)

	log.Println("\n--- Opportunity Stages ---")
	for _, s := range stageStats {
		log.Printf("  %s: %d", s.Stage, s.Count)
	}

	log.Println("\n--- Top 10 Customers by Opportunities ---")
	for i, c := range topCustomers {
		log.Printf("  %d. %s (%d)", i+1, c.BusinessName, c.Count)
	}

	log.Println("==========================================")
	log.Println("Database ready! Run: wails dev")
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
