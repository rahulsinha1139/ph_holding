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

	// Intent classification constants
	MaxKeywordMatchScore         = 4.0
	ComplexQueryDomainThreshold  = 3
	ComplexQueryLengthThreshold  = 200
	MistralAPITimeout            = 120 * time.Second
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

// extractEntityName extracts potential customer/supplier names from query
func extractEntityName(query string) string {
	words := strings.Fields(query)

	// Look for words after common prepositions/verbs
	triggers := []string{"about", "is", "for", "with", "from", "customer", "supplier", "client"}
	stopWords := map[string]bool{
		"is": true, "the": true, "doing": true, "has": true, "with": true,
		"last": true, "this": true, "next": true, "year": true, "month": true,
		"quarter": true, "week": true, "total": true, "business": true,
		"in": true, "on": true, "at": true, "to": true, "and": true,
		"we": true, "have": true, "had": true, "been": true, "was": true,
		"how": true, "what": true, "much": true, "many": true, "give": true,
		"me": true, "our": true, "their": true, "since": true, "during": true,
		"?": true, "all": true, "can": true, "you": true,
	}
	for i, word := range words {
		wordLower := strings.ToLower(word)
		for _, trigger := range triggers {
			if wordLower == trigger && i+1 < len(words) {
				// Collect subsequent words that look like entity names
				name := []string{}
				for j := i + 1; j < len(words) && j < i+5; j++ {
					w := words[j]
					wLower := strings.ToLower(w)
					if stopWords[wLower] {
						break
					}
					name = append(name, w)
				}
				if len(name) > 0 {
					result := strings.Join(name, " ")
					// Clean trailing punctuation
					result = strings.TrimRight(result, "?,.")
					return result
				}
			}
		}
	}

	return ""
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

	// 4. Build context based on intent
	context := a.buildIntentContext(intent)

	// 5. Calculate regime
	regime := a.calculateSystemRegime()
	context["system_regime"] = regime

	// 6. Always use large model for best quality
	model := mistralModelLarge

	// 7. Build system prompt with context and regime
	systemPrompt := buildMistralSystemPrompt(context, regime)

	// 8. Call Mistral API
	aiResponse, err := callMistral(model, systemPrompt, message)
	if err != nil {
		log.Printf("❌ Mistral API error: %v", err)
		return ButlerResponse{
			Message:    fmt.Sprintf("Intelligence pipeline error: %v. Please try again.", err),
			Actions:    []ButlerAction{},
			Confidence: 0.0,
			Context:    context,
		}, nil // Return nil error so frontend shows the message
	}

	// 9. Parse response (extract actions, clean message)
	butlerResponse := parseMistralResponse(aiResponse, context, intent)

	log.Printf("🤖 Butler responded via %s: %s (confidence: %.2f, actions: %d)",
		model, butlerTruncate(butlerResponse.Message, 60), butlerResponse.Confidence, len(butlerResponse.Actions))

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

	case "risk":
		context["risk_data"] = a.getRiskContext()
		context["ar_summary"] = a.getARSummary()

	default:
		// General query - provide comprehensive overview
		context["customer_data"] = a.getCustomerContext(intent.EntityName)
		context["ar_summary"] = a.getARSummary()
		context["operations_data"] = a.getOperationsContext()
		if hasFinanceAccess {
			context["financial_data"] = a.getFinancialContext()
		}
	}

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
// SYSTEM PROMPT BUILDER
// ============================================================================

// buildMistralSystemPrompt creates the PRD-style context-aware system prompt
func buildMistralSystemPrompt(context map[string]interface{}, regime map[string]interface{}) string {
	// Serialize context to JSON for injection
	contextJSON, _ := json.MarshalIndent(context, "", "  ")

	// Build regime guidance
	regimeGuidance := buildRegimeGuidance(regime)

	prompt := fmt.Sprintf(`You are the PH Trading Intelligent Assistant, a private AI advisor for PH Trading WLL employees.

COMPANY: PH Trading WLL - Bahrain-based process instrumentation company
CURRENCY: BHD (Bahraini Dinar, 3 decimal places)
MAJOR CUSTOMERS: BAPCO, Alba, EWA, GPIC, NOGA (government/industrial)
MAJOR SUPPLIERS: Endress+Hauser, Servomex, Landis+Gyr

YOUR ROLE:
- Answer questions about customers, suppliers, invoices, payments, and business operations
- Provide actionable insights and recommendations
- Surface risks and opportunities proactively
- Use ONLY the provided context data (do not hallucinate figures)

CONTEXT DATA:
%s

%s

INSTRUCTIONS:
1. Always cite specific data from context when available (e.g., "Outstanding: 5,230 BHD")
2. If data is missing from context, say "I don't have that data available"
3. Provide actionable recommendations, not just facts
4. Be concise and professional (under 150 words for simple queries)
5. Format currency as BHD with 3 decimal places where appropriate
6. If you recommend an action, include it in [ACTIONS]...[/ACTIONS] block as JSON array

ACTION FORMAT (include when recommending next steps):
[ACTIONS]
[{"type":"NAVIGATE","target":"screen_name","label":"Description","data":{}}]
[/ACTIONS]

Available action types: NAVIGATE, CREATE, FETCH, ANALYZE, ALERT
Available targets: customers, suppliers, invoices, payments, orders, rfqs, offers, deliveries, dashboard

RESPONSE STYLE:
- Concise and business-oriented
- Proactive (suggest next steps)
- Data-driven (cite numbers from context)
- Honest about limitations`, string(contextJSON), regimeGuidance)

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

	userPrompt := fmt.Sprintf("Analyze this %s document and extract structured information:\n\n%s%s",
		docType, contextInfo, text)

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
