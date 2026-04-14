package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/xuri/excelize/v2"
)

// GenerateDataImportTemplate creates a multi-sheet Excel template for client data onboarding.
// The client fills this out and returns it for import into AsymmFlow.
func (a *App) GenerateDataImportTemplate() (outputPath string, err error) {
	if err := a.requirePermission("settings:update"); err != nil {
		return "", err
	}

	f := excelize.NewFile()
	xl := newGuardedExcelFile(f)
	defer func() {
		if closeErr := xl.Close(); err == nil && closeErr != nil {
			err = fmt.Errorf("failed to close workbook: %w", closeErr)
		}
	}()

	// =========================================================================
	// STYLES
	// =========================================================================

	// Title style - dark blue background, white bold text, large font
	titleStyle := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "FFFFFF", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"1B2A4A"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "bottom", Color: "E8A838", Style: 2},
		},
	})

	// Section header style - gold accent
	sectionStyle := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "1B2A4A", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"E8A838"}},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
	})

	// Column header style - dark header with white text
	headerStyle := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"2C3E50"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "95A5A6", Style: 1},
			{Type: "right", Color: "95A5A6", Style: 1},
			{Type: "top", Color: "95A5A6", Style: 1},
			{Type: "bottom", Color: "95A5A6", Style: 1},
		},
	})

	// Required column header - red accent to show mandatory
	requiredHeaderStyle := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "FFFFFF", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"C0392B"}},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "95A5A6", Style: 1},
			{Type: "right", Color: "95A5A6", Style: 1},
			{Type: "top", Color: "95A5A6", Style: 1},
			{Type: "bottom", Color: "95A5A6", Style: 1},
		},
	})

	// Example data style - light blue background, italic
	exampleStyle := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Italic: true, Size: 10, Color: "7F8C8D", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"EBF5FB"}},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "D5DBDB", Style: 1},
			{Type: "right", Color: "D5DBDB", Style: 1},
			{Type: "top", Color: "D5DBDB", Style: 1},
			{Type: "bottom", Color: "D5DBDB", Style: 1},
		},
	})

	// Normal data cell style
	dataStyle := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Family: "Calibri"},
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "D5DBDB", Style: 1},
			{Type: "right", Color: "D5DBDB", Style: 1},
			{Type: "top", Color: "D5DBDB", Style: 1},
			{Type: "bottom", Color: "D5DBDB", Style: 1},
		},
	})

	// Note/instruction style - yellow background
	noteStyle := xl.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "856404", Family: "Calibri"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFF3CD"}},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "FFEEBA", Style: 1},
			{Type: "right", Color: "FFEEBA", Style: 1},
			{Type: "top", Color: "FFEEBA", Style: 1},
			{Type: "bottom", Color: "FFEEBA", Style: 1},
		},
	})

	// =========================================================================
	// SHEET 1: INSTRUCTIONS
	// =========================================================================
	instrSheet := "Instructions"
	xl.SetSheetName("Sheet1", instrSheet)

	xl.SetColWidth(instrSheet, "A", "A", 80)
	xl.SetRowHeight(instrSheet, 1, 45)

	xl.MergeCell(instrSheet, "A1", "A1")
	xl.SetCellValue(instrSheet, "A1", "AsymmFlow - Data Import Template")
	xl.SetCellStyle(instrSheet, "A1", "A1", titleStyle)

	instructions := []struct {
		row   int
		style int
		text  string
	}{
		{3, sectionStyle, "OVERVIEW"},
		{4, noteStyle, "This workbook contains sheets for importing master data and transactions into AsymmFlow ERP."},
		{5, noteStyle, "Please fill in the data sheets following the guidelines below, then return this file for import."},
		{7, sectionStyle, "IMPORT ORDER (IMPORTANT)"},
		{8, noteStyle, "1. Customers  - Fill first (other sheets reference customers)"},
		{9, noteStyle, "2. Customer Contacts  - References customer_code from Customers sheet"},
		{10, noteStyle, "3. Suppliers  - Fill second (products and POs reference suppliers)"},
		{11, noteStyle, "4. Supplier Contacts  - References supplier_code from Suppliers sheet"},
		{12, noteStyle, "5. Products  - References supplier_code from Suppliers sheet"},
		{13, noteStyle, "6. Orders  - References customer_code from Customers sheet"},
		{14, noteStyle, "7. Invoices  - References customer_code and order_number"},
		{15, noteStyle, "8. Payments  - References invoice_number from Invoices sheet"},
		{17, sectionStyle, "COLUMN GUIDE"},
		{18, noteStyle, "RED header columns are REQUIRED - these must be filled for every row."},
		{19, noteStyle, "DARK header columns are OPTIONAL - fill if data is available."},
		{20, noteStyle, "Row 2 (light blue italic) is an EXAMPLE row - delete it before submitting."},
		{22, sectionStyle, "DATA FORMATTING RULES"},
		{23, noteStyle, "Dates: Use DD/MM/YYYY format (e.g., 15/03/2025)"},
		{24, noteStyle, "Amounts: Use numbers without commas (e.g., 15250.500, not 15,250.500)"},
		{25, noteStyle, "Currency: BHD uses 3 decimal places (e.g., 1234.567)"},
		{26, noteStyle, "Codes: Use SHORT UPPERCASE codes (e.g., CUST001, SUP-EH, PROD-FT10)"},
		{27, noteStyle, "Booleans: Use TRUE or FALSE (not Yes/No or 1/0)"},
		{29, sectionStyle, "DROPDOWN FIELDS"},
		{30, noteStyle, "Some columns have dropdown validation (click the cell to see options)."},
		{31, noteStyle, "If your value isn't in the dropdown, type it manually - we'll review during import."},
		{33, sectionStyle, "TIPS"},
		{34, noteStyle, "- Keep customer_code / supplier_code consistent across all sheets"},
		{35, noteStyle, "- Do NOT change column headers or sheet names"},
		{36, noteStyle, "- You can add as many rows as needed below the example row"},
		{37, noteStyle, "- Leave optional fields blank if data is not available (do NOT put N/A or -)"},
		{38, noteStyle, fmt.Sprintf("- Template generated: %s", time.Now().Format("02 Jan 2006 15:04"))},
		{40, sectionStyle, "SUPPORT"},
		{41, noteStyle, "For questions about this template, contact: admin@asymmetrica.co"},
	}

	for _, instr := range instructions {
		xl.SetCellValue(instrSheet, fmt.Sprintf("A%d", instr.row), instr.text)
		xl.SetCellStyle(instrSheet, fmt.Sprintf("A%d", instr.row), fmt.Sprintf("A%d", instr.row), instr.style)
		xl.SetRowHeight(instrSheet, instr.row, 22)
	}

	// =========================================================================
	// SHEET 2: CUSTOMERS
	// =========================================================================
	custSheet := "Customers"
	xl.NewSheet(custSheet)

	custHeaders := []sheetHeader{
		{"A", "customer_code", 18, true},
		{"B", "business_name", 35, true},
		{"C", "customer_type", 18, false},
		{"D", "address_line1", 35, false},
		{"E", "city", 15, false},
		{"F", "country", 15, false},
		{"G", "trn (Tax ID)", 18, false},
		{"H", "industry", 20, false},
		{"I", "payment_terms_days", 18, false},
		{"J", "credit_limit_bhd", 18, false},
		{"K", "payment_grade", 15, false},
		{"L", "customer_grade", 15, false},
	}

	writeSheetHeaders(xl, custSheet, custHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	// Example row
	custExample := []string{
		"BAPCO-001", "Bahrain Petroleum Company", "Corporate",
		"P.O. Box 25555, Awali", "Awali", "Bahrain",
		"123456789", "Oil & Gas", "30", "50000.000", "A", "A",
	}
	writeExampleRow(xl, custSheet, custExample, exampleStyle, len(custHeaders))

	// Apply data style to rows 3-100
	applyDataStyle(xl, custSheet, dataStyle, len(custHeaders), 3, 100)

	// Dropdowns
	addDropdown(xl, custSheet, "C3:C500", `"Corporate,Government,SME,Individual,Joint Venture"`)
	addDropdown(xl, custSheet, "K3:K500", `"A,B,C,D"`)
	addDropdown(xl, custSheet, "L3:L500", `"A,B,C,D"`)

	// =========================================================================
	// SHEET 3: CUSTOMER CONTACTS
	// =========================================================================
	ccSheet := "Customer Contacts"
	xl.NewSheet(ccSheet)

	ccHeaders := []sheetHeader{
		{"A", "customer_code", 18, true},
		{"B", "contact_name", 30, true},
		{"C", "job_title", 25, false},
		{"D", "email", 30, false},
		{"E", "phone", 20, false},
		{"F", "address", 40, false},
		{"G", "is_primary_contact", 18, false},
	}

	writeSheetHeaders(xl, ccSheet, ccHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	ccExample := []string{
		"BAPCO-001", "Ahmed Al Khalifa", "Procurement Manager",
		"ahmed@bapco.net", "+973 1775 5555", "P.O. Box 25555, Awali", "TRUE",
	}
	writeExampleRow(xl, ccSheet, ccExample, exampleStyle, len(ccHeaders))
	applyDataStyle(xl, ccSheet, dataStyle, len(ccHeaders), 3, 100)
	addDropdown(xl, ccSheet, "G3:G500", `"TRUE,FALSE"`)

	// =========================================================================
	// SHEET 4: SUPPLIERS
	// =========================================================================
	supSheet := "Suppliers"
	xl.NewSheet(supSheet)

	supHeaders := []sheetHeader{
		{"A", "supplier_code", 18, true},
		{"B", "supplier_name", 35, true},
		{"C", "country", 15, false},
		{"D", "supplier_type", 18, false},
		{"E", "brands_handled", 30, false},
		{"F", "primary_contact", 25, false},
		{"G", "email", 30, false},
		{"H", "phone", 20, false},
		{"I", "address", 40, false},
		{"J", "payment_terms", 18, false},
		{"K", "lead_time_days", 15, false},
		{"L", "bank_name", 25, false},
		{"M", "iban", 30, false},
		{"N", "swift_code", 15, false},
		{"O", "tax_id", 18, false},
	}

	writeSheetHeaders(xl, supSheet, supHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	supExample := []string{
		"SUP-EH", "Endress+Hauser AG", "Switzerland", "Manufacturer",
		"Endress+Hauser", "Hans Mueller", "hans@endress.com", "+41 61 715 7575",
		"Kagenstrasse 2, 4153 Reinach", "Net 60", "45",
		"UBS AG", "CH93 0076 2011 6238 5295 7", "UBSWCHZH80A", "CHE-123.456.789",
	}
	writeExampleRow(xl, supSheet, supExample, exampleStyle, len(supHeaders))
	applyDataStyle(xl, supSheet, dataStyle, len(supHeaders), 3, 100)
	addDropdown(xl, supSheet, "D3:D500", `"Manufacturer,Distributor,Agent,Service Provider"`)

	// =========================================================================
	// SHEET 5: SUPPLIER CONTACTS
	// =========================================================================
	scSheet := "Supplier Contacts"
	xl.NewSheet(scSheet)

	scHeaders := []sheetHeader{
		{"A", "supplier_code", 18, true},
		{"B", "contact_name", 30, true},
		{"C", "job_title", 25, false},
		{"D", "email", 30, false},
		{"E", "phone", 20, false},
		{"F", "address", 40, false},
		{"G", "is_primary_contact", 18, false},
	}

	writeSheetHeaders(xl, scSheet, scHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	scExample := []string{
		"SUP-EH", "Hans Mueller", "Regional Sales Manager",
		"hans@endress.com", "+41 61 715 7575", "Kagenstrasse 2, Reinach", "TRUE",
	}
	writeExampleRow(xl, scSheet, scExample, exampleStyle, len(scHeaders))
	applyDataStyle(xl, scSheet, dataStyle, len(scHeaders), 3, 100)
	addDropdown(xl, scSheet, "G3:G500", `"TRUE,FALSE"`)

	// =========================================================================
	// SHEET 6: PRODUCTS
	// =========================================================================
	prodSheet := "Products"
	xl.NewSheet(prodSheet)

	prodHeaders := []sheetHeader{
		{"A", "product_code", 20, true},
		{"B", "product_name", 35, true},
		{"C", "product_category", 20, false},
		{"D", "supplier_code", 18, true},
		{"E", "part_number", 20, false},
		{"F", "description", 40, false},
		{"G", "standard_cost_bhd", 18, false},
		{"H", "standard_price_bhd", 18, false},
		{"I", "unit_of_measure", 15, false},
		{"J", "hs_code", 15, false},
	}

	writeSheetHeaders(xl, prodSheet, prodHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	prodExample := []string{
		"EH-FMR60", "Micropilot FMR60 Radar Level", "Level Measurement",
		"SUP-EH", "FMR60-AAACCAAPK2", "80 GHz radar level transmitter for liquids",
		"2500.000", "4200.000", "Each", "9026.10",
	}
	writeExampleRow(xl, prodSheet, prodExample, exampleStyle, len(prodHeaders))
	applyDataStyle(xl, prodSheet, dataStyle, len(prodHeaders), 3, 100)
	addDropdown(xl, prodSheet, "C3:C500", `"Level Measurement,Flow Measurement,Pressure Measurement,Temperature Measurement,Analytics,Gas Analysis,Electrical,Services,Spare Parts,Other"`)
	addDropdown(xl, prodSheet, "I3:I500", `"Each,Set,Meter,Kg,Lot,Box"`)

	// =========================================================================
	// SHEET 7: ORDERS
	// =========================================================================
	orderSheet := "Orders"
	xl.NewSheet(orderSheet)

	orderHeaders := []sheetHeader{
		{"A", "order_number", 18, true},
		{"B", "customer_code", 18, true},
		{"C", "customer_po_number", 20, false},
		{"D", "order_date (DD/MM/YYYY)", 22, true},
		{"E", "required_date (DD/MM/YYYY)", 22, false},
		{"F", "status", 18, false},
		{"G", "total_value_bhd", 18, false},
		{"H", "payment_terms", 25, false},
		{"I", "delivery_terms", 25, false},
		{"J", "customer_reference", 20, false},
		{"K", "attention_person", 25, false},
	}

	writeSheetHeaders(xl, orderSheet, orderHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	orderExample := []string{
		"ORD-2025-001", "BAPCO-001", "BAPCO-PO-44521",
		"15/03/2025", "30/04/2025", "Processing",
		"42500.000", "30 days from delivery", "DAP Bahrain",
		"RFQ-2025-188", "Ahmed Al Khalifa",
	}
	writeExampleRow(xl, orderSheet, orderExample, exampleStyle, len(orderHeaders))
	applyDataStyle(xl, orderSheet, dataStyle, len(orderHeaders), 3, 100)
	addDropdown(xl, orderSheet, "F3:F500", `"Processing,Confirmed,Shipped,Delivered,Completed,Cancelled"`)

	// =========================================================================
	// SHEET 8: INVOICES
	// =========================================================================
	invSheet := "Invoices"
	xl.NewSheet(invSheet)

	invHeaders := []sheetHeader{
		{"A", "invoice_number", 20, true},
		{"B", "invoice_date (DD/MM/YYYY)", 22, true},
		{"C", "customer_code", 18, true},
		{"D", "order_number", 18, false},
		{"E", "customer_po_number", 20, false},
		{"F", "subtotal_bhd", 15, false},
		{"G", "vat_percent", 12, false},
		{"H", "vat_bhd", 12, false},
		{"I", "grand_total_bhd", 18, true},
		{"J", "status", 15, false},
		{"K", "outstanding_bhd", 18, false},
		{"L", "due_date (DD/MM/YYYY)", 22, false},
		{"M", "payment_terms", 25, false},
		{"N", "mode_of_payment", 20, false},
	}

	writeSheetHeaders(xl, invSheet, invHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	invExample := []string{
		"INV-2025-001", "20/03/2025", "BAPCO-001",
		"ORD-2025-001", "BAPCO-PO-44521",
		"38636.364", "10", "3863.636", "42500.000",
		"Sent", "42500.000", "19/04/2025",
		"30 days from delivery", "Bank Transfer",
	}
	writeExampleRow(xl, invSheet, invExample, exampleStyle, len(invHeaders))
	applyDataStyle(xl, invSheet, dataStyle, len(invHeaders), 3, 100)
	addDropdown(xl, invSheet, "J3:J500", `"Draft,Sent,Paid,PartiallyPaid,Overdue,Cancelled"`)
	addDropdown(xl, invSheet, "N3:N500", `"Bank Transfer,Cheque,Cash,LC,Credit Card,PDC"`)

	// =========================================================================
	// SHEET 9: PAYMENTS
	// =========================================================================
	paySheet := "Payments"
	xl.NewSheet(paySheet)

	payHeaders := []sheetHeader{
		{"A", "invoice_number", 20, true},
		{"B", "payment_date (DD/MM/YYYY)", 22, true},
		{"C", "amount_bhd", 18, true},
		{"D", "payment_method", 18, false},
		{"E", "reference", 25, false},
	}

	writeSheetHeaders(xl, paySheet, payHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	payExample := []string{
		"INV-2025-001", "15/04/2025", "42500.000",
		"Bank Transfer", "TRF-2025-04-15-BAPCO",
	}
	writeExampleRow(xl, paySheet, payExample, exampleStyle, len(payHeaders))
	applyDataStyle(xl, paySheet, dataStyle, len(payHeaders), 3, 100)
	addDropdown(xl, paySheet, "D3:D500", `"Cash,Cheque,Bank Transfer,Credit Card,LC,PDC,Other"`)

	// =========================================================================
	// SHEET 10: PURCHASE ORDERS (Bonus - useful for ops data)
	// =========================================================================
	poSheet := "Purchase Orders"
	xl.NewSheet(poSheet)

	poHeaders := []sheetHeader{
		{"A", "po_number", 18, true},
		{"B", "supplier_code", 18, true},
		{"C", "order_number (internal)", 22, false},
		{"D", "po_date (DD/MM/YYYY)", 22, true},
		{"E", "expected_delivery (DD/MM/YYYY)", 25, false},
		{"F", "currency", 12, false},
		{"G", "subtotal_bhd", 15, false},
		{"H", "vat_amount_bhd", 15, false},
		{"I", "total_bhd", 15, true},
		{"J", "status", 18, false},
		{"K", "payment_terms", 25, false},
	}

	writeSheetHeaders(xl, poSheet, poHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	poExample := []string{
		"PO-2025-001", "SUP-EH", "ORD-2025-001",
		"18/03/2025", "30/04/2025", "EUR",
		"22727.273", "2272.727", "25000.000",
		"Sent", "Net 60",
	}
	writeExampleRow(xl, poSheet, poExample, exampleStyle, len(poHeaders))
	applyDataStyle(xl, poSheet, dataStyle, len(poHeaders), 3, 100)
	addDropdown(xl, poSheet, "F3:F500", `"BHD,USD,EUR,GBP,CHF,SAR,AED"`)
	addDropdown(xl, poSheet, "J3:J500", `"Draft,Approved,Sent,Acknowledged,Partially Received,Received,Cancelled"`)

	// =========================================================================
	// SHEET 11: SUPPLIER INVOICES
	// =========================================================================
	siSheet := "Supplier Invoices"
	xl.NewSheet(siSheet)

	siHeaders := []sheetHeader{
		{"A", "invoice_number", 20, true},
		{"B", "supplier_code", 18, true},
		{"C", "po_number", 18, false},
		{"D", "invoice_date (DD/MM/YYYY)", 22, true},
		{"E", "due_date (DD/MM/YYYY)", 22, false},
		{"F", "currency", 12, false},
		{"G", "total_bhd", 15, true},
		{"H", "status", 15, false},
		{"I", "payment_status", 15, false},
	}

	writeSheetHeaders(xl, siSheet, siHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	siExample := []string{
		"EH-INV-2025-4421", "SUP-EH", "PO-2025-001",
		"25/04/2025", "24/06/2025", "EUR",
		"25000.000", "Approved", "Unpaid",
	}
	writeExampleRow(xl, siSheet, siExample, exampleStyle, len(siHeaders))
	applyDataStyle(xl, siSheet, dataStyle, len(siHeaders), 3, 100)
	addDropdown(xl, siSheet, "F3:F500", `"BHD,USD,EUR,GBP,CHF,SAR,AED"`)
	addDropdown(xl, siSheet, "H3:H500", `"Pending,Approved,Rejected,Paid,Dispute"`)
	addDropdown(xl, siSheet, "I3:I500", `"Unpaid,Scheduled,Paid"`)

	// =========================================================================
	// SHEET 12: SUPPLIER PAYMENTS
	// =========================================================================
	spSheet := "Supplier Payments"
	xl.NewSheet(spSheet)

	spHeaders := []sheetHeader{
		{"A", "supplier_invoice_number", 22, true},
		{"B", "supplier_code", 18, true},
		{"C", "payment_date (DD/MM/YYYY)", 22, true},
		{"D", "amount_bhd", 18, true},
		{"E", "currency", 12, false},
		{"F", "payment_method", 18, false},
		{"G", "reference", 25, false},
		{"H", "notes", 35, false},
	}

	writeSheetHeaders(xl, spSheet, spHeaders, headerStyle, requiredHeaderStyle, titleStyle)

	spExample := []string{
		"EH-INV-2025-4421", "SUP-EH", "20/06/2025",
		"25000.000", "EUR", "Bank Transfer",
		"TT-2025-06-20-EH", "Payment for FMR60 order",
	}
	writeExampleRow(xl, spSheet, spExample, exampleStyle, len(spHeaders))
	applyDataStyle(xl, spSheet, dataStyle, len(spHeaders), 3, 100)
	addDropdown(xl, spSheet, "E3:E500", `"BHD,USD,EUR,GBP,CHF,SAR,AED"`)
	addDropdown(xl, spSheet, "F3:F500", `"Bank Transfer,Cheque,LC,Cash,Wire Transfer,PDC,Other"`)

	// =========================================================================
	// Set active sheet to Instructions
	// =========================================================================
	instrIdx := xl.GetSheetIndex(instrSheet)
	f.SetActiveSheet(instrIdx)

	// =========================================================================
	// SAVE FILE
	// =========================================================================

	// Determine output path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}
	desktopDir := filepath.Join(homeDir, "Desktop")
	if _, err := os.Stat(desktopDir); os.IsNotExist(err) {
		desktopDir = homeDir
	}

	timestamp := time.Now().Format("2006_01_02")
	fileName := fmt.Sprintf("AsymmFlow_Data_Import_Template_%s.xlsx", timestamp)
	outputPath = filepath.Join(desktopDir, fileName)

	if err := xl.Err(); err != nil {
		return "", fmt.Errorf("failed to build template workbook: %w", err)
	}
	xl.SaveAs(outputPath)
	if err := xl.Err(); err != nil {
		return "", fmt.Errorf("failed to save template: %w", err)
	}

	log.Printf("Data import template generated: %s", outputPath)
	return outputPath, nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

type sheetHeader struct {
	col      string
	name     string
	width    float64
	required bool
}

func writeSheetHeaders(xl *guardedExcelFile, sheet string, headers []sheetHeader, headerStyle, requiredHeaderStyle, titleStyle int) {

	// Title row
	lastCol := headers[len(headers)-1].col
	xl.MergeCell(sheet, "A1", lastCol+"1")
	xl.SetCellValue(sheet, "A1", fmt.Sprintf("  %s", sheet))
	xl.SetCellStyle(sheet, "A1", lastCol+"1", titleStyle)
	xl.SetRowHeight(sheet, 1, 35)

	// Header row
	xl.SetRowHeight(sheet, 2, 28)
	for _, h := range headers {
		cell := h.col + "2"
		xl.SetCellValue(sheet, cell, h.name)
		xl.SetColWidth(sheet, h.col, h.col, h.width)
		if h.required {
			xl.SetCellStyle(sheet, cell, cell, requiredHeaderStyle)
		} else {
			xl.SetCellStyle(sheet, cell, cell, headerStyle)
		}
	}
}

func writeExampleRow(xl *guardedExcelFile, sheet string, values []string, style int, colCount int) {
	xl.SetRowHeight(sheet, 3, 22)
	for i, val := range values {
		if i >= colCount {
			break
		}
		colLetter := xl.ColumnName(i + 1)
		cell := colLetter + "3"
		xl.SetCellValue(sheet, cell, val)
		xl.SetCellStyle(sheet, cell, cell, style)
	}
}

func applyDataStyle(xl *guardedExcelFile, sheet string, style int, colCount int, startRow, endRow int) {
	firstCol := "A"
	lastCol := xl.ColumnName(colCount)
	for row := startRow; row <= endRow; row++ {
		xl.SetCellStyle(sheet, fmt.Sprintf("%s%d", firstCol, row), fmt.Sprintf("%s%d", lastCol, row), style)
	}
}

func addDropdown(xl *guardedExcelFile, sheet, sqref, formula string) {
	dv := excelize.NewDataValidation(true)
	dv.Sqref = sqref
	// Parse the formula string like `"A,B,C"` into a slice
	trimmed := formula
	if len(trimmed) > 2 && trimmed[0] == '"' && trimmed[len(trimmed)-1] == '"' {
		trimmed = trimmed[1 : len(trimmed)-1]
	}
	items := splitDropdownItems(trimmed)
	xl.capture(dv.SetDropList(items))
	dv.SetError(excelize.DataValidationErrorStyleWarning, "Invalid Value", "Please select from the dropdown or type a valid value.")
	dv.SetInput("Select Value", "Choose from the dropdown or type a custom value.")
	xl.AddDataValidation(sheet, dv)
}

func splitDropdownItems(s string) []string {
	parts := []string{}
	for _, p := range splitOnComma(s) {
		trimmed := trimSpace(p)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

func splitOnComma(s string) []string {
	result := []string{}
	current := ""
	for _, ch := range s {
		if ch == ',' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	result = append(result, current)
	return result
}

func trimSpace(s string) string {
	for len(s) > 0 && s[0] == ' ' {
		s = s[1:]
	}
	for len(s) > 0 && s[len(s)-1] == ' ' {
		s = s[:len(s)-1]
	}
	return s
}
