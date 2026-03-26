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
