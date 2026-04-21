package model

import (
	"encoding/json"
	"strings"
	"time"
)

// IMMessageRequest represents an incoming IM message webhook.
type IMMessageRequest struct {
	Platform  string `json:"platform" validate:"required,oneof=feishu dingtalk slack telegram discord wecom wechat qq qqbot email"`
	ChannelID string `json:"channelId" validate:"required"`
	UserID    string `json:"userId"`
	UserName  string `json:"userName"`
	Text      string `json:"text" validate:"required"`
	ThreadID  string `json:"threadId,omitempty"`
	ReplyToID string `json:"replyToId,omitempty"`
}

// IMMessageResponse is the reply to an IM message.
type IMMessageResponse struct {
	Reply    string `json:"reply"`
	ThreadID string `json:"threadId,omitempty"`
}

// IMCommandRequest represents a slash command from IM.
type IMCommandRequest struct {
	Platform  string         `json:"platform" validate:"required"`
	Command   string         `json:"command" validate:"required"`
	Args      map[string]any `json:"args"`
	ChannelID string         `json:"channelId"`
	UserID    string         `json:"userId"`
}

// IMCommandResponse is the result of processing an IM command.
type IMCommandResponse struct {
	Result  string `json:"result"`
	Success bool   `json:"success"`
}

// IMSendRequest sends a message to an IM channel.
type IMSendRequest struct {
	Platform    string               `json:"platform" validate:"required"`
	ChannelID   string               `json:"channelId" validate:"required"`
	Text        string               `json:"text" validate:"required"`
	ThreadID    string               `json:"threadId,omitempty"`
	ProjectID   string               `json:"projectId,omitempty"`
	BridgeID    string               `json:"bridgeId,omitempty"`
	DeliveryID  string               `json:"deliveryId,omitempty"`
	Structured  *IMStructuredMessage `json:"structured,omitempty"`
	Native      *IMNativeMessage     `json:"native,omitempty"`
	Attachments []IMAttachmentRef    `json:"attachments,omitempty"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
	ReplyTarget *IMReplyTarget       `json:"replyTarget,omitempty"`
}

// IMAttachmentRef points at a file that the IM Bridge has staged. The Go
// backend persists the reference when it relays an envelope; the bridge
// resolves the actual bytes at delivery time.
type IMAttachmentRef struct {
	ID         string            `json:"id,omitempty"`
	StagedID   string            `json:"staged_id,omitempty"`
	Kind       string            `json:"kind,omitempty"`
	MimeType   string            `json:"mime_type,omitempty"`
	Filename   string            `json:"filename,omitempty"`
	SizeBytes  int64             `json:"size_bytes,omitempty"`
	ContentRef string            `json:"content_ref,omitempty"`
	URL        string            `json:"url,omitempty"`
	DataBase64 string            `json:"data_base64,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// IMReactionRequest is the payload the IM Bridge posts to
// /api/v1/im/reactions when it observes an inbound emoji reaction.
type IMReactionRequest struct {
	Platform    string            `json:"platform" validate:"required"`
	ChatID      string            `json:"chat_id,omitempty"`
	MessageID   string            `json:"message_id,omitempty"`
	UserID      string            `json:"user_id,omitempty"`
	EmojiCode   string            `json:"emoji_code,omitempty"`
	RawEmoji    string            `json:"raw_emoji,omitempty"`
	ReactedAt   time.Time         `json:"reacted_at"`
	Removed     bool              `json:"removed,omitempty"`
	BridgeID    string            `json:"bridge_id,omitempty"`
	ReplyTarget *IMReplyTarget    `json:"reply_target,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// IMReactionShortcutBinding maps a unified emoji code on a reply target to a
// review decision. Registered via POST /api/v1/im/reactions/shortcuts.
type IMReactionShortcutBinding struct {
	ReviewID    string         `json:"review_id" validate:"required"`
	Outcome     string         `json:"outcome" validate:"required,oneof=approve request-changes"`
	EmojiCode   string         `json:"emoji_code" validate:"required"`
	Platform    string         `json:"platform,omitempty"`
	BridgeID    string         `json:"bridge_id,omitempty"`
	ReplyTarget *IMReplyTarget `json:"reply_target,omitempty"`
}

// IMNotifyRequest sends a notification event to an IM channel.
type IMNotifyRequest struct {
	Platform    string               `json:"platform" validate:"required"`
	ChannelID   string               `json:"channelId" validate:"required"`
	Event       string               `json:"event" validate:"required"`
	Title       string               `json:"title"`
	Body        string               `json:"body"`
	Data        any                  `json:"data,omitempty"`
	ProjectID   string               `json:"projectId,omitempty"`
	BridgeID    string               `json:"bridgeId,omitempty"`
	DeliveryID  string               `json:"deliveryId,omitempty"`
	Structured  *IMStructuredMessage `json:"structured,omitempty"`
	Native      *IMNativeMessage     `json:"native,omitempty"`
	Metadata    map[string]string    `json:"metadata,omitempty"`
	ReplyTarget *IMReplyTarget       `json:"replyTarget,omitempty"`
}

// IMActionRequest represents a button click callback from IM.
type IMActionRequest struct {
	Platform    string            `json:"platform" validate:"required"`
	Action      string            `json:"action" validate:"required"`
	EntityID    string            `json:"entityId" validate:"required"`
	ChannelID   string            `json:"channelId" validate:"required"`
	UserID      string            `json:"userId"`
	BridgeID    string            `json:"bridgeId,omitempty"`
	ReplyTarget *IMReplyTarget    `json:"replyTarget,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// IMActionResponse is the result of processing an IM button action.
type IMActionResponse struct {
	Result        string                     `json:"result"`
	Success       bool                       `json:"success"`
	Status        string                     `json:"status,omitempty"`
	Task          *TaskDTO                   `json:"task,omitempty"`
	Dispatch      *DispatchOutcome           `json:"dispatch,omitempty"`
	Decomposition *TaskDecompositionResponse `json:"decomposition,omitempty"`
	Review        *ReviewDTO                 `json:"review,omitempty"`
	ReplyTarget   *IMReplyTarget             `json:"replyTarget,omitempty"`
	Metadata      map[string]string          `json:"metadata,omitempty"`
	Structured    *IMStructuredMessage       `json:"structured,omitempty"`
	Native        *IMNativeMessage           `json:"native,omitempty"`
}

const (
	IMActionStatusStarted   = "started"
	IMActionStatusCompleted = "completed"
	IMActionStatusBlocked   = "blocked"
	IMActionStatusFailed    = "failed"
)

// IMIntentRequest represents a natural language message for intent classification.
type IMIntentRequest struct {
	Text      string `json:"text" validate:"required"`
	UserID    string `json:"user_id"`
	ProjectID string `json:"project_id"`
}

// IMIntentResponse is the result of classifying an IM message's intent.
type IMIntentResponse struct {
	Reply  string `json:"reply"`
	Intent string `json:"intent,omitempty"`
}

// IMReplyTarget preserves enough provider context to deliver asynchronous
// progress or terminal updates back to the original IM conversation.
type IMReplyTarget struct {
	Platform           string            `json:"platform"`
	TenantID           string            `json:"tenantId,omitempty"`
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

type IMStructuredField struct {
	Label string `json:"label,omitempty"`
	Value string `json:"value,omitempty"`
}

type IMStructuredAction struct {
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
	URL   string `json:"url,omitempty"`
	Style string `json:"style,omitempty"`
}

type IMStructuredMessage struct {
	Title   string               `json:"title,omitempty"`
	Body    string               `json:"body,omitempty"`
	Fields  []IMStructuredField  `json:"fields,omitempty"`
	Actions []IMStructuredAction `json:"actions,omitempty"`
}

func (m *IMStructuredMessage) FallbackText() string {
	if m == nil {
		return ""
	}
	lines := make([]string, 0, 2+len(m.Fields)+len(m.Actions))
	if title := strings.TrimSpace(m.Title); title != "" {
		lines = append(lines, title)
	}
	if body := strings.TrimSpace(m.Body); body != "" {
		lines = append(lines, body)
	}
	for _, field := range m.Fields {
		label := strings.TrimSpace(field.Label)
		value := strings.TrimSpace(field.Value)
		switch {
		case label == "" && value == "":
			continue
		case label == "":
			lines = append(lines, value)
		default:
			lines = append(lines, label+": "+value)
		}
	}
	for _, action := range m.Actions {
		label := strings.TrimSpace(action.Label)
		url := strings.TrimSpace(action.URL)
		switch {
		case label != "" && url != "":
			lines = append(lines, label+": "+url)
		case label != "":
			lines = append(lines, label)
		case url != "":
			lines = append(lines, url)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

type IMNativeMessage struct {
	Platform      string                  `json:"platform,omitempty"`
	FeishuCard    *IMFeishuCardPayload    `json:"feishuCard,omitempty"`
	SlackBlockKit *IMSlackBlockKitPayload `json:"slackBlockKit,omitempty"`
	DiscordEmbed  *IMDiscordEmbedPayload  `json:"discordEmbed,omitempty"`
	TelegramRich  *IMTelegramRichPayload  `json:"telegramRich,omitempty"`
	DingTalkCard  *IMDingTalkCardPayload  `json:"dingTalkCard,omitempty"`
	WeComCard     *IMWeComCardPayload     `json:"weComCard,omitempty"`
	QQBotMarkdown *IMQQBotMarkdownPayload `json:"qqBotMarkdown,omitempty"`
}

type IMFeishuCardPayload struct {
	Mode                string          `json:"mode,omitempty"`
	JSON                json.RawMessage `json:"json,omitempty"`
	TemplateID          string          `json:"templateId,omitempty"`
	TemplateVersionName string          `json:"templateVersionName,omitempty"`
	TemplateVariable    map[string]any  `json:"templateVariable,omitempty"`
}

type IMSlackBlockKitPayload struct {
	Blocks json.RawMessage `json:"blocks,omitempty"`
}

type IMDiscordEmbedField struct {
	Name   string `json:"name,omitempty"`
	Value  string `json:"value,omitempty"`
	Inline bool   `json:"inline,omitempty"`
}

type IMDiscordButton struct {
	Label    string `json:"label,omitempty"`
	CustomID string `json:"customId,omitempty"`
	URL      string `json:"url,omitempty"`
	Style    string `json:"style,omitempty"`
}

type IMDiscordActionRow struct {
	Buttons []IMDiscordButton `json:"buttons,omitempty"`
}

type IMDiscordEmbedPayload struct {
	Title       string                `json:"title,omitempty"`
	Description string                `json:"description,omitempty"`
	Fields      []IMDiscordEmbedField `json:"fields,omitempty"`
	Color       int                   `json:"color,omitempty"`
	Components  []IMDiscordActionRow  `json:"components,omitempty"`
}

type IMTelegramInlineButton struct {
	Text         string `json:"text,omitempty"`
	URL          string `json:"url,omitempty"`
	CallbackData string `json:"callbackData,omitempty"`
}

type IMTelegramRichPayload struct {
	Text           string                     `json:"text,omitempty"`
	ParseMode      string                     `json:"parseMode,omitempty"`
	InlineKeyboard [][]IMTelegramInlineButton `json:"inlineKeyboard,omitempty"`
}

type IMDingTalkCardButton struct {
	Title     string `json:"title,omitempty"`
	ActionURL string `json:"actionUrl,omitempty"`
}

type IMDingTalkCardPayload struct {
	CardType string                 `json:"cardType,omitempty"`
	Title    string                 `json:"title,omitempty"`
	Markdown string                 `json:"markdown,omitempty"`
	Buttons  []IMDingTalkCardButton `json:"buttons,omitempty"`
}

type IMWeComArticle struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	PicURL      string `json:"picUrl,omitempty"`
}

type IMWeComTemplateField struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type IMWeComCardPayload struct {
	CardType       string                 `json:"cardType,omitempty"`
	Title          string                 `json:"title,omitempty"`
	Description    string                 `json:"description,omitempty"`
	URL            string                 `json:"url,omitempty"`
	Articles       []IMWeComArticle       `json:"articles,omitempty"`
	TemplateFields []IMWeComTemplateField `json:"templateFields,omitempty"`
}

type IMQQBotKeyboardButton struct {
	Label  string `json:"label,omitempty"`
	URL    string `json:"url,omitempty"`
	Action string `json:"action,omitempty"`
}

type IMQQBotMarkdownPayload struct {
	Markdown string                    `json:"markdown,omitempty"`
	Keyboard [][]IMQQBotKeyboardButton `json:"keyboard,omitempty"`
}

type IMChannel struct {
	ID             string            `json:"id"`
	Platform       string            `json:"platform"`
	Name           string            `json:"name"`
	ChannelID      string            `json:"channelId"`
	WebhookURL     string            `json:"webhookUrl"`
	PlatformConfig map[string]string `json:"platformConfig,omitempty"`
	Events         []string          `json:"events"`
	Active         bool              `json:"active"`
}

type IMBridgeProviderDetail struct {
	Platform          string            `json:"platform"`
	Status            string            `json:"status,omitempty"`
	Transport         string            `json:"transport,omitempty"`
	CallbackPaths     []string          `json:"callbackPaths,omitempty"`
	CapabilityMatrix  map[string]any    `json:"capabilityMatrix,omitempty"`
	PendingDeliveries int               `json:"pendingDeliveries"`
	RecentFailures    int               `json:"recentFailures"`
	RecentDowngrades  int               `json:"recentDowngrades"`
	LastDeliveryAt    *string           `json:"lastDeliveryAt,omitempty"`
	Diagnostics       map[string]string `json:"diagnostics,omitempty"`
}

type IMBridgeStatus struct {
	Registered        bool                     `json:"registered"`
	LastHeartbeat     *string                  `json:"lastHeartbeat"`
	Providers         []string                 `json:"providers"`
	ProviderDetails   []IMBridgeProviderDetail `json:"providerDetails"`
	Health            string                   `json:"health"`
	PendingDeliveries int                      `json:"pendingDeliveries"`
	RecentFailures    int                      `json:"recentFailures"`
	RecentDowngrades  int                      `json:"recentDowngrades"`
	AverageLatencyMs  int64                    `json:"averageLatencyMs"`
}

type IMDeliveryStatus string

const (
	IMDeliveryStatusPending    IMDeliveryStatus = "pending"
	IMDeliveryStatusDelivered  IMDeliveryStatus = "delivered"
	IMDeliveryStatusSuppressed IMDeliveryStatus = "suppressed"
	IMDeliveryStatusFailed     IMDeliveryStatus = "failed"
	IMDeliveryStatusTimeout    IMDeliveryStatus = "timeout"
)

type IMDelivery struct {
	ID              string               `json:"id"`
	BridgeID        string               `json:"bridgeId,omitempty"`
	ProjectID       string               `json:"projectId,omitempty"`
	ChannelID       string               `json:"channelId"`
	TargetChatID    string               `json:"targetChatId,omitempty"`
	Platform        string               `json:"platform"`
	EventType       string               `json:"eventType"`
	Kind            string               `json:"kind,omitempty"`
	Status          IMDeliveryStatus     `json:"status"`
	FailureReason   string               `json:"failureReason,omitempty"`
	DowngradeReason string               `json:"downgradeReason,omitempty"`
	Content         string               `json:"content,omitempty"`
	Structured      *IMStructuredMessage `json:"structured,omitempty"`
	Native          *IMNativeMessage     `json:"native,omitempty"`
	Metadata        map[string]string    `json:"metadata,omitempty"`
	ReplyTarget     *IMReplyTarget       `json:"replyTarget,omitempty"`
	CreatedAt       string               `json:"createdAt"`
	ProcessedAt     string               `json:"processedAt,omitempty"`
	LatencyMs       int64                `json:"latencyMs,omitempty"`
}

// IMBridgeRegisterRequest registers a Bridge runtime instance.
type IMBridgeRegisterRequest struct {
	BridgeID         string            `json:"bridgeId"`
	Platform         string            `json:"platform"`
	Transport        string            `json:"transport"`
	ProjectIDs       []string          `json:"projectIds,omitempty"`
	Capabilities     map[string]bool   `json:"capabilities,omitempty"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string          `json:"callbackPaths,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	// Tenants served by this provider on this bridge. Empty = single-tenant
	// legacy registration. Control plane indexes (BridgeID, Platform, TenantID).
	Tenants []string `json:"tenants,omitempty"`
	// TenantManifest enumerates every tenant hosted on this bridge with its
	// backend projectId so the router can resolve (bridge, provider, tenant)
	// triples without a separate lookup.
	TenantManifest []IMTenantBinding `json:"tenantManifest,omitempty"`
}

// IMTenantBinding maps a tenantID to a backend project.
type IMTenantBinding struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
}

// IMBridgeInstance describes the server-side view of a registered Bridge.
type IMBridgeInstance struct {
	BridgeID         string            `json:"bridgeId"`
	Platform         string            `json:"platform"`
	Transport        string            `json:"transport"`
	ProjectIDs       []string          `json:"projectIds,omitempty"`
	Capabilities     map[string]bool   `json:"capabilities,omitempty"`
	CapabilityMatrix map[string]any    `json:"capabilityMatrix,omitempty"`
	CallbackPaths    []string          `json:"callbackPaths,omitempty"`
	Metadata         map[string]string `json:"metadata,omitempty"`
	Tenants          []string          `json:"tenants,omitempty"`
	TenantManifest   []IMTenantBinding `json:"tenantManifest,omitempty"`
	LastSeenAt       string            `json:"lastSeenAt,omitempty"`
	ExpiresAt        string            `json:"expiresAt,omitempty"`
	Status           string            `json:"status,omitempty"`
}

// IMBridgeHeartbeatResponse reports refreshed liveness.
type IMBridgeHeartbeatResponse struct {
	BridgeID   string `json:"bridgeId"`
	ExpiresAt  string `json:"expiresAt"`
	LastSeenAt string `json:"lastSeenAt"`
	Status     string `json:"status"`
}

// IMControlDelivery is the backend-to-Bridge control-plane payload.
type IMControlDelivery struct {
	Cursor         int64                `json:"cursor"`
	DeliveryID     string               `json:"deliveryId"`
	TargetBridgeID string               `json:"targetBridgeId"`
	Platform       string               `json:"platform"`
	ProjectID      string               `json:"projectId,omitempty"`
	TenantID       string               `json:"tenantId,omitempty"`
	Kind           string               `json:"kind"`
	Content        string               `json:"content"`
	Structured     *IMStructuredMessage `json:"structured,omitempty"`
	Native         *IMNativeMessage     `json:"native,omitempty"`
	Metadata       map[string]string    `json:"metadata,omitempty"`
	TargetChatID   string               `json:"targetChatId,omitempty"`
	ReplyTarget    *IMReplyTarget       `json:"replyTarget,omitempty"`
	Timestamp      string               `json:"timestamp"`
	Signature      string               `json:"signature,omitempty"`
}

// IMDeliveryAck acknowledges the last successfully processed delivery cursor.
type IMDeliveryAck struct {
	BridgeID        string `json:"bridgeId"`
	Cursor          int64  `json:"cursor"`
	DeliveryID      string `json:"deliveryId,omitempty"`
	Status          string `json:"status,omitempty"`
	FailureReason   string `json:"failureReason,omitempty"`
	DowngradeReason string `json:"downgradeReason,omitempty"`
	ProcessedAt     string `json:"processedAt,omitempty"`
}

type IMRetryBatchRequest struct {
	DeliveryIDs []string `json:"deliveryIds"`
}

type IMDeliveryHistoryFilters struct {
	DeliveryID string `json:"deliveryId,omitempty"`
	Status     string `json:"status,omitempty"`
	Platform   string `json:"platform,omitempty"`
	EventType  string `json:"eventType,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Since      string `json:"since,omitempty"`
}

type IMRetryBatchItemResult struct {
	DeliveryID string           `json:"deliveryId"`
	Status     IMDeliveryStatus `json:"status"`
	Message    string           `json:"message,omitempty"`
}

type IMRetryBatchResponse struct {
	Results []IMRetryBatchItemResult `json:"results"`
}

type IMTestSendRequest struct {
	DeliveryID string            `json:"deliveryId,omitempty"`
	Platform   string            `json:"platform"`
	ChannelID  string            `json:"channelId"`
	ProjectID  string            `json:"projectId,omitempty"`
	BridgeID   string            `json:"bridgeId,omitempty"`
	Text       string            `json:"text"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

type IMTestSendResponse struct {
	DeliveryID      string           `json:"deliveryId"`
	Status          IMDeliveryStatus `json:"status"`
	FailureReason   string           `json:"failureReason,omitempty"`
	DowngradeReason string           `json:"downgradeReason,omitempty"`
	ProcessedAt     string           `json:"processedAt,omitempty"`
	LatencyMs       int64            `json:"latencyMs,omitempty"`
}

// IMActionBinding persists the link between a backend entity and the
// originating IM reply target for asynchronous updates.
type IMActionBinding struct {
	BridgeID    string         `json:"bridgeId"`
	Platform    string         `json:"platform"`
	ProjectID   string         `json:"projectId,omitempty"`
	TaskID      string         `json:"taskId,omitempty"`
	RunID       string         `json:"runId,omitempty"`
	ReviewID    string         `json:"reviewId,omitempty"`
	ReplyTarget *IMReplyTarget `json:"replyTarget,omitempty"`
}
