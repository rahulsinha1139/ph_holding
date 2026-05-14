package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCollaborativeSyncCursorUsesLocalTimeForSQLiteComparisons(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:collab_cursor?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Setting{}))

	utcCursor := time.Date(2026, 5, 5, 9, 18, 40, 0, time.UTC)
	require.NoError(t, db.Create(&Setting{
		Key:   collaborativeSyncCursorSetting,
		Value: utcCursor.Format(time.RFC3339),
	}).Error)

	app := &App{db: db}
	got := app.getCollaborativeSyncCursor()
	require.Equal(t, utcCursor.Local().Format(time.RFC3339), got.Format(time.RFC3339))

	next := time.Date(2026, 5, 5, 9, 25, 0, 0, time.UTC)
	app.setCollaborativeSyncCursor(next)

	var setting Setting
	require.NoError(t, db.Where("key = ?", collaborativeSyncCursorSetting).First(&setting).Error)
	require.Equal(t, next.Local().Format(time.RFC3339), setting.Value)
}

func TestCollaborativeSyncTablesIncludeReviewAndConflictFlows(t *testing.T) {
	require.Contains(t, collaborativeSyncTables, "delete_approval_requests")
	require.Contains(t, collaborativeSyncTables, "employee_archive_requests")
	require.Contains(t, collaborativeSyncTables, "opportunity_edit_conflicts")
}

func TestSeedEmployeesFromLicenseKeysDoesNotTouchUnchangedAccessLinks(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:employee_link_idempotent?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&LicenseKey{}, &Employee{}, &EmployeeAccessLink{}))

	employee := Employee{
		Base:         Base{ID: "employee-1", CreatedBy: "test"},
		EmployeeCode: "EMP-0001",
		FullName:     "Staff Test",
		IsActive:     true,
	}
	require.NoError(t, db.Create(&employee).Error)
	require.NoError(t, db.Create(&LicenseKey{
		CreatedBy:   "test",
		Key:         "PH-STF-B47E0B",
		Role:        "staff",
		DisplayName: "Staff Test",
	}).Error)

	originalUpdatedAt := time.Date(2026, 5, 1, 9, 0, 0, 0, time.Local)
	link := EmployeeAccessLink{
		Base: Base{
			ID:        "link-1",
			CreatedAt: originalUpdatedAt,
			UpdatedAt: originalUpdatedAt,
			CreatedBy: "test",
		},
		EmployeeID:   employee.ID,
		LicenseKey:   "PH-STF-B47E0B",
		AccessStatus: "active",
		IsPrimary:    true,
	}
	require.NoError(t, db.Create(&link).Error)

	app := &App{db: db}
	require.NoError(t, app.seedEmployeesFromLicenseKeys())

	var after EmployeeAccessLink
	require.NoError(t, db.Where("id = ?", link.ID).First(&after).Error)
	require.Equal(t, originalUpdatedAt.Format(time.RFC3339), after.UpdatedAt.Format(time.RFC3339))
}

func TestReassignCollaborativeTaskCreatesSelfAssignmentNotification(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:self_assignment_notification?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&LicenseKey{},
		&Employee{},
		&EmployeeAccessLink{},
		&TaskItem{},
		&TaskActivity{},
		&Notification{},
		&NotificationReceipt{},
		&CollaborativePendingOperation{},
	))

	app := &App{db: db}
	employee := Employee{
		Base:         Base{ID: "employee-rahul", CreatedBy: "test"},
		EmployeeCode: "EMP-RAHUL",
		FullName:     "Rahul",
		IsActive:     true,
	}
	require.NoError(t, db.Create(&employee).Error)

	now := time.Now()
	licenseKey := "PH-ADM-SELF01"
	require.NoError(t, db.Create(&LicenseKey{
		Key:         licenseKey,
		Role:        "admin",
		DisplayName: "Rahul",
		DeviceHash:  app.getDeviceHash(),
		Activated:   true,
		ActivatedAt: &now,
		CreatedBy:   "test",
	}).Error)
	require.NoError(t, db.Create(&EmployeeAccessLink{
		Base:         Base{ID: "link-rahul", CreatedBy: "test"},
		EmployeeID:   employee.ID,
		LicenseKey:   licenseKey,
		AccessStatus: "active",
		IsPrimary:    true,
	}).Error)

	task := TaskItem{
		Base:              Base{ID: "task-self-assignment", CreatedBy: "test"},
		Title:             "Review ARLA follow-up",
		Status:            "open",
		Priority:          "medium",
		CreatorEmployeeID: employee.ID,
	}
	require.NoError(t, db.Create(&task).Error)

	require.NoError(t, app.ReassignCollaborativeTask(task.ID, employee.ID))

	var count int64
	require.NoError(t, db.Model(&Notification{}).
		Where("employee_id = ? AND source_id = ? AND status = ?", employee.ID, task.ID, "unread").
		Count(&count).Error)
	require.EqualValues(t, 1, count)

	var pendingOps []CollaborativePendingOperation
	require.NoError(t, db.Where("status = ?", "pending").Find(&pendingOps).Error)
	seen := map[string]bool{}
	for _, op := range pendingOps {
		seen[op.EntityType+"/"+op.Operation] = true
	}
	require.True(t, seen["task_activity/create"], "task activity must be queued for remote devices")
	require.True(t, seen["notification/create"], "notification must be queued for remote devices")
	require.True(t, seen["notification_receipt/create"], "notification receipt must be queued for remote devices")
	require.True(t, seen["task/reassign"], "task reassignment must be queued for remote devices")
}
