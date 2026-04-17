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
	DeliveryMethodOpenThread         DeliveryMethod = "open_thread"
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
	FallbackReason  string
}

// FallbackDeliveryError reports that a platform-specific richer delivery path
// already downgraded and delivered a fallback payload.
type FallbackDeliveryError interface {
	error
	FallbackReason() string
	FallbackDelivered() bool
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
	threadPolicy := target.ThreadPolicy
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
	case threadPolicy == ThreadPolicyOpen && metadata.Capabilities.SupportsThreads && metadata.Capabilities.HasThreadPolicy(ThreadPolicyOpen):
		plan.Method = DeliveryMethodOpenThread
	case threadPolicy == ThreadPolicyIsolate:
		plan.Method = DeliveryMethodSend
	case (threadPolicy == "" || threadPolicy == ThreadPolicyReuse) && target.ThreadID != "" && metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateThreadReply):
		plan.Method = DeliveryMethodThreadReply
	case target.ThreadID != "" && metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateThreadReply):
		plan.Method = DeliveryMethodThreadReply
	case target.UseReply:
		plan.Method = DeliveryMethodReply
	}
	plan.FallbackReason = firstNonEmpty(plan.FallbackReason, requestedReplyModeFallback(metadata, target, plan), threadPolicyFallback(metadata, target, plan))
	return plan
}

// threadPolicyFallback reports a fallback reason when the requested thread
// policy cannot be honored by the platform.
func threadPolicyFallback(metadata PlatformMetadata, target *ReplyTarget, plan ReplyPlan) string {
	if target == nil {
		return ""
	}
	policy := target.ThreadPolicy
	if policy == "" {
		return ""
	}
	if metadata.Capabilities.SupportsThreads && metadata.Capabilities.HasThreadPolicy(policy) {
		return ""
	}
	return "thread_" + string(policy) + "_unsupported"
}

// DeliverText routes a text payload through the shared reply strategy before
// falling back to a plain send. The content is sanitized according to the
// process-wide SanitizeMode before any provider call so broadcast mentions,
// zero-width characters, control bytes, and over-limit text are normalized
// consistently across all providers.
func DeliverText(ctx context.Context, platform Platform, metadata PlatformMetadata, target *ReplyTarget, fallbackChatID, content string) (ReplyPlan, error) {
	plan := ResolveReplyPlan(metadata, target, fallbackChatID)

	sanitized := SanitizeEgress(metadata.Rendering, DefaultSanitizeMode(), content)
	if len(sanitized.Warnings) > 0 {
		plan.FallbackReason = appendSanitizeReason(plan.FallbackReason, sanitized.Warnings)
	}

	// Segmented delivery: only the send path (new message in a chat) can
	// be split into multiple messages. Reply/edit paths collapse segments
	// into a single joined message to preserve reply-target semantics.
	if len(sanitized.Segments) > 1 && plan.Method == DeliveryMethodSend {
		if plan.TargetChatID == "" {
			return plan, errors.New("delivery missing target chat id")
		}
		for _, seg := range sanitized.Segments {
			if err := platform.Send(ctx, plan.TargetChatID, seg); err != nil {
				return plan, err
			}
		}
		return plan, nil
	}
	content = sanitized.Text
	if len(sanitized.Segments) > 0 {
		content = strings.Join(sanitized.Segments, "")
	}

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
	case DeliveryMethodOpenThread:
		if opener, ok := platform.(ThreadOpener); ok && replyCtx != nil {
			if _, err := opener.OpenThread(ctx, replyCtx, deriveThreadTitle(target)); err == nil {
				return plan, platform.Reply(ctx, replyCtx, content)
			}
		}
		if replyCtx != nil {
			plan.Method = DeliveryMethodReply
			return plan, platform.Reply(ctx, replyCtx, content)
		}
	case DeliveryMethodReply, DeliveryMethodThreadReply, DeliveryMethodFollowUp, DeliveryMethodSessionWebhook, DeliveryMethodDeferredCardUpdate:
		if replyCtx != nil {
			if target != nil && target.ThreadPolicy == ThreadPolicyIsolate {
				content = applyIsolatePrefix(target, content)
			}
			return plan, platform.Reply(ctx, replyCtx, content)
		}
	}

	if plan.TargetChatID == "" {
		return plan, errors.New("delivery missing target chat id")
	}
	plan.Method = DeliveryMethodSend
	if target != nil && target.ThreadPolicy == ThreadPolicyIsolate {
		content = applyIsolatePrefix(target, content)
	}
	return plan, platform.Send(ctx, plan.TargetChatID, content)
}

// applyIsolatePrefix adds a `[session: short-id]` prefix so the recipient can
// visually correlate the isolated message with an active session without
// provider-native thread UX.
func applyIsolatePrefix(target *ReplyTarget, content string) string {
	if target == nil {
		return content
	}
	id := firstNonEmpty(target.ThreadParentID, target.ThreadID, target.MessageID, target.InteractionToken)
	if id == "" {
		return content
	}
	short := id
	if len(short) > 12 {
		short = short[:12]
	}
	prefix := "[session: " + short + "] "
	if strings.HasPrefix(content, prefix) {
		return content
	}
	return prefix + content
}

// deriveThreadTitle picks a reasonable title for a newly opened thread from
// the reply target metadata.
func deriveThreadTitle(target *ReplyTarget) string {
	if target == nil {
		return "AgentForge session"
	}
	if t := strings.TrimSpace(target.Metadata["thread_title"]); t != "" {
		return t
	}
	return "AgentForge session"
}

// appendSanitizeReason augments an existing fallback reason string with
// pipe-separated sanitize warnings. Multiple concurrent sanitize warnings
// are visible to operators in audit + delivery receipt metadata.
func appendSanitizeReason(existing string, warnings []SanitizeWarning) string {
	var sb strings.Builder
	sb.WriteString(existing)
	for _, w := range warnings {
		if sb.Len() > 0 {
			sb.WriteString("|")
		}
		sb.WriteString(string(w))
	}
	return sb.String()
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
		if err := sender.SendCard(ctx, plan.TargetChatID, card); err != nil {
			if fallbackErr, ok := err.(FallbackDeliveryError); ok && fallbackErr.FallbackDelivered() {
				plan.FallbackReason = fallbackErr.FallbackReason()
				return plan, nil
			}
			return plan, err
		}
		return plan, nil
	}

	replyCtx := restoreReplyContext(platform, target)
	if replyCtx != nil {
		if err := sender.ReplyCard(ctx, replyCtx, card); err != nil {
			if fallbackErr, ok := err.(FallbackDeliveryError); ok && fallbackErr.FallbackDelivered() {
				plan.FallbackReason = fallbackErr.FallbackReason()
				return plan, nil
			}
			return plan, err
		}
		return plan, nil
	}
	if plan.TargetChatID == "" {
		return plan, errors.New("delivery missing target chat id")
	}
	plan.Method = DeliveryMethodSend
	if err := sender.SendCard(ctx, plan.TargetChatID, card); err != nil {
		if fallbackErr, ok := err.(FallbackDeliveryError); ok && fallbackErr.FallbackDelivered() {
			plan.FallbackReason = fallbackErr.FallbackReason()
			return plan, nil
		}
		return plan, err
	}
	return plan, nil
}

// DeliverNative routes a provider-native payload through the shared reply
// strategy, preferring a native update path when the reply target indicates an
// in-place mutation flow.
func DeliverNative(ctx context.Context, platform Platform, metadata PlatformMetadata, target *ReplyTarget, fallbackChatID string, message *NativeMessage) (ReplyPlan, error) {
	sender, ok := platform.(NativeMessageSender)
	if !ok {
		return ReplyPlan{}, errors.New("platform does not support native delivery")
	}
	plan, err := DeliverNativePlan(metadata, target, fallbackChatID, message)
	if err != nil {
		return ReplyPlan{}, err
	}
	if plan.Method == DeliveryMethodSend {
		if plan.TargetChatID == "" {
			return plan, errors.New("delivery missing target chat id")
		}
		return plan, sender.SendNative(ctx, plan.TargetChatID, message)
	}

	replyCtx := restoreReplyContext(platform, target)
	switch plan.Method {
	case DeliveryMethodDeferredCardUpdate, DeliveryMethodEdit:
		if updater, ok := platform.(NativeMessageUpdater); ok && replyCtx != nil {
			if err := updater.UpdateNative(ctx, replyCtx, message); err == nil {
				return plan, nil
			} else {
				plan.FallbackReason = err.Error()
			}
		} else {
			plan.FallbackReason = nativeFallbackReason(plan.Method, target, replyCtx, ok)
		}
	}
	if replyCtx != nil {
		if plan.Method == DeliveryMethodDeferredCardUpdate || plan.Method == DeliveryMethodEdit {
			plan.Method = DeliveryMethodReply
		}
		return plan, sender.ReplyNative(ctx, replyCtx, message)
	}
	if plan.TargetChatID == "" {
		return plan, errors.New("delivery missing target chat id")
	}
	plan.Method = DeliveryMethodSend
	return plan, sender.SendNative(ctx, plan.TargetChatID, message)
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

func nativeFallbackReason(method DeliveryMethod, target *ReplyTarget, replyCtx any, hasUpdater bool) string {
	switch method {
	case DeliveryMethodDeferredCardUpdate:
		switch {
		case target == nil || strings.TrimSpace(target.CallbackToken) == "":
			return "deferred card update unavailable: missing callback token"
		case !hasUpdater:
			return "deferred card update unavailable: native updater not implemented"
		case replyCtx == nil:
			return "deferred card update unavailable: reply context could not be restored"
		default:
			return "deferred card update unavailable"
		}
	case DeliveryMethodEdit:
		switch {
		case !hasUpdater:
			return "native message edit unavailable: native updater not implemented"
		case replyCtx == nil:
			return "native message edit unavailable: reply context could not be restored"
		default:
			return "native message edit unavailable"
		}
	default:
		return ""
	}
}

func requestedReplyModeFallback(metadata PlatformMetadata, target *ReplyTarget, plan ReplyPlan) string {
	if target == nil {
		return ""
	}

	if target.PreferEdit && plan.Method != DeliveryMethodEdit {
		switch {
		case strings.TrimSpace(target.MessageID) == "":
			return "native message edit unavailable: missing message id"
		case !metadata.Capabilities.Mutability.CanEdit:
			return "native message edit unavailable: platform does not support editable updates"
		}
	}

	progressMode := strings.TrimSpace(target.ProgressMode)
	if progressMode == string(AsyncUpdateDeferredCardUpdate) && plan.Method != DeliveryMethodDeferredCardUpdate {
		switch {
		case strings.TrimSpace(target.CallbackToken) == "":
			return nativeFallbackReason(DeliveryMethodDeferredCardUpdate, target, nil, false)
		case !metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateDeferredCardUpdate):
			return "deferred card update unavailable: platform does not support deferred card updates"
		}
	}

	if progressMode == string(AsyncUpdateFollowUp) && target.InteractionToken != "" && plan.Method != DeliveryMethodFollowUp && !metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateFollowUp) {
		return "follow-up delivery unavailable: platform does not support follow-up updates"
	}

	if target.SessionWebhook != "" && plan.Method != DeliveryMethodSessionWebhook && !metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateSessionWebhook) {
		return "session webhook delivery unavailable: platform does not support session webhook replies"
	}

	return ""
}

func DeliverNativePlan(metadata PlatformMetadata, target *ReplyTarget, fallbackChatID string, message *NativeMessage) (ReplyPlan, error) {
	if err := message.Validate(); err != nil {
		return ReplyPlan{}, err
	}

	plan := ResolveReplyPlan(metadata, target, fallbackChatID)
	if target != nil &&
		strings.TrimSpace(target.ProgressMode) == string(AsyncUpdateDeferredCardUpdate) &&
		metadata.Capabilities.HasAsyncUpdateMode(AsyncUpdateDeferredCardUpdate) &&
		plan.Method != DeliveryMethodDeferredCardUpdate &&
		plan.FallbackReason == "" {
		if strings.TrimSpace(target.CallbackToken) == "" {
			plan.FallbackReason = nativeFallbackReason(DeliveryMethodDeferredCardUpdate, target, nil, false)
		}
	}
	return plan, nil
}
