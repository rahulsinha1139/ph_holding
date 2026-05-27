package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestManualEnsureAdminDataUpdateSchema(t *testing.T) {
	if os.Getenv("ADMIN_DATA_UPDATE_SCHEMA_COMMIT") != "1" {
		t.Skip("set ADMIN_DATA_UPDATE_SCHEMA_COMMIT=1 to widen the live SQLite DB for admin-published updates")
	}
	dbPath := os.Getenv("ADMIN_DATA_UPDATE_DB")
	if dbPath == "" {
		dbPath = filepath.Join(".", "ph_holdings.db")
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	app := &App{
		db:            db,
		cache:         NewCache(),
		appStartTime:  time.Now(),
		currentUserID: "manual-schema-admin",
		currentUser: &User{
			Base:     Base{ID: "manual-schema-admin"},
			Username: "manual-schema-admin",
			RoleName: "admin",
			Role: Role{
				Name:        "admin",
				DisplayName: "Administrator",
				Permissions: `["*"]`,
			},
		},
	}
	require.NoError(t, app.ensureCriticalDeploymentFoundations())
	require.Equal(t, "ok", app.runIntegrityCheck())
}
