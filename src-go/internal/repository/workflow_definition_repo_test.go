package repository

import (
	"context"
	"testing"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
)

func TestWorkflowDefinitionRepository_ListTemplatesForProject(t *testing.T) {
	db := openFoundationRepoTestDB(t, &workflowDefinitionRecord{})
	repo := NewWorkflowDefinitionRepository(db)
	projectID := uuid.New()
	otherProjectID := uuid.New()

	mustCreate := func(def *model.WorkflowDefinition) {
		t.Helper()
		if err := repo.Create(context.Background(), def); err != nil {
			t.Fatalf("Create() error = %v", err)
		}
	}

	mustCreate(&model.WorkflowDefinition{
		ID:          uuid.New(),
		ProjectID:   uuid.Nil,
		Name:        "System Template",
		Description: "System",
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategorySystem,
		Version:     1,
	})
	mustCreate(&model.WorkflowDefinition{
		ID:          uuid.New(),
		ProjectID:   uuid.Nil,
		Name:        "Marketplace Template",
		Description: "Marketplace",
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategoryMarketplace,
		Version:     1,
	})
	mustCreate(&model.WorkflowDefinition{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        "Project Template",
		Description: "Custom",
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategoryUser,
		Version:     1,
	})
	mustCreate(&model.WorkflowDefinition{
		ID:          uuid.New(),
		ProjectID:   otherProjectID,
		Name:        "Foreign Template",
		Description: "Should not leak",
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategoryUser,
		Version:     1,
	})

	templates, err := repo.ListTemplatesForProject(context.Background(), projectID, "", "", "")
	if err != nil {
		t.Fatalf("ListTemplatesForProject() error = %v", err)
	}
	if len(templates) != 3 {
		t.Fatalf("template count = %d, want 3", len(templates))
	}

	for _, template := range templates {
		if template.Name == "Foreign Template" {
			t.Fatalf("foreign project template leaked into project-scoped list: %+v", template)
		}
	}
}

func TestWorkflowDefinitionRepository_ListTemplatesForProjectHonorsSourceAndQueryFilters(t *testing.T) {
	db := openFoundationRepoTestDB(t, &workflowDefinitionRecord{})
	repo := NewWorkflowDefinitionRepository(db)
	projectID := uuid.New()

	if err := repo.Create(context.Background(), &model.WorkflowDefinition{
		ID:          uuid.New(),
		ProjectID:   projectID,
		Name:        "Project Delivery",
		Description: "Custom delivery workflow",
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategoryUser,
		Version:     1,
	}); err != nil {
		t.Fatalf("Create(custom) error = %v", err)
	}
	if err := repo.Create(context.Background(), &model.WorkflowDefinition{
		ID:          uuid.New(),
		ProjectID:   uuid.Nil,
		Name:        "System Review",
		Description: "System workflow",
		Status:      model.WorkflowDefStatusTemplate,
		Category:    model.WorkflowCategorySystem,
		Version:     1,
	}); err != nil {
		t.Fatalf("Create(system) error = %v", err)
	}

	customTemplates, err := repo.ListTemplatesForProject(context.Background(), projectID, "delivery", "", model.WorkflowCategoryUser)
	if err != nil {
		t.Fatalf("ListTemplatesForProject(custom) error = %v", err)
	}
	if len(customTemplates) != 1 || customTemplates[0].Name != "Project Delivery" {
		t.Fatalf("customTemplates = %+v", customTemplates)
	}
}
