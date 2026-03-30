package dingtalk

import (
	"strings"

	"github.com/agentforge/im-bridge/core"
)

func renderStructuredSections(sections []core.StructuredSection) *core.DingTalkCardPayload {
	lines := make([]string, 0, len(sections))
	buttons := make([]core.DingTalkCardButton, 0)

	for _, section := range sections {
		switch section.Type {
		case core.StructuredSectionTypeActions:
			if section.ActionsSection == nil {
				continue
			}
			for _, action := range section.ActionsSection.Actions {
				if label := strings.TrimSpace(action.Label); label != "" {
					if url := strings.TrimSpace(action.URL); url != "" {
						buttons = append(buttons, core.DingTalkCardButton{
							Title:     label,
							ActionURL: url,
						})
					} else if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
						lines = append(lines, fallback)
					}
				}
			}
		default:
			if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
				lines = append(lines, fallback)
			}
		}
	}

	title := "AgentForge Update"
	if len(lines) > 0 {
		title = firstNonEmpty(lines[0], title)
	}
	return &core.DingTalkCardPayload{
		CardType: core.DingTalkCardTypeActionCard,
		Title:    title,
		Markdown: strings.TrimSpace(strings.Join(lines, "\n\n")),
		Buttons:  buttons,
	}
}
