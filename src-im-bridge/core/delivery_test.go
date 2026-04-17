package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type structuredDeliveryTestPlatform struct {
	deliveryTestPlatform
	structuredSent    []*StructuredMessage
	structuredReplies []*StructuredMessage
}

func (p *structuredDeliveryTestPlatform) SendStructured(ctx context.Context, chatID string, message *StructuredMessage) error {
	p.structuredSent = append(p.structuredSent, message)
	p.sentContent = append(p.sentContent, chatID+":"+message.FallbackText())
	return nil
}

func (p *structuredDeliveryTestPlatform) ReplyStructured(ctx context.Context, replyCtx any, message *StructuredMessage) error {
	p.structuredReplies = append(p.structuredReplies, message)
	return nil
}

type cardDeliveryTestPlatform struct {
	deliveryTestPlatform
	sentCards  []*Card
	replyCards []*Card
}

func (p *cardDeliveryTestPlatform) SendCard(ctx context.Context, chatID string, card *Card) error {
	p.sentCards = append(p.sentCards, card)
	p.sentContent = append(p.sentContent, chatID+":"+card.Title)
	return nil
}

func (p *cardDeliveryTestPlatform) ReplyCard(ctx context.Context, replyCtx any, card *Card) error {
	p.replyCards = append(p.replyCards, card)
	return nil
}

func testFeishuTemplateNativeMessage() *NativeMessage {
	return &NativeMessage{
		Platform: "feishu",
		FeishuCard: &FeishuCardPayload{
			Mode:       FeishuCardModeTemplate,
			TemplateID: "ctp_delivery",
		},
	}
}

func TestDeliverEnvelope_RejectsNilInputs(t *testing.T) {
	if _, err := DeliverEnvelope(context.Background(), nil, PlatformMetadata{}, "chat-1", &DeliveryEnvelope{}); err == nil {
		t.Fatal("expected nil platform to fail")
	}

	if _, err := DeliverEnvelope(context.Background(), &deliveryTestPlatform{}, PlatformMetadata{}, "chat-1", nil); err == nil {
		t.Fatal("expected nil delivery to fail")
	}
}

func TestResolveRenderingPlan_UsesProviderDefaultTextFormat(t *testing.T) {
	plan, err := ResolveRenderingPlan(PlatformMetadata{
		Source: "telegram",
		Rendering: RenderingProfile{
			DefaultTextFormat: TextFormatMarkdownV2,
		},
	}, "chat-1", &DeliveryEnvelope{
		Content: "build *status*",
	})
	if err != nil {
		t.Fatalf("ResolveRenderingPlan error: %v", err)
	}
	if plan.Type != "text" || len(plan.Text) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Text[0].Format != TextFormatMarkdownV2 {
		t.Fatalf("text format = %q, want %q", plan.Text[0].Format, TextFormatMarkdownV2)
	}
}

func TestResolveRenderingPlan_PrefersNativePayloadsBeforeExecution(t *testing.T) {
	plan, err := ResolveRenderingPlan(PlatformMetadata{
		Source: "feishu",
		Capabilities: PlatformCapabilities{
			AsyncUpdateModes: []AsyncUpdateMode{AsyncUpdateReply, AsyncUpdateDeferredCardUpdate},
		},
	}, "", &DeliveryEnvelope{
		Native: testFeishuTemplateNativeMessage(),
		ReplyTarget: &ReplyTarget{
			Platform:  "feishu",
			ChatID:    "chat-1",
			UseReply:  true,
			MessageID: "msg-1",
		},
	})
	if err != nil {
		t.Fatalf("ResolveRenderingPlan error: %v", err)
	}
	if plan.Type != "native" || plan.Native == nil {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Method != DeliveryMethodReply {
		t.Fatalf("delivery method = %q, want %q", plan.Method, DeliveryMethodReply)
	}
}

func TestResolveRenderingPlan_FallsBackWhenNativePlatformMismatches(t *testing.T) {
	plan, err := ResolveRenderingPlan(PlatformMetadata{
		Source: "telegram",
		Rendering: RenderingProfile{
			NativeSurfaces: []string{NativeSurfaceTelegramRich},
		},
	}, "chat-1", &DeliveryEnvelope{
		Native: &NativeMessage{
			Platform: "slack",
			SlackBlockKit: &SlackBlockKitPayload{
				Blocks: json.RawMessage(`[{"type":"section","text":{"type":"mrkdwn","text":"*Build* ready"}}]`),
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveRenderingPlan error: %v", err)
	}
	if plan.Type != "text" || len(plan.Text) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.FallbackReason != "native_platform_mismatch" {
		t.Fatalf("fallback reason = %q", plan.FallbackReason)
	}
	if !strings.Contains(plan.Text[0].Content, "Build ready") {
		t.Fatalf("fallback text = %q", plan.Text[0].Content)
	}
}

func TestResolveRenderingPlan_FallsBackWhenNativeSurfaceUnsupported(t *testing.T) {
	plan, err := ResolveRenderingPlan(PlatformMetadata{
		Source: "slack",
		Rendering: RenderingProfile{
			NativeSurfaces: []string{NativeSurfaceFeishuCard},
		},
	}, "chat-1", &DeliveryEnvelope{
		Native: &NativeMessage{
			Platform: "slack",
			SlackBlockKit: &SlackBlockKitPayload{
				Blocks: json.RawMessage(`[{"type":"section","text":{"type":"mrkdwn","text":"*Build* ready"}}]`),
			},
		},
	})
	if err != nil {
		t.Fatalf("ResolveRenderingPlan error: %v", err)
	}
	if plan.Type != "text" || len(plan.Text) != 1 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.FallbackReason != "native_surface_unsupported" {
		t.Fatalf("fallback reason = %q", plan.FallbackReason)
	}
	if !strings.Contains(plan.Text[0].Content, "Build ready") {
		t.Fatalf("fallback text = %q", plan.Text[0].Content)
	}
}

func TestDeliverEnvelope_UsesExplicitNativePayloadAndMetadataFallback(t *testing.T) {
	platform := &nativeDeliveryTestPlatform{}

	receipt, err := DeliverEnvelope(context.Background(), platform, PlatformMetadata{Source: "feishu"}, "chat-1", &DeliveryEnvelope{
		Native: testFeishuTemplateNativeMessage(),
		Metadata: map[string]string{
			"fallback_reason": "  queued_delivery  ",
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Type != "native" || receipt.Method != DeliveryMethodSend {
		t.Fatalf("receipt = %+v", receipt)
	}
	if receipt.FallbackReason != "queued_delivery" {
		t.Fatalf("fallback_reason = %q", receipt.FallbackReason)
	}
	if len(platform.nativeSent) != 1 || platform.nativeSent[0].FeishuCard.TemplateID != "ctp_delivery" {
		t.Fatalf("nativeSent = %+v", platform.nativeSent)
	}
}

func TestDeliverEnvelope_PrefersPlanFallbackReasonForNativeReply(t *testing.T) {
	platform := &nativeDeliveryTestPlatform{}
	metadata := PlatformMetadata{
		Source: "feishu",
		Capabilities: PlatformCapabilities{
			AsyncUpdateModes: []AsyncUpdateMode{AsyncUpdateDeferredCardUpdate},
		},
	}

	receipt, err := DeliverEnvelope(context.Background(), platform, metadata, "", &DeliveryEnvelope{
		Native: testFeishuTemplateNativeMessage(),
		ReplyTarget: &ReplyTarget{
			Platform:     "feishu",
			ChatID:       "chat-1",
			ProgressMode: string(AsyncUpdateDeferredCardUpdate),
			UseReply:     true,
		},
		Metadata: map[string]string{
			"fallback_reason": "metadata_reason",
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Method != DeliveryMethodReply {
		t.Fatalf("delivery method = %q, want %q", receipt.Method, DeliveryMethodReply)
	}
	if !strings.Contains(receipt.FallbackReason, "missing callback token") {
		t.Fatalf("fallback_reason = %q", receipt.FallbackReason)
	}
	if receipt.FallbackReason == "metadata_reason" {
		t.Fatalf("expected plan fallback reason to win over metadata, got %q", receipt.FallbackReason)
	}
	if len(platform.nativeReplies) != 1 {
		t.Fatalf("nativeReplies = %d, want 1", len(platform.nativeReplies))
	}
}

func TestDeliverEnvelope_SynthesizesDeferredFeishuNativeUpdateFromStructuredPayload(t *testing.T) {
	platform := &nativeDeliveryTestPlatform{}
	metadata := PlatformMetadata{
		Source: "feishu",
		Capabilities: PlatformCapabilities{
			AsyncUpdateModes: []AsyncUpdateMode{AsyncUpdateDeferredCardUpdate},
		},
	}

	receipt, err := DeliverEnvelope(context.Background(), platform, metadata, "", &DeliveryEnvelope{
		Structured: &StructuredMessage{
			Title: "Task Update",
			Body:  "Agent completed implementation.",
		},
		ReplyTarget: &ReplyTarget{
			Platform:      "feishu",
			ChatID:        "oc_123",
			CallbackToken: "cb-token-1",
			ProgressMode:  string(AsyncUpdateDeferredCardUpdate),
			UseReply:      true,
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Type != "native" || receipt.Method != DeliveryMethodDeferredCardUpdate {
		t.Fatalf("receipt = %+v", receipt)
	}
	if len(platform.nativeUpdates) != 1 {
		t.Fatalf("nativeUpdates = %d, want 1", len(platform.nativeUpdates))
	}

	var payload map[string]any
	if err := json.Unmarshal(platform.nativeUpdates[0].FeishuCard.JSON, &payload); err != nil {
		t.Fatalf("decode synthesized card: %v", err)
	}
	header, ok := payload["header"].(map[string]any)
	if !ok || header["title"] == nil {
		t.Fatalf("payload = %+v", payload)
	}
	elements, ok := payload["elements"].([]any)
	if !ok || len(elements) != 1 {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestDeliverEnvelope_SendsStructuredPayloadWhenPlatformSupportsStructuredSender(t *testing.T) {
	platform := &structuredDeliveryTestPlatform{}
	receipt, err := DeliverEnvelope(context.Background(), platform, PlatformMetadata{
		Source: "discord",
		Capabilities: PlatformCapabilities{
			StructuredSurface: StructuredSurfaceComponents,
		},
	}, "channel-1", &DeliveryEnvelope{
		Structured: &StructuredMessage{
			Title: "Dispatch",
			Body:  "Agent is running.",
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Type != "structured" || receipt.Method != DeliveryMethodSend {
		t.Fatalf("receipt = %+v", receipt)
	}
	if len(platform.structuredSent) != 1 {
		t.Fatalf("structuredSent = %d, want 1", len(platform.structuredSent))
	}
}

func TestDeliverEnvelope_RepliesWithStructuredSectionsWhenPlatformSupportsReplyStructuredSender(t *testing.T) {
	platform := &structuredDeliveryTestPlatform{}
	receipt, err := DeliverEnvelope(context.Background(), platform, PlatformMetadata{
		Source: "slack",
		Capabilities: PlatformCapabilities{
			StructuredSurface: StructuredSurfaceBlocks,
			AsyncUpdateModes:  []AsyncUpdateMode{AsyncUpdateReply},
		},
	}, "", &DeliveryEnvelope{
		Structured: &StructuredMessage{
			Sections: []StructuredSection{
				{
					Type: StructuredSectionTypeText,
					TextSection: &TextSection{
						Body: "Build ready",
					},
				},
			},
		},
		ReplyTarget: &ReplyTarget{
			Platform:  "slack",
			ChatID:    "chat-1",
			ChannelID: "chat-1",
			UseReply:  true,
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Type != "structured" || receipt.Method != DeliveryMethodReply {
		t.Fatalf("receipt = %+v", receipt)
	}
	if len(platform.structuredReplies) != 1 {
		t.Fatalf("structuredReplies = %d, want 1", len(platform.structuredReplies))
	}
}

func TestDeliverEnvelope_UsesCardDeliveryWhenRendererPrefersCards(t *testing.T) {
	platform := &cardDeliveryTestPlatform{}
	receipt, err := DeliverEnvelope(context.Background(), platform, PlatformMetadata{
		Source: "feishu",
		Capabilities: PlatformCapabilities{
			StructuredSurface: StructuredSurfaceCards,
		},
	}, "chat-1", &DeliveryEnvelope{
		Structured: &StructuredMessage{
			Title: "Review Ready",
			Body:  "Please approve the rollout.",
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Type != "structured" || receipt.Method != DeliveryMethodSend {
		t.Fatalf("receipt = %+v", receipt)
	}
	if len(platform.sentCards) != 1 || platform.sentCards[0].Title != "Review Ready" {
		t.Fatalf("sentCards = %+v", platform.sentCards)
	}
}

func TestDeliverEnvelope_FallsBackToTextForStructuredPayload(t *testing.T) {
	tests := []struct {
		name       string
		target     *ReplyTarget
		wantMethod DeliveryMethod
		wantReason string
	}{
		{
			name:       "send fallback without reply target",
			wantMethod: DeliveryMethodSend,
			wantReason: "structured_delivery_unavailable",
		},
		{
			name: "reply fallback with reply target",
			target: &ReplyTarget{
				Platform: "slack",
				ChatID:   "chat-1",
				UseReply: true,
			},
			wantMethod: DeliveryMethodReply,
			wantReason: "structured_reply_unavailable",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			platform := &deliveryTestPlatform{}
			receipt, err := DeliverEnvelope(context.Background(), platform, PlatformMetadata{
				Source: "slack",
				Capabilities: PlatformCapabilities{
					AsyncUpdateModes: []AsyncUpdateMode{AsyncUpdateReply},
				},
			}, "chat-1", &DeliveryEnvelope{
				Structured: &StructuredMessage{
					Title: "Task Update",
					Body:  "Still running.",
				},
				ReplyTarget: tc.target,
			})
			if err != nil {
				t.Fatalf("DeliverEnvelope error: %v", err)
			}
			if receipt.Type != "text" || receipt.Method != tc.wantMethod || receipt.FallbackReason != tc.wantReason {
				t.Fatalf("receipt = %+v", receipt)
			}
		})
	}
}

func TestDeliverEnvelope_ReportsFallbackReasonsAcrossPlatforms(t *testing.T) {
	tests := []struct {
		name   string
		source string
		target *ReplyTarget
	}{
		{name: "slack send", source: "slack"},
		{name: "discord send", source: "discord"},
		{name: "telegram send", source: "telegram"},
		{name: "feishu send", source: "feishu"},
		{name: "wecom send", source: "wecom"},
		{name: "qq send", source: "qq"},
		{name: "qqbot send", source: "qqbot"},
		{
			name:   "dingtalk reply",
			source: "dingtalk",
			target: &ReplyTarget{
				Platform:       "dingtalk",
				ChatID:         "chat-1",
				SessionWebhook: "https://session.example/reply",
				UseReply:       true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			platform := &deliveryTestPlatform{}
			metadata := NormalizeMetadata(PlatformMetadata{Source: tc.source}, tc.source)
			receipt, err := DeliverEnvelope(context.Background(), platform, metadata, "chat-1", &DeliveryEnvelope{
				Structured: &StructuredMessage{
					Title: "Task Update",
					Body:  "Still running.",
				},
				ReplyTarget: tc.target,
			})
			if err != nil {
				t.Fatalf("DeliverEnvelope error: %v", err)
			}
			if strings.TrimSpace(receipt.FallbackReason) == "" {
				t.Fatalf("receipt = %+v, want fallback reason", receipt)
			}
		})
	}
}

func TestSynthesizeNativeDelivery_RequiresDeferredFeishuContextAndContent(t *testing.T) {
	if synthesized := synthesizeNativeDelivery(PlatformMetadata{Source: "slack"}, &DeliveryEnvelope{
		Content: "hello",
		ReplyTarget: &ReplyTarget{
			ProgressMode:  string(AsyncUpdateDeferredCardUpdate),
			CallbackToken: "cb-token-1",
		},
	}); synthesized != nil {
		t.Fatalf("expected non-feishu metadata to skip synthesis, got %+v", synthesized)
	}

	if synthesized := synthesizeNativeDelivery(PlatformMetadata{Source: "feishu"}, &DeliveryEnvelope{
		ReplyTarget: &ReplyTarget{
			ProgressMode: string(AsyncUpdateDeferredCardUpdate),
		},
	}); synthesized != nil {
		t.Fatalf("expected missing callback token to skip synthesis, got %+v", synthesized)
	}

	synthesized := synthesizeNativeDelivery(PlatformMetadata{Source: "feishu"}, &DeliveryEnvelope{
		Content: "  delayed update body  ",
		ReplyTarget: &ReplyTarget{
			ProgressMode:  string(AsyncUpdateDeferredCardUpdate),
			CallbackToken: "cb-token-1",
		},
	})
	if synthesized == nil || synthesized.FeishuCard == nil || synthesized.FeishuCard.Mode != FeishuCardModeJSON {
		t.Fatalf("synthesized = %+v", synthesized)
	}
}

func TestDeliveryHelpers_TrimAndResolveFallbackReasons(t *testing.T) {
	if got := inferStructuredFallbackReason(nil); got != "structured_delivery_unavailable" {
		t.Fatalf("inferStructuredFallbackReason(nil) = %q", got)
	}
	if got := inferStructuredFallbackReason(&ReplyTarget{}); got != "structured_reply_unavailable" {
		t.Fatalf("inferStructuredFallbackReason(target) = %q", got)
	}
	if got := metadataValue(map[string]string{"fallback_reason": " queued "}, "fallback_reason"); got != "queued" {
		t.Fatalf("metadataValue = %q", got)
	}
	if got := metadataValue(nil, "fallback_reason"); got != "" {
		t.Fatalf("metadataValue(nil) = %q", got)
	}
}

func TestDeliverEnvelope_ReportsFallbackReasonWhenQQCannotHonorEditableUpdate(t *testing.T) {
	platform := &deliveryTestPlatform{}
	metadata := NormalizeMetadata(PlatformMetadata{Source: "qq"}, "qq")

	receipt, err := DeliverEnvelope(context.Background(), platform, metadata, "chat-1", &DeliveryEnvelope{
		Content: "qq completion",
		ReplyTarget: &ReplyTarget{
			Platform:   "qq",
			ChatID:     "chat-1",
			MessageID:  "msg-1",
			PreferEdit: true,
			UseReply:   true,
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Method != DeliveryMethodReply {
		t.Fatalf("receipt = %+v, want reply fallback", receipt)
	}
	if !strings.Contains(receipt.FallbackReason, "editable updates") {
		t.Fatalf("fallback_reason = %q", receipt.FallbackReason)
	}
}

func TestDeliverEnvelope_ReportsFallbackReasonWhenQQBotCannotHonorDeferredUpdate(t *testing.T) {
	platform := &deliveryTestPlatform{}
	metadata := NormalizeMetadata(PlatformMetadata{Source: "qqbot"}, "qqbot")

	receipt, err := DeliverEnvelope(context.Background(), platform, metadata, "group-openid", &DeliveryEnvelope{
		Content: "qqbot completion",
		ReplyTarget: &ReplyTarget{
			Platform:      "qqbot",
			ChatID:        "group-openid",
			CallbackToken: "cb-token-1",
			ProgressMode:  string(AsyncUpdateDeferredCardUpdate),
			UseReply:      true,
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Method != DeliveryMethodReply {
		t.Fatalf("receipt = %+v, want reply fallback", receipt)
	}
	if !strings.Contains(receipt.FallbackReason, "deferred card updates") {
		t.Fatalf("fallback_reason = %q", receipt.FallbackReason)
	}
}

func TestDeliverEnvelope_SelectsEditPathWhenWeComSupportsMutableUpdate(t *testing.T) {
	platform := &deliveryTestPlatform{}
	metadata := NormalizeMetadata(PlatformMetadata{Source: "wecom"}, "wecom")

	receipt, err := DeliverEnvelope(context.Background(), platform, metadata, "chat-1", &DeliveryEnvelope{
		Content: "wecom completion",
		ReplyTarget: &ReplyTarget{
			Platform:   "wecom",
			ChatID:     "chat-1",
			MessageID:  "msg-1",
			PreferEdit: true,
			UseReply:   true,
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Method != DeliveryMethodEdit {
		t.Fatalf("receipt = %+v, want edit method", receipt)
	}
	if metadata.Capabilities.MutableUpdateMethod != "template_card_update" {
		t.Fatalf("wecom MutableUpdateMethod = %q, want template_card_update", metadata.Capabilities.MutableUpdateMethod)
	}
}

func TestDeliverEnvelope_SelectsEditPathWhenDingTalkSupportsOpenAPIUpdate(t *testing.T) {
	platform := &deliveryTestPlatform{}
	metadata := NormalizeMetadata(PlatformMetadata{Source: "dingtalk"}, "dingtalk")

	receipt, err := DeliverEnvelope(context.Background(), platform, metadata, "chat-1", &DeliveryEnvelope{
		Content: "dingtalk completion",
		ReplyTarget: &ReplyTarget{
			Platform:   "dingtalk",
			ChatID:     "chat-1",
			MessageID:  "msg-1",
			PreferEdit: true,
			UseReply:   true,
		},
	})
	if err != nil {
		t.Fatalf("DeliverEnvelope error: %v", err)
	}
	if receipt.Method != DeliveryMethodEdit {
		t.Fatalf("receipt = %+v, want edit method", receipt)
	}
	if metadata.Capabilities.MutableUpdateMethod != "openapi_only" {
		t.Fatalf("dingtalk MutableUpdateMethod = %q, want openapi_only", metadata.Capabilities.MutableUpdateMethod)
	}
}
