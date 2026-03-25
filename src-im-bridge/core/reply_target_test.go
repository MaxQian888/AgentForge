package core

import (
	"encoding/json"
	"testing"
)

func TestReplyTarget_JSONRoundTripPreservesNativeUpdateHints(t *testing.T) {
	target := &ReplyTarget{
		Platform:           "discord",
		ChannelID:          "channel-1",
		ThreadID:           "thread-1",
		MessageID:          "message-1",
		InteractionToken:   "interaction-token",
		OriginalResponseID: "@original",
		CallbackToken:      "callback-token",
		TopicID:            "topic-1",
		UseReply:           true,
		PreferEdit:         true,
		PreferredRenderer:  "blocks",
		ProgressMode:       "follow_up",
		Metadata: map[string]string{
			"response_url_expires_at": "2026-03-25T00:00:00Z",
		},
	}

	raw, err := json.Marshal(target)
	if err != nil {
		t.Fatalf("marshal reply target: %v", err)
	}

	var decoded ReplyTarget
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal reply target: %v", err)
	}

	if decoded.OriginalResponseID != "@original" {
		t.Fatalf("OriginalResponseID = %q", decoded.OriginalResponseID)
	}
	if decoded.CallbackToken != "callback-token" {
		t.Fatalf("CallbackToken = %q", decoded.CallbackToken)
	}
	if decoded.TopicID != "topic-1" {
		t.Fatalf("TopicID = %q", decoded.TopicID)
	}
	if decoded.PreferredRenderer != "blocks" {
		t.Fatalf("PreferredRenderer = %q", decoded.PreferredRenderer)
	}
	if decoded.ProgressMode != "follow_up" {
		t.Fatalf("ProgressMode = %q", decoded.ProgressMode)
	}
	if decoded.Metadata["response_url_expires_at"] != "2026-03-25T00:00:00Z" {
		t.Fatalf("Metadata = %+v", decoded.Metadata)
	}
}
