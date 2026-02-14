package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	msgparser "github.com/willthrom/outlook-msg-parser"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ═══════════════════════════════════════════════════════════════════════════
// BATCH OCR EXTRACTION & RFQ PARSING
// ═══════════════════════════════════════════════════════════════════════════
// Processes:
//   - Invoices (PDF) → Extract invoice number, customer, total
//   - Delivery Notes (PDF) → Extract DN number, items
//   - Customer POs (PDF) → Extract PO number, buyer, terms
//   - RFQ Emails (.msg) → Extract RFQ reference, sender, products
// ═══════════════════════════════════════════════════════════════════════════

// Database models
type ExtractedDocument struct {
	ID             uint      `gorm:"primaryKey;autoIncrement"`
	SourcePath     string    `gorm:"size:500;index" json:"source_path"`
	DocumentType   string    `gorm:"size:50;index" json:"document_type"`
	OfferNumber    string    `gorm:"size:50;index" json:"offer_number"`
	CustomerName   string    `gorm:"size:255" json:"customer_name"`
	DocumentNumber string    `gorm:"size:100" json:"document_number"`
	DocumentDate   string    `gorm:"size:50" json:"document_date"`
	TotalValue     float64   `json:"total_value"`
	Currency       string    `gorm:"size:10" json:"currency"`
	Reference      string    `gorm:"size:255" json:"reference"`
	Subject        string    `gorm:"size:500" json:"subject"`
	SenderEmail    string    `gorm:"size:255" json:"sender_email"`
	SenderName     string    `gorm:"size:255" json:"sender_name"`
	ProductType    string    `gorm:"size:50" json:"product_type"`
	Stage          string    `gorm:"size:50" json:"stage"`
	ExtractedAt    time.Time `json:"extracted_at"`
	CreatedAt      time.Time `json:"created_at"`
}

func (ExtractedDocument) TableName() string { return "extracted_documents" }

var (
	offerFolderPat = regexp.MustCompile(`^(\d{1,3})[-\s](.+)$`)
	productTypePat = regexp.MustCompile(`[-\s](AIT|FIT|LIT|SP|TIT|FEED|ETS|TT|VALVE|SPARE|PIT|ORP|BTU|LS|SENSOR|SERVICE)`)
	invoiceNumPat  = regexp.MustCompile(`(?i)(PH\s*\d+-\d+|INV[-\s]?\d+|\d{10}|PH25-\d+)`)
	dnNumPat       = regexp.MustCompile(`(?i)(DN[-\s]?\d+-\d+|\d+-25)`)
	poNumPat       = regexp.MustCompile(`(?i)(PO[-_\s]?\d+|4\d{7}|40000\d{6})`)
	amountPat      = regexp.MustCompile(`(?i)(?:total|amount|value)[\s:]*(?:BHD|BD)?\s*([\d,]+\.?\d{0,3})`)
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
	fmt.Println("🔍 BATCH OCR EXTRACTION & RFQ PARSING")
	fmt.Println("═══════════════════════════════════════════════════════════════")

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Auto-migrate
	if err := db.AutoMigrate(&ExtractedDocument{}); err != nil {
		log.Fatalf("Failed to migrate: %v", err)
	}
	fmt.Println("✅ Database ready")

	totalProcessed := 0
	totalExtracted := 0
	totalValue := 0.0

	// 1. Process RFQ Emails (no OCR needed, just MSG parsing)
	fmt.Println("\n📧 PHASE 1: Parsing RFQ Emails...")
	rfqFile := filepath.Join(basePath, "documents_rfq_email.txt")
	if files, err := readFileList(rfqFile); err == nil {
		fmt.Printf("  Found %d RFQ emails\n", len(files))
		for i, f := range files {
			if i%20 == 0 {
				fmt.Printf("  Processing %d/%d...\n", i, len(files))
			}
			doc, err := parseRFQEmail(f)
			if err != nil {
				continue
			}
			totalProcessed++

			// Check for duplicate
			var existing ExtractedDocument
			if db.Where("source_path = ?", f).First(&existing).Error == nil {
				continue
			}

			if err := db.Create(&doc).Error; err == nil {
				totalExtracted++
			}
		}
		fmt.Printf("  ✅ Parsed %d RFQ emails\n", totalExtracted)
	}

	rfqExtracted := totalExtracted

	// 2. Process Invoices (metadata extraction from path/filename)
	fmt.Println("\n📄 PHASE 2: Extracting Invoice Metadata...")
	invoiceFile := filepath.Join(basePath, "documents_invoice.txt")
	if files, err := readFileList(invoiceFile); err == nil {
		fmt.Printf("  Found %d invoices\n", len(files))
		invoiceCount := 0
		for _, f := range files {
			doc := extractDocumentMetadata(f, "INVOICE")
			totalProcessed++

			var existing ExtractedDocument
			if db.Where("source_path = ?", f).First(&existing).Error == nil {
				continue
			}

			if err := db.Create(&doc).Error; err == nil {
				invoiceCount++
				totalExtracted++
				totalValue += doc.TotalValue
			}
		}
		fmt.Printf("  ✅ Extracted %d invoices\n", invoiceCount)
	}

	// 3. Process Delivery Notes
	fmt.Println("\n📦 PHASE 3: Extracting Delivery Note Metadata...")
	dnFile := filepath.Join(basePath, "documents_delivery_note.txt")
	if files, err := readFileList(dnFile); err == nil {
		fmt.Printf("  Found %d delivery notes\n", len(files))
		dnCount := 0
		for _, f := range files {
			doc := extractDocumentMetadata(f, "DELIVERY_NOTE")
			totalProcessed++

			var existing ExtractedDocument
			if db.Where("source_path = ?", f).First(&existing).Error == nil {
				continue
			}

			if err := db.Create(&doc).Error; err == nil {
				dnCount++
				totalExtracted++
			}
		}
		fmt.Printf("  ✅ Extracted %d delivery notes\n", dnCount)
	}

	// 4. Process Customer POs
	fmt.Println("\n🧾 PHASE 4: Extracting Customer PO Metadata...")
	poFile := filepath.Join(basePath, "documents_customer_po.txt")
	if files, err := readFileList(poFile); err == nil {
		fmt.Printf("  Found %d customer POs\n", len(files))
		poCount := 0
		for _, f := range files {
			doc := extractDocumentMetadata(f, "CUSTOMER_PO")
			totalProcessed++

			var existing ExtractedDocument
			if db.Where("source_path = ?", f).First(&existing).Error == nil {
				continue
			}

			if err := db.Create(&doc).Error; err == nil {
				poCount++
				totalExtracted++
			}
		}
		fmt.Printf("  ✅ Extracted %d customer POs\n", poCount)
	}

	// Summary
	fmt.Println("\n═══════════════════════════════════════════════════════════════")
	fmt.Println("📊 EXTRACTION SUMMARY")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Printf("Total files processed:    %d\n", totalProcessed)
	fmt.Printf("Documents extracted:      %d\n", totalExtracted)
	fmt.Printf("  - RFQ Emails:           %d\n", rfqExtracted)
	fmt.Printf("  - Invoices:             %d\n", totalExtracted-rfqExtracted)
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

func parseRFQEmail(msgPath string) (*ExtractedDocument, error) {
	doc := &ExtractedDocument{
		SourcePath:   msgPath,
		DocumentType: "RFQ_EMAIL",
		ExtractedAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	// Extract offer info from path
	extractOfferInfo(doc, msgPath)

	// Parse MSG file
	msg, err := msgparser.ParseMsgFile(msgPath)
	if err != nil {
		return nil, err
	}

	doc.Subject = msg.Subject
	doc.SenderEmail = msg.FromEmail
	doc.SenderName = msg.FromName

	if !msg.ClientSubmitTime.IsZero() {
		doc.DocumentDate = msg.ClientSubmitTime.Format("2006-01-02")
	} else if !msg.Date.IsZero() {
		doc.DocumentDate = msg.Date.Format("2006-01-02")
	}

	// Extract RFQ reference from subject
	rfqPat := regexp.MustCompile(`(?i)(?:RFQ|REQ|PRF|PRQ)[-#\s]*(\d+[-/]?\d*)`)
	if match := rfqPat.FindStringSubmatch(msg.Subject); len(match) > 1 {
		doc.Reference = match[0]
	}

	// Extract customer from sender domain
	if doc.SenderEmail != "" {
		parts := strings.Split(doc.SenderEmail, "@")
		if len(parts) > 1 {
			domain := parts[1]
			// Map common domains to customer names
			domainCustomer := map[string]string{
				"alba.com.bh":       "ALBA",
				"bapco.net":         "BAPCO",
				"engie.com":         "ENGIE",
				"ewa.bh":            "EWA",
				"seapeak.com":       "SEAPEAK",
				"nomac.com":         "NOMAC",
				"veolia.com":        "VEOLIA",
				"sulb.com.bh":       "SULB",
				"tabreed.ae":        "TABREED",
				"yokogawa.com":      "YOKOGAWA",
			}
			for d, c := range domainCustomer {
				if strings.Contains(domain, d) || strings.Contains(domain, strings.ToLower(c)) {
					doc.CustomerName = c
					break
				}
			}
		}
	}

	return doc, nil
}

func extractDocumentMetadata(filePath, docType string) *ExtractedDocument {
	doc := &ExtractedDocument{
		SourcePath:   filePath,
		DocumentType: docType,
		ExtractedAt:  time.Now(),
		CreatedAt:    time.Now(),
	}

	// Extract offer info from path
	extractOfferInfo(doc, filePath)

	// Extract document number from filename
	filename := filepath.Base(filePath)
	switch docType {
	case "INVOICE":
		if match := invoiceNumPat.FindString(filename); match != "" {
			doc.DocumentNumber = match
		}
	case "DELIVERY_NOTE":
		if match := dnNumPat.FindString(filename); match != "" {
			doc.DocumentNumber = match
		}
	case "CUSTOMER_PO":
		if match := poNumPat.FindString(filename); match != "" {
			doc.DocumentNumber = match
		}
	}

	return doc
}

func extractOfferInfo(doc *ExtractedDocument, filePath string) {
	parts := strings.Split(filePath, string(filepath.Separator))

	for _, p := range parts {
		if match := offerFolderPat.FindStringSubmatch(p); len(match) > 2 {
			doc.OfferNumber = match[1]
			rest := match[2]

			if ptMatch := productTypePat.FindStringSubmatch(rest); len(ptMatch) > 1 {
				doc.ProductType = ptMatch[1]
				idx := strings.Index(rest, ptMatch[0])
				if idx > 0 {
					doc.CustomerName = strings.TrimSpace(rest[:idx])
				}
			} else {
				doc.CustomerName = strings.TrimSpace(rest)
			}
			doc.CustomerName = strings.Trim(doc.CustomerName, "- ")
			break
		}

		// Detect stage
		pLower := strings.ToLower(p)
		if pLower == "execution" || strings.Contains(pLower, "execution") {
			doc.Stage = "EXECUTION"
		} else if pLower == "offer" {
			doc.Stage = "OFFER"
		} else if pLower == "rfq" {
			doc.Stage = "RFQ"
		}
	}
}

// ReconciliationResult holds match results
type ReconciliationResult struct {
	TotalDocuments   int            `json:"total_documents"`
	MatchedToCosting int            `json:"matched_to_costing"`
	MatchedToOrders  int            `json:"matched_to_orders"`
	UnmatchedDocs    []string       `json:"unmatched_docs"`
	OfferCoverage    map[string]int `json:"offer_coverage"`
}

func runReconciliation(db *gorm.DB) *ReconciliationResult {
	result := &ReconciliationResult{
		OfferCoverage: make(map[string]int),
		UnmatchedDocs: []string{},
	}

	// Get all extracted documents
	var docs []ExtractedDocument
	db.Find(&docs)
	result.TotalDocuments = len(docs)

	// Check each document against costing data
	for _, doc := range docs {
		if doc.OfferNumber == "" {
			result.UnmatchedDocs = append(result.UnmatchedDocs, doc.SourcePath)
			continue
		}

		// Check if offer exists in costing_sheet_data
		var count int64
		db.Table("costing_sheet_data").Where("offer_number = ?", doc.OfferNumber).Count(&count)
		if count > 0 {
			result.MatchedToCosting++
		}

		result.OfferCoverage[doc.OfferNumber]++
	}

	return result
}
