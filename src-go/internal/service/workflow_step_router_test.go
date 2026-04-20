package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type workflowAgentSpawnerMock struct {
	taskID     uuid.UUID
	memberID   uuid.UUID
	runtime    string
	provider   string
	modelName  string
	budgetUSD  float64
	roleID     string
	err        error
	employeeID *uuid.UUID // last acting-employee id observed on a call
}

func (m *workflowAgentSpawnerMock) Spawn(_ context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	m.taskID = taskID
	m.memberID = memberID
	m.runtime = runtime
	m.provider = provider
	m.modelName = modelName
	m.budgetUSD = budgetUsd
	m.roleID = roleID
	m.employeeID = nil
	if m.err != nil {
		return nil, m.err
	}
	return &model.AgentRun{
		ID:       uuid.New(),
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
		Runtime:  runtime,
		Provider: provider,
		Model:    modelName,
		RoleID:   roleID,
	}, nil
}

// SpawnWithEmployee satisfies WorkflowEmployeeAwareAgentSpawner. Captures the
// forwarded employee id on the mock so tests can assert propagation.
func (m *workflowAgentSpawnerMock) SpawnWithEmployee(_ context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string, employeeID *uuid.UUID) (*model.AgentRun, error) {
	m.taskID = taskID
	m.memberID = memberID
	m.runtime = runtime
	m.provider = provider
	m.modelName = modelName
	m.budgetUSD = budgetUsd
	m.roleID = roleID
	m.employeeID = employeeID
	if m.err != nil {
		return nil, m.err
	}
	run := &model.AgentRun{
		ID:       uuid.New(),
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
		Runtime:  runtime,
		Provider: provider,
		Model:    modelName,
		RoleID:   roleID,
	}
	if employeeID != nil {
		eid := *employeeID
		run.EmployeeID = &eid
	}
	return run, nil
}

type workflowReviewerMock struct {
	req *model.TriggerReviewRequest
	err error
}

func (m *workflowReviewerMock) Trigger(_ context.Context, req *model.TriggerReviewRequest) (*model.Review, error) {
	m.req = req
	if m.err != nil {
		return nil, m.err
	}
	return &model.Review{
		ID:       uuid.New(),
		TaskID:   uuid.MustParse(req.TaskID),
		PRURL:    req.PRURL,
		PRNumber: req.PRNumber,
		Status:   model.ReviewStatusInProgress,
	}, nil
}

type workflowDispatcherMock struct {
	input service.DispatchSpawnInput
	err   error
}

func (m *workflowDispatcherMock) Spawn(_ context.Context, input service.DispatchSpawnInput) (*model.TaskDispatchResponse, error) {
	m.input = input
	if m.err != nil {
		return nil, m.err
	}
	task := model.TaskDTO{ID: input.TaskID.String()}
	if input.MemberID != nil {
		assigneeID := input.MemberID.String()
		task.AssigneeID = &assigneeID
	}
	return &model.TaskDispatchResponse{
		Task: task,
		Dispatch: model.DispatchOutcome{
			Status: model.DispatchStatusStarted,
			Run: &model.AgentRunDTO{
				TaskID: input.TaskID.String(),
				RoleID: input.RoleID,
			},
		},
	}, nil
}

func TestWorkflowStepRouterExecutor_ExecuteAgentAction(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	spawner := &workflowAgentSpawnerMock{}
	executor := service.NewWorkflowStepRouterExecutor(spawner, nil, nil)

	result, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "workflow.release-train",
		Step: model.WorkflowStepDefinition{
			ID:     "implement",
			Role:   "coder",
			Action: model.WorkflowActionAgent,
		},
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":    taskID.String(),
				"memberId":  memberID.String(),
				"runtime":   "codex",
				"provider":  "openai",
				"model":     "gpt-5.4",
				"budgetUsd": 5.5,
			},
		},
	})
	if err != nil {
		t.Fatalf("execute agent action: %v", err)
	}
	if spawner.taskID != taskID || spawner.memberID != memberID {
		t.Fatalf("unexpected agent spawn ids: %+v", spawner)
	}
	if spawner.roleID != "coder" {
		t.Fatalf("roleID = %q, want coder", spawner.roleID)
	}
	if result.Output["status"] != string(model.AgentRunStatusRunning) {
		t.Fatalf("unexpected agent output: %+v", result.Output)
	}
}

func TestWorkflowStepRouterExecutor_ExecuteReviewAction(t *testing.T) {
	taskID := uuid.New()
	reviewer := &workflowReviewerMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, reviewer, nil)

	result, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "workflow.release-train",
		Step: model.WorkflowStepDefinition{
			ID:     "review",
			Role:   "reviewer",
			Action: model.WorkflowActionReview,
		},
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":   taskID.String(),
				"prUrl":    "https://github.com/example/repo/pull/42",
				"prNumber": 42.0,
				"diff":     "diff --git a/app.ts b/app.ts",
			},
		},
	})
	if err != nil {
		t.Fatalf("execute review action: %v", err)
	}
	if reviewer.req == nil || reviewer.req.TaskID != taskID.String() {
		t.Fatalf("unexpected review request: %+v", reviewer.req)
	}
	if result.Output["status"] != model.ReviewStatusInProgress {
		t.Fatalf("unexpected review output: %+v", result.Output)
	}
}

func TestWorkflowStepRouterExecutor_ExecuteTaskAction(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	dispatcher := &workflowDispatcherMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, nil, dispatcher)

	result, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "workflow.release-train",
		Step: model.WorkflowStepDefinition{
			ID:     "dispatch",
			Role:   "coder",
			Action: model.WorkflowActionTask,
		},
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":    taskID.String(),
				"memberId":  memberID.String(),
				"runtime":   "codex",
				"provider":  "openai",
				"model":     "gpt-5.4",
				"budgetUsd": 3.25,
			},
		},
	})
	if err != nil {
		t.Fatalf("execute task action: %v", err)
	}
	if dispatcher.input.TaskID != taskID {
		t.Fatalf("task dispatch id = %s, want %s", dispatcher.input.TaskID, taskID)
	}
	if dispatcher.input.MemberID == nil || *dispatcher.input.MemberID != memberID {
		t.Fatalf("unexpected dispatch member id: %+v", dispatcher.input.MemberID)
	}
	if dispatcher.input.RoleID != "coder" {
		t.Fatalf("dispatch role id = %q, want coder", dispatcher.input.RoleID)
	}
	if result.Output["status"] != string(model.DispatchStatusStarted) {
		t.Fatalf("unexpected task output: %+v", result.Output)
	}
}

func TestWorkflowStepRouterExecutor_RejectsMissingTriggerFields(t *testing.T) {
	executor := service.NewWorkflowStepRouterExecutor(&workflowAgentSpawnerMock{}, nil, nil)

	if _, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "workflow.release-train",
		Step: model.WorkflowStepDefinition{
			ID:     "implement",
			Role:   "coder",
			Action: model.WorkflowActionAgent,
		},
		Input: map[string]any{
			"trigger": map[string]any{},
		},
	}); err == nil {
		t.Fatal("expected missing trigger fields to fail")
	}
}

func TestWorkflowStepRouterExecutor_PropagatesUnderlyingErrors(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	executor := service.NewWorkflowStepRouterExecutor(&workflowAgentSpawnerMock{err: errors.New("spawn failed")}, nil, nil)

	if _, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "workflow.release-train",
		Step: model.WorkflowStepDefinition{
			ID:     "implement",
			Role:   "coder",
			Action: model.WorkflowActionAgent,
		},
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":   taskID.String(),
				"memberId": memberID.String(),
			},
		},
	}); err == nil {
		t.Fatal("expected agent spawn error to be returned")
	}
}

// ---------------------------------------------------------------------------
// Section 6.5 — acting-employee attribution tests for the legacy step router
// (change bridge-employee-attribution-legacy).
// ---------------------------------------------------------------------------

// Agent action: step-level employeeId override wins over the run-level default.
func TestWorkflowStepRouterExecutor_AgentAction_StepLevelEmployeeOverride(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	runEmp := uuid.New()
	stepEmp := uuid.New()

	spawner := &workflowAgentSpawnerMock{}
	executor := service.NewWorkflowStepRouterExecutor(spawner, nil, nil)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:               uuid.New(),
		PluginID:            "wf.x",
		Step:                model.WorkflowStepDefinition{ID: "s1", Role: "coder", Action: model.WorkflowActionAgent},
		RunActingEmployeeID: &runEmp,
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":     taskID.String(),
				"memberId":   memberID.String(),
				"employeeId": stepEmp.String(),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spawner.employeeID == nil || *spawner.employeeID != stepEmp {
		t.Errorf("expected step-level employee %s, got %v", stepEmp, spawner.employeeID)
	}
}

// Agent action: falls back to run-level acting employee when step omits.
func TestWorkflowStepRouterExecutor_AgentAction_RunLevelFallback(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	runEmp := uuid.New()

	spawner := &workflowAgentSpawnerMock{}
	executor := service.NewWorkflowStepRouterExecutor(spawner, nil, nil)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:               uuid.New(),
		PluginID:            "wf.x",
		Step:                model.WorkflowStepDefinition{ID: "s1", Role: "coder", Action: model.WorkflowActionAgent},
		RunActingEmployeeID: &runEmp,
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":   taskID.String(),
				"memberId": memberID.String(),
				// no employeeId
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spawner.employeeID == nil || *spawner.employeeID != runEmp {
		t.Errorf("expected run-level employee %s, got %v", runEmp, spawner.employeeID)
	}
}

// Agent action: both absent preserves existing behavior — legacy Spawn() is
// called (not SpawnWithEmployee), no employee attribution applied.
func TestWorkflowStepRouterExecutor_AgentAction_BothAbsentNoAttribution(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()

	spawner := &workflowAgentSpawnerMock{}
	executor := service.NewWorkflowStepRouterExecutor(spawner, nil, nil)

	result, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "wf.x",
		Step:     model.WorkflowStepDefinition{ID: "s1", Role: "coder", Action: model.WorkflowActionAgent},
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":   taskID.String(),
				"memberId": memberID.String(),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spawner.employeeID != nil {
		t.Errorf("expected no employee attribution, got %v", spawner.employeeID)
	}
	if _, present := result.Output["employeeId"]; present {
		t.Errorf("expected employeeId to be absent from output, got %+v", result.Output)
	}
}

// Review action: step-level override flows into TriggerReviewRequest.
func TestWorkflowStepRouterExecutor_ReviewAction_StepLevelEmployeeOverride(t *testing.T) {
	taskID := uuid.New()
	runEmp := uuid.New()
	stepEmp := uuid.New()
	reviewer := &workflowReviewerMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, reviewer, nil)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:               uuid.New(),
		PluginID:            "wf.x",
		Step:                model.WorkflowStepDefinition{ID: "r1", Role: "reviewer", Action: model.WorkflowActionReview},
		RunActingEmployeeID: &runEmp,
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":     taskID.String(),
				"prUrl":      "https://github.com/example/repo/pull/1",
				"employeeId": stepEmp.String(),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reviewer.req == nil || reviewer.req.ActingEmployeeID != stepEmp.String() {
		t.Errorf("expected TriggerReviewRequest.ActingEmployeeID=%s, got %v",
			stepEmp, reviewer.req)
	}
}

// Review action: run-level fallback into TriggerReviewRequest.
func TestWorkflowStepRouterExecutor_ReviewAction_RunLevelFallback(t *testing.T) {
	taskID := uuid.New()
	runEmp := uuid.New()
	reviewer := &workflowReviewerMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, reviewer, nil)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:               uuid.New(),
		PluginID:            "wf.x",
		Step:                model.WorkflowStepDefinition{ID: "r1", Role: "reviewer", Action: model.WorkflowActionReview},
		RunActingEmployeeID: &runEmp,
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId": taskID.String(),
				"prUrl":  "https://github.com/example/repo/pull/1",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reviewer.req == nil || reviewer.req.ActingEmployeeID != runEmp.String() {
		t.Errorf("expected TriggerReviewRequest.ActingEmployeeID=%s, got %v",
			runEmp, reviewer.req)
	}
}

// Review action: both absent preserves existing behavior — no ActingEmployeeID on review request.
func TestWorkflowStepRouterExecutor_ReviewAction_BothAbsentPreservesBehavior(t *testing.T) {
	taskID := uuid.New()
	reviewer := &workflowReviewerMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, reviewer, nil)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "wf.x",
		Step:     model.WorkflowStepDefinition{ID: "r1", Role: "reviewer", Action: model.WorkflowActionReview},
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId": taskID.String(),
				"prUrl":  "https://github.com/example/repo/pull/1",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reviewer.req == nil || reviewer.req.ActingEmployeeID != "" {
		t.Errorf("expected empty ActingEmployeeID, got %q", reviewer.req.ActingEmployeeID)
	}
}

// Task action: step-level override flows into DispatchSpawnInput.
func TestWorkflowStepRouterExecutor_TaskAction_StepLevelEmployeeOverride(t *testing.T) {
	taskID := uuid.New()
	runEmp := uuid.New()
	stepEmp := uuid.New()
	dispatcher := &workflowDispatcherMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, nil, dispatcher)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:               uuid.New(),
		PluginID:            "wf.x",
		Step:                model.WorkflowStepDefinition{ID: "t1", Role: "coder", Action: model.WorkflowActionTask},
		RunActingEmployeeID: &runEmp,
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId":     taskID.String(),
				"employeeId": stepEmp.String(),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dispatcher.input.EmployeeID == nil || *dispatcher.input.EmployeeID != stepEmp {
		t.Errorf("expected DispatchSpawnInput.EmployeeID=%s, got %v",
			stepEmp, dispatcher.input.EmployeeID)
	}
}

// Task action: run-level fallback flows into DispatchSpawnInput.
func TestWorkflowStepRouterExecutor_TaskAction_RunLevelFallback(t *testing.T) {
	taskID := uuid.New()
	runEmp := uuid.New()
	dispatcher := &workflowDispatcherMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, nil, dispatcher)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:               uuid.New(),
		PluginID:            "wf.x",
		Step:                model.WorkflowStepDefinition{ID: "t1", Role: "coder", Action: model.WorkflowActionTask},
		RunActingEmployeeID: &runEmp,
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId": taskID.String(),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dispatcher.input.EmployeeID == nil || *dispatcher.input.EmployeeID != runEmp {
		t.Errorf("expected DispatchSpawnInput.EmployeeID=%s, got %v",
			runEmp, dispatcher.input.EmployeeID)
	}
}

// Task action: both absent preserves existing behavior — EmployeeID is nil.
func TestWorkflowStepRouterExecutor_TaskAction_BothAbsentNoAttribution(t *testing.T) {
	taskID := uuid.New()
	dispatcher := &workflowDispatcherMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, nil, dispatcher)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "wf.x",
		Step:     model.WorkflowStepDefinition{ID: "t1", Role: "coder", Action: model.WorkflowActionTask},
		Input: map[string]any{
			"trigger": map[string]any{
				"taskId": taskID.String(),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dispatcher.input.EmployeeID != nil {
		t.Errorf("expected EmployeeID nil, got %v", dispatcher.input.EmployeeID)
	}
}

// ---------------------------------------------------------------------------
// Section 2.6 — workflow action cross-engine dispatch tests (change
// bridge-legacy-to-dag-invocation).
// ---------------------------------------------------------------------------

// workflowChildSpawnerMock satisfies WorkflowChildSpawner for the plugin
// default path, recording the plugin id passed on dispatch.
type workflowChildSpawnerMock struct {
	startedPluginID string
}

func (m *workflowChildSpawnerMock) Start(_ context.Context, pluginID string, _ service.WorkflowExecutionRequest) (*model.WorkflowPluginRun, error) {
	m.startedPluginID = pluginID
	return &model.WorkflowPluginRun{
		ID:       uuid.New(),
		PluginID: pluginID,
		Status:   model.WorkflowRunStatusRunning,
	}, nil
}

// dagChildStarterMock satisfies WorkflowDAGChildStarter for DAG-target tests.
type dagChildStarterMock struct {
	startedTarget uuid.UUID
	startedInv    service.WorkflowDAGChildInvocation
	childRunID    uuid.UUID
	cancelled     []uuid.UUID
}

func (m *dagChildStarterMock) Start(_ context.Context, target uuid.UUID, _ map[string]any, inv service.WorkflowDAGChildInvocation) (uuid.UUID, error) {
	m.startedTarget = target
	m.startedInv = inv
	if m.childRunID == uuid.Nil {
		m.childRunID = uuid.New()
	}
	return m.childRunID, nil
}

func (m *dagChildStarterMock) Cancel(_ context.Context, runID uuid.UUID) error {
	m.cancelled = append(m.cancelled, runID)
	return nil
}

type parentLinkWriterMock struct {
	links []*model.WorkflowRunParentLink
}

func (m *parentLinkWriterMock) Create(_ context.Context, link *model.WorkflowRunParentLink) error {
	m.links = append(m.links, link)
	return nil
}

// cycleGuardMock satisfies WorkflowCrossEngineRecursionGuard.
type cycleGuardMock struct {
	reject error
}

func (m *cycleGuardMock) CheckFromEngine(_ context.Context, _ string, _ uuid.UUID, _ string) error {
	return m.reject
}

// Workflow action defaults to the legacy plugin-child path when no targetKind
// is declared; output envelope includes child_engine="plugin".
func TestWorkflowStepRouterExecutor_WorkflowAction_PluginDefault(t *testing.T) {
	spawner := &workflowChildSpawnerMock{}
	executor := service.NewWorkflowStepRouterExecutor(nil, nil, nil).
		WithWorkflowSpawner(spawner)

	result, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "wf.parent",
		Step:     model.WorkflowStepDefinition{ID: "nested", Role: "coder", Action: model.WorkflowActionWorkflow},
		Input: map[string]any{
			"trigger": map[string]any{
				"pluginId": "wf.child",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spawner.startedPluginID != "wf.child" {
		t.Errorf("plugin spawner started %q, want wf.child", spawner.startedPluginID)
	}
	if got, _ := result.Output["child_engine"].(string); got != "plugin" {
		t.Errorf("child_engine = %v, want plugin", result.Output["child_engine"])
	}
	if _, ok := result.Output["child_plugin"]; !ok {
		t.Errorf("plugin-default output envelope missing child_plugin: %+v", result.Output)
	}
}

// Workflow action with explicit targetKind='dag' dispatches through the DAG
// starter, persists a parent-link row with parent_kind='plugin_run', and
// returns status=awaiting_sub_workflow.
func TestWorkflowStepRouterExecutor_WorkflowAction_DAGExplicit(t *testing.T) {
	target := uuid.New()
	parentRun := uuid.New()
	project := uuid.New()
	dagStarter := &dagChildStarterMock{}
	linkWriter := &parentLinkWriterMock{}

	executor := service.NewWorkflowStepRouterExecutor(nil, nil, nil).
		WithDAGChildStarter(dagStarter, linkWriter)

	result, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:        parentRun,
		PluginID:     "wf.parent",
		Step:         model.WorkflowStepDefinition{ID: "nested", Role: "coder", Action: model.WorkflowActionWorkflow},
		RunProjectID: project,
		Input: map[string]any{
			"trigger": map[string]any{
				"targetKind":       "dag",
				"targetWorkflowId": target.String(),
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dagStarter.startedTarget != target {
		t.Errorf("dag starter target = %s, want %s", dagStarter.startedTarget, target)
	}
	if dagStarter.startedInv.ProjectID != project {
		t.Errorf("dag starter project = %s, want %s", dagStarter.startedInv.ProjectID, project)
	}
	if dagStarter.startedInv.ParentRunID != parentRun {
		t.Errorf("dag starter parent run = %s, want %s", dagStarter.startedInv.ParentRunID, parentRun)
	}
	if len(linkWriter.links) != 1 {
		t.Fatalf("expected 1 parent link row, got %d", len(linkWriter.links))
	}
	link := linkWriter.links[0]
	if link.ParentKind != model.SubWorkflowParentKindPluginRun {
		t.Errorf("link parent_kind = %q, want plugin_run", link.ParentKind)
	}
	if link.ChildEngineKind != model.SubWorkflowEngineDAG {
		t.Errorf("link child_engine_kind = %q, want dag", link.ChildEngineKind)
	}
	if link.ParentExecutionID != parentRun {
		t.Errorf("link parent_execution_id = %s, want %s", link.ParentExecutionID, parentRun)
	}
	if got, _ := result.Output["status"].(string); got != string(model.WorkflowStepRunStatusAwaitingSubWorkflow) {
		t.Errorf("output status = %v, want awaiting_sub_workflow", result.Output["status"])
	}
	if got, _ := result.Output["child_engine"].(string); got != "dag" {
		t.Errorf("output child_engine = %v, want dag", result.Output["child_engine"])
	}
	if got, _ := result.Output["child_workflow_id"].(string); got != target.String() {
		t.Errorf("output child_workflow_id = %v, want %s", result.Output["child_workflow_id"], target)
	}
}

// Unknown targetKind is rejected before any child dispatch.
func TestWorkflowStepRouterExecutor_WorkflowAction_UnknownTargetKind(t *testing.T) {
	executor := service.NewWorkflowStepRouterExecutor(nil, nil, nil)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "wf.parent",
		Step:     model.WorkflowStepDefinition{ID: "nested", Role: "coder", Action: model.WorkflowActionWorkflow},
		Input: map[string]any{
			"trigger": map[string]any{
				"targetKind": "nonsense",
			},
		},
	})
	if err == nil {
		t.Fatal("expected unknown targetKind to fail")
	}
}

// Cycle guard rejection propagates as a structured error; no dispatch occurs.
func TestWorkflowStepRouterExecutor_WorkflowAction_DAGRecursionRejected(t *testing.T) {
	target := uuid.New()
	dagStarter := &dagChildStarterMock{}
	linkWriter := &parentLinkWriterMock{}
	guard := &cycleGuardMock{reject: errors.New("sub_workflow: cycle: target forms a cycle at depth 1")}

	executor := service.NewWorkflowStepRouterExecutor(nil, nil, nil).
		WithDAGChildStarter(dagStarter, linkWriter).
		WithCrossEngineRecursionGuard(guard)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "wf.parent",
		Step:     model.WorkflowStepDefinition{ID: "nested", Role: "coder", Action: model.WorkflowActionWorkflow},
		Input: map[string]any{
			"trigger": map[string]any{
				"targetKind":       "dag",
				"targetWorkflowId": target.String(),
			},
		},
	})
	if err == nil {
		t.Fatal("expected cycle rejection to fail dispatch")
	}
	if dagStarter.startedTarget != uuid.Nil {
		t.Errorf("dag starter should not have been called on cycle rejection")
	}
	if len(linkWriter.links) != 0 {
		t.Errorf("link writer should not persist on cycle rejection")
	}
}

// When the DAG starter is not configured, the router reports the config error
// rather than silently falling through to the plugin path.
func TestWorkflowStepRouterExecutor_WorkflowAction_DAGStarterMissing(t *testing.T) {
	target := uuid.New()
	executor := service.NewWorkflowStepRouterExecutor(nil, nil, nil)

	_, err := executor.Execute(context.Background(), service.WorkflowStepExecutionRequest{
		RunID:    uuid.New(),
		PluginID: "wf.parent",
		Step:     model.WorkflowStepDefinition{ID: "nested", Role: "coder", Action: model.WorkflowActionWorkflow},
		Input: map[string]any{
			"trigger": map[string]any{
				"targetKind":       "dag",
				"targetWorkflowId": target.String(),
			},
		},
	})
	if err == nil {
		t.Fatal("expected missing DAG starter to fail")
	}
}
