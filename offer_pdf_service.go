// ═══════════════════════════════════════════════════════════════════════════
// OFFER / QUOTATION PDF GENERATION SERVICE
//
// MISSION: Generate professional quotations on PH Trading letterhead matching
//          the company's exact format from reference PDF "44 ARLA TECHNICAL AND COMMERCIAL OFFER"
//
// FORMAT:
//   PAGE 1 - QUOTATION:
//     - PH Trading letterhead background (full page template)
//     - Header: "Quotation" title (top right)
//     - Left block: PH Trading W.L.L contact info
//     - Right metadata: Date, Quote #, Valid Until, Customer ID, Issued by
//     - "Attention" section with contact details
//     - "Your Reference" line
//     - Introduction paragraph
//     - Line items table: Sr. No. | Description | Unit cost | Qty | Amount (BHD)
//     - Totals: SUBTOTAL, DISCOUNT, VAT 10%, TOTAL (BHD)
//
//   PAGE 2 - TERMS AND CONDITIONS:
//     - Same letterhead on every page
//     - "Terms and Conditions" header
//     - Bullet points with exact terms from reference
//     - Sign-off with issuer details
//     - "Thank you for your business!" footer
//
//   PAGE 3+ - ANNEXURE-1 (if needed):
//     - Detailed order codes/specifications table
//
// OUTPUTS:
//   - Offer PDFs: exports/offers/Offer_<number>.pdf
//
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// ============================================================================
// OFFER / QUOTATION PDF GENERATION
// ============================================================================

// GenerateOfferPDF creates a professional quotation on PH Trading letterhead
func (a *App) GenerateOfferPDF(offerID string) (string, error) {
	if err := a.requirePermission("offers:view"); err != nil {
		return "", err
	}

	if offerID == "" {
		return "", fmt.Errorf("offer ID is required")
	}

	log.Printf("📄 Generating offer/quotation PDF: offerID=%s", offerID)

	if a.db == nil {
		return "", fmt.Errorf("database not initialized")
	}

	// 1. Fetch offer with items from database
	var offer Offer
	if err := a.db.Preload("Items").First(&offer, "id = ?", offerID).Error; err != nil {
		return "", fmt.Errorf("failed to fetch offer: %w", err)
	}

	// Validate offer has items - with diagnostic for FK relationship issues
	if len(offer.Items) == 0 {
		// Diagnostic: Check raw count vs Preload result
		var rawItemCount int64
		a.db.Model(&OfferItem{}).Where("offer_id = ?", offerID).Count(&rawItemCount)

		if rawItemCount > 0 {
			// FK relationship broken - load items manually
			log.Printf("⚠️ Preload failed for offer %s (raw count: %d). Loading items manually.", offerID, rawItemCount)
			var items []OfferItem
			a.db.Where("offer_id = ?", offerID).Find(&items)
			offer.Items = items
		}

		// Final check after manual load attempt
		if len(offer.Items) == 0 {
			return "", fmt.Errorf("This offer has no line items. Please add items via the Costing Sheet before generating a PDF. Go to Costing → select RFQ → add items → Save as Offer")
		}
	}

	// Validate discount range
	if offer.DiscountPercent < 0 || offer.DiscountPercent > 100 {
		return "", fmt.Errorf("invalid discount percentage: %.2f (must be 0-100)", offer.DiscountPercent)
	}

	// 2. Fetch customer details for contact info
	var customer CustomerMaster
	if err := a.db.First(&customer, "id = ?", offer.CustomerID).Error; err != nil {
		log.Printf("⚠️ Could not fetch customer details: %v", err)
		// Continue with offer.CustomerName fallback
	}

	// 3. Get primary customer contact if available
	var customerContact CustomerContact
	a.db.Where("customer_id = ? AND is_primary_contact = ?", offer.CustomerID, true).First(&customerContact)
	if customerContact.ID == "" {
		// Fallback to any contact
		a.db.Where("customer_id = ?", offer.CustomerID).First(&customerContact)
	}

	// 4. Create PDF with letterhead
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
	// PAGE 1 - QUOTATION
	// ========================================================================
	pdf.AddPage()

	// Position for content start (below letterhead logo area)
	pdf.SetY(52)

	// "Quotation" title (top right)
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(29, 29, 31)
	quoteTitle := "Quotation"
	if offer.QuoteType != "" {
		quoteTitle = offer.QuoteType
	}
	pdf.CellFormat(0, 10, quoteTitle, "", 0, "R", false, 0, "")
	pdf.Ln(12)

	// Metadata block (2 columns)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)

	// Right column: Quote metadata
	y := pdf.GetY()
	rightX := 105.0

	pdf.SetXY(rightX, y)
	pdf.Cell(35, 5, "Date:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, offer.QuotationDate.Format("02 Jan 2006"))

	pdf.SetXY(rightX, y+6)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(35, 5, "Quote #:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, sanitizeForPDF(offer.OfferNumber))

	pdf.SetXY(rightX, y+12)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(35, 5, "Quote Valid Until:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, offer.ValidityDate.Format("02 Jan 2006"))

	pdf.SetXY(rightX, y+18)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(35, 5, "Customer ID:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	customerIDDisplay := customer.CustomerCode
	if customerIDDisplay == "" {
		customerIDDisplay = customer.CustomerID
	}
	pdf.Cell(0, 5, sanitizeForPDF(customerIDDisplay))

	pdf.SetXY(rightX, y+24)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(35, 5, "Issued by:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	issuedBy := offer.IssuedBy
	if issuedBy == "" {
		issuedBy = "Sales Team"
	}
	pdf.Cell(0, 5, sanitizeForPDF(issuedBy))

	pdf.SetY(y + 36)
	pdf.Ln(4)

	// Attention section
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 6, "Attention")
	pdf.Ln(7)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)

	// Attention person (use from offer, or fallback to customer contact)
	attentionPerson := offer.AttentionPerson
	if attentionPerson == "" && customerContact.ContactName != "" {
		attentionPerson = customerContact.ContactName
	}
	if attentionPerson != "" {
		pdf.Cell(0, 5, sanitizeForPDF(attentionPerson))
		pdf.Ln(5)
	}

	// Company name
	attentionCompany := offer.AttentionCompany
	if attentionCompany == "" {
		attentionCompany = customer.BusinessName
	}
	if attentionCompany != "" {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.Cell(0, 5, sanitizeForPDF(attentionCompany))
		pdf.Ln(5)
	}

	// Phone
	attentionPhone := offer.AttentionPhone
	if attentionPhone == "" && customerContact.Phone != "" {
		attentionPhone = customerContact.Phone
	}
	if attentionPhone != "" {
		pdf.SetFont("Helvetica", "", 9)
		pdf.Cell(0, 5, "Phone: "+sanitizeForPDF(attentionPhone))
		pdf.Ln(5)
	}

	// Address
	attentionAddress := offer.AttentionAddress
	if attentionAddress == "" {
		// Build from customer master
		addressParts := []string{}
		if customer.AddressLine1 != "" {
			addressParts = append(addressParts, customer.AddressLine1)
		}
		if customer.City != "" {
			addressParts = append(addressParts, customer.City)
		}
		if customer.Country != "" {
			addressParts = append(addressParts, customer.Country)
		}
		attentionAddress = strings.Join(addressParts, ", ")
	}
	if attentionAddress != "" {
		pdf.SetFont("Helvetica", "", 9)
		pdf.MultiCell(0, 5, sanitizeForPDF(attentionAddress), "", "", false)
	}

	pdf.Ln(4)

	// Your Reference line - use MultiCell for long references to prevent overlap
	if offer.CustomerReference != "" {
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(80, 80, 80)
		pdf.Cell(30, 5, "Your Reference:")
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(60, 60, 60)
		currentX := pdf.GetX()
		currentY := pdf.GetY()
		pdf.SetXY(currentX, currentY)
		pdf.MultiCell(0, 5, sanitizeForPDF(offer.CustomerReference), "", "", false)
		pdf.Ln(2)
	}

	pdf.Ln(2)

	// Introduction paragraph
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(40, 40, 40)
	introPara := "Dear Sir,\n\nWe appreciate your interest in our products and services. We are pleased to submit our quotation as detailed below. We trust that you will find our offer competitive and look forward to your valued order."
	pdf.MultiCell(0, 5, introPara, "", "", false)
	pdf.Ln(6)

	// Line items table
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFillColor(29, 29, 31) // Carbon/Onyx header

	// Table headers
	pdf.CellFormat(12, 7, "Sr. No.", "1", 0, "C", true, 0, "")
	pdf.CellFormat(92, 7, "Item Description, Long Code*", "1", 0, "C", true, 0, "")
	pdf.CellFormat(22, 7, "Unit cost", "1", 0, "C", true, 0, "")
	pdf.CellFormat(15, 7, "Qty", "1", 0, "C", true, 0, "")
	pdf.CellFormat(33, 7, "Amount (BHD)", "1", 0, "C", true, 0, "")
	pdf.Ln(7)

	// Table rows
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(29, 29, 31)

	// Column widths (must match header widths above)
	colSrNo := 12.0
	colDesc := 92.0
	colUnit := 22.0
	colQty := 15.0
	colAmt := 33.0
	lineHeight := 4.0
	minRowHeight := 6.0

	subtotal := 0.0
	for i, item := range offer.Items {
		lineNum := i + 1
		description := sanitizeForPDF(item.Description)
		if item.ProductCode != "" {
			productCode := sanitizeForPDF(item.ProductCode)
			if len(productCode) > 50 {
				productCode = productCode[:47] + "..."
			}
			description = description + "\n" + productCode
		}
		// Long Code is shown only on Annexure page (not page 1)
		// Previously rendered here, moved to Annexure for cleaner front page
		lineTotal := item.UnitPrice * item.Quantity
		subtotal += lineTotal

		// Pre-calculate the height needed for description text using SplitText
		lines := pdf.SplitText(description, colDesc)
		rowHeight := float64(len(lines)) * lineHeight
		if rowHeight < minRowHeight {
			rowHeight = minRowHeight
		}

		// Store starting position for the row
		startX := pdf.GetX()
		startY := pdf.GetY()

		// Render Sr. No. cell with calculated row height
		pdf.CellFormat(colSrNo, rowHeight, fmt.Sprintf("%d", lineNum), "1", 0, "C", false, 0, "")

		// Render Description cell using MultiCell (wraps text)
		descX := pdf.GetX()
		pdf.MultiCell(colDesc, lineHeight, description, "1", "L", false)
		multiCellEndY := pdf.GetY()

		// If MultiCell used less height than rowHeight, draw remaining border
		if multiCellEndY-startY < rowHeight {
			// Fill the gap with an empty bordered cell to complete the row border
			pdf.SetXY(descX, multiCellEndY)
			pdf.CellFormat(colDesc, rowHeight-(multiCellEndY-startY), "", "LRB", 0, "L", false, 0, "")
		}

		// Move back to startY for the remaining columns, positioned after Sr.No + Desc
		pdf.SetXY(startX+colSrNo+colDesc, startY)

		// Render remaining cells with the full calculated row height
		pdf.CellFormat(colUnit, rowHeight, fmt.Sprintf("%.3f", item.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(colQty, rowHeight, fmt.Sprintf("%.0f", item.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colAmt, rowHeight, fmt.Sprintf("%.3f", lineTotal), "1", 0, "R", false, 0, "")

		// Move to next row at the correct Y position
		pdf.SetXY(startX, startY+rowHeight)
	}

	pdf.Ln(3)

	// Totals Section (right-aligned)
	totalsX := 141.0 // X position for totals labels

	// Subtotal
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetX(totalsX)
	pdf.Cell(24, 6, "SUBTOTAL:")
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(27, 6, fmt.Sprintf("%.3f BHD", subtotal))
	pdf.Ln(7)

	// Discount (if applicable)
	discount := 0.0
	if offer.DiscountPercent > 0 {
		discount = subtotal * (offer.DiscountPercent / 100.0)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetX(totalsX)
		pdf.Cell(24, 6, fmt.Sprintf("DISCOUNT (%.1f%%):", offer.DiscountPercent))
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(29, 29, 31)
		pdf.Cell(27, 6, fmt.Sprintf("-%.3f BHD", discount))
		pdf.Ln(7)
	}

	// VAT (dynamic rate, default 10%)
	subtotalAfterDiscount := subtotal - discount
	vatPercent := offer.VatRate
	if vatPercent == 0 && subtotalAfterDiscount > 0 {
		vatPercent = 10.0 // Default to 10% only for legacy offers without explicit VatRate
	}
	vatAmount := subtotalAfterDiscount * (vatPercent / 100.0)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetX(totalsX)
	pdf.Cell(24, 6, fmt.Sprintf("VAT %.0f%%:", vatPercent))
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(27, 6, fmt.Sprintf("%.3f BHD", vatAmount))
	pdf.Ln(7)

	// Grand Total (bold, larger)
	grandTotal := subtotalAfterDiscount + vatAmount
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(29, 29, 31)
	pdf.SetX(totalsX)
	pdf.Cell(24, 7, "TOTAL (BHD):")
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(27, 7, fmt.Sprintf("%.3f BHD", grandTotal))
	pdf.Ln(10)

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
		"Offer for complete delivery: Prices valid for full order alone.",
		fmt.Sprintf("Payment Terms: %s", offer.PaymentTerms),
		"Warranty: Against Manufacturing Defects (12 months from date of delivery)",
		"Order Cancellation: Charges applicable according to OEM terms",
		fmt.Sprintf("Delivery time: Approx. %s; exact period confirmed after order placement.", offer.DeliveryWeeks),
		fmt.Sprintf("Delivery Terms: %s", offer.DeliveryTerms),
		"Installation: Excluded / not in our scope",
		"Commissioning: Excluded / not in our scope",
		"Testing: Excluded / not in our scope",
		"Test Certificate: Additional charges applicable as per OEM terms",
		"Certificate of Origin: Additional charges applicable as per OEM terms",
		fmt.Sprintf("Country of origin: %s", offer.CountryOfOrigin),
		"Certificate of Conformity: Additional charges applicable as per OEM terms",
		"Please check Annexure 1 for detailed Long Code(s)",
		"This proposal is subject to general terms and conditions of delivery and sales as per OEM.",
		"We have chosen our instruments according to the specifications given. However, we recommend that all technical details be reviewed and approved by your technical team before order placement.",
	}

	for _, term := range terms {
		pdf.SetX(20)
		pdf.Cell(5, 5, "-") // Use ASCII hyphen for PDF compatibility
		pdf.SetX(26)
		pdf.MultiCell(0, 5, sanitizeForPDF(term), "", "", false)
		pdf.Ln(2)
	}

	pdf.Ln(4)

	// Confidentiality statement
	pdf.SetFont("Helvetica", "I", 9)
	pdf.SetTextColor(100, 100, 100)
	pdf.Cell(0, 5, "This quotation is confidential.")
	pdf.Ln(10)

	// Sign-off
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(40, 40, 40)
	pdf.Cell(0, 5, "Best Regards,")
	pdf.Ln(6)

	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, sanitizeForPDF(issuedBy))
	pdf.Ln(5)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)
	pdf.Cell(0, 5, "Sales Executive")
	pdf.Ln(5)

	if offer.ContactPhone != "" {
		pdf.Cell(0, 5, "Mob: "+sanitizeForPDF(offer.ContactPhone))
		pdf.Ln(5)
	}

	pdf.Cell(0, 5, "Email: sales@phtrading.com")
	pdf.Ln(5)

	pdf.SetFont("Helvetica", "B", 9)
	pdf.Cell(0, 5, "PH TRADING WLL")
	pdf.Ln(10)

	// Thank you footer
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, "Thank you for your business!")

	// ========================================================================
	// PAGE 3+ - ANNEXURE-1 (if detailed specs needed)
	// ========================================================================
	// Check if any items have detailed specs/codes that need annexure
	hasDetailedSpecs := false
	for _, item := range offer.Items {
		if item.LongCode != "" || item.DetailedDescription != "" {
			hasDetailedSpecs = true
			break
		}
	}

	if hasDetailedSpecs {
		pdf.AddPage()
		pdf.SetY(52)

		pdf.SetFont("Helvetica", "B", 14)
		pdf.SetTextColor(29, 29, 31)
		pdf.Cell(0, 8, "ANNEXURE- 1")
		pdf.Ln(10)

		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(60, 60, 60)
		pdf.Cell(0, 5, "Detailed Long Codes and Specifications")
		pdf.Ln(8)

		// Annexure table
		pdf.SetFont("Helvetica", "B", 9)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFillColor(29, 29, 31)

		pdf.CellFormat(15, 7, "Item", "1", 0, "C", true, 0, "")
		pdf.CellFormat(20, 7, "QTY", "1", 0, "C", true, 0, "")
		pdf.CellFormat(139, 7, "Order code / Description", "1", 0, "C", true, 0, "")
		pdf.Ln(7)

		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(29, 29, 31)

		for i, item := range offer.Items {
			// Only include items that have LongCode or DetailedDescription
			if item.LongCode == "" && item.DetailedDescription == "" {
				continue
			}
			lineNum := i + 1

			// Build full specification
			fullSpec := ""
			if item.Equipment != "" {
				fullSpec = sanitizeForPDF(item.Equipment)
			} else {
				fullSpec = sanitizeForPDF(item.Description)
			}
			if item.ProductCode != "" {
				fullSpec = fullSpec + "\nModel: " + sanitizeForPDF(item.ProductCode)
			}
			if item.LongCode != "" {
				fullSpec = fullSpec + "\nLong Code: " + sanitizeForPDF(item.LongCode)
			}
			if item.DetailedDescription != "" {
				fullSpec = fullSpec + "\n\n" + sanitizeForPDF(item.DetailedDescription)
			}

			pdf.CellFormat(15, 6, fmt.Sprintf("%d", lineNum), "1", 0, "C", false, 0, "")
			pdf.CellFormat(20, 6, fmt.Sprintf("%.0f PC", item.Quantity), "1", 0, "C", false, 0, "")

			x := pdf.GetX()
			y := pdf.GetY()
			pdf.MultiCell(139, 4, fullSpec, "1", "L", false)
			endY := pdf.GetY()

			if endY-y > 6 {
				pdf.SetXY(x, endY)
			} else {
				pdf.SetXY(18, y+6)
			}

			// Page overflow: add new page if near bottom (A4 = 297mm)
			if pdf.GetY() > 260 && i < len(offer.Items)-1 {
				pdf.AddPage()
				pdf.SetY(52)
				// Reprint header on new page
				pdf.SetFont("Helvetica", "B", 9)
				pdf.SetTextColor(255, 255, 255)
				pdf.SetFillColor(29, 29, 31)
				pdf.CellFormat(15, 7, "Item", "1", 0, "C", true, 0, "")
				pdf.CellFormat(20, 7, "QTY", "1", 0, "C", true, 0, "")
				pdf.CellFormat(139, 7, "Order code / Description", "1", 0, "C", true, 0, "")
				pdf.Ln(7)
				pdf.SetFont("Helvetica", "", 8)
				pdf.SetTextColor(29, 29, 31)
			}
		}
	}

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
		footerTitle := "Quotation"
		if offer.QuoteType != "" {
			footerTitle = offer.QuoteType
		}
		pdf.CellFormat(0, 5, fmt.Sprintf("%s %s  |  Page %d of %d  |  PH Trading W.L.L.", footerTitle, offer.OfferNumber, i, totalPages), "", 0, "C", false, 0, "")
	}

	// ========================================================================
	// SAVE PDF
	// ========================================================================
	paths := a.getAppPaths()
	exportDir := filepath.Join(paths.ProjectRoot, "exports", "offers")
	os.MkdirAll(exportDir, 0755)

	// Combined filename: {SystemNumber}_{UserReference}.pdf
	cleanOfferNum := sanitizeFilename(offer.OfferNumber)
	userRef := ""
	if offer.CustomerReference != "" {
		userRef = sanitizeFilename(offer.CustomerReference)
	}
	filename := cleanOfferNum
	if userRef != "" {
		filename = fmt.Sprintf("%s_%s", cleanOfferNum, userRef)
	}
	filename = fmt.Sprintf("Offer_%s.pdf", filename)

	// Save to structured folder: <exports>/<division>/Customers/<CustomerName>/RFQ/
	customerDir := ""
	if offer.CustomerName != "" {
		safeCust := sanitizeFilename(offer.CustomerName)
		customerDir = filepath.Join(exportDir, "..", "PH Trading", "Customers", safeCust, "RFQ")
		os.MkdirAll(customerDir, 0755)
	}
	saveDir := exportDir
	if customerDir != "" {
		saveDir = customerDir
	}
	filePath := filepath.Join(saveDir, filename)

	if err := pdf.OutputFileAndClose(filePath); err != nil {
		return "", fmt.Errorf("failed to save offer PDF: %w", err)
	}

	log.Printf("✅ Offer PDF generated: %s", filePath)
	return filePath, nil
}

// sanitizeFilename is defined in reports.go (shared helper)
