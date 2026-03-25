package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	bridgeclient "github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/pool"
	"github.com/react-go-quick-starter/server/internal/repository"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
	worktreepkg "github.com/react-go-quick-starter/server/internal/worktree"
	"github.com/react-go-quick-starter/server/internal/ws"
	log "github.com/sirupsen/logrus"
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
	UpdateSpent(ctx context.Context, id uuid.UUID, spentUsd float64, status string) error
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
	GetPoolSummary(ctx context.Context) (*bridgeclient.PoolSummaryResponse, error)
	Cancel(ctx context.Context, taskID, reason string) error
	Pause(ctx context.Context, taskID, reason string) (*BridgePauseResponse, error)
	Resume(ctx context.Context, req BridgeExecuteRequest) (*BridgeResumeResponse, error)
}

type BridgeExecuteRequest = bridgeclient.ExecuteRequest
type BridgeExecuteResponse = bridgeclient.ExecuteResponse
type BridgeStatusResponse = bridgeclient.StatusResponse
type BridgePauseResponse = bridgeclient.PauseResponse
type BridgeResumeResponse = bridgeclient.ResumeResponse

type QueueAgentAdmissionInput = repository.QueueAgentAdmissionRecord

type AgentQueueStore interface {
	QueueAgentAdmission(ctx context.Context, input QueueAgentAdmissionInput) (*model.AgentPoolQueueEntry, error)
	CountQueuedByProject(ctx context.Context, projectID uuid.UUID) (int, error)
	ListAllQueued(ctx context.Context, limit int) ([]*model.AgentPoolQueueEntry, error)
	ListQueuedByProject(ctx context.Context, projectID uuid.UUID, limit int) ([]*model.AgentPoolQueueEntry, error)
	ReserveNextQueuedByProject(ctx context.Context, projectID uuid.UUID) (*model.AgentPoolQueueEntry, error)
	CompleteQueuedEntry(ctx context.Context, entryID string, status model.AgentPoolQueueStatus, reason string, runID *uuid.UUID) error
}

type agentServiceQueueAdapter struct {
	store AgentQueueStore
}

func (a agentServiceQueueAdapter) QueueAgentAdmission(ctx context.Context, input pool.QueueAdmissionInput) (*model.AgentPoolQueueEntry, error) {
	if a.store == nil {
		return nil, ErrAgentPoolFull
	}
	return a.store.QueueAgentAdmission(ctx, QueueAgentAdmissionInput(input))
}

var (
	ErrAgentAlreadyRunning      = errors.New("agent already running for this task")
	ErrAgentNotFound            = errors.New("agent run not found")
	ErrAgentNotRunning          = errors.New("agent is not running")
	ErrAgentPoolFull            = errors.New("agent pool is full")
	ErrAgentTaskNotFound        = errors.New("agent task not found")
	ErrAgentProjectNotFound     = errors.New("agent project not found")
	ErrAgentRoleNotFound        = errors.New("agent role not found")
	ErrAgentWorktreeUnavailable = errors.New("agent worktree unavailable")
)

type AgentService struct {
	runRepo    AgentRunRepository
	taskRepo   AgentTaskRepository
	projects   AgentProjectRepository
	hub        *ws.Hub
	bridge     BridgeClient
	worktrees  WorktreeManager
	roleStore  AgentRoleStore
	progress   *TaskProgressService
	imProgress IMBoundProgressNotifier
	pool       *pool.Pool
	queueStore AgentQueueStore
	teamSvc    *TeamService
	memorySvc  *MemoryService
}

func agentRunLogFields(run *model.AgentRun) log.Fields {
	if run == nil {
		return log.Fields{}
	}

	fields := log.Fields{
		"runId":     run.ID.String(),
		"taskId":    run.TaskID.String(),
		"memberId":  run.MemberID.String(),
		"status":    run.Status,
		"runtime":   run.Runtime,
		"provider":  run.Provider,
		"model":     run.Model,
		"turnCount": run.TurnCount,
		"costUsd":   run.CostUsd,
		"teamRole":  run.TeamRole,
	}
	if run.TeamID != nil {
		fields["teamId"] = run.TeamID.String()
	}
	if strings.TrimSpace(run.RoleID) != "" {
		fields["roleId"] = run.RoleID
	}
	return fields
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

func (s *AgentService) SetIMProgressNotifier(notifier IMBoundProgressNotifier) {
	s.imProgress = notifier
}

func (s *AgentService) SetPool(agentPool *pool.Pool) {
	s.pool = agentPool
}

func (s *AgentService) SetQueueStore(store AgentQueueStore) {
	s.queueStore = store
}

func (s *AgentService) SetTeamService(ts *TeamService) {
	s.teamSvc = ts
}

func (s *AgentService) SetMemoryService(ms *MemoryService) {
	s.memorySvc = ms
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
	selection, err := ResolveProjectCodingAgentSelection(project, runtime, provider, modelName)
	if err != nil {
		return nil, err
	}
	spawnFields := log.Fields{
		"taskId":      taskID.String(),
		"projectId":   task.ProjectID.String(),
		"memberId":    memberID.String(),
		"runtime":     selection.Runtime,
		"provider":    selection.Provider,
		"model":       selection.Model,
		"budgetUsd":   budgetUsd,
		"projectSlug": project.Slug,
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
		Runtime:   selection.Runtime,
		Provider:  selection.Provider,
		Model:     selection.Model,
		StartedAt: time.Now().UTC(),
	}
	if s.pool != nil {
		if err := s.pool.Acquire(run.ID.String(), taskID.String(), memberID.String()); err != nil {
			if errors.Is(err, pool.ErrPoolFull) {
				return nil, ErrAgentPoolFull
			}
			return nil, fmt.Errorf("acquire agent pool slot: %w", err)
		}
	}
	if err := s.runRepo.Create(ctx, run); err != nil {
		s.releasePoolSlot(run.ID.String())
		return nil, fmt.Errorf("create agent run: %w", err)
	}
	spawnFields["runId"] = run.ID.String()
	if resolvedRoleID != "" {
		spawnFields["roleId"] = resolvedRoleID
	}
	log.WithFields(spawnFields).Info("agent spawn persisted")

	allocation, err := s.worktrees.Prepare(ctx, project.Slug, taskID.String())
	if err != nil {
		log.WithFields(spawnFields).WithError(err).Warn("agent spawn failed to prepare worktree")
		_ = s.failSpawn(ctx, run, task, project.Slug, nil)
		return nil, fmt.Errorf("%w: %w", ErrAgentWorktreeUnavailable, err)
	}
	spawnFields["worktreePath"] = allocation.Path
	spawnFields["branchName"] = allocation.Branch
	spawnFields["worktreeReused"] = allocation.Reused

	sessionID := uuid.New().String()
	resp, err := s.bridge.Execute(ctx, BridgeExecuteRequest{
		TaskID:         taskID.String(),
		SessionID:      sessionID,
		MemberID:       memberID.String(),
		Runtime:        selection.Runtime,
		Provider:       selection.Provider,
		Model:          selection.Model,
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
		log.WithFields(spawnFields).WithError(err).Warn("agent spawn bridge execute failed")
		_ = s.failSpawn(ctx, run, task, project.Slug, allocation)
		return nil, fmt.Errorf("start bridge execution: %w", err)
	}
	if resp != nil && resp.SessionID != "" {
		sessionID = resp.SessionID
	}
	spawnFields["sessionId"] = sessionID

	if err := s.taskRepo.UpdateRuntime(ctx, task.ID, allocation.Branch, allocation.Path, sessionID); err != nil {
		log.WithFields(spawnFields).WithError(err).Warn("agent spawn failed to persist runtime")
		_ = s.failSpawn(ctx, run, task, project.Slug, allocation)
		return nil, fmt.Errorf("persist task runtime: %w", err)
	}
	if err := s.runRepo.UpdateStatus(ctx, run.ID, model.AgentRunStatusRunning); err != nil {
		log.WithFields(spawnFields).WithError(err).Warn("agent spawn failed to mark run running")
		_ = s.failSpawn(ctx, run, task, project.Slug, allocation)
		return nil, fmt.Errorf("mark run running: %w", err)
	}

	run.Status = model.AgentRunStatusRunning
	log.WithFields(spawnFields).Info("agent spawn started bridge runtime")
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
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return ErrAgentNotFound
	}
	fields := agentRunLogFields(run)
	fields["nextStatus"] = status

	switch status {
	case model.AgentRunStatusPaused:
		if run.Status != model.AgentRunStatusRunning && run.Status != model.AgentRunStatusStarting {
			return ErrAgentNotRunning
		}
		return s.pauseRun(ctx, run)
	case model.AgentRunStatusRunning:
		if run.Status == model.AgentRunStatusPaused || run.Status == model.AgentRunStatusBudgetExceeded {
			return s.resumeRun(ctx, run)
		}
	}

	if err := s.runRepo.UpdateStatus(ctx, id, status); err != nil {
		return fmt.Errorf("update agent status: %w", err)
	}
	run.Status = status
	log.WithFields(fields).Info("agent status updated")

	eventType := ws.EventAgentProgress
	switch status {
	case model.AgentRunStatusCompleted:
		eventType = ws.EventAgentCompleted
	case model.AgentRunStatusFailed, model.AgentRunStatusCancelled, model.AgentRunStatusBudgetExceeded:
		eventType = ws.EventAgentFailed
	}

	if isTerminalAgentStatus(status) {
		s.releasePoolSlot(run.ID.String())
		if shouldReleaseTaskRuntime(status) {
			if err := s.releaseTaskRuntime(ctx, run.TaskID); err != nil {
				log.WithFields(fields).WithError(err).Warn("agent status update failed to release task runtime")
				return fmt.Errorf("release managed worktree: %w", err)
			}
		}
		if run.TeamID != nil && s.teamSvc != nil {
			go s.teamSvc.ProcessRunCompletion(context.Background(), run)
		}
		s.promoteQueuedAdmission(ctx, run)
		if projectID := s.lookupProjectID(ctx, run.TaskID); projectID != "" {
			s.broadcastPoolStats(ctx, projectID)
		}
	}

	s.broadcastEvent(eventType, s.lookupProjectID(ctx, run.TaskID), run.ToDTO())
	s.recordProgress(ctx, run.TaskID, TaskActivityInput{
		Source:       model.TaskProgressSourceAgentStatus,
		UpdateHealth: true,
	})
	s.notifyIMRunUpdate(ctx, run, nextAgentRunSummary(run.Status, run.TaskID.String(), run.ID.String()), isTerminalAgentStatus(status))
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

	task, err := s.taskRepo.GetByID(ctx, run.TaskID)
	if err != nil {
		return err
	}

	totalSpent, err := s.sumTaskRunCost(ctx, run.TaskID)
	if err != nil {
		return err
	}
	costFields := agentRunLogFields(run)
	costFields["inputTokens"] = inputTokens
	costFields["outputTokens"] = outputTokens
	costFields["cacheReadTokens"] = cacheReadTokens
	costFields["reportedCostUsd"] = costUsd
	costFields["reportedTurnCount"] = turnCount
	costFields["projectId"] = task.ProjectID.String()
	costFields["taskBudgetUsd"] = task.BudgetUsd
	costFields["taskSpentUsd"] = totalSpent

	nextTaskStatus := ""
	if task.BudgetUsd > 0 && totalSpent >= task.BudgetUsd {
		nextTaskStatus = model.TaskStatusBudgetExceeded
	}
	if err := s.taskRepo.UpdateSpent(ctx, run.TaskID, totalSpent, nextTaskStatus); err != nil {
		return fmt.Errorf("update task runtime cost: %w", err)
	}

	updatedTask, err := s.taskRepo.GetByID(ctx, run.TaskID)
	if err != nil {
		return err
	}
	costFields["updatedTaskStatus"] = updatedTask.Status
	costFields["updatedTaskSpentUsd"] = updatedTask.SpentUsd
	log.WithFields(costFields).Info("agent cost updated")

	s.broadcastEvent(ws.EventAgentCostUpdate, updatedTask.ProjectID.String(), run.ToDTO())
	s.broadcastEvent(ws.EventTaskUpdated, updatedTask.ProjectID.String(), updatedTask.ToDTO())

	if task.BudgetUsd > 0 {
		previousRatio := task.SpentUsd / task.BudgetUsd
		currentRatio := updatedTask.SpentUsd / updatedTask.BudgetUsd

		if previousRatio < 0.8 && currentRatio >= 0.8 && currentRatio < 1 {
			log.WithFields(costFields).WithField("budgetPercent", currentRatio*100).Warn("agent cost crossed budget warning threshold")
			s.broadcastBudgetEvent(ws.EventBudgetWarning, updatedTask, currentRatio*100)
		}

		if previousRatio < 1 && currentRatio >= 1 {
			if s.bridge != nil {
				_ = s.bridge.Cancel(ctx, run.TaskID.String(), "budget_exceeded")
			}
			log.WithFields(costFields).WithField("budgetPercent", currentRatio*100).Warn("agent cost crossed budget exceeded threshold")
			s.broadcastBudgetEvent(ws.EventBudgetExceeded, updatedTask, currentRatio*100)
			if run.Status != model.AgentRunStatusBudgetExceeded {
				if err := s.UpdateStatus(ctx, run.ID, model.AgentRunStatusBudgetExceeded); err != nil {
					return err
				}
			}
		}
	}

	s.recordProgress(ctx, run.TaskID, TaskActivityInput{
		Source:       model.TaskProgressSourceAgentHeartbeat,
		UpdateHealth: true,
	})
	return nil
}

func shouldReleaseTaskRuntime(status string) bool {
	switch status {
	case model.AgentRunStatusBudgetExceeded:
		return false
	default:
		return isTerminalAgentStatus(status)
	}
}

func (s *AgentService) sumTaskRunCost(ctx context.Context, taskID uuid.UUID) (float64, error) {
	runs, err := s.runRepo.GetByTask(ctx, taskID)
	if err != nil {
		return 0, fmt.Errorf("list task runs: %w", err)
	}

	var total float64
	for _, run := range runs {
		if run == nil {
			continue
		}
		total += run.CostUsd
	}
	return total, nil
}

func (s *AgentService) broadcastBudgetEvent(eventType string, task *model.Task, percent float64) {
	if task == nil {
		return
	}
	payload := map[string]any{
		"taskId": task.ID.String(),
		"budget": task.BudgetUsd,
		"spent":  task.SpentUsd,
	}
	if eventType == ws.EventBudgetWarning {
		payload["percent"] = percent
	}
	s.broadcastEvent(eventType, task.ProjectID.String(), payload)
}

// GetByID returns an agent run by ID.
func (s *AgentService) GetByID(ctx context.Context, id uuid.UUID) (*model.AgentRun, error) {
	return s.runRepo.GetByID(ctx, id)
}

// GetLogs returns log entries for an agent run.
func (s *AgentService) GetLogs(ctx context.Context, id uuid.UUID) ([]model.AgentLogEntry, error) {
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return nil, ErrAgentNotFound
	}

	// Return basic log entries from the run record
	var logs []model.AgentLogEntry

	logs = append(logs, model.AgentLogEntry{
		Timestamp: run.StartedAt.Format(time.RFC3339),
		Content:   "Agent run started",
		Type:      "status",
	})

	if run.ErrorMessage != "" {
		logs = append(logs, model.AgentLogEntry{
			Timestamp: run.UpdatedAt.Format(time.RFC3339),
			Content:   run.ErrorMessage,
			Type:      "error",
		})
	}

	if run.CompletedAt != nil {
		logs = append(logs, model.AgentLogEntry{
			Timestamp: run.CompletedAt.Format(time.RFC3339),
			Content:   fmt.Sprintf("Agent run %s (turns: %d, cost: $%.4f)", run.Status, run.TurnCount, run.CostUsd),
			Type:      "status",
		})
	}

	return logs, nil
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

func (s *AgentService) ListSummaries(ctx context.Context) ([]model.AgentRunSummaryDTO, error) {
	runs, err := s.runRepo.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	summaries := make([]model.AgentRunSummaryDTO, 0, len(runs))
	for _, run := range runs {
		summary, err := s.buildSummary(ctx, run)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}
	return summaries, nil
}

func (s *AgentService) GetSummary(ctx context.Context, id uuid.UUID) (*model.AgentRunSummaryDTO, error) {
	run, err := s.runRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	summary, err := s.buildSummary(ctx, run)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

func (s *AgentService) PoolStats(_ context.Context) model.AgentPoolStatsDTO {
	if s.pool == nil {
		return model.AgentPoolStatsDTO{}
	}

	pausedResumable := 0
	if runs, err := s.runRepo.ListActive(context.Background()); err == nil {
		for _, run := range runs {
			if run.Status == model.AgentRunStatusPaused {
				pausedResumable++
			}
		}
	}

	stats := model.AgentPoolStatsDTO{
		Active:          s.pool.ActiveCount(),
		Max:             s.pool.Available() + s.pool.ActiveCount(),
		Available:       s.pool.Available(),
		PausedResumable: pausedResumable,
	}
	if s.queueStore != nil {
		if allQueued, err := s.queueStore.ListAllQueued(context.Background(), 10); err == nil {
			stats.Queued = len(allQueued)
			for _, entry := range allQueued {
				if entry != nil {
					stats.Queue = append(stats.Queue, *entry)
				}
			}
		}
	}
	if s.bridge != nil {
		if summary, err := s.bridge.GetPoolSummary(context.Background()); err == nil && summary != nil {
			stats.Warm = summary.WarmTotal
			stats.Degraded = summary.Degraded
		} else if err != nil {
			stats.Degraded = true
		}
	}
	if s.queueStore != nil && s.taskRepo != nil {
		projectIDs := make(map[uuid.UUID]struct{})
		if runs, err := s.runRepo.ListActive(context.Background()); err == nil {
			for _, run := range runs {
				task, taskErr := s.taskRepo.GetByID(context.Background(), run.TaskID)
				if taskErr == nil {
					projectIDs[task.ProjectID] = struct{}{}
				}
			}
		}
		for projectID := range projectIDs {
			if count, err := s.queueStore.CountQueuedByProject(context.Background(), projectID); err == nil {
				stats.Queued += count
			}
			if queue, err := s.queueStore.ListQueuedByProject(context.Background(), projectID, 10); err == nil {
				for _, entry := range queue {
					if entry != nil {
						stats.Queue = append(stats.Queue, *entry)
					}
				}
			}
		}
	}
	return stats
}

func (s *AgentService) QueueAgentAdmission(ctx context.Context, input QueueAgentAdmissionInput) (*model.AgentPoolQueueEntry, error) {
	if s.queueStore == nil {
		return nil, ErrAgentPoolFull
	}
	return s.queueStore.QueueAgentAdmission(ctx, input)
}

func (s *AgentService) RequestSpawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.TaskDispatchResponse, error) {
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
	selection, err := ResolveProjectCodingAgentSelection(project, runtime, provider, modelName)
	if err != nil {
		return nil, err
	}

	resolvedRoleID := strings.TrimSpace(roleID)
	if _, err := s.resolveRoleConfig(resolvedRoleID); err != nil {
		return nil, err
	}

	controller := pool.NewAdmissionController(s.pool, agentServiceQueueAdapter{store: s.queueStore})
	decision, err := controller.Decide(ctx, pool.QueueAdmissionInput{
		ProjectID: task.ProjectID,
		TaskID:    taskID,
		MemberID:  memberID,
		Runtime:   selection.Runtime,
		Provider:  selection.Provider,
		Model:     selection.Model,
		RoleID:    resolvedRoleID,
		BudgetUSD: budgetUsd,
		Reason:    "agent pool is at capacity",
	})
	if err != nil {
		return nil, err
	}
	if decision.Status == pool.AdmissionStatusQueued {
		return &model.TaskDispatchResponse{
			Task: task.ToDTO(),
			Dispatch: model.DispatchOutcome{
				Status: model.DispatchStatusQueued,
				Reason: decision.Reason,
				Queue:  decision.Queue,
			},
		}, nil
	}
	if decision.Status == pool.AdmissionStatusBlocked {
		return &model.TaskDispatchResponse{
			Task: task.ToDTO(),
			Dispatch: model.DispatchOutcome{
				Status: model.DispatchStatusBlocked,
				Reason: decision.Reason,
			},
		}, nil
	}

	run, err := s.Spawn(ctx, taskID, memberID, runtime, provider, modelName, budgetUsd, roleID)
	if err != nil {
		if errors.Is(err, ErrAgentPoolFull) && s.queueStore != nil {
			entry, queueErr := s.queueStore.QueueAgentAdmission(ctx, QueueAgentAdmissionInput{
				ProjectID: task.ProjectID,
				TaskID:    taskID,
				MemberID:  memberID,
				Runtime:   selection.Runtime,
				Provider:  selection.Provider,
				Model:     selection.Model,
				RoleID:    resolvedRoleID,
				BudgetUSD: budgetUsd,
				Reason:    "agent pool is at capacity",
			})
			if queueErr == nil {
				return &model.TaskDispatchResponse{
					Task: task.ToDTO(),
					Dispatch: model.DispatchOutcome{
						Status: model.DispatchStatusQueued,
						Reason: "agent pool is at capacity",
						Queue:  entry,
					},
				}, nil
			}
		}
		return nil, err
	}
	return &model.TaskDispatchResponse{
		Task: task.ToDTO(),
		Dispatch: model.DispatchOutcome{
			Status: model.DispatchStatusStarted,
			Run:    dtoPtr(run.ToDTO()),
		},
	}, nil
}

func (s *AgentService) failSpawn(ctx context.Context, run *model.AgentRun, task *model.Task, projectSlug string, allocation *worktreepkg.Allocation) error {
	if err := s.runRepo.UpdateStatus(ctx, run.ID, model.AgentRunStatusFailed); err != nil {
		return err
	}
	fields := agentRunLogFields(run)
	fields["projectSlug"] = projectSlug
	if task != nil {
		fields["projectId"] = task.ProjectID.String()
	}
	if allocation != nil {
		fields["worktreePath"] = allocation.Path
		fields["branchName"] = allocation.Branch
		fields["worktreeReused"] = allocation.Reused
	}
	log.WithFields(fields).Warn("agent spawn marked failed")
	s.releasePoolSlot(run.ID.String())
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
	fields := log.Fields{
		"taskId":       taskID.String(),
		"projectId":    task.ProjectID.String(),
		"branchName":   task.AgentBranch,
		"worktreePath": task.AgentWorktree,
		"sessionId":    task.AgentSessionID,
	}

	if s.worktrees != nil && s.projects != nil {
		project, err := s.projects.GetByID(ctx, task.ProjectID)
		if err != nil {
			return err
		}
		fields["projectSlug"] = project.Slug
		canonicalBranch := s.worktrees.Branch(taskID.String())
		canonicalPath := s.worktrees.Path(project.Slug, taskID.String())
		if task.AgentBranch == canonicalBranch && task.AgentWorktree == canonicalPath {
			if err := s.worktrees.Release(ctx, project.Slug, taskID.String()); err != nil {
				return err
			}
		}
	}

	log.WithFields(fields).Info("agent task runtime cleared")
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

func (s *AgentService) pauseRun(ctx context.Context, run *model.AgentRun) error {
	task, err := s.taskRepo.GetByID(ctx, run.TaskID)
	if err != nil {
		return err
	}
	resp, err := s.bridge.Pause(ctx, run.TaskID.String(), "paused_by_user")
	if err != nil {
		return fmt.Errorf("pause bridge runtime: %w", err)
	}
	sessionID := task.AgentSessionID
	if resp != nil && strings.TrimSpace(resp.SessionID) != "" {
		sessionID = resp.SessionID
	}
	if err := s.taskRepo.UpdateRuntime(ctx, task.ID, task.AgentBranch, task.AgentWorktree, sessionID); err != nil {
		return fmt.Errorf("persist paused runtime: %w", err)
	}
	if err := s.runRepo.UpdateStatus(ctx, run.ID, model.AgentRunStatusPaused); err != nil {
		return fmt.Errorf("mark run paused: %w", err)
	}
	run.Status = model.AgentRunStatusPaused
	log.WithFields(agentRunLogFields(run)).WithField("sessionId", sessionID).Info("agent runtime paused")
	s.releasePoolSlot(run.ID.String())
	s.broadcastEvent(ws.EventAgentProgress, s.lookupProjectID(ctx, run.TaskID), run.ToDTO())
	return nil
}

func (s *AgentService) resumeRun(ctx context.Context, run *model.AgentRun) error {
	if s.pool != nil {
		if err := s.pool.Acquire(run.ID.String(), run.TaskID.String(), run.MemberID.String()); err != nil {
			if errors.Is(err, pool.ErrPoolFull) {
				return ErrAgentPoolFull
			}
			return fmt.Errorf("acquire agent pool slot: %w", err)
		}
	}
	defer func() {
		if run.Status != model.AgentRunStatusRunning {
			s.releasePoolSlot(run.ID.String())
		}
	}()

	task, err := s.taskRepo.GetByID(ctx, run.TaskID)
	if err != nil {
		return err
	}
	roleConfig, err := s.resolveRoleConfigForResume(strings.TrimSpace(run.RoleID))
	if err != nil {
		return err
	}
	req := BridgeExecuteRequest{
		TaskID:         run.TaskID.String(),
		SessionID:      task.AgentSessionID,
		MemberID:       run.MemberID.String(),
		Runtime:        resolveStoredRuntime(run),
		Provider:       run.Provider,
		Model:          run.Model,
		Prompt:         buildSpawnPrompt(task),
		WorktreePath:   task.AgentWorktree,
		BranchName:     task.AgentBranch,
		MaxTurns:       resolveSpawnMaxTurns(roleConfig),
		BudgetUSD:      resolveSpawnBudget(task.BudgetUsd, 0, roleConfig),
		AllowedTools:   resolveSpawnAllowedTools(roleConfig),
		PermissionMode: resolveSpawnPermissionMode(roleConfig),
		RoleConfig:     roleConfig,
	}
	resp, err := s.bridge.Resume(ctx, req)
	if err != nil {
		return fmt.Errorf("resume bridge runtime: %w", err)
	}
	sessionID := task.AgentSessionID
	if resp != nil && strings.TrimSpace(resp.SessionID) != "" {
		sessionID = resp.SessionID
	}
	if err := s.taskRepo.UpdateRuntime(ctx, task.ID, task.AgentBranch, task.AgentWorktree, sessionID); err != nil {
		return fmt.Errorf("persist resumed runtime: %w", err)
	}
	if err := s.runRepo.UpdateStatus(ctx, run.ID, model.AgentRunStatusRunning); err != nil {
		return fmt.Errorf("mark run resumed: %w", err)
	}
	run.Status = model.AgentRunStatusRunning
	log.WithFields(agentRunLogFields(run)).WithField("sessionId", sessionID).Info("agent runtime resumed")
	s.broadcastEvent(ws.EventAgentProgress, s.lookupProjectID(ctx, run.TaskID), run.ToDTO())
	return nil
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
	if _, err := s.progress.RecordActivity(ctx, taskID, input); err != nil {
		log.WithFields(log.Fields{
			"taskId":         taskID.String(),
			"source":         input.Source,
			"occurredAt":     input.OccurredAt.Format(time.RFC3339),
			"updateHealth":   input.UpdateHealth,
			"markTransition": input.MarkTransition,
		}).WithError(err).Warn("agent progress recording failed")
	}
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

func (s *AgentService) buildSummary(ctx context.Context, run *model.AgentRun) (model.AgentRunSummaryDTO, error) {
	task, err := s.taskRepo.GetByID(ctx, run.TaskID)
	if err != nil {
		return model.AgentRunSummaryDTO{}, err
	}
	roleName := run.RoleID
	if run.RoleID == "" {
		roleName = "Unassigned Role"
	} else if s.roleStore != nil {
		if manifest, roleErr := s.roleStore.Get(run.RoleID); roleErr == nil && strings.TrimSpace(manifest.Metadata.Name) != "" {
			roleName = manifest.Metadata.Name
		}
	}

	lastActivity := run.CreatedAt
	if !run.UpdatedAt.IsZero() {
		lastActivity = run.UpdatedAt
	} else if !run.StartedAt.IsZero() {
		lastActivity = run.StartedAt
	}

	dto := model.AgentRunSummaryDTO{
		ID:              run.ID.String(),
		TaskID:          run.TaskID.String(),
		TaskTitle:       task.Title,
		MemberID:        run.MemberID.String(),
		RoleID:          run.RoleID,
		RoleName:        roleName,
		Status:          run.Status,
		Runtime:         run.Runtime,
		Provider:        run.Provider,
		Model:           run.Model,
		InputTokens:     run.InputTokens,
		OutputTokens:    run.OutputTokens,
		CacheReadTokens: run.CacheReadTokens,
		CostUsd:         run.CostUsd,
		BudgetUsd:       task.BudgetUsd,
		TurnCount:       run.TurnCount,
		WorktreePath:    task.AgentWorktree,
		BranchName:      task.AgentBranch,
		SessionID:       task.AgentSessionID,
		LastActivityAt:  lastActivity.Format(time.RFC3339),
		StartedAt:       run.StartedAt.Format(time.RFC3339),
		CreatedAt:       run.CreatedAt.Format(time.RFC3339),
		CanResume:       task.AgentSessionID != "" && run.Status != model.AgentRunStatusRunning && run.Status != model.AgentRunStatusStarting,
		MemoryStatus:    deriveMemoryStatus(task.AgentSessionID),
		TeamRole:        run.TeamRole,
	}
	if run.TeamID != nil {
		s := run.TeamID.String()
		dto.TeamID = &s
	}
	if run.CompletedAt != nil {
		completedAt := run.CompletedAt.Format(time.RFC3339)
		dto.CompletedAt = &completedAt
	}
	return dto, nil
}

func resolveStoredRuntime(run *model.AgentRun) string {
	if run == nil {
		return ""
	}
	if normalized := normalizeRuntime(run.Runtime); normalized != "" {
		return normalized
	}
	switch normalizeProvider(run.Provider) {
	case "", "anthropic":
		return model.DefaultCodingAgentRuntime
	case "codex":
		return "codex"
	case "opencode":
		return "opencode"
	default:
		return ""
	}
}

func deriveMemoryStatus(sessionID string) string {
	if strings.TrimSpace(sessionID) == "" {
		return "none"
	}
	return "available"
}

func (s *AgentService) releasePoolSlot(runID string) {
	if s.pool == nil {
		return
	}
	_ = s.pool.Release(runID)
}

func (s *AgentService) broadcastPoolStats(ctx context.Context, projectID string) {
	if projectID == "" {
		return
	}
	s.broadcastEvent(ws.EventAgentPoolUpdated, projectID, s.PoolStats(ctx))
}

func (s *AgentService) promoteQueuedAdmission(ctx context.Context, completedRun *model.AgentRun) {
	if s.queueStore == nil || completedRun == nil || s.taskRepo == nil {
		return
	}

	completedTask, err := s.taskRepo.GetByID(ctx, completedRun.TaskID)
	if err != nil || completedTask == nil {
		return
	}

	entry, err := s.queueStore.ReserveNextQueuedByProject(ctx, completedTask.ProjectID)
	if err != nil || entry == nil {
		return
	}

	taskID, err := uuid.Parse(entry.TaskID)
	if err != nil {
		_ = s.queueStore.CompleteQueuedEntry(ctx, entry.EntryID, model.AgentPoolQueueStatusFailed, "invalid queued task id", nil)
		return
	}
	memberID, err := uuid.Parse(entry.MemberID)
	if err != nil {
		_ = s.queueStore.CompleteQueuedEntry(ctx, entry.EntryID, model.AgentPoolQueueStatusFailed, "invalid queued member id", nil)
		return
	}

	run, err := s.Spawn(ctx, taskID, memberID, entry.Runtime, entry.Provider, entry.Model, entry.BudgetUSD, entry.RoleID)
	if err != nil {
		log.WithFields(log.Fields{
			"completedRunId": completedRun.ID.String(),
			"queueEntryId":   entry.EntryID,
			"projectId":      completedTask.ProjectID.String(),
			"taskId":         entry.TaskID,
			"memberId":       entry.MemberID,
			"runtime":        entry.Runtime,
			"provider":       entry.Provider,
			"model":          entry.Model,
		}).WithError(err).Warn("queued agent admission promotion failed")
		s.broadcastEvent(ws.EventAgentQueueFailed, completedTask.ProjectID.String(), map[string]any{
			"queue": entry,
			"error": err.Error(),
		})
		_ = s.queueStore.CompleteQueuedEntry(ctx, entry.EntryID, model.AgentPoolQueueStatusFailed, err.Error(), nil)
		return
	}
	log.WithFields(log.Fields{
		"completedRunId": completedRun.ID.String(),
		"queueEntryId":   entry.EntryID,
		"projectId":      completedTask.ProjectID.String(),
		"taskId":         entry.TaskID,
		"promotedRunId":  run.ID.String(),
	}).Info("queued agent admission promoted")
	s.broadcastEvent(ws.EventAgentQueuePromoted, completedTask.ProjectID.String(), map[string]any{
		"queue": entry,
		"run":   run.ToDTO(),
	})
	_ = s.queueStore.CompleteQueuedEntry(ctx, entry.EntryID, model.AgentPoolQueueStatusPromoted, "started", &run.ID)
	s.broadcastPoolStats(ctx, completedTask.ProjectID.String())
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

func (s *AgentService) resolveRoleConfigForResume(roleID string) (*bridgeclient.RoleConfig, error) {
	roleConfig, err := s.resolveRoleConfig(roleID)
	if errors.Is(err, ErrAgentRoleNotFound) {
		return nil, nil
	}
	return roleConfig, err
}

func (s *AgentService) ProcessBridgeEvent(ctx context.Context, event *ws.BridgeAgentEvent) error {
	if event == nil || strings.TrimSpace(event.TaskID) == "" {
		return nil
	}

	run, err := s.resolveRunForBridgeEvent(ctx, event.TaskID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			return nil
		}
		return err
	}
	eventFields := agentRunLogFields(run)
	eventFields["bridgeTaskId"] = event.TaskID
	eventFields["sessionId"] = event.SessionID
	eventFields["eventType"] = event.Type
	eventFields["timestampMs"] = event.TimestampMS

	switch event.Type {
	case ws.BridgeEventOutput:
		var payload ws.BridgeOutputData
		if err := event.DecodeData(&payload); err != nil {
			return fmt.Errorf("decode bridge output payload: %w", err)
		}
		if strings.TrimSpace(payload.Content) == "" {
			return nil
		}
		eventFields["turnNumber"] = payload.TurnNumber
		eventFields["contentType"] = payload.ContentType
		log.WithFields(eventFields).Debug("bridge output event received")
		s.broadcastEvent(ws.EventAgentOutput, s.lookupProjectID(ctx, run.TaskID), map[string]any{
			"agent_id":     run.ID.String(),
			"task_id":      run.TaskID.String(),
			"session_id":   event.SessionID,
			"line":         payload.Content,
			"content_type": payload.ContentType,
			"turn_number":  payload.TurnNumber,
			"timestamp":    time.UnixMilli(event.TimestampMS).UTC().Format(time.RFC3339),
		})
		s.recordProgress(ctx, run.TaskID, TaskActivityInput{
			Source:       model.TaskProgressSourceAgentHeartbeat,
			OccurredAt:   bridgeEventTime(event.TimestampMS),
			UpdateHealth: true,
		})
		s.notifyIMRunUpdate(ctx, run, summarizeBridgeOutput(payload.Content, payload.TurnNumber), false)
		return nil

	case ws.BridgeEventCostUpdate:
		var payload ws.BridgeCostUpdateData
		if err := event.DecodeData(&payload); err != nil {
			return fmt.Errorf("decode bridge cost payload: %w", err)
		}
		if isTerminalAgentStatus(run.Status) {
			return nil
		}
		eventFields["turnNumber"] = payload.TurnNumber
		eventFields["reportedCostUsd"] = payload.CostUSD
		log.WithFields(eventFields).Debug("bridge cost event received")
		return s.UpdateCost(ctx, run.ID, payload.InputTokens, payload.OutputTokens, payload.CacheReadTokens, payload.CostUSD, payload.TurnNumber)

	case ws.BridgeEventStatusChange:
		var payload ws.BridgeStatusChangeData
		if err := event.DecodeData(&payload); err != nil {
			return fmt.Errorf("decode bridge status payload: %w", err)
		}
		nextStatus, ok := mapBridgeRuntimeStatus(payload.NewStatus)
		if !ok || !isTerminalAgentStatus(nextStatus) || run.Status == nextStatus {
			return nil
		}
		eventFields["oldStatus"] = payload.OldStatus
		eventFields["newStatus"] = payload.NewStatus
		log.WithFields(eventFields).Info("bridge terminal status event received")
		return s.UpdateStatus(ctx, run.ID, nextStatus)
	}

	return nil
}

func (s *AgentService) resolveRunForBridgeEvent(ctx context.Context, taskID string) (*model.AgentRun, error) {
	parsedTaskID, err := uuid.Parse(taskID)
	if err != nil {
		return nil, fmt.Errorf("parse bridge task id %q: %w", taskID, err)
	}

	runs, err := s.runRepo.GetByTask(ctx, parsedTaskID)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, ErrAgentNotFound
	}

	activeStatuses := []string{
		model.AgentRunStatusStarting,
		model.AgentRunStatusRunning,
		model.AgentRunStatusPaused,
	}
	for _, run := range runs {
		if run == nil {
			continue
		}
		if slices.Contains(activeStatuses, run.Status) {
			return run, nil
		}
	}
	return runs[0], nil
}

func mapBridgeRuntimeStatus(status string) (string, bool) {
	switch strings.TrimSpace(status) {
	case model.AgentRunStatusCompleted:
		return model.AgentRunStatusCompleted, true
	case model.AgentRunStatusFailed:
		return model.AgentRunStatusFailed, true
	case model.AgentRunStatusCancelled:
		return model.AgentRunStatusCancelled, true
	case model.AgentRunStatusBudgetExceeded:
		return model.AgentRunStatusBudgetExceeded, true
	default:
		return "", false
	}
}

func bridgeEventTime(timestampMS int64) time.Time {
	if timestampMS <= 0 {
		return time.Now().UTC()
	}
	return time.UnixMilli(timestampMS).UTC()
}

func (s *AgentService) notifyIMRunUpdate(ctx context.Context, run *model.AgentRun, content string, terminal bool) {
	if s.imProgress == nil || run == nil || strings.TrimSpace(content) == "" {
		return
	}
	queued, err := s.imProgress.QueueBoundProgress(ctx, IMBoundProgressRequest{
		TaskID:     run.TaskID.String(),
		RunID:      run.ID.String(),
		Kind:       IMDeliveryKindProgress,
		Content:    content,
		IsTerminal: terminal,
	})
	fields := agentRunLogFields(run)
	fields["terminal"] = terminal
	fields["queued"] = queued
	if err != nil {
		log.WithFields(fields).WithError(err).Warn("agent IM progress notification failed")
		return
	}
	if queued {
		log.WithFields(fields).Debug("agent IM progress notification queued")
	}
}

func summarizeBridgeOutput(content string, turnNumber int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return ""
	}
	if turnNumber > 0 {
		return fmt.Sprintf("Agent 正在运行，第 %d 轮已有新输出。\n%s", turnNumber, trimmed)
	}
	return fmt.Sprintf("Agent 正在运行，已有新的执行输出。\n%s", trimmed)
}

func nextAgentRunSummary(status string, taskID string, runID string) string {
	switch status {
	case model.AgentRunStatusCompleted:
		return fmt.Sprintf("Agent 运行完成。\nTask: %s\nRun: %s", taskID, runID)
	case model.AgentRunStatusCancelled:
		return fmt.Sprintf("Agent 运行已取消。\nTask: %s\nRun: %s", taskID, runID)
	case model.AgentRunStatusBudgetExceeded:
		return fmt.Sprintf("Agent 运行因预算超限停止。\nTask: %s\nRun: %s", taskID, runID)
	case model.AgentRunStatusFailed:
		return fmt.Sprintf("Agent 运行失败。\nTask: %s\nRun: %s", taskID, runID)
	default:
		return fmt.Sprintf("Agent 状态已更新为 %s。\nTask: %s\nRun: %s", status, taskID, runID)
	}
}
