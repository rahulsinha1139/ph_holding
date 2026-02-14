package main

import (
	"fmt"
	"log"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	log.Println("=== Checking Database Data ===")

	dsn := "host=dpg-d5rmv2hr0fns73e8sq1g-a.singapore-postgres.render.com port=5432 user=ph_admin password=Indcy4LP1o5uDJT2oFQ4KzyXKcFllSKu dbname=ph_holdings sslmode=require"

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	log.Println("Connected!")

	// Check counts
	tables := []string{"customers", "suppliers", "invoices", "orders", "rfq_data", "roles", "payments"}
	for _, table := range tables {
		var count int64
		db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		log.Printf("  %s: %d rows", table, count)
	}

	// Check invoice years
	log.Println("\n--- Invoice Distribution by Year ---")
	var yearCounts []struct {
		Year  int
		Count int64
	}
	db.Raw(`SELECT EXTRACT(YEAR FROM invoice_date) as year, COUNT(*) as count FROM invoices GROUP BY year ORDER BY year`).Scan(&yearCounts)
	for _, yc := range yearCounts {
		log.Printf("  %d: %d invoices", yc.Year, yc.Count)
	}

	// Check invoice totals
	log.Println("\n--- Invoice Totals ---")
	var totalAmount float64
	db.Raw(`SELECT COALESCE(SUM(total_amount), 0) FROM invoices`).Scan(&totalAmount)
	log.Printf("  Total Amount: %.2f BHD", totalAmount)

	var paidAmount float64
	db.Raw(`SELECT COALESCE(SUM(total_amount), 0) FROM invoices WHERE status = 'Paid'`).Scan(&paidAmount)
	log.Printf("  Paid Amount: %.2f BHD", paidAmount)

	// Check customer links
	log.Println("\n--- Customer Linkage ---")
	var linkedCount, unlinkedCount int64
	db.Raw(`SELECT COUNT(*) FROM invoices WHERE customer_id IS NOT NULL AND customer_id != ''`).Scan(&linkedCount)
	db.Raw(`SELECT COUNT(*) FROM invoices WHERE customer_id IS NULL OR customer_id = ''`).Scan(&unlinkedCount)
	log.Printf("  Linked to customer: %d", linkedCount)
	log.Printf("  Not linked: %d", unlinkedCount)

	// Check column names
	log.Println("\n--- Invoice Columns ---")
	var columns []string
	db.Raw(`SELECT column_name FROM information_schema.columns WHERE table_name = 'invoices' ORDER BY ordinal_position`).Scan(&columns)
	for _, c := range columns {
		log.Printf("  - %s", c)
	}

	// Sample invoices
	log.Println("\n--- Sample Invoices (first 5) ---")
	var samples []struct {
		ID          string
		InvoiceNo   string
		CustomerID  string
		TotalAmount float64
		Status      string
		InvoiceDate string
	}
	db.Raw(`SELECT id, invoice_number as invoice_no, customer_id, total_amount, status, invoice_date::text FROM invoices LIMIT 5`).Scan(&samples)
	for _, s := range samples {
		custID := s.CustomerID
		if len(custID) > 8 {
			custID = custID[:8] + "..."
		}
		log.Printf("  %s | %.2f BHD | %s | %s | %s", s.InvoiceNo, s.TotalAmount, s.Status, custID, strings.Split(s.InvoiceDate, " ")[0])
	}

	log.Println("\nDone!")
}
