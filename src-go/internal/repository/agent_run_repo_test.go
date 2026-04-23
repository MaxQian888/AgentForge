package repository

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

func TestAgentRunRepositoryCreateIfNoActiveByTaskRejectsConflict(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&taskRecord{}, &agentRunRecord{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	taskID := uuid.New()
	projectID := uuid.New()
	now := time.Now().UTC()
	if err := db.Create(&taskRecord{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn lock",
		Description: "Ensure only one active run can exist",
		Status:      "todo",
		Priority:    "medium",
		CreatedAt:   now,
		UpdatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("seed task: %v", err)
	}

	repo := NewAgentRunRepository(db)
	active := &model.AgentRun{
		ID:        uuid.New(),
		TaskID:    taskID,
		MemberID:  uuid.New(),
		Status:    model.AgentRunStatusRunning,
		Runtime:   "codex",
		Provider:  "openai",
		Model:     "gpt-5",
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := repo.Create(context.Background(), active); err != nil {
		t.Fatalf("seed active run: %v", err)
	}

	next := &model.AgentRun{
		ID:        uuid.New(),
		TaskID:    taskID,
		MemberID:  uuid.New(),
		Status:    model.AgentRunStatusStarting,
		Runtime:   "codex",
		Provider:  "openai",
		Model:     "gpt-5",
		StartedAt: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
	err = repo.CreateIfNoActiveByTask(context.Background(), next)
	if err != ErrAgentRunActiveConflict {
		t.Fatalf("CreateIfNoActiveByTask() error = %v, want %v", err, ErrAgentRunActiveConflict)
	}
}
