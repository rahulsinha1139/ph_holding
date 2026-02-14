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
	fmt.Println("   Fixing Duplicate IDs in Database")
	fmt.Println("========================================")

	// Connect to SQLite
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("SQLite connection failed: %v", err)
	}
	fmt.Println("✅ SQLite connected")

	// Check current state
	var totalCustomers, uniqueCustomers int64
	db.Raw("SELECT COUNT(*) FROM customers").Scan(&totalCustomers)
	db.Raw("SELECT COUNT(DISTINCT id) FROM customers").Scan(&uniqueCustomers)
	fmt.Printf("\n📊 Current State:\n")
	fmt.Printf("  Total customers: %d\n", totalCustomers)
	fmt.Printf("  Unique IDs: %d\n", uniqueCustomers)
	fmt.Printf("  Duplicates: %d\n", totalCustomers-uniqueCustomers)

	var totalContacts, uniqueContacts int64
	db.Raw("SELECT COUNT(*) FROM customer_contacts").Scan(&totalContacts)
	db.Raw("SELECT COUNT(DISTINCT id) FROM customer_contacts").Scan(&uniqueContacts)
	fmt.Printf("\n  Total contacts: %d\n", totalContacts)
	fmt.Printf("  Unique IDs: %d\n", uniqueContacts)
	fmt.Printf("  Duplicates: %d\n", totalContacts-uniqueContacts)

	// FIX CUSTOMERS
	fmt.Println("\n📋 Fixing customer IDs...")

	// Get all customers with their row IDs
	type CustomerRow struct {
		RowID        int64
		ID           string
		BusinessName string
	}
	var customers []CustomerRow
	db.Raw("SELECT rowid, id, business_name FROM customers ORDER BY rowid").Scan(&customers)

	// Track seen IDs and update duplicates
	seenIDs := make(map[string]bool)
	updated := 0

	for _, customer := range customers {
		// If ID is empty or duplicate, generate new UUID
		if customer.ID == "" || seenIDs[customer.ID] {
			newID := uuid.New().String()
			db.Exec("UPDATE customers SET id = ? WHERE rowid = ?", newID, customer.RowID)
			updated++
			if updated <= 5 {
				oldID := customer.ID
				if len(oldID) > 8 {
					oldID = oldID[:8] + "..."
				} else if oldID == "" {
					oldID = "(empty)"
				}
				fmt.Printf("  ✅ Updated %s: %s → %s\n", customer.BusinessName, oldID, newID[:8]+"...")
			}
		} else {
			seenIDs[customer.ID] = true
		}
	}
	fmt.Printf("✅ Fixed %d duplicate customer IDs\n", updated)

	// FIX CUSTOMER_CONTACTS
	fmt.Println("\n📋 Fixing customer_contact IDs...")

	type ContactRow struct {
		RowID       int64
		ID          string
		ContactName string
	}
	var contacts []ContactRow
	db.Raw("SELECT rowid, id, contact_name FROM customer_contacts ORDER BY rowid").Scan(&contacts)

	seenIDs = make(map[string]bool)
	updated = 0

	for _, contact := range contacts {
		// If ID is empty or duplicate, generate new UUID
		if contact.ID == "" || seenIDs[contact.ID] {
			newID := uuid.New().String()
			db.Exec("UPDATE customer_contacts SET id = ? WHERE rowid = ?", newID, contact.RowID)
			updated++
			if updated <= 5 {
				oldID := contact.ID
				if len(oldID) > 8 {
					oldID = oldID[:8] + "..."
				} else if oldID == "" {
					oldID = "(empty)"
				}
				fmt.Printf("  ✅ Updated %s: %s → %s\n", contact.ContactName, oldID, newID[:8]+"...")
			}
		} else {
			seenIDs[contact.ID] = true
		}
	}
	fmt.Printf("✅ Fixed %d duplicate contact IDs\n", updated)

	// Verify fix
	fmt.Println("\n📊 After Fix:")
	db.Raw("SELECT COUNT(*) FROM customers").Scan(&totalCustomers)
	db.Raw("SELECT COUNT(DISTINCT id) FROM customers").Scan(&uniqueCustomers)
	fmt.Printf("  Total customers: %d\n", totalCustomers)
	fmt.Printf("  Unique IDs: %d\n", uniqueCustomers)

	if totalCustomers == uniqueCustomers {
		fmt.Println("  ✅ All customer IDs are now unique!")
	} else {
		fmt.Printf("  ⚠️ Still have %d duplicates\n", totalCustomers-uniqueCustomers)
	}

	db.Raw("SELECT COUNT(*) FROM customer_contacts").Scan(&totalContacts)
	db.Raw("SELECT COUNT(DISTINCT id) FROM customer_contacts").Scan(&uniqueContacts)
	fmt.Printf("\n  Total contacts: %d\n", totalContacts)
	fmt.Printf("  Unique IDs: %d\n", uniqueContacts)

	if totalContacts == uniqueContacts {
		fmt.Println("  ✅ All contact IDs are now unique!")
	} else {
		fmt.Printf("  ⚠️ Still have %d duplicates\n", totalContacts-uniqueContacts)
	}

	fmt.Println("\n========================================")
	fmt.Println("  ✅ Fix Complete!")
	fmt.Println("========================================")
}
