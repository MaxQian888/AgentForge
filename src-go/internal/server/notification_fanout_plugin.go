package server

import (
	"context"
	"fmt"
	"strings"
)

// IMNotifier routes IM notifications through the existing IMService.
// The implementation must wrap IMService.Notify() rather than calling the
// IM Bridge directly — IM Bridge ownership of all transport routing is the
// architectural invariant this plugin must preserve.
type IMNotifier interface {
	Notify(ctx context.Context, platform, channelID, text string) error
}

// EmailSender invokes the email-adapter plugin's send_email operation.
// In production this is the plugin runtime adapter; in tests it's a stub.
type EmailSender interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

// NotificationRule describes one delivery target.
type NotificationRule struct {
	// Channel selects the transport: "im" or "email".
	Channel string
	// Platform names the IM platform (feishu, slack, …) — used when Channel="im".
	Platform string
	// ChannelID is the IM chat / channel id — used when Channel="im".
	ChannelID string
	// To is the recipient email — used when Channel="email".
	To string
}

// FanoutRequest is the input to Fanout.
type FanoutRequest struct {
	Subject string
	Body    string
	Rules   []NotificationRule
}

// NotificationFanoutPlugin is a first-party in-proc service that routes one
// notification to multiple channels (IM Bridge for IM, email-adapter for
// email). Errors on individual rules are collected so a partial failure
// doesn't drop the rest of the fanout.
type NotificationFanoutPlugin struct {
	im    IMNotifier
	email EmailSender
}

func NewNotificationFanoutPlugin(im IMNotifier, email EmailSender) *NotificationFanoutPlugin {
	return &NotificationFanoutPlugin{im: im, email: email}
}

// Fanout delivers the notification to every rule. Per-rule errors are
// gathered and returned as a single aggregate error after all rules have
// been attempted — so one bad route doesn't black-hole the rest.
func (p *NotificationFanoutPlugin) Fanout(ctx context.Context, req FanoutRequest) error {
	var errs []string
	for _, rule := range req.Rules {
		switch rule.Channel {
		case "im":
			if err := p.im.Notify(ctx, rule.Platform, rule.ChannelID, req.Body); err != nil {
				errs = append(errs, fmt.Sprintf("im[%s/%s]: %v", rule.Platform, rule.ChannelID, err))
			}
		case "email":
			if err := p.email.SendEmail(ctx, rule.To, req.Subject, req.Body); err != nil {
				errs = append(errs, fmt.Sprintf("email[%s]: %v", rule.To, err))
			}
		default:
			errs = append(errs, fmt.Sprintf("unknown channel type: %q", rule.Channel))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("notification-fanout: %s", strings.Join(errs, "; "))
	}
	return nil
}
