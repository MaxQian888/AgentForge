package eventbus

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventValidate_AcceptsWellFormed(t *testing.T) {
	e := &Event{
		ID:         "01HQZE5MR1T7YBX2A8V3JPZK7M",
		Type:       "task.created",
		Source:     "agent:run-1",
		Target:     "task:7b2e",
		Payload:    json.RawMessage(`{"id":"7b2e"}`),
		Metadata:   map[string]any{},
		Timestamp:  1700000000000,
		Visibility: VisibilityChannel,
	}
	assert.NoError(t, e.Validate())
}

func TestEventValidate_RejectsEmptyRequired(t *testing.T) {
	base := func() *Event {
		return &Event{
			ID: "01HQZE5MR1T7YBX2A8V3JPZK7M", Type: "task.created",
			Source: "agent:run-1", Target: "task:7b2e",
			Timestamp: 1, Visibility: VisibilityChannel,
		}
	}
	cases := []struct {
		name  string
		mut   func(e *Event)
		field string
	}{
		{"id", func(e *Event) { e.ID = "" }, "id"},
		{"type", func(e *Event) { e.Type = "" }, "type"},
		{"source", func(e *Event) { e.Source = "" }, "source"},
		{"target", func(e *Event) { e.Target = "" }, "target"},
		{"timestamp", func(e *Event) { e.Timestamp = 0 }, "timestamp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := base()
			c.mut(e)
			err := e.Validate()
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.field)
		})
	}
}

func TestEventValidate_TypePattern(t *testing.T) {
	e := &Event{
		ID: "x", Source: "a:b", Target: "c:d",
		Timestamp: 1, Visibility: VisibilityChannel,
	}
	for _, bad := range []string{"Task.Created", "task", "task.created.", ".task", "task created"} {
		e.Type = bad
		assert.Error(t, e.Validate(), "expected reject: %q", bad)
	}
	for _, good := range []string{"task.created", "workflow.execution.started", "a.b.c.d.e"} {
		e.Type = good
		assert.NoError(t, e.Validate(), "expected accept: %q", good)
	}
}

func TestNewEvent_SetsSaneDefaults(t *testing.T) {
	e := NewEvent("task.created", "core", "task:7b2e")
	assert.NotEmpty(t, e.ID)
	assert.Equal(t, VisibilityChannel, e.Visibility)
	assert.NotZero(t, e.Timestamp)
	assert.NotNil(t, e.Metadata)
}
