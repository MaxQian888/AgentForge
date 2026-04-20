package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type MemoryRuntimeService interface {
	Store(ctx context.Context, input service.StoreMemoryInput) (*model.AgentMemory, error)
	Update(ctx context.Context, input service.UpdateMemoryInput) (*model.AgentMemory, error)
	Search(ctx context.Context, query service.MemoryExplorerQuery) ([]model.AgentMemoryDTO, error)
	Get(ctx context.Context, projectID uuid.UUID, id uuid.UUID, roleID string) (*model.AgentMemoryDetailDTO, error)
	Stats(ctx context.Context, query service.MemoryExplorerQuery) (*model.MemoryExplorerStatsDTO, error)
	ExportEpisodic(ctx context.Context, query service.MemoryExplorerQuery) (*service.EpisodicMemoryExport, error)
	BulkDelete(ctx context.Context, projectID uuid.UUID, ids []uuid.UUID, roleID string) (int64, error)
	CleanupEpisodic(ctx context.Context, input service.MemoryCleanupInput) (int64, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type MemoryHandler struct {
	service MemoryRuntimeService
}

func NewMemoryHandler(service MemoryRuntimeService) *MemoryHandler {
	return &MemoryHandler{service: service}
}

type StoreMemoryRequest struct {
	Scope          string   `json:"scope"`
	RoleID         string   `json:"roleId"`
	Category       string   `json:"category"`
	Tags           []string `json:"tags"`
	Key            string   `json:"key" validate:"required"`
	Content        string   `json:"content" validate:"required"`
	Metadata       string   `json:"metadata"`
	RelevanceScore float64  `json:"relevanceScore"`
}

type UpdateMemoryRequest struct {
	Key     *string   `json:"key"`
	Content *string   `json:"content"`
	Tags    *[]string `json:"tags"`
}

type BulkDeleteMemoriesRequest struct {
	IDs    []string `json:"ids"`
	RoleID string   `json:"roleId"`
}

type CleanupMemoriesRequest struct {
	Scope         string `json:"scope"`
	RoleID        string `json:"roleId"`
	Before        string `json:"before"`
	RetentionDays int    `json:"retentionDays"`
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
		Tags:           req.Tags,
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

func (h *MemoryHandler) Update(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	memoryID, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemoryID)
	}
	req := new(UpdateMemoryRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	updated, err := h.service.Update(c.Request().Context(), service.UpdateMemoryInput{
		ProjectID: projectID,
		ID:        memoryID,
		RoleID:    strings.TrimSpace(c.QueryParam("roleId")),
		Key:       req.Key,
		Content:   req.Content,
		Tags:      req.Tags,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrMemoryNotEditable):
			return c.JSON(http.StatusConflict, model.ErrorResponse{Message: err.Error()})
		case errors.Is(err, service.ErrMemoryUpdateRequired):
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
		default:
			return h.writeMemoryError(c, err, http.StatusInternalServerError, "failed to update memory")
		}
	}
	return c.JSON(http.StatusOK, updated.ToDetailDTO())
}

func (h *MemoryHandler) Search(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}

	query, status, err := h.parseExplorerQuery(c, 20)
	if err != nil {
		return c.JSON(status, model.ErrorResponse{Message: err.Error()})
	}
	results, err := h.service.Search(c.Request().Context(), query)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToSearchMemories)
	}
	return c.JSON(http.StatusOK, results)
}

func (h *MemoryHandler) Get(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	memoryID, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidMemoryID)
	}
	detail, err := h.service.Get(c.Request().Context(), projectID, memoryID, strings.TrimSpace(c.QueryParam("roleId")))
	if err != nil {
		return h.writeMemoryError(c, err, http.StatusInternalServerError, "failed to get memory")
	}
	return c.JSON(http.StatusOK, detail)
}

func (h *MemoryHandler) Stats(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}
	query, status, err := h.parseExplorerQuery(c, 0)
	if err != nil {
		return c.JSON(status, model.ErrorResponse{Message: err.Error()})
	}
	stats, err := h.service.Stats(c.Request().Context(), query)
	if err != nil {
		return h.writeMemoryError(c, err, http.StatusInternalServerError, "failed to get memory stats")
	}
	return c.JSON(http.StatusOK, stats)
}

func (h *MemoryHandler) Export(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}
	query, status, err := h.parseExplorerQuery(c, 0)
	if err != nil {
		return c.JSON(status, model.ErrorResponse{Message: err.Error()})
	}
	exported, err := h.service.ExportEpisodic(c.Request().Context(), query)
	if err != nil {
		return h.writeMemoryError(c, err, http.StatusInternalServerError, "failed to export memories")
	}
	return c.JSON(http.StatusOK, exported)
}

func (h *MemoryHandler) BulkDelete(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	req := new(BulkDeleteMemoriesRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(strings.TrimSpace(raw))
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "memory ids must be valid UUIDs"})
		}
		ids = append(ids, id)
	}
	deleted, err := h.service.BulkDelete(c.Request().Context(), projectID, ids, strings.TrimSpace(req.RoleID))
	if err != nil {
		return h.writeMemoryError(c, err, http.StatusInternalServerError, "failed to delete memories")
	}
	return c.JSON(http.StatusOK, model.MemoryDeleteResultDTO{DeletedCount: deleted})
}

func (h *MemoryHandler) Cleanup(c echo.Context) error {
	if h.service == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgMemoryServiceUnavailable)
	}
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	req := new(CleanupMemoriesRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	var before *time.Time
	if strings.TrimSpace(req.Before) != "" {
		parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.Before))
		if err != nil {
			return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "before must be a valid RFC3339 timestamp"})
		}
		parsed = parsed.UTC()
		before = &parsed
	}
	deleted, err := h.service.CleanupEpisodic(c.Request().Context(), service.MemoryCleanupInput{
		ProjectID:     projectID,
		Scope:         strings.TrimSpace(req.Scope),
		RoleID:        strings.TrimSpace(req.RoleID),
		Before:        before,
		RetentionDays: req.RetentionDays,
	})
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "required") || strings.Contains(err.Error(), "positive") {
			status = http.StatusBadRequest
		}
		return c.JSON(status, model.ErrorResponse{Message: err.Error()})
	}
	return c.JSON(http.StatusOK, model.MemoryDeleteResultDTO{DeletedCount: deleted})
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

func (h *MemoryHandler) parseExplorerQuery(c echo.Context, defaultLimit int) (service.MemoryExplorerQuery, int, error) {
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return service.MemoryExplorerQuery{}, http.StatusBadRequest, errors.New("invalid project id")
	}
	query := strings.TrimSpace(c.QueryParam("query"))
	if query == "" {
		query = strings.TrimSpace(c.QueryParam("q"))
	}
	limit := defaultLimit
	if rawLimit := strings.TrimSpace(c.QueryParam("limit")); rawLimit != "" {
		if parsed, err := strconv.Atoi(rawLimit); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	startAt, err := parseOptionalRFC3339(c.QueryParam("startAt"))
	if err != nil {
		return service.MemoryExplorerQuery{}, http.StatusBadRequest, errors.New("startAt must be a valid RFC3339 timestamp")
	}
	endAt, err := parseOptionalRFC3339(c.QueryParam("endAt"))
	if err != nil {
		return service.MemoryExplorerQuery{}, http.StatusBadRequest, errors.New("endAt must be a valid RFC3339 timestamp")
	}
	scope := strings.TrimSpace(c.QueryParam("scope"))
	roleID := strings.TrimSpace(c.QueryParam("roleId"))
	if scope == model.MemoryScopeRole && roleID == "" {
		return service.MemoryExplorerQuery{}, http.StatusBadRequest, errors.New("roleId is required for role scope")
	}
	return service.MemoryExplorerQuery{
		ProjectID: projectID,
		Query:     query,
		Scope:     scope,
		Category:  strings.TrimSpace(c.QueryParam("category")),
		RoleID:    roleID,
		Tag:       strings.TrimSpace(c.QueryParam("tag")),
		StartAt:   startAt,
		EndAt:     endAt,
		Limit:     limit,
	}, http.StatusOK, nil
}

func parseOptionalRFC3339(raw string) (*time.Time, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func (h *MemoryHandler) writeMemoryError(c echo.Context, err error, fallbackStatus int, fallbackMessage string) error {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: err.Error()})
	case errors.Is(err, service.ErrMemoryAccessDenied):
		return c.JSON(http.StatusForbidden, model.ErrorResponse{Message: err.Error()})
	default:
		return c.JSON(fallbackStatus, model.ErrorResponse{Message: fallbackMessage})
	}
}
