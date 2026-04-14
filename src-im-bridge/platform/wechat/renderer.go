package wechat

import (
	"encoding/json"
	"strings"
)

// newsArticle represents a single article in a WeChat news message.
type newsArticle struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	PicURL      string `json:"picurl"`
}

// renderTextReply returns JSON bytes for a WeChat customer service text message.
func renderTextReply(openID, content string) []byte {
	payload := map[string]any{
		"touser":  strings.TrimSpace(openID),
		"msgtype": "text",
		"text": map[string]string{
			"content": content,
		},
	}
	data, _ := json.Marshal(payload)
	return data
}

// renderNewsReply returns JSON bytes for a WeChat customer service news article message.
func renderNewsReply(openID string, articles []newsArticle) []byte {
	items := make([]map[string]string, 0, len(articles))
	for _, a := range articles {
		items = append(items, map[string]string{
			"title":       strings.TrimSpace(a.Title),
			"description": strings.TrimSpace(a.Description),
			"url":         strings.TrimSpace(a.URL),
			"picurl":      strings.TrimSpace(a.PicURL),
		})
	}
	payload := map[string]any{
		"touser":  strings.TrimSpace(openID),
		"msgtype": "news",
		"news": map[string]any{
			"articles": items,
		},
	}
	data, _ := json.Marshal(payload)
	return data
}
