package main

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// =============================================================================
// MASTER DATA IMPORT - CUSTOMERS & SUPPLIERS
// =============================================================================
// NOTE: All model definitions (CustomerMaster, SupplierMaster, Base, etc.)
// are in models.go. This file only contains import logic.

// =============================================================================
// CUSTOMER NORMALIZATION MAP (FROM import_clean/main.go)
// =============================================================================

// Customer name normalization map - EXACT lowercase matches for ALL 67 End Users
// Target: ~30 consolidated customers from 67 unique entries
var customerNormalizationMap = map[string]string{
	// === ALBA (49 opps) ===
	"aluminium bahrain b.s.c.(c)": "ALBA",
	"aluminium bahrain":           "ALBA",
	"alba":                        "ALBA",

	// === BAPCO GROUP (33 opps combined) ===
	"bapco":                "BAPCO",
	"bapco refining":       "BAPCO",
	"bapco upstream w.l.l": "BAPCO",
	"bapco upstream":       "BAPCO",

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
	"agile arabia co. w.l.l.":             "Agile Arabia",
	"kraum & spruk technologies":          "Kraum & Spruk",
	"orbitus/sedres":                      "Orbitus/Sedres",
	"tecton":                              "Tecton",
	"al madar trading":                    "Al Madar Trading",
	"emaar":                               "EMAAR",
	"wahat al ansari":                     "Wahat Al Ansari",
	"airmech w.l.l":                       "Airmech",
	"gulf market":                         "Gulf Market",
	"sepcon":                              "Sepcon",
	"abuobaid":                            "Abuobaid",
	"mcsc":                                "MCSC",
	"al hillo trdg co w.l.l,":             "Al Hillo Trading",
	"arrowfinch":                          "Arrowfinch",
	"jbf filims":                          "JBF Films",
	"al mufeed electrical services w.l.l": "Al Mufeed Electrical",
	"flora contracting maintenance services w.l.l": "Flora Contracting",
	"kuwait oil company":                           "Kuwait Oil Company",
	"transglobal contracting":                      "Transglobal Contracting",
	"saudi electricity company":                    "Saudi Electricity Company",
	"marasi mall":                                  "Marasi Mall",
	"omni":                                         "OMNI",
}

// Keyword-based matching for major customers (fallback)
var customerKeywordMap = map[string]string{
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

// =============================================================================
// CORE IMPORT FUNCTIONS
// =============================================================================

// importMasterData imports customers and suppliers, returns maps for downstream agents
func importMasterData(db *gorm.DB) (customerMap map[string]string, supplierMap map[string]string, err error) {
	log.Println("\n=== IMPORTING MASTER DATA (2025) ===")

	// Import customers from 2025 invoices
	customerMap, err = importCustomers(db)
	if err != nil {
		return nil, nil, fmt.Errorf("customer import failed: %w", err)
	}

	// Import suppliers (seed core + extract from 2025 purchases)
	supplierMap, err = importSuppliers(db)
	if err != nil {
		return nil, nil, fmt.Errorf("supplier import failed: %w", err)
	}

	log.Printf("\n✅ Master data import complete: %d customers, %d suppliers", len(customerMap), len(supplierMap))
	return customerMap, supplierMap, nil
}

// importCustomers extracts unique customers from 2025 invoices
func importCustomers(db *gorm.DB) (map[string]string, error) {
	log.Println("\n→ Importing customers from invoices 2025.xlsx")

	// Open Excel file
	filepath := "Data_for_database/Tally_data/invoices 2025.xlsx"
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", filepath, err)
	}
	defer f.Close()

	// Get first sheet name dynamically
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, fmt.Errorf("no sheets found in Excel file")
	}
	sheetName := sheets[0]

	// Get rows
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet %s: %w", sheetName, err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("file has no data rows")
	}

	log.Printf("  Reading sheet: %s (%d rows)", sheetName, len(rows))

	// Find actual header row (Tally exports have company info in first rows)
	headerRowIdx, header := findHeaderRow(rows)
	log.Printf("  Header found at row %d: %v", headerRowIdx+1, header)

	// Parse header
	colMap := make(map[string]int)
	for i, colName := range header {
		normalized := strings.ToLower(strings.TrimSpace(colName))
		colMap[normalized] = i
	}

	// Find customer column
	customerCol := findColumn(colMap, "customer", "customer name", "party", "party name", "ledger name", "particulars")
	if customerCol == -1 {
		return nil, fmt.Errorf("customer column not found. Headers: %v", header)
	}

	// Extract unique customer names (skip header row)
	uniqueCustomers := make(map[string]bool)
	dataStartRow := headerRowIdx + 1
	for i, row := range rows[dataStartRow:] {
		if customerCol >= len(row) {
			log.Printf("⚠ Row %d: customer column out of bounds, skipping", dataStartRow+i+1)
			continue
		}

		customerName := strings.TrimSpace(row[customerCol])
		if customerName == "" {
			continue
		}

		// Normalize the customer name
		normalized := normalizeCustomerName(customerName)
		if normalized != "" {
			uniqueCustomers[normalized] = true
		}
	}

	log.Printf("  Found %d unique customers (normalized)", len(uniqueCustomers))

	// Create customer records
	customerMap := make(map[string]string)
	customerCount := 0

	for normalizedName := range uniqueCustomers {
		customerCount++
		customerID := uuid.New().String()

		customer := CustomerMaster{
			Base: Base{
				ID:        customerID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			CompanyName:     normalizedName,
			Country:         "Bahrain",
			Grade:           "B", // Default grade
			CreditLimit:     50000,
			PaymentTerms:    "Net 30",
			IsActive:        true,
			TallyLedgerName: normalizedName, // Use normalized name since raw name was lost during map iteration
		}

		if err := db.Create(&customer).Error; err != nil {
			log.Printf("⚠ Failed to create customer %s: %v", normalizedName, err)
			continue
		}

		customerMap[normalizedName] = customerID
	}

	log.Printf("  Created %d customer records", len(customerMap))
	return customerMap, nil
}

// importSuppliers seeds core suppliers + extracts from 2025 purchases
func importSuppliers(db *gorm.DB) (map[string]string, error) {
	log.Println("\n→ Importing suppliers (seed core + extract from purchases 2025.xlsx)")

	supplierMap := make(map[string]string)

	// 1. Seed core suppliers (5 major manufacturers)
	coreSuppliers := []struct {
		Name    string
		Code    string
		Country string
		Type    string
		Terms   string
		Rating  int
	}{
		{"Endress+Hauser", "EH-001", "Germany", "Manufacturer", "Net 60", 5},
		{"Servomex", "SERVO-001", "UK", "Manufacturer", "Net 45", 5},
		{"Landis+Gyr", "LG-001", "Switzerland", "Manufacturer", "Net 30", 4},
		{"Yokogawa", "YOKO-001", "Japan", "Manufacturer", "Net 60", 5},
		{"SICK AG", "SICK-001", "Germany", "Manufacturer", "Net 45", 4},
	}

	for _, s := range coreSuppliers {
		supplierID := uuid.New().String()
		rating := "★★★" // Default 3 stars
		if s.Rating >= 5 {
			rating = "★★★★★"
		} else if s.Rating >= 4 {
			rating = "★★★★"
		}
		supplier := SupplierMaster{
			Base: Base{
				ID:        supplierID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			CompanyName:     s.Name,
			Country:         s.Country,
			PaymentTerms:    s.Terms,
			Rating:          rating,
			IsActive:        true,
			TallyLedgerName: s.Name,
		}

		if err := db.Create(&supplier).Error; err != nil {
			log.Printf("⚠ Failed to create core supplier %s: %v", s.Name, err)
			continue
		}

		supplierMap[s.Name] = supplierID
	}

	log.Printf("  Seeded %d core suppliers", len(supplierMap))

	// 2. Extract from purchase 2025.xlsx
	filepath := "Data_for_database/Tally_data/purchase 2025.xlsx"
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		// Not critical - core suppliers already created
		log.Printf("⚠ Could not open %s: %v", filepath, err)
		return supplierMap, nil
	}
	defer f.Close()

	// Get first sheet name dynamically
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		log.Printf("⚠ No sheets found in purchase 2025.xlsx")
		return supplierMap, nil
	}
	sheetName := sheets[0]

	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Printf("⚠ Failed to read sheet %s: %v", sheetName, err)
		return supplierMap, nil
	}

	if len(rows) < 2 {
		log.Printf("⚠ purchase 2025.xlsx has no data rows")
		return supplierMap, nil
	}

	log.Printf("  Reading sheet: %s (%d rows)", sheetName, len(rows))

	// Parse header
	header := rows[0]
	colMap := make(map[string]int)
	for i, colName := range header {
		normalized := strings.ToLower(strings.TrimSpace(colName))
		colMap[normalized] = i
	}

	// Find supplier column
	supplierCol := findColumn(colMap, "supplier", "supplier name", "party", "party name", "vendor")
	if supplierCol == -1 {
		log.Printf("⚠ Supplier column not found in purchases. Headers: %v", header)
		return supplierMap, nil
	}

	// Extract unique suppliers
	uniqueSuppliers := make(map[string]bool)
	for i, row := range rows[1:] {
		if supplierCol >= len(row) {
			log.Printf("⚠ Row %d: supplier column out of bounds, skipping", i+2)
			continue
		}

		supplierName := strings.TrimSpace(row[supplierCol])
		if supplierName == "" {
			continue
		}

		// Normalize supplier name
		supplierName = normalizeSupplierName(supplierName)
		if supplierName != "" && supplierMap[supplierName] == "" {
			uniqueSuppliers[supplierName] = true
		}
	}

	log.Printf("  Found %d additional suppliers from purchases", len(uniqueSuppliers))

	// Create additional supplier records
	supplierCount := len(supplierMap)
	for supplierName := range uniqueSuppliers {
		supplierCount++
		supplierID := uuid.New().String()

		supplier := SupplierMaster{
			Base: Base{
				ID:        supplierID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			CompanyName:     supplierName,
			Country:         "Unknown",
			PaymentTerms:    "Net 30",
			Rating:          "★★★", // 3-star default
			IsActive:        true,
			TallyLedgerName: supplierName,
		}

		if err := db.Create(&supplier).Error; err != nil {
			log.Printf("⚠ Failed to create supplier %s: %v", supplierName, err)
			continue
		}

		supplierMap[supplierName] = supplierID
	}

	log.Printf("  Total suppliers created: %d", len(supplierMap))
	return supplierMap, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// normalizeCustomerName applies the normalization map
func normalizeCustomerName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Normalize spaces (multiple spaces → single)
	spaceRe := regexp.MustCompile(`\s+`)
	key := strings.ToLower(spaceRe.ReplaceAllString(name, " "))

	// Try exact match first
	if canonical, ok := customerNormalizationMap[key]; ok {
		return canonical
	}

	// Try without trailing punctuation
	keyClean := strings.TrimRight(key, ".,;:")
	if canonical, ok := customerNormalizationMap[keyClean]; ok {
		return canonical
	}

	// Try keyword-based matching for major customers
	for keyword, canonical := range customerKeywordMap {
		if strings.Contains(key, keyword) {
			return canonical
		}
	}

	// If still no match, check partial match in normalization map
	for pattern, canonical := range customerNormalizationMap {
		if strings.Contains(key, pattern) {
			return canonical
		}
	}

	// Clean up generic suffixes and return as-is
	name = strings.TrimSuffix(name, " W.L.L")
	name = strings.TrimSuffix(name, " W.L.L.")
	name = strings.TrimSuffix(name, " B.S.C")
	name = strings.TrimSuffix(name, " B.S.C.")
	name = strings.TrimSuffix(name, " SPC")
	name = strings.TrimSuffix(name, " S.P.C")
	name = strings.TrimSuffix(name, " Ltd")
	name = strings.TrimSuffix(name, " Limited")

	return strings.TrimSpace(name)
}

// normalizeSupplierName cleans supplier names
func normalizeSupplierName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Remove common suffixes
	name = strings.TrimSuffix(name, " Inc.")
	name = strings.TrimSuffix(name, " Inc")
	name = strings.TrimSuffix(name, " Ltd.")
	name = strings.TrimSuffix(name, " Ltd")
	name = strings.TrimSuffix(name, " GmbH")
	name = strings.TrimSuffix(name, " AG")
	name = strings.TrimSuffix(name, " Co.")
	name = strings.TrimSuffix(name, " W.L.L")
	name = strings.TrimSuffix(name, " W.L.L.")

	return strings.TrimSpace(name)
}

// findColumn finds column index by flexible matching
func findColumn(colMap map[string]int, variants ...string) int {
	for _, variant := range variants {
		normalized := strings.ToLower(variant)
		if idx, ok := colMap[normalized]; ok {
			return idx
		}
	}
	return -1
}

// findHeaderRow finds the actual header row in Tally exports
// (Tally exports often have company name, report title in first rows)
// Returns header row index and the header row data
func findHeaderRow(rows [][]string) (int, []string) {
	// Common column names that indicate a header row
	headerKeywords := []string{
		"customer", "party", "ledger", "account",
		"date", "voucher", "invoice",
		"amount", "debit", "credit", "total",
		"particulars", "narration",
	}

	for i, row := range rows {
		if i > 10 { // Don't search beyond first 10 rows
			break
		}

		// Count how many cells match header keywords
		matchCount := 0
		for _, cell := range row {
			cellLower := strings.ToLower(strings.TrimSpace(cell))
			for _, keyword := range headerKeywords {
				if strings.Contains(cellLower, keyword) {
					matchCount++
					break
				}
			}
		}

		// If we found 2+ matches, this is likely the header row
		if matchCount >= 2 {
			return i, row
		}
	}

	// Fallback to row 0 if no header found
	if len(rows) > 0 {
		return 0, rows[0]
	}
	return 0, nil
}
