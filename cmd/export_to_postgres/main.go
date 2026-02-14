package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// This script exports SQLite data to SQL INSERT statements for PostgreSQL import
// Usage: go run ./cmd/export_to_postgres/main.go > data_export.sql

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	fmt.Println("-- PH Holdings SQLite to PostgreSQL Data Export")
	fmt.Println("-- Generated at:", time.Now().Format(time.RFC3339))
	fmt.Println("-- Run this file against your Render PostgreSQL database")
	fmt.Println()

	// Open SQLite database
	db, err := sql.Open("sqlite3", "ph_holdings.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Tables to export in dependency order
	tables := []string{
		"customers",
		"customer_contacts",
		"suppliers",
		"products",
		"roles",
		"users",
		"warehouses",
		"offers",
		"offer_items",
		"orders",
		"order_items",
		"invoices",
		"payments",
		"opportunities",
		"purchase_orders",
		"purchase_order_items",
		"goods_received_notes",
		"grn_items",
		"supplier_invoices",
		"delivery_notes",
		"delivery_note_items",
		"prediction_records",
		"followup_tasks",
		"settings",
		"conversations",
		"chat_messages",
		"alerts",
	}

	// Export each table
	for _, table := range tables {
		exportTable(db, table)
	}

	fmt.Println()
	fmt.Println("-- Export complete!")
}

func exportTable(db *sql.DB, tableName string) {
	// Check if table exists and has data
	var count int
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", tableName)).Scan(&count)
	if err != nil {
		// Table might not exist
		log.Printf("Skipping %s: %v", tableName, err)
		return
	}

	if count == 0 {
		fmt.Printf("\n-- Table %s: no data\n", tableName)
		return
	}

	fmt.Printf("\n-- ============================================================\n")
	fmt.Printf("-- Table: %s (%d records)\n", tableName, count)
	fmt.Printf("-- ============================================================\n\n")

	// Get columns
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		log.Printf("Error getting columns for %s: %v", tableName, err)
		return
	}

	var columns []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt_value interface{}
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt_value, &pk)
		columns = append(columns, name)
	}
	rows.Close()

	if len(columns) == 0 {
		return
	}

	// Get data
	dataRows, err := db.Query(fmt.Sprintf("SELECT * FROM %s WHERE deleted_at IS NULL", tableName))
	if err != nil {
		log.Printf("Error querying %s: %v", tableName, err)
		return
	}
	defer dataRows.Close()

	// Prepare column list
	colList := strings.Join(columns, ", ")

	// Generate INSERT statements
	for dataRows.Next() {
		// Create slice for scan destination
		dest := make([]interface{}, len(columns))
		destPtrs := make([]interface{}, len(columns))
		for i := range dest {
			destPtrs[i] = &dest[i]
		}

		if err := dataRows.Scan(destPtrs...); err != nil {
			log.Printf("Error scanning row in %s: %v", tableName, err)
			continue
		}

		// Build VALUES clause
		values := make([]string, len(columns))
		for i, v := range dest {
			values[i] = formatValue(v)
		}

		fmt.Printf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (id) DO NOTHING;\n",
			tableName, colList, strings.Join(values, ", "))
	}
}

func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}

	switch val := v.(type) {
	case []byte:
		s := string(val)
		// Check if it's JSON
		if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
			var js interface{}
			if err := json.Unmarshal(val, &js); err == nil {
				return fmt.Sprintf("'%s'", escapeSQL(s))
			}
		}
		return fmt.Sprintf("'%s'", escapeSQL(s))
	case string:
		return fmt.Sprintf("'%s'", escapeSQL(val))
	case int, int64, int32:
		return fmt.Sprintf("%d", val)
	case float64, float32:
		return fmt.Sprintf("%f", val)
	case bool:
		if val {
			return "TRUE"
		}
		return "FALSE"
	case time.Time:
		return fmt.Sprintf("'%s'", val.Format("2006-01-02 15:04:05"))
	default:
		// Try to convert to string
		s := fmt.Sprintf("%v", val)
		return fmt.Sprintf("'%s'", escapeSQL(s))
	}
}

func escapeSQL(s string) string {
	// Escape single quotes for PostgreSQL
	s = strings.ReplaceAll(s, "'", "''")
	// Escape backslashes
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return s
}
