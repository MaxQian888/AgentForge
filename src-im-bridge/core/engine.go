package core

import (
	"context"
	"strings"
	"sync"
	"time"
)

// CommandHandler handles a slash command with parsed arguments.
type CommandHandler func(p Platform, msg *Message, args string)

// Engine routes incoming messages to command handlers or a fallback.
type Engine struct {
	mu                    sync.RWMutex
	commands              map[string]CommandHandler
	platform              Platform
	fallback              func(p Platform, msg *Message)
	rateLimiter           *RateLimiter
	bridgeCapabilityProbe BridgeCapabilityProbe
	bridgeCapabilityTTL   time.Duration
	bridgeCapabilityCache map[BridgeCapability]bridgeCapabilityCacheEntry
	bridgeID              string
	allowlist             *CommandAllowlist
}

// SetCommandAllowlist installs a coarse-grained command gate. Passing nil
// or an empty allowlist makes the gate admit everything (default).
func (e *Engine) SetCommandAllowlist(al *CommandAllowlist) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.allowlist = al
}

// SetBridgeID attaches the stable bridge_id to the engine so rate limiting
// policies can bucket on the DimBridge axis.
func (e *Engine) SetBridgeID(id string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bridgeID = strings.TrimSpace(id)
}

// NewEngine creates an engine bound to a specific platform.
func NewEngine(platform Platform) *Engine {
	return &Engine{
		commands:              make(map[string]CommandHandler),
		platform:              platform,
		bridgeCapabilityTTL:   15 * time.Second,
		bridgeCapabilityCache: make(map[BridgeCapability]bridgeCapabilityCacheEntry),
	}
}

// SetRateLimiter sets the rate limiter for the engine.
func (e *Engine) SetRateLimiter(rl *RateLimiter) {
	e.rateLimiter = rl
}

func (e *Engine) SetBridgeCapabilityProbe(probe BridgeCapabilityProbe) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.bridgeCapabilityProbe = probe
	e.bridgeCapabilityCache = make(map[BridgeCapability]bridgeCapabilityCacheEntry)
}

func (e *Engine) SetBridgeCapabilityTTL(ttl time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if ttl <= 0 {
		ttl = 15 * time.Second
	}
	e.bridgeCapabilityTTL = ttl
	e.bridgeCapabilityCache = make(map[BridgeCapability]bridgeCapabilityCacheEntry)
}

func (e *Engine) ResolveCommandRoute(command, subcommand string) CommandRoute {
	return resolveCommandRoute(command, subcommand)
}

func (e *Engine) BridgeCapabilityAvailable(ctx context.Context, capability BridgeCapability) (bool, error) {
	capability = BridgeCapability(strings.TrimSpace(string(capability)))
	if capability == "" {
		return false, nil
	}

	e.mu.RLock()
	probe := e.bridgeCapabilityProbe
	ttl := e.bridgeCapabilityTTL
	cached, ok := e.bridgeCapabilityCache[capability]
	e.mu.RUnlock()

	if probe == nil {
		return true, nil
	}

	now := time.Now()
	if ok && ttl > 0 && now.Sub(cached.checkedAt) < ttl {
		return cached.err == nil, cached.err
	}

	err := probe.Check(ctx, capability)

	e.mu.Lock()
	e.bridgeCapabilityCache[capability] = bridgeCapabilityCacheEntry{
		checkedAt: now,
		err:       err,
	}
	e.mu.Unlock()

	return err == nil, err
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

	// Parse slash command upfront so rate limit scope carries command +
	// action_class information for multi-dim policies.
	var cmd, args string
	if strings.HasPrefix(content, "/") {
		parts := strings.SplitN(content, " ", 2)
		cmd = parts[0]
		if len(parts) > 1 {
			args = parts[1]
		}
	}
	actionClass := ActionClassForCommand(cmd)

	// Command allowlist gate (applies before rate limit so denied commands
	// never enter the rate counters).
	if cmd != "" && e.allowlist != nil && e.allowlist.Enabled() {
		if !e.allowlist.Permit(msg.Platform, cmd) {
			_ = p.Reply(context.Background(), msg.ReplyCtx, "该命令在此平台未启用，请联系管理员。")
			return
		}
	}

	// Rate limit check.
	if e.rateLimiter != nil {
		scope := Scope{
			Chat:        msg.ChatID,
			User:        msg.UserID,
			Bridge:      e.bridgeID,
			Command:     cmd,
			ActionClass: actionClass,
		}
		decision, err := e.rateLimiter.Allow(context.Background(), scope)
		if err != nil {
			// Fail closed: treat rate store errors as refusal to avoid
			// silently admitting traffic under storage outages.
			_ = p.Reply(context.Background(), msg.ReplyCtx, "限速检查失败，请稍后再试。")
			return
		}
		if !decision.Allowed {
			_ = p.Reply(context.Background(), msg.ReplyCtx, "操作过于频繁，请稍后再试（policy="+decision.Policy+"）。")
			return
		}
	}

	// Check for slash commands.
	if cmd != "" {
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

// ExecuteCommand runs a slash command programmatically as if the user had
// typed it. Returns true if a handler was found and executed.
func (e *Engine) ExecuteCommand(p Platform, msg *Message, command string) bool {
	content := strings.TrimSpace(command)
	if !strings.HasPrefix(content, "/") {
		return false
	}
	parts := strings.SplitN(content, " ", 2)
	cmd := parts[0]
	args := ""
	if len(parts) > 1 {
		args = parts[1]
	}

	e.mu.RLock()
	handler, exists := e.commands[cmd]
	e.mu.RUnlock()

	if !exists {
		return false
	}
	handler(p, msg, args)
	return true
}

// Start starts the platform and begins receiving messages.
func (e *Engine) Start() error {
	return e.platform.Start(e.HandleMessage)
}

// Stop gracefully stops the platform.
func (e *Engine) Stop() error {
	return e.platform.Stop()
}
