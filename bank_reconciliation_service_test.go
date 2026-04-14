package main

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestUpdateBankStatementLineRecalculatesAndClearsMatch(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&BankStatement{}, &BankStatementLine{}, &PayrollRun{}, &PayrollPayout{}))

	statementID := uuid.New().String()
	lineID := uuid.New().String()

	statement := BankStatement{
		Base:            Base{ID: statementID},
		BankAccountID:   uuid.New().String(),
		StatementNumber: "OCR-TEST-001",
		StatementDate:   time.Now(),
		PeriodStart:     time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC),
		PeriodEnd:       time.Date(2026, time.January, 31, 0, 0, 0, 0, time.UTC),
		OpeningBalance:  100,
		ClosingBalance:  120,
		Status:          "Reconciled",
		ReconciledBy:    "admin",
		TotalCredits:    20,
		CreditCount:     1,
		BalanceVerified: true,
	}
	require.NoError(t, app.db.Create(&statement).Error)

	line := BankStatementLine{
		Base:              Base{ID: lineID},
		BankStatementID:   statementID,
		LineNumber:        1,
		TransactionDate:   time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
		ValueDate:         time.Date(2026, time.January, 2, 0, 0, 0, 0, time.UTC),
		Description:       "OCR imported receipt",
		Credit:            20,
		Balance:           120,
		IsMatched:         true,
		MatchType:         "Manual",
		MatchConfidence:   1,
		MatchedInvoiceIDs: "invoice-1",
	}
	require.NoError(t, app.db.Create(&line).Error)

	err := app.UpdateBankStatementLine(lineID, map[string]interface{}{
		"debit":  20.0,
		"credit": 0.0,
	})
	require.NoError(t, err)

	var updatedLine BankStatementLine
	require.NoError(t, app.db.First(&updatedLine, "id = ?", lineID).Error)
	require.Equal(t, 20.0, updatedLine.Debit)
	require.Equal(t, 0.0, updatedLine.Credit)
	require.False(t, updatedLine.IsMatched)
	require.Equal(t, "Unmatched", updatedLine.MatchType)
	require.Zero(t, updatedLine.MatchConfidence)
	require.Empty(t, updatedLine.MatchedInvoiceIDs)

	var updatedStatement BankStatement
	require.NoError(t, app.db.First(&updatedStatement, "id = ?", statementID).Error)
	require.Equal(t, 20.0, updatedStatement.TotalDebits)
	require.Equal(t, 0.0, updatedStatement.TotalCredits)
	require.Equal(t, 1, updatedStatement.DebitCount)
	require.Equal(t, 0, updatedStatement.CreditCount)
	require.False(t, updatedStatement.BalanceVerified)
	require.Equal(t, 40.0, updatedStatement.DiscrepancyAmount)
	require.Equal(t, "InProgress", updatedStatement.Status)
	require.Nil(t, updatedStatement.ReconciledAt)
	require.Empty(t, updatedStatement.ReconciledBy)
}
