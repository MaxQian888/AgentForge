package client

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgentForgeClient_PostReaction_SendsEventToBackend(t *testing.T) {
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/api/v1/im/reactions" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &captured); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	c := NewAgentForgeClient(server.URL, "proj", "secret").WithSource("slack-live")
	c.bridgeID = "br-1"

	err := c.PostReaction(context.Background(), ReactionEvent{
		ChatID:    "C1",
		MessageID: "M1",
		UserID:    "U1",
		EmojiCode: "done",
		RawEmoji:  "white_check_mark",
		ReactedAt: time.Unix(1_700_000_000, 0).UTC(),
	})
	if err != nil {
		t.Fatalf("PostReaction error: %v", err)
	}
	if captured["platform"] != "slack" {
		t.Fatalf("platform = %v", captured["platform"])
	}
	if captured["emoji_code"] != "done" {
		t.Fatalf("emoji_code = %v", captured["emoji_code"])
	}
	if captured["bridge_id"] != "br-1" {
		t.Fatalf("bridge_id = %v", captured["bridge_id"])
	}
}
