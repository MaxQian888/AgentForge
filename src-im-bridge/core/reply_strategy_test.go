package core

import (
	"context"
	"testing"
)

type deliveryTestPlatform struct {
	sentContent    []string
	replyContent   []string
	updatedContent []string
}

func (p *deliveryTestPlatform) Name() string { return "delivery-test" }

func (p *deliveryTestPlatform) Start(handler MessageHandler) error { return nil }

func (p *deliveryTestPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	p.replyContent = append(p.replyContent, content)
	return nil
}

func (p *deliveryTestPlatform) Send(ctx context.Context, chatID string, content string) error {
	p.sentContent = append(p.sentContent, chatID+":"+content)
	return nil
}

func (p *deliveryTestPlatform) Stop() error { return nil }

func (p *deliveryTestPlatform) UpdateMessage(ctx context.Context, replyCtx any, content string) error {
	p.updatedContent = append(p.updatedContent, content)
	return nil
}

func (p *deliveryTestPlatform) ReplyContextFromTarget(target *ReplyTarget) any {
	return target
}

func TestResolveReplyPlan_SelectsPlatformNativeUpdatePath(t *testing.T) {
	metadata := PlatformMetadata{
		Source: "dingtalk",
		Capabilities: PlatformCapabilities{
			AsyncUpdateModes: []AsyncUpdateMode{
				AsyncUpdateSessionWebhook,
				AsyncUpdateFollowUp,
				AsyncUpdateThreadReply,
			},
			Mutability: MutabilitySemantics{CanEdit: true},
		},
	}

	tests := []struct {
		name   string
		target *ReplyTarget
		want   DeliveryMethod
	}{
		{
			name:   "session webhook wins when preserved",
			target: &ReplyTarget{SessionWebhook: "https://session.example/reply", ChatID: "cid1"},
			want:   DeliveryMethodSessionWebhook,
		},
		{
			name:   "edit wins when explicitly preferred",
			target: &ReplyTarget{MessageID: "msg-1", PreferEdit: true, ChatID: "chat-1"},
			want:   DeliveryMethodEdit,
		},
		{
			name:   "interaction token becomes follow up",
			target: &ReplyTarget{InteractionToken: "token-1", ChatID: "chat-1"},
			want:   DeliveryMethodFollowUp,
		},
		{
			name:   "thread target stays threaded",
			target: &ReplyTarget{ThreadID: "thread-1", ChatID: "chat-1"},
			want:   DeliveryMethodThreadReply,
		},
		{
			name:   "reply flag falls back to reply",
			target: &ReplyTarget{UseReply: true, ChatID: "chat-1"},
			want:   DeliveryMethodReply,
		},
		{
			name:   "empty target becomes send",
			target: &ReplyTarget{ChatID: "chat-1"},
			want:   DeliveryMethodSend,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			plan := ResolveReplyPlan(metadata, tc.target, "")
			if plan.Method != tc.want {
				t.Fatalf("method = %q, want %q", plan.Method, tc.want)
			}
		})
	}
}

func TestDeliverText_UsesUpdaterWhenEditPreferredAndSupported(t *testing.T) {
	platform := &deliveryTestPlatform{}
	metadata := PlatformMetadata{
		Source: "telegram",
		Capabilities: PlatformCapabilities{
			Mutability: MutabilitySemantics{CanEdit: true},
		},
	}

	plan, err := DeliverText(context.Background(), platform, metadata, &ReplyTarget{
		Platform:   "telegram",
		ChatID:     "chat-1",
		MessageID:  "msg-1",
		PreferEdit: true,
	}, "", "edited")
	if err != nil {
		t.Fatalf("DeliverText error: %v", err)
	}
	if plan.Method != DeliveryMethodEdit {
		t.Fatalf("method = %q, want %q", plan.Method, DeliveryMethodEdit)
	}
	if len(platform.updatedContent) != 1 || platform.updatedContent[0] != "edited" {
		t.Fatalf("updatedContent = %v", platform.updatedContent)
	}
	if len(platform.replyContent) != 0 || len(platform.sentContent) != 0 {
		t.Fatalf("reply=%v send=%v", platform.replyContent, platform.sentContent)
	}
}

func TestDeliverText_FallsBackToSendWhenReplyTargetCannotBeRestored(t *testing.T) {
	platform := &deliveryTestPlatform{}
	metadata := PlatformMetadata{
		Source: "slack",
		Capabilities: PlatformCapabilities{
			AsyncUpdateModes: []AsyncUpdateMode{AsyncUpdateThreadReply},
		},
	}

	plan, err := DeliverText(context.Background(), platform, metadata, nil, "chat-1", "hello")
	if err != nil {
		t.Fatalf("DeliverText error: %v", err)
	}
	if plan.Method != DeliveryMethodSend {
		t.Fatalf("method = %q, want %q", plan.Method, DeliveryMethodSend)
	}
	if len(platform.sentContent) != 1 || platform.sentContent[0] != "chat-1:hello" {
		t.Fatalf("sentContent = %v", platform.sentContent)
	}
}
