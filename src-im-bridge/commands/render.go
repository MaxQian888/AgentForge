package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

var errNoRichSender = fmt.Errorf("platform does not support rich rendering")

// replyStructured sends a StructuredMessage to the platform using the richest
// available surface. Returns errNoRichSender if the platform only supports
// plain text, allowing callers to use their own fallback.
func replyStructured(ctx context.Context, p core.Platform, replyCtx any, message *core.StructuredMessage) error {
	if message == nil {
		return errNoRichSender
	}
	// Prefer StructuredSender (Feishu renderer, Slack Block Kit, Discord Components).
	if ss, ok := p.(core.ReplyStructuredSender); ok {
		if err := ss.ReplyStructured(ctx, replyCtx, message); err == nil {
			return nil
		}
	}
	// Fall back to CardSender with legacy card conversion.
	if cs, ok := p.(core.CardSender); ok {
		card := message.LegacyCard()
		if card != nil && (card.Title != "" || len(card.Fields) > 0 || len(card.Buttons) > 0) {
			return cs.ReplyCard(ctx, replyCtx, card)
		}
	}
	// No rich sender available — return error so caller can use plain text.
	return errNoRichSender
}

// sendStructured is like replyStructured but sends to a chatID rather than replying.
func sendStructured(ctx context.Context, p core.Platform, chatID string, message *core.StructuredMessage) error {
	if message == nil {
		return errNoRichSender
	}
	if ss, ok := p.(core.StructuredSender); ok {
		if err := ss.SendStructured(ctx, chatID, message); err == nil {
			return nil
		}
	}
	if cs, ok := p.(core.CardSender); ok {
		card := message.LegacyCard()
		if card != nil && (card.Title != "" || len(card.Fields) > 0 || len(card.Buttons) > 0) {
			return cs.SendCard(ctx, chatID, card)
		}
	}
	return errNoRichSender
}

// replyError sends an error message as a rich card when the platform supports
// it, falling back to plain text. The hint suggests a recovery action.
func replyError(ctx context.Context, p core.Platform, replyCtx any, title, message, hint string) {
	sections := []core.StructuredSection{
		{
			Type:        core.StructuredSectionTypeText,
			TextSection: &core.TextSection{Body: strings.TrimSpace(message)},
		},
	}
	if trimmedHint := strings.TrimSpace(hint); trimmedHint != "" {
		sections = append(sections, core.StructuredSection{
			Type:           core.StructuredSectionTypeContext,
			ContextSection: &core.ContextSection{Elements: []string{trimmedHint}},
		})
	}

	sm := &core.StructuredMessage{
		Title:    strings.TrimSpace(title),
		Sections: sections,
	}
	if err := replyStructured(ctx, p, replyCtx, sm); err == nil {
		return
	}
	// Fallback: combine into plain text.
	text := strings.TrimSpace(message)
	if trimmedHint := strings.TrimSpace(hint); trimmedHint != "" {
		text += "\n" + trimmedHint
	}
	_ = p.Reply(ctx, replyCtx, text)
}

// replyProcessing sends a lightweight "processing" hint before a long-running
// operation. On platforms that support rich messages this renders as a compact
// card; on text-only platforms it sends a one-liner.
func replyProcessing(ctx context.Context, p core.Platform, replyCtx any, hint string) {
	if hint == "" {
		hint = "处理中，请稍候..."
	}
	sm := &core.StructuredMessage{
		Title: hint,
		Sections: []core.StructuredSection{
			{
				Type:           core.StructuredSectionTypeContext,
				ContextSection: &core.ContextSection{Elements: []string{"正在处理，结果将稍后送达"}},
			},
		},
	}
	if err := replyStructured(ctx, p, replyCtx, sm); err == nil {
		return
	}
	_ = p.Reply(ctx, replyCtx, hint)
}

// buildCommandGroupSection creates a StructuredSection for a group of commands.
func buildCommandGroupSection(groupLabel string, entries []commandCatalogEntry) core.StructuredSection {
	var sb strings.Builder
	sb.WriteString("**")
	sb.WriteString(groupLabel)
	sb.WriteString("**\n")
	for _, entry := range entries {
		if len(entry.Subcommands) == 0 {
			sb.WriteString("`")
			sb.WriteString(entry.Command)
			sb.WriteString("` — ")
			sb.WriteString(entry.Summary)
			sb.WriteString("\n")
			continue
		}
		sb.WriteString("`")
		sb.WriteString(entry.Command)
		sb.WriteString("` — ")
		sb.WriteString(entry.Summary)
		sb.WriteString("\n")
	}
	return core.StructuredSection{
		Type:        core.StructuredSectionTypeText,
		TextSection: &core.TextSection{Body: strings.TrimRight(sb.String(), "\n")},
	}
}
