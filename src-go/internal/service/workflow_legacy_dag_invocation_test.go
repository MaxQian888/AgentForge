package service_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	rolepkg "github.com/agentforge/server/internal/role"
	"github.com/agentforge/server/internal/service"
)

// workflowSingleWorkflowStepPluginRecord saves a minimal workflow plugin
// whose single step has action=workflow so parking and resume can be
// exercised in isolation.
func workflowSingleWorkflowStepPluginRecord(t *testing.T, repo *repository.PluginRegistryRepository) *model.PluginRecord {
	t.Helper()
	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindWorkflow,
			Metadata: model.PluginMetadata{
				ID:      "workflow.parent",
				Name:    "Parent",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     "./dist/parent.wasm",
				ABIVersion: "v1",
				Workflow: &model.WorkflowPluginSpec{
					Process: model.WorkflowProcessSequential,
					Roles:   []model.WorkflowRoleBinding{{ID: "coder"}},
					Steps: []model.WorkflowStepDefinition{
						{ID: "invoke-dag", Role: "coder", Action: model.WorkflowActionWorkflow},
					},
				},
			},
		},
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostGoOrchestrator,
	}
	if err := repo.Save(context.Background(), record); err != nil {
		t.Fatalf("save workflow plugin: %v", err)
	}
	return record
}

// parkOnlyStepExecutor returns a fixed awaiting_sub_workflow output for every
// step invocation so the plugin run parks immediately.
type parkOnlyStepExecutor struct {
	childRunID uuid.UUID
	invoked    int
}

func (e *parkOnlyStepExecutor) Execute(_ context.Context, _ service.WorkflowStepExecutionRequest) (*service.WorkflowStepExecutionResult, error) {
	e.invoked++
	return &service.WorkflowStepExecutionResult{
		Output: map[string]any{
			"status":            string(model.WorkflowStepRunStatusAwaitingSubWorkflow),
			"child_run_id":      e.childRunID.String(),
			"child_engine":      "dag",
			"child_workflow_id": uuid.New().String(),
		},
	}, nil
}

// TestWorkflowExecutionService_ParksOnAwaitingSubWorkflowStatus: when the
// `workflow` step router emits status=awaiting_sub_workflow, the run pauses
// and the parked step's status reflects awaiting_sub_workflow so the resume
// hook can find it by (runID, childRunID).
func TestWorkflowExecutionService_ParksOnAwaitingSubWorkflowStatus(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	childRunID := uuid.New()
	exec := &parkOnlyStepExecutor{childRunID: childRunID}
	record := workflowSingleWorkflowStepPluginRecord(t, pluginRepo)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		},
		exec,
	)

	run, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	if run.Status != model.WorkflowRunStatusPaused {
		t.Fatalf("run status = %s, want paused", run.Status)
	}
	if run.Steps[0].Status != model.WorkflowStepRunStatusAwaitingSubWorkflow {
		t.Fatalf("step status = %s, want awaiting_sub_workflow", run.Steps[0].Status)
	}
	if cr, _ := run.Steps[0].Output["child_run_id"].(string); cr != childRunID.String() {
		t.Errorf("step output child_run_id = %v, want %s", run.Steps[0].Output["child_run_id"], childRunID)
	}
}

// TestWorkflowExecutionService_ResumeParkedDAGChild_Completed: after a run is
// parked, ResumeParkedDAGChild with outcome=completed transitions the step
// to Completed, marks the run Completed, and records child outcome/outputs.
func TestWorkflowExecutionService_ResumeParkedDAGChild_Completed(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	childRunID := uuid.New()
	exec := &parkOnlyStepExecutor{childRunID: childRunID}
	record := workflowSingleWorkflowStepPluginRecord(t, pluginRepo)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		},
		exec,
	)

	ctx := context.Background()
	run, err := svc.Start(ctx, record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if run.Status != model.WorkflowRunStatusPaused {
		t.Fatalf("precondition failed: run %s", run.Status)
	}

	resumeErr := svc.ResumeParkedDAGChild(ctx, run.ID, childRunID, model.SubWorkflowLinkStatusCompleted, map[string]any{"result": "ok"})
	if resumeErr != nil {
		t.Fatalf("resume: %v", resumeErr)
	}

	resumed, err := runRepo.GetByID(ctx, run.ID)
	if err != nil {
		t.Fatalf("reload run: %v", err)
	}
	if resumed.Status != model.WorkflowRunStatusCompleted {
		t.Errorf("resumed run status = %s, want completed", resumed.Status)
	}
	if resumed.Steps[0].Status != model.WorkflowStepRunStatusCompleted {
		t.Errorf("resumed step status = %s, want completed", resumed.Steps[0].Status)
	}
	if outcome, _ := resumed.Steps[0].Output["child_outcome"].(string); outcome != model.SubWorkflowLinkStatusCompleted {
		t.Errorf("child_outcome = %v, want completed", resumed.Steps[0].Output["child_outcome"])
	}
	if outputs, _ := resumed.Steps[0].Output["child_outputs"].(map[string]any); outputs["result"] != "ok" {
		t.Errorf("child_outputs = %v, want {result:ok}", resumed.Steps[0].Output["child_outputs"])
	}
}

// TestWorkflowExecutionService_ResumeParkedDAGChild_Failed: failure from the
// child fails the parked step and marks the plugin run Failed.
func TestWorkflowExecutionService_ResumeParkedDAGChild_Failed(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	childRunID := uuid.New()
	exec := &parkOnlyStepExecutor{childRunID: childRunID}
	record := workflowSingleWorkflowStepPluginRecord(t, pluginRepo)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		},
		exec,
	)
	ctx := context.Background()
	run, err := svc.Start(ctx, record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	if err := svc.ResumeParkedDAGChild(ctx, run.ID, childRunID, model.SubWorkflowLinkStatusFailed, nil); err != nil {
		t.Fatalf("resume: %v", err)
	}

	resumed, _ := runRepo.GetByID(ctx, run.ID)
	if resumed.Status != model.WorkflowRunStatusFailed {
		t.Errorf("resumed run status = %s, want failed", resumed.Status)
	}
	if resumed.Steps[0].Status != model.WorkflowStepRunStatusFailed {
		t.Errorf("resumed step status = %s, want failed", resumed.Steps[0].Status)
	}
}

// TestWorkflowExecutionService_CancelRun_CascadesToParkedDAGChild: calling
// CancelRun on a plugin run whose step is parked on a DAG child invokes the
// DAG starter's Cancel hook before transitioning the parent to cancelled.
func TestWorkflowExecutionService_CancelRun_CascadesToParkedDAGChild(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	childRunID := uuid.New()
	exec := &parkOnlyStepExecutor{childRunID: childRunID}
	// Configure the router with a DAG starter so cancelRun can invoke its
	// Cancel hook. The router's WorkflowDAGChildStarter path is engaged
	// by the handleParkedStep's cascade helper, which reads the starter
	// from the executor.
	dagStarter := &dagChildStarterMock{}
	router := service.NewWorkflowStepRouterExecutor(nil, nil, nil).
		WithDAGChildStarter(dagStarter, &parentLinkWriterMock{})
	// Wrap the mock executor so the service still drives steps through it,
	// while the router exposes the DAG starter for cancellation cascade.
	composed := &composedParkExecutor{mock: exec, router: router}

	record := workflowSingleWorkflowStepPluginRecord(t, pluginRepo)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		},
		composed,
	)
	ctx := context.Background()
	run, err := svc.Start(ctx, record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if run.Status != model.WorkflowRunStatusPaused {
		t.Fatalf("precondition: run status = %s, want paused", run.Status)
	}

	if err := svc.CancelRun(ctx, run.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	if len(dagStarter.cancelled) != 1 || dagStarter.cancelled[0] != childRunID {
		t.Errorf("expected DAG child %s cancelled, got %v", childRunID, dagStarter.cancelled)
	}
	cancelled, _ := runRepo.GetByID(ctx, run.ID)
	if cancelled.Status != model.WorkflowRunStatusCancelled {
		t.Errorf("run status after cancel = %s, want cancelled", cancelled.Status)
	}
}

// composedParkExecutor is a WorkflowStepExecutor that delegates Execute to
// the mock parkOnlyStepExecutor while exposing the router's DAGChildStarter
// so the plugin runtime's cascade helper can reach it via the
// DAGChildStarter() method.
type composedParkExecutor struct {
	mock   *parkOnlyStepExecutor
	router *service.WorkflowStepRouterExecutor
}

func (c *composedParkExecutor) Execute(ctx context.Context, req service.WorkflowStepExecutionRequest) (*service.WorkflowStepExecutionResult, error) {
	return c.mock.Execute(ctx, req)
}

func (c *composedParkExecutor) DAGChildStarter() service.WorkflowDAGChildStarter {
	return c.router.DAGChildStarter()
}
