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

func (a *App) tryGroundedTaskCreationFastPath(intent Intent, message string) (string, bool) {
	if a == nil || a.db == nil {
		return "", false
	}

	assigneeRef, taskDetails, matched := parseGroundedTaskCreationRequest(intent, message)
	if !matched {
		return "", false
	}
	if strings.TrimSpace(assigneeRef) == "" || strings.TrimSpace(taskDetails) == "" {
		return "I can create that task, but I need both the assignee and the task details.", true
	}

	assignee := a.resolveEmployeeReference(assigneeRef)
	if assignee == nil || strings.TrimSpace(assignee.EntityID) == "" {
		return fmt.Sprintf("I understood that as a task creation request, but I could not resolve %s to an active employee.", assigneeRef), true
	}

	title := strings.TrimSpace(taskDetails)
	if title == "" {
		return "I understood the assignee, but the task title/details were blank after parsing the request.", true
	}

	taskType := "general"
	if strings.Contains(strings.ToLower(taskDetails), "follow up") || strings.Contains(strings.ToLower(taskDetails), "follow-up") {
		taskType = "follow_up"
	}

	var customerID *string
	if customerRef := inferCustomerReferenceFromQuery(Intent{}, strings.ToLower(taskDetails)); strings.TrimSpace(customerRef) != "" {
		if customerIDs, _ := a.resolveCustomerScope(customerRef); len(customerIDs) > 0 {
			customerID = &customerIDs[0]
		}
	}

	assigneeID := assignee.EntityID
	createdTask, err := a.CreateCollaborativeTask(TaskItem{
		Title:              title,
		Description:        taskDetails,
		TaskType:           taskType,
		Priority:           "medium",
		AssigneeEmployeeID: &assigneeID,
		CustomerID:         customerID,
	})
	if err != nil {
		return fmt.Sprintf("I understood that as a task request for %s, but I could not create it: %v", assignee.DisplayName, err), true
	}

	var notificationCount int64
	_ = a.db.Model(&Notification{}).
		Where("employee_id = ? AND source_type = ? AND source_id = ? AND notification_type = ?", assigneeID, "task", createdTask.ID, "task").
		Count(&notificationCount).Error

	response := fmt.Sprintf("Created the task %q for %s.", createdTask.Title, assignee.DisplayName)
	if notificationCount > 0 {
		response += " The assignment notification was queued as well."
	} else {
		response += " The task is created, but I could not confirm the assignment notification row yet."
	}
	if createdTask.CustomerID != nil && strings.TrimSpace(*createdTask.CustomerID) != "" {
		response += fmt.Sprintf(" I also linked it to customer %s.", *createdTask.CustomerID)
	}

	return response, true
}

func (a *App) tryGroundedWorkFastPath(intent Intent, message string) (string, bool) {
	if a == nil || a.db == nil {
		return "", false
	}

	q := strings.ToLower(strings.TrimSpace(message))
	if q == "" {
		return "", false
	}

	workSignals := []string{
		"task", "tasks", "workload", "notification", "notifications", "assigned",
		"assignee", "my work", "team board", "blocked", "open task", "open tasks",
	}
	matchedSignal := false
	for _, signal := range workSignals {
		if strings.Contains(q, signal) {
			matchedSignal = true
			break
		}
	}
	if !matchedSignal {
		return "", false
	}

	employeeRef := inferEmployeeReferenceFromQuery(intent, message)
	if employeeRef == "" {
		return "", false
	}

	resolution := a.resolveEmployeeReference(employeeRef)
	if resolution == nil || strings.TrimSpace(resolution.EntityID) == "" {
		return "", false
	}

	return a.buildEmployeeTaskOverviewResponse(resolution.EntityID, resolution.DisplayName, q), true
}

func (a *App) tryGroundedSupplierFastPath(intent Intent, message string) (string, bool) {
	if a == nil || a.db == nil {
		return "", false
	}

	q := strings.ToLower(strings.TrimSpace(message))
	if q == "" {
		return "", false
	}

	supplierSignals := []string{
		"supplier", "suppliers", "buy from", "bought from", "purchase", "purchases",
		"payment history", "supplier payment", "supplier payments", "issue", "issues",
	}
	matchedSignal := false
	for _, signal := range supplierSignals {
		if strings.Contains(q, signal) {
			matchedSignal = true
			break
		}
	}
	if !matchedSignal {
		return "", false
	}

	supplierRef := inferSupplierReferenceFromQuery(intent, message)
	if supplierRef == "" {
		return "", false
	}

	resolution := a.resolveSupplierReference(supplierRef)
	if resolution == nil || strings.TrimSpace(resolution.EntityID) == "" {
		return "", false
	}

	switch {
	case strings.Contains(q, "payment history") || strings.Contains(q, "supplier payment"):
		return a.buildSupplierPaymentHistoryResponse(resolution.EntityID, resolution.DisplayName), true
	case strings.Contains(q, "issue"):
		return a.buildSupplierIssueOverviewResponse(resolution.EntityID, resolution.DisplayName), true
	case strings.Contains(q, "buy from") || strings.Contains(q, "bought from") || strings.Contains(q, "purchase"):
		return a.buildSupplierPurchaseOverviewResponse(resolution.EntityID, resolution.DisplayName), true
	default:
		return "", false
	}
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

	if strings.Contains(q, "revenue projection") || strings.Contains(q, "revenue projections") || strings.Contains(q, "projected revenue") {
		return a.buildRevenueProjectionResponse(), true
	}

	customerRef := inferCustomerReferenceFromQuery(intent, q)
	if customerRef == "" && strings.TrimSpace(customerHint) != "" {
		customerRef = strings.TrimSpace(customerHint)
	}
	if customerRef == "" {
		return "", false
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

	if strings.Contains(q, "note") || strings.Contains(q, "notes") {
		return a.buildCustomerNotesResponse(customerIDs, displayName), true
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

func inferEmployeeReferenceFromQuery(intent Intent, message string) string {
	if strings.TrimSpace(intent.PersonName) != "" {
		return strings.TrimSpace(intent.PersonName)
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:tasks?\s+assigned\s+to|assigned\s+to|workload\s+for|notifications?\s+for|tasks?\s+for)\s+([A-Z][A-Za-z.\-]+(?:\s+[A-Z][A-Za-z.\-]+){0,2})`),
		regexp.MustCompile(`(?i)(?:notifications?\s+does|tasks?\s+does)\s+([A-Z][A-Za-z.\-]+(?:\s+[A-Z][A-Za-z.\-]+){0,2})\s+have`),
		regexp.MustCompile(`(?i)\b([A-Z][A-Za-z.\-]+(?:\s+[A-Z][A-Za-z.\-]+){0,2})'s\s+(?:tasks?|notifications?)\b`),
	}
	for _, pattern := range patterns {
		if m := pattern.FindStringSubmatch(message); len(m) > 1 {
			return strings.TrimSpace(m[1])
		}
	}

	return ""
}

func parseGroundedTaskCreationRequest(intent Intent, message string) (string, string, bool) {
	original := strings.TrimSpace(message)
	if original == "" {
		return "", "", false
	}

	lower := strings.ToLower(original)
	createSignals := []string{
		"create a task",
		"create task",
		"add a task",
		"add task",
		"make a task",
		"make task",
		"assign a task",
		"assign task",
	}
	matchedSignal := false
	for _, signal := range createSignals {
		if strings.Contains(lower, signal) {
			matchedSignal = true
			break
		}
	}
	if !matchedSignal {
		return "", "", false
	}

	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)(?:can you|could you|please|buddy)?\s*(?:create|add|make)\s+(?:a\s+)?task\s+for\s+([A-Za-z][A-Za-z.\-\s]{0,40}?)\s+(?:to\s+)?(.+)`),
		regexp.MustCompile(`(?i)(?:can you|could you|please|buddy)?\s*assign\s+(?:a\s+)?task\s+to\s+([A-Za-z][A-Za-z.\-\s]{0,40}?)\s+(?:to\s+)?(.+)`),
	}
	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(original); len(matches) == 3 {
			return strings.TrimSpace(matches[1]), sanitizeTaskCreationDetails(matches[2]), true
		}
	}

	employeeRef := inferEmployeeReferenceFromQuery(intent, original)
	if employeeRef == "" {
		employeeRef = strings.TrimSpace(intent.PersonName)
	}
	if employeeRef == "" {
		return "", "", true
	}

	employeePattern := regexp.MustCompile(`(?i)(?:for|to)\s+` + regexp.QuoteMeta(employeeRef) + `\s+(.*)$`)
	if matches := employeePattern.FindStringSubmatch(original); len(matches) == 2 {
		return employeeRef, sanitizeTaskCreationDetails(matches[1]), true
	}

	return employeeRef, "", true
}

func sanitizeTaskCreationDetails(details string) string {
	cleaned := strings.TrimSpace(details)
	if cleaned == "" {
		return ""
	}

	trailingMarkers := []string{
		"make sure",
		"ensure",
		"notify",
		"notification",
		"notifications",
	}
	lower := strings.ToLower(cleaned)
	for _, marker := range trailingMarkers {
		if idx := strings.Index(lower, marker); idx >= 0 {
			cleaned = strings.TrimSpace(cleaned[:idx])
			lower = strings.ToLower(cleaned)
		}
	}

	for _, sep := range []string{".", "!", "?"} {
		if idx := strings.Index(cleaned, sep); idx >= 0 {
			cleaned = strings.TrimSpace(cleaned[:idx])
		}
	}

	cleaned = strings.TrimSpace(strings.Trim(cleaned, " ,;:-"))
	if cleaned == "" {
		return ""
	}

	runes := []rune(cleaned)
	if len(runes) > 0 && runes[0] >= 'a' && runes[0] <= 'z' {
		runes[0] = runes[0] - 32
	}
	return string(runes)
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

func inferSupplierReferenceFromQuery(intent Intent, message string) string {
	if candidate := strings.TrimSpace(intent.EntityName); candidate != "" {
		lowerCandidate := strings.ToLower(candidate)
		if !strings.Contains(lowerCandidate, "issue") &&
			!strings.Contains(lowerCandidate, "payment") &&
			!strings.Contains(lowerCandidate, "history") &&
			!strings.Contains(lowerCandidate, "purchase") {
			return candidate
		}
	}

	q := strings.TrimSpace(message)
	if q == "" {
		return ""
	}

	re := regexp.MustCompile(`(?i)\b(?:about|from|for)\s+([^?]+)`)
	m := re.FindStringSubmatch(q)
	if len(m) != 2 {
		return ""
	}

	candidate := strings.TrimSpace(m[1])
	trimSuffixes := []string{
		"payment history",
		"payments",
		"payment",
		"issues",
		"issue",
		"purchase history",
		"purchases",
	}
	for _, suffix := range trimSuffixes {
		candidate = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(candidate), suffix))
	}
	candidate = strings.TrimSpace(candidate)
	if candidate == "" {
		return ""
	}
	return candidate
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

func (a *App) buildEmployeeTaskOverviewResponse(employeeID, employeeName, q string) string {
	type taskRow struct {
		Title       string
		Status      string
		Priority    string
		TaskType    string
		Description string
		DueDate     *time.Time
	}

	var tasks []taskRow
	a.db.Table("task_items").
		Select("title, status, priority, task_type, description, due_date").
		Where("assignee_employee_id = ?", employeeID).
		Order("updated_at DESC, created_at DESC").
		Limit(20).
		Scan(&tasks)

	openCount := 0
	completedCount := 0
	blockedCount := 0
	overdueCount := 0
	now := time.Now()
	lines := make([]string, 0, minInt(5, len(tasks)))
	for _, task := range tasks {
		status := strings.ToLower(strings.TrimSpace(task.Status))
		switch status {
		case "done", "completed", "closed":
			completedCount++
		default:
			openCount++
		}
		if status == "blocked" {
			blockedCount++
		}
		if task.DueDate != nil && task.DueDate.Before(now) && status != "done" && status != "completed" && status != "closed" {
			overdueCount++
		}
		if len(lines) < 5 {
			line := fmt.Sprintf("- %s | %s | %s", strings.TrimSpace(task.Title), strings.TrimSpace(task.Status), strings.TrimSpace(task.Priority))
			if task.DueDate != nil {
				line += " | due " + task.DueDate.Format("2006-01-02")
			}
			lines = append(lines, line)
		}
	}

	type notificationRow struct {
		Title      string
		Status     string
		SourceType string
		CreatedAt  time.Time
	}
	var notifications []notificationRow
	a.db.Table("notifications").
		Select("title, status, source_type, created_at").
		Where("employee_id = ?", employeeID).
		Order("created_at DESC").
		Limit(20).
		Scan(&notifications)

	unreadNotifications := 0
	notificationLines := make([]string, 0, 3)
	for _, notification := range notifications {
		if strings.EqualFold(strings.TrimSpace(notification.Status), "unread") {
			unreadNotifications++
		}
		if len(notificationLines) < 3 {
			notificationLines = append(notificationLines, fmt.Sprintf("- %s | %s | %s", strings.TrimSpace(notification.Title), strings.TrimSpace(notification.Status), notification.CreatedAt.Format("2006-01-02")))
		}
	}

	if len(tasks) == 0 && len(notifications) == 0 {
		return fmt.Sprintf("I checked the collaborative work records for %s and I cannot confirm any assigned tasks or notifications from current data.", employeeName)
	}

	builder := strings.Builder{}
	builder.WriteString(fmt.Sprintf("%s currently has %d active task(s)", employeeName, openCount))
	if completedCount > 0 {
		builder.WriteString(fmt.Sprintf(", %d completed task(s)", completedCount))
	}
	builder.WriteString(fmt.Sprintf(", %d blocked task(s), and %d overdue task(s).", blockedCount, overdueCount))
	builder.WriteString(fmt.Sprintf(" I also found %d unread notification(s).", unreadNotifications))

	if len(lines) > 0 {
		builder.WriteString("\n\nRecent assigned tasks:\n")
		builder.WriteString(strings.Join(lines, "\n"))
	}

	if strings.Contains(q, "notification") && len(notificationLines) > 0 {
		builder.WriteString("\n\nRecent notifications:\n")
		builder.WriteString(strings.Join(notificationLines, "\n"))
	}

	return builder.String()
}

func (a *App) resolveCustomerRecordIDs(customerIDs []string) []string {
	if len(customerIDs) == 0 {
		return nil
	}

	type row struct {
		ID string
	}
	var rows []row
	a.db.Table("customers").
		Select("id").
		Where("customer_id IN ?", customerIDs).
		Find(&rows)

	recordIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		if strings.TrimSpace(row.ID) != "" {
			recordIDs = append(recordIDs, strings.TrimSpace(row.ID))
		}
	}
	return uniqueStrings(recordIDs)
}

func (a *App) buildCustomerNotesResponse(customerIDs []string, customerName string) string {
	recordIDs := a.resolveCustomerRecordIDs(customerIDs)
	if len(recordIDs) == 0 {
		return fmt.Sprintf("I checked customer notes for %s and could not resolve a note record scope.", customerName)
	}

	type noteRow struct {
		NoteType  string
		Content   string
		CreatedAt time.Time
	}
	var notes []noteRow
	a.db.Table("entity_notes").
		Select("note_type, content, created_at").
		Where("entity_type = ? AND entity_id IN ?", "customer", recordIDs).
		Order("created_at DESC").
		Limit(10).
		Scan(&notes)

	if len(notes) == 0 {
		return fmt.Sprintf("I checked notes for %s and found no recorded customer notes.", customerName)
	}

	lines := make([]string, 0, minInt(6, len(notes)))
	for i, note := range notes {
		if i >= 6 {
			break
		}
		lines = append(lines, fmt.Sprintf("- %s | %s | %s",
			note.CreatedAt.Format("2006-01-02"),
			firstNonEmpty(strings.TrimSpace(note.NoteType), "general"),
			strings.TrimSpace(note.Content),
		))
	}

	return fmt.Sprintf("I found %d customer note(s) for %s.\n\nRecent notes:\n%s",
		len(notes), customerName, strings.Join(lines, "\n"))
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
		OfferNumber   string
		QuotationDate time.Time
		Stage         string
		TotalValueBHD float64
		QuoteType     string
		IssuedBy      string
		Attention     string
		CreatedBy     string
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

func (a *App) buildSupplierPaymentHistoryResponse(supplierID, supplierName string) string {
	type paymentRow struct {
		PaymentDate   time.Time
		AmountBHD     float64
		PaymentMethod string
		Reference     string
		InvoiceNumber string
	}
	var payments []paymentRow
	a.db.Table("supplier_payments").
		Select("payment_date, amount_bhd, payment_method, reference, invoice_number").
		Where("supplier_id = ?", supplierID).
		Order("payment_date DESC").
		Limit(12).
		Scan(&payments)

	type invoiceSummary struct {
		Count         int64
		TotalBHD      float64
		PendingCount  int64
		PendingAmount float64
	}
	var summary invoiceSummary
	a.db.Table("supplier_invoices").
		Select("COUNT(*) as count, COALESCE(SUM(total_bhd), 0) as total_bhd, SUM(CASE WHEN payment_status <> 'Paid' THEN 1 ELSE 0 END) as pending_count, COALESCE(SUM(CASE WHEN payment_status <> 'Paid' THEN total_bhd ELSE 0 END), 0) as pending_amount").
		Where("supplier_id = ?", supplierID).
		Scan(&summary)

	if len(payments) == 0 && summary.Count == 0 {
		return fmt.Sprintf("I checked supplier payment history for %s and found no recorded supplier invoices or payments.", supplierName)
	}

	lines := make([]string, 0, minInt(6, len(payments)))
	totalPaid := 0.0
	for i, payment := range payments {
		totalPaid += payment.AmountBHD
		if i >= 6 {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s | %s | %.3f BHD | %s | %s",
			payment.PaymentDate.Format("2006-01-02"),
			firstNonEmpty(strings.TrimSpace(payment.InvoiceNumber), "no invoice ref"),
			payment.AmountBHD,
			firstNonEmpty(strings.TrimSpace(payment.PaymentMethod), "payment method n/a"),
			firstNonEmpty(strings.TrimSpace(payment.Reference), "no reference"),
		))
	}

	paymentText := "No supplier payment rows found."
	if len(lines) > 0 {
		paymentText = strings.Join(lines, "\n")
	}

	return fmt.Sprintf("Here is the supplier payment history for %s.\n\nRecorded supplier invoices: %d totaling %.3f BHD\nPending supplier invoices: %d totaling %.3f BHD\nRecent supplier payments totaled %.3f BHD in the sampled history.\n\nRecent payments:\n%s",
		supplierName, summary.Count, summary.TotalBHD, summary.PendingCount, summary.PendingAmount, totalPaid, paymentText)
}

func (a *App) buildSupplierPurchaseOverviewResponse(supplierID, supplierName string) string {
	type invoiceRow struct {
		InvoiceNumber string
		InvoiceDate   time.Time
		TotalBHD      float64
		Status        string
	}
	var invoices []invoiceRow
	a.db.Table("supplier_invoices").
		Select("invoice_number, invoice_date, total_bhd, status").
		Where("supplier_id = ?", supplierID).
		Order("invoice_date DESC").
		Limit(12).
		Scan(&invoices)

	type lineRow struct {
		Description string
		Qty         float64
		TotalBHD    float64
	}
	var lines []lineRow
	a.db.Table("supplier_invoice_items").
		Select("supplier_invoice_items.description, supplier_invoice_items.quantity as qty, COALESCE(supplier_invoice_items.total_bhd, supplier_invoice_items.total_price, 0) as total_bhd").
		Joins("JOIN supplier_invoices ON supplier_invoices.id = supplier_invoice_items.supplier_invoice_id").
		Where("supplier_invoices.supplier_id = ?", supplierID).
		Order("supplier_invoices.invoice_date DESC").
		Limit(20).
		Scan(&lines)

	if len(invoices) == 0 && len(lines) == 0 {
		return fmt.Sprintf("I checked purchasing history for %s and found no recorded supplier invoices or line items.", supplierName)
	}

	total := 0.0
	invoiceLines := make([]string, 0, minInt(5, len(invoices)))
	for i, invoice := range invoices {
		total += invoice.TotalBHD
		if i >= 5 {
			continue
		}
		invoiceLines = append(invoiceLines, fmt.Sprintf("- %s | %s | %.3f BHD | %s",
			strings.TrimSpace(invoice.InvoiceNumber),
			invoice.InvoiceDate.Format("2006-01-02"),
			invoice.TotalBHD,
			strings.TrimSpace(invoice.Status),
		))
	}

	itemLines := make([]string, 0, minInt(6, len(lines)))
	for i, line := range lines {
		if i >= 6 {
			break
		}
		itemLines = append(itemLines, fmt.Sprintf("- %s | qty %.3f | %.3f BHD",
			firstNonEmpty(strings.TrimSpace(line.Description), "line item"),
			line.Qty,
			line.TotalBHD,
		))
	}

	itemsText := "No supplier line items captured."
	if len(itemLines) > 0 {
		itemsText = strings.Join(itemLines, "\n")
	}

	return fmt.Sprintf("Here is what we have bought from %s.\n\nRecorded supplier invoices: %d totaling %.3f BHD\n\nRecent supplier invoices:\n%s\n\nRecent purchased line items:\n%s",
		supplierName, len(invoices), total, strings.Join(invoiceLines, "\n"), itemsText)
}

func (a *App) buildSupplierIssueOverviewResponse(supplierID, supplierName string) string {
	type issueRow struct {
		OrderRef    string
		Description string
		Status      string
		CostBHD     float64
		CreatedAt   time.Time
	}
	var issues []issueRow
	a.db.Table("supplier_issues").
		Select("order_ref, description, status, cost_bhd, created_at").
		Where("supplier_id = ?", supplierID).
		Order("created_at DESC").
		Limit(10).
		Scan(&issues)

	if len(issues) == 0 {
		return fmt.Sprintf("I checked supplier issues for %s and found no active issue records.", supplierName)
	}

	openCount := 0
	lines := make([]string, 0, minInt(6, len(issues)))
	for i, issue := range issues {
		if !strings.EqualFold(strings.TrimSpace(issue.Status), "resolved") && !strings.EqualFold(strings.TrimSpace(issue.Status), "closed") {
			openCount++
		}
		if i >= 6 {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s | %s | %.3f BHD | %s | %s",
			firstNonEmpty(strings.TrimSpace(issue.OrderRef), "no order ref"),
			firstNonEmpty(strings.TrimSpace(issue.Status), "unknown"),
			issue.CostBHD,
			issue.CreatedAt.Format("2006-01-02"),
			strings.TrimSpace(issue.Description),
		))
	}

	return fmt.Sprintf("I found %d supplier issue record(s) for %s, with %d still open.\n\nRecent issues:\n%s",
		len(issues), supplierName, openCount, strings.Join(lines, "\n"))
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
