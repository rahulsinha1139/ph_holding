package main

import (
	"archive/zip"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xuri/excelize/v2"
)

// Test vector PDF detection
func TestIsVectorPDF(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"Non-PDF file", "document.txt", false},
		{"Image file", "scan.png", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isVectorPDF(tt.path)
			if result != tt.expected {
				t.Errorf("isVectorPDF(%s) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

// Test SimpleOCRService creation
func TestNewSimpleOCRService(t *testing.T) {
	service, err := NewSimpleOCRService()
	if err != nil {
		t.Fatalf("NewSimpleOCRService() failed: %v", err)
	}

	if service == nil {
		t.Fatal("NewSimpleOCRService() returned nil service")
	}

	if service.flyEndpoint == "" {
		t.Error("flyEndpoint should not be empty")
	}

	if service.httpClient == nil {
		t.Error("httpClient should not be nil")
	}

	if service.maxPages <= 0 {
		t.Error("maxPages should be > 0")
	}

	if service.dpi <= 0 {
		t.Error("dpi should be > 0")
	}

	// Verify default endpoint
	expectedEndpoint := "https://asymmetrica-runtime.fly.dev"
	if service.flyEndpoint != expectedEndpoint {
		t.Errorf("Expected endpoint %s, got %s", expectedEndpoint, service.flyEndpoint)
	}
}

// Test OCRResultSimple structure
func TestOCRResultSimple(t *testing.T) {
	result := &OCRResultSimple{
		Success:        true,
		Text:           "Sample extracted text",
		Confidence:     0.95,
		DocumentType:   "invoice",
		ExtractedData:  map[string]interface{}{"raw_text": "Sample extracted text"},
		ProcessingTime: 1500,
		Engine:         "pymupdf",
	}

	if !result.Success {
		t.Error("Result should be successful")
	}

	if result.Text == "" {
		t.Error("Text should not be empty")
	}

	if result.Confidence <= 0 || result.Confidence > 1.0 {
		t.Errorf("Confidence should be between 0 and 1, got: %f", result.Confidence)
	}

	if result.Engine != "pymupdf" && result.Engine != "fly-runtime" {
		t.Errorf("Unexpected engine: %s", result.Engine)
	}
}

func TestSimpleOCRServiceProcessesExcelLocally(t *testing.T) {
	tmpDir := t.TempDir()
	workbookPath := filepath.Join(tmpDir, "rfq_fixture.xlsx")

	f := excelize.NewFile()
	sheet := f.GetSheetName(0)
	if err := f.SetCellValue(sheet, "A1", "Request for Quotation"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "A2", "RFQ Number"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "B2", "RFQ-2026-LOCAL"); err != nil {
		t.Fatal(err)
	}
	if err := f.SetCellValue(sheet, "A3", "Please quote analyzer spares"); err != nil {
		t.Fatal(err)
	}
	if err := f.SaveAs(workbookPath); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	service, err := NewSimpleOCRService()
	if err != nil {
		t.Fatalf("NewSimpleOCRService() failed: %v", err)
	}

	result, err := service.ProcessDocument(workbookPath, "rfq")
	if err != nil {
		t.Fatalf("ProcessDocument() failed: %v", err)
	}
	if !result.Success {
		t.Fatal("Excel OCR result should be successful")
	}
	if result.Engine != "excelize" {
		t.Fatalf("expected excelize engine, got %s", result.Engine)
	}
	if result.DocumentType != "rfq" {
		t.Fatalf("expected rfq document type, got %s", result.DocumentType)
	}
	if !strings.Contains(result.Text, "Request for Quotation") {
		t.Fatalf("expected extracted workbook text, got %q", result.Text)
	}
	if result.ExtractedData["source_type"] != "excel" {
		t.Fatalf("expected source_type=excel, got %#v", result.ExtractedData["source_type"])
	}
}

func TestSimpleOCRServiceProcessesLocalOfficeAndEmailFormats(t *testing.T) {
	tmpDir := t.TempDir()
	service, err := NewSimpleOCRService()
	if err != nil {
		t.Fatalf("NewSimpleOCRService() failed: %v", err)
	}

	fixtures := []struct {
		name       string
		path       string
		write      func(string) error
		wantEngine string
		wantText   string
	}{
		{
			name: "docx",
			path: filepath.Join(tmpDir, "rfq.docx"),
			write: func(path string) error {
				return writeMinimalDOCX(path, "Request for Quotation RFQ-DOCX-1 Please quote analyzer spares")
			},
			wantEngine: "docx-parser",
			wantText:   "Request for Quotation",
		},
		{
			name: "rtf",
			path: filepath.Join(tmpDir, "supplier_invoice.rtf"),
			write: func(path string) error {
				return os.WriteFile(path, []byte(`{\rtf1\ansi Supplier Invoice INV-RTF-1\par Total 125.000 BHD}`), 0644)
			},
			wantEngine: "rtf-parser",
			wantText:   "Supplier Invoice",
		},
		{
			name: "eml",
			path: filepath.Join(tmpDir, "rfq.eml"),
			write: func(path string) error {
				return os.WriteFile(path, []byte("From: buyer@example.com\r\nTo: sales@phtrading.com\r\nSubject: RFQ EML 1\r\n\r\nRequest for Quotation. Please quote pressure transmitter spares."), 0644)
			},
			wantEngine: "eml-parser",
			wantText:   "Request for Quotation",
		},
	}

	for _, tt := range fixtures {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.write(tt.path); err != nil {
				t.Fatalf("write fixture failed: %v", err)
			}
			result, err := service.ProcessDocument(tt.path, "auto")
			if err != nil {
				t.Fatalf("ProcessDocument() failed: %v", err)
			}
			if !result.Success {
				t.Fatal("OCR result should be successful")
			}
			if result.Engine != tt.wantEngine {
				t.Fatalf("expected engine %s, got %s", tt.wantEngine, result.Engine)
			}
			if !strings.Contains(result.Text, tt.wantText) {
				t.Fatalf("expected text containing %q, got %q", tt.wantText, result.Text)
			}
		})
	}
}

func writeMinimalDOCX(path string, text string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	archive := zip.NewWriter(file)
	defer archive.Close()

	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		"word/document.xml": fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body><w:p><w:r><w:t>%s</w:t></w:r></w:p></w:body>
</w:document>`, html.EscapeString(text)),
	}

	for name, content := range entries {
		writer, err := archive.Create(name)
		if err != nil {
			return err
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			return err
		}
	}
	return nil
}

// Benchmark vector PDF detection (should be fast!)
func BenchmarkIsVectorPDF(b *testing.B) {
	// Test with a non-PDF file (fast path)
	path := "document.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		isVectorPDF(path)
	}
}
