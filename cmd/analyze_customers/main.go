package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xuri/excelize/v2"
)

func main() {
	f, err := excelize.OpenFile("../../Data_for_database/opportunities created 2025.xlsx")
	if err != nil {
		panic(err)
	}
	defer f.Close()

	rows, _ := f.GetRows(f.GetSheetName(0))

	// Find column indices
	header := rows[0]
	var endUserIdx, statusIdx int
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(h))
		if h == "end user" {
			endUserIdx = i
		}
		if strings.Contains(h, "status") {
			statusIdx = i
		}
	}

	// Collect unique end users and statuses
	endUsers := make(map[string]int)
	statuses := make(map[string]int)

	for _, row := range rows[1:] {
		if endUserIdx < len(row) {
			eu := strings.TrimSpace(row[endUserIdx])
			if eu != "" {
				endUsers[eu]++
			}
		}
		if statusIdx < len(row) {
			st := strings.TrimSpace(row[statusIdx])
			if st != "" {
				statuses[st]++
			}
		}
	}

	// Sort and print end users
	fmt.Println("=== UNIQUE END USERS ===")
	fmt.Printf("Total unique: %d\n\n", len(endUsers))

	type kv struct {
		Key   string
		Value int
	}
	var sortedUsers []kv
	for k, v := range endUsers {
		sortedUsers = append(sortedUsers, kv{k, v})
	}
	sort.Slice(sortedUsers, func(i, j int) bool {
		return sortedUsers[i].Value > sortedUsers[j].Value
	})

	for i, kv := range sortedUsers {
		fmt.Printf("%3d. %-50s (%d)\n", i+1, kv.Key, kv.Value)
	}

	// Print statuses
	fmt.Println("\n=== UNIQUE STATUSES ===")
	fmt.Printf("Total unique: %d\n\n", len(statuses))
	for status, count := range statuses {
		fmt.Printf("  %-40s: %d\n", status, count)
	}
}
