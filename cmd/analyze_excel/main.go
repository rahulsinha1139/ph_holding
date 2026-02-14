package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/xuri/excelize/v2"
)

func main() {
	// Just analyze 2023 files to find header row
	files := []string{
		"../../Data_for_database/Tally_data/invoices 2023.xlsx",
		"../../Data_for_database/Tally_data/purchase 2023.xlsx",
	}

	for _, file := range files {
		analyzeFileRaw(file)
	}
}

func analyzeFileRaw(filepath string) {
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
	fmt.Printf("Sheet: %s\n\n", sheetName)

	rows, _ := f.GetRows(sheetName)
	fmt.Printf("Total rows: %d\n\n", len(rows))

	// Print first 15 rows with column count
	for i := 0; i < 15 && i < len(rows); i++ {
		fmt.Printf("Row %2d (%d cols): ", i+1, len(rows[i]))
		for j, cell := range rows[i] {
			if j > 0 {
				fmt.Print(" | ")
			}
			// Truncate long cells
			if len(cell) > 30 {
				fmt.Printf("[%d]=%s...", j, cell[:30])
			} else {
				fmt.Printf("[%d]=%s", j, cell)
			}
		}
		fmt.Println()
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

	if len(rows) == 0 {
		fmt.Println("No rows found")
		return
	}

	// Print header
	fmt.Printf("Rows: %d\n", len(rows)-1)
	fmt.Printf("Columns (%d):\n", len(rows[0]))
	for i, col := range rows[0] {
		fmt.Printf("  [%d] %s\n", i, strings.TrimSpace(col))
	}

	// Print first 3 data rows
	fmt.Println("\nFirst 3 rows:")
	for i := 1; i < len(rows) && i <= 3; i++ {
		fmt.Printf("Row %d:\n", i)
		for j, val := range rows[i] {
			if j < len(rows[0]) {
				fmt.Printf("  %s: %s\n", rows[0][j], val)
			}
		}
	}
}
