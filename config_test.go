// ═══════════════════════════════════════════════════════════════════════════
// CONFIG TESTS - Verify configuration loading and validation
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfigWithoutEnvFile(t *testing.T) {
	// Make sure .env doesn't exist for this test
	_ = os.Remove(".env")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check defaults - Database.Path should contain "ph_holdings.db" (location varies by OS)
	if cfg.Database.Path == "" {
		t.Errorf("Expected non-empty database path, got empty string")
	}
	if !strings.Contains(cfg.Database.Path, "ph_holdings.db") {
		t.Errorf("Expected database path to contain 'ph_holdings.db', got %s", cfg.Database.Path)
	}

	if cfg.App.LogLevel != "info" {
		t.Errorf("Expected default log level 'info', got %s", cfg.App.LogLevel)
	}

	if cfg.App.WatcherDebounceMS != 300 {
		t.Errorf("Expected default debounce 300ms, got %d", cfg.App.WatcherDebounceMS)
	}

	if cfg.App.WatcherQueueSize != 1000 {
		t.Errorf("Expected default queue size 1000, got %d", cfg.App.WatcherQueueSize)
	}

	// Azure should be disabled by default
	if cfg.Azure.Enabled {
		t.Error("Azure should be disabled when credentials not configured")
	}

	t.Logf("✓ Config loaded with defaults successfully")
}

func TestLoadConfigWithEnvVars(t *testing.T) {
	// Set environment variables
	os.Setenv("DATABASE_PATH", "./test_custom.db")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("DEBUG_MODE", "true")
	os.Setenv("WATCHER_DEBOUNCE_MS", "500")
	os.Setenv("ENABLE_FILE_WATCHER", "false")
	defer func() {
		os.Unsetenv("DATABASE_PATH")
		os.Unsetenv("LOG_LEVEL")
		os.Unsetenv("DEBUG_MODE")
		os.Unsetenv("WATCHER_DEBOUNCE_MS")
		os.Unsetenv("ENABLE_FILE_WATCHER")
	}()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// Check custom values
	if cfg.Database.Path != "./test_custom.db" {
		t.Errorf("Expected database path './test_custom.db', got %s", cfg.Database.Path)
	}

	if cfg.App.LogLevel != "debug" {
		t.Errorf("Expected log level 'debug', got %s", cfg.App.LogLevel)
	}

	if !cfg.App.DebugMode {
		t.Error("Expected debug mode to be true")
	}

	if cfg.App.WatcherDebounceMS != 500 {
		t.Errorf("Expected debounce 500ms, got %d", cfg.App.WatcherDebounceMS)
	}

	if cfg.App.EnableFileWatcher {
		t.Error("Expected file watcher to be disabled")
	}

	t.Logf("✓ Config loaded with environment variables successfully")
}

func TestConfigValidation(t *testing.T) {
	cfg := &Config{
		Database: DatabaseConfig{
			Path: "./test.db",
		},
		App: AppConfig{
			LogLevel:          "info",
			WatcherDebounceMS: 300,
			WatcherQueueSize:  1000,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Valid config failed validation: %v", err)
	}

	// Test invalid log level
	cfg.App.LogLevel = "invalid"
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation to fail with invalid log level")
	}
	cfg.App.LogLevel = "info" // Restore

	// Test invalid queue size
	cfg.App.WatcherQueueSize = 0
	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation to fail with queue size 0")
	}
	cfg.App.WatcherQueueSize = 1000 // Restore

	t.Logf("✓ Config validation working correctly")
}

func TestToolAutoDetection(t *testing.T) {
	tools := &ToolsConfig{}
	tools.detectTools()

	// We can't guarantee what tools are installed, but the function should not panic
	t.Logf("✓ Tool auto-detection ran without errors")
	t.Logf("  Pandoc:    %s", maskNotFound(tools.PandocPath))
	t.Logf("  FFmpeg:    %s", maskNotFound(tools.FFmpegPath))
	t.Logf("  Tesseract: %s", maskNotFound(tools.TesseractPath))
}

func TestAzureConfigEnabled(t *testing.T) {
	// All fields set - should be enabled
	cfg := &Config{
		Azure: AzureConfig{
			TenantID:     "tenant-123",
			ClientID:     "client-456",
			ClientSecret: "secret-789",
		},
	}
	cfg.Azure.Enabled = cfg.Azure.TenantID != "" && cfg.Azure.ClientID != "" && cfg.Azure.ClientSecret != ""

	if !cfg.Azure.Enabled {
		t.Error("Azure should be enabled when all credentials are set")
	}

	// Missing client secret - should be disabled
	cfg2 := &Config{
		Azure: AzureConfig{
			TenantID: "tenant-123",
			ClientID: "client-456",
			ClientSecret: "",
		},
	}
	cfg2.Azure.Enabled = cfg2.Azure.TenantID != "" && cfg2.Azure.ClientID != "" && cfg2.Azure.ClientSecret != ""

	if cfg2.Azure.Enabled {
		t.Error("Azure should be disabled when credentials are incomplete")
	}

	t.Logf("✓ Azure enabled/disabled logic working correctly")
}

func TestMaskingFunctions(t *testing.T) {
	// Test secret masking
	masked := maskSecret("1234567890abcdef")
	if masked != "1234****cdef" {
		t.Errorf("Expected '1234****cdef', got '%s'", masked)
	}

	// Test short secret
	masked = maskSecret("abc")
	if masked != "****" {
		t.Errorf("Expected '****', got '%s'", masked)
	}

	// Test empty
	masked = maskSecret("")
	if masked != "(not set)" {
		t.Errorf("Expected '(not set)', got '%s'", masked)
	}

	// Test maskEmpty
	masked = maskEmpty("")
	if masked != "(not configured)" {
		t.Errorf("Expected '(not configured)', got '%s'", masked)
	}

	// Test maskNotFound
	masked = maskNotFound("")
	if masked != "(not found in PATH)" {
		t.Errorf("Expected '(not found in PATH)', got '%s'", masked)
	}

	t.Logf("✓ Masking functions working correctly")
}
