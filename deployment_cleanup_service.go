package main

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

const commercialDeploymentCleanupConfirmation = "CONFIRM_PH_TRADING_CLEAN_DEPLOYMENT"

var commercialDeploymentCleanupTables = []string{
	"supplier_payments",
	"supplier_invoice_items",
	"supplier_invoices",
	"grn_items",
	"goods_received_notes",
	"purchase_order_items",
	"purchase_orders",
	"delivery_note_items",
	"delivery_notes",
	"expense_approvals",
	"expense_attachments",
	"expense_allocations",
	"expense_entries",
	"recurring_expenses",
	"bank_expense_entries",
}

// CommercialDeploymentCleanupResult reports the local and cloud impact of the
// PH Trading clean-deployment scrub. The scrub preserves master data and
// customer-facing commercial records while hiding supplier/expense actuals.
type CommercialDeploymentCleanupResult struct {
	DryRun               bool                                     `json:"dry_run"`
	ConfirmationRequired string                                   `json:"confirmation_required,omitempty"`
	RemoteRequired       bool                                     `json:"remote_required"`
	RemoteConnected      bool                                     `json:"remote_connected"`
	LocalTotal           int64                                    `json:"local_total"`
	RemoteTotal          int64                                    `json:"remote_total"`
	Tables               []CommercialDeploymentCleanupTableResult `json:"tables"`
	Messages             []string                                 `json:"messages,omitempty"`
}

type CommercialDeploymentCleanupTableResult struct {
	Table      string `json:"table"`
	LocalRows  int64  `json:"local_rows"`
	RemoteRows int64  `json:"remote_rows"`
	Error      string `json:"error,omitempty"`
}

// PreviewCommercialDeploymentCleanup counts the records that would be hidden
// from the production app. It does not mutate local SQLite or Supabase.
func (a *App) PreviewCommercialDeploymentCleanup() (CommercialDeploymentCleanupResult, error) {
	if err := a.requirePermission("*"); err != nil {
		return CommercialDeploymentCleanupResult{}, err
	}
	return a.runCommercialDeploymentCleanup(true, nil)
}

// ApplyCommercialDeploymentCleanup hides supplier invoices, supplier POs, GRNs,
// delivery notes, and finance expense actuals locally and in Supabase. The
// updated_at tombstones make the cleanup win during subsequent bidirectional sync.
func (a *App) ApplyCommercialDeploymentCleanup(confirm string) (CommercialDeploymentCleanupResult, error) {
	if err := a.requirePermission("*"); err != nil {
		return CommercialDeploymentCleanupResult{}, err
	}
	if strings.TrimSpace(confirm) != commercialDeploymentCleanupConfirmation {
		return CommercialDeploymentCleanupResult{}, fmt.Errorf("confirmation must equal %s", commercialDeploymentCleanupConfirmation)
	}
	if a.dbManager == nil {
		return CommercialDeploymentCleanupResult{}, fmt.Errorf("cloud sync manager is not initialized")
	}
	remoteDB, err := a.connectedRemoteDB()
	if err != nil {
		return CommercialDeploymentCleanupResult{}, err
	}
	return a.runCommercialDeploymentCleanup(false, remoteDB)
}

func (a *App) runCommercialDeploymentCleanup(dryRun bool, remoteDB *gorm.DB) (CommercialDeploymentCleanupResult, error) {
	if a.db == nil {
		return CommercialDeploymentCleanupResult{}, fmt.Errorf("database not initialized")
	}

	result := CommercialDeploymentCleanupResult{
		DryRun:               dryRun,
		ConfirmationRequired: commercialDeploymentCleanupConfirmation,
		RemoteRequired:       !dryRun,
		RemoteConnected:      remoteDB != nil,
		Messages: []string{
			"Preserves customers, suppliers, products, RFQs, costing sheets, offers, orders, and customer invoices.",
			"Hides supplier invoices, supplier POs, GRNs, delivery notes, supplier payments, and expense actuals.",
			"Expense categories and vendors remain available for the accounts team to use.",
		},
	}

	if dryRun && a.dbManager != nil {
		remoteDB = a.dbManager.Remote()
		result.RemoteConnected = remoteDB != nil
	}

	when := time.Now().UTC()
	for _, table := range commercialDeploymentCleanupTables {
		tableResult := CommercialDeploymentCleanupTableResult{Table: table}

		localRows, err := cleanupSoftDeleteTable(a.db, table, dryRun, when, false)
		if err != nil {
			tableResult.Error = fmt.Sprintf("local: %v", err)
		}
		tableResult.LocalRows = localRows
		result.LocalTotal += localRows

		if remoteDB != nil {
			remoteRows, remoteErr := cleanupSoftDeleteTable(remoteDB, table, dryRun, when, true)
			if remoteErr != nil {
				if tableResult.Error != "" {
					tableResult.Error += "; "
				}
				tableResult.Error += fmt.Sprintf("remote: %v", remoteErr)
			}
			tableResult.RemoteRows = remoteRows
			result.RemoteTotal += remoteRows
		}

		result.Tables = append(result.Tables, tableResult)
	}

	serials := CommercialDeploymentCleanupTableResult{Table: "serial_numbers.delivery_note_links"}
	localSerialRows, err := cleanupSerialDeliveryLinks(a.db, dryRun, when, false)
	if err != nil {
		serials.Error = fmt.Sprintf("local: %v", err)
	}
	serials.LocalRows = localSerialRows
	result.LocalTotal += localSerialRows
	if remoteDB != nil {
		remoteSerialRows, remoteErr := cleanupSerialDeliveryLinks(remoteDB, dryRun, when, true)
		if remoteErr != nil {
			if serials.Error != "" {
				serials.Error += "; "
			}
			serials.Error += fmt.Sprintf("remote: %v", remoteErr)
		}
		serials.RemoteRows = remoteSerialRows
		result.RemoteTotal += remoteSerialRows
	}
	result.Tables = append(result.Tables, serials)

	for _, table := range result.Tables {
		if table.Error != "" {
			return result, fmt.Errorf("commercial deployment cleanup has table errors")
		}
	}
	return result, nil
}

func (a *App) connectedRemoteDB() (*gorm.DB, error) {
	if a.dbManager == nil {
		return nil, fmt.Errorf("cloud sync manager is not initialized")
	}
	if remoteDB := a.dbManager.Remote(); remoteDB != nil {
		return remoteDB, nil
	}
	if err := a.dbManager.ConnectRemote(); err != nil {
		return nil, fmt.Errorf("connect to Supabase before cleanup: %w", err)
	}
	remoteDB := a.dbManager.Remote()
	if remoteDB == nil {
		return nil, fmt.Errorf("Supabase connection is not available")
	}
	return remoteDB, nil
}

func cleanupSoftDeleteTable(db *gorm.DB, table string, dryRun bool, when time.Time, requireTable bool) (int64, error) {
	if db == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	if !db.Migrator().HasTable(table) {
		if requireTable {
			return 0, fmt.Errorf("table %s is missing", table)
		}
		return 0, nil
	}
	columns, err := syncDBColumnTypes(db, table)
	if err != nil {
		return 0, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	if !syncColumnExists(columns, "deleted_at") || !syncColumnExists(columns, "updated_at") {
		return 0, fmt.Errorf("table %s does not support sync-safe soft delete", table)
	}

	query := db.Table(table).Where("deleted_at IS NULL")
	if dryRun {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return 0, fmt.Errorf("count %s rows: %w", table, err)
		}
		return count, nil
	}

	update := query.Updates(map[string]interface{}{
		"deleted_at": when,
		"updated_at": when,
	})
	if update.Error != nil {
		return 0, fmt.Errorf("soft delete %s rows: %w", table, update.Error)
	}
	return update.RowsAffected, nil
}

func cleanupSerialDeliveryLinks(db *gorm.DB, dryRun bool, when time.Time, requireTable bool) (int64, error) {
	const table = "serial_numbers"
	if db == nil {
		return 0, fmt.Errorf("database is not initialized")
	}
	if !db.Migrator().HasTable(table) {
		if requireTable {
			return 0, fmt.Errorf("table %s is missing", table)
		}
		return 0, nil
	}
	columns, err := syncDBColumnTypes(db, table)
	if err != nil {
		return 0, fmt.Errorf("inspect %s columns: %w", table, err)
	}
	required := []string{"updated_at", "dn_item_id", "dn_number", "status", "shipped_date", "warranty_start_date", "warranty_end_date"}
	for _, column := range required {
		if !syncColumnExists(columns, column) {
			return 0, fmt.Errorf("table %s is missing column %s", table, column)
		}
	}

	query := db.Table(table).Where("deleted_at IS NULL AND (COALESCE(dn_item_id, '') <> '' OR COALESCE(dn_number, '') <> '')")
	if dryRun {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return 0, fmt.Errorf("count serial delivery links: %w", err)
		}
		return count, nil
	}

	update := query.Updates(map[string]interface{}{
		"status":              "Available",
		"dn_item_id":          "",
		"dn_number":           "",
		"shipped_date":        nil,
		"warranty_start_date": nil,
		"warranty_end_date":   nil,
		"updated_at":          when,
	})
	if update.Error != nil {
		return 0, fmt.Errorf("reset serial delivery links: %w", update.Error)
	}
	return update.RowsAffected, nil
}
