package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type PredictionRecord struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	CustomerID    string    `gorm:"index" json:"customer_id"`
	CustomerName  string    `json:"customer_name"`
	Grade         string    `gorm:"index" json:"grade"`
	PredictedDays int       `json:"predicted_days"`
	Confidence    float64   `json:"confidence"`
	R1            float64   `json:"r1"`
	R2            float64   `json:"r2"`
	R3            float64   `json:"r3"`
	CreatedAt     time.Time `gorm:"index" json:"created_at"`
}

func main() {
	// Open database
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Auto-migrate schema
	db.AutoMigrate(&PredictionRecord{})

	// Check existing count
	var count int64
	db.Model(&PredictionRecord{}).Count(&count)
	fmt.Printf("Existing prediction records: %d\n", count)

	if count > 0 {
		fmt.Println("Database already has predictions. Skipping seed.")

		// Show sample records
		var records []PredictionRecord
		db.Order("created_at desc").Limit(5).Find(&records)
		fmt.Println("\nSample records:")
		for _, r := range records {
			fmt.Printf("  %s: %s (Grade %s, %d days, %.0f%% confidence)\n",
				r.CustomerName, r.CreatedAt.Format("2006-01-02"), r.Grade,
				r.PredictedDays, r.Confidence*100)
		}
		return
	}

	// Seed demo data
	fmt.Println("Seeding demo prediction data...")

	customers := []string{
		"Acme Industries",
		"Steel Supply Co.",
		"Logistics Partners",
		"Construction Materials LLC",
		"Industrial Equipment Inc.",
		"Manufacturing Solutions",
		"Supply Chain Partners",
		"Trade Alliance Group",
	}

	rand.Seed(time.Now().UnixNano())

	for i := 0; i < 20; i++ {
		customer := customers[rand.Intn(len(customers))]

		// Generate realistic payment prediction
		r1 := 0.2 + rand.Float64()*0.2  // 20-40% exploration
		r2 := 0.15 + rand.Float64()*0.15 // 15-30% optimization
		r3 := 1.0 - r1 - r2               // Remaining stabilization

		// Determine grade based on regime distribution
		var grade string
		var predictedDays int
		var confidence float64

		if r2 > 0.25 { // High optimization = good flow
			grade = "A"
			predictedDays = 15 + rand.Intn(15) // 15-30 days
			confidence = 0.85 + rand.Float64()*0.14
		} else if r1 < 0.25 { // Low exploration = stable
			grade = "B"
			predictedDays = 30 + rand.Intn(20) // 30-50 days
			confidence = 0.75 + rand.Float64()*0.15
		} else if r1 > 0.35 { // High exploration = chaotic
			grade = "D"
			predictedDays = 60 + rand.Intn(30) // 60-90 days
			confidence = 0.50 + rand.Float64()*0.20
		} else {
			grade = "C"
			predictedDays = 40 + rand.Intn(20) // 40-60 days
			confidence = 0.65 + rand.Float64()*0.20
		}

		record := PredictionRecord{
			CustomerID:    fmt.Sprintf("CUST-%03d", rand.Intn(100)),
			CustomerName:  customer,
			Grade:         grade,
			PredictedDays: predictedDays,
			Confidence:    confidence,
			R1:            r1,
			R2:            r2,
			R3:            r3,
			CreatedAt:     time.Now().Add(-time.Duration(i) * 24 * time.Hour),
		}

		if err := db.Create(&record).Error; err != nil {
			log.Printf("Failed to create record: %v", err)
		} else {
			fmt.Printf("  Created: %s - Grade %s\n", customer, grade)
		}
	}

	fmt.Println("\nDemo data seeded successfully!")
}
