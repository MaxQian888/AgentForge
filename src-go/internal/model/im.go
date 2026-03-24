package model

// IMMessageRequest represents an incoming IM message webhook.
type IMMessageRequest struct {
	Platform  string `json:"platform" validate:"required,oneof=feishu dingtalk slack telegram discord wecom"`
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
	Platform  string `json:"platform" validate:"required"`
	ChannelID string `json:"channelId" validate:"required"`
	Text      string `json:"text" validate:"required"`
	ThreadID  string `json:"threadId,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
	BridgeID  string `json:"bridgeId,omitempty"`
	DeliveryID string `json:"deliveryId,omitempty"`
	ReplyTarget *IMReplyTarget `json:"replyTarget,omitempty"`
}

// IMNotifyRequest sends a notification event to an IM channel.
type IMNotifyRequest struct {
	Platform  string `json:"platform" validate:"required"`
	ChannelID string `json:"channelId" validate:"required"`
	Event     string `json:"event" validate:"required"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Data      any    `json:"data,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
	BridgeID  string `json:"bridgeId,omitempty"`
	DeliveryID string `json:"deliveryId,omitempty"`
	ReplyTarget *IMReplyTarget `json:"replyTarget,omitempty"`
}

// IMActionRequest represents a button click callback from IM.
type IMActionRequest struct {
	Platform  string `json:"platform" validate:"required"`
	Action    string `json:"action" validate:"required"`
	EntityID  string `json:"entityId" validate:"required"`
	ChannelID string `json:"channelId" validate:"required"`
	UserID    string `json:"userId"`
}

// IMActionResponse is the result of processing an IM button action.
type IMActionResponse struct {
	Result  string `json:"result"`
	Success bool   `json:"success"`
}

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

// IMBridgeRegisterRequest registers a Bridge runtime instance.
type IMBridgeRegisterRequest struct {
	BridgeID      string            `json:"bridgeId"`
	Platform      string            `json:"platform"`
	Transport     string            `json:"transport"`
	ProjectIDs    []string          `json:"projectIds,omitempty"`
	Capabilities  map[string]bool   `json:"capabilities,omitempty"`
	CallbackPaths []string          `json:"callbackPaths,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// IMBridgeInstance describes the server-side view of a registered Bridge.
type IMBridgeInstance struct {
	BridgeID      string            `json:"bridgeId"`
	Platform      string            `json:"platform"`
	Transport     string            `json:"transport"`
	ProjectIDs    []string          `json:"projectIds,omitempty"`
	Capabilities  map[string]bool   `json:"capabilities,omitempty"`
	CallbackPaths []string          `json:"callbackPaths,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	LastSeenAt    string            `json:"lastSeenAt,omitempty"`
	ExpiresAt     string            `json:"expiresAt,omitempty"`
	Status        string            `json:"status,omitempty"`
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
	Cursor         int64          `json:"cursor"`
	DeliveryID     string         `json:"deliveryId"`
	TargetBridgeID string         `json:"targetBridgeId"`
	Platform       string         `json:"platform"`
	ProjectID      string         `json:"projectId,omitempty"`
	Kind           string         `json:"kind"`
	Content        string         `json:"content"`
	TargetChatID   string         `json:"targetChatId,omitempty"`
	ReplyTarget    *IMReplyTarget `json:"replyTarget,omitempty"`
	Timestamp      string         `json:"timestamp"`
	Signature      string         `json:"signature,omitempty"`
}

// IMDeliveryAck acknowledges the last successfully processed delivery cursor.
type IMDeliveryAck struct {
	BridgeID    string `json:"bridgeId"`
	Cursor      int64  `json:"cursor"`
	DeliveryID  string `json:"deliveryId,omitempty"`
	ProcessedAt string `json:"processedAt,omitempty"`
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
