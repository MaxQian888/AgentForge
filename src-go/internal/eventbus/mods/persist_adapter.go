package mods

import (
	"context"

	eb "github.com/agentforge/server/internal/eventbus"
	"github.com/agentforge/server/internal/eventbus/repository"
)

// repoWriter composes the two repositories into the eventWriter interface.
type repoWriter struct {
	events *repository.EventsRepository
	dlq    *repository.DeadLetterRepository
}

func (w *repoWriter) Insert(ctx context.Context, e *eb.Event) error {
	return w.events.Insert(ctx, e)
}

func (w *repoWriter) RecordDead(ctx context.Context, e *eb.Event, cause error, retries int) error {
	return w.dlq.Record(ctx, e, cause, retries)
}

// NewPersistFromRepos creates a Persist observer backed by real database repositories.
func NewPersistFromRepos(events *repository.EventsRepository, dlq *repository.DeadLetterRepository) *Persist {
	return NewPersistWithDeps(&repoWriter{events: events, dlq: dlq})
}
