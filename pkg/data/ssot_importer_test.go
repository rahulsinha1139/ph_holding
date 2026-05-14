package data

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestImportCustomersFromCSV tests CSV customer import
func TestImportCustomersFromCSV(t *testing.T) {
	// Setup in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	// Auto-migrate
	if err := db.AutoMigrate(&CustomerMaster{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create test CSV
	testCSV := createTestCustomerCSV(t)
	defer os.Remove(testCSV)

	// Execute import
	result := &ImportResult{}
	err = importCustomersFromCSV(db, testCSV, result)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}

	// Verify results
	if result.CustomersTotal < 1 {
		t.Errorf("Expected at least 1 customer, got %d", result.CustomersTotal)
	}

	if result.CustomersImported < 1 {
		t.Errorf("Expected at least 1 imported, got %d", result.CustomersImported)
	}

	// Verify database records
	var count int64
	db.Model(&CustomerMaster{}).Count(&count)
	if count != int64(result.CustomersImported) {
		t.Errorf("Expected %d records in DB, got %d", result.CustomersImported, count)
	}

	t.Logf("✅ Imported %d/%d customers", result.CustomersImported, result.CustomersTotal)
}

// TestBatchInsertion tests Williams batching performance
func TestBatchInsertion(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&CustomerMaster{}); err != nil {
		t.Fatalf("Failed to migrate: %v", err)
	}

	// Create batch of customers (each with unique ID)
	batch := make([]CustomerMaster, 50)
	for i := range batch {
		batch[i] = CustomerMaster{
			CustomerID:   fmt.Sprintf("EC%03d", i),
			BusinessName: fmt.Sprintf("Test Company %d", i),
			CustomerType: "End Customer",
			Industry:     "Test Industry",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
	}

	// Execute batch insert
	result := &ImportResult{}
	start := time.Now()
	if err := insertCustomerBatch(db, batch, result); err != nil {
		t.Fatalf("Batch insert failed: %v", err)
	}
	duration := time.Since(start)

	t.Logf("✅ Batch insert of 50 customers: %v", duration)

	if result.CustomersImported != 50 {
		t.Errorf("Expected 50 imported, got %d", result.CustomersImported)
	}

	// Verify all records exist
	var count int64
	db.Model(&CustomerMaster{}).Count(&count)
	if count != 50 {
		t.Errorf("Expected 50 records in DB, got %d", count)
	}
}

// TestCustomerIDGeneration tests customer ID generation logic
func TestCustomerIDGeneration(t *testing.T) {
	tests := []struct {
		businessName string
		customerType string
		expectedPrefix string
	}{
		{"BASF Bahrain", "End Customer", "EC"},
		{"Nass Corporation", "Engineering Company", "EG"},
		{"ARROWFINCH TECHNOLOGIES", "System Integrator", "SI"},
		{"Al Bakali Trading", "National Reseller", "NR"},
		{"Generic Company", "Unknown Type", "GC"},
	}

	for _, tt := range tests {
		id := generateCustomerID(tt.businessName, tt.customerType)
		if id[:2] != tt.expectedPrefix {
			t.Errorf("Expected prefix %s for type %s, got %s",
				tt.expectedPrefix, tt.customerType, id[:2])
		}

		// Verify format: PREFIX + 3 digits
		if len(id) != 5 {
			t.Errorf("Expected ID length 5, got %d: %s", len(id), id)
		}

		t.Logf("✅ %s → %s", tt.businessName, id)
	}
}

// TestShortCodeExtraction tests customer type short code extraction
func TestShortCodeExtraction(t *testing.T) {
	tests := []struct {
		customerType string
		expected     string
	}{
		{"End Customer", "EC"},
		{"Engineering Company", "EG"},
		{"System Integrator", "SI"},
		{"National Reseller", "NR"},
		{"International Reseller", "IR"},
		{"Plant Builder", "PB"},
		{"Service Provider", "SP"},
		{"Consultant", "CO"},
		{"OEM", "OE"},
		{"Random Type", "GC"},
	}

	for _, tt := range tests {
		code := extractShortCode(tt.customerType)
		if code != tt.expected {
			t.Errorf("Expected %s for %s, got %s", tt.expected, tt.customerType, code)
		}
	}
}

// TestParseFloat tests float parsing with comma removal
func TestParseFloat(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"1234.56", 1234.56},
		{"1,234.56", 1234.56},
		{"  456.78  ", 456.78},
		{"0", 0.0},
		{"invalid", 0.0},
	}

	for _, tt := range tests {
		result := parseFloat(tt.input)
		if result != tt.expected {
			t.Errorf("parseFloat(%q) = %f, expected %f", tt.input, result, tt.expected)
		}
	}
}

// TestParseDate tests date parsing with multiple formats
func TestParseDate(t *testing.T) {
	tests := []struct {
		input       string
		shouldParse bool
	}{
		{"2025-01-15", true},
		{"15/01/2025", true},
		{"01/15/2025", true},
		{"2025/01/15", true},
		{"invalid-date", false},
		{"", false},
	}

	for _, tt := range tests {
		result, err := parseDate(tt.input)
		if tt.shouldParse {
			if err != nil {
				t.Errorf("parseDate(%q) failed: %v", tt.input, err)
			}
			if result.IsZero() {
				t.Errorf("parseDate(%q) returned zero time", tt.input)
			}
		} else {
			if err == nil {
				t.Errorf("parseDate(%q) should have failed", tt.input)
			}
		}
	}
}

// BenchmarkBatchInsert benchmarks batch insertion performance
func BenchmarkBatchInsert(b *testing.B) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatalf("Failed to open database: %v", err)
	}

	if err := db.AutoMigrate(&CustomerMaster{}); err != nil {
		b.Fatalf("Failed to migrate: %v", err)
	}

	// Create test batch
	batch := make([]CustomerMaster, 50)
	for i := range batch {
		batch[i] = CustomerMaster{
			CustomerID:   generateCustomerID("Test Company", "End Customer"),
			BusinessName: "Test Company",
			CustomerType: "End Customer",
			Industry:     "Test Industry",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}
	}

	result := &ImportResult{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		insertCustomerBatch(db, batch, result)
	}
}

// =============================================================================
// TEST HELPERS
// =============================================================================

func createTestCustomerCSV(t *testing.T) string {
	// Create temporary CSV file
	tmpFile := filepath.Join(os.TempDir(), "test_customers.csv")

	content := `customer_group,business_account_name,account_number,customer_type,industry,detailed_industry,email,email_domain,city,country_iso,postal_code,phone_extension,website,data_completeness_score
,Panorama Contrg & Eng. Services,,"Engineering Company,","Dealer, Reseller",Dealer national,info@panoramabh.com,panoramabh.com,Juffair,BH,5602,+97317729935,www.panoramabh.com/,100
,Fine Foods,,End Customer,Food & Beverage,food & beverage general,info@finefoodsbah.com,finefoodsbah.com,MANAMA,BH,302,17727300,https://www.finefoodsco.com,100
,BASF Bahrain,,End Customer,Chemical,chemical general,husain.matar@basf.com,basf.com,"Al Hidd,",BH,0000,17585230,www.basf.com,100
`

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test CSV: %v", err)
	}

	return tmpFile
}
