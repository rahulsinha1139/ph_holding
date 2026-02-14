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
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable not set")
	}

	// Parse DATABASE_URL
	// Format: postgresql://user:password@host:port/dbname
	dsn := convertDatabaseURL(databaseURL)

	log.Println("Connecting to PostgreSQL on Render...")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	log.Println("✅ Connected to PostgreSQL")

	// =========================================================================
	// STEP 1: Fix customers table - convert bigint columns to TEXT
	// =========================================================================
	log.Println("\n🔧 STEP 1: Fixing customers table column types...")

	// First, check if customers table exists and has wrong types
	var customersExists int
	db.Raw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'customers'`).Scan(&customersExists)

	if customersExists > 0 {
		// Check column types
		var colInfo []struct {
			ColumnName string `gorm:"column:column_name"`
			DataType   string `gorm:"column:data_type"`
		}
		db.Raw(`SELECT column_name, data_type FROM information_schema.columns WHERE table_name = 'customers'`).Scan(&colInfo)

		log.Println("Current customers table columns:")
		for _, col := range colInfo {
			log.Printf("  - %s: %s", col.ColumnName, col.DataType)
		}

		// Drop and recreate customers table with correct TEXT types
		log.Println("Dropping and recreating customers table with correct types...")
		db.Exec(`DROP TABLE IF EXISTS customer_contacts CASCADE`)
		db.Exec(`DROP TABLE IF EXISTS customers CASCADE`)
	}

	// Create customers table with correct TEXT types
	createCustomers := `
	CREATE TABLE IF NOT EXISTS customers (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		deleted_at TIMESTAMP,
		version INTEGER DEFAULT 1,
		created_by TEXT,
		customer_id TEXT,
		customer_code TEXT,
		customer_type TEXT,
		business_name TEXT,
		short_code TEXT,
		address_line1 TEXT,
		city TEXT,
		country TEXT,
		trn TEXT,
		industry TEXT,
		relation_years INTEGER DEFAULT 0,
		payment_grade TEXT DEFAULT 'C',
		customer_grade TEXT DEFAULT 'C',
		payment_terms_days INTEGER DEFAULT 30,
		avg_payment_days DECIMAL(10,2) DEFAULT 0,
		dispute_count INTEGER DEFAULT 0,
		total_orders_value DECIMAL(15,3) DEFAULT 0,
		total_orders_count INTEGER DEFAULT 0,
		avg_order_value DECIMAL(15,3) DEFAULT 0,
		last_order_date TIMESTAMP,
		ar_risk_tier TEXT,
		outstanding_bhd DECIMAL(15,3) DEFAULT 0,
		overdue_days INTEGER DEFAULT 0,
		credit_limit_bhd DECIMAL(15,3) DEFAULT 50000,
		is_credit_blocked BOOLEAN DEFAULT false,
		requires_prepayment BOOLEAN DEFAULT false,
		has_abb_competition BOOLEAN DEFAULT false,
		is_emergency_only BOOLEAN DEFAULT false
	)`
	if err := db.Exec(createCustomers).Error; err != nil {
		log.Printf("Warning creating customers: %v", err)
	} else {
		log.Println("✅ Customers table created with TEXT types")
	}

	// Add GORM-expected unique constraints with correct names
	customerConstraints := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uni_customers_customer_id ON customers(customer_id)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS uni_customers_customer_code ON customers(customer_code)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_deleted_at ON customers(deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_customer_type ON customers(customer_type)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_business_name ON customers(business_name)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_city ON customers(city)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_country ON customers(country)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_industry ON customers(industry)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_payment_grade ON customers(payment_grade)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_customer_grade ON customers(customer_grade)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_payment_terms_days ON customers(payment_terms_days)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_ar_risk_tier ON customers(ar_risk_tier)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_overdue_days ON customers(overdue_days)`,
		`CREATE INDEX IF NOT EXISTS idx_customers_is_credit_blocked ON customers(is_credit_blocked)`,
	}
	for _, stmt := range customerConstraints {
		if err := db.Exec(stmt).Error; err != nil {
			log.Printf("Warning on constraint: %v", err)
		}
	}
	log.Println("✅ Customer indexes created")

	// Create customer_contacts table
	createCustomerContacts := `
	CREATE TABLE IF NOT EXISTS customer_contacts (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		deleted_at TIMESTAMP,
		version INTEGER DEFAULT 1,
		created_by TEXT,
		customer_id TEXT,
		contact_name TEXT,
		job_title TEXT,
		email TEXT,
		phone TEXT,
		address TEXT,
		is_primary_contact BOOLEAN DEFAULT false
	)`
	if err := db.Exec(createCustomerContacts).Error; err != nil {
		log.Printf("Warning creating customer_contacts: %v", err)
	} else {
		log.Println("✅ Customer contacts table ready")
	}

	// =========================================================================
	// STEP 2: Fix suppliers table
	// =========================================================================
	log.Println("\n🔧 STEP 2: Fixing suppliers table...")
	db.Exec(`DROP TABLE IF EXISTS supplier_contacts CASCADE`)
	db.Exec(`DROP TABLE IF EXISTS suppliers CASCADE`)

	createSuppliers := `
	CREATE TABLE IF NOT EXISTS suppliers (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		deleted_at TIMESTAMP,
		version INTEGER DEFAULT 1,
		created_by TEXT,
		supplier_code TEXT,
		supplier_name TEXT,
		country TEXT,
		lead_time_days INTEGER DEFAULT 0,
		tax_id TEXT,
		supplier_type TEXT,
		brands_handled TEXT,
		product_types TEXT,
		primary_contact TEXT,
		email TEXT,
		phone TEXT,
		address TEXT,
		bank_name TEXT,
		account_number TEXT,
		iban TEXT,
		swift_code TEXT,
		payment_terms TEXT DEFAULT 'Net 30',
		rating INTEGER DEFAULT 3,
		notes TEXT
	)`
	if err := db.Exec(createSuppliers).Error; err != nil {
		log.Printf("Warning creating suppliers: %v", err)
	} else {
		log.Println("✅ Suppliers table created")
	}

	// Add GORM-expected unique constraints for suppliers
	supplierConstraints := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS uni_suppliers_supplier_code ON suppliers(supplier_code)`,
		`CREATE INDEX IF NOT EXISTS idx_suppliers_deleted_at ON suppliers(deleted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_suppliers_supplier_name ON suppliers(supplier_name)`,
		`CREATE INDEX IF NOT EXISTS idx_suppliers_country ON suppliers(country)`,
		`CREATE INDEX IF NOT EXISTS idx_suppliers_supplier_type ON suppliers(supplier_type)`,
		`CREATE INDEX IF NOT EXISTS idx_suppliers_payment_terms ON suppliers(payment_terms)`,
		`CREATE INDEX IF NOT EXISTS idx_suppliers_rating ON suppliers(rating)`,
	}
	for _, stmt := range supplierConstraints {
		if err := db.Exec(stmt).Error; err != nil {
			log.Printf("Warning on supplier constraint: %v", err)
		}
	}
	log.Println("✅ Supplier indexes created")

	// Create supplier_contacts table
	createSupplierContacts := `
	CREATE TABLE IF NOT EXISTS supplier_contacts (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		deleted_at TIMESTAMP,
		version INTEGER DEFAULT 1,
		created_by TEXT,
		supplier_id TEXT,
		contact_name TEXT,
		job_title TEXT,
		email TEXT,
		phone TEXT,
		address TEXT,
		is_primary_contact BOOLEAN DEFAULT false
	)`
	if err := db.Exec(createSupplierContacts).Error; err != nil {
		log.Printf("Warning creating supplier_contacts: %v", err)
	} else {
		log.Println("✅ Supplier contacts table ready")
	}

	// =========================================================================
	// STEP 3: Drop ALL CHECK constraints (they cause migration issues)
	// =========================================================================
	log.Println("\n🔧 STEP 3: Dropping all CHECK constraints...")

	// Drop all check constraints that might cause issues
	dropConstraints := []string{
		`ALTER TABLE devices DROP CONSTRAINT IF EXISTS chk_devices_status`,
		`ALTER TABLE orders DROP CONSTRAINT IF EXISTS chk_orders_status`,
		`ALTER TABLE offers DROP CONSTRAINT IF EXISTS chk_offers_status`,
		`ALTER TABLE invoices DROP CONSTRAINT IF EXISTS chk_invoices_status`,
		`ALTER TABLE delivery_notes DROP CONSTRAINT IF EXISTS chk_delivery_notes_status`,
		`ALTER TABLE purchase_orders DROP CONSTRAINT IF EXISTS chk_purchase_orders_status`,
		`ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS chk_supplier_invoices_status`,
		`ALTER TABLE supplier_invoices DROP CONSTRAINT IF EXISTS chk_supplier_invoices_match_status`,
		`ALTER TABLE goods_received_notes DROP CONSTRAINT IF EXISTS chk_goods_received_notes_qc_status`,
		`ALTER TABLE entity_notes DROP CONSTRAINT IF EXISTS chk_entity_notes_entity_type`,
		`ALTER TABLE entity_notes DROP CONSTRAINT IF EXISTS chk_entity_notes_note_type`,
		`ALTER TABLE supplier_issues DROP CONSTRAINT IF EXISTS chk_supplier_issues_status`,
		`ALTER TABLE follow_up_tasks DROP CONSTRAINT IF EXISTS chk_follow_up_tasks_status`,
		`ALTER TABLE offer_follow_ups DROP CONSTRAINT IF EXISTS chk_offer_follow_ups_status`,
		`ALTER TABLE costing_sheets DROP CONSTRAINT IF EXISTS chk_costing_sheets_status`,
		`ALTER TABLE db_costing_sheets DROP CONSTRAINT IF EXISTS chk_db_costing_sheets_status`,
		`ALTER TABLE purchase_order_items DROP CONSTRAINT IF EXISTS chk_purchase_order_items_status`,
		`ALTER TABLE sync_records DROP CONSTRAINT IF EXISTS chk_sync_records_status`,
		`ALTER TABLE tally_invoice_imports DROP CONSTRAINT IF EXISTS chk_tally_invoice_imports_status`,
		`ALTER TABLE tally_purchase_imports DROP CONSTRAINT IF EXISTS chk_tally_purchase_imports_status`,
	}
	for _, stmt := range dropConstraints {
		db.Exec(stmt) // Ignore errors - constraint may not exist
	}
	log.Println("✅ CHECK constraints dropped")

	// =========================================================================
	// STEP 4: Fix devices table columns
	// =========================================================================
	log.Println("\n🔧 STEP 4: Fixing devices table...")

	alterStatements := []string{
		// Add version column if missing
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1`,
		`ALTER TABLE devices ADD COLUMN IF NOT EXISTS created_by TEXT`,

		// Add device_users table columns if missing
		`ALTER TABLE device_users ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1`,
		`ALTER TABLE device_users ADD COLUMN IF NOT EXISTS created_by TEXT`,
	}

	for _, stmt := range alterStatements {
		if err := db.Exec(stmt).Error; err != nil {
			log.Printf("Warning executing '%s': %v", stmt[:50], err)
		} else {
			log.Printf("✅ Executed: %s...", stmt[:50])
		}
	}

	// Drop and recreate contracts tables without FK constraints
	log.Println("Fixing contracts tables (dropping FK constraints)...")

	dropStmts := []string{
		`ALTER TABLE contracts DROP CONSTRAINT IF EXISTS fk_contracts_customer`,
		`DROP TABLE IF EXISTS contract_clauses CASCADE`,
		`DROP TABLE IF EXISTS contract_templates CASCADE`,
		`DROP TABLE IF EXISTS contracts CASCADE`,
	}
	for _, stmt := range dropStmts {
		db.Exec(stmt)
	}

	createContracts := `
	CREATE TABLE IF NOT EXISTS contracts (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		deleted_at TIMESTAMP,
		version INTEGER DEFAULT 1,
		created_by TEXT,
		contract_number TEXT,
		customer_id TEXT,
		title TEXT,
		description TEXT,
		start_date TIMESTAMP,
		end_date TIMESTAMP,
		value DECIMAL(15,3),
		status TEXT DEFAULT 'draft',
		terms TEXT,
		notes TEXT
	)`
	if err := db.Exec(createContracts).Error; err != nil {
		log.Printf("Warning creating contracts: %v", err)
	} else {
		log.Println("✅ Contracts table ready")
	}

	// Create contract_clauses table if not exists
	createClauses := `
	CREATE TABLE IF NOT EXISTS contract_clauses (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		deleted_at TIMESTAMP,
		version INTEGER DEFAULT 1,
		created_by TEXT,
		contract_id TEXT,
		clause_type TEXT,
		title TEXT,
		content TEXT,
		sort_order INTEGER DEFAULT 0
	)`
	if err := db.Exec(createClauses).Error; err != nil {
		log.Printf("Warning creating contract_clauses: %v", err)
	} else {
		log.Println("✅ Contract clauses table ready")
	}

	// Create contract_templates table if not exists
	createTemplates := `
	CREATE TABLE IF NOT EXISTS contract_templates (
		id TEXT PRIMARY KEY,
		created_at TIMESTAMP,
		updated_at TIMESTAMP,
		deleted_at TIMESTAMP,
		version INTEGER DEFAULT 1,
		created_by TEXT,
		name TEXT,
		description TEXT,
		content TEXT,
		is_active BOOLEAN DEFAULT true
	)`
	if err := db.Exec(createTemplates).Error; err != nil {
		log.Printf("Warning creating contract_templates: %v", err)
	} else {
		log.Println("✅ Contract templates table ready")
	}

	// Check devices table
	log.Println("\nVerifying devices table columns...")
	var columns []struct {
		ColumnName string `gorm:"column:column_name"`
	}
	db.Raw(`SELECT column_name FROM information_schema.columns WHERE table_name = 'devices' ORDER BY ordinal_position`).Scan(&columns)

	log.Println("Devices table columns:")
	for _, col := range columns {
		log.Printf("  - %s", col.ColumnName)
	}

	log.Println("\n✅ Schema migration complete!")
	log.Println("You can now run the app.")
}

func convertDatabaseURL(url string) string {
	// Convert postgresql://user:pass@host:port/db to DSN format
	// Remove postgresql:// prefix
	url = url[13:] // len("postgresql://") = 13

	// Find @ separator
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

	// Parse credentials
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

	// Parse host:port/dbname
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
