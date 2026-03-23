package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/model"
	"gopkg.in/yaml.v3"
)

type RoleHandler struct {
	rolesDir string
}

func NewRoleHandler(rolesDir string) *RoleHandler {
	return &RoleHandler{rolesDir: rolesDir}
}

var presetRoles = []model.RoleManifest{
	{
		Metadata: model.RoleMetadata{
			Name:        "backend-developer",
			Version:     "1.0.0",
			Description: "Backend developer specializing in Go and API development",
			Tags:        []string{"go", "api", "backend"},
		},
		Identity: model.RoleIdentity{
			Persona: "Senior backend developer",
			Goals:   []string{"Write clean, tested Go code", "Design RESTful APIs"},
		},
		Capabilities: model.RoleCapabilities{
			Languages:  []string{"go"},
			Frameworks: []string{"echo", "gin"},
		},
	},
	{
		Metadata: model.RoleMetadata{
			Name:        "frontend-developer",
			Version:     "1.0.0",
			Description: "Frontend developer specializing in React and TypeScript",
			Tags:        []string{"react", "typescript", "frontend"},
		},
		Identity: model.RoleIdentity{
			Persona: "Senior frontend developer",
			Goals:   []string{"Build responsive UIs", "Write type-safe code"},
		},
		Capabilities: model.RoleCapabilities{
			Languages:  []string{"typescript", "javascript"},
			Frameworks: []string{"react", "nextjs"},
		},
	},
	{
		Metadata: model.RoleMetadata{
			Name:        "code-reviewer",
			Version:     "1.0.0",
			Description: "Code review specialist focused on quality and best practices",
			Tags:        []string{"review", "quality"},
		},
		Identity: model.RoleIdentity{
			Persona: "Code review specialist",
			Goals:   []string{"Ensure code quality", "Identify bugs and security issues"},
		},
	},
}

func (h *RoleHandler) List(c echo.Context) error {
	roles := h.loadRoles()
	return c.JSON(http.StatusOK, roles)
}

func (h *RoleHandler) Get(c echo.Context) error {
	roleID := c.Param("id")
	roles := h.loadRoles()
	for _, r := range roles {
		if r.Metadata.Name == roleID {
			return c.JSON(http.StatusOK, r)
		}
	}
	return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "role not found"})
}

func (h *RoleHandler) Create(c echo.Context) error {
	var role model.RoleManifest
	if err := c.Bind(&role); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if role.Metadata.Name == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "role name is required"})
	}

	if err := os.MkdirAll(h.rolesDir, 0o755); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create roles directory"})
	}

	data, err := yaml.Marshal(&role)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to marshal role"})
	}

	filePath := filepath.Join(h.rolesDir, role.Metadata.Name+".yaml")
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to save role"})
	}

	return c.JSON(http.StatusCreated, role)
}

func (h *RoleHandler) Update(c echo.Context) error {
	roleID := c.Param("id")
	var role model.RoleManifest
	if err := c.Bind(&role); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	role.Metadata.Name = roleID

	if err := os.MkdirAll(h.rolesDir, 0o755); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create roles directory"})
	}

	data, err := yaml.Marshal(&role)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to marshal role"})
	}

	filePath := filepath.Join(h.rolesDir, roleID+".yaml")
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to save role"})
	}

	return c.JSON(http.StatusOK, role)
}

func (h *RoleHandler) loadRoles() []model.RoleManifest {
	roles := make([]model.RoleManifest, len(presetRoles))
	copy(roles, presetRoles)

	entries, err := os.ReadDir(h.rolesDir)
	if err != nil {
		return roles
	}

	for _, entry := range entries {
		if entry.IsDir() || (!strings.HasSuffix(entry.Name(), ".yaml") && !strings.HasSuffix(entry.Name(), ".yml")) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(h.rolesDir, entry.Name()))
		if err != nil {
			continue
		}
		var role model.RoleManifest
		if err := yaml.Unmarshal(data, &role); err != nil {
			continue
		}
		// Replace preset if same name, otherwise append
		replaced := false
		for i, preset := range roles {
			if preset.Metadata.Name == role.Metadata.Name {
				roles[i] = role
				replaced = true
				break
			}
		}
		if !replaced {
			roles = append(roles, role)
		}
	}

	return roles
}
