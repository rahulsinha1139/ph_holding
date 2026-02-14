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
	// Render.com DATABASE_URL
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgresql://ph_admin:Indcy4LP1o5uDJT2oFQ4KzyXKcFllSKu@dpg-d5rmv2hr0fns73e8sq1g-a.singapore-postgres.render.com/ph_holdings"
	}

	fmt.Println("🔌 Testing Render.com PostgreSQL connection...")
	fmt.Printf("   URL: %s...%s\n", databaseURL[:30], databaseURL[len(databaseURL)-20:])

	start := time.Now()

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("❌ Connection failed: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("❌ Failed to get sql.DB: %v", err)
	}
	defer sqlDB.Close()

	// Ping test
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("❌ Ping failed: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("✅ Connected successfully! (latency: %v)\n", elapsed)

	// Check PostgreSQL version
	var version string
	db.Raw("SELECT version()").Scan(&version)
	fmt.Printf("📊 PostgreSQL: %s\n", version[:50]+"...")

	// List existing tables
	var tables []string
	db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public'").Scan(&tables)
	fmt.Printf("📋 Existing tables: %d\n", len(tables))
	for _, t := range tables {
		fmt.Printf("   - %s\n", t)
	}

	fmt.Println("\n🎉 Render.com database is ready!")
}
