// ═══════════════════════════════════════════════════════════════════════════
// VALIDATE_BUSINESS_RULES - Command-line tool for validating PH Trading data
//
// Usage:
//   go run cmd/validate_business_rules/main.go
//
// This tool demonstrates:
//   1. Loading customer data
//   2. Running predictions
//   3. Validating against business invariants
//   4. Generating compliance reports
//
// Built with LOVE × SIMPLICITY × TRUTH × JOY 🕉️💎⚡
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"

	"ph_holdings_app/pkg/engines"
)

// Type aliases for convenience
type Customer = engines.Customer
type PaymentPrediction = engines.PaymentPrediction
type CostingSheet = engines.CostingSheet
type CostingItem = engines.CostingItem
type CustomerGrade = engines.CustomerGrade

// Placeholder for business invariants (TODO: move to shared package)
type InvariantValidationResult struct {
	InvariantName string
	Category      string
	Severity      Severity
	Passed        bool
	Error         error
}

type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
)

type Context struct {
	Customer     *Customer
	Prediction   *PaymentPrediction
	CostingSheet *CostingSheet
}

// Stub functions for business invariants (TODO: implement)
func ValidateAll(ctx Context) []InvariantValidationResult {
	return []InvariantValidationResult{}
}

func ValidateBySeverity(ctx Context, sev Severity) []InvariantValidationResult {
	return []InvariantValidationResult{}
}

func ValidateCategory(ctx Context, category string) []InvariantValidationResult {
	return []InvariantValidationResult{}
}

func PrintInvariantIndex() {
	fmt.Println("(Business invariants index not yet imported - TODO)")
}

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║ PH TRADING - BUSINESS RULES VALIDATOR                                     ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Scenario 1: Validate Grade A Customer
	fmt.Println("SCENARIO 1: Grade A Customer - Excellent Payment History")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	validateGradeACustomer()
	fmt.Println()

	// Scenario 2: Validate Grade D Customer
	fmt.Println("SCENARIO 2: Grade D Customer - Poor Payment History")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	validateGradeDCustomer()
	fmt.Println()

	// Scenario 3: ABB Competition Test
	fmt.Println("SCENARIO 3: ABB Competition - Margin Enforcement")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	validateABBCompetition()
	fmt.Println()

	// Scenario 4: Emergency Order Premium
	fmt.Println("SCENARIO 4: Emergency Order - Premium Pricing")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	validateEmergencyOrder()
	fmt.Println()

	// Scenario 5: Product Margin Validation
	fmt.Println("SCENARIO 5: Product Margins - Standard vs Actual")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	validateProductMargins()
	fmt.Println()

	// Print invariant index
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	PrintInvariantIndex()
}

// validateGradeACustomer validates a Grade A customer scenario
func validateGradeACustomer() {
	// Real PH Trading Grade A customer profile
	customer := &Customer{
		ID:             "CUST-A001",
		BusinessName:   "ALBA Aluminium Bahrain",
		OrderValue:     12000.0,
		OrderHistory:   []float64{10500, 11200, 12500, 11800},
		PaymentHistory: []int{35, 40, 38, 42, 36}, // Consistently pays within 35-42 days
		RelationYears:  12,                        // Long-standing relationship
		Industry:       "Aluminum Smelting",
		Country:        "Bahrain",
		IsEmergency:    false,
		HasABB:         false,
		DisputeCount:   0, // Zero disputes
	}

	// Generate prediction
	predictor := engines.NewPaymentPredictor(customer)
	prediction := predictor.Predict(customer)

	// Create mock costing sheet
	sheet := &CostingSheet{
		CustomerID:       customer.ID,
		CustomerName:     customer.BusinessName,
		CustomerGrade:    CustomerGrade(prediction.Grade),
		Items: []CostingItem{
			{
				OrderCode:        "71126072",
				Description:      "Promag 50W DN50 Electromagnetic Flow Meter",
				ProductType:      "E+H Flow",
				Quantity:         3,
				UnitCostBHD:      1800.0,
				TotalCostBHD:     5400.0,
				StandardMargin:   0.15,
				UnitSellBHD:      2070.0,
				TotalSellBHD:     6210.0,
				CustomerDiscount: 0.06, // 6% discount (within 7% limit for Grade A)
				FinalUnitBHD:     1945.80,
				FinalTotalBHD:    5837.40,
				UnitProfitBHD:    145.80,
				TotalProfitBHD:   437.40,
				ActualMargin:     0.081, // 8.1% after discount
			},
		},
		TotalCostBHD:      5400.0,
		TotalSellBHD:      6210.0,
		TotalDiscountBHD:  372.60,
		TotalFinalBHD:     5837.40,
		TotalProfitBHD:    437.40,
		StandardMarginPct: 0.15,
		ActualMarginPct:   0.081,
		PaymentTerms:      "Net 45 days",
		AdvanceRequired:   0.0,
		ApprovalStatus:    "✓ APPROVE",
	}

	// Create context and validate
	ctx := Context{
		Customer:     customer,
		Prediction:   &prediction,
		CostingSheet: sheet,
	}

	results := ValidateAll(ctx)
	printScenarioResults(customer, &prediction, sheet, results)
}

// validateGradeDCustomer validates a Grade D customer scenario
func validateGradeDCustomer() {
	// Real PH Trading Grade D customer profile
	customer := &Customer{
		ID:             "CUST-D999",
		BusinessName:   "Delayed Payments Construction LLC",
		OrderValue:     4500.0,
		OrderHistory:   []float64{3800, 5200, 4100},
		PaymentHistory: []int{165, 180, 175, 190, 170}, // Pays in 165-190 days (very late)
		RelationYears:  1,                              // New customer
		Industry:       "Construction",
		Country:        "Bahrain",
		IsEmergency:    false,
		HasABB:         false,
		DisputeCount:   4, // Multiple disputes
	}

	// Generate prediction
	predictor := engines.NewPaymentPredictor(customer)
	prediction := predictor.Predict(customer)

	// Create mock costing sheet - DECLINED scenario
	sheet := &CostingSheet{
		CustomerID:       customer.ID,
		CustomerName:     customer.BusinessName,
		CustomerGrade:    CustomerGrade(prediction.Grade),
		Items: []CostingItem{
			{
				OrderCode:        "GIC-001",
				Description:      "GIC Pressure Transmitter",
				ProductType:      "GIC",
				Quantity:         5,
				UnitCostBHD:      900.0,
				TotalCostBHD:     4500.0,
				StandardMargin:   0.10,
				UnitSellBHD:      990.0,
				TotalSellBHD:     4950.0,
				CustomerDiscount: 0.00, // NO discount for Grade D
				FinalUnitBHD:     990.0,
				FinalTotalBHD:    4950.0,
				UnitProfitBHD:    90.0,
				TotalProfitBHD:   450.0,
				ActualMargin:     0.10,
			},
		},
		TotalCostBHD:      4500.0,
		TotalSellBHD:      4950.0,
		TotalDiscountBHD:  0.0,
		TotalFinalBHD:     4950.0,
		TotalProfitBHD:    450.0,
		StandardMarginPct: 0.10,
		ActualMarginPct:   0.10,
		PaymentTerms:      "100% advance or DECLINE",
		AdvanceRequired:   1.00, // 100% advance REQUIRED
		ApprovalStatus:    "✗ DECLINE",
	}

	// Create context and validate
	ctx := Context{
		Customer:     customer,
		Prediction:   &prediction,
		CostingSheet: sheet,
	}

	results := ValidateAll(ctx)
	printScenarioResults(customer, &prediction, sheet, results)
}

// validateABBCompetition validates ABB competition scenario
func validateABBCompetition() {
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
		IsEmergency:    false,
		HasABB:         true, // ⚠️ ABB IS COMPETING
		DisputeCount:   1,
	}

	// Generate prediction
	predictor := engines.NewPaymentPredictor(customer)
	prediction := predictor.Predict(customer)

	// Scenario A: LOW margin (12%) with ABB - should DECLINE
	sheetLowMargin := &CostingSheet{
		CustomerID:       customer.ID,
		CustomerName:     customer.BusinessName,
		CustomerGrade:    CustomerGrade(prediction.Grade),
		ActualMarginPct:   0.12, // 12% - TOO LOW with ABB
		ApprovalStatus:    "✗ DECLINE",
	}

	fmt.Println("→ Test 1: LOW margin (12%) with ABB competition")
	ctxLow := Context{
		Customer:     customer,
		Prediction:   &prediction,
		CostingSheet: sheetLowMargin,
	}
	resultsLow := ValidateBySeverity(ctxLow, SeverityCritical)
	printValidationSummary(resultsLow, "ABB + 12% margin + DECLINE")

	// Scenario B: HIGH margin (18%) with ABB - should APPROVE
	sheetHighMargin := &CostingSheet{
		CustomerID:       customer.ID,
		CustomerName:     customer.BusinessName,
		CustomerGrade:    CustomerGrade(prediction.Grade),
		ActualMarginPct:   0.18, // 18% - ACCEPTABLE with ABB
		ApprovalStatus:    "✓ APPROVE",
	}

	fmt.Println("→ Test 2: HIGH margin (18%) with ABB competition")
	ctxHigh := Context{
		Customer:     customer,
		Prediction:   &prediction,
		CostingSheet: sheetHighMargin,
	}
	resultsHigh := ValidateBySeverity(ctxHigh, SeverityCritical)
	printValidationSummary(resultsHigh, "ABB + 18% margin + APPROVE")
}

// validateEmergencyOrder validates emergency order premium
func validateEmergencyOrder() {
	// Emergency order customer
	customer := &Customer{
		ID:             "CUST-EM01",
		BusinessName:   "Urgent Repair Services WLL",
		OrderValue:     6000.0,
		OrderHistory:   []float64{5500, 6200, 5800},
		PaymentHistory: []int{50, 48, 52}, // Good payer (emergency = reliable)
		RelationYears:  3,
		Industry:       "Maintenance",
		Country:        "Saudi Arabia",
		IsEmergency:    true, // 🚨 EMERGENCY ORDER
		HasABB:         false,
		DisputeCount:   0,
	}

	// Generate prediction
	predictor := engines.NewPaymentPredictor(customer)
	prediction := predictor.Predict(customer)

	// Scenario A: Standard margin (no premium)
	sheetStandard := &CostingSheet{
		CustomerID:       customer.ID,
		CustomerName:     customer.BusinessName,
		CustomerGrade:    CustomerGrade(prediction.Grade),
		ActualMarginPct:   0.15, // 15% - MISSED OPPORTUNITY
		ApprovalStatus:    "✓ APPROVE",
	}

	fmt.Println("→ Test 1: Emergency order with standard 15% margin")
	ctxStandard := Context{
		Customer:     customer,
		Prediction:   &prediction,
		CostingSheet: sheetStandard,
	}
	resultsStandard := ValidateBySeverity(ctxStandard, SeverityInfo)
	printValidationSummary(resultsStandard, "Emergency + 15% margin (no premium)")

	// Scenario B: Premium margin
	sheetPremium := &CostingSheet{
		CustomerID:       customer.ID,
		CustomerName:     customer.BusinessName,
		CustomerGrade:    CustomerGrade(prediction.Grade),
		ActualMarginPct:   0.25, // 25% - PREMIUM JUSTIFIED
		ApprovalStatus:    "✓ APPROVE",
	}

	fmt.Println("→ Test 2: Emergency order with premium 25% margin")
	ctxPremium := Context{
		Customer:     customer,
		Prediction:   &prediction,
		CostingSheet: sheetPremium,
	}
	resultsPremium := ValidateBySeverity(ctxPremium, SeverityInfo)
	printValidationSummary(resultsPremium, "Emergency + 25% margin (premium applied)")
}

// validateProductMargins validates product-specific margin rules
func validateProductMargins() {
	products := []struct {
		productType    string
		standardMargin float64
	}{
		{"E+H Flow", 0.15},
		{"E+H Level", 0.18},
		{"Servomex", 0.25},
		{"GIC", 0.10},
	}

	for _, p := range products {
		// Test with CORRECT margin
		sheetCorrect := &CostingSheet{
			Items: []CostingItem{
				{
					OrderCode:      "TEST-001",
					ProductType:    p.productType,
					StandardMargin: p.standardMargin, // CORRECT
				},
			},
		}

		ctxCorrect := Context{CostingSheet: sheetCorrect}

		// Test with WRONG margin
		sheetWrong := &CostingSheet{
			Items: []CostingItem{
				{
					OrderCode:      "TEST-002",
					ProductType:    p.productType,
					StandardMargin: p.standardMargin + 0.05, // WRONG (+5%)
				},
			},
		}

		ctxWrong := Context{CostingSheet: sheetWrong}

		// Validate
		resultsCorrect := ValidateCategory(ctxCorrect, "PRICING")
		resultsWrong := ValidateCategory(ctxWrong, "PRICING")

		fmt.Printf("→ %s: Standard %.0f%%\n", p.productType, p.standardMargin*100)
		fmt.Printf("  Correct margin (%.0f%%): ", p.standardMargin*100)
		if hasFailures(resultsCorrect) {
			fmt.Println("✗ FAILED")
		} else {
			fmt.Println("✓ PASSED")
		}

		fmt.Printf("  Wrong margin (%.0f%%): ", (p.standardMargin+0.05)*100)
		if hasFailures(resultsWrong) {
			fmt.Println("✗ FAILED (as expected)")
		} else {
			fmt.Println("⚠ PASSED (unexpected)")
		}
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════

func printScenarioResults(customer *Customer, prediction *PaymentPrediction, sheet *CostingSheet, results []InvariantValidationResult) {
	fmt.Printf("Customer: %s\n", customer.BusinessName)
	fmt.Printf("Grade: %s (R1=%.1f%%, R2=%.1f%%, R3=%.1f%%)\n",
		prediction.Grade,
		prediction.ThreeRegimes.R1*100,
		prediction.ThreeRegimes.R2*100,
		prediction.ThreeRegimes.R3*100)
	fmt.Printf("Predicted Payment: %d days (Confidence: %.0f%%)\n",
		prediction.PredictedDays, prediction.Confidence*100)
	fmt.Printf("Payment Terms: %s\n", sheet.PaymentTerms)
	if sheet.AdvanceRequired > 0 {
		fmt.Printf("Advance Required: %.0f%%\n", sheet.AdvanceRequired*100)
	}
	fmt.Printf("Actual Margin: %.1f%%\n", sheet.ActualMarginPct*100)
	fmt.Printf("Approval Status: %s\n", sheet.ApprovalStatus)
	fmt.Println()

	// Print validation results
	printValidationSummary(results, customer.BusinessName)
}

func printValidationSummary(results []InvariantValidationResult, label string) {
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

	fmt.Printf("  Validation: %s\n", label)
	fmt.Printf("    ✓ Passed: %d/%d\n", passed, len(results))
	if critical > 0 {
		fmt.Printf("    ✗ CRITICAL failures: %d\n", critical)
	}
	if warning > 0 {
		fmt.Printf("    ⚠ WARNING failures: %d\n", warning)
	}
	if info > 0 {
		fmt.Printf("    ℹ INFO failures: %d\n", info)
	}

	// Print failures
	for _, r := range results {
		if !r.Passed {
			symbol := "⚠"
			if r.Severity == SeverityCritical {
				symbol = "✗"
			} else if r.Severity == SeverityInfo {
				symbol = "ℹ"
			}
			fmt.Printf("    %s %s: %v\n", symbol, r.InvariantName, r.Error)
		}
	}
	fmt.Println()
}

func hasFailures(results []InvariantValidationResult) bool {
	for _, r := range results {
		if !r.Passed {
			return true
		}
	}
	return false
}
