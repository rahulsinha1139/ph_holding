package main

import (
	"fmt"
	"log"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   Verifying Supabase via GORM")
	fmt.Println("========================================")

	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dsn := fmt.Sprintf("host=aws-1-ap-southeast-1.pooler.supabase.com port=5432 user=postgres.xkyydcxxkzjbnsrmzkpp password=%s dbname=postgres sslmode=require", supabasePassword)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	fmt.Println("✅ Supabase connected\n")

	tables := []string{
		"customers",
		"suppliers",
		"orders",
		"invoices",
		"payments",
		"supplier_invoices",
		"supplier_payments",
		"purchase_orders",
		"rfq_data",
		"offers",
		"offer_items",
		"customer_contacts",
	}

	fmt.Println("Table Counts in Supabase:")
	fmt.Println("─────────────────────────────")
	totalRecords := 0
	for _, table := range tables {
		var count int64
		db.Table(table).Count(&count)
		fmt.Printf("%-25s %d\n", table+":", count)
		totalRecords += int(count)
	}

	fmt.Println("\n========================================")
	fmt.Printf("  Total Records: %d\n", totalRecords)
	fmt.Println("========================================")

	// Sample data verification
	fmt.Println("\n🔍 Sample Data Verification:")
	fmt.Println("─────────────────────────────")

	// Top 5 customers by business_name
	type Customer struct {
		BusinessName string
	}
	var customers []Customer
	db.Table("customers").Select("business_name").Order("business_name").Limit(5).Find(&customers)
	fmt.Println("\nTop 5 Customers:")
	for i, c := range customers {
		fmt.Printf("  %d. %s\n", i+1, c.BusinessName)
	}

	// Recent invoices
	type Invoice struct {
		InvoiceNumber string
		CustomerName  string
		GrandTotalBHD float64
	}
	var invoices []Invoice
	db.Table("invoices").Select("invoice_number, customer_name, grand_total_bhd").Order("created_at DESC").Limit(5).Find(&invoices)
	fmt.Println("\nRecent 5 Invoices:")
	for i, inv := range invoices {
		fmt.Printf("  %d. %s - %s (%.3f BHD)\n", i+1, inv.InvoiceNumber, inv.CustomerName, inv.GrandTotalBHD)
	}

	fmt.Println("\n✅ Verification Complete!")
}
