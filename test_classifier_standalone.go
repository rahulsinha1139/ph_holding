// +build ignore

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Standalone test to validate document classifier logic without dependencies

type FilesystemDocClassification struct {
	FilePath     string
	FileName     string
	DocumentType string
	OfferNumber  string
	CustomerName string
	ProductType  string
	Stage        string
	ParentFolder string
	FileSize     int64
	ModTime      time.Time
	Extension    string
}

type FilesystemDocumentClassifierService struct {
	invoicePatterns        []*regexp.Regexp
	deliveryNotePatterns   []*regexp.Regexp
	customerPOPatterns     []*regexp.Regexp
	supplierPOAckPatterns  []*regexp.Regexp
	internalPOPatterns     []*regexp.Regexp
	rfqPatterns            []*regexp.Regexp
	costingPatterns        []*regexp.Regexp
	offerPatterns          []*regexp.Regexp
	shippingPatterns       []*regexp.Regexp
	technicalPatterns      []*regexp.Regexp
	offerNumberPattern     *regexp.Regexp
	productTypePattern     *regexp.Regexp
	albaPoPattern          *regexp.Regexp
	bapcoPoPattern         *regexp.Regexp
	ehOrderPattern         *regexp.Regexp
	phPoPattern            *regexp.Regexp
}

func NewFilesystemDocumentClassifierService() *FilesystemDocumentClassifierService {
	return &FilesystemDocumentClassifierService{
		invoicePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)invoice`),
			regexp.MustCompile(`(?i)\binv\b`),
			regexp.MustCompile(`(?i)^\w{3}-\d{2}\s+INV`),
			regexp.MustCompile(`(?i)INV-\d+`),
		},
		deliveryNotePatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)delivery`),
			regexp.MustCompile(`(?i)\bDN\b`),
			regexp.MustCompile(`(?i)delivery\s*note`),
		},
		customerPOPatterns: []*regexp.Regexp{
			regexp.MustCompile(`453\d{5}`),
			regexp.MustCompile(`PO_81_\d+`),
			regexp.MustCompile(`(?i)purchase\s*order`),
			regexp.MustCompile(`(?i)customer.*po`),
		},
		supplierPOAckPatterns: []*regexp.Regexp{
			regexp.MustCompile(`601\d{7}`),
			regexp.MustCompile(`(?i)order.*confirmation`),
			regexp.MustCompile(`(?i)po.*ack`),
		},
		internalPOPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)^PH\d{2}-\d+`),
			regexp.MustCompile(`(?i)PH.*purchase.*order`),
		},
		rfqPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)\brfq\b`),
			regexp.MustCompile(`(?i)request.*quote`),
			regexp.MustCompile(`(?i)quotation.*request`),
		},
		costingPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)costing`),
			regexp.MustCompile(`(?i)cost.*sheet`),
			regexp.MustCompile(`(?i)pricing.*sheet`),
		},
		offerPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)offer`),
			regexp.MustCompile(`(?i)quotation`),
			regexp.MustCompile(`(?i)proposal`),
			regexp.MustCompile(`(?i)commercial.*offer`),
		},
		shippingPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)shipping`),
			regexp.MustCompile(`(?i)shipment`),
			regexp.MustCompile(`(?i)packing.*list`),
			regexp.MustCompile(`(?i)bill.*lading`),
			regexp.MustCompile(`(?i)awb`),
		},
		technicalPatterns: []*regexp.Regexp{
			regexp.MustCompile(`(?i)technical`),
			regexp.MustCompile(`(?i)specification`),
			regexp.MustCompile(`(?i)datasheet`),
			regexp.MustCompile(`(?i)manual`),
			regexp.MustCompile(`(?i)msds`),
			regexp.MustCompile(`(?i)certificate`),
		},
		offerNumberPattern: regexp.MustCompile(`^(\d{3})\s+`),
		productTypePattern: regexp.MustCompile(`(?i)(AIT|FIT|LIT|PIT|TIT|SP|FT|LT|PT|TT|VALVE|TRANSMITTER|ANALYZER|FLOWMETER)`),
		albaPoPattern:      regexp.MustCompile(`453\d{5}`),
		bapcoPoPattern:     regexp.MustCompile(`PO_81_\d+`),
		ehOrderPattern:     regexp.MustCompile(`601\d{7}`),
		phPoPattern:        regexp.MustCompile(`PH\d{2}-\d+`),
	}
}

func (s *FilesystemDocumentClassifierService) extractOfferNumber(folderPath string) string {
	parts := strings.Split(folderPath, string(filepath.Separator))
	for _, part := range parts {
		if matches := s.offerNumberPattern.FindStringSubmatch(part); len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

func (s *FilesystemDocumentClassifierService) extractCustomerName(folderPath, fileName string) string {
	parts := strings.Split(folderPath, string(filepath.Separator))
	for _, part := range parts {
		if matches := s.offerNumberPattern.FindStringSubmatch(part); len(matches) > 0 {
			remainder := strings.TrimSpace(strings.TrimPrefix(part, matches[0]))
			customerParts := strings.Fields(remainder)
			if len(customerParts) > 0 {
				lastPart := customerParts[len(customerParts)-1]
				if s.productTypePattern.MatchString(lastPart) && len(customerParts) > 1 {
					return strings.Join(customerParts[:len(customerParts)-1], " ")
				}
				return remainder
			}
		}
		partUpper := strings.ToUpper(part)
		knownCustomers := []string{"ALBA", "BAPCO", "EWA", "ENGIE", "GPIC", "BANAGAS", "TATWEER"}
		for _, customer := range knownCustomers {
			if strings.Contains(partUpper, customer) {
				return customer
			}
		}
	}
	fileNameUpper := strings.ToUpper(fileName)
	knownCustomers := []string{"ALBA", "BAPCO", "EWA", "ENGIE", "GPIC", "BANAGAS", "TATWEER"}
	for _, customer := range knownCustomers {
		if strings.Contains(fileNameUpper, customer) {
			return customer
		}
	}
	return ""
}

func (s *FilesystemDocumentClassifierService) extractProductType(folderPath, fileName string) string {
	if matches := s.productTypePattern.FindStringSubmatch(folderPath); len(matches) > 0 {
		return strings.ToUpper(matches[1])
	}
	if matches := s.productTypePattern.FindStringSubmatch(fileName); len(matches) > 0 {
		return strings.ToUpper(matches[1])
	}
	return ""
}

func (s *FilesystemDocumentClassifierService) extractStage(folderPath string) string {
	folderPathUpper := strings.ToUpper(folderPath)
	if strings.Contains(folderPathUpper, "RFQ") {
		return "RFQ"
	}
	if strings.Contains(folderPathUpper, "OFFER") || strings.Contains(folderPathUpper, "QUOTATION") {
		return "OFFER"
	}
	if strings.Contains(folderPathUpper, "EXECUTION") || strings.Contains(folderPathUpper, "ORDER") ||
		strings.Contains(folderPathUpper, "SHIPMENT") || strings.Contains(folderPathUpper, "DELIVERY") {
		return "EXECUTION"
	}
	return ""
}

type mockFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return m.size }
func (m *mockFileInfo) Mode() os.FileMode  { return 0644 }
func (m *mockFileInfo) ModTime() time.Time { return m.modTime }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

func main() {
	service := NewFilesystemDocumentClassifierService()

	tests := []struct {
		fileName         string
		folderPath       string
		expectedCustomer string
		expectedOffer    string
		expectedStage    string
		expectedProduct  string
	}{
		{"45312345_ALBA_PO.pdf", "101 ALBA AIT/RFQ", "ALBA", "101", "RFQ", "AIT"},
		{"PO_81_1234.pdf", "102 BAPCO FIT/OFFER", "BAPCO", "102", "OFFER", "FIT"},
		{"60112345678.pdf", "103 EWA LIT/EXECUTION", "EWA", "103", "EXECUTION", "LIT"},
		{"PH25-045.pdf", "104 ENGIE SP/OFFER", "ENGIE", "104", "OFFER", "SP"},
		{"Costing Sheet v2.xlsx", "108 ALBA FLOWMETER/OFFER", "ALBA", "108", "OFFER", "FLOWMETER"},
	}

	fmt.Println("Testing Filesystem Document Classifier...")
	fmt.Println("==========================================")

	allPassed := true
	for i, tt := range tests {
		fmt.Printf("\nTest %d: %s\n", i+1, tt.fileName)
		fmt.Printf("  Folder: %s\n", tt.folderPath)

		offer := service.extractOfferNumber(tt.folderPath)
		customer := service.extractCustomerName(tt.folderPath, tt.fileName)
		product := service.extractProductType(tt.folderPath, tt.fileName)
		stage := service.extractStage(tt.folderPath)

		passed := true

		if offer != tt.expectedOffer {
			fmt.Printf("  ❌ Offer: got '%s', expected '%s'\n", offer, tt.expectedOffer)
			passed = false
			allPassed = false
		} else {
			fmt.Printf("  ✅ Offer: %s\n", offer)
		}

		if customer != tt.expectedCustomer {
			fmt.Printf("  ❌ Customer: got '%s', expected '%s'\n", customer, tt.expectedCustomer)
			passed = false
			allPassed = false
		} else {
			fmt.Printf("  ✅ Customer: %s\n", customer)
		}

		if product != tt.expectedProduct {
			fmt.Printf("  ❌ Product: got '%s', expected '%s'\n", product, tt.expectedProduct)
			passed = false
			allPassed = false
		} else {
			fmt.Printf("  ✅ Product: %s\n", product)
		}

		if stage != tt.expectedStage {
			fmt.Printf("  ❌ Stage: got '%s', expected '%s'\n", stage, tt.expectedStage)
			passed = false
			allPassed = false
		} else {
			fmt.Printf("  ✅ Stage: %s\n", stage)
		}

		if passed {
			fmt.Println("  ✅ PASSED")
		} else {
			fmt.Println("  ❌ FAILED")
		}
	}

	fmt.Println("\n==========================================")
	if allPassed {
		fmt.Println("✅ ALL TESTS PASSED!")
	} else {
		fmt.Println("❌ SOME TESTS FAILED")
		os.Exit(1)
	}
}
