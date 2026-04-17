package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

// RegisterQueueCommands registers /queue sub-commands on the engine.
func RegisterQueueCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/queue", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.Fields(strings.TrimSpace(args))
		if len(parts) == 0 {
			_ = p.Reply(context.Background(), msg.ReplyCtx, commandUsage("/queue"))
			return
		}

		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch canonicalSubcommand("/queue", parts[0]) {
		case "list":
			filter := ""
			if len(parts) > 1 {
				filter = parts[1]
			}
			entries, err := scopedClient.ListQueueEntries(ctx, filter)
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取队列失败: %v", err))
				return
			}
			if sm := buildQueueListStructuredMessage(entries); sm != nil {
				if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
					return
				}
			}
			_ = p.Reply(ctx, msg.ReplyCtx, formatQueueEntries(entries))
		case "cancel":
			if len(parts) < 2 {
				_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/queue", "cancel"))
				return
			}
			entry, err := scopedClient.CancelQueueEntry(ctx, parts[1], "manual_cancel")
			if err != nil {
				_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("取消队列失败: %v", err))
				return
			}
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("队列项 %s 已变更为 %s", entry.EntryID, entry.Status))
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, commandUsage("/queue"))
		}
	})
}

func buildQueueListStructuredMessage(entries []client.QueueEntry) *core.StructuredMessage {
	if len(entries) == 0 {
		return nil
	}
	fields := make([]core.StructuredField, 0, len(entries))
	actions := make([]core.StructuredAction, 0)
	for _, entry := range entries {
		label := fmt.Sprintf("%s [%s]", shortID(entry.EntryID), entry.Status)
		value := fmt.Sprintf("task=%s priority=%d", shortID(entry.TaskID), entry.Priority)
		if entry.Reason != "" {
			value += " " + entry.Reason
		}
		fields = append(fields, core.StructuredField{Label: label, Value: value})
		if entry.Status == "pending" || entry.Status == "waiting" {
			actions = append(actions, core.StructuredAction{
				ID:    "act:cancel-queue:" + entry.EntryID,
				Label: fmt.Sprintf("取消 %s", shortID(entry.EntryID)),
				Style: core.ActionStyleDanger,
			})
		}
	}
	sections := []core.StructuredSection{
		{Type: core.StructuredSectionTypeFields, FieldsSection: &core.FieldsSection{Fields: fields}},
	}
	if len(actions) > 0 {
		limit := 3
		if len(actions) < limit {
			limit = len(actions)
		}
		sections = append(sections, core.StructuredSection{
			Type:           core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{Actions: actions[:limit], ButtonsPerRow: 3},
		})
	}
	return &core.StructuredMessage{
		Title:    fmt.Sprintf("队列项 (%d)", len(entries)),
		Sections: sections,
	}
}

func formatQueueEntries(entries []client.QueueEntry) string {
	if len(entries) == 0 {
		return "当前没有匹配的队列项"
	}
	lines := make([]string, 0, len(entries)+1)
	lines = append(lines, fmt.Sprintf("队列项 (%d):", len(entries)))
	for _, entry := range entries {
		lines = append(lines, fmt.Sprintf("- %s [%s] task=%s member=%s priority=%d reason=%s",
			entry.EntryID,
			entry.Status,
			entry.TaskID,
			entry.MemberID,
			entry.Priority,
			entry.Reason,
		))
	}
	return strings.Join(lines, "\n")
}
