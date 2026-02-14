package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   RAW SYNC: Plain INSERT (no upsert)")
	fmt.Println("========================================")

	godotenv.Load()

	// Connect to SQLite
	local, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("SQLite error:", err)
	}
	fmt.Println("✅ SQLite connected")

	// Connect to Supabase
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=require",
		os.Getenv("SUPABASE_DB_HOST"),
		os.Getenv("SUPABASE_DB_PORT"),
		os.Getenv("SUPABASE_DB_USER"),
		os.Getenv("SUPABASE_DB_PASSWORD"))

	cloud, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Supabase error:", err)
	}
	fmt.Println("✅ Supabase connected")

	// Tables to sync
	tables := []string{
		"customers", "suppliers", "supplier_invoices",
		"supplier_payments", "purchase_orders",
	}

	for _, table := range tables {
		fmt.Printf("\n📤 Syncing %s...\n", table)

		// Clear
		cloud.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))

		// Get records
		var records []map[string]interface{}
		local.Table(table).Where("deleted_at IS NULL").Find(&records)
		fmt.Printf("  Found %d records\n", len(records))

		if len(records) == 0 {
			continue
		}

		// Insert one by one with plain Create (no OnConflict)
		success, fail := 0, 0
		for _, record := range records {
			// Use raw Create without OnConflict
			if err := cloud.Table(table).Create(record).Error; err != nil {
				fail++
				if fail <= 2 {
					fmt.Printf("  ❌ %v\n", err)
				}
			} else {
				success++
			}
		}
		fmt.Printf("  ✅ %d success, %d failed\n", success, fail)

		// Verify
		var count int64
		cloud.Table(table).Count(&count)
		fmt.Printf("  📊 Cloud: %d records\n", count)
	}

	fmt.Println("\n========================================")
	fmt.Println("   SYNC COMPLETE")
	fmt.Println("========================================")
}
