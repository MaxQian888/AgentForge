package feishu

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

// renderStructuredMessage converts a StructuredMessage to a Feishu interactive
// card JSON string. It supports section-based rendering for text, image,
// divider, context, fields, and actions — matching the capability of the Slack
// and Discord renderers.
func renderStructuredMessage(message *core.StructuredMessage) (string, error) {
	if message == nil {
		return "", fmt.Errorf("structured message is required")
	}

	var elements []map[string]any

	if len(message.Sections) > 0 {
		elements = renderStructuredSections(message.Sections)
	} else {
		elements = renderLegacySections(message)
	}

	if len(elements) == 0 {
		return "", fmt.Errorf("structured message produced no card elements")
	}

	payload := map[string]any{
		"config":   map[string]any{"wide_screen_mode": true},
		"header":   feishuCardHeader(message.Title),
		"elements": elements,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// renderStructuredSections maps each StructuredSection to Feishu card elements.
func renderStructuredSections(sections []core.StructuredSection) []map[string]any {
	elements := make([]map[string]any, 0, len(sections))
	for _, section := range sections {
		switch strings.ToLower(strings.TrimSpace(section.Type)) {
		case core.StructuredSectionTypeText:
			if section.TextSection == nil {
				continue
			}
			body := strings.TrimSpace(section.TextSection.Body)
			if body == "" {
				continue
			}
			elements = append(elements, map[string]any{
				"tag":  "div",
				"text": map[string]any{"tag": "lark_md", "content": body},
			})
		case core.StructuredSectionTypeImage:
			if section.ImageSection == nil {
				continue
			}
			url := strings.TrimSpace(section.ImageSection.URL)
			if url == "" {
				continue
			}
			alt := strings.TrimSpace(section.ImageSection.AltText)
			if alt == "" {
				alt = "image"
			}
			elements = append(elements, map[string]any{
				"tag":      "img",
				"img_key":  url,
				"alt":      map[string]any{"tag": "plain_text", "content": alt},
			})
		case core.StructuredSectionTypeDivider:
			elements = append(elements, map[string]any{"tag": "hr"})
		case core.StructuredSectionTypeContext:
			if section.ContextSection == nil || len(section.ContextSection.Elements) == 0 {
				continue
			}
			noteElements := make([]map[string]any, 0, len(section.ContextSection.Elements))
			for _, elem := range section.ContextSection.Elements {
				if trimmed := strings.TrimSpace(elem); trimmed != "" {
					noteElements = append(noteElements, map[string]any{
						"tag":     "plain_text",
						"content": trimmed,
					})
				}
			}
			if len(noteElements) == 0 {
				continue
			}
			elements = append(elements, map[string]any{
				"tag":      "note",
				"elements": noteElements,
			})
		case core.StructuredSectionTypeFields:
			if section.FieldsSection == nil || len(section.FieldsSection.Fields) == 0 {
				continue
			}
			elements = append(elements, renderFieldsAsColumns(section.FieldsSection.Fields)...)
		case core.StructuredSectionTypeActions:
			if section.ActionsSection == nil || len(section.ActionsSection.Actions) == 0 {
				continue
			}
			perRow := section.ActionsSection.ButtonsPerRow
			if perRow <= 0 {
				perRow = len(section.ActionsSection.Actions)
			}
			for start := 0; start < len(section.ActionsSection.Actions); start += perRow {
				end := start + perRow
				if end > len(section.ActionsSection.Actions) {
					end = len(section.ActionsSection.Actions)
				}
				buttons := renderButtons(section.ActionsSection.Actions[start:end])
				if len(buttons) > 0 {
					elements = append(elements, map[string]any{
						"tag":     "action",
						"actions": buttons,
					})
				}
			}
		default:
			if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
				elements = append(elements, map[string]any{
					"tag":  "div",
					"text": map[string]any{"tag": "lark_md", "content": fallback},
				})
			}
		}
	}
	return elements
}

// renderLegacySections converts the flat Title/Body/Fields/Actions structure
// into Feishu card elements for backward compatibility.
func renderLegacySections(message *core.StructuredMessage) []map[string]any {
	elements := make([]map[string]any, 0, 4)

	if body := strings.TrimSpace(message.Body); body != "" {
		elements = append(elements, map[string]any{
			"tag":  "div",
			"text": map[string]any{"tag": "lark_md", "content": body},
		})
	}

	if len(message.Fields) > 0 {
		elements = append(elements, renderFieldsAsColumns(message.Fields)...)
	}

	if len(message.Actions) > 0 {
		elements = append(elements, map[string]any{"tag": "hr"})
		buttons := renderButtons(message.Actions)
		if len(buttons) > 0 {
			elements = append(elements, map[string]any{
				"tag":     "action",
				"actions": buttons,
			})
		}
	}

	return elements
}

// renderFieldsAsColumns renders structured fields as 2-column layouts using
// Feishu's column_set element.
func renderFieldsAsColumns(fields []core.StructuredField) []map[string]any {
	elements := make([]map[string]any, 0, (len(fields)+1)/2)
	for i := 0; i < len(fields); i += 2 {
		columns := make([]map[string]any, 0, 2)
		columns = append(columns, fieldToColumn(fields[i]))
		if i+1 < len(fields) {
			columns = append(columns, fieldToColumn(fields[i+1]))
		}
		elements = append(elements, map[string]any{
			"tag":              "column_set",
			"flex_mode":        "bisect",
			"background_style": "default",
			"columns":          columns,
		})
	}
	return elements
}

func fieldToColumn(field core.StructuredField) map[string]any {
	label := strings.TrimSpace(field.Label)
	value := strings.TrimSpace(field.Value)
	content := value
	if label != "" && value != "" {
		content = fmt.Sprintf("**%s**\n%s", label, value)
	} else if label != "" {
		content = "**" + label + "**"
	}
	return map[string]any{
		"tag":            "column",
		"width":          "weighted",
		"weight":         1,
		"vertical_align": "top",
		"elements": []map[string]any{
			{
				"tag":  "div",
				"text": map[string]any{"tag": "lark_md", "content": content},
			},
		},
	}
}

// renderButtons converts structured actions to Feishu button elements.
func renderButtons(actions []core.StructuredAction) []map[string]any {
	buttons := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		label := strings.TrimSpace(action.Label)
		if label == "" {
			continue
		}
		btn := map[string]any{
			"tag":  "button",
			"text": map[string]any{"tag": "plain_text", "content": label},
			"type": normalizeButtonStyle(string(action.Style)),
		}
		if url := strings.TrimSpace(action.URL); url != "" {
			btn["url"] = url
		} else if id := strings.TrimSpace(action.ID); id != "" {
			btn["value"] = map[string]any{"action": id}
		}
		buttons = append(buttons, btn)
	}
	return buttons
}

func feishuCardHeader(title string) map[string]any {
	return map[string]any{
		"title": map[string]any{
			"tag":     "plain_text",
			"content": strings.TrimSpace(title),
		},
	}
}
