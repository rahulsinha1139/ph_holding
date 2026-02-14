package main

import (
	"fmt"
	"os"

	"github.com/xuri/excelize/v2"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run main.go <excel_file>")
		os.Exit(1)
	}

	f, err := excelize.OpenFile(os.Args[1])
	if err != nil {
		fmt.Printf("Error opening file: %v\n", err)
		os.Exit(1)
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

		// Show ALL rows to see full structure
		for i, row := range rows {
			if len(row) == 0 {
				continue
			}
			// Show first column (row label) and first 3 data values
			label := ""
			if len(row) > 0 {
				label = row[0]
				if len(label) > 25 {
					label = label[:22] + "..."
				}
			}
			vals := []string{}
			for j := 1; j < len(row) && j <= 3; j++ {
				v := row[j]
				if len(v) > 15 {
					v = v[:12] + "..."
				}
				vals = append(vals, v)
			}
			if label != "" || len(vals) > 0 {
				fmt.Printf("Row %2d: %-25s | %v\n", i+1, label, vals)
			}
		}
		fmt.Printf("\nTotal rows: %d\n", len(rows))
	}
}
