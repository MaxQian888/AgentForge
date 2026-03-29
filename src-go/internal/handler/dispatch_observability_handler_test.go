package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/handler"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

type dispatchAttemptReaderStub struct {
	byTask    []*model.DispatchAttempt
	byProject []*model.DispatchAttempt
}

func (s *dispatchAttemptReaderStub) ListByTaskID(context.Context, uuid.UUID, int) ([]*model.DispatchAttempt, error) {
	return append([]*model.DispatchAttempt(nil), s.byTask...), nil
}

func (s *dispatchAttemptReaderStub) ListByProjectID(context.Context, uuid.UUID, int) ([]*model.DispatchAttempt, error) {
	return append([]*model.DispatchAttempt(nil), s.byProject...), nil
}

type dispatchQueueStatsReaderStub struct {
	queueDepth int
	entries    []*model.AgentPoolQueueEntry
}

func (s *dispatchQueueStatsReaderStub) CountQueuedByProject(context.Context, uuid.UUID) (int, error) {
	return s.queueDepth, nil
}

func (s *dispatchQueueStatsReaderStub) ListRecentByProject(context.Context, uuid.UUID, int) ([]*model.AgentPoolQueueEntry, error) {
	return append([]*model.AgentPoolQueueEntry(nil), s.entries...), nil
}

func TestDispatchStatsHandler_GetAggregatesAttemptsAndQueue(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	base := time.Now().UTC()
	statsHandler := handler.NewDispatchStatsHandler(
		&dispatchAttemptReaderStub{byProject: []*model.DispatchAttempt{
			{ID: uuid.New(), ProjectID: projectID, TaskID: uuid.New(), Outcome: model.DispatchStatusStarted, TriggerSource: "assignment", CreatedAt: base},
			{ID: uuid.New(), ProjectID: projectID, TaskID: uuid.New(), Outcome: model.DispatchStatusQueued, TriggerSource: "manual", CreatedAt: base},
			{ID: uuid.New(), ProjectID: projectID, TaskID: uuid.New(), Outcome: model.DispatchStatusBlocked, TriggerSource: "workflow", GuardrailType: model.DispatchGuardrailTypeBudget, CreatedAt: base},
		}},
		&dispatchQueueStatsReaderStub{
			queueDepth: 2,
			entries: []*model.AgentPoolQueueEntry{
				{EntryID: uuid.NewString(), Status: model.AgentPoolQueueStatusPromoted, CreatedAt: base.Add(-40 * time.Second), UpdatedAt: base.Add(-20 * time.Second)},
				{EntryID: uuid.NewString(), Status: model.AgentPoolQueueStatusPromoted, CreatedAt: base.Add(-30 * time.Second), UpdatedAt: base.Add(-10 * time.Second)},
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/dispatch/stats", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	if err := statsHandler.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	var response handler.DispatchStatsResponse
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if response.Outcomes[model.DispatchStatusStarted] != 1 || response.Outcomes[model.DispatchStatusQueued] != 1 || response.Outcomes[model.DispatchStatusBlocked] != 1 {
		t.Fatalf("outcomes = %+v", response.Outcomes)
	}
	if response.BlockedReasons[model.DispatchGuardrailTypeBudget] != 1 {
		t.Fatalf("blocked reasons = %+v", response.BlockedReasons)
	}
	if response.QueueDepth != 2 {
		t.Fatalf("queueDepth = %d, want 2", response.QueueDepth)
	}
	if response.MedianWaitSeconds == nil || *response.MedianWaitSeconds != 20 {
		t.Fatalf("medianWaitSeconds = %v, want 20", response.MedianWaitSeconds)
	}
}

func TestDispatchHistoryHandler_GetReturnsChronologicalAttempts(t *testing.T) {
	e := newAgentTestEcho()
	taskID := uuid.New()
	projectID := uuid.New()
	memberID := uuid.New()
	base := time.Now().UTC()
	historyHandler := handler.NewDispatchHistoryHandler(
		&dispatchAttemptReaderStub{byTask: []*model.DispatchAttempt{
			{ID: uuid.New(), ProjectID: projectID, TaskID: taskID, MemberID: &memberID, Outcome: model.DispatchStatusBlocked, TriggerSource: "assignment", CreatedAt: base},
			{ID: uuid.New(), ProjectID: projectID, TaskID: taskID, MemberID: &memberID, Outcome: model.DispatchStatusQueued, TriggerSource: "manual", CreatedAt: base.Add(-time.Minute)},
		}},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+taskID.String()+"/dispatch/history", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("tid")
	c.SetParamValues(taskID.String())

	if err := historyHandler.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	var response []model.DispatchAttemptDTO
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if len(response) != 2 {
		t.Fatalf("len(response) = %d, want 2", len(response))
	}
	if response[0].Outcome != model.DispatchStatusBlocked || response[1].Outcome != model.DispatchStatusQueued {
		t.Fatalf("history = %+v", response)
	}
}
