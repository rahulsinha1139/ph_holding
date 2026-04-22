package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
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

type deploymentRoleKeySpec struct {
	Role        string
	DisplayName string
	Key         string
	Notes       string
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

	if err := sanitizeDeploymentDatabase(db); err != nil {
		return err
	}
	if err := ensureDeploymentLetterheadAssets(db); err != nil {
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

func sanitizeDeploymentDatabase(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	if db.Migrator().HasTable("license_keys") {
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
	}

	if db.Migrator().HasTable("expense_entries") {
		if db.Migrator().HasTable("expense_approvals") {
			if err := db.Exec(`
				DELETE FROM expense_approvals
				WHERE expense_entry_id IN (
					SELECT id FROM expense_entries
					WHERE source_type = 'payroll' OR cost_center = 'Payroll'
				)
			`).Error; err != nil {
				return err
			}
		}
		if db.Migrator().HasTable("expense_allocations") {
			if err := db.Exec(`
				DELETE FROM expense_allocations
				WHERE expense_entry_id IN (
					SELECT id FROM expense_entries
					WHERE source_type = 'payroll' OR cost_center = 'Payroll'
				)
			`).Error; err != nil {
				return err
			}
		}
		if db.Migrator().HasTable("expense_attachments") {
			if err := db.Exec(`
				DELETE FROM expense_attachments
				WHERE expense_entry_id IN (
					SELECT id FROM expense_entries
					WHERE source_type = 'payroll' OR cost_center = 'Payroll'
				)
			`).Error; err != nil {
				return err
			}
		}
		if err := db.Exec(`
			DELETE FROM expense_entries
			WHERE source_type = 'payroll' OR cost_center = 'Payroll'
		`).Error; err != nil {
			return err
		}
	}

	if db.Migrator().HasTable("journal_entries") {
		if db.Migrator().HasTable("journal_lines") {
			if err := db.Exec(`
				DELETE FROM journal_lines
				WHERE entry_id IN (
					SELECT id FROM journal_entries
					WHERE source_type IN ('payroll_run', 'payroll_payout')
				)
			`).Error; err != nil {
				return err
			}
		}
		if err := db.Exec(`
			DELETE FROM journal_entries
			WHERE source_type IN ('payroll_run', 'payroll_payout')
		`).Error; err != nil {
			return err
		}
	}

	for _, table := range []string{
		"payroll_components",
		"payroll_run_items",
		"payroll_payouts",
		"payroll_runs",
		"payroll_periods",
		"employee_compensation_profiles",
	} {
		if db.Migrator().HasTable(table) {
			if err := db.Exec("DELETE FROM " + table).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func cleanupSQLiteSidecars(dbPath string) {
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
}

func ensureDeploymentLetterheadAssets(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	assets := []struct {
		Name        string
		FileName    string
		Description string
	}{
		{
			Name:        AssetLetterhead,
			FileName:    "PH Trading Quotation Template.png",
			Description: "PH Trading letterhead template for PDF generation",
		},
		{
			Name:        AssetLetterheadAHS,
			FileName:    "AHS Trading Letterhead.jpg",
			Description: "AHS Trading letterhead template for PDF generation",
		},
	}

	for _, assetSpec := range assets {
		var count int64
		if err := db.Model(&Asset{}).Where("name = ?", assetSpec.Name).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		sourcePath, err := resolveDeploymentSource(
			filepath.Join("CRITICAL_PH_CONTEXT_SSOT", assetSpec.FileName),
			filepath.Join("deploy_package", "data", assetSpec.FileName),
			filepath.Join("build", "bin", "data", assetSpec.FileName),
		)
		if err != nil {
			return fmt.Errorf("failed to resolve %s: %w", assetSpec.Name, err)
		}

		data, err := os.ReadFile(sourcePath)
		if err != nil {
			return err
		}

		encoded := base64.StdEncoding.EncodeToString(data)
		record := Asset{
			ID:          assetSpec.Name,
			Name:        assetSpec.Name,
			Description: assetSpec.Description,
			MimeType:    getMimeType(filepath.Ext(sourcePath)),
			Data:        encoded,
			Size:        int64(len(data)),
		}

		if err := db.Where("name = ?", assetSpec.Name).Assign(record).FirstOrCreate(&record).Error; err != nil {
			return err
		}
	}

	return nil
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
	if err := syncDeploymentAppIcon(resourcesDir); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(resourcesDir, ".env"), deploymentEnv, 0644); err != nil {
		return err
	}
	dbDst := filepath.Join(resourcesDir, "data", "ph_holdings.db")
	if err := copySanitizedDeploymentDB(dbSrc, dbDst); err != nil {
		return err
	}
	for _, assetFile := range []string{
		"PH Trading Quotation Template.png",
		"AHS Trading Letterhead.jpg",
	} {
		src, err := resolveDeploymentSource(
			filepath.Join("CRITICAL_PH_CONTEXT_SSOT", assetFile),
			filepath.Join("deploy_package", "data", assetFile),
		)
		if err != nil {
			return err
		}
		if err := copyDeploymentFile(src, filepath.Join(resourcesDir, "data", assetFile)); err != nil {
			return err
		}
	}
	cleanupSQLiteSidecars(dbDst)
	return nil
}

func syncDeploymentAppIcon(resourcesDir string) error {
	iconSrc, err := resolveDeploymentSource(
		filepath.Join("build", "iconfile.icns"),
		filepath.Join("build", "darwin", "iconfile.icns"),
	)
	if err != nil {
		return nil
	}
	iconDst := filepath.Join(resourcesDir, "iconfile.icns")
	return copyDeploymentFile(iconSrc, iconDst)
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
		if err := resetDeploymentLicensePool(app, db); err != nil {
			t.Fatalf("failed to rebuild deployment license pool: %v", err)
		}
	}

	if err := ensureDeploymentEmployeeKeys(db); err != nil {
		t.Fatalf("failed to ensure deployment employee keys: %v", err)
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
				WHEN 'staff' THEN 5
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
	ahsTemplateImage := filepath.Join("CRITICAL_PH_CONTEXT_SSOT", "AHS Trading Letterhead.jpg")

	copyTargets := []struct {
		src string
		dst string
	}{
		{winBinary, filepath.Join(packageDir, "AsymmFlow.exe")},
		{winBinary, filepath.Join("deploy_package", "AsymmFlow.exe")},
		{envSrc, filepath.Join(packageDir, ".env")},
		{envSrc, filepath.Join("deploy_package", ".env")},
		{templateImage, filepath.Join(packageDir, "data", "PH Trading Quotation Template.png")},
		{ahsTemplateImage, filepath.Join(packageDir, "data", "AHS Trading Letterhead.jpg")},
		{templateImage, filepath.Join("deploy_package", "data", "PH Trading Quotation Template.png")},
		{ahsTemplateImage, filepath.Join("deploy_package", "data", "AHS Trading Letterhead.jpg")},
		{filepath.Join("docs", "DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.md"), filepath.Join(packageDir, "DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.md")},
		{filepath.Join("docs", "DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.pdf"), filepath.Join(packageDir, "DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.pdf")},
		{filepath.Join("docs", "MENTOR_2025_ONEDRIVE_IMPORT_GUIDE_2026_04_10.md"), filepath.Join(packageDir, "MENTOR_2025_ONEDRIVE_IMPORT_GUIDE_2026_04_10.md")},
		{filepath.Join("docs", "DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.md"), filepath.Join("deploy_package", "DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.md")},
		{filepath.Join("docs", "DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.pdf"), filepath.Join("deploy_package", "DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.pdf")},
		{filepath.Join("docs", "MENTOR_2025_ONEDRIVE_IMPORT_GUIDE_2026_04_10.md"), filepath.Join("deploy_package", "MENTOR_2025_ONEDRIVE_IMPORT_GUIDE_2026_04_10.md")},
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
	} {
		if err := os.WriteFile(target, []byte(licenseText), 0644); err != nil {
			t.Fatalf("failed to write %s: %v", target, err)
		}
	}
	if err := os.WriteFile(filepath.Join("deploy_package", "LICENSE_KEYS.txt"), []byte(buildLicenseKeysPlaceholderText(generatedAt)), 0644); err != nil {
		t.Fatalf("failed to write top-level license placeholder: %v", err)
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
		if strings.HasPrefix(trimmed, "PH_DB_PATH=") ||
			strings.HasPrefix(trimmed, "DATABASE_PATH=") ||
			strings.HasPrefix(trimmed, "ASYMMFLOW_FLUSH_LICENSE_ON_RESEED=") ||
			strings.HasPrefix(trimmed, "ASYMMFLOW_LICENSE_FLUSH_STAMP=") {
			continue
		}
		filtered = append(filtered, line)
	}
	text = strings.TrimRight(strings.Join(filtered, "\n"), "\n")
	if text != "" {
		text += "\n"
	}
	text += "ASYMMFLOW_FLUSH_LICENSE_ON_RESEED=true\n"
	if stamp := strings.TrimSpace(os.Getenv("DEPLOY_STAMP")); stamp != "" {
		text += fmt.Sprintf("ASYMMFLOW_LICENSE_FLUSH_STAMP=%s\n", stamp)
	}
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
	b.WriteString("  2. Run AsymmFlow.exe (Windows) or open AsymmFlow.app (Mac)\n")
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
		{"staff", "STAFF", "Dashboard, Work, Notifications, Intelligence"},
	}

	for _, rt := range roleTitles {
		roleKeys := make([]deploymentEmployeeKey, 0, len(keys))
		for _, key := range keys {
			if key.Role == rt.role {
				roleKeys = append(roleKeys, key)
			}
		}
		if len(roleKeys) == 0 {
			continue
		}

		b.WriteString("-------------------------------------------------------------------------------\n")
		b.WriteString(fmt.Sprintf("%s (%s)\n", rt.title, rt.desc))
		b.WriteString("-------------------------------------------------------------------------------\n\n")
		for _, key := range roleKeys {
			b.WriteString(fmt.Sprintf("  %-16s %s\n", key.DisplayName, key.Key))
		}
		b.WriteString("\n")
	}

	b.WriteString("===============================================================================\n")
	b.WriteString("                    ROLE ACCESS MATRIX\n")
	b.WriteString("===============================================================================\n\n")
	b.WriteString("  Screen            Admin   Manager   Sales   Operations   Staff\n")
	b.WriteString("  -------           -----   -------   -----   ----------   -----\n")
	b.WriteString("  Dashboard           Y       Y         Y        Y           Y\n")
	b.WriteString("  Opportunities       Y       Y         Y        -           -\n")
	b.WriteString("  Operations          Y       Y         -        Y           -\n")
	b.WriteString("  Finance Hub         Y       Y         -        -           -\n")
	b.WriteString("  Work                Y       Y         Y        Y           Y\n")
	b.WriteString("  Notifications       Y       Y         Y        Y           Y\n")
	b.WriteString("  Relationships       Y       Y         Y        -           -\n")
	b.WriteString("  Intelligence        Y       Y         Y        Y           Y\n\n")
	b.WriteString("===============================================================================\n")
	b.WriteString("  SUPPORT: Contact Abhie (SPOC) for key issues or new key requests\n")
	b.WriteString("  CLOUD SYNC: Auto-syncs every 10 minutes when online\n")
	b.WriteString("===============================================================================\n")
	return b.String()
}

func buildLicenseKeysPlaceholderText(generatedAt time.Time) string {
	return fmt.Sprintf(`===============================================================================
                    ASYMMFLOW ERP - LICENSE KEYS
                    Generated: %s
===============================================================================

This tracked top-level file is intentionally a placeholder.

The employee license keys are generated only inside the timestamped deployment
package produced by TestPrepareDeploymentPackage:

  deploy_package/AsymmFlow_Deploy_<stamp>/LICENSE_KEYS.txt

Do not commit generated employee keys, packaged .env files, packaged databases,
or historical deployment package snapshots back to source control.
===============================================================================
`, generatedAt.Format("2006-01-02"))
}

func buildInstallGuideText(generatedAt time.Time) string {
	return fmt.Sprintf(`===============================================================================
                ASYMMFLOW ERP - INSTALLATION GUIDE
                Version: %s | Deployment Package
===============================================================================

QUICK START (3 Steps)
---------------------

  1. COPY this entire folder to your PC as-is
     - Windows: C:\AsymmFlow\
     - Mac: ~/AsymmFlow/
     - Do NOT move AsymmFlow.exe or AsymmFlow.app out of this folder
     - Do NOT separate the app from the data/ folder

  2. RUN the application
     - Windows: Double-click AsymmFlow.exe
     - Mac: Open AsymmFlow.app

  3. ENTER your license key when prompted
     - See LICENSE_KEYS.txt for your personal employee key
     - Key activates on first use and binds to your PC

That's it! The app will open to the Dashboard.


IMPORTANT DATABASE RULE
-----------------------

  - The packaged data/ph_holdings.db is a first-run seed database.
  - On first launch, AsymmFlow copies it to the machine app-data folder and uses that persistent copy.
  - Bank statements, OCR imports, invoices, expenses, and license activation live in app data and survive rebuilds.
  - On upgrades, replace the app/package; do not delete the app-data folder unless you intend to reset local data.


FOLDER STRUCTURE
----------------

  deploy_package/
  ├── AsymmFlow.app          <- The application (Mac)
  ├── AsymmFlow.exe          <- The application (Windows)
  ├── .env                   <- Cloud sync configuration
  ├── data/
  │   ├── ph_holdings.db     <- Database with all company data
  │   └── PH Trading Quotation Template.png
  ├── LICENSE_KEYS.txt       <- Named PH employee license keys
  ├── DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.md
  ├── DEPLOYMENT_SIGNOFF_CHECKLIST_2026_04_08.pdf
  ├── MENTOR_2025_ONEDRIVE_IMPORT_GUIDE_2026_04_10.md
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
  Empty dashboard             Make sure data/ph_holdings.db exists for first launch
  Wrong or old database       Confirm the app-data database is the intended live database
  Wrong or old database       Replace the whole package folder, not just the app binary

  For all issues: Contact Abhie (SPOC)
===============================================================================
  PH Trading WLL | Powered by Asymmetrica Mathematical Organism
===============================================================================
`, generatedAt.Format("2006-01-02"))
}

func resetDeploymentLicensePool(app *App, db *gorm.DB) error {
	if app == nil || db == nil {
		return fmt.Errorf("deployment license pool requires initialized app/db")
	}
	if err := db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&LicenseKey{}).Error; err != nil {
		return err
	}
	return ensureDeploymentEmployeeKeys(db)
}

func ensureDeploymentEmployeeKeys(db *gorm.DB) error {
	for _, candidate := range deploymentEmployeeKeySpecs() {
		canonicalKey := strings.ToUpper(strings.TrimSpace(candidate.Key))
		if canonicalKey != "" {
			var byKey LicenseKey
			if err := db.Where("key = ?", canonicalKey).First(&byKey).Error; err == nil {
				if err := db.Model(&byKey).Updates(map[string]interface{}{
					"role":         candidate.Role,
					"display_name": candidate.DisplayName,
					"activated":    false,
					"activated_at": nil,
					"device_hash":  "",
					"notes":        candidate.Notes,
					"created_by":   "deployment-package",
				}).Error; err != nil {
					return err
				}
				continue
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			var byName LicenseKey
			if err := db.Where("role = ? AND display_name = ?", candidate.Role, candidate.DisplayName).
				Order("id ASC").
				First(&byName).Error; err == nil {
				if err := db.Model(&byName).Updates(map[string]interface{}{
					"key":          canonicalKey,
					"activated":    false,
					"activated_at": nil,
					"device_hash":  "",
					"notes":        candidate.Notes,
					"created_by":   "deployment-package",
				}).Error; err != nil {
					return err
				}
				continue
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}

			record := LicenseKey{
				Key:         canonicalKey,
				Role:        candidate.Role,
				DisplayName: candidate.DisplayName,
				Activated:   false,
				Notes:       candidate.Notes,
				CreatedBy:   "deployment-package",
			}
			if err := db.Create(&record).Error; err != nil {
				return err
			}
			continue
		}

		var existing LicenseKey
		if err := db.Where("role = ? AND display_name = ?", candidate.Role, candidate.DisplayName).First(&existing).Error; err == nil {
			if err := db.Model(&existing).Updates(map[string]interface{}{
				"activated":    false,
				"activated_at": nil,
				"device_hash":  "",
				"notes":        candidate.Notes,
				"created_by":   "deployment-package",
			}).Error; err != nil {
				return err
			}
			continue
		}

		prefix, ok := rolePrefixes[candidate.Role]
		if !ok {
			return fmt.Errorf("no license prefix configured for role %s", candidate.Role)
		}
		randomBytes := make([]byte, 3)
		if _, err := rand.Read(randomBytes); err != nil {
			return err
		}
		record := LicenseKey{
			Key:         fmt.Sprintf("PH-%s-%s", prefix, strings.ToUpper(hex.EncodeToString(randomBytes))),
			Role:        candidate.Role,
			DisplayName: candidate.DisplayName,
			Activated:   false,
			Notes:       candidate.Notes,
			CreatedBy:   "deployment-package",
		}
		if err := db.Create(&record).Error; err != nil {
			return err
		}
	}

	return nil
}

func deploymentEmployeeKeySpecs() []deploymentRoleKeySpec {
	source := phTradingNamedLicenseSpecs()
	specs := make([]deploymentRoleKeySpec, 0, len(source))
	for _, item := range source {
		specs = append(specs, deploymentRoleKeySpec{
			Role:        item.Role,
			DisplayName: item.DisplayName,
			Key:         item.Key,
			Notes:       item.Notes,
		})
	}
	return specs
}
