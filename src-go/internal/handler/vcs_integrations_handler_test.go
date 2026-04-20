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

	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/vcs"
)

// fakeIntegSvc satisfies the handler's private vcsIntegrationsService.
type fakeIntegSvc struct {
	rows       map[uuid.UUID]*model.VCSIntegration
	createErr  error
	patchErr   error
	deleteErr  error
	listErr    error
	syncErr    error
	lastInput  vcs.CreateInput
	lastPatch  vcs.PatchInput
	lastDelete *uuid.UUID
}

func newFakeIntegSvc() *fakeIntegSvc {
	return &fakeIntegSvc{rows: map[uuid.UUID]*model.VCSIntegration{}}
}

func (f *fakeIntegSvc) Create(_ context.Context, in vcs.CreateInput) (*model.VCSIntegration, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.lastInput = in
	rec := &model.VCSIntegration{
		ID:               uuid.New(),
		ProjectID:        in.ProjectID,
		Provider:         in.Provider,
		Host:             in.Host,
		Owner:            in.Owner,
		Repo:             in.Repo,
		DefaultBranch:    in.DefaultBranch,
		WebhookSecretRef: in.WebhookSecretRef,
		TokenSecretRef:   in.TokenSecretRef,
		Status:           "active",
	}
	hookID := "hook-1"
	rec.WebhookID = &hookID
	f.rows[rec.ID] = rec
	return rec, nil
}

func (f *fakeIntegSvc) Patch(_ context.Context, id uuid.UUID, in vcs.PatchInput) (*model.VCSIntegration, error) {
	if f.patchErr != nil {
		return nil, f.patchErr
	}
	f.lastPatch = in
	rec, ok := f.rows[id]
	if !ok {
		return nil, errors.New("not found")
	}
	if in.Status != nil {
		rec.Status = *in.Status
	}
	return rec, nil
}

func (f *fakeIntegSvc) Delete(_ context.Context, id uuid.UUID, actor *uuid.UUID) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.lastDelete = actor
	delete(f.rows, id)
	return nil
}

func (f *fakeIntegSvc) List(_ context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := []*model.VCSIntegration{}
	for _, v := range f.rows {
		if v.ProjectID == projectID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (f *fakeIntegSvc) QueueSync(_ context.Context, id uuid.UUID, _ *uuid.UUID) (*model.VCSIntegration, error) {
	if f.syncErr != nil {
		return nil, f.syncErr
	}
	rec, ok := f.rows[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return rec, nil
}

func TestVCSIntegrationsHandler_Create_Happy(t *testing.T) {
	e := echo.New()
	svc := newFakeIntegSvc()
	h := handler.NewVCSIntegrationsHandler(svc)
	body, _ := json.Marshal(map[string]any{
		"provider":         "github",
		"host":             "github.com",
		"owner":            "octocat",
		"repo":             "hello",
		"defaultBranch":    "main",
		"tokenSecretRef":   "vcs.github.demo.pat",
		"webhookSecretRef": "vcs.github.demo.webhook",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues(uuid.New().String())
	if err := h.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestVCSIntegrationsHandler_RejectsBadProvider(t *testing.T) {
	e := echo.New()
	h := handler.NewVCSIntegrationsHandler(newFakeIntegSvc())
	body, _ := json.Marshal(map[string]any{
		"provider":         "svn",
		"host":             "x",
		"owner":            "o",
		"repo":             "r",
		"tokenSecretRef":   "t",
		"webhookSecretRef": "w",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues(uuid.New().String())
	if err := h.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestVCSIntegrationsHandler_Create_MissingFieldsReturns400(t *testing.T) {
	e := echo.New()
	h := handler.NewVCSIntegrationsHandler(newFakeIntegSvc())
	body, _ := json.Marshal(map[string]any{"provider": "github"})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues(uuid.New().String())
	if err := h.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestVCSIntegrationsHandler_Create_AuthExpiredMaps401(t *testing.T) {
	e := echo.New()
	svc := newFakeIntegSvc()
	svc.createErr = vcs.ErrAuthExpired
	h := handler.NewVCSIntegrationsHandler(svc)
	body, _ := json.Marshal(map[string]any{
		"provider":         "github",
		"host":             "github.com",
		"owner":            "o",
		"repo":             "r",
		"tokenSecretRef":   "t",
		"webhookSecretRef": "w",
	})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues(uuid.New().String())
	if err := h.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestVCSIntegrationsHandler_DeleteReturnsNoContent(t *testing.T) {
	e := echo.New()
	svc := newFakeIntegSvc()
	rec0 := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Provider: "github"}
	svc.rows[rec0.ID] = rec0
	h := handler.NewVCSIntegrationsHandler(svc)
	req := httptest.NewRequest(http.MethodDelete, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(rec0.ID.String())
	if err := h.Delete(c); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestVCSIntegrationsHandler_SyncReturns202(t *testing.T) {
	e := echo.New()
	svc := newFakeIntegSvc()
	rec0 := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Provider: "github"}
	svc.rows[rec0.ID] = rec0
	h := handler.NewVCSIntegrationsHandler(svc)
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(rec0.ID.String())
	if err := h.Sync(c); err != nil {
		t.Fatalf("Sync: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}
}

func TestVCSIntegrationsHandler_List_EmptyReturnsArray(t *testing.T) {
	e := echo.New()
	h := handler.NewVCSIntegrationsHandler(newFakeIntegSvc())
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("pid")
	c.SetParamValues(uuid.New().String())
	if err := h.List(c); err != nil {
		t.Fatalf("List: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	var out []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, rec.Body.String())
	}
	if len(out) != 0 {
		t.Errorf("expected empty array, got %d items", len(out))
	}
}

func TestVCSIntegrationsHandler_Patch_StatusUpdate(t *testing.T) {
	e := echo.New()
	svc := newFakeIntegSvc()
	rec0 := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Provider: "github", Status: "active"}
	svc.rows[rec0.ID] = rec0
	h := handler.NewVCSIntegrationsHandler(svc)
	body, _ := json.Marshal(map[string]any{"status": "paused"})
	req := httptest.NewRequest(http.MethodPatch, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(rec0.ID.String())
	if err := h.Patch(c); err != nil {
		t.Fatalf("Patch: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if svc.rows[rec0.ID].Status != "paused" {
		t.Errorf("expected status paused, got %q", svc.rows[rec0.ID].Status)
	}
}
