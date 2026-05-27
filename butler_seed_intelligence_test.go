package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupSeedIntelligenceContextTestApp(t *testing.T) *App {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "seed-intel.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&IntelligenceOpportunityEnrichment{},
		&IntelligenceLineItemEnrichment{},
		&IntelligenceOrderEnrichment{},
	))
	return &App{db: db, cache: NewCache(), appStartTime: time.Now()}
}

func TestSeedIntelligenceContextExposesSeededSidecars(t *testing.T) {
	app := setupSeedIntelligenceContextTestApp(t)
	now := time.Now()
	require.NoError(t, app.db.Create(&IntelligenceOpportunityEnrichment{
		Base:                    Base{ID: "opp-enrich:seed-1", CreatedAt: now, UpdatedAt: now},
		OpportunityID:           "opp-1",
		Year:                    2026,
		Source:                  "2026_onedrive",
		FolderNumber:            "EH-01-26",
		FolderName:              "BAPCO flow meter",
		CustomerName:            "BAPCO",
		Stage:                   "Quoted",
		RevenueBHD:              1250,
		OfferNumber:             "EH-01-26",
		PreparedBy:              "Rahul",
		PreparedByQuality:       "source",
		PrimaryInstrumentClass:  "flow",
		PrimaryInstrumentSignal: "FIT",
		CanonicalProductType:    "Flow (FIT)",
		LineItemCount:           2,
		LineTotalBHD:            1100,
		HasLinkedOffer:          true,
		HasProductDetails:       true,
		EnrichedAt:              "2026-05-21T15:33:08+05:30",
	}).Error)
	require.NoError(t, app.db.Create(&IntelligenceLineItemEnrichment{
		Base:             Base{ID: "line-enrich:seed-1:1", CreatedAt: now, UpdatedAt: now},
		EntityKind:       "opportunity",
		EntityID:         "opp-1",
		OpportunityID:    "opp-1",
		FolderNumber:     "EH-01-26",
		CustomerName:     "BAPCO",
		DocumentNumber:   "EH-01-26",
		Description:      "Flow transmitter",
		Quantity:         2,
		UnitPriceBHD:     550,
		TotalPriceBHD:    1100,
		InstrumentClass:  "flow",
		InstrumentSignal: "FIT",
		EnrichedAt:       "2026-05-21T15:33:08+05:30",
	}).Error)
	require.NoError(t, app.db.Create(&IntelligenceOrderEnrichment{
		Base:                    Base{ID: "order-enrich:seed-1", CreatedAt: now, UpdatedAt: now},
		OrderID:                 "order-1",
		Year:                    2026,
		OrderNumber:             "EH-01-26",
		CustomerName:            "BAPCO",
		Status:                  "Confirmed",
		GrandTotalBHD:           1250,
		PrimaryInstrumentClass:  "flow",
		PrimaryInstrumentSignal: "FIT",
		CanonicalProductType:    "Flow (FIT)",
		LineItemCount:           1,
		LineTotalBHD:            1100,
		EnrichedAt:              "2026-05-21T15:33:08+05:30",
	}).Error)

	ctx := app.getSeedIntelligenceContext(Intent{RawQuery: "show bapco flow 2026"}, true)
	counts := ctx["table_counts"].(map[string]int64)
	require.Equal(t, int64(1), counts["intelligence_opportunity_enrichment"])
	require.Equal(t, int64(1), counts["intelligence_line_item_enrichment"])
	require.Equal(t, int64(1), counts["intelligence_order_enrichment"])

	matches := ctx["matching_enriched_opportunities"].([]map[string]interface{})
	require.Len(t, matches, 1)
	require.Equal(t, "BAPCO", matches[0]["customer_name"])
	require.Equal(t, 1250.0, matches[0]["revenue_bhd"])

	redacted := app.getSeedIntelligenceContext(Intent{RawQuery: "show bapco flow 2026"}, false)
	redactedMatches := redacted["matching_enriched_opportunities"].([]map[string]interface{})
	require.NotContains(t, redactedMatches[0], "revenue_bhd")
	products := redacted["product_mix_summary"].([]map[string]interface{})
	require.NotContains(t, products[0], "total_price_bhd")
}
