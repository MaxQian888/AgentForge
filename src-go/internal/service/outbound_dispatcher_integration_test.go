//go:build integration

package service_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

// TestIntegration_DispatcherEndToEnd boots a mock IM Bridge server,
// registers the dispatcher with a real eventbus, fires a
// workflow.execution.completed event, and asserts the card is delivered.
func TestIntegration_DispatcherEndToEnd(t *testing.T) {
	var mu sync.Mutex
	var received []map[string]any

	mockBridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]any
		_ = json.Unmarshal(body, &payload)
		mu.Lock()
		received = append(received, payload)
		mu.Unlock()
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"sent"}`))
	}))
	defer mockBridge.Close()

	feBaseURL := "https://app.example.com"
	execID := uuid.New()
	wfID := uuid.New()
	projID := uuid.New()

	sysMeta, _ := json.Marshal(map[string]any{
		"reply_target": map[string]any{
			"platform":   "feishu",
			"chat_id":    "test-chat-1",
			"message_id": "msg-1",
		},
	})

	exec := &model.WorkflowExecution{
		ID:             execID,
		WorkflowID:     wfID,
		ProjectID:      projID,
		Status:         model.WorkflowExecStatusCompleted,
		SystemMetadata: sysMeta,
	}

	repo := &fakeExecRepo{exec: exec}
	var emitted []eb.Event
	bus := &captureBus{onPublish: func(e *eb.Event) {
		mu.Lock()
		emitted = append(emitted, *e)
		mu.Unlock()
	}}

	d := service.NewOutboundDispatcher(repo, mockBridge.URL, feBaseURL, bus)
	d.SetRetryDelays(0, 0, 0)

	// Fire the event
	payload, _ := json.Marshal(map[string]any{
		"executionId": execID.String(),
		"workflowId":  wfID.String(),
		"status":      model.WorkflowExecStatusCompleted,
	})
	ev := eb.NewEvent(ws.EventWorkflowExecutionCompleted, "core", "project:"+projID.String())
	ev.Payload = payload
	d.Observe(context.Background(), ev, &eb.PipelineCtx{})

	// Wait for async dispatch
	deadline := time.After(2 * time.Second)
	for {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n > 0 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("mock bridge did not receive a POST within 2s")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	mu.Lock()
	defer mu.Unlock()

	// Assert the payload
	if len(received) != 1 {
		t.Fatalf("expected 1 POST, got %d", len(received))
	}
	r := received[0]

	// Platform should be feishu
	if r["platform"] != "feishu" {
		t.Errorf("expected platform=feishu, got %v", r["platform"])
	}

	// Card should be present
	card, ok := r["card"].(map[string]any)
	if !ok {
		t.Fatalf("expected card object, got %T: %v", r["card"], r["card"])
	}

	// Card title should be non-empty
	title, _ := card["title"].(string)
	if title == "" {
		t.Error("card title is empty")
	}

	// Card actions should contain a view URL with the execution ID
	actions, _ := card["actions"].([]any)
	if len(actions) == 0 {
		t.Error("card has no actions")
	} else {
		actionJSON, _ := json.Marshal(actions[0])
		if !strings.Contains(string(actionJSON), execID.String()) {
			t.Errorf("action URL should contain execution ID %s, got %s", execID, string(actionJSON))
		}
	}

	// No failure events should have been emitted
	if len(emitted) != 0 {
		t.Errorf("expected no failure events, got %d: %+v", len(emitted), emitted)
	}
}

// TestIntegration_DispatcherSkipsImDispatched verifies the dispatcher
// does NOT post when system_metadata.im_dispatched=true.
func TestIntegration_DispatcherSkipsImDispatched(t *testing.T) {
	var postCount int32
	mockBridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		postCount++
		w.WriteHeader(200)
	}))
	defer mockBridge.Close()

	execID := uuid.New()
	projID := uuid.New()

	sysMeta, _ := json.Marshal(map[string]any{
		"reply_target":  map[string]any{"platform": "feishu", "chat_id": "c"},
		"im_dispatched": true,
	})

	exec := &model.WorkflowExecution{
		ID:             execID,
		WorkflowID:     uuid.New(),
		ProjectID:      projID,
		Status:         model.WorkflowExecStatusCompleted,
		SystemMetadata: sysMeta,
	}

	d := service.NewOutboundDispatcher(&fakeExecRepo{exec: exec}, mockBridge.URL, "https://fe.example", nil)
	d.SetRetryDelays(0, 0, 0)

	payload, _ := json.Marshal(map[string]any{
		"executionId": execID.String(),
		"status":      model.WorkflowExecStatusCompleted,
	})
	ev := eb.NewEvent(ws.EventWorkflowExecutionCompleted, "core", "project:"+projID.String())
	ev.Payload = payload
	d.Observe(context.Background(), ev, &eb.PipelineCtx{})

	time.Sleep(200 * time.Millisecond)
	if postCount != 0 {
		t.Fatalf("expected 0 posts (im_dispatched=true), got %d", postCount)
	}
}
