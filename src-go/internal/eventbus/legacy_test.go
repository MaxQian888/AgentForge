package eventbus

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type capturingPublisher struct {
	events []*Event
}

func (p *capturingPublisher) Publish(_ context.Context, e *Event) error {
	p.events = append(p.events, e)
	return nil
}

func TestPublishLegacy_ProjectScoped(t *testing.T) {
	p := &capturingPublisher{}
	err := PublishLegacy(context.Background(), p, "task.created", "p-1", map[string]any{"id": "t-1"})
	assert.NoError(t, err)
	assert.Len(t, p.events, 1)
	e := p.events[0]
	assert.Equal(t, "task.created", e.Type)
	assert.Equal(t, "project:p-1", e.Target)
	assert.Equal(t, "p-1", GetString(e, MetaProjectID))
	assert.Equal(t, []string{"project:p-1"}, GetChannels(e))
	assert.Contains(t, string(e.Payload), "t-1")
}

func TestPublishLegacy_NoProjectScoped(t *testing.T) {
	p := &capturingPublisher{}
	err := PublishLegacy(context.Background(), p, "system.notice", "", nil)
	assert.NoError(t, err)
	assert.Len(t, p.events, 1)
	e := p.events[0]
	assert.Equal(t, "system:broadcast", e.Target)
	assert.Equal(t, []string{"system:broadcast"}, GetChannels(e))
	assert.Empty(t, GetString(e, MetaProjectID))
}

func TestPublishLegacy_NilPublisherIsNoop(t *testing.T) {
	err := PublishLegacy(context.Background(), nil, "x.y", "p-1", nil)
	assert.NoError(t, err)
}

func TestPublishLegacy_UnmarshalableDropsPayload(t *testing.T) {
	p := &capturingPublisher{}
	ch := make(chan int)
	err := PublishLegacy(context.Background(), p, "task.created", "p-1", ch)
	assert.NoError(t, err)
	assert.Len(t, p.events, 1)
	assert.Empty(t, p.events[0].Payload)
}
