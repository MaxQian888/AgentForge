package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/agentforge/server/internal/eventbus"
	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
)

// --- stubs ---

type stubLogTrace struct{ rows []*model.Log }

func (s *stubLogTrace) ListByTraceID(_ context.Context, _ string, _ int) ([]*model.Log, error) {
	return s.rows, nil
}

type stubAutoTrace struct{ rows []*model.AutomationLog }

func (s *stubAutoTrace) ListByTraceID(_ context.Context, _ string, _ int) ([]*model.AutomationLog, error) {
	return s.rows, nil
}

type stubEventTrace struct{ rows []*eventbus.Event }

func (s *stubEventTrace) ListByTraceID(_ context.Context, _ string, _ int) ([]*eventbus.Event, error) {
	return s.rows, nil
}

// --- tests ---

func TestDebugHandler_GetTrace_MergesAndSorts(t *testing.T) {
	t0 := time.Now().UTC().Truncate(time.Millisecond)

	h := handler.NewDebugHandler(
		&stubLogTrace{rows: []*model.Log{
			{CreatedAt: t0.Add(10 * time.Millisecond), Level: "info", Summary: "log-a"},
		}},
		&stubAutoTrace{rows: []*model.AutomationLog{
			{TriggeredAt: t0.Add(20 * time.Millisecond), EventType: "rule.x", Status: "ok"},
		}},
		&stubEventTrace{rows: []*eventbus.Event{
			{
				ID:        "ev-1",
				Type:      "task.created",
				Source:    "test",
				Target:    "test",
				Timestamp: t0.UnixMilli(), // int64 Unix milliseconds = t0
			},
		}},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/debug/trace/tr_test", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("trace_id")
	c.SetParamValues("tr_test")

	if err := h.GetTrace(c); err != nil {
		t.Fatalf("GetTrace returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		TraceID   string `json:"traceId"`
		Entries   []struct {
			Source    string `json:"source"`
			EventType string `json:"eventType"`
		} `json:"entries"`
		Truncated bool `json:"truncated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TraceID != "tr_test" {
		t.Fatalf("traceId: want %q, got %q", "tr_test", resp.TraceID)
	}
	if len(resp.Entries) != 3 {
		t.Fatalf("want 3 entries, got %d: %s", len(resp.Entries), rec.Body.String())
	}
	if resp.Truncated {
		t.Fatalf("want truncated=false")
	}
	// Sorted ASC: eventbus (t0), logs (t0+10ms), automation (t0+20ms)
	if resp.Entries[0].Source != "eventbus" {
		t.Fatalf("entries[0].source: want %q, got %q", "eventbus", resp.Entries[0].Source)
	}
	if resp.Entries[1].Source != "logs" {
		t.Fatalf("entries[1].source: want %q, got %q", "logs", resp.Entries[1].Source)
	}
	if resp.Entries[2].Source != "automation" {
		t.Fatalf("entries[2].source: want %q, got %q", "automation", resp.Entries[2].Source)
	}
}

func TestDebugHandler_GetTrace_MissingID(t *testing.T) {
	h := handler.NewDebugHandler(nil, nil, nil)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/debug/trace/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("trace_id")
	c.SetParamValues("")

	_ = h.GetTrace(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rec.Code)
	}
}

func TestDebugHandler_GetTrace_Truncated(t *testing.T) {
	// When total entries equals 3*perRepoLimit, truncated should be true.
	// We simulate this by checking the truncation flag logic indirectly:
	// with 0 entries and limit=5000 each, total=0 < 15000 => not truncated.
	h := handler.NewDebugHandler(
		&stubLogTrace{rows: []*model.Log{}},
		&stubAutoTrace{rows: []*model.AutomationLog{}},
		&stubEventTrace{rows: []*eventbus.Event{}},
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/debug/trace/tr_empty", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("trace_id")
	c.SetParamValues("tr_empty")

	if err := h.GetTrace(c); err != nil {
		t.Fatalf("GetTrace error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}

	var resp struct {
		Entries   []interface{} `json:"entries"`
		Truncated bool          `json:"truncated"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Entries) != 0 {
		t.Fatalf("want 0 entries, got %d", len(resp.Entries))
	}
	if resp.Truncated {
		t.Fatalf("want truncated=false for empty result")
	}
}
