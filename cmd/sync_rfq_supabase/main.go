package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// RFQData model
type RFQData struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	Client         string    `json:"client"`
	Project        string    `json:"project"`
	Value          float64   `json:"value"`
	Notes          string    `json:"notes"`
	Status         string    `gorm:"default:'pending'" json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DocumentHash   string    `json:"document_hash"`
	VisitLocations string    `json:"visit_locations"`
	ProductDetails string    `json:"product_details"`
	SourceDocPath  string    `json:"source_doc_path"`
}

func (RFQData) TableName() string { return "rfq_data" }

// RFQComment model
type RFQComment struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RFQID     uint      `json:"rfq_id"`
	Comment   string    `json:"comment"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

func (RFQComment) TableName() string { return "rfq_comments" }

func main() {
	fmt.Println("========================================")
	fmt.Println("   RFQ Data -> Supabase Sync")
	fmt.Println("========================================")

	// Connect to SQLite
	sqliteDB, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("SQLite connection failed: %v", err)
	}
	fmt.Println("✅ SQLite connected")

	// Connect to Supabase
	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dsn := fmt.Sprintf("host=aws-1-ap-southeast-1.pooler.supabase.com port=5432 user=postgres.xkyydcxxkzjbnsrmzkpp password=%s dbname=postgres sslmode=require", supabasePassword)
	supabaseDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Supabase connection failed: %v", err)
	}
	fmt.Println("✅ Supabase connected")

	// Create tables if they don't exist
	fmt.Println("\nCreating tables...")
	supabaseDB.Exec(`
		CREATE TABLE IF NOT EXISTS rfq_data (
			id SERIAL PRIMARY KEY,
			client TEXT,
			project TEXT,
			value DECIMAL(15,3) DEFAULT 0,
			notes TEXT,
			status TEXT DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW(),
			document_hash TEXT,
			visit_locations TEXT,
			product_details TEXT,
			source_doc_path TEXT
		)
	`)
	supabaseDB.Exec(`
		CREATE TABLE IF NOT EXISTS rfq_comments (
			id SERIAL PRIMARY KEY,
			rfq_id INTEGER,
			comment TEXT,
			created_by TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		)
	`)
	fmt.Println("✅ Tables ready")

	// Clear existing data
	supabaseDB.Exec("TRUNCATE rfq_data CASCADE")
	supabaseDB.Exec("TRUNCATE rfq_comments CASCADE")

	// Sync RFQ data
	var rfqs []RFQData
	sqliteDB.Find(&rfqs)
	fmt.Printf("\nSyncing %d RFQs...\n", len(rfqs))

	for _, rfq := range rfqs {
		if err := supabaseDB.Create(&rfq).Error; err != nil {
			fmt.Printf("  ⚠️ Error syncing RFQ %d: %v\n", rfq.ID, err)
		}
	}
	fmt.Printf("✅ RFQs synced: %d\n", len(rfqs))

	// Sync RFQ comments
	var comments []RFQComment
	sqliteDB.Find(&comments)
	fmt.Printf("\nSyncing %d RFQ comments...\n", len(comments))
	for _, c := range comments {
		supabaseDB.Create(&c)
	}
	fmt.Printf("✅ Comments synced: %d\n", len(comments))

	// Verify
	var cloudCount int64
	supabaseDB.Model(&RFQData{}).Count(&cloudCount)
	fmt.Printf("\n========================================\n")
	fmt.Printf("  RFQ Sync Complete!\n")
	fmt.Printf("  Local: %d | Cloud: %d\n", len(rfqs), cloudCount)
	fmt.Printf("========================================\n")
}
