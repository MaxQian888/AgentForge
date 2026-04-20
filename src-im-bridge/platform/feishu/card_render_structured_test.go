package feishu

import (
	"encoding/json"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestRenderStructured_TextSection(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Test Card",
		Sections: []core.StructuredSection{
			{Type: "text", TextSection: &core.TextSection{Body: "Hello world"}},
		},
	}
	result, err := renderStructured(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	elements := parsed["elements"].([]any)
	if len(elements) != 1 {
		t.Fatalf("expected 1 element, got %d", len(elements))
	}
	div := elements[0].(map[string]any)
	if div["tag"] != "div" {
		t.Errorf("expected tag=div, got %v", div["tag"])
	}
	text := div["text"].(map[string]any)
	if text["content"] != "Hello world" {
		t.Errorf("expected content=Hello world, got %v", text["content"])
	}
}

func TestRenderStructured_ImageSection(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Images",
		Sections: []core.StructuredSection{
			{Type: "image", ImageSection: &core.ImageSection{URL: "img_key_123", AltText: "screenshot"}},
		},
	}
	result, err := renderStructured(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	elements := parsed["elements"].([]any)
	img := elements[0].(map[string]any)
	if img["tag"] != "img" {
		t.Errorf("expected tag=img, got %v", img["tag"])
	}
	if img["img_key"] != "img_key_123" {
		t.Errorf("expected img_key=img_key_123, got %v", img["img_key"])
	}
}

func TestRenderStructured_DividerSection(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Divider",
		Sections: []core.StructuredSection{
			{Type: "text", TextSection: &core.TextSection{Body: "before"}},
			{Type: "divider", DividerSection: &core.DividerSection{}},
			{Type: "text", TextSection: &core.TextSection{Body: "after"}},
		},
	}
	result, err := renderStructured(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	elements := parsed["elements"].([]any)
	if len(elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(elements))
	}
	hr := elements[1].(map[string]any)
	if hr["tag"] != "hr" {
		t.Errorf("expected tag=hr, got %v", hr["tag"])
	}
}

func TestRenderStructured_ContextSection(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Context",
		Sections: []core.StructuredSection{
			{Type: "context", ContextSection: &core.ContextSection{Elements: []string{"hint 1", "hint 2"}}},
		},
	}
	result, err := renderStructured(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	elements := parsed["elements"].([]any)
	note := elements[0].(map[string]any)
	if note["tag"] != "note" {
		t.Errorf("expected tag=note, got %v", note["tag"])
	}
	noteElements := note["elements"].([]any)
	if len(noteElements) != 2 {
		t.Errorf("expected 2 note elements, got %d", len(noteElements))
	}
}

func TestRenderStructured_FieldsSection(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Fields",
		Sections: []core.StructuredSection{
			{Type: "fields", FieldsSection: &core.FieldsSection{
				Fields: []core.StructuredField{
					{Label: "Status", Value: "Running"},
					{Label: "Agent", Value: "claude-1"},
					{Label: "Duration", Value: "5m"},
				},
			}},
		},
	}
	result, err := renderStructured(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	elements := parsed["elements"].([]any)
	// 3 fields -> 2 column_sets (first has 2, second has 1)
	if len(elements) != 2 {
		t.Fatalf("expected 2 column_set elements, got %d", len(elements))
	}
	cs := elements[0].(map[string]any)
	if cs["tag"] != "column_set" {
		t.Errorf("expected tag=column_set, got %v", cs["tag"])
	}
	columns := cs["columns"].([]any)
	if len(columns) != 2 {
		t.Errorf("expected 2 columns in first set, got %d", len(columns))
	}
}

func TestRenderStructured_ActionsSection(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Actions",
		Sections: []core.StructuredSection{
			{Type: "actions", ActionsSection: &core.ActionsSection{
				Actions: []core.StructuredAction{
					{ID: "act:approve:123", Label: "Approve", Style: core.ActionStylePrimary},
					{Label: "View", URL: "https://example.com"},
				},
			}},
		},
	}
	result, err := renderStructured(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	elements := parsed["elements"].([]any)
	actionBlock := elements[0].(map[string]any)
	if actionBlock["tag"] != "action" {
		t.Errorf("expected tag=action, got %v", actionBlock["tag"])
	}
	actions := actionBlock["actions"].([]any)
	if len(actions) != 2 {
		t.Errorf("expected 2 buttons, got %d", len(actions))
	}
	btn1 := actions[0].(map[string]any)
	if btn1["type"] != "primary" {
		t.Errorf("expected type=primary, got %v", btn1["type"])
	}
	btn2 := actions[1].(map[string]any)
	if btn2["url"] != "https://example.com" {
		t.Errorf("expected url, got %v", btn2["url"])
	}
}

func TestRenderStructured_LegacyPath(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Legacy",
		Body:  "Some markdown body",
		Fields: []core.StructuredField{
			{Label: "Key", Value: "Val"},
		},
		Actions: []core.StructuredAction{
			{ID: "act:do:1", Label: "Do It", Style: core.ActionStyleDanger},
		},
	}
	result, err := renderStructured(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed map[string]any
	json.Unmarshal([]byte(result), &parsed)
	elements := parsed["elements"].([]any)
	// body div + column_set + hr + action
	if len(elements) != 4 {
		t.Fatalf("expected 4 elements, got %d", len(elements))
	}
}

func TestRenderStructured_NilReturnsError(t *testing.T) {
	_, err := renderStructured(nil)
	if err == nil {
		t.Fatal("expected error for nil message")
	}
}

func TestRenderStructured_EmptyReturnsError(t *testing.T) {
	_, err := renderStructured(&core.StructuredMessage{})
	if err == nil {
		t.Fatal("expected error for empty message")
	}
}
