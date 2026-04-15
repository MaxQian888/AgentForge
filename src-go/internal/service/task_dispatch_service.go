package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/ws"
)

var (
	ErrDispatchMemberNotFound = errors.New("dispatch member not found")
)

type DispatchTaskRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	UpdateAssignee(ctx context.Context, id uuid.UUID, assigneeID uuid.UUID, assigneeType string) error
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
}

type DispatchMemberRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.Member, error)
}

type DispatchRuntimeService interface {
	Spawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string) (*model.AgentRun, error)
}

type DispatchQueueWriter interface {
	QueueAgentAdmission(ctx context.Context, input QueueAgentAdmissionInput) (*model.AgentPoolQueueEntry, error)
}

type DispatchBudgetChecker interface {
	CheckBudget(ctx context.Context, projectID uuid.UUID, sprintID *uuid.UUID, requestedUsd float64) (*BudgetCheckResult, error)
}

type runtimeAdmissionSpawner interface {
	RequestSpawn(ctx context.Context, taskID, memberID uuid.UUID, runtime, provider, modelName string, budgetUsd float64, roleID string, priority int) (*model.TaskDispatchResponse, error)
}

type DispatchNotificationService interface {
	Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error)
}

type DispatchAttemptRecorder interface {
	Create(ctx context.Context, attempt *model.DispatchAttempt) error
}

type DispatchSpawnInput struct {
	TaskID        uuid.UUID
	MemberID      *uuid.UUID
	Runtime       string
	Provider      string
	Model         string
	Priority      int
	BudgetUSD     float64
	RoleID        string
	TriggerSource string
}

type TaskDispatchService struct {
	tasks         DispatchTaskRepository
	members       DispatchMemberRepository
	runtime       DispatchRuntimeService
	roleStore     roleReferenceRoleStore
	hub           *ws.Hub
	notifications DispatchNotificationService
	progress      *TaskProgressService
	queueWriter   DispatchQueueWriter
	budgetChecker DispatchBudgetChecker
	attempts      DispatchAttemptRecorder
}

func NewTaskDispatchService(
	tasks DispatchTaskRepository,
	members DispatchMemberRepository,
	runtime DispatchRuntimeService,
	hub *ws.Hub,
	notifications DispatchNotificationService,
	progress *TaskProgressService,
) *TaskDispatchService {
	return &TaskDispatchService{
		tasks:         tasks,
		members:       members,
		runtime:       runtime,
		hub:           hub,
		notifications: notifications,
		progress:      progress,
	}
}

func (s *TaskDispatchService) WithQueueWriter(queueWriter DispatchQueueWriter) *TaskDispatchService {
	s.queueWriter = queueWriter
	return s
}

func (s *TaskDispatchService) WithBudgetChecker(checker DispatchBudgetChecker) *TaskDispatchService {
	s.budgetChecker = checker
	return s
}

func (s *TaskDispatchService) WithAttemptRecorder(attempts DispatchAttemptRecorder) *TaskDispatchService {
	s.attempts = attempts
	return s
}

func (s *TaskDispatchService) WithRoleStore(store roleReferenceRoleStore) *TaskDispatchService {
	s.roleStore = store
	return s
}

func (s *TaskDispatchService) Assign(ctx context.Context, taskID uuid.UUID, req *model.AssignRequest) (*model.TaskDispatchResponse, error) {
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, ErrAgentTaskNotFound
	}

	memberID, err := uuid.Parse(req.AssigneeID)
	if err != nil {
		return nil, fmt.Errorf("invalid assignee id: %w", err)
	}
	member, err := s.members.GetByID(ctx, memberID)
	if err != nil {
		result := s.blockedResult(ctx, task, "dispatch target is unavailable")
		s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, resolveDispatchTriggerSource(req.TriggerSource, "assignment"))
		return result, nil
	}
	if member.ProjectID != task.ProjectID {
		result := s.blockedResult(ctx, task, "dispatch target is outside the task project")
		s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, resolveDispatchTriggerSource(req.TriggerSource, "assignment"))
		return result, nil
	}
	if req.AssigneeType == model.MemberTypeAgent && (member.Type != model.MemberTypeAgent || !member.IsActive) {
		result := s.blockedResult(ctx, task, "dispatch target is not an active agent member")
		s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, resolveDispatchTriggerSource(req.TriggerSource, "assignment"))
		return result, nil
	}
	if req.AssigneeType == model.MemberTypeHuman && member.Type != model.MemberTypeHuman {
		result := s.blockedResult(ctx, task, "dispatch target does not match the requested assignee type")
		s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, resolveDispatchTriggerSource(req.TriggerSource, "assignment"))
		return result, nil
	}

	if err := s.tasks.UpdateAssignee(ctx, taskID, memberID, member.Type); err != nil {
		return nil, fmt.Errorf("assign task: %w", err)
	}
	if task.Status != model.TaskStatusAssigned && model.ValidateTransition(task.Status, model.TaskStatusAssigned) == nil {
		if err := s.tasks.TransitionStatus(ctx, taskID, model.TaskStatusAssigned); err != nil {
			return nil, fmt.Errorf("transition task to assigned: %w", err)
		}
	}

	updatedTask, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("fetch assigned task: %w", err)
	}
	s.broadcastTaskAssigned(updatedTask)
	s.recordProgress(ctx, updatedTask.ID, TaskActivityInput{
		Source:       model.TaskProgressSourceTaskAssigned,
		UpdateHealth: true,
	})

	if member.Type != model.MemberTypeAgent {
		result := &model.TaskDispatchResponse{
			Task: updatedTask.ToDTO(),
			Dispatch: model.DispatchOutcome{
				Status: model.DispatchStatusSkipped,
				Reason: "task assigned to a human member",
			},
		}
		s.recordDispatchAttempt(ctx, updatedTask, &memberID, result.Dispatch, resolveDispatchTriggerSource(req.TriggerSource, "assignment"))
		return result, nil
	}

	return s.spawnForTask(ctx, updatedTask, memberID, DispatchSpawnInput{TriggerSource: resolveDispatchTriggerSource(req.TriggerSource, "assignment")})
}

func (s *TaskDispatchService) Spawn(ctx context.Context, input DispatchSpawnInput) (*model.TaskDispatchResponse, error) {
	task, err := s.tasks.GetByID(ctx, input.TaskID)
	if err != nil {
		return nil, ErrAgentTaskNotFound
	}

	var memberID uuid.UUID
	if input.MemberID != nil {
		memberID = *input.MemberID
	} else {
		if task.AssigneeID == nil || task.AssigneeType != model.MemberTypeAgent {
			result := s.blockedResult(ctx, task, "task has no valid assigned agent member")
			s.recordDispatchAttempt(ctx, task, nil, result.Dispatch, resolveDispatchTriggerSource(input.TriggerSource, "manual"))
			return result, nil
		}
		memberID = *task.AssigneeID
	}

	member, err := s.members.GetByID(ctx, memberID)
	if err != nil {
		result := s.blockedResult(ctx, task, "dispatch target is unavailable")
		s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, resolveDispatchTriggerSource(input.TriggerSource, "manual"))
		return result, nil
	}
	if member.ProjectID != task.ProjectID || member.Type != model.MemberTypeAgent || !member.IsActive {
		result := s.blockedResult(ctx, task, "task has no valid assigned agent member")
		s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, resolveDispatchTriggerSource(input.TriggerSource, "manual"))
		return result, nil
	}

	input.TriggerSource = resolveDispatchTriggerSource(input.TriggerSource, "manual")
	return s.spawnForTask(ctx, task, memberID, input)
}

func (s *TaskDispatchService) spawnForTask(ctx context.Context, task *model.Task, memberID uuid.UUID, input DispatchSpawnInput) (*model.TaskDispatchResponse, error) {
	member, memberErr := s.members.GetByID(ctx, memberID)
	if memberErr != nil {
		contextFields := dispatchOutcomeContextFromInput(input)
		result := s.blockedResult(ctx, task, "dispatch target is unavailable")
		result.Dispatch = applyDispatchOutcomeContext(result.Dispatch, contextFields)
		s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
		return result, nil
	}
	input.RoleID = ResolveEffectiveRoleID(input.RoleID, member)
	contextFields := dispatchOutcomeContextFromInput(input)
	if s.roleStore != nil {
		if err := NewRoleReferenceGovernanceService(nil, nil, nil, nil).
			WithRoleStore(s.roleStore).
			ValidateRoleBinding(ctx, input.RoleID); err != nil {
			result := s.blockedResultWithGuardrail(ctx, task, err.Error(), model.DispatchGuardrailTypeTarget, "role")
			result.Dispatch = applyDispatchOutcomeContext(result.Dispatch, contextFields)
			s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
			return result, nil
		}
	}
	var runReader DispatchRunReader
	if reader, ok := s.runtime.(DispatchRunReader); ok {
		runReader = reader
	}
	var poolProvider DispatchPoolStatsProvider
	if provider, ok := s.runtime.(DispatchPoolStatsProvider); ok {
		poolProvider = provider
	}
	preflight := EvaluateDispatchPreflight(ctx, task, member, input, s.budgetChecker, runReader, poolProvider)
	if preflight.Outcome.Status == model.DispatchStatusBlocked || preflight.Outcome.Status == model.DispatchStatusSkipped {
		result := &model.TaskDispatchResponse{
			Task:     task.ToDTO(),
			Dispatch: applyDispatchOutcomeContext(preflight.Outcome, contextFields),
		}
		s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
		return result, nil
	}
	warning := preflight.Outcome.BudgetWarning
	if admissionSpawner, ok := s.runtime.(runtimeAdmissionSpawner); ok {
		result, err := admissionSpawner.RequestSpawn(ctx, task.ID, memberID, input.Runtime, input.Provider, input.Model, input.BudgetUSD, input.RoleID, input.Priority)
		if err != nil {
			return nil, err
		}
		if warning != nil && result != nil {
			result.Dispatch.BudgetWarning = warning
			s.broadcastBudgetWarning(task, warning)
		}
		if result != nil {
			result.Dispatch = applyDispatchOutcomeContext(result.Dispatch, contextFields)
			s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
		}
		return result, nil
	}

	run, err := s.runtime.Spawn(ctx, task.ID, memberID, input.Runtime, input.Provider, input.Model, input.BudgetUSD, input.RoleID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAgentAlreadyRunning):
			result := s.blockedResult(ctx, task, "task already has an active agent run")
			result.Dispatch = applyDispatchOutcomeContext(result.Dispatch, contextFields)
			s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
			return result, nil
		case errors.Is(err, ErrAgentPoolFull):
			if s.queueWriter == nil {
				result := s.blockedResult(ctx, task, "agent pool is at capacity")
				result.Dispatch = applyDispatchOutcomeContext(result.Dispatch, contextFields)
				s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
				return result, nil
			}
			entry, queueErr := s.queueWriter.QueueAgentAdmission(ctx, QueueAgentAdmissionInput{
				ProjectID: task.ProjectID,
				TaskID:    task.ID,
				MemberID:  memberID,
				Runtime:   input.Runtime,
				Provider:  input.Provider,
				Model:     input.Model,
				RoleID:    input.RoleID,
				Priority:  input.Priority,
				BudgetUSD: input.BudgetUSD,
				Reason:    "agent pool is at capacity",
				GuardrailType:       model.DispatchGuardrailTypePool,
				GuardrailScope:      "project",
				RecoveryDisposition: model.QueueRecoveryDispositionPending,
			})
			if queueErr != nil {
				result := s.blockedResult(ctx, task, "agent pool is at capacity")
				result.Dispatch = applyDispatchOutcomeContext(result.Dispatch, contextFields)
				s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
				return result, nil
			}
			result := s.queuedResult(task, entry, "agent pool is at capacity")
			if warning != nil {
				result.Dispatch.BudgetWarning = warning
				s.broadcastBudgetWarning(task, warning)
			}
			s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
			return result, nil
		case errors.Is(err, ErrAgentWorktreeUnavailable):
			result := s.blockedResult(ctx, task, "agent dispatch is blocked by worktree availability")
			result.Dispatch = applyDispatchOutcomeContext(result.Dispatch, contextFields)
			s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
			return result, nil
		default:
			result := s.blockedResult(ctx, task, err.Error())
			result.Dispatch = applyDispatchOutcomeContext(result.Dispatch, contextFields)
			s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
			return result, nil
		}
	}

	updatedTask, fetchErr := s.tasks.GetByID(ctx, task.ID)
	if fetchErr != nil {
		updatedTask = task
	}

	if warning != nil {
		s.broadcastBudgetWarning(task, warning)
	}

	result := &model.TaskDispatchResponse{
		Task: updatedTask.ToDTO(),
		Dispatch: model.DispatchOutcome{
			Status:        model.DispatchStatusStarted,
			Runtime:       contextFields.Runtime,
			Provider:      contextFields.Provider,
			Model:         contextFields.Model,
			RoleID:        contextFields.RoleID,
			BudgetWarning: warning,
			Run:           dtoPtr(run.ToDTO()),
		},
	}
	s.recordDispatchAttempt(ctx, task, &memberID, result.Dispatch, input.TriggerSource)
	return result, nil
}

func (s *TaskDispatchService) recordDispatchAttempt(ctx context.Context, task *model.Task, memberID *uuid.UUID, dispatch model.DispatchOutcome, triggerSource string) {
	if s.attempts == nil || task == nil {
		return
	}
	_ = s.attempts.Create(ctx, &model.DispatchAttempt{
		ID:             uuid.New(),
		ProjectID:      task.ProjectID,
		TaskID:         task.ID,
		MemberID:       memberID,
		Outcome:        dispatch.Status,
		TriggerSource:  resolveDispatchTriggerSource(triggerSource, "manual"),
		Reason:         dispatch.Reason,
		Runtime:        dispatch.Runtime,
		Provider:       dispatch.Provider,
		Model:          dispatch.Model,
		RoleID:         dispatch.RoleID,
		QueueEntryID:   queueEntryIDFromDispatch(dispatch),
		QueuePriority:  queuePriorityFromDispatch(dispatch),
		GuardrailType:  dispatch.GuardrailType,
		GuardrailScope: dispatch.GuardrailScope,
		CreatedAt:      time.Now().UTC(),
	})
}

func resolveDispatchTriggerSource(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func (s *TaskDispatchService) blockedResult(ctx context.Context, task *model.Task, reason string) *model.TaskDispatchResponse {
	guardrailType, guardrailScope := inferDispatchGuardrail(reason)
	return s.blockedResultWithGuardrail(ctx, task, reason, guardrailType, guardrailScope)
}

func (s *TaskDispatchService) blockedResultWithGuardrail(ctx context.Context, task *model.Task, reason string, guardrailType string, guardrailScope string) *model.TaskDispatchResponse {
	if task != nil {
		s.broadcastDispatchBlocked(task, reason, guardrailType, guardrailScope)
		s.createBlockedNotification(ctx, task, reason)
	}
	taskDTO := model.TaskDTO{}
	if task != nil {
		taskDTO = task.ToDTO()
	}
	return &model.TaskDispatchResponse{
		Task: taskDTO,
		Dispatch: model.DispatchOutcome{
			Status:         model.DispatchStatusBlocked,
			Reason:         reason,
			GuardrailType:  guardrailType,
			GuardrailScope: guardrailScope,
		},
	}
}

func (s *TaskDispatchService) queuedResult(task *model.Task, entry *model.AgentPoolQueueEntry, reason string) *model.TaskDispatchResponse {
	if task != nil {
		s.broadcastDispatchQueued(task, entry, reason)
	}
	taskDTO := model.TaskDTO{}
	if task != nil {
		taskDTO = task.ToDTO()
	}
	return &model.TaskDispatchResponse{
		Task: taskDTO,
		Dispatch: model.DispatchOutcome{
			Status:         model.DispatchStatusQueued,
			Reason:         reason,
			Runtime:        entry.Runtime,
			Provider:       entry.Provider,
			Model:          entry.Model,
			RoleID:         entry.RoleID,
			GuardrailType:  entry.GuardrailType,
			GuardrailScope: entry.GuardrailScope,
			Queue:          entry,
		},
	}
}

func (s *TaskDispatchService) broadcastTaskAssigned(task *model.Task) {
	if s.hub == nil || task == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskAssigned,
		ProjectID: task.ProjectID.String(),
		Payload:   task.ToDTO(),
	})
}

func (s *TaskDispatchService) broadcastDispatchBlocked(task *model.Task, reason string, guardrailType string, guardrailScope string) {
	if s.hub == nil || task == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskDispatchBlocked,
		ProjectID: task.ProjectID.String(),
		Payload: map[string]any{
			"task": task.ToDTO(),
			"dispatch": model.DispatchOutcome{
				Status:         model.DispatchStatusBlocked,
				Reason:         reason,
				GuardrailType:  guardrailType,
				GuardrailScope: guardrailScope,
			},
		},
	})
}

func (s *TaskDispatchService) broadcastDispatchQueued(task *model.Task, entry *model.AgentPoolQueueEntry, reason string) {
	if s.hub == nil || task == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventAgentQueued,
		ProjectID: task.ProjectID.String(),
		Payload: map[string]any{
			"task": task.ToDTO(),
			"dispatch": model.DispatchOutcome{
				Status: model.DispatchStatusQueued,
				Reason: reason,
				Queue:  entry,
			},
		},
	})
	if provider, ok := s.runtime.(DispatchPoolStatsProvider); ok {
		s.hub.BroadcastEvent(&ws.Event{
			Type:      ws.EventAgentPoolUpdated,
			ProjectID: task.ProjectID.String(),
			Payload:   provider.PoolStats(context.Background()),
		})
	}
}

func (s *TaskDispatchService) createBlockedNotification(ctx context.Context, task *model.Task, reason string) {
	if s.notifications == nil || task == nil || task.AssigneeID == nil {
		return
	}
	data, _ := json.Marshal(map[string]string{
		"taskId": task.ID.String(),
		"reason": reason,
	})
	_, _ = s.notifications.Create(
		ctx,
		*task.AssigneeID,
		model.NotificationTypeTaskDispatchBlocked,
		"Agent dispatch blocked",
		reason,
		string(data),
	)
}

func (s *TaskDispatchService) checkBudget(ctx context.Context, task *model.Task, requestedUSD float64) (*model.DispatchBudgetWarning, *model.TaskDispatchResponse) {
	warning, blocked := evaluateDispatchBudget(ctx, task, requestedUSD, s.budgetChecker)
	if blocked == nil {
		return warning, nil
	}
	return nil, s.blockedResultWithGuardrail(ctx, task, blocked.Reason, blocked.GuardrailType, blocked.GuardrailScope)
}

func checkTaskBudget(task *model.Task, requestedUSD float64) (*model.DispatchBudgetWarning, *model.DispatchOutcome) {
	if task == nil || task.BudgetUsd <= 0 {
		return nil, nil
	}

	projected := task.SpentUsd + requestedUSD
	if projected > task.BudgetUsd {
		return nil, &model.DispatchOutcome{
			Status:         model.DispatchStatusBlocked,
			Reason:         fmt.Sprintf("task budget exceeded: spent $%.2f + requested $%.2f = $%.2f > limit $%.2f", task.SpentUsd, requestedUSD, projected, task.BudgetUsd),
			GuardrailType:  model.DispatchGuardrailTypeBudget,
			GuardrailScope: "task",
		}
	}
	if projected >= task.BudgetUsd*0.80 {
		return &model.DispatchBudgetWarning{
			Scope:   "task",
			Message: fmt.Sprintf("task budget warning: projected $%.2f / $%.2f (%.0f%% utilized)", projected, task.BudgetUsd, (projected/task.BudgetUsd)*100),
		}, nil
	}
	return nil, nil
}

func (s *TaskDispatchService) broadcastBudgetWarning(task *model.Task, warning *model.DispatchBudgetWarning) {
	if s.hub == nil || task == nil || warning == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventBudgetWarning,
		ProjectID: task.ProjectID.String(),
		Payload: map[string]any{
			"taskId":    task.ID.String(),
			"projectId": task.ProjectID.String(),
			"scope":     warning.Scope,
			"message":   warning.Message,
		},
	})
}

func inferDispatchGuardrail(reason string) (string, string) {
	switch {
	case strings.Contains(reason, "budget"):
		return model.DispatchGuardrailTypeBudget, inferBudgetScope(reason)
	case strings.Contains(reason, "pool"):
		return model.DispatchGuardrailTypePool, "project"
	case strings.Contains(reason, "worktree"):
		return model.DispatchGuardrailTypeSystem, "worktree"
	case strings.Contains(reason, "dispatch target"), strings.Contains(reason, "assigned agent member"), strings.Contains(reason, "assignee"):
		return model.DispatchGuardrailTypeTarget, "member"
	case strings.Contains(reason, "active agent run"):
		return model.DispatchGuardrailTypeTask, "task"
	default:
		return "", ""
	}
}

func inferBudgetScope(text string) string {
	switch {
	case strings.Contains(text, "task budget"):
		return "task"
	case strings.Contains(text, "sprint budget"):
		return "sprint"
	case strings.Contains(text, "project budget"):
		return "project"
	default:
		return ""
	}
}

func (s *TaskDispatchService) recordProgress(ctx context.Context, taskID uuid.UUID, input TaskActivityInput) {
	if s.progress == nil {
		return
	}
	_, _ = s.progress.RecordActivity(ctx, taskID, input)
}

func dtoPtr(dto model.AgentRunDTO) *model.AgentRunDTO {
	return &dto
}

func dispatchOutcomeContextFromInput(input DispatchSpawnInput) model.DispatchOutcome {
	return model.DispatchOutcome{
		Runtime:  strings.TrimSpace(input.Runtime),
		Provider: strings.TrimSpace(input.Provider),
		Model:    strings.TrimSpace(input.Model),
		RoleID:   strings.TrimSpace(input.RoleID),
	}
}

func applyDispatchOutcomeContext(outcome model.DispatchOutcome, context model.DispatchOutcome) model.DispatchOutcome {
	if outcome.Runtime == "" {
		outcome.Runtime = context.Runtime
	}
	if outcome.Provider == "" {
		outcome.Provider = context.Provider
	}
	if outcome.Model == "" {
		outcome.Model = context.Model
	}
	if outcome.RoleID == "" {
		outcome.RoleID = context.RoleID
	}
	return outcome
}

func queueEntryIDFromDispatch(dispatch model.DispatchOutcome) string {
	if dispatch.Queue == nil {
		return ""
	}
	return strings.TrimSpace(dispatch.Queue.EntryID)
}

func queuePriorityFromDispatch(dispatch model.DispatchOutcome) *int {
	if dispatch.Queue == nil {
		return nil
	}
	priority := dispatch.Queue.Priority
	return &priority
}
