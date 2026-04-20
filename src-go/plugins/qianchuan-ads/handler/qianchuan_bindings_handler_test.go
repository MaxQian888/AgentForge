package qchandler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/agentforge/server/internal/adsplatform"
	qianchuanbinding "github.com/agentforge/server/plugins/qianchuan-ads/binding"
	handler "github.com/agentforge/server/plugins/qianchuan-ads/handler"
)

// fakeBindingsSvc is an in-memory implementation of handler.QianchuanBindingsService
// that exercises the full happy path without DB or HTTP.
type fakeBindingsSvc struct {
	mu       sync.Mutex
	rows     map[uuid.UUID]*qianchuanbinding.Record
	createFn func(qianchuanbinding.CreateInput) error
}

func newFakeBindingsSvc() *fakeBindingsSvc {
	return &fakeBindingsSvc{rows: map[uuid.UUID]*qianchuanbinding.Record{}}
}

func (f *fakeBindingsSvc) Create(_ context.Context, in qianchuanbinding.CreateInput) (*qianchuanbinding.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.createFn != nil {
		if err := f.createFn(in); err != nil {
			return nil, err
		}
	}
	rec := &qianchuanbinding.Record{
		ID:                    uuid.New(),
		ProjectID:             in.ProjectID,
		AdvertiserID:          in.AdvertiserID,
		AwemeID:               in.AwemeID,
		DisplayName:           in.DisplayName,
		Status:                qianchuanbinding.StatusActive,
		ActingEmployeeID:      in.ActingEmployeeID,
		AccessTokenSecretRef:  in.AccessTokenSecretRef,
		RefreshTokenSecretRef: in.RefreshTokenSecretRef,
		CreatedBy:             in.CreatedBy,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	}
	f.rows[rec.ID] = rec
	return rec, nil
}

func (f *fakeBindingsSvc) Get(_ context.Context, id uuid.UUID) (*qianchuanbinding.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok {
		return nil, qianchuanbinding.ErrNotFound
	}
	return r, nil
}

func (f *fakeBindingsSvc) List(_ context.Context, projectID uuid.UUID) ([]*qianchuanbinding.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []*qianchuanbinding.Record{}
	for _, r := range f.rows {
		if r.ProjectID == projectID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeBindingsSvc) Update(_ context.Context, id uuid.UUID, in qianchuanbinding.UpdateInput) (*qianchuanbinding.Record, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.rows[id]
	if !ok {
		return nil, qianchuanbinding.ErrNotFound
	}
	if in.DisplayName != nil {
		r.DisplayName = *in.DisplayName
	}
	if in.Status != nil {
		r.Status = *in.Status
	}
	return r, nil
}

func (f *fakeBindingsSvc) Delete(_ context.Context, id uuid.UUID) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.rows[id]; !ok {
		return qianchuanbinding.ErrNotFound
	}
	delete(f.rows, id)
	return nil
}

func (f *fakeBindingsSvc) Sync(_ context.Context, _ uuid.UUID) error { return nil }

func (f *fakeBindingsSvc) Test(_ context.Context, _ uuid.UUID) (*adsplatform.MetricSnapshot, error) {
	return &adsplatform.MetricSnapshot{}, nil
}

// recordingAudit captures emitted audit entries for assertions.
type recordingAudit struct {
	mu    sync.Mutex
	calls []struct {
		ProjectID uuid.UUID
		Actor     uuid.UUID
		Binding   uuid.UUID
		Action    string
	}
}

func (a *recordingAudit) Emit(_ context.Context, projectID, actor, bindingID uuid.UUID, action, _ string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, struct {
		ProjectID uuid.UUID
		Actor     uuid.UUID
		Binding   uuid.UUID
		Action    string
	}{projectID, actor, bindingID, action})
}

// mountFlat is a small adapter that registers the per-binding routes WITHOUT
// the appMiddleware.Require gate — handler tests should not require a real
// JWT/RBAC chain.
func mountFlat(e *echo.Echo, h *handler.QianchuanBindingsHandler) {
	g := e.Group("/api/v1/qianchuan/bindings")
	g.PATCH("/:id", h.Update)
	g.DELETE("/:id", h.Delete)
	g.POST("/:id/sync", h.Sync)
	g.POST("/:id/test", h.Test)
}

// mountProject mounts the project-scoped endpoints WITHOUT RBAC for tests.
func mountProject(e *echo.Echo, h *handler.QianchuanBindingsHandler) {
	g := e.Group("/api/v1/projects/:pid")
	g.GET("/qianchuan/bindings", h.List)
	g.POST("/qianchuan/bindings", h.Create)
}

func TestQianchuanBindingsHandler_Create_201(t *testing.T) {
	svc := newFakeBindingsSvc()
	audit := &recordingAudit{}
	h := handler.NewQianchuanBindingsHandler(svc, audit)
	e := echo.New()
	mountProject(e, h)

	body := map[string]any{
		"advertiser_id":            "A1",
		"aweme_id":                 "W1",
		"display_name":             "店铺A",
		"access_token_secret_ref":  "qc.A1.access",
		"refresh_token_secret_ref": "qc.A1.refresh",
	}
	buf, _ := json.Marshal(body)
	pid := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid.String()+"/qianchuan/bindings", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if len(audit.calls) != 1 || audit.calls[0].Action != "qianchuan_binding.create" {
		t.Fatalf("audit calls=%+v", audit.calls)
	}
}

func TestQianchuanBindingsHandler_Create_RejectsMissingFields(t *testing.T) {
	svc := newFakeBindingsSvc()
	h := handler.NewQianchuanBindingsHandler(svc, nil)
	e := echo.New()
	mountProject(e, h)

	body := map[string]any{
		// missing advertiser_id and secret refs
		"display_name": "店铺A",
	}
	buf, _ := json.Marshal(body)
	pid := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid.String()+"/qianchuan/bindings", bytes.NewReader(buf))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestQianchuanBindingsHandler_List_ReturnsRows(t *testing.T) {
	svc := newFakeBindingsSvc()
	pid := uuid.New()
	svc.rows[uuid.New()] = &qianchuanbinding.Record{
		ID: uuid.New(), ProjectID: pid, AdvertiserID: "A1",
		Status: qianchuanbinding.StatusActive,
	}
	h := handler.NewQianchuanBindingsHandler(svc, nil)
	e := echo.New()
	mountProject(e, h)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid.String()+"/qianchuan/bindings", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["advertiser_id"] != "A1" {
		t.Fatalf("rows=%+v", rows)
	}
}

func TestQianchuanBindingsHandler_Sync_NoContent(t *testing.T) {
	svc := newFakeBindingsSvc()
	h := handler.NewQianchuanBindingsHandler(svc, nil)
	e := echo.New()
	mountFlat(e, h)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/qianchuan/bindings/"+uuid.New().String()+"/sync", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
