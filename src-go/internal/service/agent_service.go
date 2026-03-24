package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	worktreepkg "github.com/react-go-quick-starter/server/internal/worktree"
	"github.com/react-go-quick-starter/server/internal/ws"
)

// AgentRunRepository defines persistence for agent runs.
type AgentRunRepository interface {
	Create(ctx context.Context, run *model.AgentRun) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error)
	GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error)
	ListActive(ctx context.Context) ([]*model.AgentRun, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	UpdateCost(ctx context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int) error
}

type AgentTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	UpdateRuntime(ctx context.Context, id uuid.UUID, branch, worktreePath, sessionID string) error
	ClearRuntime(ctx context.Context, id uuid.UUID) error
}

type AgentProjectRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
}

type WorktreeManager interface {
	Prepare(ctx context.Context, projectSlug, taskID string) (*worktreepkg.Allocation, error)
	Release(ctx context.Context, projectSlug, taskID string) error
	Path(projectSlug, taskID string) string
	Branch(taskID string) string
}

type AgentRoleStore interface {
	Get(id string) (*rolepkg.Manifest, error)
}

// BridgeClient defines the interface for calling the TypeScript bridge.
type BridgeClient interface {
	Execute(ctx context.Context, req BridgeExecuteRequest) (*BridgeExecuteResponse, error)
	GetStatus(ctx context.Context, taskID string) (*BridgeStatusResponse, error)
	Cancel(ctx context.Context, taskID, reason string) error
}

type BridgeExecuteRequest = bridgeclient.ExecuteRequest
type BridgeExecuteResponse = bridgeclient.ExecuteResponse
type BridgeStatusResponse = bridgeclient.StatusResponse

var (
	ErrAgentAlreadyRunning      = errors.New("agent already running for this task")
	ErrAgentNotFound            = errors.New("agent run not found")
	ErrAgentNotRunning          = errors.New("agent is not running")
	ErrAgentTaskNotFound        = errors.New("agent task not found")
	ErrAgentProjectNotFound     = errors.New("agent project not found")
	ErrAgentRoleNotFound        = errors.New("agent role not found")
	ErrAgentWorktreeUnavailable = errors.New("agent worktree unavailable")
)

type AgentService struct {
	runRepo   AgentRunRepository
	taskRepo  AgentTaskRepository
	projects  AgentProjectRepository
	hub       *ws.Hub
	bridge    BridgeClient
	worktrees WorktreeManager
	roleStore AgentRoleStore
	progress  *TaskProgressService
}

func NewAgentService(
	runRepo AgentRunRepository,
	taskRepo AgentTaskRepository,
	projects AgentProjectRepository,
	hub *ws.Hub,
	bridge BridgeClient,
	worktrees WorktreeManager,
	roleStore ...AgentRoleStore,
) *AgentService {
	var roles AgentRoleStore
	if len(roleStore) > 0 {
		roles = roleStore[0]
	}
	return &AgentService{
		runRepo:   runRepo,
		taskRepo:  taskRepo,
		projects:  projects,
		hub:       hub,
		bridge:    bridge,
		worktrees: worktrees,
		roleStore: roles,
	}
}

func (s *AgentService) SetProgressTracker(progress *TaskProgressService) {
	s.progress = progress
}

// Spawn creates a run, provisions a worktree, starts bridge execution, and publishes lifecycle updates.
func (s *AgentService) Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error) {
	runs, err := s.runRepo.GetByTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("check existing runs: %w", err)
	}
	for _, r := range runs {
		if r.Status == model.AgentRunStatusRunning || r.Status == model.AgentRunStatusStarting {
			return nil, ErrAgentAlreadyRunning
		}
	}

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrAgentTaskNotFound
	}
	project, err := s.projects.GetByID(ctx, task.ProjectID)
	if err != nil {
		return nil, ErrAgentProjectNotFound
	}

	resolvedRoleID := strings.TrimSpace(roleID)
	roleConfig, err := s.resolveRoleConfig(resolvedRoleID)
	if err != nil {
		return nil, err
	}

	run := &model.AgentRun{
		ID:        uuid.New(),
		TaskID:    taskID,
		MemberID:  memberID,
		RoleID:    resolvedRoleID,
		Status:    model.AgentRunStatusStarting,
		Provider:  provider,
		Model:     modelName,
		StartedAt: time.Now().UTC(),
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		return nil, fmt.Errorf("create agent run: %w", err)
	}

	allocation, err := s.worktrees.Prepare(ctx, project.Slug, taskID.String())
	if err != nil {
		_ = s.failSpawn(ctx, run, task, project.Slug, nil)
		return nil, fmt.Errorf("%w: %w", ErrAgentWorktreeUnavailable, err)
	}

	sessionID := uuid.New().String()
	resp, err := s.bridge.Execute(ctx, BridgeExecuteRequest{
		TaskID:         taskID.String(),
		SessionID:      sessionID,
		MemberID:       memberID.String(),
		Runtime:        resolveBridgeRuntime(runtime, provider),
		Provider:       provider,
		Model:          modelName,
		Prompt:         buildSpawnPrompt(task),
		WorktreePath:   allocation.Path,
		BranchName:     allocation.Branch,
		MaxTurns:       resolveSpawnMaxTurns(roleConfig),
		BudgetUSD:      resolveSpawnBudget(task.BudgetUsd, budgetUsd, roleConfig),
		AllowedTools:   resolveSpawnAllowedTools(roleConfig),
		PermissionMode: resolveSpawnPermissionMode(roleConfig),
		RoleConfig:     roleConfig,
	})
	if err != nil {
		_ = s.failSpawn(ctx, run, task, project.Slug, allocation)
		return nil, fmt.Errorf("start bridge execution: %w", err)
	}
	if resp != nil && resp.SessionID != "" {
		sessionID = resp.SessionID
	}

	if err := s.taskRepo.UpdateRuntime(ctx, task.ID, allocation.Branch, allocation.Path, sessionID); err != nil {
		_ = s.failSpawn(ctx, run, task, project.Slug, allocation)
		return nil, fmt.Errorf("persist task runtime: %w", err)
	}
	if err := s.runRepo.UpdateStatus(ctx, run.ID, model.AgentRunStatusRunning); err != nil {
		_ = s.failSpawn(ctx, run, task, project.Slug, allocation)
		return nil, fmt.Errorf("mark run running: %w", err)
	}

	run.Status = model.AgentRunStatusRunning
	s.broadcastEvent(ws.EventAgentStarted, task.ProjectID.String(), run.ToDTO())
	s.recordProgress(ctx, taskID, TaskActivityInput{
		Source:       model.TaskProgressSourceAgentStarted,
		OccurredAt:   run.StartedAt,
		UpdateHealth: true,
	})
	return run, nil
}

// UpdateStatus changes the status of an agent run.
func (s *AgentService) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if err := s.runRepo.UpdateStatus(ctx, id, status); err != nil {
		return fmt.Errorf("update agent status: %w", err)
	}

	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	eventType := ws.EventAgentProgress
	switch status {
	case model.AgentRunStatusCompleted:
		eventType = ws.EventAgentCompleted
	case model.AgentRunStatusFailed, model.AgentRunStatusCancelled, model.AgentRunStatusBudgetExceeded:
		eventType = ws.EventAgentFailed
	}

	if isTerminalAgentStatus(status) {
		if err := s.releaseTaskRuntime(ctx, run.TaskID); err != nil {
			return fmt.Errorf("release managed worktree: %w", err)
		}
	}

	s.broadcastEvent(eventType, s.lookupProjectID(ctx, run.TaskID), run.ToDTO())
	s.recordProgress(ctx, run.TaskID, TaskActivityInput{
		Source:       model.TaskProgressSourceAgentStatus,
		UpdateHealth: true,
	})
	return nil
}

// UpdateCost records cost data for an agent run.
func (s *AgentService) UpdateCost(ctx context.Context, id uuid.UUID, inputTokens, outputTokens, cacheReadTokens int64, costUsd float64, turnCount int) error {
	if err := s.runRepo.UpdateCost(ctx, id, inputTokens, outputTokens, cacheReadTokens, costUsd, turnCount); err != nil {
		return fmt.Errorf("update agent cost: %w", err)
	}

	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	s.broadcastEvent(ws.EventAgentCostUpdate, s.lookupProjectID(ctx, run.TaskID), run.ToDTO())
	s.recordProgress(ctx, run.TaskID, TaskActivityInput{
		Source:       model.TaskProgressSourceAgentHeartbeat,
		UpdateHealth: true,
	})
	return nil
}

// GetByID returns an agent run by ID.
func (s *AgentService) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error) {
	return s.runRepo.GetByID(ctx, id)
}

// GetByTask returns all agent runs for a task.
func (s *AgentService) GetByTask(ctx context.Context, taskID uuid.UUID) ([]*model.AgentRun, error) {
	return s.runRepo.GetByTask(ctx, taskID)
}

// ListActive returns all currently active agent runs.
func (s *AgentService) ListActive(ctx context.Context) ([]*model.AgentRun, error) {
	return s.runRepo.ListActive(ctx)
}

// Cancel stops a running agent.
func (s *AgentService) Cancel(ctx context.Context, id uuid.UUID, reason string) error {
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return ErrAgentNotFound
	}
	if run.Status != model.AgentRunStatusRunning && run.Status != model.AgentRunStatusStarting {
		return ErrAgentNotRunning
	}

	if s.bridge != nil {
		_ = s.bridge.Cancel(ctx, run.TaskID.String(), reason)
	}

	return s.UpdateStatus(ctx, id, model.AgentRunStatusCancelled)
}

func (s *AgentService) failSpawn(ctx context.Context, run *model.AgentRun, task *model.Task, projectSlug string, allocation *worktreepkg.Allocation) error {
	if err := s.runRepo.UpdateStatus(ctx, run.ID, model.AgentRunStatusFailed); err != nil {
		return err
	}
	run.Status = model.AgentRunStatusFailed
	if s.taskRepo != nil {
		_ = s.taskRepo.ClearRuntime(ctx, task.ID)
	}
	if allocation != nil && !allocation.Reused && s.worktrees != nil {
		_ = s.worktrees.Release(ctx, projectSlug, task.ID.String())
	}
	s.broadcastEvent(ws.EventAgentFailed, task.ProjectID.String(), run.ToDTO())
	return nil
}

func (s *AgentService) releaseTaskRuntime(ctx context.Context, taskID uuid.UUID) error {
	if s.taskRepo == nil {
		return nil
	}

	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return err
	}

	if task.AgentBranch == "" && task.AgentWorktree == "" && task.AgentSessionID == "" {
		return nil
	}

	if s.worktrees != nil && s.projects != nil {
		project, err := s.projects.GetByID(ctx, task.ProjectID)
		if err != nil {
			return err
		}
		canonicalBranch := s.worktrees.Branch(taskID.String())
		canonicalPath := s.worktrees.Path(project.Slug, taskID.String())
		if task.AgentBranch == canonicalBranch && task.AgentWorktree == canonicalPath {
			if err := s.worktrees.Release(ctx, project.Slug, taskID.String()); err != nil {
				return err
			}
		}
	}

	return s.taskRepo.ClearRuntime(ctx, taskID)
}

func isTerminalAgentStatus(status string) bool {
	switch status {
	case model.AgentRunStatusCompleted, model.AgentRunStatusFailed, model.AgentRunStatusCancelled, model.AgentRunStatusBudgetExceeded:
		return true
	default:
		return false
	}
}

func (s *AgentService) lookupProjectID(ctx context.Context, taskID uuid.UUID) string {
	if s.taskRepo == nil {
		return ""
	}
	task, err := s.taskRepo.GetByID(ctx, taskID)
	if err != nil {
		return ""
	}
	return task.ProjectID.String()
}

func (s *AgentService) broadcastEvent(eventType, projectID string, payload any) {
	if s.hub == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      eventType,
		ProjectID: projectID,
		Payload:   payload,
	})
}

func (s *AgentService) recordProgress(ctx context.Context, taskID uuid.UUID, input TaskActivityInput) {
	if s.progress == nil {
		return
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = time.Now().UTC()
	}
	_, _ = s.progress.RecordActivity(ctx, taskID, input)
}

func buildSpawnPrompt(task *model.Task) string {
	var prompt strings.Builder
	prompt.WriteString(strings.TrimSpace(task.Title))
	if desc := strings.TrimSpace(task.Description); desc != "" {
		prompt.WriteString("\n\n")
		prompt.WriteString(desc)
	}
	return prompt.String()
}

func resolveSpawnBudget(taskBudget, requestBudget float64, roleConfig *bridgeclient.RoleConfig) float64 {
	budget := minPositive(taskBudget, requestBudget)
	if roleConfig != nil {
		budget = minPositive(budget, roleConfig.MaxBudgetUsd)
	}
	if budget > 0 {
		return budget
	}
	return 1
}

func resolveSpawnMaxTurns(roleConfig *bridgeclient.RoleConfig) int {
	if roleConfig != nil && roleConfig.MaxTurns > 0 {
		return roleConfig.MaxTurns
	}
	return 50
}

func resolveSpawnAllowedTools(roleConfig *bridgeclient.RoleConfig) []string {
	if roleConfig == nil || len(roleConfig.AllowedTools) == 0 {
		return nil
	}
	return append([]string(nil), roleConfig.AllowedTools...)
}

func resolveSpawnPermissionMode(roleConfig *bridgeclient.RoleConfig) string {
	if roleConfig != nil && strings.TrimSpace(roleConfig.PermissionMode) != "" {
		return roleConfig.PermissionMode
	}
	return "default"
}

func minPositive(values ...float64) float64 {
	var min float64
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if min == 0 || value < min {
			min = value
		}
	}
	return min
}

func resolveBridgeRuntime(runtime, provider string) string {
	switch strings.TrimSpace(strings.ToLower(runtime)) {
	case "claude_code", "codex", "opencode":
		return strings.TrimSpace(strings.ToLower(runtime))
	}

	switch strings.TrimSpace(strings.ToLower(provider)) {
	case "", "anthropic", "claude", "claude_code":
		return "claude_code"
	case "codex":
		return "codex"
	case "opencode":
		return "opencode"
	default:
		return ""
	}
}

func (s *AgentService) resolveRoleConfig(roleID string) (*bridgeclient.RoleConfig, error) {
	if roleID == "" {
		return nil, nil
	}
	if s.roleStore == nil {
		return nil, fmt.Errorf("%w: %s", ErrAgentRoleNotFound, roleID)
	}

	manifest, err := s.roleStore.Get(roleID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrAgentRoleNotFound, roleID)
		}
		return nil, fmt.Errorf("load agent role %s: %w", roleID, err)
	}

	profile := rolepkg.BuildExecutionProfile(manifest)
	if profile == nil {
		return nil, fmt.Errorf("%w: %s", ErrAgentRoleNotFound, roleID)
	}

	return &bridgeclient.RoleConfig{
		RoleID:         profile.RoleID,
		Name:           profile.Name,
		Role:           profile.Role,
		Goal:           profile.Goal,
		Backstory:      profile.Backstory,
		SystemPrompt:   profile.SystemPrompt,
		AllowedTools:   append([]string(nil), profile.AllowedTools...),
		MaxBudgetUsd:   profile.MaxBudgetUsd,
		MaxTurns:       profile.MaxTurns,
		PermissionMode: profile.PermissionMode,
	}, nil
}
