package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func shippingGatePDFText(t *testing.T, path string) string {
	t.Helper()

	output, err := exec.Command("pdftotext", path, "-").Output()
	require.NoError(t, err, "PDF text extraction failed for %s", path)
	return strings.Join(strings.Fields(string(output)), " ")
}

func requirePDFText(t *testing.T, label string, text string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		require.Contains(t, text, fragment, "%s PDF text should contain %q", label, fragment)
	}
}

func TestShippingGateInvoiceFieldVisibilityMergesPartialSettings(t *testing.T) {
	defaults := parseFieldVisibility("")
	require.True(t, defaults.ShowEquipment)
	require.True(t, defaults.ShowSpecification)
	require.True(t, defaults.ShowRFQ)
	require.True(t, defaults.ShowContact)
	require.True(t, defaults.ShowCountryOrigin)
	require.True(t, defaults.ShowDeliveryWeeks)
	require.False(t, defaults.ShowDetailedDesc)
	require.False(t, defaults.ShowCost)
	require.False(t, defaults.ShowMargin)

	emptyJSON := parseFieldVisibility("{}")
	require.Equal(t, defaults, emptyJSON, "empty saved settings should not zero customer-facing invoice PDF fields")

	partial := parseFieldVisibility(`{"show_equipment":false,"show_detailed_desc":true}`)
	require.False(t, partial.ShowEquipment)
	require.True(t, partial.ShowDetailedDesc)
	require.True(t, partial.ShowSpecification)
	require.True(t, partial.ShowRFQ)
	require.True(t, partial.ShowContact)
	require.False(t, partial.ShowCost)
	require.False(t, partial.ShowMargin)
}

func TestShippingGateDocumentsPreserveCommercialTextAndFormat(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not available for commercial document text regression")
	}
	t.Setenv("HOME", t.TempDir())

	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	require.NoError(t, app.db.AutoMigrate(
		&CustomerContact{},
		&CompanyBankAccount{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
	))

	now := time.Date(2026, time.May, 12, 0, 0, 0, 0, time.UTC)
	customerID := seedTestCustomer(t, app.db, "BAPCO SHIPPING W.L.L.")
	require.NoError(t, app.db.Model(&CustomerMaster{}).
		Where("id = ?", customerID).
		Updates(map[string]interface{}{
			"trn":           "200000770400002",
			"address_line1": "Building 123, Road 456",
			"primary_email": "procurement@bapco.example",
			"primary_phone": "+973 17000000",
		}).Error)
	require.NoError(t, app.db.Create(&CustomerContact{
		Base:             Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		CustomerID:       customerID,
		ContactName:      "Vamshi",
		JobTitle:         "Procurement",
		Email:            "vamshi@bapco.example",
		Phone:            "+973 17770000",
		IsPrimaryContact: true,
	}).Error)

	offer := Offer{
		Base:               Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now, CreatedBy: "Periyasamy N"},
		OfferNumber:        "64-26",
		CustomerID:         customerID,
		CustomerName:       "BAPCO SHIPPING W.L.L.",
		QuotationDate:      now,
		ValidityDate:       now.AddDate(0, 0, 30),
		TotalValueBHD:      594,
		EstimatedMargin:    18,
		Stage:              "Quoted",
		PaymentTerms:       "30 days from Date of Delivery",
		DeliveryTerms:      "DAP Bahrain at your store or PH Trading",
		DeliveryWeeks:      "5-7 weeks",
		CountryOfOrigin:    "Germany",
		IssuedBy:           "Periyasamy N",
		CustomerReference:  "EH-64-26-R0",
		AttentionPerson:    "Vamshi",
		AttentionCompany:   "BAPCO SHIPPING W.L.L.",
		AttentionPhone:     "+973 17770000",
		QuoteType:          "Quotation",
		VatRate:            10,
		Division:           "PH Trading",
		Subject:            "Sub: Gas analyzer package",
		Body:               "Please find our pricing and scope below for your review.",
		TermsAndConditions: "1. QUOTATION VALIDITY\nThis quotation is valid for thirty (30) days from the date of issue.\n\n2. PRICES\nAll prices are in Bahraini Dinars (BHD) unless otherwise stated.",
	}
	require.NoError(t, app.db.Create(&offer).Error)
	require.NoError(t, app.db.Create(&OfferItem{
		Base:                Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		OfferID:             offer.ID,
		LineNumber:          1,
		ProductCode:         "FMD78-AEA7HE3AFMBW",
		Model:               "FMD78-AEA7HE3AFMBW",
		Description:         "Differential Pressure Indicator Transmitter",
		Quantity:            1,
		UnitPrice:           540,
		TotalPrice:          540,
		Equipment:           "Differential Pressure Indicator Transmitter",
		Specification:       "Deltabar FMD78",
		DetailedDescription: "NEMA4X/6P IP66/IP67, with SW filter",
		Currency:            "BHD",
		TotalCost:           430,
		MarginPercent:       20,
	}).Error)
	require.NoError(t, app.db.Create(&Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		FolderNumber: "64-26",
		OfferID:      offer.ID,
		CustomerID:   customerID,
		CustomerName: "BAPCO SHIPPING W.L.L.",
		Title:        "Gas analyzer package",
		EHRef:        "EH-64-26-R0",
		Stage:        "Quoted",
	}).Error)

	offerPDF, err := app.GenerateOfferPDF(offer.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(offerPDF) })
	offerText := shippingGatePDFText(t, offerPDF)
	requirePDFText(t, "offer", offerText,
		"QUOTATION",
		"64-26",
		"BAPCO SHIPPING W.L.L.",
		"Subject: Gas analyzer package",
		"Prepared By",
		"Periyasamy Neelakandan",
		"EH-64-26-R0",
		"30 days from Date of Delivery",
		"DAP Bahrain at your store or PH Trading",
		"Differential Pressure Indicator Transmitter",
		"FMD78-AEA7HE3AFMBW",
		"VAT (10%)",
		"periyasamy.n@platinumholdings.com.bh",
	)
	require.NotContains(t, offerText, "PHO-2026", "offer PDF must use the folder/reference number, not a generated yearly sequence")

	order := Order{
		Base:              Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now, CreatedBy: "Periyasamy N"},
		OrderNumber:       "ORD-SHIP-001",
		CustomerPONumber:  "PO-BAPCO-77",
		CustomerID:        customerID,
		CustomerName:      "BAPCO SHIPPING W.L.L.",
		OrderDate:         now,
		RequiredDate:      now.AddDate(0, 0, 30),
		TotalValueBHD:     540,
		GrandTotalBHD:     594,
		Status:            "Confirmed",
		PaymentTerms:      "30 days from Date of Delivery",
		DeliveryTerms:     "DAP Bahrain at your store or PH Trading",
		OfferID:           offer.ID,
		OfferNumber:       offer.OfferNumber,
		CustomerReference: "EH-64-26-R0",
		AttentionPerson:   "Vamshi",
		AttentionCompany:  "BAPCO SHIPPING W.L.L.",
		DeliveryWeeks:     "5-7 weeks",
		CountryOfOrigin:   "Germany",
		IssuedBy:          "Periyasamy N",
		Division:          "PH Trading",
	}
	require.NoError(t, app.db.Create(&order).Error)
	orderItem := OrderItem{
		Base:                Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		OrderID:             order.ID,
		LineNumber:          1,
		ProductCode:         "FMD78-AEA7HE3AFMBW",
		Description:         "Differential Pressure Indicator Transmitter",
		Quantity:            1,
		UnitPrice:           540,
		TotalPrice:          540,
		Equipment:           "Differential Pressure Indicator Transmitter",
		Model:               "FMD78-AEA7HE3AFMBW",
		Specification:       "Deltabar FMD78",
		DetailedDescription: "NEMA4X/6P IP66/IP67, with SW filter",
		TotalCost:           430,
		MarginPercent:       20,
	}
	require.NoError(t, app.db.Create(&orderItem).Error)

	invoice := Invoice{
		Base:                 Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now, CreatedBy: "Periyasamy N"},
		InvoiceNumber:        "INV-SHIP-001",
		InvoiceDate:          now,
		CustomerID:           customerID,
		CustomerName:         "BAPCO SHIPPING W.L.L.",
		OrderID:              order.ID,
		CustomerPONumber:     "PO-BAPCO-77",
		GrandTotalBHD:        594,
		Status:               "Sent",
		OutstandingBHD:       594,
		SubtotalBHD:          540,
		DueDate:              now.AddDate(0, 0, 30),
		OfferID:              offer.ID,
		OfferNumber:          offer.OfferNumber,
		CustomerReference:    "EH-64-26-R0",
		AttentionPerson:      "Vamshi",
		AttentionCompany:     "BAPCO SHIPPING W.L.L.",
		DeliveryWeeks:        "5-7 weeks",
		CountryOfOrigin:      "Germany",
		IssuedBy:             "Periyasamy N",
		PaymentTerms:         "30 days from Date of Delivery",
		DeliveryTerms:        "DAP Bahrain at your store or PH Trading",
		Division:             "PH Trading",
		DeliveryNoteRef:      "DN-SHIP-001",
		ModeOfPayment:        "Bank Transfer",
		SuppliersRef:         "E+H",
		OtherReferences:      "EH-64-26-R0",
		BuyersOrderNumber:    "PO-BAPCO-77",
		DespatchDocumentNo:   "DN-SHIP-001",
		DespatchedThrough:    "Direct",
		Destination:          "Bahrain",
		PlaceOfSupply:        "Kingdom of Bahrain",
		TermsOfDelivery:      "DAP Bahrain at your store or PH Trading",
		VATBHD:               54,
		VATPercent:           10,
		TotalSupplierCostBHD: 430,
		GrossMarginBHD:       110,
		GrossMarginPercent:   20.37,
	}
	require.NoError(t, app.db.Create(&invoice).Error)
	require.NoError(t, app.db.Create(&DBInvoiceItem{
		Base:                Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		InvoiceID:           invoice.ID,
		LineNumber:          1,
		Description:         "Differential Pressure Indicator Transmitter",
		Quantity:            1,
		Rate:                540,
		TotalBHD:            540,
		ProductCode:         "FMD78-AEA7HE3AFMBW",
		Equipment:           "Differential Pressure Indicator Transmitter",
		Model:               "FMD78-AEA7HE3AFMBW",
		Specification:       "Deltabar FMD78",
		DetailedDescription: "NEMA4X/6P IP66/IP67, with SW filter",
		TotalCost:           430,
		MarginPercent:       20,
		TotalPrice:          540,
	}).Error)

	invoicePDF, err := app.GenerateInvoicePDF(invoice.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(invoicePDF) })
	invoiceText := shippingGatePDFText(t, invoicePDF)
	requirePDFText(t, "invoice", invoiceText,
		"TAX INVOICE",
		"INV-SHIP-001",
		"BAPCO SHIPPING W.L.L.",
		"PO-BAPCO-77",
		"DN-SHIP-001",
		"EH-64-26-R0",
		"Differential Pressure Indicator Transmitte",
		"Output VAT",
		"54.000 BHD",
		"Periyasamy Neelakandan",
	)

	supplierID := seedTestSupplier(t, app.db, "Endress+Hauser Germany")
	po := PurchaseOrder{
		Base:             Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now, CreatedBy: "Periyasamy N"},
		OrderID:          order.ID,
		SupplierID:       supplierID,
		SupplierName:     "Endress+Hauser Germany",
		PONumber:         "PO-SHIP-001",
		PODate:           now,
		ExpectedDelivery: now.AddDate(0, 0, 45),
		Currency:         "BHD",
		ExchangeRate:     1,
		SubtotalForeign:  430,
		SubtotalBHD:      430,
		VATAmount:        43,
		TotalForeign:     473,
		TotalBHD:         473,
		PaymentTerms:     "Net 30",
		PaymentDueDate:   now.AddDate(0, 0, 30),
		Status:           "Approved",
		ApprovedBy:       "Periyasamy N",
		Division:         "PH Trading",
	}
	require.NoError(t, app.db.Create(&po).Error)
	require.NoError(t, app.db.Create(&PurchaseOrderItem{
		Base:               Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		PurchaseOrderID:    po.ID,
		OrderItemID:        orderItem.ID,
		ProductCode:        "FMD78-AEA7HE3AFMBW",
		SupplierPartNumber: "E+H-FMD78",
		Description:        "Differential Pressure Indicator Transmitter",
		Quantity:           1,
		UnitPriceForeign:   430,
		UnitPriceBHD:       430,
		TotalForeign:       430,
		TotalBHD:           430,
	}).Error)

	poPDF, err := app.GeneratePurchaseOrderPDF(po.ID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(poPDF) })
	poText := shippingGatePDFText(t, poPDF)
	requirePDFText(t, "purchase order", poText,
		"PURCHASE ORDER",
		"PO-SHIP-001",
		"Endress+Hauser Germany",
		"Payment Terms",
		"Net 30",
		"Differential Pressure Indicator Transmitter",
		"E+H-FMD78",
		"Periyasamy Neelakandan",
	)
}

func shippingGateAppForRole(t *testing.T, roleName string) *App {
	t.Helper()

	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	require.NoError(t, app.db.AutoMigrate(&Role{}, &LicenseKey{}))
	require.NoError(t, app.SeedDefaultRoles())

	var role Role
	require.NoError(t, app.db.Where("name = ?", roleName).First(&role).Error)
	app.currentUserID = roleName + "-shipping-user"
	app.currentUser = &User{
		Base:     Base{ID: app.currentUserID},
		Username: roleName + "-shipping",
		RoleName: roleName,
		Role:     role,
	}
	return app
}

func shippingGateOfferData(roleName string) CostingExportData {
	return CostingExportData{
		Date:         time.Now().Format("2006-01-02"),
		PreparedBy:   "Periyasamy N",
		CustomerName: "Shipping Gate Customer " + roleName,
		RfqReference: "SG-RFQ-" + roleName,
		FolderNumber: "SG-OFFER-" + roleName,
		Subject:      "Shipping gate offer",
		QuoteType:    "Quotation",
		VatRate:      10,
		Subtotal:     100,
		NetAmount:    100,
		VAT:          10,
		GrandTotal:   110,
		Profit:       20,
		LineItems: []CostingExportLineItem{
			{SlNo: 1, Equipment: "Shipping gate item", Model: "SG-1", Quantity: 1, SuggestedPrice: 100, TotalPrice: 100, TotalCost: 80},
		},
	}
}

func shippingGateSeedOrderForInvoice(t *testing.T, app *App, roleName string) string {
	t.Helper()

	customerName := "Invoice Gate Customer " + roleName
	customerID := seedTestCustomer(t, app.db, customerName)
	order := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:   "ORD-INV-GATE-" + roleName,
		CustomerID:    customerID,
		CustomerName:  customerName,
		OrderDate:     time.Now(),
		RequiredDate:  time.Now().AddDate(0, 0, 30),
		TotalValueBHD: 100,
		GrandTotalBHD: 110,
		Status:        "Confirmed",
		PaymentTerms:  "Net 30",
		DeliveryTerms: "DAP Bahrain",
	}
	require.NoError(t, app.db.Create(&order).Error)
	require.NoError(t, app.db.Create(&OrderItem{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderID:     order.ID,
		LineNumber:  1,
		Description: "Invoice gate item",
		Quantity:    1,
		UnitPrice:   100,
		TotalPrice:  100,
	}).Error)
	return order.ID
}

func TestShippingGateRoleWorkflowCreationMatrixUsesRealMethods(t *testing.T) {
	tests := []struct {
		workflow string
		role     string
		allowed  bool
		run      func(t *testing.T, app *App, roleName string) error
	}{
		{"offer", "admin", true, func(t *testing.T, app *App, role string) error {
			_, err := app.SaveCostingAsOffer(shippingGateOfferData(role))
			return err
		}},
		{"offer", "manager", true, func(t *testing.T, app *App, role string) error {
			_, err := app.SaveCostingAsOffer(shippingGateOfferData(role))
			return err
		}},
		{"offer", "sales", true, func(t *testing.T, app *App, role string) error {
			_, err := app.SaveCostingAsOffer(shippingGateOfferData(role))
			return err
		}},
		{"offer", "operations", false, func(t *testing.T, app *App, role string) error {
			_, err := app.SaveCostingAsOffer(shippingGateOfferData(role))
			return err
		}},
		{"offer", "staff", false, func(t *testing.T, app *App, role string) error {
			_, err := app.SaveCostingAsOffer(shippingGateOfferData(role))
			return err
		}},
		{"invoice", "admin", true, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateInvoiceFromOrder(shippingGateSeedOrderForInvoice(t, app, role))
			return err
		}},
		{"invoice", "manager", true, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateInvoiceFromOrder(shippingGateSeedOrderForInvoice(t, app, role))
			return err
		}},
		{"invoice", "sales", true, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateInvoiceFromOrder(shippingGateSeedOrderForInvoice(t, app, role))
			return err
		}},
		{"invoice", "operations", true, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateInvoiceFromOrder(shippingGateSeedOrderForInvoice(t, app, role))
			return err
		}},
		{"invoice", "staff", false, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateInvoiceFromOrder(shippingGateSeedOrderForInvoice(t, app, role))
			return err
		}},
		{"order", "admin", true, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateOrder("ORD-GATE-"+role, "Order Gate Customer "+role, 100, time.Now().Format("2006-01-02"), "Confirmed")
			return err
		}},
		{"order", "manager", true, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateOrder("ORD-GATE-"+role, "Order Gate Customer "+role, 100, time.Now().Format("2006-01-02"), "Confirmed")
			return err
		}},
		{"order", "sales", true, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateOrder("ORD-GATE-"+role, "Order Gate Customer "+role, 100, time.Now().Format("2006-01-02"), "Confirmed")
			return err
		}},
		{"order", "operations", false, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateOrder("ORD-GATE-"+role, "Order Gate Customer "+role, 100, time.Now().Format("2006-01-02"), "Confirmed")
			return err
		}},
		{"order", "staff", false, func(t *testing.T, app *App, role string) error {
			_, err := app.CreateOrder("ORD-GATE-"+role, "Order Gate Customer "+role, 100, time.Now().Format("2006-01-02"), "Confirmed")
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.workflow+"/"+tt.role, func(t *testing.T) {
			app := shippingGateAppForRole(t, tt.role)
			err := tt.run(t, app, tt.role)
			if tt.allowed {
				require.NoError(t, err, "%s should be creatable by %s", tt.workflow, tt.role)
				return
			}
			require.Error(t, err, "%s should not be creatable by %s", tt.workflow, tt.role)
			require.Contains(t, strings.ToLower(err.Error()), "permission")
		})
	}
}

func TestShippingGateNotificationTablesSyncAcrossDevicePaths(t *testing.T) {
	for _, table := range []string{"task_items", "task_comments", "task_activity", "notifications", "notification_receipts"} {
		require.Contains(t, dbSyncTables, table, "cloud DB sync must include %s", table)
		require.Contains(t, collaborativeSyncTables, table, "collaboration sync must include %s", table)
	}
	require.Less(t,
		indexOfString(dbSyncTables, "task_activity"),
		indexOfString(dbSyncTables, "notifications"),
		"task activity should sync before notifications so remote devices can resolve task context")
	require.Less(t,
		indexOfString(dbSyncTables, "notifications"),
		indexOfString(dbSyncTables, "notification_receipts"),
		"notifications should sync before per-device receipts")
	require.Less(t,
		indexOfString(collaborativeSyncTables, "notifications"),
		indexOfString(collaborativeSyncTables, "notification_receipts"),
		"collaboration sync should deliver notifications before receipts")
}

func indexOfString(values []string, target string) int {
	for index, value := range values {
		if value == target {
			return index
		}
	}
	return -1
}
