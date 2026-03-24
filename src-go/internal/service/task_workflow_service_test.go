package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type fakeWorkflowConfigRepo struct {
	config *model.WorkflowConfig
	err    error
}

func (f *fakeWorkflowConfigRepo) GetByProject(_ context.Context, _ uuid.UUID) (*model.WorkflowConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.config, nil
}

func TestTaskWorkflowService_EvaluateTransition_NoConfig(t *testing.T) {
	svc := service.NewTaskWorkflowService(&fakeWorkflowConfigRepo{err: errors.New("not found")}, nil)
	task := &model.Task{ID: uuid.New(), ProjectID: uuid.New(), Status: "assigned"}
	results := svc.EvaluateTransition(context.Background(), task, "triaged", "assigned")
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func TestTaskWorkflowService_EvaluateTransition_MatchesTrigger(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{FromStatus: "triaged", ToStatus: "assigned", Action: "notify"},
		{FromStatus: "assigned", ToStatus: "in_progress", Action: "auto_assign_agent"},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}

	svc := service.NewTaskWorkflowService(repo, nil)
	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Status: "assigned"}

	results := svc.EvaluateTransition(context.Background(), task, "triaged", "assigned")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Fired {
		t.Fatal("expected trigger to fire")
	}
	if results[0].Trigger.Action != "notify" {
		t.Fatalf("action = %q, want 'notify'", results[0].Trigger.Action)
	}
}

func TestTaskWorkflowService_EvaluateTransition_NoMatch(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{FromStatus: "in_progress", ToStatus: "done", Action: "notify"},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}

	svc := service.NewTaskWorkflowService(repo, nil)
	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Status: "assigned"}

	results := svc.EvaluateTransition(context.Background(), task, "triaged", "assigned")
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func TestTaskWorkflowService_EvaluateTransition_NilTask(t *testing.T) {
	svc := service.NewTaskWorkflowService(&fakeWorkflowConfigRepo{}, nil)
	results := svc.EvaluateTransition(context.Background(), nil, "a", "b")
	if results != nil {
		t.Fatalf("expected nil results, got %v", results)
	}
}
