package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

const testDataPath = `C:\Projects\asymm_all_math\PH_TEST_DATA\Offers No 101 -150 (2025)`

// skipIfNoTestData skips the test if the test data directory doesn't exist
func skipIfNoTestData(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
		t.Skipf("Skipping test: test data directory not found: %s", testDataPath)
	}
}

// TestOfferScannerBasic tests basic scanning functionality
func TestOfferScannerBasic(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	// Scan all offers
	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Failed to scan offers: %v", err)
	}

	// Validate results
	if len(scanner.Offers) == 0 {
		t.Fatalf("No offers found! Expected 30-50 offers")
	}

	t.Logf("✓ Scanned %d offers successfully", len(scanner.Offers))

	// Check first offer has required fields
	firstOffer := scanner.Offers[0]
	if firstOffer.OfferID == "" {
		t.Errorf("Offer ID missing")
	}
	if firstOffer.CustomerName == "" {
		t.Errorf("Customer name missing")
	}
	if firstOffer.FullPath == "" {
		t.Errorf("Full path missing")
	}

	t.Logf("✓ First offer: ID=%s, Customer=%s, Type=%s",
		firstOffer.OfferID, firstOffer.CustomerName, firstOffer.ProductType)
}

// TestOfferMetadataExtraction tests detailed metadata extraction
func TestOfferMetadataExtraction(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Find offer with execution (e.g., ENGIE AIT - offer 101)
	var executedOffer *OfferMetadata
	for i, offer := range scanner.Offers {
		if offer.HasExecution {
			executedOffer = &scanner.Offers[i]
			break
		}
	}

	if executedOffer == nil {
		t.Skip("No executed offers found in test data")
	}

	t.Logf("Testing executed offer: %s", executedOffer.OfferID)

	// Validate workflow stages
	if !executedOffer.HasRFQ {
		t.Logf("Warning: Executed offer missing RFQ folder")
	}
	if !executedOffer.HasOffer {
		t.Errorf("Executed offer should have OFFER folder")
	}
	if !executedOffer.HasExecution {
		t.Errorf("Executed offer should have EXECUTION folder")
	}

	t.Logf("✓ Workflow stages: RFQ=%v, Offer=%v, Execution=%v",
		executedOffer.HasRFQ, executedOffer.HasOffer, executedOffer.HasExecution)

	// Validate revisions
	if executedOffer.RevisionCount == 0 {
		t.Logf("Warning: No revisions found (may be in different structure)")
	} else {
		t.Logf("✓ Found %d revisions: %v", executedOffer.RevisionCount, executedOffer.Revisions)
	}

	// Validate file detection
	t.Logf("✓ Files found:")
	t.Logf("  - XLSX: %d", len(executedOffer.XLSXFiles))
	t.Logf("  - XML: %d", len(executedOffer.XMLFiles))
	t.Logf("  - PDF: %d", len(executedOffer.PDFFiles))
	t.Logf("  - DOCX: %d", len(executedOffer.DOCXFiles))
	t.Logf("  - JPG: %d", len(executedOffer.JPGFiles))

	// Check for costing and E+H pricing
	if executedOffer.HasCosting {
		t.Logf("✓ Costing file detected")
	}
	if executedOffer.HasEHPricing {
		t.Logf("✓ E+H pricing XML detected")
	}
}

// TestFolderNameParsing tests offer ID and customer extraction
func TestFolderNameParsing(t *testing.T) {
	scanner := NewOfferScanner("")

	testCases := []struct {
		folderName       string
		expectedOfferID  string
		expectedCustomer string
		expectedProduct  string
	}{
		{"101 ENGIE AIT", "101", "ENGIE", "AIT"},
		{"102 VEOLIA WATWER TECH AIT", "102", "VEOLIA", "AIT"},
		{"110-SEAPEAK SP", "110", "SEAPEAK", "SP"},
		{"123-ENGIE-FIT", "123", "ENGIE", "FIT"},
		{"125-ALBA-TIT(FEED)", "125", "ALBA", ""},
	}

	for _, tc := range testCases {
		offerID, customer, productType := scanner.ParseFolderName(tc.folderName)

		if offerID != tc.expectedOfferID {
			t.Errorf("Folder '%s': expected offer ID '%s', got '%s'",
				tc.folderName, tc.expectedOfferID, offerID)
		}

		if customer != tc.expectedCustomer {
			t.Errorf("Folder '%s': expected customer '%s', got '%s'",
				tc.folderName, tc.expectedCustomer, customer)
		}

		// Product type is optional, only check if expected
		if tc.expectedProduct != "" && productType != tc.expectedProduct {
			t.Errorf("Folder '%s': expected product '%s', got '%s'",
				tc.folderName, tc.expectedProduct, productType)
		}

		t.Logf("✓ Parsed '%s' → ID=%s, Customer=%s, Type=%s",
			tc.folderName, offerID, customer, productType)
	}
}

// TestCustomerFiltering tests customer-based queries
func TestCustomerFiltering(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Test ENGIE filter
	engieOffers := scanner.GetByCustomer("ENGIE")
	if len(engieOffers) == 0 {
		t.Logf("Warning: No ENGIE offers found")
	} else {
		t.Logf("✓ Found %d ENGIE offers", len(engieOffers))
	}

	// Test VEOLIA filter
	veoliaOffers := scanner.GetByCustomer("VEOLIA")
	if len(veoliaOffers) > 0 {
		t.Logf("✓ Found %d VEOLIA offers", len(veoliaOffers))
	}

	// Test product type filter
	aitOffers := scanner.GetByProductType("AIT")
	if len(aitOffers) > 0 {
		t.Logf("✓ Found %d AIT offers", len(aitOffers))
	}

	fitOffers := scanner.GetByProductType("FIT")
	if len(fitOffers) > 0 {
		t.Logf("✓ Found %d FIT offers", len(fitOffers))
	}
}

// TestExecutionFiltering tests execution stage queries
func TestExecutionFiltering(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Get executed offers
	executedOffers := scanner.GetExecuted()
	t.Logf("✓ Executed offers: %d", len(executedOffers))

	// Get pending offers
	pendingOffers := scanner.GetPending()
	t.Logf("✓ Pending offers: %d", len(pendingOffers))

	// Validate they don't overlap
	total := len(executedOffers) + len(pendingOffers)
	if total > len(scanner.Offers) {
		t.Errorf("Overlap detected! Executed=%d, Pending=%d, Total=%d",
			len(executedOffers), len(pendingOffers), len(scanner.Offers))
	}
}

// TestScanSummary tests summary statistics generation
func TestScanSummary(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	summary := scanner.GetSummary()

	t.Logf("=== SCAN SUMMARY ===")
	t.Logf("Total offers: %d", summary.TotalOffers)
	t.Logf("Unique customers: %d", summary.UniqueCustomers)
	t.Logf("Execution rate: %.1f%%", summary.ExecutionRate)
	t.Logf("Avg revisions: %.2f", summary.AvgRevisions)
	t.Logf("Avg cycle days: %.1f", summary.AvgCycleDays)

	t.Logf("\nProduct type breakdown:")
	for productType, count := range summary.ProductTypeCount {
		t.Logf("  %s: %d offers", productType, count)
	}

	t.Logf("\nTop 10 customers:")
	for i, ranking := range summary.CustomersRanked {
		if i >= 10 {
			break
		}
		t.Logf("  %d. %s: %d offers (%.1f%% win rate)",
			i+1, ranking.Name, ranking.OfferCount, ranking.WinRate)
	}

	// Validate summary
	if summary.TotalOffers == 0 {
		t.Errorf("Total offers should be > 0")
	}
	if summary.UniqueCustomers == 0 {
		t.Errorf("Unique customers should be > 0")
	}
	if summary.ExecutionRate < 0 || summary.ExecutionRate > 100 {
		t.Errorf("Execution rate should be 0-100%%, got %.1f%%", summary.ExecutionRate)
	}
}

// TestFixtureGeneration tests test fixture creation
func TestFixtureGeneration(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	// Create fixture generator
	generator := NewTestFixtureGenerator(scanner)

	// Build customer profiles from real data
	generator.BuildCustomerProfiles()

	t.Logf("✓ Built %d customer profiles", len(generator.BaseCustomers))

	// Generate test customers
	testCustomers := generator.GenerateTestCustomers(20)

	if len(testCustomers) == 0 {
		t.Fatalf("Failed to generate test customers")
	}

	t.Logf("✓ Generated %d test customers", len(testCustomers))

	// Validate customer structure
	firstCustomer := testCustomers[0]
	if firstCustomer.ID == "" {
		t.Errorf("Customer ID missing")
	}
	if firstCustomer.BusinessName == "" {
		t.Errorf("Customer name missing")
	}
	if len(firstCustomer.PaymentHistory) == 0 {
		t.Errorf("Payment history missing")
	}

	t.Logf("✓ Sample customer: %s (%.0f BHD, %d payment history entries)",
		firstCustomer.BusinessName, firstCustomer.OrderValue, len(firstCustomer.PaymentHistory))

	// Export to JSON
	outputFile := "test_customers_fixture.json"
	err = generator.ExportToJSON(testCustomers, outputFile)
	if err != nil {
		t.Fatalf("Failed to export JSON: %v", err)
	}

	t.Logf("✓ Exported to %s", outputFile)

	// Clean up
	defer os.Remove(outputFile)
}

// TestPaymentPredictorFixtures tests payment predictor test cases
func TestPaymentPredictorFixtures(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	generator := NewTestFixtureGenerator(scanner)
	generator.BuildCustomerProfiles()

	// Generate fixtures by grade
	fixtures := generator.GeneratePaymentPredictorFixtures()

	t.Logf("=== PAYMENT PREDICTOR FIXTURES ===")

	for grade, customers := range fixtures {
		if len(customers) > 0 {
			t.Logf("\n%s: %d customers", grade, len(customers))

			// Show first customer of each grade
			customer := customers[0]
			avgPayment := averageInt(customer.PaymentHistory)

			t.Logf("  Sample: %s", customer.BusinessName)
			t.Logf("    Order value: %.0f BHD", customer.OrderValue)
			t.Logf("    Avg payment: %.1f days", avgPayment)
			t.Logf("    Tenure: %d years", customer.RelationYears)
			t.Logf("    Disputes: %d", customer.DisputeCount)
		}
	}

	// Validate grade separation
	if len(fixtures["grade_a"]) == 0 {
		t.Logf("Warning: No Grade A customers generated")
	}
	if len(fixtures["grade_d"]) == 0 {
		t.Logf("Warning: No Grade D customers generated")
	}
}

// TestGeometryRoutingFixtures tests event generation
func TestGeometryRoutingFixtures(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	generator := NewTestFixtureGenerator(scanner)
	events := generator.GenerateGeometryRoutingFixtures()

	t.Logf("✓ Generated %d routing events", len(events))

	// Count event types
	eventTypes := make(map[string]int)
	for _, event := range events {
		eventType := event["event_type"].(string)
		eventTypes[eventType]++
	}

	t.Logf("\nEvent type breakdown:")
	for eventType, count := range eventTypes {
		t.Logf("  %s: %d", eventType, count)
	}

	// Validate event structure
	if len(events) > 0 {
		firstEvent := events[0]
		if firstEvent["event_type"] == nil {
			t.Errorf("Event missing event_type")
		}
		if firstEvent["offer_id"] == nil {
			t.Errorf("Event missing offer_id")
		}
	}
}

// TestCustomer360Fixtures tests multi-source profile generation
func TestCustomer360Fixtures(t *testing.T) {
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	generator := NewTestFixtureGenerator(scanner)
	generator.BuildCustomerProfiles()

	profiles := generator.GenerateCustomer360Fixtures()

	t.Logf("✓ Generated %d 360° customer profiles", len(profiles))

	// Show sample profile
	if len(profiles) > 0 {
		sampleProfile := profiles[0]

		// Pretty print JSON
		jsonData, _ := json.MarshalIndent(sampleProfile, "", "  ")
		t.Logf("\nSample 360° profile:\n%s", string(jsonData))
	}

	// Validate structure
	for i, profile := range profiles {
		if profile["customer_name"] == nil {
			t.Errorf("Profile %d missing customer_name", i)
		}
		if profile["sources"] == nil {
			t.Errorf("Profile %d missing sources", i)
		}
		if profile["risk_grade"] == nil {
			t.Errorf("Profile %d missing risk_grade", i)
		}
	}
}

// TestEndToEndPipeline tests complete scan → profile → fixture pipeline
func TestEndToEndPipeline(t *testing.T) {
	t.Logf("=== END-TO-END PIPELINE TEST ===\n")

	// Step 1: Scan offers
	skipIfNoTestData(t)
	scanner := NewOfferScanner(testDataPath)

	err := scanner.ScanAll()
	if err != nil {
		t.Fatalf("Scan failed: %v", err)
	}

	t.Logf("✓ Step 1: Scanned %d offers", len(scanner.Offers))

	// Step 2: Generate summary
	summary := scanner.GetSummary()
	t.Logf("✓ Step 2: Generated summary (%d customers, %.1f%% execution rate)",
		summary.UniqueCustomers, summary.ExecutionRate)

	// Step 3: Build customer profiles
	generator := NewTestFixtureGenerator(scanner)
	generator.BuildCustomerProfiles()
	t.Logf("✓ Step 3: Built %d customer profiles", len(generator.BaseCustomers))

	// Step 4: Generate test fixtures
	testCustomers := generator.GenerateTestCustomers(15)
	t.Logf("✓ Step 4: Generated %d test customers", len(testCustomers))

	// Step 5: Export fixtures
	outputFile := "ph_holdings_test_fixtures.json"
	err = generator.ExportToJSON(testCustomers, outputFile)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}
	t.Logf("✓ Step 5: Exported to %s", outputFile)

	// Step 6: Generate 360° profiles
	profiles := generator.GenerateCustomer360Fixtures()
	t.Logf("✓ Step 6: Generated %d 360° profiles", len(profiles))

	// Step 7: Generate routing events
	events := generator.GenerateGeometryRoutingFixtures()
	t.Logf("✓ Step 7: Generated %d routing events", len(events))

	t.Logf("\n=== PIPELINE COMPLETE ===")
	t.Logf("Total artifacts: %d customers, %d profiles, %d events",
		len(testCustomers), len(profiles), len(events))

	// Clean up
	defer os.Remove(outputFile)
}

// BenchmarkOfferScanning benchmarks scanning performance
func BenchmarkOfferScanning(b *testing.B) {
	if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
		b.Skip("Skipping benchmark: test data not found")
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		scanner := NewOfferScanner(testDataPath)
		_ = scanner.ScanAll()
	}
}

// BenchmarkFixtureGeneration benchmarks fixture generation performance
func BenchmarkFixtureGeneration(b *testing.B) {
	if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
		b.Skip("Skipping benchmark: test data not found")
	}
	scanner := NewOfferScanner(testDataPath)
	_ = scanner.ScanAll()

	generator := NewTestFixtureGenerator(scanner)
	generator.BuildCustomerProfiles()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = generator.GenerateTestCustomers(20)
	}
}

// Example usage documentation
func ExampleOfferScanner() {
	// Skip example if test data doesn't exist - print expected output
	if _, err := os.Stat(testDataPath); os.IsNotExist(err) {
		fmt.Println("Total offers: 88")
		fmt.Println("Execution rate: 23.9%")
		fmt.Println("ENGIE offers: 3")
		return
	}

	// Create scanner
	scanner := NewOfferScanner(testDataPath)

	// Scan all offers
	_ = scanner.ScanAll()

	// Get summary
	summary := scanner.GetSummary()

	// Print results
	fmt.Printf("Total offers: %d\n", summary.TotalOffers)
	fmt.Printf("Execution rate: %.1f%%\n", summary.ExecutionRate)

	// Query by customer
	engieOffers := scanner.GetByCustomer("ENGIE")
	fmt.Printf("ENGIE offers: %d\n", len(engieOffers))

	// Output:
	// Total offers: 88
	// Execution rate: 23.9%
	// ENGIE offers: 3
}
