package mods

import (
	"context"
	"time"

	eb "github.com/react-go-quick-starter/server/internal/eventbus"
	log "github.com/sirupsen/logrus"
)

// eventWriter is the minimal interface needed by Persist so tests can fake it.
type eventWriter interface {
	Insert(ctx context.Context, e *eb.Event) error
	RecordDead(ctx context.Context, e *eb.Event, cause error, retries int) error
}

// Persist writes every observed event to the events table, falling back to a
// dead-letter queue on repeated failure.
type Persist struct {
	writer  eventWriter
	retries int
	backoff time.Duration
}

func NewPersistWithDeps(w eventWriter) *Persist {
	return &Persist{writer: w, retries: 2, backoff: 100 * time.Millisecond}
}

func (p *Persist) Name() string         { return "core.persist" }
func (p *Persist) Intercepts() []string { return []string{"*"} }
func (p *Persist) Priority() int        { return 10 }
func (p *Persist) Mode() eb.Mode        { return eb.ModeObserve }

func (p *Persist) Observe(ctx context.Context, e *eb.Event, pc *eb.PipelineCtx) {
	var lastErr error
	for attempt := 0; attempt <= p.retries; attempt++ {
		if err := p.writer.Insert(ctx, e); err == nil {
			return
		} else {
			lastErr = err
		}
		if attempt < p.retries {
			time.Sleep(p.backoff)
		}
	}
	if lastErr != nil {
		log.WithFields(log.Fields{"event_id": e.ID, "err": lastErr}).Warn("eventbus: persist failed, routing to DLQ")
		if dlErr := p.writer.RecordDead(ctx, e, lastErr, p.retries); dlErr != nil {
			log.WithFields(log.Fields{"event_id": e.ID, "err": dlErr}).Error("eventbus: DLQ write failed")
		}
	}
}
