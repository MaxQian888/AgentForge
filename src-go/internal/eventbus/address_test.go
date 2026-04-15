package eventbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAddress_Canonical(t *testing.T) {
	cases := []struct {
		raw    string
		scheme string
		name   string
	}{
		{"agent:run-abc", "agent", "run-abc"},
		{"role:planner", "role", "planner"},
		{"task:7b2e9a", "task", "7b2e9a"},
		{"project:demo", "project", "demo"},
		{"team:alpha", "team", "alpha"},
		{"workflow:wf-42", "workflow", "wf-42"},
		{"plugin:review-linter", "plugin", "review-linter"},
		{"user:max", "user", "max"},
		{"core", "core", ""},
		{"channel:project:demo", "channel", "project:demo"},
		{"channel:task:7b2e", "channel", "task:7b2e"},
	}
	for _, c := range cases {
		t.Run(c.raw, func(t *testing.T) {
			a, err := ParseAddress(c.raw)
			require.NoError(t, err)
			assert.Equal(t, c.scheme, a.Scheme)
			assert.Equal(t, c.name, a.Name)
			assert.Equal(t, c.raw, a.Raw)
		})
	}
}

func TestParseAddress_Rejects(t *testing.T) {
	bad := []string{"", "badscheme:x", "agent:", ":empty"}
	for _, s := range bad {
		_, err := ParseAddress(s)
		assert.Error(t, err, "expected reject: %q", s)
	}
}

func TestAddress_ChannelScope(t *testing.T) {
	a, _ := ParseAddress("channel:task:7b2e")
	inner, ok := a.ChannelScope()
	require.True(t, ok)
	assert.Equal(t, "task", inner.Scheme)
	assert.Equal(t, "7b2e", inner.Name)
}

func TestMakeChannel(t *testing.T) {
	assert.Equal(t, "channel:project:demo", MakeChannel("project", "demo"))
	assert.Equal(t, "channel:task:abc", MakeChannel("task", "abc"))
}
