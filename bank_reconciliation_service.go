// =============================================================================
// BANK RECONCILIATION SERVICE
//
// MISSION: Core service for bank statement reconciliation as SSOT
// FEATURES: Statement management, matching, cash position, validation
// =============================================================================

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// =============================================================================
// STATEMENT MANAGEMENT
// =============================================================================

// GetBankStatements retrieves all statements for a bank account
func (a *App) GetBankStatements(bankAccountID string) ([]BankStatement, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var statements []BankStatement
	query := a.db.Order("statement_date DESC")
	if bankAccountID != "" {
		query = query.Where("bank_account_id = ?", bankAccountID)
	}

	if err := query.Find(&statements).Error; err != nil {
		log.Printf("GetBankStatements error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	return statements, nil
}

// GetBankStatementByID retrieves a single statement with all lines
func (a *App) GetBankStatementByID(id string) (*BankStatement, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.Preload("Lines").First(&statement, "id = ?", id).Error; err != nil {
		log.Printf("GetBankStatementByID error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	return &statement, nil
}

// CreateBankStatement creates a new bank statement
func (a *App) CreateBankStatement(statement *BankStatement) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if strings.TrimSpace(statement.Status) == "" {
		statement.Status = bankStatementStatusImported
	} else {
		normalizedStatus, err := normalizeBankStatementStatus(statement.Status)
		if err != nil {
			return err
		}
		statement.Status = normalizedStatus
	}

	if err := a.db.Create(statement).Error; err != nil {
		log.Printf("CreateBankStatement error: %v", err)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	log.Printf("📊 Bank statement created: %s (Period: %s to %s)",
		statement.StatementNumber,
		statement.PeriodStart.Format("2006-01-02"),
		statement.PeriodEnd.Format("2006-01-02"))

	return nil
}

// UpdateBankStatement updates statement fields
func (a *App) UpdateBankStatement(id string, updates map[string]interface{}) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	statement, err := a.getBankStatementByIDForWorkflow(id)
	if err != nil {
		return err
	}
	if err := ensureBankStatementMutable(*statement, "edit this statement"); err != nil {
		return err
	}
	if err := normalizeBankStatementUpdates(updates); err != nil {
		return err
	}

	result := a.db.Model(&BankStatement{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		log.Printf("UpdateBankStatement error: %v", result.Error)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("statement not found: %s", id)
	}

	return nil
}

// DeleteBankStatement soft-deletes a statement
func (a *App) DeleteBankStatement(id string) error {
	if err := a.requirePermission("finance:delete"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	statement, err := a.getBankStatementByIDForWorkflow(id)
	if err != nil {
		return err
	}
	if err := ensureBankStatementMutable(*statement, "delete this statement"); err != nil {
		return err
	}

	result := a.db.Delete(&BankStatement{}, "id = ?", id)
	if result.Error != nil {
		log.Printf("DeleteBankStatement error: %v", result.Error)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	log.Printf("🗑️ Bank statement deleted: %s", id)
	return nil
}

// =============================================================================
// STATEMENT LINES
// =============================================================================

// GetBankStatementLines retrieves all lines for a statement
func (a *App) GetBankStatementLines(statementID string) ([]BankStatementLine, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var lines []BankStatementLine
	if err := a.db.Where("bank_statement_id = ?", statementID).
		Order("line_number ASC").
		Find(&lines).Error; err != nil {
		log.Printf("GetBankStatementLines error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	return lines, nil
}

// GetUnmatchedLines retrieves only unmatched lines for a statement
func (a *App) GetUnmatchedLines(statementID string) ([]BankStatementLine, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var lines []BankStatementLine
	if err := a.db.Where("bank_statement_id = ? AND is_matched = ?", statementID, false).
		Order("line_number ASC").
		Find(&lines).Error; err != nil {
		log.Printf("GetUnmatchedLines error: %v", err)
		return nil, fmt.Errorf("operation failed. Please try again or contact support")
	}

	return lines, nil
}

// UpdateBankStatementLine updates a statement line
func (a *App) UpdateBankStatementLine(lineID string, updates map[string]interface{}) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	_, statement, err := a.getBankStatementForLine(lineID)
	if err != nil {
		return err
	}
	if err := ensureBankStatementMutable(*statement, "edit statement lines"); err != nil {
		return err
	}

	result := a.db.Model(&BankStatementLine{}).Where("id = ?", lineID).Updates(updates)
	if result.Error != nil {
		log.Printf("UpdateBankStatementLine error: %v", result.Error)
		return fmt.Errorf("operation failed. Please try again or contact support")
	}

	return nil
}

// CreateBankStatementLine adds a new line to a statement
func (a *App) CreateBankStatementLine(statementID string, line map[string]interface{}) (*BankStatementLine, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	statement, err := a.getBankStatementByIDForWorkflow(statementID)
	if err != nil {
		return nil, err
	}
	if err := ensureBankStatementMutable(*statement, "create statement lines"); err != nil {
		return nil, err
	}

	// Get next line number
	var maxLineNum int
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ?", statementID).
		Select("COALESCE(MAX(line_number), 0)").Scan(&maxLineNum)

	parseFloat := func(v interface{}) float64 {
		switch val := v.(type) {
		case float64:
			return val
		case string:
			f, _ := strconv.ParseFloat(strings.ReplaceAll(val, ",", ""), 64)
			return f
		}
		return 0
	}

	newLine := BankStatementLine{
		Base:            Base{ID: uuid.New().String()},
		BankStatementID: statementID,
		LineNumber:      maxLineNum + 1,
		Description:     fmt.Sprintf("%v", line["description"]),
		Reference:       fmt.Sprintf("%v", line["reference"]),
		Debit:           parseFloat(line["debit"]),
		Credit:          parseFloat(line["credit"]),
		Balance:         parseFloat(line["balance"]),
		MatchType:       "Unmatched",
	}

	if dateStr, ok := line["transaction_date"].(string); ok && dateStr != "" {
		newLine.TransactionDate = parseDate(dateStr)
		newLine.ValueDate = newLine.TransactionDate
	} else {
		newLine.TransactionDate = time.Now()
		newLine.ValueDate = time.Now()
	}

	if err := a.db.Create(&newLine).Error; err != nil {
		return nil, fmt.Errorf("failed to create line: %w", err)
	}

	log.Printf("➕ Bank statement line created: %s in statement %s", newLine.ID, statementID)
	return &newLine, nil
}

// DeleteBankStatementLine soft-deletes a statement line
// SECURITY: Audit log before deletion (financial transaction)
func (a *App) DeleteBankStatementLine(lineID string) error {
	if err := a.requirePermission("finance:delete"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Get line details for audit log
	line, statement, err := a.getBankStatementForLine(lineID)
	if err != nil {
		return err
	}
	if err := ensureBankStatementMutable(*statement, "delete statement lines"); err != nil {
		return err
	}

	// SECURITY: Audit log BEFORE deletion
	log.Printf("🔒 AUDIT: Bank statement line %s deleted (Amount: %.3f BHD, Desc: %s) by user %s",
		lineID, line.Debit+line.Credit, line.Description, a.getCurrentUserID())

	if GlobalAuditLogger != nil {
		GlobalAuditLogger.LogFinancialTransaction(
			a.getCurrentUserID(),
			"bank_line_deleted",
			"bank_statement_line",
			lineID,
			line.Debit+line.Credit,
			"BHD",
			true,
			map[string]interface{}{
				"bank_statement_id": line.BankStatementID,
				"description":       line.Description,
				"reference":         line.Reference,
				"debit":             line.Debit,
				"credit":            line.Credit,
			},
		)
	}

	result := a.db.Delete(&BankStatementLine{}, "id = ?", lineID)
	if result.Error != nil {
		return fmt.Errorf("failed to delete line: %w", result.Error)
	}

	log.Printf("🗑️ Bank statement line deleted: %s", lineID)
	return nil
}

// =============================================================================
// CASH POSITION SSOT
// =============================================================================

// GetCashPosition returns real-time cash position for all bank accounts
func (a *App) GetCashPosition() (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	type AccountPosition struct {
		BankAccountID     string    `json:"bank_account_id"`
		BankName          string    `json:"bank_name"`
		AccountNumber     string    `json:"account_number"`
		Currency          string    `json:"currency"`
		CurrentBalance    float64   `json:"current_balance"`
		LastStatementDate time.Time `json:"last_statement_date"`
	}

	var positions []AccountPosition

	// Use the latest active statement balance per account for finance widgets.
	err := a.db.Raw(`
		SELECT
			ba.id as bank_account_id,
			ba.bank_name,
			ba.account_number,
			ba.currency,
			COALESCE(bs.closing_balance, 0) as current_balance,
			COALESCE(bs.period_end, ba.created_at) as last_statement_date
		FROM company_bank_accounts ba
		LEFT JOIN (
			SELECT bank_account_id, closing_balance, period_end,
				ROW_NUMBER() OVER (PARTITION BY bank_account_id ORDER BY period_end DESC) as rn
			FROM bank_statements
			WHERE status IN ('Imported', 'InProgress', 'Reconciled', 'Verified')
		) bs ON bs.bank_account_id = ba.id AND bs.rn = 1
		WHERE ba.is_active = 1
		ORDER BY ba.display_order
	`).Scan(&positions).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get cash position: %w", err)
	}

	// Calculate totals by currency
	totalBHD := 0.0
	totalEUR := 0.0

	for _, p := range positions {
		if p.Currency == "BHD" {
			totalBHD = roundBHD(totalBHD + p.CurrentBalance)
		} else if p.Currency == "EUR" {
			totalEUR = roundBHD(totalEUR + p.CurrentBalance)
		}
	}

	return map[string]interface{}{
		"accounts":  positions,
		"total_bhd": totalBHD,
		"total_eur": totalEUR,
		"as_of":     time.Now(),
	}, nil
}

// GetCashPositionByAccount returns cash position for a specific account
func (a *App) GetCashPositionByAccount(bankAccountID string) (float64, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return 0, err
	}
	if a.db == nil {
		return 0, fmt.Errorf("database not initialized")
	}

	var balance float64
	err := a.db.Raw(`
		SELECT COALESCE(closing_balance, 0)
		FROM bank_statements
		WHERE bank_account_id = ? AND status IN ('Imported', 'InProgress', 'Reconciled', 'Verified')
		ORDER BY period_end DESC
		LIMIT 1
	`, bankAccountID).Scan(&balance).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get account balance: %w", err)
	}

	return balance, nil
}

// =============================================================================
// RECONCILIATION WORKFLOW
// =============================================================================

// ValidateStatementBalance checks if Opening + Credits - Debits = Closing
func (a *App) ValidateStatementBalance(statementID string) (bool, float64, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return false, 0, err
	}
	if a.db == nil {
		return false, 0, fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return false, 0, fmt.Errorf("statement not found: %w", err)
	}

	// Sum credits and debits from lines
	var totals struct {
		TotalDebits  float64
		TotalCredits float64
	}

	a.db.Model(&BankStatementLine{}).
		Select("COALESCE(SUM(debit), 0) as total_debits, COALESCE(SUM(credit), 0) as total_credits").
		Where("bank_statement_id = ?", statementID).
		Scan(&totals)

	// Calculate expected closing balance with proper BHD precision
	expectedClosing := roundBHD(statement.OpeningBalance + totals.TotalCredits - totals.TotalDebits)
	discrepancy := roundBHD(math.Abs(expectedClosing - statement.ClosingBalance))

	// Allow tolerance of 0.001 BHD for rounding
	isValid := discrepancy <= 0.001

	// Update statement verification status
	a.db.Model(&statement).Updates(map[string]interface{}{
		"balance_verified":   isValid,
		"discrepancy_amount": discrepancy,
		"total_debits":       totals.TotalDebits,
		"total_credits":      totals.TotalCredits,
		"debit_count":        0, // Will be updated by trigger or separate query
		"credit_count":       0,
	})

	if isValid {
		log.Printf("✅ Statement %s balance verified: Opening %.3f + Credits %.3f - Debits %.3f = Closing %.3f",
			statementID, statement.OpeningBalance, totals.TotalCredits, totals.TotalDebits, statement.ClosingBalance)
	} else {
		log.Printf("❌ Statement %s balance mismatch: Expected %.3f, Got %.3f (Discrepancy: %.3f)",
			statementID, expectedClosing, statement.ClosingBalance, discrepancy)
	}

	return isValid, discrepancy, nil
}

// FinalizeReconciliation marks a statement as reconciled
func (a *App) FinalizeReconciliation(statementID string, reconciledBy string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return fmt.Errorf("statement not found: %w", err)
	}

	if isFinalBankStatementStatus(statement.Status) {
		return fmt.Errorf("statement already reconciled")
	}

	// Validate balance before finalizing
	isValid, discrepancy, err := a.ValidateStatementBalance(statementID)
	if err != nil {
		return fmt.Errorf("balance validation failed: %w", err)
	}

	if !isValid {
		return fmt.Errorf("cannot finalize: balance discrepancy of %.3f BHD", discrepancy)
	}

	// Check for unmatched lines
	var unmatchedCount int64
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ? AND is_matched = ?", statementID, false).Count(&unmatchedCount)

	if unmatchedCount > 0 {
		return fmt.Errorf("cannot finalize: %d unmatched transactions", unmatchedCount)
	}

	now := time.Now()
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&statement).Updates(map[string]interface{}{
			"status":        bankStatementStatusReconciled,
			"reconciled_at": now,
			"reconciled_by": reconciledBy,
		}).Error; err != nil {
			return fmt.Errorf("failed to finalize reconciliation: %w", err)
		}

		return a.logReconciliationActionWithDB(
			tx,
			statementID,
			nil,
			"RECONCILE",
			map[string]interface{}{"status": bankStatementStatusReconciled, "unmatched": unmatchedCount},
			reconciledBy,
			false,
			1.0,
			"Statement finalized",
		)
	}); err != nil {
		return err
	}

	log.Printf("✅ Bank statement %s reconciled by %s", statementID, reconciledBy)
	return nil
}

// ReopenReconciliation reverts a reconciled statement to InProgress
func (a *App) ReopenReconciliation(statementID string, user, reason string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return fmt.Errorf("statement not found: %w", err)
	}

	if !isFinalBankStatementStatus(statement.Status) {
		return fmt.Errorf("statement is not reconciled")
	}

	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&statement).Updates(map[string]interface{}{
			"status":        bankStatementStatusInProgress,
			"reconciled_at": nil,
			"reconciled_by": "",
		}).Error; err != nil {
			return fmt.Errorf("failed to reopen reconciliation: %w", err)
		}

		return a.logReconciliationActionWithDB(
			tx,
			statementID,
			nil,
			"REOPEN",
			map[string]interface{}{"previous_status": statement.Status, "reason": reason},
			user,
			false,
			1.0,
			reason,
		)
	}); err != nil {
		return err
	}

	log.Printf("🔓 Bank statement %s reopened by %s: %s", statementID, user, reason)
	return nil
}

// =============================================================================
// SUMMARY & STATS
// =============================================================================

// GetReconciliationSummary returns summary stats for a statement
func (a *App) GetReconciliationSummary(statementID string) (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var statement BankStatement
	if err := a.db.First(&statement, "id = ?", statementID).Error; err != nil {
		return nil, fmt.Errorf("statement not found: %w", err)
	}

	// Count matched and unmatched
	var matchedCount, unmatchedCount int64
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ? AND is_matched = ?", statementID, true).Count(&matchedCount)
	a.db.Model(&BankStatementLine{}).Where("bank_statement_id = ? AND is_matched = ?", statementID, false).Count(&unmatchedCount)

	totalCount := matchedCount + unmatchedCount
	matchedPercent := 0.0
	if totalCount > 0 {
		matchedPercent = float64(matchedCount) / float64(totalCount) * 100
	}

	// Sum amounts
	var amounts struct {
		TotalDebits   float64
		TotalCredits  float64
		MatchedAmount float64
	}
	a.db.Model(&BankStatementLine{}).
		Select("COALESCE(SUM(debit), 0) as total_debits, COALESCE(SUM(credit), 0) as total_credits, COALESCE(SUM(CASE WHEN is_matched THEN CASE WHEN debit > 0 THEN debit ELSE credit END ELSE 0 END), 0) as matched_amount").
		Where("bank_statement_id = ?", statementID).
		Scan(&amounts)

	return map[string]interface{}{
		"statement_id":     statementID,
		"statement_number": statement.StatementNumber,
		"status":           statement.Status,
		"period_start":     statement.PeriodStart,
		"period_end":       statement.PeriodEnd,
		"opening_balance":  statement.OpeningBalance,
		"closing_balance":  statement.ClosingBalance,
		"total_debits":     amounts.TotalDebits,
		"total_credits":    amounts.TotalCredits,
		"total_lines":      totalCount,
		"matched_count":    matchedCount,
		"unmatched_count":  unmatchedCount,
		"matched_percent":  matchedPercent,
		"matched_amount":   amounts.MatchedAmount,
		"balance_verified": statement.BalanceVerified,
		"reconciled_at":    statement.ReconciledAt,
		"reconciled_by":    statement.ReconciledBy,
	}, nil
}

// GetReconciliationStats returns account-level reconciliation statistics
func (a *App) GetReconciliationStats(bankAccountID string) (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Count statements by status
	var stats struct {
		TotalStatements      int64
		ReconciledStatements int64
		InProgressStatements int64
		ImportedStatements   int64
	}

	scopedQuery := func() *gorm.DB {
		query := a.db.Model(&BankStatement{})
		if bankAccountID != "" {
			query = query.Where("bank_account_id = ?", bankAccountID)
		}
		return query
	}

	scopedQuery().Count(&stats.TotalStatements)
	scopedQuery().Where("status = ?", bankStatementStatusReconciled).Count(&stats.ReconciledStatements)
	scopedQuery().Where("status = ?", bankStatementStatusInProgress).Count(&stats.InProgressStatements)
	scopedQuery().Where("status = ?", bankStatementStatusImported).Count(&stats.ImportedStatements)

	// Get latest statement date
	var latestDate time.Time
	scopedQuery().
		Select("MAX(period_end)").
		Scan(&latestDate)

	return map[string]interface{}{
		"bank_account_id":        bankAccountID,
		"total_statements":       stats.TotalStatements,
		"reconciled_statements":  stats.ReconciledStatements,
		"in_progress_statements": stats.InProgressStatements,
		"imported_statements":    stats.ImportedStatements,
		"latest_statement_date":  latestDate,
	}, nil
}

// =============================================================================
// HELPERS
// =============================================================================

// roundBHD rounds a float to 3 decimal places for BHD precision
func roundBHD(amount float64) float64 {
	return math.Round(amount*1000) / 1000
}

// formatBHD formats a float to BHD with 3 decimal places
func formatBHD(amount float64) string {
	return fmt.Sprintf("%.3f", roundBHD(amount))
}

// parseJSONArray parses a JSON array string to []string
func parseJSONArray(jsonStr string) []string {
	if jsonStr == "" {
		return []string{}
	}
	var result []string
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return []string{}
	}
	return result
}

// toJSONArray converts []string to JSON array string
func toJSONArray(arr []string) string {
	if len(arr) == 0 {
		return "[]"
	}
	data, _ := json.Marshal(arr)
	return string(data)
}
