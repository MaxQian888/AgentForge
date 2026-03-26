package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
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
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid page ID"})
	}
	req := new(model.DecomposeTasksFromPageRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	var parentTaskID *uuid.UUID
	if req.ParentTaskID != nil && *req.ParentTaskID != "" {
		parsed, err := uuid.Parse(*req.ParentTaskID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid parent task ID"})
		}
		parentTaskID = &parsed
	}
	resp, err := h.service.DecomposeTasksFromBlocks(c.Request().Context(), appMiddleware.GetProjectID(c), pageID, req.BlockIDs, parentTaskID, currentUserID(c))
	if err != nil {
		if errors.Is(err, service.ErrWikiPageNotFound) {
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "wiki page not found"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to decompose page into tasks"})
	}
	return c.JSON(http.StatusCreated, resp)
}
