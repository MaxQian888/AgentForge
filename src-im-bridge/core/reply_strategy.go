package core

import (
	"context"
	"errors"
	"strings"
)

// DeliveryMethod describes the native path chosen for a delivery.
type DeliveryMethod string

const (
	DeliveryMethodSend               DeliveryMethod = "send"
	DeliveryMethodReply              DeliveryMethod = "reply"
	DeliveryMethodThreadReply        DeliveryMethod = "thread_reply"
	DeliveryMethodEdit               DeliveryMethod = "edit"
	DeliveryMethodFollowUp           DeliveryMethod = "follow_up"
	DeliveryMethodSessionWebhook     DeliveryMethod = "session_webhook"
	DeliveryMethodDeferredCardUpdate DeliveryMethod = "deferred_card_update"
)

// ReplyPlan captures the preferred native delivery path for a payload.
type ReplyPlan struct {
	Method          DeliveryMethod
	TargetChatID    string
	UsedReplyTarget bool
}

// ResolveReplyPlan determines the preferred delivery path for a payload based
// on the platform capability matrix and the preserved reply target.
func ResolveReplyPlan(metadata PlatformMetadata, target *ReplyTarget, fallbackChatID string) ReplyPlan {
	plan := ReplyPlan{
		Method:       DeliveryMethodSend,
		TargetChatID: strings.TrimSpace(fallbackChatID),
	}
	if target == nil {
		return plan
	}

	plan.UsedReplyTarget = true
	if plan.TargetChatID == "" {
		plan.TargetChatID = firstNonEmpty(target.ChatID, target.ChannelID, target.ConversationID)
	}
	progressMode := strings.TrimSpace(target.ProgressMode)
	switch {
	case target.SessionWebhook != "" && metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateSessionWebhook):
		plan.Method = DeliveryMethodSessionWebhook
	case progressMode == string(AsyncUpdateDeferredCardUpdate) && target.CallbackToken != "" && metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateDeferredCardUpdate):
		plan.Method = DeliveryMethodDeferredCardUpdate
	case target.PreferEdit && metadata.Capabilities.Mutability.CanEdit && strings.TrimSpace(target.MessageID) != "":
		plan.Method = DeliveryMethodEdit
	case progressMode == string(AsyncUpdateFollowUp) && target.InteractionToken != "" && metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateFollowUp):
		plan.Method = DeliveryMethodFollowUp
	case target.InteractionToken != "" && metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateFollowUp):
		plan.Method = DeliveryMethodFollowUp
	case target.ThreadID != "" && metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateThreadReply):
		plan.Method = DeliveryMethodThreadReply
	case target.UseReply:
		plan.Method = DeliveryMethodReply
	}
	return plan
}

// DeliverText routes a text payload through the shared reply strategy before
// falling back to a plain send.
func DeliverText(ctx context.Context, platform Platform, metadata PlatformMetadata, target *ReplyTarget, fallbackChatID, content string) (ReplyPlan, error) {
	plan := ResolveReplyPlan(metadata, target, fallbackChatID)
	if plan.Method == DeliveryMethodSend {
		if plan.TargetChatID == "" {
			return plan, errors.New("delivery missing target chat id")
		}
		return plan, platform.Send(ctx, plan.TargetChatID, content)
	}

	replyCtx := restoreReplyContext(platform, target)
	switch plan.Method {
	case DeliveryMethodEdit:
		if updater, ok := platform.(MessageUpdater); ok && replyCtx != nil {
			return plan, updater.UpdateMessage(ctx, replyCtx, content)
		}
	case DeliveryMethodReply, DeliveryMethodThreadReply, DeliveryMethodFollowUp, DeliveryMethodSessionWebhook, DeliveryMethodDeferredCardUpdate:
		if replyCtx != nil {
			return plan, platform.Reply(ctx, replyCtx, content)
		}
	}

	if plan.TargetChatID == "" {
		return plan, errors.New("delivery missing target chat id")
	}
	plan.Method = DeliveryMethodSend
	return plan, platform.Send(ctx, plan.TargetChatID, content)
}

// DeliverCard routes a legacy card payload through the shared reply strategy.
func DeliverCard(ctx context.Context, platform Platform, metadata PlatformMetadata, target *ReplyTarget, fallbackChatID string, card *Card) (ReplyPlan, error) {
	sender, ok := platform.(CardSender)
	if !ok {
		return ReplyPlan{}, errors.New("platform does not support card delivery")
	}

	plan := ResolveReplyPlan(metadata, target, fallbackChatID)
	if plan.Method == DeliveryMethodSend {
		if plan.TargetChatID == "" {
			return plan, errors.New("delivery missing target chat id")
		}
		return plan, sender.SendCard(ctx, plan.TargetChatID, card)
	}

	replyCtx := restoreReplyContext(platform, target)
	if replyCtx != nil {
		return plan, sender.ReplyCard(ctx, replyCtx, card)
	}
	if plan.TargetChatID == "" {
		return plan, errors.New("delivery missing target chat id")
	}
	plan.Method = DeliveryMethodSend
	return plan, sender.SendCard(ctx, plan.TargetChatID, card)
}

func restoreReplyContext(platform Platform, target *ReplyTarget) any {
	if target == nil {
		return nil
	}
	resolver, ok := platform.(ReplyTargetResolver)
	if !ok {
		return nil
	}
	return resolver.ReplyContextFromTarget(target)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
