package main

import (
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	files := []string{
		"../../Data_for_database/Tally_data/invoices 2025.xlsx",
		"../../Data_for_database/Tally_data/AR payment defaulters.xlsx",
		"../../Data_for_database/Tally_data/purchase 2025.xlsx",
	}

	for _, file := range files {
		analyzeFile(file)
	}
}

func analyzeFile(filepath string) {
	f, err := excelize.OpenFile(filepath)
	if err != nil {
		log.Printf("ERROR opening %s: %v", filepath, err)
		return
	}
	defer f.Close()

	fmt.Printf("\n========================================\n")
	fmt.Printf("FILE: %s\n", filepath)
	fmt.Printf("========================================\n")

	sheetName := f.GetSheetName(0)
	rows, err := f.GetRows(sheetName)
	if err != nil {
		log.Printf("ERROR reading rows: %v", err)
		return
	}

	fmt.Printf("Total rows: %d\n\n", len(rows))

	// Print ALL rows (up to 50) to understand the structure
	for i := 0; i < len(rows) && i < 50; i++ {
		row := rows[i]
		fmt.Printf("Row %d (%d cols): ", i, len(row))
		for j, cell := range row {
			if j > 0 {
				fmt.Print(" | ")
			}
			// Truncate long cells
			if len(cell) > 40 {
				cell = cell[:40] + "..."
			}
			fmt.Print(cell)
		}
		fmt.Println()
	}
}
