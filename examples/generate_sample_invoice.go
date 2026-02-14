//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"os"
	"time"
)

// Import the parent package
// In production, this would be: import "ph_holdings_app"
// For now, we'll include the types here

type InvoiceItem struct {
	SlNo          int
	Description   string
	Quantity      float64
	Unit          string
	Rate          float64
	AnnualTotal   float64
	MonthlyAmount float64
	VATPercent    float64
	TaxableValue  float64
	VAT           float64
	Total         float64
}

type InvoiceData struct {
	InvoiceType      string
	InvoiceNumber    string
	InvoiceDate      time.Time
	TRN              string
	BuyerName        string
	BuyerBuilding    string
	BuyerRoad        string
	BuyerTown        string
	BuyerBlock       string
	BuyerCountry     string
	BuyerTRN         string
	BuyerOrderNo     string
	BuyerOrderDate   time.Time
	DeliveryNote     string
	DeliveryNoteDate time.Time
	PaymentTerms     string
	ModeOfPayment    string
	Destination      string
	TermsOfDelivery  string
	Items            []InvoiceItem
	Subtotal         float64
	TotalVAT         float64
	GrandTotal       float64
	Currency         string
	CurrencySymbol   string
	SupplierRef      string
	OtherReferences  string
	DespatchDoc      string
	DespatchMethod   string
	PlaceOfSupply    string
}

func main() {
	fmt.Println("=== PH Holdings PDF Invoice Generator Demo ===")
	fmt.Println()

	// Create sample invoice data (EWA example)
	invoiceData := &InvoiceData{
		// Header
		InvoiceType:   "TAX INVOICE",
		InvoiceNumber: "PH25/106",
		InvoiceDate:   time.Date(2025, 9, 15, 0, 0, 0, 0, time.UTC),
		TRN:           "200010357800002",

		// Buyer (Electricity and Water Authority)
		BuyerName:      "Electricity and Water Authority",
		BuyerBuilding:  "0",
		BuyerRoad:      "1702",
		BuyerTown:      "Manama",
		BuyerBlock:     "317",
		BuyerCountry:   "Kingdom of Bahrain",
		BuyerTRN:       "220013972300002",
		BuyerOrderNo:   "4500157334",
		BuyerOrderDate: time.Date(2025, 5, 28, 0, 0, 0, 0, time.UTC),

		// Delivery
		DeliveryNote:     "EDD/SLA/AIM JUN-JUL 25",
		DeliveryNoteDate: time.Date(2025, 7, 27, 0, 0, 0, 0, time.UTC),
		PaymentTerms:     "30 Days",
		ModeOfPayment:    "Bank Transfer",
		Destination:      "BAHRAIN",
		TermsOfDelivery:  "Direct Delivery to EWA Metering Section Bahrain",

		// Line items
		Items: []InvoiceItem{
			{
				SlNo:          1,
				Description:   "SLA L&G AIM GRIDSTREAM FOR JULY 2025",
				Quantity:      171425,
				Unit:          "Each",
				Rate:          0.232,
				AnnualTotal:   39770.600,
				MonthlyAmount: 3314.217,
				VATPercent:    10,
				TaxableValue:  3314.217,
				VAT:           331.422,
				Total:         3645.639,
			},
			{
				SlNo:          2,
				Description:   "Support Service 24/7 for July 2025",
				Quantity:      1,
				Unit:          "Service",
				Rate:          1150.000,
				AnnualTotal:   13800.000,
				MonthlyAmount: 1150.000,
				VATPercent:    10,
				TaxableValue:  1150.000,
				VAT:           115.000,
				Total:         1265.000,
			},
		},

		// Totals
		Subtotal:       4464.217,
		TotalVAT:       446.422,
		GrandTotal:     4910.639,
		Currency:       "BHD",
		CurrencySymbol: "B.D",

		// Additional
		SupplierRef:     "EDD/SLA/AIM JUN-JUL 25",
		OtherReferences: "AIM SLA",
		DespatchDoc:     "EDD/SLA/AIM JUN-JUL 25",
		DespatchMethod:  "DIRECT",
		PlaceOfSupply:   "Kingdom of Bahrain",
	}

	// Print invoice summary
	fmt.Printf("Invoice Type:   %s\n", invoiceData.InvoiceType)
	fmt.Printf("Invoice Number: %s\n", invoiceData.InvoiceNumber)
	fmt.Printf("Date:           %s\n", invoiceData.InvoiceDate.Format("02-Jan-2006"))
	fmt.Printf("Customer:       %s\n", invoiceData.BuyerName)
	fmt.Printf("Items:          %d\n", len(invoiceData.Items))
	fmt.Printf("Subtotal:       %s %.3f\n", invoiceData.CurrencySymbol, invoiceData.Subtotal)
	fmt.Printf("VAT:            %s %.3f\n", invoiceData.CurrencySymbol, invoiceData.TotalVAT)
	fmt.Printf("Grand Total:    %s %.3f\n", invoiceData.CurrencySymbol, invoiceData.GrandTotal)
	fmt.Println()

	// Instructions for generating PDF
	fmt.Println("To generate the PDF:")
	fmt.Println("1. Use the NewPDFGenerator() function")
	fmt.Println("2. Call generator.Generate(invoiceData, outputPath)")
	fmt.Println()
	fmt.Println("Example:")
	fmt.Println("  letterheadPath := \"../CRITICAL_PH_CONTEXT_SSOT/PH Trading Quotation Template.png\"")
	fmt.Println("  generator, _ := NewPDFGenerator(letterheadPath)")
	fmt.Println("  generator.Generate(invoiceData, \"output/invoice.pdf\")")
	fmt.Println()

	// Create output directory
	outputDir := "output"
	os.MkdirAll(outputDir, 0755)

	// Since we can't import the parent package easily in this example,
	// we'll just create a JSON file with the invoice data
	fmt.Printf("Sample invoice data ready!\n")
	fmt.Printf("Run: go test -v -run TestGenerateEWAInvoice\n")
	fmt.Printf("Output will be in: ../test_output/EWA_Invoice_PH25_106_Test.pdf\n")
	fmt.Println()
	fmt.Println("=== Demo Complete ===")
}
