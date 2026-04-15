package mods

import (
	"context"
	"encoding/json"
	"testing"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	"github.com/stretchr/testify/assert"
)

type fakeHub struct {
	fanout    [][]byte
	fanoutChs [][]string
	bcast     [][]byte
}

func (h *fakeHub) FanoutBytes(data []byte, channels []string) {
	h.fanout = append(h.fanout, append([]byte(nil), data...))
	h.fanoutChs = append(h.fanoutChs, append([]string(nil), channels...))
}

func (h *fakeHub) BroadcastAllBytes(data []byte) {
	h.bcast = append(h.bcast, append([]byte(nil), data...))
}

func TestWSFanout_ChannelVisibilityUsesChannels(t *testing.T) {
	h := &fakeHub{}
	m := NewWSFanout(h)
	e := eb.NewEvent("task.created", "core", "task:1")
	eb.SetChannels(e, []string{"channel:task:1"})
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Len(t, h.fanout, 1)
	assert.Len(t, h.bcast, 0)
	assert.Equal(t, []string{"channel:task:1"}, h.fanoutChs[0])

	var frame struct {
		Channel string          `json:"channel"`
		Event   json.RawMessage `json:"event"`
	}
	assert.NoError(t, json.Unmarshal(h.fanout[0], &frame))
	assert.Equal(t, "channel:task:1", frame.Channel)
	assert.NotEmpty(t, frame.Event)
}

func TestWSFanout_MultipleChannelsEmitMultipleFrames(t *testing.T) {
	h := &fakeHub{}
	m := NewWSFanout(h)
	e := eb.NewEvent("task.created", "core", "task:1")
	eb.SetChannels(e, []string{"channel:task:1", "channel:project:p-1"})
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Len(t, h.fanout, 2)
	assert.Len(t, h.bcast, 0)
	assert.Equal(t, []string{"channel:task:1"}, h.fanoutChs[0])
	assert.Equal(t, []string{"channel:project:p-1"}, h.fanoutChs[1])
}

func TestWSFanout_PublicUsesBroadcastAll(t *testing.T) {
	h := &fakeHub{}
	m := NewWSFanout(h)
	e := eb.NewEvent("system.notice", "core", "project:p")
	e.Visibility = eb.VisibilityPublic
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Len(t, h.bcast, 1)
	assert.Len(t, h.fanout, 0)
}

func TestWSFanout_ModOnlyDoesNothing(t *testing.T) {
	h := &fakeHub{}
	m := NewWSFanout(h)
	e := eb.NewEvent("x.y", "core", "task:1")
	e.Visibility = eb.VisibilityModOnly
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Empty(t, h.fanout)
	assert.Empty(t, h.bcast)
}

func TestWSFanout_DirectDoesNothing(t *testing.T) {
	h := &fakeHub{}
	m := NewWSFanout(h)
	e := eb.NewEvent("x.y", "core", "user:u1")
	e.Visibility = eb.VisibilityDirect
	m.Observe(context.Background(), e, &eb.PipelineCtx{})
	assert.Empty(t, h.fanout)
	assert.Empty(t, h.bcast)
}
