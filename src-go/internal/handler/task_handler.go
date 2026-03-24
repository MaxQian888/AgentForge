package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
	"github.com/react-go-quick-starter/server/internal/service"
)

type TaskHandler struct {
	repo       *repository.TaskRepository
	decomposer taskDecomposer
	progress   taskProgressRecorder
	dispatcher taskDispatcher
}

type taskDecomposer interface {
	Decompose(ctx context.Context, taskID uuid.UUID) (*model.TaskDecompositionResponse, error)
}

type taskProgressRecorder interface {
	RecordActivity(ctx context.Context, taskID uuid.UUID, input service.TaskActivityInput) (*model.TaskProgressSnapshot, error)
}

type taskDispatcher interface {
	Assign(ctx context.Context, taskID uuid.UUID, req *model.AssignRequest) (*model.TaskDispatchResponse, error)
}

func NewTaskHandler(repo *repository.TaskRepository, decomposer ...taskDecomposer) *TaskHandler {
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

func (h *TaskHandler) Create(c echo.Context) error {
	req := new(model.CreateTaskRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
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
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid parent ID"})
		}
		task.ParentID = &pid
	}
	if req.SprintID != nil {
		sid, err := uuid.Parse(*req.SprintID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid sprint ID"})
		}
		task.SprintID = &sid
	}
	if req.PlannedStartAt != nil && *req.PlannedStartAt != "" {
		plannedStartAt, err := time.Parse(time.RFC3339, *req.PlannedStartAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid planned start date"})
		}
		task.PlannedStartAt = &plannedStartAt
	}
	if req.PlannedEndAt != nil && *req.PlannedEndAt != "" {
		plannedEndAt, err := time.Parse(time.RFC3339, *req.PlannedEndAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid planned end date"})
		}
		task.PlannedEndAt = &plannedEndAt
	}
	if task.PlannedStartAt != nil && task.PlannedEndAt != nil && task.PlannedEndAt.Before(*task.PlannedStartAt) {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "planned end date must be after planned start date"})
	}

	if err := h.repo.Create(c.Request().Context(), task); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create task"})
	}
	task, err := h.repo.GetByID(c.Request().Context(), task.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch created task"})
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

	tasks, total, err := h.repo.List(c.Request().Context(), projectID, q)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list tasks"})
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
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	task, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "task not found"})
	}
	return c.JSON(http.StatusOK, task.ToDTO())
}

func (h *TaskHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	req := new(model.UpdateTaskRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.repo.Update(c.Request().Context(), id, req); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to update task"})
	}
	task, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch updated task"})
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
	return c.JSON(http.StatusOK, task.ToDTO())
}

func (h *TaskHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to delete task"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "task deleted"})
}

func (h *TaskHandler) Transition(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	req := new(model.TransitionRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if err := h.repo.TransitionStatus(c.Request().Context(), id, req.Status); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}
	task, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch task"})
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
	return c.JSON(http.StatusOK, task.ToDTO())
}

func (h *TaskHandler) Assign(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	req := new(model.AssignRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if h.dispatcher != nil {
		result, err := h.dispatcher.Assign(c.Request().Context(), id, req)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrAgentTaskNotFound):
				return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "task not found"})
			default:
				return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to assign task"})
			}
		}
		return c.JSON(http.StatusOK, result)
	}
	assigneeID, err := uuid.Parse(req.AssigneeID)
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid assignee ID"})
	}
	if err := h.repo.UpdateAssignee(c.Request().Context(), id, assigneeID, req.AssigneeType); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to assign task"})
	}
	task, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch task"})
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
	return c.JSON(http.StatusOK, task.ToDTO())
}

func (h *TaskHandler) Decompose(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid task ID"})
	}
	if h.decomposer == nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "task decomposer unavailable"})
	}

	result, err := h.decomposer.Decompose(c.Request().Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTaskNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "task not found"})
		case errors.Is(err, service.ErrTaskAlreadyDecomposed):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: "task already has child tasks"})
		case errors.Is(err, service.ErrInvalidTaskDecomposition):
			return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "invalid task decomposition"})
		default:
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to decompose task"})
		}
	}
	return c.JSON(http.StatusOK, result)
}
