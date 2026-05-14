package main

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShouldRefreshRuntimeDatabaseAllowsNewerPackagedSeedWhenRuntimeHasNoNewerCollaborativeState(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "runtime.db")
	packagedPath := filepath.Join(dir, "packaged.db")

	writeReseedGuardTestDB(t, runtimePath, "2026_05_05_120000", time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC), false)
	writeReseedGuardTestDB(t, packagedPath, "2026_05_05_130000", time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC), false)

	require.True(t, shouldRefreshRuntimeDatabase(runtimePath, packagedPath))
}

func TestShouldRefreshRuntimeDatabaseBlocksPackagedSeedWhenRuntimeCollaborativeStateIsNewer(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "runtime.db")
	packagedPath := filepath.Join(dir, "packaged.db")

	writeReseedGuardTestDB(t, runtimePath, "2026_05_05_120000", time.Date(2026, 4, 18, 13, 32, 31, 0, time.UTC), false)
	writeReseedGuardTestDB(t, packagedPath, "2026_05_05_130000", time.Date(2026, 4, 10, 15, 42, 10, 0, time.UTC), false)

	require.False(t, shouldRefreshRuntimeDatabase(runtimePath, packagedPath))
}

func TestShouldRefreshRuntimeDatabaseBlocksPackagedSeedWithPendingCollaborativeOperations(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, "runtime.db")
	packagedPath := filepath.Join(dir, "packaged.db")

	writeReseedGuardTestDB(t, runtimePath, "2026_05_05_120000", time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC), true)
	writeReseedGuardTestDB(t, packagedPath, "2026_05_05_130000", time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC), false)

	require.False(t, shouldRefreshRuntimeDatabase(runtimePath, packagedPath))
}

func writeReseedGuardTestDB(t *testing.T, path, stamp string, taskActivityAt time.Time, pendingOperation bool) {
	t.Helper()

	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.Ping())
	_, err = db.Exec(`
		CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT, deleted_at DATETIME);
		CREATE TABLE task_items (id TEXT PRIMARY KEY, updated_at DATETIME, deleted_at DATETIME);
		CREATE TABLE task_activity (id TEXT PRIMARY KEY, task_id TEXT, updated_at DATETIME, deleted_at DATETIME);
		CREATE TABLE collaborative_pending_operations (id TEXT PRIMARY KEY, status TEXT, updated_at DATETIME, deleted_at DATETIME);
	`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO settings (key, value, deleted_at) VALUES (?, ?, NULL)`, deploymentDatabaseReseedSettingKey, stamp)
	require.NoError(t, err)

	if !taskActivityAt.IsZero() {
		formatted := taskActivityAt.Format(time.RFC3339Nano)
		_, err = db.Exec(`INSERT INTO task_activity (id, task_id, updated_at, deleted_at) VALUES (?, ?, ?, NULL)`, "activity-1", "task-1", formatted)
		require.NoError(t, err)
	}

	if pendingOperation {
		_, err = db.Exec(`INSERT INTO collaborative_pending_operations (id, status, updated_at, deleted_at) VALUES (?, ?, ?, NULL)`, "op-1", "pending", time.Now().Format(time.RFC3339Nano))
		require.NoError(t, err)
	}
}
