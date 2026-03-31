package handler

import (
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/agentforge/marketplace/internal/model"
	"github.com/agentforge/marketplace/internal/repository"
	"github.com/agentforge/marketplace/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// VersionHandler handles marketplace item version endpoints.
type VersionHandler struct {
	svc *service.MarketplaceService
}

// NewVersionHandler creates a new VersionHandler.
func NewVersionHandler(svc *service.MarketplaceService) *VersionHandler {
	return &VersionHandler{svc: svc}
}

// List handles GET /api/v1/items/:id/versions
func (h *VersionHandler) List(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}
	versions, err := h.svc.GetVersions(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Item not found")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.JSON(http.StatusOK, versions)
}

// Upload handles POST /api/v1/items/:id/versions (multipart)
func (h *VersionHandler) Upload(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}

	uploaderID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, "Unauthorized")
	}

	var req model.CreateVersionRequest
	if err := c.Bind(&req); err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid request body")
	}
	if err := c.Validate(&req); err != nil {
		return localizedError(c, http.StatusUnprocessableEntity, err.Error())
	}

	file, err := c.FormFile("artifact")
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Artifact file is required")
	}

	src, err := file.Open()
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, "Failed to read artifact file")
	}
	defer src.Close()

	v, err := h.svc.PublishVersion(c.Request().Context(), id, uploaderID, req, src, file.Size)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Item not found")
		}
		if errors.Is(err, service.ErrNotItemOwner) {
			return localizedError(c, http.StatusForbidden, "You are not the owner of this item")
		}
		if errors.Is(err, service.ErrInvalidSemver) {
			return localizedError(c, http.StatusBadRequest, "Invalid semantic version format")
		}
		return localizedError(c, http.StatusInternalServerError, "Failed to store artifact")
	}
	return c.JSON(http.StatusCreated, v)
}

// Yank handles POST /api/v1/items/:id/versions/:ver/yank
func (h *VersionHandler) Yank(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}
	version := c.Param("ver")

	requesterID, err := claimsUserID(c)
	if err != nil {
		return localizedError(c, http.StatusUnauthorized, "Unauthorized")
	}

	if err := h.svc.YankVersion(c.Request().Context(), id, requesterID, version); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return localizedError(c, http.StatusNotFound, "Version not found")
		}
		if errors.Is(err, service.ErrNotItemOwner) {
			return localizedError(c, http.StatusForbidden, "You are not the owner of this item")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}
	return c.NoContent(http.StatusNoContent)
}

// Download handles GET /api/v1/items/:id/versions/:ver/download
func (h *VersionHandler) Download(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, "Invalid ID")
	}
	version := c.Param("ver")

	path, digest, err := h.svc.GetVersionDownloadPath(c.Request().Context(), id, version)
	if err != nil {
		if errors.Is(err, service.ErrVersionNotFound) {
			return localizedError(c, http.StatusNotFound, "Version not found")
		}
		if errors.Is(err, service.ErrVersionYanked) {
			return localizedError(c, http.StatusGone, "This version has been yanked and is unavailable")
		}
		return localizedError(c, http.StatusInternalServerError, "Internal server error")
	}

	// Best-effort download count increment.
	_ = h.svc.IncrementDownload(c.Request().Context(), id)

	f, err := os.Open(path)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, "Failed to open artifact")
	}
	defer f.Close()

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s-%s.artifact"`, id, version))
	c.Response().Header().Set("X-Content-Digest", digest)
	return c.Stream(http.StatusOK, "application/octet-stream", f)
}
