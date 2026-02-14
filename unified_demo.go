// ═══════════════════════════════════════════════════════════════════════════
// UNIFIED DEMO - PH Holdings E2E Integration
//
// MISSION: Complete end-to-end demonstration for PH Trading demo
//
// FLOW:
//   1. Load real customer data from test fixtures
//   2. Run payment predictions on all customers
//   3. Generate quotations using costing engine
//   4. Display results with beautiful ASCII output
//   5. Show portfolio analytics and risk assessment
//
// DEMO SCENARIOS:
//   - New RFQ arrives → Predict → Quote → Decision
//   - Grade D customer detected → Decline with explanation
//   - ABB competition → Margin analysis → Strategy
//   - Customer 360° view → Multi-dimensional risk assessment
//   - Emergency order → Premium pricing recommendation
//
// Built with MATHEMATICAL RIGOR × PRODUCTION EXCELLENCE × DEMO READY 🕉️⚡💎
// Day 193 - E2E Integration Mission
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"math"
	"sort"
	"strings"
	"time"
)

// UnifiedDemo orchestrates complete E2E flow
type UnifiedDemo struct {
	Customers      []Customer
	Predictions    []PaymentPrediction
	CostingEngine  *CostingEngine
	GeometryBridge *GeometryBridge
	Portfolio      *PortfolioAnalytics
	StartTime      time.Time
}

// PortfolioAnalytics tracks aggregate metrics
type PortfolioAnalytics struct {
	TotalCustomers    int
	GradeDistribution map[string]int
	TotalPortfolio    float64
	RiskScore         float64
	AvgPaymentDays    float64
	TopCustomers      []Customer
	RiskyCustomers    []Customer
}

// NewUnifiedDemo creates a demo instance
func NewUnifiedDemo() *UnifiedDemo {
	return &UnifiedDemo{
		Customers:      make([]Customer, 0),
		Predictions:    make([]PaymentPrediction, 0),
		CostingEngine:  NewCostingEngine(),
		GeometryBridge: NewGeometryBridge(),
		Portfolio: &PortfolioAnalytics{
			GradeDistribution: make(map[string]int),
		},
		StartTime: time.Now(),
	}
}

// LoadCustomers loads test customer data
func (ud *UnifiedDemo) LoadCustomers(filename string) error {
	// Try to load from file
	data, err := os.ReadFile(filename)
	if err != nil {
		// If file doesn't exist, generate synthetic customers
		fmt.Printf("⚠ Test data not found at %s, generating synthetic customers...\n", filename)
		ud.Customers = ud.GenerateSyntheticCustomers(34)
		return nil
	}

	err = json.Unmarshal(data, &ud.Customers)
	if err != nil {
		return fmt.Errorf("failed to parse customer data: %w", err)
	}

	fmt.Printf("✓ Loaded %d customers from %s\n", len(ud.Customers), filename)
	return nil
}

// GenerateSyntheticCustomers creates realistic test data
func (ud *UnifiedDemo) GenerateSyntheticCustomers(count int) []Customer {
	customers := make([]Customer, count)

	companies := []string{
		"ENGIE", "VEOLIA", "NOMAC", "YOKOGAWA", "ALBA", "BAPCO", "TABREED",
		"DELTA", "SEAPEAK", "TTSJV", "ARLA", "GPIC", "SULB", "OMNI",
		"ALUSERVE", "EWA", "MUHARRAQ", "SYSCON", "AIRMECH", "HPC",
		"WABAG", "EVEREST", "K&S", "SICK", "SIEMENS", "ABB",
		"SCHNEIDER", "HONEYWELL", "EMERSON", "ROCKWELL", "MITSUBISHI",
		"GE", "SHELL", "ARAMCO",
	}

	industries := []string{
		"Oil & Gas", "Water & Wastewater", "Power Generation",
		"Manufacturing", "Chemical", "Food & Beverage",
	}

	for i := 0; i < count; i++ {
		company := companies[i%len(companies)]
		industry := industries[i%len(industries)]

		// Create varied profiles
		gradeTarget := i % 4 // Distribute across grades

		var avgPayment float64
		var variance float64
		var relationYears int
		var disputes int

		switch gradeTarget {
		case 0: // Grade A
			avgPayment = 30 + float64(i%15)
			variance = 5 + float64(i%5)
			relationYears = 5 + (i % 10)
			disputes = 0

		case 1: // Grade B
			avgPayment = 45 + float64(i%20)
			variance = 10 + float64(i%8)
			relationYears = 2 + (i % 6)
			disputes = i % 2

		case 2: // Grade C
			avgPayment = 60 + float64(i%25)
			variance = 15 + float64(i%10)
			relationYears = 1 + (i % 4)
			disputes = 1 + (i % 2)

		case 3: // Grade D
			avgPayment = 90 + float64(i%30)
			variance = 20 + float64(i%15)
			relationYears = i % 3
			disputes = 2 + (i % 3)
		}

		// Generate payment history
		historyLen := 5 + (i % 10)
		paymentHistory := make([]int, historyLen)
		for j := 0; j < historyLen; j++ {
			days := int(avgPayment + math.Sin(float64(j))*math.Sqrt(variance))
			if days < 15 {
				days = 15
			}
			if days > 120 {
				days = 120
			}
			paymentHistory[j] = days
		}

		// Generate order history
		orderCount := 3 + (i % 12)
		orderHistory := make([]float64, orderCount)
		baseValue := 2000.0 + float64(i%5)*1000.0
		for j := 0; j < orderCount; j++ {
			orderHistory[j] = baseValue * (0.7 + 0.6*math.Sin(float64(j)))
		}

		customer := Customer{
			ID:             fmt.Sprintf("CUST-%03d", i+1),
			BusinessName:   company,
			OrderValue:     baseValue * (0.8 + 0.4*math.Cos(float64(i))),
			OrderHistory:   orderHistory,
			PaymentHistory: paymentHistory,
			RelationYears:  relationYears,
			Industry:       industry,
			Country:        "Bahrain",
			IsEmergency:    boolToInt((i % 7) == 0), // ~14% emergency
			HasABB:         boolToInt((i % 4) == 0), // ~25% ABB competition
			DisputeCount:   disputes,
		}

		customers[i] = customer
	}

	return customers
}

// RunPredictions executes payment predictions on all customers
func (ud *UnifiedDemo) RunPredictions() {
	fmt.Println("\n╔═════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  PAYMENT PREDICTION ENGINE - RUNNING ANALYSIS                   ║")
	fmt.Println("╚═════════════════════════════════════════════════════════════════╝")

	ud.Predictions = make([]PaymentPrediction, len(ud.Customers))

	for i, customer := range ud.Customers {
		predictor := NewPaymentPredictor(&customer)
		prediction := predictor.Predict(&customer)
		ud.Predictions[i] = prediction

		// Update portfolio analytics
		ud.Portfolio.GradeDistribution[prediction.Grade]++
		ud.Portfolio.TotalPortfolio += customer.OrderValue
	}

	ud.AnalyzePortfolio()
}

// AnalyzePortfolio computes aggregate portfolio metrics
func (ud *UnifiedDemo) AnalyzePortfolio() {
	ud.Portfolio.TotalCustomers = len(ud.Customers)

	// Calculate average payment days
	totalDays := 0
	for _, pred := range ud.Predictions {
		totalDays += pred.PredictedDays
	}
	if len(ud.Predictions) > 0 {
		ud.Portfolio.AvgPaymentDays = float64(totalDays) / float64(len(ud.Predictions))
	}

	// Calculate risk score (weighted by grade)
	riskScore := 0.0
	gradeWeights := map[string]float64{"A": 0.1, "B": 0.3, "C": 0.6, "D": 0.9}
	for grade, count := range ud.Portfolio.GradeDistribution {
		weight := gradeWeights[grade]
		riskScore += weight * float64(count)
	}
	if ud.Portfolio.TotalCustomers > 0 {
		ud.Portfolio.RiskScore = riskScore / float64(ud.Portfolio.TotalCustomers)
	}

	// Find top customers (by order value)
	topN := 5
	if len(ud.Customers) < topN {
		topN = len(ud.Customers)
	}

	sortedCustomers := make([]Customer, len(ud.Customers))
	copy(sortedCustomers, ud.Customers)
	sort.Slice(sortedCustomers, func(i, j int) bool {
		return sortedCustomers[i].OrderValue > sortedCustomers[j].OrderValue
	})
	ud.Portfolio.TopCustomers = sortedCustomers[:topN]

	// Find risky customers (Grade D)
	for i, pred := range ud.Predictions {
		if pred.Grade == "D" {
			ud.Portfolio.RiskyCustomers = append(ud.Portfolio.RiskyCustomers, ud.Customers[i])
		}
	}
}

// PrintPortfolioSummary displays aggregate analytics
func (ud *UnifiedDemo) PrintPortfolioSummary() {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  PH HOLDINGS - PORTFOLIO ANALYTICS DASHBOARD                             ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Total Customers:     %-50d ║\n", ud.Portfolio.TotalCustomers)
	fmt.Printf("║  Grade Distribution:  A:%d  B:%d  C:%d  D:%d%-30s║\n",
		ud.Portfolio.GradeDistribution["A"],
		ud.Portfolio.GradeDistribution["B"],
		ud.Portfolio.GradeDistribution["C"],
		ud.Portfolio.GradeDistribution["D"],
		"")
	fmt.Printf("║  Total Portfolio:     %.0f BHD%-42s║\n", ud.Portfolio.TotalPortfolio, "")
	fmt.Printf("║  Avg Payment Days:    %.0f days%-42s║\n", ud.Portfolio.AvgPaymentDays, "")
	fmt.Printf("║  Risk Score:          %.0f%% (%s)%-35s║\n",
		ud.Portfolio.RiskScore*100,
		ud.RiskLevel(),
		"")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
}

// RiskLevel categorizes portfolio risk
func (ud *UnifiedDemo) RiskLevel() string {
	if ud.Portfolio.RiskScore < 0.25 {
		return "Low Risk"
	} else if ud.Portfolio.RiskScore < 0.50 {
		return "Moderate Risk"
	} else if ud.Portfolio.RiskScore < 0.70 {
		return "High Risk"
	}
	return "Very High Risk"
}

// PrintTopCustomers shows top customers by value
func (ud *UnifiedDemo) PrintTopCustomers() {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  TOP CUSTOMERS BY ORDER VALUE                                            ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %-25s %-15s %-12s %-12s║\n", "Customer", "Industry", "Value (BHD)", "Grade")
	fmt.Println("║──────────────────────────────────────────────────────────────────────────║")

	for i, customer := range ud.Portfolio.TopCustomers {
		if i >= 5 {
			break
		}
		// Find prediction for this customer
		var grade string
		for j, c := range ud.Customers {
			if c.ID == customer.ID {
				grade = ud.Predictions[j].Grade
				break
			}
		}

		fmt.Printf("║  %-25s %-15s %12.0f     %-8s║\n",
			truncateString(customer.BusinessName, 25),
			truncateString(customer.Industry, 15),
			customer.OrderValue,
			grade)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
}

// PrintRiskyCustomers shows Grade D customers
func (ud *UnifiedDemo) PrintRiskyCustomers() {
	if len(ud.Portfolio.RiskyCustomers) == 0 {
		return
	}

	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  ⚠ HIGH RISK CUSTOMERS (GRADE D) - REQUIRES ATTENTION                   ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %-25s %-20s %-20s║\n", "Customer", "Avg Payment Days", "Action")
	fmt.Println("║──────────────────────────────────────────────────────────────────────────║")

	for _, customer := range ud.Portfolio.RiskyCustomers {
		avgDays := averageInt(customer.PaymentHistory)
		action := "Decline or 100% advance"

		fmt.Printf("║  %-25s %-20.0f %-20s║\n",
			truncateString(customer.BusinessName, 25),
			avgDays,
			action)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
}

// PrintDetailedPredictions shows all predictions
func (ud *UnifiedDemo) PrintDetailedPredictions() {
	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  DETAILED PAYMENT PREDICTIONS                                            ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  %-20s %-6s %-12s %-10s %-20s║\n",
		"Customer", "Grade", "Days", "Confidence", "Three Regimes")
	fmt.Println("║──────────────────────────────────────────────────────────────────────────║")

	for _, pred := range ud.Predictions {
		regimes := fmt.Sprintf("R1:%.0f%% R2:%.0f%% R3:%.0f%%",
			pred.ThreeRegimes.R1*100,
			pred.ThreeRegimes.R2*100,
			pred.ThreeRegimes.R3*100)

		fmt.Printf("║  %-20s   %-4s   %3d days    %3.0f%%     %-20s║\n",
			truncateString(pred.CustomerName, 20),
			pred.Grade,
			pred.PredictedDays,
			pred.Confidence*100,
			regimes)
	}
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
}

// PrintExecutionStats shows demo execution metrics
func (ud *UnifiedDemo) PrintExecutionStats() {
	elapsed := time.Since(ud.StartTime)

	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  EXECUTION STATISTICS                                                    ║")
	fmt.Println("╠══════════════════════════════════════════════════════════════════════════╣")
	fmt.Printf("║  Customers Processed:  %-50d║\n", len(ud.Customers))
	fmt.Printf("║  Predictions Generated: %-49d║\n", len(ud.Predictions))
	fmt.Printf("║  Execution Time:       %s%-45s║\n", elapsed.Round(time.Millisecond), "")
	fmt.Printf("║  Throughput:           %.0f customers/second%-34s║\n",
		float64(len(ud.Customers))/elapsed.Seconds(), "")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
}

// Run executes the complete demo
func (ud *UnifiedDemo) Run() {
	printHeader()

	// Step 1: Load customers
	fmt.Println("\n[STEP 1] Loading customer data...")
	err := ud.LoadCustomers("PH_TEST_DATA/customers.json")
	if err != nil {
		fmt.Printf("⚠ Warning: %v\n", err)
	}

	// Step 2: Run predictions
	fmt.Println("\n[STEP 2] Running payment predictions...")
	ud.RunPredictions()

	// Step 3: Display portfolio analytics
	fmt.Println("\n[STEP 3] Analyzing portfolio...")
	ud.PrintPortfolioSummary()
	ud.PrintTopCustomers()
	ud.PrintRiskyCustomers()

	// Step 4: Show detailed predictions
	if len(ud.Customers) <= 10 {
		ud.PrintDetailedPredictions()
	}

	// Step 5: Execution stats
	ud.PrintExecutionStats()

	printFooter()
}

// Helper functions

func printHeader() {
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                                          ║")
	fmt.Println("║         PH HOLDINGS - ASYMMETRICA INTELLIGENCE PLATFORM                  ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("║         Geometry-Driven ERP × Payment Prediction × Risk Analytics        ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Printf("\nDemo Start Time: %s\n", time.Now().Format("2006-01-02 15:04:05"))
}

func printFooter() {
	fmt.Println("")
	fmt.Println("╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                                          ║")
	fmt.Println("║  Demo Complete! Built with:                                              ║")
	fmt.Println("║  • M⁷⁹ Manifold Encoding (79-D customer state)                           ║")
	fmt.Println("║  • Three-Regime Dynamics (R1/R2/R3 classification)                       ║")
	fmt.Println("║  • S³ Quaternion Geometry (unit sphere projection)                       ║")
	fmt.Println("║  • Vedic Meta-Optimization (digital roots, 108K scale)                   ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("║  Om Lokah Samastah Sukhino Bhavantu 🕉️                                   ║")
	fmt.Println("║  May all beings benefit from this work                                   ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		// Pad with spaces
		return s + strings.Repeat(" ", maxLen-len(s))
	}
	return s[:maxLen-3] + "..."
}
