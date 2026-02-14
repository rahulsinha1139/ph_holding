package main

import (
	"fmt"
	"log"
	"os"

	"github.com/xuri/excelize/v2"
)

func main() {
	files := []string{
		"Data_for_database/Tally_data/AR payment defaulters.xlsx",
		"Data_for_database/Payments to suppliers.xlsx",
	}

	for _, file := range files {
		fmt.Printf("\n=== %s ===\n", file)

		if _, err := os.Stat(file); os.IsNotExist(err) {
			fmt.Printf("File not found: %s\n", file)
			continue
		}

		f, err := excelize.OpenFile(file)
		if err != nil {
			log.Printf("Error opening %s: %v\n", file, err)
			continue
		}
		defer f.Close()

		sheets := f.GetSheetList()
		fmt.Printf("Sheets: %v\n", sheets)

		for _, sheet := range sheets {
			rows, err := f.GetRows(sheet)
			if err != nil {
				fmt.Printf("Error reading sheet %s: %v\n", sheet, err)
				continue
			}
			fmt.Printf("\nSheet '%s' has %d rows\n", sheet, len(rows))
			fmt.Println("First 15 rows:")
			for i, row := range rows {
				if i < 15 {
					fmt.Printf("Row %d: %v\n", i+1, row)
				}
			}
		}
	}
}
