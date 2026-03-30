package qqbot

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
	Timestamp     time.Time `json:"timestamp"`
}

type stubMessageRequest struct {
	Content   string `json:"content"`
	UserID    string `json:"user_id"`
	UserName  string `json:"user_name"`
	ChatID    string `json:"chat_id"`
	MessageID string `json:"message_id"`
	IsGroup   bool   `json:"is_group"`
}

type replyContext struct {
	ChatID    string
	UserID    string
	MessageID string
	IsGroup   bool
}

func NewStub(port string) *Stub {
	return &Stub{port: port}
}

func (s *Stub) Name() string { return "qqbot-stub" }

func (s *Stub) Metadata() core.PlatformMetadata {
	return core.NormalizeMetadata(core.PlatformMetadata{
		Source: "qqbot",
		Capabilities: core.PlatformCapabilities{
			CommandSurface:         core.CommandSurfaceMixed,
			AsyncUpdateModes:       []core.AsyncUpdateMode{core.AsyncUpdateReply},
			ActionCallbackMode:     core.ActionCallbackWebhook,
			MessageScopes:          []core.MessageScope{core.MessageScopeChat},
			RequiresPublicCallback: true,
			SupportsMentions:       true,
			SupportsSlashCommands:  true,
		},
	}, "qqbot")
}

func (s *Stub) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	chatID := firstNonEmpty(target.ChatID, target.ChannelID, target.ConversationID)
	return replyContext{
		ChatID:    chatID,
		UserID:    strings.TrimSpace(target.UserID),
		MessageID: strings.TrimSpace(target.MessageID),
		IsGroup:   chatID != "" && chatID != strings.TrimSpace(target.UserID),
	}
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

	log.WithFields(log.Fields{"component": "qqbot-stub", "port": s.port}).Info("Test server starting")
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "qqbot-stub").WithError(err).Error("Server error")
		}
	}()
	return nil
}

func (s *Stub) Reply(ctx context.Context, rawReplyCtx any, content string) error {
	reply := toReplyContext(rawReplyCtx)
	target := firstNonEmpty(reply.ChatID, reply.UserID)
	if !reply.IsGroup && reply.UserID != "" {
		target = "user:" + reply.UserID
	}
	return s.Send(ctx, target, content)
}

func (s *Stub) Send(ctx context.Context, chatID string, content string) error {
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		ChatID:    chatID,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.WithFields(log.Fields{"component": "qqbot-stub", "chat_id": chatID}).Info("Send: " + content)
	return nil
}

func (s *Stub) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		ChatID:        chatID,
		Content:       message.FallbackText(),
		NativeSurface: message.SurfaceType(),
		Timestamp:     time.Now(),
	})
	s.mu.Unlock()
	log.WithFields(log.Fields{"component": "qqbot-stub", "chat_id": chatID}).Info("Send native: " + message.FallbackText())
	return nil
}

func (s *Stub) ReplyNative(ctx context.Context, rawReplyCtx any, message *core.NativeMessage) error {
	reply := toReplyContext(rawReplyCtx)
	target := firstNonEmpty(reply.ChatID, reply.UserID)
	if !reply.IsGroup && reply.UserID != "" {
		target = "user:" + reply.UserID
	}
	return s.SendNative(ctx, target, message)
}

func (s *Stub) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	return s.Send(ctx, chatID, strings.TrimSpace(message.FallbackText()))
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
		req.UserID = "qqbot-user"
	}
	if req.UserName == "" {
		req.UserName = "QQ Bot User"
	}
	if req.ChatID == "" {
		req.ChatID = "group-openid"
	}
	if req.MessageID == "" {
		req.MessageID = "evt-1"
	}
	if !req.IsGroup {
		req.IsGroup = true
	}

	reply := &core.ReplyTarget{
		Platform:  "qqbot",
		ChatID:    req.ChatID,
		ChannelID: req.ChatID,
		MessageID: req.MessageID,
		UserID:    req.UserID,
		UseReply:  true,
		Metadata: map[string]string{
			"scope": map[bool]string{true: "group", false: "user"}[req.IsGroup],
		},
	}
	if !req.IsGroup {
		reply.ChatID = req.UserID
		reply.ChannelID = req.UserID
	}

	msg := &core.Message{
		Platform:    s.Name(),
		SessionKey:  fmt.Sprintf("%s:%s:%s", s.Name(), reply.ChatID, req.UserID),
		UserID:      req.UserID,
		UserName:    req.UserName,
		ChatID:      reply.ChatID,
		Content:     req.Content,
		Timestamp:   time.Now(),
		IsGroup:     req.IsGroup,
		ReplyTarget: reply,
	}
	msg.ReplyCtx = replyContext{
		ChatID:    reply.ChatID,
		UserID:    req.UserID,
		MessageID: req.MessageID,
		IsGroup:   req.IsGroup,
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

func toReplyContext(raw any) replyContext {
	switch value := raw.(type) {
	case replyContext:
		return value
	case *replyContext:
		if value == nil {
			return replyContext{}
		}
		return *value
	case *core.Message:
		if value == nil {
			return replyContext{}
		}
		if ctx, ok := value.ReplyCtx.(replyContext); ok {
			return ctx
		}
		if ctx, ok := value.ReplyCtx.(*replyContext); ok && ctx != nil {
			return *ctx
		}
		return replyContext{
			ChatID:    value.ChatID,
			UserID:    value.UserID,
			MessageID: messageIDFromTarget(value.ReplyTarget),
			IsGroup:   value.IsGroup,
		}
	case *core.ReplyTarget:
		if value == nil {
			return replyContext{}
		}
		chatID := firstNonEmpty(value.ChatID, value.ChannelID, value.ConversationID)
		return replyContext{
			ChatID:    chatID,
			UserID:    strings.TrimSpace(value.UserID),
			MessageID: strings.TrimSpace(value.MessageID),
			IsGroup:   chatID != "" && chatID != strings.TrimSpace(value.UserID),
		}
	default:
		return replyContext{}
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

func messageIDFromTarget(target *core.ReplyTarget) string {
	if target == nil {
		return ""
	}
	return strings.TrimSpace(target.MessageID)
}
