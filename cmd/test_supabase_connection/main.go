package main

import (
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		fmt.Println("❌ SUPABASE_DB_PASSWORD environment variable not set")
		os.Exit(1)
	}
	dsn := fmt.Sprintf("host=aws-1-ap-southeast-1.pooler.supabase.com user=postgres.xkyydcxxkzjbnsrmzkpp password=%s dbname=postgres port=5432 sslmode=require", supabasePassword)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		fmt.Printf("❌ Connection error: %v\n", err)
		os.Exit(1)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	err = sqlDB.Ping()
	if err != nil {
		fmt.Printf("❌ Ping error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Connected to Supabase PostgreSQL")
	fmt.Printf("   Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// Check tables
	var tables []string
	db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name").Scan(&tables)

	fmt.Println("=== Tables in Supabase ===")
	for i, t := range tables {
		fmt.Printf("  %2d. %s\n", i+1, t)
	}
	fmt.Printf("\n📊 Total tables: %d\n", len(tables))

	// Check key data counts
	fmt.Println("\n=== Data Counts ===")

	tableNames := []string{"customers", "suppliers", "invoices", "supplier_invoices", "orders", "payments", "products", "purchase_orders", "offers"}
	for _, table := range tableNames {
		var count int64
		result := db.Table(table).Count(&count)
		if result.Error != nil {
			fmt.Printf("  %-20s: ⚠️  ERROR - %v\n", table, result.Error)
		} else {
			fmt.Printf("  %-20s: %d records\n", table, count)
		}
	}

	// Check invoice totals by year
	fmt.Println("\n=== Invoice Totals by Year (Revenue Verification) ===")
	type YearSummary struct {
		Year     int
		Count    int64
		Subtotal float64
	}
	var yearSummaries []YearSummary
	db.Raw(`
		SELECT EXTRACT(YEAR FROM invoice_date)::int as year,
		       COUNT(*) as count,
		       COALESCE(SUM(subtotal_bhd), 0) as subtotal
		FROM invoices
		WHERE invoice_date IS NOT NULL AND deleted_at IS NULL
		GROUP BY EXTRACT(YEAR FROM invoice_date)
		ORDER BY year
	`).Scan(&yearSummaries)

	for _, ys := range yearSummaries {
		fmt.Printf("  %d: %3d invoices, %12.2f BHD\n", ys.Year, ys.Count, ys.Subtotal)
	}

	// Expected values from FS2024
	fmt.Println("\n=== Expected Values (FS2024 Audited) ===")
	fmt.Println("  2023: 153 invoices, 5,206,212.25 BHD")
	fmt.Println("  2024: 160 invoices, 1,583,336.79 BHD")
	fmt.Println("  2025: 155 invoices,   628,227.11 BHD")

	fmt.Println("\n✅ Supabase database check complete")
}
