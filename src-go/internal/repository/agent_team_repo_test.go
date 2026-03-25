package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

func TestNewAgentTeamRepository(t *testing.T) {
	repo := NewAgentTeamRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil AgentTeamRepository")
	}
}

func TestAgentTeamRepositoryCreateNilDB(t *testing.T) {
	repo := NewAgentTeamRepository(nil)
	err := repo.Create(context.Background(), &model.AgentTeam{ID: uuid.New(), ProjectID: uuid.New(), TaskID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentTeamRepositoryGetByIDNilDB(t *testing.T) {
	repo := NewAgentTeamRepository(nil)
	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetByID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentTeamRecordPreservesPlannerAndReviewerRuns(t *testing.T) {
	plannerRunID := uuid.New()
	reviewerRunID := uuid.New()

	team := &model.AgentTeam{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		TaskID:        uuid.New(),
		Name:          "Task Delivery Team",
		Status:        model.TeamStatusExecuting,
		Strategy:      "plan-code-review",
		PlannerRunID:  &plannerRunID,
		ReviewerRunID: &reviewerRunID,
		Config:        `{"runtime":"codex","provider":"openai","model":"gpt-5.4"}`,
	}

	record := newAgentTeamRecord(team)
	result := record.toModel()

	if result.PlannerRunID == nil || *result.PlannerRunID != plannerRunID {
		t.Fatalf("PlannerRunID = %v, want %s", result.PlannerRunID, plannerRunID)
	}
	if result.ReviewerRunID == nil || *result.ReviewerRunID != reviewerRunID {
		t.Fatalf("ReviewerRunID = %v, want %s", result.ReviewerRunID, reviewerRunID)
	}
}
