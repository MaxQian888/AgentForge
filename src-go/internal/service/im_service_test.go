package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestIMService_SendCompatibilityPayloadIncludesReplyTarget(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/im/send" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewIMService(server.URL, "slack")
	err := svc.Send(context.Background(), &model.IMSendRequest{
		Platform:  "slack",
		ChannelID: "C123",
		Text:      "hello",
		ReplyTarget: &model.IMReplyTarget{
			Platform:           "slack",
			ChannelID:          "C123",
			ThreadID:           "thread-1",
			PreferredRenderer:  "blocks",
			OriginalResponseID: "resp-1",
		},
	})
	if err != nil {
		t.Fatalf("Send error: %v", err)
	}

	replyTarget, ok := payload["replyTarget"].(map[string]any)
	if !ok {
		t.Fatalf("replyTarget = %#v", payload["replyTarget"])
	}
	if replyTarget["threadId"] != "thread-1" {
		t.Fatalf("threadId = %v", replyTarget["threadId"])
	}
	if replyTarget["preferredRenderer"] != "blocks" {
		t.Fatalf("preferredRenderer = %v", replyTarget["preferredRenderer"])
	}
}

func TestIMService_NotifyCompatibilityPayloadIncludesReplyTarget(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/im/notify" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewIMService(server.URL, "telegram")
	err := svc.Notify(context.Background(), &model.IMNotifyRequest{
		Platform:  "telegram",
		ChannelID: "chat-1",
		Event:     "task_progress",
		Title:     "Task Update",
		Body:      "Still running",
		ReplyTarget: &model.IMReplyTarget{
			Platform:     "telegram",
			ChatID:       "chat-1",
			MessageID:    "42",
			ProgressMode: "edit",
		},
	})
	if err != nil {
		t.Fatalf("Notify error: %v", err)
	}

	replyTarget, ok := payload["replyTarget"].(map[string]any)
	if !ok {
		t.Fatalf("replyTarget = %#v", payload["replyTarget"])
	}
	if replyTarget["messageId"] != "42" {
		t.Fatalf("messageId = %v", replyTarget["messageId"])
	}
	if replyTarget["progressMode"] != "edit" {
		t.Fatalf("progressMode = %v", replyTarget["progressMode"])
	}
}

func TestIMService_HandleActionPreservesReplyTargetAndMetadata(t *testing.T) {
	svc := NewIMService("", "slack")
	// Wire an executor that returns a successful response to test preservation.
	svc.SetActionExecutor(IMActionExecutorFunc(func(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
		return &model.IMActionResponse{
			Result:      "Approved",
			Success:     true,
			Status:      model.IMActionStatusCompleted,
			ReplyTarget: req.ReplyTarget,
			Metadata:    req.Metadata,
		}, nil
	}))

	resp, err := svc.HandleAction(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "approve",
		EntityID:  "review-1",
		ChannelID: "C123",
		UserID:    "U123",
		BridgeID:  "bridge-slack-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:          "slack",
			ChannelID:         "C123",
			ThreadID:          "thread-1",
			PreferredRenderer: "blocks",
		},
		Metadata: map[string]string{
			"source": "block_actions",
		},
	})
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Success = false")
	}
	if resp.ReplyTarget == nil || resp.ReplyTarget.ThreadID != "thread-1" {
		t.Fatalf("ReplyTarget = %+v", resp.ReplyTarget)
	}
	if resp.Metadata["source"] != "block_actions" {
		t.Fatalf("Metadata = %+v", resp.Metadata)
	}
	if resp.Metadata[imMetadataDeliverySource] != imDeliverySourceActionResult {
		t.Fatalf("delivery source metadata = %+v", resp.Metadata)
	}
	if resp.Metadata[imMetadataBridgeBindingBridgeID] != "bridge-slack-1" {
		t.Fatalf("bridge binding metadata = %+v", resp.Metadata)
	}
}

func TestIMService_NotifyCompatibilityPayloadIncludesTypedDeliveryFields(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/im/notify" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	svc := NewIMService(server.URL, "slack")
	err := svc.Notify(context.Background(), &model.IMNotifyRequest{
		Platform:  "slack",
		ChannelID: "C123",
		Event:     "task_progress",
		Title:     "Task Update",
		Body:      "Still running",
		Structured: &model.IMStructuredMessage{
			Title: "Task Update",
			Body:  "Still running",
			Fields: []model.IMStructuredField{
				{Label: "Status", Value: "running"},
			},
		},
		Metadata: map[string]string{
			"fallback_reason": "thread_reply_unavailable",
		},
		ReplyTarget: &model.IMReplyTarget{
			Platform: "slack",
			ChatID:   "C123",
			ThreadID: "thread-1",
		},
	})
	if err != nil {
		t.Fatalf("Notify error: %v", err)
	}

	structured, ok := payload["structured"].(map[string]any)
	if !ok {
		t.Fatalf("structured = %#v", payload["structured"])
	}
	if structured["title"] != "Task Update" {
		t.Fatalf("structured title = %v", structured["title"])
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("metadata = %#v", payload["metadata"])
	}
	if metadata["fallback_reason"] != "thread_reply_unavailable" {
		t.Fatalf("metadata = %#v", metadata)
	}
}

func TestIMService_HandleActionReturnsFailureWhenExecutorReturnsNil(t *testing.T) {
	svc := NewIMService("", "", nil)
	svc.SetActionExecutor(IMActionExecutorFunc(func(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
		return nil, nil // executor returns nil to signal unhandled action
	}))

	resp, err := svc.HandleAction(context.Background(), &model.IMActionRequest{
		Action:   "unknown-action",
		EntityID: "entity-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
		},
		Metadata: map[string]string{"source": "test"},
	})
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}
	if resp.Success {
		t.Fatal("expected Success=false for unhandled action")
	}
	if resp.Status != model.IMActionStatusFailed {
		t.Fatalf("Status = %q, want %q", resp.Status, model.IMActionStatusFailed)
	}
	if resp.ReplyTarget == nil || resp.ReplyTarget.ChannelID != "C123" {
		t.Fatalf("ReplyTarget not preserved: %+v", resp.ReplyTarget)
	}
	if resp.Metadata["source"] != "test" {
		t.Fatalf("Metadata not preserved: %+v", resp.Metadata)
	}
}

func TestIMService_SendQueuesPendingDeliveryForControlPlane(t *testing.T) {
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

	svc := NewIMService("", "slack", control)
	if err := svc.Send(context.Background(), &model.IMSendRequest{
		Platform:   "slack",
		ChannelID:  "C123",
		Text:       "queued via control plane",
		ProjectID:  "project-1",
		BridgeID:   "bridge-slack-1",
		DeliveryID: "delivery-1",
	}); err != nil {
		t.Fatalf("Send error: %v", err)
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
	if history[0].ID != "delivery-1" {
		t.Fatalf("history[0].ID = %q, want delivery-1", history[0].ID)
	}
	if history[0].Metadata[imMetadataDeliverySource] != imDeliverySourceCompatSend {
		t.Fatalf("metadata = %+v", history[0].Metadata)
	}
	if history[0].Metadata[imMetadataBridgeBindingBridgeID] != "bridge-slack-1" {
		t.Fatalf("bridge binding metadata = %+v", history[0].Metadata)
	}
}

func TestIMService_NotifyQueuesPendingDeliveryForControlPlane(t *testing.T) {
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

	svc := NewIMService("", "feishu", control)
	if err := svc.Notify(context.Background(), &model.IMNotifyRequest{
		Platform:   "feishu",
		ChannelID:  "chat-1",
		Event:      "task.created",
		Title:      "Created",
		Body:       "Task created",
		ProjectID:  "project-1",
		BridgeID:   "bridge-feishu-1",
		DeliveryID: "delivery-notify-1",
	}); err != nil {
		t.Fatalf("Notify error: %v", err)
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
	if history[0].ID != "delivery-notify-1" {
		t.Fatalf("history[0].ID = %q, want delivery-notify-1", history[0].ID)
	}
}

type intentClassifierStub struct {
	resp *ClassifyIntentResponse
	err  error
}

func TestIMService_HandleActionStoresTaskBindingForLaterFollowUp(t *testing.T) {
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

	svc := NewIMService("", "slack", control)
	svc.SetActionExecutor(IMActionExecutorFunc(func(ctx context.Context, req *model.IMActionRequest) (*model.IMActionResponse, error) {
		return &model.IMActionResponse{
			Result:  "Task moved to in_progress",
			Success: true,
			Status:  model.IMActionStatusCompleted,
			Task: &model.TaskDTO{
				ID:        req.EntityID,
				ProjectID: "project-1",
				Title:     "Bridge rollout",
				Status:    model.TaskStatusInProgress,
				Priority:  "high",
			},
		}, nil
	}))

	taskID := "550e8400-e29b-41d4-a716-446655440000"
	resp, err := svc.HandleAction(context.Background(), &model.IMActionRequest{
		Platform:  "slack",
		Action:    "transition-task",
		EntityID:  taskID,
		ChannelID: "C123",
		UserID:    "U123",
		BridgeID:  "bridge-slack-1",
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
		},
	})
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}
	if resp == nil || !resp.Success {
		t.Fatalf("response = %+v", resp)
	}

	queued, err := control.QueueBoundProgress(context.Background(), IMBoundProgressRequest{
		TaskID:     taskID,
		Kind:       IMDeliveryKindTerminal,
		Content:    "workflow follow-up",
		IsTerminal: true,
		Metadata: map[string]string{
			"bridge_event_type": "task.workflow_trigger",
		},
	})
	if err != nil {
		t.Fatalf("QueueBoundProgress error: %v", err)
	}
	if !queued {
		t.Fatal("expected task binding to allow bound follow-up delivery")
	}

	history, err := control.ListDeliveryHistory(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListDeliveryHistory error: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("history len = %d, want 1", len(history))
	}
	if history[0].Metadata[imMetadataBridgeBindingTaskID] != taskID {
		t.Fatalf("metadata = %+v", history[0].Metadata)
	}
	if history[0].Metadata[imMetadataReplyTargetThreadID] != "thread-1" {
		t.Fatalf("reply target metadata = %+v", history[0].Metadata)
	}
}

func (s intentClassifierStub) ClassifyIntent(_ context.Context, req ClassifyIntentRequest) (*ClassifyIntentResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.resp, nil
}

func TestIMService_HandleIntentFallsBackToKeywordSuggestionWhenClassifierUnavailable(t *testing.T) {
	svc := NewIMService("", "slack")

	resp, err := svc.HandleIntent(context.Background(), &model.IMIntentRequest{
		Text:      "暂停 run-123",
		UserID:    "user-1",
		ProjectID: "project-1",
	})
	if err != nil {
		t.Fatalf("HandleIntent error: %v", err)
	}
	if !strings.Contains(resp.Reply, "/agent pause run-123") {
		t.Fatalf("Reply = %q", resp.Reply)
	}
}

func TestIMService_HandleIntentFallsBackToKeywordSuggestionWhenClassifierFails(t *testing.T) {
	svc := NewIMService("", "slack")
	svc.SetClassifier(intentClassifierStub{err: errors.New("bridge down")})

	resp, err := svc.HandleIntent(context.Background(), &model.IMIntentRequest{
		Text:      "暂停 run-123",
		UserID:    "user-1",
		ProjectID: "project-1",
	})
	if err != nil {
		t.Fatalf("HandleIntent error: %v", err)
	}
	if !strings.Contains(resp.Reply, "/agent pause run-123") {
		t.Fatalf("Reply = %q", resp.Reply)
	}
}
