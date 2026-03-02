package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// =============================================================================
// DELIVERY NOTE CRUD - OPERATIONS PIPELINE
// =============================================================================

// CreateDeliveryNote creates a new delivery note
func (a *App) CreateDeliveryNote(dn DeliveryNote) (DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:create"); err != nil {
		return DeliveryNote{}, err
	}
	if a.db == nil {
		return DeliveryNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Validate order exists
	var order Order
	if err := a.db.First(&order, "id = ?", dn.OrderID).Error; err != nil {
		return DeliveryNote{}, newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Auto-generate DN number if not provided
	if dn.DNNumber == "" {
		dnNumber, err := a.GenerateDNNumber()
		if err != nil {
			return DeliveryNote{}, err
		}
		dn.DNNumber = dnNumber
	}

	// Validate positive quantities first (before DB-dependent checks)
	for _, item := range dn.Items {
		if item.QuantityDelivered <= 0 {
			return DeliveryNote{}, newError("INVALID_QUANTITY",
				fmt.Sprintf("Delivery quantity must be positive, got: %.2f", item.QuantityDelivered), "")
		}
	}

	// P3 DN-A6 FIX: Force initial status to "Prepared" regardless of input
	dn.Status = "Prepared"
	if dn.CustomerID == "" {
		dn.CustomerID = order.CustomerID
	}
	if dn.DeliveryDate.IsZero() {
		dn.DeliveryDate = time.Now()
	}
	dn.CreatedBy = a.getCurrentUserID()

	// P1 TOCTOU FIX: Wrap quantity validation + DN creation in a single transaction
	// with order row lock to prevent concurrent over-delivery
	err := a.db.Transaction(func(tx *gorm.DB) error {
		// Lock order row to serialize concurrent DN creation for same order
		var lockedOrder Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&lockedOrder, "id = ?", dn.OrderID).Error; err != nil {
			return newError("ORDER_LOCK_FAILED", "Failed to lock order for delivery validation", err.Error())
		}

		// Validate delivery item quantities won't go negative (inside transaction)
		if len(dn.Items) > 0 && dn.OrderID != "" {
			// Use direct query inside tx (not the exported method which has its own RBAC + limit)
			var existingDNs []DeliveryNote
			if err := tx.Where("order_id = ?", dn.OrderID).Preload("Items").Find(&existingDNs).Error; err != nil {
				return newError("DELIVERY_STATUS_CHECK_FAILED", "Failed to validate delivery quantities", err.Error())
			}

			// Calculate remaining quantities per order item
			deliveredQty := make(map[string]float64)
			for _, existingDN := range existingDNs {
				for _, item := range existingDN.Items {
					if item.OrderItemID != "" {
						deliveredQty[item.OrderItemID] += item.QuantityDelivered
					}
				}
			}

			// Get order items to know total ordered quantities
			var orderItems []OrderItem
			if err := tx.Where("order_id = ?", dn.OrderID).Find(&orderItems).Error; err != nil {
				return newError("ORDER_ITEMS_QUERY_FAILED", "Failed to retrieve order items", err.Error())
			}
			remainingQty := make(map[string]float64)
			for _, oi := range orderItems {
				remainingQty[oi.ID] = float64(oi.Quantity) - deliveredQty[oi.ID]
			}

			for _, item := range dn.Items {
				if item.OrderItemID != "" {
					remaining, ok := remainingQty[item.OrderItemID]
					if !ok {
						return newError("INVALID_ORDER_ITEM",
							fmt.Sprintf("Order item %s not found in order %s", item.OrderItemID, dn.OrderID), "")
					}
					if item.QuantityDelivered > remaining {
						return newError("QUANTITY_EXCEEDED",
							fmt.Sprintf("Cannot deliver %.2f units for %s - only %.2f remaining",
								item.QuantityDelivered, item.ProductCode, remaining), "")
					}
				}
			}
		}

		// Create delivery note inside the same transaction
		if err := tx.Create(&dn).Error; err != nil {
			return newError("DB_CREATE_FAILED", "Failed to create delivery note", err.Error())
		}
		return nil
	})
	if err != nil {
		return DeliveryNote{}, err
	}

	log.Printf("✅ Created DeliveryNote: %s for Order %s", dn.DNNumber, order.OrderNumber)
	return dn, nil
}

// GetDeliveryNotes retrieves all delivery notes
func (a *App) GetDeliveryNotes() ([]DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// S1 FIX: Cap results to prevent memory exhaustion (Preload("Items") is expensive)
	var deliveryNotes []DeliveryNote
	if err := a.db.Preload("Items").Order("delivery_date DESC, created_at DESC").Limit(200).Find(&deliveryNotes).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve delivery notes", err.Error())
	}

	log.Printf("📦 Retrieved %d delivery notes", len(deliveryNotes))
	return deliveryNotes, nil
}

// GetDeliveryNoteByID retrieves a single delivery note by ID
func (a *App) GetDeliveryNoteByID(id string) (DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return DeliveryNote{}, err
	}
	if a.db == nil {
		return DeliveryNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var dn DeliveryNote
	if err := a.db.Preload("Items").First(&dn, "id = ?", id).Error; err != nil {
		return DeliveryNote{}, newError("DN_NOT_FOUND", "Delivery note not found", err.Error())
	}

	log.Printf("📄 Retrieved DeliveryNote: %s", dn.DNNumber)
	return dn, nil
}

// GetDeliveryNotesByOrder retrieves all delivery notes for an order
func (a *App) GetDeliveryNotesByOrder(orderID string) ([]DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var deliveryNotes []DeliveryNote
	if err := a.db.Preload("Items").Where("order_id = ?", orderID).Order("delivery_sequence ASC").Limit(200).Find(&deliveryNotes).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve delivery notes", err.Error())
	}

	log.Printf("📦 Retrieved %d delivery notes for order %s", len(deliveryNotes), orderID)
	return deliveryNotes, nil
}

// GetDeliveryNotesByCustomer retrieves all delivery notes for a customer
func (a *App) GetDeliveryNotesByCustomer(customerID string) ([]DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var deliveryNotes []DeliveryNote
	if err := a.db.Preload("Items").Where("customer_id = ?", customerID).Order("delivery_date DESC").Limit(200).Find(&deliveryNotes).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve delivery notes", err.Error())
	}

	log.Printf("📦 Retrieved %d delivery notes for customer %s", len(deliveryNotes), customerID)
	return deliveryNotes, nil
}

// UpdateDeliveryNote updates an existing delivery note
func (a *App) UpdateDeliveryNote(dn DeliveryNote) (DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:update"); err != nil {
		return DeliveryNote{}, err
	}
	if a.db == nil {
		return DeliveryNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Verify exists
	var existing DeliveryNote
	if err := a.db.First(&existing, "id = ?", dn.ID).Error; err != nil {
		return DeliveryNote{}, newError("DN_NOT_FOUND", "Delivery note not found", err.Error())
	}

	// Status guard: only Prepared DNs can be updated (allowlist — blocks InTransit, Signed, Cancelled, Dispatched, Delivered)
	if existing.Status != "Prepared" {
		return DeliveryNote{}, newError("DN_INVALID_STATUS",
			fmt.Sprintf("Cannot update DN %s: status is %s (only Prepared delivery notes can be updated)", existing.DNNumber, existing.Status), "")
	}

	// Strip Status from input — status changes must go through DispatchDeliveryNote/ConfirmDeliveryNote
	dn.Status = ""

	// Update with transaction to handle items
	tx := a.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Update delivery note
	if err := tx.Model(&existing).Updates(dn).Error; err != nil {
		tx.Rollback()
		return DeliveryNote{}, newError("DB_UPDATE_FAILED", "Failed to update delivery note", err.Error())
	}

	// Only replace items if new items are explicitly provided
	if len(dn.Items) > 0 {
		// Delete existing items ONLY when we have replacements
		if err := tx.Where("delivery_note_id = ?", dn.ID).Delete(&DeliveryNoteItem{}).Error; err != nil {
			tx.Rollback()
			return DeliveryNote{}, newError("DB_UPDATE_FAILED", "Failed to delete old items", err.Error())
		}

		// Create new items
		for i := range dn.Items {
			dn.Items[i].DeliveryNoteID = dn.ID
			if err := tx.Create(&dn.Items[i]).Error; err != nil {
				tx.Rollback()
				return DeliveryNote{}, newError("DB_CREATE_FAILED", "Failed to create item", err.Error())
			}
		}
	}
	// If no items provided, keep existing items (don't delete)

	if err := tx.Commit().Error; err != nil {
		return DeliveryNote{}, newError("DB_COMMIT_FAILED", "Failed to commit update", err.Error())
	}

	// Reload with items
	var updated DeliveryNote
	if err := a.db.Preload("Items").First(&updated, "id = ?", dn.ID).Error; err != nil {
		return DeliveryNote{}, newError("DB_QUERY_FAILED", "Failed to reload delivery note", err.Error())
	}

	log.Printf("✅ Updated DeliveryNote: %s", updated.DNNumber)
	return updated, nil
}

// DeleteDeliveryNote deletes a delivery note (soft delete)
func (a *App) DeleteDeliveryNote(id string) error {
	if err := a.requirePermission("delivery_notes:delete"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var dn DeliveryNote
	if err := a.db.First(&dn, "id = ?", id).Error; err != nil {
		return newError("DN_NOT_FOUND", "Delivery note not found", err.Error())
	}

	// Status guard: only Prepared DNs can be deleted (serial integrity)
	if dn.Status != "Prepared" {
		return newError("DN_INVALID_STATUS",
			fmt.Sprintf("Cannot delete DN %s: status is %s (only Prepared delivery notes can be deleted)", dn.DNNumber, dn.Status), "")
	}

	// P2 FIX: Release allocated serials before soft-deleting DN
	// Serials pointing to this DN should return to Available status
	if err := a.db.Model(&SerialNumber{}).
		Where("dn_number = ? AND status = ?", dn.DNNumber, "Reserved").
		Updates(map[string]interface{}{
			"status":        "Available",
			"dn_number":     "",
			"dn_item_id":    "",
			"customer_id":   "",
			"customer_name": "",
		}).Error; err != nil {
		log.Printf("Warning: Failed to release serials for DN %s: %v", dn.DNNumber, err)
		// Continue with deletion — serial cleanup is best-effort
	}

	// Soft delete (GORM will handle DeletedAt)
	if err := a.db.Delete(&dn).Error; err != nil {
		return newError("DB_DELETE_FAILED", "Failed to delete delivery note", err.Error())
	}

	log.Printf("🗑️ Deleted DeliveryNote: %s", dn.DNNumber)
	return nil
}

// GenerateDNNumber generates a new DN number in format DN-2026-0001
// S4 FIX: Rewritten to use InvoiceSequence table (same pattern as GenerateInvoiceNumber/GenerateCreditNoteNumber)
// Eliminates nested BEGIN EXCLUSIVE bug and COUNT-based fragility
func (a *App) GenerateDNNumber() (string, error) {
	if a.db == nil {
		return "", newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	now := time.Now()
	year := now.Year()
	prefix := "DN"

	var dnNumber string
	err := a.db.Transaction(func(tx *gorm.DB) error {
		var seq InvoiceSequence
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("prefix = ? AND year = ?", prefix, year).
			First(&seq).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// First DN of the year — seed from existing DN count for migration safety
				var maxExisting int64
				tx.Model(&DeliveryNote{}).
					Where("dn_number LIKE ?", fmt.Sprintf("DN-%d-%%", year)).
					Count(&maxExisting)

				seq = InvoiceSequence{
					Prefix:     prefix,
					Year:       year,
					LastNumber: int(maxExisting),
					UpdatedAt:  now,
				}
				if err := tx.Create(&seq).Error; err != nil {
					return fmt.Errorf("failed to create DN sequence: %w", err)
				}
			} else {
				return fmt.Errorf("failed to get DN sequence: %w", err)
			}
		}

		seq.LastNumber++
		seq.UpdatedAt = now
		if err := tx.Save(&seq).Error; err != nil {
			return fmt.Errorf("failed to update DN sequence: %w", err)
		}

		dnNumber = fmt.Sprintf("%s-%d-%04d", prefix, year, seq.LastNumber)
		return nil
	})

	if err != nil {
		return "", err
	}
	log.Printf("🔢 Generated DN Number: %s", dnNumber)
	return dnNumber, nil
}

// =============================================================================
// DELIVERY NOTE STATUS UPDATES
// =============================================================================

// DispatchDeliveryNote marks a delivery note as dispatched
func (a *App) DispatchDeliveryNote(id string, driverName string, vehicleNumber string) error {
	if err := a.requirePermission("delivery_notes:dispatch"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var dn DeliveryNote
	if err := a.db.First(&dn, "id = ?", id).Error; err != nil {
		return newError("DN_NOT_FOUND", "Delivery note not found", err.Error())
	}

	// P3 DN-A3 FIX: Reject empty status — cannot dispatch a DN with no status set
	if dn.Status == "" {
		return newError("DN_INVALID_STATUS",
			fmt.Sprintf("Delivery note %s has no status set — cannot dispatch", dn.DNNumber), "")
	}

	// S2 FIX: Only Prepared DNs can be dispatched — prevent status regression
	if dn.Status != "Prepared" {
		return newError("DN_INVALID_STATUS",
			fmt.Sprintf("Cannot dispatch DN %s: status is %s (only Prepared delivery notes can be dispatched)", dn.DNNumber, dn.Status), "")
	}

	// P2 FIX: Wrap DN status + serial status updates in a single transaction for atomicity
	now := time.Now()
	err := a.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":         "Dispatched",
			"driver_name":    driverName,
			"vehicle_number": vehicleNumber,
		}
		if err := tx.Model(&dn).Updates(updates).Error; err != nil {
			return newError("DB_UPDATE_FAILED", "Failed to dispatch delivery note", err.Error())
		}

		// Update serial statuses to "Shipped" + set shipped_date — inside same transaction
		if err := tx.Model(&SerialNumber{}).
			Where("dn_number = ? AND UPPER(status) IN ?", dn.DNNumber, []string{"RESERVED"}).
			Updates(map[string]interface{}{
				"status":       "Shipped",
				"shipped_date": now,
			}).Error; err != nil {
			return fmt.Errorf("failed to update serial status for dispatch: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Printf("🚚 Dispatched DN %s with driver %s, vehicle %s", dn.DNNumber, driverName, vehicleNumber)
	return nil
}

// ConfirmDeliveryNote marks a delivery note as delivered and signed
func (a *App) ConfirmDeliveryNote(id string, signedBy string) error {
	if err := a.requirePermission("delivery_notes:confirm"); err != nil {
		return err
	}
	if a.db == nil {
		return newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	var dn DeliveryNote
	if err := a.db.First(&dn, "id = ?", id).Error; err != nil {
		return newError("DN_NOT_FOUND", "Delivery note not found", err.Error())
	}

	// S3 FIX: Only Dispatched DNs can be confirmed — prevent re-confirmation and status regression
	if dn.Status != "Dispatched" {
		return newError("DN_INVALID_STATUS",
			fmt.Sprintf("Cannot confirm DN %s: status is %s (only Dispatched delivery notes can be confirmed)", dn.DNNumber, dn.Status), "")
	}

	// P2 FIX: Wrap DN status + serial status updates in a single transaction for atomicity
	now := time.Now()
	err := a.db.Transaction(func(tx *gorm.DB) error {
		updates := map[string]interface{}{
			"status":    "Delivered",
			"signed_by": signedBy,
			"signed_at": now,
		}
		if err := tx.Model(&dn).Updates(updates).Error; err != nil {
			return newError("DB_UPDATE_FAILED", "Failed to confirm delivery", err.Error())
		}

		// Mark serials as Delivered, set warranty start + end dates — inside same transaction
		// Must iterate per serial to calculate per-product warranty_end_date (varies by WarrantyMonths)
		var serials []SerialNumber
		if err := tx.Where("dn_number = ? AND UPPER(status) IN ?", dn.DNNumber, []string{"RESERVED", "SHIPPED"}).
			Find(&serials).Error; err != nil {
			return fmt.Errorf("failed to find serials for delivery: %w", err)
		}
		for _, serial := range serials {
			serialUpdates := map[string]interface{}{
				"status":              "Delivered",
				"warranty_start_date": now,
			}
			if serial.WarrantyMonths > 0 {
				serialUpdates["warranty_end_date"] = now.AddDate(0, serial.WarrantyMonths, 0)
			}
			if err := tx.Model(&serial).Updates(serialUpdates).Error; err != nil {
				return fmt.Errorf("failed to update serial %s: %w", serial.SerialNo, err)
			}
		}

		return nil
	})
	if err != nil {
		return err
	}

	// E1 Enhancement: Update QuantityShipped on order items and propagate order status
	if err := a.confirmDeliveryAndUpdateOrder(dn); err != nil {
		log.Printf("Warning: Failed to update order items after delivery confirmation: %v", err)
	}

	// Also run the existing order status checks for backward compatibility
	if err := a.updateOrderDeliveryStatus(dn.OrderID); err != nil {
		log.Printf("Warning: Failed to update order status: %v", err)
	}

	if err := a.ProgressOrderOnDelivery(dn.OrderID); err != nil {
		log.Printf("Warning: Failed to progress order on delivery: %v", err)
	}

	log.Printf("Confirmed delivery of DN %s, signed by %s", dn.DNNumber, signedBy)
	return nil
}

// updateOrderDeliveryStatus checks if all order items are delivered and updates order status
func (a *App) updateOrderDeliveryStatus(orderID string) error {
	// Get all delivery notes for this order
	deliveryNotes, err := a.GetDeliveryNotesByOrder(orderID)
	if err != nil {
		return err
	}

	// Check if all are delivered
	allDelivered := true
	for _, dn := range deliveryNotes {
		if dn.Status != "Delivered" {
			allDelivered = false
			break
		}
	}

	// Update order status if all delivered
	if allDelivered {
		var order Order
		if err := a.db.First(&order, "id = ?", orderID).Error; err != nil {
			return newError("ORDER_NOT_FOUND", "Order not found", err.Error())
		}

		if err := a.db.Model(&order).Update("status", "Delivered").Error; err != nil {
			return newError("DB_UPDATE_FAILED", "Failed to update order status", err.Error())
		}

		log.Printf("✅ Order %s marked as Delivered (all DNs delivered)", order.OrderNumber)
	}

	return nil
}

// =============================================================================
// DELIVERY NOTE CREATION FROM ORDER
// =============================================================================

// CreateDNFromOrder creates a delivery note from an order with specified items
func (a *App) CreateDNFromOrder(orderID string, items []DeliveryNoteItem) (DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:create"); err != nil {
		return DeliveryNote{}, err
	}
	if a.db == nil {
		return DeliveryNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get order with items
	var order Order
	if err := a.db.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
		return DeliveryNote{}, newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Get existing delivery notes to calculate sequence
	existingDNs, err := a.GetDeliveryNotesByOrder(orderID)
	if err != nil {
		return DeliveryNote{}, err
	}

	// Calculate delivery status to validate quantities
	remainingQty, err := a.GetOrderDeliveryStatus(orderID)
	if err != nil {
		return DeliveryNote{}, err
	}

	// P1 FIX: Validate delivery quantities don't exceed order quantities
	orderItemMap := make(map[string]OrderItem)
	for _, orderItem := range order.Items {
		orderItemMap[orderItem.ID] = orderItem
	}

	for _, item := range items {
		// Check remaining quantity
		remaining, ok := remainingQty[item.OrderItemID]
		if !ok {
			return DeliveryNote{}, newError("INVALID_ITEM", fmt.Sprintf("Order item %s not found", item.OrderItemID), "")
		}

		// P1 FIX: Validate delivery quantity doesn't exceed remaining
		if item.QuantityDelivered > remaining {
			orderItem := orderItemMap[item.OrderItemID]
			return DeliveryNote{}, newError("QUANTITY_EXCEEDED",
				fmt.Sprintf("Cannot deliver %.2f units for %s - only %.2f remaining (ordered: %.2f)",
					item.QuantityDelivered, orderItem.ProductCode, remaining, orderItem.Quantity), "")
		}

		// Additional validation: delivery quantity must be positive
		if item.QuantityDelivered <= 0 {
			return DeliveryNote{}, newError("INVALID_QUANTITY",
				fmt.Sprintf("Delivery quantity must be positive, got: %.2f", item.QuantityDelivered), "")
		}
	}

	// Determine if this is a partial delivery
	isPartial := false
	allItemsFullyDelivered := true
	for _, item := range items {
		remaining := remainingQty[item.OrderItemID]
		afterThisDelivery := remaining - item.QuantityDelivered
		if afterThisDelivery > 0.001 { // tolerance for floating point
			allItemsFullyDelivered = false
			isPartial = true
			break
		}
	}

	// Create delivery note
	dn := DeliveryNote{
		OrderID:           orderID,
		CustomerID:        order.CustomerID,
		DeliveryDate:      time.Now(),
		Status:            "Prepared",
		IsPartialDelivery: isPartial,
		DeliverySequence:  len(existingDNs) + 1,
	}

	// Calculate total deliveries (estimated)
	if isPartial {
		dn.TotalDeliveries = 0 // Unknown until all delivered
	} else {
		dn.TotalDeliveries = len(existingDNs) + 1
	}

	// Populate items with calculated remaining quantities
	dn.Items = make([]DeliveryNoteItem, len(items))
	for i, item := range items {
		remaining := remainingQty[item.OrderItemID]
		dn.Items[i] = DeliveryNoteItem{
			OrderItemID:       item.OrderItemID,
			ProductID:         item.ProductID,
			ProductCode:       item.ProductCode,
			Description:       item.Description,
			QuantityOrdered:   item.QuantityOrdered,
			QuantityDelivered: item.QuantityDelivered,
			QuantityRemaining: remaining - item.QuantityDelivered,
		}
	}

	// Create the delivery note
	createdDN, err := a.CreateDeliveryNote(dn)
	if err != nil {
		return DeliveryNote{}, err
	}

	// P1 FIX: Update order status based on delivery completeness
	newOrderStatus := "Shipping"
	if isPartial {
		newOrderStatus = "Partially Delivered"
		log.Printf("📦 Partial delivery for Order %s - not all items delivered", order.OrderNumber)
	} else if allItemsFullyDelivered {
		// Check if this was the final delivery
		allDelivered := true
		for _, remaining := range remainingQty {
			if remaining > 0.001 {
				allDelivered = false
				break
			}
		}
		if allDelivered {
			newOrderStatus = "Delivered"
			log.Printf("✅ Full delivery complete for Order %s", order.OrderNumber)
		}
	}

	if err := a.db.Model(&order).Update("status", newOrderStatus).Error; err != nil {
		log.Printf("⚠️ Failed to update order status: %v", err)
		// Don't fail the creation, just log
	}

	log.Printf("✅ Created DN %s from Order %s (%d items, partial=%v, status=%s)",
		createdDN.DNNumber, order.OrderNumber, len(items), isPartial, newOrderStatus)
	return createdDN, nil
}

// GetOrderDeliveryStatus calculates remaining quantities for each order item
func (a *App) GetOrderDeliveryStatus(orderID string) (map[string]float64, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get order items
	var order Order
	if err := a.db.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
		return nil, newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Initialize remaining quantities from order
	remainingQty := make(map[string]float64)
	for _, item := range order.Items {
		remainingQty[item.ID] = item.Quantity
	}

	// Get all delivery notes for this order
	deliveryNotes, err := a.GetDeliveryNotesByOrder(orderID)
	if err != nil {
		return nil, err
	}

	// Subtract delivered quantities
	for _, dn := range deliveryNotes {
		for _, item := range dn.Items {
			if current, ok := remainingQty[item.OrderItemID]; ok {
				remainingQty[item.OrderItemID] = current - item.QuantityDelivered
			}
		}
	}

	log.Printf("📊 Order %s delivery status: %d items tracked", orderID, len(remainingQty))
	return remainingQty, nil
}

// =============================================================================
// P2 FIX: DELIVERY ROUTE OPTIMIZATION & PLANNING
// =============================================================================

// DeliveryPlanningItem represents a delivery with urgency and location info
type DeliveryPlanningItem struct {
	DeliveryNote
	CustomerArea    string  `json:"customer_area"`
	UrgencyScore    float64 `json:"urgency_score"` // 0-1, higher = more urgent
	DaysUntilDue    int     `json:"days_until_due"`
	OrderValue      float64 `json:"order_value"`
	IsHighPriority  bool    `json:"is_high_priority"`
}

// GetDeliveriesByArea groups pending deliveries by customer area for route optimization
func (a *App) GetDeliveriesByArea(area string) ([]DeliveryPlanningItem, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get pending/dispatched deliveries
	var deliveryNotes []DeliveryNote
	query := a.db.Preload("Items").Where("status IN (?)", []string{"Prepared", "Dispatched"})

	if area != "" && area != "All" {
		// Join with customers to filter by city/area
		query = query.Joins("JOIN customers ON delivery_notes.customer_id = customers.id").
			Where("customers.city = ?", area)
	}

	if err := query.Order("delivery_date ASC").Limit(500).Find(&deliveryNotes).Error; err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to retrieve deliveries", err.Error())
	}

	// Enrich with planning data
	planningItems := make([]DeliveryPlanningItem, len(deliveryNotes))
	for i, dn := range deliveryNotes {
		// Get customer area
		var customer CustomerMaster
		customerArea := "Unknown"
		if err := a.db.First(&customer, "id = ?", dn.CustomerID).Error; err == nil {
			customerArea = customer.City
		}

		// Get order value for prioritization
		var order Order
		orderValue := 0.0
		if err := a.db.First(&order, "id = ?", dn.OrderID).Error; err == nil {
			orderValue = order.TotalValueBHD
		}

		// Calculate urgency (based on delivery date)
		daysUntilDue := int(dn.DeliveryDate.Sub(time.Now()).Hours() / 24)
		urgencyScore := 1.0
		if daysUntilDue <= 0 {
			urgencyScore = 1.0 // Overdue
		} else if daysUntilDue <= 2 {
			urgencyScore = 0.9 // Very urgent
		} else if daysUntilDue <= 5 {
			urgencyScore = 0.7 // Urgent
		} else {
			urgencyScore = 0.5 // Normal
		}

		// High priority customers
		isHighPriority := customer.CustomerGrade == "A" || orderValue > 10000.0

		planningItems[i] = DeliveryPlanningItem{
			DeliveryNote:   dn,
			CustomerArea:   customerArea,
			UrgencyScore:   urgencyScore,
			DaysUntilDue:   daysUntilDue,
			OrderValue:     orderValue,
			IsHighPriority: isHighPriority,
		}
	}

	log.Printf("📍 Retrieved %d deliveries for area: %s", len(planningItems), area)
	return planningItems, nil
}

// GetPendingDeliveries retrieves all pending deliveries sorted by urgency
func (a *App) GetPendingDeliveries() ([]DeliveryPlanningItem, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Get all pending deliveries
	deliveries, err := a.GetDeliveriesByArea("All")
	if err != nil {
		return nil, err
	}

	// Sort by urgency score (descending) then by order value (descending)
	// Most urgent and highest value first
	for i := 0; i < len(deliveries)-1; i++ {
		for j := i + 1; j < len(deliveries); j++ {
			// Primary sort: urgency
			if deliveries[j].UrgencyScore > deliveries[i].UrgencyScore {
				deliveries[i], deliveries[j] = deliveries[j], deliveries[i]
			} else if deliveries[j].UrgencyScore == deliveries[i].UrgencyScore {
				// Secondary sort: order value
				if deliveries[j].OrderValue > deliveries[i].OrderValue {
					deliveries[i], deliveries[j] = deliveries[j], deliveries[i]
				}
			}
		}
	}

	log.Printf("📋 Retrieved %d pending deliveries sorted by urgency", len(deliveries))
	return deliveries, nil
}

// GetDeliveryAreaSummary returns delivery counts grouped by area
func (a *App) GetDeliveryAreaSummary() (map[string]int, error) {
	if err := a.requirePermission("delivery_notes:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Query delivery counts by customer city
	type AreaCount struct {
		Area  string
		Count int
	}

	var results []AreaCount
	err := a.db.Raw(`
		SELECT c.city AS area, COUNT(dn.id) AS count
		FROM delivery_notes dn
		JOIN customers c ON dn.customer_id = c.id
		WHERE dn.status IN ('Prepared', 'Dispatched')
		GROUP BY c.city
		ORDER BY count DESC
	`).Scan(&results).Error

	if err != nil {
		return nil, newError("DB_QUERY_FAILED", "Failed to get area summary", err.Error())
	}

	// Convert to map
	summary := make(map[string]int)
	for _, r := range results {
		summary[r.Area] = r.Count
	}

	log.Printf("📊 Delivery area summary: %d areas with pending deliveries", len(summary))
	return summary, nil
}

// =============================================================================
// E1: PARTIAL DELIVERY ENHANCEMENT - FULFILLMENT DETAIL & ITEM-LEVEL CREATION
// =============================================================================

// OrderFulfillmentItem represents per-item delivery breakdown for an order
type OrderFulfillmentItem struct {
	OrderItemID  string  `json:"order_item_id"`
	ProductCode  string  `json:"product_code"`
	Description  string  `json:"description"`
	OrderedQty   float64 `json:"ordered_qty"`
	ShippedQty   float64 `json:"shipped_qty"`
	DeliveredQty float64 `json:"delivered_qty"`
	InvoicedQty  float64 `json:"invoiced_qty"`
	RemainingQty float64 `json:"remaining_qty"`
}

// OrderFulfillment represents the full delivery fulfillment status for an order
type OrderFulfillment struct {
	OrderID        string                 `json:"order_id"`
	OrderNumber    string                 `json:"order_number"`
	CustomerName   string                 `json:"customer_name"`
	Status         string                 `json:"status"`
	Items          []OrderFulfillmentItem `json:"items"`
	FullyDelivered bool                   `json:"fully_delivered"`
	FullyInvoiced  bool                   `json:"fully_invoiced"`
}

// GetOrderFulfillmentDetail returns per-item delivery breakdown for an order
func (a *App) GetOrderFulfillmentDetail(orderID string) (OrderFulfillment, error) {
	if err := a.requirePermission("orders:view"); err != nil {
		return OrderFulfillment{}, err
	}
	if a.db == nil {
		return OrderFulfillment{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Load order with items
	var order Order
	if err := a.db.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
		return OrderFulfillment{}, newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Get all delivery notes for this order to calculate delivered quantities
	var deliveryNotes []DeliveryNote
	if err := a.db.Preload("Items").Where("order_id = ?", orderID).Find(&deliveryNotes).Error; err != nil {
		return OrderFulfillment{}, newError("DB_QUERY_FAILED", "Failed to retrieve delivery notes", err.Error())
	}

	// Calculate delivered quantities per order item from all DNs
	deliveredQtyMap := make(map[string]float64)
	for _, dn := range deliveryNotes {
		for _, item := range dn.Items {
			deliveredQtyMap[item.OrderItemID] += item.QuantityDelivered
		}
	}

	// Build fulfillment items
	fulfillment := OrderFulfillment{
		OrderID:      orderID,
		OrderNumber:  order.OrderNumber,
		CustomerName: order.CustomerName,
		Status:       order.Status,
	}

	fullyDelivered := true
	fullyInvoiced := true

	for _, item := range order.Items {
		deliveredQty := deliveredQtyMap[item.ID]
		remainingQty := item.Quantity - deliveredQty
		if remainingQty < 0.001 {
			remainingQty = 0
		}

		fi := OrderFulfillmentItem{
			OrderItemID:  item.ID,
			ProductCode:  item.ProductCode,
			Description:  item.Description,
			OrderedQty:   item.Quantity,
			ShippedQty:   item.QuantityShipped,
			DeliveredQty: deliveredQty,
			InvoicedQty:  item.QuantityInvoiced,
			RemainingQty: remainingQty,
		}
		fulfillment.Items = append(fulfillment.Items, fi)

		if remainingQty > 0.001 {
			fullyDelivered = false
		}
		if item.QuantityInvoiced < item.Quantity-0.001 {
			fullyInvoiced = false
		}
	}

	fulfillment.FullyDelivered = fullyDelivered
	fulfillment.FullyInvoiced = fullyInvoiced

	log.Printf("📊 Order %s fulfillment: %d items, fully_delivered=%v, fully_invoiced=%v",
		order.OrderNumber, len(fulfillment.Items), fullyDelivered, fullyInvoiced)
	return fulfillment, nil
}

// DeliveryNoteItemInput is a simplified input for creating delivery notes with per-item quantities
type DeliveryNoteItemInput struct {
	OrderItemID string  `json:"order_item_id"`
	ShipQty     float64 `json:"ship_qty"`
}

// CreateDeliveryNoteWithItems creates a delivery note with per-item ship quantities
// This is the preferred way to create a DN from an order with partial delivery support
func (a *App) CreateDeliveryNoteWithItems(orderID string, items []DeliveryNoteItemInput) (DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:create"); err != nil {
		return DeliveryNote{}, err
	}
	if a.db == nil {
		return DeliveryNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	if len(items) == 0 {
		return DeliveryNote{}, newError("NO_ITEMS", "At least one item is required", "")
	}

	// Load order with items
	var order Order
	if err := a.db.Preload("Items").First(&order, "id = ?", orderID).Error; err != nil {
		return DeliveryNote{}, newError("ORDER_NOT_FOUND", "Order not found", err.Error())
	}

	// Build order item lookup
	orderItemMap := make(map[string]OrderItem)
	for _, oi := range order.Items {
		orderItemMap[oi.ID] = oi
	}

	// Get remaining quantities
	remainingQty, err := a.GetOrderDeliveryStatus(orderID)
	if err != nil {
		return DeliveryNote{}, err
	}

	// Get existing DN count for sequence
	var existingDNCount int64
	a.db.Model(&DeliveryNote{}).Where("order_id = ?", orderID).Count(&existingDNCount)

	// Validate and build DN items
	var dnItems []DeliveryNoteItem
	allFullyDelivered := true

	for _, input := range items {
		if input.ShipQty <= 0 {
			return DeliveryNote{}, newError("INVALID_QUANTITY",
				fmt.Sprintf("Ship quantity must be positive, got: %.2f", input.ShipQty), "")
		}

		orderItem, ok := orderItemMap[input.OrderItemID]
		if !ok {
			return DeliveryNote{}, newError("INVALID_ITEM",
				fmt.Sprintf("Order item %s not found in order", input.OrderItemID), "")
		}

		remaining, ok := remainingQty[input.OrderItemID]
		if !ok {
			remaining = orderItem.Quantity
		}

		if input.ShipQty > remaining+0.001 {
			return DeliveryNote{}, newError("QUANTITY_EXCEEDED",
				fmt.Sprintf("Cannot ship %.2f units for %s - only %.2f remaining",
					input.ShipQty, orderItem.ProductCode, remaining), "")
		}

		afterDelivery := remaining - input.ShipQty
		if afterDelivery > 0.001 {
			allFullyDelivered = false
		}

		dnItems = append(dnItems, DeliveryNoteItem{
			OrderItemID:       input.OrderItemID,
			ProductID:         orderItem.ProductID,
			ProductCode:       orderItem.ProductCode,
			Description:       orderItem.Description,
			QuantityOrdered:   orderItem.Quantity,
			QuantityDelivered: input.ShipQty,
			QuantityRemaining: afterDelivery,
		})
	}

	// Check if ALL order items will be fully delivered (not just the items in this DN)
	// We need to check items NOT included in this DN too
	for _, orderItem := range order.Items {
		found := false
		for _, input := range items {
			if input.OrderItemID == orderItem.ID {
				found = true
				break
			}
		}
		if !found {
			// This item is not in the DN - check if it's already fully delivered
			remaining, ok := remainingQty[orderItem.ID]
			if !ok || remaining > 0.001 {
				allFullyDelivered = false
			}
		}
	}

	isPartial := !allFullyDelivered

	// Generate DN number
	dnNumber, err := a.GenerateDNNumber()
	if err != nil {
		return DeliveryNote{}, err
	}

	// Create delivery note
	dn := DeliveryNote{
		OrderID:           orderID,
		CustomerID:        order.CustomerID,
		DNNumber:          dnNumber,
		DeliveryDate:      time.Now(),
		Status:            "Prepared",
		IsPartialDelivery: isPartial,
		DeliverySequence:  int(existingDNCount) + 1,
		TotalDeliveries:   0, // Unknown until all delivered
		Items:             dnItems,
	}
	dn.CreatedBy = a.getCurrentUserID()

	if !isPartial {
		dn.TotalDeliveries = int(existingDNCount) + 1
	}

	// TOCTOU FIX: Wrap DN creation + quantity re-validation in a single transaction
	// to prevent concurrent over-delivery between GetOrderDeliveryStatus and Create
	err = a.db.Transaction(func(tx *gorm.DB) error {
		// Re-check remaining quantities inside transaction to prevent TOCTOU
		var existingDNs []DeliveryNote
		if err := tx.Where("order_id = ?", orderID).Preload("Items").Find(&existingDNs).Error; err != nil {
			return newError("DELIVERY_STATUS_CHECK_FAILED", "Failed to re-validate delivery quantities", err.Error())
		}
		deliveredQty := make(map[string]float64)
		for _, existingDN := range existingDNs {
			for _, item := range existingDN.Items {
				if item.OrderItemID != "" {
					deliveredQty[item.OrderItemID] += item.QuantityDelivered
				}
			}
		}
		for _, input := range items {
			oi, ok := orderItemMap[input.OrderItemID]
			if !ok {
				continue
			}
			currentRemaining := oi.Quantity - deliveredQty[input.OrderItemID]
			if input.ShipQty > currentRemaining+0.001 {
				return newError("QUANTITY_EXCEEDED",
					fmt.Sprintf("Concurrent delivery detected: cannot ship %.2f units for %s - only %.2f remaining",
						input.ShipQty, oi.ProductCode, currentRemaining), "")
			}
		}

		// Create DN inside transaction
		if err := tx.Create(&dn).Error; err != nil {
			return newError("DB_CREATE_FAILED", "Failed to create delivery note", err.Error())
		}

		// Update order status inside same transaction
		newStatus := "Shipping"
		if isPartial {
			newStatus = "Partially Delivered"
		} else {
			newStatus = "Shipped"
		}
		if err := tx.Model(&order).Update("status", newStatus).Error; err != nil {
			log.Printf("Warning: Failed to update order status: %v", err)
		}

		return nil
	})
	if err != nil {
		return DeliveryNote{}, err
	}

	log.Printf("Created DN %s from Order %s (%d items, partial=%v)",
		dn.DNNumber, order.OrderNumber, len(items), isPartial)
	return dn, nil
}

// =============================================================================
// CREATE DN WITH SERIAL ALLOCATION (Phase 23)
// =============================================================================

// DNItemInputWithSerials wraps DN item input with optional serial number allocation
type DNItemInputWithSerials struct {
	OrderItemID string   `json:"order_item_id"`
	ShipQty     float64  `json:"ship_qty"`
	SerialNos   []string `json:"serial_nos"`
}

// CreateDNWithSerials creates a delivery note with serial number allocation
func (a *App) CreateDNWithSerials(orderID string, items []DNItemInputWithSerials) (DeliveryNote, error) {
	if err := a.requirePermission("delivery_notes:create"); err != nil {
		return DeliveryNote{}, err
	}
	if a.db == nil {
		return DeliveryNote{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	// Pre-validate serial count matches ship qty (availability is checked atomically in allocateSerialsToDN)
	// S6 FIX: Removed TOCTOU-prone availability pre-check — allocateSerialsToDN uses atomic UPDATE WHERE
	for _, item := range items {
		if len(item.SerialNos) > 0 {
			if float64(len(item.SerialNos)) != item.ShipQty {
				return DeliveryNote{}, newError("SERIAL_COUNT_MISMATCH",
					fmt.Sprintf("Serial count (%d) must match ship qty (%.0f) for order item %s",
						len(item.SerialNos), item.ShipQty, item.OrderItemID), "")
			}
		}
	}

	// Convert to plain DN items for existing logic
	var plainItems []DeliveryNoteItemInput
	for _, item := range items {
		plainItems = append(plainItems, DeliveryNoteItemInput{
			OrderItemID: item.OrderItemID,
			ShipQty:     item.ShipQty,
		})
	}

	// Create DN using existing logic
	dn, err := a.CreateDeliveryNoteWithItems(orderID, plainItems)
	if err != nil {
		return DeliveryNote{}, err
	}

	// P2 FIX: If post-creation steps fail, clean up the orphaned DN
	// Helper to soft-delete DN and release any allocated serials on failure
	cleanupDN := func(reason string) {
		log.Printf("Warning: Cleaning up orphaned DN %s: %s", dn.DNNumber, reason)
		// Release any serials that may have been allocated
		a.db.Model(&SerialNumber{}).
			Where("dn_number = ? AND status = ?", dn.DNNumber, "Reserved").
			Updates(map[string]interface{}{
				"status":        "Available",
				"dn_number":     "",
				"dn_item_id":    "",
				"customer_id":   "",
				"customer_name": "",
			})
		// Soft-delete the orphaned DN
		a.db.Delete(&dn)
	}

	// Load customer for serial allocation
	var order Order
	if err := a.db.Where("id = ?", orderID).First(&order).Error; err != nil {
		cleanupDN("order lookup failed")
		return DeliveryNote{}, newError("ORDER_NOT_FOUND", "Order not found for serial allocation", err.Error())
	}
	var customer CustomerMaster
	if err := a.db.Where("id = ?", order.CustomerID).First(&customer).Error; err != nil {
		cleanupDN("customer lookup failed")
		return DeliveryNote{}, newError("CUSTOMER_NOT_FOUND", "Customer not found for serial allocation", err.Error())
	}

	// Allocate serials to DN items
	for i, item := range items {
		if len(item.SerialNos) == 0 {
			continue
		}
		if i >= len(dn.Items) {
			break
		}
		dnItem := dn.Items[i]

		if err := a.allocateSerialsToDN(
			dnItem.ID, dn.DNNumber,
			customer.ID, customer.BusinessName,
			item.SerialNos,
		); err != nil {
			cleanupDN(fmt.Sprintf("serial allocation failed for item %s: %v", dnItem.ID, err))
			return DeliveryNote{}, newError("SERIAL_ALLOCATION_FAILED",
				fmt.Sprintf("Failed to allocate serials to DN item %s: %v", dnItem.ID, err), "")
		}
	}

	log.Printf("Created DN %s with serial allocation from Order %s", dn.DNNumber, order.OrderNumber)
	return dn, nil
}

// confirmDeliveryAndUpdateOrder is called by ConfirmDeliveryNote to update QuantityShipped on order items
func (a *App) confirmDeliveryAndUpdateOrder(dn DeliveryNote) error {
	return a.db.Transaction(func(tx *gorm.DB) error {
		// Load the full DN with items if not already loaded
		var fullDN DeliveryNote
		if err := tx.Preload("Items").First(&fullDN, "id = ?", dn.ID).Error; err != nil {
			return newError("DN_NOT_FOUND", "Delivery note not found for order update", err.Error())
		}

		// Update QuantityShipped on each order item based on DN items
		for _, dnItem := range fullDN.Items {
			if dnItem.OrderItemID == "" {
				continue
			}

			var orderItem OrderItem
			if err := tx.First(&orderItem, "id = ?", dnItem.OrderItemID).Error; err != nil {
				log.Printf("Warning: Could not find order item %s for DN update: %v", dnItem.OrderItemID, err)
				continue
			}

			// Recalculate total shipped from ALL confirmed DNs for this order item
			var totalDelivered float64
			err := tx.Model(&DeliveryNoteItem{}).
				Joins("JOIN delivery_notes ON delivery_note_items.delivery_note_id = delivery_notes.id").
				Where("delivery_note_items.order_item_id = ? AND delivery_notes.status = ?", dnItem.OrderItemID, "Delivered").
				Select("COALESCE(SUM(delivery_note_items.quantity_delivered), 0)").
				Row().Scan(&totalDelivered)
			if err != nil {
				log.Printf("Warning: Could not calculate total delivered for order item %s: %v", dnItem.OrderItemID, err)
				continue
			}

			// Update QuantityShipped to match total confirmed deliveries
			if err := tx.Model(&orderItem).Update("quantity_shipped", totalDelivered).Error; err != nil {
				log.Printf("Warning: Failed to update QuantityShipped for order item %s: %v", dnItem.OrderItemID, err)
			}
		}

		// Now update order status based on delivery completeness
		var order Order
		if err := tx.Preload("Items").First(&order, "id = ?", fullDN.OrderID).Error; err != nil {
			return newError("ORDER_NOT_FOUND", "Order not found", err.Error())
		}

		allDelivered := true
		someDelivered := false
		for _, item := range order.Items {
			if item.Quantity > 0 {
				// Reload the item to get updated QuantityShipped
				var freshItem OrderItem
				if err := tx.First(&freshItem, "id = ?", item.ID).Error; err == nil {
					if freshItem.QuantityShipped >= freshItem.Quantity-0.001 {
						someDelivered = true
					} else {
						allDelivered = false
						if freshItem.QuantityShipped > 0.001 {
							someDelivered = true
						}
					}
				}
			}
		}

		newStatus := order.Status
		if allDelivered {
			newStatus = "Delivered"
		} else if someDelivered {
			newStatus = "Partially Delivered"
		}

		if newStatus != order.Status {
			if err := tx.Model(&order).Update("status", newStatus).Error; err != nil {
				log.Printf("Warning: Failed to update order status to %s: %v", newStatus, err)
			} else {
				log.Printf("Order %s status updated to %s after delivery confirmation", order.OrderNumber, newStatus)
			}
		}

		return nil
	})
}
