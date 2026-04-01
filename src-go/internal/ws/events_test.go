package ws

import (
	"encoding/json"
	"testing"
)

func TestEventJSON(t *testing.T) {
	event := &Event{
		Type:      EventAgentStarted,
		ProjectID: "project-123",
		Payload: map[string]any{
			"status": "running",
			"turn":   2,
		},
	}

	var decoded map[string]any
	if err := json.Unmarshal(event.JSON(), &decoded); err != nil {
		t.Fatalf("Event.JSON() produced invalid JSON: %v", err)
	}

	if decoded["type"] != EventAgentStarted {
		t.Fatalf("decoded type = %v, want %q", decoded["type"], EventAgentStarted)
	}
	if decoded["projectId"] != "project-123" {
		t.Fatalf("decoded projectId = %v", decoded["projectId"])
	}
	payload, ok := decoded["payload"].(map[string]any)
	if !ok {
		t.Fatalf("decoded payload type = %T, want object", decoded["payload"])
	}
	if payload["status"] != "running" || payload["turn"] != float64(2) {
		t.Fatalf("decoded payload = %#v", payload)
	}
}

func TestBridgeAgentEventDecodeData(t *testing.T) {
	event := &BridgeAgentEvent{
		Type: BridgeEventCostUpdate,
		Data: []byte(`{"input_tokens":120,"output_tokens":45,"cache_read_tokens":5,"cache_creation_tokens":12,"cost_usd":0.37,"budget_remaining_usd":4.63,"turn_number":3,"cost_accounting":{"total_cost_usd":0.37,"input_tokens":120,"output_tokens":45,"cache_read_tokens":5,"cache_creation_tokens":12,"mode":"estimated_api_pricing","coverage":"full","source":"openai_api_pricing","components":[]}}`),
	}

	var payload BridgeEventCostUpdateData
	if err := event.DecodeData(&payload); err != nil {
		t.Fatalf("DecodeData() error = %v", err)
	}
	if payload.InputTokens != 120 || payload.OutputTokens != 45 || payload.CacheReadTokens != 5 {
		t.Fatalf("unexpected token payload: %+v", payload)
	}
	if payload.CostUSD != 0.37 || payload.BudgetRemainingUSD != 4.63 || payload.TurnNumber != 3 {
		t.Fatalf("unexpected cost payload: %+v", payload)
	}
	if payload.CacheCreationTokens != 12 {
		t.Fatalf("cache creation tokens = %d, want 12", payload.CacheCreationTokens)
	}
	if payload.CostAccounting == nil || payload.CostAccounting.Mode != "estimated_api_pricing" {
		t.Fatalf("cost accounting = %+v", payload.CostAccounting)
	}
}

func TestBridgeAgentEventDecodeDataHandlesEmptyAndInvalidPayload(t *testing.T) {
	var payload BridgeEventStatusChangeData

	if err := (*BridgeAgentEvent)(nil).DecodeData(&payload); err != nil {
		t.Fatalf("nil event DecodeData() error = %v, want nil", err)
	}
	if err := (&BridgeAgentEvent{}).DecodeData(&payload); err != nil {
		t.Fatalf("empty event DecodeData() error = %v, want nil", err)
	}
	if err := (&BridgeAgentEvent{Data: []byte("{")}).DecodeData(&payload); err == nil {
		t.Fatal("expected invalid JSON to return an error")
	}
}
