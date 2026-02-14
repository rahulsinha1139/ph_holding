package main

import (
	"fmt"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=dpg-d5rmv2hr0fns73e8sq1g-a.singapore-postgres.render.com port=5432 user=ph_admin password=Indcy4LP1o5uDJT2oFQ4KzyXKcFllSKu dbname=ph_holdings sslmode=require"
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	tables := []string{"customers", "invoices", "opportunities", "suppliers"}
	
	for _, t := range tables {
		fmt.Printf("\n=== %s columns ===\n", t)
		var cols []string
		db.Raw("SELECT column_name FROM information_schema.columns WHERE table_name = ? ORDER BY ordinal_position", t).Scan(&cols)
		for _, c := range cols {
			fmt.Printf("  %s\n", c)
		}
	}

	// Sample customer
	fmt.Println("\n=== SAMPLE CUSTOMER (raw) ===")
	var result map[string]interface{}
	db.Raw("SELECT * FROM customers LIMIT 1").Scan(&result)
	for k, v := range result {
		fmt.Printf("  %s: %v\n", k, v)
	}
}
