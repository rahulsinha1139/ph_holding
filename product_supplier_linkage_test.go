package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSeedProductDatabase_UsesCanonicalSupplierLinks(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}))

	require.NoError(t, app.SeedProductDatabase())

	var endress ProductMaster
	require.NoError(t, app.db.Where("product_code = ?", "8E3B50-1").First(&endress).Error)
	require.Equal(t, "EH", endress.SupplierCode)
	require.NotEmpty(t, endress.SupplierID)

	var supplier SupplierMaster
	require.NoError(t, app.db.First(&supplier, "id = ?", endress.SupplierID).Error)
	require.Equal(t, "EH", supplier.SupplierCode)

	var gic ProductMaster
	require.NoError(t, app.db.Where("product_code = ?", "GIC-PG100").First(&gic).Error)
	require.Equal(t, "GIC", gic.SupplierCode)
	require.NotEmpty(t, gic.SupplierID)
	var gicSupplier SupplierMaster
	require.NoError(t, app.db.First(&gicSupplier, "id = ?", gic.SupplierID).Error)
	require.Equal(t, "GIC", gicSupplier.SupplierCode)
}

func TestResolveSupplierForProduct_FallsBackToSupplierCode(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}))

	supplier, _, err := app.ensureSeedSupplier(supplierSeedSpec{
		Name:         "Fallback Supplier",
		Code:         "FALL",
		Country:      "Bahrain",
		PaymentTerms: "Net 30",
		SupplierType: "Manufacturer",
		Rating:       5,
	})
	require.NoError(t, err)

	product := ProductMaster{
		ProductCode:  "TEST-FALLBACK",
		ProductName:  "Fallback Product",
		SupplierID:   "sup_fall",
		SupplierCode: "FALL",
	}

	resolved, err := app.resolveSupplierForProduct(product)
	require.NoError(t, err)
	require.Equal(t, supplier.ID, resolved.ID)
}

func TestResolveSupplierForProduct_FallsBackToCommercialSupplierAlias(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}))

	supplier := SupplierMaster{
		Base:         Base{ID: "supplier-servomex"},
		SupplierCode: "SUP-25",
		SupplierName: "SERVOMEX",
		Country:      "UK",
		PaymentTerms: "Net 45",
		Rating:       5,
	}
	require.NoError(t, app.db.Create(&supplier).Error)

	product := ProductMaster{
		ProductCode:  "SVX-2200",
		ProductName:  "Servomex 2200 Oxygen Analyzer",
		SupplierID:   "sup_svx",
		SupplierCode: "SVX",
		Description:  "Paramagnetic O2 analyzer for industrial gas",
	}

	resolved, err := app.resolveSupplierForProduct(product)
	require.NoError(t, err)
	require.Equal(t, supplier.ID, resolved.ID)
}

func TestInferSupplierForOrderItems_FallsBackToSupplierCodeLink(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}))

	supplier, _, err := app.ensureSeedSupplier(supplierSeedSpec{
		Name:         "PO Supplier",
		Code:         "POC",
		Country:      "Bahrain",
		PaymentTerms: "Net 30",
		SupplierType: "Manufacturer",
		Rating:       5,
	})
	require.NoError(t, err)

	product := ProductMaster{
		ProductCode:      "PO-PROD-1",
		ProductName:      "PO Product",
		SupplierID:       "sup_poc",
		SupplierCode:     "POC",
		StandardCostBHD:  10,
		StandardPriceBHD: 15,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	order := Order{
		Items: []OrderItem{
			{
				Base:        Base{ID: "item-1"},
				ProductID:   product.ID,
				ProductCode: product.ProductCode,
				Quantity:    1,
			},
		},
	}

	inferred, err := app.inferSupplierForOrderItems(order, nil)
	require.NoError(t, err)
	require.Equal(t, supplier.ID, inferred.ID)
}

func TestInferSupplierForOrderItems_FallsBackToCommercialText(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}))

	supplier := SupplierMaster{
		Base:         Base{ID: "supplier-servomex"},
		SupplierCode: "SUP-25",
		SupplierName: "SERVOMEX",
		Country:      "UK",
		PaymentTerms: "Net 45",
		Rating:       5,
	}
	require.NoError(t, app.db.Create(&supplier).Error)

	order := Order{
		Items: []OrderItem{
			{
				Base:        Base{ID: "item-servomex-1"},
				ProductCode: "02500E1",
				Description: "SERVOMEX - 02500E1",
				Quantity:    1,
				UnitPrice:   15485,
			},
			{
				Base:        Base{ID: "item-servomex-2"},
				ProductCode: "01910B1",
				Description: "SERVOMEX - 01910B1",
				Quantity:    1,
				UnitPrice:   9702,
			},
		},
	}

	inferred, err := app.inferSupplierForOrderItems(order, nil)
	require.NoError(t, err)
	require.Equal(t, supplier.ID, inferred.ID)
}

func TestCreatePOFromOrder_UsesVisibleSelectedItemsAndCommercialSupplier(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&PurchaseOrder{}, &PurchaseOrderItem{}))

	customerID := seedTestCustomer(t, app.db, "SULB COMPANY B.S.C CLOSED")
	supplier := SupplierMaster{
		Base:         Base{ID: "supplier-servomex"},
		SupplierCode: "SUP-25",
		SupplierName: "SERVOMEX",
		Country:      "UK",
		PaymentTerms: "Net 45",
		Rating:       5,
	}
	require.NoError(t, app.db.Create(&supplier).Error)

	order := Order{
		Base:          Base{ID: "order-oth-3-26"},
		OrderNumber:   "OTH-3-26",
		CustomerID:    customerID,
		CustomerName:  "SULB COMPANY B.S.C CLOSED",
		OrderDate:     time.Now(),
		Status:        "Confirmed",
		TotalValueBHD: 42501,
	}
	require.NoError(t, app.db.Create(&order).Error)

	items := []OrderItem{
		{Base: Base{ID: "item-visible-1"}, OrderID: order.ID, ProductCode: "02500E1", Description: "SERVOMEX - 02500E1", Equipment: "SERVOMEX", Model: "02500E1", Quantity: 1, UnitPrice: 15485, TotalPrice: 15485},
		{Base: Base{ID: "item-visible-2"}, OrderID: order.ID, ProductCode: "02500E1", Description: "SERVOMEX - 02500E1", Equipment: "SERVOMEX", Model: "02500E1", Quantity: 1, UnitPrice: 17314, TotalPrice: 17314},
		{Base: Base{ID: "item-visible-3"}, OrderID: order.ID, ProductCode: "01910B1", Description: "SERVOMEX - 01910B1", Equipment: "SERVOMEX", Model: "01910B1", Quantity: 1, UnitPrice: 9702, TotalPrice: 9702},
		{Base: Base{ID: "item-duplicate-hidden"}, OrderID: order.ID, ProductCode: "01910B1", Description: "SERVOMEX - 01910B1", Equipment: "SERVOMEX", Model: "01910B1", Quantity: 1, UnitPrice: 9702, TotalPrice: 9702},
	}
	for _, item := range items {
		require.NoError(t, app.db.Create(&item).Error)
	}

	po, err := app.CreatePOFromOrder(order.ID, "", []string{"item-visible-1", "item-visible-2", "item-visible-3"})
	require.NoError(t, err)
	require.Equal(t, supplier.ID, po.SupplierID)
	require.Len(t, po.Items, 3)
}

func TestGetInventoryAlertsLowStock_UsesResolvedSupplierLink(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}, &InventoryItem{}))
	app.currentUser = &User{
		Base:     Base{ID: "test-user"},
		Username: "test-admin",
		RoleName: "admin",
		Role: Role{
			Name:        "admin",
			DisplayName: "Admin",
			Permissions: `["dashboard:view"]`,
		},
	}

	supplier, _, err := app.ensureSeedSupplier(supplierSeedSpec{
		Name:         "Inventory Supplier",
		Code:         "INV",
		Country:      "Bahrain",
		PaymentTerms: "Net 30",
		SupplierType: "Manufacturer",
		Rating:       5,
	})
	require.NoError(t, err)

	product := ProductMaster{
		ProductCode:      "INV-PROD-1",
		ProductName:      "Inventory Product",
		SupplierID:       "sup_inv",
		SupplierCode:     "INV",
		StandardCostBHD:  12,
		StandardPriceBHD: 18,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	now := time.Now().Add(-48 * time.Hour)
	require.NoError(t, app.db.Create(&InventoryItem{
		ProductID:         product.ID,
		ProductCode:       product.ProductCode,
		QuantityOnHand:    1,
		QuantityReserved:  0,
		QuantityAvailable: 1,
		ReorderPoint:      5,
		MinimumStock:      2,
		StockStatus:       "LowStock",
		IsActive:          true,
		LastMovementAt:    &now,
	}).Error)

	alerts, err := app.GetInventoryAlertsLowStock(0)
	require.NoError(t, err)
	require.Len(t, alerts, 1)
	require.Equal(t, supplier.ID, alerts[0].SupplierID)
	require.Equal(t, supplier.SupplierName, alerts[0].SupplierName)
}
