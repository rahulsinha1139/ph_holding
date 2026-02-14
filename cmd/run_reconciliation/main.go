package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ═══════════════════════════════════════════════════════════════════════════
// DATA RECONCILIATION
// ═══════════════════════════════════════════════════════════════════════════
// Matches extracted documents with:
//   1. Costing Sheet Data (337 sheets, 5.8M BHD)
//   2. Database Invoices (468 records)
//   3. Database Orders (155 records)
//   4. Customer Master (336 customers)
//
// Creates linkages and identifies gaps/discrepancies
// ═══════════════════════════════════════════════════════════════════════════

type ReconciliationReport struct {
	GeneratedAt         time.Time              `json:"generated_at"`

	// Summary
	TotalExtractedDocs  int                    `json:"total_extracted_docs"`
	TotalCostingSheets  int                    `json:"total_costing_sheets"`
	TotalDBInvoices     int                    `json:"total_db_invoices"`
	TotalDBOrders       int                    `json:"total_db_orders"`

	// Matches
	DocsMatchedToCosting int                   `json:"docs_matched_to_costing"`
	CostingWithDocs      int                   `json:"costing_with_docs"`
	InvoicesWithCosting  int                   `json:"invoices_with_costing"`

	// Coverage
	OfferCoverage       map[string]OfferStatus `json:"offer_coverage"`
	CustomerMatches     map[string]int         `json:"customer_matches"`

	// Gaps
	CostingNoInvoice    []string               `json:"costing_without_invoice"`
	InvoiceNoCosting    []string               `json:"invoice_without_costing"`

	// Value Analysis
	TotalCostingValue   float64                `json:"total_costing_value_bhd"`
	TotalInvoiceValue   float64                `json:"total_invoice_value_bhd"`
	ValueDifference     float64                `json:"value_difference_bhd"`
}

type OfferStatus struct {
	OfferNumber    string  `json:"offer_number"`
	CustomerName   string  `json:"customer_name"`
	HasRFQ         bool    `json:"has_rfq"`
	HasCosting     bool    `json:"has_costing"`
	HasInvoice     bool    `json:"has_invoice"`
	HasDN          bool    `json:"has_delivery_note"`
	HasPO          bool    `json:"has_customer_po"`
	CostingValue   float64 `json:"costing_value_bhd"`
	Status         string  `json:"status"` // complete, partial, pending
}

// Database models (read-only for reconciliation)
type ExtractedDocument struct {
	ID             uint    `gorm:"primaryKey"`
	SourcePath     string  `json:"source_path"`
	DocumentType   string  `json:"document_type"`
	OfferNumber    string  `json:"offer_number"`
	CustomerName   string  `json:"customer_name"`
	DocumentNumber string  `json:"document_number"`
	Stage          string  `json:"stage"`
}

type CostingSheetData struct {
	ID            uint    `gorm:"primaryKey"`
	OfferNumber   string  `json:"offer_number"`
	CustomerName  string  `json:"customer_name"`
	TotalValueBHD float64 `json:"total_value_bhd"`
}

type Invoice struct {
	ID           string  `gorm:"primaryKey"`
	CustomerID   string  `json:"customer_id"`
	InvoiceNumber string `json:"invoice_number"`
	TotalAmount  float64 `json:"total_amount"`
	Reference    string  `json:"reference"`
}

type Customer struct {
	ID           string `gorm:"primaryKey"`
	BusinessName string `json:"business_name"`
}

func main() {
	dbPath := "ph_holdings.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("🔄 DATA RECONCILIATION")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	report := &ReconciliationReport{
		GeneratedAt:      time.Now(),
		OfferCoverage:    make(map[string]OfferStatus),
		CustomerMatches:  make(map[string]int),
		CostingNoInvoice: []string{},
		InvoiceNoCosting: []string{},
	}

	// 1. Count totals
	var count64 int64
	db.Table("extracted_documents").Count(&count64)
	report.TotalExtractedDocs = int(count64)
	db.Table("costing_sheet_data").Count(&count64)
	report.TotalCostingSheets = int(count64)
	db.Table("invoices").Count(&count64)
	report.TotalDBInvoices = int(count64)
	db.Table("orders").Count(&count64)
	report.TotalDBOrders = int(count64)

	fmt.Printf("\n📊 DATABASE COUNTS:\n")
	fmt.Printf("  Extracted Documents:  %d\n", report.TotalExtractedDocs)
	fmt.Printf("  Costing Sheets:       %d\n", report.TotalCostingSheets)
	fmt.Printf("  Database Invoices:    %d\n", report.TotalDBInvoices)
	fmt.Printf("  Database Orders:      %d\n", report.TotalDBOrders)

	// 2. Get total costing value
	db.Table("costing_sheet_data").Select("COALESCE(SUM(total_value_bhd), 0)").Scan(&report.TotalCostingValue)

	// Get total invoice value (column is grand_total_bhd, not total_amount)
	db.Table("invoices").Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&report.TotalInvoiceValue)

	report.ValueDifference = report.TotalCostingValue - report.TotalInvoiceValue

	fmt.Printf("\n💰 VALUE SUMMARY:\n")
	fmt.Printf("  Costing Pipeline:     %.0f BHD (~$%.0f USD)\n", report.TotalCostingValue, report.TotalCostingValue*2.65)
	fmt.Printf("  Invoiced Revenue:     %.0f BHD (~$%.0f USD)\n", report.TotalInvoiceValue, report.TotalInvoiceValue*2.65)
	fmt.Printf("  Difference:           %.0f BHD\n", report.ValueDifference)

	// 3. Build offer status map from extracted documents
	var docs []ExtractedDocument
	db.Find(&docs)

	for _, doc := range docs {
		if doc.OfferNumber == "" {
			continue
		}

		status, exists := report.OfferCoverage[doc.OfferNumber]
		if !exists {
			status = OfferStatus{
				OfferNumber:  doc.OfferNumber,
				CustomerName: doc.CustomerName,
			}
		}

		// Update customer name if empty
		if status.CustomerName == "" && doc.CustomerName != "" {
			status.CustomerName = doc.CustomerName
		}

		// Set flags based on document type
		switch doc.DocumentType {
		case "RFQ_EMAIL":
			status.HasRFQ = true
		case "INVOICE":
			status.HasInvoice = true
		case "DELIVERY_NOTE":
			status.HasDN = true
		case "CUSTOMER_PO":
			status.HasPO = true
		}

		report.OfferCoverage[doc.OfferNumber] = status
	}

	// 4. Add costing data to offer status
	var costings []CostingSheetData
	db.Find(&costings)

	costingOffers := make(map[string]float64)
	for _, c := range costings {
		if c.OfferNumber == "" {
			continue
		}
		costingOffers[c.OfferNumber] += c.TotalValueBHD

		status := report.OfferCoverage[c.OfferNumber]
		status.HasCosting = true
		status.CostingValue += c.TotalValueBHD
		if status.CustomerName == "" && c.CustomerName != "" {
			status.CustomerName = c.CustomerName
		}
		report.OfferCoverage[c.OfferNumber] = status
	}

	// 5. Calculate matches and status
	completeCount := 0
	partialCount := 0
	pendingCount := 0

	for offerNum, status := range report.OfferCoverage {
		// Determine status
		if status.HasRFQ && status.HasCosting && status.HasInvoice && status.HasDN {
			status.Status = "COMPLETE"
			completeCount++
		} else if status.HasCosting && (status.HasInvoice || status.HasDN) {
			status.Status = "PARTIAL"
			partialCount++
		} else {
			status.Status = "PENDING"
			pendingCount++
		}

		// Check for gaps
		if status.HasCosting && !status.HasInvoice {
			report.CostingNoInvoice = append(report.CostingNoInvoice, offerNum)
		}
		if status.HasInvoice && !status.HasCosting {
			report.InvoiceNoCosting = append(report.InvoiceNoCosting, offerNum)
		}

		report.OfferCoverage[offerNum] = status

		// Count customer matches
		if status.CustomerName != "" {
			report.CustomerMatches[status.CustomerName]++
		}
	}

	// Sort gaps
	sort.Strings(report.CostingNoInvoice)
	sort.Strings(report.InvoiceNoCosting)

	fmt.Printf("\n📋 OFFER STATUS:\n")
	fmt.Printf("  Total offers tracked: %d\n", len(report.OfferCoverage))
	fmt.Printf("  Complete lifecycle:   %d\n", completeCount)
	fmt.Printf("  Partial progress:     %d\n", partialCount)
	fmt.Printf("  Pending:              %d\n", pendingCount)

	fmt.Printf("\n⚠️  GAPS IDENTIFIED:\n")
	fmt.Printf("  Costing without Invoice: %d offers\n", len(report.CostingNoInvoice))
	fmt.Printf("  Invoice without Costing: %d offers\n", len(report.InvoiceNoCosting))

	// 6. Top customers by offer count
	fmt.Printf("\n🏢 TOP CUSTOMERS (by offer count):\n")
	type custCount struct {
		name  string
		count int
	}
	var custList []custCount
	for name, count := range report.CustomerMatches {
		if name != "" {
			custList = append(custList, custCount{name, count})
		}
	}
	sort.Slice(custList, func(i, j int) bool {
		return custList[i].count > custList[j].count
	})
	for i, c := range custList {
		if i >= 15 {
			break
		}
		fmt.Printf("  %-30s: %3d offers\n", truncate(c.name, 30), c.count)
	}

	// 7. Show complete lifecycle offers
	fmt.Printf("\n✅ COMPLETE LIFECYCLE OFFERS:\n")
	var completeOffers []string
	for offerNum, status := range report.OfferCoverage {
		if status.Status == "COMPLETE" {
			completeOffers = append(completeOffers, offerNum)
		}
	}
	sort.Strings(completeOffers)
	if len(completeOffers) > 0 {
		fmt.Printf("  %v\n", completeOffers)
	} else {
		fmt.Println("  None found with full RFQ→Costing→Invoice→DN")
	}

	// 8. Value by status
	var completeValue, partialValue, pendingValue float64
	for _, status := range report.OfferCoverage {
		switch status.Status {
		case "COMPLETE":
			completeValue += status.CostingValue
		case "PARTIAL":
			partialValue += status.CostingValue
		case "PENDING":
			pendingValue += status.CostingValue
		}
	}

	fmt.Printf("\n💰 VALUE BY STATUS:\n")
	fmt.Printf("  Complete:  %.0f BHD (%.1f%%)\n", completeValue, pct(completeValue, report.TotalCostingValue))
	fmt.Printf("  Partial:   %.0f BHD (%.1f%%)\n", partialValue, pct(partialValue, report.TotalCostingValue))
	fmt.Printf("  Pending:   %.0f BHD (%.1f%%)\n", pendingValue, pct(pendingValue, report.TotalCostingValue))

	// 9. Save report
	outputPath := "reconciliation_report.json"
	jsonData, _ := json.MarshalIndent(report, "", "  ")
	if err := os.WriteFile(outputPath, jsonData, 0644); err != nil {
		log.Printf("Error saving report: %v", err)
	} else {
		fmt.Printf("\n💾 Report saved to: %s\n", outputPath)
	}

	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("✅ RECONCILIATION COMPLETE!")
	fmt.Println("═══════════════════════════════════════════════════════════════")
}

func pct(part, total float64) float64 {
	if total == 0 {
		return 0
	}
	return part / total * 100
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

func normalizeCustomer(name string) string {
	name = strings.ToUpper(name)
	// Map variations to canonical names
	mappings := map[string]string{
		"ALBA":       "ALBA",
		"BAPCO":      "BAPCO",
		"ENGIE":      "ENGIE",
		"EWA":        "EWA",
		"VEOLIA":     "VEOLIA",
		"SULB":       "SULB",
		"TTSJV":      "TTSJV",
		"SEAPEAK":    "SEAPEAK",
		"NOMAC":      "NOMAC",
		"TABREED":    "TABREED",
		"YOKOGAWA":   "YOKOGAWA",
		"ARLA":       "ARLA",
		"DELTA":      "DELTA",
	}

	for key, val := range mappings {
		if strings.Contains(name, key) {
			return val
		}
	}
	return name
}
