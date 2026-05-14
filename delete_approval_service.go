package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

type DeleteApprovalRequest struct {
	Base
	EntityType      string     `gorm:"index;size:80" json:"entity_type"`
	EntityID        string     `gorm:"index;size:80" json:"entity_id"`
	EntityLabel     string     `gorm:"size:255" json:"entity_label"`
	RequestedBy     string     `gorm:"index;size:100" json:"requested_by"`
	RequestedByName string     `gorm:"size:255" json:"requested_by_name"`
	RequestedRole   string     `gorm:"size:50" json:"requested_role"`
	Reason          string     `gorm:"type:text" json:"reason"`
	Status          string     `gorm:"index;size:30;default:'pending'" json:"status"`
	ReviewedBy      string     `gorm:"size:100" json:"reviewed_by"`
	ReviewedByName  string     `gorm:"size:255" json:"reviewed_by_name"`
	ReviewedAt      *time.Time `json:"reviewed_at"`
	ReviewNotes     string     `gorm:"type:text" json:"review_notes"`
}

func (DeleteApprovalRequest) TableName() string { return "delete_approval_requests" }

func isDeletePermission(permission string) bool {
	parts := strings.Split(strings.TrimSpace(permission), ":")
	return len(parts) == 2 && parts[1] == "delete"
}

func (a *App) currentSessionHasAdminRoleOnly() bool {
	if a != nil {
		switch strings.ToLower(strings.TrimSpace(a.GetLicenseRole())) {
		case "admin", "administrator", "developer":
			return true
		}
		if a.HasLicensePermission("*") {
			return true
		}
	}

	roleCandidates := make([]string, 0, 4)
	if a != nil && a.currentUser != nil {
		if err := a.hydrateUserRole(a.currentUser); err != nil {
			log.Printf("⚠️ Failed to hydrate current user for admin check: %v", err)
		}
		roleCandidates = append(roleCandidates,
			a.currentUser.Role.Name,
			a.currentUser.RoleName,
			a.currentUser.Role.DisplayName,
		)
		rolePerms := strings.TrimSpace(a.currentUser.Role.Permissions)
		if rolePerms == `["*"]` || strings.Contains(rolePerms, `"*"`) {
			return true
		}
	}
	if a != nil {
		roleCandidates = append(roleCandidates, a.GetCurrentUserRole())
	}
	for _, candidate := range roleCandidates {
		switch strings.ToLower(strings.TrimSpace(candidate)) {
		case "admin", "administrator", "developer":
			return true
		}
	}
	return false
}

func (a *App) guardDeleteOrRequest(permission, entityType, entityID, entityLabel string) (bool, error) {
	if a.currentSessionHasAdminRoleOnly() {
		if err := a.requirePermission(permission); err != nil {
			return false, err
		}
		return true, nil
	}

	request, err := a.RequestDeleteApproval(entityType, entityID, entityLabel, "")
	if err != nil {
		return false, err
	}
	return false, fmt.Errorf("delete approval requested: request %s sent to admin for approval", request.ID)
}

func (a *App) RequestDeleteApproval(entityType, entityID, entityLabel, reason string) (*DeleteApprovalRequest, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}

	entityType = strings.TrimSpace(entityType)
	entityID = strings.TrimSpace(entityID)
	entityLabel = strings.TrimSpace(entityLabel)
	if entityType == "" || entityID == "" {
		return nil, fmt.Errorf("delete approval requires an entity type and id")
	}

	current, err := a.GetCurrentEmployeeContext()
	if err != nil {
		return nil, fmt.Errorf("delete approval requires an authenticated employee: %w", err)
	}
	if a.currentSessionHasAdminRoleOnly() {
		return nil, fmt.Errorf("admin users can delete directly; approval request not needed")
	}
	if entityLabel == "" {
		entityLabel = fmt.Sprintf("%s %s", strings.ReplaceAll(entityType, "_", " "), entityID)
	}

	var existing DeleteApprovalRequest
	err = a.db.Where(
		"entity_type = ? AND entity_id = ? AND requested_by = ? AND status = ?",
		entityType,
		entityID,
		current.EmployeeID,
		"pending",
	).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("failed to check pending delete request: %w", err)
	}

	request := DeleteApprovalRequest{
		Base:            Base{CreatedBy: current.EmployeeID},
		EntityType:      entityType,
		EntityID:        entityID,
		EntityLabel:     entityLabel,
		RequestedBy:     current.EmployeeID,
		RequestedByName: current.EmployeeName,
		RequestedRole:   current.LicenseRole,
		Reason:          strings.TrimSpace(reason),
		Status:          "pending",
	}
	if request.RequestedRole == "" {
		request.RequestedRole = a.GetCurrentUserRole()
	}

	if err := a.db.Create(&request).Error; err != nil {
		return nil, fmt.Errorf("failed to create delete approval request: %w", err)
	}

	a.createDeleteApprovalNotifications(request)
	a.emitCollaborationEvent("notifications:new", map[string]interface{}{
		"source_type": "delete_approval",
		"source_id":   request.ID,
	})
	return &request, nil
}

func (a *App) createDeleteApprovalNotifications(request DeleteApprovalRequest) {
	if a.db == nil {
		return
	}

	var adminIDs []string
	if err := a.db.Table("employees").
		Select("DISTINCT employees.id").
		Joins("JOIN employee_access_links ON employee_access_links.employee_id = employees.id").
		Joins("JOIN license_keys ON license_keys.key = employee_access_links.license_key").
		Where("employees.is_active = ? AND employee_access_links.access_status = ? AND LOWER(license_keys.role) IN ?", true, "active", []string{"admin", "developer"}).
		Pluck("employees.id", &adminIDs).Error; err != nil {
		log.Printf("⚠️ Failed to resolve admin employees for delete request: %v", err)
		return
	}

	if len(adminIDs) == 0 {
		log.Printf("⚠️ Delete approval request %s created but no admin employee profile was found", request.ID)
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"action":            "delete_requested",
		"request_id":        request.ID,
		"entity_type":       request.EntityType,
		"entity_id":         request.EntityID,
		"entity_label":      request.EntityLabel,
		"actor_name":        fallbackDisplay(request.RequestedByName, "Employee"),
		"requested_by":      request.RequestedBy,
		"requested_by_role": request.RequestedRole,
	})

	for _, adminID := range adminIDs {
		notification := Notification{
			Base:             Base{CreatedBy: request.RequestedBy},
			EmployeeID:       adminID,
			NotificationType: "delete_approval",
			Title:            "Delete approval required",
			Message: fmt.Sprintf("%s requested deletion of %s.",
				fallbackDisplay(request.RequestedByName, "An employee"),
				request.EntityLabel,
			),
			Status:        "unread",
			SourceType:    "delete_approval",
			SourceID:      request.ID,
			ActionRoute:   "#notifications",
			ActionPayload: string(payload),
		}
		if err := a.db.Create(&notification).Error; err != nil {
			log.Printf("⚠️ Failed to create delete approval notification: %v", err)
			continue
		}
		a.recordNotificationReceipt(notification.ID, adminID, "", "created")
		if queuePayload, err := json.Marshal(notification); err == nil {
			a.enqueueCollaborativeOperation("notification", notification.ID, "create", string(queuePayload))
		}
	}
}

func (a *App) ReviewDeleteApprovalRequest(requestID, decision, notes string) (*DeleteApprovalRequest, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if !a.currentSessionHasAdminRoleOnly() {
		return nil, fmt.Errorf("only admin can review delete approval requests")
	}

	decision = strings.ToLower(strings.TrimSpace(decision))
	if decision != "approve" && decision != "reject" {
		return nil, fmt.Errorf("decision must be approve or reject")
	}

	var request DeleteApprovalRequest
	if err := a.db.First(&request, "id = ?", strings.TrimSpace(requestID)).Error; err != nil {
		return nil, fmt.Errorf("delete approval request not found: %w", err)
	}
	if request.Status != "pending" {
		return nil, fmt.Errorf("delete request is already %s", request.Status)
	}

	if decision == "approve" {
		if err := a.performApprovedDelete(request); err != nil {
			return nil, err
		}
		request.Status = "approved"
	} else {
		request.Status = "rejected"
	}

	now := time.Now()
	request.ReviewedAt = &now
	request.ReviewedBy = a.getCurrentUserID()
	request.ReviewedByName = a.getCurrentUserDisplayName()
	request.ReviewNotes = strings.TrimSpace(notes)

	if err := a.db.Save(&request).Error; err != nil {
		return nil, fmt.Errorf("failed to save delete approval review: %w", err)
	}

	a.markDeleteApprovalNotificationsRead(request.ID)
	a.notifyDeleteRequester(request, decision)
	a.emitCollaborationEvent("notifications:updated", map[string]interface{}{
		"source_type": "delete_approval",
		"source_id":   request.ID,
		"status":      request.Status,
	})
	return &request, nil
}

func (a *App) markDeleteApprovalNotificationsRead(requestID string) {
	now := time.Now()
	if err := a.db.Model(&Notification{}).
		Where("source_type = ? AND source_id = ?", "delete_approval", requestID).
		Updates(map[string]interface{}{"status": "read", "read_at": &now}).Error; err != nil {
		log.Printf("⚠️ Failed to mark delete approval notifications read: %v", err)
	}
}

func (a *App) notifyDeleteRequester(request DeleteApprovalRequest, decision string) {
	if strings.TrimSpace(request.RequestedBy) == "" {
		return
	}
	title := "Delete request rejected"
	message := fmt.Sprintf("Your delete request for %s was rejected.", request.EntityLabel)
	if decision == "approve" {
		title = "Delete request approved"
		message = fmt.Sprintf("Your delete request for %s was approved and completed.", request.EntityLabel)
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"action":       "delete_reviewed",
		"request_id":   request.ID,
		"entity_type":  request.EntityType,
		"entity_id":    request.EntityID,
		"entity_label": request.EntityLabel,
		"decision":     decision,
		"actor_name":   fallbackDisplay(request.ReviewedByName, "Admin"),
	})
	notification := Notification{
		Base:             Base{CreatedBy: request.ReviewedBy},
		EmployeeID:       request.RequestedBy,
		NotificationType: "delete_approval",
		Title:            title,
		Message:          message,
		Status:           "unread",
		SourceType:       "delete_approval",
		SourceID:         request.ID,
		ActionRoute:      "#notifications",
		ActionPayload:    string(payload),
	}
	if err := a.db.Create(&notification).Error; err != nil {
		log.Printf("⚠️ Failed to notify delete requester: %v", err)
		return
	}
	a.recordNotificationReceipt(notification.ID, request.RequestedBy, "", "created")
}

func (a *App) performApprovedDelete(request DeleteApprovalRequest) error {
	switch request.EntityType {
	case "rfq":
		id, err := strconv.ParseUint(request.EntityID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid RFQ id: %w", err)
		}
		return a.DeleteRFQ(uint(id))
	case "rfq_cascade":
		id, err := strconv.ParseUint(request.EntityID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid RFQ id: %w", err)
		}
		_, err = a.DeleteRFQWithCascade(uint(id), true)
		return err
	case "costing_sheet":
		id, err := strconv.ParseUint(request.EntityID, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid costing sheet id: %w", err)
		}
		return a.DeleteCostingSheet(uint(id))
	case "offer":
		return a.DeleteOffer(request.EntityID)
	case "offer_note":
		return a.DeleteOfferNote(request.EntityID)
	case "order":
		return a.DeleteOrder(request.EntityID)
	case "customer_contact":
		return a.DeleteCustomerContact(request.EntityID)
	case "supplier_contact":
		return a.DeleteSupplierContact(request.EntityID)
	case "customer":
		return a.DeleteCustomer(request.EntityID)
	case "supplier":
		return a.DeleteSupplier(request.EntityID)
	case "customer_invoice":
		return a.DeleteCustomerInvoice(request.EntityID)
	case "payment":
		return a.DeletePayment(request.EntityID)
	case "supplier_payment":
		return a.DeleteSupplierPayment(request.EntityID)
	case "supplier_invoice":
		return a.DeleteSupplierInvoice(request.EntityID)
	case "purchase_order":
		return a.DeletePurchaseOrder(request.EntityID)
	case "delivery_note":
		return a.DeleteDeliveryNote(request.EntityID)
	case "grn":
		return a.DeleteGRN(request.EntityID)
	case "bank_statement":
		return a.DeleteBankStatement(request.EntityID)
	case "bank_statement_line":
		return a.DeleteBankStatementLine(request.EntityID)
	case "expense_category":
		return a.DeleteExpenseCategory(request.EntityID)
	case "expense_vendor":
		return a.DeleteExpenseVendor(request.EntityID)
	case "expense_entry":
		return a.DeleteExpenseEntry(request.EntityID)
	case "recurring_expense":
		return a.DeleteRecurringExpense(request.EntityID)
	case "quick_capture":
		return a.DeleteQuickCapture(request.EntityID)
	case "collaborative_task":
		return a.DeleteCollaborativeTask(request.EntityID)
	default:
		return fmt.Errorf("delete approval cannot execute unsupported entity type %q", request.EntityType)
	}
}
