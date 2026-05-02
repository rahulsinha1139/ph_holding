package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func setupHybridFeatureTestApp(t *testing.T) *App {
	t.Helper()

	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(
		&Setting{},
		&Role{},
		&User{},
		&LicenseKey{},
		&Device{},
		&DeviceUser{},
		&Employee{},
		&EmployeeAccessLink{},
		&Project{},
		&ProjectMember{},
		&Notification{},
		&NotificationReceipt{},
		&TaskItem{},
		&TaskComment{},
		&TaskActivity{},
		&CollaborativePendingOperation{},
		&ChartOfAccount{},
		&JournalEntry{},
		&JournalLine{},
		&BankAccount{},
		&Invoice{},
		&SupplierInvoice{},
		&BankStatementLine{},
		&BankExpenseEntry{},
		&ExpenseCategory{},
		&ExpenseVendor{},
		&ExpenseEntry{},
		&ExpenseAllocation{},
		&RecurringExpense{},
		&ExpenseAttachment{},
		&ExpenseApproval{},
		&EmployeeCompensationProfile{},
		&PayrollPeriod{},
		&PayrollRun{},
		&PayrollRunItem{},
		&PayrollComponent{},
		&PayrollPayout{},
	))
	require.NoError(t, app.SeedDefaultRoles())
	require.NoError(t, app.EnsureExpenseFoundation())
	require.NoError(t, app.EnsurePayrollFoundation())
	return app
}

func seedLicenseRecord(t *testing.T, app *App, key, role, displayName string, activated bool, deviceHash string) LicenseKey {
	t.Helper()

	license := LicenseKey{
		Key:         key,
		Role:        role,
		DisplayName: displayName,
		Activated:   activated,
		CreatedBy:   "test",
		Notes:       fmt.Sprintf("Assigned to %s", displayName),
	}
	if deviceHash != "" {
		license.DeviceHash = deviceHash
	}
	if activated {
		now := time.Now()
		license.ActivatedAt = &now
	}
	require.NoError(t, app.db.Create(&license).Error)
	return license
}

func seedCurrentAdminContext(t *testing.T, app *App, displayName string) CurrentEmployeeContext {
	t.Helper()

	deviceHash := app.getDeviceHash()
	seedLicenseRecord(t, app, "PH-ADM-RAHUL1", "admin", displayName, true, deviceHash)
	require.NoError(t, app.EnsureCollaborativeFoundation())

	current, err := app.GetCurrentEmployeeContext()
	require.NoError(t, err)
	return current
}

func createEmployeeForTest(t *testing.T, app *App, fullName, preferredName, department, jobTitle string) Employee {
	t.Helper()

	employee, err := app.CreateEmployeeProfile(Employee{
		FullName:         fullName,
		PreferredName:    preferredName,
		Department:       department,
		JobTitle:         jobTitle,
		EmploymentStatus: "active",
		IsActive:         true,
	})
	require.NoError(t, err)
	return employee
}

func TestEnsureCollaborativeFoundation_ResolvesCurrentEmployeeFromActivatedLicense(t *testing.T) {
	app := setupHybridFeatureTestApp(t)

	current := seedCurrentAdminContext(t, app, "Rahul")

	require.Equal(t, "Rahul", current.EmployeeName)
	require.Equal(t, "license", current.ResolvedBy)
	require.Equal(t, "admin", current.LicenseRole)
	require.Contains(t, current.Permissions, "*")

	var employees []Employee
	require.NoError(t, app.db.Order("full_name ASC").Find(&employees).Error)
	require.Len(t, employees, 1)
	require.Equal(t, "Rahul", employees[0].FullName)

	var links []EmployeeAccessLink
	require.NoError(t, app.db.Find(&links).Error)
	require.Len(t, links, 1)
	require.Equal(t, employees[0].ID, links[0].EmployeeID)
	require.Equal(t, current.LicenseKey, links[0].LicenseKey)
	require.Equal(t, "active", links[0].AccessStatus)
}

func TestCollaborativeTaskFlow_CreatesNotificationsAndSupportsReadLifecycle(t *testing.T) {
	app := setupHybridFeatureTestApp(t)
	seedCurrentAdminContext(t, app, "Rahul")

	assignee := createEmployeeForTest(t, app, "Abhie Admin", "Abhie", "Management", "Manager")
	seedLicenseRecord(t, app, "PH-MGR-ABHIE1", "manager", "Abhie", false, "")
	link, err := app.CreateEmployeeAccessLink(EmployeeAccessLink{
		EmployeeID:   assignee.ID,
		LicenseKey:   "PH-MGR-ABHIE1",
		UserID:       "abhie-user",
		AccessStatus: "active",
	})
	require.NoError(t, err)
	require.Equal(t, "abhie-user", link.UserID)

	task, err := app.CreateCollaborativeTask(TaskItem{
		Title:              "Review salary expense forecast",
		Description:        "Confirm this month's HR and payroll forecast.",
		TaskType:           "internal",
		Priority:           "high",
		AssigneeEmployeeID: &assignee.ID,
	})
	require.NoError(t, err)
	require.Equal(t, "Review salary expense forecast", task.Title)
	require.NotEmpty(t, task.CreatorEmployeeID)

	var activities []TaskActivity
	require.NoError(t, app.db.Where("task_id = ?", task.ID).Find(&activities).Error)
	require.Len(t, activities, 1)
	require.Equal(t, "created", activities[0].ActivityType)

	var notifications []Notification
	require.NoError(t, app.db.Where("employee_id = ?", assignee.ID).Find(&notifications).Error)
	require.Len(t, notifications, 1)
	require.Contains(t, notifications[0].Message, task.Title)

	var receipts []NotificationReceipt
	require.NoError(t, app.db.Where("notification_id = ?", notifications[0].ID).Find(&receipts).Error)
	require.Len(t, receipts, 1)
	require.Equal(t, "created", receipts[0].ReceiptType)

	var pendingOps []CollaborativePendingOperation
	require.NoError(t, app.db.Find(&pendingOps).Error)
	require.GreaterOrEqual(t, len(pendingOps), 4)

	app.currentUserID = "abhie-user"
	app.currentUser = &User{
		Base:     Base{ID: "abhie-user"},
		Username: "abhie",
		RoleName: "manager",
		Role: Role{
			Name:        "manager",
			DisplayName: "Manager",
			Permissions: `["notifications:view","notifications:update","tasks:view"]`,
		},
	}

	current, err := app.GetCurrentEmployeeContext()
	require.NoError(t, err)
	require.Equal(t, assignee.ID, current.EmployeeID)
	require.Equal(t, "user", current.ResolvedBy)

	unread, err := app.GetUnreadNotificationsCount()
	require.NoError(t, err)
	require.Equal(t, 1, unread)

	feed, err := app.ListNotificationFeed(10, true)
	require.NoError(t, err)
	require.Len(t, feed, 1)
	require.Equal(t, notifications[0].ID, feed[0].ID)

	require.NoError(t, app.MarkNotificationAsRead(feed[0].ID))

	unread, err = app.GetUnreadNotificationsCount()
	require.NoError(t, err)
	require.Zero(t, unread)

	var updated Notification
	require.NoError(t, app.db.First(&updated, "id = ?", feed[0].ID).Error)
	require.Equal(t, "read", updated.Status)
	require.NotNil(t, updated.ReadAt)
}

func TestListMyCollaborativeTasks_ResolvesActivatedLicenseByPreferredName(t *testing.T) {
	app := setupHybridFeatureTestApp(t)
	adminContext := seedCurrentAdminContext(t, app, "Rahul")

	assignee := createEmployeeForTest(t, app, "Abhie Admin", "Abhie", "Management", "Manager")
	task, err := app.CreateCollaborativeTask(TaskItem{
		Title:              "Review mentor dashboard tasks",
		Description:        "Confirm assigned tasks land on the user's dashboard.",
		TaskType:           "internal",
		Priority:           "high",
		AssigneeEmployeeID: &assignee.ID,
	})
	require.NoError(t, err)

	require.NoError(t, app.db.Model(&LicenseKey{}).
		Where("key = ?", adminContext.LicenseKey).
		Updates(map[string]interface{}{"activated": false, "device_hash": ""}).Error)
	seedLicenseRecord(t, app, "PH-MGR-ABHIE1", "manager", "Abhie", true, app.getDeviceHash())

	current, err := app.GetCurrentEmployeeContext()
	require.NoError(t, err)
	require.Equal(t, assignee.ID, current.EmployeeID)
	require.Equal(t, "Abhie Admin", current.EmployeeName)

	myTasks, err := app.ListMyCollaborativeTasks(false)
	require.NoError(t, err)
	require.Len(t, myTasks, 1)
	require.Equal(t, task.ID, myTasks[0].ID)
	require.Equal(t, "Abhie Admin", myTasks[0].AssigneeName)
}

func TestAddCollaborativeTaskComment_AllowsViewOnlyCollaborator(t *testing.T) {
	app := setupHybridFeatureTestApp(t)
	seedCurrentAdminContext(t, app, "Rahul")

	commenter := createEmployeeForTest(t, app, "Viewer User", "Viewer", "Operations", "Coordinator")
	seedLicenseRecord(t, app, "PH-STF-VIEWR1", "staff", "Viewer User", false, "")
	_, err := app.CreateEmployeeAccessLink(EmployeeAccessLink{
		EmployeeID:   commenter.ID,
		LicenseKey:   "PH-STF-VIEWR1",
		UserID:       "viewer-user",
		AccessStatus: "active",
	})
	require.NoError(t, err)

	task, err := app.CreateCollaborativeTask(TaskItem{
		Title:              "Check January bank imports",
		Description:        "Review the imported OCR lines for January.",
		TaskType:           "internal",
		Priority:           "medium",
		AssigneeEmployeeID: &commenter.ID,
	})
	require.NoError(t, err)

	app.currentUserID = "viewer-user"
	app.currentUser = &User{
		Base:     Base{ID: "viewer-user"},
		Username: "viewer",
		RoleName: "staff",
		Role: Role{
			Name:        "staff",
			DisplayName: "Staff",
			Permissions: `["notifications:view","notifications:update","tasks:view"]`,
		},
	}

	comment, err := app.AddCollaborativeTaskComment(task.ID, "I have reviewed this and added my notes.")
	require.NoError(t, err)
	require.Equal(t, commenter.ID, comment.EmployeeID)
	require.Equal(t, "Viewer User", comment.EmployeeName)

	var persisted TaskComment
	require.NoError(t, app.db.First(&persisted, "id = ?", comment.ID).Error)
	require.Equal(t, task.ID, persisted.TaskID)
	require.Equal(t, "I have reviewed this and added my notes.", persisted.Body)
}

func TestReassignEmployeeLicenseAccess_DeactivatesPreviousLinkAndSyncsDisplayName(t *testing.T) {
	app := setupHybridFeatureTestApp(t)
	seedCurrentAdminContext(t, app, "Rahul")

	previousOwner := createEmployeeForTest(t, app, "Rishu Admin", "Rishu", "Management", "Admin")
	newOwner := createEmployeeForTest(t, app, "Rahul Sinha", "Rahul", "Management", "Admin")
	seedLicenseRecord(t, app, "PH-ADM-RELNK1", "admin", "Rishu", false, "")

	_, err := app.CreateEmployeeAccessLink(EmployeeAccessLink{
		EmployeeID: previousOwner.ID,
		LicenseKey: "PH-ADM-RELNK1",
	})
	require.NoError(t, err)

	reassigned, err := app.ReassignEmployeeLicenseAccess(newOwner.ID, "PH-ADM-RELNK1", true)
	require.NoError(t, err)
	require.Equal(t, newOwner.ID, reassigned.EmployeeID)
	require.True(t, reassigned.IsPrimary)
	require.Equal(t, "active", reassigned.AccessStatus)

	var priorLink EmployeeAccessLink
	require.NoError(t, app.db.Where("employee_id = ? AND license_key = ?", previousOwner.ID, "PH-ADM-RELNK1").First(&priorLink).Error)
	require.False(t, priorLink.IsPrimary)
	require.Equal(t, "inactive", priorLink.AccessStatus)

	var refreshed LicenseKey
	require.NoError(t, app.db.Where("key = ?", "PH-ADM-RELNK1").First(&refreshed).Error)
	require.Equal(t, "Rahul", refreshed.DisplayName)
	require.Contains(t, refreshed.Notes, "Rahul Sinha")
}

func TestProjectAndContributionSummary_ReflectsAssignmentsAndTaskProgress(t *testing.T) {
	app := setupHybridFeatureTestApp(t)
	seedCurrentAdminContext(t, app, "Rahul")

	peri := createEmployeeForTest(t, app, "Peri Sales", "Peri", "Sales", "Account Manager")
	seedLicenseRecord(t, app, "PH-SLS-PERI01", "sales", "Peri", false, "")
	_, err := app.CreateEmployeeAccessLink(EmployeeAccessLink{
		EmployeeID: peri.ID,
		LicenseKey: "PH-SLS-PERI01",
		DeviceID:   "device-sales",
	})
	require.NoError(t, err)

	project, err := app.CreateCollaborativeProject(Project{
		Name:        "EWA Retrofit",
		ProjectType: "customer",
		Description: "Cross-functional delivery project",
	})
	require.NoError(t, err)

	member, err := app.AddCollaborativeProjectMember(project.ID, peri.ID, "Owner")
	require.NoError(t, err)
	require.Equal(t, peri.ID, member.EmployeeID)

	completedTask, err := app.CreateCollaborativeTask(TaskItem{
		Title:              "Prepare updated commercial offer",
		ProjectID:          &project.ID,
		Priority:           "high",
		AssigneeEmployeeID: &peri.ID,
	})
	require.NoError(t, err)
	require.NoError(t, app.UpdateCollaborativeTaskStatus(completedTask.ID, "completed", "Done"))

	yesterday := time.Now().Add(-24 * time.Hour)
	blockedTask, err := app.CreateCollaborativeTask(TaskItem{
		Title:              "Resolve delivery blocker",
		ProjectID:          &project.ID,
		Priority:           "high",
		DueDate:            &yesterday,
		AssigneeEmployeeID: &peri.ID,
	})
	require.NoError(t, err)
	require.NoError(t, app.UpdateCollaborativeTaskStatus(blockedTask.ID, "blocked", "Awaiting supplier confirmation"))

	summaries, err := app.ListEmployeeContributionSummaries()
	require.NoError(t, err)

	var periSummary EmployeeContributionSummary
	found := false
	for _, summary := range summaries {
		if summary.EmployeeID == peri.ID {
			periSummary = summary
			found = true
			break
		}
	}
	require.True(t, found)
	require.Equal(t, 1, periSummary.ActiveProjectCount)
	require.Equal(t, 1, periSummary.ActiveTaskCount)
	require.Equal(t, 1, periSummary.CompletedTaskCount)
	require.Equal(t, 1, periSummary.BlockedTaskCount)
	require.Equal(t, 1, periSummary.OverdueTaskCount)
	require.InDelta(t, 50.0, periSummary.CompletionRate, 0.001)
	require.Equal(t, "PH-SLS-PERI01", periSummary.PrimaryLicenseKey)
}

func TestExpenseLifecycleAndCashFlowProjection_TracksExpenseAndRecurringCommitments(t *testing.T) {
	app := setupHybridFeatureTestApp(t)
	seedCurrentAdminContext(t, app, "Rahul")

	categories, err := app.ListExpenseCategories(true)
	require.NoError(t, err)
	require.NotEmpty(t, categories)
	var rentCategory ExpenseCategory
	for _, category := range categories {
		if category.Code == "RENT" {
			rentCategory = category
			break
		}
	}
	require.NotEmpty(t, rentCategory.ID)

	vendor, err := app.CreateExpenseVendor(ExpenseVendor{Name: "Landlord LLC", PaymentTerms: "Net 30"})
	require.NoError(t, err)

	require.NoError(t, app.db.Create(&BankAccount{
		Base:           Base{CreatedBy: "test"},
		BankName:       "ABC Bank",
		AccountNumber:  "123456",
		AccountName:    "Operating Account",
		Currency:       "BHD",
		CurrentBalance: 5000,
		IsActive:       true,
	}).Error)

	dueTomorrow := time.Now().Add(24 * time.Hour)
	entry, err := app.CreateExpenseEntry(ExpenseEntry{
		ExpenseDate: time.Now(),
		DueDate:     &dueTomorrow,
		Description: "April office rent",
		CategoryID:  rentCategory.ID,
		VendorID:    &vendor.ID,
		Amount:      100,
		VATAmount:   10,
	})
	require.NoError(t, err)
	require.Equal(t, "draft", entry.Status)

	entry, err = app.SubmitExpenseEntry(entry.ID)
	require.NoError(t, err)
	require.Equal(t, "submitted", entry.Status)

	entry, err = app.ApproveExpenseEntry(entry.ID, "Approved for payment")
	require.NoError(t, err)
	require.Equal(t, "approved", entry.Status)

	recurring, err := app.CreateRecurringExpense(RecurringExpense{
		Name:             "Antivirus Subscription",
		Description:      "Annual endpoint protection",
		CategoryID:       rentCategory.ID,
		DefaultAmount:    25,
		DefaultVATAmount: 2.5,
		Currency:         "BHD",
		Frequency:        "monthly",
		IntervalValue:    1,
		NextRunDate:      dueTomorrow,
		IsActive:         true,
	})
	require.NoError(t, err)
	require.Equal(t, "Antivirus Subscription", recurring.Name)

	projectionBeforePayment, err := app.GetCashFlowProjection(10)
	require.NoError(t, err)
	require.InDelta(t, 137.5, projectionBeforePayment.TotalOutflows, 0.001)

	entry, err = app.PostExpenseEntry(entry.ID)
	require.NoError(t, err)
	require.Equal(t, "posted", entry.Status)
	require.NotNil(t, entry.JournalEntryID)

	entry, err = app.MarkExpenseEntryPaid(entry.ID, time.Now().Format(time.RFC3339), "BANK-001", "bank-nbb", "NEFT")
	require.NoError(t, err)
	require.Equal(t, "paid", entry.Status)
	require.Equal(t, "paid", entry.PaymentStatus)
	require.Equal(t, "NEFT", entry.PaymentMethod)

	projectionAfterPayment, err := app.GetCashFlowProjection(10)
	require.NoError(t, err)
	require.InDelta(t, 27.5, projectionAfterPayment.TotalOutflows, 0.001)

	summary, err := app.ListExpenseDashboardSummary()
	require.NoError(t, err)
	require.Equal(t, 1, summary.TotalRecurring)
}

func TestPayrollLifecycleAndCashFlowProjection_TracksPayrollLiabilityAndPayout(t *testing.T) {
	app := setupHybridFeatureTestApp(t)
	seedCurrentAdminContext(t, app, "Rahul")

	employee := createEmployeeForTest(t, app, "Stanley Finance", "Stanley", "Finance", "Accountant")

	profile, err := app.UpsertEmployeeCompensationProfile(EmployeeCompensationProfile{
		EmployeeID:         employee.ID,
		PayFrequency:       "monthly",
		Currency:           "BHD",
		BaseSalary:         1000,
		HousingAllowance:   200,
		TransportAllowance: 100,
		OtherAllowance:     50,
		StandardDeduction:  100,
		TaxDeduction:       50,
		EmployerCost:       75,
		IsActive:           true,
	})
	require.NoError(t, err)
	require.Equal(t, employee.ID, profile.EmployeeID)

	paymentDate := time.Now().Add(48 * time.Hour)
	period, err := app.CreatePayrollPeriod(PayrollPeriod{
		Name:        "April 2026 Payroll",
		PeriodStart: time.Now().Add(-24 * time.Hour),
		PeriodEnd:   paymentDate,
		PaymentDate: &paymentDate,
	})
	require.NoError(t, err)

	run, err := app.GeneratePayrollRun(period.ID)
	require.NoError(t, err)
	require.Equal(t, 1, run.TotalEmployees)
	require.InDelta(t, 1350.0, run.GrossTotal, 0.001)
	require.InDelta(t, 150.0, run.DeductionsTotal, 0.001)
	require.InDelta(t, 1200.0, run.NetTotal, 0.001)
	require.InDelta(t, 75.0, run.EmployerCostTotal, 0.001)

	run, err = app.ApprovePayrollRun(run.ID, "Approved")
	require.NoError(t, err)
	require.Equal(t, "approved", run.Status)

	projectionBeforePayout, err := app.GetCashFlowProjection(10)
	require.NoError(t, err)
	require.InDelta(t, 1425.0, projectionBeforePayout.TotalOutflows, 0.001)

	run, err = app.PostPayrollRun(run.ID)
	require.NoError(t, err)
	require.Equal(t, "posted", run.Status)
	require.NotNil(t, run.JournalEntryID)

	postedPayrollExpenses, err := app.ListExpenseEntries("posted", true)
	require.NoError(t, err)
	require.Len(t, postedPayrollExpenses, 1)
	require.Equal(t, "payroll", postedPayrollExpenses[0].SourceType)
	require.NotNil(t, postedPayrollExpenses[0].SourceRefID)
	require.Equal(t, run.ID, *postedPayrollExpenses[0].SourceRefID)
	require.Equal(t, "unpaid", postedPayrollExpenses[0].PaymentStatus)
	require.InDelta(t, run.NetTotal, postedPayrollExpenses[0].TotalAmount, 0.001)

	run, err = app.MarkPayrollRunPaid(run.ID, paymentDate.Format(time.RFC3339), "PAYRUN-001", "")
	require.NoError(t, err)
	require.Equal(t, "paid", run.Status)
	require.NotNil(t, run.PaidAt)
	require.NotNil(t, run.PayoutJournalEntryID)
	require.Len(t, run.Payouts, 1)
	require.Equal(t, "paid", run.Payouts[0].Status)

	paidPayrollExpenses, err := app.ListExpenseEntries("", true)
	require.NoError(t, err)
	require.Len(t, paidPayrollExpenses, 1)
	require.Equal(t, "payroll", paidPayrollExpenses[0].SourceType)
	require.Equal(t, "paid", paidPayrollExpenses[0].Status)
	require.Equal(t, "paid", paidPayrollExpenses[0].PaymentStatus)
	require.Equal(t, "PAYRUN-001", paidPayrollExpenses[0].PaymentReference)
	require.NotNil(t, paidPayrollExpenses[0].PaidAt)

	projectionAfterPayout, err := app.GetCashFlowProjection(10)
	require.NoError(t, err)
	require.Zero(t, projectionAfterPayout.TotalOutflows)
}

func TestPilotReadinessAndExports_CaptureDeploymentState(t *testing.T) {
	app := setupHybridFeatureTestApp(t)
	seedCurrentAdminContext(t, app, "Rahul")

	readyEmployee := createEmployeeForTest(t, app, "Abhie Admin", "Abhie", "Management", "Administrator")
	missingEmployee := createEmployeeForTest(t, app, "Clinton Ops", "Clinton", "Operations", "Coordinator")

	deviceHash := "machine-ready-001"
	license := seedLicenseRecord(t, app, "PH-ADM-PILOT1", "admin", "Abhie", true, deviceHash)
	user := User{
		Base:     Base{ID: uuid.New().String()},
		Username: "abhie",
		FullName: "Abhie Admin",
		RoleName: "admin",
		IsActive: true,
	}
	require.NoError(t, app.db.Create(&user).Error)

	device := Device{
		Base:          Base{ID: uuid.New().String(), CreatedBy: "test"},
		DeviceName:    "Abhie Mac",
		MachineID:     deviceHash,
		Status:        "approved",
		IsAdminDevice: true,
	}
	now := time.Now()
	device.LastSeenAt = &now
	require.NoError(t, app.db.Create(&device).Error)

	_, err := app.CreateEmployeeAccessLink(EmployeeAccessLink{
		EmployeeID:   readyEmployee.ID,
		LicenseKey:   license.Key,
		UserID:       user.ID,
		DeviceID:     device.ID,
		AccessStatus: "active",
	})
	require.NoError(t, err)

	summary := app.GetPilotReadinessSummary()
	require.GreaterOrEqual(t, summary.TotalEmployees, 3)
	require.GreaterOrEqual(t, summary.ReadyEmployees, 1)
	require.GreaterOrEqual(t, summary.EmployeesWithIssues, 1)
	require.GreaterOrEqual(t, summary.EmployeesMissingAccess, 1)
	require.GreaterOrEqual(t, summary.ActivatedLicenses, 2)
	require.Equal(t, 1, summary.ApprovedDevices)

	rows, err := app.ListPilotReadinessRows(true)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(rows), 1)

	foundMissingEmployee := false
	for _, row := range rows {
		if row.EmployeeID == missingEmployee.ID {
			require.Contains(t, row.Issues, "missing_access_link")
			foundMissingEmployee = true
			break
		}
	}
	require.True(t, foundMissingEmployee, "expected missing-access employee to appear in readiness issues")

	checklist, err := app.UpdatePilotDeploymentChecklistItem("pilot_signoff", true, "Rahul pilot approved")
	require.NoError(t, err)
	require.NotEmpty(t, checklist)

	bundle, err := app.ExportPilotSupportBundle()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(bundle.Path) })
	require.FileExists(t, bundle.Path)
	require.Equal(t, summary.TotalEmployees, bundle.Rows)

	signoff, err := app.ExportPilotSignoffReport()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(signoff.Path) })
	require.FileExists(t, signoff.Path)

	bytes, err := os.ReadFile(signoff.Path)
	require.NoError(t, err)
	text := string(bytes)
	require.Contains(t, text, "Pilot Sign-Off Report")
	require.Contains(t, text, "Rahul pilot approved")
	require.Contains(t, text, missingEmployee.FullName)
	require.Contains(t, text, "missing_access_link")

	require.True(t, strings.HasSuffix(signoff.Path, ".md"))
}
