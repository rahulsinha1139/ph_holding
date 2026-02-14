// ============================================================================
// EXCEL COSTING PARSER - Extract Line Items from PH Trading Costing Sheets
//
// MISSION: Parse PH Trading Costing MasterFile.xlsx files and extract:
//   - Metadata (customer, date, reference, etc.)
//   - Line items (products, quantities, prices - stored HORIZONTALLY!)
//   - Totals (subtotal, VAT, grand total)
//
// CRITICAL: Products are stored ACROSS COLUMNS (C, D, E...), NOT down rows!
// ============================================================================

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"github.com/xuri/excelize/v2"
)

// ============================================================================
// ROW CONSTANTS - Costing Sheet Layout
// ============================================================================

const (
	// Metadata rows
	ROW_DATE          = 2
	ROW_PREPARED_BY   = 3
	ROW_CUSTOMER      = 4
	ROW_REFERENCE     = 5

	// Product data rows
	ROW_SUPPLIER       = 7
	ROW_EQUIPMENT      = 8
	ROW_MODEL          = 9
	ROW_SPECIFICATION  = 10
	ROW_QUANTITY       = 11
	ROW_FOB_EUR        = 12
	ROW_FREIGHT_EUR    = 13
	ROW_EXCHANGE_RATE  = 14
	ROW_FOB_BHD        = 15
	ROW_FREIGHT_BHD    = 16
	ROW_CNF            = 17
	ROW_INSURANCE      = 18
	ROW_CUSTOMS        = 19
	ROW_LANDED_COST    = 20
	ROW_HANDLING       = 21
	ROW_FINANCE_CHARGES = 22
	ROW_OTHER_COSTS    = 23
	ROW_TOTAL_COST     = 24
	ROW_MARKUP_PERCENT = 25
	ROW_SELLING_PRICE  = 26
	ROW_SUGGESTED_PRICE = 27
	ROW_TOTAL_SUGGESTED = 28
	ROW_VAT_AMOUNT     = 29
	ROW_GRAND_TOTAL    = 30

	// Column constants
	COL_LABELS        = 1  // Column A - Row labels
	COL_VALUES        = 2  // Column B - Metadata values
	COL_FIRST_PRODUCT = 3  // Column C - First product
	COL_MAX_PRODUCTS  = 50 // Scan up to column 50 for products
	COL_TOTALS        = 13 // Column M - Totals (in standard template)

	// Sheet names
	SHEET_COSTING = "Costing Sheet"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// ExcelCostingData represents the complete parsed data from a costing sheet
type ExcelCostingData struct {
	FilePath  string                  `json:"file_path"`
	FileName  string                  `json:"file_name"`
	Metadata  ExcelCostingMetadata    `json:"metadata"`
	LineItems []ExcelCostingLineItem  `json:"line_items"`
	Totals    ExcelCostingTotals      `json:"totals"`
	ParsedAt  time.Time               `json:"parsed_at"`
	Warnings  []string                `json:"warnings"`
}

// ExcelCostingMetadata contains header information from the costing sheet
type ExcelCostingMetadata struct {
	Date             string `json:"date"`
	FolderNumber     string `json:"folder_number"`
	EstDelivery      string `json:"est_delivery"`
	PreparedBy       string `json:"prepared_by"`
	CostingID        string `json:"costing_id"`
	DeliveryTerms    string `json:"delivery_terms"`
	Customer         string `json:"customer"`
	ContactPerson    string `json:"contact_person"`
	OrderType        string `json:"order_type"`
	Reference        string `json:"reference"`
	PaymentTerms     string `json:"payment_terms"`
	CountryOfOrigin  string `json:"country_of_origin"`
}

// ExcelCostingLineItem represents a single product/line in the costing
type ExcelCostingLineItem struct {
	ProductNumber         int     `json:"product_number"`
	ColumnIndex           int     `json:"column_index"`
	Supplier              string  `json:"supplier"`
	Equipment             string  `json:"equipment"`
	Model                 string  `json:"model"`
	Specification         string  `json:"specification"`
	Quantity              float64 `json:"quantity"`
	FobEUR                float64 `json:"fob_eur"`
	FreightEUR            float64 `json:"freight_eur"`
	ExchangeRate          float64 `json:"exchange_rate"`
	FobBHD                float64 `json:"fob_bhd"`
	FreightBHD            float64 `json:"freight_bhd"`
	CnfBHD                float64 `json:"cnf_bhd"`
	Insurance             float64 `json:"insurance"`
	Customs               float64 `json:"customs"`
	LandedCost            float64 `json:"landed_cost"`
	Handling              float64 `json:"handling"`
	FinanceCharges        float64 `json:"finance_charges"`
	OtherCosts            float64 `json:"other_costs"`
	TotalCost             float64 `json:"total_cost"`
	MarkupPercent         float64 `json:"markup_percent"`
	SellingPriceBHD       float64 `json:"selling_price_bhd"`
	SuggestedPriceBHD     float64 `json:"suggested_price_bhd"`
	TotalSuggestedBHD     float64 `json:"total_suggested_bhd"`
}

// ExcelCostingTotals contains the summary totals
type ExcelCostingTotals struct {
	Subtotal   float64 `json:"subtotal"`
	VatPercent float64 `json:"vat_percent"`
	VatAmount  float64 `json:"vat_amount"`
	GrandTotal float64 `json:"grand_total"`
}

// ExcelImportResult represents the result of importing a single file
type ExcelImportResult struct {
	FilePath    string `json:"file_path"`
	FileName    string `json:"file_name"`
	Success     bool   `json:"success"`
	Message     string `json:"message"`
	ItemCount   int    `json:"item_count"`
	GrandTotal  float64 `json:"grand_total"`
	Customer    string `json:"customer"`
	OfferNumber string `json:"offer_number"`
}

// ExcelBatchImportResult represents the result of batch importing
type ExcelBatchImportResult struct {
	TotalFiles     int                   `json:"total_files"`
	Successful     int                   `json:"successful"`
	Failed         int                   `json:"failed"`
	Results        []ExcelImportResult   `json:"results"`
	TotalLineItems int                   `json:"total_line_items"`
	TotalValue     float64               `json:"total_value"`
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

// isPlaceholder checks if a cell value is a template placeholder like "[Product 1]"
func isPlaceholder(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
}

// isEmpty checks if a string is empty or whitespace
func isEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// parseFloat safely parses a float from a cell value
func parseExcelFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" || isPlaceholder(s) {
		return 0
	}
	// Remove currency symbols and thousands separators
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.ReplaceAll(s, "EUR", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.TrimSpace(s)

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return val
}

// extractOfferNumber extracts offer number from folder name like "101 ENGIE AIT"
func extractOfferNumber(folderName string) string {
	re := regexp.MustCompile(`^(\d+)`)
	matches := re.FindStringSubmatch(folderName)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractCustomerFromFolder extracts customer name from folder name
func extractCustomerFromFolder(folderName string) string {
	re := regexp.MustCompile(`^\d+[-\s]*(.+)$`)
	matches := re.FindStringSubmatch(folderName)
	if len(matches) > 1 {
		// Clean up the customer name
		name := strings.TrimSpace(matches[1])
		// Remove common suffixes like "AIT", "FIT", "LIT", "SP"
		suffixes := []string{"-AIT", " AIT", "-FIT", " FIT", "-LIT", " LIT", "-SP", " SP", "-FEED", " FEED"}
		for _, suffix := range suffixes {
			if strings.HasSuffix(strings.ToUpper(name), strings.ToUpper(suffix)) {
				name = strings.TrimSuffix(name, suffix)
				name = strings.TrimSuffix(name, strings.ToLower(suffix))
				break
			}
		}
		return strings.TrimSpace(name)
	}
	return folderName
}

// ============================================================================
// MAIN PARSING FUNCTIONS
// ============================================================================

// ParseCostingSheet parses an Excel costing sheet and extracts all data
func ParseCostingSheet(filePath string) (*ExcelCostingData, error) {
	log.Printf("📊 Parsing costing sheet: %s", filePath)

	// Open the Excel file
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}
	defer f.Close()

	result := &ExcelCostingData{
		FilePath: filePath,
		FileName: filepath.Base(filePath),
		ParsedAt: time.Now(),
		Warnings: []string{},
	}

	// Find the costing sheet
	sheetName := SHEET_COSTING
	sheets := f.GetSheetList()
	found := false
	for _, s := range sheets {
		if strings.EqualFold(s, SHEET_COSTING) || strings.Contains(strings.ToLower(s), "costing") {
			sheetName = s
			found = true
			break
		}
	}
	if !found && len(sheets) > 1 {
		// Try second sheet (common pattern)
		sheetName = sheets[1]
		result.Warnings = append(result.Warnings, fmt.Sprintf("Costing sheet not found by name, using sheet: %s", sheetName))
	}

	log.Printf("📊 Using sheet: %s", sheetName)

	// Extract metadata
	result.Metadata = extractMetadata(f, sheetName)

	// Extract line items (HORIZONTAL - iterate columns!)
	result.LineItems = extractLineItems(f, sheetName)

	// Extract totals
	result.Totals = extractTotals(f, sheetName, result.LineItems)

	log.Printf("✅ Parsed %d line items, Grand Total: %.3f BHD", len(result.LineItems), result.Totals.GrandTotal)

	return result, nil
}

// extractMetadata extracts header/metadata from the costing sheet
func extractMetadata(f *excelize.File, sheetName string) ExcelCostingMetadata {
	meta := ExcelCostingMetadata{}

	// Row 2: Date (B2), Folder Number (D2), Est Delivery (F2)
	meta.Date, _ = f.GetCellValue(sheetName, "B2")
	meta.FolderNumber, _ = f.GetCellValue(sheetName, "D2")
	meta.EstDelivery, _ = f.GetCellValue(sheetName, "F2")

	// Row 3: Prepared By (B3), Costing ID (D3), Delivery Terms (F3)
	meta.PreparedBy, _ = f.GetCellValue(sheetName, "B3")
	meta.CostingID, _ = f.GetCellValue(sheetName, "D3")
	meta.DeliveryTerms, _ = f.GetCellValue(sheetName, "F3")

	// Row 4: Customer (B4), Contact Person (D4), Order Type (F4)
	meta.Customer, _ = f.GetCellValue(sheetName, "B4")
	meta.ContactPerson, _ = f.GetCellValue(sheetName, "D4")
	meta.OrderType, _ = f.GetCellValue(sheetName, "F4")

	// Row 5: Reference (B5), Payment Terms (D5), Country (F5)
	meta.Reference, _ = f.GetCellValue(sheetName, "A5") // Reference is often in A5
	if isEmpty(meta.Reference) {
		meta.Reference, _ = f.GetCellValue(sheetName, "B5")
	}
	meta.PaymentTerms, _ = f.GetCellValue(sheetName, "D5")
	meta.CountryOfOrigin, _ = f.GetCellValue(sheetName, "F5")

	return meta
}

// extractLineItems extracts product line items - HORIZONTAL iteration!
func extractLineItems(f *excelize.File, sheetName string) []ExcelCostingLineItem {
	items := []ExcelCostingLineItem{}

	// Get shared quantity from B11 (applies to all products)
	qtyStr, _ := f.GetCellValue(sheetName, "B11")
	sharedQuantity := parseExcelFloat(qtyStr)
	if sharedQuantity == 0 {
		sharedQuantity = 1 // Default to 1 if not specified
	}

	productNum := 0

	// Iterate COLUMNS from C (col 3) onwards
	for col := COL_FIRST_PRODUCT; col <= COL_MAX_PRODUCTS; col++ {
		// Get equipment name (Row 8) - this is the primary indicator of a valid product
		cellRef, _ := excelize.CoordinatesToCellName(col, ROW_EQUIPMENT)
		equipment, _ := f.GetCellValue(sheetName, cellRef)

		// Skip if empty or placeholder
		if isEmpty(equipment) || isPlaceholder(equipment) {
			continue
		}

		productNum++
		item := ExcelCostingLineItem{
			ProductNumber: productNum,
			ColumnIndex:   col,
		}

		// Extract all fields for this product column
		item.Equipment = equipment

		// Supplier (Row 7)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_SUPPLIER)
		item.Supplier, _ = f.GetCellValue(sheetName, cellRef)

		// Model (Row 9)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_MODEL)
		item.Model, _ = f.GetCellValue(sheetName, cellRef)

		// Specification (Row 10)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_SPECIFICATION)
		item.Specification, _ = f.GetCellValue(sheetName, cellRef)

		// Use shared quantity
		item.Quantity = sharedQuantity

		// FOB EUR (Row 12)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_FOB_EUR)
		val, _ := f.GetCellValue(sheetName, cellRef)
		item.FobEUR = parseExcelFloat(val)

		// Freight EUR (Row 13)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_FREIGHT_EUR)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.FreightEUR = parseExcelFloat(val)

		// Exchange Rate (Row 14)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_EXCHANGE_RATE)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.ExchangeRate = parseExcelFloat(val)

		// FOB BHD (Row 15)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_FOB_BHD)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.FobBHD = parseExcelFloat(val)

		// Freight BHD (Row 16)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_FREIGHT_BHD)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.FreightBHD = parseExcelFloat(val)

		// CNF BHD (Row 17)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_CNF)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.CnfBHD = parseExcelFloat(val)

		// Insurance (Row 18)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_INSURANCE)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.Insurance = parseExcelFloat(val)

		// Customs (Row 19)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_CUSTOMS)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.Customs = parseExcelFloat(val)

		// Landed Cost (Row 20)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_LANDED_COST)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.LandedCost = parseExcelFloat(val)

		// Handling (Row 21)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_HANDLING)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.Handling = parseExcelFloat(val)

		// Finance Charges (Row 22)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_FINANCE_CHARGES)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.FinanceCharges = parseExcelFloat(val)

		// Other Costs (Row 23)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_OTHER_COSTS)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.OtherCosts = parseExcelFloat(val)

		// Total Cost (Row 24)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_TOTAL_COST)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.TotalCost = parseExcelFloat(val)

		// Markup % (Row 25)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_MARKUP_PERCENT)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.MarkupPercent = parseExcelFloat(val)

		// Selling Price BHD (Row 26)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_SELLING_PRICE)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.SellingPriceBHD = parseExcelFloat(val)

		// Suggested Price BHD (Row 27)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_SUGGESTED_PRICE)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.SuggestedPriceBHD = parseExcelFloat(val)

		// Total Suggested Price (Row 28)
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_TOTAL_SUGGESTED)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.TotalSuggestedBHD = parseExcelFloat(val)

		items = append(items, item)
	}

	return items
}

// extractTotals extracts the totals from the sheet
func extractTotals(f *excelize.File, sheetName string, items []ExcelCostingLineItem) ExcelCostingTotals {
	totals := ExcelCostingTotals{
		VatPercent: 10.0, // Bahrain VAT default
	}

	// Try to get totals from column M (standard location)
	// Subtotal usually in M28
	val, _ := f.GetCellValue(sheetName, "M28")
	totals.Subtotal = parseExcelFloat(val)

	// VAT in M29
	val, _ = f.GetCellValue(sheetName, "M29")
	totals.VatAmount = parseExcelFloat(val)

	// Grand Total in M30
	val, _ = f.GetCellValue(sheetName, "M30")
	totals.GrandTotal = parseExcelFloat(val)

	// If totals not found, calculate from line items
	if totals.Subtotal == 0 && len(items) > 0 {
		for _, item := range items {
			totals.Subtotal += item.TotalSuggestedBHD
		}
		totals.VatAmount = totals.Subtotal * (totals.VatPercent / 100)
		totals.GrandTotal = totals.Subtotal + totals.VatAmount
	}

	return totals
}

// ============================================================================
// WAILS BINDINGS
// ============================================================================

// ParseCostingSheetFile parses a single costing sheet file (Wails binding)
func (a *App) ParseCostingSheetFile(filePath string) (*ExcelCostingData, error) {
	return ParseCostingSheet(filePath)
}

// ParseCostingSheetWithDialog opens file picker and parses selected file
func (a *App) ParseCostingSheetWithDialog() (*ExcelCostingData, error) {
	// Use Wails runtime to open file dialog
	filePath, err := a.PickExcelFile()
	if err != nil {
		return nil, err
	}
	if filePath == "" {
		return nil, fmt.Errorf("no file selected")
	}
	return ParseCostingSheet(filePath)
}

// PickExcelFile opens a file dialog for Excel files
func (a *App) PickExcelFile() (string, error) {
	return wailsRuntime.OpenFileDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title: "Select Costing Sheet",
		Filters: []wailsRuntime.FileFilter{
			{DisplayName: "Excel Files", Pattern: "*.xlsx;*.xls"},
		},
	})
}

// ScanOfferFolders scans the OneDrive folder for all offer data
func (a *App) ScanOfferFolders(basePath string) (*ExcelBatchImportResult, error) {
	log.Printf("📂 Scanning offer folders: %s", basePath)

	result := &ExcelBatchImportResult{
		Results: []ExcelImportResult{},
	}

	// Walk the directory tree
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors, continue walking
		}

		// Look for costing sheet files
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".xlsx") {
			// Check if it's a costing file
			name := strings.ToLower(info.Name())
			if strings.Contains(name, "costing") || strings.Contains(name, "masterfile") {
				result.TotalFiles++

				// Parse the file
				data, parseErr := ParseCostingSheet(path)

				importResult := ExcelImportResult{
					FilePath: path,
					FileName: info.Name(),
				}

				// Extract offer number from parent folder
				parentFolder := filepath.Base(filepath.Dir(path))
				importResult.OfferNumber = extractOfferNumber(parentFolder)

				if parseErr != nil {
					importResult.Success = false
					importResult.Message = parseErr.Error()
					result.Failed++
				} else {
					importResult.Success = true
					importResult.Message = fmt.Sprintf("Parsed %d items", len(data.LineItems))
					importResult.ItemCount = len(data.LineItems)
					importResult.GrandTotal = data.Totals.GrandTotal
					importResult.Customer = data.Metadata.Customer
					if importResult.Customer == "" {
						importResult.Customer = extractCustomerFromFolder(parentFolder)
					}

					result.Successful++
					result.TotalLineItems += len(data.LineItems)
					result.TotalValue += data.Totals.GrandTotal
				}

				result.Results = append(result.Results, importResult)
			}
		}

		return nil
	})

	if err != nil {
		return result, err
	}

	log.Printf("✅ Scan complete: %d files, %d successful, %d line items, %.3f BHD total",
		result.TotalFiles, result.Successful, result.TotalLineItems, result.TotalValue)

	return result, nil
}

// ImportCostingToOpportunity imports a parsed costing sheet as an opportunity
func (a *App) ImportCostingToOpportunity(data *ExcelCostingData) error {
	if data == nil || len(data.LineItems) == 0 {
		return fmt.Errorf("no data to import")
	}

	// Extract offer number from file path
	parentFolder := filepath.Base(filepath.Dir(data.FilePath))
	offerNumber := extractOfferNumber(parentFolder)

	// Look up or create customer
	customer := data.Metadata.Customer
	if customer == "" {
		customer = extractCustomerFromFolder(parentFolder)
	}

	// Check if RFQ already exists with this number
	rfqNumber := fmt.Sprintf("%s-25", offerNumber) // e.g., "101-25"
	var existingRFQ RFQData
	if err := a.db.Where("rfq_number = ?", rfqNumber).First(&existingRFQ).Error; err == nil {
		log.Printf("📊 RFQ %s exists, skipping creation", rfqNumber)
	} else {
		// Create RFQ data
		rfqData := RFQData{
			RFQNumber: rfqNumber,
			Client:    customer,
			Project:   data.Metadata.Reference,
			Status:    "Proposal",
			Stage:     "Costing",
			Value:     data.Totals.GrandTotal,
			Notes:     fmt.Sprintf("Imported from: %s", data.FileName),
		}

		if err := a.db.Create(&rfqData).Error; err != nil {
			log.Printf("⚠️ RFQ creation failed: %v", err)
		}
	}

	// Create costing sheet record
	costingNumber := fmt.Sprintf("CS-%s", offerNumber)
	var existingCS DBCostingSheet
	if err := a.db.Where("costing_number = ?", costingNumber).First(&existingCS).Error; err == nil {
		log.Printf("📊 Costing sheet %s exists, skipping", costingNumber)
		return nil // Already imported
	}

	costingSheet := DBCostingSheet{
		CostingNumber: costingNumber,
		CustomerName:  customer,
		SubtotalBHD:   data.Totals.Subtotal,
		GrandTotalBHD: data.Totals.GrandTotal,
		Status:        "Draft",
	}

	if err := a.db.Create(&costingSheet).Error; err != nil {
		return fmt.Errorf("failed to create costing sheet: %w", err)
	}

	// Create costing items
	for i, item := range data.LineItems {
		dbItem := DBCostingItem{
			CostingSheetID: costingSheet.ID,
			LineNumber:     i + 1,
			ProductType:    item.Supplier,
			Description:    fmt.Sprintf("%s - %s (%s)", item.Equipment, item.Model, item.Specification),
			Quantity:       item.Quantity,
			UnitCostBHD:    item.TotalCost,
			MarginPercent:  item.MarkupPercent,
			UnitPriceBHD:   item.SellingPriceBHD,
			LineTotalBHD:   item.TotalSuggestedBHD,
		}

		if err := a.db.Create(&dbItem).Error; err != nil {
			log.Printf("⚠️ Failed to create costing item: %v", err)
		}
	}

	log.Printf("✅ Imported costing sheet: %s with %d items", costingNumber, len(data.LineItems))
	return nil
}

// calculateMargin calculates gross margin from costing data
func calculateMargin(data *ExcelCostingData) float64 {
	if data.Totals.Subtotal == 0 {
		return 0
	}

	totalCost := 0.0
	for _, item := range data.LineItems {
		totalCost += item.TotalCost * item.Quantity
	}

	if totalCost == 0 {
		return 0
	}

	return ((data.Totals.Subtotal - totalCost) / data.Totals.Subtotal) * 100
}

// BatchImportCostingSheets imports all costing sheets from a folder
func (a *App) BatchImportCostingSheets(basePath string) (*ExcelBatchImportResult, error) {
	// First scan to get all files
	scanResult, err := a.ScanOfferFolders(basePath)
	if err != nil {
		return nil, err
	}

	// Import each successful parse
	for i, res := range scanResult.Results {
		if res.Success {
			data, parseErr := ParseCostingSheet(res.FilePath)
			if parseErr == nil {
				importErr := a.ImportCostingToOpportunity(data)
				if importErr != nil {
					scanResult.Results[i].Message = fmt.Sprintf("Parse OK, import failed: %v", importErr)
				} else {
					scanResult.Results[i].Message = "Imported successfully"
				}
			}
		}
	}

	return scanResult, nil
}
