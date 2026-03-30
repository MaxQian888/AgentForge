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
