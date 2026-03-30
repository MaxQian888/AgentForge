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

const (
	StructuredSectionTypeText    = "text"
	StructuredSectionTypeImage   = "image"
	StructuredSectionTypeDivider = "divider"
	StructuredSectionTypeContext = "context"
	StructuredSectionTypeFields  = "fields"
	StructuredSectionTypeActions = "actions"
)

type StructuredSection struct {
	Type           string          `json:"type,omitempty"`
	TextSection    *TextSection    `json:"textSection,omitempty"`
	ImageSection   *ImageSection   `json:"imageSection,omitempty"`
	DividerSection *DividerSection `json:"dividerSection,omitempty"`
	ContextSection *ContextSection `json:"contextSection,omitempty"`
	FieldsSection  *FieldsSection  `json:"fieldsSection,omitempty"`
	ActionsSection *ActionsSection `json:"actionsSection,omitempty"`
}

type TextSection struct {
	Body string `json:"body,omitempty"`
}

type ImageSection struct {
	URL     string `json:"url,omitempty"`
	AltText string `json:"altText,omitempty"`
}

type DividerSection struct{}

type ContextSection struct {
	Elements []string `json:"elements,omitempty"`
}

type FieldsSection struct {
	Fields []StructuredField `json:"fields,omitempty"`
}

type ActionsSection struct {
	Actions       []StructuredAction `json:"actions,omitempty"`
	ButtonsPerRow int                `json:"buttonsPerRow,omitempty"`
}

type StructuredMessage struct {
	Title    string
	Body     string
	Fields   []StructuredField
	Actions  []StructuredAction
	Sections []StructuredSection
}

func SelectStructuredRenderer(metadata PlatformMetadata, message *StructuredMessage) StructuredSurface {
	if message == nil {
		return StructuredSurfaceNone
	}
	switch metadata.Capabilities.StructuredSurface {
	case StructuredSurfaceBlocks, StructuredSurfaceCards, StructuredSurfaceInlineKeyboard, StructuredSurfaceActionCard, StructuredSurfaceComponents:
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
	if len(m.Sections) > 0 {
		lines := make([]string, 0, len(m.Sections))
		for _, section := range m.Sections {
			if line := strings.TrimSpace(section.FallbackText()); line != "" {
				lines = append(lines, line)
			}
		}
		return strings.TrimSpace(strings.Join(lines, "\n"))
	}
	lines := make([]string, 0, 2+len(m.Fields)+len(m.Actions))
	if title := strings.TrimSpace(m.Title); title != "" {
		lines = append(lines, title)
	}
	if body := strings.TrimSpace(m.Body); body != "" {
		lines = append(lines, body)
	}
	lines = append(lines, fallbackLinesFromFields(m.Fields)...)
	lines = append(lines, fallbackLinesFromActions(m.Actions)...)
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

func (s StructuredSection) FallbackText() string {
	switch strings.ToLower(strings.TrimSpace(s.Type)) {
	case StructuredSectionTypeText:
		if s.TextSection != nil {
			return s.TextSection.FallbackText()
		}
	case StructuredSectionTypeImage:
		if s.ImageSection != nil {
			return s.ImageSection.FallbackText()
		}
	case StructuredSectionTypeDivider:
		if s.DividerSection != nil {
			return s.DividerSection.FallbackText()
		}
	case StructuredSectionTypeContext:
		if s.ContextSection != nil {
			return s.ContextSection.FallbackText()
		}
	case StructuredSectionTypeFields:
		if s.FieldsSection != nil {
			return s.FieldsSection.FallbackText()
		}
	case StructuredSectionTypeActions:
		if s.ActionsSection != nil {
			return s.ActionsSection.FallbackText()
		}
	}
	return ""
}

func (s *TextSection) FallbackText() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(stripMarkdown(s.Body))
}

func (s *ImageSection) FallbackText() string {
	if s == nil {
		return ""
	}
	return fallbackImageText(s.AltText, s.URL)
}

func (s *DividerSection) FallbackText() string {
	if s == nil {
		return ""
	}
	return "---"
}

func (s *ContextSection) FallbackText() string {
	if s == nil {
		return ""
	}
	values := make([]string, 0, len(s.Elements))
	for _, element := range s.Elements {
		if trimmed := strings.TrimSpace(stripMarkdown(element)); trimmed != "" {
			values = append(values, trimmed)
		}
	}
	return strings.Join(values, " | ")
}

func (s *FieldsSection) FallbackText() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(strings.Join(fallbackLinesFromFields(s.Fields), "\n"))
}

func (s *ActionsSection) FallbackText() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(strings.Join(fallbackLinesFromActions(s.Actions), "\n"))
}

func fallbackLinesFromFields(fields []StructuredField) []string {
	lines := make([]string, 0, len(fields))
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
		lines = append(lines, label+": "+value)
	}
	return lines
}

func fallbackLinesFromActions(actions []StructuredAction) []string {
	lines := make([]string, 0, len(actions))
	for _, action := range actions {
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
	return lines
}
