package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type agentTestValidator struct {
	validator *validator.Validate
}

func (tv *agentTestValidator) Validate(i interface{}) error {
	return tv.validator.Struct(i)
}

type mockAgentRuntimeService struct {
	spawnRun    *model.AgentRun
	spawnErr    error
	cancelErr   error
	updateErr   error
	lastTaskID  uuid.UUID
	lastRunID   uuid.UUID
	lastReason  string
	updateState string
}

func (m *mockAgentRuntimeService) Spawn(_ context.Context, taskID, memberID uuid.UUID, provider, modelName string, budgetUsd float64) (*model.AgentRun, error) {
	m.lastTaskID = taskID
	if m.spawnErr != nil {
		return nil, m.spawnErr
	}
	if m.spawnRun != nil {
		return m.spawnRun, nil
	}
	return &model.AgentRun{
		ID:       uuid.New(),
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
		Provider: provider,
		Model:    modelName,
	}, nil
}

func (m *mockAgentRuntimeService) ListActive(_ context.Context) ([]*model.AgentRun, error) {
	return []*model.AgentRun{}, nil
}

func (m *mockAgentRuntimeService) GetByID(_ context.Context, id uuid.UUID) (*model.AgentRun, error) {
	return &model.AgentRun{ID: id, TaskID: uuid.New(), MemberID: uuid.New()}, nil
}

func (m *mockAgentRuntimeService) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	m.lastRunID = id
	m.updateState = status
	return m.updateErr
}

func (m *mockAgentRuntimeService) Cancel(_ context.Context, id uuid.UUID, reason string) error {
	m.lastRunID = id
	m.lastReason = reason
	return m.cancelErr
}

func newAgentTestEcho() *echo.Echo {
	e := echo.New()
	e.Validator = &agentTestValidator{validator: validator.New()}
	return e
}

func TestAgentHandler_Spawn_MapsAlreadyRunningToConflict(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewAgentHandler(&mockAgentRuntimeService{spawnErr: service.ErrAgentAlreadyRunning})

	req := httptest.NewRequest(http.MethodPost, "/agents/spawn", strings.NewReader(`{"taskId":"`+uuid.New().String()+`","memberId":"`+uuid.New().String()+`","provider":"anthropic","model":"sonnet"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Spawn(c)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestAgentHandler_Kill_MapsInvalidStateToConflict(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewAgentHandler(&mockAgentRuntimeService{cancelErr: service.ErrAgentNotRunning})

	req := httptest.NewRequest(http.MethodPost, "/agents/123/kill", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	_ = h.Kill(c)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}
