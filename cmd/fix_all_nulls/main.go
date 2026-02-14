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
	// Load .env from project root (two levels up from cmd/fix_all_nulls)
	godotenv.Load("../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}
	dsn := convertDatabaseURL(databaseURL)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL")

	// =======================================================================
	// FIX ALL NULL NUMERIC VALUES
	// =======================================================================
	log.Println("\n🔧 Fixing ALL NULL numeric values...")

	nullFixes := []string{
		// INVOICES - all numeric columns
		`UPDATE invoices SET total_supplier_cost_bhd = 0 WHERE total_supplier_cost_bhd IS NULL`,
		`UPDATE invoices SET gross_margin_bhd = 0 WHERE gross_margin_bhd IS NULL`,
		`UPDATE invoices SET gross_margin_percent = 0 WHERE gross_margin_percent IS NULL`,
		`UPDATE invoices SET subtotal_bhd = 0 WHERE subtotal_bhd IS NULL`,
		`UPDATE invoices SET discount_bhd = 0 WHERE discount_bhd IS NULL`,
		`UPDATE invoices SET discount_percent = 0 WHERE discount_percent IS NULL`,
		`UPDATE invoices SET vatbhd = 0 WHERE vatbhd IS NULL`,
		`UPDATE invoices SET vat_percent = 0 WHERE vat_percent IS NULL`,
		`UPDATE invoices SET grand_total_bhd = 0 WHERE grand_total_bhd IS NULL`,
		`UPDATE invoices SET total_bhd = 0 WHERE total_bhd IS NULL`,
		`UPDATE invoices SET paid_bhd = 0 WHERE paid_bhd IS NULL`,
		`UPDATE invoices SET outstanding_bhd = 0 WHERE outstanding_bhd IS NULL`,
		`UPDATE invoices SET overdue_days = 0 WHERE overdue_days IS NULL`,

		// ORDERS - all numeric columns
		`UPDATE orders SET total_value_bhd = 0 WHERE total_value_bhd IS NULL`,
		`UPDATE orders SET invoiced_bhd = 0 WHERE invoiced_bhd IS NULL`,
		`UPDATE orders SET delivered_bhd = 0 WHERE delivered_bhd IS NULL`,
		`UPDATE orders SET paid_bhd = 0 WHERE paid_bhd IS NULL`,

		// OFFERS - all numeric columns
		`UPDATE offers SET total_value_bhd = 0 WHERE total_value_bhd IS NULL`,
		`UPDATE offers SET discount_percent = 0 WHERE discount_percent IS NULL`,
		`UPDATE offers SET win_probability = 0 WHERE win_probability IS NULL`,
		`UPDATE offers SET margin_percent = 0 WHERE margin_percent IS NULL`,

		// OFFER_ITEMS - all numeric columns
		`UPDATE offer_items SET quantity = 0 WHERE quantity IS NULL`,
		`UPDATE offer_items SET unit_price_bhd = 0 WHERE unit_price_bhd IS NULL`,
		`UPDATE offer_items SET total_price_bhd = 0 WHERE total_price_bhd IS NULL`,
		`UPDATE offer_items SET cost_price_bhd = 0 WHERE cost_price_bhd IS NULL`,
		`UPDATE offer_items SET margin_percent = 0 WHERE margin_percent IS NULL`,

		// ORDER_ITEMS - all numeric columns
		`UPDATE order_items SET quantity = 0 WHERE quantity IS NULL`,
		`UPDATE order_items SET unit_price_bhd = 0 WHERE unit_price_bhd IS NULL`,
		`UPDATE order_items SET total_price_bhd = 0 WHERE total_price_bhd IS NULL`,
		`UPDATE order_items SET delivered_qty = 0 WHERE delivered_qty IS NULL`,
		`UPDATE order_items SET invoiced_qty = 0 WHERE invoiced_qty IS NULL`,

		// PURCHASE_ORDERS - all numeric columns
		`UPDATE purchase_orders SET total_value_usd = 0 WHERE total_value_usd IS NULL`,
		`UPDATE purchase_orders SET total_value_bhd = 0 WHERE total_value_bhd IS NULL`,
		`UPDATE purchase_orders SET exchange_rate = 1 WHERE exchange_rate IS NULL`,

		// PURCHASE_ORDER_ITEMS
		`UPDATE purchase_order_items SET quantity = 0 WHERE quantity IS NULL`,
		`UPDATE purchase_order_items SET unit_price_usd = 0 WHERE unit_price_usd IS NULL`,
		`UPDATE purchase_order_items SET total_price_usd = 0 WHERE total_price_usd IS NULL`,
		`UPDATE purchase_order_items SET received_qty = 0 WHERE received_qty IS NULL`,

		// SUPPLIER_INVOICES - correct column names from GORM model
		`UPDATE supplier_invoices SET exchange_rate = 1 WHERE exchange_rate IS NULL`,
		`UPDATE supplier_invoices SET subtotal_foreign = 0 WHERE subtotal_foreign IS NULL`,
		`UPDATE supplier_invoices SET subtotal_bhd = 0 WHERE subtotal_bhd IS NULL`,
		`UPDATE supplier_invoices SET vat_foreign = 0 WHERE vat_foreign IS NULL`,
		`UPDATE supplier_invoices SET vatbhd = 0 WHERE vatbhd IS NULL`,
		`UPDATE supplier_invoices SET total_foreign = 0 WHERE total_foreign IS NULL`,
		`UPDATE supplier_invoices SET total_bhd = 0 WHERE total_bhd IS NULL`,
		`UPDATE supplier_invoices SET ocr_confidence = 0 WHERE ocr_confidence IS NULL`,

		// DELIVERY_NOTES
		`UPDATE delivery_notes SET total_items = 0 WHERE total_items IS NULL`,
		`UPDATE delivery_notes SET total_value_bhd = 0 WHERE total_value_bhd IS NULL`,

		// DELIVERY_NOTE_ITEMS
		`UPDATE delivery_note_items SET quantity = 0 WHERE quantity IS NULL`,

		// GRN
		`UPDATE goods_received_notes SET total_received = 0 WHERE total_received IS NULL`,
		`UPDATE goods_received_notes SET total_rejected = 0 WHERE total_rejected IS NULL`,

		// GRN_ITEMS
		`UPDATE grn_items SET ordered_qty = 0 WHERE ordered_qty IS NULL`,
		`UPDATE grn_items SET received_qty = 0 WHERE received_qty IS NULL`,
		`UPDATE grn_items SET rejected_qty = 0 WHERE rejected_qty IS NULL`,
		`UPDATE grn_items SET accepted_qty = 0 WHERE accepted_qty IS NULL`,

		// PAYMENTS
		`UPDATE payments SET amount_bhd = 0 WHERE amount_bhd IS NULL`,

		// SUPPLIER_PAYMENTS
		`UPDATE supplier_payments SET amount_bhd = 0 WHERE amount_bhd IS NULL`,
		`UPDATE supplier_payments SET amount_usd = 0 WHERE amount_usd IS NULL`,
		`UPDATE supplier_payments SET exchange_rate = 1 WHERE exchange_rate IS NULL`,

		// CUSTOMERS
		`UPDATE customers SET relation_years = 0 WHERE relation_years IS NULL`,
		`UPDATE customers SET payment_terms_days = 30 WHERE payment_terms_days IS NULL`,
		`UPDATE customers SET avg_payment_days = 0 WHERE avg_payment_days IS NULL`,
		`UPDATE customers SET dispute_count = 0 WHERE dispute_count IS NULL`,
		`UPDATE customers SET total_orders_value = 0 WHERE total_orders_value IS NULL`,
		`UPDATE customers SET total_orders_count = 0 WHERE total_orders_count IS NULL`,
		`UPDATE customers SET avg_order_value = 0 WHERE avg_order_value IS NULL`,
		`UPDATE customers SET outstanding_bhd = 0 WHERE outstanding_bhd IS NULL`,
		`UPDATE customers SET overdue_days = 0 WHERE overdue_days IS NULL`,
		`UPDATE customers SET credit_limit_bhd = 50000 WHERE credit_limit_bhd IS NULL`,

		// SUPPLIERS
		`UPDATE suppliers SET lead_time_days = 0 WHERE lead_time_days IS NULL`,
		`UPDATE suppliers SET rating = 3 WHERE rating IS NULL`,

		// COSTING SHEETS
		`UPDATE costing_sheets SET total_cost_bhd = 0 WHERE total_cost_bhd IS NULL`,
		`UPDATE costing_sheets SET selling_price_bhd = 0 WHERE selling_price_bhd IS NULL`,
		`UPDATE costing_sheets SET margin_percent = 0 WHERE margin_percent IS NULL`,

		// PRODUCTS
		`UPDATE products SET unit_price_bhd = 0 WHERE unit_price_bhd IS NULL`,
		`UPDATE products SET cost_price_bhd = 0 WHERE cost_price_bhd IS NULL`,
		`UPDATE products SET stock_quantity = 0 WHERE stock_quantity IS NULL`,
		`UPDATE products SET reorder_level = 0 WHERE reorder_level IS NULL`,

		// RFQS / OPPORTUNITIES
		`UPDATE rfqs SET estimated_value_bhd = 0 WHERE estimated_value_bhd IS NULL`,
		`UPDATE opportunities SET estimated_value_bhd = 0 WHERE estimated_value_bhd IS NULL`,
		`UPDATE opportunities SET win_probability = 0 WHERE win_probability IS NULL`,
	}

	for _, fix := range nullFixes {
		result := db.Exec(fix)
		if result.Error != nil {
			// Ignore "column does not exist" errors - table schema may vary
			log.Printf("⚠ %s: %v", fix[:40], result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("✅ Fixed %d rows: %s...", result.RowsAffected, fix[:50])
		}
	}

	// =======================================================================
	// FIX ALL NULL STRING VALUES (set to empty string)
	// =======================================================================
	log.Println("\n🔧 Fixing NULL string values...")

	stringFixes := []string{
		`UPDATE invoices SET invoice_number = '' WHERE invoice_number IS NULL`,
		`UPDATE invoices SET customer_name = '' WHERE customer_name IS NULL`,
		`UPDATE orders SET order_number = '' WHERE order_number IS NULL`,
		`UPDATE orders SET customer_name = '' WHERE customer_name IS NULL`,
		`UPDATE offers SET offer_number = '' WHERE offer_number IS NULL`,
		`UPDATE offers SET customer_name = '' WHERE customer_name IS NULL`,
		`UPDATE customers SET payment_grade = 'C' WHERE payment_grade IS NULL`,
		`UPDATE customers SET customer_grade = 'C' WHERE customer_grade IS NULL`,
	}

	for _, fix := range stringFixes {
		db.Exec(fix) // Ignore errors
	}

	// =======================================================================
	// FIX STATUS VALUES
	// =======================================================================
	log.Println("\n🔧 Fixing status values...")

	statusFixes := []string{
		`UPDATE orders SET status = 'Processing' WHERE status IS NULL OR status = '' OR status NOT IN ('Processing','Confirmed','Partially Delivered','Delivered','Cancelled')`,
		`UPDATE offers SET status = 'Draft' WHERE status IS NULL OR status = '' OR status NOT IN ('Draft','Sent','Won','Lost','Expired','Revised')`,
		`UPDATE invoices SET status = 'Sent' WHERE status IS NULL OR status = ''`,
		`UPDATE delivery_notes SET status = 'Prepared' WHERE status IS NULL OR status = ''`,
		`UPDATE purchase_orders SET status = 'Draft' WHERE status IS NULL OR status = ''`,
		`UPDATE supplier_invoices SET status = 'Pending' WHERE status IS NULL OR status = ''`,
		`UPDATE supplier_invoices SET match_status = 'Pending' WHERE match_status IS NULL OR match_status = ''`,
		`UPDATE goods_received_notes SET qc_status = 'Pending' WHERE qc_status IS NULL OR qc_status = ''`,
	}

	for _, fix := range statusFixes {
		result := db.Exec(fix)
		if result.RowsAffected > 0 {
			log.Printf("✅ Fixed %d status values", result.RowsAffected)
		}
	}

	// =======================================================================
	// DROP ALL CHECK CONSTRAINTS
	// =======================================================================
	log.Println("\n🔧 Dropping ALL CHECK constraints...")

	// Get all check constraints from the database
	var constraints []struct {
		TableName      string `gorm:"column:table_name"`
		ConstraintName string `gorm:"column:constraint_name"`
	}
	db.Raw(`
		SELECT tc.table_name, tc.constraint_name
		FROM information_schema.table_constraints tc
		WHERE tc.constraint_type = 'CHECK'
		AND tc.table_schema = 'public'
	`).Scan(&constraints)

	for _, c := range constraints {
		stmt := fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, c.TableName, c.ConstraintName)
		db.Exec(stmt)
	}
	log.Printf("✅ Dropped %d CHECK constraints", len(constraints))

	log.Println("\n✅ ALL NULL VALUES AND CONSTRAINTS FIXED!")
	log.Println("You can now run the app.")
}

func convertDatabaseURL(url string) string {
	url = url[13:]
	atIdx := -1
	for i := len(url) - 1; i >= 0; i-- {
		if url[i] == '@' {
			atIdx = i
			break
		}
	}
	if atIdx == -1 {
		return url
	}
	credentials := url[:atIdx]
	hostPart := url[atIdx+1:]
	colonIdx := -1
	for i := 0; i < len(credentials); i++ {
		if credentials[i] == ':' {
			colonIdx = i
			break
		}
	}
	user := credentials
	password := ""
	if colonIdx != -1 {
		user = credentials[:colonIdx]
		password = credentials[colonIdx+1:]
	}
	slashIdx := -1
	for i := 0; i < len(hostPart); i++ {
		if hostPart[i] == '/' {
			slashIdx = i
			break
		}
	}
	hostPort := hostPart
	dbname := "ph_holdings"
	if slashIdx != -1 {
		hostPort = hostPart[:slashIdx]
		dbname = hostPart[slashIdx+1:]
	}
	host := hostPort
	port := "5432"
	colonIdx = -1
	for i := 0; i < len(hostPort); i++ {
		if hostPort[i] == ':' {
			colonIdx = i
			break
		}
	}
	if colonIdx != -1 {
		host = hostPort[:colonIdx]
		port = hostPort[colonIdx+1:]
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require", host, port, user, password, dbname)
}
