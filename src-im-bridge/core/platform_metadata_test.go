package core

import (
	"context"
	"reflect"
	"testing"
)

type metadataOnlyPlatform struct {
	name     string
	metadata PlatformMetadata
}

func (p *metadataOnlyPlatform) Name() string { return p.name }

func (p *metadataOnlyPlatform) Start(handler MessageHandler) error { return nil }

func (p *metadataOnlyPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	return nil
}

func (p *metadataOnlyPlatform) Send(ctx context.Context, chatID string, content string) error {
	return nil
}

func (p *metadataOnlyPlatform) Stop() error { return nil }

func (p *metadataOnlyPlatform) Metadata() PlatformMetadata {
	return p.metadata
}

func TestMetadataForPlatform_NormalizesCapabilityMatrixAndLegacyFlags(t *testing.T) {
	platform := &metadataOnlyPlatform{
		name: "slack-stub",
		metadata: PlatformMetadata{
			Source: "Slack",
			Capabilities: PlatformCapabilities{
				CommandSurface:       CommandSurfaceMixed,
				StructuredSurface:    StructuredSurfaceBlocks,
				SupportsRichMessages: true,
				AsyncUpdateModes: []AsyncUpdateMode{
					AsyncUpdateThreadReply,
					AsyncUpdateFollowUp,
				},
				ActionCallbackMode: ActionCallbackSocketPayload,
				MessageScopes: []MessageScope{
					MessageScopeChat,
					MessageScopeThread,
				},
				Mutability: MutabilitySemantics{
					CanEdit:        true,
					PrefersInPlace: false,
				},
			},
		},
	}

	metadata := MetadataForPlatform(platform)
	if metadata.Source != "slack" {
		t.Fatalf("Source = %q, want slack", metadata.Source)
	}
	if metadata.Capabilities.CommandSurface != CommandSurfaceMixed {
		t.Fatalf("CommandSurface = %q", metadata.Capabilities.CommandSurface)
	}
	if metadata.Capabilities.StructuredSurface != StructuredSurfaceBlocks {
		t.Fatalf("StructuredSurface = %q", metadata.Capabilities.StructuredSurface)
	}
	if !metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected structured surface to imply rich-message support")
	}
	if !metadata.Capabilities.SupportsDeferredReply {
		t.Fatal("expected follow-up support to imply deferred reply support")
	}
	if !metadata.Capabilities.SupportsSlashCommands {
		t.Fatal("expected mixed command surface to imply slash-command support")
	}
	if !metadata.Capabilities.SupportsMentions {
		t.Fatal("expected mixed command surface to imply mention support")
	}
	if len(metadata.Capabilities.AsyncUpdateModes) != 2 {
		t.Fatalf("AsyncUpdateModes = %+v", metadata.Capabilities.AsyncUpdateModes)
	}
	if len(metadata.Capabilities.MessageScopes) != 2 {
		t.Fatalf("MessageScopes = %+v", metadata.Capabilities.MessageScopes)
	}
	if !metadata.Capabilities.Mutability.CanEdit {
		t.Fatal("expected mutability semantics to be preserved")
	}
}

func TestMetadataForPlatform_DefaultsStructuredSurfaceAndMatrixFromLegacyCapabilities(t *testing.T) {
	platform := &metadataOnlyPlatform{
		name: "feishu-stub",
		metadata: PlatformMetadata{
			Source: "feishu",
			Capabilities: PlatformCapabilities{
				SupportsRichMessages:  true,
				SupportsSlashCommands: true,
				SupportsMentions:      true,
			},
		},
	}

	metadata := MetadataForPlatform(platform)
	if metadata.Capabilities.StructuredSurface != StructuredSurfaceCards {
		t.Fatalf("StructuredSurface = %q, want %q", metadata.Capabilities.StructuredSurface, StructuredSurfaceCards)
	}
	if metadata.Capabilities.CommandSurface != CommandSurfaceMixed {
		t.Fatalf("CommandSurface = %q, want %q", metadata.Capabilities.CommandSurface, CommandSurfaceMixed)
	}
	if len(metadata.Capabilities.MessageScopes) == 0 || metadata.Capabilities.MessageScopes[0] != MessageScopeChat {
		t.Fatalf("MessageScopes = %+v, want chat default", metadata.Capabilities.MessageScopes)
	}
	if metadata.Capabilities.ActionCallbackMode != ActionCallbackWebhook {
		t.Fatalf("ActionCallbackMode = %q, want %q", metadata.Capabilities.ActionCallbackMode, ActionCallbackWebhook)
	}
	if metadata.Rendering.DefaultTextFormat != TextFormatPlainText {
		t.Fatalf("DefaultTextFormat = %q, want %q", metadata.Rendering.DefaultTextFormat, TextFormatPlainText)
	}
	if metadata.Rendering.MaxTextLength != 30000 {
		t.Fatalf("MaxTextLength = %d, want 30000", metadata.Rendering.MaxTextLength)
	}
	if !metadata.Rendering.UsesProviderOwnedBuilders {
		t.Fatal("expected feishu rendering profile to use provider-owned builders")
	}
	if !hasTextFormat(metadata.Rendering.SupportedFormats, TextFormatLarkMD) {
		t.Fatalf("SupportedFormats = %+v, want lark_md", metadata.Rendering.SupportedFormats)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != NativeSurfaceFeishuCard {
		t.Fatalf("NativeSurfaces = %+v, want [%q]", metadata.Rendering.NativeSurfaces, NativeSurfaceFeishuCard)
	}
}

type metadataCardPlatform struct {
	name string
}

func (p *metadataCardPlatform) Name() string { return p.name }

func (p *metadataCardPlatform) Start(handler MessageHandler) error { return nil }

func (p *metadataCardPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	return nil
}

func (p *metadataCardPlatform) Send(ctx context.Context, chatID string, content string) error {
	return nil
}

func (p *metadataCardPlatform) Stop() error { return nil }

func (p *metadataCardPlatform) SendCard(ctx context.Context, chatID string, card *Card) error {
	return nil
}

func (p *metadataCardPlatform) ReplyCard(ctx context.Context, replyCtx any, card *Card) error {
	return nil
}

func TestMetadataForPlatform_InfersRichMessagesFromCardSenderWithoutMetadataProvider(t *testing.T) {
	metadata := MetadataForPlatform(&metadataCardPlatform{name: "dingtalk-stub"})

	if !metadata.Capabilities.SupportsRichMessages {
		t.Fatal("expected CardSender platforms without explicit metadata to advertise rich-message support")
	}
	if metadata.Capabilities.StructuredSurface != StructuredSurfaceActionCard {
		t.Fatalf("StructuredSurface = %q, want %q", metadata.Capabilities.StructuredSurface, StructuredSurfaceActionCard)
	}
}

func TestPlatformCapabilities_MatrixAndLookupHelpers(t *testing.T) {
	capabilities := PlatformCapabilities{
		CommandSurface:     CommandSurfaceMixed,
		StructuredSurface:  StructuredSurfaceBlocks,
		AsyncUpdateModes:   []AsyncUpdateMode{AsyncUpdateReply, AsyncUpdateFollowUp},
		ActionCallbackMode: ActionCallbackSocketPayload,
		MessageScopes:      []MessageScope{MessageScopeChat, MessageScopeThread},
		NativeSurfaces:     []string{NativeSurfaceSlackBlockKit},
		Mutability: MutabilitySemantics{
			CanEdit:        true,
			PrefersInPlace: true,
		},
	}

	matrix := capabilities.Matrix()
	if matrix["commandSurface"] != "mixed" || matrix["structuredSurface"] != "blocks" {
		t.Fatalf("matrix = %+v", matrix)
	}
	if nativeSurfaces, ok := matrix["nativeSurfaces"].([]string); !ok || len(nativeSurfaces) != 1 || nativeSurfaces[0] != NativeSurfaceSlackBlockKit {
		t.Fatalf("nativeSurfaces = %+v", matrix["nativeSurfaces"])
	}
	if !capabilities.HasAsyncUpdateMode(AsyncUpdateFollowUp) {
		t.Fatal("expected async update mode to be found")
	}
	if capabilities.HasAsyncUpdateMode(AsyncUpdateSessionWebhook) {
		t.Fatal("did not expect absent async update mode")
	}
	if !capabilities.HasMessageScope(MessageScopeThread) {
		t.Fatal("expected message scope to be found")
	}
	if capabilities.HasMessageScope(MessageScopeTopic) {
		t.Fatal("did not expect absent message scope")
	}
	if !capabilities.HasNativeSurface(" slack_block_kit ") {
		t.Fatal("expected native surface lookup to ignore case and whitespace")
	}
	if capabilities.HasNativeSurface(NativeSurfaceDiscordEmbed) {
		t.Fatal("did not expect absent native surface")
	}
}

func TestMetadataForPlatform_DefaultsTelegramRenderingProfile(t *testing.T) {
	metadata := MetadataForPlatform(&metadataOnlyPlatform{
		name: "telegram-stub",
		metadata: PlatformMetadata{
			Source: "telegram",
			Capabilities: PlatformCapabilities{
				StructuredSurface: StructuredSurfaceInlineKeyboard,
				AsyncUpdateModes:  []AsyncUpdateMode{AsyncUpdateReply, AsyncUpdateEdit},
				MessageScopes:     []MessageScope{MessageScopeChat, MessageScopeTopic},
				Mutability: MutabilitySemantics{
					CanEdit:        true,
					PrefersInPlace: true,
				},
			},
		},
	})

	if metadata.Rendering.DefaultTextFormat != TextFormatPlainText {
		t.Fatalf("DefaultTextFormat = %q, want %q", metadata.Rendering.DefaultTextFormat, TextFormatPlainText)
	}
	if metadata.Rendering.MaxTextLength != 4096 {
		t.Fatalf("MaxTextLength = %d, want 4096", metadata.Rendering.MaxTextLength)
	}
	if !metadata.Rendering.SupportsSegments {
		t.Fatal("expected telegram rendering profile to support segments")
	}
	if metadata.Rendering.StructuredSurface != StructuredSurfaceInlineKeyboard {
		t.Fatalf("StructuredSurface = %q, want %q", metadata.Rendering.StructuredSurface, StructuredSurfaceInlineKeyboard)
	}
	if !hasTextFormat(metadata.Rendering.SupportedFormats, TextFormatMarkdownV2) {
		t.Fatalf("SupportedFormats = %+v, want markdown_v2", metadata.Rendering.SupportedFormats)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != NativeSurfaceTelegramRich {
		t.Fatalf("NativeSurfaces = %+v, want [%q]", metadata.Rendering.NativeSurfaces, NativeSurfaceTelegramRich)
	}
}

func TestNormalizeMetadata_UsesFallbackSourceForRenderingDefaults(t *testing.T) {
	metadata := NormalizeMetadata(PlatformMetadata{}, "discord-live")
	if metadata.Source != "discord" {
		t.Fatalf("Source = %q, want discord", metadata.Source)
	}
	if metadata.Rendering.DefaultTextFormat != TextFormatPlainText {
		t.Fatalf("DefaultTextFormat = %q", metadata.Rendering.DefaultTextFormat)
	}
	if metadata.Rendering.StructuredSurface != StructuredSurfaceComponents {
		t.Fatalf("StructuredSurface = %q, want %q", metadata.Rendering.StructuredSurface, StructuredSurfaceComponents)
	}
	if !hasTextFormat(metadata.Rendering.SupportedFormats, TextFormatDiscordMD) {
		t.Fatalf("SupportedFormats = %+v, want discord_md", metadata.Rendering.SupportedFormats)
	}
	if len(metadata.Rendering.NativeSurfaces) != 1 || metadata.Rendering.NativeSurfaces[0] != NativeSurfaceDiscordEmbed {
		t.Fatalf("NativeSurfaces = %+v, want [%q]", metadata.Rendering.NativeSurfaces, NativeSurfaceDiscordEmbed)
	}
}

func TestDefaultCapabilitiesForSource_CoversAdditionalPlatformsAndFallbacks(t *testing.T) {
	qqbot := defaultCapabilitiesForSource("qqbot", nil)
	if qqbot.ReadinessTier != ReadinessTierNativeSendWithFallback {
		t.Fatalf("qqbot ReadinessTier = %q, want %q", qqbot.ReadinessTier, ReadinessTierNativeSendWithFallback)
	}
	if !qqbot.RequiresPublicCallback {
		t.Fatal("expected qqbot to require public callback")
	}
	if !qqbot.SupportsSlashCommands || !qqbot.SupportsMentions {
		t.Fatalf("qqbot capabilities = %+v", qqbot)
	}
	if !reflect.DeepEqual(qqbot.NativeSurfaces, []string{NativeSurfaceQQBotMarkdown}) {
		t.Fatalf("qqbot NativeSurfaces = %+v", qqbot.NativeSurfaces)
	}
	if qqbot.MutableUpdateMethod != "openapi_patch" {
		t.Fatalf("qqbot MutableUpdateMethod = %q, want openapi_patch", qqbot.MutableUpdateMethod)
	}

	wecom := defaultCapabilitiesForSource("wecom", nil)
	if wecom.ReadinessTier != ReadinessTierFullNativeLifecycle {
		t.Fatalf("wecom ReadinessTier = %q, want %q", wecom.ReadinessTier, ReadinessTierFullNativeLifecycle)
	}
	if wecom.CommandSurface != CommandSurfaceInteraction {
		t.Fatalf("wecom CommandSurface = %q", wecom.CommandSurface)
	}
	if !reflect.DeepEqual(wecom.NativeSurfaces, []string{NativeSurfaceWeComCard}) {
		t.Fatalf("wecom NativeSurfaces = %+v", wecom.NativeSurfaces)
	}
	if wecom.MutableUpdateMethod != "template_card_update" {
		t.Fatalf("wecom MutableUpdateMethod = %q, want template_card_update", wecom.MutableUpdateMethod)
	}

	feishu := defaultCapabilitiesForSource("feishu", nil)
	if feishu.ReadinessTier != ReadinessTierFullNativeLifecycle {
		t.Fatalf("feishu ReadinessTier = %q, want %q", feishu.ReadinessTier, ReadinessTierFullNativeLifecycle)
	}

	dingtalk := defaultCapabilitiesForSource("dingtalk", nil)
	if dingtalk.ReadinessTier != ReadinessTierFullNativeLifecycle {
		t.Fatalf("dingtalk ReadinessTier = %q, want %q", dingtalk.ReadinessTier, ReadinessTierFullNativeLifecycle)
	}
	if !dingtalk.SupportsRichMessages {
		t.Fatalf("dingtalk capabilities = %+v, want SupportsRichMessages", dingtalk)
	}
	if dingtalk.MutableUpdateMethod != "openapi_only" {
		t.Fatalf("dingtalk MutableUpdateMethod = %q, want openapi_only", dingtalk.MutableUpdateMethod)
	}

	qq := defaultCapabilitiesForSource("qq", nil)
	if qq.ReadinessTier != ReadinessTierTextFirst {
		t.Fatalf("qq ReadinessTier = %q, want %q", qq.ReadinessTier, ReadinessTierTextFirst)
	}
	if qq.MutableUpdateMethod != "simulated" {
		t.Fatalf("qq MutableUpdateMethod = %q, want simulated", qq.MutableUpdateMethod)
	}

	customCard := defaultCapabilitiesForSource("custom", &metadataCardPlatform{name: "custom-card"})
	if customCard.StructuredSurface != StructuredSurfaceCards || !customCard.SupportsRichMessages {
		t.Fatalf("customCard capabilities = %+v", customCard)
	}
	if !reflect.DeepEqual(customCard.MessageScopes, []MessageScope{MessageScopeChat}) {
		t.Fatalf("customCard MessageScopes = %+v", customCard.MessageScopes)
	}

	custom := defaultCapabilitiesForSource("custom", nil)
	if custom.CommandSurface != CommandSurfaceNone {
		t.Fatalf("custom CommandSurface = %q", custom.CommandSurface)
	}
	if !reflect.DeepEqual(custom.MessageScopes, []MessageScope{MessageScopeChat}) {
		t.Fatalf("custom MessageScopes = %+v", custom.MessageScopes)
	}
}

func TestNormalizeCapabilities_DerivesLegacyFlagsAndDefaults(t *testing.T) {
	defaults := PlatformCapabilities{
		StructuredSurface:  StructuredSurfaceBlocks,
		AsyncUpdateModes:   []AsyncUpdateMode{AsyncUpdateReply},
		ActionCallbackMode: ActionCallbackWebhook,
		MessageScopes:      []MessageScope{MessageScopeThread},
		NativeSurfaces:     []string{NativeSurfaceSlackBlockKit},
		Mutability: MutabilitySemantics{
			CanEdit: true,
		},
	}

	got := normalizeCapabilities(PlatformCapabilities{
		SupportsSlashCommands:  true,
		SupportsMentions:       true,
		SupportsRichMessages:   true,
		SupportsDeferredReply:  true,
		RequiresPublicCallback: true,
	}, defaults)

	if got.CommandSurface != CommandSurfaceMixed {
		t.Fatalf("CommandSurface = %q, want %q", got.CommandSurface, CommandSurfaceMixed)
	}
	if got.StructuredSurface != StructuredSurfaceBlocks {
		t.Fatalf("StructuredSurface = %q, want %q", got.StructuredSurface, StructuredSurfaceBlocks)
	}
	if !reflect.DeepEqual(got.AsyncUpdateModes, []AsyncUpdateMode{AsyncUpdateFollowUp}) {
		t.Fatalf("AsyncUpdateModes = %+v", got.AsyncUpdateModes)
	}
	if got.ActionCallbackMode != ActionCallbackInteractionToken {
		t.Fatalf("ActionCallbackMode = %q, want %q", got.ActionCallbackMode, ActionCallbackInteractionToken)
	}
	if !reflect.DeepEqual(got.MessageScopes, defaults.MessageScopes) {
		t.Fatalf("MessageScopes = %+v, want %+v", got.MessageScopes, defaults.MessageScopes)
	}
	if !reflect.DeepEqual(got.NativeSurfaces, defaults.NativeSurfaces) {
		t.Fatalf("NativeSurfaces = %+v, want %+v", got.NativeSurfaces, defaults.NativeSurfaces)
	}
	if got.Mutability != defaults.Mutability {
		t.Fatalf("Mutability = %+v, want %+v", got.Mutability, defaults.Mutability)
	}
	if !got.SupportsDeferredReply || !got.RequiresPublicCallback || !got.SupportsSlashCommands || !got.SupportsMentions {
		t.Fatalf("normalized flags = %+v", got)
	}
}

func TestNormalizeCapabilities_DefaultsWebhookForStructuredSurface(t *testing.T) {
	got := normalizeCapabilities(PlatformCapabilities{
		StructuredSurface: StructuredSurfaceCards,
	}, PlatformCapabilities{})

	if got.ActionCallbackMode != ActionCallbackWebhook {
		t.Fatalf("ActionCallbackMode = %q, want %q", got.ActionCallbackMode, ActionCallbackWebhook)
	}
	if !reflect.DeepEqual(got.MessageScopes, []MessageScope{MessageScopeChat}) {
		t.Fatalf("MessageScopes = %+v", got.MessageScopes)
	}
}

func TestDefaultRenderingProfileForSource_AssignsChinaPlatformReadinessTiers(t *testing.T) {
	tests := []struct {
		source string
		want   ReadinessTier
	}{
		{source: "feishu", want: ReadinessTierFullNativeLifecycle},
		{source: "dingtalk", want: ReadinessTierNativeSendWithFallback},
		{source: "wecom", want: ReadinessTierNativeSendWithFallback},
		{source: "qq", want: ReadinessTierTextFirst},
		{source: "qqbot", want: ReadinessTierMarkdownFirst},
	}

	for _, tc := range tests {
		t.Run(tc.source, func(t *testing.T) {
			profile := defaultRenderingProfileForSource(tc.source, defaultCapabilitiesForSource(tc.source, nil))
			if profile.ReadinessTier != tc.want {
				t.Fatalf("ReadinessTier = %q, want %q", profile.ReadinessTier, tc.want)
			}
		})
	}
}

func TestPlatformCapabilities_MatrixIncludesReadinessTier(t *testing.T) {
	matrix := PlatformCapabilities{
		CommandSurface:           CommandSurfaceMixed,
		ReadinessTier:            ReadinessTierNativeSendWithFallback,
		PreferredAsyncUpdateMode: AsyncUpdateSessionWebhook,
		FallbackAsyncUpdateMode:  AsyncUpdateReply,
	}.Matrix()

	if matrix["readinessTier"] != string(ReadinessTierNativeSendWithFallback) {
		t.Fatalf("matrix = %+v", matrix)
	}
	if matrix["preferredAsyncUpdateMode"] != string(AsyncUpdateSessionWebhook) {
		t.Fatalf("matrix = %+v", matrix)
	}
	if matrix["fallbackAsyncUpdateMode"] != string(AsyncUpdateReply) {
		t.Fatalf("matrix = %+v", matrix)
	}
}

func TestDefaultCapabilitiesForSource_ExposeChinaPlatformCompletionPreferences(t *testing.T) {
	dingtalk := defaultCapabilitiesForSource("dingtalk", nil)
	if dingtalk.PreferredAsyncUpdateMode != AsyncUpdateSessionWebhook {
		t.Fatalf("dingtalk PreferredAsyncUpdateMode = %q, want %q", dingtalk.PreferredAsyncUpdateMode, AsyncUpdateSessionWebhook)
	}
	if dingtalk.FallbackAsyncUpdateMode != AsyncUpdateReply {
		t.Fatalf("dingtalk FallbackAsyncUpdateMode = %q, want %q", dingtalk.FallbackAsyncUpdateMode, AsyncUpdateReply)
	}

	wecom := defaultCapabilitiesForSource("wecom", nil)
	if wecom.PreferredAsyncUpdateMode != AsyncUpdateSessionWebhook {
		t.Fatalf("wecom PreferredAsyncUpdateMode = %q, want %q", wecom.PreferredAsyncUpdateMode, AsyncUpdateSessionWebhook)
	}
	if wecom.FallbackAsyncUpdateMode != AsyncUpdateReply {
		t.Fatalf("wecom FallbackAsyncUpdateMode = %q, want %q", wecom.FallbackAsyncUpdateMode, AsyncUpdateReply)
	}

	qqbot := defaultCapabilitiesForSource("qqbot", nil)
	if qqbot.PreferredAsyncUpdateMode != AsyncUpdateReply {
		t.Fatalf("qqbot PreferredAsyncUpdateMode = %q, want %q", qqbot.PreferredAsyncUpdateMode, AsyncUpdateReply)
	}

	qq := defaultCapabilitiesForSource("qq", nil)
	if qq.PreferredAsyncUpdateMode != AsyncUpdateReply {
		t.Fatalf("qq PreferredAsyncUpdateMode = %q, want %q", qq.PreferredAsyncUpdateMode, AsyncUpdateReply)
	}
}
