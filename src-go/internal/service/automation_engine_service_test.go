package service

import (
	"context"
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

	if err := engine.CheckDueDateApproaching(context.Background(), time.Hour); err != nil {
		t.Fatalf("CheckDueDateApproaching() error = %v", err)
	}
	if tasks.transitionedTo != "blocked" {
		t.Fatalf("due-date action transition = %q, want blocked", tasks.transitionedTo)
	}
}
