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

func main() {
	fmt.Println("========================================")
	fmt.Println("   DEBUG SYNC: With error logging")
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

	// Connect to Supabase with verbose logging
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=require",
		os.Getenv("SUPABASE_DB_HOST"),
		os.Getenv("SUPABASE_DB_PORT"),
		os.Getenv("SUPABASE_DB_USER"),
		os.Getenv("SUPABASE_DB_PASSWORD"))

	cloud, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("Supabase error:", err)
	}
	fmt.Println("✅ Supabase connected")

	// Clear and sync one table at a time
	tables := []string{"customers", "suppliers", "supplier_invoices", "supplier_payments", "purchase_orders"}

	for _, table := range tables {
		fmt.Printf("\n📤 Syncing %s...\n", table)

		// Clear
		if err := cloud.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error; err != nil {
			fmt.Printf("  ⚠️  Truncate error: %v\n", err)
		}

		// Get records from local
		var records []map[string]interface{}
		if err := local.Table(table).Where("deleted_at IS NULL").Find(&records).Error; err != nil {
			fmt.Printf("  ❌ Local read error: %v\n", err)
			continue
		}
		fmt.Printf("  Found %d local records\n", len(records))

		if len(records) == 0 {
			continue
		}

		// Push to cloud with upsert
		success, fail := 0, 0
		for _, record := range records {
			err := cloud.Table(table).Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				UpdateAll: true,
			}).Create(&record).Error

			if err != nil {
				fail++
				if fail <= 3 {
					fmt.Printf("  ❌ Error: %v\n", err)
				}
			} else {
				success++
			}
		}
		fmt.Printf("  ✅ %s: %d success, %d failed\n", table, success, fail)

		// Verify
		var count int64
		cloud.Table(table).Count(&count)
		fmt.Printf("  📊 Cloud count: %d\n", count)
	}

	fmt.Println("\n========================================")
	fmt.Println("   DEBUG SYNC COMPLETE")
	fmt.Println("========================================")
}
