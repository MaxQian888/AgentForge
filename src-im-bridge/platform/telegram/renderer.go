package telegram

import (
	"strings"
	"unicode/utf8"

	"github.com/agentforge/im-bridge/core"
)

const telegramMaxTextLength = 4096

func renderTelegramText(message *core.FormattedText) []telegramTextMessage {
	if message == nil {
		return nil
	}

	content := strings.TrimSpace(message.Content)
	if content == "" {
		return nil
	}

	format := message.Format
	switch format {
	case core.TextFormatMarkdownV2:
		escaped := escapeMarkdownV2(content)
		return splitTelegramText(escaped, telegramParseMode(format))
	default:
		return splitTelegramText(content, "")
	}
}

func splitTelegramText(content string, parseMode string) []telegramTextMessage {
	if content == "" {
		return nil
	}
	if utf8.RuneCountInString(content) <= telegramMaxTextLength {
		return []telegramTextMessage{{Text: content, ParseMode: parseMode}}
	}

	runes := []rune(content)
	segments := make([]telegramTextMessage, 0, (len(runes)/telegramMaxTextLength)+1)
	for start := 0; start < len(runes); start += telegramMaxTextLength {
		end := start + telegramMaxTextLength
		if end > len(runes) {
			end = len(runes)
		}
		segments = append(segments, telegramTextMessage{
			Text:      string(runes[start:end]),
			ParseMode: parseMode,
		})
	}
	return segments
}

func telegramParseMode(format core.TextFormatMode) string {
	switch format {
	case core.TextFormatMarkdownV2:
		return "MarkdownV2"
	default:
		return ""
	}
}

func renderStructuredSections(sections []core.StructuredSection) (telegramTextMessage, *inlineKeyboardMarkup) {
	lines := make([]string, 0, len(sections))
	var keyboard [][]inlineKeyboardButton

	for _, section := range sections {
		switch section.Type {
		case core.StructuredSectionTypeActions:
			if section.ActionsSection == nil || len(section.ActionsSection.Actions) == 0 {
				continue
			}
			perRow := section.ActionsSection.ButtonsPerRow
			if perRow <= 0 {
				perRow = 1
			}
			for start := 0; start < len(section.ActionsSection.Actions); start += perRow {
				end := start + perRow
				if end > len(section.ActionsSection.Actions) {
					end = len(section.ActionsSection.Actions)
				}
				row := make([]inlineKeyboardButton, 0, end-start)
				for _, action := range section.ActionsSection.Actions[start:end] {
					label := strings.TrimSpace(action.Label)
					if label == "" {
						continue
					}
					button := inlineKeyboardButton{Text: label}
					if url := strings.TrimSpace(action.URL); url != "" {
						button.URL = url
					} else if callback := strings.TrimSpace(action.ID); callback != "" {
						button.CallbackData = callback
					} else {
						continue
					}
					row = append(row, button)
				}
				if len(row) > 0 {
					keyboard = append(keyboard, row)
				}
			}
		default:
			if fallback := strings.TrimSpace(section.FallbackText()); fallback != "" {
				lines = append(lines, fallback)
			}
		}
	}

	var markup *inlineKeyboardMarkup
	if len(keyboard) > 0 {
		markup = &inlineKeyboardMarkup{InlineKeyboard: keyboard}
	}
	return telegramTextMessage{Text: strings.TrimSpace(strings.Join(lines, "\n"))}, markup
}

func escapeMarkdownV2(content string) string {
	replacer := strings.NewReplacer(
		`\\`, `\\\\`,
		"_", `\_`,
		"*", `\*`,
		"[", `\[`,
		"]", `\]`,
		"(", `\(`,
		")", `\)`,
		"~", `\~`,
		"`", "\\`",
		">", `\>`,
		"#", `\#`,
		"+", `\+`,
		"-", `\-`,
		"=", `\=`,
		"|", `\|`,
		"{", `\{`,
		"}", `\}`,
		".", `\.`,
		"!", `\!`,
	)
	return replacer.Replace(content)
}
