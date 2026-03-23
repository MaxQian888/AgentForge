package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterAgentCommands registers /agent sub-commands on the engine.
func RegisterAgentCommands(engine *core.Engine, apiClient *client.AgentForgeClient) {
	engine.RegisterCommand("/agent", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, "用法: /agent list|spawn <task-id>")
			return
		}
		subCmd := parts[0]
		subArgs := ""
		if len(parts) > 1 {
			subArgs = parts[1]
		}

		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform)
		switch subCmd {
		case "list":
			status, err := scopedClient.GetAgentPoolStatus(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Agent 状态失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("Agent 池状态: %d/%d 活跃",
				status.ActiveAgents, status.MaxAgents))
		case "spawn":
			if subArgs == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, "用法: /agent spawn <task-id>")
				return
			}
			run, err := scopedClient.SpawnAgent(ctx, subArgs)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("启动 Agent 失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已启动 Agent #%s 执行任务 %s",
				shortID(run.ID), shortID(run.TaskID)))
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, "用法: /agent list|spawn <task-id>")
		}
	})
}
