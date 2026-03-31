package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterMemoryCommands registers /memory sub-commands on the engine.
func RegisterMemoryCommands(engine *core.Engine, apiClient *client.AgentForgeClient) {
	engine.RegisterCommand("/memory", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, commandUsage("/memory"))
			return
		}

		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch canonicalSubcommand("/memory", parts[0]) {
		case "search":
			if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/memory", "search"))
				return
			}
			results, err := scopedClient.SearchProjectMemory(ctx, parts[1], 5)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("搜索记忆失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatMemorySearchResults(results))
		case "note":
			if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/memory", "note"))
				return
			}
			entry, err := scopedClient.StoreProjectMemoryNote(ctx, "operator-note", parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("记录记忆失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已记录记忆 %s [%s]", entry.ID, entry.Category))
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/memory"))
		}
	})
}

func formatMemorySearchResults(entries []client.MemoryEntry) string {
	if len(entries) == 0 {
		return "没有找到匹配的项目记忆"
	}
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, fmt.Sprintf("项目记忆 (%d):", len(entries)))
	for _, entry := range entries {
		content := strings.TrimSpace(entry.Content)
		if len(content) > 80 {
			content = content[:80] + "..."
		}
		lines = append(lines, fmt.Sprintf("- %s [%s] %s", entry.Key, entry.Category, content))
	}
	return strings.Join(lines, "\n")
}
