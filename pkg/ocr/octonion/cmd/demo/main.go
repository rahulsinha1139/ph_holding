package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"ph_holdings_app/pkg/ocr/octonion"
)

func main() {
	// Parse flags
	testType := flag.String("type", "all", "Test type: mixed, faded, check, or all")
	outputDir := flag.String("output", ".", "Output directory for images")
	flag.Parse()

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║   Octonion Color Processing Demo Tool                    ║")
	fmt.Println("║   ACE Engine - Mathematical Weaponization                 ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")

	// Ensure output directory exists
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		log.Fatalf("Failed to create output dir: %v", err)
	}

	if *testType == "all" {
		// Run all demos
		if err := octonion.RunAllDemos(*outputDir); err != nil {
			log.Fatalf("Demo failed: %v", err)
		}

		fmt.Println("\n✓ All demos completed successfully!")
		fmt.Printf("✓ Check %s/ for before/after images\n", *outputDir)

	} else {
		// Run single demo
		output, err := octonion.RunDemo(*testType)
		if err != nil {
			log.Fatalf("Demo failed: %v", err)
		}

		octonion.PrintDemoReport(output)
		octonion.ComparePixelStats(output.Before, output.After)

		prefix := fmt.Sprintf("%s/%s", *outputDir, *testType)
		if err := octonion.SaveDemoOutput(output, prefix); err != nil {
			log.Fatalf("Save failed: %v", err)
		}

		fmt.Printf("\n✓ Demo completed successfully!\n")
		fmt.Printf("✓ Saved to %s_before.png and %s_after.png\n", prefix, prefix)
	}

	fmt.Println("\n" + strings.Repeat("─", 59))
	fmt.Println("Om Lokah Samastah Sukhino Bhavantu 🙏")
	fmt.Println("May all beings benefit from this mathematical weaponization!")
	fmt.Println(strings.Repeat("─", 59))
}
