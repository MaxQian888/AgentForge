package mods

import (
	"context"
	"errors"
	"testing"

	eb "github.com/agentforge/server/internal/eventbus"
	"github.com/stretchr/testify/assert"
)

type fakeWriter struct {
	inserts  []*eb.Event
	failOnce error
	dlq      []*eb.Event
	dlqErr   []error
}

func (f *fakeWriter) Insert(ctx context.Context, e *eb.Event) error {
	if f.failOnce != nil {
		err := f.failOnce
		f.failOnce = nil
		return err
	}
	f.inserts = append(f.inserts, e)
	return nil
}

func (f *fakeWriter) RecordDead(ctx context.Context, e *eb.Event, err error, retries int) error {
	f.dlq = append(f.dlq, e)
	f.dlqErr = append(f.dlqErr, err)
	return nil
}

func TestPersist_HappyPath(t *testing.T) {
	w := &fakeWriter{}
	p := NewPersistWithDeps(w)
	p.Observe(context.Background(), eb.NewEvent("task.created", "core", "task:1"), &eb.PipelineCtx{})
	assert.Len(t, w.inserts, 1)
}

func TestPersist_FailureRoutesToDLQ(t *testing.T) {
	alwaysFail := &fakeWriter{failOnce: errors.New("boom")}
	// Make failOnce permanent by re-setting after each call
	p := NewPersistWithDeps(alwaysFail)
	p.retries = 0 // no retry for test speed
	p.Observe(context.Background(), eb.NewEvent("task.created", "core", "task:1"), &eb.PipelineCtx{})
	assert.Len(t, alwaysFail.dlq, 1)
	assert.EqualError(t, alwaysFail.dlqErr[0], "boom")
}
