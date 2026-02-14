package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

// Row constants - matches excel_costing_parser.go
const (
	ROW_DATE          = 2
	ROW_PREPARED_BY   = 3
	ROW_CUSTOMER      = 4
	ROW_REFERENCE     = 5
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
	COL_FIRST_PRODUCT  = 3
	COL_MAX_PRODUCTS  = 50
	SHEET_COSTING = "Costing Sheet"
)

type ExcelCostingData struct {
	FilePath  string                  `json:"file_path"`
	FileName  string                  `json:"file_name"`
	Metadata  ExcelCostingMetadata    `json:"metadata"`
	LineItems []ExcelCostingLineItem  `json:"line_items"`
	Totals    ExcelCostingTotals      `json:"totals"`
	ParsedAt  time.Time               `json:"parsed_at"`
	Warnings  []string                `json:"warnings"`
}

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

type ExcelCostingLineItem struct {
	ProductNumber         int     `json:"product_number"`
	ColumnIndex           int     `json:"column_index"`
	Supplier              string  `json:"supplier"`
	Equipment             string  `json:"equipment"`
	Model                 string  `json:"model"`
	Specification         string  `json:"specification"`
	Quantity              float64 `json:"quantity"`
	FobEUR                float64 `json:"fob_eur"`
	ExchangeRate          float64 `json:"exchange_rate"`
	TotalCost             float64 `json:"total_cost"`
	MarkupPercent         float64 `json:"markup_percent"`
	SellingPriceBHD       float64 `json:"selling_price_bhd"`
	TotalSuggestedBHD     float64 `json:"total_suggested_bhd"`
}

type ExcelCostingTotals struct {
	Subtotal   float64 `json:"subtotal"`
	VatPercent float64 `json:"vat_percent"`
	VatAmount  float64 `json:"vat_amount"`
	GrandTotal float64 `json:"grand_total"`
}

func isPlaceholder(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]")
}

func isEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

func parseExcelFloat(s string) float64 {
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

func extractOfferNumber(folderName string) string {
	re := regexp.MustCompile(`^(\d+)`)
	matches := re.FindStringSubmatch(folderName)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test_excel_parser <path_to_excel_file>")
		fmt.Println("       test_excel_parser --scan <folder_path>")
		os.Exit(1)
	}

	if os.Args[1] == "--scan" {
		if len(os.Args) < 3 {
			fmt.Println("Usage: test_excel_parser --scan <folder_path>")
			os.Exit(1)
		}
		scanFolder(os.Args[2])
		return
	}

	filePath := os.Args[1]
	data, err := ParseCostingSheet(filePath)
	if err != nil {
		log.Fatalf("Failed to parse: %v", err)
	}

	// Pretty print
	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	fmt.Println(string(jsonBytes))
}

func scanFolder(basePath string) {
	totalFiles := 0
	successful := 0
	totalItems := 0
	totalValue := 0.0

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if !info.IsDir() && strings.HasSuffix(strings.ToLower(info.Name()), ".xlsx") {
			name := strings.ToLower(info.Name())
			if strings.Contains(name, "costing") || strings.Contains(name, "masterfile") {
				totalFiles++
				data, parseErr := ParseCostingSheet(path)
				if parseErr != nil {
					fmt.Printf("❌ %s: %v\n", info.Name(), parseErr)
				} else {
					successful++
					totalItems += len(data.LineItems)
					totalValue += data.Totals.GrandTotal
					fmt.Printf("✅ %s: %d items, %.3f BHD (Customer: %s)\n",
						info.Name(), len(data.LineItems), data.Totals.GrandTotal, data.Metadata.Customer)
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Printf("Walk error: %v", err)
	}

	fmt.Println("\n========================================")
	fmt.Printf("Total files: %d\n", totalFiles)
	fmt.Printf("Successful: %d\n", successful)
	fmt.Printf("Total line items: %d\n", totalItems)
	fmt.Printf("Total value: %.3f BHD\n", totalValue)
}

func ParseCostingSheet(filePath string) (*ExcelCostingData, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open: %w", err)
	}
	defer f.Close()

	result := &ExcelCostingData{
		FilePath: filePath,
		FileName: filepath.Base(filePath),
		ParsedAt: time.Now(),
		Warnings: []string{},
	}

	// Find costing sheet
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
		sheetName = sheets[1]
	}

	// Extract metadata
	result.Metadata.Date, _ = f.GetCellValue(sheetName, "B2")
	result.Metadata.FolderNumber, _ = f.GetCellValue(sheetName, "D2")
	result.Metadata.PreparedBy, _ = f.GetCellValue(sheetName, "B3")
	result.Metadata.Customer, _ = f.GetCellValue(sheetName, "B4")
	result.Metadata.Reference, _ = f.GetCellValue(sheetName, "A5")
	if isEmpty(result.Metadata.Reference) {
		result.Metadata.Reference, _ = f.GetCellValue(sheetName, "B5")
	}
	result.Metadata.PaymentTerms, _ = f.GetCellValue(sheetName, "D5")

	// Get shared quantity
	qtyStr, _ := f.GetCellValue(sheetName, "B11")
	sharedQty := parseExcelFloat(qtyStr)
	if sharedQty == 0 {
		sharedQty = 1
	}

	// Extract line items (HORIZONTAL!)
	productNum := 0
	for col := COL_FIRST_PRODUCT; col <= COL_MAX_PRODUCTS; col++ {
		cellRef, _ := excelize.CoordinatesToCellName(col, ROW_EQUIPMENT)
		equipment, _ := f.GetCellValue(sheetName, cellRef)

		if isEmpty(equipment) || isPlaceholder(equipment) {
			continue
		}

		productNum++
		item := ExcelCostingLineItem{
			ProductNumber: productNum,
			ColumnIndex:   col,
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
		item.FobEUR = parseExcelFloat(val)

		// Exchange Rate
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_EXCHANGE_RATE)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.ExchangeRate = parseExcelFloat(val)

		// Total Cost
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_TOTAL_COST)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.TotalCost = parseExcelFloat(val)

		// Markup
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_MARKUP_PERCENT)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.MarkupPercent = parseExcelFloat(val)

		// Selling Price
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_SELLING_PRICE)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.SellingPriceBHD = parseExcelFloat(val)

		// Total Suggested
		cellRef, _ = excelize.CoordinatesToCellName(col, ROW_TOTAL_SUGGESTED)
		val, _ = f.GetCellValue(sheetName, cellRef)
		item.TotalSuggestedBHD = parseExcelFloat(val)

		result.LineItems = append(result.LineItems, item)
	}

	// Extract totals
	result.Totals.VatPercent = 10.0
	val, _ := f.GetCellValue(sheetName, "M28")
	result.Totals.Subtotal = parseExcelFloat(val)
	val, _ = f.GetCellValue(sheetName, "M29")
	result.Totals.VatAmount = parseExcelFloat(val)
	val, _ = f.GetCellValue(sheetName, "M30")
	result.Totals.GrandTotal = parseExcelFloat(val)

	// Calculate if not found
	if result.Totals.Subtotal == 0 && len(result.LineItems) > 0 {
		for _, item := range result.LineItems {
			result.Totals.Subtotal += item.TotalSuggestedBHD
		}
		result.Totals.VatAmount = result.Totals.Subtotal * 0.10
		result.Totals.GrandTotal = result.Totals.Subtotal + result.Totals.VatAmount
	}

	return result, nil
}
