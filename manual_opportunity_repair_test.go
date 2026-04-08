package main

import (
	"os"
	"path/filepath"
	"testing"

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
		Where("source = ? AND (year < 2024 OR year > 2026 OR opp_number = 0)", "2026_onedrive").
		Count(&malformedCount).Error; err != nil {
		t.Fatalf("failed to verify repaired records: %v", err)
	}

	t.Logf("repair_summary malformed_remaining=%d", malformedCount)
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
