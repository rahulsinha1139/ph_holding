package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Customer name mappings from AR file to our database
var customerMappings = map[string]string{
	"AL EZZEL":      "Al Ezzel",
	"ALUMINIUM":     "Aluminium Bahrain",
	"ARLA":          "Arla Foods",
	"BAPCO":         "Bapco",
	"ELECTRICITY":   "Electricity",
	"GPIC":          "Gulf Petrochemical",
	"GULF PETRO":    "Gulf Petrochemical",
	"TTSJV":         "Technip-Technicas",
	"TECTON":        "Tecton",
	"VEOLIA":        "Veolia",
	"TABREED":       "Tabreed",
	"TATWEER":       "Tatweer",
	"MINISTRY":      "Ministry",
	"MUHARRAQ":      "Muharraq",
	"YATEEM":        "Yateem",
	"ALUSERV":       "Aluserv",
	"INTERCOL":      "Intercol",
	"AL FARIS":      "Al Faris",
	"ARABIAN IND":   "Arabian Industrial",
}

type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
}

type CustomerMaster struct {
	Base
	CustomerID       string  `gorm:"uniqueIndex;size:50"`
	CustomerCode     string  `gorm:"uniqueIndex;size:50"`
	BusinessName     string  `gorm:"index;size:255"`
	PaymentGrade     string  `gorm:"index;size:10"`
	CustomerGrade    string  `gorm:"index;size:10"`
	AvgPaymentDays   float64
	TotalOrdersValue float64
}

func (CustomerMaster) TableName() string { return "customers" }

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	
	dbFile := "build/bin/data/ph_holdings.db"
	arFile := "Data_for_database/Tally_data/AR payment defaulters.xlsx"
	
	fmt.Println("📊 Importing AR Payment Defaulters...")
	
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	
	// Load all customers
	var customers []CustomerMaster
	db.Find(&customers)
	fmt.Printf("Loaded %d customers from database\n", len(customers))
	
	f, err := excelize.OpenFile(arFile)
	if err != nil {
		log.Fatalf("Failed to open Excel: %v", err)
	}
	defer f.Close()
	
	rows, err := f.GetRows("Trade Debtors")
	if err != nil {
		log.Fatalf("Failed to read sheet: %v", err)
	}
	
	fmt.Printf("Found %d rows in Trade Debtors\n\n", len(rows))
	
	updated := 0
	for i, row := range rows {
		if i < 12 || len(row) < 2 {
			continue
		}
		
		customerName := strings.TrimSpace(row[0])
		if customerName == "" || strings.Contains(strings.ToLower(customerName), "total") {
			continue
		}
		
		// Parse amounts
		var over91Days, total float64
		for j, cell := range row {
			val := parseFloat(cell)
			if j == len(row)-1 && val > 0 {
				total = val
			}
			if j == len(row)-2 && val > 0 {
				over91Days = val
			}
		}
		
		if total == 0 {
			continue
		}
		
		// Determine grade
		grade := "A"
		overduePct := float64(0)
		if total > 0 {
			overduePct = over91Days / total * 100
			if overduePct > 50 {
				grade = "D"
			} else if overduePct > 25 {
				grade = "C"
			} else if overduePct > 10 {
				grade = "B"
			}
		}
		
		// Find matching customer
		customer := findCustomer(customers, customerName)
		if customer != nil {
			oldGrade := customer.PaymentGrade
			customer.PaymentGrade = grade
			customer.CustomerGrade = grade
			customer.TotalOrdersValue = total
			customer.UpdatedAt = time.Now()
			
			if err := db.Save(&customer).Error; err == nil {
				updated++
				fmt.Printf("  ✓ %s: %s → %s (%.2f BHD, %.0f%% >91d)\n", 
					customer.BusinessName, oldGrade, grade, total, overduePct)
			}
		}
	}
	
	fmt.Printf("\n✅ Updated %d customer grades from AR defaulters data\n", updated)
	
	// Show grade distribution
	fmt.Println("\n📊 Grade Distribution:")
	var gradeA, gradeB, gradeC, gradeD int64
	db.Model(&CustomerMaster{}).Where("payment_grade = ?", "A").Count((*int64)(&gradeA))
	db.Model(&CustomerMaster{}).Where("payment_grade = ?", "B").Count((*int64)(&gradeB))
	db.Model(&CustomerMaster{}).Where("payment_grade = ?", "C").Count((*int64)(&gradeC))
	db.Model(&CustomerMaster{}).Where("payment_grade = ?", "D").Count((*int64)(&gradeD))
	fmt.Printf("   A (Excellent): %d\n   B (Good): %d\n   C (Warning): %d\n   D (Risk): %d\n", gradeA, gradeB, gradeC, gradeD)
}

func findCustomer(customers []CustomerMaster, arName string) *CustomerMaster {
	upperName := strings.ToUpper(arName)
	
	// Try mapping first
	for key, val := range customerMappings {
		if strings.Contains(upperName, key) {
			for i := range customers {
				if strings.Contains(strings.ToUpper(customers[i].BusinessName), strings.ToUpper(val)) {
					return &customers[i]
				}
			}
		}
	}
	
	// Try direct partial match
	for i := range customers {
		custUpper := strings.ToUpper(customers[i].BusinessName)
		// Check if first significant word matches
		arWords := strings.Fields(upperName)
		custWords := strings.Fields(custUpper)
		if len(arWords) > 0 && len(custWords) > 0 {
			if arWords[0] == custWords[0] {
				return &customers[i]
			}
		}
	}
	
	return nil
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
