package nodetypes

import (
	"context"
	"encoding/json"
	"testing"
)

func TestWaitEventHandler_EmitsTimeoutSeconds(t *testing.T) {
	result, err := (WaitEventHandler{}).Execute(context.Background(), &NodeExecRequest{
		Config: map[string]any{
			"event_type":      "im.card.clicked",
			"match_key":       "approve",
			"timeout_seconds": float64(90),
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result == nil || len(result.Effects) != 1 {
		t.Fatalf("effects = %+v, want 1 wait_event effect", result)
	}

	var payload WaitEventPayload
	if err := json.Unmarshal(result.Effects[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if payload.TimeoutSeconds != 90 {
		t.Fatalf("TimeoutSeconds = %d, want 90", payload.TimeoutSeconds)
	}
	if payload.EventType != "im.card.clicked" || payload.MatchKey != "approve" {
		t.Fatalf("payload = %+v", payload)
	}
}
