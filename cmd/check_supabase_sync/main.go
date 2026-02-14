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
	fmt.Println("   Supabase Sync Status Check")
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

	// Check record counts
	fmt.Println("📊 Current Record Counts in Supabase:")
	fmt.Println("─────────────────────────────────────")

	tables := []string{
		"customers", "customer_contacts", "suppliers",
		"invoices", "orders", "payments",
		"supplier_invoices", "supplier_payments",
		"purchase_orders", "rfq_data", "offers", "offer_items",
	}

	for _, table := range tables {
		var count int64
		if err := db.Table(table).Count(&count).Error; err != nil {
			fmt.Printf("  %-20s: ERROR - %v\n", table, err)
		} else {
			fmt.Printf("  %-20s: %d\n", table, count)
		}
	}

	// Check for recent updates (if updated_at column exists)
	fmt.Println("\n📅 Recent Activity (last 7 days):")
	fmt.Println("─────────────────────────────────────")

	type RecentUpdate struct {
		Table     string
		LastUpdate time.Time
		Count     int64
	}

	// Check invoices for recent activity
	var invoiceResult struct {
		MaxUpdated time.Time
		Count      int64
	}
	db.Table("invoices").Select("MAX(updated_at) as max_updated, COUNT(*) as count").Scan(&invoiceResult)
	if !invoiceResult.MaxUpdated.IsZero() {
		daysSince := int(time.Since(invoiceResult.MaxUpdated).Hours() / 24)
		fmt.Printf("  invoices:     Last update %d days ago (%s)\n", daysSince, invoiceResult.MaxUpdated.Format("2006-01-02 15:04"))
	}

	// Check customers
	var customerResult struct {
		MaxUpdated time.Time
		Count      int64
	}
	db.Table("customers").Select("MAX(updated_at) as max_updated, COUNT(*) as count").Scan(&customerResult)
	if !customerResult.MaxUpdated.IsZero() {
		daysSince := int(time.Since(customerResult.MaxUpdated).Hours() / 24)
		fmt.Printf("  customers:    Last update %d days ago (%s)\n", daysSince, customerResult.MaxUpdated.Format("2006-01-02 15:04"))
	}

	// Check orders
	var orderResult struct {
		MaxUpdated time.Time
	}
	db.Table("orders").Select("MAX(updated_at) as max_updated").Scan(&orderResult)
	if !orderResult.MaxUpdated.IsZero() {
		daysSince := int(time.Since(orderResult.MaxUpdated).Hours() / 24)
		fmt.Printf("  orders:       Last update %d days ago (%s)\n", daysSince, orderResult.MaxUpdated.Format("2006-01-02 15:04"))
	}

	// Check offers
	var offerResult struct {
		MaxUpdated time.Time
	}
	db.Table("offers").Select("MAX(updated_at) as max_updated").Scan(&offerResult)
	if !offerResult.MaxUpdated.IsZero() {
		daysSince := int(time.Since(offerResult.MaxUpdated).Hours() / 24)
		fmt.Printf("  offers:       Last update %d days ago (%s)\n", daysSince, offerResult.MaxUpdated.Format("2006-01-02 15:04"))
	}

	// Check RFQ data
	var rfqResult struct {
		MaxUpdated time.Time
	}
	db.Table("rfq_data").Select("MAX(updated_at) as max_updated").Scan(&rfqResult)
	if !rfqResult.MaxUpdated.IsZero() {
		daysSince := int(time.Since(rfqResult.MaxUpdated).Hours() / 24)
		fmt.Printf("  rfq_data:     Last update %d days ago (%s)\n", daysSince, rfqResult.MaxUpdated.Format("2006-01-02 15:04"))
	}

	// Check for sync_records table if exists
	fmt.Println("\n🔄 Sync Records (if available):")
	fmt.Println("─────────────────────────────────────")

	var syncCount int64
	if err := db.Table("sync_records").Count(&syncCount).Error; err != nil {
		fmt.Printf("  sync_records table: Not found or error\n")
	} else {
		fmt.Printf("  sync_records: %d records\n", syncCount)

		// Get last 5 sync records
		type SyncRecord struct {
			ID        string
			TableName string
			RecordID  string
			Action    string
			SyncedAt  time.Time
		}
		var recentSyncs []SyncRecord
		db.Table("sync_records").Order("synced_at DESC").Limit(5).Find(&recentSyncs)
		if len(recentSyncs) > 0 {
			fmt.Println("\n  Last 5 sync operations:")
			for _, s := range recentSyncs {
				fmt.Printf("    %s: %s %s at %s\n", s.TableName, s.Action, s.RecordID[:8], s.SyncedAt.Format("2006-01-02 15:04"))
			}
		}
	}

	fmt.Println("\n========================================")
	fmt.Println("   Sync Check Complete")
	fmt.Println("========================================")
}
