package mods

import (
	"context"
	"encoding/json"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	log "github.com/sirupsen/logrus"
)

// HubClient is the minimal Hub surface the ws-fanout observer needs.
// *ws.Hub satisfies it; tests fake it.
type HubClient interface {
	FanoutBytes(data []byte, channels []string)
	BroadcastAllBytes(data []byte)
}

// WSFanout is an observer that serializes events into the client-facing
// frame format and dispatches them through the WebSocket hub based on the
// event's visibility.
type WSFanout struct {
	hub HubClient
}

func NewWSFanout(h HubClient) *WSFanout { return &WSFanout{hub: h} }

func (w *WSFanout) Name() string         { return "core.ws-fanout" }
func (w *WSFanout) Intercepts() []string { return []string{"*"} }
func (w *WSFanout) Priority() int        { return 50 }
func (w *WSFanout) Mode() eb.Mode        { return eb.ModeObserve }

type wsFrame struct {
	Channel string    `json:"channel,omitempty"`
	Event   *eb.Event `json:"event"`
}

func (w *WSFanout) Observe(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) {
	if w.hub == nil || e == nil {
		return
	}
	switch e.Visibility {
	case eb.VisibilityModOnly, eb.VisibilityDirect:
		// M1: direct delivery is deferred to M2; mod_only never reaches WS.
		return
	case eb.VisibilityPublic:
		data, err := json.Marshal(wsFrame{Event: e})
		if err != nil {
			log.WithFields(log.Fields{"event_id": e.ID, "err": err}).Warn("ws-fanout: marshal")
			return
		}
		w.hub.BroadcastAllBytes(data)
		return
	}
	// channel visibility: emit one frame per target channel.
	for _, ch := range eb.GetChannels(e) {
		data, err := json.Marshal(wsFrame{Channel: ch, Event: e})
		if err != nil {
			continue
		}
		w.hub.FanoutBytes(data, []string{ch})
	}
}
