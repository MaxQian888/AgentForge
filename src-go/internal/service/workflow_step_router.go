package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/plugin"
	"github.com/google/uuid"
)

// Target-kind discriminator values accepted on the trigger payload (or step
// config) of a `workflow` action. Used by executeWorkflow to select between
// the legacy plugin-child spawn path and the DAG cross-engine dispatch path
// introduced by bridge-legacy-to-dag-invocation.
const (
	WorkflowStepTargetKindPlugin = "plugin"
	WorkflowStepTargetKindDAG    = "dag"
)

type WorkflowAgentSpawner interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
}

// WorkflowEmployeeAwareAgentSpawner is the attribution-aware extension to
// WorkflowAgentSpawner. When the step router has resolved an effective
// employee id (step-level override or run-level default) and the injected
// spawner implements this interface, the router calls SpawnWithEmployee so
// the spawned AgentRun carries employee_id. Legacy spawners that do not
// implement it fall back to Spawn with the acting_employee_id dropped.
//
// Added in change bridge-employee-attribution-legacy.
type WorkflowEmployeeAwareAgentSpawner interface {
	WorkflowAgentSpawner
	SpawnWithEmployee(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string, employeeID *uuid.UUID) (*model.AgentRun, error)
}

type WorkflowReviewRunner interface {
	Trigger(ctx context.Context, req *model.TriggerReviewRequest) (*model.Review, error)
}

type WorkflowTaskDispatcher interface {
	Spawn(ctx context.Context, input DispatchSpawnInput) (*model.TaskDispatchResponse, error)
}

// WorkflowChildSpawner starts a child workflow execution. Typically backed by
// WorkflowExecutionService.Start so that hierarchical workflows can recurse.
type WorkflowChildSpawner interface {
	Start(ctx context.Context, pluginID string, req WorkflowExecutionRequest) (*model.WorkflowPluginRun, error)
}

// WorkflowDAGChildInvocation carries the parent-side provenance the DAG child
// starter needs to persist attribution onto the new execution and the
// parent↔child linkage row. Introduced by bridge-legacy-to-dag-invocation so
// a plugin run's `workflow` step can cross engines without the router having
// to import the DAG workflow service.
type WorkflowDAGChildInvocation struct {
	ProjectID        uuid.UUID
	ActingEmployeeID *uuid.UUID
	ParentRunID      uuid.UUID
	ParentStepID     string
}

// WorkflowDAGChildStarter launches a DAG child run from the legacy step router
// and cancels it on parent-cancel cascade. Backed at wiring time by a thin
// adapter around *DAGWorkflowService; tests inject a stub so the router stays
// decoupled from DAG-specific plumbing.
type WorkflowDAGChildStarter interface {
	Start(ctx context.Context, targetWorkflowID uuid.UUID, seed map[string]any, inv WorkflowDAGChildInvocation) (uuid.UUID, error)
	Cancel(ctx context.Context, runID uuid.UUID) error
}

// WorkflowParentLinkWriter persists a parent↔child linkage row when a plugin
// run invokes a DAG child. Satisfied by *repository.WorkflowRunParentLinkRepository.
// Kept narrow so the router has no knowledge of the repo surface beyond
// insertion of a single row with parent_kind='plugin_run'.
type WorkflowParentLinkWriter interface {
	Create(ctx context.Context, link *model.WorkflowRunParentLink) error
}

// WorkflowCrossEngineRecursionGuard rejects plugin → DAG invocations that
// would form a cycle across engine boundaries (plugin invokes a DAG whose
// ancestor chain already contains the same DAG). Satisfied by
// *nodetypes.RecursionGuard via its CheckFromEngine method.
type WorkflowCrossEngineRecursionGuard interface {
	CheckFromEngine(ctx context.Context, startKind string, startRunID uuid.UUID, targetWorkflowID string) error
}

type WorkflowStepRouterExecutor struct {
	agents     WorkflowAgentSpawner
	reviews    WorkflowReviewRunner
	tasks      WorkflowTaskDispatcher
	workflows  WorkflowChildSpawner
	dagChild   WorkflowDAGChildStarter
	linkWriter WorkflowParentLinkWriter
	cycleGuard WorkflowCrossEngineRecursionGuard
	// toolChain dispatches the WorkflowActionToolChain action. When nil,
	// the router rejects tool_chain steps with a clear error so a missing
	// wiring fails loudly rather than silently dropping the call.
	toolChain *plugin.ToolChainExecutor
}

func NewWorkflowStepRouterExecutor(
	agents WorkflowAgentSpawner,
	reviews WorkflowReviewRunner,
	tasks WorkflowTaskDispatcher,
) *WorkflowStepRouterExecutor {
	return &WorkflowStepRouterExecutor{
		agents:  agents,
		reviews: reviews,
		tasks:   tasks,
	}
}

// WithWorkflowSpawner attaches a child-workflow spawner used by the "workflow"
// action type in hierarchical process mode.
func (e *WorkflowStepRouterExecutor) WithWorkflowSpawner(spawner WorkflowChildSpawner) *WorkflowStepRouterExecutor {
	e.workflows = spawner
	return e
}

// WithDAGChildStarter wires the cross-engine seam the router uses when a
// `workflow` step's trigger input declares targetKind='dag'. The optional
// linkWriter persists the parent↔child link row so the DAG engine's terminal
// hook can resume the parked plugin step (bridge-legacy-to-dag-invocation).
func (e *WorkflowStepRouterExecutor) WithDAGChildStarter(starter WorkflowDAGChildStarter, linkWriter WorkflowParentLinkWriter) *WorkflowStepRouterExecutor {
	e.dagChild = starter
	e.linkWriter = linkWriter
	return e
}

// WithCrossEngineRecursionGuard wires the shared cross-engine recursion guard
// used to reject plugin → DAG invocations that would create a cycle. Nil is
// valid (guard disabled), which matches test wiring that doesn't exercise the
// ancestor chain.
func (e *WorkflowStepRouterExecutor) WithCrossEngineRecursionGuard(guard WorkflowCrossEngineRecursionGuard) *WorkflowStepRouterExecutor {
	e.cycleGuard = guard
	return e
}

// WithToolChainExecutor wires the executor used by the tool_chain action.
// Nil is valid (action disabled); steps that declare action=tool_chain
// will then fail with a clear error.
func (e *WorkflowStepRouterExecutor) WithToolChainExecutor(executor *plugin.ToolChainExecutor) *WorkflowStepRouterExecutor {
	e.toolChain = executor
	return e
}

// DAGChildStarter returns the cross-engine starter wired into the router, if
// any. Used by the workflow execution service's cancellation path to cascade
// a cancel to an in-flight DAG child when the parent plugin run is cancelled.
func (e *WorkflowStepRouterExecutor) DAGChildStarter() WorkflowDAGChildStarter {
	return e.dagChild
}

func (e *WorkflowStepRouterExecutor) Execute(ctx context.Context, req WorkflowStepExecutionRequest) (*WorkflowStepExecutionResult, error) {
	trigger, err := workflowInputMap(req.Input, "trigger")
	if err != nil {
		return nil, err
	}

	switch req.Step.Action {
	case model.WorkflowActionAgent:
		return e.executeAgent(ctx, req, trigger)
	case model.WorkflowActionReview:
		return e.executeReview(ctx, req, trigger)
	case model.WorkflowActionTask:
		return e.executeTask(ctx, req, trigger)
	case model.WorkflowActionWorkflow:
		return e.executeWorkflow(ctx, req, trigger)
	case model.WorkflowActionApproval:
		return e.executeApproval(ctx, req)
	case model.WorkflowActionToolChain:
		return e.executeToolChain(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported workflow action: %s", req.Step.Action)
	}
}

// executeToolChain dispatches the tool_chain action. The step's ToolChain
// spec is required; the resolver consumes the run's input map for
// {{workflow.input.*}} expansion. Outputs of the chain land under
// "completed_steps", "final_output", and "step_outputs" so downstream
// steps in the surrounding workflow can read them.
func (e *WorkflowStepRouterExecutor) executeToolChain(ctx context.Context, req WorkflowStepExecutionRequest) (*WorkflowStepExecutionResult, error) {
	if e.toolChain == nil {
		return nil, fmt.Errorf("tool_chain executor is not configured")
	}
	if req.Step.ToolChain == nil {
		return nil, fmt.Errorf("step %q has action tool_chain but no tool_chain spec", req.Step.ID)
	}
	chainResult, err := e.toolChain.Execute(ctx, req.PluginID, req.Step.ToolChain, req.Input)
	if err != nil {
		return nil, err
	}
	output := map[string]any{
		"completed_steps": chainResult.CompletedSteps,
		"final_output":    chainResult.FinalOutput,
		"step_outputs":    chainResult.StepOutputs,
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
}

func (e *WorkflowStepRouterExecutor) executeAgent(ctx context.Context, req WorkflowStepExecutionRequest, trigger map[string]any) (*WorkflowStepExecutionResult, error) {
	if e.agents == nil {
		return nil, fmt.Errorf("workflow agent executor is not configured")
	}
	taskID, err := workflowUUID(trigger, "taskId")
	if err != nil {
		return nil, err
	}
	memberID, err := workflowUUID(trigger, "memberId")
	if err != nil {
		return nil, err
	}
	// Resolve the effective acting employee: step-level override wins, then
	// run-level default (change bridge-employee-attribution-legacy).
	employeeID := resolveStepActingEmployee(trigger, req.RunActingEmployeeID)

	var run *model.AgentRun
	if employeeID != nil {
		if aware, ok := e.agents.(WorkflowEmployeeAwareAgentSpawner); ok {
			run, err = aware.SpawnWithEmployee(
				ctx,
				taskID,
				memberID,
				workflowString(trigger, "runtime"),
				workflowString(trigger, "provider"),
				workflowString(trigger, "model"),
				workflowFloat(trigger, "budgetUsd"),
				req.Step.Role,
				employeeID,
			)
		} else {
			run, err = e.agents.Spawn(
				ctx,
				taskID,
				memberID,
				workflowString(trigger, "runtime"),
				workflowString(trigger, "provider"),
				workflowString(trigger, "model"),
				workflowFloat(trigger, "budgetUsd"),
				req.Step.Role,
			)
		}
	} else {
		run, err = e.agents.Spawn(
			ctx,
			taskID,
			memberID,
			workflowString(trigger, "runtime"),
			workflowString(trigger, "provider"),
			workflowString(trigger, "model"),
			workflowFloat(trigger, "budgetUsd"),
			req.Step.Role,
		)
	}
	if err != nil {
		return nil, err
	}
	output := map[string]any{
		"runId":    run.ID.String(),
		"taskId":   run.TaskID.String(),
		"memberId": run.MemberID.String(),
		"roleId":   run.RoleID,
		"status":   run.Status,
		"runtime":  run.Runtime,
		"provider": run.Provider,
		"model":    run.Model,
	}
	if run.EmployeeID != nil {
		output["employeeId"] = run.EmployeeID.String()
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
}

func (e *WorkflowStepRouterExecutor) executeReview(ctx context.Context, req WorkflowStepExecutionRequest, trigger map[string]any) (*WorkflowStepExecutionResult, error) {
	if e.reviews == nil {
		return nil, fmt.Errorf("workflow review executor is not configured")
	}
	reviewReq := &model.TriggerReviewRequest{
		TaskID:     workflowString(trigger, "taskId"),
		PRURL:      workflowString(trigger, "prUrl"),
		PRNumber:   workflowInt(trigger, "prNumber"),
		Trigger:    "manual",
		Dimensions: workflowStringSlice(trigger, "dimensions"),
		Diff:       workflowString(trigger, "diff"),
	}
	// Attribute the review to an acting employee: step-level override wins,
	// then run-level default (change bridge-employee-attribution-legacy).
	if empID := resolveStepActingEmployee(trigger, req.RunActingEmployeeID); empID != nil {
		reviewReq.ActingEmployeeID = empID.String()
	}
	if reviewReq.TaskID == "" && reviewReq.PRURL == "" {
		return nil, fmt.Errorf("workflow review step requires trigger.taskId or trigger.prUrl")
	}
	review, err := e.reviews.Trigger(ctx, reviewReq)
	if err != nil {
		return nil, err
	}
	output := map[string]any{
		"reviewId": review.ID.String(),
		"taskId":   review.TaskID.String(),
		"prUrl":    review.PRURL,
		"prNumber": review.PRNumber,
		"status":   review.Status,
	}
	if reviewReq.ActingEmployeeID != "" {
		output["employeeId"] = reviewReq.ActingEmployeeID
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
}

func (e *WorkflowStepRouterExecutor) executeTask(ctx context.Context, req WorkflowStepExecutionRequest, trigger map[string]any) (*WorkflowStepExecutionResult, error) {
	if e.tasks == nil {
		return nil, fmt.Errorf("workflow task executor is not configured")
	}
	taskID, err := workflowUUID(trigger, "taskId")
	if err != nil {
		return nil, err
	}
	dispatchInput := DispatchSpawnInput{
		TaskID:        taskID,
		Runtime:       workflowString(trigger, "runtime"),
		Provider:      workflowString(trigger, "provider"),
		Model:         workflowString(trigger, "model"),
		Priority:      workflowInt(trigger, "priority"),
		BudgetUSD:     workflowFloat(trigger, "budgetUsd"),
		RoleID:        req.Step.Role,
		TriggerSource: "workflow",
	}
	if rawMemberID := workflowString(trigger, "memberId"); rawMemberID != "" {
		memberID, err := uuid.Parse(rawMemberID)
		if err != nil {
			return nil, fmt.Errorf("trigger.memberId must be a valid UUID: %w", err)
		}
		dispatchInput.MemberID = &memberID
	}
	// Forward acting employee attribution so the dispatched task's spawned
	// runs receive the attribution (change bridge-employee-attribution-legacy).
	if empID := resolveStepActingEmployee(trigger, req.RunActingEmployeeID); empID != nil {
		dispatchInput.EmployeeID = empID
	}
	result, err := e.tasks.Spawn(ctx, dispatchInput)
	if err != nil {
		return nil, err
	}
	output := map[string]any{
		"taskId": taskID.String(),
		"status": result.Dispatch.Status,
		"reason": result.Dispatch.Reason,
	}
	if result.Dispatch.Run != nil {
		output["runId"] = result.Dispatch.Run.ID
	}
	if dispatchInput.EmployeeID != nil {
		output["employeeId"] = dispatchInput.EmployeeID.String()
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
}

func (e *WorkflowStepRouterExecutor) executeWorkflow(ctx context.Context, req WorkflowStepExecutionRequest, trigger map[string]any) (*WorkflowStepExecutionResult, error) {
	targetKind := resolveWorkflowStepTargetKind(trigger, req.Step.Config)
	switch targetKind {
	case WorkflowStepTargetKindPlugin, "":
		return e.executeWorkflowPluginChild(ctx, req, trigger)
	case WorkflowStepTargetKindDAG:
		return e.executeWorkflowDAGChild(ctx, req, trigger)
	default:
		return nil, fmt.Errorf("workflow step: unknown targetKind %q (expected %q or %q)", targetKind, WorkflowStepTargetKindPlugin, WorkflowStepTargetKindDAG)
	}
}

// executeWorkflowPluginChild preserves the legacy synchronous child-plugin
// path exactly as it was before bridge-legacy-to-dag-invocation — resolving
// `trigger.pluginId` / step config `plugin_id` and starting the child plugin
// run through WorkflowChildSpawner. The output envelope now also advertises
// `child_engine='plugin'` so downstream consumers can disambiguate between
// engines without inspecting `child_plugin`'s presence.
func (e *WorkflowStepRouterExecutor) executeWorkflowPluginChild(ctx context.Context, req WorkflowStepExecutionRequest, trigger map[string]any) (*WorkflowStepExecutionResult, error) {
	if e.workflows == nil {
		return nil, fmt.Errorf("workflow child spawner is not configured")
	}
	childPluginID := workflowString(trigger, "pluginId")
	if childPluginID == "" {
		// Fall back to step config if present.
		if req.Step.Config != nil {
			childPluginID = workflowString(req.Step.Config, "plugin_id")
		}
	}
	if childPluginID == "" {
		return nil, fmt.Errorf("workflow step requires trigger.pluginId or step config plugin_id for child workflow")
	}
	childRun, err := e.workflows.Start(ctx, childPluginID, WorkflowExecutionRequest{
		Trigger: cloneWorkflowPayload(trigger),
	})
	if err != nil {
		return nil, fmt.Errorf("child workflow %s failed: %w", childPluginID, err)
	}
	output := map[string]any{
		"child_run_id": childRun.ID.String(),
		"child_plugin": childRun.PluginID,
		"child_engine": WorkflowStepTargetKindPlugin,
		"status":       string(childRun.Status),
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
}

// executeWorkflowDAGChild dispatches a DAG child run for a legacy plugin
// workflow step. The parent step is parked with status
// `awaiting_sub_workflow` so the execution service pauses the plugin run; the
// DAG engine's terminal-state hook later resumes the parent via the
// plugin runtime's parked-step resume path. A parent↔child linkage row with
// parent_kind='plugin_run' is persisted so the resume hook can route.
func (e *WorkflowStepRouterExecutor) executeWorkflowDAGChild(ctx context.Context, req WorkflowStepExecutionRequest, trigger map[string]any) (*WorkflowStepExecutionResult, error) {
	if e.dagChild == nil {
		return nil, fmt.Errorf("workflow step: dag child starter is not configured")
	}
	rawTarget := workflowString(trigger, "targetWorkflowId")
	if rawTarget == "" && req.Step.Config != nil {
		rawTarget = workflowString(req.Step.Config, "target_workflow_id")
		if rawTarget == "" {
			rawTarget = workflowString(req.Step.Config, "targetWorkflowId")
		}
	}
	rawTarget = strings.TrimSpace(rawTarget)
	if rawTarget == "" {
		return nil, fmt.Errorf("workflow step: targetKind=dag requires trigger.targetWorkflowId or step config target_workflow_id")
	}
	targetID, err := uuid.Parse(rawTarget)
	if err != nil {
		return nil, fmt.Errorf("workflow step: targetWorkflowId %q is not a valid UUID: %w", rawTarget, err)
	}

	// Cross-engine cycle check: reject plugin → DAG invocations that would
	// reintroduce a DAG workflow already in the ancestor chain. The guard
	// walks through both engines using the parent_kind discriminator.
	if e.cycleGuard != nil {
		if err := e.cycleGuard.CheckFromEngine(ctx, "plugin", req.RunID, targetID.String()); err != nil {
			return nil, fmt.Errorf("workflow step: dag child rejected: %w", err)
		}
	}

	inv := WorkflowDAGChildInvocation{
		ProjectID:        req.RunProjectID,
		ActingEmployeeID: resolveStepActingEmployee(trigger, req.RunActingEmployeeID),
		ParentRunID:      req.RunID,
		ParentStepID:     req.Step.ID,
	}
	childRunID, err := e.dagChild.Start(ctx, targetID, cloneWorkflowPayload(trigger), inv)
	if err != nil {
		return nil, fmt.Errorf("workflow step: start dag child %s: %w", targetID, err)
	}

	// Persist the parent↔child linkage with parent_kind='plugin_run' so the
	// DAG engine's terminal-state hook routes the resume back through the
	// plugin runtime. Soft-fail: if persistence fails, the parent can't
	// auto-resume; operators can reconcile manually.
	if e.linkWriter != nil {
		if linkErr := e.linkWriter.Create(ctx, &model.WorkflowRunParentLink{
			ID:                uuid.New(),
			ParentExecutionID: req.RunID,
			ParentKind:        model.SubWorkflowParentKindPluginRun,
			ParentNodeID:      req.Step.ID,
			ChildEngineKind:   model.SubWorkflowEngineDAG,
			ChildRunID:        childRunID,
			Status:            model.SubWorkflowLinkStatusRunning,
		}); linkErr != nil {
			return nil, fmt.Errorf("workflow step: persist parent link: %w", linkErr)
		}
	}

	output := map[string]any{
		"child_run_id":      childRunID.String(),
		"child_engine":      WorkflowStepTargetKindDAG,
		"child_workflow_id": targetID.String(),
		"status":            string(model.WorkflowStepRunStatusAwaitingSubWorkflow),
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
}

// resolveWorkflowStepTargetKind reads the `targetKind` discriminator from the
// trigger payload first, falling back to step config. The empty string is
// returned unchanged so callers can distinguish "omitted" (legacy default)
// from an explicit value.
func resolveWorkflowStepTargetKind(trigger map[string]any, config map[string]any) string {
	if v := strings.TrimSpace(workflowString(trigger, "targetKind")); v != "" {
		return strings.ToLower(v)
	}
	if config != nil {
		if v := strings.TrimSpace(workflowString(config, "targetKind")); v != "" {
			return strings.ToLower(v)
		}
		if v := strings.TrimSpace(workflowString(config, "target_kind")); v != "" {
			return strings.ToLower(v)
		}
	}
	return ""
}

func (e *WorkflowStepRouterExecutor) executeApproval(_ context.Context, req WorkflowStepExecutionRequest) (*WorkflowStepExecutionResult, error) {
	output := map[string]any{
		"status":  "awaiting_approval",
		"step_id": req.Step.ID,
		"run_id":  req.RunID.String(),
		"message": "workflow paused pending human approval",
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
}

// resolveStepActingEmployee returns the effective acting-employee id for a
// step. Precedence (change bridge-employee-attribution-legacy):
//
//  1. trigger.employeeId (step-level override, UUID string)
//  2. runActingEmployeeID (run-level default)
//  3. nil
//
// A malformed step-level value is treated the same as absent, i.e. the
// run-level default still applies.
func resolveStepActingEmployee(trigger map[string]any, runActingEmployeeID *uuid.UUID) *uuid.UUID {
	if rawEmp := workflowString(trigger, "employeeId"); rawEmp != "" {
		if parsed, err := uuid.Parse(rawEmp); err == nil {
			return &parsed
		}
	}
	if runActingEmployeeID != nil {
		eid := *runActingEmployeeID
		return &eid
	}
	return nil
}

func workflowInputMap(input map[string]any, key string) (map[string]any, error) {
	if input == nil {
		return nil, fmt.Errorf("workflow step input is required")
	}
	value, ok := input[key]
	if !ok {
		return nil, fmt.Errorf("workflow step input missing %s", key)
	}
	typed, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("workflow step input %s must be an object", key)
	}
	return typed, nil
}

func workflowUUID(input map[string]any, key string) (uuid.UUID, error) {
	raw := workflowString(input, key)
	if raw == "" {
		return uuid.Nil, fmt.Errorf("trigger.%s is required", key)
	}
	value, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, fmt.Errorf("trigger.%s must be a valid UUID: %w", key, err)
	}
	return value, nil
}

func workflowString(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	switch value := input[key].(type) {
	case string:
		return value
	default:
		return ""
	}
}

func workflowFloat(input map[string]any, key string) float64 {
	if input == nil {
		return 0
	}
	switch value := input[key].(type) {
	case float64:
		return value
	case float32:
		return float64(value)
	case int:
		return float64(value)
	case int64:
		return float64(value)
	case string:
		parsed, err := strconv.ParseFloat(value, 64)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func workflowInt(input map[string]any, key string) int {
	if input == nil {
		return 0
	}
	switch value := input[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed
		}
	}
	return 0
}

func workflowStringSlice(input map[string]any, key string) []string {
	if input == nil {
		return nil
	}
	value, ok := input[key]
	if !ok {
		return nil
	}
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			if asString, ok := item.(string); ok {
				items = append(items, asString)
			}
		}
		return items
	default:
		return nil
	}
}
