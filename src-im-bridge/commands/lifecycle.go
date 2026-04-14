package commands

import (
	"context"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/im-bridge/core"
)

// BotLifecycleHandler sends onboarding and cleanup messages when the bot joins
// or leaves group chats.
type BotLifecycleHandler struct{}

func (h *BotLifecycleHandler) OnBotAdded(ctx context.Context, p core.Platform, chatID string) error {
	log.WithFields(log.Fields{"component": "lifecycle", "chat_id": chatID}).Info("Bot added to group, sending welcome card")

	welcome := &core.StructuredMessage{
		Title: "AgentForge 已加入群组",
		Sections: []core.StructuredSection{
			{
				Type:        core.StructuredSectionTypeText,
				TextSection: &core.TextSection{Body: "我是 **AgentForge IM 助手**，可以帮你管理任务、派发 Agent、审查代码、查看费用等。"},
			},
			{Type: core.StructuredSectionTypeDivider, DividerSection: &core.DividerSection{}},
			{
				Type:        core.StructuredSectionTypeText,
				TextSection: &core.TextSection{Body: "**快速开始**\n1. `/project set <slug>` 选择项目\n2. `/task create <标题>` 创建任务\n3. `/agent spawn <task-id>` 派发 Agent\n4. `@AgentForge <你的需求>` 自然语言"},
			},
			{
				Type: core.StructuredSectionTypeActions,
				ActionsSection: &core.ActionsSection{
					Actions: []core.StructuredAction{
						{ID: "act:cmd:/help", Label: "查看所有命令", Style: core.ActionStylePrimary},
						{ID: "act:cmd:/project list", Label: "项目列表", Style: core.ActionStyleDefault},
						{ID: "act:cmd:/login status", Label: "Runtime 状态", Style: core.ActionStyleDefault},
					},
					ButtonsPerRow: 3,
				},
			},
		},
	}

	if err := sendStructured(ctx, p, chatID, welcome); err == nil {
		return nil
	}

	// Fallback to plain text.
	text := "AgentForge 已加入群组！\n\n" +
		"发送 /help 查看可用命令\n" +
		"发送 /project set <slug> 选择项目\n" +
		"或 @AgentForge <你的需求> 使用自然语言"
	return p.Send(ctx, chatID, text)
}

func (h *BotLifecycleHandler) OnBotRemoved(ctx context.Context, p core.Platform, chatID string) error {
	log.WithFields(log.Fields{"component": "lifecycle", "chat_id": chatID}).Info("Bot removed from group")
	return nil
}
