package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════════╗")
	fmt.Println("║         NUCLEAR RESET (GORM): Complete Supabase Recreation     ║")
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")

	godotenv.Load()

	// Connect to SQLite
	local, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("SQLite error:", err)
	}
	fmt.Println("✅ SQLite connected")

	// Connect to Supabase with connection pool settings
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=require pool_max_conns=5",
		os.Getenv("SUPABASE_DB_HOST"),
		os.Getenv("SUPABASE_DB_PORT"),
		os.Getenv("SUPABASE_DB_USER"),
		os.Getenv("SUPABASE_DB_PASSWORD"))

	cloud, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Supabase error:", err)
	}
	fmt.Println("✅ Supabase connected")

	// STEP 1: Get SQLite tables
	fmt.Println("\n📋 STEP 1: Getting SQLite tables...")
	var tables []string
	local.Raw(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`).Scan(&tables)
	fmt.Printf("   Found %d tables\n", len(tables))

	// STEP 2: Drop all Supabase tables
	fmt.Println("\n🗑️  STEP 2: Dropping ALL Supabase tables...")
	var pgTables []string
	cloud.Raw(`SELECT tablename FROM pg_tables WHERE schemaname = 'public'`).Scan(&pgTables)
	fmt.Printf("   Found %d tables to drop\n", len(pgTables))

	for _, t := range pgTables {
		if err := cloud.Exec(fmt.Sprintf("DROP TABLE IF EXISTS \"%s\" CASCADE", t)).Error; err != nil {
			fmt.Printf("   ⚠️  %s: %v\n", t, err)
		} else {
			fmt.Printf("   ✅ %s\n", t)
		}
	}

	// STEP 3: Create tables (using simple schema)
	fmt.Println("\n📝 STEP 3: Creating tables...")
	for _, table := range tables {
		createTableInPostgres(local, cloud, table)
	}

	// STEP 4: Sync data
	fmt.Println("\n📤 STEP 4: Syncing data...")
	totalSuccess, totalFail := 0, 0

	for _, table := range tables {
		s, f := syncData(local, cloud, table)
		totalSuccess += s
		totalFail += f
		if s > 0 || f > 0 {
			status := "✅"
			if f > 0 && s == 0 {
				status = "❌"
			} else if f > 0 {
				status = "⚠️"
			}
			fmt.Printf("   %s %s: %d/%d\n", status, table, s, s+f)
		}
	}

	// STEP 5: Summary
	fmt.Println("\n╔════════════════════════════════════════════════════════════════╗")
	fmt.Printf("║  COMPLETE: %d synced, %d failed                               \n", totalSuccess, totalFail)
	fmt.Println("╚════════════════════════════════════════════════════════════════╝")
}

func createTableInPostgres(local, cloud *gorm.DB, table string) {
	// Get column info from SQLite
	type ColInfo struct {
		CID        int
		Name       string
		Type       string
		NotNull    int
		DefaultVal interface{}
		PK         int
	}

	var cols []ColInfo
	local.Raw(fmt.Sprintf("PRAGMA table_info(%s)", table)).Scan(&cols)

	if len(cols) == 0 {
		fmt.Printf("   ⚠️  %s: no columns\n", table)
		return
	}

	// Build CREATE TABLE
	var colDefs []string
	for _, col := range cols {
		pgType := sqliteToPostgresType(col.Type)
		def := fmt.Sprintf("\"%s\" %s", col.Name, pgType)
		if col.PK == 1 {
			def += " PRIMARY KEY"
		}
		colDefs = append(colDefs, def)
	}

	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS \"%s\" (%s)", table, strings.Join(colDefs, ", "))

	if err := cloud.Exec(sql).Error; err != nil {
		fmt.Printf("   ❌ %s: %v\n", table, err)
	} else {
		fmt.Printf("   ✅ %s (%d columns)\n", table, len(cols))
	}
}

func sqliteToPostgresType(sqliteType string) string {
	t := strings.ToUpper(sqliteType)
	switch {
	case strings.Contains(t, "INT"):
		return "BIGINT"
	case strings.Contains(t, "CHAR"), strings.Contains(t, "TEXT"), strings.Contains(t, "CLOB"):
		return "TEXT"
	case strings.Contains(t, "BLOB"):
		return "BYTEA"
	case strings.Contains(t, "REAL"), strings.Contains(t, "FLOA"), strings.Contains(t, "DOUB"):
		return "DOUBLE PRECISION"
	case strings.Contains(t, "NUMERIC"), strings.Contains(t, "DECIMAL"):
		return "NUMERIC"
	case strings.Contains(t, "BOOL"):
		return "BOOLEAN"
	case strings.Contains(t, "DATE"), strings.Contains(t, "TIME"):
		return "TIMESTAMP"
	default:
		return "TEXT"
	}
}

func syncData(local, cloud *gorm.DB, table string) (int, int) {
	// Get all records from local
	var records []map[string]interface{}
	if err := local.Table(table).Where("deleted_at IS NULL").Find(&records).Error; err != nil {
		// Try without deleted_at
		if err := local.Table(table).Find(&records).Error; err != nil {
			return 0, 0
		}
	}

	if len(records) == 0 {
		return 0, 0
	}

	success, fail := 0, 0
	for _, record := range records {
		if err := cloud.Table(table).Create(record).Error; err != nil {
			fail++
		} else {
			success++
		}
	}

	return success, fail
}
