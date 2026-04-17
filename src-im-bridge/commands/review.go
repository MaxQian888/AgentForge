package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

var reviewUsage = commandUsage("/review")

// RegisterReviewCommands registers /review sub-commands on the engine.
func RegisterReviewCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/review", func(p core.Platform, msg *core.Message, args string) {
		trimmed := strings.TrimSpace(args)
		if trimmed == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, reviewUsage)
			return
		}

		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		parts := strings.Fields(trimmed)
		if len(parts) == 0 {
			_ = p.Reply(ctx, msg.ReplyCtx, reviewUsage)
			return
		}

		switch parts[0] {
		case "status":
			handleReviewStatus(ctx, p, msg, scopedClient, strings.TrimSpace(strings.TrimPrefix(trimmed, "status")))
		case "deep":
			handleReviewDeep(ctx, p, msg, scopedClient, strings.TrimSpace(strings.TrimPrefix(trimmed, "deep")))
		case "approve":
			handleReviewApprove(ctx, p, msg, scopedClient, strings.TrimSpace(strings.TrimPrefix(trimmed, "approve")))
		case "request-changes":
			handleReviewRequestChanges(ctx, p, msg, scopedClient, strings.TrimSpace(strings.TrimPrefix(trimmed, "request-changes")))
		case "approve-reaction":
			handleReviewReactionShortcut(ctx, p, msg, scopedClient, strings.TrimSpace(strings.TrimPrefix(trimmed, "approve-reaction")), "approve")
		case "reject-reaction":
			handleReviewReactionShortcut(ctx, p, msg, scopedClient, strings.TrimSpace(strings.TrimPrefix(trimmed, "reject-reaction")), "request-changes")
		default:
			handleReviewTrigger(ctx, p, msg, scopedClient, trimmed)
		}
	})
}

// handleReviewReactionShortcut binds a reaction-driven shortcut against a
// review. Once bound, the backend treats the configured unified emoji code on
// the bound message as approve / request-changes, so humans can approve by
// reacting with a thumbs up instead of typing the command.
// Usage: /review approve-reaction <review-id> [emoji-code=thumbs_up]
func handleReviewReactionShortcut(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string, outcome string) {
	fields := strings.Fields(strings.TrimSpace(args))
	if len(fields) == 0 {
		_ = p.Reply(ctx, msg.ReplyCtx, "Usage: /review "+outcome+"-reaction <review-id> [emoji-code]")
		return
	}
	reviewID := strings.TrimSpace(fields[0])
	code := "thumbs_up"
	if outcome == "request-changes" {
		code = "thumbs_down"
	}
	if len(fields) >= 2 && strings.TrimSpace(fields[1]) != "" {
		code = strings.TrimSpace(fields[1])
	}
	if err := c.BindReviewReactionShortcut(ctx, reviewID, outcome, code, msg.ReplyTarget); err != nil {
		replyError(ctx, p, msg.ReplyCtx, "绑定 reaction 快捷键失败", fmt.Sprintf("%v", err), "请确认 review id 有效")
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已绑定 reaction 快捷键：对本消息添加 %s 将会执行 %s (review=%s)", code, outcome, shortID(reviewID)))
}

func handleReviewTrigger(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, prURL string) {
	replyProcessing(ctx, p, msg.ReplyCtx, "正在触发代码审查...")

	review, err := c.TriggerReview(ctx, prURL)
	if err != nil {
		replyError(ctx, p, msg.ReplyCtx, "触发审查失败", fmt.Sprintf("%v", err), "请确认 PR URL 有效且项目已配置")
		return
	}
	bindReviewActionContext(ctx, c, msg, review)
	replyReview(ctx, p, msg, review, "已创建代码审查")
}

func handleReviewDeep(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	prURL := strings.TrimSpace(args)
	if prURL == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /review deep <pr-url>")
		return
	}

	replyProcessing(ctx, p, msg.ReplyCtx, "正在创建深度审查...")
	review, err := c.TriggerStandaloneDeepReview(ctx, prURL)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("创建深度审查失败: %v", err))
		return
	}
	bindReviewActionContext(ctx, c, msg, review)
	replyReview(ctx, p, msg, review, "已创建深度审查")
}

func handleReviewApprove(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	reviewID := strings.TrimSpace(args)
	if reviewID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /review approve <review-id>")
		return
	}

	review, err := c.ApproveReview(ctx, reviewID, "")
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("审批失败: %v", err))
		return
	}
	replyReview(ctx, p, msg, review, "审查已批准")
}

func handleReviewRequestChanges(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, args string) {
	reviewID, comment := parseReviewActionArgs(args)
	if reviewID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /review request-changes <review-id> [comment]")
		return
	}

	review, err := c.RequestChangesReview(ctx, reviewID, comment)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("请求修改失败: %v", err))
		return
	}
	replyReview(ctx, p, msg, review, "已提交修改请求")
}

func handleReviewStatus(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, reviewID string) {
	reviewID = strings.TrimSpace(reviewID)
	if reviewID == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, "用法: /review status <review-id>")
		return
	}
	review, err := c.GetReview(ctx, reviewID)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取审查失败: %v", err))
		return
	}
	replyReview(ctx, p, msg, review, "")
}

func parseReviewActionArgs(args string) (string, string) {
	trimmed := strings.TrimSpace(args)
	if trimmed == "" {
		return "", ""
	}
	parts := strings.SplitN(trimmed, " ", 2)
	reviewID := strings.TrimSpace(parts[0])
	if len(parts) == 1 {
		return reviewID, ""
	}
	return reviewID, strings.TrimSpace(parts[1])
}

func bindReviewActionContext(ctx context.Context, c *client.AgentForgeClient, msg *core.Message, review *client.Review) {
	if review == nil || msg.ReplyTarget == nil {
		return
	}
	_ = c.BindActionContext(ctx, client.IMActionBinding{
		Platform:    msg.Platform,
		TaskID:      review.TaskID,
		ReviewID:    review.ID,
		ReplyTarget: msg.ReplyTarget,
	})
}

func replyReview(ctx context.Context, p core.Platform, msg *core.Message, review *client.Review, title string) {
	if review == nil {
		_ = p.Reply(ctx, msg.ReplyCtx, "审查结果为空")
		return
	}
	if sm := buildReviewStructuredMessage(review); sm != nil {
		if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
			return
		}
	}
	if cs, ok := p.(core.CardSender); ok {
		card := buildReviewCard(review)
		_ = cs.ReplyCard(ctx, msg.ReplyCtx, card)
		return
	}
	prefix := ""
	if strings.TrimSpace(title) != "" {
		prefix = title + "\n"
	}
	reply := fmt.Sprintf("%s代码审查 #%s\nPR: %s\n状态: %s\n建议: %s",
		prefix,
		shortID(review.ID),
		review.PRURL,
		review.Status,
		review.Recommendation,
	)
	if followups := formatReviewFollowUpTasks(review); followups != "" {
		reply += "\n后续任务建议:\n" + followups
	}
	_ = p.Reply(ctx, msg.ReplyCtx, reply)
}

func buildReviewStructuredMessage(review *client.Review) *core.StructuredMessage {
	if review == nil {
		return nil
	}
	fields := []core.StructuredField{
		{Label: "PR", Value: review.PRURL},
		{Label: "状态", Value: review.Status},
		{Label: "风险等级", Value: review.RiskLevel},
	}
	if review.Summary != "" {
		fields = append(fields, core.StructuredField{Label: "摘要", Value: review.Summary})
	}
	if review.Recommendation != "" {
		fields = append(fields, core.StructuredField{Label: "建议", Value: review.Recommendation})
	}
	if review.CostUSD > 0 {
		fields = append(fields, core.StructuredField{Label: "费用", Value: fmt.Sprintf("$%.2f", review.CostUSD)})
	}

	sections := []core.StructuredSection{
		{Type: core.StructuredSectionTypeFields, FieldsSection: &core.FieldsSection{Fields: fields}},
	}

	if followups := formatReviewFollowUpTasks(review); followups != "" {
		sections = append(sections, core.StructuredSection{
			Type: core.StructuredSectionTypeDivider, DividerSection: &core.DividerSection{},
		})
		sections = append(sections, core.StructuredSection{
			Type:        core.StructuredSectionTypeText,
			TextSection: &core.TextSection{Body: "**后续任务建议**\n" + followups},
		})
	}

	actions := make([]core.StructuredAction, 0, 3)
	actions = append(actions, core.StructuredAction{
		Label: "查看详情", URL: "/reviews/" + review.ID, Style: core.ActionStyleDefault,
	})
	if review.Status == "pending_human" {
		actions = append(actions, core.StructuredAction{
			ID: "act:approve:" + review.ID, Label: "Approve", Style: core.ActionStylePrimary,
		})
		actions = append(actions, core.StructuredAction{
			ID: "act:request-changes:" + review.ID, Label: "Request Changes", Style: core.ActionStyleDanger,
		})
	}
	sections = append(sections, core.StructuredSection{
		Type:           core.StructuredSectionTypeActions,
		ActionsSection: &core.ActionsSection{Actions: actions},
	})

	return &core.StructuredMessage{
		Title:    fmt.Sprintf("代码审查 #%s", shortID(review.ID)),
		Sections: sections,
	}
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
	if followups := formatReviewFollowUpTasks(review); followups != "" {
		card.AddField("后续任务", followups)
	}
	if review.CostUSD > 0 {
		card.AddField("费用", fmt.Sprintf("$%.2f", review.CostUSD))
	}

	card.AddButton("查看详情", "link:/reviews/"+review.ID)
	if review.Status == "pending_human" {
		card.AddButton("Approve", "act:approve:"+review.ID)
		card.AddButton("Request Changes", "act:request-changes:"+review.ID)
	}
	if isTerminalReviewStatus(review.Status) {
		// Terminal cards intentionally keep only the details link.
		return card
	}
	return card
}

func formatReviewFollowUpTasks(review *client.Review) string {
	if review == nil || !isTerminalReviewStatus(review.Status) || len(review.Findings) == 0 {
		return ""
	}
	commands := make([]string, 0, 3)
	for _, finding := range review.Findings {
		if finding.Dismissed {
			continue
		}
		title := strings.TrimSpace(finding.Suggestion)
		if title == "" {
			title = strings.TrimSpace(finding.Message)
		}
		if title == "" {
			continue
		}
		if file := strings.TrimSpace(finding.File); file != "" {
			title += " [" + file + "]"
		}
		commands = append(commands, "/task create 修复审查问题: "+title)
		if len(commands) == 3 {
			break
		}
	}
	return strings.Join(commands, "\n")
}

func isTerminalReviewStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "completed", "failed":
		return true
	default:
		return false
	}
}
