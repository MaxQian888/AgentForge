package commands

import (
	"context"
	"fmt"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterCostCommands registers the /cost command on the engine.
func RegisterCostCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/cost", func(p core.Platform, msg *core.Message, args string) {
		ctx := context.Background()
		stats, err := factory.For(msg.TenantID).WithSource(msg.Platform).GetCostStats(ctx)
		if err != nil {
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取费用统计失败: %v", err))
			return
		}

		sm := &core.StructuredMessage{
			Title: "费用统计",
			Sections: []core.StructuredSection{
				{
					Type: core.StructuredSectionTypeFields,
					FieldsSection: &core.FieldsSection{
						Fields: []core.StructuredField{
							{Label: "总费用", Value: fmt.Sprintf("$%.2f", stats.TotalUsd)},
							{Label: "预算", Value: fmt.Sprintf("$%.2f", stats.BudgetUsd)},
							{Label: "今日", Value: fmt.Sprintf("$%.2f", stats.DailyUsd)},
							{Label: "本周", Value: fmt.Sprintf("$%.2f", stats.WeeklyUsd)},
							{Label: "本月", Value: fmt.Sprintf("$%.2f", stats.MonthlyUsd)},
						},
					},
				},
			},
		}
		if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
			return
		}
		if cs, ok := p.(core.CardSender); ok {
			card := core.NewCard().
				SetTitle("费用统计").
				AddField("总费用", fmt.Sprintf("$%.2f", stats.TotalUsd)).
				AddField("预算", fmt.Sprintf("$%.2f", stats.BudgetUsd)).
				AddField("今日", fmt.Sprintf("$%.2f", stats.DailyUsd)).
				AddField("本周", fmt.Sprintf("$%.2f", stats.WeeklyUsd)).
				AddField("本月", fmt.Sprintf("$%.2f", stats.MonthlyUsd))
			_ = cs.ReplyCard(ctx, msg.ReplyCtx, card)
			return
		}

		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf(
			"费用统计:\n  总费用: $%.2f / $%.2f\n  今日: $%.2f\n  本周: $%.2f\n  本月: $%.2f",
			stats.TotalUsd, stats.BudgetUsd, stats.DailyUsd, stats.WeeklyUsd, stats.MonthlyUsd))
	})
}
