package feishu

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

// Stub is a test-only Feishu platform implementation that exposes HTTP
// endpoints for simulating incoming messages and inspecting replies.
type Stub struct {
	port    string
	handler core.MessageHandler
	server  *http.Server

	mu      sync.Mutex
	replies []stubReply
	cards   []stubCardReply
	native  []stubNativeReply
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

type stubNativeReply struct {
	ChatID    string              `json:"chat_id,omitempty"`
	Message   *core.NativeMessage `json:"message"`
	Updated   bool                `json:"updated"`
	Timestamp time.Time           `json:"timestamp"`
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
			CommandSurface:     core.CommandSurfaceMixed,
			StructuredSurface:  core.StructuredSurfaceCards,
			AsyncUpdateModes:   []core.AsyncUpdateMode{core.AsyncUpdateReply, core.AsyncUpdateDeferredCardUpdate},
			ActionCallbackMode: core.ActionCallbackWebhook,
			MessageScopes:      []core.MessageScope{core.MessageScopeChat, core.MessageScopeThread},
			Mutability: core.MutabilitySemantics{
				CanEdit:        true,
				PrefersInPlace: true,
			},
			SupportsRichMessages:  true,
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
		chatID = target.ChannelID
	}
	return &core.Message{ChatID: chatID, ReplyTarget: target}
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

	log.WithFields(log.Fields{"component": "feishu-stub", "port": s.port}).Info("Test server starting")
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithField("component", "feishu-stub").WithError(err).Error("Server error")
		}
	}()
	return nil
}

func (s *Stub) Reply(ctx context.Context, replyCtx any, content string) error {
	chatID := ""
	if msg, ok := replyCtx.(*core.Message); ok {
		chatID = msg.ChatID
	}
	if target, ok := replyCtx.(*core.ReplyTarget); ok {
		chatID = firstNonEmpty(target.ChatID, target.ChannelID)
	}
	s.mu.Lock()
	s.replies = append(s.replies, stubReply{
		ChatID:    chatID,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.WithFields(log.Fields{"component": "feishu-stub", "chat_id": chatID}).Info("Reply: " + content)
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
	log.WithFields(log.Fields{"component": "feishu-stub", "chat_id": chatID}).Info("Send: " + content)
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
	log.WithFields(log.Fields{"component": "feishu-stub", "chat_id": chatID, "card_title": card.Title}).Info("SendCard")
	return nil
}

// ReplyCard implements core.CardSender.
func (s *Stub) ReplyCard(ctx context.Context, replyCtx any, card *core.Card) error {
	chatID := ""
	if msg, ok := replyCtx.(*core.Message); ok {
		chatID = msg.ChatID
	}
	if target, ok := replyCtx.(*core.ReplyTarget); ok {
		chatID = firstNonEmpty(target.ChatID, target.ChannelID)
	}
	s.mu.Lock()
	s.cards = append(s.cards, stubCardReply{
		ChatID:    chatID,
		Card:      card,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	log.WithFields(log.Fields{"component": "feishu-stub", "chat_id": chatID, "card_title": card.Title}).Info("ReplyCard")
	return nil
}

func (s *Stub) SendNative(ctx context.Context, chatID string, message *core.NativeMessage) error {
	s.mu.Lock()
	s.native = append(s.native, stubNativeReply{
		ChatID:    chatID,
		Message:   message,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	return nil
}

func (s *Stub) ReplyNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	chatID := ""
	if msg, ok := replyCtx.(*core.Message); ok {
		chatID = msg.ChatID
	}
	if target, ok := replyCtx.(*core.ReplyTarget); ok {
		chatID = firstNonEmpty(target.ChatID, target.ChannelID)
	}
	s.mu.Lock()
	s.native = append(s.native, stubNativeReply{
		ChatID:    chatID,
		Message:   message,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
	return nil
}

func (s *Stub) UpdateNative(ctx context.Context, replyCtx any, message *core.NativeMessage) error {
	chatID := ""
	if target, ok := replyCtx.(*core.ReplyTarget); ok {
		chatID = firstNonEmpty(target.ChatID, target.ChannelID)
	}
	s.mu.Lock()
	s.native = append(s.native, stubNativeReply{
		ChatID:    chatID,
		Message:   message,
		Updated:   true,
		Timestamp: time.Now(),
	})
	s.mu.Unlock()
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
		ReplyTarget: &core.ReplyTarget{
			Platform:  "feishu",
			ChatID:    req.ChatID,
			ChannelID: req.ChatID,
			UseReply:  true,
		},
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
	s.native = nil
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}
