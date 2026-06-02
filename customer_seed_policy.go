package main

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

func normalizeSeedCustomerName(extracted ExtractedCustomer) string {
	if name := strings.TrimSpace(extracted.NormalizedName); name != "" {
		return name
	}
	return strings.TrimSpace(extracted.Name)
}

func defaultSeedCustomer(extracted ExtractedCustomer) CustomerMaster {
	return CustomerMaster{
		BusinessName:  normalizeSeedCustomerName(extracted),
		CustomerType:  "Corporate",
		Country:       "Bahrain",
		City:          "Manama",
		Industry:      "Process Instrumentation",
		PaymentGrade:  "B",
		CustomerGrade: "B",
		Status:        "Active",
	}
}

func (a *App) ensureSeedCustomerFromExtracted(extracted ExtractedCustomer) (*CustomerMaster, bool, error) {
	businessName := normalizeSeedCustomerName(extracted)
	if businessName == "" {
		return nil, false, errors.New("seed customer name is required")
	}

	var customer CustomerMaster
	err := a.db.Where("LOWER(business_name) = ?", strings.ToLower(businessName)).First(&customer).Error
	switch {
	case err == nil:
		if err := a.ensurePrimarySeedCustomerContact(&customer, extracted); err != nil {
			return nil, false, err
		}
		return &customer, false, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return nil, false, err
	}

	customer = defaultSeedCustomer(extracted)
	preparedCustomer, err := newCustomerWritePolicy(GlobalValidator).PrepareForCreate(customer)
	if err != nil {
		return nil, false, err
	}

	if err := a.db.Create(&preparedCustomer).Error; err != nil {
		return nil, false, err
	}

	if err := a.ensurePrimarySeedCustomerContact(&preparedCustomer, extracted); err != nil {
		return nil, false, err
	}

	return &preparedCustomer, true, nil
}

func (a *App) ensurePrimarySeedCustomerContact(customer *CustomerMaster, extracted ExtractedCustomer) error {
	email, phone := extractPrimarySeedCustomerContact(extracted)
	if email == "" && phone == "" {
		return nil
	}

	var primaryContact CustomerContact
	err := a.db.
		Where("customer_id = ? AND is_primary_contact = ?", customer.ID, true).
		First(&primaryContact).Error
	switch {
	case err == nil:
		updated := false
		if primaryContact.Email == "" && email != "" {
			primaryContact.Email = email
			updated = true
		}
		if primaryContact.Phone == "" && phone != "" {
			primaryContact.Phone = phone
			updated = true
		}
		if !updated {
			return nil
		}
		return a.db.Save(&primaryContact).Error
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return err
	}

	contact := CustomerContact{
		CustomerID:       customer.ID,
		Email:            email,
		Phone:            phone,
		IsPrimaryContact: true,
	}
	return a.db.Create(&contact).Error
}

func extractPrimarySeedCustomerContact(extracted ExtractedCustomer) (string, string) {
	var email string
	for _, candidate := range extracted.EmailAddresses {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			email = trimmed
			break
		}
	}

	var phone string
	for _, candidate := range extracted.PhoneNumbers {
		if trimmed := strings.TrimSpace(candidate); trimmed != "" {
			phone = trimmed
			break
		}
	}

	return email, phone
}
