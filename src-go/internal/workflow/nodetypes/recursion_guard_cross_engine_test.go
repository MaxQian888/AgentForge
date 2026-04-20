package nodetypes

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// crossEngineLinkRepo is an in-memory SubWorkflowLinkRepo whose link rows
// carry ParentKind so the recursion guard can walk across engines. Indexed
// by (engineKind, childRunID) so GetByChild is O(1).
type crossEngineLinkRepo struct {
	byChild map[string]*SubWorkflowLinkRecord
}

func newCrossEngineLinkRepo(rows ...*SubWorkflowLinkRecord) *crossEngineLinkRepo {
	r := &crossEngineLinkRepo{byChild: map[string]*SubWorkflowLinkRecord{}}
	for _, row := range rows {
		r.byChild[row.ChildEngineKind+":"+row.ChildRunID.String()] = row
	}
	return r
}

func (r *crossEngineLinkRepo) Create(context.Context, *SubWorkflowLinkRecord) error { return nil }
func (r *crossEngineLinkRepo) GetByParent(context.Context, uuid.UUID, string) (*SubWorkflowLinkRecord, error) {
	return nil, errors.New("not implemented")
}
func (r *crossEngineLinkRepo) GetByChild(_ context.Context, kind string, id uuid.UUID) (*SubWorkflowLinkRecord, error) {
	if v, ok := r.byChild[kind+":"+id.String()]; ok {
		return v, nil
	}
	return nil, errors.New("not found")
}
func (r *crossEngineLinkRepo) ListByParentExecution(context.Context, uuid.UUID) ([]*SubWorkflowLinkRecord, error) {
	return nil, nil
}
func (r *crossEngineLinkRepo) UpdateStatus(context.Context, uuid.UUID, string) error { return nil }

// execLookupByMap maps a DAG exec id to its workflow id.
type execLookupByMap struct {
	m map[uuid.UUID]uuid.UUID
}

func (l *execLookupByMap) GetExecutionWorkflowID(_ context.Context, id uuid.UUID) (uuid.UUID, error) {
	if v, ok := l.m[id]; ok {
		return v, nil
	}
	return uuid.Nil, errors.New("exec not found")
}

// TestRecursionGuard_CrossEngine_DAGPluginDAGCycle: a DAG execution D1 whose
// sub_workflow node invokes plugin P, plugin P's `workflow` step invokes DAG
// D2, and D2 invokes D1 again — CheckFromEngine at the D2→D1 hop must
// detect the cycle while walking across D1 → P (plugin) → D2.
func TestRecursionGuard_CrossEngine_DAGPluginDAGCycle(t *testing.T) {
	ctx := context.Background()

	dagWorkflowID := uuid.New() // target DAG that forms the cycle
	d1ExecID := uuid.New()      // original DAG execution (workflow = dagWorkflowID)
	pluginRunID := uuid.New()   // plugin run invoked as D1's child
	d2ExecID := uuid.New()      // proposed new DAG child of the plugin

	// Linkage:
	// D1 (parent_kind=dag_execution) invokes plugin P  →  link(child=plugin:P, parent=D1)
	// Plugin P (parent_kind=plugin_run)  invokes D2   →  link(child=dag:D2, parent=P, parent_kind=plugin_run)
	repo := newCrossEngineLinkRepo(
		&SubWorkflowLinkRecord{
			ParentExecutionID: d1ExecID,
			ParentKind:        "dag_execution",
			ChildEngineKind:   string(SubWorkflowTargetPlugin),
			ChildRunID:        pluginRunID,
		},
		&SubWorkflowLinkRecord{
			ParentExecutionID: pluginRunID,
			ParentKind:        "plugin_run",
			ChildEngineKind:   string(SubWorkflowTargetDAG),
			ChildRunID:        d2ExecID,
		},
	)

	lookup := &execLookupByMap{
		m: map[uuid.UUID]uuid.UUID{
			d1ExecID: dagWorkflowID,
			d2ExecID: uuid.New(),
		},
	}
	guard := NewRecursionGuard(repo, lookup, MaxSubWorkflowDepth)

	// D2 would invoke dagWorkflowID — must be rejected because D1 ran
	// dagWorkflowID as an ancestor.
	err := guard.CheckFromEngine(ctx, string(SubWorkflowTargetDAG), d2ExecID, dagWorkflowID.String())
	if err == nil {
		t.Fatalf("expected cross-engine cycle to be detected")
	}
	se, ok := err.(*SubWorkflowInvocationError)
	if !ok || se.Reason != SubWorkflowRejectCycle {
		t.Errorf("expected cycle rejection, got %v", err)
	}
}

// TestRecursionGuard_CrossEngine_PluginDAGPluginCycle: a plugin Pa invokes
// DAG D, which invokes plugin Pb, which would now invoke DAG D' (the new
// proposed target). The ancestor chain Pa → D → Pb has a DAG exec D in it;
// the new target D' forming the cycle is verified against the DAG workflow.
// Here we verify that starting a walk from the plugin side correctly
// locates a DAG exec ancestor and catches a cycle on that DAG's workflow.
func TestRecursionGuard_CrossEngine_PluginDAGPluginCycle(t *testing.T) {
	ctx := context.Background()

	dagWorkflowID := uuid.New()
	dagExecID := uuid.New()
	pbRunID := uuid.New() // plugin Pb — ancestor chain leads to this

	repo := newCrossEngineLinkRepo(
		// DAG D invoked plugin Pb — link at child=plugin:Pb, parent=DAG exec
		&SubWorkflowLinkRecord{
			ParentExecutionID: dagExecID,
			ParentKind:        "dag_execution",
			ChildEngineKind:   string(SubWorkflowTargetPlugin),
			ChildRunID:        pbRunID,
		},
	)
	lookup := &execLookupByMap{m: map[uuid.UUID]uuid.UUID{dagExecID: dagWorkflowID}}
	guard := NewRecursionGuard(repo, lookup, MaxSubWorkflowDepth)

	// Pb now tries to invoke dagWorkflowID — must be rejected as cycle.
	err := guard.CheckFromEngine(ctx, string(SubWorkflowTargetPlugin), pbRunID, dagWorkflowID.String())
	if err == nil {
		t.Fatalf("expected cross-engine cycle from plugin start to be detected")
	}
	se, ok := err.(*SubWorkflowInvocationError)
	if !ok || se.Reason != SubWorkflowRejectCycle {
		t.Errorf("expected cycle rejection, got %v", err)
	}
}

// TestRecursionGuard_CrossEngine_BenignChainPasses: a plugin Pa invoked by a
// DAG D, where D's workflow is different from the newly proposed DAG target.
// Walking the chain should succeed without a cycle.
func TestRecursionGuard_CrossEngine_BenignChainPasses(t *testing.T) {
	ctx := context.Background()

	benignTarget := uuid.New() // DAG target being invoked
	dagExecID := uuid.New()
	paRunID := uuid.New()

	repo := newCrossEngineLinkRepo(
		&SubWorkflowLinkRecord{
			ParentExecutionID: dagExecID,
			ParentKind:        "dag_execution",
			ChildEngineKind:   string(SubWorkflowTargetPlugin),
			ChildRunID:        paRunID,
		},
	)
	// DAG exec's workflow id is NOT benignTarget — no cycle.
	lookup := &execLookupByMap{m: map[uuid.UUID]uuid.UUID{dagExecID: uuid.New()}}
	guard := NewRecursionGuard(repo, lookup, MaxSubWorkflowDepth)

	if err := guard.CheckFromEngine(ctx, string(SubWorkflowTargetPlugin), paRunID, benignTarget.String()); err != nil {
		t.Errorf("expected benign cross-engine chain to pass; got %v", err)
	}
}
