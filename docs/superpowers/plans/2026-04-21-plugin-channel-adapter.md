# Plugin Integration Expansion — CI/CD Webhooks & Notifications

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add four Integration Plugins: `github-actions-adapter` (GitHub webhook → EventBus), `generic-webhook-adapter` (any HTTP webhook → EventBus), `email-adapter` (SMTP outbound), and `notification-fanout` (route notifications to IM Bridge + email).

**Architecture:** All IM platform support (Feishu, Slack, DingTalk, Discord, etc.) already lives in `src-im-bridge/` and must NOT be duplicated. These four plugins handle CI/CD event ingestion and notification delivery — complementary, non-overlapping roles. CI/CD WASM plugins receive HTTP webhooks via a new Go orchestrator route `POST /api/v1/integrations/:id/webhook`, validate the payload, and publish typed events to the EventBus. `notification-fanout` is a firstparty-inproc plugin that routes notifications through the existing IM Bridge (`POST /api/v1/im/notify`) for IM channels.

**Tech Stack:** Go 1.23+, wazero WASM runtime, existing `pluginsdk` package (`src-go/plugin-sdk-go/`), `src-go/internal/eventbus/`, `src-go/internal/handler/`, SMTP via Go standard library.

---

## File Map

| Action | Path | Responsibility |
|--------|------|---------------|
| Create | `src-go/internal/handler/integration_webhook_handler.go` | HTTP route `POST /api/v1/integrations/:id/webhook` — receives raw body+headers, calls WASM plugin, publishes EventBus event |
| Create | `src-go/internal/handler/integration_webhook_handler_test.go` | Handler tests |
| Create | `src-go/cmd/github-actions-adapter/main.go` | WASM plugin: validate HMAC-SHA256, map GitHub event types |
| Create | `src-go/cmd/github-actions-adapter/main_test.go` | Plugin unit tests |
| Create | `plugins/integrations/github-actions-adapter/manifest.yaml` | Manifest |
| Create | `src-go/cmd/generic-webhook-adapter/main.go` | WASM plugin: accept any POST, publish with configurable event type |
| Create | `src-go/cmd/generic-webhook-adapter/main_test.go` | Plugin unit tests |
| Create | `plugins/integrations/generic-webhook-adapter/manifest.yaml` | Manifest |
| Create | `src-go/cmd/email-adapter/main.go` | WASM plugin: SMTP send via operation `send_email` |
| Create | `src-go/cmd/email-adapter/main_test.go` | Plugin unit tests |
| Create | `plugins/integrations/email-adapter/manifest.yaml` | Manifest |
| Create | `src-go/internal/server/notification_fanout_plugin.go` | firstparty-inproc: routes notification → IM Bridge + email-adapter |
| Create | `src-go/internal/server/notification_fanout_plugin_test.go` | Plugin tests |
| Create | `plugins/integrations/notification-fanout/manifest.yaml` | Manifest |

---

## Task 1: Webhook ingestion handler in Go Orchestrator

The WASM plugins cannot directly receive HTTP requests. The Go Orchestrator exposes a shared webhook endpoint that reads raw body and headers, calls the plugin's `handle_webhook` operation, then publishes the returned event to the EventBus.

**Files:**
- Create: `src-go/internal/handler/integration_webhook_handler.go`
- Create: `src-go/internal/handler/integration_webhook_handler_test.go`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/handler/integration_webhook_handler_test.go`:

```go
package handler_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

// stubWebhookPluginRuntime satisfies the interface IntegrationWebhookHandler needs.
type stubWebhookPluginRuntime struct {
	calledWith map[string]any
	returnEvent map[string]any
	returnErr  error
}

func (s *stubWebhookPluginRuntime) InvokeIntegrationPlugin(
	ctx context.Context, pluginID, operation string, payload map[string]any,
) (map[string]any, error) {
	s.calledWith = payload
	return s.returnEvent, s.returnErr
}

type stubEventBus struct {
	published []map[string]any
}

func (s *stubEventBus) PublishRaw(ctx context.Context, eventType string, payload map[string]any) error {
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
	bus := &stubEventBus{}
	h := NewIntegrationWebhookHandler(stub, bus)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"action":"opened"}`))
	req.Header.Set("Content-Type", "application/json")
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
	bus := &stubEventBus{}
	h := NewIntegrationWebhookHandler(stub, bus)

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("github-actions-adapter")

	if err := h.Handle(c); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// Plugin returned nothing — no event published, still 200
	if len(bus.published) != 0 {
		t.Errorf("expected no published events, got %d", len(bus.published))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/handler/... -run TestIntegrationWebhookHandler -v
```

Expected: compile error — `NewIntegrationWebhookHandler` not defined.

- [ ] **Step 3: Implement the handler**

Create `src-go/internal/handler/integration_webhook_handler.go`:

```go
package handler

import (
	"context"
	"io"
	"net/http"

	"github.com/labstack/echo/v4"
)

// IntegrationPluginInvoker is satisfied by the existing plugin runtime.
type IntegrationPluginInvoker interface {
	InvokeIntegrationPlugin(ctx context.Context, pluginID, operation string, payload map[string]any) (map[string]any, error)
}

// WebhookEventPublisher publishes a typed event to the EventBus.
type WebhookEventPublisher interface {
	PublishRaw(ctx context.Context, eventType string, payload map[string]any) error
}

// IntegrationWebhookHandler serves POST /api/v1/integrations/:id/webhook.
// It passes the raw request body and all headers to the plugin's "handle_webhook" operation,
// then publishes the returned event to the EventBus.
type IntegrationWebhookHandler struct {
	invoker   IntegrationPluginInvoker
	publisher WebhookEventPublisher
}

func NewIntegrationWebhookHandler(invoker IntegrationPluginInvoker, publisher WebhookEventPublisher) *IntegrationWebhookHandler {
	return &IntegrationWebhookHandler{invoker: invoker, publisher: publisher}
}

func (h *IntegrationWebhookHandler) Handle(c echo.Context) error {
	pluginID := c.Param("id")
	ctx := c.Request().Context()

	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cannot read body"})
	}

	headers := make(map[string]string, len(c.Request().Header))
	for k, v := range c.Request().Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	payload := map[string]any{
		"body":    string(body),
		"headers": headers,
	}

	result, err := h.invoker.InvokeIntegrationPlugin(ctx, pluginID, "handle_webhook", payload)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	// If the plugin returned an event_type, publish it.
	if result != nil {
		if eventType, ok := result["event_type"].(string); ok && eventType != "" {
			eventPayload, _ := result["payload"].(map[string]any)
			if eventPayload == nil {
				eventPayload = map[string]any{}
			}
			if pubErr := h.publisher.PublishRaw(ctx, eventType, eventPayload); pubErr != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": pubErr.Error()})
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
```

- [ ] **Step 4: Register the route in `src-go/internal/server/routes.go`**

Find the route registration file and add:

```go
// Under /api/v1/integrations
integrations := v1.Group("/integrations")
integrations.POST("/:id/webhook", integrationWebhookHandler.Handle)
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-go && go test ./internal/handler/... -run TestIntegrationWebhookHandler -v
```

Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add src-go/internal/handler/integration_webhook_handler.go src-go/internal/handler/integration_webhook_handler_test.go src-go/internal/server/routes.go
git commit -m "feat(handler): add webhook ingestion endpoint for integration plugins"
```

---

## Task 2: `github-actions-adapter` WASM plugin

Receives GitHub webhook deliveries, validates the HMAC-SHA256 `X-Hub-Signature-256` header, and maps GitHub event types to AgentForge EventBus event types.

**Files:**
- Create: `src-go/cmd/github-actions-adapter/main.go`
- Create: `src-go/cmd/github-actions-adapter/main_test.go`
- Create: `plugins/integrations/github-actions-adapter/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/cmd/github-actions-adapter/main_test.go`:

```go
package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	pluginsdk "agentforge/plugin-sdk-go"
)

func sig(secret, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestGitHubAdapter_PullRequestEvent(t *testing.T) {
	p := &githubActionsPlugin{}
	ctx := &pluginsdk.Context{}

	body := `{"action":"opened","number":42,"pull_request":{"title":"feat: add X"}}`
	secret := "test-secret"

	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body": body,
			"headers": map[string]any{
				"X-GitHub-Event":    "pull_request",
				"X-Hub-Signature-256": sig(secret, body),
			},
			"webhook_secret": secret,
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	et, _ := result.Data["event_type"].(string)
	if et != "vcs.pull_request.opened" {
		t.Errorf("event_type = %q, want vcs.pull_request.opened", et)
	}
}

func TestGitHubAdapter_PushEvent(t *testing.T) {
	p := &githubActionsPlugin{}
	ctx := &pluginsdk.Context{}
	body := `{"ref":"refs/heads/main","commits":[]}`
	secret := "secret"

	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body": body,
			"headers": map[string]any{
				"X-GitHub-Event":    "push",
				"X-Hub-Signature-256": sig(secret, body),
			},
			"webhook_secret": secret,
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	et, _ := result.Data["event_type"].(string)
	if et != "vcs.push" {
		t.Errorf("event_type = %q, want vcs.push", et)
	}
}

func TestGitHubAdapter_InvalidSignature(t *testing.T) {
	p := &githubActionsPlugin{}
	ctx := &pluginsdk.Context{}
	_, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body": `{}`,
			"headers": map[string]any{
				"X-GitHub-Event":    "push",
				"X-Hub-Signature-256": "sha256=badhash",
			},
			"webhook_secret": "real-secret",
		},
	})
	if err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestGitHubAdapter_Describe(t *testing.T) {
	p := &githubActionsPlugin{}
	ctx := &pluginsdk.Context{}
	d, err := p.Describe(ctx)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if d.ID != "github-actions-adapter" {
		t.Errorf("ID = %q", d.ID)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./cmd/github-actions-adapter/... -v
```

Expected: compile error.

- [ ] **Step 3: Implement `github-actions-adapter/main.go`**

Create `src-go/cmd/github-actions-adapter/main.go`:

```go
//go:build wasip1

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	pluginsdk "agentforge/plugin-sdk-go"
)

var runtime = pluginsdk.NewRuntime(&githubActionsPlugin{})

type githubActionsPlugin struct{}

func (p *githubActionsPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion: "agentforge/v1",
		Kind:       "IntegrationPlugin",
		ID:         "github-actions-adapter",
		Name:       "GitHub Actions Adapter",
		Version:    "0.1.0",
		Runtime:    "wasm",
		ABIVersion: pluginsdk.ABIVersion,
		Capabilities: []pluginsdk.Capability{
			{Name: "health"},
			{Name: "handle_webhook"},
		},
	}, nil
}

func (p *githubActionsPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (p *githubActionsPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.OKResult(map[string]any{"ok": true}), nil
}

// githubEventMap maps GitHub X-GitHub-Event + action pairs to AgentForge event types.
var githubEventMap = map[string]map[string]string{
	"pull_request": {
		"opened":      "vcs.pull_request.opened",
		"closed":      "vcs.pull_request.closed",
		"synchronize": "vcs.pull_request.updated",
		"reopened":    "vcs.pull_request.reopened",
	},
	"push": {
		"": "vcs.push",
	},
	"release": {
		"published": "vcs.release.published",
	},
	"workflow_run": {
		"completed": "vcs.workflow_run.completed",
	},
}

func (p *githubActionsPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch inv.Operation {
	case "health":
		return pluginsdk.OKResult(map[string]any{"ok": true}), nil

	case "handle_webhook":
		return p.handleWebhook(inv.Payload)

	default:
		return nil, fmt.Errorf("unsupported operation: %s", inv.Operation)
	}
}

func (p *githubActionsPlugin) handleWebhook(payload map[string]any) (*pluginsdk.Result, error) {
	body, _ := payload["body"].(string)
	secret, _ := payload["webhook_secret"].(string)
	headers, _ := payload["headers"].(map[string]any)

	// Validate HMAC-SHA256 signature.
	if secret != "" {
		sigHeader, _ := headers["X-Hub-Signature-256"].(string)
		if !validateSignature(secret, body, sigHeader) {
			return nil, fmt.Errorf("webhook signature validation failed")
		}
	}

	githubEvent, _ := headers["X-GitHub-Event"].(string)

	// Parse action from body.
	var bodyMap map[string]any
	_ = json.Unmarshal([]byte(body), &bodyMap)
	action, _ := bodyMap["action"].(string)

	// Map to AgentForge event type.
	eventType := mapGitHubEvent(githubEvent, action)
	if eventType == "" {
		// Unknown event type — return empty result (handler will not publish).
		return pluginsdk.OKResult(nil), nil
	}

	return pluginsdk.OKResult(map[string]any{
		"event_type": eventType,
		"payload":    bodyMap,
	}), nil
}

func validateSignature(secret, body, sigHeader string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sigHeader))
}

func mapGitHubEvent(event, action string) string {
	actions, ok := githubEventMap[event]
	if !ok {
		return ""
	}
	if et, ok := actions[action]; ok {
		return et
	}
	if et, ok := actions[""]; ok {
		return et
	}
	return ""
}

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return pluginsdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return pluginsdk.ExportRun(runtime) }
```

- [ ] **Step 4: Create manifest**

Create `plugins/integrations/github-actions-adapter/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: github-actions-adapter
  name: GitHub Actions Adapter
  version: 0.1.0
  description: Receives GitHub webhook events and publishes them to the AgentForge event bus
  tags: [builtin, integration, vcs, github, cicd]
spec:
  runtime: wasm
  module: ./dist/github-actions.wasm
  abiVersion: v1
  capabilities:
    - health
    - handle_webhook
  configSchema:
    type: object
    required: [webhook_secret]
    properties:
      webhook_secret:
        type: string
        description: HMAC-SHA256 secret configured in the GitHub webhook settings
      events:
        type: array
        description: GitHub event types to listen for (default all supported)
permissions:
  network:
    required: false
source:
  type: builtin
  path: ./plugins/integrations/github-actions-adapter/manifest.yaml
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-go && go test ./cmd/github-actions-adapter/... -v
```

Expected: PASS (4 tests).

- [ ] **Step 6: Commit**

```bash
git add src-go/cmd/github-actions-adapter/ plugins/integrations/github-actions-adapter/
git commit -m "feat(plugin): add github-actions-adapter — webhook → EventBus"
```

---

## Task 3: `generic-webhook-adapter` WASM plugin

Accepts any HTTP POST webhook and publishes it to the EventBus with a configurable event type. Useful for Zapier, internal tools, and any service not covered by a dedicated adapter.

**Files:**
- Create: `src-go/cmd/generic-webhook-adapter/main.go`
- Create: `src-go/cmd/generic-webhook-adapter/main_test.go`
- Create: `plugins/integrations/generic-webhook-adapter/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/cmd/generic-webhook-adapter/main_test.go`:

```go
package main

import (
	"testing"
	pluginsdk "agentforge/plugin-sdk-go"
)

func TestGenericWebhookAdapter_PublishesConfiguredEventType(t *testing.T) {
	p := &genericWebhookPlugin{}
	ctx := &pluginsdk.Context{}

	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body":    `{"foo":"bar"}`,
			"headers": map[string]any{"Content-Type": "application/json"},
			"event_type": "custom.deploy.finished", // configured event type
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	et, _ := result.Data["event_type"].(string)
	if et != "custom.deploy.finished" {
		t.Errorf("event_type = %q, want custom.deploy.finished", et)
	}
}

func TestGenericWebhookAdapter_DefaultEventType(t *testing.T) {
	p := &genericWebhookPlugin{}
	ctx := &pluginsdk.Context{}

	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body":    `{"x":1}`,
			"headers": map[string]any{},
			// no event_type configured → uses default
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	et, _ := result.Data["event_type"].(string)
	if et != "integration.webhook.received" {
		t.Errorf("event_type = %q, want integration.webhook.received", et)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./cmd/generic-webhook-adapter/... -v
```

Expected: compile error.

- [ ] **Step 3: Implement `generic-webhook-adapter/main.go`**

Create `src-go/cmd/generic-webhook-adapter/main.go`:

```go
//go:build wasip1

package main

import (
	"encoding/json"
	"fmt"

	pluginsdk "agentforge/plugin-sdk-go"
)

var runtime = pluginsdk.NewRuntime(&genericWebhookPlugin{})

type genericWebhookPlugin struct{}

func (p *genericWebhookPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion: "agentforge/v1",
		Kind:       "IntegrationPlugin",
		ID:         "generic-webhook-adapter",
		Name:       "Generic Webhook Adapter",
		Version:    "0.1.0",
		Runtime:    "wasm",
		ABIVersion: pluginsdk.ABIVersion,
		Capabilities: []pluginsdk.Capability{
			{Name: "health"},
			{Name: "handle_webhook"},
		},
	}, nil
}

func (p *genericWebhookPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (p *genericWebhookPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.OKResult(map[string]any{"ok": true}), nil
}

func (p *genericWebhookPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch inv.Operation {
	case "health":
		return pluginsdk.OKResult(map[string]any{"ok": true}), nil

	case "handle_webhook":
		body, _ := inv.Payload["body"].(string)

		// Use caller-provided event_type (from plugin config injected at invocation time),
		// falling back to the default.
		eventType, _ := inv.Payload["event_type"].(string)
		if eventType == "" {
			eventType = "integration.webhook.received"
		}

		var parsed map[string]any
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			// Non-JSON body: wrap as raw string
			parsed = map[string]any{"raw": body}
		}

		return pluginsdk.OKResult(map[string]any{
			"event_type": eventType,
			"payload":    parsed,
		}), nil

	default:
		return nil, fmt.Errorf("unsupported operation: %s", inv.Operation)
	}
}

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return pluginsdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return pluginsdk.ExportRun(runtime) }
```

- [ ] **Step 4: Create manifest**

Create `plugins/integrations/generic-webhook-adapter/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: generic-webhook-adapter
  name: Generic Webhook Adapter
  version: 0.1.0
  description: Accepts any HTTP webhook and publishes it to the event bus with a configurable event type
  tags: [builtin, integration, webhook]
spec:
  runtime: wasm
  module: ./dist/generic-webhook.wasm
  abiVersion: v1
  capabilities:
    - health
    - handle_webhook
  configSchema:
    type: object
    properties:
      event_type:
        type: string
        description: "EventBus event type to publish (default: integration.webhook.received)"
      secret:
        type: string
        description: Optional shared secret for basic HMAC validation
permissions:
  network:
    required: false
source:
  type: builtin
  path: ./plugins/integrations/generic-webhook-adapter/manifest.yaml
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-go && go test ./cmd/generic-webhook-adapter/... -v
```

Expected: PASS (2 tests).

- [ ] **Step 6: Commit**

```bash
git add src-go/cmd/generic-webhook-adapter/ plugins/integrations/generic-webhook-adapter/
git commit -m "feat(plugin): add generic-webhook-adapter — any HTTP webhook → EventBus"
```

---

## Task 4: `email-adapter` WASM plugin

Sends outbound email via SMTP. Triggered by calling the plugin's `send_email` operation (from `notification-fanout` or directly from a workflow tool chain).

**Files:**
- Create: `src-go/cmd/email-adapter/main.go`
- Create: `src-go/cmd/email-adapter/main_test.go`
- Create: `plugins/integrations/email-adapter/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/cmd/email-adapter/main_test.go`:

```go
package main

import (
	"testing"
	pluginsdk "agentforge/plugin-sdk-go"
)

func TestEmailAdapter_Describe(t *testing.T) {
	p := &emailPlugin{}
	ctx := &pluginsdk.Context{}
	d, err := p.Describe(ctx)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if d.ID != "email-adapter" {
		t.Errorf("ID = %q", d.ID)
	}
	ops := make(map[string]bool)
	for _, c := range d.Capabilities {
		ops[c.Name] = true
	}
	if !ops["send_email"] {
		t.Error("missing send_email capability")
	}
}

func TestEmailAdapter_SendEmail_ValidationError(t *testing.T) {
	p := &emailPlugin{}
	ctx := &pluginsdk.Context{}

	// Missing required fields should return error.
	_, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "send_email",
		Payload: map[string]any{
			"to": "", // empty to
		},
	})
	if err == nil {
		t.Error("expected validation error for empty 'to'")
	}
}

func TestEmailAdapter_SendEmail_ValidInput(t *testing.T) {
	p := &emailPlugin{}
	ctx := &pluginsdk.Context{}

	// In WASM test (no real SMTP), plugin returns a structured result
	// without actually sending (stub mode when smtp_host is empty).
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "send_email",
		Payload: map[string]any{
			"to":      "team@example.com",
			"subject": "Build failed",
			"body":    "The main branch build failed.",
			// smtp_host intentionally omitted → stub mode
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	sent, _ := result.Data["sent"].(bool)
	stub, _ := result.Data["stub"].(bool)
	if !stub && !sent {
		t.Error("expected either sent=true or stub=true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./cmd/email-adapter/... -v
```

Expected: compile error.

- [ ] **Step 3: Implement `email-adapter/main.go`**

Create `src-go/cmd/email-adapter/main.go`:

```go
//go:build wasip1

package main

import (
	"fmt"
	"net/smtp"
	"strings"

	pluginsdk "agentforge/plugin-sdk-go"
)

var runtime = pluginsdk.NewRuntime(&emailPlugin{})

type emailPlugin struct{}

func (p *emailPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion: "agentforge/v1",
		Kind:       "IntegrationPlugin",
		ID:         "email-adapter",
		Name:       "Email Adapter",
		Version:    "0.1.0",
		Runtime:    "wasm",
		ABIVersion: pluginsdk.ABIVersion,
		Capabilities: []pluginsdk.Capability{
			{Name: "health"},
			{Name: "send_email"},
		},
	}, nil
}

func (p *emailPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (p *emailPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.OKResult(map[string]any{"ok": true}), nil
}

func (p *emailPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch inv.Operation {
	case "health":
		return pluginsdk.OKResult(map[string]any{"ok": true}), nil

	case "send_email":
		return p.sendEmail(inv.Payload)

	default:
		return nil, fmt.Errorf("unsupported operation: %s", inv.Operation)
	}
}

func (p *emailPlugin) sendEmail(payload map[string]any) (*pluginsdk.Result, error) {
	to, _ := payload["to"].(string)
	subject, _ := payload["subject"].(string)
	body, _ := payload["body"].(string)
	smtpHost, _ := payload["smtp_host"].(string)
	smtpPort, _ := payload["smtp_port"].(string)
	from, _ := payload["from"].(string)
	username, _ := payload["username"].(string)
	password, _ := payload["password"].(string)

	if strings.TrimSpace(to) == "" {
		return nil, fmt.Errorf("send_email: 'to' address is required")
	}
	if strings.TrimSpace(subject) == "" {
		return nil, fmt.Errorf("send_email: 'subject' is required")
	}

	// Stub mode when no SMTP host configured (e.g., in tests).
	if smtpHost == "" {
		return pluginsdk.OKResult(map[string]any{
			"sent": false,
			"stub": true,
			"to":   to,
		}), nil
	}

	if smtpPort == "" {
		smtpPort = "587"
	}
	if from == "" {
		from = username
	}

	addr := smtpHost + ":" + smtpPort
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s", from, to, subject, body)

	var auth smtp.Auth
	if username != "" && password != "" {
		auth = smtp.PlainAuth("", username, password, smtpHost)
	}

	if err := smtp.SendMail(addr, auth, from, []string{to}, []byte(msg)); err != nil {
		return nil, fmt.Errorf("send_email: SMTP error: %w", err)
	}

	return pluginsdk.OKResult(map[string]any{
		"sent": true,
		"to":   to,
	}), nil
}

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return pluginsdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return pluginsdk.ExportRun(runtime) }
```

- [ ] **Step 4: Create manifest**

Create `plugins/integrations/email-adapter/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: email-adapter
  name: Email Adapter
  version: 0.1.0
  description: Sends email notifications via SMTP
  tags: [builtin, integration, notification, email]
spec:
  runtime: wasm
  module: ./dist/email.wasm
  abiVersion: v1
  capabilities:
    - health
    - send_email
  configSchema:
    type: object
    required: [smtp_host, username, password]
    properties:
      smtp_host:
        type: string
        description: SMTP server hostname
      smtp_port:
        type: string
        description: "SMTP port (default: 587)"
      from:
        type: string
        description: Sender email address (defaults to username)
      username:
        type: string
        description: SMTP authentication username
      password:
        type: string
        description: SMTP authentication password
permissions:
  network:
    required: true
    domains: []   # any SMTP host; tighten per-deployment
source:
  type: builtin
  path: ./plugins/integrations/email-adapter/manifest.yaml
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-go && go test ./cmd/email-adapter/... -v
```

Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add src-go/cmd/email-adapter/ plugins/integrations/email-adapter/
git commit -m "feat(plugin): add email-adapter — SMTP outbound notification plugin"
```

---

## Task 5: `notification-fanout` firstparty-inproc plugin

Routes one notification to multiple delivery channels. For IM channels it calls `POST /api/v1/im/notify` on the Go Orchestrator (which routes through the IM Bridge). For email it invokes `email-adapter` directly. This keeps the IM Bridge as the sole transport owner.

**Files:**
- Create: `src-go/internal/server/notification_fanout_plugin.go`
- Create: `src-go/internal/server/notification_fanout_plugin_test.go`
- Create: `plugins/integrations/notification-fanout/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/server/notification_fanout_plugin_test.go`:

```go
package server_test

import (
	"context"
	"testing"
)

type stubIMNotifier struct {
	calls []map[string]any
}

func (s *stubIMNotifier) Notify(ctx context.Context, platform, channelID, text string) error {
	s.calls = append(s.calls, map[string]any{
		"platform":   platform,
		"channel_id": channelID,
		"text":       text,
	})
	return nil
}

type stubEmailSender struct {
	calls []map[string]any
}

func (s *stubEmailSender) SendEmail(ctx context.Context, to, subject, body string) error {
	s.calls = append(s.calls, map[string]any{"to": to, "subject": subject})
	return nil
}

func TestNotificationFanout_RoutesToIMAndEmail(t *testing.T) {
	imNotifier := &stubIMNotifier{}
	emailSender := &stubEmailSender{}

	plugin := NewNotificationFanoutPlugin(imNotifier, emailSender)

	rules := []NotificationRule{
		{Channel: "im", Platform: "slack", ChannelID: "C123"},
		{Channel: "email", To: "team@example.com"},
	}

	err := plugin.Fanout(context.Background(), FanoutRequest{
		Subject: "Build failed",
		Body:    "main branch build failed",
		Rules:   rules,
	})
	if err != nil {
		t.Fatalf("Fanout: %v", err)
	}
	if len(imNotifier.calls) != 1 {
		t.Errorf("expected 1 IM notify call, got %d", len(imNotifier.calls))
	}
	if imNotifier.calls[0]["platform"] != "slack" {
		t.Errorf("IM platform = %v", imNotifier.calls[0]["platform"])
	}
	if len(emailSender.calls) != 1 {
		t.Errorf("expected 1 email send call, got %d", len(emailSender.calls))
	}
	if emailSender.calls[0]["to"] != "team@example.com" {
		t.Errorf("email to = %v", emailSender.calls[0]["to"])
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/server/... -run TestNotificationFanout -v
```

Expected: compile error.

- [ ] **Step 3: Implement `notification_fanout_plugin.go`**

Create `src-go/internal/server/notification_fanout_plugin.go`:

```go
package server

import (
	"context"
	"fmt"
)

// IMNotifier routes IM notifications through the existing IMService.
// Implemented by wrapping IMService.Notify() — do NOT call IM Bridge directly.
type IMNotifier interface {
	Notify(ctx context.Context, platform, channelID, text string) error
}

// EmailSender invokes the email-adapter plugin's send_email operation.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// NotificationRule describes one delivery target.
type NotificationRule struct {
	Channel   string // "im" or "email"
	Platform  string // IM platform (feishu, slack, etc.) — used when Channel="im"
	ChannelID string // IM channel/chat ID — used when Channel="im"
	To        string // Email address — used when Channel="email"
}

// FanoutRequest is the input to Fanout.
type FanoutRequest struct {
	Subject string
	Body    string
	Rules   []NotificationRule
}

// NotificationFanoutPlugin is a firstparty-inproc plugin registered at startup.
// It routes one notification to multiple channels (IM Bridge for IM, email-adapter for email).
type NotificationFanoutPlugin struct {
	im    IMNotifier
	email EmailSender
}

func NewNotificationFanoutPlugin(im IMNotifier, email EmailSender) *NotificationFanoutPlugin {
	return &NotificationFanoutPlugin{im: im, email: email}
}

// Fanout delivers the notification to all configured channels.
// Errors from individual channels are collected and returned as a combined error.
func (p *NotificationFanoutPlugin) Fanout(ctx context.Context, req FanoutRequest) error {
	var errs []string
	for _, rule := range req.Rules {
		switch rule.Channel {
		case "im":
			if err := p.im.Notify(ctx, rule.Platform, rule.ChannelID, req.Body); err != nil {
				errs = append(errs, fmt.Sprintf("im[%s/%s]: %v", rule.Platform, rule.ChannelID, err))
			}
		case "email":
			if err := p.email.SendEmail(ctx, rule.To, req.Subject, req.Body); err != nil {
				errs = append(errs, fmt.Sprintf("email[%s]: %v", rule.To, err))
			}
		default:
			errs = append(errs, fmt.Sprintf("unknown channel type: %q", rule.Channel))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("fanout errors: %v", errs)
	}
	return nil
}
```

- [ ] **Step 4: Register via firstparty-inproc hook**

In `src-go/internal/server/` (the startup file that registers other firstparty-inproc plugins like qianchuan), add:

```go
func installNotificationFanoutPlugin(imService IMNotifier, emailPlugin EmailSender) {
	fanout := NewNotificationFanoutPlugin(imService, emailPlugin)
	// Register under plugin ID "notification-fanout" with the plugin runtime.
	pluginRuntime.RegisterInproc("notification-fanout", fanout)
}
```

- [ ] **Step 5: Create manifest**

Create `plugins/integrations/notification-fanout/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: notification-fanout
  name: Notification Fanout
  version: 0.1.0
  description: Routes one notification to multiple delivery channels (IM via IM Bridge, email via email-adapter)
  tags: [builtin, integration, notification]
spec:
  runtime: firstparty-inproc
  capabilities:
    - health
    - fanout
  config:
    hook: src-go/internal/server/notification_fanout_plugin.go:installNotificationFanoutPlugin
source:
  type: builtin
  path: ./plugins/integrations/notification-fanout/manifest.yaml
```

- [ ] **Step 6: Run all integration plugin tests**

```bash
cd src-go && go test ./internal/server/... -run TestNotificationFanout -v
cd src-go && go test ./cmd/github-actions-adapter/... ./cmd/generic-webhook-adapter/... ./cmd/email-adapter/... -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add src-go/internal/server/notification_fanout_plugin.go src-go/internal/server/notification_fanout_plugin_test.go plugins/integrations/notification-fanout/
git commit -m "feat(plugin): add notification-fanout firstparty-inproc plugin"
```

---

## Final Check

- [ ] **Run all affected tests**

```bash
cd src-go && go test ./internal/handler/... ./internal/server/... ./cmd/github-actions-adapter/... ./cmd/generic-webhook-adapter/... ./cmd/email-adapter/... -v
```

Expected: all PASS.

- [ ] **Confirm no IM Bridge code was modified**

```bash
git diff HEAD -- src-im-bridge/
```

Expected: empty diff — IM Bridge untouched.
