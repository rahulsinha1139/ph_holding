package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// CostingSheetData matches existing schema
type CostingSheetData struct {
	ID           uint   `gorm:"primaryKey;column:id"`
	OfferNumber  string `gorm:"column:offer_number"`
	CustomerName string `gorm:"column:customer_name"`
}

func (CostingSheetData) TableName() string { return "costing_sheet_data" }

// CostingLineItem matches existing schema
type CostingLineItem struct {
	ID              string  `gorm:"column:id"`
	CostingSheetID  int     `gorm:"column:costing_sheet_id"`
	ProductNumber   int     `gorm:"column:product_number"`
	Equipment       string  `gorm:"column:equipment"`
	Model           string  `gorm:"column:model"`
	Specification   string  `gorm:"column:specification"`
	Supplier        string  `gorm:"column:supplier"`
	Quantity        float64 `gorm:"column:quantity"`
	FobEUR          float64 `gorm:"column:fob_eur"`
	ExchangeRate    float64 `gorm:"column:exchange_rate"`
	TotalCostBHD    float64 `gorm:"column:total_cost_bhd"`
	MarkupPercent   float64 `gorm:"column:markup_percent"`
	SellingPriceBHD float64 `gorm:"column:selling_price_bhd"`
}

func (CostingLineItem) TableName() string { return "costing_line_items" }

// Offer matches existing schema
type Offer struct {
	ID           string `gorm:"primaryKey;column:id"`
	OfferNumber  string `gorm:"column:offer_number"`
	CustomerName string `gorm:"column:customer_name"`
}

func (Offer) TableName() string { return "offers" }

// OfferItem matches existing schema
type OfferItem struct {
	ID                  string    `gorm:"primaryKey;column:id"`
	CreatedAt           time.Time `gorm:"column:created_at"`
	UpdatedAt           time.Time `gorm:"column:updated_at"`
	Version             int       `gorm:"column:version;default:1"`
	OfferID             string    `gorm:"column:offer_id"`
	LineNumber          int       `gorm:"column:line_number"`
	Description         string    `gorm:"column:description"`
	Quantity            float64   `gorm:"column:quantity"`
	UnitPrice           float64   `gorm:"column:unit_price"`
	Model               string    `gorm:"column:model"`
	Equipment           string    `gorm:"column:equipment"`
	Specification       string    `gorm:"column:specification"`
	Currency            string    `gorm:"column:currency"`
	FOB                 float64   `gorm:"column:fob"`
	TotalCost           float64   `gorm:"column:total_cost"`
	MarginPercent       float64   `gorm:"column:margin_percent"`
	TotalPrice          float64   `gorm:"column:total_price"`
}

func (OfferItem) TableName() string { return "offer_items" }

func main() {
	db, err := gorm.Open(sqlite.Open("ph_holdings.db"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// Get initial counts
	var initialOfferItems int64
	db.Table("offer_items").Count(&initialOfferItems)
	fmt.Printf("Initial offer_items count: %d\n\n", initialOfferItems)

	// Get offers without items (or with no items)
	var offersWithoutItems []Offer
	db.Raw(`
		SELECT o.id, o.offer_number, o.customer_name
		FROM offers o
		WHERE o.deleted_at IS NULL
		AND NOT EXISTS (
			SELECT 1 FROM offer_items oi
			WHERE oi.offer_id = o.id AND oi.deleted_at IS NULL
		)
	`).Scan(&offersWithoutItems)

	fmt.Printf("Found %d offers without items\n\n", len(offersWithoutItems))

	totalLinked := 0
	totalItems := 0

	for _, offer := range offersWithoutItems {
		// Strategy 1: Try exact offer number match
		offerNum := extractOfferNumber(offer.OfferNumber)
		costingSheetID := findCostingSheetByOfferNumber(db, offerNum)

		// Strategy 2: If no match, try customer name match
		if costingSheetID == 0 {
			costingSheetID = findCostingSheetByCustomer(db, offer.CustomerName)
		}

		if costingSheetID == 0 {
			fmt.Printf("  [SKIP] %s (%s): No matching costing data found\n",
				offer.OfferNumber, offer.CustomerName)
			continue
		}

		// Get costing line items for this sheet
		var costingItems []CostingLineItem
		db.Where("costing_sheet_id = ?", costingSheetID).Find(&costingItems)

		if len(costingItems) == 0 {
			fmt.Printf("  [SKIP] %s: Costing sheet %d has no line items\n",
				offer.OfferNumber, costingSheetID)
			continue
		}

		// Create offer items from costing items
		itemsCreated := createOfferItems(db, offer.ID, costingItems)
		if itemsCreated > 0 {
			fmt.Printf("  [LINK] %s (%s): Created %d items from costing sheet %d\n",
				offer.OfferNumber, offer.CustomerName, itemsCreated, costingSheetID)
			totalLinked++
			totalItems += itemsCreated
		}
	}

	// Final counts
	var finalOfferItems int64
	db.Table("offer_items").Count(&finalOfferItems)

	fmt.Printf("\n========================================\n")
	fmt.Printf("SUMMARY\n")
	fmt.Printf("========================================\n")
	fmt.Printf("Offers linked: %d\n", totalLinked)
	fmt.Printf("Items created: %d\n", totalItems)
	fmt.Printf("offer_items: %d -> %d\n", initialOfferItems, finalOfferItems)
	fmt.Printf("========================================\n")

	// Verification query
	fmt.Println("\nVerification - Offers with item counts:")
	rows, err := db.Raw(`
		SELECT o.offer_number, o.customer_name, COUNT(oi.id) as item_count
		FROM offers o
		LEFT JOIN offer_items oi ON oi.offer_id = o.id AND oi.deleted_at IS NULL
		WHERE o.deleted_at IS NULL
		GROUP BY o.id
		ORDER BY item_count DESC
	`).Rows()
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var offerNum, custName string
			var itemCount int
			rows.Scan(&offerNum, &custName, &itemCount)
			fmt.Printf("  %s | %s | %d items\n", offerNum, custName, itemCount)
		}
	}
}

// extractOfferNumber extracts numeric part: "181-25" -> "181", "02-25" -> "2"
func extractOfferNumber(offerNumber string) string {
	parts := strings.Split(offerNumber, "-")
	if len(parts) > 0 {
		num := strings.TrimLeft(parts[0], "0")
		if num == "" {
			num = "0"
		}
		return num
	}
	return offerNumber
}

// findCostingSheetByOfferNumber finds costing sheet by exact offer number
func findCostingSheetByOfferNumber(db *gorm.DB, offerNum string) int {
	var result struct {
		ID int
	}
	err := db.Raw(`
		SELECT id FROM costing_sheet_data
		WHERE offer_number = ?
		ORDER BY id DESC
		LIMIT 1
	`, offerNum).Scan(&result).Error

	if err != nil || result.ID == 0 {
		// Also try as integer comparison
		if num, err := strconv.Atoi(offerNum); err == nil {
			db.Raw(`
				SELECT id FROM costing_sheet_data
				WHERE CAST(offer_number AS INTEGER) = ?
				ORDER BY id DESC
				LIMIT 1
			`, num).Scan(&result)
		}
	}
	return result.ID
}

// findCostingSheetByCustomer finds the best matching costing sheet for a customer
func findCostingSheetByCustomer(db *gorm.DB, customerName string) int {
	var result struct {
		ID int
	}

	// Get key words from customer name for fuzzy matching
	keywords := extractKeywords(customerName)
	if len(keywords) == 0 {
		return 0
	}

	// Try each keyword to find a match, prioritizing sheets with most items
	for _, keyword := range keywords {
		if len(keyword) < 4 {
			continue
		}
		db.Raw(`
			SELECT csd.id
			FROM costing_sheet_data csd
			WHERE csd.id IN (SELECT DISTINCT costing_sheet_id FROM costing_line_items)
			AND UPPER(csd.customer_name) LIKE '%' || ? || '%'
			ORDER BY (SELECT COUNT(*) FROM costing_line_items WHERE costing_sheet_id = csd.id) DESC
			LIMIT 1
		`, strings.ToUpper(keyword)).Scan(&result)

		if result.ID > 0 {
			return result.ID
		}
	}

	return 0
}

// extractKeywords extracts significant words from customer name for matching
func extractKeywords(name string) []string {
	name = strings.ToUpper(name)

	// Remove common suffixes
	suffixes := []string{
		"W.L.L", "W.L.L.", "B.S.C", "B.S.C.", "S.P.C", "S.P.C.",
		"CO.", "COMPANY", "LLC", "LTD", "LTD.", "INC", "INC.",
		"(C)", "(CLOSED)", "BAHRAIN", "MIDDLE EAST",
	}
	for _, suffix := range suffixes {
		name = strings.Replace(name, suffix, " ", -1)
	}

	// Split into words
	words := strings.Fields(name)
	var keywords []string

	// Add full phrases and individual words
	for _, word := range words {
		word = strings.Trim(word, " &-.,()[]")
		if len(word) >= 4 {
			keywords = append(keywords, word)
		}
	}

	// Add first two words combined if available
	if len(words) >= 2 {
		combined := words[0] + " " + words[1]
		keywords = append([]string{combined}, keywords...)
	}

	return keywords
}


// createOfferItems creates offer items from costing line items
func createOfferItems(db *gorm.DB, offerID string, costingItems []CostingLineItem) int {
	now := time.Now()
	created := 0

	for i, ci := range costingItems {
		qty := ci.Quantity
		if qty <= 0 {
			qty = 1
		}

		unitPrice := ci.SellingPriceBHD
		if unitPrice <= 0 {
			unitPrice = ci.TotalCostBHD * 1.25 // Default 25% margin if no selling price
		}

		item := OfferItem{
			ID:            uuid.New().String(),
			CreatedAt:     now,
			UpdatedAt:     now,
			Version:       1,
			OfferID:       offerID,
			LineNumber:    i + 1,
			Description:   strings.TrimSpace(ci.Equipment),
			Quantity:      qty,
			UnitPrice:     unitPrice,
			Model:         strings.TrimSpace(ci.Model),
			Equipment:     strings.TrimSpace(ci.Equipment),
			Specification: strings.TrimSpace(ci.Specification),
			Currency:      "BHD",
			FOB:           ci.FobEUR,
			TotalCost:     ci.TotalCostBHD,
			MarginPercent: ci.MarkupPercent,
			TotalPrice:    unitPrice * qty,
		}

		if err := db.Create(&item).Error; err != nil {
			log.Printf("Error creating item for offer %s: %v", offerID, err)
			continue
		}
		created++
	}

	return created
}
