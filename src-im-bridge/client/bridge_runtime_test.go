package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestBridgeLifecycleRequestsUseExpectedEndpointsAndPayloads(t *testing.T) {
	var gotRegister BridgeRegistration
	var gotHeartbeat map[string]string
	var gotUnregister map[string]string
	var gotBinding IMActionBinding

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/im/bridge/register":
			if err := json.NewDecoder(r.Body).Decode(&gotRegister); err != nil {
				t.Fatalf("decode register: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(BridgeInstance{BridgeID: "bridge-1", Status: "online"})
		case "/api/v1/im/bridge/heartbeat":
			if err := json.NewDecoder(r.Body).Decode(&gotHeartbeat); err != nil {
				t.Fatalf("decode heartbeat: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(BridgeHeartbeat{BridgeID: "bridge-1", Status: "online"})
		case "/api/v1/im/bridge/unregister":
			if err := json.NewDecoder(r.Body).Decode(&gotUnregister); err != nil {
				t.Fatalf("decode unregister: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case "/api/v1/im/bridge/bind":
			if err := json.NewDecoder(r.Body).Decode(&gotBinding); err != nil {
				t.Fatalf("decode binding: %v", err)
			}
			w.WriteHeader(http.StatusCreated)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	target := &core.ReplyTarget{Platform: "slack", ChannelID: "C123", ThreadID: "thread-1"}
	client := NewAgentForgeClient(server.URL, "proj-1", "secret").WithSource("slack-stub").WithBridgeContext("bridge-1", target)

	instance, err := client.RegisterBridge(context.Background(), BridgeRegistration{BridgeID: "bridge-1", Platform: "slack", Transport: "stub"})
	if err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}
	if instance.BridgeID != "bridge-1" || instance.Status != "online" {
		t.Fatalf("instance = %+v", instance)
	}

	heartbeat, err := client.HeartbeatBridge(context.Background(), "bridge-1")
	if err != nil {
		t.Fatalf("HeartbeatBridge error: %v", err)
	}
	if heartbeat.BridgeID != "bridge-1" || heartbeat.Status != "online" {
		t.Fatalf("heartbeat = %+v", heartbeat)
	}

	if err := client.BindActionContext(context.Background(), IMActionBinding{TaskID: "task-1"}); err != nil {
		t.Fatalf("BindActionContext error: %v", err)
	}

	if err := client.UnregisterBridge(context.Background(), "bridge-1"); err != nil {
		t.Fatalf("UnregisterBridge error: %v", err)
	}

	if gotRegister.BridgeID != "bridge-1" || gotRegister.Platform != "slack" {
		t.Fatalf("register payload = %+v", gotRegister)
	}
	if gotHeartbeat["bridgeId"] != "bridge-1" {
		t.Fatalf("heartbeat payload = %+v", gotHeartbeat)
	}
	if gotUnregister["bridgeId"] != "bridge-1" {
		t.Fatalf("unregister payload = %+v", gotUnregister)
	}
	if gotBinding.TaskID != "task-1" || gotBinding.Platform != "slack" || gotBinding.BridgeID != "bridge-1" {
		t.Fatalf("binding payload = %+v", gotBinding)
	}
	if gotBinding.ReplyTarget == nil || gotBinding.ReplyTarget.ThreadID != "thread-1" {
		t.Fatalf("binding reply target = %+v", gotBinding.ReplyTarget)
	}
}

func TestReadError_FormatsStatusAndBody(t *testing.T) {
	client := NewAgentForgeClient("http://example.test", "proj-1", "secret")
	resp := &http.Response{
		StatusCode: http.StatusBadGateway,
		Body:       io.NopCloser(strings.NewReader("bridge failed")),
	}

	err := client.readError(resp)
	if err == nil {
		t.Fatal("expected readError to return an error")
	}
	if !strings.Contains(err.Error(), "API error 502: bridge failed") {
		t.Fatalf("err = %v", err)
	}
}
