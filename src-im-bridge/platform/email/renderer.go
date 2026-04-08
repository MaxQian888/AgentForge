package email

import (
	"fmt"
	"html"
	"strings"

	"github.com/agentforge/im-bridge/core"
)

// renderEmailHTML converts a structured message to HTML and plain text email bodies.
func renderEmailHTML(message *core.StructuredMessage) (htmlBody string, plainText string) {
	if message == nil {
		return "", ""
	}

	plain := structuredToPlainText(message)
	if plain == "" {
		return "", ""
	}

	var body strings.Builder
	if title := strings.TrimSpace(message.Title); title != "" {
		body.WriteString("<h2>" + html.EscapeString(title) + "</h2>\n")
	}
	if text := strings.TrimSpace(message.Body); text != "" {
		body.WriteString("<p>" + html.EscapeString(text) + "</p>\n")
	}

	if len(message.Fields) > 0 {
		body.WriteString("<table border=\"0\" cellpadding=\"4\" cellspacing=\"0\" style=\"border-collapse:collapse;\">\n")
		for _, field := range message.Fields {
			label := strings.TrimSpace(field.Label)
			value := strings.TrimSpace(field.Value)
			if label == "" && value == "" {
				continue
			}
			body.WriteString("<tr>")
			body.WriteString("<td style=\"font-weight:bold;padding-right:8px;\">" + html.EscapeString(label) + "</td>")
			body.WriteString("<td>" + html.EscapeString(value) + "</td>")
			body.WriteString("</tr>\n")
		}
		body.WriteString("</table>\n")
	}

	if len(message.Actions) > 0 {
		body.WriteString("<p>")
		var links []string
		for _, action := range message.Actions {
			label := strings.TrimSpace(action.Label)
			url := strings.TrimSpace(action.URL)
			if label == "" {
				continue
			}
			if url != "" {
				links = append(links, fmt.Sprintf("<a href=\"%s\">%s</a>", html.EscapeString(url), html.EscapeString(label)))
			} else {
				links = append(links, html.EscapeString(label))
			}
		}
		body.WriteString(strings.Join(links, " &nbsp;|&nbsp; "))
		body.WriteString("</p>\n")
	}

	return emailHTMLTemplate(strings.TrimSpace(message.Title), body.String()), plain
}

// renderFormattedTextEmail converts a FormattedText to HTML and plain text parts.
func renderFormattedTextEmail(message *core.FormattedText) (htmlBody string, plainText string) {
	if message == nil || strings.TrimSpace(message.Content) == "" {
		return "", ""
	}

	content := strings.TrimSpace(message.Content)

	if message.Format == core.TextFormatHTML {
		return emailHTMLTemplate("", content), stripHTMLTags(content)
	}

	// Plain text or other formats: wrap in pre tag.
	escaped := html.EscapeString(content)
	return emailHTMLTemplate("", "<pre style=\"white-space:pre-wrap;\">"+escaped+"</pre>"), content
}

// emailHTMLTemplate wraps body HTML in a responsive email template.
func emailHTMLTemplate(title, bodyHTML string) string {
	var buf strings.Builder
	buf.WriteString("<!DOCTYPE html>\n")
	buf.WriteString("<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n")
	buf.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1.0\">\n")
	if title != "" {
		buf.WriteString("<title>" + html.EscapeString(title) + "</title>\n")
	}
	buf.WriteString("</head>\n<body style=\"margin:0;padding:0;font-family:sans-serif;font-size:14px;line-height:1.5;color:#333;\">\n")
	buf.WriteString("<table width=\"100%\" cellpadding=\"0\" cellspacing=\"0\" border=\"0\">\n")
	buf.WriteString("<tr><td align=\"center\">\n")
	buf.WriteString("<table width=\"600\" cellpadding=\"20\" cellspacing=\"0\" border=\"0\" style=\"max-width:600px;width:100%;\">\n")
	buf.WriteString("<tr><td>\n")
	buf.WriteString(bodyHTML)
	buf.WriteString("<hr style=\"border:none;border-top:1px solid #eee;margin:20px 0;\">\n")
	buf.WriteString("<p style=\"font-size:12px;color:#999;\">Sent by AgentForge</p>\n")
	buf.WriteString("</td></tr>\n</table>\n")
	buf.WriteString("</td></tr>\n</table>\n")
	buf.WriteString("</body>\n</html>")
	return buf.String()
}

// structuredToPlainText converts a structured message to plain text.
func structuredToPlainText(message *core.StructuredMessage) string {
	if message == nil {
		return ""
	}
	return strings.TrimSpace(message.FallbackText())
}

// stripHTMLTags removes HTML tags from a string (simple implementation).
func stripHTMLTags(s string) string {
	var buf strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			buf.WriteRune(r)
		}
	}
	return strings.TrimSpace(buf.String())
}

// emailSubjectFromStructured derives an email subject from a structured message.
func emailSubjectFromStructured(message *core.StructuredMessage) string {
	if message == nil {
		return "AgentForge Notification"
	}
	if title := strings.TrimSpace(message.Title); title != "" {
		return title
	}
	return "AgentForge Notification"
}
