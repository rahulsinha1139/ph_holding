package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupDataUpdateTestApp(t *testing.T) *App {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "ph_holdings.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Setting{}, &CustomerMaster{}))
	app := &App{
		db:            db,
		cache:         NewCache(),
		appStartTime:  time.Now(),
		currentUserID: "admin-user",
		currentUser: &User{
			Base:     Base{ID: "admin-user"},
			Username: "admin",
			RoleName: "admin",
			Role: Role{
				Name:        "admin",
				DisplayName: "Administrator",
				Permissions: `["*"]`,
			},
		},
	}
	require.NoError(t, app.EnsureAdminDataUpdateFoundation())
	return app
}

func seedDataUpdateManifest(t *testing.T, app *App, version string, ops ...DataUpdateOperation) DataUpdateManifest {
	t.Helper()
	publishedAt := time.Now().Add(-time.Minute)
	manifest := DataUpdateManifest{
		Base:         Base{ID: "manifest-" + version, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		VersionLabel: version,
		Title:        "Seed " + version,
		Status:       "published",
		PublishedAt:  &publishedAt,
		ApplyMode:    "upsert_only",
		Source:       "test",
	}
	require.NoError(t, app.db.Create(&manifest).Error)
	for idx := range ops {
		ops[idx].ManifestID = manifest.ID
		if ops[idx].Sequence == 0 {
			ops[idx].Sequence = idx + 1
		}
		require.NoError(t, app.db.Create(&ops[idx]).Error)
	}
	return manifest
}

func dataUpdateJSON(t *testing.T, row map[string]interface{}) string {
	t.Helper()
	encoded, err := json.Marshal(row)
	require.NoError(t, err)
	return string(encoded)
}

func TestApplyDataUpdateManifestCreatesBackupAndIsIdempotent(t *testing.T) {
	app := setupDataUpdateTestApp(t)
	customer := CustomerMaster{
		Base:         Base{ID: "customer-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:   "CUST-1",
		BusinessName: "Old Name WLL",
		Status:       "Active",
	}
	require.NoError(t, app.db.Create(&customer).Error)

	row := map[string]interface{}{
		"id":            customer.ID,
		"customer_id":   "CUST-1",
		"business_name": "Updated Customer WLL",
		"status":        "Active",
	}
	manifest := seedDataUpdateManifest(t, app, "2025-seed-v1", DataUpdateOperation{
		Base:        Base{ID: "op-1", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		TargetTable: "customers",
		Operation:   "upsert",
		PrimaryKey:  customer.ID,
		RowJSON:     dataUpdateJSON(t, row),
		RowHash:     dataUpdateHashRecord(row),
	})

	result, err := app.ApplyDataUpdateManifest(manifest.ID, false)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.FileExists(t, result.BackupPath)

	var updated CustomerMaster
	require.NoError(t, app.db.First(&updated, "id = ?", customer.ID).Error)
	require.Equal(t, "Updated Customer WLL", updated.BusinessName)

	pending, err := app.ListPendingDataUpdateManifests()
	require.NoError(t, err)
	require.Empty(t, pending)

	second, err := app.ApplyDataUpdateManifest(manifest.ID, false)
	require.NoError(t, err)
	require.True(t, second.AlreadyApplied)

	var count int64
	require.NoError(t, app.db.Model(&DataUpdateReceipt{}).Where("manifest_id = ? AND status = ?", manifest.ID, "applied").Count(&count).Error)
	require.Equal(t, int64(1), count)
}

func TestPendingDataUpdatesAutoApplyUsesPublishedManifests(t *testing.T) {
	app := setupDataUpdateTestApp(t)
	customer := CustomerMaster{
		Base:         Base{ID: "customer-auto", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:   "CUST-AUTO",
		BusinessName: "Auto Before WLL",
		Status:       "Active",
	}
	require.NoError(t, app.db.Create(&customer).Error)

	row := map[string]interface{}{
		"id":            customer.ID,
		"customer_id":   customer.CustomerID,
		"business_name": "Auto Applied WLL",
		"status":        "Active",
	}
	seedDataUpdateManifest(t, app, "auto-v1", DataUpdateOperation{
		Base:        Base{ID: "op-auto", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		TargetTable: "customers",
		Operation:   "upsert",
		PrimaryKey:  customer.ID,
		RowJSON:     dataUpdateJSON(t, row),
		RowHash:     dataUpdateHashRecord(row),
	})

	summary, err := app.applyPendingDataUpdatesInternal(false, "test_sync")
	require.NoError(t, err)
	require.Equal(t, 1, summary.PendingCount)
	require.Equal(t, 1, summary.AppliedCount)
	require.Equal(t, 0, summary.FailedCount)

	var updated CustomerMaster
	require.NoError(t, app.db.First(&updated, "id = ?", customer.ID).Error)
	require.Equal(t, "Auto Applied WLL", updated.BusinessName)

	second, err := app.applyPendingDataUpdatesInternal(false, "test_sync")
	require.NoError(t, err)
	require.Zero(t, second.PendingCount)
	require.Zero(t, second.AppliedCount)
}

func TestDataUpdateDryRunDoesNotMutateOrCreateBackup(t *testing.T) {
	app := setupDataUpdateTestApp(t)
	customer := CustomerMaster{
		Base:         Base{ID: "customer-2", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:   "CUST-2",
		BusinessName: "Dry Run Customer",
		Status:       "Active",
	}
	require.NoError(t, app.db.Create(&customer).Error)

	manifest := seedDataUpdateManifest(t, app, "dry-run-v1", DataUpdateOperation{
		Base:        Base{ID: "op-dry", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		TargetTable: "customers",
		Operation:   "upsert",
		PrimaryKey:  customer.ID,
		RowJSON: dataUpdateJSON(t, map[string]interface{}{
			"id":            customer.ID,
			"customer_id":   "CUST-2",
			"business_name": "Should Not Apply",
			"status":        "Active",
		}),
	})

	result, err := app.ApplyDataUpdateManifest(manifest.ID, true)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Empty(t, result.BackupPath)

	var unchanged CustomerMaster
	require.NoError(t, app.db.First(&unchanged, "id = ?", customer.ID).Error)
	require.Equal(t, "Dry Run Customer", unchanged.BusinessName)

	var receipts int64
	require.NoError(t, app.db.Model(&DataUpdateReceipt{}).Count(&receipts).Error)
	require.Zero(t, receipts)
}

func TestDataUpdateRejectsUnknownTablesAndDeletes(t *testing.T) {
	app := setupDataUpdateTestApp(t)
	manifest := seedDataUpdateManifest(t, app, "reject-v1",
		DataUpdateOperation{
			Base:        Base{ID: "op-delete", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			TargetTable: "customers",
			Operation:   "delete",
			PrimaryKey:  "customer-x",
			RowJSON:     dataUpdateJSON(t, map[string]interface{}{"id": "customer-x"}),
		},
		DataUpdateOperation{
			Base:        Base{ID: "op-unknown", CreatedAt: time.Now(), UpdatedAt: time.Now()},
			TargetTable: "supplier_invoices",
			Operation:   "upsert",
			PrimaryKey:  "supplier-invoice-x",
			RowJSON:     dataUpdateJSON(t, map[string]interface{}{"id": "supplier-invoice-x"}),
		},
	)

	validation, err := app.ValidateDataUpdateManifest(manifest.ID)
	require.NoError(t, err)
	require.False(t, validation.Valid)
	require.Contains(t, strings.Join(validation.Issues, " | "), "unsupported operation")
	require.Contains(t, strings.Join(validation.Issues, " | "), "non-allowlisted table")

	result, err := app.ApplyDataUpdateManifest(manifest.ID, false)
	require.Error(t, err)
	require.False(t, result.Applied)

	backups, globErr := filepath.Glob(filepath.Join(filepath.Dir(app.getTestDBPath(t)), "backups", "*.db"))
	if globErr == nil {
		require.Empty(t, backups)
	}
}

func (a *App) getTestDBPath(t *testing.T) string {
	t.Helper()
	sqlDB, err := a.db.DB()
	require.NoError(t, err)
	row := sqlDB.QueryRow("PRAGMA database_list")
	var seq int
	var name, file string
	require.NoError(t, row.Scan(&seq, &name, &file))
	if _, err := os.Stat(file); err == nil {
		return file
	}
	return ""
}
