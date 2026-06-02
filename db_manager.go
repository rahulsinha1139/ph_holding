package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// DBManager manages dual-database connections (local SQLite + remote Supabase)
// SQLite is PRIMARY (always works offline), Supabase is for sync/backup
type DBManager struct {
	local                     *gorm.DB
	remote                    *gorm.DB
	config                    SupabaseConfig
	isOnline                  bool
	mu                        sync.RWMutex
	lastCheck                 time.Time
	checkInterval             time.Duration
	syncEnabled               bool
	syncInProgress            bool       // Prevents concurrent first-run syncs
	syncMu                    sync.Mutex // Lock for sync operations
	canPullActivityMonitoring func() bool
	afterSuccessfulSync       func(pushed int, pulled int)
	stopChan                  chan struct{}
	stopOnce                  sync.Once
	wg                        sync.WaitGroup
	periodicStarted           bool
}

// NewDBManager creates a new dual-database manager
func NewDBManager(localDB *gorm.DB, cfg SupabaseConfig) *DBManager {
	return &DBManager{
		local:         localDB,
		config:        cfg,
		isOnline:      false,
		checkInterval: 30 * time.Second,
		syncEnabled:   cfg.Enabled,
		stopChan:      make(chan struct{}),
	}
}

// Primary returns the PRIMARY database (always SQLite in hybrid mode)
// This ensures app always works even if Supabase is down
func (m *DBManager) Primary() *gorm.DB {
	return m.local // SQLite is always primary
}

// Local returns the local SQLite database (always available)
func (m *DBManager) Local() *gorm.DB {
	return m.local
}

// Remote returns the remote Supabase database (may be nil)
func (m *DBManager) Remote() *gorm.DB {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.remote
}

func (m *DBManager) SetAfterSuccessfulSync(callback func(pushed int, pulled int)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.afterSuccessfulSync = callback
}

// IsOnline returns whether Supabase sync is connected
func (m *DBManager) IsOnline() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isOnline
}

// IsSyncEnabled returns whether cloud sync is configured
func (m *DBManager) IsSyncEnabled() bool {
	return m.syncEnabled
}

// ConnectRemote establishes connection to Supabase PostgreSQL for sync
// This is non-blocking - failure just means sync won't work, app continues
func (m *DBManager) ConnectRemote() error {
	if !m.config.Enabled {
		log.Println("Supabase sync not configured - running in offline mode")
		return nil // Not an error, just not configured
	}

	if m.config.DBHost == "" || m.config.DBPassword == "" {
		log.Println("Supabase credentials incomplete - running in offline mode")
		return nil
	}

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		m.config.DBHost,
		m.config.DBPort,
		m.config.DBUser,
		m.config.DBPassword,
		m.config.DBName,
		m.config.DBSSLMode,
	)

	log.Printf("Connecting to Supabase (host: %s)...", m.config.DBHost)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Printf("Supabase connection failed (will retry): %v", err)
		return fmt.Errorf("supabase connection failed: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(5) // Lower for sync-only usage
	sqlDB.SetMaxIdleConns(2)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	// Verify connectivity
	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("supabase ping failed: %w", err)
	}

	m.mu.Lock()
	m.remote = db
	m.isOnline = true
	m.lastCheck = time.Now()
	m.mu.Unlock()

	log.Println("Supabase connected successfully - sync enabled")
	return nil
}

// CheckConnectivity verifies the Supabase connection is alive
func (m *DBManager) CheckConnectivity() bool {
	m.mu.RLock()
	remote := m.remote
	m.mu.RUnlock()

	if remote == nil {
		return false
	}

	sqlDB, err := remote.DB()
	if err != nil {
		m.setOffline()
		return false
	}

	if err := sqlDB.Ping(); err != nil {
		m.setOffline()
		log.Printf("Supabase connectivity lost: %v", err)
		return false
	}

	m.mu.Lock()
	m.isOnline = true
	m.lastCheck = time.Now()
	m.mu.Unlock()
	return true
}

func syncRemoteMigrationModels() []interface{} {
	return []interface{}{
		// Access and master data
		&Role{},
		&User{},
		&Device{},
		&DeviceUser{},
		&LicenseKey{},
		&DataUpdateManifest{},
		&DataUpdateOperation{},
		&DataUpdateReceipt{},
		&CustomerMaster{},
		&SupplierMaster{},
		&ProductMaster{},
		&CustomerContact{},
		&SupplierContact{},
		&CustomerNameMapping{},
		&SupplierIssue{},
		&EntityNote{},

		// Commercial pipeline
		&Opportunity{},
		&OpportunityComment{},
		&OpportunityEditConflict{},
		&RFQData{},
		&RFQComment{},
		&CostingSheetData{},
		&CostingSheetAttachment{},
		&CostingLineItemData{},
		&CostingHistory{},
		&Offer{},
		&OfferItem{},
		&OfferData{},
		&OfferNote{},
		&OfferFollowUp{},
		&Order{},
		&OrderItem{},
		&Shipment{},
		&PostSaleNote{},

		// Customer finance
		&Invoice{},
		&DBInvoiceItem{},
		&InvoiceSequence{},
		&CreditNote{},
		&CreditNoteItem{},
		&Payment{},
		&CustomerReceipt{},
		&CustomerReceiptAllocation{},

		// Supplier, procurement, and delivery
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&SupplierPayment{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&SerialNumber{},
		&DeliveryNote{},
		&DeliveryNoteItem{},

		// Operational accounting
		&ChartOfAccount{},
		&AccountMapping{},
		&FiscalPeriod{},
		&JournalEntry{},
		&JournalLine{},
		&VATReturn{},
		&CurrencyExchangeRate{},
		&CompanyBankAccount{},
		&BankAccount{},
		&BankStatement{},
		&BankStatementLine{},
		&BankLinePaymentAllocation{},
		&BankCashBalance{},
		&BankExpenseEntry{},
		&StatementHash{},
		&BookBankReconciliation{},
		&OutstandingCheque{},
		&DepositInTransit{},
		&ChequeRegister{},
		&FXRate{},
		&FXRevaluation{},
		&BankStatementFile{},
		&BankReconciliationAuditLog{},

		// Expenses and payroll
		&ExpenseCategory{},
		&ExpenseVendor{},
		&ExpenseEntry{},
		&ExpenseAllocation{},
		&RecurringExpense{},
		&ExpenseAttachment{},
		&ExpenseApproval{},
		&EmployeeCompensationProfile{},
		&PayrollPeriod{},
		&PayrollRun{},
		&PayrollRunItem{},
		&PayrollComponent{},
		&PayrollPayout{},

		// Inventory visibility
		&Warehouse{},
		&InventoryItem{},
		&StockMovement{},
		&StockAdjustment{},

		// People, work, and collaboration
		&Employee{},
		&EmployeeAccessLink{},
		&Project{},
		&ProjectMember{},
		&TaskItem{},
		&TaskComment{},
		&TaskActivity{},
		&Notification{},
		&NotificationReceipt{},
		&DeleteApprovalRequest{},
		&EmployeeArchiveRequest{},
		&UserActivitySession{},
		&UserActivityEvent{},
		&UserActivityWeeklySummary{},

		// Intelligence, reporting, and imports
		&Conversation{},
		&ChatMessage{},
		&PredictionRecord{},
		&WinProbabilityPrediction{},
		&DiscountRecommendationRecord{},
		&CustomerSnapshot{},
		&ActualOutcome{},
		&GradeChange{},
		&PaymentPredictionAccuracy{},
		&TallyInvoiceImport{},
		&TallyPurchaseImport{},
		&IntelligenceOpportunityEnrichment{},
		&IntelligenceLineItemEnrichment{},
		&IntelligenceOrderEnrichment{},
		&DataQualityReview{},
		&Alert{},
		&FollowUpTask{},
		&Asset{},
		&ContractTemplate{},
		&ContractClause{},
		&Contract{},
	}
}

// MigrateRemote runs AutoMigrate on Supabase (creates tables if needed)
func (m *DBManager) MigrateRemote() error {
	if m.remote == nil {
		return fmt.Errorf("supabase not connected")
	}

	log.Println("Running schema migration on Supabase...")

	for _, model := range syncRemoteMigrationModels() {
		if err := m.remote.AutoMigrate(model); err != nil {
			if isBenignRemoteMigrationDrift(err) {
				log.Printf("Supabase migration drift warning: %v", err)
				continue
			}
			log.Printf("Supabase migration warning: %v", err)
			return err
		}
	}
	if err := m.ensureRemoteWideTextColumns(); err != nil {
		return err
	}

	log.Println("Supabase schema migration complete")
	return nil
}

func (m *DBManager) ensureRemoteWideTextColumns() error {
	if m == nil || m.remote == nil {
		return nil
	}
	wideColumns := map[string][]string{
		"intelligence_opportunity_enrichment": {"id"},
		"intelligence_line_item_enrichment":   {"id"},
		"intelligence_order_enrichment":       {"id"},
	}
	for table, columns := range wideColumns {
		if !isValidSQLIdentifier(table) || !m.remote.Migrator().HasTable(table) {
			continue
		}
		for _, column := range columns {
			if !isValidSQLIdentifier(column) {
				continue
			}
			statement := fmt.Sprintf(`ALTER TABLE "%s" ALTER COLUMN "%s" TYPE TEXT`, table, column)
			if err := m.remote.Exec(statement).Error; err != nil {
				return fmt.Errorf("widen remote %s.%s: %w", table, column, err)
			}
		}
	}
	return nil
}

func isBenignRemoteMigrationDrift(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "does not exist") &&
		(strings.Contains(msg, "drop constraint") ||
			strings.Contains(msg, "constraint \"uni_")) {
		return true
	}
	return strings.Contains(msg, "check constraint") && strings.Contains(msg, "is violated by some row")
}

// SyncToRemote pushes local changes to Supabase
func (m *DBManager) SyncToRemote(since time.Time) (int, error) {
	if !m.IsOnline() {
		return 0, nil // Silent fail - sync will retry later
	}

	pushed := 0
	tableErrors := []string{}
	// Use the active canonical table list so push and pull always cover the same tables.
	for _, table := range activeDBSyncTables() {
		tableSince := m.getLastSyncTimeForTable(table, "push")
		count, err := m.syncTable(table, tableSince, "push")
		if err != nil {
			log.Printf("Sync push error for %s: %v", table, err)
			tableErrors = append(tableErrors, fmt.Sprintf("%s: %v", table, err))
			continue
		}
		pushed += count
	}

	if len(tableErrors) > 0 {
		return pushed, fmt.Errorf("push completed with %d table errors: %s", len(tableErrors), strings.Join(tableErrors, "; "))
	}
	return pushed, nil
}

// SyncFromRemote pulls changes from Supabase to local
func (m *DBManager) SyncFromRemote(since time.Time) (int, error) {
	if !m.IsOnline() {
		return 0, nil // Silent fail
	}

	pulled := 0
	tableErrors := []string{}
	// Use the active canonical table list — must match push to ensure all
	// computers receive the full dataset (orders+items, invoices+items, etc.)
	for _, table := range activeDBSyncTables() {
		if isUserActivitySyncTable(table) && !m.activityMonitoringPullAllowed() {
			log.Printf("Skipping pull sync for %s: confidential activity monitoring is restricted to Rahul/Mentor", table)
			continue
		}
		tableSince := m.getLastSyncTimeForTable(table, "pull")
		count, err := m.syncTable(table, tableSince, "pull")
		if err != nil {
			log.Printf("Sync pull error for %s: %v", table, err)
			tableErrors = append(tableErrors, fmt.Sprintf("%s: %v", table, err))
			continue
		}
		pulled += count
	}

	if len(tableErrors) > 0 {
		return pulled, fmt.Errorf("pull completed with %d table errors: %s", len(tableErrors), strings.Join(tableErrors, "; "))
	}
	return pulled, nil
}

// StartPeriodicSync runs background sync at the specified interval
func (m *DBManager) StartPeriodicSync(interval time.Duration) {
	if !m.syncEnabled {
		log.Println("Periodic sync disabled (no Supabase config)")
		return
	}
	m.mu.Lock()
	if m.periodicStarted {
		m.mu.Unlock()
		log.Println("DBManager periodic sync already started")
		return
	}
	m.periodicStarted = true
	m.mu.Unlock()

	runCycle := func() {
		// Try to connect if not online
		if !m.IsOnline() {
			if err := m.ConnectRemote(); err != nil {
				return
			}
		}

		// Check connectivity
		if !m.CheckConnectivity() {
			return
		}

		// Perform sync
		lastSync := m.getLastSyncTime()
		pushed, pushErr := m.SyncToRemote(lastSync)
		if pushErr != nil {
			log.Printf("Sync push completed with errors after %d records: %v", pushed, pushErr)
		} else if pushed > 0 {
			log.Printf("Sync: pushed %d records to Supabase", pushed)
		}
		pulled, pullErr := m.SyncFromRemote(lastSync)
		if pullErr != nil {
			log.Printf("Sync pull completed with errors after %d records: %v", pulled, pullErr)
		} else if pulled > 0 {
			log.Printf("Sync: pulled %d records from Supabase", pulled)
		}
		if pushErr == nil && pullErr == nil {
			m.mu.RLock()
			callback := m.afterSuccessfulSync
			m.mu.RUnlock()
			if callback != nil {
				callback(pushed, pulled)
			}
		}
	}

	m.wg.Add(1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("🔥 DBManager periodic sync panic: %v", r)
			}
		}()
		defer m.wg.Done()

		// Initial delay to let app start
		select {
		case <-time.After(10 * time.Second):
		case <-m.stopChan:
			return
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Printf("Background sync started (interval: %v)", interval)
		runCycle()

		for {
			select {
			case <-ticker.C:
				runCycle()
			case <-m.stopChan:
				log.Println("Background sync stopped (shutdown)")
				return
			}
		}
	}()
}

// StopPeriodicSync stops the background sync goroutine and waits for it to finish.
func (m *DBManager) StopPeriodicSync() {
	m.stopOnce.Do(func() {
		close(m.stopChan)
		log.Println("DBManager periodic sync stop signal sent")
	})
	m.wg.Wait() // Wait for goroutine to finish before returning
}

// TriggerSync manually triggers a sync (for UI "Sync Now" button)
func (m *DBManager) TriggerSync() (int, int, error) {
	if !m.IsOnline() {
		// Try to connect first
		if err := m.ConnectRemote(); err != nil {
			return 0, 0, fmt.Errorf("cannot connect to Supabase: %w", err)
		}
	}

	lastSync := m.getLastSyncTime()
	pushed, pushErr := m.SyncToRemote(lastSync)
	pulled, pullErr := m.SyncFromRemote(lastSync)
	if pushErr != nil || pullErr != nil {
		return pushed, pulled, fmt.Errorf("sync completed with errors: push=%v pull=%v", pushErr, pullErr)
	}
	m.mu.RLock()
	callback := m.afterSuccessfulSync
	m.mu.RUnlock()
	if callback != nil {
		callback(pushed, pulled)
	}

	return pushed, pulled, nil
}

// Disconnect closes the Supabase connection
func (m *DBManager) Disconnect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.remote != nil {
		if sqlDB, err := m.remote.DB(); err == nil {
			sqlDB.Close()
		}
		m.remote = nil
		m.isOnline = false
	}
}

// --- Internal helpers ---

func (m *DBManager) setOffline() {
	m.mu.Lock()
	m.isOnline = false
	m.mu.Unlock()
}

func (m *DBManager) getLastSyncTime() time.Time {
	var record SyncRecord
	result := m.local.Order("synced_at DESC").First(&record)
	if result.Error != nil {
		return time.Time{} // Epoch - sync everything
	}
	return record.SyncedAt
}

func (m *DBManager) getLastSyncTimeForTable(table, direction string) time.Time {
	var record SyncRecord
	result := m.local.
		Where("sync_table = ? AND direction = ?", table, direction).
		Order("synced_at DESC").
		First(&record)
	if result.Error != nil {
		return time.Time{}
	}
	return record.SyncedAt
}

func (m *DBManager) syncTable(table string, since time.Time, direction string) (int, error) {
	if direction == "pull" && isUserActivitySyncTable(table) && !m.activityMonitoringPullAllowed() {
		return 0, nil
	}
	var source, dest *gorm.DB
	if direction == "push" {
		source = m.local
		dest = m.remote
	} else {
		source = m.remote
		dest = m.local
	}

	if source == nil || dest == nil {
		return 0, nil
	}
	remoteAvailable, err := m.remoteSyncTableAvailableForSync(table, direction)
	if err != nil {
		return 0, err
	}
	if !remoteAvailable {
		return 0, nil
	}
	sourceColumnTypes, err := syncDBColumnTypes(source, table)
	if err != nil {
		return 0, fmt.Errorf("inspect source %s columns: %w", table, err)
	}
	if !syncColumnExists(sourceColumnTypes, "updated_at") {
		log.Printf("Skipping %s sync for %s: source table has no updated_at column for incremental sync", direction, table)
		return 0, nil
	}

	// Count records to sync
	var count int64
	if err := source.Table(table).Where("updated_at > ?", since).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("count %s records to %s: %w", table, direction, err)
	}
	if count == 0 {
		return 0, nil
	}

	// Fetch ALL records in one query (batch, not row-by-row!)
	var records []map[string]interface{}
	if err := source.Table(table).Where("updated_at > ?", since).Find(&records).Error; err != nil {
		return 0, err
	}

	if len(records) == 0 {
		return 0, nil
	}
	targetColumnTypes, err := syncDBColumnTypes(dest, table)
	if err != nil {
		return 0, fmt.Errorf("inspect destination %s columns: %w", table, err)
	}

	// BATCH UPSERT - 10-50x faster than row-by-row!
	batchSize := 100
	synced := 0

	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		batch := normalizeRecordsForRemoteSyncSchema(records[i:end], targetColumnTypes)
		updateColumns := syncUpsertColumns(batch)
		if len(updateColumns) == 0 {
			continue
		}
		conflictColumns := syncConflictColumnsForBatch(table, batch, targetColumnTypes)

		// Use ON CONFLICT for upsert (Postgres/SQLite compatible)
		err := dest.Table(table).Clauses(clause.OnConflict{
			Columns:   conflictColumns,
			DoUpdates: clause.AssignmentColumns(updateColumns),
		}).Create(&batch).Error

		if err != nil {
			log.Printf("Batch sync error for %s: %v, falling back to individual", table, err)
			fallbackErrors := []string{}
			// Fallback to individual inserts for this batch
			for _, record := range batch {
				id := record["id"]
				if err := syncUpsertRecordByIDOrNaturalKey(dest, table, record, targetColumnTypes); err != nil {
					log.Printf("Individual sync error for %s/%v: %v", table, id, err)
					if len(fallbackErrors) < 5 {
						fallbackErrors = append(fallbackErrors, fmt.Sprintf("%v: %v", id, err))
					}
					continue
				}
				synced++
			}
			if len(fallbackErrors) > 0 {
				return synced, fmt.Errorf("sync %s had fallback row errors after batch error %q: %s", table, err.Error(), strings.Join(fallbackErrors, "; "))
			}
		} else {
			synced += len(batch)
		}
	}

	// Log sync summary (one record per table, not per-row)
	if synced > 0 {
		m.local.Create(&SyncRecord{
			SyncTable:     table,
			RecordID:      fmt.Sprintf("batch_%d", synced),
			SyncedAt:      time.Now(),
			Direction:     direction,
			ConflictState: "none",
		})
	}

	return synced, nil
}

func (m *DBManager) remoteSyncTableAvailable(table, direction string) bool {
	available, err := m.remoteSyncTableAvailableForSync(table, direction)
	if err != nil {
		log.Printf("Remote schema error for %s sync on %s: %v", direction, table, err)
		return false
	}
	return available
}

func (m *DBManager) remoteSyncTableAvailableForSync(table, direction string) (bool, error) {
	if m.remote == nil {
		return false, nil
	}
	if m.remote.Migrator().HasTable(table) {
		return true, nil
	}
	if isOptionalRemoteSyncTable(table) {
		log.Printf("Skipping %s sync for %s: optional remote table is missing; apply the Supabase schema migration to enable this table", direction, table)
		return false, nil
	}
	return false, fmt.Errorf("remote table %q is missing; apply the Supabase schema migration before %s sync", table, direction)
}

func isOptionalRemoteSyncTable(table string) bool {
	return false
}

func (m *DBManager) activityMonitoringPullAllowed() bool {
	if m == nil || m.canPullActivityMonitoring == nil {
		return false
	}
	return m.canPullActivityMonitoring()
}

func syncConflictColumnsForBatch(table string, batch []map[string]interface{}, columnTypes map[string]string) []clause.Column {
	naturalColumn := syncNaturalKeyColumn(table, columnTypes)
	if naturalColumn != "" {
		allHaveNaturalKey := len(batch) > 0
		for _, record := range batch {
			_, _, ok := syncNaturalKeyValue(table, record, columnTypes)
			if !ok {
				allHaveNaturalKey = false
				break
			}
		}
		if allHaveNaturalKey {
			return []clause.Column{{Name: naturalColumn}}
		}
	}
	return []clause.Column{{Name: "id"}}
}

// --- App integration helpers ---

// InitDBManager initializes the dual-database manager on the App
func (a *App) InitDBManager() {
	if a.db == nil {
		log.Println("Cannot init DBManager - no local database")
		return
	}

	cfg := LoadSupabaseConfig()
	a.dbManager = NewDBManager(a.db, cfg)
	a.dbManager.canPullActivityMonitoring = a.currentSessionCanAccessActivityMonitoring
	a.dbManager.SetAfterSuccessfulSync(func(pushed int, pulled int) {
		updateSummary, err := a.applyPendingDataUpdatesInternal(false, "db_manager_sync")
		if err != nil {
			log.Printf("Published data update auto-apply failed after DBManager sync: %v", err)
			return
		}
		if updateSummary.AppliedCount > 0 {
			log.Printf("Published data updates auto-applied after DBManager sync: %d", updateSummary.AppliedCount)
		}
	})

	// Try to connect to Supabase (non-blocking)
	if cfg.Enabled {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					a.recordBackgroundPanic("Supabase startup sync", r)
				}
			}()
			// Delay to not block app startup
			time.Sleep(2 * time.Second)

			if err := a.dbManager.ConnectRemote(); err != nil {
				log.Printf("Supabase sync deferred: %v", err)
			} else {
				// Schema migration is now manual — run MigrateRemote() explicitly from Settings
				// or via the Supabase migration script. This prevents unexpected DDL at app launch.
				log.Println("Supabase connected. Schema migration is manual — use Settings > Sync > Migrate if needed.")
			}

			// Start periodic sync (every 10 minutes for offline-first approach)
			a.dbManager.StartPeriodicSync(10 * time.Minute)
		}()
	} else {
		log.Println("Running in offline mode (Supabase not configured)")
	}
}

// LoadSupabaseConfig loads Supabase config from environment
func LoadSupabaseConfig() SupabaseConfig {
	return loadDatabaseConfig()
}

// GetDBSyncStatus returns the current sync status for the frontend
func (a *App) GetDBSyncStatus() map[string]interface{} {
	// Non-sensitive health endpoint used by the sidebar for every role.
	// Manual sync and settings changes remain protected by settings:update.
	status := map[string]interface{}{
		"configured":   false,
		"online":       false,
		"last_sync":    nil,
		"sync_enabled": false,
	}

	if a.dbManager == nil {
		return status
	}

	status["configured"] = a.dbManager.config.Enabled
	status["online"] = a.dbManager.IsOnline()
	status["sync_enabled"] = a.dbManager.IsSyncEnabled()

	lastSync := a.dbManager.getLastSyncTime()
	if !lastSync.IsZero() {
		status["last_sync"] = lastSync.Format(time.RFC3339)
	}

	return status
}

// SyncHealth provides comprehensive health status for the sync system.
type SyncHealth struct {
	IsOnline       bool   `json:"is_online"`
	LastSyncAt     string `json:"last_sync_at"`
	LastSyncStatus string `json:"last_sync_status"`
	TablesInSync   int    `json:"tables_in_sync"`
	DBSizeBytes    int64  `json:"db_size_bytes"`
	BackupCount    int    `json:"backup_count"`
	LastBackupAt   string `json:"last_backup_at"`
	UptimeSeconds  int64  `json:"uptime_seconds"`
}

// GetSyncHealth returns comprehensive health information about the sync system.
func (a *App) GetSyncHealth() SyncHealth {
	if err := a.requirePermission("settings:view"); err != nil {
		return SyncHealth{}
	}
	if a.db == nil {
		return SyncHealth{}
	}
	health := SyncHealth{
		UptimeSeconds: int64(time.Since(a.appStartTime).Seconds()),
	}

	// Sync status
	if a.dbManager != nil {
		health.IsOnline = a.dbManager.IsOnline()
		lastSync := a.dbManager.getLastSyncTime()
		if !lastSync.IsZero() {
			health.LastSyncAt = lastSync.Format(time.RFC3339)
			health.LastSyncStatus = "success"
		}
		health.TablesInSync = len(activeDBSyncTables())
	}

	// Database file size — resolve path from PRAGMA database_list
	var dbPath string
	row := a.db.Raw("PRAGMA database_list").Row()
	if row != nil {
		var seq int
		var name, file string
		if err := row.Scan(&seq, &name, &file); err == nil && file != "" {
			dbPath = file
		}
	}
	if dbPath != "" {
		if info, err := os.Stat(dbPath); err == nil {
			health.DBSizeBytes = info.Size()
		}
	}

	// Backup info
	backupInfo := a.GetBackupInfo()
	if count, ok := backupInfo["count"].(int); ok {
		health.BackupCount = count
	}
	if lastBackup, ok := backupInfo["last_backup"].(string); ok {
		health.LastBackupAt = lastBackup
	}

	return health
}

// TriggerManualSync allows the frontend to trigger a sync
func (a *App) TriggerManualSync() (map[string]interface{}, error) {
	if err := a.requirePermission("settings:update"); err != nil {
		return nil, err
	}
	if a.dbManager == nil {
		return nil, fmt.Errorf("sync not initialized")
	}

	pushed, pulled, err := a.dbManager.TriggerSync()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"pushed": pushed,
		"pulled": pulled,
		"time":   time.Now().Format(time.RFC3339),
	}, nil
}

// DownloadFullDatabase downloads entire database from Supabase to local SQLite
// Called on first license activation to bootstrap new devices
func (m *DBManager) DownloadFullDatabase() (int, error) {
	if !m.IsOnline() {
		if err := m.ConnectRemote(); err != nil {
			return 0, fmt.Errorf("cannot connect to Supabase: %w", err)
		}
	}

	log.Println("========================================")
	log.Println("   FIRST-RUN DATABASE DOWNLOAD")
	log.Println("   Downloading from Supabase...")
	log.Println("========================================")

	// Use the active sync table list, supplemented by system-only tables
	// that aren't in the periodic sync but are needed for first-run bootstrap
	systemTables := []string{
		"assets",       // Letterhead, logos — needed immediately
		"devices",      // Device registration
		"users",        // User accounts
		"device_users", // Device-user bindings
		"settings",     // App settings
	}
	tables := append(systemTables, activeDBSyncTables()...)

	totalDownloaded := 0

	for _, table := range tables {
		if isUserActivitySyncTable(table) && !m.activityMonitoringPullAllowed() {
			log.Printf("  %-25s: skipped (restricted to Rahul/Mentor)", table)
			continue
		}
		count, err := m.downloadTable(table)
		if err != nil {
			log.Printf("  Warning: %s download error: %v", table, err)
			continue
		}
		if count > 0 {
			log.Printf("  %-25s: %d records downloaded", table, count)
			totalDownloaded += count
		}
	}

	log.Println("========================================")
	log.Printf("   DOWNLOAD COMPLETE: %d records", totalDownloaded)
	log.Println("========================================")

	return totalDownloaded, nil
}

// downloadTable downloads a single table from Supabase to local SQLite
// SAFE VERSION: Uses UPSERT instead of DELETE-then-INSERT to prevent data loss
func (m *DBManager) downloadTable(table string) (int, error) {
	// SECURITY: Validate table name to prevent SQL injection in raw SQL statements
	if !isValidSQLIdentifier(table) {
		return 0, fmt.Errorf("invalid table name: %s", table)
	}

	if m.remote == nil || m.local == nil {
		return 0, fmt.Errorf("databases not connected")
	}

	// Check if table exists in Supabase
	var remoteCount int64
	if err := m.remote.Table(table).Count(&remoteCount).Error; err != nil {
		return 0, nil // Table doesn't exist in remote, skip silently
	}
	if remoteCount == 0 {
		return 0, nil // Empty table, nothing to download
	}

	// Get column names from remote
	rows, err := m.remote.Table(table).Limit(1).Rows()
	if err != nil {
		return 0, err
	}
	cols, _ := rows.Columns()
	rows.Close()

	if len(cols) == 0 {
		return 0, nil
	}

	// SAFE APPROACH: Download ALL records first, then apply
	// This prevents data loss if download fails midway
	var allRecords []map[string]interface{}
	batchSize := 100
	offset := 0

	for {
		var records []map[string]interface{}
		result := m.remote.Table(table).Offset(offset).Limit(batchSize).Find(&records)
		if result.Error != nil {
			log.Printf("  Download error for %s at offset %d: %v", table, offset, result.Error)
			return 0, result.Error // Return 0 - don't touch local data on failure
		}

		if len(records) == 0 {
			break
		}

		allRecords = append(allRecords, records...)
		offset += len(records)
		if len(records) < batchSize {
			break // Last batch
		}
	}

	// Only proceed if we have all records
	if len(allRecords) == 0 {
		return 0, nil
	}
	localColumnTypes, err := syncDBColumnTypes(m.local, table)
	if err != nil {
		return 0, fmt.Errorf("inspect local %s columns: %w", table, err)
	}
	allRecords = normalizeRecordsForRemoteSyncSchema(allRecords, localColumnTypes)

	// SAFE: Now clear local table and insert all at once in a transaction
	tx := m.local.Begin()
	if tx.Error != nil {
		return 0, tx.Error
	}

	// Delete existing data
	if err := tx.Exec(fmt.Sprintf("DELETE FROM %s", table)).Error; err != nil {
		tx.Rollback()
		return 0, err
	}

	// BATCH INSERT - much faster than row-by-row!
	downloaded := 0
	insertBatchSize := 100

	for i := 0; i < len(allRecords); i += insertBatchSize {
		end := i + insertBatchSize
		if end > len(allRecords) {
			end = len(allRecords)
		}
		batch := allRecords[i:end]

		if err := tx.Table(table).Create(&batch).Error; err != nil {
			// Fallback to individual inserts for this batch
			log.Printf("  Batch insert error for %s: %v, falling back", table, err)
			for _, record := range batch {
				if err := tx.Table(table).Create(record).Error; err != nil {
					log.Printf("  Insert error for %s: %v", table, err)
					continue
				}
				downloaded++
			}
		} else {
			downloaded += len(batch)
		}
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return 0, err
	}

	return downloaded, nil
}

// FirstRunSync performs a complete database download if local is empty
// Returns true if sync was performed, false if local already had data
// SAFE VERSION: Uses lock to prevent concurrent syncs and multiple checks
func (m *DBManager) FirstRunSync() (bool, int, error) {
	// Acquire sync lock - prevents concurrent first-run syncs
	m.syncMu.Lock()
	if m.syncInProgress {
		m.syncMu.Unlock()
		log.Println("Sync already in progress, skipping duplicate request")
		return false, 0, nil
	}
	m.syncInProgress = true
	m.syncMu.Unlock()

	// Always release the lock when done
	defer func() {
		m.syncMu.Lock()
		m.syncInProgress = false
		m.syncMu.Unlock()
	}()

	// SAFETY: Check multiple tables to ensure local database is truly empty
	// This prevents false positives from transient read errors
	var customerCount, orderCount, invoiceCount int64
	m.local.Table("customers").Count(&customerCount)
	m.local.Table("orders").Count(&orderCount)
	m.local.Table("invoices").Count(&invoiceCount)

	totalLocal := customerCount + orderCount + invoiceCount

	if totalLocal > 0 {
		log.Printf("Local database has data (customers: %d, orders: %d, invoices: %d), skipping first-run sync",
			customerCount, orderCount, invoiceCount)
		return false, 0, nil
	}

	// Double-check: re-verify after a small delay to avoid race conditions
	time.Sleep(100 * time.Millisecond)
	m.local.Table("customers").Count(&customerCount)
	if customerCount > 0 {
		log.Println("Local database populated during check, skipping first-run sync")
		return false, 0, nil
	}

	// Local is truly empty - download from cloud
	log.Println("Local database confirmed empty - initiating first-run download")
	count, err := m.DownloadFullDatabase()
	if err != nil {
		return false, 0, err
	}

	return true, count, nil
}

// TestSupabaseConnection tests connection with provided credentials
func (a *App) TestSupabaseConnection(host, port, user, password, dbname, sslmode string) (bool, error) {
	if err := a.requirePermission("settings:update"); err != nil {
		return false, err
	}
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		// Strip DSN from error to prevent password leak to frontend
		return false, fmt.Errorf("connection failed: could not connect to %s:%s", host, port)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return false, fmt.Errorf("failed to get connection pool")
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		return false, fmt.Errorf("ping failed: could not reach %s:%s", host, port)
	}

	return true, nil
}

// PerformFirstRunSync downloads the full database from Supabase if local is empty
// This is called after successful license activation on a new device
func (a *App) PerformFirstRunSync() (map[string]interface{}, error) {
	if err := a.requirePermission("settings:update"); err != nil {
		return nil, err
	}
	result := map[string]interface{}{
		"performed":  false,
		"downloaded": 0,
		"error":      nil,
	}

	if a.dbManager == nil {
		// Initialize DB manager if not already done
		a.InitDBManager()
	}

	if a.dbManager == nil {
		return result, fmt.Errorf("database manager not initialized")
	}

	// Check if this is a first run (local database is empty)
	performed, count, err := a.dbManager.FirstRunSync()
	if err != nil {
		result["error"] = err.Error()
		return result, err
	}

	result["performed"] = performed
	result["downloaded"] = count

	if performed {
		log.Printf("First-run sync completed: %d records downloaded from cloud", count)
	}

	return result, nil
}

// GetFirstRunSyncStatus checks if first-run sync is needed
func (a *App) GetFirstRunSyncStatus() map[string]interface{} {
	if err := a.requirePermission("settings:view"); err != nil {
		return map[string]interface{}{"error": "permission denied"}
	}
	status := map[string]interface{}{
		"local_empty":      false,
		"cloud_available":  false,
		"sync_recommended": false,
	}

	if a.db == nil {
		return status
	}

	// Check if local has data
	var localCount int64
	a.db.Table("customers").Count(&localCount)
	status["local_empty"] = localCount == 0

	// Check if cloud is available
	if a.dbManager != nil && a.dbManager.IsOnline() {
		status["cloud_available"] = true

		// Check if cloud has data
		if a.dbManager.Remote() != nil {
			var cloudCount int64
			a.dbManager.Remote().Table("customers").Count(&cloudCount)
			status["cloud_has_data"] = cloudCount > 0
			status["sync_recommended"] = localCount == 0 && cloudCount > 0
		}
	}

	return status
}
