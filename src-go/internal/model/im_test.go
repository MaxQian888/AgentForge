package model

import (
	"encoding/json"
	"testing"
)

func TestIMReplyTarget_JSONRoundTripPreservesNativeUpdateHints(t *testing.T) {
	target := &IMReplyTarget{
		Platform:           "feishu",
		ChatID:             "chat-1",
		MessageID:          "om_123",
		CallbackToken:      "callback-token",
		OriginalResponseID: "orig-1",
		TopicID:            "topic-1",
		PreferredRenderer:  "cards",
		ProgressMode:       "deferred_card_update",
		Metadata: map[string]string{
			"card_token": "card-1",
		},
	}

	raw, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("marshal reply target: %v", err)
	}

	var decoded IMReplyTarget
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal reply target: %v", err)
	}

	if decoded.CallbackToken != "callback-token" {
		t.Fatalf("CallbackToken = %q", decoded.CallbackToken)
	}
	if decoded.OriginalResponseID != "orig-1" {
		t.Fatalf("OriginalResponseID = %q", decoded.OriginalResponseID)
	}
	if decoded.TopicID != "topic-1" {
		t.Fatalf("TopicID = %q", decoded.TopicID)
	}
	if decoded.PreferredRenderer != "cards" {
		t.Fatalf("PreferredRenderer = %q", decoded.PreferredRenderer)
	}
	if decoded.ProgressMode != "deferred_card_update" {
		t.Fatalf("ProgressMode = %q", decoded.ProgressMode)
	}
	if decoded.Metadata["card_token"] != "card-1" {
		t.Fatalf("Metadata = %+v", decoded.Metadata)
	}
}

func TestIMActionRequest_JSONRoundTripPreservesReplyTargetAndMetadata(t *testing.T) {
	req := &IMActionRequest{
		Platform:  "slack",
		Action:    "approve",
		EntityID:  "review-1",
		ChannelID: "C123",
		UserID:    "U123",
		BridgeID:  "bridge-slack-1",
		ReplyTarget: &IMReplyTarget{
			Platform:          "slack",
			ChannelID:         "C123",
			ThreadID:          "thread-1",
			PreferredRenderer: "blocks",
		},
		Metadata: map[string]string{
			"source": "block_actions",
		},
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal action request: %v", err)
	}

	var decoded IMActionRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal action request: %v", err)
	}

	if decoded.BridgeID != "bridge-slack-1" {
		t.Fatalf("BridgeID = %q", decoded.BridgeID)
	}
	if decoded.ReplyTarget == nil || decoded.ReplyTarget.ThreadID != "thread-1" {
		t.Fatalf("ReplyTarget = %+v", decoded.ReplyTarget)
	}
	if decoded.Metadata["source"] != "block_actions" {
		t.Fatalf("Metadata = %+v", decoded.Metadata)
	}
}
