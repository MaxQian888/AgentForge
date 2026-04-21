package main

import (
	"context"
	"testing"

	"github.com/agentforge/im-bridge/client"
	"github.com/agentforge/im-bridge/core"
	"github.com/agentforge/im-bridge/core/plugin"
)

func TestBuildRegistrationInventory_MultiProvider(t *testing.T) {
	providers := []*activeProvider{
		{
			Descriptor: providerDescriptor{
				ID: "feishu",
				Metadata: core.PlatformMetadata{
					Source: "feishu",
					Capabilities: core.PlatformCapabilities{
						ReadinessTier: core.ReadinessTierFullNativeLifecycle,
					},
				},
			},
			Platform:      &fakePlatform{name: "feishu"},
			TransportMode: core.TransportModeLive,
			Tenants:       []string{"acme"},
		},
		{
			Descriptor: providerDescriptor{
				ID: "slack",
				Metadata: core.PlatformMetadata{
					Source: "slack",
					Capabilities: core.PlatformCapabilities{
						ReadinessTier: "",
					},
				},
			},
			Platform:      &fakePlatform{name: "slack"},
			TransportMode: core.TransportModeStub,
			Tenants:       []string{"beta"},
		},
	}

	reg := plugin.NewRegistry("")

	inv := buildRegistrationInventory(providers, reg)

	if len(inv.Providers) != 2 {
		t.Fatalf("Providers len = %d, want 2", len(inv.Providers))
	}
	if inv.Providers[0].ID != "feishu" {
		t.Errorf("Providers[0].ID = %q, want feishu", inv.Providers[0].ID)
	}
	if inv.Providers[0].Transport != "live" {
		t.Errorf("Providers[0].Transport = %q, want live", inv.Providers[0].Transport)
	}
	if inv.Providers[0].ReadinessTier != "full_native_lifecycle" {
		t.Errorf("Providers[0].ReadinessTier = %q", inv.Providers[0].ReadinessTier)
	}
	if len(inv.Providers[0].Tenants) == 0 || inv.Providers[0].Tenants[0] != "acme" {
		t.Errorf("Providers[0].Tenants = %v", inv.Providers[0].Tenants)
	}
	if inv.Providers[0].MetadataSource != "builtin" {
		t.Errorf("Providers[0].MetadataSource = %q, want builtin", inv.Providers[0].MetadataSource)
	}
	if inv.Providers[1].ID != "slack" || inv.Providers[1].Transport != "stub" {
		t.Errorf("Providers[1] = %+v", inv.Providers[1])
	}
	if len(inv.CommandPlugins) != 0 {
		t.Errorf("CommandPlugins len = %d, want 0", len(inv.CommandPlugins))
	}
}

func TestBuildRegistrationInventory_NilPluginRegistry(t *testing.T) {
	providers := []*activeProvider{{
		Descriptor: providerDescriptor{
			ID:       "feishu",
			Metadata: core.PlatformMetadata{Source: "feishu"},
		},
		Platform:      &fakePlatform{name: "feishu"},
		TransportMode: core.TransportModeStub,
	}}
	inv := buildRegistrationInventory(providers, nil)
	if len(inv.CommandPlugins) != 0 {
		t.Errorf("nil registry should yield 0 command plugins, got %d", len(inv.CommandPlugins))
	}
}

// TestBuildRegistrationInventory_WireShape guards the serialized JSON
// matches the backend model exactly.
func TestBuildRegistrationInventory_WireShape(t *testing.T) {
	providers := []*activeProvider{{
		Descriptor: providerDescriptor{
			ID: "slack",
			Metadata: core.PlatformMetadata{
				Source:       "slack",
				Capabilities: core.PlatformCapabilities{SupportsRichMessages: true},
			},
		},
		Platform:      &fakePlatform{name: "slack"},
		TransportMode: "live",
		Tenants:       []string{"acme"},
	}}
	inv := buildRegistrationInventory(providers, nil)
	// Confirm the struct is assignable to the exported client.BridgeRegistration
	// payload field without intermediate transformation.
	var reg client.BridgeRegistration
	reg.Providers = inv.Providers
	reg.CommandPlugins = inv.CommandPlugins
	if len(reg.Providers) != 1 {
		t.Errorf("wire assignment dropped providers: %v", reg.Providers)
	}
}

// TestBuildRegistrationInventory_ReflectsTenantUpdates verifies that
// buildRegistrationInventory reads p.Tenants as it stands at call time, so
// the SIGHUP redistribution wiring (which writes refreshed IDs back to each
// activeProvider.Tenants before calling reregister) is correctly reflected in
// the registration payload sent to the orchestrator.
func TestBuildRegistrationInventory_ReflectsTenantUpdates(t *testing.T) {
	provider := &activeProvider{
		Descriptor: providerDescriptor{
			ID:       "feishu",
			Metadata: core.PlatformMetadata{Source: "feishu"},
		},
		Platform:      &fakePlatform{name: "feishu"},
		TransportMode: core.TransportModeLive,
		Tenants:       []string{"acme"},
	}

	inv1 := buildRegistrationInventory([]*activeProvider{provider}, nil)
	if len(inv1.Providers[0].Tenants) != 1 || inv1.Providers[0].Tenants[0] != "acme" {
		t.Fatalf("initial Tenants = %v, want [acme]", inv1.Providers[0].Tenants)
	}

	// Simulate SIGHUP tenant refresh: redistribute the new tenant set to the
	// activeProvider (mirrors the fix in the SIGHUP handler).
	provider.Tenants = []string{"acme", "beta"}

	inv2 := buildRegistrationInventory([]*activeProvider{provider}, nil)
	if len(inv2.Providers[0].Tenants) != 2 || inv2.Providers[0].Tenants[1] != "beta" {
		t.Errorf("post-refresh Tenants = %v, want [acme beta]", inv2.Providers[0].Tenants)
	}
}

// fakePlatform is a minimal core.Platform test double. It provides just
// the methods buildRegistrationInventory exercises; unused methods return
// zero values.
type fakePlatform struct {
	name string
}

func (f *fakePlatform) Name() string                                                        { return f.name }
func (f *fakePlatform) Start(handler core.MessageHandler) error                             { return nil }
func (f *fakePlatform) Reply(ctx context.Context, replyCtx any, content string) error      { return nil }
func (f *fakePlatform) Send(ctx context.Context, chatID string, content string) error       { return nil }
func (f *fakePlatform) Stop() error                                                         { return nil }
