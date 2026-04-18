// Package handler — audit_handler.go serves the project-scoped audit
// query API: list with cursor pagination and per-event detail. Both
// endpoints are gated by appMiddleware.Require(ActionAuditRead) at route
// registration time, so handlers can assume the caller has admin+.
package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
)

const (
	auditListDefaultLimit = 50
	auditListMaxLimit     = 200
)

// AuditQueryService is the contract the handler depends on. Implemented
// by *service.AuditService.
type AuditQueryService interface {
	Query(
		ctx context.Context,
		projectID uuid.UUID,
		filters model.AuditEventQueryFilters,
		cursor string,
		limit int,
	) ([]*model.AuditEvent, string, error)
	GetByID(ctx context.Context, projectID, eventID uuid.UUID) (*model.AuditEvent, error)
}

type AuditHandler struct {
	svc AuditQueryService
}

func NewAuditHandler(svc AuditQueryService) *AuditHandler {
	return &AuditHandler{svc: svc}
}

// List handles GET /projects/:pid/audit-events.
//
// Supported query params:
//   - actionId       — exact match against the canonical ActionID enum
//   - actorUserId    — exact match (UUID); 400 if malformed
//   - resourceType   — one of project|member|task|...; passed through
//   - resourceId     — exact match
//   - from / to      — RFC3339 timestamps; either or both
//   - cursor         — opaque cursor returned by a prior page
//   - limit          — 1..200, default 50
func (h *AuditHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	if projectID == uuid.Nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}

	filters := model.AuditEventQueryFilters{
		ActionID:     strings.TrimSpace(c.QueryParam("actionId")),
		ActorUserID:  strings.TrimSpace(c.QueryParam("actorUserId")),
		ResourceType: strings.TrimSpace(c.QueryParam("resourceType")),
		ResourceID:   strings.TrimSpace(c.QueryParam("resourceId")),
	}
	if v := strings.TrimSpace(c.QueryParam("from")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		}
		filters.From = &t
	}
	if v := strings.TrimSpace(c.QueryParam("to")); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		}
		filters.To = &t
	}

	limit := auditListDefaultLimit
	if v := strings.TrimSpace(c.QueryParam("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		}
		if n > auditListMaxLimit {
			n = auditListMaxLimit
		}
		limit = n
	}

	cursor := strings.TrimSpace(c.QueryParam("cursor"))

	events, nextCursor, err := h.svc.Query(c.Request().Context(), projectID, filters, cursor, limit)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
	}
	resp := model.AuditEventListResponse{
		Events:     make([]model.AuditEventDTO, 0, len(events)),
		NextCursor: nextCursor,
	}
	for _, e := range events {
		resp.Events = append(resp.Events, e.ToDTO())
	}
	return c.JSON(http.StatusOK, resp)
}

// Get handles GET /projects/:pid/audit-events/:eventId.
func (h *AuditHandler) Get(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	if projectID == uuid.Nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	eventID, err := uuid.Parse(c.Param("eventId"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	event, err := h.svc.GetByID(c.Request().Context(), projectID, eventID)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgNotFound)
	}
	if event == nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgNotFound)
	}
	return c.JSON(http.StatusOK, event.ToDTO())
}
