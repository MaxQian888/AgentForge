package commands

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

func TestToolsCommand_RequiresSubcommand(t *testing.T) {
	apiClient := client.NewAgentForgeClient("http://example.test", "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterToolsCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/tools",
	})

	if len(platform.replies) != 1 || platform.replies[0] != commandUsage("/tools") {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestToolsCommand_UsageTextIsReadableChinese(t *testing.T) {
	if got, want := commandUsage("/tools"), "用法: /tools list|install|uninstall|restart <参数>"; got != want {
		t.Fatalf("commandUsage(/tools) = %q, want %q", got, want)
	}
}

func TestToolsCommand_ListAndRestart(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/bridge/tools":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tools": []map[string]any{
					{"plugin_id": "web-search", "name": "search", "description": "Search repos"},
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bridge/tools/web-search/restart":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"metadata":        map[string]any{"id": "web-search", "name": "Web Search", "version": "1.0.0"},
				"lifecycle_state": "active",
				"restart_count":   1,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterToolsCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/tools list",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/tools restart web-search",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "web-search") || !strings.Contains(platform.replies[0], "search") {
		t.Fatalf("list reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "web-search") || !strings.Contains(platform.replies[1], "active") {
		t.Fatalf("restart reply = %q", platform.replies[1])
	}
}

func TestToolsCommand_InstallAndUninstallRequireAdminRole(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj/members":
			_ = json.NewEncoder(w).Encode([]client.Member{
				{ID: "member-1", Name: "Alice", Type: "human", Role: "contributor", Status: "active", IsActive: true, IMUserID: "user-1"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterToolsCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		UserID:   "user-1",
		Content:  "/tools install https://registry.example.com/web-search.yaml",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		UserID:   "user-1",
		Content:  "/tools uninstall web-search",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "Admin role required") {
		t.Fatalf("install reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "Admin role required") {
		t.Fatalf("uninstall reply = %q", platform.replies[1])
	}
}

func TestToolsCommand_InstallAndUninstallAllowLeadRole(t *testing.T) {
	calls := make([]string, 0, 3)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj/members":
			_ = json.NewEncoder(w).Encode([]client.Member{
				{ID: "member-1", Name: "Alice", Type: "human", Role: "lead", Status: "active", IsActive: true, IMUserID: "user-1"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bridge/tools/install":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"metadata":        map[string]any{"id": "web-search", "name": "Web Search", "version": "1.0.0"},
				"lifecycle_state": "active",
				"restart_count":   0,
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/bridge/tools/uninstall":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"metadata":        map[string]any{"id": "web-search", "name": "Web Search", "version": "1.0.0"},
				"lifecycle_state": "disabled",
				"restart_count":   0,
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterToolsCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		UserID:   "user-1",
		Content:  "/tools install https://registry.example.com/web-search.yaml",
	})
	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		UserID:   "user-1",
		Content:  "/tools uninstall web-search",
	})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "web-search") || !strings.Contains(platform.replies[0], "active") {
		t.Fatalf("install reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "web-search") || !strings.Contains(platform.replies[1], "disabled") {
		t.Fatalf("uninstall reply = %q", platform.replies[1])
	}
	if !strings.Contains(strings.Join(calls, ","), "/api/v1/bridge/tools/install") || !strings.Contains(strings.Join(calls, ","), "/api/v1/bridge/tools/uninstall") {
		t.Fatalf("calls = %v", calls)
	}
}

func TestToolsCommand_InstallRejectsManifestURLOutsideAllowlist(t *testing.T) {
	oldAllowlist := os.Getenv("BRIDGE_TOOL_MANIFEST_ALLOWLIST")
	if err := os.Setenv("BRIDGE_TOOL_MANIFEST_ALLOWLIST", "registry.example.com"); err != nil {
		t.Fatalf("Setenv() error = %v", err)
	}
	defer func() {
		_ = os.Setenv("BRIDGE_TOOL_MANIFEST_ALLOWLIST", oldAllowlist)
	}()

	calls := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/projects/proj/members":
			_ = json.NewEncoder(w).Encode([]client.Member{
				{ID: "member-1", Name: "Alice", Type: "human", Role: "lead", Status: "active", IsActive: true, IMUserID: "user-1"},
			})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterToolsCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		UserID:   "user-1",
		Content:  "/tools install https://untrusted.example.com/web-search.yaml",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "allowlist") {
		t.Fatalf("reply = %q", platform.replies[0])
	}
	if strings.Contains(strings.Join(calls, ","), "/api/v1/bridge/tools/install") {
		t.Fatalf("install should not be called, calls = %v", calls)
	}
}
