package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/internal/tracectx"
)

func TestAgentForgeClient_SendsTraceHeader(t *testing.T) {
	got := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Trace-ID")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewAgentForgeClient(srv.URL, "proj", "secret").WithSource("slack-stub")
	ctx := tracectx.With(context.Background(), "tr_imout0000000000000000000")
	_ = c.PostReaction(ctx, ReactionEvent{
		ChatID:    "C1",
		MessageID: "M1",
		UserID:    "U1",
		EmojiCode: "done",
		RawEmoji:  "white_check_mark",
		ReactedAt: time.Unix(1_700_000_000, 0).UTC(),
	})
	if got != "tr_imout0000000000000000000" {
		t.Fatalf("X-Trace-ID = %q, want tr_imout0000000000000000000", got)
	}
}

func TestAgentForgeClient_NoHeaderWhenEmptyTrace(t *testing.T) {
	hadKey := false
	got := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("X-Trace-ID")
		hadKey = got != ""
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	c := NewAgentForgeClient(srv.URL, "proj", "secret").WithSource("slack-stub")
	_ = c.PostReaction(context.Background(), ReactionEvent{
		ChatID:    "C1",
		MessageID: "M1",
		UserID:    "U1",
		EmojiCode: "done",
		RawEmoji:  "white_check_mark",
		ReactedAt: time.Unix(1_700_000_000, 0).UTC(),
	})
	if hadKey {
		t.Fatalf("expected no X-Trace-ID header, got %q", got)
	}
}
