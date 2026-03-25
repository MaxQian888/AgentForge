package core

import (
	"strings"
	"testing"
)

func TestSelectStructuredRendererPrefersPlatformNativeSurface(t *testing.T) {
	message := &StructuredMessage{
		Title: "Task Update",
		Body:  "Agent completed implementation.",
		Actions: []StructuredAction{
			{ID: "open-task", Label: "Open Task", URL: "https://example.test/tasks/1", Style: ActionStylePrimary},
		},
	}

	slackRenderer := SelectStructuredRenderer(PlatformMetadata{
		Source: "slack",
		Capabilities: PlatformCapabilities{
			StructuredSurface: StructuredSurfaceBlocks,
		},
	}, message)
	if slackRenderer != StructuredSurfaceBlocks {
		t.Fatalf("slack renderer = %q, want %q", slackRenderer, StructuredSurfaceBlocks)
	}

	telegramRenderer := SelectStructuredRenderer(PlatformMetadata{
		Source: "telegram",
		Capabilities: PlatformCapabilities{
			StructuredSurface: StructuredSurfaceInlineKeyboard,
		},
	}, message)
	if telegramRenderer != StructuredSurfaceInlineKeyboard {
		t.Fatalf("telegram renderer = %q, want %q", telegramRenderer, StructuredSurfaceInlineKeyboard)
	}
}

func TestStructuredMessageFallbackTextIncludesTitleBodyFieldsAndActions(t *testing.T) {
	message := &StructuredMessage{
		Title: "Review Ready",
		Body:  "PR #42 needs final approval.",
		Fields: []StructuredField{
			{Label: "Risk", Value: "medium"},
			{Label: "Owner", Value: "alice"},
		},
		Actions: []StructuredAction{
			{ID: "approve", Label: "Approve", URL: "https://example.test/reviews/42", Style: ActionStylePrimary},
		},
	}

	text := message.FallbackText()
	if text == "" {
		t.Fatal("expected fallback text")
	}
	for _, want := range []string{"Review Ready", "PR #42 needs final approval.", "Risk: medium", "Owner: alice", "Approve: https://example.test/reviews/42"} {
		if !strings.Contains(text, want) {
			t.Fatalf("fallback text %q missing %q", text, want)
		}
	}
}

func TestStructuredMessage_LegacyCardPreservesActionStyles(t *testing.T) {
	card := (&StructuredMessage{
		Title: "Review Ready",
		Fields: []StructuredField{
			{Label: "Risk", Value: "medium"},
		},
		Actions: []StructuredAction{
			{ID: "approve", Label: "Approve", Style: ActionStylePrimary},
			{ID: "reject", Label: "Reject", Style: ActionStyleDanger},
			{URL: "https://example.test/reviews/42", Label: "Open"},
		},
	}).LegacyCard()

	if card == nil {
		t.Fatal("expected legacy card")
	}
	if card.Title != "Review Ready" {
		t.Fatalf("title = %q", card.Title)
	}
	if len(card.Fields) != 1 {
		t.Fatalf("fields = %+v", card.Fields)
	}
	if len(card.Buttons) != 3 {
		t.Fatalf("buttons = %+v", card.Buttons)
	}
	if card.Buttons[0].Style != "primary" || card.Buttons[1].Style != "danger" || card.Buttons[2].Action != "link:https://example.test/reviews/42" {
		t.Fatalf("buttons = %+v", card.Buttons)
	}

	if (*StructuredMessage)(nil).LegacyCard() != nil {
		t.Fatal("expected nil structured message to return nil legacy card")
	}
}
