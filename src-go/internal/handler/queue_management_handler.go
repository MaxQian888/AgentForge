package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type queueManagementService interface {
	ListQueueEntries(ctx context.Context, projectID uuid.UUID, statusFilter string) ([]*model.AgentPoolQueueEntry, error)
	CancelQueueEntry(ctx context.Context, projectID uuid.UUID, entryID string, reason string) (*model.AgentPoolQueueEntry, error)
}

type QueueManagementHandler struct {
	service queueManagementService
}

func NewQueueManagementHandler(service queueManagementService) *QueueManagementHandler {
	return &QueueManagementHandler{service: service}
}

func (h *QueueManagementHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	entries, err := h.service.ListQueueEntries(c.Request().Context(), projectID, c.QueryParam("status"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
	}

	dtos := make([]model.QueueEntryDTO, 0, len(entries))
	for _, entry := range entries {
		dtos = append(dtos, entry.ToDTO())
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *QueueManagementHandler) Cancel(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	entryID := strings.TrimSpace(c.Param("entryId"))
	if entryID == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "queue entry id is required"})
	}

	entry, err := h.service.CancelQueueEntry(c.Request().Context(), projectID, entryID, c.QueryParam("reason"))
	if err != nil {
		switch {
		case errors.Is(err, service.ErrQueueEntryNotFound):
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
		default:
			var conflictErr *service.QueueEntryStatusConflictError
			if errors.As(err, &conflictErr) {
				return c.JSON(http.StatusConflict, map[string]any{
					"message": err.Error(),
					"status":  conflictErr.Status,
				})
			}
			return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: err.Error()})
		}
	}

	return c.JSON(http.StatusOK, entry.ToDTO())
}
