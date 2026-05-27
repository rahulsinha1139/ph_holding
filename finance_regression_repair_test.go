package main

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func migrateFinanceRegressionRepairTables(t *testing.T, app *App) {
	t.Helper()
	require.NoError(t, app.db.AutoMigrate(
		&CustomerReceipt{},
		&CustomerReceiptAllocation{},
		&BankStatement{},
		&BankStatementLine{},
		&BankLinePaymentAllocation{},
		&BankReconciliationAuditLog{},
		&SupplierInvoice{},
		&SupplierPayment{},
		&ExpenseCategory{},
		&ExpenseVendor{},
		&ExpenseEntry{},
	))
}

func seedFinanceRepairInvoice(t *testing.T, app *App, customerID, customerName, number string, amount float64, invoiceDate time.Time) Invoice {
	t.Helper()
	invoice := Invoice{
		Base:                 Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		InvoiceNumber:        number,
		InvoiceDate:          invoiceDate,
		DueDate:              invoiceDate.AddDate(0, 0, 30),
		CustomerID:           customerID,
		CustomerName:         customerName,
		SubtotalBHD:          amount,
		GrandTotalBHD:        amount,
		OutstandingBHD:       amount,
		TotalSupplierCostBHD: roundBHD(amount * 0.6),
		Status:               "Sent",
		Division:             "PH Trading",
	}
	require.NoError(t, app.db.Create(&invoice).Error)
	return invoice
}

func seedFinanceRepairStatement(t *testing.T, app *App, statementNumber string) BankStatement {
	t.Helper()
	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "bank-finance-repair",
		StatementNumber: statementNumber,
		StatementDate:   time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		PeriodStart:     time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:       time.Date(2026, time.May, 31, 0, 0, 0, 0, time.UTC),
		Division:        "PH Trading",
		Status:          bankStatementStatusImported,
	}
	require.NoError(t, app.db.Create(&statement).Error)
	return statement
}

func seedFinanceRepairCreditLine(t *testing.T, app *App, statementID string, lineNumber int, amount float64) BankStatementLine {
	t.Helper()
	line := BankStatementLine{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankStatementID: statementID,
		LineNumber:      lineNumber,
		TransactionDate: time.Date(2026, time.May, lineNumber, 0, 0, 0, 0, time.UTC),
		ValueDate:       time.Date(2026, time.May, lineNumber, 0, 0, 0, 0, time.UTC),
		Description:     "Customer receipt regression",
		Credit:          amount,
		Balance:         amount,
		MatchType:       "Unmatched",
	}
	require.NoError(t, app.db.Create(&line).Error)
	return line
}

func TestCustomerReceiptOnAccountCanAllocateAndAppearsInCustomerLedger(t *testing.T) {
	app := setupTestApp(t)
	migrateFinanceRegressionRepairTables(t, app)
	customerName := "Receipt Repair WLL"
	customerID := seedTestCustomer(t, app.db, customerName)
	invoice := seedFinanceRepairInvoice(t, app, customerID, customerName, "INV-RCT-001", 110, time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC))

	receipt, err := app.CreateCustomerReceipt(CustomerReceiptInput{
		CustomerID:    customerID,
		CustomerName:  customerName,
		AmountBHD:     75,
		ReceiptDate:   "2026-05-10",
		PaymentMethod: "Cash",
		Division:      "PH Trading",
		Notes:         "Advance collected on account",
	})
	require.NoError(t, err)
	require.Equal(t, "OnAccount", receipt.Status)
	require.Equal(t, 75.0, receipt.UnappliedAmountBHD)

	allocation, err := app.ApplyCustomerReceiptToInvoice(receipt.ID, invoice.ID, 25)
	require.NoError(t, err)
	require.Equal(t, 25.0, allocation.AllocatedAmountBHD)

	var updatedReceipt CustomerReceipt
	require.NoError(t, app.db.First(&updatedReceipt, "id = ?", receipt.ID).Error)
	require.Equal(t, "PartiallyApplied", updatedReceipt.Status)
	require.Equal(t, 25.0, updatedReceipt.AppliedAmountBHD)
	require.Equal(t, 50.0, updatedReceipt.UnappliedAmountBHD)

	var updatedInvoice Invoice
	require.NoError(t, app.db.First(&updatedInvoice, "id = ?", invoice.ID).Error)
	require.Equal(t, 85.0, updatedInvoice.OutstandingBHD)
	require.Equal(t, "PartiallyPaid", updatedInvoice.Status)

	var payment Payment
	require.NoError(t, app.db.First(&payment, "receipt_id = ?", receipt.ID).Error)
	require.Equal(t, invoice.ID, payment.InvoiceID)
	require.Equal(t, 25.0, payment.AmountBHD)

	ledger, err := app.GetCustomerLedger(customerID, "2026-05-01", "2026-05-31")
	require.NoError(t, err)
	require.Len(t, ledger, 2)
	require.Equal(t, "CUSTOMER_INVOICE", ledger[0].SourceType)
	require.Equal(t, "CUSTOMER_RECEIPT", ledger[1].SourceType)
	require.Equal(t, 35.0, ledger[1].BalanceBHD)
}

func TestCustomerReceiptInvoiceModeCreatesAppliedReceiptAndPayment(t *testing.T) {
	app := setupTestApp(t)
	migrateFinanceRegressionRepairTables(t, app)
	customerName := "Invoice Receipt Repair WLL"
	customerID := seedTestCustomer(t, app.db, customerName)
	invoice := seedFinanceRepairInvoice(t, app, customerID, customerName, "INV-RCT-002", 50, time.Date(2026, time.May, 2, 0, 0, 0, 0, time.UTC))

	receipt, err := app.CreateCustomerReceipt(CustomerReceiptInput{
		InvoiceID:     invoice.ID,
		AmountBHD:     50,
		ReceiptDate:   "2026-05-11",
		PaymentMethod: "Bank Transfer",
		Reference:     "BANK-RCT-50",
	})
	require.NoError(t, err)
	require.Equal(t, "Applied", receipt.Status)
	require.Equal(t, 50.0, receipt.AppliedAmountBHD)
	require.Zero(t, receipt.UnappliedAmountBHD)

	var payment Payment
	require.NoError(t, app.db.First(&payment, "receipt_id = ?", receipt.ID).Error)
	require.Equal(t, invoice.ID, payment.InvoiceID)

	var updatedInvoice Invoice
	require.NoError(t, app.db.First(&updatedInvoice, "id = ?", invoice.ID).Error)
	require.Equal(t, "Paid", updatedInvoice.Status)
	require.Zero(t, updatedInvoice.OutstandingBHD)
}

func TestOperationalProfitLossUsesValidInvoicesAndApprovedExpensesInDateRange(t *testing.T) {
	app := setupTestApp(t)
	migrateFinanceRegressionRepairTables(t, app)
	customerID := seedTestCustomer(t, app.db, "P&L Customer WLL")
	seedFinanceRepairInvoice(t, app, customerID, "P&L Customer WLL", "INV-PL-001", 1000, time.Date(2026, time.February, 12, 0, 0, 0, 0, time.UTC))
	seedFinanceRepairInvoice(t, app, customerID, "P&L Customer WLL", "INV-PL-OLD", 500, time.Date(2026, time.January, 12, 0, 0, 0, 0, time.UTC))

	categoryID := uuid.New().String()
	require.NoError(t, app.db.Create(&ExpenseCategory{
		Base: Base{ID: categoryID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Name: "Regression Office Supplies",
		Code: "REPAIR_OFFICE",
	}).Error)
	require.NoError(t, app.db.Create(&ExpenseEntry{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		EntryNumber:   "EXP-PL-001",
		Division:      "PH Trading",
		ExpenseDate:   time.Date(2026, time.February, 15, 0, 0, 0, 0, time.UTC),
		Description:   "Office supplies",
		CategoryID:    categoryID,
		CategoryName:  "Regression Office Supplies",
		Currency:      "BHD",
		Amount:        125,
		TotalAmount:   125,
		Status:        "approved",
		PaymentStatus: "unpaid",
	}).Error)
	require.NoError(t, app.db.Create(&ExpenseEntry{
		Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		EntryNumber:   "EXP-PL-DRAFT",
		Division:      "PH Trading",
		ExpenseDate:   time.Date(2026, time.February, 16, 0, 0, 0, 0, time.UTC),
		Description:   "Draft should not hit P&L",
		CategoryID:    categoryID,
		CategoryName:  "Regression Office Supplies",
		Currency:      "BHD",
		Amount:        50,
		TotalAmount:   50,
		Status:        "draft",
		PaymentStatus: "unpaid",
	}).Error)

	report, err := app.GetOperationalProfitLoss(OperationalLedgerFilter{
		Division:  "PH Trading",
		StartDate: "2026-02-01",
		EndDate:   "2026-02-28",
	})
	require.NoError(t, err)
	require.Equal(t, 1, report.InvoiceCount)
	require.Equal(t, 1, report.ExpenseCount)
	require.Equal(t, 1000.0, report.RevenueBHD)
	require.Equal(t, 600.0, report.COGSBHD)
	require.Equal(t, 125.0, report.ExpensesBHD)
	require.Equal(t, 275.0, report.NetIncomeBHD)
}

func TestBankReconciliationMatchesCustomerReceiptAndLegacyPaymentAllocations(t *testing.T) {
	app := setupTestApp(t)
	migrateFinanceRegressionRepairTables(t, app)
	customerName := "Recon Customer WLL"
	customerID := seedTestCustomer(t, app.db, customerName)
	statement := seedFinanceRepairStatement(t, app, "STMT-RCT-001")

	receipt, err := app.CreateCustomerReceipt(CustomerReceiptInput{
		CustomerID:    customerID,
		CustomerName:  customerName,
		AmountBHD:     100,
		ReceiptDate:   "2026-05-03",
		PaymentMethod: "Cash",
		Division:      "PH Trading",
	})
	require.NoError(t, err)

	invoice := seedFinanceRepairInvoice(t, app, customerID, customerName, "INV-LEGACY-PAY", 50, time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC))
	payment, err := app.RecordPayment(invoice.ID, 50, "Cash", "2026-05-04", "LEGACY-CASH-50")
	require.NoError(t, err)

	line := seedFinanceRepairCreditLine(t, app, statement.ID, 3, 150)
	require.NoError(t, app.CreateSplitAllocation(line.ID, []AllocationInput{
		{AllocationType: "CUSTOMER_RECEIPT", EntityID: receipt.ID, AllocatedAmount: 100},
		{AllocationType: "CUSTOMER_PAYMENT", EntityID: payment.ID, AllocatedAmount: 50},
	}, "admin"))

	var allocations []BankLinePaymentAllocation
	require.NoError(t, app.db.Where("bank_statement_line_id = ?", line.ID).Find(&allocations).Error)
	require.Len(t, allocations, 2)

	var receiptAllocation BankLinePaymentAllocation
	require.NoError(t, app.db.Where("customer_receipt_id = ?", receipt.ID).First(&receiptAllocation).Error)
	require.Equal(t, 100.0, receiptAllocation.AllocatedAmount)

	var paymentAllocation BankLinePaymentAllocation
	require.NoError(t, app.db.Where("customer_payment_id = ?", payment.ID).First(&paymentAllocation).Error)
	require.Equal(t, 50.0, paymentAllocation.AllocatedAmount)
}

func TestDeleteExpenseEntryAllowsApprovedUnpostedAndBlocksPostedRegardlessOfCase(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.EnsureExpenseFoundation())

	category, err := app.CreateExpenseCategory(ExpenseCategory{
		Name: "Repair Delete Category",
		Code: "REPAIR_DELETE",
	})
	require.NoError(t, err)

	approved, err := app.CreateExpenseEntry(ExpenseEntry{
		EntryNumber:   "EXP-DELETE-APPROVED",
		Description:   "Approved but unposted expense can be deleted",
		CategoryID:    category.ID,
		Amount:        40,
		ExpenseDate:   time.Date(2026, time.May, 6, 0, 0, 0, 0, time.UTC),
		Status:        "approved",
		PaymentStatus: "unpaid",
	})
	require.NoError(t, err)
	require.NoError(t, app.DeleteExpenseEntry(approved.ID))

	posted, err := app.CreateExpenseEntry(ExpenseEntry{
		EntryNumber:   "EXP-DELETE-POSTED",
		Description:   "Posted expense remains locked",
		CategoryID:    category.ID,
		Amount:        60,
		ExpenseDate:   time.Date(2026, time.May, 7, 0, 0, 0, 0, time.UTC),
		Status:        "Posted",
		PaymentStatus: "unpaid",
	})
	require.NoError(t, err)

	err = app.DeleteExpenseEntry(posted.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "posted or paid expenses cannot be deleted")
}

func TestPostedExpenseFeedsLedgerProfitLossAndJournalAudit(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.EnsureExpenseFoundation())

	category, err := app.CreateExpenseCategory(ExpenseCategory{
		Name: "Stress Test Compliance",
		Code: "STRESS_COMPLIANCE",
	})
	require.NoError(t, err)

	entry, err := app.CreateExpenseEntry(ExpenseEntry{
		EntryNumber:   "EXP-STRESS-LEDGER",
		Description:   "Stress test locked finance expense",
		CategoryID:    category.ID,
		Amount:        123.456,
		TotalAmount:   123.456,
		Currency:      "BHD",
		Division:      "PH Trading",
		ExpenseDate:   time.Date(2026, time.May, 25, 0, 0, 0, 0, time.UTC),
		Status:        "approved",
		PaymentStatus: "unpaid",
	})
	require.NoError(t, err)

	posted, err := app.PostExpenseEntry(entry.ID)
	require.NoError(t, err)
	require.Equal(t, "posted", strings.ToLower(posted.Status))
	require.NotNil(t, posted.JournalEntryID)

	ledger, err := app.GetExpenseLedger(OperationalLedgerFilter{
		Division:   "PH Trading",
		StartDate:  "2026-05-01",
		EndDate:    "2026-05-31",
		SourceType: "EXPENSE",
	})
	require.NoError(t, err)
	require.Len(t, ledger, 1)
	require.Equal(t, entry.ID, ledger[0].SourceID)
	require.InDelta(t, 123.456, ledger[0].DebitBHD, 0.001)

	pl, err := app.GetOperationalProfitLoss(OperationalLedgerFilter{
		Division:  "PH Trading",
		StartDate: "2026-05-01",
		EndDate:   "2026-05-31",
	})
	require.NoError(t, err)
	require.Equal(t, 1, pl.ExpenseCount)
	require.InDelta(t, 123.456, pl.ExpensesBHD, 0.001)
	require.InDelta(t, -123.456, pl.NetIncomeBHD, 0.001)

	isPosted := true
	journals, err := app.GetJournalEntries(2026, 0, &isPosted, 50)
	require.NoError(t, err)
	require.NotEmpty(t, journals)
	require.Equal(t, *posted.JournalEntryID, journals[0].ID)
	require.Equal(t, "expense_entry", journals[0].SourceType)
	require.Equal(t, entry.ID, journals[0].SourceID)

	err = app.DeleteExpenseEntry(entry.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "posted or paid expenses cannot be deleted")
}
