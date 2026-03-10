// ═══════════════════════════════════════════════════════════════════════════
// COMPANY BANK ACCOUNTS SERVICE
//
// MISSION: Manage PH Trading's bank account details for invoice PDFs
//          Provides the 4 official bank accounts used on all invoices
//
// USED BY: Invoice PDF generation (bank details section)
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CompanyBankAccount stores PH Trading's bank account details for invoices
type CompanyBankAccount struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	BankName      string    `gorm:"not null;size:100" json:"bank_name"`
	AccountName   string    `gorm:"not null;size:100" json:"account_name"`
	AccountNumber string    `gorm:"not null;size:50" json:"account_number"`
	IBAN          string    `gorm:"size:50" json:"iban"`
	SwiftBIC      string    `gorm:"size:20" json:"swift_bic"`
	Currency      string    `gorm:"size:3;default:'BHD'" json:"currency"`
	IsActive      bool      `gorm:"default:true" json:"is_active"`
	DisplayOrder  int       `json:"display_order"`
	// BookingRate is the exchange rate at which this foreign-currency account was opened.
	// Used for first-ever FX revaluation gain/loss calculation.
	// 0 = not set; first revaluation will use current rate as baseline (zero gain/loss).
	BookingRate   float64   `gorm:"default:0" json:"booking_rate"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (CompanyBankAccount) TableName() string { return "company_bank_accounts" }

// SeedCompanyBankAccounts creates the 4 PH Trading bank accounts
// Called on app startup to ensure bank details are always available
func (a *App) SeedCompanyBankAccounts() error {
	if a.db == nil {
		return nil
	}

	banks := []CompanyBankAccount{
		{
			ID:            "bank-nbb",
			BankName:      "National Bank of Bahrain BSC",
			AccountName:   "P H TRADING WLL",
			AccountNumber: "99570076",
			IBAN:          "BH16NBOB00000099570076",
			SwiftBIC:      "NBOBBHBM",
			Currency:      "BHD",
			IsActive:      true,
			DisplayOrder:  1,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            "bank-ahli",
			BankName:      "Ahli United Bank",
			AccountName:   "P H TRADING WLL",
			AccountNumber: "0010521957001",
			IBAN:          "BH04AUBB00010521957001",
			SwiftBIC:      "AUBBBHBM",
			Currency:      "BHD",
			IsActive:      true,
			DisplayOrder:  2,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            "bank-alsalam",
			BankName:      "Al Salam Bank",
			AccountName:   "P H TRADING WLL",
			AccountNumber: "205220101000",
			IBAN:          "BH37ALSA00205220101000",
			SwiftBIC:      "ALSABHBM",
			Currency:      "BHD",
			IsActive:      true,
			DisplayOrder:  3,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
		{
			ID:            "bank-bbk",
			BankName:      "BBK",
			AccountName:   "P H TRADING WLL",
			AccountNumber: "100000451009",
			IBAN:          "BH71BBKU00100000451009",
			SwiftBIC:      "BBKUBHBM",
			Currency:      "BHD",
			IsActive:      true,
			DisplayOrder:  4,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		},
	}

	for _, bank := range banks {
		// Use FirstOrCreate to avoid duplicates
		result := a.db.Where("id = ?", bank.ID).FirstOrCreate(&bank)
		if result.Error != nil {
			log.Printf("⚠️ Failed to seed bank account %s: %v", bank.ID, result.Error)
			return result.Error
		}
	}

	log.Println("✅ Company bank accounts seeded (4 banks)")
	return nil
}

// GetActiveBankAccounts returns all active bank accounts for invoice PDF
// Ordered by display_order for consistent presentation
func (a *App) GetActiveBankAccounts() ([]CompanyBankAccount, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, nil
	}

	var banks []CompanyBankAccount
	err := a.db.Where("is_active = ?", true).Order("display_order").Find(&banks).Error
	return banks, err
}

// GetBankAccountByID retrieves a specific bank account
func (a *App) GetBankAccountByID(id string) (*CompanyBankAccount, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, nil
	}

	var bank CompanyBankAccount
	err := a.db.First(&bank, "id = ?", id).Error
	if err != nil {
		// Log detailed error for debugging
		log.Printf("GetBankAccountByID error: %v", err)
		// Return sanitized error to frontend
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}
	return &bank, nil
}

// GetAllBankAccounts returns ALL accounts including inactive (for management UI)
func (a *App) GetAllBankAccounts() ([]CompanyBankAccount, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, nil
	}

	var banks []CompanyBankAccount
	err := a.db.Order("display_order").Find(&banks).Error
	if err != nil {
		log.Printf("GetAllBankAccounts error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}
	return banks, nil
}

// CreateBankAccount creates a new bank account (user-entered or from OCR)
func (a *App) CreateBankAccount(account CompanyBankAccount) (*CompanyBankAccount, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Validation
	if account.BankName == "" {
		return nil, fmt.Errorf("bank name is required")
	}
	if account.AccountNumber == "" {
		return nil, fmt.Errorf("account number is required")
	}
	// BookingRate is the historic rate when the account was opened (for FX revaluation).
	// 0 means "not set"; anything else must be a realistic FX rate (< 1000 BHD per foreign unit).
	if account.BookingRate != 0 && (account.BookingRate < 0.0001 || account.BookingRate > 1000) {
		return nil, fmt.Errorf("booking rate %.6f is outside valid FX range [0.0001, 1000]", account.BookingRate)
	}

	// Generate UUID if not provided
	if account.ID == "" {
		account.ID = uuid.New().String()
	}

	// Set timestamps
	now := time.Now()
	account.CreatedAt = now
	account.UpdatedAt = now

	// Default currency to BHD
	if account.Currency == "" {
		account.Currency = "BHD"
	}

	// Default is_active to true
	account.IsActive = true

	// Set display_order to max+1 if not provided
	if account.DisplayOrder == 0 {
		var maxOrder int
		a.db.Model(&CompanyBankAccount{}).Select("COALESCE(MAX(display_order), 0)").Scan(&maxOrder)
		account.DisplayOrder = maxOrder + 1
	}

	// Create in database
	err := a.db.Create(&account).Error
	if err != nil {
		log.Printf("CreateBankAccount error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	log.Printf("✅ Created bank account: %s - %s", account.BankName, account.AccountNumber)
	return &account, nil
}

// UpdateBankAccount updates an existing bank account
func (a *App) UpdateBankAccount(id string, updates map[string]interface{}) (*CompanyBankAccount, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Find existing account
	var account CompanyBankAccount
	err := a.db.First(&account, "id = ?", id).Error
	if err != nil {
		log.Printf("UpdateBankAccount find error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	// Set updated_at
	updates["updated_at"] = time.Now()

	// Apply updates
	err = a.db.Model(&account).Updates(updates).Error
	if err != nil {
		log.Printf("UpdateBankAccount update error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	// Fetch updated record
	err = a.db.First(&account, "id = ?", id).Error
	if err != nil {
		log.Printf("UpdateBankAccount refetch error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	log.Printf("✅ Updated bank account: %s", id)
	return &account, nil
}

// DeleteBankAccount soft deletes a bank account (sets is_active=false)
func (a *App) DeleteBankAccount(id string) error {
	if err := a.requirePermission("finance:delete"); err != nil {
		return err
	}

	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Check if account has linked bank statements
	var statementCount int64
	err := a.db.Model(&BankStatement{}).Where("bank_account_id = ?", id).Count(&statementCount).Error
	if err != nil {
		log.Printf("DeleteBankAccount count error: %v", err)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	if statementCount > 0 {
		return fmt.Errorf("cannot delete account with linked bank statements - please deactivate instead")
	}

	// Soft delete - set is_active to false
	err = a.db.Model(&CompanyBankAccount{}).Where("id = ?", id).Update("is_active", false).Error
	if err != nil {
		log.Printf("DeleteBankAccount deactivate error: %v", err)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	log.Printf("✅ Deactivated bank account: %s", id)
	return nil
}

// ═══════════════════════════════════════════════════════════════════════════
// GORM HOOKS - Transparent field-level encryption for sensitive bank data
// ═══════════════════════════════════════════════════════════════════════════

// BeforeCreate encrypts AccountNumber and IBAN before inserting to DB.
func (b *CompanyBankAccount) BeforeCreate(tx *gorm.DB) error {
	return b.encryptSensitiveFields()
}

// BeforeSave encrypts AccountNumber and IBAN before updating in DB.
func (b *CompanyBankAccount) BeforeSave(tx *gorm.DB) error {
	return b.encryptSensitiveFields()
}

// AfterFind decrypts AccountNumber and IBAN after reading from DB.
func (b *CompanyBankAccount) AfterFind(tx *gorm.DB) error {
	return b.decryptSensitiveFields()
}

func (b *CompanyBankAccount) encryptSensitiveFields() error {
	if globalFieldCrypto == nil {
		return nil // Encryption not initialized yet (e.g., during early startup seeding)
	}

	// Only encrypt if not already encrypted
	if b.AccountNumber != "" && !globalFieldCrypto.IsEncrypted(b.AccountNumber) {
		encrypted, err := globalFieldCrypto.Encrypt(b.AccountNumber)
		if err != nil {
			return fmt.Errorf("failed to encrypt account number: %w", err)
		}
		b.AccountNumber = encrypted
	}

	if b.IBAN != "" && !globalFieldCrypto.IsEncrypted(b.IBAN) {
		encrypted, err := globalFieldCrypto.Encrypt(b.IBAN)
		if err != nil {
			return fmt.Errorf("failed to encrypt IBAN: %w", err)
		}
		b.IBAN = encrypted
	}

	return nil
}

func (b *CompanyBankAccount) decryptSensitiveFields() error {
	if globalFieldCrypto == nil {
		return nil
	}

	// Only decrypt if value looks encrypted (base64 with version byte)
	if b.AccountNumber != "" && globalFieldCrypto.IsEncrypted(b.AccountNumber) {
		decrypted, err := globalFieldCrypto.Decrypt(b.AccountNumber)
		if err != nil {
			log.Printf("⚠ Failed to decrypt account number for %s: %v", b.ID, err)
			return nil // Don't fail the query - return encrypted value
		}
		b.AccountNumber = decrypted
	}

	if b.IBAN != "" && globalFieldCrypto.IsEncrypted(b.IBAN) {
		decrypted, err := globalFieldCrypto.Decrypt(b.IBAN)
		if err != nil {
			log.Printf("⚠ Failed to decrypt IBAN for %s: %v", b.ID, err)
			return nil
		}
		b.IBAN = decrypted
	}

	return nil
}

// MigrateBankAccountEncryption encrypts any existing plaintext bank account fields.
// Idempotent - skips values that are already encrypted.
// Called once during App startup after FieldCrypto is initialized.
func (a *App) MigrateBankAccountEncryption() {
	if a.db == nil || a.fieldCrypto == nil {
		return
	}

	var banks []CompanyBankAccount
	// Use a raw SQL query to bypass AfterFind hook (we want raw DB values)
	rows, err := a.db.Raw("SELECT id, account_number, iban FROM company_bank_accounts").Rows()
	if err != nil {
		log.Printf("⚠ Bank encryption migration: query failed: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var b CompanyBankAccount
		if err := rows.Scan(&b.ID, &b.AccountNumber, &b.IBAN); err != nil {
			continue
		}
		banks = append(banks, b)
	}

	migrated := 0
	for _, bank := range banks {
		needsUpdate := false
		updates := map[string]interface{}{}

		if bank.AccountNumber != "" && !a.fieldCrypto.IsEncrypted(bank.AccountNumber) {
			encrypted, err := a.fieldCrypto.Encrypt(bank.AccountNumber)
			if err != nil {
				log.Printf("⚠ Failed to encrypt account %s: %v", bank.ID, err)
				continue
			}
			updates["account_number"] = encrypted
			needsUpdate = true
		}

		if bank.IBAN != "" && !a.fieldCrypto.IsEncrypted(bank.IBAN) {
			encrypted, err := a.fieldCrypto.Encrypt(bank.IBAN)
			if err != nil {
				log.Printf("⚠ Failed to encrypt IBAN %s: %v", bank.ID, err)
				continue
			}
			updates["iban"] = encrypted
			needsUpdate = true
		}

		if needsUpdate {
			if err := a.db.Model(&CompanyBankAccount{}).Where("id = ?", bank.ID).Updates(updates).Error; err != nil {
				log.Printf("⚠ Failed to update encrypted fields for %s: %v", bank.ID, err)
			} else {
				migrated++
			}
		}
	}

	if migrated > 0 {
		log.Printf("🔐 Encrypted %d bank account(s) (account numbers + IBANs)", migrated)
	} else {
		log.Printf("✅ Bank account encryption: all fields already encrypted")
	}
}
