// Package handler — project_template_handler.go serves the
// /project-templates and /projects/:pid/save-as-template endpoints.
//
// Access control:
//   - /project-templates CRUD is NOT project-scoped; it operates on a user's
//     personal template library. Authorization is caller == owner enforced at
//     service layer.
//   - /projects/:pid/save-as-template IS project-scoped; the router attaches
//     Require(ActionProjectSaveAsTemplate).
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/agentforge/server/internal/i18n"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ProjectTemplateService is the narrow contract the handler needs. Defined
// here (not in service) so tests can swap it for a fake.
type ProjectTemplateService interface {
	SaveAsTemplate(ctx context.Context, projectID, ownerID uuid.UUID, req model.CreateProjectTemplateRequest) (*model.ProjectTemplate, error)
	ListVisible(ctx context.Context, userID uuid.UUID) ([]*model.ProjectTemplate, error)
	Get(ctx context.Context, id, userID uuid.UUID) (*model.ProjectTemplate, error)
	UpdateMetadata(ctx context.Context, id, userID uuid.UUID, req model.UpdateProjectTemplateRequest) (*model.ProjectTemplate, error)
	Delete(ctx context.Context, id, userID uuid.UUID) error
	MaterializeMarketplaceInstall(ctx context.Context, installer uuid.UUID, name, description, snapshotJSON string, snapshotVersion int) (*model.ProjectTemplate, error)
}

type ProjectTemplateHandler struct {
	svc ProjectTemplateService
}

func NewProjectTemplateHandler(svc ProjectTemplateService) *ProjectTemplateHandler {
	return &ProjectTemplateHandler{svc: svc}
}

// List returns the templates visible to the caller.
// GET /project-templates
func (h *ProjectTemplateHandler) List(c echo.Context) error {
	userID, err := claimsUserID(c)
	if err != nil || userID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	templates, err := h.svc.ListVisible(c.Request().Context(), *userID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListProjectTemplates)
	}
	out := make([]model.ProjectTemplateDTO, 0, len(templates))
	for _, t := range templates {
		dto := t.ToDTO()
		// List responses omit the full snapshot to keep payloads small.
		dto.Snapshot = nil
		out = append(out, dto)
	}
	return c.JSON(http.StatusOK, model.ProjectTemplateListResponse{Templates: out})
}

// Get returns a single template with its snapshot payload.
// GET /project-templates/:id
func (h *ProjectTemplateHandler) Get(c echo.Context) error {
	userID, err := claimsUserID(c)
	if err != nil || userID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTemplateID)
	}
	tpl, err := h.svc.Get(c.Request().Context(), id, *userID)
	if err != nil {
		if errors.Is(err, service.ErrProjectTemplateNotFound) {
			return localizedError(c, http.StatusNotFound, i18n.MsgProjectTemplateNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
	}
	dto := tpl.ToDTO()
	if snap, parseErr := model.ParseProjectTemplateSnapshot(tpl.SnapshotJSON); parseErr == nil {
		dto.Snapshot = snap
	}
	return c.JSON(http.StatusOK, dto)
}

// Update edits metadata (name/description) on a user-source template.
// PUT /project-templates/:id
func (h *ProjectTemplateHandler) Update(c echo.Context) error {
	userID, err := claimsUserID(c)
	if err != nil || userID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTemplateID)
	}
	req := new(model.UpdateProjectTemplateRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	tpl, err := h.svc.UpdateMetadata(c.Request().Context(), id, *userID, *req)
	if err != nil {
		return templateServiceError(c, err)
	}
	return c.JSON(http.StatusOK, tpl.ToDTO())
}

// Delete removes a user-source or marketplace-source template.
// DELETE /project-templates/:id
func (h *ProjectTemplateHandler) Delete(c echo.Context) error {
	userID, err := claimsUserID(c)
	if err != nil || userID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTemplateID)
	}
	if err := h.svc.Delete(c.Request().Context(), id, *userID); err != nil {
		return templateServiceError(c, err)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "template deleted"})
}

// SaveAsTemplate builds a snapshot of the project at :pid and persists it
// as a user template owned by the caller. RBAC: admin+ on project.
// POST /projects/:pid/save-as-template
func (h *ProjectTemplateHandler) SaveAsTemplate(c echo.Context) error {
	userID, err := claimsUserID(c)
	if err != nil || userID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	req := new(model.CreateProjectTemplateRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	tpl, err := h.svc.SaveAsTemplate(c.Request().Context(), projectID, *userID, *req)
	if err != nil {
		return templateServiceError(c, err)
	}
	return c.JSON(http.StatusCreated, tpl.ToDTO())
}

// templateServiceError maps service-layer errors to localized HTTP responses.
func templateServiceError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, service.ErrProjectTemplateNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgProjectTemplateNotFound)
	case errors.Is(err, service.ErrProjectTemplateImmutableSystem):
		return localizedError(c, http.StatusForbidden, i18n.MsgProjectTemplateImmutableSystem)
	case errors.Is(err, service.ErrProjectTemplateOwnerMismatch):
		return localizedError(c, http.StatusForbidden, i18n.MsgProjectTemplateOwnerMismatch)
	case errors.Is(err, service.ErrProjectTemplateSnapshotInvalid):
		return localizedError(c, http.StatusUnprocessableEntity, i18n.MsgProjectTemplateSnapshotInvalid)
	case errors.Is(err, service.ErrProjectTemplateSnapshotTooLarge):
		return localizedError(c, http.StatusRequestEntityTooLarge, i18n.MsgProjectTemplateSnapshotTooBig)
	default:
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToSaveProjectTemplate)
	}
}
