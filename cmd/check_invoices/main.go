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

	fmt.Println("=== INVOICES TABLE COLUMNS ===")
	type ColInfo struct {
		ColumnName string
		DataType   string
	}
	var cols []ColInfo
	db.Raw(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'invoices' ORDER BY ordinal_position`).Scan(&cols)
	for _, c := range cols {
		fmt.Printf("  %s (%s)\n", c.ColumnName, c.DataType)
	}

	fmt.Println("\n=== SAMPLE INVOICE (all columns) ===")
	var result map[string]interface{}
	db.Raw("SELECT * FROM invoices LIMIT 1").Scan(&result)
	for k, v := range result {
		fmt.Printf("  %s: %v\n", k, v)
	}

	fmt.Println("\n=== INVOICE TOTALS CHECK ===")
	// Check which total columns have data
	var totalBHD, grandTotalBHD, totalAmount float64
	db.Raw("SELECT COALESCE(SUM(total_bhd), 0) FROM invoices").Scan(&totalBHD)
	db.Raw("SELECT COALESCE(SUM(grand_total_bhd), 0) FROM invoices").Scan(&grandTotalBHD)
	db.Raw("SELECT COALESCE(SUM(total_amount), 0) FROM invoices").Scan(&totalAmount)
	fmt.Printf("  SUM(total_bhd): %.2f\n", totalBHD)
	fmt.Printf("  SUM(grand_total_bhd): %.2f\n", grandTotalBHD)
	fmt.Printf("  SUM(total_amount): %.2f\n", totalAmount)
}
