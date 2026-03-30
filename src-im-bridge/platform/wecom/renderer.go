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
