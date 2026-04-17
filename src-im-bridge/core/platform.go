package core

import "context"

// Platform abstracts an IM platform (Feishu, Slack, etc.)
type Platform interface {
	Name() string
	Start(handler MessageHandler) error
	Reply(ctx context.Context, replyCtx any, content string) error
	Send(ctx context.Context, chatID string, content string) error
	Stop() error
}

// MessageHandler processes incoming IM messages.
type MessageHandler func(p Platform, msg *Message)

// CardSender is an optional interface for platforms with rich card messaging.
type CardSender interface {
	SendCard(ctx context.Context, chatID string, card *Card) error
	ReplyCard(ctx context.Context, replyCtx any, card *Card) error
}

// StructuredSender is an optional interface for platforms that can natively
// render structured message payloads without first converting to legacy cards.
type StructuredSender interface {
	SendStructured(ctx context.Context, chatID string, message *StructuredMessage) error
}

// ReplyStructuredSender is an optional interface for platforms that can render
// structured payloads back into an existing reply/update context.
type ReplyStructuredSender interface {
	ReplyStructured(ctx context.Context, replyCtx any, message *StructuredMessage) error
}

// FormattedText captures a text payload with an explicit provider text mode.
type FormattedText struct {
	Content string
	Format  TextFormatMode
}

// FormattedTextSender is an optional interface for platforms that support
// provider-aware text formatting modes beyond plain text.
type FormattedTextSender interface {
	SendFormattedText(ctx context.Context, chatID string, message *FormattedText) error
	ReplyFormattedText(ctx context.Context, replyCtx any, message *FormattedText) error
	UpdateFormattedText(ctx context.Context, replyCtx any, message *FormattedText) error
}

// NativeTextMessageBuilder is an optional interface for platforms that can
// build a provider-native richer message representation from plain completion
// text.
type NativeTextMessageBuilder interface {
	BuildNativeTextMessage(title, content string) (*NativeMessage, error)
}

// NativeMessageSender is an optional interface for platforms that support a
// provider-native payload surface that cannot be faithfully represented as a
// generic structured message.
type NativeMessageSender interface {
	SendNative(ctx context.Context, chatID string, message *NativeMessage) error
	ReplyNative(ctx context.Context, replyCtx any, message *NativeMessage) error
}

// NativeMessageUpdater is an optional interface for platforms that can update
// a previously-sent provider-native payload in place.
type NativeMessageUpdater interface {
	UpdateNative(ctx context.Context, replyCtx any, message *NativeMessage) error
}

// MessageUpdater is an optional interface for platforms that support editing messages.
type MessageUpdater interface {
	UpdateMessage(ctx context.Context, replyCtx any, content string) error
}

// ReplyTargetResolver converts a serialized reply target back into the
// platform-specific reply context needed by Reply/UpdateMessage.
type ReplyTargetResolver interface {
	ReplyContextFromTarget(target *ReplyTarget) any
}

// TypingIndicator is an optional interface for platforms that support typing indicators.
type TypingIndicator interface {
	StartTyping(ctx context.Context, chatID string) error
	StopTyping(ctx context.Context, chatID string) error
}

// LifecycleHandler is an optional callback interface for bot lifecycle events
// such as being added to or removed from a group chat.
type LifecycleHandler interface {
	OnBotAdded(ctx context.Context, platform Platform, chatID string) error
	OnBotRemoved(ctx context.Context, platform Platform, chatID string) error
}

// ReconcileConfig carries the new credentials or tunable values the bridge
// has read from its environment (or secrets vault in a future iteration)
// and wants the provider to reconcile against without a process restart.
type ReconcileConfig struct {
	// Credentials is a key-value map of provider-specific secrets. Keys
	// mirror the env-var names (e.g. FEISHU_APP_SECRET, SLACK_BOT_TOKEN).
	// Providers MUST treat missing keys as "leave unchanged".
	Credentials map[string]string
}

// ReconcileResult reports what the provider reconciled in response to a
// ReconcileConfig. Applied and Deferred are human-readable field names
// used in operator logs/audit; they do not affect control flow.
type ReconcileResult struct {
	Applied  []string
	Deferred []string
	Errors   []error
}

// HotReloader is an optional interface for platforms that can refresh
// credentials or transport state in-place, typically in response to SIGHUP.
// Providers that cannot honor hot reload (e.g. webhook-port bound) MUST NOT
// implement this interface; the bridge's SIGHUP handler treats them as
// `manual_restart_required`.
type HotReloader interface {
	Reconcile(ctx context.Context, cfg ReconcileConfig) ReconcileResult
}
