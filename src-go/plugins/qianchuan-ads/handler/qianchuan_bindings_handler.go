package qchandler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/react-go-quick-starter/server/internal/adsplatform"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/plugins/qianchuan-ads/binding"
)

// QianchuanBindingsService is the narrow contract the handler depends on.
type QianchuanBindingsService interface {
	Create(ctx context.Context, in qianchuanbinding.CreateInput) (*qianchuanbinding.Record, error)
	Get(ctx context.Context, id uuid.UUID) (*qianchuanbinding.Record, error)
	List(ctx context.Context, projectID uuid.UUID) ([]*qianchuanbinding.Record, error)
	Update(ctx context.Context, id uuid.UUID, in qianchuanbinding.UpdateInput) (*qianchuanbinding.Record, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Sync(ctx context.Context, id uuid.UUID) error
	Test(ctx context.Context, id uuid.UUID) (*adsplatform.MetricSnapshot, error)
}

// QianchuanBindingsAuditEmitter is the narrow audit contract.
type QianchuanBindingsAuditEmitter interface {
	Emit(ctx context.Context, projectID, actorUserID, bindingID uuid.UUID, action, payload string)
}

// QianchuanBindingsHandler exposes /api/v1/.../qianchuan/bindings endpoints.
type QianchuanBindingsHandler struct {
	service QianchuanBindingsService
	audit   QianchuanBindingsAuditEmitter
}

// NewQianchuanBindingsHandler wires the handler.
func NewQianchuanBindingsHandler(svc QianchuanBindingsService, audit QianchuanBindingsAuditEmitter) *QianchuanBindingsHandler {
	return &QianchuanBindingsHandler{service: svc, audit: audit}
}

// Register attaches routes onto a project-scoped Echo group (/api/v1/projects/:pid).
func (h *QianchuanBindingsHandler) Register(g *echo.Group) {
	g.GET("/qianchuan/bindings", h.List, appMiddleware.Require(appMiddleware.ActionQianchuanBindingRead))
	g.POST("/qianchuan/bindings", h.Create, appMiddleware.Require(appMiddleware.ActionQianchuanBindingCreate))
}

// RegisterFlat attaches the per-binding endpoints (project-id is read from the row).
func (h *QianchuanBindingsHandler) RegisterFlat(e *echo.Echo) {
	g := e.Group("/api/v1/qianchuan/bindings")
	g.PATCH("/:id", h.Update, appMiddleware.Require(appMiddleware.ActionQianchuanBindingUpdate))
	g.DELETE("/:id", h.Delete, appMiddleware.Require(appMiddleware.ActionQianchuanBindingDelete))
	g.POST("/:id/sync", h.Sync, appMiddleware.Require(appMiddleware.ActionQianchuanBindingSync))
	g.POST("/:id/test", h.Test, appMiddleware.Require(appMiddleware.ActionQianchuanBindingTest))
}

// ---------------- request / response shapes ----------------

type createBindingRequest struct {
	AdvertiserID          string  `json:"advertiser_id"`
	AwemeID               string  `json:"aweme_id"`
	DisplayName           string  `json:"display_name"`
	ActingEmployeeID      *string `json:"acting_employee_id,omitempty"`
	AccessTokenSecretRef  string  `json:"access_token_secret_ref"`
	RefreshTokenSecretRef string  `json:"refresh_token_secret_ref"`
}

type updateBindingRequest struct {
	DisplayName      *string `json:"display_name,omitempty"`
	Status           *string `json:"status,omitempty"`
	ActingEmployeeID *string `json:"acting_employee_id,omitempty"`
}

type bindingDTO struct {
	ID                    uuid.UUID  `json:"id"`
	ProjectID             uuid.UUID  `json:"project_id"`
	AdvertiserID          string     `json:"advertiser_id"`
	AwemeID               string     `json:"aweme_id,omitempty"`
	DisplayName           string     `json:"display_name,omitempty"`
	Status                string     `json:"status"`
	ActingEmployeeID      *uuid.UUID `json:"acting_employee_id,omitempty"`
	AccessTokenSecretRef  string     `json:"access_token_secret_ref"`
	RefreshTokenSecretRef string     `json:"refresh_token_secret_ref"`
	TokenExpiresAt        any        `json:"token_expires_at,omitempty"`
	LastSyncedAt          any        `json:"last_synced_at,omitempty"`
}

func toBindingDTO(r *qianchuanbinding.Record) *bindingDTO {
	return &bindingDTO{
		ID: r.ID, ProjectID: r.ProjectID, AdvertiserID: r.AdvertiserID, AwemeID: r.AwemeID,
		DisplayName: r.DisplayName, Status: r.Status, ActingEmployeeID: r.ActingEmployeeID,
		AccessTokenSecretRef: r.AccessTokenSecretRef, RefreshTokenSecretRef: r.RefreshTokenSecretRef,
		TokenExpiresAt: r.TokenExpiresAt, LastSyncedAt: r.LastSyncedAt,
	}
}

// ---------------- endpoints ----------------

// List GET /api/v1/projects/:pid/qianchuan/bindings.
func (h *QianchuanBindingsHandler) List(c echo.Context) error {
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	rows, err := h.service.List(c.Request().Context(), projectID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
	}
	out := make([]*bindingDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toBindingDTO(r))
	}
	return c.JSON(http.StatusOK, out)
}

// Create POST /api/v1/projects/:pid/qianchuan/bindings.
func (h *QianchuanBindingsHandler) Create(c echo.Context) error {
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	var req createBindingRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.AdvertiserID == "" || req.AccessTokenSecretRef == "" || req.RefreshTokenSecretRef == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	actor := actorUserID(c)
	var actingEmp *uuid.UUID
	if req.ActingEmployeeID != nil {
		id, err := uuid.Parse(*req.ActingEmployeeID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		}
		actingEmp = &id
	}
	rec, err := h.service.Create(c.Request().Context(), qianchuanbinding.CreateInput{
		ProjectID:             projectID,
		AdvertiserID:          req.AdvertiserID,
		AwemeID:               req.AwemeID,
		DisplayName:           req.DisplayName,
		ActingEmployeeID:      actingEmp,
		AccessTokenSecretRef:  req.AccessTokenSecretRef,
		RefreshTokenSecretRef: req.RefreshTokenSecretRef,
		CreatedBy:             actor,
	})
	if err != nil {
		return mapBindingError(c, err)
	}
	h.emitAudit(c.Request().Context(), projectID, actor, rec.ID, "qianchuan_binding.create", "")
	return c.JSON(http.StatusCreated, toBindingDTO(rec))
}

// Update PATCH /api/v1/qianchuan/bindings/:id.
func (h *QianchuanBindingsHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	var req updateBindingRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	in := qianchuanbinding.UpdateInput{DisplayName: req.DisplayName, Status: req.Status}
	if req.ActingEmployeeID != nil {
		empID, err := uuid.Parse(*req.ActingEmployeeID)
		if err != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
		}
		in.ActingEmployeeID = &empID
	}
	rec, err := h.service.Update(c.Request().Context(), id, in)
	if err != nil {
		return mapBindingError(c, err)
	}
	h.emitAudit(c.Request().Context(), rec.ProjectID, actorUserID(c), rec.ID, "qianchuan_binding.update", "")
	return c.JSON(http.StatusOK, toBindingDTO(rec))
}

// Delete DELETE /api/v1/qianchuan/bindings/:id.
func (h *QianchuanBindingsHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	rec, err := h.service.Get(c.Request().Context(), id)
	if err != nil {
		return mapBindingError(c, err)
	}
	if err := h.service.Delete(c.Request().Context(), id); err != nil {
		return mapBindingError(c, err)
	}
	h.emitAudit(c.Request().Context(), rec.ProjectID, actorUserID(c), id, "qianchuan_binding.delete", "")
	return c.NoContent(http.StatusNoContent)
}

// Sync POST /api/v1/qianchuan/bindings/:id/sync.
func (h *QianchuanBindingsHandler) Sync(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := h.service.Sync(c.Request().Context(), id); err != nil {
		return mapBindingError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// Test POST /api/v1/qianchuan/bindings/:id/test.
func (h *QianchuanBindingsHandler) Test(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	snap, err := h.service.Test(c.Request().Context(), id)
	if err != nil {
		return mapBindingError(c, err)
	}
	return c.JSON(http.StatusOK, snap)
}

// mapBindingError converts service-layer errors to HTTP status + i18n message key.
func mapBindingError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, qianchuanbinding.ErrAdvertiserAlreadyBound):
		return localizedError(c, http.StatusConflict, i18n.MsgQianchuanAdvertiserAlreadyBound)
	case errors.Is(err, qianchuanbinding.ErrSecretMissing):
		return localizedError(c, http.StatusBadRequest, i18n.MsgQianchuanSecretMissing)
	case errors.Is(err, qianchuanbinding.ErrNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgQianchuanBindingNotFound)
	case errors.Is(err, adsplatform.ErrAuthExpired):
		return localizedError(c, http.StatusUnauthorized, i18n.MsgQianchuanAuthExpired)
	case errors.Is(err, adsplatform.ErrRateLimited):
		return localizedError(c, http.StatusTooManyRequests, i18n.MsgQianchuanRateLimited)
	default:
		return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
	}
}

func (h *QianchuanBindingsHandler) emitAudit(ctx context.Context, projectID, actor, bindingID uuid.UUID, action, payload string) {
	if h.audit == nil {
		return
	}
	h.audit.Emit(ctx, projectID, actor, bindingID, action, payload)
}

// actorUserID is supplied by middleware; falls back to nil-uuid in tests.
func actorUserID(c echo.Context) uuid.UUID {
	id, err := claimsUserID(c)
	if err != nil || id == nil {
		return uuid.Nil
	}
	return *id
}
