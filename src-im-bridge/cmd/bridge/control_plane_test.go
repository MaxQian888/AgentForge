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
		if ack.Status != "delivered" {
			t.Fatalf("ack status = %q, want delivered", ack.Status)
		}
		if ack.ProcessedAt == "" {
			t.Fatalf("ack processedAt = %q, want non-empty timestamp", ack.ProcessedAt)
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

func TestBridgeRuntimeControl_ApplyDeliveryPrefersTypedStructuredPayload(t *testing.T) {
	platform := &structuredRuntimeTestPlatform{}
	control := &bridgeRuntimeControl{
		cfg: &config{},
		provider: &activeProvider{
			Platform: platform,
			Descriptor: providerDescriptor{
				ID:       "discord",
				Metadata: platform.Metadata(),
			},
			TransportMode: "stub",
		},
	}

	downgradeReason, err := control.applyDelivery(context.Background(), &client.ControlDelivery{
		Platform:     "discord",
		TargetChatID: "channel-1",
		Content:      "plain fallback",
		Structured: &core.StructuredMessage{
			Title: "Task Update",
			Body:  "Agent is still running",
		},
		Metadata: map[string]string{
			"fallback_reason": "structured_reply_unavailable",
		},
		ReplyTarget: &core.ReplyTarget{
			Platform:         "discord",
			ChannelID:        "channel-1",
			InteractionToken: "token-1",
			UseReply:         true,
		},
	})
	if err != nil {
		t.Fatalf("applyDelivery error: %v", err)
	}
	if downgradeReason != "structured_reply_unavailable" {
		t.Fatalf("downgradeReason = %q, want structured_reply_unavailable", downgradeReason)
	}

	if len(platform.structured) != 0 {
		t.Fatalf("structured deliveries = %+v, want reply-aware fallback instead", platform.structured)
	}
	if len(platform.replies) != 1 || platform.replies[0] != "Task Update\nAgent is still running" {
		t.Fatalf("replies = %+v", platform.replies)
	}
}

func TestBridgeRuntimeControl_ApplyDeliveryUsesDeferredNativeUpdateForFeishuProgress(t *testing.T) {
	platform := &feishuDeferredRuntimeTestPlatform{}
	control := &bridgeRuntimeControl{
		cfg: &config{},
		provider: &activeProvider{
			Platform: platform,
			Descriptor: providerDescriptor{
				ID:       "feishu",
				Metadata: platform.Metadata(),
			},
			TransportMode: "stub",
		},
	}

	downgradeReason, err := control.applyDelivery(context.Background(), &client.ControlDelivery{
		Platform:     "feishu",
		TargetChatID: "oc_123",
		Content:      "Agent 已完成最新阶段",
		ReplyTarget: &core.ReplyTarget{
			Platform:      "feishu",
			ChatID:        "oc_123",
			MessageID:     "om_123",
			CallbackToken: "token-1",
			ProgressMode:  string(core.AsyncUpdateDeferredCardUpdate),
			UseReply:      true,
		},
	})
	if err != nil {
		t.Fatalf("applyDelivery error: %v", err)
	}
	if downgradeReason != "" {
		t.Fatalf("downgradeReason = %q, want empty", downgradeReason)
	}

	if len(platform.nativeUpdates) != 1 {
		t.Fatalf("nativeUpdates = %d, want 1", len(platform.nativeUpdates))
	}
	if len(platform.replies) != 0 {
		t.Fatalf("replies = %+v, want no text fallback", platform.replies)
	}
}

func TestBridgeRuntimeControl_StartIncludesWeComCallbackPathInRegistration(t *testing.T) {
	upgrader := websocket.Upgrader{}
	type counts struct {
		mu         sync.Mutex
		lastReg    client.BridgeRegistration
		registered bool
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
			seen.lastReg = req
			seen.registered = true
			seen.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(client.BridgeInstance{BridgeID: req.BridgeID, Status: "online"})
		case "/api/v1/im/bridge/heartbeat":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(client.BridgeHeartbeat{BridgeID: "bridge-wecom-1", Status: "online"})
		case "/api/v1/im/bridge/unregister":
			w.WriteHeader(http.StatusOK)
		case "/ws/im-bridge":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("Upgrade error: %v", err)
			}
			defer conn.Close()
			delivery := client.ControlDelivery{
				Cursor:         1,
				DeliveryID:     "delivery-1",
				TargetBridgeID: "bridge-wecom-1",
				Platform:       "wecom",
				Kind:           "task.progress",
				Content:        "hello",
				TargetChatID:   "chat-1",
				Timestamp:      "2026-03-26T00:00:00Z",
			}
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

	platform := &wecomRuntimeTestPlatform{}
	apiClient := client.NewAgentForgeClient(server.URL, "proj-1", "secret").WithPlatform(platform)
	provider := &activeProvider{
		Descriptor: providerDescriptor{
			ID:       "wecom",
			Metadata: platform.Metadata(),
		},
		Platform:      platform,
		TransportMode: "live",
	}
	control := newBridgeRuntimeControl(&config{
		APIBase:               server.URL,
		ProjectID:             "proj-1",
		TransportMode:         "live",
		HeartbeatInterval:     10 * time.Millisecond,
		ControlReconnectDelay: 10 * time.Millisecond,
	}, "bridge-wecom-1", provider, apiClient)

	if err := control.Start(context.Background()); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	select {
	case <-ackCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ack")
	}
	if err := control.Stop(context.Background()); err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	seen.mu.Lock()
	defer seen.mu.Unlock()
	if !seen.registered {
		t.Fatal("expected registration to occur")
	}
	if len(seen.lastReg.CallbackPaths) != 3 || seen.lastReg.CallbackPaths[2] != "/wecom/callback" {
		t.Fatalf("callback paths = %+v", seen.lastReg.CallbackPaths)
	}
	if seen.lastReg.Capabilities["requires_public_callback"] != true {
		t.Fatalf("capabilities = %+v", seen.lastReg.Capabilities)
	}
	if seen.lastReg.Metadata["readiness_tier"] != string(core.ReadinessTierFullNativeLifecycle) {
		t.Fatalf("register metadata = %+v", seen.lastReg.Metadata)
	}
	if seen.lastReg.Metadata["preferred_async_update_mode"] != string(core.AsyncUpdateSessionWebhook) {
		t.Fatalf("register metadata = %+v", seen.lastReg.Metadata)
	}
	if seen.lastReg.Metadata["fallback_async_update_mode"] != string(core.AsyncUpdateReply) {
		t.Fatalf("register metadata = %+v", seen.lastReg.Metadata)
	}
	if seen.lastReg.CapabilityMatrix["preferredAsyncUpdateMode"] != string(core.AsyncUpdateSessionWebhook) {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
	if seen.lastReg.CapabilityMatrix["fallbackAsyncUpdateMode"] != string(core.AsyncUpdateReply) {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
}

func TestBridgeRuntimeControl_ApplyDeliveryUsesWeComResponseURLReplyTarget(t *testing.T) {
	platform := &wecomRuntimeTestPlatform{}
	control := &bridgeRuntimeControl{
		cfg: &config{},
		provider: &activeProvider{
			Platform: platform,
			Descriptor: providerDescriptor{
				ID:       "wecom",
				Metadata: platform.Metadata(),
			},
			TransportMode: "live",
		},
	}

	downgradeReason, err := control.applyDelivery(context.Background(), &client.ControlDelivery{
		Platform:     "wecom",
		TargetChatID: "chat-1",
		Content:      "queued update",
		ReplyTarget: &core.ReplyTarget{
			Platform:       "wecom",
			ChatID:         "chat-1",
			SessionWebhook: "https://work.weixin.qq.com/response",
			UserID:         "zhangsan",
			UseReply:       true,
		},
	})
	if err != nil {
		t.Fatalf("applyDelivery error: %v", err)
	}
	if downgradeReason != "" {
		t.Fatalf("downgradeReason = %q, want empty", downgradeReason)
	}
	if len(platform.replies) != 1 || platform.replies[0] != "queued update" {
		t.Fatalf("replies = %+v", platform.replies)
	}
	if len(platform.replyTargets) != 1 || platform.replyTargets[0] == nil || platform.replyTargets[0].SessionWebhook != "https://work.weixin.qq.com/response" {
		t.Fatalf("reply targets = %+v", platform.replyTargets)
	}
}

func TestBridgeRuntimeControl_StartIncludesQQBotCallbackPathInRegistration(t *testing.T) {
	upgrader := websocket.Upgrader{}
	type counts struct {
		mu         sync.Mutex
		lastReg    client.BridgeRegistration
		registered bool
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
			seen.lastReg = req
			seen.registered = true
			seen.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(client.BridgeInstance{BridgeID: req.BridgeID, Status: "online"})
		case "/api/v1/im/bridge/heartbeat":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(client.BridgeHeartbeat{BridgeID: "bridge-qqbot-1", Status: "online"})
		case "/api/v1/im/bridge/unregister":
			w.WriteHeader(http.StatusOK)
		case "/ws/im-bridge":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("Upgrade error: %v", err)
			}
			defer conn.Close()
			delivery := client.ControlDelivery{
				Cursor:         1,
				DeliveryID:     "delivery-1",
				TargetBridgeID: "bridge-qqbot-1",
				Platform:       "qqbot",
				Kind:           "task.progress",
				Content:        "hello",
				TargetChatID:   "group-openid",
				Timestamp:      "2026-03-28T00:00:00Z",
			}
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

	platform := &qqbotRuntimeTestPlatform{}
	apiClient := client.NewAgentForgeClient(server.URL, "proj-1", "secret").WithPlatform(platform)
	provider := &activeProvider{
		Descriptor: providerDescriptor{
			ID:       "qqbot",
			Metadata: platform.Metadata(),
		},
		Platform:      platform,
		TransportMode: "live",
	}
	control := newBridgeRuntimeControl(&config{
		APIBase:               server.URL,
		ProjectID:             "proj-1",
		TransportMode:         "live",
		HeartbeatInterval:     10 * time.Millisecond,
		ControlReconnectDelay: 10 * time.Millisecond,
	}, "bridge-qqbot-1", provider, apiClient)

	if err := control.Start(context.Background()); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	select {
	case <-ackCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ack")
	}
	if err := control.Stop(context.Background()); err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	seen.mu.Lock()
	defer seen.mu.Unlock()
	if !seen.registered {
		t.Fatal("expected registration to occur")
	}
	if len(seen.lastReg.CallbackPaths) != 3 || seen.lastReg.CallbackPaths[2] != "/qqbot/callback" {
		t.Fatalf("callback paths = %+v", seen.lastReg.CallbackPaths)
	}
	if seen.lastReg.Capabilities["requires_public_callback"] != true {
		t.Fatalf("capabilities = %+v", seen.lastReg.Capabilities)
	}
	if seen.lastReg.CapabilityMatrix["commandSurface"] != "mixed" {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
	if seen.lastReg.Metadata["readiness_tier"] != string(core.ReadinessTierNativeSendWithFallback) {
		t.Fatalf("register metadata = %+v", seen.lastReg.Metadata)
	}
	if seen.lastReg.Metadata["preferred_async_update_mode"] != string(core.AsyncUpdateReply) {
		t.Fatalf("register metadata = %+v", seen.lastReg.Metadata)
	}
	if seen.lastReg.CapabilityMatrix["readinessTier"] != string(core.ReadinessTierNativeSendWithFallback) {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
	if seen.lastReg.CapabilityMatrix["preferredAsyncUpdateMode"] != string(core.AsyncUpdateReply) {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
}

func TestBridgeRuntimeControl_StartIncludesQQCapabilityMatrixInRegistration(t *testing.T) {
	upgrader := websocket.Upgrader{}
	type counts struct {
		mu         sync.Mutex
		lastReg    client.BridgeRegistration
		registered bool
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
			seen.lastReg = req
			seen.registered = true
			seen.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(client.BridgeInstance{BridgeID: req.BridgeID, Status: "online"})
		case "/api/v1/im/bridge/heartbeat":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(client.BridgeHeartbeat{BridgeID: "bridge-qq-1", Status: "online"})
		case "/api/v1/im/bridge/unregister":
			w.WriteHeader(http.StatusOK)
		case "/ws/im-bridge":
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Fatalf("Upgrade error: %v", err)
			}
			defer conn.Close()
			delivery := client.ControlDelivery{
				Cursor:         1,
				DeliveryID:     "delivery-1",
				TargetBridgeID: "bridge-qq-1",
				Platform:       "qq",
				Kind:           "task.progress",
				Content:        "hello",
				TargetChatID:   "chat-1",
				Timestamp:      "2026-03-28T00:00:00Z",
			}
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
	apiClient := client.NewAgentForgeClient(server.URL, "proj-1", "secret").WithSource("qq")
	provider := &activeProvider{
		Descriptor: providerDescriptor{
			ID:       "qq",
			Metadata: core.NormalizeMetadata(core.PlatformMetadata{Source: "qq"}, "qq"),
		},
		Platform:      platform,
		TransportMode: "live",
	}
	control := newBridgeRuntimeControl(&config{
		APIBase:               server.URL,
		ProjectID:             "proj-1",
		TransportMode:         "live",
		HeartbeatInterval:     10 * time.Millisecond,
		ControlReconnectDelay: 10 * time.Millisecond,
	}, "bridge-qq-1", provider, apiClient)

	if err := control.Start(context.Background()); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	select {
	case <-ackCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for ack")
	}
	if err := control.Stop(context.Background()); err != nil {
		t.Fatalf("Stop error: %v", err)
	}

	seen.mu.Lock()
	defer seen.mu.Unlock()
	if !seen.registered {
		t.Fatal("expected registration to occur")
	}
	if seen.lastReg.CapabilityMatrix["commandSurface"] != "mixed" {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
	if seen.lastReg.CapabilityMatrix["actionCallbackMode"] != "none" {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
	if seen.lastReg.Metadata["readiness_tier"] != string(core.ReadinessTierTextFirst) {
		t.Fatalf("register metadata = %+v", seen.lastReg.Metadata)
	}
	if seen.lastReg.Metadata["preferred_async_update_mode"] != string(core.AsyncUpdateReply) {
		t.Fatalf("register metadata = %+v", seen.lastReg.Metadata)
	}
	if seen.lastReg.CapabilityMatrix["readinessTier"] != string(core.ReadinessTierTextFirst) {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
	if seen.lastReg.CapabilityMatrix["preferredAsyncUpdateMode"] != string(core.AsyncUpdateReply) {
		t.Fatalf("capability matrix = %+v", seen.lastReg.CapabilityMatrix)
	}
	if scopes, ok := seen.lastReg.CapabilityMatrix["messageScopes"].([]interface{}); !ok || len(scopes) == 0 || scopes[0] != "chat" {
		t.Fatalf("capability matrix scopes = %+v", seen.lastReg.CapabilityMatrix["messageScopes"])
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
	payload, err := json.Marshal(struct {
		TargetBridgeID string                  `json:"targetBridgeId"`
		Cursor         int64                   `json:"cursor"`
		DeliveryID     string                  `json:"deliveryId"`
		Platform       string                  `json:"platform"`
		ProjectID      string                  `json:"projectId,omitempty"`
		Kind           string                  `json:"kind"`
		Content        string                  `json:"content,omitempty"`
		Structured     *core.StructuredMessage `json:"structured,omitempty"`
		Native         *core.NativeMessage     `json:"native,omitempty"`
		Metadata       map[string]string       `json:"metadata,omitempty"`
		TargetChatID   string                  `json:"targetChatId,omitempty"`
		ReplyTarget    *core.ReplyTarget       `json:"replyTarget,omitempty"`
		Timestamp      string                  `json:"timestamp"`
	}{
		TargetBridgeID: strings.TrimSpace(delivery.TargetBridgeID),
		Cursor:         delivery.Cursor,
		DeliveryID:     strings.TrimSpace(delivery.DeliveryID),
		Platform:       core.NormalizePlatformName(delivery.Platform),
		ProjectID:      strings.TrimSpace(delivery.ProjectID),
		Kind:           strings.ToLower(strings.TrimSpace(delivery.Kind)),
		Content:        strings.TrimSpace(delivery.Content),
		Structured:     delivery.Structured,
		Native:         delivery.Native,
		Metadata:       delivery.Metadata,
		TargetChatID:   strings.TrimSpace(delivery.TargetChatID),
		ReplyTarget:    delivery.ReplyTarget,
		Timestamp:      strings.TrimSpace(delivery.Timestamp),
	})
	if err != nil {
		return ""
	}
	mac := hmac.New(sha256.New, []byte(strings.TrimSpace(secret)))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

type structuredRuntimeTestPlatform struct {
	runtimeTestPlatform
	structured []*core.StructuredMessage
}

func (p *structuredRuntimeTestPlatform) Metadata() core.PlatformMetadata {
	return core.PlatformMetadata{
		Source: "discord",
		Capabilities: core.PlatformCapabilities{
			StructuredSurface:  core.StructuredSurfaceComponents,
			ActionCallbackMode: core.ActionCallbackWebhook,
			AsyncUpdateModes:   []core.AsyncUpdateMode{core.AsyncUpdateFollowUp},
			MessageScopes:      []core.MessageScope{core.MessageScopeChat},
		},
	}
}

func (p *structuredRuntimeTestPlatform) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	p.structured = append(p.structured, message)
	return nil
}

type feishuDeferredRuntimeTestPlatform struct {
	runtimeTestPlatform
	nativeUpdates []*core.NativeMessage
}

func (p *feishuDeferredRuntimeTestPlatform) Metadata() core.PlatformMetadata {
	return core.PlatformMetadata{
		Source: "feishu",
		Capabilities: core.PlatformCapabilities{
			StructuredSurface:    core.StructuredSurfaceCards,
			AsyncUpdateModes:     []core.AsyncUpdateMode{core.AsyncUpdateDeferredCardUpdate},
			SupportsRichMessages: true,
		},
	}
}

func (p *feishuDeferredRuntimeTestPlatform) UpdateNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	p.nativeUpdates = append(p.nativeUpdates, message)
	return nil
}

func (p *feishuDeferredRuntimeTestPlatform) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	p.nativeUpdates = append(p.nativeUpdates, message)
	return nil
}

func (p *feishuDeferredRuntimeTestPlatform) ReplyNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	p.nativeUpdates = append(p.nativeUpdates, message)
	return nil
}

type wecomRuntimeTestPlatform struct {
	runtimeTestPlatform
}

func (p *wecomRuntimeTestPlatform) Name() string { return "wecom-live" }

func (p *wecomRuntimeTestPlatform) Metadata() core.PlatformMetadata {
	return core.PlatformMetadata{
		Source: "wecom",
		Capabilities: core.PlatformCapabilities{
			StructuredSurface:      core.StructuredSurfaceCards,
			ActionCallbackMode:     core.ActionCallbackWebhook,
			AsyncUpdateModes:       []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateSessionWebhook},
			MessageScopes:          []core.MessageScope{core.MessageScopeChat},
			RequiresPublicCallback: true,
			SupportsRichMessages:   true,
			SupportsMentions:       true,
			SupportsSlashCommands:  true,
		},
	}
}

func (p *wecomRuntimeTestPlatform) CallbackPaths() []string {
	return []string{"/wecom/callback"}
}

type qqbotRuntimeTestPlatform struct {
	runtimeTestPlatform
}

func (p *qqbotRuntimeTestPlatform) Name() string { return "qqbot-live" }

func (p *qqbotRuntimeTestPlatform) Metadata() core.PlatformMetadata {
	return core.PlatformMetadata{
		Source: "qqbot",
		Capabilities: core.PlatformCapabilities{
			ActionCallbackMode:     core.ActionCallbackWebhook,
			AsyncUpdateModes:       []core.AsyncUpdateMode{core.AsyncUpdateReply},
			MessageScopes:          []core.MessageScope{core.MessageScopeChat},
			RequiresPublicCallback: true,
			SupportsMentions:       true,
			SupportsSlashCommands:  true,
		},
	}
}

func (p *qqbotRuntimeTestPlatform) CallbackPaths() []string {
	return []string{"/qqbot/callback"}
}
