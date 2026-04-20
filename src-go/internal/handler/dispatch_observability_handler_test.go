package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentforge/server/internal/handler"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
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
			{ID: uuid.New(), ProjectID: projectID, TaskID: taskID, MemberID: &memberID, Outcome: model.DispatchStatusBlocked, TriggerSource: "assignment", Runtime: "codex", Provider: "openai", Model: "gpt-5-codex", GuardrailType: model.DispatchGuardrailTypeBudget, GuardrailScope: "project", CreatedAt: base},
			{ID: uuid.New(), ProjectID: projectID, TaskID: taskID, MemberID: &memberID, Outcome: model.DispatchStatusQueued, TriggerSource: "manual", Runtime: "codex", Provider: "openai", Model: "gpt-5-codex", QueueEntryID: "entry-1", CreatedAt: base.Add(-time.Minute)},
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
	if response[0].Runtime != "codex" || response[0].Provider != "openai" || response[0].GuardrailType != model.DispatchGuardrailTypeBudget {
		t.Fatalf("history[0] = %+v", response[0])
	}
	if response[1].QueueEntryID != "entry-1" {
		t.Fatalf("history[1] = %+v", response[1])
	}
}

func TestDispatchStatsHandler_GetSupportsTimeWindowAndPromotionMetrics(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	base := time.Now().UTC().Truncate(time.Second)
	statsHandler := handler.NewDispatchStatsHandler(
		&dispatchAttemptReaderStub{byProject: []*model.DispatchAttempt{
			{ID: uuid.New(), ProjectID: projectID, TaskID: uuid.New(), Outcome: model.DispatchStatusStarted, TriggerSource: "assignment", CreatedAt: base.Add(-30 * time.Minute)},
			{ID: uuid.New(), ProjectID: projectID, TaskID: uuid.New(), Outcome: model.DispatchStatusBlocked, TriggerSource: "promotion", GuardrailType: model.DispatchGuardrailTypeBudget, RecoveryDisposition: model.QueueRecoveryDispositionRecoverable, CreatedAt: base.Add(-10 * time.Minute)},
			{ID: uuid.New(), ProjectID: projectID, TaskID: uuid.New(), Outcome: model.DispatchStatusQueued, TriggerSource: "manual", CreatedAt: base.Add(-2 * time.Hour)},
		}},
		&dispatchQueueStatsReaderStub{
			queueDepth: 3,
			entries: []*model.AgentPoolQueueEntry{
				{EntryID: uuid.NewString(), Status: model.AgentPoolQueueStatusPromoted, RecoveryDisposition: model.QueueRecoveryDispositionPromoted, CreatedAt: base.Add(-50 * time.Minute), UpdatedAt: base.Add(-30 * time.Minute)},
				{EntryID: uuid.NewString(), Status: model.AgentPoolQueueStatusCancelled, RecoveryDisposition: model.QueueRecoveryDispositionCancelled, CreatedAt: base.Add(-40 * time.Minute), UpdatedAt: base.Add(-20 * time.Minute)},
				{EntryID: uuid.NewString(), Status: model.AgentPoolQueueStatusFailed, RecoveryDisposition: model.QueueRecoveryDispositionTerminal, CreatedAt: base.Add(-35 * time.Minute), UpdatedAt: base.Add(-15 * time.Minute)},
				{EntryID: uuid.NewString(), Status: model.AgentPoolQueueStatusQueued, RecoveryDisposition: model.QueueRecoveryDispositionRecoverable, CreatedAt: base.Add(-3 * time.Hour), UpdatedAt: base.Add(-3 * time.Hour)},
			},
		},
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/dispatch/stats?since="+base.Add(-time.Hour).Format(time.RFC3339)+"&until="+base.Format(time.RFC3339), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)

	if err := statsHandler.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	type responseShape struct {
		Outcomes                  map[string]int `json:"outcomes"`
		BlockedReasons            map[string]int `json:"blockedReasons"`
		QueueDepth                int            `json:"queueDepth"`
		CancelledWithoutPromotion int            `json:"cancelledWithoutPromotion"`
		TerminalPromotionFailures int            `json:"terminalPromotionFailures"`
		PromotionSuccessRate      *float64       `json:"promotionSuccessRate"`
		MedianWaitSeconds         *float64       `json:"medianWaitSeconds"`
	}

	var response responseShape
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	if response.Outcomes[model.DispatchStatusStarted] != 1 || response.Outcomes[model.DispatchStatusBlocked] != 1 {
		t.Fatalf("outcomes = %+v", response.Outcomes)
	}
	if response.Outcomes[model.DispatchStatusQueued] != 0 {
		t.Fatalf("queued outcomes should be filtered out by since window, got %+v", response.Outcomes)
	}
	if response.BlockedReasons[model.DispatchGuardrailTypeBudget] != 1 {
		t.Fatalf("blocked reasons = %+v", response.BlockedReasons)
	}
	if response.QueueDepth != 3 {
		t.Fatalf("queueDepth = %d, want 3", response.QueueDepth)
	}
	if response.CancelledWithoutPromotion != 1 || response.TerminalPromotionFailures != 1 {
		t.Fatalf("promotion lifecycle counts = %+v", response)
	}
	if response.PromotionSuccessRate == nil || *response.PromotionSuccessRate != 1.0/3.0 {
		t.Fatalf("promotionSuccessRate = %v, want 1/3", response.PromotionSuccessRate)
	}
	if response.MedianWaitSeconds == nil || *response.MedianWaitSeconds != 20*60 {
		t.Fatalf("medianWaitSeconds = %v, want 1200", response.MedianWaitSeconds)
	}
}

func TestDispatchHistoryHandler_GetIncludesRecoveryDisposition(t *testing.T) {
	e := newAgentTestEcho()
	taskID := uuid.New()
	projectID := uuid.New()
	memberID := uuid.New()
	base := time.Now().UTC()
	historyHandler := handler.NewDispatchHistoryHandler(
		&dispatchAttemptReaderStub{byTask: []*model.DispatchAttempt{
			{
				ID:                  uuid.New(),
				ProjectID:           projectID,
				TaskID:              taskID,
				MemberID:            &memberID,
				Outcome:             model.DispatchStatusBlocked,
				TriggerSource:       "promotion",
				Reason:              "project budget exceeded",
				Runtime:             "codex",
				Provider:            "openai",
				Model:               "gpt-5-codex",
				QueueEntryID:        "entry-1",
				GuardrailType:       model.DispatchGuardrailTypeBudget,
				GuardrailScope:      "project",
				RecoveryDisposition: model.QueueRecoveryDispositionRecoverable,
				CreatedAt:           base,
			},
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
	if len(response) != 1 {
		t.Fatalf("len(response) = %d, want 1", len(response))
	}
	if response[0].RecoveryDisposition != model.QueueRecoveryDispositionRecoverable {
		t.Fatalf("history[0] = %+v, want recoverable disposition", response[0])
	}
}
