package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type AutomationEvent struct {
	EventType             string
	ProjectID             uuid.UUID
	TaskID                *uuid.UUID
	Task                  *model.Task
	TriggeredByAutomation bool
	Data                  map[string]any
}

type AutomationEventEvaluator interface {
	EvaluateRules(ctx context.Context, event AutomationEvent) error
}

type automationRuleReader interface {
	ListByProjectAndEvent(ctx context.Context, projectID uuid.UUID, eventType string) ([]*model.AutomationRule, error)
}

type automationLogWriter interface {
	Create(ctx context.Context, entry *model.AutomationLog) error
}

type automationTaskRepository interface {
	Create(ctx context.Context, task *model.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
	UpdateAssignee(ctx context.Context, id uuid.UUID, assigneeID uuid.UUID, assigneeType string) error
	ListOpenForProgress(ctx context.Context) ([]*model.Task, error)
}

type automationCustomFieldWriter interface {
	SetValue(ctx context.Context, value *model.CustomFieldValue) error
}

type automationNotificationSender interface {
	Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error)
}

type automationIMSender interface {
	Send(ctx context.Context, req *model.IMSendRequest) error
}

type automationPluginInvoker interface {
	Invoke(ctx context.Context, pluginID, operation string, payload map[string]any) (map[string]any, error)
}

type automationWorkflowStarter interface {
	Start(ctx context.Context, pluginID string, req WorkflowExecutionRequest) (*model.WorkflowPluginRun, error)
	ListRuns(ctx context.Context, pluginID string, limit int) ([]*model.WorkflowPluginRun, error)
}

type AutomationActionOutcome struct {
	Type       string `json:"type"`
	Outcome    string `json:"outcome"`
	ReasonCode string `json:"reasonCode,omitempty"`
	Reason     string `json:"reason,omitempty"`
	PluginID   string `json:"pluginId,omitempty"`
	RunID      string `json:"runId,omitempty"`
}

type AutomationEvaluationSummary struct {
	MatchedRules     int `json:"matchedRules"`
	StartedWorkflows int `json:"startedWorkflows"`
	BlockedWorkflows int `json:"blockedWorkflows"`
	FailedWorkflows  int `json:"failedWorkflows"`
}

type AutomationDueDateSummary struct {
	EvaluatedTasks   int `json:"evaluatedTasks"`
	MatchedRules     int `json:"matchedRules"`
	StartedWorkflows int `json:"startedWorkflows"`
	BlockedWorkflows int `json:"blockedWorkflows"`
	FailedWorkflows  int `json:"failedWorkflows"`
}

type AutomationEngineService struct {
	rules         automationRuleReader
	logs          automationLogWriter
	tasks         automationTaskRepository
	customFields  automationCustomFieldWriter
	notifications automationNotificationSender
	im            automationIMSender
	imChannels    IMEventChannelResolver
	plugins       automationPluginInvoker
	workflows     automationWorkflowStarter
	now           func() time.Time
}

func NewAutomationEngineService(
	rules automationRuleReader,
	logs automationLogWriter,
	tasks automationTaskRepository,
	customFields automationCustomFieldWriter,
	notifications automationNotificationSender,
	im automationIMSender,
	plugins automationPluginInvoker,
) *AutomationEngineService {
	return &AutomationEngineService{
		rules:         rules,
		logs:          logs,
		tasks:         tasks,
		customFields:  customFields,
		notifications: notifications,
		im:            im,
		plugins:       plugins,
		now:           func() time.Time { return time.Now().UTC() },
	}
}

func (s *AutomationEngineService) SetIMSender(im automationIMSender) { s.im = im }

func (s *AutomationEngineService) SetWorkflowStarter(workflows automationWorkflowStarter) {
	s.workflows = workflows
}

func (s *AutomationEngineService) SetIMChannelResolver(resolver IMEventChannelResolver) {
	s.imChannels = resolver
}

func (s *AutomationEngineService) EvaluateRules(ctx context.Context, event AutomationEvent) error {
	_, err := s.EvaluateRulesWithSummary(ctx, event)
	return err
}

func (s *AutomationEngineService) EvaluateRulesWithSummary(ctx context.Context, event AutomationEvent) (AutomationEvaluationSummary, error) {
	var summary AutomationEvaluationSummary
	if event.TriggeredByAutomation {
		return summary, nil
	}
	if event.ProjectID == uuid.Nil || strings.TrimSpace(event.EventType) == "" {
		return summary, nil
	}

	if event.Task == nil && event.TaskID != nil && s.tasks != nil {
		task, err := s.tasks.GetByID(ctx, *event.TaskID)
		if err != nil {
			return summary, fmt.Errorf("load automation task: %w", err)
		}
		event.Task = task
	}

	rules, err := s.rules.ListByProjectAndEvent(ctx, event.ProjectID, event.EventType)
	if err != nil {
		return summary, fmt.Errorf("list automation rules: %w", err)
	}
	for _, rule := range rules {
		if rule == nil || !rule.Enabled {
			continue
		}
		conditions, err := decodeAutomationConditions(rule.Conditions)
		if err != nil {
			s.writeAutomationLog(ctx, rule, event, model.AutomationLogStatusFailed, map[string]any{"error": err.Error(), "stage": "decode_conditions"})
			continue
		}
		matched, reason := s.conditionsMatch(event, conditions)
		if !matched {
			s.writeAutomationLog(ctx, rule, event, model.AutomationLogStatusSkipped, map[string]any{"reason": reason})
			continue
		}
		summary.MatchedRules++
		actions, err := decodeAutomationActions(rule.Actions)
		if err != nil {
			s.writeAutomationLog(ctx, rule, event, model.AutomationLogStatusFailed, map[string]any{"error": err.Error(), "stage": "decode_actions"})
			continue
		}
		actionOutcomes, err := s.executeActions(ctx, rule, event, actions)
		summary.addActionOutcomes(actionOutcomes)
		detail := map[string]any{
			"actionCount": len(actions),
		}
		if len(actionOutcomes) > 0 {
			detail["actionOutcomes"] = actionOutcomes
		}
		if err != nil {
			detail["error"] = err.Error()
			s.writeAutomationLog(ctx, rule, event, model.AutomationLogStatusFailed, detail)
			continue
		}
		s.writeAutomationLog(ctx, rule, event, model.AutomationLogStatusSuccess, detail)
	}
	return summary, nil
}

func (s *AutomationEngineService) CheckDueDateApproaching(ctx context.Context, threshold time.Duration) (*AutomationDueDateSummary, error) {
	summary := &AutomationDueDateSummary{}
	if s.tasks == nil {
		return summary, nil
	}
	tasks, err := s.tasks.ListOpenForProgress(ctx)
	if err != nil {
		return nil, fmt.Errorf("list open tasks: %w", err)
	}
	now := s.now()
	for _, task := range tasks {
		if task == nil || task.PlannedEndAt == nil {
			continue
		}
		if task.PlannedEndAt.Before(now) {
			continue
		}
		if task.PlannedEndAt.Sub(now) <= threshold {
			summary.EvaluatedTasks++
			taskID := task.ID
			evalSummary, err := s.EvaluateRulesWithSummary(ctx, AutomationEvent{
				EventType: model.AutomationEventTaskDueDateApproach,
				ProjectID: task.ProjectID,
				TaskID:    &taskID,
				Task:      task,
				Data: map[string]any{
					"due_at":       task.PlannedEndAt.Format(time.RFC3339),
					"threshold_ms": threshold.Milliseconds(),
				},
			})
			if err != nil {
				return nil, err
			}
			summary.MatchedRules += evalSummary.MatchedRules
			summary.StartedWorkflows += evalSummary.StartedWorkflows
			summary.BlockedWorkflows += evalSummary.BlockedWorkflows
			summary.FailedWorkflows += evalSummary.FailedWorkflows
		}
	}
	return summary, nil
}

type automationCondition struct {
	Field string `json:"field"`
	Op    string `json:"op"`
	Value any    `json:"value"`
}

type automationAction struct {
	Type   string         `json:"type"`
	Config map[string]any `json:"config"`
}

func decodeAutomationConditions(raw string) ([]automationCondition, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var conditions []automationCondition
	if err := json.Unmarshal([]byte(raw), &conditions); err != nil {
		return nil, err
	}
	return conditions, nil
}

func decodeAutomationActions(raw string) ([]automationAction, error) {
	var actions []automationAction
	if err := json.Unmarshal([]byte(defaultJSON(raw, "[]")), &actions); err != nil {
		return nil, err
	}
	return actions, nil
}

func (s *AutomationEngineService) conditionsMatch(event AutomationEvent, conditions []automationCondition) (bool, string) {
	for _, condition := range conditions {
		actual := automationFieldValue(event, condition.Field)
		if !compareAutomationValue(actual, condition.Op, condition.Value) {
			return false, condition.Field
		}
	}
	return true, ""
}

func automationFieldValue(event AutomationEvent, field string) any {
	if event.Data != nil {
		if value, ok := event.Data[field]; ok {
			return value
		}
	}
	switch field {
	case "event_type":
		return event.EventType
	case "project_id":
		return event.ProjectID.String()
	case "task_id":
		if event.TaskID != nil {
			return event.TaskID.String()
		}
	}
	if event.Task == nil {
		return nil
	}
	switch field {
	case "status":
		return event.Task.Status
	case "priority":
		return event.Task.Priority
	case "assignee_type":
		return event.Task.AssigneeType
	case "assignee_id":
		if event.Task.AssigneeID != nil {
			return event.Task.AssigneeID.String()
		}
		return ""
	case "title":
		return event.Task.Title
	}
	return nil
}

func compareAutomationValue(actual any, op string, expected any) bool {
	switch strings.ToLower(strings.TrimSpace(op)) {
	case "", "eq":
		return fmt.Sprint(actual) == fmt.Sprint(expected)
	case "ne":
		return fmt.Sprint(actual) != fmt.Sprint(expected)
	case "contains":
		return strings.Contains(strings.ToLower(fmt.Sprint(actual)), strings.ToLower(fmt.Sprint(expected)))
	case "gt", "gte", "lt", "lte":
		actualFloat, errA := strconv.ParseFloat(fmt.Sprint(actual), 64)
		expectedFloat, errB := strconv.ParseFloat(fmt.Sprint(expected), 64)
		if errA != nil || errB != nil {
			return false
		}
		switch strings.ToLower(op) {
		case "gt":
			return actualFloat > expectedFloat
		case "gte":
			return actualFloat >= expectedFloat
		case "lt":
			return actualFloat < expectedFloat
		case "lte":
			return actualFloat <= expectedFloat
		}
	}
	return false
}

func (s *AutomationEngineService) executeActions(ctx context.Context, rule *model.AutomationRule, event AutomationEvent, actions []automationAction) ([]AutomationActionOutcome, error) {
	outcomes := make([]AutomationActionOutcome, 0, len(actions))
	for _, action := range actions {
		outcome, err := s.executeAction(ctx, rule, event, action)
		if outcome != nil {
			outcomes = append(outcomes, *outcome)
		}
		if err != nil {
			return outcomes, err
		}
	}
	return outcomes, nil
}

func (s *AutomationEngineService) executeAction(ctx context.Context, rule *model.AutomationRule, event AutomationEvent, action automationAction) (*AutomationActionOutcome, error) {
	switch action.Type {
	case "update_field":
		return nil, s.executeUpdateField(ctx, event, action.Config)
	case "assign_user":
		return nil, s.executeAssignUser(ctx, event, action.Config)
	case "send_notification":
		return nil, s.executeSendNotification(ctx, event, action.Config)
	case "move_to_column":
		return nil, s.executeMoveToColumn(ctx, event, action.Config)
	case "create_subtask":
		return nil, s.executeCreateSubtask(ctx, event, action.Config)
	case "send_im_message":
		return nil, s.executeSendIMMessage(ctx, event, action.Config)
	case "invoke_plugin":
		return nil, s.executeInvokePlugin(ctx, event, action.Config)
	case "start_workflow":
		return s.executeStartWorkflow(ctx, rule, event, action.Config)
	default:
		return nil, fmt.Errorf("unsupported automation action %q", action.Type)
	}
}

func (s *AutomationEngineService) executeUpdateField(ctx context.Context, event AutomationEvent, config map[string]any) error {
	if event.TaskID == nil {
		return fmt.Errorf("update_field requires task context")
	}
	field := stringValue(config["field"])
	value := config["value"]
	switch {
	case field == "status":
		return s.tasks.TransitionStatus(ctx, *event.TaskID, fmt.Sprint(value))
	case field == "priority":
		val := fmt.Sprint(value)
		return s.tasks.Update(ctx, *event.TaskID, &model.UpdateTaskRequest{Priority: &val})
	case strings.HasPrefix(field, "cf:"):
		fieldID, err := uuid.Parse(strings.TrimPrefix(field, "cf:"))
		if err != nil {
			return err
		}
		return s.customFields.SetValue(ctx, &model.CustomFieldValue{
			ID:         uuid.New(),
			TaskID:     *event.TaskID,
			FieldDefID: fieldID,
			Value:      marshalAutomationValue(value),
			CreatedAt:  s.now(),
			UpdatedAt:  s.now(),
		})
	default:
		return fmt.Errorf("unsupported update_field target %q", field)
	}
}

func (s *AutomationEngineService) executeAssignUser(ctx context.Context, event AutomationEvent, config map[string]any) error {
	if event.TaskID == nil {
		return fmt.Errorf("assign_user requires task context")
	}
	assigneeID, err := uuid.Parse(stringValue(config["assigneeId"]))
	if err != nil {
		return err
	}
	assigneeType := stringValue(config["assigneeType"])
	if assigneeType == "" {
		assigneeType = model.MemberTypeHuman
	}
	return s.tasks.UpdateAssignee(ctx, *event.TaskID, assigneeID, assigneeType)
}

func (s *AutomationEngineService) executeSendNotification(ctx context.Context, event AutomationEvent, config map[string]any) error {
	if s.notifications == nil {
		return nil
	}
	var targetID uuid.UUID
	if raw := stringValue(config["targetId"]); raw != "" {
		parsed, err := uuid.Parse(raw)
		if err != nil {
			return err
		}
		targetID = parsed
	} else if event.Task != nil && event.Task.AssigneeID != nil {
		targetID = *event.Task.AssigneeID
	} else {
		return nil
	}
	payload, _ := json.Marshal(map[string]any{"eventType": event.EventType, "taskId": stringPtrValue(event.TaskID)})
	_, err := s.notifications.Create(ctx, targetID, model.NotificationTypeAutomationAction, stringValue(config["title"]), stringValue(config["body"]), string(payload))
	return err
}

func (s *AutomationEngineService) executeMoveToColumn(ctx context.Context, event AutomationEvent, config map[string]any) error {
	if event.TaskID == nil {
		return fmt.Errorf("move_to_column requires task context")
	}
	return s.tasks.TransitionStatus(ctx, *event.TaskID, stringValue(config["status"]))
}

func (s *AutomationEngineService) executeCreateSubtask(ctx context.Context, event AutomationEvent, config map[string]any) error {
	if event.Task == nil {
		return fmt.Errorf("create_subtask requires task context")
	}
	parentID := event.Task.ID.String()
	description := stringValue(config["description"])
	priority := stringValue(config["priority"])
	if priority == "" {
		priority = event.Task.Priority
	}
	task := &model.Task{
		ID:          uuid.New(),
		ProjectID:   event.Task.ProjectID,
		Title:       stringValue(config["title"]),
		Description: description,
		Status:      model.TaskStatusInbox,
		Priority:    priority,
		CreatedAt:   s.now(),
		UpdatedAt:   s.now(),
	}
	if parentUUID, err := uuid.Parse(parentID); err == nil {
		task.ParentID = &parentUUID
	}
	return s.tasks.Create(ctx, task)
}

func (s *AutomationEngineService) executeSendIMMessage(ctx context.Context, event AutomationEvent, config map[string]any) error {
	if s.im == nil {
		return nil
	}
	platform := stringValue(config["platform"])
	channelID := stringValue(config["channelId"])
	text := renderAutomationTemplate(stringValue(config["text"]), event)

	channels := make([]*model.IMChannel, 0)
	if s.imChannels != nil {
		resolved, err := s.imChannels.ResolveChannelsForEvent(ctx, event.EventType, platform, channelID)
		if err != nil {
			return err
		}
		channels = resolved
	} else if strings.TrimSpace(platform) != "" && strings.TrimSpace(channelID) != "" {
		channels = append(channels, &model.IMChannel{
			Platform:  strings.TrimSpace(platform),
			ChannelID: strings.TrimSpace(channelID),
			Active:    true,
		})
	}

	if len(channels) == 0 {
		return fmt.Errorf("no usable IM route configured")
	}

	for _, channel := range channels {
		if channel == nil {
			continue
		}
		if err := s.im.Send(ctx, &model.IMSendRequest{
			Platform:  strings.TrimSpace(channel.Platform),
			ChannelID: strings.TrimSpace(channel.ChannelID),
			Text:      text,
			ProjectID: event.ProjectID.String(),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *AutomationEngineService) executeInvokePlugin(ctx context.Context, event AutomationEvent, config map[string]any) error {
	if s.plugins == nil {
		return nil
	}
	_, err := s.plugins.Invoke(ctx, stringValue(config["pluginId"]), stringValue(config["operation"]), mapValue(config["input"]))
	return err
}

func (s *AutomationEngineService) executeStartWorkflow(
	ctx context.Context,
	rule *model.AutomationRule,
	event AutomationEvent,
	config map[string]any,
) (*AutomationActionOutcome, error) {
	pluginID := stringValue(config["pluginId"])
	if pluginID == "" {
		return &AutomationActionOutcome{
			Type:       "start_workflow",
			Outcome:    "failed",
			ReasonCode: "missing_plugin_id",
			Reason:     "start_workflow requires pluginId",
		}, fmt.Errorf("start_workflow requires pluginId")
	}
	if s.workflows == nil {
		return &AutomationActionOutcome{
			Type:       "start_workflow",
			Outcome:    "failed",
			ReasonCode: "workflow_runtime_unavailable",
			Reason:     "workflow starter is not configured",
			PluginID:   pluginID,
		}, fmt.Errorf("workflow starter is not configured")
	}
	duplicate, err := s.hasActiveAutomationWorkflowRun(ctx, pluginID, rule, event)
	if err != nil {
		return &AutomationActionOutcome{
			Type:       "start_workflow",
			Outcome:    "failed",
			ReasonCode: "duplicate_check_failed",
			Reason:     err.Error(),
			PluginID:   pluginID,
		}, err
	}
	if duplicate {
		return &AutomationActionOutcome{
			Type:       "start_workflow",
			Outcome:    "blocked",
			ReasonCode: "duplicate_active_run",
			Reason:     "an equivalent automation-triggered workflow run is already active",
			PluginID:   pluginID,
		}, nil
	}

	trigger := cloneWorkflowPayload(triggerObjectValue(config["trigger"]))
	if trigger == nil {
		trigger = map[string]any{}
	}
	trigger["source"] = "automation_rule"
	trigger["eventType"] = event.EventType
	trigger["projectId"] = event.ProjectID.String()
	if rule != nil {
		trigger["ruleId"] = rule.ID.String()
	}
	if event.TaskID != nil {
		trigger["taskId"] = event.TaskID.String()
	}
	if event.Task != nil {
		trigger["task"] = map[string]any{
			"id":     event.Task.ID.String(),
			"title":  event.Task.Title,
			"status": event.Task.Status,
		}
	}
	for key, value := range cloneWorkflowPayload(event.Data) {
		if _, exists := trigger[key]; exists {
			continue
		}
		trigger[key] = value
	}

	run, err := s.workflows.Start(ctx, pluginID, WorkflowExecutionRequest{Trigger: trigger})
	if err != nil {
		return &AutomationActionOutcome{
			Type:       "start_workflow",
			Outcome:    "failed",
			ReasonCode: "workflow_start_failed",
			Reason:     err.Error(),
			PluginID:   pluginID,
		}, err
	}
	return &AutomationActionOutcome{
		Type:     "start_workflow",
		Outcome:  "started",
		PluginID: pluginID,
		RunID:    run.ID.String(),
	}, nil
}

func (s *AutomationEngineService) writeAutomationLog(ctx context.Context, rule *model.AutomationRule, event AutomationEvent, status string, detail map[string]any) {
	if s.logs == nil || rule == nil {
		return
	}
	raw, _ := json.Marshal(detail)
	_ = s.logs.Create(ctx, &model.AutomationLog{
		ID:          uuid.New(),
		RuleID:      rule.ID,
		TaskID:      event.TaskID,
		EventType:   event.EventType,
		TriggeredAt: s.now(),
		Status:      status,
		Detail:      string(raw),
	})
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func mapValue(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return map[string]any{"value": value}
}

func triggerObjectValue(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	return nil
}

func marshalAutomationValue(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("%q", fmt.Sprint(value))
	}
	return string(raw)
}

func renderAutomationTemplate(template string, event AutomationEvent) string {
	result := template
	if event.Task != nil {
		result = strings.ReplaceAll(result, "{{task.id}}", event.Task.ID.String())
		result = strings.ReplaceAll(result, "{{task.title}}", event.Task.Title)
		result = strings.ReplaceAll(result, "{{task.status}}", event.Task.Status)
	}
	result = strings.ReplaceAll(result, "{{event.type}}", event.EventType)
	return result
}

func defaultJSON(raw string, fallback string) string {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	return raw
}

func stringPtrValue(value *uuid.UUID) string {
	if value == nil {
		return ""
	}
	return value.String()
}

func (s *AutomationEngineService) hasActiveAutomationWorkflowRun(
	ctx context.Context,
	pluginID string,
	rule *model.AutomationRule,
	event AutomationEvent,
) (bool, error) {
	runs, err := s.workflows.ListRuns(ctx, pluginID, 50)
	if err != nil {
		return false, err
	}
	projectID := event.ProjectID.String()
	taskID := stringPtrValue(event.TaskID)
	ruleID := ""
	if rule != nil {
		ruleID = rule.ID.String()
	}
	for _, run := range runs {
		if run == nil || !isWorkflowRunActive(run.Status) {
			continue
		}
		if workflowTriggerString(run.Trigger, "source") != "automation_rule" {
			continue
		}
		if workflowTriggerString(run.Trigger, "projectId") != projectID {
			continue
		}
		if workflowTriggerString(run.Trigger, "eventType") != event.EventType {
			continue
		}
		if ruleID != "" && workflowTriggerString(run.Trigger, "ruleId") != ruleID {
			continue
		}
		if taskID == "" {
			if workflowTriggerString(run.Trigger, "taskId") != "" {
				continue
			}
		} else if workflowTriggerString(run.Trigger, "taskId") != taskID {
			continue
		}
		return true, nil
	}
	return false, nil
}

func (s *AutomationEvaluationSummary) addActionOutcomes(outcomes []AutomationActionOutcome) {
	for _, outcome := range outcomes {
		switch outcome.Outcome {
		case "started":
			s.StartedWorkflows++
		case "blocked":
			s.BlockedWorkflows++
		case "failed":
			s.FailedWorkflows++
		}
	}
}
