package model

import (
	"encoding/json"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestIMMessageRequest_PlatformValidatorAcceptsAllRegisteredProviders(t *testing.T) {
	v := validator.New()
	// Must stay in sync with the oneof validator on IMMessageRequest.Platform.
	for _, platform := range []string{
		"feishu",
		"dingtalk",
		"slack",
		"telegram",
		"discord",
		"wecom",
		"wechat",
		"qq",
		"qqbot",
		"email",
	} {
		req := &IMMessageRequest{Platform: platform, ChannelID: "c", Text: "t"}
		if err := v.Struct(req); err != nil {
			t.Fatalf("platform %q should validate, got %v", platform, err)
		}
	}

	if err := v.Struct(&IMMessageRequest{Platform: "unknown-platform", ChannelID: "c", Text: "t"}); err == nil {
		t.Fatalf("unknown platform should fail validation")
	}
}

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

func TestIMSendRequest_JSONRoundTripPreservesTypedNativePayloads(t *testing.T) {
	req := &IMSendRequest{
		Platform:  "slack",
		ChannelID: "C123",
		Text:      "fallback text",
		Native: &IMNativeMessage{
			Platform: "slack",
			SlackBlockKit: &IMSlackBlockKitPayload{
				Blocks: json.RawMessage(`[{"type":"section","text":{"type":"mrkdwn","text":"*Build* ready"}}]`),
			},
		},
	}

	raw, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal send request: %v", err)
	}

	var decoded IMSendRequest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal send request: %v", err)
	}

	if decoded.Native == nil || decoded.Native.SlackBlockKit == nil {
		t.Fatalf("Native = %+v", decoded.Native)
	}
	if string(decoded.Native.SlackBlockKit.Blocks) == "" {
		t.Fatalf("SlackBlockKit = %+v", decoded.Native.SlackBlockKit)
	}
}

func TestIMNativeMessage_JSONRoundTripPreservesAdditionalPlatformPayloads(t *testing.T) {
	payload := &IMNativeMessage{
		Platform: "discord",
		DiscordEmbed: &IMDiscordEmbedPayload{
			Title:       "Build Ready",
			Description: "Agent finished the run.",
			Fields: []IMDiscordEmbedField{{
				Name:  "Status",
				Value: "success",
			}},
			Components: []IMDiscordActionRow{{
				Buttons: []IMDiscordButton{{
					Label: "Open",
					URL:   "https://example.test/builds/1",
				}},
			}},
		},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal native payload: %v", err)
	}

	var decoded IMNativeMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal native payload: %v", err)
	}

	if decoded.DiscordEmbed == nil || decoded.DiscordEmbed.Title != "Build Ready" {
		t.Fatalf("DiscordEmbed = %+v", decoded.DiscordEmbed)
	}
	if len(decoded.DiscordEmbed.Components) != 1 || len(decoded.DiscordEmbed.Components[0].Buttons) != 1 {
		t.Fatalf("Components = %+v", decoded.DiscordEmbed.Components)
	}
}
