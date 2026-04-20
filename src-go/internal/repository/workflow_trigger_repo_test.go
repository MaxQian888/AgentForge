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

func TestWorkflowTriggerRepository_Create_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	wfID := uuid.New()
	err := repo.Create(context.Background(), &model.WorkflowTrigger{
		WorkflowID:  &wfID,
		ProjectID:   uuid.New(),
		Source:      model.TriggerSourceIM,
		TargetKind:  model.TriggerTargetDAG,
		Config:      []byte(`{}`),
		CreatedVia:  model.TriggerCreatedViaManual,
		DisplayName: "manual row",
	})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowTriggerRepository_GetByID_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowTriggerRepository_Update_NilDB(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	err := repo.Update(context.Background(), &model.WorkflowTrigger{
		ID:     uuid.New(),
		Config: []byte(`{}`),
	})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

// TestWorkflowTriggerRepository_Update_MissingID asserts the precondition that
// callers must supply a populated ID. Exercised even without a DB.
func TestWorkflowTriggerRepository_Update_MissingID(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	err := repo.Update(context.Background(), &model.WorkflowTrigger{Config: []byte(`{}`)})
	if err == nil {
		t.Fatal("expected error for missing ID")
	}
}

// TestWorkflowTriggerRepository_Create_NilTrigger guards the nil-input branch.
func TestWorkflowTriggerRepository_Create_NilTrigger(t *testing.T) {
	repo := repository.NewWorkflowTriggerRepository(nil)
	err := repo.Create(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil trigger")
	}
}
