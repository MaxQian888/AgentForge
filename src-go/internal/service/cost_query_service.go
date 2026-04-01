package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

type costQueryTaskReader interface {
	List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error)
}

type costQuerySprintReader interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.Sprint, error)
}

type costQueryRunReader interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentRun, error)
	ListBySprint(ctx context.Context, sprintID uuid.UUID) ([]*model.AgentRun, error)
	ListActive(ctx context.Context) ([]*model.AgentRun, error)
}

type costQueryBudgetReader interface {
	GetProjectBudgetSummary(ctx context.Context, projectID uuid.UUID) (*model.ProjectBudgetSummary, error)
}

type CostQueryService struct {
	tasks   costQueryTaskReader
	sprints costQuerySprintReader
	runs    costQueryRunReader
	budget  costQueryBudgetReader
	now     func() time.Time
}

func NewCostQueryService(tasks costQueryTaskReader, sprints costQuerySprintReader, runs costQueryRunReader, budget costQueryBudgetReader) *CostQueryService {
	return &CostQueryService{
		tasks:   tasks,
		sprints: sprints,
		runs:    runs,
		budget:  budget,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *CostQueryService) ProjectSummary(ctx context.Context, projectID uuid.UUID) (*model.ProjectCostSummaryDTO, error) {
	runs, err := s.runs.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project cost summary: list runs: %w", err)
	}
	tasks, err := s.listProjectTasks(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project cost summary: list tasks: %w", err)
	}
	sprints, err := s.sprints.ListByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project cost summary: list sprints: %w", err)
	}

	summary := aggregateCostSummary(runs)
	budgetSummary, err := s.projectBudgetSummary(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("project cost summary: budget summary: %w", err)
	}

	return &model.ProjectCostSummaryDTO{
		TotalCostUsd:         summary.TotalCostUsd,
		TotalInputTokens:     summary.TotalInputTokens,
		TotalOutputTokens:    summary.TotalOutputTokens,
		TotalCacheReadTokens: summary.TotalCacheReadTokens,
		TotalTurns:           summary.TotalTurns,
		RunCount:             summary.RunCount,
		ActiveAgents:         countActiveRuns(runs),
		SprintCosts:          buildSprintCostSummaries(sprints, tasks, runs),
		TaskCosts:            buildTaskCostDetails(tasks, runs),
		DailyCosts:           buildDailyCostSeries(runs),
		BudgetSummary:        budgetSummary,
		PeriodRollups:        buildPeriodRollups(runs, s.now()),
		CostCoverage:         buildCostCoverageSummary(runs),
		RuntimeBreakdown:     buildRuntimeCostBreakdown(runs),
	}, nil
}

func (s *CostQueryService) SprintSummary(ctx context.Context, sprintID uuid.UUID) (*model.CostSummaryDTO, error) {
	runs, err := s.runs.ListBySprint(ctx, sprintID)
	if err != nil {
		return nil, fmt.Errorf("sprint cost summary: list runs: %w", err)
	}
	summary := aggregateCostSummary(runs)
	return &summary, nil
}

func (s *CostQueryService) ActiveSummary(ctx context.Context) (*model.CostSummaryDTO, error) {
	runs, err := s.runs.ListActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("active cost summary: list runs: %w", err)
	}
	summary := aggregateCostSummary(runs)
	return &summary, nil
}

func (s *CostQueryService) projectBudgetSummary(ctx context.Context, projectID uuid.UUID) (*model.ProjectBudgetSummary, error) {
	if s.budget == nil {
		return nil, nil
	}
	return s.budget.GetProjectBudgetSummary(ctx, projectID)
}

func (s *CostQueryService) listProjectTasks(ctx context.Context, projectID uuid.UUID) ([]*model.Task, error) {
	if s.tasks == nil {
		return nil, nil
	}
	const pageSize = 200
	page := 1
	tasks := make([]*model.Task, 0)
	for {
		items, total, err := s.tasks.List(ctx, projectID, model.TaskListQuery{Page: page, Limit: pageSize})
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, items...)
		if len(tasks) >= total || len(items) == 0 {
			return tasks, nil
		}
		page++
	}
}

func aggregateCostSummary(runs []*model.AgentRun) model.CostSummaryDTO {
	summary := model.CostSummaryDTO{RunCount: len(runs)}
	for _, run := range runs {
		if run == nil {
			continue
		}
		summary.TotalCostUsd += pricedCostForRun(run)
		summary.TotalInputTokens += run.InputTokens
		summary.TotalOutputTokens += run.OutputTokens
		summary.TotalCacheReadTokens += run.CacheReadTokens
		summary.TotalTurns += run.TurnCount
	}
	summary.TotalCostUsd = roundCost(summary.TotalCostUsd)
	return summary
}

func countActiveRuns(runs []*model.AgentRun) int {
	count := 0
	for _, run := range runs {
		if run == nil {
			continue
		}
		switch run.Status {
		case model.AgentRunStatusStarting, model.AgentRunStatusRunning, model.AgentRunStatusPaused:
			count++
		}
	}
	return count
}

func buildDailyCostSeries(runs []*model.AgentRun) []model.CostTimeSeriesDTO {
	type bucket struct {
		cost float64
		runs int
	}
	byDay := map[string]*bucket{}
	for _, run := range runs {
		if run == nil {
			continue
		}
		day := run.CreatedAt.UTC().Format("2006-01-02")
		entry := byDay[day]
		if entry == nil {
			entry = &bucket{}
			byDay[day] = entry
		}
		entry.cost += pricedCostForRun(run)
		entry.runs++
	}
	days := make([]string, 0, len(byDay))
	for day := range byDay {
		days = append(days, day)
	}
	sort.Strings(days)

	series := make([]model.CostTimeSeriesDTO, 0, len(days))
	for _, day := range days {
		entry := byDay[day]
		series = append(series, model.CostTimeSeriesDTO{
			Date:    day,
			CostUsd: roundCost(entry.cost),
			Runs:    entry.runs,
		})
	}
	return series
}

func buildTaskCostDetails(tasks []*model.Task, runs []*model.AgentRun) []model.TaskCostDetailDTO {
	type aggregate struct {
		summary      *model.TaskCostDetailDTO
		order        string
		fallbackCost float64
		hasRunCost   bool
	}
	byTask := map[uuid.UUID]*aggregate{}
	for _, task := range tasks {
		if task == nil {
			continue
		}
		byTask[task.ID] = &aggregate{
			summary: &model.TaskCostDetailDTO{
				TaskID:    task.ID.String(),
				TaskTitle: task.Title,
			},
			order:        task.Title,
			fallbackCost: roundCost(task.SpentUsd),
		}
	}

	for _, run := range runs {
		if run == nil {
			continue
		}
		entry := byTask[run.TaskID]
		if entry == nil {
			entry = &aggregate{
				summary: &model.TaskCostDetailDTO{
					TaskID:    run.TaskID.String(),
					TaskTitle: run.TaskID.String(),
				},
				order: run.TaskID.String(),
			}
			byTask[run.TaskID] = entry
		}
		entry.summary.AgentRuns++
		entry.summary.InputTokens += run.InputTokens
		entry.summary.OutputTokens += run.OutputTokens
		entry.summary.CacheReadTokens += run.CacheReadTokens
		entry.summary.CostUsd += pricedCostForRun(run)
		entry.hasRunCost = true
	}

	items := make([]model.TaskCostDetailDTO, 0, len(byTask))
	for _, entry := range byTask {
		if entry.hasRunCost {
			entry.summary.CostUsd = roundCost(entry.summary.CostUsd)
		} else {
			entry.summary.CostUsd = entry.fallbackCost
		}
		if entry.summary.AgentRuns == 0 && entry.summary.CostUsd == 0 {
			continue
		}
		items = append(items, *entry.summary)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CostUsd == items[j].CostUsd {
			return items[i].TaskTitle < items[j].TaskTitle
		}
		return items[i].CostUsd > items[j].CostUsd
	})
	return items
}

func buildSprintCostSummaries(sprints []*model.Sprint, tasks []*model.Task, runs []*model.AgentRun) []model.SprintCostSummaryDTO {
	byTaskSprint := map[uuid.UUID]uuid.UUID{}
	for _, task := range tasks {
		if task == nil || task.SprintID == nil {
			continue
		}
		byTaskSprint[task.ID] = *task.SprintID
	}

	bySprint := map[uuid.UUID]*model.SprintCostSummaryDTO{}
	for _, sprint := range sprints {
		if sprint == nil {
			continue
		}
		bySprint[sprint.ID] = &model.SprintCostSummaryDTO{
			SprintID:   sprint.ID.String(),
			SprintName: sprint.Name,
			CostUsd:    0,
			BudgetUsd:  roundCost(sprint.TotalBudgetUsd),
		}
	}

	for _, run := range runs {
		if run == nil {
			continue
		}
		sprintID, ok := byTaskSprint[run.TaskID]
		if !ok {
			continue
		}
		entry := bySprint[sprintID]
		if entry == nil {
			continue
		}
		entry.CostUsd += pricedCostForRun(run)
		entry.InputTokens += run.InputTokens
		entry.OutputTokens += run.OutputTokens
	}

	items := make([]model.SprintCostSummaryDTO, 0, len(bySprint))
	for _, entry := range bySprint {
		entry.CostUsd = roundCost(entry.CostUsd)
		if entry.CostUsd == 0 && entry.BudgetUsd == 0 && entry.InputTokens == 0 && entry.OutputTokens == 0 {
			continue
		}
		items = append(items, *entry)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].CostUsd == items[j].CostUsd {
			return items[i].SprintName < items[j].SprintName
		}
		return items[i].CostUsd > items[j].CostUsd
	})
	return items
}

func buildPeriodRollups(runs []*model.AgentRun, now time.Time) map[string]model.CostPeriodRollupDTO {
	now = now.UTC()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	ranges := map[string]time.Time{
		"today":      startOfToday,
		"last7Days":  startOfToday.AddDate(0, 0, -6),
		"last30Days": startOfToday.AddDate(0, 0, -29),
	}
	rollups := map[string]model.CostPeriodRollupDTO{
		"today":      {},
		"last7Days":  {},
		"last30Days": {},
	}
	for _, run := range runs {
		if run == nil {
			continue
		}
		created := run.CreatedAt.UTC()
		for key, start := range ranges {
			if created.Before(start) {
				continue
			}
			entry := rollups[key]
			entry.CostUsd += pricedCostForRun(run)
			entry.InputTokens += run.InputTokens
			entry.OutputTokens += run.OutputTokens
			entry.CacheReadTokens += run.CacheReadTokens
			entry.Turns += run.TurnCount
			entry.RunCount++
			rollups[key] = entry
		}
	}
	for key, entry := range rollups {
		entry.CostUsd = roundCost(entry.CostUsd)
		rollups[key] = entry
	}
	return rollups
}

func buildCostCoverageSummary(runs []*model.AgentRun) *model.CostCoverageSummaryDTO {
	summary := &model.CostCoverageSummaryDTO{}
	for _, run := range runs {
		if run == nil {
			continue
		}
		summary.TotalRunCount++
		accounting := effectiveCostAccounting(run)
		costUsd := pricedCostForRun(run)
		switch accounting.Mode {
		case model.CostAccountingModeAuthoritativeTotal:
			summary.PricedRunCount++
			summary.AuthoritativeRunCount++
			summary.AuthoritativeCostUsd += costUsd
			summary.TotalCostUsd += costUsd
		case model.CostAccountingModeEstimatedAPI:
			summary.PricedRunCount++
			summary.EstimatedRunCount++
			summary.EstimatedCostUsd += costUsd
			summary.TotalCostUsd += costUsd
		case model.CostAccountingModePlanIncluded:
			summary.PlanIncludedRunCount++
			summary.HasCoverageGap = true
		default:
			summary.UnpricedRunCount++
			summary.HasCoverageGap = true
		}
		if accounting.Coverage != model.CostAccountingCoverageFull {
			summary.HasCoverageGap = true
		}
	}
	summary.TotalCostUsd = roundCost(summary.TotalCostUsd)
	summary.AuthoritativeCostUsd = roundCost(summary.AuthoritativeCostUsd)
	summary.EstimatedCostUsd = roundCost(summary.EstimatedCostUsd)
	return summary
}

func buildRuntimeCostBreakdown(runs []*model.AgentRun) []model.RuntimeCostBreakdownDTO {
	type key struct {
		runtime  string
		provider string
		model    string
	}
	byKey := map[key]*model.RuntimeCostBreakdownDTO{}
	for _, run := range runs {
		if run == nil {
			continue
		}
		k := key{runtime: run.Runtime, provider: run.Provider, model: run.Model}
		entry := byKey[k]
		if entry == nil {
			entry = &model.RuntimeCostBreakdownDTO{
				Runtime:  run.Runtime,
				Provider: run.Provider,
				Model:    run.Model,
			}
			byKey[k] = entry
		}
		entry.RunCount++
		accounting := effectiveCostAccounting(run)
		costUsd := pricedCostForRun(run)
		switch accounting.Mode {
		case model.CostAccountingModeAuthoritativeTotal:
			entry.PricedRunCount++
			entry.AuthoritativeRunCount++
			entry.TotalCostUsd += costUsd
		case model.CostAccountingModeEstimatedAPI:
			entry.PricedRunCount++
			entry.EstimatedRunCount++
			entry.TotalCostUsd += costUsd
		case model.CostAccountingModePlanIncluded:
			entry.PlanIncludedRunCount++
		default:
			entry.UnpricedRunCount++
		}
	}

	items := make([]model.RuntimeCostBreakdownDTO, 0, len(byKey))
	for _, entry := range byKey {
		entry.TotalCostUsd = roundCost(entry.TotalCostUsd)
		items = append(items, *entry)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].TotalCostUsd == items[j].TotalCostUsd {
			if items[i].Runtime == items[j].Runtime {
				if items[i].Provider == items[j].Provider {
					return items[i].Model < items[j].Model
				}
				return items[i].Provider < items[j].Provider
			}
			return items[i].Runtime < items[j].Runtime
		}
		return items[i].TotalCostUsd > items[j].TotalCostUsd
	})
	return items
}

func effectiveCostAccounting(run *model.AgentRun) *model.CostAccountingSnapshot {
	if run == nil {
		return &model.CostAccountingSnapshot{
			Mode:     model.CostAccountingModeUnpriced,
			Coverage: model.CostAccountingCoverageNone,
			Source:   "missing_run",
		}
	}
	if run.CostAccounting != nil {
		return run.CostAccounting.Clone()
	}
	if run.CostUsd > 0 {
		if run.Provider == "codex" || (run.Runtime == "codex" && run.Provider != "openai") {
			return &model.CostAccountingSnapshot{
				Mode:     model.CostAccountingModePlanIncluded,
				Coverage: model.CostAccountingCoverageNone,
				Source:   "legacy_plan_usage",
			}
		}
		return &model.CostAccountingSnapshot{
			Mode:     model.CostAccountingModeEstimatedAPI,
			Coverage: model.CostAccountingCoveragePartial,
			Source:   "legacy_inferred_pricing",
		}
	}
	return &model.CostAccountingSnapshot{
		Mode:     model.CostAccountingModeUnpriced,
		Coverage: model.CostAccountingCoverageNone,
		Source:   "legacy_unpriced",
	}
}

func pricedCostForRun(run *model.AgentRun) float64 {
	if run == nil {
		return 0
	}
	switch effectiveCostAccounting(run).Mode {
	case model.CostAccountingModePlanIncluded, model.CostAccountingModeUnpriced:
		return 0
	default:
		return run.CostUsd
	}
}

func roundCost(value float64) float64 {
	return math.Round(value*10000) / 10000
}
