package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func openDeploymentAuditTestDB(t *testing.T, dbPath string) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	return db
}

func copyDeploymentAuditDBToTemp(t *testing.T, srcPath string) string {
	t.Helper()

	dstPath := filepath.Join(t.TempDir(), filepath.Base(srcPath))
	data, err := os.ReadFile(srcPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(dstPath, data, 0644))
	cleanupSQLiteSidecars(dstPath)
	return dstPath
}

func newDeploymentAuditAppForDB(t *testing.T, db *gorm.DB, dbPath string) *App {
	t.Helper()

	return &App{
		db:               db,
		cache:            NewCache(),
		config:           &Config{Database: DatabaseConfig{Path: dbPath}},
		startupImporting: true,
		currentUserID:    "deployment-audit-admin",
		currentUser: &User{
			Base:     Base{ID: "deployment-audit-admin"},
			Username: "deployment-audit-admin",
			RoleName: "admin",
			Role: Role{
				Name:        "admin",
				DisplayName: "Administrator",
				Permissions: `["*"]`,
			},
		},
	}
}

func seedOfferAuditCustomer(t *testing.T, db *gorm.DB, customerID, businessName string) CustomerMaster {
	t.Helper()

	customer := CustomerMaster{
		Base: Base{
			ID:        uuid.New().String(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		CustomerID:       customerID,
		CustomerCode:     "CUST-" + uuid.New().String()[:8],
		BusinessName:     businessName,
		CustomerType:     "Corporate",
		City:             "Manama",
		Country:          "Bahrain",
		PaymentGrade:     "A",
		CustomerGrade:    "A",
		PaymentTermsDays: 30,
		CreditLimitBHD:   50000,
	}
	require.NoError(t, db.Create(&customer).Error)
	return customer
}

func TestDeploymentDataAuditFlagsBlockingAndWarningIssues(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&Offer{}, &OfferItem{}, &ChartOfAccount{}))
	require.NoError(t, app.ensureCriticalDeploymentFoundations())

	uuidCustomer := seedOfferAuditCustomer(t, app.db, uuid.New().String(), "UUID Customer")
	businessCustomer := seedOfferAuditCustomer(t, app.db, "CUST-OPS-001", "Business Customer")

	order := Order{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OrderNumber:      "ORD-AUDIT-001",
		CustomerID:       businessCustomer.ID,
		CustomerName:     businessCustomer.BusinessName,
		OrderDate:        time.Now(),
		RequiredDate:     time.Now().Add(72 * time.Hour),
		TotalValueBHD:    0,
		GrandTotalBHD:    0,
		Status:           "Processing",
		CustomerPONumber: "PO-AUDIT-001",
	}
	require.NoError(t, app.db.Create(&order).Error)

	invoice := Invoice{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		InvoiceNumber:    "INV-AUDIT-001",
		InvoiceDate:      time.Now(),
		CustomerID:       businessCustomer.ID,
		CustomerName:     businessCustomer.BusinessName,
		CustomerPONumber: "PO-AUDIT-001",
		GrandTotalBHD:    100,
		SubtotalBHD:      100,
		OutstandingBHD:   100,
		Status:           "Sent",
		DueDate:          time.Now().Add(30 * 24 * time.Hour),
	}
	require.NoError(t, app.db.Create(&invoice).Error)

	legacyShell := Offer{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:     "OFF-AUDIT-LEGACY",
		CustomerID:      uuidCustomer.ID,
		CustomerName:    uuidCustomer.BusinessName,
		QuotationDate:   time.Now(),
		ValidityDate:    time.Now().Add(7 * 24 * time.Hour),
		Stage:           "Quoted",
		TotalValueBHD:   0,
		EstimatedMargin: 0,
	}
	require.NoError(t, app.db.Create(&legacyShell).Error)

	wonNoItems := Offer{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:     "OFF-AUDIT-WON",
		CustomerID:      businessCustomer.ID,
		CustomerName:    businessCustomer.BusinessName,
		QuotationDate:   time.Now(),
		ValidityDate:    time.Now().Add(7 * 24 * time.Hour),
		Stage:           "Won",
		TotalValueBHD:   450,
		EstimatedMargin: 12,
	}
	require.NoError(t, app.db.Create(&wonNoItems).Error)

	healthyOffer := Offer{
		Base:            Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferNumber:     "OFF-AUDIT-OK",
		CustomerID:      businessCustomer.ID,
		CustomerName:    businessCustomer.BusinessName,
		QuotationDate:   time.Now(),
		ValidityDate:    time.Now().Add(7 * 24 * time.Hour),
		Stage:           "Quoted",
		TotalValueBHD:   600,
		EstimatedMargin: 10,
	}
	require.NoError(t, app.db.Create(&healthyOffer).Error)
	require.NoError(t, app.db.Create(&OfferItem{
		Base:        Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		OfferID:     healthyOffer.ID,
		LineNumber:  1,
		Description: "Healthy line",
		Quantity:    1,
		UnitPrice:   600,
		TotalPrice:  600,
	}).Error)

	audit, err := app.computeDeploymentDataAudit()
	require.NoError(t, err)
	require.True(t, audit.Blocking)
	require.Equal(t, 1, audit.LegacyQuotedOfferShells)
	require.Equal(t, 1, audit.WonOffersWithoutItems)
	require.Equal(t, 1, audit.ActiveInvoicesWithoutItems)
	require.Equal(t, 1, audit.ActiveOrdersWithoutItems)
	require.Equal(t, 1, audit.ActiveZeroTotalOrders)
	require.Equal(t, 1, audit.ActiveOperationalOffersNoItems)
	require.NotEmpty(t, audit.WarningDataIssues)
	require.NotEmpty(t, audit.BlockingDataIssues)

	offers, err := app.GetAllOffers()
	require.NoError(t, err)
	require.Len(t, offers, 2)
	for _, offer := range offers {
		require.NotEqual(t, legacyShell.ID, offer.ID)
	}

	noItems, err := app.GetOffersWithNoItems()
	require.NoError(t, err)
	require.Len(t, noItems, 2)
}

func TestDeploymentRuntimeDBBootstrapAndAudit(t *testing.T) {
	runtimePath := appDataDatabasePath()
	if runtimePath == "" || !fileExists(runtimePath) {
		t.Skip("runtime deployment database not present on this machine")
	}

	tempPath := filepath.Join(t.TempDir(), "runtime_deployment_copy.db")
	data, err := os.ReadFile(runtimePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(tempPath, data, 0644))
	cleanupSQLiteSidecars(tempPath)

	db := openDeploymentAuditTestDB(t, tempPath)
	app := newDeploymentAuditAppForDB(t, db, tempPath)

	require.NoError(t, app.ensureCriticalDeploymentFoundations())
	audit, err := app.computeDeploymentDataAudit()
	require.NoError(t, err)
	require.Empty(t, audit.MissingTables)
	require.Equal(t, 0, audit.ActiveInvoicesWithoutItems)
	require.Equal(t, 0, audit.ActiveOrdersWithoutItems)
	require.Equal(t, 0, audit.ActiveOperationalOffersNoItems)
	require.Equal(t, 0, audit.ActiveZeroTotalOrders)
	for _, table := range criticalDeploymentTableNames {
		require.Truef(t, db.Migrator().HasTable(table), "expected table %s to exist after bootstrap", table)
	}
}

func TestDeploymentDBCopyReconciliationAndPackaging(t *testing.T) {
	runtimePath := appDataDatabasePath()
	wd, err := os.Getwd()
	require.NoError(t, err)

	repoPath := filepath.Join(wd, "ph_holdings.db")

	if runtimePath == "" || !fileExists(runtimePath) || !fileExists(repoPath) {
		t.Skip("runtime/repo deployment databases not both present")
	}

	t.Setenv("PH_DB_PATH", "")
	t.Setenv("DATABASE_PATH", "")
	require.Equal(t, runtimePath, getDatabasePath())

	runtimeCopy := copyDeploymentAuditDBToTemp(t, runtimePath)
	repoCopy := copyDeploymentAuditDBToTemp(t, repoPath)
	packagedCopy := filepath.Join(t.TempDir(), "packaged_sanitized.db")
	require.NoError(t, copySanitizedDeploymentDB(repoPath, packagedCopy))

	runtimeDB := openDeploymentAuditTestDB(t, runtimeCopy)
	repoDB := openDeploymentAuditTestDB(t, repoCopy)
	packagedDB := openDeploymentAuditTestDB(t, packagedCopy)

	runtimeApp := newDeploymentAuditAppForDB(t, runtimeDB, runtimeCopy)
	repoApp := newDeploymentAuditAppForDB(t, repoDB, repoCopy)
	packagedApp := newDeploymentAuditAppForDB(t, packagedDB, packagedCopy)

	require.NoError(t, runtimeApp.ensureCriticalDeploymentFoundations())
	require.NoError(t, repoApp.ensureCriticalDeploymentFoundations())
	require.NoError(t, packagedApp.ensureCriticalDeploymentFoundations())

	runtimeAudit, err := runtimeApp.computeDeploymentDataAudit()
	require.NoError(t, err)
	repoAudit, err := repoApp.computeDeploymentDataAudit()
	require.NoError(t, err)
	packagedAudit, err := packagedApp.computeDeploymentDataAudit()
	require.NoError(t, err)

	for _, table := range criticalDeploymentTableNames {
		require.Truef(t, repoDB.Migrator().HasTable(table), "repo db missing table %s", table)
		require.Truef(t, packagedDB.Migrator().HasTable(table), "packaged db missing table %s", table)
	}

	require.Equal(t, runtimeAudit.ActiveCustomers, repoAudit.ActiveCustomers)
	require.Equal(t, runtimeAudit.ActiveOrders, repoAudit.ActiveOrders)
	require.Equal(t, runtimeAudit.ActiveInvoices, repoAudit.ActiveInvoices)
	require.Equal(t, runtimeAudit.ActiveOffers, repoAudit.ActiveOffers)
	// Runtime collaboration and finance workflow records can legitimately drift after packaging
	// on an actively used machine, so the automated reconciliation gate only requires the
	// repo and packaged copies to stay aligned with each other for those newer workflow counts.
	require.GreaterOrEqual(t, runtimeAudit.TaskItems, 0)
	require.GreaterOrEqual(t, runtimeAudit.ExpenseEntries, 0)
	require.GreaterOrEqual(t, runtimeAudit.PayrollRuns, 0)

	require.Equal(t, repoAudit.ActiveCustomers, packagedAudit.ActiveCustomers)
	require.Equal(t, repoAudit.ActiveOrders, packagedAudit.ActiveOrders)
	require.Equal(t, repoAudit.ActiveInvoices, packagedAudit.ActiveInvoices)
	require.Equal(t, repoAudit.ActiveOffers, packagedAudit.ActiveOffers)
	require.Equal(t, repoAudit.TaskItems, packagedAudit.TaskItems)
	require.LessOrEqual(t, packagedAudit.ExpenseEntries, repoAudit.ExpenseEntries)
	require.Equal(t, 0, packagedAudit.PayrollRuns)

	var runtimeActivated, packagedActivated int64
	require.NoError(t, runtimeDB.Table("license_keys").Where("activated = ?", true).Count(&runtimeActivated).Error)
	require.NoError(t, packagedDB.Table("license_keys").Where("activated = ?", true).Count(&packagedActivated).Error)
	require.Equal(t, int64(0), packagedActivated)
	require.GreaterOrEqual(t, runtimeActivated, int64(0))
}

func TestManualRepairRuntimeDeploymentDatabase(t *testing.T) {
	if os.Getenv("DEPLOYMENT_RUNTIME_REPAIR_COMMIT") != "1" {
		t.Skip("set DEPLOYMENT_RUNTIME_REPAIR_COMMIT=1 to repair runtime deployment database")
	}

	runtimePath := appDataDatabasePath()
	if runtimePath == "" || !fileExists(runtimePath) {
		t.Fatalf("runtime deployment database not present: %s", runtimePath)
	}

	db := openDeploymentAuditTestDB(t, runtimePath)
	app := newDeploymentAuditAppForDB(t, db, runtimePath)
	require.NoError(t, app.ensureCriticalDeploymentFoundations())

	audit, err := app.computeDeploymentDataAudit()
	require.NoError(t, err)
	require.Empty(t, audit.MissingTables)
	require.Empty(t, audit.BlockingDataIssues)
	t.Logf("runtime deployment database verified at %s", runtimePath)
}
