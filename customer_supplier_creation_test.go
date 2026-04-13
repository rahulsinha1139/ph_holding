package main

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func makeCreationTestApp(t *testing.T) *App {
	t.Helper()

	app := setupTestApp(t)
	app.startupImporting = false
	app.currentUser = &User{
		Base:     Base{ID: "test-user"},
		Username: "test-admin",
		RoleName: "admin",
		Role: Role{
			Name:        "admin",
			DisplayName: "Admin",
			Permissions: `["customers:create","customers:view","customers:edit","suppliers:create","suppliers:view"]`,
		},
	}
	return app
}

func TestCustomerCreation_WithValidRole(t *testing.T) {
	app := makeCreationTestApp(t)

	customer := CustomerMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BusinessName:     "Creation Check Customer",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	}

	created, err := app.CreateCustomer(customer)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEmpty(t, created.CustomerCode)
	require.NotEmpty(t, created.CustomerID)
	require.Equal(t, created.CustomerCode, created.CustomerID)
	require.Equal(t, "Creation Check Customer", created.BusinessName)
}

func TestCustomerCreation_UsesProvidedCustomerIDAsCodeFallback(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.CreateCustomer(CustomerMaster{
		BusinessName:     "Explicit ID Customer",
		CustomerID:       "CUST-EXPLICIT-42",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, "CUST-EXPLICIT-42", created.CustomerID)
	require.Equal(t, "CUST-EXPLICIT-42", created.CustomerCode)
}

func TestCustomerUpdate_PreservesIdentifiersWhenOmitted(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.CreateCustomer(CustomerMaster{
		BusinessName:     "Update Identifier Customer",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	})
	require.NoError(t, err)

	updated, err := app.UpdateCustomer(CustomerMaster{
		Base:             Base{ID: created.ID},
		BusinessName:     "",
		CustomerID:       "",
		CustomerCode:     "",
		CustomerType:     "Corporate",
		City:             "Muharraq",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	})
	require.NoError(t, err)
	require.Equal(t, created.BusinessName, updated.BusinessName)
	require.Equal(t, created.CustomerID, updated.CustomerID)
	require.Equal(t, created.CustomerCode, updated.CustomerCode)
	require.Equal(t, "Muharraq", updated.City)
}

func TestCustomerUpdate_ValidatesInput(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.CreateCustomer(CustomerMaster{
		BusinessName:     "Update Validation Customer",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	})
	require.NoError(t, err)

	updated, err := app.UpdateCustomer(CustomerMaster{
		Base:             Base{ID: created.ID},
		BusinessName:     created.BusinessName,
		CustomerID:       created.CustomerID,
		CustomerCode:     created.CustomerCode,
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "Z",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	})
	require.Error(t, err)
	require.Nil(t, updated)
	require.Contains(t, err.Error(), "Invalid customer data")
}

func TestCreateCustomerFromButler_UsesCustomerCreatePolicy(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.CreateCustomerFromButler(ButlerCustomerRequest{
		BusinessName: "Butler Customer",
		PaymentGrade: "invalid",
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, "B", created.PaymentGrade)
	require.Equal(t, "Bahrain", created.Country)
	require.NotEmpty(t, created.CustomerCode)
	require.Equal(t, created.CustomerCode, created.CustomerID)
}

func TestFindOrCreateCustomer_UsesSharedIdentifierPolicy(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.findOrCreateCustomer("Import Helper Customer", "ihc")
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, "CUST-IHC", created.CustomerCode)
	require.Equal(t, created.CustomerCode, created.CustomerID)
	require.Equal(t, "Bahrain", created.Country)
}

func TestFindOrCreateCustomer_GeneratesFallbackIdentifierWithoutShortCode(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.findOrCreateCustomer("No Shortcode Customer", "")
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEmpty(t, created.CustomerCode)
	require.Equal(t, created.CustomerCode, created.CustomerID)
}

func TestSeedCustomerDatabase_UsesSharedCustomerPolicy(t *testing.T) {
	app := setupTestApp(t)

	err := app.SeedCustomerDatabase()
	require.NoError(t, err)

	var customers []CustomerMaster
	require.NoError(t, app.db.Order("business_name ASC").Find(&customers).Error)
	require.Len(t, customers, 5)
	for _, customer := range customers {
		require.NotEmpty(t, customer.CustomerID)
		require.NotEmpty(t, customer.CustomerCode)
		require.Equal(t, customer.CustomerID, customer.CustomerCode)
	}
}

func TestBackfillBusinessCustomerIDs_RepairsLegacyIdentifiers(t *testing.T) {
	app := makeCreationTestApp(t)

	legacyID := uuid.New().String()
	legacyCustomer := CustomerMaster{
		Base:             Base{ID: legacyID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BusinessName:     "Legacy Backfill Customer",
		CustomerID:       legacyID,
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	}
	require.NoError(t, app.db.Create(&legacyCustomer).Error)

	results, err := app.BackfillBusinessCustomerIDs()
	require.NoError(t, err)
	require.Equal(t, 1, results["updated"].(int))

	var stored CustomerMaster
	require.NoError(t, app.db.First(&stored, "id = ?", legacyID).Error)
	require.NotEmpty(t, stored.CustomerID)
	require.Equal(t, stored.CustomerID, stored.CustomerCode)
	require.NotEqual(t, legacyID, stored.CustomerID)
	require.True(t, strings.HasPrefix(stored.CustomerID, "CUST-"))
}

func TestBackfillBusinessCustomerIDs_PreservesExistingCustomerCode(t *testing.T) {
	app := makeCreationTestApp(t)

	legacyID := uuid.New().String()
	legacyCustomer := CustomerMaster{
		Base:             Base{ID: legacyID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BusinessName:     "Legacy Code Customer",
		CustomerID:       legacyID,
		CustomerCode:     "EC14",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	}
	require.NoError(t, app.db.Create(&legacyCustomer).Error)

	_, err := app.BackfillBusinessCustomerIDs()
	require.NoError(t, err)

	var stored CustomerMaster
	require.NoError(t, app.db.First(&stored, "id = ?", legacyID).Error)
	require.Equal(t, "EC14", stored.CustomerID)
	require.Equal(t, "EC14", stored.CustomerCode)
}

func TestGetCustomerFullProfile_RepairsLegacyDisplayIdentifier(t *testing.T) {
	app := makeCreationTestApp(t)

	legacyID := uuid.New().String()
	legacyCustomer := CustomerMaster{
		Base:             Base{ID: legacyID, CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BusinessName:     "Profile Legacy Customer",
		CustomerID:       legacyID,
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	}
	require.NoError(t, app.db.Create(&legacyCustomer).Error)

	profile, err := app.GetCustomerFullProfile(legacyID)
	require.NoError(t, err)
	require.NotEqual(t, legacyID, profile.CustomerID)
	require.True(t, strings.HasPrefix(profile.CustomerID, "CUST-"))
}

func TestExtractPrimarySeedCustomerContact_UsesFirstNonEmptyTrimmedValues(t *testing.T) {
	email, phone := extractPrimarySeedCustomerContact(ExtractedCustomer{
		EmailAddresses: []string{"", "  sales@example.com  ", "other@example.com"},
		PhoneNumbers:   []string{" ", "  +973 1234 5678  ", "999"},
	})

	require.Equal(t, "sales@example.com", email)
	require.Equal(t, "+973 1234 5678", phone)
}

func TestEnsureSeedCustomerFromExtracted_CreatesCanonicalCustomerAndPrimaryContact(t *testing.T) {
	app := makeCreationTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerContact{}))

	customer, created, err := app.ensureSeedCustomerFromExtracted(ExtractedCustomer{
		NormalizedName: "Seeded Opportunity Customer",
		EmailAddresses: []string{"sales@seeded.example"},
		PhoneNumbers:   []string{"+973-1700-0000"},
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, customer)
	require.Equal(t, customer.CustomerCode, customer.CustomerID)
	require.Equal(t, "Corporate", customer.CustomerType)
	require.Equal(t, "B", customer.PaymentGrade)
	require.Equal(t, "Active", customer.Status)

	var contacts []CustomerContact
	require.NoError(t, app.db.Where("customer_id = ?", customer.ID).Find(&contacts).Error)
	require.Len(t, contacts, 1)
	require.True(t, contacts[0].IsPrimaryContact)
	require.Equal(t, "sales@seeded.example", contacts[0].Email)
	require.Equal(t, "+973-1700-0000", contacts[0].Phone)
}

func TestEnsureSeedCustomerFromExtracted_EnrichesExistingCustomerWithoutDuplicateContact(t *testing.T) {
	app := makeCreationTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerContact{}))

	existing, err := app.CreateCustomer(CustomerMaster{
		BusinessName:     "Existing Seed Customer",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	})
	require.NoError(t, err)

	customer, created, err := app.ensureSeedCustomerFromExtracted(ExtractedCustomer{
		NormalizedName: "existing seed customer",
		EmailAddresses: []string{"ops@example.com"},
		PhoneNumbers:   []string{"+973-9988-7766"},
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, existing.ID, customer.ID)

	customer, created, err = app.ensureSeedCustomerFromExtracted(ExtractedCustomer{
		NormalizedName: "Existing Seed Customer",
		EmailAddresses: []string{"ops@example.com"},
		PhoneNumbers:   []string{"+973-9988-7766"},
	})
	require.NoError(t, err)
	require.False(t, created)

	var contacts []CustomerContact
	require.NoError(t, app.db.Where("customer_id = ?", existing.ID).Find(&contacts).Error)
	require.Len(t, contacts, 1)
	require.True(t, contacts[0].IsPrimaryContact)
	require.Equal(t, "ops@example.com", contacts[0].Email)
	require.Equal(t, "+973-9988-7766", contacts[0].Phone)
}

func TestEnsureSeedCustomerFromExtracted_DoesNotCreateBlankContact(t *testing.T) {
	app := makeCreationTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&CustomerContact{}))

	customer, created, err := app.ensureSeedCustomerFromExtracted(ExtractedCustomer{
		NormalizedName: "Seeded Without Contact",
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, customer)

	var contacts []CustomerContact
	require.NoError(t, app.db.Where("customer_id = ?", customer.ID).Find(&contacts).Error)
	require.Len(t, contacts, 0)
}

func TestSupplierCreation_WithValidRole(t *testing.T) {
	app := makeCreationTestApp(t)

	supplier := SupplierMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierName: "Creation Check Supplier",
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		LeadTimeDays: 14,
	}

	created, err := app.CreateSupplier(supplier)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.NotEmpty(t, created.SupplierCode)
	require.Equal(t, "Creation Check Supplier", created.SupplierName)
	require.Equal(t, 3, created.Rating)
}

func TestCustomerCreation_RequiresCreatePermission(t *testing.T) {
	app := makeCreationTestApp(t)
	app.currentUser = &User{
		Base:     Base{ID: "limited-user"},
		Username: "limited-sales",
		RoleName: "sales",
		Role: Role{
			Name:        "sales",
			DisplayName: "Sales",
			Permissions: `["customers:view"]`,
		},
	}

	customer := CustomerMaster{
		Base:             Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		BusinessName:     "Blocked Customer",
		CustomerType:     "Corporate",
		Country:          "Bahrain",
		PaymentGrade:     "B",
		CustomerGrade:    "B",
		PaymentTermsDays: 30,
	}

	created, err := app.CreateCustomer(customer)
	require.Error(t, err)
	require.Nil(t, created)
	require.Contains(t, err.Error(), "customers:create")
}

func TestSupplierCreation_RequiresCreatePermission(t *testing.T) {
	app := makeCreationTestApp(t)
	app.currentUser = &User{
		Base:     Base{ID: "limited-user"},
		Username: "limited-sales",
		RoleName: "sales",
		Role: Role{
			Name:        "sales",
			DisplayName: "Sales",
			Permissions: `["suppliers:view"]`,
		},
	}

	supplier := SupplierMaster{
		Base:         Base{ID: uuid.New().String(), CreatedAt: time.Now(), UpdatedAt: time.Now()},
		SupplierName: "Blocked Supplier",
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		LeadTimeDays: 14,
	}

	created, err := app.CreateSupplier(supplier)
	require.Error(t, err)
	require.Nil(t, created)
	require.Contains(t, err.Error(), "suppliers:create")
}
