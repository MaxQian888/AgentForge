package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type mockTeamRuntimeService struct {
	summary       *model.AgentTeamSummaryDTO
	summaries     []*model.AgentTeamSummaryDTO
	teams         []*model.AgentTeam
	getSummaryErr error
	listErr       error
}

func (m *mockTeamRuntimeService) StartTeam(_ context.Context, _ service.StartTeamInput) (*model.AgentTeam, error) {
	return nil, nil
}

func (m *mockTeamRuntimeService) GetSummary(_ context.Context, _ uuid.UUID) (*model.AgentTeamSummaryDTO, error) {
	if m.getSummaryErr != nil {
		return nil, m.getSummaryErr
	}
	if m.summary != nil {
		return m.summary, nil
	}
	if len(m.summaries) > 0 {
		return m.summaries[0], nil
	}
	return nil, service.ErrTeamNotFound
}

func (m *mockTeamRuntimeService) ListByProject(_ context.Context, _ uuid.UUID, _ string) ([]*model.AgentTeam, error) {
	return m.teams, m.listErr
}

func (m *mockTeamRuntimeService) ListSummaries(_ context.Context, _ uuid.UUID, _ string) ([]*model.AgentTeamSummaryDTO, error) {
	return m.summaries, m.listErr
}

func (m *mockTeamRuntimeService) CancelTeam(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockTeamRuntimeService) RetryTeam(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockTeamRuntimeService) DeleteTeam(_ context.Context, _ uuid.UUID) error {
	return nil
}

func (m *mockTeamRuntimeService) UpdateTeam(_ context.Context, _ uuid.UUID, _ *model.UpdateTeamRequest) (*model.AgentTeam, error) {
	return nil, nil
}

func (m *mockTeamRuntimeService) ListArtifacts(_ context.Context, _ uuid.UUID) ([]model.TeamArtifactDTO, error) {
	return []model.TeamArtifactDTO{}, nil
}

func TestTeamHandlerListReturnsSummaryDTOs(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	teamID := uuid.New()
	now := time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)

	summary := &model.AgentTeamSummaryDTO{
		AgentTeamDTO: model.AgentTeamDTO{
			ID:             teamID.String(),
			ProjectID:      projectID.String(),
			TaskID:         taskID.String(),
			Name:           "Review queue team",
			Status:         model.TeamStatusExecuting,
			Strategy:       "plan-code-review",
			TotalBudgetUsd: 25,
			TotalSpentUsd:  11.5,
			CreatedAt:      now.Format(time.RFC3339),
			UpdatedAt:      now.Format(time.RFC3339),
		},
		TaskTitle:      "Review queue",
		PlannerStatus:  model.AgentRunStatusCompleted,
		ReviewerStatus: model.AgentRunStatusRunning,
		CoderRuns: []model.AgentRunDTO{
			{
				ID:        uuid.NewString(),
				TaskID:    taskID.String(),
				MemberID:  uuid.NewString(),
				Status:    model.AgentRunStatusCompleted,
				CreatedAt: now.Format(time.RFC3339),
				StartedAt: now.Format(time.RFC3339),
			},
		},
		CoderTotal:     1,
		CoderCompleted: 1,
	}

	mockService := &mockTeamRuntimeService{
		summaries: []*model.AgentTeamSummaryDTO{summary},
		teams: []*model.AgentTeam{
			{
				ID:        teamID,
				ProjectID: projectID,
				TaskID:    taskID,
				Name:      "Review queue team",
				Status:    model.TeamStatusExecuting,
				Strategy:  "plan-code-review",
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/teams?projectId="+projectID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handler.NewTeamHandler(mockService)
	if err := h.List(c); err != nil {
		t.Fatalf("List() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var response []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(response) != 1 {
		t.Fatalf("len(response) = %d, want 1", len(response))
	}
	if response[0]["taskTitle"] != "Review queue" {
		t.Fatalf("taskTitle = %#v, want Review queue", response[0]["taskTitle"])
	}
	if response[0]["plannerStatus"] != model.AgentRunStatusCompleted {
		t.Fatalf("plannerStatus = %#v, want %q", response[0]["plannerStatus"], model.AgentRunStatusCompleted)
	}
	if response[0]["reviewerStatus"] != model.AgentRunStatusRunning {
		t.Fatalf("reviewerStatus = %#v, want %q", response[0]["reviewerStatus"], model.AgentRunStatusRunning)
	}
}
