package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CustomerMaster for restore
type CustomerMaster struct {
	ID                 string     `gorm:"primaryKey;size:36"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
	CustomerID         string
	CustomerCode       string
	CustomerType       string
	BusinessName       string
	ShortCode          string
	AddressLine1       string
	City               string
	Country            string
	TRN                string
	Industry           string
	RelationYears      int
	PaymentGrade       string
	CustomerGrade      string
	PaymentTermsDays   int
	AvgPaymentDays     float64
	DisputeCount       int
	TotalOrdersValue   float64
	TotalOrdersCount   int
	AvgOrderValue      float64
	LastOrderDate      *time.Time
	ARRiskTier         string
	OutstandingBHD     float64
	OverdueDays        int
	CreditLimitBHD     float64
	IsCreditBlocked    bool
	RequiresPrepayment bool
	HasABBCompetition  bool
	IsEmergencyOnly    bool
}

func (CustomerMaster) TableName() string { return "customers" }

// SupplierMaster for restore
type SupplierMaster struct {
	ID             string `gorm:"primaryKey;size:36"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
	SupplierCode   string
	SupplierName   string
	Country        string
	LeadTimeDays   int
	TaxID          string
	SupplierType   string
	BrandsHandled  string
	ProductTypes   string
	PrimaryContact string
	Email          string
	Phone          string
	Address        string
	BankName       string
	AccountNumber  string
	IBAN           string
	SwiftCode      string
	PaymentTerms   string
	Rating         int
	Notes          string
}

func (SupplierMaster) TableName() string { return "suppliers" }

func main() {
	// Load .env
	godotenv.Load()

	// Connect to SQLite (source)
	sqlitePath := `C:\Users\rishu\OneDrive\Desktop\asymm\production_shenanigans\asymmetrica-production\ph_holdings_sovereign_ui\build\bin\data\ph_holdings.db`
	sqliteDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to SQLite: %v", err)
	}
	log.Println("✅ Connected to SQLite (source)")

	// Connect to PostgreSQL (destination)
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}
	dsn := convertDatabaseURL(databaseURL)
	pgDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	log.Println("✅ Connected to PostgreSQL (destination)")

	// Restore Customers
	log.Println("\n📦 Restoring customers...")
	var customers []CustomerMaster
	if err := sqliteDB.Find(&customers).Error; err != nil {
		log.Fatalf("Failed to read customers: %v", err)
	}
	log.Printf("Found %d customers in SQLite", len(customers))

	restored := 0
	for _, c := range customers {
		// Use ON CONFLICT DO NOTHING to avoid duplicates
		result := pgDB.Exec(`
			INSERT INTO customers (id, created_at, updated_at, customer_id, customer_code, customer_type,
				business_name, short_code, address_line1, city, country, trn, industry, relation_years,
				payment_grade, customer_grade, payment_terms_days, avg_payment_days, dispute_count,
				total_orders_value, total_orders_count, avg_order_value, last_order_date, ar_risk_tier,
				outstanding_bhd, overdue_days, credit_limit_bhd, is_credit_blocked, requires_prepayment,
				has_abb_competition, is_emergency_only, version)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
			ON CONFLICT (id) DO NOTHING`,
			c.ID, c.CreatedAt, c.UpdatedAt, c.CustomerID, c.CustomerCode, c.CustomerType,
			c.BusinessName, c.ShortCode, c.AddressLine1, c.City, c.Country, c.TRN, c.Industry, c.RelationYears,
			c.PaymentGrade, c.CustomerGrade, c.PaymentTermsDays, c.AvgPaymentDays, c.DisputeCount,
			c.TotalOrdersValue, c.TotalOrdersCount, c.AvgOrderValue, c.LastOrderDate, c.ARRiskTier,
			c.OutstandingBHD, c.OverdueDays, c.CreditLimitBHD, c.IsCreditBlocked, c.RequiresPrepayment,
			c.HasABBCompetition, c.IsEmergencyOnly,
		)
		if result.Error != nil {
			log.Printf("⚠ Customer %s: %v", c.BusinessName, result.Error)
		} else if result.RowsAffected > 0 {
			restored++
		}
	}
	log.Printf("✅ Restored %d customers", restored)

	// Restore Suppliers
	log.Println("\n📦 Restoring suppliers...")
	var suppliers []SupplierMaster
	if err := sqliteDB.Find(&suppliers).Error; err != nil {
		log.Fatalf("Failed to read suppliers: %v", err)
	}
	log.Printf("Found %d suppliers in SQLite", len(suppliers))

	restored = 0
	for _, s := range suppliers {
		result := pgDB.Exec(`
			INSERT INTO suppliers (id, created_at, updated_at, supplier_code, supplier_name, country,
				lead_time_days, tax_id, supplier_type, brands_handled, product_types, primary_contact,
				email, phone, address, bank_name, account_number, iban, swift_code, payment_terms, rating, notes, version)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
			ON CONFLICT (id) DO NOTHING`,
			s.ID, s.CreatedAt, s.UpdatedAt, s.SupplierCode, s.SupplierName, s.Country,
			s.LeadTimeDays, s.TaxID, s.SupplierType, s.BrandsHandled, s.ProductTypes, s.PrimaryContact,
			s.Email, s.Phone, s.Address, s.BankName, s.AccountNumber, s.IBAN, s.SwiftCode, s.PaymentTerms, s.Rating, s.Notes,
		)
		if result.Error != nil {
			log.Printf("⚠ Supplier %s: %v", s.SupplierName, result.Error)
		} else if result.RowsAffected > 0 {
			restored++
		}
	}
	log.Printf("✅ Restored %d suppliers", restored)

	// Verify
	var custCount, suppCount int64
	pgDB.Table("customers").Count(&custCount)
	pgDB.Table("suppliers").Count(&suppCount)
	log.Printf("\n✅ PostgreSQL now has: %d customers, %d suppliers", custCount, suppCount)
}

func convertDatabaseURL(url string) string {
	url = url[13:] // Remove "postgresql://"
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
