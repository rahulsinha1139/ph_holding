package main

import (
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

// Tables to sync in dependency order
var tablesToSync = []string{
	// Master data first
	"roles",
	"customers",
	"suppliers",
	"products",
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
	"purchase_orders",
	"purchase_order_items",
	"supplier_invoices",
	"supplier_invoice_items",
	"supplier_payments",
	"delivery_notes",
	"delivery_note_items",
	"goods_received_notes",
	"grn_items",
	"license_keys",
	"assets",
	"settings",
}

func main() {
	log.Println("=== Full Push: SQLite → Supabase PostgreSQL ===")
	log.Println("SQLite is MASTER - PostgreSQL will be updated to match")
	log.Println("")

	// SQLite connection
	sqliteDB, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to open SQLite: %v", err)
	}
	log.Println("✅ Connected to SQLite")

	// PostgreSQL connection
	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dsn := "host=aws-1-ap-southeast-1.pooler.supabase.com port=5432 user=postgres.xkyydcxxkzjbnsrmzkpp password=" + supabasePassword + " dbname=postgres sslmode=require"
	pgDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to open PostgreSQL: %v", err)
	}
	log.Println("✅ Connected to Supabase PostgreSQL")
	log.Println("")

	totalSynced := 0

	for _, table := range tablesToSync {
		synced := syncTable(sqliteDB, pgDB, table)
		totalSynced += synced
	}

	log.Println("")
	log.Printf("=== Complete: %d total records synced ===", totalSynced)
}

func syncTable(sqliteDB, pgDB *gorm.DB, table string) int {
	// Get all records from SQLite
	var records []map[string]interface{}
	if err := sqliteDB.Table(table).Find(&records).Error; err != nil {
		log.Printf("⚠️  %s: Failed to read from SQLite: %v", table, err)
		return 0
	}

	if len(records) == 0 {
		log.Printf("⏭️  %s: No records in SQLite", table)
		return 0
	}

	// Clear PostgreSQL table and insert fresh data
	if err := pgDB.Exec("DELETE FROM " + table).Error; err != nil {
		log.Printf("⚠️  %s: Failed to clear PostgreSQL: %v", table, err)
		// Try upsert instead
		return upsertRecords(pgDB, table, records)
	}

	// Batch insert
	synced := 0
	batchSize := 100
	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]

		if err := pgDB.Table(table).Create(&batch).Error; err != nil {
			// Try one by one on batch failure
			for _, record := range batch {
				if err := pgDB.Table(table).Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "id"}},
					UpdateAll: true,
				}).Create(&record).Error; err == nil {
					synced++
				}
			}
		} else {
			synced += len(batch)
		}
	}

	log.Printf("✅ %s: %d records synced", table, synced)
	return synced
}

func upsertRecords(pgDB *gorm.DB, table string, records []map[string]interface{}) int {
	synced := 0
	for _, record := range records {
		if err := pgDB.Table(table).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).Create(&record).Error; err == nil {
			synced++
		}
	}
	log.Printf("✅ %s: %d records upserted", table, synced)
	return synced
}

func init() {
	log.SetFlags(log.Ltime)
	log.SetOutput(os.Stdout)
}
