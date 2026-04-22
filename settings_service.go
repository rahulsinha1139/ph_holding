package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"gorm.io/gorm"
)

// ========================================================================
// SETTINGS SERVICE - Encrypted persistence with HKDF key derivation
// ========================================================================

// SettingsService manages application settings with encryption.
// Uses FieldCrypto (HKDF + AES-256-GCM) when available, with backward
// compatibility for values encrypted with the old SHA-256 derived key.
type SettingsService struct {
	db            *gorm.DB
	encryptionKey []byte // Legacy key (SHA-256 of hardware ID) - kept for migration
	fieldCrypto   *FieldCrypto
	hardwareID    string
}

// NewSettingsService creates a settings service
func NewSettingsService(db *gorm.DB) (*SettingsService, error) {
	svc := &SettingsService{
		db: db,
	}

	// Derive encryption key from hardware ID
	hardwareID, err := getHardwareID()
	if err != nil {
		log.Printf("⚠ Failed to get hardware ID: %v (using fallback)", err)
		hardwareID = "fallback-key-ace-engine"
	}
	svc.hardwareID = hardwareID

	// Keep legacy key for backward-compatible decryption of old values
	hash := sha256.Sum256([]byte(hardwareID + "asymmetrica-salt-2025"))
	svc.encryptionKey = hash[:]

	return svc, nil
}

// SetFieldCrypto upgrades the SettingsService to use FieldCrypto for new encryptions.
// Called after FieldCrypto is initialized in App startup.
func (s *SettingsService) SetFieldCrypto(fc *FieldCrypto) {
	s.fieldCrypto = fc
}

// GetSetting retrieves a setting by key, auto-decrypting if needed.
func (s *SettingsService) GetSetting(key string) (string, error) {
	var setting Setting
	result := s.db.Where("key = ?", key).First(&setting)

	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("setting not found: %s", key)
		}
		return "", result.Error
	}

	if setting.IsEncrypted {
		decrypted, err := s.decryptWithFallback(setting.Value)
		if err != nil {
			return "", fmt.Errorf("decrypt setting %s: %w", key, err)
		}
		return decrypted, nil
	}

	return setting.Value, nil
}

// SetSetting saves a setting. If encrypt=true, uses FieldCrypto (preferred) or legacy.
func (s *SettingsService) SetSetting(key, value, category string, encrypt bool) error {
	valueToStore := value

	if encrypt {
		encrypted, err := s.encrypt(value)
		if err != nil {
			return err
		}
		valueToStore = encrypted
	}

	setting := Setting{
		Key:         key,
		Value:       valueToStore,
		Category:    category,
		IsEncrypted: encrypt,
	}

	result := s.db.Where("key = ?", key).Assign(setting).FirstOrCreate(&setting)
	return result.Error
}

// encrypt uses FieldCrypto if available, falls back to legacy AES-256-GCM.
func (s *SettingsService) encrypt(plaintext string) (string, error) {
	if s.fieldCrypto != nil {
		return s.fieldCrypto.Encrypt(plaintext)
	}
	return s.legacyEncrypt(plaintext)
}

// decryptWithFallback tries FieldCrypto first, then legacy key.
// If legacy key works, re-encrypts with FieldCrypto for migration.
func (s *SettingsService) decryptWithFallback(ciphertext string) (string, error) {
	// 1. Try FieldCrypto (versioned, HKDF-derived)
	if s.fieldCrypto != nil && s.fieldCrypto.IsEncrypted(ciphertext) {
		plaintext, err := s.fieldCrypto.Decrypt(ciphertext)
		if err == nil {
			return plaintext, nil
		}
		// Fall through to legacy
	}

	// 2. Try legacy decrypt (SHA-256 derived key, no version byte)
	plaintext, err := s.legacyDecrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("decryption failed (both new and legacy keys): %w", err)
	}

	// 3. Re-encrypt with FieldCrypto for transparent migration
	if s.fieldCrypto != nil {
		newEncrypted, reErr := s.fieldCrypto.Encrypt(plaintext)
		if reErr == nil {
			// Update in DB with new encryption (fire-and-forget)
			s.db.Model(&Setting{}).Where("value = ?", ciphertext).Update("value", newEncrypted)
			log.Printf("settings: migrated encrypted value to HKDF key")
		}
	}

	return plaintext, nil
}

// legacyEncrypt encrypts using the old SHA-256-derived key (AES-256-GCM, no version byte).
func (s *SettingsService) legacyEncrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// legacyDecrypt decrypts using the old SHA-256-derived key.
func (s *SettingsService) legacyDecrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// getHardwareID returns a machine-specific identifier
func getHardwareID() (string, error) {
	switch runtime.GOOS {
	case "windows":
		// Use WMIC to get motherboard serial
		cmd := exec.Command("wmic", "baseboard", "get", "serialnumber")
		suppressCommandWindow(cmd)
		output, err := cmd.Output()
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.EqualFold(line, "SerialNumber") {
					return line, nil
				}
			}
		}
	case "darwin":
		// Use ioreg to get IOPlatformSerialNumber (Mac serial number)
		cmd := exec.Command("ioreg", "-rd1", "-c", "IOPlatformExpertDevice")
		suppressCommandWindow(cmd)
		output, err := cmd.Output()
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				line = strings.TrimSpace(line)
				if strings.Contains(line, "IOPlatformSerialNumber") {
					// Format: "IOPlatformSerialNumber" = "XXXXXXXXXXXX"
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 {
						serial := strings.TrimSpace(parts[1])
						serial = strings.Trim(serial, "\"")
						if serial != "" {
							return serial, nil
						}
					}
				}
			}
		}
	default:
		// Linux: try /etc/machine-id
		if data, err := os.ReadFile("/etc/machine-id"); err == nil {
			id := strings.TrimSpace(string(data))
			if id != "" {
				return id, nil
			}
		}
	}

	// Fallback: use hostname
	return os.Hostname()
}
