package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func migrateBankReconciliationPolicyTables(t *testing.T, app *App) {
	t.Helper()
	require.NoError(t, app.db.AutoMigrate(
		&SupplierInvoice{},
		&SupplierPayment{},
		&BankStatement{},
		&BankStatementLine{},
		&BankReconciliationAuditLog{},
	))
}

func dropBankReconciliationAuditLogTable(t *testing.T, app *App) {
	t.Helper()
	require.NoError(t, app.db.Migrator().DropTable(&BankReconciliationAuditLog{}))
}

func TestUpdateBankStatement_NormalizesEditableStatusAndRejectsFinalStates(t *testing.T) {
	app := setupTestApp(t)
	migrateBankReconciliationPolicyTables(t, app)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-EDIT-001",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -7),
		PeriodEnd:       time.Now(),
		Status:          bankStatementStatusImported,
	}
	require.NoError(t, app.db.Create(&statement).Error)

	require.NoError(t, app.UpdateBankStatement(statement.ID, map[string]interface{}{"status": "In Progress"}))

	var updated BankStatement
	require.NoError(t, app.db.First(&updated, "id = ?", statement.ID).Error)
	require.Equal(t, bankStatementStatusInProgress, updated.Status)

	err := app.UpdateBankStatement(statement.ID, map[string]interface{}{"status": "Reconciled"})
	require.ErrorContains(t, err, "use the reconciliation workflow")
}

func TestGetReconciliationStats_FiltersScopedStatusCounts(t *testing.T) {
	app := setupTestApp(t)
	migrateBankReconciliationPolicyTables(t, app)

	makeStatement := func(accountID, number, status string) {
		statement := BankStatement{
			Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			BankAccountID:   accountID,
			StatementNumber: number,
			StatementDate:   time.Now(),
			PeriodStart:     time.Now().AddDate(0, 0, -7),
			PeriodEnd:       time.Now(),
			Status:          status,
		}
		require.NoError(t, app.db.Create(&statement).Error)
	}

	makeStatement("acct-1", "STMT-A-IMP", bankStatementStatusImported)
	makeStatement("acct-1", "STMT-A-IP", bankStatementStatusInProgress)
	makeStatement("acct-1", "STMT-A-REC", bankStatementStatusReconciled)
	makeStatement("acct-2", "STMT-B-IMP", bankStatementStatusImported)
	makeStatement("acct-2", "STMT-B-IP", bankStatementStatusInProgress)

	stats, err := app.GetReconciliationStats("acct-1")
	require.NoError(t, err)
	require.EqualValues(t, 3, stats["total_statements"])
	require.EqualValues(t, 1, stats["imported_statements"])
	require.EqualValues(t, 1, stats["in_progress_statements"])
	require.EqualValues(t, 1, stats["reconciled_statements"])
}

func TestManualMatchLine_RejectsReconciledStatementMutations(t *testing.T) {
	app := setupTestApp(t)
	migrateBankReconciliationPolicyTables(t, app)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-REC-001",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -7),
		PeriodEnd:       time.Now(),
		Status:          bankStatementStatusReconciled,
	}
	require.NoError(t, app.db.Create(&statement).Error)

	line := BankStatementLine{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankStatementID: statement.ID,
		LineNumber:      1,
		TransactionDate: time.Now(),
		ValueDate:       time.Now(),
		Description:     "Locked supplier transfer",
		Debit:           100,
		Balance:         900,
		MatchType:       "Unmatched",
	}
	require.NoError(t, app.db.Create(&line).Error)

	payment := SupplierPayment{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierInvoiceID: "supp-inv-1",
		SupplierID:        "sup-1",
		SupplierName:      "Endress+Hauser",
		InvoiceNumber:     "SUPP-2026-002",
		AmountForeign:     100,
		Currency:          "BHD",
		ExchangeRate:      1,
		AmountBHD:         100,
		PaymentDate:       time.Now(),
		PaymentMethod:     "Bank Transfer",
		Reference:         "TXN-LOCKED",
	}
	require.NoError(t, app.db.Create(&payment).Error)

	err := app.ManualMatchLine(line.ID, "SUPPLIER_PAYMENT", payment.ID, "admin")
	require.ErrorContains(t, err, "reopen the reconciliation first")
}

func TestUpdateBankStatementLine_RejectsFinalizedStatementMutations(t *testing.T) {
	app := setupTestApp(t)
	migrateBankReconciliationPolicyTables(t, app)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-REC-002",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -7),
		PeriodEnd:       time.Now(),
		Status:          bankStatementStatusVerified,
	}
	require.NoError(t, app.db.Create(&statement).Error)

	line := BankStatementLine{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankStatementID: statement.ID,
		LineNumber:      1,
		TransactionDate: time.Now(),
		ValueDate:       time.Now(),
		Description:     "Verified line",
		Debit:           50,
		Balance:         950,
	}
	require.NoError(t, app.db.Create(&line).Error)

	err := app.UpdateBankStatementLine(line.ID, map[string]interface{}{"description": "Edited after reconciliation"})
	require.ErrorContains(t, err, "reopen the reconciliation first")
}

func TestUnmatchLine_ClearsMatchedSupplierPaymentLink(t *testing.T) {
	app := setupTestApp(t)
	migrateBankReconciliationPolicyTables(t, app)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-UNMATCH-001",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -7),
		PeriodEnd:       time.Now(),
		Status:          bankStatementStatusImported,
	}
	require.NoError(t, app.db.Create(&statement).Error)

	line := BankStatementLine{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankStatementID:  statement.ID,
		LineNumber:       1,
		TransactionDate:  time.Now(),
		ValueDate:        time.Now(),
		Description:      "Matched transfer",
		Debit:            100,
		Balance:          900,
		IsMatched:        true,
		MatchedPaymentID: "pay-123",
		MatchType:        "Manual",
		MatchConfidence:  1,
	}
	require.NoError(t, app.db.Create(&line).Error)

	require.NoError(t, app.UnmatchLine(line.ID, "admin", "regression check"))

	var updated BankStatementLine
	require.NoError(t, app.db.First(&updated, "id = ?", line.ID).Error)
	require.False(t, updated.IsMatched)
	require.Empty(t, updated.MatchedPaymentID)
}

func TestManualMatchLine_RollsBackWhenAuditLogWriteFails(t *testing.T) {
	app := setupTestApp(t)
	migrateBankReconciliationPolicyTables(t, app)
	dropBankReconciliationAuditLogTable(t, app)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-AUDIT-ROLLBACK-001",
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
		Description:     "Supplier transfer",
		Debit:           100,
		Balance:         900,
		MatchType:       "Unmatched",
	}
	require.NoError(t, app.db.Create(&line).Error)

	payment := SupplierPayment{
		Base:              Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierInvoiceID: "supp-inv-rollback",
		SupplierID:        "sup-1",
		SupplierName:      "Rollback Supplier",
		InvoiceNumber:     "SUPP-ROLLBACK-001",
		AmountForeign:     100,
		Currency:          "BHD",
		ExchangeRate:      1,
		AmountBHD:         100,
		PaymentDate:       time.Now(),
		PaymentMethod:     "Bank Transfer",
		Reference:         "TXN-ROLLBACK",
	}
	require.NoError(t, app.db.Create(&payment).Error)

	err := app.ManualMatchLine(line.ID, "SUPPLIER_PAYMENT", payment.ID, "admin")
	require.ErrorContains(t, err, "failed to log reconciliation action")

	var updated BankStatementLine
	require.NoError(t, app.db.First(&updated, "id = ?", line.ID).Error)
	require.False(t, updated.IsMatched)
	require.Empty(t, updated.MatchedPaymentID)
}

func TestReopenReconciliation_RollsBackWhenAuditLogWriteFails(t *testing.T) {
	app := setupTestApp(t)
	migrateBankReconciliationPolicyTables(t, app)
	dropBankReconciliationAuditLogTable(t, app)
	reconciledAt := time.Now()

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-REOPEN-ROLLBACK-001",
		StatementDate:   time.Now(),
		PeriodStart:     time.Now().AddDate(0, 0, -7),
		PeriodEnd:       time.Now(),
		Status:          bankStatementStatusReconciled,
		ReconciledAt:    &reconciledAt,
		ReconciledBy:    "admin",
	}
	require.NoError(t, app.db.Create(&statement).Error)

	err := app.ReopenReconciliation(statement.ID, "admin", "audit regression")
	require.ErrorContains(t, err, "failed to log reconciliation action")

	var updated BankStatement
	require.NoError(t, app.db.First(&updated, "id = ?", statement.ID).Error)
	require.Equal(t, bankStatementStatusReconciled, updated.Status)
	require.NotNil(t, updated.ReconciledAt)
	require.Equal(t, "admin", updated.ReconciledBy)
}

func TestForceReimportStatement_RollsBackWhenAuditLogWriteFails(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&BankStatement{}, &BankStatementLine{}, &StatementHash{}, &BankReconciliationAuditLog{}))
	dropBankReconciliationAuditLogTable(t, app)

	statement := BankStatement{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankAccountID:   "acct-1",
		StatementNumber: "STMT-FORCE-ROLLBACK-001",
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
		Description:     "Rollback line",
		Debit:           42,
		Balance:         958,
	}
	require.NoError(t, app.db.Create(&line).Error)
	require.NoError(t, app.db.Create(&StatementHash{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BankStatementID: statement.ID,
		StatementHash:   "abc123",
	}).Error)

	err := app.ForceReimportStatement(statement.ID, "admin", "audit regression")
	require.ErrorContains(t, err, "failed to log reconciliation action")

	var statementCount int64
	require.NoError(t, app.db.Model(&BankStatement{}).Where("id = ?", statement.ID).Count(&statementCount).Error)
	require.EqualValues(t, 1, statementCount)

	var lineCount int64
	require.NoError(t, app.db.Model(&BankStatementLine{}).Where("bank_statement_id = ?", statement.ID).Count(&lineCount).Error)
	require.EqualValues(t, 1, lineCount)

	var hashCount int64
	require.NoError(t, app.db.Model(&StatementHash{}).Where("bank_statement_id = ?", statement.ID).Count(&hashCount).Error)
	require.EqualValues(t, 1, hashCount)
}
