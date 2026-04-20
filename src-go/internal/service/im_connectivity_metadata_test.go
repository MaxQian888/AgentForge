package service

import (
	"testing"

	"github.com/agentforge/server/internal/model"
)

func TestBuildIMConnectivityMetadata_PreservesReplyTargetCompletionHints(t *testing.T) {
	metadata := buildIMConnectivityMetadata(
		map[string]string{"existing": "value"},
		imDeliverySourceBoundTerminal,
		"bridge-1",
		"wecom",
		"project-1",
		"task-1",
		"run-1",
		"review-1",
		&model.IMReplyTarget{
			Platform:          "wecom",
			ChatID:            "chat-1",
			ChannelID:         "channel-1",
			ThreadID:          "thread-1",
			MessageID:         "msg-1",
			UserID:            "zhangsan",
			ResponseURL:       "https://work.weixin.qq.com/response",
			SessionWebhook:    "https://session.example/reply",
			ConversationID:    "conversation-1",
			UseReply:          true,
			PreferEdit:        true,
			PreferredRenderer: "wecom_markdown",
			ProgressMode:      "session_webhook",
		},
	)

	if metadata["existing"] != "value" {
		t.Fatalf("existing metadata = %+v", metadata)
	}
	if metadata[imMetadataReplyTargetPlatform] != "wecom" {
		t.Fatalf("reply target platform metadata = %+v", metadata)
	}
	if metadata["reply_target_user_id"] != "zhangsan" {
		t.Fatalf("reply target user metadata = %+v", metadata)
	}
	if metadata["reply_target_response_url"] != "https://work.weixin.qq.com/response" {
		t.Fatalf("reply target response url metadata = %+v", metadata)
	}
	if metadata["reply_target_session_webhook"] != "https://session.example/reply" {
		t.Fatalf("reply target session webhook metadata = %+v", metadata)
	}
	if metadata["reply_target_conversation_id"] != "conversation-1" {
		t.Fatalf("reply target conversation metadata = %+v", metadata)
	}
	if metadata["reply_target_use_reply"] != "true" {
		t.Fatalf("reply target use-reply metadata = %+v", metadata)
	}
	if metadata["reply_target_prefer_edit"] != "true" {
		t.Fatalf("reply target prefer-edit metadata = %+v", metadata)
	}
	if metadata["reply_target_preferred_renderer"] != "wecom_markdown" {
		t.Fatalf("reply target renderer metadata = %+v", metadata)
	}
	if metadata["reply_target_progress_mode"] != "session_webhook" {
		t.Fatalf("reply target progress-mode metadata = %+v", metadata)
	}
}
