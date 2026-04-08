package dingtalk

import (
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestRenderStructuredSections_TextFallback(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type:        core.StructuredSectionTypeText,
			TextSection: &core.TextSection{Body: "Hello DingTalk"},
		},
	}
	card := renderStructuredSections(sections)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if card.CardType != core.DingTalkCardTypeActionCard {
		t.Errorf("CardType = %q, want %q", card.CardType, core.DingTalkCardTypeActionCard)
	}
	if !strings.Contains(card.Markdown, "Hello DingTalk") {
		t.Errorf("Markdown should contain text, got %q", card.Markdown)
	}
	if card.Title != "Hello DingTalk" {
		t.Errorf("Title = %q, want %q", card.Title, "Hello DingTalk")
	}
}

func TestRenderStructuredSections_WithActions(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type:        core.StructuredSectionTypeText,
			TextSection: &core.TextSection{Body: "Deploy update"},
		},
		{
			Type: core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{
				Actions: []core.StructuredAction{
					{Label: "View Details", URL: "https://example.com/deploy"},
					{Label: "No URL Action"},
				},
			},
		},
	}
	card := renderStructuredSections(sections)
	if card == nil {
		t.Fatal("expected non-nil card")
	}
	if len(card.Buttons) != 1 {
		t.Fatalf("expected 1 button (only ones with URLs), got %d", len(card.Buttons))
	}
	if card.Buttons[0].Title != "View Details" {
		t.Errorf("Button title = %q", card.Buttons[0].Title)
	}
	if card.Buttons[0].ActionURL != "https://example.com/deploy" {
		t.Errorf("Button URL = %q", card.Buttons[0].ActionURL)
	}
}

func TestRenderStructuredSections_EmptySections(t *testing.T) {
	card := renderStructuredSections(nil)
	if card == nil {
		t.Fatal("expected non-nil card even for nil sections")
	}
	if card.Title != "AgentForge Update" {
		t.Errorf("Title = %q, want default", card.Title)
	}
}

func TestRenderStructuredSections_NilActionsSection(t *testing.T) {
	sections := []core.StructuredSection{
		{Type: core.StructuredSectionTypeActions, ActionsSection: nil},
	}
	card := renderStructuredSections(sections)
	if len(card.Buttons) != 0 {
		t.Errorf("nil actions section should produce no buttons")
	}
}

func TestRenderStructuredSections_MultipleSections(t *testing.T) {
	sections := []core.StructuredSection{
		{Type: core.StructuredSectionTypeText, TextSection: &core.TextSection{Body: "Line 1"}},
		{Type: core.StructuredSectionTypeDivider},
		{Type: core.StructuredSectionTypeText, TextSection: &core.TextSection{Body: "Line 2"}},
	}
	card := renderStructuredSections(sections)
	if card.Title != "Line 1" {
		t.Errorf("Title should be first text line, got %q", card.Title)
	}
	if !strings.Contains(card.Markdown, "Line 1") || !strings.Contains(card.Markdown, "Line 2") {
		t.Errorf("Markdown should contain both lines: %q", card.Markdown)
	}
}
