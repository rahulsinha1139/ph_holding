// +build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
)

// ========================================================================
// SIMPLE DOCUMENT CLASSIFIER TEST (Standalone)
// ========================================================================
//
// Run: go run classifier_test_simple.go
// ========================================================================

// Simple ClassificationResult for testing
type ClassificationResult struct {
	DocumentType    string
	Confidence      float64
	Method          string
	KeywordsFound   []string
	RouteTo         string
	SuggestedAction string
	Explanation     string
}

// Simple DocumentClassifier for testing
type DocumentClassifier struct {
	invoicePatterns   []*regexp.Regexp
	rfqPatterns       []*regexp.Regexp
	quotationPatterns []*regexp.Regexp
	contractPatterns  []*regexp.Regexp
	poPatterns        []*regexp.Regexp
}

func NewDocumentClassifier() *DocumentClassifier {
	c := &DocumentClassifier{}

	// Invoice patterns
	c.invoicePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\binvoice\b`),
		regexp.MustCompile(`(?i)\bamount\s*due\b`),
		regexp.MustCompile(`(?i)\bpayment\s*terms?\b`),
	}

	// RFQ patterns
	c.rfqPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\brequest\s*for\s*quotation\b`),
		regexp.MustCompile(`(?i)\bRFQ\b`),
		regexp.MustCompile(`(?i)\bplease\s*quote\b`),
	}

	// Quotation patterns
	c.quotationPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bquotation\b`),
		regexp.MustCompile(`(?i)\bvalid\s*until\b`),
		regexp.MustCompile(`(?i)\bquoted\s*price\b`),
	}

	// Contract patterns
	c.contractPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bcontract\b`),
		regexp.MustCompile(`(?i)\bagreement\b`),
		regexp.MustCompile(`(?i)\bwhereas\b`),
	}

	// PO patterns
	c.poPatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\bpurchase\s*order\b`),
		regexp.MustCompile(`(?i)\bP\.?O\.?\s*#?\s*\d+`),
		regexp.MustCompile(`(?i)\bdelivery\s*date\b`),
	}

	return c
}

func (c *DocumentClassifier) Classify(text string, filename string) *ClassificationResult {
	// Try keyword-based classification
	if result := c.classifyByKeywords(text, filename); result != nil {
		return result
	}

	// Fallback to heuristics
	return c.classifyByHeuristics(text, filename)
}

func (c *DocumentClassifier) classifyByKeywords(text string, filename string) *ClassificationResult {
	lowerFilename := strings.ToLower(filename)

	candidates := []struct {
		docType  string
		patterns []*regexp.Regexp
		route    string
		action   string
	}{
		{"Invoice", c.invoicePatterns, "dashboard", "Link to dashboard and flag for reconciliation"},
		{"RFQ", c.rfqPatterns, "opportunities", "Create opportunity in Opportunities screen"},
		{"Quotation", c.quotationPatterns, "pricing", "Link to Pricing screen for review"},
		{"Contract", c.contractPatterns, "inbox", "Flag for contract review"},
		{"PO", c.poPatterns, "orders", "Create order in Orders screen"},
	}

	var bestMatch *ClassificationResult
	maxKeywords := 0

	for _, candidate := range candidates {
		keywordsFound := []string{}

		for _, pattern := range candidate.patterns {
			if matches := pattern.FindAllString(text, -1); len(matches) > 0 {
				for _, match := range matches {
					keywordsFound = append(keywordsFound, match)
				}
			}
		}

		for _, pattern := range candidate.patterns {
			if pattern.MatchString(lowerFilename) {
				keywordsFound = append(keywordsFound, "filename:"+candidate.docType)
			}
		}

		if len(keywordsFound) > maxKeywords {
			maxKeywords = len(keywordsFound)

			confidence := 0.5
			if maxKeywords >= 3 {
				confidence = 0.95
			} else if maxKeywords >= 1 {
				confidence = 0.75
			}

			bestMatch = &ClassificationResult{
				DocumentType:    candidate.docType,
				Confidence:      confidence,
				Method:          "keyword",
				KeywordsFound:   keywordsFound,
				RouteTo:         candidate.route,
				SuggestedAction: candidate.action,
				Explanation:     fmt.Sprintf("Found %d keywords matching %s", len(keywordsFound), candidate.docType),
			}
		}
	}

	if bestMatch != nil && maxKeywords >= 1 {
		return bestMatch
	}

	return nil
}

func (c *DocumentClassifier) classifyByHeuristics(text string, filename string) *ClassificationResult {
	lowerFilename := strings.ToLower(filename)

	if strings.Contains(lowerFilename, "invoice") {
		return &ClassificationResult{
			DocumentType:    "Invoice",
			Confidence:      0.6,
			Method:          "heuristic",
			RouteTo:         "dashboard",
			SuggestedAction: "Link to dashboard",
			Explanation:     "Filename contains 'invoice'",
		}
	}

	return &ClassificationResult{
		DocumentType:    "Other",
		Confidence:      0.3,
		Method:          "heuristic",
		RouteTo:         "inbox",
		SuggestedAction: "Send to Inbox for manual review",
		Explanation:     "No clear document type patterns found",
	}
}

func main() {
	log.Println("🧪 Document Classifier Test Suite")
	log.Println("==============================================================")
	log.Println()

	classifier := NewDocumentClassifier()

	testDocs := []struct {
		filename     string
		expectedType string
		filePath     string
	}{
		{"test_invoice.txt", "Invoice", "test_data/test_invoice.txt"},
		{"test_rfq.txt", "RFQ", "test_data/test_rfq.txt"},
		{"test_quotation.txt", "Quotation", "test_data/test_quotation.txt"},
		{"test_contract.txt", "Contract", "test_data/test_contract.txt"},
		{"test_po.txt", "PO", "test_data/test_po.txt"},
	}

	totalTests := len(testDocs)
	passedTests := 0

	for i, test := range testDocs {
		log.Printf("📄 Test %d: %s", i+1, test.filename)
		log.Printf("   Expected Type: %s", test.expectedType)

		content, err := os.ReadFile(test.filePath)
		if err != nil {
			log.Printf("   ❌ FAIL: Could not read file: %v", err)
			continue
		}

		result := classifier.Classify(string(content), test.filename)

		passed := result.DocumentType == test.expectedType
		if passed {
			passedTests++
			log.Printf("   ✅ PASS: Classified as %s (confidence: %.0f%%)", result.DocumentType, result.Confidence*100)
		} else {
			log.Printf("   ❌ FAIL: Classified as %s, expected %s (confidence: %.0f%%)",
				result.DocumentType, test.expectedType, result.Confidence*100)
		}

		log.Printf("   Method: %s", result.Method)
		log.Printf("   Keywords: %d found", len(result.KeywordsFound))
		log.Printf("   Routing: %s → %s", result.RouteTo, result.SuggestedAction)
		log.Println()
	}

	log.Println("==============================================================")
	log.Printf("📊 Test Results: %d/%d passed (%.0f%% accuracy)", passedTests, totalTests, float64(passedTests)/float64(totalTests)*100)
	log.Println()

	if passedTests == totalTests {
		log.Println("✅ All tests passed! Classifier is working correctly.")
	} else {
		log.Printf("⚠️  %d tests failed. Review classification logic.", totalTests-passedTests)
	}
}
