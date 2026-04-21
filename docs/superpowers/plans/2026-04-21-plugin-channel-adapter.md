# Plugin Channel Adapter Framework + IM Integration Plugins

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `ChannelAdapter` Go-side wrapper interface over the existing WASM operation model, refactor the Feishu adapter to use it, then build Dingtalk, Slack, and Discord integration plugins following the same pattern.

**Architecture:** `ChannelAdapter` is a Go interface wrapping WASM `Invoke()` calls — WASM plugins declare `handle_inbound` / `send_outbound` / `health` capabilities and route them in their `Invoke` switch. A `WASMChannelAdapter` struct adapts any WASM plugin with these capabilities into the interface. `ChannelPluginSpec` is added to `PluginSpec` to carry channel metadata.

**Tech Stack:** Go 1.23+, wazero WASM runtime, existing `pluginsdk` package at `src-go/plugin-sdk-go/`, YAML manifest parsing, `src-go/internal/model/plugin.go`, `src-go/internal/plugin/runtime.go`.

---

## File Map

| Action | Path | Responsibility |
|--------|------|---------------|
| Modify | `src-go/internal/model/plugin.go` | Add `ChannelPluginSpec`, `Channel` field to `PluginSpec` |
| Create | `src-go/internal/plugin/channel_adapter.go` | `ChannelAdapter` interface, `WASMChannelAdapter`, result types |
| Create | `src-go/internal/plugin/channel_adapter_test.go` | Unit tests for adapter routing |
| Modify | `src-go/cmd/sample-wasm-plugin/main.go` | Add `handle_inbound` + `send_outbound` capabilities to Feishu example |
| Modify | `src-go/cmd/sample-wasm-plugin/main_test.go` | Tests for new capabilities |
| Modify | `plugins/integrations/feishu-adapter/manifest.yaml` | Add `channel` section |
| Create | `src-go/cmd/dingtalk-adapter/main.go` | Dingtalk WASM plugin |
| Create | `src-go/cmd/dingtalk-adapter/main_test.go` | Dingtalk tests |
| Create | `plugins/integrations/dingtalk-adapter/manifest.yaml` | Dingtalk manifest |
| Create | `src-go/cmd/slack-adapter/main.go` | Slack WASM plugin |
| Create | `src-go/cmd/slack-adapter/main_test.go` | Slack tests |
| Create | `plugins/integrations/slack-adapter/manifest.yaml` | Slack manifest |
| Create | `src-go/cmd/discord-adapter/main.go` | Discord WASM plugin |
| Create | `src-go/cmd/discord-adapter/main_test.go` | Discord tests |
| Create | `plugins/integrations/discord-adapter/manifest.yaml` | Discord manifest |

---

## Task 1: Add `ChannelPluginSpec` to the model

**Files:**
- Modify: `src-go/internal/model/plugin.go`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/model/plugin_channel_test.go`:

```go
package model_test

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestChannelPluginSpec_Unmarshal(t *testing.T) {
	raw := `
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: slack-adapter
  name: Slack Adapter
  version: 0.1.0
spec:
  runtime: wasm
  module: ./dist/slack.wasm
  capabilities: [health, handle_inbound, send_outbound]
  channel:
    platform: slack
    capabilities: [inbound, outbound, threading, rich_cards]
    inboundEvents: [message.received, mention.received]
    outboundFormats: [text, markdown, card]
`
	var m PluginManifest
	if err := yaml.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ch := m.Spec.Channel
	if ch == nil {
		t.Fatal("expected Channel to be non-nil")
	}
	if ch.Platform != "slack" {
		t.Errorf("Platform = %q, want slack", ch.Platform)
	}
	if len(ch.Capabilities) != 4 {
		t.Errorf("Capabilities len = %d, want 4", len(ch.Capabilities))
	}
	if len(ch.InboundEvents) != 2 {
		t.Errorf("InboundEvents len = %d, want 2", len(ch.InboundEvents))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/model/... -run TestChannelPluginSpec -v
```

Expected: compile error — `ChannelPluginSpec` not defined.

- [ ] **Step 3: Add `ChannelPluginSpec` to `plugin.go`**

Open `src-go/internal/model/plugin.go`. Find the `PluginSpec` struct. Add after the existing `Review *ReviewPluginSpec` field:

```go
// ChannelPluginSpec carries channel metadata for IntegrationPlugin adapters.
type ChannelPluginSpec struct {
	Platform      string   `yaml:"platform,omitempty" json:"platform,omitempty"`
	Capabilities  []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
	InboundEvents []string `yaml:"inboundEvents,omitempty" json:"inboundEvents,omitempty"`
	OutboundFmts  []string `yaml:"outboundFormats,omitempty" json:"outboundFormats,omitempty"`
}
```

Then add the field to `PluginSpec`:

```go
// inside PluginSpec struct, after Review field:
Channel *ChannelPluginSpec `yaml:"channel,omitempty" json:"channel,omitempty"`
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd src-go && go test ./internal/model/... -run TestChannelPluginSpec -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
cd src-go && git add internal/model/plugin.go internal/model/plugin_channel_test.go
git commit -m "feat(model): add ChannelPluginSpec to PluginSpec"
```

---

## Task 2: Create `ChannelAdapter` interface and `WASMChannelAdapter`

**Files:**
- Create: `src-go/internal/plugin/channel_adapter.go`
- Create: `src-go/internal/plugin/channel_adapter_test.go`

- [ ] **Step 1: Write the failing test**

Create `src-go/internal/plugin/channel_adapter_test.go`:

```go
package plugin_test

import (
	"context"
	"encoding/json"
	"testing"
)

// stubWASMInvoker simulates calling a WASM plugin's Invoke function.
type stubWASMInvoker struct {
	ops []string // records which operations were called
}

func (s *stubWASMInvoker) Invoke(ctx context.Context, op string, payload map[string]any) (map[string]any, error) {
	s.ops = append(s.ops, op)
	switch op {
	case "health":
		return map[string]any{"ok": true}, nil
	case "handle_inbound":
		return map[string]any{"handled": true, "platform": payload["platform"]}, nil
	case "send_outbound":
		return map[string]any{"sent": true, "channel_id": payload["channel_id"]}, nil
	case "capabilities":
		return map[string]any{
			"inbound": true, "outbound": true, "threading": false,
			"reactions": false, "file_attach": false, "rich_cards": true,
		}, nil
	}
	return nil, fmt.Errorf("unknown op: %s", op)
}

func TestWASMChannelAdapter_HealthCheck(t *testing.T) {
	stub := &stubWASMInvoker{}
	adapter := NewWASMChannelAdapter(stub)

	if err := adapter.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if len(stub.ops) != 1 || stub.ops[0] != "health" {
		t.Errorf("expected op 'health', got %v", stub.ops)
	}
}

func TestWASMChannelAdapter_HandleInbound(t *testing.T) {
	stub := &stubWASMInvoker{}
	adapter := NewWASMChannelAdapter(stub)

	result, err := adapter.HandleInbound(context.Background(), map[string]any{
		"platform":   "slack",
		"channel_id": "C123",
		"text":       "hello",
	})
	if err != nil {
		t.Fatalf("HandleInbound: %v", err)
	}
	if !result.Handled {
		t.Error("expected Handled=true")
	}
}

func TestWASMChannelAdapter_SendOutbound(t *testing.T) {
	stub := &stubWASMInvoker{}
	adapter := NewWASMChannelAdapter(stub)

	result, err := adapter.SendOutbound(context.Background(), OutboundMessage{
		ChannelID: "C123",
		Text:      "hello back",
		Format:    "text",
	})
	if err != nil {
		t.Fatalf("SendOutbound: %v", err)
	}
	if !result.Sent {
		t.Error("expected Sent=true")
	}
}

func TestWASMChannelAdapter_Capabilities(t *testing.T) {
	stub := &stubWASMInvoker{}
	adapter := NewWASMChannelAdapter(stub)

	caps := adapter.Capabilities()
	if !caps.Inbound {
		t.Error("expected Inbound=true")
	}
	if !caps.RichCards {
		t.Error("expected RichCards=true")
	}
	if caps.Threading {
		t.Error("expected Threading=false")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./internal/plugin/... -run TestWASMChannelAdapter -v
```

Expected: compile error — types not defined.

- [ ] **Step 3: Implement `channel_adapter.go`**

Create `src-go/internal/plugin/channel_adapter.go`:

```go
package plugin

import (
	"context"
	"fmt"
)

// WASMInvoker abstracts the operation-based call into a WASM plugin.
// Implemented by the existing WASM runtime wrapper.
type WASMInvoker interface {
	Invoke(ctx context.Context, operation string, payload map[string]any) (map[string]any, error)
}

// ChannelCapabilities describes what an integration plugin adapter supports.
type ChannelCapabilities struct {
	Inbound    bool
	Outbound   bool
	Threading  bool
	Reactions  bool
	FileAttach bool
	RichCards  bool
}

// InboundResult is returned by HandleInbound.
type InboundResult struct {
	Handled  bool
	Platform string
	Raw      map[string]any
}

// OutboundMessage is the input to SendOutbound.
type OutboundMessage struct {
	ChannelID string
	ThreadID  string
	Text      string
	Format    string // "text" | "markdown" | "card"
	Raw       map[string]any
}

// OutboundResult is returned by SendOutbound.
type OutboundResult struct {
	Sent      bool
	MessageID string
	Raw       map[string]any
}

// ChannelAdapter is a Go-side abstraction over an integration plugin's
// WASM operation routing. WASM plugins expose these via Invoke() operations.
type ChannelAdapter interface {
	Capabilities() ChannelCapabilities
	HandleInbound(ctx context.Context, payload map[string]any) (*InboundResult, error)
	SendOutbound(ctx context.Context, msg OutboundMessage) (*OutboundResult, error)
	HealthCheck(ctx context.Context) error
}

// WASMChannelAdapter wraps a WASMInvoker and exposes it as a ChannelAdapter.
// WASM operation mapping:
//   HealthCheck   → "health"
//   HandleInbound → "handle_inbound"
//   SendOutbound  → "send_outbound"
//   Capabilities  → "capabilities"
type WASMChannelAdapter struct {
	invoker WASMInvoker
}

// NewWASMChannelAdapter returns a ChannelAdapter backed by the given WASMInvoker.
func NewWASMChannelAdapter(invoker WASMInvoker) ChannelAdapter {
	return &WASMChannelAdapter{invoker: invoker}
}

func (a *WASMChannelAdapter) HealthCheck(ctx context.Context) error {
	result, err := a.invoker.Invoke(ctx, "health", nil)
	if err != nil {
		return fmt.Errorf("channel health check: %w", err)
	}
	if ok, _ := result["ok"].(bool); !ok {
		return fmt.Errorf("channel health check returned not-ok")
	}
	return nil
}

func (a *WASMChannelAdapter) HandleInbound(ctx context.Context, payload map[string]any) (*InboundResult, error) {
	result, err := a.invoker.Invoke(ctx, "handle_inbound", payload)
	if err != nil {
		return nil, fmt.Errorf("handle_inbound: %w", err)
	}
	handled, _ := result["handled"].(bool)
	platform, _ := result["platform"].(string)
	return &InboundResult{Handled: handled, Platform: platform, Raw: result}, nil
}

func (a *WASMChannelAdapter) SendOutbound(ctx context.Context, msg OutboundMessage) (*OutboundResult, error) {
	payload := map[string]any{
		"channel_id": msg.ChannelID,
		"thread_id":  msg.ThreadID,
		"text":       msg.Text,
		"format":     msg.Format,
	}
	if msg.Raw != nil {
		for k, v := range msg.Raw {
			payload[k] = v
		}
	}
	result, err := a.invoker.Invoke(ctx, "send_outbound", payload)
	if err != nil {
		return nil, fmt.Errorf("send_outbound: %w", err)
	}
	sent, _ := result["sent"].(bool)
	msgID, _ := result["message_id"].(string)
	return &OutboundResult{Sent: sent, MessageID: msgID, Raw: result}, nil
}

func (a *WASMChannelAdapter) Capabilities() ChannelCapabilities {
	result, err := a.invoker.Invoke(context.Background(), "capabilities", nil)
	if err != nil {
		return ChannelCapabilities{}
	}
	boolField := func(key string) bool {
		v, _ := result[key].(bool)
		return v
	}
	return ChannelCapabilities{
		Inbound:    boolField("inbound"),
		Outbound:   boolField("outbound"),
		Threading:  boolField("threading"),
		Reactions:  boolField("reactions"),
		FileAttach: boolField("file_attach"),
		RichCards:  boolField("rich_cards"),
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd src-go && go test ./internal/plugin/... -run TestWASMChannelAdapter -v
```

Expected: all 4 tests PASS.

- [ ] **Step 5: Commit**

```bash
git add src-go/internal/plugin/channel_adapter.go src-go/internal/plugin/channel_adapter_test.go
git commit -m "feat(plugin): add ChannelAdapter interface and WASMChannelAdapter"
```

---

## Task 3: Refactor Feishu WASM plugin to add channel capabilities

**Files:**
- Modify: `src-go/cmd/sample-wasm-plugin/main.go`
- Modify: `src-go/cmd/sample-wasm-plugin/main_test.go`
- Modify: `plugins/integrations/feishu-adapter/manifest.yaml`

- [ ] **Step 1: Write the failing test**

In `src-go/cmd/sample-wasm-plugin/main_test.go`, add:

```go
func TestSamplePlugin_HandleInbound(t *testing.T) {
	p := &samplePlugin{}
	ctx := &pluginsdk.Context{}
	inv := pluginsdk.Invocation{
		Operation: "handle_inbound",
		Payload: map[string]any{
			"platform":   "feishu",
			"channel_id": "oc_abc123",
			"text":       "hello from feishu",
		},
	}
	result, err := p.Invoke(ctx, inv)
	if err != nil {
		t.Fatalf("Invoke handle_inbound: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSamplePlugin_SendOutbound(t *testing.T) {
	p := &samplePlugin{}
	ctx := &pluginsdk.Context{}
	inv := pluginsdk.Invocation{
		Operation: "send_outbound",
		Payload: map[string]any{
			"channel_id": "oc_abc123",
			"text":       "reply from agent",
			"format":     "text",
		},
	}
	result, err := p.Invoke(ctx, inv)
	if err != nil {
		t.Fatalf("Invoke send_outbound: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestSamplePlugin_Capabilities(t *testing.T) {
	p := &samplePlugin{}
	ctx := &pluginsdk.Context{}
	inv := pluginsdk.Invocation{Operation: "capabilities"}
	result, err := p.Invoke(ctx, inv)
	if err != nil {
		t.Fatalf("Invoke capabilities: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd src-go && go test ./cmd/sample-wasm-plugin/... -run TestSamplePlugin_Handle -v
```

Expected: FAIL — operations not handled.

- [ ] **Step 3: Extend `samplePlugin.Invoke` in `main.go`**

In `src-go/cmd/sample-wasm-plugin/main.go`, extend the `Invoke` switch:

```go
func (p samplePlugin) Invoke(ctx *pluginsdk.Context, invocation pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch invocation.Operation {
	case "health":
		return pluginsdk.OKResult(map[string]any{"ok": true}), nil

	case "send_message", "send_outbound":
		channelID, _ := invocation.Payload["channel_id"].(string)
		text, _ := invocation.Payload["text"].(string)
		if channelID == "" {
			channelID, _ = invocation.Payload["receive_id"].(string)
		}
		// In production: call Feishu API here.
		return pluginsdk.OKResult(map[string]any{
			"sent":       true,
			"message_id": "placeholder_" + channelID,
		}), nil

	case "handle_inbound":
		platform, _ := invocation.Payload["platform"].(string)
		channelID, _ := invocation.Payload["channel_id"].(string)
		// In production: validate webhook signature, parse event type.
		_ = channelID
		return pluginsdk.OKResult(map[string]any{
			"handled":  true,
			"platform": platform,
		}), nil

	case "capabilities":
		return pluginsdk.OKResult(map[string]any{
			"inbound":    true,
			"outbound":   true,
			"threading":  true,
			"reactions":  false,
			"file_attach": false,
			"rich_cards": true,
		}), nil

	default:
		return nil, fmt.Errorf("unsupported operation: %s", invocation.Operation)
	}
}
```

Also update `Describe` capabilities list:

```go
func (p samplePlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		// ... existing fields ...
		Capabilities: []pluginsdk.Capability{
			{Name: "health"},
			{Name: "send_message"},
			{Name: "send_outbound"},
			{Name: "handle_inbound"},
			{Name: "capabilities"},
		},
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd src-go && go test ./cmd/sample-wasm-plugin/... -v
```

Expected: all tests PASS.

- [ ] **Step 5: Update Feishu manifest**

Edit `plugins/integrations/feishu-adapter/manifest.yaml`, add the `channel` section under `spec`:

```yaml
spec:
  runtime: wasm
  module: ./dist/feishu.wasm
  abiVersion: v1
  capabilities:
    - health
    - send_message
    - send_outbound
    - handle_inbound
    - capabilities
  channel:
    platform: feishu
    capabilities: [inbound, outbound, threading, rich_cards]
    inboundEvents: [message.received, mention.received]
    outboundFormats: [text, markdown, card]
```

- [ ] **Step 6: Commit**

```bash
git add src-go/cmd/sample-wasm-plugin/ plugins/integrations/feishu-adapter/manifest.yaml
git commit -m "feat(plugin): extend Feishu adapter with channel capabilities"
```

---

## Task 4: Dingtalk WASM adapter

**Files:**
- Create: `src-go/cmd/dingtalk-adapter/main.go`
- Create: `src-go/cmd/dingtalk-adapter/main_test.go`
- Create: `plugins/integrations/dingtalk-adapter/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/cmd/dingtalk-adapter/main_test.go`:

```go
package main

import (
	"testing"
	pluginsdk "agentforge/plugin-sdk-go"
)

func TestDingtalkPlugin_Describe(t *testing.T) {
	p := &dingtalkPlugin{}
	ctx := &pluginsdk.Context{}
	d, err := p.Describe(ctx)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if d.ID != "dingtalk-adapter" {
		t.Errorf("ID = %q, want dingtalk-adapter", d.ID)
	}
	ops := make(map[string]bool)
	for _, c := range d.Capabilities {
		ops[c.Name] = true
	}
	for _, required := range []string{"health", "handle_inbound", "send_outbound", "capabilities"} {
		if !ops[required] {
			t.Errorf("missing capability: %s", required)
		}
	}
}

func TestDingtalkPlugin_HandleInbound(t *testing.T) {
	p := &dingtalkPlugin{}
	ctx := &pluginsdk.Context{}
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_inbound",
		Payload: map[string]any{
			"platform":   "dingtalk",
			"channel_id": "group_123",
			"text":       "hello dingtalk",
		},
	})
	if err != nil {
		t.Fatalf("handle_inbound: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestDingtalkPlugin_SendOutbound(t *testing.T) {
	p := &dingtalkPlugin{}
	ctx := &pluginsdk.Context{}
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "send_outbound",
		Payload: map[string]any{
			"channel_id": "group_123",
			"text":       "reply",
			"format":     "markdown",
		},
	})
	if err != nil {
		t.Fatalf("send_outbound: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./cmd/dingtalk-adapter/... -v
```

Expected: compile error — package not found.

- [ ] **Step 3: Implement `dingtalk-adapter/main.go`**

Create `src-go/cmd/dingtalk-adapter/main.go`:

```go
//go:build wasip1

package main

import (
	"fmt"
	pluginsdk "agentforge/plugin-sdk-go"
)

var runtime = pluginsdk.NewRuntime(&dingtalkPlugin{})

type dingtalkPlugin struct{}

func (p *dingtalkPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion: "agentforge/v1",
		Kind:       "IntegrationPlugin",
		ID:         "dingtalk-adapter",
		Name:       "Dingtalk Adapter",
		Version:    "0.1.0",
		Runtime:    "wasm",
		ABIVersion: pluginsdk.ABIVersion,
		Capabilities: []pluginsdk.Capability{
			{Name: "health"},
			{Name: "handle_inbound"},
			{Name: "send_outbound"},
			{Name: "capabilities"},
		},
	}, nil
}

func (p *dingtalkPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (p *dingtalkPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.OKResult(map[string]any{"ok": true}), nil
}

func (p *dingtalkPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch inv.Operation {
	case "health":
		return pluginsdk.OKResult(map[string]any{"ok": true}), nil

	case "handle_inbound":
		platform, _ := inv.Payload["platform"].(string)
		channelID, _ := inv.Payload["channel_id"].(string)
		// Production: validate DingTalk signature, parse msgtype/content.
		_ = channelID
		return pluginsdk.OKResult(map[string]any{
			"handled":  true,
			"platform": platform,
		}), nil

	case "send_outbound":
		channelID, _ := inv.Payload["channel_id"].(string)
		text, _ := inv.Payload["text"].(string)
		format, _ := inv.Payload["format"].(string)
		// Production: call DingTalk custom robot webhook or work notification API.
		_ = text
		_ = format
		return pluginsdk.OKResult(map[string]any{
			"sent":       true,
			"message_id": "dt_" + channelID,
		}), nil

	case "capabilities":
		return pluginsdk.OKResult(map[string]any{
			"inbound":     true,
			"outbound":    true,
			"threading":   false,
			"reactions":   false,
			"file_attach": false,
			"rich_cards":  true,
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

Create `plugins/integrations/dingtalk-adapter/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: dingtalk-adapter
  name: Dingtalk Adapter
  version: 0.1.0
  description: DingTalk group bot and work notification integration
  tags: [builtin, integration, im, dingtalk]
spec:
  runtime: wasm
  module: ./dist/dingtalk.wasm
  abiVersion: v1
  capabilities:
    - health
    - handle_inbound
    - send_outbound
    - capabilities
  channel:
    platform: dingtalk
    capabilities: [inbound, outbound, rich_cards]
    inboundEvents: [message.received, mention.received]
    outboundFormats: [text, markdown, card]
  config:
    mode: webhook
  configSchema:
    type: object
    required: [webhook_url]
    properties:
      webhook_url:
        type: string
        description: DingTalk custom robot webhook URL
      secret:
        type: string
        description: Signing secret for webhook verification
permissions:
  network:
    required: true
    domains:
      - oapi.dingtalk.com
source:
  type: builtin
  path: ./plugins/integrations/dingtalk-adapter/manifest.yaml
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-go && go test ./cmd/dingtalk-adapter/... -v
```

Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add src-go/cmd/dingtalk-adapter/ plugins/integrations/dingtalk-adapter/
git commit -m "feat(plugin): add dingtalk-adapter integration plugin"
```

---

## Task 5: Slack WASM adapter

**Files:**
- Create: `src-go/cmd/slack-adapter/main.go`
- Create: `src-go/cmd/slack-adapter/main_test.go`
- Create: `plugins/integrations/slack-adapter/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/cmd/slack-adapter/main_test.go`:

```go
package main

import (
	"testing"
	pluginsdk "agentforge/plugin-sdk-go"
)

func TestSlackPlugin_Describe(t *testing.T) {
	p := &slackPlugin{}
	ctx := &pluginsdk.Context{}
	d, err := p.Describe(ctx)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if d.ID != "slack-adapter" {
		t.Errorf("ID = %q, want slack-adapter", d.ID)
	}
	ops := make(map[string]bool)
	for _, c := range d.Capabilities {
		ops[c.Name] = true
	}
	for _, required := range []string{"health", "handle_inbound", "send_outbound", "capabilities"} {
		if !ops[required] {
			t.Errorf("missing capability: %s", required)
		}
	}
}

func TestSlackPlugin_HandleInbound(t *testing.T) {
	p := &slackPlugin{}
	ctx := &pluginsdk.Context{}
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_inbound",
		Payload: map[string]any{
			"platform":   "slack",
			"channel_id": "C012AB3CD",
			"text":       "<@UBOT> help",
		},
	})
	if err != nil {
		t.Fatalf("handle_inbound: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
}

func TestSlackPlugin_SendOutbound_Threading(t *testing.T) {
	p := &slackPlugin{}
	ctx := &pluginsdk.Context{}
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "send_outbound",
		Payload: map[string]any{
			"channel_id": "C012AB3CD",
			"thread_id":  "1234567890.123456",
			"text":       "replied in thread",
			"format":     "markdown",
		},
	})
	if err != nil {
		t.Fatalf("send_outbound: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./cmd/slack-adapter/... -v
```

Expected: compile error.

- [ ] **Step 3: Implement `slack-adapter/main.go`**

Create `src-go/cmd/slack-adapter/main.go`:

```go
//go:build wasip1

package main

import (
	"fmt"
	pluginsdk "agentforge/plugin-sdk-go"
)

var runtime = pluginsdk.NewRuntime(&slackPlugin{})

type slackPlugin struct{}

func (p *slackPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion: "agentforge/v1",
		Kind:       "IntegrationPlugin",
		ID:         "slack-adapter",
		Name:       "Slack Adapter",
		Version:    "0.1.0",
		Runtime:    "wasm",
		ABIVersion: pluginsdk.ABIVersion,
		Capabilities: []pluginsdk.Capability{
			{Name: "health"},
			{Name: "handle_inbound"},
			{Name: "send_outbound"},
			{Name: "capabilities"},
		},
	}, nil
}

func (p *slackPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (p *slackPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.OKResult(map[string]any{"ok": true}), nil
}

func (p *slackPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch inv.Operation {
	case "health":
		return pluginsdk.OKResult(map[string]any{"ok": true}), nil

	case "handle_inbound":
		platform, _ := inv.Payload["platform"].(string)
		channelID, _ := inv.Payload["channel_id"].(string)
		// Production: verify Slack signing secret, parse event type.
		_ = channelID
		return pluginsdk.OKResult(map[string]any{
			"handled":  true,
			"platform": platform,
		}), nil

	case "send_outbound":
		channelID, _ := inv.Payload["channel_id"].(string)
		threadID, _ := inv.Payload["thread_id"].(string)
		text, _ := inv.Payload["text"].(string)
		_ = text
		_ = threadID
		// Production: call chat.postMessage or chat.postEphemeral.
		return pluginsdk.OKResult(map[string]any{
			"sent":       true,
			"message_id": "slack_" + channelID,
		}), nil

	case "capabilities":
		return pluginsdk.OKResult(map[string]any{
			"inbound":     true,
			"outbound":    true,
			"threading":   true,
			"reactions":   true,
			"file_attach": true,
			"rich_cards":  true,
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

Create `plugins/integrations/slack-adapter/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: slack-adapter
  name: Slack Adapter
  version: 0.1.0
  description: Slack bot integration with Block Kit and slash command support
  tags: [builtin, integration, im, slack]
spec:
  runtime: wasm
  module: ./dist/slack.wasm
  abiVersion: v1
  capabilities:
    - health
    - handle_inbound
    - send_outbound
    - capabilities
  channel:
    platform: slack
    capabilities: [inbound, outbound, threading, reactions, file_attach, rich_cards]
    inboundEvents: [message.received, mention.received, slash_command]
    outboundFormats: [text, markdown, card]
  configSchema:
    type: object
    required: [bot_token, signing_secret]
    properties:
      bot_token:
        type: string
        description: Slack Bot OAuth token (xoxb-...)
      signing_secret:
        type: string
        description: Slack app signing secret for webhook verification
permissions:
  network:
    required: true
    domains:
      - slack.com
source:
  type: builtin
  path: ./plugins/integrations/slack-adapter/manifest.yaml
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-go && go test ./cmd/slack-adapter/... -v
```

Expected: PASS (3 tests).

- [ ] **Step 6: Commit**

```bash
git add src-go/cmd/slack-adapter/ plugins/integrations/slack-adapter/
git commit -m "feat(plugin): add slack-adapter integration plugin"
```

---

## Task 6: Discord WASM adapter

**Files:**
- Create: `src-go/cmd/discord-adapter/main.go`
- Create: `src-go/cmd/discord-adapter/main_test.go`
- Create: `plugins/integrations/discord-adapter/manifest.yaml`

- [ ] **Step 1: Write the failing test**

Create `src-go/cmd/discord-adapter/main_test.go`:

```go
package main

import (
	"testing"
	pluginsdk "agentforge/plugin-sdk-go"
)

func TestDiscordPlugin_Describe(t *testing.T) {
	p := &discordPlugin{}
	ctx := &pluginsdk.Context{}
	d, err := p.Describe(ctx)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if d.ID != "discord-adapter" {
		t.Errorf("ID = %q, want discord-adapter", d.ID)
	}
}

func TestDiscordPlugin_HandleInbound(t *testing.T) {
	p := &discordPlugin{}
	ctx := &pluginsdk.Context{}
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_inbound",
		Payload: map[string]any{
			"platform":   "discord",
			"channel_id": "123456789",
			"text":       "!help",
		},
	})
	if err != nil {
		t.Fatalf("handle_inbound: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
}

func TestDiscordPlugin_SendOutbound_Embed(t *testing.T) {
	p := &discordPlugin{}
	ctx := &pluginsdk.Context{}
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "send_outbound",
		Payload: map[string]any{
			"channel_id": "123456789",
			"text":       "build succeeded",
			"format":     "card",
		},
	})
	if err != nil {
		t.Fatalf("send_outbound: %v", err)
	}
	if result == nil {
		t.Fatal("nil result")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd src-go && go test ./cmd/discord-adapter/... -v
```

Expected: compile error.

- [ ] **Step 3: Implement `discord-adapter/main.go`**

Create `src-go/cmd/discord-adapter/main.go`:

```go
//go:build wasip1

package main

import (
	"fmt"
	pluginsdk "agentforge/plugin-sdk-go"
)

var runtime = pluginsdk.NewRuntime(&discordPlugin{})

type discordPlugin struct{}

func (p *discordPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion: "agentforge/v1",
		Kind:       "IntegrationPlugin",
		ID:         "discord-adapter",
		Name:       "Discord Adapter",
		Version:    "0.1.0",
		Runtime:    "wasm",
		ABIVersion: pluginsdk.ABIVersion,
		Capabilities: []pluginsdk.Capability{
			{Name: "health"},
			{Name: "handle_inbound"},
			{Name: "send_outbound"},
			{Name: "capabilities"},
		},
	}, nil
}

func (p *discordPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (p *discordPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.OKResult(map[string]any{"ok": true}), nil
}

func (p *discordPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch inv.Operation {
	case "health":
		return pluginsdk.OKResult(map[string]any{"ok": true}), nil

	case "handle_inbound":
		platform, _ := inv.Payload["platform"].(string)
		channelID, _ := inv.Payload["channel_id"].(string)
		// Production: verify Ed25519 signature from Discord, parse interaction type.
		_ = channelID
		return pluginsdk.OKResult(map[string]any{
			"handled":  true,
			"platform": platform,
		}), nil

	case "send_outbound":
		channelID, _ := inv.Payload["channel_id"].(string)
		text, _ := inv.Payload["text"].(string)
		format, _ := inv.Payload["format"].(string)
		_ = text
		_ = format
		// Production: call POST /channels/{channel.id}/messages with embeds for card format.
		return pluginsdk.OKResult(map[string]any{
			"sent":       true,
			"message_id": "discord_" + channelID,
		}), nil

	case "capabilities":
		return pluginsdk.OKResult(map[string]any{
			"inbound":     true,
			"outbound":    true,
			"threading":   false,
			"reactions":   true,
			"file_attach": true,
			"rich_cards":  true,
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

Create `plugins/integrations/discord-adapter/manifest.yaml`:

```yaml
apiVersion: agentforge/v1
kind: IntegrationPlugin
metadata:
  id: discord-adapter
  name: Discord Adapter
  version: 0.1.0
  description: Discord bot with slash commands, embeds, and channel routing
  tags: [builtin, integration, im, discord]
spec:
  runtime: wasm
  module: ./dist/discord.wasm
  abiVersion: v1
  capabilities:
    - health
    - handle_inbound
    - send_outbound
    - capabilities
  channel:
    platform: discord
    capabilities: [inbound, outbound, reactions, file_attach, rich_cards]
    inboundEvents: [message.received, slash_command, interaction]
    outboundFormats: [text, markdown, card]
  configSchema:
    type: object
    required: [bot_token, public_key]
    properties:
      bot_token:
        type: string
        description: Discord bot token
      public_key:
        type: string
        description: Discord application public key for Ed25519 signature verification
permissions:
  network:
    required: true
    domains:
      - discord.com
      - discordapp.com
source:
  type: builtin
  path: ./plugins/integrations/discord-adapter/manifest.yaml
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd src-go && go test ./cmd/discord-adapter/... -v
```

Expected: PASS (3 tests).

- [ ] **Step 6: Run all plugin tests to confirm no regressions**

```bash
cd src-go && go test ./internal/plugin/... ./internal/model/... ./cmd/sample-wasm-plugin/... ./cmd/dingtalk-adapter/... ./cmd/slack-adapter/... ./cmd/discord-adapter/... -v
```

Expected: all PASS.

- [ ] **Step 7: Commit**

```bash
git add src-go/cmd/discord-adapter/ plugins/integrations/discord-adapter/
git commit -m "feat(plugin): add discord-adapter integration plugin"
```
