package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestApplyCustomerReferenceSeed_UpdatesLegacyCustomerIDs(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerMaster{}))

	customer := CustomerMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:   "CUST-OLD-001",
		CustomerCode: "CUST-OLD-001",
		BusinessName: "ALUMINIUM BAHRAIN B.S.C",
		CustomerType: "Industrial",
		Status:       "Active",
	}
	require.NoError(t, app.db.Create(&customer).Error)

	seedPath := filepath.Join(t.TempDir(), "customer_reference_seed.tsv")
	seedText := "business_name\tcustomer_type\tshort_code\tserial_no\tcustomer_id\n" +
		"ALUMINIUM BAHRAIN B.S.C.(C)\tEnd Customer\tEC\t14\tEC14\n"
	require.NoError(t, os.WriteFile(seedPath, []byte(seedText), 0644))

	result, err := applyCustomerReferenceSeed(app.db, seedPath)
	require.NoError(t, err)
	require.Equal(t, 1, result.SeedRows)
	require.Equal(t, 1, result.MatchedExisting)
	require.Equal(t, 1, result.UpdatedExisting)
	require.Equal(t, 0, result.InsertedNew)

	var stored CustomerMaster
	require.NoError(t, app.db.First(&stored, "id = ?", customer.ID).Error)
	require.Equal(t, "EC14", stored.CustomerID)
	require.Equal(t, "EC14", stored.CustomerCode)
	require.Equal(t, "EC", stored.ShortCode)
	require.Equal(t, "EC", stored.CustomerType)
}

func TestApplyCustomerReferenceSeed_InsertsNewCustomerUsingSharedPolicy(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerMaster{}))

	seedPath := filepath.Join(t.TempDir(), "customer_reference_seed.tsv")
	seedText := "business_name\tcustomer_type\tshort_code\tserial_no\tcustomer_id\n" +
		"New Reseller Co\tNational Reseller\tNR\t99\tNR99\n"
	require.NoError(t, os.WriteFile(seedPath, []byte(seedText), 0644))

	result, err := applyCustomerReferenceSeed(app.db, seedPath)
	require.NoError(t, err)
	require.Equal(t, 1, result.SeedRows)
	require.Equal(t, 1, result.InsertedNew)
	require.Equal(t, 0, result.UpdatedExisting)

	var stored CustomerMaster
	require.NoError(t, app.db.First(&stored, "business_name = ?", "New Reseller Co").Error)
	require.Equal(t, "NR99", stored.CustomerID)
	require.Equal(t, "NR99", stored.CustomerCode)
	require.Equal(t, "NR", stored.ShortCode)
	require.Equal(t, "Corporate", stored.CustomerType)
}

func TestApplyCustomerReferenceSeed_SkipsConflictingIdentityUpdate(t *testing.T) {
	app := setupTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerMaster{}))

	conflict := CustomerMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:   "OTHER-001",
		CustomerCode: "EC14",
		BusinessName: "Existing Conflict Customer",
		CustomerType: "EC",
		Status:       "Active",
	}
	legacy := CustomerMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		CustomerID:       "CUST-OLD-002",
		CustomerCode:     "CUST-OLD-002",
		BusinessName:     "ALUMINIUM BAHRAIN B.S.C",
		CustomerType:     "Corporate",
		Status:           "Active",
		TotalOrdersValue: 2000000,
	}
	require.NoError(t, app.db.Create(&conflict).Error)
	require.NoError(t, app.db.Create(&legacy).Error)

	seedPath := filepath.Join(t.TempDir(), "customer_reference_seed.tsv")
	seedText := "business_name\tcustomer_type\tshort_code\tserial_no\tcustomer_id\n" +
		"ALUMINIUM BAHRAIN B.S.C.(C)\tEnd Customer\tEC\t14\tEC14\n"
	require.NoError(t, os.WriteFile(seedPath, []byte(seedText), 0644))

	result, err := applyCustomerReferenceSeed(app.db, seedPath)
	require.NoError(t, err)
	require.Equal(t, 1, result.SeedRows)
	require.Equal(t, 1, result.MatchedExisting)
	require.Equal(t, 0, result.UpdatedExisting)
	require.Equal(t, 1, result.SkippedAmbiguous)

	var stored CustomerMaster
	require.NoError(t, app.db.First(&stored, "id = ?", legacy.ID).Error)
	require.Equal(t, "CUST-OLD-002", stored.CustomerID)
	require.Equal(t, "CUST-OLD-002", stored.CustomerCode)
}
