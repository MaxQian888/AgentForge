package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

type textOnlyPlatform struct {
	name string
	sent []string
	chat []string
}

func (p *textOnlyPlatform) Name() string                                                  { return p.name }
func (p *textOnlyPlatform) Start(handler core.MessageHandler) error                       { return nil }
func (p *textOnlyPlatform) Reply(ctx context.Context, replyCtx any, content string) error { return nil }
func (p *textOnlyPlatform) Stop() error                                                   { return nil }
func (p *textOnlyPlatform) Send(ctx context.Context, chatID string, content string) error {
	p.chat = append(p.chat, chatID)
	p.sent = append(p.sent, content)
	return nil
}

type cardPlatform struct {
	textOnlyPlatform
	cardTitles []string
}

func (p *cardPlatform) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	p.chat = append(p.chat, chatID)
	p.cardTitles = append(p.cardTitles, card.Title)
	return nil
}

func (p *cardPlatform) ReplyCard(ctx context.Context, replyCtx any, card *core.Card) error {
	return nil
}

func TestReceiver_RejectsPlatformMismatch(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	body, err := json.Marshal(Notification{
		Platform:     "dingtalk",
		TargetChatID: "chat-1",
		Content:      "hello",
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

func TestReceiver_FallsBackToTextWhenCardSenderUnavailable(t *testing.T) {
	p := &textOnlyPlatform{name: "slack-stub"}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "slack",
		TargetChatID: "chat-1",
		Content:      "plain fallback",
		Card:         core.NewCard().SetTitle("card title"),
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.sent) != 1 || p.sent[0] != "plain fallback" {
		t.Fatalf("sent = %v, want plain fallback", p.sent)
	}
}

func TestReceiver_SendsCardWhenPlatformSupportsCards(t *testing.T) {
	p := &cardPlatform{textOnlyPlatform: textOnlyPlatform{name: "dingtalk-stub"}}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "dingtalk",
		TargetChatID: "chat-1",
		Content:      "plain fallback",
		Card:         core.NewCard().SetTitle("card title"),
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if len(p.cardTitles) != 1 || p.cardTitles[0] != "card title" {
		t.Fatalf("cardTitles = %v, want [card title]", p.cardTitles)
	}
}
