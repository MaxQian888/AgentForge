package service

import (
	"context"
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

	history, err := control.ListDeliveryHistory(context.Background())
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
