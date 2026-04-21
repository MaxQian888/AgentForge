package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
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
			"headers": map[string]string{
				"X-Github-Event":      "pull_request",
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
	payload, _ := result.Data["payload"].(map[string]any)
	if payload["number"] != float64(42) {
		t.Errorf("payload.number = %v, want 42", payload["number"])
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
			"headers": map[string]string{
				"X-Github-Event":      "push",
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
			"headers": map[string]string{
				"X-Github-Event":      "push",
				"X-Hub-Signature-256": "sha256=badhash",
			},
			"webhook_secret": "real-secret",
		},
	})
	if err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestGitHubAdapter_NoSecretSkipsValidation(t *testing.T) {
	p := &githubActionsPlugin{}
	ctx := &pluginsdk.Context{}
	body := `{"ref":"refs/heads/main"}`
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body": body,
			"headers": map[string]string{
				"X-Github-Event": "push",
			},
			// no webhook_secret → validation skipped
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.Data["event_type"] != "vcs.push" {
		t.Errorf("event_type = %v, want vcs.push", result.Data["event_type"])
	}
}

func TestGitHubAdapter_UnknownEvent(t *testing.T) {
	p := &githubActionsPlugin{}
	ctx := &pluginsdk.Context{}
	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body": `{}`,
			"headers": map[string]string{
				"X-Github-Event": "ping",
			},
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	// Unknown events return success but without event_type — handler skips publish.
	if et, ok := result.Data["event_type"]; ok && et != "" {
		t.Errorf("expected no event_type for unknown event, got %v", et)
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
	if d.Kind != "IntegrationPlugin" {
		t.Errorf("Kind = %q", d.Kind)
	}
	if d.Runtime != "wasm" {
		t.Errorf("Runtime = %q", d.Runtime)
	}
}

func TestGitHubAdapter_HealthAndUnsupported(t *testing.T) {
	p := &githubActionsPlugin{}
	ctx := &pluginsdk.Context{}
	if err := p.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	h, err := p.Health(ctx)
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if h.Data["ok"] != true {
		t.Errorf("health = %v", h.Data)
	}
	if _, err := p.Invoke(ctx, pluginsdk.Invocation{Operation: "bogus"}); err == nil {
		t.Error("expected error for unsupported operation")
	}
}
