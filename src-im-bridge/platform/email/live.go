package email

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/notify"
	"github.com/google/uuid"
)

var liveMetadata = core.PlatformMetadata{
	Source: "email",
	Capabilities: core.PlatformCapabilities{
		CommandSurface:     core.CommandSurfaceNone,
		StructuredSurface:  core.StructuredSurfaceNone,
		AsyncUpdateModes:   []core.AsyncUpdateMode{core.AsyncUpdateReply},
		ActionCallbackMode: core.ActionCallbackNone,
		MessageScopes:      []core.MessageScope{core.MessageScopeChat},
	},
	Rendering: core.RenderingProfile{
		DefaultTextFormat: core.TextFormatHTML,
		SupportedFormats:  []core.TextFormatMode{core.TextFormatHTML, core.TextFormatPlainText},
		MaxTextLength:     0,
		SupportsSegments:  false,
		StructuredSurface: core.StructuredSurfaceNone,
	},
}

// smtpSender abstracts SMTP sending for testability.
type smtpSender interface {
	SendMail(ctx context.Context, to, subject, htmlBody, textBody string, headers map[string]string) error
}

// imapPoller abstracts IMAP polling for testability.
type imapPoller interface {
	Poll(ctx context.Context) ([]incomingEmail, error)
	Close() error
}

type incomingEmail struct {
	From      string
	To        string
	Subject   string
	Body      string
	MessageID string
	InReplyTo string
	Date      time.Time
}

// LiveOption configures the Live email platform.
type LiveOption func(*Live) error

// Live is the production email platform using SMTP for outbound.
type Live struct {
	smtpHost    string
	smtpPort    string
	smtpUser    string
	smtpPass    string
	fromAddress string
	useTLS      bool

	sender smtpSender
	poller imapPoller

	imapPollInterval time.Duration
	actionHandler    notify.ActionHandler

	handler     core.MessageHandler
	startCtx    context.Context
	startCancel context.CancelFunc
	started     bool
	mu          sync.Mutex
}

// NewLive creates a production email platform with SMTP transport.
func NewLive(smtpHost, smtpPort, smtpUser, smtpPass, fromAddress string, opts ...LiveOption) (*Live, error) {
	if strings.TrimSpace(smtpHost) == "" {
		return nil, errors.New("email live transport requires SMTP host")
	}
	if strings.TrimSpace(smtpPort) == "" {
		smtpPort = "587"
	}
	if strings.TrimSpace(fromAddress) == "" {
		return nil, errors.New("email live transport requires from address")
	}

	live := &Live{
		smtpHost:         strings.TrimSpace(smtpHost),
		smtpPort:         strings.TrimSpace(smtpPort),
		smtpUser:         strings.TrimSpace(smtpUser),
		smtpPass:         smtpPass,
		fromAddress:      strings.TrimSpace(fromAddress),
		useTLS:           true,
		imapPollInterval: 60 * time.Second,
	}

	for _, opt := range opts {
		if err := opt(live); err != nil {
			return nil, err
		}
	}

	if live.sender == nil {
		live.sender = &defaultSMTPSender{
			host:     live.smtpHost,
			port:     live.smtpPort,
			user:     live.smtpUser,
			pass:     live.smtpPass,
			from:     live.fromAddress,
			useTLS:   live.useTLS,
		}
	}

	return live, nil
}

// WithTLS configures TLS for SMTP connections.
func WithTLS(enabled bool) LiveOption {
	return func(l *Live) error {
		l.useTLS = enabled
		return nil
	}
}

// WithSMTPSender injects a custom SMTP sender (for testing).
func WithSMTPSender(s smtpSender) LiveOption {
	return func(l *Live) error {
		if s == nil {
			return errors.New("smtp sender cannot be nil")
		}
		l.sender = s
		return nil
	}
}

// WithIMAPPoller injects an IMAP poller for inbound email.
func WithIMAPPoller(p imapPoller) LiveOption {
	return func(l *Live) error {
		l.poller = p
		return nil
	}
}

// WithIMAPPollInterval sets the IMAP polling interval.
func WithIMAPPollInterval(d time.Duration) LiveOption {
	return func(l *Live) error {
		if d < time.Second {
			return errors.New("imap poll interval must be at least 1 second")
		}
		l.imapPollInterval = d
		return nil
	}
}

func (l *Live) Name() string { return "email-live" }

func (l *Live) Metadata() core.PlatformMetadata {
	return core.NormalizeMetadata(liveMetadata, liveMetadata.Source)
}

func (l *Live) SetActionHandler(handler notify.ActionHandler) {
	l.actionHandler = handler
}

func (l *Live) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	ctx := &emailReplyContext{
		To:      firstNonEmpty(target.ChatID, target.ChannelID),
		Subject: metadataValue(target.Metadata, "subject"),
	}
	if msgID := strings.TrimSpace(target.MessageID); msgID != "" {
		ctx.MessageID = msgID
		ctx.References = []string{msgID}
	}
	return ctx
}

func (l *Live) Start(handler core.MessageHandler) error {
	if handler == nil {
		return errors.New("message handler is required")
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.started {
		return nil
	}

	l.handler = handler
	ctx, cancel := context.WithCancel(context.Background())
	l.startCtx = ctx
	l.startCancel = cancel

	// Start IMAP polling if poller is configured.
	if l.poller != nil {
		go l.pollIMAP(ctx)
	}

	l.started = true
	return nil
}

func (l *Live) Reply(ctx context.Context, rawReplyCtx any, content string) error {
	rc := toEmailReplyContext(rawReplyCtx)
	if rc.To == "" {
		return errors.New("email reply requires recipient address")
	}
	subject := rc.Subject
	if subject == "" {
		subject = "Re: AgentForge Notification"
	}

	headers := make(map[string]string)
	if rc.MessageID != "" {
		headers["In-Reply-To"] = rc.MessageID
		headers["References"] = strings.Join(rc.References, " ")
	}

	htmlBody, _ := renderFormattedTextEmail(&core.FormattedText{Content: content, Format: core.TextFormatPlainText})
	return l.sender.SendMail(ctx, rc.To, subject, htmlBody, content, headers)
}

func (l *Live) Send(ctx context.Context, chatID string, content string) error {
	to := strings.TrimSpace(chatID)
	if to == "" {
		return errors.New("email send requires recipient address")
	}
	htmlBody, _ := renderFormattedTextEmail(&core.FormattedText{Content: content, Format: core.TextFormatPlainText})
	return l.sender.SendMail(ctx, to, "AgentForge Notification", htmlBody, content, nil)
}

func (l *Live) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	to := strings.TrimSpace(chatID)
	if to == "" {
		return errors.New("email send requires recipient address")
	}
	htmlBody, plainText := renderEmailHTML(message)
	subject := emailSubjectFromStructured(message)
	return l.sender.SendMail(ctx, to, subject, htmlBody, plainText, nil)
}

func (l *Live) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	to := strings.TrimSpace(chatID)
	if to == "" {
		return errors.New("email send requires recipient address")
	}
	if message == nil {
		return errors.New("formatted text is required")
	}
	htmlBody, plainText := renderFormattedTextEmail(message)
	return l.sender.SendMail(ctx, to, "AgentForge Notification", htmlBody, plainText, nil)
}

func (l *Live) ReplyFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	rc := toEmailReplyContext(rawReplyCtx)
	if rc.To == "" {
		return errors.New("email reply requires recipient address")
	}
	if message == nil {
		return errors.New("formatted text is required")
	}
	subject := rc.Subject
	if subject == "" {
		subject = "Re: AgentForge Notification"
	}

	headers := make(map[string]string)
	if rc.MessageID != "" {
		headers["In-Reply-To"] = rc.MessageID
		headers["References"] = strings.Join(rc.References, " ")
	}

	htmlBody, plainText := renderFormattedTextEmail(message)
	return l.sender.SendMail(ctx, rc.To, subject, htmlBody, plainText, headers)
}

func (l *Live) UpdateFormattedText(ctx context.Context, rawReplyCtx any, message *core.FormattedText) error {
	// Email cannot update messages; send a new reply instead.
	return l.ReplyFormattedText(ctx, rawReplyCtx, message)
}

func (l *Live) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.started {
		return nil
	}
	if l.startCancel != nil {
		l.startCancel()
	}
	if l.poller != nil {
		_ = l.poller.Close()
	}
	l.started = false
	return nil
}

func (l *Live) pollIMAP(ctx context.Context) {
	logger := log.WithField("component", "email-live-imap")
	ticker := time.NewTicker(l.imapPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			emails, err := l.poller.Poll(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logger.WithError(err).Error("IMAP poll error")
				continue
			}
			for _, incoming := range emails {
				msg := normalizeIncomingEmail(incoming)
				if l.handler != nil {
					l.handler(l, msg)
				}
			}
		}
	}
}

func normalizeIncomingEmail(incoming incomingEmail) *core.Message {
	replyTarget := &core.ReplyTarget{
		Platform:  "email",
		ChatID:    strings.TrimSpace(incoming.From),
		ChannelID: strings.TrimSpace(incoming.From),
		MessageID: strings.TrimSpace(incoming.MessageID),
		UseReply:  true,
		Metadata:  map[string]string{"subject": strings.TrimSpace(incoming.Subject)},
	}

	return &core.Message{
		Platform:    "email",
		SessionKey:  fmt.Sprintf("email:%s:%s", incoming.To, incoming.From),
		UserID:      strings.TrimSpace(incoming.From),
		UserName:    strings.TrimSpace(incoming.From),
		ChatID:      strings.TrimSpace(incoming.From),
		Content:     strings.TrimSpace(incoming.Body),
		ReplyCtx: &emailReplyContext{
			To:         strings.TrimSpace(incoming.From),
			Subject:    "Re: " + strings.TrimSpace(incoming.Subject),
			MessageID:  strings.TrimSpace(incoming.MessageID),
			References: buildReferences(incoming.InReplyTo, incoming.MessageID),
		},
		ReplyTarget: replyTarget,
		Timestamp:   incoming.Date,
		IsGroup:     false,
	}
}

func buildReferences(inReplyTo, messageID string) []string {
	var refs []string
	if trimmed := strings.TrimSpace(inReplyTo); trimmed != "" {
		refs = append(refs, trimmed)
	}
	if trimmed := strings.TrimSpace(messageID); trimmed != "" {
		refs = append(refs, trimmed)
	}
	return refs
}

// defaultSMTPSender is the production SMTP sender.
type defaultSMTPSender struct {
	host   string
	port   string
	user   string
	pass   string
	from   string
	useTLS bool
}

func (s *defaultSMTPSender) SendMail(_ context.Context, to, subject, htmlBody, textBody string, headers map[string]string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return errors.New("email recipient is required")
	}

	messageID := fmt.Sprintf("<%s@agentforge>", uuid.New().String())
	boundary := "agentforge-boundary-" + uuid.New().String()[:8]

	var msg strings.Builder
	msg.WriteString("From: " + s.from + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("Date: " + time.Now().Format("Mon, 02 Jan 2006 15:04:05 -0700") + "\r\n")
	msg.WriteString("Message-ID: " + messageID + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")

	for key, value := range headers {
		if strings.TrimSpace(value) != "" {
			msg.WriteString(key + ": " + strings.TrimSpace(value) + "\r\n")
		}
	}

	msg.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	msg.WriteString("\r\n")

	// Plain text part.
	msg.WriteString("--" + boundary + "\r\n")
	msg.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if textBody != "" {
		msg.WriteString(textBody)
	} else {
		msg.WriteString(stripHTMLTags(htmlBody))
	}
	msg.WriteString("\r\n")

	// HTML part.
	if htmlBody != "" {
		msg.WriteString("--" + boundary + "\r\n")
		msg.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
		msg.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		msg.WriteString(htmlBody)
		msg.WriteString("\r\n")
	}

	msg.WriteString("--" + boundary + "--\r\n")

	addr := net.JoinHostPort(s.host, s.port)

	var auth smtp.Auth
	if s.user != "" {
		auth = smtp.PlainAuth("", s.user, s.pass, s.host)
	}

	if s.useTLS {
		return s.sendWithTLS(addr, auth, to, msg.String())
	}
	return smtp.SendMail(addr, auth, s.from, []string{to}, []byte(msg.String()))
}

func (s *defaultSMTPSender) sendWithTLS(addr string, auth smtp.Auth, to, msg string) error {
	tlsConfig := &tls.Config{ServerName: s.host}

	conn, err := tls.Dial("tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("email tls dial: %w", err)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("email smtp client: %w", err)
	}
	defer client.Close()

	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("email smtp mail: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("email smtp rcpt: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("email smtp data: %w", err)
	}
	if _, err := writer.Write([]byte(msg)); err != nil {
		return fmt.Errorf("email smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("email smtp close: %w", err)
	}
	return client.Quit()
}

// Interface compliance assertions.
var (
	_ core.Platform            = (*Live)(nil)
	_ core.StructuredSender    = (*Live)(nil)
	_ core.FormattedTextSender = (*Live)(nil)
	_ core.ReplyTargetResolver = (*Live)(nil)
)
