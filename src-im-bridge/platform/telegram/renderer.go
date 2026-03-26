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
