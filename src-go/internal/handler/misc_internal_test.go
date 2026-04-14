package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	bridge "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/service"
)

type bridgeHealthReaderStub struct {
	snapshot service.BridgeHealthSnapshot
}

func (s bridgeHealthReaderStub) Snapshot() service.BridgeHealthSnapshot {
	return s.snapshot
}

func TestBridgeHealthHandlerGet(t *testing.T) {
	e := echo.New()

	handlerWithNil := NewBridgeHealthHandler(nil)
	req := httptest.NewRequest(http.MethodGet, "/bridge/health", nil)
	rec := httptest.NewRecorder()
	if err := handlerWithNil.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Get(nil) error = %v", err)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Get(nil) status = %d, want 503", rec.Code)
	}

	handlerWithHealth := NewBridgeHealthHandler(bridgeHealthReaderStub{
		snapshot: service.BridgeHealthSnapshot{Status: service.BridgeStatusReady},
	})
	req = httptest.NewRequest(http.MethodGet, "/bridge/health", nil)
	rec = httptest.NewRecorder()
	if err := handlerWithHealth.Get(e.NewContext(req, rec)); err != nil {
		t.Fatalf("Get(ready) error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("Get(ready) status = %d, want 200", rec.Code)
	}
}

func TestParseStatsParams(t *testing.T) {
	e := echo.New()
	projectID := "11111111-1111-1111-1111-111111111111"

	req := httptest.NewRequest(http.MethodGet, "/stats?from=2026-03-01&to=2026-03-10&projectId="+projectID, nil)
	c := e.NewContext(req, httptest.NewRecorder())
	from, to, parsedProjectID, err := parseStatsParams(c)
	if err != nil {
		t.Fatalf("parseStatsParams(valid) error = %v", err)
	}
	if from.Format("2006-01-02") != "2026-03-01" {
		t.Fatalf("from = %s, want 2026-03-01", from.Format("2006-01-02"))
	}
	wantTo := time.Date(2026, 3, 10, 23, 59, 59, int(time.Second-time.Nanosecond), time.UTC)
	if !to.Equal(wantTo) {
		t.Fatalf("to = %s, want %s", to.Format(time.RFC3339Nano), wantTo.Format(time.RFC3339Nano))
	}
	if parsedProjectID == nil || parsedProjectID.String() != projectID {
		t.Fatalf("projectID = %#v, want %s", parsedProjectID, projectID)
	}

	req = httptest.NewRequest(http.MethodGet, "/stats?from=bad-date", nil)
	c = e.NewContext(req, httptest.NewRecorder())
	if _, _, _, err := parseStatsParams(c); err == nil {
		t.Fatal("parseStatsParams(invalid from) expected error")
	}

	req = httptest.NewRequest(http.MethodGet, "/stats?projectId=bad-id", nil)
	c = e.NewContext(req, httptest.NewRecorder())
	if _, _, _, err := parseStatsParams(c); err == nil {
		t.Fatal("parseStatsParams(invalid projectId) expected error")
	}
}

func TestProjectCatalogHelpers(t *testing.T) {
	selection := fallbackCodingAgentSelection()
	if selection.Runtime == "" {
		t.Fatalf("fallbackCodingAgentSelection() = %#v, want non-empty runtime", selection)
	}

	defaultCatalog := projectCatalogFromBridge(nil, selection)
	if defaultCatalog == nil || defaultCatalog.DefaultSelection.Runtime != selection.Runtime {
		t.Fatalf("projectCatalogFromBridge(nil) = %#v", defaultCatalog)
	}

	catalog := projectCatalogFromBridge(&bridge.RuntimeCatalogResponse{
		DefaultRuntime: "",
		Runtimes: []bridge.RuntimeCatalogEntryDTO{{
			Key:                 "codex",
			Label:               "Codex",
			DefaultProvider:     "openai",
			CompatibleProviders: []string{"openai", "codex"},
			DefaultModel:        "gpt-5-codex",
			Available:           true,
			Diagnostics: []bridge.RuntimeDiagnosticDTO{{
				Code:     "missing_token",
				Message:  "token missing",
				Blocking: true,
			}},
		}},
	}, selection)
	if catalog.DefaultRuntime != "codex" {
		t.Fatalf("catalog.DefaultRuntime = %q, want %q", catalog.DefaultRuntime, "codex")
	}
	if catalog.DefaultSelection.Runtime != "codex" {
		t.Fatalf("catalog.DefaultSelection = %#v, want runtime codex", catalog.DefaultSelection)
	}
	if len(catalog.Runtimes) != 1 || len(catalog.Runtimes[0].Diagnostics) != 1 || catalog.Runtimes[0].Diagnostics[0].Code != "missing_token" {
		t.Fatalf("catalog.Runtimes = %#v", catalog.Runtimes)
	}
}

func TestReviewHandlerHelpers(t *testing.T) {
	e := echo.New()
	h := NewReviewHandler(nil)
	if h.WithAggregationService(struct{}{}) != h {
		t.Fatal("WithAggregationService() should return the same handler")
	}

	if actor := resolveReviewActor(nil); actor != "api" {
		t.Fatalf("resolveReviewActor(nil) = %q, want api", actor)
	}

	req := httptest.NewRequest(http.MethodGet, "/reviews", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("userId", "  reviewer-1 ")
	if actor := resolveReviewActor(c); actor != "reviewer-1" {
		t.Fatalf("resolveReviewActor(c) = %q, want reviewer-1", actor)
	}

	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if err := h.handleServiceError(c, service.ErrReviewNotFound); err != nil {
		t.Fatalf("handleServiceError(not found) error = %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("not found status = %d, want 404", rec.Code)
	}

	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if err := h.handleServiceError(c, service.ErrReviewInvalidTransition); err != nil {
		t.Fatalf("handleServiceError(conflict) error = %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("conflict status = %d, want 409", rec.Code)
	}

	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	if err := h.handleServiceError(c, errors.New("boom")); err != nil {
		t.Fatalf("handleServiceError(default) error = %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("default status = %d, want 500", rec.Code)
	}
}
