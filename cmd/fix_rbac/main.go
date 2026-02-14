package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	log.Println("=== RBAC Deprecation Fix ===")
	log.Println("Removing foreign key constraints and seeding admin role")

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

	// Step 1: Drop foreign key constraints on users table
	log.Println("\n1. Dropping foreign key constraints on users table...")

	// Get all foreign key constraints on users table
	var fkConstraints []string
	db.Raw(`
		SELECT constraint_name
		FROM information_schema.table_constraints
		WHERE table_name = 'users'
		AND constraint_type = 'FOREIGN KEY'
	`).Scan(&fkConstraints)

	for _, fk := range fkConstraints {
		log.Printf("   Dropping constraint: %s", fk)
		db.Exec(fmt.Sprintf("ALTER TABLE users DROP CONSTRAINT IF EXISTS %s", fk))
	}

	// Also drop device_users foreign keys
	log.Println("\n2. Dropping foreign key constraints on device_users table...")
	var duFkConstraints []string
	db.Raw(`
		SELECT constraint_name
		FROM information_schema.table_constraints
		WHERE table_name = 'device_users'
		AND constraint_type = 'FOREIGN KEY'
	`).Scan(&duFkConstraints)

	for _, fk := range duFkConstraints {
		log.Printf("   Dropping constraint: %s", fk)
		db.Exec(fmt.Sprintf("ALTER TABLE device_users DROP CONSTRAINT IF EXISTS %s", fk))
	}

	// Step 2: Make role_id nullable
	log.Println("\n3. Making role_id nullable in users table...")
	db.Exec("ALTER TABLE users ALTER COLUMN role_id DROP NOT NULL")

	log.Println("   Making role_id nullable in device_users table...")
	db.Exec("ALTER TABLE device_users ALTER COLUMN role_id DROP NOT NULL")

	// Step 3: Create admin role if not exists
	log.Println("\n4. Ensuring admin role exists...")

	var roleCount int64
	db.Raw("SELECT COUNT(*) FROM roles WHERE name = 'admin'").Scan(&roleCount)

	if roleCount == 0 {
		adminRoleID := uuid.New().String()
		result := db.Exec(`
			INSERT INTO roles (id, name, description, permissions, is_system, created_at, updated_at)
			VALUES (?, 'admin', 'Full system administrator', '["*"]', true, NOW(), NOW())
			ON CONFLICT (name) DO NOTHING
		`, adminRoleID)

		if result.Error != nil {
			log.Printf("   Warning: Could not create admin role: %v", result.Error)
		} else {
			log.Printf("   Created admin role with ID: %s", adminRoleID)
		}
	} else {
		log.Println("   Admin role already exists")
	}

	// Step 4: List all roles
	log.Println("\n5. Current roles in database:")
	var roles []struct {
		ID          string
		Name        string
		Description string
	}
	db.Raw("SELECT id, name, description FROM roles").Scan(&roles)
	for _, r := range roles {
		log.Printf("   - %s: %s (%s)", r.Name, r.Description, r.ID[:8]+"...")
	}

	// Step 5: Update any users with NULL role_id to admin
	log.Println("\n6. Setting NULL role_ids to admin...")

	var adminRoleID string
	db.Raw("SELECT id FROM roles WHERE name = 'admin' LIMIT 1").Scan(&adminRoleID)

	if adminRoleID != "" {
		result := db.Exec("UPDATE users SET role_id = ? WHERE role_id IS NULL OR role_id = ''", adminRoleID)
		log.Printf("   Updated %d users to admin role", result.RowsAffected)

		result = db.Exec("UPDATE device_users SET role_id = ? WHERE role_id IS NULL OR role_id = ''", adminRoleID)
		log.Printf("   Updated %d device_users to admin role", result.RowsAffected)
	}

	// Verify
	log.Println("\n=== Verification ===")
	var userCount, deviceUserCount int64
	db.Raw("SELECT COUNT(*) FROM users").Scan(&userCount)
	db.Raw("SELECT COUNT(*) FROM device_users").Scan(&deviceUserCount)
	log.Printf("Users: %d", userCount)
	log.Printf("Device Users: %d", deviceUserCount)

	log.Println("\nDone! RBAC constraints removed. App should start now.")
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
