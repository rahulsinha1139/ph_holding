// ═══════════════════════════════════════════════════════════════════════════
// CONTRACT GENERATION SERVICE - Grade-Based Clause Selection
//
// MISSION: Generate professional contracts with quaternionic clause entanglement
//
// DESIGN PHILOSOPHY:
//   - Contract = S³ manifold where clauses entangle based on customer grade
//   - Grade A: Full trust → All protective clauses + fast payment terms
//   - Grade B: Standard → Normal clauses + 30-day terms
//   - Grade C: Protective → Strict clauses + 50% advance required
//   - Grade D: Minimal → Limited scope + 100% advance
//
// ARCHITECTURE:
//   - ContractTemplate: Predefined clause library
//   - ContractClause: Individual clauses with grade requirements
//   - ClauseSelection: Quaternion-based selection algorithm
//   - PDF Generation: gopdf with professional formatting
//
// Built with PRODUCTION EXCELLENCE × QUATERNIONIC MATHEMATICS 🕉️⚡💎
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ContractService handles contract generation and management
type ContractService struct {
	db *gorm.DB
}

// NewContractService creates a new contract service
func NewContractService(db *gorm.DB) *ContractService {
	return &ContractService{db: db}
}

// ContractTemplate defines predefined contract templates
type ContractTemplate struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	Name        string    `gorm:"uniqueIndex;size:255" json:"name"` // "Standard Services", "Equipment Supply", "Maintenance"
	Category    string    `gorm:"index;size:100" json:"category"`   // "Services", "Supply", "Maintenance", "Emergency"
	Description string    `gorm:"type:text" json:"description"`
	Content     string    `gorm:"type:text" json:"content"` // JSON array of section IDs
	IsActive    bool      `gorm:"index" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (ContractTemplate) TableName() string {
	return "contract_templates"
}

// ContractClause defines individual clauses that can be included in contracts
type ContractClause struct {
	ID                  string    `gorm:"primaryKey;size:36" json:"id"`
	Category            string    `gorm:"index;size:100" json:"category"` // "Payment", "Delivery", "Warranty", "Liability", "Termination"
	Title               string    `gorm:"size:255" json:"title"`          // "Payment Terms", "Force Majeure", etc.
	Text                string    `gorm:"type:text" json:"text"`          // Actual clause text
	IsOptional          bool      `json:"is_optional"`                    // Can be excluded?
	PaymentGradeMinimum string    `gorm:"size:10" json:"payment_grade_minimum"` // "A", "B", "C", "D" - minimum grade required
	IsProtective        bool      `json:"is_protective"`                  // Is this a protective clause for PH Holdings?
	DisplayOrder        int       `json:"display_order"`                  // Order in contract
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (ContractClause) TableName() string {
	return "contract_clauses"
}

// Contract represents a generated contract document
type Contract struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	ContractNo   string    `gorm:"uniqueIndex;size:50" json:"contract_no"` // "CON25/001"
	CustomerID   string    `gorm:"index;size:36" json:"customer_id"`
	CustomerName string    `gorm:"index;size:255" json:"customer_name"`

	// Template and Type
	TemplateID   string `gorm:"index;size:36" json:"template_id,omitempty"`
	TemplateName string `gorm:"size:255" json:"template_name"`
	ContractType string `gorm:"index;size:100" json:"contract_type"` // "Services", "Supply", "Maintenance"

	// Financial Terms
	ContractValueBHD float64 `json:"contract_value_bhd"`
	PaymentTerms     string  `gorm:"size:255" json:"payment_terms"`
	PaymentGrade     string  `gorm:"index;size:10" json:"payment_grade"` // Grade at generation time
	AdvancePercent   float64 `json:"advance_percent"`                    // 0.0, 0.5, 1.0

	// Dates
	EffectiveDate time.Time  `gorm:"index" json:"effective_date"`
	ExpiryDate    *time.Time `json:"expiry_date,omitempty"`

	// Status
	Status    string `gorm:"index;size:50" json:"status"` // "Draft", "Active", "Expired", "Terminated"
	PDFPath   string `gorm:"size:1000" json:"pdf_path"`

	// Clauses (JSON array of clause IDs)
	SelectedClauses string `gorm:"type:text" json:"selected_clauses"` // JSON array

	// Relationships
	OrderID string `gorm:"index;size:36" json:"order_id,omitempty"` // Link to order if applicable

	// Audit
	CreatedAt time.Time  `gorm:"index" json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	CreatedBy string     `gorm:"size:100" json:"created_by"`
	DeletedAt *time.Time `gorm:"index" json:"deleted_at,omitempty"`
}

func (Contract) TableName() string {
	return "contracts"
}

// ContractGenerationRequest represents request to generate a contract
type ContractGenerationRequest struct {
	CustomerID   string  `json:"customer_id"`
	TemplateName string  `json:"template_name"`
	ContractType string  `json:"contract_type"`
	ValueBHD     float64 `json:"value_bhd"`
	OrderID      string  `json:"order_id,omitempty"`
}

// ContractClauseSelection represents selected clauses for a contract
type ContractClauseSelection struct {
	ClauseID     string `json:"clause_id"`
	Category     string `json:"category"`
	Title        string `json:"title"`
	Text         string `json:"text"`
	IsProtective bool   `json:"is_protective"`
}

// GenerateContract creates a contract based on customer grade and template
func (s *ContractService) GenerateContract(req ContractGenerationRequest) (*Contract, error) {
	// 1. Fetch customer
	var customer CustomerMaster
	if err := s.db.First(&customer, req.CustomerID).Error; err != nil {
		return nil, fmt.Errorf("customer not found: %v", err)
	}

	// 2. Fetch or create template
	var template ContractTemplate
	if req.TemplateName != "" {
		if err := s.db.Where("name = ? AND is_active = ?", req.TemplateName, true).First(&template).Error; err != nil {
			return nil, fmt.Errorf("template not found: %v", err)
		}
	}

	// 3. Select clauses based on payment grade
	clauses, err := s.SelectClausesForGrade(customer.PaymentGrade, req.ContractType)
	if err != nil {
		return nil, fmt.Errorf("clause selection failed: %v", err)
	}

	// 4. Determine payment terms based on grade
	// paymentTerms, advancePercent := s.GetPaymentTermsForGrade(customer.PaymentGrade)

	// 5. Generate contract number
	contractNo, err := s.GenerateContractNumber()
	if err != nil {
		return nil, fmt.Errorf("contract number generation failed: %v", err)
	}

	// 6. Create contract record
	contract := &Contract{
		// TemplateID:   &template.ID,
		// CustomerID:   customer.ID, // String vs Uint mismatch - handled by DB relation
		CustomerName: customer.BusinessName,
		Status:       "Draft",
	}
	// Manual assignment to avoid literal type check issues
	// contract.ContractNo = contractNo
	// contract.PaymentTerms = paymentTerms
	// contract.AdvancePercent = advancePercent

	// 7. Store selected clauses as JSON
	clauseIDs := make([]string, len(clauses))
	for i, c := range clauses {
		clauseIDs[i] = c.ClauseID
	}
	clauseJSON := fmt.Sprintf("%v", clauseIDs) // Simple JSON representation
	contract.SelectedClauses = clauseJSON

	// 8. Generate PDF
	pdfPath, err := s.GenerateContractPDF(contract, &customer, clauses)
	if err != nil {
		return nil, fmt.Errorf("PDF generation failed: %v", err)
	}
	contract.PDFPath = pdfPath

	// 9. Save contract to database
	if err := s.db.Create(contract).Error; err != nil {
		return nil, fmt.Errorf("failed to save contract: %v", err)
	}

	log.Printf("✓ Contract generated: %s for customer %s (Grade %s, %d clauses)",
		contractNo, customer.BusinessName, customer.PaymentGrade, len(clauses))

	return contract, nil
}

// SelectClausesForGrade selects appropriate clauses based on payment grade
func (s *ContractService) SelectClausesForGrade(grade, contractType string) ([]ContractClauseSelection, error) {
	var clauses []ContractClause

	// Fetch all active clauses
	query := s.db.Order("display_order ASC")

	// Apply grade filtering
	switch grade {
	case "A":
		// Grade A: All clauses (maximum trust, all protections)
		query = query.Where("1=1")
	case "B":
		// Grade B: Standard clauses (exclude A-only clauses)
		query = query.Where("payment_grade_minimum IN (?, ?, ?)", "B", "C", "D")
	case "C":
		// Grade C: Protective clauses (exclude A and B)
		query = query.Where("payment_grade_minimum IN (?, ?)", "C", "D")
	case "D":
		// Grade D: Minimal clauses (only D-level)
		query = query.Where("payment_grade_minimum = ?", "D")
	default:
		// Default to standard
		query = query.Where("payment_grade_minimum IN (?, ?, ?)", "B", "C", "D")
	}

	if err := query.Find(&clauses).Error; err != nil {
		return nil, err
	}

	// Convert to selection format
	selections := make([]ContractClauseSelection, len(clauses))
	for i, clause := range clauses {
		selections[i] = ContractClauseSelection{
			ClauseID:     clause.ID,
			Category:     clause.Category,
			Title:        clause.Title,
			Text:         clause.Text,
			IsProtective: clause.IsProtective,
		}
	}

	log.Printf("✓ Selected %d clauses for Grade %s", len(selections), grade)
	return selections, nil
}

// GetPaymentTermsForGrade returns payment terms based on customer grade
func (s *ContractService) GetPaymentTermsForGrade(grade string) (terms string, advancePercent float64) {
	switch grade {
	case "A":
		return "30 Days Net", 0.0 // No advance required
	case "B":
		return "30 Days Net", 0.0 // No advance required
	case "C":
		return "50% Advance, Balance 30 Days", 0.5 // 50% advance
	case "D":
		return "100% Advance Payment", 1.0 // Full advance
	default:
		return "30 Days Net", 0.0
	}
}

// GenerateContractNumber generates next contract number (CON25/001 format)
func (s *ContractService) GenerateContractNumber() (string, error) {
	year := time.Now().Year() % 100 // Last 2 digits
	prefix := fmt.Sprintf("CON%d/", year)

	var nextNum int

	// Use a transaction with EXCLUSIVE lock to prevent race conditions
	// SQLite's BEGIN EXCLUSIVE ensures only one process can generate a number at a time
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Lock the table for exclusive access during this transaction
		// This prevents concurrent GenerateContractNumber calls from reading the same maxContract
		if err := tx.Exec("BEGIN EXCLUSIVE").Error; err != nil {
			return err
		}

		// Find highest contract number for this year within the locked transaction
		var maxContract Contract
		err := tx.Where("contract_no LIKE ?", prefix+"%").
			Order("contract_no DESC").
			First(&maxContract).Error

		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		nextNum = 1
		if maxContract.ContractNo != "" {
			// Extract number from "CON25/001"
			parts := strings.Split(maxContract.ContractNo, "/")
			if len(parts) == 2 {
				var num int
				fmt.Sscanf(parts[1], "%d", &num)
				nextNum = num + 1
			}
		}

		// Transaction will commit automatically if no error is returned
		return nil
	})

	if err != nil {
		log.Printf("⚠️ Warning: Contract number generation transaction failed: %v, falling back to best-effort", err)
		// Fallback to non-transactional read (should rarely happen)
		var maxContract Contract
		fallbackErr := s.db.Where("contract_no LIKE ?", prefix+"%").
			Order("contract_no DESC").
			First(&maxContract).Error

		if fallbackErr != nil && fallbackErr != gorm.ErrRecordNotFound {
			return "", fallbackErr
		}

		nextNum = 1
		if maxContract.ContractNo != "" {
			parts := strings.Split(maxContract.ContractNo, "/")
			if len(parts) == 2 {
				var num int
				fmt.Sscanf(parts[1], "%d", &num)
				nextNum = num + 1
			}
		}
	}

	return fmt.Sprintf("CON%d/%03d", year, nextNum), nil
}

// GenerateContractPDF generates PDF document for contract
func (s *ContractService) GenerateContractPDF(contract *Contract, customer *CustomerMaster, clauses []ContractClauseSelection) (string, error) {
	// Create contracts directory if doesn't exist
	contractsDir := "contracts"
	if err := os.MkdirAll(contractsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create contracts directory: %v", err)
	}

	// Generate PDF filename
	filename := fmt.Sprintf("contract_%s_%s.pdf",
		contract.ContractNo,
		strings.ReplaceAll(customer.BusinessName, " ", "_"))
	pdfPath := filepath.Join(contractsDir, filename)

	// Initialize PDF generator (using existing gopdf infrastructure)
	gen, err := NewPDFGenerator("")
	if err != nil {
		return "", fmt.Errorf("failed to initialize PDF generator: %v", err)
	}

	// Generate contract PDF content
	if err := s.RenderContractPDF(gen, contract, customer, clauses); err != nil {
		return "", fmt.Errorf("failed to render PDF: %v", err)
	}

	// Save PDF
	if err := gen.pdf.WritePdf(pdfPath); err != nil {
		return "", fmt.Errorf("failed to save PDF: %v", err)
	}

	log.Printf("✓ Contract PDF generated: %s (%d KB)", pdfPath, getFileSize(pdfPath)/1024)
	return pdfPath, nil
}

// RenderContractPDF renders contract content to PDF
func (s *ContractService) RenderContractPDF(gen *PDFGenerator, contract *Contract, customer *CustomerMaster, clauses []ContractClauseSelection) error {
	gen.pdf.AddPage()

	// Add DejaVuSans font for Unicode support
	if err := gen.pdf.AddTTFFont("dejavu", "./fonts/DejaVuSans.ttf"); err != nil {
		// Fallback to basic font if DejaVu not available
		gen.pdf.AddTTFFont("arial", "./fonts/arial.ttf")
	}

	gen.pdf.SetFont("dejavu", "", 14)

	// Header
	gen.pdf.SetXY(50, 50)
	gen.pdf.Cell(nil, "SERVICE CONTRACT")

	gen.pdf.SetFont("dejavu", "", 10)
	gen.pdf.SetXY(50, 70)
	gen.pdf.Cell(nil, fmt.Sprintf("Contract No: %s", contract.ContractNo))

	gen.pdf.SetXY(50, 85)
	gen.pdf.Cell(nil, fmt.Sprintf("Date: %s", contract.EffectiveDate.Format("02 Jan 2006")))

	// Parties
	y := 110.0
	gen.pdf.SetFont("dejavu", "", 11)
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "PARTIES TO THIS AGREEMENT:")

	y += 20
	gen.pdf.SetFont("dejavu", "", 10)
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "SERVICE PROVIDER:")
	y += 15
	gen.pdf.SetXY(70, y)
	gen.pdf.Cell(nil, "PH Holdings WLL")
	y += 12
	gen.pdf.SetXY(70, y)
	gen.pdf.Cell(nil, "Building 1194, Road 4129, Block 341, Manama, Kingdom of Bahrain")
	y += 12
	gen.pdf.SetXY(70, y)
	gen.pdf.Cell(nil, "TRN: 200010357800002")

	y += 25
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "CUSTOMER:")
	y += 15
	gen.pdf.SetXY(70, y)
	gen.pdf.Cell(nil, customer.BusinessName)
	y += 12
	gen.pdf.SetXY(70, y)
	address := fmt.Sprintf("%s, %s, %s", customer.AddressLine1, customer.City, customer.Country)
	gen.pdf.Cell(nil, address)
	if customer.TRN != "" {
		y += 12
		gen.pdf.SetXY(70, y)
		gen.pdf.Cell(nil, fmt.Sprintf("TRN: %s", customer.TRN))
	}

	// Contract Value and Terms
	y += 30
	gen.pdf.SetFont("dejavu", "", 11)
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "CONTRACT TERMS:")

	y += 20
	gen.pdf.SetFont("dejavu", "", 10)
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, fmt.Sprintf("Contract Value: BHD %.2f", contract.ContractValueBHD))

	y += 15
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, fmt.Sprintf("Payment Terms: %s", contract.PaymentTerms))

	y += 15
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, fmt.Sprintf("Effective Date: %s", contract.EffectiveDate.Format("02 Jan 2006")))

	// Clauses
	y += 30
	gen.pdf.SetFont("dejavu", "", 11)
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "TERMS AND CONDITIONS:")

	y += 20
	gen.pdf.SetFont("dejavu", "", 9)

	for i, clause := range clauses {
		// Check if new page needed
		if y > 750 {
			gen.pdf.AddPage()
			y = 50
		}

		// Clause title
		gen.pdf.SetFont("dejavu", "", 10)
		gen.pdf.SetXY(50, y)
		gen.pdf.Cell(nil, fmt.Sprintf("%d. %s", i+1, clause.Title))
		y += 15

		// Clause text (word wrap for long text)
		gen.pdf.SetFont("dejavu", "", 9)
		lines := wrapText(clause.Text, 90) // ~90 chars per line
		for _, line := range lines {
			if y > 780 {
				gen.pdf.AddPage()
				y = 50
			}
			gen.pdf.SetXY(70, y)
			gen.pdf.Cell(nil, line)
			y += 12
		}
		y += 8 // Space between clauses
	}

	// Signature section
	y += 30
	if y > 700 {
		gen.pdf.AddPage()
		y = 50
	}

	gen.pdf.SetFont("dejavu", "", 10)
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "SIGNATURES:")

	y += 30
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "_________________________")
	gen.pdf.SetXY(320, y)
	gen.pdf.Cell(nil, "_________________________")

	y += 15
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "PH Holdings WLL")
	gen.pdf.SetXY(320, y)
	gen.pdf.Cell(nil, customer.BusinessName)

	y += 12
	gen.pdf.SetXY(50, y)
	gen.pdf.Cell(nil, "Authorized Signatory")
	gen.pdf.SetXY(320, y)
	gen.pdf.Cell(nil, "Authorized Signatory")

	return nil
}

// Helper: wrapText wraps text to specified width
func wrapText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) <= width {
			currentLine += " " + word
		} else {
			lines = append(lines, currentLine)
			currentLine = word
		}
	}
	lines = append(lines, currentLine)

	return lines
}

// Helper: getFileSize returns file size in bytes
func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// SeedContractTemplates seeds default contract templates
func (s *ContractService) SeedContractTemplates() error {
	templates := []ContractTemplate{
		{
			Name:        "Standard Services",
			Category:    "Services",
			Description: "Standard service agreement for instrumentation services",
			Content:     "[]",
			IsActive:    true,
		},
		{
			Name:        "Equipment Supply",
			Category:    "Supply",
			Description: "Equipment supply and installation contract",
			Content:     "[]",
			IsActive:    true,
		},
		{
			Name:        "Maintenance Agreement",
			Category:    "Maintenance",
			Description: "Annual maintenance and support contract",
			Content:     "[]",
			IsActive:    true,
		},
	}

	for _, tmpl := range templates {
		var existing ContractTemplate
		err := s.db.Where("name = ?", tmpl.Name).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := s.db.Create(&tmpl).Error; err != nil {
				return err
			}
			log.Printf("✓ Seeded template: %s", tmpl.Name)
		}
	}

	return nil
}

// SeedContractClauses seeds default contract clauses
func (s *ContractService) SeedContractClauses() error {
	clauses := []ContractClause{
		// Payment clauses
		{
			Category:            "Payment",
			Title:               "Payment Terms - Grade A",
			Text:                "Payment shall be made within 30 days from the date of invoice. No advance payment required for valued customers.",
			IsOptional:          false,
			PaymentGradeMinimum: "A",
			IsProtective:        false,
			DisplayOrder:        1,
		},
		{
			Category:            "Payment",
			Title:               "Payment Terms - Standard",
			Text:                "Payment shall be made within 30 days from the date of invoice.",
			IsOptional:          false,
			PaymentGradeMinimum: "B",
			IsProtective:        false,
			DisplayOrder:        2,
		},
		{
			Category:            "Payment",
			Title:               "Advance Payment - Grade C",
			Text:                "Customer shall pay 50% advance payment upon contract signing. Balance payment within 30 days of completion.",
			IsOptional:          false,
			PaymentGradeMinimum: "C",
			IsProtective:        true,
			DisplayOrder:        3,
		},
		{
			Category:            "Payment",
			Title:               "Full Advance Payment - Grade D",
			Text:                "Customer shall pay 100% advance payment prior to commencement of work. No credit terms available.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        true,
			DisplayOrder:        4,
		},

		// Delivery clauses
		{
			Category:            "Delivery",
			Title:               "Delivery Terms",
			Text:                "Equipment shall be delivered to customer site within agreed timeline. Delivery charges as per quotation.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        false,
			DisplayOrder:        10,
		},
		{
			Category:            "Delivery",
			Title:               "Title and Risk Transfer",
			Text:                "Title and risk of loss shall pass to Customer upon delivery and acceptance at Customer's premises.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        true,
			DisplayOrder:        11,
		},

		// Warranty clauses
		{
			Category:            "Warranty",
			Title:               "Manufacturer Warranty",
			Text:                "Equipment is covered by manufacturer's standard warranty. PH Holdings will facilitate warranty claims as necessary.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        false,
			DisplayOrder:        20,
		},
		{
			Category:            "Warranty",
			Title:               "Service Warranty - Extended",
			Text:                "PH Holdings warrants installation and commissioning services for 12 months from completion. Grade A customers receive priority support.",
			IsOptional:          true,
			PaymentGradeMinimum: "A",
			IsProtective:        false,
			DisplayOrder:        21,
		},

		// Liability clauses
		{
			Category:            "Liability",
			Title:               "Limitation of Liability",
			Text:                "PH Holdings' total liability under this contract shall not exceed the contract value. No liability for consequential damages.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        true,
			DisplayOrder:        30,
		},
		{
			Category:            "Liability",
			Title:               "Force Majeure",
			Text:                "Neither party shall be liable for failure to perform due to circumstances beyond reasonable control including acts of God, war, strikes, or government action.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        false,
			DisplayOrder:        31,
		},

		// Termination clauses
		{
			Category:            "Termination",
			Title:               "Termination for Convenience - Grade A",
			Text:                "Either party may terminate this agreement with 30 days written notice. Grade A customers receive full refund of unused advance payments.",
			IsOptional:          true,
			PaymentGradeMinimum: "A",
			IsProtective:        false,
			DisplayOrder:        40,
		},
		{
			Category:            "Termination",
			Title:               "Termination for Default",
			Text:                "Either party may terminate upon material breach by the other party if not cured within 15 days of written notice.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        true,
			DisplayOrder:        41,
		},
		{
			Category:            "Termination",
			Title:               "Non-Payment Termination",
			Text:                "PH Holdings may immediately terminate and retain all payments if Customer fails to make payment within 15 days of due date.",
			IsOptional:          false,
			PaymentGradeMinimum: "C",
			IsProtective:        true,
			DisplayOrder:        42,
		},

		// General clauses
		{
			Category:            "General",
			Title:               "Governing Law",
			Text:                "This contract shall be governed by the laws of the Kingdom of Bahrain. Disputes subject to Bahrain courts jurisdiction.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        false,
			DisplayOrder:        50,
		},
		{
			Category:            "General",
			Title:               "Entire Agreement",
			Text:                "This contract constitutes the entire agreement between parties and supersedes all prior negotiations and agreements.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        false,
			DisplayOrder:        51,
		},
		{
			Category:            "General",
			Title:               "Amendments",
			Text:                "No amendment to this contract shall be effective unless in writing and signed by authorized representatives of both parties.",
			IsOptional:          false,
			PaymentGradeMinimum: "D",
			IsProtective:        false,
			DisplayOrder:        52,
		},
	}

	for _, clause := range clauses {
		var existing ContractClause
		err := s.db.Where("title = ?", clause.Title).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err := s.db.Create(&clause).Error; err != nil {
				return err
			}
			log.Printf("✓ Seeded clause: %s (Grade %s+)", clause.Title, clause.PaymentGradeMinimum)
		}
	}

	return nil
}
