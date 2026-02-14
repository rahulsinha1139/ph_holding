package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// =============================================================================
// OPPORTUNITY IMPORT - 2025 SALES PIPELINE (ARCHAEOLOGY DATA)
// =============================================================================
// This file imports sales opportunities from "opportunities created 2025.xlsx"
// Data extracted from project archaeology includes:
//   - Folder numbers (FolderNumber)
//   - Customer names (CustomerName)
//   - Project descriptions (Description)
//   - Values (EstimatedValue)
//   - Status/Stage information
//   - Salesperson/Owner (AssignedTo)
//   - Dates (CreatedAt, EstimatedCloseDate)

// import2025Opportunities imports sales opportunities from 2025 Excel file
func import2025Opportunities(db *gorm.DB, customerMap map[string]string) error {
	log.Println("\n=== IMPORTING 2025 OPPORTUNITIES ===")

	filepath := "Data_for_database/opportunities created 2025.xlsx"
	log.Printf("→ Opening file: %s", filepath)

	// Open Excel file
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", filepath, err)
	}
	defer f.Close()

	// Get first sheet name (should be "Sheet1" or similar)
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return fmt.Errorf("no sheets found in Excel file")
	}
	sheetName := sheets[0]
	log.Printf("  Reading sheet: %s", sheetName)

	// Get all rows
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("failed to read sheet %s: %w", sheetName, err)
	}

	if len(rows) < 2 {
		return fmt.Errorf("file has no data rows (only header)")
	}

	// =============================================================================
	// PARSE HEADER TO FIND COLUMNS
	// =============================================================================
	header := rows[0]
	colMap := make(map[string]int)
	for i, colName := range header {
		normalized := strings.ToLower(strings.TrimSpace(colName))
		colMap[normalized] = i
	}

	log.Printf("  Found %d columns in header", len(header))
	log.Printf("  Header: %v", header)

	// Find required columns (flexible matching)
	folderCol := findColumn(colMap, "folder", "folder number", "folder no", "rfq", "rfq number")
	customerCol := findColumn(colMap, "customer", "customer name", "client", "end user", "party")
	projectCol := findColumn(colMap, "project", "project name", "description", "title", "details")
	valueCol := findColumn(colMap, "value", "estimated value", "amount", "opportunity value", "worth")
	statusCol := findColumn(colMap, "status", "opportunity status", "state", "stage")
	salespersonCol := findColumn(colMap, "salesperson", "owner", "assigned to", "sales rep", "responsible")
	createdCol := findColumn(colMap, "created", "created date", "date created", "date", "quotation date")
	closeCol := findColumn(colMap, "close date", "expected close", "target date", "due date", "delivery date")

	// Log column findings
	log.Printf("\n  Column mapping:")
	log.Printf("    Folder:      %d (%v)", folderCol, folderCol >= 0)
	log.Printf("    Customer:    %d (%v)", customerCol, customerCol >= 0)
	log.Printf("    Project:     %d (%v)", projectCol, projectCol >= 0)
	log.Printf("    Value:       %d (%v)", valueCol, valueCol >= 0)
	log.Printf("    Status:      %d (%v)", statusCol, statusCol >= 0)
	log.Printf("    Salesperson: %d (%v)", salespersonCol, salespersonCol >= 0)
	log.Printf("    Created:     %d (%v)", createdCol, createdCol >= 0)
	log.Printf("    Close Date:  %d (%v)", closeCol, closeCol >= 0)

	// Customer column is mandatory
	if customerCol == -1 {
		return fmt.Errorf("customer column not found. Available columns: %v", header)
	}

	// =============================================================================
	// PROCESS ROWS
	// =============================================================================
	log.Printf("\n  Processing %d data rows...", len(rows)-1)

	opportunities := []Opportunity{}
	skippedCount := 0
	unmatchedCustomers := make(map[string]int)

	for rowIdx, row := range rows[1:] {
		lineNum := rowIdx + 2 // Excel line number (1-indexed, +1 for header)

		// Extract customer name (REQUIRED)
		if customerCol >= len(row) {
			log.Printf("⚠ Row %d: customer column out of bounds, skipping", lineNum)
			skippedCount++
			continue
		}

		customerNameRaw := strings.TrimSpace(row[customerCol])
		if customerNameRaw == "" {
			log.Printf("⚠ Row %d: empty customer name, skipping", lineNum)
			skippedCount++
			continue
		}

		// Normalize customer name
		customerName := normalizeCustomerName(customerNameRaw)
		if customerName == "" {
			log.Printf("⚠ Row %d: customer name normalization failed for '%s', skipping", lineNum, customerNameRaw)
			skippedCount++
			continue
		}

		// Look up CustomerID from map
		customerID, found := customerMap[customerName]
		if !found {
			// Track unmatched customers for summary
			unmatchedCustomers[customerName]++
			// Use a placeholder UUID and log warning
			customerID = uuid.New().String()
			if unmatchedCustomers[customerName] == 1 {
				log.Printf("⚠ Row %d: customer '%s' not found in master data, using placeholder ID", lineNum, customerName)
			}
		}

		// Extract folder number (optional - generate if missing)
		folderNumber := ""
		if folderCol >= 0 && folderCol < len(row) {
			folderNumber = strings.TrimSpace(row[folderCol])
		}
		if folderNumber == "" {
			// Generate sequential folder number: 2025-OPP-XXXX
			folderNumber = fmt.Sprintf("2025-OPP-%04d", rowIdx+1)
		}

		// Extract project description (optional)
		description := ""
		if projectCol >= 0 && projectCol < len(row) {
			description = strings.TrimSpace(row[projectCol])
		}
		if description == "" {
			description = "2025 Opportunity"
		}

		// Extract value (optional - default to 0)
		estimatedValue := 0.0
		if valueCol >= 0 && valueCol < len(row) {
			valueStr := strings.TrimSpace(row[valueCol])
			// Try parsing as float (handle commas, currency symbols)
			valueStr = strings.ReplaceAll(valueStr, ",", "")
			valueStr = strings.ReplaceAll(valueStr, "BHD", "")
			valueStr = strings.ReplaceAll(valueStr, "$", "")
			valueStr = strings.TrimSpace(valueStr)
			if valueStr != "" {
				fmt.Sscanf(valueStr, "%f", &estimatedValue)
			}
		}

		// Extract status (optional - default to Active)
		statusRaw := ""
		if statusCol >= 0 && statusCol < len(row) {
			statusRaw = strings.TrimSpace(row[statusCol])
		}

		// Map status to Stage and Status fields
		stage, status := mapStatusToStage(statusRaw)

		// Extract salesperson (optional)
		salesperson := ""
		if salespersonCol >= 0 && salespersonCol < len(row) {
			salesperson = strings.TrimSpace(row[salespersonCol])
		}
		if salesperson == "" {
			salesperson = "Sales Team"
		}

		// Extract created date (optional - default to start of 2025)
		createdDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
		if createdCol >= 0 && createdCol < len(row) {
			dateStr := strings.TrimSpace(row[createdCol])
			if dateStr != "" {
				parsed := parseFlexibleDate(dateStr)
				if !parsed.IsZero() {
					createdDate = parsed
				}
			}
		}

		// Extract close date (optional - default to 90 days from created)
		closeDate := createdDate.AddDate(0, 0, 90)
		if closeCol >= 0 && closeCol < len(row) {
			dateStr := strings.TrimSpace(row[closeCol])
			if dateStr != "" {
				parsed := parseFlexibleDate(dateStr)
				if !parsed.IsZero() {
					closeDate = parsed
				}
			}
		}

		// Calculate win probability based on stage
		winProbability := calculateWinProbability(stage, status)

		// Create Opportunity record
		opportunity := Opportunity{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: createdDate,
				UpdatedAt: time.Now(),
			},
			RFQNumber:          folderNumber,
			CustomerID:         customerID,
			CustomerName:       customerName,
			Description:        description,
			EstimatedValue:     estimatedValue,
			EstimatedCloseDate: closeDate,
			Stage:              stage,
			Status:             status,
			WinProbability:     winProbability,
			AssignedTo:         salesperson,
			ImportSource:       "opportunities created 2025.xlsx",
			Source:             "2025 Archaeology",
		}

		// Set won/lost dates if applicable
		if stage == "Won" {
			wonDate := createdDate
			opportunity.WonDate = &wonDate
			opportunity.ActualValue = estimatedValue
		} else if stage == "Lost" {
			lostDate := createdDate
			opportunity.LostDate = &lostDate
			opportunity.LostReason = "Extracted from 2025 archaeology data"
		}

		opportunities = append(opportunities, opportunity)
	}

	log.Printf("\n  Parsed %d opportunities from %d rows", len(opportunities), len(rows)-1)
	if skippedCount > 0 {
		log.Printf("  ⚠ Skipped %d rows (missing/invalid data)", skippedCount)
	}
	if len(unmatchedCustomers) > 0 {
		log.Printf("\n  ⚠ Warning: %d customers not found in master data:", len(unmatchedCustomers))
		count := 0
		for name, occurrences := range unmatchedCustomers {
			count++
			if count <= 10 { // Show first 10
				log.Printf("    - %s (%d occurrences)", name, occurrences)
			}
		}
		if len(unmatchedCustomers) > 10 {
			log.Printf("    ... and %d more", len(unmatchedCustomers)-10)
		}
	}

	// =============================================================================
	// BATCH INSERT
	// =============================================================================
	if len(opportunities) == 0 {
		log.Println("\n  ⚠ No valid opportunities to insert")
		return nil
	}

	log.Printf("\n  Batch inserting %d opportunities...", len(opportunities))

	batchSize := 100
	for i := 0; i < len(opportunities); i += batchSize {
		end := i + batchSize
		if end > len(opportunities) {
			end = len(opportunities)
		}

		batch := opportunities[i:end]
		if err := db.Create(&batch).Error; err != nil {
			log.Printf("⚠ Batch insert failed for rows %d-%d: %v", i+1, end, err)
			// Continue with next batch
			continue
		}

		log.Printf("    ✓ Inserted batch %d-%d", i+1, end)
	}

	log.Printf("\n✅ Successfully imported %d opportunities", len(opportunities))
	return nil
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// mapStatusToStage converts raw status string to Stage and Status fields
func mapStatusToStage(statusRaw string) (stage string, status string) {
	statusLower := strings.ToLower(strings.TrimSpace(statusRaw))

	// Default values
	stage = "Lead"
	status = "Active"

	// Map based on keywords
	switch {
	case strings.Contains(statusLower, "won") || strings.Contains(statusLower, "closed won"):
		stage = "Won"
		status = "Won"

	case strings.Contains(statusLower, "lost") || strings.Contains(statusLower, "closed lost"):
		stage = "Lost"
		status = "Lost"

	case strings.Contains(statusLower, "proposal") || strings.Contains(statusLower, "quoted") || strings.Contains(statusLower, "quotation"):
		stage = "Proposal"
		status = "Active"

	case strings.Contains(statusLower, "negotiation") || strings.Contains(statusLower, "negotiate"):
		stage = "Negotiation"
		status = "Active"

	case strings.Contains(statusLower, "pending") || strings.Contains(statusLower, "under review"):
		stage = "Lead"
		status = "Pending"

	case strings.Contains(statusLower, "open") || strings.Contains(statusLower, "active"):
		stage = "Proposal"
		status = "Active"

	case strings.Contains(statusLower, "closed") || strings.Contains(statusLower, "completed"):
		// Ambiguous - default to Lost unless explicitly Won
		if !strings.Contains(statusLower, "won") {
			stage = "Lost"
			status = "Lost"
		}

	default:
		// Unknown status - default to Lead/Active
		stage = "Lead"
		status = "Active"
	}

	return stage, status
}

// calculateWinProbability estimates win probability based on stage and status
func calculateWinProbability(stage string, status string) float64 {
	// Standard sales pipeline probabilities
	switch stage {
	case "Lead":
		return 0.10 // 10% - Early stage
	case "Qualified":
		return 0.25 // 25% - Qualified lead
	case "Proposal":
		return 0.50 // 50% - Proposal submitted
	case "Negotiation":
		return 0.75 // 75% - Negotiation in progress
	case "Won":
		return 1.00 // 100% - Deal won
	case "Lost":
		return 0.00 // 0% - Deal lost
	default:
		return 0.20 // 20% - Unknown stage
	}
}

// Note: parseFlexibleDate is already defined in invoices.go
// It returns time.Time (not error) and handles date parsing for the entire package
