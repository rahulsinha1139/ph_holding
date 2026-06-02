package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newPhase7TestApp(t *testing.T) *App {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite test db: %v", err)
	}

	app := &App{
		db:            db,
		currentUserID: "test-user",
		currentUser: &User{
			Base: Base{ID: "test-user"},
			Role: Role{
				Name:        "Administrator",
				Permissions: `["*"]`,
			},
		},
	}

	if err := db.AutoMigrate(
		&Setting{},
		&User{},
		&LicenseKey{},
		&Employee{},
		&EmployeeAccessLink{},
		&FollowUpTask{},
		&TaskItem{},
		&TaskActivity{},
		&CollaborativePendingOperation{},
		&PayrollPayout{},
	); err != nil {
		t.Fatalf("failed to migrate test models: %v", err)
	}

	return app
}

func TestEnsurePhase7RolloutBackfillsLegacyFollowUpsOnce(t *testing.T) {
	app := newPhase7TestApp(t)

	employee := Employee{
		FullName:         "Abhie Admin",
		EmployeeCode:     "EMP-001",
		EmploymentStatus: "active",
		IsActive:         true,
	}
	if err := app.db.Create(&employee).Error; err != nil {
		t.Fatalf("failed to seed employee: %v", err)
	}

	now := time.Now().Add(-24 * time.Hour)
	followUp := FollowUpTask{
		Base: Base{
			CreatedAt: now,
			UpdatedAt: now,
			CreatedBy: "legacy-user-1",
		},
		Title:       "Call customer about overdue payment",
		Description: "Legacy CRM follow-up",
		DueDate:     time.Now().Add(48 * time.Hour),
		Status:      "pending",
		Priority:    "high",
		Notes:       "Customer asked for revised payment plan",
	}
	if err := app.db.Create(&followUp).Error; err != nil {
		t.Fatalf("failed to seed follow-up task: %v", err)
	}

	if err := app.EnsurePhase7Rollout(); err != nil {
		t.Fatalf("EnsurePhase7Rollout failed: %v", err)
	}
	if err := app.EnsurePhase7Rollout(); err != nil {
		t.Fatalf("EnsurePhase7Rollout second pass failed: %v", err)
	}

	var tasks []TaskItem
	if err := app.db.Find(&tasks).Error; err != nil {
		t.Fatalf("failed to load task items: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected exactly 1 migrated task, got %d", len(tasks))
	}
	if tasks[0].LegacyFollowUpID == nil || *tasks[0].LegacyFollowUpID != followUp.ID {
		t.Fatalf("expected migrated task to reference legacy follow-up %s", followUp.ID)
	}
	if tasks[0].TaskType != "customer_followup" {
		t.Fatalf("expected customer_followup task type, got %s", tasks[0].TaskType)
	}

	var activities []TaskActivity
	if err := app.db.Find(&activities).Error; err != nil {
		t.Fatalf("failed to load task activity: %v", err)
	}
	if len(activities) != 1 {
		t.Fatalf("expected 1 migration activity, got %d", len(activities))
	}

	status := app.GetPhase7RolloutStatus()
	if status.MigratedLegacyTasks != 1 {
		t.Fatalf("expected rollout status to report 1 migrated legacy task, got %d", status.MigratedLegacyTasks)
	}
}

func TestEnqueueCollaborativeOperationDeduplicatesUpdates(t *testing.T) {
	app := newPhase7TestApp(t)

	app.enqueueCollaborativeOperation("task", "task-123", "update", `{"title":"first"}`)
	app.enqueueCollaborativeOperation("task", "task-123", "update", `{"title":"second"}`)

	var ops []CollaborativePendingOperation
	if err := app.db.Find(&ops).Error; err != nil {
		t.Fatalf("failed to load pending operations: %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("expected one deduplicated pending operation, got %d", len(ops))
	}
	if ops[0].Payload != `{"title":"second"}` {
		t.Fatalf("expected latest payload to win, got %s", ops[0].Payload)
	}

	app.enqueueCollaborativeOperation("task", "task-123", "create", `{"title":"created"}`)
	if err := app.db.Find(&ops).Error; err != nil {
		t.Fatalf("failed to reload pending operations: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("expected create operation to remain separate, got %d records", len(ops))
	}
}

func TestRetryCollaborativePendingOperationRevivesDeadLetter(t *testing.T) {
	app := newPhase7TestApp(t)

	op := CollaborativePendingOperation{
		Base:         Base{CreatedBy: "system"},
		EntityType:   "task",
		EntityID:     "task-999",
		Operation:    "update",
		Payload:      `{"status":"blocked"}`,
		Status:       "dead_letter",
		Attempts:     25,
		ErrorMessage: "remote timeout",
	}
	if err := app.db.Create(&op).Error; err != nil {
		t.Fatalf("failed to seed collaborative operation: %v", err)
	}

	if err := app.RetryCollaborativePendingOperation(op.ID); err != nil {
		t.Fatalf("RetryCollaborativePendingOperation failed: %v", err)
	}

	var updated CollaborativePendingOperation
	if err := app.db.First(&updated, "id = ?", op.ID).Error; err != nil {
		t.Fatalf("failed to reload collaborative operation: %v", err)
	}
	if updated.Status != "pending" {
		t.Fatalf("expected status pending, got %s", updated.Status)
	}
	if updated.Attempts != 0 {
		t.Fatalf("expected attempts reset to 0, got %d", updated.Attempts)
	}
	if updated.ErrorMessage != "" {
		t.Fatalf("expected error message to be cleared, got %q", updated.ErrorMessage)
	}
	if updated.NextAttemptAt == nil {
		t.Fatalf("expected next attempt timestamp to be set")
	}
}

func TestRerunPhase7FollowUpBackfillDoesNotDuplicateMigratedTasks(t *testing.T) {
	app := newPhase7TestApp(t)

	employee := Employee{
		FullName:         "Abhie Admin",
		EmployeeCode:     "EMP-001",
		EmploymentStatus: "active",
		IsActive:         true,
	}
	if err := app.db.Create(&employee).Error; err != nil {
		t.Fatalf("failed to seed employee: %v", err)
	}

	followUp := FollowUpTask{
		Base: Base{
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
			CreatedBy: "legacy-user-1",
		},
		Title:    "Legacy task",
		Status:   "pending",
		Priority: "medium",
	}
	if err := app.db.Create(&followUp).Error; err != nil {
		t.Fatalf("failed to seed follow-up task: %v", err)
	}

	if err := app.EnsurePhase7Rollout(); err != nil {
		t.Fatalf("EnsurePhase7Rollout failed: %v", err)
	}

	result, err := app.RerunPhase7FollowUpBackfill()
	if err != nil {
		t.Fatalf("RerunPhase7FollowUpBackfill failed: %v", err)
	}
	if result.Processed != 0 {
		t.Fatalf("expected rerun to add 0 tasks, got %d", result.Processed)
	}

	var tasks []TaskItem
	if err := app.db.Where("legacy_follow_up_id = ?", followUp.ID).Find(&tasks).Error; err != nil {
		t.Fatalf("failed to load migrated tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected exactly 1 migrated task after rerun, got %d", len(tasks))
	}
}

func TestEnsurePhase7RolloutRepairsStaleFollowUpBackfillMarker(t *testing.T) {
	app := newPhase7TestApp(t)

	employee := Employee{
		FullName:         "Abhie Admin",
		EmployeeCode:     "EMP-001",
		EmploymentStatus: "active",
		IsActive:         true,
	}
	if err := app.db.Create(&employee).Error; err != nil {
		t.Fatalf("failed to seed employee: %v", err)
	}

	followUp := FollowUpTask{
		Base: Base{
			CreatedAt: time.Now().Add(-24 * time.Hour),
			UpdatedAt: time.Now().Add(-24 * time.Hour),
			CreatedBy: "legacy-user-1",
		},
		Title:    "Legacy task with stale marker",
		Status:   "pending",
		Priority: "medium",
	}
	if err := app.db.Create(&followUp).Error; err != nil {
		t.Fatalf("failed to seed follow-up task: %v", err)
	}

	legacyID := followUp.ID
	softDeletedTask := TaskItem{
		Base: Base{
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-2 * time.Hour),
			DeletedAt: gorm.DeletedAt{Time: time.Now().Add(-time.Hour), Valid: true},
		},
		Title:             "Soft-deleted migrated task",
		TaskType:          "customer_followup",
		LegacyFollowUpID:  &legacyID,
		Status:            "blocked",
		Priority:          "medium",
		CreatorEmployeeID: employee.ID,
	}
	if err := app.db.Unscoped().Create(&softDeletedTask).Error; err != nil {
		t.Fatalf("failed to seed stale soft-deleted task: %v", err)
	}
	if err := app.db.Create(&Setting{Key: phase7FollowUpBackfillSetting, Value: "2026-04-08T08:38:42Z"}).Error; err != nil {
		t.Fatalf("failed to seed stale completion setting: %v", err)
	}
	if err := app.db.Create(&Setting{Key: phase7FollowUpBackfillCount, Value: "1"}).Error; err != nil {
		t.Fatalf("failed to seed stale count setting: %v", err)
	}

	if err := app.EnsurePhase7Rollout(); err != nil {
		t.Fatalf("EnsurePhase7Rollout failed: %v", err)
	}

	var activeTasks []TaskItem
	if err := app.db.Where("legacy_follow_up_id = ?", followUp.ID).Find(&activeTasks).Error; err != nil {
		t.Fatalf("failed to load active migrated tasks: %v", err)
	}
	if len(activeTasks) != 1 {
		t.Fatalf("expected one active migrated task to be recreated, got %d", len(activeTasks))
	}
	if activeTasks[0].ID == softDeletedTask.ID {
		t.Fatal("expected repair to create a fresh active task instead of reusing the soft-deleted row")
	}
}
