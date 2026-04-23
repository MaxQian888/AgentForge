package service

import (
	"context"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

type stubDashboardTaskRepo struct {
	tasks           []*model.Task
	tasksBySprint   map[uuid.UUID][]*model.Task
	completedCounts []repository.TaskDateCount
}

func (r *stubDashboardTaskRepo) List(_ context.Context, _ uuid.UUID, _ model.TaskListQuery) ([]*model.Task, int, error) {
	return r.tasks, len(r.tasks), nil
}
func (r *stubDashboardTaskRepo) ListBySprint(_ context.Context, _ uuid.UUID, sprintID uuid.UUID) ([]*model.Task, error) {
	return r.tasksBySprint[sprintID], nil
}
func (r *stubDashboardTaskRepo) CountCompletedByDateRange(_ context.Context, _, _ time.Time, _ *uuid.UUID) ([]repository.TaskDateCount, error) {
	return r.completedCounts, nil
}

type stubDashboardSprintRepo struct{ sprint *model.Sprint }

func (r *stubDashboardSprintRepo) GetByID(_ context.Context, _ uuid.UUID) (*model.Sprint, error) {
	return r.sprint, nil
}

type stubDashboardAgentRunRepo struct {
	runs        []*model.AgentRun
	projectCost *model.CostSummaryDTO
	performance []repository.AgentPerformanceRow
}

func (r *stubDashboardAgentRunRepo) ListByProject(_ context.Context, _ uuid.UUID) ([]*model.AgentRun, error) {
	return r.runs, nil
}
func (r *stubDashboardAgentRunRepo) AggregateByProject(_ context.Context, _ uuid.UUID) (*model.CostSummaryDTO, error) {
	return r.projectCost, nil
}
func (r *stubDashboardAgentRunRepo) AggregatePerformance(_ context.Context, _, _ time.Time, _ *uuid.UUID) ([]repository.AgentPerformanceRow, error) {
	return r.performance, nil
}

type stubDashboardCache struct{ store map[string]string }
type ttlCaptureDashboardCache struct {
	lastTTL time.Duration
	store   map[string]string
}

func (c *stubDashboardCache) GetWidgetData(_ context.Context, key string) (string, error) {
	value, ok := c.store[key]
	if !ok {
		return "", repository.ErrNotFound
	}
	return value, nil
}
func (c *stubDashboardCache) SetWidgetData(_ context.Context, key, payload string, _ time.Duration) error {
	if c.store == nil {
		c.store = map[string]string{}
	}
	c.store[key] = payload
	return nil
}

func (c *ttlCaptureDashboardCache) GetWidgetData(_ context.Context, key string) (string, error) {
	value, ok := c.store[key]
	if !ok {
		return "", repository.ErrNotFound
	}
	return value, nil
}

func (c *ttlCaptureDashboardCache) SetWidgetData(_ context.Context, key, payload string, ttl time.Duration) error {
	if c.store == nil {
		c.store = map[string]string{}
	}
	c.lastTTL = ttl
	c.store[key] = payload
	return nil
}

func TestDashboardWidgetServiceThroughputUsesCache(t *testing.T) {
	projectID := uuid.New()
	cache := &stubDashboardCache{}
	service := NewDashboardWidgetService(
		&stubDashboardTaskRepo{completedCounts: []repository.TaskDateCount{
			{Date: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC), Count: 2},
			{Date: time.Date(2026, 3, 26, 0, 0, 0, 0, time.UTC), Count: 3},
		}},
		&stubDashboardSprintRepo{},
		&stubDashboardAgentRunRepo{},
		cache,
	)
	service.now = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }

	payload, err := service.WidgetData(context.Background(), projectID, model.DashboardWidgetThroughputChart, `{"days":7}`)
	if err != nil {
		t.Fatalf("WidgetData() error = %v", err)
	}
	points, ok := payload["points"].([]map[string]any)
	if !ok || len(points) != 2 {
		t.Fatalf("unexpected throughput points: %#v", payload["points"])
	}

	cached, err := service.WidgetData(context.Background(), projectID, model.DashboardWidgetThroughputChart, `{"days":7}`)
	if err != nil {
		t.Fatalf("WidgetData() cached error = %v", err)
	}
	if cached["cached"] != true {
		t.Fatalf("expected cached payload, got %#v", cached)
	}
}

func TestDashboardWidgetServiceBurndownAndDerivedWidgets(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	now := time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC)
	doneAt := now.Add(-time.Hour)
	tasks := []*model.Task{
		{ID: uuid.New(), ProjectID: projectID, SprintID: &sprintID, Title: "Done", Status: model.TaskStatusDone, Priority: "high", BudgetUsd: 10, SpentUsd: 8, CreatedAt: now.Add(-72 * time.Hour), UpdatedAt: now, CompletedAt: &doneAt, PlannedEndAt: ptrTime(now.Add(24 * time.Hour))},
		{ID: uuid.New(), ProjectID: projectID, SprintID: &sprintID, Title: "Blocked", Status: model.TaskStatusBlocked, Priority: "medium", BudgetUsd: 12, SpentUsd: 5, CreatedAt: now.Add(-48 * time.Hour), UpdatedAt: now, PlannedEndAt: ptrTime(now.Add(-2 * time.Hour))},
		{ID: uuid.New(), ProjectID: projectID, Title: "Review", Status: model.TaskStatusInReview, Priority: "medium", BudgetUsd: 5, SpentUsd: 3, CreatedAt: now.Add(-24 * time.Hour), UpdatedAt: now},
	}
	service := NewDashboardWidgetService(
		&stubDashboardTaskRepo{tasks: tasks, tasksBySprint: map[uuid.UUID][]*model.Task{sprintID: tasks[:2]}},
		&stubDashboardSprintRepo{sprint: &model.Sprint{ID: sprintID, ProjectID: projectID, Name: "Sprint 7", StartDate: now.AddDate(0, 0, -7), EndDate: now.AddDate(0, 0, 7), Status: model.SprintStatusActive}},
		&stubDashboardAgentRunRepo{
			runs:        []*model.AgentRun{{ID: uuid.New(), TaskID: tasks[0].ID, RoleID: "agent.dev", CostUsd: 2.5, CreatedAt: now.Add(-24 * time.Hour)}},
			projectCost: &model.CostSummaryDTO{TotalCostUsd: 2.5},
			performance: []repository.AgentPerformanceRow{{RoleID: "agent.dev", TotalRuns: 1, CompletedRuns: 1, AvgCostUsd: 2.5, TotalCostUsd: 2.5}},
		},
		nil,
	)
	service.now = func() time.Time { return now }

	burndown, err := service.WidgetData(context.Background(), projectID, model.DashboardWidgetBurndown, `{"sprintId":"`+sprintID.String()+`"}`)
	if err != nil {
		t.Fatalf("burndown error = %v", err)
	}
	if _, ok := burndown["points"].([]map[string]any); !ok {
		t.Fatalf("unexpected burndown payload: %#v", burndown)
	}

	blockers, _ := service.WidgetData(context.Background(), projectID, model.DashboardWidgetBlockerCount, "")
	if blockers["count"] != 1 {
		t.Fatalf("blocker count = %#v", blockers["count"])
	}
	budget, _ := service.WidgetData(context.Background(), projectID, model.DashboardWidgetBudgetConsumption, "")
	if budget["allocated"] != 27.0 || budget["spent"] != 16.0 {
		t.Fatalf("budget payload = %#v", budget)
	}
	agentCost, _ := service.WidgetData(context.Background(), projectID, model.DashboardWidgetAgentCost, "")
	entries, ok := agentCost["entries"].([]map[string]any)
	if !ok || len(entries) != 1 || entries[0]["roleId"] != "agent.dev" {
		t.Fatalf("agent cost payload = %#v", agentCost)
	}
	reviewBacklog, _ := service.WidgetData(context.Background(), projectID, model.DashboardWidgetReviewBacklog, "")
	if reviewBacklog["count"] != 1 {
		t.Fatalf("review backlog = %#v", reviewBacklog["count"])
	}
	taskAging, _ := service.WidgetData(context.Background(), projectID, model.DashboardWidgetTaskAging, "")
	buckets, ok := taskAging["buckets"].([]map[string]any)
	if !ok || len(buckets) != 4 {
		t.Fatalf("task aging payload = %#v", taskAging)
	}
	sla, _ := service.WidgetData(context.Background(), projectID, model.DashboardWidgetSLACompliance, "")
	if sla["total"] != 2 {
		t.Fatalf("sla payload = %#v", sla)
	}
}

func TestDashboardWidgetService_UsesConfiguredCacheTTL(t *testing.T) {
	projectID := uuid.New()
	cache := &ttlCaptureDashboardCache{}
	service := NewDashboardWidgetService(
		&stubDashboardTaskRepo{completedCounts: []repository.TaskDateCount{{Date: time.Date(2026, 3, 25, 0, 0, 0, 0, time.UTC), Count: 1}}},
		&stubDashboardSprintRepo{},
		&stubDashboardAgentRunRepo{},
		cache,
	).WithCacheTTL(15 * time.Second)
	service.now = func() time.Time { return time.Date(2026, 3, 26, 12, 0, 0, 0, time.UTC) }

	if _, err := service.WidgetData(context.Background(), projectID, model.DashboardWidgetThroughputChart, `{"days":7}`); err != nil {
		t.Fatalf("WidgetData() error = %v", err)
	}
	if cache.lastTTL != 15*time.Second {
		t.Fatalf("cache ttl = %v, want 15s", cache.lastTTL)
	}
}

func ptrTime(value time.Time) *time.Time { return &value }
