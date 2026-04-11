// Asymmetrica Runtime OCR Processor (VPS asymm-engine, formerly Fly.io)
//
// Connects to the Asymmetrica Runtime now hosted on the Asymmetrica VPS at
// https://api.asymmetrica.ai/engine (reverse-proxied by Caddy to the
// asymm-engine container on port 8090). Replaces the retired Fly.io
// deployment at https://asymmetrica-runtime.fly.dev.
//
// Endpoint:
// - POST /api/ocr/cloud — single-shot OCR that accepts a base64 image and
//   returns { success, rawText, enhancedText, confidence, structuredData, error }.
//   The consciousness-enhanced pipeline (previously split across
//   batch-preprocess + quality-gate + table_extraction) is now handled
//   server-side inside /api/ocr/cloud and its downstream kernels.
//
// Auth:
// - All requests MUST send X-Asymm-Service-Token. The token is read from the
//   ASYMM_SERVICE_TOKEN environment variable (or the legacy FlyAPIKey field
//   in FlyOCRConfig, which is now repurposed for this). Without a valid token
//   the runtime returns 401 regardless of path.
//
// Type names retained as `Fly*` (FlyOCRProcessor, FlyOCRConfig, FlyOCRResult)
// purely to minimize blast radius on callers. Rename to `AsymmEngine*` in a
// follow-up refactor when there's slack in the schedule.
//
// Original build: January 20, 2026 — Wave 1 Backend OCR Integration.
// Migrated: April 11, 2026 — off Fly.io, onto Asymmetrica VPS.
package orchestrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// FlyOCRProcessor handles OCR via Asymmetrica Runtime.
// Despite the name, the runtime now lives on the Asymmetrica VPS
// (https://api.asymmetrica.ai/engine), not Fly.io. Name retained for
// caller compatibility — rename planned for a follow-up refactor.
type FlyOCRProcessor struct {
	apiURL       string
	serviceToken string // sent as X-Asymm-Service-Token on every request
	httpClient   *http.Client
	stats        *FlyOCRStats
	mu           sync.RWMutex
}

// FlyOCRConfig configures the OCR processor.
// APIKey is retained for back-compat but is now the X-Asymm-Service-Token
// value (not a bearer token). Prefer setting it via the ASYMM_SERVICE_TOKEN
// environment variable.
type FlyOCRConfig struct {
	APIURL  string
	APIKey  string // service token (X-Asymm-Service-Token value)
	Timeout time.Duration
}

// FlyOCRStats tracks Fly.io OCR statistics
type FlyOCRStats struct {
	TotalRequests   int
	SuccessCount    int
	ErrorCount      int
	TotalCharacters int
	TotalDuration   time.Duration
	TotalCost       float64 // Estimated cost in USD
	QualityPasses   int
	QualityFails    int
	TablesExtracted int
}

// FlyOCRResult represents OCR result from Fly.io
type FlyOCRResult struct {
	Success       bool          `json:"success"`
	Text          string        `json:"text"`
	Characters    int           `json:"characters"`
	Quality       string        `json:"quality"`       // "PASS" | "FAIL"
	Confidence    float64       `json:"confidence"`
	Duration      time.Duration `json:"-"`
	EstimatedCost float64       `json:"-"`
	TablesFound   int           `json:"tables_found"`
	EntitiesFound int           `json:"entities_found"`
	Error         string        `json:"error,omitempty"`
}

// AsymmEngineCloudResponse matches the /api/ocr/cloud response shape.
// The runtime returns a richer payload than the old batch-preprocess
// endpoint; we only parse the fields we actually use.
type AsymmEngineCloudResponse struct {
	Success          bool                   `json:"success"`
	RawText          string                 `json:"rawText"`
	EnhancedText     string                 `json:"enhancedText"`
	Confidence       float64                `json:"confidence"`
	ProcessingTimeMs int                    `json:"processingTimeMs"`
	StructuredData   map[string]interface{} `json:"structuredData"`
	GraphOperations  int                    `json:"graphOperations"`
	DocumentType     string                 `json:"documentType"`
	Error            string                 `json:"error,omitempty"`
}

// FlyTableExtractionResponse is retained for back-compat with ExtractTables().
// The underlying endpoint /api/kernel/table_extraction is GONE on the new
// runtime; ExtractTables() is now a stub that returns an empty result with
// a clear error. Callers that need table extraction should migrate to
// /api/ocr/document (full document pipeline) or use a dedicated kernel
// like FormExtractor via /api/kernels/FormExtractor/run.
type FlyTableExtractionResponse struct {
	Tables   []map[string]interface{} `json:"tables"`
	Entities []map[string]interface{} `json:"entities"`
}

// DefaultFlyOCRConfig returns production defaults.
// URL can be overridden via ASYMM_ENGINE_URL env var (falls back to the
// legacy FLY_OCR_URL for back-compat during the transition).
// The service token is read from ASYMM_SERVICE_TOKEN.
func DefaultFlyOCRConfig() *FlyOCRConfig {
	url := os.Getenv("ASYMM_ENGINE_URL")
	if url == "" {
		url = os.Getenv("FLY_OCR_URL") // legacy fallback
	}
	if url == "" {
		url = "https://api.asymmetrica.ai/engine"
	}
	return &FlyOCRConfig{
		APIURL:  url,
		APIKey:  os.Getenv("ASYMM_SERVICE_TOKEN"),
		Timeout: 60 * time.Second,
	}
}

// NewFlyOCRProcessor creates a new OCR processor pointed at the
// Asymmetrica Runtime (VPS asymm-engine).
func NewFlyOCRProcessor(config *FlyOCRConfig) (*FlyOCRProcessor, error) {
	if config == nil {
		config = DefaultFlyOCRConfig()
	}

	// If the caller passed a config with no APIKey but the env has one,
	// pick it up automatically. Keeps the call sites that build their own
	// config working without changes.
	token := config.APIKey
	if token == "" {
		token = os.Getenv("ASYMM_SERVICE_TOKEN")
	}

	return &FlyOCRProcessor{
		apiURL:       config.APIURL,
		serviceToken: token,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		stats: &FlyOCRStats{},
	}, nil
}

// OCRImage performs OCR on an image via the Asymmetrica Runtime.
// Calls /api/ocr/cloud as a single-shot request (preprocessing, OCR,
// consciousness enhancement, and quality assessment all happen server-side).
func (p *FlyOCRProcessor) OCRImage(ctx context.Context, img image.Image) (*FlyOCRResult, error) {
	start := time.Now()

	// Convert image to base64 JPEG
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		return nil, fmt.Errorf("failed to encode image: %w", err)
	}
	imageBase64 := base64.StdEncoding.EncodeToString(buf.Bytes())

	cloudReq := map[string]interface{}{
		"ImageBase64":  imageBase64,
		"DocumentType": "Generic",
	}
	respBody, err := p.post(ctx, "/api/ocr/cloud", cloudReq)
	if err != nil {
		p.recordError()
		return nil, fmt.Errorf("asymm-engine /api/ocr/cloud failed: %w", err)
	}

	var cloudResp AsymmEngineCloudResponse
	if err := json.Unmarshal(respBody, &cloudResp); err != nil {
		p.recordError()
		return nil, fmt.Errorf("failed to parse /api/ocr/cloud response: %w", err)
	}

	// The runtime returns 200 even on backend failures, signaling via the
	// Success bool and Error field instead of HTTP status. Honor that.
	if !cloudResp.Success {
		p.recordError()
		errMsg := cloudResp.Error
		if errMsg == "" {
			errMsg = "asymm-engine OCR returned success=false with no error message"
		}
		return &FlyOCRResult{
			Success: false,
			Error:   errMsg,
		}, fmt.Errorf("asymm-engine OCR failed: %s", errMsg)
	}

	text := cloudResp.RawText
	if text == "" {
		text = cloudResp.EnhancedText
	}
	if text == "" {
		p.recordError()
		return &FlyOCRResult{
			Success: false,
			Error:   "no text extracted",
		}, fmt.Errorf("no text extracted from image")
	}

	// Synthesize a legacy quality label for back-compat with callers that
	// expect the old batch-preprocess + quality-gate pipeline shape.
	confidence := cloudResp.Confidence
	quality := "PASS"
	if confidence < 0.6 {
		quality = "FAIL"
		p.recordQualityFail()
	} else {
		p.recordQualityPass()
	}

	duration := time.Since(start)
	// Cost is now effectively zero — we own the compute. Keep the field for
	// back-compat with stats/summary code but report $0.00.
	cost := 0.0

	result := &FlyOCRResult{
		Success:       true,
		Text:          text,
		Characters:    len(text),
		Quality:       quality,
		Confidence:    confidence,
		Duration:      duration,
		EstimatedCost: cost,
		TablesFound:   0,
		EntitiesFound: 0,
	}

	p.recordSuccess(result.Characters, duration, cost)

	return result, nil
}

// OCRImageBytes performs OCR on raw image bytes via /api/ocr/cloud.
// This is the byte-slice entry point; OCRImage uses the image.Image
// entry point. Both converge on the same runtime endpoint.
func (p *FlyOCRProcessor) OCRImageBytes(ctx context.Context, imageData []byte) (*FlyOCRResult, error) {
	start := time.Now()

	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	cloudReq := map[string]interface{}{
		"ImageBase64":  imageBase64,
		"DocumentType": "Generic",
	}
	respBody, err := p.post(ctx, "/api/ocr/cloud", cloudReq)
	if err != nil {
		p.recordError()
		return nil, fmt.Errorf("asymm-engine /api/ocr/cloud failed: %w", err)
	}

	var cloudResp AsymmEngineCloudResponse
	if err := json.Unmarshal(respBody, &cloudResp); err != nil {
		p.recordError()
		return nil, fmt.Errorf("failed to parse /api/ocr/cloud response: %w", err)
	}

	if !cloudResp.Success {
		p.recordError()
		errMsg := cloudResp.Error
		if errMsg == "" {
			errMsg = "asymm-engine OCR returned success=false with no error message"
		}
		return &FlyOCRResult{
			Success: false,
			Error:   errMsg,
		}, fmt.Errorf("asymm-engine OCR failed: %s", errMsg)
	}

	text := cloudResp.RawText
	if text == "" {
		text = cloudResp.EnhancedText
	}
	if text == "" {
		p.recordError()
		return &FlyOCRResult{
			Success: false,
			Error:   "no text extracted",
		}, fmt.Errorf("no text extracted from image")
	}

	confidence := cloudResp.Confidence
	quality := "PASS"
	if confidence < 0.6 {
		quality = "FAIL"
		p.recordQualityFail()
	} else {
		p.recordQualityPass()
	}

	duration := time.Since(start)
	cost := 0.0 // self-hosted compute

	result := &FlyOCRResult{
		Success:       true,
		Text:          text,
		Characters:    len(text),
		Quality:       quality,
		Confidence:    confidence,
		Duration:      duration,
		EstimatedCost: cost,
		TablesFound:   0,
		EntitiesFound: 0,
	}

	p.recordSuccess(result.Characters, duration, cost)

	return result, nil
}

// ExtractTables is retained as a stub for back-compat.
// The old /api/kernel/table_extraction endpoint has been REMOVED from the
// Asymmetrica Runtime — tables and entities are now produced inline by
// /api/ocr/document (full pipeline) or via dedicated kernel dispatch at
// /api/kernels/FormExtractor/run (which expects docs already loaded into
// the runtime workspace).
//
// This stub returns an empty result with a descriptive error rather than
// panicking. Callers who actually need tables should migrate to use
// /api/ocr/document through a new helper and parse StructuredData from
// the richer cloud/document response shape.
//
// ExtractTables has ZERO callers in ph_holdings as of 2026-04-11 (verified
// by grep), so the stub path is never actually exercised in production.
func (p *FlyOCRProcessor) ExtractTables(ctx context.Context, text string) (*FlyTableExtractionResponse, error) {
	_ = ctx
	_ = text
	return &FlyTableExtractionResponse{
			Tables:   []map[string]interface{}{},
			Entities: []map[string]interface{}{},
		}, fmt.Errorf("ExtractTables is deprecated: /api/kernel/table_extraction no longer exists on the Asymmetrica Runtime. Use /api/ocr/document for full document analysis including tables")
}

// HealthCheck verifies the Asymmetrica Runtime is reachable and our
// service token is accepted. Uses /api/catalog as the health probe —
// it's a lightweight auth-gated GET that returns the kernel list.
// The old /health endpoint was unreliable (returned 500 in testing).
func (p *FlyOCRProcessor) HealthCheck(ctx context.Context) error {
	url := p.apiURL + "/api/catalog"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	if p.serviceToken != "" {
		req.Header.Set("X-Asymm-Service-Token", p.serviceToken)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("health check failed: HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// post sends an authenticated POST to the Asymmetrica Runtime.
// Sends X-Asymm-Service-Token on every request (required by the backend).
func (p *FlyOCRProcessor) post(ctx context.Context, endpoint string, reqData interface{}) ([]byte, error) {
	jsonBody, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := p.apiURL + endpoint

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if p.serviceToken != "" {
		req.Header.Set("X-Asymm-Service-Token", p.serviceToken)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// recordSuccess updates stats on success
func (p *FlyOCRProcessor) recordSuccess(chars int, duration time.Duration, cost float64) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalRequests++
	p.stats.SuccessCount++
	p.stats.TotalCharacters += chars
	p.stats.TotalDuration += duration
	p.stats.TotalCost += cost
}

// recordError updates stats on error
func (p *FlyOCRProcessor) recordError() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.stats.TotalRequests++
	p.stats.ErrorCount++
}

// recordWarning logs a warning (quality gate failure, etc.)
func (p *FlyOCRProcessor) recordWarning() {
	// No-op for now, could add warning counter if needed
}

// recordQualityPass updates stats on quality gate pass
func (p *FlyOCRProcessor) recordQualityPass() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats.QualityPasses++
}

// recordQualityFail updates stats on quality gate fail
func (p *FlyOCRProcessor) recordQualityFail() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stats.QualityFails++
}

// GetStats returns OCR statistics
func (p *FlyOCRProcessor) GetStats() *FlyOCRStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return &FlyOCRStats{
		TotalRequests:   p.stats.TotalRequests,
		SuccessCount:    p.stats.SuccessCount,
		ErrorCount:      p.stats.ErrorCount,
		TotalCharacters: p.stats.TotalCharacters,
		TotalDuration:   p.stats.TotalDuration,
		TotalCost:       p.stats.TotalCost,
		QualityPasses:   p.stats.QualityPasses,
		QualityFails:    p.stats.QualityFails,
		TablesExtracted: p.stats.TablesExtracted,
	}
}

// Summary returns a formatted summary
func (p *FlyOCRProcessor) Summary() string {
	stats := p.GetStats()

	avgDuration := time.Duration(0)
	throughput := float64(0)
	qualityRate := float64(0)

	if stats.SuccessCount > 0 {
		avgDuration = stats.TotalDuration / time.Duration(stats.SuccessCount)
		throughput = float64(stats.SuccessCount) / stats.TotalDuration.Seconds()
	}

	if stats.QualityPasses+stats.QualityFails > 0 {
		qualityRate = float64(stats.QualityPasses) * 100.0 / float64(stats.QualityPasses+stats.QualityFails)
	}

	return fmt.Sprintf(`
🚀 ASYMMETRICA RUNTIME OCR SUMMARY
═══════════════════════════════════════════════════
Requests:      %d total, %d success, %d errors
Characters:    %d extracted
Duration:      %v total, %v avg/request
Throughput:    %.2f pages/sec
Cost:          $%.4f (self-hosted — no per-request pricing)
Quality:       %.1f%% pass rate (%d passed, %d failed)
Tables:        %d extracted
Deployment:    %s (asymm-engine on VPS)

Performance:
  Latency: ~2s per page when backend OCR provider is healthy
  Cost:    $0.00 per page (self-hosted compute, no per-call pricing)
  Engine:  Consciousness-enhanced pipeline via /api/ocr/cloud
`,
		stats.TotalRequests, stats.SuccessCount, stats.ErrorCount,
		stats.TotalCharacters,
		stats.TotalDuration, avgDuration,
		throughput,
		stats.TotalCost,
		qualityRate, stats.QualityPasses, stats.QualityFails,
		stats.TablesExtracted,
		p.apiURL,
	)
}

// Close cleans up processor resources
func (p *FlyOCRProcessor) Close() error {
	// Nothing to clean up for HTTP client
	return nil
}
