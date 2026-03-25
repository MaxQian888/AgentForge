package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

type nativeNotificationPlatform struct {
	textOnlyPlatform
	metadata     core.PlatformMetadata
	nativeSent   []*core.NativeMessage
	nativeUpdate []*core.NativeMessage
	updateErr    error
}

func (p *nativeNotificationPlatform) Metadata() core.PlatformMetadata {
	return p.metadata
}

func (p *nativeNotificationPlatform) ReplyContextFromTarget(target *core.ReplyTarget) any {
	return target
}

func (p *nativeNotificationPlatform) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	p.nativeSent = append(p.nativeSent, message)
	return nil
}

func (p *nativeNotificationPlatform) ReplyNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	p.nativeSent = append(p.nativeSent, message)
	return nil
}

func (p *nativeNotificationPlatform) UpdateNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	if p.updateErr != nil {
		return p.updateErr
	}
	p.nativeUpdate = append(p.nativeUpdate, message)
	return nil
}

func TestReceiver_PrefersNativePayloadWhenPlatformSupportsIt(t *testing.T) {
	p := &nativeNotificationPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "feishu-stub"},
		metadata: core.PlatformMetadata{
			Source: "feishu",
			Capabilities: core.PlatformCapabilities{
				StructuredSurface:    core.StructuredSurfaceCards,
				AsyncUpdateModes:     []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateDeferredCardUpdate},
				SupportsRichMessages: true,
			},
		},
	}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "feishu",
		TargetChatID: "chat-1",
		Content:      "fallback",
		Native: &core.NativeMessage{
			Platform: "feishu",
			FeishuCard: &core.FeishuCardPayload{
				Mode:       core.FeishuCardModeTemplate,
				TemplateID: "ctp_native",
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest("POST", "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(p.nativeSent) != 1 {
		t.Fatalf("nativeSent = %d, want 1", len(p.nativeSent))
	}
	if len(p.sent) != 0 {
		t.Fatalf("text fallback should not be used, sent=%v", p.sent)
	}
}

func TestReceiver_UsesDeferredNativeUpdateWhenFeishuReplyTargetSupportsIt(t *testing.T) {
	p := &nativeNotificationPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "feishu-stub"},
		metadata: core.PlatformMetadata{
			Source: "feishu",
			Capabilities: core.PlatformCapabilities{
				StructuredSurface:    core.StructuredSurfaceCards,
				AsyncUpdateModes:     []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateDeferredCardUpdate},
				SupportsRichMessages: true,
			},
		},
	}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "feishu",
		TargetChatID: "chat-1",
		Native: &core.NativeMessage{
			Platform: "feishu",
			FeishuCard: &core.FeishuCardPayload{
				Mode:       core.FeishuCardModeTemplate,
				TemplateID: "ctp_deferred",
			},
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:      "feishu",
			ChatID:        "chat-1",
			CallbackToken: "cb-token-1",
			ProgressMode:  string(core.AsyncUpdateDeferredCardUpdate),
		},
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest("POST", "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(p.nativeUpdate) != 1 {
		t.Fatalf("nativeUpdate = %d, want 1", len(p.nativeUpdate))
	}
	if len(p.nativeSent) != 0 {
		t.Fatalf("nativeSent = %d, want 0", len(p.nativeSent))
	}
}

func TestReceiver_ReportsFallbackReasonWhenDeferredUpdateContextMissing(t *testing.T) {
	p := &nativeNotificationPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "feishu-stub"},
		metadata: core.PlatformMetadata{
			Source: "feishu",
			Capabilities: core.PlatformCapabilities{
				StructuredSurface:    core.StructuredSurfaceCards,
				AsyncUpdateModes:     []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateDeferredCardUpdate},
				SupportsRichMessages: true,
			},
		},
	}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "feishu",
		TargetChatID: "chat-1",
		Content:      "fallback text",
		Native: &core.NativeMessage{
			Platform: "feishu",
			FeishuCard: &core.FeishuCardPayload{
				Mode:       core.FeishuCardModeTemplate,
				TemplateID: "ctp_missing",
			},
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:     "feishu",
			ChatID:       "chat-1",
			ProgressMode: string(core.AsyncUpdateDeferredCardUpdate),
			UseReply:     true,
		},
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest("POST", "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if len(p.nativeUpdate) != 0 {
		t.Fatalf("nativeUpdate = %d, want 0", len(p.nativeUpdate))
	}
	if len(p.nativeSent) != 1 {
		t.Fatalf("nativeSent = %d, want 1", len(p.nativeSent))
	}

	var response map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response["delivery_method"] != string(core.DeliveryMethodReply) {
		t.Fatalf("response = %+v", response)
	}
	if response["fallback_reason"] == "" {
		t.Fatalf("response = %+v, want fallback_reason", response)
	}
}
