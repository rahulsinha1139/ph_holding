package main

import (
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	files := []string{
		"Data_for_database/Tally_data/purchase 2023.xlsx",
		"Data_for_database/Tally_data/purchase 2024.xlsx",
		"Data_for_database/Tally_data/purchase 2025.xlsx",
	}

	for _, file := range files {
		fmt.Printf("\n=== %s ===\n", file)
		f, err := excelize.OpenFile(file)
		if err != nil {
			log.Printf("Error: %v\n", err)
			continue
		}
		defer f.Close()

		sheets := f.GetSheetList()
		fmt.Printf("Sheets: %v\n", sheets)

		for _, sheet := range sheets {
			rows, _ := f.GetRows(sheet)
			fmt.Printf("Sheet '%s' has %d rows\n", sheet, len(rows))
			for i, row := range rows {
				if i < 10 {
					fmt.Printf("Row %d: %v\n", i+1, row)
				}
			}
		}
	}
}
