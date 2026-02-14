package main

import (
	"fmt"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Import all types from database.go (copy minimal definitions)
type PredictionRecord struct {
	ID            uint
	CustomerID    string
	CustomerName  string
	Grade         string
	PredictedDays int
	Confidence    float64
	R1, R2, R3    float64
}

func (PredictionRecord) TableName() string {
	return "prediction_records"
}

func main() {
	fmt.Println("=== Testing Database Migration ===")
	fmt.Println()

	// Connect to database
	db, err := gorm.Open(sqlite.Open("../ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	// Query table count
	var tables []string
	db.Raw("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name").Scan(&tables)

	fmt.Printf("Total Tables: %d\n\n", len(tables))
	fmt.Println("Tables in Database:")
	for i, table := range tables {
		var count int64
		db.Table(table).Count(&count)
		fmt.Printf("  %2d. %-30s (%d rows)\n", i+1, table, count)
	}

	fmt.Println("\n✓ Migration verification complete!")
}
