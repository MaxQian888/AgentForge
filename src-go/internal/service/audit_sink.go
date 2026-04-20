// Package service — audit_sink.go is the asynchronous persistence machinery
// for audit events. The sink decouples the caller's request path from the
// audit table's health: a slow or unreachable database produces a metric +
// disk spill, never a user-visible failure.
//
// Lifecycle:
//  1. NewAuditSink builds an unstarted sink.
//  2. Start(ctx) launches the worker goroutine.
//  3. AuditService.RecordEvent → sink.Enqueue (non-blocking when room).
//  4. Worker drains the queue, retrying with exponential backoff on
//     ErrDatabaseUnavailable. After degradationWindow of sustained
//     failure, events spill to logs/audit_backlog.jsonl and the sink
//     emits a degraded signal via the supplied logger.
//  5. Stop drains in-flight events with a bounded shutdown deadline.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/repository"
)

// AuditSinkConfig tunes the sink's retry/spill behavior. Zero values fall
// back to documented defaults so simple wiring stays simple.
type AuditSinkConfig struct {
	// QueueCapacity bounds in-memory pending events. Default 1000.
	QueueCapacity int
	// MaxAttempts is the maximum retry count before spilling. Default 8.
	MaxAttempts int
	// InitialBackoff is the first retry delay. Default 200ms.
	InitialBackoff time.Duration
	// MaxBackoff caps exponential backoff. Default 30s.
	MaxBackoff time.Duration
	// DegradationWindow is the sustained-failure threshold past which the
	// sink starts spilling to disk and emits the audit_sink_degraded
	// signal. Default 5 minutes.
	DegradationWindow time.Duration
	// SpillFilePath is where spilled events land. Default
	// "logs/audit_backlog.jsonl" relative to the server CWD.
	SpillFilePath string
	// DedupWindow controls the rbac_denied dedup horizon. Identical
	// (actor_user_id, action_id, resource_id) tuples within this window
	// are folded. Default 60s.
	DedupWindow time.Duration
}

func (c *AuditSinkConfig) withDefaults() {
	if c.QueueCapacity <= 0 {
		c.QueueCapacity = 1000
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 8
	}
	if c.InitialBackoff <= 0 {
		c.InitialBackoff = 200 * time.Millisecond
	}
	if c.MaxBackoff <= 0 {
		c.MaxBackoff = 30 * time.Second
	}
	if c.DegradationWindow <= 0 {
		c.DegradationWindow = 5 * time.Minute
	}
	if c.SpillFilePath == "" {
		c.SpillFilePath = filepath.Join("logs", "audit_backlog.jsonl")
	}
	if c.DedupWindow <= 0 {
		c.DedupWindow = 60 * time.Second
	}
}

// AuditWriter is the persistence contract. Repository implements it.
type AuditWriter interface {
	Insert(ctx context.Context, event *model.AuditEvent) error
}

// AuditSink owns the queue worker. Safe for concurrent Enqueue from any
// goroutine; only one worker drains.
type AuditSink struct {
	cfg    AuditSinkConfig
	writer AuditWriter
	logger *log.Entry

	queue       chan *model.AuditEvent
	dedup       *dedupCache
	failingFrom atomic.Int64 // unix-nano of the first failure in the current degradation streak; 0 when healthy

	wg     sync.WaitGroup
	cancel context.CancelFunc

	// Test hooks. nil in production.
	onPersistAttempt func(event *model.AuditEvent, err error)
}

// NewAuditSink constructs a sink. Call Start to launch the worker.
func NewAuditSink(writer AuditWriter, cfg AuditSinkConfig) *AuditSink {
	cfg.withDefaults()
	logger := log.WithField("component", "audit_sink")
	return &AuditSink{
		cfg:    cfg,
		writer: writer,
		logger: logger,
		queue:  make(chan *model.AuditEvent, cfg.QueueCapacity),
		dedup:  newDedupCache(cfg.DedupWindow),
	}
}

// Start launches the worker goroutine. Calling Start twice is a no-op.
func (s *AuditSink) Start(ctx context.Context) {
	if s.cancel != nil {
		return
	}
	ctx, s.cancel = context.WithCancel(ctx)
	s.wg.Add(1)
	go s.run(ctx)
}

// Stop signals the worker and waits up to the given deadline for the
// queue to drain. Events still pending after the deadline are spilled.
func (s *AuditSink) Stop(deadline time.Duration) {
	if s.cancel == nil {
		return
	}
	s.cancel()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(deadline):
		s.logger.Warn("audit_sink: shutdown deadline exceeded; remaining events spilled to disk")
		s.drainToSpill()
	}
}

// Enqueue is the AuditEventSink contract. Non-blocking: when the queue is
// full, the event is spilled directly to disk and a warning is emitted.
//
// The sink also enforces dedup for rbac_denied events here so duplicate
// reject bursts don't take down the queue.
func (s *AuditSink) Enqueue(ctx context.Context, event *model.AuditEvent) {
	if event == nil {
		return
	}
	if s.shouldDedup(event) {
		return
	}
	select {
	case s.queue <- event:
	default:
		s.logger.WithField("action_id", event.ActionID).Warn("audit_sink: queue full, spilling event to disk")
		s.spill(event, errors.New("queue_full"))
	}
}

func (s *AuditSink) shouldDedup(event *model.AuditEvent) bool {
	// Only dedup the synthetic rbac_denied class — real handler-success
	// events are unique business records and must not be folded.
	if !isRBACDeniedEvent(event) {
		return false
	}
	actorID := ""
	if event.ActorUserID != nil {
		actorID = event.ActorUserID.String()
	}
	key := dedupKey(actorID, event.ActionID, event.ResourceID)
	return !s.dedup.shouldEmit(key, time.Now())
}

func (s *AuditSink) run(ctx context.Context) {
	defer s.wg.Done()
	for {
		select {
		case <-ctx.Done():
			s.drainToSpill()
			return
		case event := <-s.queue:
			s.persistWithRetry(ctx, event)
		}
	}
}

func (s *AuditSink) persistWithRetry(ctx context.Context, event *model.AuditEvent) {
	backoff := s.cfg.InitialBackoff
	for attempt := 1; attempt <= s.cfg.MaxAttempts; attempt++ {
		err := s.writer.Insert(ctx, event)
		if s.onPersistAttempt != nil {
			s.onPersistAttempt(event, err)
		}
		if err == nil {
			s.markHealthy()
			return
		}
		// Non-retryable conditions: skip backoff and spill immediately.
		// Today only "writer not wired" qualifies; transient DB errors
		// retry until MaxAttempts.
		if errors.Is(err, repository.ErrDatabaseUnavailable) {
			// First failure starts the degradation timer.
			s.markFailing(time.Now())
		}
		s.logger.WithError(err).WithField("attempt", attempt).
			Warn("audit_sink: insert failed; will retry")

		// If we're already past the degradation window, stop retrying
		// against the database and spill — the operator already needs
		// to intervene.
		if s.shouldGiveUpToSpill(time.Now()) {
			s.spill(event, fmt.Errorf("degradation_window_exceeded: %w", err))
			return
		}

		select {
		case <-ctx.Done():
			s.spill(event, fmt.Errorf("shutdown_during_retry: %w", err))
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > s.cfg.MaxBackoff {
			backoff = s.cfg.MaxBackoff
		}
	}
	// Exhausted retries — final spill.
	s.spill(event, fmt.Errorf("max_attempts_exceeded after %d", s.cfg.MaxAttempts))
}

func (s *AuditSink) markFailing(now time.Time) {
	s.failingFrom.CompareAndSwap(0, now.UnixNano())
}

func (s *AuditSink) markHealthy() {
	s.failingFrom.Store(0)
}

func (s *AuditSink) shouldGiveUpToSpill(now time.Time) bool {
	from := s.failingFrom.Load()
	if from == 0 {
		return false
	}
	return now.Sub(time.Unix(0, from)) >= s.cfg.DegradationWindow
}

func (s *AuditSink) drainToSpill() {
	for {
		select {
		case event := <-s.queue:
			s.spill(event, errors.New("shutdown_drain"))
		default:
			return
		}
	}
}

// spill writes an event to the append-only spill file. Errors here are
// terminal — there is no further fallback. We log loudly so operators can
// recover by hand.
func (s *AuditSink) spill(event *model.AuditEvent, reason error) {
	if event == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.cfg.SpillFilePath), 0o755); err != nil {
		s.logger.WithError(err).Error("audit_sink: cannot create spill directory; event dropped")
		return
	}
	f, err := os.OpenFile(s.cfg.SpillFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		s.logger.WithError(err).Error("audit_sink: cannot open spill file; event dropped")
		return
	}
	defer f.Close()

	wrapper := struct {
		SpilledAt string              `json:"spilledAt"`
		Reason    string              `json:"reason"`
		Event     model.AuditEventDTO `json:"event"`
	}{
		SpilledAt: time.Now().UTC().Format(time.RFC3339Nano),
		Reason:    reason.Error(),
		Event:     event.ToDTO(),
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(wrapper); err != nil {
		s.logger.WithError(err).Error("audit_sink: failed to write spill record")
		return
	}
	s.logger.WithFields(log.Fields{
		"audit_sink_degraded": true,
		"action_id":           event.ActionID,
		"reason":              reason.Error(),
	}).Warn("audit_sink: event spilled to disk; operator replay required")
}

func isRBACDeniedEvent(event *model.AuditEvent) bool {
	return event != nil && event.ResourceType == model.AuditResourceTypeAuth
}

// dedupCache is a tiny LRU keyed by (actor, action, resource). Entries
// expire after `window`; mid-window duplicates are suppressed.
type dedupCache struct {
	mu     sync.Mutex
	window time.Duration
	seen   map[string]time.Time
}

func newDedupCache(window time.Duration) *dedupCache {
	return &dedupCache{
		window: window,
		seen:   make(map[string]time.Time),
	}
}

func (d *dedupCache) shouldEmit(key string, now time.Time) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	if last, ok := d.seen[key]; ok && now.Sub(last) < d.window {
		return false
	}
	d.seen[key] = now
	// Light GC: opportunistically prune stale keys when the cache grows.
	if len(d.seen) > 4096 {
		for k, t := range d.seen {
			if now.Sub(t) >= d.window {
				delete(d.seen, k)
			}
		}
	}
	return true
}

func dedupKey(actorID, actionID, resourceID string) string {
	return actorID + "|" + actionID + "|" + resourceID
}
