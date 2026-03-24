package core

import (
	"context"
	"strings"
	"sync"
)

// CommandHandler handles a slash command with parsed arguments.
type CommandHandler func(p Platform, msg *Message, args string)

// Engine routes incoming messages to command handlers or a fallback.
type Engine struct {
	mu          sync.RWMutex
	commands    map[string]CommandHandler
	platform    Platform
	fallback    func(p Platform, msg *Message)
	rateLimiter *RateLimiter
}

// NewEngine creates an engine bound to a specific platform.
func NewEngine(platform Platform) *Engine {
	return &Engine{
		commands: make(map[string]CommandHandler),
		platform: platform,
	}
}

// SetRateLimiter sets the rate limiter for the engine.
func (e *Engine) SetRateLimiter(rl *RateLimiter) {
	e.rateLimiter = rl
}

// RegisterCommand registers a slash command handler (e.g. "/task").
func (e *Engine) RegisterCommand(cmd string, handler CommandHandler) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.commands[cmd] = handler
}

// SetFallback sets a handler for messages that don't match any command.
func (e *Engine) SetFallback(handler func(p Platform, msg *Message)) {
	e.fallback = handler
}

// HandleMessage routes a message to the appropriate handler.
func (e *Engine) HandleMessage(p Platform, msg *Message) {
	content := strings.TrimSpace(msg.Content)

	// Rate limit check.
	if e.rateLimiter != nil {
		key := msg.SessionKey
		if key == "" {
			key = msg.Platform + ":" + msg.ChatID + ":" + msg.UserID
		}
		if !e.rateLimiter.Allow(key) {
			_ = p.Reply(context.Background(), msg.ReplyCtx, "操作过于频繁，请稍后再试。")
			return
		}
	}

	// Check for slash commands.
	if strings.HasPrefix(content, "/") {
		parts := strings.SplitN(content, " ", 2)
		cmd := parts[0]
		args := ""
		if len(parts) > 1 {
			args = parts[1]
		}

		e.mu.RLock()
		handler, exists := e.commands[cmd]
		e.mu.RUnlock()

		if exists {
			handler(p, msg, args)
			return
		}
	}

	// Check for @AgentForge mention or use fallback for any non-command.
	if strings.Contains(content, "@AgentForge") || e.fallback != nil {
		if e.fallback != nil {
			e.fallback(p, msg)
			return
		}
	}

	// Default: echo help.
	_ = p.Reply(context.Background(), msg.ReplyCtx,
		"发送 /help 查看可用命令，或 @AgentForge <你的需求> 使用自然语言")
}

// Start starts the platform and begins receiving messages.
func (e *Engine) Start() error {
	return e.platform.Start(e.HandleMessage)
}

// Stop gracefully stops the platform.
func (e *Engine) Stop() error {
	return e.platform.Stop()
}
