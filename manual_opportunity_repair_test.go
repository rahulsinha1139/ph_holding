package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestManualRepairImportedOpportunityMetadata(t *testing.T) {
	dbPath := os.Getenv("OPPORTUNITY_REPAIR_DB")
	if dbPath == "" {
		dbPath = filepath.Join(".", "ph_holdings.db")
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open database %s: %v", dbPath, err)
	}

	if err := repairImportedOpportunityMetadata(db); err != nil {
		t.Fatalf("repair failed: %v", err)
	}

	var malformedCount int64
	if err := db.Model(&Opportunity{}).
		Where("(source = ? OR source LIKE ?) AND (year < 2024 OR year > 2026 OR opp_number = 0)", "onedrive_import", "%_onedrive").
		Count(&malformedCount).Error; err != nil {
		t.Fatalf("failed to verify repaired records: %v", err)
	}

	t.Logf("repair_summary malformed_remaining=%d", malformedCount)
}

func TestRepairImportedOpportunityMetadataFixesSeedYearParse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Opportunity{}))

	rows := []Opportunity{
		{
			Base:         Base{ID: "opp-year-blank"},
			FolderNumber: "2025-02",
			CustomerName: "Tabreed",
			Stage:        "Quoted",
			Year:         0,
		},
		{
			Base:         Base{ID: "opp-year-2025-excel"},
			Source:       "2025_excel",
			FolderNumber: "262 C-25",
			Stage:        "Won",
			Year:         0,
		},
		{
			Base:         Base{ID: "opp-year-sa"},
			Source:       "2026_onedrive",
			FolderNumber: "SA-05-SULB",
			FolderName:   "SA-05-SULB AIT",
			Stage:        "Quoted",
			Year:         2005,
		},
		{
			Base:         Base{ID: "opp-year-source-only"},
			Source:       "2026_onedrive",
			FolderNumber: "NOMAC",
			FolderName:   "NOMAC",
			Stage:        "Qualified",
			Year:         0,
		},
	}
	require.NoError(t, db.Create(&rows).Error)
	require.NoError(t, repairImportedOpportunityMetadata(db))

	expected := map[string]int{
		"opp-year-blank":       2025,
		"opp-year-2025-excel":  2025,
		"opp-year-sa":          2026,
		"opp-year-source-only": 2026,
	}
	for id, wantYear := range expected {
		var got Opportunity
		require.NoError(t, db.First(&got, "id = ?", id).Error)
		require.Equal(t, wantYear, got.Year, id)
	}
}

func TestManualRepairCommercialPlaceholders(t *testing.T) {
	if os.Getenv("COMMERCIAL_PLACEHOLDER_REPAIR_COMMIT") != "1" {
		t.Skip("set COMMERCIAL_PLACEHOLDER_REPAIR_COMMIT=1 to clean imported Line Item placeholders")
	}

	dbPaths := []string{
		filepath.Join(".", "ph_holdings.db"),
		filepath.Join(os.Getenv("HOME"), ".local", "share", "AsymmFlow", "ph_holdings.db"),
	}
	if explicit := os.Getenv("OPPORTUNITY_REPAIR_DB"); explicit != "" {
		dbPaths = []string{explicit}
	}

	for _, dbPath := range dbPaths {
		db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("failed to open database %s: %v", dbPath, err)
		}

		if err := repairImportedCommercialDocuments(db); err != nil {
			t.Fatalf("commercial placeholder repair failed for %s: %v", dbPath, err)
		}

		var opportunityPlaceholders int64
		if err := db.Model(&Opportunity{}).
			Where("deleted_at IS NULL AND LOWER(COALESCE(product_details, '')) LIKE ?", "%line item%").
			Count(&opportunityPlaceholders).Error; err != nil {
			t.Fatalf("failed to count opportunity placeholders for %s: %v", dbPath, err)
		}

		var offerPlaceholders int64
		if err := db.Raw(`
			SELECT COUNT(DISTINCT o.id)
			FROM offers o
			JOIN offer_items oi ON oi.offer_id = o.id AND oi.deleted_at IS NULL
			WHERE o.deleted_at IS NULL
			  AND (LOWER(COALESCE(oi.description, '')) LIKE 'line item%'
			    OR LOWER(COALESCE(oi.product_code, '')) LIKE 'line item%'
			    OR LOWER(COALESCE(oi.model, '')) LIKE 'line item%'
			    OR LOWER(COALESCE(oi.equipment, '')) LIKE 'line item%')
		`).Scan(&offerPlaceholders).Error; err != nil {
			t.Fatalf("failed to count offer placeholders for %s: %v", dbPath, err)
		}

		var orderPlaceholders int64
		if err := db.Raw(`
			SELECT COUNT(DISTINCT o.id)
			FROM orders o
			JOIN order_items oi ON oi.order_id = o.id AND oi.deleted_at IS NULL
			WHERE o.deleted_at IS NULL
			  AND (LOWER(COALESCE(oi.description, '')) LIKE 'line item%'
			    OR LOWER(COALESCE(oi.product_code, '')) LIKE 'line item%'
			    OR LOWER(COALESCE(oi.model, '')) LIKE 'line item%'
			    OR LOWER(COALESCE(oi.equipment, '')) LIKE 'line item%')
		`).Scan(&orderPlaceholders).Error; err != nil {
			t.Fatalf("failed to count order placeholders for %s: %v", dbPath, err)
		}

		t.Logf("%s placeholders_remaining opportunities=%d offers=%d orders=%d", dbPath, opportunityPlaceholders, offerPlaceholders, orderPlaceholders)
	}
}

func TestManualParseCostingFile(t *testing.T) {
	filePath := os.Getenv("COSTING_PARSE_FILE")
	if filePath == "" {
		t.Skip("set COSTING_PARSE_FILE to inspect one costing workbook")
	}

	data, err := ParseCostingSheet(filePath)
	if err != nil {
		t.Fatalf("failed to parse costing file %s: %v", filePath, err)
	}

	t.Logf("costing_file=%s customer=%q reference=%q folder=%q items=%d subtotal=%.3f vat=%.3f grand_total=%.3f",
		filePath,
		data.Metadata.Customer,
		data.Metadata.Reference,
		data.Metadata.FolderNumber,
		len(data.LineItems),
		data.Totals.Subtotal,
		data.Totals.VatAmount,
		data.Totals.GrandTotal,
	)
	for _, item := range data.LineItems {
		t.Logf("item[%d] supplier=%q equipment=%q model=%q qty=%.3f suggested=%.3f total=%.3f",
			item.ProductNumber,
			item.Supplier,
			item.Equipment,
			item.Model,
			item.Quantity,
			item.SuggestedPriceBHD,
			item.TotalSuggestedBHD,
		)
	}
}

func TestManualExtractAnnexureSpecs(t *testing.T) {
	filePath := os.Getenv("ANNEXURE_PARSE_FILE")
	if filePath == "" {
		t.Skip("set ANNEXURE_PARSE_FILE to inspect one techno-commercial annexure source")
	}

	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("failed to stat annexure file: %v", err)
	}
	file := DiscoveredFile{
		FileName:  filepath.Base(filePath),
		FilePath:  filePath,
		FileType:  classifyDiscoveredFile(filepath.Base(filePath)),
		Extension: filepath.Ext(filePath),
		SizeBytes: info.Size(),
		ModTime:   info.ModTime(),
	}

	text, err := extractAnnexureSourceText(file)
	if err != nil {
		t.Fatalf("failed to extract annexure text: %v", err)
	}
	specs := parseAnnexureSpecBlocks(text, filePath)
	if len(specs) == 0 {
		t.Fatalf("no annexure specs parsed from %s", filePath)
	}
	for _, spec := range specs {
		t.Logf("line=%d qty=%.3f equipment=%q model=%q long_code=%q spec=%q details=%q",
			spec.LineNumber,
			spec.Quantity,
			spec.Equipment,
			spec.Model,
			spec.LongCode,
			spec.Specification,
			spec.DetailedDescription,
		)
	}
}

func TestManualBackfillOfferAnnexures(t *testing.T) {
	if os.Getenv("OFFER_ANNEXURE_BACKFILL_COMMIT") != "1" {
		t.Skip("set OFFER_ANNEXURE_BACKFILL_COMMIT=1 to backfill offer annexure details from source folders")
	}

	root := os.Getenv("OFFERS_2026_ROOT")
	if root == "" {
		root = filepath.Join(os.Getenv("HOME"), "Downloads", "Offers 2026", "1-50")
	}

	dbPaths := []string{
		filepath.Join(".", "ph_holdings.db"),
		filepath.Join(os.Getenv("HOME"), ".local", "share", "AsymmFlow", "ph_holdings.db"),
	}
	if explicit := os.Getenv("OPPORTUNITY_REPAIR_DB"); explicit != "" {
		dbPaths = []string{explicit}
	}

	for _, dbPath := range dbPaths {
		db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Silent),
		})
		if err != nil {
			t.Fatalf("failed to open database %s: %v", dbPath, err)
		}

		summary, err := backfillOfferAnnexureDetailsFromFolder(db, root)
		if err != nil {
			t.Fatalf("annexure backfill failed for %s: %v", dbPath, err)
		}
		t.Logf("%s annexure_backfill deals=%d offers_seen=%d offers_updated=%d items_updated=%d specs_found=%d",
			dbPath,
			summary.DealsScanned,
			summary.OffersSeen,
			summary.OffersUpdated,
			summary.ItemsUpdated,
			summary.SpecsFound,
		)
	}
}

func TestManualPipelineOpportunitySummary(t *testing.T) {
	dbPath := os.Getenv("OPPORTUNITY_REPAIR_DB")
	if dbPath == "" {
		dbPath = filepath.Join(".", "ph_holdings.db")
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open database %s: %v", dbPath, err)
	}

	var raw []Opportunity
	if err := db.Where("(offer_id IS NULL OR offer_id = '') AND stage NOT IN ('Won', 'Lost')").Find(&raw).Error; err != nil {
		t.Fatalf("failed to load open opportunities: %v", err)
	}

	deduped := dedupePipelineOpportunities(raw)
	yearCounts := map[int]int{}
	for _, opp := range deduped {
		yearCounts[opp.Year]++
	}

	t.Logf("pipeline_summary raw=%d deduped=%d y2026=%d y2025=%d y2024=%d", len(raw), len(deduped), yearCounts[2026], yearCounts[2025], yearCounts[2024])
}
