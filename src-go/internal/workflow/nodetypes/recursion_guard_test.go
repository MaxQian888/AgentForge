package nodetypes

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// fakeLinkRepo satisfies SubWorkflowLinkRepo for guard tests. Only GetByChild
// and ListByParentExecution are consulted by the guard.
type fakeLinkRepo struct {
	// byChild maps childRunID → link record (parent exec walk).
	byChild map[uuid.UUID]*SubWorkflowLinkRecord
}

func (f *fakeLinkRepo) Create(ctx context.Context, link *SubWorkflowLinkRecord) error {
	return nil
}
func (f *fakeLinkRepo) GetByParent(ctx context.Context, parentExecutionID uuid.UUID, parentNodeID string) (*SubWorkflowLinkRecord, error) {
	return nil, errors.New("not found")
}
func (f *fakeLinkRepo) GetByChild(ctx context.Context, engineKind string, childRunID uuid.UUID) (*SubWorkflowLinkRecord, error) {
	if link, ok := f.byChild[childRunID]; ok {
		return link, nil
	}
	return nil, errors.New("not found")
}
func (f *fakeLinkRepo) ListByParentExecution(ctx context.Context, parentExecutionID uuid.UUID) ([]*SubWorkflowLinkRecord, error) {
	return nil, nil
}
func (f *fakeLinkRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return nil
}

// fakeExecLookup returns predictable workflow ids per exec id.
type fakeExecLookup struct {
	byExec map[uuid.UUID]uuid.UUID
}

func (f *fakeExecLookup) GetExecutionWorkflowID(ctx context.Context, executionID uuid.UUID) (uuid.UUID, error) {
	if wf, ok := f.byExec[executionID]; ok {
		return wf, nil
	}
	return uuid.Nil, errors.New("exec not found")
}

func TestRecursionGuard_DirectSelfRecursionRejected(t *testing.T) {
	execA := uuid.New()
	wfA := uuid.New()
	links := &fakeLinkRepo{byChild: map[uuid.UUID]*SubWorkflowLinkRecord{}}
	lookup := &fakeExecLookup{byExec: map[uuid.UUID]uuid.UUID{execA: wfA}}
	g := NewRecursionGuard(links, lookup, MaxSubWorkflowDepth)

	err := g.Check(context.Background(), execA, wfA.String())
	if err == nil {
		t.Fatal("Check() returned nil error, want cycle rejection")
	}
	var invErr *SubWorkflowInvocationError
	if !errors.As(err, &invErr) {
		t.Fatalf("Check() err type = %T, want *SubWorkflowInvocationError", err)
	}
	if invErr.Reason != SubWorkflowRejectCycle {
		t.Errorf("reason = %s, want %s", invErr.Reason, SubWorkflowRejectCycle)
	}
}

func TestRecursionGuard_ThreeCycleRejected(t *testing.T) {
	// A → B → C → A: C's exec is about to invoke A.
	execA, execB, execC := uuid.New(), uuid.New(), uuid.New()
	wfA, wfB, wfC := uuid.New(), uuid.New(), uuid.New()
	links := &fakeLinkRepo{
		byChild: map[uuid.UUID]*SubWorkflowLinkRecord{
			execC: {ParentExecutionID: execB, ChildRunID: execC},
			execB: {ParentExecutionID: execA, ChildRunID: execB},
		},
	}
	lookup := &fakeExecLookup{
		byExec: map[uuid.UUID]uuid.UUID{
			execA: wfA, execB: wfB, execC: wfC,
		},
	}
	g := NewRecursionGuard(links, lookup, MaxSubWorkflowDepth)

	err := g.Check(context.Background(), execC, wfA.String())
	if err == nil {
		t.Fatal("Check() returned nil error, want transitive-cycle rejection")
	}
	var invErr *SubWorkflowInvocationError
	if !errors.As(err, &invErr) || invErr.Reason != SubWorkflowRejectCycle {
		t.Fatalf("Check() err = %v, want cycle rejection", err)
	}
}

func TestRecursionGuard_SevenDeepChainPasses(t *testing.T) {
	// 7-deep chain: every ancestor has a distinct workflow id.
	execs := make([]uuid.UUID, 7)
	wfs := make([]uuid.UUID, 7)
	for i := range execs {
		execs[i] = uuid.New()
		wfs[i] = uuid.New()
	}
	links := &fakeLinkRepo{byChild: map[uuid.UUID]*SubWorkflowLinkRecord{}}
	for i := 1; i < 7; i++ {
		links.byChild[execs[i]] = &SubWorkflowLinkRecord{ParentExecutionID: execs[i-1], ChildRunID: execs[i]}
	}
	lookup := &fakeExecLookup{byExec: map[uuid.UUID]uuid.UUID{}}
	for i := range execs {
		lookup.byExec[execs[i]] = wfs[i]
	}
	g := NewRecursionGuard(links, lookup, MaxSubWorkflowDepth)

	// Invoke a brand-new target from the leaf.
	err := g.Check(context.Background(), execs[6], uuid.New().String())
	if err != nil {
		t.Errorf("Check() returned %v, want nil for 7-deep non-cyclic chain", err)
	}
}

func TestRecursionGuard_NineDeepChainRejected(t *testing.T) {
	// 9-deep chain exceeds the max depth of 8.
	execs := make([]uuid.UUID, 9)
	wfs := make([]uuid.UUID, 9)
	for i := range execs {
		execs[i] = uuid.New()
		wfs[i] = uuid.New()
	}
	links := &fakeLinkRepo{byChild: map[uuid.UUID]*SubWorkflowLinkRecord{}}
	for i := 1; i < 9; i++ {
		links.byChild[execs[i]] = &SubWorkflowLinkRecord{ParentExecutionID: execs[i-1], ChildRunID: execs[i]}
	}
	lookup := &fakeExecLookup{byExec: map[uuid.UUID]uuid.UUID{}}
	for i := range execs {
		lookup.byExec[execs[i]] = wfs[i]
	}
	g := NewRecursionGuard(links, lookup, MaxSubWorkflowDepth)

	// Target is a brand-new workflow id — cycle is not the rejection reason,
	// depth is. Check from the deepest node (execs[8]).
	err := g.Check(context.Background(), execs[8], uuid.New().String())
	if err == nil {
		t.Fatal("Check() returned nil, want depth-limit rejection")
	}
	var invErr *SubWorkflowInvocationError
	if !errors.As(err, &invErr) || invErr.Reason != SubWorkflowRejectDepthExceeded {
		t.Errorf("Check() reason = %v, want depth_exceeded", err)
	}
}

func TestRecursionGuard_NilGuardNoop(t *testing.T) {
	var g *RecursionGuard
	if err := g.Check(context.Background(), uuid.New(), uuid.New().String()); err != nil {
		t.Errorf("nil guard Check() = %v, want nil", err)
	}
	g = &RecursionGuard{} // no deps wired
	if err := g.Check(context.Background(), uuid.New(), uuid.New().String()); err != nil {
		t.Errorf("empty guard Check() = %v, want nil", err)
	}
}
