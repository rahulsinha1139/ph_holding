package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// RFQData represents an opportunity/RFQ
type RFQData struct {
	ID             uint      `gorm:"primaryKey"`
	Client         string    `json:"client"`
	Project        string    `json:"project"`
	Value          float64   `json:"value"`
	Notes          string    `json:"notes"`
	Status         string    `gorm:"default:'pending'"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	DocumentHash   string    `json:"document_hash"`
	VisitLocations string    `json:"visit_locations"`
	ProductDetails string    `json:"product_details"`
	SourceDocPath  string    `json:"source_doc_path"`
}

func (RFQData) TableName() string { return "rfq_data" }

// OfferItem represents a line item in an offer
type OfferItem struct {
	ID                  string    `gorm:"primaryKey;size:36"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	OfferID             string    `gorm:"index;size:36" json:"offer_id"`
	LineNumber          int       `json:"line_number"`
	ProductID           string    `gorm:"size:36" json:"product_id"`
	ProductCode         string    `gorm:"size:100" json:"product_code"`
	Model               string    `gorm:"size:255" json:"model"`
	Description         string    `gorm:"type:varchar(2000)" json:"description"`
	Quantity            float64   `json:"quantity"`
	UnitPrice           float64   `json:"unit_price_bhd"`
	Equipment           string    `gorm:"size:255" json:"equipment"`
	Specification       string    `gorm:"type:varchar(2000)" json:"specification"`
	DetailedDescription string    `gorm:"type:varchar(5000)" json:"detailed_description"`
	Currency            string    `gorm:"size:10" json:"currency"`
	FOB                 float64   `json:"fob"`
	Freight             float64   `json:"freight"`
	TotalCost           float64   `json:"total_cost"`
	MarginPercent       float64   `json:"margin_percent"`
	TotalPrice          float64   `json:"total_price"`
}

func (OfferItem) TableName() string { return "offer_items" }

// Offer represents a quotation
type Offer struct {
	ID              string    `gorm:"primaryKey;size:36"`
	OfferNumber     string    `gorm:"uniqueIndex;size:50" json:"offer_number"`
	RFQID           string    `gorm:"size:36" json:"rfq_id"`
	CustomerID      string    `gorm:"size:36" json:"customer_id"`
	CustomerName    string    `gorm:"size:255" json:"customer_name"`
	QuotationDate   time.Time `json:"quotation_date"`
	TotalValueBHD   float64   `json:"total_value_bhd"`
	EstimatedMargin float64   `json:"estimated_margin"`
	Stage           string    `gorm:"size:50" json:"stage"`
	CreatedBy       string    `json:"created_by"`
}

func (Offer) TableName() string { return "offers" }

// Customer for lookups
type Customer struct {
	ID           string `gorm:"primaryKey;size:36"`
	BusinessName string `gorm:"size:255" json:"business_name"`
}

func (Customer) TableName() string { return "customers" }

func main() {
	costingDir := "Costing_Sheets"
	dbPath := "ph_holdings.db"

	if len(os.Args) > 1 {
		costingDir = os.Args[1]
	}
	if len(os.Args) > 2 {
		dbPath = os.Args[2]
	}

	fmt.Println("📊 Costing Sheets Import Tool")
	fmt.Println("==============================")
	fmt.Printf("Costing directory: %s\n", costingDir)
	fmt.Printf("Database: %s\n\n", dbPath)

	// Open database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Find all Excel files
	files, err := filepath.Glob(filepath.Join(costingDir, "*.xlsx"))
	if err != nil {
		log.Fatalf("Failed to find Excel files: %v", err)
	}
	fmt.Printf("Found %d Excel files\n\n", len(files))

	rfqsCreated := 0
	offerItemsCreated := 0
	offersUpdated := 0
	errors := 0

	for _, file := range files {
		fmt.Printf("Processing: %s\n", filepath.Base(file))

		rfq, items, offerNum, err := parseCosting(file)
		if err != nil {
			fmt.Printf("  ⚠️ Error: %v\n", err)
			errors++
			continue
		}

		if rfq != nil {
			// Check if RFQ already exists
			var existing RFQData
			result := db.Where("client = ? AND project = ?", rfq.Client, rfq.Project).First(&existing)
			if result.Error == gorm.ErrRecordNotFound {
				if err := db.Create(rfq).Error; err != nil {
					fmt.Printf("  ❌ Failed to create RFQ: %v\n", err)
					errors++
				} else {
					fmt.Printf("  ✅ Created RFQ: %s - %s (%.2f BHD)\n", rfq.Client, rfq.Project, rfq.Value)
					rfqsCreated++
				}
			} else {
				fmt.Printf("  ⏭️ RFQ exists: %s - %s\n", rfq.Client, rfq.Project)
			}
		}

		if len(items) > 0 && offerNum != "" {
			// Find or create offer
			var offer Offer
			result := db.Where("offer_number = ?", offerNum).First(&offer)
			if result.Error == gorm.ErrRecordNotFound {
				// Try to find by similar pattern
				result = db.Where("offer_number LIKE ? ESCAPE '\\\\'", "%"+escapeLikeWildcards(strings.Split(offerNum, "-")[0])+"%").First(&offer)
			}

			if result.Error == nil {
				// Update offer with items
				for i, item := range items {
					item.OfferID = offer.ID
					item.LineNumber = i + 1
					item.ID = uuid.New().String()
					item.CreatedAt = time.Now()
					item.UpdatedAt = time.Now()

					// Check if item already exists
					var existing OfferItem
					if db.Where("offer_id = ? AND line_number = ?", offer.ID, item.LineNumber).First(&existing).Error == gorm.ErrRecordNotFound {
						if err := db.Create(&item).Error; err != nil {
							fmt.Printf("  ❌ Failed to create item: %v\n", err)
							errors++
						} else {
							offerItemsCreated++
						}
					}
				}
				fmt.Printf("  ✅ Added %d items to offer %s\n", len(items), offerNum)
				offersUpdated++
			} else {
				fmt.Printf("  ⚠️ No matching offer found for: %s\n", offerNum)
			}
		}
	}

	fmt.Println("\n==============================")
	fmt.Printf("📊 Import Summary\n")
	fmt.Printf("==============================\n")
	fmt.Printf("RFQs created:       %d\n", rfqsCreated)
	fmt.Printf("Offer items created: %d\n", offerItemsCreated)
	fmt.Printf("Offers updated:     %d\n", offersUpdated)
	fmt.Printf("Errors:             %d\n", errors)
	fmt.Println("==============================")
}

func parseCosting(filePath string) (*RFQData, []OfferItem, string, error) {
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to open: %v", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	var costingSheet string
	for _, s := range sheets {
		if strings.Contains(strings.ToLower(s), "costing") {
			costingSheet = s
			break
		}
	}
	if costingSheet == "" {
		// Use first sheet if no costing sheet found
		if len(sheets) > 0 {
			costingSheet = sheets[0]
		} else {
			return nil, nil, "", fmt.Errorf("no sheets found")
		}
	}

	rows, err := f.GetRows(costingSheet)
	if err != nil {
		return nil, nil, "", fmt.Errorf("failed to read sheet: %v", err)
	}

	var customer, project, preparedBy, folderNum string
	var totalValue float64
	var items []OfferItem

	// Parse rows looking for key fields
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}

		label := strings.TrimSpace(strings.ToLower(row[0]))

		// Extract customer
		if strings.Contains(label, "customer") && len(row) > 1 {
			customer = cleanValue(row[1])
		}

		// Extract folder number / project
		if strings.Contains(label, "date") || strings.Contains(label, "folder") {
			for _, cell := range row[1:] {
				if strings.Contains(strings.ToLower(cell), "folder") {
					re := regexp.MustCompile(`(\d+)\s*-?\s*(\d{4}|\d{2})`)
					if matches := re.FindStringSubmatch(cell); len(matches) > 0 {
						folderNum = matches[0]
						project = fmt.Sprintf("Folder %s", folderNum)
					}
				}
			}
		}

		// Extract prepared by
		if strings.Contains(label, "prepared") && len(row) > 1 {
			preparedBy = cleanValue(row[1])
		}

		// Extract costing ID as project
		if strings.Contains(label, "costing") && project == "" {
			for _, cell := range row {
				if strings.Contains(strings.ToUpper(cell), "/") {
					project = cleanValue(cell)
					break
				}
			}
		}

		// Look for equipment/model rows to extract items
		if strings.Contains(label, "equipment") && len(row) > 1 {
			// Parse items from columns
			for j := 1; j < len(row); j++ {
				equip := cleanValue(row[j])
				if equip != "" && !strings.Contains(strings.ToLower(equip), "[product") {
					item := OfferItem{
						Equipment: equip,
					}

					// Try to get model from next row
					if i+1 < len(rows) && len(rows[i+1]) > j {
						item.Model = cleanValue(rows[i+1][j])
					}
					// Try to get spec from row after that
					if i+2 < len(rows) && len(rows[i+2]) > j {
						item.Specification = cleanValue(rows[i+2][j])
					}

					items = append(items, item)
				}
			}
		}

		// Extract total value
		if strings.Contains(label, "total") && strings.Contains(label, "po") {
			for _, cell := range row[1:] {
				if v := parseFloat(cell); v > 0 {
					totalValue = v
					break
				}
			}
		}

		// Extract margin and costs for items
		if strings.Contains(label, "total cost") || strings.Contains(label, "totalcost") {
			for j := 1; j < len(row) && j-1 < len(items); j++ {
				if v := parseFloat(row[j]); v > 0 {
					items[j-1].TotalCost = v
				}
			}
		}
		if strings.Contains(label, "markup") || strings.Contains(label, "mark up") {
			for j := 1; j < len(row) && j-1 < len(items); j++ {
				if v := parseFloat(row[j]); v > 0 {
					items[j-1].MarginPercent = v
				}
			}
		}
		if strings.Contains(label, "selling") && strings.Contains(label, "price") {
			for j := 1; j < len(row) && j-1 < len(items); j++ {
				if v := parseFloat(row[j]); v > 0 {
					items[j-1].UnitPrice = v
					items[j-1].TotalPrice = v
				}
			}
		}
		if strings.Contains(label, "fob") && !strings.Contains(label, "bhd") {
			for j := 1; j < len(row) && j-1 < len(items); j++ {
				if v := parseFloat(row[j]); v > 0 {
					items[j-1].FOB = v
				}
			}
		}
		if strings.Contains(label, "freight") {
			for j := 1; j < len(row) && j-1 < len(items); j++ {
				if v := parseFloat(row[j]); v > 0 {
					items[j-1].Freight = v
				}
			}
		}
		if strings.Contains(label, "quantity") || strings.Contains(label, "qty") {
			for j := 1; j < len(row) && j-1 < len(items); j++ {
				if v := parseFloat(row[j]); v > 0 {
					items[j-1].Quantity = v
				}
			}
		}
	}

	// Create RFQ if we have customer and project
	var rfq *RFQData
	if customer != "" {
		if project == "" {
			project = filepath.Base(filePath)
		}
		rfq = &RFQData{
			Client:        customer,
			Project:       project,
			Value:         totalValue,
			Notes:         fmt.Sprintf("Imported from costing sheet. Prepared by: %s", preparedBy),
			Status:        "quoted",
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
			SourceDocPath: filePath,
		}
	}

	// Generate offer number from folder number
	offerNum := ""
	if folderNum != "" {
		offerNum = folderNum
	} else {
		// Try to extract from filename
		re := regexp.MustCompile(`^(\d+)`)
		if matches := re.FindStringSubmatch(filepath.Base(filePath)); len(matches) > 0 {
			offerNum = matches[1] + "-25"
		}
	}

	return rfq, items, offerNum, nil
}

func cleanValue(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, " \t\n\r")
	return s
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, " ", "")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func escapeLikeWildcards(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "_", "\\_")
	return s
}
