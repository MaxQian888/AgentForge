package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
	"gorm.io/gorm"
)

type mockCostStatsService struct {
	projectSummary *model.ProjectCostSummaryDTO
	projectErr     error
	sprintSummary  *model.CostSummaryDTO
	sprintErr      error
	activeSummary  *model.CostSummaryDTO
	activeErr      error
}

func (m *mockCostStatsService) ProjectSummary(_ context.Context, _ uuid.UUID) (*model.ProjectCostSummaryDTO, error) {
	return m.projectSummary, m.projectErr
}

func (m *mockCostStatsService) SprintSummary(_ context.Context, _ uuid.UUID) (*model.CostSummaryDTO, error) {
	return m.sprintSummary, m.sprintErr
}

func (m *mockCostStatsService) ActiveSummary(_ context.Context) (*model.CostSummaryDTO, error) {
	return m.activeSummary, m.activeErr
}

func TestCostHandler_GetStats_Default(t *testing.T) {
	h := handler.NewCostHandler(&mockCostStatsService{activeErr: repository.ErrDatabaseUnavailable})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/cost", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetStats(c); err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	// With nil db, should get 500
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestCostHandler_GetStats_InvalidProjectId(t *testing.T) {
	h := handler.NewCostHandler(&mockCostStatsService{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/cost?projectId=invalid", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetStats(c); err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body model.ErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Message != "invalid projectId" {
		t.Fatalf("message = %q", body.Message)
	}
}

func TestCostHandler_GetStats_InvalidSprintId(t *testing.T) {
	h := handler.NewCostHandler(&mockCostStatsService{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/cost?sprintId=nope", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetStats(c); err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestCostHandler_GetStats_ProjectSummaryIncludesAuthoritativeBreakdowns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	for _, stmt := range []string{
		`CREATE TABLE tasks (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			sprint_id TEXT,
			title TEXT,
			status TEXT,
			priority TEXT,
			budget_usd REAL,
			spent_usd REAL,
			created_at DATETIME,
			updated_at DATETIME,
			completed_at DATETIME
		)`,
		`CREATE TABLE task_progress_snapshots (
			task_id TEXT PRIMARY KEY,
			last_activity_at DATETIME,
			last_activity_source TEXT,
			last_transition_at DATETIME,
			health_status TEXT,
			risk_reason TEXT,
			risk_since_at DATETIME,
			last_alert_state TEXT,
			last_alert_at DATETIME,
			last_recovered_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE sprints (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			name TEXT,
			start_date DATETIME,
			end_date DATETIME,
			status TEXT,
			total_budget_usd REAL,
			spent_usd REAL,
			created_at DATETIME,
			updated_at DATETIME
		)`,
		`CREATE TABLE agent_runs (
			id TEXT PRIMARY KEY,
			task_id TEXT NOT NULL,
			member_id TEXT NOT NULL,
			role_id TEXT,
			status TEXT,
			runtime TEXT,
			provider TEXT,
			model TEXT,
			input_tokens INTEGER,
			output_tokens INTEGER,
			cache_read_tokens INTEGER,
			cost_usd REAL,
			turn_count INTEGER,
			error_message TEXT,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME,
			updated_at DATETIME,
			team_id TEXT,
			team_role TEXT,
			cost_accounting JSON
		)`,
	} {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("exec schema %q: %v", stmt, err)
		}
	}

	projectID := uuid.New()
	sprintID := uuid.New()
	taskOneID := uuid.New()
	taskTwoID := uuid.New()
	memberID := uuid.New()
	now := time.Date(2026, 3, 30, 12, 0, 0, 0, time.UTC)
	yesterday := now.Add(-24 * time.Hour)
	completedAt := now.Add(-2 * time.Hour)

	if err := db.Exec(
		`INSERT INTO sprints (id, project_id, name, start_date, end_date, status, total_budget_usd, spent_usd, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sprintID.String(), projectID.String(), "Sprint 7", now.AddDate(0, 0, -7), now.AddDate(0, 0, 7), model.SprintStatusActive, 20, 6.5, now, now,
	).Error; err != nil {
		t.Fatalf("insert sprint: %v", err)
	}

	for _, args := range [][]any{
		{taskOneID.String(), projectID.String(), sprintID.String(), "Ship truthful summary", model.TaskStatusDone, "high", 8.0, 2.5, yesterday, now, completedAt},
		{taskTwoID.String(), projectID.String(), sprintID.String(), "Align workspace copy", model.TaskStatusInProgress, "medium", 5.0, 4.0, yesterday, now, nil},
	} {
		if err := db.Exec(
			`INSERT INTO tasks (id, project_id, sprint_id, title, status, priority, budget_usd, spent_usd, created_at, updated_at, completed_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			args...,
		).Error; err != nil {
			t.Fatalf("insert task %v: %v", args[3], err)
		}
	}

	for _, args := range [][]any{
		{uuid.New().String(), taskOneID.String(), memberID.String(), "planner", model.AgentRunStatusCompleted, "claude_code", "anthropic", "claude-sonnet-4-6", 120, 40, 10, 2.5, 3, "", yesterday, completedAt, yesterday, completedAt, nil, ""},
		{uuid.New().String(), taskTwoID.String(), memberID.String(), "reviewer", model.AgentRunStatusRunning, "codex", "openai", "gpt-5.2", 200, 75, 15, 4.0, 5, "", now.Add(-90 * time.Minute), nil, now, now, nil, ""},
	} {
		if err := db.Exec(
			`INSERT INTO agent_runs (id, task_id, member_id, role_id, status, runtime, provider, model, input_tokens, output_tokens, cache_read_tokens, cost_usd, turn_count, error_message, started_at, completed_at, created_at, updated_at, team_id, team_role)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			args...,
		).Error; err != nil {
			t.Fatalf("insert run %v: %v", args[3], err)
		}
	}

	taskRepo := repository.NewTaskRepository(db)
	sprintRepo := repository.NewSprintRepository(db)
	runRepo := repository.NewAgentRunRepository(db)
	budgetSvc := service.NewBudgetGovernanceService(sprintRepo, taskRepo)
	costSvc := service.NewCostQueryService(taskRepo, sprintRepo, runRepo, budgetSvc)
	h := handler.NewCostHandler(costSvc)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/cost?projectId="+projectID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.GetStats(c); err != nil {
		t.Fatalf("GetStats() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, key := range []string{"activeAgents", "dailyCosts", "sprintCosts", "taskCosts", "budgetSummary", "periodRollups", "costCoverage", "runtimeBreakdown"} {
		if _, ok := body[key]; !ok {
			t.Fatalf("response missing %q: %s", key, rec.Body.String())
		}
	}
}
