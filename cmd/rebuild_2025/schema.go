package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// =============================================================================
// NOTE: Model definitions are in models.go
// This file only contains seeding and table management functions
// =============================================================================

// =============================================================================
// TABLE DROPPING (FK-SAFE ORDER: LEAF → PARENT)
// =============================================================================

// dropAllTables drops all transactional tables in FK-safe order while
// preserving devices, device_users, and roles tables.
//
// Drop order follows dependency tree:
// Tier 1 (leaf tables) → Tier 2 (middle) → Tier 3 (parent) → Tier 4 (master)
func dropAllTables(db *gorm.DB) {
	log.Println("Dropping all tables (FK-safe order)...")

	// TIER 1: Leaf tables (no foreign keys pointing TO them, only FROM them)
	tier1 := []string{
		"chat_messages",
		"grn_items",
		"invoice_items",
		"payment_prediction_accuracy",
		"grade_changes",
		"costing_additional_costs",
		"costing_items",
		"delivery_note_items",
		"purchase_order_items",
		"supplier_invoice_items",
		"bank_statement_lines",
		"journal_lines",
		"offer_items",
		"order_items",
	}

	// TIER 2: Middle tier (depend on tier 3, have tier 1 depending on them)
	tier2 := []string{
		"payments",
		"supplier_payments",
		"offer_follow_ups",
		"bank_statements",
		"goods_received_notes",
		"delivery_notes",
		"supplier_invoices",
		"journal_entries",
	}

	// TIER 3: Parent tier (depend on master data, have many children)
	tier3 := []string{
		"invoices",
		"orders",
		"offers",
		"purchase_orders",
		"opportunities",
		"costing_sheets",
		"conversations",
		"bank_accounts",
	}

	// TIER 4: Master data (EXCEPT devices, device_users, roles which we preserve)
	tier4 := []string{
		"customers",
		"suppliers",
		"products",
		"warehouses",
		"chart_of_accounts",
	}

	// TIER 5: Supporting tables (no FK dependencies, can drop anytime)
	tier5 := []string{
		"tally_invoice_imports",
		"tally_purchase_imports",
		"sync_records",
		"entity_notes",
		"supplier_issues",
		"customer_contacts",
		"supplier_contacts",
		"alerts",
		"settings",
		"ar_aging",
		"prediction_records",
		"quick_captures",
		"shipments",
		"post_sale_notes",
		"followup_tasks",
		"audit_logs",
		"file_watch_events",
		"sync_status",
		"jobs",
		"fiscal_periods",
		"account_mappings",
		"user_sessions",
		"stock_movements",
		"stock_adjustments",
		"inventory_items",
		"win_probability_predictions",
		"discount_recommendations",
		"customer_snapshots",
		"actual_outcomes",
		"costing_history",
		"vat_returns",
	}

	// Drop in reverse dependency order
	allTables := append(tier1, tier2...)
	allTables = append(allTables, tier3...)
	allTables = append(allTables, tier4...)
	allTables = append(allTables, tier5...)

	dropped := 0
	for _, table := range allTables {
		result := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS %s CASCADE", table))
		if result.Error != nil {
			log.Printf("Warning: Could not drop table %s: %v", table, result.Error)
		} else {
			dropped++
		}
	}

	log.Printf("Dropped %d tables (preserved devices, device_users, roles)", dropped)
}

// =============================================================================
// LOOKUP TABLE SEEDING
// =============================================================================

// seedLookupTables seeds essential lookup tables: Roles, Warehouse, Chart of Accounts
func seedLookupTables(db *gorm.DB) error {
	log.Println("Seeding lookup tables...")

	// 1. Seed 5 Roles
	if err := seedRoles(db); err != nil {
		return fmt.Errorf("failed to seed roles: %w", err)
	}

	// 2. Seed Main Warehouse
	if err := seedWarehouse(db); err != nil {
		return fmt.Errorf("failed to seed warehouse: %w", err)
	}

	// 3. Seed Chart of Accounts
	if err := seedChartOfAccounts(db); err != nil {
		return fmt.Errorf("failed to seed chart of accounts: %w", err)
	}

	log.Println("Lookup tables seeded successfully")
	return nil
}

// seedRoles creates 5 standard roles with appropriate permissions
func seedRoles(db *gorm.DB) error {
	roles := []Role{
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:        "admin",
			Permissions: `["*"]`,
			Description: "Full system access including device and user management",
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:        "manager",
			Permissions: `["read","write","finance","operations","crm","reports"]`,
			Description: "Manager access with full finance permissions",
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:        "sales",
			Permissions: `["read","write","crm","offers","orders"]`,
			Description: "Sales team - CRM and order management (no finance access)",
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:        "operations",
			Permissions: `["read","write","operations","grn","delivery"]`,
			Description: "Operations team - GRN and delivery (no finance access)",
		},
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			Name:        "staff",
			Permissions: `["read"]`,
			Description: "Read-only access across all modules",
		},
	}

	for _, role := range roles {
		// Check if role already exists
		var existing Role
		result := db.Where("name = ?", role.Name).First(&existing)
		if result.Error == nil {
			log.Printf("  Role already exists: %s (skipped)", role.Name)
			continue
		}

		// Create new role
		if err := db.Create(&role).Error; err != nil {
			return fmt.Errorf("failed to create role %s: %w", role.Name, err)
		}
		log.Printf("  Created role: %s (%s)", role.Name, role.Description)
	}

	return nil
}

// seedWarehouse creates the main warehouse in Sitra
func seedWarehouse(db *gorm.DB) error {
	// Check if warehouse already exists
	var existing Warehouse
	result := db.Where("code = ?", "WH-SITRA-01").First(&existing)
	if result.Error == nil {
		log.Printf("  Warehouse already exists: %s (skipped)", existing.Code)
		return nil
	}

	warehouse := Warehouse{
		Base:     Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		Code:     "WH-SITRA-01",
		Name:     "Main Warehouse - Sitra",
		IsActive: true,
	}

	if err := db.Create(&warehouse).Error; err != nil {
		return fmt.Errorf("failed to create warehouse: %w", err)
	}

	log.Printf("  Created warehouse: %s (%s)", warehouse.Code, warehouse.Name)
	return nil
}

// seedChartOfAccounts creates a simplified chart of accounts structure
func seedChartOfAccounts(db *gorm.DB) error {
	accounts := []ChartOfAccount{
		// 1000: Assets
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode: "1000",
			AccountName: "Assets",
			AccountType: "Asset",
			IsActive:    true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "1100",
			AccountName:   "Bank Accounts",
			AccountType:   "Asset",
			ParentAccount: "1000",
			IsActive:      true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "1200",
			AccountName:   "Accounts Receivable",
			AccountType:   "Asset",
			ParentAccount: "1000",
			IsActive:      true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "1300",
			AccountName:   "Inventory",
			AccountType:   "Asset",
			ParentAccount: "1000",
			IsActive:      true,
		},

		// 2000: Liabilities
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode: "2000",
			AccountName: "Liabilities",
			AccountType: "Liability",
			IsActive:    true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "2100",
			AccountName:   "Accounts Payable",
			AccountType:   "Liability",
			ParentAccount: "2000",
			IsActive:      true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "2200",
			AccountName:   "VAT Payable",
			AccountType:   "Liability",
			ParentAccount: "2000",
			IsActive:      true,
		},

		// 3000: Equity
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode: "3000",
			AccountName: "Equity",
			AccountType: "Equity",
			IsActive:    true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "3100",
			AccountName:   "Retained Earnings",
			AccountType:   "Equity",
			ParentAccount: "3000",
			IsActive:      true,
		},

		// 4000: Revenue
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode: "4000",
			AccountName: "Revenue",
			AccountType: "Revenue",
			IsActive:    true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "4100",
			AccountName:   "Sales - Instrumentation",
			AccountType:   "Revenue",
			ParentAccount: "4000",
			IsActive:      true,
		},

		// 5000: COGS
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode: "5000",
			AccountName: "Cost of Goods Sold",
			AccountType: "Expense",
			IsActive:    true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "5100",
			AccountName:   "Supplier Purchases",
			AccountType:   "Expense",
			ParentAccount: "5000",
			IsActive:      true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "5200",
			AccountName:   "Freight & Shipping",
			AccountType:   "Expense",
			ParentAccount: "5000",
			IsActive:      true,
		},

		// 6000: Operating Expenses
		{
			Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode: "6000",
			AccountName: "Operating Expenses",
			AccountType: "Expense",
			IsActive:    true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "6100",
			AccountName:   "Salaries & Wages",
			AccountType:   "Expense",
			ParentAccount: "6000",
			IsActive:      true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "6200",
			AccountName:   "Rent & Utilities",
			AccountType:   "Expense",
			ParentAccount: "6000",
			IsActive:      true,
		},
		{
			Base:          Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
			AccountCode:   "6300",
			AccountName:   "Marketing & Sales",
			AccountType:   "Expense",
			ParentAccount: "6000",
			IsActive:      true,
		},
	}

	createdCount := 0
	skippedCount := 0

	for _, account := range accounts {
		// Check if account already exists
		var existing ChartOfAccount
		result := db.Where("account_code = ?", account.AccountCode).First(&existing)
		if result.Error == nil {
			skippedCount++
			continue
		}

		// Create new account
		if err := db.Create(&account).Error; err != nil {
			return fmt.Errorf("failed to create account %s: %w", account.AccountCode, err)
		}
		createdCount++
	}

	if skippedCount > 0 {
		log.Printf("  Chart of accounts: %d created, %d already existed (skipped)", createdCount, skippedCount)
	} else {
		log.Printf("  Created %d chart of accounts entries", createdCount)
	}
	return nil
}

// =============================================================================
// DATABASE URL CONVERTER (Render/Heroku/Railway format)
// =============================================================================

// convertDatabaseURL converts postgres:// URLs to GORM DSN format
// Handles Render.com, Heroku, Railway automatic DATABASE_URL injection
func convertDatabaseURL(url string) string {
	// Handle postgresql:// alias
	url = strings.Replace(url, "postgresql://", "postgres://", 1)

	// If not postgres://, assume already DSN format
	if !strings.HasPrefix(url, "postgres://") {
		return url
	}

	// Remove prefix
	url = url[11:] // "postgres://" = 11 chars

	// Split credentials from host
	atIdx := strings.LastIndex(url, "@")
	if atIdx == -1 {
		return url // Invalid format, return as-is
	}

	credentials := url[:atIdx]
	hostPart := url[atIdx+1:]

	// Extract user:password
	colonIdx := strings.Index(credentials, ":")
	user := credentials
	password := ""
	if colonIdx != -1 {
		user = credentials[:colonIdx]
		password = credentials[colonIdx+1:]
	}

	// Extract host:port/dbname
	slashIdx := strings.Index(hostPart, "/")
	hostPort := hostPart
	dbname := "ph_holdings"
	if slashIdx != -1 {
		hostPort = hostPart[:slashIdx]
		dbname = hostPart[slashIdx+1:]
		// Remove query params if present
		if qIdx := strings.Index(dbname, "?"); qIdx != -1 {
			dbname = dbname[:qIdx]
		}
	}

	// Extract host and port
	host := hostPort
	port := "5432"
	if colonIdx := strings.Index(hostPort, ":"); colonIdx != -1 {
		host = hostPort[:colonIdx]
		port = hostPort[colonIdx+1:]
	}

	// Build GORM DSN
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		host, port, user, password, dbname)
}
