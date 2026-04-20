package service

import (
	"context"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

type stubWorkflowTemplateRepo struct {
	definitions     map[uuid.UUID]*model.WorkflowDefinition
	listProjectArgs struct {
		projectID uuid.UUID
		query     string
		category  string
		source    string
	}
}

func (r *stubWorkflowTemplateRepo) Create(_ context.Context, def *model.WorkflowDefinition) error {
	if r.definitions == nil {
		r.definitions = map[uuid.UUID]*model.WorkflowDefinition{}
	}
	cloned := *def
	r.definitions[def.ID] = &cloned
	return nil
}

func (r *stubWorkflowTemplateRepo) GetByID(_ context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	def, ok := r.definitions[id]
	if !ok {
		return nil, ErrWorkflowTemplateNotFound
	}
	cloned := *def
	return &cloned, nil
}

func (r *stubWorkflowTemplateRepo) Update(_ context.Context, id uuid.UUID, def *model.WorkflowDefinition) error {
	current, ok := r.definitions[id]
	if !ok {
		return ErrWorkflowTemplateNotFound
	}
	if def.Name != "" {
		current.Name = def.Name
	}
	if def.Description != "" {
		current.Description = def.Description
	}
	return nil
}

func (r *stubWorkflowTemplateRepo) Delete(_ context.Context, id uuid.UUID) error {
	delete(r.definitions, id)
	return nil
}

func (r *stubWorkflowTemplateRepo) ListTemplates(_ context.Context, _ string) ([]*model.WorkflowDefinition, error) {
	out := make([]*model.WorkflowDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		cloned := *def
		out = append(out, &cloned)
	}
	return out, nil
}

func (r *stubWorkflowTemplateRepo) ListTemplatesForProject(_ context.Context, projectID uuid.UUID, query string, category string, source string) ([]*model.WorkflowDefinition, error) {
	r.listProjectArgs = struct {
		projectID uuid.UUID
		query     string
		category  string
		source    string
	}{
		projectID: projectID,
		query:     query,
		category:  category,
		source:    source,
	}
	out := make([]*model.WorkflowDefinition, 0)
	for _, def := range r.definitions {
		if def.Status != model.WorkflowDefStatusTemplate {
			continue
		}
		if def.Category == model.WorkflowCategoryUser && def.ProjectID != projectID {
			continue
		}
		cloned := *def
		out = append(out, &cloned)
	}
	return out, nil
}

func (r *stubWorkflowTemplateRepo) ListTemplatesByName(_ context.Context, name string) ([]*model.WorkflowDefinition, error) {
	out := make([]*model.WorkflowDefinition, 0)
	for _, def := range r.definitions {
		if def.Name == name {
			cloned := *def
			out = append(out, &cloned)
		}
	}
	return out, nil
}

func TestWorkflowTemplateService_ListTemplatesUsesProjectScopedRepoCall(t *testing.T) {
	projectID := uuid.New()
	repo := &stubWorkflowTemplateRepo{
		definitions: map[uuid.UUID]*model.WorkflowDefinition{
			uuid.New(): {
				ID:        uuid.New(),
				ProjectID: projectID,
				Name:      "Project Template",
				Status:    model.WorkflowDefStatusTemplate,
				Category:  model.WorkflowCategoryUser,
				Version:   1,
			},
		},
	}
	svc := NewWorkflowTemplateService(repo, nil)

	if _, err := svc.ListTemplates(context.Background(), projectID, "delivery", "", "user"); err != nil {
		t.Fatalf("ListTemplates() error = %v", err)
	}
	if repo.listProjectArgs.projectID != projectID || repo.listProjectArgs.query != "delivery" || repo.listProjectArgs.source != "user" {
		t.Fatalf("project scoped list args = %+v", repo.listProjectArgs)
	}
}

func TestWorkflowTemplateService_PublishAndDuplicateCreateProjectOwnedTemplates(t *testing.T) {
	projectID := uuid.New()
	definitionID := uuid.New()
	templateID := uuid.New()
	repo := &stubWorkflowTemplateRepo{
		definitions: map[uuid.UUID]*model.WorkflowDefinition{
			definitionID: {
				ID:          definitionID,
				ProjectID:   projectID,
				Name:        "Delivery Flow",
				Description: "Workflow",
				Status:      model.WorkflowDefStatusActive,
				Category:    model.WorkflowCategoryUser,
				Version:     2,
			},
			templateID: {
				ID:          templateID,
				ProjectID:   uuid.Nil,
				Name:        "System Template",
				Description: "System",
				Status:      model.WorkflowDefStatusTemplate,
				Category:    model.WorkflowCategorySystem,
				Version:     1,
			},
		},
	}
	svc := NewWorkflowTemplateService(repo, nil)

	published, err := svc.PublishDefinitionAsTemplate(context.Background(), definitionID, projectID, "Published Template", "Reusable")
	if err != nil {
		t.Fatalf("PublishDefinitionAsTemplate() error = %v", err)
	}
	if published.Status != model.WorkflowDefStatusTemplate || published.Category != model.WorkflowCategoryUser || published.ProjectID != projectID {
		t.Fatalf("published template = %+v", published)
	}
	if published.SourceID == nil || *published.SourceID != definitionID {
		t.Fatalf("published source = %+v", published.SourceID)
	}

	duplicated, err := svc.DuplicateTemplate(context.Background(), templateID, projectID, "System Copy", "Project-owned")
	if err != nil {
		t.Fatalf("DuplicateTemplate() error = %v", err)
	}
	if duplicated.Category != model.WorkflowCategoryUser || duplicated.ProjectID != projectID {
		t.Fatalf("duplicated template = %+v", duplicated)
	}
	if duplicated.SourceID == nil || *duplicated.SourceID != templateID {
		t.Fatalf("duplicated source = %+v", duplicated.SourceID)
	}
}

func TestWorkflowTemplateService_DeleteTemplateRejectsImmutableSources(t *testing.T) {
	projectID := uuid.New()
	customTemplateID := uuid.New()
	systemTemplateID := uuid.New()
	repo := &stubWorkflowTemplateRepo{
		definitions: map[uuid.UUID]*model.WorkflowDefinition{
			customTemplateID: {
				ID:        customTemplateID,
				ProjectID: projectID,
				Name:      "Custom Template",
				Status:    model.WorkflowDefStatusTemplate,
				Category:  model.WorkflowCategoryUser,
				Version:   1,
			},
			systemTemplateID: {
				ID:        systemTemplateID,
				ProjectID: uuid.Nil,
				Name:      "System Template",
				Status:    model.WorkflowDefStatusTemplate,
				Category:  model.WorkflowCategorySystem,
				Version:   1,
			},
		},
	}
	svc := NewWorkflowTemplateService(repo, nil)

	if err := svc.DeleteTemplate(context.Background(), customTemplateID, projectID); err != nil {
		t.Fatalf("DeleteTemplate(custom) error = %v", err)
	}
	if _, ok := repo.definitions[customTemplateID]; ok {
		t.Fatalf("custom template should be deleted: %+v", repo.definitions[customTemplateID])
	}

	if err := svc.DeleteTemplate(context.Background(), systemTemplateID, projectID); err != ErrWorkflowTemplateImmutable {
		t.Fatalf("DeleteTemplate(system) error = %v, want %v", err, ErrWorkflowTemplateImmutable)
	}
}
