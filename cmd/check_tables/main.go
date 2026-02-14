package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "ph_holdings.db")
	if err != nil {
		log.Fatalf("Failed to open: %v", err)
	}
	defer db.Close()

	// List all tables
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;")
	if err != nil {
		log.Fatalf("Failed to query: %v", err)
	}
	defer rows.Close()

	fmt.Println("=== ALL TABLES IN DATABASE ===")
	count := 0
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		count++
		fmt.Printf("  %d. %s\n", count, name)
	}

	fmt.Printf("\nTotal tables: %d\n", count)

	// Check for supplier-related tables
	fmt.Println("\n=== SUPPLIER-RELATED TABLES ===")
	supplierTables := []string{
		"supplier_invoices",
		"supplier_master",
		"suppliers",
		"SupplierInvoice",
		"SupplierMaster",
	}

	for _, table := range supplierTables {
		var exists int
		query := fmt.Sprintf("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='%s';", table)
		db.QueryRow(query).Scan(&exists)
		if exists > 0 {
			// Count records
			var count int
			countQuery := fmt.Sprintf("SELECT COUNT(*) FROM `%s`;", table)
			db.QueryRow(countQuery).Scan(&count)
			fmt.Printf("  ✅ %s: %d records\n", table, count)
		} else {
			fmt.Printf("  ❌ %s: NOT FOUND\n", table)
		}
	}
}
