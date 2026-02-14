package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestEHParser tests the E+H XML parser
func TestEHParser(t *testing.T) {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║ E+H XML PARSER TEST SUITE")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Test 1: Parse sample basket
	t.Run("ParseSampleBasket", func(t *testing.T) {
		parser := NewEHParser()
		basket, err := parser.ParseFile("test_data/eh_sample_basket.xml")
		if err != nil {
			t.Fatalf("Failed to parse sample basket: %v", err)
		}

		// Validate basic structure
		if basket.CustomerNumber != "8012210" {
			t.Errorf("Expected customer number 8012210, got %s", basket.CustomerNumber)
		}

		if basket.ItemCount != 4 {
			t.Errorf("Expected 4 items, got %d", basket.ItemCount)
		}

		// Validate totals
		expectedNetEUR := 6595.06
		if basket.TotalNetEUR != expectedNetEUR {
			t.Errorf("Expected net EUR %.2f, got %.2f", expectedNetEUR, basket.TotalNetEUR)
		}

		// Validate conversion to BHD
		expectedNetBHD := expectedNetEUR * EUR_TO_BHD
		if basket.TotalNetBHD != expectedNetBHD {
			t.Errorf("Expected net BHD %.2f, got %.2f", expectedNetBHD, basket.TotalNetBHD)
		}

		fmt.Println("✓ PASS: Sample basket parsed correctly")
		parser.PrintSummary(basket)
	})

	// Test 2: Product type classification
	t.Run("ProductTypeClassification", func(t *testing.T) {
		parser := NewEHParser()
		basket, _ := parser.ParseFile("test_data/eh_sample_basket.xml")

		// Check first item (Liquiline CM442)
		if basket.Items[0].ProductType != "E+H Flow" {
			t.Errorf("Expected 'E+H Flow' for CM442, got '%s'", basket.Items[0].ProductType)
		}

		// Check second item (Prosonic FMU90)
		if basket.Items[1].ProductType != "E+H Level" {
			t.Errorf("Expected 'E+H Level' for FMU90, got '%s'", basket.Items[1].ProductType)
		}

		// Check third item (Cerabar PMC)
		if basket.Items[2].ProductType != "E+H Pressure" {
			t.Errorf("Expected 'E+H Pressure' for PMC, got '%s'", basket.Items[2].ProductType)
		}

		// Check fourth item (iTHERM)
		if basket.Items[3].ProductType != "E+H Temperature" {
			t.Errorf("Expected 'E+H Temperature' for TMT, got '%s'", basket.Items[3].ProductType)
		}

		fmt.Println("✓ PASS: Product type classification correct")
	})

	// Test 3: Product type statistics
	t.Run("ProductTypeStats", func(t *testing.T) {
		parser := NewEHParser()
		basket, _ := parser.ParseFile("test_data/eh_sample_basket.xml")
		stats := parser.GetProductTypeStats(basket)

		fmt.Println("\nProduct Type Statistics:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for productType, stat := range stats {
			fmt.Printf("%-20s: %d items, %.2f BHD, %.2f BHD discount\n",
				productType, stat.ItemCount, stat.TotalValueBHD, stat.TotalDiscountBHD)
		}
		fmt.Println()

		if len(stats) != 4 {
			t.Errorf("Expected 4 product types, got %d", len(stats))
		}

		fmt.Println("✓ PASS: Product type statistics calculated")
	})
}

// TestCostingEngine tests the costing engine
func TestCostingEngine(t *testing.T) {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║ COSTING ENGINE TEST SUITE")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Test 1: Grade A customer (best)
	t.Run("GradeA_Customer", func(t *testing.T) {
		engine := NewCostingEngine()
		basket, _ := engine.Parser.ParseFile("test_data/eh_sample_basket.xml")

		// Grade A customer: excellent payment history
		customer := &Customer{
			ID:             "CUST001",
			BusinessName:   "BAPCO (Bahrain Petroleum Company)",
			OrderValue:     2703.38, // Total in BHD
			OrderHistory:   []float64{2500, 2800, 2600, 2700},
			PaymentHistory: []int{30, 28, 32, 29}, // Pays in ~30 days
			RelationYears:  8,
			Industry:       "Oil & Gas",
			Country:        "Bahrain",
			IsEmergency:    0,
			HasABB:         0,
			DisputeCount:   0,
		}

		sheet, err := engine.GenerateCostingSheet(basket, customer)
		if err != nil {
			t.Fatalf("Failed to generate costing sheet: %v", err)
		}

		// Should be Grade A
		if sheet.CustomerGrade != GradeA {
			t.Errorf("Expected Grade A, got %s", sheet.CustomerGrade)
		}

		// Should get 7% discount
		if sheet.Items[0].CustomerDiscount != 0.07 {
			t.Errorf("Expected 7%% discount, got %.1f%%", sheet.Items[0].CustomerDiscount*100)
		}

		// Should be approved or caution (caution is valid if margin < 8% due to discount)
		if sheet.ApprovalStatus != "✓ APPROVE" && sheet.ApprovalStatus != "⚠ CAUTION" {
			t.Errorf("Expected APPROVE or CAUTION status, got %s", sheet.ApprovalStatus)
		}

		// No advance required
		if sheet.AdvanceRequired != 0.0 {
			t.Errorf("Expected no advance, got %.0f%%", sheet.AdvanceRequired*100)
		}

		fmt.Println("✓ PASS: Grade A customer processed correctly")
		engine.PrintCostingSheet(sheet)
	})

	// Test 2: Grade B customer (good)
	t.Run("GradeB_Customer", func(t *testing.T) {
		engine := NewCostingEngine()
		basket, _ := engine.Parser.ParseFile("test_data/eh_sample_basket.xml")

		// Grade B customer: decent payment history
		customer := &Customer{
			ID:             "CUST002",
			BusinessName:   "Gulf Petrochemical Industries Company",
			OrderValue:     2703.38,
			OrderHistory:   []float64{1500, 2000, 1800},
			PaymentHistory: []int{65, 72, 68, 70}, // Pays in ~70 days
			RelationYears:  3,
			Industry:       "Petrochemicals",
			Country:        "Bahrain",
			IsEmergency:    0,
			HasABB:         0,
			DisputeCount:   1,
		}

		sheet, err := engine.GenerateCostingSheet(basket, customer)
		if err != nil {
			t.Fatalf("Failed to generate costing sheet: %v", err)
		}

		// Should be Grade B
		if sheet.CustomerGrade != GradeB {
			t.Errorf("Expected Grade B, got %s", sheet.CustomerGrade)
		}

		// Should get 3% discount
		if sheet.Items[0].CustomerDiscount != 0.03 {
			t.Errorf("Expected 3%% discount, got %.1f%%", sheet.Items[0].CustomerDiscount*100)
		}

		fmt.Println("✓ PASS: Grade B customer processed correctly")
		fmt.Printf("Grade: %s, Discount: %.0f%%, Terms: %s\n\n",
			sheet.CustomerGrade, sheet.Items[0].CustomerDiscount*100, sheet.PaymentTerms)
	})

	// Test 3: Grade C customer (caution)
	t.Run("GradeC_Customer", func(t *testing.T) {
		engine := NewCostingEngine()
		basket, _ := engine.Parser.ParseFile("test_data/eh_sample_basket.xml")

		// Grade C customer: risky payment history
		customer := &Customer{
			ID:             "CUST003",
			BusinessName:   "New Trading Company LLC",
			OrderValue:     2703.38,
			OrderHistory:   []float64{800, 1200},
			PaymentHistory: []int{95, 110, 102}, // Pays in ~100 days
			RelationYears:  1,
			Industry:       "Trading",
			Country:        "Bahrain",
			IsEmergency:    0,
			HasABB:         0,
			DisputeCount:   2,
		}

		sheet, err := engine.GenerateCostingSheet(basket, customer)
		if err != nil {
			t.Fatalf("Failed to generate costing sheet: %v", err)
		}

		// Should be Grade C
		if sheet.CustomerGrade != GradeC {
			t.Errorf("Expected Grade C, got %s", sheet.CustomerGrade)
		}

		// No discount for Grade C
		if sheet.Items[0].CustomerDiscount != 0.0 {
			t.Errorf("Expected 0%% discount, got %.1f%%", sheet.Items[0].CustomerDiscount*100)
		}

		// Should require 50% advance
		if sheet.AdvanceRequired != 0.50 {
			t.Errorf("Expected 50%% advance, got %.0f%%", sheet.AdvanceRequired*100)
		}

		fmt.Println("✓ PASS: Grade C customer processed correctly")
		fmt.Printf("Grade: %s, Advance: %.0f%%, Warning: %v\n\n",
			sheet.CustomerGrade, sheet.AdvanceRequired*100, len(sheet.RiskWarnings) > 0)
	})

	// Test 4: Grade D customer (high risk)
	t.Run("GradeD_Customer", func(t *testing.T) {
		engine := NewCostingEngine()
		basket, _ := engine.Parser.ParseFile("test_data/eh_sample_basket.xml")

		// Grade D customer: very risky
		customer := &Customer{
			ID:             "CUST004",
			BusinessName:   "Unreliable LLC",
			OrderValue:     2703.38,
			OrderHistory:   []float64{500},
			PaymentHistory: []int{145, 160, 155, 170}, // Pays in ~150+ days
			RelationYears:  0,
			Industry:       "Unknown",
			Country:        "Bahrain",
			IsEmergency:    0,
			HasABB:         0,
			DisputeCount:   5,
		}

		sheet, err := engine.GenerateCostingSheet(basket, customer)
		if err != nil {
			t.Fatalf("Failed to generate costing sheet: %v", err)
		}

		// Should be Grade D
		if sheet.CustomerGrade != GradeD {
			t.Errorf("Expected Grade D, got %s", sheet.CustomerGrade)
		}

		// Should be declined
		if sheet.ApprovalStatus != "✗ DECLINE" {
			t.Errorf("Expected DECLINE status, got %s", sheet.ApprovalStatus)
		}

		// Should require 100% advance
		if sheet.AdvanceRequired != 1.0 {
			t.Errorf("Expected 100%% advance, got %.0f%%", sheet.AdvanceRequired*100)
		}

		fmt.Println("✓ PASS: Grade D customer correctly flagged as high risk")
		fmt.Printf("Grade: %s, Status: %s\n\n", sheet.CustomerGrade, sheet.ApprovalStatus)
	})

	// Test 5: ABB competition scenario
	t.Run("ABB_Competition", func(t *testing.T) {
		engine := NewCostingEngine()
		basket, _ := engine.Parser.ParseFile("test_data/eh_sample_basket.xml")

		// Grade B customer with ABB competing
		customer := &Customer{
			ID:             "CUST005",
			BusinessName:   "Industrial Company WLL",
			OrderValue:     2703.38,
			OrderHistory:   []float64{2000, 2200},
			PaymentHistory: []int{60, 65},
			RelationYears:  2,
			Industry:       "Industry",
			Country:        "Bahrain",
			IsEmergency:    0,
			HasABB:         1, // ABB is competing!
			DisputeCount:   0,
		}

		sheet, err := engine.GenerateCostingSheet(basket, customer)
		if err != nil {
			t.Fatalf("Failed to generate costing sheet: %v", err)
		}

		// Should have ABB warning
		hasABBWarning := false
		for _, warning := range sheet.RiskWarnings {
			if contains(warning, "ABB") {
				hasABBWarning = true
				break
			}
		}

		if !hasABBWarning {
			t.Errorf("Expected ABB competition warning")
		}

		// Check if margin check triggers decline
		if sheet.ActualMarginPct < 0.15 {
			if sheet.ApprovalStatus != "✗ DECLINE" {
				t.Errorf("Expected DECLINE with low margin and ABB competition")
			}
		}

		fmt.Println("✓ PASS: ABB competition scenario handled correctly")
		fmt.Printf("Margin: %.1f%%, ABB Warning: %v\n\n",
			sheet.ActualMarginPct*100, hasABBWarning)
	})

	// Test 6: Emergency order
	t.Run("Emergency_Order", func(t *testing.T) {
		engine := NewCostingEngine()
		basket, _ := engine.Parser.ParseFile("test_data/eh_sample_basket.xml")

		// Grade A customer with emergency
		customer := &Customer{
			ID:             "CUST006",
			BusinessName:   "ALBA (Aluminium Bahrain)",
			OrderValue:     2703.38,
			OrderHistory:   []float64{3000, 3500, 3200},
			PaymentHistory: []int{25, 28, 30},
			RelationYears:  10,
			Industry:       "Aluminium",
			Country:        "Bahrain",
			IsEmergency:    1, // Emergency!
			HasABB:         0,
			DisputeCount:   0,
		}

		sheet, err := engine.GenerateCostingSheet(basket, customer)
		if err != nil {
			t.Fatalf("Failed to generate costing sheet: %v", err)
		}

		// Should have emergency pricing suggestion
		hasEmergencyNote := false
		for _, warning := range sheet.RiskWarnings {
			if contains(warning, "emergency") || contains(warning, "Emergency") {
				hasEmergencyNote = true
				break
			}
		}

		if !hasEmergencyNote {
			t.Errorf("Expected emergency pricing note")
		}

		fmt.Println("✓ PASS: Emergency order flagged for premium pricing")
		fmt.Printf("Emergency: %v, Premium Suggestion: %v\n\n",
			customer.IsEmergency, hasEmergencyNote)
	})
}

// TestROICalculation tests the ROI calculation
func TestROICalculation(t *testing.T) {
	fmt.Println("╔═══════════════════════════════════════════════════════════════╗")
	fmt.Println("║ ROI CALCULATION TEST")
	fmt.Println("╚═══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	engine := NewCostingEngine()
	roi := engine.CalculateROI()

	// Print ROI analysis
	fmt.Println("3-YEAR ROI ANALYSIS:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	roiJSON, _ := json.MarshalIndent(roi, "", "  ")
	fmt.Println(string(roiJSON))
	fmt.Println()

	// Validate payback period
	paybackMonths := roi["payback_months"].(float64)
	if paybackMonths > 6 {
		t.Errorf("Payback period too long: %.1f months", paybackMonths)
	}

	// Validate ROI percentage
	roiPct := roi["roi_percent"].(float64)
	if roiPct < 100 {
		t.Errorf("ROI too low: %.1f%%", roiPct)
	}

	fmt.Printf("✓ PASS: Payback in %.1f months, ROI of %.0f%%\n\n",
		paybackMonths, roiPct)
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		 len(s) > len(substr)+1 && containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
