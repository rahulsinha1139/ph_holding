package main

import (
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

// Customer name mappings from abbreviations
var customerMappings = map[string]string{
	"EWA":        "Electricity & Water Authority",
	"ARLA":       "Arla Foods Bahrain S.P.C",
	"ALBA":       "Aluminium Bahrain B.S.C.(C)",
	"BAPCO":      "Bapco Energies",
	"GPIC":       "Gulf Petrochemical Industries Co.",
	"SULB":       "SULB Company B.S.C.",
	"ENGIE":      "Al Ezzel Engie O&M Co. W.L.L",
	"TTSJV":      "Technip-Technicas Reunidas JV",
	"SEAPEAK":    "Seapeak Maritime",
	"MOW":        "Ministry of Works",
	"ALUSERV":    "Aluserv Middle East W.L.L.",
	"INTERCOL":   "Intercol",
	"VEOLIA":     "Veolia Water Technologies",
	"NOMAC":      "Nomac Bahrain",
	"BANAGAS":    "Banagas Co. B.S.C.",
	"YOKOGAWA":   "Yokogawa Middle East",
	"BSR":        "Bahrain Sugar Refinery",
	"MUHARRAQ":   "Muharraq Wastewater Services",
	"WEST POINT": "West Point Engineering",
	"WESTPOINT":  "West Point Engineering",
	"TECTON":     "Tecton Engineering",
	"KRAUM":      "Kraum & Spruk Technologies",
	"YATEEM":     "Yateem Group",
	"AIC":        "Arabian Industrial Chemicals",
	"AL FARIS":   "Al Faris",
}

func main() {
	log.SetFlags(log.Ltime | log.Lshortfile)

	basePath := "Offers No 1-50 (2025)"
	dbFile := "ph_holdings.db"

	fmt.Println("🔍 Extracting data from offer folders...")

	// Find all offer folders
	entries, err := os.ReadDir(basePath)
	if err != nil {
		log.Fatalf("Failed to read directory: %v", err)
	}

	type FolderData struct {
		Number       string
		CustomerAbbr string
		CustomerFull string
		ProductType  string
		TotalValue   float64
		Salesperson  string
		Date         time.Time
	}

	var folders []FolderData
	customerSet := make(map[string]bool)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		folderName := entry.Name()

		// Extract folder number and customer abbreviation
		re := regexp.MustCompile(`^(\d+)[\s\-]+(.+)$`)
		match := re.FindStringSubmatch(folderName)
		if match == nil {
			continue
		}

		folderNum := match[1]
		customerPart := strings.TrimSpace(match[2])

		// Extract customer abbreviation (first word, uppercase)
		words := strings.Fields(customerPart)
		customerAbbr := strings.ToUpper(words[0])

		// Remove common suffixes
		customerAbbr = strings.TrimSuffix(customerAbbr, "-")

		// Map to full name
		customerFull := mapToFullName(customerAbbr, customerPart)

		// Extract product type from folder name
		productType := extractProductType(customerPart)

		// Try to get value from costing sheet
		costingValue, salesperson, date := findCostingData(filepath.Join(basePath, folderName))

		folder := FolderData{
			Number:       folderNum,
			CustomerAbbr: customerAbbr,
			CustomerFull: customerFull,
			ProductType:  productType,
			TotalValue:   costingValue,
			Salesperson:  salesperson,
			Date:         date,
		}

		folders = append(folders, folder)
		customerSet[customerFull] = true

		fmt.Printf("📁 %s → %s (Value: %.3f BHD)\n", folderName, customerFull, costingValue)
	}

	fmt.Printf("\n📈 Summary:\n")
	fmt.Printf("   Folders processed: %d\n", len(folders))
	fmt.Printf("   Unique customers: %d\n", len(customerSet))

	// Update database
	fmt.Println("\n🗄️ Updating database...")
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	// Clear existing data
	db.Exec("DELETE FROM customers")
	db.Exec("DELETE FROM opportunities")

	// Insert customers
	customerIDs := make(map[string]string)
	for customerName := range customerSet {
		id := uuid.New().String()
		code := generateCode(customerName)

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
	fmt.Printf("   Created %d customers\n", len(customerSet))

	// Insert opportunities
	for _, folder := range folders {
		opp := Opportunity{
			Base: Base{
				ID:        uuid.New().String(),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			FolderNumber: fmt.Sprintf("2025-%s", folder.Number),
			CustomerID:   customerIDs[folder.CustomerFull],
			CustomerName: folder.CustomerFull,
			Salesperson:  folder.Salesperson,
			OfferDate:    folder.Date,
			RevenueBHD:   folder.TotalValue,
			Stage:        "Quoted",
			ProductType:  folder.ProductType,
		}
		db.Create(&opp)
	}
	fmt.Printf("   Created %d opportunities\n", len(folders))

	// Copy to build
	input, _ := os.ReadFile(dbFile)
	os.WriteFile("build/bin/data/ph_holdings.db", input, 0644)
	fmt.Println("   Database deployed to build/bin/data/")

	// Final verification
	var custCount, oppCount int64
	db.Model(&CustomerMaster{}).Count(&custCount)
	db.Model(&Opportunity{}).Count(&oppCount)
	fmt.Printf("\n✅ Final counts: %d customers, %d opportunities\n", custCount, oppCount)
}

func mapToFullName(abbr string, fullPart string) string {
	// Check exact mapping
	if full, ok := customerMappings[abbr]; ok {
		return full
	}

	// Check if any mapping key is contained in the full part
	upperPart := strings.ToUpper(fullPart)
	for key, full := range customerMappings {
		if strings.Contains(upperPart, key) {
			return full
		}
	}

	// Title case the first word if no mapping found
	words := strings.Fields(fullPart)
	if len(words) > 0 {
		// Return cleaned first word(s)
		name := strings.Title(strings.ToLower(words[0]))
		if len(words) > 1 && !isProductType(words[1]) {
			name += " " + strings.Title(strings.ToLower(words[1]))
		}
		return name
	}

	return fullPart
}

func isProductType(s string) bool {
	productTypes := []string{"FIT", "LIT", "TIT", "AIT", "PIT", "DPT", "DPIT", "PT", "LS", "BTU", "SP", "ELE", "MOD"}
	upper := strings.ToUpper(s)
	for _, pt := range productTypes {
		if upper == pt || strings.HasPrefix(upper, pt+"-") || strings.HasSuffix(upper, "-"+pt) {
			return true
		}
	}
	return false
}

func extractProductType(s string) string {
	productTypes := []string{"FIT", "LIT", "TIT", "AIT", "PIT", "DPT", "DPIT", "PT", "LS", "BTU", "SP", "ELE"}
	upper := strings.ToUpper(s)
	for _, pt := range productTypes {
		if strings.Contains(upper, pt) {
			return pt
		}
	}
	return ""
}

func findCostingData(folderPath string) (float64, string, time.Time) {
	var maxValue float64
	var salesperson string
	date := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	// Walk folder looking for costing sheets
	filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		lower := strings.ToLower(info.Name())
		if !strings.HasSuffix(lower, ".xlsx") {
			return nil
		}
		if !strings.Contains(lower, "costing") && !strings.Contains(lower, "masterfile") {
			return nil
		}

		// Parse this costing sheet
		f, err := excelize.OpenFile(path)
		if err != nil {
			return nil
		}
		defer f.Close()

		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return nil
		}

		rows, _ := f.GetRows(sheets[0])
		for _, row := range rows {
			if len(row) == 0 {
				continue
			}

			firstCell := strings.ToUpper(strings.TrimSpace(row[0]))

			// Look for salesperson
			if strings.Contains(firstCell, "COSTING MADE BY") || strings.Contains(firstCell, "PREPARED BY") {
				if len(row) > 1 {
					salesperson = strings.TrimSpace(row[1])
				}
			}

			// Look for date
			if strings.Contains(firstCell, "DATE") && len(row) > 1 {
				d := parseDate(strings.TrimSpace(row[1]))
				if !d.IsZero() {
					date = d
				}
			}

			// Look for total value
			if strings.Contains(firstCell, "TOTAL") {
				for _, cell := range row {
					val := parseFloat(cell)
					if val > 1000 && val > maxValue {
						maxValue = val
					}
				}
			}
		}

		return nil
	})

	return maxValue, salesperson, date
}

func generateCode(name string) string {
	words := strings.Fields(name)
	code := ""
	for _, word := range words {
		if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
			code += string(word[0])
		}
		if len(code) >= 5 {
			break
		}
	}
	if code == "" {
		code = "CUST"
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
	}
	for _, fmt := range formats {
		if t, err := time.Parse(fmt, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}
