package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func migrateFinanceSmokeTables(t *testing.T, app *App) {
	t.Helper()
	require.NoError(t, app.db.AutoMigrate(
		&Role{},
		&User{},
		&LicenseKey{},
		&CompanyBankAccount{},
		&BankStatement{},
		&BankStatementLine{},
		&BankStatementFile{},
		&BankReconciliationAuditLog{},
		&StatementHash{},
		&ChequeRegister{},
		&OutstandingCheque{},
		&DepositInTransit{},
		&BookBankReconciliation{},
		&FXRate{},
		&FXRevaluation{},
		&ChartOfAccount{},
		&JournalEntry{},
		&JournalLine{},
		&CustomerMaster{},
		&CustomerReceipt{},
		&CustomerReceiptAllocation{},
		&SupplierMaster{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&SupplierPayment{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&ExpenseCategory{},
		&ExpenseVendor{},
		&ExpenseEntry{},
		&ExpenseAllocation{},
		&ExpenseAttachment{},
		&ExpenseApproval{},
		&RecurringExpense{},
		&BankExpenseEntry{},
		&Employee{},
		&EmployeeAccessLink{},
		&EmployeeCompensationProfile{},
		&PayrollPeriod{},
		&PayrollRun{},
		&PayrollRunItem{},
		&PayrollComponent{},
		&PayrollPayout{},
	))
}

func TestFinanceHubBackendEndpointsSmoke(t *testing.T) {
	app := setupTestApp(t)
	migrateFinanceSmokeTables(t, app)

	require.NoError(t, app.SeedCompanyBankAccounts())
	require.NoError(t, app.EnsureExpenseFoundation())
	require.NoError(t, app.EnsurePayrollFoundation())

	accounts, err := app.GetActiveBankAccounts()
	require.NoError(t, err)
	require.NotEmpty(t, accounts)
	primaryAccountID := accounts[0].ID

	allAccounts, err := app.GetAllBankAccounts()
	require.NoError(t, err)
	require.NotEmpty(t, allAccounts)

	_, err = app.GetCashPosition()
	require.NoError(t, err)

	statements, err := app.GetBankStatements(primaryAccountID)
	require.NoError(t, err)
	require.Empty(t, statements)

	register, err := app.CreateChequeRegister(primaryAccountID, "SMOKE-BOOK", 100, 105)
	require.NoError(t, err)
	require.Equal(t, "ACTIVE", register.Status)

	nextCheque, err := app.GetNextChequeNumber(primaryAccountID)
	require.NoError(t, err)
	require.Equal(t, "000100", nextCheque)

	issued, err := app.IssueCheque(primaryAccountID, 25.500, "Smoke Supplier", "OTHER", nil, "Finance hub smoke")
	require.NoError(t, err)
	require.Equal(t, "000100", issued.ChequeNumber)

	outstanding, _, err := app.GetOutstandingCheques(primaryAccountID)
	require.NoError(t, err)
	require.Len(t, outstanding, 1)

	stale, err := app.GetStaleCheques(primaryAccountID)
	require.NoError(t, err)
	require.Empty(t, stale)

	bookBank, err := app.CreateBookBankReconciliation(primaryAccountID, time.Now(), 1000, 974.5)
	require.NoError(t, err)
	require.InDelta(t, 0, bookBank.Difference, 0.001)

	recons, err := app.GetBookBankReconciliations(primaryAccountID)
	require.NoError(t, err)
	require.NotEmpty(t, recons)

	variances, err := app.GetReconciliationVariances(bookBank.ID)
	require.NoError(t, err)
	require.NotEmpty(t, variances)

	require.NoError(t, app.FinalizeBookBankReconciliation(bookBank.ID, "finance-smoke"))

	_, err = app.GetBookBankReconciliationReport(bookBank.ID)
	require.NoError(t, err)

	usdAccount := CompanyBankAccount{
		ID:            uuid.New().String(),
		Division:      "PH Trading",
		BankName:      "Smoke FX Bank",
		AccountName:   "Smoke USD",
		AccountNumber: "USD-SMOKE",
		Currency:      "USD",
		IsActive:      true,
		BookingRate:   0.360,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	require.NoError(t, app.db.Create(&usdAccount).Error)
	require.NoError(t, app.db.Create(&BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   usdAccount.ID,
		StatementNumber: "USD-SMOKE-001",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -30),
		PeriodEnd:       time.Now(),
		OpeningBalance:  900,
		ClosingBalance:  1000,
		Currency:        "USD",
		Status:          "Imported",
		Division:        "PH Trading",
	}).Error)

	_, err = app.CreateFXRate("USD", "BHD", 0.377, time.Now(), "SMOKE")
	require.NoError(t, err)

	_, err = app.CalculateFXRevaluation(usdAccount.ID, time.Now())
	require.NoError(t, err)

	fxRows, err := app.GetFXRevaluations(usdAccount.ID)
	require.NoError(t, err)
	require.Len(t, fxRows, 1)

	_, _, err = app.GetFXExposureReport()
	require.NoError(t, err)

	fxSummary, err := app.GetFXGainLossSummary(time.Now().Year())
	require.NoError(t, err)
	require.NotNil(t, fxSummary)

	_, err = app.ListExpenseDashboardSummary()
	require.NoError(t, err)
	entries, err := app.ListExpenseEntries("", true)
	require.NoError(t, err)
	require.Empty(t, entries)

	_, err = app.ListPayrollDashboardSummary()
	require.NoError(t, err)
	periods, err := app.ListPayrollPeriods(true)
	require.NoError(t, err)
	require.Empty(t, periods)
	runs, err := app.ListPayrollRuns("")
	require.NoError(t, err)
	require.Empty(t, runs)

	_, err = app.GetOperationalLedger(OperationalLedgerFilter{
		StartDate: time.Now().AddDate(0, -1, 0).Format("2006-01-02"),
		EndDate:   time.Now().Format("2006-01-02"),
		Division:  "PH Trading",
	})
	require.NoError(t, err)
	_, err = app.GetOperationalProfitLoss(OperationalLedgerFilter{
		StartDate: time.Now().AddDate(0, -1, 0).Format("2006-01-02"),
		EndDate:   time.Now().Format("2006-01-02"),
		Division:  "PH Trading",
	})
	require.NoError(t, err)
	_, err = app.GetOperationalBalanceSheet(time.Now().Format("2006-01-02"), "PH Trading")
	require.NoError(t, err)
}
