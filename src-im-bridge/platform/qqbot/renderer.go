package qqbot

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

// renderStructuredAsMarkdown converts a StructuredMessage into QQ Bot markdown
// text. Returns an empty string when the message has no renderable sections,
// which signals the caller to fall back to plain-text delivery.
func renderStructuredAsMarkdown(message *core.StructuredMessage) string {
	if message == nil {
		return ""
	}

	if len(message.Sections) > 0 {
		return renderSectionsAsMarkdown(message.Sections)
	}

	// Legacy field-based structured message
	var parts []string
	if title := strings.TrimSpace(message.Title); title != "" {
		parts = append(parts, "**"+title+"**")
	}
	if body := strings.TrimSpace(message.Body); body != "" {
		parts = append(parts, body)
	}
	for _, field := range message.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		if label == "" && value == "" {
			continue
		}
		if label == "" {
			parts = append(parts, value)
			continue
		}
		parts = append(parts, "**"+label+":** "+value)
	}
	for _, action := range message.Actions {
		label := strings.TrimSpace(action.Label)
		url := strings.TrimSpace(action.URL)
		switch {
		case label != "" && url != "":
			parts = append(parts, fmt.Sprintf("[%s](%s)", label, url))
		case label != "":
			parts = append(parts, label)
		case url != "":
			parts = append(parts, url)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func renderSectionsAsMarkdown(sections []core.StructuredSection) string {
	var parts []string
	for _, section := range sections {
		if line := renderSectionAsMarkdown(section); line != "" {
			parts = append(parts, line)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func renderSectionAsMarkdown(section core.StructuredSection) string {
	switch strings.ToLower(strings.TrimSpace(section.Type)) {
	case core.StructuredSectionTypeText:
		if section.TextSection != nil {
			return strings.TrimSpace(section.TextSection.Body)
		}
	case core.StructuredSectionTypeFields:
		if section.FieldsSection != nil {
			return renderFieldsAsMarkdown(section.FieldsSection.Fields)
		}
	case core.StructuredSectionTypeDivider:
		return "---"
	case core.StructuredSectionTypeContext:
		if section.ContextSection != nil && len(section.ContextSection.Elements) > 0 {
			trimmed := make([]string, 0, len(section.ContextSection.Elements))
			for _, elem := range section.ContextSection.Elements {
				if e := strings.TrimSpace(elem); e != "" {
					trimmed = append(trimmed, e)
				}
			}
			return strings.Join(trimmed, " | ")
		}
	case core.StructuredSectionTypeImage:
		if section.ImageSection != nil {
			alt := strings.TrimSpace(section.ImageSection.AltText)
			url := strings.TrimSpace(section.ImageSection.URL)
			if url != "" {
				if alt == "" {
					alt = "image"
				}
				return fmt.Sprintf("![%s](%s)", alt, url)
			}
		}
	case core.StructuredSectionTypeActions:
		if section.ActionsSection != nil {
			return renderActionsAsMarkdown(section.ActionsSection.Actions)
		}
	}
	return ""
}

func renderFieldsAsMarkdown(fields []core.StructuredField) string {
	var lines []string
	for _, field := range fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		if label == "" && value == "" {
			continue
		}
		if label == "" {
			lines = append(lines, value)
			continue
		}
		lines = append(lines, "**"+label+":** "+value)
	}
	return strings.Join(lines, "\n")
}

func renderActionsAsMarkdown(actions []core.StructuredAction) string {
	var lines []string
	for _, action := range actions {
		label := strings.TrimSpace(action.Label)
		url := strings.TrimSpace(action.URL)
		switch {
		case label != "" && url != "":
			lines = append(lines, fmt.Sprintf("[%s](%s)", label, url))
		case label != "":
			lines = append(lines, label)
		case url != "":
			lines = append(lines, url)
		}
	}
	return strings.Join(lines, "\n")
}
