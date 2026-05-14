package main

import (
	"strings"

	"github.com/jung-kurt/gofpdf"
)

type PHOfferSignatureBlock struct {
	DisplayName string
	Title       string
	Company     string
	Address     []string
	Mobile      string
	Office      string
	Fax         string
	Email       string
	Aliases     []string
}

var phOfferSignatureBlocks = []PHOfferSignatureBlock{
	{
		DisplayName: "EBIN CHACKO",
		Title:       "Technical Sales Engineer",
		Company:     "PH TRADING W.L.L",
		Address:     []string{"P.O. BOX 815, MANAMA,", "KINGDOM OF BAHRAIN"},
		Mobile:      "+973 36418072",
		Office:      "+973 17587654",
		Fax:         "+973 17564456",
		Email:       "ebin.chacko@platinumholdings.com.bh",
		Aliases:     []string{"Ebin", "Ebin Chacko", "EBIN CHACKO"},
	},
	{
		DisplayName: "Periyasamy Neelakandan",
		Title:       "Assistant Sales Manager",
		Company:     "PH Trading WLL",
		Address:     []string{"P.O. BOX 815, MANAMA,", "KINGDOM OF BAHRAIN"},
		Mobile:      "+973 34128132",
		Office:      "+973 17587654",
		Fax:         "+973 17564456",
		Email:       "periyasamy.n@platinumholdings.com.bh",
		Aliases:     []string{"Periyasamy N", "Periyasamy Neelakandan", "Peri", "Periyasamy"},
	},
	{
		DisplayName: "V.M.Sundar",
		Title:       "Business Development Manager",
		Company:     "PH TRADING W.L.L.",
		Address:     []string{"P.O. BOX 815, MANAMA,", "KINGDOM OF BAHRAIN"},
		Mobile:      "+973 38260157",
		Office:      "+973 17587654",
		Fax:         "+973 17564456",
		Email:       "vm.sundar@platinumholdings.com.bh",
		Aliases:     []string{"V.M. Sundar", "V.M.Sundar", "VM Sundar", "Sundar"},
	},
	{
		DisplayName: "RAMYA BASKARAN",
		Company:     "PH TRADING W.L.L",
		Address:     []string{"P.O. BOX 815, MANAMA,", "KINGDOM OF BAHRAIN"},
		Mobile:      "+973 38890513",
		Office:      "+973 17587654",
		Fax:         "+973 17564456",
		Email:       "support@platinumholdings.com.bh",
		Aliases:     []string{"Ramya", "Ramya Baskaran", "RAMYA BASKARAN", "Support"},
	},
	{
		DisplayName: "STANLEY VAZ",
		Title:       "Chief Operating Officer",
		Company:     "PH TRADING W.L.L.",
		Address:     []string{"P.O. BOX 815, MANAMA,", "KINGDOM OF BAHRAIN"},
		Mobile:      "+973 36623662",
		Office:      "+973 17587654",
		Fax:         "+973 17564456",
		Aliases:     []string{"Stanley", "Stanley Vaz", "STANLEY VAZ"},
	},
	{
		DisplayName: "Abhie Kore",
		Title:       "General Manager",
		Company:     "PH TRADING W.L.L.",
		Address:     []string{"P.O. BOX 815, MANAMA,", "KINGDOM OF BAHRAIN"},
		Mobile:      "+973 36771513",
		Office:      "+973 17587654",
		Fax:         "+973 17564456",
		Aliases:     []string{"Abhie", "Abhie Kore"},
	},
}

func normalizePHSignatureKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func resolvePHOfferSignatureBlock(preparedBy string) *PHOfferSignatureBlock {
	key := normalizePHSignatureKey(preparedBy)
	if key == "" {
		return nil
	}
	for i := range phOfferSignatureBlocks {
		block := &phOfferSignatureBlocks[i]
		if normalizePHSignatureKey(block.DisplayName) == key {
			return block
		}
		for _, alias := range block.Aliases {
			if normalizePHSignatureKey(alias) == key {
				return block
			}
		}
	}
	return nil
}

func defaultPHOfferSignatureNames() []string {
	names := make([]string, 0, len(phOfferSignatureBlocks))
	for _, block := range phOfferSignatureBlocks {
		names = append(names, block.DisplayName)
	}
	return names
}

func offerIssuerDisplayName(preparedBy string) string {
	if block := resolvePHOfferSignatureBlock(preparedBy); block != nil {
		return block.DisplayName
	}
	return strings.TrimSpace(preparedBy)
}

type phSignaturePDFLine struct {
	Text string
	Bold bool
}

func phOfferSignaturePDFLines(block PHOfferSignatureBlock, includeRegards bool) []phSignaturePDFLine {
	lines := []phSignaturePDFLine{}
	if includeRegards {
		lines = append(lines,
			phSignaturePDFLine{Text: "Best Regards,"},
			phSignaturePDFLine{},
		)
	}
	lines = append(lines, phSignaturePDFLine{Text: block.DisplayName, Bold: true})
	for _, value := range []string{block.Title, block.Company} {
		if strings.TrimSpace(value) != "" {
			lines = append(lines, phSignaturePDFLine{Text: value})
		}
	}
	for _, line := range block.Address {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, phSignaturePDFLine{Text: line})
		}
	}
	if strings.TrimSpace(block.Mobile) != "" {
		lines = append(lines, phSignaturePDFLine{Text: "Mob: " + block.Mobile})
	}
	if strings.TrimSpace(block.Office) != "" {
		lines = append(lines, phSignaturePDFLine{Text: "Office: " + block.Office})
	}
	if strings.TrimSpace(block.Fax) != "" {
		lines = append(lines, phSignaturePDFLine{Text: "Fax: " + block.Fax})
	}
	if strings.TrimSpace(block.Email) != "" {
		lines = append(lines, phSignaturePDFLine{Text: block.Email})
	}
	return lines
}

func drawPHSignaturePDFLines(pdf *gofpdf.Fpdf, x, y, width, lineHeight, fontSize float64, block PHOfferSignatureBlock, includeRegards bool) float64 {
	currentY := y
	for _, line := range phOfferSignaturePDFLines(block, includeRegards) {
		text := strings.TrimSpace(line.Text)
		if text == "" {
			currentY += lineHeight
			continue
		}
		style := ""
		if line.Bold {
			style = "B"
		}
		pdf.SetFont("Helvetica", style, fontSize)
		pdf.SetTextColor(60, 60, 60)
		if line.Bold {
			pdf.SetTextColor(29, 29, 31)
		}
		pdf.SetXY(x, currentY)
		pdf.MultiCell(width, lineHeight, sanitizeForPDF(text), "", "L", false)
		currentY = pdf.GetY()
	}
	return currentY
}

func (a *App) resolvePreparedBySignatureBlock(preparedBy string) PHOfferSignatureBlock {
	preparedBy = strings.TrimSpace(preparedBy)
	block := PHOfferSignatureBlock{
		DisplayName: preparedBy,
		Company:     "PH TRADING W.L.L",
		Address:     []string{"P.O. BOX 815, MANAMA,", "KINGDOM OF BAHRAIN"},
		Office:      "+973 17587654",
		Fax:         "+973 17564456",
		Email:       "sales@phtrading.com",
	}
	if staticBlock := resolvePHOfferSignatureBlock(preparedBy); staticBlock != nil {
		block = *staticBlock
	}

	if a == nil || a.db == nil || preparedBy == "" {
		return block
	}

	var employees []Employee
	if err := a.db.Where("deleted_at IS NULL AND is_active = ?", true).Find(&employees).Error; err != nil {
		return block
	}
	target := normalizePHSignatureKey(preparedBy)
	for _, employee := range employees {
		if normalizePHSignatureKey(employee.FullName) != target &&
			normalizePHSignatureKey(employee.PreferredName) != target &&
			normalizePHSignatureKey(employee.EmployeeCode) != target {
			continue
		}
		if strings.TrimSpace(employee.FullName) != "" {
			block.DisplayName = strings.TrimSpace(employee.FullName)
		}
		if strings.TrimSpace(employee.JobTitle) != "" {
			block.Title = strings.TrimSpace(employee.JobTitle)
		}
		if strings.TrimSpace(employee.Phone) != "" {
			block.Mobile = strings.TrimSpace(employee.Phone)
		}
		if strings.TrimSpace(employee.Email) != "" {
			block.Email = strings.TrimSpace(employee.Email)
		}
		return block
	}

	return block
}

func (a *App) resolveDocumentSignerName(candidates ...string) string {
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if a != nil && a.db != nil {
			var user User
			if err := a.db.First(&user, "id = ?", candidate).Error; err == nil {
				for _, value := range []string{user.DisplayName, user.FullName, user.Username, user.Email} {
					if strings.TrimSpace(value) != "" {
						return strings.TrimSpace(value)
					}
				}
			}
		}
		return candidate
	}
	if a != nil {
		return strings.TrimSpace(a.getCurrentUserDisplayName())
	}
	return ""
}

func drawPHOfferSignatureBlock(pdf *gofpdf.Fpdf, preparedBy string, fallbackPhone string) {
	preparedBy = strings.TrimSpace(preparedBy)
	if preparedBy == "" {
		preparedBy = "Sales Team"
	}

	if pdf.GetY() > 205 {
		pdf.AddPage()
		pdf.SetY(52)
	}

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(40, 40, 40)
	pdf.Cell(0, 5, "Best Regards,")
	pdf.Ln(8)

	block := resolvePHOfferSignatureBlock(preparedBy)
	if block == nil {
		pdf.SetFont("Helvetica", "B", 10)
		pdf.SetTextColor(29, 29, 31)
		pdf.Cell(0, 5, sanitizeForPDF(preparedBy))
		pdf.Ln(5)

		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(60, 60, 60)
		pdf.Cell(0, 5, "PH TRADING WLL")
		pdf.Ln(5)
		if strings.TrimSpace(fallbackPhone) != "" {
			pdf.Cell(0, 5, "Mob: "+sanitizeForPDF(fallbackPhone))
			pdf.Ln(5)
		}
		pdf.Cell(0, 5, "Email: sales@phtrading.com")
		pdf.Ln(8)
		return
	}

	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 5, sanitizeForPDF(block.DisplayName))
	pdf.Ln(5)

	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)
	if strings.TrimSpace(block.Title) != "" {
		pdf.Cell(0, 5, sanitizeForPDF(block.Title))
		pdf.Ln(5)
	}
	if strings.TrimSpace(block.Company) != "" {
		pdf.Cell(0, 5, sanitizeForPDF(block.Company))
		pdf.Ln(5)
	}
	for _, line := range block.Address {
		if strings.TrimSpace(line) == "" {
			continue
		}
		pdf.Cell(0, 5, sanitizeForPDF(line))
		pdf.Ln(5)
	}
	if strings.TrimSpace(block.Mobile) != "" {
		pdf.Cell(0, 5, "Mob: "+sanitizeForPDF(block.Mobile))
		pdf.Ln(5)
	}
	if strings.TrimSpace(block.Office) != "" {
		pdf.Cell(0, 5, "Office: "+sanitizeForPDF(block.Office))
		pdf.Ln(5)
	}
	if strings.TrimSpace(block.Fax) != "" {
		pdf.Cell(0, 5, "Fax: "+sanitizeForPDF(block.Fax))
		pdf.Ln(5)
	}
	if strings.TrimSpace(block.Email) != "" {
		pdf.Cell(0, 5, sanitizeForPDF(block.Email))
		pdf.Ln(5)
	}
	pdf.Ln(3)
}
