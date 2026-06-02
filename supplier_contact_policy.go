package main

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

type supplierPrimaryContactFields struct {
	Name  string
	Email string
	Phone string
}

func extractSupplierPrimaryContactFields(supplier SupplierMaster) supplierPrimaryContactFields {
	return supplierPrimaryContactFields{
		Name:  strings.TrimSpace(supplier.PrimaryContact),
		Email: strings.TrimSpace(supplier.Email),
		Phone: strings.TrimSpace(supplier.Phone),
	}
}

func (f supplierPrimaryContactFields) isEmpty() bool {
	return f.Name == "" && f.Email == "" && f.Phone == ""
}

func (a *App) syncSupplierPrimaryContactRecord(supplier SupplierMaster) error {
	fields := extractSupplierPrimaryContactFields(supplier)
	if fields.isEmpty() {
		return nil
	}

	var primaryContact SupplierContact
	err := a.db.Where("supplier_id = ? AND is_primary_contact = ?", supplier.ID, true).First(&primaryContact).Error
	switch {
	case err == nil:
		if fields.Name != "" {
			primaryContact.ContactName = fields.Name
		}
		if fields.Email != "" {
			primaryContact.Email = fields.Email
		}
		if fields.Phone != "" {
			primaryContact.Phone = fields.Phone
		}
		return a.db.Save(&primaryContact).Error
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return err
	}

	contact := SupplierContact{
		SupplierID:       supplier.ID,
		ContactName:      fields.Name,
		Email:            fields.Email,
		Phone:            fields.Phone,
		IsPrimaryContact: true,
	}
	return a.db.Create(&contact).Error
}

func (a *App) syncSupplierMasterFromPrimaryContact(contact SupplierContact) error {
	if !contact.IsPrimaryContact {
		return nil
	}

	return a.db.Model(&SupplierMaster{}).
		Where("id = ?", contact.SupplierID).
		Updates(map[string]interface{}{
			"primary_contact": strings.TrimSpace(contact.ContactName),
			"email":           strings.TrimSpace(contact.Email),
			"phone":           strings.TrimSpace(contact.Phone),
		}).Error
}

func (a *App) refreshSupplierMasterPrimaryContact(supplierID string) error {
	var contact SupplierContact
	err := a.db.
		Where("supplier_id = ? AND is_primary_contact = ?", supplierID, true).
		Order("updated_at DESC, created_at DESC").
		First(&contact).Error
	switch {
	case err == nil:
		return a.syncSupplierMasterFromPrimaryContact(contact)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return a.db.Model(&SupplierMaster{}).
			Where("id = ?", supplierID).
			Updates(map[string]interface{}{
				"primary_contact": "",
				"email":           "",
				"phone":           "",
			}).Error
	default:
		return err
	}
}

func (a *App) ensureSinglePrimarySupplierContact(contact SupplierContact) error {
	if !contact.IsPrimaryContact {
		return nil
	}

	return a.db.Model(&SupplierContact{}).
		Where("supplier_id = ? AND id <> ?", contact.SupplierID, contact.ID).
		Update("is_primary_contact", false).Error
}

func (a *App) resolveSupplierPrimaryContactFields(supplier SupplierMaster) (supplierPrimaryContactFields, error) {
	fields := extractSupplierPrimaryContactFields(supplier)
	if !fields.isEmpty() {
		return fields, nil
	}

	var contact SupplierContact
	if err := a.db.Where("supplier_id = ? AND is_primary_contact = ?", supplier.ID, true).First(&contact).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fields, nil
		}
		return fields, err
	}

	return supplierPrimaryContactFields{
		Name:  strings.TrimSpace(contact.ContactName),
		Email: strings.TrimSpace(contact.Email),
		Phone: strings.TrimSpace(contact.Phone),
	}, nil
}
