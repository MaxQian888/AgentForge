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

func TestProjectCommand_ListCurrentAndSet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/projects":
			_ = json.NewEncoder(w).Encode([]client.Project{
				{
					ID:   "proj-1",
					Name: "Alpha",
					Slug: "alpha",
					Settings: client.ProjectSettings{
						CodingAgent: client.CodingAgentSelection{Runtime: "codex", Provider: "openai", Model: "gpt-5-codex"},
					},
				},
				{
					ID:   "proj-2",
					Name: "Beta",
					Slug: "beta",
					Settings: client.ProjectSettings{
						CodingAgent: client.CodingAgentSelection{Runtime: "claude_code", Provider: "anthropic", Model: "claude-sonnet-4-5"},
					},
				},
			})
		case "/api/v1/projects/proj-2":
			_ = json.NewEncoder(w).Encode(client.Project{
				ID:   "proj-2",
				Name: "Beta",
				Slug: "beta",
				Settings: client.ProjectSettings{
					CodingAgent: client.CodingAgentSelection{Runtime: "claude_code", Provider: "anthropic", Model: "claude-sonnet-4-5"},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterProjectCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/project list"})
	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/project set beta"})
	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/project current"})

	if len(platform.replies) != 3 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "Alpha (alpha)") || !strings.Contains(platform.replies[0], "Beta (beta)") {
		t.Fatalf("list reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "已切换到项目: Beta (beta)") || !strings.Contains(platform.replies[1], "claude_code / anthropic / claude-sonnet-4-5") {
		t.Fatalf("set reply = %q", platform.replies[1])
	}
	if !strings.Contains(platform.replies[2], "当前项目: Beta (beta)") {
		t.Fatalf("current reply = %q", platform.replies[2])
	}
	if got := apiClient.ProjectScope(); got != "proj-2" {
		t.Fatalf("project scope = %q, want proj-2", got)
	}
}

func TestProjectCommand_SetRejectsUnknownProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]client.Project{
			{ID: "proj-1", Name: "Alpha", Slug: "alpha"},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterProjectCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/project set missing"})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "找不到项目") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}

func TestProjectCommand_CreateAndDeleteRequireLeadRole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/projects/proj/members":
			_ = json.NewEncoder(w).Encode([]client.Member{
				{ID: "member-1", Name: "Alice", Type: "human", Role: "contributor", Status: "active", IsActive: true, IMUserID: "user-1"},
			})
		case "/api/v1/projects":
			_ = json.NewEncoder(w).Encode([]client.Project{
				{ID: "proj", Name: "Alpha", Slug: "alpha"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterProjectCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", UserID: "user-1", Content: "/project create New Workspace"})
	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", UserID: "user-1", Content: "/project delete alpha"})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "Admin role required") {
		t.Fatalf("create reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "Admin role required") {
		t.Fatalf("delete reply = %q", platform.replies[1])
	}
}

func TestProjectCommand_CreateAndDeleteForLeadRole(t *testing.T) {
	requests := make([]string, 0, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/projects/proj/members" || r.URL.Path == "/api/v1/projects/proj-created/members"):
			_ = json.NewEncoder(w).Encode([]client.Member{
				{ID: "member-1", Name: "Lead", Type: "human", Role: "lead", Status: "active", IsActive: true, IMUserID: "user-1"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/projects":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create body: %v", err)
			}
			if body["name"] != "Release Ops" || body["slug"] != "release-ops" {
				t.Fatalf("create body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(client.Project{
				ID:   "proj-created",
				Name: "Release Ops",
				Slug: "release-ops",
				Settings: client.ProjectSettings{
					CodingAgent: client.CodingAgentSelection{Runtime: "codex", Provider: "openai", Model: "gpt-5-codex"},
				},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects":
			_ = json.NewEncoder(w).Encode([]client.Project{
				{ID: "proj-created", Name: "Release Ops", Slug: "release-ops"},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/projects/proj-created":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "project deleted"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterProjectCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", UserID: "user-1", Content: "/project create Release Ops"})
	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", UserID: "user-1", Content: "/project delete release-ops"})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "已创建项目: Release Ops (release-ops)") {
		t.Fatalf("create reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "已删除项目: Release Ops (release-ops)") {
		t.Fatalf("delete reply = %q", platform.replies[1])
	}
	if got := strings.Join(requests, ","); !strings.Contains(got, "POST /api/v1/projects") || !strings.Contains(got, "DELETE /api/v1/projects/proj-created") {
		t.Fatalf("requests = %v", requests)
	}
}

func TestProjectCommand_InfoAndMembers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/projects":
			_ = json.NewEncoder(w).Encode([]client.Project{
				{
					ID:   "proj-1",
					Name: "Alpha",
					Slug: "alpha",
					Settings: client.ProjectSettings{
						CodingAgent: client.CodingAgentSelection{Runtime: "codex", Provider: "openai", Model: "gpt-5-codex"},
					},
				},
				{
					ID:   "proj-2",
					Name: "Beta",
					Slug: "beta",
					Settings: client.ProjectSettings{
						CodingAgent: client.CodingAgentSelection{Runtime: "claude_code", Provider: "anthropic", Model: "claude-sonnet-4-5"},
					},
				},
			})
		case "/api/v1/projects/proj-2":
			_ = json.NewEncoder(w).Encode(client.Project{
				ID:   "proj-2",
				Name: "Beta",
				Slug: "beta",
				Settings: client.ProjectSettings{
					CodingAgent: client.CodingAgentSelection{Runtime: "claude_code", Provider: "anthropic", Model: "claude-sonnet-4-5"},
				},
			})
		case "/api/v1/projects/proj-2/members":
			_ = json.NewEncoder(w).Encode([]client.Member{
				{ID: "member-1", Name: "Alice", Type: "human", Role: "lead", Status: "active", IsActive: true},
				{ID: "member-2", Name: "Codex Worker", Type: "agent", Role: "developer", Status: "active", IsActive: true},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj-2", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterProjectCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/project info"})
	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/project members beta"})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "项目: Beta (beta)") {
		t.Fatalf("info reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "Alice") || !strings.Contains(platform.replies[1], "Codex Worker") {
		t.Fatalf("members reply = %q", platform.replies[1])
	}
}

func TestProjectCommand_RenameProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj/members":
			_ = json.NewEncoder(w).Encode([]client.Member{
				{ID: "member-1", Name: "Lead", Type: "human", Role: "lead", Status: "active", IsActive: true, IMUserID: "user-1"},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects":
			_ = json.NewEncoder(w).Encode([]client.Project{
				{ID: "proj", Name: "Alpha", Slug: "alpha"},
			})
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/projects/proj":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode update body: %v", err)
			}
			if body["name"] != "Alpha Prime" {
				t.Fatalf("body = %+v", body)
			}
			_ = json.NewEncoder(w).Encode(client.Project{
				ID:   "proj",
				Name: "Alpha Prime",
				Slug: "alpha",
				Settings: client.ProjectSettings{
					CodingAgent: client.CodingAgentSelection{Runtime: "codex", Provider: "openai", Model: "gpt-5-codex"},
				},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterProjectCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", UserID: "user-1", Content: "/project rename alpha Alpha Prime"})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "已重命名项目: Alpha Prime (alpha)") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
}
