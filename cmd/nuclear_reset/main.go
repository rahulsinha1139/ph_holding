package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║         NUCLEAR RESET: Complete Supabase Recreation            ║")
	fmt.Println("║         Drop all tables, recreate from SQLite schema           ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")

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

	// Test connection
	if err := pg.Ping(); err != nil {
		log.Fatal("Supabase ping failed:", err)
	}
	fmt.Println("✅ Supabase connected")

	// STEP 1: Get all tables from SQLite
	fmt.Println("\n📋 STEP 1: Getting SQLite tables...")
	rows, err := sqlite.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		log.Fatal(err)
	}

	var tables []string
	for rows.Next() {
		var name string
		rows.Scan(&name)
		tables = append(tables, name)
	}
	rows.Close()
	fmt.Printf("   Found %d tables in SQLite\n", len(tables))

	// STEP 2: Drop all existing tables in Supabase
	fmt.Println("\n🗑️  STEP 2: Dropping ALL Supabase tables...")

	// Get all Supabase tables
	pgRows, err := pg.Query(`SELECT tablename FROM pg_tables WHERE schemaname = 'public'`)
	if err != nil {
		log.Printf("   Warning: %v\n", err)
	} else {
		var pgTables []string
		for pgRows.Next() {
			var name string
			pgRows.Scan(&name)
			pgTables = append(pgTables, name)
		}
		pgRows.Close()

		fmt.Printf("   Found %d tables in Supabase to drop\n", len(pgTables))

		// Drop each table with CASCADE
		for _, t := range pgTables {
			_, err := pg.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", t))
			if err != nil {
				fmt.Printf("   ⚠️  %s: %v\n", t, err)
			} else {
				fmt.Printf("   ✅ Dropped %s\n", t)
			}
		}
	}

	// STEP 3: Create tables in Supabase from SQLite schema
	fmt.Println("\n📝 STEP 3: Creating tables in Supabase...")

	for _, table := range tables {
		// Get CREATE TABLE statement from SQLite
		var createSQL string
		err := sqlite.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&createSQL)
		if err != nil || createSQL == "" {
			fmt.Printf("   ⚠️  %s: no schema found\n", table)
			continue
		}

		// Convert SQLite schema to PostgreSQL
		pgSQL := convertSQLiteToPostgres(createSQL, table)

		// Create table in Supabase
		_, err = pg.Exec(pgSQL)
		if err != nil {
			fmt.Printf("   ❌ %s: %v\n", table, err)
			// Try simpler version
			simpleSQL := createSimpleTable(table, sqlite)
			if simpleSQL != "" {
				_, err = pg.Exec(simpleSQL)
				if err != nil {
					fmt.Printf("      Retry failed: %v\n", err)
				} else {
					fmt.Printf("   ✅ %s (simplified)\n", table)
				}
			}
		} else {
			fmt.Printf("   ✅ %s\n", table)
		}
	}

	// STEP 4: Sync data
	fmt.Println("\n📤 STEP 4: Syncing data...")

	totalSuccess := 0
	totalFail := 0

	for _, table := range tables {
		success, fail := syncTableData(sqlite, pg, table)
		totalSuccess += success
		totalFail += fail
		if success > 0 || fail > 0 {
			status := "✅"
			if fail > 0 && success == 0 {
				status = "❌"
			} else if fail > 0 {
				status = "⚠️"
			}
			fmt.Printf("   %s %s: %d synced, %d failed\n", status, table, success, fail)
		}
	}

	// STEP 5: Verify
	fmt.Println("\n🔍 STEP 5: Verification...")
	for _, table := range tables {
		var localCount, cloudCount int
		sqlite.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE deleted_at IS NULL", table)).Scan(&localCount)
		pg.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&cloudCount)

		if localCount > 0 || cloudCount > 0 {
			match := "✅"
			if localCount != cloudCount {
				match = "⚠️"
			}
			fmt.Printf("   %s %s: Local=%d, Cloud=%d\n", match, table, localCount, cloudCount)
		}
	}

	fmt.Println("\n╔════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║  COMPLETE: %d synced, %d failed                               \n", totalSuccess, totalFail)
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
}

func convertSQLiteToPostgres(sqliteSQL, table string) string {
	sql := sqliteSQL

	// Basic conversions
	sql = strings.ReplaceAll(sql, "INTEGER PRIMARY KEY AUTOINCREMENT", "SERIAL PRIMARY KEY")
	sql = strings.ReplaceAll(sql, "AUTOINCREMENT", "")
	sql = strings.ReplaceAll(sql, "INTEGER", "BIGINT")
	sql = strings.ReplaceAll(sql, "REAL", "DOUBLE PRECISION")
	sql = strings.ReplaceAll(sql, "BLOB", "BYTEA")
	sql = strings.ReplaceAll(sql, "datetime", "TIMESTAMP")
	sql = strings.ReplaceAll(sql, "DATETIME", "TIMESTAMP")
	sql = strings.ReplaceAll(sql, "\"", "")

	// Remove SQLite-specific stuff
	sql = strings.ReplaceAll(sql, "IF NOT EXISTS ", "")

	// Handle NUMERIC with precision - Postgres doesn't like NUMERIC in some contexts
	// Just use NUMERIC without precision for simplicity

	return sql
}

func createSimpleTable(table string, sqlite *sql.DB) string {
	// Get column info
	rows, err := sqlite.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return ""
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var defaultVal interface{}
		rows.Scan(&cid, &name, &colType, &notNull, &defaultVal, &pk)

		pgType := "TEXT"
		switch strings.ToUpper(colType) {
		case "INTEGER", "INT", "BIGINT":
			pgType = "BIGINT"
		case "REAL", "FLOAT", "DOUBLE":
			pgType = "DOUBLE PRECISION"
		case "BOOLEAN", "BOOL":
			pgType = "BOOLEAN"
		case "DATETIME", "TIMESTAMP":
			pgType = "TIMESTAMP"
		case "NUMERIC":
			pgType = "NUMERIC"
		default:
			pgType = "TEXT"
		}

		col := fmt.Sprintf("%s %s", name, pgType)
		if pk == 1 {
			col += " PRIMARY KEY"
		}
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		return ""
	}

	return fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (%s)", table, strings.Join(columns, ", "))
}

func syncTableData(sqlite, pg *sql.DB, table string) (int, int) {
	// Get all rows from SQLite
	rows, err := sqlite.Query(fmt.Sprintf("SELECT * FROM %s WHERE deleted_at IS NULL", table))
	if err != nil {
		// Try without deleted_at filter
		rows, err = sqlite.Query(fmt.Sprintf("SELECT * FROM %s", table))
		if err != nil {
			return 0, 0
		}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil || len(cols) == 0 {
		return 0, 0
	}

	success, fail := 0, 0

	for rows.Next() {
		// Scan row
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			fail++
			continue
		}

		// Build INSERT
		placeholders := make([]string, len(cols))
		for i := range cols {
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}

		query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) ON CONFLICT DO NOTHING",
			table,
			strings.Join(cols, ", "),
			strings.Join(placeholders, ", "))

		_, err := pg.Exec(query, values...)
		if err != nil {
			fail++
		} else {
			success++
		}
	}

	return success, fail
}
