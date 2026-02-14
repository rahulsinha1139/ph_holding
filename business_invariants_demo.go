// ═══════════════════════════════════════════════════════════════════════════
// BUSINESS_INVARIANTS_DEMO - Demonstration of business rule validation
//
// This file can be run with: go run . --demo-invariants
//
// Built with LOVE × SIMPLICITY × TRUTH × JOY 🕉️💎⚡
// ═══════════════════════════════════════════════════════════════════════════

package main

import "fmt"

// RunBusinessInvariantsDemo demonstrates all business rule validations
func RunBusinessInvariantsDemo() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║ PH TRADING - BUSINESS RULES VALIDATOR DEMO                                ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Print invariant index
	PrintInvariantIndex()
	fmt.Println()

	// Scenario 1: Grade A Customer
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("SCENARIO 1: Grade A Customer - Excellent Payment History")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	demoGradeACustomer()
	fmt.Println()

	// Scenario 2: Grade D Customer
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("SCENARIO 2: Grade D Customer - Poor Payment History")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	demoGradeDCustomer()
	fmt.Println()

	// Scenario 3: ABB Competition
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("SCENARIO 3: ABB Competition - Margin Enforcement")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	demoABBCompetition()
	fmt.Println()

	// Summary
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	stats := InvariantStatistics()
	fmt.Printf("TOTAL: %d business invariants validated across 5 categories\n", stats["total"])
	fmt.Printf("  CRITICAL: %d | WARNING: %d | INFO: %d\n",
		stats["severity_CRITICAL"], stats["severity_WARNING"], stats["severity_INFO"])
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
}

func demoGradeACustomer() {
	// Create Grade A customer
	customer := &Customer{
		ID:             "CUST-A001",
		BusinessName:   "ALBA Aluminium Bahrain",
		OrderValue:     12000.0,
		OrderHistory:   []float64{10500, 11200, 12500, 11800},
		PaymentHistory: []int{35, 40, 38, 42, 36},
		RelationYears:  12,
		Industry:       "Aluminum Smelting",
		Country:        "Bahrain",
		IsEmergency:    0,
		HasABB:         0,
		DisputeCount:   0,
	}

	// Generate prediction
	predictor := NewPaymentPredictor(customer)
	prediction := predictor.Predict(customer)

	// Create context
	ctx := Context{
		Customer:   customer,
		Prediction: &prediction,
	}

	// Validate
	results := ValidateAll(ctx)

	// Print results
	fmt.Printf("Customer: %s\n", customer.BusinessName)
	fmt.Printf("Grade: %s (R1=%.1f%%, R2=%.1f%%, R3=%.1f%%)\n",
		prediction.Grade,
		prediction.ThreeRegimes.R1*100,
		prediction.ThreeRegimes.R2*100,
		prediction.ThreeRegimes.R3*100)
	fmt.Printf("Predicted Payment: %d days (Confidence: %.0f%%)\n",
		prediction.PredictedDays, prediction.Confidence*100)
	fmt.Println()

	printQuickSummary(results)
}

func demoGradeDCustomer() {
	// Create Grade D customer
	customer := &Customer{
		ID:             "CUST-D999",
		BusinessName:   "Delayed Payments Construction LLC",
		OrderValue:     4500.0,
		OrderHistory:   []float64{3800, 5200, 4100},
		PaymentHistory: []int{165, 180, 175, 190, 170},
		RelationYears:  1,
		Industry:       "Construction",
		Country:        "Bahrain",
		IsEmergency:    0,
		HasABB:         0,
		DisputeCount:   4,
	}

	// Generate prediction
	predictor := NewPaymentPredictor(customer)
	prediction := predictor.Predict(customer)

	// Create context
	ctx := Context{
		Customer:   customer,
		Prediction: &prediction,
	}

	// Validate
	results := ValidateAll(ctx)

	// Print results
	fmt.Printf("Customer: %s\n", customer.BusinessName)
	fmt.Printf("Grade: %s (R1=%.1f%%, R2=%.1f%%, R3=%.1f%%)\n",
		prediction.Grade,
		prediction.ThreeRegimes.R1*100,
		prediction.ThreeRegimes.R2*100,
		prediction.ThreeRegimes.R3*100)
	fmt.Printf("Predicted Payment: %d days (Confidence: %.0f%%)\n",
		prediction.PredictedDays, prediction.Confidence*100)
	fmt.Println()

	printQuickSummary(results)
}

func demoABBCompetition() {
	// Customer with ABB competition
	customer := &Customer{
		ID:             "CUST-B005",
		BusinessName:   "Industrial Automation LLC",
		OrderValue:     8000.0,
		OrderHistory:   []float64{7500, 8200, 7800},
		PaymentHistory: []int{70, 75, 68, 72},
		RelationYears:  4,
		Industry:       "Manufacturing",
		Country:        "Bahrain",
		IsEmergency:    0,
		HasABB:         1,
		DisputeCount:   1,
	}

	// Generate prediction
	predictor := NewPaymentPredictor(customer)
	prediction := predictor.Predict(customer)

	fmt.Printf("Customer: %s\n", customer.BusinessName)
	fmt.Printf("ABB Competition: YES ⚠️\n")
	fmt.Printf("Grade: %s\n", prediction.Grade)
	fmt.Println()

	// Test 1: Low margin (12%) - should DECLINE
	fmt.Println("→ Test 1: LOW margin (12%) with ABB")
	sheetLow := &CostingSheet{
		ActualMarginPct: 0.12,
		ApprovalStatus:  "✗ DECLINE",
	}
	ctxLow := Context{
		Customer:     customer,
		Prediction:   &prediction,
		CostingSheet: sheetLow,
	}
	resultsLow := ValidateBySeverity(ctxLow, SeverityCritical)
	printQuickSummary(resultsLow)

	// Test 2: High margin (18%) - should APPROVE
	fmt.Println("→ Test 2: HIGH margin (18%) with ABB")
	sheetHigh := &CostingSheet{
		ActualMarginPct: 0.18,
		ApprovalStatus:  "✓ APPROVE",
	}
	ctxHigh := Context{
		Customer:     customer,
		Prediction:   &prediction,
		CostingSheet: sheetHigh,
	}
	resultsHigh := ValidateBySeverity(ctxHigh, SeverityCritical)
	printQuickSummary(resultsHigh)
}

func printQuickSummary(results []InvariantValidationResult) {
	passed := 0
	critical := 0
	warning := 0
	info := 0

	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			switch r.Severity {
			case SeverityCritical:
				critical++
			case SeverityWarning:
				warning++
			case SeverityInfo:
				info++
			}
		}
	}

	fmt.Printf("Validation: %d/%d PASSED", passed, len(results))
	if critical > 0 {
		fmt.Printf(" | %d CRITICAL", critical)
	}
	if warning > 0 {
		fmt.Printf(" | %d WARNING", warning)
	}
	if info > 0 {
		fmt.Printf(" | %d INFO", info)
	}
	fmt.Println()

	// Print failures
	for _, r := range results {
		if !r.Passed {
			symbol := "⚠"
			if r.Severity == SeverityCritical {
				symbol = "✗"
			} else if r.Severity == SeverityInfo {
				symbol = "ℹ"
			}
			fmt.Printf("  %s %s: %v\n", symbol, r.InvariantName, r.Error)
		}
	}
}
