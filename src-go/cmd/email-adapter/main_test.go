package main

import (
	"testing"

	pluginsdk "github.com/agentforge/server/plugin-sdk-go"
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

	if _, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "send_email",
		Payload: map[string]any{
			"to": "",
		},
	}); err == nil {
		t.Error("expected validation error for empty 'to'")
	}

	if _, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "send_email",
		Payload: map[string]any{
			"to":      "team@example.com",
			"subject": "",
		},
	}); err == nil {
		t.Error("expected validation error for empty 'subject'")
	}
}

func TestEmailAdapter_SendEmail_StubMode(t *testing.T) {
	p := &emailPlugin{}
	ctx := &pluginsdk.Context{}

	result, err := p.Invoke(ctx, pluginsdk.Invocation{
		Operation: "send_email",
		Payload: map[string]any{
			"to":      "team@example.com",
			"subject": "Build failed",
			"body":    "main branch build failed",
			// smtp_host omitted → stub mode
		},
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if result.Data["stub"] != true {
		t.Errorf("expected stub=true when smtp_host empty, got %v", result.Data)
	}
	if result.Data["sent"] != false {
		t.Errorf("expected sent=false in stub mode, got %v", result.Data["sent"])
	}
	if result.Data["to"] != "team@example.com" {
		t.Errorf("expected to=team@example.com, got %v", result.Data["to"])
	}
}

func TestEmailAdapter_HealthAndUnsupported(t *testing.T) {
	p := &emailPlugin{}
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
