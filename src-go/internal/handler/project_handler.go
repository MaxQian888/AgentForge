package handler

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/service"
)

type ProjectHandler struct {
	repo          ProjectRepository
	runtimeClient ProjectRuntimeCatalogClient
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

func NewProjectHandler(repo ProjectRepository, runtimeClient ProjectRuntimeCatalogClient) *ProjectHandler {
	return &ProjectHandler{repo: repo, runtimeClient: runtimeClient}
}

func (h *ProjectHandler) Create(c echo.Context) error {
	req := new(model.CreateProjectRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
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
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to create project"})
	}
	return c.JSON(http.StatusCreated, h.toProjectDTO(c.Request().Context(), project))
}

func (h *ProjectHandler) List(c echo.Context) error {
	projects, err := h.repo.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to list projects"})
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
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid project ID"})
	}
	project, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "project not found"})
	}
	return c.JSON(http.StatusOK, h.toProjectDTO(c.Request().Context(), project))
}

func (h *ProjectHandler) Update(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid project ID"})
	}
	req := new(model.UpdateProjectRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid request body"})
	}
	if err := h.repo.Update(c.Request().Context(), id, req); err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to update project"})
	}
	project, err := h.repo.GetByID(c.Request().Context(), id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, model.ErrorResponse{Message: "failed to fetch updated project"})
	}
	return c.JSON(http.StatusOK, h.toProjectDTO(c.Request().Context(), project))
}

func (h *ProjectHandler) Delete(c echo.Context) error {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, model.ErrorResponse{Message: "invalid project ID"})
	}
	if err := h.repo.Delete(c.Request().Context(), id); err != nil {
		return c.JSON(http.StatusNotFound, model.ErrorResponse{Message: "project not found"})
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
			Runtime:             runtime.Key,
			Label:               runtime.Label,
			DefaultProvider:     runtime.DefaultProvider,
			CompatibleProviders: append([]string(nil), runtime.CompatibleProviders...),
			DefaultModel:        runtime.DefaultModel,
			Available:           runtime.Available,
			Diagnostics:         diagnostics,
		})
	}

	defaultRuntime := strings.TrimSpace(runtimeCatalog.DefaultRuntime)
	if defaultRuntime == "" {
		defaultRuntime = model.DefaultCodingAgentRuntime
	}

	return &model.CodingAgentCatalogDTO{
		DefaultRuntime:   defaultRuntime,
		DefaultSelection: selection,
		Runtimes:         runtimes,
	}
}
