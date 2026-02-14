package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   Client Activity Check")
	fmt.Println("========================================")

	// Connect to Supabase
	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dsn := fmt.Sprintf("host=aws-1-ap-southeast-1.pooler.supabase.com port=5432 user=postgres.xkyydcxxkzjbnsrmzkpp password=%s dbname=postgres sslmode=require", supabasePassword)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Supabase connection failed: %v", err)
	}
	fmt.Println("✅ Connected to Supabase")
	fmt.Println()

	// Our bulk sync was around 10:40 AM today (Jan 31, 2026)
	// Check for any records created/updated AFTER that
	bulkSyncTime := time.Date(2026, 1, 31, 5, 30, 0, 0, time.UTC) // 10:40 IST = 05:10 UTC, adding buffer

	fmt.Printf("Checking for activity after: %s\n\n", bulkSyncTime.Format("2006-01-02 15:04 UTC"))

	// Check various tables for new records
	tables := []struct {
		name string
		hasCreatedAt bool
	}{
		{"customers", true},
		{"invoices", true},
		{"orders", true},
		{"offers", true},
		{"rfq_data", true},
		{"payments", true},
		{"customer_contacts", true},
	}

	fmt.Println("📊 Records Updated/Created After Bulk Sync:")
	fmt.Println("─────────────────────────────────────────────")

	totalNewActivity := 0
	for _, t := range tables {
		var countAfter int64
		err := db.Table(t.name).Where("updated_at > ?", bulkSyncTime).Count(&countAfter).Error
		if err != nil {
			fmt.Printf("  %-20s: Error - %v\n", t.name, err)
			continue
		}

		if countAfter > 0 {
			fmt.Printf("  %-20s: %d new/updated records ⚠️\n", t.name, countAfter)
			totalNewActivity += int(countAfter)

			// Show details of recent records
			type RecentRecord struct {
				ID        string
				CreatedAt time.Time
				UpdatedAt time.Time
			}
			var recent []RecentRecord
			db.Table(t.name).Select("id, created_at, updated_at").Where("updated_at > ?", bulkSyncTime).Order("updated_at DESC").Limit(3).Find(&recent)
			for _, r := range recent {
				fmt.Printf("      → %s (updated: %s)\n", r.ID[:12]+"...", r.UpdatedAt.Format("15:04:05"))
			}
		} else {
			fmt.Printf("  %-20s: No new activity\n", t.name)
		}
	}

	fmt.Println("\n========================================")
	if totalNewActivity > 0 {
		fmt.Printf("⚠️  DETECTED: %d records updated after bulk sync\n", totalNewActivity)
		fmt.Println("   This indicates client app IS syncing!")
	} else {
		fmt.Println("ℹ️  No new activity detected after bulk sync")
		fmt.Println("   Client may not have synced yet, or hasn't")
		fmt.Println("   created/modified any records.")
	}
	fmt.Println("========================================")

	// Also check the exact latest timestamps
	fmt.Println("\n📅 Latest Record Timestamps (All Tables):")
	fmt.Println("─────────────────────────────────────────────")

	for _, t := range tables {
		var latest struct {
			MaxCreated time.Time
			MaxUpdated time.Time
		}
		db.Table(t.name).Select("MAX(created_at) as max_created, MAX(updated_at) as max_updated").Scan(&latest)
		fmt.Printf("  %-20s: Created: %s | Updated: %s\n",
			t.name,
			latest.MaxCreated.Format("Jan 02 15:04"),
			latest.MaxUpdated.Format("Jan 02 15:04"))
	}
}
