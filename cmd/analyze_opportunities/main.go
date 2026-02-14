package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
// OPPORTUNITY ANALYSIS & CORROBORATION
// ═══════════════════════════════════════════════════════════════════════════
// Reads "opportunities created 2025.xlsx" and corroborates with:
//   1. Costing Sheet Data (337 sheets)
//   2. Extracted Documents (359 records)
//   3. Database Invoices (468 records)
//
// Determines accurate status: WON, LOST, IN_PIPELINE
// ═══════════════════════════════════════════════════════════════════════════

type OpportunityRow struct {
	RowNum         int       `json:"row_num"`
	OfferNumber    string    `json:"offer_number"`
	Customer       string    `json:"customer"`
	RFQDate        string    `json:"rfq_date"`
	EHOrderNumber  string    `json:"eh_order_number"`
	PHInvoice      string    `json:"ph_invoice"`
	InvoiceDate    string    `json:"invoice_date"`
	ValueBHD       float64   `json:"value_bhd"`
	Status         string    `json:"status"`
	HasCosting     bool      `json:"has_costing"`
	HasExtractedDoc bool     `json:"has_extracted_doc"`
	HasDBInvoice   bool      `json:"has_db_invoice"`
	CostingValue   float64   `json:"costing_value_bhd"`
}

type AnalysisResult struct {
	GeneratedAt      time.Time         `json:"generated_at"`
	TotalRows        int               `json:"total_rows"`
	WonCount         int               `json:"won_count"`
	LostCount        int               `json:"lost_count"`
	PipelineCount    int               `json:"pipeline_count"`
	TotalValueBHD    float64           `json:"total_value_bhd"`
	WonValueBHD      float64           `json:"won_value_bhd"`
	PipelineValueBHD float64           `json:"pipeline_value_bhd"`
	Opportunities    []OpportunityRow  `json:"opportunities"`
	ByCustomer       map[string]int    `json:"by_customer"`
	ByStatus         map[string]int    `json:"by_status"`
}

func main() {
	excelPath := "Data_for_database/opportunities created 2025.xlsx"
	dbPath := "ph_holdings.db"

	if len(os.Args) > 1 {
		excelPath = os.Args[1]
	}
	if len(os.Args) > 2 {
		dbPath = os.Args[2]
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("📊 OPPORTUNITY ANALYSIS & CORROBORATION")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("📄 Source: %s\n", excelPath)

	// Open Excel file
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		log.Fatalf("Failed to open Excel: %v", err)
	}
	defer f.Close()

	// Get sheet names
	sheets := f.GetSheetList()
	fmt.Printf("📑 Sheets: %v\n", sheets)

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Load costing data for lookup
	costingOffers := loadCostingOffers(db)
	fmt.Printf("📦 Loaded %d costing offer records\n", len(costingOffers))

	// Load extracted documents for lookup
	extractedOffers := loadExtractedOffers(db)
	fmt.Printf("📁 Loaded %d extracted offer records\n", len(extractedOffers))

	// Load database invoices
	dbInvoices := loadDBInvoices(db)
	fmt.Printf("🧾 Loaded %d database invoice records\n", len(dbInvoices))

	result := &AnalysisResult{
		GeneratedAt:   time.Now(),
		Opportunities: []OpportunityRow{},
		ByCustomer:    make(map[string]int),
		ByStatus:      make(map[string]int),
	}

	// Read first sheet
	sheetName := sheets[0]
	fmt.Printf("\n📋 Analyzing sheet: %s\n", sheetName)

	// First, let's understand the structure by reading headers
	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Fatalf("Failed to get rows: %v", err)
	}

	fmt.Printf("📊 Total rows: %d\n\n", len(rows))

	// Print first few rows to understand structure
	fmt.Println("📝 HEADER ANALYSIS:")
	for i := 0; i < min(5, len(rows)); i++ {
		row := rows[i]
		fmt.Printf("Row %d: ", i+1)
		for j := 0; j < min(10, len(row)); j++ {
			fmt.Printf("[%s] ", truncate(row[j], 15))
		}
		fmt.Println()
	}

	// Detect column indices from headers (first row is the header)
	headerRow := 0
	colOffer := -1
	colCustomer := -1
	colRFQDate := -1
	colEHOrder := -1
	colPHInvoice := -1
	colInvDate := -1
	colValue := -1
	colStatus := -1

	// Only look at first row for headers
	if len(rows) > 0 {
		row := rows[0]
		for j, cell := range row {
			cellLower := strings.ToLower(strings.TrimSpace(cell))
			// Offer/Opportunity number - be specific to avoid SFDC column
			if cellLower == "opp. no." || cellLower == "opp no" || cellLower == "offer no." ||
				(strings.Contains(cellLower, "offer") && strings.Contains(cellLower, "no") && !strings.Contains(cellLower, "sfdc")) {
				colOffer = j
			}
			// Customer/End User
			if strings.Contains(cellLower, "end user") || strings.Contains(cellLower, "customer") || strings.Contains(cellLower, "client") {
				colCustomer = j
			}
			// Folder Name (backup for customer)
			if colCustomer == -1 && strings.Contains(cellLower, "folder name") {
				colCustomer = j
			}
			if strings.Contains(cellLower, "rfq") && strings.Contains(cellLower, "date") {
				colRFQDate = j
			}
			if strings.Contains(cellLower, "e+h") || strings.Contains(cellLower, "eh order") || strings.Contains(cellLower, "e+h ref") {
				colEHOrder = j
			}
			if strings.Contains(cellLower, "ph inv") || strings.Contains(cellLower, "invoice no") {
				colPHInvoice = j
			}
			if strings.Contains(cellLower, "inv") && strings.Contains(cellLower, "date") {
				colInvDate = j
			}
			if strings.Contains(cellLower, "value") || strings.Contains(cellLower, "bhd") {
				colValue = j
			}
			if cellLower == "status" || strings.HasPrefix(cellLower, "status") {
				colStatus = j
			}
		}
	}

	fmt.Printf("\n🔍 COLUMN DETECTION:\n")
	fmt.Printf("  Header row: %d\n", headerRow+1)
	fmt.Printf("  Offer No: col %d\n", colOffer)
	fmt.Printf("  Customer: col %d\n", colCustomer)
	fmt.Printf("  RFQ Date: col %d\n", colRFQDate)
	fmt.Printf("  E+H Order: col %d\n", colEHOrder)
	fmt.Printf("  PH Invoice: col %d\n", colPHInvoice)
	fmt.Printf("  Inv Date: col %d\n", colInvDate)
	fmt.Printf("  Value: col %d\n", colValue)
	fmt.Printf("  Status: col %d\n", colStatus)

	// Parse data rows
	offerNumPat := regexp.MustCompile(`(\d+)[-/]?(\d+)?`)

	for i := headerRow + 1; i < len(rows); i++ {
		row := rows[i]
		if len(row) == 0 {
			continue
		}

		opp := OpportunityRow{RowNum: i + 1}

		// Extract offer number
		if colOffer >= 0 && colOffer < len(row) {
			opp.OfferNumber = strings.TrimSpace(row[colOffer])
			// Normalize offer number (extract just the number)
			if match := offerNumPat.FindStringSubmatch(opp.OfferNumber); len(match) > 1 {
				opp.OfferNumber = match[1]
			}
		}

		if opp.OfferNumber == "" {
			continue
		}

		// Extract other fields
		if colCustomer >= 0 && colCustomer < len(row) {
			opp.Customer = strings.TrimSpace(row[colCustomer])
		}
		if colRFQDate >= 0 && colRFQDate < len(row) {
			opp.RFQDate = strings.TrimSpace(row[colRFQDate])
		}
		if colEHOrder >= 0 && colEHOrder < len(row) {
			opp.EHOrderNumber = strings.TrimSpace(row[colEHOrder])
		}
		if colPHInvoice >= 0 && colPHInvoice < len(row) {
			opp.PHInvoice = strings.TrimSpace(row[colPHInvoice])
		}
		if colInvDate >= 0 && colInvDate < len(row) {
			opp.InvoiceDate = strings.TrimSpace(row[colInvDate])
		}
		if colValue >= 0 && colValue < len(row) {
			opp.ValueBHD = parseAmount(row[colValue])
		}
		if colStatus >= 0 && colStatus < len(row) {
			opp.Status = strings.TrimSpace(row[colStatus])
		}

		// Corroborate with database
		if val, ok := costingOffers[opp.OfferNumber]; ok {
			opp.HasCosting = true
			opp.CostingValue = val
		}
		if _, ok := extractedOffers[opp.OfferNumber]; ok {
			opp.HasExtractedDoc = true
		}
		if _, ok := dbInvoices[opp.PHInvoice]; ok {
			opp.HasDBInvoice = true
		}

		// Determine status if not set
		if opp.Status == "" {
			if opp.PHInvoice != "" && opp.PHInvoice != "-" && !strings.HasPrefix(strings.ToLower(opp.PHInvoice), "n/a") {
				opp.Status = "WON"
			} else if opp.EHOrderNumber != "" && opp.EHOrderNumber != "-" {
				opp.Status = "WON"
			} else {
				opp.Status = "IN_PIPELINE"
			}
		}

		// Normalize status - check LOST first since "Closed (lost)" contains both
		statusUpper := strings.ToUpper(opp.Status)
		if strings.Contains(statusUpper, "LOST") || strings.Contains(statusUpper, "CANCEL") || strings.Contains(statusUpper, "NOT QUOTED") {
			opp.Status = "LOST"
		} else if strings.Contains(statusUpper, "WON") || strings.Contains(statusUpper, "INVOICED") ||
			strings.Contains(statusUpper, "PO RECEIVED") || strings.Contains(statusUpper, "PAYMENT") ||
			(strings.Contains(statusUpper, "CLOSED") && !strings.Contains(statusUpper, "LOST")) {
			opp.Status = "WON"
		} else {
			opp.Status = "IN_PIPELINE"
		}

		result.Opportunities = append(result.Opportunities, opp)
		result.TotalRows++
		result.TotalValueBHD += opp.ValueBHD

		// Count by status
		result.ByStatus[opp.Status]++
		switch opp.Status {
		case "WON":
			result.WonCount++
			result.WonValueBHD += opp.ValueBHD
		case "LOST":
			result.LostCount++
		default:
			result.PipelineCount++
			result.PipelineValueBHD += opp.ValueBHD
		}

		// Count by customer
		if opp.Customer != "" {
			result.ByCustomer[opp.Customer]++
		}
	}

	// Print results
	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("📊 OPPORTUNITY ANALYSIS RESULTS")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Total opportunities:     %d\n", result.TotalRows)
	fmt.Printf("  ✅ WON:                %d (%.0f BHD)\n", result.WonCount, result.WonValueBHD)
	fmt.Printf("  ❌ LOST:               %d\n", result.LostCount)
	fmt.Printf("  ⏳ IN PIPELINE:        %d (%.0f BHD)\n", result.PipelineCount, result.PipelineValueBHD)
	fmt.Printf("\nTotal Value:             %.0f BHD (~$%.0f USD)\n", result.TotalValueBHD, result.TotalValueBHD*2.65)

	// Corroboration stats
	costingMatch := 0
	extractedMatch := 0
	dbMatch := 0
	for _, opp := range result.Opportunities {
		if opp.HasCosting {
			costingMatch++
		}
		if opp.HasExtractedDoc {
			extractedMatch++
		}
		if opp.HasDBInvoice {
			dbMatch++
		}
	}

	fmt.Printf("\n🔗 CORROBORATION:\n")
	fmt.Printf("  Matched to Costing Data:    %d/%d (%.1f%%)\n", costingMatch, result.TotalRows, pct(costingMatch, result.TotalRows))
	fmt.Printf("  Matched to Extracted Docs:  %d/%d (%.1f%%)\n", extractedMatch, result.TotalRows, pct(extractedMatch, result.TotalRows))
	fmt.Printf("  Matched to DB Invoices:     %d/%d (%.1f%%)\n", dbMatch, result.TotalRows, pct(dbMatch, result.TotalRows))

	// Top customers
	fmt.Printf("\n🏢 TOP CUSTOMERS:\n")
	printTopN(result.ByCustomer, 15)

	// Sample opportunities
	fmt.Printf("\n📋 SAMPLE OPPORTUNITIES:\n")
	for i, opp := range result.Opportunities {
		if i >= 20 {
			break
		}
		marker := "⏳"
		if opp.Status == "WON" {
			marker = "✅"
		} else if opp.Status == "LOST" {
			marker = "❌"
		}
		fmt.Printf("  %s %s: %s - %.0f BHD (Costing:%v, Doc:%v, DB:%v)\n",
			marker, opp.OfferNumber, truncate(opp.Customer, 20),
			opp.ValueBHD, opp.HasCosting, opp.HasExtractedDoc, opp.HasDBInvoice)
	}

	// Save results
	outputPath := "opportunity_analysis.json"
	jsonData, _ := json.MarshalIndent(result, "", "  ")
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		log.Printf("Error saving: %v", err)
	} else {
		fmt.Printf("\n💾 Results saved to: %s\n", outputPath)
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ ANALYSIS COMPLETE!")
	fmt.Println("═══════════════════════════════════════════════════════════════")
}

func loadCostingOffers(db *gorm.DB) map[string]float64 {
	result := make(map[string]float64)
	rows, err := db.Table("costing_sheet_data").Select("offer_number, SUM(total_value_bhd) as total").Group("offer_number").Rows()
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var offerNum string
		var total float64
		rows.Scan(&offerNum, &total)
		if offerNum != "" {
			result[offerNum] = total
		}
	}
	return result
}

func loadExtractedOffers(db *gorm.DB) map[string]bool {
	result := make(map[string]bool)
	rows, err := db.Table("extracted_documents").Select("DISTINCT offer_number").Rows()
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var offerNum string
		rows.Scan(&offerNum)
		if offerNum != "" {
			result[offerNum] = true
		}
	}
	return result
}

func loadDBInvoices(db *gorm.DB) map[string]bool {
	result := make(map[string]bool)
	rows, err := db.Table("invoices").Select("invoice_number").Where("deleted_at IS NULL").Rows()
	if err != nil {
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var invNum string
		rows.Scan(&invNum)
		if invNum != "" {
			result[invNum] = true
		}
	}
	return result
}

func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.ReplaceAll(s, "BD", "")
	s = strings.TrimSpace(s)
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func printTopN(m map[string]int, n int) {
	type kv struct {
		Key   string
		Value int
	}
	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Value > sorted[i].Value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	for i, item := range sorted {
		if i >= n {
			break
		}
		fmt.Printf("  %-30s: %3d\n", truncate(item.Key, 30), item.Value)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
