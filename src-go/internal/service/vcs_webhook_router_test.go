package service_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/agentforge/server/internal/model"
	"github.com/agentforge/server/internal/service"
)

type fakeReviewTrigger struct {
	called  bool
	lastReq *model.TriggerReviewRequest
}

func (f *fakeReviewTrigger) Trigger(_ context.Context, r *model.TriggerReviewRequest) (*model.Review, error) {
	f.called = true
	f.lastReq = r
	return &model.Review{ID: uuid.New()}, nil
}

func TestRouter_PullRequestOpened_TriggersReview(t *testing.T) {
	rt := &fakeReviewTrigger{}
	r := service.NewVCSWebhookRouter(rt)
	integ := &model.VCSIntegration{ID: uuid.New(), ProjectID: uuid.New(), Owner: "o", Repo: "r", Host: "github.com"}

	body := []byte(`{"action":"opened","pull_request":{"number":42,
      "head":{"sha":"abc"},"base":{"sha":"def"},
      "html_url":"https://github.com/o/r/pull/42"}}`)

	if err := r.RouteEvent(context.Background(), integ, "pull_request", "delivery-1", body); err != nil {
		t.Fatalf("route: %v", err)
	}
	if !rt.called {
		t.Fatal("ReviewService.Trigger not called")
	}
	if rt.lastReq.PRNumber != 42 || rt.lastReq.HeadSHA != "abc" {
		t.Fatalf("review req fields wrong: %+v", rt.lastReq)
	}
	if rt.lastReq.BaseSHA != "def" {
		t.Fatalf("base SHA wrong: %s", rt.lastReq.BaseSHA)
	}
	if rt.lastReq.Trigger != "vcs_webhook" {
		t.Fatalf("trigger wrong: %s", rt.lastReq.Trigger)
	}
}

func TestRouter_PullRequestSynchronize_DelegatesToPushHandler(t *testing.T) {
	r := service.NewVCSWebhookRouter(&fakeReviewTrigger{})
	integ := &model.VCSIntegration{ID: uuid.New()}
	body := []byte(`{"action":"synchronize","pull_request":{"number":42}}`)
	err := r.RouteEvent(context.Background(), integ, "pull_request", "d", body)
	if !errors.Is(err, service.ErrPushHandlerNotImplemented) {
		t.Fatalf("expected ErrPushHandlerNotImplemented (Plan 2C seam), got %v", err)
	}
}

func TestRouter_PushEvent_DelegatesToPushHandler(t *testing.T) {
	r := service.NewVCSWebhookRouter(&fakeReviewTrigger{})
	err := r.RouteEvent(context.Background(), &model.VCSIntegration{ID: uuid.New()}, "push", "d", []byte(`{}`))
	if !errors.Is(err, service.ErrPushHandlerNotImplemented) {
		t.Fatalf("expected ErrPushHandlerNotImplemented, got %v", err)
	}
}

func TestRouter_UnknownEvent_NoOp(t *testing.T) {
	r := service.NewVCSWebhookRouter(&fakeReviewTrigger{})
	if err := r.RouteEvent(context.Background(), &model.VCSIntegration{}, "ping", "d", []byte(`{}`)); err != nil {
		t.Fatalf("ping should be no-op, got %v", err)
	}
}
