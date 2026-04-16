package nodetypes

import (
	"context"
	"encoding/json"
)

// HumanReviewHandler implements the "human_review" node type. It emits a
// single EffectRequestReview effect carrying the prompt and a serialized copy
// of the full node config as the review context. The applier (Task 4)
// persists the WorkflowPendingReview row, broadcasts the request, and signals
// parked=true. The DAG service (Task 16) is responsible for flipping node and
// execution status to waiting/paused on seeing the park signal.
//
// The handler is pure: no repository or websocket access happens here.
type HumanReviewHandler struct{}

// Execute builds the request_review effect.
//
// Config fields:
//   - prompt (string, optional): human-facing prompt; defaults to
//     "Review required" to match the service-layer behavior.
//
// Any additional config keys are preserved verbatim in the effect's Context
// field (serialized as JSON), so reviewers can see the full review intent.
func (HumanReviewHandler) Execute(_ context.Context, req *NodeExecRequest) (*NodeExecResult, error) {
	var config map[string]any
	if req != nil {
		config = req.Config
	}

	prompt, _ := config["prompt"].(string)
	if prompt == "" {
		prompt = "Review required"
	}

	reviewCtx, _ := json.Marshal(config)
	payload, _ := json.Marshal(RequestReviewPayload{
		Prompt:  prompt,
		Context: reviewCtx,
	})
	return &NodeExecResult{
		Effects: []Effect{{Kind: EffectRequestReview, Payload: payload}},
	}, nil
}

// ConfigSchema describes the human_review node configuration. Additional
// keys are allowed; they are forwarded as the review context.
func (HumanReviewHandler) ConfigSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "properties": {
    "prompt": {"type": "string"}
  }
}`)
}

// Capabilities declares the request_review effect this handler emits.
func (HumanReviewHandler) Capabilities() []EffectKind {
	return []EffectKind{EffectRequestReview}
}
