package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (a *App) ensureInventoryItemForReceipt(tx *gorm.DB, grn GoodsReceivedNote, poItem PurchaseOrderItem) (*InventoryItem, error) {
	warehouseID := strings.TrimSpace(grn.WarehouseID)

	var item InventoryItem
	query := tx.Where("product_id = ?", poItem.ProductID)
	if warehouseID == "" {
		query = query.Where("(warehouse_id = ? OR warehouse_id IS NULL)", "")
	} else {
		query = query.Where("warehouse_id = ?", warehouseID)
	}

	if err := query.First(&item).Error; err == nil {
		if strings.TrimSpace(item.ProductCode) == "" && strings.TrimSpace(poItem.ProductCode) != "" {
			item.ProductCode = poItem.ProductCode
		}
		return &item, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	item = InventoryItem{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		ProductID:         poItem.ProductID,
		ProductCode:       poItem.ProductCode,
		WarehouseID:       warehouseID,
		QuantityOnHand:    0,
		QuantityReserved:  0,
		QuantityAvailable: 0,
		StockStatus:       "OutOfStock",
		IsActive:          true,
	}
	item.CreatedBy = a.getCurrentUserID()

	if err := tx.Create(&item).Error; err != nil {
		return nil, err
	}

	return &item, nil
}

func (a *App) reconcileInventoryReceipt(tx *gorm.DB, grn GoodsReceivedNote, poItem PurchaseOrderItem, grnItem GRNItem) error {
	if grnItem.QuantityAccepted <= 0 {
		return nil
	}

	inventoryItem, err := a.ensureInventoryItemForReceipt(tx, grn, poItem)
	if err != nil {
		return fmt.Errorf("failed to ensure inventory item: %w", err)
	}

	movement := StockMovement{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		InventoryItemID: inventoryItem.ID,
		MovementType:    "GRN Receipt",
		MovementNumber:  fmt.Sprintf("%s-%s", grn.GRNNumber, grnItem.ID),
		ReferenceType:   "goods_received_note",
		ReferenceID:     grn.ID,
		ReferenceNumber: grn.GRNNumber,
		Quantity:        grnItem.QuantityAccepted,
		Direction:       "IN",
		MovementDate:    grn.ReceivedDate,
	}
	movement.CreatedBy = a.getCurrentUserID()

	referenceCost := a.resolvePurchaseOrderItemReferenceCost(poItem)
	movement.BalanceBefore = inventoryItem.QuantityOnHand
	inventoryItem.QuantityOnHand += movement.Quantity
	inventoryItem.QuantityAvailable = inventoryItem.QuantityOnHand - inventoryItem.QuantityReserved
	movement.BalanceAfter = inventoryItem.QuantityOnHand
	inventoryItem.StockStatus = a.calculateStockStatus(
		inventoryItem.QuantityOnHand,
		inventoryItem.ReorderPoint,
		inventoryItem.MinimumStock,
		inventoryItem.MaximumStock,
	)
	movement.UnitCost = referenceCost.UnitCostBHD
	a.applyInventoryMovementValuation(inventoryItem, &movement)
	now := time.Now()
	inventoryItem.LastMovementAt = &now

	if err := tx.Create(&movement).Error; err != nil {
		return fmt.Errorf("failed to create stock movement: %w", err)
	}
	if err := tx.Save(inventoryItem).Error; err != nil {
		return fmt.Errorf("failed to update inventory item: %w", err)
	}

	return nil
}
