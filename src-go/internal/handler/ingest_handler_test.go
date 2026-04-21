// src-go/internal/handler/ingest_handler_test.go
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/agentforge/server/internal/handler"
	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
)

type stubLogService struct {
	calls []model.CreateLogInput
}

func (s *stubLogService) CreateLog(_ context.Context, in model.CreateLogInput) (*model.Log, error) {
	s.calls = append(s.calls, in)
	return &model.Log{ID: uuid.New(), ProjectID: in.ProjectID}, nil
}

func TestIngest_SingleObject_PersistsWithTraceID(t *testing.T) {
	svc := &stubLogService{}
	h := handler.NewIngestHandler(svc)

	projectID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"projectId": projectID.String(),
		"tab":       "system",
		"level":     "warn",
		"source":    "ts-bridge",
		"summary":   "plugin load failed",
		"detail":    map[string]any{"plugin": "acme"},
	})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(req.WithContext(applog.WithTrace(req.Context(), "tr_single0000000000000000")))

	if err := h.Ingest(c); err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", rec.Code)
	}
	if len(svc.calls) != 1 {
		t.Fatalf("want 1 call, got %d", len(svc.calls))
	}
	got := svc.calls[0]
	if got.Summary != "plugin load failed" {
		t.Fatalf("summary: %q", got.Summary)
	}
	if got.Detail["trace_id"] != "tr_single0000000000000000" {
		t.Fatalf("detail.trace_id: %v", got.Detail["trace_id"])
	}
	if got.Detail["source"] != "ts-bridge" {
		t.Fatalf("detail.source: %v", got.Detail["source"])
	}
	if got.Detail["plugin"] != "acme" {
		t.Fatalf("detail.plugin preserved: %v", got.Detail["plugin"])
	}
}

func TestIngest_Batch_PersistsAll(t *testing.T) {
	svc := &stubLogService{}
	h := handler.NewIngestHandler(svc)

	batch := []map[string]any{
		{"tab": "system", "level": "warn", "source": "frontend", "summary": "a"},
		{"tab": "system", "level": "error", "source": "frontend", "summary": "b"},
	}
	body, _ := json.Marshal(batch)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetRequest(req.WithContext(applog.WithTrace(req.Context(), "tr_batch00000000000000000")))

	if err := h.Ingest(c); err != nil {
		t.Fatalf("handler err: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("want 202, got %d", rec.Code)
	}
	if len(svc.calls) != 2 {
		t.Fatalf("want 2 calls, got %d", len(svc.calls))
	}
	if svc.calls[0].Summary != "a" || svc.calls[1].Summary != "b" {
		t.Fatal("summaries out of order")
	}
	if svc.calls[0].Detail["trace_id"] != "tr_batch00000000000000000" {
		t.Fatal("batch entry missing trace_id")
	}
}

func TestIngest_RejectsInvalidLevel(t *testing.T) {
	svc := &stubLogService{}
	h := handler.NewIngestHandler(svc)
	body, _ := json.Marshal(map[string]any{"tab": "system", "level": "bogus", "summary": "x"})
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	if err := h.Ingest(e.NewContext(req, rec)); err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
	if len(svc.calls) != 0 {
		t.Fatalf("no calls on invalid input, got %d", len(svc.calls))
	}
}

func TestIngest_RejectsMissingSummary(t *testing.T) {
	svc := &stubLogService{}
	h := handler.NewIngestHandler(svc)
	body, _ := json.Marshal(map[string]any{"tab": "system", "level": "warn"})
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	if err := h.Ingest(e.NewContext(req, rec)); err != nil {
		t.Fatalf("err: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}
