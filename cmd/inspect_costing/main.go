package main

import (
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	filePath := `Offers No 1-50 (2025)/01 EWA-FIT(FEED)/OFFER/REV-0/1-Costing sheet 2025-Rev_0.xlsx`

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		log.Fatalf("Failed to open: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	fmt.Println("=== SHEETS ===")
	for _, sheet := range sheets {
		fmt.Println(" -", sheet)
	}

	// Read first sheet
	if len(sheets) > 0 {
		rows, _ := f.GetRows(sheets[0])
		fmt.Printf("\n=== SHEET: %s (%d rows) ===\n", sheets[0], len(rows))
		for i, row := range rows {
			if i < 40 { // First 40 rows should have customer info
				fmt.Printf("Row %2d: %v\n", i+1, row)
			}
		}
	}
}
