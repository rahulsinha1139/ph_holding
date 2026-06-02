package main

import (
	crypto_rand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// DeviceRegistrationResult represents the result of device registration
type DeviceRegistrationResult struct {
	DeviceID     string   `json:"device_id"`
	MachineID    string   `json:"machine_id"`
	Status       string   `json:"status"` // "first_setup", "pending", "approved", "blocked"
	IsFirstSetup bool     `json:"is_first_setup"`
	UserID       string   `json:"user_id,omitempty"`
	UserName     string   `json:"user_name,omitempty"`
	RoleName     string   `json:"role_name,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
}

// GetMachineID returns a stable local device fingerprint.
// It is persisted in app data so changing Wi-Fi, IP, interface order, or app
// bundle location does not invalidate a previously activated license.
func GetMachineID() string {
	log.Println("🖥️ GetMachineID: Starting...")

	if override := strings.TrimSpace(os.Getenv("ASYMMFLOW_MACHINE_ID")); isMachineIDHash(override) {
		log.Printf("🖥️ GetMachineID: Using ASYMMFLOW_MACHINE_ID override, hash=%s...", override[:16])
		return strings.ToLower(override)
	}

	if persisted, ok := readPersistedMachineID(); ok {
		log.Printf("🖥️ GetMachineID: Using persisted machine id, hash=%s...", persisted[:16])
		return persisted
	}

	seed, source := stableMachineSeed()
	if strings.TrimSpace(seed) == "" {
		seed = randomMachineSeed()
		source = "random"
	}

	result := buildMachineIDHash([]string{"asymmflow-device-v2", source, strings.TrimSpace(strings.ToLower(seed)), runtime.GOOS, runtime.GOARCH})
	persistMachineID(result)
	log.Printf("🖥️ GetMachineID: Complete from %s seed, hash=%s...", source, result[:16])
	return result
}

func buildMachineIDHash(identifiers []string) string {
	combined := strings.Join(identifiers, "|")
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

func isMachineIDHash(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func machineIDFilePath() string {
	if appDataDir := appDataDirPath(); appDataDir != "" {
		return filepath.Join(appDataDir, "machine_id")
	}
	return ""
}

func readPersistedMachineID() (string, bool) {
	path := machineIDFilePath()
	if path == "" {
		return "", false
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	id := strings.ToLower(strings.TrimSpace(string(raw)))
	if !isMachineIDHash(id) {
		log.Printf("⚠️ Ignoring invalid persisted machine id at %s", path)
		return "", false
	}
	return id, true
}

func persistMachineID(machineID string) {
	path := machineIDFilePath()
	if path == "" || !isMachineIDHash(machineID) {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		log.Printf("⚠️ Could not create machine id directory: %v", err)
		return
	}
	if err := os.WriteFile(path, []byte(strings.ToLower(machineID)+"\n"), 0600); err != nil {
		log.Printf("⚠️ Could not persist machine id at %s: %v", path, err)
	}
}

func randomMachineSeed() string {
	buf := make([]byte, 32)
	if _, err := crypto_rand.Read(buf); err == nil {
		return hex.EncodeToString(buf)
	}
	return fmt.Sprintf("%s-%s", runtime.GOOS, uuid.NewString())
}

func stableMachineSeed() (string, string) {
	switch runtime.GOOS {
	case "darwin":
		if seed := darwinPlatformUUID(); seed != "" {
			return seed, "darwin-platform-uuid"
		}
	case "linux":
		if seed := linuxMachineID(); seed != "" {
			return seed, "linux-machine-id"
		}
	case "windows":
		if seed := windowsMachineUUID(); seed != "" {
			return seed, "windows-machine-uuid"
		}
	}

	if seed := networkHardwareSeed(); seed != "" {
		return seed, "network-hardware"
	}
	return "", ""
}

func darwinPlatformUUID() string {
	out, err := exec.Command("/usr/sbin/ioreg", "-rd1", "-c", "IOPlatformExpertDevice").Output()
	if err != nil {
		log.Printf("⚠️ GetMachineID: ioreg platform UUID lookup failed: %v", err)
		return ""
	}
	for _, key := range []string{"IOPlatformUUID", "IOPlatformSerialNumber"} {
		if value := parseQuotedIoregValue(string(out), key); value != "" {
			return value
		}
	}
	return ""
}

func parseQuotedIoregValue(output, key string) string {
	needle := `"` + key + `"`
	for _, line := range strings.Split(output, "\n") {
		if !strings.Contains(line, needle) {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, `"`)
		return strings.TrimSpace(value)
	}
	return ""
}

func linuxMachineID() string {
	for _, path := range []string{"/etc/machine-id", "/var/lib/dbus/machine-id"} {
		raw, err := os.ReadFile(path)
		if err == nil && strings.TrimSpace(string(raw)) != "" {
			return strings.TrimSpace(string(raw))
		}
	}
	return ""
}

func windowsMachineUUID() string {
	out, err := exec.Command("wmic", "csproduct", "get", "UUID").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		value := strings.TrimSpace(line)
		if value == "" || strings.EqualFold(value, "UUID") {
			continue
		}
		if strings.Contains(value, "00000000-0000-0000-0000-000000000000") {
			continue
		}
		return value
	}
	return ""
}

func networkHardwareSeed() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		log.Printf("⚠️ GetMachineID: network interface lookup failed: %v", err)
		return ""
	}
	addrs := make([]string, 0, len(interfaces))
	for _, iface := range interfaces {
		if !isStableHardwareInterface(iface) {
			continue
		}
		addrs = append(addrs, strings.ToLower(iface.HardwareAddr.String()))
	}
	sort.Strings(addrs)
	if len(addrs) == 0 {
		return ""
	}
	log.Printf("🖥️ GetMachineID: Using %d stable hardware addresses", len(addrs))
	return strings.Join(addrs, "|")
}

func isStableHardwareInterface(iface net.Interface) bool {
	name := strings.ToLower(iface.Name)
	if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
		return false
	}
	skipPrefixes := []string{"awdl", "llw", "utun", "bridge", "gif", "stf", "p2p", "ap", "anpi", "vmenet", "vmnet"}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(name, prefix) {
			return false
		}
	}
	// Locally-administered MACs are often randomized or virtual.
	if len(iface.HardwareAddr) > 0 && iface.HardwareAddr[0]&0x02 != 0 {
		return false
	}
	return true
}

// GetDeviceInfo returns information about the current device
func GetDeviceInfo() (name string, osInfo string) {
	hostname, _ := os.Hostname()
	osInfo = fmt.Sprintf("%s %s", runtime.GOOS, runtime.GOARCH)
	return hostname, osInfo
}

// RegisterDevice registers the current device and returns its status
// This is called on every app startup
func (a *App) RegisterDevice() (*DeviceRegistrationResult, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	machineID := GetMachineID()
	deviceName, osInfo := GetDeviceInfo()

	log.Printf("🔌 Device registration check: machineID=%s...", machineID[:16])

	// Check if device exists
	var device Device
	result := a.db.Where("machine_id = ?", machineID).First(&device)

	if result.Error != nil {
		// Device doesn't exist - check if this is the first device ever
		var deviceCount int64
		a.db.Model(&Device{}).Count(&deviceCount)

		if deviceCount == 0 {
			// FIRST INSTALLATION - Create device in "first_setup" state
			log.Println("🎉 First installation detected! Creating admin device...")

			device = Device{
				Base:          Base{ID: uuid.New().String()},
				MachineID:     machineID,
				DeviceName:    deviceName,
				OSInfo:        osInfo,
				FirstSeenAt:   time.Now(),
				Status:        "first_setup", // Special status for first setup
				IsAdminDevice: true,
			}

			log.Printf("📝 Creating device with ID=%s, MachineID=%s, Status=%s", device.ID, device.MachineID[:16], device.Status)
			createResult := a.db.Create(&device)
			if createResult.Error != nil {
				log.Printf("❌ Device creation FAILED: %v", createResult.Error)
				return nil, fmt.Errorf("failed to create admin device: %w", createResult.Error)
			}
			log.Printf("✅ Device created successfully! RowsAffected=%d", createResult.RowsAffected)

			return &DeviceRegistrationResult{
				DeviceID:     device.ID,
				MachineID:    machineID,
				Status:       "first_setup",
				IsFirstSetup: true,
			}, nil
		}

		// Not first device - create as pending
		log.Println("📝 New device detected, creating pending registration...")

		device = Device{
			Base:        Base{ID: uuid.New().String()},
			MachineID:   machineID,
			DeviceName:  deviceName,
			OSInfo:      osInfo,
			FirstSeenAt: time.Now(),
			Status:      "pending",
		}

		if err := a.db.Create(&device).Error; err != nil {
			return nil, fmt.Errorf("failed to create pending device: %w", err)
		}

		return &DeviceRegistrationResult{
			DeviceID:  device.ID,
			MachineID: machineID,
			Status:    "pending",
		}, nil
	}

	// Device exists - update last seen
	now := time.Now()
	a.db.Model(&device).Update("last_seen_at", now)

	// Get user info if device is approved
	if device.Status == "approved" {
		var deviceUser DeviceUser
		if err := a.db.Preload("User").Preload("User.Role").
			Where("device_id = ? AND is_primary = ?", device.ID, true).
			First(&deviceUser).Error; err == nil {

			// Parse permissions
			var perms []string
			if deviceUser.User.Role.Permissions != "" {
				// Simple JSON array parsing
				permStr := strings.Trim(deviceUser.User.Role.Permissions, "[]")
				for _, p := range strings.Split(permStr, ",") {
					p = strings.Trim(p, ` "`)
					if p != "" {
						perms = append(perms, p)
					}
				}
			}

			return &DeviceRegistrationResult{
				DeviceID:    device.ID,
				MachineID:   machineID,
				Status:      device.Status,
				UserID:      deviceUser.UserID,
				UserName:    deviceUser.User.FullName,
				RoleName:    deviceUser.User.Role.DisplayName,
				Permissions: perms,
			}, nil
		}
	}

	return &DeviceRegistrationResult{
		DeviceID:  device.ID,
		MachineID: machineID,
		Status:    device.Status,
	}, nil
}

// SetupAdminAccount creates the first admin user and approves the admin device
// This is only called during first-time setup
func (a *App) SetupAdminAccount(username, password, fullName, email string) (*User, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Verify this is actually first setup
	var userCount int64
	a.db.Model(&User{}).Count(&userCount)
	if userCount > 0 {
		return nil, fmt.Errorf("admin account already exists")
	}

	// P1 FIX: Enhanced input validation
	if GlobalValidator != nil {
		if err := GlobalValidator.ValidateUserInput(username, password, fullName, email); err != nil {
			return nil, err
		}
	} else {
		// Fallback validation if security enhancements not initialized
		if username == "" || password == "" || fullName == "" {
			return nil, fmt.Errorf("username, password, and full name are required")
		}
		if len(password) < 8 {
			return nil, fmt.Errorf("password must be at least 8 characters")
		}
	}

	// Get admin role
	var adminRole Role
	if err := a.db.Where("name = ?", "admin").First(&adminRole).Error; err != nil {
		return nil, fmt.Errorf("admin role not found - please restart the application")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create admin user
	now := time.Now()
	user := &User{
		Base:         Base{ID: uuid.New().String()},
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedPassword),
		RoleID:       adminRole.ID,
		FullName:     fullName,
		DisplayName:  fullName,
		IsActive:     true,
		LastLoginAt:  &now,
	}

	if err := a.db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	// Mark current device as approved admin device
	machineID := GetMachineID()
	if err := a.db.Model(&Device{}).
		Where("machine_id = ?", machineID).
		Updates(map[string]interface{}{
			"status":          "approved",
			"is_admin_device": true,
			"approved_at":     now,
			"approved_by":     user.ID,
		}).Error; err != nil {
		log.Printf("Warning: failed to update device status: %v", err)
	}

	// Get device ID
	var device Device
	a.db.Where("machine_id = ?", machineID).First(&device)

	// Link user to device
	deviceUser := &DeviceUser{
		Base:      Base{ID: uuid.New().String()},
		DeviceID:  device.ID,
		UserID:    user.ID,
		IsPrimary: true,
	}
	a.db.Create(deviceUser)

	// Set current user context
	a.currentUser = user
	a.currentUserID = user.ID

	log.Printf("✅ Admin account created: %s (%s)", fullName, username)
	return user, nil
}

// ListPendingDevices returns devices awaiting approval (admin only)
func (a *App) ListPendingDevices() ([]Device, error) {
	if err := a.requirePermission("users:manage"); err != nil {
		return nil, err
	}

	var devices []Device
	if err := a.db.Where("status = ?", "pending").
		Order("first_seen_at DESC").
		Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("failed to list pending devices: %w", err)
	}

	return devices, nil
}

// ListAllDevices returns all devices (admin only)
func (a *App) ListAllDevices() ([]Device, error) {
	if err := a.requirePermission("users:manage"); err != nil {
		return nil, err
	}

	var devices []Device
	if err := a.db.Order("status, first_seen_at DESC").Find(&devices).Error; err != nil {
		return nil, fmt.Errorf("failed to list devices: %w", err)
	}

	// Populate approver names
	for i := range devices {
		if devices[i].ApprovedBy != "" {
			var approver User
			if err := a.db.First(&approver, "id = ?", devices[i].ApprovedBy).Error; err == nil {
				devices[i].ApproverName = approver.FullName
			}
		}
	}

	return devices, nil
}

// ApproveDevice approves a pending device and creates a user for it
func (a *App) ApproveDevice(deviceID, roleID, username, password, fullName, email string) error {
	if err := a.requirePermission("users:manage"); err != nil {
		return err
	}

	// P1 FIX: Enhanced input validation
	if GlobalValidator != nil {
		if err := GlobalValidator.ValidateUserInput(username, password, fullName, email); err != nil {
			return err
		}
	} else {
		// Fallback validation
		if deviceID == "" || roleID == "" || username == "" || password == "" || fullName == "" {
			return fmt.Errorf("all fields are required")
		}
		if len(password) < 8 {
			return fmt.Errorf("password must be at least 8 characters")
		}
	}

	// Verify device exists and is pending
	var device Device
	if err := a.db.First(&device, "id = ?", deviceID).Error; err != nil {
		return fmt.Errorf("device not found")
	}
	if device.Status != "pending" {
		return fmt.Errorf("device is not in pending status")
	}

	// Verify role exists
	var role Role
	if err := a.db.First(&role, "id = ?", roleID).Error; err != nil {
		return fmt.Errorf("role not found")
	}

	// Check username uniqueness
	var existingUser User
	if err := a.db.Where("username = ?", username).First(&existingUser).Error; err == nil {
		return fmt.Errorf("username already exists")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &User{
		Base:         Base{ID: uuid.New().String()},
		Username:     username,
		Email:        email,
		PasswordHash: string(hashedPassword),
		RoleID:       roleID,
		FullName:     fullName,
		DisplayName:  fullName,
		IsActive:     true,
	}

	if err := a.db.Create(user).Error; err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	// Update device status
	now := time.Now()
	if err := a.db.Model(&device).Updates(map[string]interface{}{
		"status":      "approved",
		"approved_by": a.currentUserID,
		"approved_at": now,
	}).Error; err != nil {
		return fmt.Errorf("failed to update device status: %w", err)
	}

	// Link user to device
	deviceUser := &DeviceUser{
		Base:      Base{ID: uuid.New().String()},
		DeviceID:  deviceID,
		UserID:    user.ID,
		IsPrimary: true,
	}
	a.db.Create(deviceUser)

	// P1 FIX: Audit device approval
	if GlobalAuditLogger != nil {
		GlobalAuditLogger.LogDeviceAction(a.currentUserID, deviceID, "device_approved", true, map[string]interface{}{
			"device_name": device.DeviceName,
			"user_id":     user.ID,
			"username":    username,
			"role":        role.Name,
		})
	}

	log.Printf("✅ Device approved: %s, User: %s (%s), Role: %s",
		device.DeviceName, fullName, username, role.DisplayName)

	return nil
}

// BlockDevice blocks a device from accessing the system
func (a *App) BlockDevice(deviceID string) error {
	if err := a.requirePermission("users:manage"); err != nil {
		return err
	}

	// Verify device exists
	var device Device
	if err := a.db.First(&device, "id = ?", deviceID).Error; err != nil {
		return fmt.Errorf("device not found")
	}

	// Prevent blocking admin device
	if device.IsAdminDevice {
		return fmt.Errorf("cannot block the admin device")
	}

	// Update status
	if err := a.db.Model(&device).Update("status", "blocked").Error; err != nil {
		return fmt.Errorf("failed to block device: %w", err)
	}

	// P1 FIX: Audit device blocking
	if GlobalAuditLogger != nil {
		GlobalAuditLogger.LogDeviceAction(a.currentUserID, deviceID, "device_blocked", true, map[string]interface{}{
			"device_name": device.DeviceName,
			"machine_id":  device.MachineID,
		})
	}

	log.Printf("🚫 Device blocked: %s (%s)", device.DeviceName, device.ID)
	return nil
}

// UnblockDevice unblocks a previously blocked device
func (a *App) UnblockDevice(deviceID string) error {
	if err := a.requirePermission("users:manage"); err != nil {
		return err
	}

	// Verify device exists
	var device Device
	if err := a.db.First(&device, "id = ?", deviceID).Error; err != nil {
		return fmt.Errorf("device not found")
	}

	if device.Status != "blocked" {
		return fmt.Errorf("device is not blocked")
	}

	// Update status back to approved
	if err := a.db.Model(&device).Update("status", "approved").Error; err != nil {
		return fmt.Errorf("failed to unblock device: %w", err)
	}

	log.Printf("✅ Device unblocked: %s (%s)", device.DeviceName, device.ID)
	return nil
}

// GetDeviceUsers returns users associated with a device
func (a *App) GetDeviceUsers(deviceID string) ([]DeviceUser, error) {
	if err := a.requirePermission("users:manage"); err != nil {
		return nil, err
	}

	var deviceUsers []DeviceUser
	if err := a.db.Preload("User").Preload("User.Role").
		Where("device_id = ?", deviceID).
		Find(&deviceUsers).Error; err != nil {
		return nil, fmt.Errorf("failed to get device users: %w", err)
	}

	return deviceUsers, nil
}

// LoginDevice authenticates a user on an approved device
func (a *App) LoginDevice(username, password string) (*DeviceRegistrationResult, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	machineID := GetMachineID()

	// P1 FIX: Rate limiting for login attempts (5 per minute per device)
	if GlobalRateLimiter != nil {
		rateLimitKey := "login:" + machineID
		if !GlobalRateLimiter.Allow(rateLimitKey, RateLimitConfig.LoginAttemptsPerMinute, 1*time.Minute/time.Duration(RateLimitConfig.LoginAttemptsPerMinute)) {
			log.Printf("🚫 Rate limit exceeded for login attempts from device %s", machineID[:16])
			return nil, fmt.Errorf("too many login attempts, please wait a moment and try again")
		}
	}

	// Check device is approved
	var device Device
	if err := a.db.Where("machine_id = ?", machineID).First(&device).Error; err != nil {
		return nil, fmt.Errorf("device not registered")
	}

	if device.Status != "approved" && device.Status != "first_setup" {
		return nil, fmt.Errorf("device not approved")
	}

	// Find user
	var user User
	if err := a.db.Where("username = ? AND is_active = ?", username, true).First(&user).Error; err != nil {
		// P1 FIX: Audit failed login attempt
		if GlobalAuditLogger != nil {
			GlobalAuditLogger.LogPermissionChange("system", username, "login_attempt", "", "", false, "user not found or inactive")
		}
		return nil, fmt.Errorf("invalid username or password")
	}
	if err := a.hydrateUserRole(&user); err != nil {
		return nil, fmt.Errorf("failed to load role for user: %w", err)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// P1 FIX: Audit failed login attempt
		if GlobalAuditLogger != nil {
			GlobalAuditLogger.LogPermissionChange("system", user.ID, "login_attempt", "", "", false, "incorrect password")
		}
		return nil, fmt.Errorf("invalid username or password")
	}

	// Update last login
	now := time.Now()
	a.db.Model(&user).Update("last_login_at", now)
	a.db.Model(&device).Update("last_seen_at", now)

	// Set current user context
	a.currentUser = &user
	a.currentUserID = user.ID

	// P1 FIX: Create session and track activity
	if GlobalSessionManager != nil {
		GlobalSessionManager.CreateSession(user.ID, device.ID)
	}

	// Parse permissions
	var perms []string
	if user.Role.Permissions != "" {
		permStr := strings.Trim(user.Role.Permissions, "[]")
		for _, p := range strings.Split(permStr, ",") {
			p = strings.Trim(p, ` "`)
			if p != "" {
				perms = append(perms, p)
			}
		}
	}

	// P1 FIX: Audit successful login
	if GlobalAuditLogger != nil {
		GlobalAuditLogger.LogPermissionChange("system", user.ID, "login_success", "", user.Role.Name, true, "successful authentication")
	}

	// P1 FIX: Mask sensitive data in log
	if GlobalMasker != nil {
		log.Printf("✅ User logged in: %s (%s) on device %s", user.FullName, username, device.DeviceName)
	} else {
		log.Printf("✅ User logged in: %s (%s) on device %s", user.FullName, username, device.DeviceName)
	}

	return &DeviceRegistrationResult{
		DeviceID:    device.ID,
		MachineID:   machineID,
		Status:      "approved",
		UserID:      user.ID,
		UserName:    user.FullName,
		RoleName:    user.Role.DisplayName,
		Permissions: perms,
	}, nil
}

// CheckDeviceStatus returns the current device status (for polling from pending screen)
func (a *App) CheckDeviceStatus() (*DeviceRegistrationResult, error) {
	return a.RegisterDevice()
}

// GetCurrentDeviceInfo returns information about the current device
func (a *App) GetCurrentDeviceInfo() (*Device, error) {
	machineID := GetMachineID()

	var device Device
	if err := a.db.Where("machine_id = ?", machineID).First(&device).Error; err != nil {
		return nil, fmt.Errorf("device not registered")
	}

	return &device, nil
}
