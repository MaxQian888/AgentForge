package scheduler

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/worktree"
)

type taskProgressEvaluatorStub struct {
	changed int
	err     error
}

func (s taskProgressEvaluatorStub) EvaluateOpenTasks(_ context.Context) (int, error) {
	if s.err != nil {
		return 0, s.err
	}
	return s.changed, nil
}

func TestNewTaskProgressDetectorHandler_ReturnsSummaryAndMetrics(t *testing.T) {
	handler := NewTaskProgressDetectorHandler(taskProgressEvaluatorStub{changed: 3})

	result, err := handler(context.Background(), &model.ScheduledJob{JobKey: "task-progress-detector"}, &model.ScheduledJobRun{})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}
	if result == nil {
		t.Fatal("handler() result = nil, want summary")
	}
	if result.Summary != "evaluated 3 open tasks" {
		t.Fatalf("result.Summary = %q, want evaluated summary", result.Summary)
	}
	if result.Metrics != `{"changedTasks":3}` {
		t.Fatalf("result.Metrics = %q, want metrics payload", result.Metrics)
	}
}

func TestNewTaskProgressDetectorHandler_PropagatesFailures(t *testing.T) {
	handler := NewTaskProgressDetectorHandler(taskProgressEvaluatorStub{err: errors.New("boom")})

	if _, err := handler(context.Background(), &model.ScheduledJob{JobKey: "task-progress-detector"}, &model.ScheduledJobRun{}); err == nil {
		t.Fatal("handler() error = nil, want evaluator failure")
	}
}

type projectSourceStub struct {
	projects []*model.Project
	err      error
}

func (s projectSourceStub) ListProjectSlugs(_ context.Context) ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	slugs := make([]string, 0, len(s.projects))
	for _, project := range s.projects {
		slugs = append(slugs, project.Slug)
	}
	return slugs, nil
}

type worktreeManagerStub struct {
	inventoryByProject map[string][]*worktree.Inventory
	cleanedByProject   map[string][]worktree.Inspection
}

func (s *worktreeManagerStub) Inventory(_ context.Context, projectSlug string) (*worktree.Inventory, error) {
	entries := s.inventoryByProject[projectSlug]
	if len(entries) == 0 {
		return &worktree.Inventory{ProjectSlug: projectSlug}, nil
	}
	current := entries[0]
	if len(entries) > 1 {
		s.inventoryByProject[projectSlug] = entries[1:]
	}
	return current, nil
}

func (s *worktreeManagerStub) GarbageCollectAll(_ context.Context, projectSlug string) ([]worktree.Inspection, error) {
	return s.cleanedByProject[projectSlug], nil
}

func TestNewWorktreeGarbageCollectorHandler_SummarizesRepairsAcrossProjects(t *testing.T) {
	handler := NewWorktreeGarbageCollectorHandler(
		projectSourceStub{
			projects: []*model.Project{
				{Slug: "alpha"},
				{Slug: "beta"},
			},
		},
		&worktreeManagerStub{
			inventoryByProject: map[string][]*worktree.Inventory{
				"alpha": {
					{ProjectSlug: "alpha", Total: 3, Stale: 2},
					{ProjectSlug: "alpha", Total: 3, Stale: 1},
				},
				"beta": {
					{ProjectSlug: "beta", Total: 1, Stale: 0},
					{ProjectSlug: "beta", Total: 1, Stale: 0},
				},
			},
			cleanedByProject: map[string][]worktree.Inspection{
				"alpha": {{ProjectSlug: "alpha", TaskID: "t-1", Stale: true}, {ProjectSlug: "alpha", TaskID: "t-2", Stale: true}},
				"beta":  {},
			},
		},
	)

	result, err := handler(context.Background(), &model.ScheduledJob{JobKey: "worktree-garbage-collector"}, &model.ScheduledJobRun{})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}
	if result == nil {
		t.Fatal("handler() result = nil, want summary")
	}
	if result.Summary != "inspected 4 managed worktrees across 2 projects, repaired 2 stale worktrees, 1 unresolved" {
		t.Fatalf("result.Summary = %q, want repair summary", result.Summary)
	}
	if result.Metrics != `{"inspected":4,"projects":2,"repaired":2,"unresolved":1}` {
		t.Fatalf("result.Metrics = %q, want metrics payload", result.Metrics)
	}
}

type bridgeHealthStub struct {
	err error
}

func (s bridgeHealthStub) Health(_ context.Context) error {
	return s.err
}

func TestNewBridgeHealthReconcileHandler_ReturnsHealthySummary(t *testing.T) {
	handler := NewBridgeHealthReconcileHandler(bridgeHealthStub{})

	result, err := handler(context.Background(), &model.ScheduledJob{JobKey: "bridge-health-reconcile"}, &model.ScheduledJobRun{})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}
	if result == nil || result.Summary != "bridge health check passed" {
		t.Fatalf("result = %+v, want healthy summary", result)
	}
	if result.Metrics != `{"healthy":true}` {
		t.Fatalf("result.Metrics = %q, want healthy metrics", result.Metrics)
	}
}

func TestNewBridgeHealthReconcileHandler_PropagatesFailures(t *testing.T) {
	handler := NewBridgeHealthReconcileHandler(bridgeHealthStub{err: errors.New("bridge unhealthy")})

	if _, err := handler(context.Background(), &model.ScheduledJob{JobKey: "bridge-health-reconcile"}, &model.ScheduledJobRun{}); err == nil {
		t.Fatal("handler() error = nil, want bridge health failure")
	}
}

type costProjectRepositoryStub struct {
	projects []*model.Project
}

func (s costProjectRepositoryStub) List(_ context.Context) ([]*model.Project, error) {
	return s.projects, nil
}

type costTaskRepositoryStub struct {
	tasks       map[uuid.UUID][]*model.Task
	updatedTask map[uuid.UUID]float64
	statuses    map[uuid.UUID]string
}

func (s *costTaskRepositoryStub) List(_ context.Context, projectID uuid.UUID, _ model.TaskListQuery) ([]*model.Task, int, error) {
	items := s.tasks[projectID]
	return items, len(items), nil
}

func (s *costTaskRepositoryStub) UpdateSpent(_ context.Context, id uuid.UUID, spentUsd float64, status string) error {
	if s.updatedTask == nil {
		s.updatedTask = make(map[uuid.UUID]float64)
		s.statuses = make(map[uuid.UUID]string)
	}
	s.updatedTask[id] = spentUsd
	s.statuses[id] = status
	return nil
}

type costTeamRepositoryStub struct {
	teams       map[uuid.UUID][]*model.AgentTeam
	updatedTeam map[uuid.UUID]float64
}

func (s *costTeamRepositoryStub) ListByProject(_ context.Context, projectID uuid.UUID) ([]*model.AgentTeam, error) {
	return s.teams[projectID], nil
}

func (s *costTeamRepositoryStub) UpdateSpent(_ context.Context, id uuid.UUID, spent float64) error {
	if s.updatedTeam == nil {
		s.updatedTeam = make(map[uuid.UUID]float64)
	}
	s.updatedTeam[id] = spent
	return nil
}

type costRunRepositoryStub struct {
	taskRuns map[uuid.UUID][]*model.AgentRun
	teamRuns map[uuid.UUID][]*model.AgentRun
}

func (s costRunRepositoryStub) GetByTask(_ context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	return s.taskRuns[taskID], nil
}

func (s costRunRepositoryStub) ListByTeam(_ context.Context, teamID uuid.UUID) ([]*model.AgentRun, error) {
	return s.teamRuns[teamID], nil
}

func TestNewCostReconcileHandler_ReconcilesTaskAndTeamSpend(t *testing.T) {
	projectID := uuid.New()
	taskA := uuid.New()
	taskB := uuid.New()
	teamID := uuid.New()
	taskRepo := &costTaskRepositoryStub{
		tasks: map[uuid.UUID][]*model.Task{
			projectID: {
				{ID: taskA, ProjectID: projectID, Title: "A", BudgetUsd: 2},
				{ID: taskB, ProjectID: projectID, Title: "B", BudgetUsd: 1},
			},
		},
	}
	teamRepo := &costTeamRepositoryStub{
		teams: map[uuid.UUID][]*model.AgentTeam{
			projectID: {
				{ID: teamID, ProjectID: projectID, TaskID: taskA},
			},
		},
	}
	runRepo := costRunRepositoryStub{
		taskRuns: map[uuid.UUID][]*model.AgentRun{
			taskA: {{CostUsd: 1.25}, {CostUsd: 0.5}},
			taskB: {{CostUsd: 1.5}},
		},
		teamRuns: map[uuid.UUID][]*model.AgentRun{
			teamID: {{CostUsd: 0.75}, {CostUsd: 0.25}},
		},
	}

	handler := NewCostReconcileHandler(
		costProjectRepositoryStub{projects: []*model.Project{{ID: projectID, Slug: "alpha"}}},
		taskRepo,
		teamRepo,
		runRepo,
	)

	result, err := handler(context.Background(), &model.ScheduledJob{JobKey: "cost-reconcile"}, &model.ScheduledJobRun{})
	if err != nil {
		t.Fatalf("handler() error = %v", err)
	}
	if result == nil {
		t.Fatal("handler() result = nil, want summary")
	}
	if taskRepo.updatedTask[taskA] != 1.75 {
		t.Fatalf("taskRepo.updatedTask[taskA] = %v, want 1.75", taskRepo.updatedTask[taskA])
	}
	if taskRepo.updatedTask[taskB] != 1.5 {
		t.Fatalf("taskRepo.updatedTask[taskB] = %v, want 1.5", taskRepo.updatedTask[taskB])
	}
	if taskRepo.statuses[taskB] != model.TaskStatusBudgetExceeded {
		t.Fatalf("taskRepo.statuses[taskB] = %q, want budget_exceeded", taskRepo.statuses[taskB])
	}
	if teamRepo.updatedTeam[teamID] != 1 {
		t.Fatalf("teamRepo.updatedTeam[teamID] = %v, want 1", teamRepo.updatedTeam[teamID])
	}
	if result.Summary != "reconciled 2 tasks and 1 teams across 1 projects" {
		t.Fatalf("result.Summary = %q, want reconcile summary", result.Summary)
	}
	if result.Metrics != `{"projects":1,"tasks":2,"teams":1}` {
		t.Fatalf("result.Metrics = %q, want metrics payload", result.Metrics)
	}
}
