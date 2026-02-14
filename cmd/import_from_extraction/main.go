package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ═══════════════════════════════════════════════════════════════════════════
// IMPORT FROM EXTRACTION - Batch Import Costing Data
// ═══════════════════════════════════════════════════════════════════════════
// Uses the extracted file lists to import costing data into the database.
// Reads from documents_costing_sheet.txt and processes all costing files.
// ═══════════════════════════════════════════════════════════════════════════

// Row constants
const (
	ROW_DATE           = 2
	ROW_PREPARED_BY    = 3
	ROW_CUSTOMER       = 4
	ROW_REFERENCE      = 5
	ROW_SUPPLIER       = 7
	ROW_EQUIPMENT      = 8
	ROW_MODEL          = 9
	ROW_SPECIFICATION  = 10
	ROW_QUANTITY       = 11
	ROW_FOB_EUR        = 12
	ROW_EXCHANGE_RATE  = 14
	ROW_TOTAL_COST     = 24
	ROW_MARKUP_PERCENT = 25
	ROW_SELLING_PRICE  = 26
	ROW_TOTAL_SUGGESTED = 28
	COL_FIRST_PRODUCT  = 3
	COL_MAX_PRODUCTS   = 50
)

// Database models matching existing schema
type CostingSheetData struct {
	ID              uint      `gorm:"primaryKey;autoIncrement"`
	OfferNumber     string    `gorm:"column:offer_number;size:50;index" json:"offer_number"`
	CustomerName    string    `gorm:"column:customer_name;size:255" json:"customer_name"`
	ProductType     string    `gorm:"column:product_type;size:50" json:"product_type"`
	TotalValueBHD   float64   `gorm:"column:total_value_bhd" json:"total_value_bhd"`
	LineItemCount   int       `gorm:"column:line_item_count" json:"line_item_count"`
	SourceFilePath  string    `gorm:"column:source_file_path;size:500" json:"source_file_path"`
	ExtractedAt     time.Time `gorm:"column:extracted_at" json:"extracted_at"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (CostingSheetData) TableName() string { return "costing_sheet_data" }

type CostingLineItem struct {
	ID                 uint      `gorm:"primaryKey;autoIncrement"`
	CostingSheetID     uint      `gorm:"index" json:"costing_sheet_id"`
	ProductNumber      int       `json:"product_number"`
	Equipment          string    `gorm:"size:255" json:"equipment"`
	Model              string    `gorm:"size:255" json:"model"`
	Specification      string    `gorm:"type:text" json:"specification"`
	Supplier           string    `gorm:"size:255" json:"supplier"`
	Quantity           float64   `json:"quantity"`
	FobEUR             float64   `json:"fob_eur"`
	ExchangeRate       float64   `json:"exchange_rate"`
	TotalCostBHD       float64   `json:"total_cost_bhd"`
	MarkupPercent      float64   `json:"markup_percent"`
	SellingPriceBHD    float64   `json:"selling_price_bhd"`
	TotalSuggestedBHD  float64   `json:"total_suggested_bhd"`
	CreatedAt          time.Time `json:"created_at"`
}

func (CostingLineItem) TableName() string { return "costing_line_items" }

var (
	offerFolderPat = regexp.MustCompile(`^(\d{1,3})[-\s](.+)$`)
	productTypePat = regexp.MustCompile(`[-\s](AIT|FIT|LIT|SP|TIT|FEED|ETS|TT|VALVE|SPARE|PIT|ORP|BTU|LS|SENSOR|SERVICE|CALIBRATION|WDD)[-\s]?`)
)

func main() {
	basePath := "."
	dbPath := "ph_holdings.db"

	if len(os.Args) > 1 {
		basePath = os.Args[1]
	}
	if len(os.Args) > 2 {
		dbPath = os.Args[2]
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("📥 IMPORT FROM EXTRACTION - Costing Data Import")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate tables
	if err := db.AutoMigrate(&CostingSheetData{}, &CostingLineItem{}); err != nil {
		log.Fatalf("Failed to migrate: %v", err)
	}
	fmt.Println("✅ Database tables ready")

	// Read costing sheet list
	listFile := filepath.Join(basePath, "documents_costing_sheet.txt")
	files, err := readFileList(listFile)
	if err != nil {
		log.Fatalf("Failed to read file list: %v", err)
	}

	fmt.Printf("📁 Found %d costing sheets to process\n\n", len(files))

	sheetsCreated := 0
	itemsCreated := 0
	totalValueBHD := 0.0
	errors := 0

	for i, filePath := range files {
		// Progress indicator
		if i%50 == 0 {
			fmt.Printf("  Processing %d/%d...\n", i, len(files))
		}

		// Debug first file
		if i == 0 {
			fmt.Printf("  First file path: %s\n", filePath)
		}

		// Extract offer info from path
		offerNum, customerName, productType := extractOfferInfo(filePath)

		// Parse the Excel file
		sheet, items, err := parseCosting(filePath)
		if err != nil {
			if errors < 5 {
				fmt.Printf("  ❌ Error parsing %s: %v\n", filepath.Base(filePath), err)
			}
			errors++
			continue
		}

		// Debug first successful parse
		if i == 0 || (sheetsCreated == 0 && len(items) > 0 && i < 20) {
			fmt.Printf("  📄 Parsed: offer=%s, customer=%s, items=%d, value=%.0f BHD\n",
				offerNum, customerName, len(items), sheet.TotalValueBHD)
		}

		// Override with path-based info if not found in file
		if sheet.CustomerName == "" && customerName != "" {
			sheet.CustomerName = customerName
		}
		if sheet.OfferNumber == "" && offerNum != "" {
			sheet.OfferNumber = offerNum
		}
		sheet.ProductType = productType
		sheet.SourceFilePath = filePath
		sheet.ExtractedAt = time.Now()

		// Skip files with no actual items (just templates)
		if len(items) == 0 || (len(items) == 1 && strings.Contains(items[0].Equipment, "Total for Order")) {
			if i < 5 {
				fmt.Printf("  ⏭️ Skipping template: %s\n", filepath.Base(filePath))
			}
			continue
		}

		// Check if already imported
		var existing CostingSheetData
		if db.Where("source_file_path = ?", filePath).First(&existing).Error == nil {
			continue // Skip duplicate
		}

		// Create sheet record (ID auto-generated)
		sheet.ID = 0 // Let DB auto-increment
		sheet.CreatedAt = time.Now()
		sheet.UpdatedAt = time.Now()

		if err := db.Create(&sheet).Error; err != nil {
			if sheetsCreated == 0 && errors < 3 {
				fmt.Printf("  ❌ DB error creating sheet: %v\n", err)
			}
			errors++
			continue
		}
		sheetsCreated++
		totalValueBHD += sheet.TotalValueBHD

		// Create line items (skip Total row)
		for _, item := range items {
			if strings.Contains(item.Equipment, "Total for Order") {
				continue // Skip total row
			}
			item.ID = 0 // Let DB auto-increment
			item.CostingSheetID = sheet.ID
			item.CreatedAt = time.Now()

			if err := db.Create(&item).Error; err != nil {
				if itemsCreated == 0 && errors < 3 {
					fmt.Printf("  ❌ DB error creating item: %v\n", err)
				}
				errors++
				continue
			}
			itemsCreated++
		}
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("📊 IMPORT SUMMARY")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Costing sheets created:  %d\n", sheetsCreated)
	fmt.Printf("Line items created:      %d\n", itemsCreated)
	fmt.Printf("Total value:             %.3f BHD (~$%.0f USD)\n", totalValueBHD, totalValueBHD*2.65)
	fmt.Printf("Errors:                  %d\n", errors)
	fmt.Println("═══════════════════════════════════════════════════════════════")
}

func readFileList(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var files []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			files = append(files, line)
		}
	}
	return files, scanner.Err()
}

func extractOfferInfo(filePath string) (offerNum, customer, productType string) {
	parts := strings.Split(filePath, string(filepath.Separator))

	for _, p := range parts {
		if match := offerFolderPat.FindStringSubmatch(p); len(match) > 2 {
			offerNum = match[1]
			rest := match[2]

			if ptMatch := productTypePat.FindStringSubmatch(rest); len(ptMatch) > 1 {
				productType = ptMatch[1]
				idx := strings.Index(rest, ptMatch[0])
				if idx > 0 {
					customer = strings.TrimSpace(rest[:idx])
				}
			} else {
				customer = strings.TrimSpace(rest)
			}
			customer = strings.Trim(customer, "- ")
			break
		}
	}
	return
}

func parseCosting(filePath string) (CostingSheetData, []CostingLineItem, error) {
	sheet := CostingSheetData{}
	var items []CostingLineItem

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return sheet, items, err
	}
	defer f.Close()

	// Find costing sheet
	sheetName := "Costing Sheet"
	sheets := f.GetSheetList()
	found := false
	for _, s := range sheets {
		if strings.EqualFold(s, "Costing Sheet") || strings.Contains(strings.ToLower(s), "costing") {
			sheetName = s
			found = true
			break
		}
	}
	if !found && len(sheets) > 1 {
		sheetName = sheets[1]
	}

	// Extract metadata
	sheet.CustomerName, _ = f.GetCellValue(sheetName, "B4")

	// Get shared quantity
	qtyStr, _ := f.GetCellValue(sheetName, "B11")
	sharedQty := parseFloat(qtyStr)
	if sharedQty == 0 {
		sharedQty = 1
	}

	totalValue := 0.0

	// Extract line items (HORIZONTAL layout)
	productNum := 0
	for col := COL_FIRST_PRODUCT; col <= COL_MAX_PRODUCTS; col++ {
		cellRef, _ := excelize.CoordinatesToCellName(col, ROW_EQUIPMENT)
		equipment, _ := f.GetCellValue(sheetName, cellRef)

		if isEmpty(equipment) || isPlaceholder(equipment) {
			continue
		}

		productNum++
		item := CostingLineItem{
			ProductNumber: productNum,
			Equipment:     equipment,
			Quantity:      sharedQty,
		}

		// Supplier
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_SUPPLIER)
		item.Supplier, _ = f.GetCellValue(sheetName, cellRef)

		// Model
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_MODEL)
		item.Model, _ = f.GetCellValue(sheetName, cellRef)

		// Specification
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_SPECIFICATION)
		item.Specification, _ = f.GetCellValue(sheetName, cellRef)

		// FOB EUR
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_FOB_EUR)
		val, _ := f.GetCellValue(sheetName, cellRef)
		item.FobEUR = parseFloat(val)

		// Exchange Rate
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_EXCHANGE_RATE)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.ExchangeRate = parseFloat(val)

		// Total Cost
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_TOTAL_COST)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.TotalCostBHD = parseFloat(val)

		// Markup
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_MARKUP_PERCENT)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.MarkupPercent = parseFloat(val)

		// Selling Price
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_SELLING_PRICE)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.SellingPriceBHD = parseFloat(val)

		// Total Suggested
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_TOTAL_SUGGESTED)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.TotalSuggestedBHD = parseFloat(val)

		items = append(items, item)
		totalValue += item.TotalSuggestedBHD
	}

	sheet.TotalValueBHD = totalValue
	sheet.LineItemCount = len(items)

	return sheet, items, nil
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || isPlaceholder(s) {
		return 0
	}
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.ReplaceAll(s, "EUR", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.TrimSpace(s)
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

func isPlaceholder(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
}

func isEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}
