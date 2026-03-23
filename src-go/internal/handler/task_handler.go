package handler

import (
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type TaskHandler struct {
	repo *repository.TaskRepository
}

func NewTaskHandler(repo *repository.TaskRepository) *TaskHandler {
	return &TaskHandler{repo: repo}
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
		ID:        uuid.New(),
		ProjectID: projectID,
		Title:     req.Title,
		Description: req.Description,
		Status:    model.TaskStatusInbox,
		Priority:  req.Priority,
		Labels:    req.Labels,
		BudgetUsd: req.BudgetUsd,
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

	if err := h.repo.Create(c.Request().Context(), task); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create task"})
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
	return c.JSON(http.StatusOK, task.ToDTO())
}
