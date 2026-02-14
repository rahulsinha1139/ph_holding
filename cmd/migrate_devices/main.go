package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type BaseModel struct {
	ID        string `gorm:"primaryKey;type:varchar(36)"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// Device represents a registered installation of the application
type Device struct {
	BaseModel
	MachineID     string     `gorm:"uniqueIndex;size:255"`
	DeviceName    string     `gorm:"size:255"`
	OSInfo        string     `gorm:"size:255"`
	FirstSeenAt   time.Time
	LastSeenAt    *time.Time
	Status        string     `gorm:"size:20;default:pending;index"`
	ApprovedBy    string     `gorm:"size:36"`
	ApprovedAt    *time.Time
	IsAdminDevice bool       `gorm:"default:false"`
	Notes         string     `gorm:"type:text"`
}

func (Device) TableName() string { return "devices" }

// DeviceUser links users to devices
type DeviceUser struct {
	BaseModel
	DeviceID  string `gorm:"index;size:36"`
	UserID    string `gorm:"index;size:36"`
	IsPrimary bool   `gorm:"default:false"`
}

func (DeviceUser) TableName() string { return "device_users" }

// Role model for updating permissions
type Role struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;size:50"`
	DisplayName string `gorm:"size:100"`
	Description string `gorm:"size:255"`
	Permissions string `gorm:"type:text"`
	IsActive    bool   `gorm:"default:true"`
	IsSystem    bool   `gorm:"default:false"`
}

func (Role) TableName() string { return "roles" }

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgresql://ph_admin:Indcy4LP1o5uDJT2oFQ4KzyXKcFllSKu@dpg-d5rmv2hr0fns73e8sq1g-a.singapore-postgres.render.com/ph_holdings"
	}

	fmt.Println("🚀 Migrating Device tables to Render PostgreSQL...")
	fmt.Println("═══════════════════════════════════════════════════════════")

	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatalf("❌ Connection failed: %v", err)
	}

	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// Migrate Device tables
	fmt.Println("\n📋 Creating Device tables...")
	if err := db.AutoMigrate(&Device{}, &DeviceUser{}); err != nil {
		log.Fatalf("❌ Migration failed: %v", err)
	}
	fmt.Println("✅ Device and DeviceUser tables created")

	// Fix roles table schema - add missing columns if they don't exist
	fmt.Println("\n📋 Fixing roles table schema...")
	db.Exec("ALTER TABLE roles ADD COLUMN IF NOT EXISTS display_name VARCHAR(100)")
	db.Exec("ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_active BOOLEAN DEFAULT true")
	db.Exec("ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_system BOOLEAN DEFAULT false")
	fmt.Println("✅ Roles table schema updated")

	// Update roles with new permissions
	fmt.Println("\n📋 Updating role permissions...")

	roles := []struct {
		Name        string
		DisplayName string
		Description string
		Permissions string
	}{
		{
			Name:        "admin",
			DisplayName: "Administrator",
			Description: "Full system access with user and device management",
			Permissions: `["*"]`,
		},
		{
			Name:        "manager",
			DisplayName: "Manager",
			Description: "All operations including finance, no user/device management",
			Permissions: `[
				"dashboard:view",
				"opportunities:view", "opportunities:create", "opportunities:edit",
				"operations:view", "operations:create", "operations:edit",
				"finance:view", "finance:create", "finance:edit",
				"customers:view", "customers:edit", "customers:delete",
				"suppliers:view", "suppliers:edit", "suppliers:delete",
				"invoices:view", "invoices:create", "invoices:approve",
				"orders:view", "orders:create",
				"payments:view", "payments:record",
				"reports:view", "reports:generate",
				"settings:view",
				"intelligence:chat",
				"offers:view", "offers:create"
			]`,
		},
		{
			Name:        "sales",
			DisplayName: "Sales Staff",
			Description: "Sales operations only - NO finance access",
			Permissions: `[
				"dashboard:view",
				"opportunities:view", "opportunities:create",
				"operations:view",
				"customers:view", "customers:edit",
				"suppliers:view",
				"orders:view", "orders:create",
				"offers:view", "offers:create",
				"intelligence:chat",
				"settings:view"
			]`,
		},
		{
			Name:        "operations",
			DisplayName: "Operations Staff",
			Description: "Operations only - NO finance access",
			Permissions: `[
				"dashboard:view",
				"operations:view", "operations:create", "operations:edit",
				"suppliers:view",
				"orders:view",
				"settings:view"
			]`,
		},
		{
			Name:        "staff",
			DisplayName: "Staff",
			Description: "Basic read access - NO finance",
			Permissions: `[
				"dashboard:view",
				"customers:view",
				"suppliers:view",
				"orders:view",
				"offers:view",
				"settings:view"
			]`,
		},
	}

	for _, r := range roles {
		var existing Role
		result := db.Where("name = ?", r.Name).First(&existing)

		if result.Error != nil {
			// Create new role
			newRole := Role{
				BaseModel:   BaseModel{ID: fmt.Sprintf("role_%s", r.Name)},
				Name:        r.Name,
				DisplayName: r.DisplayName,
				Description: r.Description,
				Permissions: r.Permissions,
				IsActive:    true,
				IsSystem:    true,
			}
			db.Create(&newRole)
			fmt.Printf("   ✅ Created role: %s\n", r.DisplayName)
		} else {
			// Update existing
			existing.DisplayName = r.DisplayName
			existing.Description = r.Description
			existing.Permissions = r.Permissions
			existing.IsActive = true
			existing.IsSystem = true
			db.Save(&existing)
			fmt.Printf("   ✅ Updated role: %s\n", r.DisplayName)
		}
	}

	// Verify tables
	var tables []string
	db.Raw("SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename IN ('devices', 'device_users') ORDER BY tablename").Scan(&tables)
	fmt.Printf("\n📊 Device tables verified: %v\n", tables)

	// Count roles
	var roleCount int64
	db.Model(&Role{}).Count(&roleCount)
	fmt.Printf("📊 Roles configured: %d\n", roleCount)

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Println("🎉 Device registration system ready!")
	fmt.Println("\nRole Summary:")
	fmt.Println("  - admin:      Full access (can manage devices & users)")
	fmt.Println("  - manager:    All operations INCLUDING finance")
	fmt.Println("  - sales:      Sales + CRM (NO finance)")
	fmt.Println("  - operations: Operations only (NO finance)")
	fmt.Println("  - staff:      Read-only (NO finance)")
}
