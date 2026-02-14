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
	"time"
	"unicode/utf8"
)

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

	err := a.db.
		Where("is_active = ?", true).
		Order("last_msg_at DESC").
		Limit(100).
		Find(&conversations).Error

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
	if err := a.db.First(&conv, "id = ? AND is_active = ?", conversationID, true).Error; err != nil {
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

	log.Printf("📨 Retrieved %d messages for conversation %s", len(messages), conversationID)
	return messages, nil
}

// DeleteConversation soft-deletes a conversation and all its messages
func (a *App) DeleteConversation(conversationID string) error {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return err
	}

	// Verify conversation exists before deleting (ownership check)
	var conv Conversation
	if err := a.db.First(&conv, "id = ? AND is_active = ?", conversationID, true).Error; err != nil {
		return fmt.Errorf("conversation not found")
	}

	// Soft-delete all messages in this conversation
	if err := a.db.Where("conversation_id = ?", conversationID).Delete(&ChatMessage{}).Error; err != nil {
		log.Printf("❌ Failed to delete messages for conversation %s: %v", conversationID, err)
		return fmt.Errorf("failed to delete messages: %w", err)
	}

	// Soft-delete the conversation
	if err := a.db.Delete(&Conversation{}, "id = ?", conversationID).Error; err != nil {
		log.Printf("❌ Failed to delete conversation %s: %v", conversationID, err)
		return fmt.Errorf("failed to delete conversation: %w", err)
	}

	log.Printf("🗑️ Deleted conversation %s and all its messages", conversationID)
	return nil
}

// ============================================================================
// PERSISTENT CHAT WITH CONTEXT
// ============================================================================

// ChatResponse represents a chat response with conversation ID
type ChatResponse struct {
	Response       string         `json:"response"`
	ConversationID string         `json:"conversation_id"`
	Actions        []ButlerAction `json:"actions"`
	Confidence     float64        `json:"confidence"`
	TokensUsed     int            `json:"tokens_used"`
}

// ChatWithButlerPersistent handles chat with conversation persistence
func (a *App) ChatWithButlerPersistent(conversationID, message string) (ChatResponse, error) {
	if err := a.requirePermission("intelligence:chat"); err != nil {
		return ChatResponse{}, err
	}

	// 1. Ensure we have a conversation ID
	var conversation *Conversation
	var err error

	if conversationID == "" {
		// Create new conversation with title from first message (UTF-8 safe truncation)
		title := truncateTitle(message, 50)
		if utf8.RuneCountInString(message) > 50 {
			title = title + "..."
		}
		conversation, err = a.CreateConversation(title)
		if err != nil {
			return ChatResponse{}, fmt.Errorf("failed to create conversation: %w", err)
		}
		conversationID = conversation.ID
	} else {
		// Verify conversation exists
		if err := a.db.First(&conversation, "id = ? AND is_active = ?", conversationID, true).Error; err != nil {
			return ChatResponse{}, fmt.Errorf("conversation not found: %w", err)
		}
	}

	// 2. Save user message
	userMsg := ChatMessage{
		ConversationID: conversationID,
		Role:           "user",
		Content:        message,
		TokensUsed:     0, // User messages don't consume tokens
	}

	if err := a.db.Create(&userMsg).Error; err != nil {
		log.Printf("❌ Failed to save user message: %v", err)
		return ChatResponse{}, fmt.Errorf("failed to save user message: %w", err)
	}

	// 3. Load conversation history (last 20 messages for context)
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

	// Reverse the messages to chronological order (oldest first)
	for i, j := 0, len(recentMessages)-1; i < j; i, j = i+1, j-1 {
		recentMessages[i], recentMessages[j] = recentMessages[j], recentMessages[i]
	}

	log.Printf("📚 Loaded %d messages from conversation history", len(recentMessages))

	// 4. Build context-aware messages for Mistral API
	// Classify intent for the current message
	intent := classifyIntent(message)
	log.Printf("🧠 Intent classified: domain=%s entity=%q confidence=%.2f complex=%v",
		intent.Domain, intent.EntityName, intent.Confidence, intent.IsComplex)

	// Check if this is a report request
	if isReportRequest(message) {
		butlerResp, err := a.handleReportRequest(message, intent)
		if err != nil {
			return ChatResponse{}, err
		}

		// Save assistant response
		assistantMsg := ChatMessage{
			ConversationID: conversationID,
			Role:           "assistant",
			Content:        butlerResp.Message,
			TokensUsed:     100, // Estimate for report generation
		}

		if err := a.db.Create(&assistantMsg).Error; err != nil {
			log.Printf("⚠️ Failed to save assistant message: %v", err)
		}

		// Update conversation timestamp
		a.db.Model(&Conversation{}).Where("id = ?", conversationID).Update("last_msg_at", time.Now())

		return ChatResponse{
			Response:       butlerResp.Message,
			ConversationID: conversationID,
			Actions:        butlerResp.Actions,
			Confidence:     butlerResp.Confidence,
			TokensUsed:     100,
		}, nil
	}

	// 5. Build business context based on intent
	context := a.buildIntentContext(intent)

	// 6. Calculate system regime
	regime := a.calculateSystemRegime()
	context["system_regime"] = regime

	// 7. Select model based on complexity
	model := mistralModelSmall
	if intent.IsComplex {
		model = mistralModelLarge
	}

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
	for _, msg := range recentMessages[startIdx:] {
		mistralMessages = append(mistralMessages, map[string]interface{}{
			"role":    msg.Role,
			"content": msg.Content,
		})
	}

	// Current user message is already at the end from recent history
	// If not in history yet (edge case), add it
	if len(recentMessages) == 0 || recentMessages[len(recentMessages)-1].Content != message {
		mistralMessages = append(mistralMessages, map[string]interface{}{
			"role":    "user",
			"content": message,
		})
	}

	// 10. Call Mistral API with full context
	aiResponse, err := callMistralWithMessages(model, mistralMessages)
	if err != nil {
		log.Printf("❌ Mistral API error: %v", err)

		// Save error response as assistant message
		errorMsg := fmt.Sprintf("Intelligence pipeline error: %v. Please try again.", err)
		assistantMsg := ChatMessage{
			ConversationID: conversationID,
			Role:           "assistant",
			Content:        errorMsg,
			TokensUsed:     0,
		}

		if saveErr := a.db.Create(&assistantMsg).Error; saveErr != nil {
			log.Printf("⚠️ Failed to save error message: %v", saveErr)
		}

		// Update conversation timestamp
		a.db.Model(&Conversation{}).Where("id = ?", conversationID).Update("last_msg_at", time.Now())

		return ChatResponse{
			Response:       errorMsg,
			ConversationID: conversationID,
			Actions:        []ButlerAction{},
			Confidence:     0.0,
			TokensUsed:     0,
		}, nil // Return nil error so frontend shows the message
	}

	// 11. Parse response (extract actions, clean message)
	butlerResponse := parseMistralResponse(aiResponse, context, intent)

	// 12. Save assistant response
	tokensUsed := len(aiResponse) / 4 // Rough estimate: 1 token ≈ 4 characters
	assistantMsg := ChatMessage{
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        butlerResponse.Message,
		TokensUsed:     tokensUsed,
	}

	if err := a.db.Create(&assistantMsg).Error; err != nil {
		log.Printf("⚠️ Failed to save assistant message: %v", err)
	}

	// 13. Update conversation timestamp
	a.db.Model(&Conversation{}).Where("id = ?", conversationID).Update("last_msg_at", time.Now())

	log.Printf("🤖 Butler responded via %s: %s (confidence: %.2f, actions: %d, tokens: %d)",
		model, butlerTruncate(butlerResponse.Message, 60), butlerResponse.Confidence,
		len(butlerResponse.Actions), tokensUsed)

	return ChatResponse{
		Response:       butlerResponse.Message,
		ConversationID: conversationID,
		Actions:        butlerResponse.Actions,
		Confidence:     butlerResponse.Confidence,
		TokensUsed:     tokensUsed,
	}, nil
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
			preview := msg.Content
			if len(preview) > 100 {
				preview = preview[:100] + "..."
			}
			conversationContext += fmt.Sprintf("- %s: %s\n", role, preview)
		}

		conversationContext += "\nMaintain continuity with this conversation history. Reference previous topics when relevant."
	}

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
- Maintain conversation continuity and reference previous topics when relevant

CONTEXT DATA:
%s

%s%s

INSTRUCTIONS:
1. Always cite specific data from context when available (e.g., "Outstanding: 5,230 BHD")
2. If data is missing from context, say "I don't have that data available"
3. Provide actionable recommendations, not just facts
4. Be concise and professional (under 150 words for simple queries)
5. Format currency as BHD with 3 decimal places where appropriate
6. If you recommend an action, include it in [ACTIONS]...[/ACTIONS] block as JSON array
7. Reference previous conversation topics when relevant to maintain continuity

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
- Honest about limitations
- Conversational and contextually aware`, string(contextJSON), regimeGuidance, conversationContext)

	return prompt
}
