package main

import (
	"encoding/json"
	"fmt"

	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
)

type genericWebhookPlugin struct{}

func (genericWebhookPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion:  "agentforge/v1",
		Kind:        "IntegrationPlugin",
		ID:          "generic-webhook-adapter",
		Name:        "Generic Webhook Adapter",
		Version:     "0.1.0",
		Runtime:     "wasm",
		ABIVersion:  pluginsdk.ABIVersion,
		Description: "Accepts any HTTP webhook and publishes it to the AgentForge event bus with a configurable event type.",
		Capabilities: []pluginsdk.Capability{
			{Name: "health", Description: "Report plugin health"},
			{Name: "handle_webhook", Description: "Wrap an arbitrary webhook body into an event envelope"},
		},
	}, nil
}

func (genericWebhookPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (genericWebhookPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.Success(map[string]any{"ok": true}), nil
}

func (p genericWebhookPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
	switch inv.Operation {
	case "health":
		return pluginsdk.Success(map[string]any{"ok": true}), nil
	case "handle_webhook":
		return p.handleWebhook(inv.Payload)
	default:
		return nil, pluginsdk.NewRuntimeError("unsupported_operation",
			fmt.Sprintf("unsupported operation %s", inv.Operation)).
			WithDetail("operation", inv.Operation)
	}
}

func (genericWebhookPlugin) handleWebhook(payload map[string]any) (*pluginsdk.Result, error) {
	body, _ := payload["body"].(string)

	eventType, _ := payload["event_type"].(string)
	if eventType == "" {
		eventType = "integration.webhook.received"
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		parsed = map[string]any{"raw": body}
	}

	return pluginsdk.Success(map[string]any{
		"event_type": eventType,
		"payload":    parsed,
	}), nil
}

var runtime = pluginsdk.NewRuntime(genericWebhookPlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return pluginsdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return pluginsdk.ExportRun(runtime) }

func main() {
	pluginsdk.Autorun(runtime)
}
