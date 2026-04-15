package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type stubAutomationRuleRepo struct{ rules []*model.AutomationRule }

func (r *stubAutomationRuleRepo) ListByProjectAndEvent(_ context.Context, projectID uuid.UUID, eventType string) ([]*model.AutomationRule, error) {
	result := make([]*model.AutomationRule, 0)
	for _, rule := range r.rules {
		if rule.ProjectID == projectID && rule.EventType == eventType {
			result = append(result, rule)
		}
	}
	return result, nil
}

type stubAutomationLogRepo struct{ entries []*model.AutomationLog }

func (r *stubAutomationLogRepo) Create(_ context.Context, entry *model.AutomationLog) error {
	r.entries = append(r.entries, entry)
	return nil
}

type stubAutomationTaskRepo struct {
	created        *model.Task
	updated        *model.UpdateTaskRequest
	transitionedTo string
	assignedTo     uuid.UUID
	assignedType   string
	openTasks      []*model.Task
	task           *model.Task
}

func (r *stubAutomationTaskRepo) Create(_ context.Context, task *model.Task) error {
	r.created = task
	return nil
}
func (r *stubAutomationTaskRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Task, error) {
	return r.task, nil
}
func (r *stubAutomationTaskRepo) Update(_ context.Context, _ uuid.UUID, req *model.UpdateTaskRequest) error {
	r.updated = req
	return nil
}
func (r *stubAutomationTaskRepo) TransitionStatus(_ context.Context, _ uuid.UUID, status string) error {
	r.transitionedTo = status
	return nil
}
func (r *stubAutomationTaskRepo) UpdateAssignee(_ context.Context, _ uuid.UUID, assigneeID uuid.UUID, assigneeType string) error {
	r.assignedTo = assigneeID
	r.assignedType = assigneeType
	return nil
}
func (r *stubAutomationTaskRepo) ListOpenForProgress(_ context.Context) ([]*model.Task, error) {
	return r.openTasks, nil
}

type stubAutomationFieldRepo struct{ value *model.CustomFieldValue }

func (r *stubAutomationFieldRepo) SetValue(_ context.Context, value *model.CustomFieldValue) error {
	r.value = value
	return nil
}

type stubAutomationNotifications struct {
	title string
	body  string
}

func (n *stubAutomationNotifications) Create(_ context.Context, _ uuid.UUID, _, title, body, _ string) (*model.Notification, error) {
	n.title, n.body = title, body
	return &model.Notification{}, nil
}

type stubAutomationIM struct{ sent *model.IMSendRequest }

func (i *stubAutomationIM) Send(_ context.Context, req *model.IMSendRequest) error {
	i.sent = req
	return nil
}

type stubAutomationPlugin struct {
	pluginID  string
	operation string
	payload   map[string]any
}

func (p *stubAutomationPlugin) Invoke(_ context.Context, pluginID, operation string, payload map[string]any) (map[string]any, error) {
	p.pluginID, p.operation, p.payload = pluginID, operation, payload
	return map[string]any{"ok": true}, nil
}

type stubAutomationWorkflowRunner struct {
	startedPlugin string
	startedReq    WorkflowExecutionRequest
	started       int
	runs          []*model.WorkflowPluginRun
	err           error
}

func (r *stubAutomationWorkflowRunner) Start(_ context.Context, pluginID string, req WorkflowExecutionRequest) (*model.WorkflowPluginRun, error) {
	r.startedPlugin = pluginID
	r.startedReq = req
	r.started++
	if r.err != nil {
		return nil, r.err
	}
	return &model.WorkflowPluginRun{
		ID:       uuid.New(),
		PluginID: pluginID,
		Status:   model.WorkflowRunStatusRunning,
		Trigger:  req.Trigger,
	}, nil
}

func (r *stubAutomationWorkflowRunner) ListRuns(_ context.Context, pluginID string, _ int) ([]*model.WorkflowPluginRun, error) {
	filtered := make([]*model.WorkflowPluginRun, 0, len(r.runs))
	for _, run := range r.runs {
		if run != nil && run.PluginID == pluginID {
			filtered = append(filtered, run)
		}
	}
	return filtered, nil
}

func TestAutomationEngineServiceExecutesActionsAndLogs(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Name:       "done automation",
		Enabled:    true,
		EventType:  model.AutomationEventTaskStatusChanged,
		Conditions: `[{"field":"status","op":"eq","value":"done"}]`,
		Actions:    `[{"type":"update_field","config":{"field":"priority","value":"critical"}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Ship", Status: model.TaskStatusDone, Priority: "high"}}
	fields := &stubAutomationFieldRepo{}
	engine := NewAutomationEngineService(&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}}, logs, tasks, fields, nil, nil, nil)
	engine.now = func() time.Time { return time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC) }

	if err := engine.EvaluateRules(context.Background(), AutomationEvent{
		EventType: model.AutomationEventTaskStatusChanged,
		ProjectID: projectID,
		TaskID:    &taskID,
		Task:      tasks.task,
		Data:      map[string]any{"status": "done"},
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}
	if tasks.updated == nil || tasks.updated.Priority == nil || *tasks.updated.Priority != "critical" {
		t.Fatalf("priority update = %+v", tasks.updated)
	}
	if len(logs.entries) != 1 || logs.entries[0].Status != model.AutomationLogStatusSuccess {
		t.Fatalf("logs = %+v", logs.entries)
	}
}

func TestAutomationEngineServiceExecutesNotificationIMAndPluginActions(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	targetID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Enabled:    true,
		EventType:  model.AutomationEventTaskStatusChanged,
		Conditions: `[]`,
		Actions:    `[{"type":"send_notification","config":{"title":"Done","body":"Task finished","targetId":"` + targetID.String() + `"}},{"type":"send_im_message","config":{"platform":"slack","channelId":"C1","text":"{{task.title}} done"}},{"type":"invoke_plugin","config":{"pluginId":"plugin.test","operation":"notify","input":{"task":"x"}}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Ship", Status: model.TaskStatusDone}}
	notifs := &stubAutomationNotifications{}
	im := &stubAutomationIM{}
	plugins := &stubAutomationPlugin{}
	engine := NewAutomationEngineService(&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}}, logs, tasks, nil, notifs, im, plugins)

	if err := engine.EvaluateRules(context.Background(), AutomationEvent{
		EventType: model.AutomationEventTaskStatusChanged,
		ProjectID: projectID,
		TaskID:    &taskID,
		Task:      tasks.task,
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}
	if notifs.title != "Done" {
		t.Fatalf("notification title = %q", notifs.title)
	}
	if im.sent == nil || im.sent.Text != "Ship done" {
		t.Fatalf("im send = %+v", im.sent)
	}
	if plugins.pluginID != "plugin.test" || plugins.operation != "notify" {
		t.Fatalf("plugin invoke = %s/%s", plugins.pluginID, plugins.operation)
	}
}

func TestAutomationEngineServiceSendIMMessageUsesConfiguredChannelResolver(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Enabled:    true,
		EventType:  model.AutomationEventTaskStatusChanged,
		Conditions: `[]`,
		Actions:    `[{"type":"send_im_message","config":{"text":"{{task.title}} done"}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Ship", Status: model.TaskStatusDone}}
	im := &stubAutomationIM{}
	engine := NewAutomationEngineService(&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}}, logs, tasks, nil, nil, im, nil)
	engine.SetIMChannelResolver(&stubIMEventChannelResolver{
		channels: []*model.IMChannel{{
			Platform:  "slack",
			Name:      "Automation",
			ChannelID: "C-automation",
			Events:    []string{model.AutomationEventTaskStatusChanged},
			Active:    true,
		}},
	})

	if err := engine.EvaluateRules(context.Background(), AutomationEvent{
		EventType: model.AutomationEventTaskStatusChanged,
		ProjectID: projectID,
		TaskID:    &taskID,
		Task:      tasks.task,
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}
	if im.sent == nil {
		t.Fatal("expected automation IM send request")
	}
	if im.sent.Platform != "slack" || im.sent.ChannelID != "C-automation" || im.sent.Text != "Ship done" {
		t.Fatalf("im send = %+v", im.sent)
	}
}

func TestAutomationEngineServiceSendIMMessageFailsExplicitlyWithoutUsableRoute(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Enabled:    true,
		EventType:  model.AutomationEventTaskStatusChanged,
		Conditions: `[]`,
		Actions:    `[{"type":"send_im_message","config":{"text":"{{task.title}} done"}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Ship", Status: model.TaskStatusDone}}
	im := &stubAutomationIM{}
	engine := NewAutomationEngineService(&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}}, logs, tasks, nil, nil, im, nil)
	engine.SetIMChannelResolver(&stubIMEventChannelResolver{})

	if err := engine.EvaluateRules(context.Background(), AutomationEvent{
		EventType: model.AutomationEventTaskStatusChanged,
		ProjectID: projectID,
		TaskID:    &taskID,
		Task:      tasks.task,
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}
	if im.sent != nil {
		t.Fatalf("unexpected IM send = %+v", im.sent)
	}
	if len(logs.entries) != 1 || logs.entries[0].Status != model.AutomationLogStatusFailed {
		t.Fatalf("logs = %+v", logs.entries)
	}
}

func TestAutomationEngineServiceSupportsAssignSubtaskAndCustomField(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	fieldID := uuid.New()
	assigneeID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Enabled:    true,
		EventType:  model.AutomationEventTaskFieldChanged,
		Conditions: `[]`,
		Actions:    `[{"type":"assign_user","config":{"assigneeId":"` + assigneeID.String() + `","assigneeType":"agent"}},{"type":"create_subtask","config":{"title":"Follow-up","description":"child","priority":"low"}},{"type":"update_field","config":{"field":"cf:` + fieldID.String() + `","value":"P0"}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Parent", Priority: "high"}}
	fields := &stubAutomationFieldRepo{}
	engine := NewAutomationEngineService(&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}}, logs, tasks, fields, nil, nil, nil)

	if err := engine.EvaluateRules(context.Background(), AutomationEvent{
		EventType: model.AutomationEventTaskFieldChanged,
		ProjectID: projectID,
		TaskID:    &taskID,
		Task:      tasks.task,
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}
	if tasks.assignedTo != assigneeID || tasks.assignedType != model.MemberTypeAgent {
		t.Fatalf("assign action = %s/%s", tasks.assignedTo, tasks.assignedType)
	}
	if tasks.created == nil || tasks.created.ParentID == nil || *tasks.created.ParentID != taskID {
		t.Fatalf("subtask create = %+v", tasks.created)
	}
	if fields.value == nil || fields.value.FieldDefID != fieldID {
		t.Fatalf("custom field action = %+v", fields.value)
	}
}

func TestAutomationEngineServiceSkipsTriggeredByAutomationAndChecksDueDates(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	now := time.Date(2026, 3, 27, 12, 0, 0, 0, time.UTC)
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Enabled:    true,
		EventType:  model.AutomationEventTaskDueDateApproach,
		Conditions: `[]`,
		Actions:    `[{"type":"move_to_column","config":{"status":"blocked"}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{
		openTasks: []*model.Task{{ID: taskID, ProjectID: projectID, Status: model.TaskStatusInProgress, PlannedEndAt: ptrTime(now.Add(30 * time.Minute))}},
	}
	engine := NewAutomationEngineService(&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}}, logs, tasks, nil, nil, nil, nil)
	engine.now = func() time.Time { return now }

	if err := engine.EvaluateRules(context.Background(), AutomationEvent{
		EventType:             model.AutomationEventTaskDueDateApproach,
		ProjectID:             projectID,
		TaskID:                &taskID,
		TriggeredByAutomation: true,
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}
	if len(logs.entries) != 0 {
		t.Fatalf("expected no logs for automation-triggered event, got %+v", logs.entries)
	}

	summary, err := engine.CheckDueDateApproaching(context.Background(), time.Hour)
	if err != nil {
		t.Fatalf("CheckDueDateApproaching() error = %v", err)
	}
	if summary == nil || summary.EvaluatedTasks != 1 || summary.MatchedRules != 1 {
		t.Fatalf("due-date summary = %+v, want evaluated task and matched rule counts", summary)
	}
	if tasks.transitionedTo != "blocked" {
		t.Fatalf("due-date action transition = %q, want blocked", tasks.transitionedTo)
	}
}

func TestAutomationEngineServiceStartsWorkflowThroughCanonicalRuntimeAndLogsOutcome(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Enabled:    true,
		EventType:  model.AutomationEventTaskDueDateApproach,
		Conditions: `[]`,
		Actions:    `[{"type":"start_workflow","config":{"pluginId":"task-delivery-flow","trigger":{"from":"automation"}}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Ship", Status: model.TaskStatusBlocked}}
	workflows := &stubAutomationWorkflowRunner{}
	engine := NewAutomationEngineService(&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}}, logs, tasks, nil, nil, nil, nil)
	engine.SetWorkflowStarter(workflows)

	if err := engine.EvaluateRules(context.Background(), AutomationEvent{
		EventType: model.AutomationEventTaskDueDateApproach,
		ProjectID: projectID,
		TaskID:    &taskID,
		Task:      tasks.task,
		Data:      map[string]any{"due_at": "2026-04-16T09:00:00Z"},
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}
	if workflows.started != 1 || workflows.startedPlugin != "task-delivery-flow" {
		t.Fatalf("workflow starts = %d via %q", workflows.started, workflows.startedPlugin)
	}
	if got := workflows.startedReq.Trigger["source"]; got != "automation_rule" {
		t.Fatalf("workflow trigger source = %v, want automation_rule", got)
	}
	if got := workflows.startedReq.Trigger["taskId"]; got != taskID.String() {
		t.Fatalf("workflow trigger taskId = %v, want %s", got, taskID.String())
	}
	if got := workflows.startedReq.Trigger["ruleId"]; got != rule.ID.String() {
		t.Fatalf("workflow trigger ruleId = %v, want %s", got, rule.ID.String())
	}
	if got := workflows.startedReq.Trigger["from"]; got != "automation" {
		t.Fatalf("workflow trigger custom field = %v, want automation", got)
	}
	if len(logs.entries) != 1 || logs.entries[0].Status != model.AutomationLogStatusSuccess {
		t.Fatalf("logs = %+v", logs.entries)
	}
	var detail struct {
		ActionOutcomes []AutomationActionOutcome `json:"actionOutcomes"`
	}
	if err := json.Unmarshal([]byte(logs.entries[0].Detail), &detail); err != nil {
		t.Fatalf("unmarshal automation log detail: %v", err)
	}
	if len(detail.ActionOutcomes) != 1 || detail.ActionOutcomes[0].Outcome != "started" || detail.ActionOutcomes[0].RunID == "" {
		t.Fatalf("action outcomes = %+v", detail.ActionOutcomes)
	}
}

func TestAutomationEngineServiceBlocksDuplicateWorkflowStartsAndDoesNotCreateSecondRun(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	rule := &model.AutomationRule{
		ID:         uuid.New(),
		ProjectID:  projectID,
		Enabled:    true,
		EventType:  model.AutomationEventTaskDueDateApproach,
		Conditions: `[]`,
		Actions:    `[{"type":"start_workflow","config":{"pluginId":"task-delivery-flow"}}]`,
	}
	logs := &stubAutomationLogRepo{}
	tasks := &stubAutomationTaskRepo{task: &model.Task{ID: taskID, ProjectID: projectID, Title: "Ship", Status: model.TaskStatusBlocked}}
	workflows := &stubAutomationWorkflowRunner{
		runs: []*model.WorkflowPluginRun{{
			ID:       uuid.New(),
			PluginID: "task-delivery-flow",
			Status:   model.WorkflowRunStatusRunning,
			Trigger: map[string]any{
				"source":    "automation_rule",
				"projectId": projectID.String(),
				"taskId":    taskID.String(),
				"eventType": model.AutomationEventTaskDueDateApproach,
				"ruleId":    rule.ID.String(),
			},
		}},
	}
	engine := NewAutomationEngineService(&stubAutomationRuleRepo{rules: []*model.AutomationRule{rule}}, logs, tasks, nil, nil, nil, nil)
	engine.SetWorkflowStarter(workflows)

	if err := engine.EvaluateRules(context.Background(), AutomationEvent{
		EventType: model.AutomationEventTaskDueDateApproach,
		ProjectID: projectID,
		TaskID:    &taskID,
		Task:      tasks.task,
	}); err != nil {
		t.Fatalf("EvaluateRules() error = %v", err)
	}
	if workflows.started != 0 {
		t.Fatalf("workflow starts = %d, want duplicate block before start", workflows.started)
	}
	if len(logs.entries) != 1 || logs.entries[0].Status != model.AutomationLogStatusSuccess {
		t.Fatalf("logs = %+v", logs.entries)
	}
	var detail struct {
		ActionOutcomes []AutomationActionOutcome `json:"actionOutcomes"`
	}
	if err := json.Unmarshal([]byte(logs.entries[0].Detail), &detail); err != nil {
		t.Fatalf("unmarshal automation log detail: %v", err)
	}
	if len(detail.ActionOutcomes) != 1 || detail.ActionOutcomes[0].Outcome != "blocked" || detail.ActionOutcomes[0].ReasonCode != "duplicate_active_run" {
		t.Fatalf("action outcomes = %+v", detail.ActionOutcomes)
	}
}
