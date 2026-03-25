package core

import (
	"context"
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
