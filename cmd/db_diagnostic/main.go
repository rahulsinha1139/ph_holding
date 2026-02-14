package main

import (
	"fmt"
	"log"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Minimal schema to query
type PredictionRecord struct {
	ID            uint
	CustomerID    string
	CustomerName  string
	Grade         string
	PredictedDays int
	Confidence    float64
}

func main() {
	// Connect to database
	db, err := gorm.Open(sqlite.Open("../ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	fmt.Println("=== PH Holdings Database Diagnostic ===")
	fmt.Println()

	// Check prediction_records table
	var predCount int64
	db.Model(&PredictionRecord{}).Count(&predCount)
	fmt.Printf("Prediction Records: %d\n", predCount)

	if predCount > 0 {
		// Get sample records
		var records []PredictionRecord
		db.Order("id DESC").Limit(5).Find(&records)

		fmt.Println("\nRecent Predictions:")
		for _, r := range records {
			fmt.Printf("  ID %d: %s (Grade %s, %d days, %.1f%% confidence)\n",
				r.ID, r.CustomerName, r.Grade, r.PredictedDays, r.Confidence*100)
		}

		// Grade distribution
		var gradeStats []struct {
			Grade string
			Count int
		}
		db.Model(&PredictionRecord{}).
			Select("grade, COUNT(*) as count").
			Group("grade").
			Order("grade").
			Scan(&gradeStats)

		fmt.Println("\nGrade Distribution:")
		for _, stat := range gradeStats {
			fmt.Printf("  Grade %s: %d records\n", stat.Grade, stat.Count)
		}
	}

	// Check what other tables exist
	var tables []string
	db.Raw("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name").Scan(&tables)

	fmt.Println("\nAll Tables in Database:")
	for _, table := range tables {
		var count int64
		db.Table(table).Count(&count)
		fmt.Printf("  - %s (%d rows)\n", table, count)
	}

	fmt.Println("\n✓ Diagnostic complete!")
}
