package repository

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestWorkflowPluginRunRepository_CreateUpdateListAndClone(t *testing.T) {
	ctx := context.Background()
	repo := NewWorkflowPluginRunRepository()
	now := time.Date(2026, 3, 30, 17, 30, 0, 0, time.UTC)
	completedAt := now.Add(time.Minute)

	run := &model.WorkflowPluginRun{
		ID:       uuid.New(),
		PluginID: "workflow.release-train",
		Process:  model.WorkflowProcessSequential,
		Status:   model.WorkflowRunStatusRunning,
		Trigger: map[string]any{
			"projectId": "project-1",
			"nested":    map[string]any{"labels": []any{"ops", map[string]any{"key": "value"}}},
		},
		Steps: []model.WorkflowStepRun{{
			StepID:      "build",
			RoleID:      "coder",
			Action:      model.WorkflowActionAgent,
			Status:      model.WorkflowStepRunStatusCompleted,
			Input:       map[string]any{"taskId": "task-1"},
			Output:      map[string]any{"status": "ok"},
			StartedAt:   &now,
			CompletedAt: &completedAt,
			Attempts: []model.WorkflowStepAttempt{{
				Attempt:     1,
				Status:      model.WorkflowStepRunStatusCompleted,
				Output:      map[string]any{"status": "ok"},
				StartedAt:   now,
				CompletedAt: &completedAt,
			}},
		}},
		StartedAt:   now,
		CompletedAt: &completedAt,
	}

	if err := repo.Create(ctx, run); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	loaded, err := repo.GetByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	loaded.Trigger["nested"].(map[string]any)["labels"].([]any)[1].(map[string]any)["key"] = "mutated"
	loaded.Steps[0].Output["status"] = "changed"

	reloaded, err := repo.GetByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetByID() reload error = %v", err)
	}
	if reloaded.Trigger["nested"].(map[string]any)["labels"].([]any)[1].(map[string]any)["key"] != "value" {
		t.Fatal("expected stored trigger payload to remain cloned")
	}
	if reloaded.Steps[0].Output["status"] != "ok" {
		t.Fatal("expected stored step output to remain cloned")
	}

	run.Status = model.WorkflowRunStatusCompleted
	if err := repo.Update(ctx, run); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	other := &model.WorkflowPluginRun{ID: uuid.New(), PluginID: run.PluginID, StartedAt: now.Add(2 * time.Hour)}
	if err := repo.Create(ctx, other); err != nil {
		t.Fatalf("Create(other) error = %v", err)
	}

	listed, err := repo.ListByPluginID(ctx, run.PluginID, 1)
	if err != nil {
		t.Fatalf("ListByPluginID() error = %v", err)
	}
	if len(listed) != 1 || listed[0].ID != other.ID {
		t.Fatalf("ListByPluginID(limit=1) = %#v", listed)
	}
}

func TestWorkflowPluginRunRepository_ErrorPathsAndCloneMap(t *testing.T) {
	ctx := context.Background()
	repo := NewWorkflowPluginRunRepository()

	if err := repo.Create(ctx, nil); err == nil {
		t.Fatal("Create(nil) expected error")
	}
	if err := repo.Update(ctx, nil); err == nil {
		t.Fatal("Update(nil) expected error")
	}
	if _, err := repo.GetByID(ctx, uuid.New()); err == nil {
		t.Fatal("GetByID(missing) expected error")
	}
	if err := repo.Update(ctx, &model.WorkflowPluginRun{ID: uuid.New()}); err == nil {
		t.Fatal("Update(missing) expected error")
	}

	clone := cloneMap(map[string]any{
		"nested": map[string]any{"count": 2},
		"list":   []any{map[string]any{"key": "value"}, "done"},
	})
	clone["nested"].(map[string]any)["count"] = 3
	clone["list"].([]any)[0].(map[string]any)["key"] = "changed"
	if cloneMap(nil) != nil {
		t.Fatal("cloneMap(nil) should return nil")
	}
}
