package service

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

// WorkflowConfigProvider fetches workflow configuration for a project.
type WorkflowConfigProvider interface {
	GetByProject(ctx context.Context, projectID uuid.UUID) (*model.WorkflowConfig, error)
}

// TaskWorkflowService evaluates workflow triggers after task transitions.
type TaskWorkflowService struct {
	workflowRepo WorkflowConfigProvider
	hub          *ws.Hub
}

// NewTaskWorkflowService creates a new trigger engine.
func NewTaskWorkflowService(workflowRepo WorkflowConfigProvider, hub *ws.Hub) *TaskWorkflowService {
	return &TaskWorkflowService{
		workflowRepo: workflowRepo,
		hub:          hub,
	}
}

// TriggerResult captures the outcome of a fired trigger.
type TriggerResult struct {
	Trigger model.WorkflowTrigger
	Fired   bool
	Error   error
}

// EvaluateTransition checks all triggers for a project and fires matching ones.
func (s *TaskWorkflowService) EvaluateTransition(ctx context.Context, task *model.Task, fromStatus, toStatus string) []TriggerResult {
	if s.workflowRepo == nil || task == nil {
		return nil
	}

	wfConfig, err := s.workflowRepo.GetByProject(ctx, task.ProjectID)
	if err != nil {
		return nil // no workflow configured — nothing to fire
	}

	var triggers []model.WorkflowTrigger
	if len(wfConfig.Triggers) > 0 {
		if err := json.Unmarshal(wfConfig.Triggers, &triggers); err != nil {
			slog.Warn("failed to parse workflow triggers", "error", err, "projectId", task.ProjectID)
			return nil
		}
	}

	var results []TriggerResult
	for _, trigger := range triggers {
		if !matchesTrigger(trigger, fromStatus, toStatus) {
			continue
		}
		result := TriggerResult{Trigger: trigger, Fired: true}
		s.executeTrigger(ctx, task, trigger)
		results = append(results, result)
	}
	return results
}

func matchesTrigger(trigger model.WorkflowTrigger, fromStatus, toStatus string) bool {
	if trigger.FromStatus != "" && trigger.FromStatus != fromStatus {
		return false
	}
	if trigger.ToStatus != "" && trigger.ToStatus != toStatus {
		return false
	}
	// At least one condition must be specified
	return trigger.FromStatus != "" || trigger.ToStatus != ""
}

func (s *TaskWorkflowService) executeTrigger(ctx context.Context, task *model.Task, trigger model.WorkflowTrigger) {
	switch trigger.Action {
	case "notify":
		s.broadcastTriggerFired(task, trigger)
	case "auto_assign_agent":
		// Broadcast event so the frontend or dispatcher can pick it up
		s.broadcastTriggerFired(task, trigger)
	case "auto_transition":
		// Broadcast event for the handler to process
		s.broadcastTriggerFired(task, trigger)
	default:
		slog.Warn("unknown workflow trigger action", "action", trigger.Action, "taskId", task.ID)
	}
}

func (s *TaskWorkflowService) broadcastTriggerFired(task *model.Task, trigger model.WorkflowTrigger) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventWorkflowTriggerFired,
		ProjectID: task.ProjectID.String(),
		Payload: map[string]any{
			"taskId": task.ID.String(),
			"action": trigger.Action,
			"from":   trigger.FromStatus,
			"to":     trigger.ToStatus,
			"config": trigger.Config,
		},
	})
}
