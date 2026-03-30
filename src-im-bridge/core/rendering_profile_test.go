package core

import (
	"reflect"
	"testing"
)

func TestNormalizeRenderingProfile_FillsDefaultsAndCopiesSlices(t *testing.T) {
	defaults := RenderingProfile{
		DefaultTextFormat:         TextFormatSlackMrkdwn,
		SupportedFormats:          []TextFormatMode{TextFormatPlainText, TextFormatSlackMrkdwn},
		NativeSurfaces:            []string{NativeSurfaceSlackBlockKit},
		MaxTextLength:             4000,
		SupportsSegments:          true,
		StructuredSurface:         StructuredSurfaceBlocks,
		UsesProviderOwnedBuilders: true,
	}

	got := normalizeRenderingProfile(RenderingProfile{}, defaults)

	if got.DefaultTextFormat != TextFormatSlackMrkdwn {
		t.Fatalf("DefaultTextFormat = %q, want %q", got.DefaultTextFormat, TextFormatSlackMrkdwn)
	}
	if !reflect.DeepEqual(got.SupportedFormats, defaults.SupportedFormats) {
		t.Fatalf("SupportedFormats = %+v, want %+v", got.SupportedFormats, defaults.SupportedFormats)
	}
	if !reflect.DeepEqual(got.NativeSurfaces, defaults.NativeSurfaces) {
		t.Fatalf("NativeSurfaces = %+v, want %+v", got.NativeSurfaces, defaults.NativeSurfaces)
	}
	if got.MaxTextLength != 4000 {
		t.Fatalf("MaxTextLength = %d, want 4000", got.MaxTextLength)
	}
	if !got.SupportsSegments {
		t.Fatal("expected SupportsSegments to inherit defaults")
	}
	if got.StructuredSurface != StructuredSurfaceBlocks {
		t.Fatalf("StructuredSurface = %q, want %q", got.StructuredSurface, StructuredSurfaceBlocks)
	}
	if !got.UsesProviderOwnedBuilders {
		t.Fatal("expected UsesProviderOwnedBuilders to inherit defaults")
	}

	got.SupportedFormats[0] = TextFormatHTML
	got.NativeSurfaces[0] = NativeSurfaceDiscordEmbed
	if defaults.SupportedFormats[0] != TextFormatPlainText {
		t.Fatalf("defaults.SupportedFormats mutated = %+v", defaults.SupportedFormats)
	}
	if defaults.NativeSurfaces[0] != NativeSurfaceSlackBlockKit {
		t.Fatalf("defaults.NativeSurfaces mutated = %+v", defaults.NativeSurfaces)
	}
}

func TestNormalizeRenderingProfile_UsesPlainTextAndPrependsMissingDefault(t *testing.T) {
	got := normalizeRenderingProfile(RenderingProfile{
		DefaultTextFormat: TextFormatHTML,
		SupportedFormats:  []TextFormatMode{TextFormatPlainText},
	}, RenderingProfile{})

	if got.DefaultTextFormat != TextFormatHTML {
		t.Fatalf("DefaultTextFormat = %q, want %q", got.DefaultTextFormat, TextFormatHTML)
	}
	if !reflect.DeepEqual(got.SupportedFormats, []TextFormatMode{TextFormatHTML, TextFormatPlainText}) {
		t.Fatalf("SupportedFormats = %+v", got.SupportedFormats)
	}

	plain := normalizeRenderingProfile(RenderingProfile{}, RenderingProfile{})
	if plain.DefaultTextFormat != TextFormatPlainText {
		t.Fatalf("plain DefaultTextFormat = %q, want %q", plain.DefaultTextFormat, TextFormatPlainText)
	}
	if !reflect.DeepEqual(plain.SupportedFormats, []TextFormatMode{TextFormatPlainText}) {
		t.Fatalf("plain SupportedFormats = %+v", plain.SupportedFormats)
	}
}

func TestRenderingProfileHelpers_AreCaseInsensitive(t *testing.T) {
	if !hasTextFormat([]TextFormatMode{TextFormatSlackMrkdwn}, TextFormatMode("SLACK_MRKDWN")) {
		t.Fatal("expected hasTextFormat to be case-insensitive")
	}
	if hasTextFormat([]TextFormatMode{TextFormatPlainText}, TextFormatMarkdownV2) {
		t.Fatal("did not expect unrelated format to match")
	}
	if !hasStringFold([]string{" slack_block_kit "}, "SLACK_BLOCK_KIT") {
		t.Fatal("expected hasStringFold to trim and compare case-insensitively")
	}
	if hasStringFold([]string{"slack_block_kit"}, NativeSurfaceDiscordEmbed) {
		t.Fatal("did not expect unrelated surface to match")
	}
}

func TestDefaultRenderingProfileForSource_CoversProviderDefaults(t *testing.T) {
	testCases := []struct {
		name                      string
		source                    string
		capabilities              PlatformCapabilities
		wantDefault               TextFormatMode
		wantFormats               []TextFormatMode
		wantNative                []string
		wantMax                   int
		wantSurface               StructuredSurface
		wantProviderOwnedBuilders bool
	}{
		{
			name:                      "feishu",
			source:                    "FEISHU-LIVE",
			wantDefault:               TextFormatPlainText,
			wantFormats:               []TextFormatMode{TextFormatPlainText, TextFormatLarkMD},
			wantNative:                []string{NativeSurfaceFeishuCard},
			wantMax:                   30000,
			wantSurface:               StructuredSurfaceCards,
			wantProviderOwnedBuilders: true,
		},
		{
			name:        "telegram",
			source:      "telegram",
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText, TextFormatMarkdownV2},
			wantNative:  []string{NativeSurfaceTelegramRich},
			wantMax:     4096,
			wantSurface: StructuredSurfaceInlineKeyboard,
		},
		{
			name:        "slack",
			source:      "slack",
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText, TextFormatSlackMrkdwn},
			wantNative:  []string{NativeSurfaceSlackBlockKit},
			wantMax:     4000,
			wantSurface: StructuredSurfaceBlocks,
		},
		{
			name:        "discord",
			source:      "discord",
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText, TextFormatDiscordMD},
			wantNative:  []string{NativeSurfaceDiscordEmbed},
			wantMax:     2000,
			wantSurface: StructuredSurfaceComponents,
		},
		{
			name:        "dingtalk",
			source:      "dingtalk",
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText, TextFormatDingTalkMD},
			wantNative:  []string{NativeSurfaceDingTalkCard},
			wantMax:     20000,
			wantSurface: StructuredSurfaceActionCard,
		},
		{
			name:        "qq",
			source:      "qq",
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText},
			wantNative:  nil,
			wantMax:     4096,
			wantSurface: StructuredSurfaceNone,
		},
		{
			name:        "qqbot",
			source:      "qqbot",
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText},
			wantNative:  []string{NativeSurfaceQQBotMarkdown},
			wantMax:     2000,
			wantSurface: StructuredSurfaceNone,
		},
		{
			name:        "wecom",
			source:      "wecom",
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText},
			wantNative:  []string{NativeSurfaceWeComCard},
			wantMax:     2000,
			wantSurface: StructuredSurfaceNone,
		},
		{
			name:   "custom keeps explicit capabilities",
			source: "custom",
			capabilities: PlatformCapabilities{
				StructuredSurface: StructuredSurfaceActionCard,
				NativeSurfaces:    []string{"custom_native"},
			},
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText},
			wantNative:  []string{"custom_native"},
			wantMax:     4096,
			wantSurface: StructuredSurfaceActionCard,
		},
		{
			name:        "custom falls back to none",
			source:      "custom",
			wantDefault: TextFormatPlainText,
			wantFormats: []TextFormatMode{TextFormatPlainText},
			wantNative:  nil,
			wantMax:     4096,
			wantSurface: StructuredSurfaceNone,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := defaultRenderingProfileForSource(tc.source, tc.capabilities)

			if got.DefaultTextFormat != tc.wantDefault {
				t.Fatalf("DefaultTextFormat = %q, want %q", got.DefaultTextFormat, tc.wantDefault)
			}
			if !reflect.DeepEqual(got.SupportedFormats, tc.wantFormats) {
				t.Fatalf("SupportedFormats = %+v, want %+v", got.SupportedFormats, tc.wantFormats)
			}
			if !reflect.DeepEqual(got.NativeSurfaces, tc.wantNative) {
				t.Fatalf("NativeSurfaces = %+v, want %+v", got.NativeSurfaces, tc.wantNative)
			}
			if got.MaxTextLength != tc.wantMax {
				t.Fatalf("MaxTextLength = %d, want %d", got.MaxTextLength, tc.wantMax)
			}
			if got.StructuredSurface != tc.wantSurface {
				t.Fatalf("StructuredSurface = %q, want %q", got.StructuredSurface, tc.wantSurface)
			}
			if !got.SupportsSegments {
				t.Fatal("expected SupportsSegments to stay enabled for default profiles")
			}
			if got.UsesProviderOwnedBuilders != tc.wantProviderOwnedBuilders {
				t.Fatalf("UsesProviderOwnedBuilders = %v, want %v", got.UsesProviderOwnedBuilders, tc.wantProviderOwnedBuilders)
			}
		})
	}
}
