package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
	"github.com/agentforge/server/internal/service"
)

func TestSequentialExecutor_Mode(t *testing.T) {
	exec := service.NewSequentialExecutor(nil)
	if exec.Mode() != model.WorkflowProcessSequential {
		t.Errorf("Mode() = %q, want sequential", exec.Mode())
	}
}

func TestSequentialExecutor_EmptyStepsCompletes(t *testing.T) {
	exec := service.NewSequentialExecutor(&seqStubStepRouter{})
	plan := plugin.WorkflowPlan{
		PluginID: "test-workflow",
		Spec: &model.WorkflowPluginSpec{
			Process: model.WorkflowProcessSequential,
			Steps:   []model.WorkflowStepDefinition{},
		},
		Input: map[string]any{},
	}
	ch, err := exec.Execute(context.Background(), plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var events []plugin.WorkflowEvent
	for e := range ch {
		events = append(events, e)
	}
	if len(events) != 1 || events[0].Type != "completed" {
		t.Errorf("expected single [completed] event, got %v", events)
	}
}

func TestSequentialExecutor_RunsStepsInOrder(t *testing.T) {
	router := &seqRecordingStepRouter{}
	exec := service.NewSequentialExecutor(router)
	plan := plugin.WorkflowPlan{
		PluginID: "test-workflow",
		Spec: &model.WorkflowPluginSpec{
			Process: model.WorkflowProcessSequential,
			Steps: []model.WorkflowStepDefinition{
				{ID: "s1", Role: "coding-agent", Action: model.WorkflowActionAgent},
				{ID: "s2", Role: "test-engineer", Action: model.WorkflowActionAgent},
			},
		},
		Input: map[string]any{"task": "x"},
	}
	ch, err := exec.Execute(context.Background(), plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var events []plugin.WorkflowEvent
	for e := range ch {
		events = append(events, e)
	}
	if len(router.executed) != 2 {
		t.Fatalf("expected 2 step executions, got %d", len(router.executed))
	}
	if router.executed[0] != "s1" || router.executed[1] != "s2" {
		t.Errorf("steps ran out of order: %v", router.executed)
	}
	if events[len(events)-1].Type != "completed" {
		t.Errorf("last event = %v, want completed", events[len(events)-1])
	}
}

func TestSequentialExecutor_StopsOnStepFailure(t *testing.T) {
	router := &seqRecordingStepRouter{failAt: "s1"}
	exec := service.NewSequentialExecutor(router)
	plan := plugin.WorkflowPlan{
		PluginID: "test-workflow",
		Spec: &model.WorkflowPluginSpec{
			Steps: []model.WorkflowStepDefinition{
				{ID: "s1", Role: "x", Action: model.WorkflowActionAgent},
				{ID: "s2", Role: "y", Action: model.WorkflowActionAgent},
			},
		},
	}
	ch, _ := exec.Execute(context.Background(), plan)
	var events []plugin.WorkflowEvent
	for e := range ch {
		events = append(events, e)
	}
	if len(router.executed) != 1 {
		t.Errorf("expected only s1 to run, got %v", router.executed)
	}
	if events[len(events)-1].Type != "failed" {
		t.Errorf("last event type = %q, want failed", events[len(events)-1].Type)
	}
}

// seqStubStepRouter satisfies WorkflowStepExecutor and returns success.
type seqStubStepRouter struct{}

func (s *seqStubStepRouter) Execute(ctx context.Context, req service.WorkflowStepExecutionRequest) (*service.WorkflowStepExecutionResult, error) {
	return &service.WorkflowStepExecutionResult{Output: map[string]any{"ok": true}}, nil
}

// seqRecordingStepRouter records step IDs in execution order and can fail at one.
type seqRecordingStepRouter struct {
	executed []string
	failAt   string
}

func (r *seqRecordingStepRouter) Execute(ctx context.Context, req service.WorkflowStepExecutionRequest) (*service.WorkflowStepExecutionResult, error) {
	r.executed = append(r.executed, req.Step.ID)
	if r.failAt != "" && req.Step.ID == r.failAt {
		return nil, errors.New("simulated step failure")
	}
	return &service.WorkflowStepExecutionResult{Output: map[string]any{"step": req.Step.ID}}, nil
}
