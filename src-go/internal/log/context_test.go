package log_test

import (
	"context"
	"testing"

	applog "github.com/agentforge/server/internal/log"
)

func TestTraceID_EmptyContext(t *testing.T) {
	if got := applog.TraceID(context.Background()); got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestWithTrace_RoundTrip(t *testing.T) {
	ctx := applog.WithTrace(context.Background(), "tr_abc")
	if got := applog.TraceID(ctx); got != "tr_abc" {
		t.Fatalf("want tr_abc, got %q", got)
	}
}

func TestNewTraceID_FormatAndUniqueness(t *testing.T) {
	a := applog.NewTraceID()
	b := applog.NewTraceID()
	if a == b {
		t.Fatal("IDs must be unique")
	}
	if len(a) != 27 { // "tr_" + 24 chars
		t.Fatalf("want length 27, got %d (%q)", len(a), a)
	}
	if a[:3] != "tr_" {
		t.Fatalf("want prefix tr_, got %q", a[:3])
	}
}
