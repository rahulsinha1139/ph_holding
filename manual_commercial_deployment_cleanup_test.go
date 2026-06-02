package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestManualCommercialDeploymentCleanup is an explicitly gated live-operation
// runner for the PH Trading commercial handoff cleanup. It is skipped during
// normal test runs. Set RUN_COMMERCIAL_CLEANUP=1 to preview, and add
// COMMERCIAL_CLEANUP_APPLY=1 to apply the soft-delete tombstones locally and
// remotely. Set COMMERCIAL_CLEANUP_MIGRATE=1 when Supabase needs the latest
// schema before preview/apply.
func TestManualCommercialDeploymentCleanup(t *testing.T) {
	if os.Getenv("RUN_COMMERCIAL_CLEANUP") != "1" {
		t.Skip("set RUN_COMMERCIAL_CLEANUP=1 to preview the commercial deployment cleanup")
	}

	_ = godotenv.Overload("deploy_package/.env")
	cfg, err := LoadConfig()
	require.NoError(t, err)

	dbPath := strings.TrimSpace(os.Getenv("COMMERCIAL_CLEANUP_DB_PATH"))
	if dbPath == "" {
		dbPath = cfg.Database.Path
	}
	require.NotEmpty(t, dbPath, "commercial cleanup requires a local SQLite database path")

	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_foreign_keys=ON"), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Warn),
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	app := &App{
		db: db,
		currentUser: &User{
			Base:     Base{ID: "manual-commercial-cleanup-admin"},
			Username: "manual-commercial-cleanup-admin",
			RoleName: "admin",
			Role: Role{
				Name:        "admin",
				DisplayName: "Administrator",
				Permissions: `["*"]`,
			},
			IsActive: true,
		},
	}

	manager := NewDBManager(db, cfg.Supabase)
	require.NoError(t, manager.ConnectRemote())
	t.Cleanup(manager.Disconnect)
	if os.Getenv("COMMERCIAL_CLEANUP_MIGRATE") == "1" {
		require.NoError(t, manager.MigrateRemote())
	}
	app.dbManager = manager

	preview, err := app.PreviewCommercialDeploymentCleanup()
	require.NoError(t, err)
	logCommercialCleanupResult(t, "preview", preview)

	if os.Getenv("COMMERCIAL_CLEANUP_APPLY") != "1" {
		t.Log("preview only; set COMMERCIAL_CLEANUP_APPLY=1 to apply")
		return
	}

	applied, err := app.ApplyCommercialDeploymentCleanup(commercialDeploymentCleanupConfirmation)
	require.NoError(t, err)
	logCommercialCleanupResult(t, "applied", applied)
}

func logCommercialCleanupResult(t *testing.T, label string, result CommercialDeploymentCleanupResult) {
	t.Helper()
	payload, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)
	fmt.Fprintf(os.Stdout, "\nCOMMERCIAL_CLEANUP_%s=%s\n", strings.ToUpper(label), payload)
}
