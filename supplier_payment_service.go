package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// Supplier payment service constants
const (
	FloatingPointTolerance = 0.001          // Tolerance for BHD float comparisons
	FuturePaymentWindow    = 24 * time.Hour // Maximum future date allowed for payments
)

// RecordSupplierPayment creates a new payment record for a supplier invoice
// and updates the invoice payment status if fully paid
func (a *App) RecordSupplierPayment(invoiceID string, amount float64, currency, method, date, reference string) (*SupplierPayment, error) {
	if err := a.requirePermission("payments:record"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Validate invoice exists
	var invoice SupplierInvoice
	if err := a.db.First(&invoice, "id = ?", invoiceID).Error; err != nil {
		return nil, fmt.Errorf("invoice not found: %w", err)
	}

	// Only approved, matched invoices should move into the payment ledger.
	if invoice.Status == "Dispute" {
		return nil, fmt.Errorf("cannot pay disputed invoice %s - resolve dispute first", invoice.InvoiceNumber)
	}
	if invoice.MatchStatus != "Matched" {
		return nil, fmt.Errorf("cannot pay invoice %s until 3-way match is Matched", invoice.InvoiceNumber)
	}
	if invoice.Status != "Approved" {
		return nil, fmt.Errorf("invoice must be Approved before payment, current status: %s", invoice.Status)
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

	// SECURITY FIX (TOCTOU): Start transaction BEFORE balance check to prevent race conditions
	// Two concurrent payments can no longer both pass validation (SELECT FOR UPDATE locks the row)
	tx := a.db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	// Check for existing payments to calculate outstanding balance (inside the transaction).
	totalsBefore, err := a.getSupplierInvoicePaymentTotals(tx, invoiceID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to calculate existing payments: %w", err)
	}

	// Validate payment doesn't exceed outstanding balance (within locked transaction)
	outstanding := supplierInvoiceOutstandingBHD(invoice, totalsBefore.AmountBHD)
	if amountBHD > outstanding+FloatingPointTolerance { // Small tolerance for floating point
		tx.Rollback()
		return nil, fmt.Errorf("payment amount %.3f BHD exceeds outstanding balance %.3f BHD", amountBHD, outstanding)
	}

	// SECURITY FIX: Duplicate payment detection regardless of reference
	var duplicateCount int64
	duplicateQuery := tx.Model(&SupplierPayment{}).Where("supplier_invoice_id = ? AND amount_bhd = ?", invoiceID, amountBHD)
	if reference != "" {
		duplicateQuery = duplicateQuery.Where("reference = ?", reference)
	} else {
		// For payments without reference, check same amount on same date
		duplicateQuery = duplicateQuery.Where("DATE(payment_date) = DATE(?)", paymentDate)
	}
	duplicateQuery.Count(&duplicateCount)
	if duplicateCount > 0 {
		tx.Rollback()
		return nil, fmt.Errorf("duplicate payment detected: matching invoice, amount, and %s already exists",
			func() string {
				if reference != "" {
					return "reference"
				}
				return "date"
			}())
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
		Division:          a.resolveSupplierInvoiceDivision(invoice),
	}

	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create payment record: %w", err)
	}

	log.Printf("💰 Supplier payment recorded: %.3f %s for invoice %s", amount, currency, invoiceID)

	state, err := a.applySupplierInvoicePaymentState(tx, invoiceID)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to update invoice payment state: %w", err)
	}

	log.Printf("📊 Supplier invoice %s payment state: %s / %s (paid %.3f BHD, outstanding %.3f BHD)",
		invoiceID, state.InvoiceStatus, state.PaymentStatus, state.TotalPaidBHD, state.OutstandingBHD)

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
				"supplier_id":    invoice.SupplierID,
				"invoice_number": invoice.InvoiceNumber,
				"payment_method": method,
				"payment_date":   date,
				"reference":      reference,
				"amount_foreign": amount,
				"currency":       currency,
				"exchange_rate":  exchangeRate,
				"payment_status": "recorded",
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
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
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

	// Count of invoices with live outstanding balances.
	var invoices []SupplierInvoice
	if err := a.db.Where("status IS NULL OR status NOT IN ?", []string{"Rejected", "Dispute"}).
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve supplier invoices for payment summary: %w", err)
	}
	if err := a.hydrateSupplierInvoicesPaymentState(a.db, invoices); err != nil {
		return nil, fmt.Errorf("failed to normalize supplier invoices for payment summary: %w", err)
	}

	var outstandingCount int64
	var overdueCount int64
	var outstandingBHD float64
	now := time.Now()
	for _, invoice := range invoices {
		if invoice.OutstandingBHD <= FloatingPointTolerance {
			continue
		}
		outstandingCount++
		outstandingBHD += invoice.OutstandingBHD
		if invoice.DueDate.Before(now) {
			overdueCount++
		}
	}

	summary["outstanding_count"] = outstandingCount
	summary["overdue_count"] = overdueCount
	summary["outstanding_bhd"] = outstandingBHD

	log.Printf("📊 Supplier payments summary: %.3f BHD paid, %.3f BHD outstanding, %d outstanding, %d overdue",
		totalPaidBHD, outstandingBHD, outstandingCount, overdueCount)

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
// SECURITY FIX: Added validation for amount, invoice bounds, and invoice change prevention
func (a *App) UpdateSupplierPayment(id string, payment SupplierPayment) (*SupplierPayment, error) {
	if err := a.requirePermission("supplier_payments:update"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	tx := a.db.Begin()
	if tx.Error != nil {
		return nil, newError("DB_TRANSACTION_FAILED", "Failed to start transaction", tx.Error.Error())
	}

	// Check if payment exists
	var existing SupplierPayment
	if err := tx.First(&existing, "id = ?", id).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return nil, newError("NOT_FOUND", "Supplier payment not found", fmt.Sprintf("ID: %s", id))
		}
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve supplier payment", err.Error())
	}

	// SECURITY FIX: Validate amount is positive
	if payment.AmountBHD <= 0 {
		return nil, newError("INVALID_AMOUNT", "Payment amount must be greater than zero", fmt.Sprintf("got: %.3f", payment.AmountBHD))
	}

	// SECURITY FIX: Prevent changing the linked supplier invoice
	if payment.SupplierInvoiceID != "" && payment.SupplierInvoiceID != existing.SupplierInvoiceID {
		return nil, newError("INVALID_UPDATE", "Cannot change the supplier invoice linked to a payment - delete and recreate instead", "")
	}

	// SECURITY FIX: Validate amount against supplier invoice outstanding balance
	var invoice SupplierInvoice
	if err := tx.First(&invoice, "id = ?", existing.SupplierInvoiceID).Error; err != nil {
		tx.Rollback()
		return nil, newError("INVOICE_NOT_FOUND", "Linked supplier invoice not found", err.Error())
	}

	// Calculate total already paid for this invoice (excluding current payment being updated)
	var totalPaidExcludingCurrent float64
	if err := tx.Model(&SupplierPayment{}).
		Where("supplier_invoice_id = ? AND id != ?", existing.SupplierInvoiceID, id).
		Select("COALESCE(SUM(amount_bhd), 0)").
		Scan(&totalPaidExcludingCurrent).Error; err != nil {
		tx.Rollback()
		return nil, newError("DB_QUERY_FAILED", "Failed to calculate existing payments", err.Error())
	}

	// The max allowed is invoice total minus what others have paid
	maxAllowedAmount := invoice.TotalBHD - totalPaidExcludingCurrent + FloatingPointTolerance
	payment.Division = a.resolveSupplierInvoiceDivision(invoice)
	if payment.AmountBHD > maxAllowedAmount {
		return nil, newError("INVALID_AMOUNT",
			fmt.Sprintf("Payment amount %.3f BHD exceeds available balance %.3f BHD (invoice total %.3f - other payments %.3f)",
				payment.AmountBHD, maxAllowedAmount-FloatingPointTolerance, invoice.TotalBHD, totalPaidExcludingCurrent),
			"")
	}

	log.Printf("AUDIT: UpdateSupplierPayment id=%s by user=%s: amount %.3f -> %.3f, invoice=%s",
		id, a.getCurrentUserID(), existing.AmountBHD, payment.AmountBHD, existing.SupplierInvoiceID)

	// Preserve ID, timestamps, and invoice linkage
	payment.ID = existing.ID
	payment.CreatedAt = existing.CreatedAt
	payment.SupplierInvoiceID = existing.SupplierInvoiceID
	payment.SupplierID = existing.SupplierID
	if payment.SupplierName == "" {
		payment.SupplierName = existing.SupplierName
	}
	if payment.InvoiceNumber == "" {
		payment.InvoiceNumber = existing.InvoiceNumber
	}

	// Update payment
	if err := tx.Model(&existing).Updates(payment).Error; err != nil {
		tx.Rollback()
		return nil, newError("DB_UPDATE_FAILED", "Failed to update supplier payment", err.Error())
	}

	state, err := a.applySupplierInvoicePaymentState(tx, existing.SupplierInvoiceID)
	if err != nil {
		tx.Rollback()
		return nil, newError("DB_UPDATE_FAILED", "Failed to update linked supplier invoice payment state", err.Error())
	}

	log.Printf("AUDIT: Supplier invoice %s payment state recalculated: %s / %s (paid %.3f / %.3f)",
		invoice.InvoiceNumber, state.InvoiceStatus, state.PaymentStatus, state.TotalPaidBHD, invoice.TotalBHD)

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return nil, newError("DB_COMMIT_FAILED", "Failed to commit supplier payment update", err.Error())
	}

	// Reload
	if err := a.db.First(&existing, "id = ?", id).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to reload supplier payment", err.Error())
	}

	log.Printf("AUDIT: Supplier payment updated successfully: id=%s, amount=%.3f BHD, invoice=%s",
		id, existing.AmountBHD, existing.SupplierInvoiceID)
	return &existing, nil
}

// DeleteSupplierPayment deletes a supplier payment by ID
// P3 FIX: Added missing CRUD operation for consistency
func (a *App) DeleteSupplierPayment(id string) error {
	if ok, err := a.guardDeleteOrRequest("supplier_payments:delete", "supplier_payment", id, "Supplier payment"); !ok {
		return err
	}
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

	// Delete payment
	if err := tx.Delete(&payment).Error; err != nil {
		tx.Rollback()
		return newError("DB_DELETE_FAILED", "Failed to delete supplier payment", err.Error())
	}

	if _, err := a.applySupplierInvoicePaymentState(tx, payment.SupplierInvoiceID); err != nil {
		tx.Rollback()
		return newError("DB_UPDATE_FAILED", "Failed to recalculate linked supplier invoice after deleting payment", err.Error())
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return newError("DB_COMMIT_FAILED", "Failed to commit transaction", err.Error())
	}

	log.Printf("🗑️ Deleted Supplier Payment: %s (rolled back %.3f BHD)", id, payment.AmountBHD)
	return nil
}
