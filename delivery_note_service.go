package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
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

	// Validate delivery item quantities won't go negative
	if len(dn.Items) > 0 && dn.OrderID != "" {
		remainingQty, err := a.GetOrderDeliveryStatus(dn.OrderID)
		if err == nil {
			for _, item := range dn.Items {
				if item.OrderItemID != "" {
					if remaining, ok := remainingQty[item.OrderItemID]; ok {
						if item.QuantityDelivered > remaining {
							return DeliveryNote{}, newError("QUANTITY_EXCEEDED",
								fmt.Sprintf("Cannot deliver %.2f units for %s - only %.2f remaining",
									item.QuantityDelivered, item.ProductCode, remaining), "")
						}
					}
				}
				if item.QuantityDelivered <= 0 {
					return DeliveryNote{}, newError("INVALID_QUANTITY",
						fmt.Sprintf("Delivery quantity must be positive, got: %.2f", item.QuantityDelivered), "")
				}
			}
		}
	}

	// Set defaults
	if dn.Status == "" {
		dn.Status = "Prepared"
	}
	if dn.CustomerID == "" {
		dn.CustomerID = order.CustomerID
	}
	if dn.DeliveryDate.IsZero() {
		dn.DeliveryDate = time.Now()
	}
	dn.CreatedBy = a.getCurrentUserID()

	// Create delivery note
	if err := a.db.Create(&dn).Error; err != nil {
		return DeliveryNote{}, newError("DB_CREATE_FAILED", "Failed to create delivery note", err.Error())
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

	var deliveryNotes []DeliveryNote
	if err := a.db.Preload("Items").Order("delivery_date DESC, created_at DESC").Find(&deliveryNotes).Error; err != nil {
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
	if err := a.db.Preload("Items").Where("order_id = ?", orderID).Order("delivery_sequence ASC").Find(&deliveryNotes).Error; err != nil {
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
	if err := a.db.Preload("Items").Where("customer_id = ?", customerID).Order("delivery_date DESC").Find(&deliveryNotes).Error; err != nil {
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

	// Soft delete (GORM will handle DeletedAt)
	if err := a.db.Delete(&dn).Error; err != nil {
		return newError("DB_DELETE_FAILED", "Failed to delete delivery note", err.Error())
	}

	log.Printf("🗑️ Deleted DeliveryNote: %s", dn.DNNumber)
	return nil
}

// GenerateDNNumber generates a new DN number in format DN-2026-0001
func (a *App) GenerateDNNumber() (string, error) {
	if a.db == nil {
		return "", newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	year := time.Now().Year()
	var count int64
	var dnNumber string

	// Use a transaction with EXCLUSIVE lock to prevent race conditions
	// SQLite's BEGIN EXCLUSIVE ensures only one process can generate a number at a time
	err := a.db.Transaction(func(tx *gorm.DB) error {
		// Lock the table for exclusive access during this transaction
		// This prevents concurrent GenerateDNNumber calls from reading the same count
		if err := tx.Exec("BEGIN EXCLUSIVE").Error; err != nil {
			return err
		}

		// Count DNs in current year within the locked transaction
		if err := tx.Model(&DeliveryNote{}).Where("dn_number LIKE ?", fmt.Sprintf("DN-%d-%%", year)).Count(&count).Error; err != nil {
			return newError("DB_QUERY_FAILED", "Failed to count delivery notes", err.Error())
		}

		// Transaction will commit automatically if no error is returned
		return nil
	})

	if err != nil {
		// Check if it's a query error (should be returned to caller)
		if errObj, ok := err.(*AppError); ok && errObj.Code == "DB_QUERY_FAILED" {
			return "", err
		}

		log.Printf("⚠️ Warning: DN number generation transaction failed: %v, falling back to best-effort", err)
		// Fallback to non-transactional read (should rarely happen)
		if err := a.db.Model(&DeliveryNote{}).Where("dn_number LIKE ?", fmt.Sprintf("DN-%d-%%", year)).Count(&count).Error; err != nil {
			return "", newError("DB_QUERY_FAILED", "Failed to count delivery notes", err.Error())
		}
	}

	dnNumber = fmt.Sprintf("DN-%d-%04d", year, count+1)
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

	// Update status and transport details
	updates := map[string]interface{}{
		"status":         "Dispatched",
		"driver_name":    driverName,
		"vehicle_number": vehicleNumber,
	}

	if err := a.db.Model(&dn).Updates(updates).Error; err != nil {
		return newError("DB_UPDATE_FAILED", "Failed to dispatch delivery note", err.Error())
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

	// Update status and signature
	now := time.Now()
	updates := map[string]interface{}{
		"status":    "Delivered",
		"signed_by": signedBy,
		"signed_at": now,
	}

	if err := a.db.Model(&dn).Updates(updates).Error; err != nil {
		return newError("DB_UPDATE_FAILED", "Failed to confirm delivery", err.Error())
	}

	// Update order status if all deliveries are complete
	if err := a.updateOrderDeliveryStatus(dn.OrderID); err != nil {
		log.Printf("⚠️ Failed to update order status: %v", err)
		// Don't fail the confirmation, just log the warning
	}

	// Progress order based on delivery completion
	if err := a.ProgressOrderOnDelivery(dn.OrderID); err != nil {
		log.Printf("⚠️ Failed to progress order on delivery: %v", err)
		// Don't fail the confirmation, just log the warning
	}

	log.Printf("✅ Confirmed delivery of DN %s, signed by %s", dn.DNNumber, signedBy)
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

	if err := query.Order("delivery_date ASC").Find(&deliveryNotes).Error; err != nil {
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
