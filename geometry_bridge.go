// ═══════════════════════════════════════════════════════════════════════════
// GEOMETRY BRIDGE - Wails App ↔ Geometry Pipelines Integration
//
// MISSION: Bridge PH Holdings Wails app to 4 geometric pipelines
//
// ARCHITECTURE:
//   1. Import geometry_erp packages
//   2. Route business events to correct pipeline
//   3. Translate domain objects (Invoice, Tender, Customer) to events
//   4. Return pipeline results to Wails frontend
//
// ROUTING LOGIC:
//   Invoice events      → Oblate Spheroid (Navier-Stokes flows)
//   Tender/RFQ events   → Truncated Icosahedron (Constraint satisfaction)
//   Customer events     → Quaternionic S³ (Entanglement, 360°)
//   Compliance events   → Banach Ball (Bounded convergence)
//
// Built with MATHEMATICAL RIGOR × E2E INTEGRATION × ZERO CRUFT 🕉️⚡💎
// Day 193 - Pipeline Integration Mission
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"
	"math"
	"time"
	// NOTE: Full implementations exist in geometry_erp package
	// For initial integration, using mock adapters defined in pipeline_adapters.go
	// geometry "asymm_all_math/asymm_mathematical_organism/05_ORGANS/geometry_erp"
	// "asymm_all_math/asymm_mathematical_organism/05_ORGANS/geometry_erp/pipelines"
)

// ============================================================================
// CORE TYPES (For Geometry Pipelines)
// ============================================================================
// NOTE: These are simplified types for geometry processing
// The database.go file has the full GORM models

// InvoiceGeometry represents a simplified invoice for geometry processing
type InvoiceGeometry struct {
	ID              string    `json:"id"`
	CustomerID      string    `json:"customer_id"`
	Amount          float64   `json:"amount"` // BHD
	IssueDate       time.Time `json:"issue_date"`
	DueDate         time.Time `json:"due_date"`
	PaymentDate     time.Time `json:"payment_date"` // Zero if unpaid
	Status          string    `json:"status"`       // "pending", "paid", "overdue"
	Currency        string    `json:"currency"`     // "BHD"
	ItemCount       int       `json:"item_count"`
	DiscountApplied float64   `json:"discount_applied"` // Percentage
}

// TenderGeometry represents an RFQ (Request for Quotation)
type TenderGeometry struct {
	ID           string       `json:"id"`
	CustomerID   string       `json:"customer_id"`
	Description  string       `json:"description"`
	Items        []TenderItem `json:"items"`
	Deadline     time.Time    `json:"deadline"`
	Budget       float64      `json:"budget"`        // Max budget
	IsABB        bool         `json:"is_abb"`        // ABB competing?
	IsEmergency  bool         `json:"is_emergency"`  // Urgent RFQ?
	RequiredDate time.Time    `json:"required_date"` // Delivery date
}

// TenderItem represents a line item in tender
type TenderItem struct {
	ProductID   string  `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	UnitPrice   float64 `json:"unit_price"` // Requested price
	Margin      float64 `json:"margin"`     // Target margin %
}

// ComplianceData represents compliance check input
type ComplianceData struct {
	Type      string                 `json:"type"`      // "contract", "invoice", "tender"
	Data      map[string]interface{} `json:"data"`      // Flexible data
	RuleSet   string                 `json:"rule_set"`  // "bahrain", "gcc", "iso"
	Threshold float64                `json:"threshold"` // Tolerance
}

// ============================================================================
// RESULT TYPES
// ============================================================================

// InvoiceResult represents invoice processing outcome
type InvoiceResult struct {
	InvoiceID       string             `json:"invoice_id"`
	FlowAnalysis    map[string]float64 `json:"flow_analysis"`  // Navier-Stokes stats
	Reconciled      bool               `json:"reconciled"`     // Equilibrium reached?
	PredictedDays   int                `json:"predicted_days"` // Payment forecast
	Confidence      float64            `json:"confidence"`     // [0, 1]
	Recommendations []string           `json:"recommendations"`
	Timestamp       string             `json:"timestamp"`
}

// TenderResult represents tender processing outcome
type TenderResult struct {
	TenderID       string       `json:"tender_id"`
	Feasible       bool         `json:"feasible"`      // Satisfiable?
	OptimalQuote   float64      `json:"optimal_quote"` // Best quote amount
	Margin         float64      `json:"margin"`        // Achieved margin
	MatchedItems   []TenderItem `json:"matched_items"`
	Constraints    int          `json:"constraints"`    // Total constraints
	Satisfied      int          `json:"satisfied"`      // Constraints met
	Recommendation string       `json:"recommendation"` // "BID", "DECLINE", "NEGOTIATE"
	ABBWarning     bool         `json:"abb_warning"`    // ABB risk detected
	Timestamp      string       `json:"timestamp"`
}

// Customer360 represents multi-view customer profile
type Customer360 struct {
	CustomerID     string                 `json:"customer_id"`
	BusinessName   string                 `json:"business_name"`
	TotalOrders    int                    `json:"total_orders"`
	TotalValue     float64                `json:"total_value"` // BHD
	AvgPaymentDays float64                `json:"avg_payment_days"`
	Grade          string                 `json:"grade"`        // A/B/C/D
	Entanglement   map[string]interface{} `json:"entanglement"` // Multi-view data
	RiskFactors    []string               `json:"risk_factors"`
	RelationYears  int                    `json:"relation_years"`
	LastContact    time.Time              `json:"last_contact"`
	Timestamp      string                 `json:"timestamp"`
}

// ComplianceResult represents compliance check outcome
type ComplianceResult struct {
	Compliant       bool     `json:"compliant"`
	Score           float64  `json:"score"` // [0, 1]
	Violations      []string `json:"violations"`
	Convergence     bool     `json:"convergence"` // Banach convergence
	Iterations      int      `json:"iterations"`
	Recommendations []string `json:"recommendations"`
	Timestamp       string   `json:"timestamp"`
}

// ERPEvent represents a generic ERP business event
type ERPEvent struct {
	Type      string                 `json:"type"`     // "invoice", "tender", "customer", etc.
	Data      map[string]interface{} `json:"data"`     // Flexible payload
	Source    string                 `json:"source"`   // "api", "file", "manual"
	Priority  float64                `json:"priority"` // 0.0-1.0
	Timestamp string                 `json:"timestamp"`
}

// RoutingResult shows which pipeline was selected
type RoutingResult struct {
	EventID      string `json:"event_id"`
	Geometry     string `json:"geometry"` // "OblateSpheroid", etc.
	ThreeRegimes struct {
		R1 float64 `json:"r1"`
		R2 float64 `json:"r2"`
		R3 float64 `json:"r3"`
	} `json:"three_regimes"`
	Difficulty  float64 `json:"difficulty"` // 0.0-1.0
	RouteReason string  `json:"route_reason"`
	Timestamp   string  `json:"timestamp"`
}

// ============================================================================
// GEOMETRY BRIDGE
// ============================================================================

type GeometryBridge struct {
	// Pipeline instances (using mock adapters for initial integration)
	oblateSpheroid       *MockOblateSpheroidPipeline
	truncatedIcosahedron *MockTruncatedIcosahedronPipeline
	quaternionicS3       *MockQuaternionicS3Pipeline
	banachBall           *MockBanachBallPipeline

	// Statistics
	totalEvents    int
	routingHistory []RoutingResult
}

// NewGeometryBridge creates a new bridge instance
func NewGeometryBridge() *GeometryBridge {
	return &GeometryBridge{
		totalEvents:    0,
		routingHistory: make([]RoutingResult, 0),
	}
}

// ============================================================================
// INVOICE PROCESSING (Oblate Spheroid Pipeline)
// ============================================================================

// ProcessInvoice routes invoice to Oblate Spheroid pipeline
// MATHEMATICAL FOUNDATION: Fractional Navier-Stokes flow (cashflow = fluid dynamics)
func (gb *GeometryBridge) ProcessInvoice(invoice InvoiceGeometry) (*InvoiceResult, error) {
	// Initialize Oblate Spheroid pipeline if needed
	if gb.oblateSpheroid == nil {
		// Create 20x20 grid flow field with default parameters
		params := OblateSpheroidParams{
			Viscosity: 0.01, // Transaction friction
			Density:   1.0,  // Money density
			Beta:      0.8,  // Invoice flow (moderate memory)
		}
		gb.oblateSpheroid = NewMockOblateSpheroidPipeline(20, 20, params)
	}

	// Map invoice to flow field position
	// Amount determines x-position (normalized to [0,1])
	// Age determines y-position
	posX := normalizeAmount(invoice.Amount)
	daysSince := daysSinceIssue(invoice)
	posY := math.Min(float64(daysSince)/180.0, 1.0) // Normalize age to [0,1]

	// Add invoice to flow system
	gb.oblateSpheroid.AddInvoice(invoice.ID, invoice.Amount, posX, posY)

	// Evolve flow system (simulate payment dynamics)
	for i := 0; i < 20; i++ {
		gb.oblateSpheroid.Evolve(0.01)
	}

	// Get flow statistics
	stats := gb.oblateSpheroid.GetFlowStatistics()

	// Forecast cashflow (predict payment time)
	forecast := gb.oblateSpheroid.ForecastCashflow([2]float64{posX, posY}, 1.0)

	// Predicted days based on turbulence
	// Low turbulence = fast payment, High turbulence = slow payment
	predictedDays := int(45.0 + forecast.Turbulence*60.0) // 45-105 days range
	if predictedDays < 30 {
		predictedDays = 30
	}
	if predictedDays > 180 {
		predictedDays = 180
	}

	// Confidence = forecast confidence
	confidence := forecast.Confidence
	if confidence > 1.0 {
		confidence = 1.0
	}
	if confidence < 0.3 {
		confidence = 0.3
	}

	// Check for equilibrium (reconciled invoices)
	reconciled := forecast.Turbulence < 0.1 // Low turbulence = equilibrium

	// Generate recommendations
	recommendations := make([]string, 0)
	if invoice.Amount > 20000 {
		recommendations = append(recommendations, "HIGH VALUE: Priority collection recommended")
	}
	if forecast.Turbulence > 0.5 {
		recommendations = append(recommendations, "HIGH TURBULENCE: Cash flow risk detected")
	}
	if confidence > 0.75 {
		recommendations = append(recommendations, "✓ STABLE: Good payment forecast")
	}
	if reconciled {
		recommendations = append(recommendations, "✓ EQUILIBRIUM: Invoice ready for reconciliation")
	}

	result := &InvoiceResult{
		InvoiceID:       invoice.ID,
		FlowAnalysis:    stats,
		Reconciled:      reconciled,
		PredictedDays:   predictedDays,
		Confidence:      confidence,
		Recommendations: recommendations,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	gb.totalEvents++
	return result, nil
}

// ============================================================================
// TENDER PROCESSING (Truncated Icosahedron Pipeline)
// ============================================================================

// ProcessTender routes tender to Truncated Icosahedron pipeline
func (gb *GeometryBridge) ProcessTender(tender TenderGeometry) (*TenderResult, error) {
	// Initialize pipeline if needed
	if gb.truncatedIcosahedron == nil {
		gb.truncatedIcosahedron = NewMockTruncatedIcosahedronPipeline()
	}

	// Build constraint problem
	constraints := make([]PipelineConstraint, 0)

	// Budget constraint
	totalCost := 0.0
	for _, item := range tender.Items {
		totalCost += item.UnitPrice * float64(item.Quantity)

		// Per-item margin constraint
		minMargin := 0.10 // 10% minimum
		if tender.IsABB {
			minMargin = 0.15 // 15% if ABB competing
		}
		constraints = append(constraints, PipelineConstraint{
			Type:   "margin",
			Value:  item.Margin,
			Target: minMargin,
		})
	}

	// Total budget constraint
	constraints = append(constraints, PipelineConstraint{
		Type:   "budget",
		Value:  totalCost,
		Target: tender.Budget,
	})

	// Deadline constraint
	daysToDeadline := time.Until(tender.Deadline).Hours() / 24.0
	constraints = append(constraints, PipelineConstraint{
		Type:   "deadline",
		Value:  daysToDeadline,
		Target: 7.0, // Minimum 7 days for preparation
	})

	// Solve constraint satisfaction problem
	solution := gb.truncatedIcosahedron.Solve(constraints)

	// Calculate optimal quote based on achieved margins
	optimalQuote := 0.0
	totalMarginWeighted := 0.0

	if solution.Feasible {
		// For each item, calculate optimal selling price based on item's margin
		for _, item := range tender.Items {
			itemCost := item.UnitPrice * float64(item.Quantity)
			// Apply the item's margin to get selling price
			itemSellingPrice := itemCost * (1.0 + item.Margin)
			optimalQuote += itemSellingPrice
			totalMarginWeighted += item.Margin * itemCost
		}
	} else {
		// If not feasible, quote at cost
		optimalQuote = totalCost
	}

	// Calculate achieved margin as weighted average
	achievedMargin := 0.0
	if totalCost > 0 {
		achievedMargin = (optimalQuote - totalCost) / totalCost * 100.0
	}

	// Generate recommendation
	recommendation := "DECLINE"
	if solution.Feasible && achievedMargin >= 10.0 {
		recommendation = "BID"
	} else if solution.Feasible && achievedMargin >= 5.0 {
		recommendation = "NEGOTIATE"
	}

	// ABB warning
	abbWarning := false
	if tender.IsABB && achievedMargin < 15.0 {
		abbWarning = true
		recommendation = "DECLINE - ABB margin too low"
	}

	result := &TenderResult{
		TenderID:       tender.ID,
		Feasible:       solution.Feasible,
		OptimalQuote:   optimalQuote,
		Margin:         achievedMargin,
		MatchedItems:   tender.Items,
		Constraints:    len(constraints),
		Satisfied:      solution.SatisfiedCount,
		Recommendation: recommendation,
		ABBWarning:     abbWarning,
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	gb.totalEvents++
	return result, nil
}

// ============================================================================
// CUSTOMER 360° (Quaternionic S³ Pipeline)
// ============================================================================

// GetCustomer360 retrieves entangled customer profile
func (gb *GeometryBridge) GetCustomer360(customerID string, customer *Customer) (*Customer360, error) {
	// Initialize pipeline if needed
	if gb.quaternionicS3 == nil {
		gb.quaternionicS3 = NewMockQuaternionicS3Pipeline()
	}

	// Create multi-view entanglement
	// View 1: Payment behavior
	// View 2: Order history
	// View 3: Relationship quality
	// View 4: Risk profile

	avgPayment := averageInt(customer.PaymentHistory)
	avgOrder := averageFloat(customer.OrderHistory)
	riskScore := calculateRiskScore(customer)

	entanglement := map[string]interface{}{
		"payment_view": map[string]interface{}{
			"avg_days":    avgPayment,
			"consistency": computeConsistency(customer.PaymentHistory),
			"trend":       computeTrend(customer.PaymentHistory),
		},
		"order_view": map[string]interface{}{
			"avg_value":   avgOrder,
			"frequency":   len(customer.OrderHistory),
			"growth_rate": computeGrowthRate(customer.OrderHistory),
		},
		"relationship_view": map[string]interface{}{
			"years":    customer.RelationYears,
			"disputes": customer.DisputeCount,
			"industry": customer.Industry,
			"country":  customer.Country,
		},
		"risk_view": map[string]interface{}{
			"score":      riskScore,
			"emergency":  customer.IsEmergency,
			"abb_factor": customer.HasABB,
			"grade":      determineGrade(customer),
		},
	}

	// Quantum entanglement correlation
	// Non-commutative joins (order matters!)
	_ = gb.quaternionicS3.ComputeEntanglement(entanglement) // Correlation computed

	// Extract risk factors
	riskFactors := make([]string, 0)
	if customer.DisputeCount > 2 {
		riskFactors = append(riskFactors, fmt.Sprintf("High disputes (%d)", customer.DisputeCount))
	}
	if customer.HasABB == 1 {
		riskFactors = append(riskFactors, "ABB competition detected")
	}
	if avgPayment > 90 {
		riskFactors = append(riskFactors, fmt.Sprintf("Slow payment (%.0f days avg)", avgPayment))
	}

	result := &Customer360{
		CustomerID:     customerID,
		BusinessName:   customer.BusinessName,
		TotalOrders:    len(customer.OrderHistory),
		TotalValue:     sumFloat(customer.OrderHistory),
		AvgPaymentDays: avgPayment,
		Grade:          determineGrade(customer),
		Entanglement:   entanglement,
		RiskFactors:    riskFactors,
		RelationYears:  customer.RelationYears,
		LastContact:    time.Now(), // Would come from CRM
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
	}

	gb.totalEvents++
	return result, nil
}

// ============================================================================
// COMPLIANCE CHECK (Banach Ball Pipeline)
// ============================================================================

// CheckCompliance validates data against rules using Banach convergence
func (gb *GeometryBridge) CheckCompliance(data ComplianceData) (*ComplianceResult, error) {
	// Initialize pipeline if needed
	if gb.banachBall == nil {
		gb.banachBall = NewMockBanachBallPipeline()
	}

	// Convert data to normed vector
	dataVector := convertToNormedVector(data.Data)

	// Define contraction mapping (compliance rules)
	ruleVector := getRuleVector(data.RuleSet)

	// Iterate to fixed point (convergence = compliant!)
	maxIter := 50
	converged, iterations := gb.banachBall.IterateToFixedPoint(dataVector, ruleVector, maxIter, data.Threshold)

	// Compute compliance score - normalize distance to [0, 1] scale
	distance := computeNormDistance(dataVector, ruleVector)
	score := 1.0 - distance
	if score < 0 {
		score = 0
	}
	if score > 1.0 {
		score = 1.0
	}

	// If vectors are empty or incompatible, use convergence as score
	if len(dataVector) == 0 || len(ruleVector) == 0 || len(dataVector) != len(ruleVector) {
		if converged {
			score = 0.95
		} else {
			score = 0.50
		}
	}

	// Check violations
	violations := make([]string, 0)
	if score < 0.90 {
		violations = append(violations, "Score below 90% threshold")
	}
	if !converged {
		violations = append(violations, "Failed to converge to compliant state")
	}

	// Recommendations
	recommendations := make([]string, 0)
	if !converged {
		recommendations = append(recommendations, "Manual review required - no convergence")
	}
	if score >= 0.95 {
		recommendations = append(recommendations, "✓ COMPLIANT - Auto-approve")
	} else if score >= 0.85 {
		recommendations = append(recommendations, "⚠ MARGINAL - Supervisor review")
	} else {
		recommendations = append(recommendations, "✗ NON-COMPLIANT - Reject or remediate")
	}

	result := &ComplianceResult{
		Compliant:       score >= 0.90 && converged,
		Score:           score,
		Violations:      violations,
		Convergence:     converged,
		Iterations:      iterations,
		Recommendations: recommendations,
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
	}

	gb.totalEvents++
	return result, nil
}

// ============================================================================
// GENERIC EVENT ROUTING
// ============================================================================

// RouteEvent routes any event to appropriate pipeline
func (gb *GeometryBridge) RouteEvent(event ERPEvent) (*RoutingResult, error) {
	// Mock routing for initial integration
	// TODO: Replace with full geometry classification

	// Simple routing based on event type
	geometry := "Unknown"
	r1, r2, r3 := 30.0, 20.0, 50.0 // Default three-regime

	switch event.Type {
	case "invoice":
		geometry = "OblateSpheroid"
		r1, r2, r3 = 35.0, 15.0, 50.0
	case "tender":
		geometry = "TruncatedIcosahedron"
		r1, r2, r3 = 45.0, 10.0, 45.0
	case "customer":
		geometry = "QuaternionicS3"
		r1, r2, r3 = 30.0, 20.0, 50.0
	case "compliance":
		geometry = "BanachBall"
		r1, r2, r3 = 40.0, 25.0, 35.0
	}

	difficulty := (r1*0.5 + r2*1.5 + (100.0-r3)*0.8) / 100.0

	routeReason := fmt.Sprintf("Three-regime signature [R1=%.1f%%, R2=%.1f%%, R3=%.1f%%] → %s",
		r1, r2, r3, geometry)

	result := &RoutingResult{
		EventID:     fmt.Sprintf("evt_%s_%d", event.Type, gb.totalEvents),
		Geometry:    geometry,
		Difficulty:  difficulty,
		RouteReason: routeReason,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	result.ThreeRegimes.R1 = r1
	result.ThreeRegimes.R2 = r2
	result.ThreeRegimes.R3 = r3

	// Save to history
	gb.routingHistory = append(gb.routingHistory, *result)
	gb.totalEvents++

	return result, nil
}

// ============================================================================
// UTILITY FUNCTIONS
// ============================================================================

func calculateInvoicePriority(inv InvoiceGeometry) float64 {
	// Priority based on amount and status
	priority := 0.5
	if inv.Amount > 10000 {
		priority += 0.2
	}
	if inv.Status == "overdue" {
		priority += 0.3
	}
	if priority > 1.0 {
		priority = 1.0
	}
	return priority
}

func calculateTenderPriority(tender TenderGeometry) float64 {
	priority := 0.5
	if tender.IsEmergency {
		priority += 0.3
	}
	if tender.Budget > 50000 {
		priority += 0.2
	}
	if priority > 1.0 {
		priority = 1.0
	}
	return priority
}

func normalizeAmount(amount float64) float64 {
	// Normalize to [0, 1] using sigmoid
	return 1.0 / (1.0 + math.Exp(-amount/10000.0))
}

func daysSinceIssue(inv InvoiceGeometry) int {
	return int(time.Since(inv.IssueDate).Hours() / 24.0)
}

func calculateRiskScore(customer *Customer) float64 {
	score := 0.5

	// Negative factors
	if customer.DisputeCount > 2 {
		score += 0.2
	}
	if customer.HasABB == 1 {
		score += 0.1
	}
	avgPayment := averageInt(customer.PaymentHistory)
	if avgPayment > 90 {
		score += 0.15
	}

	// Positive factors
	if customer.RelationYears >= 5 {
		score -= 0.2
	}
	if customer.IsEmergency == 1 {
		score -= 0.1
	}

	if score < 0 {
		score = 0
	}
	if score > 1.0 {
		score = 1.0
	}

	return score
}

func determineGrade(customer *Customer) string {
	risk := calculateRiskScore(customer)
	avgPayment := averageInt(customer.PaymentHistory)

	// Enhanced grading logic:
	// - Payment history: <45 days=excellent, 45-75=good, 75-105=caution, >105=risk
	// - Relationship years: >=5=trusted, 3-4=established, 1-2=new, 0=untested
	// - Disputes: 0-1=clean, 2-3=moderate, 4+=problematic

	score := 0.0

	// Payment speed (50% weight)
	if avgPayment < 50 {
		score += 5.0
	} else if avgPayment < 80 {
		score += 3.5
	} else if avgPayment < 110 {
		score += 2.0
	} else {
		score += 0.5
	}

	// Relationship (25% weight)
	if customer.RelationYears >= 5 {
		score += 2.5
	} else if customer.RelationYears >= 3 {
		score += 1.75
	} else if customer.RelationYears >= 1 {
		score += 1.0
	} else {
		score += 0.25
	}

	// Disputes (20% weight)
	if customer.DisputeCount == 0 {
		score += 2.0
	} else if customer.DisputeCount <= 2 {
		score += 1.0
	} else if customer.DisputeCount <= 4 {
		score += 0.5
	}

	// Risk factors (5% weight)
	if risk < 0.4 {
		score += 0.5
	}

	// Grade assignment: 0-10 scale
	// A: 8.0+ (fast payment, long relationship, few disputes)
	// B: 5.0+ (reasonable payment, established relationship)
	// C: 2.5+ (slow payment or short relationship or disputes)
	// D: <2.5 (high risk on multiple dimensions)
	if score >= 8.0 {
		return "A"
	} else if score >= 5.0 {
		return "B"
	} else if score >= 2.5 {
		return "C"
	} else {
		return "D"
	}
}

func computeConsistency(values []int) float64 {
	if len(values) < 2 {
		return 1.0
	}
	variance := computeVarianceInt(values, averageInt(values))
	return 1.0 / (1.0 + variance/100.0)
}

func computeTrend(values []int) string {
	if len(values) < 2 {
		return "stable"
	}
	recent := values[len(values)-1]
	older := values[0]
	if recent > older+10 {
		return "worsening"
	} else if recent < older-10 {
		return "improving"
	}
	return "stable"
}

func computeGrowthRate(values []float64) float64 {
	if len(values) < 2 {
		return 0.0
	}
	recent := values[len(values)-1]
	older := values[0]
	return (recent - older) / older * 100.0
}

func sumFloat(values []float64) float64 {
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum
}

func convertToNormedVector(data map[string]interface{}) []float64 {
	// Extract and normalize values to [0, 1] range for comparison
	vec := make([]float64, 4)

	// Amount: normalize to reasonable range [0, 100000]
	if amount, ok := data["amount"].(float64); ok {
		vec[0] = math.Min(amount/100000.0, 1.0)
	} else if amount, ok := data["amount"].(int); ok {
		vec[0] = math.Min(float64(amount)/100000.0, 1.0)
	}

	// Discount: already normalized [0, 1]
	if discount, ok := data["discount"].(float64); ok {
		vec[1] = discount
	} else if discount, ok := data["discount"].(int); ok {
		vec[1] = float64(discount)
	}

	// Margin: already normalized [0, 1]
	if margin, ok := data["margin"].(float64); ok {
		vec[2] = margin
	} else if margin, ok := data["margin"].(int); ok {
		vec[2] = float64(margin)
	}

	// Valid: boolean to float
	if valid, ok := data["valid"].(bool); ok {
		if valid {
			vec[3] = 1.0
		} else {
			vec[3] = 0.0
		}
	}

	return vec
}

func getRuleVector(ruleSet string) []float64 {
	// Compliance rules as normalized vectors [amount_norm, max_discount, min_margin, must_be_valid]
	// Returns target values that compliant data should be close to
	switch ruleSet {
	case "bahrain":
		return []float64{0.1, 0.10, 0.10, 1.0} // Reasonable amount, max 10% discount, min 10% margin, must be valid
	case "gcc":
		return []float64{0.1, 0.10, 0.10, 1.0}
	case "iso":
		return []float64{0.1, 0.15, 0.08, 1.0} // ISO allows more discount
	default:
		return []float64{0.1, 0.10, 0.10, 1.0}
	}
}

func computeNormDistance(v1, v2 []float64) float64 {
	if len(v1) != len(v2) {
		return 1.0 // Maximum distance if incompatible
	}

	dist := 0.0
	for i := 0; i < len(v1); i++ {
		diff := v1[i] - v2[i]
		dist += diff * diff
	}
	return math.Sqrt(dist) / float64(len(v1))
}

func eventTypeStringToInt(s string) int {
	// Map string to geometry.EventType enum
	switch s {
	case "invoice":
		return 1
	case "tender":
		return 2
	case "contract":
		return 3
	case "customer":
		return 8
	case "compliance":
		return 9
	default:
		return 0
	}
}
