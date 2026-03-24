package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
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

type fakeTaskRepo struct {
	tasks          map[uuid.UUID]*model.Task
	lastUpdateID   uuid.UUID
	lastUpdateReq  *model.UpdateTaskRequest
	lastTransition []struct {
		id     uuid.UUID
		status string
	}
}

func (f *fakeTaskRepo) Create(context.Context, *model.Task) error {
	return nil
}

func (f *fakeTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	task, ok := f.tasks[id]
	if !ok {
		return nil, errors.New("task not found")
	}
	cloned := *task
	if task.BlockedBy != nil {
		cloned.BlockedBy = append([]string(nil), task.BlockedBy...)
	}
	return &cloned, nil
}

func (f *fakeTaskRepo) List(context.Context, uuid.UUID, model.TaskListQuery) ([]*model.Task, int, error) {
	return nil, 0, nil
}

func (f *fakeTaskRepo) Update(_ context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error {
	f.lastUpdateID = id
	f.lastUpdateReq = req
	task, ok := f.tasks[id]
	if !ok {
		return errors.New("task not found")
	}
	if req.BlockedBy != nil {
		task.BlockedBy = append([]string(nil), (*req.BlockedBy)...)
	}
	task.UpdatedAt = time.Now().UTC()
	return nil
}

func (f *fakeTaskRepo) Delete(context.Context, uuid.UUID) error {
	return nil
}

func (f *fakeTaskRepo) TransitionStatus(_ context.Context, id uuid.UUID, newStatus string) error {
	task, ok := f.tasks[id]
	if !ok {
		return errors.New("task not found")
	}
	f.lastTransition = append(f.lastTransition, struct {
		id     uuid.UUID
		status string
	}{id: id, status: newStatus})
	task.Status = newStatus
	now := time.Now().UTC()
	task.UpdatedAt = now
	if newStatus == model.TaskStatusDone {
		task.CompletedAt = &now
	}
	return nil
}

func (f *fakeTaskRepo) UpdateAssignee(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}

func (f *fakeTaskRepo) ListDependents(_ context.Context, blockerID uuid.UUID) ([]*model.Task, error) {
	dependents := make([]*model.Task, 0)
	blocker := blockerID.String()
	for _, task := range f.tasks {
		if slices.Contains(task.BlockedBy, blocker) {
			cloned := *task
			cloned.BlockedBy = append([]string(nil), task.BlockedBy...)
			dependents = append(dependents, &cloned)
		}
	}
	return dependents, nil
}

type fakeTaskDecomposer struct {
	result *model.TaskDecompositionResponse
	err    error
	lastID uuid.UUID
}

type fakeTaskDispatcher struct {
	result  *model.TaskDispatchResponse
	err     error
	lastID  uuid.UUID
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

func TestTaskHandler_UpdatePersistsBlockedBy(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	blockerID := uuid.New()
	repo := &fakeTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {
				ID:        taskID,
				ProjectID: projectID,
				Title:     "Implement dependency rail",
				Status:    model.TaskStatusTriaged,
				Priority:  "high",
				CreatedAt: time.Now().Add(-2 * time.Hour).UTC(),
				UpdatedAt: time.Now().Add(-time.Hour).UTC(),
				BlockedBy: []string{},
			},
			blockerID: {
				ID:        blockerID,
				ProjectID: projectID,
				Title:     "Finish API contract",
				Status:    model.TaskStatusInProgress,
				Priority:  "high",
				CreatedAt: time.Now().Add(-3 * time.Hour).UTC(),
				UpdatedAt: time.Now().Add(-2 * time.Hour).UTC(),
			},
		},
	}

	e := echo.New()
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/tasks/"+taskID.String(),
		strings.NewReader(`{"blockedBy":["`+blockerID.String()+`"]}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/tasks/:id")
	c.SetParamNames("id")
	c.SetParamValues(taskID.String())

	h := handler.NewTaskHandler(repo)
	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if repo.lastUpdateReq == nil || repo.lastUpdateReq.BlockedBy == nil {
		t.Fatal("expected blockedBy update request to be passed to repository")
	}
	if got := *repo.lastUpdateReq.BlockedBy; len(got) != 1 || got[0] != blockerID.String() {
		t.Fatalf("blockedBy = %v, want [%s]", got, blockerID.String())
	}

	var body model.TaskDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body.BlockedBy) != 1 || body.BlockedBy[0] != blockerID.String() {
		t.Fatalf("response blockedBy = %v, want [%s]", body.BlockedBy, blockerID.String())
	}
}

func TestTaskHandler_TransitionDoneAutoUnblocksReadyDependents(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	assignedDependentID := uuid.New()
	triagedDependentID := uuid.New()
	reporterID := uuid.New()
	repo := &fakeTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {
				ID:        taskID,
				ProjectID: projectID,
				Title:     "Finish API contract",
				Status:    model.TaskStatusInReview,
				Priority:  "high",
				CreatedAt: time.Now().Add(-4 * time.Hour).UTC(),
				UpdatedAt: time.Now().Add(-2 * time.Hour).UTC(),
			},
			assignedDependentID: {
				ID:           assignedDependentID,
				ProjectID:    projectID,
				Title:        "Ship integration",
				Status:       model.TaskStatusBlocked,
				Priority:     "high",
				AssigneeID:   &reporterID,
				AssigneeType: model.MemberTypeHuman,
				BlockedBy:    []string{taskID.String()},
				CreatedAt:    time.Now().Add(-3 * time.Hour).UTC(),
				UpdatedAt:    time.Now().Add(-time.Hour).UTC(),
			},
			triagedDependentID: {
				ID:        triagedDependentID,
				ProjectID: projectID,
				Title:     "Prepare release notes",
				Status:    model.TaskStatusBlocked,
				Priority:  "medium",
				BlockedBy: []string{taskID.String()},
				CreatedAt: time.Now().Add(-3 * time.Hour).UTC(),
				UpdatedAt: time.Now().Add(-time.Hour).UTC(),
			},
		},
	}

	e := echo.New()
	e.Validator = &agentTestValidator{validator: validator.New()}
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/tasks/"+taskID.String()+"/transition",
		strings.NewReader(`{"status":"done"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/tasks/:id/transition")
	c.SetParamNames("id")
	c.SetParamValues(taskID.String())

	h := handler.NewTaskHandler(repo)
	if err := h.Transition(c); err != nil {
		t.Fatalf("Transition() error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if repo.tasks[assignedDependentID].Status != model.TaskStatusAssigned {
		t.Fatalf("assigned dependent status = %q, want %q", repo.tasks[assignedDependentID].Status, model.TaskStatusAssigned)
	}
	if repo.tasks[triagedDependentID].Status != model.TaskStatusTriaged {
		t.Fatalf("triaged dependent status = %q, want %q", repo.tasks[triagedDependentID].Status, model.TaskStatusTriaged)
	}
}

func ptr(value string) *string {
	return &value
}
