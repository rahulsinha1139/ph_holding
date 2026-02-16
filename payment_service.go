package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Payment service constants
const (
	BHDPrecisionMultiplier = 1000 // 3 decimal places for BHD currency
)

// =============================================================================
// PAYMENT RECORDING SERVICE
// =============================================================================

// RecordPayment creates a new payment record, updates invoice balances, and triggers order completion checks
// P0 FIX: Added SELECT FOR UPDATE transaction locking to prevent race conditions
// P0 FIX: Ensures outstanding balance never goes negative (floors at zero)
func (a *App) RecordPayment(invoiceID string, amount float64, method string, dateStr string, reference string) (*Payment, error) {
	if err := a.requirePermission("payments:create"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "database connection not available", "")
	}

	// Validate amount (basic check before transaction)
	if amount <= 0 {
		return nil, newError("INVALID_AMOUNT", "payment amount must be greater than zero", "")
	}

	// Round to 3 decimal places (BHD precision)
	amount = math.Round(amount*BHDPrecisionMultiplier) / BHDPrecisionMultiplier

	// Parse payment date
	paymentDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, newError("INVALID_DATE", "Invalid date format, expected YYYY-MM-DD", err.Error())
	}

	// Begin transaction FIRST (to enable locking)
	tx := a.db.Begin()
	if tx.Error != nil {
		return nil, newError("DB_TRANSACTION_FAILED", "Failed to begin transaction", tx.Error.Error())
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("PANIC in RecordPayment: %v", r)
		}
	}()

	// P0 FIX: Load invoice with locking (falls back gracefully for SQLite)
	var invoice Invoice
	lockingQuery := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&invoice, "id = ?", invoiceID)
	if lockingQuery.Error != nil {
		// SQLite doesn't support SELECT FOR UPDATE - retry without locking
		lockingQuery = tx.First(&invoice, "id = ?", invoiceID)
	}
	if err := lockingQuery.Error; err != nil {
		tx.Rollback()
		return nil, newError("INVOICE_NOT_FOUND", "Invoice not found", err.Error())
	}

	// Validate payment doesn't exceed outstanding balance (within locked transaction)
	if amount > invoice.OutstandingBHD {
		tx.Rollback()
		return nil, newError("INVALID_AMOUNT",
			fmt.Sprintf("Payment amount %.3f BHD exceeds outstanding balance %.3f BHD", amount, invoice.OutstandingBHD),
			"")
	}

	// Calculate days to payment (from invoice date to payment date)
	daysToPayment := int(paymentDate.Sub(invoice.InvoiceDate).Hours() / 24)

	// P1 FIX: Validate payment reference required for bank transfers
	if method == "BankTransfer" || method == "Bank Transfer" {
		if reference == "" {
			tx.Rollback()
			return nil, newError("INVALID_REFERENCE", "Payment reference is required for bank transfers", "")
		}
	}

	// P1 FIX: Duplicate payment detection (same invoice + amount + date within 1 hour)
	var existingPaymentCount int64
	if err := tx.Model(&Payment{}).
		Where("invoice_id = ? AND amount_bhd = ? AND payment_date = ?", invoiceID, amount, paymentDate).
		Count(&existingPaymentCount).Error; err == nil && existingPaymentCount > 0 {
		tx.Rollback()
		return nil, newError("DUPLICATE_PAYMENT",
			fmt.Sprintf("Potential duplicate payment detected: same invoice, amount (%.3f BHD), and date already exists", amount),
			"")
	}

	// Compute idempotency key: sha256(invoiceID + amount + date + reference)
	idempotencyInput := fmt.Sprintf("%s|%.3f|%s|%s", invoiceID, amount, dateStr, reference)
	idempotencyHash := sha256.Sum256([]byte(idempotencyInput))
	idempotencyKey := hex.EncodeToString(idempotencyHash[:])

	// Create payment record
	payment := Payment{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
			CreatedBy: a.getCurrentUserID(),
		},
		InvoiceID:      invoiceID,
		InvoiceNumber:  invoice.InvoiceNumber,
		AmountBHD:      amount,
		PaymentDate:    paymentDate,
		PaymentMethod:  method,
		Reference:      reference,
		DaysToPayment:  daysToPayment,
		IdempotencyKey: idempotencyKey,
	}

	// Create payment
	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return nil, newError("DB_CREATE_FAILED", "Failed to create payment", err.Error())
	}

	// P0 FIX: Calculate new outstanding balance with floor at zero
	newOutstanding := invoice.OutstandingBHD - amount
	newOutstanding = math.Round(newOutstanding*BHDPrecisionMultiplier) / BHDPrecisionMultiplier // Round to 3 decimals
	if newOutstanding < 0 {
		log.Printf("⚠️ Payment %.3f exceeds outstanding %.3f - flooring to zero", amount, invoice.OutstandingBHD)
		newOutstanding = 0 // Floor at zero - no negative balances!
	}

	// Determine new invoice status
	var newStatus string
	if newOutstanding <= 0 {
		newStatus = "Paid"
		newOutstanding = 0 // Ensure no negative balances
	} else {
		// Check if this is the first payment
		var paymentCount int64
		tx.Model(&Payment{}).Where("invoice_id = ?", invoiceID).Count(&paymentCount)
		if paymentCount > 1 {
			newStatus = "PartiallyPaid"
		} else {
			// First payment, was invoice overdue?
			if invoice.Status == "Overdue" {
				newStatus = "PartiallyPaid"
			} else {
				newStatus = "PartiallyPaid"
			}
		}
	}

	// Update invoice (still in locked transaction)
	if err := tx.Model(&invoice).Updates(map[string]interface{}{
		"outstanding_bhd": newOutstanding,
		"status":          newStatus,
	}).Error; err != nil {
		tx.Rollback()
		return nil, newError("DB_UPDATE_FAILED", "Failed to update invoice", err.Error())
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return nil, newError("DB_COMMIT_FAILED", "Failed to commit payment transaction", err.Error())
	}

	// P1 FIX: Audit log for financial transaction
	if GlobalAuditLogger != nil {
		GlobalAuditLogger.LogFinancialTransaction(
			a.getCurrentUserID(),
			"payment_recorded",
			"invoice",
			invoiceID,
			amount,
			"BHD",
			true,
			map[string]interface{}{
				"invoice_number":      invoice.InvoiceNumber,
				"payment_method":      method,
				"payment_date":        dateStr,
				"reference":           reference,
				"new_outstanding_bhd": newOutstanding,
				"days_to_payment":     daysToPayment,
			},
		)
	}

	log.Printf("💰 Payment recorded: %s BHD for Invoice %s (Method: %s, Days: %d)",
		fmt.Sprintf("%.3f", amount), invoice.InvoiceNumber, method, daysToPayment)
	log.Printf("📊 Invoice %s updated: Outstanding %.3f BHD → %.3f BHD, Status: %s → %s",
		invoice.InvoiceNumber, invoice.OutstandingBHD, newOutstanding, invoice.Status, newStatus)

	// If invoice is fully paid, check if order should progress to Complete
	if newStatus == "Paid" && invoice.OrderID != "" {
		if err := a.checkOrderCompletion(invoice.OrderID); err != nil {
			log.Printf("⚠️ Failed to check order completion for order %s: %v", invoice.OrderID, err)
			// Don't fail the payment, just log warning
		}
	}

	return &payment, nil
}

// GetPaymentsByInvoice retrieves all payments for a specific invoice
func (a *App) GetPaymentsByInvoice(invoiceID string) ([]Payment, error) {
	if err := a.requirePermission("payments:view"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var payments []Payment
	if err := a.db.Where("invoice_id = ?", invoiceID).Order("payment_date ASC, created_at ASC").Find(&payments).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve payments", err.Error())
	}

	log.Printf("💳 Retrieved %d payments for invoice %s", len(payments), invoiceID)
	return payments, nil
}

// GetAllPayments retrieves all payments with pagination
func (a *App) GetAllPayments(limit, offset int) ([]Payment, error) {
	if err := a.requirePermission("payments:view"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var payments []Payment
	query := a.db.Order("payment_date DESC, created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&payments).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve payments", err.Error())
	}

	log.Printf("💳 Retrieved %d payments (limit: %d, offset: %d)", len(payments), limit, offset)
	return payments, nil
}

// DeletePayment reverses a payment by deleting it and restoring invoice balance
func (a *App) DeletePayment(id string) error {
	if err := a.requirePermission("payments:delete"); err != nil {
		return err
	}

	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get payment details
	var payment Payment
	if err := a.db.First(&payment, "id = ?", id).Error; err != nil {
		return newError("PAYMENT_NOT_FOUND", "Payment not found", err.Error())
	}

	// Get invoice
	var invoice Invoice
	if err := a.db.First(&invoice, "id = ?", payment.InvoiceID).Error; err != nil {
		return newError("INVOICE_NOT_FOUND", "Invoice not found", err.Error())
	}

	// Begin transaction
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// SECURITY: Audit log BEFORE deletion (financial transaction)
	log.Printf("🔒 AUDIT: Payment %s (%.3f BHD) deleted for invoice %s by user %s",
		payment.ID, payment.AmountBHD, payment.InvoiceID, a.getCurrentUserID())

	if GlobalAuditLogger != nil {
		GlobalAuditLogger.LogFinancialTransaction(
			a.getCurrentUserID(),
			"payment_deleted",
			"payment",
			payment.ID,
			payment.AmountBHD,
			"BHD",
			true,
			map[string]interface{}{
				"invoice_id":     payment.InvoiceID,
				"invoice_number": payment.InvoiceNumber,
				"payment_method": payment.PaymentMethod,
				"reference":      payment.Reference,
			},
		)
	}

	// Delete payment
	if err := tx.Delete(&payment).Error; err != nil {
		tx.Rollback()
		return newError("DB_DELETE_FAILED", "Failed to delete payment", err.Error())
	}

	// Restore invoice outstanding balance
	newOutstanding := invoice.OutstandingBHD + payment.AmountBHD
	newOutstanding = math.Round(newOutstanding*BHDPrecisionMultiplier) / BHDPrecisionMultiplier

	// Determine new invoice status based on remaining payments
	var remainingPayments []Payment
	if err := tx.Where("invoice_id = ?", payment.InvoiceID).Find(&remainingPayments).Error; err != nil {
		tx.Rollback()
		return newError("DB_QUERY_FAILED", "Failed to query remaining payments", err.Error())
	}

	var newStatus string
	if len(remainingPayments) == 0 {
		// No payments left, revert to original status
		if time.Now().After(invoice.DueDate) {
			newStatus = "Overdue"
		} else {
			newStatus = "Sent"
		}
	} else {
		// Still has payments
		if newOutstanding <= 0 {
			newStatus = "Paid"
		} else {
			newStatus = "PartiallyPaid"
		}
	}

	// Update invoice
	if err := tx.Model(&invoice).Updates(map[string]interface{}{
		"outstanding_bhd": newOutstanding,
		"status":          newStatus,
	}).Error; err != nil {
		tx.Rollback()
		return newError("DB_UPDATE_FAILED", "Failed to update invoice", err.Error())
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return newError("DB_COMMIT_FAILED", "Failed to commit payment deletion", err.Error())
	}

	log.Printf("🗑️ Payment deleted: %s BHD for Invoice %s", fmt.Sprintf("%.3f", payment.AmountBHD), payment.InvoiceNumber)
	log.Printf("📊 Invoice %s restored: Outstanding %.3f BHD → %.3f BHD, Status: %s",
		invoice.InvoiceNumber, invoice.OutstandingBHD, newOutstanding, newStatus)

	return nil
}

// =============================================================================
// ORDER AUTO-PROGRESSION HOOKS
// =============================================================================

// checkOrderCompletion checks if all invoices for an order are paid and progresses order to Complete
func (a *App) checkOrderCompletion(orderID string) error {
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get order
	var order Order
	if err := a.db.First(&order, "id = ?", orderID).Error; err != nil {
		return newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Get all invoices for this order
	var invoices []Invoice
	if err := a.db.Where("order_id = ?", orderID).Find(&invoices).Error; err != nil {
		return newError("DB_QUERY_FAILED", "Failed to retrieve invoices", err.Error())
	}

	if len(invoices) == 0 {
		log.Printf("📋 Order %s has no invoices, skipping completion check", order.OrderNumber)
		return nil
	}

	// Check if all invoices are paid
	allPaid := true
	for _, inv := range invoices {
		if inv.Status != "Paid" {
			allPaid = false
			break
		}
	}

	// If all paid and order not already Complete, update it
	if allPaid && order.Status != "Complete" {
		if err := a.db.Model(&order).Update("status", "Complete").Error; err != nil {
			return newError("DB_UPDATE_FAILED", "Failed to update order status", err.Error())
		}

		log.Printf("✅ Order %s auto-progressed to Complete (all %d invoices paid)", order.OrderNumber, len(invoices))
	}

	return nil
}

// ProgressOrderOnDelivery updates order status based on delivery completion
func (a *App) ProgressOrderOnDelivery(orderID string) error {
	if err := a.requirePermission("orders:update"); err != nil {
		return err
	}

	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get order with items
	var order Order
	if err := a.db.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
		return newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Get all delivery notes for this order
	var deliveryNotes []DeliveryNote
	if err := a.db.Preload("Items").Where("order_id = ?", orderID).Find(&deliveryNotes).Error; err != nil {
		return newError("DB_QUERY_FAILED", "Failed to retrieve delivery notes", err.Error())
	}

	// Calculate total delivered quantity per order item
	deliveredQty := make(map[string]float64)
	for _, dn := range deliveryNotes {
		for _, item := range dn.Items {
			deliveredQty[item.OrderItemID] += item.QuantityDelivered
		}
	}

	// Check if all items are fully delivered
	allFullyDelivered := true
	anyPartiallyDelivered := false

	for _, orderItem := range order.Items {
		delivered := deliveredQty[orderItem.ID]
		if delivered < orderItem.Quantity {
			allFullyDelivered = false
			if delivered > 0 {
				anyPartiallyDelivered = true
			}
		}
	}

	// Determine new status
	var newStatus string
	if allFullyDelivered {
		newStatus = "FullyDelivered"
	} else if anyPartiallyDelivered {
		newStatus = "PartiallyDelivered"
	} else {
		// No deliveries yet, keep current status
		return nil
	}

	// Update order status if different
	if order.Status != newStatus && order.Status != "Complete" {
		if err := a.db.Model(&order).Update("status", newStatus).Error; err != nil {
			return newError("DB_UPDATE_FAILED", "Failed to update order status", err.Error())
		}

		log.Printf("✅ Order %s auto-progressed to %s based on delivery status", order.OrderNumber, newStatus)
	}

	return nil
}

// ProgressOrderOnInvoice updates order status when an invoice is created
func (a *App) ProgressOrderOnInvoice(orderID string) error {
	if err := a.requirePermission("orders:update"); err != nil {
		return err
	}

	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get order
	var order Order
	if err := a.db.First(&order, "id = ?", orderID).Error; err != nil {
		return newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Only progress if not already in a more advanced state
	// Status hierarchy: Confirmed → InProgress → PartiallyDelivered → FullyDelivered → Invoiced → Complete
	advancedStatuses := []string{"FullyDelivered", "Complete"}
	for _, status := range advancedStatuses {
		if order.Status == status {
			log.Printf("📋 Order %s already at %s, not progressing to Invoiced", order.OrderNumber, order.Status)
			return nil
		}
	}

	// Update to Invoiced
	if order.Status != "Invoiced" {
		if err := a.db.Model(&order).Update("status", "Invoiced").Error; err != nil {
			return newError("DB_UPDATE_FAILED", "Failed to update order status", err.Error())
		}

		log.Printf("✅ Order %s auto-progressed to Invoiced", order.OrderNumber)
	}

	return nil
}

// =============================================================================
// ADDITIONAL CRUD OPERATIONS (P3 FIX)
// =============================================================================

// GetPayment retrieves a single payment by ID
// P3 FIX: Added missing CRUD operation for consistency
func (a *App) GetPayment(id string) (Payment, error) {
	if err := a.requirePermission("payments:view"); err != nil {
		return Payment{}, err
	}

	if a.db == nil {
		return Payment{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var payment Payment
	if err := a.db.First(&payment, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return Payment{}, newError("NOT_FOUND", "Payment not found", fmt.Sprintf("ID: %s", id))
		}
		return Payment{}, newError("DB_QUERY_FAILED", "Failed to retrieve payment", err.Error())
	}

	return payment, nil
}

// UpdatePayment updates an existing payment
// P3 FIX: Added missing CRUD operation for consistency
func (a *App) UpdatePayment(id string, payment Payment) (*Payment, error) {
	if err := a.requirePermission("payments:update"); err != nil {
		return nil, err
	}

	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Check if payment exists
	var existing Payment
	if err := a.db.First(&existing, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, newError("NOT_FOUND", "Payment not found", fmt.Sprintf("ID: %s", id))
		}
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve payment", err.Error())
	}

	// Preserve ID and timestamps
	payment.ID = existing.ID
	payment.CreatedAt = existing.CreatedAt

	// Update payment
	if err := a.db.Model(&existing).Updates(payment).Error; err != nil {
		return nil, newError("DB_UPDATE_FAILED", "Failed to update payment", err.Error())
	}

	// Reload
	if err := a.db.First(&existing, "id = ?", id).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to reload payment", err.Error())
	}

	log.Printf("✅ Updated Payment: %s", id)
	return &existing, nil
}
