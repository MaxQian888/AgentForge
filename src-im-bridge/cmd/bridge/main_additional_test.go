package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
)

func TestDurationEnvOrDefault_UsesFallbackForInvalidValue(t *testing.T) {
	t.Setenv("IM_BRIDGE_RECONNECT_DELAY", "not-a-duration")
	if got := durationEnvOrDefault("IM_BRIDGE_RECONNECT_DELAY", 5*time.Second); got != 5*time.Second {
		t.Fatalf("durationEnvOrDefault = %v", got)
	}
}

func TestBackendActionRelay_HandleAction_UsesRequestPlatformAndBridgeContext(t *testing.T) {
	var gotBody client.IMActionRequest
	var gotSource string
	var gotBridgeID string
	var gotReplyTarget string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotSource = r.Header.Get("X-IM-Source")
		gotBridgeID = r.Header.Get("X-IM-Bridge-ID")
		gotReplyTarget = r.Header.Get("X-IM-Reply-Target")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(client.IMActionResponse{
			Result:      "Approved",
			ReplyTarget: gotBody.ReplyTarget,
			Metadata:    map[string]string{"source": "block_actions"},
		})
	}))
	defer server.Close()

	relay := &backendActionRelay{
		client:   client.NewAgentForgeClient(server.URL, "proj", "secret"),
		bridgeID: "bridge-default",
	}

	resp, err := relay.HandleAction(context.Background(), &notify.ActionRequest{
		Platform: "slack-stub",
		Action:   "approve",
		EntityID: "review-1",
		ChatID:   "C123",
		UserID:   "U123",
		ReplyTarget: &core.ReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "thread-1",
		},
	})
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}
	if resp == nil || resp.Result != "Approved" || resp.Metadata["source"] != "block_actions" {
		t.Fatalf("response = %+v", resp)
	}
	if gotSource != "slack" {
		t.Fatalf("X-IM-Source = %q", gotSource)
	}
	if gotBridgeID != "bridge-default" {
		t.Fatalf("X-IM-Bridge-ID = %q", gotBridgeID)
	}
	if !strings.Contains(gotReplyTarget, "\"threadId\":\"thread-1\"") {
		t.Fatalf("X-IM-Reply-Target = %q", gotReplyTarget)
	}
	if gotBody.Action != "approve" || gotBody.EntityID != "review-1" || gotBody.BridgeID != "bridge-default" {
		t.Fatalf("body = %+v", gotBody)
	}
}

func TestBackendActionRelay_HandleAction_NilInputsAreSafe(t *testing.T) {
	var relay *backendActionRelay
	resp, err := relay.HandleAction(context.Background(), nil)
	if err != nil {
		t.Fatalf("HandleAction error: %v", err)
	}
	if resp != nil {
		t.Fatalf("response = %+v, want nil", resp)
	}
}
