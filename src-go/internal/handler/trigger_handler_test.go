package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/trigger"
)

type mockTriggerRouter struct {
	lastEvent trigger.Event
	started   int
	err       error
	outcomes  []trigger.Outcome
}

func (m *mockTriggerRouter) Route(_ context.Context, ev trigger.Event) (int, error) {
	m.lastEvent = ev
	return m.started, m.err
}

func (m *mockTriggerRouter) RouteWithOutcomes(_ context.Context, ev trigger.Event) ([]trigger.Outcome, error) {
	m.lastEvent = ev
	if m.outcomes != nil {
		return m.outcomes, m.err
	}
	// Default: synthesize one Started outcome per recorded `started` count.
	out := make([]trigger.Outcome, 0, m.started)
	for i := 0; i < m.started; i++ {
		runID := uuid.New()
		out = append(out, trigger.Outcome{
			TriggerID:  uuid.New(),
			TargetKind: model.TriggerTargetDAG,
			Status:     trigger.OutcomeStarted,
			RunID:      &runID,
		})
	}
	return out, m.err
}

func setupTriggerHandler(router *mockTriggerRouter) *echo.Echo {
	e := echo.New()
	h := handler.NewTriggerHandler(router)
	h.RegisterRoutes(e)
	return e
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func TestTriggerHandler_HandleIMEvent_Success(t *testing.T) {
	router := &mockTriggerRouter{started: 1}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{
		"platform": "feishu", "command": "/review", "content": "/review http://example.com",
		"args": []any{"http://example.com"}, "chatId": "chat-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status: want 202, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	if router.lastEvent.Source != model.TriggerSourceIM {
		t.Errorf("source: want im, got %s", router.lastEvent.Source)
	}
	if router.lastEvent.Data["command"] != "/review" {
		t.Errorf("command: want /review, got %v", router.lastEvent.Data["command"])
	}
}

func TestTriggerHandler_HandleIMEvent_NoMatchReturns404(t *testing.T) {
	router := &mockTriggerRouter{started: 0}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{"platform": "feishu", "command": "/unknown"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rec.Code)
	}
}

func TestTriggerHandler_HandleIMEvent_MissingPlatform(t *testing.T) {
	router := &mockTriggerRouter{}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{"command": "/review"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}
}

func TestTriggerHandler_HandleIMEvent_InvalidBody(t *testing.T) {
	router := &mockTriggerRouter{}
	e := setupTriggerHandler(router)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events",
		bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}
}

func TestTriggerHandler_HandleIMEvent_RouterError_NoExecutions(t *testing.T) {
	router := &mockTriggerRouter{started: 0, err: errors.New("boom")}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{"platform": "feishu", "command": "/review"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}
}

func TestTriggerHandler_HandleIMEvent_RouterError_PartialSuccess(t *testing.T) {
	router := &mockTriggerRouter{started: 1, err: errors.New("one of them failed")}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{"platform": "feishu", "command": "/review"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Partial success → 202, not 500.
	if rec.Code != http.StatusAccepted {
		t.Errorf("status: want 202 on partial success, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Spec 1C — Trigger CRUD handler tests
// ---------------------------------------------------------------------------

type mockCRUDService struct {
	createOut  *model.WorkflowTrigger
	createErr  error
	patchOut   *model.WorkflowTrigger
	patchErr   error
	deleteErr  error
	listOut    []*model.WorkflowTrigger
	listErr    error
	testOut    *trigger.DryRunResult
	testErr    error
	lastEvent  map[string]any
	lastListID uuid.UUID
}

func (m *mockCRUDService) Create(_ context.Context, _ trigger.CreateTriggerInput) (*model.WorkflowTrigger, error) {
	return m.createOut, m.createErr
}
func (m *mockCRUDService) Patch(_ context.Context, _ uuid.UUID, _ trigger.PatchTriggerInput) (*model.WorkflowTrigger, error) {
	return m.patchOut, m.patchErr
}
func (m *mockCRUDService) Delete(_ context.Context, _ uuid.UUID) error {
	return m.deleteErr
}
func (m *mockCRUDService) ListByEmployee(_ context.Context, id uuid.UUID) ([]*model.WorkflowTrigger, error) {
	m.lastListID = id
	return m.listOut, m.listErr
}
func (m *mockCRUDService) Test(_ context.Context, _ uuid.UUID, ev map[string]any) (*trigger.DryRunResult, error) {
	m.lastEvent = ev
	return m.testOut, m.testErr
}

func setupCRUDHandler(crud *mockCRUDService) *echo.Echo {
	e := echo.New()
	h := handler.NewTriggerHandler(&mockTriggerRouter{}).WithCRUDService(crud)
	h.RegisterRoutes(e)
	return e
}

func TestTriggerHandler_Create_HappyPath(t *testing.T) {
	wfID := uuid.New()
	out := &model.WorkflowTrigger{
		ID:         uuid.New(),
		WorkflowID: &wfID,
		Source:     model.TriggerSourceIM,
		CreatedVia: model.TriggerCreatedViaManual,
	}
	crud := &mockCRUDService{createOut: out}
	e := setupCRUDHandler(crud)

	body := jsonBody(t, map[string]any{
		"workflowId":   wfID.String(),
		"source":       "im",
		"config":       map[string]any{"platform": "feishu", "command": "/echo"},
		"inputMapping": map[string]any{},
		"displayName":  "echo",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("status: want 201, got %d (body=%s)", rec.Code, rec.Body.String())
	}
}

func TestTriggerHandler_Create_WorkflowNotFound(t *testing.T) {
	crud := &mockCRUDService{createErr: trigger.ErrTriggerWorkflowNotFound}
	e := setupCRUDHandler(crud)

	body := jsonBody(t, map[string]any{
		"workflowId": uuid.New().String(),
		"source":     "im",
		"config":     map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp["code"] != "trigger:workflow_not_found" {
		t.Errorf("code: want trigger:workflow_not_found, got %v", resp["code"])
	}
}

func TestTriggerHandler_Create_InvalidWorkflowID(t *testing.T) {
	crud := &mockCRUDService{}
	e := setupCRUDHandler(crud)

	body := jsonBody(t, map[string]any{
		"workflowId": "not-a-uuid",
		"source":     "im",
		"config":     map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400 for invalid uuid, got %d", rec.Code)
	}
}

func TestTriggerHandler_Create_InvalidSource(t *testing.T) {
	crud := &mockCRUDService{}
	e := setupCRUDHandler(crud)

	body := jsonBody(t, map[string]any{
		"workflowId": uuid.New().String(),
		"source":     "webhook",
		"config":     map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400 for invalid source, got %d", rec.Code)
	}
}

func TestTriggerHandler_Patch_RejectsImmutableField(t *testing.T) {
	crud := &mockCRUDService{}
	e := setupCRUDHandler(crud)

	body := jsonBody(t, map[string]any{
		"workflowId": uuid.New().String(),
		"enabled":    false,
	})
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/triggers/"+uuid.New().String(), body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400 when patching workflowId, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["code"] != "trigger:immutable_field" {
		t.Errorf("code: want trigger:immutable_field, got %v", resp["code"])
	}
}

func TestTriggerHandler_Delete_DAGManagedReturns409(t *testing.T) {
	crud := &mockCRUDService{deleteErr: trigger.ErrTriggerCannotDeleteDAGManaged}
	e := setupCRUDHandler(crud)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/triggers/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Errorf("status: want 409, got %d", rec.Code)
	}
	var resp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["code"] != "trigger:cannot_delete_dag_managed" {
		t.Errorf("code: want trigger:cannot_delete_dag_managed, got %v", resp["code"])
	}
}

func TestTriggerHandler_Delete_NotFound(t *testing.T) {
	crud := &mockCRUDService{deleteErr: trigger.ErrTriggerNotFound}
	e := setupCRUDHandler(crud)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/triggers/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rec.Code)
	}
}

func TestTriggerHandler_Delete_ManualSucceeds(t *testing.T) {
	crud := &mockCRUDService{deleteErr: nil}
	e := setupCRUDHandler(crud)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/triggers/"+uuid.New().String(), nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("status: want 204, got %d", rec.Code)
	}
}

func TestTriggerHandler_ListByEmployee_OK(t *testing.T) {
	empID := uuid.New()
	wfID := uuid.New()
	crud := &mockCRUDService{listOut: []*model.WorkflowTrigger{
		{ID: uuid.New(), WorkflowID: &wfID, ActingEmployeeID: &empID},
	}}
	e := setupCRUDHandler(crud)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/employees/"+empID.String()+"/triggers", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d", rec.Code)
	}
	if crud.lastListID != empID {
		t.Errorf("lastListID: want %s, got %s", empID, crud.lastListID)
	}
	var arr []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &arr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(arr) != 1 {
		t.Errorf("expected 1 row, got %d", len(arr))
	}
}

func TestTriggerHandler_Test_DryRun(t *testing.T) {
	crud := &mockCRUDService{testOut: &trigger.DryRunResult{
		Matched:       true,
		WouldDispatch: true,
		RenderedInput: map[string]any{"text": "hi"},
	}}
	e := setupCRUDHandler(crud)

	body := jsonBody(t, map[string]any{
		"event": map[string]any{
			"platform": "feishu", "command": "/echo", "content": "/echo hi",
			"chat_id": "c-1", "args": []any{"hi"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/"+uuid.New().String()+"/test", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: want 200, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	if crud.lastEvent["command"] != "/echo" {
		t.Errorf("lastEvent.command: want /echo, got %v", crud.lastEvent["command"])
	}
	var resp trigger.DryRunResult
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !resp.Matched || !resp.WouldDispatch {
		t.Errorf("expected matched+would_dispatch, got %+v", resp)
	}
	if resp.RenderedInput["text"] != "hi" {
		t.Errorf("RenderedInput.text: want hi, got %v", resp.RenderedInput["text"])
	}
}
