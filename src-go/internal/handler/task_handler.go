package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
	"github.com/react-go-quick-starter/server/internal/ws"
)

type taskWorkflowEvaluator interface {
	EvaluateTransition(ctx context.Context, task *model.Task, fromStatus, toStatus string) []service.TriggerResult
}

type taskRecommender interface {
	Recommend(ctx context.Context, taskID uuid.UUID) ([]service.AssignmentCandidate, error)
}

type TaskHandler struct {
	repo        taskRepository
	decomposer  taskDecomposer
	progress    taskProgressRecorder
	dispatcher  taskDispatcher
	recommender taskRecommender
	hub         *ws.Hub
	workflowSvc taskWorkflowEvaluator
	automation  service.AutomationEventEvaluator
}

type taskDecomposer interface {
	Decompose(ctx context.Context, taskID uuid.UUID) (*model.TaskDecompositionResponse, error)
}

type taskRepository interface {
	Create(ctx context.Context, task *model.Task) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Task, error)
	List(ctx context.Context, projectID uuid.UUID, q model.TaskListQuery) ([]*model.Task, int, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateTaskRequest) error
	Delete(ctx context.Context, id uuid.UUID) error
	TransitionStatus(ctx context.Context, id uuid.UUID, newStatus string) error
	UpdateAssignee(ctx context.Context, id uuid.UUID, assigneeID uuid.UUID, assigneeType string) error
	ListDependents(ctx context.Context, blockerID uuid.UUID) ([]*model.Task, error)
}

type taskProgressRecorder interface {
	RecordActivity(ctx context.Context, taskID uuid.UUID, input service.TaskActivityInput) (*model.TaskProgressSnapshot, error)
}

type taskDispatcher interface {
	Assign(ctx context.Context, taskID uuid.UUID, req *model.AssignRequest) (*model.TaskDispatchResponse, error)
}

func NewTaskHandler(repo taskRepository, decomposer ...taskDecomposer) *TaskHandler {
	handler := &TaskHandler{repo: repo}
	if len(decomposer) > 0 {
		handler.decomposer = decomposer[0]
	}
	return handler
}

func (h *TaskHandler) WithProgress(progress taskProgressRecorder) *TaskHandler {
	h.progress = progress
	return h
}

func (h *TaskHandler) WithDispatcher(dispatcher taskDispatcher) *TaskHandler {
	h.dispatcher = dispatcher
	return h
}

func (h *TaskHandler) WithHub(hub *ws.Hub) *TaskHandler {
	h.hub = hub
	return h
}

func (h *TaskHandler) WithWorkflowService(svc taskWorkflowEvaluator) *TaskHandler {
	h.workflowSvc = svc
	return h
}

func (h *TaskHandler) WithRecommender(rec taskRecommender) *TaskHandler {
	h.recommender = rec
	return h
}

func (h *TaskHandler) WithAutomation(evaluator service.AutomationEventEvaluator) *TaskHandler {
	h.automation = evaluator
	return h
}

func (h *TaskHandler) RecommendAssignee(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	if h.recommender == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgRecommenderUnavailable)
	}
	candidates, err := h.recommender.Recommend(c.Request().Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "fetch task") {
			return localizedError(c, http.StatusNotFound, i18n.MsgTaskNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToGenerateRecommendations)
	}
	return c.JSON(http.StatusOK, candidates)
}

func (h *TaskHandler) Create(c echo.Context) error {
	req := new(model.CreateTaskRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	projectID := appMiddleware.GetProjectID(c)

	task := &model.Task{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Title:       req.Title,
		Description: req.Description,
		Status:      model.TaskStatusInbox,
		Priority:    req.Priority,
		Labels:      req.Labels,
		BudgetUsd:   req.BudgetUsd,
	}

	if req.ParentID != nil {
		pid, err := uuid.Parse(*req.ParentID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidParentID)
		}
		task.ParentID = &pid
	}
	if req.SprintID != nil {
		sid, err := uuid.Parse(*req.SprintID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidSprintID)
		}
		task.SprintID = &sid
	}
	if req.PlannedStartAt != nil && *req.PlannedStartAt != "" {
		plannedStartAt, err := time.Parse(time.RFC3339, *req.PlannedStartAt)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPlannedStartDate)
		}
		task.PlannedStartAt = &plannedStartAt
	}
	if req.PlannedEndAt != nil && *req.PlannedEndAt != "" {
		plannedEndAt, err := time.Parse(time.RFC3339, *req.PlannedEndAt)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPlannedEndDate)
		}
		task.PlannedEndAt = &plannedEndAt
	}
	if task.PlannedStartAt != nil && task.PlannedEndAt != nil && task.PlannedEndAt.Before(*task.PlannedStartAt) {
		return localizedError(c, http.StatusBadRequest, i18n.MsgPlannedEndBeforeStart)
	}

	if err := h.repo.Create(c.Request().Context(), task); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateTask)
	}
	task, err := h.repo.GetByID(c.Request().Context(), task.ID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchCreatedTask)
	}
	if h.progress != nil {
		snapshot, progressErr := h.progress.RecordActivity(c.Request().Context(), task.ID, service.TaskActivityInput{
			Source:         model.TaskProgressSourceTaskCreated,
			OccurredAt:     task.CreatedAt,
			UpdateHealth:   true,
			MarkTransition: true,
		})
		if progressErr == nil {
			task.Progress = snapshot
		}
	}
	return c.JSON(http.StatusCreated, task.ToDTO())
}

func (h *TaskHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	q := model.TaskListQuery{
		Status:     c.QueryParam("status"),
		AssigneeID: c.QueryParam("assignee_id"),
		SprintID:   c.QueryParam("sprint_id"),
		Priority:   c.QueryParam("priority"),
		Search:     c.QueryParam("search"),
		Page:       page,
		Limit:      limit,
		Sort:       c.QueryParam("sort"),
	}
	if rawFilters := strings.TrimSpace(c.QueryParam("customFieldFilters")); rawFilters != "" {
		if err := json.Unmarshal([]byte(rawFilters), &q.CustomFieldFilters); err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidCustomFieldFilters)
		}
	}
	if rawSort := strings.TrimSpace(c.QueryParam("customFieldSort")); rawSort != "" {
		var customSort model.TaskCustomFieldSort
		if err := json.Unmarshal([]byte(rawSort), &customSort); err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidCustomFieldSort)
		}
		q.CustomFieldSort = &customSort
	}

	tasks, total, err := h.repo.List(c.Request().Context(), projectID, q)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListTasks)
	}

	dtos := make([]model.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		dtos = append(dtos, t.ToDTO())
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"items": dtos,
		"total": total,
		"page":  q.Page,
		"limit": q.Limit,
	})
}

func (h *TaskHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	task, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgTaskNotFound)
	}
	return c.JSON(http.StatusOK, task.ToDTO())
}

func (h *TaskHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	req := new(model.UpdateTaskRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.BlockedBy != nil {
		sanitized, err := sanitizeBlockedByIDs(id, *req.BlockedBy)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
		}
		req.BlockedBy = &sanitized
	}
	if err := h.repo.Update(c.Request().Context(), id, req); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateTask)
	}
	task, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchUpdatedTask)
	}
	if req.BlockedBy != nil {
		task, err = h.resolveReadyTaskState(c.Request().Context(), task)
		if err != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToReconcileDeps)
		}
	}
	if h.progress != nil {
		snapshot, progressErr := h.progress.RecordActivity(c.Request().Context(), task.ID, service.TaskActivityInput{
			Source:       model.TaskProgressSourceTaskUpdated,
			OccurredAt:   task.UpdatedAt,
			UpdateHealth: true,
		})
		if progressErr == nil {
			task.Progress = snapshot
		}
	}
	h.broadcastTaskUpdated(task)
	if h.automation != nil {
		taskID := task.ID
		_ = h.automation.EvaluateRules(c.Request().Context(), service.AutomationEvent{
			EventType: model.AutomationEventTaskFieldChanged,
			ProjectID: task.ProjectID,
			TaskID:    &taskID,
			Task:      task,
			Data: map[string]any{
				"field": "task",
			},
		})
	}
	return c.JSON(http.StatusOK, task.ToDTO())
}

func (h *TaskHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteTask)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "task deleted"})
}

func (h *TaskHandler) Transition(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	req := new(model.TransitionRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	// Capture previous status before transition for workflow evaluation.
	var previousStatus string
	if preTask, fetchErr := h.repo.GetByID(c.Request().Context(), id); fetchErr == nil {
		previousStatus = preTask.Status
	}

	if err := h.repo.TransitionStatus(c.Request().Context(), id, req.Status); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	task, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchTask)
	}
	if h.progress != nil {
		snapshot, progressErr := h.progress.RecordActivity(c.Request().Context(), task.ID, service.TaskActivityInput{
			Source:         model.TaskProgressSourceTaskTransition,
			OccurredAt:     task.UpdatedAt,
			UpdateHealth:   true,
			MarkTransition: true,
		})
		if progressErr == nil {
			task.Progress = snapshot
		}
	}
	updatedDependents, err := h.autoUnblockDependents(c.Request().Context(), task)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateDependentTasks)
	}
	h.broadcastTaskTransitioned(task, req.Reason)
	for _, dependent := range updatedDependents {
		h.broadcastTaskUpdated(dependent)
		h.broadcastDependencyResolved(dependent, task)
	}

	// Evaluate workflow triggers after successful transition.
	if h.workflowSvc != nil && previousStatus != "" {
		go h.workflowSvc.EvaluateTransition(context.Background(), task, previousStatus, req.Status)
	}
	if h.automation != nil {
		taskID := task.ID
		_ = h.automation.EvaluateRules(c.Request().Context(), service.AutomationEvent{
			EventType: model.AutomationEventTaskStatusChanged,
			ProjectID: task.ProjectID,
			TaskID:    &taskID,
			Task:      task,
			Data: map[string]any{
				"status":         task.Status,
				"previousStatus": previousStatus,
				"newStatus":      req.Status,
			},
		})
	}

	return c.JSON(http.StatusOK, task.ToDTO())
}

func (h *TaskHandler) Assign(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	req := new(model.AssignRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if h.dispatcher != nil {
		result, err := h.dispatcher.Assign(c.Request().Context(), id, req)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrAgentTaskNotFound):
				return localizedError(c, http.StatusNotFound, i18n.MsgTaskNotFound)
			default:
				return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToAssignTask)
			}
		}
		return c.JSON(http.StatusOK, result)
	}
	assigneeID, err := uuid.Parse(req.AssigneeID)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidAssigneeID)
	}
	if err := h.repo.UpdateAssignee(c.Request().Context(), id, assigneeID, req.AssigneeType); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToAssignTask)
	}
	task, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchTask)
	}
	if h.progress != nil {
		snapshot, progressErr := h.progress.RecordActivity(c.Request().Context(), task.ID, service.TaskActivityInput{
			Source:       model.TaskProgressSourceTaskAssigned,
			OccurredAt:   task.UpdatedAt,
			UpdateHealth: true,
		})
		if progressErr == nil {
			task.Progress = snapshot
		}
	}
	if h.automation != nil {
		taskID := task.ID
		_ = h.automation.EvaluateRules(c.Request().Context(), service.AutomationEvent{
			EventType: model.AutomationEventTaskAssigneeChanged,
			ProjectID: task.ProjectID,
			TaskID:    &taskID,
			Task:      task,
			Data: map[string]any{
				"assignee_id":   req.AssigneeID,
				"assignee_type": req.AssigneeType,
			},
		})
	}
	return c.JSON(http.StatusOK, task.ToDTO())
}

func (h *TaskHandler) Decompose(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	if h.decomposer == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgTaskDecomposerUnavailable)
	}

	result, err := h.decomposer.Decompose(c.Request().Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			return localizedError(c, http.StatusNotFound, i18n.MsgTaskNotFound)
		case errors.Is(err, service.ErrTaskAlreadyDecomposed):
			return localizedError(c, http.StatusConflict, i18n.MsgTaskAlreadyHasChildren)
		case errors.Is(err, service.ErrInvalidTaskDecomposition):
			return localizedError(c, http.StatusBadGateway, i18n.MsgInvalidTaskDecomposition)
		default:
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDecomposeTask)
		}
	}
	return c.JSON(http.StatusOK, result)
}

func sanitizeBlockedByIDs(taskID uuid.UUID, blockedBy []string) ([]string, error) {
	seen := make(map[string]struct{}, len(blockedBy))
	sanitized := make([]string, 0, len(blockedBy))
	for _, rawID := range blockedBy {
		trimmed := strings.TrimSpace(rawID)
		if trimmed == "" {
			return nil, errors.New("blockedBy entries must be valid task IDs")
		}
		parsedID, err := uuid.Parse(trimmed)
		if err != nil {
			return nil, errors.New("blockedBy entries must be valid task IDs")
		}
		if parsedID == taskID {
			return nil, errors.New("task cannot depend on itself")
		}
		normalized := parsedID.String()
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		sanitized = append(sanitized, normalized)
	}
	return sanitized, nil
}

func (h *TaskHandler) resolveReadyTaskState(ctx context.Context, task *model.Task) (*model.Task, error) {
	if task == nil || task.Status != model.TaskStatusBlocked {
		return task, nil
	}
	ready, err := h.hasNoRemainingBlockers(ctx, task)
	if err != nil || !ready {
		return task, err
	}

	nextStatus := model.TaskStatusTriaged
	if task.AssigneeID != nil {
		nextStatus = model.TaskStatusAssigned
	}
	if err := model.ValidateTransition(task.Status, nextStatus); err != nil {
		return task, nil
	}
	if err := h.repo.TransitionStatus(ctx, task.ID, nextStatus); err != nil {
		return nil, err
	}
	return h.repo.GetByID(ctx, task.ID)
}

func (h *TaskHandler) hasNoRemainingBlockers(ctx context.Context, task *model.Task) (bool, error) {
	if task == nil || len(task.BlockedBy) == 0 {
		return true, nil
	}

	for _, blockerID := range task.BlockedBy {
		parsedID, err := uuid.Parse(blockerID)
		if err != nil {
			return false, nil
		}
		blocker, err := h.repo.GetByID(ctx, parsedID)
		if err != nil {
			return false, nil
		}
		if blocker.Status != model.TaskStatusDone {
			return false, nil
		}
	}
	return true, nil
}

func (h *TaskHandler) autoUnblockDependents(ctx context.Context, task *model.Task) ([]*model.Task, error) {
	if task == nil || task.Status != model.TaskStatusDone {
		return nil, nil
	}

	dependents, err := h.repo.ListDependents(ctx, task.ID)
	if err != nil {
		return nil, err
	}

	updated := make([]*model.Task, 0, len(dependents))
	for _, dependent := range dependents {
		if dependent == nil || dependent.Status != model.TaskStatusBlocked {
			continue
		}
		nextTask, err := h.resolveReadyTaskState(ctx, dependent)
		if err != nil {
			return nil, err
		}
		if nextTask != nil && nextTask.Status != dependent.Status {
			updated = append(updated, nextTask)
		}
	}
	return updated, nil
}

func (h *TaskHandler) broadcastTaskUpdated(task *model.Task) {
	if h.hub == nil || task == nil {
		return
	}
	h.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskUpdated,
		ProjectID: task.ProjectID.String(),
		Payload: map[string]any{
			"task": task.ToDTO(),
		},
	})
}

func (h *TaskHandler) broadcastDependencyResolved(dependent *model.Task, resolved *model.Task) {
	if h.hub == nil || dependent == nil || resolved == nil {
		return
	}
	h.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskDependencyResolved,
		ProjectID: dependent.ProjectID.String(),
		Payload: map[string]any{
			"taskId":         dependent.ID.String(),
			"resolvedTaskId": resolved.ID.String(),
			"newStatus":      dependent.Status,
		},
	})
}

func (h *TaskHandler) broadcastTaskTransitioned(task *model.Task, reason string) {
	if h.hub == nil || task == nil {
		return
	}
	h.hub.BroadcastEvent(&ws.Event{
		Type:      ws.EventTaskTransitioned,
		ProjectID: task.ProjectID.String(),
		Payload: map[string]any{
			"task":   task.ToDTO(),
			"reason": reason,
		},
	})
}
