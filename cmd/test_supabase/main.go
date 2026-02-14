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
	// Session Pooler connection (IPv4 compatible)
	host := "aws-1-ap-southeast-1.pooler.supabase.com"
	port := "5432"
	user := "postgres.jgpwtuyfndtuxxyrytel"
	password := os.Getenv("SUPABASE_DB_PASSWORD")
	if password == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dbname := "postgres"

	fmt.Println("========================================")
	fmt.Println("   SUPABASE CONNECTION TEST")
	fmt.Println("   Using: Session Pooler (IPv4)")
	fmt.Println("========================================")
	fmt.Printf("\nHost: %s\n", host)
	fmt.Printf("Port: %s\n", port)
	fmt.Printf("User: %s\n", user)

	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		host, port, user, password, dbname,
	)

	fmt.Println("\nConnecting...")
	start := time.Now()

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("CONNECTION FAILED: %v", err)
	}

	sqlDB, _ := db.DB()
	if err := sqlDB.Ping(); err != nil {
		log.Fatalf("PING FAILED: %v", err)
	}

	elapsed := time.Since(start)
	fmt.Printf("\nSUCCESS! Connected in %v\n", elapsed)

	// Test query
	var result int
	db.Raw("SELECT 1").Scan(&result)
	fmt.Printf("Test query: SELECT 1 = %d\n", result)

	// Get version
	var version string
	db.Raw("SELECT version()").Scan(&version)
	if len(version) > 50 {
		version = version[:50] + "..."
	}
	fmt.Printf("PostgreSQL: %s\n", version)

	// List tables
	var tables []string
	db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public'").Scan(&tables)
	fmt.Printf("\nExisting tables: %d\n", len(tables))

	fmt.Println("\n========================================")
	fmt.Println("   SUPABASE READY FOR SYNC!")
	fmt.Println("========================================")

	sqlDB.Close()
}
