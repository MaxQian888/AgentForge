package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

func TestQueueCommands_ListAndCancel(t *testing.T) {
	calls := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj/queue":
			_ = json.NewEncoder(w).Encode([]client.QueueEntry{
				{EntryID: "entry-1", TaskID: "task-1", MemberID: "member-1", Status: "queued", Priority: 20, Reason: "agent pool is at capacity"},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/projects/proj/queue/entry-1":
			_ = json.NewEncoder(w).Encode(&client.QueueEntry{
				EntryID: "entry-1", TaskID: "task-1", MemberID: "member-1", Status: "cancelled", Priority: 20, Reason: "manual_cancel",
			})
		default:
			t.Fatalf("unexpected request: %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterQueueCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/queue list queued",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/queue cancel entry-1",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "entry-1") || !strings.Contains(platform.replies[0], "queued") {
		t.Fatalf("list reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "cancelled") {
		t.Fatalf("cancel reply = %q", platform.replies[1])
	}
}

func TestTeamCommand_ListSummarizesProjectMembers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/projects/proj/members" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Member{
			{ID: "member-1", Name: "Alice", Type: "human", Role: "lead", Status: "active", IsActive: true},
			{ID: "member-2", Name: "Codex", Type: "agent", Role: "coder", Status: "active", IsActive: true, Skills: []string{"go", "nextjs"}},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTeamCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/team list",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	for _, want := range []string{"Alice", "Codex", "lead", "coder"} {
		if !strings.Contains(platform.replies[0], want) {
			t.Fatalf("reply = %q, want substring %q", platform.replies[0], want)
		}
	}
}

func TestMemoryCommands_SearchAndNote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj/memory":
			_ = json.NewEncoder(w).Encode([]client.MemoryEntry{
				{ID: "mem-1", Key: "release-plan", Content: "Coordinate deployment in phases", Category: "semantic", Scope: "project"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/proj/memory":
			_ = json.NewEncoder(w).Encode(&client.MemoryEntry{
				ID: "mem-2", Key: "operator-note", Content: "Remember to reuse Codex", Category: "operator_note", Scope: "project",
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterMemoryCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/memory search release",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/memory note Remember to reuse Codex",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "release-plan") || !strings.Contains(platform.replies[0], "Coordinate deployment") {
		t.Fatalf("search reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "mem-2") || !strings.Contains(platform.replies[1], "operator_note") {
		t.Fatalf("note reply = %q", platform.replies[1])
	}
}
