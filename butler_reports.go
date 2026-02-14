// ═══════════════════════════════════════════════════════════════════════════
// BUTLER INTELLIGENCE REPORTS - AI-POWERED PDF GENERATION
//
// MISSION: Generate detailed business intelligence reports via Mistral AI
//          combining live database context with AI analysis into professional PDFs
//
// REPORT TYPES:
//   - customer: Customer analysis (specific or portfolio)
//   - financial: Financial health and cash flow
//   - risk: Risk assessment and overdue analysis
//   - operations: Pipeline and fulfillment status
//   - supplier: Supplier performance and procurement
//   - executive: Executive summary (all domains)
//
// PIPELINE:
//   1. Classify report type from user request
//   2. Build rich context (GORM queries)
//   3. Call Mistral for structured analysis
//   4. Generate multi-page PDF (cover + data + analysis)
//   5. Return file path for download
//
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
)

// ============================================================================
// REPORT GENERATION ENTRY POINT
// ============================================================================

// GenerateButlerReport creates a detailed PDF intelligence report
// reportType: customer, financial, risk, operations, supplier, executive
// query: optional context (e.g., customer name, specific question)
func (a *App) GenerateButlerReport(reportType string, query string) (string, error) {
	log.Printf("📊 Generating intelligence report: type=%s query=%q", reportType, query)

	// P1 FIX: Rate limiting for report generation (3 per minute per user)
	if GlobalRateLimiter != nil {
		rateLimitKey := "report:" + a.getCurrentUserID()
		if !GlobalRateLimiter.Allow(rateLimitKey, RateLimitConfig.ReportGenerationPerMinute, 1*time.Minute/time.Duration(RateLimitConfig.ReportGenerationPerMinute)) {
			log.Printf("🚫 Rate limit exceeded for report generation by user %s", a.getCurrentUserID())
			return "", fmt.Errorf("too many report requests, please wait a moment before generating another report")
		}
	}

	// 0. RBAC: Check if report type requires finance:view permission
	financialReportTypes := []string{"financial", "risk", "executive"}
	requiresFinanceAccess := false
	for _, ft := range financialReportTypes {
		if reportType == ft {
			requiresFinanceAccess = true
			break
		}
	}

	if requiresFinanceAccess {
		userRole := a.GetCurrentUserRole()
		hasFinanceAccess := a.CheckPermissionByRole(userRole, "finance:view")

		if !hasFinanceAccess {
			log.Printf("🔒 Butler report generation blocked: type=%s user_role=%s lacks finance:view", reportType, userRole)
			return "", fmt.Errorf("permission denied: %s reports require manager or admin privileges (finance:view permission)", reportType)
		}
	}

	// 1. Build intent from report type
	intent := Intent{
		Domain:     reportType,
		EntityName: query,
		Confidence: 1.0,
		IsComplex:  true, // Reports always use large model
	}

	// Map report types to intent domains
	switch reportType {
	case "executive":
		intent.Domain = "general"
	case "risk":
		intent.Domain = "risk"
	case "customer":
		intent.Domain = "customer"
		intent.EntityName = query
	case "supplier":
		intent.Domain = "supplier"
		intent.EntityName = query
	case "financial":
		intent.Domain = "financial"
	case "operations":
		intent.Domain = "operations"
	}

	// 2. Build rich context
	context := a.buildIntentContext(intent)

	// 3. Call Mistral for structured analysis
	contextJSON, _ := json.MarshalIndent(context, "", "  ")
	reportPrompt := buildReportPrompt(reportType, query, string(contextJSON))

	aiAnalysis, err := callMistral(mistralModelLarge, reportPrompt, query)
	if err != nil {
		log.Printf("❌ Mistral report analysis failed: %v", err)
		// Generate report without AI analysis
		aiAnalysis = "AI analysis unavailable. Report contains raw data only."
	}

	// 4. Generate PDF
	filePath, err := a.generateIntelligenceReportPDF(reportType, query, context, aiAnalysis)
	if err != nil {
		return "", fmt.Errorf("PDF generation failed: %w", err)
	}

	log.Printf("✅ Intelligence report generated: %s", filePath)
	return filePath, nil
}

// ============================================================================
// REPORT PROMPT BUILDER
// ============================================================================

func buildReportPrompt(reportType, query, contextJSON string) string {
	return fmt.Sprintf(`You are a senior management consultant preparing a business intelligence brief for the executive team at PH Trading WLL, a process instrumentation and industrial automation company based in Bahrain.

REPORT FOCUS: %s
SPECIFIC SUBJECT: %s

LIVE BUSINESS DATA:
%s

DELIVERABLE:
Produce a concise, insight-driven management brief structured as follows:

1. EXECUTIVE SUMMARY
Two to three sentences capturing the headline finding and its business implication.

2. KEY FIGURES
The 6-8 most decision-relevant metrics, presented as a clean bullet list with exact values.

3. ANALYSIS
Three to five paragraphs of substantive analysis. Organize by business theme (e.g., Revenue Performance, Working Capital, Portfolio Concentration, Collection Efficiency). Lead each paragraph with the insight, then support with data.

4. RISK FACTORS
Identify 3-5 specific business risks grounded in the data. For each, state the risk, the evidence, and the potential impact.

5. RECOMMENDATIONS
Provide 3-5 specific, actionable management actions. Each should state what to do, why, and the expected benefit.

6. OUTLOOK
One paragraph on the near-term business trajectory based on current trends.

WRITING STANDARDS:
- Write in clear, direct business English. No jargon, no technical terms, no mathematical language.
- Use exact figures from the data. Currency is BHD (Bahraini Dinar, 3 decimal places).
- Lead with insights, not data dumps. Every number should support a point.
- Tone: authoritative, concise, suitable for a board presentation.
- Do NOT use asterisks, markdown formatting, or any special characters for emphasis.
- Use "- " for bullet points.
- Use numbered sections as shown above.
- If data is insufficient for a conclusion, state what additional data would be needed.
- Do NOT use words like "regime", "stabilization phase", "exploration", "optimization" or any systems/mathematical terminology. Use plain business language only.

Generate the report now.`, reportType, query, contextJSON)
}

// ============================================================================
// PDF GENERATION
// ============================================================================

// letterheadImagePath returns the path to the PH Trading letterhead template
// First checks database (synced via Supabase), then falls back to file system
func (a *App) letterheadImagePath() string {
	// PRIMARY: Try to get from database (synced via Supabase)
	// This eliminates the need to include asset folders in deployment
	if a.HasAsset(AssetLetterhead) {
		cachePath, err := a.GetAssetToFile(AssetLetterhead)
		if err == nil {
			log.Printf("📄 Using letterhead from database cache: %s", cachePath)
			return cachePath
		}
		log.Printf("⚠️ Failed to extract letterhead from DB: %v, falling back to file system", err)
	}

	// FALLBACK: Search file system (for dev mode or first-run before sync)
	letterheadFile := "PH Trading Quotation Template.png"

	// Try multiple locations in order of preference
	candidatePaths := []string{}

	// 1. From project root (dev mode / wails dev)
	paths := a.GetApplicationPaths()
	candidatePaths = append(candidatePaths, filepath.Join(paths.ProjectRoot, "CRITICAL_PH_CONTEXT_SSOT", letterheadFile))

	// 2. Relative to executable (production mode - exe is in build/bin/)
	if exePath, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exePath)
		// From build/bin/ go up 2 levels to project root
		candidatePaths = append(candidatePaths, filepath.Join(exeDir, "..", "..", "CRITICAL_PH_CONTEXT_SSOT", letterheadFile))
		// Also try if letterhead is copied to build/bin/data/
		candidatePaths = append(candidatePaths, filepath.Join(exeDir, "data", letterheadFile))
	}

	// 3. Current working directory
	if cwd, err := os.Getwd(); err == nil {
		candidatePaths = append(candidatePaths, filepath.Join(cwd, "CRITICAL_PH_CONTEXT_SSOT", letterheadFile))
	}

	// Find first existing path
	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			log.Printf("📄 Found letterhead at: %s", path)
			return path
		}
	}

	// Return default path (will trigger fallback if not found)
	defaultPath := filepath.Join(paths.ProjectRoot, "CRITICAL_PH_CONTEXT_SSOT", letterheadFile)
	log.Printf("⚠️ Letterhead not found in any location, using default: %s", defaultPath)
	return defaultPath
}

// addLetterheadBackground places the full letterhead template as a page background
func (a *App) addLetterheadBackground(pdf *gofpdf.Fpdf) {
	imgPath := a.letterheadImagePath()
	fileInfo, err := os.Stat(imgPath)
	if err != nil {
		// Fallback: simple text header if image not found
		addLetterheadFallback(pdf)
		return
	}

	// Check file size to prevent memory issues (10MB max)
	if fileInfo.Size() > 10*1024*1024 {
		log.Printf("⚠️ Letterhead file too large (%d bytes), using fallback", fileInfo.Size())
		addLetterheadFallback(pdf)
		return
	}

	// Place the full letterhead template as background (A4: 210x297mm)
	opt := gofpdf.ImageOptions{
		ImageType: "PNG",
		ReadDpi:   true,
	}
	pdf.ImageOptions(imgPath, 0, 0, 210, 297, false, opt, 0, "")

	// Set cursor below the header area (logo is ~35mm tall)
	pdf.SetY(42)
}

// addLetterheadFallback renders a simple text letterhead when image is unavailable
func addLetterheadFallback(pdf *gofpdf.Fpdf) {
	// Simple text header
	pdf.SetY(10)
	pdf.SetFont("Helvetica", "B", 14)
	pdf.SetTextColor(80, 80, 80)
	pdf.CellFormat(0, 8, "PH TRADING W.L.L.", "", 0, "C", false, 0, "")
	pdf.Ln(10)

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(0, 5, "P.O. Box 815, Manama, Kingdom of Bahrain | Tel: +973 17 587654 | CR. No.68034-1", "", 0, "C", false, 0, "")
	pdf.Ln(8)

	// Divider
	pdf.SetDrawColor(200, 200, 200)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(6)
}

func (a *App) generateIntelligenceReportPDF(reportType, query string, context map[string]interface{}, aiAnalysis string) (string, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTopMargin(50)           // Start content below letterhead header (~45mm tall)
	pdf.SetAutoPageBreak(true, 35) // Bottom margin: 35mm to avoid letterhead footer
	pdf.SetLeftMargin(18)
	pdf.SetRightMargin(18)

	// Set header function so letterhead appears on ALL pages including auto-break pages
	imgPath := a.letterheadImagePath()
	pdf.SetHeaderFuncMode(func() {
		fileInfo, err := os.Stat(imgPath)
		if err == nil && fileInfo.Size() <= 10*1024*1024 { // 10MB max
			opt := gofpdf.ImageOptions{ImageType: "PNG", ReadDpi: true}
			pdf.ImageOptions(imgPath, 0, 0, 210, 297, false, opt, 0, "")
		} else if err == nil {
			log.Printf("⚠️ Letterhead file too large (%d bytes), skipping", fileInfo.Size())
		}
	}, true) // true = allow content to overlap the header area

	// === COVER PAGE ===
	pdf.AddPage()
	addCoverPage(pdf, reportType, query)

	// === CONTENT PAGES (data + analysis flow continuously, no forced page breaks) ===
	pdf.AddPage()

	// Data section
	hasData := a.hasDataForReport(reportType, context)
	if hasData {
		a.addDataPage(pdf, reportType, context)
	}

	// AI Analysis section (flows right after data, no new page)
	cleanAnalysis := strings.TrimSpace(aiAnalysis)
	if cleanAnalysis != "" && cleanAnalysis != "AI analysis unavailable. Report contains raw data only." {
		// Add a divider between data and analysis
		if hasData {
			pdf.Ln(6)
			pdf.SetDrawColor(200, 200, 200)
			pdf.Line(18, pdf.GetY(), 192, pdf.GetY())
			pdf.Ln(6)
		}
		addAnalysisPage(pdf, cleanAnalysis)
	}

	// === PAGE NUMBERS (positioned above letterhead footer) ===
	totalPages := pdf.PageCount()
	for i := 1; i <= totalPages; i++ {
		pdf.SetPage(i)
		pdf.SetY(260) // Just above the letterhead footer area
		pdf.SetFont("Helvetica", "I", 7)
		pdf.SetTextColor(160, 160, 160)
		pdf.CellFormat(0, 5, fmt.Sprintf("Intelligence Report  |  Page %d of %d  |  Confidential", i, totalPages), "", 0, "C", false, 0, "")
	}

	// Save
	paths := a.GetApplicationPaths()
	exportDir := filepath.Join(paths.ProjectRoot, "exports", "intelligence")
	os.MkdirAll(exportDir, 0755)

	timestamp := time.Now().Format("20060102_150405")
	// Sanitize query: only allow alphanumeric, underscore, and hyphen (prevent path traversal)
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	safeQuery := reg.ReplaceAllString(query, "_")
	if len(safeQuery) > 30 {
		safeQuery = safeQuery[:30]
	}
	if safeQuery == "" {
		safeQuery = "general"
	}
	filename := fmt.Sprintf("intelligence_%s_%s_%s.pdf", reportType, safeQuery, timestamp)
	filePath := filepath.Join(exportDir, filename)

	if err := pdf.OutputFileAndClose(filePath); err != nil {
		return "", fmt.Errorf("failed to save PDF: %w", err)
	}

	return filePath, nil
}

// hasDataForReport checks if we actually have data to render for this report type
func (a *App) hasDataForReport(reportType string, context map[string]interface{}) bool {
	switch reportType {
	case "customer":
		_, ok := context["customer_data"]
		_, ok2 := context["ar_summary"]
		return ok || ok2
	case "financial":
		_, ok := context["financial_data"]
		return ok
	case "risk":
		_, ok := context["risk_data"]
		return ok
	case "operations":
		_, ok := context["operations_data"]
		return ok
	case "supplier":
		_, ok := context["supplier_data"]
		return ok
	case "executive":
		return true // Always has business_summary
	default:
		_, ok := context["business_summary"]
		return ok
	}
}

// addCoverPage creates a professional cover page (letterhead already rendered above)
func addCoverPage(pdf *gofpdf.Fpdf, reportType, query string) {
	// Report title - large and prominent
	pdf.Ln(10)
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetTextColor(29, 29, 31)
	reportTitle := getReportTitle(reportType)
	pdf.Cell(0, 10, reportTitle)
	pdf.Ln(14)

	if query != "" {
		pdf.SetFont("Helvetica", "", 13)
		pdf.SetTextColor(60, 60, 65)
		pdf.Cell(0, 8, fmt.Sprintf("Subject: %s", sanitizeForPDF(query)))
		pdf.Ln(14)
	}

	// Metadata block
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(80, 80, 80)
	pdf.Cell(40, 6, "Date:")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(0, 6, time.Now().Format("2 January 2006"))
	pdf.Ln(7)

	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(40, 6, "Period:")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(0, 6, fmt.Sprintf("Year to Date %d", time.Now().Year()))
	pdf.Ln(7)

	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(40, 6, "Classification:")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(0, 6, "Internal - Confidential")
	pdf.Ln(7)

	pdf.SetFont("Helvetica", "", 10)
	pdf.Cell(40, 6, "Prepared by:")
	pdf.SetFont("Helvetica", "B", 10)
	pdf.Cell(0, 6, "PH Trading Business Intelligence")
	pdf.Ln(14)

	// Disclaimer
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(130, 130, 130)
	pdf.MultiCell(0, 4, "This report is generated from live ERP data and AI-powered analysis. All figures are derived from actual transaction records. Intended for internal management use only.", "", "", false)
}

func getReportTitle(reportType string) string {
	switch reportType {
	case "customer":
		return "Customer Intelligence Report"
	case "financial":
		return "Financial Health Report"
	case "risk":
		return "Risk Assessment Report"
	case "operations":
		return "Operations Pipeline Report"
	case "supplier":
		return "Supplier Performance Report"
	case "executive":
		return "Executive Summary Report"
	default:
		return "Intelligence Report"
	}
}

// addDataPage adds raw business data tables
func (a *App) addDataPage(pdf *gofpdf.Fpdf, reportType string, context map[string]interface{}) {
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 7, "Key Business Data")
	pdf.Ln(8)

	// Business summary (always present)
	if summary, ok := context["business_summary"].(map[string]interface{}); ok {
		addSectionHeader(pdf, "Company Overview")
		addMetricRow(pdf, "Total Customers", fmt.Sprintf("%v", summary["total_customers"]))
		addMetricRow(pdf, "Total Suppliers", fmt.Sprintf("%v", summary["total_suppliers"]))
		addMetricRow(pdf, "Total Invoices", fmt.Sprintf("%v", summary["total_invoices"]))
		addMetricRow(pdf, "Total Orders", fmt.Sprintf("%v", summary["total_orders"]))
		if rev, ok := summary["total_revenue_bhd"].(float64); ok {
			addMetricRow(pdf, "Total Revenue", fmt.Sprintf("%.3f BHD", rev))
		}
		if out, ok := summary["total_outstanding_bhd"].(float64); ok {
			addMetricRow(pdf, "Total Outstanding", fmt.Sprintf("%.3f BHD", out))
		}
		if grades, ok := summary["grade_distribution"].(map[string]int64); ok {
			addMetricRow(pdf, "Grade A Customers", fmt.Sprintf("%d", grades["A"]))
			addMetricRow(pdf, "Grade B Customers", fmt.Sprintf("%d", grades["B"]))
			addMetricRow(pdf, "Grade C Customers", fmt.Sprintf("%d", grades["C"]))
			addMetricRow(pdf, "Grade D Customers", fmt.Sprintf("%d", grades["D"]))
		}
		pdf.Ln(4)
	}

	// Domain-specific data
	switch reportType {
	case "customer":
		a.addCustomerDataToPDF(pdf, context)
	case "financial":
		a.addFinancialDataToPDF(pdf, context)
	case "risk":
		a.addRiskDataToPDF(pdf, context)
	case "operations":
		a.addOperationsDataToPDF(pdf, context)
	case "supplier":
		a.addSupplierDataToPDF(pdf, context)
	case "executive":
		// Add all sections
		a.addFinancialDataToPDF(pdf, context)
		a.addOperationsDataToPDF(pdf, context)
		a.addRiskDataToPDF(pdf, context)
	}
}

func (a *App) addCustomerDataToPDF(pdf *gofpdf.Fpdf, context map[string]interface{}) {
	if custData, ok := context["customer_data"].(map[string]interface{}); ok {
		if customer, ok := custData["customer"].(map[string]interface{}); ok {
			addSectionHeader(pdf, fmt.Sprintf("Customer: %v", customer["name"]))
			addMetricRow(pdf, "Grade", fmt.Sprintf("%v", customer["grade"]))
			addMetricRow(pdf, "Payment Grade", fmt.Sprintf("%v", customer["payment_grade"]))
			addMetricRow(pdf, "Avg Payment Days", fmt.Sprintf("%.0f", customer["avg_payment_days"]))
			addMetricRow(pdf, "Outstanding", fmt.Sprintf("%.3f BHD", customer["outstanding_bhd"]))
			addMetricRow(pdf, "Overdue Days", fmt.Sprintf("%v", customer["overdue_days"]))
			addMetricRow(pdf, "Total Orders", fmt.Sprintf("%v", customer["total_orders"]))
			addMetricRow(pdf, "Total Value", fmt.Sprintf("%.3f BHD", customer["total_value"]))
			addMetricRow(pdf, "AR Risk Tier", fmt.Sprintf("%v", customer["ar_risk_tier"]))
			pdf.Ln(6)
		}

		// Top outstanding customers
		if topCusts, ok := custData["top_outstanding_customers"].([]map[string]interface{}); ok && len(topCusts) > 0 {
			addSectionHeader(pdf, "Top Outstanding Customers")
			addTableHeader(pdf, []string{"Customer", "Outstanding (BHD)", "Grade", "Overdue"})
			for _, c := range topCusts {
				addTableRow(pdf, []string{
					fmt.Sprintf("%v", c["name"]),
					fmt.Sprintf("%.3f", c["outstanding"]),
					fmt.Sprintf("%v", c["grade"]),
					fmt.Sprintf("%v days", c["overdue"]),
				})
			}
			pdf.Ln(6)
		}
	}

	if ar, ok := context["ar_summary"].(map[string]interface{}); ok {
		addSectionHeader(pdf, "Accounts Receivable Summary")
		addMetricRow(pdf, "Total Receivables", fmt.Sprintf("%.3f BHD", ar["total_receivables_bhd"]))
		addMetricRow(pdf, "Overdue Receivables", fmt.Sprintf("%.3f BHD", ar["overdue_receivables_bhd"]))
		if pct, ok := ar["overdue_percentage"].(float64); ok {
			addMetricRow(pdf, "Overdue %", fmt.Sprintf("%.1f%%", pct))
		}
		pdf.Ln(3)
	}
}

func (a *App) addFinancialDataToPDF(pdf *gofpdf.Fpdf, context map[string]interface{}) {
	if finData, ok := context["financial_data"].(map[string]interface{}); ok {
		addSectionHeader(pdf, "Financial Metrics")
		addMetricRow(pdf, "Total Invoiced", fmt.Sprintf("%.3f BHD", finData["total_invoiced_bhd"]))
		addMetricRow(pdf, "Total Paid", fmt.Sprintf("%.3f BHD", finData["total_paid_bhd"]))
		addMetricRow(pdf, "Total Outstanding", fmt.Sprintf("%.3f BHD", finData["total_outstanding_bhd"]))
		addMetricRow(pdf, "Avg Days to Payment", fmt.Sprintf("%.0f days", finData["avg_days_to_payment"]))

		if counts, ok := finData["invoice_counts"].(map[string]int64); ok {
			addMetricRow(pdf, "Invoices Sent", fmt.Sprintf("%d", counts["sent"]))
			addMetricRow(pdf, "Invoices Paid", fmt.Sprintf("%d", counts["paid"]))
			addMetricRow(pdf, "Invoices Overdue", fmt.Sprintf("%d", counts["overdue"]))
		}
		pdf.Ln(3)
	}
}

func (a *App) addRiskDataToPDF(pdf *gofpdf.Fpdf, context map[string]interface{}) {
	if riskData, ok := context["risk_data"].(map[string]interface{}); ok {
		addSectionHeader(pdf, "Risk Assessment")
		addMetricRow(pdf, "Total Overdue", fmt.Sprintf("%.3f BHD", riskData["total_overdue_bhd"]))
		addMetricRow(pdf, "Critical Risk Customers", fmt.Sprintf("%v", riskData["critical_risk_customers"]))
		addMetricRow(pdf, "High Risk Customers", fmt.Sprintf("%v", riskData["high_risk_customers"]))

		if blocked, ok := riskData["credit_blocked_customers"].([]string); ok && len(blocked) > 0 {
			addMetricRow(pdf, "Credit Blocked", strings.Join(blocked, ", "))
		}

		// Overdue invoices table
		if overdueList, ok := riskData["overdue_invoices"].([]map[string]interface{}); ok && len(overdueList) > 0 {
			pdf.Ln(4)
			addSectionHeader(pdf, "Overdue Invoices Detail")
			addTableHeader(pdf, []string{"Invoice", "Customer", "Outstanding", "Days Overdue"})
			for _, inv := range overdueList {
				addTableRow(pdf, []string{
					fmt.Sprintf("%v", inv["invoice"]),
					fmt.Sprintf("%v", inv["customer"]),
					fmt.Sprintf("%.3f BHD", inv["outstanding"]),
					fmt.Sprintf("%v", inv["days_overdue"]),
				})
			}
		}
		pdf.Ln(3)
	}
}

func (a *App) addOperationsDataToPDF(pdf *gofpdf.Fpdf, context map[string]interface{}) {
	if opsData, ok := context["operations_data"].(map[string]interface{}); ok {
		addSectionHeader(pdf, "Operations Pipeline")
		if offers, ok := opsData["offers"].(map[string]int64); ok {
			addMetricRow(pdf, "Total Offers", fmt.Sprintf("%d", offers["total"]))
			addMetricRow(pdf, "Quoted", fmt.Sprintf("%d", offers["quoted"]))
			addMetricRow(pdf, "Won", fmt.Sprintf("%d", offers["won"]))
			addMetricRow(pdf, "Lost", fmt.Sprintf("%d", offers["lost"]))
		}
		if wr, ok := opsData["win_rate"].(float64); ok {
			addMetricRow(pdf, "Win Rate", fmt.Sprintf("%.1f%%", wr))
		}
		addMetricRow(pdf, "Active Orders", fmt.Sprintf("%v", opsData["active_orders"]))
		addMetricRow(pdf, "Active Order Value", fmt.Sprintf("%.3f BHD", opsData["active_order_value_bhd"]))
		addMetricRow(pdf, "Active POs", fmt.Sprintf("%v", opsData["active_purchase_orders"]))
		addMetricRow(pdf, "Active PO Value", fmt.Sprintf("%.3f BHD", opsData["active_po_value_bhd"]))
		addMetricRow(pdf, "Pending Deliveries", fmt.Sprintf("%v", opsData["pending_deliveries"]))
		pdf.Ln(3)
	}
}

func (a *App) addSupplierDataToPDF(pdf *gofpdf.Fpdf, context map[string]interface{}) {
	if suppData, ok := context["supplier_data"].(map[string]interface{}); ok {
		if supplier, ok := suppData["supplier"].(map[string]interface{}); ok {
			addSectionHeader(pdf, fmt.Sprintf("Supplier: %v", supplier["name"]))
			addMetricRow(pdf, "Type", fmt.Sprintf("%v", supplier["type"]))
			addMetricRow(pdf, "Country", fmt.Sprintf("%v", supplier["country"]))
			addMetricRow(pdf, "Rating", fmt.Sprintf("%v / 5", supplier["rating"]))
			addMetricRow(pdf, "Lead Time", fmt.Sprintf("%v days", supplier["lead_time_days"]))
			addMetricRow(pdf, "Brands", fmt.Sprintf("%v", supplier["brands"]))
			pdf.Ln(6)
		}

		if topSupp, ok := suppData["top_suppliers"].([]map[string]interface{}); ok && len(topSupp) > 0 {
			addSectionHeader(pdf, "Top Suppliers by PO Value")
			addTableHeader(pdf, []string{"Supplier", "Total PO (BHD)", "Rating", "Lead Days"})
			for _, s := range topSupp {
				addTableRow(pdf, []string{
					fmt.Sprintf("%v", s["name"]),
					fmt.Sprintf("%.3f", s["total_po"]),
					fmt.Sprintf("%v/5", s["rating"]),
					fmt.Sprintf("%v", s["lead_days"]),
				})
			}
		}
		pdf.Ln(3)
	}
}

// addAnalysisPage adds the AI-generated analysis with proper markdown rendering
func addAnalysisPage(pdf *gofpdf.Fpdf, aiAnalysis string) {
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 7, "Analysis")
	pdf.Ln(8)

	// Clean up the AI text (remove any [ACTIONS] blocks, sanitize encoding)
	cleanText := actionBlockRegex.ReplaceAllString(aiAnalysis, "")
	cleanText = strings.TrimSpace(cleanText)
	cleanText = sanitizeForPDF(cleanText) // Convert UTF-8 special chars to ASCII

	// Process line by line for proper markdown rendering
	lines := strings.Split(cleanText, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines (just add small spacing)
		if line == "" {
			pdf.Ln(2)
			continue
		}

		// Handle --- dividers
		if line == "---" || line == "***" || line == "___" {
			pdf.Ln(1)
			pdf.SetDrawColor(200, 200, 200)
			pdf.Line(18, pdf.GetY(), 192, pdf.GetY())
			pdf.Ln(2)
			continue
		}

		// Handle ## headers
		if strings.HasPrefix(line, "## ") {
			headerText := strings.TrimPrefix(line, "## ")
			headerText = stripMarkdownBold(headerText)
			pdf.Ln(3)
			pdf.SetFont("Helvetica", "B", 11)
			pdf.SetTextColor(29, 29, 31)
			pdf.MultiCell(0, 5, sanitizeForPDF(headerText), "", "", false)
			pdf.Ln(1)
			continue
		}

		// Handle ### subheaders
		if strings.HasPrefix(line, "### ") {
			headerText := strings.TrimPrefix(line, "### ")
			headerText = stripMarkdownBold(headerText)
			pdf.Ln(2)
			pdf.SetFont("Helvetica", "B", 10)
			pdf.SetTextColor(50, 50, 55)
			pdf.MultiCell(0, 5, sanitizeForPDF(headerText), "", "", false)
			pdf.Ln(1)
			continue
		}

		// Handle numbered section headers: "1. EXECUTIVE SUMMARY" or "1. **SECTION**"
		if isNumberedHeader(line) {
			headerText := cleanHeaderText(line)
			pdf.Ln(3)
			pdf.SetFont("Helvetica", "B", 11)
			pdf.SetTextColor(29, 29, 31)
			pdf.MultiCell(0, 5, sanitizeForPDF(headerText), "", "", false)
			pdf.Ln(1)
			continue
		}

		// Handle standalone bold lines as subheaders: "**Something**"
		if isStandaloneBold(line) {
			headerText := stripMarkdownBold(line)
			pdf.Ln(1)
			pdf.SetFont("Helvetica", "B", 9)
			pdf.SetTextColor(40, 40, 45)
			pdf.MultiCell(0, 4, sanitizeForPDF(headerText), "", "", false)
			pdf.Ln(1)
			continue
		}

		// Handle bullet points
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			bulletText := line[2:]
			bulletText = stripMarkdownBold(bulletText)
			pdf.SetFont("Helvetica", "", 9)
			pdf.SetTextColor(40, 40, 40)
			pdf.SetX(22)
			pdf.MultiCell(0, 4, "- "+sanitizeForPDF(bulletText), "", "", false)
			continue
		}

		// Regular text - strip ** and render
		plainText := stripMarkdownBold(line)
		pdf.SetFont("Helvetica", "", 9)
		pdf.SetTextColor(40, 40, 40)
		pdf.MultiCell(0, 4, sanitizeForPDF(plainText), "", "", false)
	}
}

// stripMarkdownBold removes all ** markers from text, returning plain text
func stripMarkdownBold(s string) string {
	return strings.ReplaceAll(s, "**", "")
}

// sanitizeForPDF replaces UTF-8 special characters with ASCII equivalents
// gofpdf uses Windows-1252 encoding and cannot render multi-byte UTF-8 chars
func sanitizeForPDF(s string) string {
	replacer := strings.NewReplacer(
		"\u2019", "'", // right single quote
		"\u2018", "'", // left single quote
		"\u201C", "\"", // left double quote
		"\u201D", "\"", // right double quote
		"\u2013", "-", // en-dash
		"\u2014", "-", // em-dash
		"\u2026", "...", // ellipsis
		"\u2022", "-", // bullet
		"\u2023", "-", // triangular bullet
		"\u2043", "-", // hyphen bullet
		"\u00A0", " ", // non-breaking space
		"\u200B", "", // zero-width space
		"\u00B7", "-", // middle dot
		"\u2212", "-", // minus sign
		"\u2032", "'", // prime
		"\u2033", "\"", // double prime
		"\u00AB", "\"", // left guillemet
		"\u00BB", "\"", // right guillemet
		"\u2010", "-", // hyphen
		"\u2011", "-", // non-breaking hyphen
		"\u2015", "-", // horizontal bar
		"\u00D7", "x", // multiplication sign
		"\u00F7", "/", // division sign
	)
	return replacer.Replace(s)
}

// isNumberedHeader checks if a line is a numbered section header like "1. TITLE" or "1. **TITLE**"
func isNumberedHeader(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 3 {
		return false
	}
	// Match: digit(s) followed by . or ) then space
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i == 0 || i >= len(s) {
		return false
	}
	if s[i] != '.' && s[i] != ')' {
		return false
	}
	// Rest should be mostly uppercase or bold-wrapped
	rest := strings.TrimSpace(s[i+1:])
	rest = stripMarkdownBold(rest)
	if len(rest) < 3 || len(rest) > 80 {
		return false
	}
	// Check if it's a title (mostly uppercase or short enough to be a header)
	upper := strings.ToUpper(rest)
	if rest == upper {
		return true
	}
	// Also treat bold-wrapped numbered items as headers
	if strings.Contains(s, "**") {
		return true
	}
	return false
}

// cleanHeaderText extracts clean header text from numbered headers
func cleanHeaderText(s string) string {
	// Remove leading number and separator
	i := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
	}
	if i < len(s) && (s[i] == '.' || s[i] == ')') {
		i++
	}
	result := strings.TrimSpace(s[i:])
	result = stripMarkdownBold(result)
	return result
}

// isStandaloneBold checks if the entire line is wrapped in ** (a bold subheader)
func isStandaloneBold(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 5 {
		return false
	}
	// Entire line is **text**
	if strings.HasPrefix(s, "**") && strings.HasSuffix(s, "**") && strings.Count(s, "**") == 2 {
		return true
	}
	return false
}

// ============================================================================
// PDF HELPER FUNCTIONS
// ============================================================================

func addSectionHeader(pdf *gofpdf.Fpdf, title string) {
	pdf.Ln(2)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(29, 29, 31)
	pdf.Cell(0, 6, sanitizeForPDF(title))
	pdf.Ln(7)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(60, 60, 60)
}

func addMetricRow(pdf *gofpdf.Fpdf, label, value string) {
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(110, 110, 115)
	pdf.Cell(65, 5, sanitizeForPDF(label))
	pdf.SetTextColor(29, 29, 31)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.Cell(0, 5, sanitizeForPDF(value))
	pdf.Ln(6)
	pdf.SetFont("Helvetica", "", 9)
}

func addTableHeader(pdf *gofpdf.Fpdf, headers []string) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFillColor(45, 45, 48)

	colWidth := 170.0 / float64(len(headers))
	for _, h := range headers {
		pdf.CellFormat(colWidth, 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(7)
	pdf.SetTextColor(29, 29, 31)
	pdf.SetFont("Helvetica", "", 9)
}

func addTableRow(pdf *gofpdf.Fpdf, values []string) {
	colWidth := 170.0 / float64(len(values))
	for i, v := range values {
		align := "L"
		if i > 0 {
			align = "C"
		}
		pdf.CellFormat(colWidth, 6, sanitizeForPDF(v), "1", 0, align, false, 0, "")
	}
	pdf.Ln(6)
}
