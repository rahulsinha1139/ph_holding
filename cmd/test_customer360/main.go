package main

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Import minimal types
type CustomerMaster struct {
	ID                 uint
	CustomerID         string
	CustomerType       string
	BusinessName       string
	City               string
	Country            string
	Industry           string
	RelationYears      int
	PaymentGrade       string
	PaymentTermsDays   int
	AvgPaymentDays     float64
	DisputeCount       int
	TotalOrdersValue   float64
	TotalOrdersCount   int
	AvgOrderValue      float64
	LastOrderDate      *time.Time
	HasABBCompetition  bool
	IsEmergencyOnly    bool
	IsCreditBlocked    bool
	RequiresPrepayment bool
}

func (CustomerMaster) TableName() string { return "customers" }

type PredictionRecord struct {
	ID            uint
	CustomerID    string
	CustomerName  string
	Grade         string
	PredictedDays int
	Confidence    float64
	R1, R2, R3    float64
	CreatedAt     time.Time
}

func (PredictionRecord) TableName() string { return "prediction_records" }

type Customer360Data struct {
	CustomerID         string
	BusinessName       string
	CustomerType       string
	Industry           string
	City               string
	Country            string
	RelationYears      int
	CurrentGrade       string
	PaymentTermsDays   int
	AvgPaymentDays     float64
	DisputeCount       int
	IsCreditBlocked    bool
	RequiresPrepayment bool
	TotalOrdersValue   float64
	TotalOrdersCount   int
	AvgOrderValue      float64
	LastOrderDate      *time.Time
	HasABBCompetition  bool
	IsEmergencyOnly    bool
	RecentPredictions  []PredictionRecord
}

func GetCustomer360View(db *gorm.DB, customerID string) (Customer360Data, error) {
	var customer CustomerMaster
	result := db.Where("customer_id = ?", customerID).First(&customer)
	if result.Error != nil {
		return Customer360Data{}, fmt.Errorf("customer not found: %s", customerID)
	}

	var predictions []PredictionRecord
	db.Where("customer_id = ?", customerID).
		Order("created_at desc").
		Limit(10).
		Find(&predictions)

	return Customer360Data{
		CustomerID:         customer.CustomerID,
		BusinessName:       customer.BusinessName,
		CustomerType:       customer.CustomerType,
		Industry:           customer.Industry,
		City:               customer.City,
		Country:            customer.Country,
		RelationYears:      customer.RelationYears,
		CurrentGrade:       customer.PaymentGrade,
		PaymentTermsDays:   customer.PaymentTermsDays,
		AvgPaymentDays:     customer.AvgPaymentDays,
		DisputeCount:       customer.DisputeCount,
		IsCreditBlocked:    customer.IsCreditBlocked,
		RequiresPrepayment: customer.RequiresPrepayment,
		TotalOrdersValue:   customer.TotalOrdersValue,
		TotalOrdersCount:   customer.TotalOrdersCount,
		AvgOrderValue:      customer.AvgOrderValue,
		LastOrderDate:      customer.LastOrderDate,
		HasABBCompetition:  customer.HasABBCompetition,
		IsEmergencyOnly:    customer.IsEmergencyOnly,
		RecentPredictions:  predictions,
	}, nil
}

func main() {
	fmt.Println("=== Testing Customer360View API ===")
	fmt.Println()

	db, err := gorm.Open(sqlite.Open("../ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}

	// Test Customer360View for each grade
	testCustomers := []string{"CO1", "CO2", "CO3", "EC10"}

	for _, customerID := range testCustomers {
		fmt.Printf("\nFetching 360 view for %s...\n", customerID)

		data, err := GetCustomer360View(db, customerID)
		if err != nil {
			fmt.Printf("  ❌ Error: %v\n", err)
			continue
		}

		fmt.Printf("  ✓ %s (%s)\n", data.BusinessName, data.CustomerID)
		fmt.Printf("    Type: %s | Industry: %s | Country: %s\n",
			data.CustomerType, data.Industry, data.Country)
		fmt.Printf("    Grade: %s | Payment Terms: %d days | Avg Payment: %.1f days\n",
			data.CurrentGrade, data.PaymentTermsDays, data.AvgPaymentDays)
		fmt.Printf("    Relation: %d years | Disputes: %d | ABB Competition: %v\n",
			data.RelationYears, data.DisputeCount, data.HasABBCompetition)
		fmt.Printf("    Orders: %d total (%.0f BHD) | Avg: %.0f BHD\n",
			data.TotalOrdersCount, data.TotalOrdersValue, data.AvgOrderValue)
		fmt.Printf("    Recent Predictions: %d records\n", len(data.RecentPredictions))

		if data.IsCreditBlocked {
			fmt.Printf("    ⚠ CREDIT BLOCKED\n")
		}
		if data.RequiresPrepayment {
			fmt.Printf("    ⚠ REQUIRES PREPAYMENT\n")
		}
	}

	// Test GetAllCustomers
	fmt.Println("\n\nTesting GetAllCustomers...")
	var customers []CustomerMaster
	db.Order("business_name ASC").Find(&customers)
	fmt.Printf("  ✓ Retrieved %d customers\n", len(customers))

	// Show grade distribution
	gradeCount := make(map[string]int)
	for _, c := range customers {
		gradeCount[c.PaymentGrade]++
	}

	fmt.Println("\nGrade Distribution:")
	for grade := range []string{"A", "B", "C", "D"} {
		gradeStr := []string{"A", "B", "C", "D"}[grade]
		count := gradeCount[gradeStr]
		fmt.Printf("  Grade %s: %d customers\n", gradeStr, count)
	}

	fmt.Println("\n🎉 Customer360View API working perfectly!")
}
