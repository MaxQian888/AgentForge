package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
}
