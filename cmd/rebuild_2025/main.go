package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// =============================================================================
// MAIN ORCHESTRATOR - PH HOLDINGS 2025 DATABASE REBUILD
// =============================================================================
//
// This program rebuilds the PH Holdings database with 2025 data only.
//
// Execution Order:
//  1. Load .env file (DATABASE_URL)
//  2. Connect to PostgreSQL using convertDatabaseURL()
//  3. Drop all transactional tables (preserves devices, device_users, roles)
//  4. Run GORM AutoMigrate for all 70+ models
//  5. Seed lookup tables (roles, warehouse, chart of accounts)
//  6. Import master data (customers, suppliers) - PLACEHOLDER
//  7. Import 2025 invoices - PLACEHOLDER
//  8. Import 2025 purchases - PLACEHOLDER
//  9. Import 2025 opportunities (archaeology) - PLACEHOLDER
// 10. Run validation - PLACEHOLDER
// 11. Print summary report
//
// =============================================================================

func main() {
	startTime := time.Now()

	log.Println("╔══════════════════════════════════════════════════════════════╗")
	log.Println("║     PH HOLDINGS 2025 DATABASE REBUILD                       ║")
	log.Println("║     Clean slate: 2025 data only                             ║")
	log.Println("╚══════════════════════════════════════════════════════════════╝")
	log.Println()

	// =============================================================================
	// STEP 1: LOAD ENVIRONMENT VARIABLES
	// =============================================================================
	log.Println("🔧 STEP 1: Loading .env file...")

	// Try multiple .env locations (project root, cmd/rebuild_2025 relative, current dir)
	envPaths := []string{".env", "../../.env", "../.env"}
	envLoaded := false
	for _, envPath := range envPaths {
		if err := godotenv.Load(envPath); err == nil {
			log.Printf("✅ Environment variables loaded from %s", envPath)
			envLoaded = true
			break
		}
	}
	if !envLoaded {
		log.Println("⚠️  Warning: Could not load .env file from any location")
		log.Println("   Continuing with system environment variables...")
	}

	// =============================================================================
	// STEP 2: CONNECT TO POSTGRESQL
	// =============================================================================
	log.Println("\n🔌 STEP 2: Connecting to PostgreSQL...")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("❌ DATABASE_URL not set in environment")
	}

	// Convert Render/Heroku/Railway URL format to GORM DSN
	dsn := convertDatabaseURL(databaseURL)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}

	log.Println("✅ Connected to PostgreSQL")

	// =============================================================================
	// STEP 3: DROP ALL TABLES (FK-SAFE ORDER)
	// =============================================================================
	log.Println("\n🗑️  STEP 3: Dropping all tables (preserving devices/device_users/roles)...")

	dropAllTables(db)

	log.Println("✅ Tables dropped successfully")

	// =============================================================================
	// STEP 4: AUTOMIGRATE ALL MODELS
	// =============================================================================
	log.Println("\n🏗️  STEP 4: Running GORM AutoMigrate for all models...")

	if err := migrateAllModels(db); err != nil {
		log.Fatalf("❌ AutoMigrate failed: %v", err)
	}

	log.Println("✅ All tables created successfully")

	// =============================================================================
	// STEP 5: SEED LOOKUP TABLES
	// =============================================================================
	log.Println("\n🌱 STEP 5: Seeding lookup tables...")

	if err := seedLookupTables(db); err != nil {
		log.Fatalf("❌ Seeding failed: %v", err)
	}

	log.Println("✅ Lookup tables seeded successfully")

	// =============================================================================
	// STEP 6: IMPORT MASTER DATA
	// =============================================================================
	log.Println("\n📦 STEP 6: Importing master data (customers, suppliers)...")

	customerMap, supplierMap, err := importMasterData(db)
	if err != nil {
		log.Fatalf("❌ Master data import failed: %v", err)
	}

	log.Printf("✅ Imported %d customers, %d suppliers", len(customerMap), len(supplierMap))

	// =============================================================================
	// STEP 7: IMPORT 2025 INVOICES
	// =============================================================================
	log.Println("\n📄 STEP 7: Importing 2025 invoices...")

	if err := import2025Invoices(db, customerMap); err != nil {
		log.Fatalf("❌ Invoice import failed: %v", err)
	}

	log.Println("✅ Invoices imported successfully")

	// =============================================================================
	// STEP 8: IMPORT 2025 PURCHASES
	// =============================================================================
	log.Println("\n🛒 STEP 8: Importing 2025 purchases...")

	if err := import2025Purchases(db, supplierMap); err != nil {
		log.Fatalf("❌ Purchase import failed: %v", err)
	}
	log.Println("✅ Purchases imported successfully")

	// Import supplier payments as well
	if err := import2025SupplierPayments(db, supplierMap); err != nil {
		log.Printf("⚠️  Supplier payments import warning: %v", err)
	} else {
		log.Println("✅ Supplier payments imported successfully")
	}

	// =============================================================================
	// STEP 9: IMPORT 2025 OPPORTUNITIES
	// =============================================================================
	log.Println("\n🎯 STEP 9: Importing 2025 opportunities (archaeology data)...")

	if err := import2025Opportunities(db, customerMap); err != nil {
		log.Printf("⚠️  Opportunities import warning: %v", err)
	} else {
		log.Println("✅ Opportunities imported successfully")
	}

	// =============================================================================
	// STEP 10: RUN VALIDATION
	// =============================================================================
	log.Println("\n✔️  STEP 10: Running validation...")

	if err := runValidation(db); err != nil {
		log.Printf("⚠️  Validation warnings: %v", err)
	} else {
		log.Println("✅ All validation checks passed")
	}

	// =============================================================================
	// STEP 11: PRINT SUMMARY REPORT
	// =============================================================================
	log.Println("\n📊 STEP 11: Generating summary report...")

	printSummary(db)

	// =============================================================================
	// COMPLETION
	// =============================================================================
	elapsed := time.Since(startTime)

	log.Println("\n╔══════════════════════════════════════════════════════════════╗")
	log.Println("║     REBUILD COMPLETE                                         ║")
	log.Printf("║     Total time: %-45s ║\n", elapsed.Round(time.Millisecond))
	log.Println("╚══════════════════════════════════════════════════════════════╝")

	log.Println("\n✨ All steps completed! The database has been rebuilt with 2025 data.")
	log.Println("   Run 'wails dev' to start the application with the fresh database.")
}

// =============================================================================
// AUTOMIGRATE ALL MODELS
// =============================================================================

func migrateAllModels(db *gorm.DB) error {
	log.Println("   Creating tables for 70+ models...")

	err := db.AutoMigrate(
		// CRM & Master Data (9 models)
		&CustomerMaster{},
		&SupplierMaster{},
		&CustomerContact{},
		&SupplierContact{},
		&ProductMaster{},
		&EntityNote{},
		&SupplierIssue{},

		// Sales & RFQ (4 models)
		&Offer{},
		&OfferItem{},
		&Opportunity{},
		&ARAgingBucket{},

		// Operations (9 models)
		&Order{},
		&OrderItem{},
		&DeliveryNote{},
		&DeliveryNoteItem{},
		&Shipment{},
		&PostSaleNote{},

		// Costing (3 models)
		&DBCostingSheet{},
		&DBCostingItem{},
		&DBCostingAdditionalCost{},

		// Finance (8 models)
		&Invoice{},
		&DBInvoiceItem{},
		&Payment{},
		&ChartOfAccount{},
		&JournalEntry{},
		&JournalLine{},
		&VATReturn{},

		// Inventory (5 models)
		&InventoryItem{},
		&StockMovement{},
		&StockAdjustment{},
		&Warehouse{},

		// Intelligence (9 models)
		&PredictionRecord{},
		&WinProbabilityPrediction{},
		&DiscountRecommendationRecord{},
		&CustomerSnapshot{},
		&ActualOutcome{},
		&CostingHistory{},
		&GradeChange{},
		&PaymentPredictionAccuracy{},

		// Admin (12 models)
		&Role{},
		&User{},
		&Device{},
		&DeviceUser{},
		&Alert{},
		&FollowUpTask{},
		&AuditLog{},
		&FileWatchEvent{},
		&SyncStatus{},
		&Job{},
		&Setting{},
		&UserSession{},
		&QuickCapture{},

		// Procurement (7 models)
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&GoodsReceivedNote{},
		&GRNItem{},
		&SupplierInvoice{},
		&SupplierInvoiceItem{},
		&SupplierPayment{},

		// Chat (2 models)
		&Conversation{},
		&ChatMessage{},

		// Follow-ups (1 model)
		&OfferFollowUp{},

		// Sync & Import (3 models)
		&SyncRecord{},
		&TallyInvoiceImport{},
		&TallyPurchaseImport{},

		// Accounting (5 models)
		&AccountMapping{},
		&FiscalPeriod{},
		&BankAccount{},
		&BankStatement{},
		&BankStatementLine{},
	)

	if err != nil {
		return fmt.Errorf("AutoMigrate failed: %w", err)
	}

	log.Println("   ✅ AutoMigrate completed (70+ tables)")
	return nil
}

// =============================================================================
// PRINT SUMMARY REPORT
// =============================================================================

func printSummary(db *gorm.DB) {
	log.Println("\n═══════════════════════════════════════════════════════════════")
	log.Println("                    DATABASE SUMMARY REPORT")
	log.Println("═══════════════════════════════════════════════════════════════")

	// Define tables to count
	tables := []struct {
		name  string
		model interface{}
	}{
		{"Customers", &CustomerMaster{}},
		{"Suppliers", &SupplierMaster{}},
		{"Products", &ProductMaster{}},
		{"Roles", &Role{}},
		{"Warehouses", &Warehouse{}},
		{"Chart of Accounts", &ChartOfAccount{}},
		{"Orders", &Order{}},
		{"Invoices", &Invoice{}},
		{"Payments", &Payment{}},
		{"Purchase Orders", &PurchaseOrder{}},
		{"Supplier Invoices", &SupplierInvoice{}},
		{"Supplier Payments", &SupplierPayment{}},
		{"GRNs", &GoodsReceivedNote{}},
		{"Delivery Notes", &DeliveryNote{}},
		{"Offers", &Offer{}},
		{"Opportunities", &Opportunity{}},
		{"Devices", &Device{}},
		{"Device Users", &DeviceUser{}},
	}

	log.Println("\n📋 MASTER DATA:")
	log.Println("───────────────────────────────────────────────────────────────")
	for _, table := range tables[:6] {
		var count int64
		db.Model(table.model).Count(&count)
		log.Printf("   %-25s %d", table.name+":", count)
	}

	log.Println("\n📦 TRANSACTIONS:")
	log.Println("───────────────────────────────────────────────────────────────")
	for _, table := range tables[6:16] {
		var count int64
		db.Model(table.model).Count(&count)
		log.Printf("   %-25s %d", table.name+":", count)
	}

	log.Println("\n👤 AUTHENTICATION:")
	log.Println("───────────────────────────────────────────────────────────────")
	for _, table := range tables[16:] {
		var count int64
		db.Model(table.model).Count(&count)
		log.Printf("   %-25s %d", table.name+":", count)
	}

	// Calculate financial metrics (if data exists)
	log.Println("\n💰 FINANCIAL METRICS (2025):")
	log.Println("───────────────────────────────────────────────────────────────")

	var totalRevenue float64
	db.Model(&Invoice{}).Select("COALESCE(SUM(net_amount), 0)").Scan(&totalRevenue)
	log.Printf("   Total Revenue:            BHD %.3f", totalRevenue)

	var totalPurchases float64
	db.Model(&SupplierInvoice{}).Select("COALESCE(SUM(net_amount), 0)").Scan(&totalPurchases)
	log.Printf("   Total Purchases:          BHD %.3f", totalPurchases)

	var grossProfit float64
	if totalRevenue > 0 && totalPurchases > 0 {
		grossProfit = totalRevenue - totalPurchases
		grossMargin := (grossProfit / totalRevenue) * 100
		log.Printf("   Gross Profit:             BHD %.3f", grossProfit)
		log.Printf("   Gross Margin:             %.1f%%", grossMargin)
	} else {
		log.Println("   Gross Profit:             BHD 0.000 (no data)")
		log.Println("   Gross Margin:             0.0% (no data)")
	}

	var arOutstanding float64
	db.Model(&Invoice{}).Where("payment_status != ?", "Paid").Select("COALESCE(SUM(amount_due), 0)").Scan(&arOutstanding)
	log.Printf("   AR Outstanding:           BHD %.3f", arOutstanding)

	var apOutstanding float64
	db.Model(&SupplierInvoice{}).Where("payment_status != ?", "Paid").Select("COALESCE(SUM(amount_due), 0)").Scan(&apOutstanding)
	log.Printf("   AP Outstanding:           BHD %.3f", apOutstanding)

	log.Println("\n═══════════════════════════════════════════════════════════════")
	log.Println()
}
