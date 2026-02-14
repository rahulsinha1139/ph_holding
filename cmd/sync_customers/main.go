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

func main() {
	log.Println("=== Customer Data Sync ===")

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

	// Get all customers from SQLite
	var customers []map[string]interface{}
	if err := sqliteDB.Table("customers").Find(&customers).Error; err != nil {
		log.Fatalf("Failed to read customers from SQLite: %v", err)
	}
	log.Printf("📊 Found %d customers in SQLite", len(customers))

	// Upsert to PostgreSQL
	synced := 0
	for _, customer := range customers {
		// Use ON CONFLICT to upsert
		if err := pgDB.Table("customers").Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			UpdateAll: true,
		}).Create(&customer).Error; err != nil {
			log.Printf("⚠️  Failed to sync customer %v: %v", customer["id"], err)
		} else {
			synced++
		}
	}

	log.Printf("✅ Synced %d/%d customers to PostgreSQL", synced, len(customers))

	// Verify
	var pgCount int64
	pgDB.Table("customers").Count(&pgCount)
	log.Printf("📊 PostgreSQL customers now: %d", pgCount)

	log.Println("\n=== Customer Sync Complete ===")
}

func init() {
	log.SetFlags(log.Ltime)
	log.SetOutput(os.Stdout)
}
