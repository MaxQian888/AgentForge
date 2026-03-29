package main

import (
	"context"
	"strings"
	"testing"

	"github.com/agentforge/im-bridge/core"
)

type namedProviderTestPlatform struct {
	name string
}

func (p *namedProviderTestPlatform) Name() string { return p.name }

func (p *namedProviderTestPlatform) Start(handler core.MessageHandler) error { return nil }

func (p *namedProviderTestPlatform) Reply(ctx context.Context, replyCtx any, content string) error {
	return nil
}

func (p *namedProviderTestPlatform) Send(ctx context.Context, chatID string, content string) error {
	return nil
}

func (p *namedProviderTestPlatform) Stop() error { return nil }

func TestProviderDescriptor_NormalizedMetadataAndTransportHelpers(t *testing.T) {
	descriptor := providerDescriptor{
		ID: "discord",
		Metadata: core.PlatformMetadata{
			Source: " DISCORD-LIVE ",
		},
		SupportedTransportModes: []string{transportModeStub, strings.ToUpper(transportModeLive)},
		NewStub: func(cfg *config) (core.Platform, error) {
			return &namedProviderTestPlatform{name: "stub-platform"}, nil
		},
		NewLive: func(cfg *config) (core.Platform, error) {
			return &namedProviderTestPlatform{name: "live-platform"}, nil
		},
	}

	if got := descriptor.normalizedMetadata().Source; got != "discord" {
		t.Fatalf("normalizedMetadata.Source = %q, want discord", got)
	}
	if descriptor.normalizedMetadata().Rendering.StructuredSurface != core.StructuredSurfaceComponents {
		t.Fatalf("normalizedMetadata.Rendering.StructuredSurface = %q, want %q", descriptor.normalizedMetadata().Rendering.StructuredSurface, core.StructuredSurfaceComponents)
	}
	if !descriptor.supportsTransport(" live ") {
		t.Fatal("expected live transport to be supported")
	}
	if descriptor.supportsTransport("webhook") {
		t.Fatal("did not expect unknown transport to be supported")
	}

	stubPlatform, err := descriptor.factoryForMode("stub")(&config{})
	if err != nil {
		t.Fatalf("stub factory error: %v", err)
	}
	if stubPlatform.Name() != "stub-platform" {
		t.Fatalf("stub factory platform = %q", stubPlatform.Name())
	}

	livePlatform, err := descriptor.factoryForMode(" live ")(&config{})
	if err != nil {
		t.Fatalf("live factory error: %v", err)
	}
	if livePlatform.Name() != "live-platform" {
		t.Fatalf("live factory platform = %q", livePlatform.Name())
	}

	if descriptor.factoryForMode("invalid") != nil {
		t.Fatal("expected invalid mode to return nil factory")
	}

	fallback := providerDescriptor{ID: " SLACK "}
	if got := fallback.normalizedMetadata().Source; got != "slack" {
		t.Fatalf("fallback normalized source = %q, want slack", got)
	}
	if fallback.normalizedMetadata().Rendering.StructuredSurface != core.StructuredSurfaceBlocks {
		t.Fatalf("fallback rendering structured surface = %q, want %q", fallback.normalizedMetadata().Rendering.StructuredSurface, core.StructuredSurfaceBlocks)
	}
}

func TestProviderDescriptor_UnavailableErrorReflectsPlannedState(t *testing.T) {
	plannedErr := providerDescriptor{
		ID:            "wecom",
		Metadata:      core.PlatformMetadata{Source: "wecom"},
		PlannedReason: "runtime wiring pending",
	}.unavailableError()
	if plannedErr == nil || !strings.Contains(plannedErr.Error(), "planned but not yet runnable") {
		t.Fatalf("planned error = %v", plannedErr)
	}

	genericErr := providerDescriptor{
		ID:       "custom",
		Metadata: core.PlatformMetadata{Source: "custom"},
	}.unavailableError()
	if genericErr == nil || !strings.Contains(genericErr.Error(), "not runnable") {
		t.Fatalf("generic error = %v", genericErr)
	}
}

func TestActiveProvider_MetadataAndSource_AreNilSafe(t *testing.T) {
	var provider *activeProvider
	if got := provider.Metadata(); got.Source != "" {
		t.Fatalf("nil provider metadata = %+v", got)
	}
	if got := provider.Source(); got != "" {
		t.Fatalf("nil provider source = %q", got)
	}

	provider = &activeProvider{
		Descriptor: providerDescriptor{
			ID:       "slack",
			Metadata: core.PlatformMetadata{Source: " SLACK-STUB "},
		},
	}
	if got := provider.Source(); got != "slack" {
		t.Fatalf("provider source = %q, want slack", got)
	}
}

func TestSelectProvider_RejectsUnsupportedTransportMode(t *testing.T) {
	if _, err := selectProvider(&config{Platform: "slack", TransportMode: "invalid"}); err == nil {
		t.Fatal("expected unsupported IM_TRANSPORT_MODE to fail")
	}
}

func TestSelectProvider_ConstructsWeComStubPlatform(t *testing.T) {
	provider, err := selectProvider(&config{Platform: "wecom", TransportMode: transportModeStub, TestPort: "0"})
	if err != nil {
		t.Fatalf("selectProvider error: %v", err)
	}
	if provider == nil || provider.Platform == nil {
		t.Fatalf("provider = %+v", provider)
	}
	if provider.Source() != "wecom" {
		t.Fatalf("source = %q, want wecom", provider.Source())
	}
}

func TestSelectProvider_ConstructsQQStubPlatform(t *testing.T) {
	provider, err := selectProvider(&config{Platform: "qq", TransportMode: transportModeStub, TestPort: "0"})
	if err != nil {
		t.Fatalf("selectProvider error: %v", err)
	}
	if provider == nil || provider.Platform == nil {
		t.Fatalf("provider = %+v", provider)
	}
	if provider.Source() != "qq" {
		t.Fatalf("source = %q, want qq", provider.Source())
	}
}

func TestSelectProvider_ConstructsQQBotStubPlatform(t *testing.T) {
	provider, err := selectProvider(&config{Platform: "qqbot", TransportMode: transportModeStub, TestPort: "0"})
	if err != nil {
		t.Fatalf("selectProvider error: %v", err)
	}
	if provider == nil || provider.Platform == nil {
		t.Fatalf("provider = %+v", provider)
	}
	if provider.Source() != "qqbot" {
		t.Fatalf("source = %q, want qqbot", provider.Source())
	}
}

func TestSelectProvider_ConstructsStubPlatform(t *testing.T) {
	provider, err := selectProvider(&config{
		Platform:      "slack",
		TransportMode: "",
		TestPort:      "0",
	})
	if err != nil {
		t.Fatalf("selectProvider error: %v", err)
	}
	if provider == nil || provider.Platform == nil {
		t.Fatalf("provider = %+v", provider)
	}
	if provider.TransportMode != transportModeStub {
		t.Fatalf("transport mode = %q, want %q", provider.TransportMode, transportModeStub)
	}
	if provider.Source() != "slack" {
		t.Fatalf("source = %q, want slack", provider.Source())
	}
}

func TestSelectProvider_ValidatesLivePlatformConfig(t *testing.T) {
	_, err := selectProvider(&config{
		Platform:      "slack",
		TransportMode: transportModeLive,
	})
	if err == nil || !strings.Contains(err.Error(), "SLACK_BOT_TOKEN") {
		t.Fatalf("error = %v", err)
	}
}
