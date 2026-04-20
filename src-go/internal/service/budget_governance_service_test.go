package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
)

type mockBudgetSprintReader struct {
	sprintsByID      map[uuid.UUID]*model.Sprint
	sprintsByProject map[uuid.UUID][]*model.Sprint
	getErr           error
	listErr          error
}

func (m *mockBudgetSprintReader) GetByID(_ context.Context, id uuid.UUID) (*model.Sprint, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	sprint, ok := m.sprintsByID[id]
	if !ok {
		return nil, errors.New("sprint not found")
	}
	cloned := *sprint
	return &cloned, nil
}

func (m *mockBudgetSprintReader) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.Sprint, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	sprints := m.sprintsByProject[projectID]
	out := make([]*model.Sprint, 0, len(sprints))
	for _, sprint := range sprints {
		cloned := *sprint
		out = append(out, &cloned)
	}
	return out, nil
}

type mockBudgetTaskReader struct {
	tasksByProject  map[uuid.UUID][]*model.Task
	tasksBySprint   map[uuid.UUID][]*model.Task
	listErr         error
	listBySprintErr error
}

func (m *mockBudgetTaskReader) List(_ context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	tasks := m.tasksByProject[projectID]
	cloned := make([]*model.Task, 0, len(tasks))
	for _, task := range tasks {
		if q.SprintID != "" {
			if task.SprintID == nil || task.SprintID.String() != q.SprintID {
				continue
			}
		}
		item := *task
		cloned = append(cloned, &item)
	}
	total := len(cloned)
	limit := q.Limit
	if limit <= 0 || limit > total {
		limit = total
	}
	page := q.Page
	if page <= 0 {
		page = 1
	}
	start := (page - 1) * limit
	if start >= total {
		return []*model.Task{}, total, nil
	}
	end := start + limit
	if end > total {
		end = total
	}
	return cloned[start:end], total, nil
}

func (m *mockBudgetTaskReader) ListBySprint(_ context.Context, projectID uuid.UUID, sprintID uuid.UUID) ([]*model.Task, error) {
	if m.listBySprintErr != nil {
		return nil, m.listBySprintErr
	}
	tasks := m.tasksBySprint[sprintID]
	cloned := make([]*model.Task, 0, len(tasks))
	for _, task := range tasks {
		if task.ProjectID != projectID {
			continue
		}
		item := *task
		cloned = append(cloned, &item)
	}
	return cloned, nil
}

func TestBudgetGovernanceService_CheckSprintBudget(t *testing.T) {
	sprintID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByID: map[uuid.UUID]*model.Sprint{
			sprintID: {
				ID:             sprintID,
				TotalBudgetUsd: 10,
				SpentUsd:       5,
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader)

	result, err := svc.CheckSprintBudget(context.Background(), sprintID, 3)
	if err != nil {
		t.Fatalf("CheckSprintBudget() error = %v", err)
	}
	if !result.Allowed || !result.Warning {
		t.Fatalf("expected warning result, got %+v", result)
	}
	if !strings.Contains(result.WarningMessage, "80% utilized") {
		t.Fatalf("WarningMessage = %q, want 80%% utilization", result.WarningMessage)
	}

	result, err = svc.CheckSprintBudget(context.Background(), sprintID, 6)
	if err != nil {
		t.Fatalf("CheckSprintBudget() error = %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected blocked result, got %+v", result)
	}
	if !strings.Contains(result.Reason, "sprint budget exceeded") {
		t.Fatalf("Reason = %q", result.Reason)
	}
}

func TestBudgetGovernanceService_CheckSprintBudgetAllowsWithoutConfiguredLimit(t *testing.T) {
	sprintID := uuid.New()
	svc := service.NewBudgetGovernanceService(&mockBudgetSprintReader{
		sprintsByID: map[uuid.UUID]*model.Sprint{
			sprintID: {ID: sprintID, TotalBudgetUsd: 0, SpentUsd: 99},
		},
	})

	result, err := svc.CheckSprintBudget(context.Background(), sprintID, 100)
	if err != nil {
		t.Fatalf("CheckSprintBudget() error = %v", err)
	}
	if !result.Allowed || result.Warning || result.Reason != "" {
		t.Fatalf("unexpected unconstrained result: %+v", result)
	}
}

func TestBudgetGovernanceService_CheckSprintBudgetReturnsReaderError(t *testing.T) {
	svc := service.NewBudgetGovernanceService(&mockBudgetSprintReader{getErr: errors.New("boom")})

	_, err := svc.CheckSprintBudget(context.Background(), uuid.New(), 1)
	if err == nil || !strings.Contains(err.Error(), "budget check: fetch sprint") {
		t.Fatalf("CheckSprintBudget() error = %v, want wrapped fetch error", err)
	}
}

func TestBudgetGovernanceService_CheckProjectBudget(t *testing.T) {
	projectID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByProject: map[uuid.UUID][]*model.Sprint{
			projectID: {
				{ID: uuid.New(), ProjectID: projectID, SpentUsd: 4.5},
				{ID: uuid.New(), ProjectID: projectID, SpentUsd: 3.0},
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader)
	svc.SetProjectBudgetLimit(10)

	result, err := svc.CheckProjectBudget(context.Background(), projectID, 0.5)
	if err != nil {
		t.Fatalf("CheckProjectBudget() error = %v", err)
	}
	if !result.Allowed || !result.Warning {
		t.Fatalf("expected project warning result, got %+v", result)
	}
	if !strings.Contains(result.WarningMessage, "80% utilized") {
		t.Fatalf("WarningMessage = %q, want 80%% utilization", result.WarningMessage)
	}

	result, err = svc.CheckProjectBudget(context.Background(), projectID, 3)
	if err != nil {
		t.Fatalf("CheckProjectBudget() error = %v", err)
	}
	if result.Allowed {
		t.Fatalf("expected blocked project result, got %+v", result)
	}
	if !strings.Contains(result.Reason, "project budget exceeded") {
		t.Fatalf("Reason = %q", result.Reason)
	}
}

func TestBudgetGovernanceService_CheckProjectBudgetAllowsWithoutLimitAndWrapsErrors(t *testing.T) {
	projectID := uuid.New()
	svc := service.NewBudgetGovernanceService(&mockBudgetSprintReader{})

	result, err := svc.CheckProjectBudget(context.Background(), projectID, 20)
	if err != nil {
		t.Fatalf("CheckProjectBudget() error = %v", err)
	}
	if !result.Allowed || result.Warning {
		t.Fatalf("unexpected unconstrained project result: %+v", result)
	}

	svc.SetProjectBudgetLimit(10)
	svc = service.NewBudgetGovernanceService(&mockBudgetSprintReader{listErr: errors.New("project read failed")})
	svc.SetProjectBudgetLimit(10)
	_, err = svc.CheckProjectBudget(context.Background(), projectID, 1)
	if err == nil || !strings.Contains(err.Error(), "budget check: list project sprints") {
		t.Fatalf("CheckProjectBudget() error = %v, want wrapped list error", err)
	}
}

func TestBudgetGovernanceService_CheckBudgetDelegatesSprintThenProject(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByID: map[uuid.UUID]*model.Sprint{
			sprintID: {
				ID:             sprintID,
				ProjectID:      projectID,
				TotalBudgetUsd: 10,
				SpentUsd:       7.5,
			},
		},
		sprintsByProject: map[uuid.UUID][]*model.Sprint{
			projectID: {
				{ID: sprintID, ProjectID: projectID, SpentUsd: 7.5},
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader)
	svc.SetProjectBudgetLimit(20)

	result, err := svc.CheckBudget(context.Background(), projectID, &sprintID, 0.5)
	if err != nil {
		t.Fatalf("CheckBudget() error = %v", err)
	}
	if !result.Allowed || !result.Warning {
		t.Fatalf("CheckBudget() = %+v, want sprint warning", result)
	}

	result, err = svc.CheckBudget(context.Background(), projectID, &sprintID, 3)
	if err != nil {
		t.Fatalf("CheckBudget() error = %v", err)
	}
	if result.Allowed || !strings.Contains(result.Reason, "sprint budget exceeded") {
		t.Fatalf("CheckBudget() = %+v, want sprint block", result)
	}

	result, err = svc.CheckBudget(context.Background(), projectID, nil, 13)
	if err != nil {
		t.Fatalf("CheckBudget() project-only error = %v", err)
	}
	if result.Allowed || !strings.Contains(result.Reason, "project budget exceeded") {
		t.Fatalf("CheckBudget() project-only = %+v, want project block", result)
	}
}

func TestBudgetGovernanceService_GetProjectBudgetSummaryAggregatesScopes(t *testing.T) {
	projectID := uuid.New()
	activeSprintID := uuid.New()
	closedSprintID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByProject: map[uuid.UUID][]*model.Sprint{
			projectID: {
				{
					ID:             activeSprintID,
					ProjectID:      projectID,
					Name:           "Active sprint",
					Status:         model.SprintStatusActive,
					TotalBudgetUsd: 10,
					SpentUsd:       8.5,
				},
				{
					ID:             closedSprintID,
					ProjectID:      projectID,
					Name:           "Closed sprint",
					Status:         model.SprintStatusClosed,
					TotalBudgetUsd: 4,
					SpentUsd:       2,
				},
			},
		},
	}
	taskReader := &mockBudgetTaskReader{
		tasksByProject: map[uuid.UUID][]*model.Task{
			projectID: {
				{ID: uuid.New(), ProjectID: projectID, SprintID: &activeSprintID, Title: "Budget warning", BudgetUsd: 5, SpentUsd: 4.2},
				{ID: uuid.New(), ProjectID: projectID, SprintID: &activeSprintID, Title: "Healthy", BudgetUsd: 3, SpentUsd: 1.5},
				{ID: uuid.New(), ProjectID: projectID, SprintID: &closedSprintID, Title: "Task aggregate", BudgetUsd: 2, SpentUsd: 2.2},
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader, taskReader)
	svc.SetProjectBudgetLimit(12)

	summary, err := svc.GetProjectBudgetSummary(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProjectBudgetSummary() error = %v", err)
	}
	if summary.ProjectID != projectID.String() {
		t.Fatalf("ProjectID = %q, want %q", summary.ProjectID, projectID.String())
	}
	if summary.Allocated != 12 || summary.Spent != 10.5 || summary.Remaining != 1.5 {
		t.Fatalf("summary budget = %+v, want allocated=12 spent=10.5 remaining=1.5", summary)
	}
	if summary.ThresholdStatus != model.BudgetThresholdWarning {
		t.Fatalf("ThresholdStatus = %q, want warning", summary.ThresholdStatus)
	}
	if summary.TasksAtRiskCount != 2 {
		t.Fatalf("TasksAtRiskCount = %d, want 2", summary.TasksAtRiskCount)
	}
	if summary.TasksExceededCount != 1 {
		t.Fatalf("TasksExceededCount = %d, want 1", summary.TasksExceededCount)
	}
	if len(summary.Scopes) != 3 {
		t.Fatalf("len(Scopes) = %d, want 3", len(summary.Scopes))
	}
}

func TestBudgetGovernanceService_GetProjectBudgetSummaryReturnsZeroWhenInactive(t *testing.T) {
	projectID := uuid.New()
	svc := service.NewBudgetGovernanceService(
		&mockBudgetSprintReader{sprintsByProject: map[uuid.UUID][]*model.Sprint{projectID: nil}},
		&mockBudgetTaskReader{tasksByProject: map[uuid.UUID][]*model.Task{projectID: nil}},
	)

	summary, err := svc.GetProjectBudgetSummary(context.Background(), projectID)
	if err != nil {
		t.Fatalf("GetProjectBudgetSummary() error = %v", err)
	}
	if summary.Allocated != 0 || summary.Spent != 0 || summary.Remaining != 0 {
		t.Fatalf("summary budget = %+v, want zero values", summary)
	}
	if summary.ThresholdStatus != model.BudgetThresholdInactive {
		t.Fatalf("ThresholdStatus = %q, want inactive", summary.ThresholdStatus)
	}
	if len(summary.Scopes) != 0 {
		t.Fatalf("Scopes = %+v, want none", summary.Scopes)
	}
}

func TestBudgetGovernanceService_GetSprintBudgetDetailReturnsRealtimeSpend(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByID: map[uuid.UUID]*model.Sprint{
			sprintID: {
				ID:             sprintID,
				ProjectID:      projectID,
				Name:           "Realtime sprint",
				Status:         model.SprintStatusActive,
				TotalBudgetUsd: 12,
				SpentUsd:       9.6,
			},
		},
	}
	taskReader := &mockBudgetTaskReader{
		tasksBySprint: map[uuid.UUID][]*model.Task{
			sprintID: {
				{ID: uuid.New(), ProjectID: projectID, SprintID: &sprintID, Title: "Task A", BudgetUsd: 5, SpentUsd: 4.8},
				{ID: uuid.New(), ProjectID: projectID, SprintID: &sprintID, Title: "Task B", BudgetUsd: 7, SpentUsd: 4.8},
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader, taskReader)

	detail, err := svc.GetSprintBudgetDetail(context.Background(), sprintID)
	if err != nil {
		t.Fatalf("GetSprintBudgetDetail() error = %v", err)
	}
	if detail.SprintID != sprintID.String() || detail.ProjectID != projectID.String() {
		t.Fatalf("detail ids = %+v", detail)
	}
	if detail.Allocated != 12 || detail.Spent != 9.6 || detail.Remaining != 2.4 {
		t.Fatalf("detail budget = %+v, want allocated=12 spent=9.6 remaining=2.4", detail)
	}
	if detail.ThresholdStatus != model.BudgetThresholdWarning {
		t.Fatalf("ThresholdStatus = %q, want warning", detail.ThresholdStatus)
	}
	if len(detail.Tasks) != 2 {
		t.Fatalf("len(Tasks) = %d, want 2", len(detail.Tasks))
	}
}

func TestBudgetGovernanceService_GetSprintBudgetDetailReturnsInactiveWhenNoBudgetConfigured(t *testing.T) {
	projectID := uuid.New()
	sprintID := uuid.New()
	reader := &mockBudgetSprintReader{
		sprintsByID: map[uuid.UUID]*model.Sprint{
			sprintID: {
				ID:             sprintID,
				ProjectID:      projectID,
				Name:           "No budget sprint",
				Status:         model.SprintStatusPlanning,
				TotalBudgetUsd: 0,
				SpentUsd:       0,
			},
		},
	}
	svc := service.NewBudgetGovernanceService(reader, &mockBudgetTaskReader{})

	detail, err := svc.GetSprintBudgetDetail(context.Background(), sprintID)
	if err != nil {
		t.Fatalf("GetSprintBudgetDetail() error = %v", err)
	}
	if detail.Allocated != 0 || detail.Spent != 0 || detail.Remaining != 0 {
		t.Fatalf("detail budget = %+v, want zero values", detail)
	}
	if detail.ThresholdStatus != model.BudgetThresholdInactive {
		t.Fatalf("ThresholdStatus = %q, want inactive", detail.ThresholdStatus)
	}
}
