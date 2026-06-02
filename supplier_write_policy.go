package main

import (
	"fmt"
	"strings"
	"time"
)

type supplierWritePolicy struct {
	validator *InputValidator
	now       func() time.Time
}

func newSupplierWritePolicy(validator *InputValidator) supplierWritePolicy {
	if validator == nil {
		validator = NewInputValidator()
	}

	return supplierWritePolicy{
		validator: validator,
		now:       time.Now,
	}
}

func (p supplierWritePolicy) PrepareForCreate(supplier SupplierMaster) (SupplierMaster, error) {
	supplier.SupplierName = strings.TrimSpace(supplier.SupplierName)
	if supplier.SupplierName == "" {
		return supplier, fmt.Errorf("supplier name is required")
	}

	assignSupplierBusinessIdentifiers(&supplier, p.now())

	if supplier.Rating == 0 {
		supplier.Rating = 3
	}

	if p.validator != nil {
		if err := p.validator.ValidateSupplierInput(&supplier); err != nil {
			return supplier, err
		}
	}

	return supplier, nil
}

func (p supplierWritePolicy) PrepareForUpdate(existing SupplierMaster, supplier SupplierMaster) (SupplierMaster, error) {
	if strings.TrimSpace(supplier.ID) == "" {
		supplier.ID = existing.ID
	}

	supplier.SupplierName = strings.TrimSpace(supplier.SupplierName)
	if supplier.SupplierName == "" {
		supplier.SupplierName = existing.SupplierName
	}

	if strings.TrimSpace(supplier.SupplierCode) == "" {
		supplier.SupplierCode = existing.SupplierCode
	}

	if supplier.Rating == 0 {
		supplier.Rating = existing.Rating
	}

	assignSupplierBusinessIdentifiers(&supplier, p.now())

	if p.validator != nil {
		if err := p.validator.ValidateSupplierInput(&supplier); err != nil {
			return supplier, err
		}
	}

	supplier.CreatedAt = existing.CreatedAt
	supplier.CreatedBy = existing.CreatedBy
	supplier.Version = existing.Version + 1

	return supplier, nil
}

func assignSupplierBusinessIdentifiers(supplier *SupplierMaster, now time.Time) {
	supplier.SupplierCode = strings.TrimSpace(supplier.SupplierCode)
	if supplier.SupplierCode == "" {
		supplier.SupplierCode = defaultSupplierCode(supplier.SupplierName, now)
	}
}

func defaultSupplierCode(supplierName string, now time.Time) string {
	prefix := supplierBusinessPrefix(supplierName, 4)
	return fmt.Sprintf("SUP-%s%d", prefix, now.UnixMilli()%100000)
}

func supplierBusinessPrefix(name string, limit int) string {
	name = strings.ToUpper(name)
	builder := strings.Builder{}
	for _, r := range name {
		if r >= 'A' && r <= 'Z' {
			builder.WriteRune(r)
			if builder.Len() == limit {
				break
			}
		}
	}

	prefix := builder.String()
	if prefix == "" {
		return "SUP"
	}
	return prefix
}
