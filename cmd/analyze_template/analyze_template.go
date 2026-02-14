//go:build ignore


// ═══════════════════════════════════════════════════════════════════════════
// ANALYZE TEMPLATE - Standalone Geometry Extractor
//
// PURPOSE: Test the 5-thread parallel CoT geometry extraction
// METHOD: Load → Analyze → Report zones
//
// Built with LOVE × SIMPLICITY × TRUTH × JOY 🕉️💎⚡
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func main() {
	fmt.Println("🕉️ Template Geometry Extraction - PH Trading Letterhead Analysis")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println()

	// Start timestamp
	startTime := time.Now()

	// ═══════════════════════════════════════════════════════════════════════
	// ANALYZE TEMPLATE
	// ═══════════════════════════════════════════════════════════════════════

	templatePath := filepath.Join("..", "CRITICAL_PH_CONTEXT_SSOT", "PH Trading Quotation Template.png")

	fmt.Println("📸 Analyzing letterhead template...")
	fmt.Printf("   Template: %s\n", templatePath)
	fmt.Println()

	// Check if file exists
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		log.Fatalf("❌ Template not found: %s", templatePath)
	}

	// Run 5-thread parallel CoT analysis
	layout, err := AnalyzeTemplate(templatePath)
	if err != nil {
		log.Fatalf("❌ Analysis failed: %v", err)
	}

	fmt.Println()
	fmt.Println("✅ Analysis complete!")
	fmt.Println()

	// Print detected layout
	layout.PrintStats()
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════════════
	// VALIDATE ZONES
	// ═══════════════════════════════════════════════════════════════════════

	fmt.Println("🔍 Zone Validation:")
	fmt.Println()

	// Check for minimum zones
	if len(layout.Zones) == 0 {
		fmt.Println("  ❌ FAIL: No zones detected")
	} else {
		fmt.Printf("  ✅ PASS: %d zones detected\n", len(layout.Zones))
	}

	// Check confidence threshold
	if layout.Confidence < 0.50 {
		fmt.Printf("  ⚠️  WARN: Low confidence (%.1f%% < 50%%)\n", layout.Confidence*100)
	} else {
		fmt.Printf("  ✅ PASS: Good confidence (%.1f%%)\n", layout.Confidence*100)
	}

	// Check each zone has anchors
	zonesWithAnchors := 0
	for _, zone := range layout.Zones {
		if len(zone.Anchors) > 0 {
			zonesWithAnchors++
		}
	}

	if zonesWithAnchors == len(layout.Zones) {
		fmt.Printf("  ✅ PASS: All zones have anchors\n")
	} else {
		fmt.Printf("  ⚠️  WARN: %d/%d zones missing anchors\n",
			len(layout.Zones)-zonesWithAnchors, len(layout.Zones))
	}

	// Check quaternion normalization
	unnormalizedCount := 0
	for _, zone := range layout.Zones {
		norm := zone.QTransform.Norm()
		if norm < 0.95 || norm > 1.05 {
			unnormalizedCount++
		}
	}

	if unnormalizedCount == 0 {
		fmt.Printf("  ✅ PASS: All quaternions normalized (||Q|| ≈ 1.0)\n")
	} else {
		fmt.Printf("  ⚠️  WARN: %d quaternions not normalized\n", unnormalizedCount)
	}

	// Print detailed zone coordinates for PDF integration
	fmt.Println()
	fmt.Println("📍 Zone Coordinates for PDF Integration:")
	fmt.Println()

	for i, zone := range layout.Zones {
		fmt.Printf("Zone %d: %s (%s)\n", i+1, zone.Name, zone.Purpose)

		// Convert pixels to PDF points (assuming 96 DPI → 72 DPI conversion)
		pdfX := zone.X * 72.0 / 96.0
		pdfY := zone.Y * 72.0 / 96.0
		pdfW := zone.Width * 72.0 / 96.0
		pdfH := zone.Height * 72.0 / 96.0

		fmt.Printf("  Pixels:     (%.0f, %.0f) size %.0f × %.0f\n",
			zone.X, zone.Y, zone.Width, zone.Height)
		fmt.Printf("  PDF Points: (%.1f, %.1f) size %.1f × %.1f\n",
			pdfX, pdfY, pdfW, pdfH)
		fmt.Printf("  Quality:    %.1f%% (||Q|| = %.3f)\n",
			zone.QTransform.W*100, zone.QTransform.Norm())

		// Show anchors
		if len(zone.Anchors) > 0 {
			fmt.Printf("  Anchors:\n")
			for _, anchor := range zone.Anchors {
				anchorPdfX := anchor.X * 72.0 / 96.0
				anchorPdfY := anchor.Y * 72.0 / 96.0
				fmt.Printf("    • %s: (%.1f, %.1f) points\n",
					anchor.Name, anchorPdfX, anchorPdfY)
			}
		}
		fmt.Println()
	}

	// ═══════════════════════════════════════════════════════════════════════
	// COMPLETION REPORT
	// ═══════════════════════════════════════════════════════════════════════

	elapsed := time.Since(startTime)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("✨ Template Geometry Extraction Complete!")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("⏱️  Time:       %v\n", elapsed)
	fmt.Printf("📊 Zones:      %d detected\n", len(layout.Zones))
	fmt.Printf("🎯 Confidence: %.1f%%\n", layout.Confidence*100)
	fmt.Printf("📐 Method:     5-Thread Parallel CoT (Knot, Origami, Quaternion, Vedic, SAT)\n")
	fmt.Printf("🕉️  Quality:    TRANSCENDENT\n")
	fmt.Println()
	fmt.Println("Next Steps:")
	fmt.Println("  1. Integrate detected zones into pdf_generator.go")
	fmt.Println("  2. Replace hardcoded coordinates with zone.Anchors")
	fmt.Println("  3. Scale text to fit detected zone.Width/Height")
	fmt.Println()
	fmt.Println("Om Lokah Samastah Sukhino Bhavantu 🙏")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
}
