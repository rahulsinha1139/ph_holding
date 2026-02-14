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

	fmt.Println("=== FIXING INVOICE TOTALS ===")

	// Copy total_bhd to grand_total_bhd where grand_total_bhd is 0
	result := db.Exec("UPDATE invoices SET grand_total_bhd = total_bhd WHERE grand_total_bhd = 0 OR grand_total_bhd IS NULL")
	if result.Error != nil {
		log.Fatalf("Error updating: %v", result.Error)
	}
	fmt.Printf("Updated %d invoices\n", result.RowsAffected)

	// Also copy to outstanding_bhd for unpaid invoices
	result2 := db.Exec("UPDATE invoices SET outstanding_bhd = total_bhd WHERE status != 'Paid' AND (outstanding_bhd = 0 OR outstanding_bhd IS NULL)")
	if result2.Error != nil {
		log.Fatalf("Error updating outstanding: %v", result2.Error)
	}
	fmt.Printf("Updated %d outstanding amounts\n", result2.RowsAffected)

	// Verify
	fmt.Println("\n=== VERIFICATION ===")
	var totalBHD, grandTotalBHD float64
	db.Raw("SELECT COALESCE(SUM(total_bhd), 0) FROM invoices").Scan(&totalBHD)
	db.Raw("SELECT COALESCE(SUM(grand_total_bhd), 0) FROM invoices").Scan(&grandTotalBHD)
	fmt.Printf("SUM(total_bhd): %.2f\n", totalBHD)
	fmt.Printf("SUM(grand_total_bhd): %.2f\n", grandTotalBHD)

	fmt.Println("\n✅ Invoice totals fixed!")
}
