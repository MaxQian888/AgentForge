package repository

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestNewAgentRunRepository(t *testing.T) {
	repo := NewAgentRunRepository(nil)
	if repo == nil {
		t.Fatal("expected non-nil AgentRunRepository")
	}
}

func TestAgentRunRepositoryCreateNilDB(t *testing.T) {
	repo := NewAgentRunRepository(nil)
	err := repo.Create(context.Background(), &model.AgentRun{ID: uuid.New(), TaskID: uuid.New(), MemberID: uuid.New()})
	if err != ErrDatabaseUnavailable {
		t.Fatalf("Create() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentRunRepositoryGetByIDNilDB(t *testing.T) {
	repo := NewAgentRunRepository(nil)
	_, err := repo.GetByID(context.Background(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Fatalf("GetByID() error = %v, want %v", err, ErrDatabaseUnavailable)
	}
}

func TestAgentRunRepository_SetEmployeeID_NilDB(t *testing.T) {
	repo := NewAgentRunRepository(nil)
	err := repo.SetEmployeeID(context.Background(), uuid.New(), uuid.New())
	if err != ErrDatabaseUnavailable {
		t.Errorf("expected ErrDatabaseUnavailable, got %v", err)
	}
}

func TestAgentRunRecordPreservesRoleAndTeamFields(t *testing.T) {
	teamID := uuid.New()
	emp := uuid.New()
	now := time.Now().UTC()

	run := &model.AgentRun{
		ID:         uuid.New(),
		TaskID:     uuid.New(),
		MemberID:   uuid.New(),
		RoleID:     "frontend-developer",
		Status:     model.AgentRunStatusRunning,
		Runtime:    "codex",
		Provider:   "openai",
		Model:      "gpt-5.4",
		TeamID:     &teamID,
		TeamRole:   model.TeamRoleCoder,
		EmployeeID: &emp,
		StartedAt:  now,
	}

	record := newAgentRunRecord(run)
	result := record.toModel()

	if result.RoleID != "frontend-developer" {
		t.Fatalf("RoleID = %q, want frontend-developer", result.RoleID)
	}
	if result.TeamID == nil || *result.TeamID != teamID {
		t.Fatalf("TeamID = %v, want %s", result.TeamID, teamID)
	}
	if result.TeamRole != model.TeamRoleCoder {
		t.Fatalf("TeamRole = %q, want %q", result.TeamRole, model.TeamRoleCoder)
	}
	if result.EmployeeID == nil || *result.EmployeeID != emp {
		t.Errorf("EmployeeID: expected %v, got %v", emp, result.EmployeeID)
	}
}
