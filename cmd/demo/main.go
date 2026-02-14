// ═══════════════════════════════════════════════════════════════════════════
// DEMO MAIN - Entry Point for PH Holdings Demo
//
// USAGE:
//   go run demo_main.go --mode=portfolio    # Portfolio analytics
//   go run demo_main.go --mode=scenarios    # Interactive scenarios
//   go run demo_main.go --mode=full         # Full demo (default)
//
// Built with MATHEMATICAL RIGOR × PRODUCTION EXCELLENCE × DEMO READY 🕉️⚡💎
// Day 193 - E2E Integration Mission
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Command-line flags
	mode := flag.String("mode", "full", "Demo mode: portfolio, scenarios, full")
	customers := flag.Int("customers", 34, "Number of customers to generate")
	flag.Parse()

	fmt.Println("\n╔══════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║                                                                          ║")
	fmt.Println("║         PH HOLDINGS - ASYMMETRICA INTELLIGENCE PLATFORM                  ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("║         UNIFIED DEMO - E2E INTEGRATION                                   ║")
	fmt.Println("║                                                                          ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	switch *mode {
	case "portfolio":
		runPortfolioDemo(*customers)

	case "scenarios":
		runScenariosDemo()

	case "full":
		runPortfolioDemo(*customers)
		fmt.Println()
		fmt.Println()
		runScenariosDemo()

	default:
		fmt.Printf("Unknown mode: %s\n", *mode)
		fmt.Println("Valid modes: portfolio, scenarios, full")
		os.Exit(1)
	}

	fmt.Println("\n✓ Demo execution complete!")
	fmt.Println("\nNext steps:")
	fmt.Println("  1. Review portfolio analytics and grade distribution")
	fmt.Println("  2. Examine scenario outcomes and business logic")
	fmt.Println("  3. Test with real customer data from PH_TEST_DATA/")
	fmt.Println("  4. Deploy to production with live E+H integration")
	fmt.Println()
}

func runPortfolioDemo(customerCount int) {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("  PORTFOLIO ANALYTICS DEMO")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	demo := NewUnifiedDemo()

	// Try to load real data, fall back to synthetic
	err := demo.LoadCustomers("PH_TEST_DATA/customers.json")
	if err != nil {
		fmt.Printf("Generating %d synthetic customers...\n", customerCount)
		demo.Customers = demo.GenerateSyntheticCustomers(customerCount)
	}

	demo.RunPredictions()
	demo.PrintPortfolioSummary()
	demo.PrintTopCustomers()
	demo.PrintRiskyCustomers()
	demo.PrintExecutionStats()
}

func runScenariosDemo() {
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("  BUSINESS SCENARIOS DEMO")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	runner := NewScenarioRunner()
	runner.RunAllScenarios()
}
