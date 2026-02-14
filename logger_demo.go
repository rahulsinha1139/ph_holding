// +build ignore

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Println("🔥 PH Holdings Structured Logger Demo")
	fmt.Println("=" + string(make([]byte, 60)) + "=")
	fmt.Println()

	// Demo 1: Dev Mode (Console with Emojis)
	fmt.Println("📊 DEMO 1: Dev Mode (Console Output with Emojis)")
	fmt.Println("-" + string(make([]byte, 60)) + "-")
	devLogger := NewLogger(LoggerConfig{
		Mode:  "dev",
		Level: LevelDebug,
	})

	devLogger.Info("Application started", map[string]interface{}{
		"version":     "1.0.0",
		"environment": "development",
	})

	devLogger.Debug("Database connection established", map[string]interface{}{
		"host": "localhost",
		"port": 5432,
	})

	devLogger.Warn("Missing optional configuration", map[string]interface{}{
		"missing_key": "API_TOKEN",
		"using_default": true,
	})

	devLogger.Error("Database query timeout", errors.New("connection timeout after 5s"), map[string]interface{}{
		"query":       "SELECT * FROM customers",
		"duration_ms": 5000,
	})

	fmt.Println()

	// Demo 2: Production Mode (JSON Output)
	fmt.Println("📊 DEMO 2: Production Mode (JSON Output)")
	fmt.Println("-" + string(make([]byte, 60)) + "-")

	// Create temp file for JSON logs
	tmpFile, err := os.CreateTemp("", "ph_json_logs_*.log")
	if err != nil {
		fmt.Printf("❌ Failed to create temp file: %v\n", err)
		return
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	prodLogger := NewLogger(LoggerConfig{
		Mode:       "production",
		Level:      LevelInfo,
		OutputFile: tmpFile,
	})

	prodLogger.Info("Order created", map[string]interface{}{
		"order_id":    "ORD-2024-001",
		"customer_id": 42,
		"total":       1500.50,
		"currency":    "BHD",
	})

	prodLogger.Error("Payment processing failed", errors.New("insufficient funds"), map[string]interface{}{
		"order_id":      "ORD-2024-001",
		"amount":        1500.50,
		"account_balance": 1000.00,
	})

	// Flush and display
	tmpFile.Sync()
	content, _ := os.ReadFile(tmpFile.Name())
	fmt.Printf("JSON Logs (written to %s):\n", tmpFile.Name())
	fmt.Println(string(content))
	fmt.Println()

	// Demo 3: Request ID Tracking
	fmt.Println("📊 DEMO 3: Request ID Tracking (Context Propagation)")
	fmt.Println("-" + string(make([]byte, 60)) + "-")

	reqLogger := NewLogger(LoggerConfig{
		Mode:  "dev",
		Level: LevelInfo,
	})

	ctx := reqLogger.WithRequestID(context.Background(), "req-abc-123-xyz")

	reqLogger.InfoCtx(ctx, "Processing customer order", map[string]interface{}{
		"customer_id": 42,
		"action":      "create_order",
	})

	reqLogger.InfoCtx(ctx, "Order validation complete", map[string]interface{}{
		"validation_status": "passed",
	})

	fmt.Println()

	// Demo 4: Performance Metrics
	fmt.Println("📊 DEMO 4: Performance Metrics")
	fmt.Println("-" + string(make([]byte, 60)) + "-")

	perfLogger := NewLogger(LoggerConfig{
		Mode:  "dev",
		Level: LevelInfo,
	})

	start := time.Now()
	time.Sleep(50 * time.Millisecond) // Simulate work
	duration := time.Since(start)

	perfLogger.Performance("customer_360_query", duration, map[string]interface{}{
		"nodes_traversed": 1500,
		"edges_traversed": 3200,
		"depth":           5,
	})

	fmt.Println()

	// Demo 5: Business Intelligence Metrics
	fmt.Println("📊 DEMO 5: Business Intelligence Metrics")
	fmt.Println("-" + string(make([]byte, 60)) + "-")

	bizLogger := NewLogger(LoggerConfig{
		Mode:  "dev",
		Level: LevelInfo,
	})

	bizLogger.Business("monthly_revenue", 45000.50, map[string]interface{}{
		"currency": "BHD",
		"month":    "December",
		"year":     2024,
	})

	bizLogger.Business("orders_completed", 127, map[string]interface{}{
		"period": "daily",
		"date":   "2024-12-25",
	})

	fmt.Println()

	// Demo 6: GPU Operations
	fmt.Println("📊 DEMO 6: GPU Operations")
	fmt.Println("-" + string(make([]byte, 60)) + "-")

	gpuLogger := NewLogger(LoggerConfig{
		Mode:  "dev",
		Level: LevelInfo,
	})

	gpuLogger.GPU("quaternion_evolution", true, map[string]interface{}{
		"matrix_size":  "2048x2048",
		"duration_ms":  15,
		"throughput":   "71M ops/sec",
		"device":       "Intel Level Zero",
	})

	gpuLogger.GPU("gpu_initialization", false, map[string]interface{}{
		"error":    "device not found",
		"fallback": "CPU mode",
	})

	fmt.Println()

	// Demo 7: Security Events
	fmt.Println("📊 DEMO 7: Security Events")
	fmt.Println("-" + string(make([]byte, 60)) + "-")

	secLogger := NewLogger(LoggerConfig{
		Mode:  "dev",
		Level: LevelInfo,
	})

	secLogger.Security("login_attempt", true, map[string]interface{}{
		"username": "admin",
		"ip":       "192.168.1.100",
		"method":   "oauth2",
	})

	secLogger.Security("unauthorized_access_attempt", false, map[string]interface{}{
		"username": "unknown",
		"resource": "/api/admin/users",
		"ip":       "203.0.113.42",
		"blocked":  true,
	})

	fmt.Println()

	// Demo 8: Component Logger (Scoped Fields)
	fmt.Println("📊 DEMO 8: Component Logger (Scoped Fields)")
	fmt.Println("-" + string(make([]byte, 60)) + "-")

	baseLogger := NewLogger(LoggerConfig{
		Mode:  "dev",
		Level: LevelDebug,
	})

	ocrComponent := baseLogger.WithFields(map[string]interface{}{
		"component": "ocr_service",
		"version":   "2.0",
		"engine":    "ACE",
	})

	ocrComponent.Info("Processing document", map[string]interface{}{
		"document_id": "DOC-123",
		"pages":       5,
	})

	ocrComponent.Error("OCR extraction failed", errors.New("corrupted PDF"), map[string]interface{}{
		"document_id": "DOC-456",
		"error_code":  "ERR_CORRUPT",
	})

	fmt.Println()

	// Demo 9: Startup Banner
	fmt.Println("📊 DEMO 9: Startup Banner")
	fmt.Println("-" + string(make([]byte, 60)) + "-")

	startupLogger := NewLogger(LoggerConfig{
		Mode:  "dev",
		Level: LevelInfo,
	})

	startupLogger.Startup("PH Holdings Sovereign UI", map[string]interface{}{
		"version":    "1.0.0",
		"go_version": "1.25.5",
		"os":         "windows",
		"arch":       "amd64",
	})

	fmt.Println()
	fmt.Println("✅ Demo complete! Logger is production-ready.")
}
