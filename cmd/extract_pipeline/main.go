package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ═══════════════════════════════════════════════════════════════════════════
// EXTRACT PIPELINE - Complete Offer-to-Document Mapping
// ═══════════════════════════════════════════════════════════════════════════
// Maps all documents back to their offer numbers and links with costing data.
// This creates the complete picture of each offer's lifecycle:
//   Offer Folder → RFQ → Costing → Invoice → Delivery Note → Payment
// ═══════════════════════════════════════════════════════════════════════════

type OfferRecord struct {
	OfferNumber   string              `json:"offer_number"`
	CustomerName  string              `json:"customer_name"`
	ProductType   string              `json:"product_type"`
	HasRFQ        bool                `json:"has_rfq"`
	HasCosting    bool                `json:"has_costing"`
	HasOffer      bool                `json:"has_offer"`
	HasExecution  bool                `json:"has_execution"`
	HasInvoice    bool                `json:"has_invoice"`
	HasDN         bool                `json:"has_delivery_note"`
	HasCustomerPO bool                `json:"has_customer_po"`
	CostingValue  float64             `json:"costing_value_bhd"`
	Documents     []DocumentReference `json:"documents"`
}

type DocumentReference struct {
	Type     string `json:"type"`
	FilePath string `json:"file_path"`
	FileName string `json:"file_name"`
	Stage    string `json:"stage"`
}

type PipelineSummary struct {
	GeneratedAt       time.Time              `json:"generated_at"`
	TotalOffers       int                    `json:"total_offers"`
	OffersWithRFQ     int                    `json:"offers_with_rfq"`
	OffersWithCosting int                    `json:"offers_with_costing"`
	OffersWithInvoice int                    `json:"offers_with_invoice"`
	OffersWithDN      int                    `json:"offers_with_dn"`
	TotalCostingValue float64                `json:"total_costing_value_bhd"`
	CustomerStats     map[string]int         `json:"customer_stats"`
	ProductTypeStats  map[string]int         `json:"product_type_stats"`
	Offers            map[string]OfferRecord `json:"offers"`
}

// Known customer names for fuzzy matching
var knownCustomers = map[string]string{
	"ENGIE":        "ENGIE",
	"BAPCO":        "BAPCO",
	"ALBA":         "ALBA",
	"SULB":         "SULB",
	"EWA":          "EWA",
	"TTSJV":        "TTSJV",
	"SEAPEAK":      "SEAPEAK",
	"ARLA":         "ARLA",
	"DELTA":        "DELTA",
	"TABREED":      "TABREED",
	"NOMAC":        "NOMAC",
	"VEOLIA":       "VEOLIA",
	"HPC":          "HPC",
	"YOKOGAWA":     "YOKOGAWA",
	"MUHARRAQ":     "MUHARRAQ",
	"BAHRAIN STEEL": "BAHRAIN STEEL",
	"WABAG":        "WABAG",
	"EAGLEHILLS":   "EAGLEHILLS",
	"COMSIP":       "COMSIP",
	"MESAS":        "MESAS",
	"MWW":          "MWW",
	"OMNI":         "OMNI",
	"AIRMECH":      "AIRMECH",
	"K&S":          "K&S",
	"SYSCON":       "SYSCON",
	"EVEREST":      "EVEREST",
	"AHS":          "AHS",
	"FIVES":        "FIVES",
	"JBF":          "JBF",
	"AL MADAR":     "AL MADAR",
	"WAHA":         "WAHA",
	"ALUSERVE":     "ALUSERVE",
}

var (
	offerFolderPat = regexp.MustCompile(`^(\d{1,3})[-\s](.+)$`)
	productTypePat = regexp.MustCompile(`[-\s](AIT|FIT|LIT|SP|TIT|FEED|ETS|TT|VALVE|SPARE|PIT|ORP|BTU|LS|SENSOR|SERVICE|CALIBRATION|WDD)[-\s]?`)
)

func main() {
	basePath := "."
	if len(os.Args) > 1 {
		basePath = os.Args[1]
	}

	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println("🔄 EXTRACTION PIPELINE - Offer to Document Mapping")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	summary := &PipelineSummary{
		GeneratedAt:      time.Now(),
		CustomerStats:    make(map[string]int),
		ProductTypeStats: make(map[string]int),
		Offers:           make(map[string]OfferRecord),
	}

	// Load costing data if available
	costingData := loadCostingData(basePath)
	fmt.Printf("📊 Loaded %d costing records\n", len(costingData))

	// Process each document type file
	docTypes := map[string]string{
		"invoice":       "documents_invoice.txt",
		"delivery_note": "documents_delivery_note.txt",
		"customer_po":   "documents_customer_po.txt",
		"rfq_email":     "documents_rfq_email.txt",
		"costing_sheet": "documents_costing_sheet.txt",
		"commercial_offer": "documents_commercial_offer.txt",
	}

	for docType, fileName := range docTypes {
		filePath := filepath.Join(basePath, fileName)
		files, err := readFileList(filePath)
		if err != nil {
			continue
		}
		fmt.Printf("📁 Processing %d %s files...\n", len(files), docType)

		for _, f := range files {
			offerNum, customer, productType := extractOfferInfo(f)
			if offerNum == "" || len(offerNum) > 3 {
				continue // Skip non-offer numbers
			}

			offer, exists := summary.Offers[offerNum]
			if !exists {
				offer = OfferRecord{
					OfferNumber:  offerNum,
					CustomerName: customer,
					ProductType:  productType,
					Documents:    []DocumentReference{},
				}
			}

			// Better customer name extraction
			if offer.CustomerName == "" && customer != "" {
				offer.CustomerName = customer
			}
			if offer.ProductType == "" && productType != "" {
				offer.ProductType = productType
			}

			// Determine stage
			stage := extractStage(f)

			// Add document reference
			doc := DocumentReference{
				Type:     docType,
				FilePath: f,
				FileName: filepath.Base(f),
				Stage:    stage,
			}
			offer.Documents = append(offer.Documents, doc)

			// Set flags
			switch docType {
			case "invoice":
				offer.HasInvoice = true
				offer.HasExecution = true
			case "delivery_note":
				offer.HasDN = true
				offer.HasExecution = true
			case "customer_po":
				offer.HasCustomerPO = true
			case "rfq_email":
				offer.HasRFQ = true
			case "costing_sheet":
				offer.HasCosting = true
			case "commercial_offer":
				offer.HasOffer = true
			}

			// Link costing value if available
			if val, ok := costingData[offerNum]; ok {
				offer.CostingValue = val
				offer.HasCosting = true
			}

			summary.Offers[offerNum] = offer
		}
	}

	// Calculate stats
	for _, offer := range summary.Offers {
		summary.TotalOffers++
		if offer.HasRFQ {
			summary.OffersWithRFQ++
		}
		if offer.HasCosting {
			summary.OffersWithCosting++
		}
		if offer.HasInvoice {
			summary.OffersWithInvoice++
		}
		if offer.HasDN {
			summary.OffersWithDN++
		}
		summary.TotalCostingValue += offer.CostingValue

		if offer.CustomerName != "" {
			customer := normalizeCustomer(offer.CustomerName)
			summary.CustomerStats[customer]++
		}
		if offer.ProductType != "" {
			summary.ProductTypeStats[offer.ProductType]++
		}
	}

	// Print summary
	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("📊 PIPELINE SUMMARY")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Total offers tracked:       %d\n", summary.TotalOffers)
	fmt.Printf("Offers with RFQ:            %d (%.1f%%)\n", summary.OffersWithRFQ, pct(summary.OffersWithRFQ, summary.TotalOffers))
	fmt.Printf("Offers with Costing:        %d (%.1f%%)\n", summary.OffersWithCosting, pct(summary.OffersWithCosting, summary.TotalOffers))
	fmt.Printf("Offers with Invoice:        %d (%.1f%%)\n", summary.OffersWithInvoice, pct(summary.OffersWithInvoice, summary.TotalOffers))
	fmt.Printf("Offers with Delivery Note:  %d (%.1f%%)\n", summary.OffersWithDN, pct(summary.OffersWithDN, summary.TotalOffers))
	fmt.Printf("Total Costing Value:        %.3f BHD (~$%.0f USD)\n", summary.TotalCostingValue, summary.TotalCostingValue*2.65)

	// Top customers
	fmt.Println("\n🏢 TOP CUSTOMERS:")
	printTopN(summary.CustomerStats, 15)

	// Product types
	fmt.Println("\n📦 PRODUCT TYPES:")
	printTopN(summary.ProductTypeStats, 15)

	// Offers with complete lifecycle
	completeOffers := []string{}
	for num, offer := range summary.Offers {
		if offer.HasRFQ && offer.HasCosting && offer.HasInvoice && offer.HasDN {
			completeOffers = append(completeOffers, num)
		}
	}
	sort.Strings(completeOffers)
	fmt.Printf("\n✅ COMPLETE LIFECYCLE OFFERS (RFQ→Costing→Invoice→DN): %d\n", len(completeOffers))
	if len(completeOffers) > 0 && len(completeOffers) <= 20 {
		fmt.Printf("   %v\n", completeOffers)
	}

	// Missing costing data
	missingCosting := []string{}
	for num, offer := range summary.Offers {
		if offer.HasInvoice && !offer.HasCosting {
			missingCosting = append(missingCosting, num)
		}
	}
	sort.Strings(missingCosting)
	fmt.Printf("\n⚠️  INVOICED BUT NO COSTING DATA: %d offers\n", len(missingCosting))
	if len(missingCosting) > 0 && len(missingCosting) <= 20 {
		fmt.Printf("   %v\n", missingCosting)
	}

	// Save results
	outputFile := filepath.Join(basePath, "offer_pipeline_summary.json")
	jsonData, _ := json.MarshalIndent(summary, "", "  ")
	if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
		fmt.Printf("Error saving: %v\n", err)
	} else {
		fmt.Printf("\n💾 Results saved to: %s\n", outputFile)
	}

	// Save offer mapping CSV for easy viewing
	csvFile := filepath.Join(basePath, "offer_mapping.csv")
	saveOfferCSV(summary.Offers, csvFile)
	fmt.Printf("📋 Offer mapping saved to: %s\n", csvFile)

	fmt.Println("\n✅ PIPELINE EXTRACTION COMPLETE!")
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
		// Look for offer folder pattern like "101 ENGIE AIT" or "112-BAPCO-SP"
		if match := offerFolderPat.FindStringSubmatch(p); len(match) > 2 {
			offerNum = match[1]
			rest := match[2]

			// Extract product type
			if ptMatch := productTypePat.FindStringSubmatch(rest); len(ptMatch) > 1 {
				productType = ptMatch[1]
				// Customer is before product type
				idx := strings.Index(rest, ptMatch[0])
				if idx > 0 {
					customer = strings.TrimSpace(rest[:idx])
				}
			} else {
				customer = strings.TrimSpace(rest)
			}

			// Clean up customer name
			customer = strings.Trim(customer, "- ")
			customer = strings.TrimSuffix(customer, "(2)")
			customer = strings.TrimSpace(customer)

			break
		}
	}
	return
}

func extractStage(filePath string) string {
	pathLower := strings.ToLower(filePath)
	if strings.Contains(pathLower, "execution") || strings.Contains(pathLower, "project execution") {
		return "EXECUTION"
	}
	if strings.Contains(pathLower, "\\offer\\") || strings.Contains(pathLower, "/offer/") {
		return "OFFER"
	}
	if strings.Contains(pathLower, "\\rfq\\") || strings.Contains(pathLower, "/rfq/") {
		return "RFQ"
	}
	return ""
}

func normalizeCustomer(name string) string {
	nameUpper := strings.ToUpper(name)
	for key, normalized := range knownCustomers {
		if strings.Contains(nameUpper, key) {
			return normalized
		}
	}
	return name
}

func loadCostingData(basePath string) map[string]float64 {
	result := make(map[string]float64)

	// Try to load from extraction_results.json if available
	extractFile := filepath.Join(basePath, "extraction_results.json")
	data, err := os.ReadFile(extractFile)
	if err != nil {
		return result
	}

	var extResults struct {
		Documents []struct {
			OfferNumber string `json:"offer_number"`
			DocumentType string `json:"document_type"`
		} `json:"documents"`
	}

	if err := json.Unmarshal(data, &extResults); err != nil {
		return result
	}

	// For now just mark that costing exists - actual values need deeper parsing
	for _, doc := range extResults.Documents {
		if doc.DocumentType == "COSTING_SHEET" && doc.OfferNumber != "" {
			result[doc.OfferNumber] = 1.0 // Mark as having costing
		}
	}

	return result
}

func pct(part, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
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
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Value > sorted[j].Value
	})

	for i, item := range sorted {
		if i >= n {
			break
		}
		fmt.Printf("  %-25s: %3d offers\n", item.Key, item.Value)
	}
}

func saveOfferCSV(offers map[string]OfferRecord, path string) {
	file, err := os.Create(path)
	if err != nil {
		return
	}
	defer file.Close()

	// Header
	file.WriteString("Offer,Customer,ProductType,HasRFQ,HasCosting,HasInvoice,HasDN,CostingBHD,DocumentCount\n")

	// Sort offers by number
	var keys []string
	for k := range offers {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i] < keys[j]
	})

	for _, k := range keys {
		o := offers[k]
		file.WriteString(fmt.Sprintf("%s,%s,%s,%v,%v,%v,%v,%.3f,%d\n",
			o.OfferNumber,
			strings.ReplaceAll(o.CustomerName, ",", " "),
			o.ProductType,
			o.HasRFQ,
			o.HasCosting,
			o.HasInvoice,
			o.HasDN,
			o.CostingValue,
			len(o.Documents),
		))
	}
}
