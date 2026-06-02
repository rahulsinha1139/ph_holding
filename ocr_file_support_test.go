package main

import (
	"slices"
	"strings"
	"testing"
)

func TestSupportedOCRFileExtensionsCoverage(t *testing.T) {
	expected := []string{
		".pdf", ".xlsx", ".xls", ".docx", ".rtf", ".msg", ".eml",
		".png", ".jpg", ".jpeg", ".bmp", ".tiff", ".tif", ".webp",
	}

	got := supportedOCRFileExtensions()
	for _, ext := range expected {
		if !slices.Contains(got, ext) {
			t.Fatalf("missing OCR extension %s in %v", ext, got)
		}
	}
}

func TestSupportedOCRWatcherExtensionsCoverage(t *testing.T) {
	got := supportedOCRWatcherExtensions()
	for _, ext := range supportedOCRFileExtensions() {
		if !slices.Contains(got, ext) {
			t.Fatalf("watcher is missing OCR extension %s", ext)
		}
	}
	if !slices.Contains(got, ".xml") {
		t.Fatal("watcher should still include .xml")
	}
}

func TestSupportedOCRFileDialogPattern(t *testing.T) {
	pattern := supportedOCRFileDialogPattern()
	for _, ext := range supportedOCRFileExtensions() {
		if !strings.Contains(pattern, "*"+ext) {
			t.Fatalf("dialog pattern %q missing %s", pattern, ext)
		}
	}
}

func TestRuntimeDocumentTypeDetectionCoversOCRFormats(t *testing.T) {
	tests := map[string]string{
		"customer_invoice.pdf": "invoice",
		"rfq_2026.pdf":         "rfq",
		"supplier_po.pdf":      "purchase_order",
		"quotation.msg":        "email",
		"rfq.xlsx":             "spreadsheet",
		"rfq.xls":              "spreadsheet",
		"terms.docx":           "document",
		"terms.rtf":            "document",
		"scan.png":             "image",
		"scan.jpg":             "image",
		"scan.jpeg":            "image",
		"scan.tiff":            "image",
	}

	for path, expected := range tests {
		if actual := detectDocumentType(path); actual != expected {
			t.Fatalf("detectDocumentType(%q) = %q, want %q", path, actual, expected)
		}
	}
}
