package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

func (a *App) tryGroundedCustomerFastPath(intent Intent, message string) (string, bool) {
	return a.tryGroundedCustomerFastPathWithHint(intent, message, "")
}

func (a *App) tryGroundedCustomerFastPathWithHint(intent Intent, message, customerHint string) (string, bool) {
	if a == nil || a.db == nil {
		return "", false
	}

	q := strings.ToLower(strings.TrimSpace(message))
	if q == "" {
		return "", false
	}

	if (strings.Contains(q, "all three are same") || strings.Contains(q, "all are the same") || strings.Contains(q, "same customer")) && strings.TrimSpace(customerHint) != "" {
		_, displayName := a.resolveCustomerScope(customerHint)
		if strings.TrimSpace(displayName) == "" {
			displayName = strings.TrimSpace(customerHint)
		}
		return fmt.Sprintf("Understood. I will treat those records as a single customer group (%s) for this chat and proceed with grouped data/actions.", displayName), true
	}

	customerRef := inferCustomerReferenceFromQuery(intent, q)
	if customerRef == "" && strings.TrimSpace(customerHint) != "" {
		customerRef = strings.TrimSpace(customerHint)
	}
	if customerRef == "" {
		return "", false
	}

	if strings.Contains(q, "revenue projection") || strings.Contains(q, "revenue projections") || strings.Contains(q, "projected revenue") {
		return a.buildRevenueProjectionResponse(), true
	}

	customerIDs, displayName := a.resolveCustomerScope(customerRef)
	if len(customerIDs) == 0 && strings.TrimSpace(customerHint) != "" && !strings.EqualFold(strings.TrimSpace(customerHint), strings.TrimSpace(customerRef)) {
		customerIDs, displayName = a.resolveCustomerScope(customerHint)
	}
	if len(customerIDs) == 0 {
		return "", false
	}

	if strings.Contains(q, "invoice") && (strings.Contains(q, "this quarter") || strings.Contains(q, "current quarter")) {
		return a.buildCustomerQuarterInvoiceAndHandlerResponse(customerIDs, displayName, q), true
	}

	if strings.Contains(q, "line item") || strings.Contains(q, "line items") || strings.Contains(q, "what we have sold") || strings.Contains(q, "what have we sold") || strings.Contains(q, "sold to them") || strings.Contains(q, "sold to") || strings.Contains(q, "equipment sold") || strings.Contains(q, "what sold") || strings.Contains(q, "products sold") {
		return a.buildCustomerLineItemsResponse(customerIDs, displayName), true
	}

	if strings.Contains(q, "invoice") {
		return a.buildCustomerInvoiceOverviewResponse(customerIDs, displayName), true
	}

	if strings.Contains(q, "offer") && strings.Contains(q, "this year") {
		return a.buildCustomerYearOffersResponse(customerIDs, displayName, time.Now().Year()), true
	}

	if strings.Contains(q, "offer") {
		return a.buildCustomerOfferOverviewResponse(customerIDs, displayName), true
	}

	return "", false
}

func inferCustomerReferenceFromQuery(intent Intent, q string) string {
	if strings.TrimSpace(intent.EntityName) != "" {
		return strings.TrimSpace(intent.EntityName)
	}

	switch {
	case strings.Contains(q, "bapco"):
		return "BAPCO"
	case strings.Contains(q, "alba"):
		return "ALBA"
	case strings.Contains(q, "al ezzel"), strings.Contains(q, "alezzel"):
		return "Al Ezzel"
	}

	re := regexp.MustCompile(`\b(?:for|from|to|with)\s+([a-z0-9&\.\-\s]{2,60})`)
	m := re.FindStringSubmatch(q)
	if len(m) == 2 {
		candidate := strings.TrimSpace(m[1])
		candidate = strings.TrimSuffix(candidate, "this quarter")
		candidate = strings.TrimSuffix(candidate, "current quarter")
		candidate = strings.TrimSuffix(candidate, "this year")
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}

	return ""
}

func (a *App) resolveCustomerScope(reference string) ([]string, string) {
	ref := strings.ToLower(strings.TrimSpace(reference))
	if ref == "" {
		return nil, ""
	}

	if strings.Contains(ref, "bapco") || strings.Contains(ref, "bahrain petroleum") {
		var rows []struct {
			CustomerID   string
			BusinessName string
		}
		a.db.Table("customers").
			Select("customer_id, business_name").
			Where("business_name LIKE ? OR business_name LIKE ?", "%BAPCO%", "%Bahrain Petroleum%").
			Find(&rows)
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			if strings.TrimSpace(r.CustomerID) != "" {
				ids = append(ids, strings.TrimSpace(r.CustomerID))
			}
		}
		if len(ids) > 0 {
			return uniqueStrings(ids), "BAPCO group"
		}
	}

	if strings.Contains(ref, "alba") || strings.Contains(ref, "aluminium bahrain") {
		var rows []struct {
			CustomerID string
		}
		a.db.Table("customers").
			Select("customer_id").
			Where("business_name LIKE ? OR business_name LIKE ?", "%ALBA%", "%Aluminium Bahrain%").
			Find(&rows)
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			if strings.TrimSpace(r.CustomerID) != "" {
				ids = append(ids, strings.TrimSpace(r.CustomerID))
			}
		}
		if len(ids) > 0 {
			return uniqueStrings(ids), "ALBA group"
		}
	}

	if strings.Contains(ref, "al ezzel") || strings.Contains(ref, "alezzel") {
		var rows []struct {
			CustomerID string
		}
		a.db.Table("customers").
			Select("customer_id").
			Where("business_name LIKE ?", "%Ezzel%").
			Find(&rows)
		ids := make([]string, 0, len(rows))
		for _, r := range rows {
			if strings.TrimSpace(r.CustomerID) != "" {
				ids = append(ids, strings.TrimSpace(r.CustomerID))
			}
		}
		if len(ids) > 0 {
			return uniqueStrings(ids), "Al Ezzel group"
		}
	}

	resolution := a.resolveCustomerReference(reference)
	if resolution == nil || strings.TrimSpace(resolution.EntityID) == "" {
		return nil, ""
	}
	return []string{resolution.EntityID}, firstNonEmpty(strings.TrimSpace(resolution.DisplayName), reference)
}

func (a *App) buildCustomerQuarterInvoiceAndHandlerResponse(customerIDs []string, customerName, q string) string {
	start, end, label, ok := parseQuarterWindowFromQuery("this quarter")
	if !ok {
		now := time.Now()
		month := int(now.Month())
		quarter := (month-1)/3 + 1
		startMonth := time.Month((quarter-1)*3 + 1)
		start = time.Date(now.Year(), startMonth, 1, 0, 0, 0, 0, time.Local)
		end = start.AddDate(0, 3, 0).Add(-time.Second)
		label = fmt.Sprintf("Q%d %d", quarter, now.Year())
	}

	type invRow struct {
		InvoiceNumber string
		InvoiceDate   time.Time
		GrandTotalBHD float64
		Status        string
		IssuedBy      string
		Attention     string
		CreatedBy     string
	}

	var invoices []invRow
	a.db.Table("invoices").
		Select("invoice_number, invoice_date, grand_total_bhd, status, issued_by, attention_person, created_by").
		Where("customer_id IN ? AND invoice_date >= ? AND invoice_date <= ? AND status NOT IN ?", customerIDs, start, end, []string{"Cancelled", "Void", "Proforma", "Draft"}).
		Order("invoice_date DESC").
		Limit(20).
		Scan(&invoices)

	handlers := a.collectAccountHandlers(customerIDs)
	for _, inv := range invoices {
		handlers = appendIfPresent(handlers, inv.IssuedBy)
		handlers = appendIfPresent(handlers, inv.Attention)
		handlers = appendIfPresent(handlers, inv.CreatedBy)
	}

	if len(invoices) == 0 {
		return fmt.Sprintf("I checked %s invoices for %s and found no recorded invoices in %s.\n\nAccount handlers on record: %s.",
			label, customerName, label, joinOrNone(handlers))
	}

	total := 0.0
	lines := make([]string, 0, minInt(5, len(invoices)))
	for i, inv := range invoices {
		total += inv.GrandTotalBHD
		if i < 5 {
			lines = append(lines, fmt.Sprintf("- %s | %s | %.3f BHD | %s",
				strings.TrimSpace(inv.InvoiceNumber),
				inv.InvoiceDate.Format("2006-01-02"),
				inv.GrandTotalBHD,
				strings.TrimSpace(inv.Status),
			))
		}
	}

	return fmt.Sprintf("I found %d invoice(s) for %s in %s, totaling %.3f BHD.\n\nRecent invoices:\n%s\n\nPeople handling this account (from contacts/docs): %s.",
		len(invoices), customerName, label, total, strings.Join(lines, "\n"), joinOrNone(handlers))
}

func (a *App) buildCustomerInvoiceOverviewResponse(customerIDs []string, customerName string) string {
	type invRow struct {
		ID            string
		InvoiceNumber string
		InvoiceDate   time.Time
		GrandTotalBHD float64
		Status        string
		IssuedBy      string
		Attention     string
		CreatedBy     string
	}

	var invoices []invRow
	a.db.Table("invoices").
		Select("id, invoice_number, invoice_date, grand_total_bhd, status, issued_by, attention_person, created_by").
		Where("customer_id IN ? AND status NOT IN ?", customerIDs, []string{"Cancelled", "Void", "Proforma", "Draft"}).
		Order("invoice_date DESC").
		Limit(25).
		Scan(&invoices)

	handlers := a.collectAccountHandlers(customerIDs)
	total := 0.0
	invoiceIDs := make([]string, 0, len(invoices))
	for _, inv := range invoices {
		total += inv.GrandTotalBHD
		invoiceIDs = append(invoiceIDs, inv.ID)
		handlers = appendIfPresent(handlers, inv.IssuedBy)
		handlers = appendIfPresent(handlers, inv.Attention)
		handlers = appendIfPresent(handlers, inv.CreatedBy)
	}
	handlers = uniqueStrings(handlers)

	if len(invoices) == 0 {
		return fmt.Sprintf("I checked invoices for %s and found no recorded invoices.\n\nPeople handling this account: %s.",
			customerName, joinOrNone(handlers))
	}

	type instRow struct {
		Equipment   string
		Model       string
		Description string
	}
	var instRows []instRow
	a.db.Table("invoice_items").
		Select("equipment, model, description").
		Where("invoice_id IN ?", invoiceIDs).
		Limit(80).
		Scan(&instRows)

	instruments := make([]string, 0, len(instRows))
	for _, r := range instRows {
		label := strings.TrimSpace(firstNonEmpty(r.Equipment, r.Model, r.Description))
		if label != "" {
			instruments = append(instruments, label)
		}
	}
	instruments = uniqueStrings(instruments)

	type payRow struct {
		InvoiceNumber string
		PaymentDate   time.Time
		AmountBHD     float64
	}
	var payments []payRow
	a.db.Table("payments").
		Select("invoice_number, payment_date, amount_bhd").
		Where("invoice_id IN ?", invoiceIDs).
		Order("payment_date DESC").
		Limit(20).
		Scan(&payments)

	receivedLines := make([]string, 0, minInt(6, len(payments)))
	for i, p := range payments {
		if i >= 6 {
			break
		}
		receivedLines = append(receivedLines, fmt.Sprintf("- %s | %s | %.3f BHD",
			strings.TrimSpace(p.InvoiceNumber),
			p.PaymentDate.Format("2006-01-02"),
			p.AmountBHD,
		))
	}

	invoiceLines := make([]string, 0, minInt(6, len(invoices)))
	for i, inv := range invoices {
		if i >= 6 {
			break
		}
		invoiceLines = append(invoiceLines, fmt.Sprintf("- %s | %s | %.3f BHD | %s",
			strings.TrimSpace(inv.InvoiceNumber),
			inv.InvoiceDate.Format("2006-01-02"),
			inv.GrandTotalBHD,
			strings.TrimSpace(inv.Status),
		))
	}

	instText := "not captured on invoice lines"
	if len(instruments) > 0 {
		max := minInt(8, len(instruments))
		instText = strings.Join(instruments[:max], "; ")
	}

	receivedText := "no payment receipt entries found"
	if len(receivedLines) > 0 {
		receivedText = strings.Join(receivedLines, "\n")
	}

	return fmt.Sprintf("Here is the latest invoice snapshot for %s.\n\nTotal invoices: %d\nTotal billed: %.3f BHD\n\nRecent invoices:\n%s\n\nInstruments seen on invoice lines:\n%s\n\nPayments received (latest):\n%s\n\nPeople handling this account:\n%s",
		customerName,
		len(invoices),
		total,
		strings.Join(invoiceLines, "\n"),
		instText,
		receivedText,
		joinOrNone(handlers),
	)
}

func (a *App) buildCustomerYearOffersResponse(customerIDs []string, customerName string, year int) string {
	start := time.Date(year, time.January, 1, 0, 0, 0, 0, time.Local)
	end := time.Date(year, time.December, 31, 23, 59, 59, 0, time.Local)

	type offerRow struct {
		OfferNumber    string
		QuotationDate  time.Time
		Stage          string
		TotalValueBHD  float64
		QuoteType      string
		IssuedBy       string
		Attention      string
		CreatedBy      string
	}

	var offers []offerRow
	a.db.Table("offers").
		Select("offer_number, quotation_date, stage, total_value_bhd, quote_type, issued_by, attention_person, created_by").
		Where("customer_id IN ? AND quotation_date >= ? AND quotation_date <= ?", customerIDs, start, end).
		Order("quotation_date DESC").
		Limit(30).
		Scan(&offers)

	handlers := a.collectAccountHandlers(customerIDs)
	for _, off := range offers {
		handlers = appendIfPresent(handlers, off.IssuedBy)
		handlers = appendIfPresent(handlers, off.Attention)
		handlers = appendIfPresent(handlers, off.CreatedBy)
	}

	if len(offers) == 0 {
		return fmt.Sprintf("I checked offers for %s in %d and found no recorded offers.\n\nAccount handlers on record: %s.",
			customerName, year, joinOrNone(handlers))
	}

	total := 0.0
	lines := make([]string, 0, minInt(6, len(offers)))
	for i, off := range offers {
		total += off.TotalValueBHD
		if i < 6 {
			lines = append(lines, fmt.Sprintf("- %s | %s | %s | %.3f BHD | %s",
				strings.TrimSpace(off.OfferNumber),
				off.QuotationDate.Format("2006-01-02"),
				strings.TrimSpace(off.Stage),
				off.TotalValueBHD,
				firstNonEmpty(strings.TrimSpace(off.QuoteType), "Quotation"),
			))
		}
	}

	return fmt.Sprintf("I found %d offer(s) for %s in %d, totaling %.3f BHD.\n\nRecent offers:\n%s\n\nPeople handling this account (from contacts/docs): %s.",
		len(offers), customerName, year, total, strings.Join(lines, "\n"), joinOrNone(handlers))
}

func (a *App) buildCustomerOfferOverviewResponse(customerIDs []string, customerName string) string {
	type offerRow struct {
		OfferNumber   string
		QuotationDate time.Time
		Stage         string
		TotalValueBHD float64
		QuoteType     string
	}

	var offers []offerRow
	a.db.Table("offers").
		Select("offer_number, quotation_date, stage, total_value_bhd, quote_type").
		Where("customer_id IN ?", customerIDs).
		Order("quotation_date DESC").
		Limit(20).
		Scan(&offers)

	if len(offers) == 0 {
		return fmt.Sprintf("I checked offers for %s and found no recorded offers.", customerName)
	}

	total := 0.0
	lines := make([]string, 0, minInt(8, len(offers)))
	for i, off := range offers {
		total += off.TotalValueBHD
		if i >= 8 {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s | %s | %s | %.3f BHD | %s",
			strings.TrimSpace(off.OfferNumber),
			off.QuotationDate.Format("2006-01-02"),
			strings.TrimSpace(off.Stage),
			off.TotalValueBHD,
			firstNonEmpty(strings.TrimSpace(off.QuoteType), "Quotation"),
		))
	}

	return fmt.Sprintf("I found %d offer(s) for %s totaling %.3f BHD.\n\nRecent offers:\n%s",
		len(offers), customerName, total, strings.Join(lines, "\n"))
}

func (a *App) buildCustomerLineItemsResponse(customerIDs []string, customerName string) string {
	type lineRow struct {
		InvoiceDate time.Time
		InvoiceNo   string
		Equipment   string
		Model       string
		Description string
		Qty         float64
		TotalBHD    float64
	}

	var lines []lineRow
	a.db.Table("invoice_items").
		Select("invoices.invoice_date as invoice_date, invoices.invoice_number as invoice_no, invoice_items.equipment, invoice_items.model, invoice_items.description, invoice_items.quantity as qty, invoice_items.total_bhd as total_bhd").
		Joins("JOIN invoices ON invoices.id = invoice_items.invoice_id").
		Where("invoices.customer_id IN ? AND invoices.status NOT IN ?", customerIDs, []string{"Cancelled", "Void", "Proforma", "Draft"}).
		Order("invoices.invoice_date DESC").
		Limit(80).
		Scan(&lines)

	if len(lines) == 0 {
		return fmt.Sprintf("I checked sold line items for %s but found no invoice line-item records in current data.", customerName)
	}

	aggQty := make(map[string]float64)
	aggValue := make(map[string]float64)
	for _, ln := range lines {
		key := strings.TrimSpace(firstNonEmpty(ln.Equipment, ln.Model, ln.Description))
		if key == "" {
			continue
		}
		aggQty[key] += ln.Qty
		aggValue[key] += ln.TotalBHD
	}

	type itemAgg struct {
		Name  string
		Qty   float64
		Value float64
	}
	items := make([]itemAgg, 0, len(aggQty))
	for k, q := range aggQty {
		items = append(items, itemAgg{Name: k, Qty: q, Value: aggValue[k]})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Value > items[j].Value })

	summary := make([]string, 0, minInt(8, len(items)))
	for i, it := range items {
		if i >= 8 {
			break
		}
		summary = append(summary, fmt.Sprintf("- %s | Qty %.2f | %.3f BHD", it.Name, it.Qty, it.Value))
	}

	recent := make([]string, 0, minInt(6, len(lines)))
	for i, ln := range lines {
		if i >= 6 {
			break
		}
		label := strings.TrimSpace(firstNonEmpty(ln.Equipment, ln.Model, ln.Description))
		recent = append(recent, fmt.Sprintf("- %s | %s | %s | Qty %.2f | %.3f BHD",
			strings.TrimSpace(ln.InvoiceNo),
			ln.InvoiceDate.Format("2006-01-02"),
			label,
			ln.Qty,
			ln.TotalBHD,
		))
	}

	return fmt.Sprintf("Yes, I have line-item visibility for %s.\n\nTop sold items (by billed value):\n%s\n\nRecent sold lines:\n%s",
		customerName, strings.Join(summary, "\n"), strings.Join(recent, "\n"))
}

func (a *App) buildRevenueProjectionResponse() string {
	type yearRevenue struct {
		Year  string
		Total float64
	}
	var yearly []yearRevenue
	a.db.Table("invoices").
		Select("strftime('%Y', invoice_date) as year, COALESCE(SUM(grand_total_bhd),0) as total").
		Where("status NOT IN ?", []string{"Cancelled", "Void", "Proforma", "Draft"}).
		Group("strftime('%Y', invoice_date)").
		Order("year DESC").
		Limit(4).
		Scan(&yearly)

	currentYear := time.Now().Year()
	lastYear := currentYear - 1
	var lastYearRevenue float64
	var currentYTD float64
	for _, yr := range yearly {
		if yr.Year == fmt.Sprintf("%d", lastYear) {
			lastYearRevenue = yr.Total
		}
		if yr.Year == fmt.Sprintf("%d", currentYear) {
			currentYTD = yr.Total
		}
	}

	var activePipeline float64
	a.db.Table("offers").
		Where("stage NOT IN ?", []string{"Lost", "Cancelled", "Expired"}).
		Select("COALESCE(SUM(total_value_bhd),0)").
		Scan(&activePipeline)

	month := int(time.Now().Month())
	runRateProjection := lastYearRevenue
	if month > 0 && currentYTD > 0 {
		runRateProjection = (currentYTD / float64(month)) * 12.0
	}
	blendedProjection := (0.7 * runRateProjection) + (0.3 * (lastYearRevenue + 0.20*activePipeline))

	return fmt.Sprintf("Latest revenue projection (grounded from current records):\n\n- Last full year (%d) revenue: %.3f BHD\n- Current year (%d) YTD billed: %.3f BHD\n- Active offer pipeline: %.3f BHD\n\nProjection scenarios:\n- Run-rate projection: %.3f BHD\n- Blended projection (run-rate + pipeline conversion): %.3f BHD\n\nAssumption used: ~20%% pipeline conversion in-year for blended view.",
		lastYear, lastYearRevenue,
		currentYear, currentYTD,
		activePipeline,
		runRateProjection,
		blendedProjection,
	)
}

func (a *App) tryGroundedOfferDraftFastPath(message, customerHint, productHint string) (string, []ButlerAction, bool) {
	q := strings.ToLower(strings.TrimSpace(message))
	if q == "" {
		return "", nil, false
	}
	if !strings.Contains(q, "quantity") && !strings.Contains(q, "qty") {
		return "", nil, false
	}
	if !strings.Contains(q, "price") && !strings.Contains(q, "bhd") {
		return "", nil, false
	}
	if strings.TrimSpace(productHint) == "" {
		// Strip "for [customer]" phrase from query before product extraction to avoid
		// capturing "BAPCO for Probe FMP51" instead of just "Probe FMP51"
		productQuery := q
		if strings.TrimSpace(customerHint) != "" {
			custLower := strings.ToLower(strings.TrimSpace(customerHint))
			// Remove "for customer" or "to customer" as a unit
			for _, prep := range []string{"for ", "to "} {
				productQuery = strings.Replace(productQuery, prep+custLower, "", 1)
			}
		}
		productHint = inferProductHintFromText(productQuery)
	}
	if strings.TrimSpace(customerHint) == "" || strings.TrimSpace(productHint) == "" {
		return "", nil, false
	}

	qty, okQty := extractNumericToken(q, `(?i)(?:quantity|qty)\s*(?:is|=|:|would\s+be|to\s+be)?\s*([0-9]+(?:\.[0-9]+)?)`)
	price, okPrice := extractNumericToken(q, `(?i)(?:price(?:\s+per\s+unit)?|unit price)\s*(?:is|=|at|:|would\s+be|to\s+be)?\s*([0-9]+(?:\.[0-9]+)?)`)
	if !okPrice {
		price, okPrice = extractNumericToken(q, `(?i)([0-9]+(?:\.[0-9]+)?)\s*bhd`)
	}
	if !okQty || !okPrice || qty <= 0 || price <= 0 {
		return "", nil, false
	}

	customerIDs, displayName := a.resolveCustomerScope(customerHint)
	if len(customerIDs) == 0 {
		return "", nil, false
	}

	total := qty * price
	action := ButlerAction{
		Type:   "create_offer_draft",
		Target: "quotation",
		Label:  "Create offer draft",
		Data: map[string]interface{}{
			"customer_id":   customerIDs[0],
			"customer_name": displayName,
			"amount":        total,
			"line_items": []map[string]interface{}{
				{
					"equipment":      productHint,
					"description":    productHint,
					"quantity":       qty,
					"unit_price_bhd": price,
					"total_price":    total,
				},
			},
		},
	}

	msg := fmt.Sprintf("Prepared a grounded offer draft for %s:\n- Item: %s\n- Quantity: %.2f\n- Unit Price: %.3f BHD\n- Line Total: %.3f BHD\n\nRun the action to create the draft in the offers module.",
		displayName, productHint, qty, price, total)
	return msg, []ButlerAction{action}, true
}

func inferProductHintFromText(q string) string {
	re := regexp.MustCompile(`(?i)\bfor\s+(.+?)(?:\.\s*|,\s*|;\s*|\s+quantity|\s+qty|\s+price|\s+bhd|$)`)
	m := re.FindStringSubmatch(strings.TrimSpace(q))
	if len(m) != 2 {
		return ""
	}
	return strings.TrimSpace(strings.Trim(m[1], "."))
}

func extractNumericToken(text, pattern string) (float64, bool) {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0, false
	}
	var out float64
	_, err := fmt.Sscanf(m[1], "%f", &out)
	if err != nil {
		return 0, false
	}
	return out, true
}

func (a *App) collectAccountHandlers(customerIDs []string) []string {
	type contactRow struct {
		ContactName string
		JobTitle    string
		Email       string
		Phone       string
		IsPrimary   bool
	}

	var contacts []contactRow
	a.db.Table("customer_contacts").
		Select("contact_name, job_title, email, phone, is_primary_contact").
		Where("customer_id IN ?", customerIDs).
		Order("is_primary_contact DESC, contact_name ASC").
		Limit(20).
		Scan(&contacts)

	handlers := make([]string, 0, len(contacts))
	for _, c := range contacts {
		name := strings.TrimSpace(c.ContactName)
		if name == "" {
			continue
		}
		role := strings.TrimSpace(c.JobTitle)
		email := strings.TrimSpace(c.Email)
		if role != "" && email != "" {
			handlers = append(handlers, fmt.Sprintf("%s (%s, %s)", name, role, email))
			continue
		}
		if role != "" {
			handlers = append(handlers, fmt.Sprintf("%s (%s)", name, role))
			continue
		}
		if email != "" {
			handlers = append(handlers, fmt.Sprintf("%s (%s)", name, email))
			continue
		}
		handlers = append(handlers, name)
	}

	sort.Strings(handlers)
	return uniqueStrings(handlers)
}

func appendIfPresent(items []string, raw string) []string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return items
	}
	return append(items, value)
}

func uniqueStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func joinOrNone(items []string) string {
	if len(items) == 0 {
		return "none found in current records"
	}
	if len(items) > 8 {
		items = items[:8]
	}
	return strings.Join(items, "; ")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
