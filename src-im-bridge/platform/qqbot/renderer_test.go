package qqbot

import (
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestRenderStructuredAsMarkdown_SectionsProduceMarkdown(t *testing.T) {
	message := &core.StructuredMessage{
		Sections: []core.StructuredSection{
			{
				Type:        core.StructuredSectionTypeText,
				TextSection: &core.TextSection{Body: "Review Ready"},
			},
			{
				Type:           core.StructuredSectionTypeDivider,
				DividerSection: &core.DividerSection{},
			},
			{
				Type: core.StructuredSectionTypeFields,
				FieldsSection: &core.FieldsSection{
					Fields: []core.StructuredField{
						{Label: "Status", Value: "Open"},
						{Label: "Author", Value: "User-1"},
					},
				},
			},
			{
				Type: core.StructuredSectionTypeContext,
				ContextSection: &core.ContextSection{
					Elements: []string{"Updated 2m ago", "3 comments"},
				},
			},
			{
				Type: core.StructuredSectionTypeActions,
				ActionsSection: &core.ActionsSection{
					Actions: []core.StructuredAction{
						{Label: "Open", URL: "https://example.test/reviews/1"},
						{Label: "Approve"},
					},
				},
			},
			{
				Type: core.StructuredSectionTypeImage,
				ImageSection: &core.ImageSection{
					URL:     "https://example.test/screenshot.png",
					AltText: "Screenshot",
				},
			},
		},
	}

	rendered := renderStructuredAsMarkdown(message)
	if rendered == "" {
		t.Fatal("expected non-empty markdown")
	}
	if !strings.Contains(rendered, "Review Ready") {
		t.Fatalf("missing text section, got: %q", rendered)
	}
	if !strings.Contains(rendered, "---") {
		t.Fatalf("missing divider, got: %q", rendered)
	}
	if !strings.Contains(rendered, "**Status:**") {
		t.Fatalf("missing field label, got: %q", rendered)
	}
	if !strings.Contains(rendered, "Updated 2m ago") {
		t.Fatalf("missing context element, got: %q", rendered)
	}
	if !strings.Contains(rendered, "[Open](https://example.test/reviews/1)") {
		t.Fatalf("missing action link, got: %q", rendered)
	}
	if !strings.Contains(rendered, "![Screenshot](https://example.test/screenshot.png)") {
		t.Fatalf("missing image, got: %q", rendered)
	}
}

func TestRenderStructuredAsMarkdown_LegacyFieldsProduceMarkdown(t *testing.T) {
	message := &core.StructuredMessage{
		Title: "Review Ready",
		Body:  "Choose the next step.",
		Fields: []core.StructuredField{
			{Label: "Status", Value: "Open"},
		},
		Actions: []core.StructuredAction{
			{Label: "Open", URL: "https://example.test/reviews/1"},
		},
	}

	rendered := renderStructuredAsMarkdown(message)
	if !strings.Contains(rendered, "**Review Ready**") {
		t.Fatalf("missing title, got: %q", rendered)
	}
	if !strings.Contains(rendered, "Choose the next step.") {
		t.Fatalf("missing body, got: %q", rendered)
	}
	if !strings.Contains(rendered, "**Status:**") {
		t.Fatalf("missing field, got: %q", rendered)
	}
	if !strings.Contains(rendered, "[Open](https://example.test/reviews/1)") {
		t.Fatalf("missing action, got: %q", rendered)
	}
}

func TestRenderStructuredAsMarkdown_NilAndEmptyReturnEmpty(t *testing.T) {
	if got := renderStructuredAsMarkdown(nil); got != "" {
		t.Fatalf("nil message = %q", got)
	}
	if got := renderStructuredAsMarkdown(&core.StructuredMessage{}); got != "" {
		t.Fatalf("empty message = %q", got)
	}
}
