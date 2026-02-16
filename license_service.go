// ============================================================================
// LICENSE SERVICE - Key-Based Installer System
//
// MISSION: Generate and validate license keys that bind roles to PCs
//
// KEY FORMAT: PH-{ROLE}-{6-char-hash}
//   - PH-ADM-A1B2C3 = Admin
//   - PH-MGR-X4Y5Z6 = Manager
//   - PH-SLS-M7N8O9 = Sales
//   - PH-OPS-P0Q1R2 = Operations
//
// ARCHITECTURE:
//   1. Key binds to PC (device hash)
//   2. Role determines feature access
//   3. Butler AI queries restricted by role
//   4. License persists in SQLite
// ============================================================================

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

// BE-7 FIX: License rate limiting to prevent brute force attacks
// Limits to 10 attempts per minute per IP/device
var (
	licenseAttempts     = make(map[string][]time.Time) // deviceHash -> timestamps
	licenseAttemptsMu   sync.Mutex
	maxAttemptsPerMin   = 10
	rateLimitWindowSecs = 60
)

// LicenseKey represents a license key in the database
type LicenseKey struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	Key         string     `gorm:"uniqueIndex;size:20" json:"key"`       // PH-ADM-A1B2C3
	Role        string     `gorm:"size:20" json:"role"`                  // admin, manager, sales, operations
	DisplayName string     `gorm:"size:100" json:"display_name"`         // Employee name (e.g. "Rishu", "Stanley")
	DeviceHash  string     `gorm:"size:64;index" json:"device_hash"`     // Bound to this PC
	Activated   bool       `gorm:"default:false" json:"activated"`
	ActivatedAt *time.Time `json:"activated_at"`
	IssuedAt    time.Time  `gorm:"autoCreateTime" json:"issued_at"`
	ExpiresAt   *time.Time `json:"expires_at"`                           // Optional expiry
	Notes       string     `gorm:"size:255" json:"notes"`                // Admin notes
	CreatedBy   string     `gorm:"size:100" json:"created_by"`           // Who generated this key
}

// LicenseActivationResult is returned after license activation
type LicenseActivationResult struct {
	Success     bool     `json:"success"`
	Message     string   `json:"message"`
	Role        string   `json:"role"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
	DeviceHash  string   `json:"device_hash"`
}

// LicenseValidationResult is returned when checking current license
type LicenseValidationResult struct {
	Valid       bool     `json:"valid"`
	Role        string   `json:"role"`
	Key         string   `json:"key"`
	DisplayName string   `json:"display_name"`
	Permissions []string `json:"permissions"`
	ExpiresAt   *string  `json:"expires_at"`
}

// Role prefix mapping
var rolePrefixes = map[string]string{
	"admin":      "ADM",
	"manager":    "MGR",
	"sales":      "SLS",
	"operations": "OPS",
	"developer":  "DEV", // Developer/test keys with full admin access
}

// Reverse mapping: prefix -> role
var prefixToRole = map[string]string{
	"ADM": "admin",
	"MGR": "manager",
	"SLS": "sales",
	"OPS": "operations",
	"DEV": "admin", // DEV keys get admin permissions
}

// Role permissions mapping
var rolePermissions = map[string][]string{
	"admin":     {"*"}, // Full access
	"developer": {"*"}, // Full access for testing (DEV keys)
	"manager": {
		"dashboard:view",
		"offers:view", "offers:create", "offers:edit",
		"orders:view", "orders:create", "orders:edit", "orders:update",
		"finance:view", "finance:create", "finance:delete", // Bank reconciliation needs finance:delete
		"invoices:view", "invoices:create", "invoices:update", // Invoice management
		"payments:view", "payments:create", "payments:update", // Payment management (NO payments:delete - admin only)
		"customers:view", "customers:edit",
		"suppliers:view", "suppliers:edit",
		"po:view", "po:create", // Operations access for POs and supplier invoices
		"documents:create", "documents:view", // OCR features
		"intelligence:chat", "intelligence:reports",
		"users:view",
	},
	"sales": {
		"dashboard:view",
		"offers:view", "offers:create", "offers:edit",
		"orders:view", "orders:create", "orders:update",
		"invoices:view", // Can VIEW invoices but NOT create (manager/admin only)
		"payments:view", // Can VIEW payments but NOT create (manager/admin only)
		"customers:view", "customers:edit",
		"documents:create", "documents:view", // OCR features for scanning customer docs
		"intelligence:chat",
	},
	"operations": {
		"dashboard:view",
		"orders:view", "orders:update",
		"invoices:view", // Can VIEW invoices for procurement tracking
		"suppliers:view", "suppliers:edit",
		"grn:view", "grn:create",
		"po:view", "po:create", "po:edit",
		"documents:create", "documents:view", // OCR features for scanning supplier docs
		"intelligence:chat",
	},
}

// GenerateLicenseKey creates a new license key for a role (admin only)
func (a *App) GenerateLicenseKey(role, notes, createdBy string) (string, error) {
	if err := a.requirePermission("licenses:manage"); err != nil {
		return "", err
	}
	role = strings.ToLower(role)
	prefix, ok := rolePrefixes[role]
	if !ok {
		return "", fmt.Errorf("invalid role: %s (valid: admin, manager, sales, operations)", role)
	}

	// Generate unique hash from timestamp + random
	timestamp := time.Now().UnixNano()
	hashInput := fmt.Sprintf("%d-%s-%d", timestamp, role, time.Now().Nanosecond())
	hash := sha256.Sum256([]byte(hashInput))
	hashStr := strings.ToUpper(hex.EncodeToString(hash[:])[:6])

	key := fmt.Sprintf("PH-%s-%s", prefix, hashStr)

	// Store in database
	license := LicenseKey{
		Key:       key,
		Role:      role,
		Activated: false,
		Notes:     notes,
		CreatedBy: createdBy,
	}

	if err := a.db.Create(&license).Error; err != nil {
		return "", fmt.Errorf("failed to store license key: %w", err)
	}

	log.Printf("Generated license key: %s (role: %s)", key, role)
	return key, nil
}

// GenerateBatchLicenseKeys creates multiple license keys at once (admin only)
func (a *App) GenerateBatchLicenseKeys(role string, count int, notes, createdBy string) ([]string, error) {
	if err := a.requirePermission("licenses:manage"); err != nil {
		return nil, err
	}
	if count < 1 || count > 100 {
		return nil, fmt.Errorf("count must be between 1 and 100")
	}

	keys := make([]string, 0, count)
	for i := 0; i < count; i++ {
		key, err := a.GenerateLicenseKey(role, notes, createdBy)
		if err != nil {
			return keys, fmt.Errorf("failed at key %d: %w", i+1, err)
		}
		keys = append(keys, key)
		// Small delay to ensure unique timestamps
		time.Sleep(time.Millisecond)
	}

	return keys, nil
}

// ActivateLicense activates a license key and binds it to this PC
func (a *App) ActivateLicense(key string) (LicenseActivationResult, error) {
	key = strings.ToUpper(strings.TrimSpace(key))

	// BE-7 FIX: Rate limiting - prevent brute force attacks on license keys
	deviceHash := a.getDeviceHash()
	licenseAttemptsMu.Lock()
	now := time.Now()
	windowStart := now.Add(-time.Duration(rateLimitWindowSecs) * time.Second)

	// Clean old attempts outside window
	attempts := licenseAttempts[deviceHash]
	validAttempts := make([]time.Time, 0, len(attempts))
	for _, t := range attempts {
		if t.After(windowStart) {
			validAttempts = append(validAttempts, t)
		}
	}

	// Check if rate limited
	if len(validAttempts) >= maxAttemptsPerMin {
		licenseAttemptsMu.Unlock()
		log.Printf("🚫 License activation rate limited for device: %s (%d attempts in last minute)", deviceHash[:16]+"...", len(validAttempts))
		return LicenseActivationResult{
			Success: false,
			Message: "Too many activation attempts. Please wait 1 minute before trying again.",
		}, nil
	}

	// Record this attempt
	licenseAttempts[deviceHash] = append(validAttempts, now)
	licenseAttemptsMu.Unlock()

	// Validate key format: PH-XXX-YYYYYY
	if !strings.HasPrefix(key, "PH-") || len(key) != 13 {
		return LicenseActivationResult{
			Success: false,
			Message: "Invalid key format. Keys look like: PH-ADM-A1B2C3",
		}, nil
	}

	// Check if this device already has an active license
	var existingLicense LicenseKey
	if err := a.db.Where("device_hash = ? AND activated = 1", deviceHash).First(&existingLicense).Error; err == nil {
		return LicenseActivationResult{
			Success:     true,
			Message:     "This device is already licensed",
			Role:        existingLicense.Role,
			DisplayName: existingLicense.DisplayName,
			Permissions: rolePermissions[existingLicense.Role],
			DeviceHash:  deviceHash,
		}, nil
	}

	// Use transaction to prevent race condition on concurrent activation
	tx := a.db.Begin()
	if tx.Error != nil {
		return LicenseActivationResult{Success: false, Message: "Database error"}, tx.Error
	}

	// Find the license key - SECURITY: Keys must be pre-generated by admin
	// Auto-creation REMOVED to prevent bypass attacks (P0 security fix)
	var license LicenseKey
	if err := tx.Where("key = ?", key).First(&license).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			log.Printf("🔒 License key not found: %s (auto-creation disabled for security)", key)
			return LicenseActivationResult{
				Success: false,
				Message: "Invalid license key. Please contact your administrator for a valid key.",
			}, nil
		}
		return LicenseActivationResult{Success: false, Message: "Database error"}, err
	}

	// Check if already activated on another device
	if license.Activated && license.DeviceHash != deviceHash {
		tx.Rollback()
		return LicenseActivationResult{
			Success: false,
			Message: "This license key is already activated on another device.",
		}, nil
	}

	// Check expiry
	if license.ExpiresAt != nil && license.ExpiresAt.Before(time.Now()) {
		tx.Rollback()
		return LicenseActivationResult{
			Success: false,
			Message: "This license key has expired.",
		}, nil
	}

	// Activate the license within transaction
	now = time.Now()
	license.Activated = true
	license.ActivatedAt = &now
	license.DeviceHash = deviceHash

	if err := tx.Save(&license).Error; err != nil {
		tx.Rollback()
		return LicenseActivationResult{Success: false, Message: "Failed to activate license"}, err
	}

	if err := tx.Commit().Error; err != nil {
		return LicenseActivationResult{Success: false, Message: "Failed to activate license"}, err
	}

	log.Printf("License activated: %s (role: %s) on device: %s", key, license.Role, deviceHash[:16]+"...")

	// NOTE: First-run sync is handled by the frontend (LicenseActivationScreen.svelte)
	// with proper UI feedback. Do NOT call it here to avoid race conditions
	// and duplicate sync attempts that could cause data loss.

	return LicenseActivationResult{
		Success:     true,
		Message:     fmt.Sprintf("License activated successfully! Role: %s", strings.Title(license.Role)),
		Role:        license.Role,
		DisplayName: license.DisplayName,
		Permissions: rolePermissions[license.Role],
		DeviceHash:  deviceHash,
	}, nil
}

// ValidateLicense checks if this device has a valid license
func (a *App) ValidateLicense() (LicenseValidationResult, error) {
	log.Println("🔑 ValidateLicense: Starting...")

	// Check database is initialized
	if a.db == nil {
		log.Println("🔑 ValidateLicense: ERROR - Database not initialized!")
		return LicenseValidationResult{Valid: false}, fmt.Errorf("database not initialized")
	}

	log.Println("🔑 ValidateLicense: Getting device hash...")
	deviceHash := a.getDeviceHash()
	log.Printf("🔑 ValidateLicense: Device hash obtained (%s...)", deviceHash[:16])

	log.Println("🔑 ValidateLicense: Querying database for license...")
	var license LicenseKey
	// Use integer 1 for activated to ensure SQLite compatibility
	if err := a.db.Where("device_hash = ? AND activated = 1", deviceHash).First(&license).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("🔑 ValidateLicense: No active license found for device hash: %s", deviceHash[:16])
			return LicenseValidationResult{Valid: false}, nil
		}
		log.Printf("🔑 ValidateLicense: Database error: %v", err)
		return LicenseValidationResult{Valid: false}, err
	}
	log.Printf("🔑 ValidateLicense: Found license: %s (role: %s)", license.Key, license.Role)

	// Check expiry
	if license.ExpiresAt != nil && license.ExpiresAt.Before(time.Now()) {
		return LicenseValidationResult{
			Valid:   false,
			Role:    license.Role,
			Key:     license.Key,
		}, nil
	}

	var expiresAt *string
	if license.ExpiresAt != nil {
		exp := license.ExpiresAt.Format("2006-01-02")
		expiresAt = &exp
	}

	return LicenseValidationResult{
		Valid:       true,
		Role:        license.Role,
		Key:         license.Key,
		DisplayName: license.DisplayName,
		Permissions: rolePermissions[license.Role],
		ExpiresAt:   expiresAt,
	}, nil
}

// GetLicenseRole returns the current device's license role (for Butler AI restrictions)
func (a *App) GetLicenseRole() string {
	result, err := a.ValidateLicense()
	if err != nil || !result.Valid {
		return "" // No valid license
	}
	return result.Role
}

// HasLicensePermission checks if the current license has a specific permission
func (a *App) HasLicensePermission(permission string) bool {
	result, err := a.ValidateLicense()
	if err != nil || !result.Valid {
		return false
	}

	for _, perm := range result.Permissions {
		if perm == "*" || perm == permission {
			return true
		}
		// Check wildcard match (e.g., "finance:*" matches "finance:view")
		if strings.HasSuffix(perm, ":*") {
			prefix := strings.TrimSuffix(perm, ":*")
			if strings.HasPrefix(permission, prefix+":") {
				return true
			}
		}
	}
	return false
}

// ListLicenseKeys returns all license keys (admin only)
func (a *App) ListLicenseKeys() ([]LicenseKey, error) {
	if err := a.requirePermission("licenses:manage"); err != nil {
		return nil, err
	}
	var licenses []LicenseKey
	if err := a.db.Order("issued_at DESC").Find(&licenses).Error; err != nil {
		return nil, err
	}
	return licenses, nil
}

// RevokeLicense deactivates a license key (admin only)
func (a *App) RevokeLicense(key string) error {
	if err := a.requirePermission("licenses:manage"); err != nil {
		return err
	}
	result := a.db.Model(&LicenseKey{}).Where("key = ?", key).Updates(map[string]interface{}{
		"activated":    false,
		"device_hash":  "",
		"activated_at": nil,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("license key not found: %s", key)
	}
	log.Printf("License revoked: %s", key)
	return nil
}

// getDeviceHash generates a unique hash for this PC (reuses GetMachineID from device_service.go)
func (a *App) getDeviceHash() string {
	log.Println("🔐 getDeviceHash: Calling GetMachineID...")
	hash := GetMachineID()
	log.Printf("🔐 getDeviceHash: Complete, hash=%s...", hash[:16])
	return hash
}

// EnsureLicenseTableExists creates the license_keys table if it doesn't exist
func (a *App) EnsureLicenseTableExists() error {
	return a.db.AutoMigrate(&LicenseKey{})
}

// SeedLicenseKeys generates 10 keys per role if fewer than 10 unactivated keys exist per role.
// Called on startup to ensure the org always has enough keys available.
func (a *App) SeedLicenseKeys() error {
	log.Println("🔑 SeedLicenseKeys: Starting...")

	roles := []string{"admin", "manager", "sales", "operations", "developer"}
	targetPerRole := 10
	var totalSeeded int

	for _, role := range roles {
		prefix, ok := rolePrefixes[role]
		if !ok {
			log.Printf("🔑 SeedLicenseKeys: No prefix for role '%s', skipping", role)
			continue
		}

		// Count existing unactivated keys for this role
		var existingCount int64
		if err := a.db.Model(&LicenseKey{}).Where("role = ? AND activated = 0", role).Count(&existingCount).Error; err != nil {
			log.Printf("⚠️ SeedLicenseKeys: Failed to count keys for role '%s': %v", role, err)
			continue
		}

		needed := targetPerRole - int(existingCount)
		log.Printf("🔑 SeedLicenseKeys: role=%s prefix=%s existing=%d needed=%d", role, prefix, existingCount, needed)
		if needed <= 0 {
			continue
		}

		for i := 0; i < needed; i++ {
			// Generate deterministic but unique key using role + index + fixed seed
			hashInput := fmt.Sprintf("ph-trading-2026-%s-%d-%d", role, i, existingCount)
			hash := sha256.Sum256([]byte(hashInput))
			hashStr := strings.ToUpper(hex.EncodeToString(hash[:])[:6])
			key := fmt.Sprintf("PH-%s-%s", prefix, hashStr)

			// Check for collision
			var collision int64
			a.db.Model(&LicenseKey{}).Where("key = ?", key).Count(&collision)
			if collision > 0 {
				// Use timestamp-based fallback for uniqueness
				hashInput = fmt.Sprintf("%d-%s-%d", time.Now().UnixNano(), role, i)
				hash = sha256.Sum256([]byte(hashInput))
				hashStr = strings.ToUpper(hex.EncodeToString(hash[:])[:6])
				key = fmt.Sprintf("PH-%s-%s", prefix, hashStr)
				time.Sleep(time.Millisecond)
			}

			license := LicenseKey{
				Key:       key,
				Role:      role,
				Activated: false,
				Notes:     "Pre-generated for PH Trading org",
				CreatedBy: "system",
			}

			if err := a.db.Create(&license).Error; err != nil {
				log.Printf("⚠️ Failed to seed key %s: %v", key, err)
				continue
			}
			log.Printf("🔑 Created key: %s (role: %s)", key, role)
			totalSeeded++
		}
	}

	if totalSeeded > 0 {
		log.Printf("🔑 Seeded %d new license keys", totalSeeded)
	} else {
		log.Println("🔑 SeedLicenseKeys: No new keys needed (all roles have 10+ unactivated keys)")
	}

	// ALWAYS print all available (unactivated) keys prominently
	var availableKeys []LicenseKey
	a.db.Where("activated = 0").Order("role, key").Find(&availableKeys)
	log.Println("")
	log.Println("╔══════════════════════════════════════════════════════════╗")
	log.Println("║              AVAILABLE LICENSE KEYS                     ║")
	log.Println("╠══════════════════════════════════════════════════════════╣")
	if len(availableKeys) > 0 {
		log.Printf("║  %-11s | %-13s | %-20s   ║", "ROLE", "KEY", "NOTES")
		log.Println("║  ------------|---------------|----------------------  ║")
		for _, k := range availableKeys {
			log.Printf("║  %-11s | %s | %-20s   ║", k.Role, k.Key, k.Notes)
		}
	} else {
		log.Println("║  No unactivated keys available!                        ║")
		log.Println("║  All keys may already be activated.                    ║")
	}
	log.Println("╚══════════════════════════════════════════════════════════╝")
	log.Println("")

	return nil
}

// SeedEmployeeKeys creates specific license keys linked to named employees.
// These are the primary keys given to PH Trading staff.
// Called on startup after SeedLicenseKeys.
func (a *App) SeedEmployeeKeys() error {
	log.Println("🔑 SeedEmployeeKeys: Assigning keys to employees...")

	// Employee key assignments: role, display_name
	type employeeKey struct {
		Role        string
		DisplayName string
	}

	employees := []employeeKey{
		// Admin keys
		{Role: "admin", DisplayName: "Rishu"},
		{Role: "admin", DisplayName: "Stanley"},
		{Role: "admin", DisplayName: "Daniel"},
		// Manager keys
		{Role: "manager", DisplayName: "Sundar"},
		// Sales keys
		{Role: "sales", DisplayName: "Peri"},
		{Role: "sales", DisplayName: "Ebin"},
		{Role: "sales", DisplayName: "Support"},
		// Operations keys
		{Role: "operations", DisplayName: "Clinton"},
	}

	var assigned int
	for _, emp := range employees {
		// Check if this employee already has a key assigned
		var existing LicenseKey
		if err := a.db.Where("display_name = ?", emp.DisplayName).First(&existing).Error; err == nil {
			// Already assigned
			continue
		}

		// Find an unactivated, unnamed key for this role
		var availableKey LicenseKey
		if err := a.db.Where("role = ? AND activated = 0 AND (display_name = '' OR display_name IS NULL)", emp.Role).
			First(&availableKey).Error; err != nil {
			log.Printf("⚠️ No available %s key for %s - generating new one", emp.Role, emp.DisplayName)
			// Generate a new key for this employee
			prefix := rolePrefixes[emp.Role]
			hashInput := fmt.Sprintf("ph-employee-%s-%s-2026", emp.Role, emp.DisplayName)
			hash := sha256.Sum256([]byte(hashInput))
			hashStr := strings.ToUpper(hex.EncodeToString(hash[:])[:6])
			key := fmt.Sprintf("PH-%s-%s", prefix, hashStr)

			newKey := LicenseKey{
				Key:         key,
				Role:        emp.Role,
				DisplayName: emp.DisplayName,
				Activated:   false,
				Notes:       fmt.Sprintf("Assigned to %s", emp.DisplayName),
				CreatedBy:   "system",
			}
			if err := a.db.Create(&newKey).Error; err != nil {
				log.Printf("⚠️ Failed to create key for %s: %v", emp.DisplayName, err)
				continue
			}
			log.Printf("🔑 Created & assigned key %s to %s (%s)", key, emp.DisplayName, emp.Role)
			assigned++
			continue
		}

		// Assign the existing key to this employee
		availableKey.DisplayName = emp.DisplayName
		availableKey.Notes = fmt.Sprintf("Assigned to %s", emp.DisplayName)
		if err := a.db.Save(&availableKey).Error; err != nil {
			log.Printf("⚠️ Failed to assign key to %s: %v", emp.DisplayName, err)
			continue
		}
		log.Printf("🔑 Assigned key %s to %s (%s)", availableKey.Key, emp.DisplayName, emp.Role)
		assigned++
	}

	if assigned > 0 {
		log.Printf("🔑 Assigned %d employee keys", assigned)
	}

	// Print employee key assignments
	var employeeKeys []LicenseKey
	a.db.Where("display_name != '' AND display_name IS NOT NULL").Order("role, display_name").Find(&employeeKeys)
	if len(employeeKeys) > 0 {
		log.Println("")
		log.Println("╔══════════════════════════════════════════════════════════╗")
		log.Println("║              EMPLOYEE LICENSE KEYS                      ║")
		log.Println("╠══════════════════════════════════════════════════════════╣")
		log.Printf("║  %-11s | %-13s | %-20s   ║", "EMPLOYEE", "KEY", "ROLE")
		log.Println("║  ------------|---------------|----------------------  ║")
		for _, k := range employeeKeys {
			status := "Available"
			if k.Activated {
				status = "ACTIVATED"
			}
			log.Printf("║  %-11s | %s | %-11s %-8s   ║", k.DisplayName, k.Key, k.Role, status)
		}
		log.Println("╚══════════════════════════════════════════════════════════╝")
		log.Println("")
	}

	return nil
}

// CheckFirstInstall returns true if no licenses exist (first installation)
func (a *App) CheckFirstInstall() bool {
	var count int64
	a.db.Model(&LicenseKey{}).Where("activated = 1").Count(&count)
	return count == 0
}

// NeedsLicenseActivation checks if this device needs to activate a license
func (a *App) NeedsLicenseActivation() (bool, error) {
	result, err := a.ValidateLicense()
	if err != nil {
		return true, err
	}
	return !result.Valid, nil
}
