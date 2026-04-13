package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func migrateCustomerReceiptReconciliationTables(t *testing.T, app *App) {
	t.Helper()
	require.NoError(t, app.db.AutoMigrate(
		&BankStatement{},
		&BankStatementLine{},
		&BankReconciliationAuditLog{},
	))
}

func createCustomerReceiptTestInvoice(t *testing.T, app *App, customerID string, customerName string, invoiceNumber string, total float64, outstanding float64, status string, dueDate time.Time) Invoice {
	t.Helper()

	invoice := Invoice{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		InvoiceNumber:  invoiceNumber,
		InvoiceDate:    dueDate.AddDate(0, 0, -30),
		DueDate:        dueDate,
		CustomerID:     customerID,
		CustomerName:   customerName,
		GrandTotalBHD:  total,
		OutstandingBHD: outstanding,
		Status:         status,
	}

	require.NoError(t, app.db.Create(&invoice).Error)
	return invoice
}

func TestListCustomerInvoices_HydratesSettlementState(t *testing.T) {
	app := setupTestApp(t)
	customerID := seedTestCustomer(t, app.db, "Hydration Trading")

	invoice := createCustomerReceiptTestInvoice(
		t,
		app,
		customerID,
		"Hydration Trading",
		"INV-HYDRATE-001",
		100,
		35,
		"Paid",
		time.Now().AddDate(0, 0, 7),
	)

	invoices, err := app.ListCustomerInvoices(50, 0)
	require.NoError(t, err)
	require.NotEmpty(t, invoices)

	var found Invoice
	foundOK := false
	for _, candidate := range invoices {
		if candidate.ID == invoice.ID {
			found = candidate
			foundOK = true
			break
		}
	}
	require.True(t, foundOK)
	assert.Equal(t, "PartiallyPaid", found.Status)
	assert.Equal(t, 35.0, found.OutstandingBHD)
}

func TestGetOverdueInvoices_UsesNormalizedOpenState(t *testing.T) {
	app := setupTestApp(t)
	customerID := seedTestCustomer(t, app.db, "Overdue Receivables Co")

	openInvoice := createCustomerReceiptTestInvoice(
		t,
		app,
		customerID,
		"Overdue Receivables Co",
		"INV-OD-OPEN",
		120,
		20,
		"Paid",
		time.Now().AddDate(0, 0, -10),
	)
	createCustomerReceiptTestInvoice(
		t,
		app,
		customerID,
		"Overdue Receivables Co",
		"INV-OD-CANCEL",
		75,
		75,
		"Cancelled",
		time.Now().AddDate(0, 0, -12),
	)

	invoices, err := app.GetOverdueInvoices()
	require.NoError(t, err)
	require.Len(t, invoices, 1)
	assert.Equal(t, openInvoice.ID, invoices[0].ID)
	assert.Equal(t, "PartiallyPaid", invoices[0].Status)
}

func TestMatchToCustomerInvoice_UsesOutstandingBalance(t *testing.T) {
	app := setupTestApp(t)
	customerID := seedTestCustomer(t, app.db, "Receipt Matching WLL")
	invoice := createCustomerReceiptTestInvoice(
		t,
		app,
		customerID,
		"Receipt Matching WLL",
		"INV-MATCH-001",
		100,
		40,
		"Paid",
		time.Now().AddDate(0, 0, 14),
	)

	line := &BankStatementLine{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Credit:      40,
		Description: "Receipt from Receipt Matching WLL",
	}

	matched, matchedID, confidence := app.matchToCustomerInvoice(line, nil, "Receipt Matching WLL")
	require.True(t, matched)
	assert.Equal(t, invoice.ID, matchedID)
	assert.GreaterOrEqual(t, confidence, 0.60)
}

func TestManualMatchLine_UsesOutstandingBalanceForCustomerInvoice(t *testing.T) {
	app := setupTestApp(t)
	migrateCustomerReceiptReconciliationTables(t, app)

	customerID := seedTestCustomer(t, app.db, "Manual Match Co")
	invoice := createCustomerReceiptTestInvoice(
		t,
		app,
		customerID,
		"Manual Match Co",
		"INV-MANUAL-001",
		100,
		40,
		"Paid",
		time.Now().AddDate(0, 0, 10),
	)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-customer-1",
		StatementNumber: "STMT-CUST-001",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -7),
		PeriodEnd:       time.Now(),
		Status:          bankStatementStatusImported,
	}
	require.NoError(t, app.db.Create(&statement).Error)

	line := BankStatementLine{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankStatementID: statement.ID,
		LineNumber:      1,
		TransactionDate: time.Now(),
		ValueDate:       time.Now(),
		Description:     "Customer receipt",
		Credit:          40,
		Balance:         400,
		MatchType:       "Unmatched",
	}
	require.NoError(t, app.db.Create(&line).Error)

	require.NoError(t, app.ManualMatchLine(line.ID, "CUSTOMER_INVOICE", invoice.ID, "admin"))

	var updated BankStatementLine
	require.NoError(t, app.db.First(&updated, "id = ?", line.ID).Error)
	assert.True(t, updated.IsMatched)
	assert.Equal(t, invoice.ID, updated.MatchedInvoiceIDs)
}
