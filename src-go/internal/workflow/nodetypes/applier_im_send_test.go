package nodetypes

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type stubDispatcher struct {
	sentCards   []json.RawMessage
	replyTarget map[string]any
}

func (s *stubDispatcher) Send(_ context.Context, target map[string]any, card json.RawMessage) (string, error) {
	s.replyTarget = target
	s.sentCards = append(s.sentCards, card)
	return "msg-1", nil
}

type stubCorrelationsCreator struct {
	created   []*CorrelationCreateInput
	nextToken uuid.UUID
}

func (s *stubCorrelationsCreator) Create(_ context.Context, in *CorrelationCreateInput) (uuid.UUID, error) {
	s.created = append(s.created, in)
	if s.nextToken == uuid.Nil {
		return uuid.New(), nil
	}
	return s.nextToken, nil
}

type stubMetaWriter struct{ patches []map[string]any }

func (s *stubMetaWriter) MergeSystemMetadata(_ context.Context, _ uuid.UUID, p map[string]any) error {
	s.patches = append(s.patches, p)
	return nil
}

func TestApplyExecuteIMSend_ReplyToTriggerWithCallbackTokens(t *testing.T) {
	execID := uuid.New()
	sysMeta := map[string]any{
		"reply_target": map[string]any{
			"provider": "feishu", "chat_id": "C1", "thread_id": "T1",
		},
	}
	sysMetaBytes, _ := json.Marshal(sysMeta)
	exec := &model.WorkflowExecution{ID: execID, SystemMetadata: sysMetaBytes}

	dispatch := &stubDispatcher{}
	creator := &stubCorrelationsCreator{}
	meta := &stubMetaWriter{}
	ds := &stubDataStoreMerger{}

	a := &EffectApplier{
		IMSendDispatcher:    dispatch,
		CorrelationsCreator: creator,
		ExecutionMetaWriter: meta,
		DataStoreMerger:     ds,
	}

	cardRaw := json.RawMessage(`{
	  "title":"Done",
	  "actions":[
	    {"id":"approve","label":"Approve","type":"callback","payload":{"k":"v"}},
	    {"id":"link","label":"View","type":"url","url":"https://x"}
	  ]
	}`)
	payload := ExecuteIMSendPayload{RawCard: cardRaw, Target: "reply_to_trigger"}
	raw, _ := json.Marshal(payload)

	if err := a.applyExecuteIMSend(context.Background(), exec, &model.WorkflowNode{ID: "im-1"}, raw); err != nil {
		t.Fatalf("applyExecuteIMSend: %v", err)
	}
	if len(creator.created) != 1 {
		t.Fatalf("expected 1 callback correlation, got %d", len(creator.created))
	}
	if creator.created[0].ActionID != "approve" {
		t.Errorf("action id = %s", creator.created[0].ActionID)
	}
	// Token expires within 7 days +/- 1 minute.
	if creator.created[0].ExpiresAt.Sub(time.Now()) > 7*24*time.Hour+time.Minute {
		t.Error("default lifetime too long")
	}
	if len(dispatch.sentCards) != 1 {
		t.Fatal("card not dispatched")
	}
	if got := dispatch.replyTarget["chat_id"]; got != "C1" {
		t.Errorf("reply chat_id = %v", got)
	}
	if len(meta.patches) != 1 || meta.patches[0]["im_dispatched"] != true {
		t.Errorf("im_dispatched not stamped: %+v", meta.patches)
	}

	// The dispatched card MUST have the callback action's value rewritten
	// to carry correlation_token.
	var dispatched map[string]any
	_ = json.Unmarshal(dispatch.sentCards[0], &dispatched)
	actions := dispatched["actions"].([]any)
	cb := actions[0].(map[string]any)
	if cb["correlation_token"] == nil {
		t.Error("correlation_token not injected")
	}
}

func TestApplyExecuteIMSend_NoReplyTarget(t *testing.T) {
	a := &EffectApplier{
		IMSendDispatcher:    &stubDispatcher{},
		CorrelationsCreator: &stubCorrelationsCreator{},
		ExecutionMetaWriter: &stubMetaWriter{},
		DataStoreMerger:     &stubDataStoreMerger{},
	}
	payload := ExecuteIMSendPayload{RawCard: json.RawMessage(`{"title":"x"}`), Target: "reply_to_trigger"}
	raw, _ := json.Marshal(payload)
	err := a.applyExecuteIMSend(context.Background(), &model.WorkflowExecution{}, &model.WorkflowNode{ID: "im-1"}, raw)
	if err == nil || err.Error() != "im_send:no_reply_target" {
		t.Fatalf("err = %v, want im_send:no_reply_target", err)
	}
}
