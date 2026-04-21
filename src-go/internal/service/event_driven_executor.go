package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agentforge/server/internal/eventbus"
	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
	log "github.com/sirupsen/logrus"
)

// EventDrivenExecutor is a persistent EventBus subscriber. It watches for
// events whose Type matches one of the spec's Triggers, applies any
// per-trigger Filter to the event payload, and dispatches a workflow step
// when both match. MaxConcurrent caps in-flight dispatches per trigger.
//
// The executor is NOT routed through the Trigger Engine (which handles
// one-time dispatch); the lifetime of the subscription matches the ctx
// passed to Execute. Cancel the ctx to stop the subscriber and close the
// emitted event channel.
type EventDrivenExecutor struct {
	stepRouter WorkflowStepExecutor
	bus        *eventbus.Bus
}

func NewEventDrivenExecutor(router WorkflowStepExecutor, bus *eventbus.Bus) *EventDrivenExecutor {
	return &EventDrivenExecutor{stepRouter: router, bus: bus}
}

func (e *EventDrivenExecutor) Mode() model.WorkflowProcessMode {
	return model.WorkflowProcessEventDriven
}

// Cancel is a no-op: cancellation is via the ctx passed to Execute.
func (e *EventDrivenExecutor) Cancel(_ context.Context, _ string) error { return nil }

func (e *EventDrivenExecutor) Execute(ctx context.Context, plan plugin.WorkflowPlan) (<-chan plugin.WorkflowEvent, error) {
	if plan.Spec == nil {
		return nil, fmt.Errorf("event-driven workflow %q has no spec", plan.PluginID)
	}
	if len(plan.Spec.Triggers) == 0 {
		return nil, fmt.Errorf("event-driven workflow %q requires at least one trigger", plan.PluginID)
	}
	if e.bus == nil {
		return nil, fmt.Errorf("event-driven workflow %q requires a bus", plan.PluginID)
	}

	outCh := make(chan plugin.WorkflowEvent, 64)

	// Per-trigger semaphore enforces MaxConcurrent.
	sems := make([]chan struct{}, len(plan.Spec.Triggers))
	for i, trig := range plan.Spec.Triggers {
		capacity := trig.MaxConcurrent
		if capacity <= 0 {
			capacity = 1
		}
		sems[i] = make(chan struct{}, capacity)
	}

	go func() {
		defer close(outCh)

		eventCh := e.bus.Subscribe(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-eventCh:
				if !ok {
					return
				}
				for i, trig := range plan.Spec.Triggers {
					if ev.Type != trig.Event {
						continue
					}
					if !matchesEventFilter(ev, trig.Filter) {
						continue
					}
					select {
					case sems[i] <- struct{}{}:
					default:
						continue // at capacity, skip this event for this trigger
					}
					go e.dispatch(ctx, plan, trig, sems[i], ev, outCh)
				}
			}
		}
	}()

	return outCh, nil
}

func (e *EventDrivenExecutor) dispatch(
	ctx context.Context,
	plan plugin.WorkflowPlan,
	trigger model.PluginWorkflowTrigger,
	sem chan struct{},
	busEvent *eventbus.Event,
	outCh chan<- plugin.WorkflowEvent,
) {
	defer func() { <-sem }()
	if applog.TraceID(ctx) == "" {
		ctx = applog.WithTrace(ctx, applog.NewTraceID())
		log.WithFields(log.Fields{"trace_id": applog.TraceID(ctx), "origin": "eventbus.subscriber"}).Info("trace.generated_for_background_job")
	}

	stepID := fmt.Sprintf("event:%s:%s", trigger.Event, trigger.Role)
	outCh <- plugin.WorkflowEvent{Type: "step_started", StepID: stepID}

	var payload map[string]any
	_ = json.Unmarshal(busEvent.Payload, &payload)

	req := WorkflowStepExecutionRequest{
		PluginID: plan.PluginID,
		Process:  model.WorkflowProcessEventDriven,
		Step: model.WorkflowStepDefinition{
			ID:     stepID,
			Role:   trigger.Role,
			Action: model.WorkflowActionType(trigger.Action),
		},
		Input: sequentialMergeInputs(plan.Input, payload),
	}
	result, err := e.stepRouter.Execute(ctx, req)
	if err != nil {
		outCh <- plugin.WorkflowEvent{Type: "step_failed", StepID: stepID, Err: err}
		return
	}
	output := map[string]any{}
	if result != nil {
		output = result.Output
	}
	outCh <- plugin.WorkflowEvent{Type: "step_completed", StepID: stepID, Payload: output}
}

// matchesEventFilter returns true when every key in filter is present in the
// event payload with a stringly-equal value. An empty filter matches all
// payloads. Compares via fmt.Sprint so int/string/bool comparisons survive
// the JSON-roundtrip the bus payload undergoes.
func matchesEventFilter(ev *eventbus.Event, filter map[string]any) bool {
	if len(filter) == 0 {
		return true
	}
	var payload map[string]any
	if err := json.Unmarshal(ev.Payload, &payload); err != nil {
		return false
	}
	for k, want := range filter {
		got, ok := payload[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
			return false
		}
	}
	return true
}
