package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterTeamCommands registers /team sub-commands on the engine.
func RegisterTeamCommands(engine *core.Engine, apiClient *client.AgentForgeClient) {
	engine.RegisterCommand("/team", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.Fields(strings.TrimSpace(args))
		if len(parts) == 0 {
			_ = p.Reply(context.Background(), msg.ReplyCtx, commandUsage("/team"))
			return
		}

		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch canonicalSubcommand("/team", parts[0]) {
		case "list":
			members, err := scopedClient.ListProjectMembers(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取团队失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatTeamMembers(members))
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/team"))
		}
	})
}

func formatTeamMembers(members []client.Member) string {
	if len(members) == 0 {
		return "当前项目还没有成员"
	}
	lines := make([]string, 0, len(members)+1)
	lines = append(lines, fmt.Sprintf("项目成员 (%d):", len(members)))
	for _, member := range members {
		lines = append(lines, fmt.Sprintf("- %s [%s/%s] role=%s",
			member.Name,
			member.Type,
			member.Status,
			member.Role,
		))
	}
	return strings.Join(lines, "\n")
}
