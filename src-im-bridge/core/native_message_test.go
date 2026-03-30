package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type nativeDeliveryTestPlatform struct {
	deliveryTestPlatform
	nativeSent    []*NativeMessage
	nativeReplies []*NativeMessage
	nativeUpdates []*NativeMessage
}

func (p *nativeDeliveryTestPlatform) SendNative(ctx context.Context, chatID string, message *NativeMessage) error {
	p.nativeSent = append(p.nativeSent, message)
	return nil
}

func (p *nativeDeliveryTestPlatform) ReplyNative(ctx context.Context, replyCtx any, message *NativeMessage) error {
	p.nativeReplies = append(p.nativeReplies, message)
	return nil
}

func (p *nativeDeliveryTestPlatform) UpdateNative(ctx context.Context, replyCtx any, message *NativeMessage) error {
	p.nativeUpdates = append(p.nativeUpdates, message)
	return nil
}

func TestNativeMessage_ValidateFeishuJSONAndTemplatePayloads(t *testing.T) {
	jsonMessage := &NativeMessage{
		Platform: "feishu",
		FeishuCard: &FeishuCardPayload{
			Mode: FeishuCardModeJSON,
			JSON: json.RawMessage(`{"header":{"title":{"tag":"plain_text","content":"Hello"}}}`),
		},
	}
	if err := jsonMessage.Validate(); err != nil {
		t.Fatalf("json Validate error: %v", err)
	}

	templateMessage := &NativeMessage{
		FeishuCard: &FeishuCardPayload{
			Mode:                FeishuCardModeTemplate,
			TemplateID:          "ctp_123",
			TemplateVersionName: "1.0.0",
			TemplateVariable: map[string]any{
				"title": "Hello",
			},
		},
	}
	if err := templateMessage.Validate(); err != nil {
		t.Fatalf("template Validate error: %v", err)
	}

	invalid := &NativeMessage{
		Platform: "feishu",
		FeishuCard: &FeishuCardPayload{
			Mode: FeishuCardModeJSON,
			JSON: json.RawMessage(`["not","an","object"]`),
		},
	}
	if err := invalid.Validate(); err == nil {
		t.Fatal("expected invalid feishu json card payload to fail")
	}
}

func TestNativeMessage_ConstructorsBuildTypedFeishuMessages(t *testing.T) {
	jsonMessage, err := NewFeishuJSONCardMessage(map[string]any{
		"header": map[string]any{
			"title": map[string]any{
				"tag":     "plain_text",
				"content": "Hello",
			},
		},
	})
	if err != nil {
		t.Fatalf("NewFeishuJSONCardMessage error: %v", err)
	}
	if jsonMessage.FeishuCard == nil || jsonMessage.FeishuCard.Mode != FeishuCardModeJSON {
		t.Fatalf("jsonMessage = %+v", jsonMessage)
	}

	templateMessage, err := NewFeishuTemplateCardMessage("ctp_123", "1.0.0", map[string]any{"status": "done"})
	if err != nil {
		t.Fatalf("NewFeishuTemplateCardMessage error: %v", err)
	}
	if templateMessage.FeishuCard == nil || templateMessage.FeishuCard.TemplateID != "ctp_123" {
		t.Fatalf("templateMessage = %+v", templateMessage)
	}

	markdownMessage, err := NewFeishuMarkdownCardMessage("AgentForge Update", "Hello **world**")
	if err != nil {
		t.Fatalf("NewFeishuMarkdownCardMessage error: %v", err)
	}
	if markdownMessage.FeishuCard == nil || markdownMessage.FeishuCard.Mode != FeishuCardModeJSON {
		t.Fatalf("markdownMessage = %+v", markdownMessage)
	}
}

func TestNativeMessage_ConstructorsBuildTypedPlatformMessages(t *testing.T) {
	slackMessage, err := NewSlackBlockKitMessage([]map[string]any{
		{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": "*Build* passed",
			},
		},
		{
			"type": "actions",
			"elements": []map[string]any{
				{
					"type": "button",
					"text": map[string]any{
						"type": "plain_text",
						"text": "Open",
					},
					"url": "https://example.test/builds/1",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("NewSlackBlockKitMessage error: %v", err)
	}
	if slackMessage.Platform != "slack" || slackMessage.SlackBlockKit == nil {
		t.Fatalf("slackMessage = %+v", slackMessage)
	}
	if fallback := slackMessage.SlackBlockKit.FallbackText(); !strings.Contains(fallback, "Build passed") || !strings.Contains(fallback, "Open") {
		t.Fatalf("slack fallback = %q", fallback)
	}

	discordMessage, err := NewDiscordEmbedMessage(
		"Build Ready",
		"Agent finished the run.",
		[]DiscordEmbedField{{Name: "Status", Value: "success", Inline: true}},
		0x00FF00,
		[]DiscordActionRow{{
			Buttons: []DiscordButton{{
				Label: "Open",
				URL:   "https://example.test/builds/1",
				Style: "link",
			}},
		}},
	)
	if err != nil {
		t.Fatalf("NewDiscordEmbedMessage error: %v", err)
	}
	if discordMessage.Platform != "discord" || discordMessage.DiscordEmbed == nil {
		t.Fatalf("discordMessage = %+v", discordMessage)
	}
	if fallback := discordMessage.DiscordEmbed.FallbackText(); !strings.Contains(fallback, "Build Ready") || !strings.Contains(fallback, "Status: success") {
		t.Fatalf("discord fallback = %q", fallback)
	}

	telegramMessage, err := NewTelegramRichMessage(
		"*Build* passed",
		"MarkdownV2",
		[][]TelegramInlineButton{{
			{Text: "Open", URL: "https://example.test/builds/1"},
		}},
	)
	if err != nil {
		t.Fatalf("NewTelegramRichMessage error: %v", err)
	}
	if telegramMessage.Platform != "telegram" || telegramMessage.TelegramRich == nil {
		t.Fatalf("telegramMessage = %+v", telegramMessage)
	}
	if fallback := telegramMessage.TelegramRich.FallbackText(); !strings.Contains(fallback, "Build passed") || !strings.Contains(fallback, "Open") {
		t.Fatalf("telegram fallback = %q", fallback)
	}

	dingTalkMessage, err := NewDingTalkCardMessage(
		DingTalkCardTypeActionCard,
		"Build Ready",
		"### Build passed",
		[]DingTalkCardButton{{Title: "Open", ActionURL: "https://example.test/builds/1"}},
	)
	if err != nil {
		t.Fatalf("NewDingTalkCardMessage error: %v", err)
	}
	if dingTalkMessage.Platform != "dingtalk" || dingTalkMessage.DingTalkCard == nil {
		t.Fatalf("dingTalkMessage = %+v", dingTalkMessage)
	}
	if fallback := dingTalkMessage.DingTalkCard.FallbackText(); !strings.Contains(fallback, "Build Ready") || !strings.Contains(fallback, "Build passed") {
		t.Fatalf("dingtalk fallback = %q", fallback)
	}

	weComMessage, err := NewWeComCardMessage(
		WeComCardTypeNews,
		"Build Ready",
		"Agent finished the run.",
		"https://example.test/builds/1",
		[]WeComArticle{{
			Title:       "Build #1",
			Description: "Agent finished the run.",
			URL:         "https://example.test/builds/1",
		}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewWeComCardMessage error: %v", err)
	}
	if weComMessage.Platform != "wecom" || weComMessage.WeComCard == nil {
		t.Fatalf("weComMessage = %+v", weComMessage)
	}
	if fallback := weComMessage.WeComCard.FallbackText(); !strings.Contains(fallback, "Build #1") {
		t.Fatalf("wecom fallback = %q", fallback)
	}

	qqBotMessage, err := NewQQBotMarkdownMessage(
		"## Build Ready",
		[][]QQBotKeyboardButton{{
			{Label: "Open", URL: "https://example.test/builds/1"},
		}},
	)
	if err != nil {
		t.Fatalf("NewQQBotMarkdownMessage error: %v", err)
	}
	if qqBotMessage.Platform != "qqbot" || qqBotMessage.QQBotMarkdown == nil {
		t.Fatalf("qqBotMessage = %+v", qqBotMessage)
	}
	if fallback := qqBotMessage.QQBotMarkdown.FallbackText(); !strings.Contains(fallback, "Build Ready") || !strings.Contains(fallback, "Open") {
		t.Fatalf("qqbot fallback = %q", fallback)
	}
}

func TestNativeMessage_ValidateRejectsInvalidPayloadsAndMultipleSurfaces(t *testing.T) {
	if _, err := NewSlackBlockKitMessage(make([]map[string]any, 51)); err == nil {
		t.Fatal("expected slack block limit validation to fail")
	}

	if _, err := NewDiscordEmbedMessage("", "", nil, 0, nil); err == nil {
		t.Fatal("expected discord embed without title or description to fail")
	}

	if _, err := NewTelegramRichMessage(strings.Repeat("a", 4097), "MarkdownV2", nil); err == nil {
		t.Fatal("expected telegram max length validation to fail")
	}

	if _, err := NewDingTalkCardMessage(DingTalkCardTypeActionCard, "", "body", nil); err == nil {
		t.Fatal("expected dingtalk title validation to fail")
	}

	if _, err := NewWeComCardMessage(WeComCardTypeNews, "", "", "", nil, nil); err == nil {
		t.Fatal("expected wecom validation to fail")
	}

	if _, err := NewQQBotMarkdownMessage("", nil); err == nil {
		t.Fatal("expected qqbot markdown validation to fail")
	}

	message := &NativeMessage{
		SlackBlockKit: &SlackBlockKitPayload{Blocks: json.RawMessage(`[{"type":"section","text":{"type":"mrkdwn","text":"hello"}}]`)},
		DiscordEmbed:  &DiscordEmbedPayload{Title: "Build Ready"},
	}
	if err := message.Validate(); err == nil {
		t.Fatal("expected multiple native surfaces to fail validation")
	}
}

func TestDeliverNative_UsesUpdaterWhenDeferredCardUpdateIsPreferred(t *testing.T) {
	platform := &nativeDeliveryTestPlatform{}
	metadata := PlatformMetadata{
		Source: "feishu",
		Capabilities: PlatformCapabilities{
			AsyncUpdateModes: []AsyncUpdateMode{AsyncUpdateDeferredCardUpdate},
		},
	}
	message := &NativeMessage{
		Platform: "feishu",
		FeishuCard: &FeishuCardPayload{
			Mode:       FeishuCardModeTemplate,
			TemplateID: "ctp_123",
		},
	}

	plan, err := DeliverNative(context.Background(), platform, metadata, &ReplyTarget{
		Platform:      "feishu",
		ChatID:        "chat-1",
		CallbackToken: "cb-token-1",
		ProgressMode:  string(AsyncUpdateDeferredCardUpdate),
	}, "", message)
	if err != nil {
		t.Fatalf("DeliverNative error: %v", err)
	}
	if plan.Method != DeliveryMethodDeferredCardUpdate {
		t.Fatalf("method = %q, want %q", plan.Method, DeliveryMethodDeferredCardUpdate)
	}
	if len(platform.nativeUpdates) != 1 {
		t.Fatalf("nativeUpdates = %d, want 1", len(platform.nativeUpdates))
	}
	if len(platform.nativeSent) != 0 || len(platform.nativeReplies) != 0 {
		t.Fatalf("sent=%d replies=%d", len(platform.nativeSent), len(platform.nativeReplies))
	}
}
