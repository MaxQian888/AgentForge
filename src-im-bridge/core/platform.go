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
