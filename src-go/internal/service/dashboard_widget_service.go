package service

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

type dashboardWidgetTaskReader interface {
	List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error)
	ListBySprint(ctx context.Context, projectID uuid.UUID, sprintID uuid.UUID) ([]*model.Task, error)
	CountCompletedByDateRange(ctx context.Context, from, to time.Time, projectID *uuid.UUID) ([]repository.TaskDateCount, error)
}

type dashboardWidgetSprintReader interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Sprint, error)
}

type dashboardWidgetAgentRunReader interface {
	ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.AgentRun, error)
	AggregateByProject(ctx context.Context, projectID uuid.UUID) (*model.CostSummaryDTO, error)
	AggregatePerformance(ctx context.Context, from, to time.Time, projectID *uuid.UUID) ([]repository.AgentPerformanceRow, error)
}

type dashboardWidgetCache interface {
	GetWidgetData(ctx context.Context, key string) (string, error)
	SetWidgetData(ctx context.Context, key, payload string, ttl time.Duration) error
}

type DashboardWidgetService struct {
	tasks    dashboardWidgetTaskReader
	sprints  dashboardWidgetSprintReader
	runs     dashboardWidgetAgentRunReader
	cache    dashboardWidgetCache
	now      func() time.Time
	cacheTTL time.Duration
}

func NewDashboardWidgetService(tasks dashboardWidgetTaskReader, sprints dashboardWidgetSprintReader, runs dashboardWidgetAgentRunReader, cache dashboardWidgetCache) *DashboardWidgetService {
	return &DashboardWidgetService{
		tasks:    tasks,
		sprints:  sprints,
		runs:     runs,
		cache:    cache,
		now:      func() time.Time { return time.Now().UTC() },
		cacheTTL: 60 * time.Second,
	}
}

func (s *DashboardWidgetService) WithCacheTTL(ttl time.Duration) *DashboardWidgetService {
	if ttl > 0 {
		s.cacheTTL = ttl
	}
	return s
}

func (s *DashboardWidgetService) WidgetData(ctx context.Context, projectID uuid.UUID, widgetType string, configRaw string) (map[string]any, error) {
	cacheKey := widgetCacheKey(projectID, widgetType, configRaw)
	if s.cache != nil {
		if cached, err := s.cache.GetWidgetData(ctx, cacheKey); err == nil {
			var payload map[string]any
			if unmarshalErr := json.Unmarshal([]byte(cached), &payload); unmarshalErr == nil {
				payload["cached"] = true
				return payload, nil
			}
		}
	}

	payload, err := s.computeWidgetData(ctx, projectID, widgetType, configRaw)
	if err != nil {
		return nil, err
	}
	if s.cache != nil {
		if encoded, marshalErr := json.Marshal(payload); marshalErr == nil {
			_ = s.cache.SetWidgetData(ctx, cacheKey, string(encoded), s.cacheTTL)
		}
	}
	return payload, nil
}

func (s *DashboardWidgetService) computeWidgetData(ctx context.Context, projectID uuid.UUID, widgetType string, configRaw string) (map[string]any, error) {
	config := map[string]any{}
	if strings.TrimSpace(configRaw) != "" {
		if err := json.Unmarshal([]byte(configRaw), &config); err != nil {
			return nil, fmt.Errorf("parse widget config: %w", err)
		}
	}
	switch widgetType {
	case model.DashboardWidgetThroughputChart:
		return s.throughputData(ctx, projectID, config)
	case model.DashboardWidgetBurndown:
		return s.burndownData(ctx, projectID, config)
	case model.DashboardWidgetBlockerCount:
		return s.blockerCountData(ctx, projectID)
	case model.DashboardWidgetBudgetConsumption:
		return s.budgetConsumptionData(ctx, projectID)
	case model.DashboardWidgetAgentCost:
		return s.agentCostData(ctx, projectID)
	case model.DashboardWidgetReviewBacklog:
		return s.reviewBacklogData(ctx, projectID)
	case model.DashboardWidgetTaskAging:
		return s.taskAgingData(ctx, projectID)
	case model.DashboardWidgetSLACompliance:
		return s.slaComplianceData(ctx, projectID)
	default:
		return nil, fmt.Errorf("unsupported widget type %q", widgetType)
	}
}

func (s *DashboardWidgetService) throughputData(ctx context.Context, projectID uuid.UUID, config map[string]any) (map[string]any, error) {
	days := intValue(config["days"], 30)
	to := s.now()
	from := to.AddDate(0, 0, -days)
	pointsRaw, err := s.tasks.CountCompletedByDateRange(ctx, from, to, &projectID)
	if err != nil {
		return nil, fmt.Errorf("load throughput data: %w", err)
	}
	points := make([]map[string]any, 0, len(pointsRaw))
	for _, item := range pointsRaw {
		points = append(points, map[string]any{"date": item.Date.Format("2006-01-02"), "count": item.Count})
	}
	return map[string]any{"widgetType": model.DashboardWidgetThroughputChart, "points": points}, nil
}

func (s *DashboardWidgetService) burndownData(ctx context.Context, projectID uuid.UUID, config map[string]any) (map[string]any, error) {
	sprintIDRaw, ok := config["sprintId"].(string)
	if !ok || strings.TrimSpace(sprintIDRaw) == "" {
		return nil, fmt.Errorf("burndown widget requires sprintId")
	}
	sprintID, err := uuid.Parse(sprintIDRaw)
	if err != nil {
		return nil, fmt.Errorf("parse sprintId: %w", err)
	}
	sprint, err := s.sprints.GetByID(ctx, sprintID)
	if err != nil {
		return nil, fmt.Errorf("load sprint: %w", err)
	}
	tasks, err := s.tasks.ListBySprint(ctx, projectID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("load sprint tasks: %w", err)
	}
	metrics := model.BuildSprintMetricsDTO(sprint, tasks, s.now())
	points := make([]map[string]any, 0, len(metrics.Burndown))
	for _, item := range metrics.Burndown {
		points = append(points, map[string]any{
			"date":           item.Date,
			"remainingTasks": item.RemainingTasks,
			"completedTasks": item.CompletedTasks,
		})
	}
	return map[string]any{"widgetType": model.DashboardWidgetBurndown, "points": points}, nil
}

func (s *DashboardWidgetService) blockerCountData(ctx context.Context, projectID uuid.UUID) (map[string]any, error) {
	tasks, _, err := s.tasks.List(ctx, projectID, model.TaskListQuery{})
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}
	count := 0
	for _, task := range tasks {
		if task.Status == model.TaskStatusBlocked {
			count++
		}
	}
	return map[string]any{"widgetType": model.DashboardWidgetBlockerCount, "count": count}, nil
}

func (s *DashboardWidgetService) budgetConsumptionData(ctx context.Context, projectID uuid.UUID) (map[string]any, error) {
	tasks, _, err := s.tasks.List(ctx, projectID, model.TaskListQuery{})
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}
	totalBudget := 0.0
	totalSpent := 0.0
	for _, task := range tasks {
		totalBudget += task.BudgetUsd
		totalSpent += task.SpentUsd
	}
	var trend []map[string]any
	var totalCost *model.CostSummaryDTO
	if s.runs != nil {
		totalCost, _ = s.runs.AggregateByProject(ctx, projectID)
		runs, _ := s.runs.ListByProject(ctx, projectID)
		byDay := map[string]float64{}
		for _, run := range runs {
			day := run.CreatedAt.UTC().Format("2006-01-02")
			byDay[day] += run.CostUsd
		}
		days := make([]string, 0, len(byDay))
		for day := range byDay {
			days = append(days, day)
		}
		sort.Strings(days)
		trend = make([]map[string]any, 0, len(days))
		for _, day := range days {
			trend = append(trend, map[string]any{"date": day, "costUsd": byDay[day]})
		}
	}
	payload := map[string]any{
		"widgetType": model.DashboardWidgetBudgetConsumption,
		"allocated":  totalBudget,
		"spent":      totalSpent,
		"trend":      trend,
	}
	if totalCost != nil {
		payload["runCostUsd"] = totalCost.TotalCostUsd
	}
	return payload, nil
}

func (s *DashboardWidgetService) agentCostData(ctx context.Context, projectID uuid.UUID) (map[string]any, error) {
	from := s.now().AddDate(0, 0, -30)
	to := s.now()
	rows, err := s.runs.AggregatePerformance(ctx, from, to, &projectID)
	if err != nil {
		return nil, fmt.Errorf("load agent performance: %w", err)
	}
	entries := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		entries = append(entries, map[string]any{
			"roleId":        row.RoleID,
			"totalRuns":     row.TotalRuns,
			"completedRuns": row.CompletedRuns,
			"avgCostUsd":    row.AvgCostUsd,
			"totalCostUsd":  row.TotalCostUsd,
		})
	}
	return map[string]any{"widgetType": model.DashboardWidgetAgentCost, "entries": entries}, nil
}

func (s *DashboardWidgetService) reviewBacklogData(ctx context.Context, projectID uuid.UUID) (map[string]any, error) {
	tasks, _, err := s.tasks.List(ctx, projectID, model.TaskListQuery{})
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}
	count := 0
	for _, task := range tasks {
		if task.Status == model.TaskStatusInReview || task.Status == model.TaskStatusChangesRequested {
			count++
		}
	}
	return map[string]any{"widgetType": model.DashboardWidgetReviewBacklog, "count": count}, nil
}

func (s *DashboardWidgetService) taskAgingData(ctx context.Context, projectID uuid.UUID) (map[string]any, error) {
	tasks, _, err := s.tasks.List(ctx, projectID, model.TaskListQuery{})
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}
	buckets := []map[string]any{
		{"bucket": "0-3", "count": 0},
		{"bucket": "4-7", "count": 0},
		{"bucket": "8-14", "count": 0},
		{"bucket": "15+", "count": 0},
	}
	now := s.now()
	for _, task := range tasks {
		ageDays := int(now.Sub(task.CreatedAt).Hours() / 24)
		switch {
		case ageDays <= 3:
			buckets[0]["count"] = buckets[0]["count"].(int) + 1
		case ageDays <= 7:
			buckets[1]["count"] = buckets[1]["count"].(int) + 1
		case ageDays <= 14:
			buckets[2]["count"] = buckets[2]["count"].(int) + 1
		default:
			buckets[3]["count"] = buckets[3]["count"].(int) + 1
		}
	}
	return map[string]any{"widgetType": model.DashboardWidgetTaskAging, "buckets": buckets}, nil
}

func (s *DashboardWidgetService) slaComplianceData(ctx context.Context, projectID uuid.UUID) (map[string]any, error) {
	tasks, _, err := s.tasks.List(ctx, projectID, model.TaskListQuery{})
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}
	total := 0
	compliant := 0
	now := s.now()
	for _, task := range tasks {
		if task.PlannedEndAt == nil {
			continue
		}
		total++
		if task.CompletedAt != nil && !task.CompletedAt.After(*task.PlannedEndAt) {
			compliant++
			continue
		}
		if task.CompletedAt == nil && now.Before(*task.PlannedEndAt) {
			compliant++
		}
	}
	rate := 0.0
	if total > 0 {
		rate = float64(compliant) / float64(total) * 100
	}
	return map[string]any{"widgetType": model.DashboardWidgetSLACompliance, "total": total, "compliant": compliant, "rate": rate}, nil
}

func widgetCacheKey(projectID uuid.UUID, widgetType, configRaw string) string {
	sum := sha1.Sum([]byte(widgetType + "|" + strings.TrimSpace(configRaw)))
	return fmt.Sprintf("%s:%s:%x", projectID.String(), widgetType, sum[:])
}

func intValue(value any, fallback int) int {
	switch typed := value.(type) {
	case float64:
		return int(typed)
	case int:
		return typed
	default:
		return fallback
	}
}
