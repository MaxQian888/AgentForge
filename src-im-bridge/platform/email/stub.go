package email

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/agentforge/im-bridge/core"
)

// Stub is a test implementation of the email platform.
type Stub struct {
	port    string
	handler core.MessageHandler
	server  *http.Server

	mu      sync.Mutex
	replies []stubReply
}

type stubReply struct {
	To            string    `json:"to,omitempty"`
	Subject       string    `json:"subject,omitempty"`
	Content       string    `json:"content"`
	HTMLBody      string    `json:"htmlBody,omitempty"`
	Format        string    `json:"format,omitempty"`
	InReplyTo     string    `json:"inReplyTo,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

type stubMessageRequest struct {
	Content  string `json:"content"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	From     string `json:"from"`
	Subject  string `json:"subject"`
}

// emailReplyContext carries threading context for email replies.
type emailReplyContext struct {
	To         string
	Subject    string
	MessageID  string
	References []string
}

// NewStub creates a new test email platform.
func NewStub(port string) *Stub {
	return &Stub{port: port}
}

func (s *Stub) Name() string { return "email-stub" }

func (s *Stub) Metadata() core.PlatformMetadata {
	return core.NormalizeMetadata(core.PlatformMetadata{
		Source: "email",
		Capabilities: core.PlatformCapabilities{
			CommandSurface:     core.CommandSurfaceNone,
			StructuredSurface:  core.StructuredSurfaceNone,
			AsyncUpdateModes:   []core.AsyncUpdateMode{core.AsyncUpdateReply},
			ActionCallbackMode: core.ActionCallbackNone,
			MessageScopes:      []core.MessageScope{core.MessageScopeChat},
		},
	}, "email")
}

func (s *Stub) ReplyContextFromTarget(target *core.ReplyTarget) any {
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

func (s *Stub) Start(handler core.MessageHandler) error {
	s.handler = handler

	mux := http.NewServeMux()
	mux.HandleFunc("POST /test/message", s.handleTestMessage)
	mux.HandleFunc("GET /test/replies", s.handleGetReplies)
	mux.HandleFunc("DELETE /test/replies", s.handleClearReplies)

	s.server = &http.Server{
		Addr:    ":" + s.port,
		Handler: mux,
	}

	log.WithFields(log.Fields{"component": "email-stub", "port": s.port}).Info("Test server starting")
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "email-stub").WithError(err).Error("Server error")
		}
	}()
	return nil
}

func (s *Stub) Reply(ctx context.Context, replyCtx any, content string) error {
	rc := toEmailReplyContext(replyCtx)
	return s.recordReply(rc.To, rc.Subject, content, "", "", rc.MessageID)
}

func (s *Stub) Send(ctx context.Context, chatID string, content string) error {
	return s.recordReply(chatID, "AgentForge Notification", content, "", "", "")
}

func (s *Stub) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	htmlBody, plainText := renderEmailHTML(message)
	subject := emailSubjectFromStructured(message)
	return s.recordReply(chatID, subject, plainText, htmlBody, "", "")
}

func (s *Stub) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	if message == nil {
		return fmt.Errorf("formatted text is required")
	}
	htmlBody, plainText := renderFormattedTextEmail(message)
	return s.recordReply(chatID, "AgentForge Notification", plainText, htmlBody, string(message.Format), "")
}

func (s *Stub) ReplyFormattedText(ctx context.Context, replyCtx any, message *core.FormattedText) error {
	rc := toEmailReplyContext(replyCtx)
	if message == nil {
		return fmt.Errorf("formatted text is required")
	}
	htmlBody, plainText := renderFormattedTextEmail(message)
	return s.recordReply(rc.To, rc.Subject, plainText, htmlBody, string(message.Format), rc.MessageID)
}

func (s *Stub) UpdateFormattedText(ctx context.Context, replyCtx any, message *core.FormattedText) error {
	// Email cannot edit messages; send a new reply instead.
	return s.ReplyFormattedText(ctx, replyCtx, message)
}

func (s *Stub) Stop() error {
	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}
	return nil
}

func (s *Stub) recordReply(to, subject, content, htmlBody, format, inReplyTo string) error {
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		To:        strings.TrimSpace(to),
		Subject:   strings.TrimSpace(subject),
		Content:   content,
		HTMLBody:  htmlBody,
		Format:    strings.TrimSpace(format),
		InReplyTo: strings.TrimSpace(inReplyTo),
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.WithFields(log.Fields{"component": "email-stub", "to": to, "subject": subject}).Info("Send: " + truncate(content, 80))
	return nil
}

func (s *Stub) handleTestMessage(w http.ResponseWriter, r *http.Request) {
	var req stubMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		req.UserID = "test@example.com"
	}
	if req.UserName == "" {
		req.UserName = "Test User"
	}
	if req.From == "" {
		req.From = req.UserID
	}
	if req.Subject == "" {
		req.Subject = "Test Email"
	}

	msg := &core.Message{
		Platform:   "email",
		SessionKey: fmt.Sprintf("email:%s:%s", req.From, req.UserID),
		UserID:     req.UserID,
		UserName:   req.UserName,
		ChatID:     req.From,
		Content:    req.Content,
		Timestamp:  time.Now(),
		IsGroup:    false,
		ReplyTarget: &core.ReplyTarget{
			Platform:  "email",
			ChatID:    req.From,
			ChannelID: req.From,
			UseReply:  true,
			Metadata:  map[string]string{"subject": req.Subject},
		},
	}
	msg.ReplyCtx = &emailReplyContext{
		To:      req.From,
		Subject: "Re: " + req.Subject,
	}

	if s.handler != nil {
		s.handler(s, msg)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Stub) handleGetReplies(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	replies := make([]stubReply, len(s.replies))
	copy(replies, s.replies)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(replies)
}

func (s *Stub) handleClearReplies(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.replies = nil
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

func toEmailReplyContext(raw any) emailReplyContext {
	switch value := raw.(type) {
	case emailReplyContext:
		return value
	case *emailReplyContext:
		if value == nil {
			return emailReplyContext{}
		}
		return *value
	case *core.Message:
		if value == nil {
			return emailReplyContext{}
		}
		if ctx, ok := value.ReplyCtx.(emailReplyContext); ok {
			return ctx
		}
		if ctx, ok := value.ReplyCtx.(*emailReplyContext); ok && ctx != nil {
			return *ctx
		}
		rc := emailReplyContext{To: strings.TrimSpace(value.ChatID)}
		if value.ReplyTarget != nil {
			rc.MessageID = strings.TrimSpace(value.ReplyTarget.MessageID)
			rc.Subject = metadataValue(value.ReplyTarget.Metadata, "subject")
		}
		return rc
	default:
		return emailReplyContext{}
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func metadataValue(metadata map[string]string, key string) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(metadata[key])
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Interface compliance assertions.
var (
	_ core.Platform            = (*Stub)(nil)
	_ core.StructuredSender    = (*Stub)(nil)
	_ core.FormattedTextSender = (*Stub)(nil)
	_ core.ReplyTargetResolver = (*Stub)(nil)
)
