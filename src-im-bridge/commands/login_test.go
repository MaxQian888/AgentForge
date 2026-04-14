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

func TestLoginCommand_StatusShowsRuntimeReadiness(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/bridge/runtimes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"default_runtime": "codex",
			"runtimes": []map[string]any{
				{
					"key":              "codex",
					"label":            "Codex",
					"default_provider": "openai",
					"default_model":    "gpt-5-codex",
					"available":        true,
				},
				{
					"key":              "claude_code",
					"label":            "Claude Code",
					"default_provider": "anthropic",
					"default_model":    "claude-sonnet-4-5",
					"available":        false,
					"diagnostics": []map[string]any{
						{"code": "missing_credentials", "message": "Missing required environment variable for runtime claude_code: ANTHROPIC_API_KEY", "blocking": true},
					},
				},
			},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterLoginCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/login"})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	reply := platform.replies[0]
	for _, want := range []string{"Runtime 登录/凭据状态", "codex [ready]", "claude_code [blocked]", "ANTHROPIC_API_KEY"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply = %q, want substring %q", reply, want)
		}
	}
}

func TestLoginCommand_RuntimeGuidanceIsReasonable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/bridge/runtimes" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"default_runtime": "codex",
			"runtimes": []map[string]any{
				{
					"key":              "codex",
					"label":            "Codex",
					"default_provider": "openai",
					"default_model":    "gpt-5-codex",
					"available":        false,
					"diagnostics": []map[string]any{
						{"code": "missing_credentials", "message": "Codex CLI authentication is unavailable", "blocking": true},
					},
				},
				{
					"key":              "claude_code",
					"label":            "Claude Code",
					"default_provider": "anthropic",
					"default_model":    "claude-sonnet-4-5",
					"available":        false,
					"diagnostics": []map[string]any{
						{"code": "missing_credentials", "message": "Missing required environment variable for runtime claude_code: ANTHROPIC_API_KEY", "blocking": true},
					},
				},
			},
		})
	}))
	defer server.Close()

	apiClient := client.NewAgentForgeClient(server.URL, "proj", "secret")
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterLoginCommands(engine, apiClient)

	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/login codex"})
	engine.HandleMessage(platform, &core.Message{Platform: "slack-stub", Content: "/login claude"})

	if len(platform.replies) != 2 {
		t.Fatalf("replies = %v", platform.replies)
	}
	if !strings.Contains(platform.replies[0], "codex login") {
		t.Fatalf("codex reply = %q", platform.replies[0])
	}
	if !strings.Contains(platform.replies[1], "ANTHROPIC_API_KEY") {
		t.Fatalf("claude reply = %q", platform.replies[1])
	}
}
