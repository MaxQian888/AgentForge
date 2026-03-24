package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeWorkflowRepo struct {
	config    *model.WorkflowConfig
	getErr    error
	upsertErr error
	upserted  *model.WorkflowConfig
}

func (f *fakeWorkflowRepo) GetByProject(_ context.Context, projectID uuid.UUID) (*model.WorkflowConfig, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.config == nil {
		return nil, errors.New("not found")
	}
	return f.config, nil
}

func (f *fakeWorkflowRepo) Upsert(_ context.Context, projectID uuid.UUID, transitions json.RawMessage, triggers json.RawMessage) (*model.WorkflowConfig, error) {
	if f.upsertErr != nil {
		return nil, f.upsertErr
	}
	wf := &model.WorkflowConfig{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Transitions: transitions,
		Triggers:    triggers,
	}
	f.upserted = wf
	return wf, nil
}

func TestWorkflowHandler_Get_ReturnsDefault(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeWorkflowRepo{getErr: errors.New("not found")}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewWorkflowHandler(repo)
	if err := h.Get(c); err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var dto model.WorkflowConfigDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if dto.ProjectID != projectID.String() {
		t.Fatalf("projectId = %q, want %q", dto.ProjectID, projectID.String())
	}
	if len(dto.Transitions) != 0 {
		t.Fatalf("transitions = %v, want empty", dto.Transitions)
	}
}

func TestWorkflowHandler_Put_Success(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeWorkflowRepo{}

	body := `{"transitions":{"inbox":["triaged"]},"triggers":[{"fromStatus":"triaged","toStatus":"assigned","action":"auto_assign"}]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewWorkflowHandler(repo)
	if err := h.Put(c); err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if repo.upserted == nil {
		t.Fatal("expected upserted config")
	}

	var dto model.WorkflowConfigDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dto); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(dto.Transitions) != 1 {
		t.Fatalf("transitions count = %d, want 1", len(dto.Transitions))
	}
	if len(dto.Triggers) != 1 {
		t.Fatalf("triggers count = %d, want 1", len(dto.Triggers))
	}
}

func TestWorkflowHandler_Put_RepoError(t *testing.T) {
	projectID := uuid.New()
	repo := &fakeWorkflowRepo{upsertErr: errors.New("db error")}

	body := `{"transitions":{},"triggers":[]}`
	e := echo.New()
	req := httptest.NewRequest(http.MethodPut, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	h := handler.NewWorkflowHandler(repo)
	if err := h.Put(c); err != nil {
		t.Fatalf("Put() error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
