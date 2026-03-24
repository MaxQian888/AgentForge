package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
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

func (m *mockAgentRunRepo) UpdateCost(_ context.Context, _ uuid.UUID, _, _, _ int64, _ float64, _ int) error {
	return nil
}

type mockAgentBridge struct {
	executeErr   error
	lastExecute  service.BridgeExecuteRequest
	cancelTaskID string
	cancelReason string
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

func (m *mockAgentBridge) Cancel(_ context.Context, taskID, reason string) error {
	m.cancelTaskID = taskID
	m.cancelReason = reason
	return nil
}

type mockAgentTaskRepo struct {
	task            *model.Task
	updatedBranch   string
	updatedWorktree string
	updatedSession  string
	clearCalls      int
}

func (m *mockAgentTaskRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Task, error) {
	if m.task == nil || m.task.ID != id {
		return nil, service.ErrAgentTaskNotFound
	}
	cloned := *m.task
	return &cloned, nil
}

func (m *mockAgentTaskRepo) UpdateRuntime(_ context.Context, _ uuid.UUID, branch, worktreePath, sessionID string) error {
	m.updatedBranch = branch
	m.updatedWorktree = worktreePath
	m.updatedSession = sessionID
	if m.task != nil {
		m.task.AgentBranch = branch
		m.task.AgentWorktree = worktreePath
		m.task.AgentSessionID = sessionID
	}
	return nil
}

func (m *mockAgentTaskRepo) ClearRuntime(_ context.Context, _ uuid.UUID) error {
	m.clearCalls++
	if m.task != nil {
		m.task.AgentBranch = ""
		m.task.AgentWorktree = ""
		m.task.AgentSessionID = ""
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

type mockAgentRoleStore struct {
	roles map[string]*rolepkg.Manifest
}

func (m *mockAgentRoleStore) Get(id string) (*rolepkg.Manifest, error) {
	if manifest, ok := m.roles[id]; ok {
		cloned := *manifest
		return &cloned, nil
	}
	return nil, os.ErrNotExist
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

	_, err := svc.Spawn(context.Background(), taskID, memberID, "opencode", "anthropic", "claude-sonnet", 5, "")
	if err != nil {
		t.Fatalf("Spawn() error = %v", err)
	}
	if bridge.lastExecute.Runtime != "opencode" {
		t.Fatalf("bridge runtime = %q, want opencode", bridge.lastExecute.Runtime)
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
				},
				Security: model.RoleSecurity{
					MaxBudgetUsd:   3.5,
					PermissionMode: "bypassPermissions",
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
