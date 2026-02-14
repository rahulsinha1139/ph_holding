package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   MIGRATE SUPABASE SCHEMA")
	fmt.Println("   Adding missing columns from SQLite")
	fmt.Println("========================================")

	godotenv.Load()

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=postgres sslmode=require",
		os.Getenv("SUPABASE_DB_HOST"),
		os.Getenv("SUPABASE_DB_PORT"),
		os.Getenv("SUPABASE_DB_USER"),
		os.Getenv("SUPABASE_DB_PASSWORD"))

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatal("Supabase error:", err)
	}
	fmt.Println("✅ Supabase connected")

	// Migrations - add missing columns
	migrations := []string{
		// Customers
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS address_line1 TEXT",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS ar_risk_tier TEXT DEFAULT 'Low'",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS avg_order_value NUMERIC DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS avg_payment_days INTEGER DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS credit_limit_bhd NUMERIC DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS customer_code TEXT",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS customer_grade TEXT DEFAULT 'B'",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS customer_id TEXT",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS customer_type TEXT",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS dispute_count INTEGER DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS has_abb_competition BOOLEAN DEFAULT false",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS is_credit_blocked BOOLEAN DEFAULT false",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS is_emergency_only BOOLEAN DEFAULT false",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS last_order_date TIMESTAMP",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS outstanding_bhd NUMERIC DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS overdue_days INTEGER DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS payment_grade TEXT DEFAULT 'B'",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS payment_terms_days INTEGER DEFAULT 30",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS relation_years INTEGER DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS requires_prepayment BOOLEAN DEFAULT false",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS short_code TEXT",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS tax_code TEXT",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS total_orders_count INTEGER DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS total_orders_value NUMERIC DEFAULT 0",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS trn TEXT",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS vat_number TEXT",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1",
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS created_by TEXT",

		// Suppliers
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS account_number TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS bank_name TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS brands_handled TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS iban TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS lead_time_days INTEGER DEFAULT 0",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS primary_contact TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS product_types TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS rating INTEGER DEFAULT 3",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS supplier_code TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS supplier_name TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS supplier_type TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS swift_code TEXT",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS tax_id TEXT",

		// Supplier Invoices
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS approved_at TIMESTAMP",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS approved_by TEXT",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS attachment_path TEXT",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS currency TEXT DEFAULT 'BHD'",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC DEFAULT 1",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS matched_to_po BOOLEAN DEFAULT false",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS ocr_confidence NUMERIC DEFAULT 0",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS ocr_raw_text TEXT",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS original_amount NUMERIC DEFAULT 0",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS payment_status TEXT",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS po_id TEXT",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS supplier_name TEXT",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS vat_bhd NUMERIC DEFAULT 0",

		// Supplier Payments
		"ALTER TABLE supplier_payments ADD COLUMN IF NOT EXISTS bank_account_id TEXT",
		"ALTER TABLE supplier_payments ADD COLUMN IF NOT EXISTS cleared_date TIMESTAMP",
		"ALTER TABLE supplier_payments ADD COLUMN IF NOT EXISTS payment_reference TEXT",
		"ALTER TABLE supplier_payments ADD COLUMN IF NOT EXISTS status TEXT DEFAULT 'completed'",
		"ALTER TABLE supplier_payments ADD COLUMN IF NOT EXISTS supplier_invoice_id TEXT",

		// Purchase Orders
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS approved_at TIMESTAMP",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS approved_by TEXT",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS currency TEXT DEFAULT 'BHD'",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS exchange_rate NUMERIC DEFAULT 1",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS expected_date TIMESTAMP",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS order_id TEXT",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS original_amount NUMERIC DEFAULT 0",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS po_date TIMESTAMP",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS po_number TEXT",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS rfq_id TEXT",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS supplier_name TEXT",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS total_bhd NUMERIC DEFAULT 0",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS vat_bhd NUMERIC DEFAULT 0",

		// Customer Contacts
		"ALTER TABLE customer_contacts ADD COLUMN IF NOT EXISTS is_primary BOOLEAN DEFAULT false",
		"ALTER TABLE customer_contacts ADD COLUMN IF NOT EXISTS job_title TEXT",
		"ALTER TABLE customer_contacts ADD COLUMN IF NOT EXISTS name TEXT",
		"ALTER TABLE customer_contacts ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP",

		// Missing GORM columns
		"ALTER TABLE customers ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP",
		"ALTER TABLE suppliers ADD COLUMN IF NOT EXISTS bank_account TEXT",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP",
		"ALTER TABLE supplier_invoices ADD COLUMN IF NOT EXISTS created_by TEXT",
		"ALTER TABLE supplier_payments ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP",
		"ALTER TABLE supplier_payments ADD COLUMN IF NOT EXISTS created_by TEXT",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP",
		"ALTER TABLE purchase_orders ADD COLUMN IF NOT EXISTS created_by TEXT",
	}

	fmt.Println("\n📝 Running migrations...")
	success := 0
	for _, sql := range migrations {
		if err := db.Exec(sql).Error; err != nil {
			fmt.Printf("  ⚠️  %s\n", err)
		} else {
			success++
		}
	}
	fmt.Printf("\n✅ %d/%d migrations successful\n", success, len(migrations))

	fmt.Println("\n========================================")
	fmt.Println("   MIGRATION COMPLETE")
	fmt.Println("========================================")
}
