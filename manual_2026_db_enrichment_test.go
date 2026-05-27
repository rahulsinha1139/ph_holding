package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type db2026EnrichmentSummary struct {
	GeneratedAt                  time.Time `json:"generated_at"`
	DatabasePath                 string    `json:"database_path"`
	BackupPath                   string    `json:"backup_path"`
	OutputDir                    string    `json:"output_dir"`
	Committed                    bool      `json:"committed"`
	OpportunitiesScanned         int       `json:"opportunities_scanned"`
	OpportunityRowsEnriched      int       `json:"opportunity_rows_enriched"`
	OpportunityLineRowsEnriched  int       `json:"opportunity_line_rows_enriched"`
	OrdersScanned                int       `json:"orders_scanned"`
	OrderRowsEnriched            int       `json:"order_rows_enriched"`
	OrderLineRowsEnriched        int       `json:"order_line_rows_enriched"`
	ProductTypeBackfilled        int       `json:"product_type_backfilled"`
	MissingPreparedBy            int       `json:"missing_prepared_by"`
	UnknownOpportunityLineClass  int       `json:"unknown_opportunity_line_class"`
	UnknownOrderLineClass        int       `json:"unknown_order_line_class"`
	OpportunitiesWithoutLineData int       `json:"opportunities_without_line_data"`
}

type db2026OpportunityRow struct {
	ID              string
	FolderNumber    string
	FolderName      string
	CustomerID      string
	CustomerName    string
	Source          string
	Stage           string
	RevenueBHD      float64
	ProductType     string
	ProductDetails  string
	OfferID         string
	OfferNumber     string
	QuoteType       string
	RevisionNumber  int
	IssuedBy        string
	QuotationDate   string
	HasProductJSON  bool
	HasLinkedOffer  bool
	CreatedBy       string
	OpportunityDate string
}

type db2026LineRow struct {
	EntityKind       string
	EntityID         string
	SourceLineKind   string
	SourceLineID     string
	OpportunityID    string
	OfferID          string
	OrderID          string
	LineNumber       int
	FolderNumber     string
	FolderName       string
	CustomerName     string
	DocumentNumber   string
	Description      string
	Equipment        string
	Model            string
	Specification    string
	Quantity         float64
	UnitPriceBHD     float64
	TotalPriceBHD    float64
	PreparedBy       string
	InstrumentClass  string
	InstrumentSignal string
	SourceConfidence string
}

type db2026OpportunityEnrichmentRow struct {
	ID                   string
	OpportunityID        string
	Year                 int
	Source               string
	FolderNumber         string
	FolderName           string
	CustomerID           string
	CustomerName         string
	Stage                string
	RevenueBHD           float64
	OfferID              string
	OfferNumber          string
	QuoteType            string
	RevisionNumber       int
	PreparedBy           string
	PreparedByQuality    string
	PrimaryClass         string
	PrimarySignal        string
	CanonicalProductType string
	ProductMixJSON       string
	LineItemCount        int
	LineTotalBHD         float64
	LineSource           string
	HasLinkedOffer       bool
	HasProductDetails    bool
	EnrichmentNotes      string
	EnrichedAt           string
}

type db2026OrderRow struct {
	ID           string
	OrderNumber  string
	CustomerID   string
	CustomerName string
	OrderDate    string
	Status       string
	GrandTotal   float64
}

type db2026OrderEnrichmentRow struct {
	ID                   string
	OrderID              string
	Year                 int
	OrderNumber          string
	CustomerID           string
	CustomerName         string
	OrderDate            string
	Status               string
	GrandTotalBHD        float64
	PrimaryClass         string
	PrimarySignal        string
	CanonicalProductType string
	ProductMixJSON       string
	LineItemCount        int
	LineTotalBHD         float64
	EnrichedAt           string
}

type db2026MixEntry struct {
	InstrumentClass  string  `json:"instrument_class"`
	InstrumentSignal string  `json:"instrument_signal"`
	Lines            int     `json:"lines"`
	TotalBHD         float64 `json:"total_bhd"`
}

type db2026ProductDetail struct {
	Description   string  `json:"description"`
	Quantity      float64 `json:"quantity"`
	UnitPrice     float64 `json:"unit_price"`
	TotalPrice    float64 `json:"total_price"`
	PartNumber    string  `json:"part_number"`
	Specification string  `json:"specification"`
}

func TestManualEnrich2026DatabaseIntelligence(t *testing.T) {
	if os.Getenv("ENRICH_2026_DB") != "1" {
		t.Skip("set ENRICH_2026_DB=1 to run the 2026 DB intelligence enrichment pass")
	}

	dbPath := resolveOneDriveImportDBPath()
	stamp := time.Now().Format("20060102_150405")
	outputDir := strings.TrimSpace(os.Getenv("ENRICH_2026_OUTPUT_DIR"))
	if outputDir == "" {
		outputDir = filepath.Join("reports", "db_2026_enrichment_"+stamp)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("failed to create output dir: %v", err)
	}

	commit := os.Getenv("ENRICH_2026_DB_COMMIT") == "1"
	backupPath := ""
	if commit {
		backupPath = filepath.Join(outputDir, "ph_holdings_pre_2026_enrichment.db")
		if err := copy2026EnrichmentFile(dbPath, backupPath); err != nil {
			t.Fatalf("failed to backup db: %v", err)
		}
	}

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}

	opportunities := load2026OpportunityRows(t, db)
	opportunityLines := load2026OpportunityLineRows(t, db, opportunities)
	opportunityEnrichments, productTypeUpdates := build2026OpportunityEnrichments(opportunities, opportunityLines)

	orders := load2026OrderRows(t, db)
	orderLines := load2026OrderLineRows(t, db, orders)
	orderEnrichments := build2026OrderEnrichments(orders, orderLines)

	summary := db2026EnrichmentSummary{
		GeneratedAt:                 time.Now(),
		DatabasePath:                dbPath,
		BackupPath:                  backupPath,
		OutputDir:                   outputDir,
		Committed:                   commit,
		OpportunitiesScanned:        len(opportunities),
		OpportunityRowsEnriched:     len(opportunityEnrichments),
		OpportunityLineRowsEnriched: len(opportunityLines),
		OrdersScanned:               len(orders),
		OrderRowsEnriched:           len(orderEnrichments),
		OrderLineRowsEnriched:       len(orderLines),
		ProductTypeBackfilled:       len(productTypeUpdates),
	}
	for _, row := range opportunityEnrichments {
		if row.PreparedByQuality != "source" {
			summary.MissingPreparedBy++
		}
		if row.LineItemCount == 0 {
			summary.OpportunitiesWithoutLineData++
		}
	}
	for _, line := range opportunityLines {
		if line.InstrumentClass == "unknown" {
			summary.UnknownOpportunityLineClass++
		}
	}
	for _, line := range orderLines {
		if line.InstrumentClass == "unknown" {
			summary.UnknownOrderLineClass++
		}
	}

	if commit {
		if err := persist2026Enrichment(db, opportunityEnrichments, opportunityLines, orderEnrichments, orderLines, productTypeUpdates); err != nil {
			t.Fatalf("failed to persist enrichment: %v", err)
		}
	}

	if err := writeSeedCSV(filepath.Join(outputDir, "opportunities_enriched.csv"), opportunityEnrichments, []string{
		"opportunity_id", "source", "folder_number", "folder_name", "customer_name", "stage", "revenue_bhd",
		"offer_number", "prepared_by", "prepared_by_quality", "primary_class", "primary_signal",
		"canonical_product_type", "line_item_count", "line_total_bhd", "line_source", "enrichment_notes",
	}, func(row db2026OpportunityEnrichmentRow) []string {
		return []string{
			row.OpportunityID,
			row.Source,
			row.FolderNumber,
			row.FolderName,
			row.CustomerName,
			row.Stage,
			floatString(row.RevenueBHD),
			row.OfferNumber,
			row.PreparedBy,
			row.PreparedByQuality,
			row.PrimaryClass,
			row.PrimarySignal,
			row.CanonicalProductType,
			intString(row.LineItemCount),
			floatString(row.LineTotalBHD),
			row.LineSource,
			row.EnrichmentNotes,
		}
	}); err != nil {
		t.Fatalf("failed to write opportunity csv: %v", err)
	}

	if err := writeSeedCSV(filepath.Join(outputDir, "line_items_enriched.csv"), opportunityLines, []string{
		"entity_kind", "entity_id", "source_line_kind", "source_line_id", "folder_number", "folder_name",
		"customer_name", "document_number", "line_number", "description", "equipment", "model", "specification",
		"quantity", "unit_price_bhd", "total_price_bhd", "prepared_by", "instrument_class", "instrument_signal",
		"source_confidence",
	}, db2026LineRecord); err != nil {
		t.Fatalf("failed to write opportunity line csv: %v", err)
	}

	if err := writeSeedCSV(filepath.Join(outputDir, "orders_enriched.csv"), orderEnrichments, []string{
		"order_id", "order_number", "customer_name", "order_date", "status", "grand_total_bhd",
		"primary_class", "primary_signal", "canonical_product_type", "line_item_count", "line_total_bhd",
	}, func(row db2026OrderEnrichmentRow) []string {
		return []string{
			row.OrderID,
			row.OrderNumber,
			row.CustomerName,
			row.OrderDate,
			row.Status,
			floatString(row.GrandTotalBHD),
			row.PrimaryClass,
			row.PrimarySignal,
			row.CanonicalProductType,
			intString(row.LineItemCount),
			floatString(row.LineTotalBHD),
		}
	}); err != nil {
		t.Fatalf("failed to write order csv: %v", err)
	}

	if err := writeSeedCSV(filepath.Join(outputDir, "order_line_items_enriched.csv"), orderLines, []string{
		"entity_kind", "entity_id", "source_line_kind", "source_line_id", "folder_number", "folder_name",
		"customer_name", "document_number", "line_number", "description", "equipment", "model", "specification",
		"quantity", "unit_price_bhd", "total_price_bhd", "prepared_by", "instrument_class", "instrument_signal",
		"source_confidence",
	}, db2026LineRecord); err != nil {
		t.Fatalf("failed to write order line csv: %v", err)
	}

	if err := writeSeedJSON(filepath.Join(outputDir, "summary.json"), summary); err != nil {
		t.Fatalf("failed to write summary: %v", err)
	}

	t.Logf("2026_enrichment output=%s commit=%t opportunities=%d lines=%d orders=%d order_lines=%d product_type_backfills=%d",
		outputDir, commit, len(opportunityEnrichments), len(opportunityLines), len(orderEnrichments), len(orderLines), len(productTypeUpdates))
}

func load2026OpportunityRows(t *testing.T, db *gorm.DB) []db2026OpportunityRow {
	t.Helper()
	var rows []db2026OpportunityRow
	err := db.Raw(`
		SELECT
			op.id,
			op.folder_number,
			op.folder_name,
			op.customer_id,
			op.customer_name,
			op.source,
			op.stage,
			op.revenue_bhd,
			op.product_type,
			COALESCE(op.product_details, '') AS product_details,
			COALESCE(op.offer_id, '') AS offer_id,
			COALESCE(off.offer_number, '') AS offer_number,
			COALESCE(off.quote_type, '') AS quote_type,
			COALESCE(off.revision_number, 0) AS revision_number,
			COALESCE(off.issued_by, '') AS issued_by,
			COALESCE(off.quotation_date, '') AS quotation_date,
			COALESCE(op.created_by, '') AS created_by,
			COALESCE(op.offer_date, '') AS opportunity_date,
			CASE WHEN COALESCE(op.product_details, '') <> '' THEN 1 ELSE 0 END AS has_product_json,
			CASE WHEN COALESCE(op.offer_id, '') <> '' THEN 1 ELSE 0 END AS has_linked_offer
		FROM opportunities op
		LEFT JOIN offers off ON off.id = op.offer_id AND off.deleted_at IS NULL
		WHERE op.year = 2026 AND op.deleted_at IS NULL
		ORDER BY op.source, op.opp_number, op.folder_number
	`).Scan(&rows).Error
	if err != nil {
		t.Fatalf("failed to load 2026 opportunities: %v", err)
	}
	return rows
}

func load2026OpportunityLineRows(t *testing.T, db *gorm.DB, opportunities []db2026OpportunityRow) []db2026LineRow {
	t.Helper()
	var offerRows []struct {
		OpportunityID string
		OfferID       string
		SourceLineID  string
		LineNumber    int
		FolderNumber  string
		FolderName    string
		CustomerName  string
		OfferNumber   string
		ProductType   string
		IssuedBy      string
		Description   string
		Equipment     string
		Model         string
		Specification string
		Quantity      float64
		UnitPrice     float64
		TotalPrice    float64
	}
	err := db.Raw(`
		SELECT
			op.id AS opportunity_id,
			op.offer_id AS offer_id,
			oi.id AS source_line_id,
			COALESCE(oi.line_number, 0) AS line_number,
			op.folder_number,
			op.folder_name,
			op.customer_name,
			COALESCE(off.offer_number, '') AS offer_number,
			COALESCE(op.product_type, '') AS product_type,
			COALESCE(off.issued_by, '') AS issued_by,
			COALESCE(oi.description, '') AS description,
			COALESCE(oi.equipment, '') AS equipment,
			COALESCE(oi.model, '') AS model,
			COALESCE(oi.specification, '') AS specification,
			COALESCE(oi.quantity, 0) AS quantity,
			COALESCE(oi.unit_price, 0) AS unit_price,
			COALESCE(oi.total_price, 0) AS total_price
		FROM opportunities op
		JOIN offers off ON off.id = op.offer_id AND off.deleted_at IS NULL
		JOIN offer_items oi ON oi.offer_id = off.id AND oi.deleted_at IS NULL
		WHERE op.year = 2026 AND op.deleted_at IS NULL
		ORDER BY op.source, op.opp_number, oi.line_number
	`).Scan(&offerRows).Error
	if err != nil {
		t.Fatalf("failed to load 2026 offer lines: %v", err)
	}

	lines := make([]db2026LineRow, 0, len(offerRows))
	hasOfferLine := map[string]bool{}
	for _, row := range offerRows {
		class, signal := classifyLegacyInstrumentLine(row.FolderName, row.ProductType, "", "", row.Equipment, firstNonEmptyCostingValue(row.Model, row.Description), row.Specification)
		lines = append(lines, db2026LineRow{
			EntityKind:       "opportunity",
			EntityID:         row.OpportunityID,
			SourceLineKind:   "offer_item",
			SourceLineID:     row.SourceLineID,
			OpportunityID:    row.OpportunityID,
			OfferID:          row.OfferID,
			LineNumber:       row.LineNumber,
			FolderNumber:     row.FolderNumber,
			FolderName:       row.FolderName,
			CustomerName:     row.CustomerName,
			DocumentNumber:   row.OfferNumber,
			Description:      row.Description,
			Equipment:        row.Equipment,
			Model:            row.Model,
			Specification:    row.Specification,
			Quantity:         row.Quantity,
			UnitPriceBHD:     row.UnitPrice,
			TotalPriceBHD:    row.TotalPrice,
			PreparedBy:       normalizePreparedByName(row.IssuedBy),
			InstrumentClass:  class,
			InstrumentSignal: signal,
			SourceConfidence: "offer_item",
		})
		hasOfferLine[row.OpportunityID] = true
	}

	for _, opp := range opportunities {
		if hasOfferLine[opp.ID] || strings.TrimSpace(opp.ProductDetails) == "" {
			continue
		}
		var details []db2026ProductDetail
		if err := json.Unmarshal([]byte(opp.ProductDetails), &details); err != nil {
			continue
		}
		for idx, detail := range details {
			description := strings.TrimSpace(detail.Description)
			class, signal := classifyLegacyInstrumentLine(opp.FolderName, opp.ProductType, "", "", description, firstNonEmptyCostingValue(detail.PartNumber, description), detail.Specification)
			lines = append(lines, db2026LineRow{
				EntityKind:       "opportunity",
				EntityID:         opp.ID,
				SourceLineKind:   "product_details_json",
				SourceLineID:     fmt.Sprintf("%s:json:%d", opp.ID, idx+1),
				OpportunityID:    opp.ID,
				OfferID:          opp.OfferID,
				LineNumber:       idx + 1,
				FolderNumber:     opp.FolderNumber,
				FolderName:       opp.FolderName,
				CustomerName:     opp.CustomerName,
				DocumentNumber:   opp.OfferNumber,
				Description:      description,
				Equipment:        description,
				Model:            detail.PartNumber,
				Specification:    detail.Specification,
				Quantity:         detail.Quantity,
				UnitPriceBHD:     detail.UnitPrice,
				TotalPriceBHD:    detail.TotalPrice,
				PreparedBy:       normalizePreparedByName(opp.IssuedBy),
				InstrumentClass:  class,
				InstrumentSignal: signal,
				SourceConfidence: "product_details_json",
			})
		}
	}

	return lines
}

func build2026OpportunityEnrichments(opportunities []db2026OpportunityRow, lines []db2026LineRow) ([]db2026OpportunityEnrichmentRow, map[string]string) {
	linesByOpp := map[string][]db2026LineRow{}
	for _, line := range lines {
		linesByOpp[line.OpportunityID] = append(linesByOpp[line.OpportunityID], line)
	}

	now := time.Now().Format(time.RFC3339)
	rows := make([]db2026OpportunityEnrichmentRow, 0, len(opportunities))
	productTypeUpdates := map[string]string{}
	for _, opp := range opportunities {
		oppLines := linesByOpp[opp.ID]
		mix, primaryClass, primarySignal, lineTotal := build2026Mix(oppLines)
		canonicalType := canonicalProductType(primaryClass, primarySignal)
		if canonicalType == "" {
			class, signal := classifyLegacyInstrumentLine(opp.FolderName, opp.ProductType, "", "", opp.FolderName, opp.ProductType, "")
			primaryClass, primarySignal = class, signal
			canonicalType = canonicalProductType(class, signal)
		}
		if strings.TrimSpace(canonicalType) == "" && strings.TrimSpace(opp.ProductType) != "" {
			canonicalType = strings.TrimSpace(opp.ProductType)
		}

		preparedBy := normalizePreparedByName(opp.IssuedBy)
		preparedQuality := "source"
		if preparedBy == "" {
			preparedBy = "Unknown (not persisted in 2026 DB)"
			preparedQuality = "missing_in_source"
		}

		lineSource := "none"
		switch {
		case len(oppLines) == 0:
			lineSource = "none"
		case allLineSourceKind(oppLines, "offer_item"):
			lineSource = "offer_items"
		case allLineSourceKind(oppLines, "product_details_json"):
			lineSource = "product_details_json"
		default:
			lineSource = "mixed"
		}

		notes := []string{}
		if len(oppLines) == 0 {
			notes = append(notes, "no_line_items_or_product_details")
		}
		if preparedQuality != "source" {
			notes = append(notes, "prepared_by_not_persisted")
		}
		if strings.TrimSpace(opp.ProductType) == "" && canonicalType != "" {
			productTypeUpdates[opp.ID] = canonicalType
			notes = append(notes, "product_type_backfill_candidate")
		}

		rows = append(rows, db2026OpportunityEnrichmentRow{
			ID:                   "opp-enrich:" + opp.ID,
			OpportunityID:        opp.ID,
			Year:                 2026,
			Source:               opp.Source,
			FolderNumber:         opp.FolderNumber,
			FolderName:           opp.FolderName,
			CustomerID:           opp.CustomerID,
			CustomerName:         opp.CustomerName,
			Stage:                opp.Stage,
			RevenueBHD:           opp.RevenueBHD,
			OfferID:              opp.OfferID,
			OfferNumber:          opp.OfferNumber,
			QuoteType:            opp.QuoteType,
			RevisionNumber:       opp.RevisionNumber,
			PreparedBy:           preparedBy,
			PreparedByQuality:    preparedQuality,
			PrimaryClass:         primaryClass,
			PrimarySignal:        primarySignal,
			CanonicalProductType: canonicalType,
			ProductMixJSON:       mustJSON(mix),
			LineItemCount:        len(oppLines),
			LineTotalBHD:         round3(lineTotal),
			LineSource:           lineSource,
			HasLinkedOffer:       opp.HasLinkedOffer,
			HasProductDetails:    opp.HasProductJSON,
			EnrichmentNotes:      strings.Join(notes, " | "),
			EnrichedAt:           now,
		})
	}
	return rows, productTypeUpdates
}

func load2026OrderRows(t *testing.T, db *gorm.DB) []db2026OrderRow {
	t.Helper()
	var rows []db2026OrderRow
	err := db.Raw(`
		SELECT id, order_number, customer_id, customer_name, COALESCE(order_date, '') AS order_date, status, grand_total_bhd AS grand_total
		FROM orders
		WHERE deleted_at IS NULL AND strftime('%Y', order_date) = '2026'
		ORDER BY order_date, order_number
	`).Scan(&rows).Error
	if err != nil {
		t.Fatalf("failed to load 2026 orders: %v", err)
	}
	return rows
}

func load2026OrderLineRows(t *testing.T, db *gorm.DB, orders []db2026OrderRow) []db2026LineRow {
	t.Helper()
	var rows []struct {
		OrderID       string
		SourceLineID  string
		LineNumber    int
		OrderNumber   string
		CustomerName  string
		Description   string
		Equipment     string
		Model         string
		Specification string
		Quantity      float64
		UnitPrice     float64
		TotalPrice    float64
		UnitPriceBHD  float64
	}
	err := db.Raw(`
		SELECT
			o.id AS order_id,
			oi.id AS source_line_id,
			COALESCE(oi.line_number, 0) AS line_number,
			o.order_number,
			o.customer_name,
			COALESCE(oi.description, '') AS description,
			COALESCE(oi.equipment, '') AS equipment,
			COALESCE(oi.model, '') AS model,
			COALESCE(oi.specification, '') AS specification,
			COALESCE(oi.quantity, 0) AS quantity,
			COALESCE(oi.unit_price, 0) AS unit_price,
			COALESCE(oi.total_price, 0) AS total_price,
			COALESCE(oi.unit_price_bhd, 0) AS unit_price_bhd
		FROM orders o
		JOIN order_items oi ON oi.order_id = o.id AND oi.deleted_at IS NULL
		WHERE o.deleted_at IS NULL AND strftime('%Y', o.order_date) = '2026'
		ORDER BY o.order_date, o.order_number, oi.line_number
	`).Scan(&rows).Error
	if err != nil {
		t.Fatalf("failed to load 2026 order lines: %v", err)
	}

	orderByID := map[string]db2026OrderRow{}
	for _, order := range orders {
		orderByID[order.ID] = order
	}
	out := make([]db2026LineRow, 0, len(rows))
	for _, row := range rows {
		order := orderByID[row.OrderID]
		unitPrice := row.UnitPriceBHD
		if unitPrice == 0 {
			unitPrice = row.UnitPrice
		}
		class, signal := classifyLegacyInstrumentLine(order.OrderNumber, "", "", "", row.Equipment, firstNonEmptyCostingValue(row.Model, row.Description), row.Specification)
		out = append(out, db2026LineRow{
			EntityKind:       "order",
			EntityID:         row.OrderID,
			SourceLineKind:   "order_item",
			SourceLineID:     row.SourceLineID,
			OrderID:          row.OrderID,
			LineNumber:       row.LineNumber,
			CustomerName:     row.CustomerName,
			DocumentNumber:   row.OrderNumber,
			Description:      row.Description,
			Equipment:        row.Equipment,
			Model:            row.Model,
			Specification:    row.Specification,
			Quantity:         row.Quantity,
			UnitPriceBHD:     unitPrice,
			TotalPriceBHD:    row.TotalPrice,
			InstrumentClass:  class,
			InstrumentSignal: signal,
			SourceConfidence: "order_item",
		})
	}
	return out
}

func build2026OrderEnrichments(orders []db2026OrderRow, lines []db2026LineRow) []db2026OrderEnrichmentRow {
	linesByOrder := map[string][]db2026LineRow{}
	for _, line := range lines {
		linesByOrder[line.OrderID] = append(linesByOrder[line.OrderID], line)
	}
	now := time.Now().Format(time.RFC3339)
	rows := make([]db2026OrderEnrichmentRow, 0, len(orders))
	for _, order := range orders {
		orderLines := linesByOrder[order.ID]
		mix, class, signal, lineTotal := build2026Mix(orderLines)
		rows = append(rows, db2026OrderEnrichmentRow{
			ID:                   "order-enrich:" + order.ID,
			OrderID:              order.ID,
			Year:                 2026,
			OrderNumber:          order.OrderNumber,
			CustomerID:           order.CustomerID,
			CustomerName:         order.CustomerName,
			OrderDate:            order.OrderDate,
			Status:               order.Status,
			GrandTotalBHD:        order.GrandTotal,
			PrimaryClass:         class,
			PrimarySignal:        signal,
			CanonicalProductType: canonicalProductType(class, signal),
			ProductMixJSON:       mustJSON(mix),
			LineItemCount:        len(orderLines),
			LineTotalBHD:         round3(lineTotal),
			EnrichedAt:           now,
		})
	}
	return rows
}

func persist2026Enrichment(db *gorm.DB, oppRows []db2026OpportunityEnrichmentRow, lineRows []db2026LineRow, orderRows []db2026OrderEnrichmentRow, orderLineRows []db2026LineRow, productTypeUpdates map[string]string) error {
	return db.Transaction(func(tx *gorm.DB) error {
		statements := []string{
			`CREATE TABLE IF NOT EXISTS intelligence_opportunity_enrichment (
				id TEXT PRIMARY KEY,
				opportunity_id TEXT NOT NULL,
				year INTEGER NOT NULL,
				source TEXT,
				folder_number TEXT,
				folder_name TEXT,
				customer_id TEXT,
				customer_name TEXT,
				stage TEXT,
				revenue_bhd REAL,
				offer_id TEXT,
				offer_number TEXT,
				quote_type TEXT,
				revision_number INTEGER,
				prepared_by TEXT,
				prepared_by_quality TEXT,
				primary_instrument_class TEXT,
				primary_instrument_signal TEXT,
				canonical_product_type TEXT,
				product_mix_json TEXT,
				line_item_count INTEGER,
				line_total_bhd REAL,
				line_source TEXT,
				has_linked_offer INTEGER,
				has_product_details INTEGER,
				enrichment_notes TEXT,
				enriched_at TEXT
			)`,
			`CREATE TABLE IF NOT EXISTS intelligence_line_item_enrichment (
				id TEXT PRIMARY KEY,
				entity_kind TEXT,
				entity_id TEXT,
				source_line_kind TEXT,
				source_line_id TEXT,
				opportunity_id TEXT,
				offer_id TEXT,
				order_id TEXT,
				line_number INTEGER,
				folder_number TEXT,
				folder_name TEXT,
				customer_name TEXT,
				document_number TEXT,
				description TEXT,
				equipment TEXT,
				model TEXT,
				specification TEXT,
				quantity REAL,
				unit_price_bhd REAL,
				total_price_bhd REAL,
				prepared_by TEXT,
				instrument_class TEXT,
				instrument_signal TEXT,
				source_confidence TEXT,
				enriched_at TEXT
			)`,
			`CREATE TABLE IF NOT EXISTS intelligence_order_enrichment (
				id TEXT PRIMARY KEY,
				order_id TEXT NOT NULL,
				year INTEGER NOT NULL,
				order_number TEXT,
				customer_id TEXT,
				customer_name TEXT,
				order_date TEXT,
				status TEXT,
				grand_total_bhd REAL,
				primary_instrument_class TEXT,
				primary_instrument_signal TEXT,
				canonical_product_type TEXT,
				product_mix_json TEXT,
				line_item_count INTEGER,
				line_total_bhd REAL,
				enriched_at TEXT
			)`,
			`CREATE INDEX IF NOT EXISTS idx_intel_opp_enrich_year ON intelligence_opportunity_enrichment(year, source)`,
			`CREATE INDEX IF NOT EXISTS idx_intel_opp_enrich_customer ON intelligence_opportunity_enrichment(customer_name)`,
			`CREATE INDEX IF NOT EXISTS idx_intel_opp_enrich_instrument ON intelligence_opportunity_enrichment(primary_instrument_class, primary_instrument_signal)`,
			`CREATE INDEX IF NOT EXISTS idx_intel_line_enrich_entity ON intelligence_line_item_enrichment(entity_kind, entity_id)`,
			`CREATE INDEX IF NOT EXISTS idx_intel_line_enrich_instrument ON intelligence_line_item_enrichment(instrument_class, instrument_signal)`,
			`CREATE INDEX IF NOT EXISTS idx_intel_order_enrich_year ON intelligence_order_enrichment(year)`,
			`CREATE INDEX IF NOT EXISTS idx_intel_order_enrich_instrument ON intelligence_order_enrichment(primary_instrument_class, primary_instrument_signal)`,
			`DELETE FROM intelligence_opportunity_enrichment WHERE year = 2026`,
			`DELETE FROM intelligence_line_item_enrichment WHERE (entity_kind = 'opportunity' OR entity_kind = 'order') AND (opportunity_id IN (SELECT id FROM opportunities WHERE year = 2026) OR order_id IN (SELECT id FROM orders WHERE strftime('%Y', order_date) = '2026'))`,
			`DELETE FROM intelligence_order_enrichment WHERE year = 2026`,
		}
		for _, stmt := range statements {
			if err := tx.Exec(stmt).Error; err != nil {
				return err
			}
		}

		for _, row := range oppRows {
			if err := tx.Exec(`INSERT INTO intelligence_opportunity_enrichment (
				id, opportunity_id, year, source, folder_number, folder_name, customer_id, customer_name, stage, revenue_bhd,
				offer_id, offer_number, quote_type, revision_number, prepared_by, prepared_by_quality,
				primary_instrument_class, primary_instrument_signal, canonical_product_type, product_mix_json,
				line_item_count, line_total_bhd, line_source, has_linked_offer, has_product_details, enrichment_notes, enriched_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				row.ID, row.OpportunityID, row.Year, row.Source, row.FolderNumber, row.FolderName, row.CustomerID, row.CustomerName, row.Stage, row.RevenueBHD,
				row.OfferID, row.OfferNumber, row.QuoteType, row.RevisionNumber, row.PreparedBy, row.PreparedByQuality,
				row.PrimaryClass, row.PrimarySignal, row.CanonicalProductType, row.ProductMixJSON,
				row.LineItemCount, row.LineTotalBHD, row.LineSource, row.HasLinkedOffer, row.HasProductDetails, row.EnrichmentNotes, row.EnrichedAt).Error; err != nil {
				return err
			}
		}
		for _, row := range append(lineRows, orderLineRows...) {
			if err := tx.Exec(`INSERT INTO intelligence_line_item_enrichment (
				id, entity_kind, entity_id, source_line_kind, source_line_id, opportunity_id, offer_id, order_id, line_number,
				folder_number, folder_name, customer_name, document_number, description, equipment, model, specification,
				quantity, unit_price_bhd, total_price_bhd, prepared_by, instrument_class, instrument_signal, source_confidence, enriched_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				lineEnrichmentID(row), row.EntityKind, row.EntityID, row.SourceLineKind, row.SourceLineID, row.OpportunityID, row.OfferID, row.OrderID, row.LineNumber,
				row.FolderNumber, row.FolderName, row.CustomerName, row.DocumentNumber, row.Description, row.Equipment, row.Model, row.Specification,
				row.Quantity, row.UnitPriceBHD, row.TotalPriceBHD, row.PreparedBy, row.InstrumentClass, row.InstrumentSignal, row.SourceConfidence, time.Now().Format(time.RFC3339)).Error; err != nil {
				return err
			}
		}
		for _, row := range orderRows {
			if err := tx.Exec(`INSERT INTO intelligence_order_enrichment (
				id, order_id, year, order_number, customer_id, customer_name, order_date, status, grand_total_bhd,
				primary_instrument_class, primary_instrument_signal, canonical_product_type, product_mix_json,
				line_item_count, line_total_bhd, enriched_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				row.ID, row.OrderID, row.Year, row.OrderNumber, row.CustomerID, row.CustomerName, row.OrderDate, row.Status, row.GrandTotalBHD,
				row.PrimaryClass, row.PrimarySignal, row.CanonicalProductType, row.ProductMixJSON, row.LineItemCount, row.LineTotalBHD, row.EnrichedAt).Error; err != nil {
				return err
			}
		}
		for opportunityID, productType := range productTypeUpdates {
			if err := tx.Exec(`UPDATE opportunities SET product_type = ?, updated_at = ? WHERE id = ? AND year = 2026 AND (product_type IS NULL OR TRIM(product_type) = '')`,
				productType, time.Now(), opportunityID).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func build2026Mix(lines []db2026LineRow) ([]db2026MixEntry, string, string, float64) {
	type agg struct {
		class  string
		signal string
		lines  int
		total  float64
	}
	mixMap := map[string]*agg{}
	total := 0.0
	for _, line := range lines {
		class := firstNonEmptyCostingValue(line.InstrumentClass, "unknown")
		signal := line.InstrumentSignal
		key := class + "\x00" + signal
		row := mixMap[key]
		if row == nil {
			row = &agg{class: class, signal: signal}
			mixMap[key] = row
		}
		row.lines++
		row.total += line.TotalPriceBHD
		total += line.TotalPriceBHD
	}
	aggs := make([]*agg, 0, len(mixMap))
	for _, row := range mixMap {
		aggs = append(aggs, row)
	}
	sort.Slice(aggs, func(i, j int) bool {
		if aggs[i].total == aggs[j].total {
			return aggs[i].class < aggs[j].class
		}
		return aggs[i].total > aggs[j].total
	})
	out := make([]db2026MixEntry, 0, len(aggs))
	for _, row := range aggs {
		out = append(out, db2026MixEntry{
			InstrumentClass:  row.class,
			InstrumentSignal: row.signal,
			Lines:            row.lines,
			TotalBHD:         round3(row.total),
		})
	}
	if len(aggs) == 0 {
		return out, "", "", 0
	}
	return out, aggs[0].class, aggs[0].signal, total
}

func canonicalProductType(class, signal string) string {
	switch class {
	case "flow":
		return "Flow (FIT)"
	case "pressure":
		if signal == "DPIT" {
			return "Pressure (DPIT)"
		}
		return "Pressure (PIT)"
	case "temperature":
		return "Temperature (TIT)"
	case "level":
		return "Level (LIT)"
	case "analysis":
		switch signal {
		case "pH":
			return "Analytical (pH)"
		case "conductivity":
			return "Analytical (Conductivity)"
		case "dissolved-oxygen":
			return "Analytical (DO)"
		case "chlorine":
			return "Analytical (Chlorine)"
		case "turbidity":
			return "Analytical (Turbidity)"
		case "ORP":
			return "Analytical (ORP)"
		default:
			return "Analytical (AIT)"
		}
	case "energy_metering":
		return "Energy Metering (BTU)"
	case "valve":
		return "Valves"
	case "gas_analysis":
		return "Gas Analysis"
	case "gas_detection":
		return "Gas Detection"
	case "cems":
		return "CEMS"
	case "junction_box":
		return "Junction Box"
	case "locator":
		return "Locator"
	case "instrumentation_package":
		return "Instrumentation Package"
	case "spares":
		return "Spares"
	case "service":
		return "Service"
	case "panel":
		return "Panel"
	case "software":
		return "Software"
	case "consumable":
		return "Consumables"
	case "indicator_display":
		return "Indicator / Display"
	case "communicator":
		return "Field Communicator"
	default:
		if class != "" && class != "unknown" {
			return strings.Title(strings.ReplaceAll(class, "_", " "))
		}
		return ""
	}
}

func allLineSourceKind(lines []db2026LineRow, kind string) bool {
	for _, line := range lines {
		if line.SourceLineKind != kind {
			return false
		}
	}
	return len(lines) > 0
}

func mustJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func lineEnrichmentID(row db2026LineRow) string {
	return strings.Join([]string{"line-enrich", row.EntityKind, row.EntityID, row.SourceLineKind, row.SourceLineID, intString(row.LineNumber)}, ":")
}

func db2026LineRecord(row db2026LineRow) []string {
	return []string{
		row.EntityKind,
		row.EntityID,
		row.SourceLineKind,
		row.SourceLineID,
		row.FolderNumber,
		row.FolderName,
		row.CustomerName,
		row.DocumentNumber,
		intString(row.LineNumber),
		row.Description,
		row.Equipment,
		row.Model,
		row.Specification,
		floatString(row.Quantity),
		floatString(row.UnitPriceBHD),
		floatString(row.TotalPriceBHD),
		row.PreparedBy,
		row.InstrumentClass,
		row.InstrumentSignal,
		row.SourceConfidence,
	}
}

func copy2026EnrichmentFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
