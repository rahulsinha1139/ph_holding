package main

import (
	"fmt"
	"log"
	"time"

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

type SupplierInvoice struct {
	Base
	InvoiceNumber string  `gorm:"size:100"`
	SupplierName  string  `gorm:"size:255"`
	TotalBHD      float64 `gorm:"type:decimal(15,3)"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

func main() {
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Check 1: Distribution by type
	fmt.Println("=== INVOICE NUMBER TYPE DISTRIBUTION ===")
	var results []struct {
		Type  string
		Count int
	}

	db.Raw(`
		SELECT
			CASE
				WHEN invoice_number LIKE 'SI-%' THEN 'Auto-Gen SI'
				WHEN invoice_number LIKE 'SUPP-%' THEN 'Auto-Gen SUPP'
				WHEN invoice_number LIKE 'OCR-%' THEN 'OCR-Gen'
				ELSE 'Original'
			END as type,
			COUNT(*) as count
		FROM supplier_invoices
		GROUP BY type
	`).Scan(&results)

	for _, r := range results {
		fmt.Printf("  %s: %d invoices\n", r.Type, r.Count)
	}

	// Check 2: Sample E+H invoices
	fmt.Println("\n=== SAMPLE E+H INVOICES ===")
	var ehInvoices []SupplierInvoice
	db.Where("supplier_name LIKE ?", "%Endress%").Limit(5).Find(&ehInvoices)
	for _, inv := range ehInvoices {
		fmt.Printf("  %s | %s | %.3f BHD\n", inv.InvoiceNumber, inv.SupplierName, inv.TotalBHD)
	}

	// Check 3: Sample L&G invoices
	fmt.Println("\n=== SAMPLE L&G INVOICES ===")
	var lgInvoices []SupplierInvoice
	db.Where("supplier_name LIKE ? OR supplier_name LIKE ?", "%Landis%", "%Landys%").Limit(5).Find(&lgInvoices)
	for _, inv := range lgInvoices {
		fmt.Printf("  %s | %s | %.3f BHD\n", inv.InvoiceNumber, inv.SupplierName, inv.TotalBHD)
	}

	// Check 4: Total count
	var total int64
	db.Model(&SupplierInvoice{}).Count(&total)
	fmt.Printf("\n=== TOTAL SUPPLIER INVOICES: %d ===\n", total)
}
