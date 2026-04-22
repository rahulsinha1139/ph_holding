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
	"math"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strconv"
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
	Metadata   ButlerResponseMetadata `json:"metadata"`
}

type ButlerResponseMetadata struct {
	UsedBackend       string                 `json:"used_backend"`
	RequestedModel    string                 `json:"requested_model"`
	UsedModel         string                 `json:"used_model"`
	FallbackReason    string                 `json:"fallback_reason"`
	FinanceDataAccess bool                   `json:"finance_data_access"`
	ContextMode       string                 `json:"context_mode"`
	DataCoverage      []string               `json:"data_coverage"`
	EntityResolution  map[string]interface{} `json:"entity_resolution"`
	GeneratedAt       string                 `json:"generated_at"`
	Error             string                 `json:"error"`
}

// ButlerAction represents a suggested action from Butler
type ButlerAction struct {
	Type   string      `json:"type"`   // "navigate", "create", "update", "fetch", "analyze", "alert"
	Target string      `json:"target"` // screen or entity
	Label  string      `json:"label"`
	Data   interface{} `json:"data"`
}

// Intent represents classified user intent
type Intent struct {
	RawQuery      string
	Domain        string   // customer, supplier, financial, operations, risk, market, general
	EntityName    string   // extracted customer/supplier name
	PersonName    string   // extracted contact/account-manager reference
	ReferenceKind string   // customer, contact_person, account_manager
	Confidence    float64  // 0.0 - 1.0
	Keywords      []string // matched keywords
	NeedsScraper  bool     // true if external data needed
	IsComplex     bool     // true if requires mistral-large
}

type ButlerResolvedEntity struct {
	EntityType        string
	EntityID          string
	DisplayName       string
	Confidence        float64
	MatchReason       string
	RelatedCustomerID string
	RelatedCustomer   string
	Ambiguous         bool
	Alternatives      []map[string]interface{}
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
	aimlModelID     = "x-ai/grok-4-fast-reasoning" // xAI Grok 4 via AIML API (prefixed model IDs)
	aimlMaxTokens   = 16384
	aimlTemperature = 0.4

	// Intent classification constants
	MaxKeywordMatchScore        = 4.0
	ComplexQueryDomainThreshold = 3
	ComplexQueryLengthThreshold = 200
	MistralAPITimeout           = 120 * time.Second
	AimlAPITimeout              = 120 * time.Second
	butlerMaxContextChars       = 60000
	butlerMaxReportContextChars = 80000
)

var fallbackAIMLModels = []string{
	"x-ai/grok-4-1-fast-reasoning",
	"x-ai/grok-3-beta",
	"gpt-4o",
	"gpt-4o-mini",
}

// ============================================================================
// INTENT CLASSIFICATION
// ============================================================================

var (
	customerKeywords   = []string{"customer", "client", "buyer", "account", "bapco", "alba", "ewa", "gpic", "noga", "aluminium bahrain", "bahrain petroleum", "electricity water", "gulf petrochemical", "tatweer", "banagas", "business with", "total business", "how much", "who is our", "top customer", "biggest customer", "best customer", "worst customer", "most profitable", "profitab", "dormant", "inactive", "haven't ordered", "not ordered", "lapsed", "churn", "retention", "new customer", "acquisition", "which customer"}
	supplierKeywords   = []string{"supplier", "vendor", "source", "endress", "hauser", "servomex", "landis", "gyr", "e+h", "bought from", "purchased from", "procurement", "supply chain", "top supplier", "lead time", "delivery time", "reliable", "on time", "supplier performance", "qc rate", "quality"}
	financialKeywords  = []string{"cash", "payment", "invoice", "receivable", "payable", "balance", "revenue", "profit", "margin", "ar", "aging", "dso", "outstanding", "collection", "money", "owe", "owes", "paid", "unpaid", "earned", "income", "expense", "cashflow", "cash flow", "how much did we", "total sales", "turnover", "gross", "net", "bhd", "fiscal", "financial year", "days to pay", "payment speed", "slow payer", "fast payer", "days sales outstanding", "margin by", "profit by"}
	operationsKeywords = []string{"quote", "rfq", "order", "pipeline", "delivery", "purchase", "costing", "offer", "grn", "shipment", "po", "purchase order", "win rate", "conversion", "open orders", "pending", "active orders", "quotation", "expir", "validity", "expire", "lapsing", "about to expire", "win loss", "lost reason", "competition", "competitor", "abb", "won", "lost deals", "service", "maintenance", "calibration", "install", "installation", "commission", "commissioning", "repair", "visit"}
	workKeywords       = []string{"task", "tasks", "assigned", "assignee", "employee", "employees", "staff", "notification", "notifications", "workload", "work queue", "my work", "team board", "commented", "comment", "blocked task", "blocked reason", "collaborative", "project task"}
	riskKeywords       = []string{"overdue", "late", "risk", "aging", "collection", "blocked", "dispute", "critical", "warning", "defaulter", "slow payer", "bad debt", "credit block", "action item", "what should", "what do i", "today", "urgent", "priorit", "focus", "stuck", "follow up", "pending approval", "awaiting", "service due", "warranty", "calibration due", "cashflow", "cash flow"}
	marketKeywords     = []string{"price", "market", "competitor", "trend", "industry", "steel", "instrumentation"}

	// Patterns that indicate complex queries needing mistral-large
	complexPatterns = []string{"compare", "analyze", "recommend", "strategy", "forecast", "predict", "trend", "correlat", "multiple", "relationship", "which", "most", "least", "highest", "lowest", "best", "worst", "over time", "by month", "by quarter", "by year", "profitab", "dormant", "churn", "performance"}

	// Action regex for parsing [ACTIONS]...[/ACTIONS] blocks
	actionBlockRegex = regexp.MustCompile(`(?s)\[ACTIONS\](.*?)\[/ACTIONS\]`)
)

// classifyIntent determines query domain and extracts entities
func classifyIntent(query string) Intent {
	queryLower := strings.ToLower(query)

	intent := Intent{
		RawQuery:   query,
		Domain:     "general",
		Confidence: 0.5,
		Keywords:   []string{},
	}

	customerScore := countKeywordMatches(queryLower, customerKeywords)
	supplierScore := countKeywordMatches(queryLower, supplierKeywords)
	workScore := countKeywordMatches(queryLower, workKeywords)
	financialScore := countKeywordMatches(queryLower, financialKeywords)
	operationsScore := countKeywordMatches(queryLower, operationsKeywords)
	riskScore := countKeywordMatches(queryLower, riskKeywords)
	marketScore := countKeywordMatches(queryLower, marketKeywords)

	// Count keyword matches per domain using a stable priority order so ties are deterministic.
	domainScores := []struct {
		domain string
		score  int
	}{
		{"customer", customerScore},
		{"supplier", supplierScore},
		{"work", workScore},
		{"financial", financialScore},
		{"operations", operationsScore},
		{"risk", riskScore},
		{"market", marketScore},
	}

	// Find highest scoring domain with deterministic tie-breaking by list order above.
	maxScore := 0
	for _, candidate := range domainScores {
		if candidate.score > maxScore {
			maxScore = candidate.score
			intent.Domain = candidate.domain
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

	// Extract entity names even for mixed-domain asks so downstream grounding can still resolve them.
	intent.EntityName = extractEntityName(query)
	if intent.EntityName != "" {
		switch {
		case supplierScore > customerScore && supplierScore > 0:
			intent.ReferenceKind = "supplier"
		case customerScore > 0:
			intent.ReferenceKind = "customer"
		case intent.Domain == "supplier":
			intent.ReferenceKind = "supplier"
		case intent.Domain == "customer":
			intent.ReferenceKind = "customer"
		}
	}

	intent.PersonName = extractPersonReference(query)
	if intent.PersonName != "" {
		if looksLikeAccountManagerReference(queryLower) {
			intent.ReferenceKind = "account_manager"
		} else if intent.Domain == "work" {
			intent.ReferenceKind = "employee"
		} else if intent.ReferenceKind == "" {
			intent.ReferenceKind = "contact_person"
		}
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
	for _, candidate := range domainScores {
		if candidate.score > 0 {
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
		"payment": true, "payments": true, "history": true, "invoice": true, "invoices": true,
		"order": true, "orders": true, "task": true, "tasks": true, "notification": true,
		"notifications": true, "quotation": true, "quote": true, "calibration": true,
		"service": true, "visit": true, "package": true,
	}
	return stops[w]
}

func extractPersonReference(query string) string {
	triggerPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:contact|attention|attn|spoc|person)\s+(?:for|at|of)?\s*([A-Z][A-Za-z.\-]+(?:\s+[A-Z][A-Za-z.\-]+){0,3})`),
		regexp.MustCompile(`(?i)(?:account manager|salesperson|sales person|owner|handled by|handling)\s+(?:for|is|:)?\s*([A-Z][A-Za-z.\-]+(?:\s+[A-Z][A-Za-z.\-]+){0,3})`),
		regexp.MustCompile(`(?i)(?:tasks?\s+assigned\s+to|assigned\s+to|workload\s+for|notifications?\s+for|tasks?\s+for|employee|staff member)\s+([A-Z][A-Za-z.\-]+(?:\s+[A-Z][A-Za-z.\-]+){0,2})`),
		regexp.MustCompile(`(?i)(?:notifications?\s+does|tasks?\s+does)\s+([A-Z][A-Za-z.\-]+(?:\s+[A-Z][A-Za-z.\-]+){0,2})\s+have`),
	}
	for _, pattern := range triggerPatterns {
		if m := pattern.FindStringSubmatch(query); len(m) > 1 {
			return cleanExtractedPersonReference(m[1])
		}
	}

	fullNamePattern := regexp.MustCompile(`\b([A-Z][A-Za-z.\-]+(?:\s+[A-Z][A-Za-z.\-]+)+)\b`)
	if m := fullNamePattern.FindStringSubmatch(query); len(m) > 1 {
		return cleanExtractedPersonReference(m[1])
	}

	return ""
}

func cleanExtractedPersonReference(raw string) string {
	candidate := strings.TrimSpace(raw)
	if candidate == "" {
		return ""
	}

	trailingNoise := []string{
		" today", " right now", " now", " currently", " this week", " this month", " this quarter",
	}
	lower := strings.ToLower(candidate)
	for _, suffix := range trailingNoise {
		if strings.HasSuffix(lower, suffix) {
			candidate = strings.TrimSpace(candidate[:len(candidate)-len(suffix)])
			lower = strings.ToLower(candidate)
		}
	}

	return strings.TrimSpace(candidate)
}

func looksLikeAccountManagerReference(queryLower string) bool {
	accountManagerHints := []string{
		"account manager", "salesperson", "sales person", "owner",
		"who is handling", "who's handling", "handled by", "managed by",
	}
	for _, hint := range accountManagerHints {
		if strings.Contains(queryLower, hint) {
			return true
		}
	}
	return false
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
	hasFinanceAccess := a.requirePermission("finance:view") == nil

	// 2. RBAC: Check if user has permission to access financial data
	if requiresFinancePrivilege(intent, message) {
		userRole := a.GetCurrentUserRole()
		licenseRole := a.GetLicenseRole()
		if !hasFinanceAccess {
			log.Printf("🔒 Butler financial query blocked: user role=%s license role=%s lacks finance:view", userRole, licenseRole)
			return ButlerResponse{
				Message:    "Financial data access requires manager or admin privileges.\n\nYou can still ask me about:\n• Customer information\n• Supplier details\n• Order status\n• Operations and pipeline\n\nFor financial reports and metrics, please contact your manager.",
				Actions:    []ButlerAction{},
				Confidence: 0.95,
				Context: map[string]interface{}{
					"blocked_domain": intent.Domain,
					"user_role":      userRole,
					"license_role":   licenseRole,
				},
				Metadata: buildButlerMetadata(intent, map[string]interface{}{
					"blocked_domain": intent.Domain,
					"user_role":      userRole,
					"license_role":   licenseRole,
				}, false, "blocked", "", "", "", nil),
			}, nil
		}
	}

	if routeReply, routeActions, handled := a.tryButlerIntentClarificationFastPath(intent, message, hasFinanceAccess); handled {
		return ButlerResponse{
			Message:    routeReply,
			Actions:    routeActions,
			Confidence: 0.99,
			Context:    map[string]interface{}{"fast_path": "butler_intent_clarification"},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"fast_path": "butler_intent_clarification"}, hasFinanceAccess, "grounded_sql", "", "", "", nil),
		}, nil
	}
	if groundedReply, groundedActions, handled := a.tryGroundedARProjectionFastPath(intent, message, hasFinanceAccess); handled {
		return ButlerResponse{
			Message:    groundedReply,
			Actions:    groundedActions,
			Confidence: 0.99,
			Context:    map[string]interface{}{"fast_path": "grounded_ar_projection"},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"fast_path": "grounded_ar_projection"}, hasFinanceAccess, "grounded_sql", "", "", "", nil),
		}, nil
	}
	if groundedReply, handled := a.tryGroundedCapabilitiesFastPath(intent, message, hasFinanceAccess); handled {
		return ButlerResponse{
			Message:    groundedReply,
			Actions:    []ButlerAction{},
			Confidence: 0.99,
			Context:    map[string]interface{}{"fast_path": "grounded_capabilities"},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"fast_path": "grounded_capabilities"}, hasFinanceAccess, "grounded_sql", "", "", "", nil),
		}, nil
	}
	if groundedReply, groundedActions, handled := a.tryGroundedManagerFinancialBriefFastPath(intent, message, hasFinanceAccess); handled {
		return ButlerResponse{
			Message:    groundedReply,
			Actions:    groundedActions,
			Confidence: 0.99,
			Context:    map[string]interface{}{"fast_path": "grounded_manager_financial_brief"},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"fast_path": "grounded_manager_financial_brief"}, hasFinanceAccess, "grounded_sql", "", "", "", nil),
		}, nil
	}
	if groundedReply, handled := a.tryGroundedTaskCreationFastPath(intent, message); handled {
		return ButlerResponse{
			Message:    groundedReply,
			Actions:    []ButlerAction{},
			Confidence: 0.99,
			Context:    map[string]interface{}{"fast_path": "grounded_task_create"},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"fast_path": "grounded_task_create"}, hasFinanceAccess, "grounded_sql", "", "", "", nil),
		}, nil
	}
	if groundedReply, handled := a.tryGroundedWorkFastPath(intent, message); handled {
		return ButlerResponse{
			Message:    groundedReply,
			Actions:    []ButlerAction{},
			Confidence: 0.98,
			Context:    map[string]interface{}{"fast_path": "grounded_work"},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"fast_path": "grounded_work"}, hasFinanceAccess, "grounded_sql", "", "", "", nil),
		}, nil
	}
	if groundedReply, handled := a.tryGroundedSupplierFastPath(intent, message); handled {
		return ButlerResponse{
			Message:    groundedReply,
			Actions:    []ButlerAction{},
			Confidence: 0.98,
			Context:    map[string]interface{}{"fast_path": "grounded_supplier"},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"fast_path": "grounded_supplier"}, hasFinanceAccess, "grounded_sql", "", "", "", nil),
		}, nil
	}
	if groundedReply, handled := a.tryGroundedCustomerFastPath(intent, message); handled {
		return ButlerResponse{
			Message:    groundedReply,
			Actions:    []ButlerAction{},
			Confidence: 0.98,
			Context:    map[string]interface{}{"fast_path": "grounded_customer"},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"fast_path": "grounded_customer"}, hasFinanceAccess, "grounded_sql", "", "", "", nil),
		}, nil
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
	requestedModel := getAIMLModelID()
	usedModel := ""
	fallbackReason := ""
	if aimlKey := getAIMLAPIKey(); aimlKey != "" {
		aiResponse, usedModel, aiErr = callAIMLWithFallback(aimlKey, systemPrompt, message)
		usedBackend = "AIML/Grok"
		if aiErr != nil {
			fallbackReason = aiErr.Error()
			log.Printf("⚠️ AIML API error, falling back to Mistral: %v", aiErr)
			aiResponse, aiErr = callMistral(mistralModelLarge, systemPrompt, message)
			usedBackend = "Mistral (fallback)"
		}
	} else {
		aiResponse, aiErr = callMistral(mistralModelLarge, systemPrompt, message)
	}

	if aiErr != nil {
		log.Printf("❌ AI backend error: %v", aiErr)
		fallbackResponse := a.buildGroundedModelFallbackResponse(intent, message, context, hasFinanceAccess, aiErr)
		fallbackResponse.Metadata.UsedBackend = firstNonEmpty(fallbackResponse.Metadata.UsedBackend, "grounded_sql_fallback")
		fallbackResponse.Metadata.RequestedModel = requestedModel
		fallbackResponse.Metadata.UsedModel = usedModel
		fallbackResponse.Metadata.FallbackReason = firstNonEmpty(fallbackResponse.Metadata.FallbackReason, fallbackReason, aiErr.Error())
		return fallbackResponse, nil
	}

	// 8. Parse response (extract actions, clean message)
	butlerResponse := parseMistralResponse(aiResponse, context, intent)
	butlerResponse.Metadata = buildButlerMetadata(
		intent,
		context,
		hasFinanceAccess,
		usedBackend,
		requestedModel,
		usedModel,
		fallbackReason,
		nil,
	)

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
		"prepare a document", "create a document", "financial document", "manager document",
		"manager brief", "management brief", "manager briefing", "management briefing",
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
	financeAccess := a.currentSessionHasPermission("finance:view")
	reportMetadata := buildButlerMetadata(intent, map[string]interface{}{
		"request":     message,
		"report_type": reportType,
	}, financeAccess, "local_report", "", "", "", nil)

	filePath, err := a.GenerateButlerReport(reportType, intent.EntityName)
	if err != nil {
		// Check if this is a permission error
		errorMsg := err.Error()
		isPermissionError := strings.Contains(errorMsg, "permission denied") || strings.Contains(errorMsg, "finance:view")

		if isPermissionError {
			return ButlerResponse{
				Message:    "I cannot generate this report due to insufficient permissions.\n\nFinancial, risk, and executive reports require manager or admin privileges.\n\nYou can still request:\n• Customer reports\n• Supplier reports\n• Operations reports\n\nPlease contact your manager for financial report access.",
				Actions:    []ButlerAction{},
				Confidence: 0.95,
				Context:    map[string]interface{}{"report_type": reportType, "blocked": "permission_denied"},
				Metadata:   reportMetadata,
			}, nil
		}

		return ButlerResponse{
			Message:    fmt.Sprintf("I attempted to generate the report but encountered an error: %v", err),
			Actions:    []ButlerAction{},
			Confidence: 0.3,
			Context:    map[string]interface{}{"report_type": reportType, "error": err.Error()},
			Metadata:   buildButlerMetadata(intent, map[string]interface{}{"report_type": reportType, "error": err.Error()}, financeAccess, "local_report", "", "", err.Error(), err),
		}, nil
	}

	responseMsg := fmt.Sprintf("Your %s report has been generated successfully.\n\nFile saved to:\n%s\n\nThe report contains live business data from your database combined with AI-powered analysis covering key metrics, risk factors, and strategic recommendations.",
		getReportTitle(reportType), filePath)

	return ButlerResponse{
		Message:    responseMsg,
		Actions:    []ButlerAction{{Type: "fetch", Target: "report", Data: filePath}},
		Confidence: 0.95,
		Context:    map[string]interface{}{"report_type": reportType, "file_path": filePath},
		Metadata:   reportMetadata,
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
	hasFinanceAccess := a.requirePermission("finance:view") == nil

	// Always include dashboard summary (redacted for non-finance users)
	summary := a.getBusinessSummary()
	if !hasFinanceAccess {
		delete(summary, "total_revenue_bhd")
		delete(summary, "total_outstanding_bhd")
	}
	context["business_summary"] = summary
	context["system_time"] = a.getSystemTimeContext()
	if periodSummary := a.getBusinessPeriodSummary(intent.RawQuery); len(periodSummary) > 0 {
		context["business_period_summary"] = periodSummary
	}
	if yearSummary := a.getBusinessYearSummary(intent.RawQuery); len(yearSummary) > 0 {
		context["business_year_summary"] = yearSummary
	}
	a.injectGroundedReferenceContext(context, intent, hasFinanceAccess)
	context["butler_memory"] = a.getButlerMemoryContext(intent)
	context["proactive_briefing"] = a.getProactiveBriefingContext(hasFinanceAccess)
	context["work_data"] = a.getWorkContext(intent)

	// Domain-specific context injection
	switch intent.Domain {
	case "customer":
		customerCtx, ok := context["customer_data"].(map[string]interface{})
		if !ok || len(customerCtx) == 0 {
			customerCtx = a.getCustomerContext(intent.EntityName)
		}
		if !hasFinanceAccess {
			// Redact financial data from customer context for non-finance users
			a.redactFinancialFromContext(customerCtx)
		}
		context["customer_data"] = customerCtx
		if quarterSummary := a.getCustomerPeriodSummary(intent.EntityName, intent.RawQuery); len(quarterSummary) > 0 {
			context["customer_period_summary"] = quarterSummary
		} else if _, ok := parseCustomerPeriodLabel(intent.RawQuery); ok {
			context["customer_period_summary"] = a.getMissingCustomerPeriodSummary(intent.EntityName, intent.RawQuery)
		}
		context["installed_base_summary"] = a.getInstalledBaseSummary(intent.EntityName)
		if hasFinanceAccess {
			context["ar_summary"] = a.getARSummary()
		}

	case "supplier":
		context["supplier_data"] = a.getSupplierContext(intent.EntityName)
		context["po_summary"] = a.getPurchaseOrderSummary()

	case "financial":
		context["financial_data"] = a.getFinancialContext()
		context["ar_summary"] = a.getARSummary()
		context["cashflow_projection"] = a.getCashflowProjectionContext()

	case "operations":
		context["operations_data"] = a.getOperationsContext()
		context["po_summary"] = a.getPurchaseOrderSummary()
		context["risk_data"] = a.getRiskContext() // Include risk for operations awareness
		context["service_due_installations"] = a.getServiceDueInstallationsContext()
		context["quotation_precheck"] = a.getQuotationPrecheckContext(intent)

	case "risk":
		context["risk_data"] = a.getRiskContext()
		context["ar_summary"] = a.getARSummary()
		context["operations_data"] = a.getOperationsContext() // Include ops for delivery risk context
		context["service_due_installations"] = a.getServiceDueInstallationsContext()
		context["closeable_opportunities"] = a.getCloseableOpportunitiesContext()
		if hasFinanceAccess {
			context["cashflow_projection"] = a.getCashflowProjectionContext()
		}

	default:
		// General query - provide comprehensive overview
		context["customer_data"] = a.getCustomerContext(intent.EntityName)
		if quarterSummary := a.getCustomerPeriodSummary(intent.EntityName, intent.RawQuery); len(quarterSummary) > 0 {
			context["customer_period_summary"] = quarterSummary
		} else if _, ok := parseCustomerPeriodLabel(intent.RawQuery); ok {
			context["customer_period_summary"] = a.getMissingCustomerPeriodSummary(intent.EntityName, intent.RawQuery)
		}
		context["installed_base_summary"] = a.getInstalledBaseSummary(intent.EntityName)
		context["ar_summary"] = a.getARSummary()
		context["operations_data"] = a.getOperationsContext()
		context["risk_data"] = a.getRiskContext()
		context["service_due_installations"] = a.getServiceDueInstallationsContext()
		context["closeable_opportunities"] = a.getCloseableOpportunitiesContext()
		context["quotation_precheck"] = a.getQuotationPrecheckContext(intent)
		if hasFinanceAccess {
			context["financial_data"] = a.getFinancialContext()
			context["cashflow_projection"] = a.getCashflowProjectionContext()
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

	hasFinanceAccess := a.requirePermission("finance:view") == nil

	// Business summary (always included)
	summary := a.getBusinessSummary()
	if !hasFinanceAccess {
		delete(summary, "total_revenue_bhd")
		delete(summary, "total_outstanding_bhd")
	}
	context["business_summary"] = summary
	context["system_time"] = a.getSystemTimeContext()
	if periodSummary := a.getBusinessPeriodSummary(intent.RawQuery); len(periodSummary) > 0 {
		context["business_period_summary"] = periodSummary
	}
	if yearSummary := a.getBusinessYearSummary(intent.RawQuery); len(yearSummary) > 0 {
		context["business_year_summary"] = yearSummary
	}
	a.injectGroundedReferenceContext(context, intent, hasFinanceAccess)
	context["butler_memory"] = a.getButlerMemoryContext(intent)
	context["proactive_briefing"] = a.getProactiveBriefingContext(hasFinanceAccess)

	// Customer context — includes entity-specific data if name was extracted from query
	customerCtx, ok := context["customer_data"].(map[string]interface{})
	if !ok || len(customerCtx) == 0 {
		customerCtx = a.getCustomerContext(intent.EntityName)
	}
	if !hasFinanceAccess {
		a.redactFinancialFromContext(customerCtx)
	}
	context["customer_data"] = customerCtx

	// Supplier context — entity-specific if supplier name extracted, always top suppliers
	context["supplier_data"] = a.getSupplierContext(intent.EntityName)
	if quarterSummary := a.getCustomerPeriodSummary(intent.EntityName, intent.RawQuery); len(quarterSummary) > 0 {
		context["customer_period_summary"] = quarterSummary
	} else if _, ok := parseCustomerPeriodLabel(intent.RawQuery); ok {
		context["customer_period_summary"] = a.getMissingCustomerPeriodSummary(intent.EntityName, intent.RawQuery)
	}

	// Financial context (only if user has finance:view permission)
	if hasFinanceAccess {
		context["financial_data"] = a.getFinancialContext()
		context["ar_summary"] = a.getARSummary()
	}
	context["action_items"] = a.getActionItemsContext()
	context["competition_intelligence"] = a.getCompetitionContext()

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

	// DSO — Days Sales Outstanding per customer (who pays slow/fast)
	if hasFinanceAccess {
		context["dso_analysis"] = a.getDSOContext()
		context["cashflow_projection"] = a.getCashflowProjectionContext()
	}

	// Offer expiry — active quotes approaching their ValidityDate
	context["offer_expiry"] = a.getOfferExpiryContext()

	// Customer profitability — revenue and margin rankings (finance-gated)
	if hasFinanceAccess {
		context["customer_profitability"] = a.getCustomerProfitabilityContext()
	}

	// Supplier delivery performance — lead times and QC pass rates
	context["supplier_performance"] = a.getSupplierPerformanceContext()

	// Customer activity — dormant, new, and lapsed customers
	context["customer_activity"] = a.getCustomerActivityContext()
	context["service_due_installations"] = a.getServiceDueInstallationsContext()
	context["closeable_opportunities"] = a.getCloseableOpportunitiesContext()
	context["quotation_precheck"] = a.getQuotationPrecheckContext(intent)
	context["work_data"] = a.getWorkContext(intent)
	context["database_access"] = a.getDatabaseAccessContext(intent, hasFinanceAccess)
	context["database_catalog"] = a.getDatabaseInventoryContext(hasFinanceAccess)

	// Action items — prioritized list of things needing attention today
	context["action_items"] = a.getActionItemsContext()

	// Competition — win/loss analysis, ABB flag, lost reasons, win rate by product
	context["competition_analysis"] = a.getCompetitionContext()

	return context
}

func buildContextSearchTerms(intent Intent) []string {
	stopWords := map[string]bool{
		"a": true, "about": true, "all": true, "an": true, "and": true, "any": true,
		"are": true, "assigned": true, "at": true, "be": true, "by": true, "can": true,
		"current": true, "currently": true, "customer": true, "data": true, "did": true,
		"do": true, "does": true, "for": true, "from": true, "get": true, "give": true,
		"has": true, "have": true, "how": true, "i": true, "in": true, "into": true,
		"is": true, "it": true, "its": true, "line": true, "me": true, "more": true,
		"my": true, "need": true, "notes": true, "of": true, "on": true, "or": true,
		"our": true, "please": true, "show": true, "statements": true, "tasks": true,
		"tell": true, "that": true, "the": true, "their": true, "them": true, "there": true,
		"these": true, "this": true, "to": true, "us": true, "view": true, "what": true,
		"when": true, "where": true, "which": true, "who": true, "with": true, "you": true,
	}

	raw := strings.NewReplacer("?", " ", ",", " ", ".", " ", ":", " ", ";", " ", "(", " ", ")", " ", "/", " ", "\\", " ", "\n", " ").Replace(strings.ToLower(intent.RawQuery))
	candidates := make([]string, 0, 16)
	candidates = append(candidates, strings.ToLower(strings.TrimSpace(intent.EntityName)))
	candidates = append(candidates, strings.ToLower(strings.TrimSpace(intent.PersonName)))
	for _, token := range strings.Fields(raw) {
		token = strings.TrimSpace(token)
		if len(token) < 3 || stopWords[token] {
			continue
		}
		candidates = append(candidates, token)
	}

	seen := make(map[string]bool)
	terms := make([]string, 0, 10)
	for _, candidate := range candidates {
		if candidate == "" || seen[candidate] {
			continue
		}
		seen[candidate] = true
		terms = append(terms, candidate)
		if len(terms) >= 10 {
			break
		}
	}
	return terms
}

func selectContextEntries(entries []map[string]interface{}, terms []string, limit int) []map[string]interface{} {
	if len(entries) == 0 {
		return []map[string]interface{}{}
	}

	selected := entries
	if len(terms) > 0 {
		matched := make([]map[string]interface{}, 0, len(entries))
		for _, entry := range entries {
			raw, _ := json.Marshal(entry)
			haystack := strings.ToLower(string(raw))
			for _, term := range terms {
				if term != "" && strings.Contains(haystack, term) {
					matched = append(matched, entry)
					break
				}
			}
		}
		if len(matched) > 0 {
			selected = matched
		}
	}

	if limit > 0 && len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func (a *App) getWorkContext(intent Intent) map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	terms := buildContextSearchTerms(intent)

	var employees []Employee
	_ = a.db.Where("is_active = ?", true).Order("full_name ASC").Limit(25).Find(&employees).Error
	employeeEntries := make([]map[string]interface{}, 0, len(employees))
	employeeNames := make(map[string]string, len(employees))
	for _, employee := range employees {
		employeeNames[employee.ID] = employee.FullName
		employeeEntries = append(employeeEntries, map[string]interface{}{
			"employee_id": employee.ID,
			"name":        employee.FullName,
			"department":  employee.Department,
			"job_title":   employee.JobTitle,
			"status":      employee.EmploymentStatus,
			"notes":       employee.Notes,
		})
	}
	result["employees"] = selectContextEntries(employeeEntries, terms, 8)

	var tasks []TaskItem
	_ = a.db.Order("updated_at DESC").Limit(30).Find(&tasks).Error
	taskEntries := make([]map[string]interface{}, 0, len(tasks))
	taskIDs := make([]string, 0, len(tasks))
	for _, task := range tasks {
		taskIDs = append(taskIDs, task.ID)
		taskEntries = append(taskEntries, map[string]interface{}{
			"task_id":        task.ID,
			"title":          task.Title,
			"description":    task.Description,
			"status":         task.Status,
			"priority":       task.Priority,
			"task_type":      task.TaskType,
			"blocked_reason": task.BlockedReason,
			"due_date":       formatOptionalDate(task.DueDate),
			"creator_name":   employeeNames[task.CreatorEmployeeID],
			"assignee_name":  employeeNames[firstNonEmptyStringPointer(task.AssigneeEmployeeID)],
		})
	}
	selectedTasks := selectContextEntries(taskEntries, terms, 10)
	result["tasks"] = selectedTasks

	taskIDTitle := make(map[string]string, len(tasks))
	for _, task := range tasks {
		taskIDTitle[task.ID] = task.Title
	}

	var comments []TaskComment
	if len(taskIDs) > 0 {
		_ = a.db.Where("task_id IN ?", taskIDs).Order("created_at DESC").Limit(25).Find(&comments).Error
	}
	commentEntries := make([]map[string]interface{}, 0, len(comments))
	for _, comment := range comments {
		commentEntries = append(commentEntries, map[string]interface{}{
			"task_id":    comment.TaskID,
			"task_title": taskIDTitle[comment.TaskID],
			"employee":   employeeNames[comment.EmployeeID],
			"body":       comment.Body,
			"created_at": comment.CreatedAt.Format("2006-01-02"),
		})
	}
	result["task_comments"] = selectContextEntries(commentEntries, terms, 8)

	var notifications []Notification
	_ = a.db.Where("source_type = ?", "task").Order("created_at DESC").Limit(20).Find(&notifications).Error
	notificationEntries := make([]map[string]interface{}, 0, len(notifications))
	for _, notification := range notifications {
		notificationEntries = append(notificationEntries, map[string]interface{}{
			"notification_id": notification.ID,
			"title":           notification.Title,
			"message":         notification.Message,
			"status":          notification.Status,
			"employee_id":     notification.EmployeeID,
			"source_id":       notification.SourceID,
			"created_at":      notification.CreatedAt.Format("2006-01-02"),
		})
	}
	result["task_notifications"] = selectContextEntries(notificationEntries, terms, 8)
	result["search_terms"] = terms

	return result
}

func (a *App) getDatabaseAccessContext(intent Intent, hasFinanceAccess bool) map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	terms := buildContextSearchTerms(intent)
	result["search_terms"] = terms
	result["notes_ledger"] = a.getCrossModuleNotesContext(terms, hasFinanceAccess)
	result["commercial_records"] = a.getCommercialRecordContext(terms)
	if hasFinanceAccess {
		result["financial_records"] = a.getFinancialRecordContext(terms)
	}
	return result
}

func (a *App) getDatabaseInventoryContext(hasFinanceAccess bool) map[string]interface{} {
	result := map[string]interface{}{
		"query_policy":        "All answers must use controlled ERP queries and RBAC. Finance, banking, payroll, payments, and invoice amounts require finance:view.",
		"finance_data_access": hasFinanceAccess,
	}
	if a.db == nil {
		return result
	}

	tables, err := a.db.Migrator().GetTables()
	if err != nil {
		result["error"] = err.Error()
		return result
	}
	sort.Strings(tables)

	financeTerms := []string{"invoice", "payment", "bank", "cash", "cheque", "expense", "payroll", "salary", "journal", "account", "vat", "credit_note", "supplier_invoice"}
	tableEntries := make([]map[string]interface{}, 0, len(tables))
	moduleCounts := make(map[string]int)
	for _, table := range tables {
		if strings.HasPrefix(table, "sqlite_") {
			continue
		}
		module := classifyDatabaseTableModule(table)
		moduleCounts[module]++
		row := map[string]interface{}{
			"table":  table,
			"module": module,
		}
		if hasFinanceAccess || !databaseTableLooksFinancial(table, financeTerms) {
			var count int64
			if err := a.db.Table(table).Count(&count).Error; err == nil {
				row["rows"] = count
			} else {
				row["count_error"] = err.Error()
			}
			row["access"] = "queryable"
		} else {
			row["access"] = "finance_view_required"
		}
		tableEntries = append(tableEntries, row)
	}

	result["total_tables"] = len(tableEntries)
	result["modules"] = moduleCounts
	result["tables"] = tableEntries
	result["controlled_query_surfaces"] = []string{
		"customers", "suppliers", "contacts", "orders", "order_items", "offers", "offer_items", "opportunities", "rfqs",
		"costing_sheets", "purchase_orders", "goods_received_notes", "delivery_notes", "serial_numbers", "products",
		"invoices", "payments", "supplier_invoices", "supplier_payments", "bank_statements", "expenses", "payroll",
		"employees", "tasks", "comments", "notifications", "notes", "audit_logs",
	}
	return result
}

func databaseTableLooksFinancial(table string, financeTerms []string) bool {
	lower := strings.ToLower(table)
	for _, term := range financeTerms {
		if strings.Contains(lower, term) {
			return true
		}
	}
	return false
}

func classifyDatabaseTableModule(table string) string {
	lower := strings.ToLower(table)
	switch {
	case strings.Contains(lower, "customer") || strings.Contains(lower, "supplier") || strings.Contains(lower, "contact") || strings.Contains(lower, "entity_note"):
		return "relationships"
	case strings.Contains(lower, "offer") || strings.Contains(lower, "opportunit") || strings.Contains(lower, "rfq") || strings.Contains(lower, "costing"):
		return "commercial"
	case strings.Contains(lower, "order") || strings.Contains(lower, "delivery") || strings.Contains(lower, "grn") || strings.Contains(lower, "serial") || strings.Contains(lower, "purchase"):
		return "operations"
	case strings.Contains(lower, "invoice") || strings.Contains(lower, "payment") || strings.Contains(lower, "bank") || strings.Contains(lower, "expense") || strings.Contains(lower, "payroll") || strings.Contains(lower, "cheque") || strings.Contains(lower, "journal") || strings.Contains(lower, "vat"):
		return "finance"
	case strings.Contains(lower, "employee") || strings.Contains(lower, "task") || strings.Contains(lower, "project") || strings.Contains(lower, "notification"):
		return "work"
	case strings.Contains(lower, "conversation") || strings.Contains(lower, "chat") || strings.Contains(lower, "prediction") || strings.Contains(lower, "memory"):
		return "intelligence"
	case strings.Contains(lower, "license") || strings.Contains(lower, "role") || strings.Contains(lower, "user") || strings.Contains(lower, "device") || strings.Contains(lower, "sync") || strings.Contains(lower, "setting") || strings.Contains(lower, "audit"):
		return "system"
	default:
		return "other"
	}
}

func (a *App) getCrossModuleNotesContext(terms []string, hasFinanceAccess bool) map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	var entityNotes []EntityNote
	_ = a.db.Order("created_at DESC").Limit(30).Find(&entityNotes).Error
	entityEntries := make([]map[string]interface{}, 0, len(entityNotes))
	for _, note := range entityNotes {
		entityEntries = append(entityEntries, map[string]interface{}{
			"module":      "entity_notes",
			"entity_type": note.EntityType,
			"entity_id":   note.EntityID,
			"note_type":   note.NoteType,
			"content":     note.Content,
			"created_at":  note.CreatedAt.Format("2006-01-02"),
		})
	}
	result["entity_notes"] = selectContextEntries(entityEntries, terms, 10)

	var offerNotes []OfferNote
	_ = a.db.Order("note_date DESC").Limit(20).Find(&offerNotes).Error
	offerNoteEntries := make([]map[string]interface{}, 0, len(offerNotes))
	for _, note := range offerNotes {
		var offer Offer
		_ = a.db.Select("offer_number, customer_name, stage").Where("id = ?", note.OfferID).First(&offer).Error
		offerNoteEntries = append(offerNoteEntries, map[string]interface{}{
			"module":       "offer_notes",
			"offer_id":     note.OfferID,
			"offer_number": offer.OfferNumber,
			"customer":     offer.CustomerName,
			"stage":        offer.Stage,
			"content":      note.Content,
			"note_date":    note.NoteDate.Format("2006-01-02"),
		})
	}
	result["offer_notes"] = selectContextEntries(offerNoteEntries, terms, 8)

	var statements []BankStatement
	_ = a.db.Where("TRIM(COALESCE(notes,'')) != ''").Order("statement_date DESC").Limit(20).Find(&statements).Error
	statementEntries := make([]map[string]interface{}, 0, len(statements))
	for _, statement := range statements {
		statementEntries = append(statementEntries, map[string]interface{}{
			"module":           "bank_statements",
			"statement_id":     statement.ID,
			"statement_number": statement.StatementNumber,
			"statement_date":   statement.StatementDate.Format("2006-01-02"),
			"division":         statement.Division,
			"status":           statement.Status,
			"notes":            statement.Notes,
		})
	}
	result["bank_statement_notes"] = selectContextEntries(statementEntries, terms, 8)

	if hasFinanceAccess {
		var recons []BookBankReconciliation
		_ = a.db.Where("TRIM(COALESCE(notes,'')) != ''").Order("reconciliation_date DESC").Limit(15).Find(&recons).Error
		reconEntries := make([]map[string]interface{}, 0, len(recons))
		for _, recon := range recons {
			reconEntries = append(reconEntries, map[string]interface{}{
				"module":              "book_bank_reconciliations",
				"reconciliation_id":   recon.ID,
				"reconciliation_date": recon.ReconciliationDate.Format("2006-01-02"),
				"difference":          recon.Difference,
				"notes":               recon.Notes,
			})
		}
		result["book_bank_notes"] = selectContextEntries(reconEntries, terms, 6)

		var expenses []ExpenseEntry
		_ = a.db.Where("TRIM(COALESCE(notes,'')) != ''").Order("expense_date DESC").Limit(20).Find(&expenses).Error
		expenseEntries := make([]map[string]interface{}, 0, len(expenses))
		for _, expense := range expenses {
			expenseEntries = append(expenseEntries, map[string]interface{}{
				"module":       "expense_entries",
				"entry_number": expense.EntryNumber,
				"division":     expense.Division,
				"expense_date": expense.ExpenseDate.Format("2006-01-02"),
				"description":  expense.Description,
				"total_amount": expense.TotalAmount,
				"notes":        expense.Notes,
			})
		}
		result["expense_notes"] = selectContextEntries(expenseEntries, terms, 8)

		var runs []PayrollRun
		_ = a.db.Where("TRIM(COALESCE(notes,'')) != ''").Order("created_at DESC").Limit(10).Find(&runs).Error
		payrollEntries := make([]map[string]interface{}, 0, len(runs))
		for _, run := range runs {
			payrollEntries = append(payrollEntries, map[string]interface{}{
				"module":       "payroll_runs",
				"run_number":   run.RunNumber,
				"division":     run.Division,
				"status":       run.Status,
				"net_total":    run.NetTotal,
				"notes":        run.Notes,
				"generated_at": formatOptionalDate(run.GeneratedAt),
			})
		}
		result["payroll_notes"] = selectContextEntries(payrollEntries, terms, 5)
	}

	return result
}

func (a *App) getCommercialRecordContext(terms []string) map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	var offers []Offer
	_ = a.db.Order("quotation_date DESC").Limit(15).Find(&offers).Error
	offerEntries := make([]map[string]interface{}, 0, len(offers))
	for _, offer := range offers {
		entry := map[string]interface{}{
			"offer_id":      offer.ID,
			"offer_number":  offer.OfferNumber,
			"customer":      offer.CustomerName,
			"stage":         offer.Stage,
			"date":          offer.QuotationDate.Format("2006-01-02"),
			"validity_date": offer.ValidityDate.Format("2006-01-02"),
			"value_bhd":     offer.TotalValueBHD,
			"division":      offer.Division,
			"quote_type":    offer.QuoteType,
		}
		var items []OfferItem
		_ = a.db.Where("offer_id = ?", offer.ID).Order("line_number ASC").Find(&items).Error
		lineItems := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			lineItems = append(lineItems, map[string]interface{}{
				"line_number":     item.LineNumber,
				"description":     item.Description,
				"equipment":       item.Equipment,
				"model":           item.Model,
				"quantity":        item.Quantity,
				"unit_price_bhd":  item.UnitPrice,
				"total_price_bhd": item.TotalPrice,
				"margin_pct":      item.MarginPercent,
			})
		}
		entry["line_items"] = lineItems
		offerEntries = append(offerEntries, entry)
	}
	result["offers"] = selectContextEntries(offerEntries, terms, 8)

	var orders []Order
	_ = a.db.Order("order_date DESC").Limit(15).Find(&orders).Error
	orderEntries := make([]map[string]interface{}, 0, len(orders))
	for _, order := range orders {
		entry := map[string]interface{}{
			"order_id":      order.ID,
			"order_number":  order.OrderNumber,
			"customer":      order.CustomerName,
			"status":        order.Status,
			"date":          order.OrderDate.Format("2006-01-02"),
			"value_bhd":     order.GrandTotalBHD,
			"division":      order.Division,
			"offer_number":  order.OfferNumber,
			"customer_po":   order.CustomerPONumber,
			"payment_terms": order.PaymentTerms,
		}
		var items []OrderItem
		_ = a.db.Where("order_id = ?", order.ID).Order("line_number ASC").Find(&items).Error
		lineItems := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			lineItems = append(lineItems, map[string]interface{}{
				"line_number":     item.LineNumber,
				"description":     item.Description,
				"equipment":       item.Equipment,
				"model":           item.Model,
				"quantity":        item.Quantity,
				"unit_price_bhd":  item.UnitPrice,
				"total_price_bhd": item.TotalPrice,
				"margin_pct":      item.MarginPercent,
			})
		}
		entry["line_items"] = lineItems
		orderEntries = append(orderEntries, entry)
	}
	result["orders"] = selectContextEntries(orderEntries, terms, 8)

	var invoices []Invoice
	_ = a.db.Order("invoice_date DESC").Limit(15).Find(&invoices).Error
	invoiceEntries := make([]map[string]interface{}, 0, len(invoices))
	for _, invoice := range invoices {
		entry := map[string]interface{}{
			"invoice_id":      invoice.ID,
			"invoice_number":  invoice.InvoiceNumber,
			"customer":        invoice.CustomerName,
			"status":          invoice.Status,
			"invoice_date":    invoice.InvoiceDate.Format("2006-01-02"),
			"due_date":        invoice.DueDate.Format("2006-01-02"),
			"grand_total_bhd": invoice.GrandTotalBHD,
			"outstanding_bhd": invoice.OutstandingBHD,
			"division":        invoice.Division,
			"offer_number":    invoice.OfferNumber,
		}
		var items []DBInvoiceItem
		_ = a.db.Where("invoice_id = ?", invoice.ID).Order("line_number ASC").Find(&items).Error
		lineItems := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			lineItems = append(lineItems, map[string]interface{}{
				"line_number":     item.LineNumber,
				"description":     item.Description,
				"equipment":       item.Equipment,
				"model":           item.Model,
				"quantity":        item.Quantity,
				"unit_price_bhd":  item.Rate,
				"total_price_bhd": item.TotalBHD,
				"margin_pct":      item.MarginPercent,
			})
		}
		entry["line_items"] = lineItems
		invoiceEntries = append(invoiceEntries, entry)
	}
	result["customer_invoices"] = selectContextEntries(invoiceEntries, terms, 8)

	var supplierInvoices []SupplierInvoice
	_ = a.db.Order("invoice_date DESC").Limit(15).Find(&supplierInvoices).Error
	supplierInvoiceEntries := make([]map[string]interface{}, 0, len(supplierInvoices))
	for _, invoice := range supplierInvoices {
		entry := map[string]interface{}{
			"supplier_invoice_id": invoice.ID,
			"invoice_number":      invoice.InvoiceNumber,
			"supplier":            invoice.SupplierName,
			"status":              invoice.Status,
			"invoice_date":        invoice.InvoiceDate.Format("2006-01-02"),
			"due_date":            invoice.DueDate.Format("2006-01-02"),
			"total_bhd":           invoice.TotalBHD,
			"payment_status":      invoice.PaymentStatus,
			"division":            invoice.Division,
			"po_number":           invoice.PONumber,
		}
		var items []SupplierInvoiceItem
		_ = a.db.Where("supplier_invoice_id = ?", invoice.ID).Order("line_number ASC").Find(&items).Error
		lineItems := make([]map[string]interface{}, 0, len(items))
		for _, item := range items {
			lineItems = append(lineItems, map[string]interface{}{
				"line_number": item.LineNumber,
				"description": item.Description,
				"quantity":    item.Quantity,
				"unit_price":  item.UnitPrice,
				"total_price": item.TotalPrice,
				"currency":    item.Currency,
			})
		}
		entry["line_items"] = lineItems
		supplierInvoiceEntries = append(supplierInvoiceEntries, entry)
	}
	result["supplier_invoices"] = selectContextEntries(supplierInvoiceEntries, terms, 8)

	var opportunities []Opportunity
	_ = a.db.Order("offer_date DESC").Limit(15).Find(&opportunities).Error
	opportunityEntries := make([]map[string]interface{}, 0, len(opportunities))
	for _, opportunity := range opportunities {
		opportunityEntries = append(opportunityEntries, map[string]interface{}{
			"opportunity_id": opportunity.ID,
			"folder_number":  opportunity.FolderNumber,
			"title":          firstNonEmpty(opportunity.Title, opportunity.FolderName),
			"customer":       opportunity.CustomerName,
			"stage":          opportunity.Stage,
			"salesperson":    opportunity.Salesperson,
			"revenue_bhd":    opportunity.RevenueBHD,
			"comment":        opportunity.Comment,
			"owner_notes":    opportunity.OwnerNotes,
			"division":       opportunity.Division,
		})
	}
	result["opportunities"] = selectContextEntries(opportunityEntries, terms, 8)

	var followUps []OfferFollowUp
	_ = a.db.Order("follow_up_date DESC").Limit(20).Find(&followUps).Error
	followUpEntries := make([]map[string]interface{}, 0, len(followUps))
	for _, followUp := range followUps {
		var offer Offer
		_ = a.db.Select("offer_number, customer_name").Where("id = ?", followUp.OfferID).First(&offer).Error
		followUpEntries = append(followUpEntries, map[string]interface{}{
			"offer_id":       followUp.OfferID,
			"offer_number":   offer.OfferNumber,
			"customer":       offer.CustomerName,
			"follow_up_date": followUp.FollowUpDate.Format("2006-01-02"),
			"status":         followUp.Status,
			"notes":          followUp.Notes,
		})
	}
	result["offer_follow_ups"] = selectContextEntries(followUpEntries, terms, 8)

	return result
}

func (a *App) getFinancialRecordContext(terms []string) map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	var payments []Payment
	_ = a.db.Order("payment_date DESC").Limit(20).Find(&payments).Error
	paymentEntries := make([]map[string]interface{}, 0, len(payments))
	for _, payment := range payments {
		var invoice Invoice
		_ = a.db.Select("customer_name").Where("id = ?", payment.InvoiceID).First(&invoice).Error
		paymentEntries = append(paymentEntries, map[string]interface{}{
			"payment_id":      payment.ID,
			"invoice_number":  payment.InvoiceNumber,
			"customer":        invoice.CustomerName,
			"amount_bhd":      payment.AmountBHD,
			"payment_date":    payment.PaymentDate.Format("2006-01-02"),
			"payment_method":  payment.PaymentMethod,
			"reference":       payment.Reference,
			"days_to_payment": payment.DaysToPayment,
			"division":        payment.Division,
		})
	}
	result["customer_payments"] = selectContextEntries(paymentEntries, terms, 10)

	var supplierPayments []SupplierPayment
	_ = a.db.Order("payment_date DESC").Limit(20).Find(&supplierPayments).Error
	supplierPaymentEntries := make([]map[string]interface{}, 0, len(supplierPayments))
	for _, payment := range supplierPayments {
		supplierPaymentEntries = append(supplierPaymentEntries, map[string]interface{}{
			"supplier_payment_id": payment.ID,
			"supplier":            payment.SupplierName,
			"invoice_number":      payment.InvoiceNumber,
			"amount_bhd":          payment.AmountBHD,
			"payment_date":        payment.PaymentDate.Format("2006-01-02"),
			"payment_method":      payment.PaymentMethod,
			"reference":           payment.Reference,
			"currency":            payment.Currency,
			"division":            payment.Division,
			"notes":               payment.Notes,
		})
	}
	result["supplier_payments"] = selectContextEntries(supplierPaymentEntries, terms, 10)

	var statements []BankStatement
	_ = a.db.Order("statement_date DESC").Limit(10).Find(&statements).Error
	statementEntries := make([]map[string]interface{}, 0, len(statements))
	for _, statement := range statements {
		entry := map[string]interface{}{
			"statement_id":     statement.ID,
			"statement_number": statement.StatementNumber,
			"statement_date":   statement.StatementDate.Format("2006-01-02"),
			"status":           statement.Status,
			"division":         statement.Division,
			"opening_balance":  statement.OpeningBalance,
			"closing_balance":  statement.ClosingBalance,
			"total_debits":     statement.TotalDebits,
			"total_credits":    statement.TotalCredits,
			"notes":            statement.Notes,
		}
		var lines []BankStatementLine
		_ = a.db.Where("bank_statement_id = ?", statement.ID).Order("transaction_date DESC, line_number DESC").Limit(5).Find(&lines).Error
		lineEntries := make([]map[string]interface{}, 0, len(lines))
		for _, line := range lines {
			lineEntries = append(lineEntries, map[string]interface{}{
				"line_number":        line.LineNumber,
				"transaction_date":   line.TransactionDate.Format("2006-01-02"),
				"description":        line.Description,
				"reference":          line.Reference,
				"debit":              line.Debit,
				"credit":             line.Credit,
				"is_matched":         line.IsMatched,
				"transaction_type":   line.TransactionType,
				"extracted_customer": line.ExtractedCustomer,
				"extracted_supplier": line.ExtractedSupplier,
				"notes":              line.Notes,
			})
		}
		entry["lines"] = lineEntries
		statementEntries = append(statementEntries, entry)
	}
	result["bank_statements"] = selectContextEntries(statementEntries, terms, 6)

	var expenses []ExpenseEntry
	_ = a.db.Order("expense_date DESC").Limit(15).Find(&expenses).Error
	expenseEntries := make([]map[string]interface{}, 0, len(expenses))
	for _, expense := range expenses {
		expenseEntries = append(expenseEntries, map[string]interface{}{
			"expense_id":     expense.ID,
			"entry_number":   expense.EntryNumber,
			"division":       expense.Division,
			"expense_date":   expense.ExpenseDate.Format("2006-01-02"),
			"description":    expense.Description,
			"status":         expense.Status,
			"payment_status": expense.PaymentStatus,
			"total_amount":   expense.TotalAmount,
			"payment_method": expense.PaymentMethod,
			"payment_ref":    expense.PaymentReference,
			"notes":          expense.Notes,
		})
	}
	result["expenses"] = selectContextEntries(expenseEntries, terms, 8)

	return result
}

func firstNonEmptyStringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func (a *App) getServiceDueInstallationsContext() map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	type serviceCandidate struct {
		SerialID           string
		SerialNo           string
		ProductID          string
		ProductCode        string
		ProductName        string
		CustomerID         string
		CustomerName       string
		DeliveryAddress    string
		InvoiceID          string
		InvoiceNumber      string
		InvoiceDate        *time.Time
		LineValueBHD       float64
		OEMName            string
		WarrantyEndDate    *time.Time
		CalibrationDueDate *time.Time
		WarrantyStartDate  *time.Time
	}

	var rows []serviceCandidate
	a.db.Table("serial_numbers").
		Select(`serial_numbers.id as serial_id, serial_numbers.serial_no, serial_numbers.product_id, serial_numbers.product_code,
			products.product_name, serial_numbers.customer_id, serial_numbers.customer_name,
			delivery_notes.delivery_address, serial_numbers.invoice_id, serial_numbers.invoice_number,
			invoices.invoice_date, COALESCE(invoice_items.total_bhd, invoices.grand_total_bhd, 0) as line_value_bhd,
			suppliers.supplier_name as oem_name, serial_numbers.warranty_end_date, serial_numbers.calibration_due_date,
			serial_numbers.warranty_start_date`).
		Joins("LEFT JOIN products ON products.id = serial_numbers.product_id").
		Joins("LEFT JOIN suppliers ON suppliers.id = products.supplier_id").
		Joins("LEFT JOIN invoices ON invoices.id = serial_numbers.invoice_id").
		Joins("LEFT JOIN invoice_items ON invoice_items.invoice_id = serial_numbers.invoice_id AND invoice_items.product_id = serial_numbers.product_id").
		Joins("LEFT JOIN delivery_notes ON delivery_notes.id = invoices.delivery_note_id").
		Where("serial_numbers.status = ?", "Delivered").
		Order("COALESCE(invoice_items.total_bhd, invoices.grand_total_bhd, 0) DESC, invoices.invoice_date DESC").
		Limit(50).
		Scan(&rows)

	now := time.Now()
	dueSoon := now.AddDate(0, 0, 45)
	top := make([]map[string]interface{}, 0, 5)
	seen := make(map[string]bool)

	for _, row := range rows {
		serviceDate, dueReason, isDue := deriveServiceDueStatus(now, dueSoon, row.InvoiceDate, row.WarrantyStartDate, row.WarrantyEndDate, row.CalibrationDueDate)
		if !isDue {
			continue
		}

		groupKey := firstNonEmpty(row.InvoiceID+"|"+row.ProductID, row.InvoiceNumber+"|"+row.ProductCode, row.SerialID)
		if groupKey == "" || seen[groupKey] {
			continue
		}
		seen[groupKey] = true

		top = append(top, map[string]interface{}{
			"customer_name":       row.CustomerName,
			"customer_id":         row.CustomerID,
			"site":                row.DeliveryAddress,
			"equipment":           firstNonEmpty(row.ProductName, row.ProductCode),
			"product_code":        row.ProductCode,
			"serial_no":           row.SerialNo,
			"oem":                 row.OEMName,
			"invoice_number":      row.InvoiceNumber,
			"invoice_date":        formatOptionalDate(row.InvoiceDate),
			"estimated_value_bhd": row.LineValueBHD,
			"service_due_date":    serviceDate,
			"due_reason":          dueReason,
		})
		if len(top) >= 5 {
			break
		}
	}

	result["top_due_installations"] = top
	result["field_follow_up_candidates"] = buildFieldFollowUpCandidates(top)
	result["basis"] = "Delivered serial-tracked installations due by calibration date, warranty milestone, or age-from-delivery heuristic"
	result["window_days"] = 45
	return result
}

func deriveServiceDueStatus(now, dueSoon time.Time, invoiceDate, warrantyStartDate, warrantyEndDate, calibrationDueDate *time.Time) (string, string, bool) {
	if calibrationDueDate != nil && !calibrationDueDate.After(dueSoon) {
		return calibrationDueDate.Format("2006-01-02"), "calibration due", true
	}
	if warrantyEndDate != nil && !warrantyEndDate.After(dueSoon) {
		return warrantyEndDate.Format("2006-01-02"), "warranty milestone due", true
	}
	baseDate := invoiceDate
	if warrantyStartDate != nil {
		baseDate = warrantyStartDate
	}
	if baseDate != nil {
		heuristicDue := baseDate.AddDate(1, 0, 0)
		if !heuristicDue.After(dueSoon) {
			return heuristicDue.Format("2006-01-02"), "annual service heuristic", true
		}
	}
	return "", "", false
}

func (a *App) getCloseableOpportunitiesContext() map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	type closeableCandidate struct {
		OpportunityID    string
		FolderNumber     string
		Title            string
		CustomerID       string
		CustomerName     string
		Stage            string
		Salesperson      string
		OfferID          string
		OfferNumber      string
		RevenueBHD       float64
		InvoiceNumber    string
		OutstandingBHD   float64
		PaymentDate      *time.Time
		PaymentAmountBHD float64
	}

	var rows []closeableCandidate
	a.db.Table("opportunities").
		Select(`opportunities.id as opportunity_id, opportunities.folder_number, opportunities.title,
			opportunities.customer_id, opportunities.customer_name, opportunities.stage, opportunities.salesperson,
			opportunities.offer_id, invoices.offer_number, opportunities.revenue_bhd,
			invoices.invoice_number, invoices.outstanding_bhd, payments.payment_date, payments.amount_bhd as payment_amount_bhd`).
		Joins("LEFT JOIN invoices ON invoices.offer_id = opportunities.offer_id").
		Joins("LEFT JOIN payments ON payments.invoice_id = invoices.id").
		Where("opportunities.stage NOT IN ?", []string{"Won", "Lost"}).
		Where("opportunities.offer_id <> ''").
		Where("invoices.id IS NOT NULL").
		Where("invoices.outstanding_bhd <= ?", 0.001).
		Order("payments.payment_date DESC, invoices.invoice_date DESC").
		Limit(25).
		Scan(&rows)

	candidates := make([]map[string]interface{}, 0, len(rows))
	seen := make(map[string]bool)
	for _, row := range rows {
		if seen[row.OpportunityID] {
			continue
		}
		seen[row.OpportunityID] = true
		candidates = append(candidates, map[string]interface{}{
			"opportunity_id":        row.OpportunityID,
			"folder_number":         row.FolderNumber,
			"title":                 firstNonEmpty(row.Title, row.OfferNumber, row.InvoiceNumber),
			"customer_id":           row.CustomerID,
			"customer_name":         row.CustomerName,
			"current_stage":         row.Stage,
			"salesperson":           row.Salesperson,
			"offer_id":              row.OfferID,
			"offer_number":          row.OfferNumber,
			"invoiced_reference":    row.InvoiceNumber,
			"payment_received_date": formatOptionalDate(row.PaymentDate),
			"payment_amount_bhd":    row.PaymentAmountBHD,
			"expected_revenue_bhd":  row.RevenueBHD,
			"close_readiness":       "Fully paid invoice linked to opportunity offer",
			"evidence_trail": []map[string]interface{}{
				{"step": "offer_linked", "value": row.OfferID != "", "detail": row.OfferNumber},
				{"step": "invoice_linked", "value": row.InvoiceNumber != "", "detail": row.InvoiceNumber},
				{"step": "invoice_paid", "value": true, "detail": fmt.Sprintf("Outstanding %.3f BHD", row.OutstandingBHD)},
				{"step": "payment_recorded", "value": row.PaymentDate != nil, "detail": formatOptionalDate(row.PaymentDate)},
			},
		})
	}

	result["candidates"] = candidates
	result["rule"] = "Candidate when linked invoice exists against the opportunity offer and outstanding amount is effectively zero"
	return result
}

func (a *App) getInstalledBaseSummary(customerName string) map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil || strings.TrimSpace(customerName) == "" {
		return result
	}

	escaped := escapeLikeWildcards(customerName)
	type installedBaseRow struct {
		CustomerName       string
		ProductCode        string
		ProductName        string
		OEMName            string
		SerialNo           string
		InvoiceNumber      string
		InvoiceDate        *time.Time
		DeliveryAddress    string
		WarrantyEndDate    *time.Time
		CalibrationDueDate *time.Time
	}

	var rows []installedBaseRow
	a.db.Table("serial_numbers").
		Select(`serial_numbers.customer_name, serial_numbers.product_code, products.product_name, suppliers.supplier_name as oem_name,
			serial_numbers.serial_no, serial_numbers.invoice_number, invoices.invoice_date, delivery_notes.delivery_address,
			serial_numbers.warranty_end_date, serial_numbers.calibration_due_date`).
		Joins("LEFT JOIN products ON products.id = serial_numbers.product_id").
		Joins("LEFT JOIN suppliers ON suppliers.id = products.supplier_id").
		Joins("LEFT JOIN invoices ON invoices.id = serial_numbers.invoice_id").
		Joins("LEFT JOIN delivery_notes ON delivery_notes.id = invoices.delivery_note_id").
		Where("serial_numbers.customer_name LIKE ? ESCAPE '\\'", "%"+escaped+"%").
		Order("invoices.invoice_date DESC").
		Limit(25).
		Scan(&rows)

	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]interface{}{
			"customer_name":        row.CustomerName,
			"equipment":            firstNonEmpty(row.ProductName, row.ProductCode),
			"product_code":         row.ProductCode,
			"oem":                  row.OEMName,
			"serial_no":            row.SerialNo,
			"invoice_number":       row.InvoiceNumber,
			"invoice_date":         formatOptionalDate(row.InvoiceDate),
			"site":                 row.DeliveryAddress,
			"warranty_end_date":    formatOptionalDate(row.WarrantyEndDate),
			"calibration_due_date": formatOptionalDate(row.CalibrationDueDate),
		})
	}

	result["customer_name"] = customerName
	result["serial_tracked_items"] = items
	result["installed_base_count"] = len(items)
	return result
}

func (a *App) getButlerMemoryContext(intent Intent) map[string]interface{} {
	ctx := make(map[string]interface{})
	if a.db == nil {
		return ctx
	}

	resolution := a.resolveBestEntityReference(intent)
	if resolution != nil {
		switch resolution.EntityType {
		case "customer":
			ctx["entity_notes"] = a.getEntityNotesForMemory("customer", resolution.EntityID)
			ctx["follow_ups"] = a.getFollowUpsForMemory(resolution.EntityID)
		case "customer_contact":
			ctx["entity_notes"] = a.getEntityNotesForMemory("customer", resolution.RelatedCustomerID)
			ctx["follow_ups"] = a.getFollowUpsForMemory(resolution.RelatedCustomerID)
		}
	}

	ctx["quick_memory"] = a.getRecentOpenQuickCaptures()
	return ctx
}

func (a *App) getEntityNotesForMemory(entityType, entityID string) []map[string]interface{} {
	if strings.TrimSpace(entityID) == "" {
		return []map[string]interface{}{}
	}

	var notes []EntityNote
	a.db.Where("entity_type = ? AND entity_id = ?", entityType, entityID).
		Order("created_at DESC").
		Limit(5).
		Find(&notes)
	if entityType == "customer" && len(notes) == 0 {
		var customer CustomerMaster
		if err := a.db.Where("customer_id = ?", entityID).First(&customer).Error; err == nil && strings.TrimSpace(customer.ID) != "" {
			a.db.Where("entity_type = ? AND entity_id = ?", entityType, customer.ID).
				Order("created_at DESC").
				Limit(5).
				Find(&notes)
		}
	}

	memories := make([]map[string]interface{}, 0, len(notes))
	for _, note := range notes {
		memories = append(memories, map[string]interface{}{
			"type":       note.NoteType,
			"content":    note.Content,
			"created_at": note.CreatedAt.Format("2006-01-02"),
		})
	}
	return memories
}

func (a *App) getFollowUpsForMemory(customerID string) []map[string]interface{} {
	if strings.TrimSpace(customerID) == "" {
		return []map[string]interface{}{}
	}

	var tasks []FollowUpTask
	a.db.Where("customer_id = ? AND status IN ?", customerID, []string{"pending", "in_progress", "overdue"}).
		Order("due_date ASC").
		Limit(5).
		Find(&tasks)

	memories := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		memories = append(memories, map[string]interface{}{
			"title":       task.Title,
			"status":      task.Status,
			"priority":    task.Priority,
			"due_date":    task.DueDate.Format("2006-01-02"),
			"description": task.Description,
		})
	}
	return memories
}

func (a *App) getRecentOpenQuickCaptures() []map[string]interface{} {
	var captures []QuickCapture
	a.db.Where("status IN ?", []string{"Open", "In Progress"}).
		Order("created_at DESC").
		Limit(5).
		Find(&captures)

	items := make([]map[string]interface{}, 0, len(captures))
	for _, capture := range captures {
		items = append(items, map[string]interface{}{
			"title":      capture.Title,
			"priority":   capture.Priority,
			"status":     capture.Status,
			"source":     firstNonEmpty(capture.Tags, "quick_capture"),
			"created_at": capture.CreatedAt.Format("2006-01-02"),
		})
	}
	return items
}

type contactMatch struct {
	CustomerContact
	BusinessName string
	BusinessID   string
}

func (a *App) getProactiveBriefingContext(hasFinanceAccess bool) map[string]interface{} {
	ctx := make(map[string]interface{})
	actionItems := a.getActionItemsContext()
	riskData := a.getRiskContext()
	serviceDue := a.getServiceDueInstallationsContext()

	ctx["action_items"] = actionItems
	ctx["risk_summary"] = riskData
	ctx["service_due"] = serviceDue

	if hasFinanceAccess {
		ctx["cashflow_projection"] = a.getCashflowProjectionContext()
		ctx["ar_summary"] = a.getARSummary()
	}

	return ctx
}

func (a *App) getQuotationPrecheckContext(intent Intent) map[string]interface{} {
	result := make(map[string]interface{})
	resolution := a.resolveBestEntityReference(intent)
	if resolution == nil {
		result["status"] = "needs_customer_resolution"
		result["missing_fields"] = []string{"customer"}
		result["draft_clues"] = extractQuotationDraftClues(intent.RawQuery)
		return result
	}

	customerName := resolution.DisplayName
	if resolution.EntityType == "customer_contact" && resolution.RelatedCustomer != "" {
		customerName = resolution.RelatedCustomer
	}

	customerCtx := a.getCustomerContext(customerName)
	customer, _ := customerCtx["customer"].(map[string]interface{})
	contacts, _ := customerCtx["contacts"].([]map[string]interface{})
	offers, _ := customerCtx["active_offers"].([]map[string]interface{})

	var primaryContact map[string]interface{}
	for _, contact := range contacts {
		if isPrimary, _ := contact["is_primary"].(bool); isPrimary {
			primaryContact = contact
			break
		}
	}
	if primaryContact == nil && len(contacts) > 0 {
		primaryContact = contacts[0]
	}

	missing := make([]string, 0)
	if customerName == "" {
		missing = append(missing, "customer")
	}
	if len(primaryContact) == 0 {
		missing = append(missing, "contact_person")
	}
	paymentTerms, _ := customer["payment_terms_days"].(int)
	if paymentTerms == 0 {
		missing = append(missing, "payment_terms")
	}

	result["status"] = "ready"
	if len(missing) > 0 {
		result["status"] = "needs_inputs"
	}
	if resolution.Ambiguous {
		result["status"] = "needs_clarification"
	}
	result["resolved_customer"] = customer
	result["primary_contact"] = primaryContact
	result["recent_active_offers"] = offers
	result["missing_fields"] = missing
	result["draft_clues"] = extractQuotationDraftClues(intent.RawQuery)
	result["ambiguity_summary"] = buildAmbiguitySummary(resolution)
	result["draft_payload"] = buildQuotationDraftPayload(customer, primaryContact, extractQuotationDraftClues(intent.RawQuery), missing, resolution)
	result["default_guidance"] = map[string]interface{}{
		"vat_percent": 10,
		"quote_types": []string{"Quotation", "Budgetary Quote", "Estimate", "Technical", "Commercial"},
	}
	return result
}

func (a *App) injectGroundedReferenceContext(context map[string]interface{}, intent Intent, hasFinanceAccess bool) {
	resolution := a.resolveBestEntityReference(intent)
	if resolution == nil {
		return
	}

	context["entity_resolution"] = map[string]interface{}{
		"reference_kind":      intent.ReferenceKind,
		"entity_type":         resolution.EntityType,
		"entity_id":           resolution.EntityID,
		"display_name":        resolution.DisplayName,
		"confidence":          resolution.Confidence,
		"match_reason":        resolution.MatchReason,
		"related_customer":    resolution.RelatedCustomer,
		"related_customer_id": resolution.RelatedCustomerID,
		"ambiguous":           resolution.Ambiguous,
		"alternatives":        resolution.Alternatives,
	}
	context["clarification_prompt"] = buildAmbiguitySummary(resolution)

	switch resolution.EntityType {
	case "customer":
		customerCtx := a.getCustomerContext(resolution.DisplayName)
		if !hasFinanceAccess {
			a.redactFinancialFromContext(customerCtx)
		}
		context["customer_data"] = customerCtx
		context["customer_360"] = customerCtx
		context["installed_base_summary"] = a.getInstalledBaseSummary(resolution.DisplayName)
	case "customer_contact":
		contactCtx := a.getCustomerContactContext(resolution)
		if !hasFinanceAccess {
			delete(contactCtx, "linked_customer_financials")
		}
		context["contact_context"] = contactCtx
		if resolution.RelatedCustomer != "" {
			customerCtx := a.getCustomerContext(resolution.RelatedCustomer)
			if !hasFinanceAccess {
				a.redactFinancialFromContext(customerCtx)
			}
			context["customer_data"] = customerCtx
			context["customer_360"] = customerCtx
			context["installed_base_summary"] = a.getInstalledBaseSummary(resolution.RelatedCustomer)
		}
	case "user":
		context["account_manager_context"] = a.getAccountManagerContext(resolution)
	case "supplier":
		context["supplier_data"] = a.getSupplierContext(resolution.DisplayName)
	case "employee":
		context["employee_context"] = a.getEmployeeContext(resolution)
	}

	context["action_items"] = a.getActionItemsContext()
	context["competition_intelligence"] = a.getCompetitionContext()
}

func (a *App) resolveBestEntityReference(intent Intent) *ButlerResolvedEntity {
	if a.db == nil {
		return nil
	}

	reference := strings.TrimSpace(intent.EntityName)
	if reference == "" {
		reference = strings.TrimSpace(intent.PersonName)
	}
	if reference == "" {
		return nil
	}

	if intent.ReferenceKind == "account_manager" {
		if user := a.resolveUserReference(reference); user != nil {
			return user
		}
	}

	if intent.ReferenceKind == "employee" || intent.Domain == "work" {
		if employee := a.resolveEmployeeReference(reference); employee != nil {
			return employee
		}
	}

	if intent.ReferenceKind == "supplier" || intent.Domain == "supplier" {
		if supplier := a.resolveSupplierReference(reference); supplier != nil {
			return supplier
		}
	}

	if customer := a.resolveCustomerReference(reference); customer != nil {
		return customer
	}

	if supplier := a.resolveSupplierReference(reference); supplier != nil {
		return supplier
	}

	if contact := a.resolveCustomerContactReference(reference); contact != nil {
		return contact
	}

	if employee := a.resolveEmployeeReference(reference); employee != nil {
		return employee
	}

	if user := a.resolveUserReference(reference); user != nil {
		return user
	}

	return nil
}

func (a *App) resolveCustomerReference(reference string) *ButlerResolvedEntity {
	if reference == "" {
		return nil
	}

	var customer CustomerMaster
	exactErr := a.db.Where(
		"business_name = ? OR short_code = ? OR customer_code = ? OR customer_id = ?",
		reference, reference, reference, reference,
	).First(&customer).Error
	if exactErr == nil {
		return &ButlerResolvedEntity{
			EntityType:  "customer",
			EntityID:    customer.CustomerID,
			DisplayName: customer.BusinessName,
			Confidence:  0.98,
			MatchReason: "exact customer match",
		}
	}

	escaped := escapeLikeWildcards(reference)
	pattern := "%" + escaped + "%"
	var matches []CustomerMaster
	if err := a.db.Where(
		"business_name LIKE ? ESCAPE '\\' OR short_code LIKE ? ESCAPE '\\' OR customer_code LIKE ? ESCAPE '\\'",
		pattern, pattern, pattern,
	).Order("business_name").Limit(3).Find(&matches).Error; err == nil && len(matches) > 0 {
		customer = matches[0]
		resolution := &ButlerResolvedEntity{
			EntityType:  "customer",
			EntityID:    customer.CustomerID,
			DisplayName: customer.BusinessName,
			Confidence:  0.82,
			MatchReason: "fuzzy customer match",
		}
		if len(matches) > 1 {
			resolution.Ambiguous = true
			resolution.Confidence = 0.55
			resolution.MatchReason = "multiple customer matches"
			resolution.Alternatives = customerAlternatives(matches)
		}
		return resolution
	}

	return nil
}

func (a *App) resolveCustomerContactReference(reference string) *ButlerResolvedEntity {
	if reference == "" {
		return nil
	}

	var match contactMatch
	exactErr := a.db.Table("customer_contacts").
		Select("customer_contacts.*, customers.business_name, customers.customer_id as business_id").
		Joins("JOIN customers ON customers.id = customer_contacts.customer_id").
		Where("customer_contacts.contact_name = ?", reference).
		Order("customer_contacts.is_primary_contact DESC, customer_contacts.contact_name").
		Scan(&match).Error
	if exactErr == nil && match.ContactName != "" {
		return &ButlerResolvedEntity{
			EntityType:        "customer_contact",
			EntityID:          match.ID,
			DisplayName:       match.ContactName,
			Confidence:        0.96,
			MatchReason:       "exact contact match",
			RelatedCustomerID: match.BusinessID,
			RelatedCustomer:   match.BusinessName,
		}
	}

	escaped := escapeLikeWildcards(reference)
	pattern := "%" + escaped + "%"
	var matches []contactMatch
	if err := a.db.Table("customer_contacts").
		Select("customer_contacts.*, customers.business_name, customers.customer_id as business_id").
		Joins("JOIN customers ON customers.id = customer_contacts.customer_id").
		Where("customer_contacts.contact_name LIKE ? ESCAPE '\\'", pattern).
		Order("customer_contacts.is_primary_contact DESC, customer_contacts.contact_name").
		Limit(3).
		Scan(&matches).Error; err == nil && len(matches) > 0 && matches[0].ContactName != "" {
		match = matches[0]
		resolution := &ButlerResolvedEntity{
			EntityType:        "customer_contact",
			EntityID:          match.ID,
			DisplayName:       match.ContactName,
			Confidence:        0.79,
			MatchReason:       "fuzzy contact match",
			RelatedCustomerID: match.BusinessID,
			RelatedCustomer:   match.BusinessName,
		}
		if len(matches) > 1 {
			resolution.Ambiguous = true
			resolution.Confidence = 0.5
			resolution.MatchReason = "multiple contact matches"
			resolution.Alternatives = contactAlternatives(matches)
		}
		return resolution
	}

	return nil
}

func (a *App) resolveUserReference(reference string) *ButlerResolvedEntity {
	if reference == "" {
		return nil
	}

	var user User
	exactErr := a.db.Where(
		"full_name = ? OR display_name = ? OR username = ?",
		reference, reference, reference,
	).First(&user).Error
	if exactErr == nil {
		return &ButlerResolvedEntity{
			EntityType:  "user",
			EntityID:    user.ID,
			DisplayName: firstNonEmpty(user.DisplayName, user.FullName, user.Username),
			Confidence:  0.95,
			MatchReason: "exact internal user match",
		}
	}

	escaped := escapeLikeWildcards(reference)
	pattern := "%" + escaped + "%"
	var matches []User
	if err := a.db.Where(
		"full_name LIKE ? ESCAPE '\\' OR display_name LIKE ? ESCAPE '\\' OR username LIKE ? ESCAPE '\\'",
		pattern, pattern, pattern,
	).Order("full_name").Limit(3).Find(&matches).Error; err == nil && len(matches) > 0 {
		user = matches[0]
		resolution := &ButlerResolvedEntity{
			EntityType:  "user",
			EntityID:    user.ID,
			DisplayName: firstNonEmpty(user.DisplayName, user.FullName, user.Username),
			Confidence:  0.78,
			MatchReason: "fuzzy internal user match",
		}
		if len(matches) > 1 {
			resolution.Ambiguous = true
			resolution.Confidence = 0.52
			resolution.MatchReason = "multiple internal user matches"
			resolution.Alternatives = userAlternatives(matches)
		}
		return resolution
	}

	return nil
}

func (a *App) resolveSupplierReference(reference string) *ButlerResolvedEntity {
	if reference == "" {
		return nil
	}

	var supplier SupplierMaster
	exactErr := a.db.Where(
		"supplier_name = ? OR supplier_code = ?",
		reference, reference,
	).First(&supplier).Error
	if exactErr == nil {
		return &ButlerResolvedEntity{
			EntityType:  "supplier",
			EntityID:    supplier.ID,
			DisplayName: supplier.SupplierName,
			Confidence:  0.97,
			MatchReason: "exact supplier match",
		}
	}

	escaped := escapeLikeWildcards(reference)
	pattern := "%" + escaped + "%"
	var matches []SupplierMaster
	if err := a.db.Where(
		"supplier_name LIKE ? ESCAPE '\\' OR supplier_code LIKE ? ESCAPE '\\'",
		pattern, pattern,
	).Order("supplier_name").Limit(3).Find(&matches).Error; err == nil && len(matches) > 0 {
		supplier = matches[0]
		resolution := &ButlerResolvedEntity{
			EntityType:  "supplier",
			EntityID:    supplier.ID,
			DisplayName: supplier.SupplierName,
			Confidence:  0.81,
			MatchReason: "fuzzy supplier match",
		}
		if len(matches) > 1 {
			resolution.Ambiguous = true
			resolution.Confidence = 0.56
			resolution.MatchReason = "multiple supplier matches"
			resolution.Alternatives = supplierAlternatives(matches)
		}
		return resolution
	}

	return nil
}

func (a *App) resolveEmployeeReference(reference string) *ButlerResolvedEntity {
	if reference == "" {
		return nil
	}

	var employee Employee
	exactErr := a.db.Where(
		"full_name = ? OR preferred_name = ? OR employee_code = ? OR email = ?",
		reference, reference, reference, reference,
	).First(&employee).Error
	if exactErr == nil {
		return &ButlerResolvedEntity{
			EntityType:  "employee",
			EntityID:    employee.ID,
			DisplayName: firstNonEmpty(employee.PreferredName, employee.FullName, employee.EmployeeCode),
			Confidence:  0.97,
			MatchReason: "exact employee match",
		}
	}

	escaped := escapeLikeWildcards(reference)
	pattern := "%" + escaped + "%"
	var matches []Employee
	if err := a.db.Where(
		"full_name LIKE ? ESCAPE '\\' OR preferred_name LIKE ? ESCAPE '\\' OR employee_code LIKE ? ESCAPE '\\' OR email LIKE ? ESCAPE '\\'",
		pattern, pattern, pattern, pattern,
	).Order("preferred_name, full_name").Limit(3).Find(&matches).Error; err == nil && len(matches) > 0 {
		employee = matches[0]
		resolution := &ButlerResolvedEntity{
			EntityType:  "employee",
			EntityID:    employee.ID,
			DisplayName: firstNonEmpty(employee.PreferredName, employee.FullName, employee.EmployeeCode),
			Confidence:  0.84,
			MatchReason: "fuzzy employee match",
		}
		if len(matches) > 1 {
			resolution.Ambiguous = true
			resolution.Confidence = 0.58
			resolution.MatchReason = "multiple employee matches"
			resolution.Alternatives = employeeAlternatives(matches)
		}
		return resolution
	}

	return nil
}

func (a *App) getCustomerContactContext(resolution *ButlerResolvedEntity) map[string]interface{} {
	result := make(map[string]interface{})
	if resolution == nil || resolution.EntityType != "customer_contact" {
		return result
	}

	var contact CustomerContact
	if err := a.db.First(&contact, "id = ?", resolution.EntityID).Error; err != nil {
		return result
	}

	result["contact"] = map[string]interface{}{
		"id":                 contact.ID,
		"name":               contact.ContactName,
		"job_title":          contact.JobTitle,
		"email":              contact.Email,
		"phone":              contact.Phone,
		"is_primary_contact": contact.IsPrimaryContact,
	}
	result["linked_customer"] = map[string]interface{}{
		"customer_id":   resolution.RelatedCustomerID,
		"business_name": resolution.RelatedCustomer,
	}

	if resolution.RelatedCustomer != "" {
		customerCtx := a.getCustomerContext(resolution.RelatedCustomer)
		if customerSummary, ok := customerCtx["customer"]; ok {
			result["linked_customer_financials"] = customerSummary
		}
		if offers, ok := customerCtx["active_offers"]; ok {
			result["active_offers"] = offers
		}
		if tasks, ok := customerCtx["pending_tasks"]; ok {
			result["pending_tasks"] = tasks
		}
	}

	return result
}

func (a *App) getAccountManagerContext(resolution *ButlerResolvedEntity) map[string]interface{} {
	result := make(map[string]interface{})
	if resolution == nil || resolution.EntityType != "user" {
		return result
	}

	var user User
	if err := a.db.First(&user, "id = ?", resolution.EntityID).Error; err != nil {
		return result
	}

	identifiers := uniqueNonEmptyStrings(user.FullName, user.DisplayName, user.Username)
	result["user"] = map[string]interface{}{
		"id":           user.ID,
		"full_name":    user.FullName,
		"display_name": user.DisplayName,
		"username":     user.Username,
		"department":   user.Department,
		"job_title":    user.JobTitle,
		"is_active":    user.IsActive,
	}

	if len(identifiers) == 0 {
		return result
	}

	var opportunities []Opportunity
	if err := a.db.Where("salesperson IN ?", identifiers).
		Order("offer_date DESC").
		Limit(10).
		Find(&opportunities).Error; err != nil {
		return result
	}

	openCount := 0
	totalPipeline := 0.0
	customers := map[string]bool{}
	oppSummary := make([]map[string]interface{}, 0, len(opportunities))
	for _, opp := range opportunities {
		if opp.Stage != "Won" && opp.Stage != "Lost" {
			openCount++
			totalPipeline += opp.RevenueBHD
		}
		if opp.CustomerName != "" {
			customers[opp.CustomerName] = true
		}
		oppSummary = append(oppSummary, map[string]interface{}{
			"folder_number": opp.FolderNumber,
			"title":         firstNonEmpty(opp.Title, opp.FolderName),
			"customer_name": opp.CustomerName,
			"stage":         opp.Stage,
			"revenue_bhd":   opp.RevenueBHD,
			"offer_date":    opp.OfferDate.Format("2006-01-02"),
			"expected_date": formatOptionalDate(opp.ExpectedDate),
		})
	}

	result["portfolio_summary"] = map[string]interface{}{
		"open_opportunities": openCount,
		"tracked_customers":  len(customers),
		"open_pipeline_bhd":  totalPipeline,
	}
	result["recent_opportunities"] = oppSummary

	return result
}

func (a *App) getEmployeeContext(resolution *ButlerResolvedEntity) map[string]interface{} {
	result := make(map[string]interface{})
	if resolution == nil || resolution.EntityType != "employee" {
		return result
	}

	var employee Employee
	if err := a.db.First(&employee, "id = ?", resolution.EntityID).Error; err != nil {
		return result
	}

	result["employee"] = map[string]interface{}{
		"id":                employee.ID,
		"employee_code":     employee.EmployeeCode,
		"full_name":         employee.FullName,
		"preferred_name":    employee.PreferredName,
		"department":        employee.Department,
		"job_title":         employee.JobTitle,
		"employment_status": employee.EmploymentStatus,
		"is_active":         employee.IsActive,
		"notes":             employee.Notes,
	}

	var tasks []TaskItem
	_ = a.db.Where("assignee_employee_id = ?", employee.ID).
		Order("updated_at DESC").
		Limit(10).
		Find(&tasks).Error
	taskSummary := make([]map[string]interface{}, 0, len(tasks))
	for _, task := range tasks {
		taskSummary = append(taskSummary, map[string]interface{}{
			"title":       task.Title,
			"status":      task.Status,
			"priority":    task.Priority,
			"task_type":   task.TaskType,
			"due_date":    formatOptionalDate(task.DueDate),
			"description": task.Description,
		})
	}
	result["assigned_tasks"] = taskSummary

	var notifications []Notification
	_ = a.db.Where("employee_id = ?", employee.ID).
		Order("created_at DESC").
		Limit(10).
		Find(&notifications).Error
	notificationSummary := make([]map[string]interface{}, 0, len(notifications))
	for _, notification := range notifications {
		notificationSummary = append(notificationSummary, map[string]interface{}{
			"title":       notification.Title,
			"message":     notification.Message,
			"status":      notification.Status,
			"source_type": notification.SourceType,
			"created_at":  notification.CreatedAt.Format("2006-01-02"),
		})
	}
	result["notifications"] = notificationSummary

	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func uniqueNonEmptyStrings(values ...string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		result = append(result, trimmed)
	}
	return result
}

func formatOptionalDate(ts *time.Time) string {
	if ts == nil {
		return ""
	}
	return ts.Format("2006-01-02")
}

func customerAlternatives(matches []CustomerMaster) []map[string]interface{} {
	alternatives := make([]map[string]interface{}, 0, len(matches))
	for _, match := range matches {
		alternatives = append(alternatives, map[string]interface{}{
			"entity_type": "customer",
			"id":          match.CustomerID,
			"name":        match.BusinessName,
			"short_code":  match.ShortCode,
		})
	}
	return alternatives
}

func contactAlternatives(matches []contactMatch) []map[string]interface{} {
	alternatives := make([]map[string]interface{}, 0, len(matches))
	for _, match := range matches {
		alternatives = append(alternatives, map[string]interface{}{
			"entity_type":   "customer_contact",
			"id":            match.ID,
			"name":          match.ContactName,
			"customer_name": match.BusinessName,
			"job_title":     match.JobTitle,
		})
	}
	return alternatives
}

func userAlternatives(matches []User) []map[string]interface{} {
	alternatives := make([]map[string]interface{}, 0, len(matches))
	for _, match := range matches {
		alternatives = append(alternatives, map[string]interface{}{
			"entity_type":  "user",
			"id":           match.ID,
			"display_name": firstNonEmpty(match.DisplayName, match.FullName, match.Username),
			"department":   match.Department,
			"job_title":    match.JobTitle,
		})
	}
	return alternatives
}

func supplierAlternatives(matches []SupplierMaster) []map[string]interface{} {
	alternatives := make([]map[string]interface{}, 0, len(matches))
	for _, match := range matches {
		alternatives = append(alternatives, map[string]interface{}{
			"entity_type":   "supplier",
			"id":            match.ID,
			"display_name":  match.SupplierName,
			"supplier_code": match.SupplierCode,
			"country":       match.Country,
		})
	}
	return alternatives
}

func employeeAlternatives(matches []Employee) []map[string]interface{} {
	alternatives := make([]map[string]interface{}, 0, len(matches))
	for _, match := range matches {
		alternatives = append(alternatives, map[string]interface{}{
			"entity_type":  "employee",
			"id":           match.ID,
			"display_name": firstNonEmpty(match.PreferredName, match.FullName, match.EmployeeCode),
			"department":   match.Department,
			"job_title":    match.JobTitle,
		})
	}
	return alternatives
}

func extractQuotationDraftClues(query string) map[string]interface{} {
	clues := make(map[string]interface{})
	if strings.TrimSpace(query) == "" {
		return clues
	}

	queryLower := strings.ToLower(query)
	if strings.Contains(queryLower, "service visit") {
		clues["service_type"] = "service_visit"
	}
	if strings.Contains(queryLower, "calibration") {
		clues["scope"] = "calibration"
	}

	dayRe := regexp.MustCompile(`(?i)(\d+)\s*day`)
	if m := dayRe.FindStringSubmatch(query); len(m) > 1 {
		clues["visit_days"] = atoiSafe(m[1])
	}

	priceRe := regexp.MustCompile(`(?i)(?:daily price.*?around|price.*?around|around)\s*(\d+(?:\.\d+)?)\s*(?:bd|bhd)`)
	if m := priceRe.FindStringSubmatch(query); len(m) > 1 {
		clues["daily_rate_bhd"] = atofSafe(m[1])
	}

	optionalRe := regexp.MustCompile(`(?i)(?:optional.*?line item.*?)(\d+(?:\.\d+)?)\s*(?:bd|bhd)`)
	if m := optionalRe.FindStringSubmatch(query); len(m) > 1 {
		clues["optional_line_amount_bhd"] = atofSafe(m[1])
		clues["optional_line_label"] = "mobilization_of_repair_expert"
	}

	if strings.Contains(queryLower, "expert") && clues["optional_line_label"] == nil {
		clues["optional_line_label"] = "mobilization_of_repair_expert"
	}

	return clues
}

func atoiSafe(value string) int {
	var out int
	fmt.Sscanf(value, "%d", &out)
	return out
}

func atofSafe(value string) float64 {
	var out float64
	fmt.Sscanf(value, "%f", &out)
	return out
}

func (a *App) getCashflowProjectionContext() map[string]interface{} {
	result := make(map[string]interface{})
	if a.db == nil {
		return result
	}

	projection, err := a.GetCashFlowProjection(365)
	if err != nil {
		result["status"] = "unavailable"
		result["error"] = err.Error()
		return result
	}

	type monthlyBucket struct {
		Inflows  float64
		Outflows float64
	}

	buckets := make(map[string]*monthlyBucket)
	order := make([]string, 0, 12)
	for _, day := range projection.DailyProjections {
		key := day.Date.Format("2006-01")
		if buckets[key] == nil {
			buckets[key] = &monthlyBucket{}
			order = append(order, key)
		}
		buckets[key].Inflows += day.ExpectedInflows
		buckets[key].Outflows += day.ExpectedOutflows
	}

	staticOpex := 16500.0
	monthly := make([]map[string]interface{}, 0, len(order))
	for _, key := range order {
		bucket := buckets[key]
		net := bucket.Inflows - bucket.Outflows - staticOpex
		monthly = append(monthly, map[string]interface{}{
			"month":                       key,
			"payments_from_customers_bhd": round3(bucket.Inflows),
			"payments_to_suppliers_bhd":   round3(bucket.Outflows),
			"operations_cost_bhd":         round3(staticOpex),
			"net_cashflow_bhd":            round3(net),
		})
	}

	result["status"] = "available"
	result["start_date"] = projection.StartDate.Format("2006-01-02")
	result["end_date"] = projection.EndDate.Format("2006-01-02")
	result["opening_cash_bhd"] = round3(projection.OpeningCash)
	result["projected_cash_bhd"] = round3(projection.ProjectedCash)
	result["monthly_projection"] = monthly
	result["static_operations_cost_bhd"] = staticOpex
	result["basis"] = "Customer inflows from projected invoice collections, supplier outflows from due supplier invoices, plus static operating cost"
	return result
}

func round3(value float64) float64 {
	return math.Round(value*1000) / 1000
}

func (a *App) getSystemTimeContext() map[string]interface{} {
	now := time.Now()
	return map[string]interface{}{
		"current_date":    now.Format("2006-01-02"),
		"current_time":    now.Format(time.RFC3339),
		"current_year":    now.Year(),
		"current_quarter": fmt.Sprintf("Q%d", (int(now.Month())-1)/3+1),
		"timezone":        now.Location().String(),
	}
}

func buildFieldFollowUpCandidates(items []map[string]interface{}) []map[string]interface{} {
	candidates := make([]map[string]interface{}, 0, len(items))
	for idx, item := range items {
		candidate := map[string]interface{}{
			"rank":                idx + 1,
			"customer_name":       item["customer_name"],
			"site":                item["site"],
			"equipment":           item["equipment"],
			"oem":                 item["oem"],
			"estimated_value_bhd": item["estimated_value_bhd"],
			"service_due_date":    item["service_due_date"],
			"follow_up_reason":    item["due_reason"],
			"recommended_action":  "Schedule field check and propose service visit",
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func buildAmbiguitySummary(resolution *ButlerResolvedEntity) map[string]interface{} {
	if resolution == nil || !resolution.Ambiguous || len(resolution.Alternatives) == 0 {
		return map[string]interface{}{
			"needs_clarification": false,
		}
	}

	return map[string]interface{}{
		"needs_clarification": true,
		"message":             fmt.Sprintf("I found multiple %s matches for %q. Please tell me which one you mean.", resolution.EntityType, resolution.DisplayName),
		"alternatives":        resolution.Alternatives,
	}
}

func buildQuotationDraftPayload(customer map[string]interface{}, primaryContact map[string]interface{}, clues map[string]interface{}, missing []string, resolution *ButlerResolvedEntity) map[string]interface{} {
	payload := map[string]interface{}{
		"ready_to_draft": false,
		"document_type":  "Quotation",
	}

	if len(clues) > 0 {
		if scope, ok := clues["scope"]; ok {
			payload["scope"] = scope
		}
		if serviceType, ok := clues["service_type"]; ok {
			payload["service_type"] = serviceType
		}
		if visitDays, ok := clues["visit_days"]; ok {
			payload["visit_days"] = visitDays
		}
		if dailyRate, ok := clues["daily_rate_bhd"]; ok {
			payload["daily_rate_bhd"] = dailyRate
		}
		if optionalAmount, ok := clues["optional_line_amount_bhd"]; ok {
			payload["optional_line_amount_bhd"] = optionalAmount
		}
		if optionalLabel, ok := clues["optional_line_label"]; ok {
			payload["optional_line_label"] = optionalLabel
		}
	}

	if resolution != nil && !resolution.Ambiguous {
		payload["resolved_entity_type"] = resolution.EntityType
		payload["resolved_entity_name"] = resolution.DisplayName
	}

	if customer != nil {
		if name, ok := customer["name"]; ok && name != "" {
			payload["customer_name"] = name
		}
		if grade, ok := customer["grade"]; ok {
			payload["customer_grade"] = grade
		}
		if paymentTermsDays, ok := customer["payment_terms_days"]; ok {
			payload["payment_terms_days"] = paymentTermsDays
		}
	}

	if len(primaryContact) > 0 {
		payload["attention_person"] = primaryContact["name"]
		payload["attention_phone"] = primaryContact["phone"]
		payload["attention_email"] = primaryContact["email"]
	}

	lineItems := make([]map[string]interface{}, 0, 2)
	if visitDays, ok := clues["visit_days"].(int); ok && visitDays > 0 {
		if dailyRate, ok := clues["daily_rate_bhd"].(float64); ok && dailyRate > 0 {
			lineItems = append(lineItems, map[string]interface{}{
				"description":     "Service visit",
				"quantity":        visitDays,
				"unit_price_bhd":  round3(dailyRate),
				"total_price_bhd": round3(float64(visitDays) * dailyRate),
			})
		}
	}
	if optionalAmount, ok := clues["optional_line_amount_bhd"].(float64); ok && optionalAmount > 0 {
		label := "Optional line item"
		if optionalLabel, ok := clues["optional_line_label"].(string); ok && optionalLabel != "" {
			label = optionalLabel
		}
		lineItems = append(lineItems, map[string]interface{}{
			"description":     label,
			"quantity":        1,
			"unit_price_bhd":  round3(optionalAmount),
			"total_price_bhd": round3(optionalAmount),
			"optional":        true,
		})
	}
	payload["line_items"] = lineItems
	payload["missing_fields"] = missing
	payload["ready_to_draft"] = len(missing) == 0 && resolution != nil && !resolution.Ambiguous && len(lineItems) > 0

	return payload
}

// getBusinessSummary provides high-level business metrics
func (a *App) getBusinessSummary() map[string]interface{} {
	summary := make(map[string]interface{})
	now := time.Now()

	var customerCount, supplierCount, invoiceCount, orderCount int64
	a.db.Model(&CustomerMaster{}).Count(&customerCount)
	a.db.Model(&SupplierMaster{}).Count(&supplierCount)
	a.db.Model(&Invoice{}).Count(&invoiceCount)
	a.db.Model(&Order{}).Count(&orderCount)

	summary["total_customers"] = customerCount
	summary["total_suppliers"] = supplierCount
	summary["total_invoices"] = invoiceCount
	summary["total_orders"] = orderCount
	summary["as_of_date"] = now.Format("2006-01-02")
	summary["current_year"] = now.Year()

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
			"name":         c.BusinessName,
			"total_value":  c.TotalOrdersValue,
			"total_orders": c.TotalOrdersCount,
			"outstanding":  c.OutstandingBHD,
			"grade":        c.CustomerGrade,
		})
	}
	summary["top_customers_by_business"] = topBizList

	var latestInvoice Invoice
	if err := a.db.Order("invoice_date DESC").Limit(1).First(&latestInvoice).Error; err == nil {
		summary["latest_invoice_date"] = latestInvoice.InvoiceDate.Format("2006-01-02")
		summary["latest_invoice_year"] = latestInvoice.InvoiceDate.Year()
	}

	var latestOrder Order
	if err := a.db.Order("order_date DESC").Limit(1).First(&latestOrder).Error; err == nil {
		summary["latest_order_date"] = latestOrder.OrderDate.Format("2006-01-02")
		summary["latest_order_year"] = latestOrder.OrderDate.Year()
	}

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
				"id":               customer.CustomerID,
				"name":             customer.BusinessName,
				"grade":            customer.CustomerGrade,
				"payment_grade":    customer.PaymentGrade,
				"avg_payment_days": customer.AvgPaymentDays,
				"outstanding_bhd":  customer.OutstandingBHD,
				"overdue_days":     customer.OverdueDays,
				"total_orders":     customer.TotalOrdersCount,
				"total_value":      customer.TotalOrdersValue,
				"industry":         customer.Industry,
				"credit_blocked":   customer.IsCreditBlocked,
				"ar_risk_tier":     customer.ARRiskTier,
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
					"year":          year,
					"total_bhd":     fmt.Sprintf("%.3f", total),
					"invoice_count": yearlyCount[year],
					"outstanding":   fmt.Sprintf("%.3f", yearlyOutstanding[year]),
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
					"amount":      pay.AmountBHD,
					"date":        pay.PaymentDate.Format("2006-01-02"),
					"method":      pay.PaymentMethod,
					"days_to_pay": pay.DaysToPayment,
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
						"equipment":      item.Equipment,
						"model":          item.Model,
						"qty":            item.Quantity,
						"unit_price_bhd": item.UnitPrice,
						"margin_pct":     item.MarginPercent,
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

func (a *App) getCustomerPeriodSummary(entityName, query string) map[string]interface{} {
	if strings.TrimSpace(entityName) == "" || strings.TrimSpace(query) == "" {
		return nil
	}

	start, end, label, ok := parseQuarterWindowFromQuery(query)
	if !ok {
		return nil
	}

	resolution := a.resolveCustomerReference(entityName)
	if resolution == nil {
		return map[string]interface{}{
			"status":         "customer_not_resolved",
			"period":         label,
			"message":        fmt.Sprintf("Could not resolve a unique customer for %q in context.", entityName),
			"invoices":       []map[string]interface{}{},
			"invoices_count": 0,
			"orders":         []map[string]interface{}{},
			"orders_count":   0,
		}
	}

	var invoices []Invoice
	if err := a.db.Where("customer_id = ? AND invoice_date >= ? AND invoice_date <= ? AND status NOT IN ?", resolution.EntityID, start, end, []string{"Cancelled", "Void", "Proforma", "Draft"}).
		Order("invoice_date DESC").Find(&invoices).Error; err != nil {
		return map[string]interface{}{
			"status":         "error",
			"period":         label,
			"error":          err.Error(),
			"invoices":       []map[string]interface{}{},
			"invoices_count": 0,
			"orders":         []map[string]interface{}{},
			"orders_count":   0,
		}
	}

	invoiceSummary := make([]map[string]interface{}, 0, len(invoices))
	invoiceTotals := 0.0
	for _, inv := range invoices {
		invoiceTotals += inv.GrandTotalBHD
		invoiceSummary = append(invoiceSummary, map[string]interface{}{
			"number":      inv.InvoiceNumber,
			"date":        inv.InvoiceDate.Format("2006-01-02"),
			"total":       inv.GrandTotalBHD,
			"status":      inv.Status,
			"outstanding": inv.OutstandingBHD,
		})
	}

	var orders []Order
	if err := a.db.Where("customer_id = ? AND order_date >= ? AND order_date <= ?", resolution.EntityID, start, end).
		Order("order_date DESC").Find(&orders).Error; err != nil {
		return map[string]interface{}{
			"status":             "error",
			"period":             label,
			"error":              err.Error(),
			"invoices":           invoiceSummary,
			"invoices_count":     len(invoices),
			"invoices_total_bhd": invoiceTotals,
			"orders":             []map[string]interface{}{},
			"orders_count":       0,
		}
	}

	orderSummary := make([]map[string]interface{}, 0, len(orders))
	orderTotals := 0.0
	for _, order := range orders {
		orderTotals += order.GrandTotalBHD
		orderSummary = append(orderSummary, map[string]interface{}{
			"number":   order.OrderNumber,
			"date":     order.OrderDate.Format("2006-01-02"),
			"total":    order.GrandTotalBHD,
			"status":   order.Status,
			"division": order.Division,
		})
	}

	status := "available"
	if len(invoices) == 0 && len(orders) == 0 {
		status = "empty_period_window"
	}

	return map[string]interface{}{
		"status":             status,
		"period":             label,
		"invoices_count":     len(invoices),
		"invoices_total_bhd": round3(invoiceTotals),
		"invoices":           invoiceSummary,
		"orders_count":       len(orders),
		"orders_total_bhd":   round3(orderTotals),
		"orders":             orderSummary,
	}
}

func (a *App) getMissingCustomerPeriodSummary(entityName, query string) map[string]interface{} {
	periodLabel, ok := parseCustomerPeriodLabel(query)
	if !ok {
		return map[string]interface{}{
			"status": "not_requested",
		}
	}

	resolution := a.resolveCustomerReference(entityName)
	if resolution == nil {
		return map[string]interface{}{
			"status":  "customer_not_resolved",
			"period":  periodLabel,
			"message": fmt.Sprintf("Could not resolve a unique customer for %q in context.", entityName),
		}
	}

	return map[string]interface{}{
		"status":             "empty_period_window",
		"period":             periodLabel,
		"invoices_count":     0,
		"invoices_total_bhd": 0.0,
		"invoices":           []map[string]interface{}{},
		"orders_count":       0,
		"orders_total_bhd":   0.0,
		"orders":             []map[string]interface{}{},
		"message":            "No invoices or orders found in this period in the connected data.",
	}
}

func (a *App) getBusinessPeriodSummary(query string) map[string]interface{} {
	start, end, label, ok := parseQuarterWindowFromQuery(query)
	if !ok || a.db == nil {
		return nil
	}

	var invoices []Invoice
	if err := a.db.Where("invoice_date >= ? AND invoice_date <= ? AND status NOT IN ?", start, end, []string{"Cancelled", "Void", "Proforma", "Draft"}).
		Order("invoice_date DESC").Find(&invoices).Error; err != nil {
		return map[string]interface{}{
			"status": "error",
			"period": label,
			"error":  err.Error(),
		}
	}

	var orders []Order
	if err := a.db.Where("order_date >= ? AND order_date <= ?", start, end).
		Order("order_date DESC").Find(&orders).Error; err != nil {
		return map[string]interface{}{
			"status": "error",
			"period": label,
			"error":  err.Error(),
		}
	}

	var wonOpportunities []Opportunity
	if err := a.db.Where("stage = ? AND ((closed_date IS NOT NULL AND closed_date >= ? AND closed_date <= ?) OR (order_date IS NOT NULL AND order_date >= ? AND order_date <= ?))",
		"Won", start, end, start, end).
		Order("COALESCE(order_date, closed_date) DESC").
		Find(&wonOpportunities).Error; err != nil {
		return map[string]interface{}{
			"status": "error",
			"period": label,
			"error":  err.Error(),
		}
	}

	type stageSummary struct {
		Stage      string
		Count      int64
		TotalValue float64
	}
	var openStageRows []stageSummary
	a.db.Model(&Opportunity{}).
		Where("stage NOT IN ?", []string{"Won", "Lost", "Expired"}).
		Where("(expected_date BETWEEN ? AND ?) OR (offer_date BETWEEN ? AND ?) OR (created_at BETWEEN ? AND ?)",
			start, end, start, end, start, end).
		Select("stage, COUNT(*) as count, COALESCE(SUM(revenue_bhd), 0) as total_value").
		Group("stage").
		Scan(&openStageRows)

	invoiceTotal := 0.0
	invoiceSummary := make([]map[string]interface{}, 0, len(invoices))
	for _, inv := range invoices {
		invoiceTotal += inv.GrandTotalBHD
		invoiceSummary = append(invoiceSummary, map[string]interface{}{
			"number":      inv.InvoiceNumber,
			"customer":    inv.CustomerName,
			"date":        inv.InvoiceDate.Format("2006-01-02"),
			"total_bhd":   round3(inv.GrandTotalBHD),
			"status":      inv.Status,
			"outstanding": round3(inv.OutstandingBHD),
		})
	}

	orderTotal := 0.0
	orderSummary := make([]map[string]interface{}, 0, len(orders))
	for _, order := range orders {
		orderTotal += order.GrandTotalBHD
		orderSummary = append(orderSummary, map[string]interface{}{
			"number":      order.OrderNumber,
			"customer":    order.CustomerName,
			"date":        order.OrderDate.Format("2006-01-02"),
			"total_bhd":   round3(order.GrandTotalBHD),
			"status":      order.Status,
			"offer_id":    order.OfferID,
			"opportunity": order.RFQID,
		})
	}

	wonTotal := 0.0
	wonSummary := make([]map[string]interface{}, 0, len(wonOpportunities))
	for _, opp := range wonOpportunities {
		wonTotal += opp.RevenueBHD
		wonSummary = append(wonSummary, map[string]interface{}{
			"folder_number": opp.FolderNumber,
			"customer":      opp.CustomerName,
			"title":         firstNonEmpty(opp.Title, opp.FolderName),
			"closed_date":   formatOptionalDate(opp.ClosedDate),
			"order_date":    formatOptionalDate(opp.OrderDate),
			"revenue_bhd":   round3(opp.RevenueBHD),
			"offer_id":      opp.OfferID,
			"source":        opp.Source,
		})
	}

	openStageSummary := make([]map[string]interface{}, 0, len(openStageRows))
	for _, row := range openStageRows {
		openStageSummary = append(openStageSummary, map[string]interface{}{
			"stage":           row.Stage,
			"count":           row.Count,
			"total_value_bhd": round3(row.TotalValue),
		})
	}

	status := "available"
	if len(invoices) == 0 && len(orders) == 0 && len(wonOpportunities) == 0 {
		status = "empty_period_window"
	}

	return map[string]interface{}{
		"status":                      status,
		"period":                      label,
		"window_start":                start.Format("2006-01-02"),
		"window_end":                  end.Format("2006-01-02"),
		"invoices_count":              len(invoices),
		"invoices_total_bhd":          round3(invoiceTotal),
		"invoices":                    invoiceSummary,
		"orders_count":                len(orders),
		"orders_total_bhd":            round3(orderTotal),
		"orders":                      orderSummary,
		"won_opportunities_count":     len(wonOpportunities),
		"won_opportunities_total_bhd": round3(wonTotal),
		"won_opportunities":           wonSummary,
		"open_pipeline_in_period":     openStageSummary,
		"message":                     "This period summary is authoritative for company-wide quarter questions.",
	}
}

func (a *App) getBusinessYearSummary(query string) map[string]interface{} {
	start, end, year, label, ok := parseYearWindowFromQuery(query)
	if !ok || a.db == nil {
		return nil
	}

	result := map[string]interface{}{
		"status":       "available",
		"year":         year,
		"period":       label,
		"window_start": start.Format("2006-01-02"),
		"window_end":   end.Format("2006-01-02"),
	}

	var invoices []Invoice
	if err := a.db.Where("invoice_date >= ? AND invoice_date <= ? AND status NOT IN ?",
		start, end, []string{"Cancelled", "Void", "Proforma", "Draft"}).
		Order("invoice_date ASC").
		Find(&invoices).Error; err != nil {
		return map[string]interface{}{
			"status": "error",
			"year":   year,
			"period": label,
			"error":  err.Error(),
		}
	}

	invoiceTotal := 0.0
	for _, inv := range invoices {
		invoiceTotal += inv.GrandTotalBHD
	}

	var orders []Order
	if err := a.db.Where("order_date >= ? AND order_date <= ?", start, end).
		Order("order_date ASC").
		Find(&orders).Error; err != nil {
		return map[string]interface{}{
			"status": "error",
			"year":   year,
			"period": label,
			"error":  err.Error(),
		}
	}

	orderTotal := 0.0
	for _, order := range orders {
		orderTotal += order.GrandTotalBHD
	}

	var offers []Offer
	if err := a.db.Where("quotation_date >= ? AND quotation_date <= ?", start, end).
		Order("quotation_date ASC").
		Find(&offers).Error; err != nil {
		return map[string]interface{}{
			"status": "error",
			"year":   year,
			"period": label,
			"error":  err.Error(),
		}
	}

	offerTotal := 0.0
	for _, offer := range offers {
		offerTotal += offer.TotalValueBHD
	}

	var opportunities []Opportunity
	if err := a.db.Where("year = ?", year).
		Order("offer_date ASC, created_at ASC").
		Find(&opportunities).Error; err != nil {
		return map[string]interface{}{
			"status": "error",
			"year":   year,
			"period": label,
			"error":  err.Error(),
		}
	}

	opportunityTotal := 0.0
	openOpportunityCount := 0
	openOpportunityTotal := 0.0
	wonOpportunityCount := 0
	wonOpportunityTotal := 0.0
	stageRollup := make(map[string]map[string]interface{})
	for _, opp := range opportunities {
		opportunityTotal += opp.RevenueBHD
		if opp.Stage != "Won" && opp.Stage != "Lost" && opp.Stage != "Expired" {
			openOpportunityCount++
			openOpportunityTotal += opp.RevenueBHD
		}
		if opp.Stage == "Won" {
			wonOpportunityCount++
			wonOpportunityTotal += opp.RevenueBHD
		}
		row, exists := stageRollup[opp.Stage]
		if !exists {
			row = map[string]interface{}{
				"stage":           opp.Stage,
				"count":           0,
				"total_value_bhd": 0.0,
			}
			stageRollup[opp.Stage] = row
		}
		row["count"] = row["count"].(int) + 1
		row["total_value_bhd"] = round3(row["total_value_bhd"].(float64) + opp.RevenueBHD)
	}

	stageBreakdown := make([]map[string]interface{}, 0, len(stageRollup))
	for _, row := range stageRollup {
		stageBreakdown = append(stageBreakdown, row)
	}
	sort.Slice(stageBreakdown, func(i, j int) bool {
		return fmt.Sprint(stageBreakdown[i]["stage"]) < fmt.Sprint(stageBreakdown[j]["stage"])
	})

	availableDataTypes := make([]string, 0, 4)
	if len(invoices) > 0 {
		availableDataTypes = append(availableDataTypes, "invoices")
	}
	if len(orders) > 0 {
		availableDataTypes = append(availableDataTypes, "orders")
	}
	if len(offers) > 0 {
		availableDataTypes = append(availableDataTypes, "offers")
	}
	if len(opportunities) > 0 {
		availableDataTypes = append(availableDataTypes, "opportunities")
	}

	if len(availableDataTypes) == 0 {
		result["status"] = "empty_year_window"
		result["message"] = "No invoices, orders, offers, or opportunities were found for this year in the connected data."
	}

	result["available_data_types"] = availableDataTypes
	result["invoices"] = map[string]interface{}{
		"count":       len(invoices),
		"total_bhd":   round3(invoiceTotal),
		"first_date":  firstInvoiceDate(invoices),
		"latest_date": lastInvoiceDate(invoices),
	}
	result["orders"] = map[string]interface{}{
		"count":       len(orders),
		"total_bhd":   round3(orderTotal),
		"first_date":  firstOrderDate(orders),
		"latest_date": lastOrderDate(orders),
	}
	result["offers"] = map[string]interface{}{
		"count":       len(offers),
		"total_bhd":   round3(offerTotal),
		"first_date":  firstOfferDate(offers),
		"latest_date": lastOfferDate(offers),
	}
	result["opportunities"] = map[string]interface{}{
		"count":                      len(opportunities),
		"total_value_bhd":            round3(opportunityTotal),
		"open_count":                 openOpportunityCount,
		"open_pipeline_bhd":          round3(openOpportunityTotal),
		"won_count":                  wonOpportunityCount,
		"won_value_bhd":              round3(wonOpportunityTotal),
		"stage_breakdown":            stageBreakdown,
		"latest_activity_date":       latestOpportunityActivityDate(opportunities),
		"latest_recorded_offer_date": latestOpportunityOfferDate(opportunities),
	}
	result["message"] = "This year summary is authoritative for questions about whether data exists for a given year and how complete that coverage is."

	return result
}

func parseCustomerPeriodLabel(query string) (string, bool) {
	_, _, periodLabel, ok := parseQuarterWindowFromQuery(query)
	return periodLabel, ok
}

func parseYearWindowFromQuery(query string) (time.Time, time.Time, int, string, bool) {
	now := time.Now()
	q := strings.ToLower(strings.TrimSpace(query))

	year := 0
	switch {
	case strings.Contains(q, "this year") || strings.Contains(q, "current year"):
		year = now.Year()
	case strings.Contains(q, "last year") || strings.Contains(q, "previous year"):
		year = now.Year() - 1
	default:
		re := regexp.MustCompile(`\b(20\d{2})\b`)
		if match := re.FindStringSubmatch(q); len(match) > 1 {
			parsed, err := strconv.Atoi(match[1])
			if err == nil {
				year = parsed
			}
		}
	}

	if year == 0 {
		return time.Time{}, time.Time{}, 0, "", false
	}

	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(year, time.December, 31, 23, 59, 59, 0, time.Local)
	return start, end, year, fmt.Sprintf("%d", year), true
}

func firstInvoiceDate(invoices []Invoice) string {
	if len(invoices) == 0 {
		return ""
	}
	return invoices[0].InvoiceDate.Format("2006-01-02")
}

func lastInvoiceDate(invoices []Invoice) string {
	if len(invoices) == 0 {
		return ""
	}
	return invoices[len(invoices)-1].InvoiceDate.Format("2006-01-02")
}

func firstOrderDate(orders []Order) string {
	if len(orders) == 0 {
		return ""
	}
	return orders[0].OrderDate.Format("2006-01-02")
}

func lastOrderDate(orders []Order) string {
	if len(orders) == 0 {
		return ""
	}
	return orders[len(orders)-1].OrderDate.Format("2006-01-02")
}

func firstOfferDate(offers []Offer) string {
	if len(offers) == 0 {
		return ""
	}
	return offers[0].QuotationDate.Format("2006-01-02")
}

func lastOfferDate(offers []Offer) string {
	if len(offers) == 0 {
		return ""
	}
	return offers[len(offers)-1].QuotationDate.Format("2006-01-02")
}

func latestOpportunityOfferDate(opportunities []Opportunity) string {
	latest := time.Time{}
	for _, opp := range opportunities {
		if opp.OfferDate.After(latest) {
			latest = opp.OfferDate
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.Format("2006-01-02")
}

func latestOpportunityActivityDate(opportunities []Opportunity) string {
	latest := time.Time{}
	for _, opp := range opportunities {
		for _, candidate := range []time.Time{opp.OfferDate, opp.CreatedAt, opp.UpdatedAt} {
			if candidate.After(latest) {
				latest = candidate
			}
		}
		if opp.ExpectedDate != nil && opp.ExpectedDate.After(latest) {
			latest = *opp.ExpectedDate
		}
		if opp.ClosedDate != nil && opp.ClosedDate.After(latest) {
			latest = *opp.ClosedDate
		}
		if opp.OrderDate != nil && opp.OrderDate.After(latest) {
			latest = *opp.OrderDate
		}
	}
	if latest.IsZero() {
		return ""
	}
	return latest.Format("2006-01-02")
}

func parseQuarterWindowFromQuery(query string) (time.Time, time.Time, string, bool) {
	now := time.Now()
	q := strings.ToLower(strings.TrimSpace(query))

	if strings.Contains(q, "last 2 quarter") || strings.Contains(q, "last two quarter") {
		month := int(now.Month())
		currentQuarter := (month-1)/3 + 1
		endQuarter := currentQuarter - 1
		endYear := now.Year()
		if endQuarter == 0 {
			endQuarter = 4
			endYear--
		}

		startQuarter := endQuarter - 1
		startYear := endYear
		if startQuarter == 0 {
			startQuarter = 4
			startYear--
		}

		startMonth := time.Month((startQuarter-1)*3 + 1)
		start := time.Date(startYear, startMonth, 1, 0, 0, 0, 0, time.Local)

		endStartMonth := time.Month((endQuarter-1)*3 + 1)
		endStart := time.Date(endYear, endStartMonth, 1, 0, 0, 0, 0, time.Local)
		end := endStart.AddDate(0, 3, 0).Add(-time.Second)

		label := fmt.Sprintf("last 2 quarters (Q%d %d to Q%d %d)", startQuarter, startYear, endQuarter, endYear)
		return start, end, label, true
	}

	if strings.Contains(q, "last quarter") || strings.Contains(q, "previous quarter") {
		month := int(now.Month())
		currentQuarter := (month-1)/3 + 1
		previousQuarter := currentQuarter - 1
		year := now.Year()
		if previousQuarter == 0 {
			previousQuarter = 4
			year--
		}
		startMonth := time.Month((previousQuarter-1)*3 + 1)
		start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 3, 0).Add(-time.Second)
		label := fmt.Sprintf("Q%d %d", previousQuarter, year)
		return start, end, label, true
	}

	if strings.Contains(q, "this quarter") || strings.Contains(q, "current quarter") {
		month := int(now.Month())
		currentQuarter := (month-1)/3 + 1
		year := now.Year()
		startMonth := time.Month((currentQuarter-1)*3 + 1)
		start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 3, 0).Add(-time.Second)
		label := fmt.Sprintf("Q%d %d", currentQuarter, year)
		return start, end, label, true
	}

	re := regexp.MustCompile(`(?i)\bq([1-4])(?:\s+)?(?:\(?(\d{4})\)?)?`)
	m := re.FindStringSubmatch(q)
	if len(m) == 3 {
		var quarter int
		fmt.Sscanf(m[1], "%d", &quarter)
		year := now.Year()
		if m[2] != "" {
			fmt.Sscanf(m[2], "%d", &year)
		}
		startMonth := time.Month((quarter-1)*3 + 1)
		start := time.Date(year, startMonth, 1, 0, 0, 0, 0, time.Local)
		end := start.AddDate(0, 3, 0).Add(-time.Second)
		label := fmt.Sprintf("Q%d %d", quarter, year)
		return start, end, label, true
	}

	return time.Time{}, time.Time{}, "", false
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
				"id":             supplier.ID,
				"name":           supplier.SupplierName,
				"type":           supplier.SupplierType,
				"country":        supplier.Country,
				"rating":         supplier.Rating,
				"lead_time_days": supplier.LeadTimeDays,
				"brands":         supplier.BrandsHandled,
				"contact":        supplier.PrimaryContact,
				"email":          supplier.Email,
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
					"year":      year,
					"total_bhd": fmt.Sprintf("%.3f", total),
					"po_count":  yearlyPOCount[year],
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
					"total_grns": grnCount,
					"passed":     grnPassedCount,
					"pass_rate":  fmt.Sprintf("%.1f%%", float64(grnPassedCount)/float64(grnCount)*100),
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
	now := time.Now()
	result["as_of_date"] = now.Format("2006-01-02")
	result["current_year"] = now.Year()

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
			"invoice": p.InvoiceNumber,
			"amount":  p.AmountBHD,
			"date":    p.PaymentDate.Format("2006-01-02"),
			"method":  p.PaymentMethod,
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

	var latestInvoice Invoice
	if err := a.db.Order("invoice_date DESC").Limit(1).First(&latestInvoice).Error; err == nil {
		result["latest_invoice_date"] = latestInvoice.InvoiceDate.Format("2006-01-02")
		result["latest_invoice_year"] = latestInvoice.InvoiceDate.Year()
	}

	var latestPayment Payment
	if err := a.db.Order("payment_date DESC").Limit(1).First(&latestPayment).Error; err == nil {
		result["latest_payment_date"] = latestPayment.PaymentDate.Format("2006-01-02")
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
				"equipment":      item.Equipment,
				"model":          item.Model,
				"qty":            item.Quantity,
				"unit_price_bhd": item.UnitPrice,
				"margin_pct":     item.MarginPercent,
				"currency":       item.Currency,
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
		"total_items":    totalGRNItems,
		"total_rejected": rejectedItems,
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
			"offer":    offerNum,
			"customer": offer.CustomerName,
			"date":     fu.FollowUpDate.Format("2006-01-02"),
			"notes":    fu.Notes,
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
			"equipment":   tp.Equipment,
			"model":       tp.Model,
			"order_count": tp.Count,
			"total_qty":   tp.TotalQty,
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
	now := time.Now()
	result["as_of_date"] = now.Format("2006-01-02")
	result["current_year"] = now.Year()

	var allOpportunities []Opportunity
	a.db.Find(&allOpportunities)

	closedKeys := make(map[string]bool)
	for _, opp := range allOpportunities {
		normalized := normalizeOpportunityForList(opp)
		if normalized.Stage != "Won" && normalized.Stage != "Lost" {
			continue
		}
		key := canonicalOpportunityKey(normalized)
		if strings.TrimSpace(key) != "" {
			closedKeys[key] = true
		}
	}

	openByKey := make(map[string]Opportunity)
	for _, opp := range allOpportunities {
		normalized := normalizeOpportunityForList(opp)
		if shouldSuppressSyntheticOCR(normalized) {
			continue
		}
		if normalized.Stage == "Won" || normalized.Stage == "Lost" || normalized.Stage == "Expired" {
			continue
		}
		key := canonicalOpportunityKey(normalized)
		if key != "" && closedKeys[key] {
			continue
		}
		if key == "" {
			key = normalized.ID
		}
		existing, exists := openByKey[key]
		if !exists || shouldPreferOpportunity(normalized, existing) {
			openByKey[key] = normalized
		}
	}

	openOpportunities := make([]Opportunity, 0, len(openByKey))
	for _, opp := range openByKey {
		openOpportunities = append(openOpportunities, opp)
	}

	// --- Pipeline by stage with weighted value ---
	type stageData struct {
		Count      int64
		TotalValue float64
	}
	stageBreakdown := make(map[string]*stageData)
	for _, opp := range openOpportunities {
		stage := strings.TrimSpace(opp.Stage)
		if stage == "" {
			stage = "New"
		}
		if stageBreakdown[stage] == nil {
			stageBreakdown[stage] = &stageData{}
		}
		stageBreakdown[stage].Count++
		stageBreakdown[stage].TotalValue += opp.RevenueBHD
	}
	pipelineStages := make([]map[string]interface{}, 0)
	var weightedPipeline float64
	stageWeights := map[string]float64{
		"Lead": 0.10, "Qualified": 0.25, "Proposal": 0.50, "Negotiation": 0.75,
	}
	for stage, s := range stageBreakdown {
		w := stageWeights[stage]
		if w == 0 {
			w = 0.50
		}
		weighted := s.TotalValue * w
		weightedPipeline += weighted
		pipelineStages = append(pipelineStages, map[string]interface{}{
			"stage":             stage,
			"count":             s.Count,
			"total_value_bhd":   fmt.Sprintf("%.3f", s.TotalValue),
			"weighted_bhd":      fmt.Sprintf("%.3f", weighted),
			"close_probability": fmt.Sprintf("%.0f%%", w*100),
		})
	}
	result["pipeline_by_stage"] = pipelineStages
	result["weighted_pipeline_bhd"] = fmt.Sprintf("%.3f", weightedPipeline)

	// --- Opportunities closing soon (next 60 days) ---
	closingSoon := make([]Opportunity, 0)
	for _, opp := range openOpportunities {
		if opp.ExpectedDate == nil {
			continue
		}
		if opp.ExpectedDate.Before(now) || opp.ExpectedDate.After(now.AddDate(0, 0, 60)) {
			continue
		}
		closingSoon = append(closingSoon, opp)
	}
	sort.Slice(closingSoon, func(i, j int) bool {
		return closingSoon[i].ExpectedDate.Before(*closingSoon[j].ExpectedDate)
	})
	if len(closingSoon) > 10 {
		closingSoon = closingSoon[:10]
	}
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
	perfMap := make(map[string]*salespersonPerf)
	for _, opp := range openOpportunities {
		if strings.TrimSpace(opp.Salesperson) == "" {
			continue
		}
		if perfMap[opp.Salesperson] == nil {
			perfMap[opp.Salesperson] = &salespersonPerf{Salesperson: opp.Salesperson}
		}
		perfMap[opp.Salesperson].OpenCount++
		perfMap[opp.Salesperson].TotalValue += opp.RevenueBHD
	}
	for _, perf := range perfMap {
		salesPerf = append(salesPerf, *perf)
	}
	sort.Slice(salesPerf, func(i, j int) bool {
		return salesPerf[i].TotalValue > salesPerf[j].TotalValue
	})
	salesList := make([]map[string]interface{}, 0)
	for _, sp := range salesPerf {
		salesList = append(salesList, map[string]interface{}{
			"salesperson":  sp.Salesperson,
			"open_opps":    sp.OpenCount,
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

// getDSOContext computes Days Sales Outstanding — overall and per-customer.
// Uses payments.days_to_payment which is pre-calculated at payment recording time.
func (a *App) getDSOContext() map[string]interface{} {
	ctx := make(map[string]interface{})
	if a.db == nil {
		return ctx
	}

	// Overall DSO
	var overallDSO float64
	a.db.Model(&Payment{}).
		Select("COALESCE(AVG(days_to_payment), 0)").
		Where("days_to_payment > 0").
		Scan(&overallDSO)
	ctx["overall_dso_days"] = math.Round(overallDSO*10) / 10

	// Per-customer DSO — slowest 10 and fastest 10
	type CustomerDSO struct {
		CustomerName string  `gorm:"column:customer_name"`
		AvgDays      float64 `gorm:"column:avg_days"`
		PaymentCount int     `gorm:"column:payment_count"`
	}
	var customerDSOs []CustomerDSO
	a.db.Raw(`
		SELECT i.customer_name,
		       ROUND(AVG(p.days_to_payment), 1) as avg_days,
		       COUNT(p.id) as payment_count
		FROM payments p
		JOIN invoices i ON p.invoice_id = i.id
		WHERE p.days_to_payment > 0 AND i.deleted_at IS NULL
		GROUP BY i.customer_id, i.customer_name
		HAVING payment_count >= 2
		ORDER BY avg_days DESC
		LIMIT 20
	`).Scan(&customerDSOs)

	slowest, fastest := []map[string]interface{}{}, []map[string]interface{}{}
	for _, c := range customerDSOs {
		if len(slowest) < 10 {
			slowest = append(slowest, map[string]interface{}{
				"customer": c.CustomerName, "avg_days": c.AvgDays, "payments": c.PaymentCount,
			})
		}
	}
	for i := len(customerDSOs) - 1; i >= 0 && len(fastest) < 10; i-- {
		c := customerDSOs[i]
		fastest = append(fastest, map[string]interface{}{
			"customer": c.CustomerName, "avg_days": c.AvgDays, "payments": c.PaymentCount,
		})
	}
	ctx["slowest_payers"] = slowest
	ctx["fastest_payers"] = fastest

	return ctx
}

// getOfferExpiryContext fetches active offers approaching their ValidityDate.
// Groups by urgency: this week / this month / next 60 days.
func (a *App) getOfferExpiryContext() map[string]interface{} {
	ctx := make(map[string]interface{})
	if a.db == nil {
		return ctx
	}

	now := time.Now()
	in7 := now.AddDate(0, 0, 7)
	in30 := now.AddDate(0, 0, 30)
	in60 := now.AddDate(0, 0, 60)

	type ExpiringOffer struct {
		OfferNumber   string    `gorm:"column:offer_number"`
		CustomerName  string    `gorm:"column:customer_name"`
		TotalValueBHD float64   `gorm:"column:total_value_bhd"`
		ValidityDate  time.Time `gorm:"column:validity_date"`
		Stage         string    `gorm:"column:stage"`
	}
	var expiring []ExpiringOffer
	a.db.Raw(`
		SELECT offer_number, customer_name, total_value_bhd, validity_date, stage
		FROM offers
		WHERE validity_date BETWEEN ? AND ?
		  AND stage IN ('RFQ', 'Quoted')
		  AND deleted_at IS NULL
		ORDER BY validity_date ASC
		LIMIT 50
	`, now, in60).Scan(&expiring)

	week, month, later := []map[string]interface{}{}, []map[string]interface{}{}, []map[string]interface{}{}
	var totalAtRisk float64
	for _, o := range expiring {
		entry := map[string]interface{}{
			"offer": o.OfferNumber, "customer": o.CustomerName,
			"value_bhd": o.TotalValueBHD, "expires": o.ValidityDate.Format("2006-01-02"), "stage": o.Stage,
		}
		totalAtRisk += o.TotalValueBHD
		if o.ValidityDate.Before(in7) {
			week = append(week, entry)
		} else if o.ValidityDate.Before(in30) {
			month = append(month, entry)
		} else {
			later = append(later, entry)
		}
	}
	ctx["expiring_this_week"] = week
	ctx["expiring_this_month"] = month
	ctx["expiring_30_to_60_days"] = later
	ctx["total_value_at_risk_bhd"] = math.Round(totalAtRisk*1000) / 1000
	ctx["count_expiring_60_days"] = len(expiring)

	return ctx
}

// getCustomerProfitabilityContext ranks customers by revenue and margin.
func (a *App) getCustomerProfitabilityContext() map[string]interface{} {
	ctx := make(map[string]interface{})
	if a.db == nil {
		return ctx
	}

	type CustomerProfit struct {
		CustomerName    string  `gorm:"column:customer_name"`
		TotalRevenueBHD float64 `gorm:"column:total_revenue"`
		TotalMarginBHD  float64 `gorm:"column:total_margin"`
		AvgMarginPct    float64 `gorm:"column:avg_margin_pct"`
		InvoiceCount    int     `gorm:"column:invoice_count"`
	}

	var topRevenue []CustomerProfit
	a.db.Raw(`
		SELECT customer_name,
		       SUM(grand_total_bhd) as total_revenue,
		       SUM(gross_margin_bhd) as total_margin,
		       ROUND(AVG(gross_margin_percent), 2) as avg_margin_pct,
		       COUNT(*) as invoice_count
		FROM invoices
		WHERE status NOT IN ('Cancelled', 'Void', 'Proforma', 'Draft')
		  AND deleted_at IS NULL
		GROUP BY customer_id, customer_name
		ORDER BY total_revenue DESC
		LIMIT 15
	`).Scan(&topRevenue)

	revenueResult := []map[string]interface{}{}
	for _, p := range topRevenue {
		revenueResult = append(revenueResult, map[string]interface{}{
			"customer": p.CustomerName, "revenue_bhd": p.TotalRevenueBHD,
			"margin_bhd": p.TotalMarginBHD, "avg_margin_pct": p.AvgMarginPct,
			"invoice_count": p.InvoiceCount,
		})
	}
	ctx["top_customers_by_revenue"] = revenueResult

	// Lowest margin customers (pricing opportunities)
	var lowMargin []CustomerProfit
	a.db.Raw(`
		SELECT customer_name,
		       SUM(grand_total_bhd) as total_revenue,
		       SUM(gross_margin_bhd) as total_margin,
		       ROUND(AVG(gross_margin_percent), 2) as avg_margin_pct,
		       COUNT(*) as invoice_count
		FROM invoices
		WHERE status NOT IN ('Cancelled', 'Void', 'Proforma', 'Draft')
		  AND deleted_at IS NULL AND gross_margin_percent > 0
		GROUP BY customer_id, customer_name
		HAVING invoice_count >= 2
		ORDER BY avg_margin_pct ASC
		LIMIT 10
	`).Scan(&lowMargin)

	lowResult := []map[string]interface{}{}
	for _, p := range lowMargin {
		lowResult = append(lowResult, map[string]interface{}{
			"customer": p.CustomerName, "revenue_bhd": p.TotalRevenueBHD,
			"avg_margin_pct": p.AvgMarginPct, "invoice_count": p.InvoiceCount,
		})
	}
	ctx["lowest_margin_customers"] = lowResult

	return ctx
}

// getSupplierPerformanceContext calculates delivery lead times and QC pass rates per supplier.
func (a *App) getSupplierPerformanceContext() map[string]interface{} {
	ctx := make(map[string]interface{})
	if a.db == nil {
		return ctx
	}

	type SupplierPerf struct {
		SupplierName string  `gorm:"column:supplier_name"`
		AvgLeadDays  float64 `gorm:"column:avg_lead_days"`
		GRNCount     int     `gorm:"column:grn_count"`
		QCPassRate   float64 `gorm:"column:qc_pass_rate"`
	}
	var perfs []SupplierPerf
	a.db.Raw(`
		SELECT po.supplier_name,
		       ROUND(AVG(CAST((julianday(grn.received_date) - julianday(po.po_date)) AS REAL)), 1) as avg_lead_days,
		       COUNT(grn.id) as grn_count,
		       ROUND(SUM(CASE WHEN grn.qc_status = 'Passed' THEN 1 ELSE 0 END) * 100.0 / COUNT(grn.id), 1) as qc_pass_rate
		FROM goods_received_notes grn
		JOIN purchase_orders po ON grn.purchase_order_id = po.id
		WHERE po.deleted_at IS NULL AND grn.deleted_at IS NULL
		  AND grn.received_date IS NOT NULL AND po.po_date IS NOT NULL
		GROUP BY po.supplier_id, po.supplier_name
		HAVING grn_count >= 1
		ORDER BY avg_lead_days ASC
	`).Scan(&perfs)

	result := []map[string]interface{}{}
	for _, p := range perfs {
		result = append(result, map[string]interface{}{
			"supplier": p.SupplierName, "avg_lead_days": p.AvgLeadDays,
			"deliveries": p.GRNCount, "qc_pass_rate_pct": p.QCPassRate,
		})
	}
	ctx["supplier_delivery_performance"] = result

	return ctx
}

// getCustomerActivityContext identifies dormant customers, new customers this year,
// and customers who ordered last year but not this year.
func (a *App) getCustomerActivityContext() map[string]interface{} {
	ctx := make(map[string]interface{})
	if a.db == nil {
		return ctx
	}

	now := time.Now()
	sixMonthsAgo := now.AddDate(0, -6, 0)
	oneYearAgo := now.AddDate(-1, 0, 0)
	thisYearStart := time.Date(now.Year(), 1, 1, 0, 0, 0, 0, time.UTC)

	type CustomerActivity struct {
		CustomerName    string    `gorm:"column:customer_name"`
		LastOrderDate   time.Time `gorm:"column:last_order_date"`
		TotalOrders     int       `gorm:"column:total_orders"`
		TotalRevenueBHD float64   `gorm:"column:total_revenue"`
	}

	// Dormant: had invoices historically but nothing in last 6 months
	var dormant []CustomerActivity
	a.db.Raw(`
		SELECT customer_name,
		       MAX(invoice_date) as last_order_date,
		       COUNT(*) as total_orders,
		       SUM(grand_total_bhd) as total_revenue
		FROM invoices
		WHERE status NOT IN ('Cancelled', 'Void', 'Proforma', 'Draft') AND deleted_at IS NULL
		GROUP BY customer_id, customer_name
		HAVING last_order_date < ?
		ORDER BY total_revenue DESC
		LIMIT 15
	`, sixMonthsAgo).Scan(&dormant)

	dormantResult := []map[string]interface{}{}
	for _, c := range dormant {
		dormantResult = append(dormantResult, map[string]interface{}{
			"customer": c.CustomerName, "last_order": c.LastOrderDate.Format("2006-01-02"),
			"days_silent":            int(now.Sub(c.LastOrderDate).Hours() / 24),
			"historical_revenue_bhd": c.TotalRevenueBHD,
		})
	}
	ctx["dormant_customers"] = dormantResult

	// New customers who placed their first-ever order this year
	type NewCust struct {
		CustomerName string    `gorm:"column:customer_name"`
		FirstOrder   time.Time `gorm:"column:first_order"`
		OrderCount   int       `gorm:"column:order_count"`
		RevenueBHD   float64   `gorm:"column:revenue_bhd"`
	}
	var newCustomers []NewCust
	a.db.Raw(`
		SELECT customer_name,
		       MIN(invoice_date) as first_order,
		       COUNT(*) as order_count,
		       SUM(grand_total_bhd) as revenue_bhd
		FROM invoices
		WHERE status NOT IN ('Cancelled', 'Void', 'Proforma', 'Draft') AND deleted_at IS NULL
		GROUP BY customer_id, customer_name
		HAVING first_order >= ?
		ORDER BY revenue_bhd DESC
		LIMIT 10
	`, thisYearStart).Scan(&newCustomers)

	newResult := []map[string]interface{}{}
	for _, c := range newCustomers {
		newResult = append(newResult, map[string]interface{}{
			"customer": c.CustomerName, "first_order": c.FirstOrder.Format("2006-01-02"),
			"orders": c.OrderCount, "revenue_bhd": c.RevenueBHD,
		})
	}
	ctx["new_customers_this_year"] = newResult

	// Lapsed: ordered last year but not placed anything yet this year
	var lapsed []CustomerActivity
	a.db.Raw(`
		SELECT customer_name,
		       MAX(invoice_date) as last_order_date,
		       COUNT(*) as total_orders,
		       SUM(grand_total_bhd) as total_revenue
		FROM invoices
		WHERE invoice_date BETWEEN ? AND ?
		  AND status NOT IN ('Cancelled', 'Void', 'Proforma', 'Draft') AND deleted_at IS NULL
		  AND customer_id NOT IN (
		      SELECT DISTINCT customer_id FROM invoices
		      WHERE invoice_date >= ?
		        AND status NOT IN ('Cancelled', 'Void', 'Proforma', 'Draft')
		        AND deleted_at IS NULL
		  )
		GROUP BY customer_id, customer_name
		ORDER BY total_revenue DESC
		LIMIT 10
	`, oneYearAgo, thisYearStart, thisYearStart).Scan(&lapsed)

	lapsedResult := []map[string]interface{}{}
	for _, c := range lapsed {
		lapsedResult = append(lapsedResult, map[string]interface{}{
			"customer":              c.CustomerName,
			"last_order":            c.LastOrderDate.Format("2006-01-02"),
			"prev_year_revenue_bhd": c.TotalRevenueBHD,
		})
	}
	ctx["lapsed_this_year"] = lapsedResult

	return ctx
}

// getActionItemsContext builds a prioritized list of items requiring attention right now.
func (a *App) getActionItemsContext() map[string]interface{} {
	ctx := make(map[string]interface{})
	if a.db == nil {
		return ctx
	}

	now := time.Now()
	in7Days := now.AddDate(0, 0, 7)
	sevenDaysAgo := now.AddDate(0, 0, -7)
	fourteenDaysAgo := now.AddDate(0, 0, -14)

	// 1. Overdue invoices to chase (top 10 by amount)
	type OverdueInv struct {
		CustomerName   string    `gorm:"column:customer_name"`
		InvoiceNumber  string    `gorm:"column:invoice_number"`
		OutstandingBHD float64   `gorm:"column:outstanding_bhd"`
		DueDate        time.Time `gorm:"column:due_date"`
	}
	var overdueInvoices []OverdueInv
	a.db.Raw(`
		SELECT customer_name, invoice_number, outstanding_bhd, due_date
		FROM invoices
		WHERE status IN ('Overdue', 'Sent', 'PartiallyPaid')
		  AND due_date < ? AND outstanding_bhd > 0 AND deleted_at IS NULL
		ORDER BY outstanding_bhd DESC LIMIT 10
	`, now).Scan(&overdueInvoices)

	overdueResult := []map[string]interface{}{}
	for _, inv := range overdueInvoices {
		overdueResult = append(overdueResult, map[string]interface{}{
			"customer": inv.CustomerName, "invoice": inv.InvoiceNumber,
			"outstanding_bhd": inv.OutstandingBHD,
			"days_overdue":    int(now.Sub(inv.DueDate).Hours() / 24),
		})
	}
	ctx["overdue_invoices_to_chase"] = overdueResult

	// 2. Offers expiring this week
	type ExpiryItem struct {
		OfferNumber   string    `gorm:"column:offer_number"`
		CustomerName  string    `gorm:"column:customer_name"`
		TotalValueBHD float64   `gorm:"column:total_value_bhd"`
		ValidityDate  time.Time `gorm:"column:validity_date"`
	}
	var expiringOffers []ExpiryItem
	a.db.Raw(`
		SELECT offer_number, customer_name, total_value_bhd, validity_date
		FROM offers
		WHERE validity_date BETWEEN ? AND ? AND stage IN ('RFQ', 'Quoted') AND deleted_at IS NULL
		ORDER BY validity_date ASC LIMIT 10
	`, now, in7Days).Scan(&expiringOffers)

	expiryResult := []map[string]interface{}{}
	for _, o := range expiringOffers {
		expiryResult = append(expiryResult, map[string]interface{}{
			"offer": o.OfferNumber, "customer": o.CustomerName,
			"value_bhd": o.TotalValueBHD, "expires": o.ValidityDate.Format("2006-01-02"),
		})
	}
	ctx["offers_expiring_this_week"] = expiryResult

	// 3. Delivery notes stuck as Prepared for >7 days
	type StuckDN struct {
		DNNumber  string    `gorm:"column:dn_number"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	var stuckDNs []StuckDN
	a.db.Raw(`
		SELECT dn_number, created_at FROM delivery_notes
		WHERE status = 'Prepared' AND created_at < ? AND deleted_at IS NULL
		ORDER BY created_at ASC LIMIT 10
	`, sevenDaysAgo).Scan(&stuckDNs)

	stuckResult := []map[string]interface{}{}
	for _, dn := range stuckDNs {
		stuckResult = append(stuckResult, map[string]interface{}{
			"dn": dn.DNNumber, "days_pending": int(now.Sub(dn.CreatedAt).Hours() / 24),
		})
	}
	ctx["stuck_delivery_notes"] = stuckResult

	// 4. POs sent but not acknowledged for >14 days
	type SentPO struct {
		PONumber     string    `gorm:"column:po_number"`
		SupplierName string    `gorm:"column:supplier_name"`
		TotalBHD     float64   `gorm:"column:total_bhd"`
		UpdatedAt    time.Time `gorm:"column:updated_at"`
	}
	var sentPOs []SentPO
	a.db.Raw(`
		SELECT po_number, supplier_name, total_bhd, updated_at
		FROM purchase_orders
		WHERE status = 'Sent' AND updated_at < ? AND deleted_at IS NULL
		ORDER BY total_bhd DESC LIMIT 10
	`, fourteenDaysAgo).Scan(&sentPOs)

	sentPOResult := []map[string]interface{}{}
	for _, po := range sentPOs {
		sentPOResult = append(sentPOResult, map[string]interface{}{
			"po": po.PONumber, "supplier": po.SupplierName,
			"value_bhd": po.TotalBHD, "days_waiting": int(now.Sub(po.UpdatedAt).Hours() / 24),
		})
	}
	ctx["pos_awaiting_acknowledgment"] = sentPOResult

	// 5. Supplier invoices pending approval for >7 days
	type PendingSupInv struct {
		InvoiceNumber string    `gorm:"column:invoice_number"`
		SupplierName  string    `gorm:"column:supplier_name"`
		TotalBHD      float64   `gorm:"column:total_bhd"`
		InvoiceDate   time.Time `gorm:"column:invoice_date"`
	}
	var pendingSI []PendingSupInv
	a.db.Raw(`
		SELECT invoice_number, supplier_name, total_bhd, invoice_date
		FROM supplier_invoices
		WHERE status = 'Pending' AND invoice_date < ? AND deleted_at IS NULL
		ORDER BY total_bhd DESC LIMIT 10
	`, sevenDaysAgo).Scan(&pendingSI)

	pendingSIResult := []map[string]interface{}{}
	for _, si := range pendingSI {
		pendingSIResult = append(pendingSIResult, map[string]interface{}{
			"invoice": si.InvoiceNumber, "supplier": si.SupplierName,
			"value_bhd": si.TotalBHD, "days_pending": int(now.Sub(si.InvoiceDate).Hours() / 24),
		})
	}
	ctx["supplier_invoices_pending_approval"] = pendingSIResult

	return ctx
}

// getCompetitionContext analyzes win/loss patterns, ABB competition, and lost deal reasons.
func (a *App) getCompetitionContext() map[string]interface{} {
	ctx := make(map[string]interface{})
	if a.db == nil {
		return ctx
	}

	var totalOffers, abbOffers, abbWins, totalWins int64
	a.db.Model(&Offer{}).Where("deleted_at IS NULL").Count(&totalOffers)
	a.db.Model(&Offer{}).Where("has_abb_competition = ? AND deleted_at IS NULL", true).Count(&abbOffers)
	a.db.Model(&Offer{}).Where("has_abb_competition = ? AND stage = ? AND deleted_at IS NULL", true, "Won").Count(&abbWins)
	a.db.Model(&Offer{}).Where("stage = ? AND deleted_at IS NULL", "Won").Count(&totalWins)

	var abbWinRate, overallWinRate float64
	if abbOffers > 0 {
		abbWinRate = math.Round(float64(abbWins)/float64(abbOffers)*1000) / 10
	}
	if totalOffers > 0 {
		overallWinRate = math.Round(float64(totalWins)/float64(totalOffers)*1000) / 10
	}
	ctx["total_offers"] = totalOffers
	ctx["abb_competitive_offers"] = abbOffers
	ctx["win_rate_vs_abb_pct"] = abbWinRate
	ctx["overall_win_rate_pct"] = overallWinRate

	// Top lost reasons
	type LostReason struct {
		Reason string `gorm:"column:lost_reason"`
		Count  int    `gorm:"column:cnt"`
	}
	var lostReasons []LostReason
	a.db.Raw(`
		SELECT lost_reason, COUNT(*) as cnt
		FROM offers
		WHERE stage = 'Lost' AND lost_reason != '' AND lost_reason IS NOT NULL AND deleted_at IS NULL
		GROUP BY lost_reason ORDER BY cnt DESC LIMIT 10
	`).Scan(&lostReasons)

	reasonResult := []map[string]interface{}{}
	for _, r := range lostReasons {
		reasonResult = append(reasonResult, map[string]interface{}{"reason": r.Reason, "count": r.Count})
	}
	ctx["top_lost_reasons"] = reasonResult

	// Win rate by product code (requires joining offer_items → offers)
	type ProductWinRate struct {
		ProductCode string `gorm:"column:product_code"`
		TotalOffers int    `gorm:"column:total_offers"`
		WonOffers   int    `gorm:"column:won_offers"`
	}
	var productWinRates []ProductWinRate
	a.db.Raw(`
		SELECT oi.product_code,
		       COUNT(DISTINCT o.id) as total_offers,
		       SUM(CASE WHEN o.stage = 'Won' THEN 1 ELSE 0 END) as won_offers
		FROM offer_items oi
		JOIN offers o ON oi.offer_id = o.id
		WHERE o.deleted_at IS NULL AND oi.deleted_at IS NULL AND oi.product_code != ''
		GROUP BY oi.product_code HAVING total_offers >= 3
		ORDER BY won_offers * 1.0 / total_offers DESC LIMIT 10
	`).Scan(&productWinRates)

	productResult := []map[string]interface{}{}
	for _, p := range productWinRates {
		winRate := 0.0
		if p.TotalOffers > 0 {
			winRate = math.Round(float64(p.WonOffers)/float64(p.TotalOffers)*1000) / 10
		}
		productResult = append(productResult, map[string]interface{}{
			"product": p.ProductCode, "win_rate_pct": winRate,
			"total_offers": p.TotalOffers, "won": p.WonOffers,
		})
	}
	ctx["win_rate_by_product"] = productResult

	return ctx
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
var settingsBasedAIMLModel func() string

// SetAIMLKeyProvider allows the App to inject the settings-based AIML key provider
func SetAIMLKeyProvider(provider func() string) {
	settingsBasedAIMLKey = provider
}

// SetAIMLModelProvider allows the App to inject the settings-based AIML model provider.
func SetAIMLModelProvider(provider func() string) {
	settingsBasedAIMLModel = provider
}

// getAIMLAPIKey retrieves the AIML API key from settings or environment.
// Priority: settings DB → ASYMM/AIML env var.
func getAIMLAPIKey() string {
	// 1. Try settings-based provider (injected by App)
	if settingsBasedAIMLKey != nil {
		if key := strings.TrimSpace(settingsBasedAIMLKey()); key != "" {
			return key
		}
	}
	// 2. Try environment variables
	if key := strings.TrimSpace(os.Getenv("ASYMM_AIML_API_KEY")); key != "" {
		return key
	}
	if key := strings.TrimSpace(os.Getenv("AIML_API_KEY")); key != "" {
		return key
	}
	return ""
}

func getAIMLModelID() string {
	if settingsBasedAIMLModel != nil {
		if model := strings.TrimSpace(settingsBasedAIMLModel()); model != "" {
			return model
		}
	}
	if model := strings.TrimSpace(os.Getenv("ASYMM_AIML_MODEL")); model != "" {
		return model
	}
	if model := strings.TrimSpace(os.Getenv("AIML_MODEL")); model != "" {
		return model
	}
	return aimlModelID
}

func getAIMLModelCandidates() []string {
	requested := getAIMLModelID()
	models := []string{}
	if requested != "" {
		models = append(models, requested)
	}
	models = append(models, fallbackAIMLModels...)
	return dedupeStringSlice(models)
}

func dedupeStringSlice(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

// callAIML sends a chat request to the AIML API (OpenAI-compatible format).
// Uses explicit model selection to support runtime fallback orchestration.
func callAIML(apiKey, systemPrompt, userMessage, model string) (string, error) {
	requestBody := map[string]interface{}{
		"model": model,
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

func callAIMLWithFallback(apiKey, systemPrompt, userMessage string) (string, string, error) {
	var lastErr error
	for _, model := range getAIMLModelCandidates() {
		response, err := callAIML(apiKey, systemPrompt, userMessage, model)
		if err == nil {
			return response, model, nil
		}
		lastErr = err
		log.Printf("⚠️ AIML model attempt failed (%s): %v", model, err)
	}
	return "", "", lastErr
}

// callAIMLWithMessages sends the full messages array to AIML API (preserves conversation history).
func callAIMLWithMessages(apiKey string, messages []map[string]interface{}) (string, string, error) {
	var lastErr error
	for _, model := range getAIMLModelCandidates() {
		requestBody := map[string]interface{}{
			"model":       model,
			"messages":    messages,
			"max_tokens":  aimlMaxTokens,
			"temperature": aimlTemperature,
		}
		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			lastErr = fmt.Errorf("marshal error: %w", err)
			continue
		}
		req, err := http.NewRequest("POST", aimlAPIURL, bytes.NewBuffer(jsonData))
		if err != nil {
			lastErr = fmt.Errorf("request creation error: %w", err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		client := &http.Client{Timeout: AimlAPITimeout}
		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			log.Printf("⚠️ AIML model attempt failed (%s): %v", model, lastErr)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			lastErr = fmt.Errorf("AIML API error (status %d): %s", resp.StatusCode, string(body))
			log.Printf("⚠️ AIML model attempt failed (%s): %v", model, lastErr)
			continue
		}
		var apiResponse struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(body, &apiResponse); err != nil {
			lastErr = fmt.Errorf("JSON parse error: %w", err)
			log.Printf("⚠️ AIML model attempt failed (%s): %v", model, lastErr)
			continue
		}
		if len(apiResponse.Choices) == 0 {
			lastErr = fmt.Errorf("no response choices from AIML API")
			log.Printf("⚠️ AIML model attempt failed (%s): %v", model, lastErr)
			continue
		}
		return apiResponse.Choices[0].Message.Content, model, nil
	}
	return "", "", lastErr
}

// ============================================================================
// SYSTEM PROMPT BUILDER
// ============================================================================

// buildMistralSystemPrompt creates the PRD-style context-aware system prompt
func buildMistralSystemPrompt(context map[string]interface{}, regime map[string]interface{}) string {
	contextStr := marshalContextForPrompt(context, butlerMaxContextChars)

	// Build regime guidance
	regimeGuidance := buildRegimeGuidance(regime)

	prompt := fmt.Sprintf(`You are Butler, the business intelligence AI for PH Trading WLL — a Bahrain-based process instrumentation company.
You have access to connected business data in this system and should answer only from that evidence.
Do not invent entities, people, products, projects, invoice numbers, payment events, competitor notes, or business assertions not present in context.

CURRENT SYSTEM DATE:
- Treat today as %s.
- Treat the current operating year as %d.
- Do not describe explicitly requested periods in %d as "future" unless the period is actually after today.

DATA-GROUNDING RULES:
- Answer only from facts present in context JSON and keys listed below.
- If a requested data point is not present, state explicitly: "I cannot confirm this from current data." Then immediately provide the closest grounded data, adjacent metric, or next-best action you can support from context.
- For period-scoped questions (quarter/month/year), use 'customer_period_summary' when provided.
- For company-wide quarter/month/year questions, use 'business_period_summary' when provided and treat it as authoritative.
- For year-scoped questions, use 'business_year_summary' when provided and treat it as authoritative.
- If the user asks whether the system has access to a year's data, answer with coverage across invoices, orders, offers, and opportunities rather than only invoiced revenue.
- If period summary is empty, do not infer from lifetime data and present period-specific result only.

COMPANY PROFILE:
- Industry: Process Instrumentation & Control Systems (Bahrain)
- Currency: BHD (Bahraini Dinar, 3 decimal places = fils)
- Divisions: PH Trading (primary) + AHS Trading (secondary)
- VAT Rate: 10%%

FULL DATA ACCESS — you have live data on:
• CUSTOMERS: live customer accounts — profiles, grades (A/B/C/D), payment behavior, credit limits, outstanding AR, invoices, orders, contacts, CRM notes, follow-up tasks, active offers, AR risk tiers
• CUSTOMER ACTIVITY: Dormant customers (no orders 6+ months), new customers this year, lapsed customers (ordered last year but not this year), customer retention signals
• CUSTOMER PROFITABILITY: Revenue and gross margin by customer — top earners, lowest margin accounts (pricing opportunities)
• SUPPLIERS: live supplier records — profiles, PO history, invoice matching, payment history, GRN quality stats
• SUPPLIER PERFORMANCE: Average delivery lead times (PO date → GRN date), QC pass/fail rates by supplier — who is reliable, who isn't
• SALES PIPELINE: Offers (RFQ → Quotation → Won/Lost/Expired), costing sheets, win rates, opportunities by stage, competition markers when stored
• OFFER EXPIRY: Active quotes approaching ValidityDate — this week / this month / next 60 days, total value at risk
• COMPETITION: Win/loss trend, top lost-deal reasons, and supplier/product competitiveness signals from stored records
• ORDERS & INVOICES: live orders and customer invoices, including status, line items, and margins
• PROCUREMENT: live purchase orders (multi-currency EUR/USD/BHD), 3-way matching (PO/GRN/Invoice), GRN quality
• DELIVERY: live delivery notes — Prepared/Dispatched/Delivered, serial traceability
• SERVICE DUE INTELLIGENCE: Serial-tracked installations nearing calibration, warranty, or annual-service milestones
• FINANCE: AR/AP, bank balances (multiple accounts), FX rates, monthly revenue trends, payment predictions
• DSO ANALYSIS: Days Sales Outstanding per customer — who pays in 15 days vs 90 days, overall DSO trend
• BANKING: Bank statements, reconciliation status, outstanding/stale/bounced cheques
• PAYMENT CLOSURE READINESS: Opportunities with linked invoices fully paid and therefore eligible to review for closure
• PRODUCTS: live product catalog, pricing, margins, order frequency, serial tracking
• FORECASTING: Weighted pipeline by stage, opportunities closing in 60 days, payment day predictions, win rates by product type, salesperson performance
• RISK: Overdue AR/AP, credit-blocked customers, expiring offers, system alerts
• ACTION ITEMS: Prioritized list — overdue invoices to chase, offers expiring this week, stuck delivery notes, POs awaiting supplier acknowledgment, supplier invoices pending approval
• DOCUMENT PRECHECKS: Quotation readiness context — resolved customer, primary contact, recent offers, missing required fields before drafting
• STRUCTURED MEMORY: Recent entity notes, pending follow-ups, and tactical quick captures relevant to the active customer/workflow
• PROACTIVE BRIEFING: Compact daily operating picture — action items, risks, service follow-ups, and cashflow cues
• WORK MANAGEMENT: Employees, collaborative tasks, task comments, and task notifications
• DATABASE ACCESS: Cross-module record retrieval covering notes, offers and line items, orders and line items, invoices, supplier invoices, payments, bank statements, expenses, and related operational records
• INTELLIGENCE: Three-Regime dynamics (R1=Exploration/R2=Optimization/R3=Stabilization), payment behavior models

COMPLETE BUSINESS DATA:
%s

%s

YOUR CAPABILITIES:
1. Answer ANY question about the business — customers, suppliers, finance, operations, pipeline, risk
2. Discuss FUTURE PROSPECTS — use only explicit pipeline data, payment predictions, and win probability from context
3. STRATEGIC ANALYSIS — recommend actions, identify patterns, surface insights from data trends
4. FORECASTING — estimate future revenue using pipeline × close probability, analyze payment timing risk
5. WHAT-IF SCENARIOS — reason about hypothetical situations with explicit stated assumptions
7. RELATIONSHIP ANALYSIS — cross-reference customers, orders, offers, and payments to find patterns
8. DAILY PRIORITIZATION — tell the team what to focus on today: overdue collections, expiring offers, stuck deliveries, pending approvals
9. CUSTOMER HEALTH — identify dormant accounts, churn risk, and new customer opportunities worth nurturing
10. COMPETITIVE INTELLIGENCE — analyze win/loss patterns and pricing vs margin trade-offs when present in context
11. SUPPLIER BENCHMARKING — compare lead times and QC reliability by supplier from context
12. PAYMENT BEHAVIOR — DSO by customer, predict who will pay late, recommend collection priority
13. SERVICE FOLLOW-UPS — identify installed equipment that is likely due for calibration, warranty action, or annual service
14. DOCUMENT READINESS — before drafting quotations or service offers, verify resolved customer/contact context and missing fields
15. PROACTIVE BRIEFING — summarize what needs attention today across collections, operations, service, and approvals
16. CONTINUITY — use structured memory to recall recent notes, follow-ups, and tactical tasks without inventing new facts
17. CREATE CUSTOMERS — create new customer records from natural language (name, type, grade, city, contact)
18. CREATE SUPPLIERS — create new supplier records from natural language (name, type, country, contact, brands)

RESPONSE GUIDELINES:
- Always cite specific numbers from the context data when available
- For forward-looking questions, clearly distinguish between actual data and projections
- Be as detailed or as brief as the question requires — match depth to complexity
- Write in polished executive prose. Do not use emojis, markdown heading markers like ###, or asterisks for emphasis.
- Use short section titles and simple dash bullets only when they improve readability.
- If data is not in context, say so and offer what you do know
- Do not dead-end with a refusal when related grounded data exists; pivot to the nearest useful answer, comparison, shortlist, or clarifying question
- Suggest next actions only when supported by context
- For strategic recommendations, ground them in the actual data provided
- Never invent contact people, internal owners, invoice numbers, project names, or payment events that are absent from context
- Treat entity_resolution, customer_data, supplier_data, employee_context, contact_context, account_manager_context, service_due_installations, closeable_opportunities, quotation_precheck, work_data, and database_access as higher-trust sources when present
- Treat clarification_prompt and quotation_precheck.draft_payload as the preferred handoff rails when they are present
- Treat butler_memory and proactive_briefing as trusted context for continuity and daily-priority answers
- For "who handled"/ownership questions, only mention people present in context fields (e.g., user references, account_manager_context, team context tags); otherwise state no owner is currently recorded
- If a period is requested (quarter/year/month) and customer_period_summary.status is empty_period_window or no period rows exist, present that clearly as no activity in that period
- If business_year_summary is present, use it to distinguish invoice activity from broader commercial coverage such as orders, offers, and opportunities
- Do not claim a PDF, Excel export, or any document file was generated unless the context explicitly includes a real file path or generated artifact reference
- If entity_resolution indicates ambiguous=true, ask a short clarification question and show the alternatives instead of committing to one match
- For cashflow questions, prefer cashflow_projection monthly_projection when present and state that operating cost is a static assumption if included
- For quotation drafting, use quotation_precheck and draft_clues to summarize what is known, what was inferred from the user's request, and what fields are still missing
- For questions about external events (bank outages, AWS incidents, political disruption, etc.), do not speculate about real-world causation from outside knowledge. Instead, quantify the internal exposure you can see in the system and state clearly that the external event itself is not confirmed in current data
- For create/update/delete requests, ask concise clarifying questions when required fields are missing; otherwise provide a short preview and executable action block rather than only descriptive text

ACTION FORMAT CONTRACT (must be explicit and execute-ready):
%s

Available targets:
%s
`,
		time.Now().Format("2006-01-02"), time.Now().Year(), time.Now().Year(), contextStr, regimeGuidance, buildButlerActionContractPrompt(), strings.Join(buildButlerActionTargetAliases(), ", "))

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

func buildButlerActionContract() string {
	return buildButlerActionContractPrompt()
}

func buildButlerActionContractPrompt() string {
	return `Use [ACTIONS]...[/ACTIONS] to return machine-executable actions.

Rules:
- Emit JSON action objects only when the capability is supported by live context.
- Use lower-case values for type and target.
- Include a short label for every action.
- Never emit an action if required fields are missing or unclear.
- If data is missing, ask for clarification instead of guessing IDs, statuses, or entities.
- Never invent entities, ids, statuses, or workflow statuses.

General schema:
{"type":"...","target":"...","label":"...","data":{...}}

1) APPROVE / REJECT
{"type":"approve|reject","target":"<target>","label":"...","data":{"entity_id":"<id>","reason":"<optional>"}}
Requirements:
- required: type, target, data.entity_id
- supported targets: purchase_order, costing_sheet, supplier_invoice, stock_adjustment, offer, quotation, order, rfq, invoice
- Never invent ids.

2) UPDATE
{"type":"update","target":"<target>","label":"...","data":{"entity_id":"<id>","status":"<exact status/stage string>"}}
Requirements:
- required: type, target, data.entity_id, data.status or data.stage
- allowed status/stage by target:
  - opportunity: New, Qualified, Proposal, Quoted, Won, Lost, On Hold
  - purchase_order: Draft, Pending Approval, Approved, Sent, Acknowledged, Partially Received, Received, Closed, Cancelled
  - order: Draft, Confirmed, Processing, InProgress, Shipped, PartiallyDelivered, FullyDelivered, Delivered, Invoiced, Complete, Cancelled
  - rfq: RFQ Received, Offer Sent, Follow-up/Eval, PO/LOI Received, Order Placed, In Process, Delivered, Closed (Payment), Closed (Lost)
  - offer/quotation: draft, quoted, sent, accepted, rejected, won, lost
  - costing_sheet: draft, pending_approval, approved, rejected
  - follow_up: pending, in_progress, completed, cancelled, overdue
  - stock_adjustment: pending, approved, rejected
- opportunity updates may also include comment and owner_notes when the request is about notes or detail cleanup

3) CREATE
{"type":"create", "target":"<target>", "label":"...", "data":{...}}
Requirements by target:
- offer/quotation: customer_id or customer_name, and one of line_items/items OR amount (grand_total, total, total_amount, amount, amount_bhd)
- follow_up: customer_id or customer_name, title
- order: order_number/reference, customer_id or customer_name, and amount
- opportunity: customer_id or customer_name, and project/title/opportunity_name
- stock_adjustment: inventory_item_id or item_id, reason, and one of variance or (system_quantity + physical_quantity)
- customer: business_name (required), optional: customer_type, payment_grade, city, country, primary_email, primary_phone, industry
- supplier: supplier_name (required), optional: supplier_type, country, primary_contact, email, phone, brands_handled
- customer_contact: customer_id or customer_name, contact_name, optional job_title, email, phone, address, is_primary_contact
- supplier_contact: supplier_id or supplier_name, contact_name, optional job_title, email, phone, address, is_primary_contact
- optional fields: unit_cost, notes, due_date, priority, quote_type, vat_rate

4) OPEN / NAVIGATE
{"type":"open|navigate","target":"<screen_or_entity>","label":"...","data":{}}
Requirements:
- required: type and target
- target should map to app screens: dashboard, opportunities, relationships, finance, operations, intelligence, settings, customers, suppliers, orders, offers, rfq, costing_sheet, purchase_order, supplier_invoice, invoice, stock_adjustment
- include lookup ids when available (offer_id, order_id, customer_id)

5) ANALYZE / FETCH / DAILY_BRIEFING
{"type":"analyze|fetch|daily_briefing","target":"<topic>","label":"...","data":{}}
Requirements:
- required: type and target.

6) STATUS FIELDING
- Always send explicit status/stage in data.status or data.stage for update actions.
- Never emit placeholders.
- Do not claim readiness if required fields are missing.

Examples:
[ACTIONS]
[{"type":"approve","target":"costing_sheet","label":"Approve costing sheet","data":{"entity_id":123,"reason":"Margin approval complete"}}]
[{"type":"reject","target":"supplier_invoice","label":"Reject supplier invoice","data":{"entity_id":456,"reason":"Mismatch against GRN"}}]
[{"type":"create","target":"offer_draft","label":"Create offer draft","data":{"customer_id":789,"customer_name":"BAPCO","quote_type":"Quotation","vat_rate":10}}]
[{"type":"create","target":"follow_up","label":"Create follow-up","data":{"customer_name":"BAPCO","title":"Request additional requirements","due_date":"2026-01-12","priority":"high"}}]
[{"type":"create","target":"stock_adjustment","label":"Create stock adjustment","data":{"inventory_item_id":"ITM-0001","reason":"Physical audit adjustment","physical_quantity":40,"system_quantity":38,"unit_cost":12.5}}]
[{"type":"create","target":"order","label":"Create order","data":{"order_number":"ORD-2026-111","customer_name":"BAPCO","total_amount":12450}}]
[{"type":"create","target":"opportunity","label":"Create opportunity","data":{"customer_name":"BAPCO","title":"Upgrade custody transfer skids","amount":18500,"notes":"Created from Butler request"}}]
[{"type":"create","target":"customer","label":"Create customer","data":{"business_name":"Gulf Industrial Co","customer_type":"Corporate","city":"Manama","country":"Bahrain","industry":"Oil & Gas"}}]
[{"type":"create","target":"supplier","label":"Create supplier","data":{"supplier_name":"Endress+Hauser AG","supplier_type":"Manufacturer","country":"Switzerland","brands_handled":"Endress+Hauser"}}]
[{"type":"create","target":"customer_contact","label":"Create customer contact","data":{"customer_name":"BAPCO","contact_name":"Bader Nabeel","job_title":"Instrument Engineer - FPE Department","email":"bader.nabeel@bapcoenergies.com","phone":"+973-17752985","mobile":"+973-32009292","address":"P.O. Box 25555, Awali, Kingdom of Bahrain"}}]
[{"type":"update","target":"opportunity","label":"Update opportunity notes","data":{"entity_id":"OPP-123","comment":"Customer requested revised commercial split","owner_notes":"Follow up after internal pricing review"}}]
[{"type":"update","target":"rfq","label":"Move RFQ to In Process","data":{"entity_id":333,"stage":"In Process"}}]
[{"type":"update","target":"stock_adjustment","label":"Approve adjustment","data":{"entity_id":77,"status":"approved"}}]
[{"type":"navigate","target":"finance","label":"Open finance hub","data":{}}]
[{"type":"open","target":"orders","label":"Open recent orders","data":{"order_id":42}}]
[{"type":"fetch","target":"daily_briefing","label":"Show daily briefing","data":{}}]
[/ACTIONS]`
}

func buildButlerActionTargetAliases() []string {
	return []string{
		"customers",
		"suppliers",
		"customer",
		"supplier",
		"invoices",
		"offers",
		"order",
		"orders",
		"opportunity",
		"opportunities",
		"rfq",
		"rfqs",
		"offer",
		"offer_draft",
		"deliveries",
		"dashboard",
		"opportunities",
		"purchase_order",
		"purchase_orders",
		"po",
		"delivery_note",
		"delivery_notes",
		"bank_reconciliation",
		"cheque_register",
		"cash_position",
		"butler",
		"costing_sheet",
		"costingsheet",
		"costing",
		"supplier_invoice",
		"supplier_invoices",
		"follow_up",
		"follow_up_task",
		"tasks",
		"follow_ups",
		"customer_contact",
		"customer_contacts",
		"supplier_contact",
		"supplier_contacts",
		"contact",
		"contacts",
		"stock_adjustment",
		"stock_adjustments",
		"daily_briefing",
		"contact",
		"quotation",
		"payment",
		"payments",
	}
}

// parseMistralResponse converts Mistral output to ButlerResponse
func parseMistralResponse(aiText string, context map[string]interface{}, intent Intent) ButlerResponse {
	// Extract actions from [ACTIONS]...[/ACTIONS] blocks
	actions := parseActionBlocks(aiText)

	// Clean the message (remove action blocks from display text)
	cleanMessage := actionBlockRegex.ReplaceAllString(aiText, "")
	cleanMessage = strings.TrimSpace(cleanMessage)
	displayMessage := normalizeAssistantMessageContent(cleanMessage, actions)

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
		Message:    displayMessage,
		Actions:    actions,
		Confidence: confidence,
		Context:    context,
	}
}

func buildButlerMetadata(intent Intent, context map[string]interface{}, hasFinanceAccess bool, usedBackend, requestedModel, usedModel, fallbackReason string, err error) ButlerResponseMetadata {
	metadata := ButlerResponseMetadata{
		UsedBackend:       usedBackend,
		RequestedModel:    requestedModel,
		UsedModel:         usedModel,
		FallbackReason:    strings.TrimSpace(fallbackReason),
		FinanceDataAccess: hasFinanceAccess,
		ContextMode:       intent.Domain,
		DataCoverage:      collectContextCoverage(context),
		EntityResolution:  getEntityResolutionFromContext(context),
		GeneratedAt:       time.Now().Format(time.RFC3339),
	}
	if err != nil {
		metadata.Error = err.Error()
	}
	return metadata
}

func collectContextCoverage(context map[string]interface{}) []string {
	if len(context) == 0 {
		return []string{}
	}

	keys := []string{
		"business_summary",
		"business_period_summary",
		"business_year_summary",
		"entity_resolution",
		"customer_data",
		"customer_period_summary",
		"installed_base_summary",
		"ar_summary",
		"supplier_data",
		"financial_data",
		"operations_data",
		"risk_data",
		"banking_data",
		"credit_notes",
		"product_catalog",
		"delivery_data",
		"po_summary",
		"forecast_intelligence",
		"serial_inventory",
		"cashflow_projection",
		"dso_analysis",
		"offer_expiry",
		"customer_profitability",
		"supplier_performance",
		"customer_activity",
		"service_due_installations",
		"closeable_opportunities",
		"quotation_precheck",
		"competition_analysis",
		"competition_intelligence",
		"customer_360",
		"contact_context",
		"employee_context",
		"account_manager_context",
		"work_data",
		"database_access",
		"proactive_briefing",
		"action_items",
		"customer_notes",
	}

	coverage := make([]string, 0, len(keys))
	for _, key := range keys {
		if value, ok := context[key]; ok && !isContextEmpty(value) {
			coverage = append(coverage, key)
		}
	}
	return coverage
}

func isContextEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case map[string]interface{}:
		return len(v) == 0
	case map[interface{}]interface{}:
		return len(v) == 0
	case []interface{}:
		return len(v) == 0
	case string:
		return strings.TrimSpace(v) == ""
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return false
		}
		switch string(data) {
		case "null", "{}", "[]":
			return true
		default:
			return false
		}
	}
}

func getEntityResolutionFromContext(context map[string]interface{}) map[string]interface{} {
	raw, ok := context["entity_resolution"]
	if !ok {
		return nil
	}
	resolution, ok := raw.(map[string]interface{})
	if !ok {
		return nil
	}
	return resolution
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

		type actionAlias struct {
			Type        string      `json:"type"`
			ActionType  string      `json:"action_type"`
			Target      string      `json:"target"`
			Topic       string      `json:"topic"`
			ActionTag   string      `json:"action_tag"`
			Screen      string      `json:"screen"`
			Entity      string      `json:"entity"`
			EntityType  string      `json:"entity_type"`
			Resource    string      `json:"resource"`
			Label       string      `json:"label"`
			ActionLabel string      `json:"action_label"`
			Name        string      `json:"name"`
			Data        interface{} `json:"data"`
			Parameters  interface{} `json:"parameters"`
			Payload     interface{} `json:"payload"`
		}

		getStringFromData := func(raw interface{}, keys ...string) string {
			if raw == nil {
				return ""
			}
			mapValue, ok := raw.(map[string]interface{})
			if !ok {
				return ""
			}
			for _, key := range keys {
				if text, ok := mapValue[key].(string); ok {
					text = strings.TrimSpace(text)
					if text != "" {
						return text
					}
				}
			}
			return ""
		}

		rawActions := []actionAlias{}
		if err := json.Unmarshal([]byte(jsonStr), &rawActions); err != nil {
			// Try single-action object fallback
			var single actionAlias
			if err2 := json.Unmarshal([]byte(jsonStr), &single); err2 == nil {
				rawActions = append(rawActions, single)
			} else {
				log.Printf("⚠️ Failed to parse action block: %v", err)
				continue
			}
		}

		for _, ra := range rawActions {
			typeValue := strings.ToLower(strings.TrimSpace(coalesceString(ra.Type, ra.ActionType, ra.ActionTag)))
			rawTarget := coalesceString(ra.Target, ra.Screen, ra.Topic, ra.Entity, ra.EntityType, ra.Resource)
			nType, nTarget := normalizedButlerActionTypeTarget(typeValue, rawTarget)
			if nType == "" {
				continue
			}
			target := nTarget
			label := strings.TrimSpace(coalesceString(ra.Label, ra.ActionLabel, ra.Name))
			data := coalesceInterface(ra.Data, ra.Parameters, ra.Payload)

			if target == "" && data != nil {
				if fallback := getStringFromData(data, "target", "entity", "entity_type", "screen", "topic", "resource"); fallback != "" {
					target = normalizeButlerActionTarget(fallback)
				}
			}

			if nType == "daily_briefing" && target == "" {
				target = "daily_briefing"
			}

			if nType == "create" && target == "" {
				if getStringFromData(data, "entity", "entity_type", "target", "table") == "quotation" {
					target = "offer"
				}
			}

			if label == "" {
				label = inferActionLabel(nType, target)
			}

			if target == "" || typeValue == "" {
				continue
			}

			actions = append(actions, ButlerAction{
				Type:   nType,
				Target: target,
				Label:  label,
				Data:   data,
			})
		}
	}

	return actions
}

func coalesceString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func coalesceInterface(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func inferActionLabel(actionType, target string) string {
	if actionType == "" {
		return "Action"
	}

	prettyTarget := strings.ReplaceAll(target, "_", " ")
	if prettyTarget == "" {
		return fmt.Sprintf("%s action", strings.ToUpper(string(actionType[0]))+actionType[1:])
	}

	return fmt.Sprintf("%s %s", strings.ToUpper(string(actionType[0]))+actionType[1:], strings.TrimSpace(prettyTarget))
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

func marshalContextForPrompt(context map[string]interface{}, maxChars int) string {
	contextJSON, _ := json.MarshalIndent(context, "", "  ")
	contextStr := sanitizeForPrompt(string(contextJSON))
	contextStr = contextCleanupPatterns.ReplaceAllString(contextStr, "")
	if maxChars > 0 && len(contextStr) > maxChars {
		return contextStr[:maxChars] + fmt.Sprintf("\n... [context truncated at %d chars]", maxChars)
	}
	return contextStr
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
	Summary           string                   `json:"summary"`           // 1-2 sentence summary
	ExtractedItems    []map[string]interface{} `json:"extracted_items"`   // Line items (flexible: RFQ items OR bank transactions)
	DetectedCustomer  string                   `json:"detected_customer"` // Customer name if found
	DetectedProject   string                   `json:"detected_project"`  // Project/reference if found
	RequiredDeadline  string                   `json:"required_deadline"` // Deadline if mentioned
	Confidence        float64                  `json:"confidence"`        // 0.0-1.0 confidence score
	SuggestedActions  []ButlerAction           `json:"suggested_actions"` // Recommended next steps
	DocumentType      string                   `json:"document_type"`     // RFQ, PO, Invoice, etc.
	ExtractedMetadata map[string]interface{}   `json:"metadata"`          // Key-value pairs extracted (bank name, balances, etc.)
}

// RFQLineItem represents a line item extracted from an RFQ/email
type RFQLineItem struct {
	Description string `json:"description"`
	Quantity    int    `json:"quantity"`
	Unit        string `json:"unit"`
	PartNumber  string `json:"part_number"`
	Notes       string `json:"notes"`
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
		"costing": true, "Costing": true,
		"excel_data": true, "ExcelData": true,
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
4. Extract line items with quantities, descriptions, part/model numbers, product codes, unit prices, and currency when present
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
    {"description": "...", "quantity": 1, "unit": "pcs", "part_number": "...", "product_code": "...", "model": "...", "unit_price": 0.000, "currency": "BHD", "raw_text": "..."}
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
- Do not put currency codes such as BHD, USD, or EUR in the "unit" field. Use "currency" for currency and "unit" only for physical units such as pcs, sets, m, kg, lot, days, or hours.
- For Endress+Hauser and instrumentation RFQs, preserve item/model codes exactly in part_number/product_code/model even when the description is long.
- For bank statements: DEBIT = money LEAVING the account (outgoing payments, transfers out, charges). CREDIT = money ENTERING the account (incoming payments, transfers in, deposits). Incoming transfers from customers (Fawri, INWD, deposit, TRF-IN, receipts) should be CREDITS. Outgoing payments (DGC, ONUS, Fawri-out, TRF-OUT, withdrawals, fees, charges) should be DEBITS. Use the running balance to validate: if the balance increases after a transaction it is a credit; if it decreases it is a debit. Preserve the bank statement exactly as presented: bank fees (EFTS Charges, SWIFT FEES, Corr Bnk Chg), VAT on fees, and similar service charges must remain as separate transaction rows if the statement shows them separately. Do NOT merge VAT/fee lines into a parent transaction. Use date format YYYY-MM-DD (e.g. 2026-01-15).`

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
