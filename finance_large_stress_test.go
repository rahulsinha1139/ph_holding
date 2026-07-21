package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type largeFinanceStressSeed struct {
	BankAccounts       []CompanyBankAccount
	ForeignBankAccount CompanyBankAccount
	Customers          []CustomerMaster
	Suppliers          []SupplierMaster
	Invoices           []Invoice
	SupplierInvoices   []SupplierInvoice
	BankStatements     []BankStatement
}

func migrateLargeFinanceStressTables(t *testing.T, app *App) {
	t.Helper()
	migrateFinanceSmokeTables(t, app)
	require.NoError(t, app.db.AutoMigrate(
		&CustomerMaster{},
		&SupplierMaster{},
		&Invoice{},
		&DBInvoiceItem{},
		&Payment{},
		&CustomerReceipt{},
		&CustomerReceiptAllocation{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&SupplierPayment{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&ChartOfAccount{},
		&JournalEntry{},
		&JournalLine{},
		&VATReturn{},
		&CreditNote{},
		&CreditNoteItem{},
		&TallyInvoiceImport{},
		&TallyPurchaseImport{},
		&BankLinePaymentAllocation{},
		&StatementHash{},
		&OperationalLedgerEntry{},
	))
}

func stressBase(now time.Time) Base {
	return Base{ID: uuid.New().String(), CreatedAt: now, UpdatedAt: now, CreatedBy: "finance-stress"}
}

func createInBatches[T any](t *testing.T, app *App, rows []T, size int) {
	t.Helper()
	if len(rows) == 0 {
		return
	}
	require.NoError(t, app.db.CreateInBatches(rows, size).Error)
}

func seedLargeFinanceStressData(t *testing.T, app *App) largeFinanceStressSeed {
	t.Helper()

	now := time.Now()
	currentYear := now.Year()
	require.NoError(t, app.SeedCompanyBankAccounts())
	require.NoError(t, app.EnsureExpenseFoundation())
	require.NoError(t, app.EnsurePayrollFoundation())
	require.NoError(t, app.ensureCriticalDeploymentFoundations())

	var bankAccounts []CompanyBankAccount
	require.NoError(t, app.db.Order("display_order asc").Find(&bankAccounts).Error)
	require.NotEmpty(t, bankAccounts)

	foreignBank := CompanyBankAccount{
		ID:            "stress-usd-bank",
		Division:      "PH Trading",
		BankName:      "Stress USD Bank",
		AccountName:   "P H TRADING WLL USD",
		AccountNumber: "USD-STRESS-0001",
		IBAN:          "BH00STRESSUSD0001",
		SwiftBIC:      "STRSBHBM",
		Currency:      "USD",
		IsActive:      true,
		DisplayOrder:  99,
		BookingRate:   0.365,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	require.NoError(t, app.db.Create(&foreignBank).Error)
	bankAccounts = append(bankAccounts, foreignBank)

	customers := make([]CustomerMaster, 160)
	for i := range customers {
		customers[i] = CustomerMaster{
			Base:             stressBase(now),
			CustomerID:       fmt.Sprintf("STRESS-CUST-%04d", i+1),
			CustomerCode:     fmt.Sprintf("SC%04d", i+1),
			BusinessName:     fmt.Sprintf("Stress Customer %04d W.L.L.", i+1),
			Status:           "Active",
			Country:          "Bahrain",
			PaymentGrade:     []string{"A", "B", "C", "D"}[i%4],
			CustomerGrade:    []string{"A", "B", "C", "D"}[i%4],
			PaymentTermsDays: 30 + (i%3)*15,
			CreditLimitBHD:   50000 + float64(i%10)*5000,
		}
	}
	createInBatches(t, app, customers, 100)

	suppliers := make([]SupplierMaster, 100)
	for i := range suppliers {
		suppliers[i] = SupplierMaster{
			Base:          stressBase(now),
			SupplierCode:  fmt.Sprintf("STRESS-SUP-%04d", i+1),
			SupplierName:  fmt.Sprintf("Stress Supplier %04d", i+1),
			Country:       []string{"Bahrain", "Germany", "UAE", "USA"}[i%4],
			SupplierType:  "Distributor",
			PaymentTerms:  "Net 30",
			Rating:        3 + i%3,
			LeadTimeDays:  14 + i%28,
			BrandsHandled: "Stress Instruments",
		}
	}
	createInBatches(t, app, suppliers, 100)

	invoices := make([]Invoice, 2400)
	invoiceItems := make([]DBInvoiceItem, 0, len(invoices)*2)
	payments := make([]Payment, 0, 1600)
	receipts := make([]CustomerReceipt, 0, 720)
	allocations := make([]CustomerReceiptAllocation, 0, 720)
	statuses := []string{"Sent", "PartiallyPaid", "Paid", "Overdue", "Draft", "Cancelled", "Void", "Proforma"}
	for i := range invoices {
		customer := customers[i%len(customers)]
		status := statuses[i%len(statuses)]
		total := 150 + float64(i%80)*17.25
		paid := 0.0
		if status == "Paid" {
			paid = total
		} else if status == "PartiallyPaid" {
			paid = total * 0.45
		}
		outstanding := total - paid
		if status == "Cancelled" || status == "Void" || status == "Draft" || status == "Proforma" {
			outstanding = 0
		}
		invoiceDate := now.AddDate(0, 0, -(i % 420))
		dueDate := invoiceDate.AddDate(0, 0, 30+(i%3)*15)
		invoices[i] = Invoice{
			Base:                 stressBase(now),
			InvoiceNumber:        fmt.Sprintf("STRESS-INV-%05d", i+1),
			InvoiceDate:          invoiceDate,
			CustomerID:           customer.ID,
			CustomerName:         customer.BusinessName,
			CustomerPONumber:     fmt.Sprintf("CPO-STRESS-%05d", i+1),
			GrandTotalBHD:        roundTo3(total),
			OutstandingBHD:       roundTo3(outstanding),
			SubtotalBHD:          roundTo3(total / 1.1),
			VATBHD:               roundTo3(total - total/1.1),
			VATPercent:           10,
			Status:               status,
			DueDate:              dueDate,
			Division:             []string{"PH Trading", "AHS Trading"}[i%2],
			PaymentTerms:         "Net 30",
			TotalSupplierCostBHD: roundTo3(total * 0.63),
			GrossMarginBHD:       roundTo3(total * 0.27),
			GrossMarginPercent:   27,
		}
		for line := 0; line < 2; line++ {
			lineTotal := roundTo3(total / 2)
			invoiceItems = append(invoiceItems, DBInvoiceItem{
				Base:        stressBase(now),
				InvoiceID:   invoices[i].ID,
				LineNumber:  line + 1,
				Description: fmt.Sprintf("Stress invoice item %d", line+1),
				Quantity:    1 + float64(line),
				Rate:        lineTotal,
				TotalBHD:    lineTotal,
				ProductCode: fmt.Sprintf("STRESS-PROD-%03d", i%50),
				Model:       fmt.Sprintf("STRESS-MODEL-%03d", i%50),
				TotalCost:   roundTo3(lineTotal * 0.63),
				TotalPrice:  lineTotal,
			})
		}
		if paid > 0 {
			paymentID := uuid.New().String()
			payments = append(payments, Payment{
				Base:           Base{ID: paymentID, CreatedAt: now, UpdatedAt: now, CreatedBy: "finance-stress"},
				InvoiceID:      invoices[i].ID,
				InvoiceNumber:  invoices[i].InvoiceNumber,
				AmountBHD:      roundTo3(paid),
				PaymentDate:    invoiceDate.AddDate(0, 0, 20+i%25),
				PaymentMethod:  []string{"Bank Transfer", "Cheque", "Cash"}[i%3],
				DaysToPayment:  20 + i%25,
				IdempotencyKey: fmt.Sprintf("stress-pay-%05d", i+1),
				BankAccountID:  bankAccounts[i%len(bankAccounts)].ID,
				Reference:      fmt.Sprintf("PAY-STRESS-%05d", i+1),
				Division:       invoices[i].Division,
			})
		}
		if i < 720 {
			applied := roundTo3(total * 0.25)
			receiptID := uuid.New().String()
			receipts = append(receipts, CustomerReceipt{
				Base:               Base{ID: receiptID, CreatedAt: now, UpdatedAt: now, CreatedBy: "finance-stress"},
				ReceiptNumber:      fmt.Sprintf("RCPT-STRESS-%05d", i+1),
				CustomerID:         customer.ID,
				CustomerName:       customer.BusinessName,
				Division:           invoices[i].Division,
				ReceiptDate:        invoiceDate.AddDate(0, 0, 10),
				AmountBHD:          applied,
				AppliedAmountBHD:   applied,
				UnappliedAmountBHD: 0,
				PaymentMethod:      "Bank Transfer",
				Reference:          fmt.Sprintf("R-STRESS-%05d", i+1),
				Status:             "Applied",
			})
			allocations = append(allocations, CustomerReceiptAllocation{
				Base:               stressBase(now),
				ReceiptID:          receiptID,
				InvoiceID:          invoices[i].ID,
				InvoiceNumber:      invoices[i].InvoiceNumber,
				PaymentID:          paymentIDForInvoice(payments, invoices[i].ID),
				AllocatedAmountBHD: applied,
				AllocationDate:     invoiceDate.AddDate(0, 0, 10),
				Status:             "Applied",
			})
		}
	}
	createInBatches(t, app, invoices, 250)
	createInBatches(t, app, invoiceItems, 500)
	createInBatches(t, app, payments, 250)
	createInBatches(t, app, receipts, 250)
	createInBatches(t, app, allocations, 250)

	supplierInvoices := make([]SupplierInvoice, 1800)
	supplierItems := make([]SupplierInvoiceItem, 0, len(supplierInvoices)*2)
	supplierPayments := make([]SupplierPayment, 0, 1000)
	for i := range supplierInvoices {
		supplier := suppliers[i%len(suppliers)]
		total := 100 + float64(i%90)*11.75
		status := []string{"Pending", "Approved", "Paid", "Dispute", "Rejected"}[i%5]
		paymentStatus := "Unpaid"
		if status == "Paid" {
			paymentStatus = "Paid"
		}
		invoiceDate := now.AddDate(0, 0, -(i % 390))
		supplierInvoices[i] = SupplierInvoice{
			Base:            stressBase(now),
			SupplierID:      supplier.ID,
			SupplierName:    supplier.SupplierName,
			InvoiceNumber:   fmt.Sprintf("SUPP-STRESS-%05d", i+1),
			InvoiceDate:     invoiceDate,
			DueDate:         invoiceDate.AddDate(0, 0, 45),
			Currency:        "BHD",
			ExchangeRate:    1,
			SubtotalForeign: roundTo3(total / 1.1),
			SubtotalBHD:     roundTo3(total / 1.1),
			VATForeign:      roundTo3(total - total/1.1),
			VATBHD:          roundTo3(total - total/1.1),
			TotalForeign:    roundTo3(total),
			TotalBHD:        roundTo3(total),
			MatchStatus:     []string{"Pending", "Matched", "Discrepancy", "Dispute"}[i%4],
			Status:          status,
			PaymentStatus:   paymentStatus,
			Division:        []string{"PH Trading", "AHS Trading"}[i%2],
		}
		for line := 0; line < 2; line++ {
			supplierItems = append(supplierItems, SupplierInvoiceItem{
				Base:              stressBase(now),
				SupplierInvoiceID: supplierInvoices[i].ID,
				LineNumber:        line + 1,
				Description:       fmt.Sprintf("Stress supplier item %d", line+1),
				Quantity:          1,
				UnitPrice:         roundTo3(total / 2),
				TotalPrice:        roundTo3(total / 2),
				Currency:          "BHD",
			})
		}
		if i%5 == 2 || i%5 == 1 {
			supplierPayments = append(supplierPayments, SupplierPayment{
				Base:              stressBase(now),
				SupplierInvoiceID: supplierInvoices[i].ID,
				SupplierID:        supplier.ID,
				AmountForeign:     roundTo3(total * 0.65),
				Currency:          "BHD",
				ExchangeRate:      1,
				AmountBHD:         roundTo3(total * 0.65),
				PaymentDate:       invoiceDate.AddDate(0, 0, 30),
				PaymentMethod:     []string{"Bank Transfer", "Cheque"}[i%2],
				Reference:         fmt.Sprintf("SPAY-STRESS-%05d", i+1),
				BankAccountID:     bankAccounts[i%len(bankAccounts)].ID,
				Division:          supplierInvoices[i].Division,
			})
		}
	}
	createInBatches(t, app, supplierInvoices, 250)
	createInBatches(t, app, supplierItems, 500)
	createInBatches(t, app, supplierPayments, 250)

	var categories []ExpenseCategory
	require.NoError(t, app.db.Find(&categories).Error)
	var vendors []ExpenseVendor
	require.NoError(t, app.db.Find(&vendors).Error)
	require.NotEmpty(t, categories)
	if len(vendors) == 0 {
		vendor := ExpenseVendor{Base: stressBase(now), Name: "Stress Expense Vendor", IsActive: true}
		require.NoError(t, app.db.Create(&vendor).Error)
		vendors = append(vendors, vendor)
	}
	expenses := make([]ExpenseEntry, 900)
	for i := range expenses {
		due := now.AddDate(0, 0, 15+i%20)
		vendorID := vendors[i%len(vendors)].ID
		amount := 20 + float64(i%50)*3.25
		status := []string{"draft", "submitted", "approved", "posted", "paid"}[i%5]
		paymentStatus := "unpaid"
		var postedAt *time.Time
		var paidAt *time.Time
		if status == "posted" || status == "paid" {
			pt := now.AddDate(0, 0, -i%40)
			postedAt = &pt
		}
		if status == "paid" {
			pd := now.AddDate(0, 0, -i%20)
			paidAt = &pd
			paymentStatus = "paid"
		}
		expenses[i] = ExpenseEntry{
			Base:          stressBase(now),
			EntryNumber:   fmt.Sprintf("EXP-STRESS-%05d", i+1),
			Division:      []string{"PH Trading", "AHS Trading"}[i%2],
			ExpenseDate:   now.AddDate(0, 0, -i%365),
			DueDate:       &due,
			Description:   fmt.Sprintf("Stress expense %05d", i+1),
			CategoryID:    categories[i%len(categories)].ID,
			VendorID:      &vendorID,
			SourceType:    "manual",
			Currency:      "BHD",
			Amount:        amount,
			VATAmount:     roundTo3(amount * 0.1),
			TotalAmount:   roundTo3(amount * 1.1),
			Status:        status,
			PaymentStatus: paymentStatus,
			PostedAt:      postedAt,
			PaidAt:        paidAt,
		}
	}
	createInBatches(t, app, expenses, 250)

	recurring := make([]RecurringExpense, 80)
	for i := range recurring {
		vendorID := vendors[i%len(vendors)].ID
		recurring[i] = RecurringExpense{
			Base:             stressBase(now),
			Name:             fmt.Sprintf("Stress recurring %03d", i+1),
			Division:         []string{"PH Trading", "AHS Trading"}[i%2],
			Description:      "Stress recurring expense",
			CategoryID:       categories[i%len(categories)].ID,
			VendorID:         &vendorID,
			Frequency:        "monthly",
			IntervalValue:    1,
			NextRunDate:      now.AddDate(0, 0, i%30),
			DefaultAmount:    125 + float64(i),
			DefaultVATAmount: 12.5,
			Currency:         "BHD",
			IsActive:         true,
		}
	}
	createInBatches(t, app, recurring, 100)

	employees := make([]Employee, 48)
	compProfiles := make([]EmployeeCompensationProfile, 48)
	for i := range employees {
		employees[i] = Employee{
			Base:             stressBase(now),
			EmployeeCode:     fmt.Sprintf("FST-%03d", i+1),
			FullName:         fmt.Sprintf("Finance Stress Employee %03d", i+1),
			Department:       []string{"Finance", "Operations", "Sales"}[i%3],
			JobTitle:         "Analyst",
			EmploymentStatus: "active",
			IsActive:         true,
		}
		compProfiles[i] = EmployeeCompensationProfile{
			Base:               stressBase(now),
			EmployeeID:         employees[i].ID,
			Division:           []string{"PH Trading", "AHS Trading"}[i%2],
			PayFrequency:       "monthly",
			Currency:           "BHD",
			BaseSalary:         350 + float64(i%20)*15,
			HousingAllowance:   40,
			TransportAllowance: 25,
			EmployerCost:       430 + float64(i%20)*15,
			IsActive:           true,
		}
	}
	createInBatches(t, app, employees, 100)
	createInBatches(t, app, compProfiles, 100)

	payrollPeriods := make([]PayrollPeriod, 8)
	payrollRuns := make([]PayrollRun, 8)
	payrollItems := make([]PayrollRunItem, 0, 8*len(employees))
	payrollPayouts := make([]PayrollPayout, 0, 8*len(employees))
	for p := range payrollPeriods {
		start := time.Date(currentYear, time.Month(p+1), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)
		payDate := end.AddDate(0, 0, 5)
		payrollPeriods[p] = PayrollPeriod{
			Base:        stressBase(now),
			Name:        fmt.Sprintf("Stress Payroll %02d/%d", p+1, currentYear),
			Division:    "PH Trading",
			PeriodStart: start,
			PeriodEnd:   end,
			PaymentDate: &payDate,
			Status:      []string{"open", "closed"}[p%2],
		}
		netTotal := 0.0
		runID := uuid.New().String()
		for i := range employees {
			net := compProfiles[i].BaseSalary + compProfiles[i].HousingAllowance + compProfiles[i].TransportAllowance
			netTotal += net
			itemID := uuid.New().String()
			payrollItems = append(payrollItems, PayrollRunItem{
				Base:                 Base{ID: itemID, CreatedAt: now, UpdatedAt: now, CreatedBy: "finance-stress"},
				PayrollRunID:         runID,
				EmployeeID:           employees[i].ID,
				EmployeeNameSnapshot: employees[i].FullName,
				JobTitleSnapshot:     employees[i].JobTitle,
				BaseSalary:           compProfiles[i].BaseSalary,
				AllowancesTotal:      compProfiles[i].HousingAllowance + compProfiles[i].TransportAllowance,
				DeductionsTotal:      0,
				EmployerCostTotal:    compProfiles[i].EmployerCost,
				GrossPay:             net,
				NetPay:               net,
				Status:               []string{"draft", "approved", "paid"}[p%3],
			})
			payrollPayouts = append(payrollPayouts, PayrollPayout{
				Base:             stressBase(now),
				PayrollRunID:     runID,
				PayrollRunItemID: itemID,
				EmployeeID:       employees[i].ID,
				Division:         "PH Trading",
				ScheduledAt:      &payDate,
				Amount:           net,
				Currency:         "BHD",
				Status:           []string{"scheduled", "paid", "reconciled"}[p%3],
				BankAccountID:    &bankAccounts[0].ID,
			})
		}
		payrollRuns[p] = PayrollRun{
			Base:              Base{ID: runID, CreatedAt: now, UpdatedAt: now, CreatedBy: "finance-stress"},
			RunNumber:         fmt.Sprintf("PAY-STRESS-%04d", p+1),
			PayrollPeriodID:   payrollPeriods[p].ID,
			Division:          "PH Trading",
			Status:            []string{"draft", "approved", "paid"}[p%3],
			TotalEmployees:    len(employees),
			GrossTotal:        netTotal,
			NetTotal:          netTotal,
			EmployerCostTotal: netTotal,
			Currency:          "BHD",
		}
	}
	createInBatches(t, app, payrollPeriods, 100)
	createInBatches(t, app, payrollRuns, 100)
	createInBatches(t, app, payrollItems, 250)
	createInBatches(t, app, payrollPayouts, 250)

	statements := make([]BankStatement, 0, len(bankAccounts)*12)
	statementLines := make([]BankStatementLine, 0, len(bankAccounts)*12*80)
	for accountIndex, account := range bankAccounts {
		balance := 10000 + float64(accountIndex)*1000
		for month := 0; month < 12; month++ {
			start := time.Date(currentYear, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
			end := start.AddDate(0, 1, -1)
			statement := BankStatement{
				Base:            stressBase(now),
				BankAccountID:   account.ID,
				StatementNumber: fmt.Sprintf("BSTRESS-%02d-%02d", accountIndex+1, month+1),
				StatementDate:   end,
				PeriodStart:     start,
				PeriodEnd:       end,
				OpeningBalance:  balance,
				Currency:        account.Currency,
				Status:          []string{"Imported", "InProgress", "Reconciled", "Verified"}[month%4],
				ImportMethod:    "CSV",
				Division:        account.Division,
			}
			debits := 0.0
			credits := 0.0
			for line := 0; line < 80; line++ {
				debit := 0.0
				credit := 0.0
				if line%3 == 0 {
					credit = roundTo3(40 + float64(line%25)*4.2)
					credits += credit
					balance += credit
				} else {
					debit = roundTo3(20 + float64(line%35)*2.6)
					debits += debit
					balance -= debit
				}
				statementLines = append(statementLines, BankStatementLine{
					Base:            stressBase(now),
					BankStatementID: statement.ID,
					LineNumber:      line + 1,
					TransactionDate: start.AddDate(0, 0, line%28),
					ValueDate:       start.AddDate(0, 0, line%28),
					Description:     fmt.Sprintf("Stress bank line %02d-%02d-%03d", accountIndex+1, month+1, line+1),
					Reference:       fmt.Sprintf("BNK-STRESS-%02d-%02d-%03d", accountIndex+1, month+1, line+1),
					Debit:           debit,
					Credit:          credit,
					Balance:         roundTo3(balance),
					TransactionType: []string{"customer_receipt", "supplier_payment", "expense"}[line%3],
					MatchType:       []string{"Unmatched", "Auto", "Manual"}[line%3],
					IsMatched:       line%3 != 0,
					MatchConfidence: float64(70 + line%30),
				})
			}
			statement.TotalDebits = roundTo3(debits)
			statement.TotalCredits = roundTo3(credits)
			statement.DebitCount = 53
			statement.CreditCount = 27
			statement.ClosingBalance = roundTo3(balance)
			statements = append(statements, statement)
		}
	}
	createInBatches(t, app, statements, 100)
	createInBatches(t, app, statementLines, 500)

	register, err := app.CreateChequeRegister(bankAccounts[0].ID, "STRESS-CHEQUE-BOOK", 1000, 1400)
	require.NoError(t, err)
	require.Equal(t, "ACTIVE", register.Status)
	for i := 0; i < 320; i++ {
		cheque, err := app.IssueCheque(bankAccounts[0].ID, 25+float64(i%80), fmt.Sprintf("Cheque Payee %03d", i+1), "OTHER", nil, "large finance stress")
		require.NoError(t, err)
		if i%5 == 0 {
			require.NoError(t, app.MarkChequePresented(cheque.ChequeNumber))
		}
		if i%7 == 0 {
			require.NoError(t, app.MarkChequeStale(cheque.ChequeNumber))
		}
	}

	for i := 0; i < 12; i++ {
		date := time.Date(currentYear, time.Month(i+1), 1, 0, 0, 0, 0, time.UTC)
		_, err := app.CreateFXRate("USD", "BHD", 0.365+float64(i)*0.001, date, "STRESS")
		require.NoError(t, err)
		_, err = app.CreateFXRate("EUR", "BHD", 0.405+float64(i)*0.001, date, "STRESS")
		require.NoError(t, err)
	}
	_, err = app.CalculateFXRevaluation(foreignBank.ID, now)
	require.NoError(t, err)

	for i := 0; i < 14; i++ {
		_, err := app.CreateDepositInTransit(bankAccounts[0].ID, now.AddDate(0, 0, -i), 100+float64(i)*3, fmt.Sprintf("DEP-STRESS-%03d", i+1), "Stress deposit", "CUSTOMER_PAYMENT", &customers[i%len(customers)].ID, []string{invoices[i].ID})
		require.NoError(t, err)
	}
	bookRecon, err := app.CreateBookBankReconciliation(bankAccounts[0].ID, now, 10000, 9850)
	require.NoError(t, err)
	_, err = app.UpdateBookBankReconciliation(bookRecon.ID, 15, 10, 0, 0, 5, "stress adjustments")
	require.NoError(t, err)

	return largeFinanceStressSeed{
		BankAccounts:       bankAccounts,
		ForeignBankAccount: foreignBank,
		Customers:          customers,
		Suppliers:          suppliers,
		Invoices:           invoices,
		SupplierInvoices:   supplierInvoices,
		BankStatements:     statements,
	}
}

func paymentIDForInvoice(payments []Payment, invoiceID string) string {
	for _, payment := range payments {
		if payment.InvoiceID == invoiceID {
			return payment.ID
		}
	}
	return ""
}

func TestLargeFinancePagesStressSimulation(t *testing.T) {
	app := setupTestApp(t)
	migrateLargeFinanceStressTables(t, app)

	seed := seedLargeFinanceStressData(t, app)
	now := time.Now()
	currentYear := now.Year()
	startDate := time.Date(currentYear, 1, 1, 0, 0, 0, 0, time.UTC)

	pageChecks := []struct {
		name string
		call func(t *testing.T)
	}{
		{name: "finance dashboard", call: func(t *testing.T) {
			_ = app.GetFinancialDashboard()
			_, err := app.GetDynamicFinancialDashboard(currentYear)
			require.NoError(t, err)
			years, err := app.GetAvailableFinancialYears()
			require.NoError(t, err)
			require.Contains(t, years, currentYear)
			_, err = app.GetCashPosition()
			require.NoError(t, err)
			_, err = app.GetPaymentAgingReport()
			require.NoError(t, err)
			_, err = app.GetCashFlowProjection(90)
			require.NoError(t, err)
			_, err = app.GetVATReconciliation(startDate.Format("2006-01-02"), now.Format("2006-01-02"))
			require.NoError(t, err)
			_, err = app.GetARAgingReport()
			require.NoError(t, err)
			_, err = app.GetAPAgingReport()
			require.NoError(t, err)
		}},
		{name: "customer invoices page", call: func(t *testing.T) {
			rows, err := app.ListCustomerInvoices(200, 0)
			require.NoError(t, err)
			require.Len(t, rows, 200)
			_, _, _, _, err = app.GetInvoiceRevenueSummary()
			require.NoError(t, err)
			_, err = app.GetUnpaidInvoices()
			require.NoError(t, err)
			_, err = app.GetOverdueInvoices()
			require.NoError(t, err)
			_, err = app.GetInvoicesByStatus("Sent")
			require.NoError(t, err)
			_, err = app.GetInvoicesByDateRange(startDate, now)
			require.NoError(t, err)
		}},
		{name: "receipts page", call: func(t *testing.T) {
			rows, err := app.GetAllPayments(200, 0)
			require.NoError(t, err)
			require.NotEmpty(t, rows)
			receipts, err := app.ListCustomerReceipts(200, 0)
			require.NoError(t, err)
			require.NotEmpty(t, receipts)
			allocs, err := app.GetCustomerReceiptAllocations(receipts[0].ID)
			require.NoError(t, err)
			require.NotNil(t, allocs)
		}},
		{name: "supplier settlements page", call: func(t *testing.T) {
			invoices, err := app.GetSupplierInvoices()
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(invoices), 1800)
			payments, err := app.GetAllSupplierPayments()
			require.NoError(t, err)
			require.NotEmpty(t, payments)
			_, err = app.GetSupplierPaymentsSummary()
			require.NoError(t, err)
			_, err = app.GetSupplierInvoicesBySupplier(seed.Suppliers[0].ID)
			require.NoError(t, err)
		}},
		{name: "expenses page", call: func(t *testing.T) {
			_, err := app.ListExpenseDashboardSummary()
			require.NoError(t, err)
			entries, err := app.ListExpenseEntries("", true)
			require.NoError(t, err)
			require.GreaterOrEqual(t, len(entries), 900)
			_, err = app.ListExpenseEntries("approved", true)
			require.NoError(t, err)
			_, err = app.ListExpenseCategories(true)
			require.NoError(t, err)
			_, err = app.ListExpenseVendors(true)
			require.NoError(t, err)
			_, err = app.ListRecurringExpenses(true)
			require.NoError(t, err)
			_, err = app.ListBankExpenseCandidates(true)
			require.NoError(t, err)
		}},
		{name: "accounting page", call: func(t *testing.T) {
			_, err := app.GetChartOfAccounts("All")
			require.NoError(t, err)
			posted := true
			_, err = app.GetJournalEntries(currentYear, 0, &posted, 200)
			require.NoError(t, err)
			_, err = app.GetOperationalLedger(OperationalLedgerFilter{StartDate: startDate.Format("2006-01-02"), EndDate: now.Format("2006-01-02"), Division: "PH Trading"})
			require.NoError(t, err)
			_, err = app.GetOperationalProfitLoss(OperationalLedgerFilter{StartDate: startDate.Format("2006-01-02"), EndDate: now.Format("2006-01-02"), Division: "PH Trading"})
			require.NoError(t, err)
			_, err = app.GetOperationalBalanceSheet(now.Format("2006-01-02"), "PH Trading")
			require.NoError(t, err)
		}},
		{name: "reports exports page", call: func(t *testing.T) {
			_, err := app.GetReportData("financial", "year")
			require.NoError(t, err)
			_, err = app.GenerateProfitAndLoss(currentYear)
			require.NoError(t, err)
			_, err = app.GenerateBalanceSheet(currentYear)
			require.NoError(t, err)
			_, err = app.ExportVATReturnData(currentYear, 1)
			require.NoError(t, err)
		}},
		{name: "approvals page", call: func(t *testing.T) {
			entries, err := app.ListExpenseEntries("submitted", true)
			require.NoError(t, err)
			require.NotEmpty(t, entries)
			require.Equal(t, "submitted", entries[0].Status)
		}},
		{name: "payroll page", call: func(t *testing.T) {
			_, err := app.ListPayrollDashboardSummary()
			require.NoError(t, err)
			profiles, err := app.ListEmployeeCompensationProfiles(true)
			require.NoError(t, err)
			require.Len(t, profiles, 48)
			periods, err := app.ListPayrollPeriods(true)
			require.NoError(t, err)
			require.Len(t, periods, 8)
			runs, err := app.ListPayrollRuns("")
			require.NoError(t, err)
			require.Len(t, runs, 8)
			payouts, err := app.ListPayrollPayouts(runs[0].ID)
			require.NoError(t, err)
			require.NotEmpty(t, payouts)
			_, err = app.ListUnreconciledPayrollPayouts()
			require.NoError(t, err)
		}},
		{name: "bank reconciliation page", call: func(t *testing.T) {
			statements, err := app.GetBankStatements(seed.BankAccounts[0].ID)
			require.NoError(t, err)
			require.Len(t, statements, 12)
			lines, err := app.GetBankStatementLines(statements[0].ID)
			require.NoError(t, err)
			require.Len(t, lines, 80)
			_, err = app.GetUnmatchedLines(statements[0].ID)
			require.NoError(t, err)
			_, err = app.GetReconciliationSummary(statements[0].ID)
			require.NoError(t, err)
			_, err = app.GetReconciliationStats(seed.BankAccounts[0].ID)
			require.NoError(t, err)
			_, err = app.AutoMatchBankLines(statements[0].ID)
			require.NoError(t, err)
		}},
		{name: "book-bank reconciliation page", call: func(t *testing.T) {
			recons, err := app.GetBookBankReconciliations(seed.BankAccounts[0].ID)
			require.NoError(t, err)
			require.NotEmpty(t, recons)
			_, err = app.GetLatestBookBankReconciliation(seed.BankAccounts[0].ID)
			require.NoError(t, err)
			_, err = app.GetReconciliationVariances(recons[0].ID)
			require.NoError(t, err)
			_, err = app.GetBookBankReconciliationReport(recons[0].ID)
			require.NoError(t, err)
			_, err = app.GetReconciliationStatusSummary()
			require.NoError(t, err)
			_, _, err = app.GetDepositsInTransit(seed.BankAccounts[0].ID)
			require.NoError(t, err)
		}},
		{name: "cheques page", call: func(t *testing.T) {
			registers, err := app.GetChequeRegisters(seed.BankAccounts[0].ID)
			require.NoError(t, err)
			require.NotEmpty(t, registers)
			next, err := app.GetNextChequeNumber(seed.BankAccounts[0].ID)
			require.NoError(t, err)
			require.Equal(t, "001320", next)
			outstanding, total, err := app.GetOutstandingCheques(seed.BankAccounts[0].ID)
			require.NoError(t, err)
			require.NotEmpty(t, outstanding)
			require.Greater(t, total, 0.0)
			_, err = app.GetChequesByStatus(seed.BankAccounts[0].ID, "ISSUED")
			require.NoError(t, err)
			_, err = app.GetStaleCheques(seed.BankAccounts[0].ID)
			require.NoError(t, err)
			_, err = app.GetChequeRegisterReport(seed.BankAccounts[0].ID, startDate, now)
			require.NoError(t, err)
		}},
		{name: "fx page", call: func(t *testing.T) {
			_, err := app.GetLatestFXRate("USD", "BHD")
			require.NoError(t, err)
			_, err = app.GetFXRateHistory("USD", "BHD", startDate, now)
			require.NoError(t, err)
			_, err = app.GetAllFXRates("BHD", now)
			require.NoError(t, err)
			revals, err := app.GetFXRevaluations(seed.ForeignBankAccount.ID)
			require.NoError(t, err)
			require.NotEmpty(t, revals)
			_, _, err = app.GetUnpostedRevaluations()
			require.NoError(t, err)
			_, _, err = app.GetFXExposureReport()
			require.NoError(t, err)
			_, err = app.GetFXGainLossSummary(currentYear)
			require.NoError(t, err)
		}},
	}

	for _, check := range pageChecks {
		t.Run(check.name, check.call)
	}

	var invoiceCount, paymentCount, supplierInvoiceCount, supplierPaymentCount, statementLineCount, chequeCount int64
	require.NoError(t, app.db.Model(&Invoice{}).Count(&invoiceCount).Error)
	require.NoError(t, app.db.Model(&Payment{}).Count(&paymentCount).Error)
	require.NoError(t, app.db.Model(&SupplierInvoice{}).Count(&supplierInvoiceCount).Error)
	require.NoError(t, app.db.Model(&SupplierPayment{}).Count(&supplierPaymentCount).Error)
	require.NoError(t, app.db.Model(&BankStatementLine{}).Count(&statementLineCount).Error)
	require.NoError(t, app.db.Model(&OutstandingCheque{}).Count(&chequeCount).Error)
	require.EqualValues(t, 2400, invoiceCount)
	require.GreaterOrEqual(t, paymentCount, int64(600))
	require.EqualValues(t, 1800, supplierInvoiceCount)
	require.GreaterOrEqual(t, supplierPaymentCount, int64(700))
	require.GreaterOrEqual(t, statementLineCount, int64(5600))
	require.EqualValues(t, 320, chequeCount)
}
