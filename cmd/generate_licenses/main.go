package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

var rolePrefixes = map[string]string{
	"admin":      "ADM",
	"manager":    "MGR",
	"sales":      "SLS",
	"operations": "OPS",
}

func generateKey(role string) string {
	prefix := rolePrefixes[role]
	timestamp := time.Now().UnixNano()
	hashInput := fmt.Sprintf("%d-%s-%d", timestamp, role, time.Now().Nanosecond())
	hash := sha256.Sum256([]byte(hashInput))
	hashStr := strings.ToUpper(hex.EncodeToString(hash[:])[:6])
	return fmt.Sprintf("PH-%s-%s", prefix, hashStr)
}

func main() {
	fmt.Println("=======================================================")
	fmt.Println("  PH HOLDINGS LICENSE KEYS")
	fmt.Println("  Generated:", time.Now().Format("2006-01-02 15:04"))
	fmt.Println("=======================================================")
	fmt.Println()

	fmt.Println("ADMIN (Full Access):")
	for i := 0; i < 1; i++ {
		fmt.Printf("  %s\n", generateKey("admin"))
		time.Sleep(time.Millisecond)
	}

	fmt.Println("\nMANAGER (Finance + Operations):")
	for i := 0; i < 2; i++ {
		fmt.Printf("  %s\n", generateKey("manager"))
		time.Sleep(time.Millisecond)
	}

	fmt.Println("\nSALES (Sales Pipeline Only):")
	for i := 0; i < 5; i++ {
		fmt.Printf("  %s\n", generateKey("sales"))
		time.Sleep(time.Millisecond)
	}

	fmt.Println("\nOPERATIONS (Procurement Only):")
	for i := 0; i < 2; i++ {
		fmt.Printf("  %s\n", generateKey("operations"))
		time.Sleep(time.Millisecond)
	}

	fmt.Println()
	fmt.Println("=======================================================")
	fmt.Println("  Store these keys securely. Each key activates ONE PC.")
	fmt.Println("=======================================================")
}
