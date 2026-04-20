package mods

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	eb "github.com/agentforge/server/internal/eventbus"
	"github.com/stretchr/testify/assert"
)

type fakeIM struct {
	hits []string
	err  error
}

func (f *fakeIM) Dispatch(ctx context.Context, projectID, eventType string, payload json.RawMessage) error {
	f.hits = append(f.hits, projectID+"/"+eventType)
	return f.err
}

func TestIMForwardLegacy_ForwardsWithProject(t *testing.T) {
	im := &fakeIM{}
	m := NewIMForwardLegacy(im)

	send := func(typ string) {
		e := eb.NewEvent(typ, "core", "task:1")
		eb.SetString(e, eb.MetaProjectID, "p-1")
		m.Observe(context.Background(), e, &eb.PipelineCtx{})
	}
	send("task.created")
	send("review.completed")

	assert.Contains(t, im.hits, "p-1/task.created")
	assert.Contains(t, im.hits, "p-1/review.completed")
}

func TestIMForwardLegacy_NoProjectSkips(t *testing.T) {
	im := &fakeIM{}
	m := NewIMForwardLegacy(im)
	e := eb.NewEvent("task.created", "core", "task:1")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Empty(t, im.hits)
}

func TestIMForwardLegacy_NilRouterDoesNothing(t *testing.T) {
	m := NewIMForwardLegacy(nil)
	e := eb.NewEvent("task.created", "core", "task:1")
	eb.SetString(e, eb.MetaProjectID, "p-1")
	// Should not panic.
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
}

func TestIMForwardLegacy_DispatchErrorIsLoggedNotPropagated(t *testing.T) {
	im := &fakeIM{err: errors.New("bridge down")}
	m := NewIMForwardLegacy(im)
	e := eb.NewEvent("task.created", "core", "task:1")
	eb.SetString(e, eb.MetaProjectID, "p-1")
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Contains(t, im.hits, "p-1/task.created")
}

func TestIMForwardLegacy_InterceptsCoversLegacyTypes(t *testing.T) {
	m := NewIMForwardLegacy(nil)
	intercepts := m.Intercepts()
	assert.Contains(t, intercepts, "task.*")
	assert.Contains(t, intercepts, "review.*")
	assert.Contains(t, intercepts, "agent.*")
	assert.Contains(t, intercepts, "notification")
}

func TestIMForwardLegacy_MetadataIdentity(t *testing.T) {
	m := NewIMForwardLegacy(nil)
	assert.Equal(t, "im.forward-legacy", m.Name())
	assert.Equal(t, 80, m.Priority())
	assert.Equal(t, eb.ModeObserve, m.Mode())
}
