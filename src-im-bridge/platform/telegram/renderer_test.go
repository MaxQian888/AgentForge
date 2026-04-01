package telegram

import (
	"context"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestRenderTelegramText_EscapesMarkdownV2AndSetsParseMode(t *testing.T) {
	segments := renderTelegramText(&core.FormattedText{
		Content: "build *status* [ok]",
		Format:  core.TextFormatMarkdownV2,
	})
	if len(segments) != 1 {
		t.Fatalf("segments = %+v", segments)
	}
	if segments[0].ParseMode != "MarkdownV2" {
		t.Fatalf("ParseMode = %q", segments[0].ParseMode)
	}
	if segments[0].Text != `build \*status\* \[ok\]` {
		t.Fatalf("Text = %q", segments[0].Text)
	}
}

func TestRenderTelegramText_SplitsOversizedMessages(t *testing.T) {
	segments := renderTelegramText(&core.FormattedText{
		Content: strings.Repeat("a", telegramMaxTextLength+10),
		Format:  core.TextFormatPlainText,
	})
	if len(segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(segments))
	}
	if len([]rune(segments[0].Text)) != telegramMaxTextLength {
		t.Fatalf("first segment length = %d", len([]rune(segments[0].Text)))
	}
	if len([]rune(segments[1].Text)) != 10 {
		t.Fatalf("second segment length = %d", len([]rune(segments[1].Text)))
	}
}

func TestRenderTelegramText_FallsBackToPlainTextForUnsupportedFormat(t *testing.T) {
	segments := renderTelegramText(&core.FormattedText{
		Content: "hello <world>",
		Format:  core.TextFormatHTML,
	})
	if len(segments) != 1 {
		t.Fatalf("segments = %+v", segments)
	}
	if segments[0].ParseMode != "" {
		t.Fatalf("ParseMode = %q, want plain-text fallback", segments[0].ParseMode)
	}
	if segments[0].Text != "hello <world>" {
		t.Fatalf("Text = %q", segments[0].Text)
	}
}

func TestLive_UpdateFormattedTextFallsBackToSegmentedRepliesWhenOversized(t *testing.T) {
	runner := &fakeUpdateRunner{}
	sender := &fakeSender{}

	live, err := NewLive("bot-token", WithUpdateRunner(runner), WithSender(sender))
	if err != nil {
		t.Fatalf("NewLive error: %v", err)
	}

	err = live.UpdateFormattedText(context.Background(), replyContext{ChatID: -2001, MessageID: 42, TopicID: 777}, &core.FormattedText{
		Content: strings.Repeat("a", telegramMaxTextLength+10),
		Format:  core.TextFormatMarkdownV2,
	})
	if err != nil {
		t.Fatalf("UpdateFormattedText error: %v", err)
	}

	if len(sender.edits) != 0 {
		t.Fatalf("edits = %+v, want none", sender.edits)
	}
	if len(sender.calls) != 2 {
		t.Fatalf("calls = %+v, want 2 sends", sender.calls)
	}
	if sender.calls[0].ReplyToMessageID != 42 {
		t.Fatalf("first call = %+v, want reply to original message", sender.calls[0])
	}
	if sender.calls[0].ParseMode != "MarkdownV2" || sender.calls[1].ParseMode != "MarkdownV2" {
		t.Fatalf("calls = %+v", sender.calls)
	}
}

func TestRenderCardToTelegram_BuildsMarkdownV2WithKeyboard(t *testing.T) {
	card := core.NewCard().
		SetTitle("Build #42").
		AddField("Status", "success").
		AddField("Branch", "main").
		AddPrimaryButton("Approve", "act:approve:build-42").
		AddButton("View", "link:https://example.test/builds/42")

	text, markup := renderCardToTelegram(card)

	if !strings.Contains(text, "*Build \\#42*") {
		t.Fatalf("title not escaped: %q", text)
	}
	if !strings.Contains(text, "*Status:* success") {
		t.Fatalf("field not formatted: %q", text)
	}
	if !strings.Contains(text, "*Branch:* main") {
		t.Fatalf("second field not formatted: %q", text)
	}
	if markup == nil || len(markup.InlineKeyboard) != 2 {
		t.Fatalf("markup = %+v", markup)
	}
	if markup.InlineKeyboard[0][0].Text != "Approve" || markup.InlineKeyboard[0][0].CallbackData != "act:approve:build-42" {
		t.Fatalf("first button = %+v", markup.InlineKeyboard[0][0])
	}
	if markup.InlineKeyboard[1][0].Text != "View" || markup.InlineKeyboard[1][0].URL != "https://example.test/builds/42" {
		t.Fatalf("second button = %+v", markup.InlineKeyboard[1][0])
	}
}

func TestRenderCardToTelegram_NilCardReturnsEmpty(t *testing.T) {
	text, markup := renderCardToTelegram(nil)
	if text != "" || markup != nil {
		t.Fatalf("nil card: text=%q, markup=%+v", text, markup)
	}
}

func TestRenderCardToTelegram_SkipsEmptyButtonsAndFields(t *testing.T) {
	card := core.NewCard().
		AddField("", "").
		AddField("", "standalone value").
		AddButton("", "act:noop").
		AddButton("OK", "")

	text, markup := renderCardToTelegram(card)

	if !strings.Contains(text, "standalone value") {
		t.Fatalf("text = %q", text)
	}
	if strings.Contains(text, "noop") {
		t.Fatalf("empty text button should be skipped: %q", text)
	}
	if markup != nil {
		t.Fatalf("expected no keyboard for empty/invalid buttons, got %+v", markup)
	}
}

func TestCardFallbackText_RendersPlainTextSummary(t *testing.T) {
	card := core.NewCard().
		SetTitle("Deploy").
		AddField("Env", "prod").
		AddButton("Rollback", "act:rollback:deploy-1")

	text := cardFallbackText(card)
	if !strings.Contains(text, "Deploy") || !strings.Contains(text, "Env: prod") || !strings.Contains(text, "[Rollback] act:rollback:deploy-1") {
		t.Fatalf("fallback text = %q", text)
	}
}

func TestCardFallbackText_NilCardReturnsEmpty(t *testing.T) {
	if got := cardFallbackText(nil); got != "" {
		t.Fatalf("nil card fallback = %q", got)
	}
}
