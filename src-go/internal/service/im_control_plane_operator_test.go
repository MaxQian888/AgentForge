package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestIMControlPlane_OperatorStateSupportsChannelsStatusAndHistory(t *testing.T) {
	now := time.Date(2026, 3, 26, 8, 0, 0, 0, time.UTC)
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              2 * time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		Now: func() time.Time {
			return now
		},
	})

	channel, err := control.UpsertChannel(context.Background(), &model.IMChannel{
		Platform:   "feishu",
		Name:       "Alerts",
		ChannelID:  "chat-1",
		WebhookURL: "https://example.test/webhook",
		Events:     []string{"task.created"},
		Active:     true,
	})
	if err != nil {
		t.Fatalf("UpsertChannel(create) error = %v", err)
	}
	if channel.ID == "" {
		t.Fatal("expected generated channel id")
	}

	channel.Name = "Ops Alerts"
	if _, err := control.UpsertChannel(context.Background(), channel); err != nil {
		t.Fatalf("UpsertChannel(update) error = %v", err)
	}

	channels, err := control.ListChannels(context.Background())
	if err != nil {
		t.Fatalf("ListChannels() error = %v", err)
	}
	if len(channels) != 1 || channels[0].Name != "Ops Alerts" {
		t.Fatalf("ListChannels() = %+v", channels)
	}

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-feishu-1",
		Platform:   "feishu",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge() error = %v", err)
	}

	control.RecordDeliveryResult(model.IMDelivery{
		ChannelID: "chat-1",
		Platform:  "feishu",
		EventType: "task.created",
		Status:    model.IMDeliveryStatusDelivered,
	})

	status, err := control.GetBridgeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetBridgeStatus() error = %v", err)
	}
	if !status.Registered {
		t.Fatal("expected registered bridge status")
	}
	if status.Health != "healthy" {
		t.Fatalf("Health = %q, want healthy", status.Health)
	}
	if len(status.Providers) != 1 || status.Providers[0] != "feishu" {
		t.Fatalf("Providers = %+v", status.Providers)
	}
	if status.LastHeartbeat == nil || *status.LastHeartbeat == "" {
		t.Fatalf("LastHeartbeat = %+v, want non-empty timestamp", status.LastHeartbeat)
	}

	history, err := control.ListDeliveryHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDeliveryHistory() error = %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("len(ListDeliveryHistory()) = %d, want 1", len(history))
	}
	if history[0].Status != model.IMDeliveryStatusDelivered {
		t.Fatalf("history[0].Status = %q", history[0].Status)
	}
	if history[0].CreatedAt == "" || history[0].ID == "" {
		t.Fatalf("history[0] = %+v, want generated id and timestamp", history[0])
	}

	if err := control.DeleteChannel(context.Background(), channel.ID); err != nil {
		t.Fatalf("DeleteChannel() error = %v", err)
	}
	channels, err = control.ListChannels(context.Background())
	if err != nil {
		t.Fatalf("ListChannels() after delete error = %v", err)
	}
	if len(channels) != 0 {
		t.Fatalf("len(ListChannels()) after delete = %d, want 0", len(channels))
	}
}

func TestIMControlPlane_GetBridgeStatusIncludesOperatorSummary(t *testing.T) {
	now := time.Date(2026, 3, 26, 9, 0, 0, 0, time.UTC)
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              2 * time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		Now: func() time.Time {
			return now
		},
	})

	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-slack-1",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge() error = %v", err)
	}
	if _, err := control.RegisterBridge(context.Background(), &IMBridgeRegisterRequest{
		BridgeID:   "bridge-slack-2",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{"project-1"},
	}); err != nil {
		t.Fatalf("RegisterBridge(second) error = %v", err)
	}

	pendingDelivery, err := control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		TargetBridgeID: "bridge-slack-1",
		Platform:       "slack",
		ProjectID:      "project-1",
		Kind:           IMDeliveryKindNotify,
		Content:        "pending",
		TargetChatID:   "C123",
	})
	if err != nil {
		t.Fatalf("QueueDelivery(pending) error = %v", err)
	}
	control.RecordDeliveryResult(model.IMDelivery{
		ID:           pendingDelivery.DeliveryID,
		BridgeID:     "bridge-slack-1",
		ProjectID:    "project-1",
		ChannelID:    "C123",
		TargetChatID: "C123",
		Platform:     "slack",
		EventType:    "task.created",
		Kind:         IMDeliveryKindNotify,
		Status:       model.IMDeliveryStatusPending,
		Content:      "pending",
	})

	now = now.Add(15 * time.Second)
	settledDelivery, err := control.QueueDelivery(context.Background(), IMQueueDeliveryRequest{
		TargetBridgeID: "bridge-slack-2",
		Platform:       "slack",
		ProjectID:      "project-1",
		Kind:           IMDeliveryKindNotify,
		Content:        "delivered",
		TargetChatID:   "C123",
	})
	if err != nil {
		t.Fatalf("QueueDelivery(settled) error = %v", err)
	}
	control.RecordDeliveryResult(model.IMDelivery{
		ID:           settledDelivery.DeliveryID,
		BridgeID:     "bridge-slack-2",
		ProjectID:    "project-1",
		ChannelID:    "C123",
		TargetChatID: "C123",
		Platform:     "slack",
		EventType:    "task.completed",
		Kind:         IMDeliveryKindNotify,
		Status:       model.IMDeliveryStatusPending,
		Content:      "delivered",
	})

	now = now.Add(5 * time.Second)
	if err := control.AckDelivery(context.Background(), &model.IMDeliveryAck{
		BridgeID:   "bridge-slack-2",
		Cursor:     settledDelivery.Cursor,
		DeliveryID: settledDelivery.DeliveryID,
		Status:     string(model.IMDeliveryStatusDelivered),
	}); err != nil {
		t.Fatalf("AckDelivery() error = %v", err)
	}

	status, err := control.GetBridgeStatus(context.Background())
	if err != nil {
		t.Fatalf("GetBridgeStatus() error = %v", err)
	}

	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatalf("unmarshal status payload: %v", err)
	}

	if payload["pendingDeliveries"] != float64(1) {
		t.Fatalf("pendingDeliveries = %#v, want 1", payload["pendingDeliveries"])
	}
	if payload["averageLatencyMs"] == nil {
		t.Fatalf("averageLatencyMs missing from status payload: %#v", payload)
	}

	rawProviderDetails, ok := payload["providerDetails"].([]any)
	if !ok || len(rawProviderDetails) != 1 {
		t.Fatalf("providerDetails = %#v, want one provider entry", payload["providerDetails"])
	}
	providerDetails, ok := rawProviderDetails[0].(map[string]any)
	if !ok {
		t.Fatalf("providerDetails[0] = %#v, want object", rawProviderDetails[0])
	}
	if providerDetails["pendingDeliveries"] != float64(1) {
		t.Fatalf("provider pendingDeliveries = %#v, want 1", providerDetails["pendingDeliveries"])
	}
	if providerDetails["lastDeliveryAt"] == nil {
		t.Fatalf("provider lastDeliveryAt missing: %#v", providerDetails)
	}
}

func TestIMControlPlane_ListDeliveryHistoryAppliesFilters(t *testing.T) {
	now := time.Date(2026, 3, 26, 10, 0, 0, 0, time.UTC)
	control := NewIMControlPlane(IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		Now: func() time.Time {
			return now
		},
	})

	control.RecordDeliveryResult(model.IMDelivery{
		ID:        "delivery-1",
		Platform:  "slack",
		EventType: "task.created",
		Kind:      IMDeliveryKindNotify,
		Status:    model.IMDeliveryStatusFailed,
		CreatedAt: "2026-03-26T09:00:00Z",
	})
	control.RecordDeliveryResult(model.IMDelivery{
		ID:        "delivery-2",
		Platform:  "slack",
		EventType: "task.completed",
		Kind:      IMDeliveryKindNotify,
		Status:    model.IMDeliveryStatusDelivered,
		CreatedAt: "2026-03-26T09:30:00Z",
	})
	control.RecordDeliveryResult(model.IMDelivery{
		ID:        "delivery-3",
		Platform:  "feishu",
		EventType: "task.created",
		Kind:      IMDeliveryKindSend,
		Status:    model.IMDeliveryStatusFailed,
		CreatedAt: "2026-03-26T09:45:00Z",
	})

	history, err := control.ListDeliveryHistory(context.Background(), &model.IMDeliveryHistoryFilters{
		Status:    string(model.IMDeliveryStatusFailed),
		Platform:  "slack",
		EventType: "task.created",
		Kind:      IMDeliveryKindNotify,
		Since:     "2026-03-26T08:30:00Z",
	})
	if err != nil {
		t.Fatalf("ListDeliveryHistory() error = %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("len(history) = %d, want 1", len(history))
	}
	if history[0].ID != "delivery-1" {
		t.Fatalf("history[0].ID = %q, want delivery-1", history[0].ID)
	}
}
