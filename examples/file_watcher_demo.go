// ═══════════════════════════════════════════════════════════════════════════
// FILE WATCHER DEMO - Example usage of the file watcher
//
// USAGE: go run file_watcher_demo.go [watch_directory]
//
// EXAMPLE:
//   go run file_watcher_demo.go "C:\Users\schan\OneDrive - PH Holdings\RFQs"
//
// This will watch the specified directory and print events to console.
// ═══════════════════════════════════════════════════════════════════════════

//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Copy types from file_watcher.go for standalone compilation
// In production, these would be imported from the main package

func main() {
	// Get watch directory from args or use temp directory
	watchDir := os.TempDir()
	if len(os.Args) > 1 {
		watchDir = os.Args[1]
	}

	// Ensure directory exists
	if _, err := os.Stat(watchDir); os.IsNotExist(err) {
		fmt.Printf("Creating directory: %s\n", watchDir)
		if err := os.MkdirAll(watchDir, 0755); err != nil {
			fmt.Printf("Failed to create directory: %v\n", err)
			os.Exit(1)
		}
	}

	fmt.Printf("╔═══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║ PH Holdings File Watcher Demo                                ║\n")
	fmt.Printf("╚═══════════════════════════════════════════════════════════════╝\n\n")
	fmt.Printf("Watching: %s\n", watchDir)
	fmt.Printf("Press Ctrl+C to stop\n\n")

	// Create watcher configuration
	config := &WatchConfig{
		RFQPath:       filepath.Join(watchDir, "RFQs"),
		EHXMLPath:     filepath.Join(watchDir, "EH_XML"),
		OfferPath:     filepath.Join(watchDir, "Offers"),
		InvoicePath:   filepath.Join(watchDir, "Invoices"),
		ExtraPaths:    []string{watchDir}, // Watch root too
		DebounceDelay: 300 * time.Millisecond,
		MaxQueueSize:  100,
		Recursive:     true,
	}

	// Create file watcher
	fw, err := NewFileWatcher(config)
	if err != nil {
		fmt.Printf("Failed to create file watcher: %v\n", err)
		os.Exit(1)
	}

	// Register event handlers
	fw.OnNewRFQ(func(ctx context.Context, event WatchEvent) error {
		fmt.Printf("🟢 NEW RFQ: %s (size: %d bytes)\n", event.Path, event.Size)
		return nil
	})

	fw.OnEHXML(func(ctx context.Context, event WatchEvent) error {
		fmt.Printf("🔵 E+H XML: %s (size: %d bytes)\n", event.Path, event.Size)
		return nil
	})

	fw.OnOfferChange(func(ctx context.Context, event WatchEvent) error {
		fmt.Printf("🟡 OFFER CHANGE: %s (size: %d bytes)\n", event.Path, event.Size)
		return nil
	})

	fw.OnInvoice(func(ctx context.Context, event WatchEvent) error {
		fmt.Printf("🟣 INVOICE: %s (size: %d bytes)\n", event.Path, event.Size)
		return nil
	})

	fw.OnEvent(EventGenericFile, func(ctx context.Context, event WatchEvent) error {
		fmt.Printf("⚪ GENERIC: %s (%s)\n", event.Path, event.Extension)
		return nil
	})

	// Start watcher
	if err := fw.Start(); err != nil {
		fmt.Printf("Failed to start watcher: %v\n", err)
		os.Exit(1)
	}
	defer fw.Stop()

	fmt.Printf("✓ Watcher started successfully\n\n")

	// Print instructions
	fmt.Printf("Try creating files in the following directories:\n")
	fmt.Printf("  - RFQs/    → Create .msg or .eml files\n")
	fmt.Printf("  - EH_XML/  → Create .xml files\n")
	fmt.Printf("  - Offers/  → Create any files\n")
	fmt.Printf("  - Invoices/ → Create .pdf files\n\n")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Periodic status report
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Printf("\n\nShutting down...\n")
			return

		case <-ticker.C:
			// Print sync status summary
			allStatuses := fw.GetSyncStatus().GetAllStatuses()
			if len(allStatuses) > 0 {
				fmt.Printf("\n--- Status Report ---\n")
				syncedCount := 0
				for _, state := range allStatuses {
					if state.Status == WatchStatusSynced {
						syncedCount++
					}
				}
				fmt.Printf("Files tracked: %d (synced: %d)\n\n", len(allStatuses), syncedCount)
			}
		}
	}
}
