package main

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// SyncService handles bidirectional synchronization between local SQLite and remote Supabase
type DBSyncService struct {
	app               *App
	syncMutex         sync.Mutex
	stopChan          chan struct{}
	syncInterval      time.Duration
	lastSyncTimes     map[string]time.Time // table name -> last sync time
	isRunning         bool
	consecutiveErrors int
	maxErrors         int  // circuit breaker threshold
	circuitOpen       bool // circuit breaker state
}

// syncableModel represents a model that can be synchronized
type syncableModel struct {
	tableName string
	model     interface{}
	records   interface{} // pointer to slice for query results
}

// getSyncableModels returns the list of models that should be synced
func (s *DBSyncService) getSyncableModels() []syncableModel {
	return []syncableModel{
		{"customer_masters", &CustomerMaster{}, &[]CustomerMaster{}},
		{"supplier_masters", &SupplierMaster{}, &[]SupplierMaster{}},
		{"invoices", &Invoice{}, &[]Invoice{}},
		{"supplier_invoices", &SupplierInvoice{}, &[]SupplierInvoice{}},
		{"purchase_orders", &PurchaseOrder{}, &[]PurchaseOrder{}},
		{"orders", &Order{}, &[]Order{}},
		{"payments", &Payment{}, &[]Payment{}},
		{"supplier_payments", &SupplierPayment{}, &[]SupplierPayment{}},
		{"offers", &Offer{}, &[]Offer{}},
		{"delivery_notes", &DeliveryNote{}, &[]DeliveryNote{}},
		{"goods_received_notes", &GoodsReceivedNote{}, &[]GoodsReceivedNote{}},
	}
}

// newSyncService creates a new sync service instance
func newDBSyncService(app *App) *DBSyncService {
	return &DBSyncService{
		app:               app,
		stopChan:          make(chan struct{}),
		syncInterval:      5 * time.Minute,
		lastSyncTimes:     make(map[string]time.Time),
		isRunning:         false,
		consecutiveErrors: 0,
		maxErrors:         5, // circuit breaker threshold
		circuitOpen:       false,
	}
}

// PullChanges queries remote DB for records updated after 'since' and merges into local
func (a *App) PullChanges(since time.Time) error {
	if err := a.requirePermission("settings:manage"); err != nil {
		return err
	}

	// Try to acquire lock with timeout to prevent indefinite blocking
	done := make(chan struct{})
	go func() {
		a.dbSyncService.syncMutex.Lock()
		close(done)
	}()

	select {
	case <-done:
		defer a.dbSyncService.syncMutex.Unlock()
	case <-time.After(30 * time.Second):
		log.Println("[SYNC] ⚠️ Could not acquire sync lock within 30s for pull, skipping")
		return fmt.Errorf("sync lock timeout")
	}

	log.Printf("[SYNC] Starting pull from remote (since: %v)", since)

	// Check if remote is available
	if !a.dbManager.IsOnline() {
		log.Println("[SYNC] Remote unavailable, skipping pull")
		return fmt.Errorf("remote database unavailable")
	}

	remoteDB := a.dbManager.Remote()
	localDB := a.dbManager.Local()

	if remoteDB == nil || localDB == nil {
		return fmt.Errorf("database connections not available")
	}

	models := a.dbSyncService.getSyncableModels()
	totalPulled := 0
	totalConflicts := 0

	for _, sm := range models {
		pulled, conflicts, err := a.dbSyncService.pullTableChanges(localDB, remoteDB, sm, since)
		if err != nil {
			log.Printf("[SYNC] Error pulling %s: %v", sm.tableName, err)
			continue
		}
		totalPulled += pulled
		totalConflicts += conflicts

		// Update last sync time for this table
		a.dbSyncService.lastSyncTimes[sm.tableName] = time.Now()
	}

	log.Printf("[SYNC] Pull complete: %d records pulled, %d conflicts resolved", totalPulled, totalConflicts)
	return nil
}

// pullTableChanges handles pulling changes for a single table
func (s *DBSyncService) pullTableChanges(localDB, remoteDB *gorm.DB, sm syncableModel, since time.Time) (int, int, error) {
	// Query remote for records updated after 'since'
	var remoteRecords []map[string]interface{}

	err := remoteDB.Table(sm.tableName).
		Where("updated_at > ?", since).
		Find(&remoteRecords).Error

	if err != nil {
		return 0, 0, fmt.Errorf("query remote failed: %w", err)
	}

	if len(remoteRecords) == 0 {
		return 0, 0, nil
	}

	pulled := 0
	conflicts := 0

	for _, remoteRecord := range remoteRecords {
		recordID, ok := remoteRecord["id"].(string)
		if !ok {
			continue
		}

		// Check if record exists locally
		var localRecord map[string]interface{}
		result := localDB.Table(sm.tableName).Where("id = ?", recordID).Find(&localRecord)

		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			log.Printf("[SYNC] Error checking local record %s: %v", recordID, result.Error)
			continue
		}

		if result.RowsAffected == 0 {
			// Record doesn't exist locally - insert it
			if err := localDB.Table(sm.tableName).Create(&remoteRecord).Error; err != nil {
				log.Printf("[SYNC] Error inserting record %s: %v", recordID, err)
				continue
			}
			pulled++
			s.recordSyncOperation(localDB, sm.tableName, recordID, "pull", getVersionFromMap(remoteRecord), 0, "")
		} else {
			// Record exists - check for conflicts
			remoteVersion := getVersionFromMap(remoteRecord)
			localVersion := getVersionFromMap(localRecord)
			remoteUpdatedAt := getTimeFromMap(remoteRecord, "updated_at")
			localUpdatedAt := getTimeFromMap(localRecord, "updated_at")

			conflictState := ""
			shouldUpdate := false

			if remoteVersion > localVersion {
				// Remote is newer by version
				shouldUpdate = true
				conflictState = "resolved_remote_version"
			} else if remoteVersion < localVersion {
				// Local is newer - will be handled by push
				conflictState = "resolved_local_version"
			} else if remoteVersion == localVersion {
				// Same version, check timestamps
				if remoteUpdatedAt.After(localUpdatedAt) {
					shouldUpdate = true
					conflictState = "resolved_remote_timestamp"
					conflicts++
				} else {
					conflictState = "resolved_local_timestamp"
				}
			}

			if shouldUpdate {
				// Update local record with remote data
				if err := localDB.Table(sm.tableName).Where("id = ?", recordID).Updates(remoteRecord).Error; err != nil {
					log.Printf("[SYNC] Error updating record %s: %v", recordID, err)
					continue
				}
				pulled++
			}

			if conflictState != "" {
				s.recordSyncOperation(localDB, sm.tableName, recordID, "pull", remoteVersion, localVersion, conflictState)
			}
		}
	}

	return pulled, conflicts, nil
}

// PushChanges pushes local changes not yet synced to remote
func (a *App) PushChanges() error {
	if err := a.requirePermission("settings:manage"); err != nil {
		return err
	}

	// Try to acquire lock with timeout to prevent indefinite blocking
	done := make(chan struct{})
	go func() {
		a.dbSyncService.syncMutex.Lock()
		close(done)
	}()

	select {
	case <-done:
		defer a.dbSyncService.syncMutex.Unlock()
	case <-time.After(30 * time.Second):
		log.Println("[SYNC] ⚠️ Could not acquire sync lock within 30s for push, skipping")
		return fmt.Errorf("sync lock timeout")
	}

	log.Println("[SYNC] Starting push to remote")

	// Check if remote is available
	if !a.dbManager.IsOnline() {
		log.Println("[SYNC] Remote unavailable, skipping push")
		return fmt.Errorf("remote database unavailable")
	}

	remoteDB := a.dbManager.Remote()
	localDB := a.dbManager.Local()

	if remoteDB == nil || localDB == nil {
		return fmt.Errorf("database connections not available")
	}

	models := a.dbSyncService.getSyncableModels()
	totalPushed := 0
	totalConflicts := 0

	for _, sm := range models {
		pushed, conflicts, err := a.dbSyncService.pushTableChanges(localDB, remoteDB, sm)
		if err != nil {
			log.Printf("[SYNC] Error pushing %s: %v", sm.tableName, err)
			continue
		}
		totalPushed += pushed
		totalConflicts += conflicts
	}

	log.Printf("[SYNC] Push complete: %d records pushed, %d conflicts resolved", totalPushed, totalConflicts)
	return nil
}

// pushTableChanges handles pushing changes for a single table
func (s *DBSyncService) pushTableChanges(localDB, remoteDB *gorm.DB, sm syncableModel) (int, int, error) {
	// Find local records that haven't been synced or have been modified since last sync
	lastSync, exists := s.lastSyncTimes[sm.tableName]
	if !exists {
		lastSync = time.Time{} // Unix epoch if never synced
	}

	var localRecords []map[string]interface{}
	err := localDB.Table(sm.tableName).
		Where("updated_at > ?", lastSync).
		Find(&localRecords).Error

	if err != nil {
		return 0, 0, fmt.Errorf("query local failed: %w", err)
	}

	if len(localRecords) == 0 {
		return 0, 0, nil
	}

	pushed := 0
	conflicts := 0

	for _, localRecord := range localRecords {
		recordID, ok := localRecord["id"].(string)
		if !ok {
			continue
		}

		// Check if record exists remotely
		var remoteRecord map[string]interface{}
		result := remoteDB.Table(sm.tableName).Where("id = ?", recordID).Find(&remoteRecord)

		if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
			log.Printf("[SYNC] Error checking remote record %s: %v", recordID, result.Error)
			continue
		}

		if result.RowsAffected == 0 {
			// Record doesn't exist remotely - insert it
			if err := remoteDB.Table(sm.tableName).Create(&localRecord).Error; err != nil {
				log.Printf("[SYNC] Error inserting remote record %s: %v", recordID, err)
				continue
			}
			pushed++
			s.recordSyncOperation(localDB, sm.tableName, recordID, "push", 0, getVersionFromMap(localRecord), "")
		} else {
			// Record exists - check for conflicts
			localVersion := getVersionFromMap(localRecord)
			remoteVersion := getVersionFromMap(remoteRecord)
			localUpdatedAt := getTimeFromMap(localRecord, "updated_at")
			remoteUpdatedAt := getTimeFromMap(remoteRecord, "updated_at")

			conflictState := ""
			shouldUpdate := false

			if localVersion > remoteVersion {
				// Local is newer by version
				shouldUpdate = true
				conflictState = "resolved_local_version"
			} else if localVersion < remoteVersion {
				// Remote is newer - should have been pulled
				conflictState = "resolved_remote_version"
			} else if localVersion == remoteVersion {
				// Same version, check timestamps
				if localUpdatedAt.After(remoteUpdatedAt) {
					shouldUpdate = true
					conflictState = "resolved_local_timestamp"
					conflicts++
				} else {
					conflictState = "resolved_remote_timestamp"
				}
			}

			if shouldUpdate {
				// Update remote record with local data
				if err := remoteDB.Table(sm.tableName).Where("id = ?", recordID).Updates(localRecord).Error; err != nil {
					log.Printf("[SYNC] Error updating remote record %s: %v", recordID, err)
					continue
				}
				pushed++
			}

			if conflictState != "" {
				s.recordSyncOperation(localDB, sm.tableName, recordID, "push", remoteVersion, localVersion, conflictState)
			}
		}
	}

	return pushed, conflicts, nil
}

// StartPeriodicSync starts background sync with specified interval (default 5min)
func (a *App) StartPeriodicSync(interval time.Duration) {
	a.dbSyncService.syncMutex.Lock()
	if a.dbSyncService.isRunning {
		a.dbSyncService.syncMutex.Unlock()
		log.Println("[SYNC] Periodic sync already running")
		return
	}

	if interval > 0 {
		a.dbSyncService.syncInterval = interval
	}

	a.dbSyncService.isRunning = true
	a.dbSyncService.stopChan = make(chan struct{})
	a.dbSyncService.syncMutex.Unlock()

	log.Printf("[SYNC] Starting periodic sync (interval: %v)", a.dbSyncService.syncInterval)

	go func() {
		ticker := time.NewTicker(a.dbSyncService.syncInterval)
		defer ticker.Stop()

		// Run initial sync immediately
		a.performBidirectionalSync()

		for {
			select {
			case <-ticker.C:
				a.performBidirectionalSync()
			case <-a.dbSyncService.stopChan:
				log.Println("[SYNC] Periodic sync stopped")
				return
			}
		}
	}()
}

// StopPeriodicSync stops the background sync goroutine
func (a *App) StopPeriodicSync() {
	a.dbSyncService.syncMutex.Lock()
	defer a.dbSyncService.syncMutex.Unlock()

	if !a.dbSyncService.isRunning {
		log.Println("[SYNC] Periodic sync not running")
		return
	}

	log.Println("[SYNC] Stopping periodic sync...")
	close(a.dbSyncService.stopChan)
	a.dbSyncService.isRunning = false
}

// performBidirectionalSync executes both pull and push operations
func (a *App) performBidirectionalSync() {
	// Circuit breaker check
	if a.dbSyncService.circuitOpen {
		log.Println("[SYNC] ⚠️ Circuit breaker OPEN, skipping sync")
		return
	}

	log.Println("[SYNC] === Starting bidirectional sync ===")

	// Determine the earliest last sync time across all tables
	var oldestSync time.Time
	for _, t := range a.dbSyncService.lastSyncTimes {
		if oldestSync.IsZero() || t.Before(oldestSync) {
			oldestSync = t
		}
	}

	// If never synced, use a reasonable lookback (e.g., 1 week)
	if oldestSync.IsZero() {
		oldestSync = time.Now().Add(-7 * 24 * time.Hour)
	}

	// Track if sync succeeded
	syncFailed := false

	// Pull changes first (remote → local)
	if err := a.PullChanges(oldestSync); err != nil {
		log.Printf("[SYNC] Pull failed: %v", err)
		syncFailed = true
	}

	// Then push changes (local → remote)
	if err := a.PushChanges(); err != nil {
		log.Printf("[SYNC] Push failed: %v", err)
		syncFailed = true
	}

	// Update circuit breaker state
	if syncFailed {
		a.dbSyncService.consecutiveErrors++
		if a.dbSyncService.consecutiveErrors >= a.dbSyncService.maxErrors {
			a.dbSyncService.circuitOpen = true
			log.Printf("[SYNC] ⚠️ Circuit breaker OPEN after %d consecutive errors", a.dbSyncService.consecutiveErrors)
		}
	} else {
		// Success - reset circuit breaker
		a.dbSyncService.consecutiveErrors = 0
		if a.dbSyncService.circuitOpen {
			log.Println("[SYNC] ✓ Circuit breaker CLOSED (sync recovered)")
		}
		a.dbSyncService.circuitOpen = false
	}

	log.Println("[SYNC] === Bidirectional sync complete ===")
}

// recordSyncOperation creates a SyncRecord entry to track the sync operation
func (s *DBSyncService) recordSyncOperation(db *gorm.DB, tableName, recordID, direction string, remoteVersion, localVersion int, conflictState string) {
	syncRecord := SyncRecord{
		Base: Base{
			ID: uuid.New().String(),
		},
		SyncTable:     tableName,
		RecordID:      recordID,
		SyncedAt:      time.Now(),
		Direction:     direction,
		RemoteVersion: remoteVersion,
		LocalVersion:  localVersion,
		ConflictState: conflictState,
	}

	if err := db.Create(&syncRecord).Error; err != nil {
		log.Printf("[SYNC] Failed to record sync operation: %v", err)
	}
}

// Helper functions to extract data from map[string]interface{}

func getVersionFromMap(m map[string]interface{}) int {
	if v, ok := m["version"]; ok {
		switch val := v.(type) {
		case int:
			return val
		case int64:
			return int(val)
		case float64:
			return int(val)
		}
	}
	return 0
}

func getTimeFromMap(m map[string]interface{}, key string) time.Time {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case time.Time:
			return val
		case string:
			t, err := time.Parse(time.RFC3339, val)
			if err == nil {
				return t
			}
		}
	}
	return time.Time{}
}
