package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Extracted data structures
type CostingData struct {
	FilePath     string
	FolderNumber string
	Customer     string
	Date         string
	Salesperson  string
	Supplier     string
	TotalValue   float64
	Products     []ProductInfo
}

type ProductInfo struct {
	Model       string
	Spec        string
	SupplierRef string
	Quantity    float64
	UnitPrice   float64
	TotalPrice  float64
}

// Database models
type Base struct {
	ID        string `gorm:"primaryKey;size:36"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	Version   int            `gorm:"default:1"`
	CreatedBy string
}

type CustomerMaster struct {
	Base
	CustomerID    string `gorm:"uniqueIndex;size:50"`
	CustomerCode  string `gorm:"uniqueIndex;size:50"`
	CustomerType  string `gorm:"index;size:50"`
	BusinessName  string `gorm:"index;size:255"`
	ShortCode     string `gorm:"size:10"`
	City          string `gorm:"index;size:100"`
	Country       string `gorm:"index;size:100"`
	Industry      string `gorm:"index;size:100"`
	CustomerGrade string `gorm:"index;size:10"`
	PaymentGrade  string `gorm:"index;size:10"`
}

func (CustomerMaster) TableName() string { return "customers" }

type Opportunity struct {
	Base
	FolderNumber string    `gorm:"uniqueIndex;size:50"`
	CustomerID   string    `gorm:"index;size:36"`
	CustomerName string    `gorm:"index;size:255"`
	Salesperson  string    `gorm:"index;size:100"`
	OfferDate    time.Time `gorm:"index"`
	RevenueBHD   float64
	Stage        string `gorm:"index;size:50"`
	ProductType  string `gorm:"index;size:100"`
}

func (Opportunity) TableName() string { return "opportunities" }

// Known customer mappings (abbreviations to full names)
var customerMappings = map[string]string{
	"EWA":       "Electricity & Water Authority",
	"ARLA":      "Arla Foods Bahrain S.P.C",
	"ALBA":      "Aluminium Bahrain B.S.C.(C)",
	"BAPCO":     "Bapco Energies",
	"GPIC":      "Gulf Petrochemical Industries Co.",
	"SULB":      "SULB Company B.S.C.",
	"ENGIE":     "Engie Power & Water",
	"TTSJV":     "Technip-Technicas Reunidas JV",
	"SEAPEAK":   "Seapeak Maritime",
	"MOW":       "Ministry of Works",
	"ALUSERV":   "Aluserv Middle East W.L.L.",
	"INTERCOL":  "Intercol",
	"VEOLIA":    "Veolia Water Technologies",
	"NOMAC":     "Nomac",
	"BANAGAS":   "Banagas",
	"TABREED":   "Tabreed",
	"GARMCO":    "Gulf Aluminium Rolling Mill",
	"BSR":       "Bahrain Sugar Refinery",
	"ASRY":      "Arab Shipbuilding & Repair Yard",
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	basePath := "Offers No 1-50 (2025)"
	dbFile := "ph_holdings.db"

	fmt.Println("🔍 Extracting customer data from costing sheets...")
	fmt.Printf("📁 Base path: %s\n", basePath)

	// Find all costing sheets
	var costingFiles []string
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() && strings.HasSuffix(strings.ToLower(path), ".xlsx") {
			lower := strings.ToLower(info.Name())
			if strings.Contains(lower, "costing") || strings.Contains(lower, "masterfile") {
				costingFiles = append(costingFiles, path)
			}
		}
		return nil
	})
	if err != nil {
		log.Fatalf("Failed to walk directory: %v", err)
	}

	fmt.Printf("📊 Found %d costing sheets\n\n", len(costingFiles))

	// Extract data from each costing sheet
	var allData []CostingData
	customers := make(map[string]bool)

	for _, file := range costingFiles {
		data, err := extractCostingData(file)
		if err != nil {
			fmt.Printf("⚠️ Error processing %s: %v\n", filepath.Base(file), err)
			continue
		}
		if data.Customer != "" {
			allData = append(allData, data)
			normalizedCustomer := normalizeCustomerName(data.Customer)
			customers[normalizedCustomer] = true
			fmt.Printf("✅ %s → Customer: %s, Value: %.3f BHD\n",
				filepath.Base(file), normalizedCustomer, data.TotalValue)
		}
	}

	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   Costing sheets processed: %d\n", len(allData))
	fmt.Printf("   Unique customers found: %d\n", len(customers))

	// Save extracted data to JSON for reference
	jsonData, _ := json.MarshalIndent(allData, "", "  ")
	os.WriteFile("extracted_costing_data.json", jsonData, 0644)
	fmt.Println("   Saved to: extracted_costing_data.json")

	// Update database
	fmt.Println("\n🗄️ Updating database...")
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Clear existing data for fresh import
	db.Exec("DELETE FROM customers")
	db.Exec("DELETE FROM opportunities")

	// Insert unique customers
	customerIDs := make(map[string]string) // name -> ID
	for customerName := range customers {
		id := uuid.New().String()
		code := generateCustomerCode(customerName)

		customer := CustomerMaster{
			Base: Base{
				ID:        id,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			CustomerID:    id,
			CustomerCode:  code,
			BusinessName:  customerName,
			CustomerType:  "Industrial",
			Country:       "Bahrain",
			City:          "Manama",
			Industry:      "Process Instrumentation",
			CustomerGrade: "B",
			PaymentGrade:  "B",
		}
		db.Create(&customer)
		customerIDs[customerName] = id
	}
	fmt.Printf("   Created %d customers\n", len(customers))

	// Insert opportunities from costing data
	opportunitiesCreated := 0
	for _, data := range allData {
		customerName := normalizeCustomerName(data.Customer)
		customerID := customerIDs[customerName]

		// Parse date
		offerDate := parseDate(data.Date)

		opp := Opportunity{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			FolderNumber: data.FolderNumber,
			CustomerID:   customerID,
			CustomerName: customerName,
			Salesperson:  data.Salesperson,
			OfferDate:    offerDate,
			RevenueBHD:   data.TotalValue,
			Stage:        "Quoted",
			ProductType:  data.Supplier,
		}
		if err := db.Create(&opp).Error; err == nil {
			opportunitiesCreated++
		}
	}
	fmt.Printf("   Created %d opportunities\n", opportunitiesCreated)

	// Copy to build
	input, _ := os.ReadFile(dbFile)
	os.WriteFile("build/bin/data/ph_holdings.db", input, 0644)
	fmt.Println("   Database deployed to build/bin/data/")

	fmt.Println("\n✨ Done!")
}

func extractCostingData(filePath string) (CostingData, error) {
	data := CostingData{FilePath: filePath}

	// Extract folder number from path
	parts := strings.Split(filePath, string(os.PathSeparator))
	for _, part := range parts {
		if len(part) > 2 && (part[0] >= '0' && part[0] <= '9') {
			// Extract leading number
			re := regexp.MustCompile(`^(\d+)`)
			if match := re.FindString(part); match != "" {
				data.FolderNumber = fmt.Sprintf("2025-%s", match)
				break
			}
		}
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return data, err
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return data, fmt.Errorf("no sheets found")
	}

	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return data, err
	}

	// Parse rows looking for key fields
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		firstCell := strings.ToUpper(strings.TrimSpace(row[0]))

		// Customer name
		if strings.Contains(firstCell, "CUSTOMER") {
			if len(row) > 1 {
				data.Customer = strings.TrimSpace(row[1])
			} else {
				// Customer name might be in same cell after label
				parts := strings.SplitN(row[0], " ", 2)
				if len(parts) > 1 {
					data.Customer = strings.TrimSpace(parts[1])
				}
			}
		}

		// Date
		if strings.Contains(firstCell, "DATE") {
			if len(row) > 1 {
				data.Date = strings.TrimSpace(row[1])
			}
		}

		// Salesperson
		if strings.Contains(firstCell, "COSTING MADE BY") || strings.Contains(firstCell, "PREPARED BY") {
			if len(row) > 1 {
				data.Salesperson = strings.TrimSpace(row[1])
			} else {
				parts := strings.SplitN(row[0], " ", 4)
				if len(parts) > 3 {
					data.Salesperson = strings.TrimSpace(parts[3])
				}
			}
		}

		// Supplier
		if strings.Contains(firstCell, "SUPPLIER") {
			if len(row) > 1 {
				data.Supplier = strings.TrimSpace(row[1])
			}
		}

		// Total value - look for TOTAL SP or similar
		if strings.Contains(firstCell, "TOTAL SP") || strings.Contains(firstCell, "TOTAL") && strings.Contains(firstCell, "VAT") {
			// Find numeric value in row
			for _, cell := range row {
				val := parseFloat(cell)
				if val > data.TotalValue {
					data.TotalValue = val
				}
			}
		}

		// Also check for Total row with large values
		if i > 30 && strings.Contains(firstCell, "TOTAL") {
			for _, cell := range row {
				val := parseFloat(cell)
				if val > 1000 && val > data.TotalValue {
					data.TotalValue = val
				}
			}
		}
	}

	return data, nil
}

func normalizeCustomerName(name string) string {
	name = strings.TrimSpace(name)
	upper := strings.ToUpper(name)

	// Check mappings
	for abbr, fullName := range customerMappings {
		if strings.Contains(upper, abbr) {
			return fullName
		}
	}

	// Clean up common patterns
	name = strings.TrimSuffix(name, " spare")
	name = strings.TrimSuffix(name, " Spare")
	name = strings.TrimSuffix(name, " FIT")
	name = strings.TrimSuffix(name, " TIT")
	name = strings.TrimSuffix(name, " AIT")
	name = strings.TrimSuffix(name, " PIT")

	// Title case
	if len(name) > 0 {
		return strings.Title(strings.ToLower(name))
	}
	return name
}

func generateCustomerCode(name string) string {
	// Extract first letters of each word
	words := strings.Fields(name)
	code := ""
	for _, word := range words {
		if len(word) > 0 {
			code += strings.ToUpper(string(word[0]))
		}
		if len(code) >= 6 {
			break
		}
	}
	return code + "-" + uuid.New().String()[:4]
}

func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	formats := []string{
		"02-01-2006",
		"2-Jan-2006",
		"02/01/2006",
		"2006-01-02",
		"02-Jan-06",
		"2-1-2006",
		"30-12-2024",
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}
	return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	s = strings.ReplaceAll(s, "BHD", "")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
