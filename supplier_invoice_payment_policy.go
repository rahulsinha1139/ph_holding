package main

import (
	"math"
	"time"

	"gorm.io/gorm"
)

type supplierInvoicePaymentState struct {
	InvoiceID         string
	TotalPaidBHD      float64
	OutstandingBHD    float64
	PaymentStatus     string
	InvoiceStatus     string
	LatestPaymentDate *time.Time
	LatestPaymentRef  string
	LatestPaymentType string
}

type supplierInvoicePaymentAggregate struct {
	SupplierInvoiceID string
	TotalPaidBHD      float64
}

type supplierInvoicePaymentTotals struct {
	AmountBHD     float64
	AmountForeign float64
}

func supplierInvoiceOutstandingBHD(invoice SupplierInvoice, totalPaidBHD float64) float64 {
	outstanding := invoice.TotalBHD - totalPaidBHD
	if math.Abs(outstanding) <= FloatingPointTolerance {
		return 0
	}
	if outstanding < 0 {
		return 0
	}
	return outstanding
}

func supplierInvoiceNonPaymentStatus(invoice SupplierInvoice) string {
	switch invoice.Status {
	case "Rejected", "Dispute":
		return invoice.Status
	case "Approved", "Paid":
		return "Approved"
	default:
		if invoice.ApprovedAt != nil || invoice.ApprovedBy != "" {
			return "Approved"
		}
		return "Pending"
	}
}

func supplierInvoicePaymentStateFromLedger(invoice SupplierInvoice, totalPaidBHD float64, latestPayment *SupplierPayment) supplierInvoicePaymentState {
	outstanding := supplierInvoiceOutstandingBHD(invoice, totalPaidBHD)

	state := supplierInvoicePaymentState{
		InvoiceID:      invoice.ID,
		TotalPaidBHD:   totalPaidBHD,
		OutstandingBHD: outstanding,
		InvoiceStatus:  supplierInvoiceNonPaymentStatus(invoice),
		PaymentStatus:  "Unpaid",
	}

	if totalPaidBHD > FloatingPointTolerance {
		state.PaymentStatus = "Partial"
	}
	if outstanding == 0 {
		state.PaymentStatus = "Paid"
	}

	switch state.InvoiceStatus {
	case "Rejected", "Dispute":
		// Preserve exceptional workflow states even if legacy data contains payments.
	default:
		if state.PaymentStatus == "Paid" {
			state.InvoiceStatus = "Paid"
		}
	}

	if latestPayment != nil {
		paymentDate := latestPayment.PaymentDate
		state.LatestPaymentDate = &paymentDate
		state.LatestPaymentRef = latestPayment.Reference
		state.LatestPaymentType = latestPayment.PaymentMethod
	}

	return state
}

func (a *App) getSupplierInvoicePaymentTotals(tx *gorm.DB, invoiceID string) (supplierInvoicePaymentTotals, error) {
	var totals supplierInvoicePaymentTotals
	err := tx.Model(&SupplierPayment{}).
		Where("supplier_invoice_id = ?", invoiceID).
		Select("COALESCE(SUM(amount_bhd), 0) AS amount_bhd, COALESCE(SUM(amount_foreign), 0) AS amount_foreign").
		Scan(&totals).Error
	return totals, err
}

func (a *App) loadSupplierInvoicePaymentState(tx *gorm.DB, invoice SupplierInvoice) (supplierInvoicePaymentState, error) {
	totals, err := a.getSupplierInvoicePaymentTotals(tx, invoice.ID)
	if err != nil {
		return supplierInvoicePaymentState{}, err
	}

	var latestPayment SupplierPayment
	var latest *SupplierPayment
	if err := tx.Where("supplier_invoice_id = ?", invoice.ID).
		Order("payment_date DESC").
		Order("updated_at DESC").
		First(&latestPayment).Error; err == nil {
		latest = &latestPayment
	} else if err != gorm.ErrRecordNotFound {
		return supplierInvoicePaymentState{}, err
	}

	return supplierInvoicePaymentStateFromLedger(invoice, totals.AmountBHD, latest), nil
}

func (a *App) applySupplierInvoicePaymentState(tx *gorm.DB, invoiceID string) (supplierInvoicePaymentState, error) {
	var invoice SupplierInvoice
	if err := tx.Where("id = ?", invoiceID).First(&invoice).Error; err != nil {
		return supplierInvoicePaymentState{}, err
	}

	state, err := a.loadSupplierInvoicePaymentState(tx, invoice)
	if err != nil {
		return supplierInvoicePaymentState{}, err
	}

	updates := map[string]interface{}{
		"payment_status": state.PaymentStatus,
		"status":         state.InvoiceStatus,
		"updated_at":     time.Now(),
	}

	if state.LatestPaymentDate != nil {
		updates["payment_date"] = *state.LatestPaymentDate
		updates["payment_ref"] = state.LatestPaymentRef
		updates["payment_method"] = state.LatestPaymentType
	} else {
		updates["payment_date"] = nil
		updates["payment_ref"] = ""
		updates["payment_method"] = ""
	}

	if err := tx.Model(&SupplierInvoice{}).Where("id = ?", invoiceID).Updates(updates).Error; err != nil {
		return supplierInvoicePaymentState{}, err
	}

	return state, nil
}

func (a *App) buildSupplierInvoicePaymentStateMap(tx *gorm.DB, invoices []SupplierInvoice) (map[string]supplierInvoicePaymentState, error) {
	states := make(map[string]supplierInvoicePaymentState, len(invoices))
	if len(invoices) == 0 {
		return states, nil
	}

	ids := make([]string, 0, len(invoices))
	for _, invoice := range invoices {
		ids = append(ids, invoice.ID)
	}

	var aggregates []supplierInvoicePaymentAggregate
	if err := tx.Model(&SupplierPayment{}).
		Where("supplier_invoice_id IN ?", ids).
		Select("supplier_invoice_id, COALESCE(SUM(amount_bhd), 0) AS total_paid_bhd").
		Group("supplier_invoice_id").
		Scan(&aggregates).Error; err != nil {
		return nil, err
	}

	paidByInvoiceID := make(map[string]float64, len(aggregates))
	for _, aggregate := range aggregates {
		paidByInvoiceID[aggregate.SupplierInvoiceID] = aggregate.TotalPaidBHD
	}

	for _, invoice := range invoices {
		states[invoice.ID] = supplierInvoicePaymentStateFromLedger(invoice, paidByInvoiceID[invoice.ID], nil)
	}

	return states, nil
}

func (a *App) createRemainingSupplierInvoicePayment(tx *gorm.DB, invoice SupplierInvoice, paymentDate time.Time, paymentMethod string, paymentRef string, notes string) (bool, error) {
	totals, err := a.getSupplierInvoicePaymentTotals(tx, invoice.ID)
	if err != nil {
		return false, err
	}

	remainingBHD := supplierInvoiceOutstandingBHD(invoice, totals.AmountBHD)
	if remainingBHD <= FloatingPointTolerance {
		return false, nil
	}

	currency := invoice.Currency
	if currency == "" {
		currency = "BHD"
	}

	exchangeRate := invoice.ExchangeRate
	if exchangeRate <= 0 {
		exchangeRate = 1
	}

	amountForeign := remainingBHD
	if currency != "BHD" {
		remainingForeign := invoice.TotalForeign - totals.AmountForeign
		if remainingForeign > FloatingPointTolerance {
			amountForeign = remainingForeign
		} else {
			amountForeign = remainingBHD / exchangeRate
		}
	}

	if paymentMethod == "" {
		paymentMethod = "Other"
	}

	payment := SupplierPayment{
		SupplierInvoiceID: invoice.ID,
		SupplierID:        invoice.SupplierID,
		AmountForeign:     amountForeign,
		Currency:          currency,
		ExchangeRate:      exchangeRate,
		AmountBHD:         remainingBHD,
		PaymentDate:       paymentDate,
		PaymentMethod:     paymentMethod,
		Reference:         paymentRef,
		Notes:             notes,
		SupplierName:      invoice.SupplierName,
		InvoiceNumber:     invoice.InvoiceNumber,
	}

	if err := tx.Create(&payment).Error; err != nil {
		return false, err
	}

	return true, nil
}
