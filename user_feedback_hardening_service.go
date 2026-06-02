package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"gorm.io/gorm"
)

type UnifiedThreadEntry struct {
	ID          string    `json:"id"`
	SourceType  string    `json:"source_type"`
	SourceID    string    `json:"source_id"`
	WorkflowKey string    `json:"workflow_key"`
	Comment     string    `json:"comment"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type TenderFolderPreview struct {
	FolderPath  string `json:"folder_path"`
	FolderName  string `json:"folder_name"`
	WorkflowKey string `json:"workflow_key"`
	Title       string `json:"title"`
	Existing    bool   `json:"existing"`
	Status      string `json:"status"`
}

type DataQualityIssue struct {
	ID            string `json:"id"`
	Severity      string `json:"severity"`
	IssueType     string `json:"issue_type"`
	EntityType    string `json:"entity_type"`
	EntityID      string `json:"entity_id"`
	Summary       string `json:"summary"`
	Detail        string `json:"detail"`
	PrimaryAction string `json:"primary_action"`
	ReviewStatus  string `json:"review_status"`
	ReviewNote    string `json:"review_note"`
	ReviewedBy    string `json:"reviewed_by"`
	ReviewedAt    string `json:"reviewed_at"`
}

type DataQualityReview struct {
	Base
	IssueID       string     `gorm:"uniqueIndex;size:180" json:"issue_id"`
	IssueType     string     `gorm:"index;size:80" json:"issue_type"`
	Severity      string     `gorm:"index;size:40" json:"severity"`
	EntityType    string     `gorm:"index;size:80" json:"entity_type"`
	EntityID      string     `gorm:"index;size:100" json:"entity_id"`
	Summary       string     `gorm:"size:500" json:"summary"`
	Detail        string     `gorm:"type:text" json:"detail"`
	PrimaryAction string     `gorm:"size:255" json:"primary_action"`
	Status        string     `gorm:"index;size:40" json:"status"`
	ReviewNote    string     `gorm:"type:text" json:"review_note"`
	ReviewedByID  string     `gorm:"index;size:100" json:"reviewed_by_id"`
	ReviewedBy    string     `gorm:"size:255" json:"reviewed_by"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
}

func (DataQualityReview) TableName() string { return "data_quality_reviews" }

type InventoryPendingFulfillmentRow struct {
	OrderID           string  `json:"order_id"`
	OrderNumber       string  `json:"order_number"`
	CustomerName      string  `json:"customer_name"`
	ProductCode       string  `json:"product_code"`
	Description       string  `json:"description"`
	OrderedQuantity   float64 `json:"ordered_quantity"`
	DeliveredQuantity float64 `json:"delivered_quantity"`
	InvoicedQuantity  float64 `json:"invoiced_quantity"`
	PendingQuantity   float64 `json:"pending_quantity"`
	AvailableQuantity float64 `json:"available_quantity"`
	ShortageQuantity  float64 `json:"shortage_quantity"`
	Status            string  `json:"status"`
}

func (a *App) UpdateOpportunityCommercialFields(opportunityID string, updates map[string]interface{}) (*Opportunity, error) {
	// Commercial opportunity corrections are deliberately open to every
	// licensed employee; destructive actions remain admin-only.
	if err := a.requirePermission("dashboard:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	opportunityID = strings.TrimSpace(opportunityID)
	if opportunityID == "" {
		return nil, fmt.Errorf("opportunity id is required")
	}

	allowed := map[string]bool{
		"folder_number": true, "folder_name": true, "title": true, "eh_ref": true,
		"customer_id": true, "customer_name": true, "salesperson": true, "division": true,
		"delivery_terms": true, "payment_terms": true, "revenue_bhd": true, "cost_bhd": true,
		"profit_bhd": true, "stage": true, "spoc_status": true, "wip_status": true,
		"comment": true, "owner_notes": true, "product_type": true, "product_details": true,
		"customer_grade": true, "source": true, "year": true, "expected_date": true,
		"offer_date": true, "won_reason": true, "lost_reason": true,
	}
	clean := make(map[string]interface{})
	for key, value := range updates {
		if !allowed[key] {
			continue
		}
		switch key {
		case "year":
			year := int(asFloat64(value))
			if year >= 2000 && year <= 2100 {
				clean[key] = year
			}
			continue
		case "expected_date", "offer_date":
			parsed, hasDate, err := parseOpportunityDateValue(value)
			if err != nil {
				return nil, err
			}
			if hasDate {
				clean[key] = parsed
			} else if key == "expected_date" {
				clean[key] = nil
			}
			continue
		}
		if text, ok := value.(string); ok {
			clean[key] = strings.TrimSpace(text)
			continue
		}
		clean[key] = value
	}
	if len(clean) == 0 {
		return nil, fmt.Errorf("no supported opportunity updates supplied")
	}
	if title, ok := clean["title"].(string); ok && len(title) > 500 {
		clean["title"] = title[:500]
	}
	if folderNumber, ok := clean["folder_number"].(string); ok && len(folderNumber) > 50 {
		clean["folder_number"] = folderNumber[:50]
	}

	var updated Opportunity
	var changedFields []string
	if err := a.db.Transaction(func(tx *gorm.DB) error {
		var current Opportunity
		if err := tx.First(&current, "id = ?", opportunityID).Error; err != nil {
			return fmt.Errorf("opportunity not found: %w", err)
		}
		for key, next := range clean {
			if opportunityFieldChanged(current, key, next) {
				changedFields = append(changedFields, key)
			}
		}
		clean["version"] = gorm.Expr("version + ?", 1)
		clean["updated_at"] = time.Now()
		if err := tx.Model(&Opportunity{}).Where("id = ?", opportunityID).Updates(clean).Error; err != nil {
			return fmt.Errorf("failed to update opportunity: %w", err)
		}
		return tx.First(&updated, "id = ?", opportunityID).Error
	}); err != nil {
		return nil, err
	}

	if len(changedFields) > 0 {
		sort.Strings(changedFields)
		_ = a.db.Create(&OpportunityComment{
			OpportunityID: opportunityID,
			Comment:       "Updated fields: " + strings.Join(changedFields, ", "),
			CreatedBy:     firstNonEmpty(a.getCurrentUserDisplayName(), a.getCurrentUserID(), "System"),
		}).Error
	}
	return &updated, nil
}

func (a *App) DeleteOpportunity(opportunityID string) error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if err := a.requireAdminDelete("delete opportunities"); err != nil {
		return err
	}
	if err := a.requirePermission("offers:delete"); err != nil {
		return err
	}

	opportunityID = strings.TrimSpace(opportunityID)
	if opportunityID == "" {
		return fmt.Errorf("opportunity id is required")
	}

	var opportunity Opportunity
	if err := a.db.First(&opportunity, "id = ?", opportunityID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("opportunity not found")
		}
		return fmt.Errorf("failed to load opportunity: %w", err)
	}

	if err := a.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Delete(&Opportunity{}, "id = ?", opportunityID)
		if result.Error != nil {
			return fmt.Errorf("failed to delete opportunity: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("opportunity not found")
		}
		return nil
	}); err != nil {
		return err
	}

	label := firstNonEmpty(opportunity.FolderNumber, opportunity.Title, opportunity.CustomerName, opportunityID)
	log.Printf("✅ Admin deleted opportunity %s", label)
	return nil
}

func parseOpportunityDateValue(value interface{}) (time.Time, bool, error) {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || strings.EqualFold(text, "<nil>") {
		return time.Time{}, false, nil
	}
	if typed, ok := value.(time.Time); ok {
		if typed.IsZero() {
			return time.Time{}, false, nil
		}
		return typed, true, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02", "02/01/2006", "2/1/2006", "01/02/2006"} {
		parsed, err := time.Parse(layout, text)
		if err == nil {
			return parsed, true, nil
		}
	}
	return time.Time{}, false, fmt.Errorf("invalid opportunity date %q", text)
}

func opportunityFieldChanged(current Opportunity, key string, next interface{}) bool {
	nextText := strings.TrimSpace(fmt.Sprint(next))
	switch key {
	case "folder_number":
		return current.FolderNumber != nextText
	case "folder_name":
		return current.FolderName != nextText
	case "title":
		return current.Title != nextText
	case "eh_ref":
		return current.EHRef != nextText
	case "customer_id":
		return current.CustomerID != nextText
	case "customer_name":
		return current.CustomerName != nextText
	case "customer_grade":
		return current.CustomerGrade != nextText
	case "salesperson":
		return current.Salesperson != nextText
	case "division":
		return current.Division != nextText
	case "source":
		return current.Source != nextText
	case "delivery_terms":
		return current.DeliveryTerms != nextText
	case "payment_terms":
		return current.PaymentTerms != nextText
	case "product_details":
		return current.ProductDetails != nextText
	case "stage":
		return current.Stage != nextText
	case "spoc_status":
		return current.SPOCStatus != nextText
	case "wip_status":
		return current.WIPStatus != nextText
	case "comment":
		return current.Comment != nextText
	case "owner_notes":
		return current.OwnerNotes != nextText
	case "product_type":
		return current.ProductType != nextText
	case "won_reason":
		return current.WonReason != nextText
	case "lost_reason":
		return current.LostReason != nextText
	case "year":
		return current.Year != int(asFloat64(next))
	case "expected_date":
		if next == nil {
			return current.ExpectedDate != nil
		}
		parsed, hasDate, _ := parseOpportunityDateValue(next)
		if !hasDate {
			return current.ExpectedDate != nil
		}
		return current.ExpectedDate == nil || !current.ExpectedDate.Equal(parsed)
	case "offer_date":
		parsed, hasDate, _ := parseOpportunityDateValue(next)
		return hasDate && !current.OfferDate.Equal(parsed)
	case "revenue_bhd":
		return fmt.Sprintf("%.3f", current.RevenueBHD) != fmt.Sprintf("%.3f", asFloat64(next))
	case "cost_bhd":
		return fmt.Sprintf("%.3f", current.CostBHD) != fmt.Sprintf("%.3f", asFloat64(next))
	case "profit_bhd":
		return fmt.Sprintf("%.3f", current.ProfitBHD) != fmt.Sprintf("%.3f", asFloat64(next))
	default:
		return true
	}
}

func asFloat64(value interface{}) float64 {
	switch typed := value.(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int64:
		return float64(typed)
	case string:
		parsed, _ := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return parsed
	default:
		parsed, _ := strconv.ParseFloat(fmt.Sprint(value), 64)
		return parsed
	}
}

func (a *App) GetUnifiedOfferThread(offerID string) ([]UnifiedThreadEntry, error) {
	if err := a.requirePermission("offers:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	offerID = strings.TrimSpace(offerID)
	if offerID == "" {
		return nil, fmt.Errorf("offer id is required")
	}

	var offer Offer
	if err := a.db.First(&offer, "id = ?", offerID).Error; err != nil {
		return nil, fmt.Errorf("offer not found: %w", err)
	}

	keys := map[string]bool{}
	addKey := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			keys[value] = true
		}
	}
	addKey(offer.OfferNumber)
	addKey(offer.CustomerReference)
	addKey(offer.RFQID)

	var linkedOpps []Opportunity
	query := a.db.Where("offer_id = ?", offer.ID)
	for key := range keys {
		query = query.Or("folder_number = ? OR eh_ref = ?", key, key)
		if !isPureNumericThreadKey(key) {
			query = query.Or("folder_name LIKE ? OR title LIKE ?", "%"+key+"%", "%"+key+"%")
		}
	}
	var candidateOpps []Opportunity
	_ = query.Find(&candidateOpps).Error
	for _, opp := range candidateOpps {
		if opportunityMatchesThreadKeys(opp, offer.ID, keys) {
			linkedOpps = append(linkedOpps, opp)
		}
	}
	for _, opp := range linkedOpps {
		addKey(opp.FolderNumber)
		addKey(opp.EHRef)
	}

	entries := make([]UnifiedThreadEntry, 0)

	var notes []OfferNote
	if err := a.db.Where("offer_id = ?", offer.ID).Order("note_date ASC, created_at ASC").Limit(200).Find(&notes).Error; err == nil {
		for _, note := range notes {
			entries = append(entries, UnifiedThreadEntry{
				ID:          note.ID,
				SourceType:  "offer",
				SourceID:    offer.ID,
				WorkflowKey: firstThreadKey(keys),
				Comment:     htmlUnescapeForThread(note.Content),
				CreatedBy:   firstNonEmpty(note.CreatedBy, "Legacy note"),
				CreatedAt:   firstNonZeroTime(note.NoteDate, note.CreatedAt),
			})
		}
	}

	for _, opp := range linkedOpps {
		var oppComments []OpportunityComment
		if err := a.db.Where("opportunity_id = ?", opp.ID).Order("created_at ASC").Find(&oppComments).Error; err == nil {
			for _, comment := range oppComments {
				entries = append(entries, UnifiedThreadEntry{
					ID:          fmt.Sprintf("opp-%d", comment.ID),
					SourceType:  "opportunity",
					SourceID:    opp.ID,
					WorkflowKey: firstNonEmpty(opp.FolderNumber, opp.EHRef, firstThreadKey(keys)),
					Comment:     comment.Comment,
					CreatedBy:   firstNonEmpty(comment.CreatedBy, "Legacy note"),
					CreatedAt:   comment.CreatedAt,
				})
			}
		}
		for _, legacy := range []string{opp.Comment, opp.OwnerNotes} {
			if strings.TrimSpace(legacy) == "" {
				continue
			}
			entries = append(entries, UnifiedThreadEntry{
				ID:          "opp-legacy-" + opp.ID + "-" + strconv.Itoa(len(entries)),
				SourceType:  "opportunity",
				SourceID:    opp.ID,
				WorkflowKey: firstNonEmpty(opp.FolderNumber, opp.EHRef, firstThreadKey(keys)),
				Comment:     strings.TrimSpace(legacy),
				CreatedBy:   "Legacy note",
				CreatedAt:   firstNonZeroTime(opp.UpdatedAt, opp.CreatedAt),
			})
		}
	}

	for key := range keys {
		if id, err := strconv.ParseUint(key, 10, 64); err == nil && id > 0 {
			appendRFQThreadEntries(a.db, &entries, uint(id), key)
		}
		var rfqs []RFQData
		if err := a.db.Where("rfq_number = ? OR rfq_ref = ?", key, key).Find(&rfqs).Error; err == nil {
			for _, rfq := range rfqs {
				appendRFQThreadEntries(a.db, &entries, rfq.ID, firstNonEmpty(rfq.RFQNumber, rfq.RFQRef, key))
				if strings.TrimSpace(rfq.Notes) != "" {
					entries = append(entries, UnifiedThreadEntry{
						ID:          fmt.Sprintf("rfq-legacy-%d", rfq.ID),
						SourceType:  "rfq",
						SourceID:    strconv.FormatUint(uint64(rfq.ID), 10),
						WorkflowKey: firstNonEmpty(rfq.RFQNumber, rfq.RFQRef, key),
						Comment:     strings.TrimSpace(rfq.Notes),
						CreatedBy:   "Legacy note",
						CreatedAt:   firstNonZeroTime(rfq.UpdatedAt, rfq.CreatedAt),
					})
				}
			}
		}
	}

	entries = dedupeThreadEntries(entries)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].CreatedAt.Before(entries[j].CreatedAt)
	})
	return entries, nil
}

func appendRFQThreadEntries(db *gorm.DB, entries *[]UnifiedThreadEntry, rfqID uint, workflowKey string) {
	var comments []RFQComment
	if err := db.Where("rfq_id = ?", rfqID).Order("created_at ASC").Find(&comments).Error; err != nil {
		return
	}
	for _, comment := range comments {
		*entries = append(*entries, UnifiedThreadEntry{
			ID:          fmt.Sprintf("rfq-%d", comment.ID),
			SourceType:  "rfq",
			SourceID:    strconv.FormatUint(uint64(rfqID), 10),
			WorkflowKey: workflowKey,
			Comment:     comment.Comment,
			CreatedBy:   firstNonEmpty(comment.CreatedBy, "Legacy note"),
			CreatedAt:   comment.CreatedAt,
		})
	}
}

func opportunityMatchesThreadKeys(opp Opportunity, offerID string, keys map[string]bool) bool {
	if strings.TrimSpace(opp.OfferID) != "" && strings.TrimSpace(opp.OfferID) == offerID {
		return true
	}
	for key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(opp.FolderNumber), key) || strings.EqualFold(strings.TrimSpace(opp.EHRef), key) {
			return true
		}
		if !isPureNumericThreadKey(key) &&
			(threadKeyAppearsAsToken(opp.FolderName, key) || threadKeyAppearsAsToken(opp.Title, key)) {
			return true
		}
	}
	return false
}

func isPureNumericThreadKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return false
	}
	for _, r := range key {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func threadKeyAppearsAsToken(text, key string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	key = strings.ToLower(strings.TrimSpace(key))
	if text == "" || key == "" {
		return false
	}
	start := 0
	for {
		idx := strings.Index(text[start:], key)
		if idx < 0 {
			return false
		}
		absolute := start + idx
		beforeOK := absolute == 0 || !isThreadKeyTokenRune(rune(text[absolute-1]))
		afterIndex := absolute + len(key)
		afterOK := afterIndex >= len(text) || !isThreadKeyTokenRune(rune(text[afterIndex]))
		if beforeOK && afterOK {
			return true
		}
		start = absolute + 1
	}
}

func isThreadKeyTokenRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
}

func dedupeThreadEntries(entries []UnifiedThreadEntry) []UnifiedThreadEntry {
	seen := make(map[string]bool, len(entries))
	out := make([]UnifiedThreadEntry, 0, len(entries))
	for _, entry := range entries {
		key := strings.Join([]string{
			strings.ToLower(strings.TrimSpace(entry.SourceType)),
			strings.TrimSpace(entry.SourceID),
			strings.ToLower(strings.TrimSpace(entry.Comment)),
			entry.CreatedAt.Format(time.RFC3339Nano),
		}, "|")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, entry)
	}
	return out
}

func firstThreadKey(keys map[string]bool) string {
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	if len(ordered) == 0 {
		return ""
	}
	return ordered[0]
}

func htmlUnescapeForThread(value string) string {
	value = strings.ReplaceAll(value, "&lt;", "<")
	value = strings.ReplaceAll(value, "&gt;", ">")
	value = strings.ReplaceAll(value, "&amp;", "&")
	value = strings.ReplaceAll(value, "&#34;", "\"")
	value = strings.ReplaceAll(value, "&#39;", "'")
	return value
}

func firstNonZeroTime(values ...time.Time) time.Time {
	for _, value := range values {
		if !value.IsZero() {
			return value
		}
	}
	return time.Now()
}

var tenderFolderPattern = regexp.MustCompile(`^\s*(\d+)(?:\.\d+)?\s+(.+?)\s*$`)

func ParseTenderFolderName(folderName string) (TenderFolderPreview, bool) {
	matches := tenderFolderPattern.FindStringSubmatch(strings.TrimSpace(folderName))
	if len(matches) != 3 {
		return TenderFolderPreview{}, false
	}
	number, err := strconv.Atoi(matches[1])
	if err != nil || number <= 0 {
		return TenderFolderPreview{}, false
	}
	title := strings.TrimSpace(matches[2])
	if title == "" {
		return TenderFolderPreview{}, false
	}
	return TenderFolderPreview{
		FolderName:  strings.TrimSpace(folderName),
		WorkflowKey: fmt.Sprintf("T-%d", number),
		Title:       title,
		Status:      "RFQ Received / Tender",
	}, true
}

func (a *App) PreviewTenderFolders(rootPath string) ([]TenderFolderPreview, error) {
	if err := a.requirePermission("offers:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	rootPath = strings.TrimSpace(rootPath)
	if rootPath == "" {
		return nil, fmt.Errorf("tender folder path is required")
	}

	entries, err := os.ReadDir(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read tender folder: %w", err)
	}

	previews := make([]TenderFolderPreview, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		preview, ok := ParseTenderFolderName(entry.Name())
		if !ok {
			continue
		}
		preview.FolderPath = filepath.Join(rootPath, entry.Name())
		var count int64
		_ = a.db.Model(&RFQData{}).Where("rfq_number = ? OR rfq_ref = ?", preview.WorkflowKey, preview.WorkflowKey).Count(&count).Error
		if count == 0 {
			_ = a.db.Model(&Opportunity{}).Where("folder_number = ?", preview.WorkflowKey).Count(&count).Error
		}
		preview.Existing = count > 0
		previews = append(previews, preview)
	}

	sort.Slice(previews, func(i, j int) bool {
		return previews[i].WorkflowKey < previews[j].WorkflowKey
	})
	return previews, nil
}

func (a *App) ImportTenderFolders(rootPath string) ([]TenderFolderPreview, error) {
	if err := a.requirePermission("offers:create"); err != nil {
		return nil, err
	}
	previews, err := a.PreviewTenderFolders(rootPath)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	imported := make([]TenderFolderPreview, 0)
	for _, preview := range previews {
		if preview.Existing {
			continue
		}
		rfq := RFQData{
			RFQNumber:     preview.WorkflowKey,
			RFQRef:        preview.WorkflowKey,
			Client:        "Tender",
			Project:       preview.Title,
			Value:         0,
			Notes:         "Imported from tender folder: " + preview.FolderPath,
			Status:        "Tender",
			Stage:         "RFQ Received",
			SourceDocPath: preview.FolderPath,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := a.db.Create(&rfq).Error; err != nil {
			return imported, fmt.Errorf("failed to import tender %s: %w", preview.WorkflowKey, err)
		}
		preview.Existing = true
		imported = append(imported, preview)
	}

	if len(imported) > 0 {
		runtime.EventsEmit(a.ctx, "opportunities:updated", map[string]interface{}{"source": "tender_import", "count": len(imported)})
	}
	return imported, nil
}

func (a *App) PreviewCustomerDataQuality(limit int) ([]DataQualityIssue, error) {
	if err := a.requirePermission("customers:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := a.ensureDataQualityReviewFoundation(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}

	reviews, err := a.dataQualityReviewsByIssueID()
	if err != nil {
		return nil, err
	}
	issues := make([]DataQualityIssue, 0)
	appendIssue := func(issue DataQualityIssue) {
		if review, ok := reviews[issue.ID]; ok {
			switch strings.ToLower(strings.TrimSpace(review.Status)) {
			case "dismissed", "resolved":
				return
			default:
				issue.ReviewStatus = review.Status
				issue.ReviewNote = review.ReviewNote
				issue.ReviewedBy = review.ReviewedBy
				if review.ReviewedAt != nil {
					issue.ReviewedAt = review.ReviewedAt.Format(time.RFC3339)
				}
			}
		}
		if len(issues) < limit {
			issues = append(issues, issue)
		}
	}

	var customers []CustomerMaster
	_ = a.db.Where("deleted_at IS NULL").Find(&customers).Error
	byName := map[string][]CustomerMaster{}
	for _, customer := range customers {
		key := normalizeDataQualityName(customer.BusinessName)
		if key == "" {
			appendIssue(DataQualityIssue{ID: "customer-blank-" + customer.ID, Severity: "high", IssueType: "blank_customer_name", EntityType: "customer", EntityID: customer.ID, Summary: "Customer has no business name", Detail: customer.CustomerID, PrimaryAction: "Complete customer name"})
			continue
		}
		byName[key] = append(byName[key], customer)
	}
	for _, group := range byName {
		if len(group) < 2 {
			continue
		}
		names := make([]string, 0, len(group))
		for _, customer := range group {
			names = append(names, customer.BusinessName)
		}
		appendIssue(DataQualityIssue{ID: "customer-duplicate-" + group[0].ID, Severity: "medium", IssueType: "duplicate_customer", EntityType: "customer", EntityID: group[0].ID, Summary: "Possible duplicate customer records", Detail: strings.Join(names, " | "), PrimaryAction: "Review merge candidates"})
	}

	var opps []Opportunity
	_ = a.db.Limit(limit).Find(&opps).Error
	for _, opp := range opps {
		if strings.TrimSpace(opp.Title) == "" && strings.TrimSpace(opp.FolderName) == "" {
			appendIssue(DataQualityIssue{ID: "opp-blank-title-" + opp.ID, Severity: "medium", IssueType: "blank_opportunity_name", EntityType: "opportunity", EntityID: opp.ID, Summary: "Opportunity has no clear name", Detail: firstNonEmpty(opp.FolderNumber, opp.CustomerName), PrimaryAction: "Add project/opportunity title"})
		}
		if strings.TrimSpace(opp.CustomerID) == "" || strings.TrimSpace(opp.CustomerName) == "" {
			appendIssue(DataQualityIssue{ID: "opp-missing-customer-" + opp.ID, Severity: "high", IssueType: "missing_customer_link", EntityType: "opportunity", EntityID: opp.ID, Summary: "Opportunity is missing a customer link", Detail: firstNonEmpty(opp.FolderNumber, opp.Title), PrimaryAction: "Assign correct customer"})
		}
	}

	var offers []Offer
	_ = a.db.Where("customer_id = '' OR customer_name = ''").Limit(limit).Find(&offers).Error
	for _, offer := range offers {
		appendIssue(DataQualityIssue{ID: "offer-missing-customer-" + offer.ID, Severity: "high", IssueType: "offer_missing_customer", EntityType: "offer", EntityID: offer.ID, Summary: "Offer is missing customer data", Detail: firstNonEmpty(offer.OfferNumber, offer.CustomerReference), PrimaryAction: "Reassign offer customer"})
	}

	return issues, nil
}

func (a *App) ReviewDataQualityIssue(issue DataQualityIssue, action string, note string) (DataQualityReview, error) {
	if !a.currentSessionHasAdminRoleOnly() {
		return DataQualityReview{}, fmt.Errorf("data quality review requires admin permission")
	}
	if a.db == nil {
		return DataQualityReview{}, fmt.Errorf("database not initialized")
	}
	if err := a.ensureDataQualityReviewFoundation(); err != nil {
		return DataQualityReview{}, err
	}

	issue.ID = strings.TrimSpace(issue.ID)
	if issue.ID == "" {
		return DataQualityReview{}, fmt.Errorf("issue id is required")
	}
	status := strings.ToLower(strings.TrimSpace(action))
	switch status {
	case "reviewed", "resolved", "dismissed":
	default:
		return DataQualityReview{}, fmt.Errorf("unsupported data quality review action %q", action)
	}

	now := time.Now()
	reviewerID := strings.TrimSpace(a.getCurrentUserID())
	reviewerName := firstNonEmpty(a.getCurrentUserDisplayName(), reviewerID, "Admin")
	var review DataQualityReview
	err := a.db.Where("issue_id = ?", issue.ID).First(&review).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return DataQualityReview{}, fmt.Errorf("failed to load data quality review: %w", err)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		review = DataQualityReview{IssueID: issue.ID}
	}

	review.IssueType = strings.TrimSpace(issue.IssueType)
	review.Severity = strings.TrimSpace(issue.Severity)
	review.EntityType = strings.TrimSpace(issue.EntityType)
	review.EntityID = strings.TrimSpace(issue.EntityID)
	review.Summary = trimToLength(issue.Summary, 500)
	review.Detail = strings.TrimSpace(issue.Detail)
	review.PrimaryAction = trimToLength(issue.PrimaryAction, 255)
	review.Status = status
	review.ReviewNote = strings.TrimSpace(note)
	review.ReviewedByID = reviewerID
	review.ReviewedBy = reviewerName
	review.ReviewedAt = &now
	review.CreatedBy = firstNonEmpty(review.CreatedBy, reviewerID)

	if err := a.db.Save(&review).Error; err != nil {
		return DataQualityReview{}, fmt.Errorf("failed to save data quality review: %w", err)
	}
	a.logAudit(&reviewerID, "data_quality_"+status, "data_quality_issue", &review.IssueID, review.ReviewNote)
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "data-quality:updated", map[string]interface{}{"issue_id": review.IssueID, "status": review.Status})
	}
	return review, nil
}

func (a *App) GetDataQualityReviewHistory(limit int) ([]DataQualityReview, error) {
	if err := a.requirePermission("customers:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := a.ensureDataQualityReviewFoundation(); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var reviews []DataQualityReview
	if err := a.db.Order("updated_at DESC, created_at DESC").Limit(limit).Find(&reviews).Error; err != nil {
		return nil, fmt.Errorf("failed to load data quality review history: %w", err)
	}
	return reviews, nil
}

func (a *App) ensureDataQualityReviewFoundation() error {
	if a.db == nil {
		return fmt.Errorf("database not initialized")
	}
	if !a.db.Migrator().HasTable("data_quality_reviews") {
		if err := a.db.Exec(`
			CREATE TABLE data_quality_reviews (
				id TEXT PRIMARY KEY,
				created_at DATETIME,
				updated_at DATETIME,
				deleted_at DATETIME,
				version INTEGER DEFAULT 1,
				created_by TEXT,
				issue_id TEXT,
				issue_type TEXT,
				severity TEXT,
				entity_type TEXT,
				entity_id TEXT,
				summary TEXT,
				detail TEXT,
				primary_action TEXT,
				status TEXT,
				review_note TEXT,
				reviewed_by_id TEXT,
				reviewed_by TEXT,
				reviewed_at DATETIME
			)
		`).Error; err != nil {
			return fmt.Errorf("failed to initialize data quality review table: %w", err)
		}
	}
	a.ensureSyncBaseColumns("data_quality_reviews")
	for _, column := range []struct {
		name string
		ddl  string
	}{
		{"issue_id", "TEXT"},
		{"issue_type", "TEXT"},
		{"severity", "TEXT"},
		{"entity_type", "TEXT"},
		{"entity_id", "TEXT"},
		{"summary", "TEXT"},
		{"detail", "TEXT"},
		{"primary_action", "TEXT"},
		{"status", "TEXT"},
		{"review_note", "TEXT"},
		{"reviewed_by_id", "TEXT"},
		{"reviewed_by", "TEXT"},
		{"reviewed_at", "DATETIME"},
	} {
		a.addColumnIfNotExists("data_quality_reviews", column.name, column.ddl)
	}
	if err := a.db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_data_quality_reviews_issue_id ON data_quality_reviews(issue_id)").Error; err != nil {
		return fmt.Errorf("failed to initialize data quality issue index: %w", err)
	}
	_ = a.db.Exec("CREATE INDEX IF NOT EXISTS idx_data_quality_reviews_status ON data_quality_reviews(status)").Error
	_ = a.db.Exec("CREATE INDEX IF NOT EXISTS idx_data_quality_reviews_entity ON data_quality_reviews(entity_type, entity_id)").Error
	_ = a.db.Exec("UPDATE data_quality_reviews SET created_at = CURRENT_TIMESTAMP WHERE created_at IS NULL").Error
	_ = a.db.Exec("UPDATE data_quality_reviews SET updated_at = CURRENT_TIMESTAMP WHERE updated_at IS NULL").Error
	_ = a.db.Exec("UPDATE data_quality_reviews SET version = 1 WHERE version IS NULL OR version = 0").Error
	return nil
}

func (a *App) dataQualityReviewsByIssueID() (map[string]DataQualityReview, error) {
	var reviews []DataQualityReview
	if err := a.db.Find(&reviews).Error; err != nil {
		return nil, fmt.Errorf("failed to load data quality reviews: %w", err)
	}
	out := make(map[string]DataQualityReview, len(reviews))
	for _, review := range reviews {
		out[review.IssueID] = review
	}
	return out, nil
}

func trimToLength(value string, maxLen int) string {
	value = strings.TrimSpace(value)
	if maxLen <= 0 || len(value) <= maxLen {
		return value
	}
	return value[:maxLen]
}

func normalizeDataQualityName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(".", "", ",", "", "(", "", ")", "", "-", " ", "_", " ")
	value = replacer.Replace(value)
	for _, suffix := range []string{" wll", " w l l", " bsc", " b s c", " company", " co"} {
		value = strings.TrimSuffix(value, suffix)
	}
	return strings.Join(strings.Fields(value), " ")
}

func (a *App) GetInventoryPendingFulfillmentReport(limit int) ([]InventoryPendingFulfillmentRow, error) {
	if err := a.requirePermission("inventory:view"); err != nil {
		return nil, err
	}
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if limit <= 0 || limit > 1000 {
		limit = 500
	}

	var orders []Order
	if err := a.db.Preload("Items").Order("order_date DESC, created_at DESC").Limit(limit).Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("failed to load order fulfillment: %w", err)
	}

	rows := make([]InventoryPendingFulfillmentRow, 0)
	for _, order := range orders {
		for _, item := range order.Items {
			delivered := item.QuantityShipped
			if delivered == 0 {
				var dnDelivered float64
				_ = a.db.Model(&DeliveryNoteItem{}).
					Where("order_item_id = ?", item.ID).
					Select("COALESCE(SUM(quantity_delivered), 0)").
					Scan(&dnDelivered).Error
				delivered = dnDelivered
			}
			pending := item.Quantity - delivered
			if pending < 0 {
				pending = 0
			}
			productCode := firstNonEmpty(item.ProductCode, item.Model)
			var available float64
			if strings.TrimSpace(productCode) != "" {
				_ = a.db.Model(&InventoryItem{}).
					Where("deleted_at IS NULL AND is_active = ? AND (product_code = ? OR product_id = ?)", true, productCode, item.ProductID).
					Select("COALESCE(SUM(quantity_available), 0)").
					Scan(&available).Error
			}
			shortage := pending - available
			if shortage < 0 {
				shortage = 0
			}
			rows = append(rows, InventoryPendingFulfillmentRow{
				OrderID:           order.ID,
				OrderNumber:       order.OrderNumber,
				CustomerName:      order.CustomerName,
				ProductCode:       productCode,
				Description:       firstNonEmpty(item.Description, item.Equipment, item.Specification),
				OrderedQuantity:   item.Quantity,
				DeliveredQuantity: delivered,
				InvoicedQuantity:  item.QuantityInvoiced,
				PendingQuantity:   pending,
				AvailableQuantity: available,
				ShortageQuantity:  shortage,
				Status:            order.Status,
			})
		}
	}
	return rows, nil
}

func (a *App) GetInventoryMovementsWorkspace(limit int) ([]StockMovement, error) {
	if limit <= 0 || limit > 1000 {
		limit = 250
	}
	return a.GetStockMovements(nil, "All", time.Time{}, time.Time{}, limit)
}
