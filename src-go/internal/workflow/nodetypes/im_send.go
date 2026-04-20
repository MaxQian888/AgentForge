package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
)

// IMSendHandler implements the "im_send" node type. The handler emits a
// single EffectExecuteIMSend carrying the templated card config. The
// applier is what mints correlation tokens, renders the card, dispatches
// via IM Bridge, and stamps system_metadata.im_dispatched.
type IMSendHandler struct{}

func (IMSendHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	if req == nil {
		return nil, fmt.Errorf("nil request")
	}
	cfg := req.Config

	target, _ := cfg["target"].(string)
	if target == "" {
		target = "reply_to_trigger"
	}
	if target != "reply_to_trigger" && target != "explicit" {
		return nil, fmt.Errorf("im_send: invalid target %q", target)
	}

	cardRaw, ok := cfg["card"]
	if !ok {
		return nil, fmt.Errorf("im_send: card is required")
	}
	cardBytes, err := json.Marshal(cardRaw)
	if err != nil {
		return nil, fmt.Errorf("im_send: marshal card: %w", err)
	}

	payload := ExecuteIMSendPayload{
		RawCard: cardBytes,
		Target:  target,
	}
	if target == "explicit" {
		if exp, ok := cfg["explicit_target"].(map[string]any); ok {
			payload.ExplicitChat = &IMSendExplicit{
				Provider: stringOf(exp["provider"]),
				ChatID:   stringOf(exp["chat_id"]),
				ThreadID: stringOf(exp["thread_id"]),
			}
		}
	}
	if v, ok := cfg["token_lifetime"].(string); ok {
		payload.TokenLifetime = v
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectExecuteIMSend, Payload: raw}},
	}, nil
}

func (IMSendHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type":"object",
  "required":["card"],
  "properties":{
    "target":{"type":"string","enum":["reply_to_trigger","explicit"]},
    "explicit_target":{"type":"object","properties":{
      "provider":{"type":"string"},
      "chat_id":{"type":"string"},
      "thread_id":{"type":"string"}
    }},
    "card":{"type":"object","required":["title"]},
    "token_lifetime":{"type":"string","description":"Go duration; default 168h"}
  }
}`)
}

func (IMSendHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectExecuteIMSend}
}

func stringOf(v any) string { s, _ := v.(string); return s }
