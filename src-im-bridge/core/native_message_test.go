package core

import (
	"context"
	"encoding/json"
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
