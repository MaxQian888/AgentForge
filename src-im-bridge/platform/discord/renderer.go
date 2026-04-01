package discord

import (
	"fmt"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

const defaultCardEmbedColor = 0x5865F2 // Discord blurple

func renderCardToDiscordMessage(card *core.Card) discordOutgoingMessage {
	if card == nil {
		return discordOutgoingMessage{}
	}

	embed := discordEmbed{
		Title: strings.TrimSpace(card.Title),
		Color: defaultCardEmbedColor,
	}
	for _, field := range card.Fields {
		name := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		if name == "" && value == "" {
			continue
		}
		if name == "" {
			name = "Field"
		}
		embed.Fields = append(embed.Fields, discordEmbedField{
			Name:   name,
			Value:  value,
			Inline: true,
		})
	}

	var components []discordComponent
	if len(card.Buttons) > 0 {
		rowButtons := make([]discordComponent, 0, len(card.Buttons))
		for i, button := range card.Buttons {
			label := strings.TrimSpace(button.Text)
			if label == "" {
				continue
			}
			component := discordComponent{
				Type:  componentTypeButton,
				Label: label,
			}
			action := strings.TrimSpace(button.Action)
			if strings.HasPrefix(action, "link:") {
				component.Style = componentStyleLink
				component.URL = strings.TrimPrefix(action, "link:")
			} else {
				component.Style = cardButtonStyle(button.Style)
				if action != "" {
					component.CustomID = action
				} else {
					component.CustomID = fmt.Sprintf("card-btn-%d", i)
				}
			}
			rowButtons = append(rowButtons, component)
		}
		if len(rowButtons) > 0 {
			components = append(components, discordComponent{
				Type:       componentTypeActionRow,
				Components: rowButtons,
			})
		}
	}

	fallback := strings.TrimSpace(card.Title)
	return discordOutgoingMessage{
		Content:    fallback,
		Embeds:     []discordEmbed{embed},
		Components: components,
	}
}

func cardButtonStyle(style string) int {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "primary":
		return componentStylePrimary
	case "danger":
		return componentStyleDanger
	default:
		return componentStyleSecondary
	}
}

func renderStructuredSections(sections []core.StructuredSection) (discordEmbed, []discordComponent) {
	embed := discordEmbed{}
	var descriptionParts []string
	var components []discordComponent

	for _, section := range sections {
		switch strings.ToLower(strings.TrimSpace(section.Type)) {
		case core.StructuredSectionTypeText:
			if section.TextSection == nil {
				continue
			}
			if body := strings.TrimSpace(section.TextSection.Body); body != "" {
				descriptionParts = append(descriptionParts, body)
			}
		case core.StructuredSectionTypeDivider:
			descriptionParts = append(descriptionParts, "---")
		case core.StructuredSectionTypeContext:
			if section.ContextSection == nil || len(section.ContextSection.Elements) == 0 {
				continue
			}
			values := make([]string, 0, len(section.ContextSection.Elements))
			for _, element := range section.ContextSection.Elements {
				if trimmed := strings.TrimSpace(element); trimmed != "" {
					values = append(values, trimmed)
				}
			}
			if len(values) > 0 {
				descriptionParts = append(descriptionParts, strings.Join(values, " | "))
			}
		case core.StructuredSectionTypeFields:
			if section.FieldsSection == nil {
				continue
			}
			for _, field := range section.FieldsSection.Fields {
				name := strings.TrimSpace(field.Label)
				value := strings.TrimSpace(field.Value)
				if name == "" && value == "" {
					continue
				}
				if name == "" {
					name = "Field"
				}
				embed.Fields = append(embed.Fields, discordEmbedField{
					Name:   name,
					Value:  value,
					Inline: true,
				})
			}
		case core.StructuredSectionTypeImage:
			if section.ImageSection == nil {
				continue
			}
			if embed.Image.URL == "" {
				embed.Image.URL = strings.TrimSpace(section.ImageSection.URL)
			} else if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
				descriptionParts = append(descriptionParts, fallback)
			}
		case core.StructuredSectionTypeActions:
			if section.ActionsSection == nil {
				continue
			}
			components = append(components, renderDiscordActionRows(fromStructuredActions(section.ActionsSection.Actions, section.ActionsSection.ButtonsPerRow))...)
		default:
			if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
				descriptionParts = append(descriptionParts, fallback)
			}
		}
	}

	embed.Description = strings.TrimSpace(strings.Join(descriptionParts, "\n"))
	return embed, components
}

func renderDiscordActionRows(rows []core.DiscordActionRow) []discordComponent {
	components := make([]discordComponent, 0, len(rows))
	for rowIndex, row := range rows {
		rowComponents := make([]discordComponent, 0, len(row.Buttons))
		for buttonIndex, button := range row.Buttons {
			label := strings.TrimSpace(button.Label)
			if label == "" {
				continue
			}
			component := discordComponent{
				Type:  componentTypeButton,
				Label: label,
			}
			if url := strings.TrimSpace(button.URL); url != "" {
				component.Style = componentStyleLink
				component.URL = url
			} else {
				component.Style = discordButtonStyle(button.Style)
				customID := strings.TrimSpace(button.CustomID)
				if customID == "" {
					customID = fmt.Sprintf("section-action-%d-%d", rowIndex, buttonIndex)
				}
				component.CustomID = customID
			}
			rowComponents = append(rowComponents, component)
		}
		if len(rowComponents) == 0 {
			continue
		}
		components = append(components, discordComponent{
			Type:       componentTypeActionRow,
			Components: rowComponents,
		})
	}
	return components
}

func fromStructuredActions(actions []core.StructuredAction, buttonsPerRow int) []core.DiscordActionRow {
	if len(actions) == 0 {
		return nil
	}
	if buttonsPerRow <= 0 {
		buttonsPerRow = 1
	}
	rows := make([]core.DiscordActionRow, 0, (len(actions)/buttonsPerRow)+1)
	for start := 0; start < len(actions); start += buttonsPerRow {
		end := start + buttonsPerRow
		if end > len(actions) {
			end = len(actions)
		}
		row := core.DiscordActionRow{Buttons: make([]core.DiscordButton, 0, end-start)}
		for _, action := range actions[start:end] {
			button := core.DiscordButton{
				Label:    strings.TrimSpace(action.Label),
				CustomID: strings.TrimSpace(action.ID),
				URL:      strings.TrimSpace(action.URL),
				Style:    string(action.Style),
			}
			if button.Label == "" {
				continue
			}
			row.Buttons = append(row.Buttons, button)
		}
		if len(row.Buttons) > 0 {
			rows = append(rows, row)
		}
	}
	return rows
}

func discordButtonStyle(style string) int {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case string(core.ActionStylePrimary):
		return componentStylePrimary
	case string(core.ActionStyleDanger):
		return componentStyleDanger
	default:
		return componentStyleSecondary
	}
}
