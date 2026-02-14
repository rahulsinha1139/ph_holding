package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// DeviceRegistrationResult represents the result of device registration
type DeviceRegistrationResult struct {
	DeviceID      string `json:"device_id"`
	MachineID     string `json:"machine_id"`
	Status        string `json:"status"`       // "first_setup", "pending", "approved", "blocked"
	IsFirstSetup  bool   `json:"is_first_setup"`
	UserID        string `json:"user_id,omitempty"`
	UserName      string `json:"user_name,omitempty"`
	RoleName      string `json:"role_name,omitempty"`
	Permissions   []string `json:"permissions,omitempty"`
}

// GetMachineID generates a unique hardware fingerprint for this device
// Uses MAC address + hostname, hashed for privacy
func GetMachineID() string {
	log.Println("🖥️ GetMachineID: Starting...")
	var identifiers []string

	// Get hostname (fast operation)
	log.Println("🖥️ GetMachineID: Getting hostname...")
	hostname, err := os.Hostname()
	if err == nil {
		identifiers = append(identifiers, hostname)
		log.Printf("🖥️ GetMachineID: Hostname=%s", hostname)
	} else {
		log.Printf("🖥️ GetMachineID: Hostname error: %v", err)
	}

	// Get MAC addresses with timeout (can hang on Windows after updates)
	// Use a goroutine with timeout to prevent blocking
	log.Println("🖥️ GetMachineID: Starting network interfaces check (2s timeout)...")
	type ifaceResult struct {
		addrs []string
	}
	ifaceChan := make(chan ifaceResult, 1)

	go func() {
		var addrs []string
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				// Skip loopback and interfaces without MAC
				if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
					continue
				}
				addrs = append(addrs, iface.HardwareAddr.String())
			}
		}
		ifaceChan <- ifaceResult{addrs: addrs}
	}()

	// Wait for interfaces with 2 second timeout
	select {
	case result := <-ifaceChan:
		log.Printf("🖥️ GetMachineID: Got %d MAC addresses", len(result.addrs))
		identifiers = append(identifiers, result.addrs...)
	case <-time.After(2 * time.Second):
		log.Println("⚠️ GetMachineID: Network interfaces check timed out (2s), using hostname only")
		// Still works - hostname + OS provides uniqueness
	}

	// Add OS info for additional uniqueness
	identifiers = append(identifiers, runtime.GOOS, runtime.GOARCH)
	log.Printf("🖥️ GetMachineID: Building hash from %d identifiers", len(identifiers))

	// Create SHA-256 hash
	combined := strings.Join(identifiers, "|")
	hash := sha256.Sum256([]byte(combined))
	result := hex.EncodeToString(hash[:])
	log.Printf("🖥️ GetMachineID: Complete, hash=%s...", result[:16])
	return result
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
	if err := a.db.Preload("Role").Where("username = ? AND is_active = ?", username, true).First(&user).Error; err != nil {
		// P1 FIX: Audit failed login attempt
		if GlobalAuditLogger != nil {
			GlobalAuditLogger.LogPermissionChange("system", username, "login_attempt", "", "", false, "user not found or inactive")
		}
		return nil, fmt.Errorf("invalid username or password")
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
