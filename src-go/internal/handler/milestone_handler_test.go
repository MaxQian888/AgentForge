package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/server/internal/handler"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type milestoneServiceMock struct {
	milestones []*model.Milestone
	metrics    map[uuid.UUID]model.MilestoneMetrics
	created    *model.Milestone
	updated    *model.Milestone
	deletedID  uuid.UUID
}

func (m *milestoneServiceMock) CreateMilestone(_ context.Context, milestone *model.Milestone) error {
	m.created = milestone
	return nil
}
func (m *milestoneServiceMock) GetMilestone(_ context.Context, id uuid.UUID) (*model.Milestone, error) {
	for _, milestone := range m.milestones {
		if milestone.ID == id {
			return milestone, nil
		}
	}
	return nil, nil
}
func (m *milestoneServiceMock) ListMilestones(_ context.Context, _ uuid.UUID) ([]*model.Milestone, error) {
	return m.milestones, nil
}
func (m *milestoneServiceMock) UpdateMilestone(_ context.Context, milestone *model.Milestone) error {
	m.updated = milestone
	return nil
}
func (m *milestoneServiceMock) DeleteMilestone(_ context.Context, id uuid.UUID) error {
	m.deletedID = id
	return nil
}
func (m *milestoneServiceMock) AssignTaskToMilestone(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *milestoneServiceMock) AssignSprintToMilestone(_ context.Context, _, _ uuid.UUID) error {
	return nil
}
func (m *milestoneServiceMock) GetCompletionMetrics(_ context.Context, milestoneID uuid.UUID) (model.MilestoneMetrics, error) {
	return m.metrics[milestoneID], nil
}

func TestMilestoneHandler_ListAndCreate(t *testing.T) {
	projectID := uuid.New()
	milestoneID := uuid.New()
	e := echo.New()
	e.Validator = &customFieldValidator{validator: validator.New()}
	svc := &milestoneServiceMock{
		milestones: []*model.Milestone{{
			ID:        milestoneID,
			ProjectID: projectID,
			Name:      "v2.0",
			Status:    model.MilestoneStatusPlanned,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		}},
		metrics: map[uuid.UUID]model.MilestoneMetrics{
			milestoneID: {TotalTasks: 3, CompletedTasks: 1},
		},
	}
	h := handler.NewMilestoneHandler(svc)

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/milestones", nil)
	listRec := httptest.NewRecorder()
	listCtx := e.NewContext(listReq, listRec)
	listCtx.Set(appMiddleware.ProjectIDContextKey, projectID)

	if err := h.List(listCtx); err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, want %d", listRec.Code, http.StatusOK)
	}
	var body []map[string]any
	if err := json.Unmarshal(listRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body) != 1 || body[0]["name"] != "v2.0" {
		t.Fatalf("unexpected body: %#v", body)
	}

	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+projectID.String()+"/milestones", strings.NewReader(`{"name":"v3.0","status":"planned"}`))
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	createCtx := e.NewContext(createReq, createRec)
	createCtx.Set(appMiddleware.ProjectIDContextKey, projectID)
	if err := h.Create(createCtx); err != nil {
		t.Fatalf("Create() error: %v", err)
	}
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want %d", createRec.Code, http.StatusCreated)
	}
	if svc.created == nil || svc.created.ProjectID != projectID {
		t.Fatalf("unexpected created milestone: %+v", svc.created)
	}
}
