package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/vcs"
)

// vcsIntegrationsService is the narrow surface the handler consumes.
// The concrete implementation is *vcs.Service.
type vcsIntegrationsService interface {
	Create(ctx context.Context, in vcs.CreateInput) (*model.VCSIntegration, error)
	Patch(ctx context.Context, id uuid.UUID, in vcs.PatchInput) (*model.VCSIntegration, error)
	Delete(ctx context.Context, id uuid.UUID, actor *uuid.UUID) error
	List(ctx context.Context, projectID uuid.UUID) ([]*model.VCSIntegration, error)
	QueueSync(ctx context.Context, id uuid.UUID, actor *uuid.UUID) (*model.VCSIntegration, error)
}

// VCSIntegrationsHandler exposes /vcs-integrations CRUD.
type VCSIntegrationsHandler struct{ svc vcsIntegrationsService }

// NewVCSIntegrationsHandler returns a wired handler.
func NewVCSIntegrationsHandler(svc vcsIntegrationsService) *VCSIntegrationsHandler {
	return &VCSIntegrationsHandler{svc: svc}
}

// Register attaches both project-scoped and id-scoped routes.
// projectGroup carries the :pid param + project RBAC; protected is the
// top-level authenticated group used for /vcs-integrations/:id.
func (h *VCSIntegrationsHandler) Register(projectGroup *echo.Group, protected *echo.Group) {
	projectGroup.GET("/vcs-integrations", h.List, appMiddleware.Require(appMiddleware.ActionVCSIntegrationRead))
	projectGroup.POST("/vcs-integrations", h.Create, appMiddleware.Require(appMiddleware.ActionVCSIntegrationCreate))
	protected.PATCH("/vcs-integrations/:id", h.Patch)
	protected.DELETE("/vcs-integrations/:id", h.Delete)
	protected.POST("/vcs-integrations/:id/sync", h.Sync)
}

type createIntegrationRequest struct {
	Provider         string  `json:"provider"`
	Host             string  `json:"host"`
	Owner            string  `json:"owner"`
	Repo             string  `json:"repo"`
	DefaultBranch    string  `json:"defaultBranch"`
	TokenSecretRef   string  `json:"tokenSecretRef"`
	WebhookSecretRef string  `json:"webhookSecretRef"`
	ActingEmployeeID *string `json:"actingEmployeeId"`
}

// List handles GET /api/v1/projects/:pid/vcs-integrations.
func (h *VCSIntegrationsHandler) List(c echo.Context) error {
	pid := appMiddleware.GetProjectID(c)
	if pid == uuid.Nil {
		// Fallback to URL param when project middleware did not run
		// (handler-level tests construct context manually).
		parsed, err := uuid.Parse(c.Param("pid"))
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		}
		pid = parsed
	}
	rows, err := h.svc.List(c.Request().Context(), pid)
	if err != nil {
		return mapVCSError(c, err)
	}
	if rows == nil {
		rows = []*model.VCSIntegration{}
	}
	return c.JSON(http.StatusOK, rows)
}

// Create handles POST /api/v1/projects/:pid/vcs-integrations.
func (h *VCSIntegrationsHandler) Create(c echo.Context) error {
	pid, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid project id"})
	}
	req := new(createIntegrationRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	if req.Provider == "" || req.Host == "" || req.Owner == "" || req.Repo == "" ||
		req.TokenSecretRef == "" || req.WebhookSecretRef == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provider/host/owner/repo/tokenSecretRef/webhookSecretRef are required"})
	}
	if req.Provider != "github" && req.Provider != "gitlab" && req.Provider != "gitea" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "vcs:provider_unsupported"})
	}
	in := vcs.CreateInput{
		ProjectID:        pid,
		Provider:         req.Provider,
		Host:             req.Host,
		Owner:            req.Owner,
		Repo:             req.Repo,
		DefaultBranch:    req.DefaultBranch,
		TokenSecretRef:   req.TokenSecretRef,
		WebhookSecretRef: req.WebhookSecretRef,
		Actor:            callerUserPtr(c),
	}
	if req.ActingEmployeeID != nil && *req.ActingEmployeeID != "" {
		id, err := uuid.Parse(*req.ActingEmployeeID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid actingEmployeeId"})
		}
		in.ActingEmployeeID = &id
	}
	rec, err := h.svc.Create(c.Request().Context(), in)
	if err != nil {
		return mapVCSError(c, err)
	}
	return c.JSON(http.StatusCreated, rec)
}

type patchIntegrationRequest struct {
	Status           *string `json:"status"`
	TokenSecretRef   *string `json:"tokenSecretRef"`
	ActingEmployeeID *string `json:"actingEmployeeId"`
}

// Patch handles PATCH /api/v1/vcs-integrations/:id.
func (h *VCSIntegrationsHandler) Patch(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	req := new(patchIntegrationRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid body"})
	}
	in := vcs.PatchInput{Status: req.Status, TokenSecretRef: req.TokenSecretRef, Actor: callerUserPtr(c)}
	if req.ActingEmployeeID != nil && *req.ActingEmployeeID != "" {
		empID, perr := uuid.Parse(*req.ActingEmployeeID)
		if perr != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid actingEmployeeId"})
		}
		in.ActingEmployeeID = &empID
	}
	rec, err := h.svc.Patch(c.Request().Context(), id, in)
	if err != nil {
		return mapVCSError(c, err)
	}
	return c.JSON(http.StatusOK, rec)
}

// Delete handles DELETE /api/v1/vcs-integrations/:id.
func (h *VCSIntegrationsHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if err := h.svc.Delete(c.Request().Context(), id, callerUserPtr(c)); err != nil {
		return mapVCSError(c, err)
	}
	return c.NoContent(http.StatusNoContent)
}

// Sync handles POST /api/v1/vcs-integrations/:id/sync. Returns 202 to
// the FE while the background sync (Spec 2B) lands; v1 only stamps
// last_synced_at.
func (h *VCSIntegrationsHandler) Sync(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	rec, err := h.svc.QueueSync(c.Request().Context(), id, callerUserPtr(c))
	if err != nil {
		return mapVCSError(c, err)
	}
	return c.JSON(http.StatusAccepted, map[string]any{
		"integration": rec,
		"note":        "background sync pending — full implementation in Spec 2B",
	})
}

func mapVCSError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, vcs.ErrProviderUnsupported):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "vcs:provider_unsupported"})
	case errors.Is(err, vcs.ErrSecretNotResolvable):
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "vcs:secret_not_resolvable"})
	case errors.Is(err, vcs.ErrPublicBaseURLNotConfigured):
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "vcs:public_base_url_not_configured"})
	case errors.Is(err, vcs.ErrAuthExpired):
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "vcs:auth_expired"})
	}
	return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

// callerUserPtr extracts the actor uuid pointer from JWT claims. nil
// when claims are absent — Require() should already have rejected
// unauthenticated callers, this is defense-in-depth.
func callerUserPtr(c echo.Context) *uuid.UUID {
	claims, err := appMiddleware.GetClaims(c)
	if err != nil || claims == nil {
		return nil
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil {
		return nil
	}
	return &id
}
