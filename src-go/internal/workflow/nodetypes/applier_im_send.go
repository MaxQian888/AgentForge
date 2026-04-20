package nodetypes

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/agentforge/server/internal/model"
)

// applyExecuteIMSend performs:
//  1. Resolve target (reply_to_trigger | explicit).
//  2. Walk card.actions[]; for each {type:"callback"}, mint a
//     correlation row and rewrite the action to carry only
//     {correlation_token, action_id} per spec.
//  3. POST the rendered card to IM Bridge via IMSendDispatcher.
//  4. Stamp system_metadata.im_dispatched=true so the outbound
//     dispatcher skips the default reply.
//  5. Write {sent:true, message_id} into dataStore[nodeID].
func (a *EffectApplier) applyExecuteIMSend(ctx context.Context, exec *model.WorkflowExecution, node *model.WorkflowNode, raw json.RawMessage) error {
	if a.IMSendDispatcher == nil {
		return fmt.Errorf("im_send: IMSendDispatcher not configured")
	}
	if a.CorrelationsCreator == nil {
		return fmt.Errorf("im_send: CorrelationsCreator not configured")
	}
	if a.ExecutionMetaWriter == nil {
		return fmt.Errorf("im_send: ExecutionMetaWriter not configured")
	}
	if a.DataStoreMerger == nil {
		return fmt.Errorf("im_send: DataStoreMerger not configured")
	}

	var p ExecuteIMSendPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	// --- 1. Resolve target ---
	var replyTarget map[string]any
	switch p.Target {
	case "reply_to_trigger":
		sysMeta := map[string]any{}
		if len(exec.SystemMetadata) > 0 {
			_ = json.Unmarshal(exec.SystemMetadata, &sysMeta)
		}
		rt, ok := sysMeta["reply_target"].(map[string]any)
		if !ok || len(rt) == 0 {
			return fmt.Errorf("im_send:no_reply_target")
		}
		replyTarget = rt
	case "explicit":
		if p.ExplicitChat == nil {
			return fmt.Errorf("im_send: explicit target requires explicit_target")
		}
		replyTarget = map[string]any{
			"provider":  p.ExplicitChat.Provider,
			"chat_id":   p.ExplicitChat.ChatID,
			"thread_id": p.ExplicitChat.ThreadID,
		}
	default:
		return fmt.Errorf("im_send: invalid target %q", p.Target)
	}

	// --- 2. Walk + mint correlations ---
	var card map[string]any
	if err := json.Unmarshal(p.RawCard, &card); err != nil {
		return fmt.Errorf("im_send: invalid card: %w", err)
	}
	lifetime := 7 * 24 * time.Hour
	if p.TokenLifetime != "" {
		if d, err := time.ParseDuration(p.TokenLifetime); err == nil {
			lifetime = d
		}
	}
	if actions, ok := card["actions"].([]any); ok {
		for i, rawAct := range actions {
			act, ok := rawAct.(map[string]any)
			if !ok {
				continue
			}
			if act["type"] != "callback" {
				continue
			}
			actionID, _ := act["id"].(string)
			if actionID == "" {
				return fmt.Errorf("im_send: callback action missing id")
			}
			payloadFromAuthor := map[string]any{}
			if pl, ok := act["payload"].(map[string]any); ok {
				payloadFromAuthor = pl
			}
			token, err := a.CorrelationsCreator.Create(ctx, &CorrelationCreateInput{
				ExecutionID: exec.ID,
				NodeID:      node.ID,
				ActionID:    actionID,
				Payload:     payloadFromAuthor,
				ExpiresAt:   time.Now().Add(lifetime),
			})
			if err != nil {
				return fmt.Errorf("im_send: mint correlation %q: %w", actionID, err)
			}
			// Rewrite the action: drop the author payload from the wire
			// (it lives in the correlations row), inject correlation_token.
			act["correlation_token"] = token.String()
			delete(act, "payload")
			actions[i] = act
		}
		card["actions"] = actions
	}
	renderedCard, err := json.Marshal(card)
	if err != nil {
		return fmt.Errorf("im_send: re-marshal card: %w", err)
	}

	// --- 3. Dispatch ---
	messageID, err := a.IMSendDispatcher.Send(ctx, replyTarget, renderedCard)
	if err != nil {
		return fmt.Errorf("im_send: dispatch: %w", err)
	}

	// --- 4. Stamp im_dispatched ---
	if err := a.ExecutionMetaWriter.MergeSystemMetadata(ctx, exec.ID, map[string]any{
		"im_dispatched": true,
	}); err != nil {
		if a.AuditSink != nil {
			_ = a.AuditSink.Record(ctx, "im_send_meta_stamp_failed", map[string]any{
				"executionId": exec.ID.String(), "nodeId": node.ID, "error": err.Error(),
			})
		}
	}

	// --- 5. Write result ---
	result := map[string]any{"sent": true}
	if messageID != "" {
		result["message_id"] = messageID
	}
	if err := a.DataStoreMerger.MergeNodeResult(ctx, exec.ID, node.ID, result); err != nil {
		return fmt.Errorf("im_send: merge result: %w", err)
	}
	return nil
}
