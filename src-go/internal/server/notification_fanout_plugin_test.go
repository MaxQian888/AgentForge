package server

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type stubIMNotifier struct {
	calls   []map[string]any
	failPat string
}

func (s *stubIMNotifier) Notify(ctx context.Context, platform, channelID, text string) error {
	s.calls = append(s.calls, map[string]any{
		"platform":   platform,
		"channel_id": channelID,
		"text":       text,
	})
	if s.failPat != "" && platform == s.failPat {
		return errors.New("im notify failed")
	}
	return nil
}

type stubEmailSender struct {
	calls   []map[string]any
	failTo  string
}

func (s *stubEmailSender) SendEmail(ctx context.Context, to, subject, body string) error {
	s.calls = append(s.calls, map[string]any{"to": to, "subject": subject, "body": body})
	if s.failTo != "" && to == s.failTo {
		return errors.New("email send failed")
	}
	return nil
}

func TestNotificationFanout_RoutesToIMAndEmail(t *testing.T) {
	imNotifier := &stubIMNotifier{}
	emailSender := &stubEmailSender{}

	plugin := NewNotificationFanoutPlugin(imNotifier, emailSender)

	rules := []NotificationRule{
		{Channel: "im", Platform: "slack", ChannelID: "C123"},
		{Channel: "email", To: "team@example.com"},
	}

	err := plugin.Fanout(context.Background(), FanoutRequest{
		Subject: "Build failed",
		Body:    "main branch build failed",
		Rules:   rules,
	})
	if err != nil {
		t.Fatalf("Fanout: %v", err)
	}
	if len(imNotifier.calls) != 1 {
		t.Errorf("expected 1 IM notify call, got %d", len(imNotifier.calls))
	}
	if imNotifier.calls[0]["platform"] != "slack" {
		t.Errorf("IM platform = %v", imNotifier.calls[0]["platform"])
	}
	if imNotifier.calls[0]["channel_id"] != "C123" {
		t.Errorf("IM channel_id = %v", imNotifier.calls[0]["channel_id"])
	}
	if len(emailSender.calls) != 1 {
		t.Errorf("expected 1 email send call, got %d", len(emailSender.calls))
	}
	if emailSender.calls[0]["to"] != "team@example.com" {
		t.Errorf("email to = %v", emailSender.calls[0]["to"])
	}
	if emailSender.calls[0]["subject"] != "Build failed" {
		t.Errorf("email subject = %v", emailSender.calls[0]["subject"])
	}
}

func TestNotificationFanout_CollectsErrorsAndContinues(t *testing.T) {
	imNotifier := &stubIMNotifier{failPat: "feishu"}
	emailSender := &stubEmailSender{failTo: "broken@example.com"}

	plugin := NewNotificationFanoutPlugin(imNotifier, emailSender)

	err := plugin.Fanout(context.Background(), FanoutRequest{
		Subject: "x",
		Body:    "y",
		Rules: []NotificationRule{
			{Channel: "im", Platform: "feishu", ChannelID: "F1"},
			{Channel: "im", Platform: "slack", ChannelID: "C1"},
			{Channel: "email", To: "broken@example.com"},
			{Channel: "email", To: "ok@example.com"},
		},
	})
	if err == nil {
		t.Fatal("expected aggregate error")
	}
	if !strings.Contains(err.Error(), "feishu") || !strings.Contains(err.Error(), "broken@example.com") {
		t.Errorf("error should mention both failures, got: %v", err)
	}
	// Even with failures, the surviving rules must still fire.
	if len(imNotifier.calls) != 2 {
		t.Errorf("expected 2 IM notify calls (including the failing one), got %d", len(imNotifier.calls))
	}
	if len(emailSender.calls) != 2 {
		t.Errorf("expected 2 email send calls (including the failing one), got %d", len(emailSender.calls))
	}
}

func TestNotificationFanout_RejectsUnknownChannel(t *testing.T) {
	plugin := NewNotificationFanoutPlugin(&stubIMNotifier{}, &stubEmailSender{})
	err := plugin.Fanout(context.Background(), FanoutRequest{
		Rules: []NotificationRule{{Channel: "carrier_pigeon"}},
	})
	if err == nil || !strings.Contains(err.Error(), "carrier_pigeon") {
		t.Errorf("expected error mentioning carrier_pigeon, got %v", err)
	}
}
