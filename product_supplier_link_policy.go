package main

import (
	"fmt"
	"strings"
)

func (a *App) resolveSupplierForProduct(product ProductMaster) (*SupplierMaster, error) {
	if a.db == nil {
		return nil, newError("DB_NOT_INITIALIZED", "Database connection not available", "")
	}

	if supplierID := strings.TrimSpace(product.SupplierID); supplierID != "" {
		var supplier SupplierMaster
		if err := a.db.First(&supplier, "id = ?", supplierID).Error; err == nil {
			return &supplier, nil
		}
	}

	if supplierCode := strings.TrimSpace(product.SupplierCode); supplierCode != "" {
		var supplier SupplierMaster
		if err := a.db.Where("supplier_code = ?", supplierCode).First(&supplier).Error; err == nil {
			return &supplier, nil
		}
		if canonicalCode := canonicalSupplierCodeRef(supplierCode); canonicalCode != supplierCode {
			if err := a.db.Where("supplier_code = ?", canonicalCode).First(&supplier).Error; err == nil {
				return &supplier, nil
			}
		}
	}

	for _, token := range []string{product.SupplierCode, product.ProductName, product.Description, product.ProductCode} {
		if supplier, err := a.findSupplierByCommercialToken(token); err == nil {
			return supplier, nil
		}
	}

	return nil, fmt.Errorf("supplier not found for product %s", product.ProductCode)
}

func (a *App) normalizeProductSupplierLink(product ProductMaster) (ProductMaster, error) {
	supplier, err := a.resolveSupplierForProduct(product)
	if err != nil {
		return product, err
	}

	product.SupplierID = supplier.ID
	product.SupplierCode = supplier.SupplierCode
	return product, nil
}

func canonicalSupplierCodeRef(raw string) string {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "SVX":
		return "SRVX"
	default:
		return strings.ToUpper(strings.TrimSpace(raw))
	}
}
