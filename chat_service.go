// ═══════════════════════════════════════════════════════════════════════════
// CHAT PERSISTENCE SERVICE - BUTLER AI CONVERSATIONS
//
// MISSION: Persistent multi-conversation chat history for Butler AI
//
// ARCHITECTURE:
//   1. Conversation Management (create, list, delete)
//   2. Message Storage (save user + assistant messages)
//   3. Context Loading (last N messages for continuity)
//   4. Integration with Butler AI (ChatWithButler pipeline)
//
// FEATURES:
//   - Multiple conversation threads
//   - Context-aware responses using history
//   - Automatic conversation creation
//   - Soft-delete for conversations
//   - Message ordering by timestamp
//
// Built with FULL STATE COMPLETION × MATHEMATICAL RIGOR × BUTLER AI INTEGRATION
// Day 196+ - Persistent Chat Infrastructure
// ═══════════════════════════════════════════════════════════════════════════

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
)

const butlerActionMetadataContractVersion = "v2.7"

var butlerStatusConstraintMap = map[string][]string{
	"opportunity": {
		"New", "Qualified", "Proposal", "Quoted", "Won", "Lost", "On Hold",
	},
	"purchase_order": {
		"Draft", "Pending Approval", "Approved", "Sent", "Acknowledged", "Partially Received",
		"Received", "Closed", "Cancelled",
	},
	"order": {
		"Draft", "Confirmed", "Processing", "InProgress", "Shipped", "PartiallyDelivered",
		"FullyDelivered", "Delivered", "Invoiced", "Complete", "Cancelled",
	},
	"rfq": {
		"RFQ Received", "Offer Sent", "Follow-up/Eval", "PO/LOI Received", "Order Placed",
		"In Process", "Delivered", "Closed (Payment)", "Closed (Lost)",
	},
	"costing_sheet": {
		"draft", "pending_approval", "approved", "rejected",
	},
	"offer": {
		"draft", "quoted", "sent", "accepted", "rejected", "won", "lost",
	},
	"follow_up": {
		"pending", "in_progress", "completed", "cancelled", "overdue",
	},
	"stock_adjustment": {
		"pending", "approved", "rejected",
	},
	"quotation": {
		"draft", "quoted", "sent", "accepted", "rejected", "won", "lost",
	},
}

var butlerActionTypeAlias = map[string]string{
	"create_offer_draft":      "create",
	"create_offer":            "create",
	"create_quotation":        "create",
	"createoffer":             "create",
	"create_order":            "create",
	"createorder":             "create",
	"create_purchase_order":   "create",
	"createfollowup":          "create",
	"create_followup":         "create",
	"create_follow_up":        "create",
	"create_followup_task":    "create",
	"create_follow_up_task":   "create",
	"create_stock_adjustment": "create",
	"create_stock":            "create",
	"create_stockadjustment":  "create",
	"create_customer":         "create",
	"createcustomer":          "create",
	"create_supplier":         "create",
	"createsupplier":          "create",
	"daily_briefing":          "daily_briefing",
	"dailybriefing":           "daily_briefing",
	"open":                    "open",
	"navigate":                "navigate",
	"analyze":                 "analyze",
	"fetch":                   "fetch",
}

var butlerActionTargetAlias = map[string]string{
	"offer_draft":       "offer",
	"offerdraft":        "offer",
	"po":                "purchase_order",
	"purchaseorder":     "purchase_order",
	"purchase-orders":   "purchase_order",
	"purchase_orders":   "purchase_order",
	"quotation":         "offer",
	"quote":             "offer",
	"quotations":        "offer",
	"follow-up":         "follow_up",
	"followup":          "follow_up",
	"customercontact":   "customer_contact",
	"customer_contact":  "customer_contact",
	"customer contact":  "customer_contact",
	"customer-contacts": "customer_contact",
	"suppliercontact":   "supplier_contact",
	"supplier_contact":  "supplier_contact",
	"supplier contact":  "supplier_contact",
	"supplier-contacts": "supplier_contact",
	"followup_task":     "follow_up",
	"follow_up_task":    "follow_up",
	"followup task":     "follow_up",
	"follow-up task":    "follow_up",
	"costingsheet":      "costing_sheet",
	"costings":          "costing_sheet",
	"costingsheets":     "costing_sheet",
	"supplierinvoice":   "supplier_invoice",
	"supplier_invoice":  "supplier_invoice",
	"stockadjustment":   "stock_adjustment",
	"stock-adjustment":  "stock_adjustment",
	"stockadjust":       "stock_adjustment",
	"stockadjustments":  "stock_adjustment",
	"stock_adjustments": "stock_adjustment",
	"customer":          "customer",
	"customers":         "customer",
	"supplier":          "supplier",
	"suppliers":         "supplier",
	"purchase_order":    "purchase_order",
	"offer":             "offer",
	"daily briefing":    "daily_briefing",
	"dailybriefing":     "daily_briefing",
	"tasks":             "follow_up",
	"task":              "follow_up",
	"follow up":         "follow_up",
	"opportunities":     "opportunity",
	"contacts":          "contact",
}

var butlerActionValidationRules = map[string][]string{
	"approve": {
		"entity_id",
	},
	"reject": {
		"entity_id",
	},
	"update": {
		"entity_id",
		"status_or_stage",
	},
	"create": {
		"target",
	},
	"analyze": {
		"target",
	},
	"fetch": {
		"target",
	},
	"navigate": {
		"target",
	},
	"open": {
		"target",
	},
	"daily_briefing": {
		"target",
	},
}

var butlerActionRequiresStatus = map[string]map[string]bool{
	"update": {
		"purchase_order":   true,
		"order":            true,
		"rfq":              true,
		"offer":            true,
		"costing_sheet":    true,
		"follow_up":        true,
		"stock_adjustment": true,
		"quotation":        true,
	},
}

func normalizeActionToken(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "-", "_")
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "\t", "_")
	value = strings.ReplaceAll(value, "\n", "_")
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return strings.Trim(value, "_")
}

func normalizeButlerActionType(actionType string) string {
	value := normalizeActionToken(actionType)
	if alias, ok := butlerActionTypeAlias[value]; ok {
		return alias
	}
	return value
}

func normalizeButlerActionTarget(target string) string {
	value := normalizeActionToken(target)
	if alias, ok := butlerActionTargetAlias[value]; ok {
		return alias
	}

	if value == "" {
		return value
	}

	return value
}

func normalizedButlerActionTypeTarget(actionType, target string) (string, string) {
	normalizedType := normalizeButlerActionType(actionType)
	normalizedTarget := normalizeButlerActionTarget(target)

	if normalizedType == "create" && (normalizedTarget == "quotation" || normalizedTarget == "quote") {
		normalizedTarget = "offer"
	}

	if normalizedType == "daily_briefing" {
		normalizedTarget = "daily_briefing"
	}

	return normalizedType, normalizedTarget
}

func isActionStatusAllowedForTarget(target, status string) bool {
	requiredStatus := strings.TrimSpace(strings.ToLower(normalizeButlerActionTarget(target)))
	actualStatus := strings.TrimSpace(strings.ToLower(status))
	if requiredStatus == "" || actualStatus == "" {
		return false
	}

	allowed, ok := butlerStatusConstraintMap[requiredStatus]
	if !ok {
		return true
	}

	for _, candidate := range allowed {
		normalize := func(text string) string {
			return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(text)), " ", ""), "-", ""), "_", "")
		}
		normalizedCandidate := normalize(candidate)
		normalizedActual := normalize(actualStatus)
		if normalizedCandidate == normalizedActual {
			return true
		}
	}
	return false
}

func isStatusRequiredForTarget(actionType, target string) bool {
	if actionType != "update" {
		return false
	}
	requires, ok := butlerActionRequiresStatus[actionType]
	if !ok {
		return false
	}
	return requires[normalizeButlerActionTarget(target)]
}

func isNumericLike(value string) bool {
	if strings.TrimSpace(value) == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if (value[i] < '0' || value[i] > '9') && value[i] != '.' && value[i] != '-' {
			return false
		}
	}
	return true
}

func getActionValueAsString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%v", typed), "0"), ".")
	case int:
		return fmt.Sprintf("%d", typed)
	case int64:
		return fmt.Sprintf("%d", typed)
	case uint:
		return fmt.Sprintf("%d", typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return ""
	}
}

func getActionValueAsStringSlice(value interface{}) []string {
	out := []string{}
	switch typed := value.(type) {
	case []string:
		for _, item := range typed {
			text := strings.TrimSpace(item)
			if text != "" {
				out = append(out, text)
			}
		}
	case []interface{}:
		for _, item := range typed {
			text := getActionValueAsString(item)
			if text != "" {
				out = append(out, text)
			}
		}
	case string:
		text := strings.TrimSpace(typed)
		if text != "" {
			out = append(out, text)
		}
	}
	return out
}

func isActionDataTrue(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true")
	default:
		return false
	}
}

func buildActionMetadataSummary(actions []map[string]interface{}) map[string]interface{} {
	summary := map[string]interface{}{
		"count":               len(actions),
		"ready_for_execution": 0,
		"needs_input":         0,
		"needs_approval":      0,
		"invalid_payload":     0,
		"targets":             []string{},
		"types":               []string{},
	}

	ready := 0
	needsInput := 0
	needsApproval := 0
	invalid := 0
	targetsSeen := map[string]struct{}{}
	typesSeen := map[string]struct{}{}

	for _, action := range actions {
		aType := normalizeActionToken(getActionValueAsString(action["type"]))
		target := normalizeActionToken(getActionValueAsString(action["target"]))
		if aType != "" {
			typesSeen[aType] = struct{}{}
		}
		if target != "" {
			targetsSeen[target] = struct{}{}
		}

		executionStatus := strings.ToLower(strings.TrimSpace(getActionValueAsString(action["execution_status"])))
		if executionStatus == "" {
			executionStatus = strings.ToLower(strings.TrimSpace(getActionValueAsString(action["executionStatus"])))
		}

		missingFields := getActionValueAsStringSlice(action["missing_fields"])
		if len(missingFields) == 0 {
			missingFields = getActionValueAsStringSlice(action["missingFields"])
		}
		invalidReason := strings.TrimSpace(getActionValueAsString(action["invalid_reason"]))
		if invalidReason == "" {
			invalidReason = strings.TrimSpace(getActionValueAsString(action["invalidReason"]))
		}

		switch executionStatus {
		case "ready_for_execution":
			ready++
		case "pending_approval", "needs_approval":
			needsApproval++
		case "needs_input":
			needsInput++
		case "invalid_payload":
			invalid++
		default:
			if len(missingFields) > 0 || invalidReason != "" {
				invalid++
			}
		}
	}

	targets := make([]string, 0, len(targetsSeen))
	for target := range targetsSeen {
		targets = append(targets, target)
	}

	types := make([]string, 0, len(typesSeen))
	for actionType := range typesSeen {
		types = append(types, actionType)
	}

	summary["ready_for_execution"] = ready
	summary["needs_input"] = needsInput
	summary["needs_approval"] = needsApproval
	summary["invalid_payload"] = invalid
	summary["targets"] = targets
	summary["types"] = types

	return summary
}

// ============================================================================
// CONVERSATION MANAGEMENT
// ============================================================================

// truncateTitle safely truncates a string to maxRunes using UTF-8 rune counting
func truncateTitle(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes])
}

// isDailyBriefingRequest detects briefing-style prompts routed through the chat UI.
func isDailyBriefingRequest(message string) bool {
	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}

	switch normalized {
	case "briefing", "daily briefing", "morning briefing", "today's briefing", "todays briefing":
		return true
	}

	return strings.Contains(normalized, "daily briefing") ||
		strings.Contains(normalized, "morning briefing") ||
		strings.Contains(normalized, "today briefing") ||
		strings.Contains(normalized, "brief me") ||
		strings.Contains(normalized, "give me a briefing")
}

// requiresFinancePrivilege enforces the finance gate only for truly finance-sensitive asks.
// Some sales/operations prompts can be misclassified as "financial" due wording like "sold" or "quarter".
func requiresFinancePrivilege(intent Intent, message string) bool {
	if intent.Domain != "financial" && intent.Domain != "risk" {
		return false
	}

	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return true
	}

	opsSignals := []string{
		"equipment sold",
		"what was sold",
		"sold to",
		"what equipment",
		"last quarter",
		"last 2 quarter",
		"last 2 quarters",
		"last two quarter",
		"last two quarters",
		"opportunity",
		"opportunities",
		"pipeline",
		"rfq",
		"offer",
		"quotation",
		"order status",
		"deliver",
		"service due",
		"invoices this quarter",
		"invoices this year",
		"invoice status",
		"invoice for",
		"invoices for",
	}
	for _, signal := range opsSignals {
		if strings.Contains(normalized, signal) {
			return false
		}
	}

	return true
}

// prepareButlerConversation ensures the conversation exists and persists the trigger message.
func (a *App) prepareButlerConversation(conversationID, title, message string) (string, []ChatMessage, error) {
	var conversation *Conversation
	var err error

	if conversationID == "" {
		conversation, err = a.CreateConversation(title)
		if err != nil {
			return "", nil, fmt.Errorf("failed to create conversation: %w", err)
		}
		conversationID = conversation.ID
	} else {
		if err := a.db.First(&conversation, "id = ? AND is_active = ?", conversationID, true).Error; err != nil {
			return "", nil, fmt.Errorf("conversation not found: %w", err)
		}
	}

	userMsg := ChatMessage{
		ConversationID: conversationID,
		Role:           "user",
		Content:        message,
		TokensUsed:     0,
	}

	if err := a.db.Create(&userMsg).Error; err != nil {
		log.Printf("❌ Failed to save user message: %v", err)
		return "", nil, fmt.Errorf("failed to save user message: %w", err)
	}

	var recentMessages []ChatMessage
	err = a.db.
		Where("conversation_id = ?", conversationID).
		Order("created_at DESC").
		Limit(20).
		Find(&recentMessages).Error
	if err != nil {
		log.Printf("⚠️ Failed to load conversation history: %v (continuing without history)", err)
		recentMessages = []ChatMessage{}
	}

	for i, j := 0, len(recentMessages)-1; i < j; i, j = i+1, j-1 {
		recentMessages[i], recentMessages[j] = recentMessages[j], recentMessages[i]
	}

	log.Printf("📚 Loaded %d messages from conversation history", len(recentMessages))
	return conversationID, recentMessages, nil
}

// persistButlerAssistantMessage stores the assistant response and updates conversation activity.
func (a *App) persistButlerAssistantMessage(conversationID, content string, tokensUsed int, actions []ButlerAction, responseMetadata *ButlerResponseMetadata) {
	normalizedActions := normalizeButlerActionList(actions)
	normalizedContent := normalizeAssistantMessageContent(content, normalizedActions)
	messageType, actionType, actionTarget, actionStatus, actionData, actionLabel := classifyActionMetadata(normalizedActions)
	richActionPayload := hydrateActionMetadataPayload(normalizedActions)
	actionSummary := buildActionMetadataSummary(richActionPayload)

	persistenceMetadata := map[string]interface{}{
		"persisted_at": time.Now().Format(time.RFC3339),
		"classification": map[string]interface{}{
			"action_type":   actionType,
			"action_target": actionTarget,
			"action_label":  actionLabel,
			"action_status": actionStatus,
			"action_count":  len(normalizedActions),
			"targets":       uniqueActionTargets(normalizedActions),
		},
		"model_context": nil,
	}
	persistenceMetadata["actions"] = richActionPayload
	persistenceMetadata["action_summary"] = actionSummary
	persistenceMetadata["action_contract"] = map[string]interface{}{
		"version":         butlerActionMetadataContractVersion,
		"supported_types": []string{"navigate", "fetch", "analyze", "update", "approve", "reject", "create", "open", "daily_briefing"},
		"supported_targets": []string{
			"purchase_order",
			"order",
			"rfq",
			"offer",
			"quotation",
			"follow_up",
			"supplier_invoice",
			"costing_sheet",
			"stock_adjustment",
			"customer",
			"report",
			"daily_briefing",
		},
		"status_constraints": butlerStatusConstraintMap,
		"required_fields_by_type": map[string][]string{
			"approve":                 {"entity_id"},
			"reject":                  {"entity_id"},
			"update":                  {"entity_id", "status"},
			"create_offer_draft":      {"customer_id|customer_name", "line_items|amount"},
			"create_offer":            {"customer_id|customer_name", "line_items|amount"},
			"create_order":            {"order_number", "customer_id|customer_name", "amount"},
			"create_followup":         {"customer_id|customer_name", "title"},
			"create_follow_up":        {"customer_id|customer_name", "title"},
			"create_stock_adjustment": {"inventory_item_id|item_id", "reason", "variance|system_quantity|physical_quantity"},
			"create": {
				"target",
				"required fields based on target (offer=customer/line_items/amount, follow_up=customer/title, order=order_number/customer/amount, stock_adjustment=item_id/reason/variance)",
			},
			"analyze":        {"target"},
			"fetch":          {"target"},
			"navigate":       {"target"},
			"open":           {"target"},
			"daily_briefing": {"target"},
		},
		"requires_context_key": "entity_id|id|offer_id|order_id|purchase_order_id|supplier_invoice_id|costing_sheet_id|stock_adjustment_id|customer_id|follow_up_id|rfq_id|invoice_id",
	}
	if responseMetadata != nil {
		persistenceMetadata["model_context"] = map[string]interface{}{
			"used_backend":    responseMetadata.UsedBackend,
			"requested_model": responseMetadata.RequestedModel,
			"used_model":      responseMetadata.UsedModel,
			"fallback_reason": responseMetadata.FallbackReason,
			"finance_access":  responseMetadata.FinanceDataAccess,
			"context_mode":    responseMetadata.ContextMode,
			"data_coverage":   responseMetadata.DataCoverage,
			"generated_at":    responseMetadata.GeneratedAt,
			"error":           responseMetadata.Error,
		}
	}

	actionMetadataJSON, _ := json.Marshal(persistenceMetadata)

	assistantMsg := ChatMessage{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        normalizedContent,
		TokensUsed:     tokensUsed,
		MessageType:    messageType,
		ActionType:     actionType,
		ActionTarget:   actionTarget,
		ActionData:     actionData,
		ActionStatus:   actionStatus,
		ActionLabel:    actionLabel,
		ActionMetadata: string(actionMetadataJSON),
	}

	if err := a.db.Create(&assistantMsg).Error; err != nil {
		log.Printf("⚠️ Failed to save assistant message: %v", err)
	}

	a.db.Model(&Conversation{}).Where("id = ?", conversationID).Update("last_msg_at", time.Now())
}

func normalizeAssistantMessageContent(content string, actions []ButlerAction) string {
	trimmed := strings.TrimSpace(content)
	if trimmed != "" {
		return trimmed
	}

	if summary := summarizeButlerActions(actions); summary != "" {
		return summary
	}

	return "I processed your request, but I do not have a replay-safe text response for this turn."
}

func summarizeButlerActions(actions []ButlerAction) string {
	normalized := normalizeButlerActionList(actions)
	if len(normalized) == 0 {
		return ""
	}

	labels := make([]string, 0, len(normalized))
	seen := make(map[string]struct{})
	for _, action := range normalized {
		label := strings.TrimSpace(action.Label)
		if label == "" {
			label = inferActionLabel(action.Type, action.Target)
		}
		if label == "" {
			continue
		}
		if _, exists := seen[label]; exists {
			continue
		}
		seen[label] = struct{}{}
		labels = append(labels, label)
		if len(labels) == 3 {
			break
		}
	}

	if len(labels) == 0 {
		return ""
	}
	if len(labels) == 1 {
		return fmt.Sprintf("I prepared a suggested action: %s.", labels[0])
	}

	return fmt.Sprintf("I prepared %d suggested actions: %s.", len(normalized), strings.Join(labels, "; "))
}

func extractStoredButlerActions(msg ChatMessage) []ButlerAction {
	if strings.TrimSpace(msg.ActionMetadata) != "" {
		var persisted struct {
			Actions []struct {
				Type   string      `json:"type"`
				Target string      `json:"target"`
				Label  string      `json:"label"`
				Data   interface{} `json:"data"`
			} `json:"actions"`
		}
		if err := json.Unmarshal([]byte(msg.ActionMetadata), &persisted); err == nil && len(persisted.Actions) > 0 {
			actions := make([]ButlerAction, 0, len(persisted.Actions))
			for _, action := range persisted.Actions {
				actions = append(actions, ButlerAction{
					Type:   action.Type,
					Target: action.Target,
					Label:  action.Label,
					Data:   action.Data,
				})
			}
			return normalizeButlerActionList(actions)
		}
	}

	if strings.TrimSpace(msg.ActionType) != "" || strings.TrimSpace(msg.ActionTarget) != "" || strings.TrimSpace(msg.ActionLabel) != "" {
		return normalizeButlerActionList([]ButlerAction{{
			Type:   msg.ActionType,
			Target: msg.ActionTarget,
			Label:  msg.ActionLabel,
		}})
	}

	return nil
}

func replaySafeChatMessageContent(msg ChatMessage) string {
	trimmed := strings.TrimSpace(msg.Content)
	if trimmed != "" {
		return trimmed
	}
	if msg.Role != "assistant" {
		return ""
	}
	return normalizeAssistantMessageContent("", extractStoredButlerActions(msg))
}

func buildReplayMessage(msg ChatMessage) (map[string]interface{}, bool) {
	content := replaySafeChatMessageContent(msg)
	if content == "" {
		return nil, false
	}

	return map[string]interface{}{
		"role":    msg.Role,
		"content": content,
	}, true
}

func classifyActionMetadata(actions []ButlerAction) (string, string, string, string, string, string) {
	if len(actions) == 0 {
		return "assistant", "", "", "", "", ""
	}

	// Primary action is still preserved for compatibility,
	// while full action payload is stored as a JSON array for auditability.
	primary := actions[0]
	actionType := normalizeButlerActionType(primary.Type)
	actionTarget := normalizeButlerActionTarget(primary.Target)
	actionLabel := strings.TrimSpace(primary.Label)

	needsApproval := false
	hasWorkflow := false
	for _, action := range actions {
		aType := normalizeButlerActionType(action.Type)
		target := normalizeButlerActionTarget(action.Target)
		switch aType {
		case "approve", "reject":
			needsApproval = true
		case "create", "create_offer_draft", "create_follow_up", "create_followup", "analyze", "fetch", "navigate", "open", "update", "daily_briefing":
			hasWorkflow = true
		case "create_offer", "create_order", "create_stock_adjustment":
			hasWorkflow = true
		}

		if actionType == "" && aType != "" {
			actionType = aType
		}
		if actionTarget == "" && target != "" {
			actionTarget = target
		}
		if actionLabel == "" {
			actionLabel = strings.TrimSpace(action.Label)
		}
	}

	if actionLabel == "" {
		actionLabel = inferActionLabel(actionType, actionTarget)
	}

	serializedActions := hydrateActionMetadataPayload(actions)
	summary := buildActionMetadataSummary(serializedActions)
	invalidPayload := 0
	needsInput := 0
	needsApprovalCount := 0
	readyForExecution := 0
	if raw, ok := summary["invalid_payload"].(int); ok {
		invalidPayload = raw
	}
	if raw, ok := summary["needs_input"].(int); ok {
		needsInput = raw
	}
	if raw, ok := summary["needs_approval"].(int); ok {
		needsApprovalCount = raw
	}
	if raw, ok := summary["ready_for_execution"].(int); ok {
		readyForExecution = raw
	}

	var status string
	switch {
	case invalidPayload > 0:
		status = "invalid_payload"
	case needsInput > 0:
		status = "needs_input"
	case needsApproval || needsApprovalCount > 0:
		status = "needs_approval"
	case hasWorkflow && readyForExecution > 0:
		status = "pending_execution"
	case hasWorkflow && readyForExecution == 0:
		status = "needs_input"
	default:
		status = "suggested"
	}

	actionData := ""
	if len(serializedActions) > 0 {
		primaryRecord := serializedActions[0]
		if rawPayload, err := json.Marshal(primaryRecord); err == nil {
			actionData = string(rawPayload)
		} else if primary.Data != nil {
			if primaryBytes, err := json.Marshal(primary.Data); err == nil {
				actionData = string(primaryBytes)
			}
		}
	}

	return "assistant_actionable", actionType, actionTarget, status, actionData, actionLabel
}

func normalizeButlerActionList(actions []ButlerAction) []ButlerAction {
	normalized := make([]ButlerAction, 0, len(actions))
	for _, action := range actions {
		nType, nTarget := normalizedButlerActionTypeTarget(action.Type, action.Target)
		label := strings.TrimSpace(action.Label)
		if label == "" {
			label = inferActionLabel(nType, nTarget)
		}
		normalized = append(normalized, ButlerAction{
			Type:   nType,
			Target: nTarget,
			Label:  label,
			Data:   action.Data,
		})
	}
	return normalized
}

func uniqueActionTargets(actions []ButlerAction) []string {
	seen := map[string]struct{}{}
	targets := make([]string, 0, len(actions))
	for _, action := range actions {
		target := normalizeButlerActionTarget(action.Target)
		if target == "" {
			continue
		}
		if _, ok := seen[target]; ok {
			continue
		}
		seen[target] = struct{}{}
		targets = append(targets, target)
	}
	return targets
}

func hydrateActionMetadataPayload(actions []ButlerAction) []map[string]interface{} {
	payload := make([]map[string]interface{}, 0, len(actions))
	if len(actions) == 0 {
		return payload
	}

	validateCreateRequirements := func(actionType, target string, data map[string]interface{}) []string {
		missing := []string{}
		target = normalizeButlerActionTarget(target)

		if actionType != "create" &&
			actionType != "create_offer_draft" &&
			actionType != "create_offer" &&
			actionType != "create_follow_up" &&
			actionType != "create_followup" &&
			actionType != "create_order" &&
			actionType != "create_stock_adjustment" {
			return missing
		}

		if target == "" {
			missing = append(missing, "target")
			return missing
		}

		if target == "offer" {
			if extractActionDataString(data, "customer_id", "customer_name", "customer") == "" {
				missing = append(missing, "customer")
			}
			lineItems := getActionValueAsStringSlice(data["line_items"])
			if len(lineItems) == 0 {
				lineItems = getActionValueAsStringSlice(data["items"])
			}
			if len(lineItems) == 0 &&
				!actionDataNumericLike(data, "amount", "grand_total", "total", "total_amount", "amount_bhd") {
				missing = append(missing, "line items or amount")
			}
		}

		if target == "follow_up" {
			if extractActionDataString(data, "customer_id", "customer_name", "customer") == "" {
				missing = append(missing, "customer")
			}
			if extractActionDataString(data, "title", "subject") == "" {
				missing = append(missing, "follow-up title")
			}
		}

		if target == "order" {
			if extractActionDataString(data, "order_number", "reference", "order_no") == "" {
				missing = append(missing, "order number")
			}
			if extractActionDataString(data, "customer_id", "customer_name", "customer") == "" {
				missing = append(missing, "customer")
			}
			if !actionDataNumericLike(data, "amount", "grand_total", "total", "total_amount", "amount_bhd") {
				missing = append(missing, "amount")
			}
		}

		if target == "opportunity" {
			if extractActionDataString(data, "customer_id", "customer_name", "customer") == "" {
				missing = append(missing, "customer")
			}
			if extractActionDataString(data, "title", "project", "opportunity_name", "name") == "" {
				missing = append(missing, "project/title")
			}
		}

		if target == "customer_contact" || target == "contact" {
			if extractActionDataString(data, "customer_id", "customer_name", "customer") == "" &&
				extractActionDataString(data, "supplier_id", "supplier_name", "supplier") == "" {
				missing = append(missing, "customer or supplier")
			}
			if extractActionDataString(data, "contact_name", "name", "person", "primary_contact") == "" {
				missing = append(missing, "contact name")
			}
		}

		if target == "supplier_contact" {
			if extractActionDataString(data, "supplier_id", "supplier_name", "supplier") == "" {
				missing = append(missing, "supplier")
			}
			if extractActionDataString(data, "contact_name", "name", "person", "primary_contact") == "" {
				missing = append(missing, "contact name")
			}
		}

		if target == "stock_adjustment" {
			if extractActionDataString(data, "inventory_item_id", "item_id") == "" {
				missing = append(missing, "inventory item id")
			}
			if extractActionDataString(data, "reason") == "" {
				missing = append(missing, "reason")
			}
			hasVariance := actionDataNumericLike(data, "variance")
			hasSystem := actionDataNumericLike(data, "system_quantity")
			hasPhysical := actionDataNumericLike(data, "physical_quantity")
			if !hasVariance && !(hasSystem && hasPhysical) {
				missing = append(missing, "variance/system_quantity/physical_quantity")
			}
		}

		return missing
	}

	for _, action := range actions {
		actionType := normalizeButlerActionType(action.Type)
		target := normalizeButlerActionTarget(action.Target)
		label := strings.TrimSpace(action.Label)
		if label == "" {
			label = inferActionLabel(actionType, target)
		}

		data := coerceActionDataMap(action.Data)
		allowedStatuses := []string{}
		if statuses, ok := butlerStatusConstraintMap[target]; ok {
			allowedStatuses = append([]string{}, statuses...)
		}
		entityID := extractActionDataString(data, "entity_id", "id", "uuid", "offer_id", "order_id", "purchase_order_id", "supplier_invoice_id", "costing_sheet_id", "stock_adjustment_id")
		targetStatus := extractActionDataString(data, "status", "stage", "new_status", "new_stage", "target_status", "approved_to", "to")
		reason := extractActionDataString(data, "reason", "rejection_reason", "resolution", "note", "notes")

		executionStatus := "pending_review"
		missingFields := []string{}
		invalidReason := ""
		switch actionType {
		case "approve", "reject":
			if entityID != "" {
				executionStatus = "needs_approval"
			} else {
				executionStatus = "invalid_payload"
				missingFields = append(missingFields, "entity id")
				invalidReason = "entity id is required"
			}
		case "update":
			if strings.TrimSpace(target) == "" {
				executionStatus = "needs_input"
				missingFields = append(missingFields, "target")
				invalidReason = "target is required"
				break
			}
			if isStatusRequiredForTarget(actionType, target) && targetStatus == "" {
				executionStatus = "needs_input"
				missingFields = append(missingFields, "status")
				invalidReason = "status/stage is required for this target"
				break
			}
			if target == "opportunity" {
				comment := extractActionDataString(data, "comment", "notes", "description")
				ownerNotes := extractActionDataString(data, "owner_notes", "ownerNotes")
				if strings.TrimSpace(entityID) == "" {
					executionStatus = "needs_input"
					missingFields = append(missingFields, "entity id")
					invalidReason = "entity id is required"
					break
				}
				if targetStatus == "" && comment == "" && ownerNotes == "" {
					executionStatus = "needs_input"
					missingFields = append(missingFields, "stage/status or comment/owner_notes")
					invalidReason = "opportunity update requires stage/status or notes"
					break
				}
				if targetStatus != "" && !isActionStatusAllowedForTarget(target, targetStatus) {
					executionStatus = "invalid_payload"
					invalidReason = fmt.Sprintf("invalid status/stage '%s' for target '%s'", targetStatus, target)
					break
				}
				executionStatus = "ready_for_execution"
				break
			}
			if !isActionStatusAllowedForTarget(target, targetStatus) {
				executionStatus = "invalid_payload"
				invalidReason = fmt.Sprintf("invalid status/stage '%s' for target '%s'", targetStatus, target)
				break
			}
			if strings.TrimSpace(entityID) == "" {
				executionStatus = "needs_input"
				missingFields = append(missingFields, "entity id")
				invalidReason = "entity id is required"
			} else {
				executionStatus = "ready_for_execution"
			}
		case "create", "create_offer_draft", "create_offer", "create_follow_up", "create_followup", "create_order", "create_stock_adjustment":
			missingFields = validateCreateRequirements(actionType, target, data)
			if len(missingFields) > 0 {
				executionStatus = "needs_input"
				invalidReason = "missing required create fields"
			} else {
				executionStatus = "ready_for_execution"
			}
		case "analyze", "fetch", "navigate", "open", "daily_briefing":
			if strings.TrimSpace(target) == "" {
				executionStatus = "needs_input"
				missingFields = append(missingFields, "target")
				invalidReason = "target is required"
				break
			}
			executionStatus = "ready_for_execution"
		default:
			executionStatus = "invalid_payload"
			invalidReason = "unsupported action type"
		}

		record := map[string]interface{}{
			"type":                  actionType,
			"target":                target,
			"label":                 label,
			"requires_approval":     actionType == "approve" || actionType == "reject",
			"requires_confirmation": executionStatus == "needs_approval",
			"entity_id":             entityID,
			"status_or_stage":       targetStatus,
			"reason":                reason,
			"missing_fields":        missingFields,
			"required_fields":       append([]string{}, missingFields...),
			"status_constraints":    allowedStatuses,
			"invalid_reason":        invalidReason,
			"execution_status":      executionStatus,
			"runtime_verification": map[string]interface{}{
				"ready_for_execution":   executionStatus == "ready_for_execution",
				"metadata_fields_found": len(data) > 0,
				"requires_status":       isStatusRequiredForTarget(actionType, target),
			},
			"data": data,
		}
		payload = append(payload, record)
	}

	return payload
}

func actionDataNumericLike(data map[string]interface{}, keys ...string) bool {
	for _, key := range keys {
		value := extractActionDataString(data, key)
		if isNumericLike(value) {
			return true
		}
	}
	return false
}

func coerceActionDataMap(raw interface{}) map[string]interface{} {
	data := map[string]interface{}{}
	switch typed := raw.(type) {
	case map[string]interface{}:
		for k, v := range typed {
			data[k] = v
		}
	case map[string]string:
		for k, v := range typed {
			data[k] = v
		}
	case map[interface{}]interface{}:
		for rk, rv := range typed {
			key, ok := rk.(string)
			if !ok {
				continue
			}
			data[key] = rv
		}
	default:
		if raw != nil {
			data["raw"] = raw
		}
	}
	return data
}

func extractActionDataString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if rawValue, ok := data[key]; ok {
			switch typed := rawValue.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			case float64:
				if typed == float64(int64(typed)) {
					return fmt.Sprintf("%.0f", typed)
				}
			case int:
				return fmt.Sprintf("%d", typed)
			case int64:
				return fmt.Sprintf("%d", typed)
			case uint:
				return fmt.Sprintf("%d", typed)
			}
		}
	}
	return ""
}

// buildDailyBriefingSystemPromptWithHistory adds briefing-specific guidance on top of the standard prompt.
func buildDailyBriefingSystemPromptWithHistory(context map[string]interface{}, regime map[string]interface{}, history []ChatMessage) string {
	basePrompt := buildMistralSystemPromptWithHistory(context, regime, history)
	return basePrompt + ` 

DAILY BRIEFING MODE:
- Act as an executive morning briefing for PH Trading WLL.
- Prioritize the current operating picture, what changed since the last briefing, and what needs attention today.
- Structure the answer with short sections: Snapshot, Risks, Priorities, Suggested Actions.
- Prefer concrete numbers, named entities, and date-aware observations when the context contains them.
- If there is insufficient data for a section, say so briefly instead of inventing details.
- Keep the tone concise, operational, and decision-oriented.
- Include 2-5 actionable follow-ups when useful, using the existing [ACTIONS] block format.`
}

// runButlerBriefingFlow executes the briefing conversation flow using the existing chat persistence pipeline.
func (a *App) runButlerBriefingFlow(conversationID, triggerMessage, conversationTitle string) (ChatResponse, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return ChatResponse{}, err
	}

	if triggerMessage == "" {
		triggerMessage = "Generate the daily briefing for today."
	}

	conversationID, recentMessages, err := a.prepareButlerConversation(conversationID, conversationTitle, triggerMessage)
	if err != nil {
		return ChatResponse{}, err
	}

	intent := classifyIntent(triggerMessage)
	log.Printf("🧠 Briefing intent classified: domain=%s entity=%q confidence=%.2f complex=%v",
		intent.Domain, intent.EntityName, intent.Confidence, intent.IsComplex)
	hasFinanceAccess := a.requirePermission("finance:view") == nil

	context := a.buildFullContext(intent)
	regime := a.calculateSystemRegime()
	context["system_regime"] = regime
	context["briefing_mode"] = true
	context["briefing_date"] = time.Now().Format("2006-01-02")
	context["briefing_time"] = time.Now().Format(time.RFC3339)

	model := mistralModelLarge
	if !intent.IsComplex {
		model = mistralModelSmall
	}
	usedBackend := "Mistral"

	systemPrompt := buildDailyBriefingSystemPromptWithHistory(context, regime, recentMessages)
	mistralMessages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
	}

	historyLimit := 10
	startIdx := 0
	if len(recentMessages) > historyLimit {
		startIdx = len(recentMessages) - historyLimit
	}
	for _, msg := range recentMessages[startIdx:] {
		mistralMessages = append(mistralMessages, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	if len(recentMessages) == 0 || recentMessages[len(recentMessages)-1].Content != triggerMessage {
		mistralMessages = append(mistralMessages, map[string]interface{}{
			"role":    "user",
			"content": triggerMessage,
		})
	}

	var aiResponse string
	var modelErr error
	var aiErr error
	fallbackReason := ""
	requestedModel := getAIMLModelID()
	usedModel := ""
	if aimlKey := getAIMLAPIKey(); aimlKey != "" {
		aiResponse, usedModel, aiErr = callAIMLWithFallback(aimlKey, systemPrompt, triggerMessage)
		usedBackend = "AIML/Grok"
		if aiErr != nil {
			log.Printf("⚠️ AIML API error in daily briefing, falling back to Mistral: %v", aiErr)
			fallbackReason = aiErr.Error()
			aiResponse, modelErr = callMistralWithMessages(model, mistralMessages)
			usedBackend = "Mistral (fallback)"
		}
	} else {
		aiResponse, modelErr = callMistralWithMessages(model, mistralMessages)
	}
	if modelErr != nil {
		log.Printf("❌ Mistral API error in daily briefing: %v", modelErr)
		errReason := modelErr.Error()
		errorMsg := fmt.Sprintf("Daily briefing pipeline error: %v. Please try again.", modelErr)
		metadata := buildButlerMetadata(intent, context, hasFinanceAccess, usedBackend, requestedModel, usedModel, errReason, modelErr)
		metadata.ContextMode = "briefing"
		a.persistButlerAssistantMessage(conversationID, errorMsg, 0, []ButlerAction{}, &metadata)

		return ChatResponse{
			Response:       errorMsg,
			ConversationID: conversationID,
			Actions:        []ButlerAction{},
			Confidence:     0.0,
			TokensUsed:     0,
			Metadata:       metadata,
		}, nil
	}

	butlerResponse := parseMistralResponse(aiResponse, context, intent)
	metadata := buildButlerMetadata(intent, context, hasFinanceAccess, usedBackend, requestedModel, usedModel, fallbackReason, nil)
	metadata.ContextMode = "briefing"
	tokensUsed := len(aiResponse) / 4
	a.persistButlerAssistantMessage(conversationID, butlerResponse.Message, tokensUsed, butlerResponse.Actions, &metadata)

	log.Printf("🗞️ Butler daily briefing via %s: %s (confidence: %.2f, actions: %d, tokens: %d)",
		usedBackend, butlerTruncate(butlerResponse.Message, 60), butlerResponse.Confidence,
		len(butlerResponse.Actions), tokensUsed)

	return ChatResponse{
		Response:       butlerResponse.Message,
		ConversationID: conversationID,
		Actions:        butlerResponse.Actions,
		Confidence:     butlerResponse.Confidence,
		TokensUsed:     tokensUsed,
		Metadata:       metadata,
	}, nil
}

// GenerateDailyBriefing creates a dedicated Butler daily briefing for the current user.
func (a *App) GenerateDailyBriefing(conversationID string) (ChatResponse, error) {
	return a.runButlerBriefingFlow(conversationID, "Generate the daily briefing for today.", "Daily Briefing")
}

// CreateConversation creates a new conversation with optional title
func (a *App) CreateConversation(title string) (*Conversation, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return nil, err
	}

	if title == "" {
		title = "New Conversation"
	}

	conversation := &Conversation{
		Title:     title,
		IsActive:  true,
		LastMsgAt: time.Now(),
		Base:      Base{CreatedBy: a.getCurrentUserID()},
	}

	if err := a.db.Create(conversation).Error; err != nil {
		log.Printf("❌ Failed to create conversation: %v", err)
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	log.Printf("💬 Created conversation: id=%s title=%q", conversation.ID, conversation.Title)
	return conversation, nil
}

// ListConversations returns all active conversations ordered by last message time
func (a *App) ListConversations() ([]Conversation, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return nil, err
	}

	var conversations []Conversation
	currentActor := a.getCurrentUserID()

	query := a.db.Where("is_active = ?", true)
	if !a.currentSessionHasPermission("*") {
		query = query.Where("created_by = ?", currentActor)
	}

	err := query.Order("last_msg_at DESC").Limit(100).Find(&conversations).Error

	if err != nil {
		log.Printf("❌ Failed to list conversations: %v", err)
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}

	log.Printf("📋 Listed %d conversations", len(conversations))
	return conversations, nil
}

// GetConversationMessages returns all messages for a conversation in chronological order
func (a *App) GetConversationMessages(conversationID string) ([]ChatMessage, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return nil, err
	}

	// Verify conversation exists and is active (ownership check)
	var conv Conversation
	query := a.db.Where("id = ? AND is_active = ?", conversationID, true)
	if !a.currentSessionHasPermission("*") {
		query = query.Where("created_by = ?", a.getCurrentUserID())
	}
	if err := query.First(&conv).Error; err != nil {
		return nil, fmt.Errorf("conversation not found or deleted")
	}

	var messages []ChatMessage

	err := a.db.
		Where("conversation_id = ?", conversationID).
		Order("created_at ASC").
		Find(&messages).Error

	if err != nil {
		log.Printf("❌ Failed to get messages for conversation %s: %v", conversationID, err)
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}

	for i := range messages {
		messages[i].Content = replaySafeChatMessageContent(messages[i])
	}

	log.Printf("📨 Retrieved %d messages for conversation %s", len(messages), conversationID)
	return messages, nil
}

// DeleteConversation removes a conversation and all its messages.
// Legacy rows can be soft-deleted/inactive already, so we use an unscoped cleanup path.
func (a *App) DeleteConversation(conversationID string) error {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not available")
	}
	if strings.TrimSpace(conversationID) == "" {
		return fmt.Errorf("conversation id is required")
	}
	currentActor := a.getCurrentUserID()
	isAdmin := a.currentSessionHasPermission("*")

	var exists int64
	existsQuery := a.db.Unscoped().Model(&Conversation{}).Where("id = ?", conversationID)
	if !isAdmin {
		existsQuery = existsQuery.Where("created_by = ?", currentActor)
	}
	if err := existsQuery.Count(&exists).Error; err != nil {
		return fmt.Errorf("failed to verify conversation: %w", err)
	}
	if exists == 0 {
		// Legacy fallback: some rows may be referenced by title from older UI payloads.
		titleQuery := a.db.Unscoped().Model(&Conversation{}).Where("title = ?", conversationID)
		if !isAdmin {
			titleQuery = titleQuery.Where("created_by = ?", currentActor)
		}
		if err := titleQuery.Count(&exists).Error; err != nil {
			return fmt.Errorf("failed to verify conversation title: %w", err)
		}
		if exists > 0 {
			var legacy Conversation
			legacyQuery := a.db.Unscoped().Where("title = ?", conversationID)
			if !isAdmin {
				legacyQuery = legacyQuery.Where("created_by = ?", currentActor)
			}
			if err := legacyQuery.Order("last_msg_at DESC").First(&legacy).Error; err == nil {
				conversationID = legacy.ID
			}
		}
	}
	if exists == 0 {
		return fmt.Errorf("conversation not found")
	}

	// Hard-delete messages + conversation in a single transaction (no orphan risk)
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("conversation_id = ?", conversationID).Delete(&ChatMessage{}).Error; err != nil {
			return fmt.Errorf("failed to delete messages: %w", err)
		}
		deleteQuery := tx.Unscoped().Where("id = ?", conversationID)
		if !isAdmin {
			deleteQuery = deleteQuery.Where("created_by = ?", currentActor)
		}
		if err := deleteQuery.Delete(&Conversation{}).Error; err != nil {
			return fmt.Errorf("failed to delete conversation: %w", err)
		}
		return nil
	}); err != nil {
		log.Printf("❌ Failed to delete conversation %s: %v", conversationID, err)
		return err
	}

	log.Printf("🗑️ Deleted conversation %s and all its messages", conversationID)
	return nil
}

// PurgeAllConversations force-removes all Butler conversations and messages.
func (a *App) PurgeAllConversations() error {
	if err := a.requirePermission("*"); err != nil {
		return err
	}
	if a.db == nil {
		return fmt.Errorf("database not available")
	}
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("1 = 1").Delete(&ChatMessage{}).Error; err != nil {
			return fmt.Errorf("failed to purge chat messages: %w", err)
		}
		if err := tx.Unscoped().Where("1 = 1").Delete(&Conversation{}).Error; err != nil {
			return fmt.Errorf("failed to purge conversations: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	log.Printf("🧹 Purged all conversations and chat messages")
	return nil
}

// ============================================================================
// PERSISTENT CHAT WITH CONTEXT
// ============================================================================

// ChatResponse represents a chat response with conversation ID
type ChatResponse struct {
	Response       string                 `json:"response"`
	ConversationID string                 `json:"conversation_id"`
	Actions        []ButlerAction         `json:"actions"`
	Confidence     float64                `json:"confidence"`
	TokensUsed     int                    `json:"tokens_used"`
	Metadata       ButlerResponseMetadata `json:"metadata"`
}

// GetButlerDailyBriefing generates a daily briefing using the existing
// persistent chat pipeline so the result lands in the conversation history.
func (a *App) GetButlerDailyBriefing(conversationID string) (ChatResponse, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return ChatResponse{}, err
	}

	briefingPrompt := `Give me a Butler daily briefing for today.

Use the proactive briefing, action items, risk signals, service follow-ups, follow-up tasks, and quick captures already available in context.

Format:
1. Top priorities today
2. Collections / finance watchpoints
3. Service / field follow-up watchpoints
4. Approvals or workflow bottlenecks
5. Recommended next 3 actions

Be concise, factual, and do not invent data.`

	return a.ChatWithButlerPersistent(conversationID, briefingPrompt)
}

// ChatWithButlerPersistent handles chat with conversation persistence
func (a *App) ChatWithButlerPersistent(conversationID, message string) (ChatResponse, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return ChatResponse{}, err
	}

	if isDailyBriefingRequest(message) {
		return a.runButlerBriefingFlow(conversationID, message, "Daily Briefing")
	}

	// 1. Ensure we have a conversation ID and persist the user message.
	var err error
	var recentMessages []ChatMessage
	if conversationID == "" {
		// Create new conversation with title from first message (UTF-8 safe truncation)
		title := truncateTitle(message, 50)
		if utf8.RuneCountInString(message) > 50 {
			title = title + "..."
		}
		conversationID, recentMessages, err = a.prepareButlerConversation("", title, message)
		if err != nil {
			return ChatResponse{}, err
		}
	} else {
		conversationID, recentMessages, err = a.prepareButlerConversation(conversationID, "", message)
		if err != nil {
			return ChatResponse{}, err
		}
	}

	// 4. Build context-aware messages for Mistral API
	// Classify intent for the current message
	intent := classifyIntent(message)
	log.Printf("🧠 Intent classified: domain=%s entity=%q confidence=%.2f complex=%v",
		intent.Domain, intent.EntityName, intent.Confidence, intent.IsComplex)
	currentUserRole := a.GetCurrentUserRole()
	licenseRole := a.GetLicenseRole()
	hasFinanceAccess := a.requirePermission("finance:view") == nil

	if requiresFinancePrivilege(intent, message) {
		if !hasFinanceAccess {
			log.Printf("🔒 Butler financial query blocked (persistent): user role=%s license role=%s lacks finance:view", currentUserRole, licenseRole)
			errMsg := "Financial data access requires manager or admin privileges.\n\nYou can still ask me about:\n• Customer information\n• Supplier details\n• Order status\n• Operations and pipeline\n\nFor financial reports and metrics, please contact your manager."
			blockedMetadata := buildButlerMetadata(intent, map[string]interface{}{
				"blocked_domain": intent.Domain,
				"user_role":      currentUserRole,
				"license_role":   licenseRole,
			}, false, "permission_gate", "", "", "finance_access_blocked", nil)
			blockedMetadata.ContextMode = "blocked"
			a.persistButlerAssistantMessage(conversationID, errMsg, 0, []ButlerAction{}, &blockedMetadata)
			return ChatResponse{
				Response:       errMsg,
				ConversationID: conversationID,
				Actions:        []ButlerAction{},
				Confidence:     0.95,
				TokensUsed:     0,
				Metadata:       blockedMetadata,
			}, nil
		}
	}

	customerHint := inferCustomerHintFromRecentMessages(recentMessages, message)
	productHint := inferProductHintFromRecentMessages(recentMessages, message)
	offerSeed := inferOfferDraftSeedFromRecentMessages(recentMessages)
	offerInput := strings.TrimSpace(message)
	if offerSeed != "" {
		offerInput = strings.TrimSpace(offerSeed + " " + offerInput)
	}
	if offerMsg, offerActions, offerHandled := a.tryGroundedOfferDraftFastPath(offerInput, customerHint, productHint); offerHandled {
		metadata := buildButlerMetadata(intent, map[string]interface{}{
			"fast_path":     "grounded_offer_draft",
			"customer_hint": customerHint,
			"product_hint":  productHint,
		}, hasFinanceAccess, "grounded_sql", "", "", "", nil)
		a.persistButlerAssistantMessage(conversationID, offerMsg, 0, offerActions, &metadata)
		return ChatResponse{
			Response:       offerMsg,
			ConversationID: conversationID,
			Actions:        offerActions,
			Confidence:     0.99,
			TokensUsed:     0,
			Metadata:       metadata,
		}, nil
	}
	if groundedReply, handled := a.tryGroundedTaskCreationFastPath(intent, message); handled {
		metadata := buildButlerMetadata(intent, map[string]interface{}{
			"fast_path": "grounded_task_create",
		}, hasFinanceAccess, "grounded_sql", "", "", "", nil)
		a.persistButlerAssistantMessage(conversationID, groundedReply, 0, []ButlerAction{}, &metadata)
		return ChatResponse{
			Response:       groundedReply,
			ConversationID: conversationID,
			Actions:        []ButlerAction{},
			Confidence:     0.99,
			TokensUsed:     0,
			Metadata:       metadata,
		}, nil
	}
	if groundedReply, handled := a.tryGroundedWorkFastPath(intent, message); handled {
		metadata := buildButlerMetadata(intent, map[string]interface{}{
			"fast_path": "grounded_work",
		}, hasFinanceAccess, "grounded_sql", "", "", "", nil)
		a.persistButlerAssistantMessage(conversationID, groundedReply, 0, []ButlerAction{}, &metadata)
		return ChatResponse{
			Response:       groundedReply,
			ConversationID: conversationID,
			Actions:        []ButlerAction{},
			Confidence:     0.98,
			TokensUsed:     0,
			Metadata:       metadata,
		}, nil
	}
	if groundedReply, handled := a.tryGroundedSupplierFastPath(intent, message); handled {
		metadata := buildButlerMetadata(intent, map[string]interface{}{
			"fast_path": "grounded_supplier",
		}, hasFinanceAccess, "grounded_sql", "", "", "", nil)
		a.persistButlerAssistantMessage(conversationID, groundedReply, 0, []ButlerAction{}, &metadata)
		return ChatResponse{
			Response:       groundedReply,
			ConversationID: conversationID,
			Actions:        []ButlerAction{},
			Confidence:     0.98,
			TokensUsed:     0,
			Metadata:       metadata,
		}, nil
	}
	if groundedReply, handled := a.tryGroundedCustomerFastPathWithHint(intent, message, customerHint); handled {
		metadata := buildButlerMetadata(intent, map[string]interface{}{
			"fast_path":     "grounded_customer",
			"customer_hint": customerHint,
		}, hasFinanceAccess, "grounded_sql", "", "", "", nil)
		a.persistButlerAssistantMessage(conversationID, groundedReply, 0, []ButlerAction{}, &metadata)
		return ChatResponse{
			Response:       groundedReply,
			ConversationID: conversationID,
			Actions:        []ButlerAction{},
			Confidence:     0.98,
			TokensUsed:     0,
			Metadata:       metadata,
		}, nil
	}

	// Check if this is a report request
	if isReportRequest(message) {
		butlerResp, err := a.handleReportRequest(message, intent)
		if err != nil {
			return ChatResponse{}, err
		}

		// Save assistant response
		a.persistButlerAssistantMessage(conversationID, butlerResp.Message, 100, butlerResp.Actions, &butlerResp.Metadata)

		// Update conversation timestamp
		a.db.Model(&Conversation{}).Where("id = ?", conversationID).Update("last_msg_at", time.Now())

		return ChatResponse{
			Response:       butlerResp.Message,
			ConversationID: conversationID,
			Actions:        butlerResp.Actions,
			Confidence:     butlerResp.Confidence,
			TokensUsed:     100,
			Metadata:       butlerResp.Metadata,
		}, nil
	}

	// 5. Build business context based on intent
	context := a.buildFullContext(intent)

	// 6. Calculate system regime
	regime := a.calculateSystemRegime()
	context["system_regime"] = regime

	// 7. Select model based on complexity
	model := mistralModelSmall
	if intent.IsComplex {
		model = mistralModelLarge
	}
	usedBackend := "Mistral"

	// 8. Build system prompt with context, regime, AND conversation history
	systemPrompt := buildMistralSystemPromptWithHistory(context, regime, recentMessages)

	// 9. Build messages array for Mistral API (with conversation context)
	mistralMessages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
	}

	// Add recent conversation messages for context (last 10 exchanges max)
	historyLimit := 10
	startIdx := 0
	if len(recentMessages) > historyLimit {
		startIdx = len(recentMessages) - historyLimit
	}
	currentUserMessageInHistory := false
	for _, msg := range recentMessages[startIdx:] {
		replayMsg, ok := buildReplayMessage(msg)
		if !ok {
			continue
		}
		mistralMessages = append(mistralMessages, replayMsg)
		if msg.Role == "user" && strings.TrimSpace(msg.Content) == strings.TrimSpace(message) {
			currentUserMessageInHistory = true
		}
	}

	// Current user message is already at the end from recent history
	// If not in history yet (edge case), add it
	if !currentUserMessageInHistory {
		mistralMessages = append(mistralMessages, map[string]interface{}{
			"role":    "user",
			"content": message,
		})
	}

	// 10. Call Mistral API with full context
	var aiResponse string
	var modelErr error
	var aiErr error
	fallbackReason := ""
	requestedModel := getAIMLModelID()
	usedModel := ""
	if aimlKey := getAIMLAPIKey(); aimlKey != "" {
		aiResponse, usedModel, aiErr = callAIMLWithMessages(aimlKey, mistralMessages)
		usedBackend = "AIML/Grok"
		if aiErr != nil {
			log.Printf("⚠️ AIML API error in persistent chat, falling back to Mistral: %v", aiErr)
			aiResponse, modelErr = callMistralWithMessages(model, mistralMessages)
			usedBackend = "Mistral (fallback)"
		}
	} else {
		aiResponse, modelErr = callMistralWithMessages(model, mistralMessages)
	}
	if aiErr != nil {
		fallbackReason = aiErr.Error()
	}
	if modelErr != nil {
		log.Printf("❌ Mistral API error: %v", modelErr)

		// Save error response as assistant message
		errorMsg := fmt.Sprintf("Intelligence pipeline error: %v. Please try again.", modelErr)
		metadata := buildButlerMetadata(intent, context, hasFinanceAccess, usedBackend, requestedModel, usedModel, fallbackReason, modelErr)
		a.persistButlerAssistantMessage(conversationID, errorMsg, 0, []ButlerAction{}, &metadata)

		return ChatResponse{
			Response:       errorMsg,
			ConversationID: conversationID,
			Actions:        []ButlerAction{},
			Confidence:     0.0,
			TokensUsed:     0,
			Metadata:       metadata,
		}, nil // Return nil error so frontend shows the message
	}

	// 11. Parse response (extract actions, clean message)
	butlerResponse := parseMistralResponse(aiResponse, context, intent)
	butlerResponse.Metadata = buildButlerMetadata(intent, context, hasFinanceAccess, usedBackend, requestedModel, usedModel, fallbackReason, nil)

	// 12. Save assistant response
	tokensUsed := len(aiResponse) / 4 // Rough estimate: 1 token ≈ 4 characters
	a.persistButlerAssistantMessage(conversationID, butlerResponse.Message, tokensUsed, butlerResponse.Actions, &butlerResponse.Metadata)

	log.Printf("🤖 Butler responded via %s: %s (confidence: %.2f, actions: %d, tokens: %d)",
		usedBackend, butlerTruncate(butlerResponse.Message, 60), butlerResponse.Confidence,
		len(butlerResponse.Actions), tokensUsed)

	return ChatResponse{
		Response:       butlerResponse.Message,
		ConversationID: conversationID,
		Actions:        butlerResponse.Actions,
		Confidence:     butlerResponse.Confidence,
		TokensUsed:     tokensUsed,
		Metadata:       butlerResponse.Metadata,
	}, nil
}

func inferCustomerHintFromRecentMessages(messages []ChatMessage, currentMessage string) string {
	if len(messages) == 0 {
		q0 := strings.ToLower(strings.TrimSpace(currentMessage))
		switch {
		case strings.Contains(q0, "bapco"):
			return "BAPCO"
		case strings.Contains(q0, "alba"):
			return "ALBA"
		case strings.Contains(q0, "al ezzel"), strings.Contains(q0, "alezzel"):
			return "Al Ezzel"
		}
		return ""
	}
	q := strings.ToLower(strings.TrimSpace(currentMessage))
	switch {
	case strings.Contains(q, "bapco"):
		return "BAPCO"
	case strings.Contains(q, "alba"):
		return "ALBA"
	case strings.Contains(q, "al ezzel"), strings.Contains(q, "alezzel"):
		return "Al Ezzel"
	}
	if !strings.Contains(q, "them") && !strings.Contains(q, "they") && !strings.Contains(q, "those") && !strings.Contains(q, "their") {
		return ""
	}
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		text := strings.ToLower(messages[i].Content)
		switch {
		case strings.Contains(text, "bapco"):
			return "BAPCO"
		case strings.Contains(text, "alba"):
			return "ALBA"
		case strings.Contains(text, "al ezzel"), strings.Contains(text, "alezzel"):
			return "Al Ezzel"
		}
	}
	return ""
}

func inferOfferDraftSeedFromRecentMessages(messages []ChatMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		text := strings.ToLower(strings.TrimSpace(messages[i].Content))
		if text == "" {
			continue
		}
		if (strings.Contains(text, "offer") || strings.Contains(text, "quote") || strings.Contains(text, "quotation")) &&
			(strings.Contains(text, "quantity") || strings.Contains(text, "qty") || strings.Contains(text, "price") || strings.Contains(text, "bhd")) {
			return messages[i].Content
		}
	}
	return ""
}

func inferProductHintFromRecentMessages(messages []ChatMessage, currentMessage string) string {
	if len(messages) == 0 {
		return ""
	}
	q := strings.ToLower(strings.TrimSpace(currentMessage))
	if !strings.Contains(q, "quantity") && !strings.Contains(q, "qty") && !strings.Contains(q, "price") {
		return ""
	}
	re := regexp.MustCompile(`(?i)\boffer\s+to\s+.+?\s+for\s+(.+)$`)
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role != "user" {
			continue
		}
		msg := strings.TrimSpace(messages[i].Content)
		m := re.FindStringSubmatch(msg)
		if len(m) == 2 {
			product := strings.TrimSpace(strings.Trim(m[1], ".!?"))
			if product != "" {
				return product
			}
		}
	}
	return ""
}

// ============================================================================
// MISTRAL API WITH FULL MESSAGE HISTORY
// ============================================================================

// callMistralWithMessages calls Mistral API with full message array (system + history + current)
func callMistralWithMessages(model string, messages []map[string]interface{}) (string, error) {
	requestBody := map[string]interface{}{
		"model":       model,
		"messages":    messages,
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

	// Get API key from environment/config
	apiKey := getMistralAPIKey()

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 45 * time.Second}
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

// ============================================================================
// SYSTEM PROMPT WITH CONVERSATION HISTORY
// ============================================================================

// buildMistralSystemPromptWithHistory creates PRD-style prompt with conversation awareness
func buildMistralSystemPromptWithHistory(context map[string]interface{}, regime map[string]interface{}, history []ChatMessage) string {
	// Serialize context to JSON for injection
	contextJSON, _ := json.MarshalIndent(context, "", "  ")

	// Build regime guidance
	regimeGuidance := buildRegimeGuidance(regime)

	// Build conversation summary if history exists
	conversationContext := ""
	if len(history) > 0 {
		conversationContext = fmt.Sprintf("\n\nCONVERSATION CONTEXT:\nThis is a continuing conversation. The user has previously discussed:\n")

		// Summarize last 3 exchanges for context
		summaryLimit := 6 // 3 exchanges (user + assistant)
		startIdx := 0
		if len(history) > summaryLimit {
			startIdx = len(history) - summaryLimit
		}

		for i := startIdx; i < len(history); i++ {
			msg := history[i]
			role := "User"
			if msg.Role == "assistant" {
				role = "Assistant"
			}
			// Sanitize history content before injecting into system prompt (P1: prompt injection prevention)
			preview := sanitizeForPrompt(replaySafeChatMessageContent(msg))
			preview = contextCleanupPatterns.ReplaceAllString(preview, "")
			if strings.TrimSpace(preview) == "" {
				continue
			}
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			conversationContext += fmt.Sprintf("- %s: %s\n", role, preview)
		}

		conversationContext += "\nMaintain continuity with this conversation history. Reference previous topics when relevant."
	}

	// Sanitize context data to prevent prompt injection from database-sourced values
	contextStr := sanitizeForPrompt(string(contextJSON))
	contextStr = contextCleanupPatterns.ReplaceAllString(contextStr, "")
	const maxContextChars = 200000
	if len(contextStr) > maxContextChars {
		contextStr = contextStr[:maxContextChars] + "\n... [context truncated at 200K chars]"
	}

	prompt := fmt.Sprintf(`You are Butler, the business intelligence AI for PH Trading WLL — a Bahrain-based process instrumentation company.
You have access to connected business data in this system and should answer only from that evidence.
Do not invent entities, people, products, projects, invoice numbers, payment events, competitor notes, or business assertions not present in context.

CURRENT SYSTEM DATE:
- Treat today as %s.
- Treat the current operating year as %d.
- Do not describe explicitly requested periods in %d as "future" unless the period is actually after today.

DATA-GROUNDING RULES:
- Answer only from facts present in context JSON and keys listed below.
- If a requested data point is not present, state: "I cannot confirm this from current data" and offer what you do know.
- For period-scoped questions (quarter/month/year), use 'customer_period_summary' when provided.
- For company-wide quarter/month/year questions, use 'business_period_summary' when provided and treat it as authoritative.
- If period summary is empty, do not infer from lifetime data and present period-specific result only.

COMPANY PROFILE:
- Industry: Process Instrumentation & Control Systems (Bahrain)
- Currency: BHD (Bahraini Dinar, 3 decimal places = fils)
- Divisions: PH Trading (primary) + AHS Trading (secondary)
- VAT Rate: 10%%

FULL DATA ACCESS — you have live data on:
• CUSTOMERS: live customer accounts — profiles, grades (A/B/C/D), payment behavior, credit limits, outstanding AR, invoices, orders, contacts, CRM notes, follow-up tasks, active offers, AR risk tiers
• CUSTOMER ACTIVITY: Dormant customers (no orders 6+ months), new customers this year, lapsed customers, retention signals
• CUSTOMER PROFITABILITY: Revenue and gross margin by customer — top earners, lowest margin accounts
• SUPPLIERS: live supplier records — profiles, PO history, invoice matching, payment history, GRN quality stats
• SUPPLIER PERFORMANCE: Average delivery lead times, QC pass/fail rates by supplier
• SALES PIPELINE: live opportunities, offers (RFQ → Quotation → Won/Lost/Expired), costing sheets, win rates, competition markers
• OFFER EXPIRY: Active quotes approaching ValidityDate — this week / this month / next 60 days
• COMPETITION: Win/loss trend, top lost-deal reasons, and supplier/product competitiveness signals
• ORDERS & INVOICES: live orders and customer invoices, including status, line items, and margins
• PROCUREMENT: 45 POs (multi-currency EUR/USD/BHD), 3-way matching (PO/GRN/Invoice), GRN quality
• DELIVERY: 155 delivery notes — Prepared/Dispatched/Delivered, serial traceability
• SERVICE DUE: Serial-tracked installations nearing calibration, warranty, or annual-service milestones
• FINANCE: AR/AP, bank balances, FX rates, monthly revenue trends, payment predictions
• DSO ANALYSIS: Days Sales Outstanding per customer — who pays fast vs slow
• BANKING: Bank statements, reconciliation status, outstanding/stale/bounced cheques
• CREDIT NOTES: Draft/Issued/Applied pipeline, outstanding credits
• PRODUCTS: 19 active products — catalog, pricing, margins, order frequency, serial tracking
• FORECASTING: Weighted pipeline by stage, opportunities closing in 60 days, payment predictions, win rates
• RISK: Overdue AR/AP, credit-blocked customers, expiring offers, system alerts
• ACTION ITEMS: Prioritized list — overdue invoices, expiring offers, stuck deliveries, pending approvals
• STRUCTURED MEMORY: Recent entity notes, pending follow-ups, and tactical quick captures
• WORK MANAGEMENT: Employees, collaborative tasks, task comments, and task notifications
• DATABASE ACCESS: Cross-module record retrieval covering notes, offers and line items, orders and line items, invoices, supplier invoices, payments, bank statements, expenses, and related operational records

COMPLETE BUSINESS DATA:
%s

%s%s

YOUR CAPABILITIES:
1. Answer ANY question about the business — customers, suppliers, finance, operations, pipeline, risk
2. FUTURE PROSPECTS — use pipeline data, payment predictions, and win probability from context
3. STRATEGIC ANALYSIS — recommend actions, identify patterns, surface insights from data trends
4. FORECASTING — estimate revenue using pipeline × close probability, analyze payment timing risk
5. RELATIONSHIP ANALYSIS — cross-reference customers, orders, offers, and payments
6. DAILY PRIORITIZATION — tell the team what to focus on today
7. CUSTOMER HEALTH — identify dormant accounts, churn risk, new customer opportunities
8. COMPETITIVE INTELLIGENCE — analyze win/loss patterns and pricing vs margin trade-offs
9. SUPPLIER BENCHMARKING — compare lead times and QC reliability
10. PAYMENT BEHAVIOR — DSO by customer, predict who will pay late, recommend collection priority
11. SERVICE FOLLOW-UPS — identify equipment due for calibration, warranty, or service
12. DOCUMENT READINESS — verify customer/contact context and missing fields before quotation drafting
13. PROACTIVE BRIEFING — summarize what needs attention across collections, operations, service, approvals
14. CONTINUITY — use structured memory and conversation history to maintain context
15. CREATE CUSTOMERS — create new customer records from natural language (name, type, grade, city, contact)
16. CREATE SUPPLIERS — create new supplier records from natural language (name, type, country, contact, brands)

RESPONSE GUIDELINES:
- Always cite specific numbers from context data when available
- For forward-looking questions, clearly distinguish between actual data and projections
- Be as detailed or as brief as the question requires — match depth to complexity
- If data is not in context, say so and offer what you do know
- Suggest next actions only when supported by context
- For strategic recommendations, ground them in actual data provided
- Never invent contact people, invoice numbers, project names, or payment events absent from context
- Treat entity_resolution, customer_data, supplier_data, employee_context, quotation_precheck, butler_memory, proactive_briefing, work_data, and database_access as higher-trust sources
- If entity_resolution indicates ambiguous=true, ask a short clarification question
- Reference previous conversation topics when relevant to maintain continuity
- Do not claim a PDF or document was generated unless context includes a real file path

ACTION FORMAT CONTRACT (include when recommending next steps):
%s

Available targets:
%s
`, time.Now().Format("2006-01-02"), time.Now().Year(), time.Now().Year(), contextStr, regimeGuidance, conversationContext, buildButlerActionContractPrompt(), strings.Join(buildButlerActionTargetAliases(), ", "))

	return prompt
}
