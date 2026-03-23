package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterTaskCommands registers /task sub-commands on the engine.
func RegisterTaskCommands(engine *core.Engine, apiClient *client.AgentForgeClient) {
	engine.RegisterCommand("/task", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, "用法: /task create|list|status|assign")
			return
		}
		subCmd := parts[0]
		subArgs := ""
		if len(parts) > 1 {
			subArgs = parts[1]
		}

		ctx := context.Background()
		switch subCmd {
		case "create":
			handleTaskCreate(ctx, p, msg, apiClient, subArgs)
		case "list":
			handleTaskList(ctx, p, msg, apiClient, subArgs)
		case "status":
			handleTaskStatus(ctx, p, msg, apiClient, subArgs)
		case "assign":
			handleTaskAssign(ctx, p, msg, apiClient, subArgs)
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, "用法: /task create|list|status|assign")
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
	task, err := c.AssignTask(ctx, parts[0], parts[1])
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("分配失败: %v", err))
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已将任务 #%s 分配给 %s", shortID(task.ID), task.AssigneeName))
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
