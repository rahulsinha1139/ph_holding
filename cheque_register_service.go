// =============================================================================
// CHEQUE REGISTER SERVICE
//
// MISSION: Manage cheque lifecycle from issuance to clearance
// FEATURES: Cheque books, issuance, clearance tracking, stale cheque handling
// =============================================================================

package main

import (
	"fmt"
	"log"
	"time"
)

// =============================================================================
// CHEQUE REGISTER MANAGEMENT
// =============================================================================

// CreateChequeRegister creates a new cheque book register
func (a *App) CreateChequeRegister(bankAccountID, chequeBookNo string, startNum, endNum int) (*ChequeRegister, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	if startNum >= endNum {
		return nil, fmt.Errorf("start number must be less than end number")
	}

	// Check for overlapping registers
	var existing ChequeRegister
	err := a.db.Where("bank_account_id = ? AND status = ? AND ((start_number <= ? AND end_number >= ?) OR (start_number <= ? AND end_number >= ?))",
		bankAccountID, "ACTIVE", startNum, startNum, endNum, endNum).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("overlapping cheque register exists: %s", existing.ChequeBookNo)
	}

	register := &ChequeRegister{
		BankAccountID: bankAccountID,
		ChequeBookNo:  chequeBookNo,
		StartNumber:   startNum,
		EndNumber:     endNum,
		CurrentNumber: startNum,
		Status:        "ACTIVE",
		IssuedDate:    time.Now(),
	}

	if err := a.db.Create(register).Error; err != nil {
		return nil, fmt.Errorf("failed to create register: %w", err)
	}

	log.Printf("📗 Cheque register created: %s (#%d - #%d)", chequeBookNo, startNum, endNum)
	return register, nil
}

// GetChequeRegisters retrieves all registers for a bank account
func (a *App) GetChequeRegisters(bankAccountID string) ([]ChequeRegister, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var registers []ChequeRegister
	err := a.db.Where("bank_account_id = ?", bankAccountID).
		Order("issued_date DESC").
		Find(&registers).Error

	return registers, err
}

// GetActiveChequeRegister gets the active register for a bank account
func (a *App) GetActiveChequeRegister(bankAccountID string) (*ChequeRegister, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var register ChequeRegister
	err := a.db.Where("bank_account_id = ? AND status = ?", bankAccountID, "ACTIVE").
		First(&register).Error

	if err != nil {
		return nil, fmt.Errorf("no active cheque register found")
	}

	return &register, nil
}

// GetNextChequeNumber returns the next available cheque number
func (a *App) GetNextChequeNumber(bankAccountID string) (string, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return "", err
	}
	if a.db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	register, err := a.GetActiveChequeRegister(bankAccountID)
	if err != nil {
		return "", err
	}

	if register.CurrentNumber > register.EndNumber {
		return "", fmt.Errorf("cheque book exhausted - please create a new register")
	}

	return fmt.Sprintf("%06d", register.CurrentNumber), nil
}

// ExhaustChequeRegister marks a register as exhausted
func (a *App) ExhaustChequeRegister(registerID string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	now := time.Now()
	return a.db.Model(&ChequeRegister{}).
		Where("id = ?", registerID).
		Updates(map[string]interface{}{
			"status":         "EXHAUSTED",
			"exhausted_date": now,
		}).Error
}

// =============================================================================
// CHEQUE ISSUANCE
// =============================================================================

// IssueCheque issues a new cheque and records it as outstanding
func (a *App) IssueCheque(bankAccountID string, amount float64, payeeName, payeeType string, supplierID *string, purpose string) (*OutstandingCheque, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get next cheque number
	chequeNumber, err := a.GetNextChequeNumber(bankAccountID)
	if err != nil {
		return nil, err
	}

	cheque := &OutstandingCheque{
		BankAccountID: bankAccountID,
		ChequeNumber:  chequeNumber,
		Amount:        amount,
		Currency:      "BHD",
		IssuedDate:    time.Now(),
		PayeeName:     payeeName,
		PayeeType:     payeeType,
		SupplierID:    supplierID,
		Purpose:       purpose,
		Status:        "ISSUED",
	}

	if err := a.db.Create(cheque).Error; err != nil {
		return nil, fmt.Errorf("failed to issue cheque: %w", err)
	}

	// Increment the register's current number
	register, _ := a.GetActiveChequeRegister(bankAccountID)
	if register != nil {
		newNumber := register.CurrentNumber + 1
		a.db.Model(register).Update("current_number", newNumber)

		// Check if exhausted
		if newNumber > register.EndNumber {
			a.ExhaustChequeRegister(register.ID)
		}
	}

	log.Printf("📝 Cheque issued: #%s for %.3f BHD to %s", chequeNumber, amount, payeeName)
	return cheque, nil
}

// =============================================================================
// CHEQUE LIFECYCLE
// =============================================================================

// MarkChequePresented marks a cheque as presented
func (a *App) MarkChequePresented(chequeNumber string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	result := a.db.Model(&OutstandingCheque{}).
		Where("cheque_number = ? AND status = ?", chequeNumber, "ISSUED").
		Update("status", "PRESENTED")

	if result.RowsAffected == 0 {
		return fmt.Errorf("cheque not found or not in ISSUED status")
	}

	log.Printf("📥 Cheque presented: #%s", chequeNumber)
	return nil
}

// MarkChequeCleared marks a cheque as cleared and links to bank statement line
func (a *App) MarkChequeCleared(chequeNumber, bankStatementLineID string, clearedDate time.Time) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	result := a.db.Model(&OutstandingCheque{}).
		Where("cheque_number = ? AND status IN (?, ?)", chequeNumber, "ISSUED", "PRESENTED").
		Updates(map[string]interface{}{
			"status":          "CLEARED",
			"cleared_date":    clearedDate,
			"matched_line_id": bankStatementLineID,
		})

	if result.RowsAffected == 0 {
		return fmt.Errorf("cheque not found or already cleared")
	}

	log.Printf("✅ Cheque cleared: #%s on %s", chequeNumber, clearedDate.Format("2006-01-02"))
	return nil
}

// MarkChequeStale marks a cheque as stale (>6 months)
func (a *App) MarkChequeStale(chequeNumber string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	now := time.Now()
	result := a.db.Model(&OutstandingCheque{}).
		Where("cheque_number = ? AND status IN (?, ?)", chequeNumber, "ISSUED", "PRESENTED").
		Updates(map[string]interface{}{
			"status":     "STALE",
			"is_stale":   true,
			"stale_date": now,
		})

	if result.RowsAffected == 0 {
		return fmt.Errorf("cheque not found or cannot be marked stale")
	}

	log.Printf("⏳ Cheque marked stale: #%s", chequeNumber)
	return nil
}

// MarkChequeBounced marks a cheque as bounced
func (a *App) MarkChequeBounced(chequeNumber, reason string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	result := a.db.Model(&OutstandingCheque{}).
		Where("cheque_number = ? AND status IN (?, ?)", chequeNumber, "ISSUED", "PRESENTED").
		Updates(map[string]interface{}{
			"status":  "BOUNCED",
			"purpose": reason, // Append reason to purpose
		})

	if result.RowsAffected == 0 {
		return fmt.Errorf("cheque not found or cannot be marked bounced")
	}

	log.Printf("❌ Cheque bounced: #%s - %s", chequeNumber, reason)
	return nil
}

// CancelCheque cancels a cheque
func (a *App) CancelCheque(chequeNumber, reason string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	result := a.db.Model(&OutstandingCheque{}).
		Where("cheque_number = ? AND status = ?", chequeNumber, "ISSUED").
		Updates(map[string]interface{}{
			"status":  "CANCELLED",
			"purpose": reason,
		})

	if result.RowsAffected == 0 {
		return fmt.Errorf("cheque not found or not in ISSUED status")
	}

	log.Printf("🚫 Cheque cancelled: #%s - %s", chequeNumber, reason)
	return nil
}

// ReissueCheque issues a new cheque to replace a stale/cancelled one
func (a *App) ReissueCheque(oldChequeNumber, bankAccountID string) (*OutstandingCheque, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get old cheque
	var oldCheque OutstandingCheque
	if err := a.db.Where("cheque_number = ?", oldChequeNumber).First(&oldCheque).Error; err != nil {
		return nil, fmt.Errorf("original cheque not found: %w", err)
	}

	if oldCheque.Status != "STALE" && oldCheque.Status != "CANCELLED" {
		return nil, fmt.Errorf("only stale or cancelled cheques can be reissued")
	}

	// Issue new cheque
	newCheque, err := a.IssueCheque(bankAccountID, oldCheque.Amount, oldCheque.PayeeName, oldCheque.PayeeType, oldCheque.SupplierID, "Reissue of #"+oldChequeNumber)
	if err != nil {
		return nil, err
	}

	// Update old cheque with reference to new one
	a.db.Model(&oldCheque).Update("reissued_as", newCheque.ChequeNumber)

	log.Printf("🔄 Cheque reissued: #%s → #%s", oldChequeNumber, newCheque.ChequeNumber)
	return newCheque, nil
}

// =============================================================================
// QUERIES
// =============================================================================

// GetOutstandingCheques returns all outstanding (not cleared) cheques
func (a *App) GetOutstandingCheques(bankAccountID string) ([]OutstandingCheque, float64, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, 0, err
	}
	if a.db == nil {
		return nil, 0, fmt.Errorf("database not initialized")
	}

	var cheques []OutstandingCheque
	query := a.db.Where("status IN (?, ?)", "ISSUED", "PRESENTED")
	if bankAccountID != "" {
		query = query.Where("bank_account_id = ?", bankAccountID)
	}

	if err := query.Order("issued_date DESC").Find(&cheques).Error; err != nil {
		return nil, 0, err
	}

	var total float64
	for _, c := range cheques {
		total += c.Amount
	}

	return cheques, total, nil
}

// GetChequeByNumber retrieves a cheque by its number
func (a *App) GetChequeByNumber(chequeNumber string) (*OutstandingCheque, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var cheque OutstandingCheque
	if err := a.db.Where("cheque_number = ?", chequeNumber).First(&cheque).Error; err != nil {
		return nil, fmt.Errorf("cheque not found: %w", err)
	}

	return &cheque, nil
}

// GetChequesByStatus retrieves cheques by status
func (a *App) GetChequesByStatus(bankAccountID, status string) ([]OutstandingCheque, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var cheques []OutstandingCheque
	query := a.db.Where("status = ?", status)
	if bankAccountID != "" {
		query = query.Where("bank_account_id = ?", bankAccountID)
	}

	err := query.Order("issued_date DESC").Find(&cheques).Error
	return cheques, err
}

// GetStaleCheques returns cheques that are >6 months old and not cleared
func (a *App) GetStaleCheques(bankAccountID string) ([]OutstandingCheque, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	sixMonthsAgo := time.Now().AddDate(0, -6, 0)

	var cheques []OutstandingCheque
	query := a.db.Where("status IN (?, ?) AND issued_date < ?", "ISSUED", "PRESENTED", sixMonthsAgo)
	if bankAccountID != "" {
		query = query.Where("bank_account_id = ?", bankAccountID)
	}

	err := query.Order("issued_date ASC").Find(&cheques).Error
	return cheques, err
}

// GetChequeRegisterReport generates a report for a date range
func (a *App) GetChequeRegisterReport(bankAccountID string, startDate, endDate time.Time) (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get all cheques in date range
	var cheques []OutstandingCheque
	a.db.Where("bank_account_id = ? AND issued_date BETWEEN ? AND ?",
		bankAccountID, startDate, endDate).
		Order("issued_date ASC").
		Find(&cheques)

	// Count by status
	statusCounts := make(map[string]int)
	statusAmounts := make(map[string]float64)
	for _, c := range cheques {
		statusCounts[c.Status]++
		statusAmounts[c.Status] += c.Amount
	}

	var totalAmount float64
	for _, c := range cheques {
		totalAmount += c.Amount
	}

	// Get outstanding totals
	outstandingCheques, outstandingTotal, _ := a.GetOutstandingCheques(bankAccountID)

	return map[string]interface{}{
		"bank_account_id":     bankAccountID,
		"start_date":          startDate,
		"end_date":            endDate,
		"total_cheques":       len(cheques),
		"total_amount":        totalAmount,
		"status_counts":       statusCounts,
		"status_amounts":      statusAmounts,
		"outstanding_count":   len(outstandingCheques),
		"outstanding_total":   outstandingTotal,
		"cheques":             cheques,
	}, nil
}
