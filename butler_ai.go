// ═══════════════════════════════════════════════════════════════════════════
// BUTLER AI CHAT - MISTRAL INTELLIGENCE PIPELINE
//
// MISSION: AI-powered business intelligence agent with contextual data injection
//
// ARCHITECTURE:
//   1. Intent Classification (keyword + entity extraction)
//   2. Context Builder (GORM queries based on intent domain)
//   3. Three-Regime Awareness (adjusts LLM guidance)
//   4. Mistral API Client (small for simple, large for complex)
//   5. Action Parser ([ACTIONS]...[/ACTIONS] JSON blocks)
//   6. Response Formatter (ButlerResponse for frontend)
//
// MODELS:
//   - mistral-small-latest: Simple queries (customer lookup, basic stats)
//   - mistral-large-latest: Complex multi-entity analysis, recommendations
//
// Built with MATHEMATICAL RIGOR × AI AUGMENTATION × BUSINESS INTELLIGENCE
// Day 195+ - Mistral Intelligence Pipeline
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// ButlerResponse represents AI response with suggested actions
type ButlerResponse struct {
	Message    string                 `json:"message"`
	Actions    []ButlerAction         `json:"actions"`
	Confidence float64                `json:"confidence"`
	Context    map[string]interface{} `json:"context"`
}

// ButlerAction represents a suggested action from Butler
type ButlerAction struct {
	Type   string      `json:"type"`   // "navigate", "create", "update", "fetch", "analyze", "alert"
	Target string      `json:"target"` // screen or entity
	Data   interface{} `json:"data"`
}

// Intent represents classified user intent
type Intent struct {
	Domain       string   // customer, supplier, financial, operations, risk, market, general
	EntityName   string   // extracted customer/supplier name
	Confidence   float64  // 0.0 - 1.0
	Keywords     []string // matched keywords
	NeedsScraper bool     // true if external data needed
	IsComplex    bool     // true if requires mistral-large
}

// ============================================================================
// MISTRAL API CONSTANTS
// ============================================================================

const (
	mistralAPIURL      = "https://api.mistral.ai/v1/chat/completions"
	mistralModelSmall  = "mistral-small-latest"
	mistralModelLarge  = "mistral-large-latest"
	mistralMaxTokens   = 8192
	mistralTemperature = 0.4

	// AIML API (Grok) constants — primary LLM backend for chat queries
	// AIML API is OpenAI-compatible; Grok offers ~1M token context window
	// Model ID: verify exact name in your AIML API dashboard (https://api.aimlapi.com)
	aimlAPIURL      = "https://api.aimlapi.com/v1/chat/completions"
	aimlModelID     = "grok-4"  // xAI Grok 4 via AIML API — update if model ID differs
	aimlMaxTokens   = 16384
	aimlTemperature = 0.4

	// Intent classification constants
	MaxKeywordMatchScore         = 4.0
	ComplexQueryDomainThreshold  = 3
	ComplexQueryLengthThreshold  = 200
	MistralAPITimeout            = 120 * time.Second
	AimlAPITimeout               = 120 * time.Second
)

// ============================================================================
// INTENT CLASSIFICATION
// ============================================================================

var (
	customerKeywords   = []string{"customer", "client", "buyer", "account", "bapco", "alba", "ewa", "gpic", "noga", "aluminium bahrain", "bahrain petroleum", "electricity water", "gulf petrochemical", "tatweer", "banagas", "business with", "total business", "how much", "who is our", "top customer", "biggest customer", "best customer", "worst customer"}
	supplierKeywords   = []string{"supplier", "vendor", "source", "endress", "hauser", "servomex", "landis", "gyr", "e+h", "bought from", "purchased from", "procurement", "supply chain", "top supplier", "lead time"}
	financialKeywords  = []string{"cash", "payment", "invoice", "receivable", "payable", "balance", "revenue", "profit", "margin", "ar", "aging", "dso", "outstanding", "collection", "money", "owe", "owes", "paid", "unpaid", "earned", "income", "expense", "cashflow", "cash flow", "how much did we", "total sales", "turnover", "gross", "net", "bhd", "fiscal", "financial year"}
	operationsKeywords = []string{"quote", "rfq", "order", "pipeline", "delivery", "purchase", "costing", "offer", "grn", "shipment", "po", "purchase order", "win rate", "conversion", "open orders", "pending", "active orders", "quotation"}
	riskKeywords       = []string{"overdue", "late", "risk", "aging", "collection", "blocked", "dispute", "critical", "warning", "defaulter", "slow payer", "bad debt", "credit block"}
	marketKeywords     = []string{"price", "market", "competitor", "trend", "industry", "steel", "instrumentation"}

	// Patterns that indicate complex queries needing mistral-large
	complexPatterns = []string{"compare", "analyze", "recommend", "strategy", "forecast", "predict", "trend", "correlat", "multiple", "relationship"}

	// Action regex for parsing [ACTIONS]...[/ACTIONS] blocks
	actionBlockRegex = regexp.MustCompile(`(?s)\[ACTIONS\](.*?)\[/ACTIONS\]`)
)

// classifyIntent determines query domain and extracts entities
func classifyIntent(query string) Intent {
	queryLower := strings.ToLower(query)

	intent := Intent{
		Domain:     "general",
		Confidence: 0.5,
		Keywords:   []string{},
	}

	// Count keyword matches per domain
	scores := map[string]int{
		"customer":   countKeywordMatches(queryLower, customerKeywords),
		"supplier":   countKeywordMatches(queryLower, supplierKeywords),
		"financial":  countKeywordMatches(queryLower, financialKeywords),
		"operations": countKeywordMatches(queryLower, operationsKeywords),
		"risk":       countKeywordMatches(queryLower, riskKeywords),
		"market":     countKeywordMatches(queryLower, marketKeywords),
	}

	// Find highest scoring domain
	maxScore := 0
	for domain, score := range scores {
		if score > maxScore {
			maxScore = score
			intent.Domain = domain
		}
	}

	// Set confidence based on match count
	intent.Confidence = float64(maxScore) / MaxKeywordMatchScore
	if intent.Confidence > 1.0 {
		intent.Confidence = 1.0
	}
	if maxScore == 0 {
		intent.Confidence = 0.3
	}

	// Extract entity name for customer/supplier queries
	if intent.Domain == "customer" || intent.Domain == "supplier" {
		intent.EntityName = extractEntityName(query)
	}

	// Market queries need scraper
	if intent.Domain == "market" {
		intent.NeedsScraper = true
	}

	// Check if query is complex (needs larger model)
	for _, pattern := range complexPatterns {
		if strings.Contains(queryLower, pattern) {
			intent.IsComplex = true
			break
		}
	}

	// Multi-domain or long queries are complex
	domainCount := 0
	for _, score := range scores {
		if score > 0 {
			domainCount++
		}
	}
	if domainCount >= ComplexQueryDomainThreshold || len(query) > ComplexQueryLengthThreshold {
		intent.IsComplex = true
	}

	return intent
}

// countKeywordMatches counts how many keywords appear in the query
func countKeywordMatches(query string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if strings.Contains(query, kw) {
			count++
		}
	}
	return count
}

// extractEntityName extracts a customer or supplier name from the query using
// three strategies in priority order: possessive detection, trigger-word scan,
// and all-caps acronym detection (BAPCO, EWA, GPIC, ALBA, etc.).
func extractEntityName(query string) string {
	words := strings.Fields(query)

	// --- Strategy 1: possessive ("EWA's", "Alba's", "BAPCO's") ---
	possessiveRe := regexp.MustCompile(`(?i)\b([A-Z][A-Za-z&().\-]{1,40}(?:\s+[A-Z][A-Za-z&().\-]{1,40}){0,4})'s?\b`)
	if m := possessiveRe.FindStringSubmatch(query); len(m) > 1 {
		candidate := strings.TrimSpace(m[1])
		if !isEntityStopWord(strings.ToLower(candidate)) {
			return candidate
		}
	}

	// --- Strategy 2: trigger-word scan (expanded from original) ---
	triggers := map[string]bool{
		"about": true, "is": true, "for": true, "with": true, "from": true,
		"customer": true, "supplier": true, "client": true,
		"how's": true, "hows": true, "show": true, "get": true, "find": true,
		"tell": true, "regarding": true, "on": true, "contact": true,
		"invoice": true, "invoices": true, "order": true, "orders": true,
		"payment": true, "payments": true, "outstanding": true, "balance": true,
		"status": true, "update": true, "info": true, "details": true,
	}
	for i, word := range words {
		wLower := strings.ToLower(strings.TrimRight(word, "?,.'"))
		if triggers[wLower] && i+1 < len(words) {
			name := []string{}
			for j := i + 1; j < len(words) && j < i+6; j++ {
				w := strings.TrimRight(words[j], "?,.'")
				if isEntityStopWord(strings.ToLower(w)) {
					break
				}
				name = append(name, w)
			}
			if len(name) > 0 {
				return strings.TrimRight(strings.Join(name, " "), "?.,")
			}
		}
	}

	// --- Strategy 3: all-caps acronyms (BAPCO, EWA, GPIC, NOGA, TTSJV) ---
	// Skip generic ERP acronyms (AR, AP, PO, GRN, DN, VAT, BHD, FX, etc.)
	erpAcronyms := map[string]bool{
		"AR": true, "AP": true, "PO": true, "GRN": true, "DN": true, "VAT": true,
		"BHD": true, "FX": true, "AI": true, "ERP": true, "PDF": true, "OCR": true,
		"CEO": true, "CFO": true, "COO": true, "HR": true, "IT": true,
		"Q1": true, "Q2": true, "Q3": true, "Q4": true, "YTD": true, "MTD": true,
	}
	for _, word := range words {
		clean := strings.TrimRight(word, "?,.'")
		if len(clean) >= 2 && clean == strings.ToUpper(clean) && !erpAcronyms[clean] {
			return clean
		}
	}

	return ""
}

// isEntityStopWord returns true for words that are unlikely to be part of an entity name
func isEntityStopWord(w string) bool {
	stops := map[string]bool{
		"is": true, "the": true, "doing": true, "has": true, "with": true,
		"last": true, "this": true, "next": true, "year": true, "month": true,
		"quarter": true, "week": true, "total": true, "business": true,
		"in": true, "at": true, "to": true, "and": true, "a": true, "an": true,
		"we": true, "have": true, "had": true, "been": true, "was": true,
		"how": true, "what": true, "much": true, "many": true, "give": true,
		"me": true, "our": true, "their": true, "since": true, "during": true,
		"all": true, "can": true, "you": true, "any": true, "some": true,
		"its": true, "my": true, "your": true, "show": true,
		"get": true, "find": true, "tell": true, "list": true, "latest": true,
		"recent": true, "current": true, "pending": true, "overdue": true,
		"paid": true, "unpaid": true, "active": true, "closed": true,
	}
	return stops[w]
}

// ============================================================================
// MAIN CHAT FUNCTION
// ============================================================================

// ChatWithButler handles AI chat requests with business context injection
func (a *App) ChatWithButler(message string) (ButlerResponse, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return ButlerResponse{}, err
	}
	// 1. Classify intent
	intent := classifyIntent(message)
	log.Printf("🧠 Intent classified: domain=%s entity=%q confidence=%.2f complex=%v",
		intent.Domain, intent.EntityName, intent.Confidence, intent.IsComplex)

	// 2. RBAC: Check if user has permission to access financial data
	if intent.Domain == "financial" || intent.Domain == "risk" {
		userRole := a.GetCurrentUserRole()
		hasFinanceAccess := a.CheckPermissionByRole(userRole, "finance:view")

		if !hasFinanceAccess {
			log.Printf("🔒 Butler financial query blocked: user role=%s lacks finance:view", userRole)
			return ButlerResponse{
				Message: "Financial data access requires manager or admin privileges.\n\nYou can still ask me about:\n• Customer information\n• Supplier details\n• Order status\n• Operations and pipeline\n\nFor financial reports and metrics, please contact your manager.",
				Actions:    []ButlerAction{},
				Confidence: 0.95,
				Context:    map[string]interface{}{"blocked_domain": intent.Domain, "user_role": userRole},
			}, nil
		}
	}

	// 3. Check if this is a report generation request
	if isReportRequest(message) {
		return a.handleReportRequest(message, intent)
	}

	// 4. Build full context (all domains — Grok's large context window can handle it)
	context := a.buildFullContext(intent)

	// 5. Calculate regime
	regime := a.calculateSystemRegime()
	context["system_regime"] = regime

	// 6. Build system prompt with full context and regime
	systemPrompt := buildMistralSystemPrompt(context, regime)

	// 7. Call AIML/Grok (primary) or fall back to Mistral
	var aiResponse string
	var aiErr error
	usedBackend := "Mistral"
	if aimlKey := getAIMLAPIKey(); aimlKey != "" {
		aiResponse, aiErr = callAIML(aimlKey, systemPrompt, message)
		usedBackend = "AIML/Grok"
		if aiErr != nil {
			log.Printf("⚠️ AIML API error, falling back to Mistral: %v", aiErr)
			aiResponse, aiErr = callMistral(mistralModelLarge, systemPrompt, message)
			usedBackend = "Mistral (fallback)"
		}
	} else {
		aiResponse, aiErr = callMistral(mistralModelLarge, systemPrompt, message)
	}

	if aiErr != nil {
		log.Printf("❌ AI backend error: %v", aiErr)
		return ButlerResponse{
			Message:    fmt.Sprintf("Intelligence pipeline error: %v. Please try again.", aiErr),
			Actions:    []ButlerAction{},
			Confidence: 0.0,
			Context:    context,
		}, nil
	}

	// 8. Parse response (extract actions, clean message)
	butlerResponse := parseMistralResponse(aiResponse, context, intent)

	log.Printf("🤖 Butler responded via %s: %s (confidence: %.2f, actions: %d)",
		usedBackend, butlerTruncate(butlerResponse.Message, 60), butlerResponse.Confidence, len(butlerResponse.Actions))

	return butlerResponse, nil
}

// isReportRequest detects if the user is asking for a report/PDF generation
func isReportRequest(message string) bool {
	lower := strings.ToLower(message)
	reportTriggers := []string{"generate report", "create report", "make report", "produce report",
		"generate a report", "create a report", "make a report", "pdf report",
		"generate pdf", "create pdf", "detailed report", "intelligence report",
		"write a report", "write report", "prepare report", "prepare a report",
		"report on", "report for", "report about"}
	for _, trigger := range reportTriggers {
		if strings.Contains(lower, trigger) {
			return true
		}
	}
	return false
}

// detectReportType determines which report type to generate from the message
func detectReportType(message string, intent Intent) string {
	lower := strings.ToLower(message)

	if strings.Contains(lower, "customer") || strings.Contains(lower, "client") {
		return "customer"
	}
	if strings.Contains(lower, "financial") || strings.Contains(lower, "cash") || strings.Contains(lower, "revenue") {
		return "financial"
	}
	if strings.Contains(lower, "risk") || strings.Contains(lower, "overdue") || strings.Contains(lower, "aging") {
		return "risk"
	}
	if strings.Contains(lower, "operation") || strings.Contains(lower, "pipeline") || strings.Contains(lower, "order") {
		return "operations"
	}
	if strings.Contains(lower, "supplier") || strings.Contains(lower, "vendor") || strings.Contains(lower, "procurement") {
		return "supplier"
	}

	// Fall back to intent domain
	switch intent.Domain {
	case "customer":
		return "customer"
	case "supplier":
		return "supplier"
	case "financial":
		return "financial"
	case "risk":
		return "risk"
	case "operations":
		return "operations"
	}

	return "executive"
}

// handleReportRequest generates a PDF report and returns the path in the response
func (a *App) handleReportRequest(message string, intent Intent) (ButlerResponse, error) {
	reportType := detectReportType(message, intent)
	log.Printf("📊 Report request detected: type=%s entity=%q", reportType, intent.EntityName)

	filePath, err := a.GenerateButlerReport(reportType, intent.EntityName)
	if err != nil {
		// Check if this is a permission error
		errorMsg := err.Error()
		isPermissionError := strings.Contains(errorMsg, "permission denied") || strings.Contains(errorMsg, "finance:view")

		if isPermissionError {
			return ButlerResponse{
				Message: "I cannot generate this report due to insufficient permissions.\n\nFinancial, risk, and executive reports require manager or admin privileges.\n\nYou can still request:\n• Customer reports\n• Supplier reports\n• Operations reports\n\nPlease contact your manager for financial report access.",
				Actions:    []ButlerAction{},
				Confidence: 0.95,
				Context:    map[string]interface{}{"report_type": reportType, "blocked": "permission_denied"},
			}, nil
		}

		return ButlerResponse{
			Message:    fmt.Sprintf("I attempted to generate the report but encountered an error: %v", err),
			Actions:    []ButlerAction{},
			Confidence: 0.3,
			Context:    map[string]interface{}{"report_type": reportType, "error": err.Error()},
		}, nil
	}

	responseMsg := fmt.Sprintf("Your %s report has been generated successfully.\n\nFile saved to:\n%s\n\nThe report contains live business data from your database combined with AI-powered analysis covering key metrics, risk factors, and strategic recommendations.",
		getReportTitle(reportType), filePath)

	return ButlerResponse{
		Message:    responseMsg,
		Actions:    []ButlerAction{{Type: "fetch", Target: "report", Data: filePath}},
		Confidence: 0.95,
		Context:    map[string]interface{}{"report_type": reportType, "file_path": filePath},
	}, nil
}

// ============================================================================
// CONTEXT BUILDING (GORM-based, intent-aware)
// ============================================================================

// buildIntentContext gathers relevant data based on classified intent
func (a *App) buildIntentContext(intent Intent) map[string]interface{} {
	context := make(map[string]interface{})

	if a.db == nil {
		return context
	}

	// Check if user has finance access (for redacting financial data from non-finance contexts)
	userRole := a.GetCurrentUserRole()
	hasFinanceAccess := a.CheckPermissionByRole(userRole, "finance:view")

	// Always include dashboard summary (redacted for non-finance users)
	summary := a.getBusinessSummary()
	if !hasFinanceAccess {
		delete(summary, "total_revenue_bhd")
		delete(summary, "total_outstanding_bhd")
	}
	context["business_summary"] = summary

	// Domain-specific context injection
	switch intent.Domain {
	case "customer":
		customerCtx := a.getCustomerContext(intent.EntityName)
		if !hasFinanceAccess {
			// Redact financial data from customer context for non-finance users
			a.redactFinancialFromContext(customerCtx)
		}
		context["customer_data"] = customerCtx
		if hasFinanceAccess {
			context["ar_summary"] = a.getARSummary()
		}

	case "supplier":
		context["supplier_data"] = a.getSupplierContext(intent.EntityName)
		context["po_summary"] = a.getPurchaseOrderSummary()

	case "financial":
		context["financial_data"] = a.getFinancialContext()
		context["ar_summary"] = a.getARSummary()

	case "operations":
		context["operations_data"] = a.getOperationsContext()
		context["po_summary"] = a.getPurchaseOrderSummary()
		context["risk_data"] = a.getRiskContext() // Include risk for operations awareness

	case "risk":
		context["risk_data"] = a.getRiskContext()
		context["ar_summary"] = a.getARSummary()
		context["operations_data"] = a.getOperationsContext() // Include ops for delivery risk context

	default:
		// General query - provide comprehensive overview
		context["customer_data"] = a.getCustomerContext(intent.EntityName)
		context["ar_summary"] = a.getARSummary()
		context["operations_data"] = a.getOperationsContext()
		context["risk_data"] = a.getRiskContext()
		if hasFinanceAccess {
			context["financial_data"] = a.getFinancialContext()
		}
	}

	return context
}

// buildFullContext gathers ALL domain data for Grok's large context window.
// Unlike buildIntentContext (domain-specific), this loads everything at once —
// customer, supplier, financial, operations, risk — and lets Grok figure out
// what's relevant to the user's query. RBAC filtering still applies.
func (a *App) buildFullContext(intent Intent) map[string]interface{} {
	context := make(map[string]interface{})

	if a.db == nil {
		return context
	}

	userRole := a.GetCurrentUserRole()
	hasFinanceAccess := a.CheckPermissionByRole(userRole, "finance:view")

	// Business summary (always included)
	summary := a.getBusinessSummary()
	if !hasFinanceAccess {
		delete(summary, "total_revenue_bhd")
		delete(summary, "total_outstanding_bhd")
	}
	context["business_summary"] = summary

	// Customer context — includes entity-specific data if name was extracted from query
	customerCtx := a.getCustomerContext(intent.EntityName)
	if !hasFinanceAccess {
		a.redactFinancialFromContext(customerCtx)
	}
	context["customer_data"] = customerCtx

	// Supplier context — entity-specific if supplier name extracted, always top suppliers
	context["supplier_data"] = a.getSupplierContext(intent.EntityName)

	// Financial context (only if user has finance:view permission)
	if hasFinanceAccess {
		context["financial_data"] = a.getFinancialContext()
		context["ar_summary"] = a.getARSummary()
	}

	// Operations context (always included — non-sensitive pipeline data)
	context["operations_data"] = a.getOperationsContext()
	context["po_summary"] = a.getPurchaseOrderSummary()

	// Risk context (financial amounts redacted for non-finance users)
	riskCtx := a.getRiskContext()
	if !hasFinanceAccess {
		delete(riskCtx, "total_overdue_bhd")
		delete(riskCtx, "total_overdue_ap_bhd")
		delete(riskCtx, "overdue_invoices")
		delete(riskCtx, "overdue_supplier_invoices")
	}
	context["risk_data"] = riskCtx

	// Banking context (cash position, unreconciled lines, outstanding cheques)
	if hasFinanceAccess {
		context["banking_data"] = a.getBankingContext()
	}

	// Credit notes (outstanding credits against customers)
	if hasFinanceAccess {
		context["credit_notes"] = a.getCreditNoteContext()
	}

	// Product catalog (always — needed for quoting and procurement discussions)
	context["product_catalog"] = a.getProductContext()

	// Delivery pipeline (active DNs and recent completions)
	context["delivery_data"] = a.getDeliveryContext()

	// Forward-looking intelligence (pipeline projections, payment predictions, win rates, trends)
	context["forecast_intelligence"] = a.getForecastContext()

	// Serial number overview (inventory status and traceability)
	context["serial_inventory"] = a.getSerialContext()

	return context
}

// getBusinessSummary provides high-level business metrics
func (a *App) getBusinessSummary() map[string]interface{} {
	summary := make(map[string]interface{})

	var customerCount, supplierCount, invoiceCount, orderCount int64
	a.db.Model(&CustomerMaster{}).Count(&customerCount)
	a.db.Model(&SupplierMaster{}).Count(&supplierCount)
	a.db.Model(&Invoice{}).Count(&invoiceCount)
	a.db.Model(&Order{}).Count(&orderCount)

	summary["total_customers"] = customerCount
	summary["total_suppliers"] = supplierCount
	summary["total_invoices"] = invoiceCount
	summary["total_orders"] = orderCount

	// Customer grade distribution
	var gradeA, gradeB, gradeC, gradeD int64
	a.db.Model(&CustomerMaster{}).Where("customer_grade = ?", "A").Count(&gradeA)
	a.db.Model(&CustomerMaster{}).Where("customer_grade = ?", "B").Count(&gradeB)
	a.db.Model(&CustomerMaster{}).Where("customer_grade = ?", "C").Count(&gradeC)
	a.db.Model(&CustomerMaster{}).Where("customer_grade = ?", "D").Count(&gradeD)
	summary["grade_distribution"] = map[string]int64{"A": gradeA, "B": gradeB, "C": gradeC, "D": gradeD}

	// Revenue from invoices
	var totalRevenue float64
	a.db.Model(&Invoice{}).Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&totalRevenue)
	summary["total_revenue_bhd"] = totalRevenue

	// Outstanding receivables
	var totalOutstanding float64
	a.db.Model(&Invoice{}).Where("status != ?", "Paid").Select("COALESCE(SUM(outstanding_bhd), 0)").Scan(&totalOutstanding)
	summary["total_outstanding_bhd"] = totalOutstanding

	// Yearly revenue breakdown
	var allInvoices []Invoice
	a.db.Select("grand_total_bhd, invoice_date, status").Find(&allInvoices)
	yearlyRevenue := make(map[string]float64)
	yearlyInvCount := make(map[string]int)
	yearlyPaidCount := make(map[string]int)
	for _, inv := range allInvoices {
		year := inv.InvoiceDate.Format("2006")
		yearlyRevenue[year] += inv.GrandTotalBHD
		yearlyInvCount[year]++
		if inv.Status == "Paid" {
			yearlyPaidCount[year]++
		}
	}
	yearBreakdown := make([]map[string]interface{}, 0)
	for year, rev := range yearlyRevenue {
		yearBreakdown = append(yearBreakdown, map[string]interface{}{
			"year":           year,
			"revenue_bhd":    fmt.Sprintf("%.3f", rev),
			"invoice_count":  yearlyInvCount[year],
			"paid_count":     yearlyPaidCount[year],
			"collection_pct": fmt.Sprintf("%.1f", float64(yearlyPaidCount[year])/float64(yearlyInvCount[year])*100),
		})
	}
	summary["yearly_revenue"] = yearBreakdown

	// Total payments received
	var totalPaymentsReceived float64
	a.db.Model(&Payment{}).Select("COALESCE(SUM(amount_bhd), 0)").Scan(&totalPaymentsReceived)
	summary["total_payments_received_bhd"] = totalPaymentsReceived

	// Top 10 customers by total business
	var topByBusiness []CustomerMaster
	a.db.Where("total_orders_value > 0").Order("total_orders_value DESC").Limit(10).Find(&topByBusiness)
	topBizList := make([]map[string]interface{}, 0)
	for _, c := range topByBusiness {
		topBizList = append(topBizList, map[string]interface{}{
			"name":          c.BusinessName,
			"total_value":   c.TotalOrdersValue,
			"total_orders":  c.TotalOrdersCount,
			"outstanding":   c.OutstandingBHD,
			"grade":         c.CustomerGrade,
		})
	}
	summary["top_customers_by_business"] = topBizList

	return summary
}

// getCustomerContext fetches customer-specific data
func (a *App) getCustomerContext(entityName string) map[string]interface{} {
	result := make(map[string]interface{})

	if entityName != "" {
		// Find specific customer by name (fuzzy match with wildcard escaping)
		var customer CustomerMaster
		escapedName := escapeLikeWildcards(entityName)
		query := "%" + escapedName + "%"
		if err := a.db.Where("business_name LIKE ? ESCAPE '\\'", query).First(&customer).Error; err == nil {
			result["customer"] = map[string]interface{}{
				"id":              customer.CustomerID,
				"name":            customer.BusinessName,
				"grade":           customer.CustomerGrade,
				"payment_grade":   customer.PaymentGrade,
				"avg_payment_days": customer.AvgPaymentDays,
				"outstanding_bhd": customer.OutstandingBHD,
				"overdue_days":    customer.OverdueDays,
				"total_orders":    customer.TotalOrdersCount,
				"total_value":     customer.TotalOrdersValue,
				"industry":        customer.Industry,
				"credit_blocked":  customer.IsCreditBlocked,
				"ar_risk_tier":    customer.ARRiskTier,
			}

			// Get ALL invoices for this customer (for yearly aggregation)
			var invoices []Invoice
			a.db.Where("customer_id = ?", customer.CustomerID).
				Order("invoice_date DESC").Find(&invoices)

			// Build yearly aggregates
			yearlyTotals := make(map[string]float64)
			yearlyCount := make(map[string]int)
			yearlyOutstanding := make(map[string]float64)
			for _, inv := range invoices {
				year := inv.InvoiceDate.Format("2006")
				yearlyTotals[year] += inv.GrandTotalBHD
				yearlyCount[year]++
				yearlyOutstanding[year] += inv.OutstandingBHD
			}

			yearSummary := make([]map[string]interface{}, 0)
			for year, total := range yearlyTotals {
				yearSummary = append(yearSummary, map[string]interface{}{
					"year":         year,
					"total_bhd":    fmt.Sprintf("%.3f", total),
					"invoice_count": yearlyCount[year],
					"outstanding":  fmt.Sprintf("%.3f", yearlyOutstanding[year]),
				})
			}
			result["yearly_business_summary"] = yearSummary

			// Recent invoices (last 10)
			invSummary := make([]map[string]interface{}, 0)
			limit := 10
			if len(invoices) < limit {
				limit = len(invoices)
			}
			for _, inv := range invoices[:limit] {
				invSummary = append(invSummary, map[string]interface{}{
					"number":      inv.InvoiceNumber,
					"date":        inv.InvoiceDate.Format("2006-01-02"),
					"total":       inv.GrandTotalBHD,
					"outstanding": inv.OutstandingBHD,
					"status":      inv.Status,
				})
			}
			result["recent_invoices"] = invSummary

			// Get recent payments
			var payments []Payment
			a.db.Joins("JOIN invoices ON invoices.id = payments.invoice_id").
				Where("invoices.customer_id = ?", customer.CustomerID).
				Order("payments.payment_date DESC").Limit(10).Find(&payments)
			paySummary := make([]map[string]interface{}, 0)
			for _, pay := range payments {
				paySummary = append(paySummary, map[string]interface{}{
					"amount":       pay.AmountBHD,
					"date":         pay.PaymentDate.Format("2006-01-02"),
					"method":       pay.PaymentMethod,
					"days_to_pay":  pay.DaysToPayment,
				})
			}
			result["recent_payments"] = paySummary

			// Customer contacts (decision makers)
			var contacts []CustomerContact
			a.db.Where("customer_id = ?", customer.ID).Find(&contacts)
			contactList := make([]map[string]interface{}, 0)
			for _, c := range contacts {
				contactList = append(contactList, map[string]interface{}{
					"name":       c.ContactName,
					"job_title":  c.JobTitle,
					"email":      c.Email,
					"phone":      c.Phone,
					"is_primary": c.IsPrimaryContact,
				})
			}
			result["contacts"] = contactList

			// Recent orders with line item details
			var orders []Order
			a.db.Where("customer_id = ?", customer.CustomerID).
				Order("order_date DESC").Limit(5).Find(&orders)
			orderSummary := make([]map[string]interface{}, 0)
			for _, o := range orders {
				orderEntry := map[string]interface{}{
					"number":   o.OrderNumber,
					"date":     o.OrderDate.Format("2006-01-02"),
					"total":    o.GrandTotalBHD,
					"status":   o.Status,
					"division": o.Division,
				}
				// Get line items for product mix insight
				var items []OrderItem
				a.db.Where("order_id = ?", o.ID).Find(&items)
				itemList := make([]map[string]interface{}, 0)
				for _, item := range items {
					itemList = append(itemList, map[string]interface{}{
						"equipment":   item.Equipment,
						"model":       item.Model,
						"qty":         item.Quantity,
						"unit_price":  item.UnitPrice,
						"margin_pct":  item.MarginPercent,
					})
				}
				orderEntry["items"] = itemList
				orderSummary = append(orderSummary, orderEntry)
			}
			result["recent_orders_detail"] = orderSummary

			// Entity notes (CRM notes)
			var notes []EntityNote
			a.db.Where("entity_type = ? AND entity_id = ?", "customer", customer.ID).
				Order("created_at DESC").Limit(10).Find(&notes)
			noteList := make([]map[string]interface{}, 0)
			for _, n := range notes {
				noteList = append(noteList, map[string]interface{}{
					"type":    n.NoteType,
					"content": n.Content,
					"date":    n.CreatedAt.Format("2006-01-02"),
				})
			}
			result["notes"] = noteList

			// Pending follow-up tasks
			var tasks []FollowUpTask
			a.db.Where("customer_id = ? AND status IN ?", customer.ID, []string{"pending", "in_progress", "overdue"}).
				Order("due_date ASC").Limit(5).Find(&tasks)
			taskList := make([]map[string]interface{}, 0)
			for _, t := range tasks {
				taskList = append(taskList, map[string]interface{}{
					"title":    t.Title,
					"due_date": t.DueDate.Format("2006-01-02"),
					"status":   t.Status,
					"priority": t.Priority,
				})
			}
			result["pending_tasks"] = taskList

			// Active offers for this customer (pipeline)
			var offers []Offer
			a.db.Where("customer_id = ? AND stage NOT IN ?", customer.CustomerID, []string{"Lost", "Expired"}).
				Order("quotation_date DESC").Limit(5).Find(&offers)
			offerList := make([]map[string]interface{}, 0)
			for _, o := range offers {
				offerList = append(offerList, map[string]interface{}{
					"number":   o.OfferNumber,
					"stage":    o.Stage,
					"value":    o.TotalValueBHD,
					"margin":   o.EstimatedMargin,
					"date":     o.QuotationDate.Format("2006-01-02"),
					"validity": o.ValidityDate.Format("2006-01-02"),
				})
			}
			result["active_offers"] = offerList
		}
	}

	// Top customers by outstanding
	var topCustomers []CustomerMaster
	a.db.Where("outstanding_bhd > 0").Order("outstanding_bhd DESC").Limit(5).Find(&topCustomers)
	topList := make([]map[string]interface{}, 0)
	for _, c := range topCustomers {
		topList = append(topList, map[string]interface{}{
			"name":        c.BusinessName,
			"outstanding": c.OutstandingBHD,
			"grade":       c.CustomerGrade,
			"overdue":     c.OverdueDays,
		})
	}
	result["top_outstanding_customers"] = topList

	return result
}

// redactFinancialFromContext removes financial data from context maps for non-finance users
func (a *App) redactFinancialFromContext(ctx map[string]interface{}) {
	// Redact financial fields from customer data
	if customer, ok := ctx["customer"].(map[string]interface{}); ok {
		delete(customer, "outstanding_bhd")
		delete(customer, "payment_grade")
		delete(customer, "avg_payment_days")
		delete(customer, "overdue_days")
		delete(customer, "total_value")
		delete(customer, "ar_risk_tier")
		delete(customer, "credit_blocked")
	}
	// Remove invoice amounts and payment data entirely
	delete(ctx, "recent_invoices")
	delete(ctx, "recent_payments")
	delete(ctx, "top_outstanding_customers")
}

// getSupplierContext fetches supplier-specific data
func (a *App) getSupplierContext(entityName string) map[string]interface{} {
	result := make(map[string]interface{})

	if entityName != "" {
		var supplier SupplierMaster
		escapedName := escapeLikeWildcards(entityName)
		query := "%" + escapedName + "%"
		if err := a.db.Where("supplier_name LIKE ? ESCAPE '\\'", query).First(&supplier).Error; err == nil {
			result["supplier"] = map[string]interface{}{
				"id":            supplier.ID,
				"name":          supplier.SupplierName,
				"type":          supplier.SupplierType,
				"country":       supplier.Country,
				"rating":        supplier.Rating,
				"lead_time_days": supplier.LeadTimeDays,
				"brands":        supplier.BrandsHandled,
				"contact":       supplier.PrimaryContact,
				"email":         supplier.Email,
			}

			// Get ALL POs for this supplier (for yearly aggregation)
			var pos []PurchaseOrder
			a.db.Where("supplier_id = ?", supplier.ID).
				Order("po_date DESC").Find(&pos)

			// Build yearly aggregates
			yearlyPO := make(map[string]float64)
			yearlyPOCount := make(map[string]int)
			for _, po := range pos {
				year := po.PODate.Format("2006")
				yearlyPO[year] += po.TotalBHD
				yearlyPOCount[year]++
			}
			yearlyPOSummary := make([]map[string]interface{}, 0)
			for year, total := range yearlyPO {
				yearlyPOSummary = append(yearlyPOSummary, map[string]interface{}{
					"year":     year,
					"total_bhd": fmt.Sprintf("%.3f", total),
					"po_count": yearlyPOCount[year],
				})
			}
			result["yearly_purchase_summary"] = yearlyPOSummary

			// Recent POs (last 10)
			poSummary := make([]map[string]interface{}, 0)
			var totalPOValue float64
			limit := 10
			if len(pos) < limit {
				limit = len(pos)
			}
			for _, po := range pos[:limit] {
				totalPOValue += po.TotalBHD
				poSummary = append(poSummary, map[string]interface{}{
					"number": po.PONumber,
					"date":   po.PODate.Format("2006-01-02"),
					"total":  po.TotalBHD,
					"status": po.Status,
				})
			}
			result["recent_purchase_orders"] = poSummary
			result["total_po_value_recent"] = totalPOValue

			// Supplier invoices
			var supInvoices []SupplierInvoice
			a.db.Where("supplier_id = ?", supplier.ID).
				Order("invoice_date DESC").Limit(10).Find(&supInvoices)
			supInvSummary := make([]map[string]interface{}, 0)
			var totalSupInvValue float64
			for _, si := range supInvoices {
				totalSupInvValue += si.TotalBHD
				supInvSummary = append(supInvSummary, map[string]interface{}{
					"number": si.InvoiceNumber,
					"date":   si.InvoiceDate.Format("2006-01-02"),
					"total":  si.TotalBHD,
					"status": si.Status,
				})
			}
			result["recent_supplier_invoices"] = supInvSummary
			result["total_supplier_invoice_value"] = totalSupInvValue

			// Supplier contacts
			var contacts []SupplierContact
			a.db.Where("supplier_id = ?", supplier.ID).Find(&contacts)
			contactList := make([]map[string]interface{}, 0)
			for _, c := range contacts {
				contactList = append(contactList, map[string]interface{}{
					"name":       c.ContactName,
					"job_title":  c.JobTitle,
					"email":      c.Email,
					"phone":      c.Phone,
					"is_primary": c.IsPrimaryContact,
				})
			}
			result["contacts"] = contactList

			// Entity notes (CRM notes for supplier)
			var notes []EntityNote
			a.db.Where("entity_type = ? AND entity_id = ?", "supplier", supplier.ID).
				Order("created_at DESC").Limit(10).Find(&notes)
			noteList := make([]map[string]interface{}, 0)
			for _, n := range notes {
				noteList = append(noteList, map[string]interface{}{
					"type":    n.NoteType,
					"content": n.Content,
					"date":    n.CreatedAt.Format("2006-01-02"),
				})
			}
			result["notes"] = noteList

			// Supplier issues (quality/delivery problems)
			var issues []SupplierIssue
			a.db.Where("supplier_id = ?", supplier.ID).
				Order("created_at DESC").Limit(5).Find(&issues)
			issueList := make([]map[string]interface{}, 0)
			for _, iss := range issues {
				issueList = append(issueList, map[string]interface{}{
					"order_ref":   iss.OrderRef,
					"description": iss.Description,
					"status":      iss.Status,
					"cost_bhd":    iss.CostBHD,
				})
			}
			result["issues"] = issueList

			// Supplier payments summary
			var supPayments []SupplierPayment
			a.db.Where("supplier_id = ?", supplier.ID).
				Order("payment_date DESC").Limit(10).Find(&supPayments)
			supPayList := make([]map[string]interface{}, 0)
			for _, sp := range supPayments {
				supPayList = append(supPayList, map[string]interface{}{
					"amount":   sp.AmountBHD,
					"date":     sp.PaymentDate.Format("2006-01-02"),
					"method":   sp.PaymentMethod,
					"currency": sp.Currency,
				})
			}
			result["recent_payments"] = supPayList

			// GRN quality stats for this supplier
			var grnCount int64
			var grnPassedCount int64
			a.db.Model(&GoodsReceivedNote{}).Joins("JOIN purchase_orders ON purchase_orders.id = goods_received_notes.purchase_order_id").
				Where("purchase_orders.supplier_id = ?", supplier.ID).Count(&grnCount)
			a.db.Model(&GoodsReceivedNote{}).Joins("JOIN purchase_orders ON purchase_orders.id = goods_received_notes.purchase_order_id").
				Where("purchase_orders.supplier_id = ? AND goods_received_notes.qc_status = ?", supplier.ID, "Passed").Count(&grnPassedCount)
			if grnCount > 0 {
				result["grn_quality"] = map[string]interface{}{
					"total_grns":  grnCount,
					"passed":      grnPassedCount,
					"pass_rate":   fmt.Sprintf("%.1f%%", float64(grnPassedCount)/float64(grnCount)*100),
				}
			}
		}
	}

	// Top suppliers by recent PO value (compute from PO table)
	var allSuppliers []SupplierMaster
	a.db.Limit(20).Find(&allSuppliers)
	topList := make([]map[string]interface{}, 0)
	for _, s := range allSuppliers {
		var poTotal float64
		a.db.Model(&PurchaseOrder{}).Where("supplier_id = ?", s.ID).
			Select("COALESCE(SUM(total_bhd), 0)").Scan(&poTotal)
		if poTotal > 0 {
			topList = append(topList, map[string]interface{}{
				"name":      s.SupplierName,
				"total_po":  poTotal,
				"rating":    s.Rating,
				"lead_days": s.LeadTimeDays,
			})
		}
	}
	result["top_suppliers"] = topList

	return result
}

// getFinancialContext fetches financial metrics
func (a *App) getFinancialContext() map[string]interface{} {
	result := make(map[string]interface{})

	// Invoice statistics
	var totalInvoiced, totalPaid, totalOutstanding float64
	a.db.Model(&Invoice{}).Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&totalInvoiced)
	a.db.Model(&Invoice{}).Where("status = ?", "Paid").Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&totalPaid)
	a.db.Model(&Invoice{}).Where("status != ?", "Paid").Select("COALESCE(SUM(outstanding_bhd), 0)").Scan(&totalOutstanding)

	result["total_invoiced_bhd"] = totalInvoiced
	result["total_paid_bhd"] = totalPaid
	result["total_outstanding_bhd"] = totalOutstanding

	// Invoice status counts
	var sentCount, paidCount, overdueCount int64
	a.db.Model(&Invoice{}).Where("status = ?", "Sent").Count(&sentCount)
	a.db.Model(&Invoice{}).Where("status = ?", "Paid").Count(&paidCount)
	a.db.Model(&Invoice{}).Where("status = ?", "Overdue").Count(&overdueCount)
	result["invoice_counts"] = map[string]int64{"sent": sentCount, "paid": paidCount, "overdue": overdueCount}

	// Average days to payment
	var avgDays float64
	a.db.Model(&Payment{}).Select("COALESCE(AVG(days_to_payment), 0)").Scan(&avgDays)
	result["avg_days_to_payment"] = avgDays

	// Recent payments
	var recentPayments []Payment
	a.db.Order("payment_date DESC").Limit(10).Find(&recentPayments)
	payList := make([]map[string]interface{}, 0)
	for _, p := range recentPayments {
		payList = append(payList, map[string]interface{}{
			"invoice":  p.InvoiceNumber,
			"amount":   p.AmountBHD,
			"date":     p.PaymentDate.Format("2006-01-02"),
			"method":   p.PaymentMethod,
		})
	}
	result["recent_payments"] = payList

	// Yearly financial breakdown
	var allInvs []Invoice
	a.db.Select("grand_total_bhd, outstanding_bhd, invoice_date, status, gross_margin_percent").Find(&allInvs)
	yearlyData := make(map[string]map[string]float64)
	for _, inv := range allInvs {
		year := inv.InvoiceDate.Format("2006")
		if yearlyData[year] == nil {
			yearlyData[year] = map[string]float64{}
		}
		yearlyData[year]["invoiced"] += inv.GrandTotalBHD
		yearlyData[year]["outstanding"] += inv.OutstandingBHD
		yearlyData[year]["count"]++
		if inv.Status == "Paid" {
			yearlyData[year]["paid_count"]++
			yearlyData[year]["paid_value"] += inv.GrandTotalBHD
		}
		if inv.GrossMarginPercent > 0 {
			yearlyData[year]["margin_sum"] += inv.GrossMarginPercent
			yearlyData[year]["margin_count"]++
		}
	}
	yearlyFinance := make([]map[string]interface{}, 0)
	for year, data := range yearlyData {
		entry := map[string]interface{}{
			"year":            year,
			"total_invoiced":  fmt.Sprintf("%.3f", data["invoiced"]),
			"total_paid":      fmt.Sprintf("%.3f", data["paid_value"]),
			"outstanding":     fmt.Sprintf("%.3f", data["outstanding"]),
			"invoice_count":   int(data["count"]),
			"collection_rate": fmt.Sprintf("%.1f%%", data["paid_count"]/data["count"]*100),
		}
		if data["margin_count"] > 0 {
			entry["avg_margin"] = fmt.Sprintf("%.1f%%", data["margin_sum"]/data["margin_count"])
		}
		yearlyFinance = append(yearlyFinance, entry)
	}
	result["yearly_financial_summary"] = yearlyFinance

	// Supplier payments summary
	var totalSupplierPayments float64
	a.db.Model(&SupplierPayment{}).Select("COALESCE(SUM(amount_bhd), 0)").Scan(&totalSupplierPayments)
	result["total_supplier_payments_bhd"] = totalSupplierPayments

	// Bank accounts (cash position)
	var bankAccounts []BankAccount
	a.db.Where("is_active = ?", true).Find(&bankAccounts)
	bankList := make([]map[string]interface{}, 0)
	var totalBankBalance float64
	for _, ba := range bankAccounts {
		totalBankBalance += ba.CurrentBalance
		bankList = append(bankList, map[string]interface{}{
			"bank":     ba.BankName,
			"account":  ba.AccountName,
			"currency": ba.Currency,
			"balance":  ba.CurrentBalance,
		})
	}
	result["bank_accounts"] = bankList
	result["total_bank_balance_bhd"] = totalBankBalance

	// Current FX rates
	var fxRates []CurrencyExchangeRate
	a.db.Where("effective_to IS NULL").Find(&fxRates)
	rateList := make([]map[string]interface{}, 0)
	for _, fx := range fxRates {
		rateList = append(rateList, map[string]interface{}{
			"currency": fx.CurrencyCode,
			"rate":     fx.Rate,
			"since":    fx.EffectiveFrom.Format("2006-01-02"),
		})
	}
	result["current_fx_rates"] = rateList

	// Accounts payable summary (unpaid supplier invoices)
	var totalAP float64
	var apCount int64
	a.db.Model(&SupplierInvoice{}).Where("status NOT IN ?", []string{"Paid", "Rejected"}).
		Select("COALESCE(SUM(total_bhd), 0)").Scan(&totalAP)
	a.db.Model(&SupplierInvoice{}).Where("status NOT IN ?", []string{"Paid", "Rejected"}).Count(&apCount)
	result["total_payables_bhd"] = totalAP
	result["unpaid_supplier_invoices"] = apCount

	// Division-wise revenue breakdown (PH Trading vs AHS Trading)
	var phRevenue, ahsRevenue float64
	a.db.Model(&Invoice{}).Where("division = ?", "PH Trading").
		Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&phRevenue)
	a.db.Model(&Invoice{}).Where("division = ?", "AHS Trading").
		Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&ahsRevenue)
	result["division_revenue"] = map[string]interface{}{
		"ph_trading":  phRevenue,
		"ahs_trading": ahsRevenue,
	}

	return result
}

// getOperationsContext fetches operations pipeline data
func (a *App) getOperationsContext() map[string]interface{} {
	result := make(map[string]interface{})

	// Offer pipeline
	var totalOffers, quotedOffers, wonOffers, lostOffers int64
	a.db.Model(&Offer{}).Count(&totalOffers)
	a.db.Model(&Offer{}).Where("stage = ?", "Quoted").Count(&quotedOffers)
	a.db.Model(&Offer{}).Where("stage = ?", "Won").Count(&wonOffers)
	a.db.Model(&Offer{}).Where("stage = ?", "Lost").Count(&lostOffers)
	result["offers"] = map[string]int64{
		"total": totalOffers, "quoted": quotedOffers, "won": wonOffers, "lost": lostOffers,
	}

	// Calculate win rate
	if wonOffers+lostOffers > 0 {
		result["win_rate"] = float64(wonOffers) / float64(wonOffers+lostOffers) * 100
	}

	// Order pipeline
	var activeOrders int64
	var orderValue float64
	a.db.Model(&Order{}).Where("status NOT IN ?", []string{"Completed", "Cancelled"}).Count(&activeOrders)
	a.db.Model(&Order{}).Where("status NOT IN ?", []string{"Completed", "Cancelled"}).
		Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&orderValue)
	result["active_orders"] = activeOrders
	result["active_order_value_bhd"] = orderValue

	// PO pipeline
	var activePOs int64
	var poValue float64
	a.db.Model(&PurchaseOrder{}).Where("status NOT IN ?", []string{"Completed", "Cancelled"}).Count(&activePOs)
	a.db.Model(&PurchaseOrder{}).Where("status NOT IN ?", []string{"Completed", "Cancelled"}).
		Select("COALESCE(SUM(total_bhd), 0)").Scan(&poValue)
	result["active_purchase_orders"] = activePOs
	result["active_po_value_bhd"] = poValue

	// Delivery notes pending
	var pendingDN int64
	a.db.Model(&DeliveryNote{}).Where("status != ?", "Delivered").Count(&pendingDN)
	result["pending_deliveries"] = pendingDN

	// RFQ pipeline (RFQs are Offers at stage "RFQ")
	var totalRFQs int64
	a.db.Model(&Offer{}).Where("stage = ?", "RFQ").Count(&totalRFQs)
	result["open_rfqs"] = totalRFQs

	// GRN stats
	var totalGRNs int64
	a.db.Model(&GoodsReceivedNote{}).Count(&totalGRNs)
	result["total_grns"] = totalGRNs

	// Recent orders (last 10 for context)
	var recentOrders []Order
	a.db.Order("created_at DESC").Limit(10).Find(&recentOrders)
	orderList := make([]map[string]interface{}, 0)
	for _, o := range recentOrders {
		orderList = append(orderList, map[string]interface{}{
			"number":   o.OrderNumber,
			"customer": o.CustomerName,
			"total":    o.GrandTotalBHD,
			"status":   o.Status,
			"date":     o.OrderDate.Format("2006-01-02"),
		})
	}
	result["recent_orders"] = orderList

	// Offer pipeline value
	var totalOfferValue float64
	a.db.Model(&Offer{}).Where("stage NOT IN ?", []string{"Lost", "Cancelled"}).
		Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&totalOfferValue)
	result["active_offer_pipeline_bhd"] = totalOfferValue

	// Recent offers with item-level detail (product mix in pipeline)
	var recentOffers []Offer
	a.db.Where("stage NOT IN ?", []string{"Lost", "Expired"}).
		Order("quotation_date DESC").Limit(5).Find(&recentOffers)
	offerDetailList := make([]map[string]interface{}, 0)
	for _, o := range recentOffers {
		offerEntry := map[string]interface{}{
			"number":   o.OfferNumber,
			"customer": o.CustomerName,
			"stage":    o.Stage,
			"value":    o.TotalValueBHD,
			"margin":   o.EstimatedMargin,
			"date":     o.QuotationDate.Format("2006-01-02"),
		}
		var items []OfferItem
		a.db.Where("offer_id = ?", o.ID).Find(&items)
		itemList := make([]map[string]interface{}, 0)
		for _, item := range items {
			itemList = append(itemList, map[string]interface{}{
				"equipment":   item.Equipment,
				"model":       item.Model,
				"qty":         item.Quantity,
				"unit_price":  item.UnitPrice,
				"margin_pct":  item.MarginPercent,
				"currency":    item.Currency,
			})
		}
		offerEntry["items"] = itemList
		offerDetailList = append(offerDetailList, offerEntry)
	}
	result["recent_offers_detail"] = offerDetailList

	// Delivery fulfillment status
	var totalDN, deliveredDN, partialDN int64
	a.db.Model(&DeliveryNote{}).Count(&totalDN)
	a.db.Model(&DeliveryNote{}).Where("status = ?", "Delivered").Count(&deliveredDN)
	a.db.Model(&DeliveryNote{}).Where("is_partial_delivery = ?", true).Count(&partialDN)
	result["delivery_stats"] = map[string]interface{}{
		"total":     totalDN,
		"delivered": deliveredDN,
		"partial":   partialDN,
	}

	// GRN quality overview
	var totalGRNItems int64
	var rejectedItems float64
	a.db.Model(&GRNItem{}).Count(&totalGRNItems)
	a.db.Model(&GRNItem{}).Select("COALESCE(SUM(quantity_rejected), 0)").Scan(&rejectedItems)
	result["grn_quality"] = map[string]interface{}{
		"total_items":      totalGRNItems,
		"total_rejected":   rejectedItems,
	}

	// Pending offer follow-ups (upcoming pipeline actions)
	var pendingFollowUps []OfferFollowUp
	a.db.Where("status = ?", "pending").Order("follow_up_date ASC").Limit(5).Find(&pendingFollowUps)
	followUpList := make([]map[string]interface{}, 0)
	for _, fu := range pendingFollowUps {
		// Get offer number for context
		var offer Offer
		offerNum := ""
		if err := a.db.Select("offer_number, customer_name").Where("id = ?", fu.OfferID).First(&offer).Error; err == nil {
			offerNum = offer.OfferNumber
		}
		followUpList = append(followUpList, map[string]interface{}{
			"offer":     offerNum,
			"customer":  offer.CustomerName,
			"date":      fu.FollowUpDate.Format("2006-01-02"),
			"notes":     fu.Notes,
		})
	}
	result["pending_follow_ups"] = followUpList

	// Product catalog summary
	var totalProducts, activeProducts int64
	a.db.Model(&ProductMaster{}).Count(&totalProducts)
	a.db.Model(&ProductMaster{}).Where("is_active = ?", true).Count(&activeProducts)
	result["product_catalog"] = map[string]interface{}{
		"total":  totalProducts,
		"active": activeProducts,
	}

	// Opportunity pipeline (Three-Regime dynamics)
	var opportunities []Opportunity
	a.db.Where("stage NOT IN ?", []string{"Won", "Lost", "Expired"}).
		Order("confidence DESC").Limit(10).Find(&opportunities)
	oppList := make([]map[string]interface{}, 0)
	for _, opp := range opportunities {
		oppList = append(oppList, map[string]interface{}{
			"folder":     opp.FolderNumber,
			"customer":   opp.CustomerName,
			"stage":      opp.Stage,
			"revenue":    opp.RevenueBHD,
			"confidence": opp.Confidence,
			"regime":     opp.Regime,
		})
	}
	result["active_opportunities"] = oppList

	return result
}

// getRiskContext fetches risk-related data
func (a *App) getRiskContext() map[string]interface{} {
	result := make(map[string]interface{})

	// Overdue invoices
	var overdueInvoices []Invoice
	a.db.Where("status = ? OR (status = ? AND due_date < ?)", "Overdue", "Sent", time.Now()).
		Order("outstanding_bhd DESC").Limit(10).Find(&overdueInvoices)
	overdueList := make([]map[string]interface{}, 0)
	var totalOverdue float64
	for _, inv := range overdueInvoices {
		daysOverdue := int(time.Since(inv.DueDate).Hours() / 24)
		if daysOverdue < 0 {
			daysOverdue = 0
		}
		totalOverdue += inv.OutstandingBHD
		overdueList = append(overdueList, map[string]interface{}{
			"invoice":      inv.InvoiceNumber,
			"customer":     inv.CustomerName,
			"outstanding":  inv.OutstandingBHD,
			"days_overdue": daysOverdue,
		})
	}
	result["overdue_invoices"] = overdueList
	result["total_overdue_bhd"] = totalOverdue

	// Credit-blocked customers
	var blockedCustomers []CustomerMaster
	a.db.Where("is_credit_blocked = ?", true).Find(&blockedCustomers)
	blockedList := make([]string, 0)
	for _, c := range blockedCustomers {
		blockedList = append(blockedList, c.BusinessName)
	}
	result["credit_blocked_customers"] = blockedList

	// High-risk AR tiers
	var criticalCount, highCount int64
	a.db.Model(&CustomerMaster{}).Where("ar_risk_tier = ?", "Critical").Count(&criticalCount)
	a.db.Model(&CustomerMaster{}).Where("ar_risk_tier = ?", "High").Count(&highCount)
	result["critical_risk_customers"] = criticalCount
	result["high_risk_customers"] = highCount

	// Active alerts
	var alerts []Alert
	a.db.Where("is_active = ? AND is_acknowledged = ?", true, false).
		Order("CASE severity WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END").
		Limit(10).Find(&alerts)
	alertList := make([]map[string]interface{}, 0)
	for _, al := range alerts {
		alertList = append(alertList, map[string]interface{}{
			"type":     al.AlertType,
			"severity": al.Severity,
			"title":    al.Title,
			"message":  al.Message,
		})
	}
	result["active_alerts"] = alertList

	// Overdue supplier invoices (AP risk)
	var overdueSupInv []SupplierInvoice
	a.db.Where("status NOT IN ? AND due_date < ?", []string{"Paid", "Rejected"}, time.Now()).
		Order("total_bhd DESC").Limit(10).Find(&overdueSupInv)
	overdueAPList := make([]map[string]interface{}, 0)
	var totalOverdueAP float64
	for _, si := range overdueSupInv {
		daysOverdue := int(time.Since(si.DueDate).Hours() / 24)
		totalOverdueAP += si.TotalBHD
		overdueAPList = append(overdueAPList, map[string]interface{}{
			"supplier":     si.SupplierName,
			"invoice":      si.InvoiceNumber,
			"amount":       si.TotalBHD,
			"days_overdue": daysOverdue,
		})
	}
	result["overdue_supplier_invoices"] = overdueAPList
	result["total_overdue_ap_bhd"] = totalOverdueAP

	// Expiring offers (validity ending soon - opportunity risk)
	var expiringOffers []Offer
	a.db.Where("stage = ? AND validity_date BETWEEN ? AND ?", "Quoted",
		time.Now(), time.Now().AddDate(0, 0, 14)).
		Order("validity_date ASC").Limit(5).Find(&expiringOffers)
	expiringList := make([]map[string]interface{}, 0)
	for _, o := range expiringOffers {
		expiringList = append(expiringList, map[string]interface{}{
			"number":   o.OfferNumber,
			"customer": o.CustomerName,
			"value":    o.TotalValueBHD,
			"expires":  o.ValidityDate.Format("2006-01-02"),
		})
	}
	result["expiring_offers"] = expiringList

	// Overdue follow-up tasks
	var overdueTasks []FollowUpTask
	a.db.Where("status IN ? AND due_date < ?", []string{"pending", "overdue"}, time.Now()).
		Order("due_date ASC").Limit(10).Find(&overdueTasks)
	taskList := make([]map[string]interface{}, 0)
	for _, t := range overdueTasks {
		taskList = append(taskList, map[string]interface{}{
			"title":    t.Title,
			"due_date": t.DueDate.Format("2006-01-02"),
			"priority": t.Priority,
			"type":     t.Type,
		})
	}
	result["overdue_tasks"] = taskList

	return result
}

// getARSummary provides accounts receivable summary
func (a *App) getARSummary() map[string]interface{} {
	result := make(map[string]interface{})

	var totalAR, overdueAR float64
	a.db.Model(&Invoice{}).Where("status != ?", "Paid").
		Select("COALESCE(SUM(outstanding_bhd), 0)").Scan(&totalAR)
	a.db.Model(&Invoice{}).Where("status = ?", "Overdue").
		Select("COALESCE(SUM(outstanding_bhd), 0)").Scan(&overdueAR)

	result["total_receivables_bhd"] = totalAR
	result["overdue_receivables_bhd"] = overdueAR
	if totalAR > 0 {
		result["overdue_percentage"] = (overdueAR / totalAR) * 100
	}

	return result
}

// getBankingContext fetches bank accounts, unreconciled lines, and cheque position
func (a *App) getBankingContext() map[string]interface{} {
	result := make(map[string]interface{})

	// Active bank accounts with balances (from latest bank statement per account)
	var accounts []CompanyBankAccount
	a.db.Where("is_active = ?", true).Find(&accounts)
	accountList := make([]map[string]interface{}, 0)
	var totalCashBHD float64
	for _, acc := range accounts {
		// Get closing balance from most recent statement
		var latestStmt BankStatement
		balance := 0.0
		if err := a.db.Where("bank_account_id = ?", acc.ID).
			Order("period_end DESC").First(&latestStmt).Error; err == nil {
			balance = latestStmt.ClosingBalance
		}
		totalCashBHD += balance
		accountList = append(accountList, map[string]interface{}{
			"name":         acc.AccountName,
			"bank":         acc.BankName,
			"currency":     acc.Currency,
			"balance":      balance,
			"booking_rate": acc.BookingRate,
		})
	}
	result["bank_accounts"] = accountList
	result["total_cash_bhd"] = totalCashBHD

	// Unreconciled statement lines (pending reconciliation work)
	var unmatchedCount int64
	var unmatchedCredits, unmatchedDebits float64
	a.db.Model(&BankStatementLine{}).Where("is_matched = ?", false).Count(&unmatchedCount)
	a.db.Model(&BankStatementLine{}).Where("is_matched = ? AND credit > 0", false).
		Select("COALESCE(SUM(credit), 0)").Scan(&unmatchedCredits)
	a.db.Model(&BankStatementLine{}).Where("is_matched = ? AND debit > 0", false).
		Select("COALESCE(SUM(debit), 0)").Scan(&unmatchedDebits)
	result["unreconciled_lines"] = unmatchedCount
	result["unreconciled_credits_bhd"] = unmatchedCredits
	result["unreconciled_debits_bhd"] = unmatchedDebits

	// Outstanding cheques (ISSUED or PRESENTED — money gone but not cleared)
	var outstandingCheques []OutstandingCheque
	a.db.Where("status IN ?", []string{"ISSUED", "PRESENTED"}).
		Order("issued_date DESC").Limit(20).Find(&outstandingCheques)
	chequeList := make([]map[string]interface{}, 0)
	var totalOutstandingCheques float64
	for _, ch := range outstandingCheques {
		totalOutstandingCheques += ch.Amount
		chequeList = append(chequeList, map[string]interface{}{
			"payee":   ch.PayeeName,
			"amount":  ch.Amount,
			"status":  ch.Status,
			"number":  ch.ChequeNumber,
			"date":    ch.IssuedDate.Format("2006-01-02"),
			"purpose": ch.Purpose,
		})
	}
	result["outstanding_cheques"] = chequeList
	result["total_outstanding_cheques_bhd"] = totalOutstandingCheques

	// Problem cheques (STALE or BOUNCED — need action)
	var problemCheques []OutstandingCheque
	a.db.Where("status IN ?", []string{"STALE", "BOUNCED"}).Find(&problemCheques)
	problemList := make([]map[string]interface{}, 0)
	for _, ch := range problemCheques {
		problemList = append(problemList, map[string]interface{}{
			"payee":  ch.PayeeName,
			"amount": ch.Amount,
			"status": ch.Status,
			"number": ch.ChequeNumber,
		})
	}
	result["problem_cheques"] = problemList

	return result
}

// getCreditNoteContext fetches credit note pipeline (Draft → Issued → Applied)
func (a *App) getCreditNoteContext() map[string]interface{} {
	result := make(map[string]interface{})

	// Counts by status
	var draftCount, issuedCount, appliedCount int64
	a.db.Model(&CreditNote{}).Where("status = ?", "Draft").Count(&draftCount)
	a.db.Model(&CreditNote{}).Where("status = ?", "Issued").Count(&issuedCount)
	a.db.Model(&CreditNote{}).Where("status = ?", "Applied").Count(&appliedCount)
	result["credit_note_counts"] = map[string]int64{
		"draft": draftCount, "issued": issuedCount, "applied": appliedCount,
	}

	// Outstanding credit (Issued but not yet applied — reduces future AR)
	var outstandingCredit float64
	a.db.Model(&CreditNote{}).Where("status = ?", "Issued").
		Select("COALESCE(SUM(grand_total_bhd), 0)").Scan(&outstandingCredit)
	result["outstanding_credit_bhd"] = outstandingCredit

	// Recent credit notes (last 10)
	var recentCNs []CreditNote
	a.db.Order("credit_note_date DESC").Limit(10).Find(&recentCNs)
	cnList := make([]map[string]interface{}, 0)
	for _, cn := range recentCNs {
		cnList = append(cnList, map[string]interface{}{
			"number":   cn.CreditNoteNumber,
			"customer": cn.CustomerName,
			"amount":   cn.GrandTotalBHD,
			"status":   cn.Status,
			"reason":   cn.Reason,
			"date":     cn.CreditNoteDate.Format("2006-01-02"),
		})
	}
	result["recent_credit_notes"] = cnList

	return result
}

// getProductContext fetches the full product catalog with pricing and sales velocity
func (a *App) getProductContext() map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	// Full product catalog
	var products []ProductMaster
	a.db.Where("is_active = ?", true).Find(&products)
	productList := make([]map[string]interface{}, 0)
	categoryMap := make(map[string]int)
	for _, p := range products {
		categoryMap[p.ProductCategory]++
		productList = append(productList, map[string]interface{}{
			"code":           p.ProductCode,
			"name":           p.ProductName,
			"category":       p.ProductCategory,
			"standard_cost":  p.StandardCostBHD,
			"standard_price": p.StandardPriceBHD,
			"stock_qty":      p.StockQuantity,
			"supplier_code":  p.SupplierCode,
			"part_number":    p.PartNumber,
			"description":    p.Description,
		})
	}
	result["products"] = productList
	result["product_categories"] = categoryMap
	result["total_active_products"] = len(products)

	// Top products by order frequency (which products get ordered most)
	type productFreq struct {
		Equipment string
		Model     string
		Count     int64
		TotalQty  float64
	}
	var topProducts []productFreq
	a.db.Model(&OrderItem{}).
		Select("equipment, model, COUNT(*) as count, SUM(quantity) as total_qty").
		Group("equipment, model").
		Order("count DESC").
		Limit(10).
		Scan(&topProducts)
	topList := make([]map[string]interface{}, 0)
	for _, tp := range topProducts {
		topList = append(topList, map[string]interface{}{
			"equipment": tp.Equipment,
			"model":     tp.Model,
			"order_count": tp.Count,
			"total_qty": tp.TotalQty,
		})
	}
	result["top_ordered_products"] = topList

	return result
}

// getDeliveryContext fetches active and recent delivery notes
func (a *App) getDeliveryContext() map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	// Active deliveries (Prepared + Dispatched — in progress)
	var activeDNs []DeliveryNote
	a.db.Where("status IN ?", []string{"Prepared", "Dispatched"}).
		Order("delivery_date ASC").Find(&activeDNs)
	activeList := make([]map[string]interface{}, 0)
	for _, dn := range activeDNs {
		activeList = append(activeList, map[string]interface{}{
			"dn_number":   dn.DNNumber,
			"customer_id": dn.CustomerID,
			"status":      dn.Status,
			"date":        dn.DeliveryDate.Format("2006-01-02"),
			"transport":   dn.TransportMethod,
			"driver":      dn.DriverName,
			"contact":     dn.ContactPerson,
		})
	}
	result["active_deliveries"] = activeList
	result["active_delivery_count"] = len(activeDNs)

	// Recent completed deliveries (last 20)
	var recentDNs []DeliveryNote
	a.db.Where("status = ?", "Delivered").
		Order("delivery_date DESC").Limit(20).Find(&recentDNs)
	recentList := make([]map[string]interface{}, 0)
	for _, dn := range recentDNs {
		recentList = append(recentList, map[string]interface{}{
			"dn_number":   dn.DNNumber,
			"customer_id": dn.CustomerID,
			"date":        dn.DeliveryDate.Format("2006-01-02"),
			"partial":     dn.IsPartialDelivery,
		})
	}
	result["recent_deliveries"] = recentList

	// Delivery stats by transport method
	type transportStat struct {
		TransportMethod string
		Count           int64
	}
	var transportStats []transportStat
	a.db.Model(&DeliveryNote{}).
		Select("transport_method, COUNT(*) as count").
		Group("transport_method").
		Scan(&transportStats)
	statsMap := make(map[string]int64)
	for _, ts := range transportStats {
		if ts.TransportMethod != "" {
			statsMap[ts.TransportMethod] = ts.Count
		}
	}
	result["transport_breakdown"] = statsMap

	return result
}

// getForecastContext builds forward-looking intelligence: pipeline projections,
// payment predictions, win probabilities, and revenue trends by month
func (a *App) getForecastContext() map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	// --- Pipeline by stage with weighted value ---
	type stageData struct {
		Stage      string
		Count      int64
		TotalValue float64
	}
	var stageBreakdown []stageData
	a.db.Model(&Opportunity{}).
		Where("stage NOT IN ?", []string{"Won", "Lost", "Expired"}).
		Select("stage, COUNT(*) as count, SUM(revenue_bhd) as total_value").
		Group("stage").
		Scan(&stageBreakdown)
	pipelineStages := make([]map[string]interface{}, 0)
	var weightedPipeline float64
	stageWeights := map[string]float64{
		"Lead": 0.10, "Qualified": 0.25, "Proposal": 0.50, "Negotiation": 0.75,
	}
	for _, s := range stageBreakdown {
		w := stageWeights[s.Stage]
		if w == 0 {
			w = 0.50
		}
		weighted := s.TotalValue * w
		weightedPipeline += weighted
		pipelineStages = append(pipelineStages, map[string]interface{}{
			"stage":            s.Stage,
			"count":            s.Count,
			"total_value_bhd":  fmt.Sprintf("%.3f", s.TotalValue),
			"weighted_bhd":     fmt.Sprintf("%.3f", weighted),
			"close_probability": fmt.Sprintf("%.0f%%", w*100),
		})
	}
	result["pipeline_by_stage"] = pipelineStages
	result["weighted_pipeline_bhd"] = fmt.Sprintf("%.3f", weightedPipeline)

	// --- Opportunities closing soon (next 60 days) ---
	var closingSoon []Opportunity
	a.db.Where("stage NOT IN ? AND expected_date BETWEEN ? AND ?",
		[]string{"Won", "Lost", "Expired"},
		time.Now(), time.Now().AddDate(0, 0, 60)).
		Order("expected_date ASC").Limit(10).Find(&closingSoon)
	closingList := make([]map[string]interface{}, 0)
	for _, opp := range closingSoon {
		expectedStr := ""
		if opp.ExpectedDate != nil {
			expectedStr = opp.ExpectedDate.Format("2006-01-02")
		}
		closingList = append(closingList, map[string]interface{}{
			"folder":      opp.FolderNumber,
			"customer":    opp.CustomerName,
			"stage":       opp.Stage,
			"revenue":     opp.RevenueBHD,
			"profit":      opp.ProfitBHD,
			"confidence":  opp.Confidence,
			"expected":    expectedStr,
			"salesperson": opp.Salesperson,
			"regime":      opp.Regime,
		})
	}
	result["closing_soon"] = closingList

	// --- Payment predictions by customer grade ---
	var predictions []PredictionRecord
	a.db.Order("confidence DESC").Limit(20).Find(&predictions)
	predList := make([]map[string]interface{}, 0)
	for _, p := range predictions {
		predList = append(predList, map[string]interface{}{
			"customer":       p.CustomerName,
			"grade":          p.Grade,
			"predicted_days": p.PredictedDays,
			"confidence":     fmt.Sprintf("%.0f%%", p.Confidence*100),
		})
	}
	result["payment_predictions"] = predList

	// --- Monthly revenue trend (last 12 months from invoices) ---
	var allInvoices []Invoice
	twelveMonthsAgo := time.Now().AddDate(0, -12, 0)
	a.db.Where("invoice_date > ? AND status IN ?", twelveMonthsAgo,
		[]string{"Sent", "Paid", "PartiallyPaid", "Overdue"}).
		Select("invoice_date, grand_total_bhd, status").Find(&allInvoices)
	monthlyRevenue := make(map[string]float64)
	for _, inv := range allInvoices {
		key := inv.InvoiceDate.Format("2006-01")
		monthlyRevenue[key] += inv.GrandTotalBHD
	}
	result["monthly_revenue_last_12m"] = monthlyRevenue

	// --- Win rate by product type (using Opportunity) ---
	type productWin struct {
		ProductType string
		Total       int64
		Won         int64
	}
	var productWins []productWin
	a.db.Model(&Opportunity{}).
		Select("product_type, COUNT(*) as total, SUM(CASE WHEN stage = 'Won' THEN 1 ELSE 0 END) as won").
		Where("product_type != ''").
		Group("product_type").
		Scan(&productWins)
	winByProduct := make([]map[string]interface{}, 0)
	for _, pw := range productWins {
		rate := 0.0
		if pw.Total > 0 {
			rate = float64(pw.Won) / float64(pw.Total) * 100
		}
		winByProduct = append(winByProduct, map[string]interface{}{
			"product_type": pw.ProductType,
			"total":        pw.Total,
			"won":          pw.Won,
			"win_rate":     fmt.Sprintf("%.1f%%", rate),
		})
	}
	result["win_rate_by_product"] = winByProduct

	// --- Salesperson pipeline performance ---
	type salespersonPerf struct {
		Salesperson string
		OpenCount   int64
		TotalValue  float64
	}
	var salesPerf []salespersonPerf
	a.db.Model(&Opportunity{}).
		Where("stage NOT IN ? AND salesperson != ''", []string{"Won", "Lost", "Expired"}).
		Select("salesperson, COUNT(*) as open_count, SUM(revenue_bhd) as total_value").
		Group("salesperson").
		Order("total_value DESC").
		Scan(&salesPerf)
	salesList := make([]map[string]interface{}, 0)
	for _, sp := range salesPerf {
		salesList = append(salesList, map[string]interface{}{
			"salesperson": sp.Salesperson,
			"open_opps":   sp.OpenCount,
			"pipeline_bhd": fmt.Sprintf("%.3f", sp.TotalValue),
		})
	}
	result["salesperson_pipeline"] = salesList

	return result
}

// getSerialContext fetches serial number inventory and traceability overview
func (a *App) getSerialContext() map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	// Status distribution
	type statusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []statusCount
	a.db.Model(&SerialNumber{}).
		Select("status, COUNT(*) as count").
		Group("status").
		Scan(&statusCounts)
	statusMap := make(map[string]int64)
	for _, sc := range statusCounts {
		statusMap[sc.Status] = sc.Count
	}
	result["serial_status_distribution"] = statusMap

	// Shipped but not yet confirmed delivered (in-transit risk)
	var shippedNotDelivered int64
	a.db.Model(&SerialNumber{}).Where("status = ?", "Shipped").Count(&shippedNotDelivered)
	result["serials_in_transit"] = shippedNotDelivered

	// Recently received (last 30 days)
	var recentlyReceived int64
	a.db.Model(&SerialNumber{}).
		Where("created_at > ?", time.Now().AddDate(0, 0, -30)).
		Count(&recentlyReceived)
	result["serials_received_last_30d"] = recentlyReceived

	// Serials by product (top 10)
	type productSerial struct {
		ProductCode string
		Count       int64
	}
	var productSerials []productSerial
	a.db.Model(&SerialNumber{}).
		Select("product_code, COUNT(*) as count").
		Group("product_code").
		Order("count DESC").
		Limit(10).
		Scan(&productSerials)
	serialsByProduct := make([]map[string]interface{}, 0)
	for _, ps := range productSerials {
		serialsByProduct = append(serialsByProduct, map[string]interface{}{
			"product_code": ps.ProductCode,
			"count":        ps.Count,
		})
	}
	result["serials_by_product"] = serialsByProduct

	return result
}

// getPurchaseOrderSummary provides PO overview
func (a *App) getPurchaseOrderSummary() map[string]interface{} {
	result := make(map[string]interface{})

	var totalPOs int64
	var totalValue float64
	a.db.Model(&PurchaseOrder{}).Count(&totalPOs)
	a.db.Model(&PurchaseOrder{}).Select("COALESCE(SUM(total_bhd), 0)").Scan(&totalValue)

	result["total_pos"] = totalPOs
	result["total_value_bhd"] = totalValue

	return result
}

// ============================================================================
// THREE-REGIME CALCULATION
// ============================================================================

// calculateSystemRegime determines current system's three-regime state
func (a *App) calculateSystemRegime() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"dominant": "R3", "r1": 0.10, "r2": 0.20, "r3": 0.70, "state": "Stabilization",
		}
	}

	// R1 indicators: new activity, exploration
	var recentOrders int64
	a.db.Model(&Order{}).Where("created_at > ?", time.Now().AddDate(0, 0, -30)).Count(&recentOrders)
	var recentCustomers int64
	a.db.Model(&CustomerMaster{}).Where("created_at > ?", time.Now().AddDate(0, 0, -30)).Count(&recentCustomers)
	r1Score := float64(recentOrders)*0.6 + float64(recentCustomers)*0.4

	// R2 indicators: optimization opportunity
	var avgMargin float64
	a.db.Model(&Invoice{}).Where("gross_margin_percent > 0").
		Select("COALESCE(AVG(gross_margin_percent), 18)").Scan(&avgMargin)
	r2Score := avgMargin

	// R3 indicators: stability/collection pressure
	var overdueCount int64
	a.db.Model(&Invoice{}).Where("status = ?", "Overdue").Count(&overdueCount)
	var totalInvoices int64
	a.db.Model(&Invoice{}).Where("status != ?", "Paid").Count(&totalInvoices)
	r3Score := 50.0
	if totalInvoices > 0 {
		overdueRatio := float64(overdueCount) / float64(totalInvoices)
		r3Score = 50.0 + overdueRatio*50.0 // Higher overdue = more stabilization needed
	}

	// Normalize
	total := r1Score + r2Score + r3Score
	var r1, r2, r3 float64
	if total > 0 {
		r1 = r1Score / total
		r2 = r2Score / total
		r3 = r3Score / total
	} else {
		r1, r2, r3 = 0.10, 0.20, 0.70
	}

	// Determine dominant
	dominant := "R3"
	state := "Stabilization"
	if r1 > r2 && r1 > r3 {
		dominant = "R1"
		state = "Exploration"
	} else if r2 > r1 && r2 > r3 {
		dominant = "R2"
		state = "Optimization"
	}

	return map[string]interface{}{
		"dominant": dominant,
		"r1":       r1,
		"r2":       r2,
		"r3":       r3,
		"state":    state,
	}
}

// ============================================================================
// MISTRAL API CLIENT
// ============================================================================

// callMistral makes HTTP request to Mistral API
func callMistral(model, systemPrompt, userMessage string) (string, error) {
	requestBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"max_tokens":  mistralMaxTokens,
		"temperature": mistralTemperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", mistralAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("request creation error: %w", err)
	}

	// Get API key from app config (fallback to env, then hardcoded for backward compatibility)
	apiKey := getMistralAPIKey()

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: MistralAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response error: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Mistral API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", fmt.Errorf("JSON parse error: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("no response choices from Mistral")
	}

	return apiResponse.Choices[0].Message.Content, nil
}

// callMistralVision sends an image to Mistral's pixtral vision model for OCR
func callMistralVision(base64Image string, mimeType string, prompt string) (string, error) {
	apiKey := getMistralAPIKey()
	if apiKey == "" {
		return "", fmt.Errorf("Mistral API key not configured")
	}

	requestBody := map[string]interface{}{
		"model": "pixtral-large-latest",
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{
						"type": "text",
						"text": prompt,
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": "data:" + mimeType + ";base64," + base64Image,
						},
					},
				},
			},
		},
		"max_tokens":  8192,
		"temperature": 0.1,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", mistralAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("request creation error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: MistralAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response error: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("Mistral Vision API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", fmt.Errorf("JSON parse error: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("no response from Mistral Vision")
	}

	return apiResponse.Choices[0].Message.Content, nil
}

// ============================================================================
// AIML API CLIENT (Grok — primary chat backend)
// ============================================================================

// settingsBasedAIMLKey is injected by the App during startup for settings-based key lookup
var settingsBasedAIMLKey func() string

// SetAIMLKeyProvider allows the App to inject the settings-based AIML key provider
func SetAIMLKeyProvider(provider func() string) {
	settingsBasedAIMLKey = provider
}

// getAIMLAPIKey retrieves the AIML API key from settings, environment, or hardcoded fallback.
// Priority: settings DB → env var → hardcoded fallback (ensures Grok pipeline never breaks).
func getAIMLAPIKey() string {
	// 1. Try settings-based provider (injected by App)
	if settingsBasedAIMLKey != nil {
		if key := strings.TrimSpace(settingsBasedAIMLKey()); key != "" {
			return key
		}
	}
	// 2. Try environment variable
	if key := strings.TrimSpace(os.Getenv("AIML_API_KEY")); key != "" {
		return key
	}
	// 3. Hardcoded fallback — ensures Butler/Grok pipeline always works on deployment
	return "bf395c42f717475798bdfdd56891c208"
}

// callAIML sends a chat request to the AIML API (OpenAI-compatible format).
// Uses aimlModelID (Grok) which supports a ~1M token context window, enabling
// full business context injection without truncation.
func callAIML(apiKey, systemPrompt, userMessage string) (string, error) {
	requestBody := map[string]interface{}{
		"model": aimlModelID,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userMessage},
		},
		"max_tokens":  aimlMaxTokens,
		"temperature": aimlTemperature,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", aimlAPIURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("request creation error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: AimlAPITimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response error: %w", err)
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("AIML API error (status %d): %s", resp.StatusCode, string(body))
	}

	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		return "", fmt.Errorf("JSON parse error: %w", err)
	}

	if len(apiResponse.Choices) == 0 {
		return "", fmt.Errorf("no response choices from AIML API")
	}

	return apiResponse.Choices[0].Message.Content, nil
}

// ============================================================================
// SYSTEM PROMPT BUILDER
// ============================================================================

// buildMistralSystemPrompt creates the PRD-style context-aware system prompt
func buildMistralSystemPrompt(context map[string]interface{}, regime map[string]interface{}) string {
	// Serialize context to JSON for injection
	contextJSON, _ := json.MarshalIndent(context, "", "  ")

	// Sanitize context data to prevent prompt injection from database-sourced values
	contextStr := sanitizeForPrompt(string(contextJSON))
	// Case-insensitive removal of action/system/instruction markers
	contextStr = contextCleanupPatterns.ReplaceAllString(contextStr, "")
	// Cap context size to prevent unbounded API payloads (Mistral max ~32K tokens ≈ 128K chars)
	const maxContextChars = 200000
	if len(contextStr) > maxContextChars {
		contextStr = contextStr[:maxContextChars] + "\n... [context truncated at 200K chars]"
	}

	// Build regime guidance
	regimeGuidance := buildRegimeGuidance(regime)

	prompt := fmt.Sprintf(`You are Butler, the business intelligence AI for PH Trading WLL — a Bahrain-based process instrumentation company. You have access to the company's complete live database and can answer any question the team has.

COMPANY PROFILE:
- Industry: Process Instrumentation & Control Systems (Bahrain)
- Currency: BHD (Bahraini Dinar, 3 decimal places = fils)
- Key Customers: BAPCO, EWA, Alba, GPIC, NOGA, Tatweer, Banagas (government/industrial)
- Key Suppliers: Endress+Hauser (E+H), Servomex, Landis+Gyr, GIC
- Divisions: PH Trading (primary) + AHS Trading (secondary)
- VAT Rate: 10%%

FULL DATA ACCESS — you have live data on:
• CUSTOMERS: 286 active accounts — profiles, grades (A/B/C/D), payment behavior, credit limits, outstanding AR, invoices, orders, contacts, CRM notes, follow-up tasks, active offers, AR risk tiers
• SUPPLIERS: 34 suppliers — profiles, PO history, invoice matching, payment history, GRN quality stats, supplier issues
• SALES PIPELINE: Offers (RFQ → Quotation → Won/Lost), costing sheets, win rates, opportunities by stage/salesperson, ABB competition flags
• ORDERS & INVOICES: 175 orders, 468 customer invoices (2023-2025), invoice status, line items, margins
• PROCUREMENT: 45 POs (multi-currency EUR/USD/BHD), 3-way matching (PO/GRN/Invoice), GRN quality
• DELIVERY: 155 delivery notes — Prepared/Dispatched/Delivered, serial traceability
• FINANCE: AR/AP, bank balances (multiple accounts), FX rates, monthly revenue trends, payment predictions
• BANKING: Bank statements, reconciliation status, outstanding/stale/bounced cheques
• CREDIT NOTES: Draft/Issued/Applied pipeline, outstanding credits
• PRODUCTS: 19 active products — catalog, pricing, margins, order frequency, serial tracking
• FORECASTING: Weighted pipeline by stage, opportunities closing in 60 days, payment day predictions, win rates by product type, salesperson performance
• RISK: Overdue AR/AP, credit-blocked customers, expiring offers, system alerts
• INTELLIGENCE: Three-Regime dynamics (R1=Exploration/R2=Optimization/R3=Stabilization), payment behavior models

COMPLETE BUSINESS DATA:
%s

%s

YOUR CAPABILITIES:
1. Answer ANY question about the business — customers, suppliers, finance, operations, pipeline, risk
2. Discuss FUTURE PROSPECTS — use pipeline data, payment predictions, and win probability to project outcomes
3. STRATEGIC ANALYSIS — recommend actions, identify patterns, surface insights from data trends
4. FORECASTING — estimate future revenue using pipeline × close probability, analyze payment timing risk
5. MARKET INTELLIGENCE — use your knowledge of the instrumentation industry to add context to data
6. WHAT-IF SCENARIOS — reason about hypothetical situations ("what if EWA delays payment?", "what margin do we need to break even on this order?")
7. RELATIONSHIP ANALYSIS — cross-reference customers, orders, offers, and payments to find patterns

RESPONSE GUIDELINES:
- Always cite specific numbers from the context data when available
- For forward-looking questions, clearly distinguish between actual data and projections
- Be as detailed or as brief as the question requires — match depth to complexity
- If data is not in context, say so and offer what you do know
- Suggest next actions when relevant
- For strategic recommendations, ground them in the actual data provided

ACTION FORMAT (when recommending next steps):
[ACTIONS]
[{"type":"NAVIGATE","target":"screen_name","label":"Description","data":{}}]
[/ACTIONS]

Available targets: customers, suppliers, invoices, payments, orders, rfqs, offers, deliveries, dashboard, opportunities, purchase_orders, delivery_notes, bank_reconciliation, cheque_register, cash_position, butler`, contextStr, regimeGuidance)

	return prompt
}

// buildRegimeGuidance generates regime-specific LLM guidance
func buildRegimeGuidance(regime map[string]interface{}) string {
	dominant, _ := regime["dominant"].(string)
	r1, _ := regime["r1"].(float64)
	r2, _ := regime["r2"].(float64)
	r3, _ := regime["r3"].(float64)
	state, _ := regime["state"].(string)

	guidance := fmt.Sprintf("BUSINESS REGIME: %s (%s) [R1=%.0f%% R2=%.0f%% R3=%.0f%%]\n", dominant, state, r1*100, r2*100, r3*100)

	switch dominant {
	case "R1":
		guidance += `REGIME FOCUS: EXPLORATION
- Suggest new business opportunities and market expansion
- Highlight growth potential in customer/supplier relationships
- Recommend calculated risks for new ventures
- Be optimistic but data-grounded`
	case "R2":
		guidance += `REGIME FOCUS: OPTIMIZATION
- Focus on efficiency improvements and margin enhancement
- Suggest cost reduction and process improvements
- Recommend pricing adjustments based on data
- Balance growth with profitability`
	case "R3":
		guidance += `REGIME FOCUS: STABILIZATION
- Prioritize collections and cash flow management
- Flag overdue payments and high-risk accounts
- Recommend conservative credit decisions
- Focus on risk mitigation and working capital`
	}

	return guidance
}

// ============================================================================
// RESPONSE PARSING
// ============================================================================

// parseMistralResponse converts Mistral output to ButlerResponse
func parseMistralResponse(aiText string, context map[string]interface{}, intent Intent) ButlerResponse {
	// Extract actions from [ACTIONS]...[/ACTIONS] blocks
	actions := parseActionBlocks(aiText)

	// Clean the message (remove action blocks from display text)
	cleanMessage := actionBlockRegex.ReplaceAllString(aiText, "")
	cleanMessage = strings.TrimSpace(cleanMessage)

	// Calculate confidence
	confidence := 0.85
	if intent.Confidence < 0.5 {
		confidence = 0.6
	}
	if len(cleanMessage) < 20 {
		confidence = 0.5
	}
	if strings.Contains(strings.ToLower(cleanMessage), "don't have") ||
		strings.Contains(strings.ToLower(cleanMessage), "not available") {
		confidence = 0.4
	}
	if len(actions) > 0 {
		confidence += 0.05 // Slight boost for actionable responses
	}
	if confidence > 1.0 {
		confidence = 1.0
	}

	return ButlerResponse{
		Message:    cleanMessage,
		Actions:    actions,
		Confidence: confidence,
		Context:    context,
	}
}

// parseActionBlocks extracts structured actions from [ACTIONS]...[/ACTIONS] JSON
func parseActionBlocks(text string) []ButlerAction {
	actions := []ButlerAction{}

	matches := actionBlockRegex.FindAllStringSubmatch(text, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		jsonStr := strings.TrimSpace(match[1])

		// Try parsing as array of action objects
		var rawActions []struct {
			Type       string      `json:"type"`
			Target     string      `json:"target"`
			Label      string      `json:"label"`
			Data       interface{} `json:"data"`
			Parameters interface{} `json:"parameters"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &rawActions); err != nil {
			log.Printf("⚠️ Failed to parse action block: %v", err)
			continue
		}

		for _, ra := range rawActions {
			data := ra.Data
			if data == nil {
				data = ra.Parameters
			}
			if data == nil {
				data = ra.Label
			}
			actions = append(actions, ButlerAction{
				Type:   strings.ToLower(ra.Type),
				Target: strings.ToLower(ra.Target),
				Data:   data,
			})
		}
	}

	return actions
}

// ============================================================================
// UTILITY
// ============================================================================

// promptInjectionPatterns matches known Mistral/Llama prompt injection markers (case-insensitive)
var promptInjectionPatterns = regexp.MustCompile(`(?i)(\[/?INST\]|<</?SYS>>|<\||(\|>)|</?s>)`)

// contextCleanupPatterns matches action/system/instruction markers that could hijack responses (case-insensitive)
var contextCleanupPatterns = regexp.MustCompile(`(?i)(\[/?ACTIONS\]|SYSTEM:|INSTRUCTIONS:)`)

// sanitizeForPrompt removes potential prompt injection markers from user-supplied data
// before it is interpolated into Mistral API prompts.
// Does NOT truncate — callers are responsible for their own size limits.
func sanitizeForPrompt(input string) string {
	sanitized := strings.ReplaceAll(input, "```", "")
	sanitized = promptInjectionPatterns.ReplaceAllString(sanitized, "")
	return sanitized
}

// settingsBasedMistralKey is set by the App during startup to provide settings-based key lookup
var settingsBasedMistralKey func() string

// SetMistralKeyProvider allows the App to inject the settings-based key provider
func SetMistralKeyProvider(provider func() string) {
	settingsBasedMistralKey = provider
}

// getMistralAPIKey retrieves API key from settings, then environment
func getMistralAPIKey() string {
	// Helper to check if a key looks valid (not placeholder text)
	isValidKey := func(key string) bool {
		key = strings.TrimSpace(key)
		if key == "" {
			return false
		}
		// Reject placeholder values from settings UI
		lower := strings.ToLower(key)
		if strings.Contains(lower, "not set") || strings.Contains(lower, "not_set") ||
			strings.Contains(lower, "placeholder") || strings.Contains(lower, "your-") ||
			strings.Contains(lower, "your_") || strings.HasPrefix(key, "(") {
			return false
		}
		return true
	}

	// 1. Try settings-based provider (injected by App)
	if settingsBasedMistralKey != nil {
		if key := settingsBasedMistralKey(); isValidKey(key) {
			return key
		}
	}

	// 2. Try environment variable
	if key := os.Getenv("MISTRAL_API_KEY"); isValidKey(key) {
		return key
	}

	// 3. Hardcoded fallback for deployment (ensures client pipeline never breaks)
	return "2vdwji2sDL8ny9JD3MnIxjoNQD6tblbE"
}

// butlerTruncate truncates a string to maxLen characters
func butlerTruncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ============================================================================
// INTELLIGENT OCR ANALYSIS
// ============================================================================

// ButlerOCRInsight represents AI-extracted insights from document text
type ButlerOCRInsight struct {
	Summary           string                   `json:"summary"`            // 1-2 sentence summary
	ExtractedItems    []map[string]interface{} `json:"extracted_items"`    // Line items (flexible: RFQ items OR bank transactions)
	DetectedCustomer  string                   `json:"detected_customer"`  // Customer name if found
	DetectedProject   string                   `json:"detected_project"`   // Project/reference if found
	RequiredDeadline  string                   `json:"required_deadline"`  // Deadline if mentioned
	Confidence        float64                  `json:"confidence"`         // 0.0-1.0 confidence score
	SuggestedActions  []ButlerAction           `json:"suggested_actions"`  // Recommended next steps
	DocumentType      string                   `json:"document_type"`      // RFQ, PO, Invoice, etc.
	ExtractedMetadata map[string]interface{}   `json:"metadata"`           // Key-value pairs extracted (bank name, balances, etc.)
}

// RFQLineItem represents a line item extracted from an RFQ/email
type RFQLineItem struct {
	Description string  `json:"description"`
	Quantity    int     `json:"quantity"`
	Unit        string  `json:"unit"`
	PartNumber  string  `json:"part_number"`
	Notes       string  `json:"notes"`
}

// AnalyzeDocumentWithButler uses Mistral to extract structured insights from document text
// Note: metadata accepts map[string]interface{} for flexibility with OCR extracted_data
func (a *App) AnalyzeDocumentWithButler(text string, docType string, metadata map[string]interface{}) (ButlerOCRInsight, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return ButlerOCRInsight{}, err
	}
	if text == "" {
		return ButlerOCRInsight{}, fmt.Errorf("no text provided for analysis")
	}

	// SECURITY: Input validation - limit text size to prevent DoS
	const maxTextLength = 100000 // ~100KB, ~25K tokens max
	if len(text) > maxTextLength {
		text = text[:maxTextLength]
		log.Printf("⚠️ Butler: Text truncated to %d chars for analysis", maxTextLength)
	}

	// SECURITY: Validate docType to prevent injection
	validDocTypes := map[string]bool{
		"rfq": true, "RFQ": true, "po": true, "PO": true, "purchase_order": true, "PurchaseOrder": true,
		"invoice": true, "Invoice": true, "quote": true, "Quote": true,
		"email": true, "Email": true, "delivery_note": true, "DeliveryNote": true,
		"grn": true, "GRN": true, "unknown": true, "": true,
		"bank_statement": true, "BankStatement": true,
		"supplier_invoice": true, "SupplierInvoice": true,
		"quotation": true, "Quotation": true,
		"contract": true, "Contract": true,
		"Other": true,
	}
	if !validDocTypes[docType] {
		docType = "unknown"
		log.Printf("⚠️ Butler: Invalid document type sanitized to 'unknown'")
	}

	// Check API key availability
	apiKey := getMistralAPIKey()
	if apiKey == "" {
		return ButlerOCRInsight{
			Summary:    "Mistral API key not configured. Please add your API key in Settings > AI & Intelligence.",
			Confidence: 0.0,
		}, fmt.Errorf("MISTRAL_API_KEY not configured")
	}

	// Build context from metadata (handles any value type via %v)
	contextInfo := ""
	if metadata != nil {
		if from, ok := metadata["from"]; ok && from != nil && fmt.Sprintf("%v", from) != "" {
			contextInfo += fmt.Sprintf("From: %v\n", from)
		}
		if subject, ok := metadata["subject"]; ok && subject != nil && fmt.Sprintf("%v", subject) != "" {
			contextInfo += fmt.Sprintf("Subject: %v\n", subject)
		}
		if date, ok := metadata["date"]; ok && date != nil && fmt.Sprintf("%v", date) != "" {
			contextInfo += fmt.Sprintf("Date: %v\n", date)
		}
		// Also include customer_name and invoice_number if present (common OCR fields)
		if customer, ok := metadata["customer_name"]; ok && customer != nil && fmt.Sprintf("%v", customer) != "" {
			contextInfo += fmt.Sprintf("Customer: %v\n", customer)
		}
		if invNum, ok := metadata["invoice_number"]; ok && invNum != nil && fmt.Sprintf("%v", invNum) != "" {
			contextInfo += fmt.Sprintf("Invoice/Reference: %v\n", invNum)
		}
		if total, ok := metadata["total"]; ok && total != nil {
			contextInfo += fmt.Sprintf("Total Amount: %v\n", total)
		}
	}

	// Build analysis prompt
	systemPrompt := `You are a document analysis AI for PH Trading WLL, a process instrumentation company in Bahrain.

Your task is to extract structured information from business documents (emails, RFQs, purchase orders, invoices, bank statements, delivery notes, quotations).

COMPANY CONTEXT:
- Industry: Process Instrumentation
- Key Suppliers: Endress+Hauser (E+H), Servomex, Landis+Gyr, GIC
- Key Customers: BAPCO, Alba, EWA, GPIC, NOGA (industrial/government)
- Products: Flow meters, level transmitters, pressure sensors, analyzers, temperature sensors

EXTRACTION RULES:
1. Identify the document type (RFQ, Purchase Order, Invoice, Quote, BankStatement, DeliveryNote, SupplierInvoice, Inquiry, Other)
2. Extract customer/sender company name
3. Extract project name or reference number if mentioned
4. Extract line items with quantities, descriptions, and part numbers
5. Identify any deadlines or urgency indicators
6. Note any special requirements or conditions
7. For bank statements: Extract bank name, account number, statement period, opening/closing balance, and list transactions as line items

OUTPUT FORMAT (JSON):
For RFQs, Invoices, POs, Quotes:
{
  "document_type": "RFQ|PO|Invoice|Quote|DeliveryNote|SupplierInvoice|Inquiry|Other",
  "summary": "1-2 sentence summary of the document",
  "customer": "Company name",
  "project": "Project name or reference",
  "deadline": "Date or urgency description",
  "line_items": [
    {"description": "...", "quantity": 1, "unit": "pcs", "part_number": "..."}
  ],
  "metadata": {},
  "confidence": 0.85,
  "suggested_actions": ["Create RFQ", "Generate Costing", "Contact customer"]
}

For Bank Statements:
{
  "document_type": "BankStatement",
  "summary": "Bank statement summary with key figures",
  "customer": "",
  "project": "",
  "deadline": "",
  "line_items": [
    {"date": "2026-01-02", "description": "Payment received", "reference": "TXN123", "debit": 0, "credit": 500.000, "balance": 71058.251}
  ],
  "metadata": {
    "bank_name": "National Bank of Bahrain",
    "account_number": "0000000099570076",
    "iban": "BH16NBOB00000099570076",
    "opening_balance": "70558.251",
    "closing_balance": "87980.027",
    "period_start": "01/01/2026",
    "period_end": "31/01/2026",
    "currency": "BHD",
    "total_debits": "123456.789",
    "total_credits": "140878.565"
  },
  "confidence": 0.95,
  "suggested_actions": ["Import to Bank Reconciliation"]
}

IMPORTANT:
- Return ONLY valid JSON, no markdown or explanation
- Use 0.0-1.0 for confidence based on how clear the information is
- If information is not found, use empty string or empty array
- Be conservative with quantities - only extract if clearly stated
- For bank statements: DEBIT = money LEAVING the account (outgoing payments, transfers out, charges). CREDIT = money ENTERING the account (incoming payments, transfers in, deposits). Incoming transfers from customers (Fawri, INWD, deposit, TRF-IN, receipts) should be CREDITS. Outgoing payments (DGC, ONUS, Fawri-out, TRF-OUT, withdrawals, fees, charges) should be DEBITS. Use the running balance to validate: if the balance increases after a transaction it is a credit; if it decreases it is a debit. Consolidate related charges into their parent transaction. Bank fees (EFTS Charges, SWIFT FEES, Corr Bnk Chg), VAT on fees, and similar service charges that reference the same transaction should be MERGED into the parent transaction by adding the amounts together. Use the parent description. Do NOT list fee/VAT lines as separate transactions. Use date format YYYY-MM-DD (e.g. 2026-01-15).`

	// Sanitize user-supplied data before injection into prompt
	userPrompt := fmt.Sprintf("Analyze this %s document and extract structured information:\n\n%s%s",
		docType, sanitizeForPrompt(contextInfo), sanitizeForPrompt(text))

	// Call Mistral API
	response, err := callMistral(mistralModelLarge, systemPrompt, userPrompt)
	if err != nil {
		log.Printf("❌ Mistral analysis error: %v", err)
		return ButlerOCRInsight{
			Summary:    fmt.Sprintf("Analysis failed: %v", err),
			Confidence: 0.0,
		}, err
	}

	// Log raw response for debugging (truncated to 3000 chars)
	logLen := len(response)
	if logLen > 3000 {
		logLen = 3000
	}
	log.Printf("📋 Raw Mistral response (%d chars): %s", len(response), response[:logLen])

	// Parse JSON response
	insight := ButlerOCRInsight{
		ExtractedItems:    []map[string]interface{}{},
		SuggestedActions:  []ButlerAction{},
		ExtractedMetadata: make(map[string]interface{}),
	}

	// Try to parse as flexible JSON (captures ALL fields from Mistral, including bank transaction fields)
	var parsed struct {
		DocumentType     string                   `json:"document_type"`
		Summary          string                   `json:"summary"`
		Customer         string                   `json:"customer"`
		Project          string                   `json:"project"`
		Deadline         string                   `json:"deadline"`
		LineItems        []map[string]interface{} `json:"line_items"`
		Metadata         map[string]interface{}   `json:"metadata"`
		Confidence       float64                  `json:"confidence"`
		SuggestedActions []string                 `json:"suggested_actions"`
	}

	// Extract JSON from response - robust approach that handles markdown wrapping,
	// trailing text, and partial truncation. Find first '{' and last '}' to isolate JSON.
	cleanResponse := strings.TrimSpace(response)
	jsonStart := strings.Index(cleanResponse, "{")
	jsonEnd := strings.LastIndex(cleanResponse, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		cleanResponse = cleanResponse[jsonStart : jsonEnd+1]
	}
	log.Printf("📋 Extracted JSON (%d chars), starts with: %.100s", len(cleanResponse), cleanResponse)

	if err := json.Unmarshal([]byte(cleanResponse), &parsed); err != nil {
		log.Printf("⚠️ Failed to parse AI response as JSON: %v (first 200 chars: %s)", err, cleanResponse[:min(len(cleanResponse), 200)])
		// Return raw response as summary
		insight.Summary = response
		insight.Confidence = 0.5
		return insight, nil
	}

	// Map parsed data to insight
	insight.DocumentType = parsed.DocumentType
	insight.Summary = parsed.Summary
	insight.DetectedCustomer = parsed.Customer
	insight.DetectedProject = parsed.Project
	insight.RequiredDeadline = parsed.Deadline
	insight.Confidence = parsed.Confidence
	insight.ExtractedMetadata = parsed.Metadata

	// Pass line items through directly - flexible map captures ALL fields
	// (RFQ: description, quantity, unit, part_number)
	// (Bank: date, description, reference, debit, credit, balance)
	insight.ExtractedItems = parsed.LineItems

	// Convert suggested actions
	for _, action := range parsed.SuggestedActions {
		insight.SuggestedActions = append(insight.SuggestedActions, ButlerAction{
			Type:   "navigate",
			Target: strings.ToLower(strings.ReplaceAll(action, " ", "_")),
			Data:   action,
		})
	}

	log.Printf("🧠 Document analyzed: type=%s customer=%q items=%d confidence=%.2f metadata=%v",
		insight.DocumentType, insight.DetectedCustomer, len(insight.ExtractedItems), insight.Confidence, parsed.Metadata)

	return insight, nil
}

// TestMistralConnection tests the Mistral API connection with a simple query
func (a *App) TestMistralConnection() (bool, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return false, err
	}
	apiKey := getMistralAPIKey()
	if apiKey == "" {
		return false, fmt.Errorf("Mistral API key not configured. Add your key in Settings > AI & Intelligence")
	}

	// Make a simple test request
	response, err := callMistral(mistralModelSmall, "You are a test assistant.", "Say 'OK' if you can hear me.")
	if err != nil {
		return false, fmt.Errorf("Mistral API test failed: %v", err)
	}

	// Check if we got a reasonable response
	if len(response) > 0 {
		log.Printf("✅ Mistral API connection successful: %s", butlerTruncate(response, 50))
		return true, nil
	}

	return false, fmt.Errorf("empty response from Mistral API")
}
