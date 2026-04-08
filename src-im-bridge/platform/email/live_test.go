package email

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

type mockSMTPSender struct {
	mu       sync.Mutex
	sent     []mockSentEmail
	sendErr  error
}

type mockSentEmail struct {
	To       string
	Subject  string
	HTMLBody string
	TextBody string
	Headers  map[string]string
}

func (m *mockSMTPSender) SendMail(_ context.Context, to, subject, htmlBody, textBody string, headers map[string]string) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.mu.Lock()
	m.sent = append(m.sent, mockSentEmail{
		To:       to,
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
		Headers:  headers,
	})
	m.mu.Unlock()
	return nil
}

func newTestLive(t *testing.T, mock *mockSMTPSender) *Live {
	t.Helper()
	live, err := NewLive("smtp.example.com", "587", "user", "pass", "noreply@example.com",
		WithSMTPSender(mock),
	)
	if err != nil {
		t.Fatal(err)
	}
	return live
}

func TestNewLive_Validation(t *testing.T) {
	tests := []struct {
		name string
		host string
		from string
		err  string
	}{
		{"empty host", "", "a@b.com", "SMTP host"},
		{"empty from", "smtp.example.com", "", "from address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewLive(tt.host, "587", "", "", tt.from)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.err) {
				t.Errorf("error %q should contain %q", err.Error(), tt.err)
			}
		})
	}
}

func TestLiveName(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)
	if live.Name() != "email-live" {
		t.Errorf("Name() = %q", live.Name())
	}
}

func TestLiveMetadata(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)
	meta := live.Metadata()

	if meta.Source != "email" {
		t.Errorf("Source = %q", meta.Source)
	}
	if meta.Rendering.DefaultTextFormat != core.TextFormatHTML {
		t.Errorf("DefaultTextFormat = %q", meta.Rendering.DefaultTextFormat)
	}
}

func TestLiveSend(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	if err := live.Send(context.Background(), "user@example.com", "Hello from AgentForge"); err != nil {
		t.Fatal(err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(mock.sent))
	}
	email := mock.sent[0]
	if email.To != "user@example.com" {
		t.Errorf("To = %q", email.To)
	}
	if email.Subject != "AgentForge Notification" {
		t.Errorf("Subject = %q", email.Subject)
	}
	if email.TextBody != "Hello from AgentForge" {
		t.Errorf("TextBody = %q", email.TextBody)
	}
	if !strings.Contains(email.HTMLBody, "Hello from AgentForge") {
		t.Error("HTML body should contain text")
	}
}

func TestLiveReply_Threading(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	rc := &emailReplyContext{
		To:         "sender@example.com",
		Subject:    "Re: Task Update",
		MessageID:  "<original@example.com>",
		References: []string{"<original@example.com>"},
	}

	if err := live.Reply(context.Background(), rc, "Got it!"); err != nil {
		t.Fatal(err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(mock.sent))
	}
	email := mock.sent[0]
	if email.Headers["In-Reply-To"] != "<original@example.com>" {
		t.Errorf("In-Reply-To = %q", email.Headers["In-Reply-To"])
	}
	if !strings.Contains(email.Headers["References"], "<original@example.com>") {
		t.Errorf("References = %q", email.Headers["References"])
	}
	if email.Subject != "Re: Task Update" {
		t.Errorf("Subject = %q", email.Subject)
	}
}

func TestLiveSendStructured(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	msg := &core.StructuredMessage{
		Title: "Deploy Complete",
		Body:  "Version 2.0 deployed successfully",
		Fields: []core.StructuredField{
			{Label: "Environment", Value: "production"},
		},
	}

	if err := live.SendStructured(context.Background(), "ops@example.com", msg); err != nil {
		t.Fatal(err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(mock.sent))
	}
	email := mock.sent[0]
	if email.Subject != "Deploy Complete" {
		t.Errorf("Subject = %q", email.Subject)
	}
	if !strings.Contains(email.HTMLBody, "<h2>Deploy Complete</h2>") {
		t.Error("HTML should contain title")
	}
	if !strings.Contains(email.HTMLBody, "production") {
		t.Error("HTML should contain field value")
	}
}

func TestLiveSendFormattedText(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	msg := &core.FormattedText{Content: "<b>Alert</b>", Format: core.TextFormatHTML}
	if err := live.SendFormattedText(context.Background(), "admin@example.com", msg); err != nil {
		t.Fatal(err)
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.sent) != 1 {
		t.Fatalf("expected 1 sent, got %d", len(mock.sent))
	}
	if !strings.Contains(mock.sent[0].HTMLBody, "<b>Alert</b>") {
		t.Error("HTML body should contain formatted text")
	}
}

func TestLiveSend_EmptyRecipient(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	if err := live.Send(context.Background(), "", "test"); err == nil {
		t.Error("expected error for empty recipient")
	}
}

func TestLiveReply_EmptyRecipient(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	rc := &emailReplyContext{To: ""}
	if err := live.Reply(context.Background(), rc, "test"); err == nil {
		t.Error("expected error for empty recipient")
	}
}

func TestLiveStartStop(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	handler := func(p core.Platform, msg *core.Message) {}
	if err := live.Start(handler); err != nil {
		t.Fatal(err)
	}
	// Starting again should be idempotent.
	if err := live.Start(handler); err != nil {
		t.Fatal(err)
	}
	if err := live.Stop(); err != nil {
		t.Fatal(err)
	}
}

func TestLiveStart_NilHandler(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	if err := live.Start(nil); err == nil {
		t.Error("expected error for nil handler")
	}
}

func TestLiveReplyContextFromTarget(t *testing.T) {
	mock := &mockSMTPSender{}
	live := newTestLive(t, mock)

	t.Run("nil", func(t *testing.T) {
		if live.ReplyContextFromTarget(nil) != nil {
			t.Error("nil should return nil")
		}
	})

	t.Run("with target", func(t *testing.T) {
		target := &core.ReplyTarget{
			ChatID:    "user@example.com",
			MessageID: "<msg@ex>",
			Metadata:  map[string]string{"subject": "Re: Hello"},
		}
		raw := live.ReplyContextFromTarget(target)
		rc, ok := raw.(*emailReplyContext)
		if !ok || rc == nil {
			t.Fatal("expected *emailReplyContext")
		}
		if rc.To != "user@example.com" {
			t.Errorf("To = %q", rc.To)
		}
		if rc.Subject != "Re: Hello" {
			t.Errorf("Subject = %q", rc.Subject)
		}
	})
}

func TestNormalizeIncomingEmail(t *testing.T) {
	incoming := incomingEmail{
		From:      "sender@example.com",
		To:        "bridge@agentforge.dev",
		Subject:   "Help with task",
		Body:      "Please assign me",
		MessageID: "<abc@example.com>",
		InReplyTo: "<prev@example.com>",
	}

	msg := normalizeIncomingEmail(incoming)
	if msg.Platform != "email" {
		t.Errorf("Platform = %q", msg.Platform)
	}
	if msg.UserID != "sender@example.com" {
		t.Errorf("UserID = %q", msg.UserID)
	}
	if msg.ChatID != "sender@example.com" {
		t.Errorf("ChatID = %q", msg.ChatID)
	}
	if msg.Content != "Please assign me" {
		t.Errorf("Content = %q", msg.Content)
	}
	if msg.ReplyTarget == nil {
		t.Fatal("ReplyTarget should not be nil")
	}
	if msg.ReplyTarget.MessageID != "<abc@example.com>" {
		t.Errorf("ReplyTarget.MessageID = %q", msg.ReplyTarget.MessageID)
	}
}
