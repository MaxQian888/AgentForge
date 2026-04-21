package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agentforge/server/internal/eventbus"
	applog "github.com/agentforge/server/internal/log"
	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
	"github.com/google/uuid"
)

// stubLogRepo captures the last Log passed to Create.
type stubLogRepo struct {
	last *model.Log
}

func (r *stubLogRepo) Create(_ context.Context, log *model.Log) error {
	r.last = log
	return nil
}

func (r *stubLogRepo) List(_ context.Context, _ model.LogListRequest) ([]model.Log, int64, error) {
	return nil, 0, nil
}

// noopPublisher satisfies eventbus.Publisher without doing anything.
type noopPublisher struct{}

func (noopPublisher) Publish(_ context.Context, _ *eventbus.Event) error { return nil }

func TestCreateLog_MergesTraceIDIntoDetail(t *testing.T) {
	repo := &stubLogRepo{}
	svc := service.NewLogService(repo, nil, noopPublisher{})

	ctx := applog.WithTrace(context.Background(), "tr_service00000000000000")
	in := model.CreateLogInput{
		ProjectID: uuid.New(),
		Tab:       model.LogTabSystem,
		Level:     model.LogLevelInfo,
		Summary:   "x",
		Detail:    map[string]any{"foo": "bar"},
	}
	if _, err := svc.CreateLog(ctx, in); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(repo.last.Detail, &got); err != nil {
		t.Fatal(err)
	}
	if got["trace_id"] != "tr_service00000000000000" {
		t.Fatalf("missing trace_id in detail: %+v", got)
	}
	if got["foo"] != "bar" {
		t.Fatalf("preexisting detail clobbered: %+v", got)
	}
}

func TestCreateLog_NoTraceID_DetailUnchanged(t *testing.T) {
	repo := &stubLogRepo{}
	svc := service.NewLogService(repo, nil, noopPublisher{})

	in := model.CreateLogInput{
		ProjectID: uuid.New(),
		Tab:       model.LogTabSystem,
		Level:     model.LogLevelInfo,
		Summary:   "x",
		Detail:    map[string]any{"foo": "bar"},
	}
	if _, err := svc.CreateLog(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	_ = json.Unmarshal(repo.last.Detail, &got)
	if _, hasTrace := got["trace_id"]; hasTrace {
		t.Fatalf("no trace_id should be added on empty ctx: %+v", got)
	}
	if got["foo"] != "bar" {
		t.Fatalf("detail preserved: %+v", got)
	}
}
