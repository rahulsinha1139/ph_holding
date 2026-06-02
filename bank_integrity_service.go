// =============================================================================
// BANK INTEGRITY SERVICE
//
// MISSION: Ensure data integrity for bank reconciliation
// FEATURES: Continuity validation, duplicate detection, audit trail, PDF archival
// =============================================================================

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"path/filepath"
	"time"

	"gorm.io/gorm"
)

// =============================================================================
// STATEMENT CONTINUITY
// =============================================================================

// ValidateStatementContinuity ensures previous closing = new opening
func (a *App) ValidateStatementContinuity(bankAccountID string, newStatement *BankStatement) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	var lastStatement BankStatement
	err := a.db.Where("bank_account_id = ? AND status IN ('Reconciled', 'Verified')", bankAccountID).
		Order("period_end DESC").
		First(&lastStatement).Error

	if err == nil {
		// Previous statement exists - validate continuity
		gap := math.Abs(lastStatement.ClosingBalance - newStatement.OpeningBalance)
		if gap > 0.001 {
			return fmt.Errorf(
				"BALANCE GAP DETECTED: Previous closing (%.3f) ≠ New opening (%.3f). Gap: %.3f %s",
				lastStatement.ClosingBalance,
				newStatement.OpeningBalance,
				gap,
				newStatement.Currency,
			)
		}
	}
	// First statement for this account - no validation needed
	return nil
}

// BalanceGap represents a gap in balance continuity
type BalanceGap struct {
	FromStatementID string    `json:"from_statement_id"`
	ToStatementID   string    `json:"to_statement_id"`
	FromDate        time.Time `json:"from_date"`
	ToDate          time.Time `json:"to_date"`
	ClosingBalance  float64   `json:"closing_balance"`
	OpeningBalance  float64   `json:"opening_balance"`
	GapAmount       float64   `json:"gap_amount"`
}

// BalanceContinuityReport provides a full continuity audit
type BalanceContinuityReportData struct {
	BankAccountID     string       `json:"bank_account_id"`
	BankName          string       `json:"bank_name"`
	Gaps              []BalanceGap `json:"gaps"`
	TotalGapAmount    float64      `json:"total_gap_amount"`
	IsContinuous      bool         `json:"is_continuous"`
	StatementsCovered int          `json:"statements_covered"`
}

// GetBalanceContinuityReport generates a continuity report for an account
func (a *App) GetBalanceContinuityReport(bankAccountID string) (*BalanceContinuityReportData, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var bankAccount CompanyBankAccount
	if err := a.db.First(&bankAccount, "id = ?", bankAccountID).Error; err != nil {
		return nil, fmt.Errorf("bank account not found: %w", err)
	}

	var statements []BankStatement
	a.db.Where("bank_account_id = ?", bankAccountID).
		Order("period_end ASC").
		Find(&statements)

	report := &BalanceContinuityReportData{
		BankAccountID:     bankAccountID,
		BankName:          bankAccount.BankName,
		IsContinuous:      true,
		StatementsCovered: len(statements),
	}

	for i := 1; i < len(statements); i++ {
		prev := statements[i-1]
		curr := statements[i]

		gap := curr.OpeningBalance - prev.ClosingBalance
		if math.Abs(gap) > 0.001 {
			report.Gaps = append(report.Gaps, BalanceGap{
				FromStatementID: prev.ID,
				ToStatementID:   curr.ID,
				FromDate:        prev.PeriodEnd,
				ToDate:          curr.PeriodStart,
				ClosingBalance:  prev.ClosingBalance,
				OpeningBalance:  curr.OpeningBalance,
				GapAmount:       gap,
			})
			report.TotalGapAmount += math.Abs(gap)
			report.IsContinuous = false
		}
	}

	return report, nil
}

// =============================================================================
// DUPLICATE DETECTION
// =============================================================================

// ComputeStatementHash generates a unique SHA-256 hash for a statement
func (a *App) ComputeStatementHash(statement *BankStatement, lines []BankStatementLine) string {
	if err := a.requirePermission("finance:view"); err != nil {
		return ""
	}
	firstRef := ""
	lastRef := ""
	if len(lines) > 0 {
		firstRef = lines[0].Reference
		lastRef = lines[len(lines)-1].Reference
	}

	data := fmt.Sprintf("%s|%s|%s|%d|%.3f|%s|%s",
		statement.BankAccountID,
		statement.PeriodStart.Format("2006-01-02"),
		statement.PeriodEnd.Format("2006-01-02"),
		len(lines),
		statement.ClosingBalance,
		firstRef,
		lastRef,
	)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// CheckDuplicateStatement checks if a statement already exists
func (a *App) CheckDuplicateStatement(statement *BankStatement, lines []BankStatementLine) (bool, *StatementHash, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return false, nil, err
	}
	if a.db == nil {
		return false, nil, fmt.Errorf("database not initialized")
	}

	hash := a.ComputeStatementHash(statement, lines)

	var existing StatementHash
	err := a.db.Where("statement_hash = ?", hash).First(&existing).Error
	if err == nil {
		// Duplicate found!
		return true, &existing, fmt.Errorf(
			"DUPLICATE STATEMENT: This statement was already imported on %s (Statement ID: %s)",
			existing.ImportedAt.Format("2006-01-02 15:04"),
			existing.BankStatementID,
		)
	}

	return false, nil, nil
}

// SaveStatementHash saves the hash for a new statement
func (a *App) SaveStatementHash(statement *BankStatement, lines []BankStatementLine) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	hash := a.ComputeStatementHash(statement, lines)

	stmtHash := StatementHash{
		BankAccountID:    statement.BankAccountID,
		StatementHash:    hash,
		PeriodStart:      statement.PeriodStart,
		PeriodEnd:        statement.PeriodEnd,
		TransactionCount: len(lines),
		ClosingBalance:   statement.ClosingBalance,
		ImportedAt:       time.Now(),
		BankStatementID:  statement.ID,
	}

	return a.db.Create(&stmtHash).Error
}

// ForceReimportStatement allows re-import with audit trail
func (a *App) ForceReimportStatement(statementID string, user, reason string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("bank_statement_id = ?", statementID).Delete(&StatementHash{}).Error; err != nil {
			return fmt.Errorf("failed to delete statement hash: %w", err)
		}
		if err := tx.Where("bank_statement_id = ?", statementID).Delete(&BankStatementLine{}).Error; err != nil {
			return fmt.Errorf("failed to delete statement lines: %w", err)
		}
		if err := tx.Delete(&BankStatement{}, "id = ?", statementID).Error; err != nil {
			return fmt.Errorf("failed to delete statement: %w", err)
		}

		return a.logReconciliationActionWithDB(
			tx,
			statementID,
			nil,
			"FORCE_REIMPORT",
			map[string]interface{}{"reason": reason},
			user,
			false,
			1.0,
			reason,
		)
	}); err != nil {
		return err
	}

	log.Printf("🔄 Statement %s force reimported by %s: %s", statementID, user, reason)
	return nil
}

// =============================================================================
// AUDIT TRAIL
// =============================================================================

// LogReconciliationAction logs any reconciliation action
func (a *App) LogReconciliationAction(
	statementID string,
	lineID *string,
	action string,
	detail interface{},
	user string,
	isAuto bool,
	confidence float64,
	reason string,
) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("failed to encode reconciliation audit detail for %s: %w", action, err)
	}

	logEntry := BankReconciliationAuditLog{
		BankStatementID:     statementID,
		BankStatementLineID: lineID,
		Action:              action,
		ActionDetail:        string(detailJSON),
		PerformedBy:         user,
		PerformedAt:         time.Now(),
		IsAutomatic:         isAuto,
		ConfidenceScore:     confidence,
		Reason:              reason,
	}

	if err := a.db.Create(&logEntry).Error; err != nil {
		log.Printf("⚠️ Failed to log reconciliation action: %v", err)
		return err
	}

	return nil
}

func (a *App) logReconciliationActionWithDB(
	db *gorm.DB,
	statementID string,
	lineID *string,
	action string,
	detail interface{},
	user string,
	isAuto bool,
	confidence float64,
	reason string,
) error {
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	detailJSON, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("failed to encode reconciliation audit detail for %s: %w", action, err)
	}

	logEntry := BankReconciliationAuditLog{
		BankStatementID:     statementID,
		BankStatementLineID: lineID,
		Action:              action,
		ActionDetail:        string(detailJSON),
		PerformedBy:         user,
		PerformedAt:         time.Now(),
		IsAutomatic:         isAuto,
		ConfidenceScore:     confidence,
		Reason:              reason,
	}

	if err := db.Create(&logEntry).Error; err != nil {
		return fmt.Errorf("failed to log reconciliation action %s for statement %s: %w", action, statementID, err)
	}

	return nil
}

// GetAuditTrail retrieves all audit logs for a statement
func (a *App) GetAuditTrail(statementID string) ([]BankReconciliationAuditLog, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var logs []BankReconciliationAuditLog
	err := a.db.Where("bank_statement_id = ?", statementID).
		Order("performed_at ASC").
		Find(&logs).Error

	return logs, err
}

// GetAuditTrailByDateRange retrieves audit logs for a date range
func (a *App) GetAuditTrailByDateRange(bankAccountID string, startDate, endDate time.Time) ([]BankReconciliationAuditLog, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var logs []BankReconciliationAuditLog
	query := a.db.Where("performed_at BETWEEN ? AND ?", startDate, endDate)

	if bankAccountID != "" {
		// Join with statements to filter by bank account
		query = query.Joins("JOIN bank_statements bs ON bs.id = bank_reconciliation_audit_logs.bank_statement_id").
			Where("bs.bank_account_id = ?", bankAccountID)
	}

	err := query.Order("performed_at ASC").Find(&logs).Error
	return logs, err
}

// ReverseAction marks an audit log entry as reversed
func (a *App) ReverseAction(logID string, user, reason string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	now := time.Now()
	return a.db.Model(&BankReconciliationAuditLog{}).
		Where("id = ?", logID).
		Updates(map[string]interface{}{
			"is_reversed":     true,
			"reversed_by":     user,
			"reversed_at":     now,
			"reversal_reason": reason,
		}).Error
}

// =============================================================================
// PDF ARCHIVAL
// =============================================================================

// ArchiveStatementPDF stores the original PDF for audit trail
func (a *App) ArchiveStatementPDF(statementID string, filePath string) error {
	if err := a.requirePermission("finance:create"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file info: %w", err)
	}

	// Calculate file hash
	fileHash, err := a.GetFileHash(filePath)
	if err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}

	// Create archive directory
	archiveDir := filepath.Join("data", "bank_statements", statementID)
	if err := os.MkdirAll(archiveDir, 0755); err != nil {
		log.Printf("⚠️ Failed to create archive directory: %v", err)
		// Continue even if directory creation fails - we'll store the path
	}

	// Copy file to archive
	destPath := filepath.Join(archiveDir, filepath.Base(filePath))
	if err := copyFile(filePath, destPath); err != nil {
		log.Printf("⚠️ Failed to archive file: %v", err)
		destPath = filePath // Keep original path if copy fails
	}

	// Determine file type
	ext := filepath.Ext(filePath)
	fileType := "PDF"
	if ext == ".csv" {
		fileType = "CSV"
	} else if ext == ".xls" || ext == ".xlsx" {
		fileType = "XLS"
	}

	// Create archive record
	archive := BankStatementFile{
		BankStatementID: statementID,
		FileName:        filepath.Base(filePath),
		FileType:        fileType,
		FileSize:        fileInfo.Size(),
		FileHash:        fileHash,
		StoragePath:     destPath,
		IsStored:        destPath != filePath,
	}

	if err := a.db.Create(&archive).Error; err != nil {
		return fmt.Errorf("failed to create archive record: %w", err)
	}

	log.Printf("📁 Archived statement file: %s (%d bytes)", filepath.Base(filePath), fileInfo.Size())
	return nil
}

// RetrieveOriginalPDF returns the archived PDF content
func (a *App) RetrieveOriginalPDF(statementID string) ([]byte, string, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, "", err
	}
	if a.db == nil {
		return nil, "", fmt.Errorf("database not initialized")
	}

	var archive BankStatementFile
	if err := a.db.Where("bank_statement_id = ?", statementID).First(&archive).Error; err != nil {
		return nil, "", fmt.Errorf("archived file not found: %w", err)
	}

	data, err := os.ReadFile(archive.StoragePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read archived file: %w", err)
	}

	return data, archive.FileName, nil
}

// GetFileHash calculates SHA-256 hash of a file
func (a *App) GetFileHash(filePath string) (string, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return "", err
	}
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// =============================================================================
// COMPLIANCE REPORTS
// =============================================================================

// GenerateAuditReport creates a comprehensive audit report
func (a *App) GenerateAuditReport(bankAccountID string, startDate, endDate time.Time) (map[string]interface{}, error) {
	if err := a.requirePermission("finance:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	// Get account info
	var bankAccount CompanyBankAccount
	if err := a.db.First(&bankAccount, "id = ?", bankAccountID).Error; err != nil {
		return nil, fmt.Errorf("bank account not found: %w", err)
	}

	// Get statements in period
	var statements []BankStatement
	a.db.Where("bank_account_id = ? AND period_start >= ? AND period_end <= ?",
		bankAccountID, startDate, endDate).
		Order("period_start ASC").
		Find(&statements)

	// Get audit logs
	logs, _ := a.GetAuditTrailByDateRange(bankAccountID, startDate, endDate)

	// Get continuity report
	continuity, _ := a.GetBalanceContinuityReport(bankAccountID)

	// Count by action type
	actionCounts := make(map[string]int)
	for _, log := range logs {
		actionCounts[log.Action]++
	}

	return map[string]interface{}{
		"bank_account":     bankAccount.BankName,
		"account_number":   bankAccount.AccountNumber,
		"period_start":     startDate,
		"period_end":       endDate,
		"statements_count": len(statements),
		"statements":       statements,
		"audit_logs_count": len(logs),
		"action_counts":    actionCounts,
		"is_continuous":    continuity.IsContinuous,
		"gaps":             continuity.Gaps,
		"generated_at":     time.Now(),
	}, nil
}

// =============================================================================
// HELPERS
// =============================================================================

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}
