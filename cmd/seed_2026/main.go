package main

import (
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Minimal GORM models matching the main app schema

type Base struct {
	ID        string         `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
	CreatedBy string
}

type CustomerMaster struct {
	Base
	CustomerID   string `gorm:"uniqueIndex;size:50"`
	CustomerCode string `gorm:"uniqueIndex;size:50"`
	CustomerType string `gorm:"index;size:50"`
	BusinessName string `gorm:"index;size:255"`
	ShortCode    string `gorm:"size:10"`
	City         string `gorm:"index;size:100"`
	Country      string `gorm:"index;size:100"`
	Industry     string `gorm:"index;size:100"`
}

func (CustomerMaster) TableName() string { return "customers" }

type Offer struct {
	Base
	OfferNumber     string    `gorm:"uniqueIndex;size:50"`
	RevisionNumber  int       `gorm:"index"`
	RFQID           string    `gorm:"index;size:36"`
	CustomerID      string    `gorm:"index;size:36"`
	CustomerName    string    `gorm:"index;size:255"`
	QuotationDate   time.Time `gorm:"index"`
	ValidityDate    time.Time
	TotalValueBHD   float64   `gorm:"not null;default:0"`
	EstimatedMargin float64   `gorm:"not null;default:0"`
	Stage           string    `gorm:"index;size:50"`
	PaymentTerms    string    `gorm:"type:varchar(500)"`
	DeliveryTerms   string    `gorm:"type:varchar(500)"`
	DeliveryWeeks   string    `gorm:"size:50"`
	CountryOfOrigin string    `gorm:"size:100"`
}

func (Offer) TableName() string { return "offers" }

type OfferItem struct {
	Base
	OfferID     string  `gorm:"index;size:36"`
	LineNumber  int
	ProductCode string  `gorm:"size:100"`
	Model       string  `gorm:"size:255"`
	Description string  `gorm:"type:varchar(2000)"`
	Quantity    float64
	UnitPrice   float64
	Equipment   string  `gorm:"size:255"`
	Currency    string  `gorm:"size:10"`
	FOB         float64
	TotalPrice  float64
}

func (OfferItem) TableName() string { return "offer_items" }

type Order struct {
	Base
	OrderNumber      string    `gorm:"uniqueIndex;size:50"`
	CustomerName     string    `gorm:"size:255"`
	CustomerID       string    `gorm:"index;size:36"`
	CustomerPONumber string    `gorm:"size:100"`
	GrandTotalBHD    float64   `gorm:"column:grand_total_bhd;type:decimal(15,3)"`
	TotalValueBHD    float64   `gorm:"column:total_value_bhd;type:decimal(15,3)"`
	OrderDate        time.Time `gorm:"index"`
	Status           string    `gorm:"index;size:30;default:'Confirmed'"`
	PaymentTerms     string    `gorm:"size:50"`
	DeliveryTerms    string    `gorm:"size:50"`
	OfferID          string    `gorm:"index;size:36"`
}

func (Order) TableName() string { return "orders" }

type OrderItem struct {
	Base
	OrderID          string  `gorm:"index;size:36"`
	LineNumber       int
	ProductCode      string  `gorm:"size:100"`
	Description      string  `gorm:"type:varchar(2000)"`
	Quantity         float64
	UnitPriceBHD     float64 `gorm:"column:unit_price_bhd;type:decimal(15,3)"`
	QuantityShipped  float64 `gorm:"column:quantity_shipped"`
	QuantityInvoiced float64 `gorm:"column:quantity_invoiced"`
}

func (OrderItem) TableName() string { return "order_items" }

type BankAccount struct {
	Base
	AccountID      string  `gorm:"index;size:36"`
	BankName       string  `gorm:"size:255"`
	AccountNumber  string  `gorm:"size:100"`
	AccountName    string  `gorm:"size:255"`
	Currency       string  `gorm:"size:3;default:'BHD'"`
	CurrentBalance float64 `gorm:"type:decimal(15,3)"`
	IsActive       bool    `gorm:"index;default:true"`
}

func (BankAccount) TableName() string { return "bank_accounts" }

type CompanyBankAccount struct {
	ID           string    `gorm:"primaryKey;size:36"`
	BankName     string    `gorm:"not null;size:100"`
	AccountName  string    `gorm:"not null;size:100"`
	AccountNumber string   `gorm:"not null;size:50"`
	IBAN         string    `gorm:"size:50"`
	SwiftBIC     string    `gorm:"size:20"`
	Currency     string    `gorm:"size:3;default:'BHD'"`
	IsActive     bool      `gorm:"default:true"`
	DisplayOrder int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (CompanyBankAccount) TableName() string { return "company_bank_accounts" }

type BankStatement struct {
	Base
	BankAccountID   string    `gorm:"index;size:36"`
	StatementNumber string    `gorm:"uniqueIndex;size:50"`
	StatementDate   time.Time `gorm:"index"`
	PeriodStart     time.Time
	PeriodEnd       time.Time
	OpeningBalance  float64   `gorm:"type:decimal(15,3)"`
	ClosingBalance  float64   `gorm:"type:decimal(15,3)"`
	Currency        string    `gorm:"size:3;default:'BHD'"`
	TotalDebits     float64   `gorm:"type:decimal(15,3)"`
	TotalCredits    float64   `gorm:"type:decimal(15,3)"`
	DebitCount      int
	CreditCount     int
	Status          string    `gorm:"index;size:20;default:'Imported'"`
	ImportedFrom    string    `gorm:"size:500"`
	ImportMethod    string    `gorm:"size:20"`
}

func (BankStatement) TableName() string { return "bank_statements" }

type BankStatementLine struct {
	Base
	BankStatementID string    `gorm:"index;size:36"`
	LineNumber      int
	TransactionDate time.Time `gorm:"index"`
	ValueDate       time.Time
	Description     string    `gorm:"type:varchar(2000)"`
	Reference       string    `gorm:"index;size:100"`
	Debit           float64   `gorm:"type:decimal(15,3)"`
	Credit          float64   `gorm:"type:decimal(15,3)"`
	Balance         float64   `gorm:"type:decimal(15,3)"`
}

func (BankStatementLine) TableName() string { return "bank_statement_lines" }

// E+H XML types
type EHBasket struct {
	XMLName xml.Name `xml:"basket"`
	Header  EHHeader `xml:"header"`
	Items   []EHItem `xml:"item"`
}

type EHHeader struct {
	Pricing EHPricing `xml:"pricing"`
}

type EHPricing struct {
	TotalsSales EHTotalsSales `xml:"totalsSales"`
}

type EHTotalsSales struct {
	NetValue   EHCurrency `xml:"netValue"`
	GrossValue EHCurrency `xml:"grossValue"`
}

type EHCurrency struct {
	Value    float64 `xml:",chardata"`
	Currency string  `xml:"currency,attr"`
}

type EHItem struct {
	Product     EHProduct     `xml:"product"`
	ItemPricing EHItemPricing `xml:"itemPricing"`
}

type EHProduct struct {
	OrderCode     string     `xml:"orderCode"`
	OrderCodeLong string     `xml:"orderCodeLong"`
	Quantity      EHQuantity `xml:"quantity"`
	Texts         EHTexts    `xml:"texts"`
}

type EHQuantity struct {
	Value int    `xml:",chardata"`
	Unit  string `xml:"unit,attr"`
}

type EHTexts struct {
	ShortDescription string `xml:"shortDescription"`
}

type EHItemPricing struct {
	UnitSalesPrice EHCurrency `xml:"unitSalesPrice"`
	ItemSalesPrice EHCurrency `xml:"itemSalesPrice"`
}

// EUR to BHD conversion rate
const EUR_TO_BHD = 0.41

// Customer name mappings
var customerNameMap = map[string]string{
	"GPIC":           "Gulf Petrochemical Industries Co.",
	"ALBA":           "Aluminium Bahrain B.S.C.(C)",
	"BAPCO":          "Bapco Energies",
	"EWA":            "Electricity & Water Authority",
	"TTSJV":          "Technip-Technicas Reunidas JV",
	"INTERCOL":       "Intercol",
	"ARLA":           "Arla Foods Bahrain S.P.C",
	"VWT":            "Veolia Water Technologies",
	"VEOLIA":         "Veolia Water Technologies",
	"VEOLIA ENERGY":  "Veolia Water Technologies",
	"ABRAQYAH":       "Abraqyah",
	"NOMAC":          "Nomac Bahrain",
	"XENITT":         "Xenitt",
	"SEAPEAK":        "Seapeak Maritime",
	"AQUA TECH":      "Aqua Tech",
	"WABAG":          "WABAG",
	"ALDUR":          "Aldur",
	"YATEEM":         "Yateem Group",
	"ZOHAL":          "Zohal Trading",
	"ENGIE":          "Al Ezzel Engie O&M Co. W.L.L",
	"AIC":            "Arabian Industrial Chemicals",
	"SYSCON":         "Syscon",
	"AHS TRADING WLL": "AHS Trading WLL",
	"AHS":            "AHS Trading WLL",
}

var productTypeMap = map[string]string{
	"TW":  "Thermowell",
	"FIT": "Flow Instrument",
	"LIT": "Level Instrument",
	"TIT": "Temperature Instrument",
	"PIT": "Pressure Instrument",
	"AIT": "Analytical Instrument",
	"SP":  "Spare Parts",
	"DSP": "Display",
	"BTU": "BTU Meter",
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	dbFile := "ph_holdings.db"
	offersPath := `C:\Users\rishu\OneDrive\Desktop\PH_Holdings_Business_Docs\Offers No 1-50 (2026)`
	bankCSVPath := `C:\Users\rishu\OneDrive\Desktop\PH_Holdings_Business_Docs\Bank_Statements\AL SALAM 1-1-26 TO 31-1-26.csv`

	fmt.Println("=== PH Holdings 2026 Data Seeder ===")
	fmt.Printf("Database: %s\n", dbFile)
	fmt.Printf("Offers: %s\n", offersPath)
	fmt.Printf("Bank CSV: %s\n", bankCSVPath)
	fmt.Println()

	// Open database
	db, err := gorm.Open(sqlite.Open(dbFile+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	fmt.Println("Database connected.")

	// Auto-migrate all tables (may not exist yet)
	db.AutoMigrate(&BankAccount{}, &BankStatement{}, &BankStatementLine{}, &Order{}, &OrderItem{})

	// Stats
	offersCreated := 0
	offersSkipped := 0
	itemsCreated := 0
	customersCreated := 0
	customersMatched := 0

	// ============================================================
	// PHASE 1: Import 2026 Offers from folder structure
	// ============================================================
	fmt.Println("\n--- PHASE 1: Importing 2026 Offers ---")

	entries, err := os.ReadDir(offersPath)
	if err != nil {
		log.Printf("WARNING: Failed to read offers directory: %v", err)
	} else {
		// Pattern: "01-26 GPIC TW" or "07-EWA- HAMALA-EH" (no 26)
		offerPattern := regexp.MustCompile(`^(\d+)\s*-?\s*26\s+(.+)$`)
		// Fallback for folders without "26": "07-EWA- HAMALA-EH"
		fallbackPattern := regexp.MustCompile(`^(\d+)\s*-\s*(.+)$`)

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			folderName := entry.Name()
			matches := offerPattern.FindStringSubmatch(folderName)
			if len(matches) < 3 {
				// Try fallback pattern
				matches = fallbackPattern.FindStringSubmatch(folderName)
				if len(matches) < 3 {
					log.Printf("  Skipping (no match): %s", folderName)
					continue
				}
			}

			offerNum := matches[1]
			customerProduct := strings.TrimSpace(matches[2])

			// Parse customer and product
			customerName, productType := parseCustomerAndProduct(customerProduct)
			if customerName == "" {
				continue
			}

			fullCustomerName := mapCustomerName(customerName)
			offerNumber := fmt.Sprintf("%s-26", offerNum)

			// Check if offer already exists
			var existingCount int64
			db.Model(&Offer{}).Where("offer_number = ?", offerNumber).Count(&existingCount)
			if existingCount > 0 {
				offersSkipped++
				continue
			}

			// Find or create customer
			customer, created := findOrCreateCustomer(db, fullCustomerName, customerName)
			if created {
				customersCreated++
			} else {
				customersMatched++
			}

			// Determine stage from folder structure
			folderPath := filepath.Join(offersPath, folderName)
			stage := determineStage(folderPath)

			// Get folder mod time for quotation date
			info, _ := entry.Info()
			quotationDate := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
			if info != nil {
				quotationDate = info.ModTime()
			}

			// Create offer
			offer := Offer{
				Base: Base{
					ID:        uuid.New().String(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				OfferNumber:     offerNumber,
				RevisionNumber:  0,
				CustomerID:      customer.ID,
				CustomerName:    fullCustomerName,
				QuotationDate:   quotationDate,
				ValidityDate:    quotationDate.AddDate(0, 1, 0),
				Stage:           stage,
				PaymentTerms:    "30 days from Date of Delivery",
				DeliveryTerms:   "DAP Bahrain",
				DeliveryWeeks:   "4-6 weeks",
				CountryOfOrigin: "Germany / USA",
			}

			if err := db.Create(&offer).Error; err != nil {
				log.Printf("  ERROR creating offer %s: %v", offerNumber, err)
				continue
			}

			offersCreated++

			// Parse E+H XML files for line items
			xmlItems := parseXMLFiles(db, folderPath, offer.ID)
			itemsCreated += xmlItems

			// Update offer total from items
			if xmlItems > 0 {
				var total float64
				db.Model(&OfferItem{}).Where("offer_id = ?", offer.ID).
					Select("COALESCE(SUM(quantity * unit_price), 0)").Scan(&total)
				db.Model(&Offer{}).Where("id = ?", offer.ID).Update("total_value_bhd", total)
			}

			fmt.Printf("  [%s] %s - %s (stage: %s, product: %s, items: %d)\n",
				offerNumber, fullCustomerName, stage, stage, productType, xmlItems)
		}
	}

	fmt.Printf("\nOffers: %d created, %d skipped (already exist)\n", offersCreated, offersSkipped)
	fmt.Printf("Customers: %d created, %d matched\n", customersCreated, customersMatched)
	fmt.Printf("Line items: %d created from E+H XML\n", itemsCreated)

	// ============================================================
	// PHASE 2: Import Bank Statement CSV
	// ============================================================
	fmt.Println("\n--- PHASE 2: Importing Bank Statement ---")
	bankTxns := importBankStatement(db, bankCSVPath)
	fmt.Printf("Bank transactions imported: %d\n", bankTxns)

	// ============================================================
	// PHASE 3: Seed Additional Bank Accounts
	// ============================================================
	fmt.Println("\n--- PHASE 3: Seeding Additional Bank Accounts ---")
	seedBankAccounts(db)

	// ============================================================
	// PHASE 4: Create Orders from Won Offers
	// ============================================================
	fmt.Println("\n--- PHASE 4: Creating Orders from Won Offers ---")
	ordersCreated := createOrdersFromWonOffers(db)
	fmt.Printf("Orders created: %d\n", ordersCreated)

	// ============================================================
	// Final Summary
	// ============================================================
	fmt.Println("\n=== SEED COMPLETE ===")
	fmt.Printf("Offers created: %d\n", offersCreated)
	fmt.Printf("Offer items: %d\n", itemsCreated)
	fmt.Printf("Customers created: %d\n", customersCreated)
	fmt.Printf("Bank transactions: %d\n", bankTxns)
	fmt.Printf("Orders created: %d\n", ordersCreated)

	// Show totals
	var totalOffers, totalCustomers, totalItems, totalOrders, totalOrderItems int64
	db.Model(&Offer{}).Count(&totalOffers)
	db.Model(&CustomerMaster{}).Count(&totalCustomers)
	db.Model(&OfferItem{}).Count(&totalItems)
	db.Model(&Order{}).Count(&totalOrders)
	db.Model(&OrderItem{}).Count(&totalOrderItems)
	fmt.Printf("\nDatabase totals: %d offers, %d offer items, %d customers, %d orders, %d order items\n",
		totalOffers, totalItems, totalCustomers, totalOrders, totalOrderItems)
}

func parseCustomerAndProduct(input string) (string, string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", ""
	}

	productCode := ""
	customerParts := parts

	if len(parts) > 1 {
		lastPart := strings.ToUpper(parts[len(parts)-1])
		if _, exists := productTypeMap[lastPart]; exists {
			productCode = lastPart
			customerParts = parts[:len(parts)-1]
		}
	}

	return strings.Join(customerParts, " "), productCode
}

func mapCustomerName(shortCode string) string {
	upper := strings.ToUpper(strings.TrimSpace(shortCode))

	if fullName, ok := customerNameMap[upper]; ok {
		return fullName
	}
	for code, fullName := range customerNameMap {
		if strings.Contains(upper, code) {
			return fullName
		}
	}
	return shortCode
}

func determineStage(folderPath string) string {
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return "RFQ"
	}

	hasExecution := false
	hasOffer := false

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := strings.ToUpper(entry.Name())
		if strings.Contains(name, "EXECUTION") {
			hasExecution = true
		}
		if strings.Contains(name, "OFFER") {
			hasOffer = true
		}
	}

	if hasExecution {
		return "Won"
	}
	if hasOffer {
		return "Quoted"
	}
	return "RFQ"
}

func findOrCreateCustomer(db *gorm.DB, businessName, shortCode string) (*CustomerMaster, bool) {
	customer := &CustomerMaster{}

	// Try to find existing
	result := db.Where("LOWER(business_name) LIKE ?", "%"+strings.ToLower(businessName)+"%").First(customer)
	if result.Error == nil {
		return customer, false
	}

	// Also try short code
	result = db.Where("LOWER(business_name) LIKE ?", "%"+strings.ToLower(shortCode)+"%").First(customer)
	if result.Error == nil {
		return customer, false
	}

	// Create new
	customer = &CustomerMaster{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		CustomerID:   fmt.Sprintf("CUST-%s", strings.ToUpper(strings.ReplaceAll(shortCode, " ", "-"))),
		CustomerCode: func() string {
			code := strings.ToUpper(strings.ReplaceAll(shortCode, " ", ""))
			if len(code) > 6 {
				code = code[:6]
			}
			return fmt.Sprintf("C-%s", code)
		}(),
		BusinessName: businessName,
		ShortCode:    shortCode,
		Industry:     "Process Instrumentation",
		Country:      "Bahrain",
		City:         "Manama",
	}

	if err := db.Create(customer).Error; err != nil {
		log.Printf("  WARNING: Failed to create customer %s: %v", businessName, err)
		// Try to find again in case of unique constraint
		db.Where("LOWER(business_name) LIKE ?", "%"+strings.ToLower(businessName)+"%").First(customer)
		return customer, false
	}

	fmt.Printf("  + Created customer: %s (%s)\n", businessName, customer.CustomerID)
	return customer, true
}

func parseXMLFiles(db *gorm.DB, offerFolderPath string, offerID string) int {
	count := 0

	filepath.Walk(offerFolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(info.Name()), ".xml") {
			return nil
		}

		n, parseErr := parseEHXML(db, path, offerID)
		if parseErr != nil {
			return nil // Skip non-E+H XML
		}
		count += n
		return nil
	})

	return count
}

func parseEHXML(db *gorm.DB, xmlPath string, offerID string) (int, error) {
	file, err := os.Open(xmlPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return 0, err
	}

	var basket EHBasket
	if err := xml.Unmarshal(data, &basket); err != nil {
		return 0, err
	}

	if len(basket.Items) == 0 {
		return 0, nil
	}

	for i, item := range basket.Items {
		qty := float64(item.Product.Quantity.Value)
		if qty == 0 {
			qty = 1
		}
		unitPriceEUR := item.ItemPricing.UnitSalesPrice.Value
		unitPriceBHD := unitPriceEUR * EUR_TO_BHD
		desc := item.Product.Texts.ShortDescription

		offerItem := OfferItem{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OfferID:     offerID,
			LineNumber:  i + 1,
			ProductCode: item.Product.OrderCode,
			Model:       item.Product.OrderCodeLong,
			Description: desc,
			Quantity:    qty,
			UnitPrice:   unitPriceBHD,
			Equipment:   desc,
			Currency:    "EUR",
			FOB:         unitPriceEUR * qty,
			TotalPrice:  unitPriceBHD * qty,
		}

		db.Create(&offerItem)
	}

	fmt.Printf("    Parsed XML: %s (%d items)\n", filepath.Base(xmlPath), len(basket.Items))
	return len(basket.Items), nil
}

func importBankStatement(db *gorm.DB, csvPath string) int {
	file, err := os.Open(csvPath)
	if err != nil {
		log.Printf("WARNING: Could not open bank CSV: %v", err)
		return 0
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		log.Printf("WARNING: Could not read bank CSV: %v", err)
		return 0
	}

	// Check if already imported
	var existingCount int64
	db.Model(&BankStatement{}).Where("statement_number LIKE ?", "ALSALAM-JAN-2026%").Count(&existingCount)
	if existingCount > 0 {
		fmt.Println("  Bank statement already exists, skipping.")
		return 0
	}

	// Find bank account ID
	bankAccountID := "bank-alsalam"
	var accountExists int64
	db.Model(&BankAccount{}).Where("id = ?", bankAccountID).Count(&accountExists)
	if accountExists == 0 {
		db.Model(&CompanyBankAccount{}).Where("id = ?", bankAccountID).Count(&accountExists)
	}

	// Create statement
	stmtID := uuid.New().String()
	statement := BankStatement{
		Base: Base{
			ID:        stmtID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		BankAccountID:   bankAccountID,
		StatementNumber: "ALSALAM-JAN-2026",
		StatementDate:   time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		PeriodStart:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:       time.Date(2026, 1, 31, 0, 0, 0, 0, time.UTC),
		Currency:        "BHD",
		Status:          "Imported",
		ImportedFrom:    csvPath,
		ImportMethod:    "CSV",
	}

	// Parse the CSV - find data rows after "Posting Date" header
	var openingBal, closingBal, totalDebits, totalCredits float64
	var lines []BankStatementLine
	headerFound := false
	lineNum := 0

	for _, record := range records {
		if len(record) == 0 {
			continue
		}

		firstCol := strings.TrimSpace(record[0])

		// Find header row
		if strings.Contains(firstCol, "Posting Date") {
			headerFound = true
			continue
		}

		if !headerFound {
			continue
		}

		// Handle Opening/Closing Balance
		desc := ""
		if len(record) > 3 {
			desc = strings.TrimSpace(record[3])
		}

		if firstCol == "" && strings.Contains(desc, "Opening Balance") {
			if len(record) > 5 {
				openingBal = parseAmount(record[5])
			}
			continue
		}
		if firstCol == "" && strings.Contains(desc, "Closing Balance") {
			if len(record) > 5 {
				closingBal = parseAmount(record[5])
			}
			continue
		}

		if firstCol == "" {
			continue
		}

		// Parse transaction
		txnDate, err := parseBankDate(firstCol)
		if err != nil {
			continue
		}

		valueDate := txnDate
		if len(record) > 1 {
			if vd, err := parseBankDate(strings.TrimSpace(record[1])); err == nil {
				valueDate = vd
			}
		}

		reference := ""
		if len(record) > 2 {
			reference = strings.TrimSpace(record[2])
		}

		var debit, credit float64
		if len(record) > 4 {
			amountStr := strings.TrimSpace(record[4])
			if strings.HasPrefix(amountStr, "(-)") {
				amountStr = strings.TrimPrefix(amountStr, "(-)")
				amountStr = strings.TrimSpace(amountStr)
				debit = parseAmount(amountStr)
			} else if amountStr != "" {
				credit = parseAmount(amountStr)
			}
		}

		var balance float64
		if len(record) > 5 {
			balance = parseAmount(record[5])
		}

		lineNum++
		line := BankStatementLine{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			BankStatementID: stmtID,
			LineNumber:      lineNum,
			TransactionDate: txnDate,
			ValueDate:       valueDate,
			Description:     desc,
			Reference:       reference,
			Debit:           debit,
			Credit:          credit,
			Balance:         balance,
		}
		lines = append(lines, line)

		if debit > 0 {
			totalDebits += debit
		}
		if credit > 0 {
			totalCredits += credit
		}

		fmt.Printf("  Txn: %s | %s | Debit: %.3f | Credit: %.3f | Bal: %.3f\n",
			txnDate.Format("02-Jan-2006"), desc, debit, credit, balance)
	}

	statement.OpeningBalance = openingBal
	statement.ClosingBalance = closingBal
	statement.TotalDebits = totalDebits
	statement.TotalCredits = totalCredits
	statement.DebitCount = len(lines)

	if err := db.Create(&statement).Error; err != nil {
		log.Printf("ERROR: Failed to create bank statement: %v", err)
		return 0
	}

	for _, line := range lines {
		db.Create(&line)
	}

	fmt.Printf("  Statement: Opening %.3f → Closing %.3f BHD (%d transactions)\n",
		openingBal, closingBal, len(lines))

	return len(lines)
}

func seedBankAccounts(db *gorm.DB) {
	accounts := []BankAccount{
		{
			Base:          Base{ID: "bank-kfh", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			BankName:      "Kuwait Finance House",
			AccountNumber: "KFH-PHTRADING",
			AccountName:   "P H TRADING WLL",
			Currency:      "BHD",
			IsActive:      true,
		},
		{
			Base:          Base{ID: "bank-nbb-euro", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			BankName:      "National Bank of Bahrain - Euro Call",
			AccountNumber: "NBB-EURO-PHTRADING",
			AccountName:   "P H TRADING WLL",
			Currency:      "EUR",
			IsActive:      true,
		},
	}

	for _, acct := range accounts {
		var exists int64
		db.Model(&BankAccount{}).Where("id = ?", acct.ID).Count(&exists)
		if exists > 0 {
			fmt.Printf("  Bank account already exists: %s\n", acct.BankName)
			continue
		}
		if err := db.Create(&acct).Error; err != nil {
			log.Printf("  WARNING: Failed to create bank account %s: %v", acct.BankName, err)
			continue
		}
		fmt.Printf("  + Created bank account: %s (%s)\n", acct.BankName, acct.Currency)
	}
}

func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	if s == "" || s == "-" {
		return 0
	}
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

func parseBankDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty")
	}
	formats := []string{
		"2 Jan 2006", "02 Jan 2006", "2 JAN 2006", "02 JAN 2006",
		"2006-01-02", "02/01/2006",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unknown format: %s", s)
}

func createOrdersFromWonOffers(db *gorm.DB) int {
	// Get all offers with "Won" stage
	var wonOffers []Offer
	if err := db.Where("stage = ?", "Won").Find(&wonOffers).Error; err != nil {
		log.Printf("WARNING: Failed to query won offers: %v", err)
		return 0
	}

	if len(wonOffers) == 0 {
		fmt.Println("  No won offers found to convert to orders.")
		return 0
	}

	ordersCreated := 0
	orderItemsCreated := 0
	orderNum := 1

	// Check highest existing order number to continue sequence
	var lastOrder Order
	if err := db.Order("created_at DESC").First(&lastOrder).Error; err == nil {
		// Extract number from ORD-2026-XXX
		if strings.HasPrefix(lastOrder.OrderNumber, "ORD-2026-") {
			numStr := strings.TrimPrefix(lastOrder.OrderNumber, "ORD-2026-")
			if n, err := strconv.Atoi(numStr); err == nil {
				orderNum = n + 1
			}
		}
	}

	for _, offer := range wonOffers {
		orderNumber := fmt.Sprintf("ORD-2026-%03d", orderNum)

		// Check if order already exists for this offer
		var existingCount int64
		db.Model(&Order{}).Where("offer_id = ?", offer.ID).Count(&existingCount)
		if existingCount > 0 {
			fmt.Printf("  Skipping %s (order already exists for offer %s)\n", orderNumber, offer.OfferNumber)
			continue
		}

		// Also check if order number exists
		db.Model(&Order{}).Where("order_number = ?", orderNumber).Count(&existingCount)
		if existingCount > 0 {
			orderNum++
			orderNumber = fmt.Sprintf("ORD-2026-%03d", orderNum)
		}

		// Determine order date (use quotation date or offset by a few days/weeks)
		orderDate := offer.QuotationDate.AddDate(0, 0, 7) // 1 week after quotation
		if orderDate.After(time.Now()) {
			orderDate = time.Now().AddDate(0, 0, -7) // Recent date
		}

		// Create order
		order := Order{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrderNumber:      orderNumber,
			CustomerName:     offer.CustomerName,
			CustomerID:       offer.CustomerID,
			CustomerPONumber: offer.OfferNumber, // Link back to offer number
			GrandTotalBHD:    offer.TotalValueBHD,
			TotalValueBHD:    offer.TotalValueBHD,
			OrderDate:        orderDate,
			Status:           "Confirmed",
			PaymentTerms:     offer.PaymentTerms,
			DeliveryTerms:    offer.DeliveryTerms,
			OfferID:          offer.ID,
		}

		if err := db.Create(&order).Error; err != nil {
			log.Printf("  ERROR creating order %s: %v", orderNumber, err)
			continue
		}

		ordersCreated++
		fmt.Printf("  [%s] %s - %.3f BHD (from offer %s)\n",
			orderNumber, offer.CustomerName, offer.TotalValueBHD, offer.OfferNumber)

		// Get offer items
		var offerItems []OfferItem
		if err := db.Where("offer_id = ?", offer.ID).Find(&offerItems).Error; err != nil {
			log.Printf("  WARNING: Failed to get offer items for %s: %v", offer.OfferNumber, err)
			continue
		}

		// Create order items
		for _, offerItem := range offerItems {
			unitPrice := offerItem.UnitPrice
			if offerItem.Quantity > 0 && offerItem.TotalPrice > 0 {
				unitPrice = offerItem.TotalPrice / offerItem.Quantity
			}

			orderItem := OrderItem{
				Base: Base{
					ID:        uuid.New().String(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				OrderID:          order.ID,
				LineNumber:       offerItem.LineNumber,
				ProductCode:      offerItem.ProductCode,
				Description:      offerItem.Description,
				Quantity:         offerItem.Quantity,
				UnitPriceBHD:     unitPrice,
				QuantityShipped:  0,
				QuantityInvoiced: 0,
			}

			if err := db.Create(&orderItem).Error; err != nil {
				log.Printf("  WARNING: Failed to create order item: %v", err)
				continue
			}
			orderItemsCreated++
		}

		fmt.Printf("    + %d order items created\n", len(offerItems))
		orderNum++
	}

	// Create additional manual orders for major customers (if none exist yet)
	createAdditionalOrders(db, &orderNum)

	fmt.Printf("  Total: %d orders, %d order items created\n", ordersCreated, orderItemsCreated)
	return ordersCreated
}

func createAdditionalOrders(db *gorm.DB, orderNum *int) {
	// Major customers with realistic orders for Jan-Feb 2026
	manualOrders := []struct {
		CustomerShortCode string
		Amount            float64
		Description       string
		OrderDate         time.Time
	}{
		{"GPIC", 15750.500, "Process instrumentation for refinery upgrade", time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)},
		{"ALBA", 28900.250, "Temperature sensors and transmitters", time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)},
		{"BAPCO", 42500.000, "Flow meters and control valves", time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)},
		{"VWT", 8350.750, "Level instruments for water treatment", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)},
		{"INTERCOL", 12450.000, "Analytical instruments", time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC)},
		{"GPIC", 6800.500, "Spare parts for existing installations", time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC)},
		{"EWA", 19200.000, "Pressure transmitters for water distribution", time.Date(2026, 2, 12, 0, 0, 0, 0, time.UTC)},
	}

	for _, mo := range manualOrders {
		// Find customer
		fullName := mapCustomerName(mo.CustomerShortCode)
		customer, _ := findOrCreateCustomer(db, fullName, mo.CustomerShortCode)

		orderNumber := fmt.Sprintf("ORD-2026-%03d", *orderNum)

		// Check if order exists
		var existingCount int64
		db.Model(&Order{}).Where("order_number = ?", orderNumber).Count(&existingCount)
		if existingCount > 0 {
			continue
		}

		order := Order{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrderNumber:      orderNumber,
			CustomerName:     fullName,
			CustomerID:       customer.ID,
			CustomerPONumber: fmt.Sprintf("PO-%s-%d", mo.CustomerShortCode, mo.OrderDate.Year()),
			GrandTotalBHD:    mo.Amount,
			TotalValueBHD:    mo.Amount,
			OrderDate:        mo.OrderDate,
			Status:           "Confirmed",
			PaymentTerms:     "Net 30",
			DeliveryTerms:    "DDP",
		}

		if err := db.Create(&order).Error; err != nil {
			log.Printf("  WARNING: Failed to create manual order %s: %v", orderNumber, err)
			continue
		}

		// Create single order item
		orderItem := OrderItem{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			OrderID:          order.ID,
			LineNumber:       1,
			ProductCode:      "MIXED",
			Description:      mo.Description,
			Quantity:         1,
			UnitPriceBHD:     mo.Amount,
			QuantityShipped:  0,
			QuantityInvoiced: 0,
		}

		db.Create(&orderItem)

		fmt.Printf("  [%s] %s - %.3f BHD (manual)\n", orderNumber, fullName, mo.Amount)
		(*orderNum)++
	}
}
