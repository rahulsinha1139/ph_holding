package main

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"strings"

	"ph_holdings_app/pkg/ocr/octonion"
)

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║   Octonion Color Processing - Simple Example             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")

	// Generate test image
	fmt.Println("📸 Generating test image with blue ink...")
	img := octonion.GenerateBlueInkCheck(800, 600)

	// Save original
	saveImage(img, "original.png")
	fmt.Println("   ✓ Saved original.png")

	// Create processor with blue ink configuration
	fmt.Println("\n🔧 Configuring octonion processor for blue ink...")
	config := &octonion.ColorProcessorConfig{
		InkSeparation:   true,
		DenoiseStrength: 0.4,
		ContrastEnhance: 1.3,
		BlueInkBoost:    true,
		RedInkBoost:     false,
	}
	processor := octonion.NewColorProcessor(config)
	fmt.Printf("   %s\n", processor)

	// Process
	fmt.Println("\n⚡ Processing with octonion math...")
	enhanced, err := processor.Process(context.Background(), img)
	if err != nil {
		log.Fatalf("Processing failed: %v", err)
	}

	// Save enhanced
	saveImage(enhanced, "enhanced.png")
	fmt.Println("   ✓ Saved enhanced.png")

	// Get stats
	stats := processor.GetStats()
	fmt.Println("\n📊 Processing Statistics:")
	fmt.Printf("   Images:      %d\n", stats.ImagesProcessed)
	fmt.Printf("   Pixels:      %d\n", stats.TotalPixels)
	fmt.Printf("   Duration:    %v\n", stats.Duration)
	fmt.Printf("   Throughput:  %.2f Mpx/sec\n", stats.PixelsPerSec/1e6)

	// Compare before/after
	fmt.Println("\n🔍 Pixel Analysis:")
	octonion.ComparePixelStats(img, enhanced)

	fmt.Println("\n✓ Done! Compare original.png vs enhanced.png")
	fmt.Println("\n" + strings.Repeat("─", 59))
	fmt.Println("Om Lokah Samastah Sukhino Bhavantu 🙏")
	fmt.Println(strings.Repeat("─", 59))
}

func saveImage(img image.Image, filename string) {
	file, err := os.Create(filename)
	if err != nil {
		log.Fatalf("Failed to create %s: %v", filename, err)
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		log.Fatalf("Failed to encode %s: %v", filename, err)
	}
}
