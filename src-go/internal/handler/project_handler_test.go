package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/react-go-quick-starter/server/internal/bridge"
	"github.com/react-go-quick-starter/server/internal/handler"
	"github.com/react-go-quick-starter/server/internal/model"
)

type projectTestValidator struct {
	validator *validator.Validate
}

func (v *projectTestValidator) Validate(i interface{}) error {
	return v.validator.Struct(i)
}

type mockProjectRepo struct {
	projects     map[uuid.UUID]*model.Project
	lastCreate   *model.Project
	lastUpdateID uuid.UUID
	lastUpdate   *model.UpdateProjectRequest
}

func (m *mockProjectRepo) Create(_ context.Context, project *model.Project) error {
	cloned := *project
	if m.projects == nil {
		m.projects = make(map[uuid.UUID]*model.Project)
	}
	m.projects[project.ID] = &cloned
	m.lastCreate = &cloned
	return nil
}

func (m *mockProjectRepo) GetByID(_ context.Context, id uuid.UUID) (*model.Project, error) {
	project, ok := m.projects[id]
	if !ok {
		return nil, errors.New("project not found")
	}
	cloned := *project
	return &cloned, nil
}

func (m *mockProjectRepo) List(_ context.Context) ([]*model.Project, error) {
	projects := make([]*model.Project, 0, len(m.projects))
	for _, project := range m.projects {
		cloned := *project
		projects = append(projects, &cloned)
	}
	return projects, nil
}

func (m *mockProjectRepo) Update(_ context.Context, id uuid.UUID, req *model.UpdateProjectRequest) error {
	project, ok := m.projects[id]
	if !ok {
		return errors.New("project not found")
	}
	m.lastUpdateID = id
	m.lastUpdate = req
	if req.Settings != nil {
		merged, err := model.MergeProjectSettings(project.Settings, req.Settings)
		if err != nil {
			return err
		}
		project.Settings = merged
	}
	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.RepoURL != nil {
		project.RepoURL = *req.RepoURL
	}
	if req.DefaultBranch != nil {
		project.DefaultBranch = *req.DefaultBranch
	}
	return nil
}

func (m *mockProjectRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(m.projects, id)
	return nil
}

type mockProjectRuntimeCatalogClient struct {
	response *bridge.RuntimeCatalogResponse
	err      error
}

func (m *mockProjectRuntimeCatalogClient) GetRuntimeCatalog(_ context.Context) (*bridge.RuntimeCatalogResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

type mockProjectWikiBootstrap struct {
	spaceCreatedFor uuid.UUID
	templateSeeded  bool
	deleteCalledFor uuid.UUID
}

func (m *mockProjectWikiBootstrap) CreateSpace(_ context.Context, projectID uuid.UUID) (*model.WikiSpace, error) {
	m.spaceCreatedFor = projectID
	return &model.WikiSpace{ID: uuid.New(), ProjectID: projectID}, nil
}

func (m *mockProjectWikiBootstrap) SeedBuiltInTemplates(_ context.Context, projectID uuid.UUID, spaceID uuid.UUID) ([]*model.WikiPage, error) {
	_ = projectID
	_ = spaceID
	m.templateSeeded = true
	return []*model.WikiPage{{ID: uuid.New(), SpaceID: spaceID, Title: "PRD"}}, nil
}

func (m *mockProjectWikiBootstrap) DeleteProjectSpace(_ context.Context, projectID uuid.UUID) error {
	m.deleteCalledFor = projectID
	return nil
}

func newProjectTestEcho() *echo.Echo {
	e := echo.New()
	e.Validator = &projectTestValidator{validator: validator.New()}
	return e
}

func TestProjectHandler_GetIncludesRuntimeCatalogAndResolvedDefaults(t *testing.T) {
	projectID := uuid.New()
	repo := &mockProjectRepo{
		projects: map[uuid.UUID]*model.Project{
			projectID: {
				ID:            projectID,
				Name:          "AgentForge",
				Slug:          "agentforge",
				DefaultBranch: "main",
				Settings:      `{"coding_agent":{"runtime":"codex","provider":"openai","model":"gpt-5-codex"}}`,
			},
		},
	}
	client := &mockProjectRuntimeCatalogClient{
		response: &bridge.RuntimeCatalogResponse{
			DefaultRuntime: "claude_code",
			Runtimes: []bridge.RuntimeCatalogEntryDTO{
				{
					Key:                 "codex",
					Label:               "Codex",
					DefaultProvider:     "openai",
					CompatibleProviders: []string{"openai", "codex"},
					DefaultModel:        "gpt-5-codex",
					ModelOptions:        []string{"gpt-5-codex", "o3"},
					Available:           true,
					SupportedFeatures:   []string{"reasoning", "fork"},
				},
			},
		},
	}

	e := newProjectTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+projectID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:id")
	c.SetParamNames("id")
	c.SetParamValues(projectID.String())

	h := handler.NewProjectHandler(repo, client)
	if err := h.Get(c); err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body model.ProjectDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body.Settings.CodingAgent.Runtime != "codex" || body.Settings.CodingAgent.Provider != "openai" || body.Settings.CodingAgent.Model != "gpt-5-codex" {
		t.Fatalf("settings coding agent = %+v", body.Settings.CodingAgent)
	}
	if body.CodingAgentCatalog == nil {
		t.Fatal("expected coding agent catalog")
	}
	if body.CodingAgentCatalog.DefaultSelection.Runtime != "codex" {
		t.Fatalf("default selection = %+v, want codex", body.CodingAgentCatalog.DefaultSelection)
	}
	if len(body.CodingAgentCatalog.Runtimes) != 1 || body.CodingAgentCatalog.Runtimes[0].Runtime != "codex" {
		t.Fatalf("runtime catalog = %+v", body.CodingAgentCatalog.Runtimes)
	}
	if len(body.CodingAgentCatalog.Runtimes[0].ModelOptions) != 2 {
		t.Fatalf("model options = %+v, want two codex options", body.CodingAgentCatalog.Runtimes[0].ModelOptions)
	}
	if len(body.CodingAgentCatalog.Runtimes[0].SupportedFeatures) != 2 {
		t.Fatalf("supported features = %+v, want two codex features", body.CodingAgentCatalog.Runtimes[0].SupportedFeatures)
	}
}

func TestProjectHandler_UpdateFallsBackToDefaultCatalogWhenBridgeUnavailable(t *testing.T) {
	projectID := uuid.New()
	repo := &mockProjectRepo{
		projects: map[uuid.UUID]*model.Project{
			projectID: {
				ID:            projectID,
				Name:          "AgentForge",
				Slug:          "agentforge",
				DefaultBranch: "main",
				Settings:      "{}",
			},
		},
	}
	client := &mockProjectRuntimeCatalogClient{err: errors.New("bridge unavailable")}

	e := newProjectTestEcho()
	req := httptest.NewRequest(
		http.MethodPut,
		"/api/v1/projects/"+projectID.String(),
		strings.NewReader(`{"settings":{"codingAgent":{"runtime":"opencode","provider":"opencode","model":"opencode-default"}}}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:id")
	c.SetParamNames("id")
	c.SetParamValues(projectID.String())

	h := handler.NewProjectHandler(repo, client)
	if err := h.Update(c); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var body model.ProjectDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if body.Settings.CodingAgent.Runtime != "opencode" {
		t.Fatalf("updated runtime = %q, want opencode", body.Settings.CodingAgent.Runtime)
	}
	if body.CodingAgentCatalog == nil {
		t.Fatal("expected fallback coding agent catalog")
	}
	if body.CodingAgentCatalog.DefaultSelection.Runtime != "opencode" {
		t.Fatalf("fallback default selection = %+v", body.CodingAgentCatalog.DefaultSelection)
	}
	if len(body.CodingAgentCatalog.Runtimes) < 3 {
		t.Fatalf("fallback runtime catalog = %+v", body.CodingAgentCatalog.Runtimes)
	}
}

func TestProjectHandler_CreateBootstrapsWikiSpace(t *testing.T) {
	repo := &mockProjectRepo{}
	client := &mockProjectRuntimeCatalogClient{}
	bootstrap := &mockProjectWikiBootstrap{}

	e := newProjectTestEcho()
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/projects",
		strings.NewReader(`{"name":"Docs","slug":"docs","description":"wiki"}`),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handler.NewProjectHandler(repo, client, bootstrap)
	if err := h.Create(c); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if repo.lastCreate == nil {
		t.Fatal("expected project create to be called")
	}
	if bootstrap.spaceCreatedFor != repo.lastCreate.ID {
		t.Fatalf("space bootstrap project = %s, want %s", bootstrap.spaceCreatedFor, repo.lastCreate.ID)
	}
	if !bootstrap.templateSeeded {
		t.Fatal("expected built-in templates to be seeded")
	}
}
