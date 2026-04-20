package repository_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestNewWorkflowRunParentLinkRepository_Constructs(t *testing.T) {
	repo := repository.NewWorkflowRunParentLinkRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil WorkflowRunParentLinkRepository")
	}
}

func TestWorkflowRunParentLinkRepository_Create_NilDB(t *testing.T) {
	repo := repository.NewWorkflowRunParentLinkRepository(nil)
	err := repo.Create(context.Background(), &model.WorkflowRunParentLink{
		ID:                uuid.New(),
		ParentExecutionID: uuid.New(),
		ParentNodeID:      "node-a",
		ChildEngineKind:   model.SubWorkflowEngineDAG,
		ChildRunID:        uuid.New(),
	})
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowRunParentLinkRepository_GetByParent_NilDB(t *testing.T) {
	repo := repository.NewWorkflowRunParentLinkRepository(nil)
	_, err := repo.GetByParent(context.Background(), uuid.New(), "node-a")
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowRunParentLinkRepository_GetByChild_NilDB(t *testing.T) {
	repo := repository.NewWorkflowRunParentLinkRepository(nil)
	_, err := repo.GetByChild(context.Background(), model.SubWorkflowEngineDAG, uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowRunParentLinkRepository_ListByParentExecution_NilDB(t *testing.T) {
	repo := repository.NewWorkflowRunParentLinkRepository(nil)
	_, err := repo.ListByParentExecution(context.Background(), uuid.New())
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestWorkflowRunParentLinkRepository_UpdateStatus_NilDB(t *testing.T) {
	repo := repository.NewWorkflowRunParentLinkRepository(nil)
	err := repo.UpdateStatus(context.Background(), uuid.New(), model.SubWorkflowLinkStatusCompleted)
	if err != repository.ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}
