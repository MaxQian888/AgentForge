package slack

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
	goslack "github.com/slack-go/slack"
)

func renderStructuredSections(sections []core.StructuredSection) []goslack.Block {
	blocks := make([]goslack.Block, 0, len(sections))
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
			blocks = append(blocks, goslack.NewSectionBlock(
				goslack.NewTextBlockObject(goslack.MarkdownType, body, false, false),
				nil,
				nil,
			))
		case core.StructuredSectionTypeImage:
			if section.ImageSection == nil {
				continue
			}
			url := strings.TrimSpace(section.ImageSection.URL)
			if url == "" {
				continue
			}
			blocks = append(blocks, goslack.NewImageBlock(
				url,
				strings.TrimSpace(section.ImageSection.AltText),
				"",
				nil,
			))
		case core.StructuredSectionTypeDivider:
			blocks = append(blocks, goslack.NewDividerBlock())
		case core.StructuredSectionTypeContext:
			if section.ContextSection == nil || len(section.ContextSection.Elements) == 0 {
				continue
			}
			elements := make([]goslack.MixedElement, 0, len(section.ContextSection.Elements))
			for _, element := range section.ContextSection.Elements {
				if trimmed := strings.TrimSpace(element); trimmed != "" {
					elements = append(elements, goslack.NewTextBlockObject(goslack.MarkdownType, trimmed, false, false))
				}
			}
			if len(elements) == 0 {
				continue
			}
			blocks = append(blocks, goslack.NewContextBlock("", elements...))
		case core.StructuredSectionTypeFields:
			if section.FieldsSection == nil || len(section.FieldsSection.Fields) == 0 {
				continue
			}
			fields := make([]*goslack.TextBlockObject, 0, len(section.FieldsSection.Fields))
			for _, field := range section.FieldsSection.Fields {
				value := strings.TrimSpace(field.Value)
				label := strings.TrimSpace(field.Label)
				switch {
				case label != "" && value != "":
					fields = append(fields, goslack.NewTextBlockObject(goslack.MarkdownType, fmt.Sprintf("*%s*\n%s", label, value), false, false))
				case value != "":
					fields = append(fields, goslack.NewTextBlockObject(goslack.MarkdownType, value, false, false))
				case label != "":
					fields = append(fields, goslack.NewTextBlockObject(goslack.MarkdownType, label, false, false))
				}
			}
			if len(fields) == 0 {
				continue
			}
			blocks = append(blocks, goslack.NewSectionBlock(nil, fields, nil))
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
				elements := make([]goslack.BlockElement, 0, end-start)
				for index, action := range section.ActionsSection.Actions[start:end] {
					label := strings.TrimSpace(action.Label)
					if label == "" {
						continue
					}
					element := goslack.NewButtonBlockElement(
						fmt.Sprintf("section-action-%d-%d", start, index),
						strings.TrimSpace(action.ID),
						goslack.NewTextBlockObject(goslack.PlainTextType, label, false, false),
					)
					if url := strings.TrimSpace(action.URL); url != "" {
						element.URL = url
					}
					element.Style = normalizeButtonStyle(string(action.Style))
					elements = append(elements, element)
				}
				if len(elements) > 0 {
					blocks = append(blocks, goslack.NewActionBlock(fmt.Sprintf("section-actions-%d", start), elements...))
				}
			}
		default:
			if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
				blocks = append(blocks, goslack.NewSectionBlock(
					goslack.NewTextBlockObject(goslack.MarkdownType, fallback, false, false),
					nil,
					nil,
				))
			}
		}
	}
	return blocks
}
