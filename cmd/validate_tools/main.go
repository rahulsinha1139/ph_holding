// ═══════════════════════════════════════════════════════════════════════════
// TOOLS VALIDATOR DEMO
//
// Demonstrates runtime external tools validation with graceful degradation
//
// Usage:
//   go run cmd/validate_tools/main.go
//
// Built with SIMPLICITY × CLARITY × USER_EXPERIENCE 🕉️⚡💎
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"encoding/json"
	"fmt"
	"os"

	"ph_holdings_app/integration"
)

func main() {
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println("PH HOLDINGS - EXTERNAL TOOLS VALIDATOR")
	fmt.Println("═══════════════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Validate all tools
	report := integration.ValidateAndLog()

	// Show JSON output
	fmt.Println("\n" + "JSON Report:")
	fmt.Println("═" + "══════════════════════════════════════════════════════════════════════════")

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		fmt.Printf("Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(jsonData))

	// Exit with appropriate code
	if report.ReadyToUse {
		fmt.Println("\n✓ Application is READY TO USE")
		os.Exit(0)
	} else {
		fmt.Println("\n⚠ Some required tools are missing")
		fmt.Println("\nInstallation Instructions:")
		fmt.Println(report.Summary)
		os.Exit(1)
	}
}
