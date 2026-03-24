package service

import (
	"context"
	"testing"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

type fakeBridgeDeliveryListener struct {
	deliveries []*IMControlDelivery
}

func (l *fakeBridgeDeliveryListener) Send(_ context.Context, delivery *IMControlDelivery) error {
	l.deliveries = append(l.deliveries, delivery)
	return nil
}

func (l *fakeBridgeDeliveryListener) Close() error { return nil }

func TestIMControlPlane_RegistrationHeartbeatAndExpiry(t *testing.T) {
	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              2 * time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
		Now: func() time.Time {
			return now
		},
	})

	instance, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:      "bridge-slack-1",
		Platform:      "slack",
		Transport:     "live",
		ProjectIDs:    []string{"project-1"},
		Capabilities:  map[string]bool{"supports_deferred_reply": true},
		CallbackPaths: []string{"/im/notify"},
	})
	if err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}
	if instance.BridgeID != "bridge-slack-1" {
		t.Fatalf("bridge id = %q", instance.BridgeID)
	}

	selected, err := control.ResolveBridgeTarget("slack", "project-1", "")
	if err != nil {
		t.Fatalf("ResolveBridgeTarget error: %v", err)
	}
	if selected.BridgeID != "bridge-slack-1" {
		t.Fatalf("selected bridge = %q", selected.BridgeID)
	}

	now = now.Add(90 * time.Second)
	if _, err := control.RecordHeartbeat(context.Background(), "bridge-slack-1"); err != nil {
		t.Fatalf("RecordHeartbeat error: %v", err)
	}

	now = now.Add(90 * time.Second)
	if _, err := control.ResolveBridgeTarget("slack", "project-1", ""); err != nil {
		t.Fatalf("ResolveBridgeTarget after heartbeat error: %v", err)
	}

	now = now.Add(3 * time.Minute)
	if _, err := control.ResolveBridgeTarget("slack", "project-1", ""); err == nil {
		t.Fatal("expected stale bridge to be rejected")
	}

	if err := control.UnregisterBridge(context.Background(), "bridge-slack-1"); err != nil {
		t.Fatalf("UnregisterBridge error: %v", err)
	}
}

func TestIMControlPlane_ReplayAndAckSuppressDuplicateDelivery(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-feishu-1",
		Platform:   "feishu",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	listener := &fakeBridgeDeliveryListener{}
	if _, err := control.AttachBridgeListener(context.Background(), "bridge-feishu-1", 0, listener); err != nil {
		t.Fatalf("AttachBridgeListener error: %v", err)
	}

	delivery, err := control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		TargetBridgeID: "bridge-feishu-1",
		Platform:       "feishu",
		ProjectID:      "project-1",
		Kind:           IMDeliveryKindProgress,
		Content:        "Agent 已启动，正在处理中",
		ReplyTarget: &model.IMReplyTarget{
			Platform: "feishu",
			ChatID:   "oc_123",
			MessageID:"om_123",
		},
	})
	if err != nil {
		t.Fatalf("QueueDelivery error: %v", err)
	}
	if len(listener.deliveries) != 1 {
		t.Fatalf("listener deliveries = %d, want 1", len(listener.deliveries))
	}
	if listener.deliveries[0].DeliveryID != delivery.DeliveryID {
		t.Fatalf("delivery id = %q, want %q", listener.deliveries[0].DeliveryID, delivery.DeliveryID)
	}

	if err := control.AckDelivery(context.Background(), "bridge-feishu-1", delivery.Cursor, delivery.DeliveryID); err != nil {
		t.Fatalf("AckDelivery error: %v", err)
	}

	replayListener := &fakeBridgeDeliveryListener{}
	replayed, err := control.AttachBridgeListener(context.Background(), "bridge-feishu-1", delivery.Cursor, replayListener)
	if err != nil {
		t.Fatalf("AttachBridgeListener replay error: %v", err)
	}
	if len(replayed) != 0 {
		t.Fatalf("replayed deliveries = %d, want 0", len(replayed))
	}
	if len(replayListener.deliveries) != 0 {
		t.Fatalf("listener replay deliveries = %d, want 0", len(replayListener.deliveries))
	}
}

func TestIMControlPlane_BindActionAndThrottleProgressHeartbeats(t *testing.T) {
	now := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
		Now: func() time.Time {
			return now
		},
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-discord-1",
		Platform:   "discord",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	listener := &fakeBridgeDeliveryListener{}
	if _, err := control.AttachBridgeListener(context.Background(), "bridge-discord-1", 0, listener); err != nil {
		t.Fatalf("AttachBridgeListener error: %v", err)
	}

	if err := control.BindAction(context.Background(), &IMActionBinding{
		BridgeID:  "bridge-discord-1",
		Platform:  "discord",
		ProjectID: "project-1",
		TaskID:    "task-1",
		RunID:     "run-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:         "discord",
			ChannelID:        "channel-1",
			InteractionToken: "token-1",
		},
	}); err != nil {
		t.Fatalf("BindAction error: %v", err)
	}

	sent, err := control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:    "run-1",
		TaskID:   "task-1",
		Kind:     IMDeliveryKindProgress,
		Content:  "Agent 仍在运行，继续处理中",
		IsTerminal: false,
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress error: %v", err)
	}
	if !sent {
		t.Fatal("expected first heartbeat delivery to be sent")
	}

	sent, err = control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:    "run-1",
		TaskID:   "task-1",
		Kind:     IMDeliveryKindProgress,
		Content:  "这条消息应该被节流",
		IsTerminal: false,
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress second error: %v", err)
	}
	if sent {
		t.Fatal("expected duplicate heartbeat within interval to be throttled")
	}

	now = now.Add(31 * time.Second)
	sent, err = control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:     "run-1",
		TaskID:    "task-1",
		Kind:      IMDeliveryKindTerminal,
		Content:   "Agent 已完成，任务进入总结阶段",
		IsTerminal:true,
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress terminal error: %v", err)
	}
	if !sent {
		t.Fatal("expected terminal delivery to bypass heartbeat throttle")
	}

	if len(listener.deliveries) != 2 {
		t.Fatalf("listener deliveries = %d, want 2", len(listener.deliveries))
	}
	if listener.deliveries[1].Kind != IMDeliveryKindTerminal {
		t.Fatalf("delivery kind = %q, want %q", listener.deliveries[1].Kind, IMDeliveryKindTerminal)
	}
}
