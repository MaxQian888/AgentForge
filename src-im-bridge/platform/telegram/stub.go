package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
)

type Stub struct {
	port    string
	handler core.MessageHandler
	server  *http.Server

	mu      sync.Mutex
	replies []stubReply
}

type stubReply struct {
	ChatID        string    `json:"chat_id,omitempty"`
	Content       string    `json:"content"`
	NativeSurface string    `json:"native_surface,omitempty"`
	Format        string    `json:"format,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
}

type stubMessageRequest struct {
	Content  string `json:"content"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	ChatID   string `json:"chat_id"`
	IsGroup  bool   `json:"is_group"`
}

func NewStub(port string) *Stub {
	return &Stub{port: port}
}

func (s *Stub) Name() string { return "telegram-stub" }

func (s *Stub) Metadata() core.PlatformMetadata {
	return core.NormalizeMetadata(core.PlatformMetadata{
		Source: "telegram",
		Capabilities: core.PlatformCapabilities{
			CommandSurface:     core.CommandSurfaceMixed,
			StructuredSurface:  core.StructuredSurfaceInlineKeyboard,
			AsyncUpdateModes:   []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateEdit},
			ActionCallbackMode: core.ActionCallbackQuery,
			MessageScopes:      []core.MessageScope{core.MessageScopeChat, core.MessageScopeTopic},
			Mutability: core.MutabilitySemantics{
				CanEdit:        true,
				PrefersInPlace: true,
			},
			SupportsSlashCommands: true,
			SupportsMentions:      true,
		},
	}, "telegram")
}

func (s *Stub) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	chatID := target.ChatID
	if chatID == "" {
		chatID = target.ChannelID
	}
	return &core.Message{ChatID: chatID, ReplyTarget: target}
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

	log.WithFields(log.Fields{"component": "telegram-stub", "port": s.port}).Info("Test server starting")
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "telegram-stub").WithError(err).Error("Server error")
		}
	}()
	return nil
}

func (s *Stub) Reply(ctx context.Context, replyCtx any, content string) error {
	return s.recordReply(chatIDFromReplyContext(replyCtx), content, "", "")
}

func (s *Stub) Send(ctx context.Context, chatID string, content string) error {
	return s.recordReply(chatID, content, "", "")
}

func (s *Stub) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	return s.recordReply(chatID, message.FallbackText(), message.SurfaceType(), "")
}

func (s *Stub) ReplyNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	return s.recordReply(chatIDFromReplyContext(replyCtx), message.FallbackText(), message.SurfaceType(), "")
}

func (s *Stub) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	textMessage := telegramTextMessage{Text: structuredFallbackText(message)}
	if message != nil && len(message.Sections) > 0 {
		textMessage, _ = renderStructuredSections(message.Sections)
	}
	return s.recordReply(chatID, textMessage.Text, "", "")
}

func (s *Stub) SendFormattedText(ctx context.Context, chatID string, message *core.FormattedText) error {
	if message == nil {
		return fmt.Errorf("formatted text is required")
	}
	segments := renderTelegramText(message)
	if len(segments) == 0 {
		return fmt.Errorf("formatted text content is required")
	}
	return s.recordReply(chatID, segments[0].Text, "", string(message.Format))
}

func (s *Stub) ReplyFormattedText(ctx context.Context, replyCtx any, message *core.FormattedText) error {
	return s.SendFormattedText(ctx, chatIDFromReplyContext(replyCtx), message)
}

func (s *Stub) UpdateFormattedText(ctx context.Context, replyCtx any, message *core.FormattedText) error {
	return s.ReplyFormattedText(ctx, replyCtx, message)
}

func (s *Stub) recordReply(chatID, content, nativeSurface, format string) error {
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		ChatID:        strings.TrimSpace(chatID),
		Content:       content,
		NativeSurface: strings.TrimSpace(nativeSurface),
		Format:        strings.TrimSpace(format),
		Timestamp:     time.Now(),
	})
	s.mu.Unlock()
	log.WithFields(log.Fields{"component": "telegram-stub", "chat_id": chatID}).Info("Send: " + content)
	return nil
}

func (s *Stub) Stop() error {
	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}
	return nil
}

func (s *Stub) handleTestMessage(w http.ResponseWriter, r *http.Request) {
	var req stubMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		req.UserID = "telegram-user"
	}
	if req.UserName == "" {
		req.UserName = "Telegram User"
	}
	if req.ChatID == "" {
		req.ChatID = "123456"
	}

	msg := &core.Message{
		Platform:   s.Name(),
		SessionKey: fmt.Sprintf("%s:%s:%s", s.Name(), req.ChatID, req.UserID),
		UserID:     req.UserID,
		UserName:   req.UserName,
		ChatID:     req.ChatID,
		Content:    req.Content,
		Timestamp:  time.Now(),
		IsGroup:    req.IsGroup,
		ReplyTarget: &core.ReplyTarget{
			Platform:  "telegram",
			ChatID:    req.ChatID,
			ChannelID: req.ChatID,
			UseReply:  true,
		},
	}
	msg.ReplyCtx = msg

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

func chatIDFromReplyContext(replyCtx any) string {
	switch value := replyCtx.(type) {
	case *core.Message:
		if value != nil {
			return value.ChatID
		}
	case *core.ReplyTarget:
		if value != nil {
			if trimmed := strings.TrimSpace(value.ChatID); trimmed != "" {
				return trimmed
			}
			return strings.TrimSpace(value.ChannelID)
		}
	}
	return ""
}
