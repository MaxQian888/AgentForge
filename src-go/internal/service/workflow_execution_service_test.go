package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/service"
)

type workflowStepExecutorMock struct {
	calls      []service.WorkflowStepExecutionRequest
	failCounts map[string]int
	seen       map[string]int
	outputs    map[string]map[string]any
}

func (m *workflowStepExecutorMock) Execute(_ context.Context, req service.WorkflowStepExecutionRequest) (*service.WorkflowStepExecutionResult, error) {
	m.calls = append(m.calls, req)
	if m.seen == nil {
		m.seen = make(map[string]int)
	}
	m.seen[req.Step.ID]++
	if remaining := m.failCounts[req.Step.ID]; remaining > 0 {
		m.failCounts[req.Step.ID] = remaining - 1
		return nil, errors.New("step execution failed")
	}
	output := map[string]any{
		"step":    req.Step.ID,
		"attempt": req.Attempt,
	}
	if custom := m.outputs[req.Step.ID]; custom != nil {
		output = custom
	}
	return &service.WorkflowStepExecutionResult{
		Output: output,
	}, nil
}

func saveWorkflowPluginRecord(t *testing.T, repo *repository.PluginRegistryRepository, state model.PluginLifecycleState, process model.WorkflowProcessMode, maxRetries int) *model.PluginRecord {
	t.Helper()
	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindWorkflow,
			Metadata: model.PluginMetadata{
				ID:      "workflow.release-train",
				Name:    "Release Train",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     "./dist/release-train.wasm",
				ABIVersion: "v1",
				Workflow: &model.WorkflowPluginSpec{
					Process: process,
					Roles: []model.WorkflowRoleBinding{
						{ID: "coder"},
						{ID: "reviewer"},
					},
					Steps: []model.WorkflowStepDefinition{
						{ID: "implement", Role: "coder", Action: model.WorkflowActionAgent, Next: []string{"review"}},
						{ID: "review", Role: "reviewer", Action: model.WorkflowActionReview},
					},
					Limits: &model.WorkflowExecutionLimits{
						MaxRetries: maxRetries,
					},
				},
			},
		},
		LifecycleState: state,
		RuntimeHost:    model.PluginHostGoOrchestrator,
	}
	if err := repo.Save(context.Background(), record); err != nil {
		t.Fatalf("save workflow plugin record: %v", err)
	}
	return record
}

func TestWorkflowExecutionService_StartSequentialWorkflowPersistsOrderedSteps(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{}
	record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateActive, model.WorkflowProcessSequential, 0)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
				"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
			},
		},
		executor,
	)

	run, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	if run.Status != model.WorkflowRunStatusCompleted {
		t.Fatalf("workflow status = %s, want completed", run.Status)
	}
	if len(run.Steps) != 2 {
		t.Fatalf("len(run.Steps) = %d, want 2", len(run.Steps))
	}
	if len(executor.calls) != 2 {
		t.Fatalf("len(executor.calls) = %d, want 2", len(executor.calls))
	}
	if executor.calls[0].Step.ID != "implement" || executor.calls[1].Step.ID != "review" {
		t.Fatalf("unexpected execution order: %+v", executor.calls)
	}
	stepsInput, ok := executor.calls[1].Input["steps"].(map[string]any)
	if !ok {
		t.Fatalf("expected second step input to include prior step outputs, got %+v", executor.calls[1].Input)
	}
	if _, ok := stepsInput["implement"]; !ok {
		t.Fatalf("expected implement output in second step input, got %+v", stepsInput)
	}

	stored, err := runRepo.GetByID(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("get stored workflow run: %v", err)
	}
	if stored.Status != model.WorkflowRunStatusCompleted {
		t.Fatalf("stored workflow status = %s, want completed", stored.Status)
	}
}

func TestWorkflowExecutionService_RetriesFailedStepAndPersistsAttemptHistory(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{
		failCounts: map[string]int{
			"implement": 1,
		},
	}
	record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateEnabled, model.WorkflowProcessSequential, 1)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
				"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
			},
		},
		executor,
	)

	run, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatalf("start workflow with retry: %v", err)
	}
	if run.Status != model.WorkflowRunStatusCompleted {
		t.Fatalf("workflow status = %s, want completed", run.Status)
	}
	if run.Steps[0].RetryCount != 1 {
		t.Fatalf("retry count = %d, want 1", run.Steps[0].RetryCount)
	}
	if len(run.Steps[0].Attempts) != 2 {
		t.Fatalf("len(attempts) = %d, want 2", len(run.Steps[0].Attempts))
	}
	if run.Steps[0].Attempts[0].Status != model.WorkflowStepRunStatusFailed {
		t.Fatalf("first attempt status = %s, want failed", run.Steps[0].Attempts[0].Status)
	}
	if run.Steps[0].Attempts[1].Status != model.WorkflowStepRunStatusCompleted {
		t.Fatalf("second attempt status = %s, want completed", run.Steps[0].Attempts[1].Status)
	}
}

func TestWorkflowExecutionService_FailsDisabledOrUnsupportedWorkflow(t *testing.T) {
	roleStore := &fakePluginRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
		},
	}

	t.Run("disabled workflow", func(t *testing.T) {
		pluginRepo := repository.NewPluginRegistryRepository()
		runRepo := repository.NewWorkflowPluginRunRepository()
		executor := &workflowStepExecutorMock{}
		record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateDisabled, model.WorkflowProcessSequential, 0)
		svc := service.NewWorkflowExecutionService(pluginRepo, runRepo, roleStore, executor)

		if _, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{}); err == nil {
			t.Fatal("expected disabled workflow start to fail")
		}
	})

	t.Run("hierarchical workflow now supported", func(t *testing.T) {
		pluginRepo := repository.NewPluginRegistryRepository()
		runRepo := repository.NewWorkflowPluginRunRepository()
		executor := &workflowStepExecutorMock{}
		record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateEnabled, model.WorkflowProcessHierarchical, 0)
		svc := service.NewWorkflowExecutionService(pluginRepo, runRepo, roleStore, executor)

		// Hierarchical workflows are now supported - should not fail on process mode
		_, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{})
		// May fail for other reasons (no steps, etc.) but NOT because of unsupported process
		if err != nil && strings.Contains(err.Error(), "unsupported workflow process") {
			t.Fatal("hierarchical workflow should no longer be rejected as unsupported")
		}
	})
}

func TestWorkflowExecutionService_PersistsFailedRunAfterRetryBudgetExhausted(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{
		failCounts: map[string]int{
			"implement": 2,
		},
	}
	record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateActive, model.WorkflowProcessSequential, 1)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
				"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
			},
		},
		executor,
	)

	run, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{})
	if err == nil {
		t.Fatal("expected workflow run to fail after exhausting retries")
	}
	if run == nil {
		t.Fatal("expected failed workflow run to be returned")
	}
	if run.Status != model.WorkflowRunStatusFailed {
		t.Fatalf("workflow status = %s, want failed", run.Status)
	}
	if len(run.Steps[0].Attempts) != 2 {
		t.Fatalf("len(attempts) = %d, want 2", len(run.Steps[0].Attempts))
	}
	if run.Steps[0].Status != model.WorkflowStepRunStatusFailed {
		t.Fatalf("step status = %s, want failed", run.Steps[0].Status)
	}
}

func TestWorkflowExecutionService_GetAndListRuns(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{}
	record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateActive, model.WorkflowProcessSequential, 0)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder":    {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
				"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
			},
		},
		executor,
	)

	firstRun, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"ordinal": 1},
	})
	if err != nil {
		t.Fatalf("start first workflow run: %v", err)
	}

	secondRun, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"ordinal": 2},
	})
	if err != nil {
		t.Fatalf("start second workflow run: %v", err)
	}

	got, err := svc.GetRun(context.Background(), firstRun.ID)
	if err != nil {
		t.Fatalf("get workflow run: %v", err)
	}
	if got.ID != firstRun.ID {
		t.Fatalf("got workflow run id = %s, want %s", got.ID, firstRun.ID)
	}

	runs, err := svc.ListRuns(context.Background(), record.Metadata.ID, 10)
	if err != nil {
		t.Fatalf("list workflow runs: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("len(runs) = %d, want 2", len(runs))
	}
	if runs[0].ID != secondRun.ID && runs[1].ID != secondRun.ID {
		t.Fatalf("expected second run %s to be listed, got %+v", secondRun.ID, runs)
	}
}

func TestWorkflowExecutionService_StartRejectsStaleRoleReferenceBeforeCreatingRun(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{}
	record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateEnabled, model.WorkflowProcessSequential, 0)
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		},
		executor,
	)

	run, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{})
	if err == nil || !strings.Contains(err.Error(), "reviewer") {
		t.Fatalf("Start() error = %v, want stale reviewer dependency failure", err)
	}
	if run != nil {
		t.Fatalf("run = %+v, want nil when stale role blocks startup", run)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %d, want 0 when startup is blocked before step execution", len(executor.calls))
	}
	runs, listErr := runRepo.ListByPluginID(context.Background(), record.Metadata.ID, 10)
	if listErr != nil {
		t.Fatalf("ListByPluginID() error = %v", listErr)
	}
	if len(runs) != 0 {
		t.Fatalf("len(runs) = %d, want 0 when stale role blocks startup", len(runs))
	}
}

func TestWorkflowExecutionService_PausesRunWhenApprovalStepRequestsApproval(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{
		outputs: map[string]map[string]any{
			"await-approval": {
				"status": "awaiting_approval",
			},
		},
	}
	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindWorkflow,
			Metadata: model.PluginMetadata{
				ID:      "workflow.review-escalation",
				Name:    "Review Escalation",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     "./dist/review-escalation.wasm",
				ABIVersion: "v1",
				Workflow: &model.WorkflowPluginSpec{
					Process: model.WorkflowProcessSequential,
					Roles: []model.WorkflowRoleBinding{
						{ID: "reviewer"},
						{ID: "planner"},
					},
					Steps: []model.WorkflowStepDefinition{
						{ID: "deep-review", Role: "reviewer", Action: model.WorkflowActionReview, Next: []string{"await-approval"}},
						{ID: "await-approval", Role: "planner", Action: model.WorkflowActionApproval},
					},
				},
			},
		},
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostGoOrchestrator,
	}
	if err := pluginRepo.Save(context.Background(), record); err != nil {
		t.Fatalf("save workflow plugin record: %v", err)
	}
	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
				"planner":  {Metadata: model.RoleMetadata{ID: "planner", Name: "Planner"}},
			},
		},
		executor,
	)

	run, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{"source": "manual"},
	})
	if err != nil {
		t.Fatalf("start workflow: %v", err)
	}
	if run.Status != model.WorkflowRunStatusPaused {
		t.Fatalf("workflow status = %s, want paused", run.Status)
	}
	if len(executor.calls) != 2 {
		t.Fatalf("len(executor.calls) = %d, want 2", len(executor.calls))
	}
	if run.Steps[1].Status != model.WorkflowStepRunStatusCompleted {
		t.Fatalf("approval step status = %s, want completed before pause", run.Steps[1].Status)
	}
	if got := run.Steps[1].Output["status"]; got != "awaiting_approval" {
		t.Fatalf("approval step output status = %v, want awaiting_approval", got)
	}
}

func TestWorkflowExecutionService_StartRejectsBlockingRolePluginDependency(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{}
	record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateEnabled, model.WorkflowProcessSequential, 0)
	pluginSvc := service.NewPluginService(pluginRepo, &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, t.TempDir())
	svc := service.NewWorkflowExecutionService(
		pluginSvc,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {
					Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"},
					Capabilities: model.RoleCapabilities{
						ToolConfig: model.RoleToolConfig{
							External: []string{"design-mcp"},
						},
					},
				},
				"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
			},
		},
		executor,
	)

	run, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{})
	if err == nil || !strings.Contains(err.Error(), "design-mcp") {
		t.Fatalf("Start() error = %v, want missing role plugin dependency failure", err)
	}
	if run != nil {
		t.Fatalf("run = %+v, want nil when role plugin dependency blocks startup", run)
	}
	if len(executor.calls) != 0 {
		t.Fatalf("executor calls = %d, want 0 when role plugin dependency blocks startup", len(executor.calls))
	}
}

func TestWorkflowExecutionService_StartAllowsBundledRolePluginDependency(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{}
	record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateEnabled, model.WorkflowProcessSequential, 0)
	pluginsDir := t.TempDir()
	writeManifest(t, pluginsDir, "tools/github-tool/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: github-tool
  name: GitHub Tool
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["github.js"]
`)
	writeBuiltInBundle(t, pluginsDir, `
plugins:
  - id: github-tool
    kind: ToolPlugin
    manifest: tools/github-tool/manifest.yaml
    docsRef: docs/PRD.md#tool-plugin
    verificationProfile: mcp-tool
    availability:
      status: ready
      message: GitHub tool ships with AgentForge.
`)
	pluginSvc := service.NewPluginService(pluginRepo, &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir)
	svc := service.NewWorkflowExecutionService(
		pluginSvc,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"coder": {
					Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"},
					Capabilities: model.RoleCapabilities{
						ToolConfig: model.RoleToolConfig{
							External: []string{"github-tool"},
						},
					},
				},
				"reviewer": {Metadata: model.RoleMetadata{ID: "reviewer", Name: "Reviewer"}},
			},
		},
		executor,
	)

	run, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{})
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if run == nil || run.Status != model.WorkflowRunStatusCompleted {
		t.Fatalf("run = %+v, want completed workflow run", run)
	}
	if len(executor.calls) != 2 {
		t.Fatalf("executor calls = %d, want 2", len(executor.calls))
	}
}

func TestWorkflowExecutionService_GetRunReturnsNotFound(t *testing.T) {
	svc := service.NewWorkflowExecutionService(
		repository.NewPluginRegistryRepository(),
		repository.NewWorkflowPluginRunRepository(),
		&fakePluginRoleStore{},
		&workflowStepExecutorMock{},
	)

	if _, err := svc.GetRun(context.Background(), uuid.New()); err == nil {
		t.Fatal("expected missing run lookup to fail")
	}
}

func TestWorkflowExecutionService_StartTaskTriggeredAddsTaskContextAndProfile(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{}
	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindWorkflow,
			Metadata: model.PluginMetadata{
				ID:      "task-delivery-flow",
				Name:    "Task Delivery Flow",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     "./dist/task-delivery-flow.wasm",
				ABIVersion: "v1",
				Workflow: &model.WorkflowPluginSpec{
					Process: model.WorkflowProcessSequential,
					Roles: []model.WorkflowRoleBinding{
						{ID: "planner"},
						{ID: "coder"},
					},
					Steps: []model.WorkflowStepDefinition{
						{ID: "plan", Role: "planner", Action: model.WorkflowActionAgent, Next: []string{"implement"}},
						{ID: "implement", Role: "coder", Action: model.WorkflowActionAgent},
					},
					Triggers: []model.PluginWorkflowTrigger{
						{Event: "manual"},
						{Event: "task.transition", Profile: "task-delivery", RequiresTask: true},
					},
				},
			},
		},
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostGoOrchestrator,
	}
	if err := pluginRepo.Save(context.Background(), record); err != nil {
		t.Fatalf("save workflow plugin record: %v", err)
	}

	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"planner": {Metadata: model.RoleMetadata{ID: "planner", Name: "Planner"}},
				"coder":   {Metadata: model.RoleMetadata{ID: "coder", Name: "Coder"}},
			},
		},
		executor,
	)

	taskID := uuid.New()
	run, err := svc.StartTaskTriggered(context.Background(), record.Metadata.ID, "task-delivery", taskID, service.WorkflowExecutionRequest{
		Trigger: map[string]any{
			"fromStatus": "triaged",
			"toStatus":   "assigned",
		},
	})
	if err != nil {
		t.Fatalf("StartTaskTriggered() error = %v", err)
	}
	if got := run.Trigger["taskId"]; got != taskID.String() {
		t.Fatalf("run trigger taskId = %v, want %s", got, taskID.String())
	}
	if got := run.Trigger["profile"]; got != "task-delivery" {
		t.Fatalf("run trigger profile = %v, want task-delivery", got)
	}
	if got := run.Trigger["source"]; got != "task.trigger" {
		t.Fatalf("run trigger source = %v, want task.trigger", got)
	}
}

func TestWorkflowExecutionService_StartTaskTriggeredRejectsDuplicateActiveRun(t *testing.T) {
	pluginRepo := repository.NewPluginRegistryRepository()
	runRepo := repository.NewWorkflowPluginRunRepository()
	executor := &workflowStepExecutorMock{}
	record := &model.PluginRecord{
		PluginManifest: model.PluginManifest{
			APIVersion: "agentforge/v1",
			Kind:       model.PluginKindWorkflow,
			Metadata: model.PluginMetadata{
				ID:      "task-delivery-flow",
				Name:    "Task Delivery Flow",
				Version: "1.0.0",
			},
			Spec: model.PluginSpec{
				Runtime:    model.PluginRuntimeWASM,
				Module:     "./dist/task-delivery-flow.wasm",
				ABIVersion: "v1",
				Workflow: &model.WorkflowPluginSpec{
					Process: model.WorkflowProcessSequential,
					Roles: []model.WorkflowRoleBinding{
						{ID: "planner"},
					},
					Steps: []model.WorkflowStepDefinition{
						{ID: "plan", Role: "planner", Action: model.WorkflowActionAgent},
					},
					Triggers: []model.PluginWorkflowTrigger{
						{Event: "task.transition", Profile: "task-delivery", RequiresTask: true},
					},
				},
			},
		},
		LifecycleState: model.PluginStateActive,
		RuntimeHost:    model.PluginHostGoOrchestrator,
	}
	if err := pluginRepo.Save(context.Background(), record); err != nil {
		t.Fatalf("save workflow plugin record: %v", err)
	}

	taskID := uuid.New()
	if err := runRepo.Create(context.Background(), &model.WorkflowPluginRun{
		ID:       uuid.New(),
		PluginID: record.Metadata.ID,
		Process:  model.WorkflowProcessSequential,
		Status:   model.WorkflowRunStatusPaused,
		Trigger: map[string]any{
			"source":  "task.trigger",
			"taskId":  taskID.String(),
			"profile": "task-delivery",
		},
	}); err != nil {
		t.Fatalf("seed duplicate workflow run: %v", err)
	}

	svc := service.NewWorkflowExecutionService(
		pluginRepo,
		runRepo,
		&fakePluginRoleStore{
			roles: map[string]*rolepkg.Manifest{
				"planner": {Metadata: model.RoleMetadata{ID: "planner", Name: "Planner"}},
			},
		},
		executor,
	)

	_, err := svc.StartTaskTriggered(context.Background(), record.Metadata.ID, "task-delivery", taskID, service.WorkflowExecutionRequest{})
	if err == nil {
		t.Fatal("expected duplicate active task-triggered run to be rejected")
	}
	if !strings.Contains(err.Error(), "active workflow run") {
		t.Fatalf("duplicate error = %v, want active workflow run hint", err)
	}
}
