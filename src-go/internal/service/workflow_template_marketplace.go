package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
)

// WorkflowTemplatePackage is the portable format for marketplace distribution.
// This is serialized as workflow.json inside the artifact ZIP.
type WorkflowTemplatePackage struct {
	APIVersion   string                `json:"apiVersion"` // "agentforge/workflow/v1"
	Kind         string                `json:"kind"`       // "WorkflowTemplate"
	Name         string                `json:"name"`
	Description  string                `json:"description"`
	Version      int                   `json:"version"`
	Nodes        []model.WorkflowNode  `json:"nodes"`
	Edges        []model.WorkflowEdge  `json:"edges"`
	TemplateVars map[string]any        `json:"templateVars,omitempty"`
}

// ExportTemplate serializes a workflow definition to a portable package.
func ExportTemplate(def *model.WorkflowDefinition) (*WorkflowTemplatePackage, error) {
	var nodes []model.WorkflowNode
	var edges []model.WorkflowEdge
	if err := json.Unmarshal(def.Nodes, &nodes); err != nil {
		return nil, fmt.Errorf("parse nodes: %w", err)
	}
	if err := json.Unmarshal(def.Edges, &edges); err != nil {
		return nil, fmt.Errorf("parse edges: %w", err)
	}

	var vars map[string]any
	if len(def.TemplateVars) > 0 {
		_ = json.Unmarshal(def.TemplateVars, &vars)
	}

	return &WorkflowTemplatePackage{
		APIVersion:   "agentforge/workflow/v1",
		Kind:         "WorkflowTemplate",
		Name:         def.Name,
		Description:  def.Description,
		Version:      def.Version,
		Nodes:        nodes,
		Edges:        edges,
		TemplateVars: vars,
	}, nil
}

// ImportTemplate creates a workflow template definition from a portable package.
func ImportTemplate(ctx context.Context, repo WorkflowTemplateRepo, projectID uuid.UUID, pkg *WorkflowTemplatePackage) (*model.WorkflowDefinition, error) {
	if pkg.Kind != "WorkflowTemplate" {
		return nil, fmt.Errorf("invalid package kind: %s (expected WorkflowTemplate)", pkg.Kind)
	}

	nodesJSON, err := json.Marshal(pkg.Nodes)
	if err != nil {
		return nil, fmt.Errorf("marshal nodes: %w", err)
	}
	edgesJSON, err := json.Marshal(pkg.Edges)
	if err != nil {
		return nil, fmt.Errorf("marshal edges: %w", err)
	}

	varsJSON := json.RawMessage("{}")
	if pkg.TemplateVars != nil {
		varsJSON, _ = json.Marshal(pkg.TemplateVars)
	}

	def := &model.WorkflowDefinition{
		ID:           uuid.New(),
		ProjectID:    projectID,
		Name:         pkg.Name,
		Description:  pkg.Description,
		Status:       model.WorkflowDefStatusTemplate,
		Category:     model.WorkflowCategoryMarketplace,
		Nodes:        nodesJSON,
		Edges:        edgesJSON,
		TemplateVars: varsJSON,
		Version:      pkg.Version,
	}

	if err := repo.Create(ctx, def); err != nil {
		return nil, fmt.Errorf("create template: %w", err)
	}
	return def, nil
}
