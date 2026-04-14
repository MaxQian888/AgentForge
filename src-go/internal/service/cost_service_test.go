package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type costServiceRunRepo struct {
	run             *model.AgentRun
	taskRuns        []*model.AgentRun
	activeRuns      []*model.AgentRun
	updateCostInput struct {
		id              uuid.UUID
		inputTokens     int64
		outputTokens    int64
		cacheReadTokens int64
		costUsd         float64
		turnCount       int
	}
	updateCostErr error
	getByIDErr    error
	getByTaskErr  error
	listActiveErr error
}

func (r *costServiceRunRepo) Create(context.Context, *model.AgentRun) error         { return nil }
func (r *costServiceRunRepo) UpdateStatus(context.Context, uuid.UUID, string) error { return nil }
func (r *costServiceRunRepo) UpdateStructuredOutput(context.Context, uuid.UUID, json.RawMessage) error {
	return nil
}

func (r *costServiceRunRepo) UpdateCost(_ context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int, _ *model.CostAccountingSnapshot) error {
	r.updateCostInput.id = id
	r.updateCostInput.inputTokens = inputTokens
	r.updateCostInput.outputTokens = outputTokens
	r.updateCostInput.cacheReadTokens = cacheReadTokens
	r.updateCostInput.costUsd = costUsd
	r.updateCostInput.turnCount = turnCount
	if r.updateCostErr != nil {
		return r.updateCostErr
	}
	if r.run != nil && r.run.ID == id {
		r.run.InputTokens = inputTokens
		r.run.OutputTokens = outputTokens
		r.run.CacheReadTokens = cacheReadTokens
		r.run.CostUsd = costUsd
		r.run.TurnCount = turnCount
	}
	for _, run := range r.taskRuns {
		if run != nil && run.ID == id {
			run.InputTokens = inputTokens
			run.OutputTokens = outputTokens
			run.CacheReadTokens = cacheReadTokens
			run.CostUsd = costUsd
			run.TurnCount = turnCount
		}
	}
	return nil
}

func (r *costServiceRunRepo) GetByID(context.Context, uuid.UUID) (*model.AgentRun, error) {
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	return r.run, nil
}

func (r *costServiceRunRepo) GetByTask(context.Context, uuid.UUID) ([]*model.AgentRun, error) {
	if r.getByTaskErr != nil {
		return nil, r.getByTaskErr
	}
	return r.taskRuns, nil
}

func (r *costServiceRunRepo) ListActive(context.Context) ([]*model.AgentRun, error) {
	if r.listActiveErr != nil {
		return nil, r.listActiveErr
	}
	return r.activeRuns, nil
}

type costServiceTaskRepo struct {
	task       *model.Task
	getByIDErr error
}

func (r *costServiceTaskRepo) Create(context.Context, *model.Task) error { return nil }
func (r *costServiceTaskRepo) List(context.Context, uuid.UUID, model.TaskListQuery) ([]*model.Task, int, error) {
	return nil, 0, nil
}
func (r *costServiceTaskRepo) Update(context.Context, uuid.UUID, *model.UpdateTaskRequest) error {
	return nil
}
func (r *costServiceTaskRepo) Delete(context.Context, uuid.UUID) error                   { return nil }
func (r *costServiceTaskRepo) TransitionStatus(context.Context, uuid.UUID, string) error { return nil }
func (r *costServiceTaskRepo) UpdateAssignee(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}

func (r *costServiceTaskRepo) GetByID(context.Context, uuid.UUID) (*model.Task, error) {
	if r.getByIDErr != nil {
		return nil, r.getByIDErr
	}
	return r.task, nil
}

func TestCostServiceConstructionAndAggregates(t *testing.T) {
	runRepo := &costServiceRunRepo{}
	taskRepo := &costServiceTaskRepo{}
	svc := NewCostService(runRepo, taskRepo, ws.NewHub())
	if svc.budgetWarnPct != 0.8 || svc.budgetKillPct != 1.0 {
		t.Fatalf("budget thresholds = %v/%v, want 0.8/1.0", svc.budgetWarnPct, svc.budgetKillPct)
	}

	stats := aggregateCosts([]*model.AgentRun{
		{CostUsd: 1.5, InputTokens: 10, OutputTokens: 20, TurnCount: 1},
		{CostUsd: 2.25, InputTokens: 5, OutputTokens: 8, TurnCount: 2},
	})
	if stats.TotalCostUsd != 3.75 || stats.TotalInputTokens != 15 || stats.TotalOutputTokens != 28 || stats.TotalTurns != 3 || stats.RunCount != 2 {
		t.Fatalf("aggregateCosts() = %#v", stats)
	}
}

func TestCostServiceGettersAndErrors(t *testing.T) {
	taskID := uuid.New()
	runRepo := &costServiceRunRepo{
		taskRuns: []*model.AgentRun{
			{TaskID: taskID, CostUsd: 1.5, InputTokens: 10, OutputTokens: 20, TurnCount: 1},
			{TaskID: taskID, CostUsd: 2.25, InputTokens: 5, OutputTokens: 8, TurnCount: 2},
		},
		activeRuns: []*model.AgentRun{
			{TaskID: taskID, CostUsd: 3.5, InputTokens: 7, OutputTokens: 9, TurnCount: 2},
		},
	}
	svc := NewCostService(runRepo, &costServiceTaskRepo{}, ws.NewHub())

	taskStats, err := svc.GetTaskCost(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetTaskCost() error = %v", err)
	}
	if taskStats.TotalCostUsd != 3.75 || taskStats.RunCount != 2 {
		t.Fatalf("GetTaskCost() = %#v", taskStats)
	}

	activeStats, err := svc.GetCostStats(context.Background())
	if err != nil {
		t.Fatalf("GetCostStats() error = %v", err)
	}
	if activeStats.TotalCostUsd != 3.5 || activeStats.RunCount != 1 {
		t.Fatalf("GetCostStats() = %#v", activeStats)
	}

	runRepo.getByTaskErr = errors.New("task runs unavailable")
	if _, err := svc.GetTaskCost(context.Background(), taskID); err == nil {
		t.Fatal("GetTaskCost() expected wrapped error")
	}
	runRepo.getByTaskErr = nil
	runRepo.listActiveErr = errors.New("active runs unavailable")
	if _, err := svc.GetCostStats(context.Background()); err == nil {
		t.Fatal("GetCostStats() expected wrapped error")
	}
}

func TestCostServiceRecordCostCoversHealthyWarningAndExceededBranches(t *testing.T) {
	taskID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()

	makeService := func(budget float64, existingRuns []*model.AgentRun) (*CostService, *costServiceRunRepo) {
		run := &model.AgentRun{ID: runID, TaskID: taskID, Status: model.AgentRunStatusRunning}
		runRepo := &costServiceRunRepo{run: run, taskRuns: existingRuns}
		taskRepo := &costServiceTaskRepo{task: &model.Task{ID: taskID, ProjectID: projectID, BudgetUsd: budget}}
		return NewCostService(runRepo, taskRepo, ws.NewHub()), runRepo
	}

	svc, runRepo := makeService(0, []*model.AgentRun{{ID: runID, TaskID: taskID, CostUsd: 1}})
	if err := svc.RecordCost(context.Background(), runID, 10, 20, 3, 1, 1); err != nil {
		t.Fatalf("RecordCost(no budget) error = %v", err)
	}
	if runRepo.updateCostInput.id != runID || runRepo.updateCostInput.costUsd != 1 {
		t.Fatalf("UpdateCost input = %+v", runRepo.updateCostInput)
	}

	svc, _ = makeService(10, []*model.AgentRun{
		{ID: runID, TaskID: taskID, CostUsd: 8, InputTokens: 5, OutputTokens: 5, TurnCount: 1},
	})
	if err := svc.RecordCost(context.Background(), runID, 5, 5, 0, 8, 1); err != nil {
		t.Fatalf("RecordCost(warning) error = %v", err)
	}

	svc, _ = makeService(10, []*model.AgentRun{
		{ID: runID, TaskID: taskID, CostUsd: 10, InputTokens: 5, OutputTokens: 5, TurnCount: 1},
	})
	if err := svc.RecordCost(context.Background(), runID, 5, 5, 0, 10, 1); err != nil {
		t.Fatalf("RecordCost(exceeded) error = %v", err)
	}
}

func TestCostServiceRecordCostPropagatesRepositoryErrors(t *testing.T) {
	runID := uuid.New()
	runRepo := &costServiceRunRepo{updateCostErr: errors.New("update failed")}
	svc := NewCostService(runRepo, &costServiceTaskRepo{}, ws.NewHub())
	if err := svc.RecordCost(context.Background(), runID, 1, 2, 0, 0.5, 1); err == nil {
		t.Fatal("RecordCost() expected update error")
	}

	runRepo = &costServiceRunRepo{
		run:        &model.AgentRun{ID: runID, TaskID: uuid.New()},
		getByIDErr: errors.New("run missing"),
	}
	svc = NewCostService(runRepo, &costServiceTaskRepo{}, ws.NewHub())
	if err := svc.RecordCost(context.Background(), runID, 1, 2, 0, 0.5, 1); err == nil {
		t.Fatal("RecordCost() expected GetByID error")
	}

	runRepo = &costServiceRunRepo{
		run:      &model.AgentRun{ID: runID, TaskID: uuid.New()},
		taskRuns: []*model.AgentRun{{ID: runID, TaskID: uuid.New(), CostUsd: 0.5}},
	}
	taskRepo := &costServiceTaskRepo{getByIDErr: errors.New("task missing")}
	svc = NewCostService(runRepo, taskRepo, ws.NewHub())
	if err := svc.RecordCost(context.Background(), runID, 1, 2, 0, 0.5, 1); err == nil {
		t.Fatal("RecordCost() expected task lookup error")
	}
}
