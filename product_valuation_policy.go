package main

import "strings"

type productReferenceCost struct {
	UnitCostBHD float64
	Source      string
}

func (a *App) resolveProductReferenceCost(productID string) productReferenceCost {
	if strings.TrimSpace(productID) == "" {
		return productReferenceCost{}
	}

	profile, err := a.loadProductOperationsProfile(productID)
	if err != nil {
		return productReferenceCost{}
	}
	if profile.Product.StandardCostBHD > 0 {
		return productReferenceCost{
			UnitCostBHD: profile.Product.StandardCostBHD,
			Source:      "product_standard_cost",
		}
	}

	return productReferenceCost{}
}

func (a *App) resolvePurchaseOrderItemReferenceCost(poItem PurchaseOrderItem) productReferenceCost {
	if poItem.UnitPriceBHD > 0 {
		return productReferenceCost{
			UnitCostBHD: poItem.UnitPriceBHD,
			Source:      "po_unit_price_bhd",
		}
	}

	return a.resolveProductReferenceCost(poItem.ProductID)
}

func (a *App) resolveSupplierInvoiceItemUnitPriceBHD(invoice SupplierInvoice, item SupplierInvoiceItem) float64 {
	if item.UnitPrice <= 0 {
		return 0
	}
	if invoice.Currency == "BHD" || invoice.Currency == "" || invoice.ExchangeRate <= 0 {
		return item.UnitPrice
	}

	return item.UnitPrice * invoice.ExchangeRate
}

func (a *App) resolveInventoryItemUnitCost(item InventoryItem) productReferenceCost {
	if item.UnitCost > 0 {
		return productReferenceCost{
			UnitCostBHD: item.UnitCost,
			Source:      "inventory_unit_cost",
		}
	}
	if item.LastPurchaseCost > 0 {
		return productReferenceCost{
			UnitCostBHD: item.LastPurchaseCost,
			Source:      "inventory_last_purchase_cost",
		}
	}

	return a.resolveProductReferenceCost(item.ProductID)
}

func (a *App) applyInventoryMovementValuation(item *InventoryItem, movement *StockMovement) {
	referenceCost := a.resolveInventoryItemUnitCost(*item)
	existingUnitCost := referenceCost.UnitCostBHD

	incomingUnitCost := movement.UnitCost
	if incomingUnitCost <= 0 {
		incomingUnitCost = referenceCost.UnitCostBHD
	}

	if movement.Direction == "IN" {
		if incomingUnitCost > 0 && item.QuantityOnHand > 0 {
			totalValue := (existingUnitCost * movement.BalanceBefore) + (incomingUnitCost * movement.Quantity)
			item.UnitCost = totalValue / item.QuantityOnHand
			item.LastPurchaseCost = incomingUnitCost
		} else if incomingUnitCost > 0 {
			item.UnitCost = incomingUnitCost
			item.LastPurchaseCost = incomingUnitCost
		}
	} else if item.UnitCost <= 0 && existingUnitCost > 0 {
		item.UnitCost = existingUnitCost
	}

	if movement.UnitCost <= 0 {
		movement.UnitCost = incomingUnitCost
	}
	if movement.UnitCost > 0 {
		movement.TotalValue = movement.Quantity * movement.UnitCost
	} else {
		movement.TotalValue = movement.Quantity * item.UnitCost
	}

	item.TotalValue = item.QuantityOnHand * item.UnitCost
}

func (a *App) resolveInventoryItemTotalValue(item InventoryItem) float64 {
	if item.TotalValue > 0 {
		return item.TotalValue
	}

	referenceCost := a.resolveInventoryItemUnitCost(item)
	if referenceCost.UnitCostBHD <= 0 {
		return 0
	}

	return item.QuantityOnHand * referenceCost.UnitCostBHD
}
