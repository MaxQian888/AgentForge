package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

const taskUsage = "用法: /task create|list|status|assign|decompose"

// RegisterTaskCommands registers /task sub-commands on the engine.
func RegisterTaskCommands(engine *core.Engine, apiClient *client.AgentForgeClient) {
	engine.RegisterCommand("/task", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, taskUsage)
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
		case "create":
			handleTaskCreate(ctx, p, msg, scopedClient, subArgs)
		case "list":
			handleTaskList(ctx, p, msg, scopedClient, subArgs)
		case "status":
			handleTaskStatus(ctx, p, msg, scopedClient, subArgs)
		case "assign":
			handleTaskAssign(ctx, p, msg, scopedClient, subArgs)
		case "decompose":
			handleTaskDecompose(ctx, p, msg, scopedClient, subArgs)
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, taskUsage)
		}
	})
}

func handleTaskCreate(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, title string) {
	if title == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task create <标题>")
		return
	}
	task, err := c.CreateTask(ctx, title, "")
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("创建失败: %v", err))
		return
	}
	if cs, ok := p.(core.CardSender); ok {
		card := buildTaskCard(task)
		_ = cs.ReplyCard(ctx, msg.ReplyCtx, card)
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已创建任务 #%s: %s", shortID(task.ID), task.Title))
}

func handleTaskList(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, filter string) {
	tasks, err := c.ListTasks(ctx, filter)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取任务列表失败: %v", err))
		return
	}
	if len(tasks) == 0 {
		_ = p.Reply(ctx, msg.ReplyCtx, "暂无任务")
		return
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务列表 (%d):\n", len(tasks)))
	for _, t := range tasks {
		sb.WriteString(fmt.Sprintf("  #%s [%s] %s", shortID(t.ID), t.Status, t.Title))
		if t.AssigneeName != "" {
			sb.WriteString(fmt.Sprintf(" (@%s)", t.AssigneeName))
		}
		sb.WriteString("\n")
	}
	_ = p.Reply(ctx, msg.ReplyCtx, sb.String())
}

func handleTaskStatus(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, taskID string) {
	if taskID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task status <task-id>")
		return
	}
	task, err := c.GetTask(ctx, taskID)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取任务失败: %v", err))
		return
	}
	if cs, ok := p.(core.CardSender); ok {
		card := buildTaskCard(task)
		_ = cs.ReplyCard(ctx, msg.ReplyCtx, card)
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("#%s [%s] %s\n优先级: %s\n负责人: %s\n花费: $%.2f / $%.2f",
		shortID(task.ID), task.Status, task.Title, task.Priority, task.AssigneeName, task.SpentUsd, task.BudgetUsd))
}

func handleTaskAssign(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
	if len(parts) < 2 {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task assign <task-id> <assignee>")
		return
	}
	member, err := resolveProjectMember(ctx, c, parts[1])
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("分配失败: %v", err))
		return
	}

	result, err := c.AssignTask(ctx, parts[0], member.ID, member.Type)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("分配失败: %v", err))
		return
	}

	_ = p.Reply(ctx, msg.ReplyCtx, formatTaskDispatchReply(result, member.Name))
}

func handleTaskDecompose(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, taskID string) {
	if strings.TrimSpace(taskID) == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task decompose <task-id>")
		return
	}

	_ = p.Reply(ctx, msg.ReplyCtx, "正在分解任务，请稍候...")

	result, err := c.DecomposeTask(ctx, strings.TrimSpace(taskID))
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("任务分解失败: %v\n未创建任何子任务，请稍后重试。", err))
		return
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("任务分解完成: #%s %s\n", shortID(result.ParentTask.ID), result.ParentTask.Title))
	sb.WriteString(fmt.Sprintf("摘要: %s\n", result.Summary))
	sb.WriteString("子任务:\n")
	for i, subtask := range result.Subtasks {
		sb.WriteString(fmt.Sprintf("%d. #%s [%s/%s] %s\n",
			i+1, shortID(subtask.ID), subtask.Status, subtask.Priority, subtask.Title))
	}

	_ = p.Reply(ctx, msg.ReplyCtx, strings.TrimRight(sb.String(), "\n"))
}

func buildTaskCard(task *client.Task) *core.Card {
	card := core.NewCard().
		SetTitle(fmt.Sprintf("任务 #%s", shortID(task.ID))).
		AddField("标题", task.Title).
		AddField("状态", task.Status).
		AddField("优先级", task.Priority)
	if task.AssigneeName != "" {
		card.AddField("负责人", task.AssigneeName)
	}
	if task.BudgetUsd > 0 {
		card.AddField("预算", fmt.Sprintf("$%.2f / $%.2f", task.SpentUsd, task.BudgetUsd))
	}
	card.AddPrimaryButton("分配给 Agent", "act:assign-agent:"+task.ID)
	card.AddButton("查看详情", "link:/tasks/"+task.ID)
	return card
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func resolveProjectMember(ctx context.Context, c *client.AgentForgeClient, assignee string) (*client.Member, error) {
	members, err := c.ListProjectMembers(ctx)
	if err != nil {
		return nil, err
	}

	query := strings.TrimSpace(assignee)
	lowerQuery := strings.ToLower(query)
	for i := range members {
		member := &members[i]
		if member.ID == query || strings.EqualFold(member.Name, query) || strings.ToLower(member.Name) == lowerQuery {
			return member, nil
		}
	}

	return nil, fmt.Errorf("未找到成员 %q", assignee)
}

func formatTaskDispatchReply(result *client.TaskDispatchResponse, assigneeName string) string {
	taskID := shortID(result.Task.ID)
	switch result.Dispatch.Status {
	case "started":
		if result.Dispatch.Run != nil {
			return fmt.Sprintf("已将任务 #%s 分配给 %s，并启动 Agent #%s", taskID, assigneeName, shortID(result.Dispatch.Run.ID))
		}
		return fmt.Sprintf("已将任务 #%s 分配给 %s，并启动 Agent", taskID, assigneeName)
	case "blocked":
		if reason := strings.TrimSpace(result.Dispatch.Reason); reason != "" {
			return fmt.Sprintf("已将任务 #%s 分配给 %s，但未启动 Agent：%s", taskID, assigneeName, reason)
		}
		return fmt.Sprintf("已将任务 #%s 分配给 %s，但未启动 Agent", taskID, assigneeName)
	default:
		return fmt.Sprintf("已将任务 #%s 分配给 %s", taskID, assigneeName)
	}
}
