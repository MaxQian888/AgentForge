package service

import (
	"strings"

	"github.com/agentforge/server/internal/model"
)

const (
	imMetadataDeliverySource               = "delivery_source"
	imMetadataBridgeBindingBridgeID        = "bridge_binding_bridge_id"
	imMetadataBridgeBindingPlatform        = "bridge_binding_platform"
	imMetadataBridgeBindingProject         = "bridge_binding_project_id"
	imMetadataBridgeBindingTaskID          = "bridge_binding_task_id"
	imMetadataBridgeBindingRunID           = "bridge_binding_run_id"
	imMetadataBridgeBindingReviewID        = "bridge_binding_review_id"
	imMetadataReplyTargetPlatform          = "reply_target_platform"
	imMetadataReplyTargetChatID            = "reply_target_chat_id"
	imMetadataReplyTargetChannelID         = "reply_target_channel_id"
	imMetadataReplyTargetThreadID          = "reply_target_thread_id"
	imMetadataReplyTargetMessageID         = "reply_target_message_id"
	imMetadataReplyTargetUserID            = "reply_target_user_id"
	imMetadataReplyTargetResponseURL       = "reply_target_response_url"
	imMetadataReplyTargetSessionWebhook    = "reply_target_session_webhook"
	imMetadataReplyTargetConversationID    = "reply_target_conversation_id"
	imMetadataReplyTargetUseReply          = "reply_target_use_reply"
	imMetadataReplyTargetPreferEdit        = "reply_target_prefer_edit"
	imMetadataReplyTargetPreferredRenderer = "reply_target_preferred_renderer"
	imMetadataReplyTargetProgressMode      = "reply_target_progress_mode"
)

const (
	imDeliverySourceCompatSend    = "compat_send"
	imDeliverySourceCompatNotify  = "compat_notify"
	imDeliverySourceBoundProgress = "bound_progress"
	imDeliverySourceBoundTerminal = "bound_terminal"
	imDeliverySourceActionResult  = "im_action_result"
)

func buildIMConnectivityMetadata(
	metadata map[string]string,
	source string,
	bridgeID string,
	platform string,
	projectID string,
	taskID string,
	runID string,
	reviewID string,
	replyTarget *model.IMReplyTarget,
) map[string]string {
	cloned := cloneStringMap(metadata)
	if cloned == nil {
		cloned = map[string]string{}
	}
	if trimmed := strings.TrimSpace(source); trimmed != "" {
		cloned[imMetadataDeliverySource] = trimmed
	}
	if trimmed := strings.TrimSpace(bridgeID); trimmed != "" {
		cloned[imMetadataBridgeBindingBridgeID] = trimmed
	}
	if trimmed := normalizePlatform(platform); trimmed != "" {
		cloned[imMetadataBridgeBindingPlatform] = trimmed
	}
	if trimmed := strings.TrimSpace(projectID); trimmed != "" {
		cloned[imMetadataBridgeBindingProject] = trimmed
	}
	if trimmed := strings.TrimSpace(taskID); trimmed != "" {
		cloned[imMetadataBridgeBindingTaskID] = trimmed
	}
	if trimmed := strings.TrimSpace(runID); trimmed != "" {
		cloned[imMetadataBridgeBindingRunID] = trimmed
	}
	if trimmed := strings.TrimSpace(reviewID); trimmed != "" {
		cloned[imMetadataBridgeBindingReviewID] = trimmed
	}
	if replyTarget == nil {
		return cloned
	}
	if trimmed := normalizePlatform(replyTarget.Platform); trimmed != "" {
		cloned[imMetadataReplyTargetPlatform] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.ChatID); trimmed != "" {
		cloned[imMetadataReplyTargetChatID] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.ChannelID); trimmed != "" {
		cloned[imMetadataReplyTargetChannelID] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.ThreadID); trimmed != "" {
		cloned[imMetadataReplyTargetThreadID] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.MessageID); trimmed != "" {
		cloned[imMetadataReplyTargetMessageID] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.UserID); trimmed != "" {
		cloned[imMetadataReplyTargetUserID] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.ResponseURL); trimmed != "" {
		cloned[imMetadataReplyTargetResponseURL] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.SessionWebhook); trimmed != "" {
		cloned[imMetadataReplyTargetSessionWebhook] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.ConversationID); trimmed != "" {
		cloned[imMetadataReplyTargetConversationID] = trimmed
	}
	if replyTarget.UseReply {
		cloned[imMetadataReplyTargetUseReply] = "true"
	}
	if replyTarget.PreferEdit {
		cloned[imMetadataReplyTargetPreferEdit] = "true"
	}
	if trimmed := strings.TrimSpace(replyTarget.PreferredRenderer); trimmed != "" {
		cloned[imMetadataReplyTargetPreferredRenderer] = trimmed
	}
	if trimmed := strings.TrimSpace(replyTarget.ProgressMode); trimmed != "" {
		cloned[imMetadataReplyTargetProgressMode] = trimmed
	}
	return cloned
}

func buildIMActionResponseMetadata(req *model.IMActionRequest, status string) map[string]string {
	if req == nil {
		return buildIMConnectivityMetadata(nil, imDeliverySourceActionResult, "", "", "", "", "", "", nil)
	}
	metadata := buildIMConnectivityMetadata(
		req.Metadata,
		imDeliverySourceActionResult,
		req.BridgeID,
		req.Platform,
		"",
		"",
		"",
		"",
		req.ReplyTarget,
	)
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		metadata["action_status"] = trimmed
	}
	return metadata
}
