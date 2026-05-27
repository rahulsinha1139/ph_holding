package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DataUpdateTableImpact struct {
	Table     string `json:"table"`
	Operation string `json:"operation"`
	Count     int    `json:"count"`
}

type DataUpdateValidationResult struct {
	ManifestID     string                  `json:"manifest_id"`
	Version        string                  `json:"version"`
	Title          string                  `json:"title"`
	Status         string                  `json:"status"`
	Valid          bool                    `json:"valid"`
	OperationCount int                     `json:"operation_count"`
	Issues         []string                `json:"issues"`
	Tables         []DataUpdateTableImpact `json:"tables"`
	AlreadyApplied bool                    `json:"already_applied"`
}

type DataUpdateApplyResult struct {
	ManifestID     string                  `json:"manifest_id"`
	Version        string                  `json:"version"`
	DryRun         bool                    `json:"dry_run"`
	Applied        bool                    `json:"applied"`
	AlreadyApplied bool                    `json:"already_applied"`
	BackupPath     string                  `json:"backup_path"`
	OperationCount int                     `json:"operation_count"`
	Tables         []DataUpdateTableImpact `json:"tables"`
	IntegrityCheck string                  `json:"integrity_check"`
	Error          string                  `json:"error,omitempty"`
}

type DataUpdateAutoApplySummary struct {
	Source              string                  `json:"source"`
	DryRun              bool                    `json:"dry_run"`
	PendingCount        int                     `json:"pending_count"`
	AppliedCount        int                     `json:"applied_count"`
	AlreadyAppliedCount int                     `json:"already_applied_count"`
	FailedCount         int                     `json:"failed_count"`
	Results             []DataUpdateApplyResult `json:"results"`
	Error               string                  `json:"error,omitempty"`
}

var dataUpdateAllowedTables = map[string]bool{
	"customers":                           true,
	"customer_contacts":                   true,
	"opportunities":                       true,
	"rfq_data":                            true,
	"costing_sheet_data":                  true,
	"costing_line_items":                  true,
	"offers":                              true,
	"offer_items":                         true,
	"orders":                              true,
	"order_items":                         true,
	"intelligence_opportunity_enrichment": true,
	"intelligence_line_item_enrichment":   true,
	"intelligence_order_enrichment":       true,
}

func (a *App) EnsureAdminDataUpdateFoundation() error {
	if a == nil || a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.db.AutoMigrate(
		&DataUpdateManifest{},
		&DataUpdateOperation{},
		&DataUpdateReceipt{},
		&IntelligenceOpportunityEnrichment{},
		&IntelligenceLineItemEnrichment{},
		&IntelligenceOrderEnrichment{},
	); err != nil {
		return fmt.Errorf("admin data update migration failed: %w", err)
	}

	for _, table := range []string{
		"intelligence_opportunity_enrichment",
		"intelligence_line_item_enrichment",
		"intelligence_order_enrichment",
	} {
		a.ensureSyncBaseColumns(table)
		a.backfillDataUpdateTimestamps(table)
	}
	return nil
}

func (a *App) ensureSyncBaseColumns(table string) {
	if a == nil || a.db == nil || !a.db.Migrator().HasTable(table) {
		return
	}
	a.addColumnIfNotExists(table, "created_at", "DATETIME")
	a.addColumnIfNotExists(table, "updated_at", "DATETIME")
	a.addColumnIfNotExists(table, "deleted_at", "DATETIME")
	a.addColumnIfNotExists(table, "version", "INTEGER DEFAULT 1")
	a.addColumnIfNotExists(table, "created_by", "TEXT")
}

func (a *App) backfillDataUpdateTimestamps(table string) {
	if a == nil || a.db == nil || !a.db.Migrator().HasTable(table) {
		return
	}
	if a.hasColumn(table, "enriched_at") {
		_ = a.db.Exec(fmt.Sprintf("UPDATE %s SET created_at = COALESCE(NULLIF(enriched_at, ''), CURRENT_TIMESTAMP) WHERE created_at IS NULL", table)).Error
		_ = a.db.Exec(fmt.Sprintf("UPDATE %s SET updated_at = COALESCE(NULLIF(enriched_at, ''), CURRENT_TIMESTAMP) WHERE updated_at IS NULL", table)).Error
	} else {
		_ = a.db.Exec(fmt.Sprintf("UPDATE %s SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL", table)).Error
		_ = a.db.Exec(fmt.Sprintf("UPDATE %s SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL", table)).Error
	}
	_ = a.db.Exec(fmt.Sprintf("UPDATE %s SET version = 1 WHERE version IS NULL OR version = 0", table)).Error
}

func (a *App) ListPendingDataUpdateManifests() ([]DataUpdateManifest, error) {
	if err := a.requirePermission("settings:view"); err != nil {
		return nil, err
	}
	if err := a.EnsureAdminDataUpdateFoundation(); err != nil {
		return nil, err
	}
	return a.listPendingDataUpdateManifestsInternal()
}

func (a *App) listPendingDataUpdateManifestsInternal() ([]DataUpdateManifest, error) {
	if a == nil || a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	deviceID := dataUpdateDeviceID(a)
	var manifests []DataUpdateManifest
	err := a.db.
		Where("LOWER(status) IN ?", []string{"approved", "published"}).
		Where("published_at IS NULL OR published_at <= ?", time.Now()).
		Where(`NOT EXISTS (
			SELECT 1 FROM data_update_receipts r
			WHERE r.manifest_id = data_update_manifests.id
			  AND r.device_id = ?
			  AND LOWER(r.status) = 'applied'
			  AND r.deleted_at IS NULL
		)`, deviceID).
		Order("published_at ASC, created_at ASC").
		Find(&manifests).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list pending data updates: %w", err)
	}
	return manifests, nil
}

func (a *App) ApplyPendingDataUpdatesNow(dryRun bool) (DataUpdateAutoApplySummary, error) {
	if err := a.requirePermission("settings:update"); err != nil {
		return DataUpdateAutoApplySummary{}, err
	}
	return a.applyPendingDataUpdatesInternal(dryRun, "manual")
}

func (a *App) ValidateDataUpdateManifest(manifestID string) (DataUpdateValidationResult, error) {
	if err := a.requirePermission("settings:view"); err != nil {
		return DataUpdateValidationResult{}, err
	}
	if err := a.EnsureAdminDataUpdateFoundation(); err != nil {
		return DataUpdateValidationResult{}, err
	}
	return a.validateDataUpdateManifestInternal(strings.TrimSpace(manifestID))
}

func (a *App) ApplyDataUpdateManifest(manifestID string, dryRun bool) (DataUpdateApplyResult, error) {
	if err := a.requirePermission("settings:update"); err != nil {
		return DataUpdateApplyResult{}, err
	}
	return a.applyDataUpdateManifestInternal(manifestID, dryRun)
}

func (a *App) applyPendingDataUpdatesInternal(dryRun bool, source string) (DataUpdateAutoApplySummary, error) {
	summary := DataUpdateAutoApplySummary{
		Source: strings.TrimSpace(source),
		DryRun: dryRun,
	}
	if summary.Source == "" {
		summary.Source = "background"
	}
	if err := a.EnsureAdminDataUpdateFoundation(); err != nil {
		summary.Error = err.Error()
		return summary, err
	}
	pending, err := a.listPendingDataUpdateManifestsInternal()
	if err != nil {
		summary.Error = err.Error()
		return summary, err
	}
	summary.PendingCount = len(pending)
	for _, manifest := range pending {
		result, err := a.applyDataUpdateManifestInternal(manifest.ID, dryRun)
		summary.Results = append(summary.Results, result)
		if result.AlreadyApplied {
			summary.AlreadyAppliedCount++
		}
		if err != nil {
			summary.FailedCount++
			summary.Error = err.Error()
			return summary, err
		}
		if result.Applied && !result.AlreadyApplied && !dryRun {
			summary.AppliedCount++
		}
	}
	return summary, nil
}

func (a *App) applyDataUpdateManifestInternal(manifestID string, dryRun bool) (DataUpdateApplyResult, error) {
	if err := a.EnsureAdminDataUpdateFoundation(); err != nil {
		return DataUpdateApplyResult{}, err
	}

	validation, err := a.validateDataUpdateManifestInternal(strings.TrimSpace(manifestID))
	if err != nil {
		return DataUpdateApplyResult{}, err
	}
	result := DataUpdateApplyResult{
		ManifestID:     validation.ManifestID,
		Version:        validation.Version,
		DryRun:         dryRun,
		AlreadyApplied: validation.AlreadyApplied,
		OperationCount: validation.OperationCount,
		Tables:         validation.Tables,
	}
	if validation.AlreadyApplied {
		result.Applied = true
		return result, nil
	}
	if !validation.Valid {
		result.Error = strings.Join(validation.Issues, "; ")
		return result, fmt.Errorf("data update validation failed: %s", result.Error)
	}
	if dryRun {
		return result, nil
	}

	var manifest DataUpdateManifest
	if err := a.db.First(&manifest, "id = ?", validation.ManifestID).Error; err != nil {
		return result, fmt.Errorf("failed to load data update manifest: %w", err)
	}
	status := strings.ToLower(strings.TrimSpace(manifest.Status))
	if status != "approved" && status != "published" {
		return result, fmt.Errorf("data update %s is not approved/published", manifest.VersionLabel)
	}

	backupPath, err := a.backupDatabaseInternal()
	result.BackupPath = backupPath
	if err != nil {
		result.Error = err.Error()
		a.recordDataUpdateReceipt(manifest, "failed", result, err.Error())
		return result, fmt.Errorf("backup before data update failed: %w", err)
	}

	var ops []DataUpdateOperation
	if err := a.db.Where("manifest_id = ?", manifest.ID).Order("sequence ASC, id ASC").Find(&ops).Error; err != nil {
		result.Error = err.Error()
		a.recordDataUpdateReceipt(manifest, "failed", result, err.Error())
		return result, err
	}

	applyErr := a.db.Transaction(func(tx *gorm.DB) error {
		for _, op := range ops {
			record, err := dataUpdateOperationRecord(op)
			if err != nil {
				return err
			}
			if err := applyDataUpdateOperation(tx, op, record); err != nil {
				return err
			}
		}
		return nil
	})
	if applyErr != nil {
		result.Error = applyErr.Error()
		a.recordDataUpdateReceipt(manifest, "failed", result, applyErr.Error())
		return result, fmt.Errorf("failed to apply data update: %w", applyErr)
	}

	result.IntegrityCheck = a.runIntegrityCheck()
	if result.IntegrityCheck != "ok" {
		result.Error = result.IntegrityCheck
		a.recordDataUpdateReceipt(manifest, "failed", result, result.IntegrityCheck)
		return result, fmt.Errorf("database integrity check failed after data update: %s", result.IntegrityCheck)
	}

	result.Applied = true
	a.recordDataUpdateReceipt(manifest, "applied", result, "")
	return result, nil
}

func (a *App) validateDataUpdateManifestInternal(manifestID string) (DataUpdateValidationResult, error) {
	if a == nil || a.db == nil {
		return DataUpdateValidationResult{}, fmt.Errorf("database not initialized")
	}
	if manifestID == "" {
		return DataUpdateValidationResult{}, fmt.Errorf("manifest id is required")
	}
	var manifest DataUpdateManifest
	if err := a.db.First(&manifest, "id = ?", manifestID).Error; err != nil {
		return DataUpdateValidationResult{}, fmt.Errorf("data update manifest not found: %w", err)
	}
	result := DataUpdateValidationResult{
		ManifestID: manifest.ID,
		Version:    manifest.VersionLabel,
		Title:      manifest.Title,
		Status:     manifest.Status,
		Valid:      true,
	}
	var receipt DataUpdateReceipt
	if err := a.db.Where("manifest_id = ? AND device_id = ? AND LOWER(status) = ?", manifest.ID, dataUpdateDeviceID(a), "applied").First(&receipt).Error; err == nil {
		result.AlreadyApplied = true
	}

	var ops []DataUpdateOperation
	if err := a.db.Where("manifest_id = ?", manifest.ID).Order("sequence ASC, id ASC").Find(&ops).Error; err != nil {
		return result, fmt.Errorf("failed to load data update operations: %w", err)
	}
	result.OperationCount = len(ops)
	if len(ops) == 0 {
		result.Issues = append(result.Issues, "manifest has no operations")
	}

	tableCounts := make(map[string]map[string]int)
	for _, op := range ops {
		table := strings.ToLower(strings.TrimSpace(op.TargetTable))
		operation := strings.ToLower(strings.TrimSpace(op.Operation))
		if operation == "" {
			operation = "upsert"
		}
		if operation != "upsert" {
			result.Issues = append(result.Issues, fmt.Sprintf("operation %d uses unsupported operation %q", op.Sequence, op.Operation))
		}
		if !dataUpdateAllowedTables[table] {
			result.Issues = append(result.Issues, fmt.Sprintf("operation %d targets non-allowlisted table %q", op.Sequence, op.TargetTable))
		} else if !a.db.Migrator().HasTable(table) {
			result.Issues = append(result.Issues, fmt.Sprintf("operation %d targets missing local table %q", op.Sequence, table))
		}
		record, err := dataUpdateOperationRecord(op)
		if err != nil {
			result.Issues = append(result.Issues, fmt.Sprintf("operation %d has invalid row_json: %v", op.Sequence, err))
		} else {
			idValue, hasID := record["id"]
			if !hasID || !syncValuePresent(idValue) {
				if strings.TrimSpace(op.PrimaryKey) == "" {
					result.Issues = append(result.Issues, fmt.Sprintf("operation %d has no id or primary_key", op.Sequence))
				}
			}
			if strings.TrimSpace(op.RowHash) != "" && !dataUpdateRowHashMatches(record, op.RowJSON, op.RowHash) {
				result.Issues = append(result.Issues, fmt.Sprintf("operation %d row hash mismatch", op.Sequence))
			}
		}
		if tableCounts[table] == nil {
			tableCounts[table] = make(map[string]int)
		}
		tableCounts[table][operation]++
	}
	if strings.TrimSpace(manifest.ManifestHash) != "" && !dataUpdateManifestHashMatches(ops, manifest.ManifestHash) {
		result.Issues = append(result.Issues, "manifest hash mismatch")
	}
	for table, byOperation := range tableCounts {
		for operation, count := range byOperation {
			result.Tables = append(result.Tables, DataUpdateTableImpact{Table: table, Operation: operation, Count: count})
		}
	}
	sort.Slice(result.Tables, func(i, j int) bool {
		if result.Tables[i].Table == result.Tables[j].Table {
			return result.Tables[i].Operation < result.Tables[j].Operation
		}
		return result.Tables[i].Table < result.Tables[j].Table
	})
	result.Valid = len(result.Issues) == 0
	return result, nil
}

func dataUpdateOperationRecord(op DataUpdateOperation) (map[string]interface{}, error) {
	var record map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(op.RowJSON)), &record); err != nil {
		return nil, err
	}
	if record == nil {
		record = map[string]interface{}{}
	}
	if _, ok := record["id"]; !ok && strings.TrimSpace(op.PrimaryKey) != "" {
		record["id"] = strings.TrimSpace(op.PrimaryKey)
	}
	return record, nil
}

func applyDataUpdateOperation(tx *gorm.DB, op DataUpdateOperation, record map[string]interface{}) error {
	if tx == nil {
		return fmt.Errorf("database transaction is not initialized")
	}
	table := strings.ToLower(strings.TrimSpace(op.TargetTable))
	if !dataUpdateAllowedTables[table] {
		return fmt.Errorf("table %q is not allowlisted for admin data updates", table)
	}
	if strings.ToLower(strings.TrimSpace(op.Operation)) != "" && strings.ToLower(strings.TrimSpace(op.Operation)) != "upsert" {
		return fmt.Errorf("operation %q is not supported for %s", op.Operation, table)
	}
	columnTypes, err := syncDBColumnTypes(tx, table)
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	record = normalizeRecordForRemoteSyncSchema(record, columnTypes)
	if _, ok := record["id"]; !ok || !syncValuePresent(record["id"]) {
		return fmt.Errorf("%s operation %d has no id", table, op.Sequence)
	}
	now := time.Now()
	if syncColumnExists(columnTypes, "created_at") {
		if _, ok := record["created_at"]; !ok {
			record["created_at"] = now
		}
	}
	if syncColumnExists(columnTypes, "updated_at") {
		record["updated_at"] = now
	}
	return syncUpsertRecordByConflict(tx, table, record, []clause.Column{{Name: "id"}})
}

func (a *App) recordDataUpdateReceipt(manifest DataUpdateManifest, status string, result DataUpdateApplyResult, errorText string) {
	if a == nil || a.db == nil {
		return
	}
	now := time.Now()
	deviceID := dataUpdateDeviceID(a)
	var receipt DataUpdateReceipt
	err := a.db.Where("manifest_id = ? AND device_id = ?", manifest.ID, deviceID).First(&receipt).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		receipt = DataUpdateReceipt{ManifestID: manifest.ID, DeviceID: deviceID}
	}
	receipt.Status = status
	receipt.BackupPath = result.BackupPath
	receipt.Error = strings.TrimSpace(errorText)
	receipt.OperationCount = result.OperationCount
	receipt.ManifestHash = manifest.ManifestHash
	if status == "applied" {
		receipt.AppliedAt = &now
	}
	receipt.UpdatedAt = now
	if receipt.CreatedAt.IsZero() {
		receipt.CreatedAt = now
	}
	if err := a.db.Save(&receipt).Error; err == nil {
		a.trySyncDataUpdateReceiptToRemote(receipt)
	}
}

func (a *App) trySyncDataUpdateReceiptToRemote(receipt DataUpdateReceipt) {
	if a == nil || a.dbManager == nil {
		return
	}
	remote := a.dbManager.Remote()
	if remote == nil || !remote.Migrator().HasTable("data_update_receipts") {
		return
	}
	_ = remote.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "device_id"}, {Name: "manifest_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "backup_path", "applied_at", "error", "operation_count", "manifest_hash", "updated_at",
		}),
	}).Create(&receipt).Error
}

func dataUpdateDeviceID(a *App) string {
	if a != nil && strings.TrimSpace(a.currentUserID) != "" {
		return strings.TrimSpace(a.currentUserID) + "@" + GetMachineID()
	}
	return GetMachineID()
}

func dataUpdateRowHashMatches(record map[string]interface{}, raw string, expected string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return true
	}
	return expected == dataUpdateHashRecord(record) || expected == dataUpdateHashString(strings.TrimSpace(raw))
}

func dataUpdateManifestHashMatches(ops []DataUpdateOperation, expected string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	if expected == "" {
		return true
	}
	return expected == dataUpdateManifestDigest(ops)
}

func dataUpdateManifestDigest(ops []DataUpdateOperation) string {
	type digestRow struct {
		Sequence   int    `json:"sequence"`
		TableName  string `json:"table_name"`
		Operation  string `json:"operation"`
		PrimaryKey string `json:"primary_key"`
		RowHash    string `json:"row_hash"`
	}
	rows := make([]digestRow, 0, len(ops))
	for _, op := range ops {
		rowHash := strings.ToLower(strings.TrimSpace(op.RowHash))
		if rowHash == "" {
			if record, err := dataUpdateOperationRecord(op); err == nil {
				rowHash = dataUpdateHashRecord(record)
			} else {
				rowHash = dataUpdateHashString(op.RowJSON)
			}
		}
		rows = append(rows, digestRow{
			Sequence:   op.Sequence,
			TableName:  strings.ToLower(strings.TrimSpace(op.TargetTable)),
			Operation:  firstNonEmpty(strings.ToLower(strings.TrimSpace(op.Operation)), "upsert"),
			PrimaryKey: strings.TrimSpace(op.PrimaryKey),
			RowHash:    rowHash,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Sequence == rows[j].Sequence {
			return rows[i].TableName < rows[j].TableName
		}
		return rows[i].Sequence < rows[j].Sequence
	})
	encoded, _ := json.Marshal(rows)
	return dataUpdateHashString(string(encoded))
}

func dataUpdateHashRecord(record map[string]interface{}) string {
	encoded, _ := json.Marshal(record)
	return dataUpdateHashString(string(encoded))
}

func dataUpdateHashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
