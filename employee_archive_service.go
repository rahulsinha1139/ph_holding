package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"gorm.io/gorm"
)

type EmployeeArchiveRequest struct {
	Base
	EmployeeID           string     `gorm:"index;size:36" json:"employee_id"`
	EmployeeName         string     `gorm:"size:255" json:"employee_name"`
	RequestedBy          string     `gorm:"index;size:36" json:"requested_by"`
	RequestedByName      string     `gorm:"size:255" json:"requested_by_name"`
	Reason               string     `gorm:"type:text" json:"reason"`
	Status               string     `gorm:"index;size:30;default:'pending'" json:"status"`
	RequiredApprovals    int        `gorm:"default:1" json:"required_approvals"`
	FirstApprovedBy      string     `gorm:"size:36" json:"first_approved_by"`
	FirstApprovedByName  string     `gorm:"size:255" json:"first_approved_by_name"`
	FirstApprovedAt      *time.Time `json:"first_approved_at"`
	SecondApprovedBy     string     `gorm:"size:36" json:"second_approved_by"`
	SecondApprovedByName string     `gorm:"size:255" json:"second_approved_by_name"`
	SecondApprovedAt     *time.Time `json:"second_approved_at"`
	RejectedBy           string     `gorm:"size:36" json:"rejected_by"`
	RejectedByName       string     `gorm:"size:255" json:"rejected_by_name"`
	RejectedAt           *time.Time `json:"rejected_at"`
	ReviewNotes          string     `gorm:"type:text" json:"review_notes"`
}

func (EmployeeArchiveRequest) TableName() string { return "employee_archive_requests" }

func (a *App) RequestEmployeeArchive(employeeID, reason string) (*EmployeeArchiveRequest, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := a.requirePermission("hr:update"); err != nil {
		return nil, err
	}
	if !a.currentSessionHasAdminRoleOnly() {
		return nil, fmt.Errorf("only admin can archive employees")
	}

	employeeID = strings.TrimSpace(employeeID)
	reason = strings.TrimSpace(reason)
	if employeeID == "" {
		return nil, fmt.Errorf("employee id is required")
	}
	if reason == "" {
		return nil, fmt.Errorf("archive reason is required")
	}

	current, err := a.GetCurrentEmployeeContext()
	if err != nil {
		return nil, fmt.Errorf("employee archive requires an authenticated admin employee: %w", err)
	}
	if current.EmployeeID == employeeID {
		return nil, fmt.Errorf("admins cannot archive their own employee profile")
	}

	now := time.Now()
	adminName := fallbackDisplay(current.EmployeeName, a.getCurrentUserDisplayName())
	operation := "create"

	var request EmployeeArchiveRequest
	err = a.db.Transaction(func(tx *gorm.DB) error {
		var employee Employee
		if err := tx.First(&employee, "id = ?", employeeID).Error; err != nil {
			return fmt.Errorf("employee not found: %w", err)
		}
		if !employee.IsActive && strings.EqualFold(employee.EmploymentStatus, "archived") {
			return fmt.Errorf("employee is already archived")
		}

		var existing EmployeeArchiveRequest
		err := tx.Where("employee_id = ? AND status = ?", employeeID, "pending").First(&existing).Error
		if err == nil {
			operation = "update"
			request = existing
			request.Reason = reason
			request.Status = "approved"
			request.RequiredApprovals = 1
			if strings.TrimSpace(request.FirstApprovedBy) == "" {
				request.FirstApprovedBy = current.EmployeeID
				request.FirstApprovedByName = adminName
				request.FirstApprovedAt = &now
			}
			if err := a.performEmployeeArchive(tx, request, current, now); err != nil {
				return err
			}
			return tx.Save(&request).Error
		}
		if err != gorm.ErrRecordNotFound {
			return fmt.Errorf("failed to check pending archive request: %w", err)
		}

		request = EmployeeArchiveRequest{
			Base:                Base{CreatedBy: current.EmployeeID},
			EmployeeID:          employee.ID,
			EmployeeName:        employee.FullName,
			RequestedBy:         current.EmployeeID,
			RequestedByName:     adminName,
			Reason:              reason,
			Status:              "approved",
			RequiredApprovals:   1,
			FirstApprovedBy:     current.EmployeeID,
			FirstApprovedByName: adminName,
			FirstApprovedAt:     &now,
			ReviewNotes:         "Archived by admin",
		}
		if err := tx.Create(&request).Error; err != nil {
			return fmt.Errorf("failed to create employee archive request: %w", err)
		}
		return a.performEmployeeArchive(tx, request, current, now)
	})
	if err != nil {
		return nil, err
	}

	if payload, err := json.Marshal(request); err == nil {
		a.enqueueCollaborativeOperation("employee_archive_request", request.ID, operation, string(payload))
	}
	if payload, err := json.Marshal(map[string]interface{}{
		"employee_id": request.EmployeeID,
		"request_id":  request.ID,
		"archived_by": current.EmployeeID,
		"archived_at": now,
	}); err == nil {
		a.enqueueCollaborativeOperation("employee", request.EmployeeID, "archive", string(payload))
	}
	a.markEmployeeArchiveNotificationsRead(request.ID)
	a.emitCollaborationEvent("employees:updated", map[string]interface{}{
		"employee_id": request.EmployeeID,
		"action":      "archive",
	})
	a.queueCollaborativeSync("employee_archive_request")
	return &request, nil
}

func (a *App) ReviewEmployeeArchiveRequest(requestID, decision, notes string) (*EmployeeArchiveRequest, error) {
	if a.db == nil {
		return nil, fmt.Errorf("database not initialized")
	}
	if err := a.requirePermission("hr:update"); err != nil {
		return nil, err
	}
	if !a.currentSessionHasAdminRoleOnly() {
		return nil, fmt.Errorf("only admin can review employee archive requests")
	}

	requestID = strings.TrimSpace(requestID)
	decision = strings.ToLower(strings.TrimSpace(decision))
	if requestID == "" {
		return nil, fmt.Errorf("archive request id is required")
	}
	if decision != "approve" && decision != "reject" {
		return nil, fmt.Errorf("decision must be approve or reject")
	}

	current, err := a.GetCurrentEmployeeContext()
	if err != nil {
		return nil, fmt.Errorf("employee archive review requires an authenticated admin employee: %w", err)
	}

	var request EmployeeArchiveRequest
	err = a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&request, "id = ?", requestID).Error; err != nil {
			return fmt.Errorf("employee archive request not found: %w", err)
		}
		if request.Status != "pending" {
			return fmt.Errorf("employee archive request is already %s", request.Status)
		}

		now := time.Now()
		request.ReviewNotes = strings.TrimSpace(notes)
		if decision == "reject" {
			request.Status = "rejected"
			request.RejectedBy = current.EmployeeID
			request.RejectedByName = fallbackDisplay(current.EmployeeName, a.getCurrentUserDisplayName())
			request.RejectedAt = &now
			return tx.Save(&request).Error
		}

		if strings.TrimSpace(request.FirstApprovedBy) == "" {
			request.FirstApprovedBy = current.EmployeeID
			request.FirstApprovedByName = fallbackDisplay(current.EmployeeName, a.getCurrentUserDisplayName())
			request.FirstApprovedAt = &now
		}
		request.Status = "approved"
		request.RequiredApprovals = 1
		request.SecondApprovedBy = current.EmployeeID
		request.SecondApprovedByName = fallbackDisplay(current.EmployeeName, a.getCurrentUserDisplayName())
		request.SecondApprovedAt = &now
		if err := a.performEmployeeArchive(tx, request, current, now); err != nil {
			return err
		}
		return tx.Save(&request).Error
	})
	if err != nil {
		return nil, err
	}

	if payload, err := json.Marshal(request); err == nil {
		a.enqueueCollaborativeOperation("employee_archive_request", request.ID, "update", string(payload))
	}
	if request.Status == "approved" {
		if payload, err := json.Marshal(map[string]interface{}{
			"employee_id": request.EmployeeID,
			"request_id":  request.ID,
			"archived_by": request.SecondApprovedBy,
			"archived_at": request.SecondApprovedAt,
		}); err == nil {
			a.enqueueCollaborativeOperation("employee", request.EmployeeID, "archive", string(payload))
		}
	}
	a.markEmployeeArchiveNotificationsRead(request.ID)
	a.notifyEmployeeArchiveRequester(request, decision)
	if request.Status == "approved" {
		a.emitCollaborationEvent("employees:updated", map[string]interface{}{
			"employee_id": request.EmployeeID,
			"action":      "archive",
		})
	}
	a.emitCollaborationEvent("notifications:updated", map[string]interface{}{
		"source_type": "employee_archive_approval",
		"source_id":   request.ID,
		"status":      request.Status,
	})
	a.queueCollaborativeSync("employee_archive_review")
	return &request, nil
}

func (a *App) performEmployeeArchive(tx *gorm.DB, request EmployeeArchiveRequest, reviewer CurrentEmployeeContext, archivedAt time.Time) error {
	var employee Employee
	if err := tx.First(&employee, "id = ?", request.EmployeeID).Error; err != nil {
		return fmt.Errorf("employee not found: %w", err)
	}
	if reviewer.EmployeeID == employee.ID {
		return fmt.Errorf("admins cannot archive their own employee profile")
	}

	if err := tx.Model(&employee).Updates(map[string]interface{}{
		"is_active":          false,
		"employment_status":  "archived",
		"end_date":           &archivedAt,
		"archived_at":        &archivedAt,
		"archived_by":        reviewer.EmployeeID,
		"archive_reason":     request.Reason,
		"archive_request_id": request.ID,
		"updated_at":         archivedAt,
	}).Error; err != nil {
		return fmt.Errorf("failed to archive employee profile: %w", err)
	}

	if err := tx.Model(&EmployeeAccessLink{}).
		Where("employee_id = ? AND access_status <> ?", request.EmployeeID, "archived").
		Updates(map[string]interface{}{
			"access_status": "archived",
			"is_primary":    false,
			"updated_at":    archivedAt,
		}).Error; err != nil {
		return fmt.Errorf("failed to archive employee access links: %w", err)
	}

	if err := tx.Model(&ProjectMember{}).
		Where("employee_id = ? AND is_active = ?", request.EmployeeID, true).
		Updates(map[string]interface{}{
			"is_active":  false,
			"left_at":    &archivedAt,
			"updated_at": archivedAt,
		}).Error; err != nil {
		return fmt.Errorf("failed to close employee project memberships: %w", err)
	}

	return nil
}

func (a *App) markEmployeeArchiveNotificationsRead(requestID string) {
	if a.db == nil {
		return
	}
	now := time.Now()
	_ = a.db.Model(&Notification{}).
		Where("source_type = ? AND source_id = ? AND status <> ?", "employee_archive_approval", requestID, "read").
		Updates(map[string]interface{}{
			"status":     "read",
			"read_at":    &now,
			"updated_at": now,
		}).Error
}

func (a *App) notifyEmployeeArchiveRequester(request EmployeeArchiveRequest, decision string) {
	if a.db == nil || strings.TrimSpace(request.RequestedBy) == "" {
		return
	}

	statusLabel := "approved"
	title := "Employee archive approved"
	if decision == "reject" {
		statusLabel = "rejected"
		title = "Employee archive rejected"
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"action":        "employee_archive_reviewed",
		"request_id":    request.ID,
		"employee_id":   request.EmployeeID,
		"employee_name": request.EmployeeName,
		"status":        request.Status,
		"decision":      decision,
	})
	notification := Notification{
		Base:             Base{CreatedBy: fallbackDisplay(request.SecondApprovedBy, request.RejectedBy)},
		EmployeeID:       request.RequestedBy,
		NotificationType: "employee_archive_approval",
		Title:            title,
		Message:          fmt.Sprintf("Archive request for %s was %s.", request.EmployeeName, statusLabel),
		Status:           "unread",
		SourceType:       "employee_archive_approval",
		SourceID:         request.ID,
		ActionRoute:      "#people",
		ActionPayload:    string(payload),
	}
	if err := a.db.Create(&notification).Error; err != nil {
		log.Printf("⚠️ Failed to notify employee archive requester: %v", err)
		return
	}
	a.recordNotificationReceipt(notification.ID, request.RequestedBy, "", "created")
	if queuePayload, err := json.Marshal(notification); err == nil {
		a.enqueueCollaborativeOperation("notification", notification.ID, "create", string(queuePayload))
	}
}
