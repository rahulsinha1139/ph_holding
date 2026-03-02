// ═══════════════════════════════════════════════════════════════════════════
// PURCHASE ORDER PDF GENERATION SERVICE
//
// MISSION: Generate professional purchase orders on PH Trading letterhead
//          matching the company's exact format standards
//
// FORMAT:
//   PAGE 1 - PURCHASE ORDER:
//     - PH Trading letterhead background (full page template)
//     - Header: "PURCHASE ORDER" title (top right)
//     - Left block: Supplier details ("TO:")
//     - Right block: PH Trading delivery address ("DELIVER TO:")
//     - PO metadata: PO Number, Date, Supplier Ref, Buyer Order No, Payment Terms
//     - Line items table: # | Description | Qty | Unit Price | Amount (BHD)
//     - Totals: SUBTOTAL, VAT 10%, TOTAL (BHD)
//     - If multi-currency: show foreign currency amounts + exchange rate
//
//   PAGE 2 - TERMS AND CONDITIONS:
//     - Same letterhead on every page
//     - "Terms and Conditions" header
//     - Standard procurement terms
//     - Bank details for supplier payments (4 banks)
//     - Authorized signature line
//     - Company stamp area
//
// OUTPUTS:
//   - PO PDFs: exports/purchase_orders/PO_<number>.pdf
//
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/jung-kurt/gofpdf"
)

// ============================================================================
// PURCHASE ORDER PDF GENERATION
// ============================================================================

// GeneratePurchaseOrderPDF creates a professional purchase order on PH Trading letterhead
func (a *App) GeneratePurchaseOrderPDF(poID string) (string, error) {
	if err := a.requirePermission("po:view"); err != nil {
		return "", err
	}

	if poID == "" {
		return "", fmt.Errorf("purchase order ID is required")
	}

	log.Printf("📄 Generating purchase order PDF: poID=%s", poID)

	// 1. Fetch PO with items from database
	var po PurchaseOrder
	if err := a.db.Preload("Items").First(&po, "id = ?", poID).Error; err != nil {
		return "", fmt.Errorf("failed to fetch purchase order: %w", err)
	}

	// Validate PO has items
	if len(po.Items) == 0 {
		return "", fmt.Errorf("This purchase order has no line items. Please add items before generating a PDF")
	}

	// 2. Fetch supplier details
	var supplier SupplierMaster
	if err := a.db.First(&supplier, "id = ?", po.SupplierID).Error; err != nil {
		log.Printf("⚠️ Could not fetch supplier details: %v", err)
		// Continue with minimal supplier info from PO
		supplier.SupplierName = po.SupplierName
		if supplier.SupplierName == "" {
			supplier.SupplierName = "Unknown Supplier"
		}
	}

	// 3. Create PDF with letterhead
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 35) // Bottom margin: 35mm to avoid letterhead footer
	pdf.SetTopMargin(50)           // Start content below letterhead header
	pdf.SetLeftMargin(18)
	pdf.SetRightMargin(18)

	// Set header function so letterhead appears on ALL pages
	imgPath := a.letterheadImagePath()
	pdf.SetHeaderFuncMode(func() {
		fileInfo, err := os.Stat(imgPath)
		if err == nil && fileInfo.Size() <= 10*1024*1024 { // 10MB max
			opt := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
			pdf.ImageOptions(imgPath, 0, 0, 210, 297, false, opt, 0, "")
		} else if err == nil && fileInfo.Size() > 10*1024*1024 {
			log.Printf("⚠️ Letterhead file too large (%d bytes), using fallback", fileInfo.Size())
			addLetterheadFallback(pdf)
		} else {
			// Fallback: simple text header if image not found
			addLetterheadFallback(pdf)
		}
	}, true) // true = allow content to overlap the header area

	// ========================================================================
	// PAGE 1 - PURCHASE ORDER
	// ========================================================================
	pdf.AddPage()

	// Position for content start (below letterhead logo area)
	pdf.SetY(52)

	// "PURCHASE ORDER" title (top right)
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(29, 29, 31)
	pdf.CellFormat(0, 10, "PURCHASE ORDER", "", 0, "R", false, 0, "")
	pdf.Ln(12)

	// ========================================================================
	// SUPPLIER AND DELIVERY BLOCKS (2 columns)
	// ========================================================================
	y := pdf.GetY()

	// LEFT BLOCK: SUPPLIER ("TO:")
	pdf.SetXY(18, y)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 6, "TO:")
	pdf.Ln(6)

	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(29, 29, 31)
	pdf.MultiCell(85, 5, sanitizeForPDF(supplier.SupplierName), "", "", false)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)

	// Supplier address
	if supplier.Address != "" {
		pdf.MultiCell(85, 5, sanitizeForPDF(supplier.Address), "", "", false)
	}
	if supplier.Country != "" {
		pdf.MultiCell(85, 5, sanitizeForPDF(supplier.Country), "", "", false)
	}

	// Supplier TaxID (TRN equivalent)
	if supplier.TaxID != "" {
		pdf.Ln(2)
		pdf.SetFont("Helvetica", "B", 9)
		pdf.Cell(20, 5, "TRN:")
		pdf.SetFont("Helvetica", "", 9)
		pdf.Cell(0, 5, sanitizeForPDF(supplier.TaxID))
		pdf.Ln(5)
	}

	// RIGHT BLOCK: DELIVER TO (PH Trading warehouse)
	rightX := 110.0
	pdf.SetXY(rightX, y)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 6, "DELIVER TO:")
	pdf.Ln(6)

	pdf.SetXY(rightX, pdf.GetY())
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 5, "PH TRADING W.L.L")
	pdf.Ln(6)

	pdf.SetXY(rightX, pdf.GetY())
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)
	pdf.MultiCell(82, 5, "Building 325, Road 1704, Block 117\nSitra Industrial Area\nManama, Kingdom of Bahrain", "", "", false)

	pdf.SetXY(rightX, pdf.GetY()+2)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.Cell(20, 5, "TRN:")
	pdf.SetFont("Helvetica", "", 9)
	pdf.Cell(0, 5, "200193538")
	pdf.Ln(8)

	// ========================================================================
	// PO METADATA (Right-aligned table)
	// ========================================================================
	metadataY := pdf.GetY()
	metaX := 105.0

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)

	// PO Number
	pdf.SetXY(metaX, metadataY)
	pdf.Cell(45, 5, "PO Number:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, sanitizeForPDF(po.PONumber))

	// PO Date
	pdf.SetXY(metaX, metadataY+6)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(45, 5, "Date:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, po.PODate.Format("02-Jan-2006"))

	// Supplier Reference (use RFQ ID if available, otherwise blank)
	pdf.SetXY(metaX, metadataY+12)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(45, 5, "Supplier Ref:")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(29, 29, 31)
	supplierRef := po.RfqID
	if supplierRef == "" {
		supplierRef = "_______________"
	}
	pdf.Cell(0, 5, sanitizeForPDF(supplierRef))

	// Buyer Order Number
	buyerOrderNo := "_______________"
	if po.OrderID != "" {
		var order Order
		if err := a.db.First(&order, "id = ?", po.OrderID).Error; err == nil {
			buyerOrderNo = order.OrderNumber
		}
	}
	pdf.SetXY(metaX, metadataY+18)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(45, 5, "Buyer Order No:")
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, sanitizeForPDF(buyerOrderNo))

	// Payment Terms
	pdf.SetXY(metaX, metadataY+24)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(45, 5, "Payment Terms:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	paymentTerms := po.PaymentTerms
	if paymentTerms == "" {
		paymentTerms = "Net 30 Days"
	}
	pdf.Cell(0, 5, sanitizeForPDF(paymentTerms))

	pdf.SetY(metadataY + 36)
	pdf.Ln(6)

	// ========================================================================
	// LINE ITEMS TABLE
	// ========================================================================
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFillColor(29, 29, 31) // Carbon/Onyx header

	// Table headers
	pdf.CellFormat(12, 7, "#", "1", 0, "C", true, 0, "")
	pdf.CellFormat(80, 7, "Description", "1", 0, "C", true, 0, "")
	pdf.CellFormat(20, 7, "Qty", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 7, "Unit Price", "1", 0, "C", true, 0, "")
	pdf.CellFormat(32, 7, "Amount", "1", 0, "C", true, 0, "")
	pdf.Ln(7)

	// Table rows
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(29, 29, 31)

	// Determine if we're showing foreign currency
	showForeignCurrency := (po.Currency != "BHD")
	currencyLabel := po.Currency
	if currencyLabel == "" {
		currencyLabel = "BHD"
	}

	subtotal := 0.0
	for i, item := range po.Items {
		lineNum := i + 1
		description := sanitizeForPDF(item.Description)
		if item.ProductCode != "" {
			description = description + "\n" + sanitizeForPDF(item.ProductCode)
		}

		// Use foreign currency prices if available, otherwise BHD
		unitPrice := item.UnitPriceForeign
		lineTotal := item.TotalForeign
		if !showForeignCurrency {
			unitPrice = item.UnitPriceBHD
			lineTotal = item.TotalBHD
		}

		subtotal += lineTotal

		// Calculate row height based on description length
		rowHeight := 6.0
		if len(description) > 60 {
			rowHeight = 10.0
		}

		pdf.CellFormat(12, rowHeight, fmt.Sprintf("%d", lineNum), "1", 0, "C", false, 0, "")

		// Description cell (multiline if needed)
		x := pdf.GetX()
		y := pdf.GetY()
		pdf.MultiCell(80, 4, description, "1", "L", false)
		pdf.SetXY(x+80, y)

		pdf.CellFormat(20, rowHeight, fmt.Sprintf("%.0f", item.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(30, rowHeight, fmt.Sprintf("%.3f %s", unitPrice, currencyLabel), "1", 0, "R", false, 0, "")
		pdf.CellFormat(32, rowHeight, fmt.Sprintf("%.3f", lineTotal), "1", 0, "R", false, 0, "")
		pdf.Ln(rowHeight)
	}

	pdf.Ln(3)

	// ========================================================================
	// TOTALS SECTION (right-aligned)
	// ========================================================================
	totalsX := 142.0 // X position for totals labels

	// Subtotal
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetX(totalsX)
	pdf.Cell(24, 6, "SUBTOTAL:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(27, 6, fmt.Sprintf("%.3f %s", subtotal, currencyLabel))
	pdf.Ln(7)

	// VAT (10%)
	vatAmount := po.VATAmount
	if showForeignCurrency {
		vatAmount = subtotal * 0.10
	}
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetX(totalsX)
	pdf.Cell(24, 6, "VAT 10%:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(27, 6, fmt.Sprintf("%.3f %s", vatAmount, currencyLabel))
	pdf.Ln(7)

	// Grand Total
	grandTotal := subtotal + vatAmount
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(29, 29, 31)
	pdf.SetX(totalsX)
	pdf.Cell(24, 7, fmt.Sprintf("TOTAL (%s):", currencyLabel))
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(27, 7, fmt.Sprintf("%.3f", grandTotal))
	pdf.Ln(8)

	// If foreign currency, show BHD equivalent
	if showForeignCurrency && po.ExchangeRate > 0 {
		totalBHD := po.TotalBHD
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetX(totalsX)
		pdf.Cell(24, 6, "TOTAL (BHD):")
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(0, 120, 80) // Green for BHD
		pdf.Cell(27, 6, fmt.Sprintf("%.3f BHD", totalBHD))
		pdf.Ln(5)

		// Exchange rate note
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.SetX(totalsX)
		pdf.MultiCell(51, 4, fmt.Sprintf("Exchange Rate: 1 %s = %.4f BHD", currencyLabel, po.ExchangeRate), "", "R", false)
		pdf.Ln(3)
	}

	// ========================================================================
	// PAGE 2 - TERMS AND CONDITIONS
	// ========================================================================
	pdf.AddPage()

	pdf.SetY(52)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(0, 120, 80) // Green header
	pdf.Cell(0, 8, "Terms and Conditions")
	pdf.Ln(10)

	// Terms bullets
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(40, 40, 40)

	terms := []string{
		fmt.Sprintf("Payment Terms: %s", paymentTerms),
		"Delivery Terms: CIF Bahrain (unless otherwise specified)",
		fmt.Sprintf("Expected Delivery: %s", po.ExpectedDelivery.Format("02-Jan-2006")),
		"Warranty: As per manufacturer's standard warranty terms",
		"Order Cancellation: Subject to supplier approval and restocking charges",
		"Inspection: Goods subject to inspection upon receipt",
		"Quality Standards: All items must meet specified quality standards",
		"Packaging: All items to be properly packaged for international shipping",
		"Documentation: Invoice, packing list, and certificates to accompany shipment",
		"Late Delivery: Supplier to notify buyer immediately of any delays",
		"Partial Shipments: Subject to prior approval from PH Trading",
		"Dispute Resolution: Any disputes to be resolved per Bahrain Commercial Law",
	}

	for _, term := range terms {
		pdf.SetX(20)
		pdf.Cell(5, 5, "•")
		pdf.SetX(26)
		pdf.MultiCell(0, 5, sanitizeForPDF(term), "", "", false)
		pdf.Ln(2)
	}

	pdf.Ln(6)

	// ========================================================================
	// BANK DETAILS FOR PAYMENT
	// ========================================================================
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 8, "Bank Details for Payment")
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(60, 60, 60)

	// Bank 1: BBK
	pdf.SetFont("Helvetica", "B", 8)
	pdf.Cell(0, 5, "Bank of Bahrain and Kuwait (BBK)")
	pdf.Ln(5)
	pdf.SetFont("Helvetica", "", 8)
	pdf.Cell(40, 4, "Account Name:")
	pdf.Cell(0, 4, "PH TRADING WLL")
	pdf.Ln(4)
	pdf.Cell(40, 4, "Account Number:")
	pdf.Cell(0, 4, "0001-002083-001")
	pdf.Ln(4)
	pdf.Cell(40, 4, "IBAN:")
	pdf.Cell(0, 4, "BH82BBKU00010020830001")
	pdf.Ln(4)
	pdf.Cell(40, 4, "SWIFT:")
	pdf.Cell(0, 4, "BBKUBHBM")
	pdf.Ln(6)

	// Bank 2: NBB
	pdf.SetFont("Helvetica", "B", 8)
	pdf.Cell(0, 5, "National Bank of Bahrain (NBB)")
	pdf.Ln(5)
	pdf.SetFont("Helvetica", "", 8)
	pdf.Cell(40, 4, "Account Name:")
	pdf.Cell(0, 4, "PH TRADING WLL")
	pdf.Ln(4)
	pdf.Cell(40, 4, "Account Number:")
	pdf.Cell(0, 4, "001000025074")
	pdf.Ln(4)
	pdf.Cell(40, 4, "IBAN:")
	pdf.Cell(0, 4, "BH04NBOB00001000025074")
	pdf.Ln(4)
	pdf.Cell(40, 4, "SWIFT:")
	pdf.Cell(0, 4, "NBOBHBMH")
	pdf.Ln(6)

	// Bank 3: Ahli United Bank
	pdf.SetFont("Helvetica", "B", 8)
	pdf.Cell(0, 5, "Ahli United Bank")
	pdf.Ln(5)
	pdf.SetFont("Helvetica", "", 8)
	pdf.Cell(40, 4, "Account Name:")
	pdf.Cell(0, 4, "PH TRADING WLL")
	pdf.Ln(4)
	pdf.Cell(40, 4, "Account Number:")
	pdf.Cell(0, 4, "0011005154940001")
	pdf.Ln(4)
	pdf.Cell(40, 4, "IBAN:")
	pdf.Cell(0, 4, "BH59AUBB00110051549400")
	pdf.Ln(4)
	pdf.Cell(40, 4, "SWIFT:")
	pdf.Cell(0, 4, "AUBBBHBM")
	pdf.Ln(6)

	// Bank 4: Al Salam Bank
	pdf.SetFont("Helvetica", "B", 8)
	pdf.Cell(0, 5, "Al Salam Bank-Bahrain B.S.C.")
	pdf.Ln(5)
	pdf.SetFont("Helvetica", "", 8)
	pdf.Cell(40, 4, "Account Name:")
	pdf.Cell(0, 4, "PH TRADING WLL")
	pdf.Ln(4)
	pdf.Cell(40, 4, "Account Number:")
	pdf.Cell(0, 4, "5050016804")
	pdf.Ln(4)
	pdf.Cell(40, 4, "IBAN:")
	pdf.Cell(0, 4, "BH79ASBB00005050016804")
	pdf.Ln(4)
	pdf.Cell(40, 4, "SWIFT:")
	pdf.Cell(0, 4, "ASBBBHBM")
	pdf.Ln(10)

	// ========================================================================
	// SIGNATURE SECTION
	// ========================================================================
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 6, "For PH Trading W.L.L")
	pdf.Ln(6)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)
	pdf.Cell(0, 5, "Authorized Signature: _____________________")
	pdf.Ln(6)
	pdf.Cell(0, 5, "Name: _____________________")
	pdf.Ln(6)
	pdf.Cell(0, 5, "Date: _____________________")
	pdf.Ln(8)

	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.Cell(0, 4, "[Company Stamp]")
	pdf.Ln(10)

	// ========================================================================
	// PAGE NUMBERS (on all pages, positioned above letterhead footer)
	// ========================================================================
	// Disable auto-break before writing page numbers to prevent blank trailing page
	pdf.SetAutoPageBreak(false, 0)
	totalPages := pdf.PageCount()
	for i := 1; i <= totalPages; i++ {
		pdf.SetPage(i)
		pdf.SetY(270) // Just above the letterhead footer area
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(160, 160, 160)
		pdf.CellFormat(0, 5, fmt.Sprintf("Purchase Order %s  |  Page %d of %d  |  PH Trading W.L.L.", po.PONumber, i, totalPages), "", 0, "C", false, 0, "")
	}

	// ========================================================================
	// SAVE PDF
	// ========================================================================
	paths := a.GetApplicationPaths()
	exportDir := filepath.Join(paths.ProjectRoot, "exports", "purchase_orders")
	os.MkdirAll(exportDir, 0755)

	// Combined filename: {SystemNumber}_{UserReference}.pdf
	cleanPONum := sanitizeFilename(po.PONumber)
	userRef := ""
	if po.RfqID != "" {
		userRef = sanitizeFilename(po.RfqID)
	}
	filename := cleanPONum
	if userRef != "" {
		filename = fmt.Sprintf("%s_%s", cleanPONum, userRef)
	}
	filename = fmt.Sprintf("PO_%s.pdf", filename)

	// Save to structured folder: <exports>/PH Trading/Suppliers/<SupplierName>/PO/
	supplierDir := ""
	if po.SupplierName != "" {
		safeSupp := sanitizeFilename(po.SupplierName)
		supplierDir = filepath.Join(exportDir, "..", "PH Trading", "Suppliers", safeSupp, "PO")
		os.MkdirAll(supplierDir, 0755)
	}
	saveDir := exportDir
	if supplierDir != "" {
		saveDir = supplierDir
	}
	filePath := filepath.Join(saveDir, filename)

	if err := pdf.OutputFileAndClose(filePath); err != nil {
		return "", fmt.Errorf("failed to save purchase order PDF: %w", err)
	}

	log.Printf("✅ Purchase Order PDF generated: %s", filePath)
	return filePath, nil
}
