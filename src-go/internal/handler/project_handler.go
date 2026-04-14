package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/i18n"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type ProjectHandler struct {
	repo          ProjectRepository
	runtimeClient ProjectRuntimeCatalogClient
	wikiBootstrap wikiProjectBootstrapper
}

type ProjectRepository interface {
	Create(ctx context.Context, project *model.Project) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
	List(ctx context.Context) ([]*model.Project, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateProjectRequest) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type ProjectRuntimeCatalogClient interface {
	GetRuntimeCatalog(ctx context.Context) (*bridge.RuntimeCatalogResponse, error)
}

type wikiProjectBootstrapper interface {
	CreateSpace(ctx context.Context, projectID uuid.UUID) (*model.WikiSpace, error)
	SeedBuiltInTemplates(ctx context.Context, projectID uuid.UUID, spaceID uuid.UUID) ([]*model.WikiPage, error)
	DeleteProjectSpace(ctx context.Context, projectID uuid.UUID) error
}

func NewProjectHandler(
	repo ProjectRepository,
	runtimeClient ProjectRuntimeCatalogClient,
	wikiBootstrap ...wikiProjectBootstrapper,
) *ProjectHandler {
	handler := &ProjectHandler{repo: repo, runtimeClient: runtimeClient}
	if len(wikiBootstrap) > 0 {
		handler.wikiBootstrap = wikiBootstrap[0]
	}
	return handler
}

func (h *ProjectHandler) Create(c echo.Context) error {
	req := new(model.CreateProjectRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}

	project := &model.Project{
		ID:            uuid.New(),
		Name:          req.Name,
		Slug:          req.Slug,
		Description:   req.Description,
		RepoURL:       req.RepoURL,
		DefaultBranch: "main",
		Settings:      "{}",
	}
	if err := h.repo.Create(c.Request().Context(), project); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToCreateProject)
	}
	if h.wikiBootstrap != nil {
		space, err := h.wikiBootstrap.CreateSpace(c.Request().Context(), project.ID)
		if err != nil {
			_ = h.repo.Delete(c.Request().Context(), project.ID)
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToInitProjectWiki)
		}
		if _, err := h.wikiBootstrap.SeedBuiltInTemplates(c.Request().Context(), project.ID, space.ID); err != nil {
			_ = h.repo.Delete(c.Request().Context(), project.ID)
			return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToInitProjectWikiTemplates)
		}
	}
	return c.JSON(http.StatusCreated, h.toProjectDTO(c.Request().Context(), project))
}

func (h *ProjectHandler) List(c echo.Context) error {
	projects, err := h.repo.List(c.Request().Context())
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToListProjects)
	}
	dtos := make([]model.ProjectDTO, 0, len(projects))
	for _, p := range projects {
		dtos = append(dtos, h.toProjectDTO(c.Request().Context(), p))
	}
	return c.JSON(http.StatusOK, dtos)
}

func (h *ProjectHandler) Get(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	project, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgProjectNotFound)
	}
	return c.JSON(http.StatusOK, h.toProjectDTO(c.Request().Context(), project))
}

func (h *ProjectHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	req := new(model.UpdateProjectRequest)
	if err := c.Bind(req); err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
	}
	if err := c.Validate(req); err != nil {
		return c.JSON(http.StatusUnprocessableEntity, model.ErrorResponse{Message: err.Error()})
	}
	if err := h.repo.Update(c.Request().Context(), id, req); err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUpdateProject)
	}
	project, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToFetchUpdatedProject)
	}
	return c.JSON(http.StatusOK, h.toProjectDTO(c.Request().Context(), project))
}

func (h *ProjectHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	if h.wikiBootstrap != nil {
		_ = h.wikiBootstrap.DeleteProjectSpace(c.Request().Context(), id)
	}
	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgProjectNotFound)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "project deleted"})
}

func (h *ProjectHandler) toProjectDTO(ctx context.Context, project *model.Project) model.ProjectDTO {
	if project == nil {
		return model.ProjectDTO{}
	}

	selection, err := service.ResolveProjectCodingAgentSelection(project, "", "", "")
	if err != nil {
		selection = fallbackCodingAgentSelection()
	}

	catalog := service.DefaultCodingAgentCatalog(selection)
	if h.runtimeClient != nil {
		if runtimeCatalog, catalogErr := h.runtimeClient.GetRuntimeCatalog(ctx); catalogErr == nil {
			catalog = projectCatalogFromBridge(runtimeCatalog, selection)
		}
	}

	return project.ToDTOWithCatalog(catalog)
}

func fallbackCodingAgentSelection() model.CodingAgentSelection {
	selection, err := service.ResolveProjectCodingAgentSelection(nil, "", "", "")
	if err != nil {
		return model.CodingAgentSelection{Runtime: model.DefaultCodingAgentRuntime}
	}
	return selection
}

func projectCatalogFromBridge(
	runtimeCatalog *bridge.RuntimeCatalogResponse,
	selection model.CodingAgentSelection,
) *model.CodingAgentCatalogDTO {
	if runtimeCatalog == nil {
		return service.DefaultCodingAgentCatalog(selection)
	}

	runtimes := make([]model.CodingAgentRuntimeOptionDTO, 0, len(runtimeCatalog.Runtimes))
	for _, runtime := range runtimeCatalog.Runtimes {
		diagnostics := make([]model.CodingAgentDiagnosticDTO, 0, len(runtime.Diagnostics))
		for _, diagnostic := range runtime.Diagnostics {
			diagnostics = append(diagnostics, model.CodingAgentDiagnosticDTO{
				Code:     diagnostic.Code,
				Message:  diagnostic.Message,
				Blocking: diagnostic.Blocking,
			})
		}
		runtimes = append(runtimes, model.CodingAgentRuntimeOptionDTO{
			Runtime:                 runtime.Key,
			Label:                   runtime.Label,
			DefaultProvider:         runtime.DefaultProvider,
			CompatibleProviders:     append([]string(nil), runtime.CompatibleProviders...),
			DefaultModel:            runtime.DefaultModel,
			ModelOptions:            append([]string(nil), runtime.ModelOptions...),
			Available:               runtime.Available,
			Diagnostics:             diagnostics,
			SupportedFeatures:       append([]string(nil), runtime.SupportedFeatures...),
			InteractionCapabilities: projectInteractionCapabilitiesFromBridge(runtime.InteractionCapabilities),
			Providers:               projectProvidersFromBridge(runtime.Providers),
			LaunchContract:          projectLaunchContractFromBridge(runtime.LaunchContract),
			Lifecycle:               projectLifecycleFromBridge(runtime.Lifecycle),
		})
	}

	defaultRuntime := strings.TrimSpace(runtimeCatalog.DefaultRuntime)
	if defaultRuntime == "" {
		defaultRuntime = model.DefaultCodingAgentRuntime
	}

	defaultSelection := selection
	selectedIndex := -1
	for i := range runtimes {
		if runtimes[i].Runtime == selection.Runtime {
			selectedIndex = i
			break
		}
	}
	isSelectionLaunchable := selectedIndex >= 0 && runtimes[selectedIndex].Available
	if !isSelectionLaunchable {
		if selectedIndex >= 0 {
			runtimes[selectedIndex].Diagnostics = append(
				runtimes[selectedIndex].Diagnostics,
				model.CodingAgentDiagnosticDTO{
					Code:     "stale_default_selection",
					Message:  "Saved coding-agent default is unavailable; falling back to the next launchable runtime.",
					Blocking: false,
				},
			)
		}
		if fallback := findFirstLaunchableRuntime(runtimes, defaultRuntime); fallback != nil {
			defaultSelection = model.CodingAgentSelection{
				Runtime:  fallback.Runtime,
				Provider: fallback.DefaultProvider,
				Model:    firstNonEmptyModel(fallback),
			}
			defaultRuntime = fallback.Runtime
		}
	}

	return &model.CodingAgentCatalogDTO{
		DefaultRuntime:   defaultRuntime,
		DefaultSelection: defaultSelection,
		Runtimes:         runtimes,
	}
}

func projectInteractionCapabilitiesFromBridge(
	raw bridge.RuntimeInteractionCapabilities,
) model.CodingAgentInteractionCapabilitiesDTO {
	if len(raw) == 0 {
		return nil
	}

	result := make(model.CodingAgentInteractionCapabilitiesDTO, len(raw))
	for category, group := range raw {
		if len(group) == 0 {
			continue
		}
		result[category] = make(map[string]model.CodingAgentCapabilityDescriptorDTO, len(group))
		for key, descriptor := range group {
			result[category][key] = model.CodingAgentCapabilityDescriptorDTO{
				State:                 descriptor.State,
				ReasonCode:            descriptor.ReasonCode,
				Message:               descriptor.Message,
				RequiresRequestFields: append([]string(nil), descriptor.RequiresRequestFields...),
			}
		}
	}
	return result
}

func projectProvidersFromBridge(
	providers []bridge.RuntimeCatalogProviderDTO,
) []model.CodingAgentProviderDTO {
	if len(providers) == 0 {
		return nil
	}

	result := make([]model.CodingAgentProviderDTO, 0, len(providers))
	for _, provider := range providers {
		result = append(result, model.CodingAgentProviderDTO{
			Provider:     provider.Provider,
			Connected:    provider.Connected,
			DefaultModel: provider.DefaultModel,
			ModelOptions: append([]string(nil), provider.ModelOptions...),
			AuthRequired: provider.AuthRequired,
			AuthMethods:  append([]string(nil), provider.AuthMethods...),
		})
	}
	return result
}

func projectLaunchContractFromBridge(
	raw *bridge.RuntimeLaunchContractDTO,
) *model.CodingAgentLaunchContractDTO {
	if raw == nil {
		return nil
	}
	return &model.CodingAgentLaunchContractDTO{
		PromptTransport:        raw.PromptTransport,
		OutputMode:             raw.OutputMode,
		SupportedOutputModes:   append([]string(nil), raw.SupportedOutputModes...),
		SupportedApprovalModes: append([]string(nil), raw.SupportedApprovalModes...),
		AdditionalDirectories:  raw.AdditionalDirectories,
		EnvOverrides:           raw.EnvOverrides,
	}
}

func projectLifecycleFromBridge(
	raw *bridge.RuntimeLifecycleDTO,
) *model.CodingAgentLifecycleDTO {
	if raw == nil {
		return nil
	}
	return &model.CodingAgentLifecycleDTO{
		Stage:              raw.Stage,
		SunsetAt:           raw.SunsetAt,
		ReplacementRuntime: raw.ReplacementRuntime,
		Message:            raw.Message,
	}
}

func firstNonEmptyModel(runtime *model.CodingAgentRuntimeOptionDTO) string {
	if runtime == nil {
		return ""
	}
	if len(runtime.ModelOptions) > 0 && strings.TrimSpace(runtime.ModelOptions[0]) != "" {
		return runtime.ModelOptions[0]
	}
	return runtime.DefaultModel
}

func findFirstLaunchableRuntime(
	runtimes []model.CodingAgentRuntimeOptionDTO,
	preferredRuntime string,
) *model.CodingAgentRuntimeOptionDTO {
	for i := range runtimes {
		if runtimes[i].Runtime == preferredRuntime && runtimes[i].Available {
			return &runtimes[i]
		}
	}
	for i := range runtimes {
		if runtimes[i].Available {
			return &runtimes[i]
		}
	}
	if len(runtimes) == 0 {
		return nil
	}
	return &runtimes[0]
}
