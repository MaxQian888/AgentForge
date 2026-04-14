package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterLoginCommands registers /login sub-commands on the engine.
func RegisterLoginCommands(engine *core.Engine, apiClient *client.AgentForgeClient) {
	engine.RegisterCommand("/login", func(p core.Platform, msg *core.Message, args string) {
		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		parts := strings.Fields(strings.TrimSpace(args))
		if len(parts) == 0 {
			parts = []string{"status"}
		}

		catalog, err := scopedClient.GetBridgeRuntimes(ctx)
		if err != nil {
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取登录状态失败（%s）: %v", describeBridgeFailure(err), err))
			return
		}

		target := normalizeLoginTarget(parts[0])
		if target == "" || target == "status" {
			if sm := buildLoginStatusStructuredMessage(catalog); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatLoginStatus(catalog))
			return
		}

		runtime := findBridgeRuntime(catalog, target)
		if runtime == nil {
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("不支持的登录目标 %q。用法: /login status|codex|claude|opencode|cursor|gemini|qoder|iflow", parts[0]))
			return
		}
		_ = p.Reply(ctx, msg.ReplyCtx, formatRuntimeLoginGuidance(runtime))
	})
}

func normalizeLoginTarget(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "status", "list":
		return "status"
	case "claude", "claude_code", "claudecode":
		return "claude_code"
	case "codex":
		return "codex"
	case "opencode":
		return "opencode"
	case "cursor":
		return "cursor"
	case "gemini":
		return "gemini"
	case "qoder":
		return "qoder"
	case "iflow":
		return "iflow"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func findBridgeRuntime(catalog *client.BridgeRuntimeCatalog, runtimeKey string) *client.BridgeRuntimeEntry {
	if catalog == nil {
		return nil
	}
	for i := range catalog.Runtimes {
		if catalog.Runtimes[i].Key == runtimeKey {
			return &catalog.Runtimes[i]
		}
	}
	return nil
}

func buildLoginStatusStructuredMessage(catalog *client.BridgeRuntimeCatalog) *core.StructuredMessage {
	if catalog == nil || len(catalog.Runtimes) == 0 {
		return nil
	}
	fields := make([]core.StructuredField, 0, len(catalog.Runtimes))
	for _, runtime := range catalog.Runtimes {
		status := "ready"
		if !runtime.Available {
			status = "blocked"
		}
		label := fmt.Sprintf("%s [%s]", runtime.Key, status)
		value := fmt.Sprintf("%s / %s", runtime.DefaultProvider, runtime.DefaultModel)
		if !runtime.Available {
			value = firstRuntimeDiagnostic(runtime)
		}
		fields = append(fields, core.StructuredField{Label: label, Value: value})
	}
	return &core.StructuredMessage{
		Title: "Runtime 登录状态",
		Sections: []core.StructuredSection{
			{
				Type:          core.StructuredSectionTypeFields,
				FieldsSection: &core.FieldsSection{Fields: fields},
			},
			{
				Type:           core.StructuredSectionTypeContext,
				ContextSection: &core.ContextSection{Elements: []string{"使用 /login <runtime> 查看对应登录指引"}},
			},
		},
	}
}

func formatLoginStatus(catalog *client.BridgeRuntimeCatalog) string {
	if catalog == nil {
		return "登录状态不可用"
	}
	lines := []string{"Runtime 登录/凭据状态:"}
	for _, runtime := range catalog.Runtimes {
		status := "ready"
		detail := "可直接使用"
		if !runtime.Available {
			status = "blocked"
			detail = firstRuntimeDiagnostic(runtime)
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] provider=%s model=%s", runtime.Key, status, runtime.DefaultProvider, runtime.DefaultModel))
		lines = append(lines, fmt.Sprintf("  %s", detail))
	}
	lines = append(lines, "使用 /login <runtime> 查看对应登录/配置指引。")
	return strings.Join(lines, "\n")
}

func formatRuntimeLoginGuidance(runtime *client.BridgeRuntimeEntry) string {
	if runtime == nil {
		return "登录状态不可用"
	}
	if runtime.Available {
		return fmt.Sprintf("%s 已就绪。\nprovider=%s\nmodel=%s\n现在可以直接使用 /agent run --runtime %s ... 或 @%s ...", runtime.Key, runtime.DefaultProvider, runtime.DefaultModel, runtime.Key, runtimeMentionAlias(runtime.Key))
	}

	detail := firstRuntimeDiagnostic(*runtime)
	switch runtime.Key {
	case "codex":
		return strings.Join([]string{
			"Codex 当前未就绪。",
			detail,
			"请在宿主机执行: codex login",
			"然后用 /login codex 或 /login status 复查。",
		}, "\n")
	case "claude_code":
		return strings.Join([]string{
			"Claude Code 当前未就绪。",
			detail,
			"Claude Code 不是交互式 login，而是凭据配置型运行时；请在 bridge 运行环境中配置 ANTHROPIC_API_KEY。",
		}, "\n")
	case "opencode":
		return strings.Join([]string{
			"OpenCode 当前未就绪。",
			detail,
			"先配置 OPENCODE_SERVER_URL；如果 server 已就绪，再补 provider auth。",
		}, "\n")
	case "cursor":
		return strings.Join([]string{
			"Cursor 当前未就绪。",
			detail,
			"请先在宿主机安装 Cursor CLI，并确认 `agent` 命令可用。",
		}, "\n")
	case "gemini":
		return strings.Join([]string{
			"Gemini 当前未就绪。",
			detail,
			"请先在宿主机安装 Gemini CLI，并补齐对应凭据/环境。",
		}, "\n")
	case "qoder":
		return strings.Join([]string{
			"Qoder 当前未就绪。",
			detail,
			"请先在宿主机安装 Qoder CLI。",
		}, "\n")
	case "iflow":
		return strings.Join([]string{
			"iFlow 当前未就绪。",
			detail,
			"请先在宿主机安装 iFlow CLI；如果在迁移窗口内，建议优先切到 qoder / codex。",
		}, "\n")
	default:
		return fmt.Sprintf("%s 当前未就绪。\n%s", runtime.Key, detail)
	}
}

func firstRuntimeDiagnostic(runtime client.BridgeRuntimeEntry) string {
	if len(runtime.Diagnostics) == 0 {
		return "运行时未就绪"
	}
	return strings.TrimSpace(runtime.Diagnostics[0].Message)
}

func runtimeMentionAlias(runtimeKey string) string {
	switch runtimeKey {
	case "claude_code":
		return "claude"
	default:
		return runtimeKey
	}
}
