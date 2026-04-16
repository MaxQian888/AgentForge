package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type fakeTaskWorkflowDispatcher struct {
	calls []struct {
		taskID uuid.UUID
		req    *model.AssignRequest
	}
}

func (f *fakeTaskWorkflowDispatcher) Assign(_ context.Context, taskID uuid.UUID, req *model.AssignRequest) (*model.TaskDispatchResponse, error) {
	f.calls = append(f.calls, struct {
		taskID uuid.UUID
		req    *model.AssignRequest
	}{
		taskID: taskID,
		req:    req,
	})
	return &model.TaskDispatchResponse{}, nil
}

type fakeTaskWorkflowRuntime struct {
	calls []struct {
		pluginID string
		profile  string
		taskID   uuid.UUID
		req      service.WorkflowExecutionRequest
	}
	err error
}

func (f *fakeTaskWorkflowRuntime) StartTaskTriggered(_ context.Context, pluginID, profile string, taskID uuid.UUID, req service.WorkflowExecutionRequest) (*model.WorkflowPluginRun, error) {
	f.calls = append(f.calls, struct {
		pluginID string
		profile  string
		taskID   uuid.UUID
		req      service.WorkflowExecutionRequest
	}{
		pluginID: pluginID,
		profile:  profile,
		taskID:   taskID,
		req:      req,
	})
	if f.err != nil {
		return nil, f.err
	}
	return &model.WorkflowPluginRun{ID: uuid.New(), PluginID: pluginID}, nil
}

type fakeTaskWorkflowFollowUpNotifier struct {
	calls []service.IMBoundProgressRequest
}

func (f *fakeTaskWorkflowFollowUpNotifier) QueueBoundProgress(_ context.Context, req service.IMBoundProgressRequest) (bool, error) {
	f.calls = append(f.calls, req)
	return true, nil
}

type fakeWorkflowConfigRepo struct {
	config *model.WorkflowConfig
	err    error
}

func (f *fakeWorkflowConfigRepo) GetByProject(_ context.Context, _ uuid.UUID) (*model.WorkflowConfig, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.config, nil
}

func TestTaskWorkflowService_EvaluateTransition_NoConfig(t *testing.T) {
	svc := service.NewTaskWorkflowService(&fakeWorkflowConfigRepo{err: errors.New("not found")}, nil, nil)
	task := &model.Task{ID: uuid.New(), ProjectID: uuid.New(), Status: "assigned"}
	results := svc.EvaluateTransition(context.Background(), task, "triaged", "assigned")
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func TestTaskWorkflowService_EvaluateTransition_MatchesTrigger(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{FromStatus: "triaged", ToStatus: "assigned", Action: "notify"},
		{FromStatus: "assigned", ToStatus: "in_progress", Action: "auto_assign_agent"},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}

	svc := service.NewTaskWorkflowService(repo, nil, nil)
	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Status: "assigned"}

	results := svc.EvaluateTransition(context.Background(), task, "triaged", "assigned")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !results[0].Fired {
		t.Fatal("expected trigger to fire")
	}
	if results[0].Trigger.Action != "notify" {
		t.Fatalf("action = %q, want 'notify'", results[0].Trigger.Action)
	}
}

func TestTaskWorkflowService_EvaluateTransition_NoMatch(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{FromStatus: "in_progress", ToStatus: "done", Action: "notify"},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}

	svc := service.NewTaskWorkflowService(repo, nil, nil)
	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Status: "assigned"}

	results := svc.EvaluateTransition(context.Background(), task, "triaged", "assigned")
	if len(results) != 0 {
		t.Fatalf("expected no results, got %d", len(results))
	}
}

func TestTaskWorkflowService_EvaluateTransition_NilTask(t *testing.T) {
	svc := service.NewTaskWorkflowService(&fakeWorkflowConfigRepo{}, nil, nil)
	results := svc.EvaluateTransition(context.Background(), nil, "a", "b")
	if results != nil {
		t.Fatalf("expected nil results, got %v", results)
	}
}

func TestTaskWorkflowService_EvaluateTransition_NormalizesLegacyDispatchAlias(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{
			FromStatus: "triaged",
			ToStatus:   "assigned",
			Action:     "auto_assign",
			Config: map[string]any{
				"assignee_id": "3ef54d04-cfb6-48cf-b0fc-7671d7352d8b",
			},
		},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}
	dispatcher := &fakeTaskWorkflowDispatcher{}
	svc := service.NewTaskWorkflowService(repo, nil, nil)
	svc.SetDispatcher(dispatcher)

	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Status: "assigned"}
	results := svc.EvaluateTransition(context.Background(), task, "triaged", "assigned")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if got := results[0].Trigger.Action; got != "dispatch_agent" {
		t.Fatalf("normalized action = %q, want dispatch_agent", got)
	}
	if got := results[0].Outcome.Action; got != "dispatch_agent" {
		t.Fatalf("outcome action = %q, want dispatch_agent", got)
	}
	if got := results[0].Outcome.Status; got != "completed" {
		t.Fatalf("outcome status = %q, want completed", got)
	}
	if len(dispatcher.calls) != 1 {
		t.Fatalf("dispatcher calls = %d, want 1", len(dispatcher.calls))
	}
	if dispatcher.calls[0].req == nil || dispatcher.calls[0].req.AssigneeID != "3ef54d04-cfb6-48cf-b0fc-7671d7352d8b" {
		t.Fatalf("dispatcher request = %+v, want assignee_id propagated", dispatcher.calls[0].req)
	}
}

func TestTaskWorkflowService_EvaluateTransition_InvalidStartWorkflowConfigReturnsError(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{
			FromStatus: "assigned",
			ToStatus:   "in_progress",
			Action:     "start_workflow",
			Config: map[string]any{
				"profile": "task-delivery",
			},
		},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}
	svc := service.NewTaskWorkflowService(repo, nil, nil)
	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Status: "in_progress"}

	results := svc.EvaluateTransition(context.Background(), task, "assigned", "in_progress")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Fatal("expected invalid start_workflow config to return an error")
	}
	if got := results[0].Outcome.Action; got != "start_workflow" {
		t.Fatalf("outcome action = %q, want start_workflow", got)
	}
	if got := results[0].Outcome.Status; got != "failed" {
		t.Fatalf("outcome status = %q, want failed", got)
	}
}

func TestTaskWorkflowService_EvaluateTransition_StartWorkflowUsesTaskTriggeredRuntime(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{
			FromStatus: "assigned",
			ToStatus:   "in_progress",
			Action:     "start_workflow",
			Config: map[string]any{
				"plugin_id": "task-delivery-flow",
				"profile":   "task-delivery",
			},
		},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}
	runtime := &fakeTaskWorkflowRuntime{}
	svc := service.NewTaskWorkflowService(repo, nil, nil)
	svc.SetWorkflowRuntime(runtime)
	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Status: "in_progress"}

	results := svc.EvaluateTransition(context.Background(), task, "assigned", "in_progress")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error != nil {
		t.Fatalf("expected start_workflow to succeed, got %v", results[0].Error)
	}
	if len(runtime.calls) != 1 {
		t.Fatalf("workflow runtime calls = %d, want 1", len(runtime.calls))
	}
	if got := results[0].Outcome.Status; got != "started" {
		t.Fatalf("outcome status = %q, want started", got)
	}
	if got := results[0].Outcome.WorkflowPluginID; got != "task-delivery-flow" {
		t.Fatalf("workflow plugin id = %q, want task-delivery-flow", got)
	}
	if results[0].Outcome.WorkflowRunID == "" {
		t.Fatal("expected workflow run id in trigger outcome")
	}
	if runtime.calls[0].pluginID != "task-delivery-flow" {
		t.Fatalf("pluginID = %q, want task-delivery-flow", runtime.calls[0].pluginID)
	}
	if runtime.calls[0].profile != "task-delivery" {
		t.Fatalf("profile = %q, want task-delivery", runtime.calls[0].profile)
	}
	if runtime.calls[0].taskID != task.ID {
		t.Fatalf("taskID = %s, want %s", runtime.calls[0].taskID, task.ID)
	}
}

func TestTaskWorkflowService_EvaluateTransition_DuplicateWorkflowRunIsBlocked(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{
			FromStatus: "assigned",
			ToStatus:   "in_progress",
			Action:     "start_workflow",
			Config: map[string]any{
				"plugin_id": "task-delivery-flow",
				"profile":   "task-delivery",
			},
		},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}
	runtime := &fakeTaskWorkflowRuntime{
		err: errors.New("workflow plugin task-delivery-flow already has an active workflow run for task and profile"),
	}
	svc := service.NewTaskWorkflowService(repo, nil, nil)
	svc.SetWorkflowRuntime(runtime)
	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Status: "in_progress"}

	results := svc.EvaluateTransition(context.Background(), task, "assigned", "in_progress")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Error == nil {
		t.Fatal("expected duplicate active run to return an error")
	}
	if got := results[0].Outcome.Status; got != "blocked" {
		t.Fatalf("outcome status = %q, want blocked", got)
	}
}

func TestTaskWorkflowService_EvaluateTransition_QueuesTaskFollowUpForWorkflowOutcome(t *testing.T) {
	triggers := []model.WorkflowTrigger{
		{
			FromStatus: "assigned",
			ToStatus:   "in_progress",
			Action:     "start_workflow",
			Config: map[string]any{
				"plugin_id": "task-delivery-flow",
				"profile":   "task-delivery",
			},
		},
	}
	triggersJSON, _ := json.Marshal(triggers)

	repo := &fakeWorkflowConfigRepo{
		config: &model.WorkflowConfig{
			ID:        uuid.New(),
			ProjectID: uuid.New(),
			Triggers:  triggersJSON,
		},
	}
	runtime := &fakeTaskWorkflowRuntime{}
	followUp := &fakeTaskWorkflowFollowUpNotifier{}
	svc := service.NewTaskWorkflowService(repo, nil, nil)
	svc.SetWorkflowRuntime(runtime)
	svc.SetFollowUpNotifier(followUp)
	task := &model.Task{ID: uuid.New(), ProjectID: repo.config.ProjectID, Title: "Bridge rollout", Status: "in_progress"}

	results := svc.EvaluateTransition(context.Background(), task, "assigned", "in_progress")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len(followUp.calls) != 1 {
		t.Fatalf("follow-up calls = %d, want 1", len(followUp.calls))
	}
	call := followUp.calls[0]
	if call.TaskID != task.ID.String() {
		t.Fatalf("TaskID = %q, want %q", call.TaskID, task.ID.String())
	}
	if !call.IsTerminal {
		t.Fatal("expected task workflow follow-up to use terminal delivery")
	}
	if call.Metadata["bridge_event_type"] != "task.workflow_trigger" {
		t.Fatalf("metadata = %+v", call.Metadata)
	}
	if call.Structured == nil || call.Structured.Fields == nil {
		t.Fatalf("structured = %+v", call.Structured)
	}
}
