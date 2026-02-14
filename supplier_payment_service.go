package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Supplier payment service constants
const (
	FloatingPointTolerance = 0.001           // Tolerance for BHD float comparisons
	FuturePaymentWindow    = 24 * time.Hour // Maximum future date allowed for payments
)

// RecordSupplierPayment creates a new payment record for a supplier invoice
// and updates the invoice payment status if fully paid
func (a *App) RecordSupplierPayment(invoiceID string, amount float64, currency, method, date, reference string) (*SupplierPayment, error) {
	if err := a.requirePermission("payments:record"); err != nil {
		return nil, err
	}

	// Validate invoice exists
	var invoice SupplierInvoice
	if err := a.db.First(&invoice, "id = ?", invoiceID).Error; err != nil {
		return nil, fmt.Errorf("invoice not found: %w", err)
	}

	// P0-4 Fix: Prevent paying disputed/unmatched invoices
	if invoice.Status == "Disputed" {
		return nil, fmt.Errorf("cannot pay disputed invoice %s - resolve dispute first", invoice.InvoiceNumber)
	}
	if invoice.MatchStatus == "Discrepancy" {
		return nil, fmt.Errorf("cannot pay invoice %s with 3-way match discrepancies - resolve discrepancies first", invoice.InvoiceNumber)
	}
	if invoice.Status != "Approved" && invoice.Status != "Verified" && invoice.Status != "Pending" {
		return nil, fmt.Errorf("invoice must be Approved, Verified, or Pending before payment, current status: %s", invoice.Status)
	}

	// Validate amount
	if amount <= 0 {
		return nil, fmt.Errorf("payment amount must be positive, got: %.3f", amount)
	}

	// Set default currency
	if currency == "" {
		currency = "BHD"
	}

	// Set exchange rate (default 1.0 for BHD)
	exchangeRate := 1.0
	if currency != "BHD" {
		// For non-BHD currencies, exchange rate should be provided
		// For now, we'll use 1.0 as default, but this should be enhanced
		log.Printf("Warning: Non-BHD currency %s provided without explicit exchange rate, using 1.0", currency)
	}

	// Calculate amount in BHD
	amountBHD := amount * exchangeRate

	// Parse payment date
	paymentDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("invalid date format (expected YYYY-MM-DD): %w", err)
	}

	// Prevent future payment dates
	if paymentDate.After(time.Now().Add(FuturePaymentWindow)) {
		return nil, fmt.Errorf("payment date cannot be in the future")
	}

	// Check for existing payments to calculate outstanding balance
	var totalPaidBefore float64
	if err := a.db.Model(&SupplierPayment{}).
		Where("supplier_invoice_id = ?", invoiceID).
		Select("COALESCE(SUM(amount_bhd), 0)").
		Scan(&totalPaidBefore).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate existing payments: %w", err)
	}

	// Validate payment doesn't exceed outstanding balance
	outstanding := invoice.TotalBHD - totalPaidBefore
	if amountBHD > outstanding+FloatingPointTolerance { // Small tolerance for floating point
		return nil, fmt.Errorf("payment amount %.3f BHD exceeds outstanding balance %.3f BHD", amountBHD, outstanding)
	}

	// Check for duplicate payment (same invoice, reference, and amount)
	if reference != "" {
		var existingCount int64
		a.db.Model(&SupplierPayment{}).
			Where("supplier_invoice_id = ? AND reference = ? AND amount_bhd = ?", invoiceID, reference, amountBHD).
			Count(&existingCount)
		if existingCount > 0 {
			return nil, fmt.Errorf("duplicate payment detected: same invoice, reference, and amount already exists")
		}
	}

	// Start transaction for atomic payment creation + invoice status update
	tx := a.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Create payment record with display fields populated from invoice
	payment := SupplierPayment{
		SupplierInvoiceID: invoiceID,
		SupplierID:        invoice.SupplierID,
		AmountForeign:     amount,
		Currency:          currency,
		ExchangeRate:      exchangeRate,
		AmountBHD:         amountBHD,
		PaymentDate:       paymentDate,
		PaymentMethod:     method,
		Reference:         reference,
		SupplierName:      invoice.SupplierName,  // Populate for UI display
		InvoiceNumber:     invoice.InvoiceNumber, // Populate for UI display
	}

	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}

	log.Printf("💰 Supplier payment recorded: %.3f %s for invoice %s", amount, currency, invoiceID)

	// Calculate new total paid
	var totalPaid float64
	if err := tx.Model(&SupplierPayment{}).
		Where("supplier_invoice_id = ?", invoiceID).
		Select("COALESCE(SUM(amount_bhd), 0)").
		Scan(&totalPaid).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to calculate total payments: %w", err)
	}

	// Update invoice payment status
	if totalPaid >= invoice.TotalBHD-FloatingPointTolerance { // Small tolerance for floating point
		updates := map[string]interface{}{
			"payment_status": "Paid",
			"payment_date":   paymentDate,
		}
		if err := tx.Model(&SupplierInvoice{}).Where("id = ?", invoiceID).Updates(updates).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update invoice payment status: %w", err)
		}
		log.Printf("✅ Invoice %s marked as Paid (total paid: %.3f BHD >= invoice total: %.3f BHD)",
			invoiceID, totalPaid, invoice.TotalBHD)
	} else if totalPaid > 0 {
		// Partial payment
		updates := map[string]interface{}{
			"payment_status": "Partial",
		}
		if err := tx.Model(&SupplierInvoice{}).Where("id = ?", invoiceID).Updates(updates).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("failed to update invoice to partial payment status: %w", err)
		}
		log.Printf("📊 Invoice %s marked as Partial (paid: %.3f BHD / %.3f BHD)",
			invoiceID, totalPaid, invoice.TotalBHD)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to commit payment transaction: %w", err)
	}

	// P1 FIX: Audit log for financial transaction
	if GlobalAuditLogger != nil {
		GlobalAuditLogger.LogFinancialTransaction(
			a.getCurrentUserID(),
			"supplier_payment_recorded",
			"supplier_invoice",
			invoiceID,
			amountBHD,
			"BHD",
			true,
			map[string]interface{}{
				"supplier_id":      invoice.SupplierID,
				"invoice_number":   invoice.InvoiceNumber,
				"payment_method":   method,
				"payment_date":     date,
				"reference":        reference,
				"amount_foreign":   amount,
				"currency":         currency,
				"exchange_rate":    exchangeRate,
				"payment_status":   "recorded",
			},
		)
	}

	return &payment, nil
}

// GetSupplierPaymentsByInvoice retrieves all payments for a specific supplier invoice
func (a *App) GetSupplierPaymentsByInvoice(invoiceID string) ([]SupplierPayment, error) {
	if err := a.requirePermission("payments:view"); err != nil {
		return nil, err
	}

	var payments []SupplierPayment

	if err := a.db.Where("supplier_invoice_id = ?", invoiceID).
		Order("payment_date DESC").
		Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve payments for invoice %s: %w", invoiceID, err)
	}

	return payments, nil
}

// GetAllSupplierPayments retrieves all supplier payments (limited to 500 most recent)
func (a *App) GetAllSupplierPayments() ([]SupplierPayment, error) {
	if err := a.requirePermission("payments:view"); err != nil {
		return nil, err
	}

	var payments []SupplierPayment

	if err := a.db.Order("payment_date DESC").
		Limit(500).
		Find(&payments).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve supplier payments: %w", err)
	}

	return payments, nil
}

// GetSupplierPaymentsSummary returns summary statistics for supplier payments
func (a *App) GetSupplierPaymentsSummary() (map[string]interface{}, error) {
	if err := a.requirePermission("payments:view"); err != nil {
		return nil, err
	}

	summary := make(map[string]interface{})

	// Total paid amount in BHD
	var totalPaidBHD float64
	if err := a.db.Model(&SupplierPayment{}).
		Select("COALESCE(SUM(amount_bhd), 0)").
		Scan(&totalPaidBHD).Error; err != nil {
		return nil, fmt.Errorf("failed to calculate total paid: %w", err)
	}
	summary["total_paid_bhd"] = totalPaidBHD

	// Count of invoices not fully paid
	var outstandingCount int64
	if err := a.db.Model(&SupplierInvoice{}).
		Where("payment_status != ?", "Paid").
		Count(&outstandingCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count outstanding invoices: %w", err)
	}
	summary["outstanding_count"] = outstandingCount

	// Count of overdue invoices (past due date and not paid)
	var overdueCount int64
	now := time.Now()
	if err := a.db.Model(&SupplierInvoice{}).
		Where("payment_status != ? AND due_date < ?", "Paid", now).
		Count(&overdueCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count overdue invoices: %w", err)
	}
	summary["overdue_count"] = overdueCount

	log.Printf("📊 Supplier payments summary: %.3f BHD paid, %d outstanding, %d overdue",
		totalPaidBHD, outstandingCount, overdueCount)

	return summary, nil
}

// =============================================================================
// ADDITIONAL CRUD OPERATIONS (P3 FIX)
// =============================================================================

// GetSupplierPayment retrieves a single supplier payment by ID
// P3 FIX: Added missing CRUD operation for consistency
func (a *App) GetSupplierPayment(id string) (SupplierPayment, error) {
	if err := a.requirePermission("payments:view"); err != nil {
		return SupplierPayment{}, err
	}

	if a.db == nil {
		return SupplierPayment{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var payment SupplierPayment
	if err := a.db.First(&payment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return SupplierPayment{}, newError("NOT_FOUND", "Supplier payment not found", fmt.Sprintf("ID: %s", id))
		}
		return SupplierPayment{}, newError("DB_QUERY_FAILED", "Failed to retrieve supplier payment", err.Error())
	}

	return payment, nil
}

// UpdateSupplierPayment updates an existing supplier payment
// P3 FIX: Added missing CRUD operation for consistency
func (a *App) UpdateSupplierPayment(id string, payment SupplierPayment) (*SupplierPayment, error) {
	if err := a.requirePermission("supplier_payments:update"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Check if payment exists
	var existing SupplierPayment
	if err := a.db.First(&existing, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, newError("NOT_FOUND", "Supplier payment not found", fmt.Sprintf("ID: %s", id))
		}
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve supplier payment", err.Error())
	}

	// Preserve ID and timestamps
	payment.ID = existing.ID
	payment.CreatedAt = existing.CreatedAt

	// Update payment
	if err := a.db.Model(&existing).Updates(payment).Error; err != nil {
		return nil, newError("DB_UPDATE_FAILED", "Failed to update supplier payment", err.Error())
	}

	// Reload
	if err := a.db.First(&existing, "id = ?", id).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to reload supplier payment", err.Error())
	}

	log.Printf("✅ Updated Supplier Payment: %s", id)
	return &existing, nil
}

// DeleteSupplierPayment deletes a supplier payment by ID
// P3 FIX: Added missing CRUD operation for consistency
func (a *App) DeleteSupplierPayment(id string) error {
	if err := a.requirePermission("supplier_payments:delete"); err != nil {
		return err
	}

	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// P0 FIX: Use transaction to handle invoice balance rollback
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get payment first
	var payment SupplierPayment
	if err := tx.First(&payment, "id = ?", id).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return newError("NOT_FOUND", "Supplier payment not found", fmt.Sprintf("ID: %s", id))
		}
		return newError("DB_QUERY_FAILED", "Failed to retrieve supplier payment", err.Error())
	}

	// Roll back invoice payment (add amount back to outstanding)
	if err := tx.Model(&SupplierInvoice{}).
		Where("id = ?", payment.SupplierInvoiceID).
		UpdateColumn("amount_paid_bhd", gorm.Expr("amount_paid_bhd - ?", payment.AmountBHD)).Error; err != nil {
		tx.Rollback()
		return newError("DB_UPDATE_FAILED", "Failed to roll back invoice payment", err.Error())
	}

	// Delete payment
	if err := tx.Delete(&payment).Error; err != nil {
		tx.Rollback()
		return newError("DB_DELETE_FAILED", "Failed to delete supplier payment", err.Error())
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return newError("DB_COMMIT_FAILED", "Failed to commit transaction", err.Error())
	}

	log.Printf("🗑️ Deleted Supplier Payment: %s (rolled back %.3f BHD)", id, payment.AmountBHD)
	return nil
}
