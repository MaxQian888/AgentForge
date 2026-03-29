package core

import (
	"context"
	"errors"
	"strings"
)

type DeliveryEnvelope struct {
	Content     string
	Structured  *StructuredMessage
	Native      *NativeMessage
	ReplyTarget *ReplyTarget
	Metadata    map[string]string
}

type DeliveryReceipt struct {
	Type           string
	Method         DeliveryMethod
	FallbackReason string
}

func ResolveRenderingPlan(metadata PlatformMetadata, fallbackChatID string, delivery *DeliveryEnvelope) (RenderingPlan, error) {
	if delivery == nil {
		return RenderingPlan{}, errors.New("delivery is required")
	}
	plan := RenderingPlan{}

	if delivery.Native != nil {
		replyPlan, err := DeliverNativePlan(metadata, delivery.ReplyTarget, fallbackChatID, delivery.Native)
		if err != nil {
			return RenderingPlan{}, err
		}
		plan.Type = "native"
		plan.Method = replyPlan.Method
		plan.Native = delivery.Native
		plan.FallbackReason = firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), metadataValue(delivery.Metadata, "fallback_reason"))
		return plan, nil
	}

	if delivery.Structured != nil {
		replyPlan := ResolveReplyPlan(metadata, delivery.ReplyTarget, fallbackChatID)
		renderer := SelectStructuredRenderer(metadata, delivery.Structured)
		if renderer != StructuredSurfaceNone {
			plan.Type = "structured"
			plan.Method = replyPlan.Method
			plan.Structured = delivery.Structured
			plan.FallbackReason = strings.TrimSpace(metadataValue(delivery.Metadata, "fallback_reason"))
			return plan, nil
		}
		plan.Type = "text"
		plan.Method = replyPlan.Method
		plan.Text = []RenderedText{{
			Content: strings.TrimSpace(delivery.Structured.FallbackText()),
			Format:  resolveTextFormat(metadata, delivery.Metadata),
		}}
		plan.FallbackReason = firstNonEmpty(strings.TrimSpace(metadataValue(delivery.Metadata, "fallback_reason")), inferStructuredFallbackReason(delivery.ReplyTarget))
		return plan, nil
	}

	replyPlan := ResolveReplyPlan(metadata, delivery.ReplyTarget, fallbackChatID)
	plan.Type = "text"
	plan.Method = replyPlan.Method
	plan.Text = []RenderedText{{
		Content: strings.TrimSpace(delivery.Content),
		Format:  resolveTextFormat(metadata, delivery.Metadata),
	}}
	plan.FallbackReason = strings.TrimSpace(metadataValue(delivery.Metadata, "fallback_reason"))
	return plan, nil
}

func DeliverEnvelope(ctx context.Context, platform Platform, metadata PlatformMetadata, fallbackChatID string, delivery *DeliveryEnvelope) (DeliveryReceipt, error) {
	if platform == nil {
		return DeliveryReceipt{}, errors.New("platform is required")
	}
	if delivery == nil {
		return DeliveryReceipt{}, errors.New("delivery is required")
	}
	renderingPlan, err := ResolveRenderingPlan(metadata, fallbackChatID, delivery)
	if err != nil {
		return DeliveryReceipt{}, err
	}

	return executeRenderingPlan(ctx, platform, metadata, fallbackChatID, delivery.ReplyTarget, renderingPlan)
}

func executeRenderingPlan(ctx context.Context, platform Platform, metadata PlatformMetadata, fallbackChatID string, target *ReplyTarget, plan RenderingPlan) (DeliveryReceipt, error) {
	switch plan.Type {
	case "native":
		replyPlan, err := DeliverNative(ctx, platform, metadata, target, fallbackChatID, plan.Native)
		return DeliveryReceipt{Type: "native", Method: replyPlan.Method, FallbackReason: firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), strings.TrimSpace(plan.FallbackReason))}, err
	case "structured":
		if nativeMessage, ok := synthesizeProviderNativeTextMessage(platform, metadata, target, plan.Structured.FallbackText()); ok {
			replyPlan, err := DeliverNative(ctx, platform, metadata, target, fallbackChatID, nativeMessage)
			return DeliveryReceipt{Type: "native", Method: replyPlan.Method, FallbackReason: firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), strings.TrimSpace(plan.FallbackReason))}, err
		}
		if sender, ok := platform.(StructuredSender); ok && target == nil {
			if chatID := strings.TrimSpace(fallbackChatID); chatID != "" {
				if err := sender.SendStructured(ctx, chatID, plan.Structured); err == nil {
					return DeliveryReceipt{Type: "structured", Method: DeliveryMethodSend, FallbackReason: strings.TrimSpace(plan.FallbackReason)}, nil
				}
			}
		}
		if sender, ok := platform.(CardSender); ok && sender != nil {
			renderer := SelectStructuredRenderer(metadata, plan.Structured)
			if renderer == StructuredSurfaceCards || renderer == StructuredSurfaceActionCard {
				replyPlan, err := DeliverCard(ctx, platform, metadata, target, fallbackChatID, plan.Structured.LegacyCard())
				return DeliveryReceipt{Type: "structured", Method: replyPlan.Method, FallbackReason: firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), strings.TrimSpace(plan.FallbackReason))}, err
			}
		}
		text := ""
		if len(plan.Text) > 0 {
			text = plan.Text[0].Content
		} else if plan.Structured != nil {
			text = plan.Structured.FallbackText()
		}
		fallbackReason := strings.TrimSpace(plan.FallbackReason)
		if fallbackReason == "" {
			fallbackReason = inferStructuredFallbackReason(target)
		}
		replyPlan, err := DeliverText(ctx, platform, metadata, target, fallbackChatID, text)
		return DeliveryReceipt{Type: "text", Method: replyPlan.Method, FallbackReason: firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), fallbackReason)}, err
	case "text":
		if len(plan.Text) > 0 {
			if nativeMessage, ok := synthesizeProviderNativeTextMessage(platform, metadata, target, plan.Text[0].Content); ok {
				replyPlan, err := DeliverNative(ctx, platform, metadata, target, fallbackChatID, nativeMessage)
				return DeliveryReceipt{Type: "native", Method: replyPlan.Method, FallbackReason: firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), strings.TrimSpace(plan.FallbackReason))}, err
			}
		}
		replyPlan, err := deliverRenderedText(ctx, platform, metadata, target, fallbackChatID, plan.Text)
		return DeliveryReceipt{Type: "text", Method: replyPlan.Method, FallbackReason: firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), strings.TrimSpace(plan.FallbackReason))}, err
	default:
		return DeliveryReceipt{}, errors.New("rendering plan is missing a supported type")
	}
}

func deliverRenderedText(ctx context.Context, platform Platform, metadata PlatformMetadata, target *ReplyTarget, fallbackChatID string, text []RenderedText) (ReplyPlan, error) {
	if len(text) == 0 {
		return DeliverText(ctx, platform, metadata, target, fallbackChatID, "")
	}
	rendered := text[0]
	if rendered.Format == "" || rendered.Format == TextFormatPlainText {
		return DeliverText(ctx, platform, metadata, target, fallbackChatID, rendered.Content)
	}

	sender, ok := platform.(FormattedTextSender)
	if !ok {
		return DeliverText(ctx, platform, metadata, target, fallbackChatID, rendered.Content)
	}

	message := &FormattedText{
		Content: rendered.Content,
		Format:  rendered.Format,
	}
	plan := ResolveReplyPlan(metadata, target, fallbackChatID)
	if plan.Method == DeliveryMethodSend {
		if plan.TargetChatID == "" {
			return plan, errors.New("delivery missing target chat id")
		}
		return plan, sender.SendFormattedText(ctx, plan.TargetChatID, message)
	}

	replyCtx := restoreReplyContext(platform, target)
	switch plan.Method {
	case DeliveryMethodEdit:
		if replyCtx != nil {
			return plan, sender.UpdateFormattedText(ctx, replyCtx, message)
		}
	case DeliveryMethodReply, DeliveryMethodThreadReply, DeliveryMethodFollowUp, DeliveryMethodSessionWebhook, DeliveryMethodDeferredCardUpdate:
		if replyCtx != nil {
			return plan, sender.ReplyFormattedText(ctx, replyCtx, message)
		}
	}

	if plan.TargetChatID == "" {
		return plan, errors.New("delivery missing target chat id")
	}
	plan.Method = DeliveryMethodSend
	return plan, sender.SendFormattedText(ctx, plan.TargetChatID, message)
}

func synthesizeNativeDelivery(metadata PlatformMetadata, delivery *DeliveryEnvelope) *NativeMessage {
	if delivery == nil || delivery.Native != nil || delivery.ReplyTarget == nil {
		return nil
	}
	if metadata.Source != "feishu" {
		return nil
	}
	if strings.TrimSpace(delivery.ReplyTarget.ProgressMode) != string(AsyncUpdateDeferredCardUpdate) {
		return nil
	}
	if strings.TrimSpace(delivery.ReplyTarget.CallbackToken) == "" {
		return nil
	}

	content := strings.TrimSpace(delivery.Content)
	if content == "" && delivery.Structured != nil {
		content = strings.TrimSpace(delivery.Structured.FallbackText())
	}
	if content == "" {
		return nil
	}

	message, err := NewFeishuMarkdownCardMessage("AgentForge Update", content)
	if err != nil {
		return nil
	}
	return message
}

func synthesizeProviderNativeTextMessage(platform Platform, metadata PlatformMetadata, target *ReplyTarget, content string) (*NativeMessage, bool) {
	if target == nil {
		return nil, false
	}
	if metadata.Source != "feishu" {
		return nil, false
	}
	if strings.TrimSpace(target.ProgressMode) != string(AsyncUpdateDeferredCardUpdate) {
		return nil, false
	}
	if strings.TrimSpace(target.CallbackToken) == "" {
		return nil, false
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil, false
	}
	if builder, ok := platform.(NativeTextMessageBuilder); ok {
		message, err := builder.BuildNativeTextMessage("AgentForge Update", trimmed)
		if err == nil && message != nil {
			return message, true
		}
	}
	message, err := NewFeishuMarkdownCardMessage("AgentForge Update", trimmed)
	if err != nil || message == nil {
		return nil, false
	}
	return message, true
}

func inferStructuredFallbackReason(target *ReplyTarget) string {
	if target != nil {
		return "structured_reply_unavailable"
	}
	return "structured_delivery_unavailable"
}

func metadataValue(metadata map[string]string, key string) string {
	if metadata == nil {
		return ""
	}
	return strings.TrimSpace(metadata[key])
}

func resolveTextFormat(metadata PlatformMetadata, deliveryMetadata map[string]string) TextFormatMode {
	requested := TextFormatMode(strings.TrimSpace(metadataValue(deliveryMetadata, "text_format")))
	if requested != "" && hasTextFormat(metadata.Rendering.SupportedFormats, requested) {
		return requested
	}
	return metadata.Rendering.DefaultTextFormat
}
