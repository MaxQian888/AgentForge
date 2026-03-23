package slack

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/agentforge/im-bridge/core"
)

// Stub is a test-focused Slack platform adapter with HTTP endpoints for local verification.
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

func (s *Stub) Name() string { return "slack-stub" }

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

	log.Printf("[slack-stub] Test server starting on :%s", s.port)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[slack-stub] Server error: %v", err)
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
	log.Printf("[slack-stub] Send to %s: %s", chatID, content)
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
		req.UserID = "slack-user"
	}
	if req.UserName == "" {
		req.UserName = "Slack User"
	}
	if req.ChatID == "" {
		req.ChatID = "slack-chat"
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
