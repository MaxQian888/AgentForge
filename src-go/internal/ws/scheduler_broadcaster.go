package ws

import "github.com/react-go-quick-starter/server/internal/model"

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

	b.hub.BroadcastEvent(&Event{
		Type: eventType,
		Payload: map[string]any{
			"job": job,
			"run": run,
		},
	})
}
