package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestRaiseGRNDiscrepancy_UsesPurchaseOrderReferenceCost(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(
		&ProductMaster{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&SupplierIssue{},
	))

	product := ProductMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:      "DISC-1",
		ProductName:      "Discrepancy Product",
		StandardCostBHD:  15,
		StandardPriceBHD: 20,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	po := PurchaseOrder{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PONumber:     "PO-DISC-1",
		SupplierID:   uuid.New().String(),
		SupplierName: "Discrepancy Supplier",
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
		GRNNumber:       "GRN-DISC-1",
		ReceivedDate:    time.Now(),
		ReceivedBy:      "tester",
		QCStatus:        "Pending",
	}
	require.NoError(t, app.db.Create(&grn).Error)

	grnItem := GRNItem{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		GRNID:            grn.ID,
		POItemID:         poItem.ID,
		ProductID:        product.ID,
		QuantityOrdered:  5,
		QuantityReceived: 5,
	}
	require.NoError(t, app.db.Create(&grnItem).Error)

	require.NoError(t, app.RaiseGRNDiscrepancy(grn.ID, grnItem.ID, "Damaged casing", "damaged", 2))

	var issue SupplierIssue
	require.NoError(t, app.db.First(&issue).Error)
	require.InDelta(t, 66, issue.CostBHD, 0.0001)
}

func TestPerformThreeWayMatch_UsesProductCostFallbackForPriceComparison(t *testing.T) {
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
		ProductCode:      "MATCH-1",
		ProductName:      "Match Product",
		StandardCostBHD:  25,
		StandardPriceBHD: 40,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	po := PurchaseOrder{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PONumber:        "PO-MATCH-1",
		SupplierID:      uuid.New().String(),
		SupplierName:    "Match Supplier",
		PODate:          time.Now(),
		Currency:        "BHD",
		ExchangeRate:    1,
		SubtotalBHD:     50,
		SubtotalForeign: 50,
		TotalBHD:        50,
		TotalForeign:    50,
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
		Quantity:         2,
		UnitPriceBHD:     0,
		UnitPriceForeign: 0,
		TotalBHD:         50,
		TotalForeign:     50,
	}
	require.NoError(t, app.db.Create(&poItem).Error)

	grn := GoodsReceivedNote{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PurchaseOrderID: po.ID,
		GRNNumber:       "GRN-MATCH-1",
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
		QuantityOrdered:  2,
		QuantityReceived: 2,
		QuantityAccepted: 2,
	}).Error)

	invoice := SupplierInvoice{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierID:      po.SupplierID,
		SupplierName:    po.SupplierName,
		PurchaseOrderID: po.ID,
		PONumber:        po.PONumber,
		GRNID:           grn.ID,
		InvoiceNumber:   "SI-MATCH-1",
		InvoiceDate:     time.Now(),
		DueDate:         time.Now().AddDate(0, 0, 30),
		Currency:        "BHD",
		ExchangeRate:    1,
		SubtotalBHD:     50,
		SubtotalForeign: 50,
		TotalBHD:        50,
		TotalForeign:    50,
		Status:          "Pending",
		MatchStatus:     "Pending",
	}
	require.NoError(t, app.db.Create(&invoice).Error)
	require.NoError(t, app.db.Create(&SupplierInvoiceItem{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierInvoiceID: invoice.ID,
		LineNumber:        1,
		Description:       product.ProductName,
		Quantity:          2,
		UnitPrice:         25,
		TotalPrice:        50,
		Currency:          "BHD",
	}).Error)

	result, err := app.PerformThreeWayMatch(invoice.ID)
	require.NoError(t, err)
	require.True(t, result.Matched)

	var stored SupplierInvoice
	require.NoError(t, app.db.First(&stored, "id = ?", invoice.ID).Error)
	require.Equal(t, "Matched", stored.MatchStatus)
	require.Equal(t, "Pending", stored.Status)
}

func TestRecordStockMovement_UsesProductStandardCostFallback(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}, &InventoryItem{}, &StockMovement{}))

	product := ProductMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:      "INV-VAL-1",
		ProductName:      "Inventory Valuation Product",
		StandardCostBHD:  18,
		StandardPriceBHD: 24,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	item := InventoryItem{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductID:         product.ID,
		ProductCode:       product.ProductCode,
		QuantityOnHand:    0,
		QuantityReserved:  0,
		QuantityAvailable: 0,
		StockStatus:       "OutOfStock",
		IsActive:          true,
	}
	require.NoError(t, app.db.Create(&item).Error)

	movement, err := app.RecordStockMovement(StockMovement{
		InventoryItemID: item.ID,
		MovementType:    "Receipt",
		MovementNumber:  "MOV-VAL-1",
		Quantity:        5,
		Direction:       "IN",
		MovementDate:    time.Now(),
	})
	require.NoError(t, err)
	require.InDelta(t, 18, movement.UnitCost, 0.0001)
	require.InDelta(t, 90, movement.TotalValue, 0.0001)

	var stored InventoryItem
	require.NoError(t, app.db.First(&stored, "id = ?", item.ID).Error)
	require.InDelta(t, 18, stored.UnitCost, 0.0001)
	require.InDelta(t, 18, stored.LastPurchaseCost, 0.0001)
	require.InDelta(t, 90, stored.TotalValue, 0.0001)
}

func TestGetInventoryValuation_UsesResolvedFallbackCost(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}, &InventoryItem{}))

	product := ProductMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:      "INV-SUM-1",
		ProductName:      "Inventory Summary Product",
		StandardCostBHD:  12,
		StandardPriceBHD: 18,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	require.NoError(t, app.db.Create(&InventoryItem{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductID:         product.ID,
		ProductCode:       product.ProductCode,
		QuantityOnHand:    4,
		QuantityReserved:  0,
		QuantityAvailable: 4,
		UnitCost:          0,
		TotalValue:        0,
		StockStatus:       "InStock",
		IsActive:          true,
	}).Error)

	valuation, err := app.GetInventoryValuation(nil)
	require.NoError(t, err)
	require.InDelta(t, 48, valuation["total_value"].(float64), 0.0001)
	require.InDelta(t, 4, valuation["total_quantity"].(float64), 0.0001)
}
