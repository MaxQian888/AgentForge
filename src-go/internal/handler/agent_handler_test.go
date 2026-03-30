package handler_test

import (
	"context"
	"errors"
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
	spawnRun     *model.AgentRun
	spawnErr     error
	cancelErr    error
	updateErr    error
	listErr      error
	summaryErr   error
	getByIDErr   error
	logsErr      error
	bridgeStatus string
	lastTaskID   uuid.UUID
	lastRuntime  string
	lastRoleID   string
	lastRunID    uuid.UUID
	lastReason   string
	updateState  string
	summaries    []model.AgentRunSummaryDTO
	summary      *model.AgentRunSummaryDTO
	run          *model.AgentRun
	poolStats    model.AgentPoolStatsDTO
	logs         []model.AgentLogEntry
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
	return m.summaries, m.listErr
}

func (m *mockAgentRuntimeService) GetByID(_ context.Context, id uuid.UUID) (*model.AgentRun, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if m.run != nil {
		return m.run, nil
	}
	return &model.AgentRun{ID: id, TaskID: uuid.New(), MemberID: uuid.New()}, nil
}

func (m *mockAgentRuntimeService) GetSummary(_ context.Context, id uuid.UUID) (*model.AgentRunSummaryDTO, error) {
	if m.summaryErr != nil {
		return nil, m.summaryErr
	}
	if m.summary != nil {
		return m.summary, nil
	}
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
	return m.poolStats
}

func (m *mockAgentRuntimeService) GetLogs(_ context.Context, _ uuid.UUID) ([]model.AgentLogEntry, error) {
	return m.logs, m.logsErr
}

func (m *mockAgentRuntimeService) BridgeStatus() string {
	return m.bridgeStatus
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

func TestAgentHandler_Spawn_ReturnsServiceUnavailableWhenBridgeIsDegraded(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewAgentHandler(&mockAgentRuntimeService{bridgeStatus: service.BridgeStatusDegraded})

	req := httptest.NewRequest(http.MethodPost, "/agents/spawn", strings.NewReader(`{"taskId":"`+uuid.New().String()+`","memberId":"`+uuid.New().String()+`"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Spawn(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
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

func TestAgentHandler_Pause_ReturnsServiceUnavailableWhenBridgeIsDegraded(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewAgentHandler(&mockAgentRuntimeService{bridgeStatus: service.BridgeStatusDegraded})

	req := httptest.NewRequest(http.MethodPost, "/agents/123/pause", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	_ = h.Pause(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestAgentHandler_Resume_ReturnsServiceUnavailableWhenBridgeIsDegraded(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewAgentHandler(&mockAgentRuntimeService{bridgeStatus: service.BridgeStatusDegraded})

	req := httptest.NewRequest(http.MethodPost, "/agents/123/resume", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(uuid.New().String())

	_ = h.Resume(c)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}
}

func TestAgentHandler_ListGetPoolLogsAndPauseSuccess(t *testing.T) {
	e := newAgentTestEcho()
	runID := uuid.New()
	summary := model.AgentRunSummaryDTO{ID: runID.String(), TaskID: uuid.New().String(), MemberID: uuid.New().String()}
	svc := &mockAgentRuntimeService{
		summaries: []model.AgentRunSummaryDTO{summary},
		summary:   &summary,
		run:       &model.AgentRun{ID: runID, TaskID: uuid.New(), MemberID: uuid.New()},
		poolStats: model.AgentPoolStatsDTO{Active: 2, Available: 1, Queued: 3},
		logs:      []model.AgentLogEntry{{Type: "status", Content: "running"}},
	}
	h := handler.NewAgentHandler(svc)

	listReq := httptest.NewRequest(http.MethodGet, "/agents", nil)
	listRec := httptest.NewRecorder()
	if err := h.List(e.NewContext(listReq, listRec)); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listRec.Code != http.StatusOK {
		t.Fatalf("List() status = %d, want 200", listRec.Code)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/agents/"+runID.String(), nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetParamNames("id")
	getCtx.SetParamValues(runID.String())
	if err := h.Get(getCtx); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if getRec.Code != http.StatusOK {
		t.Fatalf("Get() status = %d, want 200", getRec.Code)
	}

	poolReq := httptest.NewRequest(http.MethodGet, "/agents/pool", nil)
	poolRec := httptest.NewRecorder()
	if err := h.Pool(e.NewContext(poolReq, poolRec)); err != nil {
		t.Fatalf("Pool() error = %v", err)
	}
	if poolRec.Code != http.StatusOK {
		t.Fatalf("Pool() status = %d, want 200", poolRec.Code)
	}

	logsReq := httptest.NewRequest(http.MethodGet, "/agents/"+runID.String()+"/logs", nil)
	logsRec := httptest.NewRecorder()
	logsCtx := e.NewContext(logsReq, logsRec)
	logsCtx.SetParamNames("id")
	logsCtx.SetParamValues(runID.String())
	if err := h.Logs(logsCtx); err != nil {
		t.Fatalf("Logs() error = %v", err)
	}
	if logsRec.Code != http.StatusOK {
		t.Fatalf("Logs() status = %d, want 200", logsRec.Code)
	}

	pauseReq := httptest.NewRequest(http.MethodPost, "/agents/"+runID.String()+"/pause", nil)
	pauseRec := httptest.NewRecorder()
	pauseCtx := e.NewContext(pauseReq, pauseRec)
	pauseCtx.SetParamNames("id")
	pauseCtx.SetParamValues(runID.String())
	if err := h.Pause(pauseCtx); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if pauseRec.Code != http.StatusOK || svc.updateState != model.AgentRunStatusPaused {
		t.Fatalf("Pause() status/state = %d / %q", pauseRec.Code, svc.updateState)
	}
}

func TestAgentHandlerKillAndUpdateStatusFallbacks(t *testing.T) {
	e := newAgentTestEcho()
	runID := uuid.New()
	svc := &mockAgentRuntimeService{
		run:        &model.AgentRun{ID: runID, TaskID: uuid.New(), MemberID: uuid.New()},
		summaryErr: service.ErrAgentNotFound,
	}
	h := handler.NewAgentHandler(svc)

	killReq := httptest.NewRequest(http.MethodPost, "/agents/"+runID.String()+"/kill", nil)
	killRec := httptest.NewRecorder()
	killCtx := e.NewContext(killReq, killRec)
	killCtx.SetParamNames("id")
	killCtx.SetParamValues(runID.String())
	if err := h.Kill(killCtx); err != nil {
		t.Fatalf("Kill() error = %v", err)
	}
	if killRec.Code != http.StatusOK || svc.lastReason != "killed_by_user" {
		t.Fatalf("Kill() status/reason = %d / %q", killRec.Code, svc.lastReason)
	}

	svc.updateErr = service.ErrAgentNotFound
	resumeReq := httptest.NewRequest(http.MethodPost, "/agents/"+runID.String()+"/resume", nil)
	resumeRec := httptest.NewRecorder()
	resumeCtx := e.NewContext(resumeReq, resumeRec)
	resumeCtx.SetParamNames("id")
	resumeCtx.SetParamValues(runID.String())
	if err := h.Resume(resumeCtx); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if resumeRec.Code != http.StatusNotFound {
		t.Fatalf("Resume() status = %d, want 404", resumeRec.Code)
	}
}

func TestAgentHandlerAdditionalErrorBranches(t *testing.T) {
	e := newAgentTestEcho()

	listHandler := handler.NewAgentHandler(&mockAgentRuntimeService{listErr: errors.New("list failed")})
	listReq := httptest.NewRequest(http.MethodGet, "/agents", nil)
	listRec := httptest.NewRecorder()
	if err := listHandler.List(e.NewContext(listReq, listRec)); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if listRec.Code != http.StatusInternalServerError {
		t.Fatalf("List() status = %d, want 500", listRec.Code)
	}

	getHandler := handler.NewAgentHandler(&mockAgentRuntimeService{})
	getReq := httptest.NewRequest(http.MethodGet, "/agents/bad", nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetParamNames("id")
	getCtx.SetParamValues("bad")
	if err := getHandler.Get(getCtx); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if getRec.Code != http.StatusBadRequest {
		t.Fatalf("Get() status = %d, want 400", getRec.Code)
	}

	logsHandler := handler.NewAgentHandler(&mockAgentRuntimeService{logsErr: service.ErrAgentNotFound})
	logsReq := httptest.NewRequest(http.MethodGet, "/agents/"+uuid.New().String()+"/logs", nil)
	logsRec := httptest.NewRecorder()
	logsCtx := e.NewContext(logsReq, logsRec)
	logsCtx.SetParamNames("id")
	logsCtx.SetParamValues(uuid.New().String())
	if err := logsHandler.Logs(logsCtx); err != nil {
		t.Fatalf("Logs() error = %v", err)
	}
	if logsRec.Code != http.StatusNotFound {
		t.Fatalf("Logs() status = %d, want 404", logsRec.Code)
	}

	runID := uuid.New()
	updateHandler := handler.NewAgentHandler(&mockAgentRuntimeService{updateErr: errors.New("bridge write failed")})
	updateReq := httptest.NewRequest(http.MethodPost, "/agents/"+runID.String()+"/resume", nil)
	updateRec := httptest.NewRecorder()
	updateCtx := e.NewContext(updateReq, updateRec)
	updateCtx.SetParamNames("id")
	updateCtx.SetParamValues(runID.String())
	if err := updateHandler.Resume(updateCtx); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}
	if updateRec.Code != http.StatusBadGateway {
		t.Fatalf("Resume() status = %d, want 502", updateRec.Code)
	}

	fetchHandler := handler.NewAgentHandler(&mockAgentRuntimeService{
		run:        nil,
		getByIDErr: errors.New("load failed"),
	})
	fetchReq := httptest.NewRequest(http.MethodPost, "/agents/"+runID.String()+"/pause", nil)
	fetchRec := httptest.NewRecorder()
	fetchCtx := e.NewContext(fetchReq, fetchRec)
	fetchCtx.SetParamNames("id")
	fetchCtx.SetParamValues(runID.String())
	if err := fetchHandler.Pause(fetchCtx); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}
	if fetchRec.Code != http.StatusInternalServerError {
		t.Fatalf("Pause() status = %d, want 500", fetchRec.Code)
	}
}
