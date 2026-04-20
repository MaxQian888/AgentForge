package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agentforge/server/internal/model"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

var (
	ErrWorkflowTemplateNotFound  = errors.New("workflow template not found")
	ErrWorkflowTemplateImmutable = errors.New("workflow template is immutable")
)

// WorkflowTemplateRepo is the subset of DAGWorkflowDefinitionRepo needed by the template service,
// plus template-specific listing methods.
type WorkflowTemplateRepo interface {
	Create(ctx context.Context, def *model.WorkflowDefinition) error
	GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error)
	Update(ctx context.Context, id uuid.UUID, def *model.WorkflowDefinition) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListTemplates(ctx context.Context, category string) ([]*model.WorkflowDefinition, error)
	ListTemplatesForProject(ctx context.Context, projectID uuid.UUID, query string, category string, source string) ([]*model.WorkflowDefinition, error)
	ListTemplatesByName(ctx context.Context, name string) ([]*model.WorkflowDefinition, error)
}

// WorkflowTemplateService manages workflow templates: listing, cloning, seeding.
type WorkflowTemplateService struct {
	repo   WorkflowTemplateRepo
	dagSvc *DAGWorkflowService
}

func NewWorkflowTemplateService(repo WorkflowTemplateRepo, dagSvc *DAGWorkflowService) *WorkflowTemplateService {
	return &WorkflowTemplateService{repo: repo, dagSvc: dagSvc}
}

// ListTemplates returns workflow templates visible to the current project.
func (s *WorkflowTemplateService) ListTemplates(ctx context.Context, projectID uuid.UUID, query string, category string, source string) ([]*model.WorkflowDefinition, error) {
	return s.repo.ListTemplatesForProject(ctx, projectID, query, category, source)
}

// CloneTemplate creates a new active workflow definition from a template.
func (s *WorkflowTemplateService) CloneTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, overrides map[string]any) (*model.WorkflowDefinition, error) {
	tmpl, err := s.repo.GetByID(ctx, templateID)
	if err != nil {
		return nil, ErrWorkflowTemplateNotFound
	}
	if tmpl.Status != model.WorkflowDefStatusTemplate {
		return nil, ErrWorkflowTemplateNotFound
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
	created, err := s.repo.GetByID(ctx, clone.ID)
	if err != nil {
		return nil, fmt.Errorf("load clone: %w", err)
	}
	return created, nil
}

func (s *WorkflowTemplateService) PublishDefinitionAsTemplate(ctx context.Context, definitionID uuid.UUID, projectID uuid.UUID, name string, description string) (*model.WorkflowDefinition, error) {
	def, err := s.repo.GetByID(ctx, definitionID)
	if err != nil || def.ProjectID != projectID {
		return nil, ErrWorkflowTemplateNotFound
	}
	template := &model.WorkflowDefinition{
		ID:           uuid.New(),
		ProjectID:    projectID,
		Name:         defaultWorkflowTemplateName(strings.TrimSpace(name), def.Name),
		Description:  defaultWorkflowTemplateDescription(strings.TrimSpace(description), def.Description),
		Status:       model.WorkflowDefStatusTemplate,
		Category:     model.WorkflowCategoryUser,
		Nodes:        def.Nodes,
		Edges:        def.Edges,
		TemplateVars: def.TemplateVars,
		Version:      maxWorkflowVersion(def.Version, 1),
		SourceID:     &definitionID,
	}
	if err := s.repo.Create(ctx, template); err != nil {
		return nil, fmt.Errorf("publish workflow template: %w", err)
	}
	created, err := s.repo.GetByID(ctx, template.ID)
	if err != nil {
		return nil, fmt.Errorf("load published workflow template: %w", err)
	}
	return created, nil
}

func (s *WorkflowTemplateService) DuplicateTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, name string, description string) (*model.WorkflowDefinition, error) {
	tmpl, err := s.repo.GetByID(ctx, templateID)
	if err != nil || tmpl.Status != model.WorkflowDefStatusTemplate {
		return nil, ErrWorkflowTemplateNotFound
	}
	duplicate := &model.WorkflowDefinition{
		ID:           uuid.New(),
		ProjectID:    projectID,
		Name:         defaultWorkflowTemplateName(strings.TrimSpace(name), tmpl.Name+" Copy"),
		Description:  defaultWorkflowTemplateDescription(strings.TrimSpace(description), tmpl.Description),
		Status:       model.WorkflowDefStatusTemplate,
		Category:     model.WorkflowCategoryUser,
		Nodes:        tmpl.Nodes,
		Edges:        tmpl.Edges,
		TemplateVars: tmpl.TemplateVars,
		Version:      maxWorkflowVersion(tmpl.Version, 1),
		SourceID:     &templateID,
	}
	if err := s.repo.Create(ctx, duplicate); err != nil {
		return nil, fmt.Errorf("duplicate workflow template: %w", err)
	}
	created, err := s.repo.GetByID(ctx, duplicate.ID)
	if err != nil {
		return nil, fmt.Errorf("load duplicated workflow template: %w", err)
	}
	return created, nil
}

func (s *WorkflowTemplateService) DeleteTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID) error {
	tmpl, err := s.repo.GetByID(ctx, templateID)
	if err != nil || tmpl.Status != model.WorkflowDefStatusTemplate {
		return ErrWorkflowTemplateNotFound
	}
	if tmpl.Category != model.WorkflowCategoryUser || tmpl.ProjectID != projectID {
		return ErrWorkflowTemplateImmutable
	}
	if err := s.repo.Delete(ctx, templateID); err != nil {
		return fmt.Errorf("delete workflow template: %w", err)
	}
	return nil
}

// CreateFromTemplate clones a template and immediately starts execution.
func (s *WorkflowTemplateService) CreateFromTemplate(ctx context.Context, templateID uuid.UUID, projectID uuid.UUID, taskID *uuid.UUID, variables map[string]any) (*model.WorkflowExecution, error) {
	clone, err := s.CloneTemplate(ctx, templateID, projectID, variables)
	if err != nil {
		return nil, err
	}
	return s.dagSvc.StartExecution(ctx, clone.ID, taskID, StartOptions{})
}

// CreateFromStrategy resolves a team-strategy name to a system workflow template
// and starts an execution from it. This is the seam team startup uses now that
// the legacy TeamStrategy interface has been removed.
func (s *WorkflowTemplateService) CreateFromStrategy(ctx context.Context, projectID, taskID uuid.UUID, strategy string, variables map[string]any) (*model.WorkflowExecution, error) {
	name := mapStrategyToTemplate(strategy)
	templates, err := s.repo.ListTemplatesByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("lookup system template %q for strategy %q: %w", name, strategy, err)
	}
	if len(templates) == 0 {
		return nil, fmt.Errorf("no system template for strategy %q (template %q)", strategy, name)
	}
	return s.CreateFromTemplate(ctx, templates[0].ID, projectID, &taskID, variables)
}

// mapStrategyToTemplate maps legacy team strategy names to seeded system
// workflow templates. Unknown values fall back to plan-code-review, matching
// the previous strategy-fallback behavior.
func mapStrategyToTemplate(strategy string) string {
	switch strategy {
	case "pipeline":
		return TemplatePipeline
	case "swarm":
		return TemplateSwarm
	case "plan-code-review", "wave-based", "":
		fallthrough
	default:
		return TemplatePlanCodeReview
	}
}

func defaultWorkflowTemplateName(name string, fallback string) string {
	if name != "" {
		return name
	}
	return fallback
}

func defaultWorkflowTemplateDescription(description string, fallback string) string {
	if description != "" {
		return description
	}
	return fallback
}

func maxWorkflowVersion(value int, fallback int) int {
	if value > 0 {
		return value
	}
	return fallback
}

// FindTemplateByName returns the first system template definition matching the
// given name. Used by callers (like ReviewService) that need to instantiate a
// specific named template rather than a user-chosen UUID.
func (s *WorkflowTemplateService) FindTemplateByName(ctx context.Context, name string) (*model.WorkflowDefinition, error) {
	templates, err := s.repo.ListTemplatesByName(ctx, name)
	if err != nil {
		return nil, err
	}
	for _, t := range templates {
		if t.Category == model.WorkflowCategorySystem && t.Status == model.WorkflowDefStatusTemplate {
			return t, nil
		}
	}
	return nil, ErrWorkflowTemplateNotFound
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
