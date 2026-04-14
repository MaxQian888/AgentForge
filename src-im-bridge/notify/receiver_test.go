package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
)

type textOnlyPlatform struct {
	name    string
	sent    []string
	chat    []string
	replies []string
	sendErr error
}

func (p *textOnlyPlatform) Name() string                            { return p.name }
func (p *textOnlyPlatform) Start(handler core.MessageHandler) error { return nil }
func (p *textOnlyPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	p.replies = append(p.replies, content)
	return nil
}
func (p *textOnlyPlatform) Stop() error { return nil }
func (p *textOnlyPlatform) Send(ctx context.Context, chatID string, content string) error {
	if p.sendErr != nil {
		return p.sendErr
	}
	p.chat = append(p.chat, chatID)
	p.sent = append(p.sent, content)
	return nil
}

type replyAwareTextPlatform struct {
	textOnlyPlatform
}

func (p *replyAwareTextPlatform) ReplyContextFromTarget(target *core.ReplyTarget) any {
	return target
}

type feishuActionNativePlatform struct {
	replyAwareTextPlatform
	metadata     core.PlatformMetadata
	nativeSent   []*core.NativeMessage
	nativeUpdate []*core.NativeMessage
}

func (p *feishuActionNativePlatform) Metadata() core.PlatformMetadata {
	return p.metadata
}

func (p *feishuActionNativePlatform) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	p.nativeSent = append(p.nativeSent, message)
	return nil
}

func (p *feishuActionNativePlatform) ReplyNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	p.nativeSent = append(p.nativeSent, message)
	return nil
}

func (p *feishuActionNativePlatform) UpdateNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	p.nativeUpdate = append(p.nativeUpdate, message)
	return nil
}

type cardPlatform struct {
	textOnlyPlatform
	cardTitles  []string
	sendCardErr error
}

func (p *cardPlatform) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	if p.sendCardErr != nil {
		return p.sendCardErr
	}
	p.chat = append(p.chat, chatID)
	p.cardTitles = append(p.cardTitles, card.Title)
	return nil
}

func (p *cardPlatform) ReplyCard(ctx context.Context, replyCtx any, card *core.Card) error {
	return nil
}

type capabilityAwareCardPlatform struct {
	cardPlatform
	metadata core.PlatformMetadata
}

func (p *capabilityAwareCardPlatform) Metadata() core.PlatformMetadata {
	return p.metadata
}

type capabilityAwareTextPlatform struct {
	textOnlyPlatform
	metadata core.PlatformMetadata
}

func (p *capabilityAwareTextPlatform) Metadata() core.PlatformMetadata {
	return p.metadata
}

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

type replyTargetActionHandler struct {
	response *ActionResponse
}

func (h *replyTargetActionHandler) HandleAction(ctx context.Context, req *ActionRequest) (*ActionResponse, error) {
	return h.response, nil
}

func TestReceiver_RejectsPlatformMismatch(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	body, err := json.Marshal(Notification{
		Platform:     "dingtalk",
		TargetChatID: "chat-1",
		Content:      "hello",
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestReceiver_FallsBackToTextWhenCardSenderUnavailable(t *testing.T) {
	p := &textOnlyPlatform{name: "slack-stub"}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "slack",
		TargetChatID: "chat-1",
		Content:      "plain fallback",
		Card:         core.NewCard().SetTitle("card title"),
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.sent) != 1 || p.sent[0] != "plain fallback" {
		t.Fatalf("sent = %v, want plain fallback", p.sent)
	}
}

func TestReceiver_SendsCardWhenPlatformSupportsCards(t *testing.T) {
	p := &cardPlatform{textOnlyPlatform: textOnlyPlatform{name: "dingtalk-stub"}}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "dingtalk",
		TargetChatID: "chat-1",
		Content:      "plain fallback",
		Card:         core.NewCard().SetTitle("card title"),
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.cardTitles) != 1 || p.cardTitles[0] != "card title" {
		t.Fatalf("cardTitles = %v, want [card title]", p.cardTitles)
	}
}

func TestReceiver_FallsBackToTextWhenCapabilitiesDisableRichMessages(t *testing.T) {
	p := &capabilityAwareCardPlatform{
		cardPlatform: cardPlatform{textOnlyPlatform: textOnlyPlatform{name: "dingtalk-stub"}},
		metadata: core.PlatformMetadata{
			Source: "dingtalk",
			Capabilities: core.PlatformCapabilities{
				SupportsRichMessages: false,
			},
		},
	}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "dingtalk",
		TargetChatID: "chat-1",
		Content:      "plain fallback",
		Card:         core.NewCard().SetTitle("card title"),
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.cardTitles) != 0 {
		t.Fatalf("cardTitles = %v, want []", p.cardTitles)
	}
	if len(p.sent) != 1 || p.sent[0] != "plain fallback" {
		t.Fatalf("sent = %v, want plain fallback", p.sent)
	}
}

func TestReceiver_RejectsInvalidJSON(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestReceiver_RequiresPlatform(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	body, err := json.Marshal(Notification{
		TargetChatID: "chat-1",
		Content:      "hello",
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestReceiver_UsesTargetUserIDAsFallbackChatID(t *testing.T) {
	p := &textOnlyPlatform{name: "slack-stub"}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:       "slack",
		TargetIMUserID: "user-1",
		Content:        "hello",
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.chat) != 1 || p.chat[0] != "user-1" {
		t.Fatalf("chat = %v, want [user-1]", p.chat)
	}
}

func TestReceiver_ReturnsErrorWhenPlainTextSendFails(t *testing.T) {
	p := &textOnlyPlatform{name: "slack-stub", sendErr: errors.New("send failed")}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "slack",
		TargetChatID: "chat-1",
		Content:      "hello",
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestReceiver_ReturnsErrorWhenCardSendFails(t *testing.T) {
	p := &cardPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "dingtalk-stub"},
		sendCardErr:      errors.New("card failed"),
	}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "dingtalk",
		TargetChatID: "chat-1",
		Content:      "fallback",
		Card:         core.NewCard().SetTitle("card title"),
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestReceiver_StartExposesHealthAndStopShutsDownServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, strconv.Itoa(port))
	done := make(chan error, 1)
	go func() {
		done <- r.Start()
	}()

	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/im/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !bytes.Contains(body, []byte(`"platform":"slack-stub"`)) {
		t.Fatalf("body = %s", string(body))
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Start returned error after stop: %v", err)
	}
}

func TestReceiver_StopWithoutStartedServerIsNoop(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
}

func TestReceiver_HealthReportsNormalizedTelegramSourceAndCapabilities(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	r := NewReceiver(&capabilityAwareTextPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "telegram-stub"},
		metadata: core.PlatformMetadata{
			Source: "telegram",
			Capabilities: core.PlatformCapabilities{
				StructuredSurface:  core.StructuredSurfaceInlineKeyboard,
				ActionCallbackMode: core.ActionCallbackQuery,
				MessageScopes:      []core.MessageScope{core.MessageScopeChat, core.MessageScopeTopic},
				SupportsMentions:   true,
			},
		},
	}, strconv.Itoa(port))
	done := make(chan error, 1)
	go func() {
		done <- r.Start()
	}()

	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/im/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["platform"] != "telegram-stub" {
		t.Fatalf("platform = %v", payload["platform"])
	}
	if payload["source"] != "telegram" {
		t.Fatalf("source = %v", payload["source"])
	}
	if payload["supports_rich_messages"] != false {
		t.Fatalf("supports_rich_messages = %v", payload["supports_rich_messages"])
	}
	matrix, ok := payload["capability_matrix"].(map[string]any)
	if !ok {
		t.Fatalf("capability_matrix = %#v", payload["capability_matrix"])
	}
	if matrix["structuredSurface"] != "inline_keyboard" {
		t.Fatalf("structuredSurface = %v", matrix["structuredSurface"])
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Start returned error after stop: %v", err)
	}
}

func TestReceiver_HealthReportsNormalizedWeComSourceAndCapabilities(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	r := NewReceiver(&capabilityAwareTextPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "wecom-live"},
		metadata: core.PlatformMetadata{
			Source: "wecom",
			Capabilities: core.PlatformCapabilities{
				StructuredSurface:      core.StructuredSurfaceCards,
				ActionCallbackMode:     core.ActionCallbackWebhook,
				AsyncUpdateModes:       []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateSessionWebhook},
				MessageScopes:          []core.MessageScope{core.MessageScopeChat},
				RequiresPublicCallback: true,
				SupportsMentions:       true,
				SupportsSlashCommands:  true,
				SupportsRichMessages:   true,
			},
		},
	}, strconv.Itoa(port))
	done := make(chan error, 1)
	go func() {
		done <- r.Start()
	}()

	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/im/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["platform"] != "wecom-live" {
		t.Fatalf("platform = %v", payload["platform"])
	}
	if payload["source"] != "wecom" {
		t.Fatalf("source = %v", payload["source"])
	}
	if payload["readiness_tier"] != string(core.ReadinessTierNativeSendWithFallback) {
		t.Fatalf("readiness_tier = %v", payload["readiness_tier"])
	}
	if payload["supports_rich_messages"] != true {
		t.Fatalf("supports_rich_messages = %v", payload["supports_rich_messages"])
	}
	matrix, ok := payload["capability_matrix"].(map[string]any)
	if !ok {
		t.Fatalf("capability_matrix = %#v", payload["capability_matrix"])
	}
	if matrix["readinessTier"] != string(core.ReadinessTierNativeSendWithFallback) {
		t.Fatalf("capability_matrix = %#v", matrix)
	}
	if matrix["structuredSurface"] != "cards" {
		t.Fatalf("structuredSurface = %v", matrix["structuredSurface"])
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Start returned error after stop: %v", err)
	}
}

func TestReceiver_HealthReportsNormalizedQQSourceAndCapabilities(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	r := NewReceiver(&capabilityAwareTextPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "qq-live"},
		metadata: core.PlatformMetadata{
			Source: "qq",
			Capabilities: core.PlatformCapabilities{
				CommandSurface:        core.CommandSurfaceMixed,
				MessageScopes:         []core.MessageScope{core.MessageScopeChat},
				SupportsMentions:      true,
				SupportsSlashCommands: true,
			},
		},
	}, strconv.Itoa(port))
	done := make(chan error, 1)
	go func() {
		done <- r.Start()
	}()

	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/im/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["platform"] != "qq-live" {
		t.Fatalf("platform = %v", payload["platform"])
	}
	if payload["source"] != "qq" {
		t.Fatalf("source = %v", payload["source"])
	}
	if payload["readiness_tier"] != string(core.ReadinessTierTextFirst) {
		t.Fatalf("readiness_tier = %v", payload["readiness_tier"])
	}
	matrix, ok := payload["capability_matrix"].(map[string]any)
	if !ok {
		t.Fatalf("capability_matrix = %#v", payload["capability_matrix"])
	}
	if matrix["readinessTier"] != string(core.ReadinessTierTextFirst) {
		t.Fatalf("capability_matrix = %#v", matrix)
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Start returned error after stop: %v", err)
	}
}

func TestReceiver_HealthReportsNormalizedQQBotSourceAndCapabilities(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	r := NewReceiver(&capabilityAwareTextPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "qqbot-live"},
		metadata: core.PlatformMetadata{
			Source: "qqbot",
			Capabilities: core.PlatformCapabilities{
				CommandSurface:         core.CommandSurfaceMixed,
				ActionCallbackMode:     core.ActionCallbackWebhook,
				MessageScopes:          []core.MessageScope{core.MessageScopeChat},
				RequiresPublicCallback: true,
				SupportsMentions:       true,
				SupportsSlashCommands:  true,
			},
		},
	}, strconv.Itoa(port))
	done := make(chan error, 1)
	go func() {
		done <- r.Start()
	}()

	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/im/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["platform"] != "qqbot-live" {
		t.Fatalf("platform = %v", payload["platform"])
	}
	if payload["source"] != "qqbot" {
		t.Fatalf("source = %v", payload["source"])
	}
	if payload["readiness_tier"] != string(core.ReadinessTierMarkdownFirst) {
		t.Fatalf("readiness_tier = %v", payload["readiness_tier"])
	}
	if payload["supports_rich_messages"] != false {
		t.Fatalf("supports_rich_messages = %v", payload["supports_rich_messages"])
	}
	matrix, ok := payload["capability_matrix"].(map[string]any)
	if !ok {
		t.Fatalf("capability_matrix = %#v", payload["capability_matrix"])
	}
	if matrix["readinessTier"] != string(core.ReadinessTierMarkdownFirst) {
		t.Fatalf("capability_matrix = %#v", matrix)
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Start returned error after stop: %v", err)
	}
}

func TestReceiver_RejectsUnsignedCompatibilityDeliveryWhenSecretConfigured(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")
	r.SetSharedSecret("shared-secret")

	body, err := json.Marshal(SendRequest{
		Platform: "slack",
		ChatID:   "chat-1",
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("marshal send request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleSend(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

func TestReceiver_SuppressesDuplicateSignedCompatibilityDelivery(t *testing.T) {
	p := &textOnlyPlatform{name: "slack-stub"}
	r := NewReceiver(p, "0")
	r.SetSharedSecret("shared-secret")

	body, err := json.Marshal(SendRequest{
		Platform: "slack",
		ChatID:   "chat-1",
		Content:  "hello",
	})
	if err != nil {
		t.Fatalf("marshal send request: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeaders(req1, "/im/send", "delivery-1", body, "shared-secret")
	rec1 := httptest.NewRecorder()
	r.handleSend(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first status = %d, want %d", rec1.Code, http.StatusOK)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/im/send", bytes.NewReader(body))
	applySignedHeaders(req2, "/im/send", "delivery-1", body, "shared-secret")
	rec2 := httptest.NewRecorder()
	r.handleSend(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("second status = %d, want %d", rec2.Code, http.StatusOK)
	}
	if len(p.sent) != 1 {
		t.Fatalf("sent len = %d, want 1", len(p.sent))
	}
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

func TestReceiver_HandleSend_WritesFallbackReasonHeader(t *testing.T) {
	p := &textOnlyPlatform{name: "telegram-stub"}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(SendRequest{
		Platform: "telegram",
		ChatID:   "chat-1",
		Structured: &core.StructuredMessage{
			Title: "Task Update",
			Body:  "Agent is still working.",
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
	if rec.Header().Get("X-IM-Downgrade-Reason") != "structured_delivery_unavailable" {
		t.Fatalf("header downgrade reason = %q", rec.Header().Get("X-IM-Downgrade-Reason"))
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

func TestReceiver_FallsBackToStructuredTextWhenNativeStructuredSenderUnavailable(t *testing.T) {
	p := &textOnlyPlatform{name: "telegram-stub"}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "telegram",
		TargetChatID: "chat-1",
		Structured: &core.StructuredMessage{
			Title: "Task Update",
			Body:  "Agent is still working.",
			Fields: []core.StructuredField{
				{Label: "Status", Value: "running"},
			},
		},
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.sent) != 1 {
		t.Fatalf("sent = %v", p.sent)
	}
	if p.sent[0] != "Task Update\nAgent is still working.\nStatus: running" {
		t.Fatalf("fallback text = %q", p.sent[0])
	}
}

func TestReceiver_ActionResponseUsesReplyTargetDelivery(t *testing.T) {
	p := &replyAwareTextPlatform{textOnlyPlatform: textOnlyPlatform{name: "slack-stub"}}
	r := NewReceiver(p, "0")
	r.SetActionHandler(&replyTargetActionHandler{
		response: &ActionResponse{
			Result: "Approved",
		},
	})

	body, err := json.Marshal(ActionRequest{
		Platform: "slack",
		Action:   "approve",
		EntityID: "review-1",
		ChatID:   "C123",
		ReplyTarget: &core.ReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
			UseReply:  true,
		},
	})
	if err != nil {
		t.Fatalf("marshal action request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/action", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.sent) != 0 {
		t.Fatalf("expected reply strategy to avoid plain send fallback, got sent=%v", p.sent)
	}
	if len(p.replies) != 1 || p.replies[0] != "Approved" {
		t.Fatalf("replies = %v", p.replies)
	}
}

func TestReceiver_ActionResponsePrefersFeishuDeferredNativeUpdate(t *testing.T) {
	p := &feishuActionNativePlatform{
		replyAwareTextPlatform: replyAwareTextPlatform{textOnlyPlatform: textOnlyPlatform{name: "feishu-live"}},
		metadata: core.PlatformMetadata{
			Source: "feishu",
			Capabilities: core.PlatformCapabilities{
				StructuredSurface:    core.StructuredSurfaceCards,
				AsyncUpdateModes:     []core.AsyncUpdateMode{core.AsyncUpdateDeferredCardUpdate},
				SupportsRichMessages: true,
			},
		},
	}
	r := NewReceiverWithMetadata(p, p.metadata, "0")
	r.SetActionHandler(&replyTargetActionHandler{
		response: &ActionResponse{
			Result: "Native update completed",
		},
	})

	body, err := json.Marshal(ActionRequest{
		Platform: "feishu",
		Action:   "approve",
		EntityID: "review-1",
		ChatID:   "oc_456",
		ReplyTarget: &core.ReplyTarget{
			Platform:      "feishu",
			ChatID:        "oc_456",
			MessageID:     "om_123",
			CallbackToken: "card-token-1",
			ProgressMode:  string(core.AsyncUpdateDeferredCardUpdate),
			UseReply:      true,
		},
	})
	if err != nil {
		t.Fatalf("marshal action request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/action", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.nativeUpdate) != 1 {
		t.Fatalf("nativeUpdate = %d, want 1", len(p.nativeUpdate))
	}
	if len(p.replies) != 0 || len(p.sent) != 0 {
		t.Fatalf("expected native update only, replies=%v sent=%v", p.replies, p.sent)
	}
	if p.nativeUpdate[0] == nil || p.nativeUpdate[0].FeishuCard == nil {
		t.Fatalf("native update payload = %#v", p.nativeUpdate[0])
	}
	if p.nativeUpdate[0].FeishuCard.Mode != core.FeishuCardModeJSON {
		t.Fatalf("native update mode = %q", p.nativeUpdate[0].FeishuCard.Mode)
	}
}

func TestReceiver_ActionResponseRecordsFallbackReasonWhenFeishuDelayedUpdateContextMissing(t *testing.T) {
	p := &feishuActionNativePlatform{
		replyAwareTextPlatform: replyAwareTextPlatform{textOnlyPlatform: textOnlyPlatform{name: "feishu-live"}},
		metadata: core.PlatformMetadata{
			Source: "feishu",
			Capabilities: core.PlatformCapabilities{
				StructuredSurface:    core.StructuredSurfaceCards,
				AsyncUpdateModes:     []core.AsyncUpdateMode{core.AsyncUpdateDeferredCardUpdate},
				SupportsRichMessages: true,
			},
		},
	}
	r := NewReceiverWithMetadata(p, p.metadata, "0")
	r.SetActionHandler(&replyTargetActionHandler{
		response: &ActionResponse{
			Result: "Fallback to text reply",
		},
	})

	body, err := json.Marshal(ActionRequest{
		Platform: "feishu",
		Action:   "approve",
		EntityID: "review-1",
		ChatID:   "oc_456",
		ReplyTarget: &core.ReplyTarget{
			Platform:     "feishu",
			ChatID:       "oc_456",
			MessageID:    "om_123",
			ProgressMode: string(core.AsyncUpdateDeferredCardUpdate),
			UseReply:     true,
		},
	})
	if err != nil {
		t.Fatalf("marshal action request: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/action", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleAction(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.nativeUpdate) != 0 {
		t.Fatalf("nativeUpdate = %d, want 0", len(p.nativeUpdate))
	}
	if len(p.replies) != 1 || p.replies[0] != "Fallback to text reply" {
		t.Fatalf("replies = %v", p.replies)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %#v", payload["metadata"])
	}
	if metadata["fallback_reason"] != "missing_delayed_update_context" {
		t.Fatalf("metadata = %#v", metadata)
	}
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

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
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

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
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

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
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

func applySignedHeaders(req *http.Request, path string, deliveryID string, body []byte, secret string) {
	timestamp := "2026-03-25T00:00:00Z"
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.Join([]string{
		req.Method,
		path,
		deliveryID,
		timestamp,
		string(body),
	}, "|")))
	req.Header.Set("X-AgentForge-Delivery-Id", deliveryID)
	req.Header.Set("X-AgentForge-Delivery-Timestamp", timestamp)
	req.Header.Set("X-AgentForge-Signature", hex.EncodeToString(mac.Sum(nil)))
}
