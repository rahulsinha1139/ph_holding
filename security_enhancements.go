package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/mail"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// =============================================================================
// P1 SECURITY ENHANCEMENTS
// =============================================================================
// This file implements 5 critical security improvements:
// 1. Input Validation (length limits, format validation, sanitization)
// 2. Sensitive Data Masking (logs, payments, contacts)
// 3. Session Security (timeout, activity tracking)
// 4. Audit Logging (financial transactions, permission changes)
// 5. Rate Limiting (login attempts, API calls, reports)
// =============================================================================

// =============================================================================
// 1. INPUT VALIDATION
// =============================================================================

// ValidationLimits defines maximum lengths for various input fields
var ValidationLimits = struct {
	CustomerName       int
	SupplierName       int
	Email              int
	Phone              int
	Address            int
	Notes              int
	Reference          int
	InvoiceNumber      int
	Description        int
	Filename           int
	ShortCode          int
	Username           int
	FullName           int
	Password           int
	ReportName         int
}{
	CustomerName:  200,
	SupplierName:  255,
	Email:         255,
	Phone:         50,
	Address:       500,
	Notes:         5000,
	Reference:     100,
	InvoiceNumber: 50,
	Description:   1000,
	Filename:      255,
	ShortCode:     10,
	Username:      50,
	FullName:      100,
	Password:      100,
	ReportName:    100,
}

// InputValidator provides validation methods for user inputs
type InputValidator struct{}

// NewInputValidator creates a new input validator
func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidateString validates string length and optionally checks for dangerous characters
func (v *InputValidator) ValidateString(field, value string, maxLength int, allowSpecialChars bool) error {
	// Check UTF-8 validity
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s contains invalid UTF-8 characters", field)
	}

	// Check length
	if len(value) > maxLength {
		return fmt.Errorf("%s exceeds maximum length of %d characters (got %d)", field, maxLength, len(value))
	}

	// Check for null bytes (security risk)
	if strings.Contains(value, "\x00") {
		return fmt.Errorf("%s contains null bytes", field)
	}

	// If special characters not allowed, check for common dangerous patterns
	if !allowSpecialChars {
		// Block common injection patterns
		dangerous := []string{"<script", "javascript:", "onerror=", "onload=", "../", "..\\"}
		lowerValue := strings.ToLower(value)
		for _, pattern := range dangerous {
			if strings.Contains(lowerValue, pattern) {
				return fmt.Errorf("%s contains potentially dangerous pattern: %s", field, pattern)
			}
		}
	}

	return nil
}

// ValidateEmail validates email format
func (v *InputValidator) ValidateEmail(email string) error {
	if email == "" {
		return nil // Empty email is allowed (optional field)
	}

	if len(email) > ValidationLimits.Email {
		return fmt.Errorf("email exceeds maximum length of %d characters", ValidationLimits.Email)
	}

	// Use standard library email parser
	_, err := mail.ParseAddress(email)
	if err != nil {
		return fmt.Errorf("invalid email format: %w", err)
	}

	return nil
}

// ValidatePhone validates phone number format (international format)
func (v *InputValidator) ValidatePhone(phone string) error {
	if phone == "" {
		return nil // Empty phone is allowed (optional field)
	}

	if len(phone) > ValidationLimits.Phone {
		return fmt.Errorf("phone number exceeds maximum length of %d characters", ValidationLimits.Phone)
	}

	// Allow only digits, spaces, +, -, (, )
	phoneRegex := regexp.MustCompile(`^[\d\s\+\-\(\)]+$`)
	if !phoneRegex.MatchString(phone) {
		return fmt.Errorf("phone number contains invalid characters (only digits, spaces, +, -, (, ) allowed)")
	}

	return nil
}

// SanitizeFilePath validates and sanitizes file paths to prevent path traversal
func (v *InputValidator) SanitizeFilePath(path string) (string, error) {
	// Check length
	if len(path) > ValidationLimits.Filename {
		return "", fmt.Errorf("file path exceeds maximum length of %d characters", ValidationLimits.Filename)
	}

	// Clean the path
	cleaned := filepath.Clean(path)

	// Prevent path traversal
	if strings.Contains(cleaned, "..") {
		return "", fmt.Errorf("path traversal detected in file path")
	}

	// Block absolute paths (Windows and Unix)
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("absolute paths not allowed")
	}

	return cleaned, nil
}

// ValidateCustomerInput validates customer master data inputs
func (v *InputValidator) ValidateCustomerInput(customer *CustomerMaster) error {
	if err := v.ValidateString("Business Name", customer.BusinessName, ValidationLimits.CustomerName, false); err != nil {
		return err
	}

	if err := v.ValidateString("Short Code", customer.ShortCode, ValidationLimits.ShortCode, false); err != nil {
		return err
	}

	if err := v.ValidateEmail(customer.TRN); err != nil {
		// TRN is not email, but using for general format check
		if len(customer.TRN) > 100 {
			return fmt.Errorf("TRN exceeds maximum length of 100 characters")
		}
	}

	return nil
}

// ValidateSupplierInput validates supplier master data inputs
func (v *InputValidator) ValidateSupplierInput(supplier *SupplierMaster) error {
	if err := v.ValidateString("Supplier Name", supplier.SupplierName, ValidationLimits.SupplierName, false); err != nil {
		return err
	}

	if err := v.ValidateEmail(supplier.Email); err != nil {
		return err
	}

	if err := v.ValidatePhone(supplier.Phone); err != nil {
		return err
	}

	if err := v.ValidateString("Address", supplier.Address, ValidationLimits.Address, true); err != nil {
		return err
	}

	return nil
}

// ValidateNoteInput validates entity note inputs
func (v *InputValidator) ValidateNoteInput(content string) error {
	return v.ValidateString("Note Content", content, ValidationLimits.Notes, true)
}

// ValidateUserInput validates user account inputs
func (v *InputValidator) ValidateUserInput(username, password, fullName, email string) error {
	if err := v.ValidateString("Username", username, ValidationLimits.Username, false); err != nil {
		return err
	}

	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	if len(password) > ValidationLimits.Password {
		return fmt.Errorf("password exceeds maximum length of %d characters", ValidationLimits.Password)
	}

	if err := v.ValidateString("Full Name", fullName, ValidationLimits.FullName, false); err != nil {
		return err
	}

	if err := v.ValidateEmail(email); err != nil {
		return err
	}

	return nil
}

// =============================================================================
// 2. SENSITIVE DATA MASKING
// =============================================================================

// DataMasker provides methods to mask sensitive data in logs
type DataMasker struct{}

// NewDataMasker creates a new data masker
func NewDataMasker() *DataMasker {
	return &DataMasker{}
}

// MaskPaymentAmount masks payment amounts for logging (show last 3 digits only for amounts > 100)
func (m *DataMasker) MaskPaymentAmount(amount float64) string {
	if amount < 100 {
		// Small amounts: show full value
		return fmt.Sprintf("%.3f BHD", amount)
	}
	// Large amounts: mask most digits, show last 3 only
	amountStr := fmt.Sprintf("%.3f", amount)
	if len(amountStr) > 7 {
		return "***" + amountStr[len(amountStr)-7:] + " BHD"
	}
	return "***." + amountStr[len(amountStr)-3:] + " BHD"
}

// MaskEmail masks email addresses (show first 2 chars + domain)
func (m *DataMasker) MaskEmail(email string) string {
	if email == "" {
		return ""
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return "***@***"
	}

	local := parts[0]
	domain := parts[1]

	if len(local) <= 2 {
		return "**@" + domain
	}

	return local[:2] + "***@" + domain
}

// MaskPhone masks phone numbers (show last 4 digits only)
func (m *DataMasker) MaskPhone(phone string) string {
	if phone == "" {
		return ""
	}

	// Remove non-digit characters
	digits := regexp.MustCompile(`\d+`).FindAllString(phone, -1)
	phoneDigits := strings.Join(digits, "")

	if len(phoneDigits) <= 4 {
		return "****"
	}

	return "***-***-" + phoneDigits[len(phoneDigits)-4:]
}

// MaskBankAccount masks bank account numbers (show last 4 digits)
func (m *DataMasker) MaskBankAccount(account string) string {
	if account == "" {
		return ""
	}

	if len(account) <= 4 {
		return "****"
	}

	return "************" + account[len(account)-4:]
}

// MaskIBAN masks IBAN (show country code + last 4 digits)
func (m *DataMasker) MaskIBAN(iban string) string {
	if iban == "" {
		return ""
	}

	if len(iban) <= 6 {
		return "**************"
	}

	return iban[:2] + "**************" + iban[len(iban)-4:]
}

// SanitizeForLog removes sensitive data from log fields
func (m *DataMasker) SanitizeForLog(fields map[string]interface{}) map[string]interface{} {
	sanitized := make(map[string]interface{})

	for k, v := range fields {
		lowerKey := strings.ToLower(k)

		// Mask known sensitive fields
		switch {
		case strings.Contains(lowerKey, "password"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "amount") && strings.Contains(lowerKey, "bhd"):
			if amount, ok := v.(float64); ok {
				sanitized[k] = m.MaskPaymentAmount(amount)
			} else {
				sanitized[k] = v
			}
		case strings.Contains(lowerKey, "email"):
			if email, ok := v.(string); ok {
				sanitized[k] = m.MaskEmail(email)
			} else {
				sanitized[k] = v
			}
		case strings.Contains(lowerKey, "phone"):
			if phone, ok := v.(string); ok {
				sanitized[k] = m.MaskPhone(phone)
			} else {
				sanitized[k] = v
			}
		case strings.Contains(lowerKey, "account") || strings.Contains(lowerKey, "iban"):
			if account, ok := v.(string); ok {
				sanitized[k] = m.MaskBankAccount(account)
			} else {
				sanitized[k] = v
			}
		case strings.Contains(lowerKey, "api_key") || strings.Contains(lowerKey, "apikey") || strings.Contains(lowerKey, "api-key"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "token"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "secret"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "authorization") || strings.Contains(lowerKey, "bearer"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "private_key") || strings.Contains(lowerKey, "privatekey"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "client_secret") || strings.Contains(lowerKey, "clientsecret"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "db_password") || strings.Contains(lowerKey, "database_password") || strings.Contains(lowerKey, "dbpassword"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "conn_string") || strings.Contains(lowerKey, "connection_string") || strings.Contains(lowerKey, "connstring"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "signing_key") || strings.Contains(lowerKey, "signingkey"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "encryption_key") || strings.Contains(lowerKey, "encryptionkey"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "access_key") || strings.Contains(lowerKey, "accesskey"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "supabase_key") || strings.Contains(lowerKey, "supabasekey"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "master_key") || strings.Contains(lowerKey, "masterkey"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "jwt_secret") || strings.Contains(lowerKey, "jwtsecret"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "webhook_secret") || strings.Contains(lowerKey, "webhooksecret"):
			sanitized[k] = "***REDACTED***"
		case strings.Contains(lowerKey, "aiml_key") || strings.Contains(lowerKey, "aimlkey"):
			sanitized[k] = "***REDACTED***"
		default:
			sanitized[k] = v
		}
	}

	return sanitized
}

// =============================================================================
// 3. SESSION SECURITY
// =============================================================================

// SessionConfig defines session security parameters
var SessionConfig = struct {
	InactivityTimeout time.Duration
	MaxSessionAge     time.Duration
	ActivityCheckInterval time.Duration
}{
	InactivityTimeout: 8 * time.Hour,      // 8 hours of inactivity
	MaxSessionAge:     24 * time.Hour,     // 24 hours maximum session age
	ActivityCheckInterval: 5 * time.Minute, // Check every 5 minutes
}

// SessionManager manages user session security
type SessionManager struct {
	sessions sync.Map // map[string]*UserSession
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	sm := &SessionManager{}

	// Start background cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new session for a user (uses UserSession from database.go)
func (sm *SessionManager) CreateSession(userID, deviceID string) *UserSession {
	now := time.Now()

	// Generate random tokens
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	sessionToken := base64.URLEncoding.EncodeToString(tokenBytes)

	refreshBytes := make([]byte, 64)
	rand.Read(refreshBytes)
	refreshToken := base64.URLEncoding.EncodeToString(refreshBytes)

	session := &UserSession{
		Base: Base{
			ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
			CreatedAt: now,
			UpdatedAt: now,
		},
		UserID:             userID,
		Token:              sessionToken,
		RefreshToken:       refreshToken,
		AccessTokenExpiry:  now.Add(24 * time.Hour),
		RefreshTokenExpiry: now.Add(30 * 24 * time.Hour),
		LastActivityAt:     now,
		IsActive:           true,
	}

	sm.sessions.Store(userID, session)
	return session
}

// UpdateActivity updates last activity timestamp
func (sm *SessionManager) UpdateActivity(userID string) {
	if val, ok := sm.sessions.Load(userID); ok {
		session := val.(*UserSession)
		session.LastActivityAt = time.Now()
	}
}

// IsSessionValid checks if session is still valid
func (sm *SessionManager) IsSessionValid(userID string) (bool, string) {
	val, ok := sm.sessions.Load(userID)
	if !ok {
		return false, "session not found"
	}

	session := val.(*UserSession)

	now := time.Now()

	// Check inactivity timeout
	if now.Sub(session.LastActivityAt) > SessionConfig.InactivityTimeout {
		return false, "session expired due to inactivity"
	}

	// Check token expiry
	if now.After(session.AccessTokenExpiry) {
		return false, "session expired (token expired)"
	}

	return true, ""
}

// EndSession ends a user session
func (sm *SessionManager) EndSession(userID string) {
	sm.sessions.Delete(userID)
}

// cleanupExpiredSessions runs periodically to remove expired sessions
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(SessionConfig.ActivityCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		sm.sessions.Range(func(key, value interface{}) bool {
			userID := key.(string)
			if valid, _ := sm.IsSessionValid(userID); !valid {
				sm.EndSession(userID)
			}
			return true
		})
	}
}

// =============================================================================
// 4. AUDIT LOGGING
// =============================================================================

// AuditEvent represents an auditable event
type AuditEvent struct {
	EventType   string                 // "financial_transaction", "permission_change", "device_approval", "data_export"
	UserID      string
	DeviceID    string
	Action      string                 // "payment_recorded", "role_changed", "device_approved", etc.
	EntityType  string                 // "invoice", "user", "device", "report"
	EntityID    string
	OldValue    string                 // For changes
	NewValue    string                 // For changes
	Amount      float64                // For financial transactions
	Currency    string
	Success     bool
	Reason      string                 // For failures
	Metadata    map[string]interface{} // Additional context
	Timestamp   time.Time
	IPAddress   string
}

// AuditLogger logs security-critical events
type AuditLogger struct {
	logger *Logger
	masker *DataMasker
}

// NewAuditLogger creates a new audit logger
func NewAuditLogger(logger *Logger) *AuditLogger {
	return &AuditLogger{
		logger: logger,
		masker: NewDataMasker(),
	}
}

// LogFinancialTransaction logs financial transactions (payments, invoices)
func (a *AuditLogger) LogFinancialTransaction(userID, action, entityType, entityID string, amount float64, currency string, success bool, metadata map[string]interface{}) {
	// Mask sensitive data in metadata
	sanitized := a.masker.SanitizeForLog(metadata)

	_ = AuditEvent{
		EventType:  "financial_transaction",
		UserID:     userID,
		Action:     action,
		EntityType: entityType,
		EntityID:   entityID,
		Amount:     amount,
		Currency:   currency,
		Success:    success,
		Metadata:   sanitized,
		Timestamp:  time.Now(),
	}

	a.logger.Security(action, success, map[string]interface{}{
		"audit_event":   "financial_transaction",
		"user_id":       userID,
		"entity_type":   entityType,
		"entity_id":     entityID,
		"amount_masked": a.masker.MaskPaymentAmount(amount),
		"currency":      currency,
		"metadata":      sanitized,
	})
}

// LogPermissionChange logs changes to user permissions or roles
func (a *AuditLogger) LogPermissionChange(adminUserID, targetUserID, action, oldValue, newValue string, success bool, reason string) {
	a.logger.Security(action, success, map[string]interface{}{
		"audit_event":    "permission_change",
		"admin_user_id":  adminUserID,
		"target_user_id": targetUserID,
		"old_value":      oldValue,
		"new_value":      newValue,
		"reason":         reason,
	})
}

// LogDeviceAction logs device approval/blocking actions
func (a *AuditLogger) LogDeviceAction(adminUserID, deviceID, action string, success bool, metadata map[string]interface{}) {
	a.logger.Security(action, success, map[string]interface{}{
		"audit_event":   "device_action",
		"admin_user_id": adminUserID,
		"device_id":     deviceID,
		"metadata":      metadata,
	})
}

// LogDataExport logs data export operations
func (a *AuditLogger) LogDataExport(userID, exportType, format string, recordCount int, success bool) {
	a.logger.Security("data_export", success, map[string]interface{}{
		"audit_event":  "data_export",
		"user_id":      userID,
		"export_type":  exportType,
		"format":       format,
		"record_count": recordCount,
	})
}

// =============================================================================
// 5. RATE LIMITING
// =============================================================================

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	limiters sync.Map // map[string]*bucketLimiter
}

// bucketLimiter represents a single rate limit bucket
type bucketLimiter struct {
	tokens      int
	maxTokens   int
	refillRate  time.Duration
	lastRefill  time.Time
	mu          sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{}
}

// Allow checks if an action is allowed under rate limit
func (rl *RateLimiter) Allow(key string, maxTokens int, refillRate time.Duration) bool {
	val, _ := rl.limiters.LoadOrStore(key, &bucketLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	})

	bucket := val.(*bucketLimiter)
	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(bucket.lastRefill)
	tokensToAdd := int(elapsed / bucket.refillRate)

	if tokensToAdd > 0 {
		bucket.tokens += tokensToAdd
		if bucket.tokens > bucket.maxTokens {
			bucket.tokens = bucket.maxTokens
		}
		bucket.lastRefill = now
	}

	// Check if we have tokens available
	if bucket.tokens > 0 {
		bucket.tokens--
		return true
	}

	return false
}

// RateLimitConfig defines rate limits for various operations
var RateLimitConfig = struct {
	LoginAttemptsPerMinute    int
	APICallsPerMinute         int
	ReportGenerationPerMinute int
}{
	LoginAttemptsPerMinute:    5,  // Max 5 login attempts per minute
	APICallsPerMinute:         60, // Max 60 API calls per minute per user
	ReportGenerationPerMinute: 3,  // Max 3 reports per minute per user
}

// Global instances (initialized in app startup)
var (
	GlobalValidator      *InputValidator
	GlobalMasker         *DataMasker
	GlobalSessionManager *SessionManager
	GlobalAuditLogger    *AuditLogger
	GlobalRateLimiter    *RateLimiter
)

// InitSecurityEnhancements initializes all security components
func InitSecurityEnhancements(logger *Logger) {
	GlobalValidator = NewInputValidator()
	GlobalMasker = NewDataMasker()
	GlobalSessionManager = NewSessionManager()
	GlobalAuditLogger = NewAuditLogger(logger)
	GlobalRateLimiter = NewRateLimiter()
}
