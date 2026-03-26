package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type WorkflowAgentSpawner interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
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

type WorkflowStepRouterExecutor struct {
	agents    WorkflowAgentSpawner
	reviews   WorkflowReviewRunner
	tasks     WorkflowTaskDispatcher
	workflows WorkflowChildSpawner
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
	default:
		return nil, fmt.Errorf("unsupported workflow action: %s", req.Step.Action)
	}
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
	run, err := e.agents.Spawn(
		ctx,
		taskID,
		memberID,
		workflowString(trigger, "runtime"),
		workflowString(trigger, "provider"),
		workflowString(trigger, "model"),
		workflowFloat(trigger, "budgetUsd"),
		req.Step.Role,
	)
	if err != nil {
		return nil, err
	}
	output := map[string]any{
		"runId":   run.ID.String(),
		"taskId":  run.TaskID.String(),
		"memberId": run.MemberID.String(),
		"roleId":  run.RoleID,
		"status":  run.Status,
		"runtime": run.Runtime,
		"provider": run.Provider,
		"model":   run.Model,
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
		TaskID:    taskID,
		Runtime:   workflowString(trigger, "runtime"),
		Provider:  workflowString(trigger, "provider"),
		Model:     workflowString(trigger, "model"),
		BudgetUSD: workflowFloat(trigger, "budgetUsd"),
		RoleID:    req.Step.Role,
	}
	if rawMemberID := workflowString(trigger, "memberId"); rawMemberID != "" {
		memberID, err := uuid.Parse(rawMemberID)
		if err != nil {
			return nil, fmt.Errorf("trigger.memberId must be a valid UUID: %w", err)
		}
		dispatchInput.MemberID = &memberID
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
	return &WorkflowStepExecutionResult{Output: output}, nil
}

func (e *WorkflowStepRouterExecutor) executeWorkflow(ctx context.Context, req WorkflowStepExecutionRequest, trigger map[string]any) (*WorkflowStepExecutionResult, error) {
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
		"status":       string(childRun.Status),
	}
	return &WorkflowStepExecutionResult{Output: output}, nil
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
