// Test MSG Parser
// Simple standalone test to verify MSG parsing functionality
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	msgparser "github.com/willthrom/outlook-msg-parser"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: test_msg_parser <path-to-msg-file>")
		fmt.Println("Example: test_msg_parser C:\\RFQ\\email.msg")
		os.Exit(1)
	}

	msgPath := os.Args[1]

	fmt.Printf("🔍 Parsing MSG file: %s\n\n", msgPath)

	// Parse the MSG file
	msg, err := msgparser.ParseMsgFile(msgPath)
	if err != nil {
		log.Fatalf("❌ Failed to parse MSG file: %v", err)
	}

	// Display results
	fmt.Println("✅ Successfully parsed MSG file!")
	fmt.Println(strings.Repeat("=", 70))

	fmt.Printf("\n📧 EMAIL METADATA:\n")
	fmt.Printf("  Subject:      %s\n", msg.Subject)
	fmt.Printf("  From:         %s <%s>\n", msg.FromName, msg.FromEmail)
	fmt.Printf("  To:           %s\n", msg.To)
	fmt.Printf("  CC:           %s\n", msg.CC)
	fmt.Printf("  Date Sent:    %s\n", msg.ClientSubmitTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("  Date Received: %s\n", msg.Date.Format("2006-01-02 15:04:05"))

	fmt.Printf("\n📄 BODY:\n")
	if msg.BodyPlainText != "" {
		bodyPreview := msg.BodyPlainText
		if len(bodyPreview) > 200 {
			bodyPreview = bodyPreview[:200] + "..."
		}
		fmt.Printf("  %s\n", bodyPreview)
	} else {
		fmt.Printf("  (No plain text body)\n")
	}

	if msg.BodyHTML != "" {
		fmt.Printf("\n  [HTML body available: %d bytes]\n", len(msg.BodyHTML))
	}

	fmt.Printf("\n📎 ATTACHMENTS: %d\n", len(msg.Attachments))
	for i, att := range msg.Attachments {
		fmt.Printf("  %d. %s\n", i+1, att.Name)
	}

	// Export to JSON
	jsonData, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		log.Printf("⚠️ Failed to marshal JSON: %v", err)
	} else {
		jsonPath := msgPath + ".json"
		if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
			log.Printf("⚠️ Failed to write JSON: %v", err)
		} else {
			fmt.Printf("\n💾 Full JSON export saved to: %s\n", jsonPath)
		}
	}

	fmt.Println(strings.Repeat("=", 70))
	fmt.Println("✅ Test completed successfully!")
}
