package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type mockStatsService struct {
	velocity    *model.VelocityStatsDTO
	velocityErr error
	perf        *model.AgentPerformanceDTO
	perfErr     error
}

func (m *mockStatsService) Velocity(_ context.Context, _, _ time.Time, _ *uuid.UUID) (*model.VelocityStatsDTO, error) {
	return m.velocity, m.velocityErr
}

func (m *mockStatsService) AgentPerformance(_ context.Context, _, _ time.Time, _ *uuid.UUID) (*model.AgentPerformanceDTO, error) {
	return m.perf, m.perfErr
}

func TestStatsHandler_VelocityReturnsCostAwarePoints(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewStatsHandler(&mockStatsService{
		velocity: &model.VelocityStatsDTO{
			Points: []model.VelocityPointDTO{
				{Period: "2026-03-28", TasksCompleted: 2, CostUsd: 5.25},
			},
			TotalCompleted: 2,
			TotalCostUsd:   5.25,
			AvgPerDay:      1,
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/velocity", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Velocity(c); err != nil {
		t.Fatalf("Velocity() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	points, ok := payload["points"].([]any)
	if !ok || len(points) != 1 {
		t.Fatalf("points = %#v", payload["points"])
	}
	point := points[0].(map[string]any)
	if _, ok := point["costUsd"]; !ok {
		t.Fatalf("velocity point missing costUsd: %#v", point)
	}
}

func TestStatsHandler_AgentPerformanceReturnsTruthfulBucketFields(t *testing.T) {
	e := newAgentTestEcho()
	h := handler.NewStatsHandler(&mockStatsService{
		perf: &model.AgentPerformanceDTO{
			Entries: []model.AgentPerformanceEntryDTO{
				{
					BucketID:           "planner",
					Label:              "planner",
					RunCount:           3,
					SuccessRate:        66.67,
					AvgCostUsd:         1.5,
					AvgDurationMinutes: 18,
					TotalCostUsd:       4.5,
				},
			},
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/agent-performance", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.AgentPerformance(c); err != nil {
		t.Fatalf("AgentPerformance() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	entries, ok := payload["entries"].([]any)
	if !ok || len(entries) != 1 {
		t.Fatalf("entries = %#v", payload["entries"])
	}
	entry := entries[0].(map[string]any)
	for _, key := range []string{"bucketId", "label", "avgDurationMinutes"} {
		if _, ok := entry[key]; !ok {
			t.Fatalf("agent performance entry missing %q: %#v", key, entry)
		}
	}
}
