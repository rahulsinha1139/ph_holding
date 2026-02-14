package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// Tables to clear and re-sync (in dependency order)
var tablesToSync = []string{
	// Master data first (no foreign keys)
	"roles",
	"customers",
	"suppliers",
	"products",
	"devices",
	"users",
	"settings",
	// Then dependent tables
	"customer_contacts",
	"supplier_contacts",
	"device_users",
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
	"entity_notes",
	"conversations",
	"chat_messages",
	"offer_follow_ups",
	"supplier_issues",
	"quick_captures",
	"opportunities",
	"followup_tasks",
	"prediction_records",
	"warehouses",
	"assets",
}

func main() {
	log.Println("╔═══════════════════════════════════════════════════════════════════╗")
	log.Println("║       SUPABASE CLEAR & RE-SYNC UTILITY                           ║")
	log.Println("║       Fixes duplicate key violations by clearing cloud           ║")
	log.Println("╚═══════════════════════════════════════════════════════════════════╝")
	log.Println()

	// Load environment
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found, using system env")
	}

	// Connect to local SQLite
	log.Println("📁 Connecting to local SQLite database...")
	localDB, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to SQLite: %v", err)
	}
	log.Println("✅ Local SQLite connected")

	// Connect to Supabase
	log.Println("☁️  Connecting to Supabase PostgreSQL...")
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("SUPABASE_DB_HOST"),
		getEnvOrDefault("SUPABASE_DB_PORT", "5432"),
		os.Getenv("SUPABASE_DB_USER"),
		os.Getenv("SUPABASE_DB_PASSWORD"),
		getEnvOrDefault("SUPABASE_DB_NAME", "postgres"),
		getEnvOrDefault("SUPABASE_DB_SSLMODE", "require"),
	)

	remoteDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to Supabase: %v", err)
	}

	// Test connection
	sqlDB, _ := remoteDB.DB()
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("❌ Supabase ping failed: %v", err)
	}
	log.Println("✅ Supabase connected")
	log.Println()

	// Count local data
	var localCustomers, localContacts int64
	localDB.Table("customers").Count(&localCustomers)
	localDB.Table("customer_contacts").Count(&localContacts)
	log.Printf("📊 Local SQLite data: %d customers, %d contacts", localCustomers, localContacts)

	// Count remote data BEFORE clear
	var remoteCustomers, remoteContacts int64
	remoteDB.Table("customers").Count(&remoteCustomers)
	remoteDB.Table("customer_contacts").Count(&remoteContacts)
	log.Printf("📊 Supabase BEFORE: %d customers, %d contacts", remoteCustomers, remoteContacts)
	log.Println()

	// STEP 1: Clear Supabase tables (in reverse dependency order)
	log.Println("🧹 STEP 1: Clearing Supabase tables...")
	cleared := 0
	for i := len(tablesToSync) - 1; i >= 0; i-- {
		table := tablesToSync[i]
		log.Printf("   Clearing %s...", table)
		result := remoteDB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
		if result.Error != nil {
			// Table might not exist, skip
			log.Printf("   ⚠️  Skip %s: %v", table, result.Error)
		} else {
			cleared++
		}
	}
	log.Printf("✅ Cleared %d tables on Supabase", cleared)
	log.Println()

	// STEP 2: Upload ALL data from local SQLite to Supabase
	log.Println("⬆️  STEP 2: Uploading local data to Supabase...")
	totalUploaded := 0

	for _, table := range tablesToSync {
		// Count local records
		var count int64
		localDB.Table(table).Count(&count)
		if count == 0 {
			continue
		}

		log.Printf("   Uploading %s (%d records)...", table, count)

		// Fetch all records
		var records []map[string]interface{}
		if err := localDB.Table(table).Find(&records).Error; err != nil {
			log.Printf("   ❌ Failed to read %s: %v", table, err)
			continue
		}

		if len(records) == 0 {
			continue
		}

		// Upload in batches using UPSERT
		batchSize := 100
		uploaded := 0
		failed := 0

		for i := 0; i < len(records); i += batchSize {
			end := i + batchSize
			if end > len(records) {
				end = len(records)
			}
			batch := records[i:end]

			// Try batch upsert
			err := remoteDB.Table(table).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				UpdateAll: true,
			}).Create(&batch).Error

			if err != nil {
				// Batch failed, try individual
				log.Printf("   ⚠️  Batch failed, retrying individually...")
				for _, record := range batch {
					// Individual upsert
					id := record["id"]
					if id == nil || id == "" {
						failed++
						continue
					}

					updateResult := remoteDB.Table(table).Where("id = ?", id).Updates(record)
					if updateResult.RowsAffected == 0 {
						// Doesn't exist, create
						if err := remoteDB.Table(table).Create(record).Error; err != nil {
							log.Printf("   ❌ Failed to insert %s (id=%v): %v", table, id, err)
							failed++
							continue
						}
					}
					uploaded++
				}
			} else {
				uploaded += len(batch)
			}
		}

		if uploaded > 0 {
			log.Printf("   ✅ %s: %d succeeded, %d failed", table, uploaded, failed)
			totalUploaded += uploaded
		}
	}

	log.Println()
	log.Printf("✅ Upload complete: %d total records synced", totalUploaded)
	log.Println()

	// Verify counts
	remoteDB.Table("customers").Count(&remoteCustomers)
	remoteDB.Table("customer_contacts").Count(&remoteContacts)
	log.Printf("📊 Supabase AFTER: %d customers, %d contacts", remoteCustomers, remoteContacts)
	log.Println()

	if remoteCustomers == localCustomers && remoteContacts == localContacts {
		log.Println("╔═══════════════════════════════════════════════════════════════════╗")
		log.Println("║       ✅ SUCCESS! Supabase fully synced with local SQLite        ║")
		log.Println("╚═══════════════════════════════════════════════════════════════════╝")
	} else {
		log.Println("╔═══════════════════════════════════════════════════════════════════╗")
		log.Println("║       ⚠️  WARNING: Count mismatch after sync                     ║")
		log.Println("╚═══════════════════════════════════════════════════════════════════╝")
		log.Printf("Local:  %d customers, %d contacts", localCustomers, localContacts)
		log.Printf("Remote: %d customers, %d contacts", remoteCustomers, remoteContacts)
		log.Println()
		log.Println("This may be due to validation failures or foreign key constraints.")
		log.Println("Check the logs above for specific errors.")
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultValue
}
