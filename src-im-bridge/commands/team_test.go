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

func TestTeamCommand_AddAndRemoveMember(t *testing.T) {
	requests := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects/proj/members":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			if body["name"] != "Alice" || body["type"] != "human" || body["role"] != "lead" {
				t.Fatalf("body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"id":        "member-1",
				"projectId": "proj",
				"name":      "Alice",
				"type":      "human",
				"role":      "lead",
				"status":    "active",
				"isActive":  true,
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj/members":
			_ = json.NewEncoder(w).Encode([]client.Member{
				{ID: "member-1", Name: "Alice", Type: "human", Role: "lead", Status: "active", IsActive: true},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/members/member-1":
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "member deleted"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterTeamCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/team add human Alice lead"})
	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/team remove Alice"})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "已添加成员: Alice") {
		t.Fatalf("add reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "已移除成员: Alice") {
		t.Fatalf("remove reply = %q", platform.replies[1])
	}
	if got := strings.Join(requests, ","); !strings.Contains(got, "POST /api/v1/projects/proj/members") || !strings.Contains(got, "DELETE /api/v1/members/member-1") {
		t.Fatalf("requests = %v", requests)
	}
}
