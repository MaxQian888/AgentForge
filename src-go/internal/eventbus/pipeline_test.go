// src-go/internal/eventbus/pipeline_test.go
package eventbus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers ---------------------------------------------------

type fakeGuard struct {
	name string; prio int; inter []string
	err  error
	calls int32
}
func (f *fakeGuard) Name() string         { return f.name }
func (f *fakeGuard) Intercepts() []string { return f.inter }
func (f *fakeGuard) Priority() int        { return f.prio }
func (f *fakeGuard) Mode() Mode           { return ModeGuard }
func (f *fakeGuard) Guard(ctx context.Context, e *Event, pc *PipelineCtx) error {
	atomic.AddInt32(&f.calls, 1)
	return f.err
}

type fakeTransform struct {
	name string; prio int; inter []string
	mutate func(*Event)
	err    error
}
func (f *fakeTransform) Name() string         { return f.name }
func (f *fakeTransform) Intercepts() []string { return f.inter }
func (f *fakeTransform) Priority() int        { return f.prio }
func (f *fakeTransform) Mode() Mode           { return ModeTransform }
func (f *fakeTransform) Transform(ctx context.Context, e *Event, pc *PipelineCtx) (*Event, error) {
	if f.err != nil { return nil, f.err }
	if f.mutate != nil { f.mutate(e) }
	return e, nil
}

type fakeObserve struct {
	name string; prio int; inter []string
	calls int32
	doPanic bool
}
func (f *fakeObserve) Name() string         { return f.name }
func (f *fakeObserve) Intercepts() []string { return f.inter }
func (f *fakeObserve) Priority() int        { return f.prio }
func (f *fakeObserve) Mode() Mode           { return ModeObserve }
func (f *fakeObserve) Observe(ctx context.Context, e *Event, pc *PipelineCtx) {
	atomic.AddInt32(&f.calls, 1)
	if f.doPanic { panic("kaboom") }
}

// Tests -----------------------------------------------------------

func TestPipeline_Ordering(t *testing.T) {
	g1 := &fakeGuard{name: "g1", prio: 10, inter: []string{"*"}}
	t1 := &fakeTransform{name: "t1", prio: 10, inter: []string{"*"}, mutate: func(e *Event) { SetString(e, "t1", "ok") }}
	o1 := &fakeObserve{name: "o1", prio: 10, inter: []string{"*"}}

	p := NewPipeline([]Mod{o1, t1, g1})
	e := NewEvent("task.created", "core", "task:1")
	out, err := p.Process(context.Background(), e, &PipelineCtx{})
	require.NoError(t, err)
	assert.Equal(t, "ok", GetString(out, "t1"))
	time.Sleep(50 * time.Millisecond) // allow parallel observe to finish
	assert.Equal(t, int32(1), atomic.LoadInt32(&g1.calls))
	assert.Equal(t, int32(1), atomic.LoadInt32(&o1.calls))
}

func TestPipeline_GuardRejects(t *testing.T) {
	g := &fakeGuard{name: "g", prio: 1, inter: []string{"*"}, err: errors.New("nope")}
	o := &fakeObserve{name: "o", prio: 1, inter: []string{"*"}}
	p := NewPipeline([]Mod{g, o})
	e := NewEvent("task.created", "core", "task:1")
	_, err := p.Process(context.Background(), e, &PipelineCtx{})
	require.Error(t, err)
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(0), atomic.LoadInt32(&o.calls), "observer must not run after guard rejects")
}

func TestPipeline_InterceptPattern(t *testing.T) {
	g := &fakeGuard{name: "g", prio: 1, inter: []string{"workflow.*"}}
	p := NewPipeline([]Mod{g})
	_, err := p.Process(context.Background(), NewEvent("task.created", "c", "t:1"), &PipelineCtx{})
	require.NoError(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&g.calls))

	_, err = p.Process(context.Background(), NewEvent("workflow.execution.started", "c", "t:1"), &PipelineCtx{})
	require.NoError(t, err)
	assert.Equal(t, int32(1), atomic.LoadInt32(&g.calls))
}

func TestPipeline_ObservePanicIsolated(t *testing.T) {
	o1 := &fakeObserve{name: "o1", prio: 1, inter: []string{"*"}, doPanic: true}
	o2 := &fakeObserve{name: "o2", prio: 2, inter: []string{"*"}}
	p := NewPipeline([]Mod{o1, o2})
	_, err := p.Process(context.Background(), NewEvent("x.y", "c", "t:1"), &PipelineCtx{})
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&o2.calls))
}

func TestPipeline_PriorityWithinMode(t *testing.T) {
	seen := []string{}
	t1 := &fakeTransform{name: "tA", prio: 20, inter: []string{"*"}, mutate: func(e *Event) { seen = append(seen, "tA") }}
	t2 := &fakeTransform{name: "tB", prio: 10, inter: []string{"*"}, mutate: func(e *Event) { seen = append(seen, "tB") }}
	p := NewPipeline([]Mod{t1, t2})
	_, err := p.Process(context.Background(), NewEvent("x.y", "c", "t:1"), &PipelineCtx{})
	require.NoError(t, err)
	assert.Equal(t, []string{"tB", "tA"}, seen)
}
