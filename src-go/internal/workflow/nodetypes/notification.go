package nodetypes

import (
	"context"
	"encoding/json"
)

// NotificationHandler implements the "notification" node type. It emits a
// single EffectBroadcastEvent effect carrying the workflow notification
// payload; the applier is responsible for actually pushing the event to the
// WebSocket hub.
type NotificationHandler struct{}

// Execute builds the broadcast-event effect. The default message is used when
// config.message is missing or empty, preserving the service-layer behavior.
func (NotificationHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	message := "Workflow notification"
	if req != nil {
		if m, ok := req.Config["message"].(string); ok && m != "" {
			message = m
		}
	}

	inner := map[string]any{
		"message": message,
	}
	if req != nil {
		if req.Execution != nil {
			inner["executionId"] = req.Execution.ID.String()
		}
		if req.Node != nil {
			inner["nodeId"] = req.Node.ID
		}
	}

	payload, _ := json.Marshal(BroadcastEventPayload{
		EventType: "workflow.notification",
		Payload:   inner,
	})
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectBroadcastEvent, Payload: payload}},
	}, nil
}

// ConfigSchema describes the notification node configuration.
func (NotificationHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "message": {"type": "string"}
  }
}`)
}

// Capabilities declares the broadcast_event effect this handler emits.
func (NotificationHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectBroadcastEvent}
}
