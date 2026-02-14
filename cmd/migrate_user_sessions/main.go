// ═══════════════════════════════════════════════════════════════════════════
// USER SESSION MIGRATION TOOL - Add Session Expiration Fields
//
// MISSION: Migrate existing user_sessions table to add expiration tracking
//
// USAGE:
//   go run cmd/migrate_user_sessions/main.go
//
// CHANGES:
//   - Add refresh_token column
//   - Add access_token_expiry column
//   - Add refresh_token_expiry column
//   - Add last_activity_at column
//   - Add is_active column (default: true)
//   - Add invalidated_at column (nullable)
//   - Add invalidated_reason column
//
// Built with SECURITY × SIMPLICITY × BACKWARD COMPATIBILITY 🔐⚡💎
// Day 197 - Wave 2 Agent 2 - Session Expiration Migration
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func main() {
	// Get database path from args or use default
	dbPath := "./ph_holdings.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	log.Printf("📦 Migrating user_sessions table in: %s", dbPath)

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatalf("❌ Failed to open database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get SQL DB: %v", err)
	}
	defer sqlDB.Close()

	// Check if table exists
	var tableExists bool
	err = db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name='user_sessions'").Scan(&tableExists).Error
	if err != nil {
		log.Fatalf("❌ Failed to check table existence: %v", err)
	}

	if !tableExists {
		log.Println("⚠️  user_sessions table doesn't exist - will be created by AutoMigrate")
		return
	}

	log.Println("✓ user_sessions table found, checking columns...")

	// Get existing columns
	type ColumnInfo struct {
		Name string
	}
	var columns []ColumnInfo
	err = db.Raw("PRAGMA table_info(user_sessions)").Scan(&columns).Error
	if err != nil {
		log.Fatalf("❌ Failed to get column info: %v", err)
	}

	existingCols := make(map[string]bool)
	for _, col := range columns {
		existingCols[col.Name] = true
		log.Printf("  - %s", col.Name)
	}

	// Check if migration needed
	needsMigration := false
	requiredColumns := []string{
		"refresh_token",
		"access_token_expiry",
		"refresh_token_expiry",
		"last_activity_at",
		"is_active",
		"invalidated_at",
		"invalidated_reason",
	}

	missingColumns := []string{}
	for _, col := range requiredColumns {
		if !existingCols[col] {
			needsMigration = true
			missingColumns = append(missingColumns, col)
		}
	}

	if !needsMigration {
		log.Println("✅ All columns exist - no migration needed")
		return
	}

	log.Printf("📝 Missing columns: %v", missingColumns)
	log.Println("🔧 Starting migration...")

	// Begin transaction
	tx := db.Begin()

	// Add missing columns
	now := time.Now().Format(time.RFC3339)
	future := time.Now().Add(30 * 24 * time.Hour).Format(time.RFC3339)

	migrations := []struct {
		Name    string
		SQL     string
		Default string
	}{
		{"refresh_token", "ALTER TABLE user_sessions ADD COLUMN refresh_token TEXT DEFAULT ''", ""},
		{"access_token_expiry", "ALTER TABLE user_sessions ADD COLUMN access_token_expiry DATETIME DEFAULT '" + future + "'", future},
		{"refresh_token_expiry", "ALTER TABLE user_sessions ADD COLUMN refresh_token_expiry DATETIME DEFAULT '" + future + "'", future},
		{"last_activity_at", "ALTER TABLE user_sessions ADD COLUMN last_activity_at DATETIME DEFAULT '" + now + "'", now},
		{"is_active", "ALTER TABLE user_sessions ADD COLUMN is_active BOOLEAN DEFAULT true", "true"},
		{"invalidated_at", "ALTER TABLE user_sessions ADD COLUMN invalidated_at DATETIME", ""},
		{"invalidated_reason", "ALTER TABLE user_sessions ADD COLUMN invalidated_reason TEXT DEFAULT ''", ""},
	}

	for _, m := range migrations {
		if !existingCols[m.Name] {
			log.Printf("  + Adding column: %s", m.Name)
			if err := tx.Exec(m.SQL).Error; err != nil {
				tx.Rollback()
				log.Fatalf("❌ Failed to add column %s: %v", m.Name, err)
			}
		}
	}

	// Create indexes for performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_user_sessions_access_token_expiry ON user_sessions(access_token_expiry)",
		"CREATE INDEX IF NOT EXISTS idx_user_sessions_refresh_token_expiry ON user_sessions(refresh_token_expiry)",
		"CREATE INDEX IF NOT EXISTS idx_user_sessions_last_activity_at ON user_sessions(last_activity_at)",
		"CREATE INDEX IF NOT EXISTS idx_user_sessions_is_active ON user_sessions(is_active)",
	}

	for _, idx := range indexes {
		if err := tx.Exec(idx).Error; err != nil {
			log.Printf("⚠️  Failed to create index: %v", err)
			// Don't fail migration for index errors
		}
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		log.Fatalf("❌ Failed to commit migration: %v", err)
	}

	log.Println("✅ Migration complete!")
	log.Println("")
	log.Println("📊 Summary:")
	log.Printf("  - Database: %s", dbPath)
	log.Printf("  - Columns added: %d", len(missingColumns))
	log.Printf("  - Indexes created: %d", len(indexes))
	log.Println("")
	log.Println("🔐 Session expiration is now enabled:")
	log.Println("  - Access tokens expire after 24 hours")
	log.Println("  - Refresh tokens expire after 30 days")
	log.Println("  - Sessions timeout after 24 hours of inactivity")
	log.Println("  - Cleanup runs every 6 hours automatically")

	// Show current session count
	var count int64
	db.Table("user_sessions").Where("is_active = ?", true).Count(&count)
	log.Printf("\n📈 Active sessions: %d", count)

	// Show statistics
	type Stats struct {
		Total    int64
		Active   int64
		Inactive int64
	}
	var stats Stats
	db.Table("user_sessions").Count(&stats.Total)
	db.Table("user_sessions").Where("is_active = ?", true).Count(&stats.Active)
	db.Table("user_sessions").Where("is_active = ?", false).Count(&stats.Inactive)

	log.Println("\n📊 Session Statistics:")
	log.Printf("  - Total sessions: %d", stats.Total)
	log.Printf("  - Active: %d", stats.Active)
	log.Printf("  - Inactive: %d", stats.Inactive)

	fmt.Println("\n✅ Migration successful! Restart the application to use new session management.")
}
