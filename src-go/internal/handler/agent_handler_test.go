package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	lastRuntime string
	lastRoleID  string
	lastRunID   uuid.UUID
	lastReason  string
	updateState string
}

type mockAgentTaskDispatcher struct {
	result       *model.TaskDispatchResponse
	err          error
	lastTaskID   uuid.UUID
	lastMemberID *uuid.UUID
	lastRuntime  string
	lastRoleID   string
}

func (m *mockAgentRuntimeService) Spawn(_ context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	m.lastTaskID = taskID
	m.lastRuntime = runtime
	m.lastRoleID = roleID
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

func (m *mockAgentRuntimeService) ListSummaries(_ context.Context) ([]model.AgentRunSummaryDTO, error) {
	return []model.AgentRunSummaryDTO{}, nil
}

func (m *mockAgentRuntimeService) GetByID(_ context.Context, id uuid.UUID) (*model.AgentRun, error) {
	return &model.AgentRun{ID: id, TaskID: uuid.New(), MemberID: uuid.New()}, nil
}

func (m *mockAgentRuntimeService) GetSummary(_ context.Context, id uuid.UUID) (*model.AgentRunSummaryDTO, error) {
	return &model.AgentRunSummaryDTO{ID: id.String(), TaskID: uuid.New().String(), MemberID: uuid.New().String()}, nil
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

func (m *mockAgentRuntimeService) PoolStats(_ context.Context) model.AgentPoolStatsDTO {
	return model.AgentPoolStatsDTO{}
}

func (m *mockAgentRuntimeService) GetLogs(_ context.Context, _ uuid.UUID) ([]model.AgentLogEntry, error) {
	return nil, nil
}

func (m *mockAgentTaskDispatcher) Spawn(_ context.Context, input service.DispatchSpawnInput) (*model.TaskDispatchResponse, error) {
	m.lastTaskID = input.TaskID
	m.lastMemberID = input.MemberID
	m.lastRuntime = input.Runtime
	m.lastRoleID = input.RoleID
	if m.err != nil {
		return nil, m.err
	}
	if m.result != nil {
		return m.result, nil
	}
	return &model.TaskDispatchResponse{
		Task: model.TaskDTO{ID: input.TaskID.String()},
		Dispatch: model.DispatchOutcome{
			Status: model.DispatchStatusStarted,
			Run:    &model.AgentRunDTO{ID: uuid.New().String(), TaskID: input.TaskID.String()},
		},
	}, nil
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

func TestAgentHandler_Spawn_MapsWorktreeUnavailableToConflict(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewAgentHandler(&mockAgentRuntimeService{spawnErr: service.ErrAgentWorktreeUnavailable})

	req := httptest.NewRequest(http.MethodPost, "/agents/spawn", strings.NewReader(`{"taskId":"`+uuid.New().String()+`","memberId":"`+uuid.New().String()+`","provider":"anthropic","model":"sonnet"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Spawn(c)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestAgentHandler_Spawn_ForwardsExplicitRuntime(t *testing.T) {
	e := newAgentTestEcho()
	mockSvc := &mockAgentRuntimeService{}
	dispatcher := &mockAgentTaskDispatcher{}
	h := handler.NewAgentHandler(mockSvc).WithDispatcher(dispatcher)

	req := httptest.NewRequest(http.MethodPost, "/agents/spawn", strings.NewReader(`{"taskId":"`+uuid.New().String()+`","memberId":"`+uuid.New().String()+`","runtime":"codex","provider":"openai","model":"gpt-5-codex","roleId":"frontend-developer"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Spawn(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if dispatcher.lastRuntime != "codex" {
		t.Fatalf("expected runtime codex, got %q", dispatcher.lastRuntime)
	}
	if dispatcher.lastRoleID != "frontend-developer" {
		t.Fatalf("expected role id frontend-developer, got %q", dispatcher.lastRoleID)
	}
}

func TestAgentHandler_Spawn_AllowsMissingMemberIDWhenDispatcherCanResolveTaskAssignee(t *testing.T) {
	e := newAgentTestEcho()
	dispatcher := &mockAgentTaskDispatcher{}
	h := handler.NewAgentHandler(&mockAgentRuntimeService{}).WithDispatcher(dispatcher)

	taskID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/agents/spawn", strings.NewReader(`{"taskId":"`+taskID.String()+`","runtime":"codex","roleId":"frontend-developer"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Spawn(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if dispatcher.lastTaskID != taskID {
		t.Fatalf("task id = %s, want %s", dispatcher.lastTaskID, taskID)
	}
	if dispatcher.lastMemberID != nil {
		t.Fatalf("expected nil member id, got %v", *dispatcher.lastMemberID)
	}
}

func TestAgentHandler_Spawn_ReturnsAcceptedWhenDispatcherQueuesAdmission(t *testing.T) {
	e := newAgentTestEcho()
	dispatcher := &mockAgentTaskDispatcher{
		result: &model.TaskDispatchResponse{
			Task: model.TaskDTO{ID: uuid.New().String()},
			Dispatch: model.DispatchOutcome{
				Status: model.DispatchStatusQueued,
				Reason: "agent pool is at capacity",
				Queue: &model.AgentPoolQueueEntry{
					EntryID:   uuid.NewString(),
					TaskID:    uuid.NewString(),
					MemberID:  uuid.NewString(),
					Status:    model.AgentPoolQueueStatusQueued,
					CreatedAt: time.Now().UTC(),
					UpdatedAt: time.Now().UTC(),
				},
			},
		},
	}
	h := handler.NewAgentHandler(&mockAgentRuntimeService{}).WithDispatcher(dispatcher)

	req := httptest.NewRequest(http.MethodPost, "/agents/spawn", strings.NewReader(`{"taskId":"`+uuid.New().String()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Spawn(c)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
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
