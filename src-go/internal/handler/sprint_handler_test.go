package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeSprintRepo struct {
	sprint    *model.Sprint
	err       error
	updateErr error
	created   *model.Sprint
	updated   *model.Sprint
}

func (f *fakeSprintRepo) Create(_ context.Context, sprint *model.Sprint) error {
	f.created = sprint
	return nil
}

func (f *fakeSprintRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Sprint, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.sprint == nil || f.sprint.ID != id {
		return nil, errors.New("sprint not found")
	}
	return f.sprint, nil
}

func (f *fakeSprintRepo) ListByProject(context.Context, uuid.UUID) ([]*model.Sprint, error) {
	return nil, nil
}

func (f *fakeSprintRepo) Update(_ context.Context, sprint *model.Sprint) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updated = sprint
	return nil
}

type fakeSprintTaskRepo struct {
	tasks []*model.Task
	err   error
}

func (f *fakeSprintTaskRepo) ListBySprint(context.Context, uuid.UUID, uuid.UUID) ([]*model.Task, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.tasks, nil
}

func TestSprintHandler_MetricsReturnsDerivedSummary(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	startDate := time.Date(2026, time.March, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2026, time.March, 7, 23, 59, 59, 0, time.UTC)
	doneAt1 := time.Date(2026, time.March, 2, 12, 0, 0, 0, time.UTC)
	doneAt2 := time.Date(2026, time.March, 5, 18, 30, 0, 0, time.UTC)

	sprintRepo := &fakeSprintRepo{
		sprint: &model.Sprint{
			ID:             sprintID,
			ProjectID:      projectID,
			Name:           "Sprint 12",
			StartDate:      startDate,
			EndDate:        endDate,
			Status:         model.SprintStatusClosed,
			TotalBudgetUsd: 20,
			SpentUsd:       8,
			CreatedAt:      startDate.Add(-24 * time.Hour),
			UpdatedAt:      endDate,
		},
	}
	taskRepo := &fakeSprintTaskRepo{
		tasks: []*model.Task{
			{
				ID:          uuid.New(),
				ProjectID:   projectID,
				SprintID:    &sprintID,
				Title:       "Ship board polish",
				Status:      model.TaskStatusDone,
				Priority:    "high",
				BudgetUsd:   5,
				SpentUsd:    4,
				CreatedAt:   startDate,
				UpdatedAt:   doneAt1,
				CompletedAt: &doneAt1,
			},
			{
				ID:        uuid.New(),
				ProjectID: projectID,
				SprintID:  &sprintID,
				Title:     "Finish sprint dashboard",
				Status:    model.TaskStatusInProgress,
				Priority:  "high",
				BudgetUsd: 8,
				SpentUsd:  3,
				CreatedAt: startDate,
				UpdatedAt: endDate,
			},
			{
				ID:          uuid.New(),
				ProjectID:   projectID,
				SprintID:    &sprintID,
				Title:       "Close burndown gaps",
				Status:      model.TaskStatusDone,
				Priority:    "medium",
				BudgetUsd:   3,
				SpentUsd:    2.5,
				CreatedAt:   startDate,
				UpdatedAt:   doneAt2,
				CompletedAt: &doneAt2,
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/sprints/"+sprintID.String()+"/metrics", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:pid/sprints/:sid/metrics")
	c.SetParamNames("pid", "sid")
	c.SetParamValues(projectID.String(), sprintID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewSprintHandler(sprintRepo, taskRepo)
	if err := h.Metrics(c); err != nil {
		t.Fatalf("Metrics() error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body model.SprintMetricsDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body.Sprint.ID != sprintID.String() {
		t.Fatalf("sprint.id = %q, want %q", body.Sprint.ID, sprintID.String())
	}
	if body.PlannedTasks != 3 || body.CompletedTasks != 2 || body.RemainingTasks != 1 {
		t.Fatalf("summary = %+v", body)
	}
	if math.Abs(body.CompletionRate-66.67) > 0.01 {
		t.Fatalf("completionRate = %.2f, want about 66.67", body.CompletionRate)
	}
	if math.Abs(body.VelocityPerWeek-2) > 0.01 {
		t.Fatalf("velocityPerWeek = %.2f, want about 2.00", body.VelocityPerWeek)
	}
	if math.Abs(body.TaskBudgetUsd-16) > 0.01 || math.Abs(body.TaskSpentUsd-9.5) > 0.01 {
		t.Fatalf("budget metrics = %+v", body)
	}
	if len(body.Burndown) != 7 {
		t.Fatalf("burndown length = %d, want 7", len(body.Burndown))
	}
	lastPoint := body.Burndown[len(body.Burndown)-1]
	if lastPoint.RemainingTasks != 1 || lastPoint.CompletedTasks != 2 {
		t.Fatalf("last burndown point = %+v", lastPoint)
	}
}

func TestSprintHandler_Update_FieldsOnly(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	repo := &fakeSprintRepo{
		sprint: &model.Sprint{
			ID:             sprintID,
			ProjectID:      projectID,
			Name:           "Sprint 1",
			StartDate:      time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			EndDate:        time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
			Status:         model.SprintStatusPlanning,
			TotalBudgetUsd: 100,
		},
	}

	body := `{"name":"Sprint 1 Renamed","totalBudgetUsd":200}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:pid/sprints/:sid")
	c.SetParamNames("pid", "sid")
	c.SetParamValues(projectID.String(), sprintID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewSprintHandler(repo)
	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var dto model.SprintDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dto.Name != "Sprint 1 Renamed" {
		t.Fatalf("name = %q, want %q", dto.Name, "Sprint 1 Renamed")
	}
	if dto.Status != model.SprintStatusPlanning {
		t.Fatalf("status = %q, want %q", dto.Status, model.SprintStatusPlanning)
	}
	if repo.updated.TotalBudgetUsd != 200 {
		t.Fatalf("budget = %.2f, want 200", repo.updated.TotalBudgetUsd)
	}
}

func TestSprintHandler_Update_TransitionValid(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	repo := &fakeSprintRepo{
		sprint: &model.Sprint{
			ID:        sprintID,
			ProjectID: projectID,
			Name:      "Sprint 2",
			StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
			Status:    model.SprintStatusPlanning,
		},
	}

	body := `{"status":"active"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:pid/sprints/:sid")
	c.SetParamNames("pid", "sid")
	c.SetParamValues(projectID.String(), sprintID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewSprintHandler(repo)
	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var dto model.SprintDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dto.Status != model.SprintStatusActive {
		t.Fatalf("status = %q, want %q", dto.Status, model.SprintStatusActive)
	}
}

func TestSprintHandler_Update_TransitionInvalid(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	repo := &fakeSprintRepo{
		sprint: &model.Sprint{
			ID:        sprintID,
			ProjectID: projectID,
			Name:      "Sprint 3",
			StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
			Status:    model.SprintStatusClosed,
		},
	}

	body := `{"status":"active"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:pid/sprints/:sid")
	c.SetParamNames("pid", "sid")
	c.SetParamValues(projectID.String(), sprintID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewSprintHandler(repo)
	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnprocessableEntity)
	}
}

func TestSprintHandler_Update_NotFound(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	repo := &fakeSprintRepo{err: errors.New("not found")}

	body := `{"name":"nope"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:pid/sprints/:sid")
	c.SetParamNames("pid", "sid")
	c.SetParamValues(projectID.String(), sprintID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewSprintHandler(repo)
	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestSprintHandler_Create_PersistsOptionalMilestone(t *testing.T) {
	projectID := uuid.New()
	milestoneID := uuid.New()
	repo := &fakeSprintRepo{}

	body := `{"name":"Sprint 7","startDate":"2026-03-01T00:00:00Z","endDate":"2026-03-14T00:00:00Z","totalBudgetUsd":80,"milestoneId":"` + milestoneID.String() + `"}`
	e := newAgentTestEcho()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:pid/sprints")
	c.SetParamNames("pid")
	c.SetParamValues(projectID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewSprintHandler(repo)
	if err := h.Create(c); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if repo.created == nil || repo.created.MilestoneID == nil || *repo.created.MilestoneID != milestoneID {
		t.Fatalf("created sprint = %+v", repo.created)
	}
}

func TestSprintHandler_Update_PersistsMilestone(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	milestoneID := uuid.New()
	repo := &fakeSprintRepo{
		sprint: &model.Sprint{
			ID:        sprintID,
			ProjectID: projectID,
			Name:      "Sprint 4",
			StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
			Status:    model.SprintStatusPlanning,
		},
	}

	body := `{"milestoneId":"` + milestoneID.String() + `"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:pid/sprints/:sid")
	c.SetParamNames("pid", "sid")
	c.SetParamValues(projectID.String(), sprintID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewSprintHandler(repo)
	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if repo.updated == nil || repo.updated.MilestoneID == nil || *repo.updated.MilestoneID != milestoneID {
		t.Fatalf("updated sprint = %+v", repo.updated)
	}
}

func TestSprintHandler_Update_InvalidMilestoneID(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	repo := &fakeSprintRepo{
		sprint: &model.Sprint{
			ID:        sprintID,
			ProjectID: projectID,
			Name:      "Sprint 4",
			StartDate: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			EndDate:   time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC),
			Status:    model.SprintStatusPlanning,
		},
	}

	body := `{"milestoneId":"not-a-uuid"}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:pid/sprints/:sid")
	c.SetParamNames("pid", "sid")
	c.SetParamValues(projectID.String(), sprintID.String())
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewSprintHandler(repo)
	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
