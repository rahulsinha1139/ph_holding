package main

import (
	"fmt"
	"strings"
)

const (
	bankStatementStatusImported   = "Imported"
	bankStatementStatusInProgress = "InProgress"
	bankStatementStatusReconciled = "Reconciled"
	bankStatementStatusVerified   = "Verified"
	bankStatementStatusCancelled  = "Cancelled"
)

func normalizeBankStatementStatus(raw string) (string, error) {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(raw), " ", ""))
	switch normalized {
	case strings.ToLower(bankStatementStatusImported):
		return bankStatementStatusImported, nil
	case strings.ToLower(bankStatementStatusInProgress):
		return bankStatementStatusInProgress, nil
	case strings.ToLower(bankStatementStatusReconciled):
		return bankStatementStatusReconciled, nil
	case strings.ToLower(bankStatementStatusVerified):
		return bankStatementStatusVerified, nil
	case strings.ToLower(bankStatementStatusCancelled):
		return bankStatementStatusCancelled, nil
	case "":
		return "", fmt.Errorf("statement status cannot be blank")
	default:
		return "", fmt.Errorf("invalid bank statement status: %s", raw)
	}
}

func displayBankStatementStatus(status string) string {
	normalized, err := normalizeBankStatementStatus(status)
	if err != nil {
		return strings.TrimSpace(status)
	}

	switch normalized {
	case bankStatementStatusInProgress:
		return "In Progress"
	default:
		return normalized
	}
}

func isFinalBankStatementStatus(status string) bool {
	switch status {
	case bankStatementStatusReconciled, bankStatementStatusVerified:
		return true
	default:
		return false
	}
}

func isEditableBankStatementStatus(status string) bool {
	switch status {
	case bankStatementStatusImported, bankStatementStatusInProgress:
		return true
	default:
		return false
	}
}

func ensureBankStatementMutable(statement BankStatement, action string) error {
	if !isFinalBankStatementStatus(statement.Status) {
		return nil
	}

	return fmt.Errorf("cannot %s on a %s statement; reopen the reconciliation first", action, strings.ToLower(displayBankStatementStatus(statement.Status)))
}

func normalizeBankStatementUpdates(updates map[string]interface{}) error {
	rawStatus, ok := updates["status"]
	if !ok {
		return nil
	}

	status, err := normalizeBankStatementStatus(fmt.Sprintf("%v", rawStatus))
	if err != nil {
		return err
	}
	if !isEditableBankStatementStatus(status) {
		return fmt.Errorf("cannot set statement status to %s directly; use the reconciliation workflow", displayBankStatementStatus(status))
	}

	updates["status"] = status
	return nil
}

func (a *App) getBankStatementByIDForWorkflow(statementID string) (*BankStatement, error) {
	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return nil, fmt.Errorf("statement not found: %w", err)
	}
	return &statement, nil
}

func (a *App) getBankStatementForLine(lineID string) (*BankStatementLine, *BankStatement, error) {
	var line BankStatementLine
	if err := a.db.First(&line, "id = ?", lineID).Error; err != nil {
		return nil, nil, fmt.Errorf("line not found: %w", err)
	}

	statement, err := a.getBankStatementByIDForWorkflow(line.BankStatementID)
	if err != nil {
		return nil, nil, err
	}

	return &line, statement, nil
}
