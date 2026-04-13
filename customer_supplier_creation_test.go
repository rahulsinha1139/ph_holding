package main

import (
	"encoding/json"
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
			Permissions: `["customers:create","customers:view","customers:edit","suppliers:create","suppliers:view","suppliers:edit","suppliers:update","suppliers:delete"]`,
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

func TestSupplierUpdate_PreservesIdentifiersWhenOmitted(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.CreateSupplier(SupplierMaster{
		SupplierName: "Update Identifier Supplier",
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		LeadTimeDays: 14,
	})
	require.NoError(t, err)

	updated, err := app.UpdateSupplier(SupplierMaster{
		Base:         Base{ID: created.ID},
		SupplierName: "",
		SupplierCode: "",
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		LeadTimeDays: 21,
	})
	require.NoError(t, err)
	require.Equal(t, created.SupplierName, updated.SupplierName)
	require.Equal(t, created.SupplierCode, updated.SupplierCode)
	require.Equal(t, 21, updated.LeadTimeDays)
}

func TestSupplierUpdate_PreservesRatingWhenOmitted(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.CreateSupplier(SupplierMaster{
		SupplierName: "Rating Supplier",
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		Rating:       5,
		LeadTimeDays: 14,
	})
	require.NoError(t, err)

	updated, err := app.UpdateSupplier(SupplierMaster{
		Base:         Base{ID: created.ID},
		SupplierName: created.SupplierName,
		SupplierCode: created.SupplierCode,
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		Rating:       0,
		LeadTimeDays: 30,
	})
	require.NoError(t, err)
	require.Equal(t, 5, updated.Rating)
}

func TestSupplierUpdate_ValidatesInput(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.CreateSupplier(SupplierMaster{
		SupplierName: "Update Validation Supplier",
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		LeadTimeDays: 14,
	})
	require.NoError(t, err)

	updated, err := app.UpdateSupplier(SupplierMaster{
		Base:         Base{ID: created.ID},
		SupplierName: created.SupplierName,
		SupplierCode: created.SupplierCode,
		SupplierType: "InvalidType",
		Country:      "Bahrain",
		LeadTimeDays: 14,
	})
	require.Error(t, err)
	require.Nil(t, updated)
	require.Contains(t, err.Error(), "Invalid supplier data")
}

func TestCreateSupplierFromButler_UsesSupplierCreatePolicy(t *testing.T) {
	app := makeCreationTestApp(t)

	created, err := app.CreateSupplierFromButler(ButlerSupplierRequest{
		SupplierName: "Butler Supplier",
		LeadTimeDays: 21,
	})
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, "Manufacturer", created.SupplierType)
	require.Equal(t, "Bahrain", created.Country)
	require.NotEmpty(t, created.SupplierCode)
	require.Equal(t, 3, created.Rating)
}

func TestCanonicalSeedSupplierSpecs_MergesExtractedSupplierProducts(t *testing.T) {
	specs := canonicalSeedSupplierSpecs([]ExtractedSupplier{
		{
			NormalizedName: "Endress+Hauser",
			Products:       []string{"Flow meters", "Level transmitters"},
		},
		{
			NormalizedName: "New Supplier",
			Products:       []string{"Analyzers"},
		},
	})

	var endress supplierSeedSpec
	var foundEndress bool
	var foundNew bool
	for _, spec := range specs {
		if spec.Name == "Endress+Hauser" {
			endress = spec
			foundEndress = true
		}
		if spec.Name == "New Supplier" {
			foundNew = true
		}
	}

	require.True(t, foundEndress)
	require.True(t, foundNew)
	require.Contains(t, endress.ProductTypes, "Flow meters")
	require.Contains(t, endress.ProductTypes, "Level transmitters")
}

func TestEnsureSeedSupplier_CreatesCanonicalSupplier(t *testing.T) {
	app := makeCreationTestApp(t)

	supplier, created, err := app.ensureSeedSupplier(supplierSeedSpec{
		Name:         "Seeded Supplier",
		Code:         "SEED",
		Country:      "Bahrain",
		PaymentTerms: "Net 30",
		SupplierType: "Manufacturer",
		Rating:       5,
		ProductTypes: []string{"Analyzers"},
	})
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, supplier)
	require.Equal(t, "SEED", supplier.SupplierCode)
	require.Equal(t, "Manufacturer", supplier.SupplierType)
	require.Equal(t, 5, supplier.Rating)

	var stored SupplierMaster
	require.NoError(t, app.db.First(&stored, "id = ?", supplier.ID).Error)
	var products []string
	require.NoError(t, json.Unmarshal([]byte(stored.ProductTypes), &products))
	require.Equal(t, []string{"Analyzers"}, products)
}

func TestEnsureSeedSupplier_EnrichesExistingSupplierWithoutChangingIdentity(t *testing.T) {
	app := makeCreationTestApp(t)

	existing, err := app.CreateSupplier(SupplierMaster{
		SupplierName: "Existing Seed Supplier",
		SupplierType: "Manufacturer",
		Country:      "",
		LeadTimeDays: 10,
	})
	require.NoError(t, err)

	supplier, created, err := app.ensureSeedSupplier(supplierSeedSpec{
		Name:         "existing seed supplier",
		Code:         "EXIST",
		Country:      "Germany",
		PaymentTerms: "Net 60",
		SupplierType: "Manufacturer",
		Rating:       5,
		ProductTypes: []string{"Flow meters", "Analyzers"},
	})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, existing.ID, supplier.ID)
	require.Equal(t, existing.SupplierCode, supplier.SupplierCode)

	var stored SupplierMaster
	require.NoError(t, app.db.First(&stored, "id = ?", existing.ID).Error)
	require.Equal(t, "Germany", stored.Country)
	require.Equal(t, "Net 30", stored.PaymentTerms)
	require.Equal(t, 3, stored.Rating)

	var products []string
	require.NoError(t, json.Unmarshal([]byte(stored.ProductTypes), &products))
	require.Equal(t, []string{"Flow meters", "Analyzers"}, products)
}

func TestCreateSupplier_SyncsPrimaryContactRecord(t *testing.T) {
	app := makeCreationTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierContact{}))

	created, err := app.CreateSupplier(SupplierMaster{
		SupplierName:   "Contact Sync Supplier",
		SupplierType:   "Manufacturer",
		Country:        "Bahrain",
		PrimaryContact: "Amina",
		Email:          "amina@supplier.example",
		Phone:          "+973-1111-2222",
		LeadTimeDays:   14,
	})
	require.NoError(t, err)

	var contacts []SupplierContact
	require.NoError(t, app.db.Where("supplier_id = ?", created.ID).Find(&contacts).Error)
	require.Len(t, contacts, 1)
	require.True(t, contacts[0].IsPrimaryContact)
	require.Equal(t, "Amina", contacts[0].ContactName)
	require.Equal(t, "amina@supplier.example", contacts[0].Email)
	require.Equal(t, "+973-1111-2222", contacts[0].Phone)
}

func TestAddSupplierContact_PrimarySyncsSupplierMasterAndDemotesPreviousPrimary(t *testing.T) {
	app := makeCreationTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierContact{}))

	created, err := app.CreateSupplier(SupplierMaster{
		SupplierName: "Supplier Contact Demotion",
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		LeadTimeDays: 14,
	})
	require.NoError(t, err)

	first, err := app.AddSupplierContact(SupplierContact{
		SupplierID:       created.ID,
		ContactName:      "First Contact",
		Email:            "first@supplier.example",
		IsPrimaryContact: true,
	})
	require.NoError(t, err)

	second, err := app.AddSupplierContact(SupplierContact{
		SupplierID:       created.ID,
		ContactName:      "Second Contact",
		Email:            "second@supplier.example",
		Phone:            "+973-4444-5555",
		IsPrimaryContact: true,
	})
	require.NoError(t, err)

	var contacts []SupplierContact
	require.NoError(t, app.db.Order("created_at ASC").Where("supplier_id = ?", created.ID).Find(&contacts).Error)
	require.Len(t, contacts, 2)
	require.Equal(t, first.ID, contacts[0].ID)
	require.False(t, contacts[0].IsPrimaryContact)
	require.Equal(t, second.ID, contacts[1].ID)
	require.True(t, contacts[1].IsPrimaryContact)

	var stored SupplierMaster
	require.NoError(t, app.db.First(&stored, "id = ?", created.ID).Error)
	require.Equal(t, "Second Contact", stored.PrimaryContact)
	require.Equal(t, "second@supplier.example", stored.Email)
	require.Equal(t, "+973-4444-5555", stored.Phone)
}

func TestGetSupplierFullProfile_FallsBackToPrimaryContact(t *testing.T) {
	app := makeCreationTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierContact{}))

	created, err := app.CreateSupplier(SupplierMaster{
		SupplierName: "Profile Fallback Supplier",
		SupplierType: "Manufacturer",
		Country:      "Bahrain",
		LeadTimeDays: 14,
	})
	require.NoError(t, err)

	require.NoError(t, app.db.Create(&SupplierContact{
		SupplierID:       created.ID,
		ContactName:      "Fallback Contact",
		Email:            "fallback@supplier.example",
		Phone:            "+973-7777-8888",
		IsPrimaryContact: true,
	}).Error)

	profile, err := app.GetSupplierFullProfile(created.ID)
	require.NoError(t, err)
	require.Equal(t, "Fallback Contact", profile.PrimaryContact)
	require.Equal(t, "fallback@supplier.example", profile.Email)
	require.Equal(t, "+973-7777-8888", profile.Phone)
}

func TestDeleteSupplierContact_ClearsSupplierPrimaryFields(t *testing.T) {
	app := makeCreationTestApp(t)
	require.NoError(t, app.db.AutoMigrate(&SupplierContact{}))

	created, err := app.CreateSupplier(SupplierMaster{
		SupplierName:   "Delete Contact Supplier",
		SupplierType:   "Manufacturer",
		Country:        "Bahrain",
		PrimaryContact: "Delete Me",
		Email:          "delete@supplier.example",
		Phone:          "+973-0000-1111",
		LeadTimeDays:   14,
	})
	require.NoError(t, err)

	var contact SupplierContact
	require.NoError(t, app.db.Where("supplier_id = ? AND is_primary_contact = ?", created.ID, true).First(&contact).Error)

	require.NoError(t, app.DeleteSupplierContact(contact.ID))

	var stored SupplierMaster
	require.NoError(t, app.db.First(&stored, "id = ?", created.ID).Error)
	require.Equal(t, "", stored.PrimaryContact)
	require.Equal(t, "", stored.Email)
	require.Equal(t, "", stored.Phone)
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
