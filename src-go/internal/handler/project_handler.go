package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/agentforge/server/internal/bridge"
	"github.com/agentforge/server/internal/i18n"
	appMiddleware "github.com/agentforge/server/internal/middleware"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type ProjectHandler struct {
	repo          ProjectRepository
	runtimeClient ProjectRuntimeCatalogClient
	wikiBootstrap wikiProjectBootstrapper
	userRepo      ProjectUserLookup
	templateClone ProjectTemplateCloneService
	auditEmitter  ProjectCreatedFromTemplateEmitter
	lifecycle     ProjectLifecycleService
}

// ProjectLifecycleService is the narrow contract the project handler uses
// when archiving, unarchiving, or deleting a project. Implemented by
// service.ProjectLifecycleService — the handler accepts the interface
// directly rather than the concrete type so tests can pass a fake.
type ProjectLifecycleService interface {
	Archive(ctx context.Context, projectID, ownerUserID uuid.UUID) (*model.Project, error)
	Unarchive(ctx context.Context, projectID uuid.UUID) (*model.Project, error)
	DeleteArchived(ctx context.Context, projectID uuid.UUID, opts service.DeleteOptions) error
}

// ProjectCreatedFromTemplateEmitter receives an audit event when a project
// is created from a template. Defined here as a single-method seam so the
// handler does not take a dependency on the full audit service shape.
type ProjectCreatedFromTemplateEmitter interface {
	EmitProjectCreatedFromTemplate(
		ctx context.Context,
		projectID, actorUserID, templateID uuid.UUID,
		templateVersion int,
		templateSource string,
	)
}

// ProjectTemplateCloneService is the narrow contract the project handler
// uses when `POST /projects` includes template parameters. Defined here
// (not in service) so wiring is explicit and tests can pass a fake.
type ProjectTemplateCloneService interface {
	Get(ctx context.Context, id, userID uuid.UUID) (*model.ProjectTemplate, error)
	ApplySnapshotSettings(snap model.ProjectTemplateSnapshot) *model.ProjectSettingsPatch
	ApplySnapshot(ctx context.Context, projectID uuid.UUID, snap model.ProjectTemplateSnapshot) error
}

type ProjectRepository interface {
	Create(ctx context.Context, project *model.Project) error
	CreateWithOwner(ctx context.Context, project *model.Project, ownerMember *model.Member) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
	List(ctx context.Context) ([]*model.Project, error)
	ListWithFilter(ctx context.Context, filter repository.ProjectListFilter) ([]*model.Project, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateProjectRequest) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ProjectUserLookup is the narrow contract used to fetch the creating user's
// display name + email when auto-assigning the owner member row.
type ProjectUserLookup interface {
	GetByID(ctx context.Context, id uuid.UUID) (*model.User, error)
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

// WithUserLookup installs the user repo so Create can populate the auto-owner
// member row with the creating user's display name and email. Without it,
// the owner row falls back to placeholders derived from the JWT claims.
func (h *ProjectHandler) WithUserLookup(userRepo ProjectUserLookup) *ProjectHandler {
	h.userRepo = userRepo
	return h
}

// WithTemplateClone wires the template service so Create can accept
// `templateSource`+`templateId` and materialize the snapshot on the new
// project. Without it, those params are ignored silently (feature-flag off).
func (h *ProjectHandler) WithTemplateClone(svc ProjectTemplateCloneService) *ProjectHandler {
	h.templateClone = svc
	return h
}

// WithTemplateAuditEmitter wires the emitter used to record
// project_created_from_template audit events after a successful clone.
func (h *ProjectHandler) WithTemplateAuditEmitter(e ProjectCreatedFromTemplateEmitter) *ProjectHandler {
	h.auditEmitter = e
	return h
}

// WithLifecycleService wires the project lifecycle service used by
// Archive/Unarchive/Delete. Without it, those endpoints return 500
// (feature-flag off).
func (h *ProjectHandler) WithLifecycleService(lc ProjectLifecycleService) *ProjectHandler {
	h.lifecycle = lc
	return h
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

	creatorID, err := claimsUserID(c)
	if err != nil || creatorID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}

	ownerName, ownerEmail := "", ""
	if h.userRepo != nil {
		if user, lookupErr := h.userRepo.GetByID(c.Request().Context(), *creatorID); lookupErr == nil && user != nil {
			ownerName = user.Name
			ownerEmail = user.Email
		}
	}
	if ownerName == "" {
		ownerName = "Project Owner"
	}

	ownerMember := &model.Member{
		UserID: creatorID,
		Name:   ownerName,
		Email:  ownerEmail,
	}
	// Resolve template BEFORE creating the project so we fail fast on
	// invalid templateId without leaving an empty project behind.
	var (
		templateSnapshot *model.ProjectTemplateSnapshot
		templateID       uuid.UUID
		templateSource   string
		templateVersion  int
	)
	if h.templateClone != nil && strings.TrimSpace(req.TemplateID) != "" {
		tplID, parseErr := uuid.Parse(req.TemplateID)
		if parseErr != nil {
			return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidTemplateID)
		}
		tpl, tplErr := h.templateClone.Get(c.Request().Context(), tplID, *creatorID)
		if tplErr != nil {
			return localizedError(c, http.StatusNotFound, i18n.MsgProjectTemplateNotFound)
		}
		snap, snapErr := model.ParseProjectTemplateSnapshot(tpl.SnapshotJSON)
		if snapErr != nil {
			return localizedError(c, http.StatusUnprocessableEntity, i18n.MsgProjectTemplateSnapshotInvalid)
		}
		// Apply snapshot settings to the new project row before insert so
		// the initial settings go in atomically with the project.
		if patch := h.templateClone.ApplySnapshotSettings(snap); patch != nil {
			merged, mergeErr := model.MergeProjectSettings(project.Settings, patch)
			if mergeErr == nil && merged != "" {
				project.Settings = merged
			}
		}
		templateSnapshot = &snap
		templateID = tpl.ID
		templateSource = tpl.Source
		templateVersion = tpl.SnapshotVersion
	}

	if err := h.repo.CreateWithOwner(c.Request().Context(), project, ownerMember); err != nil {
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

	// Apply template sub-resources after project + owner are durably inserted.
	// Partial failure here is logged-and-swallowed rather than fatal: the
	// project exists and can be edited manually; returning 500 now would
	// orphan an already-created project row. Per design (add-project-
	// templates/design.md Decision 3) ideal behavior is transactional, but
	// sub-resource services currently each manage their own DB handles.
	if templateSnapshot != nil && h.templateClone != nil {
		_ = h.templateClone.ApplySnapshot(c.Request().Context(), project.ID, *templateSnapshot)
		if h.auditEmitter != nil {
			h.auditEmitter.EmitProjectCreatedFromTemplate(
				c.Request().Context(), project.ID, *creatorID,
				templateID, templateVersion, templateSource,
			)
		}
	}
	return c.JSON(http.StatusCreated, h.toProjectDTO(c.Request().Context(), project))
}

func (h *ProjectHandler) List(c echo.Context) error {
	filter := repository.ProjectListFilter{}
	if statusParam := strings.TrimSpace(c.QueryParam("status")); statusParam != "" {
		statuses := make([]string, 0)
		for _, raw := range strings.Split(statusParam, ",") {
			candidate := strings.TrimSpace(raw)
			if candidate == "" {
				continue
			}
			if !model.IsValidProjectStatus(candidate) {
				return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidRequestBody)
			}
			statuses = append(statuses, candidate)
		}
		filter.Statuses = statuses
	} else if includeArchived, _ := strconv.ParseBool(c.QueryParam("includeArchived")); includeArchived {
		filter.Statuses = []string{
			model.ProjectStatusActive,
			model.ProjectStatusPaused,
			model.ProjectStatusArchived,
		}
	}
	projects, err := h.repo.ListWithFilter(c.Request().Context(), filter)
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
	// Lifecycle service is the canonical delete path — it enforces the
	// "archived before delete" invariant. When the service is not wired,
	// fall back to the legacy direct delete path (used by tests that do not
	// exercise the lifecycle).
	if h.lifecycle != nil {
		keepAudit := true
		if raw := strings.TrimSpace(c.QueryParam("keepAudit")); raw != "" {
			if parsed, parseErr := strconv.ParseBool(raw); parseErr == nil {
				keepAudit = parsed
			}
		}
		err := h.lifecycle.DeleteArchived(c.Request().Context(), id, service.DeleteOptions{KeepAudit: keepAudit})
		switch {
		case errors.Is(err, service.ErrProjectNotFound):
			return localizedError(c, http.StatusNotFound, i18n.MsgProjectNotFound)
		case errors.Is(err, service.ErrProjectMustBeArchived):
			return localizedError(c, http.StatusConflict, i18n.MsgProjectMustBeArchivedBeforeDelete)
		case err != nil:
			return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
		}
		if h.wikiBootstrap != nil {
			_ = h.wikiBootstrap.DeleteProjectSpace(c.Request().Context(), id)
		}
		return c.JSON(http.StatusOK, map[string]string{"message": "project deleted"})
	}
	if h.wikiBootstrap != nil {
		_ = h.wikiBootstrap.DeleteProjectSpace(c.Request().Context(), id)
	}
	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		return localizedError(c, http.StatusNotFound, i18n.MsgProjectNotFound)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "project deleted"})
}

// Archive flips the project to archived status. Owner-only (enforced by
// route-level RBAC via appMiddleware.Require(ActionProjectArchive)).
func (h *ProjectHandler) Archive(c echo.Context) error {
	projectID, err := projectIDFromContext(c)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	ownerID, err := claimsUserID(c)
	if err != nil || ownerID == nil {
		return localizedError(c, http.StatusUnauthorized, i18n.MsgUnauthorized)
	}
	if h.lifecycle == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
	}
	project, err := h.lifecycle.Archive(c.Request().Context(), projectID, *ownerID)
	switch {
	case errors.Is(err, service.ErrProjectNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgProjectNotFound)
	case errors.Is(err, service.ErrProjectAlreadyArchived):
		return localizedError(c, http.StatusConflict, i18n.MsgProjectArchived)
	case err != nil:
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToArchiveProject)
	}
	return c.JSON(http.StatusOK, h.toProjectDTO(c.Request().Context(), project))
}

// Unarchive flips the project back to active status. Owner-only.
func (h *ProjectHandler) Unarchive(c echo.Context) error {
	projectID, err := projectIDFromContext(c)
	if err != nil {
		return localizedError(c, http.StatusBadRequest, i18n.MsgInvalidProjectID)
	}
	if h.lifecycle == nil {
		return localizedError(c, http.StatusInternalServerError, i18n.MsgInternalError)
	}
	project, err := h.lifecycle.Unarchive(c.Request().Context(), projectID)
	switch {
	case errors.Is(err, service.ErrProjectNotFound):
		return localizedError(c, http.StatusNotFound, i18n.MsgProjectNotFound)
	case errors.Is(err, service.ErrProjectNotArchived):
		return localizedError(c, http.StatusConflict, i18n.MsgProjectNotArchived)
	case err != nil:
		return localizedError(c, http.StatusInternalServerError, i18n.MsgFailedToUnarchiveProject)
	}
	return c.JSON(http.StatusOK, h.toProjectDTO(c.Request().Context(), project))
}

func projectIDFromContext(c echo.Context) (uuid.UUID, error) {
	if project := appMiddleware.GetProject(c); project != nil {
		return project.ID, nil
	}
	// Fallback for paths not mounted behind ProjectMiddleware.
	pid := c.Param("pid")
	if pid == "" {
		pid = c.Param("id")
	}
	return uuid.Parse(pid)
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
