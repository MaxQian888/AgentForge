package handler

import (
	"errors"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
)

type RoleHandler struct {
	store *rolepkg.FileStore
}

func NewRoleHandler(rolesDir string) *RoleHandler {
	return &RoleHandler{store: rolepkg.NewFileStore(rolesDir)}
}

func (h *RoleHandler) List(c echo.Context) error {
	roles, err := h.store.List()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load roles"})
	}
	return c.JSON(http.StatusOK, roles)
}

func (h *RoleHandler) Get(c echo.Context) error {
	roleID := c.Param("id")
	loadedRole, err := h.store.Get(roleID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "role not found"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to load role"})
	}
	return c.JSON(http.StatusOK, loadedRole)
}

func (h *RoleHandler) Create(c echo.Context) error {
	var manifest model.RoleManifest
	if err := c.Bind(&manifest); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if manifest.Metadata.ID == "" && manifest.Metadata.Name == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "role id or name is required"})
	}
	if err := h.store.Save((*rolepkg.Manifest)(&manifest)); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "failed to save role"})
	}
	loadedRole, err := h.store.Get(firstRoleID(manifest))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to reload role"})
	}
	return c.JSON(http.StatusCreated, loadedRole)
}

func (h *RoleHandler) Update(c echo.Context) error {
	roleID := c.Param("id")
	var manifest model.RoleManifest
	if err := c.Bind(&manifest); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	manifest.Metadata.ID = roleID
	if manifest.Metadata.Name == "" {
		manifest.Metadata.Name = roleID
	}
	if err := h.store.Save((*rolepkg.Manifest)(&manifest)); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "failed to save role"})
	}
	loadedRole, err := h.store.Get(roleID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to reload role"})
	}
	return c.JSON(http.StatusOK, loadedRole)
}

func firstRoleID(role model.RoleManifest) string {
	if role.Metadata.ID != "" {
		return role.Metadata.ID
	}
	return role.Metadata.Name
}
