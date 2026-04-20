package mods

import (
	"context"
	"testing"

	eb "github.com/agentforge/server/internal/eventbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate_Name(t *testing.T) {
	v := NewValidate()
	assert.Equal(t, "core.validate", v.Name())
}

func TestValidate_Intercepts(t *testing.T) {
	v := NewValidate()
	intercepts := v.Intercepts()
	assert.Len(t, intercepts, 1)
	assert.Equal(t, "*", intercepts[0])
}

func TestValidate_Priority(t *testing.T) {
	v := NewValidate()
	assert.Equal(t, 10, v.Priority())
}

func TestValidate_Mode(t *testing.T) {
	v := NewValidate()
	assert.Equal(t, eb.ModeGuard, v.Mode())
}

func TestValidate_Guard_ValidEvent(t *testing.T) {
	v := NewValidate()
	e := eb.NewEvent("user.created", "user:alice", "team:acme")
	e.Metadata[eb.MetaChannels] = []string{"general", "engineering"}

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := v.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestValidate_Guard_InvalidSourceAddress(t *testing.T) {
	v := NewValidate()
	e := eb.NewEvent("user.created", "invalid", "team:acme")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := v.Guard(ctx, e, pc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid source")
}

func TestValidate_Guard_InvalidTargetAddress(t *testing.T) {
	v := NewValidate()
	e := eb.NewEvent("user.created", "user:alice", "bad_target")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := v.Guard(ctx, e, pc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid target")
}

func TestValidate_Guard_ChannelsAsStringSlice(t *testing.T) {
	v := NewValidate()
	e := eb.NewEvent("user.created", "user:alice", "team:acme")
	e.Metadata[eb.MetaChannels] = []string{"general", "engineering", "random"}

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := v.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestValidate_Guard_ChannelsAsAnySlice(t *testing.T) {
	v := NewValidate()
	e := eb.NewEvent("user.created", "user:alice", "team:acme")
	e.Metadata[eb.MetaChannels] = []any{"general", "engineering"}

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := v.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestValidate_Guard_ChannelsNil(t *testing.T) {
	v := NewValidate()
	e := eb.NewEvent("user.created", "user:alice", "team:acme")
	e.Metadata[eb.MetaChannels] = nil

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := v.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestValidate_Guard_ChannelsNotPresent(t *testing.T) {
	v := NewValidate()
	e := eb.NewEvent("user.created", "user:alice", "team:acme")
	// Don't set channels at all

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := v.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestValidate_Guard_ChannelsMalformed(t *testing.T) {
	v := NewValidate()
	e := eb.NewEvent("user.created", "user:alice", "team:acme")
	e.Metadata[eb.MetaChannels] = "not a slice"

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := v.Guard(ctx, e, pc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.channels must be []string")
}

func TestValidate_Guard_AllValidAddressSchemes(t *testing.T) {
	v := NewValidate()
	cases := []struct {
		source string
		target string
	}{
		{"agent:run-123", "project:acme"},
		{"plugin:review-linter", "team:eng"},
		{"user:bob", "workflow:wf-1"},
		{"role:planner", "skill:sql"},
		{"task:task-1", "agent:run-456"},
		{"channel:project:acme", "user:alice"},
		{"core", "project:acme"},
	}
	for _, c := range cases {
		t.Run(c.source+"->"+c.target, func(t *testing.T) {
			e := eb.NewEvent("test.event", c.source, c.target)
			ctx := context.Background()
			pc := &eb.PipelineCtx{}
			err := v.Guard(ctx, e, pc)
			require.NoError(t, err)
		})
	}
}
