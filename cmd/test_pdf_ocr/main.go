package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image/png"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gen2brain/go-fitz"
)

// PDFOCRTest tests the OCR pipeline on a specific PDF
// Handles both vector and scanned PDFs
// Outputs results to .md file

const FlyOCREndpoint = "https://asymmetrica-runtime.fly.dev/api/ocr/pdf-upload"
const DebugMode = true // Log raw API responses

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <pdf_path>")
		fmt.Println("Example: go run main.go \"C:\\path\\to\\document.pdf\"")
		os.Exit(1)
	}

	pdfPath := os.Args[1]

	// Verify file exists
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		log.Fatalf("File not found: %s", pdfPath)
	}

	log.Printf("═══════════════════════════════════════════════════════════════")
	log.Printf("  PDF OCR TEST - %s", filepath.Base(pdfPath))
	log.Printf("═══════════════════════════════════════════════════════════════")

	startTime := time.Now()

	// Step 1: Analyze PDF type
	log.Println("\n[1] Analyzing PDF structure...")
	isVector, textSample := analyzePDF(pdfPath)

	if isVector {
		log.Printf("    ✓ VECTOR PDF detected (has extractable text)")
		log.Printf("    Sample: %.100s...", textSample)
	} else {
		log.Println("    → SCANNED PDF detected (image-based, no extractable text)")
	}

	var extractedText string
	var engine string
	var confidence float64

	if isVector {
		// Strategy 1: Local go-fitz extraction (FREE)
		log.Println("\n[2] Extracting text with go-fitz (local, free)...")
		text, err := extractVectorPDF(pdfPath)
		if err != nil {
			log.Printf("    ⚠ go-fitz failed: %v, falling back to Fly.io", err)
			isVector = false
		} else {
			extractedText = text
			engine = "go-fitz (local)"
			confidence = 1.0
			log.Printf("    ✓ Extracted %d characters", len(text))
		}
	}

	if !isVector {
		// Strategy 2: Extract images and send to Fly.io
		log.Println("\n[2] Extracting images from scanned PDF...")
		images, err := extractPDFImages(pdfPath)
		if err != nil {
			log.Fatalf("    ✗ Failed to extract images: %v", err)
		}
		log.Printf("    ✓ Extracted %d page images", len(images))

		log.Println("\n[3] Sending to Fly.io OCR endpoint...")
		text, err := callFlyOCR(pdfPath)
		if err != nil {
			log.Fatalf("    ✗ Fly.io OCR failed: %v", err)
		}
		extractedText = text
		engine = "Fly.io Runtime"
		confidence = 0.88
		log.Printf("    ✓ OCR complete, extracted %d characters", len(text))
	}

	// Step 3: Generate markdown output
	log.Println("\n[4] Generating markdown output...")

	outputPath := strings.TrimSuffix(pdfPath, filepath.Ext(pdfPath)) + "_ocr_output.md"

	mdContent := generateMarkdown(pdfPath, extractedText, engine, confidence, time.Since(startTime))

	if err := os.WriteFile(outputPath, []byte(mdContent), 0644); err != nil {
		log.Fatalf("    ✗ Failed to write markdown: %v", err)
	}

	log.Printf("    ✓ Output written to: %s", outputPath)

	// Summary
	elapsed := time.Since(startTime)
	log.Println("\n═══════════════════════════════════════════════════════════════")
	log.Printf("  COMPLETE in %v", elapsed)
	log.Printf("  Engine: %s", engine)
	log.Printf("  Confidence: %.0f%%", confidence*100)
	log.Printf("  Characters extracted: %d", len(extractedText))
	log.Printf("  Output: %s", outputPath)
	log.Println("═══════════════════════════════════════════════════════════════")
}

// analyzePDF checks if PDF has extractable text (vector) or is scanned (image-only)
func analyzePDF(pdfPath string) (bool, string) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return false, ""
	}
	defer doc.Close()

	if doc.NumPage() == 0 {
		return false, ""
	}

	// Check first page for text
	text, err := doc.Text(0)
	if err != nil {
		return false, ""
	}

	trimmed := strings.TrimSpace(text)
	// If >50 chars of actual text, it's a vector PDF
	return len(trimmed) > 50, trimmed
}

// extractVectorPDF extracts all text from a vector PDF
func extractVectorPDF(pdfPath string) (string, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	var fullText strings.Builder
	numPages := doc.NumPage()

	for i := 0; i < numPages; i++ {
		text, err := doc.Text(i)
		if err != nil {
			return "", fmt.Errorf("failed to extract page %d: %w", i+1, err)
		}
		if i > 0 {
			fullText.WriteString("\n\n--- Page " + fmt.Sprintf("%d", i+1) + " ---\n\n")
		}
		fullText.WriteString(text)
	}

	return strings.TrimSpace(fullText.String()), nil
}

// extractPDFImages renders PDF pages as images for OCR
func extractPDFImages(pdfPath string) ([][]byte, error) {
	doc, err := fitz.New(pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer doc.Close()

	var images [][]byte
	numPages := doc.NumPage()

	for i := 0; i < numPages; i++ {
		img, err := doc.Image(i)
		if err != nil {
			log.Printf("    ⚠ Failed to render page %d: %v", i+1, err)
			continue
		}

		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			log.Printf("    ⚠ Failed to encode page %d: %v", i+1, err)
			continue
		}

		images = append(images, buf.Bytes())
		log.Printf("    → Page %d: %d bytes", i+1, buf.Len())
	}

	return images, nil
}

// callFlyOCR sends PDF directly to Fly.io OCR endpoint
func callFlyOCR(pdfPath string) (string, error) {
	// Read PDF file
	fileBytes, err := os.ReadFile(pdfPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	// Encode to base64
	base64Content := base64.StdEncoding.EncodeToString(fileBytes)

	// Build request - use higher DPI for better scanned doc quality
	payload := map[string]interface{}{
		"base64Content": base64Content,
		"fileName":      filepath.Base(pdfPath),
		"maxPages":      10,
		"dpi":           300, // Bumped from 150 for better OCR accuracy
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	log.Printf("    → POST %s", FlyOCREndpoint)
	log.Printf("    → Payload size: %d bytes", len(jsonPayload))

	// Make HTTP request
	client := &http.Client{Timeout: 120 * time.Second}
	req, err := http.NewRequest("POST", FlyOCREndpoint, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Optional API key
	if apiKey := os.Getenv("ASYMM_API_KEY"); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	log.Printf("    → Response status: %d", resp.StatusCode)
	log.Printf("    → Response size: %d bytes", len(body))

	// Debug: dump raw response to file
	if DebugMode {
		debugPath := "ocr_debug_response.json"
		os.WriteFile(debugPath, body, 0644)
		log.Printf("    → Raw response saved to: %s", debugPath)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OCR API returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse response - capture all possible fields
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			RawText       string  `json:"raw_text"`
			Text          string  `json:"text"` // Alternative field name
			InvoiceNumber string  `json:"invoice_number"`
			InvoiceDate   string  `json:"invoice_date"`
			CustomerName  string  `json:"customer_name"`
			Total         float64 `json:"total"`
			VAT           float64 `json:"vat"`
			GrandTotal    float64 `json:"grand_total"`
			SubTotal      float64 `json:"sub_total"`
		} `json:"data"`
		Text      string                   `json:"text"`      // Top-level text field
		RawText   string                   `json:"raw_text"`  // Top-level raw_text
		Pages     []map[string]interface{} `json:"pages"`
		PageCount int                      `json:"page_count"`
		Error     string                   `json:"error"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w\nRaw: %s", err, string(body))
	}

	if !response.Success {
		return "", fmt.Errorf("OCR failed: %s", response.Error)
	}

	// Try multiple locations for the extracted text (API might return in different fields)
	var extractedText string

	// Priority 1: Data.RawText
	if response.Data.RawText != "" {
		extractedText = response.Data.RawText
		log.Printf("    → Using Data.RawText (%d chars)", len(extractedText))
	}

	// Priority 2: Data.Text
	if extractedText == "" && response.Data.Text != "" {
		extractedText = response.Data.Text
		log.Printf("    → Using Data.Text (%d chars)", len(extractedText))
	}

	// Priority 3: Top-level text
	if extractedText == "" && response.Text != "" {
		extractedText = response.Text
		log.Printf("    → Using top-level Text (%d chars)", len(extractedText))
	}

	// Priority 4: Top-level raw_text
	if extractedText == "" && response.RawText != "" {
		extractedText = response.RawText
		log.Printf("    → Using top-level RawText (%d chars)", len(extractedText))
	}

	// Priority 5: Concatenate page texts
	if extractedText == "" && len(response.Pages) > 0 {
		var fullText strings.Builder
		for i, page := range response.Pages {
			// Try multiple field names within pages
			for _, fieldName := range []string{"text", "raw_text", "content", "extracted_text"} {
				if text, ok := page[fieldName].(string); ok && text != "" {
					if fullText.Len() > 0 {
						fullText.WriteString("\n\n--- Page " + fmt.Sprintf("%d", i+1) + " ---\n\n")
					}
					fullText.WriteString(text)
					break
				}
			}
		}
		extractedText = fullText.String()
		log.Printf("    → Using concatenated pages (%d chars from %d pages)", len(extractedText), len(response.Pages))
	}

	if extractedText == "" {
		// Last resort: dump the entire response for debugging
		log.Println("    ⚠ No text found in standard fields, check ocr_debug_response.json")
		return "", fmt.Errorf("no text extracted - check ocr_debug_response.json for raw API response")
	}

	return extractedText, nil
}

// generateMarkdown creates a formatted markdown file from OCR results
func generateMarkdown(pdfPath, text, engine string, confidence float64, elapsed time.Duration) string {
	var md strings.Builder

	md.WriteString("# OCR Extraction Results\n\n")
	md.WriteString(fmt.Sprintf("**Source File:** `%s`\n\n", filepath.Base(pdfPath)))
	md.WriteString(fmt.Sprintf("**Processed:** %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	md.WriteString(fmt.Sprintf("**Engine:** %s\n\n", engine))
	md.WriteString(fmt.Sprintf("**Confidence:** %.0f%%\n\n", confidence*100))
	md.WriteString(fmt.Sprintf("**Processing Time:** %v\n\n", elapsed))
	md.WriteString(fmt.Sprintf("**Characters Extracted:** %d\n\n", len(text)))
	md.WriteString("---\n\n")
	md.WriteString("## Extracted Text\n\n")
	md.WriteString("```\n")
	md.WriteString(text)
	md.WriteString("\n```\n")

	return md.String()
}
