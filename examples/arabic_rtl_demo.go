// +build ignore

// Arabic RTL Invoice Demo
// Demonstrates right-to-left rendering for Bahrain market

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	fmt.Println("=== Arabic RTL Invoice Demo ===\n")

	// Create output directory
	outputDir := filepath.Join("..", "test_output", "multilang")
	os.MkdirAll(outputDir, 0755)

	// Initialize PDF generator
	gen, err := NewPDFGenerator("")
	if err != nil {
		fmt.Printf("Error creating generator: %v\n", err)
		return
	}

	// Create Arabic invoice data
	invoice := &InvoiceData{
		// CRITICAL: Set language to Arabic for RTL!
		Language: "ar",

		// Header
		InvoiceType:   "TAX INVOICE",
		InvoiceNumber: "PH25/106-AR",
		InvoiceDate:   time.Date(2025, 11, 26, 0, 0, 0, 0, time.UTC),
		TRN:           "200010357800002",

		// Buyer (in Arabic)
		BuyerName:     "هيئة الكهرباء والماء",           // Electricity and Water Authority
		BuyerBuilding: "Building 91",
		BuyerRoad:     "Road 2803",
		BuyerTown:     "السيف",                        // Al Seef
		BuyerBlock:    "Block 428",
		BuyerCountry:  "مملكة البحرين",                // Kingdom of Bahrain
		BuyerTRN:      "BH-EWA-123456789",
		BuyerOrderNo:  "PO/2025/0156",
		BuyerOrderDate: time.Date(2025, 11, 15, 0, 0, 0, 0, time.UTC),

		// Delivery
		DeliveryNote: "DN/2025/106",
		DeliveryNoteDate: time.Date(2025, 11, 25, 0, 0, 0, 0, time.UTC),
		PaymentTerms: "30 أيام", // 30 days (Arabic)
		ModeOfPayment: "تحويل بنكي", // Bank Transfer

		// Line Items
		Items: []InvoiceItem{
			{
				SlNo:          1,
				Description:   "خدمات استشارية - نظم المعلومات", // Consulting Services - IT Systems
				Quantity:      1,
				Unit:          "خدمة", // Service
				Rate:          1500.000,
				AnnualTotal:   18000.000,
				MonthlyAmount: 1500.000,
				VATPercent:    10,
				TaxableValue:  1500.000,
				VAT:           150.000,
				Total:         1650.000,
			},
			{
				SlNo:          2,
				Description:   "دعم فني سنوي", // Annual Technical Support
				Quantity:      12,
				Unit:          "شهر", // Month
				Rate:          500.000,
				AnnualTotal:   6000.000,
				MonthlyAmount: 500.000,
				VATPercent:    10,
				TaxableValue:  6000.000,
				VAT:           600.000,
				Total:         6600.000,
			},
		},

		// Totals
		Subtotal:   7500.000,
		TotalVAT:   750.000,
		GrandTotal: 8250.000,
		Currency:   "BHD",
		CurrencySymbol: "د.ب", // Bahraini Dinar in Arabic
	}

	// Generate PDF
	outputPath := filepath.Join(outputDir, "invoice_arabic_rtl_demo.pdf")
	fmt.Printf("Generating Arabic RTL invoice...\n")
	fmt.Printf("Output: %s\n\n", outputPath)

	err = gen.Generate(invoice, outputPath)

	if err != nil {
		fmt.Printf("⚠️  Generation failed: %v\n", err)
		fmt.Println("\nPossible causes:")
		fmt.Println("1. Arabic font not installed (Noto Sans Arabic or Tahoma)")
		fmt.Println("2. Install from: https://fonts.google.com/noto/specimen/Noto+Sans+Arabic")
		fmt.Println("3. Or use Windows Tahoma (C:\\Windows\\Fonts\\tahoma.ttf)")
	} else {
		fmt.Println("✓ Arabic RTL invoice generated successfully!")

		if fileExists(outputPath) {
			info, _ := os.Stat(outputPath)
			fmt.Printf("  File size: %d bytes\n", info.Size())
		}

		fmt.Println("\nRTL Features Demonstrated:")
		fmt.Println("✓ Right-to-left text alignment")
		fmt.Println("✓ Arabic numerals with Arabic separators")
		fmt.Println("✓ Currency position (after amount)")
		fmt.Println("✓ Translated invoice terms")
		fmt.Println("✓ Mirrored layout (left/right swap)")

		fmt.Println("\nLanguage Pack Info:")
		lp := NewLangPack()
		pack := lp.Get("ar")
		fmt.Printf("  Name: %s\n", pack.Name)
		fmt.Printf("  Direction: %s\n", pack.Direction)
		fmt.Printf("  Font: %s\n", pack.FontFamily)
		fmt.Printf("  Currency: %s (position: %s)\n", pack.NumberFormat.Currency, pack.CurrencyPosition)
		fmt.Printf("  Decimal: %s, Thousands: %s\n", pack.NumberFormat.Decimal, pack.NumberFormat.Thousands)

		fmt.Println("\nSample Translations:")
		keys := []string{"invoice", "buyer", "total", "trn"}
		for _, key := range keys {
			fmt.Printf("  %s: %s\n", key, pack.Translations[key])
		}
	}

	fmt.Println("\n=== Demo Complete ===")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
