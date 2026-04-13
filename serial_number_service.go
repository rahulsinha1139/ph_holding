package main

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

const maxSerialsPerBatch = 500

// =============================================================================
// SERIAL NUMBER TRACKING SERVICE (Phase 23)
//
// Provides full lifecycle tracking for serialized process instrumentation:
//   PO Receipt (GRN) → Inventory → Dispatch (DN) → Delivery → Invoice
//
// Government clients (BAPCO, Alba, EWA) require serial-level traceability.
// =============================================================================

// RegisterSerials creates serial number records for a product (manual registration)
func (a *App) RegisterSerials(productID string, serialNos []string) ([]SerialNumber, error) {
	if err := a.requirePermission("grn:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// P1-5: Enforce maximum batch size
	if len(serialNos) == 0 {
		return nil, fmt.Errorf("at least one serial number is required")
	}
	if len(serialNos) > maxSerialsPerBatch {
		return nil, fmt.Errorf("maximum %d serial numbers per batch (got %d)", maxSerialsPerBatch, len(serialNos))
	}

	// P1-8: Validate serial number format
	for i, sn := range serialNos {
		sn = strings.TrimSpace(sn)
		serialNos[i] = sn
		if sn == "" {
			return nil, fmt.Errorf("serial number at position %d is empty", i+1)
		}
		if len(sn) > 255 {
			return nil, fmt.Errorf("serial number at position %d exceeds 255 characters", i+1)
		}
	}

	profile, err := a.loadProductOperationsProfile(productID)
	if err != nil {
		return nil, err
	}
	if !profile.Product.RequiresSerialTracking {
		return nil, newError(
			"SERIAL_TRACKING_DISABLED",
			fmt.Sprintf("Product %s is not configured for serial tracking", profile.Product.ProductCode),
			"",
		)
	}
	product := profile.Product

	// S5 FIX: Duplicate check + creation inside single transaction to prevent TOCTOU race
	var created []SerialNumber
	err = a.db.Transaction(func(tx *gorm.DB) error {
		for _, sn := range serialNos {
			var existing SerialNumber
			if err := tx.Where("serial_no = ?", sn).First(&existing).Error; err == nil {
				return fmt.Errorf("serial number %s already exists (product: %s)", sn, existing.ProductCode)
			}

			record := SerialNumber{
				ProductID:   productID,
				ProductCode: product.ProductCode,
				SerialNo:    sn,
				Status:      "Available",
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("failed to create serial %s: %w", sn, err)
			}
			created = append(created, record)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	log.Printf("✅ Registered %d serial numbers for product %s", len(created), product.ProductCode)
	return created, nil
}

// GetSerialByNumber retrieves a serial number record by its serial number string
func (a *App) GetSerialByNumber(serialNo string) (SerialNumber, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return SerialNumber{}, err
	}
	if a.db == nil {
		return SerialNumber{}, fmt.Errorf("database not initialized")
	}

	var sn SerialNumber
	if err := a.db.Where("serial_no = ?", serialNo).First(&sn).Error; err != nil {
		return SerialNumber{}, fmt.Errorf("serial number not found: %w", err)
	}
	return sn, nil
}

// SearchSerials searches serial numbers by partial match on serial_no, product_code, or customer_name
func (a *App) SearchSerials(query string, limit int) ([]SerialNumber, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	searchPattern := "%" + escapeLikeWildcards(query) + "%"
	var serials []SerialNumber
	if err := a.db.Where("serial_no LIKE ? ESCAPE '\\' OR product_code LIKE ? ESCAPE '\\' OR customer_name LIKE ? ESCAPE '\\'",
		searchPattern, searchPattern, searchPattern).
		Order("created_at DESC").
		Limit(limit).
		Find(&serials).Error; err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	return serials, nil
}

// GetSerialsByProduct returns all serial numbers for a specific product
func (a *App) GetSerialsByProduct(productID string) ([]SerialNumber, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var serials []SerialNumber
	if err := a.db.Where("product_id = ?", productID).
		Order("created_at DESC").
		Limit(1000).
		Find(&serials).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	if len(serials) >= 1000 {
		log.Printf("WARNING: GetSerialsByProduct query hit 1000 limit for product %s — results may be truncated", productID)
	}
	return serials, nil
}

// GetSerialsByCustomer returns all serial numbers delivered to a specific customer
func (a *App) GetSerialsByCustomer(customerID string) ([]SerialNumber, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var serials []SerialNumber
	if err := a.db.Where("customer_id = ?", customerID).
		Order("created_at DESC").
		Limit(1000).
		Find(&serials).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	if len(serials) >= 1000 {
		log.Printf("WARNING: GetSerialsByCustomer query hit 1000 limit for customer %s — results may be truncated", customerID)
	}
	return serials, nil
}

// GetAvailableSerials returns serial numbers with status "Available" for a product
func (a *App) GetAvailableSerials(productID string) ([]SerialNumber, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var serials []SerialNumber
	if err := a.db.Where("product_id = ? AND status = ?", productID, "Available").
		Order("created_at ASC").
		Limit(1000).
		Find(&serials).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	if len(serials) >= 1000 {
		log.Printf("WARNING: GetAvailableSerials query hit 1000 limit for product %s — results may be truncated", productID)
	}
	return serials, nil
}

// UpdateSerialWarranty sets warranty duration and calculates end date
func (a *App) UpdateSerialWarranty(serialID string, warrantyMonths int) error {
	if err := a.requirePermission("grn:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if warrantyMonths <= 0 {
		return fmt.Errorf("warranty months must be positive")
	}

	var sn SerialNumber
	if err := a.db.Where("id = ?", serialID).First(&sn).Error; err != nil {
		return fmt.Errorf("serial not found: %w", err)
	}

	updates := map[string]interface{}{
		"warranty_months": warrantyMonths,
	}
	if sn.WarrantyStartDate != nil {
		endDate := sn.WarrantyStartDate.AddDate(0, warrantyMonths, 0)
		updates["warranty_end_date"] = endDate
	}

	if err := a.db.Model(&sn).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update warranty: %w", err)
	}

	log.Printf("✅ Updated warranty for serial %s: %d months", sn.SerialNo, warrantyMonths)
	return nil
}

// AttachCalibrationCert sets calibration certificate path and date on a serial
func (a *App) AttachCalibrationCert(serialID, certPath string) error {
	if err := a.requirePermission("grn:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// P1-1: Sanitize file path — store only the base filename to prevent path traversal
	certPath = strings.TrimSpace(certPath)
	if certPath == "" {
		return fmt.Errorf("certificate path is required")
	}
	certPath = filepath.Base(certPath)
	if certPath == "." || certPath == "/" || certPath == "\\" {
		return fmt.Errorf("invalid certificate path")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"calibration_cert_path": certPath,
		"calibration_date":      now,
	}
	if err := a.db.Model(&SerialNumber{}).Where("id = ?", serialID).Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to attach calibration cert: %w", err)
	}

	log.Printf("✅ Attached calibration cert to serial %s", serialID)
	return nil
}

// =============================================================================
// LIFECYCLE FUNCTIONS (called by GRN/DN services)
// =============================================================================

// assignSerialsToGRN creates serial records when goods are received against a PO
func (a *App) assignSerialsToGRN(grnItemID, grnNumber, poID, poNumber string, productID, productCode string, serialNos []string, receivedDate time.Time) error {
	// P1-5/P1-8: Validate input
	if len(serialNos) > maxSerialsPerBatch {
		return fmt.Errorf("maximum %d serial numbers per batch", maxSerialsPerBatch)
	}
	for i, sn := range serialNos {
		sn = strings.TrimSpace(sn)
		serialNos[i] = sn
		if sn == "" {
			return fmt.Errorf("serial number at position %d is empty", i+1)
		}
		if len(sn) > 255 {
			return fmt.Errorf("serial number at position %d exceeds 255 characters", i+1)
		}
	}

	return a.db.Transaction(func(tx *gorm.DB) error {
		for _, sn := range serialNos {
			// Check for existing serial
			var existing SerialNumber
			if err := tx.Where("serial_no = ?", sn).First(&existing).Error; err == nil {
				return fmt.Errorf("serial number %s already exists (product: %s)", sn, existing.ProductCode)
			}

			record := SerialNumber{
				ProductID:    productID,
				ProductCode:  productCode,
				SerialNo:     sn,
				Status:       "Available",
				POID:         poID,
				PONumber:     poNumber,
				GRNItemID:    grnItemID,
				GRNNumber:    grnNumber,
				ReceivedDate: &receivedDate,
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("failed to create serial %s: %w", sn, err)
			}
		}
		log.Printf("✅ Assigned %d serials to GRN %s (PO: %s, Product: %s)", len(serialNos), grnNumber, poNumber, productCode)
		return nil
	})
}

// allocateSerialsToDN reserves serials for a delivery note
// Uses atomic UPDATE with WHERE status='Available' to prevent double-allocation (safe on SQLite)
func (a *App) allocateSerialsToDN(dnItemID, dnNumber, customerID, customerName string, serialNos []string) error {
	// P3 S7 FIX: Batch size validation to prevent unbounded allocations
	if len(serialNos) > maxSerialsPerBatch {
		return fmt.Errorf("batch size %d exceeds maximum of %d serials per allocation", len(serialNos), maxSerialsPerBatch)
	}

	return a.db.Transaction(func(tx *gorm.DB) error {
		for _, sn := range serialNos {
			// Atomic check-and-update: only updates if status is still Available
			result := tx.Model(&SerialNumber{}).
				Where("serial_no = ? AND status = ?", sn, "Available").
				Updates(map[string]interface{}{
					"status":        "Reserved",
					"dn_item_id":    dnItemID,
					"dn_number":     dnNumber,
					"customer_id":   customerID,
					"customer_name": customerName,
				})
			if result.Error != nil {
				return fmt.Errorf("failed to allocate serial %s: %w", sn, result.Error)
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("serial %s is not available for allocation (already reserved or does not exist)", sn)
			}
		}
		log.Printf("✅ Allocated %d serials to DN %s (Customer: %s)", len(serialNos), dnNumber, customerName)
		return nil
	})
}

// markSerialsShipped updates serials on a DN to "Shipped" status
func (a *App) markSerialsShipped(dnNumber string) error {
	now := time.Now()
	result := a.db.Model(&SerialNumber{}).
		Where("dn_number = ? AND status = ?", dnNumber, "Reserved").
		Updates(map[string]interface{}{
			"status":       "Shipped",
			"shipped_date": now,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to mark serials shipped: %w", result.Error)
	}
	log.Printf("✅ Marked %d serials as Shipped for DN %s", result.RowsAffected, dnNumber)
	return nil
}

// markSerialsDelivered updates serials on a DN to "Delivered" and sets warranty start
func (a *App) markSerialsDelivered(dnID string) error {
	// Get DN details
	var dn DeliveryNote
	if err := a.db.Where("id = ?", dnID).First(&dn).Error; err != nil {
		return fmt.Errorf("DN not found: %w", err)
	}

	return a.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		var serials []SerialNumber
		if err := tx.Where("dn_number = ? AND status IN ?", dn.DNNumber, []string{"Reserved", "Shipped"}).Find(&serials).Error; err != nil {
			return fmt.Errorf("failed to find serials: %w", err)
		}

		for _, serial := range serials {
			updates := map[string]interface{}{
				"status":              "Delivered",
				"warranty_start_date": now,
			}
			if serial.WarrantyMonths > 0 {
				endDate := now.AddDate(0, serial.WarrantyMonths, 0)
				updates["warranty_end_date"] = endDate
			}
			if err := tx.Model(&serial).Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to update serial %s: %w", serial.SerialNo, err)
			}
		}

		log.Printf("✅ Marked %d serials as Delivered for DN %s", len(serials), dn.DNNumber)
		return nil
	})
}

// linkSerialsToInvoice stamps invoice ID on all serials from a delivery note
func (a *App) linkSerialsToInvoice(invoiceID, invoiceNumber, dnID string) error {
	var dn DeliveryNote
	if err := a.db.Where("id = ?", dnID).First(&dn).Error; err != nil {
		return fmt.Errorf("DN not found: %w", err)
	}

	// P2 FIX: Add race condition guard — only stamp serials that haven't been claimed by another invoice
	// P3 S1 FIX: Case-insensitive status comparison to handle data inconsistencies
	result := a.db.Model(&SerialNumber{}).
		Where("dn_number = ? AND UPPER(status) IN ? AND (invoice_id = '' OR invoice_id IS NULL)", dn.DNNumber, []string{"RESERVED", "SHIPPED", "DELIVERED"}).
		Updates(map[string]interface{}{
			"invoice_id":     invoiceID,
			"invoice_number": invoiceNumber,
		})
	if result.Error != nil {
		return fmt.Errorf("failed to link serials to invoice: %w", result.Error)
	}

	log.Printf("✅ Linked %d serials to invoice %s from DN %s", result.RowsAffected, invoiceNumber, dn.DNNumber)
	return nil
}

// GetSerialsForInvoiceItem returns serials on a specific invoice for a specific product
func (a *App) GetSerialsForInvoiceItem(invoiceID, productID string) ([]SerialNumber, error) {
	if err := a.requirePermission("grn:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	var serials []SerialNumber
	if err := a.db.Where("invoice_id = ? AND product_id = ?", invoiceID, productID).
		Order("serial_no ASC").
		Limit(1000).
		Find(&serials).Error; err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	if len(serials) >= 1000 {
		log.Printf("WARNING: GetSerialsForInvoiceItem query hit 1000 limit for invoice %s, product %s — results may be truncated", invoiceID, productID)
	}
	return serials, nil
}
