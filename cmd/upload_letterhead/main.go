package main

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Supabase connection
const (
	SUPABASE_HOST = "aws-1-ap-southeast-1.pooler.supabase.com"
	SUPABASE_PORT = "5432"
	SUPABASE_USER = "postgres.xkyydcxxkzjbnsrmzkpp"
	SUPABASE_DB   = "postgres"
)

// Asset represents a binary asset stored in the database
type Asset struct {
	ID          string    `gorm:"primaryKey;size:50" json:"id"`
	Name        string    `gorm:"size:255;uniqueIndex" json:"name"`
	Description string    `gorm:"size:500" json:"description"`
	MimeType    string    `gorm:"size:100" json:"mime_type"`
	Data        string    `gorm:"type:text" json:"data"` // Base64 encoded
	Size        int64     `json:"size"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func main() {
	fmt.Println("========================================")
	fmt.Println("   Upload Letterhead to Supabase")
	fmt.Println("========================================\n")

	// Find letterhead file
	letterheadFile := "PH Trading Quotation Template.png"
	candidatePaths := []string{
		filepath.Join("CRITICAL_PH_CONTEXT_SSOT", letterheadFile),
		filepath.Join("..", "CRITICAL_PH_CONTEXT_SSOT", letterheadFile),
		filepath.Join("..", "..", "CRITICAL_PH_CONTEXT_SSOT", letterheadFile),
	}

	var letterheadPath string
	for _, p := range candidatePaths {
		if _, err := os.Stat(p); err == nil {
			abs, _ := filepath.Abs(p)
			letterheadPath = abs
			break
		}
	}

	if letterheadPath == "" {
		log.Fatal("Letterhead file not found in any location")
	}

	fmt.Printf("Found letterhead: %s\n\n", letterheadPath)

	// Read file
	data, err := ioutil.ReadFile(letterheadPath)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}
	fmt.Printf("File size: %d bytes\n", len(data))

	// Connect to Supabase
	fmt.Println("\nConnecting to Supabase...")
	supabasePassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if supabasePassword == "" {
		log.Fatal("SUPABASE_DB_PASSWORD environment variable not set")
	}
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=require",
		SUPABASE_HOST, SUPABASE_PORT, SUPABASE_USER, supabasePassword, SUPABASE_DB,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	fmt.Println("Connected!\n")

	// Ensure assets table exists
	fmt.Println("Creating/migrating assets table...")
	if err := db.AutoMigrate(&Asset{}); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}
	fmt.Println("Table ready.\n")

	// Base64 encode
	encoded := base64.StdEncoding.EncodeToString(data)

	// Create asset record
	asset := Asset{
		ID:          "letterhead",
		Name:        "letterhead",
		Description: "PH Trading letterhead template for PDF generation",
		MimeType:    "image/png",
		Data:        encoded,
		Size:        int64(len(data)),
	}

	// Check if exists
	var existing Asset
	result := db.Where("name = ?", "letterhead").First(&existing)
	if result.Error == nil {
		// Update existing
		fmt.Println("Updating existing letterhead asset...")
		if err := db.Model(&Asset{}).Where("name = ?", "letterhead").Updates(map[string]interface{}{
			"data":        encoded,
			"size":        int64(len(data)),
			"description": asset.Description,
			"mime_type":   asset.MimeType,
			"updated_at":  time.Now(),
		}).Error; err != nil {
			log.Fatalf("Update failed: %v", err)
		}
	} else {
		// Create new
		fmt.Println("Creating new letterhead asset...")
		if err := db.Create(&asset).Error; err != nil {
			log.Fatalf("Create failed: %v", err)
		}
	}

	// Verify
	var verify Asset
	db.Where("name = ?", "letterhead").First(&verify)
	fmt.Printf("\nVerification:\n")
	fmt.Printf("  ID: %s\n", verify.ID)
	fmt.Printf("  Name: %s\n", verify.Name)
	fmt.Printf("  Size: %d bytes\n", verify.Size)
	fmt.Printf("  MimeType: %s\n", verify.MimeType)
	fmt.Printf("  Data length: %d chars (base64)\n", len(verify.Data))

	fmt.Println("\n========================================")
	fmt.Println("   LETTERHEAD UPLOADED SUCCESSFULLY!")
	fmt.Println("========================================")
	fmt.Println("\nNew devices will now automatically download")
	fmt.Println("the letterhead during first-run sync.")
}
