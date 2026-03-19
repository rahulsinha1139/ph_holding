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
// NOTE: Bank account encryption was REMOVED.
// Account numbers and IBANs are printed on every customer invoice PDF —
// they are not secrets. The old encryption was hardware-bound (derived from
// Mac serial number), making the database non-portable across machines.
// ═══════════════════════════════════════════════════════════════════════════

// MigrateBankAccountEncryption strips any leftover encrypted values from
// the 4 known seeded accounts and replaces them with plaintext.
// For any user-added accounts with unrecoverable encrypted values, it
// clears the field so the user can re-enter it via the Manage Accounts UI.
func (a *App) MigrateBankAccountEncryption() {
	if a.db == nil {
		return
	}

	// Known seeded accounts — restore their plaintext values
	knownAccounts := map[string]struct{ acct, iban string }{
		"bank-nbb":     {"99570076", "BH16NBOB00000099570076"},
		"bank-ahli":    {"0010521957001", "BH04AUBB00010521957001"},
		"bank-alsalam": {"205220101000", "BH37ALSA00205220101000"},
		"bank-bbk":     {"100000451009", "BH71BBKU00100000451009"},
	}

	rows, err := a.db.Raw("SELECT id, account_number, iban FROM company_bank_accounts").Rows()
	if err != nil {
		log.Printf("⚠ Bank decryption migration: query failed: %v", err)
		return
	}
	defer rows.Close()

	fixed := 0
	for rows.Next() {
		var id, acctNum, iban string
		if err := rows.Scan(&id, &acctNum, &iban); err != nil {
			continue
		}

		// Check if value looks encrypted (base64 with length > 20)
		acctEncrypted := len(acctNum) > 20 && globalFieldCrypto != nil && globalFieldCrypto.IsEncrypted(acctNum)
		ibanEncrypted := len(iban) > 30 && globalFieldCrypto != nil && globalFieldCrypto.IsEncrypted(iban)

		if !acctEncrypted && !ibanEncrypted {
			continue // Already plaintext
		}

		updates := map[string]interface{}{}
		if known, ok := knownAccounts[id]; ok {
			// Restore known plaintext values
			updates["account_number"] = known.acct
			updates["iban"] = known.iban
		} else {
			// Unknown account — try to decrypt, clear if impossible
			if acctEncrypted {
				if decrypted, err := globalFieldCrypto.Decrypt(acctNum); err == nil {
					updates["account_number"] = decrypted
				} else {
					updates["account_number"] = ""
					log.Printf("⚠ Cleared unrecoverable encrypted account_number for %s — re-enter via Manage Accounts", id)
				}
			}
			if ibanEncrypted {
				if decrypted, err := globalFieldCrypto.Decrypt(iban); err == nil {
					updates["iban"] = decrypted
				} else {
					updates["iban"] = ""
					log.Printf("⚠ Cleared unrecoverable encrypted iban for %s — re-enter via Manage Accounts", id)
				}
			}
		}

		if len(updates) > 0 {
			for col, val := range updates {
				a.db.Exec(fmt.Sprintf("UPDATE company_bank_accounts SET %s = ? WHERE id = ?", col), val, id)
			}
			fixed++
		}
	}

	if fixed > 0 {
		log.Printf("🔓 Decrypted %d bank account(s) — account numbers now stored as plaintext", fixed)
	} else {
		log.Printf("✅ Bank accounts: all fields already plaintext")
	}
}
