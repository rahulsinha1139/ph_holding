package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func setFeedbackHardeningTestUser(app *App, id, role, permissions, displayName string) {
	app.currentUserID = id
	app.currentUser = &User{
		Base:        Base{ID: id},
		Username:    id,
		DisplayName: displayName,
		RoleName:    role,
		Role: Role{
			Name:        role,
			DisplayName: role,
			Permissions: permissions,
		},
	}
}

func TestParseTenderFolderName(t *testing.T) {
	preview, ok := ParseTenderFolderName("29.0 BAPCO UPSTREAM-PIT")
	require.True(t, ok)
	require.Equal(t, "T-29", preview.WorkflowKey)
	require.Equal(t, "BAPCO UPSTREAM-PIT", preview.Title)
	require.Equal(t, "RFQ Received / Tender", preview.Status)

	_, ok = ParseTenderFolderName("BAPCO no numeric prefix")
	require.False(t, ok)

	_, ok = ParseTenderFolderName("../29.0 BAPCO")
	require.False(t, ok)

	preview, ok = ParseTenderFolderName("29 BAPCO UPSTREAM")
	require.True(t, ok)
	require.Equal(t, "T-29", preview.WorkflowKey)
}

func TestUpdateOpportunityCommercialFieldsRejectsUnsupportedFieldsAndAudits(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Opportunity{}, &OpportunityComment{}))
	setFeedbackHardeningTestUser(app, "sales-redteam", "sales", `["dashboard:view","offers:view","offers:edit"]`, "Peri Red Team")

	opportunity := Opportunity{
		Base:         Base{ID: "opp-redteam-1", CreatedBy: "seed"},
		FolderNumber: "T-29",
		Title:        "Old title",
		CustomerName: "Old Customer",
		Stage:        "Qualified",
	}
	require.NoError(t, app.db.Create(&opportunity).Error)

	updated, err := app.UpdateOpportunityCommercialFields(opportunity.ID, map[string]interface{}{
		"title":         "  Hardened opportunity  ",
		"customer_name": "  BAPCO  ",
		"created_by":    "attacker",
		"id":            "opp-attacker",
		"deleted_at":    time.Now(),
	})
	require.NoError(t, err)
	require.Equal(t, opportunity.ID, updated.ID)
	require.Equal(t, "Hardened opportunity", updated.Title)
	require.Equal(t, "BAPCO", updated.CustomerName)
	require.Equal(t, "seed", updated.CreatedBy)

	var comments []OpportunityComment
	require.NoError(t, app.db.Where("opportunity_id = ?", opportunity.ID).Find(&comments).Error)
	require.Len(t, comments, 1)
	require.Equal(t, "Peri Red Team", comments[0].CreatedBy)
	require.Contains(t, comments[0].Comment, "title")
	require.Contains(t, comments[0].Comment, "customer_name")
	require.NotContains(t, comments[0].Comment, "created_by")

	_, err = app.UpdateOpportunityCommercialFields(opportunity.ID, map[string]interface{}{
		"deleted_at": time.Now(),
		"created_by": "attacker",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no supported")
}

func TestUpdateOpportunityCommercialFieldsRequiresAuthenticatedOrgUser(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Opportunity{}, &OpportunityComment{}))
	setFeedbackHardeningTestUser(app, "viewer-redteam", "sales", `["offers:view"]`, "Viewer")

	opportunity := Opportunity{Base: Base{ID: "opp-redteam-rbac"}, FolderNumber: "T-30", Title: "Guarded"}
	require.NoError(t, app.db.Create(&opportunity).Error)

	_, err := app.UpdateOpportunityCommercialFields(opportunity.ID, map[string]interface{}{"title": "blocked"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "dashboard:view")
}

func TestDeleteOpportunityRequiresAdminAndSoftDeletes(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Opportunity{}))

	opportunity := Opportunity{Base: Base{ID: "opp-delete-admin"}, FolderNumber: "D-1", Title: "Delete me"}
	require.NoError(t, app.db.Create(&opportunity).Error)

	setFeedbackHardeningTestUser(app, "sales-delete", "sales", `["dashboard:view","offers:delete"]`, "Sales Delete")
	err := app.DeleteOpportunity(opportunity.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "only admins")

	setFeedbackHardeningTestUser(app, "admin-delete", "admin", `["*"]`, "Admin Delete")
	require.NoError(t, app.DeleteOpportunity(opportunity.ID))

	var remaining int64
	require.NoError(t, app.db.Model(&Opportunity{}).Where("id = ?", opportunity.ID).Count(&remaining).Error)
	require.EqualValues(t, 0, remaining)

	var archived Opportunity
	require.NoError(t, app.db.Unscoped().First(&archived, "id = ?", opportunity.ID).Error)
	require.True(t, archived.DeletedAt.Valid)
}

func TestUnifiedOfferThreadDoesNotBleedSimilarWorkflowKeys(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Offer{}, &OfferNote{}, &Opportunity{}, &OpportunityComment{}, &RFQData{}, &RFQComment{}))

	offer := Offer{
		Base:              Base{ID: "offer-thread-1"},
		OfferNumber:       "29-26",
		CustomerReference: "T-29",
		CustomerName:      "BAPCO",
		Stage:             "Quoted",
	}
	require.NoError(t, app.db.Create(&offer).Error)

	matched := Opportunity{Base: Base{ID: "opp-thread-29"}, FolderNumber: "T-29", Title: "Tender T-29 BAPCO"}
	nearMiss := Opportunity{Base: Base{ID: "opp-thread-290"}, FolderName: "Tender T-290 should not leak", Title: "Secret scope"}
	require.NoError(t, app.db.Create(&matched).Error)
	require.NoError(t, app.db.Create(&nearMiss).Error)
	require.NoError(t, app.db.Create(&OpportunityComment{OpportunityID: matched.ID, Comment: "matched visible", CreatedBy: "Peri", CreatedAt: time.Now()}).Error)
	require.NoError(t, app.db.Create(&OpportunityComment{OpportunityID: nearMiss.ID, Comment: "near miss hidden", CreatedBy: "Attacker", CreatedAt: time.Now()}).Error)
	require.NoError(t, app.db.Create(&OfferNote{Base: Base{ID: "note-thread-1"}, OfferID: offer.ID, Content: "offer note", NoteDate: time.Now()}).Error)

	thread, err := app.GetUnifiedOfferThread(offer.ID)
	require.NoError(t, err)

	comments := make([]string, 0, len(thread))
	for _, entry := range thread {
		comments = append(comments, entry.Comment)
	}
	require.Contains(t, comments, "offer note")
	require.Contains(t, comments, "matched visible")
	require.NotContains(t, comments, "near miss hidden")
}

func TestProjectAdminActionsRequireDeletePermissionAndStayOutOfActiveList(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Project{}))

	project := Project{Base: Base{ID: "proj-redteam-1"}, Name: "Commercial Cleanup", Status: "active"}
	require.NoError(t, app.db.Create(&project).Error)

	setFeedbackHardeningTestUser(app, "manager-redteam", "manager", `["projects:view","projects:update"]`, "Manager")
	_, err := app.UpdateCollaborativeProject(project.ID, map[string]interface{}{"status": "deleted"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "projects:delete")

	var still Project
	require.NoError(t, app.db.First(&still, "id = ?", project.ID).Error)
	require.Equal(t, "active", still.Status)

	setFeedbackHardeningTestUser(app, "admin-redteam", "admin", `["*"]`, "Admin")
	archived, err := app.ArchiveCollaborativeProject(project.ID, "red-team archive")
	require.NoError(t, err)
	require.Equal(t, "archived", archived.Status)

	active, err := app.ListCollaborativeProjects(true)
	require.NoError(t, err)
	for _, item := range active {
		require.NotEqual(t, project.ID, item.ID)
	}
}

func TestExpenseNumberingSkipsExistingAndPreservesNewFields(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.EnsureExpenseFoundation())

	category, err := app.CreateExpenseCategory(ExpenseCategory{Name: "Red Team Expense", Code: "RT_EXP"})
	require.NoError(t, err)

	currentYear := time.Now().Year()
	require.NoError(t, app.db.Create(&ExpenseEntry{
		EntryNumber: fmt.Sprintf("EXP-%d-0001", currentYear),
		Description: "Existing sequence",
		CategoryID:  category.ID,
		ExpenseDate: time.Now(),
		Amount:      10,
	}).Error)

	inventoryItemID := "stock-item-1"
	entry, err := app.CreateExpenseEntry(ExpenseEntry{
		Description:        "Inventory linked expense",
		CategoryID:         category.ID,
		ExpenseDate:        time.Now(),
		Amount:             25,
		PaymentMethod:      "  Bank Transfer  ",
		InventoryItemID:    &inventoryItemID,
		InventoryReference: "  PO-REDTEAM  ",
	})
	require.NoError(t, err)
	require.Equal(t, fmt.Sprintf("EXP-%d-0002", currentYear), entry.EntryNumber)
	require.Equal(t, "Bank Transfer", entry.PaymentMethod)
	require.True(t, entry.InventoryLinked)
	require.Equal(t, "PO-REDTEAM", entry.InventoryReference)

	_, err = app.CreateExpenseEntry(ExpenseEntry{
		EntryNumber: fmt.Sprintf("EXP-%d-0002", currentYear),
		Description: "Duplicate user number",
		CategoryID:  category.ID,
		ExpenseDate: time.Now(),
		Amount:      30,
	})
	require.Error(t, err)
}

func TestPreviewCustomerDataQualityFlagsReviewQueue(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerMaster{}, &Opportunity{}, &Offer{}, &DataQualityReview{}))

	require.NoError(t, app.db.Create(&CustomerMaster{CustomerID: "C-1", CustomerCode: "C-1", BusinessName: "Acme W.L.L"}).Error)
	require.NoError(t, app.db.Create(&CustomerMaster{CustomerID: "C-2", CustomerCode: "C-2", BusinessName: "ACME WLL"}).Error)
	require.NoError(t, app.db.Create(&Opportunity{FolderNumber: "T-29", Stage: "RFQ Received"}).Error)
	require.NoError(t, app.db.Create(&Offer{OfferNumber: "29-26", CustomerID: "", CustomerName: "", Stage: "Quoted"}).Error)

	issues, err := app.PreviewCustomerDataQuality(20)
	require.NoError(t, err)

	issueTypes := make(map[string]bool)
	for _, issue := range issues {
		issueTypes[issue.IssueType] = true
	}
	require.True(t, issueTypes["duplicate_customer"])
	require.True(t, issueTypes["blank_opportunity_name"])
	require.True(t, issueTypes["missing_customer_link"])
	require.True(t, issueTypes["offer_missing_customer"])
}

func TestEnsureDataQualityReviewFoundationIsIdempotent(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.Exec("DROP TABLE IF EXISTS data_quality_reviews").Error)
	require.NoError(t, app.db.Exec("CREATE TABLE data_quality_reviews (id TEXT PRIMARY KEY)").Error)

	require.NoError(t, app.ensureDataQualityReviewFoundation())
	require.NoError(t, app.ensureDataQualityReviewFoundation())

	var columns []struct {
		Name string `gorm:"column:name"`
	}
	require.NoError(t, app.db.Raw("PRAGMA table_info(data_quality_reviews)").Scan(&columns).Error)
	columnNames := make(map[string]bool, len(columns))
	for _, column := range columns {
		columnNames[column.Name] = true
	}
	require.True(t, columnNames["issue_id"])
	require.True(t, columnNames["reviewed_by"])
	require.True(t, columnNames["updated_at"])
}

func TestDataQualityReviewIsAdminOnlyAndSuppressesResolvedIssue(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerMaster{}, &Opportunity{}, &Offer{}, &DataQualityReview{}))

	require.NoError(t, app.db.Create(&CustomerMaster{CustomerID: "C-1", CustomerCode: "C-1", BusinessName: "Acme W.L.L"}).Error)
	require.NoError(t, app.db.Create(&CustomerMaster{CustomerID: "C-2", CustomerCode: "C-2", BusinessName: "ACME WLL"}).Error)

	setFeedbackHardeningTestUser(app, "manager-dq", "manager", `["customers:view","customers:edit"]`, "Manager")
	issues, err := app.PreviewCustomerDataQuality(20)
	require.NoError(t, err)
	require.NotEmpty(t, issues)
	_, err = app.ReviewDataQualityIssue(issues[0], "resolved", "manager should not be able to clear this")
	require.Error(t, err)
	require.Contains(t, err.Error(), "admin")

	setFeedbackHardeningTestUser(app, "admin-dq", "admin", `["*"]`, "Admin Reviewer")
	review, err := app.ReviewDataQualityIssue(issues[0], "resolved", "confirmed duplicate was already merged")
	require.NoError(t, err)
	require.Equal(t, "resolved", review.Status)
	require.Equal(t, "Admin Reviewer", review.ReviewedBy)

	refreshed, err := app.PreviewCustomerDataQuality(20)
	require.NoError(t, err)
	for _, issue := range refreshed {
		require.NotEqual(t, issues[0].ID, issue.ID)
	}

	history, err := app.GetDataQualityReviewHistory(10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, issues[0].ID, history[0].IssueID)
}

func TestReviewedDataQualityIssueStaysVisibleWithReviewContext(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerMaster{}, &Opportunity{}, &Offer{}, &DataQualityReview{}))

	require.NoError(t, app.db.Create(&Opportunity{Base: Base{ID: "opp-dq-review"}, FolderNumber: "T-31"}).Error)
	setFeedbackHardeningTestUser(app, "admin-dq-review", "admin", `["*"]`, "Admin Reviewer")

	issues, err := app.PreviewCustomerDataQuality(20)
	require.NoError(t, err)
	var target DataQualityIssue
	for _, issue := range issues {
		if issue.IssueType == "blank_opportunity_name" {
			target = issue
			break
		}
	}
	require.NotEmpty(t, target.ID)

	_, err = app.ReviewDataQualityIssue(target, "reviewed", "assigned to sales ops")
	require.NoError(t, err)

	refreshed, err := app.PreviewCustomerDataQuality(20)
	require.NoError(t, err)
	var reviewed DataQualityIssue
	for _, issue := range refreshed {
		if issue.ID == target.ID {
			reviewed = issue
			break
		}
	}
	require.Equal(t, "reviewed", reviewed.ReviewStatus)
	require.Equal(t, "assigned to sales ops", reviewed.ReviewNote)
	require.Equal(t, "Admin Reviewer", reviewed.ReviewedBy)
}

func TestGetInventoryPendingFulfillmentReportDerivesOrderGaps(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Order{}, &OrderItem{}, &DeliveryNote{}, &DeliveryNoteItem{}))

	order := Order{
		OrderNumber:  "ORD-TEST-1",
		CustomerID:   "cust-1",
		CustomerName: "BAPCO",
		OrderDate:    time.Now(),
	}
	require.NoError(t, app.db.Create(&order).Error)
	orderItem := OrderItem{OrderID: order.ID, ProductCode: "FMR20", Description: "Level transmitter", Quantity: 5, UnitPrice: 100}
	require.NoError(t, app.db.Create(&orderItem).Error)
	require.NoError(t, app.db.Create(&DeliveryNote{
		DNNumber:   "DN-TEST-1",
		OrderID:    order.ID,
		CustomerID: "cust-1",
		Items: []DeliveryNoteItem{
			{ProductCode: "FMR20", Description: "Level transmitter", OrderItemID: orderItem.ID, QuantityDelivered: 2},
		},
	}).Error)

	rows, err := app.GetInventoryPendingFulfillmentReport(20)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "ORD-TEST-1", rows[0].OrderNumber)
	require.Equal(t, 5.0, rows[0].OrderedQuantity)
	require.Equal(t, 2.0, rows[0].DeliveredQuantity)
	require.Equal(t, 3.0, rows[0].PendingQuantity)
}
