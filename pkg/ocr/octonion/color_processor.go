package octonion

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"sync"
	"time"
)

var rgba16To8 = [256]uint8{
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9,
	10, 11, 12, 13, 14, 15, 16, 17, 18, 19,
	20, 21, 22, 23, 24, 25, 26, 27, 28, 29,
	30, 31, 32, 33, 34, 35, 36, 37, 38, 39,
	40, 41, 42, 43, 44, 45, 46, 47, 48, 49,
	50, 51, 52, 53, 54, 55, 56, 57, 58, 59,
	60, 61, 62, 63, 64, 65, 66, 67, 68, 69,
	70, 71, 72, 73, 74, 75, 76, 77, 78, 79,
	80, 81, 82, 83, 84, 85, 86, 87, 88, 89,
	90, 91, 92, 93, 94, 95, 96, 97, 98, 99,
	100, 101, 102, 103, 104, 105, 106, 107, 108, 109,
	110, 111, 112, 113, 114, 115, 116, 117, 118, 119,
	120, 121, 122, 123, 124, 125, 126, 127, 128, 129,
	130, 131, 132, 133, 134, 135, 136, 137, 138, 139,
	140, 141, 142, 143, 144, 145, 146, 147, 148, 149,
	150, 151, 152, 153, 154, 155, 156, 157, 158, 159,
	160, 161, 162, 163, 164, 165, 166, 167, 168, 169,
	170, 171, 172, 173, 174, 175, 176, 177, 178, 179,
	180, 181, 182, 183, 184, 185, 186, 187, 188, 189,
	190, 191, 192, 193, 194, 195, 196, 197, 198, 199,
	200, 201, 202, 203, 204, 205, 206, 207, 208, 209,
	210, 211, 212, 213, 214, 215, 216, 217, 218, 219,
	220, 221, 222, 223, 224, 225, 226, 227, 228, 229,
	230, 231, 232, 233, 234, 235, 236, 237, 238, 239,
	240, 241, 242, 243, 244, 245, 246, 247, 248, 249,
	250, 251, 252, 253, 254, 255,
}

func rgba16ToByte(v uint32) uint8 {
	return rgba16To8[v>>8]
}

// ColorProcessorConfig configures octonion color processing
type ColorProcessorConfig struct {
	InkSeparation   bool    // Separate ink from background
	DenoiseStrength float64 // Octonion-based denoise (0-1)
	ContrastEnhance float64 // Color contrast enhancement (1.0 = none)
	BlueInkBoost    bool    // Enhance blue ink detection
	RedInkBoost     bool    // Enhance red ink detection
	WorkerCount     int     // Parallel workers (0 = NumCPU)
}

// ColorProcessorStats tracks processing statistics
type ColorProcessorStats struct {
	ImagesProcessed int
	TotalPixels     int64
	Duration        time.Duration
	PixelsPerSec    float64
}

// ColorProcessor processes color documents using octonion math
type ColorProcessor struct {
	config *ColorProcessorConfig
	stats  *ColorProcessorStats
	mu     sync.RWMutex
}

// DefaultColorProcessorConfig returns production defaults
func DefaultColorProcessorConfig() *ColorProcessorConfig {
	return &ColorProcessorConfig{
		InkSeparation:   true,
		DenoiseStrength: 0.3,
		ContrastEnhance: 1.2,
		BlueInkBoost:    true,
		RedInkBoost:     true,
		WorkerCount:     0, // Auto-detect
	}
}

// NewColorProcessor creates a new color processor
func NewColorProcessor(config *ColorProcessorConfig) *ColorProcessor {
	if config == nil {
		config = DefaultColorProcessorConfig()
	}
	return &ColorProcessor{
		config: config,
		stats:  &ColorProcessorStats{},
	}
}

// Process applies octonion-based color processing
func (cp *ColorProcessor) Process(ctx context.Context, img image.Image) (image.Image, error) {
	start := time.Now()
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	// Convert image to octonion field
	field := make([]Octonion, width*height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			field[y*width+x] = FromPixel(
				rgba16ToByte(r), rgba16ToByte(g), rgba16ToByte(b), rgba16ToByte(a),
				x, y, width, height,
				1.0, 0.0, // Default confidence and context
			)
		}
	}

	// Step 1: Ink Separation (project to ink subspace)
	if cp.config.InkSeparation {
		field = cp.separateInk(field, width, height)
	}

	// Step 2: Denoise via octonion neighborhood averaging
	if cp.config.DenoiseStrength > 0 {
		field = cp.denoise(field, width, height)
	}

	// Step 3: Contrast enhancement
	if cp.config.ContrastEnhance != 1.0 {
		field = cp.enhanceContrast(field)
	}

	// Step 4: Ink color boosting
	if cp.config.BlueInkBoost || cp.config.RedInkBoost {
		field = cp.boostInkColors(field)
	}

	// Convert back to image
	result := image.NewRGBA(bounds)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, a := field[y*width+x].ToRGBA()
			result.Set(x+bounds.Min.X, y+bounds.Min.Y, color.RGBA{r, g, b, a})
		}
	}

	// Update stats
	duration := time.Since(start)
	totalPixels := int64(width * height)
	pixelsPerSec := float64(totalPixels) / duration.Seconds()

	cp.mu.Lock()
	cp.stats.ImagesProcessed++
	cp.stats.TotalPixels += totalPixels
	cp.stats.Duration += duration
	cp.stats.PixelsPerSec = pixelsPerSec
	cp.mu.Unlock()

	return result, nil
}

// separateInk separates ink from background using octonion projection
func (cp *ColorProcessor) separateInk(field []Octonion, width, height int) []Octonion {
	// Compute mean color (background estimate)
	var mean Octonion
	for _, o := range field {
		mean = mean.Add(o)
	}
	mean = mean.Scale(1.0 / float64(len(field)))

	// Pixels far from mean in color space are likely ink
	result := make([]Octonion, len(field))
	for i, o := range field {
		dist := o.ProjectRGB().Distance(mean.ProjectRGB())

		// If far from background, enhance (it's ink)
		if dist > 0.2 {
			// Boost ink visibility
			result[i] = o
			result[i].E[0] = clamp(o.E[0]*1.5, 0, 1)
			result[i].E[1] = clamp(o.E[1]*1.5, 0, 1)
			result[i].E[2] = clamp(o.E[2]*1.5, 0, 1)
		} else {
			// Fade background
			result[i] = o
			result[i].E[0] = clamp(o.E[0]*0.8+0.2, 0, 1) // Push toward white
			result[i].E[1] = clamp(o.E[1]*0.8+0.2, 0, 1)
			result[i].E[2] = clamp(o.E[2]*0.8+0.2, 0, 1)
		}
	}

	return result
}

// denoise applies octonion-based denoising
func (cp *ColorProcessor) denoise(field []Octonion, width, height int) []Octonion {
	result := make([]Octonion, len(field))
	strength := cp.config.DenoiseStrength

	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			idx := y*width + x
			center := field[idx]

			// Compute neighborhood mean
			var sum Octonion
			count := 0
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					nIdx := (y+dy)*width + (x + dx)
					sum = sum.Add(field[nIdx])
					count++
				}
			}
			mean := sum.Scale(1.0 / float64(count))

			// Blend center with mean based on strength
			for i := 0; i < 8; i++ {
				result[idx].E[i] = center.E[i]*(1-strength) + mean.E[i]*strength
			}
		}
	}

	// Copy edges
	for x := 0; x < width; x++ {
		result[x] = field[x]
		result[(height-1)*width+x] = field[(height-1)*width+x]
	}
	for y := 0; y < height; y++ {
		result[y*width] = field[y*width]
		result[y*width+width-1] = field[y*width+width-1]
	}

	return result
}

// enhanceContrast enhances color contrast
func (cp *ColorProcessor) enhanceContrast(field []Octonion) []Octonion {
	factor := cp.config.ContrastEnhance
	result := make([]Octonion, len(field))

	for i, o := range field {
		result[i] = o
		// Enhance contrast for RGB channels
		for c := 0; c < 3; c++ {
			// Center around 0.5, scale, then shift back
			result[i].E[c] = clamp((o.E[c]-0.5)*factor+0.5, 0, 1)
		}
	}

	return result
}

// boostInkColors enhances blue and red ink detection
func (cp *ColorProcessor) boostInkColors(field []Octonion) []Octonion {
	result := make([]Octonion, len(field))

	for i, o := range field {
		result[i] = o

		// Detect blue ink (high B, low R)
		if cp.config.BlueInkBoost && o.E[2] > o.E[0]+0.1 && o.E[2] > 0.3 {
			result[i].E[2] = clamp(o.E[2]*1.3, 0, 1) // Boost blue
			result[i].E[0] = clamp(o.E[0]*0.7, 0, 1) // Reduce red
		}

		// Detect red ink (high R, low B)
		if cp.config.RedInkBoost && o.E[0] > o.E[2]+0.1 && o.E[0] > 0.3 {
			result[i].E[0] = clamp(o.E[0]*1.3, 0, 1) // Boost red
			result[i].E[2] = clamp(o.E[2]*0.7, 0, 1) // Reduce blue
		}
	}

	return result
}

// GetStats returns processing statistics
func (cp *ColorProcessor) GetStats() ColorProcessorStats {
	cp.mu.RLock()
	defer cp.mu.RUnlock()
	return ColorProcessorStats{
		ImagesProcessed: cp.stats.ImagesProcessed,
		TotalPixels:     cp.stats.TotalPixels,
		Duration:        cp.stats.Duration,
		PixelsPerSec:    cp.stats.PixelsPerSec,
	}
}

// ResetStats resets processing statistics
func (cp *ColorProcessor) ResetStats() {
	cp.mu.Lock()
	defer cp.mu.Unlock()
	cp.stats = &ColorProcessorStats{}
}

// String returns a summary of the processor configuration
func (cp *ColorProcessor) String() string {
	return fmt.Sprintf("OctonionColorProcessor[InkSep=%v, Denoise=%.2f, Contrast=%.2f, BlueBoost=%v, RedBoost=%v]",
		cp.config.InkSeparation,
		cp.config.DenoiseStrength,
		cp.config.ContrastEnhance,
		cp.config.BlueInkBoost,
		cp.config.RedInkBoost,
	)
}
