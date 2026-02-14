package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
}

type SupplierPayment struct {
	Base
	PaymentNumber string    `gorm:"uniqueIndex;size:50"`
	SupplierID    string    `gorm:"index;size:36"`
	SupplierName  string    `gorm:"size:255"`
	InvoiceID     string    `gorm:"index;size:36"`
	InvoiceNumber string    `gorm:"size:50"`
	PaymentDate   time.Time `gorm:"index"`
	AmountBHD     float64
	PaymentMethod string `gorm:"size:50"`
	Reference     string `gorm:"size:100"`
	Notes         string `gorm:"type:text"`
}

func (SupplierPayment) TableName() string { return "supplier_payments" }

// Map sheet names to supplier names
var supplierSheetMapping = map[string]string{
	"L&G MTE":        "Landis & Gyr",
	"Enerdat":        "Enerdat",
	"E+H":            "Endress+Hauser",
	"G+I":            "G+I",
	"AHS SALE":       "AHS Trading",
	"various brands": "Various",
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)
	
	dbFile := "ph_holdings.db"
	paymentsFile := "Data_for_database/Payments to suppliers.xlsx"
	
	fmt.Println("💰 Importing Supplier Payments...")
	
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	
	// Ensure table exists
	db.AutoMigrate(&SupplierPayment{})
	
	f, err := excelize.OpenFile(paymentsFile)
	if err != nil {
		log.Fatalf("Failed to open Excel: %v", err)
	}
	defer f.Close()
	
	sheets := f.GetSheetList()
	fmt.Printf("Found sheets: %v\n\n", sheets)
	
	totalCreated := 0
	
	for _, sheet := range sheets {
		supplierName := supplierSheetMapping[sheet]
		if supplierName == "" {
			supplierName = sheet
		}
		
		rows, err := f.GetRows(sheet)
		if err != nil || len(rows) < 4 {
			continue
		}
		
		created := 0
		for i, row := range rows {
			if i < 3 || len(row) < 4 {
				continue
			}
			
			// Try to find amount and paid date
			var amount float64
			var paidDate time.Time
			var invoiceNo, description string
			
			for j, cell := range row {
				cell = strings.TrimSpace(cell)
				
				// Look for invoice number pattern
				if strings.HasPrefix(cell, "PH") || strings.Contains(cell, "/") && len(cell) < 20 {
					if invoiceNo == "" {
						invoiceNo = cell
					}
				}
				
				// Look for amounts (Euro, $, AED, BHD)
				if strings.Contains(cell, "€") || strings.Contains(cell, "$") || 
				   strings.Contains(cell, "AED") || strings.Contains(cell, "BHD") ||
				   strings.Contains(cell, "Euro") {
					val := parseAmount(cell)
					if val > 0 && amount == 0 {
						amount = val
					}
				}
				
				// Try to parse dates
				if t := parseDate(cell); !t.IsZero() {
					paidDate = t
				}
				
				// Description is usually in column 1-2
				if j <= 2 && len(cell) > 5 && description == "" {
					description = cell
				}
			}
			
			if amount == 0 || paidDate.IsZero() {
				continue
			}
			
			// Generate unique payment number
			paymentNum := fmt.Sprintf("PAY-%s-%d-%d", strings.ToUpper(sheet[:3]), paidDate.Year(), i)
			
			// Check if exists
			var existing SupplierPayment
			if db.Where("payment_number = ?", paymentNum).First(&existing).Error == nil {
				continue
			}
			
			payment := SupplierPayment{
				Base: Base{
					ID:        uuid.New().String(),
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				},
				PaymentNumber: paymentNum,
				SupplierName:  supplierName,
				InvoiceNumber: invoiceNo,
				PaymentDate:   paidDate,
				AmountBHD:     amount,
				PaymentMethod: "Bank Transfer",
				Notes:         description,
			}
			
			if err := db.Create(&payment).Error; err == nil {
				created++
			}
		}
		
		if created > 0 {
			fmt.Printf("  ✓ %s: %d payments imported\n", supplierName, created)
			totalCreated += created
		}
	}
	
	fmt.Printf("\n✅ Total supplier payments imported: %d\n", totalCreated)
	
	// Show summary
	var totalPayments int64
	var totalAmount float64
	db.Model(&SupplierPayment{}).Count(&totalPayments)
	db.Model(&SupplierPayment{}).Select("COALESCE(SUM(amount_bhd), 0)").Scan(&totalAmount)
	fmt.Printf("📊 Total in database: %d payments, %.2f BHD\n", totalPayments, totalAmount)
}

func parseAmount(s string) float64 {
	s = strings.TrimSpace(s)
	// Remove currency symbols
	s = strings.ReplaceAll(s, "€", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.ReplaceAll(s, "Euro", "")
	s = strings.ReplaceAll(s, "AED", "")
	s = strings.ReplaceAll(s, "BHD", "")
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{
		"2/1/2006",
		"02/01/2006",
		"2-Jan-06",
		"02-Jan-2006",
		"2006-01-02",
		"1/2/2006",
		"01/02/2006",
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
