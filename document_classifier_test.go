package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFilesystemDocumentClassifier(t *testing.T) {
	service := NewFilesystemDocumentClassifierService()

	tests := []struct {
		name             string
		fileName         string
		folderPath       string
		expectedType     string
		expectedCustomer string
		expectedOffer    string
		expectedStage    string
		expectedProduct  string
	}{
		{
			name:             "ALBA PO",
			fileName:         "45312345_ALBA_PO.pdf",
			folderPath:       "101 ALBA AIT/RFQ",
			expectedType:     "CUSTOMER_PO",
			expectedCustomer: "ALBA",
			expectedOffer:    "101",
			expectedStage:    "RFQ",
			expectedProduct:  "AIT",
		},
		{
			name:             "BAPCO PO",
			fileName:         "PO_81_1234.pdf",
			folderPath:       "102 BAPCO FIT/OFFER",
			expectedType:     "CUSTOMER_PO",
			expectedCustomer: "BAPCO",
			expectedOffer:    "102",
			expectedStage:    "OFFER",
			expectedProduct:  "FIT",
		},
		{
			name:             "E+H Order Confirmation",
			fileName:         "60112345678.pdf",
			folderPath:       "103 EWA LIT/EXECUTION",
			expectedType:     "SUPPLIER_PO_ACK",
			expectedCustomer: "EWA",
			expectedOffer:    "103",
			expectedStage:    "EXECUTION",
			expectedProduct:  "LIT",
		},
		{
			name:             "PH Internal PO",
			fileName:         "PH25-045.pdf",
			folderPath:       "104 ENGIE SP/OFFER",
			expectedType:     "INTERNAL_PO",
			expectedCustomer: "ENGIE",
			expectedOffer:    "104",
			expectedStage:    "OFFER",
			expectedProduct:  "SP",
		},
		{
			name:             "Invoice",
			fileName:         "PH-25 INV 123.pdf",
			folderPath:       "105 GPIC VALVE/EXECUTION",
			expectedType:     "INVOICE",
			expectedCustomer: "GPIC",
			expectedOffer:    "105",
			expectedStage:    "EXECUTION",
			expectedProduct:  "VALVE",
		},
		{
			name:             "RFQ Email",
			fileName:         "RFQ from customer.msg",
			folderPath:       "106 BANAGAS TRANSMITTER/RFQ",
			expectedType:     "RFQ_EMAIL",
			expectedCustomer: "BANAGAS",
			expectedOffer:    "106",
			expectedStage:    "RFQ",
			expectedProduct:  "TRANSMITTER",
		},
		{
			name:             "RFQ Document",
			fileName:         "RFQ_technical_specs.pdf",
			folderPath:       "107 TATWEER ANALYZER/RFQ",
			expectedType:     "RFQ_DOCUMENT",
			expectedCustomer: "TATWEER",
			expectedOffer:    "107",
			expectedStage:    "RFQ",
			expectedProduct:  "ANALYZER",
		},
		{
			name:             "Costing Sheet",
			fileName:         "Costing Sheet v2.xlsx",
			folderPath:       "108 ALBA FLOWMETER/OFFER",
			expectedType:     "COSTING_SHEET",
			expectedCustomer: "ALBA",
			expectedOffer:    "108",
			expectedStage:    "OFFER",
			expectedProduct:  "FLOWMETER",
		},
		{
			name:             "Commercial Offer",
			fileName:         "Commercial Offer Rev 3.pdf",
			folderPath:       "109 BAPCO PIT/OFFER",
			expectedType:     "COMMERCIAL_OFFER",
			expectedCustomer: "BAPCO",
			expectedOffer:    "109",
			expectedStage:    "OFFER",
			expectedProduct:  "PIT",
		},
		{
			name:             "Delivery Note",
			fileName:         "DN_12345.pdf",
			folderPath:       "110 EWA TIT/EXECUTION",
			expectedType:     "DELIVERY_NOTE",
			expectedCustomer: "EWA",
			expectedOffer:    "110",
			expectedStage:    "EXECUTION",
			expectedProduct:  "TIT",
		},
		{
			name:             "Shipping Document",
			fileName:         "Packing List.pdf",
			folderPath:       "111 ENGIE FT/EXECUTION/Shipment",
			expectedType:     "SHIPPING_DOC",
			expectedCustomer: "ENGIE",
			expectedOffer:    "111",
			expectedStage:    "EXECUTION",
			expectedProduct:  "FT",
		},
		{
			name:             "Technical Document",
			fileName:         "Technical Datasheet.pdf",
			folderPath:       "112 GPIC LT/RFQ",
			expectedType:     "TECHNICAL_DOC",
			expectedCustomer: "GPIC",
			expectedOffer:    "112",
			expectedStage:    "RFQ",
			expectedProduct:  "LT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create mock file info
			info := &mockFileInfo{
				name:    tt.fileName,
				size:    1024,
				modTime: time.Now(),
			}

			// Classify document
			doc := service.classifyDocument(
				filepath.Join("C:\\test", tt.folderPath, tt.fileName),
				info,
				"C:\\test",
			)

			// Verify document type
			if doc.DocumentType != tt.expectedType {
				t.Errorf("Expected type %s, got %s", tt.expectedType, doc.DocumentType)
			}

			// Verify customer name
			if tt.expectedCustomer != "" && doc.CustomerName != tt.expectedCustomer {
				t.Errorf("Expected customer %s, got %s", tt.expectedCustomer, doc.CustomerName)
			}

			// Verify offer number
			if tt.expectedOffer != "" && doc.OfferNumber != tt.expectedOffer {
				t.Errorf("Expected offer %s, got %s", tt.expectedOffer, doc.OfferNumber)
			}

			// Verify stage
			if tt.expectedStage != "" && doc.Stage != tt.expectedStage {
				t.Errorf("Expected stage %s, got %s", tt.expectedStage, doc.Stage)
			}

			// Verify product type
			if tt.expectedProduct != "" && doc.ProductType != tt.expectedProduct {
				t.Errorf("Expected product %s, got %s", tt.expectedProduct, doc.ProductType)
			}
		})
	}
}

func TestExtractOfferNumber(t *testing.T) {
	service := NewFilesystemDocumentClassifierService()

	tests := []struct {
		folderPath string
		expected   string
	}{
		{"101 ALBA AIT", "101"},
		{"102 BAPCO FIT", "102"},
		{"subfolder/103 EWA LIT/RFQ", "103"},
		{"no-number-here", ""},
		{"999 GPIC VALVE/OFFER/Documents", "999"},
	}

	for _, tt := range tests {
		result := service.extractOfferNumber(tt.folderPath)
		if result != tt.expected {
			t.Errorf("extractOfferNumber(%s) = %s, want %s", tt.folderPath, result, tt.expected)
		}
	}
}

func TestExtractCustomerName(t *testing.T) {
	service := NewFilesystemDocumentClassifierService()

	tests := []struct {
		folderPath string
		fileName   string
		expected   string
	}{
		{"101 ALBA AIT", "invoice.pdf", "ALBA"},
		{"102 BAPCO FIT", "quote.pdf", "BAPCO"},
		{"103 EWA LIT", "po.pdf", "EWA"},
		{"104 ENGIE SP", "rfq.pdf", "ENGIE"},
		{"105 GPIC VALVE", "offer.pdf", "GPIC"},
		{"106 BANAGAS TRANSMITTER", "costing.xlsx", "BANAGAS"},
		{"107 TATWEER ANALYZER", "dn.pdf", "TATWEER"},
		{"no-customer-folder", "ALBA_invoice.pdf", "ALBA"},
		{"generic-folder", "BAPCO_PO.pdf", "BAPCO"},
	}

	for _, tt := range tests {
		result := service.extractCustomerName(tt.folderPath, tt.fileName)
		if result != tt.expected {
			t.Errorf("extractCustomerName(%s, %s) = %s, want %s",
				tt.folderPath, tt.fileName, result, tt.expected)
		}
	}
}

func TestExtractProductType(t *testing.T) {
	service := NewFilesystemDocumentClassifierService()

	tests := []struct {
		folderPath string
		fileName   string
		expected   string
	}{
		{"101 ALBA AIT", "doc.pdf", "AIT"},
		{"102 BAPCO FIT", "doc.pdf", "FIT"},
		{"103 EWA LIT", "doc.pdf", "LIT"},
		{"104 ENGIE SP", "doc.pdf", "SP"},
		{"105 GPIC VALVE", "doc.pdf", "VALVE"},
		{"106 BANAGAS TRANSMITTER", "doc.pdf", "TRANSMITTER"},
		{"107 TATWEER ANALYZER", "doc.pdf", "ANALYZER"},
		{"108 ALBA FLOWMETER", "doc.pdf", "FLOWMETER"},
		{"generic-folder", "AIT_datasheet.pdf", "AIT"},
		{"generic-folder", "flowmeter_specs.pdf", "FLOWMETER"},
	}

	for _, tt := range tests {
		result := service.extractProductType(tt.folderPath, tt.fileName)
		if result != tt.expected {
			t.Errorf("extractProductType(%s, %s) = %s, want %s",
				tt.folderPath, tt.fileName, result, tt.expected)
		}
	}
}

func TestExtractStage(t *testing.T) {
	service := NewFilesystemDocumentClassifierService()

	tests := []struct {
		folderPath string
		expected   string
	}{
		{"101 ALBA AIT/RFQ", "RFQ"},
		{"102 BAPCO FIT/OFFER", "OFFER"},
		{"103 EWA LIT/EXECUTION", "EXECUTION"},
		{"104 ENGIE SP/QUOTATION", "OFFER"},
		{"105 GPIC VALVE/ORDER", "EXECUTION"},
		{"106 BANAGAS TRANSMITTER/SHIPMENT", "EXECUTION"},
		{"no-stage-folder", ""},
	}

	for _, tt := range tests {
		result := service.extractStage(tt.folderPath)
		if result != tt.expected {
			t.Errorf("extractStage(%s) = %s, want %s", tt.folderPath, result, tt.expected)
		}
	}
}

// mockFileInfo implements os.FileInfo for testing
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
