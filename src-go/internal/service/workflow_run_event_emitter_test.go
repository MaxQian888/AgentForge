package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
)

type capturingEventPublisher struct {
	events []*eventbus.Event
}

func (p *capturingEventPublisher) Publish(_ context.Context, e *eventbus.Event) error {
	p.events = append(p.events, e)
	return nil
}

func TestWorkflowRunEventEmitter_DAGStatusChanged(t *testing.T) {
	pub := &capturingEventPublisher{}
	emitter := NewWorkflowRunEventEmitter(pub)

	projectID := uuid.New()
	exec := &model.WorkflowExecution{
		ID:         uuid.New(),
		WorkflowID: uuid.New(),
		ProjectID:  projectID,
		Status:     model.WorkflowExecStatusRunning,
		StartedAt:  viewPtrTime(time.Now().UTC()),
	}
	emitter.EmitDAGStatusChanged(context.Background(), exec)

	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	ev := pub.events[0]
	if ev.Type != eventbus.EventWorkflowRunStatusChanged {
		t.Errorf("event type = %q, want %q", ev.Type, eventbus.EventWorkflowRunStatusChanged)
	}
}

func TestWorkflowRunEventEmitter_DAGTerminal(t *testing.T) {
	pub := &capturingEventPublisher{}
	emitter := NewWorkflowRunEventEmitter(pub)

	exec := &model.WorkflowExecution{
		ID:         uuid.New(),
		WorkflowID: uuid.New(),
		ProjectID:  uuid.New(),
		Status:     model.WorkflowExecStatusCompleted,
	}
	emitter.EmitDAGTerminal(context.Background(), exec)
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
	if pub.events[0].Type != eventbus.EventWorkflowRunTerminal {
		t.Errorf("event type = %q", pub.events[0].Type)
	}
}

func TestWorkflowRunEventEmitter_Plugin(t *testing.T) {
	pub := &capturingEventPublisher{}
	emitter := NewWorkflowRunEventEmitter(pub)

	run := &model.WorkflowPluginRun{
		ID:        uuid.New(),
		PluginID:  "my-plugin",
		ProjectID: uuid.New(),
		Status:    model.WorkflowRunStatusRunning,
		StartedAt: time.Now().UTC(),
	}
	emitter.EmitPluginStatusChanged(context.Background(), run)
	emitter.EmitPluginTerminal(context.Background(), &model.WorkflowPluginRun{
		ID:        run.ID,
		PluginID:  run.PluginID,
		ProjectID: run.ProjectID,
		Status:    model.WorkflowRunStatusCompleted,
		StartedAt: run.StartedAt,
	})
	if len(pub.events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(pub.events))
	}
	if pub.events[0].Type != eventbus.EventWorkflowRunStatusChanged {
		t.Errorf("events[0].type = %q", pub.events[0].Type)
	}
	if pub.events[1].Type != eventbus.EventWorkflowRunTerminal {
		t.Errorf("events[1].type = %q", pub.events[1].Type)
	}
}

func TestWorkflowRunEventEmitter_NilSafe(t *testing.T) {
	var emitter *WorkflowRunEventEmitter
	// Should not panic when receiver is nil.
	emitter.EmitDAGStatusChanged(context.Background(), &model.WorkflowExecution{})
	emitter.EmitPluginTerminal(context.Background(), &model.WorkflowPluginRun{})
}
