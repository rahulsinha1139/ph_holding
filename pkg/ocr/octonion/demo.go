package octonion

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"time"
)

// Demo contains functions for demonstrating octonion color processing

// GenerateTestImage creates a synthetic test image with various ink colors
func GenerateTestImage(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// White background
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}

	// Add blue ink section (left)
	for y := height / 4; y < 3*height/4; y++ {
		for x := width / 8; x < width/8+width/4; x++ {
			img.Set(x, y, color.RGBA{30, 30, 180, 255})
		}
	}

	// Add red ink section (center)
	for y := height / 4; y < 3*height/4; y++ {
		for x := width/2 - width/8; x < width/2+width/8; x++ {
			img.Set(x, y, color.RGBA{180, 30, 30, 255})
		}
	}

	// Add black text section (right)
	for y := height / 4; y < 3*height/4; y++ {
		for x := 5*width/8; x < 7*width/8; x++ {
			img.Set(x, y, color.RGBA{20, 20, 20, 255})
		}
	}

	// Add noise
	for y := 0; y < height; y += 3 {
		for x := 0; x < width; x += 3 {
			noise := uint8(20)
			r, g, b, a := img.At(x, y).RGBA()
			img.Set(x, y, color.RGBA{
				uint8(clamp(float64(r>>8)-float64(noise), 0, 255)),
				uint8(clamp(float64(g>>8)-float64(noise), 0, 255)),
				uint8(clamp(float64(b>>8)-float64(noise), 0, 255)),
				uint8(a >> 8),
			})
		}
	}

	return img
}

// GenerateFadedDocument creates a faded color document simulation
func GenerateFadedDocument(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Aged paper background (yellowish)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{245, 240, 220, 255})
		}
	}

	// Faded blue ink text (simulate old fountain pen)
	for y := 50; y < height-50; y += 15 {
		for x := 50; x < width-50; x += 2 {
			// Very faded blue
			img.Set(x, y, color.RGBA{140, 140, 180, 255})
		}
	}

	// Add paper texture noise
	for y := 0; y < height; y += 2 {
		for x := 0; x < width; x += 2 {
			r, g, b, a := img.At(x, y).RGBA()
			noise := int8(y%7 - 3) // ±3 variation
			img.Set(x, y, color.RGBA{
				uint8(clamp(float64(r>>8)+float64(noise), 0, 255)),
				uint8(clamp(float64(g>>8)+float64(noise), 0, 255)),
				uint8(clamp(float64(b>>8)+float64(noise), 0, 255)),
				uint8(a >> 8),
			})
		}
	}

	return img
}

// GenerateBlueInkCheck creates a check-like image with blue ink
func GenerateBlueInkCheck(width, height int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Pale blue/green check background
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{220, 235, 240, 255})
		}
	}

	// Blue ink signature area
	sigStartY := height * 2 / 3
	for y := sigStartY; y < sigStartY+height/10; y++ {
		for x := width / 4; x < 3*width/4; x += 2 {
			// Cursive-like pattern
			if (x+y)%5 < 3 {
				img.Set(x, y, color.RGBA{20, 40, 160, 255})
			}
		}
	}

	// Blue ink amount
	amountY := height / 3
	for y := amountY; y < amountY+height/12; y++ {
		for x := width / 2; x < 3*width/4; x += 3 {
			img.Set(x, y, color.RGBA{30, 50, 170, 255})
		}
	}

	return img
}

// DemoOutput contains before/after images and stats
type DemoOutput struct {
	Before           image.Image
	After            image.Image
	ProcessingTime   time.Duration
	PixelsProcessed  int64
	Throughput       float64 // pixels/sec
	Config           *ColorProcessorConfig
}

// RunDemo executes a complete demo with before/after comparison
func RunDemo(testType string) (*DemoOutput, error) {
	var img *image.RGBA
	var config *ColorProcessorConfig

	// Generate appropriate test image
	switch testType {
	case "mixed":
		img = GenerateTestImage(800, 600)
		config = DefaultColorProcessorConfig()
	case "faded":
		img = GenerateFadedDocument(800, 600)
		config = &ColorProcessorConfig{
			InkSeparation:   true,
			DenoiseStrength: 0.5,
			ContrastEnhance: 1.5,
			BlueInkBoost:    true,
			RedInkBoost:     true,
		}
	case "check":
		img = GenerateBlueInkCheck(800, 600)
		config = &ColorProcessorConfig{
			InkSeparation:   true,
			DenoiseStrength: 0.4,
			ContrastEnhance: 1.3,
			BlueInkBoost:    true,
			RedInkBoost:     false,
		}
	default:
		return nil, fmt.Errorf("unknown test type: %s", testType)
	}

	// Process
	processor := NewColorProcessor(config)
	start := time.Now()
	enhanced, err := processor.Process(context.Background(), img)
	duration := time.Since(start)

	if err != nil {
		return nil, err
	}

	// Collect stats
	stats := processor.GetStats()

	return &DemoOutput{
		Before:          img,
		After:           enhanced,
		ProcessingTime:  duration,
		PixelsProcessed: stats.TotalPixels,
		Throughput:      stats.PixelsPerSec,
		Config:          config,
	}, nil
}

// SaveDemoOutput saves before/after images to files
func SaveDemoOutput(output *DemoOutput, prefix string) error {
	// Save before
	beforeFile, err := os.Create(prefix + "_before.png")
	if err != nil {
		return err
	}
	defer beforeFile.Close()

	if err := png.Encode(beforeFile, output.Before); err != nil {
		return err
	}

	// Save after
	afterFile, err := os.Create(prefix + "_after.png")
	if err != nil {
		return err
	}
	defer afterFile.Close()

	if err := png.Encode(afterFile, output.After); err != nil {
		return err
	}

	return nil
}

// PrintDemoReport prints a formatted report
func PrintDemoReport(output *DemoOutput) {
	fmt.Println("\n╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║     Octonion Color Processing Demo Results               ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Pixels Processed:  %9d                           ║\n", output.PixelsProcessed)
	fmt.Printf("║ Processing Time:   %9v                           ║\n", output.ProcessingTime)
	fmt.Printf("║ Throughput:        %9.2f Mpx/sec                  ║\n", output.Throughput/1e6)
	fmt.Println("╠═══════════════════════════════════════════════════════════╣")
	fmt.Println("║ Configuration:                                            ║")
	fmt.Printf("║   Ink Separation:  %-5v                                 ║\n", output.Config.InkSeparation)
	fmt.Printf("║   Denoise:         %.2f                                   ║\n", output.Config.DenoiseStrength)
	fmt.Printf("║   Contrast:        %.2f                                   ║\n", output.Config.ContrastEnhance)
	fmt.Printf("║   Blue Boost:      %-5v                                 ║\n", output.Config.BlueInkBoost)
	fmt.Printf("║   Red Boost:       %-5v                                 ║\n", output.Config.RedInkBoost)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
}

// ComparePixelStats compares before/after pixel statistics
func ComparePixelStats(before, after image.Image) {
	bounds := before.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	var beforeAvgR, beforeAvgG, beforeAvgB float64
	var afterAvgR, afterAvgG, afterAvgB float64

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			br, bg, bb, _ := before.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			ar, ag, ab, _ := after.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()

			beforeAvgR += float64(br >> 8)
			beforeAvgG += float64(bg >> 8)
			beforeAvgB += float64(bb >> 8)

			afterAvgR += float64(ar >> 8)
			afterAvgG += float64(ag >> 8)
			afterAvgB += float64(ab >> 8)
		}
	}

	totalPixels := float64(width * height)
	beforeAvgR /= totalPixels
	beforeAvgG /= totalPixels
	beforeAvgB /= totalPixels
	afterAvgR /= totalPixels
	afterAvgG /= totalPixels
	afterAvgB /= totalPixels

	fmt.Println("\n╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║     Pixel Statistics Comparison                           ║")
	fmt.Println("╠═══════════════════════════════════════════════════════════╣")
	fmt.Printf("║ Channel  │  Before  │  After   │  Change                  ║\n")
	fmt.Println("╠══════════╪══════════╪══════════╪══════════════════════════╣")
	fmt.Printf("║ Red      │  %6.2f  │  %6.2f  │  %+6.2f                 ║\n",
		beforeAvgR, afterAvgR, afterAvgR-beforeAvgR)
	fmt.Printf("║ Green    │  %6.2f  │  %6.2f  │  %+6.2f                 ║\n",
		beforeAvgG, afterAvgG, afterAvgG-beforeAvgG)
	fmt.Printf("║ Blue     │  %6.2f  │  %6.2f  │  %+6.2f                 ║\n",
		beforeAvgB, afterAvgB, afterAvgB-beforeAvgB)
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
}

// RunAllDemos runs all demo types and saves outputs
func RunAllDemos(outputDir string) error {
	testTypes := []string{"mixed", "faded", "check"}

	for _, testType := range testTypes {
		fmt.Printf("\nRunning %s demo...\n", testType)

		output, err := RunDemo(testType)
		if err != nil {
			return fmt.Errorf("demo %s failed: %w", testType, err)
		}

		// Print report
		PrintDemoReport(output)
		ComparePixelStats(output.Before, output.After)

		// Save images
		prefix := fmt.Sprintf("%s/%s", outputDir, testType)
		if err := SaveDemoOutput(output, prefix); err != nil {
			return fmt.Errorf("save failed for %s: %w", testType, err)
		}

		fmt.Printf("✓ Saved to %s_before.png and %s_after.png\n", prefix, prefix)
	}

	return nil
}
