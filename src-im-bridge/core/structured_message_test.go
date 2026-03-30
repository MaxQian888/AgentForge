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

func TestStructuredSection_FallbackTextCoversAllSectionTypes(t *testing.T) {
	tests := []struct {
		name    string
		section StructuredSection
		want    string
	}{
		{
			name: "text",
			section: StructuredSection{
				Type: StructuredSectionTypeText,
				TextSection: &TextSection{
					Body: "Build *ready*",
				},
			},
			want: "Build ready",
		},
		{
			name: "image",
			section: StructuredSection{
				Type: StructuredSectionTypeImage,
				ImageSection: &ImageSection{
					URL:     "https://example.test/image.png",
					AltText: "Build preview",
				},
			},
			want: "Build preview: https://example.test/image.png",
		},
		{
			name: "divider",
			section: StructuredSection{
				Type:           StructuredSectionTypeDivider,
				DividerSection: &DividerSection{},
			},
			want: "---",
		},
		{
			name: "context",
			section: StructuredSection{
				Type: StructuredSectionTypeContext,
				ContextSection: &ContextSection{
					Elements: []string{"alice", "2m ago"},
				},
			},
			want: "alice | 2m ago",
		},
		{
			name: "fields",
			section: StructuredSection{
				Type: StructuredSectionTypeFields,
				FieldsSection: &FieldsSection{
					Fields: []StructuredField{{Label: "Status", Value: "success"}},
				},
			},
			want: "Status: success",
		},
		{
			name: "actions",
			section: StructuredSection{
				Type: StructuredSectionTypeActions,
				ActionsSection: &ActionsSection{
					Actions: []StructuredAction{{Label: "Open", URL: "https://example.test/builds/1"}},
				},
			},
			want: "Open: https://example.test/builds/1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.section.FallbackText(); got != tc.want {
				t.Fatalf("FallbackText() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestStructuredMessage_FallbackTextPrefersSections(t *testing.T) {
	message := &StructuredMessage{
		Title: "Legacy Title",
		Body:  "Legacy body",
		Sections: []StructuredSection{
			{
				Type: StructuredSectionTypeText,
				TextSection: &TextSection{
					Body: "Build ready",
				},
			},
			{
				Type:           StructuredSectionTypeDivider,
				DividerSection: &DividerSection{},
			},
			{
				Type: StructuredSectionTypeFields,
				FieldsSection: &FieldsSection{
					Fields: []StructuredField{{Label: "Status", Value: "success"}},
				},
			},
			{
				Type: StructuredSectionTypeActions,
				ActionsSection: &ActionsSection{
					Actions: []StructuredAction{{Label: "Open", URL: "https://example.test/builds/1"}},
				},
			},
		},
	}

	if got, want := message.FallbackText(), "Build ready\n---\nStatus: success\nOpen: https://example.test/builds/1"; got != want {
		t.Fatalf("FallbackText() = %q, want %q", got, want)
	}
}
