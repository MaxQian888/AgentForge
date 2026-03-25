package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	"github.com/gorilla/websocket"
)

func TestLoadOrCreateBridgeID_PersistsGeneratedValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime", "bridge.id")

	bridgeID, err := loadOrCreateBridgeID(path)
	if err != nil {
		t.Fatalf("loadOrCreateBridgeID error: %v", err)
	}
	if !strings.HasPrefix(bridgeID, "bridge-") {
		t.Fatalf("bridgeID = %q", bridgeID)
	}

	reloaded, err := loadOrCreateBridgeID(path)
	if err != nil {
		t.Fatalf("reload bridge id error: %v", err)
	}
	if reloaded != bridgeID {
		t.Fatalf("reloaded bridgeID = %q, want %q", reloaded, bridgeID)
	}

	if _, err := loadOrCreateBridgeID(""); err == nil {
		t.Fatal("expected empty bridge id path to fail")
	}
}

func TestFirstNonEmpty_ReturnsFirstTrimmedValue(t *testing.T) {
	if got := firstNonEmpty("   ", "", "  bridge-1  ", "bridge-2"); got != "bridge-1" {
		t.Fatalf("firstNonEmpty = %q", got)
	}
}

func TestBridgeRuntimeControl_VerifyDeliveryRequiresValidSignature(t *testing.T) {
	control := &bridgeRuntimeControl{
		cfg: &config{ControlSharedSecret: "shared-secret"},
	}
	delivery := client.ControlDelivery{
		Cursor:         3,
		DeliveryID:     "delivery-1",
		TargetBridgeID: "bridge-1",
		Kind:           "task.progress",
		Content:        "hello",
		Timestamp:      "2026-03-25T00:00:00Z",
	}

	if control.verifyDelivery(&delivery) {
		t.Fatal("expected unsigned delivery to be rejected")
	}

	delivery.Signature = signDelivery("shared-secret", delivery)
	if !control.verifyDelivery(&delivery) {
		t.Fatal("expected signed delivery to be accepted")
	}

	control.cfg.ControlSharedSecret = ""
	if !control.verifyDelivery(&delivery) {
		t.Fatal("expected verification to be bypassed when no shared secret is configured")
	}
}

func TestBridgeRuntimeControl_StartProcessesDeliveriesAndStopsCleanly(t *testing.T) {
	upgrader := websocket.Upgrader{}
	type counts struct {
		mu         sync.Mutex
		register   int
		heartbeat  int
		unregister int
		lastReg    client.BridgeRegistration
	}
	var seen counts
	ackCh := make(chan client.ControlDeliveryAck, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/im/bridge/register":
			var req client.BridgeRegistration
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Fatalf("decode register: %v", err)
			}
			seen.mu.Lock()
			seen.register++
			seen.lastReg = req
			seen.mu.Unlock()

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(client.BridgeInstance{BridgeID: req.BridgeID, Status: "online"})
		case "/api/v1/im/bridge/heartbeat":
			seen.mu.Lock()
			seen.heartbeat++
			seen.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(client.BridgeHeartbeat{BridgeID: "bridge-1", Status: "online"})
		case "/api/v1/im/bridge/unregister":
			seen.mu.Lock()
			seen.unregister++
			seen.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case "/ws/im-bridge":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("Upgrade error: %v", err)
			}
			defer conn.Close()

			delivery := client.ControlDelivery{
				Cursor:         7,
				DeliveryID:     "delivery-1",
				TargetBridgeID: "bridge-1",
				Platform:       "slack",
				Kind:           "task.progress",
				Content:        "hello from control plane",
				TargetChatID:   "chat-1",
				ReplyTarget: &core.ReplyTarget{
					Platform: "slack",
					ChatID:   "chat-1",
					UseReply: true,
				},
				Timestamp: "2026-03-25T00:00:00Z",
			}
			delivery.Signature = signDelivery("shared-secret", delivery)
			if err := conn.WriteJSON(delivery); err != nil {
				t.Fatalf("WriteJSON error: %v", err)
			}

			var ack client.ControlDeliveryAck
			if err := conn.ReadJSON(&ack); err == nil {
				ackCh <- ack
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	platform := &runtimeTestPlatform{}
	apiClient := client.NewAgentForgeClient(server.URL, "proj-1", "secret").WithPlatform(platform)
	provider := &activeProvider{
		Descriptor: providerDescriptor{
			ID:       "slack",
			Metadata: platform.Metadata(),
		},
		Platform:      platform,
		TransportMode: "stub",
	}
	control := newBridgeRuntimeControl(&config{
		APIBase:               server.URL,
		ProjectID:             "proj-1",
		TransportMode:         "stub",
		ControlSharedSecret:   "shared-secret",
		HeartbeatInterval:     10 * time.Millisecond,
		ControlReconnectDelay: 10 * time.Millisecond,
	}, "bridge-1", provider, apiClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := control.Start(ctx); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	select {
	case ack := <-ackCh:
		if ack.BridgeID != "bridge-1" || ack.Cursor != 7 || ack.DeliveryID != "delivery-1" {
			t.Fatalf("ack = %+v", ack)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for delivery ack")
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		seen.mu.Lock()
		heartbeatCount := seen.heartbeat
		seen.mu.Unlock()
		if heartbeatCount > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := control.Stop(context.Background()); err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	seen.mu.Lock()
	defer seen.mu.Unlock()
	if seen.register != 1 {
		t.Fatalf("register count = %d", seen.register)
	}
	if seen.unregister != 1 {
		t.Fatalf("unregister count = %d", seen.unregister)
	}
	if seen.heartbeat == 0 {
		t.Fatal("expected at least one heartbeat")
	}
	if seen.lastReg.BridgeID != "bridge-1" || seen.lastReg.Platform != "slack" || seen.lastReg.Transport != "stub" {
		t.Fatalf("register payload = %+v", seen.lastReg)
	}
	if seen.lastReg.Metadata["provider_id"] != "slack" {
		t.Fatalf("register metadata = %+v", seen.lastReg.Metadata)
	}
	if len(seen.lastReg.CallbackPaths) != 2 || seen.lastReg.CallbackPaths[0] != "/im/notify" {
		t.Fatalf("callback paths = %+v", seen.lastReg.CallbackPaths)
	}

	if len(platform.replies) != 1 || platform.replies[0] != "hello from control plane" {
		t.Fatalf("replies = %+v", platform.replies)
	}
	if len(platform.replyTargets) != 1 || platform.replyTargets[0] == nil || platform.replyTargets[0].ChatID != "chat-1" {
		t.Fatalf("reply targets = %+v", platform.replyTargets)
	}
	if control.cursor() != 7 {
		t.Fatalf("cursor = %d, want 7", control.cursor())
	}
}

type runtimeTestPlatform struct {
	mu           sync.Mutex
	replies      []string
	replyTargets []*core.ReplyTarget
}

func (p *runtimeTestPlatform) Name() string { return "slack-stub" }

func (p *runtimeTestPlatform) Metadata() core.PlatformMetadata {
	return core.PlatformMetadata{
		Source: "slack",
		Capabilities: core.PlatformCapabilities{
			CommandSurface:        core.CommandSurfaceMixed,
			ActionCallbackMode:    core.ActionCallbackWebhook,
			AsyncUpdateModes:      []core.AsyncUpdateMode{core.AsyncUpdateReply},
			MessageScopes:         []core.MessageScope{core.MessageScopeChat, core.MessageScopeThread},
			SupportsMentions:      true,
			SupportsSlashCommands: true,
		},
	}
}

func (p *runtimeTestPlatform) ReplyContextFromTarget(target *core.ReplyTarget) any {
	return target
}

func (p *runtimeTestPlatform) Start(handler core.MessageHandler) error { return nil }

func (p *runtimeTestPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.replies = append(p.replies, content)
	if target, ok := replyCtx.(*core.ReplyTarget); ok {
		p.replyTargets = append(p.replyTargets, target)
	} else {
		p.replyTargets = append(p.replyTargets, nil)
	}
	return nil
}

func (p *runtimeTestPlatform) Send(ctx context.Context, chatID string, content string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.replies = append(p.replies, content)
	p.replyTargets = append(p.replyTargets, &core.ReplyTarget{ChatID: chatID})
	return nil
}

func (p *runtimeTestPlatform) Stop() error { return nil }

func signDelivery(secret string, delivery client.ControlDelivery) string {
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write([]byte(strings.Join([]string{
		strings.TrimSpace(delivery.TargetBridgeID),
		int64String(delivery.Cursor),
		strings.TrimSpace(delivery.DeliveryID),
		strings.TrimSpace(delivery.Kind),
		strings.TrimSpace(delivery.Content),
		strings.TrimSpace(delivery.Timestamp),
	}, "|")))
	return hex.EncodeToString(mac.Sum(nil))
}

func int64String(value int64) string {
	return strconv.FormatInt(value, 10)
}
