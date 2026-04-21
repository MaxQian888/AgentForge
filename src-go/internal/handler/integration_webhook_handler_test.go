package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// stubWebhookPluginRuntime satisfies the IntegrationPluginInvoker interface
// the IntegrationWebhookHandler depends on.
type stubWebhookPluginRuntime struct {
	calledPluginID  string
	calledOperation string
	calledWith      map[string]any
	returnEvent     map[string]any
	returnErr       error
}

func (s *stubWebhookPluginRuntime) InvokeIntegrationPlugin(
	ctx context.Context, pluginID, operation string, payload map[string]any,
) (map[string]any, error) {
	s.calledPluginID = pluginID
	s.calledOperation = operation
	s.calledWith = payload
	return s.returnEvent, s.returnErr
}

type stubEventPublisher struct {
	published []map[string]any
}

func (s *stubEventPublisher) PublishRaw(ctx context.Context, eventType string, payload map[string]any) error {
	s.published = append(s.published, map[string]any{"type": eventType, "payload": payload})
	return nil
}

func TestIntegrationWebhookHandler_CallsPluginAndPublishes(t *testing.T) {
	e := echo.New()
	stub := &stubWebhookPluginRuntime{
		returnEvent: map[string]any{
			"event_type": "vcs.pull_request.opened",
			"payload":    map[string]any{"pr_number": 42},
		},
	}
	bus := &stubEventPublisher{}
	h := NewIntegrationWebhookHandler(stub, bus)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"action":"opened"}`))
	req.Header.Set("Content-Type", "application/json")
	// http.Header canonicalizes "X-GitHub-Event" to "X-Github-Event" — that's
	// how real GitHub deliveries land in handlers, and what the plugin sees.
	req.Header.Set("X-GitHub-Event", "pull_request")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("github-actions-adapter")

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if stub.calledPluginID != "github-actions-adapter" {
		t.Errorf("plugin id = %q, want github-actions-adapter", stub.calledPluginID)
	}
	if stub.calledOperation != "handle_webhook" {
		t.Errorf("operation = %q, want handle_webhook", stub.calledOperation)
	}
	body, _ := stub.calledWith["body"].(string)
	if body != `{"action":"opened"}` {
		t.Errorf("body forwarded = %q", body)
	}
	headers, _ := stub.calledWith["headers"].(map[string]string)
	if headers["X-Github-Event"] != "pull_request" {
		t.Errorf("headers forwarded missing X-Github-Event: %v", headers)
	}
	if len(bus.published) != 1 {
		t.Fatalf("expected 1 published event, got %d", len(bus.published))
	}
	if bus.published[0]["type"] != "vcs.pull_request.opened" {
		t.Errorf("event type = %v", bus.published[0]["type"])
	}
}

func TestIntegrationWebhookHandler_PluginReturnsNoEvent(t *testing.T) {
	e := echo.New()
	stub := &stubWebhookPluginRuntime{returnEvent: nil}
	bus := &stubEventPublisher{}
	h := NewIntegrationWebhookHandler(stub, bus)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("github-actions-adapter")

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(bus.published) != 0 {
		t.Errorf("expected no published events, got %d", len(bus.published))
	}
}

func TestIntegrationWebhookHandler_PluginErrorReturns400(t *testing.T) {
	e := echo.New()
	stub := &stubWebhookPluginRuntime{returnErr: context.DeadlineExceeded}
	bus := &stubEventPublisher{}
	h := NewIntegrationWebhookHandler(stub, bus)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("github-actions-adapter")

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	if len(bus.published) != 0 {
		t.Errorf("expected no published events on plugin error")
	}
}
