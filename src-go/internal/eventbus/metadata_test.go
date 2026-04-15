package eventbus

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSetChannels(t *testing.T) {
	e := NewEvent("task.created", "core", "task:1")
	assert.Empty(t, GetChannels(e))

	SetChannels(e, []string{"channel:project:demo", "channel:task:1"})
	got := GetChannels(e)
	assert.Equal(t, []string{"channel:project:demo", "channel:task:1"}, got)
}

func TestGetChannels_TypeMismatch(t *testing.T) {
	e := NewEvent("task.created", "core", "task:1")
	e.Metadata["channels"] = "not-a-slice"
	got := GetChannels(e)
	assert.Empty(t, got, "malformed channels must degrade to empty, not panic")
}

func TestGetStringMetadata(t *testing.T) {
	e := NewEvent("x.y", "core", "task:1")
	e.Metadata["user_id"] = "u-1"
	assert.Equal(t, "u-1", GetString(e, "user_id"))
	assert.Equal(t, "", GetString(e, "nope"))
}

func TestCausationDepth(t *testing.T) {
	e := NewEvent("x.y", "core", "task:1")
	assert.Equal(t, 0, GetCausationDepth(e))
	IncrementCausationDepth(e)
	require.Equal(t, 1, GetCausationDepth(e))
}
