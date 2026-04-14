package core

import "time"

// ReplyTarget is the serializable reply context that can survive backend
// persistence, reconnect, and cross-process delivery.
type ReplyTarget struct {
	Platform           string            `json:"platform"`
	ChatID             string            `json:"chatId,omitempty"`
	ChannelID          string            `json:"channelId,omitempty"`
	ThreadID           string            `json:"threadId,omitempty"`
	MessageID          string            `json:"messageId,omitempty"`
	InteractionToken   string            `json:"interactionToken,omitempty"`
	OriginalResponseID string            `json:"originalResponseId,omitempty"`
	CallbackToken      string            `json:"callbackToken,omitempty"`
	ResponseURL        string            `json:"responseUrl,omitempty"`
	SessionWebhook     string            `json:"sessionWebhook,omitempty"`
	ConversationID     string            `json:"conversationId,omitempty"`
	TopicID            string            `json:"topicId,omitempty"`
	UserID             string            `json:"userId,omitempty"`
	UseReply           bool              `json:"useReply,omitempty"`
	PreferEdit         bool              `json:"preferEdit,omitempty"`
	PreferredRenderer  string            `json:"preferredRenderer,omitempty"`
	ProgressMode       string            `json:"progressMode,omitempty"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}

// Attachment represents a file or media attachment in an incoming message.
type Attachment struct {
	Type     string `json:"type"`               // "image", "file", "audio", "video"
	Key      string `json:"key,omitempty"`       // platform-specific resource key (e.g. image_key, file_key)
	Name     string `json:"name,omitempty"`      // original filename
	MIMEType string `json:"mimeType,omitempty"`  // MIME type if known
	Size     int64  `json:"size,omitempty"`      // file size in bytes if known
	URL      string `json:"url,omitempty"`       // download URL if available
}

// Message represents an incoming IM message from any platform.
type Message struct {
	Platform    string
	SessionKey  string // format: platform:chatID:userID
	UserID      string
	UserName    string
	ChatID      string
	ChatName    string
	Content     string
	ReplyCtx    any // platform-specific reply context
	ReplyTarget *ReplyTarget
	Timestamp   time.Time
	IsGroup     bool              // group chat or DM
	ThreadID    string            // platform thread/reply chain ID for conversation isolation
	MessageType string            // original platform message type (e.g. "text", "post", "image")
	Metadata    map[string]string // platform-specific metadata
	Attachments []Attachment      // file/media attachments
}
