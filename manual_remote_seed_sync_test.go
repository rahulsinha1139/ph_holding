package main

import (
	"os"
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
