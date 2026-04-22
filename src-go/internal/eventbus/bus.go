// src-go/internal/eventbus/bus.go
package eventbus

import (
	"context"
	"sync"

	applog "github.com/agentforge/server/internal/log"
	log "github.com/sirupsen/logrus"
)

const maxCausationDepth = 3

// subscriber wraps a delivery channel with its own mutex so that
// tryDeliver and shutdown are mutually exclusive: a send can never race
// with a close even when fanoutToSubscribers holds a stale snapshot that
// still contains a subscriber whose context has already been cancelled.
type subscriber struct {
	mu     sync.Mutex
	ch     chan *Event
	closed bool
}

func (s *subscriber) tryDeliver(e *Event) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	select {
	case s.ch <- e:
	default:
	}
}

func (s *subscriber) shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.closed {
		s.closed = true
		close(s.ch)
	}
}

type Bus struct {
	mu          sync.Mutex
	mods        []Mod
	pipeline    *Pipeline
	started     bool
	subscribers []*subscriber
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
	if tid := applog.TraceID(ctx); tid != "" {
		ensureMeta(e)
		if _, set := e.Metadata[MetaTraceID]; !set {
			e.Metadata[MetaTraceID] = tid
		}
	}
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
	b.fanoutToSubscribers(out)
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

// Subscribe returns a buffered channel that receives every event the bus
// publishes while ctx is live. Unlike Register, Subscribe is safe to call
// at any time — even after the pipeline has started — because subscribers
// run after pipeline mods and never participate in guard/transform
// decisions. The channel closes when ctx is cancelled.
//
// Use this for runtime listeners (e.g. EventDrivenExecutor) that need to
// react to events without becoming part of the pipeline. Slow consumers
// silently drop overflow events; the buffer (64) is sized for short
// bursts, not sustained backpressure.
func (b *Bus) Subscribe(ctx context.Context) <-chan *Event {
	sub := &subscriber{ch: make(chan *Event, 64)}
	b.mu.Lock()
	b.subscribers = append(b.subscribers, sub)
	b.mu.Unlock()
	go func() {
		<-ctx.Done()
		b.mu.Lock()
		for i, s := range b.subscribers {
			if s == sub {
				b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
				break
			}
		}
		b.mu.Unlock()
		sub.shutdown()
	}()
	return sub.ch
}

// fanoutToSubscribers delivers e to every active subscriber. Non-blocking:
// if a subscriber's buffer is full the event is dropped for that subscriber
// (the rest still receive it).
func (b *Bus) fanoutToSubscribers(e *Event) {
	b.mu.Lock()
	subs := make([]*subscriber, len(b.subscribers))
	copy(subs, b.subscribers)
	b.mu.Unlock()
	for _, sub := range subs {
		sub.tryDeliver(e)
	}
}
