// ============================================================================
// ONEDRIVE IMPORT SERVICE - Scan local OneDrive folders and import offer deals
//
// MISSION: Walk locally-synced OneDrive deal folders, detect FINAL subfolders,
// classify documents, fuzzy-match customers, and import as Offer records.
//
// Typical folder structure:
//   <OneDrive root>/
//     EWA LIT Q3 2025/
//       FINAL/
//         RFQ from EWA.pdf
//         Costing Sheet v2.xlsx
//         Quotation QT-2025-045.pdf
//     BAPCO PIT Spares/
//       FINAL/
//         BAPCO RFQ.pdf
//         Costing.xlsx
// ============================================================================

package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// DATA STRUCTURES
// ============================================================================

// DiscoveredDeal represents one deal folder found during the OneDrive scan.
type DiscoveredDeal struct {
	LocalID         string                `json:"local_id"`          // client-side UUID for UI tracking
	FolderPath      string                `json:"folder_path"`       // full path to the parent folder (not FINAL)
	FolderName      string                `json:"folder_name"`       // just the folder name e.g. "EWA LIT Q3 2025"
	FinalPath       string                `json:"final_path"`        // full path to the FINAL subfolder
	RootPath        string                `json:"root_path"`         // which root path this came from
	CustomerMatches []CustomerMatchResult `json:"customer_matches"`
	Files           []DiscoveredFile      `json:"files"`
	InstrumentType  string                `json:"instrument_type"`   // e.g. "Level (LIT)", "Pressure (PIT)"
	YearHint        string                `json:"year_hint"`         // e.g. "2025"
	Status          string                `json:"status"`            // "pending", "confirmed", "importing", "imported", "skipped", "error"
	ErrorMsg        string                `json:"error_msg,omitempty"`
	ConfirmedCustomerID string            `json:"confirmed_customer_id,omitempty"`
	ImportedOfferID     string            `json:"imported_offer_id,omitempty"`
}

// CustomerMatchResult is a single fuzzy-match candidate for a deal folder.
type CustomerMatchResult struct {
	CustomerID   string  `json:"customer_id"`
	BusinessName string  `json:"business_name"`
	ShortCode    string  `json:"short_code"`
	Score        float64 `json:"score"`        // 0.0-1.0
	MatchReason  string  `json:"match_reason"` // "acronym", "shortcode", "token_overlap", "prefix"
}

// DiscoveredFile is a single file found inside a FINAL folder.
type DiscoveredFile struct {
	FileName  string    `json:"file_name"`
	FilePath  string    `json:"file_path"`
	FileType  string    `json:"file_type"` // "costing_sheet", "rfq", "quotation", "order", "document", "email", "unknown"
	Extension string    `json:"extension"`
	SizeBytes int64     `json:"size_bytes"`
	ModTime   time.Time `json:"mod_time"`
}

// OneDriveScanResult is the full result of a scan across one or more root paths.
type OneDriveScanResult struct {
	Deals        []DiscoveredDeal `json:"deals"`
	TotalFolders int              `json:"total_folders"`
	TotalFiles   int              `json:"total_files"`
	ScanPaths    []string         `json:"scan_paths"`
	ScannedAt    time.Time        `json:"scanned_at"`
	Errors       []string         `json:"errors"`
}

// OneDriveImportResult is the per-deal result after ImportOneDriveDeals runs.
type OneDriveImportResult struct {
	DealLocalID           string `json:"deal_local_id"`
	Success               bool   `json:"success"`
	OfferID               string `json:"offer_id,omitempty"`
	Message               string `json:"message"`
	CostingSheetsImported int    `json:"costing_sheets_imported"`
	PDFsQueued            int    `json:"pdfs_queued"`
}

// ============================================================================
// STOP-TOKEN SET — tokens stripped from folder names before customer matching
// ============================================================================

var oneDriveStopTokens = map[string]bool{
	// Instrument tags
	"LIT": true, "PIT": true, "TIT": true, "FIT": true, "GIT": true, "AIT": true,
	"AT": true, "FT": true, "PT": true, "LT": true, "TT": true, "UT": true,
	// Product categories
	"VALVES": true, "VALVE": true, "SPARES": true, "SPARE": true, "PARTS": true,
	"SERVICE": true, "SERVICES": true, "MAINTENANCE": true,
	"SUPPLY": true, "SUPPLIES": true,
	"INSTRUMENTS": true, "INSTRUMENT": true,
	"TRANSMITTERS": true, "TRANSMITTER": true,
	"SENSORS": true, "SENSOR": true,
	"ANALYZERS": true, "ANALYZER": true,
	// Instrument types (spelled out)
	"LEVEL": true, "PRESSURE": true, "TEMPERATURE": true, "FLOW": true,
	"GAS": true, "ANALYTICAL": true,
	// Misc
	"GENERAL": true, "MISC": true, "MISCELLANEOUS": true, "INSTALLATION": true,
	"AND": true, "FOR": true, "THE": true, "OF": true, "IN": true,
	"NO": true, "REF": true,
	// Document types
	"RFQ": true, "OFFER": true, "QUOTE": true, "QT": true,
	// Years & quarters
	"2023": true, "2024": true, "2025": true, "2026": true,
	"Q1": true, "Q2": true, "Q3": true, "Q4": true,
	// Months
	"JAN": true, "FEB": true, "MAR": true, "APR": true, "MAY": true, "JUN": true,
	"JUL": true, "AUG": true, "SEP": true, "OCT": true, "NOV": true, "DEC": true,
	// Version markers
	"FINAL": true, "DRAFT": true, "REV": true, "V1": true, "V2": true, "V3": true, "REVISED": true,
}

// signedWordSkipForAcronym are words that shouldn't contribute a letter to a
// first-letters acronym when building the customer's initials.
var acronymSkipWords = map[string]bool{
	"AND": true, "OF": true, "THE": true, "&": true, "FOR": true,
	"W.L.L": true, "B.S.C": true, "WLL": true, "BSC": true,
	"W.L.L.": true, "B.S.C.": true, "CO": true, "LLC": true,
}

// ============================================================================
// PURE HELPER FUNCTIONS
// ============================================================================

// classifyDiscoveredFile infers the role of a file from its name and extension.
// This is a pure function with no DB access.
func classifyDiscoveredFile(fileName string) string {
	lower := strings.ToLower(fileName)
	ext := strings.ToLower(filepath.Ext(fileName))

	switch ext {
	case ".xlsx", ".xls":
		if strings.Contains(lower, "costing") || strings.Contains(lower, "masterfile") {
			return "costing_sheet"
		}
		return "document"

	case ".pdf":
		switch {
		case strings.Contains(lower, "rfq") ||
			strings.Contains(lower, "enquiry") ||
			strings.Contains(lower, "inquiry"):
			return "rfq"
		case strings.Contains(lower, "quotation") ||
			strings.Contains(lower, "quote") ||
			strings.Contains(lower, "offer") ||
			strings.Contains(lower, "qt-") ||
			strings.Contains(lower, "qt_"):
			return "quotation"
		case strings.Contains(lower, "order") ||
			strings.Contains(lower, "po ") ||
			strings.Contains(lower, "po-") ||
			strings.Contains(lower, "purchase"):
			return "order"
		default:
			return "document"
		}

	case ".docx", ".doc":
		return "document"

	case ".msg":
		return "email"

	default:
		return "unknown"
	}
}

// tokenizeName uppercases and splits a name into individual tokens after
// replacing common separators with spaces.
func tokenizeName(name string) []string {
	upper := strings.ToUpper(name)
	for _, ch := range []string{"-", "_", "/", "(", ")", ".", ","} {
		upper = strings.ReplaceAll(upper, ch, " ")
	}
	raw := strings.Fields(upper)
	out := make([]string, 0, len(raw))
	for _, t := range raw {
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

// extractInstrumentType returns a human-readable label for the first
// instrument-type keyword found in the folder name.
func extractInstrumentType(folderName string) string {
	upper := strings.ToUpper(folderName)
	typeMap := map[string]string{
		"LIT":         "Level (LIT)",
		"PIT":         "Pressure (PIT)",
		"TIT":         "Temperature (TIT)",
		"FIT":         "Flow (FIT)",
		"GIT":         "Gas (GIT)",
		"AIT":         "Analytical (AIT)",
		"VALVE":       "Valves",
		"VALVES":      "Valves",
		"SPARE":       "Spares",
		"SPARES":      "Spares",
		"PARTS":       "Spare Parts",
		"SERVICE":     "Service",
		"MAINTENANCE": "Maintenance",
	}
	// Prioritise exact instrument tags first (order matters for display quality)
	priority := []string{"LIT", "PIT", "TIT", "FIT", "GIT", "AIT"}
	for _, k := range priority {
		// Require word boundary so "SPLIT" doesn't match "LIT"
		re := regexp.MustCompile(`\b` + k + `\b`)
		if re.MatchString(upper) {
			return typeMap[k]
		}
	}
	for k, label := range typeMap {
		if strings.Contains(upper, k) {
			return label
		}
	}
	return ""
}

// extractYearHint returns the first 20xx year string found in the folder name.
func extractYearHint(folderName string) string {
	re := regexp.MustCompile(`20(2[3-9]|3[0-9])`)
	if m := re.FindString(folderName); m != "" {
		return m
	}
	return ""
}

// buildFirstLettersAcronym builds an acronym from the first letter of each
// significant word in the customer's business name.
// e.g. "Electricity & Water Authority" → "EWA"
func buildFirstLettersAcronym(businessName string) string {
	tokens := tokenizeName(businessName)
	var letters []byte
	for _, t := range tokens {
		if acronymSkipWords[t] {
			continue
		}
		if len(t) > 0 {
			letters = append(letters, t[0])
		}
	}
	if len(letters) < 2 {
		return ""
	}
	return string(letters)
}

// maxFloat64 returns the larger of two float64 values.
func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// sliceContains reports whether a string slice contains a given value.
func sliceContains(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

// matchCustomerToFolder is the core fuzzy-match algorithm.
// It returns up to 3 CustomerMatchResult entries with score > 0.1, sorted
// descending by score.  It is a pure function with no DB access.
func matchCustomerToFolder(folderName string, customers []CustomerMaster) []CustomerMatchResult {
	if len(customers) == 0 {
		return nil
	}

	// --- Step A: build cleaned folder tokens (stop-words removed) ---
	allFolderTokens := tokenizeName(folderName)
	var folderTokens []string
	for _, t := range allFolderTokens {
		if !oneDriveStopTokens[t] {
			folderTokens = append(folderTokens, t)
		}
	}

	// Pre-compile the parenthetical acronym extractor once
	parenAcronymRe := regexp.MustCompile(`\(([A-Z]{2,6})\)`)

	var results []CustomerMatchResult

	for _, c := range customers {
		score := 0.0
		reason := ""
		customerNameUpper := strings.ToUpper(c.BusinessName)

		// Rule 1: ShortCode exact match (highest priority)
		if c.ShortCode != "" {
			sc := strings.ToUpper(c.ShortCode)
			if sliceContains(folderTokens, sc) {
				score = 1.0
				reason = "shortcode"
			}
		}

		// Rule 2: Acronym from parentheses in BusinessName e.g. "(EWA)" "(GPIC)"
		if score < 1.0 {
			parenMatches := parenAcronymRe.FindStringSubmatch(customerNameUpper)
			if len(parenMatches) >= 2 {
				acronym := parenMatches[1]
				if sliceContains(folderTokens, acronym) {
					if score < 0.95 {
						score = 0.95
						reason = "acronym"
					}
				}
			}
		}

		// Rule 3: First-letters acronym of customer name
		if score < 0.88 {
			initials := buildFirstLettersAcronym(c.BusinessName)
			if len(initials) >= 2 && sliceContains(folderTokens, initials) {
				score = maxFloat64(score, 0.88)
				reason = "first_letters_acronym"
			}
		}

		// Rule 4: Customer name token is in folder tokens (length >= 3)
		if score < 1.0 {
			customerTokens := tokenizeName(c.BusinessName)
			for _, ct := range customerTokens {
				if len(ct) < 3 {
					continue
				}
				if sliceContains(folderTokens, ct) {
					// Longer token match → higher confidence
					lengthBonus := math.Min(float64(len(ct))/10.0, 0.1)
					candidate := 0.75 + lengthBonus
					if candidate > score {
						score = candidate
						reason = "token_overlap"
					}
				}
			}
		}

		// Rule 5: Partial prefix match (first 4+ chars)
		if score < 0.75 {
			customerTokens := tokenizeName(c.BusinessName)
			for _, ft := range folderTokens {
				if len(ft) < 4 {
					continue
				}
				for _, ct := range customerTokens {
					if len(ct) < 4 {
						continue
					}
					if strings.HasPrefix(ct, ft) || strings.HasPrefix(ft, ct) {
						if score < 0.60 {
							score = 0.60
							reason = "prefix"
						}
					}
				}
			}
		}

		if score > 0.1 {
			results = append(results, CustomerMatchResult{
				CustomerID:   c.ID,
				BusinessName: c.BusinessName,
				ShortCode:    c.ShortCode,
				Score:        score,
				MatchReason:  reason,
			})
		}
	}

	// Sort descending by score
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Return top 3
	if len(results) > 3 {
		results = results[:3]
	}
	return results
}

// generateOneDriveOfferRef produces a unique offer reference for an imported deal.
func generateOneDriveOfferRef(folderName string) string {
	// Sanitise: keep only alphanumeric + space, then replace spaces with dashes
	var sb strings.Builder
	for _, r := range folderName {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	sanitised := strings.Trim(sb.String(), "-")
	// Collapse repeated dashes
	dashRe := regexp.MustCompile(`-{2,}`)
	sanitised = dashRe.ReplaceAllString(sanitised, "-")

	const maxLen = 20
	if len(sanitised) > maxLen {
		sanitised = sanitised[:maxLen]
	}
	ts := time.Now().Format("060102150405") // YYMMDDHHMMSS
	return fmt.Sprintf("OD-%s-%s", sanitised, ts)
}

// ============================================================================
// APP METHODS — Wails-bound
// ============================================================================

// ValidateOneDrivePath checks that a path exists, is a directory, and gives
// an estimate of how many deal folders it contains.
// No RBAC guard is required — this is a safe read-only stat call.
func (a *App) ValidateOneDrivePath(path string) (map[string]interface{}, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if path == "" {
		return map[string]interface{}{"valid": false, "error": "path is empty"}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return map[string]interface{}{"valid": false, "error": err.Error()}, nil
	}
	if !info.IsDir() {
		return map[string]interface{}{"valid": false, "error": "path is not a directory"}, nil
	}

	// Quick estimate: walk max 3 levels deep and count FINAL dirs
	type walkState struct {
		count int
		depth int
	}

	estimatedDeals := 0
	rootDepth := strings.Count(filepath.ToSlash(path), "/")

	_ = filepath.Walk(path, func(p string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return nil // skip unreadable dirs
		}
		if !fi.IsDir() {
			return nil
		}
		currentDepth := strings.Count(filepath.ToSlash(p), "/") - rootDepth
		if currentDepth > 3 {
			return filepath.SkipDir
		}
		if strings.EqualFold(filepath.Base(p), "final") {
			estimatedDeals++
		}
		return nil
	})

	return map[string]interface{}{
		"valid":           true,
		"estimated_deals": estimatedDeals,
		"path":            path,
	}, nil
}

// ScanOneDrivePaths walks each supplied root path, discovers FINAL subfolders,
// classifies files, fuzzy-matches customers, and returns a structured result.
func (a *App) ScanOneDrivePaths(paths []string) (OneDriveScanResult, error) {
	if err := a.requirePermission("settings:view"); err != nil {
		return OneDriveScanResult{}, err
	}
	if a.db == nil {
		return OneDriveScanResult{}, fmt.Errorf("database not initialized")
	}

	const maxDeals = 500

	result := OneDriveScanResult{
		ScanPaths: paths,
		ScannedAt: time.Now(),
	}

	// Load all active customers once
	var customers []CustomerMaster
	if err := a.db.Where("deleted_at IS NULL").Find(&customers).Error; err != nil {
		return OneDriveScanResult{}, fmt.Errorf("failed to load customers: %w", err)
	}

	var scanErrors []string

	for _, rootPath := range paths {
		if rootPath == "" {
			continue
		}

		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				scanErrors = append(scanErrors, fmt.Sprintf("walk error at %s: %v", path, walkErr))
				return nil // continue walking
			}
			if !info.IsDir() {
				return nil
			}

			// We only care about directories named "FINAL" (case-insensitive)
			if !strings.EqualFold(filepath.Base(path), "final") {
				return nil
			}

			finalPath := path
			folderPath := filepath.Dir(finalPath)
			folderName := filepath.Base(folderPath)

			// List direct children of the FINAL dir (non-recursive)
			entries, err := os.ReadDir(finalPath)
			if err != nil {
				scanErrors = append(scanErrors, fmt.Sprintf("cannot read FINAL dir %s: %v", finalPath, err))
				return nil
			}

			var files []DiscoveredFile
			for _, entry := range entries {
				if entry.IsDir() {
					continue
				}
				name := entry.Name()
				// Skip hidden files and Office temp files
				if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "~$") {
					continue
				}

				fi, err := entry.Info()
				if err != nil {
					continue
				}

				result.TotalFiles++
				files = append(files, DiscoveredFile{
					FileName:  name,
					FilePath:  filepath.Join(finalPath, name),
					FileType:  classifyDiscoveredFile(name),
					Extension: strings.ToLower(filepath.Ext(name)),
					SizeBytes: fi.Size(),
					ModTime:   fi.ModTime(),
				})
			}

			// Match customers to this folder
			matches := matchCustomerToFolder(folderName, customers)

			deal := DiscoveredDeal{
				LocalID:         uuid.New().String(),
				FolderPath:      folderPath,
				FolderName:      folderName,
				FinalPath:       finalPath,
				RootPath:        rootPath,
				CustomerMatches: matches,
				Files:           files,
				InstrumentType:  extractInstrumentType(folderName),
				YearHint:        extractYearHint(folderName),
				Status:          "pending",
			}

			result.Deals = append(result.Deals, deal)
			result.TotalFolders++

			if len(result.Deals)%50 == 0 {
				log.Printf("✅ OneDrive scan: %d deals found so far...", len(result.Deals))
			}

			if len(result.Deals) >= maxDeals {
				log.Printf("⚠️ OneDrive scan capped at %d deals — stopping early", maxDeals)
				return filepath.SkipAll
			}

			return nil
		})

		if err != nil && err != filepath.SkipAll {
			scanErrors = append(scanErrors, fmt.Sprintf("error scanning %s: %v", rootPath, err))
		}
	}

	// Sort deals alphabetically by folder name for a predictable UI order
	sort.Slice(result.Deals, func(i, j int) bool {
		return result.Deals[i].FolderName < result.Deals[j].FolderName
	})

	result.Errors = scanErrors
	log.Printf("✅ OneDrive scan complete: %d deals, %d files across %d root paths",
		len(result.Deals), result.TotalFiles, len(paths))
	return result, nil
}

// ConfirmOneDriveDeal validates that the supplied customerID exists in the DB
// and returns the customer's BusinessName.  The frontend holds all scan state;
// this is purely a server-side validation call.
func (a *App) ConfirmOneDriveDeal(localID string, customerID string) (map[string]interface{}, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if customerID == "" {
		return nil, fmt.Errorf("customer ID is required")
	}

	var customer CustomerMaster
	if err := a.db.Where("id = ? AND deleted_at IS NULL", customerID).First(&customer).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("customer not found")
		}
		return nil, fmt.Errorf("failed to look up customer. Please try again or contact support")
	}

	return map[string]interface{}{
		"local_id":      localID,
		"customer_id":   customer.ID,
		"business_name": customer.BusinessName,
		"short_code":    customer.ShortCode,
	}, nil
}

// ImportOneDriveDeals processes a slice of confirmed deals, creating Offer
// records (with items from any costing sheets) for each.
func (a *App) ImportOneDriveDeals(deals []DiscoveredDeal) ([]OneDriveImportResult, error) {
	if err := a.requirePermission("finance:create"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	var results []OneDriveImportResult

	for _, deal := range deals {
		if deal.ConfirmedCustomerID == "" {
			results = append(results, OneDriveImportResult{
				DealLocalID: deal.LocalID,
				Success:     false,
				Message:     "skipped: no customer confirmed",
			})
			continue
		}

		res := a.importSingleDeal(deal)
		results = append(results, res)
	}

	log.Printf("✅ OneDrive import complete: %d deals processed", len(results))
	return results, nil
}

// importSingleDeal handles the DB work for one confirmed DiscoveredDeal.
// It is an unexported helper so it can be called within a loop without
// duplicating the DB-nil guard.
func (a *App) importSingleDeal(deal DiscoveredDeal) OneDriveImportResult {
	res := OneDriveImportResult{DealLocalID: deal.LocalID}

	// Resolve the customer
	var customer CustomerMaster
	if err := a.db.Where("id = ? AND deleted_at IS NULL", deal.ConfirmedCustomerID).
		First(&customer).Error; err != nil {
		res.Success = false
		res.Message = fmt.Sprintf("customer lookup failed: %v", err)
		return res
	}

	// Collect costing sheets and parse them
	var costingData []*ExcelCostingData
	for _, f := range deal.Files {
		if f.FileType != "costing_sheet" {
			continue
		}
		parsed, err := ParseCostingSheet(f.FilePath)
		if err != nil {
			log.Printf("⚠️ OneDrive import: failed to parse costing sheet %s: %v", f.FilePath, err)
			continue
		}
		costingData = append(costingData, parsed)
		res.CostingSheetsImported++
	}

	// Count PDFs that will be associated (queued for later document attachment)
	for _, f := range deal.Files {
		if f.Extension == ".pdf" {
			res.PDFsQueued++
		}
	}

	// Determine totals from first available costing sheet
	var totalValueBHD float64
	var estimatedMargin float64
	var quotationDate time.Time
	var offerItems []OfferItem

	if len(costingData) > 0 {
		primary := costingData[0]
		totalValueBHD = primary.Totals.GrandTotal

		// Compute margin using the same logic as calculateMargin in excel_costing_parser.go
		if primary.Totals.Subtotal > 0 {
			totalCost := 0.0
			for _, item := range primary.LineItems {
				totalCost += item.TotalCost * item.Quantity
			}
			if totalCost > 0 {
				estimatedMargin = (primary.Totals.Subtotal - totalCost) / primary.Totals.Subtotal * 100
			}
		}

		// Try to parse the date from the costing sheet metadata
		if primary.Metadata.Date != "" {
			for _, layout := range []string{"02/01/2006", "2006-01-02", "01/02/2006", "02-Jan-06"} {
				if t, err := time.Parse(layout, primary.Metadata.Date); err == nil {
					quotationDate = t
					break
				}
			}
		}

		// Build offer items
		for i, item := range primary.LineItems {
			oi := OfferItem{
				Base:        Base{ID: uuid.New().String()},
				LineNumber:  i + 1,
				ProductCode: item.Model,
				Model:       item.Model,
				Description: fmt.Sprintf("%s - %s", item.Equipment, item.Model),
				Quantity:    item.Quantity,
				UnitPrice:   item.SuggestedPriceBHD,
				Equipment:   item.Equipment,
				Specification: item.Specification,
				Currency:    "",
				FOB:         item.FobBHD,
				Freight:     item.FreightBHD,
				TotalCost:   item.TotalCost,
				MarginPercent: item.MarkupPercent,
				TotalPrice:  item.TotalSuggestedBHD,
				FobBHD:      item.FobBHD,
				FreightBHD:  item.FreightBHD,
				Insurance:   item.Insurance,
				CustomsBHD:  item.Customs,
				OtherCosts:  item.OtherCosts,
			}
			offerItems = append(offerItems, oi)
		}
	}

	if quotationDate.IsZero() {
		quotationDate = time.Now()
	}

	offerNumber := generateOneDriveOfferRef(deal.FolderName)

	offer := &Offer{
		Base:            Base{ID: uuid.New().String()},
		OfferNumber:     offerNumber,
		RevisionNumber:  1,
		CustomerID:      customer.ID,
		CustomerName:    customer.BusinessName,
		QuotationDate:   quotationDate,
		ValidityDate:    quotationDate.AddDate(0, 3, 0), // 3-month default validity
		TotalValueBHD:   totalValueBHD,
		EstimatedMargin: estimatedMargin,
		Stage:           "Quoted",
	}

	// Wire up OfferID on each item now that offer.ID is set
	for i := range offerItems {
		offerItems[i].OfferID = offer.ID
	}
	offer.Items = offerItems

	if err := a.db.Create(offer).Error; err != nil {
		log.Printf("❌ OneDrive import: failed to save offer for %s: %v", deal.FolderName, err)
		res.Success = false
		res.Message = fmt.Sprintf("failed to save offer record. Please try again or contact support")
		return res
	}

	log.Printf("✅ OneDrive import: created offer %s for %s (%d items, %.3f BHD)",
		offerNumber, customer.BusinessName, len(offerItems), totalValueBHD)

	res.Success = true
	res.OfferID = offer.ID
	res.Message = fmt.Sprintf("imported as offer %s (%d costing items, %d PDFs noted)",
		offerNumber, len(offerItems), res.PDFsQueued)
	return res
}
