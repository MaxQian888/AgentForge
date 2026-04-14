package wechat

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRenderTextReply(t *testing.T) {
	data := renderTextReply("oUser123", "hello world")
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload["touser"] != "oUser123" {
		t.Fatalf("touser = %v", payload["touser"])
	}
	if payload["msgtype"] != "text" {
		t.Fatalf("msgtype = %v", payload["msgtype"])
	}
	textObj, ok := payload["text"].(map[string]any)
	if !ok {
		t.Fatalf("text type = %T", payload["text"])
	}
	if textObj["content"] != "hello world" {
		t.Fatalf("content = %v", textObj["content"])
	}
}

func TestRenderNewsReply(t *testing.T) {
	articles := []newsArticle{
		{
			Title:       "Review Ready",
			Description: "PR #42 is ready for review",
			URL:         "https://example.test/reviews/42",
			PicURL:      "https://example.test/images/review.png",
		},
		{
			Title:       "Build Complete",
			Description: "Build #100 succeeded",
			URL:         "https://example.test/builds/100",
		},
	}

	data := renderNewsReply("oUser456", articles)
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON")
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload["touser"] != "oUser456" {
		t.Fatalf("touser = %v", payload["touser"])
	}
	if payload["msgtype"] != "news" {
		t.Fatalf("msgtype = %v", payload["msgtype"])
	}

	newsObj, ok := payload["news"].(map[string]any)
	if !ok {
		t.Fatalf("news type = %T", payload["news"])
	}
	articlesArr, ok := newsObj["articles"].([]any)
	if !ok {
		t.Fatalf("articles type = %T", newsObj["articles"])
	}
	if len(articlesArr) != 2 {
		t.Fatalf("articles count = %d", len(articlesArr))
	}

	first, ok := articlesArr[0].(map[string]any)
	if !ok {
		t.Fatalf("first article type = %T", articlesArr[0])
	}
	if first["title"] != "Review Ready" {
		t.Fatalf("title = %v", first["title"])
	}
	if !strings.Contains(first["url"].(string), "reviews/42") {
		t.Fatalf("url = %v", first["url"])
	}
}
