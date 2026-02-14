package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   NUCLEAR SYNC: Clear & Fresh Sync")
	fmt.Println("========================================")

	// Load .env
	godotenv.Load()

	// Connect to SQLite
	sqlite, err := sql.Open("sqlite3", "ph_holdings.db")
	if err != nil {
		log.Fatal("SQLite error:", err)
	}
	defer sqlite.Close()
	fmt.Println("✅ SQLite connected")

	// Connect to Supabase
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=require",
		os.Getenv("SUPABASE_DB_HOST"),
		os.Getenv("SUPABASE_DB_PORT"),
		os.Getenv("SUPABASE_DB_USER"),
		os.Getenv("SUPABASE_DB_PASSWORD"))

	pg, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Supabase error:", err)
	}
	defer pg.Close()
	fmt.Println("✅ Supabase connected")

	// Tables to sync (in order due to foreign keys)
	tables := []string{
		"customer_contacts",
		"supplier_contacts",
		"entity_notes",
		"invoice_items",
		"order_items",
		"offer_items",
		"costing_sheet_items",
		"purchase_order_items",
		"grn_items",
		"payments",
		"supplier_payments",
		"invoices",
		"supplier_invoices",
		"delivery_notes",
		"goods_received_notes",
		"purchase_orders",
		"orders",
		"offers",
		"costing_sheets",
		"rfq_data",
		"products",
		"customers",
		"suppliers",
		"license_keys",
		"devices",
		"device_users",
		"roles",
	}

	// STEP 1: Clear Supabase (reverse order for FK constraints)
	fmt.Println("\n🗑️  CLEARING SUPABASE...")
	for i := len(tables) - 1; i >= 0; i-- {
		tbl := tables[i]
		_, err := pg.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tbl))
		if err != nil {
			fmt.Printf("  ⚠️  %s: %v\n", tbl, err)
		} else {
			fmt.Printf("  ✅ %s cleared\n", tbl)
		}
	}

	// STEP 2: Sync each table
	fmt.Println("\n📤 SYNCING DATA...")

	syncResults := make(map[string][2]int) // [success, fail]

	for _, tbl := range tables {
		success, fail := syncTable(sqlite, pg, tbl)
		syncResults[tbl] = [2]int{success, fail}
		if success > 0 || fail > 0 {
			status := "✅"
			if fail > 0 {
				status = "⚠️"
			}
			fmt.Printf("  %s %s: %d synced, %d failed\n", status, tbl, success, fail)
		}
	}

	// STEP 3: Summary
	fmt.Println("\n========================================")
	fmt.Println("   SYNC COMPLETE!")
	fmt.Println("========================================")

	totalSuccess, totalFail := 0, 0
	for tbl, counts := range syncResults {
		if counts[0] > 0 || counts[1] > 0 {
			totalSuccess += counts[0]
			totalFail += counts[1]
			fmt.Printf("  %s: %d/%d\n", tbl, counts[0], counts[0]+counts[1])
		}
	}
	fmt.Printf("\n  TOTAL: %d synced, %d failed\n", totalSuccess, totalFail)

	if totalFail == 0 {
		fmt.Println("\n🎉 PERFECT SYNC - ALL RECORDS TRANSFERRED!")
	} else {
		fmt.Printf("\n⚠️  %d records failed - check logs above\n", totalFail)
	}
}

func syncTable(sqlite, pg *sql.DB, table string) (int, int) {
	// Get columns from SQLite
	rows, err := sqlite.Query(fmt.Sprintf("SELECT * FROM %s WHERE deleted_at IS NULL LIMIT 1", table))
	if err != nil {
		return 0, 0
	}
	cols, _ := rows.Columns()
	rows.Close()

	if len(cols) == 0 {
		return 0, 0
	}

	// Get all data
	rows, err = sqlite.Query(fmt.Sprintf("SELECT * FROM %s WHERE deleted_at IS NULL", table))
	if err != nil {
		return 0, 0
	}
	defer rows.Close()

	success, fail := 0, 0

	for rows.Next() {
		// Scan row
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)

		// Build INSERT
		placeholders := ""
		colNames := ""
		insertVals := make([]interface{}, 0)

		for i, col := range cols {
			if i > 0 {
				placeholders += ", "
				colNames += ", "
			}
			placeholders += fmt.Sprintf("$%d", i+1)
			colNames += col
			insertVals = append(insertVals, values[i])
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING", table, colNames, placeholders)
		_, err := pg.Exec(query, insertVals...)
		if err != nil {
			fail++
		} else {
			success++
		}
	}

	return success, fail
}
