package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestCommercialDeploymentCleanupSoftDeletesLocalAndRemote(t *testing.T) {
	app := setupTestApp(t)
	migrateCommercialCleanupTables(t, app.db)

	remoteDB := openCommercialCleanupTestDB(t, "remote")
	migrateCommercialCleanupTables(t, remoteDB)

	seedCommercialCleanupFixture(t, app.db)
	seedCommercialCleanupFixture(t, remoteDB)

	app.dbManager = &DBManager{
		local:    app.db,
		remote:   remoteDB,
		stopChan: make(chan struct{}),
	}

	preview, err := app.PreviewCommercialDeploymentCleanup()
	require.NoError(t, err)
	require.True(t, preview.DryRun)
	require.True(t, preview.RemoteConnected)
	require.Equal(t, int64(len(commercialDeploymentCleanupTables)+1), preview.LocalTotal)
	require.Equal(t, int64(len(commercialDeploymentCleanupTables)+1), preview.RemoteTotal)
	require.Equal(t, commercialDeploymentCleanupConfirmation, preview.ConfirmationRequired)

	_, err = app.ApplyCommercialDeploymentCleanup("YES")
	require.Error(t, err)
	require.Contains(t, err.Error(), "confirmation")

	applied, err := app.ApplyCommercialDeploymentCleanup(commercialDeploymentCleanupConfirmation)
	require.NoError(t, err)
	require.False(t, applied.DryRun)
	require.True(t, applied.RemoteConnected)
	require.Equal(t, int64(len(commercialDeploymentCleanupTables)+1), applied.LocalTotal)
	require.Equal(t, int64(len(commercialDeploymentCleanupTables)+1), applied.RemoteTotal)

	for _, db := range []*gorm.DB{app.db, remoteDB} {
		for _, table := range commercialDeploymentCleanupTables {
			require.Equal(t, int64(0), activeCleanupRowCount(t, db, table), "table %s should be hidden", table)
		}

		var serial map[string]interface{}
		require.NoError(t, db.Table("serial_numbers").Where("id = ?", "serial-cleanup-1").Take(&serial).Error)
		require.Equal(t, "Available", fmt.Sprint(serial["status"]))
		require.Empty(t, fmt.Sprint(serial["dn_item_id"]))
		require.Empty(t, fmt.Sprint(serial["dn_number"]))
	}
}

func TestCommercialDeploymentCleanupIsAdminOnly(t *testing.T) {
	app := setupTestApp(t)
	migrateCommercialCleanupTables(t, app.db)
	app.currentUser = &User{
		Base:     Base{ID: "manager-user"},
		Username: "manager-user",
		RoleName: "manager",
		Role: Role{
			Name:        "manager",
			DisplayName: "Manager",
			Permissions: `["dashboard:view","orders:view","reports:view"]`,
		},
	}

	_, err := app.PreviewCommercialDeploymentCleanup()
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "permission")
}

func openCommercialCleanupTestDB(t *testing.T, name string) *gorm.DB {
	t.Helper()
	dbName := strings.ReplaceAll(t.Name()+"_"+name, "/", "_")
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", dbName)), &gorm.Config{
		Logger:                                   logger.Default.LogMode(logger.Silent),
		DisableForeignKeyConstraintWhenMigrating: true,
		SkipDefaultTransaction:                   true,
	})
	require.NoError(t, err)
	return db
}

func migrateCommercialCleanupTables(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(
		&SupplierPayment{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&DeliveryNote{},
		&DeliveryNoteItem{},
		&ExpenseEntry{},
		&ExpenseAllocation{},
		&ExpenseAttachment{},
		&ExpenseApproval{},
		&RecurringExpense{},
		&BankExpenseEntry{},
		&SerialNumber{},
	))
}

func seedCommercialCleanupFixture(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Now().UTC().Add(-time.Hour)
	for _, table := range commercialDeploymentCleanupTables {
		require.NoError(t, db.Table(table).Create(map[string]interface{}{
			"id":         table + "-cleanup-1",
			"created_at": now,
			"updated_at": now,
		}).Error, "seed %s", table)
	}
	require.NoError(t, db.Table("serial_numbers").Create(map[string]interface{}{
		"id":         "serial-cleanup-1",
		"created_at": now,
		"updated_at": now,
		"serial_no":  "SN-CLEANUP-1",
		"status":     "Delivered",
		"dn_item_id": "dn-item-cleanup-1",
		"dn_number":  "DN-CLEANUP-1",
	}).Error)
}

func activeCleanupRowCount(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Table(table).Where("deleted_at IS NULL").Count(&count).Error)
	return count
}
