package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func makeDashboardTestApp(t *testing.T) *App {
	t.Helper()

	app := setupTestApp(t)
	app.startupImporting = false
	app.currentUser = &User{
		Base:     Base{ID: "dashboard-test-user"},
		Username: "dashboard-admin",
		RoleName: "admin",
		Role: Role{
			Name:        "admin",
			DisplayName: "Admin",
			Permissions: `["dashboard:view"]`,
		},
	}
	return app
}

func TestDashboardStats_PrefersConfirmedOrdersOverDraftInvoices(t *testing.T) {
	app := makeDashboardTestApp(t)

	orderDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	dueDate := orderDate.AddDate(0, 0, 30)

	order := Order{
		Base:          Base{ID: uuid.New().String(), CreatedAt: orderDate, UpdatedAt: orderDate},
		OrderNumber:   "ORD-2026-001",
		CustomerID:    uuid.New().String(),
		CustomerName:  "PH Test Customer",
		OrderDate:     orderDate,
		RequiredDate:  dueDate,
		GrandTotalBHD: 116128.811,
		Status:        "Confirmed",
	}
	require.NoError(t, app.db.Create(&order).Error)

	draftInvoice := Invoice{
		Base:           Base{ID: uuid.New().String(), CreatedAt: orderDate, UpdatedAt: orderDate},
		InvoiceNumber:  "PH2610-2026",
		InvoiceDate:    orderDate,
		CustomerID:     order.CustomerID,
		CustomerName:   order.CustomerName,
		OrderID:        order.ID,
		GrandTotalBHD:  505.12,
		Status:         "Draft",
		OutstandingBHD: 505.12,
		SubtotalBHD:    459.20,
		DueDate:        dueDate,
	}
	require.NoError(t, app.db.Create(&draftInvoice).Error)

	stats, err := app.GetDashboardStats()
	require.NoError(t, err)
	require.Equal(t, 2026, stats.ActivityYear)
	require.Equal(t, 116128.811, stats.TotalRevenue)
	require.Equal(t, "Confirmed orders YTD 2026", stats.RevenueMeta)
	require.Equal(t, 0, stats.PendingInvoices)
	require.Equal(t, 0.0, stats.OutstandingAR)
}

func TestDashboardCharts_UseDashboardPermissionAndYTD2026Data(t *testing.T) {
	app := makeDashboardTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Opportunity{}))

	customerID := seedTestCustomer(t, app.db, "Dashboard YTD Customer")
	invoiceDate := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	oldInvoiceDate := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	dueDate := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

	require.NoError(t, app.db.Create(&Invoice{
		Base:           Base{ID: uuid.New().String(), CreatedAt: invoiceDate, UpdatedAt: invoiceDate},
		InvoiceNumber:  "INV-2026-YTD-001",
		InvoiceDate:    invoiceDate,
		CustomerID:     customerID,
		CustomerName:   "Dashboard YTD Customer",
		GrandTotalBHD:  500,
		OutstandingBHD: 0,
		SubtotalBHD:    500,
		Status:         "Paid",
		DueDate:        dueDate,
	}).Error)
	require.NoError(t, app.db.Create(&Invoice{
		Base:           Base{ID: uuid.New().String(), CreatedAt: invoiceDate, UpdatedAt: invoiceDate},
		InvoiceNumber:  "INV-2026-YTD-002",
		InvoiceDate:    invoiceDate,
		CustomerID:     customerID,
		CustomerName:   "Dashboard YTD Customer",
		GrandTotalBHD:  400,
		OutstandingBHD: 400,
		SubtotalBHD:    400,
		Status:         "Sent",
		DueDate:        dueDate,
	}).Error)
	require.NoError(t, app.db.Create(&Invoice{
		Base:           Base{ID: uuid.New().String(), CreatedAt: oldInvoiceDate, UpdatedAt: oldInvoiceDate},
		InvoiceNumber:  "INV-2025-OLD-001",
		InvoiceDate:    oldInvoiceDate,
		CustomerID:     customerID,
		CustomerName:   "Dashboard YTD Customer",
		GrandTotalBHD:  900,
		OutstandingBHD: 900,
		SubtotalBHD:    900,
		Status:         "Sent",
		DueDate:        oldInvoiceDate.AddDate(0, 0, 30),
	}).Error)

	require.NoError(t, app.db.Create(&Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: invoiceDate, UpdatedAt: invoiceDate},
		FolderNumber: "2026-DASH-001",
		Year:         2026,
		Stage:        "Qualified",
		OfferDate:    invoiceDate,
		RevenueBHD:   1250,
	}).Error)
	require.NoError(t, app.db.Create(&Opportunity{
		Base:         Base{ID: uuid.New().String(), CreatedAt: oldInvoiceDate, UpdatedAt: oldInvoiceDate},
		FolderNumber: "2025-DASH-001",
		Year:         2025,
		Stage:        "Won",
		OfferDate:    oldInvoiceDate,
		RevenueBHD:   9999,
	}).Error)

	revenueRows, err := app.GetMonthlyRevenueByCustomer()
	require.NoError(t, err)
	require.Len(t, revenueRows, 1)
	require.Equal(t, 900.0, revenueRows[0].Revenue)
	require.Equal(t, 2, revenueRows[0].InvoiceCount)
	require.Equal(t, 2026, revenueRows[0].Year)

	pipelineRows, err := app.GetDashboardPipelineByStageYTD()
	require.NoError(t, err)
	require.Len(t, pipelineRows, 1)
	require.Equal(t, "Qualified", pipelineRows[0].Stage)
	require.Equal(t, 1, pipelineRows[0].Count)
	require.Equal(t, 1250.0, pipelineRows[0].Value)

	aging, err := app.GetDashboardARAgingReportYTD()
	require.NoError(t, err)
	require.Equal(t, 400.0, aging.Total)
	require.Empty(t, aging.Details)
}

func TestDashboardRevenueByCustomerFallsBackToLatestPostedInvoiceYear(t *testing.T) {
	app := makeDashboardTestApp(t)

	customerID := seedTestCustomer(t, app.db, "Dashboard Historical Customer")
	invoiceDate := time.Date(2025, 8, 15, 0, 0, 0, 0, time.UTC)
	require.NoError(t, app.db.Create(&Invoice{
		Base:           Base{ID: uuid.New().String(), CreatedAt: invoiceDate, UpdatedAt: invoiceDate},
		InvoiceNumber:  "INV-2025-FALLBACK-001",
		InvoiceDate:    invoiceDate,
		CustomerID:     customerID,
		CustomerName:   "Dashboard Historical Customer",
		GrandTotalBHD:  1234,
		OutstandingBHD: 0,
		SubtotalBHD:    1234,
		Status:         "Paid",
		DueDate:        invoiceDate.AddDate(0, 0, 30),
	}).Error)

	revenueRows, err := app.GetMonthlyRevenueByCustomer()
	require.NoError(t, err)
	require.Len(t, revenueRows, 1)
	require.Equal(t, 1234.0, revenueRows[0].Revenue)
	require.Equal(t, 1, revenueRows[0].InvoiceCount)
	require.Equal(t, 2025, revenueRows[0].Year)
}
