package main

import (
	"fmt"
	"strings"
	"time"
)

type customerWritePolicy struct {
	validator *InputValidator
	now       func() time.Time
}

func newCustomerWritePolicy(validator *InputValidator) customerWritePolicy {
	if validator == nil {
		validator = NewInputValidator()
	}

	return customerWritePolicy{
		validator: validator,
		now:       time.Now,
	}
}

func (p customerWritePolicy) PrepareForCreate(customer CustomerMaster) (CustomerMaster, error) {
	customer.BusinessName = strings.TrimSpace(customer.BusinessName)
	if customer.BusinessName == "" {
		return customer, fmt.Errorf("business name is required")
	}

	assignCustomerBusinessIdentifiers(&customer, p.now())

	if p.validator != nil {
		if err := p.validator.ValidateCustomerInput(&customer); err != nil {
			return customer, err
		}
	}

	return customer, nil
}

func (p customerWritePolicy) PrepareForUpdate(existing CustomerMaster, customer CustomerMaster) (CustomerMaster, error) {
	if strings.TrimSpace(customer.ID) == "" {
		customer.ID = existing.ID
	}

	customer.BusinessName = strings.TrimSpace(customer.BusinessName)
	if customer.BusinessName == "" {
		customer.BusinessName = existing.BusinessName
	}

	if strings.TrimSpace(customer.CustomerCode) == "" {
		customer.CustomerCode = existing.CustomerCode
	}
	if strings.TrimSpace(customer.CustomerID) == "" {
		customer.CustomerID = existing.CustomerID
	}

	assignCustomerBusinessIdentifiers(&customer, p.now())

	if p.validator != nil {
		if err := p.validator.ValidateCustomerInput(&customer); err != nil {
			return customer, err
		}
	}

	customer.CreatedAt = existing.CreatedAt
	customer.CreatedBy = existing.CreatedBy
	customer.Version = existing.Version + 1

	return customer, nil
}

func assignCustomerBusinessIdentifiers(customer *CustomerMaster, now time.Time) {
	customer.CustomerCode = strings.TrimSpace(customer.CustomerCode)
	customer.CustomerID = strings.TrimSpace(customer.CustomerID)

	if customer.CustomerCode == "" {
		if customer.CustomerID != "" {
			customer.CustomerCode = customer.CustomerID
		} else {
			customer.CustomerCode = defaultCustomerCode(customer.BusinessName, now)
		}
	}
	if customer.CustomerID == "" {
		customer.CustomerID = customer.CustomerCode
	}
}

func defaultCustomerCode(businessName string, now time.Time) string {
	prefix := customerBusinessPrefix(businessName, 4)
	return fmt.Sprintf("CUST-%s%d", prefix, now.UnixMilli()%100000)
}

func businessCustomerIDFromRecord(customer CustomerMaster) string {
	if code := strings.TrimSpace(customer.CustomerCode); code != "" {
		return code
	}
	return fmt.Sprintf("CUST-%s", customerBusinessPrefix(customer.BusinessName, 6))
}

func customerBusinessPrefix(name string, limit int) string {
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
		return "CUST"
	}
	return prefix
}
