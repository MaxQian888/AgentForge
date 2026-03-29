package handler

import (
	"context"
	"net/http"
	"strconv"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type MemoryRuntimeService interface {
	Store(ctx context.Context, input service.StoreMemoryInput) (*model.AgentMemory, error)
	Search(ctx context.Context, projectID uuid.UUID, query string, limit int) ([]model.AgentMemoryDTO, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type MemoryHandler struct {
	service MemoryRuntimeService
}

func NewMemoryHandler(service MemoryRuntimeService) *MemoryHandler {
	return &MemoryHandler{service: service}
}

type StoreMemoryRequest struct {
	Scope          string  `json:"scope"`
	RoleID         string  `json:"roleId"`
	Category       string  `json:"category"`
	Key            string  `json:"key" validate:"required"`
	Content        string  `json:"content" validate:"required"`
	Metadata       string  `json:"metadata"`
	RelevanceScore float64 `json:"relevanceScore"`
}

func (h *MemoryHandler) Store(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}

	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}

	req := new(StoreMemoryRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	mem, err := h.service.Store(c.Request().Context(), service.StoreMemoryInput{
		ProjectID:      projectID,
		Scope:          req.Scope,
		RoleID:         req.RoleID,
		Category:       req.Category,
		Key:            req.Key,
		Content:        req.Content,
		Metadata:       req.Metadata,
		RelevanceScore: req.RelevanceScore,
	})
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToStoreMemory)
	}
	return c.JSON(http.StatusCreated, mem.ToDTO())
}

func (h *MemoryHandler) Search(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}

	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}

	query := c.QueryParam("q")
	limit := 20
	if l := c.QueryParam("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	results, err := h.service.Search(c.Request().Context(), projectID, query, limit)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToSearchMemories)
	}
	return c.JSON(http.StatusOK, results)
}

func (h *MemoryHandler) Delete(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}

	id, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemoryID)
	}

	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteMemory)
	}
	return c.NoContent(http.StatusNoContent)
}
