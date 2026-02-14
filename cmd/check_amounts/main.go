package main

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := "host=dpg-d5rmv2hr0fns73e8sq1g-a.singapore-postgres.render.com port=5432 user=ph_admin password=Indcy4LP1o5uDJT2oFQ4KzyXKcFllSKu dbname=ph_holdings sslmode=require"
	db, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

	var total float64
	db.Raw("SELECT COALESCE(SUM(total_bhd), 0) FROM invoices").Scan(&total)
	log.Printf("Total (total_bhd): %.2f BHD", total)

	var subtotal float64
	db.Raw("SELECT COALESCE(SUM(subtotal_bhd), 0) FROM invoices").Scan(&subtotal)
	log.Printf("Subtotal (subtotal_bhd): %.2f BHD", subtotal)

	// Sample with amounts
	type Sample struct {
		InvoiceNumber string
		CustomerName  string
		SubtotalBhd   float64
		TotalBhd      float64
		Status        string
	}
	var samples []Sample
	db.Raw("SELECT invoice_number, customer_name, subtotal_bhd, total_bhd, status FROM invoices LIMIT 5").Scan(&samples)
	log.Println("\n--- Sample Invoices ---")
	for _, s := range samples {
		log.Printf("  %s | %s | Subtotal: %.2f | Total: %.2f | %s", s.InvoiceNumber, s.CustomerName, s.SubtotalBhd, s.TotalBhd, s.Status)
	}
}
