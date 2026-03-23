package model_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestValidateTransition(t *testing.T) {
	t.Run("allows known transition", func(t *testing.T) {
		if err := model.ValidateTransition(model.TaskStatusInbox, model.TaskStatusTriaged); err != nil {
			t.Fatalf("ValidateTransition() error = %v, want nil", err)
		}
	})

	t.Run("rejects invalid transition", func(t *testing.T) {
		err := model.ValidateTransition(model.TaskStatusInbox, model.TaskStatusDone)
		if err == nil {
			t.Fatal("ValidateTransition() error = nil, want invalid transition")
		}
	})

	t.Run("rejects unknown source status", func(t *testing.T) {
		err := model.ValidateTransition("mystery", model.TaskStatusDone)
		if err == nil {
			t.Fatal("ValidateTransition() error = nil, want unknown status")
		}
	})
}

func TestTaskToDTOWithOptionalFields(t *testing.T) {
	now := time.Date(2026, 3, 23, 14, 30, 0, 0, time.UTC)
	parentID := uuid.New()
	sprintID := uuid.New()
	assigneeID := uuid.New()
	reporterID := uuid.New()
	completedAt := now.Add(2 * time.Hour)

	task := &model.Task{
		ID:             uuid.New(),
		ProjectID:      uuid.New(),
		ParentID:       &parentID,
		SprintID:       &sprintID,
		Title:          "Implement tests",
		Description:    "Cover the task DTO and transitions",
		Status:         model.TaskStatusDone,
		Priority:       "high",
		AssigneeID:     &assigneeID,
		AssigneeType:   "agent",
		ReporterID:     &reporterID,
		Labels:         []string{"backend", "tests"},
		BudgetUsd:      12.5,
		SpentUsd:       8.25,
		AgentBranch:    "feat/tests",
		AgentWorktree:  "D:/worktrees/task",
		AgentSessionID: "session-1",
		PRUrl:          "https://example.com/pulls/1",
		PRNumber:       1,
		BlockedBy:      []string{"task-0"},
		CreatedAt:      now,
		UpdatedAt:      now.Add(time.Minute),
		CompletedAt:    &completedAt,
	}

	dto := task.ToDTO()

	if dto.ID != task.ID.String() || dto.ProjectID != task.ProjectID.String() {
		t.Fatalf("unexpected ID fields in DTO: %+v", dto)
	}
	if dto.ParentID == nil || *dto.ParentID != parentID.String() {
		t.Fatalf("ParentID = %v, want %s", dto.ParentID, parentID.String())
	}
	if dto.SprintID == nil || *dto.SprintID != sprintID.String() {
		t.Fatalf("SprintID = %v, want %s", dto.SprintID, sprintID.String())
	}
	if dto.AssigneeID == nil || *dto.AssigneeID != assigneeID.String() {
		t.Fatalf("AssigneeID = %v, want %s", dto.AssigneeID, assigneeID.String())
	}
	if dto.ReporterID == nil || *dto.ReporterID != reporterID.String() {
		t.Fatalf("ReporterID = %v, want %s", dto.ReporterID, reporterID.String())
	}
	if dto.CompletedAt == nil || *dto.CompletedAt != completedAt.Format(time.RFC3339) {
		t.Fatalf("CompletedAt = %v, want %s", dto.CompletedAt, completedAt.Format(time.RFC3339))
	}
	if dto.CreatedAt != now.Format(time.RFC3339) || dto.UpdatedAt != now.Add(time.Minute).Format(time.RFC3339) {
		t.Fatalf("unexpected timestamp formatting: created=%s updated=%s", dto.CreatedAt, dto.UpdatedAt)
	}
}

func TestTaskToDTOWithoutOptionalFields(t *testing.T) {
	now := time.Date(2026, 3, 23, 14, 30, 0, 0, time.UTC)
	task := &model.Task{
		ID:        uuid.New(),
		ProjectID: uuid.New(),
		Title:     "Minimal task",
		Status:    model.TaskStatusInbox,
		Priority:  "low",
		CreatedAt: now,
		UpdatedAt: now,
	}

	dto := task.ToDTO()

	if dto.ParentID != nil || dto.SprintID != nil || dto.AssigneeID != nil || dto.ReporterID != nil || dto.CompletedAt != nil {
		t.Fatalf("expected optional fields to stay nil, got %+v", dto)
	}
	if dto.Title != "Minimal task" || dto.Status != model.TaskStatusInbox || dto.Priority != "low" {
		t.Fatalf("unexpected base fields in DTO: %+v", dto)
	}
}
