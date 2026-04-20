package ws

import (
	"encoding/json"

	"github.com/agentforge/server/internal/model"
)

type PluginEventBroadcaster struct {
	hub *Hub
}

func NewPluginEventBroadcaster(hub *Hub) *PluginEventBroadcaster {
	return &PluginEventBroadcaster{hub: hub}
}

func (b *PluginEventBroadcaster) BroadcastPluginEvent(event *model.PluginEventRecord) {
	if b == nil || b.hub == nil || event == nil {
		return
	}
	frame := map[string]any{
		"type":    EventPluginLifecycle,
		"payload": event,
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return
	}
	b.hub.BroadcastAllBytes(data)
}
