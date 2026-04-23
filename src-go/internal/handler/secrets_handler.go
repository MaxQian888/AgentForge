package handler

import (
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/secrets"
)

// secretsService is the narrow contract SecretsHandler needs. echo.Context
// is passed through so the implementation can derive cancellation +
// request-scoped values without a separate context.Context arg.
type secretsService interface {
	CreateSecret(c echo.Context, projectID uuid.UUID, name, plaintext, description string, actor uuid.UUID) (*secrets.Record, error)
	RotateSecret(c echo.Context, projectID uuid.UUID, name, plaintext string, actor uuid.UUID) error
	DeleteSecret(c echo.Context, projectID uuid.UUID, name string, actor uuid.UUID) error
	ListSecrets(c echo.Context, projectID uuid.UUID) ([]*secrets.Record, error)
}

// SecretsHandler exposes project-scoped CRUD for encrypted secrets.
// Secret values are accepted on Create + Rotate but never returned in API
// responses. List/Get also return metadata only.
type SecretsHandler struct{ svc secretsService }

// NewSecretsHandler returns a handler backed by the given service.
func NewSecretsHandler(svc secretsService) *SecretsHandler {
	return &SecretsHandler{svc: svc}
}

// Register attaches secret routes under a project-scoped Echo group
// already protected by ProjectMiddleware + JWT.
func (h *SecretsHandler) Register(g *echo.Group) {
	g.GET("/secrets", h.List, appMiddleware.Require(appMiddleware.ActionSecretRead))
	g.POST("/secrets", h.Create, appMiddleware.Require(appMiddleware.ActionSecretWrite))
	g.PATCH("/secrets/:name", h.Rotate, appMiddleware.Require(appMiddleware.ActionSecretWrite))
	g.DELETE("/secrets/:name", h.Delete, appMiddleware.Require(appMiddleware.ActionSecretWrite))
}

type secretMetadataDTO struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	LastUsedAt  *string `json:"lastUsedAt,omitempty"`
	CreatedBy   string  `json:"createdBy"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

func toMetadataDTO(r *secrets.Record) secretMetadataDTO {
	const layout = "2006-01-02T15:04:05Z07:00"
	dto := secretMetadataDTO{
		Name:        r.Name,
		Description: r.Description,
		CreatedBy:   r.CreatedBy.String(),
		CreatedAt:   r.CreatedAt.UTC().Format(layout),
		UpdatedAt:   r.UpdatedAt.UTC().Format(layout),
	}
	if r.LastUsedAt != nil {
		s := r.LastUsedAt.UTC().Format(layout)
		dto.LastUsedAt = &s
	}
	return dto
}

// List handles GET /api/v1/projects/:pid/secrets — metadata only.
func (h *SecretsHandler) List(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	rows, err := h.svc.ListSecrets(c, projectID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "secret:list_failed"})
	}
	out := make([]secretMetadataDTO, 0, len(rows))
	for _, r := range rows {
		out = append(out, toMetadataDTO(r))
	}
	return c.JSON(http.StatusOK, out)
}

type createSecretReq struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type createSecretResp struct {
	secretMetadataDTO
}

// Create handles POST /api/v1/projects/:pid/secrets.
func (h *SecretsHandler) Create(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	req := new(createSecretReq)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	}
	if req.Name == "" || req.Value == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name_and_value_required"})
	}
	actor := callerUserIDForSecrets(c)
	rec, err := h.svc.CreateSecret(c, projectID, req.Name, req.Value, req.Description, actor)
	if err != nil {
		if errors.Is(err, secrets.ErrNameConflict) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "secret:name_conflict"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "secret:create_failed"})
	}
	return c.JSON(http.StatusCreated, createSecretResp{
		secretMetadataDTO: toMetadataDTO(rec),
	})
}

type rotateSecretReq struct {
	Value       *string `json:"value,omitempty"`
	Description *string `json:"description,omitempty"`
}

// Rotate handles PATCH /api/v1/projects/:pid/secrets/:name. Only `value`
// is wired in 1B; `description` updates land in a follow-up PR (out of
// scope for this slice but kept in the request shape so the FE can
// already send it).
func (h *SecretsHandler) Rotate(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_name"})
	}
	req := new(rotateSecretReq)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	}
	if req.Value == nil || *req.Value == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "value_required"})
	}
	actor := callerUserIDForSecrets(c)
	if err := h.svc.RotateSecret(c, projectID, name, *req.Value, actor); err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "secret:not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "secret:rotate_failed"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"name":    name,
		"rotated": true,
	})
}

// Delete handles DELETE /api/v1/projects/:pid/secrets/:name.
func (h *SecretsHandler) Delete(c echo.Context) error {
	projectID := appMiddleware.GetProjectID(c)
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "missing_name"})
	}
	actor := callerUserIDForSecrets(c)
	if err := h.svc.DeleteSecret(c, projectID, name, actor); err != nil {
		if errors.Is(err, secrets.ErrSecretNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "secret:not_found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "secret:delete_failed"})
	}
	return c.NoContent(http.StatusNoContent)
}

// callerUserIDForSecrets extracts the actor uuid from JWT claims. Returns
// uuid.Nil when claims are absent — Require() should have already rejected
// such requests, so this is a defense-in-depth fallback.
func callerUserIDForSecrets(c echo.Context) uuid.UUID {
	claims, err := appMiddleware.GetClaims(c)
	if err != nil || claims == nil {
		return uuid.Nil
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil {
		return uuid.Nil
	}
	return id
}

// EchoSecretsServiceAdapter adapts the context.Context-based secrets.Service
// to the echo.Context-based handler contract.
type EchoSecretsServiceAdapter struct{ S *secrets.Service }

// CreateSecret forwards to secrets.Service.CreateSecret.
func (a *EchoSecretsServiceAdapter) CreateSecret(c echo.Context, projectID uuid.UUID, name, plaintext, description string, actor uuid.UUID) (*secrets.Record, error) {
	return a.S.CreateSecret(c.Request().Context(), projectID, name, plaintext, description, actor)
}

// RotateSecret forwards to secrets.Service.RotateSecret.
func (a *EchoSecretsServiceAdapter) RotateSecret(c echo.Context, projectID uuid.UUID, name, plaintext string, actor uuid.UUID) error {
	return a.S.RotateSecret(c.Request().Context(), projectID, name, plaintext, actor)
}

// DeleteSecret forwards to secrets.Service.DeleteSecret.
func (a *EchoSecretsServiceAdapter) DeleteSecret(c echo.Context, projectID uuid.UUID, name string, actor uuid.UUID) error {
	return a.S.DeleteSecret(c.Request().Context(), projectID, name, actor)
}

// ListSecrets forwards to secrets.Service.ListSecrets.
func (a *EchoSecretsServiceAdapter) ListSecrets(c echo.Context, projectID uuid.UUID) ([]*secrets.Record, error) {
	return a.S.ListSecrets(c.Request().Context(), projectID)
}
