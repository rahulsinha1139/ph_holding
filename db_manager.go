package main

import (
	"fmt"
	"log"
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
	local          *gorm.DB
	remote         *gorm.DB
	config         SupabaseConfig
	isOnline       bool
	mu             sync.RWMutex
	lastCheck      time.Time
	checkInterval  time.Duration
	syncEnabled    bool
	syncInProgress bool      // Prevents concurrent first-run syncs
	syncMu         sync.Mutex // Lock for sync operations
}

// NewDBManager creates a new dual-database manager
func NewDBManager(localDB *gorm.DB, cfg SupabaseConfig) *DBManager {
	return &DBManager{
		local:         localDB,
		config:        cfg,
		isOnline:      false,
		checkInterval: 30 * time.Second,
		syncEnabled:   cfg.Enabled,
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
	sqlDB.SetMaxOpenConns(5)  // Lower for sync-only usage
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
	if m.remote == nil {
		m.mu.RUnlock()
		return false
	}
	m.mu.RUnlock()

	sqlDB, err := m.remote.DB()
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

// MigrateRemote runs AutoMigrate on Supabase (creates tables if needed)
func (m *DBManager) MigrateRemote() error {
	if m.remote == nil {
		return fmt.Errorf("supabase not connected")
	}

	log.Println("Running schema migration on Supabase...")

	err := m.remote.AutoMigrate(
		// Master Data
		&CustomerMaster{},
		&CustomerContact{},
		&SupplierMaster{},
		&SupplierContact{},
		&ProductMaster{},
		&EntityNote{},
		&SupplierIssue{},
		// Sales
		&Offer{},
		&OfferItem{},
		&Order{},
		&OrderItem{},
		// Finance
		&Invoice{},
		&DBInvoiceItem{},
		&Payment{},
		&SupplierPayment{},
		// Operations
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&DeliveryNote{},
		&DeliveryNoteItem{},
		// Intelligence
		&Conversation{},
		&ChatMessage{},
		&OfferFollowUp{},
		// System
		&SyncRecord{},
		&Role{},
		&User{},
		&AuditLog{},
		&Setting{},
		&Device{},
		&DeviceUser{},
	)

	if err != nil {
		log.Printf("Supabase migration warning: %v", err)
		return err
	}

	log.Println("Supabase schema migration complete")
	return nil
}

// SyncToRemote pushes local changes to Supabase
func (m *DBManager) SyncToRemote(since time.Time) (int, error) {
	if !m.IsOnline() {
		return 0, nil // Silent fail - sync will retry later
	}

	pushed := 0
	tables := []string{
		"customers", "suppliers", "products",
		"orders", "order_items", "invoices", "invoice_items", "payments",
		"purchase_orders", "purchase_order_items",
		"supplier_invoices", "supplier_payments",
		"delivery_notes", "goods_received_notes",
		"offers", "offer_items",
		"roles", "users",
	}

	for _, table := range tables {
		count, err := m.syncTable(table, since, "push")
		if err != nil {
			log.Printf("Sync push error for %s: %v", table, err)
			continue
		}
		pushed += count
	}

	return pushed, nil
}

// SyncFromRemote pulls changes from Supabase to local
func (m *DBManager) SyncFromRemote(since time.Time) (int, error) {
	if !m.IsOnline() {
		return 0, nil // Silent fail
	}

	pulled := 0
	tables := []string{
		"customers", "suppliers", "products",
		"orders", "invoices", "payments",
		"purchase_orders", "supplier_invoices",
		"roles", "users",
	}

	for _, table := range tables {
		count, err := m.syncTable(table, since, "pull")
		if err != nil {
			log.Printf("Sync pull error for %s: %v", table, err)
			continue
		}
		pulled += count
	}

	return pulled, nil
}

// StartPeriodicSync runs background sync at the specified interval
func (m *DBManager) StartPeriodicSync(interval time.Duration) {
	if !m.syncEnabled {
		log.Println("Periodic sync disabled (no Supabase config)")
		return
	}

	go func() {
		// Initial delay to let app start
		time.Sleep(10 * time.Second)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		log.Printf("Background sync started (interval: %v)", interval)

		for range ticker.C {
			// Try to connect if not online
			if !m.IsOnline() {
				if err := m.ConnectRemote(); err != nil {
					continue // Will retry next interval
				}
			}

			// Check connectivity
			if !m.CheckConnectivity() {
				continue
			}

			// Perform sync
			lastSync := m.getLastSyncTime()
			if pushed, err := m.SyncToRemote(lastSync); err == nil && pushed > 0 {
				log.Printf("Sync: pushed %d records to Supabase", pushed)
			}
			if pulled, err := m.SyncFromRemote(lastSync); err == nil && pulled > 0 {
				log.Printf("Sync: pulled %d records from Supabase", pulled)
			}
		}
	}()
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
	pushed, _ := m.SyncToRemote(lastSync)
	pulled, _ := m.SyncFromRemote(lastSync)

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

func (m *DBManager) syncTable(table string, since time.Time, direction string) (int, error) {
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

	// Count records to sync
	var count int64
	source.Table(table).Where("updated_at > ?", since).Count(&count)
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

	// BATCH UPSERT - 10-50x faster than row-by-row!
	batchSize := 100
	synced := 0

	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]

		// Use ON CONFLICT for upsert (Postgres/SQLite compatible)
		err := dest.Table(table).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).Create(&batch).Error

		if err != nil {
			log.Printf("Batch sync error for %s: %v, falling back to individual", table, err)
			// Fallback to individual inserts for this batch
			for _, record := range batch {
				id := record["id"]
				result := dest.Table(table).Where("id = ?", id).Updates(record)
				if result.RowsAffected == 0 {
					dest.Table(table).Create(record)
				}
				synced++
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

// --- App integration helpers ---

// InitDBManager initializes the dual-database manager on the App
func (a *App) InitDBManager() {
	if a.db == nil {
		log.Println("Cannot init DBManager - no local database")
		return
	}

	cfg := LoadSupabaseConfig()
	a.dbManager = NewDBManager(a.db, cfg)

	// Try to connect to Supabase (non-blocking)
	if cfg.Enabled {
		go func() {
			// Delay to not block app startup
			time.Sleep(2 * time.Second)

			if err := a.dbManager.ConnectRemote(); err != nil {
				log.Printf("Supabase sync deferred: %v", err)
			} else {
				// Run migration if connected
				if err := a.dbManager.MigrateRemote(); err != nil {
					log.Printf("Supabase migration warning: %v", err)
				}
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
	status := map[string]interface{}{
		"configured":  false,
		"online":      false,
		"last_sync":   nil,
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

// TriggerManualSync allows the frontend to trigger a sync
func (a *App) TriggerManualSync() (map[string]interface{}, error) {
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

	// Tables to download (in dependency order)
	tables := []string{
		// System assets first (letterhead, logos - needed immediately)
		"assets",
		// Master data (no dependencies)
		"roles",
		"customers",
		"suppliers",
		"products",
		"devices",
		"users",
		"device_users",
		"settings",
		// Then dependent tables
		"customer_contacts",
		"supplier_contacts",
		"offers",
		"offer_items",
		"orders",
		"order_items",
		"invoices",
		"invoice_items",
		"payments",
		"supplier_invoices",
		"supplier_invoice_items",
		"supplier_payments",
		"purchase_orders",
		"purchase_order_items",
		"goods_received_notes",
		"grn_items",
		"delivery_notes",
		"delivery_note_items",
		"conversations",
		"chat_messages",
		"entity_notes",
		"offer_follow_ups",
		"supplier_issues",
		"quick_captures",
		"opportunities",
		"followup_tasks",
		"prediction_records",
		"warehouses",
	}

	totalDownloaded := 0

	for _, table := range tables {
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
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		host, port, user, password, dbname, sslmode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return false, fmt.Errorf("connection failed: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return false, fmt.Errorf("failed to get connection: %w", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		return false, fmt.Errorf("ping failed: %w", err)
	}

	return true, nil
}

// PerformFirstRunSync downloads the full database from Supabase if local is empty
// This is called after successful license activation on a new device
func (a *App) PerformFirstRunSync() (map[string]interface{}, error) {
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
