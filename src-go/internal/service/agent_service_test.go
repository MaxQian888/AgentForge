package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/pool"
	"github.com/react-go-quick-starter/server/internal/repository"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/worktree"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type mockAgentRunRepo struct {
	runs       map[uuid.UUID]*model.AgentRun
	runsByTask map[uuid.UUID][]*model.AgentRun
}

func newMockAgentRunRepo() *mockAgentRunRepo {
	return &mockAgentRunRepo{
		runs:       make(map[uuid.UUID]*model.AgentRun),
		runsByTask: make(map[uuid.UUID][]*model.AgentRun),
	}
}

func (m *mockAgentRunRepo) Create(_ context.Context, run *model.AgentRun) error {
	cloned := *run
	m.runs[run.ID] = &cloned
	m.runsByTask[run.TaskID] = append(m.runsByTask[run.TaskID], &cloned)
	return nil
}

func (m *mockAgentRunRepo) GetByID(_ context.Context, id uuid.UUID) (*model.AgentRun, error) {
	run, ok := m.runs[id]
	if !ok {
		return nil, service.ErrAgentNotFound
	}
	cloned := *run
	return &cloned, nil
}

func (m *mockAgentRunRepo) GetByTask(_ context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	runs := m.runsByTask[taskID]
	if len(runs) == 0 {
		for _, run := range m.runs {
			if run.TaskID != taskID {
				continue
			}
			cloned := *run
			runs = append(runs, &cloned)
		}
	}
	out := make([]*model.AgentRun, 0, len(runs))
	for _, run := range runs {
		cloned := *run
		out = append(out, &cloned)
	}
	return out, nil
}

func (m *mockAgentRunRepo) ListActive(_ context.Context) ([]*model.AgentRun, error) {
	return nil, nil
}

func (m *mockAgentRunRepo) UpdateStatus(_ context.Context, id uuid.UUID, status string) error {
	run, ok := m.runs[id]
	if !ok {
		return service.ErrAgentNotFound
	}
	run.Status = status
	return nil
}

func (m *mockAgentRunRepo) UpdateCost(_ context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int, costAccounting *model.CostAccountingSnapshot) error {
	run, ok := m.runs[id]
	if !ok {
		return service.ErrAgentNotFound
	}
	run.InputTokens = inputTokens
	run.OutputTokens = outputTokens
	run.CacheReadTokens = cacheReadTokens
	run.CostUsd = costUsd
	run.TurnCount = turnCount
	run.CostAccounting = costAccounting.Clone()
	return nil
}

func (m *mockAgentRunRepo) UpdateStructuredOutput(_ context.Context, id uuid.UUID, output json.RawMessage) error {
	run, ok := m.runs[id]
	if !ok {
		return service.ErrAgentNotFound
	}
	run.StructuredOutput = output
	return nil
}

type mockAgentBridge struct {
	executeErr   error
	lastExecute  service.BridgeExecuteRequest
	cancelTaskID string
	cancelReason string
	pauseTaskID  string
	pauseReason  string
	resumeReq    service.BridgeExecuteRequest
	poolSummary  *bridgeclient.PoolSummaryResponse
	healthErr    error
	healthSeq    []error
	healthCalls  int
}

func (m *mockAgentBridge) Execute(_ context.Context, req service.BridgeExecuteRequest) (*service.BridgeExecuteResponse, error) {
	m.lastExecute = req
	if m.executeErr != nil {
		return nil, m.executeErr
	}
	return &service.BridgeExecuteResponse{SessionID: req.TaskID + "-session"}, nil
}

func (m *mockAgentBridge) GetStatus(_ context.Context, _ string) (*service.BridgeStatusResponse, error) {
	return nil, nil
}

func (m *mockAgentBridge) GetPoolSummary(_ context.Context) (*bridgeclient.PoolSummaryResponse, error) {
	return m.poolSummary, nil
}

func (m *mockAgentBridge) Health(_ context.Context) error {
	m.healthCalls++
	if len(m.healthSeq) > 0 {
		err := m.healthSeq[0]
		m.healthSeq = m.healthSeq[1:]
		return err
	}
	return m.healthErr
}

func (m *mockAgentBridge) Cancel(_ context.Context, taskID, reason string) error {
	m.cancelTaskID = taskID
	m.cancelReason = reason
	return nil
}

func (m *mockAgentBridge) Pause(_ context.Context, taskID, reason string) (*service.BridgePauseResponse, error) {
	m.pauseTaskID = taskID
	m.pauseReason = reason
	return &service.BridgePauseResponse{SessionID: taskID + "-session", Status: model.AgentRunStatusPaused}, nil
}

func (m *mockAgentBridge) Resume(_ context.Context, req service.BridgeExecuteRequest) (*service.BridgeResumeResponse, error) {
	m.resumeReq = req
	return &service.BridgeResumeResponse{SessionID: req.SessionID, Resumed: true}, nil
}

type mockAgentIMProgressNotifier struct {
	requests []service.IMBoundProgressRequest
	queued   bool
	err      error
}

func (m *mockAgentIMProgressNotifier) QueueBoundProgress(_ context.Context, req service.IMBoundProgressRequest) (bool, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return false, m.err
	}
	if !m.queued {
		return true, nil
	}
	return m.queued, nil
}

type mockAgentIMNotifier struct {
	requests []*model.IMNotifyRequest
}

func (m *mockAgentIMNotifier) Notify(_ context.Context, req *model.IMNotifyRequest) error {
	if req == nil {
		return nil
	}
	cloned := *req
	m.requests = append(m.requests, &cloned)
	return nil
}

type bridgeIMDeliveryListener struct {
	deliveries []*model.IMControlDelivery
}

func (l *bridgeIMDeliveryListener) Send(_ context.Context, delivery *model.IMControlDelivery) error {
	l.deliveries = append(l.deliveries, delivery)
	return nil
}

func (l *bridgeIMDeliveryListener) Close() error { return nil }

type mockAgentTaskRepo struct {
	task             *model.Task
	tasks            map[uuid.UUID]*model.Task
	updatedBranch    string
	updatedWorktree  string
	updatedSession   string
	updatedSpent     float64
	updatedStatus    string
	updateSpentCalls int
	clearCalls       int
}

func (m *mockAgentTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if m.tasks != nil {
		task, ok := m.tasks[id]
		if !ok {
			return nil, service.ErrAgentTaskNotFound
		}
		cloned := *task
		return &cloned, nil
	}
	if m.task == nil || m.task.ID != id {
		return nil, service.ErrAgentTaskNotFound
	}
	cloned := *m.task
	return &cloned, nil
}

func (m *mockAgentTaskRepo) UpdateRuntime(_ context.Context, id uuid.UUID, branch, worktreePath, sessionID string) error {
	m.updatedBranch = branch
	m.updatedWorktree = worktreePath
	m.updatedSession = sessionID
	if m.tasks != nil {
		if task, ok := m.tasks[id]; ok {
			task.AgentBranch = branch
			task.AgentWorktree = worktreePath
			task.AgentSessionID = sessionID
		}
	}
	if m.task != nil {
		m.task.AgentBranch = branch
		m.task.AgentWorktree = worktreePath
		m.task.AgentSessionID = sessionID
	}
	return nil
}

func (m *mockAgentTaskRepo) ClearRuntime(_ context.Context, id uuid.UUID) error {
	m.clearCalls++
	if m.tasks != nil {
		if task, ok := m.tasks[id]; ok {
			task.AgentBranch = ""
			task.AgentWorktree = ""
			task.AgentSessionID = ""
		}
	}
	if m.task != nil {
		m.task.AgentBranch = ""
		m.task.AgentWorktree = ""
		m.task.AgentSessionID = ""
	}
	return nil
}

func (m *mockAgentTaskRepo) UpdateSpent(_ context.Context, id uuid.UUID, spentUsd float64, status string) error {
	m.updateSpentCalls++
	m.updatedSpent = spentUsd
	m.updatedStatus = status
	if m.tasks != nil {
		if task, ok := m.tasks[id]; ok {
			task.SpentUsd = spentUsd
			if status != "" {
				task.Status = status
			}
		}
	}
	if m.task != nil {
		m.task.SpentUsd = spentUsd
		if status != "" {
			m.task.Status = status
		}
	}
	return nil
}

type mockAgentProjectRepo struct {
	project *model.Project
}

func (m *mockAgentProjectRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Project, error) {
	if m.project == nil || m.project.ID != id {
		return nil, service.ErrAgentProjectNotFound
	}
	cloned := *m.project
	return &cloned, nil
}

type mockAgentDispatchMemberRepo struct {
	member *model.Member
}

func (m *mockAgentDispatchMemberRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Member, error) {
	if m.member == nil || m.member.ID != id {
		return nil, service.ErrDispatchMemberNotFound
	}
	cloned := *m.member
	return &cloned, nil
}

type mockAgentRoleStore struct {
	roles     map[string]*rolepkg.Manifest
	skillsDir string
}

func (m *mockAgentRoleStore) Get(id string) (*rolepkg.Manifest, error) {
	if manifest, ok := m.roles[id]; ok {
		cloned := *manifest
		return &cloned, nil
	}
	return nil, os.ErrNotExist
}

func (m *mockAgentRoleStore) SkillsDir() string {
	return m.skillsDir
}

type mockAgentPluginCatalog struct {
	records []*model.PluginRecord
}

func (m *mockAgentPluginCatalog) List(_ context.Context, _ service.PluginListFilter) ([]*model.PluginRecord, error) {
	return m.records, nil
}

type mockAgentQueueStore struct {
	queued        []*model.AgentPoolQueueEntry
	completed     []string
	next          *model.AgentPoolQueueEntry
	listRecentErr error
	completeErr   error
}

func (m *mockAgentQueueStore) QueueAgentAdmission(_ context.Context, input service.QueueAgentAdmissionInput) (*model.AgentPoolQueueEntry, error) {
	recoveryDisposition := input.RecoveryDisposition
	if recoveryDisposition == "" {
		recoveryDisposition = model.QueueRecoveryDispositionPending
	}
	entry := &model.AgentPoolQueueEntry{
		EntryID:             uuid.NewString(),
		ProjectID:           input.ProjectID.String(),
		TaskID:              input.TaskID.String(),
		MemberID:            input.MemberID.String(),
		Status:              model.AgentPoolQueueStatusQueued,
		Reason:              "agent pool is at capacity",
		Runtime:             input.Runtime,
		Provider:            input.Provider,
		Model:               input.Model,
		RoleID:              input.RoleID,
		BudgetUSD:           input.BudgetUSD,
		GuardrailType:       input.GuardrailType,
		GuardrailScope:      input.GuardrailScope,
		RecoveryDisposition: recoveryDisposition,
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}
	m.queued = append(m.queued, entry)
	return entry, nil
}

func (m *mockAgentQueueStore) CountQueuedByProject(_ context.Context, projectID uuid.UUID) (int, error) {
	count := 0
	for _, entry := range m.queued {
		if entry.ProjectID == projectID.String() && entry.Status == model.AgentPoolQueueStatusQueued {
			count++
		}
	}
	return count, nil
}

func (m *mockAgentQueueStore) ListAllQueued(_ context.Context, limit int) ([]*model.AgentPoolQueueEntry, error) {
	if limit <= 0 || limit > len(m.queued) {
		limit = len(m.queued)
	}
	results := make([]*model.AgentPoolQueueEntry, 0, limit)
	for i := 0; i < len(m.queued) && len(results) < limit; i++ {
		entry := m.queued[i]
		if entry == nil || entry.Status != model.AgentPoolQueueStatusQueued {
			continue
		}
		cloned := *entry
		results = append(results, &cloned)
	}
	return results, nil
}

func (m *mockAgentQueueStore) ListQueuedByProject(_ context.Context, projectID uuid.UUID, limit int) ([]*model.AgentPoolQueueEntry, error) {
	if limit <= 0 {
		limit = len(m.queued)
	}
	var results []*model.AgentPoolQueueEntry
	for _, entry := range m.queued {
		if entry.ProjectID == projectID.String() && entry.Status == model.AgentPoolQueueStatusQueued {
			cloned := *entry
			results = append(results, &cloned)
			if len(results) == limit {
				break
			}
		}
	}
	return results, nil
}

func (m *mockAgentQueueStore) ListRecentByProject(_ context.Context, projectID uuid.UUID, limit int) ([]*model.AgentPoolQueueEntry, error) {
	if m.listRecentErr != nil {
		return nil, m.listRecentErr
	}
	if limit <= 0 {
		limit = len(m.queued)
	}
	results := make([]*model.AgentPoolQueueEntry, 0, len(m.queued))
	for _, entry := range m.queued {
		if entry.ProjectID != projectID.String() {
			continue
		}
		cloned := *entry
		results = append(results, &cloned)
		if len(results) == limit {
			break
		}
	}
	return results, nil
}

func (m *mockAgentQueueStore) ReserveNextQueuedByProject(_ context.Context, projectID uuid.UUID) (*model.AgentPoolQueueEntry, error) {
	if m.next != nil && m.next.ProjectID == projectID.String() {
		cloned := *m.next
		return &cloned, nil
	}
	for _, entry := range m.queued {
		if entry.ProjectID == projectID.String() && entry.Status == model.AgentPoolQueueStatusQueued {
			entry.Status = model.AgentPoolQueueStatusAdmitted
			cloned := *entry
			return &cloned, nil
		}
	}
	return nil, nil
}

func (m *mockAgentQueueStore) CompleteQueuedEntry(_ context.Context, entryID string, status model.AgentPoolQueueStatus, reason string, runID *uuid.UUID, guardrailType string, guardrailScope string, recoveryDisposition string) error {
	if m.completeErr != nil {
		return m.completeErr
	}
	m.completed = append(m.completed, entryID+":"+string(status)+":"+reason+":"+guardrailType+":"+guardrailScope+":"+recoveryDisposition)
	for _, entry := range m.queued {
		if entry.EntryID != entryID {
			continue
		}
		entry.Status = status
		entry.Reason = reason
		entry.GuardrailType = guardrailType
		entry.GuardrailScope = guardrailScope
		entry.RecoveryDisposition = recoveryDisposition
		if runID != nil {
			id := runID.String()
			entry.AgentRunID = &id
		} else {
			entry.AgentRunID = nil
		}
		entry.UpdatedAt = time.Now().UTC()
	}
	if m.next != nil && m.next.EntryID == entryID {
		m.next.Status = status
		m.next.Reason = reason
		m.next.GuardrailType = guardrailType
		m.next.GuardrailScope = guardrailScope
		m.next.RecoveryDisposition = recoveryDisposition
		if runID != nil {
			id := runID.String()
			m.next.AgentRunID = &id
		} else {
			m.next.AgentRunID = nil
		}
		m.next.UpdatedAt = time.Now().UTC()
	}
	return nil
}

type mockWorktreeManager struct {
	allocation      *worktree.Allocation
	prepareErr      error
	prepareCalls    int
	releaseCalls    int
	releasedProject string
	releasedTaskID  string
}

func (m *mockWorktreeManager) Prepare(_ context.Context, projectSlug, taskID string) (*worktree.Allocation, error) {
	m.prepareCalls++
	if m.prepareErr != nil {
		return nil, m.prepareErr
	}
	if m.allocation != nil {
		return m.allocation, nil
	}
	return &worktree.Allocation{
		ProjectSlug: projectSlug,
		TaskID:      taskID,
		Branch:      "agent/" + taskID,
		Path:        "/tmp/worktree/" + taskID,
	}, nil
}

func (m *mockWorktreeManager) Release(_ context.Context, projectSlug, taskID string) error {
	m.releaseCalls++
	m.releasedProject = projectSlug
	m.releasedTaskID = taskID
	return nil
}

func (m *mockWorktreeManager) Path(_ string, taskID string) string {
	return "/tmp/worktree/" + taskID
}

func (m *mockWorktreeManager) Branch(taskID string) string {
	return "agent/" + taskID
}

func TestAgentService_SpawnCreatesStartingRun(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Wire the spawn flow",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{
		allocation: &worktree.Allocation{
			ProjectSlug: "agentforge",
			TaskID:      taskID.String(),
			Branch:      "agent/" + taskID.String(),
			Path:        "/tmp/worktree/" + taskID.String(),
			Reused:      true,
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)

	run, err := svc.Spawn(context.Background(), taskID, memberID, "", "anthropic", "claude-sonnet", 5, "")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if run.Status != model.AgentRunStatusRunning {
		t.Fatalf("status = %s, want %s", run.Status, model.AgentRunStatusRunning)
	}
	if run.TaskID != taskID {
		t.Fatalf("task id = %s, want %s", run.TaskID, taskID)
	}
	if len(repo.runsByTask[taskID]) != 1 {
		t.Fatalf("expected one run stored for task %s", taskID)
	}
	if bridge.lastExecute.WorktreePath != worktrees.allocation.Path {
		t.Fatalf("bridge worktree path = %q, want %q", bridge.lastExecute.WorktreePath, worktrees.allocation.Path)
	}
	if bridge.lastExecute.BranchName != worktrees.allocation.Branch {
		t.Fatalf("bridge branch = %q, want %q", bridge.lastExecute.BranchName, worktrees.allocation.Branch)
	}
	if bridge.lastExecute.Runtime != "claude_code" {
		t.Fatalf("bridge runtime = %q, want %q", bridge.lastExecute.Runtime, "claude_code")
	}
	if taskRepo.updatedBranch != worktrees.allocation.Branch || taskRepo.updatedWorktree != worktrees.allocation.Path {
		t.Fatalf("task runtime update = branch %q path %q, want %q %q", taskRepo.updatedBranch, taskRepo.updatedWorktree, worktrees.allocation.Branch, worktrees.allocation.Path)
	}
}

func TestAgentService_SpawnRejectsExistingActiveRun(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runsByTask[taskID] = []*model.AgentRun{
		{ID: uuid.New(), TaskID: taskID, Status: model.AgentRunStatusRunning},
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Wire the spawn flow",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)

	_, err := svc.Spawn(context.Background(), taskID, memberID, "", "anthropic", "claude-sonnet", 5, "")
	if err != service.ErrAgentAlreadyRunning {
		t.Fatalf("expected ErrAgentAlreadyRunning, got %v", err)
	}
}

func TestAgentService_SpawnMapsWorktreeGuardrailErrors(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Guardrail failure",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	worktrees := &mockWorktreeManager{prepareErr: worktree.ErrCapacityReached}
	bridge := &mockAgentBridge{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)

	_, err := svc.Spawn(context.Background(), taskID, memberID, "", "anthropic", "claude-sonnet", 5, "")
	if !errors.Is(err, service.ErrAgentWorktreeUnavailable) {
		t.Fatalf("expected ErrAgentWorktreeUnavailable, got %v", err)
	}
	if bridge.lastExecute.TaskID != "" {
		t.Fatalf("bridge Execute() should not be called on worktree denial, got %+v", bridge.lastExecute)
	}
	if taskRepo.updatedWorktree != "" || taskRepo.updatedBranch != "" {
		t.Fatalf("task runtime should not be updated on worktree denial, got branch=%q worktree=%q", taskRepo.updatedBranch, taskRepo.updatedWorktree)
	}
}

func TestAgentService_SpawnPrefersExplicitRuntime(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Use explicit runtime",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{
		allocation: &worktree.Allocation{
			ProjectSlug: "agentforge",
			TaskID:      taskID.String(),
			Branch:      "agent/" + taskID.String(),
			Path:        "/tmp/worktree/" + taskID.String(),
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)

	_, err := svc.Spawn(context.Background(), taskID, memberID, "opencode", "opencode", "opencode-default", 5, "")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	if bridge.lastExecute.Runtime != "opencode" {
		t.Fatalf("bridge runtime = %q, want opencode", bridge.lastExecute.Runtime)
	}
}

func TestAgentService_SpawnPrefersExplicitCliBackedRuntime(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn cursor agent",
		Description: "Use explicit cursor runtime",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{
		allocation: &worktree.Allocation{
			ProjectSlug: "agentforge",
			TaskID:      taskID.String(),
			Branch:      "agent/" + taskID.String(),
			Path:        "/tmp/worktree/" + taskID.String(),
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)

	run, err := svc.Spawn(
		context.Background(),
		taskID,
		memberID,
		"cursor",
		"cursor",
		"claude-sonnet-4-20250514",
		5,
		"",
	)
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if bridge.lastExecute.Runtime != "cursor" || bridge.lastExecute.Provider != "cursor" || bridge.lastExecute.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("bridge execute selection = %#v", bridge.lastExecute)
	}
	if run.Runtime != "cursor" || run.Provider != "cursor" || run.Model != "claude-sonnet-4-20250514" {
		t.Fatalf("stored run selection = %#v", run)
	}
}

func TestAgentService_SpawnResolvesProjectDefaultsAndPersistsRuntime(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Use project defaults",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{
		ID:       projectID,
		Slug:     "agentforge",
		Settings: `{"coding_agent":{"runtime":"codex","provider":"openai","model":"gpt-5-codex"}}`,
	}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{
		allocation: &worktree.Allocation{
			ProjectSlug: "agentforge",
			TaskID:      taskID.String(),
			Branch:      "agent/" + taskID.String(),
			Path:        "/tmp/worktree/" + taskID.String(),
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)

	run, err := svc.Spawn(context.Background(), taskID, memberID, "", "", "", 5, "")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if bridge.lastExecute.Runtime != "codex" || bridge.lastExecute.Provider != "openai" || bridge.lastExecute.Model != "gpt-5-codex" {
		t.Fatalf("bridge execute selection = %#v", bridge.lastExecute)
	}
	if run.Runtime != "codex" || run.Provider != "openai" || run.Model != "gpt-5-codex" {
		t.Fatalf("stored run selection = %#v", run)
	}
	stored := repo.runs[run.ID]
	if stored == nil {
		t.Fatalf("expected run %s to be stored", run.ID)
	}
	if stored.Runtime != "codex" || stored.Provider != "openai" || stored.Model != "gpt-5-codex" {
		t.Fatalf("persisted run selection = %#v", stored)
	}
}

func TestAgentService_SpawnProjectsSelectedRoleIntoBridgeRequest(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Implement dashboard role binding",
		Description: "Ensure spawn uses the selected role profile",
		BudgetUsd:   8,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{
		allocation: &worktree.Allocation{
			ProjectSlug: "agentforge",
			TaskID:      taskID.String(),
			Branch:      "agent/" + taskID.String(),
			Path:        filepath.Join("tmp", "worktree", taskID.String()),
		},
	}
	roleStore := &mockAgentRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"frontend-developer": {
				Metadata: model.RoleMetadata{
					ID:   "frontend-developer",
					Name: "Frontend Developer",
				},
				Identity: model.RoleIdentity{
					Role:      "Senior Frontend Developer",
					Goal:      "Deliver polished frontend work",
					Backstory: "You specialize in React and UX detail.",
				},
				SystemPrompt: "Always preserve the established UI language.",
				Capabilities: model.RoleCapabilities{
					AllowedTools: []string{"Read", "Edit", "Write"},
					MaxTurns:     18,
					ToolConfig: model.RoleToolConfig{
						External: []string{"github-tool", "web-search"},
						PluginBindings: []model.RoleToolPluginBinding{
							{PluginID: "github-tool", Functions: []string{"search_prs"}},
						},
					},
				},
				Knowledge: model.RoleKnowledge{
					Documents: []string{"docs/PRD.md"},
					Shared: []model.RoleKnowledgeSource{
						{ID: "design-guidelines"},
					},
				},
				Security: model.RoleSecurity{
					MaxBudgetUsd:   3.5,
					PermissionMode: "bypassPermissions",
					OutputFilters:  []string{"no_credentials", "no_pii"},
				},
			},
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, roleStore)

	run, err := svc.Spawn(context.Background(), taskID, memberID, "", "anthropic", "claude-sonnet", 0, "frontend-developer")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if run.RoleID != "frontend-developer" {
		t.Fatalf("run.RoleID = %q, want frontend-developer", run.RoleID)
	}
	if bridge.lastExecute.RoleConfig == nil {
		t.Fatal("expected normalized role config to be forwarded to bridge")
	}
	if bridge.lastExecute.RoleConfig.RoleID != "frontend-developer" {
		t.Fatalf("bridge role id = %q, want frontend-developer", bridge.lastExecute.RoleConfig.RoleID)
	}
	if bridge.lastExecute.RoleConfig.Role != "Senior Frontend Developer" {
		t.Fatalf("bridge role title = %q, want Senior Frontend Developer", bridge.lastExecute.RoleConfig.Role)
	}
	if bridge.lastExecute.MaxTurns != 18 {
		t.Fatalf("bridge max turns = %d, want 18", bridge.lastExecute.MaxTurns)
	}
	if bridge.lastExecute.PermissionMode != "bypassPermissions" {
		t.Fatalf("bridge permission mode = %q, want bypassPermissions", bridge.lastExecute.PermissionMode)
	}
	if bridge.lastExecute.BudgetUSD != 3.5 {
		t.Fatalf("bridge budget = %v, want 3.5", bridge.lastExecute.BudgetUSD)
	}
	if len(bridge.lastExecute.AllowedTools) != 3 {
		t.Fatalf("bridge allowed tools len = %d, want 3", len(bridge.lastExecute.AllowedTools))
	}
	assertBridgeRoleConfigStringSlice(t, bridge.lastExecute.RoleConfig, "Tools", []string{"github-tool", "web-search"})
	bindingsField := reflect.ValueOf(bridge.lastExecute.RoleConfig).Elem().FieldByName("PluginBindings")
	if !bindingsField.IsValid() {
		t.Fatal("bridge role config missing PluginBindings field")
	}
	if bindingsField.Len() != 1 {
		t.Fatalf("bridge role config plugin bindings len = %d, want 1", bindingsField.Len())
	}
	assertBridgeRoleConfigStringSlice(t, bridge.lastExecute.RoleConfig, "OutputFilters", []string{"no_credentials", "no_pii"})
	knowledgeContext := assertBridgeRoleConfigStringField(t, bridge.lastExecute.RoleConfig, "KnowledgeContext")
	if !strings.Contains(knowledgeContext, "docs/PRD.md") {
		t.Fatalf("KnowledgeContext = %q, want docs/PRD.md reference", knowledgeContext)
	}
}

func TestAgentService_SpawnRejectsUnknownRole(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Resolve role before execution",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, &mockWorktreeManager{}, &mockAgentRoleStore{})

	_, err := svc.Spawn(context.Background(), taskID, memberID, "", "anthropic", "claude-sonnet", 0, "missing-role")
	if !errors.Is(err, service.ErrAgentRoleNotFound) {
		t.Fatalf("expected ErrAgentRoleNotFound, got %v", err)
	}
	if bridge.lastExecute.TaskID != "" {
		t.Fatalf("bridge Execute() should not be called when role lookup fails, got %+v", bridge.lastExecute)
	}
	if len(repo.runsByTask[taskID]) != 0 {
		t.Fatalf("expected no stored runs when role lookup fails, got %d", len(repo.runsByTask[taskID]))
	}
}

func TestAgentService_SpawnRejectsBlockingAutoLoadSkillProjection(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Resolve runtime skills before execution",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{}
	skillsDir := t.TempDir()
	roleStore := &mockAgentRoleStore{
		skillsDir: skillsDir,
		roles: map[string]*rolepkg.Manifest{
			"frontend-developer": {
				Metadata: model.RoleMetadata{ID: "frontend-developer", Name: "Frontend Developer"},
				Identity: model.RoleIdentity{
					Role:      "Senior Frontend Developer",
					Goal:      "Deliver polished frontend work",
					Backstory: "You specialize in React and UX detail.",
				},
				SystemPrompt: "Always preserve the established UI language.",
				Capabilities: model.RoleCapabilities{
					AllowedTools: []string{"Read", "Edit", "Write"},
					MaxTurns:     18,
					Skills: []model.RoleSkillReference{
						{Path: "skills/react", AutoLoad: true},
					},
				},
				Security: model.RoleSecurity{
					MaxBudgetUsd:   3.5,
					PermissionMode: "bypassPermissions",
				},
			},
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, roleStore)

	_, err := svc.Spawn(context.Background(), taskID, memberID, "", "anthropic", "claude-sonnet", 0, "frontend-developer")
	if err == nil || !strings.Contains(err.Error(), "skills/react") {
		t.Fatalf("Spawn() error = %v, want blocking skill projection failure mentioning skills/react", err)
	}
	if bridge.lastExecute.TaskID != "" {
		t.Fatalf("bridge Execute() should not be called when auto-load skill projection blocks, got %+v", bridge.lastExecute)
	}
}

func TestAgentService_SpawnRejectsBlockingPluginDependency(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Resolve plugin dependencies before execution",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{}
	roleStore := &mockAgentRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"design-lead": {
				Metadata: model.RoleMetadata{ID: "design-lead", Name: "Design Lead"},
				Identity: model.RoleIdentity{
					Role: "Design Lead",
					Goal: "Review UX",
				},
				SystemPrompt: "Review design consistency.",
				Capabilities: model.RoleCapabilities{
					AllowedTools: []string{"Read"},
					ToolConfig: model.RoleToolConfig{
						External: []string{"design-mcp"},
					},
				},
				Security: model.RoleSecurity{
					MaxBudgetUsd:   2,
					PermissionMode: "default",
				},
			},
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, roleStore)
	svc.SetPluginCatalog(&mockAgentPluginCatalog{})

	_, err := svc.Spawn(context.Background(), taskID, memberID, "", "anthropic", "claude-sonnet", 0, "design-lead")
	if err == nil || !strings.Contains(err.Error(), "design-mcp") {
		t.Fatalf("Spawn() error = %v, want blocking plugin dependency failure mentioning design-mcp", err)
	}
	if bridge.lastExecute.TaskID != "" {
		t.Fatalf("bridge Execute() should not be called when plugin dependency blocks, got %+v", bridge.lastExecute)
	}
}

func TestAgentService_SpawnAllowsBundledToolPluginDependencies(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Spawn agent",
		Description: "Resolve bundled tool dependencies before execution",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{
		allocation: &worktree.Allocation{
			ProjectSlug: "agentforge",
			TaskID:      taskID.String(),
			Branch:      "agent/" + taskID.String(),
			Path:        filepath.Join("tmp", "worktree", taskID.String()),
		},
	}
	roleStore := &mockAgentRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"frontend-developer": {
				Metadata: model.RoleMetadata{
					ID:   "frontend-developer",
					Name: "Frontend Developer",
				},
				Identity: model.RoleIdentity{
					Role:      "Senior Frontend Developer",
					Goal:      "Deliver polished frontend work",
					Backstory: "You specialize in React and UX detail.",
				},
				SystemPrompt: "Always preserve the established UI language.",
				Capabilities: model.RoleCapabilities{
					AllowedTools: []string{"Read", "Edit", "Write"},
					MaxTurns:     18,
					ToolConfig: model.RoleToolConfig{
						External: []string{"github-tool", "web-search"},
					},
				},
				Security: model.RoleSecurity{
					MaxBudgetUsd:   3.5,
					PermissionMode: "bypassPermissions",
				},
			},
		},
	}
	pluginsDir := t.TempDir()
	writeManifest(t, pluginsDir, "tools/github-tool/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: github-tool
  name: GitHub Tool
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["github.js"]
`)
	writeManifest(t, pluginsDir, "tools/web-search/manifest.yaml", `
apiVersion: agentforge/v1
kind: ToolPlugin
metadata:
  id: web-search
  name: Web Search
  version: 1.0.0
spec:
  runtime: mcp
  transport: stdio
  command: node
  args: ["search.js"]
`)
	writeBuiltInBundle(t, pluginsDir, `
plugins:
  - id: github-tool
    kind: ToolPlugin
    manifest: tools/github-tool/manifest.yaml
    docsRef: docs/PRD.md#tool-plugin
    verificationProfile: mcp-tool
    availability:
      status: ready
      message: GitHub tool ships with AgentForge.
  - id: web-search
    kind: ToolPlugin
    manifest: tools/web-search/manifest.yaml
    docsRef: docs/PRD.md#tool-plugin
    verificationProfile: mcp-tool
    availability:
      status: ready
      message: Web search ships with AgentForge.
`)

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, roleStore)
	svc.SetPluginCatalog(service.NewPluginService(repository.NewPluginRegistryRepository(), &fakePluginRuntimeClient{}, &fakeGoPluginRuntime{}, pluginsDir))

	run, err := svc.Spawn(context.Background(), taskID, memberID, "", "anthropic", "claude-sonnet", 0, "frontend-developer")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	if run.RoleID != "frontend-developer" {
		t.Fatalf("run.RoleID = %q, want frontend-developer", run.RoleID)
	}
	if bridge.lastExecute.RoleConfig == nil {
		t.Fatal("expected role config to be forwarded when bundled tool dependencies are available")
	}
	assertBridgeRoleConfigStringSlice(t, bridge.lastExecute.RoleConfig, "Tools", []string{"github-tool", "web-search"})
}

func TestAgentService_CancelReleasesCanonicalManagedWorktree(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:             taskID,
		ProjectID:      projectID,
		Title:          "Spawn agent",
		Description:    "Cancel cleanup",
		BudgetUsd:      5,
		AgentBranch:    "agent/" + taskID.String(),
		AgentWorktree:  "/tmp/worktree/" + taskID.String(),
		AgentSessionID: "session-1",
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)

	if err := svc.Cancel(context.Background(), runID, "user_cancelled"); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if worktrees.releaseCalls != 1 {
		t.Fatalf("Release() calls = %d, want 1", worktrees.releaseCalls)
	}
	if worktrees.releasedProject != "agentforge" || worktrees.releasedTaskID != taskID.String() {
		t.Fatalf("Release() target = %s/%s, want %s/%s", worktrees.releasedProject, worktrees.releasedTaskID, "agentforge", taskID.String())
	}
	if taskRepo.clearCalls != 1 {
		t.Fatalf("ClearRuntime() calls = %d, want 1", taskRepo.clearCalls)
	}
	if bridge.cancelTaskID != taskID.String() || bridge.cancelReason != "user_cancelled" {
		t.Fatalf("bridge Cancel() got %s/%s, want %s/%s", bridge.cancelTaskID, bridge.cancelReason, taskID.String(), "user_cancelled")
	}
}

func TestAgentService_PauseAndResumePreserveManagedRuntimeContext(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	teamID := uuid.New()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		RoleID:   "frontend-developer",
		Status:   model.AgentRunStatusRunning,
		Provider: "codex",
		Model:    "gpt-5-codex",
		TeamID:   &teamID,
		TeamRole: model.TeamRoleCoder,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:             taskID,
		ProjectID:      projectID,
		Title:          "Pause resume runtime",
		Description:    "Carry session and worktree metadata through lifecycle changes",
		BudgetUsd:      5,
		AgentBranch:    "agent/" + taskID.String(),
		AgentWorktree:  "/tmp/worktree/" + taskID.String(),
		AgentSessionID: taskID.String() + "-session",
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusPaused); err != nil {
		t.Fatalf("pause UpdateStatus() error = %v", err)
	}
	if bridge.pauseTaskID != taskID.String() || bridge.pauseReason != "paused_by_user" {
		t.Fatalf("bridge Pause() got %s/%s, want %s/%s", bridge.pauseTaskID, bridge.pauseReason, taskID.String(), "paused_by_user")
	}
	if status := repo.runs[runID].Status; status != model.AgentRunStatusPaused {
		t.Fatalf("status after pause = %s, want %s", status, model.AgentRunStatusPaused)
	}
	if worktrees.releaseCalls != 0 {
		t.Fatalf("pause should not release worktree, got %d calls", worktrees.releaseCalls)
	}

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusRunning); err != nil {
		t.Fatalf("resume UpdateStatus() error = %v", err)
	}
	if bridge.resumeReq.TaskID != taskID.String() {
		t.Fatalf("resume task id = %s, want %s", bridge.resumeReq.TaskID, taskID.String())
	}
	if bridge.resumeReq.SessionID != taskRepo.task.AgentSessionID {
		t.Fatalf("resume session = %s, want %s", bridge.resumeReq.SessionID, taskRepo.task.AgentSessionID)
	}
	if bridge.resumeReq.WorktreePath != taskRepo.task.AgentWorktree || bridge.resumeReq.BranchName != taskRepo.task.AgentBranch {
		t.Fatalf("resume worktree/branch = %s/%s, want %s/%s", bridge.resumeReq.WorktreePath, bridge.resumeReq.BranchName, taskRepo.task.AgentWorktree, taskRepo.task.AgentBranch)
	}
	if bridge.resumeReq.Runtime != "codex" {
		t.Fatalf("resume runtime = %s, want codex", bridge.resumeReq.Runtime)
	}
	assertBridgeExecuteStringField(t, bridge.resumeReq, "TeamID", teamID.String())
	assertBridgeExecuteStringField(t, bridge.resumeReq, "TeamRole", model.TeamRoleCoder)
	if status := repo.runs[runID].Status; status != model.AgentRunStatusRunning {
		t.Fatalf("status after resume = %s, want %s", status, model.AgentRunStatusRunning)
	}
}

func TestAgentService_ResumeUsesPersistedRuntimeIdentity(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusPaused,
		Runtime:  "codex",
		Provider: "openai",
		Model:    "gpt-5-codex",
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:             taskID,
		ProjectID:      projectID,
		Title:          "Resume runtime",
		Description:    "Reuse persisted runtime identity",
		BudgetUsd:      5,
		AgentBranch:    "agent/" + taskID.String(),
		AgentWorktree:  "/tmp/worktree/" + taskID.String(),
		AgentSessionID: taskID.String() + "-session",
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, &mockWorktreeManager{}, nil)

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusRunning); err != nil {
		t.Fatalf("resume UpdateStatus() error = %v", err)
	}

	if bridge.resumeReq.Runtime != "codex" || bridge.resumeReq.Provider != "openai" || bridge.resumeReq.Model != "gpt-5-codex" {
		t.Fatalf("resume request selection = %#v", bridge.resumeReq)
	}
}

func TestDiffBridgeRuntimeContextDetectsMismatch(t *testing.T) {
	expected := service.BridgeRuntimeContextSnapshot{
		Runtime:  "codex",
		Provider: "openai",
		Model:    "gpt-5-codex",
		TeamID:   "team-123",
		TeamRole: model.TeamRoleReviewer,
	}
	actual := service.BridgeRuntimeContextSnapshot{
		Runtime:  "opencode",
		Provider: "openai",
		Model:    "gpt-5-codex",
		TeamID:   "team-123",
		TeamRole: model.TeamRoleReviewer,
	}

	field, expectedValue, actualValue, ok := service.DiffBridgeRuntimeContext(expected, actual)
	if !ok {
		t.Fatal("expected context mismatch to be detected")
	}
	if field != "runtime" || expectedValue != "codex" || actualValue != "opencode" {
		t.Fatalf("mismatch = %s %s %s", field, expectedValue, actualValue)
	}
}

func TestDiffBridgeRuntimeContextTreatsMatchingValuesAsStable(t *testing.T) {
	expected := service.BridgeRuntimeContextSnapshot{
		Runtime:  "codex",
		Provider: "openai",
		Model:    "gpt-5-codex",
	}
	actual := service.BridgeRuntimeContextSnapshot{
		Runtime:  "codex",
		Provider: "openai",
		Model:    "gpt-5-codex",
	}

	if field, expectedValue, actualValue, ok := service.DiffBridgeRuntimeContext(expected, actual); ok {
		t.Fatalf("unexpected mismatch = %s %s %s", field, expectedValue, actualValue)
	}
}

func TestAgentService_SpawnForTeamIncludesTeamExecutionContext(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	teamID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Team-aware spawn",
		Description: "Ensure bridge execute receives team context before startup",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{
		allocation: &worktree.Allocation{
			ProjectSlug: "agentforge",
			TaskID:      taskID.String(),
			Branch:      "agent/" + taskID.String(),
			Path:        "/tmp/worktree/" + taskID.String(),
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)

	run, err := svc.SpawnForTeam(context.Background(), teamID, model.TeamRolePlanner, taskID, memberID, "codex", "openai", "gpt-5-codex", 5, "")
	if err != nil {
		t.Fatalf("SpawnForTeam() error = %v", err)
	}

	if run.TeamID == nil || *run.TeamID != teamID {
		t.Fatalf("run.TeamID = %v, want %s", run.TeamID, teamID)
	}
	if run.TeamRole != model.TeamRolePlanner {
		t.Fatalf("run.TeamRole = %q, want %q", run.TeamRole, model.TeamRolePlanner)
	}
	assertBridgeExecuteStringField(t, bridge.lastExecute, "TeamID", teamID.String())
	assertBridgeExecuteStringField(t, bridge.lastExecute, "TeamRole", model.TeamRolePlanner)
}

func TestAgentService_ProcessBridgeEvent_UpdatesCostFromRuntimeEvent(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Realtime runtime cost",
		BudgetUsd: 5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)

	err := svc.ProcessBridgeEvent(context.Background(), &ws.BridgeAgentEvent{
		TaskID:      taskID.String(),
		SessionID:   "session-1",
		TimestampMS: 123,
		Type:        ws.BridgeEventCostUpdate,
		Data:        []byte(`{"input_tokens":120,"output_tokens":45,"cache_read_tokens":5,"cost_usd":0.37,"budget_remaining_usd":4.63,"turn_number":3}`),
	})
	if err != nil {
		t.Fatalf("ProcessBridgeEvent() error = %v", err)
	}

	run := repo.runs[runID]
	if run.InputTokens != 120 || run.OutputTokens != 45 || run.CacheReadTokens != 5 {
		t.Fatalf("run token totals = %+v", run)
	}
	if run.CostUsd != 0.37 {
		t.Fatalf("run.CostUsd = %v, want 0.37", run.CostUsd)
	}
	if run.TurnCount != 3 {
		t.Fatalf("run.TurnCount = %d, want 3", run.TurnCount)
	}
}

func TestAgentService_ProcessBridgeEvent_CompletesRunFromTerminalStatusChange(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:             taskID,
		ProjectID:      projectID,
		Title:          "Finalize runtime lifecycle",
		BudgetUsd:      5,
		AgentBranch:    "agent/" + taskID.String(),
		AgentWorktree:  "/tmp/worktree/" + taskID.String(),
		AgentSessionID: "session-1",
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	worktrees := &mockWorktreeManager{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, worktrees, nil)

	err := svc.ProcessBridgeEvent(context.Background(), &ws.BridgeAgentEvent{
		TaskID:      taskID.String(),
		SessionID:   "session-1",
		TimestampMS: 456,
		Type:        ws.BridgeEventStatusChange,
		Data:        []byte(`{"old_status":"running","new_status":"completed","reason":"end_turn"}`),
	})
	if err != nil {
		t.Fatalf("ProcessBridgeEvent() error = %v", err)
	}

	if status := repo.runs[runID].Status; status != model.AgentRunStatusCompleted {
		t.Fatalf("run status = %s, want %s", status, model.AgentRunStatusCompleted)
	}
	if worktrees.releaseCalls != 1 {
		t.Fatalf("Release() calls = %d, want 1", worktrees.releaseCalls)
	}
	if taskRepo.clearCalls != 1 {
		t.Fatalf("ClearRuntime() calls = %d, want 1", taskRepo.clearCalls)
	}
}

func TestAgentService_ProcessBridgeEvent_PermissionRequestQueuesIMNotification(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Permission request",
		BudgetUsd: 5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	imNotifier := &mockAgentIMProgressNotifier{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetIMProgressNotifier(imNotifier)

	err := svc.ProcessBridgeEvent(context.Background(), &ws.BridgeAgentEvent{
		TaskID:      taskID.String(),
		SessionID:   "session-1",
		TimestampMS: 789,
		Type:        ws.BridgeEventPermissionRequest,
		Data:        []byte(`{"request_id":"req-123","tool_name":"Read","elicitation_type":"tool_permission","context":{"path":"README.md"}}`),
	})
	if err != nil {
		t.Fatalf("ProcessBridgeEvent() error = %v", err)
	}

	if len(imNotifier.requests) != 1 {
		t.Fatalf("im notifier requests = %d, want 1", len(imNotifier.requests))
	}
	req := imNotifier.requests[0]
	if req.TaskID != taskID.String() || req.RunID != runID.String() {
		t.Fatalf("request ids = %+v", req)
	}
	if req.Kind != service.IMDeliveryKindProgress {
		t.Fatalf("Kind = %q, want %q", req.Kind, service.IMDeliveryKindProgress)
	}
	if !strings.Contains(req.Content, "Read") || !strings.Contains(req.Content, "req-123") {
		t.Fatalf("Content = %q", req.Content)
	}
	if req.Structured == nil || !strings.Contains(req.Structured.Title, "Permission") {
		t.Fatalf("Structured = %+v", req.Structured)
	}
}

func TestAgentService_ProcessBridgeEvent_BudgetAlertQueuesBoundIMNotification(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Budget alert",
		BudgetUsd: 5,
		SpentUsd:  4.2,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	imNotifier := &mockAgentIMProgressNotifier{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetIMProgressNotifier(imNotifier)

	err := svc.ProcessBridgeEvent(context.Background(), &ws.BridgeAgentEvent{
		TaskID:      taskID.String(),
		SessionID:   "session-1",
		TimestampMS: 790,
		Type:        ws.BridgeEventBudgetAlert,
		Data:        []byte(`{"cost_usd":4.2,"budget_remaining_usd":0.8,"threshold_ratio":0.8,"threshold_percent":80,"turn_number":2}`),
	})
	if err != nil {
		t.Fatalf("ProcessBridgeEvent() error = %v", err)
	}

	if len(imNotifier.requests) != 1 {
		t.Fatalf("im notifier requests = %d, want 1", len(imNotifier.requests))
	}
	req := imNotifier.requests[0]
	if req.TaskID != taskID.String() || req.RunID != runID.String() {
		t.Fatalf("request ids = %+v", req)
	}
	if req.Metadata["bridge_event_type"] != ws.EventBudgetWarning {
		t.Fatalf("metadata = %+v", req.Metadata)
	}
	if req.Structured == nil || !strings.Contains(req.Structured.Title, "Budget") {
		t.Fatalf("structured = %+v", req.Structured)
	}
}

func TestAgentService_ProcessBridgeEvent_BudgetAlertFallsBackToConfiguredChannels(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Budget alert",
		BudgetUsd: 5,
		SpentUsd:  4.2,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	control := service.NewIMControlPlane(service.IMControlPlaneConfig{})
	if _, err := control.UpsertChannel(context.Background(), &model.IMChannel{
		Platform:  "slack",
		Name:      "Budget Alerts",
		ChannelID: "C-budget",
		Events:    []string{ws.EventBudgetWarning},
		Active:    true,
	}); err != nil {
		t.Fatalf("UpsertChannel() error = %v", err)
	}
	imNotifier := &mockAgentIMNotifier{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetIMProgressNotifier(control)
	svc.SetIMNotifier(imNotifier)
	svc.SetIMChannelResolver(control)

	err := svc.ProcessBridgeEvent(context.Background(), &ws.BridgeAgentEvent{
		TaskID:      taskID.String(),
		SessionID:   "session-1",
		TimestampMS: 791,
		Type:        ws.BridgeEventBudgetAlert,
		Data:        []byte(`{"cost_usd":4.2,"budget_remaining_usd":0.8,"threshold_ratio":0.8,"threshold_percent":80,"turn_number":2}`),
	})
	if err != nil {
		t.Fatalf("ProcessBridgeEvent() error = %v", err)
	}

	if len(imNotifier.requests) != 1 {
		t.Fatalf("notify requests = %+v", imNotifier.requests)
	}
	if imNotifier.requests[0].Platform != "slack" || imNotifier.requests[0].ChannelID != "C-budget" {
		t.Fatalf("notify request = %+v", imNotifier.requests[0])
	}
	if imNotifier.requests[0].Event != ws.EventBudgetWarning {
		t.Fatalf("event = %q", imNotifier.requests[0].Event)
	}
}

func TestAgentService_UpdateCost_SyncsTaskSpendAndBroadcastsBudgetWarning(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:              runID,
		TaskID:          taskID,
		MemberID:        memberID,
		Status:          model.AgentRunStatusRunning,
		InputTokens:     200,
		OutputTokens:    50,
		CacheReadTokens: 0,
		CostUsd:         3.5,
		TurnCount:       2,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:             taskID,
		ProjectID:      projectID,
		Title:          "Budget warning",
		Status:         model.TaskStatusInProgress,
		BudgetUsd:      5,
		SpentUsd:       3.5,
		AgentBranch:    "agent/" + taskID.String(),
		AgentWorktree:  "/tmp/worktree/" + taskID.String(),
		AgentSessionID: "session-1",
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	hub := ws.NewHub()
	stop, events := subscribeProjectEvents(t, hub, projectID.String())
	defer stop()

	svc := service.NewAgentService(repo, taskRepo, projectRepo, hub, &mockAgentBridge{}, &mockWorktreeManager{}, nil)

	if err := svc.UpdateCost(context.Background(), runID, 260, 80, 10, 4.2, 3, nil); err != nil {
		t.Fatalf("UpdateCost() error = %v", err)
	}

	if taskRepo.task.SpentUsd != 4.2 {
		t.Fatalf("task spent = %v, want 4.2", taskRepo.task.SpentUsd)
	}
	if taskRepo.task.Status != model.TaskStatusInProgress {
		t.Fatalf("task status = %s, want %s", taskRepo.task.Status, model.TaskStatusInProgress)
	}

	warning := waitForEventType(t, events, ws.EventBudgetWarning)
	if warning.Type != ws.EventBudgetWarning {
		t.Fatalf("warning event type = %s", warning.Type)
	}

	var payload map[string]any
	if err := json.Unmarshal(warning.Payload, &payload); err != nil {
		t.Fatalf("unmarshal warning payload: %v", err)
	}
	if payload["taskId"] != taskID.String() {
		t.Fatalf("warning taskId = %v, want %s", payload["taskId"], taskID)
	}
}

func TestAgentService_ProcessBridgeCostEvent_PersistsAccountingMetadata(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Persist accounting metadata",
		Status:    model.TaskStatusInProgress,
		BudgetUsd: 10,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)

	err := svc.ProcessBridgeEvent(context.Background(), &ws.BridgeAgentEvent{
		TaskID:      taskID.String(),
		SessionID:   "session-cost-accounting",
		TimestampMS: 1_700_000_000_000,
		Type:        ws.BridgeEventCostUpdate,
		Data:        []byte(`{"input_tokens":120,"output_tokens":45,"cache_read_tokens":5,"cache_creation_tokens":12,"cost_usd":0.37,"budget_remaining_usd":9.63,"turn_number":3,"cost_accounting":{"total_cost_usd":0.37,"input_tokens":120,"output_tokens":45,"cache_read_tokens":5,"cache_creation_tokens":12,"mode":"estimated_api_pricing","coverage":"full","source":"openai_api_pricing","components":[]}}`),
	})
	if err != nil {
		t.Fatalf("ProcessBridgeEvent() error = %v", err)
	}

	if repo.runs[runID].CostAccounting == nil {
		t.Fatalf("run cost accounting = nil")
	}
	if repo.runs[runID].CostAccounting.Mode != "estimated_api_pricing" || repo.runs[runID].CostAccounting.CacheCreationTokens != 12 {
		t.Fatalf("run cost accounting = %#v", repo.runs[runID].CostAccounting)
	}
}

func TestAgentService_UpdateCost_BudgetWarningQueuesIMNotification(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:              runID,
		TaskID:          taskID,
		MemberID:        memberID,
		Status:          model.AgentRunStatusRunning,
		InputTokens:     200,
		OutputTokens:    50,
		CacheReadTokens: 0,
		CostUsd:         3.5,
		TurnCount:       2,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Budget warning notify",
		Status:    model.TaskStatusInProgress,
		BudgetUsd: 5,
		SpentUsd:  3.5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	imNotifier := &mockAgentIMProgressNotifier{}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetIMProgressNotifier(imNotifier)

	if err := svc.UpdateCost(context.Background(), runID, 260, 80, 10, 4.2, 3, nil); err != nil {
		t.Fatalf("UpdateCost() error = %v", err)
	}

	if len(imNotifier.requests) != 1 {
		t.Fatalf("im notifier requests = %d, want 1", len(imNotifier.requests))
	}
	req := imNotifier.requests[0]
	if req.TaskID != taskID.String() || req.RunID != runID.String() {
		t.Fatalf("request ids = %+v", req)
	}
	if !strings.Contains(req.Content, "80") || !strings.Contains(req.Content, "4.20") {
		t.Fatalf("Content = %q", req.Content)
	}
	if req.Structured == nil || !strings.Contains(req.Structured.Title, "Budget") {
		t.Fatalf("Structured = %+v", req.Structured)
	}
}

func TestBridgeWS_PreservesEventOrderingIntoIMDeliveries(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   taskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:             taskID,
		ProjectID:      projectID,
		Title:          "Bridge event flow",
		Status:         model.TaskStatusInProgress,
		BudgetUsd:      5,
		SpentUsd:       3.5,
		AgentBranch:    "agent/" + taskID.String(),
		AgentWorktree:  "/tmp/worktree/" + taskID.String(),
		AgentSessionID: "session-1",
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}

	control := service.NewIMControlPlane(service.IMControlPlaneConfig{
		HeartbeatTTL:              time.Minute,
		ProgressHeartbeatInterval: 30 * time.Second,
		DeliverySecret:            "shared-secret",
	})
	if _, err := control.RegisterBridge(context.Background(), &service.IMBridgeRegisterRequest{
		BridgeID:   "bridge-slack-1",
		Platform:   "slack",
		Transport:  "live",
		ProjectIDs: []string{projectID.String()},
	}); err != nil {
		t.Fatalf("RegisterBridge error: %v", err)
	}
	listener := &bridgeIMDeliveryListener{}
	if _, err := control.AttachBridgeListener(context.Background(), "bridge-slack-1", 0, listener); err != nil {
		t.Fatalf("AttachBridgeListener error: %v", err)
	}
	if err := control.BindAction(context.Background(), &service.IMActionBinding{
		BridgeID:  "bridge-slack-1",
		Platform:  "slack",
		ProjectID: projectID.String(),
		TaskID:    taskID.String(),
		RunID:     runID.String(),
		ReplyTarget: &model.IMReplyTarget{
			Platform:  "slack",
			ChannelID: "C123",
			ThreadID:  "1700000000.1",
		},
	}); err != nil {
		t.Fatalf("BindAction error: %v", err)
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetIMProgressNotifier(control)

	e := echo.New()
	e.GET("/ws/bridge", ws.NewBridgeHandler(svc).HandleWS)
	srv := httptest.NewServer(e)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/bridge"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial bridge websocket: %v", err)
	}
	defer conn.Close()

	if err := conn.WriteJSON(map[string]any{
		"task_id":      taskID.String(),
		"session_id":   "session-1",
		"timestamp_ms": 100,
		"type":         "permission_request",
		"data": map[string]any{
			"request_id": "req-1",
			"tool_name":  "Read",
		},
	}); err != nil {
		t.Fatalf("write permission_request event: %v", err)
	}
	if err := conn.WriteJSON(map[string]any{
		"task_id":      taskID.String(),
		"session_id":   "session-1",
		"timestamp_ms": 200,
		"type":         "status_change",
		"data": map[string]any{
			"old_status": "running",
			"new_status": "completed",
			"reason":     "end_turn",
		},
	}); err != nil {
		t.Fatalf("write status_change event: %v", err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for len(listener.deliveries) < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if len(listener.deliveries) != 2 {
		t.Fatalf("listener deliveries = %d, want 2", len(listener.deliveries))
	}
	if listener.deliveries[0].Structured == nil || !strings.Contains(listener.deliveries[0].Structured.Title, "Permission") {
		t.Fatalf("first delivery = %+v", listener.deliveries[0])
	}
	if listener.deliveries[1].Kind != service.IMDeliveryKindTerminal {
		t.Fatalf("second delivery kind = %q, want %q", listener.deliveries[1].Kind, service.IMDeliveryKindTerminal)
	}
	if !strings.Contains(listener.deliveries[1].Content, "运行完成") {
		t.Fatalf("second delivery content = %q", listener.deliveries[1].Content)
	}
	if listener.deliveries[0].Cursor >= listener.deliveries[1].Cursor {
		t.Fatalf("delivery cursors out of order: first=%d second=%d", listener.deliveries[0].Cursor, listener.deliveries[1].Cursor)
	}
}

func TestAgentService_UpdateCost_BudgetExceededCancelsRunAndKeepsRuntimeForResume(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:        runID,
		TaskID:    taskID,
		MemberID:  memberID,
		Status:    model.AgentRunStatusRunning,
		Provider:  "codex",
		Model:     "gpt-5.4",
		CostUsd:   4.8,
		TurnCount: 4,
	}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:             taskID,
		ProjectID:      projectID,
		Title:          "Budget exceeded",
		Status:         model.TaskStatusInProgress,
		BudgetUsd:      5,
		SpentUsd:       4.8,
		AgentBranch:    "agent/" + taskID.String(),
		AgentWorktree:  "/tmp/worktree/" + taskID.String(),
		AgentSessionID: "session-keep",
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{}
	hub := ws.NewHub()
	stop, events := subscribeProjectEvents(t, hub, projectID.String())
	defer stop()

	svc := service.NewAgentService(repo, taskRepo, projectRepo, hub, bridge, worktrees, nil)

	if err := svc.UpdateCost(context.Background(), runID, 320, 140, 10, 5.3, 5, nil); err != nil {
		t.Fatalf("UpdateCost() error = %v", err)
	}

	if bridge.cancelTaskID != taskID.String() || bridge.cancelReason != "budget_exceeded" {
		t.Fatalf("bridge cancel = %s/%s, want %s/budget_exceeded", bridge.cancelTaskID, bridge.cancelReason, taskID)
	}
	if repo.runs[runID].Status != model.AgentRunStatusBudgetExceeded {
		t.Fatalf("run status = %s, want %s", repo.runs[runID].Status, model.AgentRunStatusBudgetExceeded)
	}
	if taskRepo.task.Status != model.TaskStatusBudgetExceeded {
		t.Fatalf("task status = %s, want %s", taskRepo.task.Status, model.TaskStatusBudgetExceeded)
	}
	if taskRepo.clearCalls != 0 {
		t.Fatalf("ClearRuntime() calls = %d, want 0", taskRepo.clearCalls)
	}
	if worktrees.releaseCalls != 0 {
		t.Fatalf("Release() calls = %d, want 0", worktrees.releaseCalls)
	}

	exceeded := waitForEventType(t, events, ws.EventBudgetExceeded)
	if exceeded.Type != ws.EventBudgetExceeded {
		t.Fatalf("exceeded event type = %s", exceeded.Type)
	}

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusRunning); err != nil {
		t.Fatalf("resume from budget_exceeded error = %v", err)
	}
	if bridge.resumeReq.TaskID != taskID.String() {
		t.Fatalf("resume task id = %s, want %s", bridge.resumeReq.TaskID, taskID.String())
	}
	if bridge.resumeReq.SessionID != taskRepo.task.AgentSessionID {
		t.Fatalf("resume session = %s, want %s", bridge.resumeReq.SessionID, taskRepo.task.AgentSessionID)
	}
}

func TestAgentService_UpdateCostEmitsAutomationBudgetEvent(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	runID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{ID: runID, TaskID: taskID, MemberID: memberID, Status: model.AgentRunStatusRunning, CostUsd: 3.5}
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:        taskID,
		ProjectID: projectID,
		Title:     "Automation budget",
		Status:    model.TaskStatusInProgress,
		BudgetUsd: 5,
		SpentUsd:  3.5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	automation := &automationEventProbe{}
	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetAutomationEvaluator(automation)

	if err := svc.UpdateCost(context.Background(), runID, 260, 80, 10, 4.2, 3, nil); err != nil {
		t.Fatalf("UpdateCost() error = %v", err)
	}
	if len(automation.events) != 1 || automation.events[0].EventType != model.AutomationEventBudgetThresholdReached {
		t.Fatalf("automation events = %+v", automation.events)
	}
}

func TestAgentService_PoolStatsIncludesQueuedEntries(t *testing.T) {
	projectID := uuid.New()
	queueStore := &mockAgentQueueStore{
		queued: []*model.AgentPoolQueueEntry{
			{
				EntryID:   uuid.NewString(),
				ProjectID: projectID.String(),
				TaskID:    uuid.NewString(),
				MemberID:  uuid.NewString(),
				Status:    model.AgentPoolQueueStatusQueued,
			},
		},
	}

	svc := service.NewAgentService(newMockAgentRunRepo(), &mockAgentTaskRepo{}, &mockAgentProjectRepo{}, ws.NewHub(), &mockAgentBridge{
		poolSummary: &bridgeclient.PoolSummaryResponse{
			Active:        1,
			Max:           2,
			WarmTotal:     1,
			WarmAvailable: 0,
		},
	}, &mockWorktreeManager{}, nil)
	svc.SetPool(pool.NewPool(2))
	svc.SetQueueStore(queueStore)

	stats := svc.PoolStats(context.Background())
	if stats.Queued != 1 {
		t.Fatalf("queued = %d, want 1", stats.Queued)
	}
	if stats.Warm != 1 {
		t.Fatalf("warm = %d, want 1", stats.Warm)
	}
}

func TestAgentService_RequestSpawnQueuesWhenAdmissionHasNoImmediateSlot(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Queued spawn",
		Description: "Queue instead of starting immediately",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	queueStore := &mockAgentQueueStore{}
	agentPool := pool.NewPool(1)
	if err := agentPool.Acquire("run-existing", uuid.NewString(), uuid.NewString()); err != nil {
		t.Fatalf("Acquire() error = %v", err)
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetPool(agentPool)
	svc.SetQueueStore(queueStore)

	result, err := svc.RequestSpawn(context.Background(), taskID, memberID, "codex", "openai", "gpt-5-codex", 5, "", model.PriorityNormal)
	if err != nil {
		t.Fatalf("RequestSpawn() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusQueued {
		t.Fatalf("dispatch = %+v, want queued", result.Dispatch)
	}
	if result.Dispatch.Queue == nil {
		t.Fatal("expected queue payload for queued request")
	}
	if result.Dispatch.Runtime != "codex" || result.Dispatch.Provider != "openai" || result.Dispatch.Model != "gpt-5-codex" {
		t.Fatalf("dispatch tuple = %+v", result.Dispatch)
	}
	if result.Dispatch.Queue.RecoveryDisposition != model.QueueRecoveryDispositionPending {
		t.Fatalf("queue recovery disposition = %q", result.Dispatch.Queue.RecoveryDisposition)
	}
	if len(repo.runsByTask[taskID]) != 0 {
		t.Fatalf("expected no real agent run to be created while queued, got %d", len(repo.runsByTask[taskID]))
	}
}

func TestAgentService_RequestSpawnQueuesWhenBridgePoolAtCapacity(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Bridge pool full",
		Description: "Queue because bridge runtime pool is full",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	queueStore := &mockAgentQueueStore{}
	bridge := &mockAgentBridge{
		poolSummary: &bridgeclient.PoolSummaryResponse{
			Active: 2,
			Max:    2,
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, &mockWorktreeManager{}, nil)
	svc.SetPool(pool.NewPool(4))
	svc.SetQueueStore(queueStore)

	result, err := svc.RequestSpawn(context.Background(), taskID, memberID, "codex", "openai", "gpt-5-codex", 5, "", model.PriorityNormal)
	if err != nil {
		t.Fatalf("RequestSpawn() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusQueued {
		t.Fatalf("dispatch = %+v, want queued", result.Dispatch)
	}
	if result.Dispatch.Queue == nil {
		t.Fatal("expected queue payload")
	}
	if result.Dispatch.Runtime != "codex" || result.Dispatch.Provider != "openai" || result.Dispatch.Model != "gpt-5-codex" {
		t.Fatalf("dispatch tuple = %+v", result.Dispatch)
	}
	if result.Dispatch.Queue.GuardrailType != model.DispatchGuardrailTypePool || result.Dispatch.Queue.GuardrailScope != "bridge" {
		t.Fatalf("queue verdict = %+v", result.Dispatch.Queue)
	}
	for _, want := range []string{"Bridge pool at capacity", "Wait in queue", "Proceed anyway"} {
		if !strings.Contains(result.Dispatch.Reason, want) {
			t.Fatalf("reason = %q, want substring %q", result.Dispatch.Reason, want)
		}
	}
	if len(repo.runsByTask[taskID]) != 0 {
		t.Fatalf("expected no run creation, got %d", len(repo.runsByTask[taskID]))
	}
}

func TestAgentService_RequestSpawnBlocksWhenBridgeHealthCheckFailsAfterRetries(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Bridge health failed",
		Description: "Block because bridge is unavailable",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{
		healthSeq: []error{
			errors.New("bridge down"),
			errors.New("bridge still down"),
			errors.New("bridge unavailable"),
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, &mockWorktreeManager{}, nil)
	svc.SetPool(pool.NewPool(4))

	result, err := svc.RequestSpawn(context.Background(), taskID, memberID, "codex", "openai", "gpt-5-codex", 5, "", model.PriorityNormal)
	if err != nil {
		t.Fatalf("RequestSpawn() error = %v", err)
	}

	if result.Dispatch.Status != model.DispatchStatusBlocked {
		t.Fatalf("dispatch = %+v, want blocked", result.Dispatch)
	}
	if result.Dispatch.GuardrailType != model.DispatchGuardrailTypeSystem || result.Dispatch.GuardrailScope != "bridge" {
		t.Fatalf("guardrail = %+v", result.Dispatch)
	}
	if result.Dispatch.Runtime != "codex" || result.Dispatch.Provider != "openai" || result.Dispatch.Model != "gpt-5-codex" {
		t.Fatalf("dispatch tuple = %+v", result.Dispatch)
	}
	for _, want := range []string{"Bridge is unavailable", "Retry", "Cancel"} {
		if !strings.Contains(result.Dispatch.Reason, want) {
			t.Fatalf("reason = %q, want substring %q", result.Dispatch.Reason, want)
		}
	}
	if bridge.healthCalls != 3 {
		t.Fatalf("healthCalls = %d, want 3", bridge.healthCalls)
	}
	if len(repo.runsByTask[taskID]) != 0 {
		t.Fatalf("expected no run creation, got %d", len(repo.runsByTask[taskID]))
	}
}

func TestAgentService_SpawnRetriesBridgeHealthCheckBeforeExecution(t *testing.T) {
	taskID := uuid.New()
	memberID := uuid.New()
	projectID := uuid.New()
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{task: &model.Task{
		ID:          taskID,
		ProjectID:   projectID,
		Title:       "Retry bridge health",
		Description: "Spawn after transient bridge health failures",
		BudgetUsd:   5,
	}}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{
		healthSeq: []error{
			errors.New("timeout"),
			errors.New("timeout"),
			nil,
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, &mockWorktreeManager{}, nil)
	svc.SetPool(pool.NewPool(4))

	run, err := svc.Spawn(context.Background(), taskID, memberID, "codex", "openai", "gpt-5-codex", 5, "")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}

	if run == nil || run.Status != model.AgentRunStatusRunning {
		t.Fatalf("run = %+v", run)
	}
	if bridge.healthCalls != 3 {
		t.Fatalf("healthCalls = %d, want 3", bridge.healthCalls)
	}
	if bridge.lastExecute.TaskID != taskID.String() {
		t.Fatalf("bridge execute task id = %q, want %q", bridge.lastExecute.TaskID, taskID.String())
	}
}

func TestAgentService_UpdateStatusPromotesQueuedAdmissionAfterTerminalRelease(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()
	completedTaskID := uuid.New()
	queuedTaskID := uuid.New()
	memberID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   completedTaskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			completedTaskID: {
				ID:             completedTaskID,
				ProjectID:      projectID,
				Title:          "Completed task",
				BudgetUsd:      5,
				AgentBranch:    "agent/" + completedTaskID.String(),
				AgentWorktree:  "/tmp/worktree/" + completedTaskID.String(),
				AgentSessionID: "session-complete",
			},
			queuedTaskID: {
				ID:          queuedTaskID,
				ProjectID:   projectID,
				Title:       "Queued task",
				Description: "Should be promoted",
				BudgetUsd:   4,
			},
		},
	}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{
		allocation: &worktree.Allocation{
			ProjectSlug: "agentforge",
			TaskID:      queuedTaskID.String(),
			Branch:      "agent/" + queuedTaskID.String(),
			Path:        "/tmp/worktree/" + queuedTaskID.String(),
		},
	}
	queueStore := &mockAgentQueueStore{
		next: &model.AgentPoolQueueEntry{
			EntryID:   uuid.NewString(),
			ProjectID: projectID.String(),
			TaskID:    queuedTaskID.String(),
			MemberID:  memberID.String(),
			Status:    model.AgentPoolQueueStatusQueued,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5-codex",
			RoleID:    "frontend-developer",
			BudgetUSD: 4,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	roleStore := &mockAgentRoleStore{
		roles: map[string]*rolepkg.Manifest{
			"frontend-developer": {
				Metadata: model.RoleMetadata{
					ID:   "frontend-developer",
					Name: "Frontend Developer",
				},
				Identity: model.RoleIdentity{
					Role:      "Senior Frontend Developer",
					Goal:      "Build reliable UI",
					Backstory: "A frontend specialist.",
				},
				SystemPrompt: "Keep UI consistent.",
				Capabilities: model.RoleCapabilities{
					AllowedTools: []string{"Read", "Edit"},
					ToolConfig: model.RoleToolConfig{
						External: []string{"github-tool"},
					},
					MaxTurns: 12,
				},
				Security: model.RoleSecurity{
					PermissionMode: "default",
					OutputFilters:  []string{"no_pii"},
					MaxBudgetUsd:   4,
				},
			},
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, roleStore)
	svc.SetPool(pool.NewPool(2))
	svc.SetQueueStore(queueStore)

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if bridge.lastExecute.TaskID != queuedTaskID.String() {
		t.Fatalf("bridge execute task id = %q, want %q", bridge.lastExecute.TaskID, queuedTaskID.String())
	}
	if bridge.lastExecute.RoleConfig == nil || bridge.lastExecute.RoleConfig.RoleID != "frontend-developer" {
		t.Fatalf("bridge execute role config = %#v, want frontend-developer", bridge.lastExecute.RoleConfig)
	}
	if len(queueStore.completed) == 0 {
		t.Fatal("expected queued entry completion after promotion")
	}
}

func TestAgentService_UpdateStatusRequeuesPromotionWhenBudgetCheckBlocks(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()
	completedTaskID := uuid.New()
	queuedTaskID := uuid.New()
	memberID := uuid.New()
	sprintID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   completedTaskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			completedTaskID: {
				ID:             completedTaskID,
				ProjectID:      projectID,
				Title:          "Completed task",
				BudgetUsd:      5,
				AgentBranch:    "agent/" + completedTaskID.String(),
				AgentWorktree:  "/tmp/worktree/" + completedTaskID.String(),
				AgentSessionID: "session-complete",
			},
			queuedTaskID: {
				ID:          queuedTaskID,
				ProjectID:   projectID,
				SprintID:    &sprintID,
				Title:       "Queued task",
				Description: "Should stay queued when budget blocks promotion",
				BudgetUsd:   4,
			},
		},
	}
	projectRepo := &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}
	bridge := &mockAgentBridge{}
	worktrees := &mockWorktreeManager{}
	queueStore := &mockAgentQueueStore{
		next: &model.AgentPoolQueueEntry{
			EntryID:   uuid.NewString(),
			ProjectID: projectID.String(),
			TaskID:    queuedTaskID.String(),
			MemberID:  memberID.String(),
			Status:    model.AgentPoolQueueStatusQueued,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5-codex",
			BudgetUSD: 4,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}

	svc := service.NewAgentService(repo, taskRepo, projectRepo, ws.NewHub(), bridge, worktrees, nil)
	svc.SetPool(pool.NewPool(2))
	svc.SetQueueStore(queueStore)
	svc.SetDispatchBudgetChecker(&mockDispatchBudgetChecker{
		result: &service.BudgetCheckResult{
			Allowed: false,
			Reason:  "project budget exceeded",
		},
	})

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if bridge.lastExecute.TaskID != "" {
		t.Fatalf("bridge execute should not run, got %+v", bridge.lastExecute)
	}
	if len(queueStore.completed) != 1 {
		t.Fatalf("queue completions = %+v, want one requeue", queueStore.completed)
	}
	if got := queueStore.completed[0]; !strings.Contains(got, string(model.AgentPoolQueueStatusQueued)) || !strings.Contains(got, "project budget exceeded") {
		t.Fatalf("queue completion = %q, want queued budget-blocked update", got)
	}
	if queueStore.next.GuardrailType != model.DispatchGuardrailTypeBudget || queueStore.next.GuardrailScope != "project" {
		t.Fatalf("queue verdict = %+v", queueStore.next)
	}
	if queueStore.next.RecoveryDisposition != model.QueueRecoveryDispositionRecoverable {
		t.Fatalf("queue recovery disposition = %q", queueStore.next.RecoveryDisposition)
	}
}

func TestAgentService_UpdateStatusRequeuesPromotionWhenBudgetCheckBlocksRecordsDispatchAttempt(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()
	completedTaskID := uuid.New()
	queuedTaskID := uuid.New()
	memberID := uuid.New()
	sprintID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   completedTaskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			completedTaskID: {ID: completedTaskID, ProjectID: projectID, Title: "Completed task"},
			queuedTaskID: {
				ID:          queuedTaskID,
				ProjectID:   projectID,
				SprintID:    &sprintID,
				Title:       "Queued task",
				Description: "Should record promotion requeue history",
				BudgetUsd:   4,
			},
		},
	}
	queueStore := &mockAgentQueueStore{
		next: &model.AgentPoolQueueEntry{
			EntryID:   uuid.NewString(),
			ProjectID: projectID.String(),
			TaskID:    queuedTaskID.String(),
			MemberID:  memberID.String(),
			Status:    model.AgentPoolQueueStatusQueued,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5-codex",
			RoleID:    "frontend-developer",
			Priority:  model.PriorityHigh,
			BudgetUSD: 4,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	attempts := &mockDispatchAttemptRecorder{}

	svc := service.NewAgentService(repo, taskRepo, &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetPool(pool.NewPool(2))
	svc.SetQueueStore(queueStore)
	svc.SetDispatchBudgetChecker(&mockDispatchBudgetChecker{
		result: &service.BudgetCheckResult{
			Allowed: false,
			Reason:  "project budget exceeded",
		},
	})
	svc.SetDispatchAttemptRecorder(attempts)

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if len(attempts.attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(attempts.attempts))
	}
	attempt := attempts.attempts[0]
	if attempt.TriggerSource != "promotion" || attempt.Outcome != model.DispatchStatusBlocked {
		t.Fatalf("attempt = %+v, want promotion blocked", attempt)
	}
	if attempt.QueueEntryID != queueStore.next.EntryID || attempt.QueuePriority == nil || *attempt.QueuePriority != model.PriorityHigh {
		t.Fatalf("attempt queue linkage = %+v", attempt)
	}
	if attempt.GuardrailType != model.DispatchGuardrailTypeBudget || attempt.GuardrailScope != "project" {
		t.Fatalf("attempt guardrail = %+v", attempt)
	}
	if attempt.RecoveryDisposition != model.QueueRecoveryDispositionRecoverable {
		t.Fatalf("attempt recoveryDisposition = %q", attempt.RecoveryDisposition)
	}
}

func TestAgentService_UpdateStatusPromotionFailsInvalidMemberAndRecordsAttempt(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()
	completedTaskID := uuid.New()
	queuedTaskID := uuid.New()
	memberID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   completedTaskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			completedTaskID: {ID: completedTaskID, ProjectID: projectID, Title: "Completed task"},
			queuedTaskID: {
				ID:          queuedTaskID,
				ProjectID:   projectID,
				Title:       "Queued task",
				Description: "Should fail terminally when member is inactive",
				BudgetUsd:   3,
			},
		},
	}
	queueStore := &mockAgentQueueStore{
		next: &model.AgentPoolQueueEntry{
			EntryID:   uuid.NewString(),
			ProjectID: projectID.String(),
			TaskID:    queuedTaskID.String(),
			MemberID:  memberID.String(),
			Status:    model.AgentPoolQueueStatusQueued,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5-codex",
			BudgetUSD: 3,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	memberRepo := &mockAgentDispatchMemberRepo{
		member: &model.Member{ID: memberID, ProjectID: projectID, Type: model.MemberTypeAgent, IsActive: false},
	}
	attempts := &mockDispatchAttemptRecorder{}

	hub := ws.NewHub()
	stop, events := subscribeProjectEvents(t, hub, projectID.String())
	defer stop()
	waitForHubClient(t, hub)

	svc := service.NewAgentService(repo, taskRepo, &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}, hub, &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetPool(pool.NewPool(2))
	svc.SetQueueStore(queueStore)
	svc.SetDispatchMemberReader(memberRepo)
	svc.SetDispatchAttemptRecorder(attempts)

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	if queueStore.next.Status != model.AgentPoolQueueStatusFailed || queueStore.next.RecoveryDisposition != model.QueueRecoveryDispositionTerminal {
		t.Fatalf("queue verdict = %+v", queueStore.next)
	}
	if len(attempts.attempts) != 1 {
		t.Fatalf("attempt count = %d, want 1", len(attempts.attempts))
	}
	attempt := attempts.attempts[0]
	if attempt.TriggerSource != "promotion" || attempt.Outcome != model.DispatchStatusBlocked {
		t.Fatalf("attempt = %+v, want promotion blocked", attempt)
	}
	if attempt.GuardrailType != model.DispatchGuardrailTypeTarget || attempt.GuardrailScope != "member" {
		t.Fatalf("attempt guardrail = %+v", attempt)
	}
	if attempt.RecoveryDisposition != model.QueueRecoveryDispositionTerminal {
		t.Fatalf("attempt recoveryDisposition = %q", attempt.RecoveryDisposition)
	}

	failed := waitForEventType(t, events, ws.EventAgentQueueFailed)
	var payload struct {
		Queue model.QueueEntryDTO `json:"queue"`
		Error string              `json:"error"`
	}
	if err := json.Unmarshal(failed.Payload, &payload); err != nil {
		t.Fatalf("decode failed payload: %v", err)
	}
	if payload.Queue.Status != string(model.AgentPoolQueueStatusFailed) || payload.Queue.RecoveryDisposition != model.QueueRecoveryDispositionTerminal {
		t.Fatalf("failed payload queue = %+v", payload.Queue)
	}
}

func TestAgentService_UpdateStatusPromotionEmitsFinalizedPromotedPayload(t *testing.T) {
	projectID := uuid.New()
	runID := uuid.New()
	completedTaskID := uuid.New()
	queuedTaskID := uuid.New()
	memberID := uuid.New()
	repo := newMockAgentRunRepo()
	repo.runs[runID] = &model.AgentRun{
		ID:       runID,
		TaskID:   completedTaskID,
		MemberID: memberID,
		Status:   model.AgentRunStatusRunning,
	}
	taskRepo := &mockAgentTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			completedTaskID: {ID: completedTaskID, ProjectID: projectID, Title: "Completed task"},
			queuedTaskID: {
				ID:          queuedTaskID,
				ProjectID:   projectID,
				Title:       "Queued task",
				Description: "Should emit finalized promoted payload",
				BudgetUsd:   4,
			},
		},
	}
	queueStore := &mockAgentQueueStore{
		next: &model.AgentPoolQueueEntry{
			EntryID:   uuid.NewString(),
			ProjectID: projectID.String(),
			TaskID:    queuedTaskID.String(),
			MemberID:  memberID.String(),
			Status:    model.AgentPoolQueueStatusQueued,
			Runtime:   "codex",
			Provider:  "openai",
			Model:     "gpt-5-codex",
			BudgetUSD: 4,
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
		},
	}
	memberRepo := &mockAgentDispatchMemberRepo{
		member: &model.Member{ID: memberID, ProjectID: projectID, Type: model.MemberTypeAgent, IsActive: true},
	}
	hub := ws.NewHub()
	stop, events := subscribeProjectEvents(t, hub, projectID.String())
	defer stop()
	waitForHubClient(t, hub)

	svc := service.NewAgentService(repo, taskRepo, &mockAgentProjectRepo{project: &model.Project{ID: projectID, Slug: "agentforge"}}, hub, &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetPool(pool.NewPool(2))
	svc.SetQueueStore(queueStore)
	svc.SetDispatchMemberReader(memberRepo)

	if err := svc.UpdateStatus(context.Background(), runID, model.AgentRunStatusCompleted); err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}

	promoted := waitForEventType(t, events, ws.EventAgentQueuePromoted)
	var payload struct {
		Queue model.QueueEntryDTO `json:"queue"`
		Run   model.AgentRunDTO   `json:"run"`
	}
	if err := json.Unmarshal(promoted.Payload, &payload); err != nil {
		t.Fatalf("decode promoted payload: %v", err)
	}
	if payload.Queue.Status != string(model.AgentPoolQueueStatusPromoted) {
		t.Fatalf("queue status = %q, want promoted", payload.Queue.Status)
	}
	if payload.Queue.RecoveryDisposition != model.QueueRecoveryDispositionPromoted {
		t.Fatalf("queue recoveryDisposition = %q, want promoted", payload.Queue.RecoveryDisposition)
	}
	if payload.Queue.AgentRunID == nil || *payload.Queue.AgentRunID != payload.Run.ID {
		t.Fatalf("payload linkage = %+v", payload)
	}
}

func TestAgentService_CancelQueueEntryCancelsQueuedEntryAndBroadcastsEvents(t *testing.T) {
	projectID := uuid.New()
	taskID := uuid.New()
	memberID := uuid.New()
	base := time.Now().UTC().Add(-time.Minute)
	queueEntry := &model.AgentPoolQueueEntry{
		EntryID:   uuid.NewString(),
		ProjectID: projectID.String(),
		TaskID:    taskID.String(),
		MemberID:  memberID.String(),
		Status:    model.AgentPoolQueueStatusQueued,
		Reason:    "agent pool is at capacity",
		Runtime:   "codex",
		Provider:  "openai",
		Model:     "gpt-5-codex",
		RoleID:    "dispatcher",
		Priority:  model.PriorityHigh,
		BudgetUSD: 12.5,
		CreatedAt: base,
		UpdatedAt: base,
	}
	queueStore := &mockAgentQueueStore{queued: []*model.AgentPoolQueueEntry{queueEntry}}
	repo := newMockAgentRunRepo()
	taskRepo := &mockAgentTaskRepo{
		tasks: map[uuid.UUID]*model.Task{
			taskID: {
				ID:        taskID,
				ProjectID: projectID,
				CreatedAt: base,
				UpdatedAt: base,
			},
		},
	}
	hub := ws.NewHub()
	stop, events := subscribeProjectEvents(t, hub, projectID.String())
	defer stop()
	waitForHubClient(t, hub)

	svc := service.NewAgentService(repo, taskRepo, &mockAgentProjectRepo{}, hub, &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetPool(pool.NewPool(2))
	svc.SetQueueStore(queueStore)

	entry, err := svc.CancelQueueEntry(context.Background(), projectID, queueEntry.EntryID, "cancelled_by_operator")
	if err != nil {
		t.Fatalf("CancelQueueEntry() error = %v", err)
	}
	if entry.Status != model.AgentPoolQueueStatusCancelled {
		t.Fatalf("entry.Status = %q, want %q", entry.Status, model.AgentPoolQueueStatusCancelled)
	}
	if entry.Reason != "cancelled_by_operator" {
		t.Fatalf("entry.Reason = %q, want cancelled_by_operator", entry.Reason)
	}
	if len(queueStore.completed) != 1 || !strings.Contains(queueStore.completed[0], string(model.AgentPoolQueueStatusCancelled)) {
		t.Fatalf("queue completions = %+v, want cancelled completion", queueStore.completed)
	}

	cancelled := waitForEventType(t, events, ws.EventAgentQueueCancelled)
	var cancelledPayload map[string]any
	if err := json.Unmarshal(cancelled.Payload, &cancelledPayload); err != nil {
		t.Fatalf("decode cancelled payload: %v", err)
	}
	if cancelledPayload["entryId"] != queueEntry.EntryID {
		t.Fatalf("cancelled payload entryId = %#v, want %q", cancelledPayload["entryId"], queueEntry.EntryID)
	}
	if cancelledPayload["taskId"] != queueEntry.TaskID {
		t.Fatalf("cancelled payload taskId = %#v, want %q", cancelledPayload["taskId"], queueEntry.TaskID)
	}
	if cancelledPayload["memberId"] != queueEntry.MemberID {
		t.Fatalf("cancelled payload memberId = %#v, want %q", cancelledPayload["memberId"], queueEntry.MemberID)
	}
	if cancelledPayload["projectId"] != queueEntry.ProjectID {
		t.Fatalf("cancelled payload projectId = %#v, want %q", cancelledPayload["projectId"], queueEntry.ProjectID)
	}
	if cancelledPayload["reason"] != "cancelled_by_operator" {
		t.Fatalf("cancelled payload reason = %#v, want cancelled_by_operator", cancelledPayload["reason"])
	}

	poolUpdated := waitForEventType(t, events, ws.EventAgentPoolUpdated)
	var stats model.AgentPoolStatsDTO
	if err := json.Unmarshal(poolUpdated.Payload, &stats); err != nil {
		t.Fatalf("decode pool payload: %v", err)
	}
	if stats.Queued != 0 {
		t.Fatalf("pool queued = %d, want 0 after cancel", stats.Queued)
	}
}

func TestAgentService_CancelQueueEntryReturnsConflictForPromotedEntry(t *testing.T) {
	projectID := uuid.New()
	queueStore := &mockAgentQueueStore{
		queued: []*model.AgentPoolQueueEntry{
			{
				EntryID:   uuid.NewString(),
				ProjectID: projectID.String(),
				Status:    model.AgentPoolQueueStatusPromoted,
			},
		},
	}
	svc := service.NewAgentService(newMockAgentRunRepo(), &mockAgentTaskRepo{}, &mockAgentProjectRepo{}, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetQueueStore(queueStore)

	_, err := svc.CancelQueueEntry(context.Background(), projectID, queueStore.queued[0].EntryID, "cancelled_by_operator")
	if err == nil {
		t.Fatal("CancelQueueEntry() error = nil, want conflict")
	}
	var conflictErr *service.QueueEntryStatusConflictError
	if !errors.As(err, &conflictErr) {
		t.Fatalf("CancelQueueEntry() error = %v, want QueueEntryStatusConflictError", err)
	}
	if conflictErr.Status != model.AgentPoolQueueStatusPromoted {
		t.Fatalf("conflict status = %q, want promoted", conflictErr.Status)
	}
	if len(queueStore.completed) != 0 {
		t.Fatalf("queue completions = %+v, want none", queueStore.completed)
	}
}

func TestAgentService_CancelQueueEntryReturnsNotFoundForUnknownEntry(t *testing.T) {
	projectID := uuid.New()
	svc := service.NewAgentService(newMockAgentRunRepo(), &mockAgentTaskRepo{}, &mockAgentProjectRepo{}, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetQueueStore(&mockAgentQueueStore{})

	_, err := svc.CancelQueueEntry(context.Background(), projectID, uuid.NewString(), "cancelled_by_operator")
	if !errors.Is(err, service.ErrQueueEntryNotFound) {
		t.Fatalf("CancelQueueEntry() error = %v, want ErrQueueEntryNotFound", err)
	}
}

func TestAgentService_ListQueueEntriesSortsAndFiltersStatuses(t *testing.T) {
	projectID := uuid.New()
	base := time.Now().UTC().Add(-time.Minute)
	normalOlder := &model.AgentPoolQueueEntry{
		EntryID:   "normal-older",
		ProjectID: projectID.String(),
		Status:    model.AgentPoolQueueStatusQueued,
		Priority:  model.PriorityNormal,
		CreatedAt: base.Add(10 * time.Second),
		UpdatedAt: base.Add(10 * time.Second),
	}
	highPriority := &model.AgentPoolQueueEntry{
		EntryID:   "critical",
		ProjectID: projectID.String(),
		Status:    model.AgentPoolQueueStatusQueued,
		Priority:  model.PriorityCritical,
		CreatedAt: base.Add(30 * time.Second),
		UpdatedAt: base.Add(30 * time.Second),
	}
	cancelled := &model.AgentPoolQueueEntry{
		EntryID:   "cancelled",
		ProjectID: projectID.String(),
		Status:    model.AgentPoolQueueStatusCancelled,
		Priority:  model.PriorityHigh,
		CreatedAt: base.Add(20 * time.Second),
		UpdatedAt: base.Add(20 * time.Second),
	}
	normalNewer := &model.AgentPoolQueueEntry{
		EntryID:   "normal-newer",
		ProjectID: projectID.String(),
		Status:    model.AgentPoolQueueStatusQueued,
		Priority:  model.PriorityNormal,
		CreatedAt: base.Add(40 * time.Second),
		UpdatedAt: base.Add(40 * time.Second),
	}
	queueStore := &mockAgentQueueStore{queued: []*model.AgentPoolQueueEntry{normalNewer, highPriority, cancelled, normalOlder}}
	svc := service.NewAgentService(newMockAgentRunRepo(), &mockAgentTaskRepo{}, &mockAgentProjectRepo{}, ws.NewHub(), &mockAgentBridge{}, &mockWorktreeManager{}, nil)
	svc.SetQueueStore(queueStore)

	entries, err := svc.ListQueueEntries(context.Background(), projectID, "")
	if err != nil {
		t.Fatalf("ListQueueEntries() error = %v", err)
	}
	if got := []string{entries[0].EntryID, entries[1].EntryID, entries[2].EntryID}; !reflect.DeepEqual(got, []string{"critical", "normal-older", "normal-newer"}) {
		t.Fatalf("queued order = %+v, want [critical normal-older normal-newer]", got)
	}

	cancelledEntries, err := svc.ListQueueEntries(context.Background(), projectID, string(model.AgentPoolQueueStatusCancelled))
	if err != nil {
		t.Fatalf("ListQueueEntries(cancelled) error = %v", err)
	}
	if len(cancelledEntries) != 1 || cancelledEntries[0].EntryID != "cancelled" {
		t.Fatalf("cancelled entries = %+v, want [cancelled]", cancelledEntries)
	}
}

func assertBridgeRoleConfigStringSlice(t *testing.T, cfg *bridgeclient.RoleConfig, fieldName string, want []string) {
	t.Helper()
	if cfg == nil {
		t.Fatal("expected non-nil role config")
	}
	rv := reflect.ValueOf(cfg).Elem().FieldByName(fieldName)
	if !rv.IsValid() {
		t.Fatalf("expected field %s on bridge role config", fieldName)
	}
	got := make([]string, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		got[i] = rv.Index(i).String()
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %v, want %v", fieldName, got, want)
	}
}

func assertBridgeRoleConfigStringField(t *testing.T, cfg *bridgeclient.RoleConfig, fieldName string) string {
	t.Helper()
	if cfg == nil {
		t.Fatal("expected non-nil role config")
	}
	field := reflect.ValueOf(cfg).Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected field %s on bridge role config", fieldName)
	}
	return field.String()
}

func assertBridgeExecuteStringField(t *testing.T, req service.BridgeExecuteRequest, fieldName, want string) {
	t.Helper()
	field := reflect.ValueOf(req).FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("expected field %s on bridge execute request", fieldName)
	}
	if got := field.String(); got != want {
		t.Fatalf("%s = %q, want %q", fieldName, got, want)
	}
}

type observedEvent struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func subscribeProjectEvents(t *testing.T, hub *ws.Hub, projectID string) (func(), <-chan observedEvent) {
	t.Helper()

	go hub.Run()

	e := echo.New()
	secret := "test-secret"
	e.GET("/ws", ws.NewHandler(hub, secret).HandleWS)

	server := httptest.NewServer(e)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": "tester",
	})
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		server.Close()
		t.Fatalf("sign jwt: %v", err)
	}

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws?token=" + tokenString + "&projectId=" + projectID
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial websocket: %v", err)
	}

	events := make(chan observedEvent, 16)
	done := make(chan struct{})
	ready := make(chan struct{})

	go func() {
		close(ready)
		defer close(events)
		for {
			if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				return
			}
			_, message, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var event observedEvent
			if err := json.Unmarshal(message, &event); err != nil {
				return
			}

			select {
			case events <- event:
			case <-done:
				return
			}
		}
	}()
	<-ready
	time.Sleep(25 * time.Millisecond)

	return func() {
		close(done)
		_ = conn.Close()
		server.Close()
	}, events
}

func waitForEventType(t *testing.T, events <-chan observedEvent, eventType string) observedEvent {
	t.Helper()

	deadline := time.After(5 * time.Second)
	for {
		select {
		case event, ok := <-events:
			if !ok {
				t.Fatalf("event stream closed before receiving %s", eventType)
			}
			if event.Type == eventType {
				return event
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", eventType)
		}
	}
}
