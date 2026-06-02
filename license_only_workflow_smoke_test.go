package main

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func activateLicenseOnlyRole(t *testing.T, app *App, role, displayName string) {
	t.Helper()

	require.NoError(t, app.db.AutoMigrate(&LicenseKey{}))
	require.NoError(t, app.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&LicenseKey{}).Error)

	deviceHash := app.getDeviceHash()
	key := "PH-SLS-LIC001"
	switch role {
	case "admin":
		key = "PH-ADM-LIC001"
	case "manager":
		key = "PH-MGR-LIC001"
	case "operations":
		key = "PH-OPS-LIC001"
	case "staff":
		key = "PH-STF-LIC001"
	}

	now := time.Now()
	require.NoError(t, app.db.Create(&LicenseKey{
		Key:         key,
		Role:        role,
		DisplayName: displayName,
		Activated:   true,
		ActivatedAt: &now,
		DeviceHash:  deviceHash,
		CreatedBy:   "test",
	}).Error)

	app.currentUser = nil
	app.currentUserID = ""

	validation, err := app.ValidateLicense()
	require.NoError(t, err)
	require.True(t, validation.Valid)
	require.Equal(t, role, validation.Role)
}

func requireNonEmptyFile(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	require.NoError(t, err, "expected generated file %s to exist", path)
	require.Greater(t, info.Size(), int64(1024), "expected generated file %s to contain document bytes", path)
}

func TestLicenseOnlySalesWorkflowCreatesCommercialDocumentsAndTasks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	require.NoError(t, app.db.AutoMigrate(
		&CompanyBankAccount{},
		&InvoiceSequence{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
	))
	activateLicenseOnlyRole(t, app, "sales", "Sales Smoke")

	customerID := seedTestCustomer(t, app.db, "License Smoke Customer WLL")
	offer, err := app.SaveCostingAsOffer(CostingExportData{
		Date:          time.Now().Format("2006-01-02"),
		PreparedBy:    "Sales Smoke",
		CustomerName:  "License Smoke Customer WLL",
		RfqReference:  "LIC-RFQ-001",
		FolderNumber:  "LIC-OFFER-001",
		Subject:       "License-only offer smoke",
		QuoteType:     "Quotation",
		PaymentTerms:  "Net 30",
		DeliveryTerms: "DAP Bahrain",
		VatRate:       10,
		Subtotal:      100,
		NetAmount:     100,
		VAT:           10,
		GrandTotal:    110,
		Profit:        20,
		LineItems: []CostingExportLineItem{
			{
				SlNo:           1,
				Equipment:      "License-only pressure transmitter",
				Model:          "LIC-PT-100",
				Specification:  "4-20mA HART",
				Quantity:       1,
				SuggestedPrice: 100,
				TotalPrice:     100,
				TotalCost:      80,
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, offer.ID)
	require.Len(t, offer.Items, 1)

	offerPDF, err := app.GenerateOfferPDF(offer.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(offerPDF) })
	requireNonEmptyFile(t, offerPDF)

	supplierID := seedTestSupplier(t, app.db, "License Smoke Supplier GmbH")
	po, err := app.CreatePurchaseOrder(PurchaseOrder{
		OrderID:          "",
		SupplierID:       supplierID,
		PODate:           time.Now(),
		ExpectedDelivery: time.Now().AddDate(0, 0, 30),
		Currency:         "BHD",
		ExchangeRate:     1,
		PaymentTerms:     "Net 30",
		Division:         "PH Trading",
		Items: []PurchaseOrderItem{
			{
				Description:      "License-only pressure transmitter",
				Quantity:         1,
				UnitPriceForeign: 80,
			},
		},
	})
	require.NoError(t, err)
	require.NotEmpty(t, po.ID)
	require.NotEmpty(t, po.PONumber)
	require.Len(t, po.Items, 1)

	poPDF, err := app.GeneratePurchaseOrderPDF(po.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(poPDF) })
	requireNonEmptyFile(t, poPDF)

	orderID := uuid.New().String()
	order := Order{
		Base:              Base{ID: orderID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:       "ORD-LIC-001",
		CustomerID:        customerID,
		CustomerName:      "License Smoke Customer WLL",
		OrderDate:         time.Now(),
		RequiredDate:      time.Now().AddDate(0, 0, 30),
		TotalValueBHD:     100,
		GrandTotalBHD:     110,
		Status:            "Confirmed",
		PaymentTerms:      "Net 30",
		DeliveryTerms:     "DAP Bahrain",
		OfferID:           offer.ID,
		OfferNumber:       offer.OfferNumber,
		CustomerReference: "LIC-RFQ-001",
		Division:          "PH Trading",
	}
	require.NoError(t, app.db.Create(&order).Error)
	require.NoError(t, app.db.Create(&OrderItem{
		Base:                Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderID:             order.ID,
		LineNumber:          1,
		Description:         "License-only pressure transmitter",
		Quantity:            1,
		UnitPrice:           100,
		TotalPrice:          100,
		Equipment:           "License-only pressure transmitter",
		Model:               "LIC-PT-100",
		Specification:       "4-20mA HART",
		DetailedDescription: "License-only invoice smoke line",
		TotalCost:           80,
		MarginPercent:       20,
	}).Error)

	invoice, err := app.CreateInvoiceFromOrder(order.ID)
	require.NoError(t, err)
	require.NotEmpty(t, invoice.ID)
	require.NotEmpty(t, invoice.InvoiceNumber)
	require.Len(t, invoice.Items, 1)

	invoicePDF, err := app.GenerateInvoicePDF(invoice.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(invoicePDF) })
	requireNonEmptyFile(t, invoicePDF)

	task, err := app.CreateCollaborativeTask(TaskItem{
		Title:    "License-only task smoke",
		TaskType: "general",
		Priority: "high",
	})
	require.NoError(t, err)
	require.NotEmpty(t, task.ID)
	require.NotEmpty(t, task.CreatorEmployeeID)
	require.Equal(t, task.CreatorEmployeeID, task.CreatedBy)

	employeeCtx, err := app.GetCurrentEmployeeContext()
	require.NoError(t, err)
	require.NotEmpty(t, employeeCtx.EmployeeID)
	require.Equal(t, "sales", employeeCtx.LicenseRole)
}

func TestMasterKeyActivationIsBackendReadyImmediately(t *testing.T) {
	t.Setenv("ENABLE_DEVELOPER_MASTER_KEY", "true")
	t.Setenv("HOME", t.TempDir())
	resetLicenseAttemptLimiterForTest(t)

	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	app.currentUser = nil
	app.currentUserID = ""

	result, err := app.ActivateLicense(masterKey)
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, "admin", result.Role)

	validation, err := app.ValidateLicense()
	require.NoError(t, err)
	require.True(t, validation.Valid)
	require.Equal(t, masterKey, validation.Key)
	require.Equal(t, "admin", validation.Role)

	task, err := app.CreateCollaborativeTask(TaskItem{
		Title:    "Immediate master-key task",
		TaskType: "general",
		Priority: "medium",
	})
	require.NoError(t, err)
	require.NotEmpty(t, task.ID)
}
