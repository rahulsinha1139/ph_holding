package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	log.Println("=== Fixing Invoice Status Values ===")

	godotenv.Load("../../.env")
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set")
	}

	dsn := convertDatabaseURL(databaseURL)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	log.Println("Connected to PostgreSQL")

	// Fix invalid status values
	// Valid: 'Draft','Sent','Paid','Overdue','Cancelled','Void'
	fixes := []string{
		// Fix "Unpaid" -> "Sent"
		`UPDATE invoices SET status = 'Sent' WHERE status = 'Unpaid'`,
		// Fix any other invalid values
		`UPDATE invoices SET status = 'Sent' WHERE status NOT IN ('Draft','Sent','Paid','Overdue','Cancelled','Void')`,
	}

	for _, fix := range fixes {
		result := db.Exec(fix)
		if result.Error != nil {
			log.Printf("ERROR: %v", result.Error)
		} else {
			log.Printf("Fixed %d rows", result.RowsAffected)
		}
	}

	// Verify
	var counts []struct {
		Status string
		Count  int64
	}
	db.Raw(`SELECT status, COUNT(*) as count FROM invoices GROUP BY status`).Scan(&counts)
	log.Println("\n--- Invoice Status Distribution ---")
	for _, c := range counts {
		log.Printf("  %s: %d", c.Status, c.Count)
	}

	log.Println("\nDone!")
}

func convertDatabaseURL(url string) string {
	url = strings.Replace(url, "postgresql://", "postgres://", 1)
	if !strings.HasPrefix(url, "postgres://") {
		return url
	}
	url = url[11:]
	atIdx := strings.LastIndex(url, "@")
	if atIdx == -1 {
		return url
	}
	credentials := url[:atIdx]
	hostPart := url[atIdx+1:]
	colonIdx := strings.Index(credentials, ":")
	user := credentials
	password := ""
	if colonIdx != -1 {
		user = credentials[:colonIdx]
		password = credentials[colonIdx+1:]
	}
	slashIdx := strings.Index(hostPart, "/")
	hostPort := hostPart
	dbname := "ph_holdings"
	if slashIdx != -1 {
		hostPort = hostPart[:slashIdx]
		dbname = hostPart[slashIdx+1:]
		if qIdx := strings.Index(dbname, "?"); qIdx != -1 {
			dbname = dbname[:qIdx]
		}
	}
	host := hostPort
	port := "5432"
	if colonIdx := strings.Index(hostPort, ":"); colonIdx != -1 {
		host = hostPort[:colonIdx]
		port = hostPort[colonIdx+1:]
	}
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require", host, port, user, password, dbname)
}
