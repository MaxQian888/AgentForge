package service_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/agentforge/server/internal/eventbus"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
	"github.com/agentforge/server/internal/service"
)

func TestEventDrivenExecutor_Mode(t *testing.T) {
	exec := service.NewEventDrivenExecutor(nil, nil)
	if exec.Mode() != model.WorkflowProcessEventDriven {
		t.Errorf("Mode() = %q, want event-driven", exec.Mode())
	}
}

func TestEventDrivenExecutor_RequiresTriggers(t *testing.T) {
	bus := eventbus.NewBus()
	exec := service.NewEventDrivenExecutor(&seqStubStepRouter{}, bus)
	_, err := exec.Execute(context.Background(), plugin.WorkflowPlan{
		PluginID: "x",
		Spec: &model.WorkflowPluginSpec{
			Process:  model.WorkflowProcessEventDriven,
			Triggers: nil,
		},
	})
	if err == nil {
		t.Error("expected error when no triggers configured")
	}
}

func TestEventDrivenExecutor_MatchesTriggerAndDispatchesAgent(t *testing.T) {
	router := &eventDrivenRecordingRouter{}
	bus := eventbus.NewBus()
	exec := service.NewEventDrivenExecutor(router, bus)

	plan := plugin.WorkflowPlan{
		PluginID: "test-event-driven",
		Spec: &model.WorkflowPluginSpec{
			Process: model.WorkflowProcessEventDriven,
			Triggers: []model.PluginWorkflowTrigger{
				{
					Event:         "integration.im.message_received",
					Filter:        map[string]any{"platform": "slack"},
					Role:          "project-assistant",
					Action:        "agent",
					MaxConcurrent: 2,
				},
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := exec.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	go func() {
		time.Sleep(20 * time.Millisecond)
		ev := mustEvent(t, "integration.im.message_received", map[string]any{"platform": "slack", "text": "hello"})
		_ = bus.Publish(ctx, ev)
	}()

	deadline := time.After(2 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before any event arrived")
			}
			if e.Type == "step_completed" {
				if router.snapshotLen() != 1 {
					t.Errorf("expected 1 dispatched step, got %d", router.snapshotLen())
				}
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for triggered step_completed")
		}
	}
}

func TestEventDrivenExecutor_DoesNotMatchWrongFilter(t *testing.T) {
	router := &eventDrivenRecordingRouter{}
	bus := eventbus.NewBus()
	exec := service.NewEventDrivenExecutor(router, bus)

	plan := plugin.WorkflowPlan{
		PluginID: "test",
		Spec: &model.WorkflowPluginSpec{
			Process: model.WorkflowProcessEventDriven,
			Triggers: []model.PluginWorkflowTrigger{
				{
					Event:  "integration.im.message_received",
					Filter: map[string]any{"platform": "slack"},
					Role:   "project-assistant",
					Action: "agent",
				},
			},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()

	ch, err := exec.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	ev := mustEvent(t, "integration.im.message_received", map[string]any{"platform": "discord"})
	_ = bus.Publish(ctx, ev)

	for e := range ch {
		if e.Type == "step_started" || e.Type == "step_completed" {
			t.Errorf("non-matching filter should not have dispatched: %v", e)
		}
	}
	if router.snapshotLen() != 0 {
		t.Errorf("expected 0 dispatches, got %d", router.snapshotLen())
	}
}

// eventDrivenRecordingRouter records dispatched steps in a thread-safe slice.
type eventDrivenRecordingRouter struct {
	mu      sync.Mutex
	dispatched []string
}

func (r *eventDrivenRecordingRouter) Execute(ctx context.Context, req service.WorkflowStepExecutionRequest) (*service.WorkflowStepExecutionResult, error) {
	r.mu.Lock()
	r.dispatched = append(r.dispatched, req.Step.Role)
	r.mu.Unlock()
	return &service.WorkflowStepExecutionResult{Output: map[string]any{"role": req.Step.Role}}, nil
}

func (r *eventDrivenRecordingRouter) snapshotLen() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.dispatched)
}

// mustEvent builds a valid eventbus.Event with payload encoded as JSON.
func mustEvent(t *testing.T, eventType string, payload map[string]any) *eventbus.Event {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	e := eventbus.NewEvent(eventType, "test", "system:broadcast")
	e.Payload = encoded
	return e
}
