package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type stubStatsTaskRepo struct {
	completedCounts []repository.TaskDateCost
}

func (s *stubStatsTaskRepo) SummarizeCompletedCostByDateRange(_ context.Context, _, _ time.Time, _ *uuid.UUID) ([]repository.TaskDateCost, error) {
	return s.completedCounts, nil
}

type stubStatsAgentRunRepo struct {
	performance []repository.AgentPerformanceRow
}

func (s *stubStatsAgentRunRepo) AggregatePerformance(_ context.Context, _, _ time.Time, _ *uuid.UUID) ([]repository.AgentPerformanceRow, error) {
	return s.performance, nil
}

func TestStatsService_VelocityIncludesCostPerPoint(t *testing.T) {
	service := NewStatsService(
		&stubStatsTaskRepo{
			completedCounts: []repository.TaskDateCost{
				{Date: time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC), Count: 2, CostUsd: 5.25},
				{Date: time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC), Count: 1, CostUsd: 1.5},
			},
		},
		&stubStatsAgentRunRepo{},
	)

	stats, err := service.Velocity(
		context.Background(),
		time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC),
		nil,
	)
	if err != nil {
		t.Fatalf("Velocity() error = %v", err)
	}

	raw, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("marshal velocity stats: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	points, ok := payload["points"].([]any)
	if !ok || len(points) != 2 {
		t.Fatalf("points = %#v", payload["points"])
	}
	first, ok := points[0].(map[string]any)
	if !ok {
		t.Fatalf("first point = %#v", points[0])
	}
	if _, ok := first["costUsd"]; !ok {
		t.Fatalf("velocity point missing costUsd: %#v", first)
	}
	if first["costUsd"] != 5.25 {
		t.Fatalf("velocity point costUsd = %#v, want 5.25", first["costUsd"])
	}
}

func TestStatsService_AgentPerformanceUsesTruthfulBucketFields(t *testing.T) {
	service := NewStatsService(
		&stubStatsTaskRepo{},
		&stubStatsAgentRunRepo{
			performance: []repository.AgentPerformanceRow{
				{
					RoleID:         "planner",
					TotalRuns:      4,
					CompletedRuns:  3,
					AvgCostUsd:     1.25,
					AvgDurationSec: 900,
					TotalCostUsd:   5,
				},
			},
		},
	)

	stats, err := service.AgentPerformance(
		context.Background(),
		time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC),
		nil,
	)
	if err != nil {
		t.Fatalf("AgentPerformance() error = %v", err)
	}

	raw, err := json.Marshal(stats)
	if err != nil {
		t.Fatalf("marshal agent performance: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}

	entries, ok := payload["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("entries = %#v", payload["entries"])
	}
	entry, ok := entries[0].(map[string]any)
	if !ok {
		t.Fatalf("entry = %#v", entries[0])
	}
	for _, key := range []string{"bucketId", "label", "avgDurationMinutes"} {
		if _, ok := entry[key]; !ok {
			t.Fatalf("agent performance entry missing %q: %#v", key, entry)
		}
	}
	if _, exists := entry["avgDurationSeconds"]; exists {
		t.Fatalf("agent performance entry should not expose avgDurationSeconds: %#v", entry)
	}
	if _, exists := entry["roleId"]; exists {
		t.Fatalf("agent performance entry should not expose roleId as the primary output contract: %#v", entry)
	}
}
