package core

import "strings"

// TextFormatMode describes how text should be rendered for a provider.
type TextFormatMode string

const (
	TextFormatPlainText   TextFormatMode = "plain_text"
	TextFormatMarkdownV2  TextFormatMode = "markdown_v2"
	TextFormatHTML        TextFormatMode = "html"
	TextFormatLarkMD      TextFormatMode = "lark_md"
	TextFormatSlackMrkdwn TextFormatMode = "slack_mrkdwn"
	TextFormatDiscordMD   TextFormatMode = "discord_md"
	TextFormatDingTalkMD  TextFormatMode = "dingtalk_md"
	TextFormatWeComMD    TextFormatMode = "wecom_md"
	TextFormatQQBotMD    TextFormatMode = "qqbot_md"
)

// RenderingProfile describes provider-owned delivery formatting constraints.
type RenderingProfile struct {
	DefaultTextFormat         TextFormatMode    `json:"defaultTextFormat,omitempty"`
	SupportedFormats          []TextFormatMode  `json:"supportedFormats,omitempty"`
	NativeSurfaces            []string          `json:"nativeSurfaces,omitempty"`
	MaxTextLength             int               `json:"maxTextLength,omitempty"`
	SupportsSegments          bool              `json:"supportsSegments,omitempty"`
	StructuredSurface         StructuredSurface `json:"structuredSurface,omitempty"`
	ReadinessTier             ReadinessTier     `json:"readinessTier,omitempty"`
	UsesProviderOwnedBuilders bool              `json:"usesProviderOwnedBuilders,omitempty"`
}

// RenderedText captures a text segment after provider-aware rendering.
type RenderedText struct {
	Content string
	Format  TextFormatMode
}

// RenderingPlan captures the provider-aware representation chosen for a
// delivery before transport execution.
type RenderingPlan struct {
	Type           string
	Method         DeliveryMethod
	Text           []RenderedText
	Structured     *StructuredMessage
	Native         *NativeMessage
	FallbackReason string
}

func normalizeRenderingProfile(profile RenderingProfile, defaults RenderingProfile) RenderingProfile {
	if profile.DefaultTextFormat == "" {
		profile.DefaultTextFormat = defaults.DefaultTextFormat
	}
	if len(profile.SupportedFormats) == 0 && len(defaults.SupportedFormats) > 0 {
		profile.SupportedFormats = append([]TextFormatMode(nil), defaults.SupportedFormats...)
	}
	if len(profile.NativeSurfaces) == 0 && len(defaults.NativeSurfaces) > 0 {
		profile.NativeSurfaces = append([]string(nil), defaults.NativeSurfaces...)
	}
	if profile.MaxTextLength == 0 {
		profile.MaxTextLength = defaults.MaxTextLength
	}
	if !profile.SupportsSegments {
		profile.SupportsSegments = defaults.SupportsSegments
	}
	if profile.StructuredSurface == "" {
		profile.StructuredSurface = defaults.StructuredSurface
	}
	if profile.ReadinessTier == "" {
		profile.ReadinessTier = defaults.ReadinessTier
	}
	profile.UsesProviderOwnedBuilders = profile.UsesProviderOwnedBuilders || defaults.UsesProviderOwnedBuilders

	if profile.DefaultTextFormat == "" {
		profile.DefaultTextFormat = TextFormatPlainText
	}
	if len(profile.SupportedFormats) == 0 {
		profile.SupportedFormats = []TextFormatMode{profile.DefaultTextFormat}
	}
	if !hasTextFormat(profile.SupportedFormats, profile.DefaultTextFormat) {
		profile.SupportedFormats = append([]TextFormatMode{profile.DefaultTextFormat}, profile.SupportedFormats...)
	}
	return profile
}

func hasTextFormat(formats []TextFormatMode, target TextFormatMode) bool {
	for _, format := range formats {
		if strings.EqualFold(string(format), string(target)) {
			return true
		}
	}
	return false
}

func hasStringFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func defaultRenderingProfileForSource(source string, capabilities PlatformCapabilities) RenderingProfile {
	profile := RenderingProfile{
		DefaultTextFormat: TextFormatPlainText,
		SupportedFormats:  []TextFormatMode{TextFormatPlainText},
		NativeSurfaces:    append([]string(nil), capabilities.NativeSurfaces...),
		MaxTextLength:     4096,
		SupportsSegments:  true,
		StructuredSurface: capabilities.StructuredSurface,
	}

	switch NormalizePlatformName(source) {
	case "feishu":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatLarkMD}
		if len(profile.NativeSurfaces) == 0 {
			profile.NativeSurfaces = []string{NativeSurfaceFeishuCard}
		}
		profile.MaxTextLength = 30000
		profile.StructuredSurface = StructuredSurfaceCards
		profile.ReadinessTier = ReadinessTierFullNativeLifecycle
		profile.UsesProviderOwnedBuilders = true
	case "telegram":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatMarkdownV2}
		if len(profile.NativeSurfaces) == 0 {
			profile.NativeSurfaces = []string{NativeSurfaceTelegramRich}
		}
		profile.MaxTextLength = 4096
		profile.StructuredSurface = StructuredSurfaceInlineKeyboard
	case "slack":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatSlackMrkdwn}
		if len(profile.NativeSurfaces) == 0 {
			profile.NativeSurfaces = []string{NativeSurfaceSlackBlockKit}
		}
		profile.MaxTextLength = 4000
		profile.StructuredSurface = StructuredSurfaceBlocks
	case "discord":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatDiscordMD}
		if len(profile.NativeSurfaces) == 0 {
			profile.NativeSurfaces = []string{NativeSurfaceDiscordEmbed}
		}
		profile.MaxTextLength = 2000
		profile.StructuredSurface = StructuredSurfaceComponents
	case "dingtalk":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatDingTalkMD}
		if len(profile.NativeSurfaces) == 0 {
			profile.NativeSurfaces = []string{NativeSurfaceDingTalkCard}
		}
		profile.MaxTextLength = 20000
		profile.StructuredSurface = StructuredSurfaceActionCard
		profile.ReadinessTier = ReadinessTierNativeSendWithFallback
	case "qq":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText}
		profile.MaxTextLength = 4096
		profile.StructuredSurface = StructuredSurfaceNone
		profile.ReadinessTier = ReadinessTierTextFirst
	case "qqbot":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatQQBotMD}
		if len(profile.NativeSurfaces) == 0 {
			profile.NativeSurfaces = []string{NativeSurfaceQQBotMarkdown}
		}
		profile.MaxTextLength = 2000
		profile.StructuredSurface = StructuredSurfaceNone
		profile.ReadinessTier = ReadinessTierMarkdownFirst
	case "wecom":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatWeComMD}
		if len(profile.NativeSurfaces) == 0 {
			profile.NativeSurfaces = []string{NativeSurfaceWeComCard}
		}
		profile.MaxTextLength = 2000
		profile.StructuredSurface = StructuredSurfaceNone
		profile.ReadinessTier = ReadinessTierNativeSendWithFallback
	case "wechat":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText}
		profile.MaxTextLength = 2048
		profile.StructuredSurface = StructuredSurfaceNone
		profile.ReadinessTier = ReadinessTierTextFirst
	case "email":
		profile.DefaultTextFormat = TextFormatHTML
		profile.SupportedFormats = []TextFormatMode{TextFormatHTML, TextFormatPlainText}
		profile.MaxTextLength = 0
		profile.SupportsSegments = false
		profile.StructuredSurface = StructuredSurfaceNone
	default:
		if profile.StructuredSurface == "" {
			profile.StructuredSurface = StructuredSurfaceNone
		}
	}

	return profile
}
