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
			_ = p.Reply(context.Background(), msg.ReplyCtx, "用法: /agent list|spawn|run|logs <参数>")
			return
		}
		subCmd := parts[0]
		subArgs := ""
		if len(parts) > 1 {
			subArgs = parts[1]
		}

		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
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
			result, err := scopedClient.SpawnAgent(ctx, subArgs)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("启动 Agent 失败: %v", err))
				return
			}
			bindAgentDispatch(ctx, scopedClient, result, msg)
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentSpawnReply(result, subArgs))
		case "run":
			if subArgs == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, "用法: /agent run <任务描述>")
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, "正在创建任务并启动 Agent...")
			result, err := scopedClient.QuickAgentRun(ctx, subArgs)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("执行失败: %v", err))
				return
			}
			bindAgentDispatch(ctx, scopedClient, result, msg)
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentSpawnReply(result, result.Task.ID))
		case "logs":
			if subArgs == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, "用法: /agent logs <agent-run-id>")
				return
			}
			logs, err := scopedClient.GetAgentLogs(ctx, subArgs)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取日志失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentLogs(logs, subArgs))
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, "用法: /agent list|spawn|run|logs <参数>")
		}
	})
}

func bindAgentDispatch(ctx context.Context, apiClient *client.AgentForgeClient, result *client.TaskDispatchResponse, msg *core.Message) {
	if apiClient == nil || result == nil || msg == nil || msg.ReplyTarget == nil {
		return
	}
	binding := client.IMActionBinding{
		Platform:    msg.Platform,
		ProjectID:   result.Task.ProjectID,
		TaskID:      result.Task.ID,
		ReplyTarget: msg.ReplyTarget,
	}
	if result.Dispatch.Run != nil {
		binding.RunID = result.Dispatch.Run.ID
	}
	_ = apiClient.BindActionContext(ctx, binding)
}

func formatAgentLogs(logs []client.AgentLogEntry, runID string) string {
	if len(logs) == 0 {
		return fmt.Sprintf("Agent #%s 暂无日志", shortID(runID))
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Agent #%s 最近日志:\n", shortID(runID)))
	limit := len(logs)
	if limit > 15 {
		limit = 15
	}
	for _, entry := range logs[:limit] {
		sb.WriteString(fmt.Sprintf("  [%s] %s: %s\n", entry.Timestamp, entry.Type, entry.Content))
	}
	if len(logs) > 15 {
		sb.WriteString(fmt.Sprintf("  ... 还有 %d 条日志\n", len(logs)-15))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func formatAgentSpawnReply(result *client.TaskDispatchResponse, requestedTaskID string) string {
	switch result.Dispatch.Status {
	case "started":
		if result.Dispatch.Run != nil {
			return fmt.Sprintf("已启动 Agent #%s 执行任务 %s",
				shortID(result.Dispatch.Run.ID), shortID(result.Dispatch.Run.TaskID))
		}
		return fmt.Sprintf("已启动 Agent 执行任务 %s", shortID(requestedTaskID))
	case "blocked":
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("未启动 Agent：%s", reason)
		}
		return "未启动 Agent"
	default:
		return fmt.Sprintf("任务 %s 当前未启动 Agent", shortID(requestedTaskID))
	}
}
