package service

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
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

	registrationMatrix := map[string]any{
		"commandSurface":    "mixed",
		"structuredSurface": "blocks",
		"asyncUpdateModes":  []string{"reply", "thread_reply", "follow_up"},
		"mutability": map[string]any{
			"canEdit":        false,
			"prefersInPlace": false,
		},
	}

	instance, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:         "bridge-slack-1",
		Platform:         "slack",
		Transport:        "live",
		ProjectIDs:       []string{"project-1"},
		Capabilities:     map[string]bool{"supports_deferred_reply": true},
		CapabilityMatrix: registrationMatrix,
		CallbackPaths:    []string{"/im/notify"},
	})
	if err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}
	if instance.BridgeID != "bridge-slack-1" {
		t.Fatalf("bridge id = %q", instance.BridgeID)
	}
	if instance.CapabilityMatrix["structuredSurface"] != "blocks" {
		t.Fatalf("CapabilityMatrix = %#v", instance.CapabilityMatrix)
	}
	registrationMatrix["structuredSurface"] = "mutated"
	registrationMatrix["mutability"].(map[string]any)["canEdit"] = true
	if instance.CapabilityMatrix["structuredSurface"] != "blocks" {
		t.Fatalf("stored CapabilityMatrix mutated with caller input: %#v", instance.CapabilityMatrix)
	}
	mutability, ok := instance.CapabilityMatrix["mutability"].(map[string]any)
	if !ok {
		t.Fatalf("mutability = %#v", instance.CapabilityMatrix["mutability"])
	}
	if mutability["canEdit"] != false {
		t.Fatalf("mutability.canEdit = %#v", mutability["canEdit"])
	}

	selected, err := control.ResolveBridgeTarget("slack", "project-1", "")
	if err != nil {
		t.Fatalf("ResolveBridgeTarget error: %v", err)
	}
	if selected.BridgeID != "bridge-slack-1" {
		t.Fatalf("selected bridge = %q", selected.BridgeID)
	}
	selected.CapabilityMatrix["structuredSurface"] = "components"
	again, err := control.ResolveBridgeTarget("slack", "project-1", "")
	if err != nil {
		t.Fatalf("ResolveBridgeTarget second error: %v", err)
	}
	if again.CapabilityMatrix["structuredSurface"] != "blocks" {
		t.Fatalf("resolved CapabilityMatrix leaked caller mutation: %#v", again.CapabilityMatrix)
	}

	now = now.Add(90 * time.Second)
	if _, err := control.RecordHeartbeat(context.Background(), "bridge-slack-1", nil); err != nil {
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
			Platform:  "feishu",
			ChatID:    "oc_123",
			MessageID: "om_123",
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

	if err := control.AckDelivery(context.Background(), &model.IMDeliveryAck{
		BridgeID:   "bridge-feishu-1",
		Cursor:     delivery.Cursor,
		DeliveryID: delivery.DeliveryID,
		Status:     string(model.IMDeliveryStatusDelivered),
	}); err != nil {
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

func TestIMControlPlane_AckDeliveryPersistsDowngradeReason(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-dingtalk-1",
		Platform:   "dingtalk",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	delivery, err := control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		TargetBridgeID: "bridge-dingtalk-1",
		Platform:       "dingtalk",
		ProjectID:      "project-1",
		Kind:           IMDeliveryKindNotify,
		Content:        "fallback text",
		TargetChatID:   "chat-1",
	})
	if err != nil {
		t.Fatalf("QueueDelivery error: %v", err)
	}

	control.RecordDeliveryResult(model.IMDelivery{
		ID:           delivery.DeliveryID,
		BridgeID:     "bridge-dingtalk-1",
		ProjectID:    "project-1",
		ChannelID:    "chat-1",
		TargetChatID: "chat-1",
		Platform:     "dingtalk",
		EventType:    "review.requested",
		Status:       model.IMDeliveryStatusDelivered,
		Content:      "fallback text",
	})

	if err := control.AckDelivery(context.Background(), &model.IMDeliveryAck{
		BridgeID:        "bridge-dingtalk-1",
		Cursor:          delivery.Cursor,
		DeliveryID:      delivery.DeliveryID,
		Status:          string(model.IMDeliveryStatusDelivered),
		DowngradeReason: "actioncard_send_failed",
	}); err != nil {
		t.Fatalf("AckDelivery error: %v", err)
	}

	history, err := control.ListDeliveryHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDeliveryHistory error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].DowngradeReason != "actioncard_send_failed" {
		t.Fatalf("DowngradeReason = %q, want actioncard_send_failed", history[0].DowngradeReason)
	}
}

func TestIMControlPlane_AckDeliverySettlesPendingDeliveryAsDelivered(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-slack-1",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	delivery, err := control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		TargetBridgeID: "bridge-slack-1",
		Platform:       "slack",
		ProjectID:      "project-1",
		Kind:           IMDeliveryKindSend,
		Content:        "queued text",
		TargetChatID:   "C123",
	})
	if err != nil {
		t.Fatalf("QueueDelivery error: %v", err)
	}

	control.RecordDeliveryResult(model.IMDelivery{
		ID:           delivery.DeliveryID,
		BridgeID:     "bridge-slack-1",
		ProjectID:    "project-1",
		ChannelID:    "C123",
		TargetChatID: "C123",
		Platform:     "slack",
		EventType:    "message.send",
		Kind:         IMDeliveryKindSend,
		Status:       model.IMDeliveryStatusPending,
		Content:      "queued text",
	})

	if err := control.AckDelivery(context.Background(), &model.IMDeliveryAck{
		BridgeID:   "bridge-slack-1",
		Cursor:     delivery.Cursor,
		DeliveryID: delivery.DeliveryID,
		Status:     string(model.IMDeliveryStatusDelivered),
	}); err != nil {
		t.Fatalf("AckDelivery error: %v", err)
	}

	history, err := control.ListDeliveryHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDeliveryHistory error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].Status != model.IMDeliveryStatusDelivered {
		t.Fatalf("history[0].Status = %q, want delivered", history[0].Status)
	}
}

func TestIMControlPlane_QueueBoundProgressRecordsPendingDeliveryMetadata(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-slack-1",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}
	if err := control.BindAction(context.Background(), &IMActionBinding{
		BridgeID:  "bridge-slack-1",
		Platform:  "slack",
		ProjectID: "project-1",
		TaskID:    "task-1",
		RunID:     "run-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
		},
	}); err != nil {
		t.Fatalf("BindAction error: %v", err)
	}

	queued, err := control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		TaskID:  "task-1",
		RunID:   "run-1",
		Kind:    IMDeliveryKindProgress,
		Content: "Agent is still running",
		Metadata: map[string]string{
			"bridge_event_type": "agent.progress",
		},
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress error: %v", err)
	}
	if !queued {
		t.Fatal("expected bound progress to be queued")
	}

	history, err := control.ListDeliveryHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDeliveryHistory error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	record := history[0]
	if record.Status != model.IMDeliveryStatusPending {
		t.Fatalf("status = %q", record.Status)
	}
	if record.Metadata[imMetadataDeliverySource] != imDeliverySourceBoundProgress {
		t.Fatalf("metadata = %+v", record.Metadata)
	}
	if record.Metadata[imMetadataBridgeBindingRunID] != "run-1" || record.Metadata[imMetadataBridgeBindingTaskID] != "task-1" {
		t.Fatalf("bridge binding metadata = %+v", record.Metadata)
	}
	if record.Metadata[imMetadataReplyTargetThreadID] != "thread-1" {
		t.Fatalf("reply target metadata = %+v", record.Metadata)
	}
}

func TestIMControlPlane_QueueBoundProgressPreservesDeliveryIDForAckSettlement(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-slack-1",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	listener := &fakeBridgeDeliveryListener{}
	if _, err := control.AttachBridgeListener(context.Background(), "bridge-slack-1", 0, listener); err != nil {
		t.Fatalf("AttachBridgeListener error: %v", err)
	}

	if err := control.BindAction(context.Background(), &IMActionBinding{
		BridgeID:  "bridge-slack-1",
		Platform:  "slack",
		ProjectID: "project-1",
		TaskID:    "task-ack",
		RunID:     "run-ack",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-ack",
		},
	}); err != nil {
		t.Fatalf("BindAction error: %v", err)
	}

	queued, err := control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		TaskID:  "task-ack",
		RunID:   "run-ack",
		Kind:    IMDeliveryKindProgress,
		Content: "Agent is still running",
		Metadata: map[string]string{
			"bridge_event_type": "agent.progress",
		},
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress error: %v", err)
	}
	if !queued {
		t.Fatal("expected bound progress to be queued")
	}
	if len(listener.deliveries) != 1 {
		t.Fatalf("listener deliveries = %d, want 1", len(listener.deliveries))
	}

	history, err := control.ListDeliveryHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDeliveryHistory error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	record := history[0]
	if record.ID != listener.deliveries[0].DeliveryID {
		t.Fatalf("history delivery id = %q, want queued delivery id %q", record.ID, listener.deliveries[0].DeliveryID)
	}

	if err := control.AckDelivery(context.Background(), &model.IMDeliveryAck{
		BridgeID:   "bridge-slack-1",
		Cursor:     listener.deliveries[0].Cursor,
		DeliveryID: listener.deliveries[0].DeliveryID,
		Status:     string(model.IMDeliveryStatusDelivered),
	}); err != nil {
		t.Fatalf("AckDelivery error: %v", err)
	}

	history, err = control.ListDeliveryHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDeliveryHistory error after ack: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len after ack = %d, want 1", len(history))
	}
	if history[0].Status != model.IMDeliveryStatusDelivered {
		t.Fatalf("history[0].Status = %q, want delivered", history[0].Status)
	}
}

func TestIMControlPlane_QueueBoundProgressRecordsFailedSettlementWhenBoundBridgeIsMissing(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-slack-1",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}
	if err := control.BindAction(context.Background(), &IMActionBinding{
		BridgeID:  "bridge-slack-1",
		Platform:  "slack",
		ProjectID: "project-1",
		TaskID:    "task-2",
		RunID:     "run-2",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C999",
			ThreadID:  "thread-2",
		},
	}); err != nil {
		t.Fatalf("BindAction error: %v", err)
	}
	if err := control.UnregisterBridge(context.Background(), "bridge-slack-1"); err != nil {
		t.Fatalf("UnregisterBridge error: %v", err)
	}

	queued, err := control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		TaskID:     "task-2",
		RunID:      "run-2",
		Kind:       IMDeliveryKindProgress,
		Content:    "terminal result",
		IsTerminal: true,
		Metadata:   map[string]string{"bridge_event_type": "agent.completed"},
	})
	if err == nil {
		t.Fatal("expected QueueBoundProgress error")
	}
	if queued {
		t.Fatal("expected bound terminal delivery not to queue successfully")
	}

	history, listErr := control.ListDeliveryHistory(context.Background(), nil)
	if listErr != nil {
		t.Fatalf("ListDeliveryHistory error: %v", listErr)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	record := history[0]
	if record.Status != model.IMDeliveryStatusFailed {
		t.Fatalf("status = %q", record.Status)
	}
	if record.Metadata[imMetadataDeliverySource] != imDeliverySourceBoundTerminal {
		t.Fatalf("metadata = %+v", record.Metadata)
	}
	if record.Metadata[imMetadataBridgeBindingBridgeID] != "bridge-slack-1" {
		t.Fatalf("bridge binding metadata = %+v", record.Metadata)
	}
}

func TestIMControlPlane_ListEventTypesReturnsCanonicalSet(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{})

	eventTypes, err := control.ListEventTypes(context.Background())
	if err != nil {
		t.Fatalf("ListEventTypes error: %v", err)
	}
	want := []string{
		"task.created",
		"task.completed",
		"review.completed",
		"agent.started",
		"agent.completed",
		"budget.warning",
		"sprint.started",
		"sprint.completed",
		"review.requested",
		"wiki.page.updated",
		"wiki.version.published",
		"wiki.comment.mention",
		"workflow.failed",
	}
	if len(eventTypes) != len(want) {
		t.Fatalf("event types len = %d, want %d (%v)", len(eventTypes), len(want), eventTypes)
	}
	for idx, expected := range want {
		if eventTypes[idx] != expected {
			t.Fatalf("eventTypes[%d] = %q, want %q", idx, eventTypes[idx], expected)
		}
	}
}

func TestIMControlPlane_RetryDeliveryRequeuesFailedDelivery(t *testing.T) {
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

	control.RecordDeliveryResult(model.IMDelivery{
		ID:           "delivery-1",
		BridgeID:     "bridge-feishu-1",
		ProjectID:    "project-1",
		ChannelID:    "chat-1",
		TargetChatID: "chat-1",
		Platform:     "feishu",
		EventType:    "review.requested",
		Kind:         IMDeliveryKindNotify,
		Status:       model.IMDeliveryStatusFailed,
		Content:      "retry me",
		Metadata: map[string]string{
			"fallback_reason": "thread_reply_unavailable",
		},
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "feishu",
			ChatID:    "chat-1",
			MessageID: "om_1",
		},
	})

	if _, err := control.RetryDelivery(context.Background(), "delivery-1"); err != nil {
		t.Fatalf("RetryDelivery error: %v", err)
	}

	history, err := control.ListDeliveryHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDeliveryHistory error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].Status != model.IMDeliveryStatusPending {
		t.Fatalf("history[0].Status = %q, want pending", history[0].Status)
	}
	if history[0].FailureReason != "" {
		t.Fatalf("history[0].FailureReason = %q, want cleared failure", history[0].FailureReason)
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
			Platform:          "discord",
			ChannelID:         "channel-1",
			InteractionToken:  "token-1",
			PreferredRenderer: "components",
			ProgressMode:      "follow_up",
		},
	}); err != nil {
		t.Fatalf("BindAction error: %v", err)
	}

	sent, err := control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:      "run-1",
		TaskID:     "task-1",
		Kind:       IMDeliveryKindProgress,
		Content:    "Agent 仍在运行，继续处理中",
		IsTerminal: false,
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress error: %v", err)
	}
	if !sent {
		t.Fatal("expected first heartbeat delivery to be sent")
	}

	sent, err = control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:      "run-1",
		TaskID:     "task-1",
		Kind:       IMDeliveryKindProgress,
		Content:    "这条消息应该被节流",
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
		RunID:      "run-1",
		TaskID:     "task-1",
		Kind:       IMDeliveryKindTerminal,
		Content:    "Agent 已完成，任务进入总结阶段",
		IsTerminal: true,
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
	if listener.deliveries[0].ReplyTarget == nil {
		t.Fatal("expected preserved reply target")
	}
	if listener.deliveries[0].ReplyTarget.PreferredRenderer != "components" {
		t.Fatalf("PreferredRenderer = %q", listener.deliveries[0].ReplyTarget.PreferredRenderer)
	}
	if listener.deliveries[0].ReplyTarget.ProgressMode != "follow_up" {
		t.Fatalf("ProgressMode = %q", listener.deliveries[0].ReplyTarget.ProgressMode)
	}
	if listener.deliveries[1].Kind != IMDeliveryKindTerminal {
		t.Fatalf("delivery kind = %q, want %q", listener.deliveries[1].Kind, IMDeliveryKindTerminal)
	}
}

func TestIMControlPlane_QueueBoundProgressRespectsBridgeEventPreferenceMetadata(t *testing.T) {
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
		BridgeID:   "bridge-slack-pref",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	listener := &fakeBridgeDeliveryListener{}
	if _, err := control.AttachBridgeListener(context.Background(), "bridge-slack-pref", 0, listener); err != nil {
		t.Fatalf("AttachBridgeListener error: %v", err)
	}

	if err := control.BindAction(context.Background(), &IMActionBinding{
		BridgeID:  "bridge-slack-pref",
		Platform:  "slack",
		ProjectID: "project-1",
		TaskID:    "task-1",
		RunID:     "run-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "1700000000.1",
			Metadata: map[string]string{
				"bridge_event_enabled.permission_request": "false",
				"bridge_event_enabled.budget.warning":     "false",
				"bridge_event_enabled.status_change":      "true",
			},
		},
	}); err != nil {
		t.Fatalf("BindAction error: %v", err)
	}

	sent, err := control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:   "run-1",
		TaskID:  "task-1",
		Kind:    IMDeliveryKindProgress,
		Content: "permission request should be filtered",
		Metadata: map[string]string{
			"bridge_event_type": "permission_request",
		},
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress(permission_request) error: %v", err)
	}
	if sent {
		t.Fatal("expected disabled permission_request delivery to be skipped")
	}

	sent, err = control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:   "run-1",
		TaskID:  "task-1",
		Kind:    IMDeliveryKindProgress,
		Content: "budget warning should be filtered",
		Metadata: map[string]string{
			"bridge_event_type": "budget.warning",
		},
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress(budget.warning) error: %v", err)
	}
	if sent {
		t.Fatal("expected disabled budget.warning delivery to be skipped")
	}

	sent, err = control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:      "run-1",
		TaskID:     "task-1",
		Kind:       IMDeliveryKindTerminal,
		Content:    "status change should still be delivered",
		IsTerminal: true,
		Metadata: map[string]string{
			"bridge_event_type": "status_change",
		},
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress(status_change) error: %v", err)
	}
	if !sent {
		t.Fatal("expected enabled status_change delivery to be sent")
	}
	if len(listener.deliveries) != 1 {
		t.Fatalf("listener deliveries = %d, want 1", len(listener.deliveries))
	}
	if listener.deliveries[0].Metadata["bridge_event_type"] != "status_change" {
		t.Fatalf("delivery metadata = %+v", listener.deliveries[0].Metadata)
	}
}

func TestIMControlPlane_QueueDeliveryPreservesTypedPayloadAndFallbackMetadata(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-slack-typed",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-typed"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	listener := &fakeBridgeDeliveryListener{}
	if _, err := control.AttachBridgeListener(context.Background(), "bridge-slack-typed", 0, listener); err != nil {
		t.Fatalf("AttachBridgeListener error: %v", err)
	}

	delivery, err := control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		TargetBridgeID: "bridge-slack-typed",
		Platform:       "slack",
		ProjectID:      "project-typed",
		Kind:           IMDeliveryKindNotify,
		Content:        "fallback text",
		Structured: &model.IMStructuredMessage{
			Title: "Task Update",
			Body:  "Agent is still running",
			Fields: []model.IMStructuredField{
				{Label: "Status", Value: "running"},
			},
		},
		Metadata: map[string]string{
			"fallback_reason": "thread_reply_unavailable",
			"delivery_method": "thread_reply",
		},
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "1700000000.1",
		},
	})
	if err != nil {
		t.Fatalf("QueueDelivery error: %v", err)
	}

	if delivery.Structured == nil || delivery.Structured.Title != "Task Update" {
		t.Fatalf("Structured = %+v", delivery.Structured)
	}
	if delivery.Metadata["fallback_reason"] != "thread_reply_unavailable" {
		t.Fatalf("Metadata = %+v", delivery.Metadata)
	}
	if len(listener.deliveries) != 1 {
		t.Fatalf("listener deliveries = %d, want 1", len(listener.deliveries))
	}
	if listener.deliveries[0].Structured == nil || listener.deliveries[0].Structured.Body != "Agent is still running" {
		t.Fatalf("listener structured = %+v", listener.deliveries[0].Structured)
	}
}

func TestRegisterBridge_MultiProviderInventory(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL: time.Minute,
	})

	req := &IMBridgeRegisterRequest{
		BridgeID:  "bridge-multi",
		Platform:  "feishu",
		Transport: "live",
		Providers: []model.IMBridgeProvider{
			{
				ID:             "feishu",
				Transport:      "live",
				ReadinessTier:  "full_native_lifecycle",
				Tenants:        []string{"acme"},
				MetadataSource: "builtin",
			},
			{
				ID:             "slack",
				Transport:      "stub",
				Tenants:        []string{"beta"},
				MetadataSource: "builtin",
			},
		},
		CommandPlugins: []model.IMBridgeCommandPlugin{
			{
				ID:       "@acme/jira",
				Version:  "1.0.0",
				Commands: []string{"/jira"},
				Tenants:  []string{"acme"},
			},
		},
	}

	if _, err := control.RegisterBridge(context.Background(), req); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	status, err := control.GetBridgeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetBridgeStatus error: %v", err)
	}
	if len(status.Bridges) != 1 {
		t.Fatalf("Bridges len = %d, want 1", len(status.Bridges))
	}
	bridge := status.Bridges[0]
	if len(bridge.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(bridge.Providers))
	}
	if bridge.Providers[0].ID != "feishu" || bridge.Providers[1].ID != "slack" {
		t.Errorf("provider ids = %q, %q", bridge.Providers[0].ID, bridge.Providers[1].ID)
	}
	if len(bridge.CommandPlugins) != 1 || bridge.CommandPlugins[0].ID != "@acme/jira" {
		t.Errorf("command plugins = %v", bridge.CommandPlugins)
	}
}

func TestGetBridgeStatus_LegacyBridgeSynthesizesProvider(t *testing.T) {
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL: time.Minute,
	})

	req := &IMBridgeRegisterRequest{
		BridgeID:         "bridge-legacy",
		Platform:         "slack",
		Transport:        "live",
		CapabilityMatrix: map[string]any{"supportsRichMessages": true},
		CallbackPaths:    []string{"/im/notify", "/im/send"},
		Tenants:          []string{"acme"},
		Metadata:         map[string]string{"readiness_tier": "native_send_with_fallback"},
	}
	if _, err := control.RegisterBridge(context.Background(), req); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	status, err := control.GetBridgeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetBridgeStatus error: %v", err)
	}
	if len(status.Bridges) != 1 {
		t.Fatalf("Bridges len = %d", len(status.Bridges))
	}
	providers := status.Bridges[0].Providers
	if len(providers) != 1 {
		t.Fatalf("synthesized Providers len = %d, want 1", len(providers))
	}
	if providers[0].ID != "slack" || providers[0].Transport != "live" {
		t.Errorf("synthesized provider = %+v", providers[0])
	}
	if providers[0].ReadinessTier != "native_send_with_fallback" {
		t.Errorf("synthesized ReadinessTier = %q", providers[0].ReadinessTier)
	}
	if providers[0].MetadataSource != "builtin" {
		t.Errorf("synthesized MetadataSource = %q, want builtin", providers[0].MetadataSource)
	}
}

func TestIMControlPlane_BoundProgressPreservesStructuredPayload(t *testing.T) {
	now := time.Now().UTC()
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})
	control.now = func() time.Time { return now }

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-structured",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}

	listener := &fakeBridgeDeliveryListener{}
	if _, err := control.AttachBridgeListener(context.Background(), "bridge-structured", 0, listener); err != nil {
		t.Fatalf("AttachBridgeListener error: %v", err)
	}

	if err := control.BindAction(context.Background(), &model.IMActionBinding{
		BridgeID: "bridge-structured",
		Platform: "slack",
		TaskID:   "task-1",
		RunID:    "run-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:          "slack",
			ChannelID:         "C123",
			ThreadID:          "1700000000.1",
			PreferredRenderer: "blocks",
		},
	}); err != nil {
		t.Fatalf("BindAction error: %v", err)
	}

	structured := &model.IMStructuredMessage{
		Title: "Agent Run Complete",
		Body:  "Run run-1 finished successfully.",
		Fields: []model.IMStructuredField{
			{Label: "Task", Value: "task-1"},
			{Label: "Run", Value: "run-1"},
			{Label: "Status", Value: "completed"},
		},
	}
	sent, err := control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		RunID:      "run-1",
		TaskID:     "task-1",
		Kind:       IMDeliveryKindTerminal,
		Content:    "Agent completed.",
		Structured: structured,
		IsTerminal: true,
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress error: %v", err)
	}
	if !sent {
		t.Fatal("expected terminal structured delivery to be sent")
	}

	if len(listener.deliveries) != 1 {
		t.Fatalf("listener deliveries = %d, want 1", len(listener.deliveries))
	}
	d := listener.deliveries[0]
	if d.Structured == nil {
		t.Fatal("expected delivery to preserve Structured payload")
	}
	if d.Structured.Title != "Agent Run Complete" {
		t.Fatalf("Structured.Title = %q", d.Structured.Title)
	}
	if len(d.Structured.Fields) != 3 {
		t.Fatalf("Structured.Fields = %d, want 3", len(d.Structured.Fields))
	}
	if d.Content != "Agent completed." {
		t.Fatalf("Content = %q, want explicit content preserved", d.Content)
	}
	if d.ReplyTarget == nil || d.ReplyTarget.PreferredRenderer != "blocks" {
		t.Fatalf("ReplyTarget = %+v", d.ReplyTarget)
	}
	if d.Kind != IMDeliveryKindTerminal {
		t.Fatalf("Kind = %q, want %q", d.Kind, IMDeliveryKindTerminal)
	}
}
