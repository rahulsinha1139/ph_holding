package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func seedActivatedLicense(t *testing.T, app *App, role string, displayName string) {
	t.Helper()

	require.NoError(t, app.db.AutoMigrate(&LicenseKey{}))

	now := time.Now()
	license := LicenseKey{
		Key:         "PH-SLS-ALIGN1",
		Role:        role,
		DisplayName: displayName,
		DeviceHash:  app.getDeviceHash(),
		Activated:   true,
		ActivatedAt: &now,
		CreatedBy:   "test",
	}

	require.NoError(t, app.db.Create(&license).Error)
}

func TestSeedDefaultRoles_AllowsStartupSystemContext(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Role{}, &LicenseKey{}))

	app.currentUser = nil
	app.currentUserID = ""

	require.NoError(t, app.SeedDefaultRoles())

	var salesRole Role
	require.NoError(t, app.db.Where("name = ?", "sales").First(&salesRole).Error)
	require.Contains(t, salesRole.Permissions, "suppliers:create")
	require.Contains(t, salesRole.Permissions, "po:view")

	var operationsRole Role
	require.NoError(t, app.db.Where("name = ?", "operations").First(&operationsRole).Error)
	require.Contains(t, operationsRole.Permissions, "po:create")
	require.Contains(t, operationsRole.Permissions, "delivery_notes:view")
}

func TestRequirePermission_FallsBackToLicenseWhenUserRoleIsStale(t *testing.T) {
	app := setupTestApp(t)
	seedActivatedLicense(t, app, "sales", "Ebin")

	app.currentUser = &User{
		Base:     Base{ID: "sales-user"},
		Username: "ebin",
		RoleName: "sales",
		Role: Role{
			Name:        "sales",
			DisplayName: "Sales",
			Permissions: `["orders:view"]`,
		},
	}
	app.currentUserID = "sales-user"

	require.NoError(t, app.requirePermission("suppliers:view"))
	require.NoError(t, app.requirePermission("suppliers:create"))
	require.NoError(t, app.requirePermission("po:view"))
	require.NoError(t, app.requirePermission("orders:update"))
	require.Error(t, app.requirePermission("finance:view"))
}

func TestGetUserPermissions_MergesCurrentUserRoleWithLicensePermissions(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Role{}, &User{}, &LicenseKey{}))

	role := Role{
		Base:        Base{ID: uuid.New().String()},
		Name:        "sales",
		DisplayName: "Sales",
		Permissions: `["orders:view"]`,
		IsActive:    true,
	}
	require.NoError(t, app.db.Create(&role).Error)

	user := User{
		Base:     Base{ID: uuid.New().String()},
		Username: "ebin",
		Email:    "ebin@example.com",
		RoleID:   role.ID,
		FullName: "Ebin",
		IsActive: true,
	}
	require.NoError(t, app.db.Create(&user).Error)

	app.currentUserID = user.ID
	app.currentUser = nil
	seedActivatedLicense(t, app, "sales", "Ebin")

	perms, err := app.GetUserPermissions(user.ID)
	require.NoError(t, err)
	require.Contains(t, perms, "orders:view")
	require.Contains(t, perms, "suppliers:view")
	require.Contains(t, perms, "suppliers:create")
	require.Contains(t, perms, "po:view")
}

func TestUpdateRFQNotes_RequiresOffersEditPermission(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&RFQData{}))

	rfq := RFQData{
		RFQNumber: "RFQ-2026-900",
		Client:    "BAPCO",
		Project:   "Permission guard",
		Value:     1000,
		Notes:     "keep me",
		Status:    "New",
		Stage:     "RFQ Received",
	}
	require.NoError(t, app.db.Create(&rfq).Error)

	app.currentUser = &User{
		Base:     Base{ID: "sales-user"},
		Username: "ebin",
		RoleName: "sales",
		Role: Role{
			Name:        "sales",
			DisplayName: "Sales",
			Permissions: `["offers:view"]`,
		},
	}

	_, err := app.UpdateRFQNotes(rfq.ID, "blocked")
	require.Error(t, err)
	require.Contains(t, err.Error(), "offers:edit")
}

func TestUpdateSupplierInvoiceWithPayment_RequiresPOCreatePermission(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierInvoice{}))

	invoice := SupplierInvoice{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		InvoiceNumber:   "SUPP-2026-901",
		InvoiceDate:     time.Now(),
		DueDate:         time.Now().AddDate(0, 0, 30),
		Currency:        "BHD",
		ExchangeRate:    1,
		SubtotalForeign: 10,
		SubtotalBHD:     10,
		TotalForeign:    10,
		TotalBHD:        10,
		Status:          "Approved",
		PaymentStatus:   "Unpaid",
		MatchStatus:     "Pending",
	}
	require.NoError(t, app.db.Create(&invoice).Error)

	app.currentUser = &User{
		Base:     Base{ID: "sales-user"},
		Username: "ebin",
		RoleName: "sales",
		Role: Role{
			Name:        "sales",
			DisplayName: "Sales",
			Permissions: `["po:view"]`,
		},
	}

	_, err := app.UpdateSupplierInvoiceWithPayment(invoice, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "po:create")
}

func TestManualMatchLine_RequiresFinanceCreatePermission(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&BankStatement{}, &BankStatementLine{}))

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-2026-RBAC",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -3),
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
		Description:     "Blocked match",
		Debit:           10,
	}
	require.NoError(t, app.db.Create(&line).Error)

	app.currentUser = &User{
		Base:     Base{ID: "ops-user"},
		Username: "peri",
		RoleName: "operations",
		Role: Role{
			Name:        "operations",
			DisplayName: "Operations",
			Permissions: `["finance:view"]`,
		},
	}

	err := app.ManualMatchLine(line.ID, "SUPPLIER_PAYMENT", "payment-1", "peri")
	require.Error(t, err)
	require.Contains(t, err.Error(), "finance:create")
}

func TestSeedDefaultRoles_HybridFeaturePermissionsAreAssignedByRole(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Role{}))
	require.NoError(t, app.SeedDefaultRoles())

	var managerRole Role
	require.NoError(t, app.db.Where("name = ?", "manager").First(&managerRole).Error)
	require.Contains(t, managerRole.Permissions, "tasks:view")
	require.Contains(t, managerRole.Permissions, "projects:view")
	require.Contains(t, managerRole.Permissions, "notifications:view")
	require.Contains(t, managerRole.Permissions, "expenses:view")
	require.Contains(t, managerRole.Permissions, "payroll:view")
	require.Contains(t, managerRole.Permissions, "hr:view")

	var salesRole Role
	require.NoError(t, app.db.Where("name = ?", "sales").First(&salesRole).Error)
	require.Contains(t, salesRole.Permissions, "tasks:view")
	require.Contains(t, salesRole.Permissions, "projects:view")
	require.Contains(t, salesRole.Permissions, "notifications:view")
	require.NotContains(t, salesRole.Permissions, "expenses:view")
	require.NotContains(t, salesRole.Permissions, "payroll:view")
	require.NotContains(t, salesRole.Permissions, "hr:view")

	var operationsRole Role
	require.NoError(t, app.db.Where("name = ?", "operations").First(&operationsRole).Error)
	require.Contains(t, operationsRole.Permissions, "tasks:view")
	require.Contains(t, operationsRole.Permissions, "projects:view")
	require.Contains(t, operationsRole.Permissions, "notifications:view")
	require.NotContains(t, operationsRole.Permissions, "expenses:view")
	require.NotContains(t, operationsRole.Permissions, "payroll:view")

	var staffRole Role
	require.NoError(t, app.db.Where("name = ?", "staff").First(&staffRole).Error)
	require.Contains(t, staffRole.Permissions, "tasks:view")
	require.Contains(t, staffRole.Permissions, "projects:view")
	require.Contains(t, staffRole.Permissions, "notifications:view")
	require.NotContains(t, staffRole.Permissions, "expenses:view")
	require.NotContains(t, staffRole.Permissions, "payroll:view")
}
