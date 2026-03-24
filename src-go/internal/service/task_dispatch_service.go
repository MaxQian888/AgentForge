package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

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

type DispatchNotificationService interface {
	Create(ctx context.Context, targetID uuid.UUID, ntype, title, body, data string) (*model.Notification, error)
}

type DispatchSpawnInput struct {
	TaskID    uuid.UUID
	MemberID  *uuid.UUID
	Runtime   string
	Provider  string
	Model     string
	BudgetUSD float64
	RoleID    string
}

type TaskDispatchService struct {
	tasks         DispatchTaskRepository
	members       DispatchMemberRepository
	runtime       DispatchRuntimeService
	hub           *ws.Hub
	notifications DispatchNotificationService
	progress      *TaskProgressService
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
		return s.blockedResult(ctx, task, "dispatch target is unavailable"), nil
	}
	if member.ProjectID != task.ProjectID {
		return s.blockedResult(ctx, task, "dispatch target is outside the task project"), nil
	}
	if req.AssigneeType == model.MemberTypeAgent && (member.Type != model.MemberTypeAgent || !member.IsActive) {
		return s.blockedResult(ctx, task, "dispatch target is not an active agent member"), nil
	}
	if req.AssigneeType == model.MemberTypeHuman && member.Type != model.MemberTypeHuman {
		return s.blockedResult(ctx, task, "dispatch target does not match the requested assignee type"), nil
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
		return &model.TaskDispatchResponse{
			Task: updatedTask.ToDTO(),
			Dispatch: model.DispatchOutcome{
				Status: model.DispatchStatusSkipped,
				Reason: "task assigned to a human member",
			},
		}, nil
	}

	return s.spawnForTask(ctx, updatedTask, memberID, DispatchSpawnInput{})
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
			return s.blockedResult(ctx, task, "task has no valid assigned agent member"), nil
		}
		memberID = *task.AssigneeID
	}

	member, err := s.members.GetByID(ctx, memberID)
	if err != nil {
		return s.blockedResult(ctx, task, "dispatch target is unavailable"), nil
	}
	if member.ProjectID != task.ProjectID || member.Type != model.MemberTypeAgent || !member.IsActive {
		return s.blockedResult(ctx, task, "task has no valid assigned agent member"), nil
	}

	return s.spawnForTask(ctx, task, memberID, input)
}

func (s *TaskDispatchService) spawnForTask(ctx context.Context, task *model.Task, memberID uuid.UUID, input DispatchSpawnInput) (*model.TaskDispatchResponse, error) {
	run, err := s.runtime.Spawn(ctx, task.ID, memberID, input.Runtime, input.Provider, input.Model, input.BudgetUSD, input.RoleID)
	if err != nil {
		switch {
		case errors.Is(err, ErrAgentAlreadyRunning):
			return s.blockedResult(ctx, task, "task already has an active agent run"), nil
		case errors.Is(err, ErrAgentWorktreeUnavailable):
			return s.blockedResult(ctx, task, "agent dispatch is blocked by worktree availability"), nil
		default:
			return s.blockedResult(ctx, task, err.Error()), nil
		}
	}

	updatedTask, fetchErr := s.tasks.GetByID(ctx, task.ID)
	if fetchErr != nil {
		updatedTask = task
	}

	return &model.TaskDispatchResponse{
		Task: updatedTask.ToDTO(),
		Dispatch: model.DispatchOutcome{
			Status: model.DispatchStatusStarted,
			Run:    dtoPtr(run.ToDTO()),
		},
	}, nil
}

func (s *TaskDispatchService) blockedResult(ctx context.Context, task *model.Task, reason string) *model.TaskDispatchResponse {
	if task != nil {
		s.broadcastDispatchBlocked(task, reason)
		s.createBlockedNotification(ctx, task, reason)
	}
	taskDTO := model.TaskDTO{}
	if task != nil {
		taskDTO = task.ToDTO()
	}
	return &model.TaskDispatchResponse{
		Task: taskDTO,
		Dispatch: model.DispatchOutcome{
			Status: model.DispatchStatusBlocked,
			Reason: reason,
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

func (s *TaskDispatchService) broadcastDispatchBlocked(task *model.Task, reason string) {
	if s.hub == nil || task == nil {
		return
	}
	s.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskDispatchBlocked,
		ProjectID: task.ProjectID.String(),
		Payload: map[string]any{
			"task": task.ToDTO(),
			"dispatch": model.DispatchOutcome{
				Status: model.DispatchStatusBlocked,
				Reason: reason,
			},
		},
	})
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

func (s *TaskDispatchService) recordProgress(ctx context.Context, taskID uuid.UUID, input TaskActivityInput) {
	if s.progress == nil {
		return
	}
	_, _ = s.progress.RecordActivity(ctx, taskID, input)
}

func dtoPtr(dto model.AgentRunDTO) *model.AgentRunDTO {
	return &dto
}
