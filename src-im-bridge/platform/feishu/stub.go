package feishu

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

// Stub is a test-only Feishu platform implementation that exposes HTTP
// endpoints for simulating incoming messages and inspecting replies.
type Stub struct {
	port    string
	handler core.MessageHandler
	server  *http.Server

	mu      sync.Mutex
	replies []stubReply
	cards   []stubCardReply
}

type stubReply struct {
	ChatID    string    `json:"chat_id,omitempty"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

type stubCardReply struct {
	ChatID    string     `json:"chat_id,omitempty"`
	Card      *core.Card `json:"card"`
	Timestamp time.Time  `json:"timestamp"`
}

type stubMessageRequest struct {
	Content  string `json:"content"`
	UserID   string `json:"user_id"`
	UserName string `json:"user_name"`
	ChatID   string `json:"chat_id"`
	IsGroup  bool   `json:"is_group"`
}

// NewStub creates a stub Feishu platform for testing.
func NewStub(port string) *Stub {
	return &Stub{port: port}
}

func (s *Stub) Name() string { return "feishu-stub" }

func (s *Stub) Metadata() core.PlatformMetadata {
	return core.PlatformMetadata{
		Source: "feishu",
		Capabilities: core.PlatformCapabilities{
			SupportsRichMessages:  true,
			SupportsSlashCommands: true,
			SupportsMentions:      true,
		},
	}
}

func (s *Stub) Start(handler core.MessageHandler) error {
	s.handler = handler

	mux := http.NewServeMux()
	mux.HandleFunc("POST /test/message", s.handleTestMessage)
	mux.HandleFunc("GET /test/replies", s.handleGetReplies)
	mux.HandleFunc("GET /test/cards", s.handleGetCards)
	mux.HandleFunc("DELETE /test/replies", s.handleClearReplies)

	s.server = &http.Server{
		Addr:    ":" + s.port,
		Handler: mux,
	}

	log.Printf("[feishu-stub] Test server starting on :%s", s.port)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[feishu-stub] Server error: %v", err)
		}
	}()
	return nil
}

func (s *Stub) Reply(ctx context.Context, replyCtx any, content string) error {
	chatID := ""
	if msg, ok := replyCtx.(*core.Message); ok {
		chatID = msg.ChatID
	}
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		ChatID:    chatID,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.Printf("[feishu-stub] Reply to %s: %s", chatID, content)
	return nil
}

func (s *Stub) Send(ctx context.Context, chatID string, content string) error {
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		ChatID:    chatID,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.Printf("[feishu-stub] Send to %s: %s", chatID, content)
	return nil
}

// SendCard implements core.CardSender.
func (s *Stub) SendCard(ctx context.Context, chatID string, card *core.Card) error {
	s.mu.Lock()
	s.cards = append(s.cards, stubCardReply{
		ChatID:    chatID,
		Card:      card,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.Printf("[feishu-stub] SendCard to %s: %s", chatID, card.Title)
	return nil
}

// ReplyCard implements core.CardSender.
func (s *Stub) ReplyCard(ctx context.Context, replyCtx any, card *core.Card) error {
	chatID := ""
	if msg, ok := replyCtx.(*core.Message); ok {
		chatID = msg.ChatID
	}
	s.mu.Lock()
	s.cards = append(s.cards, stubCardReply{
		ChatID:    chatID,
		Card:      card,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.Printf("[feishu-stub] ReplyCard to %s: %s", chatID, card.Title)
	return nil
}

func (s *Stub) Stop() error {
	if s.server != nil {
		return s.server.Shutdown(context.Background())
	}
	return nil
}

// --- HTTP handlers for testing ---

func (s *Stub) handleTestMessage(w http.ResponseWriter, r *http.Request) {
	var req stubMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	if req.UserID == "" {
		req.UserID = "test-user"
	}
	if req.UserName == "" {
		req.UserName = "Test User"
	}
	if req.ChatID == "" {
		req.ChatID = "test-chat"
	}

	msg := &core.Message{
		Platform:   "feishu-stub",
		SessionKey: fmt.Sprintf("feishu-stub:%s:%s", req.ChatID, req.UserID),
		UserID:     req.UserID,
		UserName:   req.UserName,
		ChatID:     req.ChatID,
		Content:    req.Content,
		ReplyCtx:   nil, // will be set below
		Timestamp:  time.Now(),
		IsGroup:    req.IsGroup,
	}
	msg.ReplyCtx = msg // use message itself as reply context

	if s.handler != nil {
		s.handler(s, msg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (s *Stub) handleGetReplies(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	replies := make([]stubReply, len(s.replies))
	copy(replies, s.replies)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(replies)
}

func (s *Stub) handleGetCards(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	cards := make([]stubCardReply, len(s.cards))
	copy(cards, s.cards)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cards)
}

func (s *Stub) handleClearReplies(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.replies = nil
	s.cards = nil
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}
