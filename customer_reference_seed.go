package main

import (
	"encoding/csv"
	"fmt"
	"html"
	"os"
	"strings"

	"gorm.io/gorm"
)

type CustomerReferenceSeedRow struct {
	BusinessName string
	CustomerType string
	ShortCode    string
	SerialNo     string
	CustomerID   string
}

type CustomerReferenceSeedResult struct {
	SeedRows         int `json:"seed_rows"`
	MatchedExisting  int `json:"matched_existing"`
	UpdatedExisting  int `json:"updated_existing"`
	InsertedNew      int `json:"inserted_new"`
	SkippedAmbiguous int `json:"skipped_ambiguous"`
	UnmatchedSeed    int `json:"unmatched_seed"`
}

func normalizeReferenceSeedCustomerType(row CustomerReferenceSeedRow) string {
	shortCode := strings.ToUpper(strings.TrimSpace(row.ShortCode))
	switch shortCode {
	case "EC":
		return "EC"
	case "GOV", "GOVERNMENT":
		return "Government"
	case "SME":
		return "SME"
	case "IND", "INDIVIDUAL":
		return "Individual"
	case "CORP", "CORPORATE":
		return "Corporate"
	}

	label := strings.ToLower(strings.TrimSpace(row.CustomerType))
	switch label {
	case "end customer":
		return "EC"
	case "government":
		return "Government"
	case "sme":
		return "SME"
	case "individual":
		return "Individual"
	case "corporate":
		return "Corporate"
	default:
		return "Corporate"
	}
}

func hasReferenceSeedIdentityConflict(target string, currentID string, customerIDIndex map[string][]CustomerMaster, customerCodeIndex map[string][]CustomerMaster) bool {
	target = strings.ToUpper(strings.TrimSpace(target))
	if target == "" {
		return false
	}

	for _, candidate := range customerIDIndex[target] {
		if candidate.ID != currentID {
			return true
		}
	}
	for _, candidate := range customerCodeIndex[target] {
		if candidate.ID != currentID {
			return true
		}
	}
	return false
}

func loadCustomerReferenceSeed(seedPath string) ([]CustomerReferenceSeedRow, error) {
	file, err := os.Open(seedPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) <= 1 {
		return nil, fmt.Errorf("customer reference seed has no data rows")
	}

	result := make([]CustomerReferenceSeedRow, 0, len(rows)-1)
	for _, record := range rows[1:] {
		if len(record) < 5 {
			continue
		}
		row := CustomerReferenceSeedRow{
			BusinessName: strings.TrimSpace(record[0]),
			CustomerType: strings.TrimSpace(record[1]),
			ShortCode:    strings.TrimSpace(record[2]),
			SerialNo:     strings.TrimSpace(record[3]),
			CustomerID:   strings.TrimSpace(record[4]),
		}
		if row.BusinessName == "" || row.CustomerID == "" {
			continue
		}
		result = append(result, row)
	}
	return result, nil
}

func canonicalCustomerSeedKeys(name string) []string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil
	}

	keys := []string{
		normalizeCustomerName(trimmed),
		normalizeCustomerName(html.UnescapeString(trimmed)),
	}

	expanded := strings.NewReplacer(
		"&", " and ",
		"/", " ",
		"-", " ",
		"(closed)", " ",
		" closed", " ",
	).Replace(strings.ToLower(html.UnescapeString(trimmed)))
	keys = append(keys, normalizeCustomerName(expanded))

	seen := make(map[string]struct{})
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func applyCustomerReferenceSeed(db *gorm.DB, seedPath string) (CustomerReferenceSeedResult, error) {
	var result CustomerReferenceSeedResult

	seedRows, err := loadCustomerReferenceSeed(seedPath)
	if err != nil {
		return result, err
	}
	result.SeedRows = len(seedRows)

	var customers []CustomerMaster
	if err := db.Where("deleted_at IS NULL").Find(&customers).Error; err != nil {
		return result, err
	}

	customerIndex := make(map[string][]CustomerMaster)
	customerIDIndex := make(map[string][]CustomerMaster)
	customerCodeIndex := make(map[string][]CustomerMaster)
	for _, customer := range customers {
		for _, key := range canonicalCustomerSeedKeys(customer.BusinessName) {
			customerIndex[key] = append(customerIndex[key], customer)
		}
		if key := strings.TrimSpace(customer.CustomerID); key != "" {
			customerIDIndex[strings.ToUpper(key)] = append(customerIDIndex[strings.ToUpper(key)], customer)
		}
		if key := strings.TrimSpace(customer.CustomerCode); key != "" {
			customerCodeIndex[strings.ToUpper(key)] = append(customerCodeIndex[strings.ToUpper(key)], customer)
		}
	}

	seenCustomer := make(map[string]struct{})

	err = db.Transaction(func(tx *gorm.DB) error {
		for _, row := range seedRows {
			matchesByID := make(map[string]CustomerMaster)

			for _, match := range customerIDIndex[strings.ToUpper(row.CustomerID)] {
				matchesByID[match.ID] = match
			}
			for _, match := range customerCodeIndex[strings.ToUpper(row.CustomerID)] {
				matchesByID[match.ID] = match
			}
			for _, key := range canonicalCustomerSeedKeys(row.BusinessName) {
				for _, match := range customerIndex[key] {
					matchesByID[match.ID] = match
				}
			}

			if len(matchesByID) == 0 {
				customer := CustomerMaster{
					CustomerID:   row.CustomerID,
					CustomerCode: row.CustomerID,
					CustomerType: normalizeReferenceSeedCustomerType(row),
					BusinessName: row.BusinessName,
					ShortCode:    row.ShortCode,
					Status:       "Active",
				}
				preparedCustomer, err := newCustomerWritePolicy(GlobalValidator).PrepareForCreate(customer)
				if err != nil {
					return fmt.Errorf("prepare customer %s: %w", row.BusinessName, err)
				}
				if err := tx.Create(&preparedCustomer).Error; err != nil {
					return fmt.Errorf("create customer %s: %w", row.BusinessName, err)
				}
				result.InsertedNew++
				continue
			}

			match, ok := selectBestCustomerReferenceMatch(row, matchesByID)
			if !ok {
				result.SkippedAmbiguous++
				continue
			}
			result.MatchedExisting++
			if _, exists := seenCustomer[match.ID]; exists {
				result.SkippedAmbiguous++
				continue
			}
			seenCustomer[match.ID] = struct{}{}

			updatedCustomer := match
			updates := map[string]interface{}{}
			shouldUpdateCode := strings.TrimSpace(match.CustomerCode) == "" || strings.HasPrefix(strings.TrimSpace(match.CustomerCode), "CUST-") || strings.TrimSpace(match.CustomerCode) == strings.TrimSpace(match.CustomerID)
			shouldChangeIdentity := strings.TrimSpace(match.CustomerID) != row.CustomerID || shouldUpdateCode
			if shouldChangeIdentity && hasReferenceSeedIdentityConflict(row.CustomerID, match.ID, customerIDIndex, customerCodeIndex) {
				result.SkippedAmbiguous++
				continue
			}

			if strings.TrimSpace(match.CustomerID) != row.CustomerID {
				updatedCustomer.CustomerID = row.CustomerID
				updates["customer_id"] = row.CustomerID
			}
			if shouldUpdateCode {
				updatedCustomer.CustomerCode = row.CustomerID
				updates["customer_code"] = row.CustomerID
			}
			if strings.TrimSpace(match.ShortCode) != row.ShortCode {
				updatedCustomer.ShortCode = row.ShortCode
				updates["short_code"] = row.ShortCode
			}
			normalizedType := normalizeReferenceSeedCustomerType(row)
			if strings.TrimSpace(match.CustomerType) != normalizedType {
				updatedCustomer.CustomerType = normalizedType
				updates["customer_type"] = normalizedType
			}
			if len(updates) == 0 {
				continue
			}

			preparedCustomer, err := newCustomerWritePolicy(GlobalValidator).PrepareForUpdate(match, updatedCustomer)
			if err != nil {
				return fmt.Errorf("prepare customer %s: %w", row.BusinessName, err)
			}
			if err := tx.Model(&CustomerMaster{}).Where("id = ?", match.ID).Updates(map[string]interface{}{
				"customer_id":   preparedCustomer.CustomerID,
				"customer_code": preparedCustomer.CustomerCode,
				"short_code":    preparedCustomer.ShortCode,
				"customer_type": preparedCustomer.CustomerType,
				"created_at":    preparedCustomer.CreatedAt,
				"created_by":    preparedCustomer.CreatedBy,
				"version":       preparedCustomer.Version,
			}).Error; err != nil {
				return fmt.Errorf("update customer %s: %w", row.BusinessName, err)
			}
			result.UpdatedExisting++
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	result.UnmatchedSeed = result.SeedRows - result.MatchedExisting - result.InsertedNew - result.SkippedAmbiguous
	if result.UnmatchedSeed < 0 {
		result.UnmatchedSeed = 0
	}

	return result, nil
}

func selectBestCustomerReferenceMatch(row CustomerReferenceSeedRow, matches map[string]CustomerMaster) (CustomerMaster, bool) {
	seedNorm := normalizeCustomerName(html.UnescapeString(row.BusinessName))
	bestScore := -1.0
	secondBest := -1.0
	var best CustomerMaster

	for _, candidate := range matches {
		score := 0.0
		if strings.EqualFold(strings.TrimSpace(candidate.CustomerID), row.CustomerID) {
			score += 10000
		}
		if strings.EqualFold(strings.TrimSpace(candidate.CustomerCode), row.CustomerID) {
			score += 9000
		}

		candidateNorm := normalizeCustomerName(html.UnescapeString(candidate.BusinessName))
		if candidateNorm == seedNorm {
			score += 5000
		} else {
			score += fuzzyStringMatch(seedNorm, candidateNorm) * 1000
		}

		score += float64(candidate.TotalOrdersCount) * 20
		score += candidate.TotalOrdersValue / 100
		score += candidate.OutstandingBHD / 100

		if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(candidate.CustomerID)), "CUST-") {
			score += 150
		}
		if strings.TrimSpace(candidate.ShortCode) == "" {
			score += 50
		}

		if score > bestScore {
			secondBest = bestScore
			bestScore = score
			best = candidate
		} else if score > secondBest {
			secondBest = score
		}
	}

	if bestScore < 0 {
		return CustomerMaster{}, false
	}
	if secondBest >= 0 && bestScore-secondBest < 25 {
		return CustomerMaster{}, false
	}
	return best, true
}
