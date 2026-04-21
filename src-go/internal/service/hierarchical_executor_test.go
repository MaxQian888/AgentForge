package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
	"github.com/agentforge/server/internal/service"
)

func TestHierarchicalExecutor_Mode(t *testing.T) {
	exec := service.NewHierarchicalExecutor(nil)
	if exec.Mode() != model.WorkflowProcessHierarchical {
		t.Errorf("Mode() = %q, want hierarchical", exec.Mode())
	}
}

func TestHierarchicalExecutor_RequiresManagerRole(t *testing.T) {
	exec := service.NewHierarchicalExecutor(&seqStubStepRouter{})
	plan := plugin.WorkflowPlan{
		PluginID: "test",
		Spec: &model.WorkflowPluginSpec{
			Process:     model.WorkflowProcessHierarchical,
			ManagerRole: "",
			WorkerRoles: []string{"coding-agent"},
		},
	}
	_, err := exec.Execute(context.Background(), plan)
	if err == nil {
		t.Error("expected error when ManagerRole is empty")
	}
}

func TestHierarchicalExecutor_RequiresWorkerRoles(t *testing.T) {
	exec := service.NewHierarchicalExecutor(&seqStubStepRouter{})
	plan := plugin.WorkflowPlan{
		PluginID: "test",
		Spec: &model.WorkflowPluginSpec{
			Process:     model.WorkflowProcessHierarchical,
			ManagerRole: "manager",
			WorkerRoles: nil,
		},
	}
	_, err := exec.Execute(context.Background(), plan)
	if err == nil {
		t.Error("expected error when WorkerRoles is empty")
	}
}

func TestHierarchicalExecutor_DispatchesWorkersAndCompletes(t *testing.T) {
	router := &hierarchicalRecordingRouter{}
	exec := service.NewHierarchicalExecutor(router)

	plan := plugin.WorkflowPlan{
		PluginID: "test",
		Spec: &model.WorkflowPluginSpec{
			Process:             model.WorkflowProcessHierarchical,
			ManagerRole:         "project-assistant",
			WorkerRoles:         []string{"coding-agent", "test-engineer"},
			MaxParallelWorkers:  2,
			WorkerFailurePolicy: "best_effort",
		},
		Input: map[string]any{"task": "build the feature"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ch, err := exec.Execute(ctx, plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var events []plugin.WorkflowEvent
	for e := range ch {
		events = append(events, e)
	}

	if events[len(events)-1].Type != "completed" {
		t.Errorf("last event type = %q, want completed; events: %v", events[len(events)-1].Type, events)
	}
	managerCalls := 0
	workerCalls := 0
	for _, role := range router.snapshot() {
		if role == "project-assistant" {
			managerCalls++
		} else {
			workerCalls++
		}
	}
	if managerCalls != 2 {
		t.Errorf("expected manager called twice (decompose + aggregate), got %d", managerCalls)
	}
	if workerCalls != 2 {
		t.Errorf("expected 2 worker calls, got %d", workerCalls)
	}
}

func TestHierarchicalExecutor_FailFastStopsOnWorkerError(t *testing.T) {
	router := &hierarchicalRecordingRouter{failRole: "coding-agent"}
	exec := service.NewHierarchicalExecutor(router)
	plan := plugin.WorkflowPlan{
		PluginID: "test",
		Spec: &model.WorkflowPluginSpec{
			Process:             model.WorkflowProcessHierarchical,
			ManagerRole:         "manager",
			WorkerRoles:         []string{"coding-agent", "test-engineer"},
			WorkerFailurePolicy: "fail_fast",
		},
	}
	ch, err := exec.Execute(context.Background(), plan)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	var last plugin.WorkflowEvent
	for e := range ch {
		last = e
	}
	if last.Type != "failed" {
		t.Errorf("expected fail_fast to terminate with failed, got %q", last.Type)
	}
}

func TestHierarchicalExecutor_BestEffortContinuesPastWorkerError(t *testing.T) {
	router := &hierarchicalRecordingRouter{failRole: "coding-agent"}
	exec := service.NewHierarchicalExecutor(router)
	plan := plugin.WorkflowPlan{
		PluginID: "test",
		Spec: &model.WorkflowPluginSpec{
			Process:             model.WorkflowProcessHierarchical,
			ManagerRole:         "manager",
			WorkerRoles:         []string{"coding-agent", "test-engineer"},
			WorkerFailurePolicy: "best_effort",
		},
	}
	ch, _ := exec.Execute(context.Background(), plan)
	var last plugin.WorkflowEvent
	for e := range ch {
		last = e
	}
	if last.Type != "completed" {
		t.Errorf("expected best_effort to complete despite one worker failure, last = %q", last.Type)
	}
}

// hierarchicalRecordingRouter records the sequence of step roles dispatched
// and can be configured to fail one specific worker role.
type hierarchicalRecordingRouter struct {
	mu       sync.Mutex
	calls    []string
	failRole string
}

func (r *hierarchicalRecordingRouter) Execute(ctx context.Context, req service.WorkflowStepExecutionRequest) (*service.WorkflowStepExecutionResult, error) {
	r.mu.Lock()
	r.calls = append(r.calls, req.Step.Role)
	r.mu.Unlock()
	if r.failRole != "" && req.Step.Role == r.failRole {
		return nil, errStubFailure{role: req.Step.Role}
	}
	return &service.WorkflowStepExecutionResult{Output: map[string]any{"role": req.Step.Role, "ok": true}}, nil
}

func (r *hierarchicalRecordingRouter) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

type errStubFailure struct{ role string }

func (e errStubFailure) Error() string { return "stub failure for role " + e.role }
