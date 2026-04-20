package service

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type stubCostQueryTaskReader struct {
	pages map[int][]*model.Task
	total int
	err   error
}

func (s *stubCostQueryTaskReader) List(_ context.Context, _ uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error) {
	if s.err != nil {
		return nil, 0, s.err
	}
	return s.pages[q.Page], s.total, nil
}

type stubCostQuerySprintReader struct {
	projectSprints []*model.Sprint
	err            error
}

func (s *stubCostQuerySprintReader) ListByProject(_ context.Context, _ uuid.UUID) ([]*model.Sprint, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.projectSprints, nil
}

type stubCostQueryRunReader struct {
	projectRuns []*model.AgentRun
	sprintRuns  []*model.AgentRun
	activeRuns  []*model.AgentRun
	projectErr  error
	sprintErr   error
	activeErr   error
}

func (s *stubCostQueryRunReader) ListByProject(_ context.Context, _ uuid.UUID) ([]*model.AgentRun, error) {
	if s.projectErr != nil {
		return nil, s.projectErr
	}
	return s.projectRuns, nil
}

func (s *stubCostQueryRunReader) ListBySprint(_ context.Context, _ uuid.UUID) ([]*model.AgentRun, error) {
	if s.sprintErr != nil {
		return nil, s.sprintErr
	}
	return s.sprintRuns, nil
}

func (s *stubCostQueryRunReader) ListActive(_ context.Context) ([]*model.AgentRun, error) {
	if s.activeErr != nil {
		return nil, s.activeErr
	}
	return s.activeRuns, nil
}

type stubCostQueryBudgetReader struct {
	summary *model.ProjectBudgetSummary
	err     error
}

func (s *stubCostQueryBudgetReader) GetProjectBudgetSummary(_ context.Context, _ uuid.UUID) (*model.ProjectBudgetSummary, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.summary, nil
}

func TestCostQueryService_ProjectSummaryAggregatesDeterministicOutputs(t *testing.T) {
	projectID := uuid.MustParse("11111111-2222-3333-4444-555555555555")
	sprintID := uuid.MustParse("66666666-7777-8888-9999-aaaaaaaaaaaa")
	taskA := uuid.MustParse("bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	taskB := uuid.MustParse("cccccccc-cccc-cccc-cccc-cccccccccccc")
	now := time.Date(2026, 3, 30, 16, 0, 0, 0, time.UTC)

	service := NewCostQueryService(
		&stubCostQueryTaskReader{
			pages: map[int][]*model.Task{
				1: {
					&model.Task{ID: taskA, ProjectID: projectID, Title: "Build", SpentUsd: 4.5, SprintID: &sprintID},
				},
				2: {
					&model.Task{ID: taskB, ProjectID: projectID, Title: "Review", SpentUsd: 0},
				},
			},
			total: 2,
		},
		&stubCostQuerySprintReader{
			projectSprints: []*model.Sprint{
				{ID: sprintID, Name: "Sprint 1", SpentUsd: 4.5, TotalBudgetUsd: 8},
			},
		},
		&stubCostQueryRunReader{
			projectRuns: []*model.AgentRun{
				{TaskID: taskA, Status: model.AgentRunStatusRunning, Runtime: "claude_code", Provider: "anthropic", Model: "claude-sonnet-4-5", CostUsd: 2.25, InputTokens: 10, OutputTokens: 20, CacheReadTokens: 3, TurnCount: 2, CreatedAt: now, CostAccounting: &model.CostAccountingSnapshot{Mode: "authoritative_total", Coverage: "full", Source: "anthropic_result_total"}},
				{TaskID: taskA, Status: model.AgentRunStatusCompleted, Runtime: "codex", Provider: "openai", Model: "gpt-5-codex", CostUsd: 2.25, InputTokens: 5, OutputTokens: 10, CacheReadTokens: 1, TurnCount: 1, CreatedAt: now.AddDate(0, 0, -1), CostAccounting: &model.CostAccountingSnapshot{Mode: "estimated_api_pricing", Coverage: "full", Source: "openai_api_pricing"}},
				{TaskID: taskB, Status: model.AgentRunStatusPaused, Runtime: "opencode", Provider: "opencode", Model: "opencode-default", CostUsd: 0, InputTokens: 7, OutputTokens: 8, CacheReadTokens: 0, TurnCount: 3, CreatedAt: now.AddDate(0, 0, -10), CostAccounting: &model.CostAccountingSnapshot{Mode: "unpriced", Coverage: "none", Source: "opencode_usage"}},
			},
		},
		&stubCostQueryBudgetReader{
			summary: &model.ProjectBudgetSummary{ProjectID: projectID.String(), Allocated: 20, Spent: 4.5},
		},
	)
	service.now = func() time.Time { return now }

	summary, err := service.ProjectSummary(context.Background(), projectID)
	if err != nil {
		t.Fatalf("ProjectSummary() error = %v", err)
	}
	if summary.TotalCostUsd != 4.5 {
		t.Fatalf("summary.TotalCostUsd = %v, want 4.5", summary.TotalCostUsd)
	}
	if summary.ActiveAgents != 2 {
		t.Fatalf("summary.ActiveAgents = %d, want 2", summary.ActiveAgents)
	}
	if len(summary.TaskCosts) != 2 || summary.TaskCosts[0].TaskID != taskA.String() {
		t.Fatalf("summary.TaskCosts = %#v", summary.TaskCosts)
	}
	if len(summary.SprintCosts) != 1 || summary.SprintCosts[0].InputTokens != 15 {
		t.Fatalf("summary.SprintCosts = %#v", summary.SprintCosts)
	}
	if len(summary.DailyCosts) != 3 || summary.DailyCosts[0].Date != "2026-03-20" {
		t.Fatalf("summary.DailyCosts = %#v", summary.DailyCosts)
	}
	if summary.BudgetSummary == nil || summary.BudgetSummary.ProjectID != projectID.String() {
		t.Fatalf("summary.BudgetSummary = %#v", summary.BudgetSummary)
	}
	if summary.PeriodRollups["today"].RunCount != 1 || summary.PeriodRollups["last7Days"].RunCount != 2 || summary.PeriodRollups["last30Days"].RunCount != 3 {
		t.Fatalf("summary.PeriodRollups = %#v", summary.PeriodRollups)
	}
	if summary.CostCoverage == nil {
		t.Fatalf("summary.CostCoverage = nil")
	}
	if summary.CostCoverage.AuthoritativeRunCount != 1 || summary.CostCoverage.EstimatedRunCount != 1 || summary.CostCoverage.UnpricedRunCount != 1 {
		t.Fatalf("summary.CostCoverage = %#v", summary.CostCoverage)
	}
	if !summary.CostCoverage.HasCoverageGap {
		t.Fatalf("summary.CostCoverage.HasCoverageGap = false, want true")
	}
	if len(summary.RuntimeBreakdown) != 3 {
		t.Fatalf("summary.RuntimeBreakdown = %#v", summary.RuntimeBreakdown)
	}
}

func TestCostQueryService_SummariesAndHelpers(t *testing.T) {
	projectID := uuid.MustParse("dddddddd-dddd-dddd-dddd-dddddddddddd")
	sprintID := uuid.MustParse("eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee")
	taskID := uuid.MustParse("ffffffff-ffff-ffff-ffff-ffffffffffff")
	now := time.Date(2026, 3, 30, 16, 30, 0, 0, time.UTC)

	runs := []*model.AgentRun{
		{TaskID: taskID, Status: model.AgentRunStatusStarting, CostUsd: 1.23456, InputTokens: 2, OutputTokens: 3, CacheReadTokens: 1, TurnCount: 1, CreatedAt: now},
		nil,
		{TaskID: taskID, Status: model.AgentRunStatusCompleted, CostUsd: 2, InputTokens: 4, OutputTokens: 5, CacheReadTokens: 0, TurnCount: 2, CreatedAt: now.AddDate(0, 0, -8)},
	}
	summary := aggregateCostSummary(runs)
	if summary.RunCount != 3 || summary.TotalCostUsd != 3.2346 {
		t.Fatalf("aggregateCostSummary() = %#v", summary)
	}
	if countActiveRuns(runs) != 1 {
		t.Fatalf("countActiveRuns() = %d, want 1", countActiveRuns(runs))
	}
	if roundCost(1.23456) != 1.2346 {
		t.Fatalf("roundCost() = %v, want 1.2346", roundCost(1.23456))
	}

	tasks, err := (&CostQueryService{tasks: &stubCostQueryTaskReader{
		pages: map[int][]*model.Task{
			1: {&model.Task{ID: taskID, ProjectID: projectID, Title: "A"}},
			2: {&model.Task{ID: uuid.New(), ProjectID: projectID, Title: "B"}},
		},
		total: 2,
	}}).listProjectTasks(context.Background(), projectID)
	if err != nil || len(tasks) != 2 {
		t.Fatalf("listProjectTasks() = %#v, %v", tasks, err)
	}

	if summary, err := (&CostQueryService{}).projectBudgetSummary(context.Background(), projectID); err != nil || summary != nil {
		t.Fatalf("projectBudgetSummary(nil reader) = %#v, %v; want nil, nil", summary, err)
	}

	service := NewCostQueryService(nil, nil, &stubCostQueryRunReader{
		sprintRuns: runs,
		activeRuns: runs,
	}, nil)
	sprintSummary, err := service.SprintSummary(context.Background(), sprintID)
	if err != nil || sprintSummary.TotalTurns != 3 {
		t.Fatalf("SprintSummary() = %#v, %v", sprintSummary, err)
	}
	activeSummary, err := service.ActiveSummary(context.Background())
	if err != nil || activeSummary.TotalCostUsd != 3.2346 {
		t.Fatalf("ActiveSummary() = %#v, %v", activeSummary, err)
	}
}

func TestCostQueryService_PropagatesErrorsAndBuildHelpers(t *testing.T) {
	projectID := uuid.MustParse("12121212-3434-5656-7878-909090909090")
	wantErr := errors.New("boom")

	service := NewCostQueryService(
		&stubCostQueryTaskReader{err: wantErr},
		&stubCostQuerySprintReader{},
		&stubCostQueryRunReader{},
		&stubCostQueryBudgetReader{},
	)
	if _, err := service.ProjectSummary(context.Background(), projectID); err == nil {
		t.Fatal("ProjectSummary() expected error")
	}

	taskA := uuid.MustParse("13131313-1313-1313-1313-131313131313")
	taskB := uuid.MustParse("14141414-1414-1414-1414-141414141414")
	sprintID := uuid.MustParse("15151515-1515-1515-1515-151515151515")
	now := time.Date(2026, 3, 30, 17, 0, 0, 0, time.UTC)

	daily := buildDailyCostSeries([]*model.AgentRun{
		{CreatedAt: now.AddDate(0, 0, -1), CostUsd: 1.11111},
		{CreatedAt: now, CostUsd: 2.22222},
		{CreatedAt: now, CostUsd: 0.33333},
	})
	if !reflect.DeepEqual(daily, []model.CostTimeSeriesDTO{
		{Date: "2026-03-29", CostUsd: 1.1111, Runs: 1},
		{Date: "2026-03-30", CostUsd: 2.5556, Runs: 2},
	}) {
		t.Fatalf("buildDailyCostSeries() = %#v", daily)
	}

	taskCosts := buildTaskCostDetails(
		[]*model.Task{
			{ID: taskA, Title: "Build", SpentUsd: 0},
			{ID: taskB, Title: "Review", SpentUsd: 5},
		},
		[]*model.AgentRun{
			{TaskID: taskA, CostUsd: 2, InputTokens: 3, OutputTokens: 4, CacheReadTokens: 1},
			{TaskID: taskA, CostUsd: 3, InputTokens: 1, OutputTokens: 2, CacheReadTokens: 0},
		},
	)
	if len(taskCosts) != 2 || taskCosts[0].TaskTitle != "Build" || taskCosts[0].CostUsd != 5 || taskCosts[1].TaskTitle != "Review" || taskCosts[1].CostUsd != 5 {
		t.Fatalf("buildTaskCostDetails() = %#v", taskCosts)
	}
	if taskCosts[0].AgentRuns != 2 || taskCosts[0].InputTokens != 4 {
		t.Fatalf("buildTaskCostDetails(agent aggregation) = %#v", taskCosts[0])
	}

	sprintCosts := buildSprintCostSummaries(
		[]*model.Sprint{{ID: sprintID, Name: "Sprint 1", SpentUsd: 4.5, TotalBudgetUsd: 9}},
		[]*model.Task{{ID: taskA, SprintID: &sprintID}},
		[]*model.AgentRun{{TaskID: taskA, InputTokens: 10, OutputTokens: 20}},
	)
	if len(sprintCosts) != 1 || sprintCosts[0].InputTokens != 10 || sprintCosts[0].BudgetUsd != 9 {
		t.Fatalf("buildSprintCostSummaries() = %#v", sprintCosts)
	}

	rollups := buildPeriodRollups([]*model.AgentRun{
		{CreatedAt: now, CostUsd: 1},
		{CreatedAt: now.AddDate(0, 0, -5), CostUsd: 2},
		{CreatedAt: now.AddDate(0, 0, -20), CostUsd: 3},
		{CreatedAt: now.AddDate(0, 0, -40), CostUsd: 4},
	}, now)
	if rollups["today"].CostUsd != 1 || rollups["last7Days"].CostUsd != 3 || rollups["last30Days"].CostUsd != 6 {
		t.Fatalf("buildPeriodRollups() = %#v", rollups)
	}
}

func TestDefaultCodingAgentCatalog(t *testing.T) {
	selection := model.CodingAgentSelection{Runtime: "codex", Provider: "openai", Model: "gpt-5-codex"}
	catalog := DefaultCodingAgentCatalog(selection)
	if catalog == nil {
		t.Fatal("DefaultCodingAgentCatalog() = nil")
	}
	if catalog.DefaultRuntime != model.DefaultCodingAgentRuntime {
		t.Fatalf("catalog.DefaultRuntime = %q, want %q", catalog.DefaultRuntime, model.DefaultCodingAgentRuntime)
	}
	if !reflect.DeepEqual(catalog.DefaultSelection, selection) {
		t.Fatalf("catalog.DefaultSelection = %#v, want %#v", catalog.DefaultSelection, selection)
	}
	if len(catalog.Runtimes) < 7 {
		t.Fatalf("len(catalog.Runtimes) = %d, want at least 7", len(catalog.Runtimes))
	}
	runtimeByKey := make(map[string]model.CodingAgentRuntimeOptionDTO, len(catalog.Runtimes))
	for _, runtime := range catalog.Runtimes {
		runtimeByKey[runtime.Runtime] = runtime
	}
	for _, key := range []string{"claude_code", "codex", "opencode", "cursor", "gemini", "qoder", "iflow"} {
		if _, ok := runtimeByKey[key]; !ok {
			t.Fatalf("expected runtime %q in default catalog", key)
		}
	}
	if !runtimeByKey["codex"].Available || runtimeByKey["codex"].DefaultProvider != "openai" {
		t.Fatalf("codex runtime entry = %#v", runtimeByKey["codex"])
	}
}
