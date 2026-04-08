package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type deploymentEmployeeKey struct {
	DisplayName string
	Role        string
	Key         string
	Activated   bool
}

func resolveDeploymentSource(candidates ...string) (string, error) {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no deployment source found in candidates: %v", candidates)
}

func copySanitizedDeploymentDB(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0644); err != nil {
		return err
	}
	cleanupSQLiteSidecars(dst)

	db, err := gorm.Open(sqlite.Open(dst), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return err
	}

	if err := db.Exec(`
		UPDATE license_keys
		SET activated = 0,
		    activated_at = NULL,
		    device_hash = ''
		WHERE activated = 1
	`).Error; err != nil {
		return err
	}

	if err := db.Exec(`
		DELETE FROM license_keys
		WHERE role = 'developer'
		   OR key LIKE 'PH-DEV-%'
	`).Error; err != nil {
		return err
	}

	if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
		return err
	}
	if err := db.Exec("PRAGMA journal_mode=DELETE").Error; err != nil {
		return err
	}
	cleanupSQLiteSidecars(dst)
	return nil
}

func cleanupSQLiteSidecars(dbPath string) {
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
}

func copyDeploymentDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

func sanitizeAppBundle(appPath string, deploymentEnv []byte, dbSrc string) error {
	resourcesDir := filepath.Join(appPath, "Contents", "Resources")
	if err := os.MkdirAll(filepath.Join(resourcesDir, "data"), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(resourcesDir, ".env"), deploymentEnv, 0644); err != nil {
		return err
	}
	dbDst := filepath.Join(resourcesDir, "data", "ph_holdings.db")
	if err := copySanitizedDeploymentDB(dbSrc, dbDst); err != nil {
		return err
	}
	cleanupSQLiteSidecars(dbDst)
	return nil
}

func TestPrepareDeploymentPackage(t *testing.T) {
	dbPath := filepath.Join(".", "ph_holdings.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	app := &App{
		db:                     db,
		cache:                  NewCache(),
		startupImporting:       true,
		startupImportStartTime: time.Now(),
		currentUserID:          "manual-deploy-package",
		currentUser:            &User{Base: Base{ID: "manual-deploy-package"}, Username: "manual-deploy-package"},
	}

	if os.Getenv("FLUSH_EMPLOYEE_KEYS") == "1" {
		if err := db.Where("activated = 0").Delete(&LicenseKey{}).Error; err != nil {
			t.Fatalf("failed to flush unactivated keys: %v", err)
		}
		if err := app.SeedLicenseKeys(); err != nil {
			t.Fatalf("failed to seed generic keys: %v", err)
		}
		if err := app.SeedEmployeeKeys(); err != nil {
			t.Fatalf("failed to seed employee keys: %v", err)
		}
	}

	var employeeKeys []deploymentEmployeeKey
	if err := db.Raw(`
		SELECT display_name, role, key, activated
		FROM license_keys
		WHERE display_name IS NOT NULL AND display_name != '' AND activated = 0
		ORDER BY
			CASE role
				WHEN 'admin' THEN 1
				WHEN 'manager' THEN 2
				WHEN 'sales' THEN 3
				WHEN 'operations' THEN 4
				ELSE 99
			END,
			display_name
	`).Scan(&employeeKeys).Error; err != nil {
		t.Fatalf("failed to load employee keys: %v", err)
	}

	if len(employeeKeys) == 0 {
		t.Fatalf("no employee keys available to package")
	}

	_ = db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")

	stamp := os.Getenv("DEPLOY_STAMP")
	if strings.TrimSpace(stamp) == "" {
		stamp = time.Now().Format("2006_01_02_150405")
	}
	packageDir := filepath.Join("deploy_package", "AsymmFlow_Deploy_"+stamp)
	if err := os.RemoveAll(packageDir); err != nil {
		t.Fatalf("failed to reset package dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(packageDir, "data"), 0755); err != nil {
		t.Fatalf("failed to create package data dir: %v", err)
	}

	// Refresh top-level deploy_package bundle as well.
	if err := os.MkdirAll(filepath.Join("deploy_package", "data"), 0755); err != nil {
		t.Fatalf("failed to create top-level package data dir: %v", err)
	}

	macAppBundle := filepath.Join("build", "bin", "AsymmFlow.app")
	if _, err := os.Stat(macAppBundle); err != nil {
		t.Fatalf("failed to locate mac app bundle: %v", err)
	}

	macBinary, err := resolveDeploymentSource(
		filepath.Join("build", "bin", "AsymmFlow.app", "Contents", "MacOS", "AsymmFlow"),
		filepath.Join("deploy_package", "AsymmFlow"),
	)
	if err != nil {
		t.Fatalf("failed to resolve mac binary: %v", err)
	}
	winBinary, err := resolveDeploymentSource(
		filepath.Join("build", "bin", "AsymmFlow.exe"),
		filepath.Join("deploy_package", "AsymmFlow.exe"),
	)
	if err != nil {
		t.Fatalf("failed to resolve windows binary: %v", err)
	}
	envSrc := filepath.Join("deploy_package", ".env")
	dbSrc := filepath.Join(".", "ph_holdings.db")
	templateImage := filepath.Join("deploy_package", "data", "PH Trading Quotation Template.png")

	copyTargets := []struct {
		src string
		dst string
	}{
		{macBinary, filepath.Join(packageDir, "AsymmFlow")},
		{winBinary, filepath.Join(packageDir, "AsymmFlow.exe")},
		{macBinary, filepath.Join("deploy_package", "AsymmFlow")},
		{winBinary, filepath.Join("deploy_package", "AsymmFlow.exe")},
		{envSrc, filepath.Join(packageDir, ".env")},
		{envSrc, filepath.Join("deploy_package", ".env")},
		{templateImage, filepath.Join(packageDir, "data", "PH Trading Quotation Template.png")},
	}

	for _, target := range copyTargets {
		if err := copyDeploymentFile(target.src, target.dst); err != nil {
			t.Fatalf("failed to copy %s -> %s: %v", target.src, target.dst, err)
		}
	}

	deploymentEnv, err := buildDeploymentEnvText(envSrc)
	if err != nil {
		t.Fatalf("failed to build deployment env: %v", err)
	}

	// Sanitize the built app bundle in place before packaging so direct local testing
	// and packaged distribution both use the same clean license state.
	if err := sanitizeAppBundle(macAppBundle, deploymentEnv, dbSrc); err != nil {
		t.Fatalf("failed to sanitize mac app bundle: %v", err)
	}
	for _, target := range []string{
		filepath.Join(packageDir, ".env"),
		filepath.Join("deploy_package", ".env"),
	} {
		if err := os.WriteFile(target, deploymentEnv, 0644); err != nil {
			t.Fatalf("failed to write deployment env %s: %v", target, err)
		}
	}

	for _, target := range []string{
		filepath.Join(packageDir, "AsymmFlow.app"),
		filepath.Join("deploy_package", "AsymmFlow.app"),
	} {
		if err := os.RemoveAll(target); err != nil {
			t.Fatalf("failed to reset app bundle target %s: %v", target, err)
		}
		if err := copyDeploymentDir(macAppBundle, target); err != nil {
			t.Fatalf("failed to copy app bundle to %s: %v", target, err)
		}
		if err := sanitizeAppBundle(target, deploymentEnv, dbSrc); err != nil {
			t.Fatalf("failed to sanitize copied app bundle %s: %v", target, err)
		}
	}

	for _, dbTarget := range []string{
		filepath.Join(packageDir, "data", "ph_holdings.db"),
		filepath.Join(packageDir, "ph_holdings.db"),
		filepath.Join("deploy_package", "data", "ph_holdings.db"),
		filepath.Join("deploy_package", "ph_holdings.db"),
	} {
		if err := copySanitizedDeploymentDB(dbSrc, dbTarget); err != nil {
			t.Fatalf("failed to prepare sanitized deployment DB %s: %v", dbTarget, err)
		}
	}

	generatedAt := time.Now()
	licenseText := buildLicenseKeysText(employeeKeys, generatedAt)
	installGuide := buildInstallGuideText(generatedAt)

	for _, target := range []string{
		filepath.Join(packageDir, "LICENSE_KEYS.txt"),
		filepath.Join("deploy_package", "LICENSE_KEYS.txt"),
	} {
		if err := os.WriteFile(target, []byte(licenseText), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", target, err)
		}
	}

	for _, target := range []string{
		filepath.Join(packageDir, "INSTALL_GUIDE.txt"),
		filepath.Join("deploy_package", "INSTALL_GUIDE.txt"),
	} {
		if err := os.WriteFile(target, []byte(installGuide), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", target, err)
		}
	}

	t.Logf("package_dir=%s employee_keys=%d", packageDir, len(employeeKeys))
}

func copyDeploymentFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func buildDeploymentEnvText(src string) ([]byte, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, err
	}

	text := string(data)
	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines)+3)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "PH_DB_PATH=") || strings.HasPrefix(trimmed, "DATABASE_PATH=") {
			continue
		}
		filtered = append(filtered, line)
	}
	text = strings.TrimRight(strings.Join(filtered, "\n"), "\n")
	return []byte(text), nil
}

func buildLicenseKeysText(keys []deploymentEmployeeKey, generatedAt time.Time) string {
	var b strings.Builder
	b.WriteString("===============================================================================\n")
	b.WriteString("                    ASYMMFLOW ERP - LICENSE KEYS\n")
	b.WriteString(fmt.Sprintf("                    Generated: %s\n", generatedAt.Format("2006-01-02")))
	b.WriteString("===============================================================================\n\n")
	b.WriteString("INSTRUCTIONS:\n")
	b.WriteString("  1. Copy the deploy_package folder to your PC\n")
	b.WriteString("  2. Run AsymmFlow.exe (Windows) or AsymmFlow (Mac)\n")
	b.WriteString("  3. Enter YOUR license key when prompted\n")
	b.WriteString("  4. Each key works on ONE PC only - do not share keys\n\n")

	roleTitles := []struct {
		role  string
		title string
		desc  string
	}{
		{"admin", "ADMIN", "Full Access - Dashboard, Finance, Operations, CRM, Intelligence"},
		{"manager", "MANAGER", "Dashboard, Finance, Operations, CRM, Intelligence"},
		{"sales", "SALES", "Dashboard, Opportunities, CRM, Intelligence"},
		{"operations", "OPERATIONS", "Dashboard, Operations Hub, Intelligence"},
	}

	for _, rt := range roleTitles {
		b.WriteString("-------------------------------------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("%s (%s)\n", rt.title, rt.desc))
		b.WriteString("-------------------------------------------------------------------------------\n\n")
		for _, key := range keys {
			if key.Role != rt.role {
				continue
			}
			b.WriteString(fmt.Sprintf("  %-16s %s\n", key.DisplayName, key.Key))
		}
		b.WriteString("\n")
	}

	b.WriteString("===============================================================================\n")
	b.WriteString("                    ROLE ACCESS MATRIX\n")
	b.WriteString("===============================================================================\n\n")
	b.WriteString("  Screen            Admin   Manager   Sales   Operations\n")
	b.WriteString("  -------           -----   -------   -----   ----------\n")
	b.WriteString("  Dashboard           Y       Y         Y        Y\n")
	b.WriteString("  Opportunities       Y       Y         Y        -\n")
	b.WriteString("  Operations          Y       Y         -        Y\n")
	b.WriteString("  Finance Hub         Y       Y         -        -\n")
	b.WriteString("  Relationships       Y       Y         Y        -\n")
	b.WriteString("  Intelligence        Y       Y         Y        Y\n\n")
	b.WriteString("===============================================================================\n")
	b.WriteString("  SUPPORT: Contact Abhie (SPOC) for key issues or new key requests\n")
	b.WriteString("  CLOUD SYNC: Auto-syncs every 10 minutes when online\n")
	b.WriteString("===============================================================================\n")
	return b.String()
}

func buildInstallGuideText(generatedAt time.Time) string {
	return fmt.Sprintf(`===============================================================================
                ASYMMFLOW ERP - INSTALLATION GUIDE
                Version: %s | Deployment Package
===============================================================================

QUICK START (3 Steps)
---------------------

  1. COPY this entire folder to your PC
     - Windows: C:\AsymmFlow\
     - Mac: ~/AsymmFlow/

  2. RUN the application
     - Windows: Double-click AsymmFlow.exe
     - Mac: Run ./AsymmFlow from Terminal

  3. ENTER your license key when prompted
     - See LICENSE_KEYS.txt for your personal key
     - Key activates on first use and binds to your PC

That's it! The app will open to the Dashboard.


FOLDER STRUCTURE
----------------

  deploy_package/
  ├── AsymmFlow              <- The application (Mac)
  ├── AsymmFlow.exe          <- The application (Windows)
  ├── .env                   <- Cloud sync configuration
  ├── data/
  │   ├── ph_holdings.db     <- Database with all company data
  │   └── PH Trading Quotation Template.png
  ├── LICENSE_KEYS.txt       <- Employee license keys
  └── INSTALL_GUIDE.txt      <- This file


WINDOWS REQUIREMENTS
--------------------

  - WebView2 Runtime: Pre-installed on Windows 10 21H2+ and Windows 11
  - No other dependencies needed


CLOUD SYNC
----------

  - Data syncs automatically every 10 minutes
  - Green dot = Connected | Yellow = Connecting | Gray = Offline
  - App works fully offline - changes sync when reconnected
  - All PCs share the same data through Supabase cloud


TROUBLESHOOTING
---------------

  Problem                     Solution
  -------                     --------
  App won't start (Windows)   Install WebView2 Runtime
  App won't start             Make sure .env file is in same folder
  "Invalid license key"       Check LICENSE_KEYS.txt for correct key
  "Already activated"         Each key works on ONE PC only
  Database locked             Close other app instances
  Data not syncing            Check internet connection, wait 10 min
  Empty dashboard             Make sure data/ph_holdings.db exists

  For all issues: Contact Abhie (SPOC)

===============================================================================
  PH Trading WLL | Powered by Asymmetrica Mathematical Organism
===============================================================================
`, generatedAt.Format("2006-01-02"))
}
