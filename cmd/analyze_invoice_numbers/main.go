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
}

type SupplierInvoice struct {
	Base
	InvoiceNumber string  `gorm:"size:100"`
	SupplierName  string  `gorm:"size:255"`
	InvoiceDate   time.Time
	TotalBHD      float64 `gorm:"type:decimal(15,3)"`
}

func (SupplierInvoice) TableName() string { return "supplier_invoices" }

func main() {
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// SI-* examples
	fmt.Println("=== SI-* EXAMPLES ===")
	var siInvoices []SupplierInvoice
	db.Where("invoice_number LIKE 'SI-%'").Order("created_at ASC").Limit(10).Find(&siInvoices)
	for _, inv := range siInvoices {
		fmt.Printf("  %s | %s | Created: %s\n",
			inv.InvoiceNumber, inv.SupplierName, inv.CreatedAt.Format("2006-01-02 15:04"))
	}

	// SUPP-* examples
	fmt.Println("\n=== SUPP-* EXAMPLES ===")
	var suppInvoices []SupplierInvoice
	db.Where("invoice_number LIKE 'SUPP-%'").Order("created_at ASC").Limit(10).Find(&suppInvoices)
	for _, inv := range suppInvoices {
		fmt.Printf("  %s | %s | Created: %s\n",
			inv.InvoiceNumber, inv.SupplierName, inv.CreatedAt.Format("2006-01-02 15:04"))
	}

	// Original examples (not SI-/SUPP-/OCR-)
	fmt.Println("\n=== ORIGINAL NUMBER EXAMPLES ===")
	var origInvoices []SupplierInvoice
	db.Where("invoice_number NOT LIKE 'SI-%' AND invoice_number NOT LIKE 'SUPP-%' AND invoice_number NOT LIKE 'OCR-%'").
		Order("created_at ASC").Limit(10).Find(&origInvoices)
	for _, inv := range origInvoices {
		fmt.Printf("  %s | %s | Created: %s\n",
			inv.InvoiceNumber, inv.SupplierName, inv.CreatedAt.Format("2006-01-02 15:04"))
	}
}
