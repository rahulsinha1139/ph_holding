// ═══════════════════════════════════════════════════════════════════════════
// COSTING ENGINE - Quotation Generation with Margins and Risk Assessment
//
// MISSION: Calculate customer quotations with margins and risk adjustments
//
// FEATURES:
//   - Product-specific markup rules (E+H Flow, Level, Pressure, etc.)
//   - Customer grade-based discounts (A/B/C/D)
//   - Payment terms based on risk assessment
//   - Profit analysis and margin calculations
//   - Risk warnings and approval recommendations
//
// Built with MATHEMATICAL RIGOR × PRODUCTION ROBUSTNESS
// ═══════════════════════════════════════════════════════════════════════════

package engines

import (
	"fmt"
	"math"
	"time"
)

// ProductMarkupRules defines markup percentages by product type
var ProductMarkupRules = map[string]float64{
	"E+H Flow":        0.15, // 15% margin
	"E+H Level":       0.18, // 18% margin
	"E+H Pressure":    0.18, // 18% margin
	"E+H Temperature": 0.15, // 15% margin
	"E+H Analytics":   0.20, // 20% margin
	"E+H General":     0.12, // 12% margin
	"Servomex":        0.25, // 25% margin
	"GIC":             0.10, // 10% margin
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
	Parser *EHParser
}

// NewCostingEngine creates a new costing engine
func NewCostingEngine() *CostingEngine {
	return &CostingEngine{
		Parser: NewEHParser(),
	}
}

// CostingItem represents a single line item with markup applied
type CostingItem struct {
	OrderCode        string  `json:"order_code"`
	Description      string  `json:"description"`
	Quantity         int     `json:"quantity"`
	ProductType      string  `json:"product_type"`
	ProductionDays   int     `json:"production_days"`
	UnitCostBHD      float64 `json:"unit_cost_bhd"`
	TotalCostBHD     float64 `json:"total_cost_bhd"`
	StandardMargin   float64 `json:"standard_margin"`
	UnitSellBHD      float64 `json:"unit_sell_bhd"`
	TotalSellBHD     float64 `json:"total_sell_bhd"`
	CustomerDiscount float64 `json:"customer_discount"`
	FinalUnitBHD     float64 `json:"final_unit_bhd"`
	FinalTotalBHD    float64 `json:"final_total_bhd"`
	UnitProfitBHD    float64 `json:"unit_profit_bhd"`
	TotalProfitBHD   float64 `json:"total_profit_bhd"`
	ActualMargin     float64 `json:"actual_margin"`
}

// CostingSheet represents complete quotation
type CostingSheet struct {
	CustomerID        string        `json:"customer_id"`
	CustomerName      string        `json:"customer_name"`
	CustomerGrade     CustomerGrade `json:"customer_grade"`
	QuotationDate     string        `json:"quotation_date"`
	ValidUntil        string        `json:"valid_until"`
	SourceBasket      string        `json:"source_basket"`
	Items             []CostingItem `json:"items"`
	TotalCostBHD      float64       `json:"total_cost_bhd"`
	TotalSellBHD      float64       `json:"total_sell_bhd"`
	TotalDiscountBHD  float64       `json:"total_discount_bhd"`
	TotalFinalBHD     float64       `json:"total_final_bhd"`
	TotalProfitBHD    float64       `json:"total_profit_bhd"`
	StandardMarginPct float64       `json:"standard_margin_pct"`
	ActualMarginPct   float64       `json:"actual_margin_pct"`
	PaymentTerms      string        `json:"payment_terms"`
	AdvanceRequired   float64       `json:"advance_required"`
	ApprovalStatus    string        `json:"approval_status"`
	RiskWarnings      []string      `json:"risk_warnings"`
	RecommendedAction string        `json:"recommended_action"`
}

// GetProductMargin returns margin percentage for product type
func (ce *CostingEngine) GetProductMargin(productType string) float64 {
	margin, exists := ProductMarkupRules[productType]
	if !exists {
		return 0.12
	}
	return margin
}

// GetCustomerDiscount returns maximum allowed discount based on grade
func (ce *CostingEngine) GetCustomerDiscount(grade CustomerGrade) float64 {
	switch grade {
	case GradeA:
		return 0.07
	case GradeB:
		return 0.03
	case GradeC:
		return 0.00
	case GradeD:
		return 0.00
	default:
		return 0.00
	}
}

// GetPaymentTerms returns payment terms and advance requirement
func (ce *CostingEngine) GetPaymentTerms(grade CustomerGrade) (terms string, advance float64) {
	switch grade {
	case GradeA:
		return "Net 45 days", 0.0
	case GradeB:
		return "Net 90 days", 0.0
	case GradeC:
		return "Net 120 days with 50% advance", 0.50
	case GradeD:
		return "100% advance or DECLINE", 1.00
	default:
		return "Net 90 days", 0.0
	}
}

// GenerateCostingFromBasket creates a costing sheet from E+H basket
func (ce *CostingEngine) GenerateCostingFromBasket(basket *ParsedEHBasket, customerName string, grade CustomerGrade) *CostingSheet {
	sheet := &CostingSheet{
		CustomerName:  customerName,
		CustomerGrade: grade,
		QuotationDate: time.Now().UTC().Format("2006-01-02"),
		ValidUntil:    time.Now().UTC().AddDate(0, 0, 30).Format("2006-01-02"),
		SourceBasket:  basket.SourceFile,
		Items:         make([]CostingItem, 0, len(basket.Items)),
		RiskWarnings:  make([]string, 0),
	}

	customerDiscount := ce.GetCustomerDiscount(grade)
	sheet.PaymentTerms, sheet.AdvanceRequired = ce.GetPaymentTerms(grade)

	for _, item := range basket.Items {
		costingItem := CostingItem{
			OrderCode:      item.OrderCode,
			Description:    item.Description,
			Quantity:       item.Quantity,
			ProductType:    item.ProductType,
			ProductionDays: item.ProductionDays,
			UnitCostBHD:    item.UnitSalesPriceBHD,
			TotalCostBHD:   item.ItemSalesPriceBHD,
		}

		costingItem.StandardMargin = ce.GetProductMargin(item.ProductType)
		costingItem.UnitSellBHD = costingItem.UnitCostBHD * (1.0 + costingItem.StandardMargin)
		costingItem.TotalSellBHD = costingItem.UnitSellBHD * float64(costingItem.Quantity)
		costingItem.CustomerDiscount = customerDiscount
		costingItem.FinalUnitBHD = costingItem.UnitSellBHD * (1.0 - customerDiscount)
		costingItem.FinalTotalBHD = costingItem.FinalUnitBHD * float64(costingItem.Quantity)
		costingItem.UnitProfitBHD = costingItem.FinalUnitBHD - costingItem.UnitCostBHD
		costingItem.TotalProfitBHD = costingItem.UnitProfitBHD * float64(costingItem.Quantity)

		if costingItem.UnitCostBHD > 0 {
			costingItem.ActualMargin = costingItem.UnitProfitBHD / costingItem.UnitCostBHD
		}

		sheet.Items = append(sheet.Items, costingItem)
	}

	// Calculate totals
	for _, item := range sheet.Items {
		sheet.TotalCostBHD += item.TotalCostBHD
		sheet.TotalSellBHD += item.TotalSellBHD
		sheet.TotalFinalBHD += item.FinalTotalBHD
		sheet.TotalProfitBHD += item.TotalProfitBHD
	}

	sheet.TotalDiscountBHD = sheet.TotalSellBHD - sheet.TotalFinalBHD

	if sheet.TotalCostBHD > 0 {
		sheet.StandardMarginPct = (sheet.TotalSellBHD - sheet.TotalCostBHD) / sheet.TotalCostBHD
		sheet.ActualMarginPct = sheet.TotalProfitBHD / sheet.TotalCostBHD
	}

	// Risk assessment
	ce.AssessRisk(sheet)

	return sheet
}

// AssessRisk performs risk analysis and sets approval status
func (ce *CostingEngine) AssessRisk(sheet *CostingSheet) {
	switch sheet.CustomerGrade {
	case GradeA:
		sheet.ApprovalStatus = "✓ APPROVE"
		sheet.RecommendedAction = "Proceed with order. Reliable customer."
	case GradeB:
		sheet.ApprovalStatus = "✓ APPROVE"
		sheet.RecommendedAction = "Proceed with caution. Monitor payment."
		sheet.RiskWarnings = append(sheet.RiskWarnings, "Moderate risk - monitor payment")
	case GradeC:
		sheet.ApprovalStatus = "⚠ CAUTION"
		sheet.RecommendedAction = "Require 50% advance payment."
		sheet.RiskWarnings = append(sheet.RiskWarnings, "Higher risk - require 50% advance")
	case GradeD:
		sheet.ApprovalStatus = "✗ DECLINE"
		sheet.RecommendedAction = "Decline or require 100% advance."
		sheet.RiskWarnings = append(sheet.RiskWarnings, "HIGH RISK - decline or full advance")
	}

	if sheet.ActualMarginPct < 0.08 {
		sheet.RiskWarnings = append(sheet.RiskWarnings,
			fmt.Sprintf("Low margin (%.1f%%) - consider rejecting", sheet.ActualMarginPct*100))
	}
}

// CalculateROI calculates 3-year ROI for the costing engine
func (ce *CostingEngine) CalculateROI() map[string]interface{} {
	quotationsPerMonth := 40.0
	manualMinutesPerQuote := 45.0
	automatedMinutesPerQuote := 3.0

	manualErrorRate := 0.12
	automatedErrorRate := 0.02
	avgQuoteValue := 3000.0
	avgMargin := 0.15

	manualMonthlyHours := (quotationsPerMonth * manualMinutesPerQuote) / 60.0
	automatedMonthlyHours := (quotationsPerMonth * automatedMinutesPerQuote) / 60.0
	savedHours := manualMonthlyHours - automatedMonthlyHours

	manualErrorCost := quotationsPerMonth * manualErrorRate * avgQuoteValue * avgMargin * 0.5
	automatedErrorCost := quotationsPerMonth * automatedErrorRate * avgQuoteValue * avgMargin * 0.5
	errorSavings := manualErrorCost - automatedErrorCost

	additionalCapacity := savedHours / (automatedMinutesPerQuote / 60.0)
	additionalRevenue := additionalCapacity * avgQuoteValue * avgMargin * 0.3

	monthlyBenefit := errorSavings + additionalRevenue
	yearlyBenefit := monthlyBenefit * 12
	threeYearBenefit := yearlyBenefit * 3
	devCost := 2000.0

	return map[string]interface{}{
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
}
