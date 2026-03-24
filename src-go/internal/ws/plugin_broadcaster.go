package ws

import "github.com/react-go-quick-starter/server/internal/model"

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

	b.hub.BroadcastEvent(&Event{
		Type:    EventPluginLifecycle,
		Payload: event,
	})
}
