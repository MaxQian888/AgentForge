package mods

import (
	"context"
	"testing"

	eb "github.com/agentforge/server/internal/eventbus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuth_Name(t *testing.T) {
	a := NewAuth()
	assert.Equal(t, "core.auth", a.Name())
}

func TestAuth_Intercepts(t *testing.T) {
	a := NewAuth()
	intercepts := a.Intercepts()
	assert.Len(t, intercepts, 1)
	assert.Equal(t, "*", intercepts[0])
}

func TestAuth_Priority(t *testing.T) {
	a := NewAuth()
	assert.Equal(t, 20, a.Priority())
}

func TestAuth_Mode(t *testing.T) {
	a := NewAuth()
	assert.Equal(t, eb.ModeGuard, a.Mode())
}

func TestAuth_Guard_CoreSourceAllowed(t *testing.T) {
	a := NewAuth()
	e := eb.NewEvent("system.init", "core", "project:acme")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := a.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestAuth_Guard_PluginSourceAllowed(t *testing.T) {
	a := NewAuth()
	e := eb.NewEvent("plugin.executed", "plugin:review-linter", "project:acme")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := a.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestAuth_Guard_AgentSourceAllowed(t *testing.T) {
	a := NewAuth()
	e := eb.NewEvent("agent.started", "agent:run-123", "project:acme")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := a.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestAuth_Guard_UserSourceWithoutContextAllowed(t *testing.T) {
	a := NewAuth()
	e := eb.NewEvent("user.created", "user:alice", "team:acme")
	// No user_id set in metadata

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := a.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestAuth_Guard_UserSourceMatchingContextAllowed(t *testing.T) {
	a := NewAuth()
	e := eb.NewEvent("user.created", "user:alice", "team:acme")
	eb.SetString(e, eb.MetaUserID, "alice")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := a.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestAuth_Guard_UserSourceSpoofRejected(t *testing.T) {
	a := NewAuth()
	e := eb.NewEvent("user.action", "user:bob", "team:acme")
	eb.SetString(e, eb.MetaUserID, "alice")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := a.Guard(ctx, e, pc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "source user \"bob\" does not match context user \"alice\"")
}

func TestAuth_Guard_OtherSourceAllowed(t *testing.T) {
	a := NewAuth()
	cases := []string{
		"role:planner",
		"task:task-1",
		"team:eng",
		"workflow:wf-1",
		"project:acme",
		"channel:project:acme",
		"skill:sql",
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			e := eb.NewEvent("test.event", src, "project:acme")
			ctx := context.Background()
			pc := &eb.PipelineCtx{}
			err := a.Guard(ctx, e, pc)
			require.NoError(t, err)
		})
	}
}

func TestAuth_Guard_UserSourceEmptyContextAllowed(t *testing.T) {
	a := NewAuth()
	e := eb.NewEvent("user.action", "user:charlie", "team:acme")
	eb.SetString(e, eb.MetaUserID, "")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := a.Guard(ctx, e, pc)
	require.NoError(t, err)
}

func TestAuth_Guard_UserSourceWithDifferentContextRejected(t *testing.T) {
	a := NewAuth()
	e := eb.NewEvent("user.update", "user:dave", "project:acme")
	eb.SetString(e, eb.MetaUserID, "eve")

	ctx := context.Background()
	pc := &eb.PipelineCtx{}
	err := a.Guard(ctx, e, pc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}
