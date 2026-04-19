package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/trigger"
)

type mockTriggerRouter struct {
	lastEvent trigger.Event
	started   int
	err       error
}

func (m *mockTriggerRouter) Route(_ context.Context, ev trigger.Event) (int, error) {
	m.lastEvent = ev
	return m.started, m.err
}

func setupTriggerHandler(router *mockTriggerRouter) *echo.Echo {
	e := echo.New()
	h := handler.NewTriggerHandler(router)
	h.RegisterRoutes(e)
	return e
}

func jsonBody(t *testing.T, v any) *bytes.Buffer {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewBuffer(b)
}

func TestTriggerHandler_HandleIMEvent_Success(t *testing.T) {
	router := &mockTriggerRouter{started: 1}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{
		"platform": "feishu", "command": "/review", "content": "/review http://example.com",
		"args": []any{"http://example.com"}, "chatId": "chat-1",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("status: want 202, got %d (body=%s)", rec.Code, rec.Body.String())
	}
	if router.lastEvent.Source != model.TriggerSourceIM {
		t.Errorf("source: want im, got %s", router.lastEvent.Source)
	}
	if router.lastEvent.Data["command"] != "/review" {
		t.Errorf("command: want /review, got %v", router.lastEvent.Data["command"])
	}
}

func TestTriggerHandler_HandleIMEvent_NoMatchReturns404(t *testing.T) {
	router := &mockTriggerRouter{started: 0}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{"platform": "feishu", "command": "/unknown"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status: want 404, got %d", rec.Code)
	}
}

func TestTriggerHandler_HandleIMEvent_MissingPlatform(t *testing.T) {
	router := &mockTriggerRouter{}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{"command": "/review"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}
}

func TestTriggerHandler_HandleIMEvent_InvalidBody(t *testing.T) {
	router := &mockTriggerRouter{}
	e := setupTriggerHandler(router)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events",
		bytes.NewBufferString("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status: want 400, got %d", rec.Code)
	}
}

func TestTriggerHandler_HandleIMEvent_RouterError_NoExecutions(t *testing.T) {
	router := &mockTriggerRouter{started: 0, err: errors.New("boom")}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{"platform": "feishu", "command": "/review"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status: want 500, got %d", rec.Code)
	}
}

func TestTriggerHandler_HandleIMEvent_RouterError_PartialSuccess(t *testing.T) {
	router := &mockTriggerRouter{started: 1, err: errors.New("one of them failed")}
	e := setupTriggerHandler(router)

	body := jsonBody(t, map[string]any{"platform": "feishu", "command": "/review"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/triggers/im/events", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	// Partial success → 202, not 500.
	if rec.Code != http.StatusAccepted {
		t.Errorf("status: want 202 on partial success, got %d", rec.Code)
	}
}
