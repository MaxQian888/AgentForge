package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/agentforge/server/internal/handler"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type fakeRunViewService struct {
	listResult  *service.UnifiedRunListResult
	listErr     error
	lastFilter  service.UnifiedRunListFilter
	lastCursor  string
	lastLimit   int
	detail      *service.UnifiedRunDetail
	detailErr   error
	detailCalls int
}

func (f *fakeRunViewService) ListRuns(_ context.Context, _ uuid.UUID, filter service.UnifiedRunListFilter, cursor string, limit int) (*service.UnifiedRunListResult, error) {
	f.lastFilter = filter
	f.lastCursor = cursor
	f.lastLimit = limit
	if f.listErr != nil {
		return nil, f.listErr
	}
	if f.listResult != nil {
		return f.listResult, nil
	}
	return &service.UnifiedRunListResult{Rows: []service.UnifiedRunRow{}}, nil
}

func (f *fakeRunViewService) GetRun(_ context.Context, _ uuid.UUID, engine string, runID uuid.UUID) (*service.UnifiedRunDetail, error) {
	f.detailCalls++
	_ = engine
	_ = runID
	if f.detailErr != nil {
		return nil, f.detailErr
	}
	return f.detail, nil
}

func newRunViewContext(t *testing.T, method, target string) (*echo.Echo, echo.Context, *httptest.ResponseRecorder) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, uuid.New())
	return e, c, rec
}

func TestWorkflowRunView_List_DefaultEnvelope(t *testing.T) {
	svc := &fakeRunViewService{listResult: &service.UnifiedRunListResult{
		Rows: []service.UnifiedRunRow{{
			Engine: service.UnifiedRunEngineDAG,
			RunID:  uuid.New().String(),
			Status: service.UnifiedRunStatusRunning,
		}},
		NextCursor: "cur-2",
		Summary:    service.UnifiedRunSummary{Running: 1},
	}}
	h := handler.NewWorkflowRunViewHandler(svc)
	_, c, rec := newRunViewContext(t, http.MethodGet, "/?engine=dag&limit=10")

	if err := h.List(c); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rows, ok := out["rows"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("rows shape wrong: %+v", out["rows"])
	}
	if out["nextCursor"] != "cur-2" {
		t.Errorf("nextCursor = %v", out["nextCursor"])
	}
	if svc.lastFilter.Engine != service.UnifiedRunEngineDAG {
		t.Errorf("engine filter not forwarded: %+v", svc.lastFilter)
	}
	if svc.lastLimit != 10 {
		t.Errorf("limit = %d, want 10", svc.lastLimit)
	}
}

func TestWorkflowRunView_List_InvalidEngine(t *testing.T) {
	svc := &fakeRunViewService{}
	h := handler.NewWorkflowRunViewHandler(svc)
	_, c, rec := newRunViewContext(t, http.MethodGet, "/?engine=nonsense")
	if err := h.List(c); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}

func TestWorkflowRunView_List_CursorRoundtrip(t *testing.T) {
	svc := &fakeRunViewService{listResult: &service.UnifiedRunListResult{
		Rows:       []service.UnifiedRunRow{},
		NextCursor: "next-cursor-abc",
	}}
	h := handler.NewWorkflowRunViewHandler(svc)

	params := url.Values{}
	params.Set("cursor", "incoming-cursor")
	params.Set("status", "running")
	_, c, rec := newRunViewContext(t, http.MethodGet, "/?"+params.Encode())
	if err := h.List(c); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.lastCursor != "incoming-cursor" {
		t.Errorf("cursor = %q", svc.lastCursor)
	}
	if len(svc.lastFilter.Statuses) != 1 || svc.lastFilter.Statuses[0] != "running" {
		t.Errorf("statuses = %+v", svc.lastFilter.Statuses)
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["nextCursor"] != "next-cursor-abc" {
		t.Errorf("nextCursor not roundtripped: %v", out["nextCursor"])
	}
}

func TestWorkflowRunView_Detail_DAG(t *testing.T) {
	runID := uuid.New()
	svc := &fakeRunViewService{detail: &service.UnifiedRunDetail{
		Row:  service.UnifiedRunRow{Engine: service.UnifiedRunEngineDAG, RunID: runID.String()},
		Body: map[string]any{"execution": map[string]any{"id": runID.String()}},
	}}
	h := handler.NewWorkflowRunViewHandler(svc)
	e, c, rec := newRunViewContext(t, http.MethodGet, "/")
	c.SetPath("/projects/:pid/workflow-runs/:engine/:id")
	c.SetParamNames("engine", "id")
	c.SetParamValues("dag", runID.String())
	_ = e

	if err := h.Detail(c); err != nil {
		t.Fatalf("Detail error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	row, _ := out["row"].(map[string]any)
	if row["engine"] != "dag" {
		t.Errorf("row.engine = %v", row["engine"])
	}
}

func TestWorkflowRunView_Detail_Plugin(t *testing.T) {
	runID := uuid.New()
	svc := &fakeRunViewService{detail: &service.UnifiedRunDetail{
		Row:  service.UnifiedRunRow{Engine: service.UnifiedRunEnginePlugin, RunID: runID.String()},
		Body: map[string]any{"plugin_id": "p1"},
	}}
	h := handler.NewWorkflowRunViewHandler(svc)
	_, c, rec := newRunViewContext(t, http.MethodGet, "/")
	c.SetParamNames("engine", "id")
	c.SetParamValues("plugin", runID.String())

	if err := h.Detail(c); err != nil {
		t.Fatalf("Detail error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestWorkflowRunView_Detail_UnknownEngineRejected(t *testing.T) {
	svc := &fakeRunViewService{}
	h := handler.NewWorkflowRunViewHandler(svc)
	_, c, rec := newRunViewContext(t, http.MethodGet, "/")
	c.SetParamNames("engine", "id")
	c.SetParamValues("mystery", uuid.New().String())

	if err := h.Detail(c); err != nil {
		t.Fatalf("Detail error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if svc.detailCalls != 0 {
		t.Errorf("service should not have been called for unknown engine")
	}
}

func TestWorkflowRunView_Detail_NotFound(t *testing.T) {
	svc := &fakeRunViewService{detailErr: repository.ErrNotFound}
	h := handler.NewWorkflowRunViewHandler(svc)
	_, c, rec := newRunViewContext(t, http.MethodGet, "/")
	c.SetParamNames("engine", "id")
	c.SetParamValues("dag", uuid.New().String())

	if err := h.Detail(c); err != nil {
		t.Fatalf("Detail error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}
