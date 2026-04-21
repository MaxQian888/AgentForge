package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
)

type githubActionsPlugin struct{}

func (githubActionsPlugin) Describe(ctx *pluginsdk.Context) (*pluginsdk.Descriptor, error) {
	return &pluginsdk.Descriptor{
		APIVersion:  "agentforge/v1",
		Kind:        "IntegrationPlugin",
		ID:          "github-actions-adapter",
		Name:        "GitHub Actions Adapter",
		Version:     "0.1.0",
		Runtime:     "wasm",
		ABIVersion:  pluginsdk.ABIVersion,
		Description: "Receives GitHub webhook events, validates HMAC-SHA256, and publishes typed events to the AgentForge event bus.",
		Capabilities: []pluginsdk.Capability{
			{Name: "health", Description: "Report plugin health"},
			{Name: "handle_webhook", Description: "Validate signature and translate GitHub webhook into an AgentForge event"},
		},
	}, nil
}

func (githubActionsPlugin) Init(ctx *pluginsdk.Context) error { return nil }

func (githubActionsPlugin) Health(ctx *pluginsdk.Context) (*pluginsdk.Result, error) {
	return pluginsdk.Success(map[string]any{"ok": true}), nil
}

// githubEventMap maps a (X-Github-Event header, body action) pair to the
// AgentForge EventBus type. Empty string under the inner map matches when
// no action field is present (e.g., push events).
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

func (p githubActionsPlugin) Invoke(ctx *pluginsdk.Context, inv pluginsdk.Invocation) (*pluginsdk.Result, error) {
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

func (githubActionsPlugin) handleWebhook(payload map[string]any) (*pluginsdk.Result, error) {
	body, _ := payload["body"].(string)
	secret, _ := payload["webhook_secret"].(string)
	headers := readHeaders(payload["headers"])

	if secret != "" {
		if !validateSignature(secret, body, headers["X-Hub-Signature-256"]) {
			return nil, pluginsdk.NewRuntimeError("invalid_signature", "webhook signature validation failed")
		}
	}

	githubEvent := headers["X-Github-Event"]

	var bodyMap map[string]any
	_ = json.Unmarshal([]byte(body), &bodyMap)
	action, _ := bodyMap["action"].(string)

	eventType := mapGitHubEvent(githubEvent, action)
	if eventType == "" {
		// Unknown event — return success without event_type so the handler
		// skips publication. Lets us advertise broad webhook coverage even
		// when we don't have a translation yet.
		return pluginsdk.Success(map[string]any{}), nil
	}

	return pluginsdk.Success(map[string]any{
		"event_type": eventType,
		"payload":    bodyMap,
	}), nil
}

// readHeaders accepts headers in any of: map[string]string, map[string]any,
// or nil. The handler hands map[string]string in production but the SDK
// JSON-roundtrip in WASM mode flattens to map[string]any.
func readHeaders(raw any) map[string]string {
	out := map[string]string{}
	switch h := raw.(type) {
	case map[string]string:
		for k, v := range h {
			out[k] = v
		}
	case map[string]any:
		for k, v := range h {
			if s, ok := v.(string); ok {
				out[k] = s
			}
		}
	}
	return out
}

func validateSignature(secret, body, sigHeader string) bool {
	if sigHeader == "" {
		return false
	}
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

var runtime = pluginsdk.NewRuntime(githubActionsPlugin{})

//go:wasmexport agentforge_abi_version
func agentforgeABIVersion() uint64 { return pluginsdk.ExportABIVersion(runtime) }

//go:wasmexport agentforge_run
func agentforgeRun() uint32 { return pluginsdk.ExportRun(runtime) }

func main() {
	pluginsdk.Autorun(runtime)
}
