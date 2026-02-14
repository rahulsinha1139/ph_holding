package main

import (
	"fmt"
	"log"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("   Regenerating ALL IDs")
	fmt.Println("========================================")

	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("SQLite connection failed: %v", err)
	}
	fmt.Println("✅ SQLite connected")

	// APPROACH: Just regenerate ALL IDs for ALL customers and contacts
	// This is safe because IDs are only used for internal references

	fmt.Println("\n📋 Regenerating ALL customer IDs...")
	var count int
	rows, err := db.Raw("SELECT rowid FROM customers").Rows()
	if err != nil {
		log.Fatalf("Failed to query customers: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var rowid int64
		rows.Scan(&rowid)
		newID := uuid.New().String()
		db.Exec("UPDATE customers SET id = ? WHERE rowid = ?", newID, rowid)
		count++
	}
	fmt.Printf("✅ Regenerated %d customer IDs\n", count)

	fmt.Println("\n📋 Regenerating ALL customer_contact IDs...")
	count = 0
	rows2, err := db.Raw("SELECT rowid FROM customer_contacts").Rows()
	if err != nil {
		log.Fatalf("Failed to query customer_contacts: %v", err)
	}
	defer rows2.Close()

	for rows2.Next() {
		var rowid int64
		rows2.Scan(&rowid)
		newID := uuid.New().String()
		db.Exec("UPDATE customer_contacts SET id = ? WHERE rowid = ?", newID, rowid)
		count++
	}
	fmt.Printf("✅ Regenerated %d contact IDs\n", count)

	// Verify
	fmt.Println("\n📊 Verification:")
	var totalCust, uniqueCust, totalCont, uniqueCont int64
	db.Raw("SELECT COUNT(*) FROM customers").Scan(&totalCust)
	db.Raw("SELECT COUNT(DISTINCT id) FROM customers").Scan(&uniqueCust)
	db.Raw("SELECT COUNT(*) FROM customer_contacts").Scan(&totalCont)
	db.Raw("SELECT COUNT(DISTINCT id) FROM customer_contacts").Scan(&uniqueCont)

	fmt.Printf("  Customers: %d total, %d unique", totalCust, uniqueCust)
	if totalCust == uniqueCust {
		fmt.Println(" ✅")
	} else {
		fmt.Printf(" ❌ (%d duplicates)\n", totalCust-uniqueCust)
	}

	fmt.Printf("  Contacts: %d total, %d unique", totalCont, uniqueCont)
	if totalCont == uniqueCont {
		fmt.Println(" ✅")
	} else {
		fmt.Printf(" ❌ (%d duplicates)\n", totalCont-uniqueCont)
	}

	fmt.Println("\n========================================")
	fmt.Println("  ✅ Complete!")
	fmt.Println("========================================")
}
