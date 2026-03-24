package repository

import (
	"context"
	"testing"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestPluginEventRepository_AppendAndListInMemory(t *testing.T) {
	ctx := context.Background()
	repo := NewPluginEventRepository()

	if err := repo.Append(ctx, &model.PluginEventRecord{
		PluginID:       "feishu",
		EventType:      model.PluginEventInstalled,
		EventSource:    model.PluginEventSourceControlPlane,
		LifecycleState: model.PluginStateInstalled,
		Summary:        "plugin installed",
	}); err != nil {
		t.Fatalf("append install event: %v", err)
	}
	if err := repo.Append(ctx, &model.PluginEventRecord{
		PluginID:       "feishu",
		EventType:      model.PluginEventActivated,
		EventSource:    model.PluginEventSourceGoRuntime,
		LifecycleState: model.PluginStateActive,
		Summary:        "plugin activated",
	}); err != nil {
		t.Fatalf("append activate event: %v", err)
	}

	events, err := repo.ListByPluginID(ctx, "feishu", 10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].EventType != model.PluginEventActivated {
		t.Fatalf("latest event type = %s, want %s", events[0].EventType, model.PluginEventActivated)
	}
	if events[1].EventType != model.PluginEventInstalled {
		t.Fatalf("older event type = %s, want %s", events[1].EventType, model.PluginEventInstalled)
	}
}
