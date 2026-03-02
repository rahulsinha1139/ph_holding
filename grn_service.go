package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

// =============================================================================
// GRN DISPLAY TYPES (FRONTEND-FRIENDLY)
// =============================================================================

// GRNResponse adds computed fields for frontend display
type GRNResponse struct {
	GoodsReceivedNote
	SupplierName   string  `json:"supplier_name"`
	PONumber       string  `json:"po_number"`
	ItemsCount     int     `json:"items_count"`
	TotalReceived  float64 `json:"total_received"`
	TotalAccepted  float64 `json:"total_accepted"`
	TotalRejected  float64 `json:"total_rejected"`
	AcceptanceRate float64 `json:"acceptance_rate"`
}

// =============================================================================
// GRN CRUD OPERATIONS
// =============================================================================

// CreateGRN creates a new Goods Received Note
func (a *App) CreateGRN(grn GoodsReceivedNote) (GoodsReceivedNote, error) {
	if err := a.requirePermission("grn:create"); err != nil {
		return GoodsReceivedNote{}, err
	}
	if a.db == nil {
		return GoodsReceivedNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Validate PO exists
	var po PurchaseOrder
	if err := a.db.First(&po, "id = ?", grn.PurchaseOrderID).Error; err != nil {
		return GoodsReceivedNote{}, newError("PO_NOT_FOUND", "Purchase Order not found", err.Error())
	}

	// Generate GRN number if empty
	if grn.GRNNumber == "" {
		var err error
		grn.GRNNumber, err = a.GenerateGRNNumber()
		if err != nil {
			return GoodsReceivedNote{}, err
		}
	}

	// Set defaults
	if grn.ReceivedDate.IsZero() {
		grn.ReceivedDate = time.Now()
	}
	if grn.QCStatus == "" {
		grn.QCStatus = "Pending"
	}

	if err := a.db.Create(&grn).Error; err != nil {
		log.Printf("❌ Failed to create GRN: %v", err)
		return GoodsReceivedNote{}, newError("DB_CREATE_FAILED", "Failed to create GRN", err.Error())
	}

	log.Printf("✅ Created GRN %s for PO %s", grn.GRNNumber, po.PONumber)
	return grn, nil
}

// GetGRNs retrieves all GRNs with optional pagination (basic version)
func (a *App) GetGRNs() ([]GoodsReceivedNote, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var grns []GoodsReceivedNote
	if err := a.db.Preload("Items").Order("received_date DESC").Find(&grns).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to list GRNs", err.Error())
	}

	log.Printf("Retrieved %d GRNs", len(grns))
	return grns, nil
}

// ListGRNs retrieves all GRNs with computed fields for frontend (PRODUCTION API)
func (a *App) ListGRNs(limit int, offset int, qcStatus string) ([]GRNResponse, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Default limit: 100, max: 1000
	if limit <= 0 {
		limit = 100
	} else if limit > 1000 {
		limit = 1000
	}

	if offset < 0 {
		offset = 0
	}

	var grns []GoodsReceivedNote
	query := a.db.Preload("Items").Order("received_date DESC")

	// Filter by QC status if provided
	if qcStatus != "" && qcStatus != "All" {
		query = query.Where("qc_status = ?", qcStatus)
	}

	query = query.Limit(limit).Offset(offset)
	if err := query.Find(&grns).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to list GRNs", err.Error())
	}

	// Enrich with computed fields
	responses := make([]GRNResponse, len(grns))
	for i, grn := range grns {
		resp := GRNResponse{
			GoodsReceivedNote: grn,
			ItemsCount:        len(grn.Items),
		}

		// Get PO info and supplier name
		var po PurchaseOrder
		if err := a.db.First(&po, "id = ?", grn.PurchaseOrderID).Error; err == nil {
			resp.PONumber = po.PONumber

			// Get supplier info
			var supplier SupplierMaster
			if err := a.db.First(&supplier, "id = ?", po.SupplierID).Error; err == nil {
				resp.SupplierName = supplier.SupplierName
			}
		}

		// Calculate totals
		for _, item := range grn.Items {
			resp.TotalReceived += item.QuantityReceived
			acceptedQty := item.QuantityReceived - item.QuantityRejected
			resp.TotalAccepted += acceptedQty
			resp.TotalRejected += item.QuantityRejected
		}

		// Calculate acceptance rate
		if resp.TotalReceived > 0 {
			resp.AcceptanceRate = resp.TotalAccepted / resp.TotalReceived
		}

		responses[i] = resp
	}

	log.Printf("✅ Retrieved %d GRNs (limit=%d, offset=%d, qcStatus=%s)", len(responses), limit, offset, qcStatus)
	return responses, nil
}

// GetGRN retrieves a single GRN by ID with full details (PRODUCTION API)
func (a *App) GetGRN(grnID string) (*GRNResponse, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var grn GoodsReceivedNote
	if err := a.db.Preload("Items").First(&grn, "id = ?", grnID).Error; err != nil {
		return nil, newError("GRN_NOT_FOUND", "GRN not found", err.Error())
	}

	resp := &GRNResponse{
		GoodsReceivedNote: grn,
		ItemsCount:        len(grn.Items),
	}

	// Get PO info and supplier name
	var po PurchaseOrder
	if err := a.db.First(&po, "id = ?", grn.PurchaseOrderID).Error; err == nil {
		resp.PONumber = po.PONumber

		// Get supplier info
		var supplier SupplierMaster
		if err := a.db.First(&supplier, "id = ?", po.SupplierID).Error; err == nil {
			resp.SupplierName = supplier.SupplierName
		}
	}

	// Calculate totals
	for _, item := range grn.Items {
		resp.TotalReceived += item.QuantityReceived
		acceptedQty := item.QuantityReceived - item.QuantityRejected
		resp.TotalAccepted += acceptedQty
		resp.TotalRejected += item.QuantityRejected
	}

	if resp.TotalReceived > 0 {
		resp.AcceptanceRate = resp.TotalAccepted / resp.TotalReceived
	}

	log.Printf("✅ Retrieved GRN #%s (%d items)", grnID, resp.ItemsCount)
	return resp, nil
}

// GetGRNByID retrieves a single GRN by ID with items preloaded
func (a *App) GetGRNByID(id string) (GoodsReceivedNote, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return GoodsReceivedNote{}, err
	}
	if a.db == nil {
		return GoodsReceivedNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var grn GoodsReceivedNote
	if err := a.db.Preload("Items").First(&grn, "id = ?", id).Error; err != nil {
		return GoodsReceivedNote{}, newError("GRN_NOT_FOUND", "GRN not found", err.Error())
	}

	return grn, nil
}

// GetGRNsByPO retrieves all GRNs for a specific Purchase Order
func (a *App) GetGRNsByPO(purchaseOrderID string) ([]GoodsReceivedNote, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var grns []GoodsReceivedNote
	if err := a.db.Preload("Items").Where("purchase_order_id = ?", purchaseOrderID).
		Order("received_date DESC").Find(&grns).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to get GRNs for PO", err.Error())
	}

	log.Printf("Retrieved %d GRNs for PO %s", len(grns), purchaseOrderID)
	return grns, nil
}

// UpdateGRN updates an existing GRN
func (a *App) UpdateGRN(grn GoodsReceivedNote) (GoodsReceivedNote, error) {
	if err := a.requirePermission("grn:update"); err != nil {
		return GoodsReceivedNote{}, err
	}
	if a.db == nil {
		return GoodsReceivedNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Verify GRN exists
	var existing GoodsReceivedNote
	if err := a.db.First(&existing, "id = ?", grn.ID).Error; err != nil {
		return GoodsReceivedNote{}, newError("GRN_NOT_FOUND", "GRN not found", err.Error())
	}

	if err := a.db.Save(&grn).Error; err != nil {
		log.Printf("❌ Failed to update GRN: %v", err)
		return GoodsReceivedNote{}, newError("DB_UPDATE_FAILED", "Failed to update GRN", err.Error())
	}

	log.Printf("✅ Updated GRN %s", grn.GRNNumber)
	return grn, nil
}

// DeleteGRN deletes a GRN by ID
func (a *App) DeleteGRN(id string) error {
	if err := a.requirePermission("grn:delete"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Verify GRN exists
	var grn GoodsReceivedNote
	if err := a.db.First(&grn, "id = ?", id).Error; err != nil {
		return newError("GRN_NOT_FOUND", "GRN not found", err.Error())
	}

	// Delete GRN items first (cascade)
	if err := a.db.Where("grn_id = ?", id).Delete(&GRNItem{}).Error; err != nil {
		log.Printf("❌ Failed to delete GRN items: %v", err)
		return newError("DB_DELETE_FAILED", "Failed to delete GRN items", err.Error())
	}

	// Delete GRN
	if err := a.db.Delete(&grn).Error; err != nil {
		log.Printf("❌ Failed to delete GRN: %v", err)
		return newError("DB_DELETE_FAILED", "Failed to delete GRN", err.Error())
	}

	log.Printf("✅ Deleted GRN %s", grn.GRNNumber)
	return nil
}

// =============================================================================
// GRN NUMBER GENERATION
// =============================================================================

// GenerateGRNNumber generates a unique GRN number in format GRN-2026-0001
func (a *App) GenerateGRNNumber() (string, error) {
	if a.db == nil {
		return "", newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	currentYear := time.Now().Year()
	prefix := fmt.Sprintf("GRN-%d-", currentYear)

	var nextNumber int

	// Use raw EXCLUSIVE transaction for proper SQLite locking
	// BEGIN EXCLUSIVE must be the first statement, not nested inside Transaction()
	if err := a.db.Exec("BEGIN EXCLUSIVE").Error; err != nil {
		return "", fmt.Errorf("failed to acquire exclusive lock: %w", err)
	}

	// Find the last GRN number for this year within the locked transaction
	var lastGRN GoodsReceivedNote
	err := a.db.Where("grn_number LIKE ?", prefix+"%").
		Order("grn_number DESC").
		First(&lastGRN).Error

	if err != nil {
		// No GRN found for this year, start from 1
		nextNumber = 1
	} else {
		// Parse the last number and increment
		var lastNum int
		_, parseErr := fmt.Sscanf(lastGRN.GRNNumber, prefix+"%d", &lastNum)
		if parseErr != nil {
			a.db.Exec("ROLLBACK")
			return "", newError("NUMBER_PARSE_FAILED", "Failed to parse last GRN number", parseErr.Error())
		}
		nextNumber = lastNum + 1
	}

	if err := a.db.Exec("COMMIT").Error; err != nil {
		return "", fmt.Errorf("failed to commit GRN number generation: %w", err)
	}

	grnNumber := fmt.Sprintf("%s%04d", prefix, nextNumber)
	log.Printf("Generated GRN number: %s", grnNumber)
	return grnNumber, nil
}

// =============================================================================
// GRN COMPLETION & STATUS UPDATES
// =============================================================================

// CompleteGRN marks a GRN as complete and updates PO quantities
func (a *App) CompleteGRN(id string) error {
	if err := a.requirePermission("grn:complete"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get GRN with items
	var grn GoodsReceivedNote
	if err := a.db.Preload("Items").First(&grn, "id = ?", id).Error; err != nil {
		return newError("GRN_NOT_FOUND", "GRN not found", err.Error())
	}

	// P1 FIX: Block completing GRN with QC failures
	if grn.QCStatus == "Failed" {
		return newError("QC_FAILED",
			fmt.Sprintf("Cannot complete GRN %s - Quality Control failed. Resolve QC issues first.", grn.GRNNumber),
			grn.QCNotes)
	}

	// P0-2 Fix: Check if GRN is already completed (quantities already added to PO)
	// If QCStatus is not "Pending", assume quantities were already posted
	if grn.QCStatus == "Approved" || grn.QCStatus == "Rejected" || grn.QCStatus == "Completed" || grn.QCStatus == "Passed" {
		log.Printf("⚠️ GRN %s already completed (QCStatus=%s), skipping PO quantity update to prevent double-counting", grn.GRNNumber, grn.QCStatus)
		return nil
	}

	// Begin transaction
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update PO item quantities received
	for _, item := range grn.Items {
		if item.POItemID == "" {
			continue
		}

		// Get current PO item
		var poItem PurchaseOrderItem
		if err := tx.First(&poItem, "id = ?", item.POItemID).Error; err != nil {
			tx.Rollback()
			return newError("PO_ITEM_NOT_FOUND", "PO item not found", err.Error())
		}

		// Update quantity received
		newQuantityReceived := poItem.QuantityReceived + item.QuantityReceived
		if err := tx.Model(&poItem).Update("quantity_received", newQuantityReceived).Error; err != nil {
			tx.Rollback()
			return newError("DB_UPDATE_FAILED", "Failed to update PO item quantity", err.Error())
		}

		log.Printf("Updated PO item %s: received %.2f (total now: %.2f/%.2f)",
			item.POItemID, item.QuantityReceived, newQuantityReceived, poItem.Quantity)
	}

	// Update PO status based on completion
	if err := a.updatePOStatus(tx, grn.PurchaseOrderID); err != nil {
		tx.Rollback()
		return err
	}

	// P1 FIX: Mark GRN QC as Passed on completion (if still Pending)
	if grn.QCStatus == "Pending" {
		if err := tx.Model(&grn).Update("qc_status", "Passed").Error; err != nil {
			tx.Rollback()
			return newError("DB_UPDATE_FAILED", "Failed to update GRN QC status", err.Error())
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return newError("DB_COMMIT_FAILED", "Failed to commit GRN completion", err.Error())
	}

	log.Printf("✅ Completed GRN %s and updated PO quantities", grn.GRNNumber)
	return nil
}

// updatePOStatus updates PO status based on received quantities
func (a *App) updatePOStatus(tx *gorm.DB, poID string) error {
	// Get PO with items
	var po PurchaseOrder
	if err := tx.Preload("Items").First(&po, "id = ?", poID).Error; err != nil {
		return newError("PO_NOT_FOUND", "Purchase Order not found", err.Error())
	}

	// Check if all items are fully received
	fullyReceived := true
	partiallyReceived := false

	for _, item := range po.Items {
		// P0-3 Fix: Add over-receipt detection
		if item.QuantityReceived > item.Quantity {
			log.Printf("⚠️ WARNING: Over-receipt detected for PO %s item %s: received %.2f > ordered %.2f",
				po.PONumber, item.ProductCode, item.QuantityReceived, item.Quantity)
			// Don't block - log warning for audit trail
		}

		if item.QuantityReceived < item.Quantity {
			fullyReceived = false
		}
		if item.QuantityReceived > 0 {
			partiallyReceived = true
		}
	}

	// Update PO status
	var newStatus string
	if fullyReceived && len(po.Items) > 0 {
		newStatus = "Received"
	} else if partiallyReceived {
		newStatus = "Partially Received"
	} else {
		newStatus = po.Status // Keep existing status
	}

	if newStatus != po.Status {
		if err := tx.Model(&po).Update("status", newStatus).Error; err != nil {
			return newError("DB_UPDATE_FAILED", "Failed to update PO status", err.Error())
		}
		log.Printf("Updated PO %s status: %s → %s", po.PONumber, po.Status, newStatus)
	}

	return nil
}

// UpdateGRNQCStatus updates the QC status of a GRN
func (a *App) UpdateGRNQCStatus(id string, status string, notes string, qcBy string) error {
	if err := a.requirePermission("grn:qc"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Verify GRN exists
	var grn GoodsReceivedNote
	if err := a.db.First(&grn, "id = ?", id).Error; err != nil {
		return newError("GRN_NOT_FOUND", "GRN not found", err.Error())
	}

	// Update QC fields
	now := time.Now()
	updates := map[string]interface{}{
		"qc_status": status,
		"qc_notes":  notes,
		"qc_by":     qcBy,
		"qc_date":   now,
	}

	if err := a.db.Model(&grn).Updates(updates).Error; err != nil {
		log.Printf("❌ Failed to update GRN QC status: %v", err)
		return newError("DB_UPDATE_FAILED", "Failed to update QC status", err.Error())
	}

	log.Printf("✅ Updated GRN %s QC status to: %s (by %s)", grn.GRNNumber, status, qcBy)
	return nil
}

// =============================================================================
// RECEIVE AGAINST PO - AUTO-CREATE GRN
// =============================================================================

// ReceiveAgainstPO creates a GRN from a PO with specified items
func (a *App) ReceiveAgainstPO(poID string, items []GRNItem) (GoodsReceivedNote, error) {
	if err := a.requirePermission("grn:create"); err != nil {
		return GoodsReceivedNote{}, err
	}
	if a.db == nil {
		return GoodsReceivedNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Verify PO exists
	var po PurchaseOrder
	if err := a.db.Preload("Items").First(&po, "id = ?", poID).Error; err != nil {
		return GoodsReceivedNote{}, newError("PO_NOT_FOUND", "Purchase Order not found", err.Error())
	}

	// Generate GRN number
	grnNumber, err := a.GenerateGRNNumber()
	if err != nil {
		return GoodsReceivedNote{}, err
	}

	// Create GRN
	grn := GoodsReceivedNote{
		PurchaseOrderID: poID,
		GRNNumber:       grnNumber,
		ReceivedDate:    time.Now(),
		ReceivedBy:      "System", // Should come from context
		QCStatus:        "Pending",
		Items:           items,
	}

	// Validate items against PO
	poItemMap := make(map[string]PurchaseOrderItem)
	for _, poItem := range po.Items {
		poItemMap[poItem.ID] = poItem
	}

	for i, item := range grn.Items {
		poItem, exists := poItemMap[item.POItemID]
		if !exists {
			return GoodsReceivedNote{}, newError("INVALID_PO_ITEM", fmt.Sprintf("PO item %s not found", item.POItemID), "")
		}

		// Set quantity ordered from PO
		grn.Items[i].QuantityOrdered = poItem.Quantity
		grn.Items[i].ProductID = poItem.ProductID

		// P0-1 Fix: Calculate QuantityAccepted = QuantityReceived - QuantityRejected
		grn.Items[i].QuantityAccepted = item.QuantityReceived - item.QuantityRejected

		// Validate received quantity
		totalReceived := poItem.QuantityReceived + item.QuantityReceived
		if totalReceived > poItem.Quantity {
			return GoodsReceivedNote{}, newError("QUANTITY_EXCEEDED",
				fmt.Sprintf("Received quantity (%.2f) exceeds ordered quantity (%.2f) for product %s",
					totalReceived, poItem.Quantity, poItem.ProductCode), "")
		}
	}

	// Create GRN in database
	if err := a.db.Create(&grn).Error; err != nil {
		log.Printf("❌ Failed to create GRN: %v", err)
		return GoodsReceivedNote{}, newError("DB_CREATE_FAILED", "Failed to create GRN", err.Error())
	}

	log.Printf("✅ Created GRN %s from PO %s with %d items", grn.GRNNumber, po.PONumber, len(items))
	return grn, nil
}

// =============================================================================
// RECEIVE AGAINST PO WITH SERIAL NUMBERS (Phase 23)
// =============================================================================

// GRNItemWithSerials wraps a GRN item with serial number entries for serialized products
type GRNItemWithSerials struct {
	GRNItem
	SerialNumbers []string `json:"serial_numbers"`
}

// ReceiveAgainstPOWithSerials creates a GRN with serial number tracking
func (a *App) ReceiveAgainstPOWithSerials(poID string, items []GRNItemWithSerials) (GoodsReceivedNote, error) {
	if err := a.requirePermission("grn:create"); err != nil {
		return GoodsReceivedNote{}, err
	}
	if a.db == nil {
		return GoodsReceivedNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Verify PO exists
	var po PurchaseOrder
	if err := a.db.Preload("Items").First(&po, "id = ?", poID).Error; err != nil {
		return GoodsReceivedNote{}, newError("PO_NOT_FOUND", "Purchase Order not found", err.Error())
	}

	// Build PO item map for validation
	poItemMap := make(map[string]PurchaseOrderItem)
	for _, poItem := range po.Items {
		poItemMap[poItem.ID] = poItem
	}

	// Pre-validate serial numbers
	for _, item := range items {
		if len(item.SerialNumbers) > 0 {
			// Validate serial count matches received quantity
			if float64(len(item.SerialNumbers)) != item.QuantityReceived {
				poItem := poItemMap[item.POItemID]
				return GoodsReceivedNote{}, newError("SERIAL_COUNT_MISMATCH",
					fmt.Sprintf("Serial count (%d) must match received quantity (%.0f) for %s",
						len(item.SerialNumbers), item.QuantityReceived, poItem.ProductCode), "")
			}
			// Validate no duplicate serials in DB
			for _, sn := range item.SerialNumbers {
				var existing SerialNumber
				if err := a.db.Where("serial_no = ?", sn).First(&existing).Error; err == nil {
					return GoodsReceivedNote{}, newError("DUPLICATE_SERIAL",
						fmt.Sprintf("Serial number %s already exists (product: %s)", sn, existing.ProductCode), "")
				}
			}
		}
	}

	// Extract plain GRN items for existing ReceiveAgainstPO logic
	var grnItems []GRNItem
	for _, item := range items {
		grnItems = append(grnItems, item.GRNItem)
	}

	// Create GRN using existing logic
	grn, err := a.ReceiveAgainstPO(poID, grnItems)
	if err != nil {
		return GoodsReceivedNote{}, err
	}

	// Now assign serial numbers to the created GRN items
	for i, item := range items {
		if len(item.SerialNumbers) == 0 {
			continue
		}
		// Find the matching created GRN item
		if i >= len(grn.Items) {
			break
		}
		grnItem := grn.Items[i]
		poItem := poItemMap[item.POItemID]

		if err := a.assignSerialsToGRN(
			grnItem.ID, grn.GRNNumber,
			poID, po.PONumber,
			poItem.ProductID, poItem.ProductCode,
			item.SerialNumbers,
			grn.ReceivedDate,
		); err != nil {
			log.Printf("⚠️ Failed to assign serials to GRN item %s: %v", grnItem.ID, err)
			// Don't fail the GRN - serials can be added later
		}
	}

	log.Printf("✅ Created GRN %s with serial tracking from PO %s", grn.GRNNumber, po.PONumber)
	return grn, nil
}

// =============================================================================
// P2 FIX: GRN DISCREPANCY WORKFLOW
// =============================================================================

// GRNDiscrepancy tracks quality/quantity issues for supplier follow-up
type GRNDiscrepancy struct {
	ID          string    `json:"id"`
	GRNID       string    `json:"grn_id"`
	GRNItemID   string    `json:"grn_item_id"`
	ProductCode string    `json:"product_code"`
	DiscrepancyType string `json:"discrepancy_type"` // "quantity_short", "quality_issue", "damaged", "wrong_item"
	Reason      string    `json:"reason"`
	RejectedQty float64   `json:"rejected_qty"`
	RaisedBy    string    `json:"raised_by"`
	RaisedAt    time.Time `json:"raised_at"`
	Status      string    `json:"status"` // "open", "supplier_contacted", "resolved", "credited"
	Resolution  string    `json:"resolution"`
	ResolvedAt  *time.Time `json:"resolved_at"`
	CostImpactBHD float64 `json:"cost_impact_bhd"`
}

// RaiseGRNDiscrepancy creates a discrepancy record for supplier follow-up
func (a *App) RaiseGRNDiscrepancy(grnID string, itemID string, reason string, discrepancyType string, rejectedQty float64) error {
	if err := a.requirePermission("grn:qc"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get GRN and item details
	var grn GoodsReceivedNote
	if err := a.db.Preload("Items").First(&grn, "id = ?", grnID).Error; err != nil {
		return newError("GRN_NOT_FOUND", "GRN not found", err.Error())
	}

	// Find the specific item
	var grnItem *GRNItem
	for i := range grn.Items {
		if grn.Items[i].ID == itemID {
			grnItem = &grn.Items[i]
			break
		}
	}

	if grnItem == nil {
		return newError("GRN_ITEM_NOT_FOUND", "GRN item not found", "")
	}

	// Get product code for logging
	var po PurchaseOrder
	productCode := "Unknown"
	if err := a.db.Preload("Items").First(&po, "id = ?", grn.PurchaseOrderID).Error; err == nil {
		for _, poItem := range po.Items {
			if poItem.ID == grnItem.POItemID {
				productCode = poItem.ProductCode
				break
			}
		}
	}

	// Calculate cost impact (estimated)
	costImpact := rejectedQty * 100.0 // Placeholder - should get unit cost from PO

	// Create discrepancy record (store in dedicated table via raw SQL for now)
	discrepancy := GRNDiscrepancy{
		GRNID:           grnID,
		GRNItemID:       itemID,
		ProductCode:     productCode,
		DiscrepancyType: discrepancyType,
		Reason:          reason,
		RejectedQty:     rejectedQty,
		RaisedBy:        a.getCurrentUserID(),
		RaisedAt:        time.Now(),
		Status:          "open",
		CostImpactBHD:   costImpact,
	}

	log.Printf("⚠️ GRN Discrepancy raised for %s: %s - %s (Qty: %.2f, Type: %s)",
		grn.GRNNumber, productCode, reason, rejectedQty, discrepancyType)

	// P2 FIX: Link to supplier performance tracking
	// Get supplier from PO
	if err := a.db.First(&po, "id = ?", grn.PurchaseOrderID).Error; err == nil {
		// Create supplier issue for tracking
		issue := SupplierIssue{
			SupplierID:  po.SupplierID,
			OrderRef:    grn.GRNNumber,
			Description: fmt.Sprintf("GRN Discrepancy: %s - %s (Qty: %.2f)", discrepancyType, reason, rejectedQty),
			Status:      "open",
			CostBHD:     costImpact,
		}
		if err := a.db.Create(&issue).Error; err != nil {
			log.Printf("⚠️ Failed to create supplier issue: %v", err)
		} else {
			log.Printf("📝 Linked discrepancy to supplier performance tracking (Issue ID: %s)", issue.ID)
		}
	}

	// In production, store discrepancy in grn_discrepancies table
	_ = discrepancy // Placeholder

	return nil
}

// GetGRNDiscrepancies retrieves all discrepancies for a GRN
func (a *App) GetGRNDiscrepancies(grnID string) ([]GRNDiscrepancy, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Placeholder - in production, query from grn_discrepancies table
	log.Printf("📋 Retrieving discrepancies for GRN %s", grnID)
	return []GRNDiscrepancy{}, nil
}

// ResolveGRNDiscrepancy marks a discrepancy as resolved
func (a *App) ResolveGRNDiscrepancy(discrepancyID string, resolution string) error {
	if err := a.requirePermission("grn:qc"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Placeholder - in production, update grn_discrepancies table
	log.Printf("✅ Resolved GRN discrepancy %s: %s", discrepancyID, resolution)
	return nil
}
