package main

import (
	"fmt"
	"log"
	"strings"
)

type productOperationsProfile struct {
	Product  ProductMaster
	Supplier *SupplierMaster
}

type productOperationalCost struct {
	UnitForeign float64
	UnitBHD     float64
	Source      string
}

func (a *App) loadProductOperationsProfile(productID string) (productOperationsProfile, error) {
	if a.db == nil {
		return productOperationsProfile{}, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}
	if strings.TrimSpace(productID) == "" {
		return productOperationsProfile{}, newError("PRODUCT_NOT_FOUND", "Product not found", "")
	}

	var product ProductMaster
	if err := a.db.Select(
		"id",
		"product_code",
		"supplier_id",
		"supplier_code",
		"standard_cost_bhd",
		"requires_serial_tracking",
	).First(&product, "id = ?", productID).Error; err != nil {
		return productOperationsProfile{}, newError("PRODUCT_NOT_FOUND", "Product not found", err.Error())
	}

	profile := productOperationsProfile{Product: product}
	if supplier, err := a.resolveSupplierForProduct(product); err == nil {
		profile.Supplier = supplier
		profile.Product.SupplierID = supplier.ID
		profile.Product.SupplierCode = supplier.SupplierCode
	} else if strings.TrimSpace(product.SupplierID) != "" || strings.TrimSpace(product.SupplierCode) != "" {
		log.Printf("⚠️ Failed to resolve supplier for product %s: %v", product.ProductCode, err)
	}

	return profile, nil
}

func productBelongsToSupplier(profile productOperationsProfile, supplierID string) bool {
	trimmedSupplierID := strings.TrimSpace(supplierID)
	if trimmedSupplierID == "" {
		return true
	}
	if profile.Supplier != nil {
		return profile.Supplier.ID == trimmedSupplierID
	}
	return strings.TrimSpace(profile.Product.SupplierID) == trimmedSupplierID
}

func deriveProductOperationalCost(orderItem OrderItem, product ProductMaster) productOperationalCost {
	estimateBHD := orderItem.UnitPrice * 0.7
	cost := productOperationalCost{
		UnitForeign: estimateBHD,
		UnitBHD:     estimateBHD,
		Source:      "sell_price_estimate",
	}

	if orderItem.FOB > 0 {
		cost.UnitForeign = orderItem.FOB
		if orderItem.TotalCost > 0 && orderItem.Quantity > 0 {
			cost.UnitBHD = orderItem.TotalCost / orderItem.Quantity
			cost.Source = "costing_sheet_landed_cost"
			return cost
		}
		if strings.EqualFold(strings.TrimSpace(orderItem.Currency), "BHD") || strings.TrimSpace(orderItem.Currency) == "" {
			cost.UnitBHD = orderItem.FOB
			cost.Source = "costing_sheet_fob_bhd"
			return cost
		}
		cost.Source = "costing_sheet_fob_foreign_estimate"
		return cost
	}

	if product.StandardCostBHD > 0 {
		cost.UnitForeign = product.StandardCostBHD
		cost.UnitBHD = product.StandardCostBHD
		cost.Source = "product_standard_cost"
	}

	return cost
}

func (a *App) validateProductSerialHandling(productID string, quantity float64, serials []string, flow string) error {
	profile, err := a.loadProductOperationsProfile(productID)
	if err != nil {
		return err
	}

	productCode := profile.Product.ProductCode
	serialCount := len(serials)
	if serialCount > 0 && !profile.Product.RequiresSerialTracking {
		return newError(
			"SERIAL_TRACKING_DISABLED",
			fmt.Sprintf("Product %s is not configured for serial tracking", productCode),
			flow,
		)
	}

	if profile.Product.RequiresSerialTracking && quantity > 0 && serialCount == 0 {
		return newError(
			"SERIALS_REQUIRED",
			fmt.Sprintf("Product %s requires serial numbers for %s", productCode, flow),
			"",
		)
	}

	if serialCount > 0 && float64(serialCount) != quantity {
		return newError(
			"SERIAL_COUNT_MISMATCH",
			fmt.Sprintf("Serial count (%d) must match quantity (%.0f) for %s", serialCount, quantity, productCode),
			"",
		)
	}

	return nil
}
