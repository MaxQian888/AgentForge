package slack

import (
	"testing"

	"github.com/agentforge/im-bridge/core"
	goslack "github.com/slack-go/slack"
)

func TestRenderStructuredSections_TextSection(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type:        core.StructuredSectionTypeText,
			TextSection: &core.TextSection{Body: "Hello world"},
		},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	section, ok := blocks[0].(*goslack.SectionBlock)
	if !ok {
		t.Fatalf("expected SectionBlock, got %T", blocks[0])
	}
	if section.Text == nil || section.Text.Text != "Hello world" {
		t.Errorf("unexpected text: %v", section.Text)
	}
}

func TestRenderStructuredSections_EmptyTextSection(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type:        core.StructuredSectionTypeText,
			TextSection: &core.TextSection{Body: "  "},
		},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 0 {
		t.Errorf("empty text should produce no blocks, got %d", len(blocks))
	}
}

func TestRenderStructuredSections_ImageSection(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type:         core.StructuredSectionTypeImage,
			ImageSection: &core.ImageSection{URL: "https://example.com/img.png", AltText: "Test"},
		},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	img, ok := blocks[0].(*goslack.ImageBlock)
	if !ok {
		t.Fatalf("expected ImageBlock, got %T", blocks[0])
	}
	if img.ImageURL != "https://example.com/img.png" {
		t.Errorf("unexpected image URL: %s", img.ImageURL)
	}
}

func TestRenderStructuredSections_DividerSection(t *testing.T) {
	sections := []core.StructuredSection{
		{Type: core.StructuredSectionTypeDivider},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if _, ok := blocks[0].(*goslack.DividerBlock); !ok {
		t.Fatalf("expected DividerBlock, got %T", blocks[0])
	}
}

func TestRenderStructuredSections_ContextSection(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type:           core.StructuredSectionTypeContext,
			ContextSection: &core.ContextSection{Elements: []string{"element1", "element2"}},
		},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	ctx, ok := blocks[0].(*goslack.ContextBlock)
	if !ok {
		t.Fatalf("expected ContextBlock, got %T", blocks[0])
	}
	if len(ctx.ContextElements.Elements) != 2 {
		t.Errorf("expected 2 elements, got %d", len(ctx.ContextElements.Elements))
	}
}

func TestRenderStructuredSections_FieldsSection(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type: core.StructuredSectionTypeFields,
			FieldsSection: &core.FieldsSection{
				Fields: []core.StructuredField{
					{Label: "Status", Value: "Active"},
					{Label: "", Value: "Standalone"},
				},
			},
		},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	section, ok := blocks[0].(*goslack.SectionBlock)
	if !ok {
		t.Fatalf("expected SectionBlock, got %T", blocks[0])
	}
	if len(section.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(section.Fields))
	}
}

func TestRenderStructuredSections_ActionsSection(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type: core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{
				Actions: []core.StructuredAction{
					{ID: "btn-1", Label: "Approve", URL: "https://example.com"},
					{ID: "btn-2", Label: "Reject"},
				},
			},
		},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	action, ok := blocks[0].(*goslack.ActionBlock)
	if !ok {
		t.Fatalf("expected ActionBlock, got %T", blocks[0])
	}
	if len(action.Elements.ElementSet) != 2 {
		t.Errorf("expected 2 elements, got %d", len(action.Elements.ElementSet))
	}
}

func TestRenderStructuredSections_ActionsWithButtonsPerRow(t *testing.T) {
	sections := []core.StructuredSection{
		{
			Type: core.StructuredSectionTypeActions,
			ActionsSection: &core.ActionsSection{
				ButtonsPerRow: 1,
				Actions: []core.StructuredAction{
					{ID: "btn-1", Label: "First"},
					{ID: "btn-2", Label: "Second"},
				},
			},
		},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 2 {
		t.Fatalf("expected 2 blocks (1 per row), got %d", len(blocks))
	}
}

func TestRenderStructuredSections_NilSections(t *testing.T) {
	blocks := renderStructuredSections(nil)
	if len(blocks) != 0 {
		t.Errorf("nil sections should produce empty blocks, got %d", len(blocks))
	}
}

func TestRenderStructuredSections_MixedSections(t *testing.T) {
	sections := []core.StructuredSection{
		{Type: core.StructuredSectionTypeText, TextSection: &core.TextSection{Body: "Title"}},
		{Type: core.StructuredSectionTypeDivider},
		{Type: core.StructuredSectionTypeFields, FieldsSection: &core.FieldsSection{
			Fields: []core.StructuredField{{Label: "Key", Value: "Val"}},
		}},
	}
	blocks := renderStructuredSections(sections)
	if len(blocks) != 3 {
		t.Errorf("expected 3 blocks, got %d", len(blocks))
	}
}
