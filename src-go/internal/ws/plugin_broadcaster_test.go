package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/react-go-quick-starter/server/internal/model"
)

func TestPluginEventBroadcaster_EmitsLifecycleEvent(t *testing.T) {
	hub := NewHub()
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
	}

	hub.mu.Lock()
	hub.clients[client] = struct{}{}
	hub.mu.Unlock()

	broadcaster := NewPluginEventBroadcaster(hub)
	createdAt := time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC)
	broadcaster.BroadcastPluginEvent(&model.PluginEventRecord{
		ID:             "evt-1",
		PluginID:       "feishu",
		EventType:      model.PluginEventActivated,
		EventSource:    model.PluginEventSourceGoRuntime,
		LifecycleState: model.PluginStateActive,
		Summary:        "plugin activated",
		CreatedAt:      createdAt,
	})

	select {
	case message := <-client.send:
		var frame struct {
			Type    string                  `json:"type"`
			Payload model.PluginEventRecord `json:"payload"`
		}
		if err := json.Unmarshal(message, &frame); err != nil {
			t.Fatalf("decode websocket event: %v", err)
		}
		if frame.Type != EventPluginLifecycle {
			t.Fatalf("event type = %q, want %q", frame.Type, EventPluginLifecycle)
		}
		if frame.Payload.PluginID != "feishu" {
			t.Fatalf("payload plugin id = %q, want feishu", frame.Payload.PluginID)
		}
		if frame.Payload.LifecycleState != model.PluginStateActive {
			t.Fatalf("payload lifecycle state = %q, want %q", frame.Payload.LifecycleState, model.PluginStateActive)
		}
	default:
		t.Fatal("expected plugin lifecycle event to be broadcast")
	}
}
