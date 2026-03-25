package service_test

import (
	"context"
	"errors"
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
	return &service.WorkflowStepExecutionResult{
		Output: map[string]any{
			"step":    req.Step.ID,
			"attempt": req.Attempt,
		},
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

	t.Run("unsupported hierarchical workflow", func(t *testing.T) {
		pluginRepo := repository.NewPluginRegistryRepository()
		runRepo := repository.NewWorkflowPluginRunRepository()
		executor := &workflowStepExecutorMock{}
		record := saveWorkflowPluginRecord(t, pluginRepo, model.PluginStateEnabled, model.WorkflowProcessHierarchical, 0)
		svc := service.NewWorkflowExecutionService(pluginRepo, runRepo, roleStore, executor)

		if _, err := svc.Start(context.Background(), record.Metadata.ID, service.WorkflowExecutionRequest{}); err == nil {
			t.Fatal("expected hierarchical workflow start to fail")
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
