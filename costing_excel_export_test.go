package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

func TestStandaloneCostingWorkbookIncludesDetailedCostBreakdown(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "costing.xlsx")
	data := CostingExportData{
		Division:           "PH Trading",
		OfferNumber:        "OFFER-1",
		Date:               "2026-04-27",
		PreparedBy:         "Rahul",
		CustomerID:         "CUST-1",
		CustomerName:       "Example Customer WLL",
		ContactPerson:      "Buyer",
		RfqReference:       "RFQ-1",
		FolderNumber:       "10-26",
		CostingId:          "COST-1",
		Subject:            "Sub: Example",
		EstDelivery:        "5-7 weeks",
		DeliveryTerms:      "DAP Bahrain",
		PaymentTerms:       "30 days",
		OrderType:          "General",
		CountryOfOrigin:    "DE",
		CocCoo:             "Yes",
		TestCertificate:    "Included",
		Installation:       "No",
		Commissioning:      "No",
		Testing:            "No",
		QuoteType:          "Quotation",
		VatRate:            10,
		HiddenCharges:      25.500,
		PlaceOfSupply:      "Kingdom of Bahrain",
		TaxCategory:        "Standard",
		CustomerTRN:        "TRN-123",
		Body:               "Customer editable PDF body.",
		Subtotal:           1200,
		Discount:           50,
		NetAmount:          1150,
		VAT:                115,
		GrandTotal:         1265,
		TotalCost:          800,
		Profit:             350,
		ProfitPercent:      30.43,
		ProjectName:        "Example Project",
		TermsAndConditions: "Terms stay attached.",
		LineItems: []CostingExportLineItem{
			{
				SlNo:                1,
				Supplier:            "EH",
				Equipment:           "Pressure Transmitter",
				Model:               "PT-100",
				LongCode:            "LC-123",
				Specification:       "Range 0-10 bar",
				DetailedDescription: "Detailed multiline technical description",
				Currency:            "EUR",
				Quantity:            2,
				FOB:                 100,
				Freight:             9,
				FreightPercent:      9,
				TotalCost:           180,
				MarkupPercent:       20,
				SuggestedPrice:      220,
				TotalPrice:          440,
				ExchangeRate:        0.45,
				FobBHD:              45,
				FreightBHD:          4.05,
				Insurance:           2,
				CustomsPercent:      5,
				CustomsBHD:          2.25,
				HandlingPercent:     4,
				HandlingBHD:         1.8,
				FinancePercent:      1,
				FinanceBHD:          0.45,
				OtherCosts:          10,
				UserPrice:           230,
				UserPriceSet:        true,
			},
		},
	}

	if err := writeStandaloneCostingWorkbook(data, outputPath); err != nil {
		t.Fatalf("writeStandaloneCostingWorkbook() error = %v", err)
	}

	workbook, err := excelize.OpenFile(outputPath)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer workbook.Close()

	expectedCells := map[string]string{
		"A3":   "Header and Commercial Terms",
		"A16":  "Sl No",
		"M16":  "Freight %",
		"W16":  "Extra Cost",
		"X16":  "Unit PH Cost BHD",
		"Y16":  "Total PH Cost BHD",
		"AB16": "Manual Unit Price",
		"AC16": "Manual Price Used",
		"C17":  "Pressure Transmitter",
		"G17":  "Detailed multiline technical description",
		"AC17": "Yes",
	}
	for cell, expected := range expectedCells {
		actual, err := workbook.GetCellValue("Detailed Costing", cell)
		if err != nil {
			t.Fatalf("GetCellValue(%s) error = %v", cell, err)
		}
		if actual != expected {
			t.Fatalf("Detailed Costing!%s = %q, want %q", cell, actual, expected)
		}
	}
}

func TestEmbeddedPHCostingTemplateIsAvailable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	workbook, source, err := openCostingTemplateWorkbook(t.TempDir())
	require.NoError(t, err)
	defer workbook.Close()

	require.True(t, strings.HasPrefix(source, "embedded:"), source)
	costingIndex, err := workbook.GetSheetIndex("Customer Costing")
	require.NoError(t, err)
	require.GreaterOrEqual(t, costingIndex, 0)
	customerIndex, err := workbook.GetSheetIndex("Customer Database")
	require.NoError(t, err)
	require.GreaterOrEqual(t, customerIndex, 0)
}

func TestExportCostingToExcelUsesPHLayoutAsActiveSheet(t *testing.T) {
	homeDir, err := os.MkdirTemp("/tmp", "afhome")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(homeDir) })
	t.Setenv("HOME", homeDir)

	app := shippingGateAppForRole(t, "sales")
	data := shippingGateOfferData("sales")
	data.Division = "PH Trading"
	data.PreparedBy = "OCR Test"
	data.RfqReference = "RFQ-PH-LAYOUT"
	data.PaymentTerms = "30 days"

	outputPath, err := app.ExportCostingToExcel(data)
	require.NoError(t, err)

	workbook, err := excelize.OpenFile(outputPath)
	require.NoError(t, err)
	defer workbook.Close()

	activeSheet := workbook.GetSheetName(workbook.GetActiveSheetIndex())
	require.Equal(t, "Customer Costing", activeSheet)
	require.Equal(t, "PH Trading WLL", cellValue(t, workbook, "Customer Costing", "B1"))
	require.Equal(t, "RFQ-PH-LAYOUT", cellValue(t, workbook, "Customer Costing", "D1"))
	require.Equal(t, "30 days", cellValue(t, workbook, "Customer Costing", "D5"))
	require.Equal(t, "Shipping gate item", cellValue(t, workbook, "Customer Costing", "C8"))
	require.Equal(t, "Model no.: SG-1", cellValue(t, workbook, "Customer Costing", "C9"))
}

func cellValue(t *testing.T, workbook *excelize.File, sheet string, cell string) string {
	t.Helper()
	value, err := workbook.GetCellValue(sheet, cell)
	require.NoError(t, err)
	return value
}
