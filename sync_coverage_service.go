package main

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SyncTableCoverage struct {
	Table              string `json:"table"`
	LocalExists        bool   `json:"local_exists"`
	LocalHasUpdatedAt  bool   `json:"local_has_updated_at"`
	LocalRows          int64  `json:"local_rows"`
	RemoteChecked      bool   `json:"remote_checked"`
	RemoteExists       bool   `json:"remote_exists"`
	RemoteHasUpdatedAt bool   `json:"remote_has_updated_at"`
	Status             string `json:"status"`
	Note               string `json:"note"`
}

type SyncCoverageReport struct {
	GeneratedAt          string              `json:"generated_at"`
	SyncTablesTotal      int                 `json:"sync_tables_total"`
	LocalTablesTotal     int                 `json:"local_tables_total"`
	RemoteOnline         bool                `json:"remote_online"`
	MissingLocalTables   []string            `json:"missing_local_tables"`
	MissingRemoteTables  []string            `json:"missing_remote_tables"`
	UntrackedLocalTables []string            `json:"untracked_local_tables"`
	Tables               []SyncTableCoverage `json:"tables"`
}

func (a *App) MigrateRemoteSchema() (map[string]interface{}, error) {
	if err := a.requirePermission("settings:update"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("local database is not initialized")
	}
	if a.dbManager == nil {
		cfg := LoadSupabaseConfig()
		a.dbManager = NewDBManager(a.db, cfg)
		a.dbManager.canPullActivityMonitoring = a.currentSessionCanAccessActivityMonitoring
	}
	if a.dbManager == nil || !a.dbManager.IsSyncEnabled() {
		return nil, fmt.Errorf("remote sync is not configured")
	}
	if !a.dbManager.IsOnline() {
		if err := a.dbManager.ConnectRemote(); err != nil {
			return nil, err
		}
	}
	if err := a.dbManager.MigrateRemote(); err != nil {
		return nil, err
	}

	report, _ := a.buildSyncCoverageReport()
	return map[string]interface{}{
		"status":                "ok",
		"remote_tables_checked": report.SyncTablesTotal,
		"missing_remote_tables": report.MissingRemoteTables,
		"generated_at":          time.Now().Format(time.RFC3339),
	}, nil
}

func (a *App) GetSyncCoverageReport() (SyncCoverageReport, error) {
	if err := a.requirePermission("settings:view"); err != nil {
		return SyncCoverageReport{}, err
	}
	return a.buildSyncCoverageReport()
}

func (a *App) buildSyncCoverageReport() (SyncCoverageReport, error) {
	report := SyncCoverageReport{
		GeneratedAt:     time.Now().Format(time.RFC3339),
		SyncTablesTotal: len(activeDBSyncTables()),
	}
	if a.db == nil {
		return report, fmt.Errorf("local database is not initialized")
	}

	localTables, err := sqliteTableNames(a.db)
	if err != nil {
		return report, err
	}
	report.LocalTablesTotal = len(localTables)

	localTableSet := make(map[string]bool, len(localTables))
	for _, table := range localTables {
		localTableSet[table] = true
	}

	syncTableSet := make(map[string]bool, len(activeDBSyncTables()))
	remote := (*DBManager)(nil)
	if a.dbManager != nil {
		remote = a.dbManager
		report.RemoteOnline = remote.IsOnline() && remote.Remote() != nil
	}

	for _, table := range activeDBSyncTables() {
		syncTableSet[table] = true
		row := SyncTableCoverage{Table: table}
		row.LocalExists = localTableSet[table]
		if !row.LocalExists {
			row.Status = "missing_local_table"
			row.Note = "Local SQLite table is missing; startup migration/foundation must create it before sync can use it."
			report.MissingLocalTables = append(report.MissingLocalTables, table)
			report.Tables = append(report.Tables, row)
			continue
		}

		localColumnTypes, err := syncDBColumnTypes(a.db, table)
		if err == nil {
			row.LocalHasUpdatedAt = syncColumnExists(localColumnTypes, "updated_at")
		}
		_ = a.db.Table(table).Count(&row.LocalRows).Error

		if remote != nil && report.RemoteOnline {
			row.RemoteChecked = true
			row.RemoteExists = remote.Remote().Migrator().HasTable(table)
			if row.RemoteExists {
				if remoteColumnTypes, err := syncDBColumnTypes(remote.Remote(), table); err == nil {
					row.RemoteHasUpdatedAt = syncColumnExists(remoteColumnTypes, "updated_at")
				}
			} else {
				report.MissingRemoteTables = append(report.MissingRemoteTables, table)
			}
		}

		row.Status, row.Note = syncCoverageStatus(row)
		report.Tables = append(report.Tables, row)
	}

	for _, table := range localTables {
		if syncTableSet[table] || isKnownLocalOnlyTable(table) {
			continue
		}
		report.UntrackedLocalTables = append(report.UntrackedLocalTables, table)
	}
	sort.Strings(report.MissingLocalTables)
	sort.Strings(report.MissingRemoteTables)
	sort.Strings(report.UntrackedLocalTables)

	return report, nil
}

func sqliteTableNames(db *gorm.DB) ([]string, error) {
	rows, err := db.Raw(`
		SELECT name
		FROM sqlite_master
		WHERE type = 'table'
		  AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

func syncCoverageStatus(row SyncTableCoverage) (string, string) {
	if !row.LocalExists {
		return "missing_local_table", "Local SQLite table is missing."
	}
	if !row.LocalHasUpdatedAt {
		return "missing_local_updated_at", "Table cannot be incrementally synced until it has updated_at."
	}
	if row.RemoteChecked {
		if !row.RemoteExists {
			return "missing_remote_table", "Remote Supabase table is missing; run remote schema repair before syncing."
		}
		if !row.RemoteHasUpdatedAt {
			return "missing_remote_updated_at", "Remote Supabase table exists but cannot be incrementally synced until it has updated_at."
		}
		return "ready", "Local and remote schemas are present for incremental sync."
	}
	return "local_ready", "Local schema is ready; remote schema was not checked because Supabase is offline."
}

func isKnownLocalOnlyTable(table string) bool {
	switch strings.ToLower(strings.TrimSpace(table)) {
	case "",
		"settings",
		"sync_records",
		"sync_status",
		"user_sessions",
		"jobs",
		"file_watch_events",
		"collaborative_pending_operations",
		"quick_captures",
		"ocr_documents",
		"inbox_documents",
		"graph_nodes",
		"graph_edges",
		"extracted_documents",
		"customer_contacts_backup",
		"customers_backup",
		"costing_sheets",
		"costing_items",
		"costing_additional_costs",
		"ar_aging":
		return true
	default:
		return false
	}
}
