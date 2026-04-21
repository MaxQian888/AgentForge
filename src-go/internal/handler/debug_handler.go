package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/agentforge/server/internal/eventbus"
	"github.com/agentforge/server/internal/model"
)

// Narrow interfaces for testability; real repos satisfy these.
type LogTraceQuery interface {
	ListByTraceID(ctx context.Context, traceID string, limit int) ([]*model.Log, error)
}

type AutomationLogTraceQuery interface {
	ListByTraceID(ctx context.Context, traceID string, limit int) ([]*model.AutomationLog, error)
}

type EventTraceQuery interface {
	ListByTraceID(ctx context.Context, traceID string, limit int) ([]*eventbus.Event, error)
}

// DebugHandler provides trace timeline and metrics endpoints.
type DebugHandler struct {
	logs       LogTraceQuery
	automation AutomationLogTraceQuery
	events     EventTraceQuery
}

// NewDebugHandler constructs a DebugHandler with the given query backends.
func NewDebugHandler(logs LogTraceQuery, automation AutomationLogTraceQuery, events EventTraceQuery) *DebugHandler {
	return &DebugHandler{logs: logs, automation: automation, events: events}
}

type timelineEntry struct {
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"` // "logs" | "automation" | "eventbus"
	Level     string                 `json:"level,omitempty"`
	EventType string                 `json:"eventType,omitempty"`
	Summary   string                 `json:"summary,omitempty"`
	Detail    map[string]interface{} `json:"detail,omitempty"`
}

type timelineResponse struct {
	TraceID   string          `json:"traceId"`
	Entries   []timelineEntry `json:"entries"`
	Truncated bool            `json:"truncated"`
}

const perRepoLimit = 5000

// GetTrace handles GET /debug/trace/:trace_id — merges log, automation, and event entries
// into a single timeline sorted by timestamp ASC.
func (h *DebugHandler) GetTrace(c echo.Context) error {
	traceID := c.Param("trace_id")
	if traceID == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "trace_id required"})
	}
	ctx := c.Request().Context()

	logs, err := h.logs.ListByTraceID(ctx, traceID, perRepoLimit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query logs failed"})
	}
	autos, err := h.automation.ListByTraceID(ctx, traceID, perRepoLimit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query automation_logs failed"})
	}
	evs, err := h.events.ListByTraceID(ctx, traceID, perRepoLimit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "query events failed"})
	}

	entries := make([]timelineEntry, 0, len(logs)+len(autos)+len(evs))

	for _, l := range logs {
		entries = append(entries, timelineEntry{
			Timestamp: l.CreatedAt,
			Source:    "logs",
			Level:     l.Level,
			EventType: l.EventType,
			Summary:   l.Summary,
			Detail:    unmarshalJSONBytes(l.Detail),
		})
	}

	for _, a := range autos {
		entries = append(entries, timelineEntry{
			Timestamp: a.TriggeredAt, // time.Time field
			Source:    "automation",
			EventType: a.EventType,
			Summary:   a.Status,
			Detail:    unmarshalJSONString(a.Detail),
		})
	}

	for _, e := range evs {
		entries = append(entries, timelineEntry{
			Timestamp: time.UnixMilli(e.Timestamp), // int64 Unix milliseconds
			Source:    "eventbus",
			EventType: e.Type,
			Detail:    mergeEventMeta(e),
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.Before(entries[j].Timestamp)
	})

	total := len(logs) + len(autos) + len(evs)
	return c.JSON(http.StatusOK, timelineResponse{
		TraceID:   traceID,
		Entries:   entries,
		Truncated: total >= perRepoLimit*3,
	})
}

// MetricsHandler returns the Prometheus exposition endpoint wrapped for Echo.
func MetricsHandler() echo.HandlerFunc {
	return echo.WrapHandler(promhttp.Handler())
}

// unmarshalJSONBytes attempts to unmarshal raw JSON bytes; returns nil on empty or error.
func unmarshalJSONBytes(raw json.RawMessage) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return m
}

// unmarshalJSONString attempts to unmarshal a JSON string; returns nil on empty or error.
func unmarshalJSONString(raw string) map[string]interface{} {
	if raw == "" {
		return nil
	}
	var m map[string]interface{}
	_ = json.Unmarshal([]byte(raw), &m)
	return m
}

// mergeEventMeta combines an event's Metadata map and decoded Payload into a single detail map.
func mergeEventMeta(e *eventbus.Event) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range e.Metadata {
		out[k] = v
	}
	if len(e.Payload) > 0 {
		var p map[string]interface{}
		if json.Unmarshal(e.Payload, &p) == nil {
			out["payload"] = p
		}
	}
	return out
}
