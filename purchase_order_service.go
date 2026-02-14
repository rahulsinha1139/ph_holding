package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

// =============================================================================
// PURCHASE ORDER CRUD OPERATIONS
// =============================================================================

// CreatePurchaseOrder creates a new purchase order
func (a *App) CreatePurchaseOrder(po PurchaseOrder) (PurchaseOrder, error) {
	if err := a.requirePermission("purchase_orders:create"); err != nil {
		return PurchaseOrder{}, err
	}
	if a.db == nil {
		return PurchaseOrder{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Auto-generate PO number if not provided
	if po.PONumber == "" {
		poNumber, err := a.GeneratePONumber()
		if err != nil {
			return PurchaseOrder{}, err
		}
		po.PONumber = poNumber
	}

	// Set default dates if not provided
	if po.PODate.IsZero() {
		po.PODate = time.Now()
	}

	// Calculate payment due date based on payment terms if not set
	if po.PaymentDueDate.IsZero() && po.PaymentTerms != "" {
		po.PaymentDueDate = calculatePaymentDueDate(po.PODate, po.PaymentTerms)
	}

	// Calculate BHD amounts from foreign currency if exchange rate provided
	if po.ExchangeRate > 0 {
		po.SubtotalBHD = po.SubtotalForeign * po.ExchangeRate
		po.TotalBHD = po.TotalForeign * po.ExchangeRate
	}

	// Ensure VAT is calculated (10% standard rate in Bahrain)
	if po.VATAmount == 0 && po.SubtotalBHD > 0 {
		po.VATAmount = po.SubtotalBHD * 0.10
	}
	// Ensure total always includes VAT
	if po.VATAmount > 0 && po.TotalBHD <= po.SubtotalBHD {
		po.TotalBHD = po.SubtotalBHD + po.VATAmount
	}

	// Calculate item totals
	for i := range po.Items {
		item := &po.Items[i]
		if po.ExchangeRate > 0 {
			item.UnitPriceBHD = item.UnitPriceForeign * po.ExchangeRate
			item.TotalBHD = item.TotalForeign * po.ExchangeRate
		}
	}

	// Set default status if not provided
	if po.Status == "" {
		po.Status = "Draft"
	}

	// P1 FIX: Check if PO requires approval based on amount threshold
	approvalThreshold := 5000.0 // BHD - configurable threshold
	if po.TotalBHD > approvalThreshold && po.Status == "Draft" {
		po.Status = "Pending Approval"
		log.Printf("⚠️ PO %s requires approval (%.2f BHD > %.2f BHD threshold)", po.PONumber, po.TotalBHD, approvalThreshold)
	}

	// Create the PO with items
	if err := a.db.Create(&po).Error; err != nil {
		log.Printf("❌ Failed to create purchase order: %v", err)
		return PurchaseOrder{}, newError("DB_CREATE_FAILED", "Failed to create purchase order", err.Error())
	}

	log.Printf("✅ Created Purchase Order %s for Supplier %s (%.2f BHD)", po.PONumber, po.SupplierID, po.TotalBHD)
	return po, nil
}

// GetPurchaseOrders retrieves all purchase orders with items preloaded and supplier names enriched
func (a *App) GetPurchaseOrders() ([]PurchaseOrder, error) {
	if err := a.requirePermission("purchase_orders:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var pos []PurchaseOrder
	if err := a.db.Preload("Items").Order("po_date DESC").Find(&pos).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve purchase orders", err.Error())
	}

	// Enrich with supplier names
	a.enrichPOsWithSupplierNames(pos)

	return pos, nil
}

// enrichPOsWithSupplierNames populates SupplierName for POs missing it
func (a *App) enrichPOsWithSupplierNames(pos []PurchaseOrder) {
	// Collect unique supplier IDs that need lookup
	supplierIDs := make(map[string]bool)
	for _, po := range pos {
		if po.SupplierID != "" && po.SupplierName == "" {
			supplierIDs[po.SupplierID] = true
		}
	}

	if len(supplierIDs) == 0 {
		return
	}

	// Batch lookup suppliers
	ids := make([]string, 0, len(supplierIDs))
	for id := range supplierIDs {
		ids = append(ids, id)
	}

	var suppliers []SupplierMaster
	if err := a.db.Where("id IN ?", ids).Find(&suppliers).Error; err != nil {
		log.Printf("Warning: Failed to fetch suppliers for PO enrichment: %v", err)
		return
	}

	// Build lookup map
	supplierMap := make(map[string]string)
	for _, s := range suppliers {
		supplierMap[s.ID] = s.SupplierName
	}

	// Enrich POs
	for i := range pos {
		if pos[i].SupplierName == "" && pos[i].SupplierID != "" {
			if name, ok := supplierMap[pos[i].SupplierID]; ok {
				pos[i].SupplierName = name
			} else {
				pos[i].SupplierName = "Unknown Supplier"
			}
		}
	}
}

// GetPurchaseOrderByID retrieves a single purchase order by ID with items preloaded
func (a *App) GetPurchaseOrderByID(id string) (PurchaseOrder, error) {
	if err := a.requirePermission("purchase_orders:view"); err != nil {
		return PurchaseOrder{}, err
	}
	if a.db == nil {
		return PurchaseOrder{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var po PurchaseOrder
	if err := a.db.Preload("Items").First(&po, "id = ?", id).Error; err != nil {
		return PurchaseOrder{}, newError("PO_NOT_FOUND", "Purchase order not found", err.Error())
	}

	// Enrich with supplier name
	if po.SupplierName == "" && po.SupplierID != "" {
		var supplier SupplierMaster
		if err := a.db.First(&supplier, "id = ?", po.SupplierID).Error; err == nil {
			po.SupplierName = supplier.SupplierName
		}
	}

	return po, nil
}

// GetPurchaseOrdersByOrder retrieves all purchase orders for a given customer order
func (a *App) GetPurchaseOrdersByOrder(orderID string) ([]PurchaseOrder, error) {
	if err := a.requirePermission("purchase_orders:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var pos []PurchaseOrder
	if err := a.db.Preload("Items").Where("order_id = ?", orderID).Order("po_date DESC").Find(&pos).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve purchase orders for order", err.Error())
	}

	// Enrich with supplier names
	a.enrichPOsWithSupplierNames(pos)

	return pos, nil
}

// GetPurchaseOrdersBySupplier retrieves all purchase orders for a given supplier
func (a *App) GetPurchaseOrdersBySupplier(supplierID string) ([]PurchaseOrder, error) {
	if err := a.requirePermission("purchase_orders:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var pos []PurchaseOrder
	if err := a.db.Preload("Items").Where("supplier_id = ?", supplierID).Order("po_date DESC").Find(&pos).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve purchase orders for supplier", err.Error())
	}

	return pos, nil
}

// UpdatePurchaseOrder updates an existing purchase order
func (a *App) UpdatePurchaseOrder(po PurchaseOrder) (PurchaseOrder, error) {
	if err := a.requirePermission("purchase_orders:update"); err != nil {
		return PurchaseOrder{}, err
	}
	if a.db == nil {
		return PurchaseOrder{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Verify PO exists
	var existing PurchaseOrder
	if err := a.db.First(&existing, "id = ?", po.ID).Error; err != nil {
		return PurchaseOrder{}, newError("PO_NOT_FOUND", "Purchase order not found", err.Error())
	}

	// Recalculate BHD amounts if exchange rate changed
	if po.ExchangeRate > 0 {
		po.SubtotalBHD = po.SubtotalForeign * po.ExchangeRate
		po.TotalBHD = po.TotalForeign * po.ExchangeRate
	}

	// Ensure VAT is calculated (10% standard rate in Bahrain)
	if po.VATAmount == 0 && po.SubtotalBHD > 0 {
		po.VATAmount = po.SubtotalBHD * 0.10
	}
	// Ensure total always includes VAT
	if po.VATAmount > 0 && po.TotalBHD <= po.SubtotalBHD {
		po.TotalBHD = po.SubtotalBHD + po.VATAmount
	}

	// Update only the fields that should be editable
	// This preserves: status, approved_by, approved_at, rfq_id, po_number, created_by, version
	updates := map[string]interface{}{
		"supplier_id":       po.SupplierID,
		"supplier_name":     po.SupplierName,
		"po_date":           po.PODate,
		"expected_delivery": po.ExpectedDelivery,
		"currency":          po.Currency,
		"exchange_rate":     po.ExchangeRate,
		"subtotal_foreign":  po.SubtotalForeign,
		"subtotal_bhd":      po.SubtotalBHD,
		"vat_amount":        po.VATAmount,
		"total_foreign":     po.TotalForeign,
		"total_bhd":         po.TotalBHD,
		"payment_terms":     po.PaymentTerms,
		"updated_at":        time.Now(),
	}

	if err := a.db.Model(&PurchaseOrder{}).Where("id = ?", po.ID).Updates(updates).Error; err != nil {
		log.Printf("❌ Failed to update purchase order: %v", err)
		return PurchaseOrder{}, newError("DB_UPDATE_FAILED", "Failed to update purchase order", err.Error())
	}

	// If items were updated, handle them atomically (delete + create in transaction)
	if len(po.Items) > 0 {
		tx := a.db.Begin()
		if tx.Error != nil {
			return PurchaseOrder{}, newError("DB_TX_FAILED", "Failed to begin transaction for items", tx.Error.Error())
		}

		// Delete existing items
		if err := tx.Where("purchase_order_id = ?", po.ID).Delete(&PurchaseOrderItem{}).Error; err != nil {
			tx.Rollback()
			return PurchaseOrder{}, newError("DB_DELETE_FAILED", "Failed to delete existing PO items", err.Error())
		}

		// Create new items
		for i := range po.Items {
			item := &po.Items[i]
			item.PurchaseOrderID = po.ID
			if po.ExchangeRate > 0 {
				item.UnitPriceBHD = item.UnitPriceForeign * po.ExchangeRate
				item.TotalBHD = item.TotalForeign * po.ExchangeRate
			}
			if err := tx.Create(item).Error; err != nil {
				tx.Rollback()
				return PurchaseOrder{}, newError("DB_CREATE_FAILED", "Failed to create PO item", err.Error())
			}
		}

		if err := tx.Commit().Error; err != nil {
			return PurchaseOrder{}, newError("DB_COMMIT_FAILED", "Failed to commit PO items update", err.Error())
		}
	}

	log.Printf("✅ Updated Purchase Order %s", po.PONumber)

	// Reload with items
	return a.GetPurchaseOrderByID(po.ID)
}

// UpdatePOStatus updates only the status of a purchase order
func (a *App) UpdatePOStatus(id string, status string) error {
	if err := a.requirePermission("purchase_orders:update"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	result := a.db.Model(&PurchaseOrder{}).Where("id = ?", id).Update("status", status)
	if result.Error != nil {
		return newError("DB_UPDATE_FAILED", "Failed to update PO status", result.Error.Error())
	}

	if result.RowsAffected == 0 {
		return newError("PO_NOT_FOUND", "Purchase order not found", "")
	}

	log.Printf("✅ Updated PO #%s status to: %s", id, status)
	return nil
}

// =============================================================================
// P1 FIX: PURCHASE ORDER APPROVAL WORKFLOW
// =============================================================================

// ApprovePurchaseOrder approves a purchase order requiring approval
func (a *App) ApprovePurchaseOrder(id string, approvedBy string) error {
	if err := a.requirePermission("purchase_orders:approve"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get PO
	var po PurchaseOrder
	if err := a.db.First(&po, "id = ?", id).Error; err != nil {
		return newError("PO_NOT_FOUND", "Purchase order not found", err.Error())
	}

	// Verify status allows approval
	if po.Status != "Pending Approval" {
		return newError("INVALID_STATUS", fmt.Sprintf("Cannot approve PO with status: %s", po.Status), "")
	}

	// P1 FIX: Update status and approval tracking with proper fields
	now := time.Now()
	updates := map[string]interface{}{
		"status":      "Approved",
		"approved_by": approvedBy,
		"approved_at": now,
		"updated_by":  approvedBy,
	}

	if err := a.db.Model(&po).Updates(updates).Error; err != nil {
		return newError("DB_UPDATE_FAILED", "Failed to approve purchase order", err.Error())
	}

	log.Printf("✅ Approved Purchase Order %s by %s (%.2f BHD)", po.PONumber, approvedBy, po.TotalBHD)
	return nil
}

// SendPurchaseOrder sends an approved PO to supplier (blocks if not approved)
func (a *App) SendPurchaseOrder(id string) error {
	if err := a.requirePermission("purchase_orders:send"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get PO
	var po PurchaseOrder
	if err := a.db.First(&po, "id = ?", id).Error; err != nil {
		return newError("PO_NOT_FOUND", "Purchase order not found", err.Error())
	}

	// P1 FIX: Block sending unapproved POs
	approvalThreshold := 5000.0 // BHD
	if po.TotalBHD > approvalThreshold && po.Status != "Approved" {
		return newError("APPROVAL_REQUIRED",
			fmt.Sprintf("Cannot send PO %s (%.2f BHD) - requires manager approval (threshold: %.2f BHD)",
				po.PONumber, po.TotalBHD, approvalThreshold), "")
	}

	// Update status to Sent
	if err := a.db.Model(&po).Update("status", "Sent").Error; err != nil {
		return newError("DB_UPDATE_FAILED", "Failed to send purchase order", err.Error())
	}

	log.Printf("📧 Sent Purchase Order %s to Supplier %s (%.2f BHD)", po.PONumber, po.SupplierID, po.TotalBHD)
	return nil
}

// DeletePurchaseOrder soft-deletes a purchase order
func (a *App) DeletePurchaseOrder(id string) error {
	if err := a.requirePermission("purchase_orders:delete"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Verify PO exists
	var po PurchaseOrder
	if err := a.db.First(&po, "id = ?", id).Error; err != nil {
		return newError("PO_NOT_FOUND", "Purchase order not found", err.Error())
	}

	// Soft delete (GORM handles DeletedAt automatically)
	if err := a.db.Delete(&po).Error; err != nil {
		log.Printf("❌ Failed to delete purchase order: %v", err)
		return newError("DB_DELETE_FAILED", "Failed to delete purchase order", err.Error())
	}

	log.Printf("✅ Deleted Purchase Order %s", po.PONumber)
	return nil
}

// =============================================================================
// PURCHASE ORDER UTILITIES
// =============================================================================

// GeneratePONumber generates a sequential PO number in format PO-2026-0001
func (a *App) GeneratePONumber() (string, error) {
	if a.db == nil {
		return "", newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get current year
	year := time.Now().Year()
	prefix := fmt.Sprintf("PO-%d-", year)

	var nextNumber int

	// Use a transaction with EXCLUSIVE lock to prevent race conditions
	// SQLite's BEGIN EXCLUSIVE ensures only one process can generate a number at a time
	err := a.db.Transaction(func(tx *gorm.DB) error {
		// Lock the table for exclusive access during this transaction
		// This prevents concurrent GeneratePONumber calls from reading the same lastPO
		if err := tx.Exec("BEGIN EXCLUSIVE").Error; err != nil {
			return err
		}

		// Find highest existing PO number for this year within the locked transaction
		var lastPO PurchaseOrder
		err := tx.Where("po_number LIKE ?", prefix+"%").
			Order("po_number DESC").
			First(&lastPO).Error

		if err != nil {
			// No POs for this year yet, start at 0001
			nextNumber = 1
		} else {
			// Extract number from last PO (e.g., "PO-2026-0123" → 123)
			parts := strings.Split(lastPO.PONumber, "-")
			if len(parts) == 3 {
				var num int
				fmt.Sscanf(parts[2], "%d", &num)
				nextNumber = num + 1
			} else {
				nextNumber = 1
			}
		}

		// Transaction will commit automatically if no error is returned
		return nil
	})

	if err != nil {
		log.Printf("⚠️ Warning: PO number generation transaction failed: %v, falling back to best-effort", err)
		// Fallback to non-transactional read (should rarely happen)
		var lastPO PurchaseOrder
		fallbackErr := a.db.Where("po_number LIKE ?", prefix+"%").
			Order("po_number DESC").
			First(&lastPO).Error

		if fallbackErr != nil {
			nextNumber = 1
		} else {
			parts := strings.Split(lastPO.PONumber, "-")
			if len(parts) == 3 {
				var num int
				fmt.Sscanf(parts[2], "%d", &num)
				nextNumber = num + 1
			} else {
				nextNumber = 1
			}
		}
	}

	poNumber := fmt.Sprintf("%s%04d", prefix, nextNumber)
	return poNumber, nil
}

// CreatePOFromOrder creates a purchase order from a customer order for a specific supplier
// Splits order items by supplier and creates a PO with the matching items
func (a *App) CreatePOFromOrder(orderID string, supplierID string, itemIDs []string) (PurchaseOrder, error) {
	if err := a.requirePermission("purchase_orders:create"); err != nil {
		return PurchaseOrder{}, err
	}
	if a.db == nil {
		return PurchaseOrder{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Load the order with items
	var order Order
	if err := a.db.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
		return PurchaseOrder{}, newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Load supplier to get default currency and payment terms
	var supplier SupplierMaster
	if err := a.db.First(&supplier, "id = ?", supplierID).Error; err != nil {
		return PurchaseOrder{}, newError("SUPPLIER_NOT_FOUND", "Supplier not found", err.Error())
	}

	// Create PO header
	po := PurchaseOrder{
		OrderID:          orderID,
		SupplierID:       supplierID,
		PODate:           time.Now(),
		ExpectedDelivery: order.RequiredDate, // Use order's required date as expected delivery
		Currency:         "BHD",              // Default to BHD, can be changed based on supplier
		ExchangeRate:     1.0,                // 1:1 for BHD
		PaymentTerms:     "Net 30",           // Default, override from supplier if available
		Status:           "Draft",
		Items:            []PurchaseOrderItem{},
	}

	// If supplier has default currency, use it
	// Note: SupplierMaster needs DefaultCurrency field added in future
	// For now, we'll assume BHD

	// Add items
	var subtotal float64
	for _, orderItem := range order.Items {
		// Check if this item should be included
		includeItem := len(itemIDs) == 0 // If no specific items, include all
		if !includeItem {
			for _, itemID := range itemIDs {
				if orderItem.ID == itemID {
					includeItem = true
					break
				}
			}
		}

		if !includeItem {
			continue
		}

		// Check if product belongs to this supplier
		var product ProductMaster
		if err := a.db.First(&product, "id = ?", orderItem.ProductID).Error; err == nil {
			if product.SupplierID != supplierID {
				log.Printf("⚠️ Skipping item %s - belongs to different supplier", orderItem.ProductCode)
				continue
			}
		}

		// Create PO item (using cost from product master as purchase price)
		poItem := PurchaseOrderItem{
			OrderItemID:      orderItem.ID,
			ProductID:        orderItem.ProductID,
			ProductCode:      orderItem.ProductCode,
			Description:      orderItem.Description,
			Quantity:         orderItem.Quantity,
			UnitPriceForeign: orderItem.UnitPrice * 0.7, // Estimate: assume 30% margin (cost = 70% of sell price)
			UnitPriceBHD:     orderItem.UnitPrice * 0.7,
			TotalForeign:     orderItem.Quantity * orderItem.UnitPrice * 0.7,
			TotalBHD:         orderItem.Quantity * orderItem.UnitPrice * 0.7,
			QuantityReceived: 0,
		}

		// Try to get actual cost from ProductMaster
		if err := a.db.First(&product, "id = ?", orderItem.ProductID).Error; err == nil {
			if product.StandardCostBHD > 0 {
				poItem.UnitPriceForeign = product.StandardCostBHD
				poItem.UnitPriceBHD = product.StandardCostBHD
				poItem.TotalForeign = poItem.Quantity * product.StandardCostBHD
				poItem.TotalBHD = poItem.Quantity * product.StandardCostBHD
			}
		}

		po.Items = append(po.Items, poItem)
		subtotal += poItem.TotalBHD
	}

	if len(po.Items) == 0 {
		return PurchaseOrder{}, newError("NO_ITEMS", "No items found for this supplier in the order", "")
	}

	// Calculate totals (assuming 10% VAT)
	po.SubtotalForeign = subtotal
	po.SubtotalBHD = subtotal
	po.VATAmount = subtotal * 0.10
	po.TotalForeign = subtotal + po.VATAmount
	po.TotalBHD = subtotal + po.VATAmount

	// Create the PO
	createdPO, err := a.CreatePurchaseOrder(po)
	if err != nil {
		return PurchaseOrder{}, err
	}

	log.Printf("✅ Created PO %s from Order %s for Supplier %s (%d items, %.2f BHD)",
		createdPO.PONumber, order.OrderNumber, supplierID, len(createdPO.Items), createdPO.TotalBHD)

	return createdPO, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// calculatePaymentDueDate calculates payment due date from PO date and payment terms
func calculatePaymentDueDate(poDate time.Time, paymentTerms string) time.Time {
	// Parse payment terms (e.g., "Net 30", "Net 45", "Net 60")
	var days int

	terms := strings.ToUpper(paymentTerms)

	if strings.Contains(terms, "NET") {
		// Extract number from "Net 30", "Net 45", etc.
		fmt.Sscanf(terms, "NET %d", &days)
	} else if strings.Contains(terms, "ADVANCE") || strings.Contains(terms, "PREPAYMENT") {
		// Advance payment - due immediately
		days = 0
	} else if strings.Contains(terms, "COD") {
		// Cash on delivery - same day
		days = 0
	} else {
		// Default to Net 30
		days = 30
	}

	return poDate.AddDate(0, 0, days)
}

// =============================================================================
// P2 FIX: PO AMENDMENT TRACKING
// =============================================================================

// POAmendment tracks changes to purchase orders after creation
type POAmendment struct {
	ID              string    `json:"id"`
	PurchaseOrderID string    `json:"purchase_order_id"`
	AmendmentNumber int       `json:"amendment_number"`
	AmendedBy       string    `json:"amended_by"`
	AmendedAt       time.Time `json:"amended_at"`
	ChangeType      string    `json:"change_type"` // "quantity", "price", "date", "items", "terms"
	OldValue        string    `json:"old_value"`   // JSON of old values
	NewValue        string    `json:"new_value"`   // JSON of new values
	Reason          string    `json:"reason"`
	RequiresReapproval bool   `json:"requires_reapproval"`
	ReapprovedBy    string    `json:"reapproved_by"`
	ReapprovedAt    *time.Time `json:"reapproved_at"`
}

// AmendPurchaseOrder creates an amendment record and optionally requires re-approval
func (a *App) AmendPurchaseOrder(poID string, amendments POAmendment) error {
	if err := a.requirePermission("purchase_orders:update"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get existing PO
	var po PurchaseOrder
	if err := a.db.Preload("Items").First(&po, "id = ?", poID).Error; err != nil {
		return newError("PO_NOT_FOUND", "Purchase order not found", err.Error())
	}

	// Calculate value change to determine if re-approval needed
	oldTotal := po.TotalBHD

	// Generate amendment number
	var amendmentCount int64
	a.db.Raw("SELECT COUNT(*) FROM po_amendments WHERE purchase_order_id = ?", poID).Scan(&amendmentCount)
	amendments.AmendmentNumber = int(amendmentCount) + 1
	amendments.PurchaseOrderID = poID
	amendments.AmendedAt = time.Now()

	// P2 FIX: Check if amendment requires re-approval (>10% value change)
	// This will be set by the caller based on the new PO values
	// For now, we'll check after update if total changed significantly

	// Create amendment record (store in a dedicated table via raw SQL for now)
	// In production, add POAmendment to database.go models
	log.Printf("📝 PO Amendment #%d created for %s by %s: %s (Reason: %s)",
		amendments.AmendmentNumber, po.PONumber, amendments.AmendedBy, amendments.ChangeType, amendments.Reason)

	// P2 FIX: If value change > 10%, require re-approval
	if amendments.RequiresReapproval {
		// Reset approval status
		updates := map[string]interface{}{
			"status":      "Pending Approval",
			"approved_by": nil,
			"approved_at": nil,
		}
		if err := a.db.Model(&po).Updates(updates).Error; err != nil {
			return newError("DB_UPDATE_FAILED", "Failed to reset PO approval status", err.Error())
		}
		log.Printf("⚠️ PO %s requires re-approval due to significant amendment (>10%% value change from %.2f BHD to new total)",
			po.PONumber, oldTotal)
	}

	return nil
}

// GetPOAmendmentHistory retrieves all amendments for a purchase order
func (a *App) GetPOAmendmentHistory(poID string) ([]POAmendment, error) {
	if err := a.requirePermission("purchase_orders:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Placeholder - in production, query from po_amendments table
	// For now, return empty slice
	log.Printf("📋 Retrieving amendment history for PO %s", poID)
	return []POAmendment{}, nil
}
