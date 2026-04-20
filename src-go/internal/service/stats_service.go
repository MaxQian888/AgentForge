package service

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/google/uuid"
)

type StatsService struct {
	taskRepo     statsTaskRepo
	agentRunRepo statsAgentRunRepo
}

type statsTaskRepo interface {
	SummarizeCompletedCostByDateRange(ctx context.Context, from, to time.Time, projectID *uuid.UUID) ([]repository.TaskDateCost, error)
}

type statsAgentRunRepo interface {
	AggregatePerformance(ctx context.Context, from, to time.Time, projectID *uuid.UUID) ([]repository.AgentPerformanceRow, error)
}

func NewStatsService(taskRepo statsTaskRepo, agentRunRepo statsAgentRunRepo) *StatsService {
	return &StatsService{taskRepo: taskRepo, agentRunRepo: agentRunRepo}
}

func (s *StatsService) Velocity(ctx context.Context, from, to time.Time, projectID *uuid.UUID) (*model.VelocityStatsDTO, error) {
	counts, err := s.taskRepo.SummarizeCompletedCostByDateRange(ctx, from, to, projectID)
	if err != nil {
		return nil, fmt.Errorf("velocity stats: %w", err)
	}

	totalCompleted := 0
	totalCost := 0.0
	points := make([]model.VelocityPointDTO, 0, len(counts))
	for _, c := range counts {
		totalCompleted += c.Count
		totalCost += c.CostUsd
		points = append(points, model.VelocityPointDTO{
			Period:         c.Date.Format("2006-01-02"),
			TasksCompleted: c.Count,
			CostUsd:        math.Round(c.CostUsd*10000) / 10000,
		})
	}

	days := to.Sub(from).Hours() / 24
	if days < 1 {
		days = 1
	}
	avgPerDay := math.Round(float64(totalCompleted)/days*100) / 100

	return &model.VelocityStatsDTO{
		Points:         points,
		TotalCompleted: totalCompleted,
		TotalCostUsd:   math.Round(totalCost*10000) / 10000,
		AvgPerDay:      avgPerDay,
	}, nil
}

func (s *StatsService) AgentPerformance(ctx context.Context, from, to time.Time, projectID *uuid.UUID) (*model.AgentPerformanceDTO, error) {
	rows, err := s.agentRunRepo.AggregatePerformance(ctx, from, to, projectID)
	if err != nil {
		return nil, fmt.Errorf("agent performance stats: %w", err)
	}

	entries := make([]model.AgentPerformanceEntryDTO, 0, len(rows))
	for _, row := range rows {
		successRate := 0.0
		if row.TotalRuns > 0 {
			successRate = math.Round(float64(row.CompletedRuns)/float64(row.TotalRuns)*10000) / 100
		}
		bucketID := strings.TrimSpace(row.RoleID)
		if bucketID == "" {
			bucketID = "unknown"
		}
		label := strings.TrimSpace(row.RoleID)
		if label == "" {
			label = "Unknown execution bucket"
		}
		entries = append(entries, model.AgentPerformanceEntryDTO{
			BucketID:           bucketID,
			Label:              label,
			RunCount:           row.TotalRuns,
			SuccessRate:        successRate,
			AvgCostUsd:         math.Round(row.AvgCostUsd*10000) / 10000,
			AvgDurationMinutes: math.Round((row.AvgDurationSec/60)*100) / 100,
			TotalCostUsd:       math.Round(row.TotalCostUsd*10000) / 10000,
		})
	}

	return &model.AgentPerformanceDTO{Entries: entries}, nil
}
