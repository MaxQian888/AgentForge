package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

func TestAgentRunRepository_CreatePersistsRoleID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewAgentRunRepository(mock)
	run := &model.AgentRun{
		ID:        uuid.New(),
		TaskID:    uuid.New(),
		MemberID:  uuid.New(),
		RoleID:    "frontend-developer",
		Status:    model.AgentRunStatusStarting,
		Provider:  "anthropic",
		Model:     "claude-sonnet",
		StartedAt: time.Now().UTC(),
	}

	mock.ExpectExec("INSERT INTO agent_runs").
		WithArgs(
			run.ID,
			run.TaskID,
			run.MemberID,
			run.RoleID,
			run.Status,
			run.Provider,
			run.Model,
			run.InputTokens,
			run.OutputTokens,
			run.CacheReadTokens,
			run.CostUsd,
			run.TurnCount,
			run.ErrorMessage,
			run.StartedAt,
			run.CompletedAt,
			run.TeamID,
			run.TeamRole,
		).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	if err := repo.Create(context.Background(), run); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}

func TestAgentRunRepository_GetByIDScansRoleID(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatal(err)
	}
	defer mock.Close()

	repo := repository.NewAgentRunRepository(mock)
	runID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	startedAt := time.Now().UTC()
	createdAt := startedAt.Add(time.Minute)
	updatedAt := createdAt.Add(time.Minute)
	roleID := "frontend-developer"

	rows := pgxmock.NewRows([]string{
		"id", "task_id", "member_id", "role_id", "status", "provider", "model",
		"input_tokens", "output_tokens", "cache_read_tokens", "cost_usd", "turn_count",
		"error_message", "started_at", "completed_at", "created_at", "updated_at",
		"team_id", "team_role",
	}).AddRow(
		runID, taskID, memberID, roleID, model.AgentRunStatusRunning, "anthropic", "claude-sonnet",
		int64(10), int64(12), int64(0), 0.42, 3,
		"", startedAt, nil, createdAt, updatedAt,
		nil, "",
	)

	mock.ExpectQuery("SELECT id, task_id, member_id, role_id, status, provider, model").
		WithArgs(runID).
		WillReturnRows(rows)

	run, err := repo.GetByID(context.Background(), runID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if run.RoleID != roleID {
		t.Fatalf("run.RoleID = %q, want %q", run.RoleID, roleID)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet expectations: %v", err)
	}
}
