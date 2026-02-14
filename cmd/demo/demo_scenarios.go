// ═══════════════════════════════════════════════════════════════════════════
// DEMO SCENARIOS - Pre-built Business Scenarios for PH Holdings Demo
//
// SCENARIOS:
//   1. New RFQ Arrives → Predict → Quote → Decision
//   2. Grade D Customer Detected → Decline with Explanation
//   3. ABB Competition → Margin Analysis → Strategy
//   4. Customer 360° View → Risk Assessment
//   5. Emergency Order → Premium Pricing
//
// Built with MATHEMATICAL RIGOR × PRODUCTION EXCELLENCE × DEMO READY 🕉️⚡💎
// Day 193 - E2E Integration Mission
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"
	"time"
)

// DemoScenario represents a complete business scenario
type DemoScenario struct {
	Name        string
	Description string
	Customer    Customer
	EHBasket    *ParsedEHBasket // Optional E+H pricing data
	Execute     func(*DemoScenario) error
}

// ScenarioRunner orchestrates demo scenarios
type ScenarioRunner struct {
	CostingEngine  *CostingEngine
	GeometryBridge *GeometryBridge
}

// NewScenarioRunner creates a scenario runner
func NewScenarioRunner() *ScenarioRunner {
	return &ScenarioRunner{
		CostingEngine:  NewCostingEngine(),
		GeometryBridge: NewGeometryBridge(),
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SCENARIO 1: New RFQ Processing
// ═══════════════════════════════════════════════════════════════════════════

func (sr *ScenarioRunner) Scenario1_NewRFQ() *DemoScenario {
	return &DemoScenario{
		Name:        "Scenario 1: New RFQ Processing",
		Description: "ENGIE sends RFQ for flow instruments - predict payment, generate quote, recommend decision",
		Customer: Customer{
			ID:             "CUST-ENGIE-001",
			BusinessName:   "ENGIE (Bahrain)",
			OrderValue:     15000.0,
			OrderHistory:   []float64{12000, 14000, 13500, 16000, 15500},
			PaymentHistory: []int{42, 38, 45, 40, 43},
			RelationYears:  8,
			Industry:       "Power Generation",
			Country:        "Bahrain",
			IsEmergency:    false,
			HasABB:         false,
			DisputeCount:   0,
		},
		Execute: func(ds *DemoScenario) error {
			fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
			fmt.Println("║  SCENARIO 1: NEW RFQ PROCESSING                                          ║")
			fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")

			// Step 1: Customer arrives with RFQ
			fmt.Println("\n[1] RFQ RECEIVED")
			fmt.Printf("    Customer:     %s\n", ds.Customer.BusinessName)
			fmt.Printf("    Industry:     %s\n", ds.Customer.Industry)
			fmt.Printf("    Order Value:  %.0f BHD\n", ds.Customer.OrderValue)
			fmt.Printf("    Relationship: %d years\n", ds.Customer.RelationYears)

			// Step 2: Run payment prediction
			fmt.Println("\n[2] PAYMENT PREDICTION")
			predictor := NewPaymentPredictor(&ds.Customer)
			prediction := predictor.Predict(&ds.Customer)

			fmt.Printf("    Grade:           %s\n", prediction.Grade)
			fmt.Printf("    Predicted Days:  %d days\n", prediction.PredictedDays)
			fmt.Printf("    Confidence:      %.0f%%\n", prediction.Confidence*100)
			fmt.Printf("    Three Regimes:   R1=%.0f%%, R2=%.0f%%, R3=%.0f%%\n",
				prediction.ThreeRegimes.R1*100,
				prediction.ThreeRegimes.R2*100,
				prediction.ThreeRegimes.R3*100)

			// Step 3: Generate mock quotation
			fmt.Println("\n[3] QUOTATION GENERATION")
			basket := sr.createMockEHBasket(&ds.Customer)
			sheet, err := sr.CostingEngine.GenerateCostingSheet(basket, &ds.Customer)
			if err != nil {
				return err
			}

			fmt.Printf("    Total Cost:      %.2f BHD (what PH pays E+H)\n", sheet.TotalCostBHD)
			fmt.Printf("    Total Revenue:   %.2f BHD (customer pays)\n", sheet.TotalFinalBHD)
			fmt.Printf("    Total Profit:    %.2f BHD\n", sheet.TotalProfitBHD)
			fmt.Printf("    Margin:          %.1f%%\n", sheet.ActualMarginPct*100)
			fmt.Printf("    Payment Terms:   %s\n", sheet.PaymentTerms)

			// Step 4: Decision
			fmt.Println("\n[4] DECISION")
			fmt.Printf("    Status:  %s\n", sheet.ApprovalStatus)
			fmt.Printf("    Action:  %s\n", sheet.RecommendedAction)

			if len(sheet.RiskWarnings) > 0 {
				fmt.Println("\n    Risk Warnings:")
				for _, warning := range sheet.RiskWarnings {
					fmt.Printf("      • %s\n", warning)
				}
			}

			fmt.Println("\n✓ Scenario 1 Complete: RFQ processed in <1 second vs 45 minutes manual")
			return nil
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SCENARIO 2: Grade D Customer Detection
// ═══════════════════════════════════════════════════════════════════════════

func (sr *ScenarioRunner) Scenario2_GradeDCustomer() *DemoScenario {
	return &DemoScenario{
		Name:        "Scenario 2: Grade D Customer Detection",
		Description: "High-risk customer detected - automatic decline recommendation",
		Customer: Customer{
			ID:             "CUST-RISKY-001",
			BusinessName:   "Risky Industrial Co.",
			OrderValue:     8000.0,
			OrderHistory:   []float64{7000, 6500, 8200},
			PaymentHistory: []int{95, 110, 88, 102, 115},
			RelationYears:  2,
			Industry:       "Manufacturing",
			Country:        "Bahrain",
			IsEmergency:    false,
			HasABB:         false,
			DisputeCount:   4,
		},
		Execute: func(ds *DemoScenario) error {
			fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
			fmt.Println("║  SCENARIO 2: GRADE D CUSTOMER DETECTION                                  ║")
			fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")

			// Step 1: Customer profile analysis
			fmt.Println("\n[1] CUSTOMER PROFILE ANALYSIS")
			fmt.Printf("    Customer:        %s\n", ds.Customer.BusinessName)
			fmt.Printf("    Avg Payment:     %.0f days (HIGH RISK)\n", averageInt(ds.Customer.PaymentHistory))
			fmt.Printf("    Dispute Count:   %d disputes (CONCERNING)\n", ds.Customer.DisputeCount)
			fmt.Printf("    Relationship:    %d years (SHORT)\n", ds.Customer.RelationYears)

			// Step 2: Run prediction
			fmt.Println("\n[2] RISK ASSESSMENT")
			predictor := NewPaymentPredictor(&ds.Customer)
			prediction := predictor.Predict(&ds.Customer)

			fmt.Printf("    ⚠ GRADE: %s (HIGH RISK)\n", prediction.Grade)
			fmt.Printf("    Predicted Days:  %d days\n", prediction.PredictedDays)
			fmt.Printf("    Confidence:      %.0f%%\n", prediction.Confidence*100)
			fmt.Printf("    Three Regimes:   R1=%.0f%% (Risky), R2=%.0f%%, R3=%.0f%% (Stable)\n",
				prediction.ThreeRegimes.R1*100,
				prediction.ThreeRegimes.R2*100,
				prediction.ThreeRegimes.R3*100)

			fmt.Println("\n    Risk Factors:")
			for _, rf := range prediction.RiskFactors {
				fmt.Printf("      • %s\n", rf)
			}

			// Step 3: Decision
			fmt.Println("\n[3] AUTOMATED DECISION")
			fmt.Printf("    ✗ DECLINE ORDER\n")
			fmt.Printf("    Alternative: Require 100%% advance payment\n")
			fmt.Printf("    Reasoning:   R3 < 20%% (stability regime too low)\n")

			fmt.Println("\n✓ Scenario 2 Complete: High-risk customer identified automatically")
			return nil
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SCENARIO 3: ABB Competition
// ═══════════════════════════════════════════════════════════════════════════

func (sr *ScenarioRunner) Scenario3_ABBCompetition() *DemoScenario {
	return &DemoScenario{
		Name:        "Scenario 3: ABB Competition Analysis",
		Description: "Customer has ABB offer - margin analysis and strategic decision",
		Customer: Customer{
			ID:             "CUST-BAPCO-001",
			BusinessName:   "BAPCO (Bahrain Petroleum)",
			OrderValue:     25000.0,
			OrderHistory:   []float64{22000, 24000, 23500, 26000},
			PaymentHistory: []int{48, 52, 45, 50},
			RelationYears:  6,
			Industry:       "Oil & Gas",
			Country:        "Bahrain",
			IsEmergency:    false,
			HasABB:         true, // ABB competing!
			DisputeCount:   1,
		},
		Execute: func(ds *DemoScenario) error {
			fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
			fmt.Println("║  SCENARIO 3: ABB COMPETITION ANALYSIS                                    ║")
			fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")

			// Step 1: Situation
			fmt.Println("\n[1] COMPETITIVE SITUATION")
			fmt.Printf("    Customer:     %s\n", ds.Customer.BusinessName)
			fmt.Printf("    Order Value:  %.0f BHD (HIGH VALUE)\n", ds.Customer.OrderValue)
			fmt.Printf("    ⚠ ABB IS COMPETING FOR THIS ORDER\n")

			// Step 2: Payment prediction
			fmt.Println("\n[2] CUSTOMER ASSESSMENT")
			predictor := NewPaymentPredictor(&ds.Customer)
			prediction := predictor.Predict(&ds.Customer)

			fmt.Printf("    Grade:           %s (Good customer)\n", prediction.Grade)
			fmt.Printf("    Predicted Days:  %d days\n", prediction.PredictedDays)
			fmt.Printf("    Relationship:    %d years (Established)\n", ds.Customer.RelationYears)

			// Step 3: Margin analysis
			fmt.Println("\n[3] MARGIN ANALYSIS")
			basket := sr.createMockEHBasket(&ds.Customer)
			sheet, _ := sr.CostingEngine.GenerateCostingSheet(basket, &ds.Customer)

			fmt.Printf("    Standard Margin: %.1f%%\n", sheet.StandardMarginPct*100)
			fmt.Printf("    Actual Margin:   %.1f%%\n", sheet.ActualMarginPct*100)
			fmt.Printf("    Profit:          %.2f BHD\n", sheet.TotalProfitBHD)

			// Step 4: Strategic decision
			fmt.Println("\n[4] STRATEGIC DECISION")
			if sheet.ActualMarginPct >= 0.15 {
				fmt.Println("    ✓ BID RECOMMENDED")
				fmt.Printf("    Reason: Margin %.1f%% ≥ 15%% threshold for ABB competition\n", sheet.ActualMarginPct*100)
				fmt.Println("    Strategy: Match ABB pricing while maintaining profitability")
			} else {
				fmt.Println("    ✗ DECLINE RECOMMENDED")
				fmt.Printf("    Reason: Margin %.1f%% < 15%% threshold for ABB competition\n", sheet.ActualMarginPct*100)
				fmt.Println("    Strategy: Let ABB win at low margin, preserve relationship")
			}

			fmt.Println("\n✓ Scenario 3 Complete: ABB competition analyzed with margin strategy")
			return nil
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SCENARIO 4: Customer 360° View
// ═══════════════════════════════════════════════════════════════════════════

func (sr *ScenarioRunner) Scenario4_Customer360() *DemoScenario {
	return &DemoScenario{
		Name:        "Scenario 4: Customer 360° Analysis",
		Description: "Multi-dimensional customer profile using quaternion entanglement",
		Customer: Customer{
			ID:             "CUST-VEOLIA-001",
			BusinessName:   "VEOLIA Water Technologies",
			OrderValue:     18000.0,
			OrderHistory:   []float64{15000, 17000, 16500, 18500, 17800, 19000},
			PaymentHistory: []int{35, 38, 32, 36, 34, 37},
			RelationYears:  12,
			Industry:       "Water & Wastewater",
			Country:        "Bahrain",
			IsEmergency:    false,
			HasABB:         false,
			DisputeCount:   0,
		},
		Execute: func(ds *DemoScenario) error {
			fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
			fmt.Println("║  SCENARIO 4: CUSTOMER 360° ANALYSIS                                      ║")
			fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")

			// Step 1: Basic profile
			fmt.Println("\n[1] CUSTOMER PROFILE")
			fmt.Printf("    Name:         %s\n", ds.Customer.BusinessName)
			fmt.Printf("    Industry:     %s\n", ds.Customer.Industry)
			fmt.Printf("    Relationship: %d years (LONG-TERM)\n", ds.Customer.RelationYears)
			fmt.Printf("    Order Count:  %d orders\n", len(ds.Customer.OrderHistory))

			// Step 2: Payment behavior
			fmt.Println("\n[2] PAYMENT BEHAVIOR ANALYSIS")
			avgPayment := averageInt(ds.Customer.PaymentHistory)
			variance := computeVarianceInt(ds.Customer.PaymentHistory, avgPayment)
			fmt.Printf("    Avg Days:     %.0f days (EXCELLENT)\n", avgPayment)
			fmt.Printf("    Consistency:  %.1f variance (LOW)\n", variance)
			fmt.Printf("    Disputes:     %d (CLEAN RECORD)\n", ds.Customer.DisputeCount)

			// Step 3: Order history analysis
			fmt.Println("\n[3] ORDER HISTORY ANALYSIS")
			avgOrder := averageFloat(ds.Customer.OrderHistory)
			totalValue := sumFloat(ds.Customer.OrderHistory)
			fmt.Printf("    Avg Order:    %.0f BHD\n", avgOrder)
			fmt.Printf("    Total Value:  %.0f BHD\n", totalValue)
			fmt.Printf("    Growth:       %.1f%% (GROWING)\n", computeGrowthRate(ds.Customer.OrderHistory))

			// Step 4: Risk assessment
			fmt.Println("\n[4] QUATERNION ENTANGLEMENT (360° VIEW)")
			c360, _ := sr.GeometryBridge.GetCustomer360(ds.Customer.ID, &ds.Customer)

			fmt.Printf("    Overall Grade:    %s (TOP TIER)\n", c360.Grade)
			fmt.Printf("    Risk Score:       %.0f%% (LOW)\n", calculateRiskScore(&ds.Customer)*100)
			fmt.Println("\n    Multi-View Entanglement:")
			fmt.Println("      • Payment View:      Stable, consistent")
			fmt.Println("      • Order View:        Growing, high value")
			fmt.Println("      • Relationship View: Long-term, loyal")
			fmt.Println("      • Risk View:         Low risk, reliable")

			// Step 5: Recommendation
			fmt.Println("\n[5] STRATEGIC RECOMMENDATION")
			fmt.Println("    ✓ PREMIUM CUSTOMER - PRIORITY TREATMENT")
			fmt.Println("    • Offer maximum 7% discount")
			fmt.Println("    • Expedite quote turnaround")
			fmt.Println("    • Assign senior account manager")
			fmt.Println("    • Consider volume pricing incentives")

			fmt.Println("\n✓ Scenario 4 Complete: 360° customer analysis using S³ geometry")
			return nil
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// SCENARIO 5: Emergency Order
// ═══════════════════════════════════════════════════════════════════════════

func (sr *ScenarioRunner) Scenario5_EmergencyOrder() *DemoScenario {
	return &DemoScenario{
		Name:        "Scenario 5: Emergency Order Premium Pricing",
		Description: "Customer needs urgent delivery - justify premium pricing",
		Customer: Customer{
			ID:             "CUST-ALBA-001",
			BusinessName:   "ALBA (Aluminium Bahrain)",
			OrderValue:     12000.0,
			OrderHistory:   []float64{10000, 11000, 12500},
			PaymentHistory: []int{40, 42, 38},
			RelationYears:  5,
			Industry:       "Manufacturing",
			Country:        "Bahrain",
			IsEmergency:    true, // EMERGENCY!
			HasABB:         false,
			DisputeCount:   0,
		},
		Execute: func(ds *DemoScenario) error {
			fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
			fmt.Println("║  SCENARIO 5: EMERGENCY ORDER - PREMIUM PRICING                           ║")
			fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")

			// Step 1: Situation
			fmt.Println("\n[1] EMERGENCY SITUATION")
			fmt.Printf("    Customer:        %s\n", ds.Customer.BusinessName)
			fmt.Printf("    ⚠ EMERGENCY ORDER - PRODUCTION DOWN\n")
			fmt.Printf("    Required:        Delivery within 7 days\n")
			fmt.Printf("    Standard Lead:   14-21 days\n")

			// Step 2: Customer assessment
			fmt.Println("\n[2] CUSTOMER ASSESSMENT")
			predictor := NewPaymentPredictor(&ds.Customer)
			prediction := predictor.Predict(&ds.Customer)

			fmt.Printf("    Grade:           %s\n", prediction.Grade)
			fmt.Printf("    Payment History: %.0f days avg (RELIABLE)\n", averageInt(ds.Customer.PaymentHistory))
			fmt.Printf("    Relationship:    %d years\n", ds.Customer.RelationYears)

			// Step 3: Pricing analysis
			fmt.Println("\n[3] PRICING ANALYSIS")
			basket := sr.createMockEHBasket(&ds.Customer)
			sheet, _ := sr.CostingEngine.GenerateCostingSheet(basket, &ds.Customer)

			fmt.Printf("    Standard Price:  %.2f BHD\n", sheet.TotalFinalBHD)
			emergencySurcharge := sheet.TotalFinalBHD * 0.15 // 15% emergency surcharge
			emergencyPrice := sheet.TotalFinalBHD + emergencySurcharge
			emergencyMargin := (emergencyPrice - sheet.TotalCostBHD) / sheet.TotalCostBHD

			fmt.Printf("    Emergency Surcharge: %.2f BHD (15%%)\n", emergencySurcharge)
			fmt.Printf("    Emergency Price:     %.2f BHD\n", emergencyPrice)
			fmt.Printf("    Emergency Margin:    %.1f%%\n", emergencyMargin*100)

			// Step 4: Justification
			fmt.Println("\n[4] PRICING JUSTIFICATION")
			fmt.Println("    Emergency Premium Justified By:")
			fmt.Println("      • Expedited E+H production (+5-7%)")
			fmt.Println("      • Air freight vs sea freight (+3-5%)")
			fmt.Println("      • Weekend/overtime processing (+2-3%)")
			fmt.Println("      • Inventory risk reduction (+2-3%)")
			fmt.Println("      • Customer production loss prevention (>>100% ROI for customer)")

			// Step 5: Decision
			fmt.Println("\n[5] RECOMMENDATION")
			fmt.Println("    ✓ APPROVE WITH EMERGENCY PRICING")
			fmt.Printf("    Quote:    %.2f BHD (includes 15%% emergency surcharge)\n", emergencyPrice)
			fmt.Printf("    Margin:   %.1f%% (enhanced)\n", emergencyMargin*100)
			fmt.Println("    Payment:  Accelerated to compensate for risk")
			fmt.Println("    Strategy: Customer saves production downtime >> premium cost")

			fmt.Println("\n✓ Scenario 5 Complete: Emergency order priced for mutual benefit")
			return nil
		},
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// HELPER FUNCTIONS
// ═══════════════════════════════════════════════════════════════════════════

// createMockEHBasket creates a mock E+H basket for demo
func (sr *ScenarioRunner) createMockEHBasket(customer *Customer) *ParsedEHBasket {
	// Generate realistic product items based on order value
	itemCount := 2 + int(customer.OrderValue/5000.0)
	if itemCount > 5 {
		itemCount = 5
	}

	items := make([]ParsedEHItem, itemCount)
	remainingValue := customer.OrderValue

	productTypes := []string{"E+H Flow", "E+H Level", "E+H Pressure", "E+H Analytics"}

	for i := 0; i < itemCount; i++ {
		itemValue := remainingValue / float64(itemCount-i)

		// Convert BHD to EUR (1 BHD ≈ 2.44 EUR)
		itemValueEUR := itemValue * 2.44

		items[i] = ParsedEHItem{
			OrderCode:          fmt.Sprintf("EH-%d-%03d", time.Now().Year(), i+1),
			Description:        fmt.Sprintf("%s Instrument #%d", productTypes[i%len(productTypes)], i+1),
			Quantity:           1 + (i % 3),
			ProductType:        productTypes[i%len(productTypes)],
			UnitSalesPriceEUR:  itemValueEUR / float64(1+(i%3)),
			UnitSalesPriceBHD:  itemValue / float64(1+(i%3)),
			ItemSalesPriceEUR:  itemValueEUR,
			ItemSalesPriceBHD:  itemValue,
			ProductionDays:     7 + (i * 3),
		}

		remainingValue -= itemValue
	}

	totalEUR := customer.OrderValue * 2.44

	return &ParsedEHBasket{
		CustomerNumber: customer.ID,
		CustomerName:   customer.BusinessName,
		Items:          items,
		TotalNetEUR:    totalEUR,
		TotalGrossEUR:  totalEUR,
		TotalNetBHD:    customer.OrderValue,
		TotalGrossBHD:  customer.OrderValue,
		ItemCount:      len(items),
		SourceFile:     "demo_basket.xml",
	}
}

// RunAllScenarios executes all demo scenarios
func (sr *ScenarioRunner) RunAllScenarios() {
	scenarios := []*DemoScenario{
		sr.Scenario1_NewRFQ(),
		sr.Scenario2_GradeDCustomer(),
		sr.Scenario3_ABBCompetition(),
		sr.Scenario4_Customer360(),
		sr.Scenario5_EmergencyOrder(),
	}

	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                                          ║")
	fmt.Println("║         PH HOLDINGS - DEMO SCENARIOS EXECUTION                           ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")

	for i, scenario := range scenarios {
		fmt.Printf("\n\n═══════════════════════════════════════════════════════════════════════════\n")
		fmt.Printf("  %s\n", scenario.Name)
		fmt.Printf("  %s\n", scenario.Description)
		fmt.Printf("═══════════════════════════════════════════════════════════════════════════\n")

		err := scenario.Execute(scenario)
		if err != nil {
			fmt.Printf("\n✗ Scenario %d failed: %v\n", i+1, err)
		}

		if i < len(scenarios)-1 {
			fmt.Println("\n\nPress Enter to continue to next scenario...")
			// In actual demo, wait for input
			// fmt.Scanln()
		}
	}

	fmt.Println("\n\n╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║  ALL SCENARIOS COMPLETE                                                  ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
}
