package main

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type CompanyDocumentProfile struct {
	Division       string
	LegalName      string
	LetterheadFile string
	AddressLines   []string
	VATNumber      string
	BankDetails    []string
}

func normalizeDivisionName(division string) string {
	switch strings.TrimSpace(strings.ToLower(division)) {
	case "ahs trading", "ahs trading wll", "a h s trading w.l.l", "a h s trading w.l.l.":
		return "AHS Trading"
	default:
		return "PH Trading"
	}
}

func companyDocumentProfile(division string) CompanyDocumentProfile {
	switch normalizeDivisionName(division) {
	case "AHS Trading":
		return CompanyDocumentProfile{
			Division:       "AHS Trading",
			LegalName:      "AHS TRADING W.L.L.",
			LetterheadFile: "AHS Trading Letterhead.jpg",
			AddressLines: []string{
				"P.O. Box 815, Manama",
				"Kingdom of Bahrain",
			},
			VATNumber: "220023565300002",
			BankDetails: []string{
				"1. Al Salam Bank, A/c No: 216114100100, IBAN: BH42ALSA00216114100100, SWIFT: ALSABHBM",
			},
		}
	default:
		return CompanyDocumentProfile{
			Division:       "PH Trading",
			LegalName:      "P H TRADING W.L.L",
			LetterheadFile: "PH Trading Quotation Template.png",
			AddressLines: []string{
				"Flat/Shop No.91, Building 198",
				"Road/Street 2803, AL SEEF, Block 428",
				"Kingdom of Bahrain",
			},
			VATNumber: "200010357800002",
		}
	}
}

func detectPDFImageType(imagePath string) string {
	switch strings.ToLower(filepath.Ext(imagePath)) {
	case ".jpg", ".jpeg":
		return "JPG"
	default:
		return "PNG"
	}
}

func (a *App) letterheadImagePathForDivision(division string) string {
	profile := companyDocumentProfile(division)
	assetName := AssetLetterhead
	if profile.Division == "AHS Trading" {
		assetName = AssetLetterheadAHS
	}

	if a.HasAsset(assetName) {
		cachePath, err := a.GetAssetToFile(assetName)
		if err == nil {
			log.Printf("📄 Using %s letterhead from database cache: %s", profile.Division, cachePath)
			return cachePath
		}
		log.Printf("⚠️ Failed to extract %s letterhead from DB: %v, falling back to file system", profile.Division, err)
	}

	candidatePaths := []string{}
	paths := a.getAppPaths()
	if paths != nil {
		candidatePaths = append(candidatePaths, filepath.Join(paths.ProjectRoot, "CRITICAL_PH_CONTEXT_SSOT", profile.LetterheadFile))
	}

	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		candidatePaths = append(candidatePaths,
			filepath.Join(exeDir, "..", "..", "CRITICAL_PH_CONTEXT_SSOT", profile.LetterheadFile),
			filepath.Join(exeDir, "data", profile.LetterheadFile),
			filepath.Join(exeDir, "CRITICAL_PH_CONTEXT_SSOT", profile.LetterheadFile),
		)
	}

	if cwd, err := os.Getwd(); err == nil {
		candidatePaths = append(candidatePaths, filepath.Join(cwd, "CRITICAL_PH_CONTEXT_SSOT", profile.LetterheadFile))
	}

	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			log.Printf("📄 Found %s letterhead at: %s", profile.Division, path)
			return path
		}
	}

	if paths != nil {
		defaultPath := filepath.Join(paths.ProjectRoot, "CRITICAL_PH_CONTEXT_SSOT", profile.LetterheadFile)
		log.Printf("⚠️ %s letterhead not found in any location, using default: %s", profile.Division, defaultPath)
		return defaultPath
	}

	log.Printf("⚠️ %s letterhead not found in any location", profile.Division)
	return ""
}

func (a *App) applyLetterheadForDivision(pdf *gofpdf.Fpdf, division string) {
	imgPath := a.letterheadImagePathForDivision(division)
	fileInfo, err := os.Stat(imgPath)
	if err == nil && fileInfo.Size() <= 10*1024*1024 {
		opt := gofpdf.ImageOptions{ImageType: detectPDFImageType(imgPath), ReadDpi: true}
		pdf.ImageOptions(imgPath, 0, 0, 210, 297, false, opt, 0, "")
		return
	}
	if err == nil && fileInfo.Size() > 10*1024*1024 {
		log.Printf("⚠️ Letterhead file too large (%d bytes), using fallback", fileInfo.Size())
	}
	addLetterheadFallbackForDivision(pdf, division)
}

func (a *App) gopdfLetterheadPathForDivision(division string) string {
	imgPath := a.letterheadImagePathForDivision(division)
	if imgPath == "" {
		return ""
	}

	if strings.EqualFold(filepath.Ext(imgPath), ".png") {
		return imgPath
	}

	data, err := os.ReadFile(imgPath)
	if err != nil {
		log.Printf("⚠️ Failed to read %s letterhead for gopdf normalization: %v", normalizeDivisionName(division), err)
		return imgPath
	}

	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		log.Printf("⚠️ Failed to decode %s letterhead for gopdf normalization: %v", normalizeDivisionName(division), err)
		return imgPath
	}
	normalized := image.NewNRGBA(decoded.Bounds())
	draw.Draw(normalized, normalized.Bounds(), decoded, decoded.Bounds().Min, draw.Src)

	cacheDir := filepath.Join(os.TempDir(), "ph_holdings_assets", "gopdf")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("⚠️ Failed to create gopdf asset cache: %v", err)
		return imgPath
	}

	profile := companyDocumentProfile(division)
	cachePath := filepath.Join(cacheDir, sanitizeFilename(strings.ToLower(profile.Division))+".png")
	file, err := os.Create(cachePath)
	if err != nil {
		log.Printf("⚠️ Failed to create normalized gopdf letterhead file: %v", err)
		return imgPath
	}
	defer file.Close()

	if err := png.Encode(file, normalized); err != nil {
		log.Printf("⚠️ Failed to encode normalized gopdf letterhead: %v", err)
		return imgPath
	}

	return cachePath
}

func addLetterheadFallbackForDivision(pdf *gofpdf.Fpdf, division string) {
	profile := companyDocumentProfile(division)
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetTextColor(29, 29, 31)
	pdf.SetXY(18, 18)
	pdf.Cell(0, 8, profile.LegalName)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(80, 80, 80)
	y := 28.0
	for _, line := range profile.AddressLines {
		pdf.SetXY(18, y)
		pdf.Cell(0, 5, line)
		y += 5
	}
	if profile.VATNumber != "" {
		pdf.SetXY(18, y)
		pdf.Cell(0, 5, "VAT/TRN: "+profile.VATNumber)
	}
}

func (a *App) resolveOrderDivision(orderID string) string {
	if a == nil || a.db == nil || strings.TrimSpace(orderID) == "" {
		return "PH Trading"
	}

	var order Order
	if err := a.db.Select("division").First(&order, "id = ?", orderID).Error; err == nil {
		return normalizeDivisionName(order.Division)
	}
	return "PH Trading"
}

func (a *App) resolveInvoiceDivision(invoiceID string) string {
	if a == nil || a.db == nil || strings.TrimSpace(invoiceID) == "" {
		return "PH Trading"
	}

	var invoice Invoice
	if err := a.db.Select("division").First(&invoice, "id = ?", invoiceID).Error; err == nil {
		return normalizeDivisionName(invoice.Division)
	}
	return "PH Trading"
}

func (a *App) resolvePurchaseOrderDivision(po PurchaseOrder) string {
	if strings.TrimSpace(po.Division) != "" {
		return normalizeDivisionName(po.Division)
	}
	return a.resolveOrderDivision(po.OrderID)
}

func (a *App) resolveCreditNoteDivision(cn CreditNote) string {
	if a == nil || a.db == nil || strings.TrimSpace(cn.InvoiceID) == "" {
		return "PH Trading"
	}

	var invoice Invoice
	if err := a.db.Select("division").First(&invoice, "id = ?", cn.InvoiceID).Error; err == nil {
		return normalizeDivisionName(invoice.Division)
	}
	return "PH Trading"
}

func (a *App) resolveSupplierInvoiceDivision(invoice SupplierInvoice) string {
	if strings.TrimSpace(invoice.Division) != "" {
		return normalizeDivisionName(invoice.Division)
	}
	if orderID := strings.TrimSpace(invoice.OrderID); orderID != "" {
		return a.resolveOrderDivision(orderID)
	}

	if a == nil || a.db == nil || strings.TrimSpace(invoice.PurchaseOrderID) == "" {
		return "PH Trading"
	}

	var po PurchaseOrder
	if err := a.db.Select("order_id").First(&po, "id = ?", invoice.PurchaseOrderID).Error; err == nil {
		return a.resolvePurchaseOrderDivision(po)
	}
	return "PH Trading"
}

func (a *App) resolveSupplierInvoiceDivisionByID(invoiceID string) string {
	if a == nil || a.db == nil || strings.TrimSpace(invoiceID) == "" {
		return "PH Trading"
	}

	var invoice SupplierInvoice
	if err := a.db.Select("division", "order_id", "purchase_order_id").First(&invoice, "id = ?", invoiceID).Error; err == nil {
		return a.resolveSupplierInvoiceDivision(invoice)
	}
	return "PH Trading"
}

func (a *App) resolveBankAccountDivision(bankAccountID string) string {
	if a == nil || a.db == nil || strings.TrimSpace(bankAccountID) == "" {
		return "PH Trading"
	}

	var account CompanyBankAccount
	if err := a.db.Select("division").First(&account, "id = ?", bankAccountID).Error; err == nil {
		return normalizeDivisionName(account.Division)
	}
	return "PH Trading"
}

func (a *App) resolveBankStatementDivision(statementID string) string {
	if a == nil || a.db == nil || strings.TrimSpace(statementID) == "" {
		return "PH Trading"
	}

	var statement BankStatement
	if err := a.db.Select("division", "bank_account_id").First(&statement, "id = ?", statementID).Error; err == nil {
		if strings.TrimSpace(statement.Division) != "" {
			return normalizeDivisionName(statement.Division)
		}
		return a.resolveBankAccountDivision(statement.BankAccountID)
	}
	return "PH Trading"
}
