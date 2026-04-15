package commands

import (
	"context"
	"net/http"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

var helpText = buildHelpText()

// RegisterHelpCommand registers the /help command on the engine.
func RegisterHelpCommand(engine *core.Engine) {
	engine.RegisterCommand("/help", func(p core.Platform, msg *core.Message, args string) {
		ctx := context.Background()

		// Try rich rendering first.
		message := buildHelpStructuredMessage(p)
		if err := replyStructured(ctx, p, msg.ReplyCtx, message); err == nil {
			return
		}

		// Fallback to plain text.
		_ = p.Reply(ctx, msg.ReplyCtx, helpText)
	})
}

type callbackHTTPProvider interface {
	HTTPCallbackHandler() http.Handler
}

type callbackPathProvider interface {
	CallbackPaths() []string
}

func buildHelpStructuredMessage(platform ...core.Platform) *core.StructuredMessage {
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

	var currentPlatform core.Platform
	if len(platform) > 0 {
		currentPlatform = platform[0]
	}

	sections := []core.StructuredSection{
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
	}

	if helpQuickActionsEnabled(currentPlatform) {
		sections = append(sections, core.StructuredSection{
			Type: core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{
				Actions: []core.StructuredAction{
					{ID: "act:cmd:/task list", Label: "任务列表", Style: core.ActionStylePrimary},
					{ID: "act:cmd:/agent status", Label: "Agent 状态", Style: core.ActionStyleDefault},
					{ID: "act:cmd:/sprint status", Label: "Sprint", Style: core.ActionStyleDefault},
					{ID: "act:cmd:/cost", Label: "费用统计", Style: core.ActionStyleDefault},
				},
				ButtonsPerRow: 2,
			},
		})
	} else if isFeishuPlatform(currentPlatform) {
		sections = append(sections, core.StructuredSection{
			Type: core.StructuredSectionTypeContext,
			ContextSection: &core.ContextSection{Elements: []string{
				"飞书快捷按钮依赖卡片回调配置。当前未检测到可用回调入口，请直接发送 /task list、/agent status、/sprint status、/cost。",
			}},
		})
	}

	return &core.StructuredMessage{
		Title:    "AgentForge IM 助手",
		Sections: sections,
	}
}

func findOrDefault(command string) commandCatalogEntry {
	if entry := findCommandCatalogEntry(command); entry != nil {
		return *entry
	}
	return commandCatalogEntry{Command: command, Summary: command}
}

func isFeishuPlatform(platform core.Platform) bool {
	if platform == nil {
		return false
	}
	return core.MetadataForPlatform(platform).Source == "feishu"
}

func helpQuickActionsEnabled(platform core.Platform) bool {
	if platform == nil {
		return true
	}
	metadata := core.MetadataForPlatform(platform)
	if metadata.Source != "feishu" {
		return true
	}
	if handlerProvider, ok := platform.(callbackHTTPProvider); ok && handlerProvider.HTTPCallbackHandler() != nil {
		if pathProvider, ok := platform.(callbackPathProvider); ok {
			for _, path := range pathProvider.CallbackPaths() {
				if strings.TrimSpace(path) != "" {
					return true
				}
			}
		}
	}
	if metadata.Capabilities.ActionCallbackMode == core.ActionCallbackSocketPayload &&
		!metadata.Capabilities.RequiresPublicCallback {
		return true
	}
	return false
}
