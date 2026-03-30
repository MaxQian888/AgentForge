package core

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// mockPlatform is a minimal Platform for testing.
type mockPlatform struct {
	mu      sync.Mutex
	replies []string
	started MessageHandler
	stopped bool
}

func (m *mockPlatform) Name() string { return "mock" }
func (m *mockPlatform) Start(handler MessageHandler) error {
	m.started = handler
	return nil
}
func (m *mockPlatform) Stop() error {
	m.stopped = true
	return nil
}
func (m *mockPlatform) Send(ctx context.Context, chatID string, content string) error {
	m.mu.Lock()
	m.replies = append(m.replies, content)
	m.mu.Unlock()
	return nil
}
func (m *mockPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	m.mu.Lock()
	m.replies = append(m.replies, content)
	m.mu.Unlock()
	return nil
}

func TestEngine_SlashCommand(t *testing.T) {
	p := &mockPlatform{}
	e := NewEngine(p)

	var captured string
	e.RegisterCommand("/ping", func(plat Platform, msg *Message, args string) {
		captured = args
		_ = plat.Reply(context.Background(), msg.ReplyCtx, "pong: "+args)
	})

	msg := &Message{Content: "/ping hello world"}
	e.HandleMessage(p, msg)

	if captured != "hello world" {
		t.Errorf("expected args 'hello world', got %q", captured)
	}
	if len(p.replies) != 1 || p.replies[0] != "pong: hello world" {
		t.Errorf("unexpected replies: %v", p.replies)
	}
}

func TestEngine_UnknownCommand_WithFallback(t *testing.T) {
	p := &mockPlatform{}
	e := NewEngine(p)

	var fallbackCalled bool
	e.SetFallback(func(plat Platform, msg *Message) {
		fallbackCalled = true
	})

	msg := &Message{Content: "/unknown"}
	e.HandleMessage(p, msg)

	if !fallbackCalled {
		t.Error("expected fallback to be called for unknown command")
	}
}

func TestEngine_AtMention(t *testing.T) {
	p := &mockPlatform{}
	e := NewEngine(p)

	var fallbackMsg string
	e.SetFallback(func(plat Platform, msg *Message) {
		fallbackMsg = msg.Content
	})

	msg := &Message{Content: "@AgentForge please create a task"}
	e.HandleMessage(p, msg)

	if fallbackMsg != "@AgentForge please create a task" {
		t.Errorf("expected fallback with mention content, got %q", fallbackMsg)
	}
}

func TestEngine_DefaultHelp(t *testing.T) {
	p := &mockPlatform{}
	e := NewEngine(p)
	// No fallback set, no commands registered.

	msg := &Message{Content: "random text"}
	e.HandleMessage(p, msg)

	if len(p.replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(p.replies))
	}
	if p.replies[0] == "" {
		t.Error("expected non-empty help reply")
	}
}

func TestEngine_StartDelegatesToPlatform(t *testing.T) {
	p := &mockPlatform{}
	e := NewEngine(p)

	if err := e.Start(); err != nil {
		t.Fatalf("Start error: %v", err)
	}
	if p.started == nil {
		t.Fatal("expected platform Start to receive handler")
	}
}

func TestEngine_StopDelegatesToPlatform(t *testing.T) {
	p := &mockPlatform{}
	e := NewEngine(p)

	if err := e.Stop(); err != nil {
		t.Fatalf("Stop error: %v", err)
	}
	if !p.stopped {
		t.Fatal("expected platform Stop to be called")
	}
}

func TestEngine_SetRateLimiterBlocksRepeatedMessagesUsingDerivedKey(t *testing.T) {
	p := &mockPlatform{}
	e := NewEngine(p)
	e.RegisterCommand("/ping", func(plat Platform, msg *Message, args string) {
		_ = plat.Reply(context.Background(), msg.ReplyCtx, "pong")
	})

	rl := NewRateLimiter(1, time.Hour)
	rl.now = func() time.Time { return time.Unix(1, 0) }
	e.SetRateLimiter(rl)

	msg := &Message{
		Platform: "slack",
		ChatID:   "chat-1",
		UserID:   "user-1",
		Content:  "/ping",
	}
	e.HandleMessage(p, msg)
	e.HandleMessage(p, msg)

	if len(p.replies) != 2 {
		t.Fatalf("replies = %v", p.replies)
	}
	if p.replies[0] != "pong" {
		t.Fatalf("first reply = %q", p.replies[0])
	}
	if p.replies[1] == "pong" || !strings.Contains(p.replies[1], "频繁") {
		t.Fatalf("second reply = %q", p.replies[1])
	}
	if _, exists := rl.buckets["slack:chat-1:user-1"]; !exists {
		t.Fatalf("rate limiter buckets = %+v", rl.buckets)
	}
}

func TestEngine_UnknownSlashCommandWithoutFallbackRepliesWithHelp(t *testing.T) {
	p := &mockPlatform{}
	e := NewEngine(p)

	e.HandleMessage(p, &Message{Content: "/unknown"})

	if len(p.replies) != 1 {
		t.Fatalf("replies = %v", p.replies)
	}
	if !strings.Contains(p.replies[0], "/help") {
		t.Fatalf("help reply = %q", p.replies[0])
	}
}
