package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Base struct {
	ID        string         `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

type Device struct {
	Base
	MachineID   string    `gorm:"uniqueIndex;size:64" json:"machine_id"`
	DeviceName  string    `gorm:"size:255" json:"device_name"`
	Status      string    `gorm:"size:20;default:pending;index;check:status IN ('first_setup','pending','approved','blocked','revoked')" json:"status"`
	FirstSeenAt time.Time `gorm:"index" json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	ApprovedBy  string    `gorm:"size:36" json:"approved_by"`
	ApprovedAt  *time.Time `json:"approved_at"`
	Notes       string    `gorm:"type:text" json:"notes"`
}

func (Device) TableName() string { return "devices" }

type DeviceUser struct {
	Base
	DeviceID     string `gorm:"index;size:36" json:"device_id"`
	Username     string `gorm:"index;size:100" json:"username"`
	PasswordHash string `gorm:"size:255" json:"password_hash"`
	RoleID       string `gorm:"index;size:36" json:"role_id"`
	IsActive     bool   `gorm:"default:true" json:"is_active"`
}

func (DeviceUser) TableName() string { return "device_users" }

func main() {
	log.Println("=== Creating Device Tables ===")

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

	// Create device tables
	log.Println("Creating devices and device_users tables...")
	err = db.AutoMigrate(&Device{}, &DeviceUser{})
	if err != nil {
		log.Printf("AutoMigrate error: %v", err)
	}

	// Verify tables exist
	var deviceCount, userCount int64
	db.Raw("SELECT COUNT(*) FROM devices").Scan(&deviceCount)
	db.Raw("SELECT COUNT(*) FROM device_users").Scan(&userCount)

	log.Printf("Devices table: %d rows", deviceCount)
	log.Printf("Device users table: %d rows", userCount)

	log.Println("\nDone! App should now show the setup screen for first device registration.")
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
