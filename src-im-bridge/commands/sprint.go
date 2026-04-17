package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

var sprintUsage = commandUsage("/sprint")

// RegisterSprintCommands registers /sprint sub-commands on the engine.
func RegisterSprintCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/sprint", func(p core.Platform, msg *core.Message, args string) {
		trimmed := strings.TrimSpace(args)
		if trimmed == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, sprintUsage)
			return
		}

		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform)

		switch trimmed {
		case "status":
			handleSprintStatus(ctx, p, msg, scopedClient)
		case "burndown":
			handleSprintBurndown(ctx, p, msg, scopedClient)
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, sprintUsage)
		}
	})
}

func handleSprintStatus(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient) {
	sprint, err := c.GetCurrentSprint(ctx)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Sprint 失败: %v", err))
		return
	}
	if sm := buildSprintStructuredMessage(sprint); sm != nil {
		if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
			return
		}
	}
	if cs, ok := p.(core.CardSender); ok {
		card := buildSprintCard(sprint)
		_ = cs.ReplyCard(ctx, msg.ReplyCtx, card)
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("Sprint: %s\n日期: %s ~ %s\n预算: $%.2f / $%.2f\n状态: %s",
		sprint.Name, sprint.StartDate, sprint.EndDate, sprint.SpentUsd, sprint.TotalBudgetUsd, sprint.Status))
}

func buildSprintStructuredMessage(sprint *client.Sprint) *core.StructuredMessage {
	if sprint == nil {
		return nil
	}
	return &core.StructuredMessage{
		Title: fmt.Sprintf("Sprint: %s", sprint.Name),
		Sections: []core.StructuredSection{
			{
				Type: core.StructuredSectionTypeFields,
				FieldsSection: &core.FieldsSection{
					Fields: []core.StructuredField{
						{Label: "名称", Value: sprint.Name},
						{Label: "状态", Value: sprint.Status},
						{Label: "起止日期", Value: fmt.Sprintf("%s ~ %s", sprint.StartDate, sprint.EndDate)},
						{Label: "预算", Value: fmt.Sprintf("$%.2f / $%.2f", sprint.SpentUsd, sprint.TotalBudgetUsd)},
					},
				},
			},
			{
				Type: core.StructuredSectionTypeActions,
				ActionsSection: &core.ActionsSection{
					Actions: []core.StructuredAction{
						{ID: "act:cmd:/sprint burndown", Label: "查看燃尽图", Style: core.ActionStylePrimary},
						{ID: "act:cmd:/task list", Label: "任务列表", Style: core.ActionStyleDefault},
					},
				},
			},
		},
	}
}

func handleSprintBurndown(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient) {
	sprint, err := c.GetCurrentSprint(ctx)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Sprint 失败: %v", err))
		return
	}
	metrics, err := c.GetSprintBurndown(ctx, sprint.ID)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取燃尽图失败: %v", err))
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, buildBurndownText(metrics))
}

func buildSprintCard(sprint *client.Sprint) *core.Card {
	card := core.NewCard().
		SetTitle(fmt.Sprintf("Sprint: %s", sprint.Name)).
		AddField("名称", sprint.Name).
		AddField("起止日期", fmt.Sprintf("%s ~ %s", sprint.StartDate, sprint.EndDate)).
		AddField("预算", fmt.Sprintf("$%.2f / $%.2f", sprint.SpentUsd, sprint.TotalBudgetUsd)).
		AddField("状态", sprint.Status)
	return card
}

func buildBurndownText(metrics *client.SprintMetrics) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Sprint: %s (%s ~ %s)\n",
		metrics.Sprint.Name, metrics.Sprint.StartDate, metrics.Sprint.EndDate))
	sb.WriteString(fmt.Sprintf("进度: %d/%d 任务 (%.0f%%)\n",
		metrics.CompletedTasks, metrics.PlannedTasks, metrics.CompletionRate*100))
	sb.WriteString(fmt.Sprintf("速率: %.1f 任务/周\n", metrics.VelocityPerWeek))

	if len(metrics.Burndown) > 0 {
		// Find max remaining for scaling the chart.
		maxRemaining := 0
		for _, bp := range metrics.Burndown {
			if bp.RemainingTasks > maxRemaining {
				maxRemaining = bp.RemainingTasks
			}
		}

		sb.WriteString("\n燃尽图:\n")
		chartWidth := 20
		for _, bp := range metrics.Burndown {
			barLen := 0
			if maxRemaining > 0 {
				barLen = bp.RemainingTasks * chartWidth / maxRemaining
			}
			bar := strings.Repeat("█", barLen) + strings.Repeat("░", chartWidth-barLen)
			// Show short date (last 5 chars, e.g. "03-24").
			dateLabel := bp.Date
			if len(dateLabel) > 5 {
				dateLabel = dateLabel[len(dateLabel)-5:]
			}
			sb.WriteString(fmt.Sprintf("  %s |%s| %d\n", dateLabel, bar, bp.RemainingTasks))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
