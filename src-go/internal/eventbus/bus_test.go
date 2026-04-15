// src-go/internal/eventbus/bus_test.go
package eventbus

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBus_Publish_InvokesPipeline(t *testing.T) {
	o := &fakeObserve{name: "o", prio: 1, inter: []string{"*"}}
	bus := NewBus()
	bus.Register(o)
	require.NoError(t, bus.Publish(context.Background(), NewEvent("task.created", "c", "task:1")))
	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), atomic.LoadInt32(&o.calls))
}

func TestBus_RegisterPanicsAfterStart(t *testing.T) {
	bus := NewBus()
	bus.Register(&fakeObserve{name: "a", prio: 1, inter: []string{"*"}})
	require.NoError(t, bus.Publish(context.Background(), NewEvent("x.y", "c", "t:1")))
	assert.Panics(t, func() {
		bus.Register(&fakeObserve{name: "late", prio: 1, inter: []string{"*"}})
	})
}

func TestBus_EmitsFollowThroughPipeline(t *testing.T) {
	emitter := &emittingTransform{inter: []string{"task.created"}, childType: "audit.trail"}
	collector := &fakeObserve{name: "obs", prio: 1, inter: []string{"*"}}
	bus := NewBus()
	bus.Register(emitter)
	bus.Register(collector)

	require.NoError(t, bus.Publish(context.Background(), NewEvent("task.created", "c", "task:1")))
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, int32(2), atomic.LoadInt32(&collector.calls), "both original and emitted must observe")
}

func TestBus_DepthLimit(t *testing.T) {
	emitter := &emittingTransform{inter: []string{"*"}, childType: "x.y"}
	bus := NewBus()
	bus.Register(emitter)
	err := bus.Publish(context.Background(), NewEvent("x.y", "c", "t:1"))
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
}

// helper
type emittingTransform struct {
	inter     []string
	childType string
}

func (e *emittingTransform) Name() string         { return "emitter" }
func (e *emittingTransform) Intercepts() []string { return e.inter }
func (e *emittingTransform) Priority() int        { return 1 }
func (e *emittingTransform) Mode() Mode           { return ModeTransform }
func (e *emittingTransform) Transform(ctx context.Context, ev *Event, pc *PipelineCtx) (*Event, error) {
	pc.Emit(*NewEvent(e.childType, "core", "task:1"))
	return ev, nil
}
