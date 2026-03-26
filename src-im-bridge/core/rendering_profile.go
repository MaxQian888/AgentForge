package core

import "strings"

// TextFormatMode describes how text should be rendered for a provider.
type TextFormatMode string

const (
	TextFormatPlainText  TextFormatMode = "plain_text"
	TextFormatMarkdownV2 TextFormatMode = "markdown_v2"
	TextFormatHTML       TextFormatMode = "html"
	TextFormatLarkMD     TextFormatMode = "lark_md"
)

// RenderingProfile describes provider-owned delivery formatting constraints.
type RenderingProfile struct {
	DefaultTextFormat         TextFormatMode    `json:"defaultTextFormat,omitempty"`
	SupportedFormats          []TextFormatMode  `json:"supportedFormats,omitempty"`
	MaxTextLength             int               `json:"maxTextLength,omitempty"`
	SupportsSegments          bool              `json:"supportsSegments,omitempty"`
	StructuredSurface         StructuredSurface `json:"structuredSurface,omitempty"`
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
	if profile.MaxTextLength == 0 {
		profile.MaxTextLength = defaults.MaxTextLength
	}
	if !profile.SupportsSegments {
		profile.SupportsSegments = defaults.SupportsSegments
	}
	if profile.StructuredSurface == "" {
		profile.StructuredSurface = defaults.StructuredSurface
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

func defaultRenderingProfileForSource(source string, capabilities PlatformCapabilities) RenderingProfile {
	profile := RenderingProfile{
		DefaultTextFormat: TextFormatPlainText,
		SupportedFormats:  []TextFormatMode{TextFormatPlainText},
		MaxTextLength:     4096,
		SupportsSegments:  true,
		StructuredSurface: capabilities.StructuredSurface,
	}

	switch NormalizePlatformName(source) {
	case "feishu":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatLarkMD}
		profile.MaxTextLength = 30000
		profile.StructuredSurface = StructuredSurfaceCards
		profile.UsesProviderOwnedBuilders = true
	case "telegram":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText, TextFormatMarkdownV2}
		profile.MaxTextLength = 4096
		profile.StructuredSurface = StructuredSurfaceInlineKeyboard
	case "slack":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText}
		profile.MaxTextLength = 4000
		profile.StructuredSurface = StructuredSurfaceBlocks
	case "discord":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText}
		profile.MaxTextLength = 2000
		profile.StructuredSurface = StructuredSurfaceComponents
	case "dingtalk":
		profile.DefaultTextFormat = TextFormatPlainText
		profile.SupportedFormats = []TextFormatMode{TextFormatPlainText}
		profile.MaxTextLength = 20000
		profile.StructuredSurface = StructuredSurfaceActionCard
	default:
		if profile.StructuredSurface == "" {
			profile.StructuredSurface = StructuredSurfaceNone
		}
	}

	return profile
}
