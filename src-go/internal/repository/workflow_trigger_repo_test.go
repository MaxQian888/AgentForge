package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestNewWorkflowTriggerRepository_Constructs(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil WorkflowTriggerRepository")
	}
}

func TestWorkflowTriggerRepository_Upsert_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	wfID := uuid.New()
	err := repo.Upsert(context.Background(), &model.WorkflowTrigger{
		ID:         uuid.New(),
		WorkflowID: &wfID,
		ProjectID:  uuid.New(),
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetDAG,
		Config:     []byte(`{"key":"value"}`),
	})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowTriggerRepository_ListEnabledBySource_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	_, err := repo.ListEnabledBySource(context.Background(), model.TriggerSourceIM)
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowTriggerRepository_ListEnabledBySourceAndKind_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	_, err := repo.ListEnabledBySourceAndKind(context.Background(),
		model.TriggerSourceIM, model.TriggerTargetPlugin)
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowTriggerRepository_Upsert_PluginTargetNilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	err := repo.Upsert(context.Background(), &model.WorkflowTrigger{
		ID:         uuid.New(),
		PluginID:   "workflow-plugin-x",
		ProjectID:  uuid.New(),
		Source:     model.TriggerSourceIM,
		TargetKind: model.TriggerTargetPlugin,
		Config:     []byte(`{"key":"value"}`),
	})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowTriggerRepository_ListByWorkflow_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	_, err := repo.ListByWorkflow(context.Background(), uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowTriggerRepository_SetEnabled_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	err := repo.SetEnabled(context.Background(), uuid.New(), true)
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowTriggerRepository_Delete_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	err := repo.Delete(context.Background(), uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}
