package core

import (
	"encoding/json"
	"testing"
)

func TestProviderNeutralCard_JSONShape(t *testing.T) {
	c := ProviderNeutralCard{
		Title:   "Echo",
		Status:  CardStatusSuccess,
		Summary: "hello world",
		Fields:  []CardField{{Label: "Run", Value: "abc"}},
		Actions: []CardAction{
			{ID: "view", Label: "查看", Type: CardActionTypeURL, URL: "https://x/runs/abc"},
			{ID: "approve", Label: "Approve", Type: CardActionTypeCallback,
				Style: CardStyleDanger, CorrelationToken: "tok-1",
				Payload: map[string]any{"who": "qa"}},
		},
		Footer: "2026-04-20T10:00:00Z",
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back["title"] != "Echo" || back["status"] != "success" {
		t.Fatalf("title/status not preserved: %v", back)
	}
	actions := back["actions"].([]any)
	if actions[0].(map[string]any)["type"] != "url" {
		t.Fatal("url action")
	}
	if actions[1].(map[string]any)["correlation_token"] != "tok-1" {
		t.Fatal("token")
	}
}

func TestCardAction_ValidateExclusiveFields(t *testing.T) {
	bad := CardAction{ID: "x", Label: "x", Type: CardActionTypeURL, URL: "", CorrelationToken: "t"}
	if err := bad.Validate(); err == nil {
		t.Fatal("expected url action without URL to fail")
	}

	good := CardAction{ID: "x", Label: "x", Type: CardActionTypeCallback, CorrelationToken: "t"}
	if err := good.Validate(); err != nil {
		t.Fatalf("good callback failed: %v", err)
	}
}
