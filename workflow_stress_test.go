package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedStressSupplierProduct(t *testing.T, app *App, prefix string, cost float64) (SupplierMaster, ProductMaster) {
	t.Helper()

	supplier := SupplierMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierCode: fmt.Sprintf("%s-SUP", prefix),
		SupplierName: fmt.Sprintf("%s Instruments", prefix),
		Country:      "Bahrain",
		PaymentTerms: "Net 30",
		Rating:       5,
	}
	require.NoError(t, app.db.Create(&supplier).Error)

	product := ProductMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:      fmt.Sprintf("%s-MODEL", prefix),
		ProductName:      fmt.Sprintf("%s transmitter", prefix),
		ProductCategory:  "Instrumentation",
		SupplierID:       supplier.ID,
		SupplierCode:     supplier.SupplierCode,
		StandardCostBHD:  cost,
		StandardPriceBHD: cost * 1.35,
		IsActive:         true,
	}
	require.NoError(t, app.db.Create(&product).Error)
	return supplier, product
}

func stressCostingPayload(customerName string, rfq RFQData, opportunity Opportunity, productA, productB ProductMaster, index int) CostingExportData {
	qtyA := 1 + (index % 3)
	qtyB := 2 + (index % 2)
	priceA := float64(qtyA) * 145
	priceB := float64(qtyB) * 210
	grandTotal := priceA + priceB
	totalCost := (float64(qtyA) * productA.StandardCostBHD) + (float64(qtyB) * productB.StandardCostBHD)

	return CostingExportData{
		Date:                time.Now().AddDate(0, 0, 45).Format("2006-01-02"),
		PreparedBy:          "Stress Admin",
		CustomerName:        customerName,
		RfqReference:        rfq.RFQRef,
		FolderNumber:        opportunity.FolderNumber,
		CostingId:           fmt.Sprintf("STRESS-COST-%03d", index),
		Subject:             fmt.Sprintf("Stress flow %03d instrumentation package", index),
		ProjectName:         opportunity.Title,
		OpportunityId:       rfq.ID,
		OpportunityRecordID: opportunity.ID,
		QuoteType:           "Quotation",
		VatRate:             10,
		PaymentTerms:        "Net 30",
		DeliveryTerms:       "DAP Bahrain",
		EstDelivery:         "5-7 weeks",
		Subtotal:            grandTotal,
		NetAmount:           grandTotal,
		VAT:                 grandTotal * 0.10,
		GrandTotal:          grandTotal,
		TotalCost:           totalCost,
		Profit:              grandTotal - totalCost,
		LineItems: []CostingExportLineItem{
			{
				SlNo:           1,
				Equipment:      "Pressure transmitter",
				Model:          productA.ProductCode,
				Quantity:       qtyA,
				Currency:       "BHD",
				TotalCost:      float64(qtyA) * productA.StandardCostBHD,
				MarginPercent:  18,
				SuggestedPrice: 145,
				TotalPrice:     priceA,
			},
			{
				SlNo:           2,
				Equipment:      "Flow meter",
				Model:          productB.ProductCode,
				Quantity:       qtyB,
				Currency:       "BHD",
				TotalCost:      float64(qtyB) * productB.StandardCostBHD,
				MarginPercent:  22,
				SuggestedPrice: 210,
				TotalPrice:     priceB,
			},
		},
	}
}

func TestStressCommercialProcurementFlowTraceability(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerName := "Stress Flow Customer W.L.L."
	customerID := seedTestCustomer(t, app.db, customerName)
	supplierA, productA := seedStressSupplierProduct(t, app, "ALPHA-STRESS", 72)
	supplierB, productB := seedStressSupplierProduct(t, app, "BETA-STRESS", 105)

	const flowCount = 30
	expectedWon := 0
	expectedLost := 0
	lostOfferIDs := make([]string, 0, flowCount/3)

	for i := 0; i < flowCount; i++ {
		rfqRef := fmt.Sprintf("STRESS-RFQ-%03d", i)
		rfq, err := app.CreateRFQWithReference(customerName, fmt.Sprintf("Stress project %03d", i), rfqRef, float64(1000+i), "stress intake")
		require.NoError(t, err)

		opportunity := Opportunity{
			Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			FolderNumber: fmt.Sprintf("STRESS-%03d-26", i),
			CustomerID:   customerID,
			CustomerName: customerName,
			Title:        fmt.Sprintf("Stress project %03d", i),
			Stage:        "Qualified",
			Year:         2026,
			OppNumber:    i + 1,
		}
		require.NoError(t, app.db.Create(&opportunity).Error)

		offer, err := app.SaveCostingAsOffer(stressCostingPayload(customerName, *rfq, opportunity, productA, productB, i))
		require.NoError(t, err)
		require.Equal(t, "Quoted", offer.Stage)
		require.Equal(t, customerID, offer.CustomerID)
		require.Len(t, offer.Items, 2)

		if i%3 == 0 {
			expectedLost++
			require.NoError(t, app.MarkOfferLost(offer.ID, "stress lost branch"))
			lostOfferIDs = append(lostOfferIDs, offer.ID)
			_, err := app.MarkOfferWon(offer.ID, fmt.Sprintf("CPO-STRESS-LOST-%03d", i))
			require.Error(t, err, "lost offer %s must not be recoverable into won/order state", offer.OfferNumber)
			continue
		}

		expectedWon++
		order, err := app.MarkOfferWon(offer.ID, fmt.Sprintf("CPO-STRESS-%03d", i))
		require.NoError(t, err)
		require.Equal(t, offer.ID, order.OfferID)
		require.Equal(t, customerID, order.CustomerID)
		require.Len(t, order.Items, 2)
		for _, item := range order.Items {
			require.NotEmpty(t, item.ProductCode)
			var product ProductMaster
			require.NoError(t, app.db.First(&product, "product_code = ?", item.ProductCode).Error)
			require.Equal(t, item.ProductCode, product.ProductCode)
			require.NotEmpty(t, product.SupplierID)
			resolvedProductSupplier, err := app.resolveSupplierForProduct(product)
			require.NoError(t, err)
			require.Equal(t, product.SupplierID, resolvedProductSupplier.ID)
			inferredSupplier, err := app.inferSupplierForOrderItem(item)
			require.NoError(t, err)
			require.Equal(t, product.SupplierID, inferredSupplier.ID)
		}

		pos, err := app.CreatePOsFromOrder(order.ID, nil)
		require.NoError(t, err)
		require.Len(t, pos, 2)

		seenSuppliers := map[string]bool{}
		for _, po := range pos {
			require.Equal(t, order.ID, po.OrderID)
			require.NotEmpty(t, po.PONumber)
			require.NotEmpty(t, po.Items)
			require.NoError(t, app.UpdatePOStatus(po.ID, "Sent"))
			require.NoError(t, app.UpdatePOStatus(po.ID, "Acknowledged"))

			var refreshed PurchaseOrder
			require.NoError(t, app.db.Preload("Items").First(&refreshed, "id = ?", po.ID).Error)
			require.Equal(t, "Acknowledged", refreshed.Status)
			require.Len(t, refreshed.Items, 1)
			require.NotEmpty(t, refreshed.Items[0].OrderItemID)
			seenSuppliers[refreshed.SupplierID] = true
		}
		require.True(t, seenSuppliers[supplierA.ID], "supplier A PO was not created")
		require.True(t, seenSuppliers[supplierB.ID], "supplier B PO was not created")
	}

	var offerCount, wonCount, lostCount, orderCount, poCount, poItemCount int64
	require.NoError(t, app.db.Model(&Offer{}).Count(&offerCount).Error)
	require.NoError(t, app.db.Model(&Offer{}).Where("stage = ?", "Won").Count(&wonCount).Error)
	require.NoError(t, app.db.Model(&Offer{}).Where("stage = ?", "Lost").Count(&lostCount).Error)
	require.NoError(t, app.db.Model(&Order{}).Count(&orderCount).Error)
	require.NoError(t, app.db.Model(&PurchaseOrder{}).Count(&poCount).Error)
	require.NoError(t, app.db.Model(&PurchaseOrderItem{}).Count(&poItemCount).Error)

	require.EqualValues(t, flowCount, offerCount)
	require.EqualValues(t, expectedWon, wonCount)
	require.EqualValues(t, expectedLost, lostCount)
	require.EqualValues(t, expectedWon, orderCount)
	require.EqualValues(t, expectedWon*2, poCount)
	require.EqualValues(t, expectedWon*2, poItemCount)

	var blankPOOrderLinks, blankPOItemOrderLinks, duplicatePONumbers int64
	require.NoError(t, app.db.Model(&PurchaseOrder{}).Where("order_id = '' OR order_id IS NULL").Count(&blankPOOrderLinks).Error)
	require.NoError(t, app.db.Model(&PurchaseOrderItem{}).Where("order_item_id = '' OR order_item_id IS NULL").Count(&blankPOItemOrderLinks).Error)
	require.NoError(t, app.db.Raw(`
		SELECT COUNT(*) FROM (
			SELECT po_number FROM purchase_orders GROUP BY po_number HAVING COUNT(*) > 1
		) duplicated_po_numbers
	`).Scan(&duplicatePONumbers).Error)
	require.Zero(t, blankPOOrderLinks)
	require.Zero(t, blankPOItemOrderLinks)
	require.Zero(t, duplicatePONumbers)

	for _, offerID := range lostOfferIDs {
		var linkedOrders int64
		require.NoError(t, app.db.Model(&Order{}).Where("offer_id = ?", offerID).Count(&linkedOrders).Error)
		require.Zero(t, linkedOrders)
	}
}

func TestStressMarkOfferWonRepeatedAttemptsDoNotDuplicateOrders(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	require.NoError(t, app.db.Exec("PRAGMA busy_timeout = 5000").Error)

	customerName := "Stress Race Customer W.L.L."
	customerID := seedTestCustomer(t, app.db, customerName)
	_, productA := seedStressSupplierProduct(t, app, "RACE-ALPHA", 64)
	_, productB := seedStressSupplierProduct(t, app, "RACE-BETA", 93)

	rfq, err := app.CreateRFQWithReference(customerName, "Concurrent won flow", "STRESS-RACE-RFQ", 1000, "race intake")
	require.NoError(t, err)
	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber: "STRESS-RACE-26",
		CustomerID:   customerID,
		CustomerName: customerName,
		Title:        "Concurrent won flow",
		Stage:        "Qualified",
		Year:         2026,
		OppNumber:    999,
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	offer, err := app.SaveCostingAsOffer(stressCostingPayload(customerName, *rfq, opportunity, productA, productB, 999))
	require.NoError(t, err)
	require.Equal(t, customerID, offer.CustomerID)

	firstOrder, err := app.MarkOfferWon(offer.ID, "CPO-STRESS-RACE")
	require.NoError(t, err)
	require.NotNil(t, firstOrder)

	const attempts = 16
	start := make(chan struct{})
	results := make(chan string, attempts)
	errors := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			order, err := app.MarkOfferWon(offer.ID, "CPO-STRESS-RACE")
			if err != nil {
				errors <- err
				return
			}
			results <- order.ID
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	close(errors)

	distinctResultIDs := map[string]bool{}
	for orderID := range results {
		distinctResultIDs[orderID] = true
	}
	for err := range errors {
		errText := strings.ToLower(err.Error())
		require.True(t,
			strings.Contains(errText, "concurrently") ||
				strings.Contains(errText, "already marked") ||
				strings.Contains(errText, "locked"),
			"unexpected MarkOfferWon race error: %v", err,
		)
	}

	var orders []Order
	require.NoError(t, app.db.Preload("Items").Where("offer_id = ?", offer.ID).Find(&orders).Error)
	require.Len(t, orders, 1)
	require.Equal(t, firstOrder.ID, orders[0].ID)
	require.Len(t, orders[0].Items, 2)
	require.Equal(t, "CPO-STRESS-RACE", orders[0].CustomerPONumber)
	if len(distinctResultIDs) > 0 {
		require.Len(t, distinctResultIDs, 1)
		require.True(t, distinctResultIDs[orders[0].ID])
	}
}
