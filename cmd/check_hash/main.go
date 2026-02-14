package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

func main() {
	var identifiers []string

	hostname, _ := os.Hostname()
	identifiers = append(identifiers, hostname)
	fmt.Printf("Hostname: %s\n", hostname)

	type ifaceResult struct{ addrs []string }
	ifaceChan := make(chan ifaceResult, 1)

	go func() {
		var addrs []string
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, iface := range interfaces {
				if iface.Flags&net.FlagLoopback != 0 || len(iface.HardwareAddr) == 0 {
					continue
				}
				addrs = append(addrs, iface.HardwareAddr.String())
			}
		}
		ifaceChan <- ifaceResult{addrs: addrs}
	}()

	select {
	case result := <-ifaceChan:
		fmt.Printf("MAC addresses found: %d\n", len(result.addrs))
		for i, addr := range result.addrs {
			fmt.Printf("  MAC[%d]: %s\n", i, addr)
		}
		identifiers = append(identifiers, result.addrs...)
	case <-time.After(2 * time.Second):
		fmt.Println("TIMEOUT: Network interfaces timed out!")
	}

	identifiers = append(identifiers, runtime.GOOS, runtime.GOARCH)

	combined := strings.Join(identifiers, "|")
	fmt.Printf("\nCombined string: %s\n", combined)

	hash := sha256.Sum256([]byte(combined))
	result := hex.EncodeToString(hash[:])
	fmt.Printf("Device Hash: %s\n", result)

	// Compare with stored hash
	storedHash := "ccb74364927dfd97b43772e61adb39dea6fe8273b9d10d950aba9268b4204157"
	if result == storedHash {
		fmt.Println("\n✅ MATCH - Device hash matches stored license!")
	} else {
		fmt.Printf("\n❌ MISMATCH!\n   Current:  %s\n   Stored:   %s\n", result, storedHash)
	}
}
