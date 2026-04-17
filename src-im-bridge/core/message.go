package core

import "time"

// ThreadPolicy controls how a reply relates to the parent message's thread
// structure. See reply_strategy.go for routing semantics.
type ThreadPolicy string

const (
	ThreadPolicyReuse   ThreadPolicy = "reuse"
	ThreadPolicyOpen    ThreadPolicy = "open"
	ThreadPolicyIsolate ThreadPolicy = "isolate"
)

// ReplyTarget is the serializable reply context that can survive backend
// persistence, reconnect, and cross-process delivery.
type ReplyTarget struct {
	Platform           string            `json:"platform"`
	TenantID           string            `json:"tenantId,omitempty"`
	ChatID             string            `json:"chatId,omitempty"`
	ChannelID          string            `json:"channelId,omitempty"`
	ThreadID           string            `json:"threadId,omitempty"`
	ThreadParentID     string            `json:"threadParentId,omitempty"`
	ThreadPolicy       ThreadPolicy      `json:"threadPolicy,omitempty"`
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

// AttachmentKind categorises the payload type of an attachment so provider-
// specific capability matrices can filter by semantic intent.
type AttachmentKind string

const (
	AttachmentKindFile   AttachmentKind = "file"
	AttachmentKindImage  AttachmentKind = "image"
	AttachmentKindLogs   AttachmentKind = "logs"
	AttachmentKindPatch  AttachmentKind = "patch"
	AttachmentKindReport AttachmentKind = "report"
	AttachmentKindAudio  AttachmentKind = "audio"
	AttachmentKindVideo  AttachmentKind = "video"
)

// Attachment represents a file or media artifact that moves through the
// bridge. `ContentRef` is a local staging path for ingress or a URL for
// egress. `ExternalRef` is the provider-native id assigned after upload.
type Attachment struct {
	ID          string            `json:"id,omitempty"`
	Kind        AttachmentKind    `json:"kind"`
	MimeType    string            `json:"mimeType,omitempty"`
	Filename    string            `json:"filename,omitempty"`
	SizeBytes   int64             `json:"sizeBytes,omitempty"`
	ContentRef  string            `json:"contentRef,omitempty"`
	ExternalRef string            `json:"externalRef,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// MessageKind identifies the semantic class of an inbound message so the
// engine can dispatch text, reactions, attachments, and system events to the
// right handler without string-sniffing.
type MessageKind string

const (
	MessageKindText       MessageKind = "text"
	MessageKindReaction   MessageKind = "reaction"
	MessageKindAttachment MessageKind = "attachment"
	MessageKindSystem     MessageKind = "system"
)

// Reaction is an emoji reaction event on a message. EmojiCode is the unified
// cross-provider code (see reaction_emoji.go); RawEmoji is the provider-native
// representation that surfaced the event.
type Reaction struct {
	UserID    string    `json:"userId,omitempty"`
	EmojiCode string    `json:"emojiCode,omitempty"`
	RawEmoji  string    `json:"rawEmoji,omitempty"`
	MessageID string    `json:"messageId,omitempty"`
	ReactedAt time.Time `json:"reactedAt,omitempty"`
	Removed   bool      `json:"removed,omitempty"`
}

// Message represents an incoming IM message from any platform.
type Message struct {
	Platform    string
	TenantID    string // resolved by TenantResolver; empty on unresolved / single-tenant legacy flows
	SessionKey  string // format: platform:chatID:userID
	UserID      string
	UserName    string
	ChatID      string
	ChatName    string
	Content     string
	Kind        MessageKind // text|reaction|attachment|system; zero-value treated as text
	ReplyCtx    any         // platform-specific reply context
	ReplyTarget *ReplyTarget
	Timestamp   time.Time
	IsGroup     bool              // group chat or DM
	ThreadID    string            // platform thread/reply chain ID for conversation isolation
	MessageType string            // original platform message type (e.g. "text", "post", "image")
	Metadata    map[string]string // platform-specific metadata
	Attachments []Attachment      // file/media attachments
	Reactions   []Reaction        // reaction events for this message
}
