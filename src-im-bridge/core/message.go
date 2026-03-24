package core

import "time"

// ReplyTarget is the serializable reply context that can survive backend
// persistence, reconnect, and cross-process delivery.
type ReplyTarget struct {
	Platform         string            `json:"platform"`
	ChatID           string            `json:"chatId,omitempty"`
	ChannelID        string            `json:"channelId,omitempty"`
	ThreadID         string            `json:"threadId,omitempty"`
	MessageID        string            `json:"messageId,omitempty"`
	InteractionToken string            `json:"interactionToken,omitempty"`
	ResponseURL      string            `json:"responseUrl,omitempty"`
	SessionWebhook   string            `json:"sessionWebhook,omitempty"`
	ConversationID   string            `json:"conversationId,omitempty"`
	UserID           string            `json:"userId,omitempty"`
	UseReply         bool              `json:"useReply,omitempty"`
	PreferEdit       bool              `json:"preferEdit,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// Message represents an incoming IM message from any platform.
type Message struct {
	Platform   string
	SessionKey string // format: platform:chatID:userID
	UserID     string
	UserName   string
	ChatID     string
	ChatName   string
	Content    string
	ReplyCtx   any       // platform-specific reply context
	ReplyTarget *ReplyTarget
	Timestamp  time.Time
	IsGroup    bool   // group chat or DM
	ThreadID   string // platform thread/reply chain ID for conversation isolation
}
