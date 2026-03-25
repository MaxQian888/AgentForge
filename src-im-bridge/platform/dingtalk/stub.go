package dingtalk

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
)

// Stub is a test-focused DingTalk platform adapter with HTTP endpoints for local verification.
type Stub struct {
	port    string
	handler core.MessageHandler
	server  *http.Server

	mu      sync.Mutex
	replies []stubReply
}

type stubReply struct {
	ChatID    string    `json:"chat_id,omitempty"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
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

func (s *Stub) Name() string { return "dingtalk-stub" }

func (s *Stub) Metadata() core.PlatformMetadata {
	return core.PlatformMetadata{
		Source: "dingtalk",
		Capabilities: core.PlatformCapabilities{
			CommandSurface:        core.CommandSurfaceMixed,
			StructuredSurface:     core.StructuredSurfaceActionCard,
			AsyncUpdateModes:      []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateSessionWebhook},
			ActionCallbackMode:    core.ActionCallbackWebhook,
			MessageScopes:         []core.MessageScope{core.MessageScopeChat},
			SupportsSlashCommands: true,
			SupportsMentions:      true,
		},
	}
}

func (s *Stub) ReplyContextFromTarget(target *core.ReplyTarget) any {
	if target == nil {
		return nil
	}
	chatID := target.ChatID
	if chatID == "" {
		chatID = target.ConversationID
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

	log.WithFields(log.Fields{"component": "dingtalk-stub", "port": s.port}).Info("Test server starting")
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "dingtalk-stub").WithError(err).Error("Server error")
		}
	}()
	return nil
}

func (s *Stub) Reply(ctx context.Context, replyCtx any, content string) error {
	chatID := ""
	if msg, ok := replyCtx.(*core.Message); ok {
		chatID = msg.ChatID
	}
	return s.Send(ctx, chatID, content)
}

func (s *Stub) Send(ctx context.Context, chatID string, content string) error {
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		ChatID:    chatID,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.WithFields(log.Fields{"component": "dingtalk-stub", "chat_id": chatID}).Info("Send: " + content)
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
		req.UserID = "dingtalk-user"
	}
	if req.UserName == "" {
		req.UserName = "DingTalk User"
	}
	if req.ChatID == "" {
		req.ChatID = "dingtalk-chat"
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
			Platform:       "dingtalk",
			ChatID:         req.ChatID,
			ChannelID:      req.ChatID,
			ConversationID: req.ChatID,
			UseReply:       true,
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
