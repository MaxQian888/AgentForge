package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNewMilestoneRepository(t *testing.T) {
	repo := NewMilestoneRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil MilestoneRepository")
	}
}

func TestMilestoneRepositoryGetWithMetrics(t *testing.T) {
	ctx := context.Background()
	db := openFoundationRepoTestDB(t, &milestoneRecord{}, &taskRecord{}, &sprintRecord{})
	repo := NewMilestoneRepository(db)

	projectID := uuid.New()
	milestoneID := uuid.New()
	now := time.Date(2026, 3, 26, 18, 0, 0, 0, time.UTC)

	milestone := &model.Milestone{
		ID:          milestoneID,
		ProjectID:   projectID,
		Name:        "v2.0",
		TargetDate:  &now,
		Status:      model.MilestoneStatusInProgress,
		Description: "Release milestone",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := repo.Create(ctx, milestone); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if err := db.Create(&taskRecord{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       "Done task",
		Status:      model.TaskStatusDone,
		Priority:    "high",
		MilestoneID: &milestoneID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create done task: %v", err)
	}
	if err := db.Create(&taskRecord{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       "Open task",
		Status:      model.TaskStatusInProgress,
		Priority:    "medium",
		MilestoneID: &milestoneID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create open task: %v", err)
	}
	if err := db.Create(&sprintRecord{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        "Sprint 7",
		Status:      model.SprintStatusActive,
		MilestoneID: &milestoneID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create sprint: %v", err)
	}

	stored, metrics, err := repo.GetWithMetrics(ctx, milestoneID)
	if err != nil {
		t.Fatalf("GetWithMetrics() error = %v", err)
	}
	if stored.ID != milestoneID {
		t.Fatalf("stored milestone id = %s, want %s", stored.ID, milestoneID)
	}
	if metrics.TotalTasks != 2 || metrics.CompletedTasks != 1 {
		t.Fatalf("unexpected task metrics: %+v", metrics)
	}
	if metrics.TotalSprints != 1 {
		t.Fatalf("metrics.TotalSprints = %d, want 1", metrics.TotalSprints)
	}
	if metrics.CompletionRate != 50 {
		t.Fatalf("metrics.CompletionRate = %.2f, want 50", metrics.CompletionRate)
	}
}
