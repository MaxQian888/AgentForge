package handler_test

import (
	"context"
	"encoding/json"
	"errors"
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

type fakeTaskDecomposer struct {
	result *model.TaskDecompositionResponse
	err    error
	lastID uuid.UUID
}

type fakeTaskDispatcher struct {
	result *model.TaskDispatchResponse
	err    error
	lastID uuid.UUID
	lastReq *model.AssignRequest
}

func (f *fakeTaskDispatcher) Assign(ctx context.Context, taskID uuid.UUID, req *model.AssignRequest) (*model.TaskDispatchResponse, error) {
	f.lastID = taskID
	f.lastReq = req
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func (f *fakeTaskDecomposer) Decompose(ctx context.Context, taskID uuid.UUID) (*model.TaskDecompositionResponse, error) {
	f.lastID = taskID
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func TestTaskHandler_DecomposeInvalidID(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/not-a-uuid/decompose", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/tasks/:id/decompose")
	c.SetParamNames("id")
	c.SetParamValues("not-a-uuid")

	h := handler.NewTaskHandler(nil)
	if err := h.Decompose(c); err != nil {
		t.Fatalf("Decompose() error: %v", err)
	}

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestTaskHandler_DecomposeSuccess(t *testing.T) {
	taskID := uuid.New()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID.String()+"/decompose", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/tasks/:id/decompose")
	c.SetParamNames("id")
	c.SetParamValues(taskID.String())

	decomposer := &fakeTaskDecomposer{
		result: &model.TaskDecompositionResponse{
			ParentTask: model.TaskDTO{ID: taskID.String(), Title: "Parent"},
			Summary:    "Created two child tasks.",
			Subtasks: []model.TaskDTO{
				{ID: uuid.New().String(), Title: "Child 1"},
				{ID: uuid.New().String(), Title: "Child 2"},
			},
		},
	}

	h := handler.NewTaskHandler(nil, decomposer)
	if err := h.Decompose(c); err != nil {
		t.Fatalf("Decompose() error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body model.TaskDecompositionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Summary != "Created two child tasks." {
		t.Fatalf("summary = %q", body.Summary)
	}
	if decomposer.lastID != taskID {
		t.Fatalf("decomposer called with %s, want %s", decomposer.lastID, taskID)
	}
}

func TestTaskHandler_DecomposeErrorMapping(t *testing.T) {
	testCases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "not found", err: service.ErrTaskNotFound, wantStatus: http.StatusNotFound},
		{name: "already decomposed", err: service.ErrTaskAlreadyDecomposed, wantStatus: http.StatusConflict},
		{name: "internal", err: errors.New("bridge unavailable"), wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			taskID := uuid.New()
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID.String()+"/decompose", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetPath("/api/v1/tasks/:id/decompose")
			c.SetParamNames("id")
			c.SetParamValues(taskID.String())

			h := handler.NewTaskHandler(nil, &fakeTaskDecomposer{err: tc.err})
			if err := h.Decompose(c); err != nil {
				t.Fatalf("Decompose() error: %v", err)
			}

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
		})
	}
}

func TestTaskHandler_AssignReturnsStructuredDispatchResult(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	e := echo.New()
	e.Validator = &agentTestValidator{validator: validator.New()}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/"+taskID.String()+"/assign", strings.NewReader(`{"assigneeId":"`+memberID.String()+`","assigneeType":"agent"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/tasks/:id/assign")
	c.SetParamNames("id")
	c.SetParamValues(taskID.String())

	dispatcher := &fakeTaskDispatcher{
		result: &model.TaskDispatchResponse{
			Task: model.TaskDTO{ID: taskID.String(), AssigneeID: ptr(memberID.String()), AssigneeType: model.MemberTypeAgent},
			Dispatch: model.DispatchOutcome{
				Status: model.DispatchStatusStarted,
				Run:    &model.AgentRunDTO{ID: uuid.New().String(), TaskID: taskID.String(), MemberID: memberID.String()},
			},
		},
	}

	h := handler.NewTaskHandler(nil).WithDispatcher(dispatcher)
	if err := h.Assign(c); err != nil {
		t.Fatalf("Assign() error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body model.TaskDispatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if body.Dispatch.Status != model.DispatchStatusStarted {
		t.Fatalf("dispatch = %+v", body.Dispatch)
	}
	if dispatcher.lastID != taskID || dispatcher.lastReq.AssigneeID != memberID.String() {
		t.Fatalf("dispatcher called with %s / %+v", dispatcher.lastID, dispatcher.lastReq)
	}
}

func ptr(value string) *string {
	return &value
}
