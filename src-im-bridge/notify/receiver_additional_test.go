package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

type structuredNotificationPlatform struct {
	replyAwareTextPlatform
	metadata   core.PlatformMetadata
	structured []*core.StructuredMessage
}

func (p *structuredNotificationPlatform) Metadata() core.PlatformMetadata {
	return p.metadata
}

func (p *structuredNotificationPlatform) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	p.chat = append(p.chat, chatID)
	p.structured = append(p.structured, message)
	return nil
}

func TestReceiver_HandleSend_RequiresChatID(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	body, err := json.Marshal(SendRequest{
		Platform: "slack",
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("marshal send request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestReceiver_HandleSend_RejectsPlatformMismatch(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	body, err := json.Marshal(SendRequest{
		Platform: "discord",
		ChatID:   "chat-1",
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("marshal send request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestReceiver_HandleSend_WritesStructuredReceipt(t *testing.T) {
	p := &structuredNotificationPlatform{
		replyAwareTextPlatform: replyAwareTextPlatform{textOnlyPlatform: textOnlyPlatform{name: "discord-stub"}},
		metadata: core.PlatformMetadata{
			Source: "discord",
			Capabilities: core.PlatformCapabilities{
				StructuredSurface: core.StructuredSurfaceComponents,
			},
		},
	}
	r := NewReceiverWithMetadata(p, p.metadata, "0")

	body, err := json.Marshal(SendRequest{
		Platform: "discord",
		ChatID:   "channel-1",
		Structured: &core.StructuredMessage{
			Title: "Task Update",
			Body:  "Agent is running.",
		},
	})
	if err != nil {
		t.Fatalf("marshal send request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.structured) != 1 {
		t.Fatalf("structured = %d, want 1", len(p.structured))
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["type"] != "structured" || payload["delivery_method"] != string(core.DeliveryMethodSend) {
		t.Fatalf("payload = %+v", payload)
	}
}

func TestReceiver_HandleAction_RequiresConfiguredHandler(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	body, err := json.Marshal(ActionRequest{
		Action:   "approve",
		EntityID: "review-1",
		ChatID:   "chat-1",
	})
	if err != nil {
		t.Fatalf("marshal action request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/action", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleAction(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestReceiver_HandleAction_RequiresActionAndEntityID(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	body, err := json.Marshal(ActionRequest{
		EntityID: "review-1",
	})
	if err != nil {
		t.Fatalf("marshal action request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/action", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleAction(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestClassifyNativeFallbackReason_MapsExpectedBuckets(t *testing.T) {
	tests := []struct {
		name   string
		target *core.ReplyTarget
		err    error
		want   string
	}{
		{
			name: "missing context",
			err:  errors.New("anything"),
			want: "missing_delayed_update_context",
		},
		{
			name: "expired token",
			target: &core.ReplyTarget{
				CallbackToken: "cb-token-1",
			},
			err:  errors.New("callback expired"),
			want: "delayed_update_context_expired",
		},
		{
			name: "used token",
			target: &core.ReplyTarget{
				CallbackToken: "cb-token-1",
			},
			err:  errors.New("token already used"),
			want: "delayed_update_context_exhausted",
		},
		{
			name: "invalid token",
			target: &core.ReplyTarget{
				CallbackToken: "cb-token-1",
			},
			err:  errors.New("invalid callback token"),
			want: "invalid_delayed_update_context",
		},
		{
			name: "generic failure",
			target: &core.ReplyTarget{
				CallbackToken: "cb-token-1",
			},
			err:  errors.New("upstream timeout"),
			want: "native_update_failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyNativeFallbackReason(tc.err, tc.target); got != tc.want {
				t.Fatalf("classifyNativeFallbackReason() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCloneMetadata_ReturnsIndependentWritableCopy(t *testing.T) {
	cloned := cloneMetadata(nil)
	cloned["new"] = "value"
	if len(cloned) != 1 {
		t.Fatalf("cloneMetadata(nil) = %+v", cloned)
	}

	original := map[string]string{"status": "started"}
	copy := cloneMetadata(original)
	copy["status"] = "completed"
	copy["new"] = "field"

	if original["status"] != "started" {
		t.Fatalf("original mutated = %+v", original)
	}
	if copy["status"] != "completed" || copy["new"] != "field" {
		t.Fatalf("copy = %+v", copy)
	}
}
