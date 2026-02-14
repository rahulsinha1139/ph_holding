// +build ignore

// Standalone test file for password security functions
// Run with: go run password_security_standalone_test.go

package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"regexp"

	"golang.org/x/crypto/bcrypt"
)

const (
	bcryptCost            = 12
	minPasswordLength     = 8
	defaultPasswordLength = 16
)

// hashPassword securely hashes a password using bcrypt
func hashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(bytes), nil
}

// verifyPassword compares a plain password with a bcrypt hash
func verifyPassword(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// validatePasswordComplexity ensures password meets security requirements
func validatePasswordComplexity(password string) error {
	if len(password) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}

	hasNumber := regexp.MustCompile(`[0-9]`).MatchString(password)
	hasLetter := regexp.MustCompile(`[a-zA-Z]`).MatchString(password)

	if !hasNumber || !hasLetter {
		return fmt.Errorf("password must contain both letters and numbers")
	}

	return nil
}

// generateSecurePassword creates a cryptographically secure random password
func generateSecurePassword(length int) (string, error) {
	if length < minPasswordLength {
		length = defaultPasswordLength
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure password: %w", err)
	}

	password := base64.URLEncoding.EncodeToString(bytes)[:length]

	if err := validatePasswordComplexity(password); err != nil {
		return generateSecurePassword(length)
	}

	return password, nil
}

func main() {
	fmt.Println("🔐 Password Security Test Suite")
	fmt.Println("================================\n")

	passed := 0
	failed := 0

	// Test 1: Hash password
	fmt.Println("Test 1: Hash Password")
	password := "SecurePass123"
	hash, err := hashPassword(password)
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n\n", err)
		failed++
	} else if hash[:4] != "$2a$" && hash[:4] != "$2b$" {
		fmt.Printf("❌ FAILED: Invalid bcrypt hash format\n\n")
		failed++
	} else {
		fmt.Printf("✅ PASSED: Generated bcrypt hash: %s...\n\n", hash[:20])
		passed++
	}

	// Test 2: Verify correct password
	fmt.Println("Test 2: Verify Correct Password")
	err = verifyPassword(password, hash)
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n\n", err)
		failed++
	} else {
		fmt.Printf("✅ PASSED: Password verified successfully\n\n")
		passed++
	}

	// Test 3: Reject wrong password
	fmt.Println("Test 3: Reject Wrong Password")
	err = verifyPassword("WrongPass456", hash)
	if err == nil {
		fmt.Printf("❌ FAILED: Wrong password was accepted\n\n")
		failed++
	} else {
		fmt.Printf("✅ PASSED: Wrong password rejected correctly\n\n")
		passed++
	}

	// Test 4: Validate password complexity - valid
	fmt.Println("Test 4: Validate Password Complexity (Valid)")
	err = validatePasswordComplexity("GoodPass123")
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n\n", err)
		failed++
	} else {
		fmt.Printf("✅ PASSED: Valid password accepted\n\n")
		passed++
	}

	// Test 5: Validate password complexity - too short
	fmt.Println("Test 5: Validate Password Complexity (Too Short)")
	err = validatePasswordComplexity("Pass1")
	if err == nil {
		fmt.Printf("❌ FAILED: Short password was accepted\n\n")
		failed++
	} else {
		fmt.Printf("✅ PASSED: Short password rejected: %v\n\n", err)
		passed++
	}

	// Test 6: Validate password complexity - no numbers
	fmt.Println("Test 6: Validate Password Complexity (No Numbers)")
	err = validatePasswordComplexity("PasswordOnly")
	if err == nil {
		fmt.Printf("❌ FAILED: Password without numbers was accepted\n\n")
		failed++
	} else {
		fmt.Printf("✅ PASSED: Password without numbers rejected: %v\n\n", err)
		passed++
	}

	// Test 7: Generate secure password
	fmt.Println("Test 7: Generate Secure Password")
	generatedPass, err := generateSecurePassword(16)
	if err != nil {
		fmt.Printf("❌ FAILED: %v\n\n", err)
		failed++
	} else if len(generatedPass) != 16 {
		fmt.Printf("❌ FAILED: Generated password has wrong length: %d\n\n", len(generatedPass))
		failed++
	} else if err := validatePasswordComplexity(generatedPass); err != nil {
		fmt.Printf("❌ FAILED: Generated password doesn't meet complexity: %v\n\n", err)
		failed++
	} else {
		fmt.Printf("✅ PASSED: Generated secure password: %s\n\n", generatedPass)
		passed++
	}

	// Test 8: Password randomness
	fmt.Println("Test 8: Password Randomness")
	pass1, _ := generateSecurePassword(16)
	pass2, _ := generateSecurePassword(16)
	if pass1 == pass2 {
		fmt.Printf("❌ FAILED: Two generated passwords are identical\n\n")
		failed++
	} else {
		fmt.Printf("✅ PASSED: Generated passwords are unique\n")
		fmt.Printf("   Pass1: %s\n", pass1)
		fmt.Printf("   Pass2: %s\n\n", pass2)
		passed++
	}

	// Test 9: Hash empty password (should fail)
	fmt.Println("Test 9: Hash Empty Password (Should Fail)")
	_, err = hashPassword("")
	if err == nil {
		fmt.Printf("❌ FAILED: Empty password was accepted\n\n")
		failed++
	} else {
		fmt.Printf("✅ PASSED: Empty password rejected: %v\n\n", err)
		passed++
	}

	// Test 10: Different hashes for same password
	fmt.Println("Test 10: Salt Verification (Same Password, Different Hashes)")
	hash1, _ := hashPassword("TestPass123")
	hash2, _ := hashPassword("TestPass123")
	if hash1 == hash2 {
		fmt.Printf("❌ FAILED: Same password produced identical hashes (salt not working)\n\n")
		failed++
	} else {
		fmt.Printf("✅ PASSED: bcrypt salt working correctly\n")
		fmt.Printf("   Hash1: %s...\n", hash1[:30])
		fmt.Printf("   Hash2: %s...\n\n", hash2[:30])
		passed++
	}

	// Summary
	fmt.Println("================================")
	fmt.Printf("Test Summary: %d passed, %d failed\n", passed, failed)
	if failed == 0 {
		fmt.Println("🎉 All tests passed! Password security is working correctly.")
	} else {
		fmt.Printf("⚠️  %d test(s) failed. Please review.\n", failed)
	}
}
