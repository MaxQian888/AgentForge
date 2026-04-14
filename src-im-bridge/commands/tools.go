package commands

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	log "github.com/sirupsen/logrus"
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
			route := engine.ResolveCommandRoute("/tools", "list")
			if available, err := engine.BridgeCapabilityAvailable(ctx, route.Capability); !available {
				log.WithFields(log.Fields{"component": "commands.tools"}).WithError(err).Warn("Bridge tools list capability unavailable")
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge tools 失败: %v", err))
				return
			}
			tools, err := scopedClient.ListBridgeTools(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取 Bridge tools 失败: %v", err))
				return
			}
			log.WithFields(log.Fields{"component": "commands.tools", "toolCount": len(tools)}).Info("Bridge tools listed")
			if sm := buildToolsListStructuredMessage(tools); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgeTools(tools))
		case "install":
			if len(parts) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/tools", "install"))
				return
			}
			route := engine.ResolveCommandRoute("/tools", "install")
			if available, err := engine.BridgeCapabilityAvailable(ctx, route.Capability); !available {
				log.WithFields(log.Fields{"component": "commands.tools"}).WithError(err).Warn("Bridge tools install capability unavailable")
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("安装插件失败: %v", err))
				return
			}
			if err := requireToolsAdmin(ctx, scopedClient, msg, "Admin role required for plugin installation"); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			if err := validateBridgeToolManifestURL(parts[1]); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("安装插件失败: %v", err))
				return
			}
			record, err := scopedClient.InstallBridgeTool(ctx, parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("安装插件失败: %v", err))
				return
			}
			log.WithFields(log.Fields{"component": "commands.tools", "pluginId": record.Metadata.ID}).Info("Bridge tool installed")
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgePluginRecord("安装完成", record))
		case "uninstall":
			if len(parts) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/tools", "uninstall"))
				return
			}
			route := engine.ResolveCommandRoute("/tools", "uninstall")
			if available, err := engine.BridgeCapabilityAvailable(ctx, route.Capability); !available {
				log.WithFields(log.Fields{"component": "commands.tools"}).WithError(err).Warn("Bridge tools uninstall capability unavailable")
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("卸载插件失败: %v", err))
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
			log.WithFields(log.Fields{"component": "commands.tools", "pluginId": record.Metadata.ID}).Info("Bridge tool uninstalled")
			_ = p.Reply(ctx, msg.ReplyCtx, formatBridgePluginRecord("卸载完成", record))
		case "restart":
			if len(parts) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/tools", "restart"))
				return
			}
			route := engine.ResolveCommandRoute("/tools", "restart")
			if available, err := engine.BridgeCapabilityAvailable(ctx, route.Capability); !available {
				log.WithFields(log.Fields{"component": "commands.tools"}).WithError(err).Warn("Bridge tools restart capability unavailable")
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("重启插件失败: %v", err))
				return
			}
			record, err := scopedClient.RestartBridgeTool(ctx, parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("重启插件失败: %v", err))
				return
			}
			log.WithFields(log.Fields{"component": "commands.tools", "pluginId": record.Metadata.ID}).Info("Bridge tool restarted")
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

func buildToolsListStructuredMessage(tools []client.BridgeTool) *core.StructuredMessage {
	if len(tools) == 0 {
		return nil
	}
	fields := make([]core.StructuredField, 0, len(tools))
	for _, tool := range tools {
		label := tool.PluginID
		if label == "" {
			label = tool.Name
		}
		value := tool.Name
		if desc := strings.TrimSpace(tool.Description); desc != "" {
			value = desc
		}
		fields = append(fields, core.StructuredField{Label: label, Value: value})
	}
	return &core.StructuredMessage{
		Title: fmt.Sprintf("Bridge Tools (%d)", len(tools)),
		Sections: []core.StructuredSection{
			{Type: core.StructuredSectionTypeFields, FieldsSection: &core.FieldsSection{Fields: fields}},
		},
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

func validateBridgeToolManifestURL(raw string) error {
	parsedURL, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsedURL == nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("invalid manifest_url")
	}
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("manifest_url must use http or https")
	}

	allowlist := bridgeToolManifestAllowlist()
	if len(allowlist) == 0 {
		return nil
	}
	if _, ok := allowlist[strings.ToLower(strings.TrimSpace(parsedURL.Hostname()))]; !ok {
		return fmt.Errorf("manifest_url host not in allowlist")
	}
	return nil
}

func bridgeToolManifestAllowlist() map[string]struct{} {
	raw := os.Getenv("BRIDGE_TOOL_MANIFEST_ALLOWLIST")
	hosts := make(map[string]struct{})
	for _, entry := range strings.Split(raw, ",") {
		host := strings.ToLower(strings.TrimSpace(entry))
		if host == "" {
			continue
		}
		hosts[host] = struct{}{}
	}
	return hosts
}
