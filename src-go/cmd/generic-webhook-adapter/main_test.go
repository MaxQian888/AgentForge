package main

import (
	"testing"

	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
)

func TestGenericWebhookAdapter_PublishesConfiguredEventType(t *testing.T) {
	p := &genericWebhookPlugin{}
	ctx := &pluginsdk.Context{}

	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body":       `{"foo":"bar"}`,
			"headers":    map[string]string{"Content-Type": "application/json"},
			"event_type": "custom.deploy.finished",
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	et, _ := result.Data["event_type"].(string)
	if et != "custom.deploy.finished" {
		t.Errorf("event_type = %q, want custom.deploy.finished", et)
	}
	payload, _ := result.Data["payload"].(map[string]any)
	if payload["foo"] != "bar" {
		t.Errorf("payload.foo = %v, want bar", payload["foo"])
	}
}

func TestGenericWebhookAdapter_DefaultEventType(t *testing.T) {
	p := &genericWebhookPlugin{}
	ctx := &pluginsdk.Context{}

	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body":    `{"x":1}`,
			"headers": map[string]string{},
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

func TestGenericWebhookAdapter_NonJSONBody(t *testing.T) {
	p := &genericWebhookPlugin{}
	ctx := &pluginsdk.Context{}

	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "handle_webhook",
		Payload: map[string]any{
			"body":    `not json`,
			"headers": map[string]string{},
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	payload, _ := result.Data["payload"].(map[string]any)
	if payload["raw"] != "not json" {
		t.Errorf("non-JSON body should land under payload.raw, got %v", payload)
	}
}

func TestGenericWebhookAdapter_DescribeAndHealth(t *testing.T) {
	p := &genericWebhookPlugin{}
	ctx := &pluginsdk.Context{}
	d, err := p.Describe(ctx)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if d.ID != "generic-webhook-adapter" {
		t.Errorf("ID = %q", d.ID)
	}
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
