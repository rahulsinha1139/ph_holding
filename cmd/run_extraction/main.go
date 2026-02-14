package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Document types
const (
	DOC_INVOICE         = "INVOICE"
	DOC_DELIVERY_NOTE   = "DELIVERY_NOTE"
	DOC_CUSTOMER_PO     = "CUSTOMER_PO"
	DOC_SUPPLIER_PO_ACK = "SUPPLIER_PO_ACK"
	DOC_INTERNAL_PO     = "INTERNAL_PO"
	DOC_RFQ_EMAIL       = "RFQ_EMAIL"
	DOC_RFQ_DOCUMENT    = "RFQ_DOCUMENT"
	DOC_COSTING_SHEET   = "COSTING_SHEET"
	DOC_COMMERCIAL_OFFER = "COMMERCIAL_OFFER"
	DOC_SHIPPING_DOC    = "SHIPPING_DOC"
	DOC_TECHNICAL_DOC   = "TECHNICAL_DOC"
	DOC_OTHER           = "OTHER"
)

type DocumentClassification struct {
	FilePath     string `json:"file_path"`
	FileName     string `json:"file_name"`
	DocumentType string `json:"document_type"`
	OfferNumber  string `json:"offer_number"`
	CustomerName string `json:"customer_name"`
	ProductType  string `json:"product_type"`
	Stage        string `json:"stage"`
	FileSize     int64  `json:"file_size"`
}

type ClassificationSummary struct {
	TotalFiles    int                      `json:"total_files"`
	ByType        map[string]int           `json:"by_type"`
	ByCustomer    map[string]int           `json:"by_customer"`
	ByOfferNumber map[string]int           `json:"by_offer_number"`
	ByStage       map[string]int           `json:"by_stage"`
	Documents     []DocumentClassification `json:"documents"`
}

// Regex patterns
var (
	offerNumberPattern = regexp.MustCompile(`^(\d+)[-\s]`)
	albaPOPattern      = regexp.MustCompile(`45[0-9]{7}`)
	bapcoPOPattern     = regexp.MustCompile(`PO_81_[0-9]+`)
	ehOrderPattern     = regexp.MustCompile(`601[0-9]{7}`)
	phPOPattern        = regexp.MustCompile(`PH\s*25[-\s]*\d+`)
	productTypePattern = regexp.MustCompile(`[-\s](AIT|FIT|LIT|SP|TIT|FEED|ETS|TT|VALVE|SPARE)[-\s]?`)
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: run_extraction <folder_path>")
		os.Exit(1)
	}

	basePath := os.Args[1]
	fmt.Printf("🚀 Starting document extraction from: %s\n", basePath)
	fmt.Println("═══════════════════════════════════════════════════════════════")

	startTime := time.Now()

	summary := &ClassificationSummary{
		ByType:        make(map[string]int),
		ByCustomer:    make(map[string]int),
		ByOfferNumber: make(map[string]int),
		ByStage:       make(map[string]int),
		Documents:     []DocumentClassification{},
	}

	// Walk the directory
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			return nil
		}

		// Skip hidden files and temp files
		if strings.HasPrefix(info.Name(), ".") || strings.HasPrefix(info.Name(), "~$") {
			return nil
		}

		doc := classifyDocument(path, info, basePath)
		summary.Documents = append(summary.Documents, doc)
		summary.TotalFiles++
		summary.ByType[doc.DocumentType]++

		if doc.CustomerName != "" {
			summary.ByCustomer[doc.CustomerName]++
		}
		if doc.OfferNumber != "" {
			summary.ByOfferNumber[doc.OfferNumber]++
		}
		if doc.Stage != "" {
			summary.ByStage[doc.Stage]++
		}

		return nil
	})

	if err != nil {
		log.Fatalf("Walk error: %v", err)
	}

	elapsed := time.Since(startTime)

	// Print summary
	fmt.Println("\n📊 CLASSIFICATION SUMMARY")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Total files scanned: %d\n", summary.TotalFiles)
	fmt.Printf("Time elapsed: %v\n\n", elapsed)

	fmt.Println("📁 BY DOCUMENT TYPE:")
	fmt.Println("───────────────────────────────────────────────────────────────")
	for docType, count := range summary.ByType {
		fmt.Printf("  %-20s: %5d files\n", docType, count)
	}

	fmt.Println("\n📂 BY STAGE:")
	fmt.Println("───────────────────────────────────────────────────────────────")
	for stage, count := range summary.ByStage {
		fmt.Printf("  %-20s: %5d files\n", stage, count)
	}

	fmt.Println("\n🏢 TOP 20 CUSTOMERS:")
	fmt.Println("───────────────────────────────────────────────────────────────")
	topCustomers := getTopN(summary.ByCustomer, 20)
	for _, kv := range topCustomers {
		fmt.Printf("  %-35s: %5d files\n", kv.Key, kv.Value)
	}

	fmt.Println("\n📋 OFFER NUMBER RANGE:")
	fmt.Println("───────────────────────────────────────────────────────────────")
	offerCount := len(summary.ByOfferNumber)
	fmt.Printf("  Unique offers: %d\n", offerCount)

	// Count key document types
	invoices := summary.ByType[DOC_INVOICE]
	deliveryNotes := summary.ByType[DOC_DELIVERY_NOTE]
	customerPOs := summary.ByType[DOC_CUSTOMER_PO]
	supplierPOs := summary.ByType[DOC_SUPPLIER_PO_ACK]
	costingSheets := summary.ByType[DOC_COSTING_SHEET]
	rfqEmails := summary.ByType[DOC_RFQ_EMAIL]
	commercialOffers := summary.ByType[DOC_COMMERCIAL_OFFER]

	fmt.Println("\n💰 KEY BUSINESS DOCUMENTS:")
	fmt.Println("───────────────────────────────────────────────────────────────")
	fmt.Printf("  Invoices:          %5d (ready for OCR extraction)\n", invoices)
	fmt.Printf("  Delivery Notes:    %5d (ready for OCR extraction)\n", deliveryNotes)
	fmt.Printf("  Customer POs:      %5d (ready for OCR extraction)\n", customerPOs)
	fmt.Printf("  Supplier PO Acks:  %5d (E+H order confirmations)\n", supplierPOs)
	fmt.Printf("  Costing Sheets:    %5d (Excel extraction ready)\n", costingSheets)
	fmt.Printf("  RFQ Emails:        %5d (MSG parsing ready)\n", rfqEmails)
	fmt.Printf("  Commercial Offers: %5d (reference documents)\n", commercialOffers)

	// Save full results to JSON
	outputPath := filepath.Join(filepath.Dir(basePath), "extraction_results.json")
	jsonData, _ := json.MarshalIndent(summary, "", "  ")
	os.WriteFile(outputPath, jsonData, 0644)
	fmt.Printf("\n📄 Full results saved to: %s\n", outputPath)

	// Save documents by type for easy access
	saveDocumentsByType(summary, filepath.Dir(basePath))

	fmt.Println("\n✅ EXTRACTION COMPLETE!")
	fmt.Println("═══════════════════════════════════════════════════════════════")
}

func classifyDocument(path string, info os.FileInfo, basePath string) DocumentClassification {
	doc := DocumentClassification{
		FilePath: path,
		FileName: info.Name(),
		FileSize: info.Size(),
	}

	// Get relative path for analysis
	relPath, _ := filepath.Rel(basePath, path)
	nameLower := strings.ToLower(info.Name())
	pathLower := strings.ToLower(relPath)

	// Extract offer number from path
	parts := strings.Split(relPath, string(filepath.Separator))
	for _, part := range parts {
		if match := offerNumberPattern.FindStringSubmatch(part); len(match) > 1 {
			doc.OfferNumber = match[1]
			// Extract customer name (after the number)
			rest := strings.TrimPrefix(part, match[0])
			rest = strings.TrimSpace(rest)
			// Remove product type suffix
			if idx := productTypePattern.FindStringIndex(rest); idx != nil {
				doc.CustomerName = strings.TrimSpace(rest[:idx[0]])
			} else {
				doc.CustomerName = rest
			}
			break
		}
	}

	// Extract product type
	if match := productTypePattern.FindStringSubmatch(relPath); len(match) > 1 {
		doc.ProductType = match[1]
	}

	// Determine stage from path
	if strings.Contains(pathLower, "/execution/") || strings.Contains(pathLower, "\\execution\\") {
		doc.Stage = "EXECUTION"
	} else if strings.Contains(pathLower, "/offer/") || strings.Contains(pathLower, "\\offer\\") {
		doc.Stage = "OFFER"
	} else if strings.Contains(pathLower, "/rfq/") || strings.Contains(pathLower, "\\rfq\\") {
		doc.Stage = "RFQ"
	}

	// Classify document type
	ext := strings.ToLower(filepath.Ext(info.Name()))

	// RFQ emails
	if ext == ".msg" {
		doc.DocumentType = DOC_RFQ_EMAIL
		return doc
	}

	// Costing sheets
	if (ext == ".xlsx" || ext == ".xls") && (strings.Contains(nameLower, "costing") || strings.Contains(nameLower, "masterfile")) {
		doc.DocumentType = DOC_COSTING_SHEET
		return doc
	}

	// Invoice patterns
	if strings.Contains(nameLower, "invoice") || strings.Contains(nameLower, " inv") ||
	   strings.Contains(nameLower, "_inv") || strings.Contains(nameLower, "-inv") {
		doc.DocumentType = DOC_INVOICE
		return doc
	}

	// Delivery note patterns
	if strings.Contains(nameLower, "delivery") || strings.Contains(nameLower, " dn") ||
	   strings.Contains(nameLower, "_dn") || strings.Contains(nameLower, "-dn") ||
	   strings.Contains(nameLower, "dn ") || strings.Contains(nameLower, "dn.") {
		doc.DocumentType = DOC_DELIVERY_NOTE
		return doc
	}

	// Customer PO patterns (ALBA, BAPCO)
	if albaPOPattern.MatchString(info.Name()) || bapcoPOPattern.MatchString(info.Name()) {
		doc.DocumentType = DOC_CUSTOMER_PO
		return doc
	}

	// E+H supplier order acknowledgments
	if ehOrderPattern.MatchString(info.Name()) {
		doc.DocumentType = DOC_SUPPLIER_PO_ACK
		return doc
	}

	// PH internal PO
	if phPOPattern.MatchString(info.Name()) || strings.Contains(nameLower, "ph 25-") || strings.Contains(nameLower, "ph25-") {
		doc.DocumentType = DOC_INTERNAL_PO
		return doc
	}

	// PO in name (generic)
	if strings.Contains(nameLower, " po ") || strings.Contains(nameLower, "_po_") ||
	   strings.Contains(nameLower, "-po-") || strings.Contains(nameLower, "po_") ||
	   strings.Contains(nameLower, "purchase order") {
		doc.DocumentType = DOC_CUSTOMER_PO
		return doc
	}

	// Commercial offer (PDF in OFFER folder)
	if ext == ".pdf" && doc.Stage == "OFFER" && (strings.Contains(nameLower, "offer") || strings.Contains(nameLower, "commercial") || strings.Contains(nameLower, "quotation")) {
		doc.DocumentType = DOC_COMMERCIAL_OFFER
		return doc
	}

	// RFQ documents (in RFQ folder)
	if doc.Stage == "RFQ" && ext == ".pdf" {
		doc.DocumentType = DOC_RFQ_DOCUMENT
		return doc
	}

	// Shipping documents
	if strings.Contains(pathLower, "shipment") || strings.Contains(nameLower, "shipping") ||
	   strings.Contains(nameLower, "tracking") || strings.Contains(nameLower, "awb") {
		doc.DocumentType = DOC_SHIPPING_DOC
		return doc
	}

	// Technical documents
	if strings.Contains(nameLower, "technical") || strings.Contains(nameLower, "msds") ||
	   strings.Contains(nameLower, "datasheet") || strings.Contains(nameLower, "manual") ||
	   strings.Contains(nameLower, " ti") || strings.Contains(nameLower, "_ti") {
		doc.DocumentType = DOC_TECHNICAL_DOC
		return doc
	}

	doc.DocumentType = DOC_OTHER
	return doc
}

type KV struct {
	Key   string
	Value int
}

func getTopN(m map[string]int, n int) []KV {
	var sorted []KV
	for k, v := range m {
		sorted = append(sorted, KV{k, v})
	}
	// Simple bubble sort for top N
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].Value > sorted[i].Value {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}
	if len(sorted) > n {
		return sorted[:n]
	}
	return sorted
}

func saveDocumentsByType(summary *ClassificationSummary, outputDir string) {
	// Group documents by type
	byType := make(map[string][]string)
	for _, doc := range summary.Documents {
		byType[doc.DocumentType] = append(byType[doc.DocumentType], doc.FilePath)
	}

	// Save each type to a separate file
	for docType, paths := range byType {
		filename := fmt.Sprintf("documents_%s.txt", strings.ToLower(docType))
		outputPath := filepath.Join(outputDir, filename)
		content := strings.Join(paths, "\n")
		os.WriteFile(outputPath, []byte(content), 0644)
	}
	fmt.Printf("📝 Document lists saved by type to: %s/documents_*.txt\n", outputDir)
}
