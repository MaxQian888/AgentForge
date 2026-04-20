package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type docDecompositionHandlerService interface {
	DecomposeTasksFromBlocks(ctx context.Context, projectID uuid.UUID, pageID uuid.UUID, blockIDs []string, parentTaskID *uuid.UUID, createdBy *uuid.UUID) (*model.DecomposeTasksFromPageResponse, error)
}

type DocDecompositionHandler struct {
	service docDecompositionHandlerService
}

func NewDocDecompositionHandler(service docDecompositionHandlerService) *DocDecompositionHandler {
	return &DocDecompositionHandler{service: service}
}

func (h *DocDecompositionHandler) Decompose(c echo.Context) error {
	pageID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidPageID)
	}
	req := new(model.DecomposeTasksFromPageRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	var parentTaskID *uuid.UUID
	if req.ParentTaskID != nil && *req.ParentTaskID != "" {
		parsed, err := uuid.Parse(*req.ParentTaskID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidParentTaskID)
		}
		parentTaskID = &parsed
	}
	resp, err := h.service.DecomposeTasksFromBlocks(c.Request().Context(), appMiddleware.GetProjectID(c), pageID, req.BlockIDs, parentTaskID, currentUserID(c))
	if err != nil {
		if errors.Is(err, service.ErrWikiPageNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgWikiPageNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDecomposePageIntoTasks)
	}
	return c.JSON(http.StatusCreated, resp)
}
