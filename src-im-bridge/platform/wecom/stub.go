package wecom

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
	Content     string `json:"content"`
	UserID      string `json:"user_id"`
	UserName    string `json:"user_name"`
	ChatID      string `json:"chat_id"`
	ResponseURL string `json:"response_url"`
	IsGroup     bool   `json:"is_group"`
}

func NewStub(port string) *Stub {
	return &Stub{port: port}
}

func (s *Stub) Name() string { return "wecom-stub" }

func (s *Stub) Metadata() core.PlatformMetadata {
	return core.NormalizeMetadata(baseMetadata(), "wecom")
}

func (s *Stub) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	return replyContext{
		ResponseURL: target.SessionWebhook,
		ChatID:      firstNonEmpty(target.ChatID, target.ChannelID, target.ConversationID),
		UserID:      target.UserID,
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

	log.WithFields(log.Fields{"component": "wecom-stub", "port": s.port}).Info("Test server starting")
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "wecom-stub").WithError(err).Error("Server error")
		}
	}()
	return nil
}

func (s *Stub) Reply(ctx context.Context, replyCtx any, content string) error {
	reply := toReplyContext(replyCtx)
	target := firstNonEmpty(reply.ChatID, reply.UserID)
	return s.recordReply(target, content, "")
}

func (s *Stub) Send(ctx context.Context, chatID string, content string) error {
	return s.recordReply(chatID, content, "")
}

func (s *Stub) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	return s.recordReply(chatID, message.FallbackText(), message.SurfaceType())
}

func (s *Stub) ReplyNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	if err := message.Validate(); err != nil {
		return err
	}
	reply := toReplyContext(replyCtx)
	target := firstNonEmpty(reply.ChatID, reply.UserID)
	return s.recordReply(target, message.FallbackText(), message.SurfaceType())
}

func (s *Stub) recordReply(chatID, content, nativeSurface string) error {
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		ChatID:        strings.TrimSpace(chatID),
		Content:       content,
		NativeSurface: strings.TrimSpace(nativeSurface),
		Timestamp:     time.Now(),
	})
	s.mu.Unlock()
	log.WithFields(log.Fields{"component": "wecom-stub", "chat_id": chatID}).Info("Send: " + content)
	return nil
}

func (s *Stub) SendStructured(ctx context.Context, chatID string, message *core.StructuredMessage) error {
	return s.Send(ctx, chatID, renderStructuredFallback(message))
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
		req.UserID = "wecom-user"
	}
	if req.UserName == "" {
		req.UserName = "WeCom User"
	}
	if req.ChatID == "" {
		req.ChatID = "wecom-chat"
	}
	if req.ResponseURL == "" {
		req.ResponseURL = "https://work.weixin.qq.com/response"
	}

	msg := &core.Message{
		Platform:   "wecom",
		SessionKey: fmt.Sprintf("wecom:%s:%s", req.ChatID, req.UserID),
		UserID:     req.UserID,
		UserName:   req.UserName,
		ChatID:     req.ChatID,
		Content:    req.Content,
		Timestamp:  time.Now(),
		IsGroup:    req.IsGroup,
		ReplyTarget: &core.ReplyTarget{
			Platform:       "wecom",
			ChatID:         req.ChatID,
			ChannelID:      req.ChatID,
			ConversationID: req.ChatID,
			SessionWebhook: req.ResponseURL,
			UserID:         req.UserID,
			UseReply:       true,
		},
	}
	msg.ReplyCtx = replyContext{
		ResponseURL: req.ResponseURL,
		ChatID:      req.ChatID,
		UserID:      req.UserID,
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
