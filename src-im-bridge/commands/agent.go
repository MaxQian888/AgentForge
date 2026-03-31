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
			_ = p.Reply(context.Background(), msg.ReplyCtx, commandUsage("/agent"))
			return
		}
		subCmd := canonicalSubcommand("/agent", parts[0])
		subArgs := ""
		if len(parts) > 1 {
			subArgs = parts[1]
		}

		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch subCmd {
		case "status":
			if strings.TrimSpace(subArgs) != "" {
				run, runErr := scopedClient.GetAgentRun(ctx, strings.TrimSpace(subArgs))
				if runErr != nil {
					_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Agent 状态失败: %v", runErr))
					return
				}
				_ = p.Reply(ctx, msg.ReplyCtx, formatAgentRunSummary(run))
				return
			}
			status, err := scopedClient.GetAgentPoolStatus(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Agent 状态失败: %v", err))
				return
			}
			bridgePool, bridgeErr := scopedClient.GetBridgePoolStatus(ctx)
			if bridgeErr != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("%s\nBridge pool unavailable: %v", formatAgentPoolStatus(status), bridgeErr))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentPoolStatusWithBridge(status, bridgePool))
		case "runtimes":
			runtimes, err := scopedClient.GetBridgeRuntimes(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge runtimes 失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgeRuntimeCatalog(runtimes))
		case "health":
			health, err := scopedClient.GetBridgeHealth(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge health 失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgeHealthStatus(health))
		case "spawn":
			if subArgs == "" {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/agent", subCmd))
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
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/agent", subCmd))
				return
			}
			logs, err := scopedClient.GetAgentLogs(ctx, subArgs)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取日志失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatAgentLogs(logs, subArgs))
		case "pause":
			runAgentLifecycleAction(ctx, p, msg, scopedClient, subCmd, subArgs)
		case "resume":
			runAgentLifecycleAction(ctx, p, msg, scopedClient, subCmd, subArgs)
		case "kill":
			runAgentLifecycleAction(ctx, p, msg, scopedClient, subCmd, subArgs)
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/agent"))
		}
	})
}

func runAgentLifecycleAction(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, action, runID string) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/agent", action))
		return
	}

	var (
		run *client.AgentRunSummary
		err error
	)
	switch action {
	case "pause":
		run, err = c.PauseAgentRun(ctx, runID)
	case "resume":
		run, err = c.ResumeAgentRun(ctx, runID)
	case "kill":
		run, err = c.KillAgentRun(ctx, runID)
	default:
		_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/agent"))
		return
	}
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("更新 Agent 状态失败: %v", err))
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, formatAgentRunSummary(run))
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

func formatAgentPoolStatus(status *client.PoolStatus) string {
	if status == nil {
		return "Agent 池状态不可用"
	}
	active := status.ActiveAgents
	if active == 0 {
		active = status.Active
	}
	max := status.MaxAgents
	if max == 0 {
		max = status.Max
	}
	return fmt.Sprintf(
		"Agent 池状态: %d/%d 活跃，可用 %d，排队 %d，可恢复 %d",
		active,
		max,
		status.Available,
		status.Queued,
		status.PausedResumable,
	)
}

func formatAgentPoolStatusWithBridge(status *client.PoolStatus, bridgePool *client.BridgePoolStatus) string {
	base := formatAgentPoolStatus(status)
	if bridgePool == nil {
		return base
	}
	return fmt.Sprintf(
		"%s\nBridge pool: %d/%d active, warm %d/%d",
		base,
		bridgePool.Active,
		bridgePool.Max,
		bridgePool.WarmAvailable,
		bridgePool.WarmTotal,
	)
}

func formatBridgeRuntimeCatalog(catalog *client.BridgeRuntimeCatalog) string {
	if catalog == nil {
		return "Bridge runtimes unavailable"
	}
	lines := []string{fmt.Sprintf("Bridge runtimes (default=%s):", catalog.DefaultRuntime)}
	for _, runtime := range catalog.Runtimes {
		lines = append(lines, fmt.Sprintf("- %s [%t] provider=%s model=%s", runtime.Key, runtime.Available, runtime.DefaultProvider, runtime.DefaultModel))
	}
	return strings.Join(lines, "\n")
}

func formatBridgeHealthStatus(health *client.BridgeHealthStatus) string {
	if health == nil {
		return "Bridge health unavailable"
	}
	return fmt.Sprintf(
		"Bridge health: %s | active=%d available=%d warm=%d",
		health.Status,
		health.Pool.Active,
		health.Pool.Available,
		health.Pool.Warm,
	)
}

func formatAgentRunSummary(run *client.AgentRunSummary) string {
	if run == nil {
		return "Agent 运行状态不可用"
	}
	parts := []string{
		fmt.Sprintf("Agent #%s", shortID(run.ID)),
		fmt.Sprintf("[%s] %s", run.Status, run.TaskTitle),
	}
	if strings.TrimSpace(run.Runtime) != "" {
		parts = append(parts, fmt.Sprintf("runtime=%s", run.Runtime))
	}
	if strings.TrimSpace(run.Provider) != "" {
		parts = append(parts, fmt.Sprintf("provider=%s", run.Provider))
	}
	if strings.TrimSpace(run.Model) != "" {
		parts = append(parts, fmt.Sprintf("model=%s", run.Model))
	}
	if run.CanResume {
		parts = append(parts, "可恢复")
	}
	return strings.Join(parts, " | ")
}
