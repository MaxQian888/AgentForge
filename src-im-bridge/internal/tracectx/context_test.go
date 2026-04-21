package tracectx_test

import (
	"context"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/internal/tracectx"
)

func TestRoundTrip(t *testing.T) {
	ctx := tracectx.With(context.Background(), "tr_x")
	if got := tracectx.TraceID(ctx); got != "tr_x" {
		t.Fatalf("got %q", got)
	}
}

func TestEmptyContext(t *testing.T) {
	if tracectx.TraceID(context.Background()) != "" {
		t.Fatal("want empty")
	}
	if tracectx.TraceID(nil) != "" {
		t.Fatal("want empty on nil ctx")
	}
}

func TestNewFormat(t *testing.T) {
	id := tracectx.New()
	if len(id) != 27 {
		t.Fatalf("want 27, got %d (%q)", len(id), id)
	}
	if !strings.HasPrefix(id, "tr_") {
		t.Fatalf("want tr_ prefix: %q", id)
	}
	if strings.ToLower(id) != id {
		t.Fatalf("want lowercase: %q", id)
	}
}

func TestNewUnique(t *testing.T) {
	a, b := tracectx.New(), tracectx.New()
	if a == b {
		t.Fatal("not unique")
	}
}
