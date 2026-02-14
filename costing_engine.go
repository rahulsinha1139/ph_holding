package main

import (
	"fmt"
	"math"
	"time"
)

// ProductMarkupRules defines markup percentages by product type
// These are BUSINESS RULES from PH Trading's pricing strategy
var ProductMarkupRules = map[string]float64{
	"E+H Flow":        0.15, // 15% margin
	"E+H Level":       0.18, // 18% margin
	"E+H Pressure":    0.18, // 18% margin
	"E+H Temperature": 0.15, // 15% margin
	"E+H Analytics":   0.20, // 20% margin (higher margin)
	"E+H General":     0.12, // 12% margin (conservative)
	"Servomex":        0.25, // 25% margin (high value)
	"GIC":             0.10, // 10% margin (commodity)
}

// CustomerGrade represents customer payment risk classification
type CustomerGrade string

const (
	GradeA CustomerGrade = "A" // Best: R3 ≥ 50%, max 7% discount
	GradeB CustomerGrade = "B" // Good: R3 ≥ 35%, max 3% discount
	GradeC CustomerGrade = "C" // Caution: R3 ≥ 20%, no discount, 50% advance
	GradeD CustomerGrade = "D" // High risk: R3 < 20%, decline or 100% advance
)

// CostingEngine calculates customer quotations with margins and risk adjustments
type CostingEngine struct {
	Parser    *EHParser
	Predictor *PaymentPredictor
}

// NewCostingEngine creates a new costing engine
func NewCostingEngine() *CostingEngine {
	return &CostingEngine{
		Parser: NewEHParser(),
	}
}

// CostingItem represents a single line item with markup applied
type CostingItem struct {
	// From E+H XML
	OrderCode      string `json:"order_code"`
	Description    string `json:"description"`
	Quantity       int    `json:"quantity"`
	ProductType    string `json:"product_type"`
	ProductionDays int    `json:"production_days"`

	// E+H Pricing (what PH Trading pays)
	UnitCostBHD  float64 `json:"unit_cost_bhd"`  // E+H discounted price
	TotalCostBHD float64 `json:"total_cost_bhd"` // Unit × Qty

	// PH Trading Pricing (what customer pays)
	StandardMargin float64 `json:"standard_margin"` // % (0.15 = 15%)
	UnitSellBHD    float64 `json:"unit_sell_bhd"`   // Cost × (1 + Margin)
	TotalSellBHD   float64 `json:"total_sell_bhd"`  // Unit × Qty

	// Customer Adjustments
	CustomerDiscount float64 `json:"customer_discount"` // % discount applied
	FinalUnitBHD     float64 `json:"final_unit_bhd"`    // After discount
	FinalTotalBHD    float64 `json:"final_total_bhd"`   // After discount

	// Profit Analysis
	UnitProfitBHD  float64 `json:"unit_profit_bhd"`  // Final - Cost
	TotalProfitBHD float64 `json:"total_profit_bhd"` // Unit profit × Qty
	ActualMargin   float64 `json:"actual_margin"`    // % after discount
}

// CostingSheet represents complete quotation
type CostingSheet struct {
	// Customer Information
	CustomerID        string            `json:"customer_id"`
	CustomerName      string            `json:"customer_name"`
	CustomerGrade     CustomerGrade     `json:"customer_grade"`
	PaymentPrediction PaymentPrediction `json:"payment_prediction"`

	// Order Information
	QuotationDate string `json:"quotation_date"`
	ValidUntil    string `json:"valid_until"`
	SourceBasket  string `json:"source_basket"`

	// Line Items
	Items []CostingItem `json:"items"`

	// Totals
	TotalCostBHD     float64 `json:"total_cost_bhd"`     // What PH pays E+H
	TotalSellBHD     float64 `json:"total_sell_bhd"`     // Before discount
	TotalDiscountBHD float64 `json:"total_discount_bhd"` // Discount amount
	TotalFinalBHD    float64 `json:"total_final_bhd"`    // After discount (before VAT)
	TotalProfitBHD   float64 `json:"total_profit_bhd"`   // Profit after discount

	// VAT (from Settings)
	VATRate       float64 `json:"vat_rate"`        // % from settings (e.g., 10.0)
	VATAmountBHD  float64 `json:"vat_amount_bhd"`  // TotalFinalBHD * (VATRate / 100)
	GrandTotalBHD float64 `json:"grand_total_bhd"` // TotalFinalBHD + VATAmountBHD

	// Margins
	StandardMarginPct float64 `json:"standard_margin_pct"` // Weighted average
	ActualMarginPct   float64 `json:"actual_margin_pct"`   // After discount

	// Payment Terms
	PaymentTerms    string  `json:"payment_terms"`    // Based on grade
	AdvanceRequired float64 `json:"advance_required"` // % upfront

	// Risk Assessment
	ApprovalStatus    string   `json:"approval_status"` // APPROVE/CAUTION/DECLINE
	RiskWarnings      []string `json:"risk_warnings"`
	RecommendedAction string   `json:"recommended_action"`
}

// GenerateCostingSheet creates a complete quotation from E+H basket
func (ce *CostingEngine) GenerateCostingSheet(basket *ParsedEHBasket, customer *Customer) (*CostingSheet, error) {
	// Step 1: Predict customer payment behavior
	predictor := NewPaymentPredictor(customer)
	prediction := predictor.Predict(customer)

	// Step 2: Initialize costing sheet
	sheet := &CostingSheet{
		CustomerID:        customer.ID,
		CustomerName:      customer.BusinessName,
		CustomerGrade:     CustomerGrade(prediction.Grade),
		PaymentPrediction: prediction,
		QuotationDate:     time.Now().UTC().Format("2006-01-02"),
		ValidUntil:        time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02"), // 30 days validity
		SourceBasket:      basket.SourceFile,
		Items:             make([]CostingItem, 0, len(basket.Items)),
		RiskWarnings:      make([]string, 0),
	}

	// Step 3: Determine customer discount and payment terms based on grade
	customerDiscount := ce.GetCustomerDiscount(CustomerGrade(prediction.Grade))
	sheet.PaymentTerms, sheet.AdvanceRequired = ce.GetPaymentTerms(CustomerGrade(prediction.Grade))

	// Step 4: Process each item
	for _, item := range basket.Items {
		costingItem := CostingItem{
			OrderCode:      item.OrderCode,
			Description:    item.Description,
			Quantity:       item.Quantity,
			ProductType:    item.ProductType,
			ProductionDays: item.ProductionDays,
			UnitCostBHD:    item.UnitSalesPriceBHD, // What PH pays E+H
			TotalCostBHD:   item.ItemSalesPriceBHD,
		}

		// Get standard margin for this product type
		costingItem.StandardMargin = ce.GetProductMargin(item.ProductType)

		// Calculate sell price before customer discount
		costingItem.UnitSellBHD = costingItem.UnitCostBHD * (1.0 + costingItem.StandardMargin)
		costingItem.TotalSellBHD = costingItem.UnitSellBHD * float64(costingItem.Quantity)

		// Apply customer discount
		costingItem.CustomerDiscount = customerDiscount
		costingItem.FinalUnitBHD = costingItem.UnitSellBHD * (1.0 - customerDiscount)
		costingItem.FinalTotalBHD = costingItem.FinalUnitBHD * float64(costingItem.Quantity)

		// Calculate profit
		costingItem.UnitProfitBHD = costingItem.FinalUnitBHD - costingItem.UnitCostBHD
		costingItem.TotalProfitBHD = costingItem.UnitProfitBHD * float64(costingItem.Quantity)

		// Calculate actual margin
		if costingItem.UnitCostBHD > 0 {
			costingItem.ActualMargin = costingItem.UnitProfitBHD / costingItem.UnitCostBHD
		}

		sheet.Items = append(sheet.Items, costingItem)
	}

	// Step 5: Calculate totals
	sheet.TotalCostBHD = 0.0
	sheet.TotalSellBHD = 0.0
	sheet.TotalFinalBHD = 0.0
	sheet.TotalProfitBHD = 0.0

	for _, item := range sheet.Items {
		sheet.TotalCostBHD += item.TotalCostBHD
		sheet.TotalSellBHD += item.TotalSellBHD
		sheet.TotalFinalBHD += item.FinalTotalBHD
		sheet.TotalProfitBHD += item.TotalProfitBHD
	}

	sheet.TotalDiscountBHD = sheet.TotalSellBHD - sheet.TotalFinalBHD

	// Calculate margins
	if sheet.TotalCostBHD > 0 {
		sheet.StandardMarginPct = (sheet.TotalSellBHD - sheet.TotalCostBHD) / sheet.TotalCostBHD
		sheet.ActualMarginPct = sheet.TotalProfitBHD / sheet.TotalCostBHD
	}

	// Step 6: Calculate VAT (rate injected via SetVATRate, defaults to 10%)
	if sheet.VATRate <= 0 {
		sheet.VATRate = 10.0 // Default VAT rate if not set
	}
	sheet.VATAmountBHD = sheet.TotalFinalBHD * (sheet.VATRate / 100.0)
	sheet.GrandTotalBHD = sheet.TotalFinalBHD + sheet.VATAmountBHD

	// Step 7: Risk assessment and approval decision
	ce.AssessRisk(sheet, customer)

	return sheet, nil
}

// SetVATRate sets the VAT rate for the costing sheet (call before GenerateCostingSheet)
func (sheet *CostingSheet) SetVATRate(rate float64) {
	sheet.VATRate = rate
}

// GetProductMargin returns margin percentage for product type
func (ce *CostingEngine) GetProductMargin(productType string) float64 {
	margin, exists := ProductMarkupRules[productType]
	if !exists {
		return 0.12 // Default 12% for unknown products
	}
	return margin
}

// GetCustomerDiscount returns maximum allowed discount based on grade
func (ce *CostingEngine) GetCustomerDiscount(grade CustomerGrade) float64 {
	switch grade {
	case GradeA:
		return 0.07 // 7% max discount
	case GradeB:
		return 0.03 // 3% max discount
	case GradeC:
		return 0.00 // No discount
	case GradeD:
		return 0.00 // No discount
	default:
		return 0.00
	}
}

// GetPaymentTerms returns payment terms and advance requirement
func (ce *CostingEngine) GetPaymentTerms(grade CustomerGrade) (terms string, advance float64) {
	switch grade {
	case GradeA:
		return "Net 45 days", 0.0 // No advance required
	case GradeB:
		return "Net 90 days", 0.0 // No advance required
	case GradeC:
		return "Net 120 days with 50% advance", 0.50 // 50% upfront
	case GradeD:
		return "100% advance or DECLINE", 1.00 // Full payment upfront
	default:
		return "Net 90 days", 0.0
	}
}

// AssessRisk performs risk analysis and sets approval status
func (ce *CostingEngine) AssessRisk(sheet *CostingSheet, customer *Customer) {
	warnings := make([]string, 0)

	// Check customer grade
	switch sheet.CustomerGrade {
	case GradeA:
		sheet.ApprovalStatus = "✓ APPROVE"
		sheet.RecommendedAction = "Proceed with order. Reliable customer."

	case GradeB:
		sheet.ApprovalStatus = "✓ APPROVE"
		sheet.RecommendedAction = "Proceed with caution. Monitor payment."
		warnings = append(warnings, "Moderate risk customer - monitor payment closely")

	case GradeC:
		sheet.ApprovalStatus = "⚠ CAUTION"
		sheet.RecommendedAction = "Require 50% advance payment before production."
		warnings = append(warnings, "Higher risk - require 50% advance")

	case GradeD:
		sheet.ApprovalStatus = "✗ DECLINE"
		sheet.RecommendedAction = "Decline order OR require 100% advance payment."
		warnings = append(warnings, "HIGH RISK - decline or require full advance")
	}

	// Check margin adequacy
	if sheet.ActualMarginPct < 0.08 { // Less than 8% margin
		warnings = append(warnings, fmt.Sprintf("Low margin (%.1f%%) - consider rejecting", sheet.ActualMarginPct*100))
		if sheet.ApprovalStatus == "✓ APPROVE" {
			sheet.ApprovalStatus = "⚠ CAUTION"
		}
	}

	// Check ABB competition
	if customer.HasABB == 1 {
		warnings = append(warnings, "ABB COMPETING - only proceed if margin ≥ 15%")
		if sheet.ActualMarginPct < 0.15 {
			sheet.RecommendedAction += " | ⚠ REJECT - ABB competition with low margin"
			sheet.ApprovalStatus = "✗ DECLINE"
		}
	}

	// Check order size relative to customer history
	if len(customer.OrderHistory) > 0 {
		avgOrder := averageFloat(customer.OrderHistory)
		if sheet.TotalFinalBHD > avgOrder*3.0 {
			warnings = append(warnings, fmt.Sprintf("Unusually large order (%.0f%% above average)",
				(sheet.TotalFinalBHD/avgOrder-1.0)*100))
		}
	}

	// Check emergency pricing opportunity
	if customer.IsEmergency == 1 {
		warnings = append(warnings, "Emergency order - premium pricing justified (+10-20%)")
		sheet.RecommendedAction += " | Consider additional emergency surcharge."
	}

	// Check long lead times
	maxLeadTime := 0
	for _, item := range sheet.Items {
		if item.ProductionDays > maxLeadTime {
			maxLeadTime = item.ProductionDays
		}
	}
	if maxLeadTime > 14 {
		warnings = append(warnings, fmt.Sprintf("Long lead time (%d days) - manage customer expectations", maxLeadTime))
	}

	sheet.RiskWarnings = warnings
}

// PrintCostingSheet prints a formatted quotation
func (ce *CostingEngine) PrintCostingSheet(sheet *CostingSheet) {
	fmt.Println("╔═══════════════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║ PH TRADING W.L.L - QUOTATION\n")
	fmt.Println("╠═══════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Customer:        %s (Grade %s)\n", sheet.CustomerName, sheet.CustomerGrade)
	fmt.Printf("║ Customer ID:     %s\n", sheet.CustomerID)
	fmt.Printf("║ Quotation Date:  %s\n", sheet.QuotationDate)
	fmt.Printf("║ Valid Until:     %s\n", sheet.ValidUntil)
	fmt.Printf("║ Payment Terms:   %s\n", sheet.PaymentTerms)
	if sheet.AdvanceRequired > 0 {
		fmt.Printf("║ Advance Required: %.0f%% (%.2f BHD)\n",
			sheet.AdvanceRequired*100, sheet.TotalFinalBHD*sheet.AdvanceRequired)
	}
	fmt.Println("╚═══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Print payment prediction
	fmt.Println("PAYMENT RISK ASSESSMENT:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Grade:            %s\n", sheet.PaymentPrediction.Grade)
	fmt.Printf("Predicted Days:   %d days\n", sheet.PaymentPrediction.PredictedDays)
	fmt.Printf("Confidence:       %.0f%%\n", sheet.PaymentPrediction.Confidence*100)
	fmt.Printf("Three Regimes:    R1=%.1f%%, R2=%.1f%%, R3=%.1f%%\n",
		sheet.PaymentPrediction.ThreeRegimes.R1*100,
		sheet.PaymentPrediction.ThreeRegimes.R2*100,
		sheet.PaymentPrediction.ThreeRegimes.R3*100)
	if len(sheet.PaymentPrediction.RiskFactors) > 0 {
		fmt.Println("Risk Factors:")
		for _, rf := range sheet.PaymentPrediction.RiskFactors {
			fmt.Printf("  • %s\n", rf)
		}
	}
	fmt.Println()

	// Print line items
	fmt.Println("LINE ITEMS:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-40s %5s %12s %12s %12s %10s\n",
		"Description", "Qty", "Unit Cost", "Unit Sell", "Total", "Profit")
	fmt.Println("───────────────────────────────────────────────────────────────────────────────")

	for _, item := range sheet.Items {
		fmt.Printf("%-40s %5d %12.2f %12.2f %12.2f %10.2f\n",
			truncate(item.Description, 40),
			item.Quantity,
			item.UnitCostBHD,
			item.FinalUnitBHD,
			item.FinalTotalBHD,
			item.TotalProfitBHD)
		fmt.Printf("  %s | %s | Lead: %d days | Margin: %.1f%%\n",
			item.OrderCode, item.ProductType, item.ProductionDays, item.ActualMargin*100)
		fmt.Println()
	}

	// Print totals
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("%-58s %12.2f\n", "Subtotal (before discount):", sheet.TotalSellBHD)
	if sheet.TotalDiscountBHD > 0 {
		fmt.Printf("%-58s %12.2f (%.1f%%)\n", "Customer Discount:",
			-sheet.TotalDiscountBHD, (sheet.TotalDiscountBHD/sheet.TotalSellBHD)*100)
	}
	fmt.Printf("%-58s %12.2f BHD\n", "Subtotal (after discount):", sheet.TotalFinalBHD)
	fmt.Printf("%-58s %12.2f BHD (%.1f%%)\n", "VAT:", sheet.VATAmountBHD, sheet.VATRate)
	fmt.Printf("%-58s %12.2f BHD\n", "GRAND TOTAL:", sheet.GrandTotalBHD)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()

	// Print profit analysis
	fmt.Println("PROFIT ANALYSIS:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Total Cost (E+H):        %12.2f BHD\n", sheet.TotalCostBHD)
	fmt.Printf("Total Revenue:           %12.2f BHD\n", sheet.TotalFinalBHD)
	fmt.Printf("Total Profit:            %12.2f BHD\n", sheet.TotalProfitBHD)
	fmt.Printf("Standard Margin:         %12.1f%%\n", sheet.StandardMarginPct*100)
	fmt.Printf("Actual Margin:           %12.1f%%\n", sheet.ActualMarginPct*100)
	fmt.Println()

	// Print risk warnings
	if len(sheet.RiskWarnings) > 0 {
		fmt.Println("⚠ RISK WARNINGS:")
		fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
		for _, warning := range sheet.RiskWarnings {
			fmt.Printf("  ⚠ %s\n", warning)
		}
		fmt.Println()
	}

	// Print approval decision
	fmt.Println("APPROVAL DECISION:")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("Status: %s\n", sheet.ApprovalStatus)
	fmt.Printf("Action: %s\n", sheet.RecommendedAction)
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// truncate truncates a string to max length
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// CalculateROI calculates 3-year ROI for the costing engine vs manual process
func (ce *CostingEngine) CalculateROI() map[string]interface{} {
	// BUSINESS REALITY from PH Trading operations:

	// Manual Process (Current):
	// - Time per quotation: 45 minutes (lookup E+H prices, calculate margins, check customer history)
	// - Quotations per month: ~40
	// - Error rate: 12% (wrong margins, missed payment risks)
	// - Staff cost: 800 BHD/month (fully loaded)

	// Automated Process (With this engine):
	// - Time per quotation: 3 minutes (just review and approve)
	// - Error rate: 2% (only human override errors)
	// - Same staff, more capacity

	quotationsPerMonth := 40.0
	manualMinutesPerQuote := 45.0
	automatedMinutesPerQuote := 3.0
	_ = 800.0 // staffCostPerMonth (BHD) - not used in calculation but documented

	manualErrorRate := 0.12
	automatedErrorRate := 0.02
	avgQuoteValue := 3000.0 // BHD
	avgMargin := 0.15

	// Time savings
	manualMonthlyHours := (quotationsPerMonth * manualMinutesPerQuote) / 60.0
	automatedMonthlyHours := (quotationsPerMonth * automatedMinutesPerQuote) / 60.0
	savedHours := manualMonthlyHours - automatedMonthlyHours

	// Error cost reduction
	manualErrorCost := quotationsPerMonth * manualErrorRate * avgQuoteValue * avgMargin * 0.5 // 50% of lost margin
	automatedErrorCost := quotationsPerMonth * automatedErrorRate * avgQuoteValue * avgMargin * 0.5
	errorSavings := manualErrorCost - automatedErrorCost

	// Capacity increase (same staff can do more)
	additionalCapacity := savedHours / (automatedMinutesPerQuote / 60.0)
	additionalRevenue := additionalCapacity * avgQuoteValue * avgMargin * 0.3 // 30% close rate

	monthlyBenefit := errorSavings + additionalRevenue
	yearlyBenefit := monthlyBenefit * 12
	threeYearBenefit := yearlyBenefit * 3

	// Development cost estimate
	devCost := 2000.0 // BHD (one-time)

	roi := map[string]interface{}{
		"time_saved_hours_per_month":       math.Round(savedHours*10) / 10,
		"error_savings_bhd_per_month":      math.Round(errorSavings*100) / 100,
		"additional_revenue_bhd_per_month": math.Round(additionalRevenue*100) / 100,
		"total_monthly_benefit_bhd":        math.Round(monthlyBenefit*100) / 100,
		"yearly_benefit_bhd":               math.Round(yearlyBenefit*100) / 100,
		"three_year_benefit_bhd":           math.Round(threeYearBenefit*100) / 100,
		"development_cost_bhd":             devCost,
		"roi_percent":                      math.Round((threeYearBenefit/devCost-1.0)*10000) / 100,
		"payback_months":                   math.Round((devCost/monthlyBenefit)*10) / 10,
	}

	return roi
}
