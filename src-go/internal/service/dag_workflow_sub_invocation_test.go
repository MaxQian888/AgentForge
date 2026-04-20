package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/react-go-quick-starter/server/internal/model"
	"github.com/react-go-quick-starter/server/internal/workflow/nodetypes"
)

// ── Fakes for the resume-path tests ─────────────────────────────────────

type fakeExecRepo struct {
	byID     map[uuid.UUID]*model.WorkflowExecution
	updated  []string // log of status updates: "<execID>:<status>"
	datastore map[uuid.UUID]json.RawMessage
}

func (r *fakeExecRepo) CreateExecution(ctx context.Context, exec *model.WorkflowExecution) error {
	r.byID[exec.ID] = exec
	return nil
}
func (r *fakeExecRepo) GetExecution(ctx context.Context, id uuid.UUID) (*model.WorkflowExecution, error) {
	if e, ok := r.byID[id]; ok {
		return e, nil
	}
	return nil, errors.New("not found")
}
func (r *fakeExecRepo) ListExecutions(ctx context.Context, workflowID uuid.UUID) ([]*model.WorkflowExecution, error) {
	return nil, nil
}
func (r *fakeExecRepo) ListActiveByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowExecution, error) {
	return nil, nil
}
func (r *fakeExecRepo) UpdateExecution(ctx context.Context, id uuid.UUID, status string, currentNodes json.RawMessage, errorMessage string) error {
	if e, ok := r.byID[id]; ok {
		e.Status = status
		if len(currentNodes) > 0 {
			e.CurrentNodes = currentNodes
		}
		e.ErrorMessage = errorMessage
	}
	r.updated = append(r.updated, id.String()+":"+status)
	return nil
}
func (r *fakeExecRepo) UpdateExecutionDataStore(ctx context.Context, id uuid.UUID, dataStore json.RawMessage) error {
	if r.datastore == nil {
		r.datastore = map[uuid.UUID]json.RawMessage{}
	}
	r.datastore[id] = dataStore
	if e, ok := r.byID[id]; ok {
		e.DataStore = dataStore
	}
	return nil
}
func (r *fakeExecRepo) CompleteExecution(ctx context.Context, id uuid.UUID, status string) error {
	if e, ok := r.byID[id]; ok {
		e.Status = status
	}
	return nil
}

type fakeNodeRepo struct {
	byExec map[uuid.UUID][]*model.WorkflowNodeExecution
	// track update calls for assertions
	updates []string // "<id>:<status>"
}

func (r *fakeNodeRepo) CreateNodeExecution(ctx context.Context, ne *model.WorkflowNodeExecution) error {
	r.byExec[ne.ExecutionID] = append(r.byExec[ne.ExecutionID], ne)
	return nil
}
func (r *fakeNodeRepo) UpdateNodeExecution(ctx context.Context, id uuid.UUID, status string, result json.RawMessage, errorMessage string) error {
	for _, lst := range r.byExec {
		for _, ne := range lst {
			if ne.ID == id {
				ne.Status = status
				if len(result) > 0 {
					ne.Result = result
				}
				ne.ErrorMessage = errorMessage
			}
		}
	}
	r.updates = append(r.updates, id.String()+":"+status)
	return nil
}
func (r *fakeNodeRepo) ListNodeExecutions(ctx context.Context, executionID uuid.UUID) ([]*model.WorkflowNodeExecution, error) {
	return r.byExec[executionID], nil
}
func (r *fakeNodeRepo) DeleteNodeExecutionsByNodeIDs(ctx context.Context, executionID uuid.UUID, nodeIDs []string) error {
	return nil
}

type fakeLinkRepo struct {
	links         []*model.WorkflowRunParentLink
	statusUpdates []string // "<id>:<status>"
}

func (r *fakeLinkRepo) Create(ctx context.Context, link *model.WorkflowRunParentLink) error {
	r.links = append(r.links, link)
	return nil
}
func (r *fakeLinkRepo) GetByParent(ctx context.Context, parentExecutionID uuid.UUID, parentNodeID string) (*model.WorkflowRunParentLink, error) {
	for _, l := range r.links {
		if l.ParentExecutionID == parentExecutionID && l.ParentNodeID == parentNodeID {
			return l, nil
		}
	}
	return nil, errors.New("not found")
}
func (r *fakeLinkRepo) GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*model.WorkflowRunParentLink, error) {
	for _, l := range r.links {
		if l.ChildEngineKind == engineKind && l.ChildRunID == childRunID {
			return l, nil
		}
	}
	return nil, errors.New("not found")
}
func (r *fakeLinkRepo) ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*model.WorkflowRunParentLink, error) {
	var out []*model.WorkflowRunParentLink
	for _, l := range r.links {
		if l.ParentExecutionID == parentExecutionID {
			out = append(out, l)
		}
	}
	return out, nil
}
func (r *fakeLinkRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	for _, l := range r.links {
		if l.ID == id {
			l.Status = status
		}
	}
	r.statusUpdates = append(r.statusUpdates, id.String()+":"+status)
	return nil
}

// newResumeTestService builds a service with enough wiring to exercise resume
// paths but no real registry / applier — handlers and effects aren't invoked
// during resume.
func newResumeTestService(t *testing.T) (*DAGWorkflowService, *fakeExecRepo, *fakeNodeRepo, *fakeLinkRepo) {
	t.Helper()
	execRepo := &fakeExecRepo{byID: map[uuid.UUID]*model.WorkflowExecution{}}
	nodeRepo := &fakeNodeRepo{byExec: map[uuid.UUID][]*model.WorkflowNodeExecution{}}
	linkRepo := &fakeLinkRepo{}
	defRepo := &fakeDefRepo{byID: map[uuid.UUID]*model.WorkflowDefinition{}}

	registry := nodetypes.NewRegistry(nil)
	svc := NewDAGWorkflowService(defRepo, execRepo, nodeRepo, nil, nil, registry, &nodetypes.EffectApplier{})
	svc.SetParentLinkRepo(linkRepo)
	return svc, execRepo, nodeRepo, linkRepo
}

type fakeDefRepo struct {
	byID map[uuid.UUID]*model.WorkflowDefinition
}

func (r *fakeDefRepo) Create(ctx context.Context, def *model.WorkflowDefinition) error {
	r.byID[def.ID] = def
	return nil
}
func (r *fakeDefRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.WorkflowDefinition, error) {
	if d, ok := r.byID[id]; ok {
		return d, nil
	}
	return nil, errors.New("not found")
}
func (r *fakeDefRepo) ListByProject(ctx context.Context, projectID uuid.UUID) ([]*model.WorkflowDefinition, error) {
	return nil, nil
}
func (r *fakeDefRepo) Update(ctx context.Context, id uuid.UUID, def *model.WorkflowDefinition) error {
	return nil
}
func (r *fakeDefRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return nil
}

// ── Tests ───────────────────────────────────────────────────────────────

func TestDAGWorkflowService_ResumeParent_OnDAGChildCompleted(t *testing.T) {
	svc, execRepo, nodeRepo, linkRepo := newResumeTestService(t)

	parentExecID := uuid.New()
	parentProject := uuid.New()
	parentWfID := uuid.New()
	childExecID := uuid.New()
	parentExec := &model.WorkflowExecution{
		ID:         parentExecID,
		WorkflowID: parentWfID,
		ProjectID:  parentProject,
		Status:     model.WorkflowExecStatusPaused,
		DataStore:  json.RawMessage(`{}`),
		Context:    json.RawMessage(`{}`),
	}
	execRepo.byID[parentExecID] = parentExec
	parkedNode := &model.WorkflowNodeExecution{
		ID:          uuid.New(),
		ExecutionID: parentExecID,
		NodeID:      "sub-1",
		Status:      model.NodeExecAwaitingSubWorkflow,
	}
	nodeRepo.byExec[parentExecID] = []*model.WorkflowNodeExecution{parkedNode}
	linkRepo.links = append(linkRepo.links, &model.WorkflowRunParentLink{
		ID:                uuid.New(),
		ParentExecutionID: parentExecID,
		ParentNodeID:      "sub-1",
		ChildEngineKind:   model.SubWorkflowEngineDAG,
		ChildRunID:        childExecID,
		Status:            model.SubWorkflowLinkStatusRunning,
		StartedAt:         time.Now().UTC(),
	})
	// Parent def has only sub-1 (already parked) — AdvanceExecution will complete.
	parentDef := &model.WorkflowDefinition{
		ID:     parentWfID,
		Status: model.WorkflowDefStatusActive,
		Nodes:  json.RawMessage(`[{"id":"sub-1","type":"sub_workflow"}]`),
		Edges:  json.RawMessage(`[]`),
	}
	svc.defRepo.(*fakeDefRepo).byID[parentWfID] = parentDef

	childExec := &model.WorkflowExecution{
		ID:        childExecID,
		ProjectID: parentProject,
		DataStore: json.RawMessage(`{"last":"value"}`),
	}

	svc.tryResumeParentFromDAGChild(context.Background(), childExec, model.SubWorkflowLinkStatusCompleted)

	// Parent node should have been marked completed.
	if parkedNode.Status != model.NodeExecCompleted {
		t.Errorf("parent node status = %s, want completed", parkedNode.Status)
	}
	if len(linkRepo.statusUpdates) == 0 {
		t.Errorf("link status not updated")
	}
}

func TestDAGWorkflowService_ResumeParent_OnDAGChildFailed(t *testing.T) {
	svc, execRepo, nodeRepo, linkRepo := newResumeTestService(t)

	parentExecID := uuid.New()
	parentProject := uuid.New()
	parentWfID := uuid.New()
	childExecID := uuid.New()
	execRepo.byID[parentExecID] = &model.WorkflowExecution{
		ID:         parentExecID,
		WorkflowID: parentWfID,
		ProjectID:  parentProject,
		Status:     model.WorkflowExecStatusPaused,
		DataStore:  json.RawMessage(`{}`),
		Context:    json.RawMessage(`{}`),
	}
	parkedNode := &model.WorkflowNodeExecution{
		ID:          uuid.New(),
		ExecutionID: parentExecID,
		NodeID:      "sub-1",
		Status:      model.NodeExecAwaitingSubWorkflow,
	}
	nodeRepo.byExec[parentExecID] = []*model.WorkflowNodeExecution{parkedNode}
	linkRepo.links = append(linkRepo.links, &model.WorkflowRunParentLink{
		ID:                uuid.New(),
		ParentExecutionID: parentExecID,
		ParentNodeID:      "sub-1",
		ChildEngineKind:   model.SubWorkflowEngineDAG,
		ChildRunID:        childExecID,
		Status:            model.SubWorkflowLinkStatusRunning,
	})
	svc.defRepo.(*fakeDefRepo).byID[parentWfID] = &model.WorkflowDefinition{
		ID:     parentWfID,
		Status: model.WorkflowDefStatusActive,
		Nodes:  json.RawMessage(`[{"id":"sub-1","type":"sub_workflow"}]`),
		Edges:  json.RawMessage(`[]`),
	}

	childExec := &model.WorkflowExecution{ID: childExecID, ProjectID: parentProject}
	svc.tryResumeParentFromDAGChild(context.Background(), childExec, model.SubWorkflowLinkStatusFailed)

	if parkedNode.Status != model.NodeExecFailed {
		t.Errorf("parent node status = %s, want failed", parkedNode.Status)
	}
}

func TestDAGWorkflowService_ResumeParent_Idempotent(t *testing.T) {
	svc, execRepo, nodeRepo, linkRepo := newResumeTestService(t)

	parentExecID := uuid.New()
	parentProject := uuid.New()
	parentWfID := uuid.New()
	childExecID := uuid.New()
	execRepo.byID[parentExecID] = &model.WorkflowExecution{
		ID:         parentExecID,
		WorkflowID: parentWfID,
		ProjectID:  parentProject,
		Status:     model.WorkflowExecStatusRunning,
		DataStore:  json.RawMessage(`{}`),
		Context:    json.RawMessage(`{}`),
	}
	parkedNode := &model.WorkflowNodeExecution{
		ID:          uuid.New(),
		ExecutionID: parentExecID,
		NodeID:      "sub-1",
		Status:      model.NodeExecCompleted, // already resumed
	}
	nodeRepo.byExec[parentExecID] = []*model.WorkflowNodeExecution{parkedNode}
	// Link already marked completed.
	linkRepo.links = append(linkRepo.links, &model.WorkflowRunParentLink{
		ID:                uuid.New(),
		ParentExecutionID: parentExecID,
		ParentNodeID:      "sub-1",
		ChildEngineKind:   model.SubWorkflowEngineDAG,
		ChildRunID:        childExecID,
		Status:            model.SubWorkflowLinkStatusCompleted,
	})
	svc.defRepo.(*fakeDefRepo).byID[parentWfID] = &model.WorkflowDefinition{
		ID:     parentWfID,
		Status: model.WorkflowDefStatusActive,
		Nodes:  json.RawMessage(`[{"id":"sub-1","type":"sub_workflow"}]`),
	}

	childExec := &model.WorkflowExecution{ID: childExecID, ProjectID: parentProject}
	svc.tryResumeParentFromDAGChild(context.Background(), childExec, model.SubWorkflowLinkStatusCompleted)

	if len(nodeRepo.updates) != 0 {
		t.Errorf("node repo updated despite idempotent resume: %v", nodeRepo.updates)
	}
}

func TestDAGWorkflowService_ResumeParent_PluginChildOutputsMaterialized(t *testing.T) {
	svc, execRepo, nodeRepo, linkRepo := newResumeTestService(t)

	parentExecID := uuid.New()
	parentProject := uuid.New()
	parentWfID := uuid.New()
	pluginRunID := uuid.New()
	execRepo.byID[parentExecID] = &model.WorkflowExecution{
		ID:         parentExecID,
		WorkflowID: parentWfID,
		ProjectID:  parentProject,
		Status:     model.WorkflowExecStatusPaused,
		DataStore:  json.RawMessage(`{}`),
		Context:    json.RawMessage(`{}`),
	}
	parkedNode := &model.WorkflowNodeExecution{
		ID:          uuid.New(),
		ExecutionID: parentExecID,
		NodeID:      "sub-plugin",
		Status:      model.NodeExecAwaitingSubWorkflow,
	}
	nodeRepo.byExec[parentExecID] = []*model.WorkflowNodeExecution{parkedNode}
	linkRepo.links = append(linkRepo.links, &model.WorkflowRunParentLink{
		ID:                uuid.New(),
		ParentExecutionID: parentExecID,
		ParentNodeID:      "sub-plugin",
		ChildEngineKind:   model.SubWorkflowEnginePlugin,
		ChildRunID:        pluginRunID,
		Status:            model.SubWorkflowLinkStatusRunning,
	})
	svc.defRepo.(*fakeDefRepo).byID[parentWfID] = &model.WorkflowDefinition{
		ID:     parentWfID,
		Status: model.WorkflowDefStatusActive,
		Nodes:  json.RawMessage(`[{"id":"sub-plugin","type":"sub_workflow"}]`),
		Edges:  json.RawMessage(`[]`),
	}

	outputs := json.RawMessage(`{"lastStep":{"output":"hi"}}`)
	svc.ResumeParentFromPluginChild(context.Background(), pluginRunID, model.SubWorkflowLinkStatusCompleted, outputs)

	if parkedNode.Status != model.NodeExecCompleted {
		t.Errorf("plugin parent node status = %s, want completed", parkedNode.Status)
	}
}
