package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	log.Println("=== Quick Customer Sync ===")

	// SQLite
	sqliteDB, err := sql.Open("sqlite3", "ph_holdings.db")
	if err != nil {
		log.Fatal(err)
	}
	defer sqliteDB.Close()

	// PostgreSQL
	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	pgDB, err := sql.Open("postgres", "host=aws-1-ap-southeast-1.pooler.supabase.com port=5432 user=postgres.xkyydcxxkzjbnsrmzkpp password="+supabasePassword+" dbname=postgres sslmode=require")
	if err != nil {
		log.Fatal(err)
	}
	defer pgDB.Close()

	if err := pgDB.Ping(); err != nil {
		log.Fatalf("PostgreSQL ping failed: %v", err)
	}
	log.Println("✅ Connected to both databases")

	// Get SQLite customers
	rows, err := sqliteDB.Query("SELECT * FROM customers")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	columns, _ := rows.Columns()
	log.Printf("Columns: %v", columns)

	// Disable FK checks
	pgDB.Exec("SET session_replication_role = 'replica'")

	// Clear customers
	pgDB.Exec("DELETE FROM customers")
	log.Println("✅ Cleared PostgreSQL customers")

	// Insert customers
	count := 0
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		rows.Scan(valuePtrs...)

		// Build INSERT
		placeholders := make([]string, len(columns))
		args := make([]interface{}, len(columns))
		for i := range columns {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
			// Convert value
			if values[i] == nil {
				args[i] = nil
			} else {
				switch v := values[i].(type) {
				case []byte:
					// Check if it looks like JSON
					s := string(v)
					if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
						args[i] = s
					} else {
						args[i] = s
					}
				default:
					args[i] = v
				}
			}
		}

		sql := fmt.Sprintf("INSERT INTO customers (%s) VALUES (%s) ON CONFLICT (id) DO NOTHING",
			strings.Join(columns, ", "),
			strings.Join(placeholders, ", "))

		if _, err := pgDB.Exec(sql, args...); err != nil {
			// Log first error for debugging
			if count == 0 {
				log.Printf("Sample error: %v", err)
				b, _ := json.Marshal(args)
				log.Printf("Args: %s", string(b))
			}
		} else {
			count++
		}
	}

	// Re-enable FK
	pgDB.Exec("SET session_replication_role = 'origin'")

	// Verify
	var pgCount int
	pgDB.QueryRow("SELECT COUNT(*) FROM customers").Scan(&pgCount)

	log.Printf("✅ Synced %d customers, PostgreSQL now has: %d", count, pgCount)
}

func init() {
	log.SetFlags(log.Ltime)
	log.SetOutput(os.Stdout)
}
