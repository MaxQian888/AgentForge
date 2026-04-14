package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	log "github.com/sirupsen/logrus"
)

// WorkflowTemplateRepo is the subset of DAGWorkflowDefinitionRepo needed by the template service,
// plus template-specific listing methods.
type WorkflowTemplateRepo interface {
	Create(ctx context.Context, def *model.WorkflowDefinition) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
	Update(ctx context.Context, id uuid.UUID, def *model.WorkflowDefinition) error
	ListTemplates(ctx context.Context, category string) ([]*model.WorkflowDefinition, error)
	ListTemplatesByName(ctx context.Context, name string) ([]*model.WorkflowDefinition, error)
}

// WorkflowTemplateService manages workflow templates: listing, cloning, seeding.
type WorkflowTemplateService struct {
	repo    WorkflowTemplateRepo
	dagSvc  *DAGWorkflowService
}

func NewWorkflowTemplateService(repo WorkflowTemplateRepo, dagSvc *DAGWorkflowService) *WorkflowTemplateService {
	return &WorkflowTemplateService{repo: repo, dagSvc: dagSvc}
}

// ListTemplates returns all workflow templates, optionally filtered by category.
func (s *WorkflowTemplateService) ListTemplates(ctx context.Context, category string) ([]*model.WorkflowDefinition, error) {
	return s.repo.ListTemplates(ctx, category)
}

// CloneTemplate creates a new active workflow definition from a template.
func (s *WorkflowTemplateService) CloneTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, overrides map[string]any) (*model.WorkflowDefinition, error) {
	tmpl, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	if tmpl.Status != model.WorkflowDefStatusTemplate {
		return nil, fmt.Errorf("definition %s is not a template (status: %s)", templateID, tmpl.Status)
	}

	// Merge overrides into template vars
	vars := make(map[string]any)
	if len(tmpl.TemplateVars) > 0 {
		_ = json.Unmarshal(tmpl.TemplateVars, &vars)
	}
	for k, v := range overrides {
		vars[k] = v
	}
	mergedVars, _ := json.Marshal(vars)

	clone := &model.WorkflowDefinition{
		ID:           uuid.New(),
		ProjectID:    projectID,
		Name:         tmpl.Name,
		Description:  tmpl.Description,
		Status:       model.WorkflowDefStatusActive,
		Category:     model.WorkflowCategoryUser,
		Nodes:        tmpl.Nodes,
		Edges:        tmpl.Edges,
		TemplateVars: mergedVars,
		Version:      tmpl.Version,
		SourceID:     &templateID,
	}

	if err := s.repo.Create(ctx, clone); err != nil {
		return nil, fmt.Errorf("create clone: %w", err)
	}
	return clone, nil
}

// CreateFromTemplate clones a template and immediately starts execution.
func (s *WorkflowTemplateService) CreateFromTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, taskID *uuid.UUID, variables map[string]any) (*model.WorkflowExecution, error) {
	clone, err := s.CloneTemplate(ctx, templateID, projectID, variables)
	if err != nil {
		return nil, err
	}
	return s.dagSvc.StartExecution(ctx, clone.ID, taskID)
}

// SeedSystemTemplates inserts or updates all built-in system templates.
// Templates are matched by name; existing ones are updated if the version is newer.
func (s *WorkflowTemplateService) SeedSystemTemplates(ctx context.Context) error {
	for _, tmpl := range AllSystemTemplates() {
		existing, err := s.repo.ListTemplatesByName(ctx, tmpl.Name)
		if err != nil {
			log.WithError(err).WithField("name", tmpl.Name).Warn("failed to check existing template")
			continue
		}

		if len(existing) == 0 {
			// Insert new template — use a nil project ID for system templates
			tmpl.ProjectID = uuid.Nil
			if err := s.repo.Create(ctx, tmpl); err != nil {
				log.WithError(err).WithField("name", tmpl.Name).Warn("failed to seed system template")
			} else {
				log.WithField("name", tmpl.Name).Info("seeded system workflow template")
			}
			continue
		}

		// Update if version is newer
		ex := existing[0]
		if tmpl.Version > ex.Version {
			updateDef := &model.WorkflowDefinition{
				Nodes:        tmpl.Nodes,
				Edges:        tmpl.Edges,
				Description:  tmpl.Description,
				TemplateVars: tmpl.TemplateVars,
				Version:      tmpl.Version,
			}
			if err := s.repo.Update(ctx, ex.ID, updateDef); err != nil {
				log.WithError(err).WithField("name", tmpl.Name).Warn("failed to update system template")
			} else {
				log.WithFields(log.Fields{
					"name":       tmpl.Name,
					"oldVersion": ex.Version,
					"newVersion": tmpl.Version,
				}).Info("updated system workflow template")
			}
		}
	}
	return nil
}
