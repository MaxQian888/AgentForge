package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentforge/server/internal/handler"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type dispatchPreflightTaskRepoStub struct {
	task *model.Task
}

func (s *dispatchPreflightTaskRepoStub) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if s.task == nil || s.task.ID != id {
		return nil, service.ErrAgentTaskNotFound
	}
	cloned := *s.task
	return &cloned, nil
}

type dispatchPreflightMemberRepoStub struct {
	member *model.Member
}

func (s *dispatchPreflightMemberRepoStub) GetByID(_ context.Context, id uuid.UUID) (*model.Member, error) {
	if s.member == nil || s.member.ID != id {
		return nil, service.ErrDispatchMemberNotFound
	}
	cloned := *s.member
	return &cloned, nil
}

type dispatchPreflightPoolStatsStub struct {
	stats model.AgentPoolStatsDTO
}

func (s *dispatchPreflightPoolStatsStub) PoolStats(context.Context) model.AgentPoolStatsDTO {
	return s.stats
}

type dispatchPreflightBudgetCheckerStub struct {
	result *service.BudgetCheckResult
	err    error
}

func (s *dispatchPreflightBudgetCheckerStub) CheckBudget(context.Context, uuid.UUID, *uuid.UUID, float64) (*service.BudgetCheckResult, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.result == nil {
		return &service.BudgetCheckResult{Allowed: true}, nil
	}
	cloned := *s.result
	return &cloned, nil
}

type dispatchPreflightRunReaderStub struct {
	runs []*model.AgentRun
}

func (s *dispatchPreflightRunReaderStub) GetByTask(context.Context, uuid.UUID) ([]*model.AgentRun, error) {
	cloned := make([]*model.AgentRun, 0, len(s.runs))
	for _, run := range s.runs {
		if run == nil {
			continue
		}
		copyRun := *run
		cloned = append(cloned, &copyRun)
	}
	return cloned, nil
}

func newDispatchPreflightContext(e *echo.Echo, projectID, taskID, memberID uuid.UUID) echo.Context {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String()+"/dispatch/preflight?taskId="+taskID.String()+"&memberId="+memberID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)
	c.SetParamNames("pid")
	c.SetParamValues(projectID.String())
	return c
}

func newDispatchPreflightContextWithQuery(e *echo.Echo, projectID, taskID, memberID uuid.UUID, rawQuery string) echo.Context {
	path := "/api/v1/projects/" + projectID.String() + "/dispatch/preflight?taskId=" + taskID.String() + "&memberId=" + memberID.String()
	if rawQuery != "" {
		path += "&" + rawQuery
	}
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(appMiddleware.ProjectIDContextKey, projectID)
	c.SetParamNames("pid")
	c.SetParamValues(projectID.String())
	return c
}

func TestDispatchPreflightHandler_GetEligibleDispatch(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	handlerUnderTest := handler.NewDispatchPreflightHandler(
		&dispatchPreflightTaskRepoStub{task: &model.Task{ID: taskID, ProjectID: projectID, BudgetUsd: 4}},
		&dispatchPreflightMemberRepoStub{member: &model.Member{ID: memberID, ProjectID: projectID, Type: model.MemberTypeAgent, IsActive: true}},
		nil,
		&dispatchPreflightPoolStatsStub{stats: model.AgentPoolStatsDTO{Active: 2, Available: 1, Queued: 3}},
	)

	c := newDispatchPreflightContext(e, projectID, taskID, memberID)

	if err := handlerUnderTest.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if c.Response().Status != http.StatusOK {
		t.Fatalf("status = %d, want 200", c.Response().Status)
	}

	resp := decodeJSON[handler.PreflightResponse](t, c.Response().Writer.(*httptest.ResponseRecorder))
	if !resp.AdmissionLikely || resp.DispatchOutcomeHint != model.DispatchStatusStarted {
		t.Fatalf("response = %+v, want likely started", resp)
	}
	if resp.PoolActive == nil || *resp.PoolActive != 2 || resp.PoolAvailable == nil || *resp.PoolAvailable != 1 || resp.PoolQueued == nil || *resp.PoolQueued != 3 {
		t.Fatalf("pool stats = %+v", resp)
	}
}

func TestDispatchPreflightHandler_GetReturnsBudgetWarning(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	handlerUnderTest := handler.NewDispatchPreflightHandler(
		&dispatchPreflightTaskRepoStub{task: &model.Task{ID: taskID, ProjectID: projectID, BudgetUsd: 4}},
		&dispatchPreflightMemberRepoStub{member: &model.Member{ID: memberID, ProjectID: projectID, Type: model.MemberTypeAgent, IsActive: true}},
		&dispatchPreflightBudgetCheckerStub{result: &service.BudgetCheckResult{Allowed: true, Warning: true, Scope: "sprint", WarningMessage: "sprint budget warning"}},
		&dispatchPreflightPoolStatsStub{stats: model.AgentPoolStatsDTO{Available: 2}},
	)

	c := newDispatchPreflightContext(e, projectID, taskID, memberID)

	if err := handlerUnderTest.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	resp := decodeJSON[handler.PreflightResponse](t, c.Response().Writer.(*httptest.ResponseRecorder))
	if resp.BudgetWarning == nil || resp.BudgetWarning.Scope != "sprint" {
		t.Fatalf("response = %+v, want sprint warning", resp)
	}
	if !resp.AdmissionLikely || resp.DispatchOutcomeHint != model.DispatchStatusStarted {
		t.Fatalf("response = %+v, want likely started", resp)
	}
}

func TestDispatchPreflightHandler_GetReturnsBudgetBlocked(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	handlerUnderTest := handler.NewDispatchPreflightHandler(
		&dispatchPreflightTaskRepoStub{task: &model.Task{ID: taskID, ProjectID: projectID, BudgetUsd: 4}},
		&dispatchPreflightMemberRepoStub{member: &model.Member{ID: memberID, ProjectID: projectID, Type: model.MemberTypeAgent, IsActive: true}},
		&dispatchPreflightBudgetCheckerStub{result: &service.BudgetCheckResult{Allowed: false, Scope: "project", Reason: "project budget exceeded"}},
		&dispatchPreflightPoolStatsStub{stats: model.AgentPoolStatsDTO{Available: 0, Queued: 4}},
	)

	c := newDispatchPreflightContext(e, projectID, taskID, memberID)

	if err := handlerUnderTest.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	resp := decodeJSON[handler.PreflightResponse](t, c.Response().Writer.(*httptest.ResponseRecorder))
	if resp.BudgetBlocked == nil || resp.BudgetBlocked.Scope != "project" {
		t.Fatalf("response = %+v, want project block", resp)
	}
	if resp.AdmissionLikely || resp.DispatchOutcomeHint != model.DispatchStatusBlocked {
		t.Fatalf("response = %+v, want blocked", resp)
	}
	if resp.PoolAvailable != nil || resp.PoolQueued != nil {
		t.Fatalf("blocked response should short-circuit before pool stats: %+v", resp)
	}
}

func TestDispatchPreflightHandler_GetReturnsSkippedForNonAgentMember(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	handlerUnderTest := handler.NewDispatchPreflightHandler(
		&dispatchPreflightTaskRepoStub{task: &model.Task{ID: taskID, ProjectID: projectID, BudgetUsd: 4}},
		&dispatchPreflightMemberRepoStub{member: &model.Member{ID: memberID, ProjectID: projectID, Type: model.MemberTypeHuman, IsActive: true}},
		nil,
		&dispatchPreflightPoolStatsStub{stats: model.AgentPoolStatsDTO{Available: 1}},
	)

	c := newDispatchPreflightContext(e, projectID, taskID, memberID)

	if err := handlerUnderTest.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	resp := decodeJSON[handler.PreflightResponse](t, c.Response().Writer.(*httptest.ResponseRecorder))
	if resp.DispatchOutcomeHint != model.DispatchStatusSkipped || resp.AdmissionLikely {
		t.Fatalf("response = %+v, want skipped", resp)
	}
	if resp.PoolActive != nil || resp.BudgetWarning != nil || resp.BudgetBlocked != nil {
		t.Fatalf("skipped response should omit runtime-only fields: %+v", resp)
	}
}

func TestDispatchPreflightHandler_GetBlocksWhenTaskBudgetWouldBeExceeded(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	handlerUnderTest := handler.NewDispatchPreflightHandler(
		&dispatchPreflightTaskRepoStub{task: &model.Task{ID: taskID, ProjectID: projectID, BudgetUsd: 3, SpentUsd: 2.75}},
		&dispatchPreflightMemberRepoStub{member: &model.Member{ID: memberID, ProjectID: projectID, Type: model.MemberTypeAgent, IsActive: true}},
		nil,
		&dispatchPreflightPoolStatsStub{stats: model.AgentPoolStatsDTO{Available: 2}},
	)

	c := newDispatchPreflightContextWithQuery(
		e,
		projectID,
		taskID,
		memberID,
		"runtime=codex&provider=openai&model=gpt-5-codex&roleId=frontend-developer&budgetUsd=0.5",
	)

	if err := handlerUnderTest.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	type responseShape struct {
		AdmissionLikely     bool   `json:"admissionLikely"`
		DispatchOutcomeHint string `json:"dispatchOutcomeHint"`
		GuardrailType       string `json:"guardrailType"`
		GuardrailScope      string `json:"guardrailScope"`
		Runtime             string `json:"runtime"`
		Provider            string `json:"provider"`
		Model               string `json:"model"`
		RoleID              string `json:"roleId"`
		BudgetBlocked       *struct {
			Scope   string `json:"scope"`
			Message string `json:"message"`
		} `json:"budgetBlocked"`
	}

	resp := decodeJSON[responseShape](t, c.Response().Writer.(*httptest.ResponseRecorder))
	if resp.DispatchOutcomeHint != model.DispatchStatusBlocked || resp.AdmissionLikely {
		t.Fatalf("response = %+v, want blocked advisory", resp)
	}
	if resp.GuardrailType != model.DispatchGuardrailTypeBudget || resp.GuardrailScope != "task" {
		t.Fatalf("guardrail = %+v, want task budget", resp)
	}
	if resp.BudgetBlocked == nil || resp.BudgetBlocked.Scope != "task" {
		t.Fatalf("response = %+v, want task budgetBlocked", resp)
	}
	if resp.Runtime != "codex" || resp.Provider != "openai" || resp.Model != "gpt-5-codex" || resp.RoleID != "frontend-developer" {
		t.Fatalf("dispatch tuple = %+v", resp)
	}
}

func TestDispatchPreflightHandler_GetBlocksWhenTaskAlreadyHasPausedRun(t *testing.T) {
	e := newAgentTestEcho()
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	handlerUnderTest := handler.NewDispatchPreflightHandler(
		&dispatchPreflightTaskRepoStub{task: &model.Task{ID: taskID, ProjectID: projectID}},
		&dispatchPreflightMemberRepoStub{member: &model.Member{ID: memberID, ProjectID: projectID, Type: model.MemberTypeAgent, IsActive: true}},
		nil,
		&dispatchPreflightPoolStatsStub{stats: model.AgentPoolStatsDTO{Available: 1}},
	).WithRunReader(&dispatchPreflightRunReaderStub{
		runs: []*model.AgentRun{{ID: uuid.New(), TaskID: taskID, MemberID: memberID, Status: model.AgentRunStatusPaused}},
	})

	c := newDispatchPreflightContext(e, projectID, taskID, memberID)

	if err := handlerUnderTest.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	type responseShape struct {
		AdmissionLikely     bool   `json:"admissionLikely"`
		DispatchOutcomeHint string `json:"dispatchOutcomeHint"`
		GuardrailType       string `json:"guardrailType"`
		GuardrailScope      string `json:"guardrailScope"`
		BudgetBlocked       any    `json:"budgetBlocked"`
	}

	resp := decodeJSON[responseShape](t, c.Response().Writer.(*httptest.ResponseRecorder))
	if resp.DispatchOutcomeHint != model.DispatchStatusBlocked || resp.AdmissionLikely {
		t.Fatalf("response = %+v, want blocked advisory", resp)
	}
	if resp.GuardrailType != model.DispatchGuardrailTypeTask || resp.GuardrailScope != "task" {
		t.Fatalf("guardrail = %+v, want task conflict", resp)
	}
	if resp.BudgetBlocked != nil {
		t.Fatalf("response = %+v, want non-budget block", resp)
	}
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var payload T
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode JSON: %v", err)
	}
	return payload
}
