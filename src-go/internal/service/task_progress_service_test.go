package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	eventbus "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type mockTaskProgressTaskRepo struct {
	tasks map[uuid.UUID]*model.Task
}

func (m *mockTaskProgressTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	task, ok := m.tasks[id]
	if !ok {
		return nil, service.ErrTaskNotFound
	}
	cloned := *task
	return &cloned, nil
}

func (m *mockTaskProgressTaskRepo) ListOpenForProgress(_ context.Context) ([]*model.Task, error) {
	result := make([]*model.Task, 0, len(m.tasks))
	for _, task := range m.tasks {
		cloned := *task
		result = append(result, &cloned)
	}
	return result, nil
}

type mockTaskProgressSnapshotRepo struct {
	snapshots map[uuid.UUID]*model.TaskProgressSnapshot
	saved     []*model.TaskProgressSnapshot
}

func newMockTaskProgressSnapshotRepo() *mockTaskProgressSnapshotRepo {
	return &mockTaskProgressSnapshotRepo{snapshots: make(map[uuid.UUID]*model.TaskProgressSnapshot)}
}

func (m *mockTaskProgressSnapshotRepo) GetByTaskID(_ context.Context, taskID uuid.UUID) (*model.TaskProgressSnapshot, error) {
	snapshot, ok := m.snapshots[taskID]
	if !ok {
		return nil, service.ErrTaskProgressSnapshotNotFound
	}
	cloned := *snapshot
	return &cloned, nil
}

func (m *mockTaskProgressSnapshotRepo) Upsert(_ context.Context, snapshot *model.TaskProgressSnapshot) error {
	cloned := *snapshot
	m.snapshots[snapshot.TaskID] = &cloned
	m.saved = append(m.saved, &cloned)
	return nil
}

type mockTaskProgressNotifications struct {
	created []string
}

func (m *mockTaskProgressNotifications) Create(_ context.Context, _ uuid.UUID, ntype, title, body, _ string) (*model.Notification, error) {
	m.created = append(m.created, ntype+"|"+title+"|"+body)
	return &model.Notification{
		ID:        uuid.New(),
		TargetID:  uuid.New(),
		Type:      ntype,
		Title:     title,
		Body:      body,
		CreatedAt: time.Now(),
	}, nil
}

type taskProgressBusCapture struct {
	mu     sync.Mutex
	events []*eventbus.Event
}

func (c *taskProgressBusCapture) Publish(_ context.Context, e *eventbus.Event) error {
	cloned := *e
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, &cloned)
	return nil
}

func (c *taskProgressBusCapture) next(t *testing.T) *eventbus.Event {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.events) == 0 {
		t.Fatal("expected at least one event, got none")
	}
	head := c.events[0]
	c.events = c.events[1:]
	return head
}

func TestTaskProgressService_EvaluateTaskEmitsStalledAlertOnlyOnceUntilRecovery(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	memberID := uuid.New()
	baseTime := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)

	taskRepo := &mockTaskProgressTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {
				ID:         taskID,
				ProjectID:  projectID,
				Title:      "Investigate stalled queue",
				Status:     model.TaskStatusInProgress,
				AssigneeID: &memberID,
				CreatedAt:  baseTime.Add(-6 * time.Hour),
				UpdatedAt:  baseTime.Add(-6 * time.Hour),
			},
		},
	}
	snapshotRepo := newMockTaskProgressSnapshotRepo()
	snapshotRepo.snapshots[taskID] = &model.TaskProgressSnapshot{
		TaskID:             taskID,
		LastActivityAt:     baseTime.Add(-5 * time.Hour),
		LastActivitySource: model.TaskProgressSourceTaskUpdated,
		LastTransitionAt:   baseTime.Add(-5 * time.Hour),
		HealthStatus:       model.TaskProgressHealthHealthy,
	}
	notifications := &mockTaskProgressNotifications{}
	capture := &taskProgressBusCapture{}

	svc := service.NewTaskProgressService(
		taskRepo,
		snapshotRepo,
		notifications,
		ws.NewHub(),
		capture,
		service.TaskProgressConfig{
			WarningAfter:   2 * time.Hour,
			StalledAfter:   4 * time.Hour,
			AlertCooldown:  30 * time.Minute,
			ExemptStatuses: []string{model.TaskStatusBlocked, model.TaskStatusDone, model.TaskStatusCancelled},
		},
		func() time.Time { return baseTime },
	)

	result, err := svc.EvaluateTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("EvaluateTask() error: %v", err)
	}
	if result.HealthStatus != model.TaskProgressHealthStalled {
		t.Fatalf("expected stalled health, got %s", result.HealthStatus)
	}
	if len(notifications.created) != 1 {
		t.Fatalf("expected one notification, got %d", len(notifications.created))
	}
	firstEvent := capture.next(t)
	if firstEvent.Type != ws.EventTaskProgressUpdated {
		t.Fatalf("expected first progress event, got %s", firstEvent.Type)
	}

	if _, err := svc.EvaluateTask(context.Background(), taskID); err != nil {
		t.Fatalf("second EvaluateTask() error: %v", err)
	}
	if len(notifications.created) != 1 {
		t.Fatalf("expected stalled alert to be deduplicated, got %d notifications", len(notifications.created))
	}

	recovered, err := svc.RecordActivity(
		context.Background(),
		taskID,
		service.TaskActivityInput{
			Source:       model.TaskProgressSourceAgentHeartbeat,
			OccurredAt:   baseTime.Add(10 * time.Minute),
			UpdateHealth: true,
		},
	)
	if err != nil {
		t.Fatalf("RecordActivity() error: %v", err)
	}
	if recovered.HealthStatus != model.TaskProgressHealthHealthy {
		t.Fatalf("expected recovery to healthy, got %s", recovered.HealthStatus)
	}
	if len(notifications.created) != 2 {
		t.Fatalf("expected recovery notification, got %d notifications", len(notifications.created))
	}
}

func TestTaskProgressService_EvaluateTaskMarksUnassignedWorkAsWarning(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	baseTime := time.Date(2026, 3, 24, 9, 0, 0, 0, time.UTC)

	taskRepo := &mockTaskProgressTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {
				ID:        taskID,
				ProjectID: projectID,
				Title:     "Triage orphaned task",
				Status:    model.TaskStatusTriaged,
				CreatedAt: baseTime.Add(-3 * time.Hour),
				UpdatedAt: baseTime.Add(-3 * time.Hour),
			},
		},
	}
	snapshotRepo := newMockTaskProgressSnapshotRepo()
	notifications := &mockTaskProgressNotifications{}

	svc := service.NewTaskProgressService(
		taskRepo,
		snapshotRepo,
		notifications,
		ws.NewHub(),
		nil,
		service.TaskProgressConfig{
			WarningAfter:   2 * time.Hour,
			StalledAfter:   4 * time.Hour,
			AlertCooldown:  30 * time.Minute,
			ExemptStatuses: []string{model.TaskStatusBlocked, model.TaskStatusDone, model.TaskStatusCancelled},
		},
		func() time.Time { return baseTime },
	)

	result, err := svc.EvaluateTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("EvaluateTask() error: %v", err)
	}
	if result.HealthStatus != model.TaskProgressHealthWarning {
		t.Fatalf("expected warning health, got %s", result.HealthStatus)
	}
	if result.RiskReason != model.TaskProgressReasonNoAssignee {
		t.Fatalf("expected no_assignee risk, got %s", result.RiskReason)
	}
	if result.RiskSinceAt == nil {
		t.Fatal("expected risk_since_at to be populated")
	}
	if len(notifications.created) != 0 {
		t.Fatalf("expected no persisted notifications without recipients, got %d", len(notifications.created))
	}
}
