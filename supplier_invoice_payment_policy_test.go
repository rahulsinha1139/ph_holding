package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func createTestSupplierInvoice(t *testing.T, app *App, supplierID string, supplierName string, invoiceNumber string, invoiceDate time.Time, dueDate time.Time, totalBHD float64) SupplierInvoice {
	t.Helper()

	approvedAt := invoiceDate.Add(2 * time.Hour)
	invoice := SupplierInvoice{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: invoiceDate,
			UpdatedAt: invoiceDate,
			CreatedBy: "creator-user",
		},
		SupplierID:      supplierID,
		SupplierName:    supplierName,
		InvoiceNumber:   invoiceNumber,
		InvoiceDate:     invoiceDate,
		DueDate:         dueDate,
		Currency:        "BHD",
		ExchangeRate:    1,
		SubtotalForeign: totalBHD,
		SubtotalBHD:     totalBHD,
		TotalForeign:    totalBHD,
		TotalBHD:        totalBHD,
		Status:          "Approved",
		ApprovedBy:      "approver-user",
		ApprovedAt:      &approvedAt,
		PaymentStatus:   "Unpaid",
		MatchStatus:     "Matched",
	}
	require.NoError(t, app.db.Create(&invoice).Error)
	return invoice
}

func TestDeleteSupplierPayment_RecalculatesInvoicePaymentState(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierInvoice{}, &SupplierPayment{}))

	supplierID := seedTestSupplier(t, app.db, "Lifecycle Supplier")
	baseDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	invoice := createTestSupplierInvoice(t, app, supplierID, "Lifecycle Supplier", "SI-LIFE-1", baseDate, baseDate.AddDate(0, 0, 30), 100)

	firstPayment, err := app.RecordSupplierPayment(invoice.ID, 40, "BHD", "Bank Transfer", "2026-04-02", "SUP-PMT-1")
	require.NoError(t, err)
	secondPayment, err := app.RecordSupplierPayment(invoice.ID, 60, "BHD", "Bank Transfer", "2026-04-03", "SUP-PMT-2")
	require.NoError(t, err)

	var paidInvoice SupplierInvoice
	require.NoError(t, app.db.First(&paidInvoice, "id = ?", invoice.ID).Error)
	require.Equal(t, "Paid", paidInvoice.PaymentStatus)
	require.Equal(t, "Paid", paidInvoice.Status)

	require.NoError(t, app.DeleteSupplierPayment(secondPayment.ID))

	var stored SupplierInvoice
	require.NoError(t, app.db.First(&stored, "id = ?", invoice.ID).Error)
	require.Equal(t, "Partial", stored.PaymentStatus)
	require.Equal(t, "Approved", stored.Status)
	require.NotNil(t, stored.PaymentDate)
	require.Equal(t, firstPayment.PaymentDate.Format("2006-01-02"), stored.PaymentDate.Format("2006-01-02"))
	require.Equal(t, "SUP-PMT-1", stored.PaymentRef)
	require.Equal(t, "Bank Transfer", stored.PaymentMethod)
}

func TestGetAPAgingReport_UsesOutstandingAfterPartialSupplierPayment(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierInvoice{}, &SupplierPayment{}))

	supplierID := seedTestSupplier(t, app.db, "Aging Supplier")
	invoiceDate := time.Now().AddDate(0, 0, -20)
	dueDate := time.Now().AddDate(0, 0, -10)
	invoice := createTestSupplierInvoice(t, app, supplierID, "Aging Supplier", "SI-AGING-1", invoiceDate, dueDate, 100)

	_, err := app.RecordSupplierPayment(invoice.ID, 40, "BHD", "Bank Transfer", time.Now().AddDate(0, 0, -5).Format("2006-01-02"), "SUP-AGING-1")
	require.NoError(t, err)

	report, err := app.GetAPAgingReport()
	require.NoError(t, err)
	require.Len(t, report.Details, 1)
	require.InDelta(t, 60, report.Total, 0.0001)
	require.InDelta(t, 60, report.Details[0].Amount, 0.0001)
	require.Equal(t, invoice.ID, report.Details[0].InvoiceID)
}

func TestGetSupplierFullProfile_UsesAllOpenInvoiceOutstanding(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierContact{}, &PurchaseOrder{}, &SupplierInvoice{}, &SupplierPayment{}, &SupplierIssue{}))

	supplierID := seedTestSupplier(t, app.db, "Profile Supplier")
	baseDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 6; i++ {
		invoiceDate := baseDate.AddDate(0, 0, i)
		createTestSupplierInvoice(t, app, supplierID, "Profile Supplier", "SI-PROFILE-"+uuid.New().String()[:8], invoiceDate, invoiceDate.AddDate(0, 0, 30), 10)
	}

	profile, err := app.GetSupplierFullProfile(supplierID)
	require.NoError(t, err)
	require.Len(t, profile.RecentInvoices, 5)
	require.InDelta(t, 60, profile.OutstandingBHD, 0.0001)
	require.InDelta(t, 0, profile.OverdueBHD, 0.0001)
}

func TestGetCRMSupplierDashboard_UsesOutstandingAfterPartialPayments(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&PurchaseOrder{}, &SupplierInvoice{}, &SupplierPayment{}))

	supplierID := seedTestSupplier(t, app.db, "Dashboard Supplier")
	invoiceDate := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	invoice := createTestSupplierInvoice(t, app, supplierID, "Dashboard Supplier", "SI-DASH-1", invoiceDate, invoiceDate.AddDate(0, 0, 15), 100)

	_, err := app.RecordSupplierPayment(invoice.ID, 35, "BHD", "Bank Transfer", "2026-04-05", "SUP-DASH-1")
	require.NoError(t, err)

	dashboard := app.GetCRMSupplierDashboard()
	require.InDelta(t, 65, dashboard.OutstandingPayables, 0.0001)

	require.NotEmpty(t, dashboard.Suppliers)
	var supplierCard *SupplierMetricCard
	for i := range dashboard.Suppliers {
		if dashboard.Suppliers[i].ID == supplierID {
			supplierCard = &dashboard.Suppliers[i]
			break
		}
	}
	require.NotNil(t, supplierCard)
	require.InDelta(t, 65, supplierCard.OutstandingBHD, 0.0001)
}

func TestGetCashFlowProjection_UsesOutstandingSupplierBalance(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Invoice{}, &SupplierInvoice{}, &SupplierPayment{}, &BankAccount{}, &ExpenseEntry{}, &RecurringExpense{}))

	supplierID := seedTestSupplier(t, app.db, "Projection Supplier")
	now := time.Now()
	invoice := createTestSupplierInvoice(t, app, supplierID, "Projection Supplier", "SI-CFLOW-1", now.AddDate(0, 0, -2), now.AddDate(0, 0, 7), 120)

	_, err := app.RecordSupplierPayment(invoice.ID, 45, "BHD", "Bank Transfer", now.AddDate(0, 0, -1).Format("2006-01-02"), "SUP-CFLOW-1")
	require.NoError(t, err)

	projection, err := app.GetCashFlowProjection(14)
	require.NoError(t, err)
	require.InDelta(t, 75, projection.TotalOutflows, 0.0001)
}
