// src-go/internal/eventbus/bus.go
package eventbus

import (
	"context"
	"sync"

	log "github.com/sirupsen/logrus"
)

const maxCausationDepth = 3

type Bus struct {
	mu       sync.Mutex
	mods     []Mod
	pipeline *Pipeline
	started  bool
}

func NewBus() *Bus {
	return &Bus{}
}

// Register adds a mod. Only allowed before the first Publish.
func (b *Bus) Register(m Mod) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.started {
		panic("eventbus: Register called after Publish; wire all mods during startup")
	}
	b.mods = append(b.mods, m)
}

func (b *Bus) ensurePipeline() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.started {
		b.pipeline = NewPipeline(b.mods)
		b.started = true
	}
}

func (b *Bus) Publish(ctx context.Context, e *Event) error {
	b.ensurePipeline()
	return b.publishInternal(ctx, e)
}

func (b *Bus) publishInternal(ctx context.Context, e *Event) error {
	pc := &PipelineCtx{Attrs: map[string]any{}}
	out, err := b.pipeline.Process(ctx, e, pc)
	if err != nil {
		log.WithFields(log.Fields{"event_id": e.ID, "event_type": e.Type, "err": err}).
			Warn("eventbus: publish rejected")
		return err
	}
	depth := GetCausationDepth(out) + 1
	for i := range pc.Emits {
		child := pc.Emits[i]
		if depth > maxCausationDepth {
			log.WithFields(log.Fields{"parent": out.ID, "child_type": child.Type}).
				Warn("eventbus: causation depth limit, child dropped")
			continue
		}
		ensureMeta(&child)
		child.Metadata[MetaCausationID] = out.ID
		child.Metadata[MetaCausationDepth] = depth
		if err := b.publishInternal(ctx, &child); err != nil {
			log.WithFields(log.Fields{"parent": out.ID, "child_type": child.Type, "err": err}).
				Warn("eventbus: emitted child publish failed")
		}
	}
	return nil
}
