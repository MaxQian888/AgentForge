package core

import (
	"strings"
)

type ActionStyle string

const (
	ActionStyleDefault ActionStyle = "default"
	ActionStylePrimary ActionStyle = "primary"
	ActionStyleDanger  ActionStyle = "danger"
)

type StructuredField struct {
	Label string
	Value string
}

type StructuredAction struct {
	ID    string
	Label string
	URL   string
	Style ActionStyle
}

type StructuredMessage struct {
	Title   string
	Body    string
	Fields  []StructuredField
	Actions []StructuredAction
}

func SelectStructuredRenderer(metadata PlatformMetadata, message *StructuredMessage) StructuredSurface {
	if message == nil {
		return StructuredSurfaceNone
	}
	switch metadata.Capabilities.StructuredSurface {
	case StructuredSurfaceBlocks, StructuredSurfaceCards, StructuredSurfaceInlineKeyboard:
		return metadata.Capabilities.StructuredSurface
	default:
		if metadata.Capabilities.SupportsRichMessages {
			return StructuredSurfaceCards
		}
		return StructuredSurfaceNone
	}
}

func (m *StructuredMessage) FallbackText() string {
	if m == nil {
		return ""
	}
	lines := make([]string, 0, 2+len(m.Fields)+len(m.Actions))
	if title := strings.TrimSpace(m.Title); title != "" {
		lines = append(lines, title)
	}
	if body := strings.TrimSpace(m.Body); body != "" {
		lines = append(lines, body)
	}
	for _, field := range m.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		if label == "" && value == "" {
			continue
		}
		if label == "" {
			lines = append(lines, value)
			continue
		}
		lines = append(lines, label+": "+value)
	}
	for _, action := range m.Actions {
		label := strings.TrimSpace(action.Label)
		url := strings.TrimSpace(action.URL)
		switch {
		case label != "" && url != "":
			lines = append(lines, label+": "+url)
		case label != "":
			lines = append(lines, label)
		case url != "":
			lines = append(lines, url)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (m *StructuredMessage) LegacyCard() *Card {
	if m == nil {
		return nil
	}
	card := NewCard().SetTitle(strings.TrimSpace(m.Title))
	for _, field := range m.Fields {
		card.AddField(strings.TrimSpace(field.Label), strings.TrimSpace(field.Value))
	}
	for _, action := range m.Actions {
		if strings.TrimSpace(action.URL) != "" {
			card.AddButton(strings.TrimSpace(action.Label), "link:"+strings.TrimSpace(action.URL))
			continue
		}
		switch action.Style {
		case ActionStylePrimary:
			card.AddPrimaryButton(strings.TrimSpace(action.Label), strings.TrimSpace(action.ID))
		case ActionStyleDanger:
			card.AddDangerButton(strings.TrimSpace(action.Label), strings.TrimSpace(action.ID))
		default:
			card.AddButton(strings.TrimSpace(action.Label), strings.TrimSpace(action.ID))
		}
	}
	return card
}
