package commands

import (
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestHelpCommand_RepliesWithHelpText(t *testing.T) {
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterHelpCommand(engine)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/help",
	})

	if len(platform.replies) != 1 || platform.replies[0] != helpText {
		t.Fatalf("replies = %v", platform.replies)
	}
}

func TestHelpCommand_UsesCanonicalCatalogAndIncludesOperatorCommands(t *testing.T) {
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterHelpCommand(engine)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/help",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	reply := platform.replies[0]
	for _, want := range []string{"/agent status", "/agent config", "/login status", "/project list", "/queue list", "/team list", "/memory search"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply = %q, want substring %q", reply, want)
		}
	}
	if strings.Contains(reply, "/agent list") {
		t.Fatalf("reply = %q, want canonical command name instead of legacy alias", reply)
	}
}

func TestHelpCommand_ShowsReadableChineseForToolsCommands(t *testing.T) {
	platform := &taskTestPlatform{}
	engine := core.NewEngine(platform)
	RegisterHelpCommand(engine)

	engine.HandleMessage(platform, &core.Message{
		Platform: "slack-stub",
		Content:  "/help",
	})

	if len(platform.replies) != 1 {
		t.Fatalf("replies = %v", platform.replies)
	}
	reply := platform.replies[0]
	for _, want := range []string{
		"/tools list              — 查看 Bridge tools",
		"/tools install <manifest-url> — 安装 Bridge tool 插件",
		"/tools uninstall <plugin-id> — 卸载 Bridge tool 插件",
		"/tools restart <plugin-id> — 重启 Bridge tool 插件",
	} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply = %q, want substring %q", reply, want)
		}
	}
	if strings.Contains(reply, "鏌ョ湅") || strings.Contains(reply, "瀹夎") {
		t.Fatalf("reply = %q, want readable Chinese instead of mojibake", reply)
	}
}

func TestSuggestCommandFromCatalogForPauseIntent(t *testing.T) {
	got := suggestCommandFromCatalog("@AgentForge 暂停 run-123")
	if got != "/agent pause run-123" {
		t.Fatalf("suggestion = %q, want /agent pause run-123", got)
	}
}

func TestIntentCatalogRanksTopCommandCandidates(t *testing.T) {
	ranked := RankIntentCandidates("@AgentForge 看一下 sprint 和任务")
	if len(ranked) < 3 {
		t.Fatalf("ranked = %+v", ranked)
	}
	if ranked[0].Command == "" {
		t.Fatalf("ranked[0] = %+v", ranked[0])
	}
}

func TestResolveDirectRuntimeMention(t *testing.T) {
	got := ResolveDirectRuntimeMention("@claude 帮我总结这个任务")
	want := "/agent run --runtime claude_code 帮我总结这个任务"
	if got != want {
		t.Fatalf("resolved = %q, want %q", got, want)
	}

	got = ResolveDirectRuntimeMention("@codex 帮我总结这个任务")
	want = "/agent run --runtime codex 帮我总结这个任务"
	if got != want {
		t.Fatalf("resolved codex = %q, want %q", got, want)
	}
}
