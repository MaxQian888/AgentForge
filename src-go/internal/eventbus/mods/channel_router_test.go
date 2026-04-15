package mods

import (
	"context"
	"testing"
	eb "github.com/react-go-quick-starter/server/internal/eventbus"
)

func TestChannelRouter_TaskTarget_AddsChannels(t *testing.T) {
	router := NewChannelRouter()
	ev := eb.NewEvent("core.test", "test:source", "task:task-123")
	eb.SetString(ev, eb.MetaProjectID, "proj-456")
	pc := &eb.PipelineCtx{}

	result, err := router.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	channels := eb.GetChannels(result)
	expected := map[string]bool{
		"channel:task:task-123":    true,
		"channel:project:proj-456": true,
	}

	if len(channels) != len(expected) {
		t.Fatalf("Expected %d channels, got %d: %v", len(expected), len(channels), channels)
	}

	for _, ch := range channels {
		if !expected[ch] {
			t.Errorf("Unexpected channel: %q", ch)
		}
	}
}

func TestChannelRouter_AgentTarget_AddsChannels(t *testing.T) {
	router := NewChannelRouter()
	ev := eb.NewEvent("core.test", "test:source", "agent:agent-789")
	eb.SetString(ev, eb.MetaProjectID, "proj-abc")
	pc := &eb.PipelineCtx{}

	result, err := router.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	channels := eb.GetChannels(result)
	expected := map[string]bool{
		"channel:agent:agent-789":  true,
		"channel:project:proj-abc": true,
	}

	if len(channels) != len(expected) {
		t.Fatalf("Expected %d channels, got %d: %v", len(expected), len(channels), channels)
	}

	for _, ch := range channels {
		if !expected[ch] {
			t.Errorf("Unexpected channel: %q", ch)
		}
	}
}

func TestChannelRouter_NoProject_OnlyTargetChannel(t *testing.T) {
	router := NewChannelRouter()
	ev := eb.NewEvent("core.test", "test:source", "task:task-xyz")
	// No project ID set
	pc := &eb.PipelineCtx{}

	result, err := router.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	channels := eb.GetChannels(result)
	if len(channels) != 1 || channels[0] != "channel:task:task-xyz" {
		t.Errorf("Expected only target channel, got %v", channels)
	}
}

func TestChannelRouter_ExistingChannels_NoDuplicates(t *testing.T) {
	router := NewChannelRouter()
	ev := eb.NewEvent("core.test", "test:source", "task:task-123")
	eb.SetString(ev, eb.MetaProjectID, "proj-456")
	eb.SetChannels(ev, []string{"channel:task:task-123"})
	pc := &eb.PipelineCtx{}

	result, err := router.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	channels := eb.GetChannels(result)
	expected := []string{
		"channel:task:task-123",
		"channel:project:proj-456",
	}

	if len(channels) != len(expected) {
		t.Fatalf("Expected %d channels, got %d: %v", len(expected), len(channels), channels)
	}

	// Verify order (existing should come first)
	if channels[0] != "channel:task:task-123" {
		t.Errorf("Expected first channel to be 'channel:task:task-123', got %q", channels[0])
	}
	if channels[1] != "channel:project:proj-456" {
		t.Errorf("Expected second channel to be 'channel:project:proj-456', got %q", channels[1])
	}
}

func TestChannelRouter_InvalidTarget_NoError(t *testing.T) {
	router := NewChannelRouter()
	ev := eb.NewEvent("core.test", "test:source", "invalid-target")
	eb.SetString(ev, eb.MetaProjectID, "proj-123")
	pc := &eb.PipelineCtx{}

	result, err := router.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform should not error on invalid target, got %v", err)
	}

	channels := eb.GetChannels(result)
	// Should only have the project channel since target is invalid
	if len(channels) != 1 || channels[0] != "channel:project:proj-123" {
		t.Errorf("Expected only project channel, got %v", channels)
	}
}

func TestChannelRouter_PreservesExistingChannels(t *testing.T) {
	router := NewChannelRouter()
	ev := eb.NewEvent("core.test", "test:source", "task:task-123")
	eb.SetChannels(ev, []string{"custom:channel:1", "custom:channel:2"})
	eb.SetString(ev, eb.MetaProjectID, "proj-456")
	pc := &eb.PipelineCtx{}

	result, err := router.Transform(context.Background(), ev, pc)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	channels := eb.GetChannels(result)
	expected := map[string]bool{
		"custom:channel:1":         true,
		"custom:channel:2":         true,
		"channel:task:task-123":    true,
		"channel:project:proj-456": true,
	}

	if len(channels) != len(expected) {
		t.Fatalf("Expected %d channels, got %d: %v", len(expected), len(channels), channels)
	}

	for _, ch := range channels {
		if !expected[ch] {
			t.Errorf("Unexpected channel: %q", ch)
		}
	}
}

func TestChannelRouter_Name(t *testing.T) {
	router := NewChannelRouter()
	if router.Name() != "core.channel-router" {
		t.Errorf("Expected Name to be 'core.channel-router', got %q", router.Name())
	}
}

func TestChannelRouter_Intercepts(t *testing.T) {
	router := NewChannelRouter()
	intercepts := router.Intercepts()
	if len(intercepts) != 1 || intercepts[0] != "*" {
		t.Errorf("Expected Intercepts to be [*], got %v", intercepts)
	}
}

func TestChannelRouter_Priority(t *testing.T) {
	router := NewChannelRouter()
	if router.Priority() != 20 {
		t.Errorf("Expected Priority to be 20, got %d", router.Priority())
	}
}

func TestChannelRouter_Mode(t *testing.T) {
	router := NewChannelRouter()
	if router.Mode() != eb.ModeTransform {
		t.Errorf("Expected Mode to be ModeTransform, got %v", router.Mode())
	}
}
