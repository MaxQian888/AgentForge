package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
)

var docUsage = commandUsage("/doc")

// RegisterDocumentCommands registers /doc sub-commands on the engine.
func RegisterDocumentCommands(engine *core.Engine, factory client.ClientProvider) {
	engine.RegisterCommand("/doc", func(p core.Platform, msg *core.Message, args string) {
		parts := strings.SplitN(strings.TrimSpace(args), " ", 2)
		if len(parts) == 0 || parts[0] == "" {
			_ = p.Reply(context.Background(), msg.ReplyCtx, docUsage)
			return
		}
		subCmd := canonicalSubcommand("/doc", parts[0])
		subArgs := ""
		if len(parts) > 1 {
			subArgs = parts[1]
		}

		ctx := context.Background()
		scopedClient := factory.For(msg.TenantID).WithSource(msg.Platform).WithBridgeContext("", msg.ReplyTarget)
		switch subCmd {
		case "list":
			handleDocList(ctx, p, msg, scopedClient)
		case "upload":
			handleDocUpload(ctx, p, msg, scopedClient, subArgs)
		default:
			_ = p.Reply(ctx, msg.ReplyCtx, docUsage)
		}
	})
}

func handleDocList(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient) {
	docs, err := c.ListDocuments(ctx)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("获取文档列表失败: %v", err))
		return
	}
	if len(docs) == 0 {
		_ = p.Reply(ctx, msg.ReplyCtx, "No documents found.")
		return
	}
	if sm := buildDocListStructuredMessage(docs); sm != nil {
		if err := replyStructured(ctx, p, msg.ReplyCtx, sm); err == nil {
			return
		}
	}
	_ = p.Reply(ctx, msg.ReplyCtx, formatDocumentList(docs))
}

func buildDocListStructuredMessage(docs []client.DocumentEntry) *core.StructuredMessage {
	if len(docs) == 0 {
		return nil
	}
	fields := make([]core.StructuredField, 0, len(docs))
	for _, doc := range docs {
		label := doc.Name
		value := fmt.Sprintf("%s, %s — %s", doc.Type, doc.Size, doc.Status)
		fields = append(fields, core.StructuredField{Label: label, Value: value})
	}
	return &core.StructuredMessage{
		Title: fmt.Sprintf("项目文档 (%d)", len(docs)),
		Sections: []core.StructuredSection{
			{Type: core.StructuredSectionTypeFields, FieldsSection: &core.FieldsSection{Fields: fields}},
		},
	}
}

func handleDocUpload(ctx context.Context, p core.Platform, msg *core.Message, c *client.AgentForgeClient, rawURL string) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		_ = p.Reply(ctx, msg.ReplyCtx, subcommandUsage("/doc", "upload"))
		return
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		_ = p.Reply(ctx, msg.ReplyCtx, "请提供有效的 URL (http:// 或 https://)")
		return
	}

	err := c.UploadDocumentFromURL(ctx, rawURL)
	if err != nil {
		_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("上传文档失败: %v", err))
		return
	}
	_ = p.Reply(ctx, msg.ReplyCtx, fmt.Sprintf("文档上传成功: %s", rawURL))
}

func formatDocumentList(docs []client.DocumentEntry) string {
	var sb strings.Builder
	sb.WriteString("项目文档:\n")
	for _, doc := range docs {
		sb.WriteString(fmt.Sprintf("  \U0001F4C4 %s (%s, %s) \u2014 %s\n", doc.Name, doc.Type, doc.Size, doc.Status))
	}
	return strings.TrimRight(sb.String(), "\n")
}
