package handler

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
)

type RoleHandler struct {
	store        *rolepkg.FileStore
	bridgeClient roleAuthoringBridgeClient
}

func NewRoleHandler(rolesDir string) *RoleHandler {
	return &RoleHandler{store: rolepkg.NewFileStore(rolesDir)}
}

type roleAuthoringBridgeClient interface {
	GetRuntimeCatalog(ctx context.Context) (*bridge.RuntimeCatalogResponse, error)
	Generate(ctx context.Context, req bridge.GenerateRequest) (*bridge.GenerateResponse, error)
}

type rolePreviewRequest struct {
	RoleID string              `json:"roleId"`
	Draft  *model.RoleManifest `json:"draft,omitempty"`
}

type rolePreviewResponse struct {
	NormalizedManifest   *model.RoleManifest           `json:"normalizedManifest,omitempty"`
	EffectiveManifest    *model.RoleManifest           `json:"effectiveManifest,omitempty"`
	ExecutionProfile     *rolepkg.ExecutionProfile     `json:"executionProfile,omitempty"`
	ValidationIssues     []roleValidationIssue         `json:"validationIssues,omitempty"`
	Inheritance          *roleInheritanceSummary       `json:"inheritance,omitempty"`
	ReadinessDiagnostics []bridge.RuntimeDiagnosticDTO `json:"readinessDiagnostics,omitempty"`
	Selection            *roleSandboxSelection         `json:"selection,omitempty"`
	Probe                *bridge.GenerateResponse      `json:"probe,omitempty"`
}

type roleValidationIssue struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type roleInheritanceSummary struct {
	ParentRoleID string `json:"parentRoleId,omitempty"`
}

type roleSandboxRequest struct {
	RoleID      string              `json:"roleId"`
	Draft       *model.RoleManifest `json:"draft,omitempty"`
	Input       string              `json:"input"`
	Runtime     string              `json:"runtime,omitempty"`
	Provider    string              `json:"provider,omitempty"`
	Model       string              `json:"model,omitempty"`
	MaxTokens   int                 `json:"maxTokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
}

type roleSandboxSelection struct {
	Runtime  string `json:"runtime"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func (h *RoleHandler) WithBridgeClient(client roleAuthoringBridgeClient) *RoleHandler {
	h.bridgeClient = client
	return h
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

func (h *RoleHandler) Delete(c echo.Context) error {
	roleID := c.Param("id")
	if roleID == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "role id required"})
	}
	if err := h.store.Delete(roleID); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "role not found"})
		}
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to delete role"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "role deleted"})
}

func (h *RoleHandler) Preview(c echo.Context) error {
	var req rolePreviewRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if req.RoleID == "" && req.Draft == nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "role id or draft is required"})
	}

	normalized, effective, err := h.store.Preview(req.RoleID, (*rolepkg.Manifest)(req.Draft))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "role not found"})
		}
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}

	response := rolePreviewResponse{
		NormalizedManifest: (*model.RoleManifest)(normalized),
		EffectiveManifest:  (*model.RoleManifest)(effective),
		ExecutionProfile:   rolepkg.BuildExecutionProfile(effective),
	}
	if normalized.Extends != "" {
		response.Inheritance = &roleInheritanceSummary{ParentRoleID: normalized.Extends}
	}
	return c.JSON(http.StatusOK, response)
}

func (h *RoleHandler) Sandbox(c echo.Context) error {
	var req roleSandboxRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if req.RoleID == "" && req.Draft == nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "role id or draft is required"})
	}
	if strings.TrimSpace(req.Input) == "" {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "sandbox input is required"})
	}
	if h.bridgeClient == nil {
		return c.JSON(http.StatusServiceUnavailable, model.ErrorResponse{Message: "role sandbox bridge is unavailable"})
	}

	normalized, effective, err := h.store.Preview(req.RoleID, (*rolepkg.Manifest)(req.Draft))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "role not found"})
		}
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}

	catalog, err := h.bridgeClient.GetRuntimeCatalog(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to load runtime catalog"})
	}

	selection, diagnostics, canProbe := resolveSandboxSelection(catalog, req)
	response := rolePreviewResponse{
		NormalizedManifest:   (*model.RoleManifest)(normalized),
		EffectiveManifest:    (*model.RoleManifest)(effective),
		ExecutionProfile:     rolepkg.BuildExecutionProfile(effective),
		ReadinessDiagnostics: diagnostics,
		Selection:            selection,
	}
	if normalized.Extends != "" {
		response.Inheritance = &roleInheritanceSummary{ParentRoleID: normalized.Extends}
	}
	if !canProbe {
		return c.JSON(http.StatusOK, response)
	}

	probe, err := h.bridgeClient.Generate(c.Request().Context(), bridge.GenerateRequest{
		Prompt:       strings.TrimSpace(req.Input),
		SystemPrompt: effective.SystemPrompt,
		Provider:     selection.Provider,
		Model:        selection.Model,
		MaxTokens:    maxInt(req.MaxTokens, 256),
		Temperature:  req.Temperature,
	})
	if err != nil {
		return c.JSON(http.StatusBadGateway, model.ErrorResponse{Message: "failed to run role sandbox probe"})
	}
	response.Probe = probe
	return c.JSON(http.StatusOK, response)
}

func firstRoleID(role model.RoleManifest) string {
	if role.Metadata.ID != "" {
		return role.Metadata.ID
	}
	return role.Metadata.Name
}

func resolveSandboxSelection(catalog *bridge.RuntimeCatalogResponse, req roleSandboxRequest) (*roleSandboxSelection, []bridge.RuntimeDiagnosticDTO, bool) {
	if catalog == nil {
		return nil, []bridge.RuntimeDiagnosticDTO{{Code: "missing_runtime_catalog", Message: "Runtime catalog unavailable", Blocking: true}}, false
	}

	runtimeKey := strings.TrimSpace(req.Runtime)
	if runtimeKey == "" {
		runtimeKey = catalog.DefaultRuntime
	}

	var runtimeEntry *bridge.RuntimeCatalogEntryDTO
	for index := range catalog.Runtimes {
		if catalog.Runtimes[index].Key == runtimeKey {
			runtimeEntry = &catalog.Runtimes[index]
			break
		}
	}
	if runtimeEntry == nil {
		return &roleSandboxSelection{Runtime: runtimeKey}, []bridge.RuntimeDiagnosticDTO{{
			Code:     "unknown_runtime",
			Message:  "Selected runtime is not available in the catalog",
			Blocking: true,
		}}, false
	}

	diagnostics := append([]bridge.RuntimeDiagnosticDTO(nil), runtimeEntry.Diagnostics...)
	provider := strings.TrimSpace(req.Provider)
	if provider == "" {
		provider = runtimeEntry.DefaultProvider
	}
	switch provider {
	case "codex":
		provider = "openai"
	case "opencode":
		diagnostics = append(diagnostics, bridge.RuntimeDiagnosticDTO{
			Code:     "unsupported_probe_provider",
			Message:  "OpenCode runtime is not supported by the lightweight role sandbox probe",
			Blocking: true,
		})
	}
	modelName := strings.TrimSpace(req.Model)
	if modelName == "" {
		modelName = runtimeEntry.DefaultModel
	}

	selection := &roleSandboxSelection{
		Runtime:  runtimeEntry.Key,
		Provider: provider,
		Model:    modelName,
	}

	blocking := !runtimeEntry.Available
	for _, diagnostic := range diagnostics {
		if diagnostic.Blocking {
			blocking = true
			break
		}
	}
	return selection, diagnostics, !blocking
}

func maxInt(value, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}
