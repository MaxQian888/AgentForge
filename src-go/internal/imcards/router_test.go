package imcards

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type stubCorrelations struct {
	c       *Correlation
	lookErr error
	marked  []uuid.UUID
}

func (s *stubCorrelations) Lookup(_ context.Context, _ uuid.UUID) (*Correlation, error) {
	if s.lookErr != nil {
		return nil, s.lookErr
	}
	return s.c, nil
}
func (s *stubCorrelations) MarkConsumed(_ context.Context, t uuid.UUID) error {
	s.marked = append(s.marked, t)
	return nil
}

type stubResumer struct {
	called bool
	retErr error
}

func (s *stubResumer) Resume(_ context.Context, _ uuid.UUID, _ string, _ map[string]any) error {
	s.called = true
	return s.retErr
}

type stubFallback struct{ events []map[string]any }

func (s *stubFallback) RouteAsIMEvent(_ context.Context, ev map[string]any) error {
	s.events = append(s.events, ev)
	return nil
}

type stubAudit struct{ entries []string }

func (s *stubAudit) Record(_ context.Context, kind string, _ map[string]any) error {
	s.entries = append(s.entries, kind)
	return nil
}

func TestRouter_HitConsumes(t *testing.T) {
	execID := uuid.New()
	token := uuid.New()
	corr := &stubCorrelations{c: &Correlation{
		Token: token, ExecutionID: execID, NodeID: "wait-1", ActionID: "approve",
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	res := &stubResumer{}
	r := &Router{Correlations: corr, Resumer: res, Fallback: &stubFallback{}, Audit: &stubAudit{}}
	out, err := r.Route(context.Background(), RouteInput{Token: token, ActionID: "approve"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if !res.called {
		t.Error("resumer not called")
	}
	if len(corr.marked) != 1 {
		t.Error("token not consumed")
	}
	if out.Outcome != OutcomeResumed {
		t.Errorf("outcome = %s", out.Outcome)
	}
}

func TestRouter_Expired(t *testing.T) {
	token := uuid.New()
	corr := &stubCorrelations{c: &Correlation{
		Token: token, ExecutionID: uuid.New(), NodeID: "wait-1", ActionID: "approve",
		ExpiresAt: time.Now().Add(-time.Minute),
	}}
	r := &Router{Correlations: corr, Resumer: &stubResumer{}, Fallback: &stubFallback{}, Audit: &stubAudit{}}
	_, err := r.Route(context.Background(), RouteInput{Token: token})
	if !errors.Is(err, ErrCardActionExpired) {
		t.Fatalf("err = %v, want ErrCardActionExpired", err)
	}
}

func TestRouter_Consumed(t *testing.T) {
	token := uuid.New()
	now := time.Now()
	corr := &stubCorrelations{c: &Correlation{
		Token: token, ExpiresAt: time.Now().Add(time.Hour),
		ConsumedAt: &now,
	}}
	r := &Router{Correlations: corr, Resumer: &stubResumer{}, Fallback: &stubFallback{}, Audit: &stubAudit{}}
	_, err := r.Route(context.Background(), RouteInput{Token: token})
	if !errors.Is(err, ErrCardActionConsumed) {
		t.Fatalf("err = %v, want ErrCardActionConsumed", err)
	}
}

func TestRouter_NotFoundFallsBack(t *testing.T) {
	corr := &stubCorrelations{lookErr: ErrCorrelationNotFound}
	fb := &stubFallback{}
	r := &Router{Correlations: corr, Resumer: &stubResumer{}, Fallback: fb, Audit: &stubAudit{}}
	out, err := r.Route(context.Background(), RouteInput{
		Token: uuid.New(), ActionID: "free-form-button",
		ReplyTarget: map[string]any{"chat_id": "C1"},
	})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if out.Outcome != OutcomeFallback {
		t.Errorf("outcome = %s", out.Outcome)
	}
	if len(fb.events) != 1 {
		t.Fatal("fallback not invoked")
	}
}

type stubAutomation struct {
	calls []struct {
		FindingID uuid.UUID
		Action    string
		Actor     string
	}
}

func (s *stubAutomation) Decide(_ context.Context, findingID uuid.UUID, action string, actor string) error {
	s.calls = append(s.calls, struct {
		FindingID uuid.UUID
		Action    string
		Actor     string
	}{findingID, action, actor})
	return nil
}

func TestRouter_AutomationBranchDispatchesToHandler(t *testing.T) {
	findingID := uuid.New()
	token := uuid.New()
	corr := &stubCorrelations{c: &Correlation{
		Token:       token,
		ExecutionID: uuid.Nil, // null execution_id → automation branch
		NodeID:      "(automation)",
		ActionID:    "apply",
		Payload:     map[string]any{"finding_id": findingID.String(), "action": "apply"},
		ExpiresAt:   time.Now().Add(time.Hour),
	}}
	auto := &stubAutomation{}
	r := &Router{
		Correlations: corr,
		Resumer:      &stubResumer{},
		Fallback:     &stubFallback{},
		Audit:        &stubAudit{},
		Automation:   auto,
	}
	out, err := r.Route(context.Background(), RouteInput{Token: token, ActionID: "apply", UserID: "user-1"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if out.Outcome != OutcomeAutomationDispatched {
		t.Errorf("outcome = %s, want automation_dispatched", out.Outcome)
	}
	if len(auto.calls) != 1 {
		t.Fatalf("automation calls = %d, want 1", len(auto.calls))
	}
	if auto.calls[0].FindingID != findingID {
		t.Errorf("finding_id mismatch")
	}
	if auto.calls[0].Action != "apply" {
		t.Errorf("action = %q, want apply", auto.calls[0].Action)
	}
	if auto.calls[0].Actor != "user-1" {
		t.Errorf("actor = %q, want user-1", auto.calls[0].Actor)
	}
	if len(corr.marked) != 1 {
		t.Error("token not consumed")
	}
}

func TestRouter_AutomationBranch_DismissAction(t *testing.T) {
	findingID := uuid.New()
	token := uuid.New()
	corr := &stubCorrelations{c: &Correlation{
		Token:       token,
		ExecutionID: uuid.Nil,
		NodeID:      "(automation)",
		ActionID:    "dismiss",
		Payload:     map[string]any{"finding_id": findingID.String(), "action": "dismiss"},
		ExpiresAt:   time.Now().Add(time.Hour),
	}}
	auto := &stubAutomation{}
	r := &Router{
		Correlations: corr,
		Resumer:      &stubResumer{},
		Fallback:     &stubFallback{},
		Audit:        &stubAudit{},
		Automation:   auto,
	}
	out, err := r.Route(context.Background(), RouteInput{Token: token, ActionID: "dismiss", UserID: "user-2"})
	if err != nil {
		t.Fatalf("Route: %v", err)
	}
	if out.Outcome != OutcomeAutomationDispatched {
		t.Errorf("outcome = %s", out.Outcome)
	}
	if len(auto.calls) != 1 || auto.calls[0].Action != "dismiss" {
		t.Errorf("unexpected calls: %+v", auto.calls)
	}
}

func TestRouter_ResumerNotWaiting(t *testing.T) {
	token := uuid.New()
	corr := &stubCorrelations{c: &Correlation{
		Token: token, ExecutionID: uuid.New(), NodeID: "wait-1", ActionID: "x",
		ExpiresAt: time.Now().Add(time.Hour),
	}}
	res := &stubResumer{retErr: errors.New("wait_event: target node is not waiting")}
	r := &Router{Correlations: corr, Resumer: res, Fallback: &stubFallback{}, Audit: &stubAudit{}}
	_, err := r.Route(context.Background(), RouteInput{Token: token})
	if !errors.Is(err, ErrExecutionNotWaiting) {
		t.Fatalf("err = %v, want ErrExecutionNotWaiting", err)
	}
}
