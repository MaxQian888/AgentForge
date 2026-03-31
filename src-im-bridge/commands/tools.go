package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

func RegisterToolsCommands(engine *core.Engine, apiClient *client.AgentForgeClient) {
	engine.RegisterCommand("/tools", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.Fields(strings.TrimSpace(args))
		if len(parts) == 0 {
			_ = p.Reply(context.Background(), msg.ReplyCtx, commandUsage("/tools"))
			return
		}

		ctx := context.Background()
		scopedClient := apiClient.WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch canonicalSubcommand("/tools", parts[0]) {
		case "list":
			tools, err := scopedClient.ListBridgeTools(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge tools 失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgeTools(tools))
		case "install":
			if len(parts) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/tools", "install"))
				return
			}
			if err := requireToolsAdmin(ctx, scopedClient, msg, "Admin role required for plugin installation"); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			record, err := scopedClient.InstallBridgeTool(ctx, parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("安装插件失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgePluginRecord("安装完成", record))
		case "uninstall":
			if len(parts) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/tools", "uninstall"))
				return
			}
			if err := requireToolsAdmin(ctx, scopedClient, msg, "Admin role required for plugin uninstallation"); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			record, err := scopedClient.UninstallBridgeTool(ctx, parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("卸载插件失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgePluginRecord("卸载完成", record))
		case "restart":
			if len(parts) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/tools", "restart"))
				return
			}
			record, err := scopedClient.RestartBridgeTool(ctx, parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("重启插件失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgePluginRecord("重启完成", record))
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/tools"))
		}
	})
}

func requireToolsAdmin(ctx context.Context, c *client.AgentForgeClient, msg *core.Message, message string) error {
	member, err := resolveToolsOperator(ctx, c, msg)
	if err != nil {
		return err
	}
	if member == nil || !isToolsAdminRole(member.Role) {
		return fmt.Errorf("%s", message)
	}
	return nil
}

func resolveToolsOperator(ctx context.Context, c *client.AgentForgeClient, msg *core.Message) (*client.Member, error) {
	members, err := c.ListProjectMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("lookup project members failed: %w", err)
	}

	userID := strings.TrimSpace(msg.UserID)
	userName := strings.TrimSpace(msg.UserName)
	for i := range members {
		member := &members[i]
		if !member.IsActive {
			continue
		}
		if member.IMUserID != "" && member.IMUserID == userID {
			return member, nil
		}
		if member.ID == userID {
			return member, nil
		}
		if userName != "" && strings.EqualFold(member.Name, userName) {
			return member, nil
		}
	}
	return nil, nil
}

func isToolsAdminRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "admin", "owner", "lead":
		return true
	default:
		return false
	}
}

func formatBridgeTools(tools []client.BridgeTool) string {
	if len(tools) == 0 {
		return "当前没有可用的 Bridge tools"
	}
	lines := make([]string, 0, len(tools)+1)
	lines = append(lines, fmt.Sprintf("Bridge tools (%d):", len(tools)))
	for _, tool := range tools {
		line := fmt.Sprintf("- %s / %s", tool.PluginID, tool.Name)
		if desc := strings.TrimSpace(tool.Description); desc != "" {
			line += " - " + desc
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func formatBridgePluginRecord(prefix string, record *client.BridgePluginRecord) string {
	if record == nil {
		return prefix
	}
	parts := []string{prefix, record.Metadata.ID, record.LifecycleState}
	if record.RestartCount > 0 {
		parts = append(parts, fmt.Sprintf("restart_count=%d", record.RestartCount))
	}
	return strings.Join(parts, " | ")
}
