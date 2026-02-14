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

	fmt.Println("=== DASHBOARD REVENUE VERIFICATION ===")

	// Match the exact query from GetDashboardStats
	var revenue float64
	db.Raw("SELECT COALESCE(SUM(grand_total_bhd), 0) FROM invoices WHERE status = 'Paid'").Scan(&revenue)
	fmt.Printf("Total Revenue (Paid): %.2f BHD\n", revenue)

	// Count by status
	fmt.Println("\n=== INVOICE COUNTS BY STATUS ===")
	type StatusCount struct {
		Status string
		Count  int64
		Total  float64
	}
	var stats []StatusCount
	db.Raw(`SELECT status, COUNT(*) as count, COALESCE(SUM(grand_total_bhd), 0) as total
		FROM invoices GROUP BY status ORDER BY count DESC`).Scan(&stats)
	for _, s := range stats {
		fmt.Printf("  %s: %d invoices (%.2f BHD)\n", s.Status, s.Count, s.Total)
	}

	// Active customers
	var customerCount int64
	db.Raw("SELECT COUNT(*) FROM customers").Scan(&customerCount)
	fmt.Printf("\nCustomers: %d\n", customerCount)

	// Active orders
	var orderCount int64
	db.Raw("SELECT COUNT(*) FROM orders WHERE status NOT IN ('Delivered', 'Cancelled')").Scan(&orderCount)
	fmt.Printf("Active Orders: %d\n", orderCount)

	// Outstanding AR
	var outstandingAR float64
	db.Raw("SELECT COALESCE(SUM(outstanding_bhd), 0) FROM invoices WHERE status != 'Paid'").Scan(&outstandingAR)
	fmt.Printf("Outstanding AR: %.2f BHD\n", outstandingAR)
}
