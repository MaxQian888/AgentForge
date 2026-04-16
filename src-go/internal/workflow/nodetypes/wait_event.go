package nodetypes

import (
	"context"
	"encoding/json"
)

// WaitEventHandler implements the "wait_event" node type. It emits a single
// EffectWaitEvent effect carrying the event_type and optional match_key from
// config. The applier (Task 4) is responsible for broadcasting the waiting
// intent and signaling parked=true; the DAG service (Task 16) flips the
// node's status to waiting on seeing the park signal.
//
// The handler is pure: no repository writes, no websocket calls.
type WaitEventHandler struct{}

// Execute builds the wait_event effect.
//
// Config fields (both optional — the service-layer behavior does not
// validate, it just broadcasts whatever is there):
//   - event_type (string): identifier the external trigger must emit.
//   - match_key  (string): optional discriminator for routing one of many
//     events to this node.
func (WaitEventHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	var config map[string]any
	if req != nil {
		config = req.Config
	}

	eventType, _ := config["event_type"].(string)
	matchKey, _ := config["match_key"].(string)

	payload, _ := json.Marshal(WaitEventPayload{
		EventType: eventType,
		MatchKey:  matchKey,
	})
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectWaitEvent, Payload: payload}},
	}, nil
}

// ConfigSchema describes the wait_event node configuration.
func (WaitEventHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "event_type": {"type": "string"},
    "match_key":  {"type": "string"}
  }
}`)
}

// Capabilities declares the wait_event effect this handler emits.
func (WaitEventHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectWaitEvent}
}
