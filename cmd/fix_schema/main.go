package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// addColumnIfNotExists safely adds a column if it doesn't exist
func addColumnIfNotExists(db *gorm.DB, table, column, dataType string) {
	var count int64
	db.Raw("SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?", table, column).Scan(&count)
	if count == 0 {
		sql := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, dataType)
		if err := db.Exec(sql).Error; err != nil {
			log.Printf("⚠️  Failed to add %s.%s: %v", table, column, err)
		} else {
			log.Printf("✅ Added %s.%s", table, column)
		}
	}
}

func main() {
	// Find database
	dbPath := "ph_holdings.db"
	if len(os.Args) > 1 {
		dbPath = os.Args[1]
	}

	// Check if build/bin/data exists
	buildDBPath := filepath.Join("build", "bin", "data", "ph_holdings.db")
	if _, err := os.Stat(buildDBPath); err == nil {
		dbPath = buildDBPath
	}

	log.Printf("🔧 Schema Migration Tool")
	log.Printf("📂 Database: %s", dbPath)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}

	log.Println("\n📋 Migrating supplier_invoices...")
	siCols := map[string]string{
		"purchase_order_id": "TEXT", "grn_id": "TEXT", "order_id": "TEXT",
		"currency": "TEXT DEFAULT 'BHD'", "exchange_rate": "REAL DEFAULT 1",
		"subtotal_foreign": "REAL DEFAULT 0", "vat_foreign": "REAL DEFAULT 0",
		"total_foreign": "REAL DEFAULT 0", "match_status": "TEXT DEFAULT 'Pending'",
		"po_match_ok": "INTEGER DEFAULT 0", "grn_match_ok": "INTEGER DEFAULT 0",
		"approved_by": "TEXT", "approved_at": "DATETIME", "updated_by": "TEXT",
		"payment_date": "DATETIME", "payment_ref": "TEXT", "payment_method": "TEXT",
		"ocr_document_id": "TEXT", "ocr_confidence": "REAL DEFAULT 0",
		"discrepancy_reason": "TEXT", "dispute_reason": "TEXT", "journal_entry_id": "TEXT",
	}
	for col, dtype := range siCols {
		addColumnIfNotExists(db, "supplier_invoices", col, dtype)
	}

	log.Println("\n📋 Migrating invoices...")
	invCols := map[string]string{
		"updated_by": "TEXT", "rfq_id": "TEXT", "quote_id": "TEXT", "offer_id": "TEXT",
		"offer_number": "TEXT", "delivery_note_id": "TEXT", "delivery_note_number": "TEXT",
		"total_supplier_cost_bhd": "REAL DEFAULT 0", "gross_margin_bhd": "REAL DEFAULT 0",
		"gross_margin_percent": "REAL DEFAULT 0", "customer_reference": "TEXT",
		"attention_person": "TEXT", "attention_company": "TEXT", "attention_phone": "TEXT",
		"attention_address": "TEXT", "delivery_weeks": "TEXT", "country_of_origin": "TEXT",
		"issued_by": "TEXT", "contact_phone": "TEXT", "discount_percent": "REAL DEFAULT 0",
		"payment_terms": "TEXT", "delivery_terms": "TEXT", "customer_po_number": "TEXT",
		"field_visibility": "TEXT", "vatbhd": "REAL DEFAULT 0", "vat_percent": "REAL DEFAULT 0",
		"journal_entry_id": "TEXT",
	}
	for col, dtype := range invCols {
		addColumnIfNotExists(db, "invoices", col, dtype)
	}

	log.Println("\n📋 Migrating purchase_orders...")
	poCols := map[string]string{
		"order_id": "TEXT", "rfq_id": "TEXT", "currency": "TEXT DEFAULT 'BHD'",
		"exchange_rate": "REAL DEFAULT 1", "subtotal_foreign": "REAL DEFAULT 0",
		"subtotal_bhd": "REAL DEFAULT 0", "vat_amount": "REAL DEFAULT 0",
		"total_foreign": "REAL DEFAULT 0", "total_bhd": "REAL DEFAULT 0",
		"payment_terms": "TEXT", "payment_due_date": "DATETIME",
		"approved_by": "TEXT", "approved_at": "DATETIME", "updated_by": "TEXT",
	}
	for col, dtype := range poCols {
		addColumnIfNotExists(db, "purchase_orders", col, dtype)
	}

	log.Println("\n📋 Migrating purchase_order_items...")
	poiCols := map[string]string{
		"order_item_id": "TEXT", "supplier_part_number": "TEXT",
		"unit_price_foreign": "REAL DEFAULT 0", "unit_price_bhd": "REAL DEFAULT 0",
		"total_foreign": "REAL DEFAULT 0", "total_bhd": "REAL DEFAULT 0",
		"quantity_received": "REAL DEFAULT 0",
	}
	for col, dtype := range poiCols {
		addColumnIfNotExists(db, "purchase_order_items", col, dtype)
	}

	log.Println("\n📋 Migrating goods_received_notes...")
	grnCols := map[string]string{
		"warehouse_id": "TEXT", "supplier_dn_number": "TEXT",
		"qc_status": "TEXT DEFAULT 'Pending'", "qc_notes": "TEXT",
		"qc_date": "DATETIME", "qc_by": "TEXT", "updated_by": "TEXT",
	}
	for col, dtype := range grnCols {
		addColumnIfNotExists(db, "goods_received_notes", col, dtype)
	}

	log.Println("\n📋 Migrating grn_items...")
	grniCols := map[string]string{
		"po_item_id": "TEXT", "quantity_ordered": "REAL DEFAULT 0",
		"quantity_accepted": "REAL DEFAULT 0", "quantity_rejected": "REAL DEFAULT 0",
		"rejection_reason": "TEXT",
	}
	for col, dtype := range grniCols {
		addColumnIfNotExists(db, "grn_items", col, dtype)
	}

	log.Println("\n📋 Migrating delivery_notes...")
	dnCols := map[string]string{
		"customer_id": "TEXT", "delivery_address": "TEXT", "contact_person": "TEXT",
		"contact_phone": "TEXT", "driver_name": "TEXT", "vehicle_number": "TEXT",
		"transport_method": "TEXT", "updated_by": "TEXT", "signed_by": "TEXT",
		"signed_at": "DATETIME", "signature_image": "TEXT",
		"is_partial_delivery": "INTEGER DEFAULT 0", "delivery_sequence": "INTEGER DEFAULT 1",
		"total_deliveries": "INTEGER DEFAULT 1",
	}
	for col, dtype := range dnCols {
		addColumnIfNotExists(db, "delivery_notes", col, dtype)
	}

	log.Println("\n📋 Migrating delivery_note_items...")
	dniCols := map[string]string{
		"order_item_id": "TEXT", "product_id": "TEXT", "product_code": "TEXT",
		"quantity_ordered": "REAL DEFAULT 0", "quantity_delivered": "REAL DEFAULT 0",
		"quantity_remaining": "REAL DEFAULT 0",
	}
	for col, dtype := range dniCols {
		addColumnIfNotExists(db, "delivery_note_items", col, dtype)
	}

	log.Println("\n📋 Migrating order_items...")
	oiCols := map[string]string{
		"quantity_shipped": "REAL DEFAULT 0", "quantity_invoiced": "REAL DEFAULT 0",
		"equipment": "TEXT", "model": "TEXT", "specification": "TEXT",
		"detailed_description": "TEXT", "currency": "TEXT", "fob": "REAL DEFAULT 0",
		"freight": "REAL DEFAULT 0", "total_cost": "REAL DEFAULT 0",
		"margin_percent": "REAL DEFAULT 0", "total_price": "REAL DEFAULT 0",
	}
	for col, dtype := range oiCols {
		addColumnIfNotExists(db, "order_items", col, dtype)
	}

	log.Println("\n📋 Migrating products...")
	prodCols := map[string]string{
		"sku": "TEXT", "part_number": "TEXT", "hs_code": "TEXT",
		"unit_of_measure": "TEXT", "datasheet_url": "TEXT", "specifications": "TEXT",
	}
	for col, dtype := range prodCols {
		addColumnIfNotExists(db, "products", col, dtype)
	}

	log.Println("\n📋 Migrating opportunities...")
	oppCols := map[string]string{
		"offer_id": "TEXT", "salesperson": "TEXT", "expected_date": "DATETIME",
		"closed_date": "DATETIME", "revenue_bhd": "REAL DEFAULT 0",
		"cost_bhd": "REAL DEFAULT 0", "profit_bhd": "REAL DEFAULT 0",
		"spoc_status": "TEXT", "wip_status": "TEXT", "regime": "INTEGER DEFAULT 1",
		"confidence": "REAL DEFAULT 0", "r1": "REAL DEFAULT 0", "r2": "REAL DEFAULT 0",
		"r3": "REAL DEFAULT 0", "has_abb_competition": "INTEGER DEFAULT 0",
		"product_type": "TEXT", "won_reason": "TEXT", "lost_reason": "TEXT",
	}
	for col, dtype := range oppCols {
		addColumnIfNotExists(db, "opportunities", col, dtype)
	}

	log.Println("\n📋 Migrating users...")
	userCols := map[string]string{
		"display_name": "TEXT", "department": "TEXT", "job_title": "TEXT",
		"password_changed_at": "DATETIME", "must_change_password": "INTEGER DEFAULT 0",
	}
	for col, dtype := range userCols {
		addColumnIfNotExists(db, "users", col, dtype)
	}

	log.Println("\n📋 Migrating journal_entries...")
	jeCols := map[string]string{
		"source_type": "TEXT", "source_id": "TEXT", "is_auto_generated": "INTEGER DEFAULT 0",
		"reversed_by_id": "TEXT", "reverses_id": "TEXT", "updated_by": "TEXT",
	}
	for col, dtype := range jeCols {
		addColumnIfNotExists(db, "journal_entries", col, dtype)
	}

	log.Println("\n📋 Migrating journal_lines...")
	addColumnIfNotExists(db, "journal_lines", "updated_by", "TEXT")

	log.Println("\n📋 Migrating chart_of_accounts...")
	coaCols := map[string]string{
		"is_vat_account": "INTEGER DEFAULT 0", "vat_direction": "TEXT",
		"parent_account_id": "TEXT", "account_group": "TEXT",
	}
	for col, dtype := range coaCols {
		addColumnIfNotExists(db, "chart_of_accounts", col, dtype)
	}

	// Create invoice_items table if not exists
	log.Println("\n📋 Creating invoice_items table if needed...")
	db.Exec(`CREATE TABLE IF NOT EXISTS invoice_items (
		id TEXT PRIMARY KEY,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME,
		version INTEGER DEFAULT 1,
		created_by TEXT,
		invoice_id TEXT,
		line_number INTEGER DEFAULT 0,
		description TEXT,
		quantity REAL DEFAULT 0,
		rate REAL DEFAULT 0,
		total_bhd REAL DEFAULT 0,
		product_id TEXT,
		product_code TEXT,
		equipment TEXT,
		model TEXT,
		specification TEXT,
		detailed_description TEXT,
		currency TEXT,
		fob REAL DEFAULT 0,
		freight REAL DEFAULT 0,
		total_cost REAL DEFAULT 0,
		margin_percent REAL DEFAULT 0,
		total_price REAL DEFAULT 0
	)`)

	// Verify counts
	log.Println("\n📊 Verification:")
	tables := []string{"offers", "orders", "invoices", "customers", "suppliers", "supplier_invoices", "purchase_orders"}
	for _, t := range tables {
		var count int64
		db.Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s", t)).Scan(&count)
		log.Printf("   %s: %d records", strings.Title(t), count)
	}

	log.Println("\n✅ Schema migration complete!")
}
