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
	Attachments []Attachment
	ReplyTarget *ReplyTarget
	Metadata    map[string]string
}

type DeliveryReceipt struct {
	Type             string
	Method           DeliveryMethod
	FallbackReason   string
	AttachmentsSent  int
	AttachmentsBytes int64
}

func ResolveRenderingPlan(metadata PlatformMetadata, fallbackChatID string, delivery *DeliveryEnvelope) (RenderingPlan, error) {
	if delivery == nil {
		return RenderingPlan{}, errors.New("delivery is required")
	}
	metadata = NormalizeMetadata(metadata, metadata.Source)
	plan := RenderingPlan{}

	if delivery.Native != nil {
		replyPlan := ResolveReplyPlan(metadata, delivery.ReplyTarget, fallbackChatID)
		if err := delivery.Native.Validate(); err != nil {
			return RenderingPlan{}, err
		}
		if nativePlatform := delivery.Native.NormalizedPlatform(); nativePlatform != "" && NormalizePlatformName(metadata.Source) != "" && nativePlatform != NormalizePlatformName(metadata.Source) {
			return nativeFallbackRenderingPlan(replyPlan, metadata, delivery.Metadata, delivery.Native, "native_platform_mismatch"), nil
		}
		if surface := delivery.Native.SurfaceType(); surface != "" && !hasStringFold(metadata.Rendering.NativeSurfaces, surface) {
			return nativeFallbackRenderingPlan(replyPlan, metadata, delivery.Metadata, delivery.Native, "native_surface_unsupported"), nil
		}
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

	attachmentsReceipt, attachmentsFallbackReason, attachmentsErr := deliverAttachments(ctx, platform, metadata, fallbackChatID, delivery)
	if attachmentsErr != nil {
		return attachmentsReceipt, attachmentsErr
	}

	renderingPlan, err := ResolveRenderingPlan(metadata, fallbackChatID, delivery)
	if err != nil {
		return DeliveryReceipt{}, err
	}
	if attachmentsFallbackReason != "" && renderingPlan.FallbackReason == "" {
		renderingPlan.FallbackReason = attachmentsFallbackReason
	}

	receipt, err := executeRenderingPlan(ctx, platform, metadata, fallbackChatID, delivery.ReplyTarget, renderingPlan)
	receipt.AttachmentsSent = attachmentsReceipt.AttachmentsSent
	receipt.AttachmentsBytes = attachmentsReceipt.AttachmentsBytes
	if err == nil {
		dispatchAckReaction(ctx, platform, metadata, delivery)
	}
	return receipt, err
}

// dispatchAckReaction fires an ack-style emoji reaction against the reply
// target when the envelope metadata requests one and the provider supports
// reactions. Failures are intentionally swallowed: the primary delivery has
// already succeeded and reaction issues should not fail it.
func dispatchAckReaction(ctx context.Context, platform Platform, metadata PlatformMetadata, delivery *DeliveryEnvelope) {
	if delivery == nil || delivery.Metadata == nil {
		return
	}
	code := strings.TrimSpace(delivery.Metadata["ack_reaction"])
	if code == "" {
		return
	}
	if !metadata.Capabilities.SupportsReactions {
		return
	}
	sender, ok := platform.(ReactionSender)
	if !ok {
		return
	}
	replyCtx := restoreReplyContext(platform, delivery.ReplyTarget)
	if replyCtx == nil {
		return
	}
	native := NativeEmojiForCode(metadata.Source, code)
	_ = sender.SendReaction(ctx, replyCtx, native)
}

// deliverAttachments handles the attachments layer of the delivery ladder.
// If the provider supports attachments and the envelope is within capability
// limits, each attachment is uploaded + sent. Otherwise the caller downgrades
// to text/structured with an appended "attachments_unsupported" fallback
// reason and an auto-generated text summary linking to ContentRef when set.
func deliverAttachments(ctx context.Context, platform Platform, metadata PlatformMetadata, fallbackChatID string, delivery *DeliveryEnvelope) (DeliveryReceipt, string, error) {
	if delivery == nil || len(delivery.Attachments) == 0 {
		return DeliveryReceipt{}, "", nil
	}

	caps := metadata.Capabilities
	if !caps.SupportsAttachments {
		summary := summarizeAttachmentFallback(delivery.Attachments)
		if strings.TrimSpace(delivery.Content) == "" {
			delivery.Content = summary
		} else if summary != "" {
			delivery.Content = delivery.Content + "\n\n" + summary
		}
		delivery.Attachments = nil
		return DeliveryReceipt{}, "attachments_unsupported", nil
	}

	sender, ok := platform.(AttachmentSender)
	if !ok {
		summary := summarizeAttachmentFallback(delivery.Attachments)
		if strings.TrimSpace(delivery.Content) == "" {
			delivery.Content = summary
		} else if summary != "" {
			delivery.Content = delivery.Content + "\n\n" + summary
		}
		delivery.Attachments = nil
		return DeliveryReceipt{}, "attachments_sender_unavailable", nil
	}

	remaining := make([]Attachment, 0, len(delivery.Attachments))
	var sent int
	var bytes int64
	for i := range delivery.Attachments {
		att := &delivery.Attachments[i]
		if reason := rejectAttachment(caps, att); reason != "" {
			remaining = append(remaining, *att)
			if delivery.Metadata == nil {
				delivery.Metadata = map[string]string{}
			}
			if delivery.Metadata["fallback_reason"] == "" {
				delivery.Metadata["fallback_reason"] = reason
			}
			continue
		}
		target := delivery.ReplyTarget
		caption := strings.TrimSpace(delivery.Content)
		if i < len(delivery.Attachments)-1 {
			caption = ""
		}
		replyCtx := restoreReplyContext(platform, target)
		var err error
		if replyCtx != nil {
			if uploadErr := sender.UploadAttachment(ctx, fallbackChatID, att); uploadErr != nil {
				err = uploadErr
			} else {
				err = sender.ReplyAttachment(ctx, replyCtx, att, caption)
			}
		} else {
			chatID := strings.TrimSpace(fallbackChatID)
			if chatID == "" && target != nil {
				chatID = firstNonEmpty(target.ChatID, target.ChannelID, target.ConversationID)
			}
			if chatID == "" {
				return DeliveryReceipt{}, "", errors.New("attachment delivery missing target chat id")
			}
			if uploadErr := sender.UploadAttachment(ctx, chatID, att); uploadErr != nil {
				err = uploadErr
			} else {
				err = sender.SendAttachment(ctx, chatID, att, caption)
			}
		}
		if err != nil {
			return DeliveryReceipt{AttachmentsSent: sent, AttachmentsBytes: bytes}, "", err
		}
		sent++
		bytes += att.SizeBytes
		if caption != "" {
			delivery.Content = ""
		}
	}
	delivery.Attachments = remaining
	return DeliveryReceipt{AttachmentsSent: sent, AttachmentsBytes: bytes}, "", nil
}

func rejectAttachment(caps PlatformCapabilities, att *Attachment) string {
	if att == nil {
		return "attachment_invalid"
	}
	if caps.MaxAttachmentSize > 0 && att.SizeBytes > caps.MaxAttachmentSize {
		return "attachment_size_exceeded"
	}
	if len(caps.AllowedAttachmentKinds) > 0 {
		for _, allowed := range caps.AllowedAttachmentKinds {
			if allowed == att.Kind {
				return ""
			}
		}
		return "attachment_kind_rejected"
	}
	return ""
}

func summarizeAttachmentFallback(atts []Attachment) string {
	if len(atts) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("[attachments degraded to text — provider does not support file delivery]")
	for _, att := range atts {
		sb.WriteString("\n- ")
		if strings.TrimSpace(att.Filename) != "" {
			sb.WriteString(att.Filename)
		} else if strings.TrimSpace(att.ID) != "" {
			sb.WriteString(att.ID)
		} else {
			sb.WriteString(string(att.Kind))
		}
		if strings.TrimSpace(att.ContentRef) != "" && isURL(att.ContentRef) {
			sb.WriteString(" (")
			sb.WriteString(att.ContentRef)
			sb.WriteString(")")
		}
	}
	return sb.String()
}

func isURL(s string) bool {
	trimmed := strings.TrimSpace(s)
	return strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://")
}

func executeRenderingPlan(ctx context.Context, platform Platform, metadata PlatformMetadata, fallbackChatID string, target *ReplyTarget, plan RenderingPlan) (DeliveryReceipt, error) {
	switch plan.Type {
	case "native":
		if _, ok := platform.(NativeMessageSender); !ok {
			replyPlan, err := DeliverText(ctx, platform, metadata, target, fallbackChatID, plan.Native.FallbackText())
			return DeliveryReceipt{Type: "text", Method: replyPlan.Method, FallbackReason: firstNonEmpty("native_sender_unavailable", strings.TrimSpace(plan.FallbackReason))}, err
		}
		replyPlan, err := DeliverNative(ctx, platform, metadata, target, fallbackChatID, plan.Native)
		return DeliveryReceipt{Type: "native", Method: replyPlan.Method, FallbackReason: firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), strings.TrimSpace(plan.FallbackReason))}, err
	case "structured":
		if nativeMessage, ok := synthesizeProviderNativeTextMessage(platform, metadata, target, plan.Structured.FallbackText()); ok {
			replyPlan, err := DeliverNative(ctx, platform, metadata, target, fallbackChatID, nativeMessage)
			return DeliveryReceipt{Type: "native", Method: replyPlan.Method, FallbackReason: firstNonEmpty(strings.TrimSpace(replyPlan.FallbackReason), strings.TrimSpace(plan.FallbackReason))}, err
		}
		if sender, ok := platform.(ReplyStructuredSender); ok && target != nil {
			replyCtx := restoreReplyContext(platform, target)
			if replyCtx != nil {
				replyPlan := ResolveReplyPlan(metadata, target, fallbackChatID)
				if err := sender.ReplyStructured(ctx, replyCtx, plan.Structured); err == nil {
					return DeliveryReceipt{Type: "structured", Method: replyPlan.Method, FallbackReason: strings.TrimSpace(plan.FallbackReason)}, nil
				}
			}
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
	// Feishu fallback: build a markdown card even when the platform does not
	// implement NativeTextMessageBuilder.
	if metadata.Source == "feishu" {
		message, err := NewFeishuMarkdownCardMessage("AgentForge Update", trimmed)
		if err != nil || message == nil {
			return nil, false
		}
		return message, true
	}
	return nil, false
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

func nativeFallbackRenderingPlan(replyPlan ReplyPlan, metadata PlatformMetadata, deliveryMetadata map[string]string, message *NativeMessage, fallbackReason string) RenderingPlan {
	return RenderingPlan{
		Type:   "text",
		Method: replyPlan.Method,
		Text: []RenderedText{{
			Content: strings.TrimSpace(message.FallbackText()),
			Format:  resolveTextFormat(metadata, deliveryMetadata),
		}},
		FallbackReason: firstNonEmpty(fallbackReason, metadataValue(deliveryMetadata, "fallback_reason")),
	}
}
