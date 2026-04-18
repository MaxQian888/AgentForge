// Package handler — permissions_handler.go serves
// GET /auth/me/projects/:pid/permissions, the canonical source of "what can
// the calling user do in this project". The frontend consumes this instead
// of duplicating the RBAC matrix client-side.
package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/i18n"
	appMiddleware "github.com/react-go-quick-starter/server/internal/middleware"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/repository"
)

type PermissionsHandler struct {
	memberRepo memberLookupForPermissions
}

type memberLookupForPermissions interface {
	GetByUserAndProject(ctx context.Context, userID, projectID uuid.UUID) (*model.Member, error)
}

type ProjectPermissionsResponse struct {
	ProjectID      string   `json:"projectId"`
	ProjectRole    string   `json:"projectRole"`
	AllowedActions []string `json:"allowedActions"`
}

func NewPermissionsHandler(memberRepo memberLookupForPermissions) *PermissionsHandler {
	return &PermissionsHandler{memberRepo: memberRepo}
}

// Get implements GET /auth/me/projects/:pid/permissions.
//
// Behavior:
//   - 403 not_a_project_member when the caller is not a member (does not leak
//     project existence).
//   - 200 with {projectRole, allowedActions} otherwise. Allowed actions are
//     derived from the server-side matrix at request time so the frontend
//     never out-of-syncs from a deploy.
func (h *PermissionsHandler) Get(c echo.Context) error {
	claims, err := appMiddleware.GetClaims(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	userID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	projectID, err := uuid.Parse(c.Param("pid"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}

	member, err := h.memberRepo.GetByUserAndProject(c.Request().Context(), userID, projectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return c.JSON(http.StatusForbidden, model.ErrorResponse{
				Message: i18nForbidden(c, i18n.MsgNotAProjectMember),
			})
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
	}
	if member == nil {
		return c.JSON(http.StatusForbidden, model.ErrorResponse{
			Message: i18nForbidden(c, i18n.MsgNotAProjectMember),
		})
	}

	role := model.NormalizeProjectRole(member.ProjectRole)
	actions := appMiddleware.AllowedActionsFor(role)
	out := ProjectPermissionsResponse{
		ProjectID:      projectID.String(),
		ProjectRole:    role,
		AllowedActions: make([]string, 0, len(actions)),
	}
	for _, a := range actions {
		out.AllowedActions = append(out.AllowedActions, string(a))
	}
	return c.JSON(http.StatusOK, out)
}
