package ws

import (
	"encoding/json"

	"github.com/agentforge/server/internal/model"
)

type SchedulerEventBroadcaster struct {
	hub *Hub
}

func NewSchedulerEventBroadcaster(hub *Hub) *SchedulerEventBroadcaster {
	return &SchedulerEventBroadcaster{hub: hub}
}

func (b *SchedulerEventBroadcaster) BroadcastSchedulerEvent(eventType string, job *model.ScheduledJob, run *model.ScheduledJobRun) {
	if b == nil || b.hub == nil {
		return
	}
	frame := map[string]any{
		"type": eventType,
		"payload": map[string]any{
			"job": job,
			"run": run,
		},
	}
	data, err := json.Marshal(frame)
	if err != nil {
		return
	}
	b.hub.BroadcastAllBytes(data)
}
