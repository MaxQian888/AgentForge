package commands

import (
	"context"

	"github.com/agentforge/im-bridge/core"
)

var helpText = buildHelpText()

// RegisterHelpCommand registers the /help command on the engine.
func RegisterHelpCommand(engine *core.Engine) {
	engine.RegisterCommand("/help", func(p core.Platform, msg *core.Message, args string) {
		ctx := context.Background()

		// Try rich rendering first.
		message := buildHelpStructuredMessage()
		if err := replyStructured(ctx, p, msg.ReplyCtx, message); err == nil {
			return
		}

		// Fallback to plain text.
		_ = p.Reply(ctx, msg.ReplyCtx, helpText)
	})
}

func buildHelpStructuredMessage() *core.StructuredMessage {
	taskGroup := []commandCatalogEntry{
		findOrDefault("/task"),
		findOrDefault("/sprint"),
		findOrDefault("/review"),
	}
	agentGroup := []commandCatalogEntry{
		findOrDefault("/agent"),
		findOrDefault("/queue"),
		findOrDefault("/login"),
	}
	projectGroup := []commandCatalogEntry{
		findOrDefault("/project"),
		findOrDefault("/team"),
		findOrDefault("/memory"),
		findOrDefault("/doc"),
	}
	toolsGroup := []commandCatalogEntry{
		findOrDefault("/tools"),
		findOrDefault("/cost"),
	}

	return &core.StructuredMessage{
		Title: "AgentForge IM 助手",
		Sections: []core.StructuredSection{
			buildCommandGroupSection("任务与代码", taskGroup),
			{Type: core.StructuredSectionTypeDivider, DividerSection: &core.DividerSection{}},
			buildCommandGroupSection("Agent 与运行", agentGroup),
			{Type: core.StructuredSectionTypeDivider, DividerSection: &core.DividerSection{}},
			buildCommandGroupSection("项目与协作", projectGroup),
			{Type: core.StructuredSectionTypeDivider, DividerSection: &core.DividerSection{}},
			buildCommandGroupSection("工具与费用", toolsGroup),
			{Type: core.StructuredSectionTypeDivider, DividerSection: &core.DividerSection{}},
			{
				Type:           core.StructuredSectionTypeContext,
				ContextSection: &core.ContextSection{Elements: []string{"直接 @AgentForge <你的需求> 使用自然语言"}},
			},
			{
				Type: core.StructuredSectionTypeActions,
				ActionsSection: &core.ActionsSection{
					Actions: []core.StructuredAction{
						{ID: "act:cmd:/task list", Label: "任务列表", Style: core.ActionStylePrimary},
						{ID: "act:cmd:/agent status", Label: "Agent 状态", Style: core.ActionStyleDefault},
						{ID: "act:cmd:/sprint status", Label: "Sprint", Style: core.ActionStyleDefault},
						{ID: "act:cmd:/cost", Label: "费用统计", Style: core.ActionStyleDefault},
					},
					ButtonsPerRow: 4,
				},
			},
		},
	}
}

func findOrDefault(command string) commandCatalogEntry {
	if entry := findCommandCatalogEntry(command); entry != nil {
		return *entry
	}
	return commandCatalogEntry{Command: command, Summary: command}
}
