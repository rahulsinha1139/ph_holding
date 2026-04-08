package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// =============================================================================
// SUPPLIER INVOICE CRUD - Full Operations Pipeline Support
// =============================================================================
//
// FEATURES:
//   - Complete CRUD for SupplierInvoice entity
//   - Multi-currency support (EUR, USD, BHD, etc.)
//   - 3-way matching (PO ↔ GRN ↔ Invoice)
//   - Approval workflow (Pending → Verified → Approved → Paid)
//   - Payment tracking with references
//   - OCR document linking
//   - Dispute management
//
// Built with MATHEMATICAL RIGOR × PRODUCTION ROBUSTNESS
// Day 196+ - Operations Pipeline Phase 1
// =============================================================================

// CreateSupplierInvoice creates a new supplier invoice
func (a *App) CreateSupplierInvoice(inv SupplierInvoice) (SupplierInvoice, error) {
	if err := a.requirePermission("po:create"); err != nil {
		return SupplierInvoice{}, err
	}
	if a.db == nil {
		return SupplierInvoice{}, fmt.Errorf("database not initialized")
	}

	// Generate UUID if not provided
	if inv.ID == "" {
		inv.ID = uuid.New().String()
	}

	// Set defaults
	if inv.Status == "" {
		inv.Status = "Pending"
	}
	if inv.PaymentStatus == "" {
		inv.PaymentStatus = "Unpaid"
	}
	if inv.MatchStatus == "" {
		inv.MatchStatus = "Pending"
	}
	if inv.Currency == "" {
		inv.Currency = "BHD" // Default to Bahraini Dinar
	}
	if inv.ExchangeRate == 0 {
		inv.ExchangeRate = 1.0 // Default to 1:1 for BHD
	}

	// Enrich supplier_name from SupplierMaster (denormalized for display)
	if inv.SupplierID != "" && inv.SupplierName == "" {
		var supplier SupplierMaster
		if err := a.db.Where("id = ?", inv.SupplierID).First(&supplier).Error; err == nil {
			inv.SupplierName = supplier.SupplierName
		} else {
			log.Printf("⚠️ Could not find supplier %s: %v", inv.SupplierID, err)
		}
	}

	// Enrich PO number from PurchaseOrder (denormalized for display)
	if inv.PurchaseOrderID != "" && inv.PONumber == "" {
		var po PurchaseOrder
		if err := a.db.Where("id = ?", inv.PurchaseOrderID).First(&po).Error; err == nil {
			inv.PONumber = po.PONumber
		}
	}

	// Check for duplicate invoice number for same supplier
	if inv.InvoiceNumber != "" && inv.SupplierID != "" {
		var existingCount int64
		a.db.Model(&SupplierInvoice{}).Where("supplier_id = ? AND invoice_number = ?",
			inv.SupplierID, inv.InvoiceNumber).Count(&existingCount)
		if existingCount > 0 {
			return SupplierInvoice{}, fmt.Errorf("duplicate invoice: invoice number '%s' already exists for this supplier", inv.InvoiceNumber)
		}
	}

	// Timestamp
	now := time.Now()
	inv.CreatedAt = now
	inv.UpdatedAt = now

	// Validate that subtotal + VAT ≈ total (arithmetic sanity check before storing)
	if inv.TotalForeign > 0 {
		expectedTotal := inv.SubtotalForeign + inv.VATForeign
		diff := expectedTotal - inv.TotalForeign
		if diff < 0 {
			diff = -diff
		}
		if diff > 0.01 {
			return SupplierInvoice{}, fmt.Errorf("invoice total mismatch: subtotal %.3f + VAT %.3f = %.3f, but total provided is %.3f",
				inv.SubtotalForeign, inv.VATForeign, expectedTotal, inv.TotalForeign)
		}
	}

	// Calculate BHD amounts from foreign currency if needed
	if inv.Currency != "BHD" {
		inv.SubtotalBHD = inv.SubtotalForeign * inv.ExchangeRate
		inv.VATBHD = inv.VATForeign * inv.ExchangeRate
		inv.TotalBHD = inv.TotalForeign * inv.ExchangeRate
	} else {
		// If BHD, sync both fields
		inv.SubtotalBHD = inv.SubtotalForeign
		inv.VATBHD = inv.VATForeign
		inv.TotalBHD = inv.TotalForeign
	}

	// Store items separately, create invoice first without them
	items := inv.Items
	inv.Items = nil

	// Use transaction to ensure invoice + items are created atomically
	tx := a.db.Begin()
	if tx.Error != nil {
		return SupplierInvoice{}, fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}

	if err := tx.Create(&inv).Error; err != nil {
		tx.Rollback()
		return SupplierInvoice{}, fmt.Errorf("failed to create supplier invoice: %w", err)
	}

	// Create line items if provided
	if len(items) > 0 {
		for i := range items {
			items[i].SupplierInvoiceID = inv.ID
			if items[i].ID == "" {
				items[i].ID = uuid.New().String()
			}
			items[i].LineNumber = i + 1
			if items[i].Currency == "" {
				items[i].Currency = inv.Currency
			}
		}
		if err := tx.Create(&items).Error; err != nil {
			tx.Rollback()
			return SupplierInvoice{}, fmt.Errorf("failed to create invoice items: %w", err)
		}
		inv.Items = items
	}

	if err := tx.Commit().Error; err != nil {
		return SupplierInvoice{}, fmt.Errorf("failed to commit supplier invoice: %w", err)
	}

	log.Printf("Created Supplier Invoice: %s (Supplier: %s, Total: %.2f %s, Items: %d)",
		inv.InvoiceNumber, inv.SupplierID, inv.TotalForeign, inv.Currency, len(items))

	return inv, nil
}

// GetSupplierInvoices retrieves all supplier invoices
func (a *App) GetSupplierInvoices() ([]SupplierInvoice, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var invoices []SupplierInvoice
	if err := a.db.Preload("Items").Order("invoice_date DESC").Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve supplier invoices: %w", err)
	}

	log.Printf("📊 Retrieved %d supplier invoices", len(invoices))
	return invoices, nil
}

// GetSupplierInvoiceByID retrieves a single supplier invoice by ID
func (a *App) GetSupplierInvoiceByID(id string) (SupplierInvoice, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return SupplierInvoice{}, err
	}
	if a.db == nil {
		return SupplierInvoice{}, fmt.Errorf("database not initialized")
	}

	var invoice SupplierInvoice
	if err := a.db.Preload("Items").Where("id = ?", id).First(&invoice).Error; err != nil {
		return SupplierInvoice{}, fmt.Errorf("supplier invoice not found: %w", err)
	}

	return invoice, nil
}

// GetSupplierInvoicesBySupplier retrieves all invoices for a specific supplier
func (a *App) GetSupplierInvoicesBySupplier(supplierID string) ([]SupplierInvoice, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var invoices []SupplierInvoice
	if err := a.db.Preload("Items").Where("supplier_id = ?", supplierID).
		Order("invoice_date DESC").
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve supplier invoices: %w", err)
	}

	log.Printf("📊 Retrieved %d invoices for supplier %s", len(invoices), supplierID)
	return invoices, nil
}

// GetSupplierInvoicesByPO retrieves all invoices for a specific purchase order
func (a *App) GetSupplierInvoicesByPO(poID string) ([]SupplierInvoice, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var invoices []SupplierInvoice
	if err := a.db.Preload("Items").Where("purchase_order_id = ?", poID).
		Order("invoice_date DESC").
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve invoices for PO: %w", err)
	}

	log.Printf("📊 Retrieved %d invoices for PO %s", len(invoices), poID)
	return invoices, nil
}

// UpdateSupplierInvoice updates an existing supplier invoice
func (a *App) UpdateSupplierInvoice(inv SupplierInvoice) (SupplierInvoice, error) {
	if err := a.requirePermission("po:create"); err != nil {
		return SupplierInvoice{}, err
	}
	if a.db == nil {
		return SupplierInvoice{}, fmt.Errorf("database not initialized")
	}

	// Verify invoice exists
	var existing SupplierInvoice
	if err := a.db.Where("id = ?", inv.ID).First(&existing).Error; err != nil {
		return SupplierInvoice{}, fmt.Errorf("supplier invoice not found: %w", err)
	}

	// Re-enrich denormalized fields if IDs changed
	if inv.SupplierID != "" && inv.SupplierID != existing.SupplierID {
		var supplier SupplierMaster
		if err := a.db.Where("id = ?", inv.SupplierID).First(&supplier).Error; err == nil {
			inv.SupplierName = supplier.SupplierName
		}
	}
	if inv.PurchaseOrderID != "" && inv.PurchaseOrderID != existing.PurchaseOrderID {
		var po PurchaseOrder
		if err := a.db.Where("id = ?", inv.PurchaseOrderID).First(&po).Error; err == nil {
			inv.PONumber = po.PONumber
		}
	}

	// Update timestamp
	inv.UpdatedAt = time.Now()

	if inv.PaymentStatus == "Paid" {
		inv.Status = "Paid"
		if inv.PaymentDate == nil {
			now := time.Now()
			inv.PaymentDate = &now
		}
	} else if inv.Status == "Paid" && inv.PaymentStatus != "Paid" {
		inv.Status = "Approved"
	}

	// Recalculate BHD amounts if currency changed
	if inv.Currency != "BHD" {
		inv.SubtotalBHD = inv.SubtotalForeign * inv.ExchangeRate
		inv.VATBHD = inv.VATForeign * inv.ExchangeRate
		inv.TotalBHD = inv.TotalForeign * inv.ExchangeRate
	} else {
		inv.SubtotalBHD = inv.SubtotalForeign
		inv.VATBHD = inv.VATForeign
		inv.TotalBHD = inv.TotalForeign
	}

	if err := a.db.Save(&inv).Error; err != nil {
		return SupplierInvoice{}, fmt.Errorf("failed to update supplier invoice: %w", err)
	}

	log.Printf("✅ Updated Supplier Invoice: %s", inv.InvoiceNumber)
	return inv, nil
}

// UpdateSupplierInvoiceWithPayment updates a supplier invoice and, when requested,
// creates the missing supplier payment ledger entry from the invoice editor itself.
func (a *App) UpdateSupplierInvoiceWithPayment(inv SupplierInvoice, createPaymentEntry bool) (SupplierInvoice, error) {
	if err := a.requirePermission("po:create"); err != nil {
		return SupplierInvoice{}, err
	}
	if a.db == nil {
		return SupplierInvoice{}, fmt.Errorf("database not initialized")
	}

	tx := a.db.Begin()
	if tx.Error != nil {
		return SupplierInvoice{}, fmt.Errorf("failed to start transaction: %w", tx.Error)
	}

	var existing SupplierInvoice
	if err := tx.Where("id = ?", inv.ID).First(&existing).Error; err != nil {
		tx.Rollback()
		return SupplierInvoice{}, fmt.Errorf("supplier invoice not found: %w", err)
	}

	if inv.SupplierID != "" && inv.SupplierID != existing.SupplierID {
		var supplier SupplierMaster
		if err := tx.Where("id = ?", inv.SupplierID).First(&supplier).Error; err == nil {
			inv.SupplierName = supplier.SupplierName
		}
	}
	if inv.PurchaseOrderID != "" && inv.PurchaseOrderID != existing.PurchaseOrderID {
		var po PurchaseOrder
		if err := tx.Where("id = ?", inv.PurchaseOrderID).First(&po).Error; err == nil {
			inv.PONumber = po.PONumber
		}
	}

	inv.UpdatedAt = time.Now()

	if inv.PaymentStatus == "Paid" {
		inv.Status = "Paid"
		if inv.PaymentDate == nil {
			now := time.Now()
			inv.PaymentDate = &now
		}
	} else if inv.Status == "Paid" && inv.PaymentStatus != "Paid" {
		inv.Status = "Approved"
	}

	if inv.Currency != "BHD" {
		inv.SubtotalBHD = inv.SubtotalForeign * inv.ExchangeRate
		inv.VATBHD = inv.VATForeign * inv.ExchangeRate
		inv.TotalBHD = inv.TotalForeign * inv.ExchangeRate
	} else {
		inv.SubtotalBHD = inv.SubtotalForeign
		inv.VATBHD = inv.VATForeign
		inv.TotalBHD = inv.TotalForeign
	}

	if err := tx.Save(&inv).Error; err != nil {
		tx.Rollback()
		return SupplierInvoice{}, fmt.Errorf("failed to update supplier invoice: %w", err)
	}

	if createPaymentEntry && inv.PaymentStatus == "Paid" {
		var totals struct {
			AmountBHD     float64
			AmountForeign float64
		}
		if err := tx.Model(&SupplierPayment{}).
			Where("supplier_invoice_id = ?", inv.ID).
			Select("COALESCE(SUM(amount_bhd), 0) AS amount_bhd, COALESCE(SUM(amount_foreign), 0) AS amount_foreign").
			Scan(&totals).Error; err != nil {
			tx.Rollback()
			return SupplierInvoice{}, fmt.Errorf("failed to calculate existing supplier payments: %w", err)
		}

		remainingBHD := inv.TotalBHD - totals.AmountBHD
		if remainingBHD > FloatingPointTolerance {
			currency := inv.Currency
			if currency == "" {
				currency = "BHD"
			}

			exchangeRate := inv.ExchangeRate
			if exchangeRate <= 0 {
				exchangeRate = 1
			}

			amountForeign := remainingBHD
			if currency != "BHD" {
				remainingForeign := inv.TotalForeign - totals.AmountForeign
				if remainingForeign > FloatingPointTolerance {
					amountForeign = remainingForeign
				} else {
					amountForeign = remainingBHD / exchangeRate
				}
			}

			paymentMethod := inv.PaymentMethod
			if paymentMethod == "" {
				paymentMethod = "Other"
			}

			paymentDate := time.Now()
			if inv.PaymentDate != nil {
				paymentDate = *inv.PaymentDate
			}

			payment := SupplierPayment{
				SupplierInvoiceID: inv.ID,
				SupplierID:        inv.SupplierID,
				AmountForeign:     amountForeign,
				Currency:          currency,
				ExchangeRate:      exchangeRate,
				AmountBHD:         remainingBHD,
				PaymentDate:       paymentDate,
				PaymentMethod:     paymentMethod,
				Reference:         inv.PaymentRef,
				Notes:             "Created from supplier invoice edit",
				SupplierName:      inv.SupplierName,
				InvoiceNumber:     inv.InvoiceNumber,
			}

			if err := tx.Create(&payment).Error; err != nil {
				tx.Rollback()
				return SupplierInvoice{}, fmt.Errorf("failed to create supplier payment ledger entry: %w", err)
			}
		}
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return SupplierInvoice{}, fmt.Errorf("failed to commit supplier invoice update: %w", err)
	}

	log.Printf("✅ Updated Supplier Invoice from editor: %s (create payment entry=%v)", inv.InvoiceNumber, createPaymentEntry)
	return inv, nil
}

// DeleteSupplierInvoice deletes a supplier invoice (soft delete)
func (a *App) DeleteSupplierInvoice(id string) error {
	if err := a.requirePermission("po:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Verify invoice exists and is deletable (not paid)
	var invoice SupplierInvoice
	if err := a.db.Where("id = ?", id).First(&invoice).Error; err != nil {
		return fmt.Errorf("supplier invoice not found: %w", err)
	}

	if invoice.PaymentStatus == "Paid" {
		return fmt.Errorf("cannot delete paid invoice: %s", invoice.InvoiceNumber)
	}

	if err := a.db.Delete(&invoice).Error; err != nil {
		return fmt.Errorf("failed to delete supplier invoice: %w", err)
	}

	log.Printf("🗑️ Deleted Supplier Invoice: %s", invoice.InvoiceNumber)
	return nil
}

// =============================================================================
// APPROVAL WORKFLOW
// =============================================================================

// ApproveSupplierInvoice approves a supplier invoice
func (a *App) ApproveSupplierInvoice(id string, approvedBy string) error {
	if err := a.requirePermission("po:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Require approver identity
	if approvedBy == "" {
		return fmt.Errorf("approver identity is required")
	}

	var invoice SupplierInvoice
	if err := a.db.Where("id = ?", id).First(&invoice).Error; err != nil {
		return fmt.Errorf("supplier invoice not found: %w", err)
	}

	// Segregation of duties: require a recorded creator, then verify approver differs.
	// If CreatedBy is empty (OCR/legacy import), require it to be set before approval
	// so there is always an audit trail linking creation and approval to different people.
	if invoice.CreatedBy == "" {
		return fmt.Errorf("segregation of duties: invoice %s has no creator recorded — set CreatedBy before approving", invoice.InvoiceNumber)
	}
	if invoice.CreatedBy == approvedBy {
		return fmt.Errorf("segregation of duties: invoice creator %s cannot approve their own invoice", approvedBy)
	}

	// Verify 3-way match passed before approving
	if invoice.MatchStatus == "Discrepancy" {
		return fmt.Errorf("cannot approve invoice with discrepancies: %s", invoice.InvoiceNumber)
	}

	now := time.Now()
	invoice.Status = "Approved"
	invoice.ApprovedBy = approvedBy
	invoice.ApprovedAt = &now
	invoice.UpdatedAt = now

	if err := a.db.Save(&invoice).Error; err != nil {
		return fmt.Errorf("failed to approve invoice: %w", err)
	}

	log.Printf("✅ Approved Supplier Invoice: %s by %s", invoice.InvoiceNumber, approvedBy)
	return nil
}

// DisputeSupplierInvoice marks an invoice as disputed
func (a *App) DisputeSupplierInvoice(id string, reason string) error {
	if err := a.requirePermission("po:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var invoice SupplierInvoice
	if err := a.db.Where("id = ?", id).First(&invoice).Error; err != nil {
		return fmt.Errorf("supplier invoice not found: %w", err)
	}

	invoice.Status = "Disputed"
	invoice.DisputeReason = reason
	invoice.UpdatedAt = time.Now()

	if err := a.db.Save(&invoice).Error; err != nil {
		return fmt.Errorf("failed to dispute invoice: %w", err)
	}

	log.Printf("⚠️ Disputed Supplier Invoice: %s - Reason: %s", invoice.InvoiceNumber, reason)
	return nil
}

// =============================================================================
// PAYMENT TRACKING
// =============================================================================

// MarkSupplierInvoicePaid marks an invoice as paid
func (a *App) MarkSupplierInvoicePaid(id string, paymentRef string, paymentMethod string) error {
	if err := a.requirePermission("po:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var invoice SupplierInvoice
	if err := a.db.Where("id = ?", id).First(&invoice).Error; err != nil {
		return fmt.Errorf("supplier invoice not found: %w", err)
	}

	// Verify invoice is approved before marking as paid
	if invoice.Status != "Approved" {
		return fmt.Errorf("invoice must be approved before payment: %s", invoice.InvoiceNumber)
	}

	now := time.Now()
	invoice.PaymentStatus = "Paid"
	invoice.Status = "Paid"
	invoice.PaymentDate = &now
	invoice.PaymentRef = paymentRef
	invoice.PaymentMethod = paymentMethod
	invoice.UpdatedAt = now

	if err := a.db.Save(&invoice).Error; err != nil {
		return fmt.Errorf("failed to mark invoice as paid: %w", err)
	}

	log.Printf("💰 Marked Supplier Invoice as Paid: %s (Ref: %s, Method: %s)",
		invoice.InvoiceNumber, paymentRef, paymentMethod)
	return nil
}

// GetUnpaidSupplierInvoices retrieves all unpaid invoices
func (a *App) GetUnpaidSupplierInvoices() ([]SupplierInvoice, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var invoices []SupplierInvoice
	if err := a.db.Preload("Items").Where("payment_status = ?", "Unpaid").
		Order("due_date ASC").
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve unpaid invoices: %w", err)
	}

	log.Printf("📊 Retrieved %d unpaid supplier invoices", len(invoices))
	return invoices, nil
}

// GetOverdueSupplierInvoices retrieves all overdue invoices
func (a *App) GetOverdueSupplierInvoices() ([]SupplierInvoice, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := time.Now()
	var invoices []SupplierInvoice
	if err := a.db.Preload("Items").Where("payment_status = ? AND due_date < ?", "Unpaid", now).
		Order("due_date ASC").
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve overdue invoices: %w", err)
	}

	log.Printf("⚠️ Retrieved %d overdue supplier invoices", len(invoices))
	return invoices, nil
}

// =============================================================================
// 3-WAY MATCHING (PO ↔ GRN ↔ Invoice)
// =============================================================================

// ThreeWayMatchResult represents the result of a 3-way match operation
type ThreeWayMatchResult struct {
	Matched bool   `json:"matched"`
	Reason  string `json:"reason"`
}

// PerformThreeWayMatch performs 3-way matching for a supplier invoice
// Returns: ThreeWayMatchResult struct with matched status and discrepancy reason
// P1 FIX: Enhanced to check unit prices with 2% tolerance threshold
func (a *App) PerformThreeWayMatch(invoiceID string) (ThreeWayMatchResult, error) {
	if err := a.requirePermission("po:create"); err != nil {
		return ThreeWayMatchResult{}, err
	}
	if a.db == nil {
		return ThreeWayMatchResult{}, fmt.Errorf("database not initialized")
	}

	// Get invoice with items
	var invoice SupplierInvoice
	if err := a.db.Preload("Items").Where("id = ?", invoiceID).First(&invoice).Error; err != nil {
		return ThreeWayMatchResult{}, fmt.Errorf("supplier invoice not found: %w", err)
	}

	discrepancies := []string{}
	priceVarianceFlag := false
	tolerance := 0.02 // 2% tolerance threshold

	// Check 1: Verify PO exists and compare amounts
	if invoice.PurchaseOrderID == "" {
		discrepancies = append(discrepancies, "No PO linked")
	} else {
		var po PurchaseOrder
		if err := a.db.Preload("Items").Where("id = ?", invoice.PurchaseOrderID).First(&po).Error; err == nil {
			// P1 FIX: Enhanced 3-way match - check BOTH total amounts AND unit prices

			// Total amount check (2% tolerance)
			amountTolerance := po.TotalBHD * tolerance
			amountDiff := invoice.TotalBHD - po.TotalBHD
			if amountDiff > amountTolerance {
				discrepancies = append(discrepancies, fmt.Sprintf("Amount exceeds PO: Invoice=%.3f BHD, PO=%.3f BHD (diff=%.3f BHD, tolerance=%.3f BHD)",
					invoice.TotalBHD, po.TotalBHD, amountDiff, amountTolerance))
			}

			// P1 FIX: Unit price matching - check each line item
			if len(invoice.Items) > 0 {
				// Build PO item map for lookup
				poItemMap := make(map[string]PurchaseOrderItem)
				for _, poItem := range po.Items {
					poItemMap[poItem.ProductID] = poItem
				}

				for _, invItem := range invoice.Items {
					// Match by line number or product (simplified - in production, use proper linking)
					// For now, check if there's a corresponding PO item
					if len(po.Items) >= invItem.LineNumber && invItem.LineNumber > 0 {
						poItem := po.Items[invItem.LineNumber-1]

						// Convert invoice item price to BHD if needed
						invUnitPriceBHD := invItem.UnitPrice
						if invoice.Currency != "BHD" {
							invUnitPriceBHD = invItem.UnitPrice * invoice.ExchangeRate
						}

						// Calculate unit price variance
						priceDiff := invUnitPriceBHD - poItem.UnitPriceBHD
						priceVariance := 0.0
						if poItem.UnitPriceBHD > 0 {
							priceVariance = (priceDiff / poItem.UnitPriceBHD)
						}

						// P1 FIX: Flag if price variance exceeds 2% tolerance
						if priceVariance > tolerance || priceVariance < -tolerance {
							priceVarianceFlag = true
							discrepancies = append(discrepancies,
								fmt.Sprintf("Line %d unit price variance: Invoice=%.3f BHD, PO=%.3f BHD (variance=%.1f%%, tolerance=±%.0f%%)",
									invItem.LineNumber, invUnitPriceBHD, poItem.UnitPriceBHD, priceVariance*100, tolerance*100))
						}
					}
				}
			}
		} else {
			discrepancies = append(discrepancies, "Linked PO not found in database")
		}
	}

	// Check 2: Verify GRN exists and quantities received
	if invoice.GRNID == "" {
		discrepancies = append(discrepancies, "No GRN linked")
	} else {
		var grn GoodsReceivedNote
		if err := a.db.Preload("Items").Where("id = ?", invoice.GRNID).First(&grn).Error; err == nil {
			// Compare invoice quantities with GRN quantities by GRN ProductID → POItemID linkage.
			// SupplierInvoiceItem has no ProductID, so we match via line number but guard the
			// bounds properly to avoid an out-of-range panic when GRN has fewer items.
			if len(invoice.Items) > 0 && len(grn.Items) > 0 {
				// Build GRN lookup by POItemID for best-effort product matching
				grnByPOItem := make(map[string]GRNItem)
				for _, gi := range grn.Items {
					if gi.POItemID != "" {
						grnByPOItem[gi.POItemID] = gi
					}
				}
				for _, invItem := range invoice.Items {
					// Line-number fallback: use GRN array index (1-based → 0-based)
					if invItem.LineNumber < 1 || invItem.LineNumber > len(grn.Items) {
						continue // line number out of GRN range — cannot compare
					}
					grnItem := grn.Items[invItem.LineNumber-1]
					qtyDiff := invItem.Quantity - grnItem.QuantityAccepted
					qtyVariance := 0.0
					if grnItem.QuantityAccepted > 0 {
						qtyVariance = qtyDiff / grnItem.QuantityAccepted
					}
					if qtyVariance > tolerance || qtyVariance < -tolerance {
						discrepancies = append(discrepancies,
							fmt.Sprintf("Line %d quantity mismatch: Invoice=%.2f, GRN Accepted=%.2f (variance=%.1f%%)",
								invItem.LineNumber, invItem.Quantity, grnItem.QuantityAccepted, qtyVariance*100))
					}
				}
			}
		} else {
			discrepancies = append(discrepancies, "Linked GRN not found in database")
		}
	}

	// Check 3: Basic data validation
	if invoice.TotalBHD <= 0 {
		discrepancies = append(discrepancies, "Invalid total amount")
	}

	if invoice.InvoiceNumber == "" {
		discrepancies = append(discrepancies, "Missing invoice number")
	}

	// Determine match status
	matched := len(discrepancies) == 0
	var discrepancyReason string

	if matched {
		invoice.MatchStatus = "Matched"
		invoice.POMatchOK = true
		invoice.GRNMatchOK = true
		invoice.Status = "Verified"
		log.Printf("✅ 3-Way Match PASSED: Invoice %s (all checks passed)", invoice.InvoiceNumber)
	} else {
		// P1 FIX: Distinguish between minor variances (within tolerance, manual review) and hard failures
		if priceVarianceFlag {
			invoice.MatchStatus = "Review Required"
			invoice.Status = "Pending"
			log.Printf("⚠️ 3-Way Match NEEDS REVIEW: Invoice %s - Price variance exceeds tolerance", invoice.InvoiceNumber)
		} else {
			invoice.MatchStatus = "Discrepancy"
			invoice.Status = "Pending"
			log.Printf("⚠️ 3-Way Match FAILED: Invoice %s", invoice.InvoiceNumber)
		}

		invoice.POMatchOK = invoice.PurchaseOrderID != ""
		invoice.GRNMatchOK = invoice.GRNID != ""
		discrepancyReason = fmt.Sprintf("Discrepancies found: %v", discrepancies)
		invoice.DiscrepancyReason = discrepancyReason
	}

	invoice.UpdatedAt = time.Now()

	// Save updated invoice
	if err := a.db.Save(&invoice).Error; err != nil {
		return ThreeWayMatchResult{}, fmt.Errorf("failed to update match status: %w", err)
	}

	return ThreeWayMatchResult{
		Matched: matched,
		Reason:  discrepancyReason,
	}, nil
}

// =============================================================================
// OCR INTEGRATION
// =============================================================================

// CreateSupplierInvoiceFromOCR creates a supplier invoice from an OCR document
func (a *App) CreateSupplierInvoiceFromOCR(ocrDocID string, supplierID string, poID string) (SupplierInvoice, error) {
	if err := a.requirePermission("po:create"); err != nil {
		return SupplierInvoice{}, err
	}
	if a.db == nil {
		return SupplierInvoice{}, fmt.Errorf("database not initialized")
	}

	// Retrieve OCR document (using uint ID instead of string)
	var ocrDoc OCRDocument
	if err := a.db.Where("id = ?", ocrDocID).First(&ocrDoc).Error; err != nil {
		return SupplierInvoice{}, fmt.Errorf("OCR document not found: %w", err)
	}

	// Parse ExtractedDataJSON into map
	var extractedData map[string]interface{}
	if ocrDoc.ExtractedDataJSON != "" {
		if err := json.Unmarshal([]byte(ocrDoc.ExtractedDataJSON), &extractedData); err != nil {
			log.Printf("⚠️ Failed to parse OCR JSON, using empty map: %v", err)
			extractedData = make(map[string]interface{})
		}
	} else {
		extractedData = make(map[string]interface{})
	}

	// Build supplier invoice from OCR data (Base struct has ID field)
	invoice := SupplierInvoice{
		SupplierID:      supplierID,
		PurchaseOrderID: poID,
		OCRDocumentID:   ocrDocID,
		OCRConfidence:   func() float64 { c := ocrDoc.Confidence; if c < 0 { return 0 }; if c > 1 { return 1 }; return c }(),
		Status:          "Pending",
		PaymentStatus:   "Unpaid",
		MatchStatus:     "Pending",
		Currency:        "BHD", // Default, can be overridden
		ExchangeRate:    1.0,
	}
	// ID will be auto-generated by Base.BeforeCreate hook

	// Enrich supplier_name from SupplierMaster (denormalized for display)
	if supplierID != "" {
		var supplier SupplierMaster
		if err := a.db.Where("id = ?", supplierID).First(&supplier).Error; err == nil {
			invoice.SupplierName = supplier.SupplierName
		}
	}

	// Enrich PO number from PurchaseOrder (denormalized for display)
	if poID != "" {
		var po PurchaseOrder
		if err := a.db.Where("id = ?", poID).First(&po).Error; err == nil {
			invoice.PONumber = po.PONumber
		}
	}

	// Extract invoice number
	if invNum, ok := extractedData["invoice_number"].(string); ok && invNum != "" {
		invoice.InvoiceNumber = invNum
	} else {
		// Generate fallback invoice number
		invoice.InvoiceNumber = fmt.Sprintf("OCR-%s", time.Now().Format("20060102-150405"))
	}

	// Extract dates
	if invDate, ok := extractedData["invoice_date"].(string); ok && invDate != "" {
		parsedDate, err := time.Parse("2006-01-02", invDate)
		if err == nil {
			invoice.InvoiceDate = parsedDate
		} else {
			invoice.InvoiceDate = time.Now()
		}
	} else {
		invoice.InvoiceDate = time.Now()
	}

	if dueDate, ok := extractedData["due_date"].(string); ok && dueDate != "" {
		parsedDate, err := time.Parse("2006-01-02", dueDate)
		if err == nil {
			invoice.DueDate = parsedDate
		} else {
			// Default to 30 days from invoice date
			invoice.DueDate = invoice.InvoiceDate.AddDate(0, 0, 30)
		}
	} else {
		invoice.DueDate = invoice.InvoiceDate.AddDate(0, 0, 30)
	}

	// Extract amounts
	if total, ok := extractedData["total"].(float64); ok {
		invoice.TotalForeign = total
		invoice.TotalBHD = total // Assuming BHD for now
	}

	if subtotal, ok := extractedData["subtotal"].(float64); ok {
		invoice.SubtotalForeign = subtotal
		invoice.SubtotalBHD = subtotal
	}

	if vat, ok := extractedData["vat"].(float64); ok {
		invoice.VATForeign = vat
		invoice.VATBHD = vat
	}

	// Extract currency if present
	if currency, ok := extractedData["currency"].(string); ok && currency != "" {
		invoice.Currency = currency
	}

	// Extract PO number if present (and poID not provided)
	if poID == "" {
		if poNum, ok := extractedData["po_number"].(string); ok && poNum != "" {
			// NOTE: Would need to look up actual PO by number when PO entity is implemented
			log.Printf("📝 OCR extracted PO Number: %s (lookup not yet implemented)", poNum)
		}
	}

	// Create the invoice
	now := time.Now()
	invoice.CreatedAt = now
	invoice.UpdatedAt = now

	if err := a.db.Create(&invoice).Error; err != nil {
		return SupplierInvoice{}, fmt.Errorf("failed to create invoice from OCR: %w", err)
	}

	log.Printf("✅ Created Supplier Invoice from OCR: %s (Confidence: %.2f, Doc: %s)",
		invoice.InvoiceNumber, invoice.OCRConfidence, ocrDocID)

	return invoice, nil
}

// =============================================================================
// P2 FIX: SUPPLIER LEAD TIME TRACKING & PERFORMANCE ANALYTICS
// =============================================================================

// SupplierLeadTimeMetrics tracks supplier delivery performance
type SupplierLeadTimeMetrics struct {
	SupplierID         string  `json:"supplier_id"`
	SupplierName       string  `json:"supplier_name"`
	QuotedLeadTimeDays int     `json:"quoted_lead_time_days"`
	ActualLeadTimeDays int     `json:"actual_lead_time_days"`
	LeadTimeVariance   int     `json:"lead_time_variance"`     // Actual - Quoted
	VariancePercent    float64 `json:"variance_percent"`
	TotalPOs           int     `json:"total_pos"`
	OnTimePOs          int     `json:"on_time_pos"`
	LatePOs            int     `json:"late_pos"`
	OnTimeRate         float64 `json:"on_time_rate"`          // Percentage
	PerformanceGrade   string  `json:"performance_grade"`      // A/B/C/D based on on-time rate
	AverageLatenessDays int    `json:"average_lateness_days"` // For late deliveries
}

// CalculateSupplierLeadTime calculates actual lead time from PO to GRN
func (a *App) CalculateSupplierLeadTime(supplierID string) (SupplierLeadTimeMetrics, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return SupplierLeadTimeMetrics{}, err
	}
	if a.db == nil {
		return SupplierLeadTimeMetrics{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get supplier details
	var supplier SupplierMaster
	if err := a.db.First(&supplier, "id = ?", supplierID).Error; err != nil {
		return SupplierLeadTimeMetrics{}, newError("SUPPLIER_NOT_FOUND", "Supplier not found", err.Error())
	}

	// Get all POs for this supplier with GRNs
	var purchaseOrders []PurchaseOrder
	if err := a.db.Where("supplier_id = ?", supplierID).
		Where("status IN (?)", []string{"Partially Received", "Received"}).
		Find(&purchaseOrders).Error; err != nil {
		return SupplierLeadTimeMetrics{}, newError("DB_QUERY_FAILED", "Failed to retrieve POs", err.Error())
	}

	if len(purchaseOrders) == 0 {
		// No completed POs yet
		return SupplierLeadTimeMetrics{
			SupplierID:         supplierID,
			SupplierName:       supplier.SupplierName,
			QuotedLeadTimeDays: supplier.LeadTimeDays,
			ActualLeadTimeDays: 0,
			LeadTimeVariance:   0,
			TotalPOs:           0,
			OnTimePOs:          0,
			LatePOs:            0,
			OnTimeRate:         0,
			PerformanceGrade:   "N/A",
		}, nil
	}

	// Calculate metrics
	totalLeadTime := 0
	totalPOs := 0
	onTimePOs := 0
	latePOs := 0
	totalLateness := 0

	for _, po := range purchaseOrders {
		// Get GRN for this PO
		var grns []GoodsReceivedNote
		if err := a.db.Where("purchase_order_id = ?", po.ID).
			Order("received_date ASC").
			Find(&grns).Error; err != nil || len(grns) == 0 {
			continue
		}

		// Use first GRN date (partial or full receipt)
		firstGRN := grns[0]

		// Calculate actual lead time (PO date to GRN received date)
		actualLeadTime := int(firstGRN.ReceivedDate.Sub(po.PODate).Hours() / 24)
		totalLeadTime += actualLeadTime
		totalPOs++

		// Check if on time (compare to expected delivery or quoted lead time)
		quotedLeadTime := supplier.LeadTimeDays
		if !po.ExpectedDelivery.IsZero() {
			quotedLeadTime = int(po.ExpectedDelivery.Sub(po.PODate).Hours() / 24)
		}

		if actualLeadTime <= quotedLeadTime {
			onTimePOs++
		} else {
			latePOs++
			totalLateness += (actualLeadTime - quotedLeadTime)
		}
	}

	// Calculate averages
	avgActualLeadTime := 0
	avgLateness := 0
	onTimeRate := 0.0
	variancePercent := 0.0

	if totalPOs > 0 {
		avgActualLeadTime = totalLeadTime / totalPOs
		onTimeRate = (float64(onTimePOs) / float64(totalPOs)) * 100

		if latePOs > 0 {
			avgLateness = totalLateness / latePOs
		}

		if supplier.LeadTimeDays > 0 {
			variancePercent = ((float64(avgActualLeadTime) - float64(supplier.LeadTimeDays)) / float64(supplier.LeadTimeDays)) * 100
		}
	}

	// Assign performance grade based on on-time rate
	performanceGrade := "D"
	if onTimeRate >= 95 {
		performanceGrade = "A" // Excellent (>= 95%)
	} else if onTimeRate >= 85 {
		performanceGrade = "B" // Good (85-94%)
	} else if onTimeRate >= 70 {
		performanceGrade = "C" // Acceptable (70-84%)
	}
	// D = Poor (< 70%)

	metrics := SupplierLeadTimeMetrics{
		SupplierID:         supplierID,
		SupplierName:       supplier.SupplierName,
		QuotedLeadTimeDays: supplier.LeadTimeDays,
		ActualLeadTimeDays: avgActualLeadTime,
		LeadTimeVariance:   avgActualLeadTime - supplier.LeadTimeDays,
		VariancePercent:    variancePercent,
		TotalPOs:           totalPOs,
		OnTimePOs:          onTimePOs,
		LatePOs:            latePOs,
		OnTimeRate:         onTimeRate,
		PerformanceGrade:   performanceGrade,
		AverageLatenessDays: avgLateness,
	}

	log.Printf("📊 Supplier %s lead time metrics: Actual=%d days, Quoted=%d days, On-time rate=%.1f%%, Grade=%s",
		supplier.SupplierName, avgActualLeadTime, supplier.LeadTimeDays, onTimeRate, performanceGrade)

	return metrics, nil
}

// GetSupplierLeadTimeReport generates lead time report for all active suppliers
func (a *App) GetSupplierLeadTimeReport() ([]SupplierLeadTimeMetrics, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get all active suppliers
	var suppliers []SupplierMaster
	if err := a.db.Find(&suppliers).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve suppliers", err.Error())
	}

	// Calculate metrics for each
	report := make([]SupplierLeadTimeMetrics, 0)
	for _, supplier := range suppliers {
		metrics, err := a.CalculateSupplierLeadTime(supplier.ID)
		if err != nil {
			log.Printf("⚠️ Failed to calculate metrics for supplier %s: %v", supplier.SupplierName, err)
			continue
		}

		// Only include suppliers with actual data
		if metrics.TotalPOs > 0 {
			report = append(report, metrics)
		}
	}

	log.Printf("📈 Generated lead time report for %d suppliers", len(report))
	return report, nil
}

// GetLateSuppliers retrieves suppliers consistently late on deliveries
func (a *App) GetLateSuppliers() ([]SupplierLeadTimeMetrics, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_QUERY_FAILED", "Database connection not available", "")
	}

	// Get full report
	report, err := a.GetSupplierLeadTimeReport()
	if err != nil {
		return nil, err
	}

	// Filter to late suppliers (on-time rate < 70% or grade D)
	lateSuppliers := make([]SupplierLeadTimeMetrics, 0)
	for _, metrics := range report {
		if metrics.OnTimeRate < 70.0 || metrics.PerformanceGrade == "D" {
			lateSuppliers = append(lateSuppliers, metrics)
		}
	}

	log.Printf("⚠️ Found %d suppliers with poor delivery performance (<70%% on-time)", len(lateSuppliers))
	return lateSuppliers, nil
}
