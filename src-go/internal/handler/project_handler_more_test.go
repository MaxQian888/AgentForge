package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentforge/server/internal/handler"
	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestProjectHandlerListReturnsProjectDTOs(t *testing.T) {
	projectA := uuid.New()
	projectB := uuid.New()
	repo := &mockProjectRepo{
		projects: map[uuid.UUID]*model.Project{
			projectA: {
				ID:            projectA,
				Name:          "AgentForge",
				Slug:          "agentforge",
				DefaultBranch: "main",
				Settings:      "{}",
			},
			projectB: {
				ID:            projectB,
				Name:          "Docs",
				Slug:          "docs",
				DefaultBranch: "main",
				Settings:      "{}",
			},
		},
	}

	e := newProjectTestEcho()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handler.NewProjectHandler(repo, nil)
	if err := h.List(c); err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var body []model.ProjectDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("len(body) = %d, want 2", len(body))
	}
}

func TestProjectHandlerDeleteRemovesProjectAndWikiSpace(t *testing.T) {
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
	bootstrap := &mockProjectWikiBootstrap{}

	e := newProjectTestEcho()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+projectID.String(), nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath("/api/v1/projects/:id")
	c.SetParamNames("id")
	c.SetParamValues(projectID.String())

	h := handler.NewProjectHandler(repo, nil, bootstrap)
	if err := h.Delete(c); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if bootstrap.deleteCalledFor != projectID {
		t.Fatalf("bootstrap delete = %s, want %s", bootstrap.deleteCalledFor, projectID)
	}
	if _, ok := repo.projects[projectID]; ok {
		t.Fatal("project should be removed from repository")
	}
}
