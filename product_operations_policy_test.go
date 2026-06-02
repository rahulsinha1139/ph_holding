package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestCreatePOFromOrder_UsesResolvedSupplierAndCatalogCost(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(
		&ProductMaster{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
	))

	supplier := SupplierMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierCode: "OPS",
		SupplierName: "Ops Supplier",
		Country:      "Bahrain",
		PaymentTerms: "Net 30",
		Rating:       5,
	}
	require.NoError(t, app.db.Create(&supplier).Error)

	product := ProductMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:      "OPS-PROD-1",
		ProductName:      "Operations Product",
		SupplierID:       "legacy-supplier-id",
		SupplierCode:     "OPS",
		StandardCostBHD:  42,
		StandardPriceBHD: 60,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	customer := CustomerMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:       "CUST-OPS-1",
		CustomerCode:     "CUST-OPS-1",
		BusinessName:     "Ops Customer",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	}
	require.NoError(t, app.db.Create(&customer).Error)

	order := Order{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:  "ORD-OPS-1",
		CustomerID:   customer.ID,
		RequiredDate: time.Now().AddDate(0, 0, 14),
		Status:       "Confirmed",
	}
	require.NoError(t, app.db.Create(&order).Error)

	orderItem := OrderItem{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderID:     order.ID,
		ProductID:   product.ID,
		ProductCode: product.ProductCode,
		Description: "Operations Product",
		Quantity:    2,
		UnitPrice:   75,
	}
	require.NoError(t, app.db.Create(&orderItem).Error)

	po, err := app.CreatePOFromOrder(order.ID, "", nil)
	require.NoError(t, err)
	require.Len(t, po.Items, 1)
	require.Equal(t, supplier.ID, po.SupplierID)
	require.Equal(t, supplier.SupplierName, po.SupplierName)
	require.Equal(t, product.ProductCode, po.Items[0].ProductCode)
	require.InDelta(t, 42, po.Items[0].UnitPriceBHD, 0.0001)
	require.InDelta(t, 42, po.Items[0].UnitPriceForeign, 0.0001)
}

func TestReceiveAgainstPO_RejectsSerializedProductsWithoutSerials(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(
		&ProductMaster{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
	))

	product := ProductMaster{
		Base:                   Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:            "SER-GRN-1",
		ProductName:            "Serialized GRN Product",
		RequiresSerialTracking: true,
		StandardCostBHD:        10,
		StandardPriceBHD:       15,
		IsActive:               true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	po := PurchaseOrder{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PONumber:     "PO-SER-GRN-1",
		SupplierID:   uuid.New().String(),
		SupplierName: "Serialized Supplier",
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
		Quantity:         2,
		UnitPriceBHD:     10,
		UnitPriceForeign: 10,
		TotalBHD:         20,
		TotalForeign:     20,
	}
	require.NoError(t, app.db.Create(&poItem).Error)

	_, err := app.ReceiveAgainstPO(po.ID, []GRNItem{
		{
			Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			POItemID:         poItem.ID,
			QuantityReceived: 2,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires serial numbers")
}

func TestCreateDeliveryNoteWithItems_RejectsSerializedProductsWithoutSerials(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(
		&ProductMaster{},
		&DeliveryNote{},
		&DeliveryNoteItem{},
	))

	customer := CustomerMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:       "CUST-DN-1",
		CustomerCode:     "CUST-DN-1",
		BusinessName:     "Serialized Customer",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	}
	require.NoError(t, app.db.Create(&customer).Error)

	product := ProductMaster{
		Base:                   Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:            "SER-DN-1",
		ProductName:            "Serialized DN Product",
		RequiresSerialTracking: true,
		StandardPriceBHD:       15,
		IsActive:               true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	order := Order{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber: "ORD-SER-DN-1",
		CustomerID:  customer.ID,
		Status:      "Confirmed",
	}
	require.NoError(t, app.db.Create(&order).Error)

	orderItem := OrderItem{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderID:     order.ID,
		ProductID:   product.ID,
		ProductCode: product.ProductCode,
		Description: product.ProductName,
		Quantity:    1,
		UnitPrice:   15,
	}
	require.NoError(t, app.db.Create(&orderItem).Error)

	_, err := app.CreateDeliveryNoteWithItems(order.ID, []DeliveryNoteItemInput{
		{
			OrderItemID: orderItem.ID,
			ShipQty:     1,
		},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "requires serial numbers")
}

func TestRegisterSerials_RejectsProductsWithoutSerialTracking(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&ProductMaster{}, &SerialNumber{}))

	product := ProductMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:      "NOSER-1",
		ProductName:      "Non Serialized Product",
		StandardPriceBHD: 10,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)

	_, err := app.RegisterSerials(product.ID, []string{"SN-001"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "not configured for serial tracking")
}
