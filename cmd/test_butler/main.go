// Quick test for Butler AI integration
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	log.Println("🧪 Testing AIMLAPI Connection...")

	apiKey := os.Getenv("AIMLAPI_KEY")
	if apiKey == "" {
		log.Fatal("AIMLAPI_KEY environment variable not set")
	}

	// Simple test message
	systemPrompt := `You are Butler, an AI assistant for PH Trading.
Keep responses under 50 words. Be professional and concise.`

	userMessage := "What is the current business status?"

	// Build request
	requestBody := map[string]interface{}{
		"model": "gpt-4o-mini", // Fast, cost-effective OpenAI model via AIMLAPI
		"messages": []map[string]interface{}{
			{
				"role":    "system",
				"content": systemPrompt,
			},
			{
				"role":    "user",
				"content": userMessage,
			},
		},
		"max_tokens":  200,
		"temperature": 0.7,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		log.Fatalf("❌ Failed to marshal: %v", err)
	}

	// Create request
	req, err := http.NewRequest("POST", "https://api.aimlapi.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Fatalf("❌ Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	log.Printf("📤 Sending request to AIMLAPI...")
	log.Printf("   Model: gpt-4o-mini")
	log.Printf("   Message: %s", userMessage)

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalf("❌ Request failed: %v", err)
	}
	defer resp.Body.Close()

	elapsed := time.Since(start)

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("❌ Failed to read response: %v", err)
	}

	if resp.StatusCode != 200 {
		log.Fatalf("❌ AIMLAPI error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var apiResponse struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &apiResponse); err != nil {
		log.Fatalf("❌ Failed to parse response: %v", err)
	}

	if len(apiResponse.Choices) == 0 {
		log.Fatalf("❌ No response from AI")
	}

	content := apiResponse.Choices[0].Message.Content

	// Display results
	fmt.Println("\n✅ AIMLAPI TEST SUCCESSFUL!")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("Response Time: %v\n", elapsed)
	fmt.Printf("Status Code: %d\n", resp.StatusCode)
	fmt.Printf("\nButler Response:\n%s\n", content)
	fmt.Println("═══════════════════════════════════════════════════════════")

	log.Println("✅ Butler AI is LIVE and operational! 🎉")
}
