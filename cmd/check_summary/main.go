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

	fmt.Println("=== TABLES WITH DATA ===")
	
	// Get all tables with counts
	var tables []string
	db.Raw(`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name`).Scan(&tables)

	for _, table := range tables {
		var count int64
		db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM \"%s\"", table)).Scan(&count)
		if count > 0 {
			fmt.Printf("  %s: %d rows\n", table, count)
		}
	}

	fmt.Println("\n=== KEY TABLE DETAILS ===")

	// Customers
	fmt.Println("\n--- CUSTOMERS (sample) ---")
	type Cust struct {
		ID   string
		Name string
		Code string
	}
	var custs []Cust
	db.Raw("SELECT id, name, customer_code as code FROM customers LIMIT 5").Scan(&custs)
	for _, c := range custs {
		fmt.Printf("  %s | %s | %s\n", c.Code, c.Name, c.ID[:8]+"...")
	}

	// Invoices
	fmt.Println("\n--- INVOICES (sample) ---")
	type Inv struct {
		InvoiceNumber string
		CustomerID    string
		CustomerName  string
		TotalBhd      float64
		Status        string
	}
	var invs []Inv
	db.Raw("SELECT invoice_number, customer_id, customer_name, total_bhd, status FROM invoices LIMIT 5").Scan(&invs)
	for _, i := range invs {
		custID := i.CustomerID
		if len(custID) > 8 {
			custID = custID[:8] + "..."
		}
		fmt.Printf("  %s | %s | %s | %.2f BHD | %s\n", i.InvoiceNumber, i.CustomerName, custID, i.TotalBhd, i.Status)
	}

	// Check if customer IDs in invoices match customers table
	fmt.Println("\n--- CUSTOMER ID MATCHING ---")
	var matchCount int64
	db.Raw(`SELECT COUNT(*) FROM invoices i 
		INNER JOIN customers c ON i.customer_id = c.id`).Scan(&matchCount)
	fmt.Printf("  Invoices with matching customer_id: %d\n", matchCount)

	var unmatchedCount int64
	db.Raw(`SELECT COUNT(*) FROM invoices i 
		LEFT JOIN customers c ON i.customer_id = c.id
		WHERE c.id IS NULL`).Scan(&unmatchedCount)
	fmt.Printf("  Invoices WITHOUT matching customer_id: %d\n", unmatchedCount)
}
