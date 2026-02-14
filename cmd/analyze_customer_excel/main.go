package main

import (
	"fmt"
	"log"

	"github.com/xuri/excelize/v2"
)

func main() {
	f, err := excelize.OpenFile("Error_Logs/customer database .xlsx")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	fmt.Printf("Sheets: %v\n\n", sheets)

	for _, sheet := range sheets {
		fmt.Printf("=== Sheet: %s ===\n", sheet)
		rows, err := f.GetRows(sheet)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Print header and first 25 rows
		for i, row := range rows {
			if i > 30 {
				fmt.Printf("... (%d more rows)\n", len(rows)-30)
				break
			}
			fmt.Printf("Row %d: %v\n", i, row)
		}
		fmt.Printf("\nTotal rows: %d\n\n", len(rows))
	}
}
