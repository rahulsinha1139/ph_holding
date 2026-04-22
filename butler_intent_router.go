package main

import (
	"fmt"
	"strings"
	"time"
)

type butlerARProjectionScope struct {
	start          time.Time
	end            time.Time
	label          string
	orderStart     time.Time
	includeOrders  bool
	includeOffers  bool
	invoiceOnly    bool
	needsClarify   bool
	intentDetected bool
}

type butlerProjectionStageRow struct {
	Stage    string  `gorm:"column:stage"`
	Count    int64   `gorm:"column:count"`
	TotalBHD float64 `gorm:"column:total_bhd"`
}

func (a *App) tryButlerIntentClarificationFastPath(intent Intent, message string, hasFinanceAccess bool) (string, []ButlerAction, bool) {
	q := normalizeButlerRouterText(message)
	if q == "" {
		return "", nil, false
	}

	if isBroadCapabilitiesQuestion(q) && !isCapabilitySelectionPrompt(q) {
		msg := "You are asking what Butler can do. I can answer that as a live ERP capability route instead of a long generic paragraph. Pick the view you want."
		return msg, []ButlerAction{
			butlerPromptAction("All capabilities", "Show me all Butler capabilities from live ERP data.", "capability_explainer", "all"),
			butlerPromptAction("Sales and OCR", "Show me Butler capabilities for sales pipeline, OCR ingestion, opportunity creation, offer PDFs, and follow-ups.", "capability_explainer", "sales_ocr"),
			butlerPromptAction("Finance and AR", "Show me Butler capabilities for finance, AR projections, orders-to-invoice exposure, and cash risk.", "capability_explainer", "finance_ar"),
			butlerPromptAction("Tasks and briefings", "Show me task, follow-up, and daily briefing capabilities.", "capability_explainer", "tasks"),
			butlerPromptAction("Reports", "Show me report and document generation capabilities.", "capability_explainer", "reports"),
		}, true
	}

	scope := parseButlerARProjectionScope(message)
	if scope.intentDetected && scope.needsClarify {
		if !hasFinanceAccess {
			return "AR projections require finance:view permission. I can still help with non-financial order or follow-up context.", []ButlerAction{}, true
		}
		msg := "You are asking for an AR projection. I can run it a few different ways, and the choice matters because issued invoices, confirmed uninvoiced orders, and weighted offers are different financial buckets."
		return msg, []ButlerAction{
			butlerPromptAction("Invoices only", "Show AR projection for next month using issued invoices only.", "ar_projection", "invoices_only"),
			butlerPromptAction("Include confirmed orders", "Show AR projection for next month including issued invoices and confirmed uninvoiced orders.", "ar_projection", "invoices_confirmed_orders"),
			butlerPromptAction("Add weighted offers", "Show AR projection for next month including issued invoices, confirmed orders, and weighted active offers.", "ar_projection", "weighted_pipeline"),
			butlerPromptAction("Manager brief", "Create a manager AR brief for the next two months with confirmed orders, offer pipeline, and collection risk.", "ar_projection", "manager_brief"),
		}, true
	}

	return "", nil, false
}

func (a *App) tryGroundedARProjectionFastPath(intent Intent, message string, hasFinanceAccess bool) (string, []ButlerAction, bool) {
	if a == nil || a.db == nil {
		return "", nil, false
	}

	scope := parseButlerARProjectionScope(message)
	if !scope.intentDetected || scope.needsClarify {
		return "", nil, false
	}
	if !hasFinanceAccess {
		return "AR projections require finance:view permission. I can still help with non-financial order or follow-up context.", []ButlerAction{}, true
	}

	now := time.Now()
	openStatuses := []string{"Sent", "PartiallyPaid", "Overdue"}

	var currentOpenAR float64
	_ = a.db.Model(&Invoice{}).
		Where("status IN ? AND outstanding_bhd > 0", openStatuses).
		Select("COALESCE(SUM(outstanding_bhd), 0)").
		Scan(&currentOpenAR).Error

	var overdueAR float64
	_ = a.db.Model(&Invoice{}).
		Where("status IN ? AND outstanding_bhd > 0 AND due_date < ?", openStatuses, now).
		Select("COALESCE(SUM(outstanding_bhd), 0)").
		Scan(&overdueAR).Error

	var invoiceDueInWindow float64
	_ = a.db.Model(&Invoice{}).
		Where("status IN ? AND outstanding_bhd > 0 AND due_date >= ? AND due_date < ?", openStatuses, scope.start, scope.end).
		Select("COALESCE(SUM(outstanding_bhd), 0)").
		Scan(&invoiceDueInWindow).Error

	var invoiceDueByWindowEnd float64
	_ = a.db.Model(&Invoice{}).
		Where("status IN ? AND outstanding_bhd > 0 AND due_date < ?", openStatuses, scope.end).
		Select("COALESCE(SUM(outstanding_bhd), 0)").
		Scan(&invoiceDueByWindowEnd).Error

	uninvoicedOrderExposure := 0.0
	pendingOrderCount := 0
	orderErrText := ""
	if scope.includeOrders {
		var err error
		uninvoicedOrderExposure, pendingOrderCount, err = a.calculateUninvoicedOrderExposure(scope.orderStart, scope.end)
		if err != nil {
			orderErrText = fmt.Sprintf(" Order exposure could not be calculated: %v", err)
		}
	}

	weightedPipeline := 0.0
	pipelineLines := []string{}
	if scope.includeOffers {
		weightedPipeline, pipelineLines = a.calculateWeightedButlerPipeline(scope.orderStart, scope.end)
	}

	totalAttention := invoiceDueByWindowEnd
	if scope.includeOrders {
		totalAttention += uninvoicedOrderExposure
	}
	if scope.includeOffers {
		totalAttention += weightedPipeline
	}

	orderLine := "- Confirmed uninvoiced orders were not included in this view."
	if scope.includeOrders {
		orderLine = fmt.Sprintf("- Confirmed uninvoiced orders from %s through %s: %d orders, %s BHD expected new AR once invoiced.%s",
			scope.orderStart.Format("2 Jan 2006"),
			scope.end.Add(-time.Nanosecond).Format("2 Jan 2006"),
			pendingOrderCount,
			formatBHD(uninvoicedOrderExposure),
			orderErrText,
		)
	}

	pipelineText := "- Weighted active offer/opportunity pipeline was not included in this view."
	if scope.includeOffers {
		if len(pipelineLines) == 0 {
			pipelineText = "- No weighted active offer/opportunity pipeline was found for this horizon."
		} else {
			pipelineText = strings.Join(pipelineLines, "\n")
		}
	}

	mode := "invoice-only"
	if scope.includeOrders && scope.includeOffers {
		mode = "issued invoices + confirmed orders + weighted active offers"
	} else if scope.includeOrders {
		mode = "issued invoices + confirmed orders"
	}

	response := fmt.Sprintf(`AR projection - %s

Basis
- Mode: %s.
- Window: %s to %s.
- Order exposure uses the fresh-start operating year from %s so confirmed 2026 orders are not ignored.

Issued invoice AR
- Current open invoice AR: %s BHD.
- Already overdue AR: %s BHD.
- Invoice AR due inside this window: %s BHD.
- Booked invoice AR due by window end, including overdue balances: %s BHD.

Confirmed order receivable path
%s

Weighted pipeline path
%s

Projection
- Receivable attention for this basis: %s BHD.
- This is an AR creation and collection focus number, not guaranteed cash collection.
- Confirmed orders should create receivables only after invoicing; cash timing then depends on payment terms and customer behavior.

Why this route exists
- Butler is using local SQL before the external model here.
- The earlier zero-collection style answer was mixing cash collection prediction with AR creation and was not enough for management decisions. Orders are now explicitly counted when you choose the confirmed-order basis.`, scope.label, mode, scope.start.Format("2 Jan 2006"), scope.end.Add(-time.Nanosecond).Format("2 Jan 2006"), scope.orderStart.Format("2 Jan 2006"), formatBHD(currentOpenAR), formatBHD(overdueAR), formatBHD(invoiceDueInWindow), formatBHD(invoiceDueByWindowEnd), orderLine, pipelineText, formatBHD(totalAttention))

	actions := []ButlerAction{
		butlerPromptAction("Invoice-only view", "Show AR projection for next month using issued invoices only.", "ar_projection", "invoices_only"),
		butlerPromptAction("Include orders", "Show AR projection for next month including issued invoices and confirmed uninvoiced orders.", "ar_projection", "invoices_confirmed_orders"),
		butlerPromptAction("Add weighted offers", "Show AR projection for next month including issued invoices, confirmed orders, and weighted active offers.", "ar_projection", "weighted_pipeline"),
		butlerPromptAction("Manager brief", "Create a manager AR brief for the next two months with confirmed orders, offer pipeline, and collection risk.", "ar_projection", "manager_brief"),
	}

	return response, actions, true
}

func butlerPromptAction(label, prompt, intentID, optionID string) ButlerAction {
	return ButlerAction{
		Type:   "clarify",
		Target: "butler_prompt",
		Label:  label,
		Data: map[string]interface{}{
			"prompt":     prompt,
			"intent_id":  intentID,
			"option_id":  optionID,
			"command_ui": true,
		},
	}
}

func parseButlerARProjectionScope(message string) butlerARProjectionScope {
	q := normalizeButlerRouterText(message)
	now := time.Now()
	start, end, label := nextCalendarMonthWindow(now)
	if strings.Contains(q, "next two months") || strings.Contains(q, "next 2 months") || strings.Contains(q, "next sixty days") || strings.Contains(q, "next 60 days") {
		start = beginningOfDay(now)
		end = start.AddDate(0, 0, 60)
		label = "next 60 days"
	} else if strings.Contains(q, "next thirty days") || strings.Contains(q, "next 30 days") {
		start = beginningOfDay(now)
		end = start.AddDate(0, 0, 30)
		label = "next 30 days"
	}

	scope := butlerARProjectionScope{
		start:      start,
		end:        end,
		label:      label,
		orderStart: time.Date(end.Year(), time.January, 1, 0, 0, 0, 0, time.Local),
	}

	hasAR := butlerContainsWord(q, "ar") ||
		strings.Contains(q, "accounts receivable") ||
		strings.Contains(q, "receivable") ||
		strings.Contains(q, "receivables") ||
		strings.Contains(q, "collection") ||
		strings.Contains(q, "collections")
	hasProjection := strings.Contains(q, "projection") ||
		strings.Contains(q, "predict") ||
		strings.Contains(q, "forecast") ||
		strings.Contains(q, "next month") ||
		strings.Contains(q, "next 30") ||
		strings.Contains(q, "next 60") ||
		strings.Contains(q, "next two months") ||
		strings.Contains(q, "next 2 months")
	hasOrderChallenge := (strings.Contains(q, "order") || strings.Contains(q, "orders")) &&
		(strings.Contains(q, "receivable") || butlerContainsWord(q, "ar") || strings.Contains(q, "pipeline") || strings.Contains(q, "invoice"))

	scope.intentDetected = (hasAR && hasProjection) || hasOrderChallenge
	if !scope.intentDetected {
		return scope
	}

	scope.invoiceOnly = strings.Contains(q, "invoice only") ||
		strings.Contains(q, "invoices only") ||
		strings.Contains(q, "issued invoices only") ||
		strings.Contains(q, "booked ar only")
	scope.includeOrders = strings.Contains(q, "confirmed order") ||
		strings.Contains(q, "confirmed uninvoiced") ||
		strings.Contains(q, "uninvoiced order") ||
		strings.Contains(q, "include orders") ||
		strings.Contains(q, "orders in the pipeline") ||
		strings.Contains(q, "order pipeline") ||
		hasOrderChallenge
	scope.includeOffers = strings.Contains(q, "weighted") ||
		strings.Contains(q, "active offers") ||
		strings.Contains(q, "offer pipeline") ||
		strings.Contains(q, "opportunity pipeline") ||
		strings.Contains(q, "opportunities")

	if scope.includeOffers {
		scope.includeOrders = true
	}

	scope.needsClarify = !scope.invoiceOnly && !scope.includeOrders && !scope.includeOffers
	return scope
}

func (a *App) calculateWeightedButlerPipeline(start, end time.Time) (float64, []string) {
	if a == nil || a.db == nil {
		return 0, []string{}
	}

	rows := []butlerProjectionStageRow{}
	_ = a.db.Table("offers").
		Select("COALESCE(stage, 'Unstaged') AS stage, COUNT(*) AS count, COALESCE(SUM(total_value_bhd), 0) AS total_bhd").
		Where("deleted_at IS NULL").
		Where("quotation_date >= ? AND quotation_date < ?", start, end).
		Where("stage IN ?", []string{"RFQ", "Qualified", "Proposal", "Quoted", "Won"}).
		Group("COALESCE(stage, 'Unstaged')").
		Order("total_bhd DESC").
		Scan(&rows).Error

	var opportunityRows []butlerProjectionStageRow
	_ = a.db.Table("opportunities").
		Select("COALESCE(stage, 'Unstaged') AS stage, COUNT(*) AS count, COALESCE(SUM(revenue_bhd), 0) AS total_bhd").
		Where("deleted_at IS NULL").
		Where("(year = ? OR (offer_date >= ? AND offer_date < ?))", end.Year(), start, end).
		Where("stage IN ?", []string{"RFQ", "Qualified", "Proposal", "Quoted", "Won"}).
		Group("COALESCE(stage, 'Unstaged')").
		Order("total_bhd DESC").
		Scan(&opportunityRows).Error

	rows = append(rows, opportunityRows...)
	if len(rows) == 0 {
		return 0, []string{}
	}

	weighted := 0.0
	lines := []string{"- Weighted active pipeline, separated from confirmed orders:"}
	for _, row := range rows {
		if row.Count == 0 || row.TotalBHD <= 0 {
			continue
		}
		weight := butlerPipelineStageWeight(row.Stage)
		weightedValue := row.TotalBHD * weight
		weighted += weightedValue
		lines = append(lines, fmt.Sprintf("  - %s: %d records, %s BHD at %.0f%% = %s BHD",
			firstNonEmpty(strings.TrimSpace(row.Stage), "Unstaged"),
			row.Count,
			formatBHD(row.TotalBHD),
			weight*100,
			formatBHD(weightedValue),
		))
	}

	if weighted <= 0 {
		return 0, []string{}
	}
	lines = append(lines, fmt.Sprintf("- Weighted pipeline contribution: %s BHD.", formatBHD(weighted)))
	return round3(weighted), lines
}

func butlerPipelineStageWeight(stage string) float64 {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "won":
		return 0.85
	case "proposal":
		return 0.50
	case "quoted":
		return 0.40
	case "qualified":
		return 0.25
	case "rfq":
		return 0.15
	default:
		return 0.10
	}
}

func isBroadCapabilitiesQuestion(q string) bool {
	signals := []string{
		"what can you do",
		"what all can you do",
		"all that you can do",
		"tell me all that you can do",
		"what you can do",
		"tell us what you can do",
		"tell me what you can do",
		"what are your capabilities",
		"understand your capabilities",
		"your capabilities",
		"what can butler do",
		"how can you help",
		"what can u do",
	}
	for _, signal := range signals {
		if strings.Contains(q, signal) {
			return true
		}
	}
	return false
}

func isCapabilitySelectionPrompt(q string) bool {
	selections := []string{
		"show me all butler capabilities",
		"show all butler capabilities",
		"all butler capabilities",
		"capabilities from live erp data",
		"capabilities for sales pipeline",
		"capabilities for finance",
		"ocr ingestion",
		"task follow up and daily briefing capabilities",
		"report and document generation capabilities",
	}
	for _, selection := range selections {
		if strings.Contains(q, selection) {
			return true
		}
	}
	return false
}

func normalizeButlerRouterText(message string) string {
	q := strings.ToLower(strings.TrimSpace(message))
	replacer := strings.NewReplacer(
		"’", "'",
		"“", "\"",
		"”", "\"",
		"\n", " ",
		"\t", " ",
	)
	q = replacer.Replace(q)
	for strings.Contains(q, "  ") {
		q = strings.ReplaceAll(q, "  ", " ")
	}
	return q
}

func butlerContainsWord(q, word string) bool {
	normalized := strings.ToLower(q)
	replacer := strings.NewReplacer(
		".", " ",
		",", " ",
		"?", " ",
		"!", " ",
		";", " ",
		":", " ",
		"/", " ",
		"\\", " ",
		"(", " ",
		")", " ",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		"-", " ",
		"_", " ",
	)
	normalized = " " + replacer.Replace(normalized) + " "
	for strings.Contains(normalized, "  ") {
		normalized = strings.ReplaceAll(normalized, "  ", " ")
	}
	return strings.Contains(normalized, " "+strings.ToLower(strings.TrimSpace(word))+" ")
}

func nextCalendarMonthWindow(now time.Time) (time.Time, time.Time, string) {
	location := now.Location()
	start := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, location)
	end := start.AddDate(0, 1, 0)
	return start, end, start.Format("January 2006")
}

func beginningOfDay(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, value.Location())
}
