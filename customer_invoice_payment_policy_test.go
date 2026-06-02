package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createPaymentStateTestCustomer(t *testing.T, app *App, name string) *CustomerMaster {
	t.Helper()

	customer := &CustomerMaster{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		CustomerID:   "CUST-" + uuid.New().String()[:8],
		CustomerCode: "CC-" + uuid.New().String()[:8],
		CustomerType: "End Customer",
		BusinessName: name,
		Status:       "Active",
	}

	require.NoError(t, app.db.Create(customer).Error)
	return customer
}

func createPaymentStateTestInvoice(t *testing.T, app *App, customer *CustomerMaster, invoiceNumber string, invoiceDate time.Time, dueDate time.Time, total float64, outstanding float64, status string) *Invoice {
	t.Helper()

	invoice := &Invoice{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		InvoiceNumber:  invoiceNumber,
		InvoiceDate:    invoiceDate,
		DueDate:        dueDate,
		CustomerID:     customer.ID,
		CustomerName:   customer.BusinessName,
		GrandTotalBHD:  total,
		OutstandingBHD: outstanding,
		Status:         status,
	}

	require.NoError(t, app.db.Create(invoice).Error)
	return invoice
}

func TestDeletePayment_RestoresOverdueStatusOnLastPayment(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Overdue Repair Co")
	invoice := createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-PAY-DELETE-001",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		100,
		100,
		"Sent",
	)

	payment, err := app.RecordPayment(invoice.ID, 100, "Cash", "2026-02-03", "DEL-REF")
	require.NoError(t, err)

	require.NoError(t, app.DeletePayment(payment.ID))

	var updated Invoice
	require.NoError(t, app.db.First(&updated, "id = ?", invoice.ID).Error)
	assert.Equal(t, 100.0, updated.OutstandingBHD)
	assert.Equal(t, "Overdue", updated.Status)
}

func TestUpdatePayment_RestoresPartialStatusFromPaidInvoice(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Update Drift Co")
	invoice := createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-PAY-UPD-001",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		100,
		100,
		"Sent",
	)

	payment, err := app.RecordPayment(invoice.ID, 100, "Cash", "2026-01-15", "UPD-REF")
	require.NoError(t, err)

	updatedPayment, err := app.UpdatePayment(payment.ID, Payment{
		AmountBHD:     60,
		PaymentMethod: "Cash",
		PaymentDate:   payment.PaymentDate,
		Reference:     payment.Reference,
	})
	require.NoError(t, err)
	require.NotNil(t, updatedPayment)

	var updated Invoice
	require.NoError(t, app.db.First(&updated, "id = ?", invoice.ID).Error)
	assert.Equal(t, 40.0, updated.OutstandingBHD)
	assert.Equal(t, "PartiallyPaid", updated.Status)
}

func TestGetPaymentHistory_JoinsInvoicesThroughCustomer(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "History Join Co")
	invoice := createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-HIST-001",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		100,
		100,
		"Sent",
	)

	_, err := app.RecordPayment(invoice.ID, 25, "Cash", "2026-01-10", "HIST-REF")
	require.NoError(t, err)

	history := app.GetPaymentHistory(customer.ID, 10)
	require.Len(t, history, 1)
	assert.Equal(t, "INV-HIST-001", history[0].InvoiceNumber)
	assert.Equal(t, 25.0, history[0].AmountBHD)
}

func TestGetARAgingReport_UsesNormalizedOpenCustomerInvoiceState(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "AR Drift Co")

	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-AR-OPEN",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		100,
		20,
		"Paid",
	)
	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-AR-CANCELLED",
		time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC),
		80,
		80,
		"Cancelled",
	)

	report, err := app.GetARAgingReport()
	require.NoError(t, err)
	require.Len(t, report.Details, 1)
	assert.Equal(t, "INV-AR-OPEN", report.Details[0].InvoiceNum)
	assert.Equal(t, 20.0, report.Total)
}

func TestGetCRMCustomerDashboardByYear_UsesNormalizedOutstanding(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Dashboard Drift Co")

	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-CRM-OPEN",
		time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC),
		100,
		30,
		"Paid",
	)
	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-CRM-CANCELLED",
		time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 6, 0, 0, 0, 0, time.UTC),
		90,
		90,
		"Cancelled",
	)

	dashboard := app.GetCRMCustomerDashboardByYear(2026)
	require.NotEmpty(t, dashboard.Customers)
	assert.Equal(t, 30.0, dashboard.TotalOutstanding)
	assert.Equal(t, 1, dashboard.Customers[0].ActiveInvoices)
	assert.Equal(t, 30.0, dashboard.Customers[0].OutstandingBHD)
}

func TestGetCashFlowProjection_UsesNormalizedOpenCustomerInvoices(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Cashflow Drift Co")

	invoiceDate := time.Now().AddDate(0, 0, -7)
	dueDate := time.Now().AddDate(0, 0, 3)
	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-CF-OPEN",
		invoiceDate,
		dueDate,
		120,
		40,
		"Paid",
	)

	projection, err := app.GetCashFlowProjection(14)
	require.NoError(t, err)
	assert.InDelta(t, 40.0, projection.TotalInflows, 0.001)
}

func TestUpdateCustomerInvoice_RejectsManualSettlementStatusEdit(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Workflow Lock Co")
	invoice := createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-WORKFLOW-001",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		100,
		25,
		"PartiallyPaid",
	)

	_, err := app.UpdateCustomerInvoice(Invoice{
		Base:             Base{ID: invoice.ID},
		Status:           "Sent",
		GrandTotalBHD:    invoice.GrandTotalBHD,
		OutstandingBHD:   invoice.OutstandingBHD,
		CustomerPONumber: "PO-LOCK",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payment-driven")
}

func TestUpdateCustomerInvoice_RejectsManualOutstandingEdit(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Outstanding Lock Co")
	invoice := createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-WORKFLOW-002",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		100,
		100,
		"Sent",
	)

	_, err := app.UpdateCustomerInvoice(Invoice{
		Base:             Base{ID: invoice.ID},
		Status:           "Sent",
		GrandTotalBHD:    invoice.GrandTotalBHD,
		OutstandingBHD:   10,
		CustomerPONumber: "PO-LOCK",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "payment-driven")
}

func TestMarkCustomerInvoicePaid_CreatesLedgerEntryForRemainingBalance(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Mark Paid Co")
	invoice := createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-WORKFLOW-003",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		100,
		40,
		"Paid",
	)

	err := app.MarkCustomerInvoicePaid(invoice.ID, time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC), "MARK-REF")
	require.NoError(t, err)

	var updated Invoice
	require.NoError(t, app.db.First(&updated, "id = ?", invoice.ID).Error)
	assert.Equal(t, 0.0, updated.OutstandingBHD)
	assert.Equal(t, "Paid", updated.Status)

	var payments []Payment
	require.NoError(t, app.db.Where("invoice_id = ?", invoice.ID).Find(&payments).Error)
	require.Len(t, payments, 1)
	assert.Equal(t, 40.0, payments[0].AmountBHD)
	assert.Equal(t, "MARK-REF", payments[0].Reference)
}

func TestGetInvoicesByStatus_UsesNormalizedSettlementState(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Status Query Co")
	invoice := createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-WORKFLOW-004",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		100,
		20,
		"Paid",
	)

	invoices, err := app.GetInvoicesByStatus("PartiallyPaid")
	require.NoError(t, err)
	require.Len(t, invoices, 1)
	assert.Equal(t, invoice.ID, invoices[0].ID)
	assert.Equal(t, "PartiallyPaid", invoices[0].Status)
}

func TestGetRevenueSummary_UsesNormalizedSettlementState(t *testing.T) {
	app := setupPaymentTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Revenue Summary Co")

	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-REV-001",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		100,
		30,
		"Paid",
	)
	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-REV-002",
		time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 2, 0, 0, 0, 0, time.UTC),
		70,
		0,
		"Paid",
	)
	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-REV-003",
		time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC),
		50,
		0,
		"Cancelled",
	)

	totalRevenue, paidRevenue, outstandingRevenue, invoiceCount, err := app.GetInvoiceRevenueSummary()
	require.NoError(t, err)
	assert.InDelta(t, 170.0, totalRevenue, 0.001)
	assert.InDelta(t, 70.0, paidRevenue, 0.001)
	assert.InDelta(t, 30.0, outstandingRevenue, 0.001)
	assert.Equal(t, int64(3), invoiceCount)
}

func TestGetCustomerFullProfile_UsesNormalizedRevenueAndRecentInvoiceOutstanding(t *testing.T) {
	app := setupTestApp(t)
	customer := createPaymentStateTestCustomer(t, app, "Profile Revenue Co")

	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-PROFILE-001",
		time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
		120,
		25,
		"Paid",
	)
	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-PROFILE-002",
		time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 5, 0, 0, 0, 0, time.UTC),
		80,
		0,
		"Paid",
	)
	createPaymentStateTestInvoice(
		t,
		app,
		customer,
		"INV-PROFILE-003",
		time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 2, 7, 0, 0, 0, 0, time.UTC),
		50,
		0,
		"Proforma",
	)

	profile, err := app.GetCustomerFullProfile(customer.ID)
	require.NoError(t, err)
	assert.InDelta(t, 200.0, profile.TotalRevenue, 0.001)
	assert.InDelta(t, 25.0, profile.OutstandingBHD, 0.001)
	require.Len(t, profile.RecentInvoices, 3)
	assert.Equal(t, "PartiallyPaid", profile.RecentInvoices[2].Status)
	assert.InDelta(t, 25.0, profile.RecentInvoices[2].OutstandingBHD, 0.001)
	assert.Equal(t, "Paid", profile.RecentInvoices[1].Status)
	assert.InDelta(t, 0.0, profile.RecentInvoices[1].OutstandingBHD, 0.001)
}
