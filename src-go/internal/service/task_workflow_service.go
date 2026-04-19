package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
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

type TaskWorkflowRuntime interface {
	StartTaskTriggered(ctx context.Context, pluginID string, profile string, taskID uuid.UUID, req WorkflowExecutionRequest) (*model.WorkflowPluginRun, error)
}

type TaskWorkflowProgressRecorder interface {
	RecordActivity(ctx context.Context, taskID uuid.UUID, input TaskActivityInput) (*model.TaskProgressSnapshot, error)
}

type TaskWorkflowFollowUpNotifier interface {
	QueueBoundProgress(ctx context.Context, req IMBoundProgressRequest) (bool, error)
}

// TaskWorkflowService evaluates workflow triggers after task transitions.
type TaskWorkflowService struct {
	workflowRepo WorkflowConfigProvider
	hub          *ws.Hub
	bus          eventbus.Publisher
	dispatcher   TaskWorkflowDispatcher
	taskRepo     WorkflowTaskTransitioner
	notifier     WorkflowNotifier
	workflow     TaskWorkflowRuntime
	progress     TaskWorkflowProgressRecorder
	followUp     TaskWorkflowFollowUpNotifier
}

// NewTaskWorkflowService creates a new trigger engine.
func NewTaskWorkflowService(workflowRepo WorkflowConfigProvider, hub *ws.Hub, bus eventbus.Publisher) *TaskWorkflowService {
	return &TaskWorkflowService{
		workflowRepo: workflowRepo,
		hub:          hub,
		bus:          bus,
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

func (s *TaskWorkflowService) SetWorkflowRuntime(runtime TaskWorkflowRuntime) {
	s.workflow = runtime
}

func (s *TaskWorkflowService) SetProgressRecorder(progress TaskWorkflowProgressRecorder) {
	s.progress = progress
}

func (s *TaskWorkflowService) SetFollowUpNotifier(notifier TaskWorkflowFollowUpNotifier) {
	s.followUp = notifier
}

// TriggerResult captures the outcome of a fired trigger.
type TriggerResult struct {
	Trigger model.TaskWorkflowTrigger
	Fired   bool
	Outcome model.TaskWorkflowTriggerOutcome
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

	var triggers []model.TaskWorkflowTrigger
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
		normalizedTrigger := normalizeWorkflowTrigger(trigger)
		triggerFields := log.Fields{
			"taskId":     task.ID.String(),
			"projectId":  task.ProjectID.String(),
			"fromStatus": fromStatus,
			"toStatus":   toStatus,
			"action":     normalizedTrigger.Action,
		}
		result := TriggerResult{
			Trigger: normalizedTrigger,
			Fired:   true,
			Outcome: model.TaskWorkflowTriggerOutcome{
				Action: normalizedTrigger.Action,
				Status: model.TaskWorkflowTriggerOutcomeSkipped,
			},
		}
		log.WithFields(triggerFields).Info("workflow trigger matched")
		result.Outcome, result.Error = s.executeTrigger(ctx, task, normalizedTrigger)
		s.recordTriggerActivity(ctx, task, result.Outcome)
		s.notifyTriggerFollowUp(ctx, task, normalizedTrigger, result.Outcome)
		results = append(results, result)
	}
	if len(results) > 0 {
		fields["matchedTriggers"] = len(results)
		log.WithFields(fields).Info("workflow transition evaluated with matches")
	}
	return results
}

func matchesTrigger(trigger model.TaskWorkflowTrigger, fromStatus, toStatus string) bool {
	if trigger.FromStatus != "" && trigger.FromStatus != fromStatus {
		return false
	}
	if trigger.ToStatus != "" && trigger.ToStatus != toStatus {
		return false
	}
	// At least one condition must be specified
	return trigger.FromStatus != "" || trigger.ToStatus != ""
}

func (s *TaskWorkflowService) executeTrigger(ctx context.Context, task *model.Task, trigger model.TaskWorkflowTrigger) (model.TaskWorkflowTriggerOutcome, error) {
	fields := log.Fields{
		"taskId":     task.ID.String(),
		"projectId":  task.ProjectID.String(),
		"action":     trigger.Action,
		"fromStatus": trigger.FromStatus,
		"toStatus":   trigger.ToStatus,
	}
	outcome := model.TaskWorkflowTriggerOutcome{
		Action: trigger.Action,
		Status: model.TaskWorkflowTriggerOutcomeSkipped,
	}

	switch trigger.Action {
	case model.TaskWorkflowTriggerActionNotify:
		log.WithFields(fields).Info("workflow trigger: notify")
		err := s.executeNotify(ctx, task, trigger)
		if err != nil {
			outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
			outcome.Reason = err.Error()
			outcome.ReasonCode = "notify_failed"
		} else {
			outcome.Status = model.TaskWorkflowTriggerOutcomeCompleted
		}
		s.broadcastTriggerFired(task, trigger, outcome)
		return outcome, err

	case model.TaskWorkflowTriggerActionDispatchAgent:
		log.WithFields(fields).Info("workflow trigger: dispatch_agent")
		dispatchOutcome, err := s.executeAutoAssignAgent(ctx, task, trigger)
		outcome.Status = dispatchOutcome.Status
		outcome.Reason = dispatchOutcome.Reason
		s.broadcastTriggerFired(task, trigger, outcome)
		return outcome, err

	case model.TaskWorkflowTriggerActionAutoTransition:
		log.WithFields(fields).Info("workflow trigger: auto_transition")
		err := s.executeAutoTransition(ctx, task, trigger)
		if err != nil {
			outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
			outcome.Reason = err.Error()
			outcome.ReasonCode = "transition_failed"
		} else {
			outcome.Status = model.TaskWorkflowTriggerOutcomeCompleted
		}
		s.broadcastTriggerFired(task, trigger, outcome)
		return outcome, err

	case model.TaskWorkflowTriggerActionStartWorkflow:
		log.WithFields(fields).Info("workflow trigger: start_workflow")
		startOutcome, err := s.executeStartWorkflow(ctx, task, trigger)
		s.broadcastTriggerFired(task, trigger, startOutcome)
		return startOutcome, err

	default:
		log.WithFields(fields).Warn("unknown workflow trigger action")
		outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
		outcome.Reason = fmt.Sprintf("unknown workflow trigger action %q", trigger.Action)
		outcome.ReasonCode = "unknown_action"
		s.broadcastTriggerFired(task, trigger, outcome)
		return outcome, fmt.Errorf("unknown workflow trigger action %q", trigger.Action)
	}
}

func normalizeWorkflowTrigger(trigger model.TaskWorkflowTrigger) model.TaskWorkflowTrigger {
	normalized := trigger
	switch trigger.Action {
	case "auto_assign", "auto_assign_agent":
		normalized.Action = model.TaskWorkflowTriggerActionDispatchAgent
	case model.TaskWorkflowTriggerActionNotify:
		normalized.Action = model.TaskWorkflowTriggerActionNotify
	case model.TaskWorkflowTriggerActionAutoTransition:
		normalized.Action = model.TaskWorkflowTriggerActionAutoTransition
	case model.TaskWorkflowTriggerActionStartWorkflow:
		normalized.Action = model.TaskWorkflowTriggerActionStartWorkflow
	}
	return normalized
}

// triggerConfigMap extracts the trigger Config as a map[string]any.
func triggerConfigMap(trigger model.TaskWorkflowTrigger) map[string]any {
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

func (s *TaskWorkflowService) executeAutoAssignAgent(ctx context.Context, task *model.Task, trigger model.TaskWorkflowTrigger) (model.TaskWorkflowTriggerOutcome, error) {
	outcome := model.TaskWorkflowTriggerOutcome{
		Action: trigger.Action,
		Status: model.TaskWorkflowTriggerOutcomeSkipped,
	}
	if s.dispatcher == nil {
		log.WithField("taskId", task.ID.String()).Warn("workflow dispatch_agent: dispatcher not available")
		return outcome, nil
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
		log.WithField("taskId", task.ID.String()).Warn("workflow dispatch_agent: no assignee_id in trigger config")
		outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
		outcome.Reason = "workflow dispatch_agent: no assignee_id in trigger config"
		outcome.ReasonCode = "missing_assignee_id"
		return outcome, fmt.Errorf("workflow dispatch_agent: no assignee_id in trigger config")
	}

	resp, err := s.dispatcher.Assign(ctx, task.ID, req)
	if err != nil {
		outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
		outcome.Reason = err.Error()
		outcome.ReasonCode = "dispatch_failed"
		return outcome, fmt.Errorf("workflow dispatch_agent: %w", err)
	}
	outcome.Status = model.TaskWorkflowTriggerOutcomeCompleted
	if resp != nil && resp.Dispatch.Status != "" {
		outcome.Status = resp.Dispatch.Status
		outcome.Reason = resp.Dispatch.Reason
	}
	return outcome, nil
}

func (s *TaskWorkflowService) recordTriggerActivity(ctx context.Context, task *model.Task, outcome model.TaskWorkflowTriggerOutcome) {
	if s.progress == nil || task == nil {
		return
	}
	if outcome.Status != model.TaskWorkflowTriggerOutcomeStarted && outcome.Status != model.TaskWorkflowTriggerOutcomeCompleted {
		return
	}
	_, _ = s.progress.RecordActivity(ctx, task.ID, TaskActivityInput{
		Source:       model.TaskProgressSourceWorkflowTrigger,
		UpdateHealth: true,
	})
}

func (s *TaskWorkflowService) notifyTriggerFollowUp(ctx context.Context, task *model.Task, trigger model.TaskWorkflowTrigger, outcome model.TaskWorkflowTriggerOutcome) {
	if s.followUp == nil || task == nil {
		return
	}
	if outcome.Status == "" || outcome.Status == model.TaskWorkflowTriggerOutcomeSkipped {
		return
	}

	transition := strings.TrimSpace(trigger.FromStatus) + " -> " + strings.TrimSpace(trigger.ToStatus)
	content := fmt.Sprintf(
		"Task workflow update\nTask: %s\nAction: %s\nOutcome: %s\nTransition: %s",
		task.Title,
		outcome.Action,
		outcome.Status,
		strings.TrimSpace(transition),
	)
	if trimmed := strings.TrimSpace(outcome.Reason); trimmed != "" {
		content += "\nReason: " + trimmed
	}

	metadata := map[string]string{
		"bridge_event_type":             "task.workflow_trigger",
		"workflow_trigger_action":       strings.TrimSpace(outcome.Action),
		"workflow_trigger_status":       strings.TrimSpace(outcome.Status),
		"workflow_trigger_transition":   strings.TrimSpace(transition),
		"workflow_trigger_reason_code":  strings.TrimSpace(outcome.ReasonCode),
		"workflow_trigger_workflow_run": strings.TrimSpace(outcome.WorkflowRunID),
	}
	if trimmed := strings.TrimSpace(outcome.WorkflowPluginID); trimmed != "" {
		metadata["workflow_trigger_plugin_id"] = trimmed
	}
	if trimmed := strings.TrimSpace(outcome.Reason); trimmed != "" {
		metadata["workflow_trigger_reason"] = trimmed
	}

	_, err := s.followUp.QueueBoundProgress(ctx, IMBoundProgressRequest{
		TaskID:     task.ID.String(),
		Kind:       IMDeliveryKindTerminal,
		IsTerminal: true,
		Content:    content,
		Structured: &model.IMStructuredMessage{
			Title: "Task workflow update",
			Body:  fmt.Sprintf("%s -> %s: %s", trigger.FromStatus, trigger.ToStatus, outcome.Status),
			Fields: []model.IMStructuredField{
				{Label: "Task", Value: task.Title},
				{Label: "Action", Value: outcome.Action},
				{Label: "Outcome", Value: outcome.Status},
			},
		},
		Metadata: metadata,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"taskId":     task.ID.String(),
			"projectId":  task.ProjectID.String(),
			"action":     outcome.Action,
			"status":     outcome.Status,
			"reasonCode": outcome.ReasonCode,
		}).WithError(err).Warn("workflow trigger IM follow-up failed")
	}
}

func (s *TaskWorkflowService) executeAutoTransition(ctx context.Context, task *model.Task, trigger model.TaskWorkflowTrigger) error {
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

func (s *TaskWorkflowService) executeStartWorkflow(ctx context.Context, task *model.Task, trigger model.TaskWorkflowTrigger) (model.TaskWorkflowTriggerOutcome, error) {
	outcome := model.TaskWorkflowTriggerOutcome{
		Action: trigger.Action,
		Status: model.TaskWorkflowTriggerOutcomeSkipped,
	}
	cfg := triggerConfigMap(trigger)
	pluginID, _ := cfg["plugin_id"].(string)
	if pluginID == "" {
		pluginID, _ = cfg["pluginId"].(string)
	}
	if pluginID == "" {
		outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
		outcome.Reason = "workflow start trigger requires plugin_id"
		outcome.ReasonCode = "missing_plugin_id"
		return outcome, fmt.Errorf("workflow start trigger requires plugin_id")
	}
	profile, _ := cfg["profile"].(string)
	if profile == "" {
		outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
		outcome.Reason = "workflow start trigger requires profile"
		outcome.ReasonCode = "missing_profile"
		outcome.WorkflowPluginID = pluginID
		return outcome, fmt.Errorf("workflow start trigger requires profile")
	}
	if s.workflow == nil {
		outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
		outcome.Reason = "workflow runtime is unavailable"
		outcome.ReasonCode = "runtime_unavailable"
		outcome.WorkflowPluginID = pluginID
		return outcome, fmt.Errorf("workflow runtime is unavailable")
	}
	triggerPayload := map[string]any{
		"fromStatus": trigger.FromStatus,
		"toStatus":   trigger.ToStatus,
	}
	run, err := s.workflow.StartTaskTriggered(ctx, pluginID, profile, task.ID, WorkflowExecutionRequest{
		Trigger: triggerPayload,
	})
	if err != nil {
		outcome.Status = model.TaskWorkflowTriggerOutcomeFailed
		if strings.Contains(err.Error(), "active workflow run") {
			outcome.Status = model.TaskWorkflowTriggerOutcomeBlocked
			outcome.ReasonCode = "duplicate_active_run"
		}
		outcome.Reason = err.Error()
		if outcome.ReasonCode == "" {
			outcome.ReasonCode = "workflow_start_failed"
		}
		outcome.WorkflowPluginID = pluginID
		return outcome, fmt.Errorf("workflow start trigger %s: %w", pluginID, err)
	}
	outcome.Status = model.TaskWorkflowTriggerOutcomeStarted
	outcome.WorkflowPluginID = pluginID
	if run != nil {
		outcome.WorkflowRunID = run.ID.String()
	}
	return outcome, nil
}

func (s *TaskWorkflowService) executeNotify(ctx context.Context, task *model.Task, trigger model.TaskWorkflowTrigger) error {
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
		ID:       uuid.New(),
		TargetID: targetID,
		Channel:  "web",
		Type:     "workflow_trigger",
		Title:    title,
		Body:     body,
	}

	return s.notifier.Create(ctx, notif)
}

func (s *TaskWorkflowService) broadcastTriggerFired(task *model.Task, trigger model.TaskWorkflowTrigger, outcome model.TaskWorkflowTriggerOutcome) {
	_ = eventbus.PublishLegacy(context.Background(), s.bus, ws.EventWorkflowTriggerFired, task.ProjectID.String(), map[string]any{
		"taskId":  task.ID.String(),
		"action":  trigger.Action,
		"from":    trigger.FromStatus,
		"to":      trigger.ToStatus,
		"config":  trigger.Config,
		"outcome": outcome,
	})
}
