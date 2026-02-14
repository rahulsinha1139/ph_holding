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

// Tables to sync - child tables first (reverse dependency order for delete)
var tablesToClear = []string{
	"grn_items",
	"goods_received_notes",
	"delivery_note_items",
	"delivery_notes",
	"supplier_payments",
	"supplier_invoice_items",
	"supplier_invoices",
	"purchase_order_items",
	"purchase_orders",
	"payments",
	"invoice_items",
	"invoices",
	"order_items",
	"orders",
	"offer_items",
	"offers",
	"supplier_contacts",
	"customer_contacts",
	"products",
	"suppliers",
	"customers",
}

// Tables to insert - parent tables first (dependency order)
var tablesToInsert = []string{
	"customers",
	"suppliers",
	"products",
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
	"roles",
}

func main() {
	log.Println("=== FULL SYNC: SQLite → Supabase ===")
	log.Println("SQLite is MASTER - PostgreSQL will mirror it exactly")
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

	// Disable FK checks
	log.Println("\n📋 Disabling FK constraints...")
	pgDB.Exec("SET session_replication_role = 'replica'")

	// Clear tables in reverse dependency order
	log.Println("\n🗑️  Clearing PostgreSQL tables...")
	for _, table := range tablesToClear {
		if err := pgDB.Exec("DELETE FROM " + table).Error; err != nil {
			log.Printf("   ⚠️  %s: %v", table, err)
		} else {
			log.Printf("   ✅ %s cleared", table)
		}
	}

	// Insert data in dependency order
	log.Println("\n📤 Pushing SQLite data to PostgreSQL...")
	totalSynced := 0
	for _, table := range tablesToInsert {
		synced := pushTable(sqliteDB, pgDB, table)
		totalSynced += synced
	}

	// Re-enable FK checks
	log.Println("\n📋 Re-enabling FK constraints...")
	pgDB.Exec("SET session_replication_role = 'origin'")

	// Verify counts
	log.Println("\n📊 Verification:")
	verifyTables := []string{"customers", "suppliers", "orders", "invoices", "supplier_payments"}
	for _, table := range verifyTables {
		var sqliteCount, pgCount int64
		sqliteDB.Table(table).Count(&sqliteCount)
		pgDB.Table(table).Count(&pgCount)
		status := "✅"
		if sqliteCount != pgCount {
			status = "⚠️"
		}
		log.Printf("   %s %s: SQLite=%d, PostgreSQL=%d", status, table, sqliteCount, pgCount)
	}

	log.Printf("\n=== Complete: %d records synced ===", totalSynced)
}

func pushTable(sqliteDB, pgDB *gorm.DB, table string) int {
	var records []map[string]interface{}
	if err := sqliteDB.Table(table).Find(&records).Error; err != nil {
		return 0
	}

	if len(records) == 0 {
		return 0
	}

	synced := 0
	for _, record := range records {
		if err := pgDB.Table(table).Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoNothing: true,
		}).Create(&record).Error; err == nil {
			synced++
		}
	}

	if synced > 0 {
		log.Printf("   ✅ %s: %d/%d records", table, synced, len(records))
	}
	return synced
}

func init() {
	log.SetFlags(log.Ltime)
	log.SetOutput(os.Stdout)
}
