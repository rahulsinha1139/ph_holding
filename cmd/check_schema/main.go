package main

import (
	"fmt"
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=dpg-d5rmv2hr0fns73e8sq1g-a.singapore-postgres.render.com port=5432 user=ph_admin password=Indcy4LP1o5uDJT2oFQ4KzyXKcFllSKu dbname=ph_holdings sslmode=require"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Get all tables
	var tables []string
	db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name`).Scan(&tables)

	fmt.Println("=== DATABASE SCHEMA ===")
	fmt.Printf("Total tables: %d\n\n", len(tables))

	for _, table := range tables {
		var count int64
		db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", table)).Scan(&count)
		fmt.Printf("TABLE: %s (%d rows)\n", table, count)

		// Get columns
		type ColInfo struct {
			ColumnName string
			DataType   string
			IsNullable string
		}
		var cols []ColInfo
		db.Raw(`SELECT column_name, data_type, is_nullable 
			FROM information_schema.columns 
			WHERE table_name = ? 
			ORDER BY ordinal_position`, table).Scan(&cols)

		for _, c := range cols {
			nullable := ""
			if c.IsNullable == "NO" {
				nullable = " NOT NULL"
			}
			fmt.Printf("  - %s (%s%s)\n", c.ColumnName, c.DataType, nullable)
		}
		fmt.Println()
	}
}
