package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
)

// WorkflowConfigProvider fetches workflow configuration for a project.
type WorkflowConfigProvider interface {
	GetByProject(ctx context.Context, projectID uuid.UUID) (*model.WorkflowConfig, error)
}

// TaskWorkflowDispatcher can spawn agents for workflow-triggered assignment.
type TaskWorkflowDispatcher interface {
	Assign(ctx context.Context, taskID uuid.UUID, req *model.AssignRequest) (*model.TaskDispatchResponse, error)
}

// WorkflowTaskTransitioner transitions task status.
type WorkflowTaskTransitioner interface {
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
}

// WorkflowNotifier creates notifications.
type WorkflowNotifier interface {
	Create(ctx context.Context, notif *model.Notification) error
}

// TaskWorkflowService evaluates workflow triggers after task transitions.
type TaskWorkflowService struct {
	workflowRepo WorkflowConfigProvider
	hub          *ws.Hub
	dispatcher   TaskWorkflowDispatcher
	taskRepo     WorkflowTaskTransitioner
	notifier     WorkflowNotifier
}

// NewTaskWorkflowService creates a new trigger engine.
func NewTaskWorkflowService(workflowRepo WorkflowConfigProvider, hub *ws.Hub) *TaskWorkflowService {
	return &TaskWorkflowService{
		workflowRepo: workflowRepo,
		hub:          hub,
	}
}

func (s *TaskWorkflowService) SetDispatcher(d TaskWorkflowDispatcher) {
	s.dispatcher = d
}

func (s *TaskWorkflowService) SetTaskRepository(r WorkflowTaskTransitioner) {
	s.taskRepo = r
}

func (s *TaskWorkflowService) SetNotifier(n WorkflowNotifier) {
	s.notifier = n
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

	fields := log.Fields{
		"taskId":     task.ID.String(),
		"projectId":  task.ProjectID.String(),
		"fromStatus": fromStatus,
		"toStatus":   toStatus,
	}

	wfConfig, err := s.workflowRepo.GetByProject(ctx, task.ProjectID)
	if err != nil {
		return nil // no workflow configured — nothing to fire
	}

	var triggers []model.WorkflowTrigger
	if len(wfConfig.Triggers) > 0 {
		if err := json.Unmarshal(wfConfig.Triggers, &triggers); err != nil {
			log.WithError(err).WithField("projectId", task.ProjectID).Warn("failed to parse workflow triggers")
			return nil
		}
	}

	var results []TriggerResult
	for _, trigger := range triggers {
		if !matchesTrigger(trigger, fromStatus, toStatus) {
			continue
		}
		triggerFields := log.Fields{
			"taskId":     task.ID.String(),
			"projectId":  task.ProjectID.String(),
			"fromStatus": fromStatus,
			"toStatus":   toStatus,
			"action":     trigger.Action,
		}
		result := TriggerResult{Trigger: trigger, Fired: true}
		log.WithFields(triggerFields).Info("workflow trigger matched")
		result.Error = s.executeTrigger(ctx, task, trigger)
		results = append(results, result)
	}
	if len(results) > 0 {
		fields["matchedTriggers"] = len(results)
		log.WithFields(fields).Info("workflow transition evaluated with matches")
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

func (s *TaskWorkflowService) executeTrigger(ctx context.Context, task *model.Task, trigger model.WorkflowTrigger) error {
	fields := log.Fields{
		"taskId":     task.ID.String(),
		"projectId":  task.ProjectID.String(),
		"action":     trigger.Action,
		"fromStatus": trigger.FromStatus,
		"toStatus":   trigger.ToStatus,
	}

	switch trigger.Action {
	case "notify":
		log.WithFields(fields).Info("workflow trigger: notify")
		s.broadcastTriggerFired(task, trigger)
		return s.executeNotify(ctx, task, trigger)

	case "auto_assign_agent":
		log.WithFields(fields).Info("workflow trigger: auto_assign_agent")
		s.broadcastTriggerFired(task, trigger)
		return s.executeAutoAssignAgent(ctx, task, trigger)

	case "auto_transition":
		log.WithFields(fields).Info("workflow trigger: auto_transition")
		s.broadcastTriggerFired(task, trigger)
		return s.executeAutoTransition(ctx, task, trigger)

	default:
		log.WithFields(fields).Warn("unknown workflow trigger action")
		s.broadcastTriggerFired(task, trigger)
		return nil
	}
}

// triggerConfigMap extracts the trigger Config as a map[string]any.
func triggerConfigMap(trigger model.WorkflowTrigger) map[string]any {
	if trigger.Config == nil {
		return nil
	}
	switch v := trigger.Config.(type) {
	case map[string]any:
		return v
	default:
		// Try JSON round-trip
		b, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			return m
		}
		return nil
	}
}

func (s *TaskWorkflowService) executeAutoAssignAgent(ctx context.Context, task *model.Task, trigger model.WorkflowTrigger) error {
	if s.dispatcher == nil {
		log.WithField("taskId", task.ID.String()).Warn("workflow auto_assign_agent: dispatcher not available")
		return nil
	}

	cfg := triggerConfigMap(trigger)
	req := &model.AssignRequest{
		AssigneeType: "agent",
	}

	if assigneeID, ok := cfg["assignee_id"].(string); ok {
		req.AssigneeID = assigneeID
	}
	if assigneeType, ok := cfg["assignee_type"].(string); ok {
		req.AssigneeType = assigneeType
	}

	if req.AssigneeID == "" {
		log.WithField("taskId", task.ID.String()).Warn("workflow auto_assign_agent: no assignee_id in trigger config")
		return nil
	}

	_, err := s.dispatcher.Assign(ctx, task.ID, req)
	if err != nil {
		return fmt.Errorf("workflow auto_assign_agent: %w", err)
	}
	return nil
}

func (s *TaskWorkflowService) executeAutoTransition(ctx context.Context, task *model.Task, trigger model.WorkflowTrigger) error {
	if s.taskRepo == nil {
		log.WithField("taskId", task.ID.String()).Warn("workflow auto_transition: task repo not available")
		return nil
	}

	cfg := triggerConfigMap(trigger)
	targetStatus, ok := cfg["target_status"].(string)
	if !ok || targetStatus == "" {
		log.WithField("taskId", task.ID.String()).Warn("workflow auto_transition: no target_status in trigger config")
		return nil
	}

	if err := s.taskRepo.TransitionStatus(ctx, task.ID, targetStatus); err != nil {
		return fmt.Errorf("workflow auto_transition to %s: %w", targetStatus, err)
	}
	return nil
}

func (s *TaskWorkflowService) executeNotify(ctx context.Context, task *model.Task, trigger model.WorkflowTrigger) error {
	if s.notifier == nil {
		return nil
	}

	cfg := triggerConfigMap(trigger)
	title := "Workflow notification"
	body := fmt.Sprintf("Task '%s' transitioned from %s to %s", task.Title, trigger.FromStatus, trigger.ToStatus)
	if customTitle, ok := cfg["title"].(string); ok && customTitle != "" {
		title = customTitle
	}
	if customBody, ok := cfg["body"].(string); ok && customBody != "" {
		body = customBody
	}

	targetID := task.ProjectID
	if task.AssigneeID != nil {
		targetID = *task.AssigneeID
	}

	notif := &model.Notification{
		ID:      uuid.New(),
		TargetID: targetID,
		Channel: "web",
		Type:    "workflow_trigger",
		Title:   title,
		Body:    body,
	}

	return s.notifier.Create(ctx, notif)
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
