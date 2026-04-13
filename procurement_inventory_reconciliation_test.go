package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCompleteGRN_ReconcilesAcceptedQuantityIntoInventory(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(
		&ProductMaster{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&InventoryItem{},
		&StockMovement{},
	))

	product := ProductMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:      "RCPT-1",
		ProductName:      "Receipt Product",
		StandardCostBHD:  21,
		StandardPriceBHD: 30,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	po := PurchaseOrder{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PONumber:     "PO-RCPT-1",
		SupplierID:   uuid.New().String(),
		SupplierName: "Receipt Supplier",
		PODate:       time.Now(),
		Currency:     "BHD",
		ExchangeRate: 1,
		PaymentTerms: "Net 30",
		Status:       "Sent",
	}
	require.NoError(t, app.db.Create(&po).Error)

	poItem := PurchaseOrderItem{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PurchaseOrderID:  po.ID,
		ProductID:        product.ID,
		ProductCode:      product.ProductCode,
		Description:      product.ProductName,
		Quantity:         5,
		UnitPriceBHD:     33,
		UnitPriceForeign: 33,
		TotalBHD:         165,
		TotalForeign:     165,
	}
	require.NoError(t, app.db.Create(&poItem).Error)

	grn := GoodsReceivedNote{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PurchaseOrderID: po.ID,
		GRNNumber:       "GRN-RCPT-1",
		ReceivedDate:    time.Now(),
		ReceivedBy:      "tester",
		QCStatus:        "Pending",
		WarehouseID:     "WH-MAIN",
	}
	require.NoError(t, app.db.Create(&grn).Error)

	grnItem := GRNItem{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		GRNID:            grn.ID,
		POItemID:         poItem.ID,
		ProductID:        product.ID,
		QuantityOrdered:  5,
		QuantityReceived: 5,
		QuantityRejected: 1,
	}
	require.NoError(t, app.db.Create(&grnItem).Error)

	require.NoError(t, app.CompleteGRN(grn.ID))

	var inventoryItem InventoryItem
	require.NoError(t, app.db.Where("product_id = ? AND warehouse_id = ?", product.ID, "WH-MAIN").First(&inventoryItem).Error)
	require.InDelta(t, 4, inventoryItem.QuantityOnHand, 0.0001)
	require.InDelta(t, 4, inventoryItem.QuantityAvailable, 0.0001)
	require.InDelta(t, 33, inventoryItem.UnitCost, 0.0001)
	require.InDelta(t, 33, inventoryItem.LastPurchaseCost, 0.0001)
	require.InDelta(t, 132, inventoryItem.TotalValue, 0.0001)

	var movement StockMovement
	require.NoError(t, app.db.Where("inventory_item_id = ?", inventoryItem.ID).First(&movement).Error)
	require.Equal(t, "GRN Receipt", movement.MovementType)
	require.InDelta(t, 4, movement.Quantity, 0.0001)
	require.InDelta(t, 33, movement.UnitCost, 0.0001)
	require.InDelta(t, 132, movement.TotalValue, 0.0001)
}

func TestApproveSupplierInvoice_RequiresMatchedStatus(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierInvoice{}))

	invoice := SupplierInvoice{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now(), CreatedBy: "creator-user"},
		SupplierID:    uuid.New().String(),
		SupplierName:  "Approval Supplier",
		InvoiceNumber: "SI-APPROVE-1",
		InvoiceDate:   time.Now(),
		DueDate:       time.Now().AddDate(0, 0, 30),
		Currency:      "BHD",
		ExchangeRate:  1,
		TotalBHD:      100,
		TotalForeign:  100,
		Status:        "Pending",
		MatchStatus:   "Pending",
	}
	require.NoError(t, app.db.Create(&invoice).Error)

	err := app.ApproveSupplierInvoice(invoice.ID, "approver-user")
	require.Error(t, err)
	require.Contains(t, err.Error(), "3-way match is Matched")

	require.NoError(t, app.db.Model(&SupplierInvoice{}).Where("id = ?", invoice.ID).Update("match_status", "Matched").Error)
	require.NoError(t, app.ApproveSupplierInvoice(invoice.ID, "approver-user"))

	var stored SupplierInvoice
	require.NoError(t, app.db.First(&stored, "id = ?", invoice.ID).Error)
	require.Equal(t, "Approved", stored.Status)
	require.Equal(t, "approver-user", stored.ApprovedBy)
	require.NotNil(t, stored.ApprovedAt)
}

func TestPerformThreeWayMatch_UsesPendingForReviewablePriceVariance(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(
		&ProductMaster{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
	))

	product := ProductMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:      "PRICE-REVIEW-1",
		ProductName:      "Variance Product",
		StandardCostBHD:  100,
		StandardPriceBHD: 130,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	po := PurchaseOrder{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PONumber:        "PO-PRICE-1",
		SupplierID:      uuid.New().String(),
		SupplierName:    "Variance Supplier",
		PODate:          time.Now(),
		Currency:        "BHD",
		ExchangeRate:    1,
		SubtotalBHD:     100,
		SubtotalForeign: 100,
		TotalBHD:        100,
		TotalForeign:    100,
		PaymentTerms:    "Net 30",
		Status:          "Sent",
	}
	require.NoError(t, app.db.Create(&po).Error)

	poItem := PurchaseOrderItem{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PurchaseOrderID:  po.ID,
		ProductID:        product.ID,
		ProductCode:      product.ProductCode,
		Description:      product.ProductName,
		Quantity:         1,
		UnitPriceBHD:     100,
		UnitPriceForeign: 100,
		TotalBHD:         100,
		TotalForeign:     100,
	}
	require.NoError(t, app.db.Create(&poItem).Error)

	grn := GoodsReceivedNote{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PurchaseOrderID: po.ID,
		GRNNumber:       "GRN-PRICE-1",
		ReceivedDate:    time.Now(),
		ReceivedBy:      "tester",
		QCStatus:        "Passed",
	}
	require.NoError(t, app.db.Create(&grn).Error)
	require.NoError(t, app.db.Create(&GRNItem{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		GRNID:            grn.ID,
		POItemID:         poItem.ID,
		ProductID:        product.ID,
		QuantityOrdered:  1,
		QuantityReceived: 1,
		QuantityAccepted: 1,
	}).Error)

	invoice := SupplierInvoice{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierID:      po.SupplierID,
		SupplierName:    po.SupplierName,
		PurchaseOrderID: po.ID,
		PONumber:        po.PONumber,
		GRNID:           grn.ID,
		InvoiceNumber:   "SI-PRICE-1",
		InvoiceDate:     time.Now(),
		DueDate:         time.Now().AddDate(0, 0, 30),
		Currency:        "BHD",
		ExchangeRate:    1,
		SubtotalBHD:     103,
		SubtotalForeign: 103,
		TotalBHD:        103,
		TotalForeign:    103,
		Status:          "Pending",
		MatchStatus:     "Pending",
	}
	require.NoError(t, app.db.Create(&invoice).Error)
	require.NoError(t, app.db.Create(&SupplierInvoiceItem{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierInvoiceID: invoice.ID,
		LineNumber:        1,
		Description:       product.ProductName,
		Quantity:          1,
		UnitPrice:         103,
		TotalPrice:        103,
		Currency:          "BHD",
	}).Error)

	result, err := app.PerformThreeWayMatch(invoice.ID)
	require.NoError(t, err)
	require.False(t, result.Matched)

	var stored SupplierInvoice
	require.NoError(t, app.db.First(&stored, "id = ?", invoice.ID).Error)
	require.Equal(t, "Pending", stored.MatchStatus)
	require.Equal(t, "Pending", stored.Status)
	require.Contains(t, stored.DiscrepancyReason, "variance")
}
