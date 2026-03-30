package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type mockQueueManagementService struct {
	listEntries   []*model.AgentPoolQueueEntry
	listErr       error
	cancelEntry   *model.AgentPoolQueueEntry
	cancelErr     error
	lastProjectID uuid.UUID
	lastEntryID   string
	lastReason    string
	lastStatus    string
}

func (m *mockQueueManagementService) ListQueueEntries(_ context.Context, projectID uuid.UUID, statusFilter string) ([]*model.AgentPoolQueueEntry, error) {
	m.lastProjectID = projectID
	m.lastStatus = statusFilter
	return m.listEntries, m.listErr
}

func (m *mockQueueManagementService) CancelQueueEntry(_ context.Context, projectID uuid.UUID, entryID string, reason string) (*model.AgentPoolQueueEntry, error) {
	m.lastProjectID = projectID
	m.lastEntryID = entryID
	m.lastReason = reason
	if m.cancelErr != nil {
		return nil, m.cancelErr
	}
	return m.cancelEntry, nil
}

func TestQueueManagementHandler_ListReturnsEntries(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	entry := &model.AgentPoolQueueEntry{
		EntryID:   uuid.NewString(),
		ProjectID: projectID.String(),
		TaskID:    uuid.NewString(),
		MemberID:  uuid.NewString(),
		Status:    model.AgentPoolQueueStatusQueued,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	mockService := &mockQueueManagementService{listEntries: []*model.AgentPoolQueueEntry{entry}}
	h := handler.NewQueueManagementHandler(mockService)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/queue?status=queued", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	if err := h.List(c); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if mockService.lastProjectID != projectID || mockService.lastStatus != "queued" {
		t.Fatalf("service inputs = project %s status %q", mockService.lastProjectID, mockService.lastStatus)
	}

	var response []model.QueueEntryDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response) != 1 || response[0].EntryID != entry.EntryID {
		t.Fatalf("response = %+v, want one entry %s", response, entry.EntryID)
	}
}

func TestQueueManagementHandler_CancelMapsConflict(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	entryID := uuid.NewString()
	h := handler.NewQueueManagementHandler(&mockQueueManagementService{
		cancelErr: &service.QueueEntryStatusConflictError{
			EntryID: entryID,
			Status:  model.AgentPoolQueueStatusPromoted,
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/queue/"+entryID+"?reason=manual_cancel", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)
	c.SetParamNames("entryId")
	c.SetParamValues(entryID)

	if err := h.Cancel(c); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", rec.Code)
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["status"] != string(model.AgentPoolQueueStatusPromoted) {
		t.Fatalf("status payload = %#v, want promoted", response["status"])
	}
}

func TestQueueManagementHandler_CancelReturnsUpdatedEntry(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	entryID := uuid.NewString()
	h := handler.NewQueueManagementHandler(&mockQueueManagementService{
		cancelEntry: &model.AgentPoolQueueEntry{
			EntryID:   entryID,
			ProjectID: projectID.String(),
			TaskID:    uuid.NewString(),
			MemberID:  uuid.NewString(),
			Status:    model.AgentPoolQueueStatusCancelled,
			Reason:    "manual_cancel",
			CreatedAt: time.Now().UTC().Add(-time.Minute),
			UpdatedAt: time.Now().UTC(),
		},
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/queue/"+entryID+"?reason=manual_cancel", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)
	c.SetParamNames("entryId")
	c.SetParamValues(entryID)

	if err := h.Cancel(c); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var response model.QueueEntryDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.EntryID != entryID || response.Status != string(model.AgentPoolQueueStatusCancelled) {
		t.Fatalf("response = %+v, want cancelled entry %s", response, entryID)
	}
}

func TestQueueManagementHandler_CancelMapsNotFound(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	entryID := uuid.NewString()
	h := handler.NewQueueManagementHandler(&mockQueueManagementService{
		cancelErr: service.ErrQueueEntryNotFound,
	})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID.String()+"/queue/"+entryID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)
	c.SetParamNames("entryId")
	c.SetParamValues(entryID)

	if err := h.Cancel(c); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
}

func TestQueueManagementHandler_ProtectedRoutesRequireAuth(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewQueueManagementHandler(&mockQueueManagementService{})
	jwtMw := appMiddleware.JWTMiddleware("test-secret", noopBlacklist{})

	e.GET("/api/v1/projects/:pid/queue", h.List, jwtMw)
	e.DELETE("/api/v1/projects/:pid/queue/:entryId", h.Cancel, jwtMw)

	for _, method := range []struct {
		name       string
		httpMethod string
		path       string
	}{
		{name: "list", httpMethod: http.MethodGet, path: "/api/v1/projects/" + uuid.NewString() + "/queue"},
		{name: "cancel", httpMethod: http.MethodDelete, path: "/api/v1/projects/" + uuid.NewString() + "/queue/" + uuid.NewString()},
	} {
		req := httptest.NewRequest(method.httpMethod, method.path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401", method.name, rec.Code)
		}
	}
}

type noopBlacklist struct{}

func (noopBlacklist) IsBlacklisted(context.Context, string) (bool, error) {
	return false, nil
}

var _ = errors.New
