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
	err := repo.Upsert(context.Background(), &model.WorkflowTrigger{
		ID:         uuid.New(),
		WorkflowID: uuid.New(),
		ProjectID:  uuid.New(),
		Source:     model.TriggerSourceIM,
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
