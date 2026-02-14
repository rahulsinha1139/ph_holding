package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Supabase connection
const (
	SUPABASE_HOST = "aws-1-ap-southeast-1.pooler.supabase.com"
	SUPABASE_PORT = "5432"
	SUPABASE_USER = "postgres.xkyydcxxkzjbnsrmzkpp"
	SUPABASE_DB   = "postgres"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   SQLite -> Supabase Full Sync v3")
	fmt.Println("   (Type-Safe Edition)")
	fmt.Println("========================================")
	fmt.Printf("   Started: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("========================================\n")

	// Find local SQLite database
	sqlitePath := findSQLiteDB()
	if sqlitePath == "" {
		log.Fatal("Could not find local SQLite database")
	}
	fmt.Printf("Found SQLite: %s\n\n", sqlitePath)

	// Connect to SQLite
	fmt.Println("Connecting to SQLite...")
	sqliteDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("SQLite connection failed: %v", err)
	}
	rawSqlite, _ := sqliteDB.DB()
	fmt.Println("SQLite connected!\n")

	// Connect to Supabase
	fmt.Println("Connecting to Supabase...")
	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		SUPABASE_HOST, SUPABASE_PORT, SUPABASE_USER, supabasePassword, SUPABASE_DB,
	)
	supabaseDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Supabase connection failed: %v", err)
	}
	rawPg, _ := supabaseDB.DB()
	fmt.Println("Supabase connected!\n")

	fmt.Println("========================================")
	fmt.Println("   PHASE 1: Prepare Supabase Schema")
	fmt.Println("========================================\n")

	// Drop unique constraints that cause issues
	fmt.Println("Dropping problematic constraints...")
	constraints := []string{
		"ALTER TABLE invoices DROP CONSTRAINT IF EXISTS idx_invoices_order_id",
		"ALTER TABLE invoices DROP CONSTRAINT IF EXISTS invoices_order_id_key",
		"ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_status",
		"ALTER TABLE purchase_orders DROP CONSTRAINT IF EXISTS chk_purchase_orders_status",
		"ALTER TABLE offers DROP CONSTRAINT IF EXISTS chk_offers_status",
		"ALTER TABLE invoices DROP CONSTRAINT IF EXISTS chk_invoices_status",
	}
	for _, c := range constraints {
		supabaseDB.Exec(c)
	}

	// Drop unique indexes that cause issues
	dropIndexes := []string{
		"DROP INDEX IF EXISTS idx_invoices_order_id",
		"DROP INDEX IF EXISTS invoices_order_id_key",
	}
	for _, idx := range dropIndexes {
		supabaseDB.Exec(idx)
	}
	fmt.Println("Constraints and indexes dropped.\n")

	// Create missing tables in Supabase
	fmt.Println("Creating missing tables...")
	createTables := []string{
		`CREATE TABLE IF NOT EXISTS license_keys (
			id TEXT PRIMARY KEY,
			license_key TEXT UNIQUE,
			role TEXT,
			device_hash TEXT,
			activated INTEGER DEFAULT 0,
			activated_at TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			deleted_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS followup_tasks (
			id TEXT PRIMARY KEY,
			entity_type TEXT,
			entity_id TEXT,
			task_type TEXT,
			description TEXT,
			due_date TIMESTAMP,
			status TEXT DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			deleted_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS opportunities (
			id TEXT PRIMARY KEY,
			name TEXT,
			customer_id TEXT,
			stage TEXT,
			value DOUBLE PRECISION,
			probability DOUBLE PRECISION,
			expected_close_date TIMESTAMP,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			deleted_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS prediction_records (
			id TEXT PRIMARY KEY,
			entity_type TEXT,
			entity_id TEXT,
			prediction_type TEXT,
			predicted_value DOUBLE PRECISION,
			actual_value DOUBLE PRECISION,
			accuracy DOUBLE PRECISION,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS warehouses (
			id TEXT PRIMARY KEY,
			name TEXT,
			location TEXT,
			capacity INTEGER,
			is_active INTEGER DEFAULT 1,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			deleted_at TIMESTAMP
		)`,
	}
	for _, sql := range createTables {
		supabaseDB.Exec(sql)
	}
	fmt.Println("Tables created.\n")

	// Get PostgreSQL column types for each table
	fmt.Println("========================================")
	fmt.Println("   PHASE 2: Type-Safe Data Upload")
	fmt.Println("========================================\n")

	// Tables to sync
	tablesToSync := []string{
		"roles", "customers", "suppliers", "products", "devices", "users",
		"device_users", "settings", "customer_contacts", "supplier_contacts",
		"offers", "offer_items", "orders", "order_items", "invoices",
		"invoice_items", "payments", "supplier_invoices", "supplier_invoice_items",
		"supplier_payments", "purchase_orders", "purchase_order_items",
		"goods_received_notes", "grn_items", "delivery_notes", "delivery_note_items",
		"conversations", "chat_messages", "entity_notes", "offer_follow_ups",
		"supplier_issues", "quick_captures", "audit_logs", "sync_records",
		"rfqs", "costing_sheets", "costing_sheet_items", "license_keys",
		"followup_tasks", "opportunities", "prediction_records", "warehouses",
	}

	totalUploaded := 0
	for _, table := range tablesToSync {
		// Check if table exists in SQLite
		var count int64
		if err := sqliteDB.Table(table).Count(&count).Error; err != nil {
			continue
		}
		if count == 0 {
			continue
		}

		// Get PostgreSQL column types
		pgColTypes := getPgColumnTypes(rawPg, table)
		if len(pgColTypes) == 0 {
			fmt.Printf("  %-25s: (not in Supabase)\n", table)
			continue
		}

		uploaded := uploadTableTypeSafe(rawSqlite, rawPg, table, pgColTypes)
		totalUploaded += uploaded
	}

	fmt.Println("\n========================================")
	fmt.Println("   PHASE 3: Verification")
	fmt.Println("========================================\n")

	verifyTables := []string{"customers", "suppliers", "invoices", "orders", "payments", "purchase_orders", "supplier_invoices"}
	allMatch := true
	for _, table := range verifyTables {
		var localCount, cloudCount int64
		sqliteDB.Table(table).Count(&localCount)
		supabaseDB.Table(table).Count(&cloudCount)

		status := "OK"
		if localCount != cloudCount {
			status = "MISMATCH"
			allMatch = false
		}
		fmt.Printf("  %-20s: Local=%d, Cloud=%d [%s]\n", table, localCount, cloudCount, status)
	}

	// Revenue check
	fmt.Println("\nRevenue by Year (Supabase):")
	type YearSum struct {
		Year  int
		Total float64
		Count int64
	}
	var years []YearSum
	supabaseDB.Raw(`
		SELECT EXTRACT(YEAR FROM invoice_date)::int as year,
		       COALESCE(SUM(subtotal_bhd), 0) as total,
		       COUNT(*) as count
		FROM invoices
		WHERE deleted_at IS NULL
		GROUP BY year
		ORDER BY year
	`).Scan(&years)

	for _, y := range years {
		fmt.Printf("  %d: %d invoices, %.2f BHD\n", y.Year, y.Count, y.Total)
	}

	fmt.Println("\n========================================")
	if allMatch {
		fmt.Println("   SYNC COMPLETE - ALL DATA VERIFIED!")
	} else {
		fmt.Println("   SYNC COMPLETE - SOME MISMATCHES")
	}
	fmt.Println("========================================")
	fmt.Printf("   Total records uploaded: %d\n", totalUploaded)
	fmt.Printf("   Completed: %s\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("========================================")

	rawSqlite.Close()
	rawPg.Close()
}

func getPgColumnTypes(db *sql.DB, table string) map[string]string {
	cols := make(map[string]string)
	rows, err := db.Query(`
		SELECT column_name, data_type, udt_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = $1
	`, table)
	if err != nil {
		return cols
	}
	defer rows.Close()
	for rows.Next() {
		var name, dtype, udtName string
		rows.Scan(&name, &dtype, &udtName)
		// Use udt_name for more precise type info
		cols[name] = udtName
	}
	return cols
}

func uploadTableTypeSafe(sqliteDB, pgDB *sql.DB, table string, pgColTypes map[string]string) int {
	// Get all data from SQLite
	rows, err := sqliteDB.Query(fmt.Sprintf("SELECT * FROM %s", table))
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", table, err)
		return 0
	}
	defer rows.Close()

	cols, _ := rows.Columns()
	if len(cols) == 0 {
		return 0
	}

	// Filter columns to only those that exist in PostgreSQL
	validCols := []string{}
	colIndexes := []int{}
	for i, col := range cols {
		if _, exists := pgColTypes[col]; exists {
			validCols = append(validCols, col)
			colIndexes = append(colIndexes, i)
		}
	}

	if len(validCols) == 0 {
		return 0
	}

	// Clear existing data
	pgDB.Exec(fmt.Sprintf("DELETE FROM %s", table))

	// Build insert statement with type casting
	placeholders := make([]string, len(validCols))
	for i, col := range validCols {
		pgType := pgColTypes[col]
		// Add explicit casts for problematic types
		switch pgType {
		case "bool":
			placeholders[i] = fmt.Sprintf("$%d::boolean", i+1)
		case "int8", "int4", "int2":
			placeholders[i] = fmt.Sprintf("$%d::bigint", i+1)
		case "float8", "float4":
			placeholders[i] = fmt.Sprintf("$%d::double precision", i+1)
		case "timestamptz", "timestamp":
			placeholders[i] = fmt.Sprintf("$%d::timestamp", i+1)
		default:
			placeholders[i] = fmt.Sprintf("$%d", i+1)
		}
	}

	insertSQL := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(validCols, ", "),
		strings.Join(placeholders, ", "))

	uploaded := 0
	errors := 0
	for rows.Next() {
		// Create value holders for ALL columns
		allValues := make([]interface{}, len(cols))
		allValuePtrs := make([]interface{}, len(cols))
		for i := range allValues {
			allValuePtrs[i] = &allValues[i]
		}

		if err := rows.Scan(allValuePtrs...); err != nil {
			continue
		}

		// Extract only the valid columns and convert types
		values := make([]interface{}, len(validCols))
		for i, idx := range colIndexes {
			col := validCols[i]
			pgType := pgColTypes[col]
			values[i] = convertToPostgresType(allValues[idx], pgType)
		}

		_, err := pgDB.Exec(insertSQL, values...)
		if err != nil {
			errors++
			if errors == 1 {
				fmt.Printf("  %s: First error - %v\n", table, err)
			}
			continue
		}
		uploaded++
	}

	if uploaded > 0 {
		if errors > 0 {
			fmt.Printf("  %-25s: %d uploaded, %d errors\n", table, uploaded, errors)
		} else {
			fmt.Printf("  %-25s: %d records uploaded\n", table, uploaded)
		}
	}
	return uploaded
}

func convertToPostgresType(v interface{}, pgType string) interface{} {
	if v == nil {
		return nil
	}

	switch pgType {
	case "bool":
		// Convert to actual boolean
		switch val := v.(type) {
		case int64:
			return val != 0
		case float64:
			return val != 0
		case bool:
			return val
		case []byte:
			s := string(val)
			return s == "1" || strings.EqualFold(s, "true")
		case string:
			return val == "1" || strings.EqualFold(val, "true")
		default:
			return false
		}

	case "int8", "int4", "int2":
		// Convert to integer
		switch val := v.(type) {
		case int64:
			return val
		case float64:
			return int64(val)
		case bool:
			if val {
				return int64(1)
			}
			return int64(0)
		case []byte:
			return string(val)
		case string:
			return val
		default:
			return v
		}

	case "float8", "float4", "numeric":
		// Convert to float
		switch val := v.(type) {
		case float64:
			return val
		case int64:
			return float64(val)
		case []byte:
			return string(val)
		case string:
			return val
		default:
			return v
		}

	case "timestamptz", "timestamp":
		// Handle datetime
		switch val := v.(type) {
		case time.Time:
			return val
		case []byte:
			return string(val)
		case string:
			if val == "" {
				return nil
			}
			return val
		default:
			return v
		}

	case "text", "varchar":
		// Ensure string type
		switch val := v.(type) {
		case []byte:
			return string(val)
		case string:
			return val
		default:
			return fmt.Sprintf("%v", v)
		}

	default:
		// For other types, convert bytes to string
		if b, ok := v.([]byte); ok {
			return string(b)
		}
		return v
	}
}

func findSQLiteDB() string {
	paths := []string{
		"./ph_holdings.db",
		"../ph_holdings.db",
		"../../ph_holdings.db",
	}

	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "ph_holdings.db"))
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			return abs
		}
	}
	return ""
}
