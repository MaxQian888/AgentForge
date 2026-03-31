package handler

import (
	"errors"
	"net/http"

	"github.com/agentforge/marketplace/internal/repository"
	"github.com/agentforge/marketplace/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AdminHandler handles admin-only marketplace endpoints.
type AdminHandler struct {
	svc          *service.MarketplaceService
	adminUserIDs []string
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(svc *service.MarketplaceService, adminUserIDs []string) *AdminHandler {
	return &AdminHandler{svc: svc, adminUserIDs: adminUserIDs}
}

// isAdmin checks whether the claims user is in the admin list.
func (h *AdminHandler) isAdmin(c echo.Context) bool {
	userID, err := claimsUserID(c)
	if err != nil {
		return false
	}
	userStr := userID.String()
	for _, id := range h.adminUserIDs {
		if id == userStr {
			return true
		}
	}
	return false
}

// Feature handles POST /admin/items/:id/feature
func (h *AdminHandler) Feature(c echo.Context) error {
	if !h.isAdmin(c) {
		return localizedError(c, http.StatusForbidden, "Admin privileges required")
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}
	if err := h.svc.AdminFeature(c.Request().Context(), id, true); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Item not found")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, map[string]bool{"featured": true})
}

// Verify handles POST /admin/items/:id/verify
func (h *AdminHandler) Verify(c echo.Context) error {
	if !h.isAdmin(c) {
		return localizedError(c, http.StatusForbidden, "Admin privileges required")
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}
	if err := h.svc.AdminVerify(c.Request().Context(), id, true); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Item not found")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, map[string]bool{"verified": true})
}
