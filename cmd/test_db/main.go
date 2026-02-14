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
	log.Println("=== Database Connection Test ===")

	// Try loading .env from multiple locations
	locations := []string{
		".env",
		"../../.env",
		"../.env",
	}

	for _, loc := range locations {
		err := godotenv.Load(loc)
		if err == nil {
			log.Printf("Loaded .env from: %s", loc)
			break
		}
	}

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL not set in environment")
	}

	log.Printf("DATABASE_URL found: %s...%s", databaseURL[:30], databaseURL[len(databaseURL)-20:])

	dsn := convertDatabaseURL(databaseURL)
	log.Printf("DSN: %s", maskPassword(dsn))

	log.Println("Attempting connection...")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("FAILED to connect: %v", err)
	}

	log.Println("✅ Connected successfully!")

	// Test query
	var count int64
	db.Raw("SELECT COUNT(*) FROM customers").Scan(&count)
	log.Printf("Customers in database: %d", count)

	db.Raw("SELECT COUNT(*) FROM invoices").Scan(&count)
	log.Printf("Invoices in database: %d", count)

	db.Raw("SELECT COUNT(*) FROM roles").Scan(&count)
	log.Printf("Roles in database: %d", count)

	log.Println("\n✅ Database connection is working!")
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

func maskPassword(dsn string) string {
	// Mask password in DSN for logging
	if idx := strings.Index(dsn, "password="); idx != -1 {
		end := strings.Index(dsn[idx:], " ")
		if end == -1 {
			end = len(dsn) - idx
		}
		return dsn[:idx] + "password=****" + dsn[idx+end:]
	}
	return dsn
}
