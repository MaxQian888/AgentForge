package handler

import (
	"context"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type taskCommentHandlerService interface {
	CreateComment(ctx context.Context, input *service.CreateTaskCommentInput) (*model.TaskComment, error)
	ReplyToComment(ctx context.Context, parentCommentID uuid.UUID, input *service.CreateTaskCommentInput) (*model.TaskComment, error)
	ResolveComment(ctx context.Context, commentID uuid.UUID) (*model.TaskComment, error)
	ReopenComment(ctx context.Context, commentID uuid.UUID) (*model.TaskComment, error)
	DeleteComment(ctx context.Context, commentID uuid.UUID) error
	ListComments(ctx context.Context, taskID uuid.UUID) ([]*model.TaskComment, error)
}

type TaskCommentHandler struct {
	service taskCommentHandlerService
}

func NewTaskCommentHandler(service taskCommentHandlerService) *TaskCommentHandler {
	return &TaskCommentHandler{service: service}
}

func (h *TaskCommentHandler) List(c echo.Context) error {
	taskID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	comments, err := h.service.ListComments(c.Request().Context(), taskID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListTaskComments)
	}
	payload := make([]model.TaskCommentDTO, 0, len(comments))
	for _, comment := range comments {
		payload = append(payload, comment.ToDTO())
	}
	return c.JSON(http.StatusOK, payload)
}

func (h *TaskCommentHandler) Create(c echo.Context) error {
	taskID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTaskID)
	}
	req := new(model.CreateTaskCommentRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	createdBy := currentUserID(c)
	if createdBy == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgMissingUserContext)
	}
	input := &service.CreateTaskCommentInput{
		ProjectID: appMiddleware.GetProjectID(c),
		TaskID:    taskID,
		Body:      req.Body,
		CreatedBy: *createdBy,
	}
	var comment *model.TaskComment
	if req.ParentCommentID != nil {
		parentCommentID, err := uuid.Parse(*req.ParentCommentID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidParentCommentID)
		}
		comment, err = h.service.ReplyToComment(c.Request().Context(), parentCommentID, input)
		if err != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateTaskComment)
		}
	} else {
		comment, err = h.service.CreateComment(c.Request().Context(), input)
		if err != nil {
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateTaskComment)
		}
	}
	return c.JSON(http.StatusCreated, comment.ToDTO())
}

func (h *TaskCommentHandler) Update(c echo.Context) error {
	commentID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidCommentID)
	}
	req := new(model.UpdateTaskCommentRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.Resolved == nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgResolvedFlagRequired)
	}
	var comment *model.TaskComment
	if *req.Resolved {
		comment, err = h.service.ResolveComment(c.Request().Context(), commentID)
	} else {
		comment, err = h.service.ReopenComment(c.Request().Context(), commentID)
	}
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateTaskComment)
	}
	return c.JSON(http.StatusOK, comment.ToDTO())
}

func (h *TaskCommentHandler) Delete(c echo.Context) error {
	commentID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidCommentID)
	}
	if err := h.service.DeleteComment(c.Request().Context(), commentID); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteTaskComment)
	}
	return c.NoContent(http.StatusNoContent)
}
