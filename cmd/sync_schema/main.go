package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	log.Println("=== Schema Sync Check (GORM) ===")
	log.Println("")

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
	dsn := fmt.Sprintf("host=aws-1-ap-southeast-1.pooler.supabase.com port=5432 user=postgres.xkyydcxxkzjbnsrmzkpp password=%s dbname=postgres sslmode=require", supabasePassword)
	pgDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to open PostgreSQL: %v", err)
	}
	log.Println("✅ Connected to Supabase PostgreSQL")

	// Get SQLite tables
	sqliteTables := getTables(sqliteDB, "sqlite")
	log.Printf("📊 SQLite tables: %d", len(sqliteTables))

	// Get PostgreSQL tables
	pgTables := getTables(pgDB, "postgres")
	log.Printf("📊 PostgreSQL tables: %d", len(pgTables))

	// Find missing tables in PostgreSQL
	missingInPG := []string{}
	for _, t := range sqliteTables {
		if !contains(pgTables, t) {
			missingInPG = append(missingInPG, t)
		}
	}

	if len(missingInPG) > 0 {
		log.Printf("\n⚠️  Tables missing in PostgreSQL (%d):", len(missingInPG))
		for _, t := range missingInPG {
			log.Printf("   - %s", t)
		}
	} else {
		log.Println("\n✅ All SQLite tables exist in PostgreSQL")
	}

	// Check and add missing columns for common tables
	log.Println("\n=== Column Comparison ===")
	columnsAdded := 0

	for _, table := range sqliteTables {
		if !contains(pgTables, table) {
			continue // Skip tables that don't exist in PG
		}

		sqliteCols := getColumns(sqliteDB, table, "sqlite")
		pgCols := getColumns(pgDB, table, "postgres")

		for _, col := range sqliteCols {
			colName := col["name"]
			if !hasColumn(pgCols, colName) {
				colType := col["type"]
				pgType := sqliteToPgType(colType)

				log.Printf("⚠️  Adding column %s.%s (%s -> %s)", table, colName, colType, pgType)

				sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s %s", table, colName, pgType)
				if err := pgDB.Exec(sql).Error; err != nil {
					log.Printf("   ❌ Failed: %v", err)
				} else {
					log.Printf("   ✅ Added!")
					columnsAdded++
				}
			}
		}
	}

	if columnsAdded == 0 {
		log.Println("✅ All columns are in sync")
	} else {
		log.Printf("✅ Added %d missing columns", columnsAdded)
	}

	// Create missing tables
	if len(missingInPG) > 0 {
		log.Println("\n=== Creating Missing Tables ===")
		for _, table := range missingInPG {
			createTableInPG(sqliteDB, pgDB, table)
		}
	}

	// Show table counts comparison
	log.Println("\n=== Record Counts ===")
	keyTables := []string{
		"customers", "suppliers", "products",
		"orders", "order_items", "invoices", "invoice_items",
		"payments", "supplier_payments",
		"purchase_orders", "delivery_notes",
		"offers", "offer_items",
	}

	for _, table := range keyTables {
		if !contains(sqliteTables, table) {
			continue
		}

		var sqliteCount, pgCount int64
		sqliteDB.Table(table).Count(&sqliteCount)

		if contains(pgTables, table) {
			pgDB.Table(table).Count(&pgCount)
		}

		status := "✅"
		if sqliteCount != pgCount {
			status = "⚠️"
		}
		log.Printf("%s %s: SQLite=%d, PostgreSQL=%d", status, table, sqliteCount, pgCount)
	}

	log.Println("\n=== Schema Sync Complete ===")
}

func getTables(db *gorm.DB, dbType string) []string {
	var tables []string

	if dbType == "sqlite" {
		rows, err := db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name").Rows()
		if err != nil {
			return tables
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			rows.Scan(&name)
			tables = append(tables, name)
		}
	} else {
		rows, err := db.Raw("SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name").Rows()
		if err != nil {
			return tables
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			rows.Scan(&name)
			tables = append(tables, name)
		}
	}

	return tables
}

func getColumns(db *gorm.DB, table, dbType string) []map[string]string {
	var cols []map[string]string

	if dbType == "sqlite" {
		rows, err := db.Raw(fmt.Sprintf("PRAGMA table_info(%s)", table)).Rows()
		if err != nil {
			return cols
		}
		defer rows.Close()

		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue interface{}
			rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk)
			cols = append(cols, map[string]string{"name": name, "type": colType})
		}
	} else {
		rows, err := db.Raw("SELECT column_name, data_type FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1", table).Rows()
		if err != nil {
			return cols
		}
		defer rows.Close()

		for rows.Next() {
			var name, colType string
			rows.Scan(&name, &colType)
			cols = append(cols, map[string]string{"name": name, "type": colType})
		}
	}

	return cols
}

func hasColumn(cols []map[string]string, name string) bool {
	name = strings.ToLower(name)
	for _, col := range cols {
		if strings.ToLower(col["name"]) == name {
			return true
		}
	}
	return false
}

func sqliteToPgType(sqliteType string) string {
	sqliteType = strings.ToUpper(sqliteType)
	switch {
	case strings.Contains(sqliteType, "INT"):
		return "BIGINT"
	case strings.Contains(sqliteType, "REAL") || strings.Contains(sqliteType, "FLOAT") || strings.Contains(sqliteType, "DOUBLE"):
		return "DOUBLE PRECISION"
	case strings.Contains(sqliteType, "DECIMAL"):
		return "DECIMAL(15,3)"
	case strings.Contains(sqliteType, "BOOL"):
		return "BOOLEAN"
	case strings.Contains(sqliteType, "DATETIME") || strings.Contains(sqliteType, "TIMESTAMP"):
		return "TIMESTAMP"
	case strings.Contains(sqliteType, "DATE"):
		return "DATE"
	case strings.Contains(sqliteType, "BLOB"):
		return "BYTEA"
	default:
		return "TEXT"
	}
}

func createTableInPG(sqliteDB, pgDB *gorm.DB, table string) {
	log.Printf("Creating table: %s", table)

	// Get SQLite table schema
	cols := getColumns(sqliteDB, table, "sqlite")
	if len(cols) == 0 {
		log.Printf("   ❌ No columns found")
		return
	}

	// Build CREATE TABLE statement
	var colDefs []string
	for _, col := range cols {
		pgType := sqliteToPgType(col["type"])
		colDefs = append(colDefs, fmt.Sprintf("%s %s", col["name"], pgType))
	}

	// Check for id column (primary key)
	hasID := false
	for _, col := range cols {
		if col["name"] == "id" {
			hasID = true
			break
		}
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n    %s", table, strings.Join(colDefs, ",\n    "))
	if hasID {
		sql += ",\n    PRIMARY KEY (id)"
	}
	sql += "\n)"

	if err := pgDB.Exec(sql).Error; err != nil {
		log.Printf("   ❌ Failed to create: %v", err)
	} else {
		log.Printf("   ✅ Created!")
	}
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func init() {
	log.SetFlags(log.Ltime)
	log.SetOutput(os.Stdout)
}
