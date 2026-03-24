package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

const reviewUsage = "用法: /review <pr-url> 或 /review status <id>"

// RegisterReviewCommands registers /review sub-commands on the engine.
func RegisterReviewCommands(engine *core.Engine, apiClient *client.AgentForgeClient) {
	engine.RegisterCommand("/review", func(p core.Platform, msg *core.Message, args string) {
		trimmed := strings.TrimSpace(args)
		if trimmed == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, reviewUsage)
			return
		}

		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)

		parts := strings.SplitN(trimmed, " ", 2)
		if parts[0] == "status" {
			subArgs := ""
			if len(parts) > 1 {
				subArgs = strings.TrimSpace(parts[1])
			}
			handleReviewStatus(ctx, p, msg, scopedClient, subArgs)
			return
		}

		// Otherwise treat the entire args as a PR URL.
		handleReviewTrigger(ctx, p, msg, scopedClient, trimmed)
	})
}

func handleReviewTrigger(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, prURL string) {
	_ = p.Reply(ctx, msg.ReplyCtx, "正在触发代码审查，请稍候...")

	review, err := c.TriggerReview(ctx, prURL)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("触发审查失败: %v", err))
		return
	}
	if msg.ReplyTarget != nil {
		_ = c.BindActionContext(ctx, client.IMActionBinding{
			Platform:    msg.Platform,
			TaskID:      review.TaskID,
			ReviewID:    review.ID,
			ReplyTarget: msg.ReplyTarget,
		})
	}
	if cs, ok := p.(core.CardSender); ok {
		card := buildReviewCard(review)
		_ = cs.ReplyCard(ctx, msg.ReplyCtx, card)
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已创建代码审查 #%s\nPR: %s\n状态: %s",
		shortID(review.ID), review.PRURL, review.Status))
}

func handleReviewStatus(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, reviewID string) {
	if reviewID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /review status <review-id>")
		return
	}
	review, err := c.GetReview(ctx, reviewID)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取审查失败: %v", err))
		return
	}
	if cs, ok := p.(core.CardSender); ok {
		card := buildReviewCard(review)
		_ = cs.ReplyCard(ctx, msg.ReplyCtx, card)
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("代码审查 #%s\nPR: %s\n状态: %s\n风险等级: %s\n摘要: %s\n建议: %s",
		shortID(review.ID), review.PRURL, review.Status, review.RiskLevel, review.Summary, review.Recommendation))
}

func buildReviewCard(review *client.Review) *core.Card {
	card := core.NewCard().
		SetTitle(fmt.Sprintf("代码审查 #%s", shortID(review.ID))).
		AddField("PR", review.PRURL).
		AddField("状态", review.Status).
		AddField("风险等级", review.RiskLevel)
	if review.Summary != "" {
		card.AddField("摘要", review.Summary)
	}
	if review.Recommendation != "" {
		card.AddField("建议", review.Recommendation)
	}
	if review.CostUSD > 0 {
		card.AddField("费用", fmt.Sprintf("$%.2f", review.CostUSD))
	}
	card.AddButton("查看详情", "link:/reviews/"+review.ID)
	return card
}
