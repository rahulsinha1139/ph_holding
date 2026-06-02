package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func migrateWorkflowRegressionTables(t *testing.T, app *App) {
	t.Helper()
	require.NoError(t, app.db.AutoMigrate(
		&Role{},
		&User{},
		&LicenseKey{},
		&CustomerMaster{},
		&RFQData{},
		&CostingSheetData{},
		&Opportunity{},
		&Offer{},
		&OfferItem{},
		&OfferNote{},
		&OfferFollowUp{},
		&DeleteApprovalRequest{},
		&Order{},
		&OrderItem{},
		&Invoice{},
		&DBInvoiceItem{},
		&Payment{},
		&ProductMaster{},
		&SupplierMaster{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&SupplierContact{},
		&EntityNote{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&SupplierPayment{},
		&BankStatement{},
		&BankStatementLine{},
		&BankReconciliationAuditLog{},
		&Employee{},
		&EmployeeAccessLink{},
		&TaskItem{},
		&TaskComment{},
		&TaskActivity{},
		&Notification{},
		&NotificationReceipt{},
		&CollaborativePendingOperation{},
	))
}

func TestCreateRFQWithReferenceUsesUserReference(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	rfq, err := app.CreateRFQWithReference("BAPCO", "Analyzer skid", "EH-REF-2026-001", 1250, "Urgent enquiry")
	require.NoError(t, err)
	require.Equal(t, "EH-REF-2026-001", rfq.RFQNumber)
	require.Equal(t, "EH-REF-2026-001", rfq.RFQRef)

	var stored RFQData
	require.NoError(t, app.db.First(&stored, "id = ?", rfq.ID).Error)
	require.Equal(t, "EH-REF-2026-001", stored.RFQNumber)
	require.Equal(t, "EH-REF-2026-001", stored.RFQRef)
}

func TestSaveCostingAsOfferPropagatesReferenceToLinkedOpportunity(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber: "2-26",
		CustomerName: "BAPCO",
		Title:        "Analyzer skid",
		Stage:        "Qualified",
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	offer, err := app.SaveCostingAsOffer(CostingExportData{
		Date:                "2026-05-02",
		PreparedBy:          "Peri",
		CustomerName:        "BAPCO",
		RfqReference:        "EH-REF-2026-777",
		FolderNumber:        "2-26",
		CostingId:           "CS-EH-REF-2026-777",
		Subject:             "Analyzer skid",
		ProjectName:         "Analyzer skid",
		OpportunityRecordID: opportunity.ID,
		QuoteType:           "Quotation",
		VatRate:             10,
		Subtotal:            100,
		NetAmount:           100,
		VAT:                 10,
		GrandTotal:          110,
		TotalCost:           80,
		Profit:              30,
		LineItems: []CostingExportLineItem{
			{
				SlNo:           1,
				Equipment:      "Analyzer",
				Quantity:       1,
				Currency:       "BHD",
				SuggestedPrice: 100,
				TotalPrice:     100,
				TotalCost:      80,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "EH-REF-2026-777", offer.CustomerReference)

	var storedOffer Offer
	require.NoError(t, app.db.First(&storedOffer, "id = ?", offer.ID).Error)
	require.Equal(t, "EH-REF-2026-777", storedOffer.CustomerReference)

	var linked Opportunity
	require.NoError(t, app.db.First(&linked, "id = ?", opportunity.ID).Error)
	require.Equal(t, offer.ID, linked.OfferID)
	require.Equal(t, "Quoted", linked.Stage)
	require.Equal(t, "EH-REF-2026-777", linked.EHRef)
	require.Equal(t, "Analyzer skid", linked.Title)
}

func TestSaveCostingAsOfferUsesFolderNumberAsOfferNumber(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	offer, err := app.SaveCostingAsOffer(CostingExportData{
		Date:         "2026-05-02",
		PreparedBy:   "Peri",
		CustomerName: "BAPCO",
		RfqReference: "EH-REF-2026-888",
		FolderNumber: "26-26",
		CostingId:    "CS-EH-REF-2026-888",
		Subject:      "Analyzer skid",
		QuoteType:    "Quotation",
		VatRate:      10,
		Subtotal:     100,
		NetAmount:    100,
		VAT:          10,
		GrandTotal:   110,
		TotalCost:    80,
		Profit:       30,
		LineItems: []CostingExportLineItem{
			{
				SlNo:           1,
				Equipment:      "Analyzer",
				Quantity:       1,
				Currency:       "BHD",
				SuggestedPrice: 100,
				TotalPrice:     100,
				TotalCost:      80,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "26-26", offer.OfferNumber)
	require.Equal(t, 0, offer.RevisionNumber)
}

func TestUpdateOfferFullSavesEditableOfferNumberAndRevisionOne(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	now := time.Now()
	offer := Offer{
		Base:           Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		OfferNumber:    "TEMP-26",
		RevisionNumber: 0,
		CustomerName:   "BAPCO",
		QuotationDate:  now.AddDate(0, 0, -1),
		ValidityDate:   now.AddDate(0, 0, 30),
		Stage:          "Quoted",
		TotalValueBHD:  100,
	}
	require.NoError(t, app.db.Create(&offer).Error)

	updated, err := app.UpdateOfferFull(offer.ID, OfferUpdateData{
		OfferNumber:   "26-26",
		CustomerName:  offer.CustomerName,
		QuotationDate: offer.QuotationDate.Format("2006-01-02"),
		ValidityDate:  offer.ValidityDate.Format("2006-01-02"),
		PaymentTerms:  "Net 30",
		Items:         []OfferUpdateItem{},
	})
	require.NoError(t, err)
	require.Equal(t, "26-26", updated.OfferNumber)
	require.Equal(t, 1, updated.RevisionNumber)
}

func TestGetOpportunityLineItems_FallsBackToProductDetails(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	seedItems := []OfferItem{
		{
			LineNumber:    1,
			Description:   "Flow transmitter",
			Quantity:      2,
			UnitPrice:     540.758,
			TotalPrice:    1081.516,
			Equipment:     "Promass F",
			ProductCode:   "PROMASS-F",
			Specification: "Coriolis flow meter",
		},
	}

	opportunity := Opportunity{
		Base:           Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber:   "2026-501",
		CustomerName:   "BAPCO",
		Title:          "Fallback item coverage",
		Stage:          "Qualified",
		ProductDetails: serializeOpportunityProductDetailsFromOfferItems(seedItems),
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	items, err := app.GetOpportunityLineItems(opportunity.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Flow transmitter", items[0].Description)
	require.Equal(t, 2.0, items[0].Quantity)
	require.Equal(t, 540.758, items[0].UnitPrice)
	require.Equal(t, 1081.516, items[0].TotalPrice)
}

func TestNormalizeOpportunityLineItems_DoesNotStoreCurrencyAsUnit(t *testing.T) {
	rawItems := []interface{}{
		map[string]interface{}{
			"description": "Bray 12 inch",
			"quantity":    8,
			"unit":        "EUR",
			"unit_price":  2737,
			"total_price": 21895,
		},
	}

	normalized, count := normalizeOpportunityLineItemsJSON(rawItems)
	require.Equal(t, 1, count)

	var rows []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(normalized), &rows))
	require.Len(t, rows, 1)
	require.NotContains(t, rows[0], "unit")
	require.Equal(t, "EUR", rows[0]["currency"])
	require.Equal(t, float64(8), rows[0]["quantity"])
}

func TestNormalizeOpportunityLineItems_ParsesCurrencyPrefixedMoney(t *testing.T) {
	rawItems := []interface{}{
		map[string]interface{}{
			"description": "Bray 20 inch",
			"quantity":    "5",
			"unit_price":  "EUR 3,176.00",
			"total_price": "EUR 15,880.00",
		},
	}

	normalized, count := normalizeOpportunityLineItemsJSON(rawItems)
	require.Equal(t, 1, count)

	var rows []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(normalized), &rows))
	require.Len(t, rows, 1)
	require.Equal(t, float64(3176), rows[0]["unit_price"])
	require.Equal(t, float64(15880), rows[0]["total_price"])
}

func TestNormalizeOpportunityLineItems_RejectsPriceOnlyRows(t *testing.T) {
	rawItems := []interface{}{
		map[string]interface{}{
			"unit_price":  2737,
			"total_price": 21895,
		},
	}

	normalized, count := normalizeOpportunityLineItemsJSON(rawItems)
	require.Equal(t, 0, count)
	require.Empty(t, normalized)
}

func TestGroundedCapabilitiesFastPath_IncludesDatabaseCoverage(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	reply, handled := app.tryGroundedCapabilitiesFastPath(Intent{}, "hey buddy! Can you tell us what you can do?", true)
	require.True(t, handled)
	require.Contains(t, reply, "Database tables indexed for guarded querying")
	require.Contains(t, reply, "controlled ERP access paths")
}

func TestBuildFullContext_IncludesCrossModuleDatabaseAndWorkAccess(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	adminRole := Role{
		Base:        Base{ID: uuid.New().String()},
		Name:        "admin",
		DisplayName: "Admin",
		Permissions: `["*","finance:view","intelligence:chat","tasks:view","notifications:view"]`,
		IsActive:    true,
	}
	require.NoError(t, app.db.Create(&adminRole).Error)
	app.currentUser = &User{
		Base:     Base{ID: "butler-admin"},
		Username: "butler-admin",
		RoleName: "admin",
		RoleID:   adminRole.ID,
		Role:     adminRole,
	}
	app.currentUserID = "butler-admin"

	customer := CustomerMaster{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:    "C-BAPCO-001",
		BusinessName:  "BAPCO",
		CustomerGrade: "A",
		PaymentGrade:  "A",
		Industry:      "Energy",
	}
	require.NoError(t, app.db.Create(&customer).Error)

	require.NoError(t, app.db.Create(&EntityNote{
		Base:       Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		EntityType: "customer",
		EntityID:   customer.ID,
		NoteType:   "general",
		Content:    "BAPCO prefers calibration packs and consolidated follow-up notes.",
	}).Error)

	offer := Offer{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:   "OFF-CTX-001",
		CustomerID:    customer.CustomerID,
		CustomerName:  customer.BusinessName,
		QuotationDate: time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC),
		ValidityDate:  time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		Stage:         "Quoted",
		TotalValueBHD: 1500,
		Division:      "PH Trading",
	}
	require.NoError(t, app.db.Create(&offer).Error)
	require.NoError(t, app.db.Create(&OfferItem{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:       offer.ID,
		LineNumber:    1,
		Description:   "Calibration service visit",
		Equipment:     "Flowmeter Calibration",
		Model:         "CAL-PACK",
		Quantity:      2,
		UnitPrice:     750,
		TotalPrice:    1500,
		MarginPercent: 32,
	}).Error)
	require.NoError(t, app.db.Create(&OfferNote{
		Base:     Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:  offer.ID,
		NoteDate: time.Date(2026, 4, 11, 0, 0, 0, 0, time.UTC),
		Content:  "Customer requested split pricing and service mobilization note.",
	}).Error)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "bank-1",
		StatementNumber: "BS-CTX-001",
		StatementDate:   time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC),
		PeriodStart:     time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:       time.Date(2026, 4, 12, 0, 0, 0, 0, time.UTC),
		Status:          "Imported",
		Notes:           "BAPCO statement contains Peri follow-up marker for unmatched service receipts.",
		Division:        "PH Trading",
	}
	require.NoError(t, app.db.Create(&statement).Error)

	peri := Employee{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		EmployeeCode:     "EMP-002",
		FullName:         "Peri",
		PreferredName:    "Peri",
		Department:       "Operations",
		JobTitle:         "Coordinator",
		EmploymentStatus: "active",
		IsActive:         true,
		Notes:            "Handles service coordination and follow-up tasks.",
	}
	require.NoError(t, app.db.Create(&peri).Error)

	task := TaskItem{
		Base:               Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Title:              "Review BAPCO calibration follow-up",
		Description:        "Check statement note and confirm the service offer line items.",
		Status:             "open",
		Priority:           "high",
		TaskType:           "service",
		AssigneeEmployeeID: &peri.ID,
		CreatorEmployeeID:  peri.ID,
		CustomerID:         &customer.ID,
	}
	require.NoError(t, app.db.Create(&task).Error)
	require.NoError(t, app.db.Create(&TaskComment{
		Base:       Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		TaskID:     task.ID,
		EmployeeID: peri.ID,
		Body:       "Peri noted that the bank statement and offer notes need a single client response.",
	}).Error)

	ctx := app.buildFullContext(Intent{
		RawQuery:   "How many tasks are assigned to Peri and what notes or bank statements do we have for BAPCO?",
		Domain:     "general",
		EntityName: "BAPCO",
		PersonName: "Peri",
	})

	workData, ok := ctx["work_data"].(map[string]interface{})
	require.True(t, ok)
	require.NotEmpty(t, workData["tasks"])

	databaseAccess, ok := ctx["database_access"].(map[string]interface{})
	require.True(t, ok)

	databaseCatalog, ok := ctx["database_catalog"].(map[string]interface{})
	require.True(t, ok)
	require.NotZero(t, databaseCatalog["total_tables"])
	require.NotEmpty(t, databaseCatalog["tables"])

	notesLedger, ok := databaseAccess["notes_ledger"].(map[string]interface{})
	require.True(t, ok)
	require.NotEmpty(t, notesLedger["bank_statement_notes"])
	require.NotEmpty(t, notesLedger["offer_notes"])

	commercialRecords, ok := databaseAccess["commercial_records"].(map[string]interface{})
	require.True(t, ok)
	offers, ok := commercialRecords["offers"].([]map[string]interface{})
	require.True(t, ok)
	require.NotEmpty(t, offers)
	require.NotEmpty(t, offers[0]["line_items"])
}

func TestGetOpportunityLineItems_FallsBackWhenLinkedOfferHasNoItems(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	offer := Offer{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:   "PHO-TEST-001",
		CustomerName:  "BAPCO",
		QuotationDate: time.Now(),
		ValidityDate:  time.Now().AddDate(0, 0, 30),
		Stage:         "Quoted",
	}
	require.NoError(t, app.db.Create(&offer).Error)

	seedItems := []OfferItem{
		{
			LineNumber:  1,
			Description: "Pressure switch",
			Quantity:    1,
			UnitPrice:   321.123,
			TotalPrice:  321.123,
		},
	}

	opportunity := Opportunity{
		Base:           Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber:   "2026-502",
		OfferID:        offer.ID,
		CustomerName:   "BAPCO",
		Title:          "Linked offer fallback",
		Stage:          "Quoted",
		ProductDetails: serializeOpportunityProductDetailsFromOfferItems(seedItems),
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	items, err := app.GetOpportunityLineItems(opportunity.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, "Pressure switch", items[0].Description)
	require.Equal(t, 321.123, items[0].UnitPrice)
}

func TestDeleteOffer_AdminSoftDeletesUnconvertedOfferAndClearsOpportunity(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	offerID := uuid.New().String()
	offer := Offer{
		Base:          Base{ID: offerID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:   "DEL-41-26",
		CustomerName:  "Sedres/Orbitus",
		QuotationDate: time.Now(),
		ValidityDate:  time.Now().AddDate(0, 0, 30),
		Stage:         "Quoted",
		TotalValueBHD: 2783,
	}
	require.NoError(t, app.db.Create(&offer).Error)
	require.NoError(t, app.db.Create(&OfferItem{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:     offerID,
		LineNumber:  1,
		Description: "Flow meter",
		Quantity:    1,
		UnitPrice:   2530,
		TotalPrice:  2530,
	}).Error)
	require.NoError(t, app.db.Create(&OfferNote{
		Base:     Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:  offerID,
		NoteDate: time.Now(),
		Content:  "Internal note",
	}).Error)
	require.NoError(t, app.db.Create(&OfferFollowUp{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:      offerID,
		FollowUpDate: time.Now().AddDate(0, 0, 7),
		Status:       "pending",
	}).Error)
	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber: "41-26",
		CustomerName: "Sedres/Orbitus",
		Title:        "Test Project",
		Stage:        "Quoted",
		OfferID:      offerID,
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	require.NoError(t, app.DeleteOffer(offerID))

	var deleted Offer
	require.NoError(t, app.db.Unscoped().First(&deleted, "id = ?", offerID).Error)
	require.True(t, deleted.DeletedAt.Valid)

	var liveItems int64
	require.NoError(t, app.db.Model(&OfferItem{}).Where("offer_id = ?", offerID).Count(&liveItems).Error)
	require.Zero(t, liveItems)
	var liveNotes int64
	require.NoError(t, app.db.Model(&OfferNote{}).Where("offer_id = ?", offerID).Count(&liveNotes).Error)
	require.Zero(t, liveNotes)
	var liveFollowUps int64
	require.NoError(t, app.db.Model(&OfferFollowUp{}).Where("offer_id = ?", offerID).Count(&liveFollowUps).Error)
	require.Zero(t, liveFollowUps)

	var relinked Opportunity
	require.NoError(t, app.db.First(&relinked, "id = ?", opportunity.ID).Error)
	require.Empty(t, relinked.OfferID)
	require.Equal(t, "Proposal", relinked.Stage)
}

func TestDeleteOffer_BlocksConvertedOfferWithOrder(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	offerID := uuid.New().String()
	require.NoError(t, app.db.Create(&Offer{
		Base:          Base{ID: offerID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:   "LOCK-42-26",
		CustomerName:  "BAPCO",
		QuotationDate: time.Now(),
		ValidityDate:  time.Now().AddDate(0, 0, 30),
		Stage:         "Quoted",
	}).Error)
	require.NoError(t, app.db.Create(&Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:   "ORD-LOCK-42",
		CustomerName:  "BAPCO",
		OrderDate:     time.Now(),
		Status:        "Confirmed",
		OfferID:       offerID,
		OfferNumber:   "LOCK-42-26",
		TotalValueBHD: 100,
		GrandTotalBHD: 100,
	}).Error)

	err := app.DeleteOffer(offerID)
	require.ErrorContains(t, err, "linked order")

	var live Offer
	require.NoError(t, app.db.First(&live, "id = ?", offerID).Error)
	require.Equal(t, "LOCK-42-26", live.OfferNumber)
}

func TestUpdateOpportunityDetails_PreservesOwnerNotesForNonManagement(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	app.currentUser = &User{
		Base:     Base{ID: "sales-user"},
		Username: "ebin",
		RoleName: "sales",
		Role: Role{
			Name:        "sales",
			DisplayName: "Sales",
			Permissions: `["offers:edit"]`,
		},
	}
	app.currentUserID = "sales-user"

	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber: "2026-503",
		CustomerName: "ACTIUM PG",
		Title:        "Editable note coverage",
		Comment:      "Original note",
		OwnerNotes:   "Management only",
		Stage:        "Qualified",
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	updated, err := app.UpdateOpportunityDetails(opportunity.ID, "  refreshed commercial note  ", "should not replace")
	require.NoError(t, err)
	require.Equal(t, "refreshed commercial note", updated.Comment)
	require.Equal(t, "Management only", updated.OwnerNotes)

	var stored Opportunity
	require.NoError(t, app.db.First(&stored, "id = ?", opportunity.ID).Error)
	require.Equal(t, "refreshed commercial note", stored.Comment)
	require.Equal(t, "Management only", stored.OwnerNotes)
}

func TestUpdateRFQNotes_AllowsClearingNotes(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	rfq := RFQData{
		RFQNumber: "RFQ-2026-777",
		Client:    "BAPCO",
		Project:   "Clear note coverage",
		Value:     2500,
		Notes:     "legacy note",
		Status:    "New",
		Stage:     "RFQ Received",
	}
	require.NoError(t, app.db.Create(&rfq).Error)

	updated, err := app.UpdateRFQNotes(rfq.ID, "   ")
	require.NoError(t, err)
	require.Equal(t, "", updated.Notes)

	var stored RFQData
	require.NoError(t, app.db.First(&stored, "id = ?", rfq.ID).Error)
	require.Equal(t, "", stored.Notes)
}

func TestSeedDefaultRoles_SalesCanCreatePurchaseOrders(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	require.NoError(t, app.SeedDefaultRoles())
	require.True(t, app.CheckPermissionByRole("sales", "po:create"))
	require.True(t, app.CheckPermissionByRole("sales", "suppliers:create"))
	require.True(t, app.CheckPermissionByRole("sales", "orders:update"))
}

func createPurchaseOrderRegressionFixture(t *testing.T, app *App, unitPrice float64) PurchaseOrder {
	t.Helper()
	require.NoError(t, app.db.AutoMigrate(&PurchaseOrder{}, &PurchaseOrderItem{}, &SupplierMaster{}))

	supplier := SupplierMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierCode: "SUP-PO-" + uuid.New().String()[:8],
		SupplierName: "PO Regression Supplier",
	}
	require.NoError(t, app.db.Create(&supplier).Error)

	po, err := app.CreatePurchaseOrder(PurchaseOrder{
		SupplierID:       supplier.ID,
		PODate:           time.Now(),
		ExpectedDelivery: time.Now().AddDate(0, 0, 14),
		Currency:         "BHD",
		ExchangeRate:     1,
		PaymentTerms:     "Net 30",
		Items: []PurchaseOrderItem{
			{
				Description:      "Regression pressure transmitter",
				Quantity:         1,
				UnitPriceForeign: unitPrice,
			},
		},
	})
	require.NoError(t, err)
	return po
}

func TestUpdatePOStatusAcceptsLegacyFrontendAliasesAndStoresCanonicalStatuses(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	po := createPurchaseOrderRegressionFixture(t, app, 100)
	require.Equal(t, "Draft", po.Status)

	require.NoError(t, app.UpdatePOStatus(po.ID, "Sent"))
	require.NoError(t, app.UpdatePOStatus(po.ID, "Acknowledged"))
	require.NoError(t, app.UpdatePOStatus(po.ID, "PartiallyReceived"))

	var stored PurchaseOrder
	require.NoError(t, app.db.First(&stored, "id = ?", po.ID).Error)
	require.Equal(t, "Partially Received", stored.Status)

	require.NoError(t, app.UpdatePOStatus(po.ID, "FullyReceived"))
	require.NoError(t, app.db.First(&stored, "id = ?", po.ID).Error)
	require.Equal(t, "Received", stored.Status)

	require.NoError(t, app.UpdatePOStatus(po.ID, "Closed"))
	require.NoError(t, app.db.First(&stored, "id = ?", po.ID).Error)
	require.Equal(t, "Closed", stored.Status)
}

func TestHighValuePurchaseOrderApprovalUsesCurrentSessionActor(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	app.currentUserID = "po-creator"
	app.currentUser = &User{
		Base:     Base{ID: "po-creator"},
		Username: "po-creator",
		Role:     Role{Name: "admin", Permissions: `["*"]`},
	}

	po := createPurchaseOrderRegressionFixture(t, app, 6000)
	require.Equal(t, "Pending Approval", po.Status)

	err := app.ApprovePurchaseOrder(po.ID, "spoofed-approver")
	require.Error(t, err)
	require.Contains(t, err.Error(), "SEGREGATION_VIOLATION")

	app.currentUserID = "po-approver"
	app.currentUser = &User{
		Base:     Base{ID: "po-approver"},
		Username: "po-approver",
		RoleName: "manager",
		Role:     Role{Name: "manager", Permissions: `["po:approve"]`},
	}

	require.NoError(t, app.ApprovePurchaseOrder(po.ID, "spoofed-approver"))

	var stored PurchaseOrder
	require.NoError(t, app.db.First(&stored, "id = ?", po.ID).Error)
	require.Equal(t, "Approved", stored.Status)
	require.Equal(t, "po-approver", stored.ApprovedBy)
	require.NotEqual(t, "spoofed-approver", stored.ApprovedBy)
	require.NotNil(t, stored.ApprovedAt)
}

func TestUpdatePOStatusUsesLifecycleSpecificPermissions(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	po := createPurchaseOrderRegressionFixture(t, app, 100)
	nextPO := createPurchaseOrderRegressionFixture(t, app, 100)

	app.currentUserID = "po-sender"
	app.currentUser = &User{
		Base:     Base{ID: "po-sender"},
		Username: "po-sender",
		RoleName: "operations",
		Role:     Role{Name: "operations", Permissions: `["po:send"]`},
	}

	require.NoError(t, app.UpdatePOStatus(po.ID, "Sent"))

	err := app.UpdatePOStatus(nextPO.ID, "Acknowledged")
	require.Error(t, err)
	require.Contains(t, err.Error(), "po:update")
}

func TestUpdatePurchaseOrderBlocksLineItemChangesAfterSupplierWorkflowStarts(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	po := createPurchaseOrderRegressionFixture(t, app, 100)
	require.NoError(t, app.UpdatePOStatus(po.ID, "Sent"))

	current, err := app.GetPurchaseOrderByID(po.ID)
	require.NoError(t, err)
	require.Len(t, current.Items, 1)
	current.Items[0].Quantity = 99
	current.Items[0].UnitPriceForeign = 1
	current.Items[0].TotalForeign = 99

	_, err = app.UpdatePurchaseOrder(current)
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot modify line items")

	var storedItem PurchaseOrderItem
	require.NoError(t, app.db.First(&storedItem, "purchase_order_id = ?", po.ID).Error)
	require.Equal(t, 1.0, storedItem.Quantity)
	require.InDelta(t, 100, storedItem.UnitPriceForeign, 0.001)
}

func TestUpdatePurchaseOrderDraftCrossingApprovalThresholdMovesToPendingApproval(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	po := createPurchaseOrderRegressionFixture(t, app, 100)
	current, err := app.GetPurchaseOrderByID(po.ID)
	require.NoError(t, err)
	current.Items[0].UnitPriceForeign = 6000
	current.Items[0].TotalForeign = 6000

	updated, err := app.UpdatePurchaseOrder(current)
	require.NoError(t, err)
	require.Equal(t, "Pending Approval", updated.Status)
	require.InDelta(t, 6600, updated.TotalBHD, 0.001)
}

func TestRouteToPurchaseOrderUsesValidatedCreatePath(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	require.NoError(t, app.db.AutoMigrate(&PurchaseOrder{}, &PurchaseOrderItem{}, &SupplierMaster{}))

	supplier := SupplierMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierCode: "OCR-SUP",
		SupplierName: "OCR Supplier",
	}
	require.NoError(t, app.db.Create(&supplier).Error)

	_, err := app.routeToPurchaseOrder(map[string]interface{}{
		"po_number": "PO-OCR-NO-SUPPLIER",
		"line_items": []interface{}{
			map[string]interface{}{"description": "Valve", "quantity": 1.0, "unit_price": 100.0},
		},
	}, "po.pdf")
	require.Error(t, err)
	require.Contains(t, err.Error(), "SUPPLIER_REQUIRED")

	poID, err := app.routeToPurchaseOrder(map[string]interface{}{
		"po_number":     "PO-OCR-VALIDATED",
		"supplier_id":   supplier.ID,
		"invoice_date":  "2026-06-01",
		"delivery_date": "2026-07-01",
		"currency":      "BHD",
		"payment_terms": "Net 30",
		"exchange_rate": 1.0,
		"division":      "PH Trading",
		"line_items": []interface{}{
			map[string]interface{}{"description": "Validated OCR PO item", "quantity": 1.0, "unit_price": 6000.0},
		},
	}, "po.pdf")
	require.NoError(t, err)

	var stored PurchaseOrder
	require.NoError(t, app.db.Preload("Items").First(&stored, "id = ?", poID).Error)
	require.Equal(t, supplier.ID, stored.SupplierID)
	require.Equal(t, "Pending Approval", stored.Status)
	require.Len(t, stored.Items, 1)
	require.InDelta(t, 6600, stored.TotalBHD, 0.001)
	require.Equal(t, "test-user", stored.CreatedBy)
}

func TestCreatePOsFromOrderSplitsSelectedItemsBySupplier(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	supplierA := SupplierMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierCode: "SUP-A",
		SupplierName: "Alpha Supplier",
		PaymentTerms: "Net 30",
	}
	supplierB := SupplierMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierCode: "SUP-B",
		SupplierName: "Beta Supplier",
		PaymentTerms: "Net 45",
	}
	require.NoError(t, app.db.Create(&supplierA).Error)
	require.NoError(t, app.db.Create(&supplierB).Error)

	productA := ProductMaster{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:     "ALPHA-PROD",
		ProductName:     "Alpha analyser",
		SupplierID:      supplierA.ID,
		SupplierCode:    supplierA.SupplierCode,
		StandardCostBHD: 50,
		IsActive:        true,
	}
	productB := ProductMaster{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		ProductCode:     "BETA-PROD",
		ProductName:     "Beta transmitter",
		SupplierID:      supplierB.ID,
		SupplierCode:    supplierB.SupplierCode,
		StandardCostBHD: 80,
		IsActive:        true,
	}
	require.NoError(t, app.db.Create(&productA).Error)
	require.NoError(t, app.db.Create(&productB).Error)

	order := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:   "ORD-MULTI-SUP",
		CustomerName:  "Multi Supplier Customer",
		OrderDate:     time.Now(),
		RequiredDate:  time.Now().AddDate(0, 0, 30),
		Status:        "Confirmed",
		TotalValueBHD: 300,
		GrandTotalBHD: 300,
		Items: []OrderItem{
			{
				Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
				ProductID:   productA.ID,
				ProductCode: productA.ProductCode,
				Description: "Alpha line",
				Quantity:    1,
				UnitPrice:   100,
				TotalPrice:  100,
			},
			{
				Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
				ProductID:   productB.ID,
				ProductCode: productB.ProductCode,
				Description: "Beta line",
				Quantity:    2,
				UnitPrice:   100,
				TotalPrice:  200,
			},
		},
	}
	require.NoError(t, app.db.Create(&order).Error)

	ids := []string{order.Items[0].ID, order.Items[1].ID}
	pos, err := app.CreatePOsFromOrder(order.ID, ids)
	require.NoError(t, err)
	require.Len(t, pos, 2)

	bySupplier := map[string]PurchaseOrder{}
	for _, po := range pos {
		bySupplier[po.SupplierID] = po
	}
	require.Contains(t, bySupplier, supplierA.ID)
	require.Contains(t, bySupplier, supplierB.ID)
	require.Len(t, bySupplier[supplierA.ID].Items, 1)
	require.Equal(t, productA.ProductCode, bySupplier[supplierA.ID].Items[0].ProductCode)
	require.Equal(t, supplierB.SupplierName, bySupplier[supplierB.ID].SupplierName)
	require.Len(t, bySupplier[supplierB.ID].Items, 1)
	require.Equal(t, productB.ProductCode, bySupplier[supplierB.ID].Items[0].ProductCode)
}

func TestFixDatabaseDatesDoesNotShiftCurrentYearOrders(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	currentYear := time.Now().Year()
	currentDate := time.Date(currentYear, time.March, 31, 0, 0, 0, 0, time.UTC)
	futureDate := currentDate.AddDate(1, 0, 0)

	currentOrder := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:   "ORD-CURRENT-YEAR",
		CustomerName:  "Current Year Customer",
		OrderDate:     currentDate,
		RequiredDate:  currentDate,
		TotalValueBHD: 100,
		GrandTotalBHD: 100,
		Status:        "Confirmed",
	}
	futureOrder := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:   "ORD-FUTURE-YEAR",
		CustomerName:  "Future Year Customer",
		OrderDate:     futureDate,
		RequiredDate:  futureDate,
		TotalValueBHD: 200,
		GrandTotalBHD: 200,
		Status:        "Confirmed",
	}
	require.NoError(t, app.db.Create(&currentOrder).Error)
	require.NoError(t, app.db.Create(&futureOrder).Error)

	results, err := app.FixDatabaseDates()
	require.NoError(t, err)
	require.Equal(t, 1, results["orders"])

	var storedCurrent Order
	require.NoError(t, app.db.First(&storedCurrent, "id = ?", currentOrder.ID).Error)
	require.Equal(t, currentDate.Format("2006-01-02"), storedCurrent.OrderDate.Format("2006-01-02"))
	require.Equal(t, currentDate.Format("2006-01-02"), storedCurrent.RequiredDate.Format("2006-01-02"))

	var storedFuture Order
	require.NoError(t, app.db.First(&storedFuture, "id = ?", futureOrder.ID).Error)
	require.Equal(t, currentDate.Format("2006-01-02"), storedFuture.OrderDate.Format("2006-01-02"))
	require.Equal(t, currentDate.Format("2006-01-02"), storedFuture.RequiredDate.Format("2006-01-02"))
}

func TestAnalyzeDocumentWithButlerFallsBackWithoutExternalAI(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	oldMistralProvider := settingsBasedMistralKey
	oldAIMLProvider := settingsBasedAIMLKey
	defer func() {
		settingsBasedMistralKey = oldMistralProvider
		settingsBasedAIMLKey = oldAIMLProvider
	}()

	SetMistralKeyProvider(func() string { return "" })
	SetAIMLKeyProvider(func() string { return "" })
	t.Setenv("MISTRAL_API_KEY", "")
	t.Setenv("AIML_API_KEY", "")
	t.Setenv("ASYMM_AIML_API_KEY", "")

	text := `From: Ministry of Works
Subject: MOW 2026 143 Supply of sensor hydrostatic level measurement
Delivery Date: 04/05/2026
RFQ: MOW-2026-143
Quantity: 2 pcs`

	insight, err := app.AnalyzeDocumentWithButler(text, "rfq", map[string]interface{}{
		"subject": "MOW 2026 143 Supply of sensor hydrostatic level measurement",
	})
	require.NoError(t, err)
	require.Greater(t, insight.Confidence, 0.0)
	require.NotEmpty(t, insight.Summary)
	require.Equal(t, "rfq", insight.DocumentType)
	require.Contains(t, insight.ExtractedMetadata, "fallback_reason")
}

func TestUpdateSupplierInvoiceWithPayment_CreatesSingleLedgerEntry(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	supplierID := seedTestSupplier(t, app.db, "Endress+Hauser")
	paymentDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	invoice := SupplierInvoice{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierID:      supplierID,
		SupplierName:    "Endress+Hauser",
		InvoiceNumber:   "SUPP-2026-001",
		InvoiceDate:     paymentDate,
		DueDate:         paymentDate.AddDate(0, 0, 30),
		Currency:        "BHD",
		ExchangeRate:    1,
		SubtotalForeign: 100,
		SubtotalBHD:     100,
		VATForeign:      10,
		VATBHD:          10,
		TotalForeign:    110,
		TotalBHD:        110,
		Status:          "Approved",
		PaymentStatus:   "Unpaid",
		MatchStatus:     "Pending",
	}
	require.NoError(t, app.db.Create(&invoice).Error)

	invoice.PaymentStatus = "Paid"
	invoice.PaymentMethod = "Bank Transfer"
	invoice.PaymentRef = "TXN-1001"
	invoice.PaymentDate = &paymentDate

	updated, err := app.UpdateSupplierInvoiceWithPayment(invoice, true)
	require.NoError(t, err)
	require.Equal(t, "Paid", updated.PaymentStatus)
	require.Equal(t, "Paid", updated.Status)

	var payments []SupplierPayment
	require.NoError(t, app.db.Where("supplier_invoice_id = ?", invoice.ID).Find(&payments).Error)
	require.Len(t, payments, 1)
	require.Equal(t, 110.0, payments[0].AmountBHD)
	require.Equal(t, "TXN-1001", payments[0].Reference)

	_, err = app.UpdateSupplierInvoiceWithPayment(updated, true)
	require.NoError(t, err)
	require.NoError(t, app.db.Where("supplier_invoice_id = ?", invoice.ID).Find(&payments).Error)
	require.Len(t, payments, 1)
}

func TestCreateSupplierInvoice_NormalizesCurrencyDivisionAndCreatorFromPO(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	require.NoError(t, app.db.AutoMigrate(&PurchaseOrder{}, &PurchaseOrderItem{}))

	supplierID := seedTestSupplier(t, app.db, "Endress+Hauser")
	po := PurchaseOrder{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PONumber:     "PO-EUR-RESTORE",
		SupplierID:   supplierID,
		SupplierName: "Endress+Hauser",
		PODate:       time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
		Currency:     "EUR",
		ExchangeRate: defaultEURToBHDRate,
		Division:     "AHS Trading",
		Status:       "Draft",
	}
	require.NoError(t, app.db.Create(&po).Error)

	created, err := app.CreateSupplierInvoice(SupplierInvoice{
		SupplierID:      supplierID,
		PurchaseOrderID: po.ID,
		InvoiceNumber:   "SUPP-EUR-RESTORE",
		InvoiceDate:     po.PODate,
		DueDate:         po.PODate.AddDate(0, 0, 30),
		Currency:        "eur",
		ExchangeRate:    1,
		SubtotalForeign: 100,
		VATForeign:      10,
		TotalForeign:    110,
	})
	require.NoError(t, err)
	require.Equal(t, "EUR", created.Currency)
	require.Equal(t, defaultEURToBHDRate, created.ExchangeRate)
	require.InDelta(t, 49.5, created.TotalBHD, 0.001)
	require.Equal(t, "AHS Trading", created.Division)
	require.Equal(t, "test-user", created.CreatedBy)
	require.Equal(t, po.PONumber, created.PONumber)
}

func TestCreateSupplierInvoiceFromOCR_LooksUpPONormalizesCurrencyAndCreator(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	require.NoError(t, app.db.AutoMigrate(&PurchaseOrder{}, &PurchaseOrderItem{}, &OCRDocument{}))

	supplierID := seedTestSupplier(t, app.db, "OCR Supplier")
	po := PurchaseOrder{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		PONumber:     "PO-OCR-RESTORE",
		SupplierID:   supplierID,
		SupplierName: "OCR Supplier",
		PODate:       time.Date(2026, 4, 2, 0, 0, 0, 0, time.UTC),
		Currency:     "USD",
		ExchangeRate: defaultExchangeRateToBHD("USD"),
		Division:     "AHS Trading",
		Status:       "Draft",
	}
	require.NoError(t, app.db.Create(&po).Error)

	payload, err := json.Marshal(map[string]interface{}{
		"invoice_number": "OCR-SUPP-RESTORE",
		"invoice_date":   "2026-04-02",
		"due_date":       "2026-05-02",
		"subtotal":       200.0,
		"vat":            20.0,
		"total":          220.0,
		"currency":       "usd",
		"po_number":      po.PONumber,
	})
	require.NoError(t, err)
	ocrDoc := OCRDocument{
		FileName:          "supplier-invoice.pdf",
		FilePath:          "/tmp/supplier-invoice.pdf",
		DocumentType:      "supplier_invoice",
		ExtractedDataJSON: string(payload),
		Confidence:        0.88,
		CreatedAt:         time.Now(),
	}
	require.NoError(t, app.db.Create(&ocrDoc).Error)

	created, err := app.CreateSupplierInvoiceFromOCR("1", "", "")
	require.NoError(t, err)
	require.Equal(t, supplierID, created.SupplierID)
	require.Equal(t, po.ID, created.PurchaseOrderID)
	require.Equal(t, po.PONumber, created.PONumber)
	require.Equal(t, "USD", created.Currency)
	require.Equal(t, defaultExchangeRateToBHD("USD"), created.ExchangeRate)
	require.InDelta(t, 82.72, created.TotalBHD, 0.001)
	require.Equal(t, "AHS Trading", created.Division)
	require.Equal(t, "test-user", created.CreatedBy)
}

func TestCustomerAndSupplierPaymentsCarryResolvedDivision(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerID := seedTestCustomer(t, app.db, "BAPCO")
	invoiceDate := time.Now().AddDate(0, 0, -10)
	customerInvoice := Invoice{
		Base:           Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		InvoiceNumber:  "INV-DIV-RESTORE",
		CustomerID:     customerID,
		CustomerName:   "BAPCO",
		InvoiceDate:    invoiceDate,
		DueDate:        invoiceDate.AddDate(0, 0, 30),
		Status:         "Sent",
		GrandTotalBHD:  100,
		OutstandingBHD: 100,
		Division:       "AHS Trading",
	}
	require.NoError(t, app.db.Create(&customerInvoice).Error)

	customerPayment, err := app.RecordPayment(customerInvoice.ID, 25, "Cash", invoiceDate.AddDate(0, 0, 1).Format("2006-01-02"), "CASH-DIV-1")
	require.NoError(t, err)
	require.Equal(t, "AHS Trading", customerPayment.Division)

	supplierID := seedTestSupplier(t, app.db, "Supplier Division")
	supplierInvoice := SupplierInvoice{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now(), CreatedBy: "creator-user"},
		SupplierID:      supplierID,
		SupplierName:    "Supplier Division",
		InvoiceNumber:   "SI-DIV-RESTORE",
		InvoiceDate:     invoiceDate,
		DueDate:         invoiceDate.AddDate(0, 0, 30),
		Currency:        "BHD",
		ExchangeRate:    1,
		SubtotalForeign: 100,
		SubtotalBHD:     100,
		TotalForeign:    100,
		TotalBHD:        100,
		Status:          "Approved",
		PaymentStatus:   "Unpaid",
		MatchStatus:     "Matched",
		Division:        "AHS Trading",
	}
	require.NoError(t, app.db.Create(&supplierInvoice).Error)

	supplierPayment, err := app.RecordSupplierPayment(supplierInvoice.ID, 40, "BHD", "Bank Transfer", invoiceDate.AddDate(0, 0, 2).Format("2006-01-02"), "SUP-DIV-1")
	require.NoError(t, err)
	require.Equal(t, "AHS Trading", supplierPayment.Division)
}

func TestGetBusinessYearSummary_CapturesCommercialCoverage(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerID := seedTestCustomer(t, app.db, "BAPCO")
	baseTime := time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC)

	invoice := Invoice{
		Base:           Base{ID: uuid.New().String(), CreatedAt: baseTime, UpdatedAt: baseTime},
		InvoiceNumber:  "INV-2026-001",
		CustomerID:     customerID,
		CustomerName:   "BAPCO",
		InvoiceDate:    baseTime,
		Status:         "Sent",
		GrandTotalBHD:  505.120,
		OutstandingBHD: 505.120,
	}
	require.NoError(t, app.db.Create(&invoice).Error)

	order := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: baseTime, UpdatedAt: baseTime},
		OrderNumber:   "ORD-2026-001",
		CustomerID:    customerID,
		CustomerName:  "BAPCO",
		OrderDate:     baseTime,
		Status:        "Confirmed",
		GrandTotalBHD: 111668.459,
	}
	require.NoError(t, app.db.Create(&order).Error)

	offer := Offer{
		Base:          Base{ID: uuid.New().String(), CreatedAt: baseTime, UpdatedAt: baseTime},
		OfferNumber:   "OFF-2026-001",
		CustomerID:    customerID,
		CustomerName:  "BAPCO",
		QuotationDate: baseTime,
		ValidityDate:  baseTime.AddDate(0, 0, 30),
		TotalValueBHD: 396609.242,
		Stage:         "Quoted",
	}
	require.NoError(t, app.db.Create(&offer).Error)

	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: baseTime, UpdatedAt: baseTime},
		FolderNumber: "2026-001",
		CustomerID:   customerID,
		CustomerName: "BAPCO",
		Title:        "2026 coverage check",
		Stage:        "Qualified",
		RevenueBHD:   1711321.860,
		Year:         2026,
		OfferDate:    baseTime,
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	summary := app.getBusinessYearSummary("do you have access to 2026 data from this?")
	require.Equal(t, "available", summary["status"])
	require.Equal(t, 2026, summary["year"])

	invoices := summary["invoices"].(map[string]interface{})
	require.Equal(t, 1, invoices["count"])
	require.Equal(t, 505.12, invoices["total_bhd"])
	require.Equal(t, "2026-03-31", invoices["latest_date"])

	orders := summary["orders"].(map[string]interface{})
	require.Equal(t, 1, orders["count"])
	require.Equal(t, 111668.459, orders["total_bhd"])

	offers := summary["offers"].(map[string]interface{})
	require.Equal(t, 1, offers["count"])
	require.Equal(t, 396609.242, offers["total_bhd"])

	opps := summary["opportunities"].(map[string]interface{})
	require.Equal(t, 1, opps["count"])
	require.Equal(t, 1, opps["open_count"])
	require.Equal(t, 1711321.86, opps["total_value_bhd"])
	require.Equal(t, 1711321.86, opps["open_pipeline_bhd"])
	require.Equal(t, "2026-03-31", opps["latest_activity_date"])

	require.ElementsMatch(t, []string{"invoices", "orders", "offers", "opportunities"}, summary["available_data_types"].([]string))
}

func TestBuildFullContext_IncludesBusinessYearSummaryForYearQueries(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber: "2026-PIPE-01",
		CustomerName: "BAPCO",
		Title:        "Pipeline visibility",
		Stage:        "Qualified",
		RevenueBHD:   1000,
		Year:         2026,
		OfferDate:    time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	ctx := app.buildFullContext(Intent{RawQuery: "do you have access to 2026 data?", Domain: "general"})
	yearSummary, ok := ctx["business_year_summary"].(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, 2026, yearSummary["year"])

	opps := yearSummary["opportunities"].(map[string]interface{})
	require.Equal(t, 1, opps["count"])
	require.Equal(t, 1000.0, opps["total_value_bhd"])
}

func TestManualMatchLine_SupportsSupplierPaymentMatches(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-2026-001",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -7),
		PeriodEnd:       time.Now(),
		Status:          "Imported",
	}
	require.NoError(t, app.db.Create(&statement).Error)

	line := BankStatementLine{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankStatementID: statement.ID,
		LineNumber:      1,
		TransactionDate: time.Now(),
		ValueDate:       time.Now(),
		Description:     "Supplier transfer",
		Debit:           110,
		Credit:          0,
		Balance:         890,
	}
	require.NoError(t, app.db.Create(&line).Error)

	payment := SupplierPayment{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierInvoiceID: "supp-inv-1",
		SupplierID:        "sup-1",
		SupplierName:      "Endress+Hauser",
		InvoiceNumber:     "SUPP-2026-001",
		AmountForeign:     110,
		Currency:          "BHD",
		ExchangeRate:      1,
		AmountBHD:         110,
		PaymentDate:       time.Now(),
		PaymentMethod:     "Bank Transfer",
		Reference:         "TXN-1001",
	}
	require.NoError(t, app.db.Create(&payment).Error)

	require.NoError(t, app.ManualMatchLine(line.ID, "SUPPLIER_PAYMENT", payment.ID, "admin"))

	var updatedLine BankStatementLine
	require.NoError(t, app.db.First(&updatedLine, "id = ?", line.ID).Error)
	require.True(t, updatedLine.IsMatched)
	require.Equal(t, "Manual", updatedLine.MatchType)
	require.Equal(t, payment.ID, updatedLine.MatchedPaymentID)

	var auditCount int64
	require.NoError(t, app.db.Model(&BankReconciliationAuditLog{}).Where("bank_statement_id = ?", statement.ID).Count(&auditCount).Error)
	require.Equal(t, int64(1), auditCount)
}

func TestRepairImportedCommercialDocuments_RemovesSyntheticRowsAndRenamesImportedOrders(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	offer := Offer{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:     "EH-64-26-R0",
		CustomerName:    "Awal Dairy Company W.L.L.",
		QuotationDate:   time.Now(),
		ValidityDate:    time.Now().AddDate(0, 0, 30),
		Stage:           "Quoted",
		PaymentTerms:    "Net 30",
		DeliveryTerms:   "DAP Bahrain",
		VatRate:         10,
		TotalValueBHD:   858,
		EstimatedMargin: 20,
	}
	require.NoError(t, app.db.Create(&offer).Error)

	require.NoError(t, app.db.Create(&[]OfferItem{
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OfferID:     offer.ID,
			LineNumber:  1,
			ProductCode: "TM411",
			Model:       "TM411-4EVR5/101",
			Description: "iTHERM ModuLine TM411",
			Quantity:    2,
			UnitPrice:   390,
			TotalPrice:  780,
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OfferID:     offer.ID,
			LineNumber:  2,
			ProductCode: "TOTAL",
			Description: "Total for Order",
			Quantity:    1,
			UnitPrice:   780,
			TotalPrice:  780,
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OfferID:     offer.ID,
			LineNumber:  3,
			Description: "Line Item 3 -",
			Equipment:   "Line Item 3",
			Quantity:    1,
			UnitPrice:   1,
			TotalPrice:  1,
		},
	}).Error)

	order := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:   "IMP-EH-64-26",
		OfferID:       offer.ID,
		OfferNumber:   offer.OfferNumber,
		CustomerName:  offer.CustomerName,
		Status:        "Confirmed",
		OrderDate:     time.Now(),
		TotalValueBHD: 1560,
		GrandTotalBHD: 1560,
	}
	require.NoError(t, app.db.Create(&order).Error)

	require.NoError(t, app.db.Create(&[]OrderItem{
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OrderID:     order.ID,
			LineNumber:  1,
			ProductCode: "TM411",
			Model:       "TM411-4EVR5/101",
			Description: "iTHERM ModuLine TM411",
			Quantity:    2,
			UnitPrice:   390,
			TotalPrice:  780,
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OrderID:     order.ID,
			LineNumber:  2,
			ProductCode: "TOTAL",
			Description: "Total for Order",
			Quantity:    1,
			UnitPrice:   780,
			TotalPrice:  780,
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OrderID:     order.ID,
			LineNumber:  3,
			Description: "Line Item 3 -",
			Equipment:   "Line Item 3",
			Quantity:    1,
			UnitPrice:   1,
			TotalPrice:  1,
		},
	}).Error)

	opportunity := Opportunity{
		Base:           Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber:   "2026-64",
		CustomerName:   offer.CustomerName,
		Year:           2026,
		Source:         "2026_onedrive",
		ProductDetails: `[{"description":"iTHERM ModuLine TM411","quantity":2,"unit_price":390,"total_price":780,"part_number":"TM411-4EVR5/101"},{"description":"Line Item 2 -","quantity":1,"unit_price":1,"total_price":1}]`,
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	require.NoError(t, repairImportedCommercialDocuments(app.db))

	var repairedOffer Offer
	require.NoError(t, app.db.Preload("Items").First(&repairedOffer, "id = ?", offer.ID).Error)
	require.Len(t, repairedOffer.Items, 1)
	require.Equal(t, 1, repairedOffer.Items[0].LineNumber)
	require.Equal(t, 858.0, repairedOffer.TotalValueBHD)

	var repairedOrder Order
	require.NoError(t, app.db.Preload("Items").First(&repairedOrder, "id = ?", order.ID).Error)
	require.Equal(t, offer.OfferNumber, repairedOrder.OrderNumber)
	require.Len(t, repairedOrder.Items, 1)
	require.Equal(t, 780.0, repairedOrder.TotalValueBHD)
	require.Equal(t, 780.0, repairedOrder.GrandTotalBHD)

	var repairedOpportunity Opportunity
	require.NoError(t, app.db.First(&repairedOpportunity, "id = ?", opportunity.ID).Error)
	require.Contains(t, repairedOpportunity.ProductDetails, "iTHERM ModuLine TM411")
	require.NotContains(t, repairedOpportunity.ProductDetails, "Line Item")
}

func TestRepairImportedCommercialDocuments_BackfillsLinkedWonOfferShellAndOrder(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	offer := Offer{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:   "OTH-4-26",
		CustomerName:  "The Bahrain Petroleum Co. (Bapco)",
		QuotationDate: time.Now(),
		ValidityDate:  time.Now().AddDate(0, 0, 30),
		Stage:         "Won",
		VatRate:       10,
		TotalValueBHD: 2948,
	}
	require.NoError(t, app.db.Create(&offer).Error)

	order := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:   "OTH-4-26",
		OfferID:       offer.ID,
		OfferNumber:   offer.OfferNumber,
		CustomerName:  offer.CustomerName,
		Status:        "Confirmed",
		OrderDate:     time.Now(),
		TotalValueBHD: 2680,
		GrandTotalBHD: 2680,
	}
	require.NoError(t, app.db.Create(&order).Error)

	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		FolderNumber: "2026-304",
		OfferID:      offer.ID,
		CustomerName: offer.CustomerName,
		Title:        "Bapco order shell",
		Stage:        "Quoted",
		RevenueBHD:   2948,
		Source:       "2026_onedrive",
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	require.NoError(t, repairImportedCommercialDocuments(app.db))

	var repairedOffer Offer
	require.NoError(t, app.db.Preload("Items").First(&repairedOffer, "id = ?", offer.ID).Error)
	require.Len(t, repairedOffer.Items, 1)
	require.Equal(t, "Per Offer OTH-4-26", repairedOffer.Items[0].Description)
	require.Equal(t, 2680.0, repairedOffer.Items[0].TotalPrice)
	require.Equal(t, 2948.0, repairedOffer.TotalValueBHD)

	var repairedOrder Order
	require.NoError(t, app.db.Preload("Items").First(&repairedOrder, "id = ?", order.ID).Error)
	require.Len(t, repairedOrder.Items, 1)
	require.Equal(t, 2680.0, repairedOrder.Items[0].TotalPrice)
	require.Equal(t, 2680.0, repairedOrder.TotalValueBHD)
	require.Equal(t, 2680.0, repairedOrder.GrandTotalBHD)

	var repairedOpportunity Opportunity
	require.NoError(t, app.db.First(&repairedOpportunity, "id = ?", opportunity.ID).Error)
	require.Equal(t, "Won", repairedOpportunity.Stage)
	require.NotNil(t, repairedOpportunity.ClosedDate)
}

func TestUpdateOfferFull_PersistsMetadataAndClearsHeaderFields(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerID := seedTestCustomer(t, app.db, "Awal Dairy Company W.L.L.")

	offer := Offer{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:       "43-26",
		RevisionNumber:    1,
		CustomerID:        customerID,
		CustomerName:      "Awal Dairy Company W.L.L.",
		QuotationDate:     time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC),
		ValidityDate:      time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		Stage:             "Quoted",
		PaymentTerms:      "Net 30",
		DeliveryTerms:     "DAP Bahrain",
		DeliveryWeeks:     "4-6 weeks",
		CountryOfOrigin:   "DE",
		IssuedBy:          "Periyasamy N",
		ContactPhone:      "34128132",
		CustomerReference: "EH-64-26-R0",
		AttentionPerson:   "Mr. Mohammed",
		AttentionCompany:  "AWAL DAIRY COMPANY W.L.L.",
		AttentionPhone:    "37779872",
		AttentionAddress:  "Manama, Bahrain",
		VatRate:           10,
		TotalValueBHD:     844.8,
	}
	require.NoError(t, app.db.Create(&offer).Error)
	require.NoError(t, app.db.Create(&OfferItem{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:     offer.ID,
		LineNumber:  1,
		ProductCode: "TM411",
		Description: "Old item",
		Quantity:    1,
		UnitPrice:   844.8,
		TotalPrice:  844.8,
	}).Error)

	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:      offer.ID,
		FolderNumber: "1-26",
		CustomerID:   customerID,
		CustomerName: offer.CustomerName,
		Title:        "Legacy title",
		Stage:        "Quoted",
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	validityDate := time.Now().AddDate(0, 0, 30).Format("2006-01-02")
	updated, err := app.UpdateOfferFull(offer.ID, OfferUpdateData{
		OfferNumber:       "64-26",
		CustomerName:      offer.CustomerName,
		ProjectName:       "Supply of E+H TEMP. TRANSMITTER",
		FolderNumber:      "64-26",
		QuotationDate:     "2026-03-30",
		ValidityDate:      validityDate,
		PaymentTerms:      "",
		DeliveryTerms:     "",
		DeliveryWeeks:     "",
		CountryOfOrigin:   "",
		IssuedBy:          "",
		ContactPhone:      "",
		CustomerReference: "",
		AttentionPerson:   "",
		AttentionCompany:  "",
		AttentionPhone:    "",
		AttentionAddress:  "",
		VatRate:           0,
		Items: []OfferUpdateItem{
			{
				Description: "iTHERM ModuLine TM411",
				ProductCode: "TM411",
				Model:       "TM411-4EVR5/101",
				Quantity:    2,
				UnitPrice:   384,
				TotalPrice:  768,
			},
			{
				Description: "Total for Order",
				ProductCode: "TOTAL",
				Quantity:    1,
				UnitPrice:   768,
				TotalPrice:  768,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 0.0, updated.VatRate)
	require.Equal(t, 768.0, updated.TotalValueBHD)

	var stored Offer
	require.NoError(t, app.db.Preload("Items").First(&stored, "id = ?", offer.ID).Error)
	require.Equal(t, "64-26", stored.OfferNumber)
	require.Equal(t, "", stored.PaymentTerms)
	require.Equal(t, "", stored.DeliveryTerms)
	require.Equal(t, "", stored.DeliveryWeeks)
	require.Equal(t, "", stored.CountryOfOrigin)
	require.Equal(t, "", stored.IssuedBy)
	require.Equal(t, "", stored.ContactPhone)
	require.Equal(t, "", stored.CustomerReference)
	require.Equal(t, "", stored.AttentionPerson)
	require.Equal(t, "", stored.AttentionCompany)
	require.Equal(t, "", stored.AttentionPhone)
	require.Equal(t, "", stored.AttentionAddress)
	require.Equal(t, 0.0, stored.VatRate)
	require.Len(t, stored.Items, 1)
	require.Equal(t, 2, stored.RevisionNumber)

	var linked Opportunity
	require.NoError(t, app.db.First(&linked, "id = ?", opportunity.ID).Error)
	require.Equal(t, "64-26", linked.FolderNumber)
	require.Equal(t, "Supply of E+H TEMP. TRANSMITTER", linked.Title)
	require.Equal(t, offer.CustomerName, linked.CustomerName)
}

func TestUpdateOfferFull_HeaderOnlySavePreservesExistingItems(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	validityDate := time.Now().AddDate(0, 0, 30)
	offer := Offer{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:     "43-26-R1",
		RevisionNumber:  1,
		CustomerName:    "Awal Dairy Company W.L.L.",
		QuotationDate:   time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC),
		ValidityDate:    validityDate,
		Stage:           "Quoted",
		PaymentTerms:    "Net 30",
		DeliveryTerms:   "DAP Bahrain",
		TotalValueBHD:   844.8,
		AttentionPerson: "Existing contact",
	}
	require.NoError(t, app.db.Create(&offer).Error)
	require.NoError(t, app.db.Create(&[]OfferItem{
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OfferID:     offer.ID,
			LineNumber:  1,
			ProductCode: "TM411",
			Description: "Temperature transmitter",
			Quantity:    1,
			UnitPrice:   420,
			TotalPrice:  420,
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			OfferID:     offer.ID,
			LineNumber:  2,
			ProductCode: "TM131",
			Description: "Thermowell",
			Quantity:    2,
			UnitPrice:   212.4,
			TotalPrice:  424.8,
		},
	}).Error)

	updated, err := app.UpdateOfferFull(offer.ID, OfferUpdateData{
		CustomerName:    offer.CustomerName,
		QuotationDate:   "2026-03-30",
		ValidityDate:    validityDate.Format("2006-01-02"),
		PaymentTerms:    "Net 45",
		DeliveryTerms:   "CIF Bahrain",
		AttentionPerson: "Updated contact",
		Items:           []OfferUpdateItem{},
	})
	require.NoError(t, err)
	require.Equal(t, 844.8, updated.TotalValueBHD)

	var stored Offer
	require.NoError(t, app.db.Preload("Items").First(&stored, "id = ?", offer.ID).Error)
	require.Equal(t, "Net 45", stored.PaymentTerms)
	require.Equal(t, "CIF Bahrain", stored.DeliveryTerms)
	require.Equal(t, "Updated contact", stored.AttentionPerson)
	require.Len(t, stored.Items, 2)
	require.Equal(t, 420.0, stored.Items[0].TotalPrice)
	require.Equal(t, 424.8, stored.Items[1].TotalPrice)
}

func TestUpdateOfferFullRejectsTerminalOfferEdits(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	offer := Offer{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:   "TERM-26",
		CustomerName:  "Locked Customer",
		QuotationDate: time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC),
		ValidityDate:  time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		Stage:         "Won",
		TotalValueBHD: 100,
	}
	require.NoError(t, app.db.Create(&offer).Error)

	_, err := app.UpdateOfferFull(offer.ID, OfferUpdateData{
		CustomerName:  "Mutated Customer",
		QuotationDate: "2026-03-30",
		ValidityDate:  "2026-04-29",
		Items:         []OfferUpdateItem{},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot edit terminal offer")

	var stored Offer
	require.NoError(t, app.db.First(&stored, "id = ?", offer.ID).Error)
	require.Equal(t, "Locked Customer", stored.CustomerName)
	require.Equal(t, "Won", stored.Stage)
}

func TestCreateOfferRevisionPreservesSourceAndRelinksOpportunity(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerID := seedTestCustomer(t, app.db, "Revision Customer WLL")
	source := Offer{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:       "REV-26",
		RevisionNumber:    0,
		CustomerID:        customerID,
		CustomerName:      "Revision Customer WLL",
		QuotationDate:     time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC),
		ValidityDate:      time.Date(2026, 4, 29, 0, 0, 0, 0, time.UTC),
		Stage:             "Lost",
		LostReason:        "Price",
		PaymentTerms:      "Net 30",
		DeliveryTerms:     "DAP Bahrain",
		CustomerReference: "EH-REV-26",
		TotalValueBHD:     500,
		VatRate:           10,
	}
	require.NoError(t, app.db.Create(&source).Error)
	require.NoError(t, app.db.Create(&OfferItem{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:     source.ID,
		LineNumber:  1,
		ProductCode: "REV-PROD",
		Description: "Revision line",
		Quantity:    2,
		UnitPrice:   250,
		TotalPrice:  500,
	}).Error)
	opportunity := Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:      source.ID,
		CustomerID:   customerID,
		CustomerName: source.CustomerName,
		FolderNumber: "REV-26",
		Stage:        "Lost",
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	revision, err := app.CreateOfferRevision(source.ID)
	require.NoError(t, err)
	require.NotEqual(t, source.ID, revision.ID)
	require.Equal(t, "REV-26-R1", revision.OfferNumber)
	require.Equal(t, 1, revision.RevisionNumber)
	require.Equal(t, source.ID, revision.RevisionOfOfferID)
	require.Equal(t, source.ID, revision.RevisionRootOfferID)
	require.Equal(t, "Quoted", revision.Stage)
	require.Empty(t, revision.LostReason)
	require.Len(t, revision.Items, 1)
	require.Equal(t, "REV-PROD", revision.Items[0].ProductCode)

	var storedSource Offer
	require.NoError(t, app.db.First(&storedSource, "id = ?", source.ID).Error)
	require.Equal(t, "Lost", storedSource.Stage)
	require.Equal(t, revision.ID, storedSource.SupersededByOfferID)
	require.NotNil(t, storedSource.SupersededAt)

	var linked Opportunity
	require.NoError(t, app.db.First(&linked, "id = ?", opportunity.ID).Error)
	require.Equal(t, revision.ID, linked.OfferID)
	require.Equal(t, "Quoted", linked.Stage)
}

func TestAddOfferFollowUpCreatesAssignedTaskAndNotification(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)
	current := seedCurrentAdminContext(t, app, "Rahul")

	customerID := seedTestCustomer(t, app.db, "Follow Up Customer WLL")
	offer := Offer{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:   "FU-26",
		CustomerID:    customerID,
		CustomerName:  "Follow Up Customer WLL",
		QuotationDate: time.Now(),
		ValidityDate:  time.Now().AddDate(0, 0, 30),
		Stage:         "Quoted",
		TotalValueBHD: 100,
	}
	require.NoError(t, app.db.Create(&offer).Error)

	followUp, err := app.AddOfferFollowUp(offer.ID, time.Now().AddDate(0, 0, 3).Format("2006-01-02"), "Call procurement")
	require.NoError(t, err)
	require.NotEmpty(t, followUp.TaskID)

	var task TaskItem
	require.NoError(t, app.db.First(&task, "id = ?", followUp.TaskID).Error)
	require.Equal(t, "offer_follow_up", task.TaskType)
	require.NotNil(t, task.AssigneeEmployeeID)
	require.Equal(t, current.EmployeeID, *task.AssigneeEmployeeID)
	require.NotNil(t, task.LegacyFollowUpID)
	require.Equal(t, followUp.ID, *task.LegacyFollowUpID)

	var notificationCount int64
	require.NoError(t, app.db.Model(&Notification{}).
		Where("employee_id = ? AND source_id = ? AND status = ?", current.EmployeeID, task.ID, "unread").
		Count(&notificationCount).Error)
	require.EqualValues(t, 1, notificationCount)

	require.NoError(t, app.CompleteOfferFollowUp(followUp.ID))
	require.NoError(t, app.db.First(&task, "id = ?", followUp.TaskID).Error)
	require.Equal(t, "completed", task.Status)
	var completed OfferFollowUp
	require.NoError(t, app.db.First(&completed, "id = ?", followUp.ID).Error)
	require.Equal(t, "completed", completed.Status)
	require.Equal(t, current.EmployeeName, completed.CompletedBy)
}

func TestUpdateOrder_SkipsSyntheticSummaryRowsAndRecomputesTotals(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerID := seedTestCustomer(t, app.db, "BAPCO")
	order := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:   "ORD-2026-001",
		CustomerID:    customerID,
		CustomerName:  "BAPCO",
		Status:        "Confirmed",
		OrderDate:     time.Now(),
		TotalValueBHD: 100,
		GrandTotalBHD: 100,
	}
	require.NoError(t, app.db.Create(&order).Error)

	updated, err := app.UpdateOrder(order.ID, Order{
		OrderNumber:  order.OrderNumber,
		CustomerName: order.CustomerName,
		Status:       order.Status,
		OrderDate:    order.OrderDate,
		Items: []OrderItem{
			{
				Description: "Pressure transmitter",
				ProductCode: "PT-100",
				Model:       "PT-100",
				Quantity:    2,
				UnitPrice:   390,
				TotalPrice:  780,
			},
			{
				Description: "Total for Order",
				ProductCode: "TOTAL",
				Quantity:    1,
				UnitPrice:   780,
				TotalPrice:  780,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 780.0, updated.TotalValueBHD)
	require.Equal(t, 780.0, updated.GrandTotalBHD)

	var stored Order
	require.NoError(t, app.db.Preload("Items").First(&stored, "id = ?", order.ID).Error)
	require.Len(t, stored.Items, 1)
	require.Equal(t, 1, stored.Items[0].LineNumber)
	require.Equal(t, 780.0, stored.Items[0].TotalPrice)
}

func TestUpdateSupplier_PreservesExistingIdentityFieldsOnPartialPayload(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	supplier := SupplierMaster{
		Base:           Base{CreatedBy: "tester", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierCode:   "SUP-EH001",
		SupplierName:   "ISKRAEMECO MIDDLE EAST FZCO",
		PrimaryContact: "Legacy Contact",
		Email:          "legacy@example.com",
		Country:        "UAE",
		PaymentTerms:   "Net 30",
		LeadTimeDays:   14,
		Rating:         3,
	}
	require.NoError(t, app.db.Create(&supplier).Error)

	updated, err := app.UpdateSupplier(SupplierMaster{
		Base:           Base{ID: supplier.ID},
		SupplierCode:   "",
		SupplierName:   "",
		PrimaryContact: "Updated Contact",
		Email:          "updated@example.com",
		Country:        "International",
		PaymentTerms:   "Net 60",
		LeadTimeDays:   21,
		Rating:         4,
	})
	require.NoError(t, err)
	require.Equal(t, "SUP-EH001", updated.SupplierCode)
	require.Equal(t, "ISKRAEMECO MIDDLE EAST FZCO", updated.SupplierName)
	require.Equal(t, "Updated Contact", updated.PrimaryContact)
	require.Equal(t, "updated@example.com", updated.Email)

	var stored SupplierMaster
	require.NoError(t, app.db.First(&stored, "id = ?", supplier.ID).Error)
	require.Equal(t, "SUP-EH001", stored.SupplierCode)
	require.Equal(t, "ISKRAEMECO MIDDLE EAST FZCO", stored.SupplierName)
	require.Equal(t, "Updated Contact", stored.PrimaryContact)
	require.Equal(t, "Net 60", stored.PaymentTerms)
}

func TestCreateCostingSheet_PersistsCommercialPayloadFields(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	rfq := RFQData{
		RFQNumber: "RFQ-2026-901",
		Client:    "Awal Dairy Company W.L.L.",
		Project:   "Temperature transmitter package",
		Value:     1250,
		Status:    "New",
		Stage:     "RFQ Received",
	}
	require.NoError(t, app.db.Create(&rfq).Error)

	payload := persistedCostingPayload{
		CustomerName:  "Awal Dairy Company W.L.L.",
		Subject:       "Subject: Supply of E+H temperature transmitter package",
		Body:          "We thank you for the opportunity and submit our techno-commercial offer.",
		QuoteType:     "Quotation",
		PreparedBy:    "Peri",
		PaymentTerms:  "Net 30",
		DeliveryTerms: "DAP Bahrain",
		ProjectName:   "Supply of E+H TEMP. TRANSMITTER",
		LineItems: []CostingExportLineItem{
			{
				SlNo:           1,
				Equipment:      "Temperature transmitter",
				Model:          "TM411",
				Quantity:       2,
				FOB:            390,
				FreightPercent: 7.5,
				ExchangeRate:   0.377,
				MarkupPercent:  18,
				TotalCost:      314.145,
				SuggestedPrice: 370.6911,
				TotalPrice:     741.3822,
			},
		},
	}
	itemsJSON, err := json.Marshal(payload)
	require.NoError(t, err)

	costing, err := app.CreateCostingSheet(rfq.ID, string(itemsJSON), "Peri")
	require.NoError(t, err)

	stored, err := app.GetCostingSheet(costing.ID)
	require.NoError(t, err)

	decoded, err := parsePersistedCosting(stored.Items)
	require.NoError(t, err)
	require.Equal(t, payload.Subject, decoded.Subject)
	require.Equal(t, payload.Body, decoded.Body)
	require.Equal(t, payload.PaymentTerms, decoded.PaymentTerms)
	require.Equal(t, payload.DeliveryTerms, decoded.DeliveryTerms)
	require.Len(t, decoded.LineItems, 1)
	require.InDelta(t, 7.5, decoded.LineItems[0].FreightPercent, 0.0001)
	require.InDelta(t, 0.377, decoded.LineItems[0].ExchangeRate, 0.0001)
	require.InDelta(t, 18, decoded.LineItems[0].MarkupPercent, 0.0001)
}

func TestSaveCostingAsOffer_UsesCostingOfferNumberAndPreparedBy(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	seedTestCustomer(t, app.db, "Awal Dairy Company W.L.L.")

	offer, err := app.SaveCostingAsOffer(CostingExportData{
		Date:          time.Now().Format("2006-01-02"),
		PreparedBy:    "Periyasamy N",
		CustomerName:  "Awal Dairy Company W.L.L.",
		RfqReference:  "EH-64-26-R0",
		OfferNumber:   "64-26",
		FolderNumber:  "64-26",
		QuoteType:     "Quotation",
		VatRate:       10,
		PaymentTerms:  "Net 30",
		DeliveryTerms: "DAP Bahrain",
		EstDelivery:   "5-7 weeks",
		GrandTotal:    768,
		Subtotal:      768,
		NetAmount:     768,
		Profit:        160,
		LineItems: []CostingExportLineItem{
			{
				SlNo:           1,
				Equipment:      "Temperature transmitter",
				Model:          "TM411-4EVR5/101",
				Quantity:       2,
				SuggestedPrice: 384,
				TotalPrice:     768,
				TotalCost:      608,
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "64-26", offer.OfferNumber)
	require.Equal(t, app.resolveCurrentPreparedByName("Periyasamy N"), offer.IssuedBy)

	var stored Offer
	require.NoError(t, app.db.Preload("Items").First(&stored, "id = ?", offer.ID).Error)
	require.Equal(t, "64-26", stored.OfferNumber)
	require.Equal(t, app.resolveCurrentPreparedByName("Periyasamy N"), stored.IssuedBy)
	require.Len(t, stored.Items, 1)
}

func TestSaveCostingAsOffer_UsesRevisionNumberWhenFolderOfferExists(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	seedTestCustomer(t, app.db, "SULB COMPANY B.S.C CLOSED")
	require.NoError(t, app.db.Create(&Offer{
		Base:              Base{ID: "existing-oth-03-26"},
		OfferNumber:       "OTH-03-26",
		CustomerName:      "SULB COMPANY B.S.C CLOSED",
		CustomerReference: "2911335453",
		Stage:             "Quoted",
		QuoteType:         "Quotation",
		VatRate:           10,
	}).Error)

	offer, err := app.SaveCostingAsOffer(CostingExportData{
		Date:          time.Now().Format("2006-01-02"),
		PreparedBy:    "Periyasamy Neelakandan",
		CustomerName:  "SULB COMPANY B.S.C CLOSED",
		RfqReference:  "2911335453",
		FolderNumber:  "OTH-03-26",
		CostingId:     "OTH-03-26-R1",
		QuoteType:     "Quotation",
		VatRate:       10,
		PaymentTerms:  "60 days from Date of Delivery",
		DeliveryTerms: "Delivered Duty Paid (DDP)",
		EstDelivery:   "16-18 weeks",
		GrandTotal:    48229.720,
		Subtotal:      43845.200,
		NetAmount:     43845.200,
		Profit:        7642.352,
		LineItems: []CostingExportLineItem{
			{SlNo: 1, Equipment: "Servomex analyzer", Model: "02500E1", Quantity: 1, SuggestedPrice: 15485, TotalPrice: 15485, TotalCost: 12000},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "OTH-03-26-R1", offer.OfferNumber)
}

func TestSaveCostingAsOffer_UsesRevisionNumberWhenArchivedFolderOfferExists(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	seedTestCustomer(t, app.db, "SULB COMPANY B.S.C CLOSED")
	archived := Offer{
		Base:              Base{ID: "archived-oth-03-26"},
		OfferNumber:       "OTH-03-26",
		CustomerName:      "SULB COMPANY B.S.C CLOSED",
		CustomerReference: "2911335453",
		Stage:             "Quoted",
		QuoteType:         "Quotation",
		VatRate:           10,
	}
	require.NoError(t, app.db.Create(&archived).Error)
	require.NoError(t, app.db.Delete(&archived).Error)

	offer, err := app.SaveCostingAsOffer(CostingExportData{
		Date:          time.Now().Format("2006-01-02"),
		PreparedBy:    "Periyasamy Neelakandan",
		CustomerName:  "SULB COMPANY B.S.C CLOSED",
		RfqReference:  "2911335453",
		FolderNumber:  "OTH-03-26",
		CostingId:     "OTH-03-26-R1",
		QuoteType:     "Quotation",
		VatRate:       10,
		PaymentTerms:  "60 days from Date of Delivery",
		DeliveryTerms: "Delivered Duty Paid (DDP)",
		EstDelivery:   "16-18 weeks",
		GrandTotal:    48229.720,
		Subtotal:      43845.200,
		NetAmount:     43845.200,
		Profit:        7642.352,
		LineItems: []CostingExportLineItem{
			{SlNo: 1, Equipment: "Servomex analyzer", Model: "02500E1", Quantity: 1, SuggestedPrice: 15485, TotalPrice: 15485, TotalCost: 12000},
		},
	})
	require.NoError(t, err)
	require.Equal(t, "OTH-03-26-R1", offer.OfferNumber)
}

func TestOfferNumberPrefersReferenceWhenCurrentValueIsGeneratedSequence(t *testing.T) {
	costingNumber := desiredOfferNumberFromCosting(CostingExportData{
		OfferNumber:  "48-26",
		RfqReference: "2508000",
	}, "48-26")
	require.Equal(t, "2508000", costingNumber)

	updateNumber := desiredOfferNumberFromOfferUpdate(OfferUpdateData{
		OfferNumber:       "48-26",
		CustomerReference: "2508000",
	}, "48-26")
	require.Equal(t, "2508000", updateNumber)
}

func TestRepairGeneratedOfferNumbersFromReferencesUsesOnlyUniqueReference(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	require.NoError(t, app.db.Create(&Offer{
		Base:              Base{ID: "offer-bapco-unique"},
		OfferNumber:       "48-26",
		CustomerReference: "2508000",
		CustomerName:      "BAPCO UPSTREAM W.L.L",
		Stage:             "Quoted",
	}).Error)
	require.NoError(t, app.db.Create(&Offer{
		Base:              Base{ID: "offer-shared-a"},
		OfferNumber:       "47-26",
		CustomerReference: "1-26",
		CustomerName:      "P H Trading W.L.L",
		Stage:             "Quoted",
	}).Error)
	require.NoError(t, app.db.Create(&Offer{
		Base:              Base{ID: "offer-shared-b"},
		OfferNumber:       "46-26",
		CustomerReference: "1-26",
		CustomerName:      "P H Trading W.L.L",
		Stage:             "Quoted",
	}).Error)

	require.Equal(t, 1, app.repairGeneratedOfferNumbersFromReferences())

	var unique Offer
	require.NoError(t, app.db.First(&unique, "id = ?", "offer-bapco-unique").Error)
	require.Equal(t, "2508000", unique.OfferNumber)

	var shared Offer
	require.NoError(t, app.db.First(&shared, "id = ?", "offer-shared-a").Error)
	require.Equal(t, "47-26", shared.OfferNumber)
}

func TestSaveCostingAsOffer_UpdatesSourceOfferFromCosting(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerID := seedTestCustomer(t, app.db, "Awal Dairy Company W.L.L.")
	customerName := "Awal Dairy Company W.L.L."

	created, err := app.SaveCostingAsOffer(CostingExportData{
		Date:          time.Now().Format("2006-01-02"),
		PreparedBy:    "Periyasamy N",
		CustomerID:    customerID,
		CustomerName:  customerName,
		RfqReference:  "EH-64-26-R0",
		FolderNumber:  "64-26",
		QuoteType:     "Quotation",
		VatRate:       10,
		PaymentTerms:  "Net 30",
		DeliveryTerms: "DAP Bahrain",
		EstDelivery:   "5-7 weeks",
		GrandTotal:    768,
		Subtotal:      768,
		NetAmount:     768,
		Profit:        160,
		LineItems: []CostingExportLineItem{
			{SlNo: 1, Equipment: "Temperature transmitter", Model: "TM411", Quantity: 2, SuggestedPrice: 384, TotalPrice: 768, TotalCost: 304},
		},
	})
	require.NoError(t, err)

	updated, err := app.SaveCostingAsOffer(CostingExportData{
		Source:             "offer",
		OfferID:            created.ID,
		OfferNumber:        created.OfferNumber,
		Date:               time.Now().Format("2006-01-02"),
		PreparedBy:         "Stanley Vaz",
		CustomerID:         customerID,
		CustomerName:       customerName,
		RfqReference:       "EH-64-26-R1",
		FolderNumber:       "64-26",
		Subject:            "Sub: Updated flow meter",
		Body:               "Updated commercial body",
		QuoteType:          "Budgetary Quote",
		VatRate:            0,
		PaymentTerms:       "100% Advance Payment with PO",
		DeliveryTerms:      "DAP Bahrain",
		EstDelivery:        "7-9 weeks",
		GrandTotal:         900,
		Subtotal:           900,
		NetAmount:          900,
		Profit:             250,
		TotalCost:          650,
		HiddenCharges:      25,
		TermsAndConditions: "Updated terms",
		LineItems: []CostingExportLineItem{
			{SlNo: 1, Equipment: "Flow meter", Model: "FM-900", Quantity: 1, SuggestedPrice: 900, TotalPrice: 900, TotalCost: 650, UserPrice: 900, UserPriceSet: true},
		},
	})
	require.NoError(t, err)
	require.Equal(t, created.ID, updated.ID)
	require.Equal(t, app.resolveCurrentPreparedByName("Stanley Vaz"), updated.IssuedBy)
	require.Equal(t, "Budgetary Quote", updated.QuoteType)
	require.Equal(t, 0.0, updated.VatRate)
	require.Equal(t, "Updated terms", updated.TermsAndConditions)
	require.Equal(t, "Sub: Updated flow meter", updated.Subject)
	require.Len(t, updated.Items, 1)
	require.Equal(t, "Flow meter", updated.Items[0].Equipment)

	var offerCount int64
	require.NoError(t, app.db.Model(&Offer{}).Count(&offerCount).Error)
	require.Equal(t, int64(1), offerCount)
}

func TestUpdateOfferFullHeaderOnlyPreservesExistingVatRate(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerID := seedTestCustomer(t, app.db, "VAT Preserve Customer")
	now := time.Now()
	offer := Offer{
		Base:            Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now},
		OfferNumber:     "VAT-KEEP-001",
		CustomerID:      customerID,
		CustomerName:    "VAT Preserve Customer",
		Stage:           "Quoted",
		QuotationDate:   now,
		ValidityDate:    now.AddDate(0, 1, 0),
		VatRate:         10,
		TotalValueBHD:   110,
		PaymentTerms:    "Net 30",
		DeliveryTerms:   "DAP Bahrain",
		DeliveryWeeks:   "4 weeks",
		CountryOfOrigin: "DE",
	}
	require.NoError(t, app.db.Create(&offer).Error)

	updated, err := app.UpdateOfferFull(offer.ID, OfferUpdateData{
		CustomerName:  "VAT Preserve Customer",
		ProjectName:   "Header-only save",
		QuotationDate: now.Format("2006-01-02"),
		ValidityDate:  now.AddDate(0, 1, 0).Format("2006-01-02"),
		Items:         []OfferUpdateItem{},
	})
	require.NoError(t, err)
	require.Equal(t, 10.0, updated.VatRate)

	var stored Offer
	require.NoError(t, app.db.First(&stored, "id = ?", offer.ID).Error)
	require.Equal(t, 10.0, stored.VatRate)
}

func TestCreateOfferDraftFromButlerPreservesExplicitZeroVat(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	customerID := seedTestCustomer(t, app.db, "Butler Zero VAT Customer")
	offer, err := app.CreateOfferDraftFromButler(ButlerOfferDraftRequest{
		CustomerID:    customerID,
		CustomerName:  "Butler Zero VAT Customer",
		QuoteType:     "Quotation",
		VatRate:       0,
		PaymentTerms:  "Advance",
		DeliveryTerms: "EXW",
		LineItems: []ButlerOfferDraftLineItem{{
			Description:  "Zero-rated export item",
			Equipment:    "Analyzer",
			Model:        "ZX-0",
			Quantity:     1,
			UnitPriceBHD: 100,
		}},
	})
	require.NoError(t, err)
	require.Equal(t, 0.0, offer.VatRate)
	require.Equal(t, 100.0, offer.TotalValueBHD)
}

func TestPHOfferSignatureBlockResolvesPreparedByAliases(t *testing.T) {
	expectedBlocks := []struct {
		name   string
		title  string
		mobile string
		office string
		fax    string
		email  string
	}{
		{"EBIN CHACKO", "Technical Sales Engineer", "+973 36418072", "+973 17587654", "+973 17564456", "ebin.chacko@platinumholdings.com.bh"},
		{"Periyasamy Neelakandan", "Assistant Sales Manager", "+973 34128132", "+973 17587654", "+973 17564456", "periyasamy.n@platinumholdings.com.bh"},
		{"V.M.Sundar", "Business Development Manager", "+973 38260157", "+973 17587654", "+973 17564456", "vm.sundar@platinumholdings.com.bh"},
		{"RAMYA BASKARAN", "", "+973 38890513", "+973 17587654", "+973 17564456", "support@platinumholdings.com.bh"},
		{"STANLEY VAZ", "Chief Operating Officer", "+973 36623662", "+973 17587654", "+973 17564456", ""},
		{"Abhie Kore", "General Manager", "+973 36771513", "+973 17587654", "+973 17564456", ""},
	}
	for _, expected := range expectedBlocks {
		block := resolvePHOfferSignatureBlock(expected.name)
		require.NotNil(t, block, expected.name)
		require.Equal(t, expected.name, block.DisplayName)
		require.Equal(t, expected.title, block.Title)
		require.Equal(t, []string{"P.O. BOX 815, MANAMA,", "KINGDOM OF BAHRAIN"}, block.Address)
		require.Equal(t, expected.mobile, block.Mobile)
		require.Equal(t, expected.office, block.Office)
		require.Equal(t, expected.fax, block.Fax)
		require.Equal(t, expected.email, block.Email)
		lines := phOfferSignaturePDFLines(*block, false)
		lineText := make([]string, 0, len(lines))
		for _, line := range lines {
			if strings.TrimSpace(line.Text) != "" {
				lineText = append(lineText, line.Text)
			}
		}
		require.Contains(t, lineText, expected.name)
		require.Contains(t, lineText, "P.O. BOX 815, MANAMA,")
		require.Contains(t, lineText, "KINGDOM OF BAHRAIN")
		require.Contains(t, lineText, "Mob: "+expected.mobile)
		require.Contains(t, lineText, "Office: "+expected.office)
		require.Contains(t, lineText, "Fax: "+expected.fax)
		if expected.title != "" {
			require.Contains(t, lineText, expected.title)
		}
		if expected.email != "" {
			require.Contains(t, lineText, expected.email)
		}
	}

	peri := resolvePHOfferSignatureBlock("Periyasamy N")
	require.NotNil(t, peri)
	require.Equal(t, "Periyasamy Neelakandan", peri.DisplayName)
	require.Equal(t, "Assistant Sales Manager", peri.Title)
	require.Equal(t, "periyasamy.n@platinumholdings.com.bh", peri.Email)
	require.Equal(t, "Periyasamy Neelakandan", offerIssuerDisplayName("Periyasamy N"))

	sundar := resolvePHOfferSignatureBlock("V.M. Sundar")
	require.NotNil(t, sundar)
	require.Equal(t, "V.M.Sundar", sundar.DisplayName)
	require.Equal(t, "Business Development Manager", sundar.Title)
	require.Equal(t, "vm.sundar@platinumholdings.com.bh", sundar.Email)
}

func TestGetPreparedByOptions_IncludesEmployeesAndLicenseNames(t *testing.T) {
	app := setupTestApp(t)
	migrateWorkflowRegressionTables(t, app)

	require.NoError(t, app.db.Create(&Employee{
		Base:          Base{ID: uuid.New().String()},
		EmployeeCode:  "PH-NEW",
		FullName:      "New PH Engineer",
		PreferredName: "New PH",
		IsActive:      true,
	}).Error)
	require.NoError(t, app.db.Create(&LicenseKey{
		Key:         "PH-SLS-NEWHR1",
		Role:        "sales",
		DisplayName: "Commission Owner",
		CreatedBy:   "test",
	}).Error)

	options, err := app.GetPreparedByOptions()
	require.NoError(t, err)
	require.Contains(t, options, "Periyasamy N")
	require.Contains(t, options, "Periyasamy Neelakandan")
	require.Contains(t, options, "V.M.Sundar")
	require.Contains(t, options, "New PH Engineer")
	require.Contains(t, options, "Commission Owner")
}
