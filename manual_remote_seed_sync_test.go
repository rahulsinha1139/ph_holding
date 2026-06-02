package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openManualSeedSQLite(t *testing.T) *gorm.DB {
	t.Helper()
	dbPath := strings.TrimSpace(os.Getenv("SEEDED_DB_PATH"))
	if dbPath == "" {
		dbPath = "ph_holdings.db"
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	require.NoError(t, err)
	return db
}

func openManualSeedRemoteManager(t *testing.T, local *gorm.DB) *DBManager {
	t.Helper()
	require.NoError(t, godotenv.Overload("deploy_package/.env"))
	cfg := LoadSupabaseConfig()
	require.True(t, cfg.Enabled, "Supabase config must be enabled and complete")
	manager := NewDBManager(local, cfg)
	require.NoError(t, manager.ConnectRemote())
	t.Cleanup(manager.Disconnect)
	return manager
}

func TestManualPushSeededSQLiteToSupabase(t *testing.T) {
	if os.Getenv("PUSH_SEEDED_SQLITE_TO_SUPABASE") != "1" {
		t.Skip("set PUSH_SEEDED_SQLITE_TO_SUPABASE=1 to push the current seeded SQLite DB to Supabase")
	}
	local := openManualSeedSQLite(t)
	manager := openManualSeedRemoteManager(t, local)
	require.NoError(t, manager.MigrateRemote())

	tableFilter := map[string]bool{}
	if raw := strings.TrimSpace(os.Getenv("SEED_SYNC_TABLES")); raw != "" {
		for _, table := range strings.Split(raw, ",") {
			table = strings.TrimSpace(table)
			if table != "" {
				tableFilter[table] = true
			}
		}
	}
	errors := []string{}
	total := 0
	for _, table := range activeDBSyncTables() {
		if len(tableFilter) > 0 && !tableFilter[table] {
			continue
		}
		count, err := manager.syncTable(table, time.Time{}, "push")
		if err != nil {
			errors = append(errors, table+": "+err.Error())
			continue
		}
		if count > 0 {
			t.Logf("pushed %s: %d", table, count)
		}
		total += count
		var localCount int64
		var remoteCount int64
		if err := local.Table(table).Count(&localCount).Error; err == nil && localCount > 0 {
			if remote := manager.Remote(); remote != nil {
				if err := remote.Table(table).Count(&remoteCount).Error; err == nil && remoteCount == 0 {
					errors = append(errors, table+": remote count remained zero after push")
				}
			}
		}
	}
	require.Empty(t, errors, strings.Join(errors, "\n"))
	require.Greater(t, total, 0, "seed push should send at least one record")
}

func TestManualCompareLocalRemoteSeedCounts(t *testing.T) {
	if os.Getenv("CHECK_REMOTE_SEED_COUNTS") != "1" {
		t.Skip("set CHECK_REMOTE_SEED_COUNTS=1 to compare key local and Supabase table counts")
	}
	local := openManualSeedSQLite(t)
	manager := openManualSeedRemoteManager(t, local)
	require.NoError(t, manager.MigrateRemote())
	remote := manager.Remote()
	require.NotNil(t, remote)

	keyTables := []string{
		"customers",
		"suppliers",
		"opportunities",
		"offers",
		"orders",
		"invoices",
		"purchase_orders",
		"supplier_invoices",
		"intelligence_opportunity_enrichment",
		"intelligence_line_item_enrichment",
		"intelligence_order_enrichment",
	}
	for _, table := range keyTables {
		var localCount int64
		var remoteCount int64
		require.NoError(t, local.Table(table).Count(&localCount).Error, table)
		require.NoError(t, remote.Table(table).Count(&remoteCount).Error, table)
		t.Logf("%s local=%d remote=%d", table, localCount, remoteCount)
		require.GreaterOrEqual(t, remoteCount, localCount, "remote %s should contain at least the local seeded rows", table)
	}
}

func TestManualCompareLocalRemoteIntakeCounts(t *testing.T) {
	if os.Getenv("CHECK_REMOTE_INTAKE_COUNTS") != "1" {
		t.Skip("set CHECK_REMOTE_INTAKE_COUNTS=1 to compare workflow intake tables against Supabase")
	}
	local := openManualSeedSQLite(t)
	manager := openManualSeedRemoteManager(t, local)
	require.NoError(t, manager.MigrateRemote())
	remote := manager.Remote()
	require.NotNil(t, remote)

	intakeTables := []string{
		"customers",
		"suppliers",
		"products",
		"rfq_data",
		"costing_sheet_data",
		"costing_line_items",
		"opportunities",
		"offers",
		"offer_items",
		"offer_follow_ups",
		"offer_notes",
		"orders",
		"order_items",
		"purchase_orders",
		"purchase_order_items",
		"goods_received_notes",
		"grn_items",
		"delivery_notes",
		"delivery_note_items",
		"invoices",
		"invoice_items",
		"payments",
		"customer_receipts",
		"supplier_invoices",
		"supplier_invoice_items",
		"supplier_payments",
		"expense_categories",
		"expense_vendors",
		"expense_entries",
		"bank_statements",
		"bank_statement_lines",
		"employees",
		"projects",
		"task_items",
		"notifications",
		"data_update_manifests",
		"data_update_operations",
		"data_update_receipts",
	}

	for _, table := range intakeTables {
		var localCount int64
		var remoteCount int64
		require.NoError(t, local.Table(table).Count(&localCount).Error, table)
		require.NoError(t, remote.Table(table).Count(&remoteCount).Error, table)
		t.Logf("%s local=%d remote=%d", table, localCount, remoteCount)
		require.GreaterOrEqual(t, remoteCount, localCount, "remote %s should contain at least the local intake rows", table)
	}
}

func TestManualDownloadRemoteSnapshotToStagingDB(t *testing.T) {
	if os.Getenv("DOWNLOAD_REMOTE_SNAPSHOT") != "1" {
		t.Skip("set DOWNLOAD_REMOTE_SNAPSHOT=1 and SEEDED_DB_PATH to a staging/snapshot SQLite file to download Supabase")
	}
	dbPath := strings.TrimSpace(os.Getenv("SEEDED_DB_PATH"))
	require.NotEmpty(t, dbPath, "SEEDED_DB_PATH must point to a staging/snapshot DB")
	base := strings.ToLower(filepath.Base(dbPath))
	require.True(t,
		strings.Contains(base, "snapshot") || strings.Contains(base, "staging"),
		"refusing destructive full download into non-staging DB path: %s", dbPath,
	)

	local := openManualSeedSQLite(t)
	manager := openManualSeedRemoteManager(t, local)
	downloaded, err := manager.DownloadFullDatabase()
	require.NoError(t, err)
	require.Greater(t, downloaded, 0, "remote snapshot download should copy at least one row")
	t.Logf("downloaded %d Supabase rows into %s", downloaded, dbPath)
}

func TestManualPullRemoteTablesToStagingDB(t *testing.T) {
	if os.Getenv("PULL_REMOTE_TABLES_TO_STAGING") != "1" {
		t.Skip("set PULL_REMOTE_TABLES_TO_STAGING=1, SEEDED_DB_PATH to staging DB, and PULL_REMOTE_TABLES to a comma list")
	}
	dbPath := strings.TrimSpace(os.Getenv("SEEDED_DB_PATH"))
	require.NotEmpty(t, dbPath, "SEEDED_DB_PATH must point to a staging/snapshot DB")
	base := strings.ToLower(filepath.Base(dbPath))
	require.True(t,
		strings.Contains(base, "snapshot") || strings.Contains(base, "staging"),
		"refusing remote pull into non-staging DB path: %s", dbPath,
	)

	rawTables := strings.TrimSpace(os.Getenv("PULL_REMOTE_TABLES"))
	require.NotEmpty(t, rawTables, "PULL_REMOTE_TABLES must name at least one table")
	local := openManualSeedSQLite(t)
	manager := openManualSeedRemoteManager(t, local)

	total := 0
	for _, rawTable := range strings.Split(rawTables, ",") {
		table := strings.TrimSpace(rawTable)
		if table == "" {
			continue
		}
		require.True(t, isValidSQLIdentifier(table), "invalid table name: %s", table)
		count, err := manager.syncTable(table, time.Time{}, "pull")
		require.NoError(t, err, table)
		t.Logf("pulled %s: %d", table, count)
		total += count
	}
	require.Greater(t, total, 0, "explicit staging pull should copy at least one row")
}

func TestManualMergeRemoteOnlyRowsIntoStagingDB(t *testing.T) {
	if os.Getenv("MERGE_REMOTE_ONLY_TO_STAGING") != "1" {
		t.Skip("set MERGE_REMOTE_ONLY_TO_STAGING=1, SEEDED_DB_PATH to a staging/candidate DB, and REMOTE_SNAPSHOT_DB_PATH")
	}
	dbPath := strings.TrimSpace(os.Getenv("SEEDED_DB_PATH"))
	require.NotEmpty(t, dbPath, "SEEDED_DB_PATH must point to a staging/candidate DB")
	base := strings.ToLower(filepath.Base(dbPath))
	require.True(t,
		strings.Contains(base, "staging") || strings.Contains(base, "candidate"),
		"refusing merge into non-staging/candidate DB path: %s", dbPath,
	)

	snapshotPath := strings.TrimSpace(os.Getenv("REMOTE_SNAPSHOT_DB_PATH"))
	require.NotEmpty(t, snapshotPath, "REMOTE_SNAPSHOT_DB_PATH is required")

	local := openManualSeedSQLite(t)
	sqlDB, err := local.DB()
	require.NoError(t, err)
	require.NoError(t, local.Exec("ATTACH DATABASE ? AS remote_snapshot", snapshotPath).Error)

	tables := activeDBSyncTables()
	if raw := strings.TrimSpace(os.Getenv("MERGE_REMOTE_TABLES")); raw != "" {
		tables = nil
		for _, table := range strings.Split(raw, ",") {
			table = strings.TrimSpace(table)
			if table != "" {
				tables = append(tables, table)
			}
		}
	}

	total := int64(0)
	for _, table := range tables {
		require.True(t, isValidSQLIdentifier(table), "invalid table name: %s", table)
		localColumns := manualSQLiteColumnSet(t, sqlDB, "main", table)
		remoteColumns := manualSQLiteColumnSet(t, sqlDB, "remote_snapshot", table)
		if len(localColumns) == 0 || len(remoteColumns) == 0 {
			continue
		}
		if _, ok := localColumns["id"]; !ok {
			continue
		}
		if _, ok := remoteColumns["id"]; !ok {
			continue
		}

		columns := make([]string, 0, len(localColumns))
		for column := range localColumns {
			if _, ok := remoteColumns[column]; ok {
				columns = append(columns, column)
			}
		}
		sort.Strings(columns)
		if len(columns) == 0 {
			continue
		}

		quotedColumns := make([]string, 0, len(columns))
		selectedColumns := make([]string, 0, len(columns))
		for _, column := range columns {
			quotedColumns = append(quotedColumns, quoteSQLiteIdentifier(column))
			selectedColumns = append(selectedColumns, "r."+quoteSQLiteIdentifier(column))
		}

		sql := fmt.Sprintf(
			`INSERT OR IGNORE INTO main.%s (%s)
			 SELECT %s
			 FROM remote_snapshot.%s r
			 WHERE NOT EXISTS (
			   SELECT 1 FROM main.%s l WHERE l.%s = r.%s
			 )`,
			quoteSQLiteIdentifier(table),
			strings.Join(quotedColumns, ", "),
			strings.Join(selectedColumns, ", "),
			quoteSQLiteIdentifier(table),
			quoteSQLiteIdentifier(table),
			quoteSQLiteIdentifier("id"),
			quoteSQLiteIdentifier("id"),
		)
		result := local.Exec(sql)
		require.NoError(t, result.Error, table)
		if result.RowsAffected > 0 {
			t.Logf("merged %s: %d remote-only rows", table, result.RowsAffected)
			total += result.RowsAffected
		}
	}
	require.Greater(t, total, int64(0), "remote-only merge should add at least one row")
	t.Logf("merged %d total remote-only rows into %s", total, dbPath)
}

func manualSQLiteColumnSet(t *testing.T, sqlDB interface {
	Query(query string, args ...interface{}) (*sql.Rows, error)
}, schema string, table string) map[string]bool {
	t.Helper()
	if !isValidSQLIdentifier(schema) || !isValidSQLIdentifier(table) {
		return nil
	}
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT 0", quoteSQLiteIdentifier(schema), quoteSQLiteIdentifier(table))
	rows, err := sqlDB.Query(query)
	if err != nil {
		return nil
	}
	defer rows.Close()
	columns, err := rows.Columns()
	require.NoError(t, err)
	result := make(map[string]bool, len(columns))
	for _, column := range columns {
		result[strings.ToLower(strings.TrimSpace(column))] = true
	}
	return result
}

func quoteSQLiteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}
