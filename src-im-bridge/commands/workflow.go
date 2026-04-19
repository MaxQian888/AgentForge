// Package commands — workflow.go registers the /workflow command that
// forwards normalized IM events to the backend trigger router
// (POST /api/v1/triggers/im/events). Any workflow with an IM trigger
// subscribed to this command name will be started.
//
// Usage: /workflow <name> [args...]
//
// Examples:
//   /workflow daily-product-selection
//   /workflow code-review https://github.com/acme/web/pull/42
//
// The command does NOT look up the workflow by name locally — instead,
// the payload's `command` field carries "/workflow <name>" concatenated,
// so trigger-node matchers that filter on `command: "/workflow foo"` will
// match. Workflows that want to respond to multiple command forms should
// register multiple trigger nodes or use a match_regex.
package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

var workflowUsage = commandUsage("/workflow")

// RegisterWorkflowCommands wires /workflow on the engine.
func RegisterWorkflowCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/workflow", func(p core.Platform, msg *core.Message, args string) {
		ctx := context.Background()
		trimmed := strings.TrimSpace(args)
		if trimmed == "" {
			_ = p.Reply(ctx, msg.ReplyCtx, workflowUsage)
			return
		}

		fields := strings.Fields(trimmed)
		name := fields[0]
		// Full command string used by trigger-node matchers: "/workflow daily-selection"
		// keeps the legacy convention where matchers compare against the leading
		// command token plus the chosen name as a single string.
		fullCommand := "/workflow " + name
		argValues := make([]any, 0, len(fields)-1)
		for _, f := range fields[1:] {
			argValues = append(argValues, f)
		}

		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)

		req := client.TriggerIMEventRequest{
			Platform:    msg.Platform,
			Command:     fullCommand,
			Content:     msg.Content,
			Args:        argValues,
			ChatID:      msg.ChatID,
			ThreadID:    msg.ThreadID,
			UserID:      msg.UserID,
			UserName:    msg.UserName,
			TenantID:    msg.TenantID,
			MessageID:   messageIDFromCtx(msg),
			ReplyTarget: msg.ReplyTarget,
		}

		resp, err := scopedClient.TriggerIMEvent(ctx, req)
		if err != nil {
			replyError(ctx, p, msg.ReplyCtx, "触发工作流失败", fmt.Sprintf("%v", err), "请确认后端触发器已注册")
			return
		}
		if resp == nil || resp.Started == 0 {
			msgBody := "未找到匹配的工作流"
			if resp != nil && resp.Message != "" {
				msgBody = resp.Message
			}
			_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("%s: /workflow %s", msgBody, name))
			return
		}
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("已启动 %d 个工作流执行 (/workflow %s)", resp.Started, name))
	})
}

// messageIDFromCtx pulls the message id from whatever platform-specific
// metadata is available. Feishu stashes it on msg.Metadata["message_id"];
// absence is fine — the backend treats empty as "no idempotency key".
func messageIDFromCtx(msg *core.Message) string {
	if msg == nil || msg.Metadata == nil {
		return ""
	}
	if id := strings.TrimSpace(msg.Metadata["message_id"]); id != "" {
		return id
	}
	return ""
}
