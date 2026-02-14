// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	fmt.Println("=== PH Holdings Offer Scanner Demo ===\n")

	// Step 1: Initialize scanner
	basePath := `C:\Projects\asymm_all_math\PH_TEST_DATA\Offers No 101 -150 (2025)`
	fmt.Printf("Scanning: %s\n\n", basePath)

	scanner := NewOfferScanner(basePath)

	// Step 2: Scan all offers
	err := scanner.ScanAll()
	if err != nil {
		fmt.Printf("ERROR: Scan failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✓ Scanned %d offers successfully\n\n", len(scanner.Offers))

	// Step 3: Generate summary
	summary := scanner.GetSummary()

	fmt.Println("=== SCAN SUMMARY ===")
	fmt.Printf("Total offers:      %d\n", summary.TotalOffers)
	fmt.Printf("Unique customers:  %d\n", summary.UniqueCustomers)
	fmt.Printf("Execution rate:    %.1f%%\n", summary.ExecutionRate)
	fmt.Printf("Avg revisions:     %.2f\n", summary.AvgRevisions)
	fmt.Printf("Avg cycle days:    %.1f\n\n", summary.AvgCycleDays)

	fmt.Println("Product type breakdown:")
	for productType, count := range summary.ProductTypeCount {
		percentage := float64(count) * 100.0 / float64(summary.TotalOffers)
		fmt.Printf("  %-6s: %2d offers (%.1f%%)\n", productType, count, percentage)
	}

	fmt.Println("\nTop 10 customers by volume:")
	for i, ranking := range summary.CustomersRanked {
		if i >= 10 {
			break
		}
		fmt.Printf("  %2d. %-15s: %2d offers (%2d executed, %.1f%% win rate)\n",
			i+1, ranking.Name, ranking.OfferCount, ranking.ExecutionCount, ranking.WinRate)
	}

	// Step 4: Show sample offers
	fmt.Println("\n=== SAMPLE OFFERS ===")

	executed := scanner.GetExecuted()
	if len(executed) > 0 {
		fmt.Printf("\nExecuted offer example: %s\n", executed[0].OfferID)
		fmt.Printf("  Customer:    %s\n", executed[0].CustomerName)
		fmt.Printf("  Product:     %s\n", executed[0].ProductType)
		fmt.Printf("  Revisions:   %d\n", executed[0].RevisionCount)
		fmt.Printf("  Has costing: %v\n", executed[0].HasCosting)
		fmt.Printf("  Files:       %d XLSX, %d XML, %d PDF\n",
			len(executed[0].XLSXFiles), len(executed[0].XMLFiles), len(executed[0].PDFFiles))
	}

	pending := scanner.GetPending()
	if len(pending) > 0 {
		fmt.Printf("\nPending offer example: %s\n", pending[0].OfferID)
		fmt.Printf("  Customer:    %s\n", pending[0].CustomerName)
		fmt.Printf("  Product:     %s\n", pending[0].ProductType)
		fmt.Printf("  Revisions:   %d\n", pending[0].RevisionCount)
	}

	// Step 5: Generate test fixtures
	fmt.Println("\n=== TEST FIXTURE GENERATION ===")

	generator := NewTestFixtureGenerator(scanner)
	generator.BuildCustomerProfiles()

	fmt.Printf("✓ Built %d customer profiles\n", len(generator.BaseCustomers))

	// Generate test customers
	testCustomers := generator.GenerateTestCustomers(20)
	fmt.Printf("✓ Generated %d test customers\n", len(testCustomers))

	// Generate payment predictor fixtures
	paymentFixtures := generator.GeneratePaymentPredictorFixtures()
	fmt.Println("\nPayment predictor test fixtures:")
	for grade, customers := range paymentFixtures {
		fmt.Printf("  %s: %d customers\n", grade, len(customers))
	}

	// Generate 360° profiles
	profiles := generator.GenerateCustomer360Fixtures()
	fmt.Printf("\n✓ Generated %d Customer 360° profiles\n", len(profiles))

	// Generate routing events
	events := generator.GenerateGeometryRoutingFixtures()
	fmt.Printf("✓ Generated %d geometry routing events\n", len(events))

	// Step 6: Export fixtures
	fmt.Println("\n=== EXPORTING FIXTURES ===")

	// Export test customers
	customersFile := "test_customers_fixture.json"
	err = generator.ExportToJSON(testCustomers, customersFile)
	if err != nil {
		fmt.Printf("ERROR: Export failed: %v\n", err)
	} else {
		fmt.Printf("✓ Exported test customers to %s\n", customersFile)
	}

	// Export payment fixtures
	paymentFile := "payment_predictor_fixtures.json"
	file, err := os.Create(paymentFile)
	if err == nil {
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		encoder.Encode(paymentFixtures)
		file.Close()
		fmt.Printf("✓ Exported payment fixtures to %s\n", paymentFile)
	}

	// Export 360° profiles
	profilesFile := "customer_360_profiles.json"
	file, err = os.Create(profilesFile)
	if err == nil {
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		encoder.Encode(profiles)
		file.Close()
		fmt.Printf("✓ Exported 360° profiles to %s\n", profilesFile)
	}

	// Export routing events
	eventsFile := "geometry_routing_events.json"
	file, err = os.Create(eventsFile)
	if err == nil {
		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		encoder.Encode(events)
		file.Close()
		fmt.Printf("✓ Exported routing events to %s\n", eventsFile)
	}

	// Step 7: Show customer queries
	fmt.Println("\n=== CUSTOMER QUERIES ===")

	// Query by customer
	for _, customerName := range []string{"ENGIE", "VEOLIA", "ALBA"} {
		offers := scanner.GetByCustomer(customerName)
		if len(offers) > 0 {
			fmt.Printf("%-10s: %d offers\n", customerName, len(offers))
		}
	}

	// Query by product type
	fmt.Println("\nProduct type queries:")
	for _, productType := range []string{"AIT", "FIT", "LIT", "SP"} {
		offers := scanner.GetByProductType(productType)
		if len(offers) > 0 {
			fmt.Printf("%-6s: %d offers\n", productType, len(offers))
		}
	}

	fmt.Println("\n=== DEMO COMPLETE ===")
	fmt.Printf("Total LOC: ~950 (offer_scanner.go + test_fixtures.go)\n")
	fmt.Printf("Offers scanned: %d\n", len(scanner.Offers))
	fmt.Printf("Test fixtures: %d customers, %d profiles, %d events\n",
		len(testCustomers), len(profiles), len(events))
}
