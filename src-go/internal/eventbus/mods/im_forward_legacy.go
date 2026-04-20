package mods

import (
	"context"
	"encoding/json"

	eb "github.com/agentforge/server/internal/eventbus"
	log "github.com/sirupsen/logrus"
)

// LegacyIMRouter is the minimal shape this transitional observer needs
// from the existing IM event routing. The real implementation still lives
// in internal/service and will be rewritten in M4 as a first-class
// core.im-forward observer.
type LegacyIMRouter interface {
	Dispatch(ctx context.Context, projectID, eventType string, payload json.RawMessage) error
}

// IMForwardLegacy wraps the pre-eventbus IM routing path behind an
// ObserveMod so the bus becomes the sole producer during M1. The event
// types it forwards are intentionally narrow — they mirror what the
// legacy router cared about. Everything else is a no-op.
type IMForwardLegacy struct {
	router LegacyIMRouter
}

func NewIMForwardLegacy(r LegacyIMRouter) *IMForwardLegacy {
	return &IMForwardLegacy{router: r}
}

func (m *IMForwardLegacy) Name() string { return "im.forward-legacy" }

// TODO(M4): replace with a proper core.im-forward covering all categories.
func (m *IMForwardLegacy) Intercepts() []string {
	return []string{"task.*", "review.*", "agent.*", "notification"}
}

func (m *IMForwardLegacy) Priority() int { return 80 }
func (m *IMForwardLegacy) Mode() eb.Mode { return eb.ModeObserve }

func (m *IMForwardLegacy) Observe(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) {
	if m == nil || m.router == nil || e == nil {
		return
	}
	pid := eb.GetString(e, eb.MetaProjectID)
	if pid == "" {
		return
	}
	if err := m.router.Dispatch(ctx, pid, e.Type, e.Payload); err != nil {
		log.WithFields(log.Fields{"event_id": e.ID, "err": err}).Warn("im.forward-legacy dispatch failed")
	}
}
