package service

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
)

// SubWorkflowLinkRepoBacking is the repository subset the applier bridge
// needs. *repository.WorkflowRunParentLinkRepository satisfies it structurally.
type SubWorkflowLinkRepoBacking interface {
	Create(ctx context.Context, link *model.WorkflowRunParentLink) error
	GetByParent(ctx context.Context, parentExecutionID uuid.UUID, parentNodeID string) (*model.WorkflowRunParentLink, error)
	GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*model.WorkflowRunParentLink, error)
	ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*model.WorkflowRunParentLink, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
}

// SubWorkflowLinkRepoAdapter bridges the persistence-layer parent-link
// repository onto the nodetypes applier's SubWorkflowLinkRepo interface.
// Conversions are intentional: the applier operates on its own transport
// shape so nodetypes stays free of the model/repository import plane.
type SubWorkflowLinkRepoAdapter struct {
	Backing SubWorkflowLinkRepoBacking
}

// NewSubWorkflowLinkRepoAdapter constructs the adapter.
func NewSubWorkflowLinkRepoAdapter(backing SubWorkflowLinkRepoBacking) *SubWorkflowLinkRepoAdapter {
	return &SubWorkflowLinkRepoAdapter{Backing: backing}
}

// Create persists a parent↔child link, converting from the applier's
// transport into the model struct the repo expects.
func (a *SubWorkflowLinkRepoAdapter) Create(ctx context.Context, link *nodetypes.SubWorkflowLinkRecord) error {
	if a.Backing == nil || link == nil {
		return nil
	}
	parentKind := link.ParentKind
	if parentKind == "" {
		parentKind = model.SubWorkflowParentKindDAGExecution
	}
	return a.Backing.Create(ctx, &model.WorkflowRunParentLink{
		ID:                link.ID,
		ParentExecutionID: link.ParentExecutionID,
		ParentKind:        parentKind,
		ParentNodeID:      link.ParentNodeID,
		ChildEngineKind:   link.ChildEngineKind,
		ChildRunID:        link.ChildRunID,
		Status:            link.Status,
	})
}

// GetByParent returns the applier-shaped record for (parentExecID, parentNodeID).
func (a *SubWorkflowLinkRepoAdapter) GetByParent(ctx context.Context, parentExecutionID uuid.UUID, parentNodeID string) (*nodetypes.SubWorkflowLinkRecord, error) {
	if a.Backing == nil {
		return nil, nil
	}
	row, err := a.Backing.GetByParent(ctx, parentExecutionID, parentNodeID)
	if err != nil {
		return nil, err
	}
	return modelLinkToApplier(row), nil
}

// GetByChild returns the applier-shaped record for (engineKind, childRunID).
func (a *SubWorkflowLinkRepoAdapter) GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*nodetypes.SubWorkflowLinkRecord, error) {
	if a.Backing == nil {
		return nil, nil
	}
	row, err := a.Backing.GetByChild(ctx, engineKind, childRunID)
	if err != nil {
		return nil, err
	}
	return modelLinkToApplier(row), nil
}

// ListByParentExecution returns every link row originating at parentExecutionID.
func (a *SubWorkflowLinkRepoAdapter) ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*nodetypes.SubWorkflowLinkRecord, error) {
	if a.Backing == nil {
		return nil, nil
	}
	rows, err := a.Backing.ListByParentExecution(ctx, parentExecutionID)
	if err != nil {
		return nil, err
	}
	out := make([]*nodetypes.SubWorkflowLinkRecord, 0, len(rows))
	for _, r := range rows {
		out = append(out, modelLinkToApplier(r))
	}
	return out, nil
}

// UpdateStatus flips a link row's status field.
func (a *SubWorkflowLinkRepoAdapter) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	if a.Backing == nil {
		return nil
	}
	return a.Backing.UpdateStatus(ctx, id, status)
}

func modelLinkToApplier(l *model.WorkflowRunParentLink) *nodetypes.SubWorkflowLinkRecord {
	if l == nil {
		return nil
	}
	parentKind := l.ParentKind
	if parentKind == "" {
		parentKind = model.SubWorkflowParentKindDAGExecution
	}
	return &nodetypes.SubWorkflowLinkRecord{
		ID:                l.ID,
		ParentExecutionID: l.ParentExecutionID,
		ParentKind:        parentKind,
		ParentNodeID:      l.ParentNodeID,
		ChildEngineKind:   l.ChildEngineKind,
		ChildRunID:        l.ChildRunID,
		Status:            l.Status,
	}
}

// PluginSubWorkflowTerminalBridge is the WorkflowPluginTerminalObserver that
// flows plugin-run terminal transitions back into the DAG service's resume
// path. Wired during startup once both services are constructed; a plugin
// run that wasn't started as a sub-workflow child is a no-op by design.
type PluginSubWorkflowTerminalBridge struct {
	DAG *DAGWorkflowService
}

// OnPluginRunTerminal is called by the workflow execution service whenever a
// plugin run reaches completed/failed/cancelled. It maps the terminal status
// to the parent-link status vocabulary and hands off to the DAG service.
func (b *PluginSubWorkflowTerminalBridge) OnPluginRunTerminal(ctx context.Context, run *model.WorkflowPluginRun) {
	if b == nil || b.DAG == nil || run == nil {
		return
	}
	linkStatus := model.SubWorkflowLinkStatusCompleted
	switch run.Status {
	case model.WorkflowRunStatusFailed:
		linkStatus = model.SubWorkflowLinkStatusFailed
	case model.WorkflowRunStatusCancelled:
		linkStatus = model.SubWorkflowLinkStatusCancelled
	case model.WorkflowRunStatusCompleted:
		linkStatus = model.SubWorkflowLinkStatusCompleted
	default:
		// Not a terminal state — no resume needed.
		return
	}
	outputs := extractPluginRunOutputs(run)
	b.DAG.ResumeParentFromPluginChild(ctx, run.ID, linkStatus, outputs)
}

// extractPluginRunOutputs returns a JSON envelope summarizing the plugin
// run's final outputs for consumption by the parent's dataStore. Shape
// mirrors the DAG child's DataStore so downstream template references work
// the same regardless of child engine.
func extractPluginRunOutputs(run *model.WorkflowPluginRun) json.RawMessage {
	if run == nil || len(run.Steps) == 0 {
		return json.RawMessage(`{}`)
	}
	summary := map[string]any{}
	for _, step := range run.Steps {
		if step.Output == nil {
			continue
		}
		summary[step.StepID] = map[string]any{"output": step.Output}
	}
	raw, err := json.Marshal(summary)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return raw
}
