package wecom

import (
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func renderStructuredSections(sections []core.StructuredSection) *core.WeComCardPayload {
	lines := make([]string, 0, len(sections))
	article := core.WeComArticle{}
	var actionURL string

	for _, section := range sections {
		switch section.Type {
		case core.StructuredSectionTypeImage:
			if section.ImageSection != nil {
				article.PicURL = strings.TrimSpace(section.ImageSection.URL)
			}
			if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
				lines = append(lines, fallback)
			}
		case core.StructuredSectionTypeActions:
			if section.ActionsSection == nil {
				continue
			}
			for _, action := range section.ActionsSection.Actions {
				if url := strings.TrimSpace(action.URL); url != "" {
					actionURL = firstNonEmpty(actionURL, url)
				}
			}
		default:
			if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
				lines = append(lines, fallback)
			}
		}
	}

	title := "AgentForge Update"
	description := strings.TrimSpace(strings.Join(lines, "\n"))
	if len(lines) > 0 {
		title = firstNonEmpty(lines[0], title)
	}
	article.Title = title
	article.Description = description
	article.URL = actionURL

	return &core.WeComCardPayload{
		CardType:    core.WeComCardTypeNews,
		Title:       title,
		Description: description,
		URL:         actionURL,
		Articles:    []core.WeComArticle{article},
	}
}

// renderCardToWeComTextCard converts a core.Card to a WeCom textcard message payload.
func renderCardToWeComTextCard(card *core.Card) map[string]any {
	title := strings.TrimSpace(card.Title)
	if title == "" {
		title = "AgentForge"
	}

	var descParts []string
	for _, field := range card.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		if label != "" && value != "" {
			descParts = append(descParts, label+": "+value)
		} else if value != "" {
			descParts = append(descParts, value)
		}
	}
	description := strings.Join(descParts, "\n")

	var cardURL string
	for _, button := range card.Buttons {
		action := strings.TrimSpace(button.Action)
		if strings.HasPrefix(action, "link:") {
			cardURL = strings.TrimSpace(strings.TrimPrefix(action, "link:"))
			break
		}
	}

	payload := map[string]any{
		"msgtype": "textcard",
		"textcard": map[string]any{
			"title":       title,
			"description": description,
			"url":         cardURL,
		},
	}
	return payload
}

// cardFallbackText renders a card as plain text for platforms without card support.
func cardFallbackText(card *core.Card) string {
	if card == nil {
		return ""
	}
	var parts []string
	if title := strings.TrimSpace(card.Title); title != "" {
		parts = append(parts, title)
	}
	for _, field := range card.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		if label != "" && value != "" {
			parts = append(parts, label+": "+value)
		} else if value != "" {
			parts = append(parts, value)
		}
	}
	for _, button := range card.Buttons {
		text := strings.TrimSpace(button.Text)
		action := strings.TrimSpace(button.Action)
		if text != "" && action != "" {
			parts = append(parts, "["+text+"] "+action)
		} else if text != "" {
			parts = append(parts, "["+text+"]")
		}
	}
	return strings.Join(parts, "\n")
}
