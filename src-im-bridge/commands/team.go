package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterTeamCommands registers /team sub-commands on the engine.
func RegisterTeamCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/team", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.Fields(strings.TrimSpace(args))
		if len(parts) == 0 {
			_ = p.Reply(context.Background(), msg.ReplyCtx, commandUsage("/team"))
			return
		}

		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch canonicalSubcommand("/team", parts[0]) {
		case "list":
			members, err := scopedClient.ListProjectMembers(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取团队失败: %v", err))
				return
			}
			if sm := buildTeamListStructuredMessage(members); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatTeamMembers(members))
		case "add":
			if len(parts) < 3 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/team", "add"))
				return
			}
			memberType := strings.TrimSpace(parts[1])
			name := strings.TrimSpace(parts[2])
			role := "developer"
			if len(parts) > 3 && strings.TrimSpace(parts[3]) != "" {
				role = strings.TrimSpace(parts[3])
			}
			member, err := scopedClient.CreateMember(ctx, client.CreateMemberInput{
				Name:   name,
				Type:   memberType,
				Role:   role,
				Status: "active",
			})
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("添加成员失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已添加成员: %s [%s/%s] role=%s", member.Name, member.Type, member.Status, member.Role))
		case "remove":
			if len(parts) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/team", "remove"))
				return
			}
			members, err := scopedClient.ListProjectMembers(ctx)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取团队失败: %v", err))
				return
			}
			member, err := resolveTeamMemberSelection(members, parts[1])
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, err.Error())
				return
			}
			if err := scopedClient.DeleteMember(ctx, member.ID); err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("移除成员失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已移除成员: %s", member.Name))
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/team"))
		}
	})
}

func buildTeamListStructuredMessage(members []client.Member) *core.StructuredMessage {
	if len(members) == 0 {
		return nil
	}
	fields := make([]core.StructuredField, 0, len(members))
	for _, m := range members {
		label := fmt.Sprintf("%s [%s]", m.Name, m.Type)
		value := fmt.Sprintf("%s — %s", m.Role, m.Status)
		fields = append(fields, core.StructuredField{Label: label, Value: value})
	}
	return &core.StructuredMessage{
		Title: fmt.Sprintf("项目成员 (%d)", len(members)),
		Sections: []core.StructuredSection{
			{
				Type:          core.StructuredSectionTypeFields,
				FieldsSection: &core.FieldsSection{Fields: fields},
			},
			{
				Type: core.StructuredSectionTypeActions,
				ActionsSection: &core.ActionsSection{
					Actions: []core.StructuredAction{
						{ID: "/team add", Label: "添加成员", Style: core.ActionStylePrimary},
					},
				},
			},
		},
	}
}

func formatTeamMembers(members []client.Member) string {
	if len(members) == 0 {
		return "当前项目还没有成员"
	}
	lines := make([]string, 0, len(members)+1)
	lines = append(lines, fmt.Sprintf("项目成员 (%d):", len(members)))
	for _, member := range members {
		lines = append(lines, fmt.Sprintf("- %s [%s/%s] role=%s",
			member.Name,
			member.Type,
			member.Status,
			member.Role,
		))
	}
	return strings.Join(lines, "\n")
}

func resolveTeamMemberSelection(members []client.Member, raw string) (*client.Member, error) {
	query := strings.TrimSpace(raw)
	if query == "" {
		return nil, fmt.Errorf("用法: /team remove <member-id|name>")
	}
	var exactMatches []*client.Member
	var fuzzyMatches []*client.Member
	for i := range members {
		member := &members[i]
		switch {
		case member.ID == query:
			exactMatches = append(exactMatches, member)
		case strings.EqualFold(member.Name, query):
			exactMatches = append(exactMatches, member)
		case strings.Contains(strings.ToLower(member.Name), strings.ToLower(query)):
			fuzzyMatches = append(fuzzyMatches, member)
		}
	}
	switch {
	case len(exactMatches) == 1:
		return exactMatches[0], nil
	case len(exactMatches) > 1:
		return nil, fmt.Errorf("匹配到多个成员，请使用更精确的 member id。")
	case len(fuzzyMatches) == 1:
		return fuzzyMatches[0], nil
	case len(fuzzyMatches) > 1:
		return nil, fmt.Errorf("匹配到多个成员，请使用更精确的 member id。")
	default:
		return nil, fmt.Errorf("找不到成员 %q。", query)
	}
}
