package main

import (
	"encoding/json"
	"errors"
	"strings"

	"gorm.io/gorm"
)

type supplierSeedSpec struct {
	Name         string
	Code         string
	Country      string
	PaymentTerms string
	SupplierType string
	Rating       int
	ProductTypes []string
}

func canonicalSeedSupplierSpecs(extracted []ExtractedSupplier) []supplierSeedSpec {
	specs := []supplierSeedSpec{
		{Name: "Endress+Hauser", Code: "EH", Country: "Germany", PaymentTerms: "Net 60", SupplierType: "Manufacturer", Rating: 5},
		{Name: "Servomex", Code: "SRVX", Country: "UK", PaymentTerms: "Net 45", SupplierType: "Manufacturer", Rating: 5},
		{Name: "Siemens", Code: "SIEM", Country: "Germany", PaymentTerms: "Net 60", SupplierType: "Manufacturer", Rating: 5},
		{Name: "Emerson", Code: "EMER", Country: "USA", PaymentTerms: "Net 45", SupplierType: "Manufacturer", Rating: 5},
		{Name: "ABB", Code: "ABB", Country: "Switzerland", PaymentTerms: "Net 45", SupplierType: "Manufacturer", Rating: 5},
		{Name: "Yokogawa", Code: "YOKO", Country: "Japan", PaymentTerms: "Net 60", SupplierType: "Manufacturer", Rating: 5},
		{Name: "Honeywell", Code: "HON", Country: "USA", PaymentTerms: "Net 45", SupplierType: "Manufacturer", Rating: 5},
		{Name: "GIC", Code: "GIC", Country: "Bahrain", PaymentTerms: "Net 30", SupplierType: "Manufacturer", Rating: 5},
	}

	indexByName := make(map[string]int, len(specs))
	for i, spec := range specs {
		indexByName[strings.ToUpper(strings.TrimSpace(spec.Name))] = i
	}

	for _, supplier := range extracted {
		name := normalizeSeedSupplierName(supplier)
		if name == "" {
			continue
		}

		key := strings.ToUpper(name)
		if idx, ok := indexByName[key]; ok {
			specs[idx].ProductTypes = mergeStringLists(specs[idx].ProductTypes, supplier.Products)
			continue
		}

		specs = append(specs, supplierSeedSpec{
			Name:         name,
			Country:      "Bahrain",
			PaymentTerms: "Net 30",
			SupplierType: "Manufacturer",
			Rating:       5,
			ProductTypes: dedupeStringList(supplier.Products),
		})
		indexByName[key] = len(specs) - 1
	}

	return specs
}

func normalizeSeedSupplierName(extracted ExtractedSupplier) string {
	if name := strings.TrimSpace(extracted.NormalizedName); name != "" {
		return name
	}
	return strings.TrimSpace(extracted.Name)
}

func defaultSeedSupplier(spec supplierSeedSpec) SupplierMaster {
	supplierType := strings.TrimSpace(spec.SupplierType)
	if supplierType == "" {
		supplierType = "Manufacturer"
	}

	rating := spec.Rating
	if rating == 0 {
		rating = 5
	}

	supplier := SupplierMaster{
		SupplierCode: spec.Code,
		SupplierName: strings.TrimSpace(spec.Name),
		Country:      strings.TrimSpace(spec.Country),
		PaymentTerms: strings.TrimSpace(spec.PaymentTerms),
		SupplierType: supplierType,
		Rating:       rating,
	}

	if encoded := mustJSONString(dedupeStringList(spec.ProductTypes)); encoded != "" {
		supplier.ProductTypes = encoded
	}

	return supplier
}

func (a *App) ensureSeedSupplier(spec supplierSeedSpec) (*SupplierMaster, bool, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" {
		return nil, false, errors.New("seed supplier name is required")
	}

	var existing SupplierMaster
	err := a.db.Where("LOWER(supplier_name) = ?", strings.ToLower(name)).First(&existing).Error
	switch {
	case err == nil:
		updatedSupplier := existing
		if strings.TrimSpace(updatedSupplier.SupplierCode) == "" && strings.TrimSpace(spec.Code) != "" {
			updatedSupplier.SupplierCode = spec.Code
		}
		if strings.TrimSpace(updatedSupplier.Country) == "" {
			updatedSupplier.Country = strings.TrimSpace(spec.Country)
		}
		if strings.TrimSpace(updatedSupplier.PaymentTerms) == "" {
			updatedSupplier.PaymentTerms = strings.TrimSpace(spec.PaymentTerms)
		}
		if strings.TrimSpace(updatedSupplier.SupplierType) == "" {
			updatedSupplier.SupplierType = strings.TrimSpace(spec.SupplierType)
		}
		if updatedSupplier.Rating == 0 && spec.Rating != 0 {
			updatedSupplier.Rating = spec.Rating
		}

		mergedProductTypes := mergeJSONStringList(updatedSupplier.ProductTypes, spec.ProductTypes)
		if mergedProductTypes != "" {
			updatedSupplier.ProductTypes = mergedProductTypes
		}

		preparedSupplier, prepErr := newSupplierWritePolicy(GlobalValidator).PrepareForUpdate(existing, updatedSupplier)
		if prepErr != nil {
			return nil, false, prepErr
		}

		if supplierChanged(existing, preparedSupplier) {
			if saveErr := a.db.Save(&preparedSupplier).Error; saveErr != nil {
				return nil, false, saveErr
			}
			existing = preparedSupplier
		}

		return &existing, false, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, false, err
	}

	newSupplier := defaultSeedSupplier(spec)
	preparedSupplier, err := newSupplierWritePolicy(GlobalValidator).PrepareForCreate(newSupplier)
	if err != nil {
		return nil, false, err
	}

	if err := a.db.Create(&preparedSupplier).Error; err != nil {
		return nil, false, err
	}

	return &preparedSupplier, true, nil
}

func supplierChanged(existing SupplierMaster, next SupplierMaster) bool {
	return existing.SupplierCode != next.SupplierCode ||
		existing.Country != next.Country ||
		existing.PaymentTerms != next.PaymentTerms ||
		existing.SupplierType != next.SupplierType ||
		existing.Rating != next.Rating ||
		existing.ProductTypes != next.ProductTypes
}

func mergeJSONStringList(existingJSON string, incoming []string) string {
	existing := parseJSONStringList(existingJSON)
	merged := mergeStringLists(existing, incoming)
	return mustJSONString(merged)
}

func parseJSONStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	var parsed []string
	if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
		return dedupeStringList(parsed)
	}

	return dedupeStringList([]string{raw})
}

func mergeStringLists(existing []string, incoming []string) []string {
	merged := append([]string{}, existing...)
	seen := make(map[string]bool, len(existing))
	for _, value := range existing {
		key := strings.ToUpper(strings.TrimSpace(value))
		if key != "" {
			seen[key] = true
		}
	}

	for _, value := range incoming {
		trimmed := strings.TrimSpace(value)
		key := strings.ToUpper(trimmed)
		if key == "" || seen[key] {
			continue
		}
		merged = append(merged, trimmed)
		seen[key] = true
	}

	return merged
}

func dedupeStringList(values []string) []string {
	return mergeStringLists(nil, values)
}

func mustJSONString(values []string) string {
	if len(values) == 0 {
		return ""
	}

	payload, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(payload)
}
