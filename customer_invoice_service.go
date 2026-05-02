package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// hmacSaltWarningOnce ensures the missing salt file warning is logged only once
var hmacSaltWarningOnce sync.Once

// computeDocumentHMAC generates an HMAC-SHA256 hash for invoice integrity verification
// Uses globalFieldCrypto.salt as the secret key (avoids disk read + path mismatch issues)
// Falls back to plain SHA-256 if FieldCrypto is not initialized (first-run scenario)
// I1 FIX: Missing salt warning logged only once via sync.Once
func computeDocumentHMAC(invoiceNumber, date string, grandTotal, vat float64) string {
	hashInput := invoiceNumber + "|" + date + "|" + fmt.Sprintf("%.3f", grandTotal) + "|" + fmt.Sprintf("%.3f", vat)

	// Use globalFieldCrypto.salt directly — avoids EvalSymlinks/path mismatch issues
	if globalFieldCrypto != nil {
		globalFieldCrypto.mu.RLock()
		salt := globalFieldCrypto.salt
		globalFieldCrypto.mu.RUnlock()
		if len(salt) > 0 {
			mac := hmac.New(sha256.New, salt)
			mac.Write([]byte(hashInput))
			return hex.EncodeToString(mac.Sum(nil))
		}
	}

	// Fallback to plain SHA-256 if FieldCrypto not available (first-run scenario)
	hmacSaltWarningOnce.Do(func() {
		log.Printf("WARNING: globalFieldCrypto not available — using plain SHA-256 for document hash. Document integrity verification is degraded.")
	})
	hash := sha256.Sum256([]byte(hashInput))
	return hex.EncodeToString(hash[:])
}

func roundInvoiceMoney(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func normalizeOrderItemForInvoice(item OrderItem) (OrderItem, bool) {
	if isSyntheticCommercialSummary(item.Description, item.ProductCode, item.Model, item.Equipment) {
		return item, false
	}

	item.Quantity = roundInvoiceMoney(item.Quantity)
	item.UnitPrice = roundInvoiceMoney(item.UnitPrice)
	item.TotalPrice = roundInvoiceMoney(item.TotalPrice)

	if item.UnitPrice <= 0 && item.Quantity > 0 && item.TotalPrice > 0 {
		item.UnitPrice = roundInvoiceMoney(item.TotalPrice / item.Quantity)
	}
	if item.TotalPrice <= 0 && item.Quantity > 0 && item.UnitPrice > 0 {
		item.TotalPrice = roundInvoiceMoney(item.Quantity * item.UnitPrice)
	}

	item.Description = strings.TrimSpace(item.Description)
	if item.Description == "" {
		switch {
		case strings.TrimSpace(item.Equipment) != "" && strings.TrimSpace(item.Model) != "":
			item.Description = fmt.Sprintf("%s - %s", strings.TrimSpace(item.Equipment), strings.TrimSpace(item.Model))
		case strings.TrimSpace(item.Equipment) != "":
			item.Description = strings.TrimSpace(item.Equipment)
		case strings.TrimSpace(item.Model) != "":
			item.Description = strings.TrimSpace(item.Model)
		case strings.TrimSpace(item.ProductCode) != "":
			item.Description = strings.TrimSpace(item.ProductCode)
		}
	}

	hasCommercialIdentity := item.Description != "" ||
		strings.TrimSpace(item.ProductCode) != "" ||
		strings.TrimSpace(item.Equipment) != "" ||
		strings.TrimSpace(item.Model) != ""

	if !hasCommercialIdentity {
		return item, false
	}
	if item.Quantity <= 0 {
		return item, false
	}
	if item.UnitPrice <= 0 && item.TotalPrice <= 0 {
		return item, false
	}

	return item, true
}

func invoiceOrderItemsFromOrder(order Order) []OrderItem {
	targetTotal := roundInvoiceMoney(order.TotalValueBHD)
	if targetTotal <= 0 {
		targetTotal = roundInvoiceMoney(order.GrandTotalBHD)
	}

	normalized := make([]OrderItem, 0, len(order.Items))
	seen := make(map[string]struct{})
	for _, item := range order.Items {
		clean, ok := normalizeOrderItemForInvoice(item)
		if !ok {
			continue
		}
		signature := strings.Join([]string{
			strings.ToLower(strings.TrimSpace(clean.ProductCode)),
			strings.ToLower(strings.TrimSpace(clean.Description)),
			fmt.Sprintf("%.3f", clean.Quantity),
			fmt.Sprintf("%.3f", clean.UnitPrice),
			fmt.Sprintf("%.3f", clean.TotalPrice),
		}, "|")
		if _, exists := seen[signature]; exists {
			continue
		}
		seen[signature] = struct{}{}
		normalized = append(normalized, clean)
	}

	if len(normalized) == 0 {
		return nil
	}

	if targetTotal > 0 {
		for _, item := range normalized {
			if math.Abs(roundInvoiceMoney(item.TotalPrice)-targetTotal) <= 0.01 {
				return []OrderItem{item}
			}
		}
	}

	sum := 0.0
	for _, item := range normalized {
		sum += item.TotalPrice
	}
	if targetTotal > 0 && math.Abs(roundInvoiceMoney(sum)-targetTotal) <= 0.01 {
		return normalized
	}

	return normalized
}

func defaultInvoiceFieldVisibilityJSON() string {
	return `{"show_fob":false,"show_freight":false,"show_margin":false,"show_cost":false,"show_contact":true,"show_rfq":true,"show_equipment":true,"show_specification":true,"show_detailed_desc":true,"show_country_origin":true,"show_delivery_weeks":true}`
}

func (a *App) BackfillInvoiceItemsFromOrders() (map[string]interface{}, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	type invoiceShell struct {
		ID string
	}

	var targets []invoiceShell
	if err := a.db.Raw(`
			SELECT i.id
			FROM invoices i
			LEFT JOIN invoice_items ii ON ii.invoice_id = i.id AND ii.deleted_at IS NULL
			WHERE i.deleted_at IS NULL
			  AND COALESCE(i.order_id, '') != ''
			GROUP BY i.id
			HAVING COUNT(ii.id) = 0
		`).Scan(&targets).Error; err != nil {
		return nil, fmt.Errorf("failed to find invoices missing items: %w", err)
	}

	repaired := 0
	skipped := 0
	examples := make([]string, 0, 5)

	for _, shell := range targets {
		var invoice Invoice
		if err := a.db.First(&invoice, "id = ?", shell.ID).Error; err != nil {
			return nil, fmt.Errorf("failed to load invoice %s: %w", shell.ID, err)
		}

		var order Order
		if err := a.db.Preload("Items").First(&order, "id = ?", invoice.OrderID).Error; err != nil {
			skipped++
			continue
		}

		items := invoiceOrderItemsFromOrder(order)
		if len(items) == 0 {
			fallbackAmount := roundInvoiceMoney(invoice.SubtotalBHD)
			if fallbackAmount <= 0 {
				fallbackAmount = roundInvoiceMoney(order.TotalValueBHD)
			}
			if fallbackAmount <= 0 {
				fallbackAmount = roundInvoiceMoney(invoice.GrandTotalBHD)
			}
			if fallbackAmount <= 0 {
				skipped++
				continue
			}
			items = []OrderItem{{
				Base:        Base{ID: uuid.New().String()},
				OrderID:     order.ID,
				LineNumber:  1,
				Description: fmt.Sprintf("Per Order %s", order.OrderNumber),
				Quantity:    1,
				UnitPrice:   fallbackAmount,
				TotalPrice:  fallbackAmount,
			}}
		}

		invoiceItems := make([]DBInvoiceItem, 0, len(items))
		totalSupplierCost := 0.0
		subtotal := 0.0
		for index, item := range items {
			lineTotal := roundInvoiceMoney(item.TotalPrice)
			if lineTotal <= 0 && item.Quantity > 0 && item.UnitPrice > 0 {
				lineTotal = roundInvoiceMoney(item.Quantity * item.UnitPrice)
			}
			unitPrice := roundInvoiceMoney(item.UnitPrice)
			if unitPrice <= 0 && item.Quantity > 0 && lineTotal > 0 {
				unitPrice = roundInvoiceMoney(lineTotal / item.Quantity)
			}
			if lineTotal <= 0 {
				continue
			}
			supplierCost := roundInvoiceMoney(item.TotalCost)
			totalSupplierCost += supplierCost
			subtotal += lineTotal
			invoiceItems = append(invoiceItems, DBInvoiceItem{
				Base:                Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
				InvoiceID:           invoice.ID,
				LineNumber:          index + 1,
				Description:         item.Description,
				Quantity:            item.Quantity,
				Rate:                unitPrice,
				TotalBHD:            lineTotal,
				ProductID:           item.ProductID,
				ProductCode:         item.ProductCode,
				Equipment:           item.Equipment,
				Model:               item.Model,
				Specification:       item.Specification,
				DetailedDescription: item.DetailedDescription,
				Currency:            item.Currency,
				FOB:                 item.FOB,
				Freight:             item.Freight,
				TotalCost:           item.TotalCost,
				MarginPercent:       item.MarginPercent,
				TotalPrice:          lineTotal,
			})
		}

		if len(invoiceItems) == 0 {
			skipped++
			continue
		}

		currentSubtotal := roundInvoiceMoney(invoice.SubtotalBHD)
		if currentSubtotal <= 0 {
			currentSubtotal = roundInvoiceMoney(subtotal)
		}

		updates := map[string]interface{}{
			"rfq_id":                  order.RFQID,
			"offer_id":                order.OfferID,
			"offer_number":            order.OfferNumber,
			"customer_reference":      order.CustomerReference,
			"attention_person":        order.AttentionPerson,
			"attention_company":       order.AttentionCompany,
			"attention_phone":         order.AttentionPhone,
			"attention_address":       order.AttentionAddress,
			"delivery_weeks":          order.DeliveryWeeks,
			"country_of_origin":       order.CountryOfOrigin,
			"issued_by":               order.IssuedBy,
			"contact_phone":           order.ContactPhone,
			"discount_percent":        order.DiscountPercent,
			"payment_terms":           order.PaymentTerms,
			"delivery_terms":          order.DeliveryTerms,
			"total_supplier_cost_bhd": roundInvoiceMoney(totalSupplierCost),
			"gross_margin_bhd":        roundInvoiceMoney(currentSubtotal - totalSupplierCost),
		}
		if invoice.SubtotalBHD <= 0 {
			updates["subtotal_bhd"] = currentSubtotal
		}
		if currentSubtotal > 0 {
			updates["gross_margin_percent"] = roundInvoiceMoney(((currentSubtotal - totalSupplierCost) / currentSubtotal) * 100)
		}
		if strings.TrimSpace(invoice.FieldVisibility) == "" {
			updates["field_visibility"] = defaultInvoiceFieldVisibilityJSON()
		}

		if err := a.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("invoice_id = ?", invoice.ID).Delete(&DBInvoiceItem{}).Error; err != nil {
				return fmt.Errorf("failed to clear stale invoice items for %s: %w", invoice.InvoiceNumber, err)
			}
			if err := tx.Create(&invoiceItems).Error; err != nil {
				return fmt.Errorf("failed to create invoice items for %s: %w", invoice.InvoiceNumber, err)
			}
			if err := tx.Model(&Invoice{}).Where("id = ?", invoice.ID).Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to update invoice header for %s: %w", invoice.InvoiceNumber, err)
			}
			return nil
		}); err != nil {
			return nil, err
		}

		repaired++
		if len(examples) < 5 {
			examples = append(examples, invoice.InvoiceNumber)
		}
	}

	return map[string]interface{}{
		"attempted": len(targets),
		"repaired":  repaired,
		"skipped":   skipped,
		"examples":  examples,
	}, nil
}

// =============================================================================
// CUSTOMER INVOICE CRUD - Complete Invoice Lifecycle Management
// =============================================================================
//
// FEATURES:
//   - Invoice generation from customer orders
//   - Automatic invoice numbering (INV-YYYYMMDD-NNNN)
//   - Payment terms parsing and due date calculation
//   - Status workflow (Draft → Sent → Paid → Overdue)
//   - Order-to-Invoice traceability with quantity tracking
//   - Multi-item invoice support with margin tracking
//   - Outstanding balance management
//   - VAT calculation (10% Bahrain rate)
//   - BHD precision (3 decimal places)
//
// Built with MATHEMATICAL RIGOR × PRODUCTION ROBUSTNESS
// Day 196+ - Finance Hub Phase 2
// =============================================================================

// CreateInvoiceFromOrder creates a customer invoice from an existing order
// Maps all order fields, generates invoice number, calculates due date, creates line items
// Updates OrderItem.QuantityInvoiced for fulfillment tracking
func (a *App) CreateInvoiceFromOrder(orderID string) (Invoice, error) {
	log.Printf("📋 CreateInvoiceFromOrder: Starting for orderID=%s", orderID)

	if err := a.requirePermission("invoices:create"); err != nil {
		log.Printf("❌ CreateInvoiceFromOrder: Permission denied: %v", err)
		return Invoice{}, err
	}
	log.Printf("✅ CreateInvoiceFromOrder: Permission granted, calling CreateInvoiceWithOptions")

	invoice, err := a.CreateInvoiceWithOptions(orderID, "", "")
	if err != nil {
		log.Printf("❌ CreateInvoiceFromOrder: CreateInvoiceWithOptions failed: %v", err)
		return Invoice{}, err
	}
	log.Printf("✅ CreateInvoiceFromOrder: Invoice created successfully: %s", invoice.InvoiceNumber)
	return invoice, nil
}

// CreateProformaInvoice creates a proforma invoice from an order
// Proforma invoices are not tax documents - they serve as reference/quotation documents
// Status is set to "Proforma" and the invoice is NOT counted toward order fulfillment
func (a *App) CreateProformaInvoice(orderID string) (Invoice, error) {
	log.Printf("📋 CreateProformaInvoice: Starting for orderID=%s", orderID)

	if err := a.requirePermission("invoices:create"); err != nil {
		return Invoice{}, err
	}
	if a.db == nil {
		return Invoice{}, fmt.Errorf("database not initialized")
	}

	// Load order with items
	var order Order
	if err := a.db.Preload("Items").Where("id = ?", orderID).First(&order).Error; err != nil {
		return Invoice{}, fmt.Errorf("order not found: %w", err)
	}

	// Generate invoice number
	invoiceNumber, err := a.GenerateInvoiceNumber()
	if err != nil {
		return Invoice{}, fmt.Errorf("failed to generate invoice number: %w", err)
	}

	// Build proforma invoice directly (skip fulfillment tracking)
	invoice := Invoice{
		InvoiceNumber:  invoiceNumber,
		OrderID:        order.ID,
		CustomerID:     order.CustomerID,
		CustomerName:   order.CustomerName,
		InvoiceDate:    time.Now(),
		DueDate:        time.Now().AddDate(0, 0, 30),
		Status:         "Proforma",
		SubtotalBHD:    order.TotalValueBHD,
		VATPercent:     10.0,
		VATBHD:         order.TotalValueBHD * 0.10,
		GrandTotalBHD:  order.TotalValueBHD * 1.10,
		OutstandingBHD: 0.0, // Proforma has no outstanding balance
		PaymentTerms:   order.PaymentTerms,
		Division:       normalizeDivisionName(order.Division),
	}

	// Copy line items from order, including the same costing/specification fields used by tax invoices.
	for index, orderItem := range order.Items {
		lineNumber := orderItem.LineNumber
		if lineNumber == 0 {
			lineNumber = index + 1
		}
		invoice.Items = append(invoice.Items, DBInvoiceItem{
			LineNumber:          lineNumber,
			Description:         orderItem.Description,
			Quantity:            orderItem.Quantity,
			Rate:                orderItem.UnitPrice,
			TotalBHD:            orderItem.TotalPrice,
			ProductID:           orderItem.ProductID,
			ProductCode:         orderItem.ProductCode,
			Equipment:           orderItem.Equipment,
			Model:               orderItem.Model,
			Specification:       orderItem.Specification,
			DetailedDescription: orderItem.DetailedDescription,
			Currency:            orderItem.Currency,
			FOB:                 orderItem.FOB,
			Freight:             orderItem.Freight,
			TotalCost:           orderItem.TotalCost,
			MarginPercent:       orderItem.MarginPercent,
			TotalPrice:          orderItem.TotalPrice,
		})
	}

	if err := a.db.Create(&invoice).Error; err != nil {
		return Invoice{}, fmt.Errorf("failed to create proforma invoice: %w", err)
	}

	// NOTE: No QuantityInvoiced update, no order status progression
	// Proforma is a reference document only

	log.Printf("✅ Created Proforma Invoice: %s (Customer: %s, Order: %s, Total: %.3f BHD)",
		invoice.InvoiceNumber, invoice.CustomerName, orderID, invoice.GrandTotalBHD)

	return invoice, nil
}

// CreateInvoiceFromOrderWithDN creates invoice with optional DN linkage
func (a *App) CreateInvoiceFromOrderWithDN(orderID string, deliveryNoteID string) (Invoice, error) {
	if err := a.requirePermission("invoices:create"); err != nil {
		return Invoice{}, err
	}
	return a.CreateInvoiceWithOptions(orderID, deliveryNoteID, "")
}

// CreateInvoiceFromDN creates an invoice directly from a delivery note
// Resolves the order from the DN and passes through to CreateInvoiceWithOptions
func (a *App) CreateInvoiceFromDN(deliveryNoteID string) (Invoice, error) {
	if err := a.requirePermission("invoices:create"); err != nil {
		return Invoice{}, err
	}
	if a.db == nil {
		return Invoice{}, fmt.Errorf("database not initialized")
	}

	// Load DN to get the order ID
	var dn DeliveryNote
	if err := a.db.Where("id = ?", deliveryNoteID).First(&dn).Error; err != nil {
		return Invoice{}, fmt.Errorf("delivery note not found: %w", err)
	}
	if dn.OrderID == "" {
		return Invoice{}, fmt.Errorf("delivery note %s is not linked to an order", dn.DNNumber)
	}

	log.Printf("📋 CreateInvoiceFromDN: DN=%s -> Order=%s", dn.DNNumber, dn.OrderID)
	return a.CreateInvoiceWithOptions(dn.OrderID, deliveryNoteID, "")
}

// CreateInvoiceWithOptions creates invoice with DN linkage and field visibility settings
// fieldVisibility is a JSON string like: {"show_fob":false,"show_freight":false,...}
func (a *App) CreateInvoiceWithOptions(orderID string, deliveryNoteID string, fieldVisibilityJSON string) (Invoice, error) {
	log.Printf("📋 CreateInvoiceWithOptions: orderID=%s, dnID=%s", orderID, deliveryNoteID)

	if err := a.requirePermission("invoices:create"); err != nil {
		log.Printf("❌ CreateInvoiceWithOptions: Permission denied: %v", err)
		return Invoice{}, err
	}
	if a.db == nil {
		log.Printf("❌ CreateInvoiceWithOptions: Database not initialized")
		return Invoice{}, fmt.Errorf("database not initialized")
	}

	// Load order with items preloaded
	log.Printf("📋 CreateInvoiceWithOptions: Loading order from database...")
	var order Order
	if err := a.db.Preload("Items").Where("id = ?", orderID).First(&order).Error; err != nil {
		log.Printf("❌ CreateInvoiceWithOptions: Order not found: %v", err)
		return Invoice{}, fmt.Errorf("order not found: %w", err)
	}
	log.Printf("✅ CreateInvoiceWithOptions: Loaded order %s with %d items, Total: %.3f BHD", order.OrderNumber, len(order.Items), order.GrandTotalBHD)

	order.Items = invoiceOrderItemsFromOrder(order)

	// FIXED: Handle orders without items (e.g., Tally imports) by creating a single line item
	// B1 FIX: Synthetic item is added to memory here but persisted inside the transaction below
	var syntheticItem *OrderItem
	if len(order.Items) == 0 {
		log.Printf("⚠️ Order %s has no items - creating single line item from order total", order.OrderNumber)
		// Create synthetic order item from order total (in-memory only; DB persist inside tx)
		item := OrderItem{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OrderID:     order.ID,
			LineNumber:  1,
			Description: fmt.Sprintf("Per Order %s", order.OrderNumber),
			Quantity:    1,
			UnitPrice:   order.GrandTotalBHD,
			TotalPrice:  order.GrandTotalBHD,
		}
		order.Items = []OrderItem{item}
		syntheticItem = &item
	}

	// Check if order has delivery notes (determines invoicing mode)
	var dnCount int64
	a.db.Model(&DeliveryNote{}).Where("order_id = ?", orderID).Count(&dnCount)
	hasDNs := dnCount > 0
	log.Printf("📋 CreateInvoiceWithOptions: Order has %d delivery notes (DN-based=%v)", dnCount, hasDNs)

	// Duplicate/invoiceable check depends on whether order uses delivery notes
	log.Printf("📋 CreateInvoiceWithOptions: Checking invoiceability...")
	var existingCount int64
	if err := a.db.Model(&Invoice{}).Where("order_id = ?", orderID).Count(&existingCount).Error; err != nil {
		log.Printf("❌ CreateInvoiceWithOptions: Failed to check existing invoices: %v", err)
		return Invoice{}, fmt.Errorf("failed to check existing invoices: %w", err)
	}

	if hasDNs {
		// DN-based flow: multiple invoices per order allowed (one per delivery)
		// Check that there are delivered-but-uninvoiced quantities remaining
		log.Printf("📦 DN-based invoicing: checking for uninvoiced delivered quantities")
		hasInvoiceable := false
		for _, item := range order.Items {
			uninvoiced := item.Quantity - item.QuantityInvoiced
			if uninvoiced > 0.001 {
				hasInvoiceable = true
				break
			}
		}
		if !hasInvoiceable {
			log.Printf("❌ CreateInvoiceWithOptions: All order items are fully invoiced")
			return Invoice{}, fmt.Errorf("all items in this order are already fully invoiced")
		}
		log.Printf("✅ DN-based invoicing: uninvoiced quantities found, proceeding")
	} else {
		// Legacy flow (no DNs): only one invoice per order allowed
		if existingCount > 0 {
			log.Printf("❌ CreateInvoiceWithOptions: Order already has %d invoice(s) (no DNs)", existingCount)
			return Invoice{}, fmt.Errorf("order already has %d invoice(s) - cannot create duplicate (use delivery notes for partial invoicing)", existingCount)
		}
		log.Printf("✅ CreateInvoiceWithOptions: No duplicate invoices found (legacy mode)")
	}

	// P0 FIX: Credit limit enforcement — check happens atomically inside the invoice creation
	// transaction below (Fix 1: TOCTOU gap closed). Preparation: just log intent here.
	log.Printf("📋 CreateInvoiceWithOptions: Credit check will run atomically with invoice creation for customer_id=%s", order.CustomerID)

	// Load delivery note if provided
	var deliveryNote DeliveryNote
	var dnItems []DeliveryNoteItem
	var dnNumber string
	var deliveryNoteDate *time.Time
	if deliveryNoteID != "" {
		if err := a.db.Preload("Items").Where("id = ?", deliveryNoteID).First(&deliveryNote).Error; err != nil {
			log.Printf("❌ Delivery note %s not found for invoice creation: %v", deliveryNoteID, err)
			return Invoice{}, fmt.Errorf("delivery note not found: %w", err)
		}
		if deliveryNote.OrderID != orderID {
			return Invoice{}, fmt.Errorf("delivery note %s does not belong to the selected order", deliveryNote.DNNumber)
		}
		var linkedInvoiceCount int64
		if err := a.db.Model(&Invoice{}).
			Where("delivery_note_id = ? AND status NOT IN ?", deliveryNoteID, []string{"Cancelled", "Void"}).
			Count(&linkedInvoiceCount).Error; err != nil {
			return Invoice{}, fmt.Errorf("failed to check delivery note invoice linkage: %w", err)
		}
		if linkedInvoiceCount > 0 {
			return Invoice{}, fmt.Errorf("delivery note %s has already been linked to an invoice", deliveryNote.DNNumber)
		}
		dnNumber = deliveryNote.DNNumber
		dnItems = deliveryNote.Items
		if !deliveryNote.DeliveryDate.IsZero() {
			dt := deliveryNote.DeliveryDate
			deliveryNoteDate = &dt
		}
		log.Printf("📦 Linking invoice to DN: %s (%d items)", dnNumber, len(dnItems))
	}

	// Generate invoice number
	log.Printf("📋 CreateInvoiceWithOptions: Generating invoice number...")
	invoiceNumber, err := a.GenerateInvoiceNumber()
	if err != nil {
		log.Printf("❌ CreateInvoiceWithOptions: Failed to generate invoice number: %v", err)
		return Invoice{}, fmt.Errorf("failed to generate invoice number: %w", err)
	}
	log.Printf("✅ CreateInvoiceWithOptions: Generated invoice number: %s", invoiceNumber)

	// Parse payment terms to calculate due date (e.g., "Net 30" → 30 days)
	dueDate := calculateDueDate(time.Now(), order.PaymentTerms)

	// Create invoice header - the costing sheet becomes the invoice
	now := time.Now()
	invoice := Invoice{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: now,
			UpdatedAt: now,
		},
		InvoiceNumber:      invoiceNumber,
		InvoiceDate:        now,
		CustomerID:         order.CustomerID,
		CustomerName:       order.CustomerName,
		OrderID:            order.ID,
		CustomerPONumber:   order.CustomerPONumber,
		GrandTotalBHD:      order.GrandTotalBHD,
		SubtotalBHD:        order.TotalValueBHD,
		Status:             "Draft",
		OutstandingBHD:     order.GrandTotalBHD, // Full amount outstanding initially
		DueDate:            dueDate,
		DeliveryNoteID:     deliveryNoteID,
		DeliveryNoteNumber: dnNumber,
		DeliveryNoteRef:    dnNumber,
		DeliveryNoteDate:   deliveryNoteDate,
		DespatchDocumentNo: dnNumber,
		ModeOfPayment:      firstNonEmptyString(order.PaymentTerms, "Direct Bank Transfer"),
		BuyersOrderNumber:  order.CustomerPONumber,
		TermsOfDelivery:    firstNonEmptyString(order.DeliveryTerms, "Direct Bank Transfer"),
		// Traceability links
		RfqID:       order.RFQID,
		OfferID:     order.OfferID,
		OfferNumber: order.OfferNumber,
		// Contact & RFQ details (copied from Order, originally from Offer/Costing Sheet)
		CustomerReference: order.CustomerReference,
		AttentionPerson:   order.AttentionPerson,
		AttentionCompany:  order.AttentionCompany,
		AttentionPhone:    order.AttentionPhone,
		AttentionAddress:  order.AttentionAddress,
		DeliveryWeeks:     order.DeliveryWeeks,
		CountryOfOrigin:   order.CountryOfOrigin,
		IssuedBy:          order.IssuedBy,
		ContactPhone:      order.ContactPhone,
		DiscountPercent:   order.DiscountPercent,
		PaymentTerms:      order.PaymentTerms,
		DeliveryTerms:     order.DeliveryTerms,
		Division:          normalizeDivisionName(order.Division),
		// Margin fields - will be calculated from items
		TotalSupplierCostBHD: 0.0,
		GrossMarginBHD:       0.0,
		GrossMarginPercent:   0.0,
		// Field visibility - use provided or default
		FieldVisibility: func() string {
			if fieldVisibilityJSON != "" {
				return fieldVisibilityJSON
			}
			return defaultInvoiceFieldVisibilityJSON()
		}(),
		Items: []DBInvoiceItem{},
	}

	// Create invoice items - logic depends on whether order has delivery notes
	// DN-based: use delivered quantities (from DN items or uninvoiced remainder)
	// Legacy: use full order quantities
	var totalSupplierCost float64
	var invoiceSubtotal float64
	lineNum := 0

	// Track actual invoiced quantity per order item ID for QuantityInvoiced update
	invoicedQtyByOrderItemID := make(map[string]float64)

	// Build a map of DN item quantities keyed by order_item_id for DN-based invoicing
	dnQtyByOrderItem := make(map[string]float64)
	if hasDNs && len(dnItems) > 0 {
		for _, di := range dnItems {
			dnQtyByOrderItem[di.OrderItemID] += di.QuantityDelivered
		}
	}

	for _, orderItem := range order.Items {
		// Determine the quantity to invoice for this item
		var invoiceQty float64
		if hasDNs {
			if len(dnItems) > 0 {
				// Specific DN provided: use that DN's delivered quantity for this item
				invoiceQty = dnQtyByOrderItem[orderItem.ID]
			} else {
				// No specific DN but order has DNs: invoice remaining uninvoiced quantity
				invoiceQty = orderItem.Quantity - orderItem.QuantityInvoiced
			}
			if invoiceQty <= 0.001 {
				continue // Skip fully invoiced items
			}
		} else {
			// Legacy (no DNs): invoice full order quantity
			invoiceQty = orderItem.Quantity
		}

		// Track invoiced quantity for this order item
		if strings.TrimSpace(orderItem.ID) != "" {
			invoicedQtyByOrderItemID[orderItem.ID] = invoiceQty
		}
		lineNum++

		// Use TotalCost from costing sheet if available, otherwise estimate
		var supplierCost float64
		if orderItem.TotalCost > 0 {
			supplierCost = orderItem.TotalCost * invoiceQty
		} else {
			// Fallback: try to get product cost
			var product ProductMaster
			if err := a.db.Where("id = ?", orderItem.ProductID).First(&product).Error; err == nil {
				supplierCost = product.StandardCostBHD * invoiceQty
			} else {
				// Estimate as 70% of selling price if no cost data
				supplierCost = orderItem.UnitPrice * invoiceQty * 0.70
			}
		}
		totalSupplierCost += supplierCost

		unitPrice := roundInvoiceMoney(orderItem.UnitPrice)
		fullLineTotal := roundInvoiceMoney(orderItem.TotalPrice)
		if unitPrice <= 0 && orderItem.Quantity > 0 && fullLineTotal > 0 {
			unitPrice = roundInvoiceMoney(fullLineTotal / orderItem.Quantity)
		}
		lineTotal := roundInvoiceMoney(invoiceQty * unitPrice)
		if lineTotal <= 0 && !hasDNs {
			lineTotal = fullLineTotal
		}
		if lineTotal <= 0 && invoiceQty > 0 && fullLineTotal > 0 && orderItem.Quantity <= 0 {
			lineTotal = fullLineTotal
		}
		if unitPrice <= 0 && invoiceQty > 0 && lineTotal > 0 {
			unitPrice = roundInvoiceMoney(lineTotal / invoiceQty)
		}
		invoiceSubtotal += lineTotal

		// Invoice item contains FULL costing sheet data (the costing sheet becomes the invoice)
		invoiceItem := DBInvoiceItem{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: now,
				UpdatedAt: now,
			},
			InvoiceID:   invoice.ID,
			LineNumber:  lineNum,
			Description: orderItem.Description,
			Quantity:    invoiceQty,
			Rate:        unitPrice,
			TotalBHD:    lineTotal,
			// Full costing data from OrderItem (originally from OfferItem/CostingSheet)
			ProductID:           orderItem.ProductID,
			ProductCode:         orderItem.ProductCode,
			Equipment:           orderItem.Equipment,
			Model:               orderItem.Model,
			Specification:       orderItem.Specification,
			DetailedDescription: orderItem.DetailedDescription,
			Currency:            orderItem.Currency,
			FOB:                 orderItem.FOB,
			Freight:             orderItem.Freight,
			TotalCost:           orderItem.TotalCost,
			MarginPercent:       orderItem.MarginPercent,
			TotalPrice:          lineTotal,
		}
		invoice.Items = append(invoice.Items, invoiceItem)
	}

	if len(invoice.Items) == 0 {
		if deliveryNoteID != "" {
			return Invoice{}, fmt.Errorf("delivery note %s has no remaining invoiceable items for this order", dnNumber)
		}
		return Invoice{}, fmt.Errorf("order %s has no invoiceable items", order.OrderNumber)
	}

	// For partial invoices (DN-based), recalculate totals from actual invoice items
	if hasDNs {
		invoice.SubtotalBHD = invoiceSubtotal
	}

	// Calculate margin metrics
	invoice.TotalSupplierCostBHD = totalSupplierCost
	invoice.GrossMarginBHD = invoice.SubtotalBHD - totalSupplierCost
	if invoice.SubtotalBHD > 0 {
		invoice.GrossMarginPercent = (invoice.GrossMarginBHD / invoice.SubtotalBHD) * 100.0
	}

	// P0-4 FIX: Alert on negative margin (don't block, but warn management)
	if invoice.GrossMarginBHD < 0 {
		log.Printf("⚠️ ALERT: Negative margin detected on invoice %s: %.3f BHD (%.2f%%)",
			invoice.InvoiceNumber, invoice.GrossMarginBHD, invoice.GrossMarginPercent)
		// Don't block invoice creation, but log for management review
	}

	// Calculate VAT and grand total
	if hasDNs {
		// Partial invoices: calculate VAT from invoice subtotal (not order total)
		invoice.VATPercent = 10.0 // Bahrain rate
		invoice.VATBHD = invoice.SubtotalBHD * 0.10
		invoice.GrandTotalBHD = invoice.SubtotalBHD + invoice.VATBHD
		invoice.OutstandingBHD = invoice.GrandTotalBHD
	} else if order.GrandTotalBHD > order.TotalValueBHD {
		// Legacy: order already has VAT baked in, extract the VAT amount
		invoice.VATBHD = order.GrandTotalBHD - order.TotalValueBHD
		if order.TotalValueBHD > 0 {
			invoice.VATPercent = (invoice.VATBHD / order.TotalValueBHD) * 100.0
		} else {
			invoice.VATPercent = 10.0 // Default Bahrain rate
		}
		// Use order's grand total (already includes VAT)
		invoice.GrandTotalBHD = order.GrandTotalBHD
		invoice.OutstandingBHD = invoice.GrandTotalBHD
	} else {
		// Legacy: order doesn't include VAT, calculate it (10% Bahrain rate)
		invoice.VATPercent = 10.0
		invoice.VATBHD = invoice.SubtotalBHD * 0.10
		invoice.GrandTotalBHD = invoice.SubtotalBHD + invoice.VATBHD
		invoice.OutstandingBHD = invoice.GrandTotalBHD
	}

	// P0 FIX (TOCTOU closed): Credit check + invoice creation + QuantityInvoiced update
	// all happen inside a SINGLE transaction. No gap between credit check and DB write.
	log.Printf("📋 CreateInvoiceWithOptions: Beginning atomic transaction (credit check + create + qty update)...")
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		// Step 0: Duplicate check inside transaction to prevent race conditions (Fix 6)
		if !hasDNs {
			var existingCount int64
			if err := tx.Model(&Invoice{}).Where("order_id = ?", orderID).Count(&existingCount).Error; err != nil {
				return fmt.Errorf("failed to check existing invoices: %w", err)
			}
			if existingCount > 0 {
				return fmt.Errorf("order already has %d invoice(s) — cannot create duplicate", existingCount)
			}
		}

		// Step 1: Credit limit check with row lock (SELECT FOR UPDATE)
		var customer CustomerMaster
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("customer_id = ?", order.CustomerID).First(&customer).Error; err == nil {
			// Customer found and locked — check credit limit atomically
			var totalOutstanding float64
			if err := tx.Model(&Invoice{}).
				Where("customer_id = ? AND status NOT IN ?", order.CustomerID, []string{"Paid", "Cancelled", "Void", "Proforma"}).
				Select("COALESCE(SUM(outstanding_bhd), 0)").
				Scan(&totalOutstanding).Error; err != nil {
				log.Printf("⚠️ Failed to calculate customer outstanding AR: %v", err)
				// Don't block invoice — log warning and continue
			} else {
				newTotalOutstanding := totalOutstanding + invoice.GrandTotalBHD
				creditLimit := customer.CreditLimitBHD
				if creditLimit == 0 {
					creditLimit = 50000.0
				}
				if newTotalOutstanding > creditLimit*0.80 && newTotalOutstanding <= creditLimit {
					log.Printf("⚠️ WARNING: Customer %s approaching credit limit (%.1f%% utilized: %.3f / %.3f BHD)",
						customer.BusinessName,
						(newTotalOutstanding/creditLimit)*100,
						newTotalOutstanding,
						creditLimit)
				}

				if customer.IsCreditBlocked {
					log.Printf("BLOCKED: Customer %s is credit blocked - no new invoices allowed", customer.BusinessName)
					return fmt.Errorf("customer %s is credit blocked - no new invoices allowed", customer.BusinessName)
				}
				if newTotalOutstanding > creditLimit {
					log.Printf("BLOCKED: Customer %s exceeds credit limit (%.3f BHD > %.3f BHD)",
						customer.BusinessName, newTotalOutstanding, creditLimit)
					return fmt.Errorf(
						"credit limit exceeded for %s: new outstanding %.3f exceeds limit %.3f",
						customer.BusinessName, newTotalOutstanding, creditLimit)
				}
			}
		}
		// Customer not found — skip credit check (not an error)

		// B1 FIX: Persist synthetic order item inside transaction (was previously outside)
		if syntheticItem != nil {
			if err := tx.Create(syntheticItem).Error; err != nil {
				log.Printf("⚠️ Could not persist synthetic order item: %v", err)
			} else {
				log.Printf("✅ Created synthetic order item for order %s (inside tx)", order.OrderNumber)
			}
		}

		// Step 2: Create invoice in database (GORM will cascade create items)
		if err := tx.Create(&invoice).Error; err != nil {
			log.Printf("❌ CreateInvoiceWithOptions: Failed to create invoice in DB: %v", err)
			return fmt.Errorf("failed to create invoice: %w", err)
		}
		log.Printf("✅ CreateInvoiceWithOptions: Invoice saved to database: ID=%s", invoice.ID)

		// Step 3: Update OrderItem.QuantityInvoiced using actual invoiced quantities
		for i, orderItem := range order.Items {
			invoicedQty, found := invoicedQtyByOrderItemID[orderItem.ID]
			if !found || invoicedQty <= 0.001 {
				continue // This order item was not invoiced (skipped in DN-based flow)
			}
			// Use atomic SQL increment to prevent race conditions
			if err := tx.Model(&OrderItem{}).
				Where("id = ?", orderItem.ID).
				Update("quantity_invoiced", gorm.Expr("quantity_invoiced + ?", invoicedQty)).Error; err != nil {
				log.Printf("⚠️ Failed to update quantity_invoiced for order item %s: %v", orderItem.ID, err)
				// Continue processing - this is not critical
			} else {
				log.Printf("✅ Updated OrderItem[%d]: quantity_invoiced += %.3f", i, invoicedQty)
			}
		}

		// Step 4: Compute and save invoice integrity hash (HMAC-SHA256 with server secret — P1-4)
		invoice.InvoiceHash = computeDocumentHMAC(invoice.InvoiceNumber, invoice.InvoiceDate.Format("2006-01-02"), invoice.GrandTotalBHD, invoice.VATBHD)
		if err := tx.Model(&invoice).Update("invoice_hash", invoice.InvoiceHash).Error; err != nil {
			log.Printf("⚠️ Failed to save invoice hash: %v", err)
		} else {
			log.Printf("✅ Invoice hash computed: %s...%s", invoice.InvoiceHash[:8], invoice.InvoiceHash[56:])
		}

		return nil
	}); err != nil {
		return Invoice{}, err
	}

	log.Printf("✅ Created Invoice: %s (Customer: %s, Order: %s, Total: %.3f BHD, Margin: %.3f BHD [%.1f%%])",
		invoice.InvoiceNumber, invoice.CustomerName, order.OrderNumber,
		invoice.GrandTotalBHD, invoice.GrossMarginBHD, invoice.GrossMarginPercent)

	// Emit event to notify frontend to refresh dashboard
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "data:refresh", map[string]interface{}{
			"source":         "invoice-created",
			"invoice_id":     invoice.ID,
			"invoice_number": invoice.InvoiceNumber,
			"order_id":       order.ID,
		})
	}

	// Phase 23: Link serial numbers from DN to this invoice
	if deliveryNoteID != "" {
		if err := a.linkSerialsToInvoice(invoice.ID, invoice.InvoiceNumber, deliveryNoteID); err != nil {
			log.Printf("⚠️ Failed to link serials to invoice: %v (non-blocking)", err)
		}
	}

	// Auto-progress order status to "Invoiced"
	if err := a.ProgressOrderOnInvoice(order.ID); err != nil {
		log.Printf("⚠️ Failed to progress order %s to Invoiced: %v", order.OrderNumber, err)
		// Don't fail the invoice creation, just log the warning
	}

	return invoice, nil
}

// ListCustomerInvoices retrieves all customer invoices with pagination
// Preloads items for complete data
func (a *App) ListCustomerInvoices(limit, offset int) ([]Invoice, error) {
	if err := a.requirePermission("invoices:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// B8 FIX: Enforce pagination bounds to prevent memory exhaustion
	if limit <= 0 {
		limit = 50
	} else if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	var invoices []Invoice
	query := a.db.Preload("Items").Order("invoice_date DESC").Limit(limit).Offset(offset)

	if err := query.Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve invoices: %w", err)
	}

	// Debug: Log items loaded per invoice
	for i, inv := range invoices {
		log.Printf("🔍 Invoice[%d] %s: %d items loaded", i, inv.InvoiceNumber, len(inv.Items))
	}

	log.Printf("📊 Retrieved %d customer invoices (limit: %d, offset: %d)", len(invoices), limit, offset)
	return invoices, nil
}

// GetCustomerInvoiceByID retrieves a single customer invoice by ID
// Preloads items for complete invoice data
func (a *App) GetCustomerInvoiceByID(id string) (Invoice, error) {
	if err := a.requirePermission("invoices:view"); err != nil {
		return Invoice{}, err
	}
	if a.db == nil {
		return Invoice{}, fmt.Errorf("database not initialized")
	}

	var invoice Invoice
	if err := a.db.Preload("Items").Where("id = ?", id).First(&invoice).Error; err != nil {
		return Invoice{}, fmt.Errorf("invoice not found: %w", err)
	}

	return invoice, nil
}

// UpdateCustomerInvoice updates an existing customer invoice
// Recalculates margin if costs changed, updates timestamp
// FIX: Uses Save() instead of Updates() to preserve ALL fields including zero-value strings
// SECURITY: Paid invoices are immutable to prevent audit trail tampering
func (a *App) UpdateCustomerInvoice(inv Invoice) (Invoice, error) {
	if err := a.requirePermission("invoices:update"); err != nil {
		return Invoice{}, err
	}
	if a.db == nil {
		return Invoice{}, fmt.Errorf("database not initialized")
	}

	// Verify invoice exists and load ALL fields
	var existing Invoice
	if err := a.db.Where("id = ?", inv.ID).First(&existing).Error; err != nil {
		return Invoice{}, fmt.Errorf("invoice not found: %w", err)
	}

	// SECURITY: Terminal states (Paid, Cancelled, Void) are immutable for audit compliance
	terminalStatuses := map[string]bool{"Paid": true, "Cancelled": true, "Void": true}
	if terminalStatuses[existing.Status] {
		return Invoice{}, fmt.Errorf("cannot modify invoice with status '%s' (terminal state — immutable for audit compliance)", existing.Status)
	}

	// CRITICAL FIX: Preserve ALL fields that weren't sent from frontend
	// The edit modal only sends 5 fields, so we must preserve the other 40+ fields

	// Fix 3: InvoiceNumber locked once past Draft — part of the audit trail + HMAC hash
	if inv.InvoiceNumber != "" && inv.InvoiceNumber != existing.InvoiceNumber {
		if existing.Status != "Draft" {
			return Invoice{}, fmt.Errorf("cannot change invoice number on %s invoice (locked for audit compliance)", existing.Status)
		}
		// Uniqueness check: verify no other invoice has the same number
		var count int64
		a.db.Model(&Invoice{}).Where("invoice_number = ? AND id != ?", inv.InvoiceNumber, inv.ID).Count(&count)
		if count > 0 {
			return Invoice{}, fmt.Errorf("invoice number %s already exists", inv.InvoiceNumber)
		}
		existing.InvoiceNumber = inv.InvoiceNumber
	}

	// Fix 1: Validate status allowlist before accepting (rejects empty string + adds Void/Proforma)
	validStatuses := map[string]bool{"Draft": true, "Sent": true, "Overdue": true, "PartiallyPaid": true, "Paid": true, "Cancelled": true, "Void": true, "Proforma": true}
	if !validStatuses[inv.Status] {
		return Invoice{}, fmt.Errorf("invalid invoice status: '%s'", inv.Status)
	}
	existing.Status = inv.Status

	// Fix 3: Prevent setting status to "Paid" when there's still outstanding balance
	if existing.Status == "Paid" && existing.OutstandingBHD > 0.001 {
		return Invoice{}, fmt.Errorf("cannot set status to Paid: outstanding balance is %.3f BHD (must be 0)", existing.OutstandingBHD)
	}

	// Fix 2: GrandTotalBHD is a computed field (SubtotalBHD + VATBHD) — NOT directly editable
	// OutstandingBHD intentionally NOT editable — must change via payments/credit notes only
	existing.CustomerPONumber = inv.CustomerPONumber

	// Update timestamp
	existing.UpdatedAt = time.Now()

	// Recalculate margin metrics if values changed
	if existing.SubtotalBHD > 0 && existing.TotalSupplierCostBHD > 0 {
		existing.GrossMarginBHD = existing.SubtotalBHD - existing.TotalSupplierCostBHD
		existing.GrossMarginPercent = (existing.GrossMarginBHD / existing.SubtotalBHD) * 100.0
	}

	// Use Save() to update ALL fields (preserves empty strings, zero values)
	if err := a.db.Save(&existing).Error; err != nil {
		return Invoice{}, fmt.Errorf("failed to update invoice: %w", err)
	}

	// If items were updated, handle them separately
	if len(inv.Items) > 0 {
		// Fix 4: Block item replacement on non-Draft invoices — items are part of the financial record
		if existing.Status != "Draft" {
			log.Printf("⚠️ Ignoring item replacement on %s invoice %s (only Draft invoices can have items modified)", existing.Status, existing.InvoiceNumber)
		} else {
			// Delete existing items
			if err := a.db.Where("invoice_id = ?", inv.ID).Delete(&DBInvoiceItem{}).Error; err != nil {
				log.Printf("⚠️ Failed to delete old invoice items: %v", err)
			}

			// Create new items
			for i := range inv.Items {
				item := &inv.Items[i]
				item.InvoiceID = inv.ID
				item.UpdatedAt = time.Now()
				if item.ID == "" {
					item.ID = uuid.New().String()
					item.CreatedAt = time.Now()
				}
				if err := a.db.Create(item).Error; err != nil {
					log.Printf("⚠️ Failed to create invoice item: %v", err)
				}
			}
		}
	}

	log.Printf("✅ Updated Invoice: %s (preserved all 44+ fields)", existing.InvoiceNumber)

	// Reload with items
	return a.GetCustomerInvoiceByID(inv.ID)
}

// DeleteCustomerInvoice deletes a customer invoice
// Only allows deletion if not paid (to prevent accounting issues)
// Reverses QuantityInvoiced updates on order items
// SECURITY: Blocks deletion if any payment history exists (audit requirement)
func (a *App) DeleteCustomerInvoice(id string) error {
	if ok, err := a.guardDeleteOrRequest("invoices:delete", "customer_invoice", id, "Customer invoice"); !ok {
		return err
	}
	if err := a.requirePermission("invoices:delete"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Verify invoice exists
	var invoice Invoice
	if err := a.db.Preload("Items").Where("id = ?", id).First(&invoice).Error; err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	// Block deletion of terminal-state invoices (audit trail preservation)
	undeletableStatuses := map[string]bool{"Paid": true, "Void": true, "Cancelled": true}
	if undeletableStatuses[invoice.Status] {
		return fmt.Errorf("cannot delete %s invoice: %s (terminal state — preserved for audit)", invoice.Status, invoice.InvoiceNumber)
	}

	// SECURITY: Block deletion if any payment history exists (audit requirement)
	var paymentCount int64
	a.db.Model(&Payment{}).Where("invoice_id = ?", id).Count(&paymentCount)
	if paymentCount > 0 {
		return fmt.Errorf("cannot delete invoice with payment history (%d payment(s) - audit requirement)", paymentCount)
	}

	// Reverse QuantityInvoiced on order items if order exists
	if invoice.OrderID != "" {
		var order Order
		if err := a.db.Preload("Items").Where("id = ?", invoice.OrderID).First(&order).Error; err == nil {
			// Match invoice items to order items and reverse quantities
			for _, invItem := range invoice.Items {
				for _, orderItem := range order.Items {
					// Match by description (simplified - could be more robust)
					if orderItem.Description == invItem.Description {
						newQuantityInvoiced := orderItem.QuantityInvoiced - invItem.Quantity
						if newQuantityInvoiced < 0 {
							newQuantityInvoiced = 0
						}
						if err := a.db.Model(&OrderItem{}).
							Where("id = ?", orderItem.ID).
							Update("quantity_invoiced", newQuantityInvoiced).Error; err != nil {
							log.Printf("⚠️ Failed to reverse quantity_invoiced for order item %s: %v", orderItem.ID, err)
						} else {
							log.Printf("✅ Reversed OrderItem quantity_invoiced: %.3f → %.3f",
								orderItem.QuantityInvoiced, newQuantityInvoiced)
						}
						break
					}
				}
			}
		}
	}

	// Delete invoice (GORM will cascade delete items)
	if err := a.db.Delete(&invoice).Error; err != nil {
		return fmt.Errorf("failed to delete invoice: %w", err)
	}

	log.Printf("🗑️ Deleted Invoice: %s", invoice.InvoiceNumber)
	return nil
}

// SendCustomerInvoice marks an invoice as sent to customer
// Updates status from Draft → Sent
func (a *App) SendCustomerInvoice(id string) error {
	if err := a.requirePermission("invoices:update"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Verify invoice exists
	var invoice Invoice
	if err := a.db.Where("id = ?", id).First(&invoice).Error; err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	// B9 FIX: Only Draft invoices can be sent — reject all other statuses
	if invoice.Status != "Draft" {
		return fmt.Errorf("cannot send invoice %s: status is %s (only Draft invoices can be sent)", invoice.InvoiceNumber, invoice.Status)
	}

	// Update status to Sent
	now := time.Now()
	if err := a.db.Model(&Invoice{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     "Sent",
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("failed to send invoice: %w", err)
	}

	log.Printf("✅ Sent Invoice: %s to customer %s", invoice.InvoiceNumber, invoice.CustomerName)
	return nil
}

// GetInvoicesByCustomer retrieves all invoices for a specific customer
// Ordered by invoice date descending (most recent first)
func (a *App) GetInvoicesByCustomer(customerID string) ([]Invoice, error) {
	if err := a.requirePermission("invoices:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var invoices []Invoice
	if err := a.db.Preload("Items").
		Where("customer_id = ?", customerID).
		Order("invoice_date DESC").
		Limit(500).
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve customer invoices: %w", err)
	}

	log.Printf("📊 Retrieved %d invoices for customer %s", len(invoices), customerID)
	return invoices, nil
}

// GenerateInvoiceNumber generates a sequential invoice number using a sequence table
// Format: INV-YYYYMMDD-NNNN (e.g., INV-20260123-0001)
// P0 FIX: Uses InvoiceSequence table with row-level locking to prevent race conditions
// The SELECT FOR UPDATE ensures atomic read-modify-write even under concurrent calls
func (a *App) GenerateInvoiceNumber() (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	now := time.Now()
	year := now.Year()
	prefix := "INV"
	datePrefix := fmt.Sprintf("%s-%s-", prefix, now.Format("20060102"))
	var invoiceNumber string

	// Use transaction with row-level locking on the sequence table
	err := a.db.Transaction(func(tx *gorm.DB) error {
		var seq InvoiceSequence

		// Try to get existing sequence with row lock (SELECT FOR UPDATE)
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("prefix = ? AND year = ?", prefix, year).
			First(&seq).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Create new sequence for this year (first invoice of the year)
				// Initialize LastNumber based on existing invoices (for migration safety)
				var maxExisting int64
				tx.Model(&Invoice{}).
					Where("invoice_number LIKE ? ESCAPE '\\'", fmt.Sprintf("%s-%d%%", prefix, year%100)).
					Count(&maxExisting)

				seq = InvoiceSequence{
					Prefix:     prefix,
					Year:       year,
					LastNumber: int(maxExisting), // Start from existing count
					UpdatedAt:  now,
				}
				if err := tx.Create(&seq).Error; err != nil {
					return fmt.Errorf("failed to create invoice sequence: %w", err)
				}
			} else {
				return fmt.Errorf("failed to lock invoice sequence: %w", err)
			}
		}

		// Increment sequence number atomically (row is locked)
		seq.LastNumber++
		seq.UpdatedAt = now

		if err := tx.Save(&seq).Error; err != nil {
			return fmt.Errorf("failed to update invoice sequence: %w", err)
		}

		invoiceNumber = fmt.Sprintf("%s%04d", datePrefix, seq.LastNumber)
		return nil
	})

	if err != nil {
		log.Printf("❌ Invoice number generation failed: %v", err)
		return "", fmt.Errorf("failed to generate invoice number: %w", err)
	}

	log.Printf("🔢 Generated invoice number: %s (sequence-locked)", invoiceNumber)
	return invoiceNumber, nil
}

// =============================================================================
// PAYMENT TRACKING & STATUS MANAGEMENT
// =============================================================================

// MarkCustomerInvoicePaid marks an invoice as paid
// Updates outstanding balance to 0 and status to Paid
// P1 FIX: Race condition protection via database transaction
func (a *App) MarkCustomerInvoicePaid(id string, paymentDate time.Time, paymentRef string) error {
	if err := a.requirePermission("payments:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// P1 FIX: Use database transaction to prevent concurrent payment race conditions
	tx := a.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("❌ Transaction rolled back due to panic: %v", r)
		}
	}()

	// Verify invoice exists (with transaction lock)
	var invoice Invoice
	if err := tx.Where("id = ?", id).First(&invoice).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("invoice not found: %w", err)
	}

	// Fix 5: Block marking terminal-state invoices as paid
	nonMarkableStatuses := map[string]bool{"Cancelled": true, "Void": true, "Proforma": true, "Paid": true}
	if nonMarkableStatuses[invoice.Status] {
		tx.Rollback()
		return fmt.Errorf("cannot mark invoice as paid: current status is '%s'", invoice.Status)
	}

	// Update status and outstanding balance atomically
	now := time.Now()
	if err := tx.Model(&Invoice{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":          "Paid",
			"outstanding_bhd": 0.0,
			"updated_at":      now,
		}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to mark invoice as paid: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit payment transaction: %w", err)
	}

	log.Printf("💰 Marked Invoice as Paid: %s (Customer: %s, Amount: %.3f BHD, Ref: %s)",
		invoice.InvoiceNumber, invoice.CustomerName, invoice.GrandTotalBHD, paymentRef)

	return nil
}

// MarkCustomerInvoiceOverdue marks an invoice as overdue
// Called by background job checking due dates
func (a *App) MarkCustomerInvoiceOverdue(id string) error {
	if err := a.requirePermission("invoices:update"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Verify invoice exists and is not paid
	var invoice Invoice
	if err := a.db.Where("id = ?", id).First(&invoice).Error; err != nil {
		return fmt.Errorf("invoice not found: %w", err)
	}

	if invoice.Status == "Paid" {
		return fmt.Errorf("cannot mark paid invoice as overdue: %s", invoice.InvoiceNumber)
	}

	// Don't regress PartiallyPaid — partial payment information is more important than overdue status
	if invoice.Status == "PartiallyPaid" {
		return fmt.Errorf("cannot mark partially paid invoice as overdue: %s (status: PartiallyPaid)", invoice.InvoiceNumber)
	}
	// Block terminal states
	if invoice.Status == "Cancelled" || invoice.Status == "Void" {
		return fmt.Errorf("cannot mark %s invoice as overdue: %s", invoice.Status, invoice.InvoiceNumber)
	}

	// Update status to Overdue
	now := time.Now()
	if err := a.db.Model(&Invoice{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     "Overdue",
			"updated_at": now,
		}).Error; err != nil {
		return fmt.Errorf("failed to mark invoice as overdue: %w", err)
	}

	log.Printf("⚠️ Marked Invoice as Overdue: %s (Customer: %s, Due: %s, Outstanding: %.3f BHD)",
		invoice.InvoiceNumber, invoice.CustomerName,
		invoice.DueDate.Format("2006-01-02"), invoice.OutstandingBHD)

	return nil
}

// GetOverdueInvoices retrieves all overdue invoices
// Invoices where due_date < now AND status != 'Paid'
func (a *App) GetOverdueInvoices() ([]Invoice, error) {
	if err := a.requirePermission("invoices:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := time.Now()
	var invoices []Invoice
	if err := a.db.Preload("Items").
		Where("due_date < ? AND status != ?", now, "Paid").
		Order("due_date ASC").
		Limit(500).
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve overdue invoices: %w", err)
	}

	log.Printf("⚠️ Retrieved %d overdue invoices", len(invoices))
	return invoices, nil
}

// GetUnpaidInvoices retrieves all unpaid invoices
// Invoices where status IN ('Draft', 'Sent', 'Overdue')
func (a *App) GetUnpaidInvoices() ([]Invoice, error) {
	if err := a.requirePermission("invoices:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var invoices []Invoice
	if err := a.db.Preload("Items").
		Where("status IN ?", []string{"Draft", "Sent", "Overdue"}).
		Order("invoice_date DESC").
		Limit(500).
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve unpaid invoices: %w", err)
	}

	log.Printf("📊 Retrieved %d unpaid invoices", len(invoices))
	return invoices, nil
}

// GetInvoicesByStatus retrieves invoices filtered by status
func (a *App) GetInvoicesByStatus(status string) ([]Invoice, error) {
	if err := a.requirePermission("invoices:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var invoices []Invoice
	if err := a.db.Preload("Items").
		Where("status = ?", status).
		Order("invoice_date DESC").
		Limit(500).
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve invoices by status: %w", err)
	}

	log.Printf("📊 Retrieved %d invoices with status: %s", len(invoices), status)
	return invoices, nil
}

// RecordPartialPayment records a partial payment against an invoice
// Reduces outstanding balance, keeps status as Sent/Overdue until fully paid
// P1 FIX: Race condition protection via database transaction with row-level locking
// BE-8 FIX: Idempotency key prevents duplicate payments from network retries
func (a *App) RecordPartialPayment(id string, paymentAmount float64, paymentDate time.Time, paymentRef string) error {
	if err := a.requirePermission("payments:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// BE-8 FIX: Generate idempotency key from payment parameters
	// Same invoice + amount + date + ref = same key = same payment (idempotent)
	idempotencyData := fmt.Sprintf("%s|%.3f|%s|%s", id, paymentAmount, paymentDate.Format("2006-01-02"), paymentRef)
	idempotencyKey := fmt.Sprintf("%x", sha256.Sum256([]byte(idempotencyData)))

	// BE-8 FIX: Check if this payment was already processed (idempotency check)
	var existingPayment Payment
	if err := a.db.Where("idempotency_key = ?", idempotencyKey).First(&existingPayment).Error; err == nil {
		log.Printf("💰 Idempotent payment request - already processed: %s (Invoice: %s, Amount: %.3f)",
			idempotencyKey[:16]+"...", id, paymentAmount)
		return nil // Idempotent - return success without creating duplicate
	}

	// P1 FIX: Use database transaction with row-level locking to prevent race conditions
	// Multiple concurrent payments cannot create negative balances because:
	// 1. BEGIN IMMEDIATE acquires write lock on database (SQLite-specific)
	// 2. Read outstanding balance with SELECT ... FOR UPDATE (row lock in PostgreSQL)
	// 3. Validate payment <= outstanding atomically
	// 4. Update outstanding balance and create payment record atomically
	// 5. COMMIT releases lock - concurrent payments are serialized
	tx := a.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			log.Printf("❌ Transaction rolled back due to panic: %v", r)
		}
	}()

	// P0 FIX: Read invoice with SELECT FOR UPDATE row lock
	// This prevents concurrent payments from reading the same outstanding balance
	// Works with both SQLite (via transaction) and PostgreSQL (true row lock)
	var invoice Invoice
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id = ?", id).First(&invoice).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("invoice not found: %w", err)
	}

	// Fix 5: Reject payments on non-payable invoices
	nonPayableStatuses := map[string]bool{"Cancelled": true, "Void": true, "Proforma": true, "Paid": true}
	if nonPayableStatuses[invoice.Status] {
		tx.Rollback()
		return fmt.Errorf("cannot record payment on invoice with status '%s'", invoice.Status)
	}

	// Verify payment amount is valid
	if paymentAmount <= 0 {
		tx.Rollback()
		return fmt.Errorf("payment amount must be positive: %.3f", paymentAmount)
	}

	// CRITICAL: Validate payment against LOCKED outstanding balance
	// This is atomic - no other payment can modify outstanding between check and update
	if paymentAmount > invoice.OutstandingBHD+0.001 {
		tx.Rollback()
		return fmt.Errorf("payment amount (%.3f) exceeds outstanding balance (%.3f)",
			paymentAmount, invoice.OutstandingBHD)
	}

	// Calculate new outstanding balance
	newOutstanding := invoice.OutstandingBHD - paymentAmount

	// Update status based on new outstanding balance
	newStatus := invoice.Status
	if newOutstanding <= 0.001 { // Tolerance for floating point comparison
		newStatus = "Paid"
		newOutstanding = 0.0
	} else if newOutstanding < invoice.GrandTotalBHD {
		newStatus = "PartiallyPaid"
	}

	// Update invoice atomically within transaction
	now := time.Now()
	if err := tx.Model(&Invoice{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"outstanding_bhd": newOutstanding,
			"status":          newStatus,
			"updated_at":      now,
		}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to record partial payment: %w", err)
	}

	// Create Payment record with idempotency key (I12 fix: was missing, broke idempotency check)
	payment := Payment{
		InvoiceID:      id,
		InvoiceNumber:  invoice.InvoiceNumber,
		AmountBHD:      paymentAmount,
		PaymentDate:    paymentDate,
		PaymentMethod:  "Bank Transfer",
		Reference:      paymentRef,
		IdempotencyKey: idempotencyKey,
		DaysToPayment:  int(time.Since(invoice.InvoiceDate).Hours() / 24),
	}
	payment.ID = uuid.New().String()
	if err := tx.Create(&payment).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create payment record: %w", err)
	}
	log.Printf("✅ Payment record created with idempotency key: %s...", idempotencyKey[:16])

	// Commit transaction - releases lock, makes changes visible
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit payment transaction: %w", err)
	}

	log.Printf("💰 Recorded Partial Payment: %.3f BHD for Invoice %s (Ref: %s, Outstanding: %.3f → %.3f BHD)",
		paymentAmount, invoice.InvoiceNumber, paymentRef, invoice.OutstandingBHD, newOutstanding)

	return nil
}

// =============================================================================
// ANALYTICS & REPORTING
// =============================================================================

// GetInvoiceRevenueSummary calculates total revenue metrics
// Returns: (total_revenue, paid_revenue, outstanding_revenue, invoice_count)
func (a *App) GetInvoiceRevenueSummary() (float64, float64, float64, int64, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return 0, 0, 0, 0, err
	}
	if a.db == nil {
		return 0, 0, 0, 0, fmt.Errorf("database not initialized")
	}

	var totalRevenue, paidRevenue, outstandingRevenue float64
	var invoiceCount int64

	// Total revenue (all invoices)
	if err := a.db.Model(&Invoice{}).
		Select("COALESCE(SUM(grand_total_bhd), 0)").
		Scan(&totalRevenue).Error; err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to calculate total revenue: %w", err)
	}

	// Paid revenue
	if err := a.db.Model(&Invoice{}).
		Where("status = ?", "Paid").
		Select("COALESCE(SUM(grand_total_bhd), 0)").
		Scan(&paidRevenue).Error; err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to calculate paid revenue: %w", err)
	}

	// Outstanding revenue
	if err := a.db.Model(&Invoice{}).
		Select("COALESCE(SUM(outstanding_bhd), 0)").
		Scan(&outstandingRevenue).Error; err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to calculate outstanding revenue: %w", err)
	}

	// Invoice count
	if err := a.db.Model(&Invoice{}).Count(&invoiceCount).Error; err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to count invoices: %w", err)
	}

	log.Printf("📊 Revenue Summary: Total=%.3f BHD, Paid=%.3f BHD, Outstanding=%.3f BHD, Count=%d",
		totalRevenue, paidRevenue, outstandingRevenue, invoiceCount)

	return totalRevenue, paidRevenue, outstandingRevenue, invoiceCount, nil
}

// GetInvoicesByDateRange retrieves invoices within a date range
func (a *App) GetInvoicesByDateRange(startDate, endDate time.Time) ([]Invoice, error) {
	if err := a.requirePermission("invoices:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var invoices []Invoice
	if err := a.db.Preload("Items").
		Where("invoice_date BETWEEN ? AND ?", startDate, endDate).
		Order("invoice_date DESC").
		Limit(500).
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve invoices by date range: %w", err)
	}

	log.Printf("📊 Retrieved %d invoices between %s and %s",
		len(invoices), startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))
	return invoices, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// calculateDueDate calculates invoice due date from invoice date and payment terms
// Supports formats: "Net 30", "Net 45", "Net 60", "COD", "Advance", etc.
func calculateDueDate(invoiceDate time.Time, paymentTerms string) time.Time {
	// Default to Net 30 if empty
	if paymentTerms == "" {
		return invoiceDate.AddDate(0, 0, 30)
	}

	// Normalize payment terms
	terms := strings.ToUpper(strings.TrimSpace(paymentTerms))

	// Parse different formats
	if strings.Contains(terms, "NET") {
		// Extract number from "Net 30", "NET 45", etc.
		parts := strings.Fields(terms)
		for _, part := range parts {
			if days, err := strconv.Atoi(part); err == nil {
				return invoiceDate.AddDate(0, 0, days)
			}
		}
	}

	// Special cases
	if strings.Contains(terms, "COD") || strings.Contains(terms, "CASH") {
		return invoiceDate // Due immediately
	}

	if strings.Contains(terms, "ADVANCE") || strings.Contains(terms, "PREPAY") {
		return invoiceDate.AddDate(0, 0, -1) // Due before invoice date
	}

	if strings.Contains(terms, "EOM") || strings.Contains(terms, "END OF MONTH") {
		// Due at end of month
		year, month, _ := invoiceDate.Date()
		lastDay := time.Date(year, month+1, 0, 0, 0, 0, 0, invoiceDate.Location())
		return lastDay
	}

	// Default to Net 30 if can't parse
	log.Printf("⚠️ Could not parse payment terms '%s', defaulting to Net 30", paymentTerms)
	return invoiceDate.AddDate(0, 0, 30)
}

// =============================================================================
// AR AGING & LATE PAYMENT TRACKING (P1 FINANCE FIXES)
// =============================================================================

// CalculateARAgingBuckets recalculates AR aging for all customers on-demand
// Returns aging buckets: Current (0-30), 30-60, 60-90, 90-120, 120+
func (a *App) CalculateARAgingBuckets() ([]ARAgingBucket, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := time.Now()
	var customers []CustomerMaster
	if err := a.db.Limit(1000).Find(&customers).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve customers: %w", err)
	}

	var agingBuckets []ARAgingBucket

	for _, customer := range customers {
		// Get all unpaid invoices for this customer
		var invoices []Invoice
		if err := a.db.Where("customer_id = ? AND status != ?", customer.CustomerID, "Paid").
			Find(&invoices).Error; err != nil {
			log.Printf("⚠️ Failed to get invoices for customer %s: %v", customer.CustomerID, err)
			continue
		}

		if len(invoices) == 0 {
			continue // Skip customers with no outstanding invoices
		}

		// Initialize aging bucket
		bucket := ARAgingBucket{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: now,
				UpdatedAt: now,
			},
			CustomerID:   customer.CustomerID,
			CustomerName: customer.BusinessName,
			SnapshotDate: now,
		}

		var oldestOverdueDays int

		// Categorize each invoice into aging buckets
		for _, inv := range invoices {
			daysOverdue := int(now.Sub(inv.DueDate).Hours() / 24)
			outstanding := inv.OutstandingBHD

			// Categorize by age
			if daysOverdue <= 0 {
				// Current (not yet due)
				bucket.Less15Days += outstanding
			} else if daysOverdue <= 30 {
				bucket.Days16_30 += outstanding
			} else if daysOverdue <= 60 {
				bucket.Days31_60 += outstanding
			} else if daysOverdue <= 90 {
				bucket.Days61_90 += outstanding
			} else {
				bucket.Over90Days += outstanding
			}

			// Track oldest overdue
			if daysOverdue > oldestOverdueDays {
				oldestOverdueDays = daysOverdue
			}

			bucket.TotalOutstanding += outstanding
		}

		// Calculate total overdue (everything except current)
		bucket.TotalOverdue = bucket.Days16_30 + bucket.Days31_60 + bucket.Days61_90 + bucket.Over90Days
		bucket.OverdueDays = oldestOverdueDays

		// Calculate risk tier based on aging distribution
		if bucket.Over90Days > bucket.TotalOutstanding*0.5 {
			bucket.RiskTier = "Critical"
			bucket.RiskScore = 1.0
		} else if bucket.Days61_90+bucket.Over90Days > bucket.TotalOutstanding*0.3 {
			bucket.RiskTier = "High"
			bucket.RiskScore = 0.75
		} else if bucket.Days31_60+bucket.Days61_90+bucket.Over90Days > bucket.TotalOutstanding*0.2 {
			bucket.RiskTier = "Medium"
			bucket.RiskScore = 0.5
		} else {
			bucket.RiskTier = "Low"
			bucket.RiskScore = 0.25
		}

		agingBuckets = append(agingBuckets, bucket)

		// Update customer's AR fields
		if err := a.db.Model(&customer).Updates(map[string]interface{}{
			"outstanding_bhd": bucket.TotalOutstanding,
			"overdue_days":    bucket.OverdueDays,
			"ar_risk_tier":    bucket.RiskTier,
		}).Error; err != nil {
			log.Printf("⚠️ Failed to update customer AR fields: %v", err)
		}
	}

	// Save aging buckets to database
	for _, bucket := range agingBuckets {
		// Clean up old aging snapshots (keep only the latest)
		a.db.Where("customer_id = ?", bucket.CustomerID).Delete(&ARAgingBucket{})
		if err := a.db.Create(&bucket).Error; err != nil {
			log.Printf("⚠️ Failed to save AR aging bucket for customer %s: %v", bucket.CustomerID, err)
		}
	}

	log.Printf("✅ Calculated AR aging for %d customers", len(agingBuckets))
	return agingBuckets, nil
}

// GetARAgingByCustomer retrieves the latest AR aging bucket for a customer
func (a *App) GetARAgingByCustomer(customerID string) (ARAgingBucket, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return ARAgingBucket{}, err
	}
	if a.db == nil {
		return ARAgingBucket{}, fmt.Errorf("database not initialized")
	}

	var bucket ARAgingBucket
	if err := a.db.Where("customer_id = ?", customerID).
		Order("snapshot_date DESC").
		First(&bucket).Error; err != nil {
		return ARAgingBucket{}, fmt.Errorf("AR aging not found for customer: %w", err)
	}

	return bucket, nil
}

// GetLatePaymentInvoices returns all invoices past due date
// Flags invoices > 30 days overdue as critical
func (a *App) GetLatePaymentInvoices() ([]Invoice, error) {
	if err := a.requirePermission("invoices:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	now := time.Now()
	var invoices []Invoice
	if err := a.db.Preload("Items").
		Where("due_date < ? AND status != ?", now, "Paid").
		Order("due_date ASC").
		Find(&invoices).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve late payment invoices: %w", err)
	}

	// Calculate days past due for each invoice
	for i := range invoices {
		daysPastDue := int(now.Sub(invoices[i].DueDate).Hours() / 24)
		invoices[i].UpdatedAt = time.Now() // Use UpdatedAt to store calculated value temporarily

		if daysPastDue > 30 {
			log.Printf("🚨 CRITICAL: Invoice %s is %d days overdue (Customer: %s, Amount: %.3f BHD)",
				invoices[i].InvoiceNumber, daysPastDue, invoices[i].CustomerName, invoices[i].OutstandingBHD)
		}
	}

	log.Printf("⚠️ Retrieved %d late payment invoices", len(invoices))
	return invoices, nil
}

// TrackLatePaymentHistory calculates late payment metrics for a customer
// Returns: (total_invoices, late_invoices, avg_days_late, max_days_late)
func (a *App) TrackLatePaymentHistory(customerID string) (int64, int64, float64, int, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return 0, 0, 0, 0, err
	}
	if a.db == nil {
		return 0, 0, 0, 0, fmt.Errorf("database not initialized")
	}

	// Get all paid invoices for customer
	var payments []Payment
	if err := a.db.Joins("JOIN invoices ON invoices.id = payments.invoice_id").
		Where("invoices.customer_id = ?", customerID).
		Find(&payments).Error; err != nil {
		return 0, 0, 0, 0, fmt.Errorf("failed to retrieve payment history: %w", err)
	}

	if len(payments) == 0 {
		return 0, 0, 0, 0, nil
	}

	var totalInvoices int64 = int64(len(payments))
	var lateInvoices int64 = 0
	var totalDaysLate int = 0
	var maxDaysLate int = 0

	for _, payment := range payments {
		// Get invoice to check due date
		var invoice Invoice
		if err := a.db.Where("id = ?", payment.InvoiceID).First(&invoice).Error; err != nil {
			continue
		}

		// Calculate days past due (payment_date - due_date)
		daysPastDue := int(payment.PaymentDate.Sub(invoice.DueDate).Hours() / 24)

		if daysPastDue > 0 {
			lateInvoices++
			totalDaysLate += daysPastDue

			if daysPastDue > maxDaysLate {
				maxDaysLate = daysPastDue
			}
		}
	}

	var avgDaysLate float64 = 0
	if lateInvoices > 0 {
		avgDaysLate = float64(totalDaysLate) / float64(lateInvoices)
	}

	log.Printf("📊 Late payment history for customer %s: %d/%d invoices late (avg: %.1f days, max: %d days)",
		customerID, lateInvoices, totalInvoices, avgDaysLate, maxDaysLate)

	return totalInvoices, lateInvoices, avgDaysLate, maxDaysLate, nil
}

// =============================================================================
// DELIVERY NOTE LINKAGE
// =============================================================================

// GetAvailableDeliveryNotesForOrder retrieves delivery notes that can be linked to an invoice
// Returns DNs for the specified order that are not already linked to another invoice
func (a *App) GetAvailableDeliveryNotesForOrder(orderID string) ([]DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get all DNs for this order
	var allDNs []DeliveryNote
	if err := a.db.Preload("Items").Where("order_id = ?", orderID).Order("delivery_date DESC").Find(&allDNs).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve delivery notes: %w", err)
	}

	// Get DN IDs already linked to invoices
	var linkedDNIDs []string
	if err := a.db.Model(&Invoice{}).
		Where("delivery_note_id IS NOT NULL AND delivery_note_id != ''").
		Pluck("delivery_note_id", &linkedDNIDs).Error; err != nil {
		log.Printf("⚠️ Failed to get linked DN IDs: %v", err)
		// Continue with all DNs if this query fails
		return allDNs, nil
	}

	// Create a set for quick lookup
	linkedSet := make(map[string]bool)
	for _, id := range linkedDNIDs {
		linkedSet[id] = true
	}

	// Filter out linked DNs
	var availableDNs []DeliveryNote
	for _, dn := range allDNs {
		if !linkedSet[dn.ID] {
			availableDNs = append(availableDNs, dn)
		}
	}

	log.Printf("📦 Found %d available DNs for order %s (total: %d, linked: %d)",
		len(availableDNs), orderID, len(allDNs), len(linkedDNIDs))
	return availableDNs, nil
}
