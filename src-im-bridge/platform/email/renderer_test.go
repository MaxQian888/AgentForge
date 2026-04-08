package email

import (
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestRenderEmailHTML_NilMessage(t *testing.T) {
	html, plain := renderEmailHTML(nil)
	if html != "" || plain != "" {
		t.Fatal("nil message should produce empty output")
	}
}

func TestRenderEmailHTML_TitleAndBody(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Test Title",
		Body:  "Hello world",
	}
	html, plain := renderEmailHTML(msg)

	if !strings.Contains(html, "<h2>Test Title</h2>") {
		t.Error("HTML should contain title as h2")
	}
	if !strings.Contains(html, "<p>Hello world</p>") {
		t.Error("HTML should contain body as p")
	}
	if !strings.Contains(plain, "Test Title") {
		t.Error("plain text should contain title")
	}
	if !strings.Contains(plain, "Hello world") {
		t.Error("plain text should contain body")
	}
}

func TestRenderEmailHTML_Fields(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Status",
		Fields: []core.StructuredField{
			{Label: "Task", Value: "Deploy"},
			{Label: "Status", Value: "Complete"},
		},
	}
	html, _ := renderEmailHTML(msg)

	if !strings.Contains(html, "<table") {
		t.Error("HTML should contain a table for fields")
	}
	if !strings.Contains(html, "Task") || !strings.Contains(html, "Deploy") {
		t.Error("HTML should contain field label and value")
	}
}

func TestRenderEmailHTML_Actions(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Review",
		Actions: []core.StructuredAction{
			{Label: "View", URL: "https://example.com/view"},
			{Label: "No URL"},
		},
	}
	html, _ := renderEmailHTML(msg)

	if !strings.Contains(html, `href="https://example.com/view"`) {
		t.Error("HTML should contain action link")
	}
	if !strings.Contains(html, "View") {
		t.Error("HTML should contain action label")
	}
}

func TestRenderFormattedTextEmail_HTML(t *testing.T) {
	msg := &core.FormattedText{
		Content: "<b>Bold</b> text",
		Format:  core.TextFormatHTML,
	}
	html, plain := renderFormattedTextEmail(msg)

	if !strings.Contains(html, "<b>Bold</b> text") {
		t.Error("HTML format should pass through")
	}
	if !strings.Contains(plain, "Bold") {
		t.Error("plain text should contain stripped text")
	}
}

func TestRenderFormattedTextEmail_PlainText(t *testing.T) {
	msg := &core.FormattedText{
		Content: "Just plain text",
		Format:  core.TextFormatPlainText,
	}
	html, plain := renderFormattedTextEmail(msg)

	if !strings.Contains(html, "<pre") {
		t.Error("plain text should be wrapped in pre tag")
	}
	if plain != "Just plain text" {
		t.Errorf("plain text should pass through, got %q", plain)
	}
}

func TestRenderFormattedTextEmail_Nil(t *testing.T) {
	html, plain := renderFormattedTextEmail(nil)
	if html != "" || plain != "" {
		t.Fatal("nil message should produce empty output")
	}
}

func TestEmailHTMLTemplate_Structure(t *testing.T) {
	result := emailHTMLTemplate("My Title", "<p>Content</p>")

	if !strings.Contains(result, "<!DOCTYPE html>") {
		t.Error("should be a full HTML document")
	}
	if !strings.Contains(result, "<title>My Title</title>") {
		t.Error("should include title")
	}
	if !strings.Contains(result, "<p>Content</p>") {
		t.Error("should include body content")
	}
	if !strings.Contains(result, "AgentForge") {
		t.Error("should include AgentForge footer")
	}
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"<b>bold</b>", "bold"},
		{"<p>hello</p><p>world</p>", "helloworld"},
		{"no tags", "no tags"},
		{"", ""},
	}

	for _, tt := range tests {
		got := stripHTMLTags(tt.input)
		if got != tt.expected {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEmailSubjectFromStructured(t *testing.T) {
	tests := []struct {
		msg      *core.StructuredMessage
		expected string
	}{
		{nil, "AgentForge Notification"},
		{&core.StructuredMessage{}, "AgentForge Notification"},
		{&core.StructuredMessage{Title: "Deploy Alert"}, "Deploy Alert"},
	}

	for _, tt := range tests {
		got := emailSubjectFromStructured(tt.msg)
		if got != tt.expected {
			t.Errorf("emailSubjectFromStructured() = %q, want %q", got, tt.expected)
		}
	}
}

func TestRenderEmailHTML_EmptyFields(t *testing.T) {
	msg := &core.StructuredMessage{
		Title: "Test",
		Fields: []core.StructuredField{
			{Label: "", Value: ""},
			{Label: "Valid", Value: "Field"},
		},
	}
	html, _ := renderEmailHTML(msg)
	if strings.Count(html, "<tr>") != 1 {
		t.Error("empty fields should be skipped")
	}
}
