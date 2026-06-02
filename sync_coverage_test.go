package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestDBSyncTablesCoverCriticalIngestionTables(t *testing.T) {
	required := []string{
		"users",
		"devices",
		"device_users",
		"data_update_manifests",
		"data_update_operations",
		"data_update_receipts",
		"customer_name_mappings",
		"supplier_issues",
		"offer_follow_ups",
		"shipments",
		"post_sale_notes",
		"chart_of_accounts",
		"account_mappings",
		"journal_entries",
		"journal_lines",
		"vat_returns",
		"currency_exchange_rates",
		"company_bank_accounts",
		"bank_cash_balances",
		"statement_hashes",
		"book_bank_reconciliations",
		"outstanding_cheques",
		"deposits_in_transit",
		"cheque_registers",
		"fx_rates",
		"fx_revaluations",
		"bank_statement_files",
		"bank_reconciliation_audit_logs",
		"warehouses",
		"inventory_items",
		"stock_movements",
		"stock_adjustments",
		"conversations",
		"chat_messages",
		"tally_invoice_imports",
		"tally_purchase_imports",
		"intelligence_opportunity_enrichment",
		"intelligence_line_item_enrichment",
		"intelligence_order_enrichment",
		"data_quality_reviews",
		"alerts",
		"followup_tasks",
		"assets",
		"contract_templates",
		"contract_clauses",
		"contracts",
	}

	for _, table := range required {
		require.Contains(t, dbSyncTables, table, "%s must sync to Supabase; it is an employee/business data ingestion point", table)
	}
}

func TestRemoteMigrationModelsCoverEverySyncTable(t *testing.T) {
	migrationTables := make(map[string]bool)
	for _, model := range syncRemoteMigrationModels() {
		migrationTables[tableNameForSyncModel(t, model)] = true
	}

	seenSyncTables := make(map[string]bool)
	for _, table := range dbSyncTables {
		require.False(t, seenSyncTables[table], "dbSyncTables contains duplicate table %s", table)
		seenSyncTables[table] = true
		require.True(t, migrationTables[table], "remote migration model list must create sync table %s", table)
	}
}

func TestNoOptionalRemoteSyncTablesRemain(t *testing.T) {
	for _, table := range dbSyncTables {
		require.False(t, isOptionalRemoteSyncTable(table), "%s must fail loudly when remote schema is missing", table)
	}
}

func TestEverySyncTableSupportsIncrementalUpdatedAt(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(syncRemoteMigrationModels()...))

	var missing []string
	for _, table := range dbSyncTables {
		columnTypes, err := syncDBColumnTypes(db, table)
		require.NoError(t, err, "must inspect sync table %s", table)
		if !syncColumnExists(columnTypes, "updated_at") {
			missing = append(missing, table)
		}
	}
	require.Empty(t, missing, "sync tables missing updated_at and would be skipped: %s", strings.Join(missing, ", "))
}

func tableNameForSyncModel(t *testing.T, model interface{}) string {
	t.Helper()
	if named, ok := model.(interface{ TableName() string }); ok {
		return named.TableName()
	}

	typ := reflect.TypeOf(model)
	require.NotNil(t, typ)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	require.Equal(t, reflect.Struct, typ.Kind())
	return schema.NamingStrategy{}.TableName(typ.Name())
}
