package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/agentforge/im-bridge/core"
)

type textOnlyPlatform struct {
	name    string
	sent    []string
	chat    []string
	sendErr error
}

func (p *textOnlyPlatform) Name() string                                                  { return p.name }
func (p *textOnlyPlatform) Start(handler core.MessageHandler) error                       { return nil }
func (p *textOnlyPlatform) Reply(ctx context.Context, replyCtx any, content string) error { return nil }
func (p *textOnlyPlatform) Stop() error                                                   { return nil }
func (p *textOnlyPlatform) Send(ctx context.Context, chatID string, content string) error {
	if p.sendErr != nil {
		return p.sendErr
	}
	p.chat = append(p.chat, chatID)
	p.sent = append(p.sent, content)
	return nil
}

type cardPlatform struct {
	textOnlyPlatform
	cardTitles  []string
	sendCardErr error
}

func (p *cardPlatform) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	if p.sendCardErr != nil {
		return p.sendCardErr
	}
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

func TestReceiver_RejectsInvalidJSON(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewBufferString("{"))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestReceiver_RequiresPlatform(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	body, err := json.Marshal(Notification{
		TargetChatID: "chat-1",
		Content:      "hello",
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestReceiver_UsesTargetUserIDAsFallbackChatID(t *testing.T) {
	p := &textOnlyPlatform{name: "slack-stub"}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:       "slack",
		TargetIMUserID: "user-1",
		Content:        "hello",
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
	if len(p.chat) != 1 || p.chat[0] != "user-1" {
		t.Fatalf("chat = %v, want [user-1]", p.chat)
	}
}

func TestReceiver_ReturnsErrorWhenPlainTextSendFails(t *testing.T) {
	p := &textOnlyPlatform{name: "slack-stub", sendErr: errors.New("send failed")}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "slack",
		TargetChatID: "chat-1",
		Content:      "hello",
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestReceiver_ReturnsErrorWhenCardSendFails(t *testing.T) {
	p := &cardPlatform{
		textOnlyPlatform: textOnlyPlatform{name: "dingtalk-stub"},
		sendCardErr:      errors.New("card failed"),
	}
	r := NewReceiver(p, "0")

	body, err := json.Marshal(Notification{
		Platform:     "dingtalk",
		TargetChatID: "chat-1",
		Content:      "fallback",
		Card:         core.NewCard().SetTitle("card title"),
	})
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/im/notify", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	r.handleNotify(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestReceiver_StartExposesHealthAndStopShutsDownServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, strconv.Itoa(port))
	done := make(chan error, 1)
	go func() {
		done <- r.Start()
	}()

	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = http.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/im/health")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !bytes.Contains(body, []byte(`"platform":"slack-stub"`)) {
		t.Fatalf("body = %s", string(body))
	}

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Start returned error after stop: %v", err)
	}
}

func TestReceiver_StopWithoutStartedServerIsNoop(t *testing.T) {
	r := NewReceiver(&textOnlyPlatform{name: "slack-stub"}, "0")

	if err := r.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
}
