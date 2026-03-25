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
	taskID    uuid.UUID
	memberID  uuid.UUID
	runtime   string
	provider  string
	modelName string
	budgetUSD float64
	roleID    string
	err       error
}

func (m *workflowAgentSpawnerMock) Spawn(_ context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	m.taskID = taskID
	m.memberID = memberID
	m.runtime = runtime
	m.provider = provider
	m.modelName = modelName
	m.budgetUSD = budgetUsd
	m.roleID = roleID
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
