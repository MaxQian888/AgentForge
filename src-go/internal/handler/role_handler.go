package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	rolepkg "github.com/react-go-quick-starter/server/internal/role"
)

type RoleHandler struct {
	store        *rolepkg.FileStore
	bridgeClient roleAuthoringBridgeClient
	skillsDir    string
}

func NewRoleHandler(rolesDir string) *RoleHandler {
	return &RoleHandler{
		store:     rolepkg.NewFileStore(rolesDir),
		skillsDir: filepath.Join(filepath.Dir(rolesDir), "skills"),
	}
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

func skillDiagnosticsToRuntimeDiagnostics(diagnostics []model.RoleExecutionSkillDiagnostic) []bridge.RuntimeDiagnosticDTO {
	if len(diagnostics) == 0 {
		return nil
	}
	converted := make([]bridge.RuntimeDiagnosticDTO, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		converted = append(converted, bridge.RuntimeDiagnosticDTO{
			Code:     diagnostic.Code,
			Message:  diagnostic.Message,
			Blocking: diagnostic.Blocking,
		})
	}
	return converted
}

func (h *RoleHandler) WithBridgeClient(client roleAuthoringBridgeClient) *RoleHandler {
	h.bridgeClient = client
	return h
}

func (h *RoleHandler) List(c echo.Context) error {
	roles, err := h.store.List()
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadRoles)
	}
	return c.JSON(http.StatusOK, roles)
}

func (h *RoleHandler) Get(c echo.Context) error {
	roleID := c.Param("id")
	loadedRole, err := h.store.Get(roleID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return localizedError(c, http.StatusNotFound, i18n.MsgRoleNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadRole)
	}
	return c.JSON(http.StatusOK, loadedRole)
}

func (h *RoleHandler) ListSkills(c echo.Context) error {
	entries, err := rolepkg.DiscoverSkillCatalog(h.skillsDir)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadRoles)
	}
	return c.JSON(http.StatusOK, entries)
}

func (h *RoleHandler) Create(c echo.Context) error {
	var manifest model.RoleManifest
	if err := c.Bind(&manifest); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if manifest.Metadata.ID == "" && manifest.Metadata.Name == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgRoleIDOrNameRequired)
	}
	if err := h.store.Save((*rolepkg.Manifest)(&manifest)); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgFailedToSaveRole)
	}
	loadedRole, err := h.store.Get(firstRoleID(manifest))
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToReloadRole)
	}
	return c.JSON(http.StatusCreated, loadedRole)
}

func (h *RoleHandler) Update(c echo.Context) error {
	roleID := c.Param("id")
	var manifest model.RoleManifest
	rawPayload, err := decodeJSONObjectBody(c, &manifest)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	manifest.Metadata.ID = roleID
	if manifest.Metadata.Name == "" {
		manifest.Metadata.Name = roleID
	}
	existingRole, err := h.store.Get(roleID)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadRole)
	}
	manifestToSave := preserveAdvancedRoleSections(existingRole, &manifest, rawPayload)
	if err := h.store.Save((*rolepkg.Manifest)(manifestToSave)); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgFailedToSaveRole)
	}
	loadedRole, err := h.store.Get(roleID)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToReloadRole)
	}
	return c.JSON(http.StatusOK, loadedRole)
}

func (h *RoleHandler) Delete(c echo.Context) error {
	roleID := c.Param("id")
	if roleID == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgRoleIDRequired)
	}
	if err := h.store.Delete(roleID); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return localizedError(c, http.StatusNotFound, i18n.MsgRoleNotFound)
		}
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToDeleteRole)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "role deleted"})
}

func (h *RoleHandler) Preview(c echo.Context) error {
	var req rolePreviewRequest
	rawPayload, err := decodeJSONObjectBody(c, &req)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.RoleID == "" && req.Draft == nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgRoleIDOrDraftRequired)
	}

	draft := req.Draft
	if req.RoleID != "" && req.Draft != nil {
		existingRole, err := h.store.Get(req.RoleID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return localizedError(c, http.StatusNotFound, i18n.MsgRoleNotFound)
			}
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadRole)
		}
		draft = preserveAdvancedRoleSections(existingRole, req.Draft, nestedJSONObject(rawPayload, "draft"))
	}

	normalized, effective, err := h.store.Preview(req.RoleID, (*rolepkg.Manifest)(draft))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return localizedError(c, http.StatusNotFound, i18n.MsgRoleNotFound)
		}
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}

	executionProfile := rolepkg.BuildExecutionProfile(effective, rolepkg.WithSkillRoot(h.skillsDir))
	response := rolePreviewResponse{
		NormalizedManifest: (*model.RoleManifest)(normalized),
		EffectiveManifest:  (*model.RoleManifest)(effective),
		ExecutionProfile:   executionProfile,
		ReadinessDiagnostics: skillDiagnosticsToRuntimeDiagnostics(executionProfile.SkillDiagnostics),
	}
	if normalized.Extends != "" {
		response.Inheritance = &roleInheritanceSummary{ParentRoleID: normalized.Extends}
	}
	return c.JSON(http.StatusOK, response)
}

func (h *RoleHandler) Sandbox(c echo.Context) error {
	var req roleSandboxRequest
	rawPayload, err := decodeJSONObjectBody(c, &req)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if req.RoleID == "" && req.Draft == nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgRoleIDOrDraftRequired)
	}
	if strings.TrimSpace(req.Input) == "" {
		return localizedError(c, http.StatusBadRequest, i18n.MsgSandboxInputRequired)
	}
	if h.bridgeClient == nil {
		return localizedError(c, http.StatusServiceUnavailable, i18n.MsgRoleSandboxUnavailable)
	}

	draft := req.Draft
	if req.RoleID != "" && req.Draft != nil {
		existingRole, err := h.store.Get(req.RoleID)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return localizedError(c, http.StatusNotFound, i18n.MsgRoleNotFound)
			}
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToLoadRole)
		}
		draft = preserveAdvancedRoleSections(existingRole, req.Draft, nestedJSONObject(rawPayload, "draft"))
	}

	normalized, effective, err := h.store.Preview(req.RoleID, (*rolepkg.Manifest)(draft))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return localizedError(c, http.StatusNotFound, i18n.MsgRoleNotFound)
		}
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: err.Error()})
	}

	catalog, err := h.bridgeClient.GetRuntimeCatalog(c.Request().Context())
	if err != nil {
		return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToLoadRuntimeCatalog)
	}

	executionProfile := rolepkg.BuildExecutionProfile(effective, rolepkg.WithSkillRoot(h.skillsDir))
	selection, diagnostics, canProbe := resolveSandboxSelection(catalog, req)
	diagnostics = append(diagnostics, skillDiagnosticsToRuntimeDiagnostics(executionProfile.SkillDiagnostics)...)
	for _, diagnostic := range executionProfile.SkillDiagnostics {
		if diagnostic.Blocking {
			canProbe = false
			break
		}
	}
	response := rolePreviewResponse{
		NormalizedManifest:   (*model.RoleManifest)(normalized),
		EffectiveManifest:    (*model.RoleManifest)(effective),
		ExecutionProfile:     executionProfile,
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
		return localizedError(c, http.StatusBadGateway, i18n.MsgFailedToRunSandboxProbe)
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

type jsonObject map[string]json.RawMessage

func preserveAdvancedRoleSections(base, overlay *model.RoleManifest, presence jsonObject) *model.RoleManifest {
	if base == nil {
		return overlay
	}
	if overlay == nil {
		return base
	}

	merged := *overlay

	if merged.APIVersion == "" && !fieldPresent(presence, "apiVersion") {
		merged.APIVersion = base.APIVersion
	}
	if merged.Kind == "" && !fieldPresent(presence, "kind") {
		merged.Kind = base.Kind
	}
	if merged.Metadata.Version == "" && !fieldPresent(presence, "metadata", "version") {
		merged.Metadata.Version = base.Metadata.Version
	}
	if merged.Metadata.Description == "" && !fieldPresent(presence, "metadata", "description") {
		merged.Metadata.Description = base.Metadata.Description
	}
	if merged.Metadata.Author == "" && !fieldPresent(presence, "metadata", "author") {
		merged.Metadata.Author = base.Metadata.Author
	}
	if len(merged.Metadata.Tags) == 0 && !fieldPresent(presence, "metadata", "tags") {
		merged.Metadata.Tags = append([]string(nil), base.Metadata.Tags...)
	}
	if merged.Metadata.Icon == "" && !fieldPresent(presence, "metadata", "icon") {
		merged.Metadata.Icon = base.Metadata.Icon
	}

	if merged.Identity.Backstory == "" && !fieldPresent(presence, "identity", "backstory") {
		merged.Identity.Backstory = base.Identity.Backstory
	}
	if merged.Identity.SystemPrompt == "" && !fieldPresent(presence, "identity", "systemPrompt") {
		merged.Identity.SystemPrompt = base.Identity.SystemPrompt
	}
	if merged.Identity.Persona == "" && !fieldPresent(presence, "identity", "persona") {
		merged.Identity.Persona = base.Identity.Persona
	}
	if len(merged.Identity.Goals) == 0 && !fieldPresent(presence, "identity", "goals") {
		merged.Identity.Goals = append([]string(nil), base.Identity.Goals...)
	}
	if len(merged.Identity.Constraints) == 0 && !fieldPresent(presence, "identity", "constraints") {
		merged.Identity.Constraints = append([]string(nil), base.Identity.Constraints...)
	}
	if merged.Identity.Personality == "" && !fieldPresent(presence, "identity", "personality") {
		merged.Identity.Personality = base.Identity.Personality
	}
	if merged.Identity.Language == "" && !fieldPresent(presence, "identity", "language") {
		merged.Identity.Language = base.Identity.Language
	}
	if merged.Identity.ResponseStyle == (model.RoleResponseStyle{}) && !fieldPresent(presence, "identity", "responseStyle") {
		merged.Identity.ResponseStyle = base.Identity.ResponseStyle
	}

	if merged.SystemPrompt == "" && !fieldPresent(presence, "systemPrompt") {
		merged.SystemPrompt = base.SystemPrompt
	}

	if len(merged.Capabilities.Packages) == 0 && !fieldPresent(presence, "capabilities", "packages") {
		merged.Capabilities.Packages = append([]string(nil), base.Capabilities.Packages...)
	}
	if len(merged.Capabilities.AllowedTools) == 0 && !fieldPresent(presence, "capabilities", "allowedTools") {
		merged.Capabilities.AllowedTools = append([]string(nil), base.Capabilities.AllowedTools...)
	}
	if len(merged.Capabilities.Tools) == 0 && !fieldPresent(presence, "capabilities", "tools") {
		merged.Capabilities.Tools = append([]string(nil), base.Capabilities.Tools...)
	}
	if len(merged.Capabilities.ToolConfig.BuiltIn) == 0 && !fieldPresent(presence, "capabilities", "toolConfig", "builtIn") {
		merged.Capabilities.ToolConfig.BuiltIn = append([]string(nil), base.Capabilities.ToolConfig.BuiltIn...)
	}
	if len(merged.Capabilities.ToolConfig.External) == 0 && !fieldPresent(presence, "capabilities", "toolConfig", "external") {
		merged.Capabilities.ToolConfig.External = append([]string(nil), base.Capabilities.ToolConfig.External...)
	}
	if len(merged.Capabilities.ToolConfig.MCPServers) == 0 && !fieldPresent(presence, "capabilities", "toolConfig", "mcpServers") {
		merged.Capabilities.ToolConfig.MCPServers = append([]model.RoleMCPServer(nil), base.Capabilities.ToolConfig.MCPServers...)
	}
	if len(merged.Capabilities.Skills) == 0 && !fieldPresent(presence, "capabilities", "skills") {
		merged.Capabilities.Skills = append([]model.RoleSkillReference(nil), base.Capabilities.Skills...)
	}
	if len(merged.Capabilities.Languages) == 0 && !fieldPresent(presence, "capabilities", "languages") {
		merged.Capabilities.Languages = append([]string(nil), base.Capabilities.Languages...)
	}
	if len(merged.Capabilities.Frameworks) == 0 && !fieldPresent(presence, "capabilities", "frameworks") {
		merged.Capabilities.Frameworks = append([]string(nil), base.Capabilities.Frameworks...)
	}
	if merged.Capabilities.MaxConcurrency == 0 && !fieldPresent(presence, "capabilities", "maxConcurrency") {
		merged.Capabilities.MaxConcurrency = base.Capabilities.MaxConcurrency
	}
	if merged.Capabilities.MaxTurns == 0 && !fieldPresent(presence, "capabilities", "maxTurns") {
		merged.Capabilities.MaxTurns = base.Capabilities.MaxTurns
	}
	if merged.Capabilities.MaxBudgetUsd == 0 && !fieldPresent(presence, "capabilities", "maxBudgetUsd") {
		merged.Capabilities.MaxBudgetUsd = base.Capabilities.MaxBudgetUsd
	}
	if len(merged.Capabilities.CustomSettings) == 0 && !fieldPresent(presence, "capabilities", "customSettings") {
		merged.Capabilities.CustomSettings = cloneStringMap(base.Capabilities.CustomSettings)
	}

	if len(merged.Knowledge.Repositories) == 0 && !fieldPresent(presence, "knowledge", "repositories") {
		merged.Knowledge.Repositories = append([]string(nil), base.Knowledge.Repositories...)
	}
	if len(merged.Knowledge.Documents) == 0 && !fieldPresent(presence, "knowledge", "documents") {
		merged.Knowledge.Documents = append([]string(nil), base.Knowledge.Documents...)
	}
	if len(merged.Knowledge.Patterns) == 0 && !fieldPresent(presence, "knowledge", "patterns") {
		merged.Knowledge.Patterns = append([]string(nil), base.Knowledge.Patterns...)
	}
	if merged.Knowledge.SystemPrompt == "" && !fieldPresent(presence, "knowledge", "systemPrompt") {
		merged.Knowledge.SystemPrompt = base.Knowledge.SystemPrompt
	}
	if len(merged.Knowledge.Shared) == 0 && !fieldPresent(presence, "knowledge", "shared") {
		merged.Knowledge.Shared = append([]model.RoleKnowledgeSource(nil), base.Knowledge.Shared...)
	}
	if len(merged.Knowledge.Private) == 0 && !fieldPresent(presence, "knowledge", "private") {
		merged.Knowledge.Private = append([]model.RoleKnowledgeSource(nil), base.Knowledge.Private...)
	}
	if merged.Knowledge.Memory == (model.RoleMemoryConfig{}) && !fieldPresent(presence, "knowledge", "memory") {
		merged.Knowledge.Memory = base.Knowledge.Memory
	}

	if merged.Security.PermissionMode == "" && !fieldPresent(presence, "security", "permissionMode") {
		merged.Security.PermissionMode = base.Security.PermissionMode
	}
	if len(merged.Security.AllowedPaths) == 0 && !fieldPresent(presence, "security", "allowedPaths") {
		merged.Security.AllowedPaths = append([]string(nil), base.Security.AllowedPaths...)
	}
	if len(merged.Security.DeniedPaths) == 0 && !fieldPresent(presence, "security", "deniedPaths") {
		merged.Security.DeniedPaths = append([]string(nil), base.Security.DeniedPaths...)
	}
	if merged.Security.MaxBudgetUsd == 0 && !fieldPresent(presence, "security", "maxBudgetUsd") {
		merged.Security.MaxBudgetUsd = base.Security.MaxBudgetUsd
	}
	if !merged.Security.RequireReview && !fieldPresent(presence, "security", "requireReview") {
		merged.Security.RequireReview = base.Security.RequireReview
	}
	if merged.Security.Profile == "" && !fieldPresent(presence, "security", "profile") {
		merged.Security.Profile = base.Security.Profile
	}
	if isEmptyRolePermissions(merged.Security.Permissions) && !fieldPresent(presence, "security", "permissions") {
		merged.Security.Permissions = base.Security.Permissions
	}
	if len(merged.Security.OutputFilters) == 0 && !fieldPresent(presence, "security", "outputFilters") {
		merged.Security.OutputFilters = append([]string(nil), base.Security.OutputFilters...)
	}
	if merged.Security.ResourceLimits == (model.RoleResourceLimits{}) && !fieldPresent(presence, "security", "resourceLimits") {
		merged.Security.ResourceLimits = base.Security.ResourceLimits
	}

	if merged.Extends == "" && !fieldPresent(presence, "extends") {
		merged.Extends = base.Extends
	}
	if len(merged.Overrides) == 0 && !fieldPresent(presence, "overrides") {
		merged.Overrides = base.Overrides
	}
	if isEmptyRoleCollaboration(merged.Collaboration) && !fieldPresent(presence, "collaboration") {
		merged.Collaboration = base.Collaboration
	}
	if len(merged.Triggers) == 0 && !fieldPresent(presence, "triggers") {
		merged.Triggers = append([]model.RoleTrigger(nil), base.Triggers...)
	}

	return &merged
}

func decodeJSONObjectBody(c echo.Context, dest any) (jsonObject, error) {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return nil, err
	}
	c.Request().Body = io.NopCloser(bytes.NewBuffer(body))
	if err := c.Bind(dest); err != nil {
		return nil, err
	}
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return jsonObject{}, nil
	}

	var payload jsonObject
	if err := json.Unmarshal(trimmed, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func nestedJSONObject(root jsonObject, key string) jsonObject {
	if len(root) == 0 {
		return nil
	}
	raw, ok := root[key]
	if !ok {
		return nil
	}

	var nested jsonObject
	if err := json.Unmarshal(raw, &nested); err != nil {
		return nil
	}
	return nested
}

func fieldPresent(root jsonObject, path ...string) bool {
	current := root
	for index, key := range path {
		if len(current) == 0 {
			return false
		}

		raw, ok := current[key]
		if !ok {
			return false
		}
		if index == len(path)-1 {
			return true
		}

		next := jsonObject{}
		if err := json.Unmarshal(raw, &next); err != nil {
			return false
		}
		current = next
	}
	return false
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func isEmptyRolePermissions(permissions model.RolePermissions) bool {
	return len(permissions.FileAccess.AllowedPaths) == 0 &&
		len(permissions.FileAccess.DeniedPaths) == 0 &&
		len(permissions.Network.AllowedDomains) == 0 &&
		len(permissions.CodeExecution.AllowedLanguages) == 0 &&
		!permissions.CodeExecution.Sandbox
}

func isEmptyRoleCollaboration(collaboration model.RoleCollaboration) bool {
	return len(collaboration.CanDelegateTo) == 0 &&
		len(collaboration.AcceptsDelegationFrom) == 0 &&
		collaboration.Communication.PreferredChannel == "" &&
		collaboration.Communication.ReportFormat == "" &&
		collaboration.Communication.EscalationPolicy == ""
}
