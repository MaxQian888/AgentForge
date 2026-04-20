package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentforge/server/internal/handler"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
)

type mockBudgetQueryService struct {
	projectSummary *model.ProjectBudgetSummary
	projectErr     error
	sprintDetail   *model.SprintBudgetDetail
	sprintErr      error
	lastProjectID  uuid.UUID
	lastSprintID   uuid.UUID
}

func (m *mockBudgetQueryService) GetProjectBudgetSummary(_ context.Context, projectID uuid.UUID) (*model.ProjectBudgetSummary, error) {
	m.lastProjectID = projectID
	return m.projectSummary, m.projectErr
}

func (m *mockBudgetQueryService) GetSprintBudgetDetail(_ context.Context, sprintID uuid.UUID) (*model.SprintBudgetDetail, error) {
	m.lastSprintID = sprintID
	return m.sprintDetail, m.sprintErr
}

func TestBudgetQueryHandler_ProjectSummaryReturnsPayload(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	h := handler.NewBudgetQueryHandler(&mockBudgetQueryService{
		projectSummary: &model.ProjectBudgetSummary{
			ProjectID:       projectID.String(),
			Allocated:       25,
			Spent:           12.5,
			Remaining:       12.5,
			ThresholdStatus: model.BudgetThresholdHealthy,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/budget/summary", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	if err := h.ProjectSummary(c); err != nil {
		t.Fatalf("ProjectSummary() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var response model.ProjectBudgetSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.ProjectID != projectID.String() {
		t.Fatalf("ProjectID = %q, want %q", response.ProjectID, projectID.String())
	}
}

func TestBudgetQueryHandler_SprintDetailMapsNotFound(t *testing.T) {
	e := newAgentTestEcho()
	sprintID := uuid.New()
	h := handler.NewBudgetQueryHandler(&mockBudgetQueryService{
		sprintErr: service.ErrBudgetSprintNotFound,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sprints/"+sprintID.String()+"/budget", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("sid")
	c.SetParamValues(sprintID.String())

	if err := h.SprintDetail(c); err != nil {
		t.Fatalf("SprintDetail() error = %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestBudgetQueryHandler_SprintDetailReturnsPayload(t *testing.T) {
	e := newAgentTestEcho()
	sprintID := uuid.New()
	projectID := uuid.New()
	h := handler.NewBudgetQueryHandler(&mockBudgetQueryService{
		sprintDetail: &model.SprintBudgetDetail{
			SprintID:        sprintID.String(),
			ProjectID:       projectID.String(),
			SprintName:      "Realtime sprint",
			Allocated:       12,
			Spent:           9.6,
			Remaining:       2.4,
			ThresholdStatus: model.BudgetThresholdWarning,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/sprints/"+sprintID.String()+"/budget", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("sid")
	c.SetParamValues(sprintID.String())

	if err := h.SprintDetail(c); err != nil {
		t.Fatalf("SprintDetail() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var response model.SprintBudgetDetail
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.SprintID != sprintID.String() || response.ProjectID != projectID.String() {
		t.Fatalf("response = %+v", response)
	}
}

func TestBudgetQueryHandler_ProtectedRoutesRequireAuth(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewBudgetQueryHandler(&mockBudgetQueryService{})
	jwtMw := appMiddleware.JWTMiddleware("test-secret", noopBlacklist{})

	e.GET("/api/v1/projects/:pid/budget/summary", h.ProjectSummary, jwtMw)
	e.GET("/api/v1/sprints/:sid/budget", h.SprintDetail, jwtMw)

	for _, tc := range []struct {
		name string
		path string
	}{
		{name: "project summary", path: "/api/v1/projects/" + uuid.NewString() + "/budget/summary"},
		{name: "sprint detail", path: "/api/v1/sprints/" + uuid.NewString() + "/budget"},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", tc.name, rec.Code)
		}
	}
}
