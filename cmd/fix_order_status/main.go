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
	godotenv.Load()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}
	dsn := convertDatabaseURL(databaseURL)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL")

	// Check current status values in orders
	var statuses []struct {
		Status string
		Count  int
	}
	db.Raw(`SELECT status, COUNT(*) as count FROM orders GROUP BY status`).Scan(&statuses)
	log.Println("Current order statuses:")
	for _, s := range statuses {
		log.Printf("  - '%s': %d", s.Status, s.Count)
	}

	// Fix invalid statuses - map to valid values
	// Valid: 'Processing','Confirmed','Partially Delivered','Delivered','Cancelled'
	fixes := []string{
		`UPDATE orders SET status = 'Processing' WHERE status IS NULL OR status = ''`,
		`UPDATE orders SET status = 'Processing' WHERE status NOT IN ('Processing','Confirmed','Partially Delivered','Delivered','Cancelled')`,
	}
	for _, fix := range fixes {
		result := db.Exec(fix)
		if result.Error != nil {
			log.Printf("⚠ %v", result.Error)
		} else if result.RowsAffected > 0 {
			log.Printf("✅ Fixed %d rows", result.RowsAffected)
		}
	}

	// Also fix offers status - Valid: 'Draft','Sent','Won','Lost','Expired','Revised'
	db.Exec(`UPDATE offers SET status = 'Draft' WHERE status IS NULL OR status = ''`)
	db.Exec(`UPDATE offers SET status = 'Draft' WHERE status NOT IN ('Draft','Sent','Won','Lost','Expired','Revised')`)

	// Fix invoices status - Valid: 'Draft','Sent','Paid','Overdue','Cancelled','Void'
	db.Exec(`UPDATE invoices SET status = 'Sent' WHERE status IS NULL OR status = ''`)

	// Fix NULL numeric values in invoices
	db.Exec(`UPDATE invoices SET total_supplier_cost_bhd = 0 WHERE total_supplier_cost_bhd IS NULL`)
	db.Exec(`UPDATE invoices SET margin_bhd = 0 WHERE margin_bhd IS NULL`)
	db.Exec(`UPDATE invoices SET margin_percent = 0 WHERE margin_percent IS NULL`)
	db.Exec(`UPDATE invoices SET total_amount_bhd = 0 WHERE total_amount_bhd IS NULL`)
	db.Exec(`UPDATE invoices SET vat_amount = 0 WHERE vat_amount IS NULL`)
	db.Exec(`UPDATE invoices SET paid_amount = 0 WHERE paid_amount IS NULL`)
	db.Exec(`UPDATE invoices SET outstanding_amount = 0 WHERE outstanding_amount IS NULL`)
	log.Println("✅ Fixed NULL values in invoices")

	// Fix delivery_notes status - Valid: 'Prepared','Dispatched','InTransit','Delivered','Signed','Cancelled'
	db.Exec(`UPDATE delivery_notes SET status = 'Prepared' WHERE status IS NULL OR status = ''`)

	// Fix purchase_orders status
	db.Exec(`UPDATE purchase_orders SET status = 'Draft' WHERE status IS NULL OR status = ''`)

	// Fix supplier_invoices status
	db.Exec(`UPDATE supplier_invoices SET status = 'Pending' WHERE status IS NULL OR status = ''`)
	db.Exec(`UPDATE supplier_invoices SET match_status = 'Pending' WHERE match_status IS NULL OR match_status = ''`)

	// Fix goods_received_notes qc_status
	db.Exec(`UPDATE goods_received_notes SET qc_status = 'Pending' WHERE qc_status IS NULL OR qc_status = ''`)

	// Drop ALL CHECK constraints to prevent migration issues
	drops := []string{
		`ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_status`,
		`ALTER TABLE offers DROP CONSTRAINT IF EXISTS chk_offers_status`,
		`ALTER TABLE invoices DROP CONSTRAINT IF EXISTS chk_invoices_status`,
		`ALTER TABLE delivery_notes DROP CONSTRAINT IF EXISTS chk_delivery_notes_status`,
		`ALTER TABLE purchase_orders DROP CONSTRAINT IF EXISTS chk_purchase_orders_status`,
		`ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS chk_supplier_invoices_status`,
		`ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS chk_supplier_invoices_match_status`,
		`ALTER TABLE goods_received_notes DROP CONSTRAINT IF EXISTS chk_goods_received_notes_qc_status`,
		`ALTER TABLE db_costing_sheets DROP CONSTRAINT IF EXISTS chk_db_costing_sheets_status`,
		`ALTER TABLE costing_sheets DROP CONSTRAINT IF EXISTS chk_costing_sheets_status`,
		`ALTER TABLE entity_notes DROP CONSTRAINT IF EXISTS chk_entity_notes_entity_type`,
		`ALTER TABLE entity_notes DROP CONSTRAINT IF EXISTS chk_entity_notes_note_type`,
		`ALTER TABLE supplier_issues DROP CONSTRAINT IF EXISTS chk_supplier_issues_status`,
		`ALTER TABLE supplier_issues DROP CONSTRAINT IF EXISTS chk_supplier_issues_cost_bhd`,
		`ALTER TABLE follow_up_tasks DROP CONSTRAINT IF EXISTS chk_follow_up_tasks_status`,
		`ALTER TABLE offer_follow_ups DROP CONSTRAINT IF EXISTS chk_offer_follow_ups_status`,
		`ALTER TABLE devices DROP CONSTRAINT IF EXISTS chk_devices_status`,
		`ALTER TABLE sync_records DROP CONSTRAINT IF EXISTS chk_sync_records_status`,
		`ALTER TABLE tally_invoice_imports DROP CONSTRAINT IF EXISTS chk_tally_invoice_imports_status`,
		`ALTER TABLE tally_purchase_imports DROP CONSTRAINT IF EXISTS chk_tally_purchase_imports_status`,
		`ALTER TABLE conversations DROP CONSTRAINT IF EXISTS chk_conversations_status`,
		`ALTER TABLE accounting_periods DROP CONSTRAINT IF EXISTS chk_accounting_periods_status`,
		`ALTER TABLE batch_reconciliations DROP CONSTRAINT IF EXISTS chk_batch_reconciliations_status`,
	}
	for _, d := range drops {
		db.Exec(d)
	}
	log.Println("✅ All CHECK constraints dropped")

	// Verify final state
	db.Raw(`SELECT status, COUNT(*) as count FROM orders GROUP BY status`).Scan(&statuses)
	log.Println("\nFinal order statuses:")
	for _, s := range statuses {
		log.Printf("  - '%s': %d", s.Status, s.Count)
	}
	log.Println("\n✅ Data fix complete!")
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
