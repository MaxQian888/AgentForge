package email

import (
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

func TestStubName(t *testing.T) {
	s := NewStub("0")
	if s.Name() != "email-stub" {
		t.Errorf("Name() = %q, want %q", s.Name(), "email-stub")
	}
}

func TestStubMetadata(t *testing.T) {
	s := NewStub("0")
	meta := s.Metadata()

	if meta.Source != "email" {
		t.Errorf("Source = %q, want %q", meta.Source, "email")
	}
	if meta.Capabilities.CommandSurface != core.CommandSurfaceNone {
		t.Errorf("CommandSurface = %q, want %q", meta.Capabilities.CommandSurface, core.CommandSurfaceNone)
	}
	if meta.Capabilities.ActionCallbackMode != core.ActionCallbackNone {
		t.Errorf("ActionCallbackMode = %q, want %q", meta.Capabilities.ActionCallbackMode, core.ActionCallbackNone)
	}
	if meta.Rendering.DefaultTextFormat != core.TextFormatHTML {
		t.Errorf("DefaultTextFormat = %q, want %q", meta.Rendering.DefaultTextFormat, core.TextFormatHTML)
	}
}

func TestStubSend(t *testing.T) {
	s := NewStub("0")
	if err := s.Send(nil, "user@example.com", "Hello"); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(s.replies))
	}
	r := s.replies[0]
	if r.To != "user@example.com" {
		t.Errorf("To = %q, want %q", r.To, "user@example.com")
	}
	if r.Content != "Hello" {
		t.Errorf("Content = %q, want %q", r.Content, "Hello")
	}
}

func TestStubReply(t *testing.T) {
	s := NewStub("0")
	rc := &emailReplyContext{
		To:        "reply@example.com",
		Subject:   "Re: Test",
		MessageID: "<abc@example.com>",
	}
	if err := s.Reply(nil, rc, "Reply content"); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(s.replies))
	}
	r := s.replies[0]
	if r.To != "reply@example.com" {
		t.Errorf("To = %q", r.To)
	}
	if r.InReplyTo != "<abc@example.com>" {
		t.Errorf("InReplyTo = %q", r.InReplyTo)
	}
}

func TestStubSendStructured(t *testing.T) {
	s := NewStub("0")
	msg := &core.StructuredMessage{
		Title: "Deploy Alert",
		Body:  "v2.1 deployed",
	}
	if err := s.SendStructured(nil, "ops@example.com", msg); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(s.replies))
	}
	r := s.replies[0]
	if r.Subject != "Deploy Alert" {
		t.Errorf("Subject = %q, want %q", r.Subject, "Deploy Alert")
	}
	if !strings.Contains(r.HTMLBody, "<h2>Deploy Alert</h2>") {
		t.Error("HTML should contain title")
	}
}

func TestStubSendFormattedText(t *testing.T) {
	s := NewStub("0")
	msg := &core.FormattedText{Content: "<b>Test</b>", Format: core.TextFormatHTML}
	if err := s.SendFormattedText(nil, "test@example.com", msg); err != nil {
		t.Fatal(err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(s.replies))
	}
	if s.replies[0].Format != string(core.TextFormatHTML) {
		t.Errorf("Format = %q", s.replies[0].Format)
	}
}

func TestStubReplyContextFromTarget(t *testing.T) {
	s := NewStub("0")

	t.Run("nil target", func(t *testing.T) {
		if s.ReplyContextFromTarget(nil) != nil {
			t.Error("nil target should return nil")
		}
	})

	t.Run("with target", func(t *testing.T) {
		target := &core.ReplyTarget{
			Platform:  "email",
			ChatID:    "user@example.com",
			MessageID: "<msg-123@example.com>",
			Metadata:  map[string]string{"subject": "Re: Test"},
		}
		raw := s.ReplyContextFromTarget(target)
		rc, ok := raw.(*emailReplyContext)
		if !ok || rc == nil {
			t.Fatal("expected *emailReplyContext")
		}
		if rc.To != "user@example.com" {
			t.Errorf("To = %q", rc.To)
		}
		if rc.MessageID != "<msg-123@example.com>" {
			t.Errorf("MessageID = %q", rc.MessageID)
		}
		if rc.Subject != "Re: Test" {
			t.Errorf("Subject = %q", rc.Subject)
		}
	})
}

func TestToEmailReplyContext(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		rc := toEmailReplyContext(nil)
		if rc.To != "" {
			t.Error("nil should produce empty context")
		}
	})

	t.Run("direct struct", func(t *testing.T) {
		rc := toEmailReplyContext(emailReplyContext{To: "a@b.com"})
		if rc.To != "a@b.com" {
			t.Errorf("To = %q", rc.To)
		}
	})

	t.Run("pointer", func(t *testing.T) {
		rc := toEmailReplyContext(&emailReplyContext{To: "c@d.com"})
		if rc.To != "c@d.com" {
			t.Errorf("To = %q", rc.To)
		}
	})

	t.Run("message", func(t *testing.T) {
		msg := &core.Message{
			ChatID: "from@example.com",
			ReplyTarget: &core.ReplyTarget{
				MessageID: "<id@ex>",
				Metadata:  map[string]string{"subject": "Re: Hi"},
			},
		}
		rc := toEmailReplyContext(msg)
		if rc.To != "from@example.com" {
			t.Errorf("To = %q", rc.To)
		}
		if rc.MessageID != "<id@ex>" {
			t.Errorf("MessageID = %q", rc.MessageID)
		}
	})
}
