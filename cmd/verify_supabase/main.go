package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   Verifying Supabase Database")
	fmt.Println("========================================")

	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dsn := fmt.Sprintf("host=aws-1-ap-southeast-1.pooler.supabase.com port=5432 user=postgres.xkyydcxxkzjbnsrmzkpp password=%s dbname=postgres sslmode=require", supabasePassword)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Ping failed: %v", err)
	}
	fmt.Println("✅ Supabase connected\n")

	tables := []string{
		"customers",
		"suppliers",
		"orders",
		"invoices",
		"payments",
		"supplier_invoices",
		"supplier_payments",
		"purchase_orders",
		"rfq_data",
		"offers",
		"offer_items",
		"customer_contacts",
	}

	totalRecords := 0
	for _, table := range tables {
		var count int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			fmt.Printf("❌ %-20s ERROR: %v\n", table, err)
			continue
		}
		fmt.Printf("✅ %-20s %d records\n", table, count)
		totalRecords += count
	}

	fmt.Println("\n========================================")
	fmt.Printf("  Total Records in Supabase: %d\n", totalRecords)
	fmt.Println("========================================")
}
