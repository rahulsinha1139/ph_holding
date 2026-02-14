package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// BATCH DOCUMENT EXTRACTION
// ═══════════════════════════════════════════════════════════════════════════
// Processes all classified documents and extracts structured data:
//   - Invoices (PDF) → ExtractedInvoice
//   - Delivery Notes (PDF) → ExtractedDeliveryNote
//   - Customer POs (PDF) → ExtractedCustomerPO
//   - RFQ Emails (.msg) → ParsedRFQEmail
//
// Usage: batch_extract [base_path]
//   base_path: Path containing documents_*.txt files (default: current dir)
//
// Output: extraction_results_YYYYMMDD_HHMMSS.json
// ═══════════════════════════════════════════════════════════════════════════

// ExtractionSummary holds all extraction results
type ExtractionSummary struct {
	StartTime       time.Time                 `json:"start_time"`
	EndTime         time.Time                 `json:"end_time"`
	Duration        string                    `json:"duration"`
	TotalProcessed  int                       `json:"total_processed"`
	SuccessCount    int                       `json:"success_count"`
	FailCount       int                       `json:"fail_count"`
	Invoices        []ExtractedInvoiceBasic   `json:"invoices"`
	DeliveryNotes   []ExtractedDNBasic        `json:"delivery_notes"`
	CustomerPOs     []ExtractedPOBasic        `json:"customer_pos"`
	TotalValueBHD   float64                   `json:"total_value_bhd"`
	UniqueCustomers []string                  `json:"unique_customers"`
	Errors          []string                  `json:"errors"`
}

// Simplified extraction structures (for file-based extraction without OCR)
type ExtractedInvoiceBasic struct {
	FilePath      string   `json:"file_path"`
	OfferNumber   string   `json:"offer_number"`
	CustomerName  string   `json:"customer_name"`
	InvoiceNumber string   `json:"invoice_number"`
	ProductType   string   `json:"product_type"`
	Stage         string   `json:"stage"`
}

type ExtractedDNBasic struct {
	FilePath      string   `json:"file_path"`
	OfferNumber   string   `json:"offer_number"`
	CustomerName  string   `json:"customer_name"`
	DNNumber      string   `json:"dn_number"`
	ProductType   string   `json:"product_type"`
	Stage         string   `json:"stage"`
}

type ExtractedPOBasic struct {
	FilePath      string   `json:"file_path"`
	OfferNumber   string   `json:"offer_number"`
	CustomerName  string   `json:"customer_name"`
	PONumber      string   `json:"po_number"`
	ProductType   string   `json:"product_type"`
	Stage         string   `json:"stage"`
}

// Regex patterns
var (
	offerPattern   = regexp.MustCompile(`(?i)Offers?\s*No\s*(\d+)\s*[-–]\s*(\d+)`)
	folderNumPat   = regexp.MustCompile(`^(\d+)[-\s]`)
	productTypePat = regexp.MustCompile(`[-\s](AIT|FIT|LIT|SP|TIT|FEED|ETS|TT|VALVE|SPARE|PIT|ORP|BTU|LS|SENSOR)[-\s]?`)
	invoiceNumPat  = regexp.MustCompile(`(?i)(PH\s*\d+-\d+|INV[-\s]?\d+|\d{10}|\d{4}[A-Z]+\d+)`)
	dnNumPat       = regexp.MustCompile(`(?i)(DN\s*\d+-\d+|Delivery\s*note\s*\d+-\d+)`)
	poNumPat       = regexp.MustCompile(`(?i)(PO[-_\s]?\d+|4\d{7})`)
)

func main() {
	basePath := "."
	if len(os.Args) > 1 {
		basePath = os.Args[1]
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("🚀 BATCH DOCUMENT EXTRACTION")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("📂 Base path: %s\n\n", basePath)

	summary := &ExtractionSummary{
		StartTime:     time.Now(),
		Invoices:      []ExtractedInvoiceBasic{},
		DeliveryNotes: []ExtractedDNBasic{},
		CustomerPOs:   []ExtractedPOBasic{},
		Errors:        []string{},
	}

	customerSet := make(map[string]bool)

	// Process invoices
	invoiceFile := filepath.Join(basePath, "documents_invoice.txt")
	if files, err := readFileList(invoiceFile); err == nil {
		fmt.Printf("📄 Processing %d invoices...\n", len(files))
		for _, f := range files {
			inv := extractInvoiceMetadata(f)
			summary.Invoices = append(summary.Invoices, inv)
			if inv.CustomerName != "" {
				customerSet[inv.CustomerName] = true
			}
			summary.TotalProcessed++
			summary.SuccessCount++
		}
	}

	// Process delivery notes
	dnFile := filepath.Join(basePath, "documents_delivery_note.txt")
	if files, err := readFileList(dnFile); err == nil {
		fmt.Printf("📦 Processing %d delivery notes...\n", len(files))
		for _, f := range files {
			dn := extractDNMetadata(f)
			summary.DeliveryNotes = append(summary.DeliveryNotes, dn)
			if dn.CustomerName != "" {
				customerSet[dn.CustomerName] = true
			}
			summary.TotalProcessed++
			summary.SuccessCount++
		}
	}

	// Process customer POs
	poFile := filepath.Join(basePath, "documents_customer_po.txt")
	if files, err := readFileList(poFile); err == nil {
		fmt.Printf("🧾 Processing %d customer POs...\n", len(files))
		for _, f := range files {
			po := extractPOMetadata(f)
			summary.CustomerPOs = append(summary.CustomerPOs, po)
			if po.CustomerName != "" {
				customerSet[po.CustomerName] = true
			}
			summary.TotalProcessed++
			summary.SuccessCount++
		}
	}

	// Build unique customer list
	for c := range customerSet {
		summary.UniqueCustomers = append(summary.UniqueCustomers, c)
	}

	summary.EndTime = time.Now()
	summary.Duration = summary.EndTime.Sub(summary.StartTime).String()

	// Print summary
	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("📊 EXTRACTION SUMMARY")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Total documents processed: %d\n", summary.TotalProcessed)
	fmt.Printf("  - Invoices:       %d\n", len(summary.Invoices))
	fmt.Printf("  - Delivery Notes: %d\n", len(summary.DeliveryNotes))
	fmt.Printf("  - Customer POs:   %d\n", len(summary.CustomerPOs))
	fmt.Printf("Unique customers:    %d\n", len(summary.UniqueCustomers))
	fmt.Printf("Processing time:     %s\n", summary.Duration)

	// Group by offer number for analysis
	offerMap := make(map[string]int)
	for _, inv := range summary.Invoices {
		if inv.OfferNumber != "" {
			offerMap[inv.OfferNumber]++
		}
	}
	for _, dn := range summary.DeliveryNotes {
		if dn.OfferNumber != "" {
			offerMap[dn.OfferNumber]++
		}
	}
	for _, po := range summary.CustomerPOs {
		if po.OfferNumber != "" {
			offerMap[po.OfferNumber]++
		}
	}

	fmt.Printf("\n📋 OFFERS COVERAGE:\n")
	fmt.Printf("  Offers with documents: %d\n", len(offerMap))

	// List top customers
	fmt.Printf("\n🏢 CUSTOMERS FOUND:\n")
	for i, c := range summary.UniqueCustomers {
		if i >= 20 {
			fmt.Printf("  ... and %d more\n", len(summary.UniqueCustomers)-20)
			break
		}
		fmt.Printf("  - %s\n", c)
	}

	// Save results
	outputFile := filepath.Join(basePath, fmt.Sprintf("batch_extraction_%s.json", time.Now().Format("20060102_150405")))
	jsonData, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		log.Printf("Error saving results: %v", err)
	} else {
		fmt.Printf("\n💾 Results saved to: %s\n", outputFile)
	}

	fmt.Println("\n✅ BATCH EXTRACTION COMPLETE!")
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

func extractInvoiceMetadata(filePath string) ExtractedInvoiceBasic {
	inv := ExtractedInvoiceBasic{
		FilePath: filePath,
	}

	// Extract from path
	parts := strings.Split(filePath, string(filepath.Separator))
	for i, p := range parts {
		pLower := strings.ToLower(p)

		// Find offer folder (e.g., "101 ENGIE AIT")
		if match := folderNumPat.FindStringSubmatch(p); len(match) > 1 {
			inv.OfferNumber = match[1]
			// Customer name is after the number
			rest := strings.TrimPrefix(p, match[0])
			rest = strings.TrimSpace(rest)
			// Remove product type
			if idx := productTypePat.FindStringIndex(rest); idx != nil {
				inv.CustomerName = strings.TrimSpace(rest[:idx[0]])
				if typeMatch := productTypePat.FindStringSubmatch(rest); len(typeMatch) > 1 {
					inv.ProductType = typeMatch[1]
				}
			} else {
				inv.CustomerName = rest
			}
		}

		// Find stage
		if pLower == "execution" || strings.Contains(pLower, "execution") {
			inv.Stage = "EXECUTION"
		} else if pLower == "offer" {
			inv.Stage = "OFFER"
		} else if pLower == "rfq" {
			inv.Stage = "RFQ"
		}
		_ = i
	}

	// Extract invoice number from filename
	filename := filepath.Base(filePath)
	if match := invoiceNumPat.FindString(filename); match != "" {
		inv.InvoiceNumber = match
	}

	return inv
}

func extractDNMetadata(filePath string) ExtractedDNBasic {
	dn := ExtractedDNBasic{
		FilePath: filePath,
	}

	parts := strings.Split(filePath, string(filepath.Separator))
	for _, p := range parts {
		pLower := strings.ToLower(p)

		if match := folderNumPat.FindStringSubmatch(p); len(match) > 1 {
			dn.OfferNumber = match[1]
			rest := strings.TrimPrefix(p, match[0])
			rest = strings.TrimSpace(rest)
			if idx := productTypePat.FindStringIndex(rest); idx != nil {
				dn.CustomerName = strings.TrimSpace(rest[:idx[0]])
				if typeMatch := productTypePat.FindStringSubmatch(rest); len(typeMatch) > 1 {
					dn.ProductType = typeMatch[1]
				}
			} else {
				dn.CustomerName = rest
			}
		}

		if pLower == "execution" || strings.Contains(pLower, "execution") {
			dn.Stage = "EXECUTION"
		} else if pLower == "offer" {
			dn.Stage = "OFFER"
		}
	}

	// Extract DN number from filename
	filename := filepath.Base(filePath)
	if match := dnNumPat.FindString(filename); match != "" {
		dn.DNNumber = strings.ReplaceAll(match, " ", "")
	} else {
		// Try to extract from filename like "Delivery note 129-25"
		numMatch := regexp.MustCompile(`(\d+-\d+)`).FindString(filename)
		if numMatch != "" {
			dn.DNNumber = "DN-" + numMatch
		}
	}

	return dn
}

func extractPOMetadata(filePath string) ExtractedPOBasic {
	po := ExtractedPOBasic{
		FilePath: filePath,
	}

	parts := strings.Split(filePath, string(filepath.Separator))
	for _, p := range parts {
		pLower := strings.ToLower(p)

		if match := folderNumPat.FindStringSubmatch(p); len(match) > 1 {
			po.OfferNumber = match[1]
			rest := strings.TrimPrefix(p, match[0])
			rest = strings.TrimSpace(rest)
			if idx := productTypePat.FindStringIndex(rest); idx != nil {
				po.CustomerName = strings.TrimSpace(rest[:idx[0]])
				if typeMatch := productTypePat.FindStringSubmatch(rest); len(typeMatch) > 1 {
					po.ProductType = typeMatch[1]
				}
			} else {
				po.CustomerName = rest
			}
		}

		if pLower == "execution" || strings.Contains(pLower, "execution") {
			po.Stage = "EXECUTION"
		} else if pLower == "offer" {
			po.Stage = "OFFER"
		}
	}

	// Extract PO number from filename
	filename := filepath.Base(filePath)
	if match := poNumPat.FindString(filename); match != "" {
		po.PONumber = match
	}

	return po
}

// linkOfferToCosting attempts to match extracted offer numbers to costing data
func linkOfferToCosting(offerNum string, costingData map[string]float64) float64 {
	// Convert offer number to int for comparison
	num, err := strconv.Atoi(offerNum)
	if err != nil {
		return 0
	}

	// Look up in costing data
	if val, ok := costingData[offerNum]; ok {
		return val
	}

	_ = num
	return 0
}
